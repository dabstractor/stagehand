package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// FileChange is one entry in a diff-tree "what landed" listing.
// diff-tree --name-status -r emits "<status>\t<path>" or "<status>\t<src>\t<dst>" (rename/copy).
// The S6 (DiffTree) implementation parses these lines into FileChange values.
type FileChange struct {
	Status  string // "A","M","D","R","C","T","U"; R/C carry a similarity score e.g. "R100"
	SrcPath string // non-empty only for R/C (the rename/copy source); "" otherwise
	Path    string // the destination path — always set
}

// StagedDiffOptions configures staged-diff capture (commit-pi parity, PRD §9.1 / FINDING 7).
// The T3.S1 (StagedDiff) implementation consumes these.
type StagedDiffOptions struct {
	MaxDiffBytes int      // byte cap on the non-markdown section (commit-pi default 300000); 0 = unlimited
	MaxMDLines   int      // per-file line cap for markdown files (commit-pi default 100); 0 = unlimited
	Excludes     []string // pathspec magic-prefix excludes, e.g. []string{":!*.lock", ":!vendor/*"}
}

// Git is the shell-free boundary to the real git binary. Every method delegates to the private
// run() helper on *gitRunner, which execs git with args as []string (NEVER sh -c — PRD §19) and
// targets the repo via the -C flag (NEVER os.Chdir — goroutine-safe).
//
// Method ownership (each implemented in its own later subtask):
//
//	RevParseHEAD      — P1.M1.T2.S2   WriteTree        — P1.M1.T2.S3
//	CommitTree        — P1.M1.T2.S4   UpdateRefCAS     — P1.M1.T2.S5
//	DiffTree          — P1.M1.T2.S6
//	StagedDiff        — P1.M1.T3.S1   HasStagedChanges — P1.M1.T3.S2
//	RecentMessages    — P1.M1.T3.S3   CommitCount      — P1.M1.T3.S3
//	RecentSubjects    — P1.M1.T3.S4   AddAll           — P1.M1.T3.S5
type Git interface {
	// RevParseHEAD returns the SHA HEAD points at. On a repo with zero commits it returns
	// sha="" and isUnborn=true (detected via git exit 128, NOT stdout emptiness — FINDING 1).
	RevParseHEAD(ctx context.Context) (sha string, isUnborn bool, err error)

	// WriteTree materializes the index into a tree object and returns its SHA. Fails (non-nil err)
	// when the index has unresolved merge conflicts (git exit 128).
	WriteTree(ctx context.Context) (sha string, err error)

	// CommitTree creates a commit object for tree with the given parents and message (delivered
	// via stdin with -F -). parents==nil/empty ⇒ root commit (no -p). Returns the new commit SHA.
	CommitTree(ctx context.Context, tree string, parents []string, msg string) (sha string, err error)

	// UpdateRefCAS atomically moves ref to newSHA only if it currently equals expectedOld
	// (3-arg compare-and-swap; NEVER the 2-arg force form). For a root commit pass expectedOld =
	// the all-zeros hash. Returns a non-nil err on CAS mismatch (HEAD moved).
	UpdateRefCAS(ctx context.Context, ref, newSHA, expectedOld string) error

	// DiffTree returns the file-level change set of sha vs its first parent ("what landed").
	// isRoot must be true for a root commit so git diffs against the empty tree (--root flag).
	DiffTree(ctx context.Context, sha string, isRoot bool) ([]FileChange, error)

	// StagedDiff returns the staged diff payload (markdown per-file + non-markdown aggregate),
	// applying byte/line caps and pathspec excludes per opts (commit-pi parity, PRD §9.1).
	StagedDiff(ctx context.Context, opts StagedDiffOptions) (diff string, err error)

	// HasStagedChanges reports whether the index differs from HEAD (git diff --cached --quiet:
	// exit 1 ⇒ true, exit 0 ⇒ false). NOT an error when changes exist (FINDING 6).
	HasStagedChanges(ctx context.Context) (bool, error)

	// RecentMessages returns up to n most-recent full commit messages (NUL-delimited query,
	// FINDING 9). Callers must short-circuit when RevParseHEAD reports isUnborn.
	RecentMessages(ctx context.Context, n int) (messages []string, err error)

	// RecentSubjects returns up to n most-recent commit subjects (first line) for duplicate
	// detection. Callers must short-circuit when isUnborn.
	RecentSubjects(ctx context.Context, n int) (subjects []string, err error)

	// CommitCount returns the number of commits reachable from HEAD (decides mature vs new-repo
	// prompt). Callers must short-circuit when isUnborn.
	CommitCount(ctx context.Context) (count int, err error)

	// AddAll stages all changes (git add -A). Used by the auto-stage-all path (PRD §9.4 / FINDING 11).
	AddAll(ctx context.Context) error
}

// gitRunner is the production Git implementation. It wraps exec.CommandContext for the real git
// binary. Construct with New.
type gitRunner struct {
	workDir string // the repo path passed as -C <repo> by every bound method
}

// New returns a Git bound to workDir. The git binary is resolved lazily inside run() (New has no
// error return); a missing git surfaces as a runtime error from the first run() call.
func New(workDir string) Git {
	return &gitRunner{workDir: workDir}
}

// run is the low-level git exec helper. It is the ONLY place Stagehand shells out to git.
//   - resolves the git binary via exec.LookPath (PRD §19: real binary, never go-git per §22.3)
//   - targets repo via the -C flag (NOT os.Chdir / cmd.Dir — goroutine-safe)
//   - captures stdout and stderr to SEPARATE buffers
//   - returns the exit code extracted from *exec.ExitError
//
// INVARIANT: a NON-ZERO git exit is returned as (stdout, stderr, exitCode, nil) — err is nil.
// Git uses exit codes as semantic signals (1 = has-staged; 128 = unborn/not-a-SHA), and callers
// inspect exitCode. Only infrastructural failures (LookPath miss, context cancel, start/I/O)
// return err != nil, with exitCode = -1.
func (g *gitRunner) run(ctx context.Context, repo string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	gitPath, lerr := exec.LookPath("git")
	if lerr != nil {
		return "", "", -1, fmt.Errorf("git binary not found in PATH: %w", lerr)
	}

	full := make([]string, 0, len(args)+2)
	full = append(full, "-C", repo) // repo via flag, not cmd.Dir (gotcha G1)
	full = append(full, args...)

	cmd := exec.CommandContext(ctx, gitPath, full...) // []string args, NO shell (PRD §19)
	var out, errb bytes.Buffer
	cmd.Stdout = &out  // separate buffer
	cmd.Stderr = &errb // separate buffer

	runErr := cmd.Run()
	stdout, stderr = out.String(), errb.String()

	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	if cerr := ctx.Err(); cerr != nil { // context cancelled (timeout/signal) — not a git exit
		return stdout, stderr, -1, cerr
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) { // non-zero git exit → capture code, err stays nil (gotcha G2)
		return stdout, stderr, exitErr.ExitCode(), nil
	}
	return stdout, stderr, -1, runErr // start / I/O failure
}

// runWithInput is run() plus a stdin pipe. It exists because run() cannot set cmd.Stdin (its body
// leaves stdin as /dev/null), and commit-tree with -F - must read the commit message from stdin
// (FINDING 4: -F - avoids ALL quoting/special-character/leading-dash issues that -m would suffer).
// It is the ONLY other place Stagehand shells out to git; it is co-located with run() and shares
// its structure exactly (LookPath → -C repo → separate buffers → errors.As(ExitError) with
// err==nil for non-zero exits). run() itself is intentionally left unmodified (see research §1).
//
// Identity: cmd.Env is NOT set here, so the child inherits the parent environment. Production
// callers commit AS the configured user (git resolves user.name/user.email from config/env);
// tests set repo-local user.name/user.email via `git config` (see committree_test.go).
func (g *gitRunner) runWithInput(ctx context.Context, repo string, stdin io.Reader, args ...string) (stdout string, stderr string, exitCode int, err error) {
	gitPath, lerr := exec.LookPath("git")
	if lerr != nil {
		return "", "", -1, fmt.Errorf("git binary not found in PATH: %w", lerr)
	}

	full := make([]string, 0, len(args)+2)
	full = append(full, "-C", repo) // repo via flag, not cmd.Dir (gotcha G1 of S1)
	full = append(full, args...)

	cmd := exec.CommandContext(ctx, gitPath, full...) // []string args, NO shell (PRD §19)
	cmd.Stdin = stdin                                 // ← the one difference from run()
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	runErr := cmd.Run()
	stdout, stderr = out.String(), errb.String()

	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	if cerr := ctx.Err(); cerr != nil { // context cancelled (timeout/signal) — not a git exit
		return stdout, stderr, -1, cerr
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) { // non-zero git exit → capture code, err stays nil
		return stdout, stderr, exitErr.ExitCode(), nil
	}
	return stdout, stderr, -1, runErr // start / I/O failure
}

// ---- Stubs: each method is implemented in its own later subtask. They panic to fail fast. ----

// RevParseHEAD returns the SHA HEAD currently points at. On a repository with zero commits it
// returns sha="" and isUnborn=true, detected via git's exit code 128 (NOT stdout emptiness —
// `git rev-parse HEAD` prints the literal string "HEAD\n" to stdout on an unborn repo, which is
// the latent bug in commit-pi; see critical_findings.md FINDING 1).
func (g *gitRunner) RevParseHEAD(ctx context.Context) (sha string, isUnborn bool, err error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "rev-parse", "HEAD")
	if err != nil {
		return "", false, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code == 128 {
		return "", true, nil // unborn repo — exit-code signal, NOT string emptiness
	}
	if code != 0 {
		return "", false, fmt.Errorf("git rev-parse HEAD: unexpected exit %d: %s", code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), false, nil
}

// WriteTree materializes the current index into a tree object and returns its SHA. It is a
// read-only-with-respect-to-refs operation: it writes a tree object to the object store but does
// NOT modify the index or HEAD (PRD §13.2). It is the immutable-snapshot primitive consumed by
// CommitTree (P1.M1.T2.S4) and the rescue protocol (P1.M3.T3).
//
// write-tree fails (non-zero exit, 128 on git 2.x) when the index has unresolved merge conflicts
// (unmerged stage 1/2/3 entries). That is surfaced here as run()'s exitCode != 0 (err stays nil per
// run()'s invariant); the error names "unresolved merge conflicts" and includes the trimmed stderr,
// whose text contains "unmerged"/"error building trees" on a real conflict (git_plumbing_reference
// §1: the stable signal is exit ≠ 0; do NOT match a single exact stderr phrase).
func (g *gitRunner) WriteTree(ctx context.Context) (sha string, err error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "write-tree")
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return "", fmt.Errorf("git write-tree: unresolved merge conflicts in index (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

// CommitTree creates a commit object for tree with the given parents and message and returns its
// SHA. The message is delivered via stdin with `-F -` (NOT -m) so it is bulletproof against special
// characters, leading dashes, quotes, and newlines (FINDING 4; verified empirically that a message
// beginning with "-n -p --foo" is stored verbatim). parents == nil/empty ⇒ root commit (no -p);
// each element of a non-empty parents slice appends a `-p <parent>` (repeatable, forward-compatible
// with v2 merge commits). Like write-tree, this does NOT move any ref: the returned commit is a
// dangling object until UpdateRefCAS (P1.M1.T2.S5) publishes it (PRD §13.2, §18.1).
//
// commit-tree fails (non-zero exit, 128 on git 2.x) when tree or a parent is not a valid object;
// that is surfaced here as runWithInput's exitCode != 0 (err stays nil per its invariant).
func (g *gitRunner) CommitTree(ctx context.Context, tree string, parents []string, msg string) (sha string, err error) {
	args := make([]string, 0, 4+len(parents)*2)
	args = append(args, "commit-tree", tree)
	for _, p := range parents {
		args = append(args, "-p", p) // repeatable; root commit = empty parents (no -p appended)
	}
	args = append(args, "-F", "-") // message via stdin — avoids all quoting pitfalls (FINDING 4)

	stdout, stderr, code, err := g.runWithInput(ctx, g.workDir, strings.NewReader(msg), args...)
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return "", fmt.Errorf("git commit-tree: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

// ErrCASFailed is returned by UpdateRefCAS when git's compare-and-swap did not match — i.e. HEAD
// moved concurrently since the snapshot (or expectedOld was the all-zeros hash on a repo that
// already has commits). The orchestrator detects it via errors.Is(err, ErrCASFailed) to emit PRD
// §13.5's "HEAD moved from <expected> to <actual>" message and exit 1 (FR41/§18.2). It is NOT
// returned for infrastructural failures (missing git binary, cancelled context); those propagate the
// underlying error unchanged so they remain distinguishable. The <actual> SHA is re-read by the
// orchestrator via RevParseHEAD when it observes this error (it is deliberately NOT captured here —
// see P1.M1.T2.S5 research §3 / decision D5).
var ErrCASFailed = errors.New("git update-ref: compare-and-swap failed (ref moved since snapshot)")

// UpdateRefCAS atomically moves ref to newSHA only if ref's current value equals expectedOld — the
// 3-arg compare-and-swap form of git update-ref (git takes the ref lock, reads the current value, and
// writes newSHA only if current == expectedOld, all under .git/<ref>.lock in one process). It is the
// SOLE point at which Stagehand mutates a ref (PRD §18.1: "refs are modified only at the final
// update-ref step, and only if HEAD is unchanged since the snapshot"). The 2-arg force form is NEVER
// used — it would silently clobber a concurrent commit (PRD §13.1/§13.2/§18.2).
//
// For a root commit (unborn repo), the caller passes expectedOld = the all-zeros hash (40 zeros for
// sha-1); the CAS then succeeds only if HEAD is truly unborn (UpdateRefCAS itself has no isUnborn
// knowledge — the caller, via RevParseHEAD, decides). On any non-zero exit the CAS did not match
// (HEAD moved, or all-zeros-expected on a repo that already has commits): return ErrCASFailed
// (wrapped, so errors.Is works) carrying the exit code + git's own stderr for diagnostics. FINDING 3:
// stderr varies by scenario/version — detection is by exit code (code != 0), NEVER by matching the
// stderr string.
func (g *gitRunner) UpdateRefCAS(ctx context.Context, ref, newSHA, expectedOld string) error {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "update-ref", ref, newSHA, expectedOld)
	_ = stdout // update-ref prints nothing on success; referenced to silence unused-var linters
	if err != nil {
		return err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
	}
	if code != 0 {
		// CAS did not match. Branch on code (!= 0), NOT on a specific exit or stderr text (FINDING 3,
		// gotcha G1/G2). Wrap with %w so errors.Is(err, ErrCASFailed) is true (gotcha G3).
		return fmt.Errorf("%w (exit %d): %s", ErrCASFailed, code, strings.TrimSpace(stderr))
	}
	return nil
}

// DiffTree returns the file-level change set of commit sha versus its first parent — the "what
// landed" report printed after a successful commit (PRD §9.9/FR42, Appendix C). It runs
// `git diff-tree --no-commit-id --name-status -r [--root] <sha>` and parses the tab-separated output
// into []FileChange. For a root commit (no parent), isRoot MUST be true so git diffs against the
// empty tree via --root; otherwise a root commit yields NO output (verified on git 2.54.0: empty
// stdout, exit 0 — the trap the isRoot parameter exists to avoid). The command intentionally does NOT
// pass -M (rename detection): it reproduces commit-pi's exact `diff-tree --name-status` UX (PRD
// Appendix C "Identical UX"), so renames surface as a D+A pair; parseDiffTree still handles 3-field
// R/C lines defensively.
//
// diff-tree exits 128 only on a bad/unresolvable SHA (verified); that is surfaced via run()'s
// exitCode != 0 (err stays nil per run()'s invariant). Empty output (root-without---root, or a
// no-change commit) is exit 0 and yields a nil slice — NOT an error.
func (g *gitRunner) DiffTree(ctx context.Context, sha string, isRoot bool) ([]FileChange, error) {
	args := []string{"diff-tree", "--no-commit-id", "--name-status", "-r"}
	if isRoot {
		args = append(args, "--root") // root commit: diff against the empty tree (G1)
	}
	args = append(args, sha) // flags first, then the positional SHA (G14)

	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
	}
	if code != 0 {
		// Only a bad SHA reaches here (exit 128). Branch on code != 0, not code == 128 (G2).
		return nil, fmt.Errorf("git diff-tree: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return parseDiffTree(stdout), nil // may be a nil slice for empty output (G5)
}

// parseDiffTree parses the tab-separated output of `git diff-tree --no-commit-id --name-status -r`
// into FileChange values. Each non-empty line is one of:
//
//	"<status>\t<path>"               (A/M/D/T — 2 fields)
//	"<status><score>\t<src>\t<dst>"  (R/C — 3 fields, e.g. "R100\told.txt\tnew.txt")
//
// Empty lines (including the trailing newline after TrimSpace) are skipped. Lines with any other
// field count are skipped defensively (git output is well-formed, so this never fires in practice).
// Returns a nil slice for empty/whitespace-only input (range-safe).
func parseDiffTree(out string) []FileChange {
	var changes []FileChange
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		switch len(fields) {
		case 2:
			changes = append(changes, FileChange{Status: fields[0], Path: fields[1]})
		case 3:
			changes = append(changes, FileChange{Status: fields[0], SrcPath: fields[1], Path: fields[2]})
		}
	}
	return changes
}

func (g *gitRunner) StagedDiff(ctx context.Context, opts StagedDiffOptions) (string, error) {
	panic("gitRunner.StagedDiff: not yet implemented — see P1.M1.T3.S1")
}

func (g *gitRunner) HasStagedChanges(ctx context.Context) (bool, error) {
	panic("gitRunner.HasStagedChanges: not yet implemented — see P1.M1.T3.S2")
}

func (g *gitRunner) RecentMessages(ctx context.Context, n int) ([]string, error) {
	panic("gitRunner.RecentMessages: not yet implemented — see P1.M1.T3.S3")
}

func (g *gitRunner) RecentSubjects(ctx context.Context, n int) ([]string, error) {
	panic("gitRunner.RecentSubjects: not yet implemented — see P1.M1.T3.S4")
}

func (g *gitRunner) CommitCount(ctx context.Context) (int, error) {
	panic("gitRunner.CommitCount: not yet implemented — see P1.M1.T3.S3")
}

func (g *gitRunner) AddAll(ctx context.Context) error {
	panic("gitRunner.AddAll: not yet implemented — see P1.M1.T3.S5")
}
