package decompose

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/generate"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/provider"
	"github.com/dabstractor/stagecoach/internal/stubtest"
)

// --- Fixture helpers (chn*-prefixed to avoid collisions with arb*/stg*/msg*/un-prefixed) ---

// chnInitRepo creates a git repo in dir with repo-local identity config.
func chnInitRepo(t *testing.T, dir string) {
	t.Helper()
	chnRunGit(t, dir, "init")
	chnRunGit(t, dir, "config", "user.name", "Test")
	chnRunGit(t, dir, "config", "user.email", "test@example.com")
}

// chnWriteFile creates a file at dir/name with the given body.
func chnWriteFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := dir + string(os.PathSeparator) + name
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("chnWriteFile %s: %v", full, err)
	}
}

// chnStageFile runs git add for name in dir.
func chnStageFile(t *testing.T, dir, name string) {
	t.Helper()
	chnRunGit(t, dir, "add", name)
}

// chnCommitRaw creates an empty commit with the given message.
func chnCommitRaw(t *testing.T, dir, msg string) {
	t.Helper()
	chnRunGit(t, dir, "commit", "--allow-empty", "-m", msg)
}

// chnRunGit executes git -C dir args... and returns trimmed stdout.
func chnRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// chnHeadSHA runs git rev-parse HEAD in dir and returns the trimmed SHA.
func chnHeadSHA(t *testing.T, dir string) string {
	t.Helper()
	return chnRunGit(t, dir, "rev-parse", "HEAD")
}

// chnDeps builds a minimal Deps for chain tests (no ResolveRoles).
func chnDeps(t *testing.T, repo string, msgManifest provider.Manifest) Deps {
	t.Helper()
	return Deps{
		Git:     git.New(repo),
		Config:  config.Defaults(),
		Roles:   RoleManifests{Message: msgManifest},
		Verbose: nil,
	}
}

// chnBuildChain builds a 3-commit chain (C0, C1, C2) with distinct files, returns the parallel
// []CommitInfo + []ChainEntry arrays PLUS the frozen tStart tree (tree2 + leftover.go) and the
// leftoverPaths set (DiffTreeNames(tree2, tStart) == ["leftover.go"]). Each commit carries a unique
// file so the tree is easy to reason about. Leaves a leftover file "leftover.go" staged into tStart
// (the working tree is then restored to tree2 so the test's starting state is a clean index == tree2).
func chnBuildChain(t *testing.T, repo string) (commits []CommitInfo, chainData []ChainEntry, tStart string, leftoverPaths []string) {
	t.Helper()
	// C0: commit c0.go
	chnWriteFile(t, repo, "c0.go", "package c0\n")
	chnStageFile(t, repo, "c0.go")
	chnCommitRaw(t, repo, "feat: add c0")
	sha0 := chnHeadSHA(t, repo)
	tree0 := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	msg0 := chnRunGit(t, repo, "log", "--format=%B", "-1")

	// C1: commit c1.go
	chnWriteFile(t, repo, "c1.go", "package c1\n")
	chnStageFile(t, repo, "c1.go")
	chnCommitRaw(t, repo, "feat: add c1")
	sha1 := chnHeadSHA(t, repo)
	tree1 := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	msg1 := chnRunGit(t, repo, "log", "--format=%B", "-1")

	// C2 (tip): commit c2.go
	chnWriteFile(t, repo, "c2.go", "package c2\n")
	chnStageFile(t, repo, "c2.go")
	chnCommitRaw(t, repo, "feat: add c2")
	sha2 := chnHeadSHA(t, repo)
	tree2 := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	msg2 := chnRunGit(t, repo, "log", "--format=%B", "-1")

	// Create leftover (untracked, NOT staged/committed).
	chnWriteFile(t, repo, "leftover.go", "package leftover\n")

	// Build tStart = tree2 + leftover.go (the frozen working-tree-as-of-run-start). Stage leftover.go,
	// capture write-tree as tStart, then read-tree tree2 to restore a clean index == tree2 (so the tests'
	// starting state is unchanged: working tree holds untracked leftover.go, index == HEAD.tree == tree2).
	chnStageFile(t, repo, "leftover.go")
	tStart = chnRunGit(t, repo, "write-tree")
	chnRunGit(t, repo, "read-tree", tree2)
	chnRunGit(t, repo, "rm", "--cached", "--ignore-unmatch", "leftover.go")

	leftoverPaths = []string{"leftover.go"}

	commits = []CommitInfo{
		{SHA: sha0, Subject: "feat: add c0", Files: nil},
		{SHA: sha1, Subject: "feat: add c1", Files: nil},
		{SHA: sha2, Subject: "feat: add c2", Files: nil},
	}
	chainData = []ChainEntry{
		{SHA: sha0, Tree: tree0, Message: msg0, Parent: ""}, // root — parent was pre-repo HEAD (empty)
		{SHA: sha1, Tree: tree1, Message: msg1, Parent: sha0},
		{SHA: sha2, Tree: tree2, Message: msg2, Parent: sha1},
	}
	return commits, chainData, tStart, leftoverPaths
}

// chnBuildChainWithSentinel is chnBuildChain PLUS a post-freeze sentinel.go (untracked, NOT in
// tStart). It exists to prove resolveArbiter builds every arbiter commit's tree from tStart ONLY —
// the sentinel, written AFTER tStart was captured, can never be swept in (FR-M1d / PRD §20.2
// "Arbiter freeze parity").
//
// This is a SEPARATE variant (not an in-place extension of chnBuildChain) because 7 existing
// TestResolveArbiter_* tests assert `git status == ""` (clean) against chnBuildChain's output;
// writing an untracked sentinel in-place would break all of them (Decision D1).
func chnBuildChainWithSentinel(t *testing.T, repo string) (commits []CommitInfo, chainData []ChainEntry, tStart string, leftoverPaths []string) {
	t.Helper()
	commits, chainData, tStart, leftoverPaths = chnBuildChain(t, repo)
	// Post-freeze sentinel: written UNSTAGED (NO git add). chnBuildChain already restored the
	// index == tree2 (clean) and captured tStart = tree2 + leftover.go, so an unstaged sentinel.go
	// is outside tStart by construction AND outside the index. resolveArbiter builds every arbiter
	// tree from tStart / frozen tree[j] ONLY, so sentinel.go can never enter any commit.
	chnWriteFile(t, repo, "sentinel.go", "package sentinel\n")
	return commits, chainData, tStart, leftoverPaths
}

// --- Tests ---

func TestResolveArbiter_NullNewCommit(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData, tStart, leftoverPaths := chnBuildChain(t, repo)

	m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: leftover"})
	deps := chnDeps(t, repo, m)

	N := len(chainData)
	tipSHA := chainData[N-1].SHA

	err := resolveArbiter(context.Background(), deps, nil, commits, chainData, tStart, leftoverPaths)
	if err != nil {
		t.Fatalf("resolveArbiter(nil): %v", err)
	}

	// Should have N+1 commits now.
	count := strings.TrimSpace(chnRunGit(t, repo, "rev-list", "--count", "HEAD"))
	if count != "4" { // was 3, now 4
		t.Fatalf("commit count = %s, want 4", count)
	}

	// HEAD's subject should be the generated message.
	subject := chnRunGit(t, repo, "log", "--format=%s", "-1")
	if subject != "chore: leftover" {
		t.Fatalf("HEAD subject = %q, want \"chore: leftover\"", subject)
	}

	// HEAD's parent should be the old tip.
	parent := chnRunGit(t, repo, "log", "--format=%P", "-1")
	if parent != tipSHA {
		t.Fatalf("HEAD parent = %q, want %q", parent, tipSHA)
	}

	// git status should be clean.
	status := chnRunGit(t, repo, "status", "--porcelain")
	if status != "" {
		t.Fatalf("git status not clean: %s", status)
	}

	// HEAD.tree should contain leftover.go.
	tree := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	ls := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", tree)
	if !strings.Contains(ls, "leftover.go") {
		t.Fatalf("HEAD tree missing leftover.go; ls-tree: %s", ls)
	}
}

func TestResolveArbiter_TipAmend(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData, tStart, leftoverPaths := chnBuildChain(t, repo)

	// Use a message manifest that returns something different to prove tip amend doesn't call it.
	m := stubtest.Manifest(bin, stubtest.Options{Out: "SHOULD NOT BE USED"})
	deps := chnDeps(t, repo, m)

	N := len(chainData)
	tip := chainData[N-1]
	tipSHA := tip.SHA
	tipMsg := strings.TrimSpace(tip.Message)
	tipParent := tip.Parent

	target := tipSHA
	err := resolveArbiter(context.Background(), deps, &target, commits, chainData, tStart, leftoverPaths)
	if err != nil {
		t.Fatalf("resolveArbiter(&tipSHA): %v", err)
	}

	// Should STILL have 3 commits (no extra).
	count := strings.TrimSpace(chnRunGit(t, repo, "rev-list", "--count", "HEAD"))
	if count != "3" {
		t.Fatalf("commit count = %s, want 3 (amend should not add)", count)
	}

	// HEAD's subject should be the ORIGINAL tip subject (reused verbatim).
	subject := chnRunGit(t, repo, "log", "--format=%s", "-1")
	if subject != "feat: add c2" {
		t.Fatalf("HEAD subject = %q, want \"feat: add c2\" (original)", subject)
	}

	// HEAD's parent should be the tip's original parent.
	parent := chnRunGit(t, repo, "log", "--format=%P", "-1")
	if parent != tipParent {
		t.Fatalf("HEAD parent = %q, want %q (tip's original parent)", parent, tipParent)
	}

	// HEAD.tree should contain both c2.go AND leftover.go.
	tree := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	ls := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", tree)
	if !strings.Contains(ls, "c2.go") {
		t.Fatalf("HEAD tree missing c2.go; ls-tree: %s", ls)
	}
	if !strings.Contains(ls, "leftover.go") {
		t.Fatalf("HEAD tree missing leftover.go; ls-tree: %s", ls)
	}

	// git status should be clean.
	status := chnRunGit(t, repo, "status", "--porcelain")
	if status != "" {
		t.Fatalf("git status not clean: %s", status)
	}

	// Verify the original tip message was preserved (not regenerated).
	fullMsg := chnRunGit(t, repo, "log", "--format=%B", "-1")
	if strings.TrimSpace(fullMsg) != tipMsg {
		t.Fatalf("message was regenerated; got %q, want %q", strings.TrimSpace(fullMsg), tipMsg)
	}
}

func TestResolveArbiter_MidChainRebuild(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData, tStart, leftoverPaths := chnBuildChain(t, repo)

	m := stubtest.Manifest(bin, stubtest.Options{Out: "SHOULD NOT BE USED"})
	deps := chnDeps(t, repo, m)

	// Target C1 (index 1, i=1).
	sha1 := chainData[1].SHA
	sha0 := chainData[0].SHA

	target := sha1
	err := resolveArbiter(context.Background(), deps, &target, commits, chainData, tStart, leftoverPaths)
	if err != nil {
		t.Fatalf("resolveArbiter(&sha1): %v", err)
	}

	// Should STILL have 3 commits (no extra).
	count := strings.TrimSpace(chnRunGit(t, repo, "rev-list", "--count", "HEAD"))
	if count != "3" {
		t.Fatalf("commit count = %s, want 3 (mid-chain rebuild should not add)", count)
	}

	// C0 should be UNCHANGED (same SHA).
	// The rebuilt C1' and C2' should have new SHAs.
	shas := strings.Split(chnRunGit(t, repo, "log", "--format=%H", "--reverse"), "\n")
	if len(shas) != 3 {
		t.Fatalf("expected 3 SHAs, got %d", len(shas))
	}
	if shas[0] != sha0 {
		t.Fatalf("C0 changed: was %q, now %q (should be unchanged)", sha0, shas[0])
	}
	if shas[1] == sha1 {
		t.Fatal("C1 was NOT rebuilt — same SHA as before")
	}

	// git status should be CLEAN (no leftover reverted).
	status := chnRunGit(t, repo, "status", "--porcelain")
	if status != "" {
		t.Fatalf("git status not clean: %s (fold-at-every-j failed?)", status)
	}

	// FINAL HEAD.tree should contain leftover.go.
	headTree := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	lsHead := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", headTree)
	if !strings.Contains(lsHead, "leftover.go") {
		t.Fatalf("HEAD tree missing leftover.go; ls-tree: %s", lsHead)
	}

	// C1' tree should also contain leftover.go (folded at j==i).
	c1tree := chnRunGit(t, repo, "rev-parse", shas[1]+"^{tree}")
	lsC1 := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", c1tree)
	if !strings.Contains(lsC1, "leftover.go") {
		t.Fatalf("C1' tree missing leftover.go (fold-at-every-j correction not applied); ls-tree: %s", lsC1)
	}

	// C2' tree should also contain leftover.go (folded at j==i+1).
	c2tree := chnRunGit(t, repo, "rev-parse", shas[2]+"^{tree}")
	lsC2 := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", c2tree)
	if !strings.Contains(lsC2, "leftover.go") {
		t.Fatalf("C2' tree missing leftover.go (fold-at-every-j correction not applied); ls-tree: %s", lsC2)
	}

	// Subjects should be preserved verbatim.
	subj1 := chnRunGit(t, repo, "log", "--format=%s", "-2", "--skip=1")
	if !strings.Contains(subj1, "feat: add c1") {
		t.Fatalf("C1' subject wrong: %s", subj1)
	}
	subj2 := chnRunGit(t, repo, "log", "--format=%s", "-1")
	if subj2 != "feat: add c2" {
		t.Fatalf("C2' subject = %q, want \"feat: add c2\"", subj2)
	}
}

func TestResolveArbiter_TargetNotFound(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData, tStart, leftoverPaths := chnBuildChain(t, repo)

	m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: leftover"})
	deps := chnDeps(t, repo, m)

	// Bogus SHA — should degrade to null (new commit path).
	bogus := "0123456789abcdef0123456789abcdef01234567"
	err := resolveArbiter(context.Background(), deps, &bogus, commits, chainData, tStart, leftoverPaths)
	if err != nil {
		t.Fatalf("resolveArbiter(bogus): %v", err)
	}

	// Should have N+1 commits (same as null path).
	count := strings.TrimSpace(chnRunGit(t, repo, "rev-list", "--count", "HEAD"))
	if count != "4" {
		t.Fatalf("commit count = %s, want 4 (target-not-found → new commit)", count)
	}
}

func TestResolveArbiter_CASFailure(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData, tStart, leftoverPaths := chnBuildChain(t, repo)

	m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: leftover"})
	deps := chnDeps(t, repo, m)

	// Move HEAD externally between resolveArbiter's tree-build and its UpdateRefCAS.
	// We do this by making a concurrent commit BEFORE calling resolveArbiter — wait,
	// resolveArbiter is synchronous. Instead, we directly test by manipulating the expectedOld.
	// The cleanest way: make an external commit AFTER the tree is built but before UpdateRefCAS.
	// But resolveArbiter is atomic in its sequence.
	//
	// Instead, we use a wrapper: call resolveArbiter but with an external commit injected
	// between. Since resolveArbiter is synchronous, we need to test the CAS path differently.
	//
	// Strategy: build the chain, then move HEAD externally (make a commit), then call
	// resolveArbiter. The tipSHA in chainData won't match HEAD anymore → CAS fails.

	// Make an external commit to move HEAD.
	chnWriteFile(t, repo, "external.go", "package external\n")
	chnStageFile(t, repo, "external.go")
	chnCommitRaw(t, repo, "external: moved HEAD")
	movedHEAD := chnHeadSHA(t, repo)

	err := resolveArbiter(context.Background(), deps, nil, commits, chainData, tStart, leftoverPaths)
	if err == nil {
		t.Fatal("resolveArbiter returned nil on CAS failure")
	}

	// Should be *generate.CASError (errors.As-able).
	var ce *generate.CASError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %T (%v), want *generate.CASError", err, err)
	}
	if ce.Expected != chainData[2].SHA {
		t.Errorf("CASError.Expected = %q, want %q (tipSHA)", ce.Expected, chainData[2].SHA)
	}
	if ce.Actual != movedHEAD {
		t.Errorf("CASError.Actual = %q, want %q (moved HEAD)", ce.Actual, movedHEAD)
	}

	// HEAD should NOT have been force-updated.
	currentHEAD := chnHeadSHA(t, repo)
	if currentHEAD != movedHEAD {
		t.Errorf("HEAD changed to %q after CAS failure (should be unchanged at %q)", currentHEAD, movedHEAD)
	}
}

func TestResolveArbiter_RescueErrorPropagation(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData, tStart, leftoverPaths := chnBuildChain(t, repo)

	// Stub that sleeps longer than the timeout → generateMessage times out.
	m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: leftover", SleepMS: 2000})
	cfg := config.Defaults()
	cfg.Timeout = 100 * time.Millisecond

	deps := chnDeps(t, repo, m)
	deps.Config = cfg

	err := resolveArbiter(context.Background(), deps, nil, commits, chainData, tStart, leftoverPaths)
	if err == nil {
		t.Fatal("resolveArbiter returned nil on timeout")
	}

	// Should be *generate.RescueError (errors.As-able, NOT wrapped in ErrArbiterResolutionFailed).
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %T (%v), want *generate.RescueError", err, err)
	}

	// ErrArbiterResolutionFailed should NOT be wrapping it.
	if errors.Is(err, ErrArbiterResolutionFailed) {
		t.Error("RescueError was wrapped in ErrArbiterResolutionFailed — should be propagated directly")
	}
}

func TestFindTargetIndex(t *testing.T) {
	cd := []ChainEntry{
		{SHA: "aaa"},
		{SHA: "bbb"},
		{SHA: "ccc"},
	}

	tests := []struct {
		sha  string
		want int
	}{
		{"aaa", 0},
		{"bbb", 1},
		{"ccc", 2},
		{"ddd", -1},
		{"", -1},
	}

	for _, tc := range tests {
		got := findTargetIndex(tc.sha, cd)
		if got != tc.want {
			t.Errorf("findTargetIndex(%q) = %d, want %d", tc.sha, got, tc.want)
		}
	}
}

func TestResolveArbiter_CleanTreePostcondition(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)

	// Build a simple 2-commit chain with a leftover.
	chnWriteFile(t, repo, "a.go", "package a\n")
	chnStageFile(t, repo, "a.go")
	chnCommitRaw(t, repo, "feat: add a")
	sha0 := chnHeadSHA(t, repo)
	tree0 := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	msg0 := chnRunGit(t, repo, "log", "--format=%B", "-1")

	chnWriteFile(t, repo, "b.go", "package b\n")
	chnStageFile(t, repo, "b.go")
	chnCommitRaw(t, repo, "feat: add b")
	sha1 := chnHeadSHA(t, repo)
	tree1 := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	msg1 := chnRunGit(t, repo, "log", "--format=%B", "-1")

	// Leftover.
	chnWriteFile(t, repo, "leftover.go", "package leftover\n")

	// Build tStart = tree1 + leftover.go (stage leftover → write-tree → restore clean index == tree1).
	chnStageFile(t, repo, "leftover.go")
	tStart := chnRunGit(t, repo, "write-tree")
	chnRunGit(t, repo, "read-tree", tree1)
	chnRunGit(t, repo, "rm", "--cached", "--ignore-unmatch", "leftover.go")
	leftoverPaths := []string{"leftover.go"}

	commits := []CommitInfo{
		{SHA: sha0, Subject: "feat: add a", Files: nil},
		{SHA: sha1, Subject: "feat: add b", Files: nil},
	}
	chainData := []ChainEntry{
		{SHA: sha0, Tree: tree0, Message: msg0, Parent: ""},
		{SHA: sha1, Tree: tree1, Message: msg1, Parent: sha0},
	}

	m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: leftover"})
	deps := chnDeps(t, repo, m)

	err := resolveArbiter(context.Background(), deps, nil, commits, chainData, tStart, leftoverPaths)
	if err != nil {
		t.Fatalf("resolveArbiter: %v", err)
	}

	// Verify the 3 clean-tree postconditions:
	// 1. git status --porcelain == ""
	status := chnRunGit(t, repo, "status", "--porcelain")
	if status != "" {
		t.Fatalf("git status not clean: %s", status)
	}
	// 2. index == HEAD.tree (write-tree matches HEAD.tree)
	indexTree := chnRunGit(t, repo, "write-tree")
	headTree := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	if indexTree != headTree {
		t.Errorf("index tree %q != HEAD tree %q", indexTree, headTree)
	}
	// 3. working tree == index (no unstaged changes — already verified by clean status)
}

// TestResolveArbiter_FreezeParitySentinelExcluded is the permanent FR-M1d / PRD §20.2 "Arbiter
// freeze parity" regression net for the arbiter's three resolution paths (new / tip-amend /
// mid-chain). It proves that EVERY arbiter commit's tree is built strictly from tStart (or an
// OverlayTreePaths overlay of frozen trees): a sentinel.go written AFTER tStart capture
// (post-freeze, NOT in tStart) is swept into NO arbiter commit and remains untracked, while the
// legitimate frozen leftover (leftover.go, IS in tStart) IS folded. HEAD^{tree} == tStart is a
// UNIFORM invariant across all three paths (null/tip set treePrime := tStart; mid rebuilds the
// tip via OverlayTreePaths(tree[N-1], tStart, leftoverPaths) which equals tStart).
//
// Fresh repo per target: resolveArbiter advances HEAD via UpdateRefCAS, so a second call on the
// same repo would read a STALE tipSHA from chainData and fail the CAS (Decision D4).
func TestResolveArbiter_FreezeParitySentinelExcluded(t *testing.T) {
	bin := stubtest.Build(t)
	targets := []struct {
		name   string
		target func(c []CommitInfo) *string
	}{
		// Path A (new commit): target nil → resolveNewCommit, treePrime := tStart.
		{"null", func(_ []CommitInfo) *string { return nil }},
		// Path B (tip amend): target &C2.SHA (idx == N-1) → resolveTipAmend, treePrime := tStart.
		{"tip", func(c []CommitInfo) *string { s := c[2].SHA; return &s }},
		// Path C (mid-chain rebuild): target &C1.SHA (idx < N-1) → resolveMidChain,
		// treePrime = OverlayTreePaths(tree[j], tStart, leftoverPaths) per j ⇒ tip == tStart.
		{"mid", func(c []CommitInfo) *string { s := c[1].SHA; return &s }},
	}
	for _, tc := range targets {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			chnInitRepo(t, repo) // FRESH repo per target — resolveArbiter advances HEAD
			commits, chainData, tStart, leftoverPaths := chnBuildChainWithSentinel(t, repo)

			// Only the Message role is exercised (null path's generateMessage; tip/mid reuse the
			// original message verbatim and never call the agent).
			deps := chnDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{Out: "chore: arbiter leftover"}))

			target := tc.target(commits)
			if err := resolveArbiter(context.Background(), deps, target, commits, chainData, tStart, leftoverPaths); err != nil {
				t.Fatalf("resolveArbiter(%s): %v", tc.name, err)
			}

			// (a) HEAD^{tree} == tStart — the "exactly T_start" proof, UNIFORM across all three
			// paths (for mid it is also the "rebuilt tip == T_start" proof; Decision D2).
			if got := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}"); got != tStart {
				t.Errorf("%s: HEAD^{tree} = %s, want exactly tStart = %s", tc.name, got, tStart)
			}

			// (b) sentinel.go in NO commit (covers loop commits + arbiter commit + every rebuilt
			// Cj'^{tree} for mid — Decision D3).
			if names := chnRunGit(t, repo, "log", "--name-only", "--format="); strings.Contains(names, "sentinel.go") {
				t.Errorf("%s: sentinel.go swept into a commit (freeze parity violated):\n%s", tc.name, names)
			}

			// (c) sentinel.go REMAINS untracked — ReadTree(tStart) synced the index to tStart, which
			// excludes the post-freeze sentinel; the run left it untouched in the working tree.
			if status := chnRunGit(t, repo, "status", "--porcelain"); !strings.Contains(status, "?? sentinel.go") {
				t.Errorf("%s: status = %q, want it to contain '?? sentinel.go'", tc.name, status)
			}

			// (d) leftover.go DID land in HEAD^{tree} — proves the arbiter folded the legitimate
			// frozen leftover (it ran, not a no-op). Distinguishes this proof from an arbiter skip.
			if ls := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", "HEAD"); !strings.Contains(ls, "leftover.go") {
				t.Errorf("%s: leftover.go missing from HEAD tree — arbiter did not fold the frozen leftover:\n%s", tc.name, ls)
			}
		})
	}
}

// --- resolveArbiter hook wiring tests (P1.M3.T3.S1 — PRD §9.25 FR-V1/V8c + §20.2 fidelity) ---

// chnInstallHook writes an executable hook script to <repo>/.git/hooks/<name>, mode 0755 (the
// owner-exec bit is what hookExecutable checks — without it the hook is skipped and the test is
// vacuous). Mirrors internal/generate/hooks_freeze_test.go's hook-install idiom.
func chnInstallHook(t *testing.T, repo, name, body string) {
	t.Helper()
	hooksDir := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, name), []byte(body), 0o755); err != nil {
		t.Fatalf("write %s hook: %v", name, err)
	}
}

// chnBuildChainWithHook runs chnBuildChain then installs a prepare-commit-msg that appends
// [HOOK-RAN] to the message file. Returns everything chnBuildChain does. Used by the three hook
// tests so they share the SAME setup (only the target differs).
func chnBuildChainWithHook(t *testing.T, repo string) (commits []CommitInfo, chainData []ChainEntry, tStart string, leftoverPaths []string) {
	t.Helper()
	commits, chainData, tStart, leftoverPaths = chnBuildChain(t, repo)
	chnInstallHook(t, repo, "prepare-commit-msg", "#!/bin/sh\necho '[HOOK-RAN]' >> \"$1\"\n")
	return commits, chainData, tStart, leftoverPaths
}

// TestResolveArbiter_NullNewCommit_RunsHooks proves resolveNewCommit (path A, null target) runs the
// repo's commit hooks: the N+1 commit's message carries the [HOOK-RAN] append (hooks ran + the
// annotated finalMsg was committed). PRD §9.25 FR-V1/V8c.
func TestResolveArbiter_NullNewCommit_RunsHooks(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData, tStart, leftoverPaths := chnBuildChainWithHook(t, repo)

	m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: leftover"})
	deps := chnDeps(t, repo, m)

	if err := resolveArbiter(context.Background(), deps, nil, commits, chainData, tStart, leftoverPaths); err != nil {
		t.Fatalf("resolveArbiter(nil): %v", err)
	}

	// The N+1 commit's message should carry the [HOOK-RAN] append.
	headMsg := chnRunGit(t, repo, "log", "--format=%B", "-1")
	if !strings.Contains("\n"+headMsg+"\n", "\n[HOOK-RAN]\n") {
		t.Errorf("HEAD message = %q, want [HOOK-RAN] on its own line (resolveNewCommit ran hooks — Issue 2 parity)", headMsg)
	}
}

// TestResolveArbiter_TipAmend_RunsHooks proves resolveTipAmend (path B, target==tip) runs the repo's
// commit hooks: the amended tip's message carries the [HOOK-RAN] append. amend parity (§5): the tip
// message is the hook INPUT and prepare-commit-msg MAY annotate it — mirrors `git commit --amend`
// re-running the msg hooks. The tip is the arbiter's TARGET, so its message MAY change.
func TestResolveArbiter_TipAmend_RunsHooks(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData, tStart, leftoverPaths := chnBuildChainWithHook(t, repo)

	m := stubtest.Manifest(bin, stubtest.Options{Out: "SHOULD NOT BE USED"})
	deps := chnDeps(t, repo, m)

	tipSHA := chainData[len(chainData)-1].SHA
	target := tipSHA
	if err := resolveArbiter(context.Background(), deps, &target, commits, chainData, tStart, leftoverPaths); err != nil {
		t.Fatalf("resolveArbiter(&tipSHA): %v", err)
	}

	// The amended tip's message should carry the [HOOK-RAN] append (amend re-runs msg hooks).
	headMsg := chnRunGit(t, repo, "log", "--format=%B", "-1")
	if !strings.Contains("\n"+headMsg+"\n", "\n[HOOK-RAN]\n") {
		t.Errorf("amended tip message = %q, want [HOOK-RAN] on its own line (resolveTipAmend ran hooks — amend parity, Issue 2)", headMsg)
	}
}

// TestResolveArbiter_MidChain_SkipsHooks is THE §20.2 mid-chain-fidelity acceptance test. It proves
// resolveMidChain (path C, target==earlier commit[i], i<N-1) is HOOK-FREE: a prepare-commit-msg that
// appends [HOOK-RAN] does NOT touch the rebuilt commits' messages (msg[j] reused VERBATIM). If a
// rebuilt commit carries the marker, resolveMidChain was wired to hooks — the §20.2 invariant
// (rebuilt non-target commits are byte-identical to the originals) is broken.
func TestResolveArbiter_MidChain_SkipsHooks(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData, tStart, leftoverPaths := chnBuildChainWithHook(t, repo)

	m := stubtest.Manifest(bin, stubtest.Options{Out: "SHOULD NOT BE USED"})
	deps := chnDeps(t, repo, m)

	// Target C1 (index 1, i=1 < N-1=2) ⇒ mid-chain rebuild. resolveMidChain rebuilds C1' and C2'.
	target := chainData[1].SHA
	if err := resolveArbiter(context.Background(), deps, &target, commits, chainData, tStart, leftoverPaths); err != nil {
		t.Fatalf("resolveArbiter(&sha1): %v", err)
	}

	// Walk the rebuilt commits (from C1' onward) and assert NONE carry the [HOOK-RAN] marker.
	// resolveMidChain reused msg[j] VERBATIM (no hooks) ⇒ byte-identical messages (§20.2).
	shas := strings.Split(chnRunGit(t, repo, "log", "--format=%H", "--reverse"), "\n")
	if len(shas) != 3 {
		t.Fatalf("expected 3 SHAs, got %d", len(shas))
	}
	for j := 1; j < len(shas); j++ { // j=1 (C1') and j=2 (C2') — the rebuilt commits
		msg := chnRunGit(t, repo, "log", "--format=%B", "-1", shas[j])
		if strings.Contains(msg, "[HOOK-RAN]") {
			t.Errorf("rebuilt commit[%d] message carries [HOOK-RAN] — resolveMidChain must be hook-free (§20.2 fidelity): %q", j, msg)
		}
	}
}
