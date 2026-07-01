package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strconv"
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

// LogEntry is one commit in a log range (oldest-first when produced by LogRange).
// It is the post-arbiter re-read primitive (PRD §13.6.5 / FR-M9): after the arbiter amends/rebuilds/
// creates commits, the orchestrator re-reads the final commits in preRunHEAD..HEAD via LogRange and
// pairs each entry's SHA with DiffTree to rebuild the accurate success report (FR42).
type LogEntry struct {
	SHA     string // full 40/64-hex commit SHA (git %H)
	Subject string // first line of the commit message (git %s — single-line by construction)
}

// StagedDiffOptions configures staged-diff capture (commit-pi parity, PRD §9.1 / FINDING 7).
// The T3.S1 (StagedDiff) implementation consumes these.
type StagedDiffOptions struct {
	MaxDiffBytes     int      // byte cap on the non-markdown section (commit-pi default 300000); 0 = unlimited
	MaxMDLines       int      // per-file line cap for markdown files (commit-pi default 100); 0 = unlimited
	Excludes         []string // pathspec magic-prefix excludes, e.g. []string{":!*.lock", ":!vendor/*"}
	BinaryExtensions []string // extra non-text extensions to filter beyond the built-in denylist
	// (png jpg … woff2 in internal/git/binary.go); nil ⇒ built-in denylist only.
	// Entries are dot-tolerant + case-insensitive (PRD §9.1 FR3a).
	// Sourced from config `binary_extensions`.
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
//	RecentSubjects    — P1.M1.T3.S4   AddAll / StagedFileCount — P1.M1.T3.S5
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

	// Add stages the given paths (modifications, additions/untracked, AND deletions) into the index
	// via `git add -- <paths...>`. It MUTATES THE INDEX (writes .git/index) but touches NO ref
	// (PRD §18.1). It is the path-specific companion to AddAll, consumed ONLY by the arbiter's
	// mid-chain chain rebuild (PRD §13.6.5 — "add leftover paths"): after ReadTree(tree[j]) the index
	// == tree[j] and Add(leftoverPaths) folds JUST the leftovers onto it (AddAll would collapse the
	// chain). Empty paths ⇒ no-op nil. The `--` guards pathspec ambiguity. ALL non-zero exits are
	// errors (the mutation convention shared with AddAll/ReadTree/WriteTree/CommitTree — no 128-as-
	// non-error special-case). Read-only w.r.t. refs.
	Add(ctx context.Context, paths []string) error

	// StagedFileCount returns the number of files currently staged (git diff --cached --name-only,
	// count of non-empty lines). Used for the FR18 "Nothing staged — staging all changes (N files)."
	// notice. Read-only with respect to refs and the index.
	StagedFileCount(ctx context.Context) (int, error)

	// RevParseTree returns the tree SHA of a commit-ish: ref is "HEAD", a branch name, or a commit SHA.
	// It runs `git rev-parse <ref>^{tree}`, where the `^{tree}` suffix peels the commit-ish to its tree
	// object (the tree a commit points at). It is the producer of tree[-1] — the original-parent tree
	// that anchors the multi-commit concept-diff loop (PRD §13.6.3: "`tree[-1]` is the original parent
	// tree (`git rev-parse HEAD^{tree}`, or the empty tree for an unborn repo)", invariant 2 mandates
	// tree-to-tree concept diffs, never index-vs-HEAD).
	//
	// On an unborn repo with ref="HEAD", or on any unresolvable ref, git exits 128; RevParseTree returns
	// ("", nil) defensively (NOT an error) — callers gate on RevParseHEAD's isUnborn before calling, so an
	// empty return is the correct non-error signal for the unborn/empty-tree base case. This 128-as-non-error
	// convention is identical to RevParseHEAD / RecentMessages / RecentSubjects / CommitCount. Branch on the
	// exit code, NOT on stdout emptiness: git prints the literal argument string to stdout on exit 128.
	RevParseTree(ctx context.Context, ref string) (tree string, err error)

	// ReadTree REPLACES the index with the contents of <tree> via `git read-tree <tree>` (the default,
	// no -m/--merge form). It MUTATES THE INDEX (writes .git/index) but touches NEITHER HEAD NOR any ref
	// (PRD §18.1: refs move ONLY at UpdateRefCAS). It is consumed ONLY by the arbiter's mid-chain chain
	// rebuild (PRD §13.6.5: "for each j, read-tree the appropriate base, fold the leftovers in at j==i,
	// write-tree, commit-tree against the rebuilt parent, update-ref"). Because it is a mutation, EVERY
	// non-zero exit (128 = bad/unresolvable tree SHA, not-a-repo, corrupt object) is a real error — the
	// SAME convention as AddAll / WriteTree / CommitTree (mutations never special-case 128 as "unborn").
	ReadTree(ctx context.Context, tree string) error

	// TreeDiff returns the concept diff between two tree SHAs via `git diff <treeA> <treeB>` — the
	// per-concept tree-to-tree diff the multi-commit message agent reasons over (PRD §13.6.3 invariant 2:
	// "the concept diff is computed tree-to-tree, never index-vs-HEAD; message[i] reasons over
	// `git diff tree[i-1] tree[i]`"). It is the tree-to-tree analogue of StagedDiff (which is index-vs-HEAD):
	// it applies the SAME caps, pathspec excludes, and FR3c binary filtering (identical placeholder format
	// in every diff path), and reuses StagedDiffOptions. For the unborn-repo base case the caller passes
	// EmptyTreeSHA as treeA (TreeDiff itself is NOT unborn-aware — the caller resolves trees via RevParseTree
	// and converts the unborn base to EmptyTreeSHA). A no-change diff (treeA == treeB) returns ("", nil).
	//
	// `git diff` (without --quiet) exits 0 whether or not there are changes; exit 128 means a bad or
	// unresolvable tree SHA, which is a REAL error (NOT an unborn signal — branch on code != 0, never on
	// code == 128). Read-only with respect to refs and the index.
	TreeDiff(ctx context.Context, treeA, treeB string, opts StagedDiffOptions) (diff string, err error)

	// StatusPorcelain returns the output of `git status --porcelain` — the arbiter trigger for
	// multi-commit decomposition (PRD §13.6.5: "After the loop, if `git status --porcelain` is non-empty
	// (some changes were not claimed by any stager), the arbiter runs … If `git status --porcelain` is
	// empty after the loop, the arbiter does not run — the perfect run."). The caller — the decompose
	// orchestrator (P3) — checks `output != ""` to decide whether to invoke the arbiter; an empty string
	// means a clean tree (the perfect run). It is read-only with respect to refs and the index (PRD §18.1).
	//
	// `git status --porcelain` exits 0 on success whether the tree is clean or dirty, born or unborn (it
	// lists each changed path with a 2-char "XY" status code; untracked files appear as "??"). Exit 128
	// means a non-repo or corrupt repo — a REAL error, surfaced as a non-nil err (NOT an "unborn" signal:
	// unlike rev-parse HEAD, porcelain works on unborn repos, so there is no 128-as-non-error convention
	// here — branch on code != 0, never on code == 128). Each line is "XY <path>"; the raw string is
	// returned trimmed (caller compares to "").
	StatusPorcelain(ctx context.Context) (output string, err error)

	// WorkingTreeDiff returns the unstaged working-tree diff payload for multi-commit decomposition's
	// planner input (PRD §13.6.2 / FR-M3: the planner "Receives the full working-tree diff snapshot
	// (with binary placeholders per FR3c) plus the style examples from §9.3"). It is the working-tree
	// analogue of StagedDiff (which is index-vs-HEAD) and the no-tree analogue of TreeDiff (which is
	// tree-to-tree): the SAME three-part payload (markdown per-file + line-capped; FR3c binary
	// placeholders; non-markdown aggregate + byte-capped) with the SAME pathspec excludes — the ONLY
	// difference is the diff domain: it runs `git diff` WITHOUT --cached (working-tree-vs-INDEX), never
	// `git diff --cached` and never `git diff HEAD`.
	//
	// IMPORTANT — the `git diff` domain omits untracked files: `git diff` (no --cached) compares the
	// working tree to the INDEX, and git never lists untracked files in a diff (untracked = not in the
	// index = nothing to diff against). Only tracked-but-modified and tracked-but-deleted files appear.
	// This is the explicit contract (the work item names `git diff` WITHOUT --cached); the tooled stager
	// (FR-M5) discovers untracked files itself. Callers must not expect untracked files in this payload.
	//
	// `git diff` (without --quiet) exits 0 whether or not there are changes (empty working tree → exit 0,
	// empty stdout; dirty → exit 0, non-empty stdout); exit 128 means a bad pathspec or corrupt repo — a
	// REAL error (NOT an unborn signal: branch on code != 0, never on code == 128; never use --quiet).
	// Read-only with respect to refs and the index (PRD §18.1). A no-change working tree returns ("", nil).
	WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions) (diff string, err error)

	// LogRange returns the commits in the range baseSHA..HEAD, oldest-first, as []LogEntry. It runs
	// `git log --reverse --format=%H%x1f%s baseSHA..HEAD`: --reverse yields oldest-first, %H is the
	// full SHA, %x1f (ASCII Unit Separator) delimits SHA from subject (a safe delimiter — subjects
	// never contain \x1f, and %s is single-line by construction), and %s is the subject (first line).
	// It is read-only with respect to refs and the index (PRD §18.1).
	//
	// baseSHA is the pre-run HEAD captured before the decompose loop/arbiter mutated it. The range
	// `baseSHA..HEAD` is "commits reachable from HEAD but not from baseSHA" — i.e. exactly the commits
	// created/rewritten this run. An empty range (baseSHA == HEAD) returns (nil, nil).
	//
	// Originally-unborn repo: pass the all-zeros SHA strings.Repeat("0", 40) as baseSHA. The
	// `<zeros>..HEAD` range is INVALID (git rejects it as "Invalid revision range"), so LogRange
	// detects the all-zeros sentinel and runs `git log --reverse … HEAD` (NO range) instead — listing
	// ALL commits reachable from HEAD, which on an originally-unborn repo are exactly the commits
	// created this run. (A real all-zeros ref is never a valid git range base.)
	//
	// Truly-unborn repo (HEAD has no commits): git exits 128 ("ambiguous argument 'HEAD'"); LogRange
	// returns (nil, nil) — the 128-as-non-error convention shared with RevParseHEAD / RecentSubjects /
	// CommitCount. Any other non-zero exit is a real error.
	LogRange(ctx context.Context, baseSHA string) ([]LogEntry, error)
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
		// PRD §13.5: when write-tree fails on an unmerged index, return a single clean line instead of
		// dumping git's raw multi-line stderr. Probe `git ls-files -u` (lists unmerged stage entries);
		// non-empty stdout ⇒ unresolved conflicts. Failure path only (not hot); on any ls-files error
		// fall through to the detailed diagnostic so a genuine non-conflict failure isn't hidden.
		if lsOut, _, _, lsErr := g.run(ctx, g.workDir, "ls-files", "-u"); lsErr == nil && strings.TrimSpace(lsOut) != "" {
			return "", errors.New("unresolved merge conflicts in the index — resolve them first, then re-run stagehand")
		}
		return "", fmt.Errorf("git write-tree failed (exit %d): %s", code, strings.TrimSpace(stderr))
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

// commit-pi defaults for staged-diff capture (PRD §9.1 / FINDING 7). Applied when the caller passes
// a zero/negative cap in StagedDiffOptions — guaranteeing commit-pi parity for any caller (the
// config system P1.M1.T4 populates these from resolved config; here we enforce the floor).
const (
	defaultMaxMDLines   = 100    // per-file line cap for markdown (commit-pi default)
	defaultMaxDiffBytes = 300000 // byte cap on the non-markdown aggregate (commit-pi default)
)

// maxRecentMessageLines is the total line cap across the style-example block (PRD §9.3/FR11).
// RecentMessages stops before exceeding this limit, keeping COMPLETE messages only (D4).
const maxRecentMessageLines = 100

// defaultExcludes is the commit-pi noise-filter pathspec set (lock files, snapshots, sourcemaps,
// vendored code). Used when StagedDiffOptions.Excludes is empty; a non-empty opts.Excludes REPLACES
// it. The structural markdown excludes (":!*.md", ":!*.markdown") are appended SEPARATELY in the
// non-markdown section (always, regardless of opts.Excludes) because markdown is captured per-file in
// Part 1 — omitting them would duplicate markdown in the payload (the double-count trap, G1).
// EmptyTreeSHA is git's well-known empty-tree object name. It is a valid `git diff` tree arg and is used
// as tree[-1] (treeA) for the unborn-repo base case of the multi-commit concept-diff loop (PRD §13.6.3:
// "tree[-1] is the original parent tree, or the empty tree for an unborn repo"). The decompose
// orchestrator (P3) passes it as treeA when RevParseTree returns "" on an unborn repo. TreeDiff itself
// treats both args as opaque tree SHAs and is NOT unborn-aware.
const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

var defaultExcludes = []string{
	":!*.lock", ":!package-lock.json", ":!pnpm-lock.yaml", ":!yarn.lock",
	":!*.snap", ":!*.map", ":!vendor/*",
}

// StagedDiff returns the concatenated staged-diff payload for prompt construction and stdin delivery
// to the agent CLI (PRD §9.1/FR1–FR4, Appendix C, FINDING 7). It mirrors commit-pi's two-part
// capture:
//
//  1. Markdown files (.md, .markdown): `git diff --cached --name-only -- '*.md' '*.markdown'` lists
//     them (git pathspec globs, NOT shell globs — passed as []string), then each is diffed
//     individually (`git diff --cached -- '<file>'`) and capped at max_md_lines lines (split on
//     "\n", take the first N). A per-file truncation sentinel marks over-cap files so the model knows
//     the diff is partial.
//  2. Non-markdown files: a single `git diff --cached -- <excludes>` with pathspec magic-prefix
//     excludes for lock/snapshot/sourcemap/vendor noise (defaultExcludes, overridable via
//     opts.Excludes) PLUS the structural markdown excludes (":!*.md", ":!*.markdown") so markdown is
//     not double-counted (verified: without them markdown appears in BOTH sections). The aggregate is
//     capped at max_diff_bytes bytes.
//
// Caps are POST-capture (FINDING 7: git has no --max-bytes/--max-lines). Zero/negative caps apply the
// commit-pi defaults (100/300000). The two parts are concatenated (markdown first). An empty repo
// (nothing staged) yields "" with no error — the caller gates on HasStagedChanges first, but
// StagedDiff is safe to call unconditionally.
//
// `git diff` WITHOUT --quiet exits 0 on success whether or not there are changes (distinct from
// HasStagedChanges' --quiet exit-1-means-staged); a non-zero exit (128) is a real error (bad
// pathspec, corrupt repo) and is surfaced as a wrapped error.
func (g *gitRunner) StagedDiff(ctx context.Context, opts StagedDiffOptions) (string, error) {
	maxMDLines := opts.MaxMDLines
	if maxMDLines <= 0 {
		maxMDLines = defaultMaxMDLines
	}
	maxDiffBytes := opts.MaxDiffBytes
	if maxDiffBytes <= 0 {
		maxDiffBytes = defaultMaxDiffBytes
	}

	var b strings.Builder

	// ---- Part 1: markdown, per-file, line-capped ----
	// "*.md" / "*.markdown" are git pathspec globs (interpreted by git, not the shell — G10); the "--"
	// guards pathspec-like filenames (G11).
	mdList, stderr, code, err := g.run(ctx, g.workDir,
		"diff", "--cached", "--name-only", "--", "*.md", "*.markdown")
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return "", fmt.Errorf("git diff (markdown list): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	for _, file := range strings.Split(strings.TrimSpace(mdList), "\n") {
		if file == "" {
			continue // nothing-staged ⇒ mdList is "" ⇒ Split yields [""] ⇒ skipped (G15)
		}
		fileDiff, fstderr, fcode, ferr := g.run(ctx, g.workDir, "diff", "--cached", "--", file)
		if ferr != nil {
			return "", ferr
		}
		if fcode != 0 {
			return "", fmt.Errorf("git diff --cached -- %s: failed (exit %d): %s", file, fcode, strings.TrimSpace(fstderr))
		}
		// Per-file line cap (post-capture, FINDING 7/G3). Split on "\n", keep first maxMDLines.
		if lines := strings.Split(fileDiff, "\n"); len(lines) > maxMDLines {
			fileDiff = strings.Join(lines[:maxMDLines], "\n") +
				fmt.Sprintf("\n... [diff truncated at %d lines]", maxMDLines)
		}
		b.WriteString(fileDiff)
		if !strings.HasSuffix(fileDiff, "\n") {
			b.WriteByte('\n') // ensure a clean boundary before the next hunk / Part 2
		}
	}

	// ---- Binary filtering (PRD §9.1 FR3a/b/c, staged path) ----
	// detectBinaryFiles applies numstat + the BUILT-IN denylist (S1 hardcodes nil extraExts); supplement
	// with the user's BinaryExtensions below. Key off fileStatuses (destination paths) to reconcile renames
	// (numstat `old => new` vs name-status `new` — findings §4).
	binSet, berr := g.detectBinaryFiles(ctx, "--cached")
	if berr != nil {
		return "", berr
	}
	statuses, serr := g.fileStatuses(ctx, "--cached")
	if serr != nil {
		return "", serr
	}

	// Collect binary paths (FR3a union: detected-by-numstat/denylist OR matched by user BinaryExtensions),
	// SORT for deterministic output, emit FR3b placeholders, and gather pathspec excludes.
	binPaths := make([]string, 0, len(statuses))
	for path := range statuses {
		if binSet[path] || isBinaryByExtension(path, opts.BinaryExtensions) {
			binPaths = append(binPaths, path)
		}
	}
	sort.Strings(binPaths)
	var binExcludes []string // SEPARATE slice — never append to `excludes` (it may alias defaultExcludes)
	for _, path := range binPaths {
		b.WriteString(binaryPlaceholderLine(statuses[path], path)) // "<status>\t[binary] <path>"
		b.WriteByte('\n')
		binExcludes = append(binExcludes, ":!"+path)
	}

	// ---- Part 2: non-markdown, aggregate, byte-capped, excluded ----
	// opts.Excludes REPLACES the noise-filter default if non-empty (G6); the markdown excludes are
	// ALWAYS appended (structural — prevents the double-count trap, G1).
	excludes := opts.Excludes
	if len(excludes) == 0 {
		excludes = defaultExcludes
	}
	nmArgs := []string{"diff", "--cached", "--"}
	nmArgs = append(nmArgs, excludes...)
	nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
	nmArgs = append(nmArgs, binExcludes...) // drop binary bodies from the aggregate
	nmDiff, nmstderr, nmcode, nmerr := g.run(ctx, g.workDir, nmArgs...)
	if nmerr != nil {
		return "", nmerr
	}
	if nmcode != 0 {
		return "", fmt.Errorf("git diff (non-markdown): failed (exit %d): %s", nmcode, strings.TrimSpace(nmstderr))
	}
	// Byte cap (post-capture, FINDING 7/G3). len() is byte length; the slice may split a UTF-8 rune —
	// matches `head -c` (G3). The sentinel is appended AFTER the cap and is not counted against it.
	if len(nmDiff) > maxDiffBytes {
		nmDiff = nmDiff[:maxDiffBytes] +
			fmt.Sprintf("\n... [diff truncated at %d bytes]", maxDiffBytes)
	}
	b.WriteString(nmDiff)

	return b.String(), nil
}

// HasStagedChanges reports whether the index differs from HEAD (PRD §9.4/FR16–FR17, FINDING 6). It
// runs `git diff --cached --quiet`, which produces NO output (--quiet disables it) and encodes the
// answer in the exit code. The semantics are INVERTED from the usual convention and must be read
// explicitly: exit 0 → nothing staged (index == HEAD); exit 1 → staged changes EXIST (this is the
// "has staged" signal, NOT an error); any other exit (e.g. 128 corrupt repo, 129 not-a-repo) → a
// real error. A naive `err != nil` check would misread exit 1 as an error — this method is the
// structural encoding of the inversion into a typed bool so no downstream caller can get it wrong.
//
// It is read-only with respect to refs and the index (PRD §18.1): it mutates nothing. The orchestrator
// (P1.M3.T4) calls it as the pre-generation gate and again after auto-stage-all (FINDING 11); the
// CLI uses it to drive the exit-2 "nothing to commit" path (PRD §15.4).
func (g *gitRunner) HasStagedChanges(ctx context.Context) (bool, error) {
	_, stderr, code, err := g.run(ctx, g.workDir, "diff", "--cached", "--quiet")
	if err != nil {
		return false, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	switch code {
	case 0:
		return false, nil // nothing staged (index == HEAD)
	case 1:
		return true, nil // staged changes exist — exit 1 is the signal, NOT an error (FINDING 6)
	default:
		msg := strings.TrimSpace(stderr)
		if code == 129 && strings.Contains(msg, "not a git repository") {
			return false, fmt.Errorf("not a git repository (or any of the parent directories): .git")
		}
		return false, fmt.Errorf("git diff --cached --quiet: failed (exit %d): %s", code, msg)
	}
}

// RecentMessages returns up to n most-recent FULL commit messages (PRD §9.3/FR11, §17.1) for the
// mature-repo prompt builder's style-example block (P1.M3.T1.S1). It runs
// `git log --format=%x00%B -<n>`, which emits a NUL byte BEFORE each commit body — a delimiter that
// CANNOT collide with commit message text (FINDING 9: commit-pi's `---%n%B` split on `---` broke on
// markdown horizontal rules inside bodies; %x00 cannot occur in object content, verified). The output
// is split on "\x00", each part is trimmed, empties (including the leading pre-first-NUL element) are
// dropped, and the TOTAL line count is capped at maxRecentMessageLines (100, PRD FR11) keeping COMPLETE
// messages only (partial style examples would mislead the model — no truncation sentinel is appended).
// Git log returns newest-first, so the slice is newest-first. It is read-only (PRD §18.1).
//
// On an unborn repo (zero commits) git log exits 128; RecentMessages returns (nil, nil) defensively
// (callers gate on RevParseHEAD/CommitCount and take the new-repo fallback path when empty). Requesting
// more messages than exist is NOT an error — git returns only what is available.
func (g *gitRunner) RecentMessages(ctx context.Context, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil // defensive guard (D7): caller passes 20 (PRD FR11); avoids undefined `git log -0`
	}
	stdout, stderr, code, err := g.run(ctx, g.workDir, "log", "--format=%x00%B", fmt.Sprintf("-%d", n))
	if err != nil {
		return nil, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code == 128 {
		return nil, nil // unborn repo — no messages; defensive (callers gate on RevParseHEAD/CommitCount)
	}
	if code != 0 {
		return nil, fmt.Errorf("git log: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}

	var messages []string
	totalLines := 0
	for _, part := range strings.Split(stdout, "\x00") {
		msg := strings.TrimSpace(part)
		if msg == "" {
			continue // leading pre-first-NUL element, or a genuinely-empty message
		}
		lines := strings.Count(msg, "\n") + 1
		if totalLines+lines > maxRecentMessageLines {
			break // keep COMPLETE messages only; stop before exceeding the cap (D4)
		}
		messages = append(messages, msg)
		totalLines += lines
	}
	return messages, nil
}

// RecentSubjects returns up to n most-recent commit SUBJECTS (the first line of each commit message)
// for duplicate detection (PRD §9.7/FR31). The dedupe loop (P1.M3.T2) builds a set/map from these for
// O(1) exact-match lookup against a freshly-generated subject (FR32's "if the subject exactly matches
// one of the 50, retry"). It runs `git log --format=%s -<n>`, which emits EXACTLY ONE LINE per commit:
// git's %s placeholder is the subject (first line) by definition and CANNOT contain a newline, so the
// records are safely newline-delimited.
//
// NOTE — why a simple "\n" split is correct here (and NOT the %x00 NUL split that RecentMessages
// uses): FINDING 9's NUL delimiter exists to disambiguate %B FULL BODIES, where a commit body may
// contain a markdown horizontal rule "---" that a naive "---"/"\n" split would fracture. Subjects
// (%s) are single-line by construction — no embedded newline is possible, and a "---" inside a
// subject stays confined to its own line (it cannot start a new record). So splitting on "\n" is both
// safe and simpler. There is NO line cap (unlike RecentMessages): each subject is exactly one line
// and the caller bounds the count (PRD FR31: n=50), so the result is at most n short lines — no
// prompt-budget risk.
//
// On an unborn repo (zero commits) git log exits 128; RecentSubjects returns (nil, nil) defensively
// (callers gate on RevParseHEAD/CommitCount; on a new repo the duplicate check is vacuous — there is
// nothing to duplicate). Requesting more subjects than exist is NOT an error — git returns only what
// is available. It is read-only with respect to refs/index (PRD §18.1).
func (g *gitRunner) RecentSubjects(ctx context.Context, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil // defensive guard (D3): caller passes 50 (PRD FR31); avoids undefined `git log -0`
	}
	stdout, stderr, code, err := g.run(ctx, g.workDir, "log", "--format=%s", fmt.Sprintf("-%d", n))
	if err != nil {
		return nil, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code == 128 {
		return nil, nil // unborn repo (zero commits) — exit-code signal, NOT an error (matches RevParseHEAD S2 / RecentMessages T3.S3)
	}
	if code != 0 {
		return nil, fmt.Errorf("git log: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}

	var subjects []string
	for _, line := range strings.Split(stdout, "\n") {
		s := strings.TrimSpace(line)
		if s == "" {
			continue // trailing newline → trailing empty element; also any genuinely-empty subject
		}
		subjects = append(subjects, s)
	}
	return subjects, nil
}

// CommitCount returns the number of commits reachable from HEAD (PRD §9.3/FR10). It decides the
// mature-repo (>1 commit) vs new-repo (≤1 commit) prompt branch (PRD §17.1 vs §17.2). It runs
// `git rev-list --count HEAD`, which prints a single integer on success (exit 0) and exits 128 on an
// unborn repo (zero commits — the SAME exit-code signal RevParseHEAD S2 uses for isUnborn). On unborn
// it returns (0, nil) per contract; callers SHOULD but need not short-circuit via RevParseHEAD first
// (the method is safe to call on unborn). It is read-only with respect to refs/index (PRD §18.1).
//
// Note (FINDING-adjacent): a non-repo directory ALSO exits 128 ("fatal: not a git repository") and is
// therefore indistinguishable from unborn at this layer — inherited from RevParseHEAD's exit-128
// semantic and acceptable (callers gate on RevParseHEAD; a non-repo never reaches here in the happy
// path). Any other non-zero exit (not 0, not 128) is a real error.
func (g *gitRunner) CommitCount(ctx context.Context) (int, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "rev-list", "--count", "HEAD")
	if err != nil {
		return 0, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code == 128 {
		return 0, nil // unborn repo (zero commits) — exit-code signal, NOT an error (matches RevParseHEAD S2)
	}
	if code != 0 {
		return 0, fmt.Errorf("git rev-list --count HEAD: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	n, perr := strconv.Atoi(strings.TrimSpace(stdout))
	if perr != nil {
		return 0, fmt.Errorf("git rev-list --count HEAD: unparseable output %q: %w", stdout, perr)
	}
	return n, nil
}

// LogRange returns the commits in the range baseSHA..HEAD, oldest-first (see the interface doc).
// Implementation: `git log --reverse --format=%H%x1f%s <baseSHA>..HEAD`, parsed one LogEntry per line.
// For the all-zeros unborn sentinel the `<zeros>..HEAD` range is invalid, so it runs the no-range
// `... HEAD` form instead (all commits reachable from HEAD).
func (g *gitRunner) LogRange(ctx context.Context, baseSHA string) ([]LogEntry, error) {
	args := []string{"log", "--reverse", "--format=%H%x1f%s"}
	if baseSHA == strings.Repeat("0", 40) {
		// Originally-unborn base: the all-zeros ref is NOT a valid `git log` range base (git rejects
		// `<zeros>..HEAD` as "Invalid revision range"). List ALL commits reachable from HEAD instead —
		// on an originally-unborn repo these are exactly the commits created this run.
		args = append(args, "HEAD")
	} else {
		args = append(args, baseSHA+"..HEAD")
	}

	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code == 128 {
		return nil, nil // truly-unborn repo (no commits on HEAD) — 128-as-non-error (matches RevParseHEAD/RecentSubjects/CommitCount)
	}
	if code != 0 {
		return nil, fmt.Errorf("git log: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}

	var entries []LogEntry
	for _, line := range strings.Split(stdout, "\n") {
		if line == "" {
			continue // trailing newline → trailing empty element
		}
		sha, subject, ok := strings.Cut(line, "\x1f")
		if !ok {
			continue // defensive: skip a line lacking the %x1f delimiter
		}
		entries = append(entries, LogEntry{SHA: sha, Subject: subject})
	}
	return entries, nil
}

// AddAll stages every change in the working tree — new, modified, AND deleted files — via
// `git add -A` (PRD §9.4/FR16, FR20; FINDING 11). It is the auto-stage-all primitive the CLI default
// action (P1.M4.T1.S2) calls when nothing is staged (and `auto_stage_all` is on, default true) and that
// `--all`/`-a` (FR20) force-invokes even when something is already staged. `-A` operates on the WHOLE
// worktree (no pathspec) — it adds untracked files, updates modified ones, and removes deleted ones,
// making the index match the working tree.
//
// It MUTATES THE INDEX (writes .git/index) — this is an EXPECTED pre-commit mutation, NOT a ref change
// (PRD §18.1: refs move ONLY at UpdateRefCAS). The immutable snapshot (WriteTree) is taken AFTER AddAll,
// from the freshly-staged index, so AddAll does not threaten the snapshot-then-CAS atomicity. On a clean
// working tree `git add -A` is a safe no-op (exit 0, index unchanged).
//
// `git add -A` exits 0 on every happy path (born or unborn repo, with or without changes); a non-zero
// exit (128 on a non-repo / corrupt repo) is a real error. So — unlike the read methods that special-case
// exit 128 as "unborn is not an error" — AddAll treats ALL non-zero exits as errors (it is a mutation,
// structurally identical to WriteTree/CommitTree). It delegates to run() (not exec) and targets the repo
// via the -C flag (not cmd.Dir).
func (g *gitRunner) AddAll(ctx context.Context) error {
	_, stderr, code, err := g.run(ctx, g.workDir, "add", "-A")
	if err != nil {
		return err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return fmt.Errorf("git add -A: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return nil
}

// Add stages the given paths (modifications, additions/untracked, AND deletions) into the index
// via `git add -- <paths...>` (PRD §13.6.5 — mid-chain chain rebuild). It MUTATES THE INDEX but
// touches NO ref (PRD §18.1). It is the path-specific companion to AddAll: after ReadTree(tree[j])
// the index == tree[j] and Add(leftoverPaths) folds JUST the leftovers onto it (AddAll would
// collapse the chain to tree[N-1]). Empty paths ⇒ no-op nil. The `--` guards pathspec ambiguity.
// ALL non-zero exits are errors (the mutation convention shared with AddAll/ReadTree/WriteTree/
// CommitTree — no 128-as-non-error special-case).
func (g *gitRunner) Add(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil // no-op (contract: "add leftover paths" — empty set stages nothing)
	}
	args := make([]string, 0, 2+len(paths))
	args = append(args, "add", "--")
	args = append(args, paths...) // each path one argv element (no shell — PRD §19)
	_, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return fmt.Errorf("git add: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return nil
}

// StagedFileCount returns the number of files currently staged in the index (PRD §9.4/FR18). It is the
// `N` in the auto-stage notice "Nothing staged — staging all changes (N files)." — the CLI layer
// (P1.M4.T1.S2) calls it AFTER AddAll to report how many files auto-staging touched. It runs
// `git diff --cached --name-only`, which lists each staged path on its own line (one per file: added,
// modified, OR deleted — all count), and returns the count of non-empty lines.
//
// NOTE — why this OMITS `--quiet` (and does NOT invert exit codes like its sibling HasStagedChanges):
// HasStagedChanges (T3.S2) runs `git diff --cached --quiet`, where `--quiet` SUPPRESSES output and
// encodes the answer in the exit code (exit 1 = staged, FINDING 6). StagedFileCount needs the file LIST
// to count it, so it uses `--name-only` and OMITS `--quiet`: without `--quiet`, `git diff` exits 0
// whether or not changes exist, and `--name-only` emits the paths. Adding `--quiet` here would SUPPRESS
// the list and silently make StagedFileCount ALWAYS return 0. So StagedFileCount uses the SIMPLE branch
// form (code != 0 → error), byte-identical to StagedDiff/DiffTree — NOT HasStagedChanges' switch form.
//
// Counting splits stdout on "\n" and counts non-empty (post-TrimSpace) lines. The trailing newline after
// the last path yields a final "" element, which is dropped; empty output (nothing staged) yields count 0.
// A filename containing SPACES stays on ONE line (git does not quote spaces without -z), so the count is
// correct for the common case. A filename with an EMBEDDED NEWLINE would inflate the count — an accepted
// limitation (vanishingly rare; FR18's N is informational); the contract mandates the `wc -l` line-split
// form, NOT the NUL-delimited `-z` alternative. It is read-only with respect to refs (PRD §18.1) — it
// mutates neither the index nor HEAD.
func (g *gitRunner) StagedFileCount(ctx context.Context) (int, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "diff", "--cached", "--name-only")
	if err != nil {
		return 0, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		// NOTE: a NON-REPO directory exits 129 here (git falls into --no-index mode; --cached invalid),
		// NOT 128 — but we branch on code != 0, not on a specific code (G5). 128 (corrupt) and 129 (non-repo)
		// are both real errors. Do NOT add --quiet (G2): it would suppress stdout and break the count.
		return 0, fmt.Errorf("git diff --cached --name-only: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	count := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) != "" {
			count++ // trailing newline → final "" element is skipped; empty output → count 0
		}
	}
	return count, nil
}

// RevParseTree returns the tree SHA of a commit-ish (ref = "HEAD", a branch, or a commit SHA) via
// `git rev-parse <ref>^{tree}`. The `^{tree}` suffix is passed as ONE argv element (run() takes
// args ...string and builds one exec.CommandContext argv; no shell — PRD §19). On an unborn repo with
// ref="HEAD" (or an unresolvable ref) git exits 128; RevParseTree returns ("", nil) defensively
// (callers gate on RevParseHEAD's isUnborn — the empty return is the unborn/empty-tree base case). This
// mirrors RevParseHEAD's 128 convention exactly: branch on the exit CODE, NOT on stdout (git prints the
// literal argument to stdout on exit 128). Producer of tree[-1] for the multi-commit concept-diff loop
// (PRD §13.6.3).
func (g *gitRunner) RevParseTree(ctx context.Context, ref string) (string, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "rev-parse", ref+"^{tree}")
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
	}
	if code == 128 {
		return "", nil // unborn repo / unresolvable ref — defensive (callers gate on isUnborn). Branch on CODE.
	}
	if code != 0 {
		return "", fmt.Errorf("git rev-parse %s^{tree}: failed (exit %d): %s", ref, code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

// ReadTree REPLACES the index with <tree>'s contents via `git read-tree <tree>`. It MUTATES THE INDEX
// (writes .git/index) but touches NEITHER HEAD NOR any ref — refs move ONLY at UpdateRefCAS (PRD §18.1).
// Consumed ONLY by the arbiter's mid-chain chain rebuild (PRD §13.6.5). Because it is a mutation, EVERY
// non-zero exit (128 = bad/unresolvable tree SHA, not-a-repo, corrupt) is a real error — the mutation
// convention shared with AddAll / WriteTree / CommitTree (no 128-as-non-error special-case). read-tree
// prints nothing to stdout on success, so stdout is discarded.
func (g *gitRunner) ReadTree(ctx context.Context, tree string) error {
	_, stderr, code, err := g.run(ctx, g.workDir, "read-tree", tree) // stdout unused (read-tree prints nothing)
	if err != nil {
		return err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
	}
	if code != 0 {
		// ALL non-zero exits are errors (mutation convention — like AddAll). NO 128 special-case.
		return fmt.Errorf("git read-tree: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return nil
}

// TreeDiff returns the concept diff between two tree SHAs (PRD §13.6.3 invariant 2). It is a port of
// StagedDiff: the same two-part payload (markdown per-file, line-capped; non-markdown aggregate,
// byte-capped) with the same pathspec excludes and FR3c binary filtering — the ONLY difference is the
// diff domain (`git diff <treeA> <treeB>` instead of `git diff --cached`). Every `git diff` invocation
// uses the simple exit-code branch (code != 0 → error); exit 128 = a bad/unresolvable tree SHA = a real
// error (NOT an unborn signal — the caller resolves trees and passes EmptyTreeSHA for the unborn base).
func (g *gitRunner) TreeDiff(ctx context.Context, treeA, treeB string, opts StagedDiffOptions) (string, error) {
	maxMDLines := opts.MaxMDLines
	if maxMDLines <= 0 {
		maxMDLines = defaultMaxMDLines
	}
	maxDiffBytes := opts.MaxDiffBytes
	if maxDiffBytes <= 0 {
		maxDiffBytes = defaultMaxDiffBytes
	}

	var b strings.Builder

	// ---- Part 1: markdown, per-file, line-capped ----
	mdList, stderr, code, err := g.run(ctx, g.workDir,
		"diff", treeA, treeB, "--name-only", "--", "*.md", "*.markdown")
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("git diff tree-to-tree (markdown list): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	for _, file := range strings.Split(strings.TrimSpace(mdList), "\n") {
		if file == "" {
			continue
		}
		fileDiff, fstderr, fcode, ferr := g.run(ctx, g.workDir, "diff", treeA, treeB, "--", file)
		if ferr != nil {
			return "", ferr
		}
		if fcode != 0 {
			return "", fmt.Errorf("git diff %s %s -- %s: failed (exit %d): %s", treeA, treeB, file, fcode, strings.TrimSpace(fstderr))
		}
		if lines := strings.Split(fileDiff, "\n"); len(lines) > maxMDLines {
			fileDiff = strings.Join(lines[:maxMDLines], "\n") +
				fmt.Sprintf("\n... [diff truncated at %d lines]", maxMDLines)
		}
		b.WriteString(fileDiff)
		if !strings.HasSuffix(fileDiff, "\n") {
			b.WriteByte('\n')
		}
	}

	// ---- Binary filtering (PRD §9.1 FR3a/b/c, tree-to-tree path) ----
	binSet, berr := g.detectBinaryFiles(ctx, treeA, treeB)
	if berr != nil {
		return "", berr
	}
	statuses, serr := g.fileStatuses(ctx, treeA, treeB)
	if serr != nil {
		return "", serr
	}

	binPaths := make([]string, 0, len(statuses))
	for path := range statuses {
		if binSet[path] || isBinaryByExtension(path, opts.BinaryExtensions) {
			binPaths = append(binPaths, path)
		}
	}
	sort.Strings(binPaths)
	var binExcludes []string
	for _, path := range binPaths {
		b.WriteString(binaryPlaceholderLine(statuses[path], path))
		b.WriteByte('\n')
		binExcludes = append(binExcludes, ":!"+path)
	}

	// ---- Part 2: non-markdown, aggregate, byte-capped, excluded ----
	excludes := opts.Excludes
	if len(excludes) == 0 {
		excludes = defaultExcludes
	}
	nmArgs := []string{"diff", treeA, treeB, "--"}
	nmArgs = append(nmArgs, excludes...)
	nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
	nmArgs = append(nmArgs, binExcludes...)
	nmDiff, nmstderr, nmcode, nmerr := g.run(ctx, g.workDir, nmArgs...)
	if nmerr != nil {
		return "", nmerr
	}
	if nmcode != 0 {
		return "", fmt.Errorf("git diff tree-to-tree (non-markdown): failed (exit %d): %s", nmcode, strings.TrimSpace(nmstderr))
	}
	if len(nmDiff) > maxDiffBytes {
		nmDiff = nmDiff[:maxDiffBytes] +
			fmt.Sprintf("\n... [diff truncated at %d bytes]", maxDiffBytes)
	}
	b.WriteString(nmDiff)

	return b.String(), nil
}

// StatusPorcelain returns the output of `git status --porcelain` (PRD §13.6.5 arbiter trigger). It is a
// port of StagedFileCount: the SAME simple two-branch structure (err-first infrastructural-failure
// propagation, then code != 0 → error), with the command swapped to `status --porcelain` and the count
// loop DROPPED (the caller only checks emptiness, so there is nothing to parse — return the trimmed
// stdout as-is). Read-only. NO 128-as-non-error special-case (porcelain exits 0 on unborn repos).
func (g *gitRunner) StatusPorcelain(ctx context.Context) (string, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "status", "--porcelain")
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
	}
	if code != 0 {
		// ALL non-zero exits are errors (128 = non-repo/corrupt). NO 128-as-non-error special-case —
		// `git status --porcelain` exits 0 on unborn repos, so a 128 here is a real caller error.
		return "", fmt.Errorf("git status --porcelain: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

// WorkingTreeDiff returns the unstaged working-tree diff payload (PRD §13.6.2 / FR-M3 — the
// planner input). It is a port of TreeDiff: the same three-part structure (markdown per-file
// line-capped; FR3c binary placeholders; non-markdown aggregate byte-capped) with the SAME
// pathspec excludes — the ONLY difference is the diff domain: it runs `git diff` WITHOUT --cached
// (working-tree-vs-index), never `git diff --cached` and never `git diff HEAD`. Every `git diff`
// invocation uses the simple exit-code branch (code != 0 → error); exit 128 = a bad pathspec or
// corrupt repo = a real error (NOT an unborn signal). Empty diffArgs (no tree positionals).
func (g *gitRunner) WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions) (string, error) {
	maxMDLines := opts.MaxMDLines
	if maxMDLines <= 0 {
		maxMDLines = defaultMaxMDLines
	}
	maxDiffBytes := opts.MaxDiffBytes
	if maxDiffBytes <= 0 {
		maxDiffBytes = defaultMaxDiffBytes
	}

	var b strings.Builder

	// ---- Part 1: markdown, per-file, line-capped ---- (working-tree domain: no --cached, no tree args)
	mdList, stderr, code, err := g.run(ctx, g.workDir,
		"diff", "--name-only", "--", "*.md", "*.markdown")
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("git diff (markdown list): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	for _, file := range strings.Split(strings.TrimSpace(mdList), "\n") {
		if file == "" {
			continue
		}
		fileDiff, fstderr, fcode, ferr := g.run(ctx, g.workDir, "diff", "--", file)
		if ferr != nil {
			return "", ferr
		}
		if fcode != 0 {
			return "", fmt.Errorf("git diff -- %s: failed (exit %d): %s", file, fcode, strings.TrimSpace(fstderr))
		}
		if lines := strings.Split(fileDiff, "\n"); len(lines) > maxMDLines {
			fileDiff = strings.Join(lines[:maxMDLines], "\n") +
				fmt.Sprintf("\n... [diff truncated at %d lines]", maxMDLines)
		}
		b.WriteString(fileDiff)
		if !strings.HasSuffix(fileDiff, "\n") {
			b.WriteByte('\n')
		}
	}

	// ---- Binary filtering (PRD §9.1 FR3a/b/c, working-tree path) ----
	// Empty diffArgs ⇒ `git diff --numstat` / `git diff --name-status` (working-tree-vs-index).
	binSet, berr := g.detectBinaryFiles(ctx)
	if berr != nil {
		return "", berr
	}
	statuses, serr := g.fileStatuses(ctx)
	if serr != nil {
		return "", serr
	}

	binPaths := make([]string, 0, len(statuses))
	for path := range statuses {
		if binSet[path] || isBinaryByExtension(path, opts.BinaryExtensions) {
			binPaths = append(binPaths, path)
		}
	}
	sort.Strings(binPaths)
	var binExcludes []string // SEPARATE slice — never append to `excludes` (it may alias defaultExcludes)
	for _, path := range binPaths {
		b.WriteString(binaryPlaceholderLine(statuses[path], path)) // "<status>\t[binary] <path>"
		b.WriteByte('\n')
		binExcludes = append(binExcludes, ":!"+path)
	}

	// ---- Part 2: non-markdown, aggregate, byte-capped, excluded ----
	excludes := opts.Excludes
	if len(excludes) == 0 {
		excludes = defaultExcludes
	}
	nmArgs := []string{"diff", "--"}
	nmArgs = append(nmArgs, excludes...)
	nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
	nmArgs = append(nmArgs, binExcludes...) // drop binary bodies from the aggregate
	nmDiff, nmstderr, nmcode, nmerr := g.run(ctx, g.workDir, nmArgs...)
	if nmerr != nil {
		return "", nmerr
	}
	if nmcode != 0 {
		return "", fmt.Errorf("git diff (non-markdown): failed (exit %d): %s", nmcode, strings.TrimSpace(nmstderr))
	}
	if len(nmDiff) > maxDiffBytes {
		nmDiff = nmDiff[:maxDiffBytes] +
			fmt.Sprintf("\n... [diff truncated at %d bytes]", maxDiffBytes)
	}
	b.WriteString(nmDiff)

	return b.String(), nil
}
