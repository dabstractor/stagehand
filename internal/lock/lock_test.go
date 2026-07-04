package lock

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// resetCurrent stores nil in the package singleton when the test finishes.
// Prevents singleton poisoning between tests (especially under -race).
func resetCurrent(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { current.Store(nil) })
}

// TestLockDir_RuntimePreferred verifies XDG_RUNTIME_DIR takes precedence.
func TestLockDir_RuntimePreferred(t *testing.T) {
	tmpAbs := filepath.Join(t.TempDir(), "runtime")
	t.Setenv("XDG_RUNTIME_DIR", tmpAbs)
	t.Setenv("XDG_CACHE_HOME", "") // clear so it doesn't interfere

	dir, err := lockDir()
	if err != nil {
		t.Fatalf("lockDir: %v", err)
	}
	want := filepath.Join(tmpAbs, "stagehand", "locks")
	if dir != want {
		t.Errorf("lockDir = %q, want %q", dir, want)
	}
}

// TestLockDir_CacheFallback verifies XDG_CACHE_HOME is used when XDG_RUNTIME_DIR is unset.
func TestLockDir_CacheFallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	tmpAbs := filepath.Join(t.TempDir(), "cache")
	t.Setenv("XDG_CACHE_HOME", tmpAbs)

	dir, err := lockDir()
	if err != nil {
		t.Fatalf("lockDir: %v", err)
	}
	want := filepath.Join(tmpAbs, "stagehand", "locks")
	if dir != want {
		t.Errorf("lockDir = %q, want %q", dir, want)
	}
}

// TestLockDir_HomeFallback verifies ~/.cache/stagehand/locks when both XDG vars are unset.
func TestLockDir_HomeFallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dir, err := lockDir()
	if err != nil {
		t.Fatalf("lockDir: %v", err)
	}
	want := filepath.Join(tmpHome, ".cache", "stagehand", "locks")
	if dir != want {
		t.Errorf("lockDir = %q, want %q", dir, want)
	}
}

// TestLockDir_RejectedRelative verifies a relative XDG_RUNTIME_DIR is skipped.
func TestLockDir_RejectedRelative(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", "rel/path")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", tmpHome)

	dir, err := lockDir()
	if err != nil {
		t.Fatalf("lockDir: %v", err)
	}
	// Should fall through to home fallback, NOT use the relative path.
	want := filepath.Join(tmpHome, ".cache", "stagehand", "locks")
	if dir != want {
		t.Errorf("lockDir = %q, want %q (relative XDG_RUNTIME_DIR should be skipped)", dir, want)
	}
}

// TestLockDir_NoCwdFallbackError verifies lockDir returns an error when no XDG
// vars are set and UserHomeDir fails (NO CWD fallback).
func TestLockDir_NoCwdFallbackError(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	// On most systems, unsetting HOME makes os.UserHomeDir() fail.
	_, err := lockDir()
	if err == nil {
		t.Error("lockDir should return an error when no resolution path exists")
	}
}

// TestHash_CanonicalSymlink verifies two paths to the same repo (one a symlink)
// produce the same lock hash.
func TestHash_CanonicalSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	tmpRepo := filepath.Join(tmpDir, "repo")
	os.MkdirAll(tmpRepo, 0o755)

	tmpLink := filepath.Join(tmpDir, "link")
	os.Symlink(tmpRepo, tmpLink)

	hash1 := lockHash(tmpRepo)
	hash2 := lockHash(tmpLink)
	if hash1 != hash2 {
		t.Errorf("lockHash(symlink)=%q != lockHash(real)=%q", hash2, hash1)
	}
	// Determinism: same path → same hash always.
	hash3 := lockHash(tmpRepo)
	if hash1 != hash3 {
		t.Errorf("lockHash not deterministic: %q != %q", hash1, hash3)
	}
}

// TestAcquireRelease_RoundTrip verifies Acquire creates the lock file with
// correct contents and Release is idempotent.
func TestAcquireRelease_RoundTrip(t *testing.T) {
	resetCurrent(t)
	repo := t.TempDir()

	l, err := Acquire(repo)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// File exists with correct contents.
	data, err := os.ReadFile(l.path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	c := parseContents(data)
	if c.Pid == "" {
		t.Error("pid is empty")
	}
	if c.Hostname == "" {
		t.Error("hostname is empty")
	}
	if c.Repo != repo {
		t.Errorf("repo = %q, want %q", c.Repo, repo)
	}
	if c.Timestamp == "" {
		t.Error("timestamp is empty")
	}
	if c.Snapshot != "" {
		t.Errorf("snapshot = %q, want empty", c.Snapshot)
	}

	// Release is idempotent.
	l.Release()
	l.Release() // second call must not panic
}

// TestSetSnapshot_UpdatesFile verifies SetSnapshot rewrites the snapshot= line.
func TestSetSnapshot_UpdatesFile(t *testing.T) {
	resetCurrent(t)
	repo := t.TempDir()

	l, err := Acquire(repo)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer l.Release()

	SetSnapshot("abc123")

	data, err := os.ReadFile(l.path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	c := parseContents(data)
	if c.Snapshot != "abc123" {
		t.Errorf("snapshot = %q, want %q", c.Snapshot, "abc123")
	}
	if c.Pid == "" {
		t.Error("pid was cleared by SetSnapshot")
	}
	if c.Repo != repo {
		t.Errorf("repo changed: %q, want %q", c.Repo, repo)
	}
}

// TestSetSnapshot_NilSafeNoOp verifies the package-level SetSnapshot is a
// no-op when no lock is held.
func TestSetSnapshot_NilSafeNoOp(t *testing.T) {
	resetCurrent(t)
	current.Store(nil)
	// Must not panic.
	SetSnapshot("should-be-noop")
}

// TestSetSnapshot_MethodAfterRelease verifies SetSnapshot is a no-op after Release.
func TestSetSnapshot_MethodAfterRelease(t *testing.T) {
	resetCurrent(t)
	repo := t.TempDir()

	l, err := Acquire(repo)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	l.Release()
	// Must not panic.
	l.SetSnapshot("after-release-noop")
}

// TestAcquire_Contention_HeldError verifies that a second Acquire on the same
// repo returns *HeldError with the holder's parsed contents, and that after
// Release a third Acquire succeeds (auto-release on close).
func TestAcquire_Contention_HeldError(t *testing.T) {
	resetCurrent(t)
	repo := t.TempDir()

	l1, err := Acquire(repo)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}

	var l2 *Locker
	var l2err error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		l2, l2err = Acquire(repo)
	}()
	wg.Wait()

	if l2 != nil {
		t.Error("second Acquire should return nil Locker on contention")
		l2.Release()
	}
	if l2err == nil {
		t.Fatal("second Acquire should return an error on contention")
	}

	var he *HeldError
	if !errors.As(l2err, &he) {
		t.Fatalf("second Acquire error type = %T, want *HeldError", l2err)
	}
	if he.Contents.Pid != l1.pid {
		t.Errorf("HeldError.Pid = %q, want %q", he.Contents.Pid, l1.pid)
	}
	if he.Path != l1.path {
		t.Errorf("HeldError.Path = %q, want %q", he.Path, l1.path)
	}

	// Release and re-acquire should succeed.
	l1.Release()
	l3, err := Acquire(repo)
	if err != nil {
		t.Fatalf("third Acquire after Release: %v", err)
	}
	l3.Release()
}

// TestIsHeldError verifies the IsHeldError helper.
func TestIsHeldError(t *testing.T) {
	if IsHeldError(nil) {
		t.Error("IsHeldError(nil) = true, want false")
	}
	he := &HeldError{Contents: LockContents{Pid: "42"}, Path: "/tmp/x.lock"}
	if !IsHeldError(he) {
		t.Error("IsHeldError(*HeldError) = false, want true")
	}
}
