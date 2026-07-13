package hooks

// runner_test.go exercises the commit-hooks runner core (RunCommitHooks + RunPostCommit) against REAL
// git output with REAL executable shell-script hooks (write via os.WriteFile + chmod 0755). It reuses
// the temp-repo helpers defined in subset_test.go (same package, white-box). Each case targets one
// branch of the runner's decision tree (research §11):
//   - pre-commit permitted mutation → re-treed; LIVE .git/index UNTOUCHED (the FR-V3 assertion)
//   - pre-commit non-zero → *RescueError (Kind=ErrRescue, TreeSHA=snapshotTree)
//   - pre-commit sweep (new path) → ErrHookSweptConcurrentWork (errors.Is)
//   - commit-msg appends → finalMsg annotated
//   - absent pre-commit → skip (finalTree=snapshotTree)
//   - timeout → *RescueError (Cause≈DeadlineExceeded)
//   - --no-verify → pre-commit+commit-msg skip; prepare-commit-msg runs
//   - dry-run → pre-commit+post-commit skip; commit-msg runs
//   - RunPostCommit non-zero → nil returned, no undo, no abort

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/generate"
	"github.com/dabstractor/stagecoach/internal/git"
)

// --- hook-install helpers ---

// installHook writes an executable shell-script hook named <name> into repo/.git/hooks/.
func installHook(t *testing.T, repo, name, body string) {
	t.Helper()
	dir := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write hook %s: %v", name, err)
	}
}

// hookPath returns repo/.git/hooks/<name> (the default hooksDir for a temp repo).
func hookPath(repo, name string) string {
	return filepath.Join(repo, ".git", "hooks", name)
}

// --- runner test fixture ---

// primeRunnerRepo builds a repo with one committed file (a.go), returns its tree SHA + a git.Git
// bound to the repo. This is the snapshot scenario RunCommitHooks runs against.
func primeRunnerRepo(t *testing.T) (repo, snapshotTree, parentSHA string, g git.Git) {
	t.Helper()
	repo = t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package x\n\nfunc a() {}\n")
	stageFile(t, repo, "a.go")
	makeEmptyCommit(t, repo, "init")
	snapshotTree = writeTreeOf(t, repo)
	parentSHA = execGit(t, repo, "rev-parse", "HEAD")
	g = git.New(repo)
	return repo, snapshotTree, parentSHA, g
}

// liveIndexBytes reads the live .git/index for the byte-equality (untouched) assertion (FR-V3).
func liveIndexBytes(t *testing.T, repo string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, ".git", "index"))
	if err != nil {
		t.Fatalf("read live .git/index: %v", err)
	}
	return b
}

func defaultCfg() config.Config {
	c := config.Defaults()
	return c
}

// --- 1. pre-commit permitted mutation → re-treed; LIVE .git/index UNTOUCHED (FR-V3) ---

func TestRunCommitHooks_PreCommitPermittedMutation_ReTree_LiveIndexUntouched(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)

	// A formatter-style pre-commit: rewrite a.go (working tree) then `git add` it into the scoped index.
	installHook(t, repo, "pre-commit", `printf 'package x // reformatted\n\nfunc a() {}\n' > a.go && git add a.go`)

	liveBefore := liveIndexBytes(t, repo)

	cfg := defaultCfg()
	finalTree, finalMsg, err := RunCommitHooks(context.Background(), g, cfg, snapshotTree, parentSHA,
		"feat: a change", HookOpts{})
	if err != nil {
		t.Fatalf("RunCommitHooks err = %v, want nil", err)
	}
	if finalTree == snapshotTree {
		t.Errorf("finalTree = snapshotTree (no mutation); want re-treed postTree")
	}
	if finalMsg != "feat: a change" {
		t.Errorf("finalMsg = %q, want input unchanged", finalMsg)
	}

	liveAfter := liveIndexBytes(t, repo)
	if string(liveBefore) != string(liveAfter) {
		t.Errorf("LIVE .git/index CHANGED by scoped pre-commit (FR-V3 violation):\n before=%q\n after =%q",
			liveBefore, liveAfter)
	}

	// postTree must contain the reformatted a.go.
	postA, gitErr := execGit(t, repo, "show", finalTree+":a.go"), error(nil)
	if gitErr != nil {
		t.Fatalf("git show postTree:a.go: %v", gitErr)
	}
	if !strings.Contains(postA, "reformatted") {
		t.Errorf("postTree a.go = %q, want the formatter's output", postA)
	}
}

// --- 2. pre-commit exits non-zero → *RescueError (Kind=ErrRescue, TreeSHA=snapshotTree) ---

func TestRunCommitHooks_PreCommitNonZero_RescueError(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
	installHook(t, repo, "pre-commit", `echo "bad" >&2; exit 1`)

	_, _, err := RunCommitHooks(context.Background(), g, defaultCfg(), snapshotTree, parentSHA,
		"feat: a change", HookOpts{})
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *generate.RescueError", err)
	}
	if !errors.Is(re, generate.ErrRescue) {
		t.Errorf("RescueError.Kind not ErrRescue (errors.Is): %v", re.Kind)
	}
	if re.TreeSHA != snapshotTree {
		t.Errorf("RescueError.TreeSHA = %q, want %q", re.TreeSHA, snapshotTree)
	}
	if re.ParentSHA != parentSHA {
		t.Errorf("RescueError.ParentSHA = %q, want %q", re.ParentSHA, parentSHA)
	}
	if re.Candidate != "feat: a change" {
		t.Errorf("RescueError.Candidate = %q, want the message", re.Candidate)
	}
	if re.Cause == nil {
		t.Errorf("RescueError.Cause = nil, want the hook exit error")
	}
}

// --- 3. pre-commit sweeps a NEW path → ErrHookSweptConcurrentWork ---

func TestRunCommitHooks_PreCommitSweepsNewPath_HookSweptConcurrentWork(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
	// The hook stages a NEW path (b.go) not in the snapshot → 'A' status → sweep violation.
	writeFile(t, repo, "b.go", "package x\n")
	installHook(t, repo, "pre-commit", `git add b.go`)

	_, _, err := RunCommitHooks(context.Background(), g, defaultCfg(), snapshotTree, parentSHA,
		"feat: a change", HookOpts{})
	if !errors.Is(err, ErrHookSweptConcurrentWork) {
		t.Fatalf("err = %v, want errors.Is(_, ErrHookSweptConcurrentWork)", err)
	}
}

// --- 4. commit-msg appends → finalMsg annotated ---

func TestRunCommitHooks_CommitMsgAppends_Annotated(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
	// commit-msg appends a Signed-off-by trailer to the message file.
	installHook(t, repo, "commit-msg", `printf '\nSigned-off-by: Test <test@example.com>\n' >> "$1"`)

	_, finalMsg, err := RunCommitHooks(context.Background(), g, defaultCfg(), snapshotTree, parentSHA,
		"feat: a change", HookOpts{})
	if err != nil {
		t.Fatalf("RunCommitHooks err = %v, want nil", err)
	}
	if !strings.Contains(finalMsg, "Signed-off-by: Test <test@example.com>") {
		t.Errorf("finalMsg = %q, want the appended trailer", finalMsg)
	}
	if !strings.HasPrefix(finalMsg, "feat: a change") {
		t.Errorf("finalMsg = %q, want the original message preserved", finalMsg)
	}
}

// --- Issue 2: trailing newline before prepare-commit-msg (append-style hooks) ---

// TestRunCommitHooks_PrepareCommitMsg_AppendOnNewLine verifies git parity for the hook message file:
// git's strbuf_complete_line() ensures COMMIT_EDITMSG always ends with \n, so an append-style hook
// (`echo "Signed-off-by: ..." >> "$1"`) starts on a NEW line instead of being concatenated onto
// the subject. The test uses `echo` (NOT `printf '\n...'`) — echo appends WITHOUT a leading \n, which
// is what exercises the bug (printf '\n...' masks it by creating the break via its own leading \n).
func TestRunCommitHooks_PrepareCommitMsg_AppendOnNewLine(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
	// echo (NOT printf '\n...') appends WITHOUT a leading \n — exercises the trailing-newline bug.
	installHook(t, repo, "prepare-commit-msg", `echo "Signed-off-by: Dev <dev@example.com>" >> "$1"`)

	_, finalMsg, err := RunCommitHooks(context.Background(), g, defaultCfg(), snapshotTree, parentSHA,
		"feat: change", HookOpts{})
	if err != nil {
		t.Fatalf("RunCommitHooks err = %v, want nil", err)
	}
	// The Signed-off-by MUST be on a separate line (preceded by \n), NOT concatenated onto the subject.
	// The corrupted output would be `feat: changeSigned-off-by: ...` — the bare "Signed-off-by:" substring
	// IS present there, so the \n-line-boundary check is what distinguishes correct from corrupt.
	if !strings.Contains(finalMsg, "\nSigned-off-by:") {
		t.Errorf("Signed-off-by not on a separate line (Issue 2 regression); finalMsg=%q", finalMsg)
	}
	if !strings.HasPrefix(finalMsg, "feat: change") {
		t.Errorf("finalMsg = %q, want the original subject preserved", finalMsg)
	}
}

// --- 5. absent pre-commit → silent skip (finalTree=snapshotTree, no error) ---

func TestRunCommitHooks_AbsentPreCommit_Skip(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
	_ = repo
	// No hooks installed at all.

	finalTree, finalMsg, err := RunCommitHooks(context.Background(), g, defaultCfg(), snapshotTree, parentSHA,
		"feat: a change", HookOpts{})
	if err != nil {
		t.Fatalf("RunCommitHooks err = %v, want nil (absent hooks → silent skip)", err)
	}
	if finalTree != snapshotTree {
		t.Errorf("finalTree = %q, want snapshotTree %q (no pre-commit ran)", finalTree, snapshotTree)
	}
	if finalMsg != "feat: a change" {
		t.Errorf("finalMsg = %q, want the input message", finalMsg)
	}
}

// --- 6. timeout → *RescueError (Cause≈DeadlineExceeded) ---

func TestRunCommitHooks_PreCommitTimeout_RescueErrorDeadline(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
	installHook(t, repo, "pre-commit", `sleep 2; exit 0`)

	cfg := defaultCfg()
	cfg.HookTimeout = 100 * time.Millisecond // tiny timeout → the hook blows past it

	_, _, err := RunCommitHooks(context.Background(), g, cfg, snapshotTree, parentSHA,
		"feat: a change", HookOpts{})
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *generate.RescueError (timeout)", err)
	}
	if !errors.Is(re.Cause, context.DeadlineExceeded) {
		t.Errorf("RescueError.Cause = %v, want errors.Is(_, DeadlineExceeded)", re.Cause)
	}
}

// --- 7. --no-verify → pre-commit + commit-msg SKIP; prepare-commit-msg RUNS ---

func TestRunCommitHooks_NoVerify_SkipsPreCommitAndCommitMsg_PrepareRuns(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)

	// pre-commit and commit-msg would ABORT if they ran. prepare-commit-msg records that it ran.
	marker := filepath.Join(repo, "prepare-ran")
	installHook(t, repo, "pre-commit", `exit 1`) // would abort if not skipped
	installHook(t, repo, "commit-msg", `exit 1`) // would abort if not skipped
	installHook(t, repo, "prepare-commit-msg", `touch `+marker)

	cfg := defaultCfg()
	cfg.NoVerify = true

	finalTree, finalMsg, err := RunCommitHooks(context.Background(), g, cfg, snapshotTree, parentSHA,
		"feat: a change", HookOpts{})
	if err != nil {
		t.Fatalf("RunCommitHooks err = %v, want nil (--no-verify skips pre-commit + commit-msg)", err)
	}
	if finalTree != snapshotTree {
		t.Errorf("finalTree = %q, want snapshotTree (pre-commit skipped)", finalTree)
	}
	if finalMsg != "feat: a change" {
		t.Errorf("finalMsg = %q, want the input (commit-msg skipped)", finalMsg)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("prepare-commit-msg did NOT run under --no-verify (git-commit(1) parity): %v", statErr)
	}
}

// --- 8. dry-run → pre-commit + post-commit SKIP; commit-msg RUNS ---

func TestRunCommitHooks_DryRun_SkipsPreCommit_RunsCommitMsg(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)

	// pre-commit would ABORT (and mutate) if it ran. commit-msg annotates (proves it ran).
	installHook(t, repo, "pre-commit", `printf 'package x // mutated\n' > a.go && git add a.go`)
	installHook(t, repo, "commit-msg", `printf '\n[DRY-RUN-LINT]\n' >> "$1"`)

	finalTree, finalMsg, err := RunCommitHooks(context.Background(), g, defaultCfg(), snapshotTree, parentSHA,
		"feat: a change", HookOpts{DryRun: true})
	if err != nil {
		t.Fatalf("RunCommitHooks err = %v, want nil (dry-run skips pre-commit)", err)
	}
	if finalTree != snapshotTree {
		t.Errorf("finalTree = %q, want snapshotTree (pre-commit skipped under dry-run)", finalTree)
	}
	if !strings.Contains(finalMsg, "[DRY-RUN-LINT]") {
		t.Errorf("finalMsg = %q, want the commit-msg annotation (commit-msg RUNS under dry-run)", finalMsg)
	}
}

// --- 8b. dry-run skips post-commit (RunPostCommit is a no-op) ---

func TestRunPostCommit_DryRun_NoOp(t *testing.T) {
	repo, _, _, g := primeRunnerRepo(t)
	marker := filepath.Join(repo, "post-ran")
	installHook(t, repo, "post-commit", `touch `+marker)

	if err := RunPostCommit(context.Background(), g, defaultCfg(), HookOpts{DryRun: true}); err != nil {
		t.Errorf("RunPostCommit (dry-run) err = %v, want nil", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Errorf("post-commit RAN under dry-run; want skipped (FR-V8a)")
	}
}

// --- 9. RunPostCommit non-zero → nil returned, no abort ---

func TestRunPostCommit_NonZero_Disregarded(t *testing.T) {
	repo, _, _, g := primeRunnerRepo(t)
	installHook(t, repo, "post-commit", `echo "noisy" >&2; exit 1`)

	if err := RunPostCommit(context.Background(), g, defaultCfg(), HookOpts{}); err != nil {
		t.Errorf("RunPostCommit err = %v, want nil (exit code disregarded — commit already landed)", err)
	}
}

// --- 9b. RunPostCommit absent → no-op nil ---

func TestRunPostCommit_Absent_NoOp(t *testing.T) {
	repo, _, _, g := primeRunnerRepo(t)
	_ = repo
	// No post-commit installed.
	if err := RunPostCommit(context.Background(), g, defaultCfg(), HookOpts{}); err != nil {
		t.Errorf("RunPostCommit (absent) err = %v, want nil", err)
	}
}

// --- 10. prepare-commit-msg strips #-comment lines on read-back ---

func TestRunCommitHooks_PrepareCommitMsg_StripsComments(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
	// prepare-commit-msg adds git-style commented metadata to the message file.
	installHook(t, repo, "prepare-commit-msg",
		`printf '# Please enter the commit message...\n# Lines starting with %s are ignored\n' '#' >> "$1"`)

	_, finalMsg, err := RunCommitHooks(context.Background(), g, defaultCfg(), snapshotTree, parentSHA,
		"feat: a change", HookOpts{})
	if err != nil {
		t.Fatalf("RunCommitHooks err = %v, want nil", err)
	}
	for _, line := range strings.Split(finalMsg, "\n") {
		if strings.HasPrefix(line, "#") {
			t.Errorf("finalMsg contains a #-comment line (should be stripped): %q", finalMsg)
			break
		}
	}
	if !strings.Contains(finalMsg, "feat: a change") {
		t.Errorf("finalMsg = %q, want the original message preserved", finalMsg)
	}
}

// --- 11. non-executable pre-commit → silent skip (X_OK parity) ---

func TestRunCommitHooks_NonExecutablePreCommit_Skip(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
	// A pre-commit that exists but is NOT executable → silently skipped (git access(X_OK) parity).
	if err := os.WriteFile(hookPath(repo, "pre-commit"), []byte("#!/bin/sh\nexit 1\n"), 0o644); err != nil {
		t.Fatalf("write non-exec hook: %v", err)
	}

	finalTree, finalMsg, err := RunCommitHooks(context.Background(), g, defaultCfg(), snapshotTree, parentSHA,
		"feat: a change", HookOpts{})
	if err != nil {
		t.Fatalf("RunCommitHooks err = %v, want nil (non-exec hook → skip)", err)
	}
	if finalTree != snapshotTree {
		t.Errorf("finalTree = %q, want snapshotTree (non-exec pre-commit skipped)", finalTree)
	}
	if finalMsg != "feat: a change" {
		t.Errorf("finalMsg = %q, want input", finalMsg)
	}
}

// --- 12. hookExecutable unit (the X_OK parity helper) ---

func TestHookExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "exe")
	nonExe := filepath.Join(dir, "nonexe")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nonExe, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hookExecutable(exe) {
		t.Errorf("hookExecutable(%q) = false, want true (0755)", exe)
	}
	if hookExecutable(nonExe) {
		t.Errorf("hookExecutable(%q) = true, want false (0644)", nonExe)
	}
	if hookExecutable(filepath.Join(dir, "absent")) {
		t.Errorf("hookExecutable(absent) = true, want false")
	}
}

// --- 13. stripCommentLines unit (parameterized by core.commentChar; default '#') ---

func TestStripCommentLines(t *testing.T) {
	in := "feat: x\n# a comment\n\nbody line\n# another"
	want := "feat: x\n\nbody line"
	got := stripCommentLines(in, "#")
	if got != want {
		t.Errorf("stripCommentLines = %q, want %q", got, want)
	}
}

// --- 14. shouldSkipStagecoachPrepareCommitMsg — FR-V4 seam via hook.Detect ---
//
// S1 stubbed this false; S2 fills it via hook.Detect(hooksDir) == hook.StatusStagecoach. A repo whose
// prepare-commit-msg contains the stagecoach Marker ⇒ StatusStagecoach ⇒ skip; a foreign (no Marker)
// hook ⇒ StatusForeign ⇒ don't skip; no hook ⇒ StatusNone ⇒ don't skip.

func TestShouldSkipStagecoachPrepareCommitMsg_StagecoachMarker_True(t *testing.T) {
	dir := t.TempDir()
	hooks := filepath.Join(dir, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	// The Marker line baked into stagecoach's own prepare-commit-msg hook (internal/hook.Marker).
	body := "#!/bin/sh\n# stagecoach prepare-commit-msg hook v1\nexec stagecoach hook exec \"$@\"\n"
	if err := os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if !shouldSkipStagecoachPrepareCommitMsg(hooks) {
		t.Errorf("shouldSkipStagecoachPrepareCommitMsg = false for stagecoach's own hook; want true (recursion)")
	}
}

func TestShouldSkipStagecoachPrepareCommitMsg_ForeignOrAbsent_False(t *testing.T) {
	t.Run("foreign", func(t *testing.T) {
		dir := t.TempDir()
		hooks := filepath.Join(dir, "hooks")
		if err := os.MkdirAll(hooks, 0o755); err != nil {
			t.Fatal(err)
		}
		// A foreign hook (no stagecoach Marker) ⇒ StatusForeign ⇒ don't skip (it may annotate).
		body := "#!/bin/sh\necho foreign\n"
		if err := os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		if shouldSkipStagecoachPrepareCommitMsg(hooks) {
			t.Errorf("shouldSkipStagecoachPrepareCommitMsg = true for a foreign hook; want false")
		}
	})
	t.Run("absent", func(t *testing.T) {
		dir := t.TempDir() // no hooks dir at all ⇒ StatusNone ⇒ don't skip
		hooks := filepath.Join(dir, "hooks")
		if shouldSkipStagecoachPrepareCommitMsg(hooks) {
			t.Errorf("shouldSkipStagecoachPrepareCommitMsg = true when no hook is installed; want false")
		}
	})
}

// --- 15. FR-V4 contract scenarios (recursion skip / foreign annotate / absent no-op) ---

// TestRunCommitHooks_PrepareCommitMsg_StagecoachMarker_Skipped verifies FR-V4 recursion prevention:
// a prepare-commit-msg that IS stagecoach's own (the Marker line present) is SKIPPED on the plumbing
// path — the message is unchanged and the hook's mutation (which would recurse via `stagecoach hook
// exec`) did NOT run. NoVerify=true isolates prepare-commit-msg (no commit-msg).
func TestRunCommitHooks_PrepareCommitMsg_StagecoachMarker_Skipped(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
	// Install stagecoach's OWN prepare-commit-msg (Marker present) that WOULD mutate the file if run.
	installHook(t, repo, "prepare-commit-msg",
		`# stagecoach prepare-commit-msg hook v1`+"\n"+`echo 'RECURRED' >> "$1"`)
	cfg := defaultCfg()
	cfg.NoVerify = true // isolate prepare-commit-msg (skip pre-commit + commit-msg)

	finalTree, finalMsg, err := RunCommitHooks(context.Background(), g, cfg, snapshotTree, parentSHA,
		"feat: test", HookOpts{})
	if err != nil {
		t.Fatalf("expected nil err (skip), got: %v", err)
	}
	_ = finalTree
	if strings.Contains(finalMsg, "RECURRED") {
		t.Errorf("stagecoach's own prepare-commit-msg was NOT skipped (recursion): %q", finalMsg)
	}
	if finalMsg != "feat: test" {
		t.Errorf("msg changed despite skip: %q", finalMsg)
	}
}

// TestRunCommitHooks_PrepareCommitMsg_Foreign_AnnotationReadBack verifies a FOREIGN
// prepare-commit-msg's appended annotation is read back from the shared file into finalMsg.
func TestRunCommitHooks_PrepareCommitMsg_Foreign_AnnotationReadBack(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
	// A foreign prepare-commit-msg (no stagecoach Marker) that appends a ticket ref.
	installHook(t, repo, "prepare-commit-msg", `echo 'Refs: #123' >> "$1"`)
	cfg := defaultCfg()
	cfg.NoVerify = true // isolate prepare (no commit-msg)

	_, finalMsg, err := RunCommitHooks(context.Background(), g, cfg, snapshotTree, parentSHA,
		"feat: test", HookOpts{})
	if err != nil {
		t.Fatalf("expected nil err, got: %v", err)
	}
	if !strings.Contains(finalMsg, "Refs: #123") {
		t.Errorf("foreign prepare-commit-msg annotation not read back: %q", finalMsg)
	}
}

// TestRunCommitHooks_PrepareCommitMsg_Absent_NoOp verifies an absent prepare-commit-msg is a no-op
// (msg unchanged). NoVerify=true isolates the prepare stage.
func TestRunCommitHooks_PrepareCommitMsg_Absent_NoOp(t *testing.T) {
	_, snapshotTree, parentSHA, g := primeRunnerRepo(t) // no hooks installed
	cfg := defaultCfg()
	cfg.NoVerify = true

	_, finalMsg, err := RunCommitHooks(context.Background(), g, cfg, snapshotTree, parentSHA,
		"feat: test", HookOpts{})
	if err != nil {
		t.Fatalf("expected nil err, got: %v", err)
	}
	if finalMsg != "feat: test" {
		t.Errorf("absent prepare-commit-msg should be a no-op: %q", finalMsg)
	}
}

// --- 16. stripCommentLines honors the comment char (pure table) ---

func TestStripCommentLines_HonorsCommentChar(t *testing.T) {
	cases := []struct {
		name, in, char, want string
	}{
		{"hash strips # lines", "feat: x\n# comment\nbody", "#", "feat: x\nbody"},
		{"semicolon strips ; lines", "feat: x\n; comment\nbody", ";", "feat: x\nbody"},
		{"empty char defaults to hash", "feat: x\n# c\nbody", "", "feat: x\nbody"},
		{"no comment lines unchanged", "feat: x\nbody", "#", "feat: x\nbody"},
		{"multi-char prefix", "feat: x\n// note\nbody", "//", "feat: x\nbody"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripCommentLines(tc.in, tc.char); got != tc.want {
				t.Errorf("stripCommentLines(%q, %q) = %q, want %q", tc.in, tc.char, got, tc.want)
			}
		})
	}
}

// --- 17. CommentChar reads core.commentChar via the Git interface (default '#' on unset) ---

func TestGitRunner_CommentChar(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	g := git.New(repo)

	t.Run("unset defaults to hash", func(t *testing.T) {
		got, err := g.CommentChar(context.Background())
		if err != nil {
			t.Fatalf("CommentChar err = %v, want nil (unset should default)", err)
		}
		if got != "#" {
			t.Errorf("CommentChar (unset) = %q, want %q", got, "#")
		}
	})
	t.Run("explicit semicolon", func(t *testing.T) {
		execGit(t, repo, "config", "core.commentChar", ";")
		got, err := g.CommentChar(context.Background())
		if err != nil {
			t.Fatalf("CommentChar err = %v, want nil", err)
		}
		if got != ";" {
			t.Errorf("CommentChar (set ;) = %q, want %q", got, ";")
		}
	})
	t.Run("auto resolves to hash", func(t *testing.T) {
		execGit(t, repo, "config", "core.commentChar", "auto")
		got, err := g.CommentChar(context.Background())
		if err != nil {
			t.Fatalf("CommentChar err = %v, want nil", err)
		}
		if got != "#" {
			t.Errorf("CommentChar (auto) = %q, want %q (common-case default)", got, "#")
		}
	})
}

// --- 18. FR-V2 lifecycle: commit-msg sees prepare-commit-msg's output (ONE shared file) ---

// TestRunCommitHooks_PrepareAndCommitMsg_SharedFile verifies the shared message-file lifecycle:
// prepare-commit-msg appends a marker, then commit-msg (running on the SAME file) appends a SECOND
// marker visible only because prepare's output is on the same file. Both markers land in finalMsg.
func TestRunCommitHooks_PrepareAndCommitMsg_SharedFile(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
	installHook(t, repo, "prepare-commit-msg", `echo 'PREPARED' >> "$1"`)
	installHook(t, repo, "commit-msg", `echo 'LINTED' >> "$1"`)

	_, finalMsg, err := RunCommitHooks(context.Background(), g, defaultCfg(), snapshotTree, parentSHA,
		"feat: test", HookOpts{})
	if err != nil {
		t.Fatalf("RunCommitHooks err = %v, want nil", err)
	}
	if !strings.Contains(finalMsg, "PREPARED") {
		t.Errorf("prepare annotation missing from finalMsg: %q", finalMsg)
	}
	if !strings.Contains(finalMsg, "LINTED") {
		t.Errorf("commit-msg annotation missing from finalMsg: %q", finalMsg)
	}
}

// --- 19. prepare-commit-msg argc: git githooks(5) — plain commit passes 1 arg ($2 unset) ---

// TestRunCommitHooks_PrepareCommitMsg_ArgcIsOne asserts git-parity for the prepare-commit-msg
// invocation: git githooks(5) specifies that for a plain commit the hook is invoked with ONE
// argument (the message file) — the <source> parameter is ABSENT ($# == 1, $2 unset). The argv must
// be []string{msgPath}, NOT []string{msgPath, ""} (which would make $# == 2 with $2 = empty string).
// A hook that branches on "$#" (e.g. `[ "$#" -eq 1 ] && ...`) would take the wrong branch under argc=2.
func TestRunCommitHooks_PrepareCommitMsg_ArgcIsOne(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)

	argcFile := filepath.Join(repo, "argc.txt")
	installHook(t, repo, "prepare-commit-msg", `echo "ARGC=$#" > `+argcFile)

	cfg := defaultCfg()
	_, _, err := RunCommitHooks(context.Background(), g, cfg, snapshotTree, parentSHA,
		"feat: a change", HookOpts{})
	if err != nil {
		t.Fatalf("RunCommitHooks err = %v", err)
	}

	data, readErr := os.ReadFile(argcFile)
	if readErr != nil {
		t.Fatalf("read argc file: %v", readErr)
	}
	got := strings.TrimSpace(string(data))
	if got != "ARGC=1" {
		t.Errorf("prepare-commit-msg $# = %q, want \"ARGC=1\" (git githooks(5): plain commit passes 1 arg; $2 unset)", got)
	}
}
