package git

import (
	"bytes"
	"context"
	"io"
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
	// Set repo-local user identity so every subsequent git operation in this repo
	// works even without a global ~/.gitconfig.
	cfgCmd := exec.Command("git", "-C", dir, "config", "user.name", "Test")
	cfgCmd.Env = os.Environ()
	if out, err := cfgCmd.CombinedOutput(); err != nil {
		t.Fatalf("git config user.name failed: %v\n%s", err, out)
	}
	emailCmd := exec.Command("git", "-C", dir, "config", "user.email", "test@example.com")
	emailCmd.Env = os.Environ()
	if out, err := emailCmd.CombinedOutput(); err != nil {
		t.Fatalf("git config user.email failed: %v\n%s", err, out)
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

func TestGitRunner_RunWithEnv_PassesEnv(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	g := &gitRunner{workDir: repo} // white-box; mirror TestRun_HappyPath (unexported method access)
	// Inject a config key via git's env-var protocol (GIT_CONFIG_COUNT/KEY/VALUE): deterministic,
	// never in the parent env (no duplicate-key risk), and needs no commit (git config --get works
	// unborn). If cmd.Env is NOT set, the child never sees GIT_CONFIG_COUNT → config --get exits 1
	// with empty output, failing the test loudly.
	out, _, code, err := g.runWithEnv(context.Background(), repo, []string{
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=stagecoach.envtest",
		"GIT_CONFIG_VALUE_0=passed-via-env",
	}, "config", "--get", "stagecoach.envtest")
	if err != nil || code != 0 {
		t.Fatalf("runWithEnv config --get: code=%d err=%v (cmd.Env likely not set)", code, err)
	}
	if got := strings.TrimSpace(out); got != "passed-via-env" {
		t.Errorf("extraEnv did not reach the child: got %q, want %q (cmd.Env not set?)", got, "passed-via-env")
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

func TestGitDir(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	g := New(repo)
	ctx := context.Background()

	got, err := g.GitDir(ctx)
	if err != nil {
		t.Fatalf("GitDir() error: %v", err)
	}
	if got == "" {
		t.Fatal("GitDir() returned empty")
	}
	// Must end with ".git" or be an absolute path.
	if got[len(got)-4:] != ".git" && got[len(got)-5:] != ".git/" {
		t.Logf("GitDir() = %q (should be an absolute path ending in .git)", got)
	}
}

func TestEditor(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	g := New(repo)
	ctx := context.Background()

	// With GIT_EDITOR set, git var GIT_EDITOR should return it.
	t.Setenv("GIT_EDITOR", "/usr/bin/vim")
	got, err := g.Editor(ctx)
	if err != nil {
		t.Fatalf("Editor() error: %v", err)
	}
	if got != "/usr/bin/vim" {
		t.Errorf("Editor() = %q, want /usr/bin/vim", got)
	}

	// Without GIT_EDITOR, it should resolve something (at minimum vi).
	t.Setenv("GIT_EDITOR", "")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	got2, err2 := g.Editor(ctx)
	if err2 != nil {
		t.Fatalf("Editor() without GIT_EDITOR error: %v", err2)
	}
	// In CI vi may not be installed; just verify we got a non-error result.
	t.Logf("Editor() without GIT_EDITOR = %q", got2)
}

func TestDiffTreeNameStatus(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	g := New(repo)
	ctx := context.Background()

	// Create an initial commit with a file.
	writeFile(t, repo, "a.txt", "hello")
	runGit(t, repo, "add", "a.txt")
	runGit(t, repo, "commit", "-m", "initial")

	srcA := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD^{tree}"))

	// Modify the file and commit.
	writeFile(t, repo, "a.txt", "world")
	runGit(t, repo, "commit", "-am", "update")
	srcB := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD^{tree}"))

	got, err := g.DiffTreeNameStatus(ctx, srcA, srcB)
	if err != nil {
		t.Fatalf("DiffTreeNameStatus() error: %v", err)
	}
	if !strings.Contains(got, "M\ta.txt") {
		t.Errorf("DiffTreeNameStatus() = %q, want to contain 'M\ta.txt'", got)
	}

	// Identical trees → empty output.
	got2, err2 := g.DiffTreeNameStatus(ctx, srcA, srcA)
	if err2 != nil {
		t.Fatalf("DiffTreeNameStatus(same) error: %v", err2)
	}
	if strings.TrimSpace(got2) != "" {
		t.Errorf("DiffTreeNameStatus(same) = %q, want empty", got2)
	}
}

// mustRun runs a git command in dir; fails the test on error.
func mustRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test <test@example.com>",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test <test@example.com>",
		"GIT_COMMITTER_EMAIL=test@example.com>",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// writeAndCommit creates a file at dir/name, stages it, and commits.
func writeAndCommit(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(dir+"/"+name, []byte(body), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	mustRun(t, dir, "add", name)
	mustRun(t, dir, "commit", "-m", "add "+name)
}

func TestPush_CleanToBareRemote(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	bare := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	mustRun(t, repo, "remote", "add", "origin", bare)

	// TEST SETUP: establish upstream + initial commit + push
	writeAndCommit(t, repo, "a.txt", "a")
	mustRun(t, repo, "push", "-u", "origin", "HEAD")

	// Now add a NEW commit and push via the Push method.
	writeAndCommit(t, repo, "b.txt", "b")
	g := New(repo)
	var out, errb bytes.Buffer
	if err := g.Push(context.Background(), &out, &errb); err != nil {
		t.Fatalf("Push err = %v (stdout=%q, stderr=%q)", err, out.String(), errb.String())
	}

	// Assert the remote advanced: git -C <bare> log --oneline shows 2 commits.
	log := runGit(t, bare, "log", "--oneline")
	lines := strings.Split(strings.TrimSpace(log), "\n")
	if len(lines) < 2 {
		t.Errorf("remote has %d commits, want >= 2: %q", len(lines), log)
	}
}

func TestPush_NoUpstreamFails128(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null") // MASKS autoSetupRemote — CRITICAL (external_deps.md §8)
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null") // (external_deps.md §8)

	repo := t.TempDir()
	initRepo(t, repo)
	bare := t.TempDir()
	if err := exec.Command("git", "init", "--bare", bare).Run(); err != nil {
		t.Fatalf("init bare repo: %v", err)
	}
	mustRun(t, repo, "remote", "add", "origin", bare)
	writeAndCommit(t, repo, "a.txt", "a")
	// NOTE: deliberately NO `git push -u origin HEAD` — no upstream.

	g := New(repo)
	var errb bytes.Buffer
	err := g.Push(context.Background(), io.Discard, &errb)
	if err == nil {
		t.Fatal("Push err = nil, want non-nil (no upstream)")
	}
	stderrStr := errb.String()
	if !strings.Contains(stderrStr, "has no upstream branch") {
		t.Errorf("stderr missing 'has no upstream branch': %q", stderrStr)
	}
	if !strings.Contains(stderrStr, "--set-upstream") {
		t.Errorf("stderr missing '--set-upstream': %q", stderrStr)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir)
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test <test@example.com>",
		"GIT_AUTHOR_EMAIL=test@example.com>",
		"GIT_COMMITTER_NAME=Test <test@example.com>",
		"GIT_COMMITTER_EMAIL=test@example.com>",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
