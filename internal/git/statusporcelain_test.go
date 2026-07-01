package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStatusPorcelain_CleanRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init") // HEAD exists; nothing changed
	g := New(repo)
	out, err := g.StatusPorcelain(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out != "" {
		t.Fatalf("out = %q, want \"\" (clean tree)", out)
	}
}

func TestStatusPorcelain_CleanUnbornRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo) // NO commits, NO files — unborn repo
	g := New(repo)
	out, err := g.StatusPorcelain(context.Background())
	if err != nil {
		t.Fatalf("err = %v, want nil (unborn repo with no files → exit 0, empty porcelain)", err)
	}
	if out != "" {
		t.Fatalf("out = %q, want \"\" (clean unborn repo)", out)
	}
}

func TestStatusPorcelain_UnstagedChanges(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "x\n")
	stageFile(t, repo, "a.txt")
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "a.txt", "modified\n") // tracked file modified, NOT staged
	writeFile(t, repo, "b.txt", "new\n")      // untracked file
	g := New(repo)
	out, err := g.StatusPorcelain(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out == "" {
		t.Fatal("out is empty, want non-empty (dirty tree)")
	}
	if !strings.Contains(out, "a.txt") {
		t.Fatalf("out = %q, want it to contain \"a.txt\"", out)
	}
	if !strings.Contains(out, "b.txt") {
		t.Fatalf("out = %q, want it to contain \"b.txt\"", out)
	}
}

func TestStatusPorcelain_StagedChanges(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "c.txt", "staged\n")
	stageFile(t, repo, "c.txt") // staged new file
	g := New(repo)
	out, err := g.StatusPorcelain(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out == "" {
		t.Fatal("out is empty, want non-empty (staged changes)")
	}
	if !strings.Contains(out, "c.txt") {
		t.Fatalf("out = %q, want it to contain \"c.txt\"", out)
	}
}

func TestStatusPorcelain_RawPorcelainFormatPreserved(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "untracked.txt", "u\n") // → "?? untracked.txt"
	writeFile(t, repo, "added.txt", "a\n")
	stageFile(t, repo, "added.txt") // → "A  added.txt"
	g := New(repo)
	out, err := g.StatusPorcelain(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out, "?? untracked.txt") {
		t.Fatalf("out = %q, want it to contain \"?? untracked.txt\" (2-char XY code + space preserved)", out)
	}
	if !strings.Contains(out, "A  added.txt") {
		t.Fatalf("out = %q, want it to contain \"A  added.txt\" (staged-add code preserved)", out)
	}
}

func TestStatusPorcelain_NotARepo(t *testing.T) {
	g := New(t.TempDir()) // plain dir, NOT a git repo (no initRepo)
	out, err := g.StatusPorcelain(context.Background())
	if err == nil {
		t.Fatal("err = nil, want non-nil (non-repo → exit 128)")
	}
	if !strings.Contains(err.Error(), "git status --porcelain: failed") {
		t.Fatalf("err = %v, want it to contain 'git status --porcelain: failed'", err)
	}
	if out != "" {
		t.Fatalf("out = %q, want \"\" on error", out)
	}
}

func TestStatusPorcelain_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "")
	g := New(t.TempDir())
	out, err := g.StatusPorcelain(context.Background())
	if err == nil {
		t.Fatal("err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("err = %v, want it to contain 'git binary not found'", err)
	}
	if out != "" {
		t.Fatalf("out = %q, want \"\" on error", out)
	}
}

func TestStatusPorcelain_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism
	g := New(t.TempDir())
	out, err := g.StatusPorcelain(ctx)
	if err == nil {
		t.Fatal("err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want errors.Is(err, context.Canceled)", err)
	}
	if out != "" {
		t.Fatalf("out = %q, want \"\" on error", out)
	}
}
