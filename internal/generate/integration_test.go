// This file is the PRD §20.1 layer-3 end-to-end integration suite
// (decisions.md §8) for the snapshot-based atomic-commit orchestrator
// generate.CommitStaged (P1.M6.T1.S1). It is white-box `package generate`
// (Decision B — see the research note P1_M6_T3_S2_research.md §2) so it can
// name the package-private gitClient/runner interfaces AND compose the
// in-package stub harness (BuildStubBinary/NewStubManifest/StubConfig/
// StubResponse from stubprovider_test.go). It wires the REAL *git.Git
// (git.New(dir)) into Deps.Git and the REAL *provider.Executor
// (provider.NewExecutor("")) into Deps.Runner with a stub Manifest — so the
// FULL real pipeline (executor execs the stub binary → stdout → parse → real
// git plumbing) is exercised end-to-end. The ONLY thing mocked is the agent
// (the stub binary, exec'd through the real *provider.Executor exactly as a
// real agent would be); the repo, the git binary, and process execution are
// all REAL.
//
// It covers all seven contract paths (PRD §18.2 failure-mode table + §13.5
// edge cases): SUCCESS, DUP-RETRY-THEN-SUCCESS, PARSE-FAIL-THEN-RESCUE,
// TIMEOUT, CAS-FAILURE (HEAD moved mid-run), ROOT-COMMIT (unborn repo), and
// NOTHING-STAGED. Every assertion is made against REAL git plumbing output
// (git cat-file/log/rev-parse/diff), proving the snapshot-based atomic-commit
// core (PRD §13 + §13.5; decisions.md §8). The suite's OUTPUT feeds the
// §18.1 invariant assertions in P1.M6.T3.S3.
//
// Dependencies: stdlib (bytes/context/errors/os/os-exec/path-filepath/
// strings/testing/time) + internal/{config,git,provider,ui} + the in-package
// stub helpers ONLY. NO testify, NO real LLM.
package generate

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/ui"
)

// Compile-time guarantee that the test-local headMoverRunner decorator
// implements the package-private runner interface (Decision C). The real
// *provider.Executor already satisfies it (see generate_test.go's
// `_ runner = (*provider.Executor)(nil)`); this decorator wraps it to move
// HEAD deterministically during the CAS test.
var _ runner = (*headMoverRunner)(nil)

// ---------------------------------------------------------------------------
// Raw-git bootstrap helpers (Decision A — the cross-package harness is
// unreachable from package generate; *git.Git exposes ONLY plumbing).
// ---------------------------------------------------------------------------

// gitRun is the fail-fast raw-git bootstrap seam (Decision A). The
// internal/git temp-repo harness (newTempRepo/writeFileStage/seedCommits in
// internal/git/gittestutil_test.go) is white-box `package git` and therefore
// UNREACHABLE from `package generate`, and *git.Git deliberately exposes ONLY
// plumbing (no init/add-file/commit — stage.go documents that staging POLICY
// is CLI-only). So this suite drives the REAL git binary directly via
// exec.Command for repo bootstrap (init, identity, write+stage, seed history),
// MIRRORING internal/git/gittestutil_test.go's conventions EXACTLY (same
// deterministic identity stagehand@example.com / "Stagehand Test", same
// repo-local commit.gpgsign=false hardening). It is NOT a re-implementation
// of plumbing: plumbing (write-tree/commit-tree/update-ref/rev-parse/
// cat-file) flows through the REAL *git.Git wired into Deps.Git and these
// raw gitRun calls used only for assertions. Accepts testing.TB (not just
// *testing.T) so the headMoverRunner decorator — which only holds a
// testing.TB — can call it to advance HEAD during Run.
func gitRun(tb testing.TB, dir string, args ...string) string {
	tb.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		tb.Fatalf("git %s: %v\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String()
}

// newTempRepo bootstraps an isolated, UNBORN git repository for one
// integration test: a fresh t.TempDir() (auto-cleaned by the testing
// package), `git init -q`, and REPO-LOCAL deterministic config
// (user.email/user.name so commits are authorable, commit.gpgsign=false as
// cross-machine hardening against hosts with global gpgsign). It mirrors
// internal/git/gittestutil_test.go's newTempRepo EXACTLY but returns the bare
// dir string (not a *git.Git): this suite drives raw git (gitRun) for
// bootstrap while plumbing goes through a separately-constructed *git.Git
// wired into Deps. The repo is returned UNBORN (no initial commit), so the
// ROOT-COMMIT test observes the unborn path; tests needing history call
// seedCommit themselves.
func newTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.email", "stagehand@example.com")
	gitRun(t, dir, "config", "user.name", "Stagehand Test")
	gitRun(t, dir, "config", "commit.gpgsign", "false") // repo-local, zero production impact
	return dir
}

// writeStage writes a file under the repo (creating parent dirs so nested
// paths like "pkg/sub/f.go" work) and stages it via the REAL `git add`,
// mirroring internal/git/gittestutil_test.go's writeFileStage. path is
// RELATIVE to dir; it is staged as-is because cmd.Dir is dir, so the index
// entry matches the on-disk path.
func writeStage(t *testing.T, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	gitRun(t, dir, "add", path)
}

// seedCommit appends ONE deterministic commit to the repo's history by
// staging a single seed.txt file (its content set to the message so repeated
// calls with distinct messages always change the tree) and committing it via
// the REAL `git commit -q -m <msg>` — the WHOLE message as ONE -m arg
// (matches seedCommits in internal/git/gittestutil_test.go, which verifies
// multi-line bodies reproduce verbatim via `git log --format=%B`). Used to
// give a repo a parent HEAD (and a recent-subject set) before a test drives
// CommitStaged.
func seedCommit(t *testing.T, dir, msg string) {
	t.Helper()
	writeStage(t, dir, "seed.txt", msg+"\n")
	gitRun(t, dir, "commit", "-q", "-m", msg)
}

// ---------------------------------------------------------------------------
// Deps wiring + the deterministic headMover runner decorator.
// ---------------------------------------------------------------------------

// e2eDeps builds a Deps wired to the REAL collaborators so the full pipeline
// (real *git.Git plumbing + real *provider.Executor exec'ing the stub binary
// + a stub Manifest + resolved config) is exercised end-to-end. A StateFile
// is ALWAYS set in its own t.TempDir() (required for multi-entry scripts —
// without it every call selects entry 0; harmless for single-entry) so
// sibling subtests never share counter state. noColor=true so out.Red is a
// no-op and captured stderr is plain for substring assertions. The CAS test
// does NOT use this helper (it wires its own headMoverRunner decorator).
func e2eDeps(t *testing.T, dir string, script []StubResponse, cfg config.Config, stdout, stderr *bytes.Buffer) Deps {
	t.Helper()
	manifest := NewStubManifest(t, StubConfig{
		Script:    script,
		StateFile: filepath.Join(t.TempDir(), "c"),
	})
	g, err := git.New(dir)
	if err != nil {
		t.Fatalf("git.New(%q): %v", dir, err)
	}
	return Deps{
		Git:      g,
		Runner:   provider.NewExecutor(""),
		Manifest: manifest,
		Config:   cfg,
		Output:   ui.NewOutput(stdout, stderr, false, true), // noColor=true ⇒ out.Red is a no-op
	}
}

// headMoverRunner is the DETERMINISTIC test-local decorator (Decision C)
// implementing the package-private runner interface. The CAS-failure contract
// is "HEAD moves mid-run → UpdateRefCAS fails → HEAD unchanged (NOT forced)
// → ErrHeadMoved". The ONLY code that executes in the
// [RevParseHEAD captures parentSHA] → [UpdateRefCAS] window is the generation
// loop (i.e. Runner.Run), so HEAD must move DURING Run. On the FIRST Run
// this decorator advances HEAD via PLUMBING on the CURRENT HEAD's tree
// (commit-tree on HEAD^{tree} + update-ref HEAD <new>), which leaves the
// INDEX/staged file UNTOUCHED (it uses HEAD's tree, NOT the index), then
// delegates Run/Parse to the real *provider.Executor.
//
// CRITICAL sub-gotcha (Decision C): it advances HEAD with plumbing on
// HEAD^{tree} — NEVER `git commit --allow-empty`, which would consume/commit
// the test's STAGED file and corrupt the index snapshot. After the move:
// parentSHA (old, captured by CommitStaged before the loop) ≠ HEAD (new) ⇒
// the real `git update-ref HEAD <stagehandSHA> <parentSHA>` CAS exits 128 ⇒
// ErrHeadMoved, HEAD stays at the decorator's commit, and the stagehand
// commit object is DANGLING (unreachable from any ref) — the REAL CAS-failure
// path, asserted with real git.
type headMoverRunner struct {
	inner runner     // the real *provider.Executor being decorated
	tb    testing.TB // for gitRun's fail-fast (Run has no *testing.T)
	dir   string     // the repo to move HEAD in
	moved bool       // advance HEAD exactly once (first Run only)
}

// Run moves HEAD on the first call (plumbing on HEAD^{tree}, never
// git commit --allow-empty) then delegates to the real executor.
func (r *headMoverRunner) Run(ctx context.Context, m provider.Manifest, model, prov, sys, payload string) (string, error) {
	if !r.moved {
		r.moved = true
		// Advance HEAD with plumbing on the CURRENT HEAD's tree (NOT the
		// index), so the staged file is untouched. commit-tree -p <parent>
		// -m <msg> <tree> builds a new child commit of HEAD; update-ref HEAD
		// <new> moves the ref. `git commit --allow-empty` would instead
		// commit the staged file and corrupt the snapshot — never used.
		tree := strings.TrimSpace(gitRun(r.tb, r.dir, "rev-parse", "HEAD^{tree}"))
		parent := strings.TrimSpace(gitRun(r.tb, r.dir, "rev-parse", "HEAD"))
		newc := strings.TrimSpace(gitRun(r.tb, r.dir, "commit-tree", "-p", parent, "-m", "concurrent commit elsewhere", tree))
		gitRun(r.tb, r.dir, "update-ref", "HEAD", newc)
	}
	return r.inner.Run(ctx, m, model, prov, sys, payload)
}

// Parse delegates to the real executor's parser (the decorator only mutates
// HEAD; parsing is the executor's concern).
func (r *headMoverRunner) Parse(raw string, m provider.Manifest) (string, bool) {
	return r.inner.Parse(raw, m)
}

// ---------------------------------------------------------------------------
// Real-git assertion helpers (no git library — raw cat-file/log plumbing).
// ---------------------------------------------------------------------------

// commitTreeLine parses the `tree <sha>` line from `git cat-file -p <sha>` (a
// commit object's raw contents). It fatals if no tree line is found (every
// commit has exactly one tree).
func commitTreeLine(t *testing.T, dir, sha string) string {
	t.Helper()
	body := gitRun(t, dir, "cat-file", "-p", sha)
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "tree ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "tree "))
		}
	}
	t.Fatalf("no tree line in commit %s:\n%s", sha, body)
	return ""
}

// commitParentLine parses the `parent <sha>` line from `git cat-file -p <sha>`,
// returning "" when the commit has NO parent line (a root commit).
func commitParentLine(t *testing.T, dir, sha string) string {
	t.Helper()
	body := gitRun(t, dir, "cat-file", "-p", sha)
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "parent ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "parent "))
		}
	}
	return "" // root commit: no parent line
}

// commitType returns `git cat-file -t <sha>` trimmed (e.g. "commit"); fatals
// if git rejects the SHA (proves the object exists + its type).
func commitType(t *testing.T, dir, sha string) string {
	t.Helper()
	return strings.TrimSpace(gitRun(t, dir, "cat-file", "-t", sha))
}

// headSHA returns the current HEAD commit SHA (trimmed), asserting the repo
// has a HEAD (a failure here means the repo was unexpectedly unborn).
func headSHA(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
}

// stagedFiles returns `git diff --cached --name-only` (the staged index
// snapshot for the §18.1 index-unchanged invariant).
func stagedFiles(t *testing.T, dir string) string {
	t.Helper()
	return gitRun(t, dir, "diff", "--cached", "--name-only")
}

// assertHeadAndIndexUnchanged asserts the §18.1 invariants for a FAILURE path
// against REAL git: HEAD (git rev-parse HEAD) and the staged index
// (git diff --cached --name-only) are byte-for-byte UNCHANGED before==after.
// beforeHead/beforeStaged are captured BEFORE CommitStaged runs.
func assertHeadAndIndexUnchanged(t *testing.T, dir, label, beforeHead, beforeStaged string) {
	t.Helper()
	if got := headSHA(t, dir); got != beforeHead {
		t.Errorf("%s: HEAD changed: before=%s after=%s (failure paths must leave HEAD unchanged)", label, beforeHead, got)
	}
	if got := stagedFiles(t, dir); got != beforeStaged {
		t.Errorf("%s: staged index changed: before=%q after=%q (failure paths must leave the index unchanged)", label, beforeStaged, got)
	}
}

// ---------------------------------------------------------------------------
// The 7 contract paths (each drives the REAL CommitStaged against a REAL temp
// repo + the REAL stub agent binary; -run Integration selects them all).
// ---------------------------------------------------------------------------

// TestIntegration_Success proves the happy path end-to-end: a unique subject
// on the first agent call yields a REAL commit whose tree is the unchanged
// index snapshot, whose parent is the pre-run HEAD, whose message is
// Result.Message, and whose FR42 success line ([<short>] <subject>) lands on
// stdout with NO rescue block on stderr.
func TestIntegration_Success(t *testing.T) {
	dir := newTempRepo(t)
	seedCommit(t, dir, "feat: baseline")
	writeStage(t, dir, "feature.go", "package x\n")
	oldHead := headSHA(t, dir)

	cfg := config.Default()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps := e2eDeps(t, dir, []StubResponse{{
		Emit: "feat: add feature module\n\nImplements the new feature.",
	}}, cfg, stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if err != nil {
		t.Fatalf("CommitStaged returned %v; want nil (success path)", err)
	}

	// The returned SHA is a REAL commit object.
	if got := commitType(t, dir, res.CommitSHA); got != "commit" {
		t.Errorf("cat-file -t = %q; want %q", got, "commit")
	}
	// Its tree line == a fresh write-tree of the (unchanged) index.
	if got := commitTreeLine(t, dir, res.CommitSHA); got != strings.TrimSpace(gitRun(t, dir, "write-tree")) {
		t.Errorf("commit tree = %q; want write-tree %q (index must be unchanged by the commit)", got, strings.TrimSpace(gitRun(t, dir, "write-tree")))
	}
	// Its parent line == the pre-run HEAD (the snapshot's fixed parent).
	if got := commitParentLine(t, dir, res.CommitSHA); got != oldHead {
		t.Errorf("commit parent = %q; want old HEAD %q", got, oldHead)
	}
	// Its message (git log %B) == Result.Message (byte-exact after trim).
	if got := strings.TrimSpace(gitRun(t, dir, "log", "-1", "--format=%B", res.CommitSHA)); got != res.Message {
		t.Errorf("commit %%B = %q; want Result.Message %q", got, res.Message)
	}
	// FR42 success print: stdout contains [<short>] <subject>.
	wantLine := "[" + res.CommitSHA[:7] + "] feat: add feature module"
	if !strings.Contains(stdout.String(), wantLine) {
		t.Errorf("stdout missing %q\n--got--\n%s", wantLine, stdout.String())
	}
	// No rescue block on the success path.
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT contain a rescue block on success\n--got--\n%s", stderr.String())
	}
}

// TestIntegration_DupRetryThenSuccess proves the OUTER duplicate-rejection
// loop end-to-end: the first agent subject duplicates a recent commit
// (rejected) and the second is unique (committed). The created commit's
// message is the unique subject and HEAD advances by exactly one commit.
func TestIntegration_DupRetryThenSuccess(t *testing.T) {
	dir := newTempRepo(t)
	seedCommit(t, dir, "feat: dup") // the recent subject the first emit duplicates
	writeStage(t, dir, "feature.go", "package x\n")
	oldHead := headSHA(t, dir)
	beforeCount := strings.TrimSpace(gitRun(t, dir, "rev-list", "--count", "HEAD"))

	cfg := config.Default()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps := e2eDeps(t, dir, []StubResponse{
		{Emit: "feat: dup"},           // duplicates the seeded subject → rejected
		{Emit: "feat: unique module"}, // unique → committed
	}, cfg, stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if err != nil {
		t.Fatalf("CommitStaged returned %v; want nil (dup-retry then success)", err)
	}
	if res.Subject != "feat: unique module" {
		t.Errorf("Result.Subject = %q; want the unique second subject", res.Subject)
	}
	// The committed message is the unique subject (byte-exact after trim).
	if got := strings.TrimSpace(gitRun(t, dir, "log", "-1", "--format=%B", res.CommitSHA)); got != "feat: unique module" {
		t.Errorf("commit %%B = %q; want %q", got, "feat: unique module")
	}
	// HEAD advanced by exactly one commit over the seeded history.
	afterCount := strings.TrimSpace(gitRun(t, dir, "rev-list", "--count", "HEAD"))
	if afterCount != "2" || beforeCount != "1" {
		t.Errorf("rev-list --count before=%s after=%s; want 1 → 2 (advanced by exactly one)", beforeCount, afterCount)
	}
	if commitParentLine(t, dir, res.CommitSHA) != oldHead {
		t.Errorf("commit parent = %q; want old HEAD %q (built on the snapshot parent)", commitParentLine(t, dir, res.CommitSHA), oldHead)
	}
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT contain a rescue block on success\n--got--\n%s", stderr.String())
	}
}

// TestIntegration_ParseFailThenRescue proves the INNER-budget-exhaustion path
// end-to-end. Decision D (research note §4): provider.parseOutput returns
// ok = (TrimSpace(msg) != "") — under RAW output a NON-EMPTY "garbage" string
// parses ok=true and would COMMIT, not rescue. So BOTH inner-try entries use
// Emit:"" (empty), the ONLY way to make Parse return ok=false here. The
// pipeline renders Rescue("") + returns ErrRescue, and HEAD + the index are
// byte-for-byte unchanged.
func TestIntegration_ParseFailThenRescue(t *testing.T) {
	dir := newTempRepo(t)
	seedCommit(t, dir, "feat: baseline")
	writeStage(t, dir, "feature.go", "package x\n")
	beforeHead, beforeStaged := headSHA(t, dir), stagedFiles(t, dir)

	cfg := config.Default()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	// EMPTY on both entries (Decision D) — non-empty would parse ok=true.
	deps := e2eDeps(t, dir, []StubResponse{{Emit: ""}, {Emit: ""}}, cfg, stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if !errors.Is(err, ErrRescue) {
		t.Fatalf("CommitStaged error = %v; want ErrRescue", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("Result.CommitSHA = %q; want empty (no commit on parse-fail)", res.CommitSHA)
	}
	if missing := containsAll(stderr.String(), "Commit generation failed", "Tree ID:"); missing != "" {
		t.Errorf("stderr missing %q (the §18.3 rescue block)\n--got--\n%s", missing, stderr.String())
	}
	// HEAD + index unchanged (§18.1 invariants, real git).
	assertHeadAndIndexUnchanged(t, dir, "parse-fail", beforeHead, beforeStaged)
}

// TestIntegration_Timeout proves the real process-group timeout path
// end-to-end. A [{Hang:true}] stub blocks forever; a short cfg.Timeout fires
// the executor's ctx deadline, which yields a *provider.TimeoutError (wraps
// context.DeadlineExceeded, NOT context.Canceled), so CommitStaged renders
// Rescue("") + returns ErrRescue. HEAD + index unchanged.
func TestIntegration_Timeout(t *testing.T) {
	dir := newTempRepo(t)
	seedCommit(t, dir, "feat: baseline")
	writeStage(t, dir, "feature.go", "package x\n")
	beforeHead, beforeStaged := headSHA(t, dir), stagedFiles(t, dir)

	cfg := config.Default()
	cfg.Timeout = 500 * time.Millisecond
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps := e2eDeps(t, dir, []StubResponse{{Hang: true}}, cfg, stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if !errors.Is(err, ErrRescue) {
		t.Fatalf("CommitStaged error = %v; want ErrRescue (timeout → rescue)", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("Result.CommitSHA = %q; want empty (no commit on timeout)", res.CommitSHA)
	}
	if missing := containsAll(stderr.String(), "Commit generation failed", "Tree ID:"); missing != "" {
		t.Errorf("stderr missing %q (the rescue block on timeout)\n--got--\n%s", missing, stderr.String())
	}
	assertHeadAndIndexUnchanged(t, dir, "timeout", beforeHead, beforeStaged)
}

// TestIntegration_HeadMovedCASFailure proves the §13.5 CAS-conflict path
// end-to-end via the deterministic headMoverRunner decorator (Decision C).
// HEAD moves DURING Runner.Run (plumbing on HEAD^{tree}, never
// git commit --allow-empty — that would corrupt the staged index), so the
// final UpdateRefCAS(expected=parentSHA) fails: CommitStaged prints the §13.5
// message + manual recovery, returns ErrHeadMoved, NEVER force-updates (HEAD
// stays at the decorator's commit, NOT the stagehand commit), NEVER rescues,
// and leaves the index untouched.
func TestIntegration_HeadMovedCASFailure(t *testing.T) {
	dir := newTempRepo(t)
	seedCommit(t, dir, "feat: baseline")
	writeStage(t, dir, "feature.go", "package x\n")
	beforeHead := headSHA(t, dir)
	beforeStaged := stagedFiles(t, dir)

	g, err := git.New(dir)
	if err != nil {
		t.Fatalf("git.New(%q): %v", dir, err)
	}
	cfg := config.Default()
	manifest := NewStubManifest(t, StubConfig{
		Script:    []StubResponse{{Emit: "feat: ok message"}},
		StateFile: filepath.Join(t.TempDir(), "c"),
	})
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps := Deps{
		Git: g,
		// Inline decorator (NOT e2eDeps' bare executor): moves HEAD once
		// during Run, then delegates to the real executor.
		Runner:   &headMoverRunner{inner: provider.NewExecutor(""), tb: t, dir: dir},
		Manifest: manifest,
		Config:   cfg,
		Output:   ui.NewOutput(stdout, stderr, false, true),
	}

	res, err := CommitStaged(context.Background(), deps)
	if !errors.Is(err, ErrHeadMoved) {
		t.Fatalf("CommitStaged error = %v; want ErrHeadMoved", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("Result.CommitSHA = %q; want empty (no commit reached HEAD)", res.CommitSHA)
	}
	// §13.5 head-moved message + manual recovery to stderr.
	if missing := containsAll(stderr.String(),
		"HEAD moved while generating",
		"aborting to avoid a non-fast-forward",
		"Your generated message was: feat: ok message",
		"git commit-tree",
		"xargs git update-ref HEAD",
	); missing != "" {
		t.Errorf("stderr missing %q (§13.5 head-moved message)\n--got--\n%s", missing, stderr.String())
	}
	// NEVER Rescue on the head-moved path (it prints its OWN message).
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT contain a rescue block on head-moved\n--got--\n%s", stderr.String())
	}
	// HEAD == the decorator's concurrent commit (NEVER force-updated to the
	// stagehand commit). HEAD moved off beforeHead and its subject is the
	// decorator's, proving the stagehand commit is NOT at HEAD.
	afterHead := headSHA(t, dir)
	if afterHead == beforeHead {
		t.Errorf("HEAD unchanged (%s); want moved by the decorator's concurrent commit", afterHead)
	}
	if subj := strings.TrimSpace(gitRun(t, dir, "log", "-1", "--format=%s", afterHead)); subj != "concurrent commit elsewhere" {
		t.Errorf("HEAD subject = %q; want the decorator's concurrent commit (NEVER the stagehand commit)", subj)
	}
	if parent := commitParentLine(t, dir, afterHead); parent != beforeHead {
		t.Errorf("HEAD parent = %q; want beforeHead %q (decorator's commit is a child of the original)", parent, beforeHead)
	}
	// The index is UNCHANGED (the decorator used HEAD^{tree}, NOT the index).
	if got := stagedFiles(t, dir); got != beforeStaged {
		t.Errorf("head-moved: staged index changed: before=%q after=%q (plumbing must not touch the index)", beforeStaged, got)
	}
	// stdout is clean (failure path).
	if stdout.Len() != 0 {
		t.Errorf("stdout must be empty on head-moved\n--got--\n%s", stdout.String())
	}
}

// TestIntegration_RootCommit proves the root-commit path (§13.5 edge case)
// end-to-end against an UNBORN repo (no seedCommit). RevParseHEAD returns
// hasParent=false, so CommitTree omits -p and UpdateRefCAS uses the 1-arg
// no-expected form. The created commit has NO parent line, its tree ==
// write-tree, and HEAD advances from unborn to the new commit.
func TestIntegration_RootCommit(t *testing.T) {
	dir := newTempRepo(t) // UNBORN — no seedCommit
	writeStage(t, dir, "README.md", "hello\n")

	cfg := config.Default()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps := e2eDeps(t, dir, []StubResponse{{Emit: "feat: initial commit"}}, cfg, stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if err != nil {
		t.Fatalf("CommitStaged returned %v; want nil (root commit succeeds)", err)
	}
	// The commit object exists and is a commit.
	if got := commitType(t, dir, res.CommitSHA); got != "commit" {
		t.Errorf("cat-file -t = %q; want %q", got, "commit")
	}
	// ROOT commit: NO parent line.
	if got := commitParentLine(t, dir, res.CommitSHA); got != "" {
		t.Errorf("root commit parent = %q; want \"\" (root commit has NO parent line)", got)
	}
	// Its tree == write-tree of the (unchanged) index.
	if got := commitTreeLine(t, dir, res.CommitSHA); got != strings.TrimSpace(gitRun(t, dir, "write-tree")) {
		t.Errorf("root commit tree = %q; want write-tree %q", got, strings.TrimSpace(gitRun(t, dir, "write-tree")))
	}
	// HEAD advanced from unborn to the new commit.
	if got := headSHA(t, dir); got != res.CommitSHA {
		t.Errorf("rev-parse HEAD = %q; want Result.CommitSHA %q (advanced from unborn)", got, res.CommitSHA)
	}
	// The committed message is Result.Message.
	if got := strings.TrimSpace(gitRun(t, dir, "log", "-1", "--format=%B", res.CommitSHA)); got != res.Message {
		t.Errorf("root commit %%B = %q; want Result.Message %q", got, res.Message)
	}
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT contain a rescue block on success\n--got--\n%s", stderr.String())
	}
}

// TestIntegration_NothingStaged proves the empty-diff gate (FR5) end-to-end.
// With NOTHING staged, StagedDiff returns "" and CommitStaged returns
// ErrNothingToCommit at the GENERATE layer WITHOUT taking a snapshot (WriteTree
// is never reached) and WITHOUT rendering Rescue. The CLI (P1.M7.T2), NOT
// generate, performs auto-stage-all — this is the v2 seam (decisions.md §1).
func TestIntegration_NothingStaged(t *testing.T) {
	dir := newTempRepo(t)
	seedCommit(t, dir, "feat: baseline")
	// Deliberately stage NOTHING (clean index).
	beforeHead, beforeStaged := headSHA(t, dir), stagedFiles(t, dir)

	cfg := config.Default()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	// Script content is irrelevant — Run is never reached on an empty diff.
	deps := e2eDeps(t, dir, []StubResponse{{Emit: "ignored"}}, cfg, stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if !errors.Is(err, ErrNothingToCommit) {
		t.Fatalf("CommitStaged error = %v; want ErrNothingToCommit", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("Result.CommitSHA = %q; want empty (no commit on nothing-staged)", res.CommitSHA)
	}
	// No rescue: no snapshot was taken (StagedDiff short-circuited before WriteTree).
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT contain a rescue block on nothing-staged (no snapshot taken)\n--got--\n%s", stderr.String())
	}
	// stdout is clean (nothing printed).
	if stdout.Len() != 0 {
		t.Errorf("stdout must be empty on nothing-to-commit\n--got--\n%s", stdout.String())
	}
	// HEAD + index unchanged — and no dangling tree created (WriteTree never ran).
	assertHeadAndIndexUnchanged(t, dir, "nothing-staged", beforeHead, beforeStaged)
	// A fresh write-tree of the (unchanged) index equals HEAD's EXISTING tree —
	// proving no NEW snapshot tree was added to the object store (the empty diff
	// short-circuited before WriteTree; the index merely reflects HEAD).
	headTree := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD^{tree}"))
	if got := strings.TrimSpace(gitRun(t, dir, "write-tree")); got != headTree {
		t.Errorf("write-tree = %q; want HEAD^{tree} %q (index must match HEAD; no snapshot taken)", got, headTree)
	}
}
