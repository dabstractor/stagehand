package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fileExists reports whether path exists on disk.
func freezeFileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

// freezeReadFile reads and returns the contents of path, fatal on error.
func freezeReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// TestFreezeWorkingTree_CapturesFullChangeSet verifies that FreezeWorkingTree captures T_start
// whose git diff-tree vs baseTree lists EXACTLY the working-tree change set (mod + add + del).
func TestFreezeWorkingTree_CapturesFullChangeSet(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "a\n")
	writeFile(t, repo, "b.txt", "b\n")
	stageFile(t, repo, "a.txt")
	stageFile(t, repo, "b.txt")
	makeEmptyCommit(t, repo, "base")
	baseTree := execGit(t, repo, "rev-parse", "HEAD^{tree}")

	// Un-staged changes: modify a.txt, add c.txt, delete b.txt.
	writeFile(t, repo, "a.txt", "a-modified\n")
	writeFile(t, repo, "c.txt", "c-new\n")
	// Delete b.txt from the working tree.
	if err := os.Remove(filepath.Join(repo, "b.txt")); err != nil {
		t.Fatalf("rm b.txt: %v", err)
	}

	g := New(repo)
	tStart, err := g.FreezeWorkingTree(context.Background(), baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree err = %v, want nil", err)
	}
	if tStart == "" || tStart == baseTree {
		t.Fatalf("tStart = %q, want a new non-empty tree SHA", tStart)
	}

	// Oracle: tStart must be a tree object.
	objType := execGit(t, repo, "cat-file", "-t", tStart)
	if objType != "tree" {
		t.Fatalf("cat-file -t tStart = %q, want \"tree\"", objType)
	}

	// Oracle: diff-tree baseTree tStart must list exactly {a.txt, b.txt, c.txt}.
	changed := execGit(t, repo, "diff-tree", "-r", "--name-only", "--no-commit-id", baseTree, tStart)
	got := strings.Split(strings.TrimSpace(changed), "\n")
	sort.Strings(got)
	want := []string{"a.txt", "b.txt", "c.txt"}
	if len(got) != len(want) {
		t.Fatalf("diff-tree paths = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("diff-tree paths[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestFreezeWorkingTree_ResetsIndexToBase verifies that after FreezeWorkingTree returns,
// the index == baseTree (verified via an independent git ls-files oracle).
func TestFreezeWorkingTree_ResetsIndexToBase(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "a\n")
	writeFile(t, repo, "b.txt", "b\n")
	stageFile(t, repo, "a.txt")
	stageFile(t, repo, "b.txt")
	makeEmptyCommit(t, repo, "base")
	baseTree := execGit(t, repo, "rev-parse", "HEAD^{tree}")

	// Un-staged changes: modify a.txt, add c.txt, delete b.txt.
	writeFile(t, repo, "a.txt", "a-modified\n")
	writeFile(t, repo, "c.txt", "c-new\n")
	if err := os.Remove(filepath.Join(repo, "b.txt")); err != nil {
		t.Fatalf("rm b.txt: %v", err)
	}

	g := New(repo)
	_, err := g.FreezeWorkingTree(context.Background(), baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree err = %v, want nil", err)
	}

	// Oracle: git ls-files must show baseTree's files (a.txt, b.txt).
	// ReadTree(baseTree) reset the index to base — b.txt is STILL in the index because
	// the baseTree was captured BEFORE the working-tree delete. This pins the index-reset semantics.
	ls := execGit(t, repo, "ls-files")
	got := strings.Split(strings.TrimSpace(ls), "\n")
	sort.Strings(got)
	want := []string{"a.txt", "b.txt"}
	if len(got) != len(want) {
		t.Fatalf("ls-files = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("ls-files[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestFreezeWorkingTree_LeavesWorkingTreeUnchanged verifies that after FreezeWorkingTree returns,
// the working-tree files on disk are UNCHANGED (read-tree only rewrites .git/index).
func TestFreezeWorkingTree_LeavesWorkingTreeUnchanged(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "a\n")
	writeFile(t, repo, "b.txt", "b\n")
	stageFile(t, repo, "a.txt")
	stageFile(t, repo, "b.txt")
	makeEmptyCommit(t, repo, "base")
	baseTree := execGit(t, repo, "rev-parse", "HEAD^{tree}")

	// Un-staged changes.
	writeFile(t, repo, "a.txt", "a-modified\n")
	writeFile(t, repo, "c.txt", "c-new\n")
	if err := os.Remove(filepath.Join(repo, "b.txt")); err != nil {
		t.Fatalf("rm b.txt: %v", err)
	}

	g := New(repo)
	_, err := g.FreezeWorkingTree(context.Background(), baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree err = %v, want nil", err)
	}

	// Oracle: working-tree files must be unchanged.
	got := freezeReadFile(t, filepath.Join(repo, "a.txt"))
	if got != "a-modified\n" {
		t.Fatalf("a.txt content = %q, want \"a-modified\\n\"", got)
	}
	if !freezeFileExists(t, filepath.Join(repo, "c.txt")) {
		t.Fatal("c.txt does not exist, want it present (freeze does not touch working tree)")
	}
	if freezeFileExists(t, filepath.Join(repo, "b.txt")) {
		t.Fatal("b.txt still exists on disk, want it deleted (freeze does not touch working tree)")
	}
}

// TestFreezeWorkingTree_UnbornEmptyTreeBase verifies the unborn case:
// baseTree=EmptyTreeSHA → T_start holds all untracked files; ls-files is EMPTY after reset.
func TestFreezeWorkingTree_UnbornEmptyTreeBase(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo) // zero commits — unborn
	writeFile(t, repo, "x.txt", "x\n")
	writeFile(t, repo, "y.txt", "y\n")

	g := New(repo)
	tStart, err := g.FreezeWorkingTree(context.Background(), EmptyTreeSHA)
	if err != nil {
		t.Fatalf("FreezeWorkingTree err = %v, want nil", err)
	}
	if tStart == "" {
		t.Fatal("tStart is empty, want a non-empty tree SHA")
	}

	// Oracle: diff-tree EmptyTreeSHA tStart lists {x.txt, y.txt}.
	changed := execGit(t, repo, "diff-tree", "-r", "--name-only", "--no-commit-id", EmptyTreeSHA, tStart)
	got := strings.Split(strings.TrimSpace(changed), "\n")
	sort.Strings(got)
	want := []string{"x.txt", "y.txt"}
	if len(got) != len(want) {
		t.Fatalf("diff-tree paths = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("diff-tree paths[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// Oracle: ls-files must be empty (ReadTree(EmptyTreeSHA) resets index to empty).
	ls := execGit(t, repo, "ls-files")
	if ls != "" {
		t.Fatalf("ls-files = %q, want empty (unborn index reset to empty)", ls)
	}
}

// TestFreezeWorkingTree_NoChangesIdempotent verifies that on a clean working tree,
// FreezeWorkingTree returns tStart == baseTree.
func TestFreezeWorkingTree_NoChangesIdempotent(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "a\n")
	stageFile(t, repo, "a.txt")
	makeEmptyCommit(t, repo, "base")
	baseTree := execGit(t, repo, "rev-parse", "HEAD^{tree}")

	g := New(repo)
	tStart, err := g.FreezeWorkingTree(context.Background(), baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree err = %v, want nil", err)
	}
	if tStart != baseTree {
		t.Fatalf("tStart = %q, want %q (no changes ⇒ tStart == baseTree)", tStart, baseTree)
	}
}

// TestFreezeWorkingTree_NotARepo verifies that FreezeWorkingTree returns a non-nil error
// on a non-repo directory.
func TestFreezeWorkingTree_NotARepo(t *testing.T) {
	g := New(t.TempDir()) // plain dir, NOT a git repo
	_, err := g.FreezeWorkingTree(context.Background(), EmptyTreeSHA)
	if err == nil {
		t.Fatal("FreezeWorkingTree err = nil, want non-nil (non-repo)")
	}
	if !strings.Contains(err.Error(), "git add -A: failed") {
		t.Fatalf("FreezeWorkingTree err = %v, want it to contain 'git add -A: failed'", err)
	}
}

// TestFreezeWorkingTree_GitBinaryMissing verifies that a missing git binary surfaces
// as a non-nil error containing "git binary not found".
func TestFreezeWorkingTree_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	_, err := g.FreezeWorkingTree(context.Background(), EmptyTreeSHA)
	if err == nil {
		t.Fatal("FreezeWorkingTree err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("FreezeWorkingTree err = %v, want it to contain 'git binary not found'", err)
	}
}

// TestFreezeWorkingTree_ContextCancelled verifies that a pre-cancelled context
// surfaces as context.Canceled.
func TestFreezeWorkingTree_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := New(t.TempDir())
	_, err := g.FreezeWorkingTree(ctx, EmptyTreeSHA)
	if err == nil {
		t.Fatal("FreezeWorkingTree err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FreezeWorkingTree err = %v, want errors.Is(err, context.Canceled)", err)
	}
}
