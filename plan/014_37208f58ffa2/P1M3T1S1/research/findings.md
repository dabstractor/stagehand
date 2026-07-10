# Codebase Findings — P1.M3.T1.S1 (lock.Status + orphan detection)

## 1. The exact signatures of the helpers Status composes (verified in internal/lock/lock.go)

```go
// lock.go:302 — returns (string, ERROR), NOT just string!
func lockPath(repoPath string) (string, error) {
    dir, err := lockDir()           // can fail (no XDG, no HOME, no CWD fallback)
    if err != nil { return "", err }
    _, hash := lockHash(repoPath)
    return filepath.Join(dir, hash+".lock"), nil
}

// lock.go:259 — no error return (silently skips malformed lines)
func parseContents(data []byte) LockContents { ... }

// lock.go:47 — the struct Status returns part of
type LockContents struct { Pid, Hostname, Repo, Timestamp, Snapshot string }
```

```go
// lock_unix.go:35 (//go:build !windows)
func processAlive(pid int, hostname string) bool { ... }   // conservative: true on any ambiguity

// lock_windows.go:19 (//go:build windows)
func processAlive(pid int, hostname string) bool { return true }  // always alive (FR-K7)
```

**CRITICAL GOTCHA #1**: `lockPath` returns `(string, error)`. The item description's pseudo-code
`lockPath(repoPath)` glossed over the error — Status MUST handle it (propagate via `err`). lockDir
can fail when XDG_RUNTIME_DIR/XDG_CACHE_HOME are unset AND os.UserHomeDir fails (no CWD fallback —
the §18.5 anti-pattern). Tests isolate via `t.Setenv("XDG_RUNTIME_DIR", t.TempDir())`.

## 2. The "no lock" vs "error" distinction for os.ReadFile

The item says: "os.ReadFile(path) — if file doesn't exist, return (`"", LockContents{}, false, false, nil`)
meaning no lock." This is the ONLY swallow case. Other ReadFile errors (permission, I/O) are real
and must propagate as `err`. Pattern: `if os.IsNotExist(err) { return no-lock } else { return err }`.

## 3. The malformed-pid case (parseContents succeeds, Atoi fails)

`parseContents` never errors; an empty/garbage `pid=` line yields `c.Pid=""` or garbage. `strconv.Atoi`
then errors. Status should STILL return the path + parsed contents (diagnostic value) but with
`alive=false, orphan=false` (can't assess liveness without a pid) — mirrors reapStaleLocks which
`continue`s (skips) on Atoi error. This is the contract's "parseContents to get contents" + the
conservative-false-on-ambiguity thread.

## 4. Build-tag convention (Go 1.22, codebase uses MODERN form only)

```go
//go:build !windows

package lock
```
(lock_unix.go / lock_unix_test.go). NO legacy `// +build` line. The constraint line MUST be followed
by a blank line before `package`. Go 1.22 → `//go:build` is canonical; the old form is unnecessary.
Match this EXACTLY in orphan_unix.go / orphan_windows.go.

## 5. The stdlib-only invariant + the runtime.GOOS approach

The lock package doc (`package lock` comment, lock.go:1) states: "imports ONLY stdlib (no
golang.org/x/sys)". The new imports needed — `os/exec` (Darwin ps), `runtime` (GOOS switch),
`bufio`, `strconv`, `strings`, `os`, `fmt` — are ALL stdlib. 

The single-file runtime.GOOS approach (one orphan_unix.go, `//go:build !windows`, with
`if runtime.GOOS == "linux" { ... } else { /* ps */ }`) is valid: the WHOLE file compiles on every
Unix target, so every import is referenced by ≥1 function (ppidLinux uses os/bufio/strconv/strings/fmt;
ppidViaPs uses os/exec/strconv/strings). Go errors on unused IMPORTS, not unused package-level
FUNCTIONS — so ppidViaPs existing-but-uncalled-on-Linux is fine. This is simpler than build-tag-per-OS
(one file, not two) and matches the item description's "on Darwin (runtime.GOOS=='darwin')".

## 6. Test patterns to reuse (internal/lock/lock_unix_test.go)

- `writeLockFile(t, path, pid, hostname)` helper (lock_unix_test.go:60) — writes a minimal key=value
  lock file in the EXACT format parseContents reads. REUSE it (it's in the same package test files).
- `resetCurrent(t)` helper (lock_test.go) — clears the singleton; needed ONLY if a test Acquires.
  Status does NOT Acquire, so Status tests do NOT need it (Status never touches `current`).
- `t.Setenv("XDG_RUNTIME_DIR", t.TempDir())` + `t.Setenv("XDG_CACHE_HOME", "")` — isolate the lock
  dir so tests don't touch the real one. REQUIRED for any Status test that plants a lock file, because
  Status resolves the path via lockPath→lockDir.
- Dead-pid acquisition pattern (lock_unix_test.go:38): `cmd := exec.Command("true"); cmd.Start();
  deadPID := cmd.Process.Pid; cmd.Wait()` — gives a guaranteed-dead pid for the conservative-false
  orphan test (proc missing / ps error → false).

## 7. The orphan==TRUE case is hard to unit-test deterministically

Creating a real reparented-to-init pid in a unit test is flaky/OS-dependent. The DETERMINISTIC net:
- appearsOrphaned(deadPid) == false (conservative — /proc missing, ps exits non-zero)
- appearsOrphaned(selfPid) == false in normal dev shells (parent != 1); assert it returns false with
  a comment noting CI-under-init could differ (heuristic limitation, FR-K4 "appears orphaned")
- The orphan==true path is proven by the E2E harness (P1.M4.T1.S1) which spawns a real stagecoach
  subprocess, kills the launcher, and asserts the lock is reclaimed. (This mirrors the existing
  processAlive test philosophy: test the branches you can pin deterministically.)

## 8. Coverage gate does NOT include internal/lock

Makefile coverage-gate (line 77) gates ONLY `internal/{git,provider,generate,config}`. internal/lock
has no coverage threshold. So the conservative test net above is sufficient; no coverage pressure.

## 9. Consumers (treat as contracts — they land AFTER this item)

- **P1.M3.T2.S1** (`stagecoach lock status` subcommand): calls `lock.Status(repoDir)`. Uses path==""
  to print "no run lock for <repo>"; prints path/contents/alive/orphan otherwise. READ-ONLY (FR-K4).
- **P1.M3.T3.S1** (Busy message orphan hint): in `handleLockContention` (default_action.go:300), it
  has `heldErr *lock.HeldError` with `heldErr.Contents.Pid`. To get the orphan bool it calls
  `lock.Status(repoDir)` (the lock file still exists at contention time — Acquire returned HeldError,
  no removal happened) and reads the `orphan` return. This keeps appearsOrphaned unexported (per the
  contract) while giving the consumer a single read-only entry point. NOTE this for the consumer PRP.

## 10. Windows test file

There is NO `lock_windows_test.go` today (processAlive-windows is untested by convention — the
cross-compile `GOOS=windows go build ./internal/lock/...` is the Windows validation). appearsOrphaned
on Windows is a one-line `return false`; adding a trivial `lock_windows_test.go` asserting
`!appearsOrphaned(1)` is OPTIONAL and low-value. Recommend: rely on cross-compile + the !windows
unit tests; add the windows test only if it's trivial. (The contract: "Mock nothing — test with temp
lock files and known pids" — all those live in the !windows file.)

## 11. Validation commands (verified against Makefile)

- `go build ./...` + `GOOS=windows go build ./...` + `GOOS=linux go build ./...` + `GOOS=darwin go build ./...`
- `go vet ./internal/lock/...`
- `gofmt -l internal/lock/` (must be empty)
- `go test ./internal/lock/ -race -v` (Unix dev host runs the !windows tests)
- `make lint` (golangci-lint)
- `make test` (full race suite — regression net; lock pkg tests must stay green)
