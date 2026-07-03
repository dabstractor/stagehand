package generate

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/stubtest"
	"github.com/dustin/stagehand/internal/ui"
)

// --- Fixture helpers (own copies — git's _test.go helpers are unimportable) ---

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

// --- Tests ---

// TestCommitStaged_Success verifies the happy path: a repo with an initial commit,
// a staged file, and a stub that returns "feat: add login". Asserts Result fields,
// HEAD moved to CommitSHA, commit message round-trips, and Changes is non-empty.
func TestCommitStaged_Success(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hello world")
	stageFile(t, repo, "new.txt")

	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add login"})
	cfg := config.Defaults()

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}

	if !shaRe.MatchString(res.CommitSHA) {
		t.Errorf("CommitSHA = %q, want hex SHA", res.CommitSHA)
	}
	if res.Subject != "feat: add login" {
		t.Errorf("Subject = %q, want %q", res.Subject, "feat: add login")
	}
	if res.Message != "feat: add login" {
		t.Errorf("Message = %q, want %q", res.Message, "feat: add login")
	}
	if res.Provider != "stub" {
		t.Errorf("Provider = %q, want %q", res.Provider, "stub")
	}
	if len(res.Changes) == 0 {
		t.Error("Changes is empty, want non-empty")
	}
	if got := headSHA(t, repo); got != res.CommitSHA {
		t.Errorf("HEAD = %q, want %q", got, res.CommitSHA)
	}
	logMsg := gitOut(t, repo, "log", "--format=%B", "-n1", res.CommitSHA)
	if logMsg != "feat: add login" {
		t.Errorf("git log message = %q, want %q", logMsg, "feat: add login")
	}
}

// TestCommitStaged_DedupeRetryThenSuccess verifies that a duplicate subject is rejected
// (FR30/FR32) and the next attempt's fresh subject is accepted. Uses NewScript with
// ["feat: existing", "feat: fresh"] on a repo whose HEAD subject is "feat: existing".
func TestCommitStaged_DedupeRetryThenSuccess(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "feat: existing") // HEAD subject = "feat: existing"
	writeFile(t, repo, "a.txt", "data")
	stageFile(t, repo, "a.txt")

	m := stubtest.NewScript(t, bin, []string{"feat: existing", "feat: fresh"})
	cfg := config.Defaults()

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	if res.Subject != "feat: fresh" {
		t.Errorf("Subject = %q, want %q (duplicate should have been rejected)", res.Subject, "feat: fresh")
	}
	if got := headSHA(t, repo); got != res.CommitSHA {
		t.Errorf("HEAD = %q, want %q", got, res.CommitSHA)
	}
}

// TestCommitStaged_ParseFailRescue verifies that an empty stub output (ok=false)
// with MaxDuplicateRetries=0 triggers a *RescueError{Kind:ErrRescue}. Asserts
// HEAD + index are unchanged (idempotent-index invariant, §20.2).
func TestCommitStaged_ParseFailRescue(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "x.txt", "data")
	stageFile(t, repo, "x.txt")

	m := stubtest.NewScript(t, bin, []string{"", "feat: good"})
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0 // single attempt → blank → loop exhausted → rescue

	beforeHEAD := headSHA(t, repo)
	beforeIndex := gitOut(t, repo, "diff", "--cached", "--name-only")

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err == nil {
		t.Fatal("expected error on parse-fail rescue, got nil")
	}

	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("error type = %T, want *RescueError", err)
	}
	if !errors.Is(err, ErrRescue) {
		t.Errorf("errors.Is(err, ErrRescue) = false, want true")
	}
	if re.TreeSHA == "" {
		t.Error("RescueError.TreeSHA is empty, want non-empty (snapshot was taken)")
	}
	if re.ParentSHA == "" {
		t.Error("RescueError.ParentSHA is empty, want non-empty (born repo)")
	}

	// Idempotent-index invariant (§20.2): HEAD + index unchanged.
	if got := headSHA(t, repo); got != beforeHEAD {
		t.Errorf("HEAD changed from %q to %q, want unchanged", beforeHEAD, got)
	}
	afterIndex := gitOut(t, repo, "diff", "--cached", "--name-only")
	if afterIndex != beforeIndex {
		t.Errorf("index changed: before=%q after=%q, want unchanged", beforeIndex, afterIndex)
	}

	// FormatRescue renders without panic.
	rescue := FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate)
	if rescue == "" {
		t.Error("FormatRescue returned empty string")
	}
}

// TestCommitStaged_CASFailure verifies that HEAD moved mid-generation results in
// *CASError with the correct Expected/Actual. Uses a stub with SleepMS=400, races
// a concurrent commit into HEAD during generation, and asserts the orchestrator's
// commit did NOT land (atomic HEAD invariant, §20.2).
func TestCommitStaged_CASFailure(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "b.txt", "data")
	stageFile(t, repo, "b.txt")

	parent := headSHA(t, repo)
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: x", SleepMS: 400})
	cfg := config.Defaults()

	done := make(chan error, 1)
	go func() {
		_, e := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
		done <- e
	}()

	// Let the orchestrator snapshot + enter generation (stub sleeping 400ms).
	time.Sleep(150 * time.Millisecond)

	// Move HEAD mid-generation.
	commitRaw(t, repo, "concurrent commit")
	concurrent := headSHA(t, repo)

	err := <-done
	if err == nil {
		t.Fatal("expected error on CAS failure, got nil")
	}

	var ce *CASError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CASError", err)
	}
	if !errors.Is(err, git.ErrCASFailed) {
		t.Errorf("errors.Is(err, git.ErrCASFailed) = false, want true")
	}
	if !errors.Is(err, ErrCASFailed) {
		t.Errorf("errors.Is(err, ErrCASFailed) = false, want true")
	}
	if ce.Expected != parent {
		t.Errorf("CASError.Expected = %q, want %q", ce.Expected, parent)
	}
	if ce.Actual != concurrent {
		t.Errorf("CASError.Actual = %q, want %q", ce.Actual, concurrent)
	}
	if ce.TreeSHA == "" {
		t.Error("CASError.TreeSHA is empty, want non-empty")
	}
	if !strings.Contains(ce.Error(), "HEAD moved") {
		t.Errorf("CASError.Error() does not contain 'HEAD moved': %s", ce.Error())
	}

	// Atomic HEAD invariant (§20.2): HEAD is the concurrent commit, NOT the orchestrator's.
	if got := headSHA(t, repo); got != concurrent {
		t.Errorf("HEAD = %q, want %q (concurrent commit, orchestrator's should NOT have landed)", got, concurrent)
	}
}

// TestCommitStaged_RootCommit verifies an unborn repo (no commits) succeeds with a
// parentless root commit. The commit object lacks a "parent" line, DiffTree ran with
// isRoot=true, and HEAD points to the new commit.
func TestCommitStaged_RootCommit(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo) // UNBORN — no commits yet
	writeFile(t, repo, "first.txt", "content")
	stageFile(t, repo, "first.txt")

	m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: initial"})
	cfg := config.Defaults()

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	if res.Subject != "chore: initial" {
		t.Errorf("Subject = %q, want %q", res.Subject, "chore: initial")
	}
	if got := headSHA(t, repo); got != res.CommitSHA {
		t.Errorf("HEAD = %q, want %q", got, res.CommitSHA)
	}

	// Verify no parent line in the commit object.
	catfile := gitOut(t, repo, "cat-file", "-p", res.CommitSHA)
	if strings.Contains(catfile, "\nparent ") {
		t.Errorf("root commit has a parent line: %s", catfile)
	}

	// DiffTree ran with isRoot=true → Changes should be non-empty.
	if len(res.Changes) == 0 {
		t.Error("Changes is empty, want non-empty (root commit DiffTree with --root)")
	}
}

// TestCommitStaged_NothingToCommit verifies that an empty staged diff returns
// ErrNothingToCommit as a bare sentinel (not wrapped in *RescueError).
func TestCommitStaged_NothingToCommit(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	// Nothing staged.

	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: x"})
	cfg := config.Defaults()

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("errors.Is(err, ErrNothingToCommit) = false, error = %v", err)
	}
	var re *RescueError
	if errors.As(err, &re) {
		t.Error("error should NOT be a *RescueError for nothing-to-commit")
	}
}

// TestCommitStaged_Timeout verifies that cfg.Timeout exceeded returns
// *RescueError{Kind:ErrTimeout} (NOT ErrRescue), with a non-empty TreeSHA,
// and HEAD remains unchanged.
func TestCommitStaged_Timeout(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "z.txt", "data")
	stageFile(t, repo, "z.txt")

	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: slow", SleepMS: 2000})
	cfg := config.Defaults()
	cfg.Timeout = 150 * time.Millisecond

	beforeHEAD := headSHA(t, repo)

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err == nil {
		t.Fatal("expected error on timeout, got nil")
	}

	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("error type = %T, want *RescueError", err)
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("errors.Is(err, ErrTimeout) = false, want true (got ErrRescue instead?)")
	}
	if errors.Is(err, ErrRescue) {
		t.Error("errors.Is(err, ErrRescue) = true, want false (should be ErrTimeout)")
	}
	if re.TreeSHA == "" {
		t.Error("RescueError.TreeSHA is empty, want non-empty (snapshot was taken)")
	}

	// HEAD unchanged.
	if got := headSHA(t, repo); got != beforeHEAD {
		t.Errorf("HEAD changed from %q to %q on timeout, want unchanged", beforeHEAD, got)
	}
}

// TestCommitStaged_IdempotentIndexOnFailure verifies the §20.2 invariant: after a
// rescue path, the index is byte-for-byte unchanged (CommitStaged never calls
// git add). Checks both HEAD and staged files.
func TestCommitStaged_IdempotentIndexOnFailure(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "c.txt", "aaa")
	stageFile(t, repo, "c.txt")

	m := stubtest.NewScript(t, bin, []string{""})
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0 // exhaust immediately

	beforeHEAD := headSHA(t, repo)
	beforeIndex := gitOut(t, repo, "diff", "--cached", "--name-only")
	beforeIndexFull := gitOut(t, repo, "diff", "--cached")

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// HEAD unchanged.
	if got := headSHA(t, repo); got != beforeHEAD {
		t.Errorf("HEAD changed: before=%s after=%s", beforeHEAD, got)
	}
	// Staged file list unchanged.
	afterIndex := gitOut(t, repo, "diff", "--cached", "--name-only")
	if afterIndex != beforeIndex {
		t.Errorf("staged files changed: before=%q after=%q", beforeIndex, afterIndex)
	}
	// Full staged diff unchanged (byte-for-byte).
	afterIndexFull := gitOut(t, repo, "diff", "--cached")
	if afterIndexFull != beforeIndexFull {
		t.Error("staged diff content changed (byte-for-byte mismatch)")
	}
}

// sliceContains reports whether args contains s.
func sliceContains(args []string, s string) bool {
	for _, a := range args {
		if a == s {
			return true
		}
	}
	return false
}

// TestCommitStaged_MessageRoleOverride verifies that per-role [role.message] overrides
// (Model + Reasoning) reach Render end-to-end AND Result.Model reports the resolved model.
// Uses the STAGEHAND_STUB_ARGSFILE knob to observe the exact rendered argv from the stub.
func TestCommitStaged_MessageRoleOverride(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hello")
	stageFile(t, repo, "new.txt")

	argsFile := filepath.Join(t.TempDir(), "args")
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add login", ArgsFile: argsFile})
	pflag := "--model"
	m.ModelFlag = &pflag
	m.ReasoningLevels = map[string][]string{"high": {"--thinking", "high"}}
	dm := "gpt-5.4"
	m.DefaultModel = &dm

	cfg := config.Defaults()
	cfg.Roles = map[string]config.RoleConfig{"message": {Model: "haiku", Reasoning: "high"}}
	// cfg.Model=="" && cfg.Reasoning=="" → ResolveRoleModel returns the role overrides

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}

	// Result.Model must reflect the message role override.
	if res.Model != "haiku" {
		t.Errorf("Result.Model = %q, want %q", res.Model, "haiku")
	}

	// Verify the rendered argv contains --model haiku and the reasoning token.
	raw, _ := os.ReadFile(argsFile)
	args := strings.Split(string(raw), "\x00")
	if !sliceContains(args, "--model") || !sliceContains(args, "haiku") {
		t.Errorf("Render did not receive message model haiku; args=%v", args)
	}
	if !sliceContains(args, "--thinking") || !sliceContains(args, "high") {
		t.Errorf("Render did not receive message reasoning high; args=%v", args)
	}
}

// TestCommitStaged_NoMessageOverride_Regression verifies that with cfg.Roles empty (no per-role
// override), CommitStaged passes cfg.Model to Render and Result.Model == cfg.Model — identical
// to the pre-fix behavior (back-compat regression guard).
func TestCommitStaged_NoMessageOverride_Regression(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hello")
	stageFile(t, repo, "new.txt")

	argsFile := filepath.Join(t.TempDir(), "args")
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add login", ArgsFile: argsFile})

	pflag := "--model"
	m.ModelFlag = &pflag

	cfg := config.Defaults()
	cfg.Model = "openrouter/gpt-5.4"
	// cfg.Roles == nil → ResolveRoleModel returns ("", cfg.Model, "") — back-compat

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}

	if res.Model != "openrouter/gpt-5.4" {
		t.Errorf("Result.Model = %q, want %q", res.Model, "openrouter/gpt-5.4")
	}

	raw, _ := os.ReadFile(argsFile)
	args := strings.Split(string(raw), "\x00")
	if !sliceContains(args, "--model") || !sliceContains(args, "openrouter/gpt-5.4") {
		t.Errorf("Render did not receive global model; args=%v", args)
	}
	if sliceContains(args, "--thinking") {
		t.Errorf("Render unexpectedly has a reasoning token; args=%v", args)
	}
}

// TestCommitStaged_ResolvesSubProviderFromManifest verifies that CommitStaged passes "" (not
// cfg.Provider) to Render, so the manifest's merged DefaultProvider is emitted as --provider.
// cfg.Provider="pi" is the manifest/agent NAME; it must NOT appear in the rendered command.
func TestCommitStaged_ResolvesSubProviderFromManifest(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "f.txt", "content")
	stageFile(t, repo, "f.txt")

	// Pi-shaped stub: ProviderFlag triggers slash-prefix splitting in Render.
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: provider ok"})
	pflag := "--provider"
	m.ProviderFlag = &pflag
	mf := "--model"
	m.ModelFlag = &mf
	dm := "gpt-5.4"
	m.DefaultModel = &dm

	cfg := config.Defaults()
	cfg.Provider = "pi"              // the manifest NAME — the conflation source; must NOT be emitted
	cfg.Model = "openrouter/gpt-5.4" // slash-prefix model → Render emits --provider openrouter

	var buf bytes.Buffer
	deps := Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true)}

	res, err := CommitStaged(context.Background(), deps, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	if res.Subject != "feat: provider ok" {
		t.Errorf("Subject = %q, want %q", res.Subject, "feat: provider ok")
	}

	cmd := buf.String()
	if !strings.Contains(cmd, "--provider openrouter") {
		t.Errorf("rendered command missing --provider openrouter\ngot: %s", cmd)
	}
	if strings.Contains(cmd, "--provider pi") {
		t.Errorf("rendered command emits the manifest name as sub-provider (conflation bug)\ngot: %s", cmd)
	}
}

func TestCommitStaged_ExcludedPayloadCapture(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "feature.go", "package main\n")
	stageFile(t, repo, "feature.go")
	writeFile(t, repo, "secret.conf", "password=abc\n")
	stageFile(t, repo, "secret.conf")

	stdinFile := filepath.Join(t.TempDir(), "stdin.txt")
	t.Setenv("STAGEHAND_STUB_STDINFILE", stdinFile)
	stub := stubtest.Build(t)
	m := stubtest.Manifest(stub, stubtest.Options{Out: "feat: add feature"})

	cfg := config.Config{
		Provider: "stub",
		Model:    "stub",
		Timeout:  30 * time.Second,
	}
	deps := Deps{
		Git:      git.New(repo),
		Manifest: m,
		Verbose:  ui.NewVerbose(io.Discard, false),
		Excludes: []string{":(exclude,glob)**/secret.conf"},
	}

	res, err := CommitStaged(context.Background(), deps, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	if res.CommitSHA == "" {
		t.Fatal("expected a commit SHA")
	}

	// Read the captured stdin payload.
	data, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("read stdin capture: %v", err)
	}
	payload := string(data)

	// secret.conf: hunk absent, placeholder present
	if strings.Contains(payload, "diff --git a/secret.conf") {
		t.Fatalf("expected secret.conf hunk ABSENT from payload, got:\n%s", payload)
	}
	if !strings.Contains(payload, "[excluded] secret.conf") {
		t.Fatalf("expected [excluded] placeholder for secret.conf, got:\n%s", payload)
	}
	// feature.go present
	if !strings.Contains(payload, "feature.go") {
		t.Fatalf("expected feature.go present in payload, got:\n%s", payload)
	}
}
