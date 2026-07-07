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

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/provider"
	"github.com/dustin/stagecoach/internal/stubtest"
	"github.com/dustin/stagecoach/internal/ui"
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

// TestCommitStaged_TemplateApplied verifies the §9.19 FR-F8 seam: with cfg.Template set, the
// generated message is templated before it becomes Result.Message/Subject.
func TestCommitStaged_TemplateApplied(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hello world")
	stageFile(t, repo, "new.txt")

	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add login"})
	cfg := config.Defaults()
	cfg.Template = "$msg (#205)"

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	want := "feat: add login (#205)"
	if res.Message != want {
		t.Errorf("Message = %q, want %q (templated)", res.Message, want)
	}
	if res.Subject != want {
		t.Errorf("Subject = %q, want %q (templated)", res.Subject, want)
	}
	logMsg := gitOut(t, repo, "log", "--format=%B", "-n1", res.CommitSHA)
	if logMsg != want {
		t.Errorf("git log message = %q, want %q", logMsg, want)
	}
}

// TestCommitStaged_TemplateSeenByDedupe verifies the §9.7/FR-F8 ordering contract: the duplicate
// check compares the TEMPLATED subject, not the bare generated one. HEAD's subject is already in
// the templated shape ("feat: dup (#205)"); the stub's first bare output templates to that same
// subject and must be rejected, forcing a retry to the second scripted output.
func TestCommitStaged_TemplateSeenByDedupe(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "feat: dup (#205)") // HEAD subject already carries the template shape
	writeFile(t, repo, "a.txt", "data")
	stageFile(t, repo, "a.txt")

	m := stubtest.NewScript(t, bin, []string{"feat: dup", "feat: new"})
	cfg := config.Defaults()
	cfg.Template = "$msg (#205)"

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	want := "feat: new (#205)"
	if res.Subject != want {
		t.Errorf("Subject = %q, want %q (templated bare-duplicate rejected, fresh accepted)", res.Subject, want)
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

	// Move HEAD mid-generation with a commit whose tree DIFFERS from the snapshot (add c.txt on
	// top of the still-staged b.txt), so the CAS error takes the generic "HEAD moved" path rather
	// than the already-committed fast path (covered by TestCommitStaged_CASFailure_AlreadyCommitted).
	writeFile(t, repo, "c.txt", "concurrent change")
	stageFile(t, repo, "c.txt")
	gitOut(t, repo, "commit", "-m", "concurrent commit")
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
	if ce.ActualTree == "" {
		t.Error("CASError.ActualTree is empty, want non-empty (Actual^{tree} re-read on CAS failure)")
	}
	if ce.ActualTree == ce.TreeSHA {
		t.Errorf("ActualTree == TreeSHA (%q) — the concurrent --allow-empty commit must have a DIFFERENT tree than the b.txt snapshot", ce.TreeSHA)
	}
	if !strings.Contains(ce.Error(), "HEAD moved") {
		t.Errorf("CASError.Error() does not contain 'HEAD moved': %s", ce.Error())
	}

	// Atomic HEAD invariant (§20.2): HEAD is the concurrent commit, NOT the orchestrator's.
	if got := headSHA(t, repo); got != concurrent {
		t.Errorf("HEAD = %q, want %q (concurrent commit, orchestrator's should NOT have landed)", got, concurrent)
	}
}

// TestCommitStaged_CASFailure_AlreadyCommitted verifies the friendly fast path: when the commit
// that wins the CAS race carries the SAME tree as our frozen snapshot (the common duplicate-run
// case), CASError.Error() must say "already committed … Nothing to do" and must NOT print the
// manual commit-tree recipe (which would create a DUPLICATE commit). The race is a non-empty
// commit of the same staged b.txt, so its tree == the snapshot tree.
func TestCommitStaged_CASFailure_AlreadyCommitted(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "b.txt", "data")
	stageFile(t, repo, "b.txt")

	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: x", SleepMS: 400})
	cfg := config.Defaults()

	done := make(chan error, 1)
	go func() {
		_, e := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
		done <- e
	}()

	// Let the orchestrator snapshot (freeze b.txt into tree T) + enter generation.
	time.Sleep(150 * time.Millisecond)

	// Race a NON-empty commit of the SAME staged b.txt → a commit whose tree == the snapshot tree T.
	gitOut(t, repo, "commit", "-m", "concurrent same-tree")
	concurrent := headSHA(t, repo)

	err := <-done
	if err == nil {
		t.Fatal("expected error on CAS failure, got nil")
	}
	var ce *CASError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CASError", err)
	}
	if ce.Actual != concurrent {
		t.Errorf("CASError.Actual = %q, want %q", ce.Actual, concurrent)
	}
	if ce.ActualTree != ce.TreeSHA {
		t.Errorf("CASError.ActualTree (%q) != TreeSHA (%q) — the concurrent commit must carry the snapshot tree", ce.ActualTree, ce.TreeSHA)
	}
	// Friendly message: "already committed … Nothing to do"; must NOT offer the duplicate-creating recipe.
	if !strings.Contains(ce.Error(), "already committed") || !strings.Contains(ce.Error(), "Nothing to do") {
		t.Errorf("CASError.Error() = %q, want to contain 'already committed' and 'Nothing to do'", ce.Error())
	}
	if strings.Contains(ce.Error(), "commit-tree") {
		t.Errorf("CASError.Error() must NOT contain the manual commit-tree recipe (would duplicate): %s", ce.Error())
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

// TestCommitStaged_FormatGitmojiLocale_ReachesRenderedPrompt is the stub-agent integration check for
// PRD §9.19 FR-F3/F5/F6 / §17.8: with cfg.Format="gitmoji" and cfg.Locale="French", the REAL rendered
// system prompt (via provider.Render) must contain the gitmoji scaffold instruction, a compiled-in
// RenderGitmojiTable() row, and the locale line — and must NOT contain the auto-mode style-examples
// intro. The stub uses stdin delivery with no system_prompt_flag, so provider.Render prepends the
// system prompt to the payload with a "\n\n" delimiter (render.go) — the captured stdin therefore
// contains the rendered system prompt (mirrors TestCommitStaged_ExcludedPayloadCapture).
func TestCommitStaged_FormatGitmojiLocale_ReachesRenderedPrompt(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "feat: first")
	commitRaw(t, repo, "fix: second") // ≥2 commits → mature path (BuildSystemPrompt, not the fallback)
	writeFile(t, repo, "auth.go", "package main\n")
	stageFile(t, repo, "auth.go")

	stdinFile := filepath.Join(t.TempDir(), "stdin.txt")
	t.Setenv("STAGEHAND_STUB_STDINFILE", stdinFile)
	bin := stubtest.Build(t)
	m := stubtest.Manifest(bin, stubtest.Options{Out: "🎨 refactor auth flow"})

	cfg := config.Defaults()
	cfg.Provider = "stub"
	cfg.Model = "stub"
	cfg.Format = "gitmoji"
	cfg.Locale = "French"

	deps := Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(io.Discard, false)}
	res, err := CommitStaged(context.Background(), deps, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	if res.CommitSHA == "" {
		t.Fatal("expected a commit SHA")
	}

	data, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("read stdin capture: %v", err)
	}
	payload := string(data)

	if !strings.Contains(payload, "Begin the subject with exactly ONE emoji") {
		t.Errorf("expected the gitmoji scaffold instruction in the rendered system prompt, got:\n%s", payload)
	}
	if !strings.Contains(payload, "🎨 - ") {
		t.Errorf("expected a RenderGitmojiTable() row in the rendered system prompt, got:\n%s", payload)
	}
	if !strings.Contains(payload, "Write the commit message in French.") {
		t.Errorf("expected the FR-F6 locale line in the rendered system prompt, got:\n%s", payload)
	}
	if strings.Contains(payload, "Match the tone and style") {
		t.Errorf("gitmoji mode must NOT contain the auto-mode style-examples intro, got:\n%s", payload)
	}
}

// TestCommitStaged_EditGate verifies the --edit integration (§9.22 FR-E1).
// Uses a fake editor script (via GIT_EDITOR env) to rewrite the commit message.
func TestCommitStaged_EditGate(t *testing.T) {
	t.Run("fake editor rewrites message", func(t *testing.T) {
		bin := stubtest.Build(t)
		repo := t.TempDir()
		initRepo(t, repo)
		commitRaw(t, repo, "initial")
		writeFile(t, repo, "new.txt", "hello world")
		stageFile(t, repo, "new.txt")

		// Create a fake editor script that rewrites the message.
		script := filepath.Join(t.TempDir(), "fakeeditor.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'edited subject' > \"$1\"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GIT_EDITOR", script)

		m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add login"})
		cfg := config.Defaults()
		cfg.Edit = true // enable the editor gate

		res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
		if err != nil {
			t.Fatalf("CommitStaged with --edit: %v", err)
		}
		if res.Message != "edited subject" {
			t.Errorf("Message = %q, want 'edited subject' (editor overwrote)", res.Message)
		}
		// Verify it landed in git.
		logMsg := gitOut(t, repo, "log", "--format=%B", "-n1", res.CommitSHA)
		if logMsg != "edited subject" {
			t.Errorf("git log message = %q, want 'edited subject'", logMsg)
		}
	})

	t.Run("fake editor empties message → ErrEmptyMessage", func(t *testing.T) {
		bin := stubtest.Build(t)
		repo := t.TempDir()
		initRepo(t, repo)
		commitRaw(t, repo, "initial")
		writeFile(t, repo, "new.txt", "hello world")
		stageFile(t, repo, "new.txt")

		// Create a fake editor that truncates the file (empty → abort).
		script := filepath.Join(t.TempDir(), "fakeeditor.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\n: > \"$1\"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GIT_EDITOR", script)

		m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add login"})
		cfg := config.Defaults()
		cfg.Edit = true

		headBefore := headSHA(t, repo)
		_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
		if err == nil {
			t.Fatal("CommitStaged with empty editor: expected error, got nil")
		}
		if !errors.Is(err, ErrEmptyMessage) {
			t.Fatalf("CommitStaged error = %v, want ErrEmptyMessage", err)
		}
		// HEAD and index MUST be untouched (abort, not rescue).
		if got := headSHA(t, repo); got != headBefore {
			t.Errorf("HEAD moved from %s to %s — abort must NOT create a commit", headBefore, got)
		}
	})
}

// --- FR-T1 multi-turn fallback trigger gate tests (P1.M1.T3.S3) ---
//
// These focus on the wiring of the FR-T1 gate inside CommitStaged (PRD §9.24):
// when ALL FOUR conditions hold (one-shot exhausted + payload>chunk +
// multi_turn_fallback + session_mode="append"), CommitStaged transparently invokes
// multiturn.Run and either commits the multi-turn message or falls through to the
// EXISTING byte-identical rescue (FR-T7). The exhaustive 4-condition truth table +
// token_limit non-interaction are P1.M1.T3.S4; the integration matrix is P1.M1.T4.

// appendScriptManifest wraps stubtest.NewScript and sets SessionMode="append" so the
// gate's condition (d) and RenderMultiTurn's own gate both pass. NewScript alone
// leaves SessionMode unset (⇒ "" after Resolve).
func appendScriptManifest(t *testing.T, bin string, responses []string) provider.Manifest {
	t.Helper()
	m := stubtest.NewScript(t, bin, responses)
	appendMode := "append"
	m.SessionMode = &appendMode
	return m
}

// TestCommitStaged_MultiTurnFallbackSuccess: one-shot exhausts (call 1 = ""), the
// FR-T1 gate fires (conditions a–d all hold), and the final multi-turn turn returns
// "feat: multi-turn win" → committed. chunkTokens=4 keeps N bounded but > 1.
func TestCommitStaged_MultiTurnFallbackSuccess(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hello world\n")
	stageFile(t, repo, "new.txt")

	m := appendScriptManifest(t, bin, []string{"", "ok", "ok", "feat: multi-turn win"})
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0  // one-shot: 1 attempt (the "")
	cfg.MultiTurnChunkTokens = 4 // small enough that EstimateTokens(payload) > 4 (cond b); N bounded

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v (expected multi-turn success)", err)
	}
	if res.Subject != "feat: multi-turn win" {
		t.Errorf("Subject = %q, want %q (the multi-turn final-turn message)", res.Subject, "feat: multi-turn win")
	}
	if got := headSHA(t, repo); got != res.CommitSHA {
		t.Errorf("HEAD = %q, want %q (commit must land)", got, res.CommitSHA)
	}
}

// TestCommitStaged_MultiTurnSkipped_NonAppend: SessionMode unset (⇒ "") ⇒ condition
// (d) false ⇒ no multi-turn ⇒ the existing rescue fires byte-identically (FR-T7).
func TestCommitStaged_MultiTurnSkipped_NonAppend(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "new.txt", "hello world\n")
	stageFile(t, repo, "new.txt")

	// SessionMode unset (⇒ "") — NO append override. cond (b) would hold, but (d) fails.
	m := stubtest.NewScript(t, bin, []string{""})
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0
	cfg.MultiTurnChunkTokens = 4

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	var re *RescueError
	if !errors.As(err, &re) || re.Kind != ErrRescue {
		t.Fatalf("err = %v, want *RescueError{Kind:ErrRescue} (non-append ⇒ no multi-turn ⇒ rescue)", err)
	}
}

// TestCommitStaged_MultiTurnSkipped_SmallPayload: default chunkTokens (32000) ⇒
// EstimateTokens(payload) ≤ 32000 ⇒ condition (b) false ⇒ a small-payload one-shot
// failure skips multi-turn (FR-T1b) ⇒ rescue.
func TestCommitStaged_MultiTurnSkipped_SmallPayload(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hi\n") // tiny diff
	stageFile(t, repo, "new.txt")

	appendMode := "append"
	m := stubtest.NewScript(t, bin, []string{""})
	m.SessionMode = &appendMode
	cfg := config.Defaults() // MultiTurnChunkTokens=32000 (default) — cond (b) false for a tiny diff
	cfg.MaxDuplicateRetries = 0

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	var re *RescueError
	if !errors.As(err, &re) || re.Kind != ErrRescue {
		t.Fatalf("err = %v, want *RescueError{Kind:ErrRescue} (small payload ⇒ no multi-turn ⇒ rescue)", err)
	}
}

// TestCommitStaged_MultiTurnDuplicateRescue: multi-turn returns a message whose
// subject matches HEAD's → duplicate → rescue carries the finalized Candidate (D3/D7).
func TestCommitStaged_MultiTurnDuplicateRescue(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "feat: dup") // HEAD subject = "feat: dup"
	writeFile(t, repo, "new.txt", "hello world\n")
	stageFile(t, repo, "new.txt")

	m := appendScriptManifest(t, bin, []string{"", "ok", "ok", "feat: dup"})
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0
	cfg.MultiTurnChunkTokens = 4

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	var re *RescueError
	if !errors.As(err, &re) || re.Kind != ErrRescue {
		t.Fatalf("err = %v, want *RescueError{Kind:ErrRescue} (multi-turn duplicate ⇒ rescue)", err)
	}
	if !strings.Contains(re.Candidate, "feat: dup") {
		t.Errorf("Candidate = %q, want it to contain %q", re.Candidate, "feat: dup")
	}
}
