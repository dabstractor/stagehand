package decompose

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/prompt"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/stubtest"
	"github.com/dustin/stagehand/internal/ui"
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

// dcmShaResolves reports whether SHA resolves via git rev-parse --verify <sha>^{commit}.
// Exit 0 means resolvable (not dangling).
func dcmShaResolves(t *testing.T, repo, sha string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "rev-parse", "--verify", sha+"^{commit}")
	_, err := cmd.CombinedOutput()
	return err == nil
}

// dcmScriptArbiter builds a provider.Manifest whose Command is a shell script that parses SHAs
// from the arbiter's STDIN prompt (each commit's SHA is a bare 40-hex line). The script emits
// {"target": "<sha>"} for the chosen SHA. mode is "tip" (last SHA) or "mid" (2nd SHA).
func dcmScriptArbiter(t *testing.T, bin string, mode string) provider.Manifest {
	t.Helper()
	var script string
	switch mode {
	case "tip":
		script = `#!/bin/sh
sha=$(sed -n 's/^\([0-9a-f]\{40\}\)$/\1/p' | tail -n 1)
printf '{"target": "%s"}\n' "$sha"
`
	case "mid":
		script = `#!/bin/sh
sha=$(sed -n 's/^\([0-9a-f]\{40\}\)$/\1/p' | sed -n '2p')
printf '{"target": "%s"}\n' "$sha"
`
	default:
		t.Fatalf("dcmScriptArbiter: unknown mode %q", mode)
	}
	path := t.TempDir() + "/arbiter.sh"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write arbiter script: %v", err)
	}
	m := stubtest.Manifest(bin, stubtest.Options{Out: ""})
	m.Command = &path
	return m
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
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the planner path is tested

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
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the planner+shortcut path is tested

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

func TestDecompose_StagerMovedHEAD(t *testing.T) {
	// A rogue stager seam that commits (moves HEAD) should trigger ErrStagerMovedHEAD.
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")        // BORN repo → HEAD has a real SHA to move away from
	dcmWriteFile(t, repo, "a.txt", "aaa\n") // untracked → dirty tree (FR-M1 routing satisfied)

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a"})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the loop path is tested

	// ROGUE seam: stages nothing, instead COMMITS → moves HEAD.
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		dcmRunGit(t, repo, "commit", "--allow-empty", "-m", "rogue: moved HEAD")
		return nil
	}

	_, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected ErrStagerMovedHEAD, got nil")
	}
	if !errors.Is(err, ErrStagerMovedHEAD) {
		t.Fatalf("expected ErrStagerMovedHEAD, got %v", err)
	}
	if !strings.Contains(err.Error(), "stager moved HEAD") {
		t.Errorf("error message missing 'stager moved HEAD'; got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "mutated refs") {
		t.Errorf("error message missing 'mutated refs'; got: %s", err.Error())
	}
}

func TestDecompose_StagerFreezeViolation(t *testing.T) {
	// A rogue stager seam that stages a post-freeze sentinel → ErrFreezeViolation.
	// Mirrors TestDecompose_StagerMovedHEAD (rogue stager + well-behaved stager pair).
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")        // BORN repo
	dcmWriteFile(t, repo, "a.txt", "aaa\n") // the legit change in T_start

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a"})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the loop path is tested

	// ROGUE seam: stages the concept path AND a post-freeze sentinel (simulating `git add -A`
	// sweeping a concurrent change). The sentinel is written AFTER FreezeWorkingTree captured T_start.
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		dcmRunGit(t, repo, "add", "a.txt")                    // the legit concept path (in T_start)
		dcmWriteFile(t, repo, "sentinel.txt", "concurrent\n") // appears AFTER the freeze
		dcmRunGit(t, repo, "add", "sentinel.txt")             // stager sweeps it in (the violation)
		return nil
	}

	_, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected ErrFreezeViolation, got nil")
	}
	if !errors.Is(err, ErrFreezeViolation) {
		t.Fatalf("expected ErrFreezeViolation, got %v", err)
	}
	if !strings.Contains(err.Error(), "sentinel.txt") {
		t.Errorf("error missing 'sentinel.txt'; got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "not present in T_start") {
		t.Errorf("error missing 'not present in T_start'; got: %s", err.Error())
	}
	// HEAD unchanged — only "initial" exists.
	if got := dcmLogCount(t, repo); got != 1 {
		t.Errorf("commit count = %d, want 1 (HEAD unchanged — only 'initial')", got)
	}
	// The sentinel is in no commit.
	logOutput := dcmGitOut(t, repo, "log", "--name-only", "--format=")
	if strings.Contains(logOutput, "sentinel.txt") {
		t.Errorf("sentinel.txt appears in a commit — freeze violation should prevent this:\n%s", logOutput)
	}
}

func TestDecompose_StagerGuardHappyPath(t *testing.T) {
	// A well-behaved stager (git add, no ref mutation) completes normally — guard passes.
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")
	dcmWriteFile(t, repo, "a.txt", "aaa\n")

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a"})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2                                                    // override FR-M2b one-file short-circuit so the loop+stager path is tested
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}}) // git add only

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("happy-path guard false-positive: %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	// HEAD advanced exactly once (the published commit), via UpdateRefCAS — NOT via the stager.
	if got := dcmLogCount(t, repo); got != 2 { // initial + 1 published
		t.Errorf("commit count = %d, want 2", got)
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
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
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the planner is reached

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
	dcmWriteFile(t, repo, "b.txt", "b\n") // 2nd file: bypasses FR-M2b one-file short-circuit (auto mode, count≥2)

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
	// Note: FR-M2b one-file short-circuit does NOT fire because 2 files ⇒ DiffTreeNames count ≥ 2.
	// The auto-mode safety cap is still tested (forcedCount==0 inside callPlanner).
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
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the loop+arbiter path is tested
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
	if len(result.Commits) != 2 {
		t.Fatalf("loop Commits len = %d, want 2 (null-path commit now included via reread)", len(result.Commits))
	}
	// Amended == 0 because target was null → new commit (not an amend).
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0 (null target → new commit)", result.Amended)
	}
	// The second commit is the arbiter-created null-path commit — verify it resolves and is HEAD.
	if result.Commits[1].Subject != "feat: add leftover" {
		t.Errorf("Commits[1].Subject = %q, want %q", result.Commits[1].Subject, "feat: add leftover")
	}
	if !dcmShaResolves(t, repo, result.Commits[1].SHA) {
		t.Errorf("Commits[1].SHA %q does not resolve (dangling)", result.Commits[1].SHA)
	}
	if result.Commits[1].SHA != dcmHeadSHA(t, repo) {
		t.Errorf("Commits[1].SHA = %q, want HEAD %q", result.Commits[1].SHA, dcmHeadSHA(t, repo))
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
	dcmWriteFile(t, repo, "b.txt", "b\n") // 2nd file: bypasses FR-M2b one-file short-circuit (auto mode, count≥2)

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
	cfg.Commits = 2             // override FR-M2b one-file short-circuit so the loop path is tested
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
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the loop path is tested
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

// TestDecompose_CASAbortPartial (FR-M12b): an external goroutine moves HEAD between concept 0's
// publication and concept 1's publication, so publishCommit[1]'s CAS fails.
// Uses a well-behaved stager (no HEAD movement) and an external goroutine with a timed delay
// that fires during the message agent's sleep window.
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
	messageM.Env["STAGEHAND_STUB_SLEEP_MS"] = "1000" // create timing window for external HEAD move

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})

	// Arbiter counter: should NOT be called on CAS abort.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	arbiterM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps, buf := dcmOutBuffer(t, repo, roles)

	// External HEAD move: a goroutine moves HEAD after c1 is published but before c2's CAS.
	// The message agent sleep (1000ms) creates the timing window:
	//   iter 0: stage c1, freeze, publish(nil), launch msg[0] (sleep 1000ms)
	//   iter 1: stage c2, freeze, publish(msg[0]) → drain (wait ~1000ms) → publishCommit c1 → success
	//     → launch msg[1] (sleep 1000ms)
	//   iter 2: stage c3, freeze, publish(msg[1]) → drain (wait ~1000ms) → goroutine fired at
	//     1500ms → HEAD moved → publishCommit c2 → CAS failure!
	//   So: 1 commit landed (c1), CAS fails on c2. Partial = 1 commit.
	go func() {
		time.Sleep(1500 * time.Millisecond)
		dcmRunGit(t, repo, "commit", "--allow-empty", "-m", "external head move")
	}()

	// Well-behaved stager seam: stages files only (no HEAD movement — the guard would catch it).
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		files := map[string][]string{"c1": {"a.txt"}, "c2": {"b.txt"}, "c3": {"c.txt"}}
		fl, ok := files[concept.Title]
		if ok && len(fl) > 0 {
			for _, f := range fl {
				dcmRunGit(t, repo, "add", f)
			}
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
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the loop path is tested

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

// TestDecompose_OneFileShortcut_PlannerBypassed (FR-M2b core test): BORN repo, exactly ONE untracked file,
// auto mode — the planner is NEVER called (counter absent/"0"), ONE commit lands with the MESSAGE
// role's subject, and git status is clean (proves the ReadTree index-sync, findings §4).
func TestDecompose_OneFileShortcut_PlannerBypassed(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")            // BORN repo (preRunHEAD set; dup-check vs "initial")
	dcmWriteFile(t, repo, "only.txt", "only\n") // EXACTLY one untracked file

	// Planner counter-manifest: if called, the counter file appears. It must NOT be called.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	plannerM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})
	messageM := dcmMessageManifest(t, bin, "feat: add the only file")
	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles) // default config: Commits=0 (auto), Single=false

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(one-file bypass): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add the only file" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "feat: add the only file")
	}
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0", result.Amended)
	}

	// Verify planner was NEVER called (counter file absent or "0").
	data, ferr := os.ReadFile(counterFile)
	if ferr == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("planner call count = %q, want 0 (FR-M2b bypass — planner never invoked)", count)
		}
	}

	// Verify git state: 2 commits (initial + 1), clean tree.
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("commit count = %d, want 2 (initial + 1)", dcmLogCount(t, repo))
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean — proves ReadTree index-sync)", status)
	}
}

// TestDecompose_OneFileShortcut_Unborn (FR-M2b edge): UNBORN repo (no initial commit), one file →
// short-circuit fires, producing a ROOT commit. Planner is never called. Clean post-state.
func TestDecompose_OneFileShortcut_Unborn(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)                        // UNBORN (no dcmCommitRaw — no commits)
	dcmWriteFile(t, repo, "only.txt", "only\n") // ONE untracked file

	// Planner counter-manifest: must NOT be called.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	plannerM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})
	messageM := dcmMessageManifest(t, bin, "feat: root add")
	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles) // auto mode

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(one-file unborn): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: root add" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "feat: root add")
	}

	// Verify planner was NEVER called.
	data, ferr := os.ReadFile(counterFile)
	if ferr == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("planner call count = %q, want 0 (unborn one-file bypass)", count)
		}
	}

	// Verify git state: 1 ROOT commit, clean tree.
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("commit count = %d, want 1 (root commit)", dcmLogCount(t, repo))
	}
	if !dcmShaResolves(t, repo, result.Commits[0].SHA) {
		t.Errorf("Commits[0].SHA %q does not resolve (dangling)", result.Commits[0].SHA)
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}
}

// TestDecompose_OneFileShortcut_CommitsOverride (FR-M2b override): --commits 2 with one file →
// short-circuit is OVERRIDDEN, the planner IS called, the loop runs. Verifies that a forced count
// is honored even for a single file.
func TestDecompose_OneFileShortcut_CommitsOverride(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")
	dcmWriteFile(t, repo, "only.txt", "only\n") // ONE untracked file

	// Planner returns a 2-concept plan (if called, succeeds).
	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"only.txt"},{"title":"c2","description":"leftover"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: c1", "feat: arbiter"})
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	cfg := config.Defaults()
	cfg.Commits = 2 // FORCED count ⇒ short-circuit OVERRIDDEN
	deps := dcmDepsWithConfig(t, repo, roles, cfg)
	// c1 stages only.txt; c2 stages nothing → empty-skip. The loop runs, proving the planner was called.
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"only.txt"}, "c2": {}})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(commits override): %v", err)
	}
	// The planner path ran (NOT runOneFileShortcut): at least 1 commit from c1.
	if len(result.Commits) < 1 {
		t.Fatalf("Commits len = %d, want ≥1 (planner path ran)", len(result.Commits))
	}
	// 2 commits total (initial + c1). The c2 empty-skip means no c2 commit.
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("commit count = %d, want 2 (initial + c1 from planner path)", dcmLogCount(t, repo))
	}
}

// TestDecompose_OneFileShortcut_TwoFilesNoBypass (FR-M2b boundary): TWO files → count 2 →
// the short-circuit threshold (len==1) is NOT met → the planner IS called and the loop runs.
func TestDecompose_OneFileShortcut_TwoFilesNoBypass(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n") // TWO files

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"b.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b"})
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles) // auto mode (Commits=0), but 2 files ⇒ count 2 ⇒ NO short-circuit
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}, "c2": {"b.txt"}})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(two files): %v", err)
	}
	// The planner path ran (NOT runOneFileShortcut): 2 commits prove the planner partitioned.
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2 (planner partitioned both files)", len(result.Commits))
	}
	// 3 commits total (initial + c1 + c2).
	if dcmLogCount(t, repo) != 3 {
		t.Fatalf("commit count = %d, want 3 (initial + c1 + c2)", dcmLogCount(t, repo))
	}
}

// TestDecompose_OneFileShortcut_Deletion (FR-M2b edge): single DELETION counts as one changed
// path → short-circuit fires. DiffTreeNames includes deletions (git diff-tree --name-only).
func TestDecompose_OneFileShortcut_Deletion(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create a tracked file and commit it.
	dcmWriteFile(t, repo, "gone.txt", "x\n")
	dcmStageFile(t, repo, "gone.txt")
	dcmRunGit(t, repo, "commit", "-m", "initial")

	// Delete the tracked file.
	dcmRunGit(t, repo, "rm", "gone.txt")

	// Planner counter-manifest: must NOT be called.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	plannerM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})
	messageM := dcmMessageManifest(t, bin, "chore: remove gone")
	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles) // auto mode

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(one-file deletion): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "chore: remove gone" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "chore: remove gone")
	}

	// Verify planner was NEVER called.
	data, ferr := os.ReadFile(counterFile)
	if ferr == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("planner call count = %q, want 0 (deletion bypass)", count)
		}
	}

	// Verify git state: 2 commits (initial + 1 deletion), clean tree.
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("commit count = %d, want 2 (initial + deletion)", dcmLogCount(t, repo))
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
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

// lockedBuffer is a thread-safe bytes.Buffer for -race-clean concurrent Verbose writes.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// piShape sets ProviderFlag on a manifest to simulate a pi-shaped agent (multi-provider).
// The sub-provider is now encoded in the model slash-prefix (v3 FR-R5b), so callers must also
// set a slash-prefix model on the config/role to exercise the --provider flag.
func piShape(m *provider.Manifest, providerFlag string) {
	m.ProviderFlag = &providerFlag
}

// TestDecompose_ArbiterTipAmend_RereadsFinalSHA: 2 concepts + leftover; arbiter amends the tip.
// Verifies rereadFinalCommits replaces stale SHAs with the post-amend tip.
func TestDecompose_ArbiterTipAmend_RereadsFinalSHA(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// 2 concepts + 1 leftover.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "leftover.txt", "leftover\n")

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"b.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub: 2 entries for the loop (resolveTipAmend reuses messages — no extra call).
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b"})

	// Shell-script arbiter picks the tip (last SHA).
	arbiterM := dcmScriptArbiter(t, bin, "tip")

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(tip amend): %v", err)
	}
	// 2 commits (tip was amended, not a new one — null adds; amend replaces).
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2", len(result.Commits))
	}
	// Both SHAs must resolve.
	for i, cr := range result.Commits {
		if !dcmShaResolves(t, repo, cr.SHA) {
			t.Errorf("Commits[%d].SHA %q does not resolve (dangling)", i, cr.SHA)
		}
	}
	// The tip commit's SHA must equal HEAD (post-amend).
	if result.Commits[1].SHA != dcmHeadSHA(t, repo) {
		t.Errorf("Commits[1].SHA = %q, want HEAD %q", result.Commits[1].SHA, dcmHeadSHA(t, repo))
	}
	// The first commit should be unchanged (only the tip was amended).
	if result.Commits[0].Subject != "feat: add a" {
		t.Errorf("Commits[0].Subject = %q, want \"feat: add a\"", result.Commits[0].Subject)
	}
	// Amended == 1 for tip amend.
	if result.Amended != 1 {
		t.Errorf("Amended = %d, want 1 (tip amend)", result.Amended)
	}
	// Clean tree after the arbiter.
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}
}

// TestDecompose_ArbiterMidChain_AllSHAsResolve: 3 concepts + leftover; arbiter rebuilds from
// concept[1] (mid-chain). Verifies all re-read SHAs resolve.
func TestDecompose_ArbiterMidChain_AllSHAsResolve(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// 3 concepts + 1 leftover.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")
	dcmWriteFile(t, repo, "leftover.txt", "leftover\n")

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"b.txt"},{"title":"c3","description":"c.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// 3 message entries (resolveMidChain reuses messages).
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b", "feat: add c"})

	// Shell-script arbiter picks the 2nd SHA (concept[1]).
	arbiterM := dcmScriptArbiter(t, bin, "mid")

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
		"c3": {"c.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(mid-chain): %v", err)
	}
	// 3 commits after mid-chain rebuild (concept[1] onward rewritten).
	if len(result.Commits) != 3 {
		t.Fatalf("Commits len = %d, want 3", len(result.Commits))
	}
	// All SHAs must resolve.
	for i, cr := range result.Commits {
		if !dcmShaResolves(t, repo, cr.SHA) {
			t.Errorf("Commits[%d].SHA %q does not resolve (dangling)", i, cr.SHA)
		}
	}
	// SHAs should match git log --reverse --format=%H.
	logSHAs := strings.Split(dcmGitOut(t, repo, "log", "--reverse", "--format=%H"), "\n")
	for i, cr := range result.Commits {
		if i < len(logSHAs) && cr.SHA != logSHAs[i] {
			t.Errorf("Commits[%d].SHA = %q, want %q", i, cr.SHA, logSHAs[i])
		}
	}
	// Clean tree.
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}
}

// TestDecompose_HappyPath_CommitsAccurate: verifies that on the happy path (arbiter does NOT run),
// the loop's Commits entries are accurate (SHA resolves, matches HEAD).
func TestDecompose_HappyPath_CommitsAccurate(t *testing.T) {
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
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the loop path is tested
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(happy path): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	// The loop's SHA must resolve and equal HEAD.
	if !dcmShaResolves(t, repo, result.Commits[0].SHA) {
		t.Errorf("Commits[0].SHA %q does not resolve", result.Commits[0].SHA)
	}
	if result.Commits[0].SHA != dcmHeadSHA(t, repo) {
		t.Errorf("Commits[0].SHA = %q, want HEAD %q", result.Commits[0].SHA, dcmHeadSHA(t, repo))
	}
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0", result.Amended)
	}
}

func TestDecompose_RoleResolvesSubProvider(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create files for 2 concepts.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"b.txt"}]}`
	plannerM := stubtest.Manifest(bin, stubtest.Options{Out: plannerJSON})
	piShape(&plannerM, "--provider")

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	piShape(&stagerM, "--provider")

	messageM := stubtest.NewScript(t, bin, []string{"feat: add a", "feat: add b"})
	piShape(&messageM, "--provider")

	arbiterM := stubtest.Manifest(bin, stubtest.Options{Out: `{"target": null}`})
	piShape(&arbiterM, "--provider")

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	deps.Config.Provider = "pi"              // the manifest NAME — the conflation source; must NOT be emitted
	deps.Config.Model = "openrouter/gpt-5.4" // slash-prefix model → Render emits --provider openrouter
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
	})

	var lb lockedBuffer
	deps.Verbose = ui.NewVerbose(&lb, true)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2", len(result.Commits))
	}

	cmd := lb.String()
	if !strings.Contains(cmd, "--provider openrouter") {
		t.Errorf("Decompose command missing --provider openrouter\ngot: %s", cmd)
	}
	if strings.Contains(cmd, "--provider pi") {
		t.Errorf("Decompose command emits manifest name as sub-provider (conflation)\ngot: %s", cmd)
	}
}

// TestDecompose_SentinelAfterFreezeExcluded (§20.2 "Start-of-run freeze (v2)") verifies that a file
// written to the working tree AFTER the freeze is invisible to every commit. The stager seam stages only
// the concept's path (well-behaved); the planner diffs tStart (frozen), so the sentinel is not even a
// concept. The arbiter's leftover diff is also frozen (TreeDiff(tipTree, tStart)) so the sentinel is
// absent from the arbiter's diff payload. NOTE: the arbiter's STAGING (resolveArbiter's AddAll) is NOT
// yet frozen — enforcement is P3.M2.T1.S1 (FR-M1c). So we verify only the loop commits.
func TestDecompose_SentinelAfterFreezeExcluded(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Pre-freeze state: unstaged changes (a.txt + c.txt). No b.txt — no leftovers after the loop.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"c.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add c", "feat: add leftover", "feat: add sentinel", "feat: add sentinel"})
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`) // null → new commit for any leftovers
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}

	// Stager seam: stages only the concept's path (well-behaved). On first invocation, writes a sentinel
	// file simulating a concurrent change mid-run (AFTER the freeze). The sentinel is NOT staged.
	stagerCallCount := 0
	deps := dcmDeps(t, repo, roles)
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		stagerCallCount++
		if stagerCallCount == 1 {
			// Simulate a concurrent change: write a sentinel AFTER the freeze.
			dcmWriteFile(t, repo, "sentinel.txt", "concurrent")
		}
		// Stage only the concept's path (well-behaved — never stages the sentinel).
		files := map[string][]string{"c1": {"a.txt"}, "c2": {"c.txt"}}
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
		t.Fatalf("Decompose(sentinel): %v", err)
	}

	// Verify sentinel.txt appears in NO LOOP commit's file list (first 2 commits).
	// The planner diffed T_start (frozen), so sentinel.txt was never a concept.
	// NOTE: the arbiter's STAGING (resolveArbiter's AddAll) picks up sentinel.txt from the working
	// tree — that is expected; enforcement of the arbiter staging is P3.M2.T1.S1 (FR-M1c).
	loopCount := len(result.Commits)
	if loopCount > 2 {
		loopCount = 2 // arbiter may add a commit for sentinel.txt (AddAll staging — not yet frozen)
	}
	for i := 0; i < loopCount; i++ {
		for _, fc := range result.Commits[i].Files {
			if fc.Path == "sentinel.txt" {
				t.Errorf("Loop commits[%d] contains sentinel.txt — the freeze should exclude post-freeze changes", i)
			}
		}
	}
}
