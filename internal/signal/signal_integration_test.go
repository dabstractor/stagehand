//go:build !windows

package signal

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestSignalIntegration_SigintPostSnapshot verifies the full signal path end-to-end:
// build the real stagecoach binary, use a hanging stub agent, send SIGINT after the snapshot
// is taken, assert exit 3 + rescue message printed + HEAD unchanged.
//
// This is the ONLY way to test the real os.Exit(3) path (see design-decisions F5).
func TestSignalIntegration_SigintPostSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test (builds real binary + slow stub)")
	}

	stubBin := buildStub(t)
	stagecoachBin := buildStagecoach(t)

	// Set up a git repo with staged changes.
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test")
	runGit(t, repo, "config", "user.email", "test@example.com")
	writeFile(t, repo, "initial.txt", "initial")
	runGit(t, repo, "add", "initial.txt")
	runGit(t, repo, "commit", "-m", "initial")

	writeFile(t, repo, "new.txt", "staged content")
	runGit(t, repo, "add", "new.txt")

	beforeHEAD := runGit(t, repo, "rev-parse", "HEAD")

	// Write a config that uses the stub agent with a 30s hang.
	cfgPath := filepath.Join(repo, ".stagecoach.toml")
	configContent := "[defaults]\nprovider = \"stub\"\n[provider.stub]\ncommand = \"" + stubBin + "\"\nprompt_delivery = \"stdin\"\noutput = \"raw\"\n"
	if err := os.WriteFile(cfgPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Run stagecoach with the hanging stub.
	cmd := exec.Command(stagecoachBin, "--config", cfgPath)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"STAGECOACH_STUB_SLEEP_MS=30000", // stub hangs 30s (plenty of time)
		"GIT_CONFIG_NOSYSTEM=1",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start stagecoach: %v", err)
	}

	// Wait for the snapshot to be taken and the agent to start. WriteTree + stub startup
	// should complete well within 800ms; the stub then hangs for 30s.
	time.Sleep(800 * time.Millisecond)

	// Send SIGINT (simulate Ctrl-C).
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}

	err := cmd.Wait()
	// The process should exit with code 3.
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *exec.ExitError, got: %v", err)
	}
	if ee.ExitCode() != 3 {
		t.Errorf("exit code = %d, want 3", ee.ExitCode())
	}

	out := stderr.String()

	// Assert rescue message content.
	if !strings.Contains(out, "Commit generation failed") {
		t.Errorf("stderr missing 'Commit generation failed':\n%s", out)
	}
	if !strings.Contains(out, "Tree ID:") {
		t.Errorf("stderr missing 'Tree ID:':\n%s", out)
	}
	if !strings.Contains(out, "git commit-tree") {
		t.Errorf("stderr missing 'git commit-tree':\n%s", out)
	}
	if !strings.Contains(out, "update-ref HEAD") {
		t.Errorf("stderr missing 'update-ref HEAD':\n%s", out)
	}

	// HEAD should be unchanged.
	afterHEAD := runGit(t, repo, "rev-parse", "HEAD")
	if afterHEAD != beforeHEAD {
		t.Errorf("HEAD changed: before=%s after=%s", beforeHEAD, afterHEAD)
	}

	// Parse the Tree ID from stderr and verify it's a real git tree object.
	treeMatch := treeIDRe.FindStringSubmatch(out)
	if len(treeMatch) < 2 {
		t.Fatalf("could not parse Tree ID from stderr")
	}
	treeType := runGit(t, repo, "cat-file", "-t", treeMatch[1])
	if treeType != "tree" {
		t.Errorf("Tree ID object type = %q, want 'tree'", treeType)
	}

	// Staged index should be unchanged.
	beforeIndex := "new.txt" // we staged only new.txt
	afterIndex := runGit(t, repo, "diff", "--cached", "--name-only")
	if !strings.Contains(afterIndex, beforeIndex) {
		t.Errorf("staged file 'new.txt' missing from index after signal")
	}
}

var treeIDRe = regexp.MustCompile(`Tree ID: ([0-9a-f]{7,40})`)

// buildStub compiles cmd/stubagent once and returns its path.
var (
	buildStubOnce sync.Once
	buildStubPath string
)

func buildStub(t *testing.T) string {
	t.Helper()
	buildStubOnce.Do(func() {
		dir, err := os.MkdirTemp("", "stagecoach-stubtest-*")
		if err != nil {
			t.Fatalf("mkdtemp: %v", err)
		}
		name := "stubagent"
		out := filepath.Join(dir, name)
		cmd := exec.Command("go", "build", "-o", out, "github.com/dabstractor/stagecoach/cmd/stubagent")
		if o, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go build stubagent: %v\n%s", err, o)
		}
		buildStubPath = out
	})
	return buildStubPath
}

// buildStagecoach compiles cmd/stagecoach once and returns its path.
var (
	buildStagecoachOnce sync.Once
	buildStagecoachPath string
)

func buildStagecoach(t *testing.T) string {
	t.Helper()
	buildStagecoachOnce.Do(func() {
		dir, err := os.MkdirTemp("", "stagecoach-inttest-*")
		if err != nil {
			t.Fatalf("mkdtemp: %v", err)
		}
		name := "stagecoach"
		out := filepath.Join(dir, name)
		cmd := exec.Command("go", "build", "-o", out, "github.com/dabstractor/stagecoach/cmd/stagecoach")
		if o, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go build stagecoach: %v\n%s", err, o)
		}
		buildStagecoachPath = out
	})
	return buildStagecoachPath
}

// runGit executes git -C dir args... and returns trimmed stdout.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// writeFile creates a file at dir/name with body.
func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}

// TestKillProcessGroup_Unix sends SIGTERM to a real child process in its own process group
// (Setpgid=true, matching the executor's pattern) and verifies the child exits.
func TestKillProcessGroup_Unix(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start sleep child: %v", err)
	}
	pid := cmd.Process.Pid

	if err := KillProcessGroup(pid, syscall.SIGTERM); err != nil {
		t.Fatalf("KillProcessGroup(%d, SIGTERM): %v", pid, err)
	}

	err := cmd.Wait()
	if err != nil {
		t.Logf("child exited: %v (expected after SIGTERM)", err)
	}
}
