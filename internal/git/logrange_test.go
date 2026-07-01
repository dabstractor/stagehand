package git

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// shaOf runs `git rev-parse <rev>` in the repo and returns the trimmed SHA (for test assertions).
func shaOf(t *testing.T, repo, rev string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "rev-parse", rev)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s failed: %v\n%s", rev, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestLogRange_ThreeCommits_ReturnsRange(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "commit A")
	aSHA := shaOf(t, repo, "HEAD") // root commit
	makeEmptyCommit(t, repo, "commit B")
	bSHA := shaOf(t, repo, "HEAD")
	makeEmptyCommit(t, repo, "commit C")
	cSHA := shaOf(t, repo, "HEAD")

	g := New(repo)
	entries, err := g.LogRange(context.Background(), aSHA)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2 (B and C, oldest-first)", len(entries))
	}
	// Oldest-first: B then C
	if entries[0].SHA != bSHA {
		t.Fatalf("entries[0].SHA = %q, want %q (commit B)", entries[0].SHA, bSHA)
	}
	if entries[0].Subject != "commit B" {
		t.Fatalf("entries[0].Subject = %q, want %q", entries[0].Subject, "commit B")
	}
	if entries[1].SHA != cSHA {
		t.Fatalf("entries[1].SHA = %q, want %q (commit C)", entries[1].SHA, cSHA)
	}
	if entries[1].Subject != "commit C" {
		t.Fatalf("entries[1].Subject = %q, want %q", entries[1].Subject, "commit C")
	}
}

func TestLogRange_EmptyRange_ReturnsNil(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "only commit")
	headSHA := shaOf(t, repo, "HEAD")

	g := New(repo)
	entries, err := g.LogRange(context.Background(), headSHA)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if entries != nil {
		t.Fatalf("entries = %v, want nil (empty range HEAD..HEAD)", entries)
	}
}

func TestLogRange_AllZerosBase_ReturnsAllCommits(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "commit A")
	makeEmptyCommit(t, repo, "commit B")
	makeEmptyCommit(t, repo, "commit C")
	// Resolve SHAs AFTER all commits exist (HEAD~N requires N ancestors).
	aSHA := shaOf(t, repo, "HEAD~2")
	bSHA := shaOf(t, repo, "HEAD~1")
	cSHA := shaOf(t, repo, "HEAD")

	g := New(repo)
	entries, err := g.LogRange(context.Background(), strings.Repeat("0", 40))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3 (A, B, C — all commits)", len(entries))
	}
	// Oldest-first: A, B, C
	if entries[0].SHA != aSHA {
		t.Fatalf("entries[0].SHA = %q, want %q (commit A)", entries[0].SHA, aSHA)
	}
	if entries[0].Subject != "commit A" {
		t.Fatalf("entries[0].Subject = %q, want %q", entries[0].Subject, "commit A")
	}
	if entries[1].SHA != bSHA {
		t.Fatalf("entries[1].SHA = %q, want %q (commit B)", entries[1].SHA, bSHA)
	}
	if entries[1].Subject != "commit B" {
		t.Fatalf("entries[1].Subject = %q, want %q", entries[1].Subject, "commit B")
	}
	if entries[2].SHA != cSHA {
		t.Fatalf("entries[2].SHA = %q, want %q (commit C)", entries[2].SHA, cSHA)
	}
	if entries[2].Subject != "commit C" {
		t.Fatalf("entries[2].Subject = %q, want %q", entries[2].Subject, "commit C")
	}
}

func TestLogRange_UnbornRepo_ReturnsNil(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo) // zero commits — truly-unborn repo

	g := New(repo)
	entries, err := g.LogRange(context.Background(), strings.Repeat("0", 40))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if entries != nil {
		t.Fatalf("entries = %v, want nil (truly-unborn repo)", entries)
	}
}

func TestLogRange_SpecialCharsInSubject(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "initial")
	initSHA := shaOf(t, repo, "HEAD")
	makeEmptyCommit(t, repo, "fix: handle --- edge case & \"quotes\"")

	g := New(repo)
	entries, err := g.LogRange(context.Background(), initSHA)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if !strings.Contains(entries[0].Subject, "---") {
		t.Fatalf("subject lost '---': %q", entries[0].Subject)
	}
	if !strings.Contains(entries[0].Subject, "&") || !strings.Contains(entries[0].Subject, "\"quotes\"") {
		t.Fatalf("subject lost special chars: %q", entries[0].Subject)
	}
}
