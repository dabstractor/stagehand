package hooks

// reconcile_test.go exercises ReconcileIndex end-to-end against REAL git. It models the F1 scenario:
// a permitted pre-commit hook mutates a snapshot path (f.txt) from "user-staged" to "formatted-by-hook",
// so the committed tree (finalTree) differs from the frozen snapshot (snapshotTree). ReconcileIndex
// must sync the live index's f.txt entry to finalTree, while leaving any non-mutated staged path
// (the "stage while generating" case) untouched. It also covers the no-mutation no-op (snapshotTree
// == finalTree) and the DryRun no-op.

import (
	"context"
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/git"
)

// indexBlob returns the blob content for path in the LIVE index (oracle). "" if path is absent.
func indexBlob(t *testing.T, dir, path string) string {
	t.Helper()
	out := execGit(t, dir, "ls-files", "-s", "--", path)
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return "" // not in the index
	}
	return execGit(t, dir, "cat-file", "blob", fields[1])
}

// TestReconcileIndex_SyncsMutatedPathPreservesOthers models the full F1 fix:
//   - snapshot: f.txt="user-staged", g.txt="user-g" (both staged at freeze)
//   - hook mutates f.txt → "formatted-by-hook" (committed tree = finalTree)
//   - live index still holds f.txt="user-staged" (the divergence F1 describes)
//   - ReconcileIndex(snapshotTree, finalTree) must sync f.txt → "formatted-by-hook" AND preserve g.txt.
func TestReconcileIndex_SyncsMutatedPathPreservesOthers(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init") // so HEAD exists

	// snapshot tree: f.txt="user-staged", g.txt="user-g" (the frozen index at freeze time).
	writeFile(t, repo, "f.txt", "user-staged\n")
	writeFile(t, repo, "g.txt", "user-g\n")
	stageFile(t, repo, "f.txt")
	stageFile(t, repo, "g.txt")
	snapshotTree := writeTreeOf(t, repo)

	// finalTree (post-hook): f.txt="formatted-by-hook" (hook reformatted it); g.txt unchanged.
	writeFile(t, repo, "f.txt", "formatted-by-hook\n")
	stageFile(t, repo, "f.txt")
	finalTree := writeTreeOf(t, repo)

	// Reset the LIVE index to the PRE-hook snapshot state (f.txt="user-staged", g.txt="user-g") to
	// model the divergence: HEAD will advance to finalTree, but the live index holds the pre-hook blob.
	execGit(t, repo, "read-tree", snapshotTree)

	g := git.New(repo)
	if err := ReconcileIndex(context.Background(), g, snapshotTree, finalTree, HookOpts{}); err != nil {
		t.Fatalf("ReconcileIndex err = %v, want nil", err)
	}

	// f.txt (mutated snapshot path) must be synced to finalTree's blob.
	if got := indexBlob(t, repo, "f.txt"); got != "formatted-by-hook" {
		t.Errorf("f.txt index blob = %q, want %q (synced to committed tree)", got, "formatted-by-hook")
	}
	// g.txt (NOT mutated by the hook) must be PRESERVED at its staged value.
	if got := indexBlob(t, repo, "g.txt"); got != "user-g" {
		t.Errorf("g.txt index blob = %q, want %q (preserved — not in the mutation set)", got, "user-g")
	}
}

// TestReconcileIndex_NoMutationIsNoOp verifies that when finalTree == snapshotTree (no hook mutation),
// ReconcileIndex is a no-op: the live index is byte-for-byte unchanged.
func TestReconcileIndex_NoMutationIsNoOp(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")

	writeFile(t, repo, "f.txt", "content\n")
	stageFile(t, repo, "f.txt")
	tree := writeTreeOf(t, repo) // snapshot == final (no mutation)

	before := execGit(t, repo, "ls-files", "-s")
	g := git.New(repo)
	if err := ReconcileIndex(context.Background(), g, tree, tree, HookOpts{}); err != nil {
		t.Fatalf("ReconcileIndex(no-mutation) err = %v, want nil", err)
	}
	if after := execGit(t, repo, "ls-files", "-s"); after != before {
		t.Errorf("ReconcileIndex(no-mutation) mutated the index:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestReconcileIndex_DeletionSyncsRemoval verifies that a path the hook DELETED (present in snapshot,
// absent in finalTree) is removed from the live index, while preserving others.
func TestReconcileIndex_DeletionSyncsRemoval(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")

	// snapshot: f.txt + g.txt staged.
	writeFile(t, repo, "f.txt", "f\n")
	writeFile(t, repo, "g.txt", "g\n")
	stageFile(t, repo, "f.txt")
	stageFile(t, repo, "g.txt")
	snapshotTree := writeTreeOf(t, repo)

	// finalTree: only g.txt (the hook deleted f.txt).
	execGit(t, repo, "rm", "--cached", "f.txt")
	finalTree := writeTreeOf(t, repo)

	// Reset live index to the snapshot (f.txt + g.txt staged) to model the divergence.
	execGit(t, repo, "read-tree", snapshotTree)

	g := git.New(repo)
	if err := ReconcileIndex(context.Background(), g, snapshotTree, finalTree, HookOpts{}); err != nil {
		t.Fatalf("ReconcileIndex err = %v, want nil", err)
	}
	if got := indexBlob(t, repo, "f.txt"); got != "" {
		t.Errorf("f.txt index entry = %q, want absent (removed by reconcile)", got)
	}
	if got := indexBlob(t, repo, "g.txt"); got != "g" {
		t.Errorf("g.txt index blob = %q, want %q (preserved)", got, "g")
	}
}
