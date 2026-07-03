package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dustin/stagehand/internal/exitcode"
	"github.com/dustin/stagehand/internal/stubtest"
)

// hookexecNewTestRepo creates a temp git repo with identity config and a seed commit.
func hookexecNewTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitCmd(t, dir, "init", "-q")
	runGitCmd(t, dir, "config", "user.name", "Test")
	runGitCmd(t, dir, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitCmd(t, dir, "add", "init.txt")
	runGitCmd(t, dir, "commit", "-m", "seed")
	return dir
}

// writeTestStubConfig writes a minimal stagehand config pointing at the stub binary.
func writeTestStubConfig(t *testing.T, stubBin string) string {
	t.Helper()
	body := `config_version = 3

[provider.stub]
command = "` + stubBin + `"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
`
	p := filepath.Join(t.TempDir(), "stagehand.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

// resetRootCmd clears rootCmd state between tests so shared package vars
// (especially --config flags) don't leak across test functions.
func resetRootCmd() {
	flagHookExecStrict = false
	flagConfig = ""
	loadedCfg = nil
}

func TestHookExec_SourceGateExit0(t *testing.T) {
	defer resetRootCmd()
	stubBin := stubtest.Build(t)
	_ = hookexecNewTestRepo(t)
	cfg := writeTestStubConfig(t, stubBin)

	msgFile := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(msgFile, []byte("# comments\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetArgs([]string{"--config", cfg, "hook", "exec", msgFile, "message"})
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)

	err := rootCmd.ExecuteContext(context.TODO())
	if err != nil {
		t.Errorf("expected nil (exit 0), got: %v", err)
	}
	data, _ := os.ReadFile(msgFile)
	if string(data) != "# comments\n" {
		t.Errorf("msg-file was modified; got:\n%s", string(data))
	}
	if bytes.Contains(errBuf.Bytes(), []byte("Generating")) {
		t.Errorf("progress line must NOT appear for a source-gated no-op; stderr:\n%s", errBuf.String())
	}
}

func TestHookExec_EmptyDiffNoProgress(t *testing.T) {
	defer resetRootCmd()
	stubBin := stubtest.Build(t)
	repoDir := hookexecNewTestRepo(t) // seed commit, nothing staged
	cfg := writeTestStubConfig(t, stubBin)

	msgFile := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(msgFile, []byte("# comments\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetArgs([]string{"--config", cfg, "hook", "exec", msgFile})
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)

	err := rootCmd.ExecuteContext(context.TODO())
	if err != nil {
		t.Errorf("expected nil (exit 0), got: %v", err)
	}
	data, _ := os.ReadFile(msgFile)
	if string(data) != "# comments\n" {
		t.Errorf("msg-file was modified; got:\n%s", string(data))
	}
	if bytes.Contains(errBuf.Bytes(), []byte("Generating")) {
		t.Errorf("progress line must NOT appear for an empty-diff no-op; stderr:\n%s", errBuf.String())
	}
}

func TestHookExec_RangeArgs(t *testing.T) {
	defer resetRootCmd()
	var outBuf, errBuf bytes.Buffer
	rootCmd.SetArgs([]string{"hook", "exec"})
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)

	err := rootCmd.ExecuteContext(context.TODO())
	if err == nil {
		t.Error("expected error for too-few args, got nil")
	}
}

func TestHookExec_StrictFailureNonZero(t *testing.T) {
	defer resetRootCmd()
	stubBin := stubtest.Build(t)
	repoDir := hookexecNewTestRepo(t)
	cfg := writeTestStubConfig(t, stubBin)

	changePath := filepath.Join(repoDir, "new.txt")
	if err := os.WriteFile(changePath, []byte("new content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitCmd(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(msgFile, []byte("# comments\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetArgs([]string{"--config", cfg, "--provider", "stub", "hook", "exec", "--strict", msgFile})
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)

	t.Setenv("STAGEHAND_STUB_EXIT", "1")

	err := rootCmd.ExecuteContext(context.TODO())
	if err == nil {
		t.Fatal("expected non-nil error for strict failure, got nil")
	}
	var ee *exitcode.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *exitcode.ExitError, got %T: %v", err, err)
	}
	if ee.Code != exitcode.Error {
		t.Errorf("expected exit code %d, got %d", exitcode.Error, ee.Code)
	}
	data, _ := os.ReadFile(msgFile)
	if string(data) != "# comments\n" {
		t.Errorf("msg-file was modified on strict failure; got:\n%s", string(data))
	}
	if !bytes.Contains(errBuf.Bytes(), []byte("Generating")) {
		t.Errorf("progress line SHOULD fire for a real staged diff (regression guard); stderr:\n%s", errBuf.String())
	}
}

func TestHookExec_NonStrictFailureExit0(t *testing.T) {
	defer resetRootCmd()
	stubBin := stubtest.Build(t)
	repoDir := hookexecNewTestRepo(t)
	cfg := writeTestStubConfig(t, stubBin)

	changePath := filepath.Join(repoDir, "new.txt")
	if err := os.WriteFile(changePath, []byte("new content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitCmd(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(msgFile, []byte("# comments\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetArgs([]string{"--config", cfg, "--provider", "stub", "hook", "exec", msgFile})
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)

	t.Setenv("STAGEHAND_STUB_EXIT", "1")

	err := rootCmd.ExecuteContext(context.TODO())
	if err != nil {
		t.Errorf("expected nil (exit 0 non-strict), got: %v", err)
	}
	data, _ := os.ReadFile(msgFile)
	if string(data) != "# comments\n" {
		t.Errorf("msg-file was modified on non-strict failure; got:\n%s", string(data))
	}
}

func TestHookExec_StrictFail_HasStderrLine(t *testing.T) {
	defer resetRootCmd()
	stubBin := stubtest.Build(t)
	repoDir := hookexecNewTestRepo(t)
	cfg := writeTestStubConfig(t, stubBin)

	changePath := filepath.Join(repoDir, "new.txt")
	if err := os.WriteFile(changePath, []byte("new content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitCmd(t, repoDir, "add", "new.txt")

	msgFile := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(msgFile, []byte("#\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetArgs([]string{"--config", cfg, "--provider", "stub", "hook", "exec", "--strict", msgFile})
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)

	t.Setenv("STAGEHAND_STUB_EXIT", "1")

	_ = rootCmd.ExecuteContext(context.TODO())

	errStr := errBuf.String()
	if !strings.Contains(errStr, "stagehand:") {
		t.Errorf("expected 'stagehand:' on stderr, got: %s", errStr)
	}
	lines := strings.Split(strings.TrimSpace(errStr), "\n")
	if len(lines) > 2 {
		t.Errorf("expected at most 2 stderr lines, got %d: %s", len(lines), errStr)
	}
}
