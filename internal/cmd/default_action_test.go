package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/exitcode"
	"github.com/dustin/stagehand/internal/stubtest"
)

// ---------------------------------------------------------------------------
// Fixture helpers (COPIED from internal/generate/generate_test.go — package-private
// there, unimportable by cmd). Bodies kept identical.
// ---------------------------------------------------------------------------

// writeFile creates a file at dir/name with the given body.
func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := dir + string(os.PathSeparator) + name
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", full, err)
	}
}

// stageFile runs git add for name in dir.
func stageFile(t *testing.T, dir, name string) {
	t.Helper()
	runGit(t, dir, "add", name)
}

// commitRaw creates an empty commit with the given message.
func commitRaw(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "commit", "--allow-empty", "-m", msg)
}

// headSHA returns the current HEAD SHA of the repo at dir.
func headSHA(t *testing.T, dir string) string {
	t.Helper()
	return runGit(t, dir, "rev-parse", "HEAD")
}

// gitOut runs a raw git command in dir and returns trimmed stdout.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return runGit(t, dir, args...)
}

// runGit executes git -C dir args... and returns trimmed stdout.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

var shaRe = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

// ---------------------------------------------------------------------------
// Test seam helper: sets up a temp git repo with a stub provider in .stagehand.toml
// and the STAGEHAND_STUB_OUT env var.
// ---------------------------------------------------------------------------

// setupStubRepo creates a temp git repo, writes a .stagehand.toml with the stub
// provider pointing at the compiled stubagent binary, sets STAGEHAND_STUB_OUT so
// the stub returns the given response, and commits the .stagehand.toml so it's
// tracked (not untracked). Returns the repo dir (caller must chdir — already done).
func setupStubRepo(t *testing.T, stubOut string) string {
	t.Helper()
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	chdir(t, repo)

	// Write .stagehand.toml with the stub provider (read by BOTH CLI PersistentPreRunE
	// and GenerateCommit via DISCOVERY — design §2/§7).
	toml := fmt.Sprintf(`[provider.stub]
command = %q
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
`, bin)
	writeConfigFile(t, repo, ".stagehand.toml", toml)

	// Commit the config so it's tracked and not an untracked file picked up by AddAll
	runGit(t, repo, "add", ".stagehand.toml")
	runGit(t, repo, "commit", "-m", "init: add stagehand config")

	t.Setenv("STAGEHAND_STUB_OUT", stubOut)
	return repo
}

// setupStubRepoWithTimeout creates a temp git repo with a stub provider that sleeps
// for sleepMs milliseconds and returns the given out. Also sets a short timeout in
// the config. The .stagehand.toml is committed so it's tracked. Returns the repo dir.
func setupStubRepoWithTimeout(t *testing.T, stubOut string, sleepMs int, timeout time.Duration) string {
	t.Helper()
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	chdir(t, repo)

	toml := fmt.Sprintf(`[provider.stub]
command = %q
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true

[defaults]
timeout = "%s"
`, bin, timeout)
	writeConfigFile(t, repo, ".stagehand.toml", toml)

	// Commit the config so it's tracked
	runGit(t, repo, "add", ".stagehand.toml")
	runGit(t, repo, "commit", "-m", "init: add stagehand config")

	t.Setenv("STAGEHAND_STUB_OUT", stubOut)
	t.Setenv("STAGEHAND_STUB_SLEEP_MS", fmt.Sprintf("%d", sleepMs))
	return repo
}

// setupStubRepoRaw creates a temp git repo with a raw .stagehand.toml (not committed).
// Used by tests that need precise control over what's tracked.
func setupStubRepoRaw(t *testing.T, tomlBody string) string {
	t.Helper()
	repo := t.TempDir()
	initRepo(t, repo)
	chdir(t, repo)
	writeConfigFile(t, repo, ".stagehand.toml", tomlBody)
	return repo
}

// objectCountLine returns the "count:" line of `git count-objects -v` for the repo at dir.
// Used to assert no NEW loose objects (no dangling tree) were written by a failed run.
func objectCountLine(t *testing.T, dir string) string {
	t.Helper()
	for _, line := range strings.Split(gitOut(t, dir, "count-objects", "-v"), "\n") {
		if strings.HasPrefix(line, "count:") {
			return line
		}
	}
	t.Fatalf("git count-objects -v: no 'count:' line in output:\n%s", gitOut(t, dir, "count-objects", "-v"))
	return ""
}

// ---------------------------------------------------------------------------
// TestRunDefault_Commit — happy path: staged file → commit + FR42 report
// ---------------------------------------------------------------------------

func TestRunDefault_Commit(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	repo := setupStubRepo(t, "feat: add login")
	writeFile(t, repo, "new.txt", "hello world")
	stageFile(t, repo, "new.txt")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	code := exitcode.For(err)
	if code != exitcode.Success {
		t.Errorf("exitcode.For(err) = %d, want %d (Success)", code, exitcode.Success)
	}

	// HEAD moved from initial
	head := headSHA(t, repo)
	if !shaRe.MatchString(head) {
		t.Errorf("HEAD = %q, want hex SHA", head)
	}

	// git log round-trips
	logMsg := gitOut(t, repo, "log", "--format=%B", "-n1")
	if logMsg != "feat: add login" {
		t.Errorf("git log message = %q, want %q", logMsg, "feat: add login")
	}

	// stdout contains FR42 report
	stdout := outBuf.String()
	if !strings.Contains(stdout, "] feat: add login") {
		t.Errorf("stdout = %q, want to contain '[<sha>] feat: add login'", stdout)
	}
	if !strings.Contains(stdout, "A  new.txt") {
		t.Errorf("stdout = %q, want to contain 'A  new.txt'", stdout)
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_RootCommit — unborn repo → root commit with DiffTree --root
// ---------------------------------------------------------------------------

func TestRunDefault_RootCommit(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	chdir(t, repo)

	// Write .stagehand.toml BUT don't commit it — repo is unborn, we test root commit.
	// The config file will be part of the root commit's tree.
	toml := fmt.Sprintf(`[provider.stub]
command = %q
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
`, bin)
	writeConfigFile(t, repo, ".stagehand.toml", toml)
	t.Setenv("STAGEHAND_STUB_OUT", "chore: initial")

	writeFile(t, repo, "first.txt", "content")
	stageFile(t, repo, "first.txt")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	head := headSHA(t, repo)
	if !shaRe.MatchString(head) {
		t.Errorf("HEAD = %q, want hex SHA", head)
	}

	// No parent line in the commit object
	catfile := gitOut(t, repo, "cat-file", "-p", head)
	if strings.Contains(catfile, "\nparent ") {
		t.Errorf("root commit has a parent line: %s", catfile)
	}

	// DiffTree --root → file list non-empty (at least first.txt)
	stdout := outBuf.String()
	if !strings.Contains(stdout, "A  first.txt") {
		t.Errorf("stdout = %q, want to contain 'A  first.txt'", stdout)
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_DryRun — --dry-run: message on stdout, no commit
// ---------------------------------------------------------------------------

func TestRunDefault_DryRun(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	repo := setupStubRepo(t, "feat: dry run")
	writeFile(t, repo, "x.txt", "data")
	stageFile(t, repo, "x.txt")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub", "--dry-run"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	// stdout = message ONLY (§15.5 pipe use case)
	stdout := strings.TrimSpace(outBuf.String())
	if stdout != "feat: dry run" {
		t.Errorf("stdout = %q, want %q (message only)", stdout, "feat: dry run")
	}

	// Appendix B.3: "(no commit created)" on stderr; stdout stays clean for piping (§15.5).
	if !strings.Contains(errBuf.String(), "(no commit created)") {
		t.Errorf("stderr = %q, want to contain '(no commit created)'", errBuf.String())
	}
	if strings.Contains(stdout, "(no commit created)") {
		t.Errorf("stdout = %q, must NOT contain '(no commit created)' (pipeable)", stdout)
	}

	// HEAD unchanged — the last commit should still be "init: add stagehand config"
	logMsg := gitOut(t, repo, "log", "--format=%s", "-n1")
	if logMsg != "init: add stagehand config" {
		t.Errorf("HEAD moved to %q, want 'init: add stagehand config' (no commit)", logMsg)
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_NothingStaged_FR17 — clean tree, AutoStageAll=true → exit 2
// ---------------------------------------------------------------------------

func TestRunDefault_NothingStaged_FR17(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	repo := setupStubRepo(t, "feat: x")
	// Everything is committed — truly clean tree, nothing to auto-stage

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (nothing to commit)")
	}

	code := exitcode.For(err)
	if code != exitcode.NothingToCommit {
		t.Errorf("exitcode.For(err) = %d, want %d (NothingToCommit)", code, exitcode.NothingToCommit)
	}

	// HEAD unchanged
	logMsg := gitOut(t, repo, "log", "--format=%s", "-n1")
	if logMsg != "init: add stagehand config" {
		t.Errorf("HEAD moved to %q, want 'init: add stagehand config'", logMsg)
	}

	// Issue 7: clean tree must NOT print the misleading "staging all changes" notice.
	stderr := errBuf.String()
	if strings.Contains(stderr, "staging all changes") {
		t.Errorf("stderr = %q, want NO auto-stage notice on a clean tree (Issue 7)", stderr)
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_NoAutoStage_FR19 — --no-auto-stage + nothing staged → exit 2
// ---------------------------------------------------------------------------

func TestRunDefault_NoAutoStage_FR19(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	repo := setupStubRepo(t, "feat: x")
	writeFile(t, repo, "y.txt", "unstaged")
	// y.txt exists but is NOT staged

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub", "--no-auto-stage"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (nothing staged with --no-auto-stage)")
	}

	code := exitcode.For(err)
	if code != exitcode.NothingToCommit {
		t.Errorf("exitcode.For(err) = %d, want %d (NothingToCommit)", code, exitcode.NothingToCommit)
	}

	// The error message should mention "Nothing staged."
	errStr := err.Error()
	if !strings.Contains(errStr, "Nothing staged.") {
		t.Errorf("err.Error() = %q, want to contain 'Nothing staged.'", errStr)
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_AllFlag — -a stages all then commits
// ---------------------------------------------------------------------------

func TestRunDefault_AllFlag(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	repo := setupStubRepo(t, "feat: all")
	writeFile(t, repo, "a.txt", "unstaged")
	// a.txt is NOT staged; -a should stage it

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub", "-a"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	// HEAD moved
	logMsg := gitOut(t, repo, "log", "--format=%s", "-n1")
	if logMsg != "feat: all" {
		t.Errorf("git log subject = %q, want 'feat: all'", logMsg)
	}

	// stdout has the file list (AddAll staged it)
	stdout := outBuf.String()
	if !strings.Contains(stdout, "A  a.txt") {
		t.Errorf("stdout = %q, want to contain 'A  a.txt'", stdout)
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_AutoStageNotice_FR18 — unstaged files + AutoStageAll=true → FR18 notice
// ---------------------------------------------------------------------------

func TestRunDefault_AutoStageNotice_FR18(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	repo := setupStubRepo(t, "feat: auto")
	writeFile(t, repo, "u.txt", "content")
	writeFile(t, repo, "v.txt", "content")
	// u.txt + v.txt are NOT staged; AutoStageAll=true (default) should auto-stage

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	// FR18 notice on stderr (verbatim, em-dash)
	stderr := errBuf.String()
	if !strings.Contains(stderr, "Nothing staged — staging all changes (2 files).") {
		t.Errorf("stderr = %q, want to contain FR18 notice 'Nothing staged — staging all changes (2 files).'", stderr)
	}

	// HEAD moved
	logMsg := gitOut(t, repo, "log", "--format=%s", "-n1")
	if logMsg != "feat: auto" {
		t.Errorf("git log subject = %q, want 'feat: auto'", logMsg)
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_Rescue — blank stub output + MaxDuplicateRetries=0 → rescue, exit 3
// ---------------------------------------------------------------------------

func TestRunDefault_Rescue(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	bin := stubtest.Build(t)
	repo := setupStubRepoRaw(t, fmt.Sprintf(`[provider.stub]
command = %q
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true

[generation]
max_duplicate_retries = 0
`, bin))

	t.Setenv("STAGEHAND_STUB_OUT", "")

	// Commit the config first, then add test file
	runGit(t, repo, "add", ".stagehand.toml")
	runGit(t, repo, "commit", "-m", "initial")
	writeFile(t, repo, "z.txt", "data")
	stageFile(t, repo, "z.txt")

	beforeHEAD := headSHA(t, repo)

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (rescue)")
	}

	code := exitcode.For(err)
	if code != exitcode.Rescue {
		t.Errorf("exitcode.For(err) = %d, want %d (Rescue=3)", code, exitcode.Rescue)
	}

	// stderr contains the §18.3 rescue block
	stderr := errBuf.String()
	if !strings.Contains(stderr, "❌ Commit generation failed.") {
		t.Errorf("stderr = %q, want to contain '❌ Commit generation failed.'", stderr)
	}
	if !strings.Contains(stderr, "Tree ID:") {
		t.Errorf("stderr = %q, want to contain 'Tree ID:'", stderr)
	}

	// HEAD unchanged (§20.2 idempotent)
	if got := headSHA(t, repo); got != beforeHEAD {
		t.Errorf("HEAD changed from %q to %q, want unchanged", beforeHEAD, got)
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_Timeout — slow stub + short timeout → timeout, exit 124
// ---------------------------------------------------------------------------

func TestRunDefault_Timeout(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	repo := setupStubRepoWithTimeout(t, "feat: slow", 2000, 150*time.Millisecond)
	writeFile(t, repo, "z.txt", "data")
	stageFile(t, repo, "z.txt")

	beforeHEAD := headSHA(t, repo)

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (timeout)")
	}

	code := exitcode.For(err)
	if code != exitcode.Timeout {
		t.Errorf("exitcode.For(err) = %d, want %d (Timeout=124)", code, exitcode.Timeout)
	}

	// stderr contains rescue block (timeout fires rescue format)
	stderr := errBuf.String()
	if !strings.Contains(stderr, "❌ Commit generation failed.") {
		t.Errorf("stderr = %q, want to contain rescue block", stderr)
	}

	// HEAD unchanged
	if got := headSHA(t, repo); got != beforeHEAD {
		t.Errorf("HEAD changed from %q to %q on timeout, want unchanged", beforeHEAD, got)
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_CAS — HEAD moves mid-generation → CAS error, exit 1
// ---------------------------------------------------------------------------

func TestRunDefault_CAS(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	bin := stubtest.Build(t)
	repo := setupStubRepoRaw(t, fmt.Sprintf(`[provider.stub]
command = %q
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
`, bin))

	// Commit config + initial commit
	runGit(t, repo, "add", ".stagehand.toml")
	runGit(t, repo, "commit", "-m", "initial")
	writeFile(t, repo, "b.txt", "data")
	stageFile(t, repo, "b.txt")

	headSHA(t, repo) // capture parent for reference

	// Use a readiness-marker file: the stub will create it after draining stdin,
	// proving that generation has started (the orchestrator has taken the snapshot
	// and dispatched to the agent). The test polls for the marker, then moves HEAD.
	marker := filepath.Join(t.TempDir(), "generation-started")
	t.Setenv("STAGEHAND_STUB_OUT", "feat: x")
	t.Setenv("STAGEHAND_STUB_MARKER", marker)
	// The stub must sleep long enough that the test can move HEAD before it exits.
	t.Setenv("STAGEHAND_STUB_SLEEP_MS", "5000")

	done := make(chan error, 1)
	go func() {
		var ob, eb bytes.Buffer
		rootCmd.SetOut(&ob)
		rootCmd.SetErr(&eb)
		rootCmd.SetArgs([]string{"--provider", "stub"})
		done <- rootCmd.ExecuteContext(context.Background())
	}()

	// Wait deterministically for generation to start (marker file created by stub).
	deadline := time.After(10 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break // stub has drained stdin and started sleeping — generation is in-flight
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for generation-started marker")
		case <-time.After(5 * time.Millisecond):
		}
	}

	// Move HEAD mid-generation
	commitRaw(t, repo, "concurrent commit")
	concurrent := headSHA(t, repo)

	err := <-done
	if err == nil {
		t.Fatal("Execute err=nil, want error (CAS failure)")
	}

	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error=1)", code, exitcode.Error)
	}

	// HEAD is the concurrent commit, NOT the orchestrator's
	if got := headSHA(t, repo); got != concurrent {
		t.Errorf("HEAD = %q, want %q (concurrent commit)", got, concurrent)
	}
}

// TestRunDefault_MissingProviderCommand_Issue3 proves PRD Issue 3 is fixed end-to-end through the CLI
// seam: `stagehand --provider <missing-command>` exits 1 with the not-found message and NO §18.3
// rescue block / no dangling tree. Before P1.M2.T1.S1 this was exit 3 + rescue block + dangling tree.
func TestRunDefault_MissingProviderCommand_Issue3(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	toml := "[provider.missing]\n" +
		"command = \"/nonexistent/path/agent\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"raw\"\n" +
		"strip_code_fence = true\n"
	repo := setupStubRepoRaw(t, toml)
	// setupStubRepoRaw does not commit; add an initial commit so HEAD exists and the new-file
	// count-objects guard is meaningful.
	runGit(t, repo, "add", ".stagehand.toml")
	runGit(t, repo, "commit", "-m", "initial")
	writeFile(t, repo, "new.txt", "content") // NEW file — see pkg/stagehand test comment for why
	stageFile(t, repo, "new.txt")

	beforeCount := objectCountLine(t, repo)

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "missing"})

	err := Execute(context.Background())

	afterCount := objectCountLine(t, repo)

	if err == nil {
		t.Fatal("Execute: err = nil, want non-nil (missing provider command)")
	}
	if code := exitcode.For(err); code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if msg := err.Error(); !strings.Contains(msg, "not found") || !strings.Contains(msg, "Is the agent installed?") {
		t.Errorf("err.Error() = %q; want to contain 'not found' and 'Is the agent installed?'", msg)
	}
	// NO §18.3 rescue block on stderr (the bug printed "❌ Commit generation failed." + "Tree ID:").
	stderr := errBuf.String()
	if strings.Contains(stderr, "❌ Commit generation failed.") {
		t.Errorf("stderr contains the rescue block (want NONE for a missing command):\n%s", stderr)
	}
	if strings.Contains(stderr, "Tree ID:") {
		t.Errorf("stderr contains 'Tree ID:' rescue recipe (want NONE):\n%s", stderr)
	}
	// NO dangling tree object.
	if beforeCount != afterCount {
		t.Errorf("dangling tree: git count-objects changed\n  before: %s\n  after:  %s", beforeCount, afterCount)
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_VerboseFlag — --verbose produces DEBUG output on stderr
// ---------------------------------------------------------------------------

func TestRunDefault_VerboseFlag(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	repo := setupStubRepo(t, "feat: verbose test")
	writeFile(t, repo, "v.txt", "content")
	stageFile(t, repo, "v.txt")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub", "--verbose"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	// stderr must contain DEBUG lines (verbose is on)
	stderr := errBuf.String()
	if !strings.Contains(stderr, "DEBUG:") {
		t.Errorf("stderr = %q, want to contain 'DEBUG:' lines (verbose output)", stderr)
	}
	if !strings.Contains(stderr, "DEBUG: command:") {
		t.Errorf("stderr = %q, want to contain 'DEBUG: command:' (verbose command)", stderr)
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_VerboseEnv — STAGEHAND_VERBOSE=1 produces DEBUG output
// ---------------------------------------------------------------------------

func TestRunDefault_VerboseEnv(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	repo := setupStubRepo(t, "feat: verbose env")
	writeFile(t, repo, "ve.txt", "content")
	stageFile(t, repo, "ve.txt")

	t.Setenv("STAGEHAND_VERBOSE", "1")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "DEBUG:") {
		t.Errorf("stderr = %q, want to contain 'DEBUG:' (STAGEHAND_VERBOSE=1)", stderr)
	}
}

// ---------------------------------------------------------------------------
// swapNoticeOut — redirects config.noticeOut for tests that need to capture the §19 notice
// ---------------------------------------------------------------------------

// swapNoticeOut redirects the §19 notice sink to w and registers a cleanup that restores it.
// The notice bypasses the cobra err sink (it writes to the package-level noticeOut var), so
// rootCmd.SetErr alone cannot capture it. Use this helper instead.
func swapNoticeOut(t *testing.T, w io.Writer) {
	t.Helper()
	prev := config.NoticeOut()
	config.SetNoticeOut(w)
	t.Cleanup(func() { config.SetNoticeOut(prev) })
}

// ---------------------------------------------------------------------------
// TestRunDefault_ConfigFlagHonored_Issue1 — --config provider resolves on default action
// ---------------------------------------------------------------------------

// TestRunDefault_ConfigFlagHonored_Issue1 proves Issue 1 is fixed: a user-defined provider
// ([provider.stub]) declared ONLY in a --config TOML resolves on the default action with --dry-run
// (exit 0 + the generated message). Before S1/S2 this was "unknown provider \"stub\"" (exit 1).
func TestRunDefault_ConfigFlagHonored_Issue1(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	bin := stubtest.Build(t)

	// Isolate the global layer; fresh repo with NO .stagehand.toml (provider source = --config ONLY).
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	repo := t.TempDir()
	initRepo(t, repo)
	chdir(t, repo)

	// A --config file (outside the repo) declaring a USER-DEFINED provider ONLY here.
	cfgPath := filepath.Join(t.TempDir(), "custom.toml")
	body := fmt.Sprintf("[provider.stub]\ncommand = %q\nprompt_delivery = \"stdin\"\noutput = \"raw\"\nstrip_code_fence = true\n", bin)
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	writeFile(t, repo, "new.txt", "hello")
	stageFile(t, repo, "new.txt")
	t.Setenv("STAGEHAND_STUB_OUT", "feat: config honored")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--config", cfgPath, "--provider", "stub", "--dry-run"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (--config must honor [provider.stub] on the default action)", err)
	}
	if got := strings.TrimSpace(outBuf.String()); got != "feat: config honored" {
		t.Errorf("dry-run stdout = %q, want %q", got, "feat: config honored")
	}
}

// ---------------------------------------------------------------------------
// TestRunDefault_RepoLocalNoticeOnce_Issue5 — §19 notice printed EXACTLY ONCE
// ---------------------------------------------------------------------------

// TestRunDefault_RepoLocalNoticeOnce_Issue5 proves Issue 5 is fixed: a repo-local .stagehand.toml
// that sets provider prints the §19 notice "repo-local config (.stagehand.toml) sets provider to"
// EXACTLY ONCE (strings.Count == 1; was 2 before S1/S2's single-Load fix).
func TestRunDefault_RepoLocalNoticeOnce_Issue5(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	chdir(t, repo)

	// Repo-local config: top-level provider= (fires the §19 notice) + [provider.stub] (resolves it).
	toml := fmt.Sprintf("[defaults]\nprovider = \"stub\"\n\n[provider.stub]\ncommand = %q\nprompt_delivery = \"stdin\"\noutput = \"raw\"\nstrip_code_fence = true\n", bin)
	writeConfigFile(t, repo, ".stagehand.toml", toml)
	runGit(t, repo, "add", ".stagehand.toml")
	runGit(t, repo, "commit", "-m", "init: config")

	writeFile(t, repo, "new.txt", "hello")
	stageFile(t, repo, "new.txt")
	t.Setenv("STAGEHAND_STUB_OUT", "feat: add file")

	var outBuf, errBuf, notice bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	swapNoticeOut(t, &notice) // §19 notice → buffer (it bypasses the cobra err sink)
	rootCmd.SetArgs([]string{"--provider", "stub"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	const needle = "repo-local config (.stagehand.toml) sets provider to"
	if got := strings.Count(notice.String(), needle); got != 1 {
		t.Errorf("§19 notice count = %d, want 1\n--- captured notice ---\n%s", got, notice.String())
	}
	if !strings.Contains(notice.String(), `"stub"`) {
		t.Errorf("notice = %q, want it to name provider \"stub\"", notice.String())
	}
	if logMsg := gitOut(t, repo, "log", "--format=%s", "-n1"); logMsg != "feat: add file" {
		t.Errorf("git log subject = %q, want %q (full seam)", logMsg, "feat: add file")
	}
}
