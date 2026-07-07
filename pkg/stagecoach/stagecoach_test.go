package stagecoach

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/generate"
	"github.com/dustin/stagecoach/internal/stubtest"
)

// --- Fixture helpers (copied from internal/generate/generate_test.go — package-private, unimportable) ---

func boolPtr(b bool) *bool { return &b }

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

// TestGenerateCommit_TemplateApplied verifies the §9.19 FR-F8 seam reaches runPipeline (call site
// #2): an injected config.Config with Template set templates the generated message end-to-end.
func TestGenerateCommit_TemplateApplied(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hello world")
	stageFile(t, repo, "new.txt")

	injected := &config.Config{
		Provider: "stub",
		Template: "$msg (#205)",
		Providers: map[string]map[string]any{
			"stub": {
				"command":          bin,
				"prompt_delivery":  "stdin",
				"output":           "raw",
				"strip_code_fence": true,
				"env":              map[string]any{"STAGEHAND_STUB_OUT": "feat: add x"},
			},
		},
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	ctx := context.Background()
	res, err := GenerateCommit(ctx, Options{Config: injected})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}

	want := "feat: add x (#205)"
	if res.Message != want {
		t.Errorf("Message = %q, want %q (templated)", res.Message, want)
	}
	if res.Subject != want {
		t.Errorf("Subject = %q, want %q (templated)", res.Subject, want)
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
// onto the manifest. Now also reachable via file/git-config loaders (StripCodeFence is *bool).
//
// TDD check (manual, do not commit): comment out the S1 bridge block and re-run — this test FAILS
// (fence stripped, message is "feat: keep the fence" without backticks).
func TestGenerateCommit_InjectedConfig_StripCodeFenceFalse_Issue4(t *testing.T) {
	bin := stubtest.Build(t)

	// Stub output: a fenced block. When cfg.StripCodeFence=false, fences are retained;
	// when true (bridge absent), stripCodeFence() removes them → "feat: keep the fence".
	stubOut := "```" + "\n" + "feat: keep the fence" + "\n" + "```"

	// Start from Defaults, then set StripCodeFence=false to exercise the bridge's copy.
	cfg := config.Defaults()
	cfg.Provider = "stub"
	cfg.StripCodeFence = boolPtr(false) // inject false — now reachable via file/git-config loaders too
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

// TestGenerateCommit_ManifestOutputWins_WhenCfgOutputNil_Issue4 proves PRD bugfix-002 Issue 2: the
// buildDeps bridge's `if cfg.Output != nil` guard lets the manifest's own output="json" win when
// cfg.Output is nil. This is a regression guard on the injected-Options.Config path.
//
// After S1 (Output is *string, Defaults() leaves it nil), nil is the natural default when [generation]
// and git-config omit output. This injected-config test pins the bridge's guard contract; the new
// TestGenerateCommit_ManifestOutputJSON_Honored_NoGeneration covers the real config.Load+registry path.
func TestGenerateCommit_ManifestOutputWins_WhenCfgOutputNil_Issue4(t *testing.T) {
	bin := stubtest.Build(t)

	// Start from Defaults so Timeout/MaxDuplicateRetries/etc. are sane, then leave Output nil (the
	// default after S1) to exercise the bridge's `if cfg.Output != nil` guard.
	cfg := config.Defaults()
	cfg.Provider = "stub"
	cfg.Output = nil // "[generation] output unset" at the field level
	cfg.Providers = map[string]map[string]any{
		"stub": {
			"command":          bin,
			"prompt_delivery":  "stdin",
			"output":           "json", // manifest's own value — must win because cfg.Output==nil
			"json_field":       "subject",
			"strip_code_fence": true,
			"env": map[string]any{
				"STAGEHAND_STUB_OUT": `{"subject":"feat: manifest wins when cfg nil"}`,
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
	// manifest output="json" won (cfg.Output=nil ⇒ guard fell through) ⇒ JSON extracted.
	if res.Message != "feat: manifest wins when cfg nil" {
		t.Errorf("Message = %q, want %q (manifest output=json must win when cfg.Output is nil)",
			res.Message, "feat: manifest wins when cfg nil")
	}
}

// TestGenerateCommit_ManifestOutputJSON_Honored_NoGeneration proves PRD bugfix-002 Issue 2: a
// [provider.stub] output="json" (+ json_field) is honored by ParseOutput with NO [generation] block.
// Before the S2 bridge fix, config.Defaults() seeded Output="raw" and buildDeps's `if cfg.Output != ""`
// guard ALWAYS passed, clobbering the manifest's "json" — so the raw JSON blob was returned verbatim.
// After S1 (Output is *string, nil default) + S2 (`if cfg.Output != nil`), the manifest's "json" wins.
func TestGenerateCommit_ManifestOutputJSON_Honored_NoGeneration(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	jsonOut := `{"subject":"feat: manifest json wins"}`

	// Manifest sets output="json" + json_field. NO [generation] block — the manifest value must win.
	toml := "[provider.stub]\n" +
		"command = \"" + bin + "\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"json\"\n" + // manifest-level — must be honored with no [generation]
		"json_field = \"subject\"\n" + // REQUIRED: parseJSON extracts obj["subject"]
		"strip_code_fence = true\n" +
		"\n[provider.stub.env]\n" +
		"STAGEHAND_STUB_OUT = '" + jsonOut + "'\n" // literal string preserves the JSON quotes
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
	// The manifest's output="json" was honored (no [generation] override) ⇒ the JSON field extracted.
	if res.Message != "feat: manifest json wins" {
		t.Errorf("Message = %q, want %q (manifest output=json must be honored with no [generation] block)",
			res.Message, "feat: manifest json wins")
	}
}

// TestGenerateCommit_ManifestStripCodeFenceFalse_Honored_NoGeneration proves PRD bugfix-002 Issue 2:
// a [provider.stub] strip_code_fence=false is honored by ParseOutput with NO [generation] block (the
// ``` fences are RETAINED). Before the fix, config.Defaults() seeded StripCodeFence=boolPtr(true) and
// the bridge's `!= nil` guard always passed, clobbering the manifest's false.
func TestGenerateCommit_ManifestStripCodeFenceFalse_Honored_NoGeneration(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	// Use a TOML double-quoted string for STAGEHAND_STUB_OUT with \\n escapes so the stub emits
	// a fenced block. In Go, \\n produces the two-char TOML escape \n; backticks are not special
	// in TOML double-quoted strings.
	toml := "[provider.stub]\n" +
		"command = \"" + bin + "\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"raw\"\n" +
		"strip_code_fence = false\n" + // manifest-level false — must be honored with no [generation]
		"\n[provider.stub.env]\n" +
		"STAGEHAND_STUB_OUT = \"```\\nfeat: keep the fence\\n```\"\n"
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
	// strip_code_fence=false honored (no [generation] override) ⇒ the ``` fence is RETAINED.
	if !strings.Contains(res.Message, "```") {
		t.Errorf("Message = %q; want to contain \"```\" (fence retained because manifest strip_code_fence=false)",
			res.Message)
	}
}

// TestGenerateCommit_ManifestDefaultRaw_StillWorks is a regression guard (PRD bugfix-002 Issue 2 clause d):
// with no [generation] block and the manifest default output="raw"/strip_code_fence=true (setupTestRepo's
// .stagehand.toml), a plain raw message still round-trips unchanged. This must pass BOTH before and after
// the S2 fix (raw/true is the unchanged default).
func TestGenerateCommit_ManifestDefaultRaw_StillWorks(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: default raw ok"}) // output="raw", strip=true, no [generation]
	repoDir, _ := os.Getwd()

	writeFile(t, repoDir, "new.txt", "data")
	stageFile(t, repoDir, "new.txt")

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	if res.Message != "feat: default raw ok" {
		t.Errorf("Message = %q, want %q (default raw path must be unchanged)", res.Message, "feat: default raw ok")
	}
}

// --- Decompose tests (PRD §14.1 / P4.M2.T1.S1) ---

// TestDecompose_SingleDelegates verifies that Decompose with Single:true delegates to GenerateCommit:
// a staged change produces exactly one new commit, err==nil, DecomposeResult has 1 commit with
// Provider populated. This is the NO-OP delegation path (PRD §14.1).
func TestDecompose_SingleDelegates(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: decompose single"})
	repoDir, _ := os.Getwd()

	writeFile(t, repoDir, "a.txt", "change\n")
	stageFile(t, repoDir, "a.txt")

	before := headSHA(t, repoDir)
	res, err := Decompose(context.Background(), DecomposeOptions{Options: Options{Provider: "stub"}, Single: true})
	if err != nil {
		t.Fatalf("Decompose(Single:true): %v", err)
	}

	after := headSHA(t, repoDir)
	if after == before {
		t.Error("HEAD did not advance — no commit was created")
	}
	if len(res.Commits) != 1 {
		t.Errorf("len(res.Commits) = %d, want 1", len(res.Commits))
	}
	if res.Amended != 0 {
		t.Errorf("Amended = %d, want 0", res.Amended)
	}
	if res.Provider == "" {
		t.Error("Provider is empty, want non-empty (resolved from GenerateCommit)")
	}
	if len(res.Commits) > 0 {
		c := res.Commits[0]
		if c.Provider == "" {
			t.Error("Commits[0].Provider is empty, want non-empty")
		}
		if !shaRe.MatchString(c.CommitSHA) {
			t.Errorf("Commits[0].CommitSHA = %q, want hex SHA", c.CommitSHA)
		}
		if !strings.HasPrefix(c.Subject, "feat: decompose single") {
			t.Errorf("Commits[0].Subject = %q, want prefix %q", c.Subject, "feat: decompose single")
		}
	}
}

// TestDecompose_Count1Delegates verifies that Decompose with Count:1 delegates to GenerateCommit
// (same NO-OP path as Single:true). Proves the Count==1 short-circuit.
func TestDecompose_Count1Delegates(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: decompose count1"})
	repoDir, _ := os.Getwd()

	writeFile(t, repoDir, "b.txt", "change\n")
	stageFile(t, repoDir, "b.txt")

	before := headSHA(t, repoDir)
	res, err := Decompose(context.Background(), DecomposeOptions{Options: Options{Provider: "stub"}, Count: 1})
	if err != nil {
		t.Fatalf("Decompose(Count:1): %v", err)
	}

	after := headSHA(t, repoDir)
	if after == before {
		t.Error("HEAD did not advance — no commit was created")
	}
	if len(res.Commits) != 1 {
		t.Errorf("len(res.Commits) = %d, want 1", len(res.Commits))
	}
	if res.Amended != 0 {
		t.Errorf("Amended = %d, want 0", res.Amended)
	}
	if res.Provider == "" {
		t.Error("Provider is empty, want non-empty")
	}
	if len(res.Commits) > 0 {
		c := res.Commits[0]
		if !shaRe.MatchString(c.CommitSHA) {
			t.Errorf("Commits[0].CommitSHA = %q, want hex SHA", c.CommitSHA)
		}
		if !strings.HasPrefix(c.Subject, "feat: decompose count1") {
			t.Errorf("Commits[0].Subject = %q, want prefix %q", c.Subject, "feat: decompose count1")
		}
	}
}

// TestDecompose_MultiEntry_RoleError verifies that the multi-commit path is entered (not the
// Single/Count==1 short-circuit). Provider is pinned to "stub" so the test is deterministic
// regardless of host-installed agents (avoids the FR-M2b one-file short-circuit succeeding with
// a real agent). Stub has no TooledFlags, so the stager fallback fires; with no tooled provider
// installed in the stub-only fixture, ResolveRoles fails at the stager. Either that or the
// planner fails — both prove the multi-commit path was entered.
func TestDecompose_MultiEntry_RoleError(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: x"})
	repoDir, _ := os.Getwd()

	// Dirty, UN-STAGED working tree (nothing staged — precondition FR-M1).
	writeFile(t, repoDir, "b.txt", "un-staged change\n")
	// Do NOT stageFile — the multi-commit path requires nothing staged.

	res, err := Decompose(context.Background(), DecomposeOptions{
		Options: Options{Provider: "stub"}, // pin to stub — deterministic across all hosts
	})
	if err == nil {
		t.Fatal("expected error (multi-commit path: ResolveRoles stager or planner failure), got nil")
	}

	// Prove the multi-commit path was entered: either ResolveRoles stager failed ("stager"+"tooled")
	// or the planner failed ("planner" or "decompose"). Both prove the single-delegation short-circuit
	// did NOT fire (which would have hit GenerateCommit and returned ErrNothingToCommit for unstaged).
	errMsg := err.Error()
	multiPath := strings.Contains(errMsg, "stager") || strings.Contains(errMsg, "planner") || strings.Contains(errMsg, "decompose")
	if !multiPath {
		t.Errorf("error %q does not look like a multi-commit path error (want \"stager\", \"planner\", or \"decompose\")", errMsg)
	}
	// Must NOT be ErrNothingToCommit (that would mean GenerateCommit ran = single path).
	if errors.Is(err, ErrNothingToCommit) {
		t.Error("error is ErrNothingToCommit — the single-delegation path was incorrectly entered")
	}
	if len(res.Commits) != 0 {
		t.Errorf("res.Commits = %d entries, want 0 (ResolveRoles/planner failed pre-loop)", len(res.Commits))
	}
}

// TestDecompose_StagerOverride verifies that a Stager RoleModel override flows into cfg.Roles["stager"]
// and is consumed by ResolveRoles. Setting Stager.Provider="nonexistent" produces an "unknown provider"
// error (distinct from the tooled_flags error), proving the override was applied.
func TestDecompose_StagerOverride(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: x"})
	repoDir, _ := os.Getwd()

	// Dirty, UN-STAGED working tree.
	writeFile(t, repoDir, "b.txt", "un-staged change\n")

	res, err := Decompose(context.Background(), DecomposeOptions{
		Options: Options{Provider: "stub"}, // pin every role to stub so the stager override is the only error
		Stager:  RoleModel{Provider: "nonexistent"},
	})
	if err == nil {
		t.Fatal("expected error (ResolveRoles unknown provider), got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "stager") {
		t.Errorf("error %q does not contain \"stager\"", errMsg)
	}
	if !strings.Contains(errMsg, "unknown provider") {
		t.Errorf("error %q does not contain \"unknown provider\" (override not applied?)", errMsg)
	}
	if len(res.Commits) != 0 {
		t.Errorf("res.Commits = %d entries, want 0 (ResolveRoles failed pre-loop)", len(res.Commits))
	}
}

// TestGenerateCommit_MessageRoleOverride_DryRun verifies that per-role [role.message] overrides
// (Model) reach the runPipeline path and Result.Model reports the resolved model.
// Uses DryRun:true to exercise runPipeline without creating a commit.
func TestGenerateCommit_MessageRoleOverride_DryRun(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()

	// Build config with message role override + stub provider registered.
	cfg := config.Defaults()
	cfg.Provider = "stub"
	cfg.Roles = map[string]config.RoleConfig{"message": {Model: "haiku", Reasoning: "high"}}
	cfg.Providers = map[string]map[string]any{
		"stub": {
			"command":          bin,
			"prompt_delivery":  "stdin",
			"output":           "raw",
			"strip_code_fence": true,
			"env": map[string]any{
				"STAGEHAND_STUB_OUT": "feat: with override",
			},
		},
	}

	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "data")
	stageFile(t, repo, "new.txt")

	wd, _ := os.Getwd()
	os.Chdir(repo)
	t.Cleanup(func() { os.Chdir(wd) })

	// DryRun forces the runPipeline path.
	res, err := GenerateCommit(context.Background(), Options{Config: &cfg, Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun: %v", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun)", res.CommitSHA)
	}
	if res.Model != "haiku" {
		t.Errorf("Result.Model = %q, want %q (message role override)", res.Model, "haiku")
	}
}

// TestGenerateCommit_NoMessageOverride_Regression verifies that with cfg.Roles empty (no per-role
// override), Result.Model == cfg.Model — identical to the pre-fix behavior.
func TestGenerateCommit_NoMessageOverride_Regression(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()

	cfg := config.Defaults()
	cfg.Provider = "stub"
	cfg.Model = "openrouter/gpt-5.4"
	cfg.Providers = map[string]map[string]any{
		"stub": {
			"command":          bin,
			"prompt_delivery":  "stdin",
			"output":           "raw",
			"strip_code_fence": true,
			"env": map[string]any{
				"STAGEHAND_STUB_OUT": "feat: no override",
			},
		},
	}

	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "data")
	stageFile(t, repo, "new.txt")

	wd, _ := os.Getwd()
	os.Chdir(repo)
	t.Cleanup(func() { os.Chdir(wd) })

	// DryRun exercises runPipeline; no Roles set → ResolveRoleModel returns globals.
	res, err := GenerateCommit(context.Background(), Options{Config: &cfg, Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun: %v", err)
	}
	if res.Model != "openrouter/gpt-5.4" {
		t.Errorf("Result.Model = %q, want %q (global model, no override)", res.Model, "openrouter/gpt-5.4")
	}
}

// TestBuildDeps_MessageProviderOverride proves buildDeps honors the message role's provider
// override (--message-provider / [role.message] provider) for manifest selection.
func TestBuildDeps_MessageProviderOverride(t *testing.T) {
	bin := stubtest.Build(t)
	cfg := config.Defaults()
	cfg.Providers = map[string]map[string]any{
		"alpha": {"command": bin, "prompt_delivery": "stdin", "print_flag": "-p", "output": "raw", "strip_code_fence": true},
		"beta":  {"command": bin, "prompt_delivery": "stdin", "print_flag": "-p", "output": "raw", "strip_code_fence": true},
	}
	cfg.Provider = "alpha"
	cfg.Roles = map[string]config.RoleConfig{"message": {Provider: "beta"}}

	deps, err := buildDeps(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("buildDeps: %v", err)
	}
	if deps.Manifest.Name != "beta" {
		t.Errorf("message provider override lost: Manifest.Name = %q, want %q", deps.Manifest.Name, "beta")
	}
}

// stageLargeDiff writes a ~60-line staged file (≈ 600 tokens) into dir so EstimateTokens(payload) ≫
// MultiTurnChunkTokens (cond b true). Mirrors generate_multiturn_test.go's big.txt builder.
func stageLargeDiff(t *testing.T, dir string) {
	t.Helper()
	var b strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&b, "line %02d: padding content for a large diff\n", i)
	}
	writeFile(t, dir, "big.txt", b.String())
	stageFile(t, dir, "big.txt")
}

// appendMultiTurnConfig returns a config.Config wired for the FR-T1 multi-turn dry-run tests. The stub
// provider is registered in script (call-varying) mode with session_mode="append" (cond d); cfg carries
// MultiTurnFallback=true (cond c) and MaxDuplicateRetries=0 (so the one-shot runs EXACTLY once and
// exhausts on "", forcing the FR-T1 gate). chunkTokens sets cfg.MultiTurnChunkTokens (cond b threshold).
// When sessionMode="" the provider is left NON-append (cond d false) for the skip test.
//
// This mirrors internal/generate's appendScriptManifest (direct manifest) at the pkg/stagehand boundary:
// here the config flows through GenerateCommit → config.Load (skipped via Options.Config) → registry, and
// the registry-built stub manifest supports multi-turn because RenderMultiTurn is a method on
// provider.Manifest (render.go:203) and SessionMode is a TOML/config field (manifest.go:66).
func appendMultiTurnConfig(t *testing.T, bin, script, counter string, sessionMode string, chunkTokens int) config.Config {
	t.Helper()
	cfg := config.Defaults()
	cfg.Provider = "stub"
	cfg.MaxDuplicateRetries = 0 // exactly 1 one-shot call ⇒ exhaust on "", then multi-turn fires
	cfg.MultiTurnFallback = true
	if chunkTokens > 0 {
		cfg.MultiTurnChunkTokens = chunkTokens // tiny ⇒ N≥2 (cond b true for a large diff)
	}
	providerEntry := map[string]any{
		"command":          bin,
		"prompt_delivery":  "stdin",
		"output":           "raw",
		"strip_code_fence": true,
		"env": map[string]any{
			"STAGEHAND_STUB_SCRIPT":  script,
			"STAGEHAND_STUB_COUNTER": counter,
		},
	}
	if sessionMode != "" {
		providerEntry["session_mode"] = sessionMode // "append" ⇒ cond (d) true
	}
	cfg.Providers = map[string]map[string]any{"stub": providerEntry}
	return cfg
}

// setupScriptedStubRepo builds a temp git repo with one initial commit (headSubject), chdirs into it,
// and returns (repoDir, bin, scriptPath, counterPath) wired for call-varying stub responses. The test
// builds its own config.Config via appendMultiTurnConfig and passes it via Options.Config.
func setupScriptedStubRepo(t *testing.T, headSubject string, responses []string) (string, string, string, string) {
	t.Helper()
	bin := stubtest.Build(t)
	repo := t.TempDir()
	aux := t.TempDir()
	script := aux + "/script.txt"
	if err := os.WriteFile(script, []byte(strings.Join(responses, "\n")), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	counter := aux + "/counter"

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
	return repo, bin, script, counter
}

// TestBuildDeps_NoMessageOverride_InheritsGlobal proves the back-compatible path: with no
// message-role provider override, buildDeps selects the global provider manifest.
func TestBuildDeps_NoMessageOverride_InheritsGlobal(t *testing.T) {
	bin := stubtest.Build(t)
	cfg := config.Defaults()
	cfg.Providers = map[string]map[string]any{
		"alpha": {"command": bin, "prompt_delivery": "stdin", "print_flag": "-p", "output": "raw", "strip_code_fence": true},
		"beta":  {"command": bin, "prompt_delivery": "stdin", "print_flag": "-p", "output": "raw", "strip_code_fence": true},
	}
	cfg.Provider = "alpha"

	deps, err := buildDeps(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("buildDeps: %v", err)
	}
	if deps.Manifest.Name != "alpha" {
		t.Errorf("no-override regression: Manifest.Name = %q, want %q (global)", deps.Manifest.Name, "alpha")
	}
}

// --- FR-T1 multi-turn dry-run tests (P1.M2.T1.S2) — Issue 1 fix ---
//
// These mirror internal/generate/generate_multiturn_test.go + generate_multiturn_failure_test.go at
// the pkg/stagehand (runPipeline / DryRun) boundary. The central fix (Issue 1, FR49): `stagehand
// --dry-run` on a large diff with an append-mode provider now activates multi-turn (previously it
// rescued — exit 1 — where the commit path succeeded via multi-turn).

// TestGenerateCommit_DryRun_MultiTurnSuccess proves Issue 1 is fixed: a DryRun on a large diff whose
// one-shot generation exhausts activates the FR-T1 multi-turn fallback, produces a message, and returns
// it WITHOUT committing (CommitSHA == ""). Before S2 this path rescued (exit 1).
//
// Setup: scripted-append repo (session_mode="append" + multi_turn_chunk_tokens=4); a ~60-line staged
// diff (≈600 tokens ≫ 4 ⇒ cond b true, N≥2). Script responses: one-shot returns "" (exhausts), then
// the multi-turn turns return "ok"/"ok" priming, and the final turn returns the message.
func TestGenerateCommit_DryRun_MultiTurnSuccess(t *testing.T) {
	// Script: call 0 = "" (one-shot parse-fail ⇒ exhaust ⇒ gate fires); calls 1..N-1 = "ok" priming;
	// the FINAL call is clamped-to-last ⇒ turn N+1 emits "feat: multi-turn result".
	repoDir, bin, script, counter := setupScriptedStubRepo(t, "initial",
		[]string{"", "ok", "ok", "feat: multi-turn result"})
	stageLargeDiff(t, repoDir)
	cfg := appendMultiTurnConfig(t, bin, script, counter, "append", 4)

	ctx := context.Background()
	res, err := GenerateCommit(ctx, Options{Config: &cfg, Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun: %v (want nil — multi-turn should win, Issue 1 fix)", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want \"\" (dry-run must not commit)", res.CommitSHA)
	}
	if res.Subject != "feat: multi-turn result" {
		t.Errorf("Subject = %q, want %q (the multi-turn final-turn message)", res.Subject, "feat: multi-turn result")
	}
	if res.Message == "" {
		t.Error("Message empty — multi-turn message did not land")
	}
}

// TestGenerateCommit_DryRun_MultiTurnSkipped_NonAppend proves cond (d) gating: with a NON-append
// provider (session_mode unset), the gate is skipped even on a large diff, and the byte-identical
// rescue fires (FR-T7).
func TestGenerateCommit_DryRun_MultiTurnSkipped_NonAppend(t *testing.T) {
	// session_mode="" ⇒ cond (d) false. Large diff + chunkTokens=4 ⇒ cond (b) true; cond (d) is the
	// ONLY failing condition.
	repoDir, bin, script, counter := setupScriptedStubRepo(t, "initial", []string{""})
	stageLargeDiff(t, repoDir)
	cfg := appendMultiTurnConfig(t, bin, script, counter, "", 4)

	ctx := context.Background()
	_, err := GenerateCommit(ctx, Options{Config: &cfg, Provider: "stub", DryRun: true})
	if err == nil {
		t.Fatal("expected error (non-append ⇒ no multi-turn ⇒ rescue), got nil")
	}
	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *RescueError (non-append ⇒ no multi-turn ⇒ rescue)", err)
	}
	if re.Kind != ErrRescue {
		t.Errorf("re.Kind = %v, want ErrRescue", re.Kind)
	}
}

// TestGenerateCommit_DryRun_MultiTurnSmallPayloadSkip proves cond (b) gating: with an append-mode
// provider but a TINY diff (EstimateTokens(payload) ≤ MultiTurnChunkTokens), the gate is skipped and
// the byte-identical rescue fires.
func TestGenerateCommit_DryRun_MultiTurnSmallPayloadSkip(t *testing.T) {
	// chunkTokens=0 ⇒ appendMultiTurnConfig leaves cfg.MultiTurnChunkTokens at its 32000 default.
	// A 1-line diff ⇒ EstimateTokens ≪ 32000 ⇒ cond (b) false.
	repoDir, bin, script, counter := setupScriptedStubRepo(t, "initial", []string{""})
	writeFile(t, repoDir, "tiny.txt", "hi\n") // tiny diff ⇒ cond (b) false
	stageFile(t, repoDir, "tiny.txt")
	cfg := appendMultiTurnConfig(t, bin, script, counter, "append", 0)

	ctx := context.Background()
	_, err := GenerateCommit(ctx, Options{Config: &cfg, Provider: "stub", DryRun: true})
	if err == nil {
		t.Fatal("expected error (small payload ⇒ no multi-turn ⇒ rescue), got nil")
	}
	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *RescueError (small payload ⇒ no multi-turn ⇒ rescue)", err)
	}
	if re.Kind != ErrRescue {
		t.Errorf("re.Kind = %v, want ErrRescue", re.Kind)
	}
}

// TestGenerateCommit_DryRun_MultiTurnMidTurnFailure proves FR-T7: any multi-turn turn failure → rescue.
// Mechanism (mirrors generate_multiturn_failure_test.go): global STAGEHAND_STUB_EXIT=1 ⇒ the one-shot
// exits 1 but its stdout is "" (script[0]) ⇒ parse-fail ⇒ exhaust ⇒ gate fires ⇒ Run's turn 1 exits 1
// ⇒ Run aborts ⇒ byte-identical rescue.
func TestGenerateCommit_DryRun_MultiTurnMidTurnFailure(t *testing.T) {
	repoDir, bin, script, counter := setupScriptedStubRepo(t, "initial",
		[]string{"", "ok", "ok", "feat: unreachable"})
	stageLargeDiff(t, repoDir)
	cfg := appendMultiTurnConfig(t, bin, script, counter, "append", 4)
	// Global exit-1 ⇒ every stub call exits non-zero. The one-shot's exit-1 is swallowed (non-zero-exit
	// branch falls through to ParseOutput("") ⇒ ok=false ⇒ exhaust); Run's turn-1 exit-1 ⇒ Run aborts
	// (FR-T7) ⇒ rescue.
	cfg.Providers["stub"]["env"] = map[string]any{
		"STAGEHAND_STUB_SCRIPT":  script,
		"STAGEHAND_STUB_COUNTER": counter,
		"STAGEHAND_STUB_EXIT":    "1",
	}

	ctx := context.Background()
	_, err := GenerateCommit(ctx, Options{Config: &cfg, Provider: "stub", DryRun: true})
	if err == nil {
		t.Fatal("expected error (mid-turn failure ⇒ rescue), got nil")
	}
	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *RescueError (FR-T7: turn failure ⇒ rescue)", err)
	}
	if re.Kind != ErrRescue {
		t.Errorf("re.Kind = %v, want ErrRescue (FR-T7 byte-identical rescue)", re.Kind)
	}
	if re.Cause == nil {
		t.Errorf("Cause = nil, want the wrapped *exec.ExitError from the failed multi-turn turn (FR-T7)")
	}
}

// --- Commit-hook wiring tests (P1.M3.T2.S2: runPipeline dry-run + SystemExtra path, FR-V8a/FR-V7) ---
//
// These exercise runPipeline's INSERT A (RunCommitHooks, shared by the dry-run and SystemExtra-commit
// paths) + INSERT B (RunPostCommit, commit-tail-only). They route through the EXPORTED GenerateCommit
// (→ buildDeps, which S1 wires with Hooks: hooks.DefaultRunner{} → runPipeline). The repo's hooks are
// installed into repo/.git/hooks/<name> (mode 0o755 so hookExecutable sees the owner-exec bit).
//
// stderr-capture note (design §6): captureStderr swaps os.Stderr globally. This is SAFE because the
// stagehand_test suite has ZERO t.Parallel() calls — do NOT add t.Parallel() to any of these tests.

// captureStderr runs fn with os.Stderr swapped to a pipe and returns whatever was written. SAFE only
// when the test is NOT t.Parallel() (the stagehand_test suite has none). Restores os.Stderr in t.Cleanup.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = old })
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	fn()
	w.Close()
	return <-done
}

// installHook writes an executable hook script into repo/.git/hooks/<name> (the runner discovers hooks
// via `git rev-parse --git-path hooks` = .git/hooks for a normal init). mode 0o755 ⇒ owner-exec bit set
// (hookExecutable).
func installHook(t *testing.T, repo, name, body string) {
	t.Helper()
	dir := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestGenerateCommit_DryRun_CommitMsgReject_PrintsMessage — FR-V8a: under --dry-run a commit-msg that
// rejects (exit 1 + a stderr lint line) is warn-and-print: the would-be message is carried (Message) AND
// the lint result is surfaced (stderr); exit 0 (err nil); nothing committed.
func TestGenerateCommit_DryRun_CommitMsgReject_PrintsMessage(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: would-be preview"}) // UNIQUE message (not "initial")
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")
	beforeSHA := headSHA(t, repoDir)

	// commit-msg hook: rejects (exit 1) + echoes a distinctive lint line to stderr.
	installHook(t, repoDir, "commit-msg", "#!/bin/sh\necho 'reject: subject not conventional' >&2\nexit 1\n")

	var res Result
	var err error
	stderr := captureStderr(t, func() {
		res, err = GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun + rejecting commit-msg: err=%v, want nil (warn-and-print, exit 0)", err)
	}
	if res.Message == "" {
		t.Errorf("Message empty, want the would-be message printed (FR-V8a)")
	}
	if !strings.Contains(res.Message, "would-be preview") {
		t.Errorf("Message=%q, want it to carry the would-be message", res.Message)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA=%q, want empty (DryRun must not commit)", res.CommitSHA)
	}
	if after := headSHA(t, repoDir); after != beforeSHA {
		t.Errorf("HEAD moved %q→%q (DryRun must not move HEAD)", beforeSHA, after)
	}
	if !strings.Contains(stderr, "reject: subject not conventional") {
		t.Errorf("stderr missing the commit-msg lint result (FR-V8a):\n%s", stderr)
	}
	if !strings.Contains(stderr, "rejected the would-be message") {
		t.Errorf("stderr missing the dry-run rejection notice:\n%s", stderr)
	}
}

// TestGenerateCommit_DryRun_SkipsPreCommit — FR-V8a: pre-commit is SKIPPED under --dry-run. A pre-commit
// that would abort (exit 1) if run must NOT abort the dry-run (it is skipped).
func TestGenerateCommit_DryRun_SkipsPreCommit(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: skip precommit preview"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")

	installHook(t, repoDir, "pre-commit", "#!/bin/sh\necho 'precommit: should not run' >&2\nexit 1\n")

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("DryRun + pre-commit exit 1: err=%v, want nil (pre-commit skipped under --dry-run)", err)
	}
	if res.Message == "" {
		t.Errorf("Message empty, want the would-be message (pre-commit skipped ⇒ no abort)")
	}
}

// TestGenerateCommit_DryRun_CommitMsgAccept — FR-V8a: commit-msg RUNS under --dry-run and an accepting
// (exit 0) hook lets the dry-run succeed (the message is carried).
func TestGenerateCommit_DryRun_CommitMsgAccept(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: accepted preview"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")

	installHook(t, repoDir, "commit-msg", "#!/bin/sh\nexit 0\n")

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("DryRun + accepting commit-msg: err=%v, want nil", err)
	}
	if res.Message == "" {
		t.Errorf("Message empty, want the would-be message (commit-msg accepted under --dry-run)")
	}
}

// TestGenerateCommit_SystemExtra_PreCommitAbort_Rescue — the shared INSERT A's !dryRun branch: a
// SystemExtra real-commit (forces runPipeline's commit tail) with a rejecting pre-commit returns a
// *generate.RescueError (FR-V7) and leaves HEAD unchanged (mirrors S1's CommitStaged).
func TestGenerateCommit_SystemExtra_PreCommitAbort_Rescue(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: systemextra rescue"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")
	beforeSHA := headSHA(t, repoDir)

	installHook(t, repoDir, "pre-commit", "#!/bin/sh\necho 'precommit: blocked' >&2\nexit 1\n")

	_, err := GenerateCommit(context.Background(), Options{Provider: "stub", SystemExtra: "extra instructions"})
	if err == nil {
		t.Fatal("SystemExtra + pre-commit exit 1: err=nil, want a rescue error (FR-V7)")
	}
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Errorf("expected *generate.RescueError (FR-V7), got %T: %v", err, err)
	}
	if after := headSHA(t, repoDir); after != beforeSHA {
		t.Errorf("HEAD moved %q→%q on pre-commit abort (FR-V7 idempotent)", beforeSHA, after)
	}
}

// TestGenerateCommit_DryRun_HookEmptiesMessage_AbortsExit1 — Issue 4 (P1.M3.T1.S2) dry-run guard: a
// commit-msg hook that empties the message file (exit 0 ⇒ not a hook failure, no RescueError) must NOT
// produce an empty dry-run preview (exit 0). git aborts "Aborting commit due to empty commit message.";
// stagehand returns the BARE generate.ErrEmptyMessage (exit 1, NOT the FR-V8a warn-and-print —
// ErrEmptyMessage is not a *RescueError, and this guard sits AFTER the hooks block). The stub Out is
// NON-empty so generation succeeds and the HOOK empties the message (not the generator). FAILS before
// the guard (err==nil, exit 0); PASSES after (errors.Is(err, generate.ErrEmptyMessage)).
func TestGenerateCommit_DryRun_HookEmptiesMessage_AbortsExit1(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: non-empty generated message"}) // non-empty ⇒ generation succeeds
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")

	// commit-msg hook that empties the message file (exit 0 ⇒ not a hook failure; the guard catches the EMPTY result).
	installHook(t, repoDir, "commit-msg", "#!/bin/sh\n> \"$1\"\nexit 0\n")

	_, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err == nil {
		t.Fatal("expected generate.ErrEmptyMessage, got nil (a dry-run with an empty message was produced — the Issue-4 bug)")
	}
	if !errors.Is(err, generate.ErrEmptyMessage) {
		t.Errorf("expected generate.ErrEmptyMessage (bare, exit 1 — NOT the dryRun warn-and-print), got %T: %v", err, err)
	}
}

// TestGenerateCommit_HookEmptiesMessage_NoCommit — Issue 4 (P1.M3.T1.S2) commit-path guard: the same
// emptying hook on runPipeline's commit tail (forced via SystemExtra, since !DryRun && SystemExtra==""
// delegates to CommitStaged, S1's path) → generate.ErrEmptyMessage + NO commit created (HEAD unchanged —
// the abort returned before CommitTree). Bare error → exit 1, NOT a rescue.
func TestGenerateCommit_HookEmptiesMessage_NoCommit(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: non-empty generated message"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "new.txt", "hello")
	stageFile(t, repoDir, "new.txt")

	installHook(t, repoDir, "commit-msg", "#!/bin/sh\n> \"$1\"\nexit 0\n")

	beforeSHA := headSHA(t, repoDir)
	_, err := GenerateCommit(context.Background(), Options{Provider: "stub", SystemExtra: "extra context"}) // SystemExtra ⇒ runPipeline !dryRun
	if err == nil {
		t.Fatal("expected generate.ErrEmptyMessage, got nil (a commit with an empty message was created — the Issue-4 bug)")
	}
	if !errors.Is(err, generate.ErrEmptyMessage) {
		t.Errorf("expected generate.ErrEmptyMessage (bare, exit 1), got %T: %v", err, err)
	}
	// NO commit created (HEAD unchanged — the abort returned before CommitTree).
	if afterSHA := headSHA(t, repoDir); afterSHA != beforeSHA {
		t.Errorf("HEAD moved on empty-message abort: %s → %s (a commit was created)", beforeSHA, afterSHA)
	}
}
