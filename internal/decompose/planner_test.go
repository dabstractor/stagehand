package decompose

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagecoach/internal/ui"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/provider"
	"github.com/dustin/stagecoach/internal/stubtest"
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

// freezeForPlanner captures baseTree + tStart for a callPlanner test (matures: rev-parse HEAD^{tree};
// unborn: EmptyTreeSHA). Mirrors what Decompose() does after baseTree derivation.
func freezeForPlanner(t *testing.T, repo string, isUnborn bool) (baseTree, tStart string) {
	t.Helper()
	if isUnborn {
		baseTree = git.EmptyTreeSHA
	} else {
		baseTree = runGit(t, repo, "rev-parse", "HEAD^{tree}")
	}
	g := git.New(repo)
	ts, err := g.FreezeWorkingTree(context.Background(), baseTree)
	if err != nil {
		t.Fatalf("freeze working tree: %v", err)
	}
	return baseTree, ts
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
	baseTree, tStart := freezeForPlanner(t, repo, false)

	out, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
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
	baseTree, tStart := freezeForPlanner(t, repo, false)

	out, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
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
	baseTree, tStart := freezeForPlanner(t, repo, false)

	// forcedCount=3 should still parse normally (BuildPlannerUserPayload handles the directive)
	out, err := callPlanner(context.Background(), deps, 3, false, baseTree, tStart)
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
	baseTree, tStart := freezeForPlanner(t, repo, false)

	out, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
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
	baseTree, tStart := freezeForPlanner(t, repo, false)

	out, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
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
	baseTree, tStart := freezeForPlanner(t, repo, false)

	_, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
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
	baseTree, tStart := freezeForPlanner(t, repo, false)

	_, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
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
	baseTree, tStart := freezeForPlanner(t, repo, false)

	_, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
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
	baseTree, tStart := freezeForPlanner(t, repo, false)

	// forcedCount=15 > 0 ⇒ cap is bypassed (forced mode trusts --commits)
	out, err := callPlanner(context.Background(), deps, 15, false, baseTree, tStart)
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
	baseTree, tStart := freezeForPlanner(t, repo, false)

	_, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
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
	baseTree, tStart := freezeForPlanner(t, repo, true)

	// isUnborn=true → plannerExamples short-circuits to nil
	// BuildPlannerSystemPrompt(nil) is nil-safe — should not panic
	out, err := callPlanner(context.Background(), deps, 0, true, baseTree, tStart)
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

func TestCallPlanner_ResolvesSubProvider(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "new content") // UNSTAGED (callPlanner reads WorkingTreeDiff)

	m := stubtest.Manifest(bin, stubtest.Options{Out: validMultiJSON})
	pflag := "--provider"
	m.ProviderFlag = &pflag // pi-shaped: ProviderFlag triggers slash-prefix splitting
	mf := "--model"
	m.ModelFlag = &mf
	dm := "gpt-5.4"
	m.DefaultModel = &dm

	deps := plannerDeps(t, repo, m)
	deps.Config.Provider = "pi"              // the manifest NAME — the conflation source; must NOT be emitted
	deps.Config.Model = "openrouter/gpt-5.4" // slash-prefix model → Render emits --provider openrouter
	baseTree, tStart := freezeForPlanner(t, repo, false)

	var buf bytes.Buffer
	deps.Verbose = ui.NewVerbose(&buf, true)

	out, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
	if err != nil {
		t.Fatalf("callPlanner: %v", err)
	}
	_ = out

	cmd := buf.String()
	if !strings.Contains(cmd, "--provider openrouter") {
		t.Errorf("planner command missing --provider openrouter\ngot: %s", cmd)
	}
	if strings.Contains(cmd, "--provider pi") {
		t.Errorf("planner command emits manifest name as sub-provider (conflation)\ngot: %s", cmd)
	}
}

// TestCallPlanner_DiffsFrozenTStart verifies that the planner diffs the FROZEN T_start,
// not the live working tree. A file written to the working tree AFTER the freeze is absent
// from the planner's diff payload (exclusion test for FR-M1b).
func TestCallPlanner_DiffsFrozenTStart(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "changed content") // UNSTAGED

	// Build a planner stub that captures its stdin payload.
	captureDir := t.TempDir()
	captureFile := captureDir + "/payload"
	captureScript := captureDir + "/capture.sh"
	if err := os.WriteFile(captureScript, []byte(`#!/bin/sh
cat > "`+captureFile+`"
echo '`+validSingleJSON+`'`), 0o755); err != nil {
		t.Fatalf("write capture script: %v", err)
	}
	m := stubtest.Manifest(bin, stubtest.Options{Out: validSingleJSON})
	m.Command = &captureScript

	deps := plannerDeps(t, repo, m)

	// Freeze — captures baseTree + tStart BEFORE the sentinel exists.
	baseTree, tStart := freezeForPlanner(t, repo, false)

	// AFTER the freeze: write a sentinel file (simulates a concurrent change).
	writeFile(t, repo, "sentinel.txt", "concurrent")

	_, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
	if err != nil {
		t.Fatalf("callPlanner: %v", err)
	}

	// Verify the sentinel is absent from the captured payload.
	payload, err := os.ReadFile(captureFile)
	if err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if strings.Contains(string(payload), "sentinel.txt") {
		t.Errorf("planner diff contains 'sentinel.txt' — the freeze should exclude post-freeze changes\npayload: %s", string(payload))
	}
	// Verify a.txt IS in the payload (the pre-freeze change is captured).
	if !strings.Contains(string(payload), "a.txt") {
		t.Errorf("planner diff missing 'a.txt' — the pre-freeze change should be captured\npayload: %s", string(payload))
	}
}
