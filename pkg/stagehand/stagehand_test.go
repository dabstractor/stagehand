package stagehand

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/exitcode"
	"github.com/dustin/stagehand/internal/stubtest"
)

// --- Fixture helpers (copied from internal/generate/generate_test.go — package-private, unimportable) ---

// initRepo creates a git repo in dir with repo-local identity config (no env pollution).
func initRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@example.com")
}

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

// headSHA returns the current HEAD SHA of the repo at dir.
func headSHA(t *testing.T, dir string) string {
	t.Helper()
	return runGit(t, dir, "rev-parse", "HEAD")
}

// commitRaw creates an empty commit with the given message.
func commitRaw(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "commit", "--allow-empty", "-m", msg)
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

// setupScriptedRepo initializes a temp git repo with a single commit whose subject is
// headSubject, changes CWD into it, and registers the stub provider in SCRIPT (call-varying)
// mode via a repo-local .stagehand.toml. The responses slice is the per-call stdout sequence;
// blank entries are significant (empty output → parse failure → FR29 retry).
// Sibling to setupTestRepo; mirrors its initRepo/commitRaw/chdir/cleanup pattern.
func setupScriptedRepo(t *testing.T, headSubject string, responses []string) string {
	t.Helper()
	bin := stubtest.Build(t)
	repo := t.TempDir()
	aux := t.TempDir() // script + counter live outside the repo (keeps git's view clean)
	script := aux + "/script.txt"
	if err := os.WriteFile(script, []byte(strings.Join(responses, "\n")), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	counter := aux + "/counter" // absent ⇒ stub reads 0

	var sb strings.Builder
	sb.WriteString("[provider.stub]\n")
	sb.WriteString("command = \"" + bin + "\"\n")
	sb.WriteString("prompt_delivery = \"stdin\"\n")
	sb.WriteString("output = \"raw\"\n")
	sb.WriteString("strip_code_fence = true\n")
	sb.WriteString("\n[provider.stub.env]\n")
	sb.WriteString("STAGEHAND_STUB_SCRIPT = \"" + script + "\"\n")
	sb.WriteString("STAGEHAND_STUB_COUNTER = \"" + counter + "\"\n")
	if err := os.WriteFile(repo+"/.stagehand.toml", []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write .stagehand.toml: %v", err)
	}

	initRepo(t, repo)
	commitRaw(t, repo, headSubject)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir %s: %v", repo, err)
	}
	t.Cleanup(func() { os.Chdir(wd) })
	return bin
}

// setupTestRepo initializes a temp git repo with an initial commit, changes CWD into it,
// and registers the stub provider via a repo-local .stagehand.toml.
func setupTestRepo(t *testing.T, stubOpts stubtest.Options) string {
	t.Helper()
	bin := stubtest.Build(t)
	repo := t.TempDir()

	// Write repo-local .stagehand.toml to register the stub provider.
	// config.Load Layer 3 reads CWD/.stagehand.toml; DecodeUserOverrides decodes [provider.stub].
	var sb strings.Builder
	sb.WriteString("[provider.stub]\n")
	sb.WriteString("command = \"" + bin + "\"\n")
	sb.WriteString("prompt_delivery = \"stdin\"\n")
	sb.WriteString("output = \"raw\"\n")
	sb.WriteString("strip_code_fence = true\n")

	if stubOpts.Out != "" || stubOpts.SleepMS > 0 {
		sb.WriteString("\n[provider.stub.env]\n")
		if stubOpts.Out != "" {
			sb.WriteString("STAGEHAND_STUB_OUT = \"" + stubOpts.Out + "\"\n")
		}
		if stubOpts.SleepMS > 0 {
			sb.WriteString("STAGEHAND_STUB_SLEEP_MS = \"" + strconv.Itoa(stubOpts.SleepMS) + "\"\n")
		}
	}

	if err := os.WriteFile(repo+"/.stagehand.toml", []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write .stagehand.toml: %v", err)
	}

	initRepo(t, repo)
	commitRaw(t, repo, "initial")

	// Chdir into the repo (GenerateCommit uses os.Getwd()).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir %s: %v", repo, err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	return bin
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

// looseObjectTypes returns a map of SHA→object type for all objects in the repo at dir.
// Uses git cat-file --batch-all-objects --batch-check (covers loose + packed).
// Symmetric to objectCountLine; used to prove a NEW tree object appeared (Issue 6).
func looseObjectTypes(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := gitOut(t, dir, "cat-file", "--batch-all-objects", "--batch-check")
	objs := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line) // "<sha> <type> <size>"
		if len(f) >= 2 {
			objs[f[0]] = f[1] // sha → type
		}
	}
	return objs
}

// --- Tests ---

// TestGenerateCommit_Success verifies the happy path: a repo with a staged change,
// stub returns "feat: add x", GenerateCommit creates a commit with the expected result.
func TestGenerateCommit_Success(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: add x"})
	repoDir, _ := os.Getwd()

	writeFile(t, repoDir, "new.txt", "hello world")
	stageFile(t, repoDir, "new.txt")

	ctx := context.Background()
	res, err := GenerateCommit(ctx, Options{Provider: "stub"})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}

	if !shaRe.MatchString(res.CommitSHA) {
		t.Errorf("CommitSHA = %q, want hex SHA", res.CommitSHA)
	}
	if res.Subject != "feat: add x" {
		t.Errorf("Subject = %q, want %q", res.Subject, "feat: add x")
	}
	if res.Message != "feat: add x" {
		t.Errorf("Message = %q, want %q", res.Message, "feat: add x")
	}
	if res.Provider != "stub" {
		t.Errorf("Provider = %q, want %q", res.Provider, "stub")
	}

	// HEAD should match CommitSHA.
	if got := headSHA(t, repoDir); got != res.CommitSHA {
		t.Errorf("HEAD = %q, want %q", got, res.CommitSHA)
	}
}

// TestGenerateCommit_DryRun verifies that DryRun returns a message without creating a commit.
func TestGenerateCommit_DryRun(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: preview"})
	repoDir, _ := os.Getwd()

	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")

	beforeSHA := headSHA(t, repoDir)

	ctx := context.Background()
	res, err := GenerateCommit(ctx, Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun: %v", err)
	}

	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun)", res.CommitSHA)
	}
	if res.Message != "feat: preview" {
		t.Errorf("Message = %q, want %q", res.Message, "feat: preview")
	}
	if res.Subject != "feat: preview" {
		t.Errorf("Subject = %q, want %q", res.Subject, "feat: preview")
	}

	// HEAD should be unchanged.
	afterSHA := headSHA(t, repoDir)
	if afterSHA != beforeSHA {
		t.Errorf("HEAD changed from %q to %q, want unchanged (DryRun)", beforeSHA, afterSHA)
	}
}

// TestGenerateCommit_NothingStaged verifies that nothing staged returns ErrNothingToCommit.
func TestGenerateCommit_NothingStaged(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: x"})

	ctx := context.Background()
	_, err := GenerateCommit(ctx, Options{Provider: "stub"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("errors.Is(err, ErrNothingToCommit) = false, error = %v", err)
	}
}

// TestGenerateCommit_ProviderOverride verifies that opts.Provider selects the stub provider.
func TestGenerateCommit_ProviderOverride(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: override"})
	repoDir, _ := os.Getwd()

	writeFile(t, repoDir, "a.txt", "data")
	stageFile(t, repoDir, "a.txt")

	ctx := context.Background()
	res, err := GenerateCommit(ctx, Options{Provider: "stub"})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	if res.Provider != "stub" {
		t.Errorf("Provider = %q, want %q", res.Provider, "stub")
	}
}

// TestGenerateCommit_Timeout verifies that a stub sleeping longer than opts.Timeout
// returns ErrTimeout (DryRun path) or *RescueError{Kind:ErrTimeout} (commit path).
func TestGenerateCommit_Timeout(t *testing.T) {
	// DryRun path: *RescueError{Kind:ErrTimeout} with a real TreeSHA (S1 snapshot + S2 loop).
	t.Run("dryrun", func(t *testing.T) {
		setupTestRepo(t, stubtest.Options{Out: "feat: slow", SleepMS: 2000})
		repoDir, _ := os.Getwd()

		writeFile(t, repoDir, "z.txt", "data")
		stageFile(t, repoDir, "z.txt")

		ctx := context.Background()
		_, err := GenerateCommit(ctx, Options{
			Provider: "stub",
			DryRun:   true,
			Timeout:  150 * time.Millisecond,
		})
		if err == nil {
			t.Fatal("expected error on timeout, got nil")
		}
		// DryRun path: *RescueError{Kind:ErrTimeout} with a real TreeSHA (S1 snapshot + S2 loop).
		if !errors.Is(err, ErrTimeout) {
			t.Errorf("errors.Is(err, ErrTimeout) = false, error = %v", err)
		}
		var re *RescueError
		if !errors.As(err, &re) {
			t.Fatalf("dryrun: error type = %T, want *RescueError", err)
		}
		if re.Kind != ErrTimeout {
			t.Errorf("dryrun: re.Kind = %v, want ErrTimeout", re.Kind)
		}
		if re.TreeSHA == "" {
			t.Error("dryrun: RescueError.TreeSHA empty, want non-empty (snapshot was taken — S1)")
		}
		if code := exitcode.For(err); code != exitcode.Timeout {
			t.Errorf("dryrun: exitcode.For(err) = %d, want %d (Timeout/124)", code, exitcode.Timeout)
		}
	})

	// Commit path (SystemExtra set): *RescueError{Kind:ErrTimeout} with TreeSHA.
	t.Run("commit_path", func(t *testing.T) {
		setupTestRepo(t, stubtest.Options{Out: "feat: slow", SleepMS: 2000})
		repoDir, _ := os.Getwd()

		writeFile(t, repoDir, "z2.txt", "data")
		stageFile(t, repoDir, "z2.txt")

		ctx := context.Background()
		_, err := GenerateCommit(ctx, Options{
			Provider:    "stub",
			SystemExtra: "extra instructions", // forces runPipeline commit path
			Timeout:     150 * time.Millisecond,
		})
		if err == nil {
			t.Fatal("expected error on timeout, got nil")
		}
		var re *RescueError
		if !errors.As(err, &re) {
			t.Fatalf("error type = %T, want *RescueError", err)
		}
		if !errors.Is(err, ErrTimeout) {
			t.Errorf("errors.Is(err, ErrTimeout) = false, got ErrRescue instead?")
		}
		if re.TreeSHA == "" {
			t.Error("RescueError.TreeSHA is empty, want non-empty (snapshot was taken)")
		}
	})
}

// TestGenerateCommit_DryRun_DedupeRetry verifies that a dry-run whose FIRST attempt duplicates
// a recent subject retries to the UNIQUE message (Issue 2 / FR32). The repo's HEAD subject is
// "feat: existing" and the stub script returns ["feat: existing", "feat: fresh"]; the duplicate
// first attempt is rejected and the second attempt's "feat: fresh" is returned.
// Mirrors TestCommitStaged_DedupeRetryThenSuccess at the pkg/stagehand boundary.
func TestGenerateCommit_DryRun_DedupeRetry(t *testing.T) {
	setupScriptedRepo(t, "feat: existing", []string{"feat: existing", "feat: fresh"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "a.txt", "data")
	stageFile(t, repoDir, "a.txt")

	beforeHEAD := headSHA(t, repoDir)
	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun dedupe-retry: %v", err)
	}
	if res.Message != "feat: fresh" {
		t.Errorf("Message = %q, want %q (duplicate first attempt should have been rejected & retried past)",
			res.Message, "feat: fresh")
	}
	if res.Subject != "feat: fresh" {
		t.Errorf("Subject = %q, want %q", res.Subject, "feat: fresh")
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun must not commit)", res.CommitSHA)
	}
	if got := headSHA(t, repoDir); got != beforeHEAD {
		t.Errorf("HEAD changed from %q to %q, want unchanged (DryRun)", beforeHEAD, got)
	}
}

// TestGenerateCommit_DryRun_ParseRetry verifies that a dry-run whose first attempt is
// unparseable retries (FR29) to a valid message. The stub script returns ["", "feat: good after parse retry"];
// the blank first attempt fails parse, the second attempt succeeds. Asserts no error (the OLD
// single-pass would have returned a plain error here).
func TestGenerateCommit_DryRun_ParseRetry(t *testing.T) {
	setupScriptedRepo(t, "initial", []string{"", "feat: good after parse retry"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "p.txt", "data")
	stageFile(t, repoDir, "p.txt")

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun parse-retry: %v (the unparseable first attempt should have been retried, not surfaced as an error)", err)
	}
	if res.Message != "feat: good after parse retry" {
		t.Errorf("Message = %q, want %q (blank first attempt should have triggered FR29 retry to the valid message)",
			res.Message, "feat: good after parse retry")
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun)", res.CommitSHA)
	}
}

// TestGenerateCommit_DryRun_Snapshot verifies Issue 6: WriteTree runs in dry-run, creating a
// dangling tree object, while HEAD remains unchanged and CommitSHA is empty (Issue 2). Proves
// git cat-file finds the snapshotted tree after a dry run.
func TestGenerateCommit_DryRun_Snapshot(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: snapshot taken"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "snap.txt", "data")
	stageFile(t, repoDir, "snap.txt") // writes the blob (counted in `before`)

	before := looseObjectTypes(t, repoDir) // captured AFTER staging, BEFORE GenerateCommit
	beforeHEAD := headSHA(t, repoDir)

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun snapshot: %v", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun must not commit)", res.CommitSHA)
	}

	// Issue 6: WriteTree ran in dry-run → a NEW tree-typed object exists in the object store.
	after := looseObjectTypes(t, repoDir)
	newTrees := 0
	for sha, typ := range after {
		if _, ok := before[sha]; !ok && typ == "tree" {
			newTrees++
		}
	}
	if newTrees == 0 {
		t.Error("dry-run snapshot: no new tree object created; WriteTree was skipped (Issue 6 regression)")
	}

	// Issue 2: dry-run skips commit-tree/update-ref → HEAD unchanged.
	if got := headSHA(t, repoDir); got != beforeHEAD {
		t.Errorf("HEAD changed from %q to %q, want unchanged (DryRun)", beforeHEAD, got)
	}
}

// TestGenerateCommit_SystemExtra forces the runPipeline path and commits with extra instructions.
func TestGenerateCommit_SystemExtra(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: with extra"})
	repoDir, _ := os.Getwd()

	writeFile(t, repoDir, "s.txt", "data")
	stageFile(t, repoDir, "s.txt")

	ctx := context.Background()
	res, err := GenerateCommit(ctx, Options{Provider: "stub", SystemExtra: "refs ticket #42"})
	if err != nil {
		t.Fatalf("GenerateCommit with SystemExtra: %v", err)
	}

	if !shaRe.MatchString(res.CommitSHA) {
		t.Errorf("CommitSHA = %q, want hex SHA", res.CommitSHA)
	}
	if res.Message != "feat: with extra" {
		t.Errorf("Message = %q, want %q", res.Message, "feat: with extra")
	}
	// HEAD should have advanced.
	if got := headSHA(t, repoDir); got != res.CommitSHA {
		t.Errorf("HEAD = %q, want %q", got, res.CommitSHA)
	}
}

// TestGenerateCommit_MissingProviderCommand_Issue3 proves PRD Issue 3 is fixed: a provider whose
// command is not on $PATH fails FAST (exit 1) in buildDeps BEFORE the write-tree snapshot — so the
// error is NOT a *RescueError, exitcode.For maps it to 1, the message names the missing command, and
// NO new tree object is written. Before P1.M2.T1.S1 this surfaced as exit 3 (rescue) + a dangling tree.
func TestGenerateCommit_MissingProviderCommand_Issue3(t *testing.T) {
	// Fresh repo with a repo-local .stagehand.toml registering a provider whose command does not exist.
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	const toml = "[provider.missing]\n" +
		"command = \"/nonexistent/path/agent\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"raw\"\n" +
		"strip_code_fence = true\n"
	if err := os.WriteFile(repo+"/.stagehand.toml", []byte(toml), 0o644); err != nil {
		t.Fatalf("write .stagehand.toml: %v", err)
	}

	// Chdir (GenerateCommit resolves the repo via os.Getwd()).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir %s: %v", repo, err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	// MUST stage a NEW file: if the pre-flight check were removed (regression), WriteTree would write
	// a new tree object and the count-objects guard below would catch it. With nothing staged, the
	// pipeline short-circuits to ErrNothingToCommit before WriteTree, masking the regression.
	writeFile(t, repo, "new.txt", "content")
	stageFile(t, repo, "new.txt")

	beforeCount := objectCountLine(t, repo)

	_, err = GenerateCommit(context.Background(), Options{Provider: "missing"})

	afterCount := objectCountLine(t, repo)

	// Must error.
	if err == nil {
		t.Fatal("GenerateCommit: err = nil, want non-nil (missing provider command)")
	}
	// (a) NOT a *RescueError (the bug returned *RescueError -> exit 3 + rescue block + dangling tree).
	var re *RescueError
	if errors.As(err, &re) {
		t.Errorf("error is *RescueError (%v); want a plain pre-generation error (exit 1)", re)
	}
	// (b) exitcode.For(err) == 1. A plain error falls through to `return Error`.
	if code := exitcode.For(err); code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error); err=%v", code, exitcode.Error, err)
	}
	// (c) message names the missing command.
	if msg := err.Error(); !strings.Contains(msg, "not found") || !strings.Contains(msg, "Is the agent installed?") || !strings.Contains(msg, "/nonexistent/path/agent") {
		t.Errorf("err.Error() = %q; want to contain 'not found', 'Is the agent installed?', and '/nonexistent/path/agent'", msg)
	}
	// (d) NO new tree object written (pre-flight ran before WriteTree).
	if beforeCount != afterCount {
		t.Errorf("dangling tree: git count-objects changed\n  before: %s\n  after:  %s\n(pre-flight must run before WriteTree)", beforeCount, afterCount)
	}

	// Optional: dry-run subtest — proves buildDeps protects the runPipeline path too.
	t.Run("dryrun", func(t *testing.T) {
		// Create a fresh repo for the dry-run subtest (can't reuse — CWD already restored).
		repo := t.TempDir()
		initRepo(t, repo)
		commitRaw(t, repo, "initial")
		if err := os.WriteFile(repo+"/.stagehand.toml", []byte(toml), 0o644); err != nil {
			t.Fatalf("write .stagehand.toml: %v", err)
		}
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("os.Getwd: %v", err)
		}
		if err := os.Chdir(repo); err != nil {
			t.Fatalf("chdir %s: %v", repo, err)
		}
		t.Cleanup(func() { os.Chdir(wd) })
		writeFile(t, repo, "new2.txt", "content")
		stageFile(t, repo, "new2.txt")

		beforeCount := objectCountLine(t, repo)
		_, err = GenerateCommit(context.Background(), Options{Provider: "missing", DryRun: true})
		afterCount := objectCountLine(t, repo)

		if err == nil {
			t.Fatal("GenerateCommit dryrun: err = nil, want non-nil")
		}
		var re *RescueError
		if errors.As(err, &re) {
			t.Errorf("dryrun: error is *RescueError (%v); want plain error", re)
		}
		if code := exitcode.For(err); code != exitcode.Error {
			t.Errorf("dryrun: exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
		}
		if msg := err.Error(); !strings.Contains(msg, "not found") || !strings.Contains(msg, "Is the agent installed?") {
			t.Errorf("dryrun: err.Error() = %q; want 'not found' + 'Is the agent installed?'", msg)
		}
		if beforeCount != afterCount {
			t.Errorf("dryrun: dangling tree: count changed\n  before: %s\n  after:  %s", beforeCount, afterCount)
		}
	})
}

// TestResolveConfig_InjectedConfig proves that when opts.Config is non-nil, resolveConfig uses
// the injected config directly and does NOT call config.Load. The proof: the injected config
// carries a Providers map entry for a stub provider, and the test runs in a temp dir with NO
// .stagehand.toml and NO STAGEHAND_CONFIG env — if Load ran, it would find no "stub" provider
// (built-ins only) and the Providers map would be empty. The injected provider surviving proves
// Load was skipped.
func TestResolveConfig_InjectedConfig(t *testing.T) {
	bin := stubtest.Build(t)

	// Build a config.Config with the stub provider registered in the Providers map.
	// This is the same shape that config.Load would produce from a .stagehand.toml [provider.stub] table.
	injected := config.Config{
		Provider: "stub",
		Providers: map[string]map[string]any{
			"stub": {
				"command":          bin,
				"prompt_delivery":  "stdin",
				"output":           "raw",
				"strip_code_fence": true,
			},
		},
	}

	// Create a temp git repo with NO .stagehand.toml (so config.Load would find no stub provider).
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")

	// Save and restore CWD (resolveConfig calls os.Getwd).
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	ctx := context.Background()

	t.Run("injected_config_used", func(t *testing.T) {
		cfg, repoDir, err := resolveConfig(ctx, Options{Config: &injected})
		if err != nil {
			t.Fatalf("resolveConfig: %v", err)
		}
		if cfg.Provider != "stub" {
			t.Errorf("cfg.Provider = %q, want %q", cfg.Provider, "stub")
		}
		if cfg.Providers == nil || cfg.Providers["stub"] == nil {
			t.Error("cfg.Providers[\"stub\"] is nil — injected providers map was lost")
		}
		if repoDir != repo {
			t.Errorf("repoDir = %q, want %q", repoDir, repo)
		}
	})

	t.Run("options_overrides_apply_on_injected", func(t *testing.T) {
		// Inject a config with Provider="" and override via Options.Provider.
		emptyProviderCfg := injected
		emptyProviderCfg.Provider = ""
		cfg, _, err := resolveConfig(ctx, Options{Config: &emptyProviderCfg, Provider: "stub"})
		if err != nil {
			t.Fatalf("resolveConfig: %v", err)
		}
		if cfg.Provider != "stub" {
			t.Errorf("cfg.Provider = %q, want %q (Options override should win)", cfg.Provider, "stub")
		}
	})

	t.Run("timeout_override_applies", func(t *testing.T) {
		cfg, _, err := resolveConfig(ctx, Options{Config: &injected, Timeout: 5 * time.Minute})
		if err != nil {
			t.Fatalf("resolveConfig: %v", err)
		}
		if cfg.Timeout != 5*time.Minute {
			t.Errorf("cfg.Timeout = %v, want %v", cfg.Timeout, 5*time.Minute)
		}
	})
}

// TestGenerateCommit_GenerationConfigFile_OutputJSON_Issue4 proves PRD Issue 4: the [generation]
// output="json" field (file-loader path) overrides the provider manifest's output="raw" and is
// honored by ParseOutput end-to-end through buildDeps. Without the S1 bridge (~196-207 in
// stagehand.go), res.Message would equal the raw JSON blob instead of the extracted field.
//
// TDD check (manual, do not commit): comment out the S1 bridge block and re-run — this test FAILS
// (raw blob observed instead of "feat: from json config").
func TestGenerateCommit_GenerationConfigFile_OutputJSON_Issue4(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	jsonOut := `{"subject":"feat: from json config"}`

	// TOML uses a literal string ('...') for STAGEHAND_STUB_OUT so the JSON quotes survive parsing.
	toml := "[provider.stub]\n" +
		"command = \"" + bin + "\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"raw\"\n" + // manifest baseline — [generation] must override this
		"json_field = \"subject\"\n" + // REQUIRED: parseJSON extracts obj["subject"]
		"strip_code_fence = true\n" +
		"\n[provider.stub.env]\n" +
		"STAGEHAND_STUB_OUT = '" + jsonOut + "'\n" +
		"\n[generation]\n" +
		"output = \"json\"\n" // the [generation] override
	if err := os.WriteFile(repo+"/.stagehand.toml", []byte(toml), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "data")
	stageFile(t, repo, "new.txt")

	wd, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun)", res.CommitSHA)
	}
	// The JSON field was extracted ⇒ the [generation] output="json" overrode the manifest's "raw".
	if res.Message != "feat: from json config" {
		t.Errorf("Message = %q, want %q ([generation] output=json must make ParseOutput extract the JSON field)",
			res.Message, "feat: from json config")
	}
}

// TestGenerateCommit_GitConfig_OutputJSON_Issue4 proves PRD Issue 4: git-config layer-4
// `stagehand.output json` overrides the manifest's output="raw" and is honored by ParseOutput
// end-to-end. Uses t.Setenv("HOME", t.TempDir()) to isolate the global git config (mirrors
// git_test.go:71).
//
// TDD check (manual, do not commit): comment out the S1 bridge block and re-run — this test FAILS
// (raw blob observed instead of extracted field).
func TestGenerateCommit_GitConfig_OutputJSON_Issue4(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate global git config
	bin := stubtest.Build(t)
	repo := t.TempDir()
	jsonOut := `{"subject":"feat: from git-config json"}`

	toml := "[provider.stub]\n" +
		"command = \"" + bin + "\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"raw\"\n" +
		"json_field = \"subject\"\n" +
		"strip_code_fence = true\n" +
		"\n[provider.stub.env]\n" +
		"STAGEHAND_STUB_OUT = '" + jsonOut + "'\n"
	if err := os.WriteFile(repo+"/.stagehand.toml", []byte(toml), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	runGit(t, repo, "config", "stagehand.output", "json") // Layer-4 override
	writeFile(t, repo, "new.txt", "data")
	stageFile(t, repo, "new.txt")

	wd, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun)", res.CommitSHA)
	}
	if res.Message != "feat: from git-config json" {
		t.Errorf("Message = %q, want %q (git config stagehand.output=json must reach ParseOutput)",
			res.Message, "feat: from git-config json")
	}
}

// TestGenerateCommit_InjectedConfig_StripCodeFenceFalse_Issue4 proves PRD Issue 4: when
// cfg.StripCodeFence=false (injected via Options.Config), ParseOutput RETAINS the ``` fences in
// the message instead of stripping them. This proves the buildDeps bridge copies cfg.StripCodeFence
// onto the manifest unconditionally (stagehand.go:208-209).
//
// NOTE: The git-config loader uses a non-zero overlay (file.go:overlay) that cannot propagate
// `false` for bool fields (FINDING G in git.go). Similarly, file.go:153 cannot set
// strip_code_fence=false (v1 quirk). Injecting Options.Config directly is the only way to reach
// cfg.StripCodeFence=false and exercise the bridge's false-path.
//
// TDD check (manual, do not commit): comment out the S1 bridge block and re-run — this test FAILS
// (fence stripped, message is "feat: keep the fence" without backticks).
func TestGenerateCommit_InjectedConfig_StripCodeFenceFalse_Issue4(t *testing.T) {
	bin := stubtest.Build(t)

	// Stub output: a fenced block. When cfg.StripCodeFence=false, fences are retained;
	// when true (bridge absent), stripCodeFence() removes them → "feat: keep the fence".
	stubOut := "```" + "\n" + "feat: keep the fence" + "\n" + "```"

	// Start from Defaults, then set StripCodeFence=false to exercise the bridge's unconditional copy.
	cfg := config.Defaults()
	cfg.Provider = "stub"
	cfg.StripCodeFence = false // inject false — cannot reach this via file/git-config loaders (FINDING G)
	cfg.Providers = map[string]map[string]any{
		"stub": {
			"command":          bin,
			"prompt_delivery":  "stdin",
			"output":           "raw",
			"strip_code_fence": true, // manifest says strip ON — cfg=false must override it
			"env": map[string]any{
				"STAGEHAND_STUB_OUT": stubOut,
			},
		},
	}

	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "data")
	stageFile(t, repo, "new.txt")

	wd, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	res, err := GenerateCommit(context.Background(), Options{Config: &cfg, Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun)", res.CommitSHA)
	}
	// With cfg.StripCodeFence=false overriding the manifest's true, the fences are RETAINED.
	if !strings.Contains(res.Message, "```") {
		t.Errorf("Message = %q; want to contain \"```\" (fence retained because cfg.StripCodeFence=false)",
			res.Message)
	}
}

// TestGenerateCommit_ManifestOutputWins_WhenCfgOutputEmpty_Issue4 proves PRD Issue 4: the
// buildDeps bridge's `if cfg.Output != ""` guard lets the manifest's own output="json" win when
// cfg.Output is empty. This is a deliberate regression guard on the bridge contract.
//
// CRITICAL NOTE: cfg.Output="" never occurs via the real loaders — config.Defaults() ALWAYS sets
// Output="raw" and config.Load() applies Defaults() first. The ONLY way to exercise the guard's
// "manifest wins" branch is to inject Options.Config directly (resolveConfig then does
// `cfg = *opts.Config` and SKIPS config.Load/Defaults), with Output explicitly set to "". This
// test pins the bridge's guard contract; it is NOT claiming a production code path.
//
// TDD check (manual, do not commit): test 4 STILL PASSES even when the S1 bridge is removed,
// because it exercises the guard's ABSENCE-of-override path (the manifest's value was never
// overwritten).
func TestGenerateCommit_ManifestOutputWins_WhenCfgOutputEmpty_Issue4(t *testing.T) {
	bin := stubtest.Build(t)

	// Start from Defaults so Timeout/MaxDuplicateRetries/etc. are sane, then ZERO Output to exercise
	// the bridge's `if cfg.Output != ""` guard.
	cfg := config.Defaults()
	cfg.Provider = "stub"
	cfg.Output = "" // "[generation] output unset" at the field level
	cfg.Providers = map[string]map[string]any{
		"stub": {
			"command":          bin,
			"prompt_delivery":  "stdin",
			"output":           "json", // manifest's own value — must win because cfg.Output==""
			"json_field":       "subject",
			"strip_code_fence": true,
			"env": map[string]any{
				"STAGEHAND_STUB_OUT": `{"subject":"feat: manifest wins when cfg unset"}`,
			},
		},
	}

	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "data")
	stageFile(t, repo, "new.txt")

	wd, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	res, err := GenerateCommit(context.Background(), Options{Config: &cfg, Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun)", res.CommitSHA)
	}
	// manifest output="json" won (cfg.Output="" ⇒ guard fell through) ⇒ JSON extracted.
	if res.Message != "feat: manifest wins when cfg unset" {
		t.Errorf("Message = %q, want %q (manifest output=json must win when cfg.Output is empty)",
			res.Message, "feat: manifest wins when cfg unset")
	}
}
