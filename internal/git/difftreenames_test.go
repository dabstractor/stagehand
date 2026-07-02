package git

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
)

// TestDiffTreeNames_ListsChangedPaths verifies that DiffTreeNames returns the sorted
// changed-path set (modifications, additions, AND deletions) between two trees.
func TestDiffTreeNames_ListsChangedPaths(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "a\n")
	writeFile(t, repo, "b.txt", "b\n")
	stageFile(t, repo, "a.txt")
	stageFile(t, repo, "b.txt")
	makeEmptyCommit(t, repo, "base")
	treeA := execGit(t, repo, "rev-parse", "HEAD^{tree}")

	// Modify a.txt, add c.txt, delete b.txt, then capture treeB.
	writeFile(t, repo, "a.txt", "a-mod\n")
	writeFile(t, repo, "c.txt", "c\n")
	stageFile(t, repo, "c.txt")
	stageFile(t, repo, "a.txt")
	rmCmd := execGit(t, repo, "rm", "-q", "b.txt") // remove from index
	_ = rmCmd
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	got, err := g.DiffTreeNames(context.Background(), treeA, treeB)
	if err != nil {
		t.Fatalf("DiffTreeNames err = %v, want nil", err)
	}
	want := []string{"a.txt", "b.txt", "c.txt"}
	if len(got) != len(want) {
		t.Fatalf("DiffTreeNames = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("DiffTreeNames[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDiffTreeNames_SortedAndDeduped verifies that DiffTreeNames returns sorted output
// with no duplicates, regardless of git's internal tree-walk order.
func TestDiffTreeNames_SortedAndDeduped(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// Create files in non-sorted order to ensure the impl sorts.
	writeFile(t, repo, "z.txt", "z\n")
	writeFile(t, repo, "m.txt", "m\n")
	writeFile(t, repo, "a.txt", "a\n")
	stageFile(t, repo, "z.txt")
	stageFile(t, repo, "m.txt")
	stageFile(t, repo, "a.txt")
	treeA := writeTreeOf(t, repo)

	// Add more files to ensure non-trivial diff.
	writeFile(t, repo, "b.txt", "b\n")
	writeFile(t, repo, "y.txt", "y\n")
	stageFile(t, repo, "b.txt")
	stageFile(t, repo, "y.txt")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	got, err := g.DiffTreeNames(context.Background(), treeA, treeB)
	if err != nil {
		t.Fatalf("DiffTreeNames err = %v, want nil", err)
	}

	// Verify sorted.
	if !sort.SliceIsSorted(got, func(i, j int) bool { return got[i] < got[j] }) {
		t.Fatalf("DiffTreeNames = %v, want sorted", got)
	}

	// Verify no duplicates.
	seen := make(map[string]bool)
	for _, p := range got {
		if seen[p] {
			t.Fatalf("DiffTreeNames has duplicate path %q", p)
		}
		seen[p] = true
	}

	// Verify expected paths.
	want := []string{"b.txt", "y.txt"}
	if len(got) != len(want) {
		t.Fatalf("DiffTreeNames = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("DiffTreeNames[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDiffTreeNames_IdenticalTreesNil verifies that identical trees return nil (len 0).
func TestDiffTreeNames_IdenticalTreesNil(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "a\n")
	stageFile(t, repo, "a.txt")
	treeA := writeTreeOf(t, repo)

	g := New(repo)
	got, err := g.DiffTreeNames(context.Background(), treeA, treeA)
	if err != nil {
		t.Fatalf("DiffTreeNames err = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("DiffTreeNames identical trees = %v, want nil/empty", got)
	}
}

// TestDiffTreeNames_EmptyTreeBase verifies that EmptyTreeSHA as treeA lists all of treeB.
func TestDiffTreeNames_EmptyTreeBase(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "a\n")
	writeFile(t, repo, "b.txt", "b\n")
	stageFile(t, repo, "a.txt")
	stageFile(t, repo, "b.txt")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	got, err := g.DiffTreeNames(context.Background(), EmptyTreeSHA, treeB)
	if err != nil {
		t.Fatalf("DiffTreeNames err = %v, want nil", err)
	}
	want := []string{"a.txt", "b.txt"}
	if len(got) != len(want) {
		t.Fatalf("DiffTreeNames EmptyTreeSHA = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("DiffTreeNames[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDiffTreeNames_DeletionsAdditionsModifications verifies that all three change types
// appear in the output.
func TestDiffTreeNames_DeletionsAdditionsModifications(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "keep.txt", "keep\n")
	writeFile(t, repo, "modify.txt", "old\n")
	writeFile(t, repo, "delete.txt", "del\n")
	stageFile(t, repo, "keep.txt")
	stageFile(t, repo, "modify.txt")
	stageFile(t, repo, "delete.txt")
	makeEmptyCommit(t, repo, "base")
	treeA := execGit(t, repo, "rev-parse", "HEAD^{tree}")

	// Modify modify.txt, delete delete.txt, add add.txt.
	writeFile(t, repo, "modify.txt", "new\n")
	writeFile(t, repo, "add.txt", "new\n")
	stageFile(t, repo, "modify.txt")
	stageFile(t, repo, "add.txt")
	execGit(t, repo, "rm", "-q", "delete.txt")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	got, err := g.DiffTreeNames(context.Background(), treeA, treeB)
	if err != nil {
		t.Fatalf("DiffTreeNames err = %v, want nil", err)
	}
	want := []string{"add.txt", "delete.txt", "modify.txt"}
	if len(got) != len(want) {
		t.Fatalf("DiffTreeNames = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("DiffTreeNames[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDiffTreeNames_BadTreeSHA verifies that DiffTreeNames returns a non-nil error
// for an invalid tree SHA.
func TestDiffTreeNames_BadTreeSHA(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "a\n")
	stageFile(t, repo, "a.txt")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	_, err := g.DiffTreeNames(context.Background(), "0000000000000000000000000000000000000000", treeB)
	if err == nil {
		t.Fatal("DiffTreeNames err = nil, want non-nil (bad tree SHA)")
	}
	if !strings.Contains(err.Error(), "git diff-tree: failed") {
		t.Fatalf("DiffTreeNames err = %v, want it to contain 'git diff-tree: failed'", err)
	}
}

// TestDiffTreeNames_NotARepo verifies that DiffTreeNames returns a non-nil error
// on a non-repo directory.
func TestDiffTreeNames_NotARepo(t *testing.T) {
	g := New(t.TempDir()) // plain dir, NOT a git repo
	_, err := g.DiffTreeNames(context.Background(), EmptyTreeSHA, EmptyTreeSHA)
	if err == nil {
		t.Fatal("DiffTreeNames err = nil, want non-nil (non-repo)")
	}
}

// TestDiffTreeNames_GitBinaryMissing verifies that a missing git binary surfaces
// as a non-nil error containing "git binary not found".
func TestDiffTreeNames_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	_, err := g.DiffTreeNames(context.Background(), EmptyTreeSHA, EmptyTreeSHA)
	if err == nil {
		t.Fatal("DiffTreeNames err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("DiffTreeNames err = %v, want it to contain 'git binary not found'", err)
	}
}

// TestDiffTreeNames_ContextCancelled verifies that a pre-cancelled context
// surfaces as context.Canceled.
func TestDiffTreeNames_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := New(t.TempDir())
	_, err := g.DiffTreeNames(ctx, EmptyTreeSHA, EmptyTreeSHA)
	if err == nil {
		t.Fatal("DiffTreeNames err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DiffTreeNames err = %v, want errors.Is(err, context.Canceled)", err)
	}
}
