package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStagedFileCount_NothingStaged(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init") // HEAD exists; nothing NEW staged
	g := New(repo)
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 (nothing staged → empty output)", count)
	}
}

func TestStagedFileCount_ThreeFiles(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "a.go", "1\n")
	stageFile(t, repo, "a.go")
	writeFile(t, repo, "b.go", "2\n")
	stageFile(t, repo, "b.go")
	writeFile(t, repo, "c.go", "3\n")
	stageFile(t, repo, "c.go")
	g := New(repo)
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}
}

func TestStagedFileCount_AfterAddAll(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "a.go", "modified\n")
	writeFile(t, repo, "b.go", "untracked\n")

	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2 (one modified + one untracked after AddAll)", count)
	}
}

func TestStagedFileCount_IncludesDeletion(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "initial\n")
	stageFile(t, repo, "a.go")
	makeEmptyCommit(t, repo, "init")
	if err := os.Remove(filepath.Join(repo, "a.go")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (deletion should be staged)", count)
	}
}

func TestStagedFileCount_FilenameWithSpace(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, repo, "sub/has space.txt", "x\n") // create file with a SPACE in the name
	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (a space in the name must NOT split into 2 lines under --name-only)",
			count)
	}
}

func TestStagedFileCount_UnbornRepoWithStaged(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "f.go", "package main\n")
	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (unborn repo staged file)", count)
	}
}

func TestStagedFileCount_NotARepo(t *testing.T) {
	g := New(t.TempDir()) // plain dir, NOT a git repo (no initRepo) → exit 129
	count, err := g.StagedFileCount(context.Background())
	if err == nil {
		t.Fatal("err = nil, want non-nil (non-repo → exit 129)")
	}
	if !strings.Contains(err.Error(), "git diff --cached --name-only: failed") {
		t.Fatalf("err = %v, want it to contain 'git diff --cached --name-only: failed'", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 on error", count)
	}
}

func TestStagedFileCount_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "")
	g := New(t.TempDir())
	count, err := g.StagedFileCount(context.Background())
	if err == nil {
		t.Fatal("err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("err = %v, want it to contain 'git binary not found'", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 on error", count)
	}
}

func TestStagedFileCount_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g := New(t.TempDir())
	count, err := g.StagedFileCount(ctx)
	if err == nil {
		t.Fatal("err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want errors.Is(err, context.Canceled)", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 on error", count)
	}
}
