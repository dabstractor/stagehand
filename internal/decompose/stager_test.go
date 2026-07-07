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

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/prompt"
	"github.com/dustin/stagecoach/internal/provider"
	"github.com/dustin/stagecoach/internal/stubtest"
	"github.com/dustin/stagecoach/internal/ui"
)

// --- Fixture helpers (stg*-prefixed to avoid colliding with planner_test.go's un-prefixed copies) ---

// stgInitRepo creates a git repo in dir with repo-local identity config (no env pollution).
func stgInitRepo(t *testing.T, dir string) {
	t.Helper()
	stgRunGit(t, dir, "init")
	stgRunGit(t, dir, "config", "user.name", "Test")
	stgRunGit(t, dir, "config", "user.email", "test@example.com")
}

// stgWriteFile creates a file at dir/name with the given body.
func stgWriteFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := dir + string(os.PathSeparator) + name
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("stgWriteFile %s: %v", full, err)
	}
}

// stgStageFile runs git add for name in dir.
func stgStageFile(t *testing.T, dir, name string) {
	t.Helper()
	stgRunGit(t, dir, "add", name)
}

// stgCommitRaw creates an empty commit with the given message.
func stgCommitRaw(t *testing.T, dir, msg string) {
	t.Helper()
	stgRunGit(t, dir, "commit", "--allow-empty", "-m", msg)
}

// stgRunGit executes git -C dir args... and returns trimmed stdout.
func stgRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// stgGitOut runs a raw git command in dir and returns trimmed stdout (alias for consistency).
func stgGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return stgRunGit(t, dir, args...)
}

// --- Test helpers ---

// tooledStubManifest wraps stubtest.Manifest with non-empty TooledFlags so RenderTooled succeeds.
// The stub ignores argv, so the flag value is cosmetic — it just needs to be non-empty.
func tooledStubManifest(t *testing.T, bin string, o stubtest.Options) provider.Manifest {
	t.Helper()
	m := stubtest.Manifest(bin, o)
	m.TooledFlags = []string{"--tooled-stub-flag"}
	return m
}

// stagerDeps builds a minimal Deps for stager tests (no ResolveRoles).
func stagerDeps(t *testing.T, repo string, m provider.Manifest) Deps {
	t.Helper()
	return Deps{
		Git:     git.New(repo),
		Config:  config.Defaults(),
		Roles:   RoleManifests{Stager: m},
		Verbose: nil,
	}
}

// stagerDepsWithConfig builds a Deps with a custom config (for timeout tests).
func stagerDepsWithConfig(t *testing.T, repo string, m provider.Manifest, cfg config.Config) Deps {
	t.Helper()
	return Deps{
		Git:     git.New(repo),
		Config:  cfg,
		Roles:   RoleManifests{Stager: m},
		Verbose: nil,
	}
}

// --- Tests ---

func TestStageConcept_Success(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	stgInitRepo(t, repo)
	stgCommitRaw(t, repo, "initial")

	m := tooledStubManifest(t, bin, stubtest.Options{Out: "staged a.txt"})
	deps := stagerDeps(t, repo, m)
	concept := prompt.PlannerCommit{Title: "Add a", Description: "a.txt"}

	err := stageConcept(context.Background(), deps, concept)
	if err != nil {
		t.Fatalf("stageConcept: %v", err)
	}
}

func TestStageConcept_NonZeroExit(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	stgInitRepo(t, repo)
	stgCommitRaw(t, repo, "initial")

	m := tooledStubManifest(t, bin, stubtest.Options{Exit: 1})
	deps := stagerDeps(t, repo, m)
	concept := prompt.PlannerCommit{Title: "Add a", Description: "a.txt"}

	err := stageConcept(context.Background(), deps, concept)
	if err == nil {
		t.Fatal("expected error on non-zero exit, got nil")
	}
	if !errors.Is(err, ErrStagerFailed) {
		t.Errorf("errors.Is(err, ErrStagerFailed) = false, error = %v", err)
	}
}

func TestStageConcept_Timeout(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	stgInitRepo(t, repo)
	stgCommitRaw(t, repo, "initial")

	cfg := config.Defaults()
	cfg.Timeout = 100 * time.Millisecond

	m := tooledStubManifest(t, bin, stubtest.Options{SleepMS: 2000})
	deps := stagerDepsWithConfig(t, repo, m, cfg)
	concept := prompt.PlannerCommit{Title: "Add a", Description: "a.txt"}

	err := stageConcept(context.Background(), deps, concept)
	if err == nil {
		t.Fatal("expected error on timeout, got nil")
	}
	if !errors.Is(err, ErrStagerFailed) {
		t.Errorf("errors.Is(err, ErrStagerFailed) = false, error = %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, error = %v", err)
	}
}

func TestStageConcept_RenderTooledMode(t *testing.T) {
	// Use the RAW stubtest.Manifest (nil TooledFlags) to prove RenderTooled errors on it.
	// RenderBare would silently succeed on nil BareFlags, so the error proves the tooled path.
	bin := stubtest.Build(t)
	repo := t.TempDir()
	stgInitRepo(t, repo)
	stgCommitRaw(t, repo, "initial")

	m := stubtest.Manifest(bin, stubtest.Options{Out: "x"}) // RAW — nil TooledFlags
	deps := stagerDeps(t, repo, m)
	concept := prompt.PlannerCommit{Title: "Add a", Description: "a.txt"}

	err := stageConcept(context.Background(), deps, concept)
	if err == nil {
		t.Fatal("expected error with nil TooledFlags in RenderTooled mode, got nil")
	}
	if !errors.Is(err, ErrStagerFailed) {
		t.Errorf("errors.Is(err, ErrStagerFailed) = false, error = %v", err)
	}
	if !strings.Contains(err.Error(), "tooled") {
		t.Errorf("error does not mention 'tooled': %s", err)
	}
}

func TestFreezeSnapshot_Success_Immutable(t *testing.T) {
	// §13.6.3 invariant 1: tree[i] is frozen before stager[i+1] starts.
	// Stage file A → freeze → tree1; stage file B → freeze → tree2.
	// tree1 != tree2; ls-tree tree1 lists ONLY a.txt (NOT b.txt — frozen).
	repo := t.TempDir()
	stgInitRepo(t, repo)
	stgCommitRaw(t, repo, "initial")

	stgWriteFile(t, repo, "a.txt", "a\n")
	stgStageFile(t, repo, "a.txt")

	deps := stagerDeps(t, repo, provider.Manifest{}) // manifest unused by freeze

	tree1, err := freezeSnapshot(context.Background(), deps)
	if err != nil {
		t.Fatalf("freezeSnapshot (tree1): %v", err)
	}
	if tree1 == "" {
		t.Fatal("tree1 is empty, want non-empty SHA")
	}

	stgWriteFile(t, repo, "b.txt", "b\n")
	stgStageFile(t, repo, "b.txt")

	tree2, err := freezeSnapshot(context.Background(), deps)
	if err != nil {
		t.Fatalf("freezeSnapshot (tree2): %v", err)
	}
	if tree2 == "" {
		t.Fatal("tree2 is empty, want non-empty SHA")
	}

	if tree1 == tree2 {
		t.Errorf("tree1 == tree2 = %q, want different (b.txt was staged between freezes)", tree1)
	}

	// IMMMUTABILITY: tree1 was frozen BEFORE staging b.txt → ls-tree tree1 lists ONLY a.txt.
	ls1 := stgGitOut(t, repo, "ls-tree", "--name-only", tree1)
	if !strings.Contains(ls1, "a.txt") {
		t.Errorf("tree1 ls-tree missing a.txt: %s", ls1)
	}
	if strings.Contains(ls1, "b.txt") {
		t.Errorf("tree1 ls-tree contains b.txt (frozen tree leaked!): %s", ls1)
	}

	// tree2 should list both.
	ls2 := stgGitOut(t, repo, "ls-tree", "--name-only", tree2)
	if !strings.Contains(ls2, "a.txt") {
		t.Errorf("tree2 ls-tree missing a.txt: %s", ls2)
	}
	if !strings.Contains(ls2, "b.txt") {
		t.Errorf("tree2 ls-tree missing b.txt: %s", ls2)
	}
}

func TestFreezeSnapshot_EmptyIndex(t *testing.T) {
	// WriteTree on an empty index returns the well-known empty tree SHA.
	repo := t.TempDir()
	stgInitRepo(t, repo)
	// No commits, empty index — unborn repo.

	deps := stagerDeps(t, repo, provider.Manifest{})
	treeSHA, err := freezeSnapshot(context.Background(), deps)
	if err != nil {
		t.Fatalf("freezeSnapshot on empty index: %v", err)
	}
	if treeSHA == "" {
		t.Fatal("treeSHA is empty on empty index, want non-empty")
	}
	if treeSHA != "4b825dc642cb6eb9a060e54bf8d69288fbee4904" {
		t.Errorf("treeSHA = %q, want empty tree SHA 4b825dc642cb6eb9a060e54bf8d69288fbee4904", treeSHA)
	}
}

func TestFreezeSnapshot_MergeConflict(t *testing.T) {
	// Create an unresolved merge conflict in the index.
	repo := t.TempDir()
	stgInitRepo(t, repo)

	// Commit base with a.txt on the initial branch (whatever git init created).
	stgWriteFile(t, repo, "a.txt", "base\n")
	stgRunGit(t, repo, "add", "a.txt")
	stgRunGit(t, repo, "commit", "-m", "base")

	// Save initial branch name BEFORE creating the side branch.
	initialBranch := stgGitOut(t, repo, "rev-parse", "--abbrev-ref", "HEAD")

	// Branch: modify a.txt.
	stgRunGit(t, repo, "checkout", "-b", "side")
	stgWriteFile(t, repo, "a.txt", "side\n")
	stgRunGit(t, repo, "add", "a.txt")
	stgRunGit(t, repo, "commit", "-m", "side")

	// Switch back to the initial branch and modify a.txt differently.
	stgRunGit(t, repo, "checkout", initialBranch)
	stgWriteFile(t, repo, "a.txt", "main\n")
	stgRunGit(t, repo, "add", "a.txt")
	stgRunGit(t, repo, "commit", "-m", "main")

	// Attempt merge → conflict (do NOT commit/resolve).
	// Use exec.Command directly because git merge returns exit 1 on conflict,
	// and stgRunGit would t.Fatalf on non-zero exit.
	mergeCmd := exec.Command("git", "-C", repo, "merge", "side")
	mergeOut, _ := mergeCmd.CombinedOutput()
	if !strings.Contains(string(mergeOut), "CONFLICT") {
		t.Fatalf("expected merge conflict, got: %s", mergeOut)
	}

	deps := stagerDeps(t, repo, provider.Manifest{})
	_, err := freezeSnapshot(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error on merge conflict, got nil")
	}
	if !strings.Contains(err.Error(), "merge conflict") {
		t.Errorf("error does not mention 'merge conflict': %v", err)
	}
}

func TestVerifyFreezeSubset_Happy(t *testing.T) {
	// Well-behaved stager: treeI changes a.txt only (a subset of T_start with matching content).
	repo := t.TempDir()
	stgInitRepo(t, repo)
	stgCommitRaw(t, repo, "initial") // baseTree = HEAD^{tree}

	stgWriteFile(t, repo, "a.txt", "aaa\n")
	stgWriteFile(t, repo, "b.txt", "bbb\n")

	g := git.New(repo)
	ctx := context.Background()
	baseTree := stgGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	tStart, err := g.FreezeWorkingTree(ctx, baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree: %v", err)
	}

	// Stage only a.txt → WriteTree → treeI (a.txt only, subset of T_start).
	stgRunGit(t, repo, "add", "a.txt")
	treeI, err := g.WriteTree(ctx)
	if err != nil {
		t.Fatalf("WriteTree: %v", err)
	}

	tStartPaths, err := g.DiffTreeNames(ctx, baseTree, tStart)
	if err != nil {
		t.Fatalf("DiffTreeNames(base, tStart): %v", err)
	}

	deps := Deps{Git: g}
	if err := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, 0, treeI); err != nil {
		t.Fatalf("verifyFreezeSubset(happy): %v", err)
	}
}

func TestVerifyFreezeSubset_PathViolation(t *testing.T) {
	// Rogue stager stages a sentinel not in T_start → path check fires.
	repo := t.TempDir()
	stgInitRepo(t, repo)
	stgCommitRaw(t, repo, "initial")

	stgWriteFile(t, repo, "a.txt", "aaa\n")
	stgWriteFile(t, repo, "b.txt", "bbb\n")

	g := git.New(repo)
	ctx := context.Background()
	baseTree := stgGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	tStart, err := g.FreezeWorkingTree(ctx, baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree: %v", err)
	}

	// Write a sentinel AFTER the freeze, add it → treeI has sentinel ∉ T_start.
	stgWriteFile(t, repo, "sentinel.txt", "concurrent\n")
	stgRunGit(t, repo, "add", "sentinel.txt")
	treeI, err := g.WriteTree(ctx)
	if err != nil {
		t.Fatalf("WriteTree: %v", err)
	}

	tStartPaths, err := g.DiffTreeNames(ctx, baseTree, tStart)
	if err != nil {
		t.Fatalf("DiffTreeNames(base, tStart): %v", err)
	}

	deps := Deps{Git: g}
	err = verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, 0, treeI)
	if err == nil {
		t.Fatal("expected error for path violation, got nil")
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
}

func TestVerifyFreezeSubset_ContentViolation(t *testing.T) {
	// Rogue stager modifies a.txt to v2 (content not in T_start) → content check fires.
	repo := t.TempDir()
	stgInitRepo(t, repo)
	stgCommitRaw(t, repo, "initial")

	stgWriteFile(t, repo, "a.txt", "aaa\n")
	stgWriteFile(t, repo, "b.txt", "bbb\n")

	g := git.New(repo)
	ctx := context.Background()
	baseTree := stgGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	tStart, err := g.FreezeWorkingTree(ctx, baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree: %v", err)
	}

	// Modify a.txt to different content (not what T_start has) and stage it.
	stgWriteFile(t, repo, "a.txt", "modified\n")
	stgRunGit(t, repo, "add", "a.txt")
	treeI, err := g.WriteTree(ctx)
	if err != nil {
		t.Fatalf("WriteTree: %v", err)
	}

	tStartPaths, err := g.DiffTreeNames(ctx, baseTree, tStart)
	if err != nil {
		t.Fatalf("DiffTreeNames(base, tStart): %v", err)
	}

	deps := Deps{Git: g}
	err = verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, 0, treeI)
	if err == nil {
		t.Fatal("expected error for content violation, got nil")
	}
	if !errors.Is(err, ErrFreezeViolation) {
		t.Fatalf("expected ErrFreezeViolation, got %v", err)
	}
	if !strings.Contains(err.Error(), "a.txt") {
		t.Errorf("error missing 'a.txt'; got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "not traceable to T_start") {
		t.Errorf("error missing 'not traceable to T_start'; got: %s", err.Error())
	}
}

func TestVerifyFreezeSubset_EmptyStaging(t *testing.T) {
	// Empty staging (treeI == baseTree) → changedTreeI is nil → both checks no-op → nil.
	repo := t.TempDir()
	stgInitRepo(t, repo)
	stgCommitRaw(t, repo, "initial")

	g := git.New(repo)
	ctx := context.Background()
	baseTree := stgGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	// treeI = baseTree (empty index → WriteTree yields empty tree if we reset first).
	// Actually: WriteTree on a clean index (matching HEAD) yields the same tree.
	treeI := baseTree
	tStartPaths := []string{} // empty T_start (no changes)
	tStart := baseTree

	deps := Deps{Git: g}
	if err := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, 0, treeI); err != nil {
		t.Fatalf("verifyFreezeSubset(empty staging): %v", err)
	}
}

func TestStageConcept_ResolvesSubProvider(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	stgInitRepo(t, repo)

	m := tooledStubManifest(t, bin, stubtest.Options{Out: ""}) // stager ignores stdout; exit 0
	pflag := "--provider"
	m.ProviderFlag = &pflag // pi-shaped: ProviderFlag triggers slash-prefix splitting
	mf := "--model"
	m.ModelFlag = &mf
	dm := "gpt-5.4"
	m.DefaultModel = &dm

	deps := stagerDeps(t, repo, m)
	deps.Config.Provider = "pi"              // the manifest NAME — the conflation source; must NOT be emitted
	deps.Config.Model = "openrouter/gpt-5.4" // slash-prefix model → Render emits --provider openrouter

	var buf bytes.Buffer
	deps.Verbose = ui.NewVerbose(&buf, true)

	concept := prompt.PlannerCommit{Title: "feat: x", Description: "stage a.txt"}
	if err := stageConcept(context.Background(), deps, concept); err != nil {
		t.Fatalf("stageConcept: %v", err)
	}

	cmd := buf.String()
	if !strings.Contains(cmd, "--provider openrouter") {
		t.Errorf("stager command missing --provider openrouter\ngot: %s", cmd)
	}
	if strings.Contains(cmd, "--provider pi") {
		t.Errorf("stager command emits manifest name as sub-provider (conflation)\ngot: %s", cmd)
	}
}
