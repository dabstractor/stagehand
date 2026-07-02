package git

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// These tests exercise Git.run against the REAL host git binary (git 2.54.0,
// PRD §20.1 layer 2) — no mocks of git, no go-git. They are white-box
// (package git, mirroring internal/provider/executor_test.go being package
// provider) so they can call the unexported run directly. Each test creates
// its temp directories INLINE (the S2 harness does not exist yet) and asserts
// one behavior per Test* function so a failure points straight at what
// regressed.

// TestNew_ResolvesGit proves New resolves the git binary via exec.LookPath and
// returns a usable *Git bound to the given directory.
func TestNew_ResolvesGit(t *testing.T) {
	g, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}
	if g == nil {
		t.Fatal("New returned nil *Git")
	}
	if g.git == "" {
		t.Error("g.git is empty; want the LookPath-resolved git binary path")
	}
}

// TestNew_MissingBinary proves New fails fast with a clear error when git is
// absent from PATH. The host always has git, so the error branch is exercised
// by emptying PATH within the test (t.Setenv auto-restores it on cleanup).
func TestNew_MissingBinary(t *testing.T) {
	t.Setenv("PATH", "")
	if _, err := New(t.TempDir()); err == nil {
		t.Fatal("New returned nil error; want a LookPath failure with PATH empty")
	}
}

// TestRun_Version proves run returns git's version banner on stdout.
func TestRun_Version(t *testing.T) {
	g, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}
	out, err := g.run("version")
	if err != nil {
		t.Fatalf("run(version) returned unexpected error: %v", err)
	}
	if !strings.Contains(out, "git version") {
		t.Errorf("run(version) stdout = %q; want it to contain %q", out, "git version")
	}
}

// TestRun_RevParseGitDirInRepo proves run executes in g.dir: in an init'd
// temp repo, `git rev-parse --git-dir` reports ".git".
func TestRun_RevParseGitDirInRepo(t *testing.T) {
	dir := t.TempDir()
	c := exec.Command("git", "init", "-q")
	c.Dir = dir
	if err := c.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}
	out, err := g.run("rev-parse", "--git-dir")
	if err != nil {
		t.Fatalf("run(rev-parse --git-dir) returned unexpected error: %v", err)
	}
	if got := strings.TrimSpace(out); got != ".git" {
		t.Errorf("rev-parse --git-dir = %q; want %q", got, ".git")
	}
}

// TestRun_NonRepoIsExitError proves a git command in a non-repo directory
// returns a typed *ExitError (assertable via errors.As) with Code 128 and a
// stderr containing "not a git repository", plus the original Args.
func TestRun_NonRepoIsExitError(t *testing.T) {
	g, err := New(t.TempDir()) // no git init → not a repository
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}
	_, err = g.run("rev-parse", "--git-dir")
	if err == nil {
		t.Fatal("run(rev-parse --git-dir) returned nil error; want *ExitError in a non-repo")
	}
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("run error is %T; want *ExitError (errors.As)", err)
	}
	if ee.Code != 128 {
		t.Errorf("ExitError.Code = %d; want 128", ee.Code)
	}
	if !strings.Contains(ee.Stderr, "not a git repository") {
		t.Errorf("ExitError.Stderr = %q; want it to contain %q", ee.Stderr, "not a git repository")
	}
	wantArgs := []string{"rev-parse", "--git-dir"}
	if !reflect.DeepEqual(ee.Args, wantArgs) {
		t.Errorf("ExitError.Args = %v; want %v", ee.Args, wantArgs)
	}
}
