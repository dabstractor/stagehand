package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddAll_StagesModifiedAndUntracked(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package main\n") // tracked
	stageFile(t, repo, "a.go")
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "a.go", "package main\nvar x = 1\n") // modified
	writeFile(t, repo, "b.go", "package main\n")            // untracked

	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	// Independent verification (NOT via StagedFileCount — decouples the assertion):
	out, err := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").Output()
	if err != nil {
		t.Fatalf("verify diff: %v", err)
	}
	got := strings.Fields(string(out)) // splits on whitespace; each path is one token here
	want := map[string]bool{"a.go": true, "b.go": true}
	for _, p := range got {
		delete(want, p)
	}
	if len(want) != 0 {
		t.Fatalf("AddAll did not stage expected files; missing %v, got %v", want, got)
	}
}

func TestAddAll_StagesDeletion(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	makeEmptyCommit(t, repo, "init")
	if err := os.Remove(filepath.Join(repo, "a.go")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	count, err := g.StagedFileCount(context.Background()) // integration: deletion counts as 1 staged
	if err != nil {
		t.Fatalf("StagedFileCount err = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (deletion should be staged)", count)
	}
}

func TestAddAll_CleanTreeNoOp(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")
	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("StagedFileCount err = %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 (clean tree)", count)
	}
}

func TestAddAll_UnbornRepoStagesFiles(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "f.go", "package main\n")
	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("StagedFileCount err = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (unborn repo staged file)", count)
	}
}

func TestAddAll_NotARepo(t *testing.T) {
	g := New(t.TempDir()) // plain dir, NOT a git repo
	err := g.AddAll(context.Background())
	if err == nil {
		t.Fatal("AddAll err = nil, want non-nil (non-repo → exit 128)")
	}
	if !strings.Contains(err.Error(), "git add -A: failed") {
		t.Fatalf("err = %v, want it to contain 'git add -A: failed'", err)
	}
}

func TestAddAll_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail
	g := New(t.TempDir())
	err := g.AddAll(context.Background())
	if err == nil {
		t.Fatal("AddAll err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("err = %v, want it to contain 'git binary not found'", err)
	}
}

func TestAddAll_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism
	g := New(t.TempDir())
	err := g.AddAll(ctx)
	if err == nil {
		t.Fatal("AddAll err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want errors.Is(err, context.Canceled)", err)
	}
}
