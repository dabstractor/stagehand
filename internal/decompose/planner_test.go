package decompose

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/stubtest"
)

// --- Fixture helpers (own copies — git's and generate's _test.go helpers are unimportable) ---

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

// commitRaw creates an empty commit with the given message.
func commitRaw(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "commit", "--allow-empty", "-m", msg)
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

// plannerDeps builds a minimal Deps for planner tests (no ResolveRoles).
func plannerDeps(t *testing.T, repo string, m provider.Manifest) Deps {
	t.Helper()
	return Deps{
		Git:     git.New(repo),
		Config:  config.Defaults(),
		Roles:   RoleManifests{Planner: m},
		Verbose: nil,
	}
}

// --- Tests ---

const validMultiJSON = `{"count":3,"single":false,"commits":[{"title":"feat: auth","description":"auth.go, auth_test.go"},{"title":"feat: api","description":"api.go"},{"title":"fix: typo","description":"README.md"}]}`

const validSingleJSON = `{"count":1,"single":true,"commits":[{"title":"feat: all","description":"everything"}],"message":"feat: all in one"}`

func TestCallPlanner_HappyMultiCommit(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "new content") // UNSTAGED

	m := stubtest.Manifest(bin, stubtest.Options{Out: validMultiJSON})
	deps := plannerDeps(t, repo, m)

	out, err := callPlanner(context.Background(), deps, 0, false)
	if err != nil {
		t.Fatalf("callPlanner: %v", err)
	}
	if out.Count != 3 {
		t.Errorf("Count = %d, want 3", out.Count)
	}
	if out.Single {
		t.Error("Single = true, want false")
	}
	if len(out.Commits) != 3 {
		t.Errorf("len(Commits) = %d, want 3", len(out.Commits))
	}
	if out.Message != "" {
		t.Errorf("Message = %q, want empty", out.Message)
	}
}

func TestCallPlanner_SingleShortcut(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "new content") // UNSTAGED

	m := stubtest.Manifest(bin, stubtest.Options{Out: validSingleJSON})
	deps := plannerDeps(t, repo, m)

	out, err := callPlanner(context.Background(), deps, 0, false)
	if err != nil {
		t.Fatalf("callPlanner: %v", err)
	}
	if !out.Single {
		t.Error("Single = false, want true")
	}
	if out.Message != "feat: all in one" {
		t.Errorf("Message = %q, want %q", out.Message, "feat: all in one")
	}
	if out.Count != 1 {
		t.Errorf("Count = %d, want 1", out.Count)
	}
}

func TestCallPlanner_ForcedCount(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "new content") // UNSTAGED

	m := stubtest.Manifest(bin, stubtest.Options{Out: validMultiJSON})
	deps := plannerDeps(t, repo, m)

	// forcedCount=3 should still parse normally (BuildPlannerUserPayload handles the directive)
	out, err := callPlanner(context.Background(), deps, 3, false)
	if err != nil {
		t.Fatalf("callPlanner with forcedCount=3: %v", err)
	}
	if out.Count != 3 {
		t.Errorf("Count = %d, want 3", out.Count)
	}
}

func TestCallPlanner_ParseRetryThenSuccess(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "new content") // UNSTAGED

	// First call: invalid JSON; second call: valid JSON
	m := stubtest.NewScript(t, bin, []string{"not valid json{{{", validMultiJSON})
	deps := plannerDeps(t, repo, m)

	out, err := callPlanner(context.Background(), deps, 0, false)
	if err != nil {
		t.Fatalf("callPlanner after retry: %v", err)
	}
	if out.Count != 3 {
		t.Errorf("Count = %d, want 3", out.Count)
	}
	if out.Single {
		t.Error("Single = true, want false")
	}
}

func TestCallPlanner_SingleWithoutMessage_RetryThenSuccess(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "new content") // UNSTAGED

	// First call: single without message (contract fail); second: single with message
	singleNoMsg := `{"count":1,"single":true,"commits":[{"title":"feat: x","description":"a.txt"}]}`
	singleWithMsg := `{"count":1,"single":true,"commits":[{"title":"feat: x","description":"a.txt"}],"message":"feat: x"}`
	m := stubtest.NewScript(t, bin, []string{singleNoMsg, singleWithMsg})
	deps := plannerDeps(t, repo, m)

	out, err := callPlanner(context.Background(), deps, 0, false)
	if err != nil {
		t.Fatalf("callPlanner after retry: %v", err)
	}
	if !out.Single {
		t.Error("Single = false, want true")
	}
	if out.Message != "feat: x" {
		t.Errorf("Message = %q, want %q", out.Message, "feat: x")
	}
}

func TestCallPlanner_UnparseableAfterRetry(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "new content") // UNSTAGED

	// Both calls: invalid JSON
	m := stubtest.NewScript(t, bin, []string{"bad json{{{{", "also bad {{{{"})
	deps := plannerDeps(t, repo, m)

	_, err := callPlanner(context.Background(), deps, 0, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrPlannerFailed) {
		t.Errorf("errors.Is(err, ErrPlannerFailed) = false, error = %v", err)
	}
}

func TestCallPlanner_SingleWithoutMessage_AfterRetry(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "new content") // UNSTAGED

	// Both calls: single without message (contract violation persists)
	singleNoMsg := `{"count":1,"single":true,"commits":[{"title":"feat: x","description":"a.txt"}]}`
	m := stubtest.NewScript(t, bin, []string{singleNoMsg, singleNoMsg})
	deps := plannerDeps(t, repo, m)

	_, err := callPlanner(context.Background(), deps, 0, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrPlannerFailed) {
		t.Errorf("errors.Is(err, ErrPlannerFailed) = false, error = %v", err)
	}
}

func TestCallPlanner_SafetyCap_Auto(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "new content") // UNSTAGED

	cfg := config.Defaults()
	cfg.MaxCommits = 12
	overCapJSON := `{"count":15,"single":false,"commits":[`
	for i := 0; i < 15; i++ {
		if i > 0 {
			overCapJSON += ","
		}
		overCapJSON += `{"title":"c` + strings.Repeat("x", 200) + `","description":"f.txt"}`
	}
	overCapJSON += "]}"

	m := stubtest.Manifest(bin, stubtest.Options{Out: overCapJSON})
	deps := plannerDeps(t, repo, m)
	deps.Config = cfg

	_, err := callPlanner(context.Background(), deps, 0, false)
	if err == nil {
		t.Fatal("expected safety cap error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "planner proposed 15 commits") {
		t.Errorf("error does not contain 'planner proposed 15 commits': %s", errMsg)
	}
	if !strings.Contains(errMsg, "exceeds max_commits (12)") {
		t.Errorf("error does not contain 'exceeds max_commits (12)': %s", errMsg)
	}
	if !strings.Contains(errMsg, "--commits") {
		t.Errorf("error does not contain '--commits': %s", errMsg)
	}
	// Safety cap error is NOT wrapped in ErrPlannerFailed
	if errors.Is(err, ErrPlannerFailed) {
		t.Error("safety cap error should NOT be wrapped in ErrPlannerFailed")
	}
}

func TestCallPlanner_SafetyCap_ForcedSkips(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "new content") // UNSTAGED

	cfg := config.Defaults()
	cfg.MaxCommits = 12
	overCapJSON := `{"count":15,"single":false,"commits":[`
	for i := 0; i < 15; i++ {
		if i > 0 {
			overCapJSON += ","
		}
		overCapJSON += `{"title":"c` + strings.Repeat("x", 200) + `","description":"f.txt"}`
	}
	overCapJSON += "]}"

	m := stubtest.Manifest(bin, stubtest.Options{Out: overCapJSON})
	deps := plannerDeps(t, repo, m)
	deps.Config = cfg

	// forcedCount=15 > 0 ⇒ cap is bypassed (forced mode trusts --commits)
	out, err := callPlanner(context.Background(), deps, 15, false)
	if err != nil {
		t.Fatalf("callPlanner with forcedCount=15 should skip cap: %v", err)
	}
	if out.Count != 15 {
		t.Errorf("Count = %d, want 15", out.Count)
	}
}

func TestCallPlanner_Timeout(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "new content") // UNSTAGED

	m := stubtest.Manifest(bin, stubtest.Options{Out: validMultiJSON, SleepMS: 2000})
	cfg := config.Defaults()
	cfg.Timeout = 100 * time.Millisecond

	deps := plannerDeps(t, repo, m)
	deps.Config = cfg

	_, err := callPlanner(context.Background(), deps, 0, false)
	if err == nil {
		t.Fatal("expected error on timeout, got nil")
	}
	if !errors.Is(err, ErrPlannerFailed) {
		t.Errorf("errors.Is(err, ErrPlannerFailed) = false, error = %v", err)
	}
	// The %w chain should reach context.DeadlineExceeded
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, error = %v", err)
	}
}

func TestCallPlanner_UnbornNilExamples(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "new content") // UNSTAGED, no commits → unborn

	m := stubtest.Manifest(bin, stubtest.Options{Out: validSingleJSON})
	deps := plannerDeps(t, repo, m)

	// isUnborn=true → plannerExamples short-circuits to nil
	// BuildPlannerSystemPrompt(nil) is nil-safe — should not panic
	out, err := callPlanner(context.Background(), deps, 0, true)
	if err != nil {
		t.Fatalf("callPlanner with isUnborn=true: %v", err)
	}
	if !out.Single {
		t.Error("Single = false, want true")
	}
	if out.Message != "feat: all in one" {
		t.Errorf("Message = %q, want %q", out.Message, "feat: all in one")
	}
}
