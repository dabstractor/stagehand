package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func initRepo(t *testing.T, dir string) {
	t.Helper()
	// Set a minimal git identity so commits and other operations don't fail.
	cmd := exec.Command("git", "-C", dir, "init")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test <test@example.com>",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test <test@example.com>",
		"GIT_COMMITTER_EMAIL=test@example.com>",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
}

func TestNew(t *testing.T) {
	g := New("/tmp")
	if g == nil {
		t.Fatal("New returned nil")
	}
	gr, ok := g.(*gitRunner)
	if !ok {
		t.Fatalf("New did not return *gitRunner, got %T", g)
	}
	if gr.workDir != "/tmp" {
		t.Fatalf("workDir = %q, want %q", gr.workDir, "/tmp")
	}
}

func TestRun_HappyPath(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	g := &gitRunner{workDir: repo}
	ctx := context.Background()
	stdout, stderr, code, err := g.run(ctx, repo, "rev-parse", "--git-dir")

	if err != nil {
		t.Fatalf("run() err = %v, want nil", err)
	}
	if code != 0 {
		t.Fatalf("run() exitCode = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != ".git" {
		t.Fatalf("run() stdout = %q, want .git", strings.TrimSpace(stdout))
	}
	if stderr != "" {
		t.Fatalf("run() stderr = %q, want empty", stderr)
	}
}

func TestRun_CapturesExitCodeAndSeparateBuffers(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// Unborn repo (zero commits): rev-parse HEAD exits 128 and prints literal "HEAD\n".
	g := &gitRunner{workDir: repo}
	ctx := context.Background()
	stdout, stderr, code, err := g.run(ctx, repo, "rev-parse", "HEAD")

	if err != nil {
		t.Fatalf("run() err = %v, want nil (exit 128 is NOT a Go error, gotcha G2)", err)
	}
	if code != 128 {
		t.Fatalf("run() exitCode = %d, want 128 (unborn repo)", code)
	}
	if strings.TrimSpace(stdout) != "HEAD" {
		t.Fatalf("run() stdout = %q, want \"HEAD\" (gotcha G3: unborn prints literal HEAD)", strings.TrimSpace(stdout))
	}
	if stderr == "" || !bytes.Contains([]byte(stderr), []byte("ambiguous argument 'HEAD'")) {
		t.Fatalf("run() stderr = %q, want it to contain 'ambiguous argument' (proves separate stderr buffer)", stderr)
	}
}

func TestRun_LookPathFailure(t *testing.T) {
	t.Setenv("PATH", "") // makes LookPath("git") fail for this test only (gotcha G10)

	g := &gitRunner{workDir: "."}
	ctx := context.Background()
	_, _, code, err := g.run(ctx, ".", "version")

	if err == nil {
		t.Fatal("run() err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("run() err = %v, want it to contain 'git binary not found'", err)
	}
	if code != -1 {
		t.Fatalf("run() exitCode = %d, want -1 (sentinel for infrastructural failure)", code)
	}
}

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("%s: expected panic, but did not panic", name)
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "not yet implemented") {
			t.Fatalf("%s: panic message = %v, want it to contain 'not yet implemented'", name, r)
		}
	}()
	fn()
}

func TestStubsPanic(t *testing.T) {
	ctx := context.Background()
	g := New(".")

	assertPanics(t, "HasStagedChanges", func() { _, _ = g.HasStagedChanges(ctx) })
	assertPanics(t, "RecentMessages", func() { _, _ = g.RecentMessages(ctx, 5) })
	assertPanics(t, "RecentSubjects", func() { _, _ = g.RecentSubjects(ctx, 5) })
	assertPanics(t, "CommitCount", func() { _, _ = g.CommitCount(ctx) })
	assertPanics(t, "AddAll", func() { _ = g.AddAll(ctx) })
}
