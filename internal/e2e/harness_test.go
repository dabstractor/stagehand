//go:build e2e

// Package e2e implements the PRD §20.5 throwaway-repo regression harness: per scenario,
// git init a temp repo, seed it, run the compiled stagehand binary as a subprocess (real agent
// when STAGEHAND_RUN_REAL=1, else a stub provider wired via --config + cmd/stubagent), and assert
// the resulting history / exit code. This subprocess angle catches CLI-routing + config-load + real-repo
// bugs that the in-process library tests (§20.1 layer 3, decompose_test.go) cannot reach.
//
// Dual-mode contract:
//   - STAGEHAND_RUN_REAL unset or !=1: stub-reachable scenarios run (S2/S3/S4/S6-single/S7);
//     stager-dependent scenarios (S1/S5/loop-S6) skip with a clear message.
//   - STAGEHAND_RUN_REAL=1: all 7 scenarios run against a real configured agent.
package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/stubtest"
)

// e2eResult bundles a stagehand subprocess run's observable outputs for assertion.
type e2eResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

var (
	stagehandOnce sync.Once
	stagehandBin  string
)

// buildStagehand compiles ./cmd/stagehand ONCE per test process (cached) and returns its path.
// Mirrors stubtest.Build — import-path build so cwd-independent. Skips t if the go toolchain
// is not on PATH.
func buildStagehand(t *testing.T) string {
	t.Helper()
	stagehandOnce.Do(func() {
		goPath, err := exec.LookPath("go")
		if err != nil {
			t.Skipf("go toolchain not on PATH; cannot build stagehand: %v", err)
			return
		}
		dir, err := os.MkdirTemp("", "stagehand-e2e-*")
		if err != nil {
			t.Fatalf("mkdtemp: %v", err)
		}
		name := "stagehand"
		if runtime.GOOS == "windows" {
			name = "stagehand.exe"
		}
		stagehandBin = filepath.Join(dir, name)
		build := exec.Command(goPath, "build", "-o", stagehandBin,
			"github.com/dustin/stagehand/cmd/stagehand")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build stagehand: %v\n%s", err, out)
		}
	})
	return stagehandBin
}

// buildStub returns the stub binary path via stubtest.Build (cached, sync.Once).
func buildStub(t *testing.T) string {
	t.Helper()
	return stubtest.Build(t)
}

// newRepo creates a fresh git repo in t.TempDir with repo-local identity config.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@example.com")
	return dir
}

// seedCommit writes name with body, stages it, and creates a commit.
func seedCommit(t *testing.T, repo, name, body string) {
	t.Helper()
	writeFile(t, repo, name, body)
	runGit(t, repo, "add", name)
	runGit(t, repo, "commit", "-m", "seed: "+name)
}

// writeFile creates a file at repo/name with the given body.
func writeFile(t *testing.T, repo, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}

// stageFile runs git add for name in repo.
func stageFile(t *testing.T, repo, name string) {
	t.Helper()
	runGit(t, repo, "add", name)
}

// runGit executes git -C repo args... and returns trimmed stdout.
func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// headSHA returns the current HEAD SHA of repo.
func headSHA(t *testing.T, repo string) string {
	t.Helper()
	return runGit(t, repo, "rev-parse", "HEAD")
}

// commitCount returns the number of commits reachable from HEAD.
func commitCount(t *testing.T, repo string) int {
	t.Helper()
	s := runGit(t, repo, "rev-list", "--count", "HEAD")
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// diffTreeNames returns the file names in the given commit's diff-tree.
func diffTreeNames(t *testing.T, repo, sha string) []string {
	t.Helper()
	out := runGit(t, repo, "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// statusPorcelain returns git status --porcelain output.
func statusPorcelain(t *testing.T, repo string) string {
	t.Helper()
	return runGit(t, repo, "status", "--porcelain")
}

// writeStubConfig writes a TOML config file to t.TempDir with the base [provider.stub] section
// (pointing at stubBin) plus any extras (for [provider.testmulti], [provider.canary], [role.planner],
// etc.). Returns the config file path. The stub knobs ride the process env (G-ENV-FLOW), not the
// config file.
func writeStubConfig(t *testing.T, stubBin, extras string) string {
	t.Helper()
	body := `config_version = 3

[provider.stub]
command = ` + fmt.Sprintf("%q", stubBin) + `
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
tooled_flags = ["--tooled"]
` + extras + "\n"
	p := filepath.Join(t.TempDir(), "stagehand.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

// stubEnv builds the stagehand process env for one scenario: os.Environ() + the given
// STAGEHAND_STUB_* knobs. Per executor.go: Render = os.Environ()+manifest.Env, so these inherit
// to the stub subprocess.
func stubEnv(knobs map[string]string) []string {
	env := os.Environ()
	for k, v := range knobs {
		env = append(env, k+"="+v)
	}
	return env
}

// runStagehand drives the compiled stagehand binary as a subprocess. Returns captured outputs + exit
// code. Uses a 60s context timeout so a hung agent fails the test (not the suite).
func runStagehand(t *testing.T, bin, repo, cfg string, env []string, args ...string) e2eResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	allArgs := append([]string{"--config", cfg, "--no-color"}, args...)
	cmd := exec.CommandContext(ctx, bin, allArgs...)
	cmd.Dir = repo
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	r := e2eResult{Stdout: out.String(), Stderr: errb.String()}
	if err != nil {
		if ee := (*exec.ExitError)(nil); errors.As(err, &ee) {
			r.ExitCode = ee.ExitCode()
		} else {
			t.Fatalf("run stagehand: %v", err)
		}
	}
	return r
}

// waitForMarker polls for the marker file's existence up to timeout. The marker is written by the
// stub AFTER draining stdin (generation in-flight) and BEFORE the sleep — purpose-built for
// deterministic concurrent races (G-MARKER-IS-DETERMINISTIC).
func waitForMarker(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return // marker found
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("waitForMarker: %s not seen after %v (stub never reached generation)", path, timeout)
}

// skipIfNotReal skips the test if STAGEHAND_RUN_REAL != "1", with a message pointing to both
// the real-run mechanism and the in-process stub suite.
func skipIfNotReal(t *testing.T, why string) {
	t.Helper()
	if os.Getenv("STAGEHAND_RUN_REAL") != "1" {
		t.Skipf("%s (set STAGEHAND_RUN_REAL=1 + STAGEHAND_E2E_PROVIDER for a real run; "+
			"see internal/decompose/decompose_test.go for the in-process stub coverage)", why)
	}
}

// realAgent returns the (provider, model) for a real-agent scenario. Skips if STAGEHAND_RUN_REAL != "1".
// Mirrors realagent_test.go's envOr convention.
func realAgent(t *testing.T) (string, string) {
	t.Helper()
	skipIfNotReal(t, "real agent required")
	provider := envOr("STAGEHAND_E2E_PROVIDER", "pi")
	model := envOr("STAGEHAND_E2E_MODEL", "")
	return provider, model
}

// envOr returns the value of the environment variable key, or def if unset or empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
