---
name: "P1.M2.T1.S1 — processAlive cross-platform helper (lock_unix.go, lock_windows.go)"
description: |
  Foundational cross-platform pid-liveness helper for §18.5 stale lock-FILE reaping (FR52). `flock`
  auto-releases on process death so the *lock* is never stale, but the lock *FILE* is orphaned by exits
  that bypass the deferred `os.Remove` (SIGKILL, crash, signal-rescue `os.Exit`) → unbounded disk litter.
  `Acquire` will reap these (S2: `reapStaleLocks`), and the safety of unlinking depends on knowing the
  recorded pid is DEAD (a dead pid holds no fd → no flock → unlinking is safe; a LIVE pid must NEVER be
  reaped — unlinking its inode-bound-flock file would let a contender `O_CREATE`+flock a fresh inode and
  defeat FR52). `processAlive(pid, hostname) bool` is the pid-liveness check that makes reaping safe. S1
  delivers the helper; S2 consumes it. Mirrors the existing `flock`/`isWouldBlock` build-tag split.

  ⚠️ **THE central design call — mirror lock_reaping.md's processAlive spec VERBATIM, split by build tag.**
  Unix (`lock_unix.go`, `//go:build !windows`): hostname empty/mismatch → true (foreign host, don't reap);
  `syscall.Kill(pid, 0)` nil → true (alive); EPERM → true (alive, different user); else → false (ESRCH/dead).
  Windows (`lock_windows.go`, `//go:build windows`): `return true` always (flock is a no-op there → no
  inode-bound-flock hazard → reaping is a no-op → "never reap a live pid" trivially holds; the §13.5 CAS is
  the guarantee). Conservative on every ambiguity — the invariant is "never reap a live pid".

  ⚠️ **THE second design call — `unused` lint runs ONLY on Linux (ci.yml:52, single ubuntu job); only the
  UNIX processAlive needs a caller.** `.golangci.yml` enables `unused`; an uncalled unexported method trips
  U1000. BUT the lint job is `runs-on: ubuntu-latest` only, so lock_windows.go (excluded on Linux) is never
  analyzed there, and Windows CI runs `go build`+`go test` (NOT lint — Go's compiler doesn't flag unused
  methods). So: put ALL processAlive tests in ONE new `internal/lock/lock_unix_test.go` (`//go:build !windows`)
  → the Unix impl is "used" (Linux lint green) + validated; the Windows stub is exercised indirectly by S2's
  reapStaleLocks tests on Windows. Do NOT //nolint — add the real tests.

  ⚠️ **THE third design call — the dead-pid test uses fork+wait; `t.Errorf` (real assertion).** Spawn
  `exec.Command("true")`, record `cmd.Process.Pid`, `cmd.Wait()` (child exits → pid dead), assert
  `processAlive(deadPID, thisHost)` == false (the ESRCH path — the reaping trigger). `t.Skip` only on fork
  failure (`true` not on PATH). The pid-recycling race (freed pid reassigned in the microsecond window) is
  astronomically unlikely (pids are assigned sequentially; noted in a comment) — a real bug like "always
  true" fails the test deterministically.

  SCOPE: edit `internal/lock/lock_unix.go` (processAlive + `os` import) + `internal/lock/lock_windows.go`
  (processAlive stub) + NEW `internal/lock/lock_unix_test.go` (4 tests, `//go:build !windows`). NO lock.go
  (S2 owns reapStaleLocks + the 3 over-claim doc fixes), NO signal/main (P1.M2.T2), NO user-facing docs.
  INPUT = the flock/isWouldBlock build-tag split as the pattern. OUTPUT = processAlive (unexported,
  package-internal), consumed by reapStaleLocks (P1.M2.T1.S2). DOCS: Mode A — the processAlive doc comment.
---

## Goal

**Feature Goal**: Add a cross-platform `processAlive(pid int, hostname string) bool` helper to the lock
package's build-tag files — a real Unix impl (`syscall.Kill(pid, 0)`) and a conservative always-true
Windows stub — that reports whether a recorded lock-file's pid is a live process on this host, so S2's
`reapStaleLocks` can safely unlink only DEAD holders' orphaned files while NEVER reaping a live one.

**Deliverable** (edits to 2 files + 1 new test file):
1. **`internal/lock/lock_unix.go`** (`//go:build !windows`) — add `processAlive(pid int, hostname string) bool`
   per lock_reaping.md; add `"os"` to the import block (for `os.Hostname`).
2. **`internal/lock/lock_windows.go`** (`//go:build windows`) — add `processAlive(pid int, hostname string) bool`
   that returns `true` always (conservative no-op; stays import-free).
3. **`internal/lock/lock_unix_test.go`** (NEW, `//go:build !windows`) — 4 tests: self-alive, foreign-host
   conservative, empty-hostname conservative, dead-pid (fork+wait → false).

**Success Definition**: `gofmt -l internal/lock/`, `go vet ./internal/lock/`, `go build ./...` clean on the
current (Unix) platform; `go test -race ./internal/lock/` green (4 new tests pass + existing lock tests
unchanged); `make lint` green (Unix processAlive is "used" → no U1000). go.mod/go.sum unchanged; only
lock_unix.go + lock_windows.go + the new lock_unix_test.go touched. The processAlive doc comment documents
the cross-platform semantics + the "never reap a live pid" invariant.

## User Persona

**Target User**: The NEXT subtask (P1.M2.T1.S2 — `reapStaleLocks`, which calls processAlive for each
`*.lock` file in the lock dir), and transitively the user whose lock directory accumulates orphaned files
(FR52 §18.5 disk hygiene). This task is the pid-liveness primitive that makes reaping safe.

**Use Case**: (internal helper, no user-visible behavior yet) `reapStaleLocks` reads a lock file's
`pid=`/`hostname=`, calls `processAlive(pid, hostname)`, and unlinks the file ONLY if it returns false.

**User Journey**: (future) `Acquire` → after flock succeeds, `reapStaleLocks(dir)` → for each orphan,
`processAlive` decides keep-vs-reap → dead holders' files unlinked, live ones untouched. This task is the
decision primitive.

**Pain Points Addressed**: removes the structural blocker (no pid-liveness check) for safe stale-file
reaping; the "never reap a live pid" invariant is encoded in the helper itself.

## Why

- **Structural enabler for S2.** `reapStaleLocks` needs a correct, conservative pid-liveness check to reap
  safely. `processAlive` is that check, with the safety invariant built in (alive/ambiguous → true).
- **Mirrors the proven flock/isWouldBlock split.** The build-tag pattern (real Unix impl, no-op Windows
  stub) is already established; processAlive follows it exactly.
- **Conservative = safe.** Every ambiguous case (foreign host, os.Hostname error, EPERM) returns true
  (don't reap). The only false is a confirmed-dead pid (ESRCH). This is precisely what makes unlinking safe.
- **Self-contained + lint-green.** The Unix tests validate the helper AND keep `unused` green (lint is
  Linux-only). No dependency on S2 landing first.

## What

Two platform-specific `processAlive` impls (real Unix, stub Windows) + a Unix-only test file. No lock.go,
no reapStaleLocks, no signal/main, no user-facing docs. The helper is unexported (package-internal).

### Success Criteria

- [ ] `lock_unix.go` has `func processAlive(pid int, hostname string) bool` matching lock_reaping.md:
      hostname empty/mismatch ⇒ true; `Kill(pid,0)` nil ⇒ true; EPERM ⇒ true; else ⇒ false.
- [ ] `lock_windows.go` has `func processAlive(pid int, hostname string) bool` returning `true` (no-op stub).
- [ ] `"os"` added to lock_unix.go's imports; lock_windows.go stays import-free.
- [ ] `lock_unix_test.go` (`//go:build !windows`) has 4 tests: self-alive (true), foreign-host (true),
      empty-hostname (true), dead-pid via fork+wait (false). All pass.
- [ ] `gofmt -l internal/lock/`, `go vet ./internal/lock/`, `go build ./...`, `go test -race ./internal/lock/`,
      `make lint` clean/green; go.mod/go.sum unchanged; only the 3 listed files touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the verbatim processAlive bodies
(quoted below), the import additions, the build-tag split, and the 4 test specs. No PRD/git-internals
knowledge required — this is a pid-liveness helper mirroring an existing build-tag pattern.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/011_98cef660a41d/P1M2T1S1/research/processAlive_helper.md
  why: the verbatim processAlive spec (both platforms), the imports, the `unused`-lint-is-Linux-only finding
       (ci.yml:52), the test placement (lock_unix_test.go `//go:build !windows`), the dead-pid fork approach,
       and the scope boundary (S2 owns reapStaleLocks + lock.go over-claim fixes).
  critical: the lint job is ubuntu-only → only the UNIX processAlive needs a caller. The Windows stub is
       never analyzed on Linux lint and Windows CI doesn't lint. Do NOT //nolint — add the Unix tests.

- docfile: plan/011_98cef660a41d/architecture/lock_reaping.md
  section: "Fix 1: Stale-File Reaping in Acquire" → "processAlive(pid int, hostname string) bool"
  why: the AUTHORITATIVE processAlive spec (verbatim code for both platforms) + the safety invariant
       ("a live pid is NEVER reaped … pid-liveness check is precisely what makes unlinking safe").
  critical: the exact Unix logic (hostname check → Kill(pid,0) → nil/EPERM/ESRCH) and the Windows
       always-true stub. Mirror these verbatim.

- file: internal/lock/lock_unix.go   (full, ~22 lines)
  why: the file you edit + the build-tag pattern to mirror (`//go:build !windows`, package lock, flock +
       isWouldBlock already there). Add processAlive after isWouldBlock; add `"os"` to imports.
  pattern: `import ( "errors"; "syscall" )` → add `"os"` (alphabetical: errors, os, syscall). Functions
           are unexported, with a doc comment each.
  gotcha: os.Hostname() returns (string, error); `host, _ := os.Hostname()` ignores the error — an error
           yields host="" which the `hostname != host` check catches (→ true, conservative). Correct.

- file: internal/lock/lock_windows.go   (full, ~15 lines)
  why: the Windows twin. `//go:build windows`, package lock, NO imports (no-op flock/isWouldBlock). Add the
           processAlive stub (return true) after isWouldBlock. STAYS import-free (no os/syscall/errors).
  pattern: one-line `func processAlive(pid int, hostname string) bool { return true }` + doc comment.

- file: internal/lock/lock.go   (package doc: stdlib-only leaf, no stagecoach imports)
  why: confirms the package constraint (stdlib-only; no internal/* imports — processAlive uses only os/
       syscall/errors, all stdlib). DO NOT EDIT lock.go — S2 (reapStaleLocks) + the lock.go over-claim
       doc fixes are P1.M2.T1.S2's scope.
  gotcha: do NOT add processAlive to lock.go — it MUST live in the build-tag files (platform-specific impl).

- file: internal/lock/lock_test.go   (no build tag → compiles on all platforms; package lock white-box)
  why: confirms the test conventions (package lock, resetCurrent helper, t.Setenv XDG isolation). The
       processAlive tests go in a SEPARATE `lock_unix_test.go` (`//go:build !windows`) — NOT lock_test.go —
       because the dead-pid test is Unix-specific (Windows processAlive always returns true).

- file: .github/workflows/ci.yml   (lint job: `runs-on: ubuntu-latest` only, line 52-54)
  why: PROVES the lint gate is Linux-only. lock_windows.go is excluded from Linux lint → its processAlive
       is never analyzed → no U1000 there. Windows CI runs build+test only (no lint). So only the Unix
       processAlive needs a caller → the Unix tests suffice.
```

### Current Codebase tree (relevant slice)

```bash
internal/lock/
  lock_unix.go        # //go:build !windows — flock, isWouldBlock — EDIT (add processAlive + "os" import)
  lock_windows.go     # //go:build windows — no-op flock, isWouldBlock — EDIT (add processAlive stub)
  lock_unix_test.go   # NEW — //go:build !windows — 4 processAlive tests
  lock.go             # package lock (shared, no build tag) — NO edit (S2 owns reapStaleLocks + over-claim fixes)
  lock_test.go        # no build tag — NO edit (existing tests; processAlive tests go in lock_unix_test.go)
go.mod / go.sum       # unchanged (stdlib only)
```

### Desired Codebase tree with files to be added

```bash
internal/lock/
  lock_unix.go        # EDIT: + "os" import, + processAlive
  lock_windows.go     # EDIT: + processAlive (return true)
  lock_unix_test.go   # NEW: //go:build !windows, 4 tests
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: processAlive lives in the BUILD-TAG files (lock_unix.go / lock_windows.go), NOT in lock.go.
// It is platform-specific (Unix uses syscall.Kill; Windows is a no-op). Mirrors flock/isWouldBlock exactly.

// CRITICAL: the Unix impl mirrors lock_reaping.md VERBATIM. hostname=="" || hostname != os.Hostname() → true
// (foreign host, don't reap); Kill(pid,0)==nil → true; errors.Is(err, syscall.EPERM) → true; else → false.
// Be conservative on every ambiguity — the invariant is "never reap a live pid".

// CRITICAL: `unused` lint is enabled but the lint JOB is ubuntu-only (ci.yml:52). Only the UNIX processAlive
// is analyzed on Linux lint → it MUST have a caller (the tests in lock_unix_test.go). lock_windows.go is
// excluded on Linux → no U1000 for the Windows impl. Windows CI runs build+test only (no lint). Do NOT
// //nolint — add the Unix tests (they validate + keep it used).

// CRITICAL: the dead-pid test goes in lock_unix_test.go (`//go:build !windows`), NOT lock_test.go. The
// dead-pid assertion (processAlive → false) is Unix-specific; on Windows processAlive always returns true,
// so a no-build-tag dead-pid test would FAIL on Windows. The 3 conservative/alive tests COULD go in
// lock_test.go (they pass on both platforms) — but keeping all 4 in lock_unix_test.go is cleaner (one home).

// GOTCHA: os.Hostname() returns (string, error). `host, _ := os.Hostname()` ignores the error; an error
// yields host="" → `hostname != ""` (assuming a real hostname was passed) → true (conservative). Correct.

// GOTCHA: errors.Is(err, syscall.EPERM) works because syscall.Kill returns a raw syscall.Errno and errors.Is's
// direct-equality first step matches syscall.EPERM. (err == syscall.EPERM also works; use errors.Is per the spec.)

// GOTCHA: the dead-pid test's fork — `exec.Command("true")`. /bin/true (or /usr/bin/true) exists on Linux +
// macOS. t.Skip on Start failure (defensive). After cmd.Wait() the child is definitively dead (zombie reaped);
// its pid is free and won't be reassigned until the sequential pid counter wraps (astronomically unlikely in
// the microsecond window) — t.Errorf (real assertion) is safe; a bug like "always true" fails it deterministically.

// GOTCHA: keep processAlive UNEXPORTED. It is package-internal (mirrors flock/isWouldBlock). The public
// surface is S2's reapStaleLocks (called from Acquire); processAlive is an implementation detail of reaping.
```

## Implementation Blueprint

### Data models and structure

No new types. The two platform impls + the test file:

```go
// internal/lock/lock_unix.go — add "os" to imports, add processAlive after isWouldBlock:
import (
	"errors"
	"os"      // ← ADD (os.Hostname for the foreign-host check)
	"syscall"
)

// processAlive reports whether pid is a live process on hostname, for stale lock-FILE reaping (PRD §18.5).
// It is the pid-liveness check that makes unlinking a dead holder's lock file safe: a dead pid holds no
// open fd → no flock → unlinking cannot defeat contention the way unlinking a LIVE holder's inode-bound
// flock file would. SAFETY INVARIANT — never reap a live pid; conservative on every ambiguity:
//   - hostname == "" or != this host → true (foreign host: don't reap; a recycled pid on THIS host is a
//     benign miss, reaped once the pid is free).
//   - syscall.Kill(pid, 0) == nil → true (alive, ours); EPERM → true (alive, different user).
//   - any other error → false (ESRCH → dead → safe to reap).
// Cross-platform: lock_windows.go provides an always-true twin (flock is a no-op there → no reaping; the
// §13.5 CAS is the guarantee). Used by reapStaleLocks (P1.M2.T1.S2).
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

```go
// internal/lock/lock_windows.go — add processAlive after isWouldBlock (stays import-free):
// processAlive is a conservative no-op on Windows: it always reports the pid as alive (never reap). flock
// is a no-op on Windows (no inode-bound-flock hazard — see flock above), so there is no dead-file reaping
// to do; the §13.5 CAS (update-ref HEAD compare-and-swap) is the safety guarantee. The "never reap a live
// pid" invariant is trivially satisfied (reap nothing). Cross-platform twin of lock_unix.go's processAlive;
// used by reapStaleLocks (P1.M2.T1.S2).
func processAlive(pid int, hostname string) bool {
	return true
}
```

```go
// internal/lock/lock_unix_test.go (NEW):
//go:build !windows

package lock

import (
	"os"
	"os/exec"
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
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: lock_unix.go — add processAlive + the "os" import
  - ADD `"os"` to the import block (alphabetical: errors, os, syscall).
  - ADD processAlive per the Data Models block, after isWouldBlock. Mirror lock_reaping.md VERBATIM.
  - DOC: cross-platform semantics + the "never reap a live pid" invariant (the Data Models comment).
  - GOTCHA: conservative on ambiguity; errors.Is(err, syscall.EPERM); host,_ := os.Hostname().

Task 2: lock_windows.go — add the processAlive no-op stub
  - ADD `func processAlive(pid int, hostname string) bool { return true }` after isWouldBlock.
  - DOC: always-true (flock no-op on Windows → no reaping; §13.5 CAS is the guarantee; invariant trivially holds).
  - GOTCHA: stays IMPORT-FREE (no os/syscall/errors). One line + doc comment.

Task 3: CREATE lock_unix_test.go (//go:build !windows) — 4 tests
  - FILE header: `//go:build !windows` then `package lock` + imports (os, os/exec, testing).
  - TestProcessAlive_SelfAlive (true), _ForeignHostConservative (true), _EmptyHostnameConservative (true),
    _DeadPID (fork+wait → false, t.Errorf, t.Skip on fork failure).
  - GOTCHA: this file is Unix-only → the dead-pid test doesn't run on Windows (where it'd fail: always-true).

Task 4: VERIFY (no further file change)
  - RUN `gofmt -w internal/lock/lock_unix.go internal/lock/lock_windows.go internal/lock/lock_unix_test.go`;
    `go vet ./internal/lock/`; `go build ./...`; `go test -race ./internal/lock/`; `make lint`.
  - go.mod/go.sum byte-unchanged. Only the 3 files touched. No lock.go / signal / main / reapStaleLocks.
```

### Implementation Patterns & Key Details

```go
// The build-tag split — mirror flock/isWouldBlock (processAlive is the third platform-split function):
//   lock_unix.go    (!windows): real impl (syscall.Kill)
//   lock_windows.go (windows) : no-op stub (return true)

// The conservative cascade — "never reap a live pid" is the keystone:
host, _ := os.Hostname()
if hostname == "" || hostname != host { return true }      // foreign host
if err := syscall.Kill(pid, 0); err == nil { return true } // alive (ours)
if errors.Is(err, syscall.EPERM)          { return true }  // alive (other user)
return false                                               // ESRCH → dead

// The dead-pid test — fork+wait makes a definitively-dead pid:
cmd := exec.Command("true")
cmd.Start()              // child pid assigned
deadPID := cmd.Process.Pid
cmd.Wait()               // child exits; zombie reaped; pid now free/dead
processAlive(deadPID, host) // → false (Kill → ESRCH)
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — stdlib only (os, syscall, errors, os/exec, testing). go mod tidy is a no-op.

FROZEN / NOT-EDITED:
  - internal/lock/lock.go — S2 (P1.M2.T1.S2) owns reapStaleLocks + the 3 over-claim doc fixes (lock_reaping.md
    "Doc-Comment Corrections": package doc line 2, Locker doc line 31, Acquire doc line 67). Do NOT touch lock.go.
  - internal/lock/lock_test.go — the existing tests; processAlive tests go in the new lock_unix_test.go.
  - internal/signal/* + cmd/stagecoach/main.go — P1.M2.T2 owns the exit-path release (ReleaseCurrent +
    OnRescueExit seam). Different concern.

DOWNSTREAM CONSUMER (do NOT implement here):
  - P1.M2.T1.S2 (next): `reapStaleLocks(dir)` globs `*.lock`, parses each (parseContents), and calls
    `processAlive(pid, c.Hostname)` — unlinking only when it returns false. Wired into Acquire after flock
    succeeds. THAT task also fixes the lock.go over-claims. This task (S1) is the primitive S2 calls.

NO DATABASE / NO ROUTES / NO CONFIG / NO CLI / NO USER-FACING DOCS.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/lock/lock_unix.go internal/lock/lock_windows.go internal/lock/lock_unix_test.go
test -z "$(gofmt -l internal/lock/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/lock/        # Catches a malformed function / wrong build-tag placement.
go build ./...                 # Compiles on the current (Unix) platform.
# Cross-build Windows to confirm the Windows stub compiles (the build matrix does this in CI):
GOOS=windows go build ./internal/lock/ && echo "windows build OK" || echo "windows build FAILED"
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: clean + both platforms build. If GOOS=windows build fails, lock_windows.go's processAlive has a typo.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./internal/lock/ -v -run 'TestProcessAlive'
# Expected: 4 tests PASS. self-alive/foreign/empty → true; dead-pid → false. The existing lock tests stay green.
go test -race ./internal/lock/   # full lock suite — no regression.
go test -race ./...              # full module — no regression.
# Expected: green throughout. (lock_unix_test.go is excluded on Windows; there it's the existing lock_test.go only.)
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only the 3 files changed:
git diff --name-only | grep -Ev '^internal/lock/lock_unix\.go$|^internal/lock/lock_windows\.go$|^internal/lock/lock_unix_test\.go$' \
  && echo "UNEXPECTED file changed" || echo "only lock_unix.go + lock_windows.go + lock_unix_test.go changed (good)"
# Confirm processAlive is in BOTH build-tag files (and NOT in lock.go):
grep -rn "func processAlive" internal/lock/   # expect exactly 2 matches: lock_unix.go + lock_windows.go
grep -n "processAlive" internal/lock/lock.go && echo "BAD: processAlive in lock.go" || echo "not in lock.go (good)"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# THE lint gate — `unused` is enabled; the Unix processAlive must have a caller (the tests):
make lint 2>&1 | grep -iE 'processAlive|unused|U1000' && echo "BAD: processAlive flagged" \
  || echo "processAlive not flagged by unused (good — tests are its caller)"
# Expected: no U1000/unused finding for processAlive (lock_unix_test.go calls it). If flagged, the test
#   file's build tag is wrong (must be //go:build !windows) or the tests don't call processAlive.
# Windows-stub audit: confirm lock_windows.go's processAlive is the always-true no-op (trivial; exercised by S2):
GOOS=windows go vet ./internal/lock/ && echo "windows vet OK"
# EPERM path (optional, hard to trigger without a different user): the errors.Is(err, syscall.EPERM) branch
# is covered by code review (it's the standard Kill(pid,0) idiom); a different-user test is out of scope.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `gofmt -l internal/lock/`, `go vet ./internal/lock/`, `go build ./...`,
      `GOOS=windows go build ./internal/lock/`, `go mod tidy` no-op.
- [ ] Level 2 green: the 4 TestProcessAlive_* tests pass; `go test -race ./internal/lock/` + `./...` green.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only the 3 files changed; processAlive in exactly
      lock_unix.go + lock_windows.go (not lock.go).
- [ ] Level 4: `make lint` green — no `unused`/U1000 for processAlive.

### Feature Validation

- [ ] `lock_unix.go` processAlive matches lock_reaping.md: foreign-host/empty → true; Kill nil → true;
      EPERM → true; else → false.
- [ ] `lock_windows.go` processAlive returns `true` (always-true stub).
- [ ] `"os"` added to lock_unix.go; lock_windows.go import-free.
- [ ] The 4 tests pass (incl. dead-pid via fork+wait → false).

### Code Quality Validation

- [ ] Mirrors the flock/isWouldBlock build-tag split; processAlive is unexported (package-internal).
- [ ] Conservative on every ambiguity ("never reap a live pid" invariant encoded in the helper).
- [ ] Doc comment documents cross-platform semantics + the invariant (Mode A).
- [ ] No scope creep into lock.go (S2), signal/main (P1.M2.T2), reapStaleLocks, or user-facing docs.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Mode-A doc comment on processAlive (both files). No user-facing docs change.
- [ ] go.mod/go.sum byte-unchanged; one new file (lock_unix_test.go) + two edits.

---

## Anti-Patterns to Avoid

- ❌ Don't put processAlive in lock.go. It is platform-specific (Unix syscall.Kill vs Windows no-op) → it
  MUST live in the build-tag files (lock_unix.go / lock_windows.go), mirroring flock/isWouldBlock.
- ❌ Don't deviate from lock_reaping.md's spec. The Unix cascade (hostname check → Kill nil → EPERM → else)
  is the authoritative design; mirror it verbatim. Conservative on every ambiguity (the invariant is "never
  reap a live pid").
- ❌ Don't forget the `unused` lint. It's enabled AND the lint job is Linux-only → only the Unix processAlive
  is analyzed. The lock_unix_test.go tests are its caller — without them, U1000. Do NOT //nolint; add the tests.
- ❌ Don't put the dead-pid test in lock_test.go (no build tag). It asserts processAlive → false, which FAILS
  on Windows (always-true). Put it in lock_unix_test.go (`//go:build !windows`).
- ❌ Don't use `t.Skip` for the dead-pid "alive" result — that masks a real bug (always-true). Use `t.Errorf`
  (real assertion); the pid-recycling race is astronomically unlikely (sequential pid assignment). `t.Skip`
  only on the fork-Start failure.
- ❌ Don't add imports to lock_windows.go. The stub `return true` needs none; adding os/syscall/errors would
  be unused-import errors. Only lock_unix.go gets the `"os"` import.
- ❌ Don't touch lock.go. S2 (P1.M2.T1.S2) owns reapStaleLocks AND the 3 lock.go over-claim doc fixes
  (lock_reaping.md "Doc-Comment Corrections"). Editing lock.go here overlaps S2.
- ❌ Don't confuse ESRCH with "reap". processAlive returns false for ESRCH (dead) — the CALLER (S2's
  reapStaleLocks) unlinks. processAlive only DECIDES; it doesn't unlink. Keep it pure (no I/O beyond Kill).
- ❌ Don't use `err == syscall.EPERM` if the spec says `errors.Is` — both work (Kill returns a raw Errno;
  errors.Is's direct-equality matches), but use `errors.Is(err, syscall.EPERM)` per lock_reaping.md for
  consistency with the codebase's isWouldBlock (`errors.Is(err, syscall.EWOULDBLOCK)`).
- ❌ Don't change go.mod/go.sum. stdlib only (os/syscall/errors/os/exec/testing).
- ❌ Don't skip `GOOS=windows go build ./internal/lock/` — it confirms the Windows stub compiles (the CI
  build matrix does this; catch a typo locally first). And don't skip `make lint` (the unused gate).
