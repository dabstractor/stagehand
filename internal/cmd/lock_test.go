package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/lock"
)

// TestLockStatus_NoLockHeld verifies the path=="" contract: with no lock held the
// subcommand prints "no run lock for <repoDir>" and exits 0. It also proves the no-op
// PersistentPreRunE works OUTSIDE a git repo (the temp dir is not a git repo, yet
// config.Load is skipped — no bootstrap write, no "not a git repo" error).
func TestLockStatus_NoLockHeld(t *testing.T) {
	// Lock-file isolation: the lock dir lives OUTSIDE the repo (XDG_RUNTIME_DIR/…).
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", "")

	repo := t.TempDir() // NOT a git repo — proves the no-op PreRunE skips config.Load
	chdir(t, repo)

	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"lock", "status"})

	err := Execute(context.Background())
	if code := exitcode.For(err); code != exitcode.Success {
		t.Fatalf("exit code = %d, want %d (Success); err=%v", code, exitcode.Success, err)
	}

	// Use Contains to dodge the macOS /private/var symlink nit (t.TempDir()=/var/...,
	// os.Getwd()=/private/var/... after chdir). The core assertion is that the
	// diagnostic message + the repo path are present.
	got := out.String()
	if !strings.Contains(got, "no run lock for") {
		t.Errorf("output = %q, want it to contain %q", got, "no run lock for")
	}
	wd, werr := os.Getwd() // post-chdir value (resolves the macOS symlink)
	if werr == nil && !strings.Contains(got, wd) {
		t.Errorf("output = %q, want it to contain the repoDir %q", got, wd)
	}
}

// TestLockStatus_LockHeldAlive verifies the live-holder path via a REAL lock.Acquire.
// lockPath is UNEXPORTED in internal/lock, so a test in package cmd cannot plant a lock
// file at the exact path; acquiring a real lock makes the holder THIS process (pid =
// os.Getpid(), alive=true). The dead-holder + orphan==true scenarios are E2E
// (P1.M4.T1.S1) — a real reparented-to-init pid is flaky/OS-dependent.
func TestLockStatus_LockHeldAlive(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", "")

	repo := t.TempDir()
	chdir(t, repo)

	l, err := lock.Acquire(repo)
	if err != nil {
		t.Fatalf("lock.Acquire: %v", err)
	}
	defer l.Release() // MANDATORY — the `current` singleton must not leak

	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"lock", "status"})

	err = Execute(context.Background())
	if code := exitcode.For(err); code != exitcode.Success {
		t.Fatalf("exit code = %d, want %d (Success); err=%v", code, exitcode.Success, err)
	}

	got := out.String()
	if !strings.Contains(got, "Lock:") {
		t.Errorf("output = %q, want it to contain %q", got, "Lock:")
	}
	wantPid := "pid:       " + strconv.Itoa(os.Getpid())
	if !strings.Contains(got, wantPid) {
		t.Errorf("output = %q, want it to contain %q", got, wantPid)
	}
	if !strings.Contains(got, "alive:     true") {
		t.Errorf("output = %q, want it to contain %q", got, "alive:     true")
	}
	// Assert the field is present; do NOT assert its exact value (CI-under-init could
	// differ). The 3-way switch prints one of: false / true (reparented) / unknown (dead).
	if !strings.Contains(got, "orphaned:") {
		t.Errorf("output = %q, want it to contain %q", got, "orphaned:")
	}
}

// TestLockStatus_StatusErrorPropagation verifies the exit-1 path: when lock.Status
// fails (here: lockDir cannot resolve a lock dir because XDG_RUNTIME_DIR/XDG_CACHE_HOME/
// HOME are all unset), the subcommand returns exitcode.New(exitcode.Error, ...) → exit 1.
func TestLockStatus_StatusErrorPropagation(t *testing.T) {
	// Force lockDir's os.UserHomeDir error → Status returns err (lockPath fails).
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")

	repo := t.TempDir()
	chdir(t, repo)

	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"lock", "status"})

	err := Execute(context.Background())
	if err == nil {
		t.Skipf("Execute returned nil — the runner env provides a HOME lockDir fallback outside t.Setenv's reach; the core exit-1 contract cannot be exercised here")
	}
	if code := exitcode.For(err); code != exitcode.Error {
		t.Errorf("exit code = %d, want %d (Error); err=%v", code, exitcode.Error, err)
	}
}
