# §18.5 Stale Lock-File Reaping & Exit-Path Release — Architecture

## Current State

`internal/lock/lock.go` implements FR52 per-repo run lock:
- Advisory `flock(LOCK_EX|LOCK_NB)` — auto-releases on process death (fd close at teardown)
- Lock file: `XDG_RUNTIME_DIR/stagecoach/locks/<sha256hex>.lock` (or cache dir fallback)
- Contents: `pid=`, `hostname=`, `repo=`, `timestamp=`, `snapshot=`
- `Release()`: close fd (release flock) → `os.Remove(path)` → clear singleton
- `Acquire()`: open file → flock → write contents → store singleton

**Gap 1 — No stale-file reaping:** orphaned lock *files* (from `SIGKILL`, crash, or the
signal-rescue `os.Exit`) accumulate as unbounded disk litter. The codebase over-claims
"no stale locks" — the *lock* never goes stale (flock auto-releases), but the *file* does.

**Gap 2 — Signal path orphans the file:** `signal.handle()` calls `os.Exit(3)` or
`os.Exit(130/143)`, which skips `defer locker.Release()` in `default_action.go:67`.

## Fix 1: Stale-File Reaping in `Acquire`

After the holder's own `flock` succeeds, reap every `*.lock` in the lock directory whose
recorded `pid` is not a live process on its recorded `hostname`.

### `processAlive(pid int, hostname string) bool` — cross-platform

**Unix (`lock_unix.go`):**
```go
func processAlive(pid int, hostname string) bool {
    host, _ := os.Hostname()
    if hostname == "" || hostname != host { return true }  // foreign host → don't reap
    err := syscall.Kill(pid, 0)
    if err == nil { return true }           // alive
    if errors.Is(err, syscall.EPERM) { return true }  // alive, not ours
    return false  // ESRCH → dead
}
```

**Windows (`lock_windows.go`):**
Return `true` always (conservative: reap nothing). `flock` is a no-op on Windows, so there
is no inode-bound-flock hazard, and the §13.5 CAS is the guarantee. Matching the literal
"never reap a live pid" + "reaping is a no-op too" wording.

### `reapStaleLocks(dir string)`

```go
func reapStaleLocks(dir string) {
    matches, _ := filepath.Glob(filepath.Join(dir, "*.lock"))
    for _, f := range matches {
        data, err := os.ReadFile(f)
        if err != nil { continue }
        c := parseContents(data)
        pid, err := strconv.Atoi(c.Pid)
        if err != nil { continue }       // malformed → skip
        if !processAlive(pid, c.Hostname) {
            os.Remove(f)                  // dead pid → safe to unlink
        }
    }
}
```

Called in `Acquire` after `flock` succeeds (after `writeContents("")`, before/after `current.Store(l)`).
The holder's own file is never reaped: its pid is `os.Getpid()` which is live.

**Safety invariant:** a live pid is NEVER reaped. The pid-liveness check is precisely what
makes unlinking safe (unlinking a live holder's file lets a contender `O_CREATE` a fresh inode
and flock it free, defeating FR52). Reaping by age/timestamp is rejected.

## Fix 2: Exit-Path Lock Release via Signal Seam

### `lock.ReleaseCurrent()` — package-level (mirrors `SetSnapshot`)

```go
func ReleaseCurrent() {
    if l := current.Load(); l != nil { l.Release() }
}
```

Idempotent + nil-safe. Called from `signal.handle()` via the injected seam.

### `signal.Options.OnRescueExit func()` — new seam

Added to `Options` alongside existing `Kill`, `Exit`, `RescueFormat`, `Out`. Defaulted to
`func(){}` (no-op) in `Install` — preserves stdlib-only leaf purity.

Called in `handle()` immediately before BOTH exit branches:
- Before `h.opts.Exit(3)` (post-snapshot rescue)
- Before `h.opts.Exit(exitCodeForSignal(sig))` (pre-snapshot 130/143)

Both branches need coverage: the lock is acquired at `default_action.go:59` BEFORE the snapshot
is armed (deep in `CommitStaged`/`runPipeline`). A Ctrl-C in the pre-snapshot window hits the
130/143 branch and also orphans the file.

### Wiring in `cmd/stagecoach/main.go`

```go
signal.Install(ctx, signal.Options{
    RescueFormat: generate.FormatRescue,
    OnRescueExit: lock.ReleaseCurrent,  // NEW
    ...
})
```

`main` already imports `signal` and `generate`; adding `lock` creates no cycle (main is root).

## Doc-Comment Corrections (Mode A — rides with the code)

Three over-claims in `internal/lock/lock.go`:
- Line 2: package doc — "auto-released on process death (no stale locks)"
- Line 31: `Locker` doc — "closes — no stale locks"
- Line 67: `Acquire` doc — "flock auto-released on process death (no stale locks)"

Rewrite to lock-vs-file framing: the *lock* never goes stale; orphaned *files* are reaped by
pid-liveness; the signal path releases before exit.

## Out of Scope

- **Decompose `lock.SetSnapshot` gap:** decompose.go:577 calls only `signal.SetSnapshot`, not
  `lock.SetSnapshot`; the FR52 no-op fast path is single-commit-only. Not part of §18.5.
- **Shared/network filesystem:** lock is per-host; §13.5 CAS catches cross-host races.
