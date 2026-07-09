package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// minGitEnv returns a minimal environment with PATH and HOME so git commands
// can find the binary and the user's home directory without leaking config.
func minGitEnv() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
}

// makeEmptyCommit creates an empty commit in the repo at dir with the given message.
// It sets author/committer identity via environment variables (gotcha G9).
func makeEmptyCommit(t *testing.T, dir, msg string) {
	t.Helper()
	env := append(minGitEnv(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	cmd := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", msg)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("makeEmptyCommit failed: %v\n%s", err, out)
	}
}

// TestRevParseHEAD_UnbornRepo verifies that RevParseHEAD returns isUnborn=true
// on a zero-commit repo, detected via git's exit code 128 (NOT stdout emptiness).
func TestRevParseHEAD_UnbornRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo) // zero commits — unborn repo (reuses S1's helper, gotcha G8)

	g := New(repo)
	sha, isUnborn, err := g.RevParseHEAD(context.Background())

	if err != nil {
		t.Fatalf("RevParseHEAD err = %v, want nil", err)
	}
	if !isUnborn {
		t.Fatalf("RevParseHEAD isUnborn = false, want true (unborn repo)")
	}
	if sha != "" {
		t.Fatalf("RevParseHEAD sha = %q, want empty string", sha)
	}
}

// TestRevParseHEAD_BornRepo verifies that RevParseHEAD returns a trimmed SHA on
// a repo with at least one commit.
func TestRevParseHEAD_BornRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "initial")

	g := New(repo)
	sha, isUnborn, err := g.RevParseHEAD(context.Background())

	if err != nil {
		t.Fatalf("RevParseHEAD err = %v, want nil", err)
	}
	if isUnborn {
		t.Fatalf("RevParseHEAD isUnborn = true, want false (repo has commits)")
	}
	if !regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(sha) {
		t.Fatalf("RevParseHEAD sha = %q, want 40 or 64 hex chars", sha)
	}
}

// TestRevParseHEAD_GitBinaryMissing verifies that a missing git binary surfaces
// as a non-nil error (NOT isUnborn=true).
func TestRevParseHEAD_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir()) // dir need not be a repo; LookPath fails first
	sha, isUnborn, err := g.RevParseHEAD(context.Background())

	if err == nil {
		t.Fatal("RevParseHEAD err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("RevParseHEAD err = %v, want it to contain 'git binary not found'", err)
	}
	if isUnborn {
		t.Fatalf("RevParseHEAD isUnborn = true, want false (LookPath miss is NOT unborn)")
	}
	if sha != "" {
		t.Fatalf("RevParseHEAD sha = %q, want empty string", sha)
	}
}

// TestRevParseHEAD_ContextCancelled verifies that a pre-cancelled context
// surfaces as context.Canceled (not exit 128 / unborn).
func TestRevParseHEAD_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism (gotcha G5)

	g := New(t.TempDir())
	sha, isUnborn, err := g.RevParseHEAD(ctx)

	if err == nil {
		t.Fatal("RevParseHEAD err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RevParseHEAD err = %v, want context.Canceled", err)
	}
	if isUnborn {
		t.Fatalf("RevParseHEAD isUnborn = true, want false (context cancel is NOT unborn)")
	}
	if sha != "" {
		t.Fatalf("RevParseHEAD sha = %q, want empty string", sha)
	}
}

// TestInsideWorkTree_InsideRepo verifies InsideWorkTree returns (true, nil) for a directory
// inside a git work tree (the not-a-repo pre-flight check's happy path).
func TestInsideWorkTree_InsideRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	g := New(repo)
	inside, err := g.InsideWorkTree(context.Background())
	if err != nil {
		t.Fatalf("InsideWorkTree() err = %v, want nil", err)
	}
	if !inside {
		t.Fatal("InsideWorkTree() = false, want true (inside a repo)")
	}
}

// TestInsideWorkTree_OutsideRepo verifies InsideWorkTree returns (false, nil) — NOT an error —
// for a plain directory outside any git repo. Outside a repo `git rev-parse --is-inside-work-tree`
// prints "false" and exits 1; that exit must NOT be surfaced as an error (it is the signal).
func TestInsideWorkTree_OutsideRepo(t *testing.T) {
	plain := t.TempDir() // a fresh temp dir is not inside any work tree

	g := New(plain)
	inside, err := g.InsideWorkTree(context.Background())
	if err != nil {
		t.Fatalf("InsideWorkTree() err = %v, want nil (outside-a-repo exit 1 is the signal, not an error)", err)
	}
	if inside {
		t.Fatal("InsideWorkTree() = true, want false (outside any git repo)")
	}
}

// TestInsideWorkTree_GitBinaryMissing verifies a missing git binary surfaces as a non-nil error
// (NOT (false, nil) — that would silently look like "not a repo").
func TestInsideWorkTree_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	inside, err := g.InsideWorkTree(context.Background())
	if err == nil {
		t.Fatal("InsideWorkTree() err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("InsideWorkTree() err = %v, want it to contain 'git binary not found'", err)
	}
	if inside {
		t.Fatal("InsideWorkTree() = true, want false (LookPath miss is NOT inside)")
	}
}

// TestInsideWorkTree_ContextCancelled verifies a pre-cancelled context surfaces as context.Canceled
// (not (false, nil)).
func TestInsideWorkTree_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	g := New(t.TempDir())
	inside, err := g.InsideWorkTree(ctx)
	if err == nil {
		t.Fatal("InsideWorkTree() err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("InsideWorkTree() err = %v, want context.Canceled", err)
	}
	if inside {
		t.Fatal("InsideWorkTree() = true, want false (context cancel is NOT inside)")
	}
}
