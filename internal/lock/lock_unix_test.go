//go:build !windows

package lock

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestProcessAlive_SelfAlive(t *testing.T) {
	host, _ := os.Hostname()
	if !processAlive(os.Getpid(), host) {
		t.Errorf("processAlive(self, thisHost) = false, want true (self is alive)")
	}
}

func TestProcessAlive_ForeignHostConservative(t *testing.T) {
	if !processAlive(os.Getpid(), "definitely-not-this-host-zzz-999") {
		t.Errorf("processAlive(self, foreignHost) = false, want true (foreign host → don't reap)")
	}
}

func TestProcessAlive_EmptyHostnameConservative(t *testing.T) {
	if !processAlive(os.Getpid(), "") {
		t.Errorf("processAlive(self, emptyHost) = false, want true (empty host → don't reap)")
	}
}

func TestProcessAlive_DeadPID(t *testing.T) {
	// Fork a child that exits immediately; after Wait its pid is dead → ESRCH → processAlive == false.
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot fork to obtain a dead pid (true not on PATH?): %v", err)
	}
	deadPID := cmd.Process.Pid
	_ = cmd.Wait() // child exits; pid is now free/dead
	host, _ := os.Hostname()
	// Negligible race: the OS could recycle the freed pid in the microsecond window (pids are assigned
	// sequentially, so this won't happen until the counter wraps). A real bug (e.g. always-true) fails
	// this deterministically.
	if processAlive(deadPID, host) {
		t.Errorf("processAlive(deadPID=%d, thisHost) = true, want false (ESRCH → dead → reapable)", deadPID)
	}
}

// writeLockFile writes a minimal lock file at path with the given pid/hostname —
// the two fields reapStaleLocks reads (via parseContents). The repo/timestamp/
// snapshot values are filler (parseContents reads them but reapStaleLocks
// ignores them). Used by the reaping tests to plant fixture orphan files in the
// exact key=value format parseContents reads.
func writeLockFile(t *testing.T, path, pid, hostname string) {
	t.Helper()
	content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=fake\ntimestamp=fake\nsnapshot=\n", pid, hostname)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// TestAcquire_ReapsDeadPidFile_SparesLive verifies §18.5 stale-FILE reaping
// (P1.M2.T1.S2): Acquire removes orphaned *.lock files whose recorded pid is
// DEAD, while SPARING live-pid files (this host), foreign-hostname files
// (conservative), malformed-pid files (Atoi-skip), and the just-acquired
// holder's own file. Each fixture pins one processAlive branch (P1.M2.T1.S1):
// dead=ESRCH→false; live=Kill-nil→true; foreign=hostname-mismatch→true;
// malformed=Atoi-error→continue. Unix-only (//go:build !windows) because the
// dead-pid-removed assertion requires Kill→ESRCH (Windows processAlive is
// always-true → the dead file would NOT be reaped → the assertion would fail
// Windows CI; mirrors S1's lock_unix_test.go placement for the processAlive
// dead-pid test).
func TestAcquire_ReapsDeadPidFile_SparesLive(t *testing.T) {
	resetCurrent(t)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // isolate — don't touch the real lock dir
	t.Setenv("XDG_CACHE_HOME", "")

	dir, err := lockDir()
	if err != nil {
		t.Fatalf("lockDir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil { // plant needs the dir first; Acquire's MkdirAll is then a no-op
		t.Fatalf("MkdirAll: %v", err)
	}

	thisHost, _ := os.Hostname()
	deadPath := filepath.Join(dir, "dead.lock")
	livePath := filepath.Join(dir, "live.lock")
	foreignPath := filepath.Join(dir, "foreign.lock")
	malformedPath := filepath.Join(dir, "malformed.lock")

	writeLockFile(t, deadPath, strconv.Itoa(math.MaxInt32), thisHost)                            // MaxInt32 ≫ pid_max → ESRCH → dead
	writeLockFile(t, livePath, strconv.Itoa(os.Getpid()), thisHost)                              // self → alive
	writeLockFile(t, foreignPath, strconv.Itoa(os.Getpid()), "definitely-not-this-host-zzz-999") // alive, foreign host
	writeLockFile(t, malformedPath, "not-a-number", thisHost)                                    // Atoi error → skip

	repo := t.TempDir()
	l, err := Acquire(repo) // creates <hash>.lock (holder's own, live → spared) + triggers reapStaleLocks(dir)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// (a) dead-pid file REAPED; live/foreign SPARED.
	if _, err := os.Stat(deadPath); !os.IsNotExist(err) {
		t.Errorf("dead-pid file should be REAPED (ESRCH), still present: %v", err)
	}
	if _, err := os.Stat(livePath); err != nil {
		t.Errorf("live-pid file should be SPARED (alive), missing: %v", err)
	}
	if _, err := os.Stat(foreignPath); err != nil {
		t.Errorf("foreign-hostname file should be SPARED (conservative), missing: %v", err)
	}
	// (b) malformed-pid file SKIPPED (Atoi error → continue, not reaped).
	if _, err := os.Stat(malformedPath); err != nil {
		t.Errorf("malformed-pid file should be SKIPPED (best-effort), missing: %v", err)
	}
	// The holder's own file is SPARED (its pid is os.Getpid, set by Acquire). Assert BEFORE Release
	// (Issue 2 removes l.path on Release).
	if _, err := os.Stat(l.path); err != nil {
		t.Errorf("holder's own lock file should be PRESENT, missing: %v", err)
	}

	l.Release()
}

// TestAppearsOrphaned_DeadPidIsConservativeFalse pins the conservative-false
// contract: a dead/gone pid (proc missing on Linux, ps non-zero exit on Darwin)
// must NOT be claimed as an orphan — a false-positive orphan claim could prompt
// the user to kill a legitimately-parented run. The only `true` is ppid == 1.
// (Unix-only //go:build !windows — orphan_windows.go is the always-false twin.)
func TestAppearsOrphaned_DeadPidIsConservativeFalse(t *testing.T) {
	// Fork a child that exits immediately; after Wait its pid is dead.
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot fork to obtain a dead pid (true not on PATH?): %v", err)
	}
	deadPID := cmd.Process.Pid
	_ = cmd.Wait() // child exits; pid is now free/dead
	if appearsOrphaned(deadPID) {
		t.Errorf("appearsOrphaned(deadPID=%d) = true, want false (proc gone / ps non-zero → conservative false)", deadPID)
	}
}

// TestAppearsOrphaned_SelfIsNotOrphan pins the common dev case: in a normal
// shell the test's parent is not init (ppid != 1), so appearsOrphaned(self) is
// false. NOTE: CI running directly under init (ppid==1) could make this true —
// orphan detection is a heuristic; the orphan==true path is proven by the E2E
// harness (P1.M4.T1.S1), not pinned here.
func TestAppearsOrphaned_SelfIsNotOrphan(t *testing.T) {
	self := os.Getpid()
	if appearsOrphaned(self) {
		// In a normal dev shell ppid != 1 → false. Under init (some CI) this could
		// legitimately be true; treat a true as a skip rather than a hard failure.
		t.Skipf("appearsOrphaned(self=%d) = true (ppid==1 — test runner is a child of init; heuristic, not a bug)", self)
	}
}

// TestStatus_NoLockFile verifies the path==” contract: with no lock file
// present, Status returns ("", zero contents, false, false, nil). FR-K4's "no
// run lock for <repo>" case. Status does NOT touch the `current` singleton, so
// resetCurrent is not needed.
func TestStatus_NoLockFile(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // isolate — don't touch the real lock dir
	t.Setenv("XDG_CACHE_HOME", "")
	repo := t.TempDir()

	path, contents, alive, orphan, err := Status(repo)
	if err != nil {
		t.Fatalf("Status(no lock) err = %v, want nil", err)
	}
	if path != "" {
		t.Errorf("Status(no lock) path = %q, want \"\"", path)
	}
	if contents != (LockContents{}) {
		t.Errorf("Status(no lock) contents = %+v, want zero LockContents", contents)
	}
	if alive {
		t.Errorf("Status(no lock) alive = true, want false")
	}
	if orphan {
		t.Errorf("Status(no lock) orphan = true, want false")
	}
}

// TestStatus_PlantedSelfLock verifies the alive path: a planted lock file holding
// our own pid yields the path + parsed contents + alive==true, with orphan set
// to appearsOrphaned(self). Compares Pid/Hostname (set verbatim by writeLockFile)
// rather than Repo (canonical-path-dependent — macOS t.TempDir() is under
// /var → /private/var). Does NOT Acquire (Status reads the file directly).
func TestStatus_PlantedSelfLock(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // isolate
	t.Setenv("XDG_CACHE_HOME", "")
	repo := t.TempDir()

	path, err := lockPath(repo)
	if err != nil {
		t.Fatalf("lockPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	thisHost, _ := os.Hostname()
	selfPID := strconv.Itoa(os.Getpid())
	writeLockFile(t, path, selfPID, thisHost)

	path2, contents, alive, orphan, err := Status(repo)
	if err != nil {
		t.Fatalf("Status(self lock) err = %v, want nil", err)
	}
	if path2 != path {
		t.Errorf("Status path = %q, want %q", path2, path)
	}
	if contents.Pid != selfPID {
		t.Errorf("contents.Pid = %q, want %q", contents.Pid, selfPID)
	}
	if contents.Hostname != thisHost {
		t.Errorf("contents.Hostname = %q, want %q", contents.Hostname, thisHost)
	}
	if !alive {
		t.Errorf("alive = false, want true (self pid is live on this host)")
	}
	if wantOrphan := appearsOrphaned(os.Getpid()); orphan != wantOrphan {
		t.Errorf("orphan = %v, want %v (appearsOrphaned(self))", orphan, wantOrphan)
	}
}

// TestStatus_MalformedPid verifies the malformed-pid branch: parseContents still
// returns the (garbage) pid string, but strconv.Atoi fails, so Status returns the
// path + contents (diagnostic value) with alive/orphan false (can't assess
// liveness without a parseable pid). Mirrors reapStaleLocks's Atoi-error skip.
func TestStatus_MalformedPid(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // isolate
	t.Setenv("XDG_CACHE_HOME", "")
	repo := t.TempDir()

	path, err := lockPath(repo)
	if err != nil {
		t.Fatalf("lockPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeLockFile(t, path, "not-a-number", "some-host")

	path2, contents, alive, orphan, err := Status(repo)
	if err != nil {
		t.Fatalf("Status(malformed pid) err = %v, want nil", err)
	}
	if path2 != path {
		t.Errorf("Status path = %q, want %q (path is still useful even with a malformed pid)", path2, path)
	}
	if contents.Pid != "not-a-number" {
		t.Errorf("contents.Pid = %q, want %q", contents.Pid, "not-a-number")
	}
	if alive {
		t.Errorf("alive = true, want false (malformed pid → can't assess liveness)")
	}
	if orphan {
		t.Errorf("orphan = true, want false (alive is false → orphan not assessed)")
	}
}

// TestStatus_LockPathError verifies the lockPath-error branch: when lockDir
// cannot resolve (no XDG + no HOME, no CWD fallback), Status propagates the
// error via err rather than misreporting "no lock".
func TestStatus_LockPathError(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "") // on most systems, os.UserHomeDir() fails with no HOME
	repo := t.TempDir()

	_, _, _, _, err := Status(repo)
	if err == nil {
		t.Error("Status(lockDir unresolved) err = nil, want non-nil (lockPath error must propagate)")
	}
}

// TestAcquire_ReapingIdempotent verifies contract (c): a second Acquire on the
// same repo (after Release) with no new dead files does NOT re-reap anything —
// the surviving fixtures (live/foreign/malformed) are stable across two Acquire
// passes. Reaping runs on every Acquire; this pins that it is a no-op on the
// survivor set when nothing new has died. (Unix-only for cohesion with the
// dead-pid test; the outcome is cross-platform-safe but reaping is a Unix
// concept — Windows is a documented no-op.)
func TestAcquire_ReapingIdempotent(t *testing.T) {
	resetCurrent(t)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", "")

	dir, err := lockDir()
	if err != nil {
		t.Fatalf("lockDir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	thisHost, _ := os.Hostname()
	survivors := []string{
		filepath.Join(dir, "live.lock"),
		filepath.Join(dir, "foreign.lock"),
		filepath.Join(dir, "malformed.lock"),
	}
	writeLockFile(t, survivors[0], strconv.Itoa(os.Getpid()), thisHost)
	writeLockFile(t, survivors[1], strconv.Itoa(os.Getpid()), "definitely-not-this-host-zzz-999")
	writeLockFile(t, survivors[2], "not-a-number", thisHost)
	// NOTE: no dead-pid file planted → the first Acquire reaps nothing.

	repo := t.TempDir()
	l1, err := Acquire(repo)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	l1.Release() // removes l1's own file; survivors untouched

	// Second Acquire — should reap nothing again (no dead file exists).
	l2, err := Acquire(repo)
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	defer l2.Release()

	for _, p := range survivors {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("idempotency: survivor %s was reaped on the second Acquire (should be stable): %v", p, err)
		}
	}
}
