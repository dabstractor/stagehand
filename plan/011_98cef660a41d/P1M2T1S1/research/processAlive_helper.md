# Research: processAlive cross-platform helper (P1.M2.T1.S1)

Verified against the live codebase + `plan/011_98cef660a41d/architecture/lock_reaping.md`. Source of
truth for the cross-platform pid-liveness helper that makes stale lock-FILE reaping safe.

## Why it exists (lock_reaping.md Fix 1)

`flock` auto-releases on process death, so the *lock* is never stale — but the lock *FILE*
(`$XDG_RUNTIME_DIR/stagecoach/locks/<hash>.lock`) is orphaned by any exit that bypasses the deferred
`os.Remove` (SIGKILL, crash, the signal-rescue `os.Exit`). These accumulate as unbounded litter.
`Acquire` reaps them (S2: `reapStaleLocks`), and the safety of unlinking depends on knowing the recorded
pid is DEAD: a dead pid holds no open fd → no flock → unlinking its file is safe (cannot defeat contention
the way unlinking a LIVE holder's inode-bound-flock file would). `processAlive` is the pid-liveness check
that makes reaping safe. S1 delivers the helper; S2 (reapStaleLocks) consumes it.

## The build-tag split (mirror flock/isWouldBlock)

`internal/lock/lock_unix.go` (`//go:build !windows`, package lock, imports errors+syscall) and
`internal/lock/lock_windows.go` (`//go:build windows`, package lock, NO imports — no-op flock/isWouldBlock).
processAlive follows the SAME split: a real impl in lock_unix.go, a conservative always-true stub in
lock_windows.go. (lock.go is `package lock`, stdlib-only, "self-contained leaf — no stagecoach imports".)

## processAlive — verbatim from lock_reaping.md (the authoritative spec)

**Unix (lock_unix.go):**
```go
func processAlive(pid int, hostname string) bool {
	host, _ := os.Hostname()
	if hostname == "" || hostname != host {
		return true // foreign host → don't reap (conservative)
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true // alive
	}
	if errors.Is(err, syscall.EPERM) {
		return true // alive, different user
	}
	return false // ESRCH → dead
}
```
- `syscall.Kill(pid, 0)`: signal 0 = "check existence, don't actually signal". Returns nil (alive, ours),
  EPERM (alive, different user), or ESRCH (no such process = dead). The only realistic returns.
- `os.Hostname()` error → host="" → `hostname != ""` → true (conservative: treat as foreign, don't reap).
- `errors.Is(err, syscall.EPERM)`: syscall.Kill returns a raw syscall.Errno; errors.Is's direct-equality
  first step matches `syscall.EPERM`. (err == syscall.EPERM also works; the contract/research uses errors.Is.)

**Windows (lock_windows.go):** `return true` always. flock is a no-op on Windows (no inode-bound-flock
hazard), so reaping is a no-op too; the §13.5 CAS is the guarantee. The "never reap a live pid" invariant
is trivially satisfied (reap nothing).

## Imports

- lock_unix.go: ADD `"os"` (for os.Hostname). Current imports: errors, syscall → becomes errors, os, syscall
  (alphabetical). `errors` and `syscall` already present.
- lock_windows.go: stays IMPORT-FREE (the stub returns true; no os/syscall/errors needed).

## ⚠️ The `unused` lint — Linux-only, so only the UNIX impl needs a caller

`.golangci.yml` enables `unused`. BUT ci.yml's lint job runs ONLY on `ubuntu-latest` (ci.yml:52-54:
"golangci-lint (single ubuntu job — faster, no windows-lint flake)"). Consequences:
- On Linux lint, lock_unix.go is compiled → the Unix processAlive is analyzed → an uncalled unexported
  method trips U1000. So S1 MUST include a Unix test that calls processAlive (keeps it "used" + validates).
- lock_windows.go is NOT compiled on Linux → its processAlive is NOT analyzed → no U1000 for the Windows
  impl on Linux lint.
- Windows CI (ci.yml build matrix) runs `go build` + `go test` only (NOT golangci-lint). Go's compiler
  does NOT error on unused unexported methods → the Windows processAlive being uncalled is fine there.
  (S2's reapStaleLocks in lock.go — no build tag — will call processAlive on BOTH platforms, making the
  Windows impl "used" once S2 lands.)

⇒ Put ALL processAlive tests in ONE new file `internal/lock/lock_unix_test.go` (`//go:build !windows`).
This keeps the Unix impl used (Linux lint green) and validates the Unix logic (the interesting one). The
Windows stub is trivial (always true) and is exercised indirectly by S2's reapStaleLocks tests on Windows.

## The tests (lock_unix_test.go, `//go:build !windows`, package lock — white-box)

Four cases (the safety-critical "alive/conservative" trio + the dead-pid reaping trigger):
1. `TestProcessAlive_SelfAlive` — `processAlive(os.Getpid(), thisHost)` == true (alive, ours).
2. `TestProcessAlive_ForeignHostConservative` — `processAlive(os.Getpid(), foreignHost)` == true (don't reap cross-host).
3. `TestProcessAlive_EmptyHostnameConservative` — `processAlive(os.Getpid(), "")` == true.
4. `TestProcessAlive_DeadPID` — fork `exec.Command("true")`, `cmd.Start()`, record `cmd.Process.Pid`,
   `cmd.Wait()` (child exits → pid dead), assert `processAlive(deadPID, thisHost)` == false (ESRCH path).
   Uses `t.Errorf` (real assertion — a bug like "always true" fails it deterministically). The pid-recycling
   race (freed pid reassigned in the microsecond window) is astronomically unlikely on a normal system (pids
   are assigned sequentially; the freed pid won't reappear until the counter wraps) — noted in a comment.
   `t.Skip` on `cmd.Start` failure (e.g. `true` not on PATH — it is on Linux/macOS as /bin/true or /usr/bin/true).

`lock_unix_test.go` imports: os, os/exec, testing.

## Scope boundary (no conflict / no overlap)

- **P1.M1.T3.S2 (parallel)** is TEST-ONLY in `internal/generate/tokenlimit_invariant_test.go` (FR3j token-
  budget invariant). DIFFERENT package → ZERO overlap with internal/lock.
- **P1.M2.T1.S2 (next)** = `reapStaleLocks` + wire into `Acquire` + fix the 3 lock.go over-claims
  (lock_reaping.md "Doc-Comment Corrections"). THAT owns lock.go edits. S1 is ONLY processAlive + its tests.
- **P1.M2.T2** = the exit-path release (`ReleaseCurrent` + `signal.OnRescueExit` + main wiring). Different.
- S1 touches: `internal/lock/lock_unix.go` (processAlive + `os` import), `internal/lock/lock_windows.go`
  (processAlive stub), `internal/lock/lock_unix_test.go` (NEW). NO lock.go, NO signal, NO main, NO docs/*.

## DOCS: Mode A — the processAlive DOC COMMENT (cross-platform semantics + 'never reap a live pid'
invariant). NO user-facing docs change. The lock.go over-claim fixes are S2's Mode-A scope, NOT S1's.
