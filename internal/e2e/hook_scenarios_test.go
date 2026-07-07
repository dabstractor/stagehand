//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runGitCommit runs git commit in a repo with a custom env (for PATH/stub knobs/GIT_EDITOR).
// Returns stdout, stderr, exit code.
func runGitCommit(t *testing.T, repo string, env []string, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	allArgs := append([]string{"-C", repo, "commit"}, args...)
	cmd := exec.CommandContext(ctx, "git", allArgs...)
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	stdout, stderr := out.String(), errb.String()
	if err != nil {
		if ee := (*exec.ExitError)(nil); errors.As(err, &ee) {
			return stdout, stderr, ee.ExitCode()
		}
		t.Fatalf("git commit: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	return stdout, stderr, 0
}

// runGitCommitHook installs the stagehand hook in repo, then runs git commit with the given env.
// The env must include PATH (with stagehand binary dir), STAGEHAND_CONFIG, and stub knobs.
func runGitCommitHook(t *testing.T, bin, repo, cfg string, env []string, gitArgs ...string) (string, string, int) {
	t.Helper()
	// Install the hook.
	installArgs := []string{"--config", cfg, "hook", "install"}
	installCmd := exec.Command(bin, installArgs...)
	installCmd.Dir = repo
	installCmd.Env = env
	if out, err := installCmd.CombinedOutput(); err != nil {
		t.Fatalf("hook install: %v\n%s", err, out)
	}
	return runGitCommit(t, repo, env, gitArgs...)
}

// buildStagecoachPath returns just the directory of the built stagehand binary.
func buildStagecoachPath(t *testing.T) string {
	t.Helper()
	bin := buildStagecoach(t)
	return filepath.Dir(bin)
}

// prependPath returns env with dir prepended to PATH.
func prependPath(env []string, dir string) []string {
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = "PATH=" + dir + ":" + e[5:]
			return env
		}
	}
	return append(env, "PATH="+dir+":"+os.Getenv("PATH"))
}

// TestE2EHookScenarios exercises the PRD §9.20 FR-H4/H5/H7 hook scenarios via real git commit.
func TestE2EHookScenarios(t *testing.T) {
	bin := buildStagecoach(t)
	stub := buildStub(t)
	binDir := buildStagecoachPath(t)

	cfg := writeStubConfig(t, stub, `
[defaults]
provider = "stub"
`)

	t.Run("happy_path", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")

		// Stage a change.
		writeFile(t, repo, "feature.txt", "feature\n")
		stageFile(t, repo, "feature.txt")

		env := stubEnv(map[string]string{
			"STAGEHAND_CONFIG":   cfg,
			"STAGEHAND_STUB_OUT": "feat: generated message\n",
			"GIT_EDITOR":         "true",
		})
		env = prependPath(env, binDir)

		_, stderr, exitCode := runGitCommitHook(t, bin, repo, cfg, env)
		if exitCode != 0 {
			t.Fatalf("git commit exit code = %d, want 0; stderr:\n%s", exitCode, stderr)
		}

		// HEAD message should be the stub output.
		msg := runGit(t, repo, "log", "-1", "--format=%s")
		if msg != "feat: generated message" {
			t.Errorf("HEAD subject = %q, want %q", msg, "feat: generated message")
		}

		// Should be 2 commits (seed + hook-generated).
		if n := commitCount(t, repo); n != 2 {
			t.Errorf("commit count = %d, want 2", n)
		}
	})

	t.Run("failure_never_block", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")

		writeFile(t, repo, "change.txt", "change\n")
		stageFile(t, repo, "change.txt")

		// Stub exits 1 → hook should never-block (exit 0), git continues.
		// GIT_EDITOR writes "fallback" so the commit has a message.
		editorScript := filepath.Join(t.TempDir(), "editor.sh")
		if err := os.WriteFile(editorScript, []byte("#!/bin/sh\necho 'fallback' > \"$1\"\n"), 0o755); err != nil {
			t.Fatalf("write editor: %v", err)
		}

		env := stubEnv(map[string]string{
			"STAGEHAND_CONFIG":    cfg,
			"STAGEHAND_STUB_EXIT": "1",
			"GIT_EDITOR":          editorScript,
		})
		env = prependPath(env, binDir)

		_, stderr, exitCode := runGitCommitHook(t, bin, repo, cfg, env)
		if exitCode != 0 {
			t.Fatalf("git commit exit code = %d, want 0 (never-block); stderr:\n%s", exitCode, stderr)
		}

		msg := runGit(t, repo, "log", "-1", "--format=%s")
		if msg != "fallback" {
			t.Errorf("HEAD subject = %q, want %q (hook should have exited 0, editor filled it)", msg, "fallback")
		}
	})

	t.Run("m_flag_noop", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")

		writeFile(t, repo, "change.txt", "change\n")
		stageFile(t, repo, "change.txt")

		env := stubEnv(map[string]string{
			"STAGEHAND_CONFIG":   cfg,
			"STAGEHAND_STUB_OUT": "SHOULD NOT APPEAR",
			"GIT_EDITOR":         "true",
		})
		env = prependPath(env, binDir)

		// git commit -m "explicit" → source=message → hook no-ops.
		_, stderr, exitCode := runGitCommitHook(t, bin, repo, cfg, env, "-m", "explicit")
		if exitCode != 0 {
			t.Fatalf("git commit exit code = %d, want 0; stderr:\n%s", exitCode, stderr)
		}

		msg := runGit(t, repo, "log", "-1", "--format=%s")
		if msg != "explicit" {
			t.Errorf("HEAD subject = %q, want %q (hook should have no-op'd on -m)", msg, "explicit")
		}
	})
}
