# Research: Stale-file reaping tests — fixture design, cross-platform placement, the 3 contract scenarios

> Subtask P1.M2.T3.S1 (test-only). Verifies the §18.5 stale lock-FILE reaping landed by P1.M2.T1.S2
> (`reapStaleLocks` + the Acquire wiring) on top of P1.M2.T1.S1 (`processAlive`). The production code is
> ALREADY IN THE TREE (read `internal/lock/lock.go`); this task adds ONLY tests.

---

## 1. The production code under test (LANDED — read-only)

`internal/lock/lock.go`:
```go
// Acquire (after flock succeeds + writeContents + current.Store):
reapStaleLocks(filepath.Dir(path))   // §18.5: reap orphaned *.lock files whose pid is dead

func reapStaleLocks(dir string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.lock"))
	for _, f := range matches {
		data, err := os.ReadFile(f)
		if err != nil { continue }
		c := parseContents(data)
		pid, err := strconv.Atoi(c.Pid)
		if err != nil { continue }              // malformed/empty pid → skip (best-effort)
		if !processAlive(pid, c.Hostname) {
			os.Remove(f)                        // dead pid → unlink (ignore error)
		}
	}
}
```
`parseContents` reads `pid=`/`hostname=`/`repo=`/`timestamp=`/`snapshot=` lines (key=value, `\n`-separated;
the `writeContents` format is `pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n`). reapStaleLocks
uses ONLY `c.Pid` + `c.Hostname`.

`processAlive(pid int, hostname string) bool` — **the cross-platform keystone** (S1):
- **Unix** (`lock_unix.go`, `//go:build !windows`): `hostname=="" || hostname != os.Hostname()` → **true**
  (foreign host, don't reap); `Kill(pid,0)==nil` → **true** (alive); `EPERM` → **true** (alive, other user);
  else → **false** (ESRCH → dead → reapable).
- **Windows** (`lock_windows.go`, `//go:build windows`): `return true` **always** (flock is a no-op there →
  reaping is a documented no-op; the §13.5 CAS is the guarantee).

## 2. THE cross-platform placement decision — reaping tests go in lock_unix_test.go (NOT lock_test.go)

The contract says "Add tests in internal/lock/lock_test.go." But the headline assertion — **"the dead-pid
file is removed"** — is **Unix-specific**: on Windows `processAlive` always returns `true`, so the dead-pid
file is NOT reaped, and the assertion FAILS on Windows CI (which runs `go test`). This is the IDENTICAL
issue S1 resolved for the `processAlive` dead-pid unit test: S1 put it in **`lock_unix_test.go`
(`//go:build !windows`)**, not lock_test.go (S1 PRP: "the dead-pid test goes in lock_unix_test.go … on
Windows processAlive always returns true, so a no-build-tag dead-pid test would FAIL on Windows").

**Decision: put BOTH reaping tests in `lock_unix_test.go` (`//go:build !windows`)** — the existing home of
the processAlive tests they exercise. Rationale:
1. **CI-correct** — the dead-pid-removed assertion requires Unix `Kill`→ESRCH; a lock_test.go placement
   breaks Windows CI. This OVERRIDES the contract's file suggestion (a hard correctness requirement).
2. **S1-consistent** — lock_unix_test.go is the established home for Unix-specific dead-pid assertions.
3. **Cohesion** — the reaping tests + the processAlive tests they depend on live in one file; reaping is a
   Unix concept (Windows is a documented no-op, so there's no meaningful reaping behavior to test there).

The contract's helpers/pattern are still REUSED: `resetCurrent(t)` (in lock_test.go, visible to
lock_unix_test.go on Unix where both compile) + the XDG-isolation pattern (`t.Setenv("XDG_RUNTIME_DIR",
t.TempDir())` + `t.Setenv("XDG_CACHE_HOME","")`, from TestRelease_RemovesLockFile). Only the placement
moves; the test DESIGN is exactly the contract's.

## 3. Planting fake lock files — the lock dir + the format

To make reapStaleLocks see the fakes, they must be `*.lock` files in the SAME dir Acquire reaps
(`filepath.Dir(path)` = the resolved lock dir). Sequence:
1. Isolate XDG: `t.Setenv("XDG_RUNTIME_DIR", t.TempDir())` + `t.Setenv("XDG_CACHE_HOME", "")`.
2. Resolve the dir via the package's own `lockDir()` (white-box): `dir, _ := lockDir()` → `<tmpdir>/stagehand/locks`.
3. `os.MkdirAll(dir, 0o700)` (Acquire's own MkdirAll is then a no-op; planting needs the dir to exist first).
4. Write each fake via a helper:
```go
func writeLockFile(t *testing.T, path, pid, hostname string) {
	t.Helper()
	content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=fake\ntimestamp=fake\nsnapshot=\n", pid, hostname)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}
```
5. `Acquire(repoPath)` → creates `<repoHash>.lock` (the holder's own, live pid → spared) + triggers
   `reapStaleLocks(dir)` which globs ALL `*.lock` (the 4 fakes + the holder's own).

**Filename collision is impossible**: the holder's own file is `<64-hex-sha256>.lock`; the fakes are
`dead.lock`/`live.lock`/`foreign.lock`/`malformed.lock` — distinct names.

## 4. The fixtures (the 4 cases + the holder's own)

| Fixture | pid | hostname | processAlive (Unix) | Expected after Acquire |
|---|---|---|---|---|
| `dead.lock` | `math.MaxInt32` (2147483647) | this host | `Kill(MaxInt32,0)`→ESRCH→**false** | **REAPED** (removed) |
| `live.lock` | `os.Getpid()` | this host | `Kill`→nil→**true** | SPARED (present) |
| `foreign.lock` | `os.Getpid()` | `"definitely-not-this-host-zzz"` | hostname mismatch→**true** | SPARED (present) |
| `malformed.lock` | `"not-a-number"` | this host | `Atoi` error→`continue` (skip) | SPARED (present) |
| holder's own `<hash>.lock` | `os.Getpid()` (set by Acquire) | this host | **true** | SPARED (present) |

**Why `math.MaxInt32` is a guaranteed-dead pid:** pid_t is int32 on Linux/macOS; pid_max is ≤ 4194304
(Linux) or far lower (macOS). 2147483647 is way above any pid ever assigned → `kill(2147483647, 0)` →
ESRCH → dead. `strconv.Atoi("2147483647")` succeeds (fits in int on both 32- and 64-bit). Simpler than
S1's fork+wait (no child process to manage) and the contract specifies it. (This is the reaping-test
fixture; S1's processAlive unit test used fork+wait — both valid; the contract chose MaxInt32 here.)

**`thisHost`**: `host, _ := os.Hostname()` (matches what `processAlive` compares against).

## 5. The 3 contract scenarios → 2 tests

**(a) + (b) → `TestAcquire_ReapsDeadPidFile_SparesLive`** (lock_unix_test.go): plant all 4 fakes → Acquire →
assert dead.lock GONE; live.lock/foreign.lock/malformed.lock + the holder's own l.path PRESENT. Covers (a)
the 3-way seed + dead-removed/live+foreign+own-spared, AND (b) malformed-pid-skipped.

**(c) → `TestAcquire_ReapingIdempotent`** (lock_unix_test.go): plant ONLY survivors (live/foreign/malformed,
no dead) → Acquire + Release → snapshot the survivor set → Acquire again → assert all 3 survivors STILL
PRESENT (nothing re-reaped). Proves reaping is idempotent (a second Acquire with no new dead files is a
no-op on the survivors). (The first Acquire reaps nothing because there's no dead file; the second reaps
nothing again — the survivors are stable across both passes.)

## 6. Read-before-Release (Issue 2 composition)

`Release()` removes the holder's OWN lock file (Issue 2, landed). The assertions on `l.path` (the holder's
own file present) must happen BEFORE `l.Release()`. The fake-file assertions (dead/live/foreign/malformed)
are unaffected by Release (Release only removes `l.path`). So: assert everything, THEN `l.Release()`.

## 7. Imports lock_unix_test.go must gain

S1's lock_unix_test.go imports `os`, `os/exec`, `testing`. The reaping tests add: `fmt` (Sprintf),
`math` (MaxInt32), `path/filepath` (filepath.Join for fake paths; or use lockDir()+"/x.lock" — Join is
cleaner), `strconv` (Itoa(os.Getpid())). `os/exec` stays (S1's dead-pid processAlive test uses it; unused
import would fail vet — but S1's tests still use it, so it stays). All stdlib; go.mod unchanged.

## 8. Scope fences (do NOT touch)

- `internal/lock/lock.go` (reapStaleLocks + Acquire wiring — P1.M2.T1.S2, LANDED).
- `internal/lock/lock_unix.go` / `lock_windows.go` (processAlive — P1.M2.T1.S1, LANDED).
- `internal/lock/lock_test.go` (existing tests — the contract's helpers/pattern are reused, not edited;
  resetCurrent stays there and is visible to lock_unix_test.go on Unix).
- `internal/signal/*` + `cmd/stagehand/main.go` (P1.M2.T2 exit-path release — different concern).
- `docs/*` (DOCS: none — test-only).
- go.mod / go.sum (stdlib only).

## 9. Files touched

- `internal/lock/lock_unix_test.go` — ADD `writeLockFile` helper + 2 reaping tests + import growth (fmt/math/
  path/filepath/strconv). This is the SOLE file edited. (Deviation from the contract's "lock_test.go":
  documented in §2 — the dead-pid assertion is Unix-only and would break Windows CI in lock_test.go.)
