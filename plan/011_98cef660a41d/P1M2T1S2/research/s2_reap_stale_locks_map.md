# S2 Verified Touchpoint Map — reapStaleLocks + Acquire wiring + doc fixes (P1.M2.T1.S2)

> Verified against the LIVE repo (module github.com/dustin/stagehand, 2026-07-07). Research only.
> S1 (processAlive) is NOT yet in the live file (still Implementing) — S2 treats S1's PRP as a HARD
> dependency. lock.go itself is fully verified.

## 1. S1 (processAlive) is the hard dependency — NOT yet applied

`grep -rn "func processAlive" internal/lock/` returns EMPTY today. S1 (P1.M2.T1.S1) adds processAlive to
lock_unix.go (!windows) + lock_windows.go (windows). S2's reapStaleLocks CALLS `processAlive(pid, c.Hostname)`,
so **S2 will not compile until S1 lands**. Assume S1 delivers exactly: Unix `processAlive(pid, hostname) bool`
(hostname empty/mismatch → true; Kill(pid,0) nil → true; EPERM → true; else false) + Windows always-true stub.
Do NOT re-implement processAlive in S2 — call it.

## 2. reapStaleLocks — zero new imports (lock.go already has os/path/filepath/strconv)

lock.go's import block: bufio, crypto/sha256, encoding/hex, errors, fmt, **os**, **path/filepath**,
**strconv**, strings, sync/atomic, time. reapStaleLocks uses `filepath.Glob`, `filepath.Join`, `os.ReadFile`,
`os.Remove`, `strconv.Atoi` — ALL already imported. ZERO new imports. go.mod unchanged.

## 3. The reapStaleLocks body (lock_reaping.md "Fix 1" — port VERBATIM)

```go
// reapStaleLocks removes every *.lock file in dir whose recorded pid is not a live process on its
// recorded hostname (PRD §18.5 stale-FILE reaping). Called from Acquire AFTER the holder's own flock
// succeeds — the holder's pid is os.Getpid() (live) so its own file is never reaped. SAFETY INVARIANT:
// a LIVE pid is NEVER reaped (processAlive is conservative on every ambiguity) — unlinking a live
// holder's inode-bound-flock file would let a contender O_CREATE a fresh inode and flock it, defeating
// FR52. Only a DEAD pid (no open fd → no flock) is safe to unlink. Malformed/empty pid → skip (best-effort).
// Errors are ignored throughout (reaping is best-effort disk hygiene; a failed ReadFile/Remove is a no-op).
func reapStaleLocks(dir string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.lock"))
	for _, f := range matches {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		c := parseContents(data)
		pid, err := strconv.Atoi(c.Pid)
		if err != nil {
			continue // malformed/empty pid → skip
		}
		if !processAlive(pid, c.Hostname) {
			os.Remove(f) // dead pid → safe to unlink (ignore error)
		}
	}
}
```

## 4. The Acquire wiring — AFTER writeContents("") + current.Store(l)

In Acquire (lock.go:71-109), the holder's own setup ends with:
```go
	l.writeContents("")
	current.Store(l)
	reapStaleLocks(filepath.Dir(path))   // ← ADD HERE (after current.Store(l), before return l, nil)
	return l, nil
```
WHY `filepath.Dir(path)` (NOT a fresh lockDir() call): `path` is already the resolved lock file path
(lockDir() + hash + ".lock"), so `filepath.Dir(path)` is EXACTLY the directory the acquired file lives in.
Reusing the already-resolved path's dir guarantees the glob and the acquired file share the same directory
(no resolution discrepancy if env vars shifted between calls). The holder's own file is in `matches` but its
pid is os.Getpid() → processAlive true → not reaped.

## 5. The 3 over-claim doc fixes (lock.go:2, 31, 67 — confirmed exact lines)

`grep -n "no stale locks" internal/lock/lock.go` → lines 2, 31, 67. Each says "(no stale locks)" — the
over-claim. Rewrite to the lock-vs-FILE framing:
- **Line 2** (package doc): `// flock(LOCK_EX|LOCK_NB) auto-released on process death — the LOCK never goes
  stale; orphaned FILES (SIGKILL/crash/signal-rescue os.Exit) are reaped by pid-liveness on Acquire.`
- **Line 31** (Locker doc): `// closes — the LOCK never goes stale (flock auto-releases); orphaned FILES are
  reaped by pid-liveness on the next Acquire. Create via Acquire; ...`
- **Line 67** (Acquire doc): `// flock auto-released on process death — the LOCK never goes stale; after
  taking its own flock, Acquire reaps orphaned *.lock FILES whose recorded pid is dead (the holder's own
  live-pid file survives).`

## 6. docs/how-it-works.md lines 170 + 179 (confirmed exact lines)

- **Line 170**: ends `...auto-releases on process death (SIGKILL, crash, power loss) — no stale-lock reaping
  or PID-liveness checks needed.` → replace the tail with: `...auto-releases on process death (SIGKILL,
  crash, power loss) — the LOCK never goes stale. Orphaned lock FILES (left by exits that bypass the deferred
  cleanup) are reaped by pid-liveness on the next Acquire, and the signal path releases the file before exiting.`
- **Line 179** (`**Auto-release.**` para): `No stale locks, no PID-liveness checks, no reaping.` → rewrite to:
  the LOCK is never stale (flock auto-releases); the lock FILE is orphaned by exits that bypass cleanup, reaped
  by pid-liveness on the next Acquire (kill(pid,0)→ESRCH), and the signal path releases before exit. Windows:
  flock is a no-op stub, reaping is a no-op too, §13.5 CAS is the guarantee.

## 7. Test scope — P1.M2.T3.S1 owns the reaping tests (NOT S2)

The plan splits: P1.M2.T1.S2 = function + wiring + doc fixes (NO tests); P1.M2.T3.S1 = "Stale-file reaping
tests (lock_test.go)". S2 does NOT add committed tests. The `unused` lint is satisfied: reapStaleLocks is
called from Acquire (same package) → it is "used". processAlive is called from reapStaleLocks + S1's tests.
S2's Validation Loop includes a THROWAWAY (non-committed) reap-sanity check (Level 3) to confirm the wiring
without overlapping P1.M2.T3.S1's committed-test scope.

## 8. parseContents + the holder-safety reasoning (verified)

parseContents (lock.go:211-239) parses key=value lines into LockContents{Pid, Hostname, Repo, Timestamp,
Snapshot}; malformed lines are skipped. reapStaleLocks reads c.Pid (string) → strconv.Atoi → processAlive.
The holder's own file (just written by writeContents) has pid=os.Getpid() → processAlive true → survives.
A live contender's file → its pid alive → survives. Only dead-pid orphans (SIGKILL/crash/signal-rescue)
are reaped. This is the FR52 "never force-break" + "never reap a live pid" invariant.
