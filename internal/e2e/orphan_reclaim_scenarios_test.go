//go:build e2e && !windows

// orphan_reclaim_scenarios_test.go is the PRD §20.5 e2e regression net for the §9.27 orphaned-run
// lock reclamation machinery shipped in P1.M1–P1.M3: the parent-death watchdog (FR-K1), the
// SIGHUP-on-terminal-close rescue (FR-K3), the read-only `stagecoach lock status` diagnostic
// (FR-K4), and the `no_parent_watchdog` opt-out (FR-K6). Every "bug found in the wild"
// (lazygit-TUI-closed-without-killing, IDE-quit, detaching-terminal, orphaned-but-alive holder)
// becomes a scenario here.
//
// These tests spawn REAL stagecoach subprocesses and assert real cross-process flock + signal +
// reparenting behavior that the in-process §20.1 layer unit tests cannot reach:
//
//   - flock is inode-bound across real processes;
//   - SIGHUP is kernel-delivered;
//   - the parent-death watchdog depends on real reparenting (a ppid CHANGE, subreaper-safe).
//
// The file is Unix-only (//go:build e2e && !windows): SIGHUP does not exist on Windows, process
// reparenting/init does not exist on Windows, and the watchdog is a Windows no-op (FR-K7) — all
// already unit-tested per-OS. The reliable lock-status cases (no-lock/live/dead) are
// platform-agnostic in behavior but co-located for simplicity.
//
// Test-only; CONSUMES the landed machinery — adds NO production code. Reuses the harness
// primitives (buildStagecoach/buildStub/newRepo/runStagecoach/waitForMarker/writeStubConfig/
// stubEnv/headSHA/commitCount/statusPorcelain/seedCommit/writeFile/stageFile/runGit) and the
// stub's STAGECOACH_STUB_MARKER + STAGECOACH_STUB_SLEEP_MS blocking pattern (NO new binary). The
// small set of process-management + lock-path helpers below are FILE-LOCAL (additive; NOT
// promoted to harness_test.go — keeps the blast radius to one file).
package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// e2eCmd bundles a started stagecoach subprocess with its capture buffers and context cancel so
// waitForExit can read the captured output + map the exit code (mirrors e2eResult). It is the
// Start-only twin of runStagecoach: runStagecoach builds exec.CommandContext(60s), sets Dir/Env,
// wires stdout/stderr, and Runs to completion; e2eCmd does the SAME setup but returns after Start
// so the caller can send a signal (SIGHUP) or detect parent-death BEFORE collecting the exit.
type e2eCmd struct {
	*exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer
	cancel context.CancelFunc
}

// startStagecoach is runStagecoach MINUS the Run: it builds the exec.CommandContext (60s), sets
// cmd.Dir=repo, cmd.Env=env, wires stdout/stderr capture, calls Start(), and returns an *e2eCmd
// (cmd.Process.Pid ready). Caller does waitForExit(ec, timeout) or waitProcessGone(pid, timeout).
// The 60s context is cancelled in waitForExit's happy path OR via t.Cleanup if the test aborts
// early, so the test process never leaks a live ctx/process.
func startStagecoach(t *testing.T, bin, repo, cfg string, env []string, args ...string) *e2eCmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	allArgs := append([]string{"--config", cfg, "--no-color"}, args...)
	cmd := exec.CommandContext(ctx, bin, allArgs...)
	cmd.Dir = repo
	cmd.Env = env
	ec := &e2eCmd{Cmd: cmd, cancel: cancel}
	cmd.Stdout = &ec.stdout
	cmd.Stderr = &ec.stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start stagecoach: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait() // best-effort reap if the test aborts before waitForExit
	})
	return ec
}

// waitForExit waits for the started subprocess (with timeout) and returns e2eResult{Stdout,Stderr,
// ExitCode}. Maps (*exec.ExitError).ExitCode(); a context-deadline/other non-ExitError is a hang →
// t.Fatalf (a hang is a test failure). Cancels the 60s context (no leak) on the happy path.
func waitForExit(t *testing.T, ec *e2eCmd, timeout time.Duration) e2eResult {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- ec.Wait() }()
	select {
	case err := <-done:
		ec.cancel() // release the 60s context
		r := e2eResult{Stdout: ec.stdout.String(), Stderr: ec.stderr.String()}
		if err != nil {
			if ee := (*exec.ExitError)(nil); errors.As(err, &ee) {
				r.ExitCode = ee.ExitCode()
			} else {
				t.Fatalf("waitForExit: %v", err)
			}
		}
		return r
	case <-time.After(timeout):
		ec.cancel()
		t.Fatalf("waitForExit: process did not exit after %v (stdout=%q stderr=%q)",
			timeout, ec.stdout.String(), ec.stderr.String())
		return e2eResult{} // unreachable
	}
}

// waitProcessGone polls syscall.Kill(pid, 0) for ESRCH (process exited) up to timeout. For a
// reparented process the test process is NOT its parent (init/subreaper reaps it), so cmd.Wait()
// can't reap it and the exit code is unreadable — this is the exit detector. Fatalf on timeout.
func waitProcessGone(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if err == syscall.ESRCH {
			return // process gone
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("waitProcessGone: pid %d still alive after %v", pid, timeout)
}

// lockFilePath replicates lock.lockPath under an isolated XDG_RUNTIME_DIR: EvalSymlinks(repo)
// (→Abs on error), sha256, hex, join(XDG_RUNTIME_DIR, "stagecoach", "locks", hash+".lock"). The
// caller MUST t.Setenv("XDG_RUNTIME_DIR", tmpDir) + t.Setenv("XDG_CACHE_HOME", "") BEFORE stubEnv
// so the stagecoach subprocess agrees on the same lock dir (mirrors lock_unix_test.go). We do NOT
// import internal/lock — these are subprocess tests; the stagecoach binary IS the SUT.
func lockFilePath(repo string) string {
	canonical, err := filepath.EvalSymlinks(repo)
	if err != nil {
		canonical, _ = filepath.Abs(repo)
	}
	sum := sha256.Sum256([]byte(canonical))
	return filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "stagecoach", "locks",
		hex.EncodeToString(sum[:])+".lock")
}

// plantLockFile writes a *.lock with known pid/hostname/repo (format mirrors
// internal/lock/lock_unix_test.go writeLockFile + parseContents: pid=…\nhostname=…\nrepo=…
// \ntimestamp=…\nsnapshot=…\n). MkdirAll the dir (0o700); WriteFile 0o600. Used by scenario (c)
// Dead + Orphan. hostname MUST be THIS host (os.Hostname) — processAlive short-circuits to TRUE
// for a foreign/empty hostname (conservative — don't reap), which would mask a dead-pid plant.
func plantLockFile(t *testing.T, path, pid, hostname, repo string) {
	t.Helper()
	content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=%s\ntimestamp=2026-07-10T00:00:00Z\nsnapshot=\n",
		pid, hostname, repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("plantLockFile: mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("plantLockFile: write: %v", err)
	}
}

// ppidOf returns the parent pid of pid (Linux /proc/<pid>/status PPid:; else `ps -o ppid= -p <pid>`).
// It mirrors internal/lock/orphan_unix.go ppidOf EXACTLY (the same platform dispatch). Used by
// scenario (c) Orphan to verify ppid==1 (skip if subreaper-reparented, ppid≠1). We do NOT import
// internal/lock — subprocess test.
func ppidOf(pid int) (int, error) {
	if runtime.GOOS == "linux" {
		return ppidLinuxLocal(pid)
	}
	return ppidViaPsLocal(pid)
}

// ppidLinuxLocal reads the PPid: field from /proc/<pid>/status (local twin of orphan_unix.go).
func ppidLinuxLocal(pid int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "PPid:" {
			return strconv.Atoi(fields[1])
		}
	}
	return 0, fmt.Errorf("orphan: no PPid field for pid %d", pid)
}

// ppidViaPsLocal runs `ps -o ppid= -p <pid>` and parses the right-justified number.
func ppidViaPsLocal(pid int) (int, error) {
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// shQuote single-quotes s for safe interpolation into a POSIX sh -c script (escapes any embedded
// single quote via the standard '"'"' idiom). t.TempDir paths are usually space-free, but be safe.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// isolateLocks sets an isolated XDG_RUNTIME_DIR (t.TempDir) and clears XDG_CACHE_HOME so the
// stagecoach subprocess uses a per-test lock dir that lockFilePath computes identically. MUST be
// called BEFORE stubEnv (stubEnv = os.Environ() + knobs reads the env at call time).
func isolateLocks(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", "")
}

// thisHost returns this machine's hostname (for plantLockFile — processAlive short-circuits to
// TRUE for a foreign hostname). A read failure falls back to "" (conservative liveness).
func thisHost() string {
	h, _ := os.Hostname()
	return h
}

// TestE2EOrphanReclaim exercises the §9.27 reclamation machinery end-to-end: the parent-death
// watchdog self-exit (FR-K1), the SIGHUP-on-terminal-close rescue (FR-K3), and the
// no_parent_watchdog opt-out (FR-K6). See each subtest for its FR + rationale.
func TestE2EOrphanReclaim(t *testing.T) {
	bin := buildStagecoach(t)
	stub := buildStub(t)
	cfg := writeStubConfig(t, stub, "")

	// B_SIGHUPRescue — FR-K3 (SIGHUP-on-terminal-close rescue).
	//
	// The test IS stagecoach's parent, so the exit code IS readable. The snapshot is armed
	// (generate.go:242 signal.SetSnapshot BEFORE Execute at L335) once the stub's marker exists,
	// so SIGHUP routes through handle()'s POST-snapshot branch → exit 3 (NOT 129). The rescue path
	// ALWAYS calls OnRescueExit (=lock.ReleaseCurrent → os.Remove) → the lock FILE is removed.
	t.Run("B_SIGHUPRescue", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n")
		stageFile(t, repo, "a.txt")
		isolateLocks(t)
		lockPath := lockFilePath(repo)

		marker := t.TempDir() + "/ready.marker"
		env := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":      "feat: a",
			"STAGECOACH_STUB_MARKER":   marker,
			"STAGECOACH_STUB_SLEEP_MS": "3000", // stub blocks mid-generation; we SIGHUP during the sleep
		})

		ec := startStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
		seedHead := headSHA(t, repo)             // capture BEFORE generation
		waitForMarker(t, marker, 10*time.Second) // snapshot armed → exit 3 deterministic

		if err := syscall.Kill(ec.Process.Pid, syscall.SIGHUP); err != nil {
			t.Fatalf("SIGHUP stagecoach: %v", err)
		}

		res := waitForExit(t, ec, 10*time.Second)
		if res.ExitCode != 3 {
			t.Fatalf("exit = %d, want 3 (rescue — snapshot armed, NOT 129); stderr:\n%s",
				res.ExitCode, res.Stderr)
		}
		if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
			t.Errorf("lock file still present at %s after rescue (want removed by OnRescueExit); stat err=%v",
				lockPath, err)
		}
		if got := headSHA(t, repo); got != seedHead {
			t.Errorf("HEAD moved %q → %q (want unchanged — no commit landed)", seedHead, got)
		}
		if got := statusPorcelain(t, repo); !strings.Contains(got, "a.txt") {
			t.Errorf("index changed; status --porcelain = %q (want a.txt still staged)", got)
		}
	})

	// A_ParentDeathWatchdog — FR-K1 (parent-death watchdog self-exit). The HARDEST e2e scenario.
	//
	// A parent shell (`sh -c`) backgrounds stagecoach, writes its PID to a pidfile, `sleep`s ~1.5s
	// (past watchdog.Arm, ~tens of ms), then exits. After sh exits the stagecoach is reparented
	// (to init/a subreaper); the watchdog's getppid poll (≤1s latency) detects the ppid CHANGE →
	// signal.Trigger(SIGTERM) → rescue exit 3 + lock release. The MANDATORY `sleep 1.5` defeats the
	// documented prctl/getppid race: if sh exited BEFORE Arm ran, originalPpid would be captured as
	// init → no change ever → the watchdog would never fire.
	//
	// After reparenting the test is NOT stagecoach's parent (init/subreaper reaps it) → the exit
	// code is UNREADABLE. We assert OUTCOMES (process gone via waitProcessGone, lock removed, HEAD
	// + index unchanged), NOT the exit code.
	t.Run("A_ParentDeathWatchdog", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n")
		stageFile(t, repo, "a.txt")
		isolateLocks(t)
		lockPath := lockFilePath(repo)
		seedHead := headSHA(t, repo) // capture BEFORE the wrapper

		marker := t.TempDir() + "/ready.marker"
		pidfile := t.TempDir() + "/child.pid"
		env := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":      "feat: a",
			"STAGECOACH_STUB_MARKER":   marker,
			"STAGECOACH_STUB_SLEEP_MS": "5000", // long enough that the watchdog fires DURING the sleep
		})

		// The race-defeating wrapper: background stagecoach (child of sh), write its PID, STAY ALIVE
		// ~1.5s (past watchdog.Arm), then exit. Non-interactive `sh -c 'cmd &'` does NOT SIGHUP the
		// backgrounded job on exit (SIGHUP-on-exit is an interactive-shell/huponexit feature) → the
		// stagecoach is cleanly orphaned (reparented), not SIGHUP'd. No `disown`/`nohup` (bash-isms).
		script := fmt.Sprintf("%s --config %s --provider stub &\n"+ // background stagecoach (child of sh)
			"echo $! > %s\n"+ // write its PID to the pidfile
			"sleep 1.5\n", // STAY ALIVE ~1.5s (past watchdog.Arm ~tens of ms), then exit → reparent
			shQuote(bin), shQuote(cfg), shQuote(pidfile))
		wrapper := exec.Command("sh", "-c", script)
		wrapper.Env = env // STAGECOACH_STUB_* inherited by the backgrounded stagecoach
		wrapper.Dir = repo
		if err := wrapper.Start(); err != nil {
			t.Fatalf("start wrapper sh: %v", err)
		}
		if err := wrapper.Wait(); err != nil {
			// sh -c with no explicit exit returns the last command's status; tolerate non-zero.
			t.Logf("wrapper sh exited non-zero (usually harmless): %v", err)
		}

		// Confirm the stub reached generation (snapshot armed). If the marker is missing the wrapper
		// raced stagecoach's start — bump the sleep. (Normally the marker exists ~0.1s after start.)
		waitForMarker(t, marker, 10*time.Second)

		// Read stagecoach's PID (written by the wrapper).
		data, err := os.ReadFile(pidfile)
		if err != nil {
			t.Fatalf("read pidfile %s: %v", pidfile, err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			t.Fatalf("parse pid %q: %v", strings.TrimSpace(string(data)), err)
		}

		// Can't Wait() a reparented process → poll syscall.Kill(pid, 0) for ESRCH (process gone).
		waitProcessGone(t, pid, 15*time.Second)

		if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
			t.Errorf("lock file still present at %s after watchdog self-exit (want removed by rescue "+
				"OnRescueExit); stat err=%v", lockPath, err)
		}
		if got := headSHA(t, repo); got != seedHead {
			t.Errorf("HEAD moved %q → %q (want unchanged — no commit landed)", seedHead, got)
		}
		if got := statusPorcelain(t, repo); !strings.Contains(got, "a.txt") {
			t.Errorf("index changed; status --porcelain = %q (want a.txt still staged)", got)
		}
	})

	// D_NoParentWatchdogOptOut — FR-K6 (the opt-out suppresses the watchdog without affecting
	// SIGHUP/lock-status).
	//
	// Identical wrapper to A_ParentDeathWatchdog, but env ADDS STAGECOACH_NO_PARENT_WATCHDOG=1. With
	// the opt-out, watchdog.Arm is NEVER called (default_action.go gates Arm on !cfg.NoParentWatchdog)
	// → no prctl, no poll goroutine → reparenting is undetected → stagecoach runs the stub to
	// completion and COMMITS normally. HEAD advancement is the DISCRIMINATOR: with the watchdog ON
	// (scenario A) HEAD is unchanged; with it OFF (this scenario) HEAD advances to the stub's message.
	t.Run("D_NoParentWatchdogOptOut", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n")
		stageFile(t, repo, "a.txt")
		isolateLocks(t)

		marker := t.TempDir() + "/ready.marker"
		pidfile := t.TempDir() + "/child.pid"
		env := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":           "feat: a",
			"STAGECOACH_STUB_MARKER":        marker,
			"STAGECOACH_STUB_SLEEP_MS":      "3000",
			"STAGECOACH_NO_PARENT_WATCHDOG": "1", // FR-K6: opt-out → watchdog never arms
		})

		script := fmt.Sprintf("%s --config %s --provider stub &\necho $! > %s\nsleep 1.5\n",
			shQuote(bin), shQuote(cfg), shQuote(pidfile))
		wrapper := exec.Command("sh", "-c", script)
		wrapper.Env = env
		wrapper.Dir = repo
		if err := wrapper.Start(); err != nil {
			t.Fatalf("start wrapper sh: %v", err)
		}
		if err := wrapper.Wait(); err != nil {
			t.Logf("wrapper sh exited non-zero (usually harmless): %v", err)
		}

		data, err := os.ReadFile(pidfile)
		if err != nil {
			t.Fatalf("read pidfile %s: %v", pidfile, err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			t.Fatalf("parse pid %q: %v", strings.TrimSpace(string(data)), err)
		}

		// The stub sleeps 3s then commits then exits. Give a GENEROUS timeout so the process runs
		// PAST the ~2-3s watchdog window (if it were gone at that point, opt-out failed). The OUTCOME
		// assertion (commitCount==2) is the proof; do not rely on timing.
		waitProcessGone(t, pid, 15*time.Second)

		if n := commitCount(t, repo); n != 2 {
			t.Fatalf("commit count = %d, want 2 (seed + the stub's commit LANDED — the watchdog did "+
				"NOT fire)", n)
		}
		if msg := runGit(t, repo, "log", "-1", "--format=%s"); msg != "feat: a" {
			t.Errorf("HEAD subject = %q, want 'feat: a' (the stub's commit)", msg)
		}
	})
}

// TestE2ELockStatus exercises the read-only `stagecoach lock status` diagnostic (FR-K4) for the
// reliable cases (NoLock/Live/Dead) plus a BEST-EFFORT genuine-orphan case (Orphan). The orphan
// case is environment-dependent (appearsOrphaned returns true ONLY for ppid==1; under a subreaper
// a reparented process's ppid is the subreaper, not 1) → it verifies ppid==1 and t.Skip's
// otherwise (no flake).
func TestE2ELockStatus(t *testing.T) {
	bin := buildStagecoach(t)
	stub := buildStub(t)
	cfg := writeStubConfig(t, stub, "")

	// NoLock — no lock held → "no run lock for <repo>" (exit 0). lock status writes to STDOUT
	// (cmd.OutOrStdout); assert res.Stdout.
	t.Run("NoLock", func(t *testing.T) {
		repo := newRepo(t)
		isolateLocks(t)
		baseEnv := stubEnv(nil)
		res := runStagecoach(t, bin, repo, cfg, baseEnv, "lock", "status")
		if res.ExitCode != 0 {
			t.Fatalf("exit = %d, want 0 (no lock is a successful read); stderr:\n%s",
				res.ExitCode, res.Stderr)
		}
		if !strings.Contains(res.Stdout, "no run lock for") {
			t.Errorf("stdout missing 'no run lock for'; got:\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout, repo) {
			t.Errorf("stdout missing repo path %q; got:\n%s", repo, res.Stdout)
		}
	})

	// Live — a real holder (sleeping stub) holds the lock → alive:true + pid/hostname/Lock present.
	// Mirrors lock_scenarios_test.go's goroutine/marker/drain idiom: spawn the holder in a goroutine,
	// waitForMarker, run `lock status`, then <-resCh to drain the holder (release the lock).
	t.Run("Live", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n")
		stageFile(t, repo, "a.txt")
		isolateLocks(t)

		readiness := t.TempDir() + "/ready.marker"
		holderEnv := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":      "feat: a",
			"STAGECOACH_STUB_MARKER":   readiness,
			"STAGECOACH_STUB_SLEEP_MS": "3000", // hold the lock while we read its status
		})

		resCh := make(chan e2eResult, 1)
		go func() { resCh <- runStagecoach(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
		waitForMarker(t, readiness, 10*time.Second) // holder holds the lock

		res := runStagecoach(t, bin, repo, cfg, stubEnv(nil), "lock", "status")
		if res.ExitCode != 0 {
			t.Fatalf("exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if !strings.Contains(res.Stdout, "Lock:") {
			t.Errorf("stdout missing 'Lock:' line; got:\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout, "alive:     true") {
			t.Errorf("stdout missing 'alive:     true'; got:\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout, "pid:") {
			t.Errorf("stdout missing 'pid:' field; got:\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout, "hostname:") {
			t.Errorf("stdout missing 'hostname:' field; got:\n%s", res.Stdout)
		}
		// Assert the orphaned field is PRESENT; do NOT pin its value (CI-under-init could differ
		// from a developer workstation under a subreaper).
		if !strings.Contains(res.Stdout, "orphaned:") {
			t.Errorf("stdout missing 'orphaned:' field; got:\n%s", res.Stdout)
		}

		if res := <-resCh; res.ExitCode != 0 { // drain holder (lets its 3s sleep finish)
			t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
	})

	// Dead — a planted lock whose pid is GUARANTEED dead → alive:false + "orphaned:  unknown
	// (holder is dead)". The dead pid is captured via Start()+Wait() of `true` (reaped, dead) — NOT a
	// magic number (which could be recycled to a live process). hostname = THIS host so processAlive
	// actually runs (it short-circuits to TRUE for a foreign/empty hostname).
	t.Run("Dead", func(t *testing.T) {
		repo := newRepo(t)
		isolateLocks(t)

		dead := exec.Command("true")
		if err := dead.Start(); err != nil {
			t.Fatalf("start true: %v", err)
		}
		deadPid := dead.Process.Pid
		if err := dead.Wait(); err != nil {
			t.Fatalf("wait true: %v", err) // reaped → dead
		}

		lp := lockFilePath(repo)
		plantLockFile(t, lp, strconv.Itoa(deadPid), thisHost(), repo)

		res := runStagecoach(t, bin, repo, cfg, stubEnv(nil), "lock", "status")
		if res.ExitCode != 0 {
			t.Fatalf("exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if !strings.Contains(res.Stdout, "alive:     false") {
			t.Errorf("stdout missing 'alive:     false'; got:\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout, "orphaned:  unknown (holder is dead)") {
			t.Errorf("stdout missing 'orphaned:  unknown (holder is dead)'; got:\n%s", res.Stdout)
		}
	})

	// Orphan — BEST-EFFORT genuine ppid==1 holder → alive:true + orphaned:true. appearsOrphaned
	// returns true ONLY for ppid==1; under a subreaper (systemd, containers, some shells) a
	// reparented process's ppid is the subreaper, not 1 → false. We verify the produced holder's
	// ppid==1 and t.Skip otherwise (no flake). The RELIABLE core of scenario (c) is NoLock/Live/Dead.
	t.Run("Orphan", func(t *testing.T) {
		repo := newRepo(t)
		isolateLocks(t)

		// Produce a genuine ppid==1 holder: a sleep reparented to init when its spawning sh exits.
		// The sleep's stdout/stderr are redirected away from the capture pipe (>/dev/null 2>&1) so the
		// backgrounded grandchild does NOT hold the write end of Output()'s stdout pipe — otherwise
		// Output() blocks for the full sleep duration waiting for EOF (a classic orphaned-grandchild-
		// holds-stdout-pipe hang).
		orph := exec.Command("sh", "-c", "sleep 10 >/dev/null 2>&1 &\necho $!")
		out, err := orph.Output()
		if err != nil {
			t.Fatalf("spawn orphan holder: %v", err)
		}
		sleepPid, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			t.Fatalf("parse sleep pid %q: %v", strings.TrimSpace(string(out)), err)
		}
		t.Cleanup(func() { _ = syscall.Kill(sleepPid, syscall.SIGKILL) }) // never leak the sleep

		// Verify the holder was reparented to init (ppid==1). Under a subreaper it is NOT → skip.
		ppid, err := ppidOf(sleepPid)
		if err != nil {
			_ = syscall.Kill(sleepPid, syscall.SIGKILL)
			t.Skipf("orphan: cannot read ppid of %d (%v) — environment-dependent; skipping", sleepPid, err)
		}
		if ppid != 1 {
			_ = syscall.Kill(sleepPid, syscall.SIGKILL)
			t.Skipf("orphan: holder ppid=%d ≠ 1 (subreaper-reparented — appearsOrphaned is conservative); "+
				"skipping (not a failure)", ppid)
		}

		lp := lockFilePath(repo)
		plantLockFile(t, lp, strconv.Itoa(sleepPid), thisHost(), repo)

		res := runStagecoach(t, bin, repo, cfg, stubEnv(nil), "lock", "status")
		if res.ExitCode != 0 {
			t.Fatalf("exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if !strings.Contains(res.Stdout, "alive:     true") {
			t.Errorf("stdout missing 'alive:     true'; got:\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout, "orphaned:  true (holder reparented") {
			t.Errorf("stdout missing 'orphaned:  true (holder reparented…)'; got:\n%s", res.Stdout)
		}
	})
}
