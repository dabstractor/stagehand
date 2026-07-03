package hook

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/stubtest"
	"github.com/dustin/stagehand/internal/ui"
)

// runGit runs a git command in repo dir. Test helper.
func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// mustWriteFile is a test helper that writes a file and fatals on error.
func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// initTempRepo creates a temp git repo with a seed commit and returns its path + the git runner.
func initTempRepo(t *testing.T) (string, git.Git) {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@example.com")
	mustWriteFile(t, filepath.Join(dir, "init.txt"), []byte("init\n"))
	runGit(t, dir, "add", "init.txt")
	runGit(t, dir, "commit", "-m", "seed")
	return dir, git.New(dir)
}

func TestNoOpSource(t *testing.T) {
	for _, src := range []string{"message", "template", "merge", "squash", "commit"} {
		if !NoOpSource(src) {
			t.Errorf("NoOpSource(%q) = false, want true", src)
		}
	}
	for _, src := range []string{"", "chat", "foo", "amend"} {
		if NoOpSource(src) {
			t.Errorf("NoOpSource(%q) = true, want false", src)
		}
	}
}

func TestWriteMessageFile_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "msg")

	orig := "# Please enter the commit message...\n# more comments\n"
	mustWriteFile(t, path, []byte(orig))

	msg := "feat: add x"
	if err := WriteMessageFile(path, msg); err != nil {
		t.Fatalf("WriteMessageFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	want := "feat: add x\n\n# Please enter the commit message...\n# more comments\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}

func TestWriteMessageFile_EmptyOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "msg")

	msg := "feat: add y"
	if err := WriteMessageFile(path, msg); err != nil {
		t.Fatalf("WriteMessageFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	want := "feat: add y\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}

func TestRun_SourceGateNoOp(t *testing.T) {
	stubBin := stubtest.Build(t)
	_, g := initTempRepo(t)

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("# comments\n"))

	m := stubtest.Manifest(stubBin, stubtest.Options{Out: "feat: should not appear"})
	cfg := config.Config{Timeout: 10 * time.Second, MaxDuplicateRetries: 2}

	for _, src := range []string{"message", "template", "merge", "squash", "commit"} {
		err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, src)
		if err != ErrNoOp {
			t.Errorf("source=%q: err=%v, want ErrNoOp", src, err)
		}
		data, _ := os.ReadFile(msgFile)
		if string(data) != "# comments\n" {
			t.Errorf("source=%q: msg-file was modified", src)
		}
	}
}

func TestRun_EmptyDiffNoOp(t *testing.T) {
	stubBin := stubtest.Build(t)
	_, g := initTempRepo(t)

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("# comments\n"))

	m := stubtest.Manifest(stubBin, stubtest.Options{Out: "feat: should not appear"})
	cfg := config.Config{Timeout: 10 * time.Second, MaxDuplicateRetries: 2}

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err != ErrNoOp {
		t.Errorf("err=%v, want ErrNoOp (no staged changes)", err)
	}
}

func TestRun_HappyPath(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)

	changePath := filepath.Join(repoDir, "new.txt")
	mustWriteFile(t, changePath, []byte("new content\n"))
	runGit(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	orig := "# Please enter the commit message...\n"
	mustWriteFile(t, msgFile, []byte(orig))

	m := stubtest.Manifest(stubBin, stubtest.Options{Out: "feat: add x\n\nbody text"})
	cfg := config.Config{Timeout: 10 * time.Second, MaxDuplicateRetries: 2}

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(msgFile)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(data)

	if !strings.HasPrefix(s, "feat: add x\n") {
		t.Errorf("msg-file does not start with generated message; got:\n%s", s)
	}
	if !strings.Contains(s, "# Please enter the commit message...") {
		t.Errorf("comment block missing from msg-file; got:\n%s", s)
	}
}

func TestRun_ParseFailRetryThenOK(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)

	changePath := filepath.Join(repoDir, "new.txt")
	mustWriteFile(t, changePath, []byte("new content\n"))
	runGit(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("#\n"))

	m := stubtest.NewScript(t, stubBin, []string{"", "feat: valid message"})
	cfg := config.Config{Timeout: 10 * time.Second, MaxDuplicateRetries: 3}

	var buf strings.Builder
	verbose := ui.NewVerbose(&buf, true)

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m, Verbose: verbose}, cfg, msgFile, "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, _ := os.ReadFile(msgFile)
	if !strings.HasPrefix(string(data), "feat: valid message") {
		t.Errorf("msg-file should start with the valid retry message; got:\n%s", string(data))
	}
}

func TestRun_DuplicateRejected(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)

	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		p := filepath.Join(repoDir, name)
		mustWriteFile(t, p, []byte(name+"\n"))
		runGit(t, repoDir, "add", name)
		runGit(t, repoDir, "commit", "-m", "feat: add x")
	}

	changePath := filepath.Join(repoDir, "new.txt")
	mustWriteFile(t, changePath, []byte("new content\n"))
	runGit(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("#\n"))

	m := stubtest.Manifest(stubBin, stubtest.Options{Out: "feat: add x"})
	cfg := config.Config{Timeout: 10 * time.Second, MaxDuplicateRetries: 2}

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil {
		t.Error("expected error (dup exhaustion), got nil")
	}
	data, _ := os.ReadFile(msgFile)
	if string(data) != "#\n" {
		t.Errorf("msg-file was modified on dup exhaustion; got:\n%s", string(data))
	}
}

func TestRun_StubExit1_NeverBlock(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)

	changePath := filepath.Join(repoDir, "new.txt")
	mustWriteFile(t, changePath, []byte("new content\n"))
	runGit(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	orig := "# original comments\n"
	mustWriteFile(t, msgFile, []byte(orig))

	// Empty output + exit 1 → ParseOutput returns ok=false → retries exhaust → error.
	m := stubtest.Manifest(stubBin, stubtest.Options{Exit: 1, Out: ""})
	cfg := config.Config{Timeout: 10 * time.Second, MaxDuplicateRetries: 1}

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil {
		t.Error("expected error (stub exit 1 + empty output → parse fail exhaustion), got nil")
	}

	data, _ := os.ReadFile(msgFile)
	if string(data) != orig {
		t.Errorf("msg-file was modified on stub exit 1; got:\n%s", string(data))
	}
}

func TestRun_TimeoutNeverBlock(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)

	changePath := filepath.Join(repoDir, "new.txt")
	mustWriteFile(t, changePath, []byte("new content\n"))
	runGit(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	orig := "# original comments\n"
	mustWriteFile(t, msgFile, []byte(orig))

	m := stubtest.Manifest(stubBin, stubtest.Options{SleepMS: 5000})
	cfg := config.Config{Timeout: 50 * time.Millisecond, MaxDuplicateRetries: 2}

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}

	data, _ := os.ReadFile(msgFile)
	if string(data) != orig {
		t.Errorf("msg-file was modified on timeout; got:\n%s", string(data))
	}
}

func TestRun_NoPlumbing(t *testing.T) {
	src, err := os.ReadFile("exec.go")
	if err != nil {
		t.Fatalf("read exec.go: %v", err)
	}
	lines := strings.Split(string(src), "\n")
	forbidden := map[string]bool{
		"WriteTree": true, "CommitTree": true, "UpdateRefCAS": true,
		"DiffTree": true, "signal.": true,
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		for f := range forbidden {
			if strings.Contains(trimmed, f) {
				t.Errorf("exec.go:%d: forbidden reference %q in: %s", i+1, f, trimmed)
			}
		}
	}
}
