package git

// White-box tests for the two staging primitives in stage.go
// (HasStagedChanges / AddAll). They are package git (NOT git_test) so they can
// call the unexported (g *Git).run seam and compose the S2 harness helpers
// (newTempRepo/seedCommits/writeFileStage/mustRun in gittestutil_test.go)
// which live as package-git _test.go files in this SAME directory. They drive
// the REAL host git binary (git 2.54.0, PRD §20.1 layer 2) — no mocks of git,
// no go-git — with one behavior per Test* function, mirroring
// plumbing_test.go/diff_test.go/log_test.go's posture.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHasStagedChanges_FalseOnCleanIndex proves the (false, nil) clean
// contract: with HEAD present and the index matching HEAD, `git diff --cached
// --quiet` exits 0 and HasStagedChanges returns (false, nil) (the CORRECTED
// semantics — exit 0 = clean, NOT staged).
func TestHasStagedChanges_FalseOnCleanIndex(t *testing.T) {
	g := newTempRepo(t)
	seedCommits(t, g, []string{"init"}) // HEAD exists, index clean

	got, err := g.HasStagedChanges()
	if err != nil {
		t.Fatalf("HasStagedChanges on clean index returned error %v; want nil", err)
	}
	if got {
		t.Error("HasStagedChanges = true on clean index; want false (exit 0 = clean)")
	}
}

// TestHasStagedChanges_TrueAfterStagingAFile proves exit 1 → (true, nil) on an
// UNBORN repo: writeFileStage stages a file, `git diff --cached --quiet` exits
// 1 (differences present), and HasStagedChanges returns (true, nil). This also
// proves the unborn-repo case needs NO special handling (unlike
// CommitCount/RecentMessages, which detect the exit-128 rootless signals).
func TestHasStagedChanges_TrueAfterStagingAFile(t *testing.T) {
	g := newTempRepo(t) // unborn is fine — no special-case needed
	writeFileStage(t, g, "a.txt", "a\n")

	got, err := g.HasStagedChanges()
	if err != nil {
		t.Fatalf("HasStagedChanges after staging returned error %v; want nil", err)
	}
	if !got {
		t.Error("HasStagedChanges = false after staging a file; want true (exit 1 = staged)")
	}
}

// TestHasStagedChanges_TrueAfterModifyingTrackedAndStaging proves exit 1 →
// (true, nil) for the modified-tracked-file case: seedCommits creates a tracked
// file0.txt, the test modifies it on disk and re-stages it, and
// HasStagedChanges returns (true, nil) because the staged content now differs
// from HEAD.
func TestHasStagedChanges_TrueAfterModifyingTrackedAndStaging(t *testing.T) {
	g := newTempRepo(t)
	seedCommits(t, g, []string{"init"}) // creates tracked file0.txt

	// Modify the tracked file on disk and re-stage it (g.dir is readable
	// white-box; cmd.Dir is g.dir so "file0.txt" is the correct relative path).
	if err := os.WriteFile(filepath.Join(g.dir, "file0.txt"), []byte("modified\n"), 0o644); err != nil {
		t.Fatalf("write file0.txt: %v", err)
	}
	mustRun(t, g, "add", "file0.txt")

	got, err := g.HasStagedChanges()
	if err != nil {
		t.Fatalf("HasStagedChanges after modifying+staging returned error %v; want nil", err)
	}
	if !got {
		t.Error("HasStagedChanges = false after modifying+staging a tracked file; want true (exit 1 = staged)")
	}
}

// TestHasStagedChanges_NonRepoIsError proves the (false, typed *ExitError)
// failure contract on a non-repo: with NO git init, `git diff --cached` fails
// with a non-zero exit that is NOT the clean (0) / staged (1) signal, so
// HasStagedChanges returns (false, err) where err is a typed *ExitError. The
// exact exit code is NOT pinned (it varies by git version/context outside a
// repo); the contract intent is "anything that isn't the clean/staged signal
// → error". This mirrors git_test.go's TestRun_NonRepoIsExitError posture.
func TestHasStagedChanges_NonRepoIsError(t *testing.T) {
	g, err := New(t.TempDir()) // NO git init → not a repository
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}

	got, err := g.HasStagedChanges()
	if err == nil {
		t.Fatal("HasStagedChanges in a non-repo returned nil error; want a typed *ExitError")
	}
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("HasStagedChanges error is %T; want *ExitError (errors.As)", err)
	}
	if got {
		t.Error("HasStagedChanges = true in a non-repo; want false (non-zero non-1 exit)")
	}
}

// TestAddAll_StagesNewModifiedDeleted proves AddAll stages ALL three classes
// of worktree change — new (A), modified (M), and deleted (D) — asserted via
// `git diff --cached --name-status`. It also proves the index is clean BEFORE
// AddAll (the on-disk-only changes are not yet staged) and that
// HasStagedChanges flips to (true, nil) AFTER AddAll.
func TestAddAll_StagesNewModifiedDeleted(t *testing.T) {
	g := newTempRepo(t)
	seedCommits(t, g, []string{"init"}) // tracked file0.txt
	// Add a SECOND tracked file so the delete case has something to remove.
	writeFileStage(t, g, "gone.txt", "g\n")
	mustRun(t, g, "commit", "-q", "-m", "two") // now gone.txt is tracked

	// On-disk-only changes (NOT yet staged): MODIFY file0.txt, ADD new.txt,
	// DELETE gone.txt.
	if err := os.WriteFile(filepath.Join(g.dir, "file0.txt"), []byte("modified\n"), 0o644); err != nil {
		t.Fatalf("write file0.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(g.dir, "new.txt"), []byte("n\n"), 0o644); err != nil {
		t.Fatalf("write new.txt: %v", err)
	}
	if err := os.Remove(filepath.Join(g.dir, "gone.txt")); err != nil {
		t.Fatalf("remove gone.txt: %v", err)
	}

	// BEFORE AddAll the index is still clean (worktree dirty, index clean):
	// exit 0 → (false, nil).
	if got, err := g.HasStagedChanges(); err != nil {
		t.Fatalf("HasStagedChanges before AddAll returned error %v; want nil", err)
	} else if got {
		t.Fatal("HasStagedChanges = true before AddAll; want false (index still clean)")
	}

	// Run AddAll — must succeed.
	if err := g.AddAll(); err != nil {
		t.Fatalf("AddAll returned error %v; want nil", err)
	}

	// Assert all three classes are staged (A=new, M=modified, D=deleted) via
	// the staged name-status output.
	out, err := g.run("diff", "--cached", "--name-status")
	if err != nil {
		t.Fatalf("git diff --cached --name-status returned error %v; want nil", err)
	}
	if !strings.Contains(out, "A\tnew.txt") {
		t.Errorf("staged set missing NEW file (A\\tnew.txt):\n%s", out)
	}
	if !strings.Contains(out, "M\tfile0.txt") {
		t.Errorf("staged set missing MODIFIED file (M\\tfile0.txt):\n%s", out)
	}
	if !strings.Contains(out, "D\tgone.txt") {
		t.Errorf("staged set missing DELETED file (D\\tgone.txt):\n%s", out)
	}

	// AFTER AddAll the index holds staged changes → (true, nil).
	if got, err := g.HasStagedChanges(); err != nil {
		t.Fatalf("HasStagedChanges after AddAll returned error %v; want nil", err)
	} else if !got {
		t.Error("HasStagedChanges = false after AddAll staged changes; want true (exit 1 = staged)")
	}
}

// TestAddAll_CleanRepoReturnsNoError proves `git add -A` exits 0 (returns nil)
// even when there is nothing to stage — "nothing to add" is NOT an error — and
// leaves HasStagedChanges at (false, nil).
func TestAddAll_CleanRepoReturnsNoError(t *testing.T) {
	g := newTempRepo(t)

	if err := g.AddAll(); err != nil {
		t.Fatalf("AddAll on clean repo returned error %v; want nil (nothing to stage is not an error)", err)
	}

	if got, err := g.HasStagedChanges(); err != nil {
		t.Fatalf("HasStagedChanges after AddAll on clean repo returned error %v; want nil", err)
	} else if got {
		t.Error("HasStagedChanges = true after AddAll on clean repo; want false (still clean)")
	}
}
