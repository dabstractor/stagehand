package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// makeMergeConflict creates an unresolved merge conflict in the repo at dir.
// The sequence: write conflict.txt="base"; commit; branch side; write "side"; commit;
// return to original branch; write "main"; commit; merge side (expect non-zero, t.Fatalf if clean).
// After this, git ls-files -u shows 3 unmerged entries and write-tree exits 128.
func makeMergeConflict(t *testing.T, dir string) {
	t.Helper()

	writeFile := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("writeFile(%q): %v", name, err)
		}
	}

	runGit := func(args ...string) string {
		t.Helper()
		env := append(minGitEnv(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		cmd := exec.Command("git", "-C", dir)
		cmd.Args = append(cmd.Args, args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out)
		}
		return string(out)
	}

	// base commit
	writeFile("conflict.txt", "base\n")
	runGit("add", "conflict.txt")
	runGit("commit", "-m", "base")

	// side branch with a conflicting change
	runGit("checkout", "-b", "side")
	writeFile("conflict.txt", "side\n")
	runGit("commit", "-am", "side-change")

	// return to original branch (branch-name-agnostic via git checkout -)
	runGit("checkout", "-")
	writeFile("conflict.txt", "main\n")
	runGit("commit", "-am", "main-change")

	// merge side — expected to FAIL with conflict (gotcha G10)
	out := runGit("merge", "side")
	if strings.Contains(out, "Already up to date") {
		t.Fatalf("makeMergeConflict: merge succeeded cleanly (no conflict created)")
	}
	// non-zero exit is EXPECTED; we only t.Fatalf if it somehow succeeds cleanly
}

// TestWriteTree_StagedFiles verifies that WriteTree returns a 40/64-hex SHA on a repo
// with staged files (happy path).
func TestWriteTree_StagedFiles(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// write and stage a file
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	env := append(minGitEnv(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	cmd := exec.Command("git", "-C", repo, "add", "a.txt")
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	g := New(repo)
	sha, err := g.WriteTree(context.Background())
	if err != nil {
		t.Fatalf("WriteTree err = %v, want nil", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(sha) {
		t.Fatalf("WriteTree sha = %q, want 40 or 64 hex chars", sha)
	}
}

// TestWriteTree_EmptyIndex verifies that WriteTree on an empty/unborn index returns
// the canonical sha-1 empty-tree object ID (gotcha G7).
func TestWriteTree_EmptyIndex(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo) // no commits, nothing staged

	g := New(repo)
	sha, err := g.WriteTree(context.Background())
	if err != nil {
		t.Fatalf("WriteTree err = %v, want nil", err)
	}
	const emptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	if sha != emptyTreeSHA {
		t.Fatalf("WriteTree sha = %q, want %q (canonical sha-1 empty tree)", sha, emptyTreeSHA)
	}
}

// TestWriteTree_MergeConflict verifies that WriteTree returns a descriptive error
// containing "unresolved merge conflicts" when the index has unmerged entries
// (the core failure mode — git write-tree exit 128).
func TestWriteTree_MergeConflict(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeMergeConflict(t, repo)

	g := New(repo)
	sha, err := g.WriteTree(context.Background())
	if err == nil {
		t.Fatal("WriteTree err = nil, want non-nil (unresolved merge conflicts)")
	}
	if !strings.Contains(err.Error(), "unresolved merge conflicts") {
		t.Fatalf("WriteTree err = %v, want it to contain 'unresolved merge conflicts'", err)
	}
	// The conflict message must be a single clean line — no raw git stderr noise (bugfix-002 Issue 3).
	if strings.Contains(err.Error(), "fatal: git-write-tree") {
		t.Errorf("WriteTree err = %q; want it to NOT contain raw 'fatal: git-write-tree' stderr", err.Error())
	}
	if strings.Contains(err.Error(), "error building trees") {
		t.Errorf("WriteTree err = %q; want it to NOT contain raw 'error building trees' stderr", err.Error())
	}
	if sha != "" {
		t.Fatalf("WriteTree sha = %q, want empty string on conflict", sha)
	}
}

// TestWriteTree_GitBinaryMissing verifies that a missing git binary surfaces
// as a non-nil error mentioning "git binary not found" (NOT misread as a conflict).
func TestWriteTree_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir()) // dir need not be a repo; LookPath fails first
	sha, err := g.WriteTree(context.Background())
	if err == nil {
		t.Fatal("WriteTree err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("WriteTree err = %v, want it to contain 'git binary not found'", err)
	}
	if sha != "" {
		t.Fatalf("WriteTree sha = %q, want empty string", sha)
	}
}

// TestWriteTree_ContextCancelled verifies that a pre-cancelled context surfaces
// as context.Canceled (not exit 0 or 128).
func TestWriteTree_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := New(t.TempDir())
	sha, err := g.WriteTree(ctx)
	if err == nil {
		t.Fatal("WriteTree err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WriteTree err = %v, want context.Canceled", err)
	}
	if sha != "" {
		t.Fatalf("WriteTree sha = %q, want empty string", sha)
	}
}
