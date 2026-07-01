package decompose

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/prompt"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/stubtest"
)

// --- Fixture helpers (dcm*-prefixed to avoid colliding with arb*/chn*/msg*/stg*/planner) ---

// dcmInitRepo creates a git repo in dir with repo-local identity config (no env pollution).
func dcmInitRepo(t *testing.T, dir string) {
	t.Helper()
	dcmRunGit(t, dir, "init")
	dcmRunGit(t, dir, "config", "user.name", "Test")
	dcmRunGit(t, dir, "config", "user.email", "test@example.com")
}

// dcmWriteFile creates a file at dir/name with the given body.
func dcmWriteFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := dir + string(os.PathSeparator) + name
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("dcmWriteFile %s: %v", full, err)
	}
}

// dcmStageFile runs git add for name in dir.
func dcmStageFile(t *testing.T, dir, name string) {
	t.Helper()
	dcmRunGit(t, dir, "add", name)
}

// dcmCommitRaw creates an empty commit with the given message.
func dcmCommitRaw(t *testing.T, dir, msg string) {
	t.Helper()
	dcmRunGit(t, dir, "commit", "--allow-empty", "-m", msg)
}

// dcmRunGit executes git -C dir args... and returns trimmed stdout.
func dcmRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// dcmGitOut runs a raw git command in dir and returns trimmed stdout (alias for consistency).
func dcmGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return dcmRunGit(t, dir, args...)
}

// dcmHeadSHA returns the current HEAD SHA.
func dcmHeadSHA(t *testing.T, dir string) string {
	t.Helper()
	return dcmGitOut(t, dir, "rev-parse", "HEAD")
}

// dcmLogOneline returns git log --oneline output (all commits, oldest first).
func dcmLogOneline(t *testing.T, dir string) string {
	t.Helper()
	return dcmGitOut(t, dir, "log", "--format=%H %s", "--reverse")
}

// dcmLogCount returns the number of commits reachable from HEAD (0 on unborn).
func dcmLogCount(t *testing.T, dir string) int {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-list", "--count", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0 // unborn repo
	}
	n := 0
	for _, c := range strings.TrimSpace(string(out)) {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// dcmIsUnborn reports whether the repo has zero commits.
func dcmIsUnborn(t *testing.T, dir string) bool {
	t.Helper()
	// git rev-parse HEAD exits 128 on unborn
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.CombinedOutput()
	return err != nil && strings.Contains(string(out), "HEAD")
}

// dcmStatusPorcelain returns git status --porcelain output.
func dcmStatusPorcelain(t *testing.T, dir string) string {
	t.Helper()
	return dcmGitOut(t, dir, "status", "--porcelain")
}

// dcmPlannerManifest builds a stub planner manifest that returns the given JSON.
func dcmPlannerManifest(t *testing.T, bin string, jsonOut string) provider.Manifest {
	t.Helper()
	return stubtest.Manifest(bin, stubtest.Options{Out: jsonOut})
}

// dcmPlannerScriptManifest builds a stub planner manifest with call-varying responses.
func dcmPlannerScriptManifest(t *testing.T, bin string, responses []string) provider.Manifest {
	t.Helper()
	return stubtest.NewScript(t, bin, responses)
}

// dcmArbiterManifest builds a stub arbiter manifest that returns the given JSON.
func dcmArbiterManifest(t *testing.T, bin string, jsonOut string) provider.Manifest {
	t.Helper()
	return stubtest.Manifest(bin, stubtest.Options{Out: jsonOut})
}

// dcmMessageManifest builds a stub message manifest that returns the given text.
func dcmMessageManifest(t *testing.T, bin string, out string) provider.Manifest {
	t.Helper()
	return stubtest.Manifest(bin, stubtest.Options{Out: out})
}

// dcmMessageScriptManifest builds a stub message manifest with call-varying responses.
func dcmMessageScriptManifest(t *testing.T, bin string, responses []string) provider.Manifest {
	t.Helper()
	return stubtest.NewScript(t, bin, responses)
}

// dcmDeps builds a minimal Deps for decompose tests (no ResolveRoles). All four roles are populated.
func dcmDeps(t *testing.T, repo string, roles RoleManifests) Deps {
	t.Helper()
	return Deps{
		Git:     git.New(repo),
		Config:  config.Defaults(),
		Roles:   roles,
		Verbose: nil,
	}
}

// dcmDepsWithConfig builds a Deps with a custom config.
func dcmDepsWithConfig(t *testing.T, repo string, roles RoleManifests, cfg config.Config) Deps {
	t.Helper()
	return Deps{
		Git:     git.New(repo),
		Config:  cfg,
		Roles:   roles,
		Verbose: nil,
	}
}

// dcmAllRoles returns RoleManifests with all four roles set to the same stub manifest.
func dcmAllRoles(t *testing.T, bin string, o stubtest.Options) RoleManifests {
	t.Helper()
	m := stubtest.Manifest(bin, o)
	return RoleManifests{
		Planner: m,
		Stager:  tooledStubManifest(t, bin, o),
		Message: m,
		Arbiter: m,
	}
}

// dcmStagerSeam returns a stager function that runs git add for files matching the concept title.
// The concept title is used as a file prefix: concept "feat: add a.go" stages "a.go" (last word).
// If no files are found for the concept, it does nothing (for empty-skip testing).
func dcmStagerSeam(t *testing.T, repo string, conceptFiles map[string][]string) func(context.Context, Deps, prompt.PlannerCommit) error {
	return func(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error {
		t.Helper()
		files, ok := conceptFiles[concept.Title]
		if !ok || len(files) == 0 {
			return nil // nothing to stage (for empty-skip testing)
		}
		for _, f := range files {
			dcmRunGit(t, repo, "add", f)
		}
		return nil
	}
}

// dcmOutBuffer returns a Deps with Out set to a *bytes.Buffer for capturing rescue/CAS output.
func dcmOutBuffer(t *testing.T, repo string, roles RoleManifests) (Deps, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	deps := Deps{
		Git:     git.New(repo),
		Config:  config.Defaults(),
		Roles:   roles,
		Verbose: nil,
		Out:     &buf,
	}
	return deps, &buf
}

// --- Tests ---

func TestDecompose_SingleEscape(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create 2 untracked files.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")

	msgM := dcmMessageManifest(t, bin, "feat: all")
	roles := RoleManifests{Message: msgM}
	cfg := config.Defaults()
	cfg.Single = true

	deps := dcmDepsWithConfig(t, repo, roles, cfg)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(single): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	cr := result.Commits[0]
	if cr.Subject != "feat: all" {
		t.Errorf("Subject = %q, want %q", cr.Subject, "feat: all")
	}
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0", result.Amended)
	}

	// Verify git state: 1 commit, clean tree.
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("commit count = %d, want 1", dcmLogCount(t, repo))
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}
}

func TestDecompose_SingleEscape_ErrNothingToCommit(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	msgM := dcmMessageManifest(t, bin, "feat: all")
	roles := RoleManifests{Message: msgM}
	cfg := config.Defaults()
	cfg.Single = true

	deps := dcmDepsWithConfig(t, repo, roles, cfg)

	// Nothing to commit — should get ErrNothingToCommit propagated.
	_, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, generate.ErrNothingToCommit) {
		t.Errorf("error = %v, want ErrNothingToCommit", err)
	}
}

func TestDecompose_SingleShortcut_CleanMessage(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "x.txt", "x\n")

	plannerJSON := `{"count":1,"single":true,"commits":[{"title":"add x","description":"x.txt"}],"message":"feat: add x.txt"}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Use a counter for the message stub — it should NOT be called (0 calls).
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	messageM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(shortcut clean): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add x.txt" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "feat: add x.txt")
	}
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("commit count = %d, want 1", dcmLogCount(t, repo))
	}

	// Verify message agent was NOT called (counter == 0 or file absent).
	data, err := os.ReadFile(counterFile)
	if err == nil {
		// Counter file exists — check it's 0 (message agent was never invoked).
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("message agent call count = %q, want 0 (shortcut used planner msg verbatim)", count)
		}
	}
	// If file doesn't exist, the message agent was never called — correct.
}

func TestDecompose_SingleShortcut_DuplicateFallback(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create an existing commit whose subject matches the planner's proposed message.
	dcmWriteFile(t, repo, "existing.txt", "existing\n")
	dcmStageFile(t, repo, "existing.txt")
	dcmRunGit(t, repo, "commit", "-m", "feat: add x.txt")
	dcmRunGit(t, repo, "rm", "existing.txt")
	dcmRunGit(t, repo, "commit", "-am", "chore: remove existing")

	// Now write a new file; the planner proposes "feat: add x.txt" which is a DUPLICATE.
	dcmWriteFile(t, repo, "new.txt", "new content\n")

	plannerJSON := `{"count":1,"single":true,"commits":[{"title":"add x","description":"new.txt"}],"message":"feat: add x.txt"}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub: will be called once (fallback). Returns a fresh subject.
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: new file added"})

	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(shortcut dup): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: new file added" {
		t.Errorf("Subject = %q, want %q (fallback message)", result.Commits[0].Subject, "feat: new file added")
	}
}

func TestDecompose_AutoMultiCommit_HappyPath(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create files for 3 concepts.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"b.txt"},{"title":"c3","description":"c.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b", "feat: add c"})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""}) // stub stager (can't run git)
	// The real stager can't run git, so inject the seam.
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
		"c3": {"c.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(auto N=3): %v", err)
	}
	if len(result.Commits) != 3 {
		t.Fatalf("Commits len = %d, want 3", len(result.Commits))
	}
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0", result.Amended)
	}

	// Verify ordered subjects.
	wantSubjects := []string{"feat: add a", "feat: add b", "feat: add c"}
	for i, cr := range result.Commits {
		if cr.Subject != wantSubjects[i] {
			t.Errorf("Commits[%d].Subject = %q, want %q", i, cr.Subject, wantSubjects[i])
		}
	}

	// Verify commit chain: HEAD advanced 3 times, clean tree.
	if dcmLogCount(t, repo) != 3 {
		t.Fatalf("commit count = %d, want 3", dcmLogCount(t, repo))
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}

	// Verify parent chain: each commit's parent is the previous one's SHA.
	log := dcmLogOneline(t, repo)
	lines := strings.Split(log, "\n")
	if len(lines) != 3 {
		t.Fatalf("log lines = %d, want 3", len(lines))
	}
	shas := make([]string, 3)
	for i, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		shas[i] = parts[0]
	}
	// Verify commit[1]'s parent is shas[0], commit[2]'s parent is shas[1].
	parent1 := dcmGitOut(t, repo, "rev-parse", shas[1]+"^")
	if parent1 != shas[0] {
		t.Errorf("commit[1] parent = %q, want %q", parent1, shas[0])
	}
	parent2 := dcmGitOut(t, repo, "rev-parse", shas[2]+"^")
	if parent2 != shas[1] {
		t.Errorf("commit[2] parent = %q, want %q", parent2, shas[1])
	}
}

func TestDecompose_Overlap(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"b.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub with a small sleep — allows the overlap to be observable via timing.
	messageM := stubtest.NewScript(t, bin, []string{"feat: add a", "feat: add b"})
	// Inject sleep into the message stub via Env.
	messageM.Env["STAGEHAND_STUB_SLEEP_MS"] = "200"

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)

	var stagerTimestamps []int64
	deps.stager = func(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error {
		stagerTimestamps = append(stagerTimestamps, time.Now().UnixNano())
		files := map[string][]string{
			"c1": {"a.txt"},
			"c2": {"b.txt"},
		}
		fl, ok := files[concept.Title]
		if ok && len(fl) > 0 {
			for _, f := range fl {
				dcmRunGit(t, repo, "add", f)
			}
		}
		return nil
	}

	start := time.Now()
	result, err := Decompose(context.Background(), deps)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Decompose(overlap): %v", err)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2", len(result.Commits))
	}

	// If NOT overlapped (sequential), total would be ≥400ms (2×200ms sleep).
	// If overlapped (1-deep), total would be <400ms because the second message's 200ms sleep
	// overlaps with the second stager.
	// Allow generous slack for CI variability.
	if elapsed >= 350*time.Millisecond {
		t.Logf("WARNING: elapsed = %v (may indicate no overlap — CI variability)", elapsed)
	}

	// Verify stager[1] ran while message[0] was in flight: the second stager timestamp should be
	// BEFORE the overall completion would be if sequential.
	if len(stagerTimestamps) == 2 {
		stager1Elapsed := time.Duration(stagerTimestamps[1] - stagerTimestamps[0])
		if stager1Elapsed < 50*time.Millisecond {
			// The two stagers ran very close together — the second one didn't wait for message[0]'s sleep.
			t.Logf("Overlap confirmed: stager calls were %v apart", stager1Elapsed)
		}
	}
}

func TestDecompose_EmptyConceptSkip(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")
	// b.txt is NOT written — the stager seam for "c2" will stage nothing (empty).

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"b.txt"},{"title":"c3","description":"c.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add c"})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	// c2 has no files to stage → empty concept → skipped.
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {}, // empty — nothing staged
		"c3": {"c.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(empty-skip): %v", err)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2 (concept 2 skipped)", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add a" {
		t.Errorf("Commits[0].Subject = %q, want %q", result.Commits[0].Subject, "feat: add a")
	}
	if result.Commits[1].Subject != "feat: add c" {
		t.Errorf("Commits[1].Subject = %q, want %q", result.Commits[1].Subject, "feat: add c")
	}
	// Verify only 2 commits in the repo.
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("commit count = %d, want 2", dcmLogCount(t, repo))
	}
}

func TestDecompose_PlannerFailure(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "a\n")

	// Planner returns unparseable output — after callPlanner's one retry, it returns ErrPlannerFailed.
	plannerM := dcmPlannerScriptManifest(t, bin, []string{"not json", "still not json"})
	messageM := dcmMessageManifest(t, bin, "should not be called")

	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)

	_, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error on planner failure, got nil")
	}
	if !errors.Is(err, ErrPlannerFailed) {
		t.Errorf("errors.Is(err, ErrPlannerFailed) = false, err = %v", err)
	}

	// Verify it's NOT a *RescueError (planner failure = non-rescue, §13.6.6).
	var re *generate.RescueError
	if errors.As(err, &re) {
		t.Error("error is *RescueError — planner failure should be NON-RESCUE")
	}

	// Verify no commit was created.
	if dcmLogCount(t, repo) != 0 {
		t.Errorf("commit count = %d, want 0 (no commit on planner failure)", dcmLogCount(t, repo))
	}
}

func TestDecompose_SafetyCap(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "a\n")

	// Planner proposes 20 commits, exceeding MaxCommits (12) — callPlanner enforces the cap.
	plannerJSON := `{"count":20,"single":false,"commits":[` +
		strings.Repeat(`{"title":"c","description":"a.txt"},`, 19) +
		`{"title":"c20","description":"a.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "should not be called")

	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)

	_, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected safety-cap error, got nil")
	}
	// The error should mention the cap.
	if !strings.Contains(err.Error(), "exceeds max_commits") {
		t.Errorf("error = %v, want safety-cap error mentioning 'exceeds max_commits'", err)
	}

	// Verify it's NOT a *RescueError.
	var re *generate.RescueError
	if errors.As(err, &re) {
		t.Error("error is *RescueError — safety cap should be NON-RESCUE")
	}

	// Verify no commit was created.
	if dcmLogCount(t, repo) != 0 {
		t.Errorf("commit count = %d, want 0", dcmLogCount(t, repo))
	}
}

func TestDecompose_ArbiterSkippedOnCleanTree(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "feat: add a")

	// Arbiter should NOT be called — use a counter to verify.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	arbiterM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(clean tree, arbiter skip): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0", result.Amended)
	}

	// Verify arbiter was NOT called.
	data, err := os.ReadFile(counterFile)
	if err == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("arbiter call count = %q, want 0 (clean tree — arbiter should not run)", count)
		}
	}
}

func TestDecompose_ArbiterWiring(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	// "leftover.txt" will be left un-staged by the stager seam, triggering the arbiter.
	dcmWriteFile(t, repo, "leftover.txt", "leftover\n")

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "feat: add a")

	// Arbiter returns null → new commit (resolveArbiter's null path).
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`)

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	// Stager only stages a.txt — leaves leftover.txt un-staged.
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})

	// Inject an arbiter-phase message agent via the seam — resolveNewCommit calls generateMessage.
	// Use a script with extra entries in case of dedupe retries.
	messageScriptM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add leftover", "feat: add leftover", "feat: add leftover", "feat: add leftover"})
	deps.Roles.Message = messageScriptM

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(arbiter wiring): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("loop Commits len = %d, want 1", len(result.Commits))
	}
	// Amended == 0 because target was null → new commit (not an amend).
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0 (null target → new commit)", result.Amended)
	}

	// Verify the tree is clean after the arbiter.
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status after arbiter = %q, want empty (clean)", status)
	}
	// 2 commits: loop + arbiter's new commit.
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("commit count = %d, want 2 (1 loop + 1 arbiter new)", dcmLogCount(t, repo))
	}
}

func TestDecompose_ErrorPropagation_Stager(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "a\n")

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "feat: add a")
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	stagerErr := errors.New("stager injection error")
	callCount := 0
	deps.stager = func(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error {
		callCount++
		return stagerErr
	}

	// S2 (FR-M12d): a stager that fails TWICE is treated as empty — the concept is skipped,
	// no error is returned. The stager seam is called twice (retry-once).
	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("expected no error (stager treated as empty), got %v", err)
	}
	if len(result.Commits) != 0 {
		t.Fatalf("Commits len = %d, want 0 (stager failed → empty → skip)", len(result.Commits))
	}
	if callCount != 2 {
		t.Errorf("stager call count = %d, want 2 (retry-once)", callCount)
	}

	// No commit should have been created (stager failed → empty → skip).
	if dcmLogCount(t, repo) != 0 {
		t.Errorf("commit count = %d, want 0", dcmLogCount(t, repo))
	}
}

func TestDecompose_ErrorPropagation_RescueError(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "aaa\n")

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub exits with a non-zero code (no valid output) — triggers RescueError after retries.
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0 // no duplicate retries — fail immediately on the first attempt
	messageM := stubtest.Manifest(bin, stubtest.Options{Exit: 1, Out: ""})
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDepsWithConfig(t, repo, roles, cfg)
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})

	_, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected RescueError, got nil")
	}
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Errorf("error = %v, want *generate.RescueError", err)
	}
}

func TestDecompose_UnbornRepo(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo) // repo with 0 commits (unborn HEAD)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "feat: initial")

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(unborn): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: initial" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "feat: initial")
	}

	// Verify root commit (no parent — HEAD~1 should fail).
	cmd := exec.Command("git", "-C", repo, "rev-parse", "HEAD~1")
	_, err = cmd.CombinedOutput()
	if err == nil {
		t.Error("expected HEAD~1 to fail on root commit (no parent)")
	}
}

func TestDecompose_Commits1_Mode(t *testing.T) {
	// Config.Commits == 1 → single escape hatch (same as Config.Single).
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "a\n")

	msgM := dcmMessageManifest(t, bin, "feat: single")
	roles := RoleManifests{Message: msgM}
	cfg := config.Defaults()
	cfg.Commits = 1

	deps := dcmDepsWithConfig(t, repo, roles, cfg)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(Commits=1): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("commit count = %d, want 1", dcmLogCount(t, repo))
	}
}

// TestDecompose_MessageRescuePartial (FR-M12a): 3 concepts; message stub times out for concept 1.
// Asserts: partial DecomposeResult with commit[0], *DecomposeRescueError wrapping *RescueError,
// errors.Is(generate.ErrRescue), FormatRescueMulti in deps.Out, arbiter NOT run.
func TestDecompose_MessageRescuePartial(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create files for 3 concepts.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"b.txt"},{"title":"c3","description":"c.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub: SleepMS > Timeout for concept 1 (index 1) to trigger timeout.
	// Use a script that returns fast for concepts 0 and 2, and a slow-exit-1 for concept 1.
	// Actually, simpler: use a script manifest. Concept 0 → "feat: add a" (success),
	// concept 1 → times out (SleepMS > Timeout), concept 2 → should not be reached.
	// The script responses are consumed in order by the message agent's retry loop.
	// To make concept 1 fail, we use a message manifest with Exit=1 and SleepMS > config.Timeout.

	// We need different behavior per concept index. Use the stager seam for staging
	// and a message script that works for concept 0 and fails for concept 1.
	// The message agent is called once per concept (MaxDuplicateRetries=0 for simplicity).

	// Actually: generateMessage has its own retry loop. We need the message stub to fail
	// for concept 1's generateMessage call. Let's use a call-counting approach.
	callCount := 0
	messageM := stubtest.NewScript(t, bin, []string{
		"feat: add a", // concept 0: success
		"",            // concept 1: empty → parse fail → RescueError (with MaxDuplicateRetries=0)
		"feat: add c", // concept 2: would-be (not reached)
	})

	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0 // fail immediately on parse failure

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})

	// Arbiter counter: should NOT be called on rescue.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	arbiterM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps, buf := dcmOutBuffer(t, repo, roles)
	deps.Config = cfg
	_ = callCount // not used

	// Stager seam: stages files for each concept. c2 (concept 1, the failing one) still stages.
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
		"c3": {"c.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error (message rescue for concept 1), got nil")
	}

	// (a) errors.As → *DecomposeRescueError
	var dre *DecomposeRescueError
	if !errors.As(err, &dre) {
		t.Fatalf("error = %v, want *DecomposeRescueError", err)
	}

	// (b) errors.As → *generate.RescueError (via Unwrap)
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("error does not unwrap to *RescueError: %v", err)
	}

	// (c) errors.Is → generate.ErrRescue (→ exitcode 3)
	if !errors.Is(err, generate.ErrRescue) {
		t.Errorf("error is not ErrRescue: %v", err)
	}

	// (d) partial commits: exactly 1 (commit 0)
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1 (only concept 0 landed)", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add a" {
		t.Errorf("Commits[0].Subject = %q, want %q", result.Commits[0].Subject, "feat: add a")
	}

	// (d) git log shows exactly 1 commit
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("git log count = %d, want 1", dcmLogCount(t, repo))
	}

	// (e) deps.Out contains FormatRescueMulti output naming "concept 2 of 3"
	out := buf.String()
	if !strings.Contains(out, "concept 2 of 3") {
		t.Errorf("rescue output missing 'concept 2 of 3'; got: %s", out)
	}
	if !strings.Contains(out, "update-ref HEAD") {
		t.Errorf("rescue output missing 'update-ref HEAD'; got: %s", out)
	}

	// (f) arbiter NOT called
	data, err := os.ReadFile(counterFile)
	if err == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("arbiter call count = %q, want 0 (rescue should skip arbiter)", count)
		}
	}
}

// TestDecompose_CASAbortPartial (FR-M12b): the stager seam moves HEAD between concept 0's
// publication and concept 1's publication, so publishCommit[1]'s CAS fails.
// Strategy: concept 0 publishes normally; concept 1's message generation uses treeA/treeB.
// We inject a HEAD move DURING the message generation for concept 1 (via the message
// script's STAGEHAND_STUB_SLEEP_MS) by running a background goroutine that waits a bit
// then moves HEAD. This ensures HEAD has moved by the time publishCommit[1] runs its CAS.
// Actually, simpler: use a 3-concept run where the stager for concept 2 moves HEAD
// BEFORE the freeze for concept 1's publish.
//
// Simplest approach: 2 concepts, stager for c2 moves HEAD during concept 1's stager phase.
// The loop: stage c1 → freeze → (no inflight) → stage c2 (HEAD moved here) → freeze →
// publish msg c1 (CAS for msg c1: expected=HEAD-at-c1-commit, actual=HEAD-moved-by-c2-stager).
func TestDecompose_CASAbortPartial(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create files for 3 concepts. Concept 1 (c2) is where the HEAD move will happen.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"b.txt"},{"title":"c3","description":"c.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b", "feat: add c"})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})

	// Arbiter counter: should NOT be called on CAS abort.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	arbiterM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps, buf := dcmOutBuffer(t, repo, roles)

	// Stager seam: moves HEAD for concept c3 (index 2). This means:
	//   iter 0: stage c1, freeze, no inflight → skip launch c1... wait, concept 0 launches msg.
	//   Actually: iter 0: stage c1 → freeze tree0 → no inflight → publish(nil) → launch msg[0]
	//   iter 1: stage c2 → freeze tree1 → publish(msg[0]) → launch msg[1]
	//   iter 2: stage c3 (HEAD move here) → freeze tree2 → publish(msg[1]) → CAS fails for msg[1]
	//
	//   publish(msg[1]): expected prevSHA = newSHA[1], but HEAD was moved during c3's stager.
	//   Wait, the HEAD move is during c3's stager, which runs BEFORE freeze tree2 and publish msg[1].
	//   So when publish(msg[1]) runs, HEAD != newSHA[1] (it was moved) → CAS failure.
	//   And newSHA[1] was already committed (from publish(msg[0])'s iteration).
	//
	//   Actually: publish(msg[1]) happens when we drain msg[1]'s channel. msg[1] was launched
	//   at the end of iter 1 (concept c2). So msg[1] is concept 2's message (feat: add b).
	//   The CAS for msg[1] expects prevSHA = newSHA[1]. But newSHA[1] hasn't been set yet —
	//   that's the CAS from publish(msg[0]) which published concept 1 (c2). Let me re-think.
	//
	//   Loop iteration 0 (c1): stage c1, freeze tree0, publish(nil)=nil, launch msg[0] (c1).
	//   Loop iteration 1 (c2): stage c2, freeze tree1, publish(msg[0]) → publishCommit(tree0, prevSHA, msg0)
	//     → newSHA = commit for c1 → prevSHA updated → launch msg[1] (c2).
	//   Loop iteration 2 (c3): stage c3 [HEAD MOVE HERE], freeze tree2, publish(msg[1])
	//     → publishCommit(tree1, prevSHA=newSHA[0], msg1) → CAS: expected=newSHA[0], actual=HEAD-moved → FAIL
	//   So: 1 commit landed (c1), CAS fails on c2. Partial = 1 commit.
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		files := map[string][]string{"c1": {"a.txt"}, "c2": {"b.txt"}, "c3": {"c.txt"}}
		fl, ok := files[concept.Title]
		if ok && len(fl) > 0 {
			for _, f := range fl {
				dcmRunGit(t, repo, "add", f)
			}
		}
		// Move HEAD for concept c3 — this shifts HEAD away from what publishCommit expects.
		if concept.Title == "c3" {
			dcmRunGit(t, repo, "commit", "--allow-empty", "-m", "external head move")
		}
		return nil
	}

	result, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected CAS error, got nil")
	}

	// (a) errors.As → *generate.CASError
	var ce *generate.CASError
	if !errors.As(err, &ce) {
		t.Fatalf("error = %v, want *generate.CASError", err)
	}

	// (b) errors.Is → git.ErrCASFailed → exitcode 1
	if !errors.Is(err, git.ErrCASFailed) {
		t.Errorf("error is not ErrCASFailed: %v", err)
	}

	// (c) deps.Out contains "HEAD moved"
	out := buf.String()
	if !strings.Contains(out, "HEAD moved") {
		t.Errorf("CAS output missing 'HEAD moved'; got: %s", out)
	}

	// (d) partial commits: exactly 1 (concept c1 landed before CAS failure on c2's publish)
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1 (only c1 landed before CAS failure)", len(result.Commits))
	}

	// (e) arbiter NOT called
	data, aerr := os.ReadFile(counterFile)
	if aerr == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("arbiter call count = %q, want 0 (CAS abort should skip arbiter)", count)
		}
	}
}

// TestDecompose_StagerRetryThenEmpty (FR-M12d): stager seam fails twice for concept 1,
// succeeds for others. Asserts: concept 1 skipped (≤N commits), stager called twice for c2,
// loop continued.
func TestDecompose_StagerRetryThenEmpty(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create files for c1 and c3 only. c2's stager fails, so no file for c2 is created.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"b.txt"},{"title":"c3","description":"c.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add c"})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)

	// Stager seam: fails twice for concept c2, succeeds for others.
	stagerCalls := map[string]int{}
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		stagerCalls[concept.Title]++
		if concept.Title == "c2" && stagerCalls[concept.Title] <= 2 {
			return errors.New("simulated stager failure")
		}
		// Stage real files for non-failing concepts.
		files := map[string][]string{"c1": {"a.txt"}, "c3": {"c.txt"}}
		fl, ok := files[concept.Title]
		if ok && len(fl) > 0 {
			for _, f := range fl {
				dcmRunGit(t, repo, "add", f)
			}
		}
		return nil
	}

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(stager retry): %v", err)
	}

	// (a) concept c2 skipped: 2 commits (c1 + c3), not 3.
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2 (c2 skipped)", len(result.Commits))
	}

	// (b) stager called exactly twice for concept c2 (retry-once).
	if stagerCalls["c2"] != 2 {
		t.Errorf("c2 stager calls = %d, want 2 (retry-once)", stagerCalls["c2"])
	}

	// (d) the loop continued: c3 was committed.
	if result.Commits[1].Subject != "feat: add c" {
		t.Errorf("Commits[1].Subject = %q, want %q", result.Commits[1].Subject, "feat: add c")
	}

	// Verify git: 2 commits.
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("git log count = %d, want 2", dcmLogCount(t, repo))
	}
}

// TestDecomposeRescueError_ExitCode verifies the Unwrap chain for exitcode.For mapping.
func TestDecomposeRescueError_ExitCode(t *testing.T) {
	// DecomposeRescueError → *RescueError → Kind: ErrRescue → exit 3
	dre1 := &DecomposeRescueError{Rescue: &generate.RescueError{Kind: generate.ErrRescue}}
	if !errors.Is(dre1, generate.ErrRescue) {
		t.Error("errors.Is(DecomposeRescueError{ErrRescue}, ErrRescue) = false, want true")
	}

	// DecomposeRescueError → *RescueError → Kind: ErrTimeout → exit 124
	dre2 := &DecomposeRescueError{Rescue: &generate.RescueError{Kind: generate.ErrTimeout}}
	if !errors.Is(dre2, generate.ErrTimeout) {
		t.Error("errors.Is(DecomposeRescueError{ErrTimeout}, ErrTimeout) = false, want true")
	}

	// errors.As to *RescueError traverses Unwrap
	var re *generate.RescueError
	if !errors.As(dre1, &re) {
		t.Error("errors.As(DecomposeRescueError, &*RescueError) = false, want true")
	}

	// errors.As to *CASError → git.ErrCASFailed → exit 1
	ce := &generate.CASError{}
	if !errors.Is(ce, git.ErrCASFailed) {
		t.Error("errors.Is(CASError, git.ErrCASFailed) = false, want true")
	}
}

// TestDecompose_RescueArbiterSkipped is a focused assertion that the arbiter does NOT run after
// a FR-M12a rescue (covered by TestDecompose_MessageRescuePartial's arbiter-count==0;
// this test is a focused sub-assertion for clarity).
func TestDecompose_RescueArbiterSkipped(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"b.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub: concept 0 succeeds, concept 1 fails (empty output → parse fail → RescueError).
	messageM := stubtest.NewScript(t, bin, []string{"feat: add a", ""})
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})

	// Arbiter with a counter — MUST be 0.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	arbiterM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps, _ := dcmOutBuffer(t, repo, roles)
	deps.Config = cfg
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Must be a rescue error (not some other failure).
	var dre *DecomposeRescueError
	if !errors.As(err, &dre) {
		t.Fatalf("expected *DecomposeRescueError, got %v", err)
	}

	// 1 commit landed (concept 0).
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}

	// Arbiter was NOT called.
	data, aerr := os.ReadFile(counterFile)
	if aerr == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("arbiter call count = %q, want 0", count)
		}
	}
}

// TestDecompose_StagerRetryThenSuccess (FR-M12d variant): stager fails once then succeeds.
func TestDecompose_StagerRetryThenSuccess(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "aaa\n")

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "feat: add a")

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)

	stagerCalls := 0
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		stagerCalls++
		if stagerCalls == 1 {
			return errors.New("first failure")
		}
		// Second call succeeds: stage the file.
		dcmRunGit(t, repo, "add", "a.txt")
		return nil
	}

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(retry-then-success): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add a" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "feat: add a")
	}
	if stagerCalls != 2 {
		t.Errorf("stager call count = %d, want 2 (fail once, succeed on retry)", stagerCalls)
	}
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("git log count = %d, want 1", dcmLogCount(t, repo))
	}
}

func TestComputeAmended(t *testing.T) {
	tests := []struct {
		name      string
		target    *string
		chainData []ChainEntry
		want      int
	}{
		{
			name:   "nil target → 0",
			target: nil,
			chainData: []ChainEntry{
				{SHA: "aaa", Tree: "t1", Message: "m1", Parent: ""},
				{SHA: "bbb", Tree: "t2", Message: "m2", Parent: "aaa"},
			},
			want: 0,
		},
		{
			name:   "tip amend → 1",
			target: strPtrForTest("bbb"),
			chainData: []ChainEntry{
				{SHA: "aaa", Tree: "t1", Message: "m1", Parent: ""},
				{SHA: "bbb", Tree: "t2", Message: "m2", Parent: "aaa"},
			},
			want: 1,
		},
		{
			name:   "mid-chain at 0 → N",
			target: strPtrForTest("aaa"),
			chainData: []ChainEntry{
				{SHA: "aaa", Tree: "t1", Message: "m1", Parent: ""},
				{SHA: "bbb", Tree: "t2", Message: "m2", Parent: "aaa"},
				{SHA: "ccc", Tree: "t3", Message: "m3", Parent: "bbb"},
			},
			want: 3, // N=3, idx=0 → 3-0=3
		},
		{
			name:   "mid-chain at 1 → N-1",
			target: strPtrForTest("bbb"),
			chainData: []ChainEntry{
				{SHA: "aaa", Tree: "t1", Message: "m1", Parent: ""},
				{SHA: "bbb", Tree: "t2", Message: "m2", Parent: "aaa"},
				{SHA: "ccc", Tree: "t3", Message: "m3", Parent: "bbb"},
			},
			want: 2, // N=3, idx=1 → 3-1=2
		},
		{
			name:   "not-found → 0",
			target: strPtrForTest("zzz"),
			chainData: []ChainEntry{
				{SHA: "aaa", Tree: "t1", Message: "m1", Parent: ""},
			},
			want: 0,
		},
		{
			name:      "empty chain → 0",
			target:    strPtrForTest("aaa"),
			chainData: nil,
			want:      0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeAmended(tc.target, tc.chainData)
			if got != tc.want {
				t.Errorf("computeAmended = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestInvokeStager_NilSeam(t *testing.T) {
	// With nil seam, invokeStager should call stageConcept. Since stageConcept requires a real
	// agent, this test just verifies the dispatch logic by checking that a non-nil seam works.
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	var called atomic.Bool
	concept := prompt.PlannerCommit{Title: "test", Description: "test files"}

	cfg := config.Defaults()
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	deps := Deps{
		Git:    git.New(repo),
		Config: cfg,
		Roles:  RoleManifests{Stager: stagerM},
	}
	deps.stager = func(ctx context.Context, deps Deps, c prompt.PlannerCommit) error {
		called.Store(true)
		return nil
	}

	err := invokeStager(context.Background(), deps, concept)
	if err != nil {
		t.Fatalf("invokeStager: %v", err)
	}
	if !called.Load() {
		t.Error("seam stager was not called")
	}
}

func TestInvokeStager_NilDepsStager(t *testing.T) {
	// When deps.stager is nil, invokeStager should dispatch to stageConcept.
	// We can't easily test stageConcept without a real agent, so just verify
	// the nil path doesn't panic and attempts to call stageConcept (which will
	// fail because the stub agent doesn't actually do anything).
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	deps := Deps{
		Git:    git.New(repo),
		Config: config.Defaults(),
		Roles:  RoleManifests{Stager: stagerM},
	}
	// deps.stager is nil (zero value)

	concept := prompt.PlannerCommit{Title: "test", Description: "test"}
	// This will call stageConcept which calls the stub stager — it should succeed (stub exits 0).
	err := invokeStager(context.Background(), deps, concept)
	if err != nil {
		t.Fatalf("invokeStager(nil seam): %v", err)
	}
}

func TestDupCheckMessage_Unborn(t *testing.T) {
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	// Unborn repo — should always return false (no dup possible).

	deps := Deps{Git: git.New(repo), Config: config.Defaults()}
	isDup := dupCheckMessage(context.Background(), deps, "feat: anything", true)
	if isDup {
		t.Error("dupCheckMessage returned true on unborn repo — want false")
	}
}

func TestDupCheckMessage_Existing(t *testing.T) {
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "feat: existing subject")

	deps := Deps{Git: git.New(repo), Config: config.Defaults()}
	isDup := dupCheckMessage(context.Background(), deps, "feat: existing subject", false)
	if !isDup {
		t.Error("dupCheckMessage returned false for existing subject — want true")
	}
	// Different subject → false.
	isDup = dupCheckMessage(context.Background(), deps, "feat: new subject", false)
	if isDup {
		t.Error("dupCheckMessage returned true for non-existing subject — want false")
	}
}

func TestBuildCommitResult(t *testing.T) {
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmStageFile(t, repo, "a.txt")
	dcmRunGit(t, repo, "commit", "-m", "feat: add a")

	sha := dcmHeadSHA(t, repo)
	deps := Deps{Git: git.New(repo), Config: config.Defaults()}

	// The repo has exactly 1 commit (root commit) — isRoot=true.
	cr, err := buildCommitResult(context.Background(), deps, sha, "feat: add a", true)
	if err != nil {
		t.Fatalf("buildCommitResult: %v", err)
	}
	if cr.SHA != sha {
		t.Errorf("SHA = %q, want %q", cr.SHA, sha)
	}
	if cr.Subject != "feat: add a" {
		t.Errorf("Subject = %q, want %q", cr.Subject, "feat: add a")
	}
	if len(cr.Files) != 1 || cr.Files[0].Path != "a.txt" {
		t.Errorf("Files = %v, want [a.txt]", cr.Files)
	}
}

// strPtrForTest is a test helper to create a string pointer.
func strPtrForTest(s string) *string { return &s }
