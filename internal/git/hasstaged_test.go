package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestHasStagedChanges_NothingStaged(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	g := New(repo)
	staged, err := g.HasStagedChanges(context.Background())
	if err != nil {
		t.Fatalf("HasStagedChanges() err = %v, want nil", err)
	}
	if staged {
		t.Fatal("HasStagedChanges() = true, want false (nothing staged → exit 0)")
	}
}

func TestHasStagedChanges_StagedFile(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")

	g := New(repo)
	staged, err := g.HasStagedChanges(context.Background())
	if err != nil {
		t.Fatalf("HasStagedChanges() err = %v, want nil", err)
	}
	if !staged {
		t.Fatal("HasStagedChanges() = false, want true (staged file → exit 1 — FINDING 6 inversion)")
	}
}

func TestHasStagedChanges_CommittedNothingStaged(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "initial") // HEAD exists, nothing NEW staged

	g := New(repo)
	staged, err := g.HasStagedChanges(context.Background())
	if err != nil {
		t.Fatalf("HasStagedChanges() err = %v, want nil", err)
	}
	if staged {
		t.Fatal("HasStagedChanges() = true, want false (committed, nothing new staged → exit 0, index == HEAD)")
	}
}

func TestHasStagedChanges_NotARepo(t *testing.T) {
	g := New(t.TempDir()) // plain dir, NOT a git repo (no initRepo)

	staged, err := g.HasStagedChanges(context.Background())
	if err == nil {
		t.Fatal("HasStagedChanges() err = nil, want non-nil (non-repo → exit >1)")
	}
	if !strings.Contains(err.Error(), "git diff --cached --quiet: failed") {
		t.Fatalf("HasStagedChanges() err = %v, want it to contain 'git diff --cached --quiet: failed'", err)
	}
	if staged {
		t.Fatal("HasStagedChanges() = true, want false (exit >1 is an error, NOT a staged signal)")
	}
}

func TestHasStagedChanges_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	staged, err := g.HasStagedChanges(context.Background())
	if err == nil {
		t.Fatal("HasStagedChanges() err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("HasStagedChanges() err = %v, want it to contain 'git binary not found'", err)
	}
	if staged {
		t.Fatal("HasStagedChanges() = true, want false (LookPath miss NOT misread as staged)")
	}
}

func TestHasStagedChanges_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism (G12)

	g := New(t.TempDir())
	staged, err := g.HasStagedChanges(ctx)
	if err == nil {
		t.Fatal("HasStagedChanges() err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("HasStagedChanges() err = %v, want errors.Is(err, context.Canceled)", err)
	}
	if staged {
		t.Fatal("HasStagedChanges() = true, want false (context cancelled)")
	}
}
