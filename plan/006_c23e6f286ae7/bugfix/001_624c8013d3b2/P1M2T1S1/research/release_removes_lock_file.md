# Research: remove lock file on Release + cleanup unit test (bugfix Issue 2, P1.M2.T1.S1)

Verified against the live codebase. Source of truth for the one-method fix + the test.

## The bug (Issue 2, Minor)

`Release()` (`internal/lock/lock.go`, currently ~L108-117) closes the fd + clears the singleton but
**never `os.Remove`s the file**. Every distinct repo path ever visited leaves a permanent
`<hash>.lock` in `$XDG_RUNTIME_DIR/stagecoach/locks/` (210 observed during QA). flock auto-releases on
fd close, so the leftovers are inert empty shells, but they grow without bound.

## Current Release()

```go
func (l *Locker) Release() {
	if l == nil || l.file == nil {
		return
	}
	l.file.Close()
	l.file = nil
	if current.Load() == l {
		current.Store(nil)
	}
}
```

## The fix (PRD option b — the conventional flock pattern)

```go
func (l *Locker) Release() {
	if l == nil || l.file == nil {
		return
	}
	l.file.Close()            // release the flock FIRST (auto-release on fd close)
	path := l.path            // capture (l.path is reused by os.Remove; left untouched on the struct)
	l.file = nil
	if current.Load() == l {
		current.Store(nil)
	}
	os.Remove(path)           // best-effort; ignore the error
}
```

Add `"os"` is already imported. No new imports.

## CRITICAL ordering — close FIRST, then remove

`l.file.Close()` MUST precede `os.Remove(path)`. Rationale: close releases the advisory flock on the
inode; only then is the path unlinked. The inverted order (remove-while-held) is a real safety bug:
unlinking the path while still holding the flock on inode A lets a contender `OpenFile(path, O_CREATE)`
create a **new** inode B and flock it (B is free) → both processes believe they hold the FR52 lock → two
concurrent stagecoach runs on the same repo. Close-then-remove guarantees the holder has released before
the path is unlinked, so the next `Acquire` recreates a fresh file/inode and flocks it cleanly.

`os.Remove` errors are IGNORED: the file may already be gone (double Release is a no-op — the idempotency
guard returns at `l.file == nil` before reaching Remove, so the *second* Release doesn't even call it; but
a concurrent Acquire may also have recreated+removed it). Both harmless.

## Windows (lock_windows.go)

`flock` is a no-op on Windows (no POSIX flock; the §13.5 update-ref CAS is the real safety guarantee).
The remove is still correct there: the file is just a marker; `Acquire`'s `OpenFile(O_CREATE|O_RDWR)`
recreates it. The fix is in `lock.go` (shared), so NO build-tag changes — both `lock_unix.go` and
`lock_windows.go` are untouched.

## Re-creation on next Acquire (why this is safe)

`Acquire` (lock.go) does `os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)` → recreates the file if absent
(and `os.MkdirAll(filepath.Dir(path), 0o700)` recreates the dir). So removing on Release cannot break a
subsequent Acquire — verified by the existing `TestAcquire_Contention_HeldError` (Release → re-Acquire
succeeds) which stays green.

## Test approach (white-box, `package lock`)

- `lock_test.go` is `package lock` (white-box) → can read `l.path`, call `lockPath`/`lockHash`, and use the
  `resetCurrent(t)` helper (stores nil in the singleton on cleanup — prevents cross-test poisoning).
- Isolate the lock dir: `t.Setenv("XDG_RUNTIME_DIR", t.TempDir())` + `t.Setenv("XDG_CACHE_HOME", "")` so
  the test never touches the real `$XDG_RUNTIME_DIR/stagecoach/locks/` (deterministic + no pollution).
  (Several existing Acquire tests do NOT isolate XDG — a pre-existing hygiene gap; my new test DOES isolate,
  the cleaner pattern shown by the `TestLockDir_*` tests.)
- Assertions: Acquire → file exists at `l.path`; Release → `os.Stat(l.path)` satisfies `os.IsNotExist`;
  second Release is a no-op (no panic); a fresh `Acquire(repo)` succeeds (file recreated) and clean up.

## No existing test breaks

The file exists between Acquire and Release; it is removed ONLY on Release. Tests that read contents
(RoundTrip, SetSnapshot) do so before Release; tests that re-Acquire after Release (Contention) rely on
OpenFile(O_CREATE) recreating it. All 14 existing tests stay green.

## Scope boundary (no conflict)

- P1.M1.T2.S1 (parallel) adds an e2e scenario to `internal/e2e/lock_scenarios_test.go` — DIFFERENT file.
- P1.M2.T2.S1 (canonical repo=) touches `Acquire`; P1.M2.T3.S1 (atomic setSnapshot) touches `setSnapshot`;
  P1.M2.T4.S1 (contention guard) touches `handleLockContention`. All are DIFFERENT methods from `Release` →
  sequential, no merge conflict.
- This task: ONLY `internal/lock/lock.go` `Release()` (doc comment + 2 lines) + a new test in
  `internal/lock/lock_test.go`. No docs (P1.M3 owns the doc sweep), no production callers change.
