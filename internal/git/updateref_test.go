package git

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// --- Fixture helpers (cas-prefixed to avoid name collisions with S4's committree_test.go helpers) ---

// casOut runs a raw git command in dir and returns its trimmed stdout.
func casOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("casOut(%v): %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// gitIdentityEnv returns env with PATH+HOME plus GIT_AUTHOR/COMMITTER identity.
// Reuses minGitEnv from revparse_test.go (same package).
func gitIdentityEnv() []string {
	return append(minGitEnv(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
}

// casCommit creates a dangling commit object (does NOT move any ref).
// If parents is non-empty, resolves the tree from parents[0]; otherwise uses the empty-tree SHA.
func casCommit(t *testing.T, dir string, parents []string, msg string) string {
	t.Helper()
	var tree string
	if len(parents) > 0 {
		tree = casOut(t, dir, "rev-parse", parents[0]+"^{tree}")
	} else {
		tree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // sha-1 empty tree
	}
	args := make([]string, 0, 3+len(parents)*2)
	args = append(args, "commit-tree", tree)
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	args = append(args, "-m", msg)

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = gitIdentityEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("casCommit: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// casHEAD returns the current HEAD SHA of the repo at dir.
func casHEAD(t *testing.T, dir string) string {
	t.Helper()
	return casOut(t, dir, "rev-parse", "HEAD")
}

// casMoveHEAD force-moves HEAD to sha using the raw 2-arg form (test-fixture ONLY;
// production code never uses the force form — gotcha G6/G16).
func casMoveHEAD(t *testing.T, dir, sha string) {
	t.Helper()
	casOut(t, dir, "update-ref", "HEAD", sha)
}

// --- Test functions ---

func TestUpdateRefCAS_Success(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "initial")
	makeEmptyCommit(t, repo, "second") // HEAD = C1

	c1 := casHEAD(t, repo)
	newSHA := casCommit(t, repo, []string{c1}, "feat: third") // dangling child of C1

	g := New(repo)
	err := g.UpdateRefCAS(context.Background(), "HEAD", newSHA, c1)
	if err != nil {
		t.Fatalf("UpdateRefCAS err = %v, want nil", err)
	}
	if got := casHEAD(t, repo); got != newSHA {
		t.Fatalf("HEAD = %q, want %q", got, newSHA)
	}
}

func TestUpdateRefCAS_StaleExpected(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "initial")
	makeEmptyCommit(t, repo, "second") // HEAD = C1

	c0 := casOut(t, repo, "rev-parse", "HEAD~1")
	c1 := casHEAD(t, repo)
	newSHA := casCommit(t, repo, []string{c1}, "feat: third")

	// Simulate the race: move HEAD to c0, then call with stale expected=c1
	casMoveHEAD(t, repo, c0)

	g := New(repo)
	err := g.UpdateRefCAS(context.Background(), "HEAD", newSHA, c1)
	if err == nil {
		t.Fatal("UpdateRefCAS err = nil, want non-nil (stale expected)")
	}
	if !errors.Is(err, ErrCASFailed) {
		t.Fatalf("errors.Is(err, ErrCASFailed) = false, want true; err = %v", err)
	}
	if !strings.Contains(err.Error(), "(exit 128)") {
		t.Fatalf("err.Error() does not contain '(exit 128)'; got %v", err)
	}
	if got := casHEAD(t, repo); got != c0 {
		t.Fatalf("HEAD = %q after failed CAS, want %q (unchanged)", got, c0)
	}
}

func TestUpdateRefCAS_RootCommit(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo) // unborn — zero commits

	const zeros = "0000000000000000000000000000000000000000"
	newSHA := casCommit(t, repo, nil, "feat: root") // dangling root commit

	g := New(repo)
	err := g.UpdateRefCAS(context.Background(), "HEAD", newSHA, zeros)
	if err != nil {
		t.Fatalf("UpdateRefCAS err = %v, want nil (root commit via all-zeros)", err)
	}
	if got := casHEAD(t, repo); got != newSHA {
		t.Fatalf("HEAD = %q, want %q (root commit published)", got, newSHA)
	}
}

func TestUpdateRefCAS_AllZerosOnBornRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "initial") // HEAD exists (born)

	const zeros = "0000000000000000000000000000000000000000"
	c0 := casHEAD(t, repo)

	g := New(repo)
	err := g.UpdateRefCAS(context.Background(), "HEAD", c0, zeros)
	if err == nil {
		t.Fatal("UpdateRefCAS err = nil, want non-nil (all-zeros on born repo)")
	}
	if !errors.Is(err, ErrCASFailed) {
		t.Fatalf("errors.Is(err, ErrCASFailed) = false, want true; err = %v", err)
	}
	if got := casHEAD(t, repo); got != c0 {
		t.Fatalf("HEAD = %q after failed CAS, want %q (unchanged)", got, c0)
	}
}

func TestUpdateRefCAS_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	err := g.UpdateRefCAS(context.Background(), "HEAD", "new", "old")
	if err == nil {
		t.Fatal("UpdateRefCAS err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("err.Error() = %v, want it to contain 'git binary not found'", err)
	}
	if errors.Is(err, ErrCASFailed) {
		t.Fatal("errors.Is(err, ErrCASFailed) = true, want false (NOT a CAS failure)")
	}
}

func TestUpdateRefCAS_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call

	g := New(t.TempDir())
	err := g.UpdateRefCAS(ctx, "HEAD", "new", "old")
	if err == nil {
		t.Fatal("UpdateRefCAS err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is(err, context.Canceled) = false, want true; err = %v", err)
	}
	if errors.Is(err, ErrCASFailed) {
		t.Fatal("errors.Is(err, ErrCASFailed) = true, want false (NOT a CAS failure)")
	}
}
