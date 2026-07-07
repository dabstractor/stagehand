---
name: "P1.M2.T3.S1 — Stale-file reaping tests (lock_unix_test.go): dead-pid reaped, live/foreign/malformed spared, idempotent"
description: |
  TEST-ONLY subtask. Verifies the §18.5 stale lock-FILE reaping landed by P1.M2.T1.S2 (`reapStaleLocks` +
  the `Acquire` wiring) on top of P1.M2.T1.S1 (`processAlive`). The production code is ALREADY IN THE TREE
  (`internal/lock/lock.go`: `reapStaleLocks(dir)` globs `*.lock` → `parseContents` → `strconv.Atoi(c.Pid)`
  → `if !processAlive(pid, c.Hostname) { os.Remove(f) }`; `Acquire` calls it after `current.Store(l)`).
  This task adds ONLY tests — no production change, no docs, no go.mod.

  ⚠️ **THE central design call — the reaping tests go in `lock_unix_test.go` (`//go:build !windows`), NOT
  `lock_test.go` as the contract literally says.** The headline assertion — "the dead-pid file is REMOVED" —
  is Unix-specific: on Windows `processAlive` is an always-`true` stub (flock is a no-op there → reaping is a
  documented no-op), so the dead-pid file is NOT reaped on Windows, and the assertion FAILS Windows CI (which
  runs `go test`). This is the IDENTICAL cross-platform issue S1 (P1.M2.T1.S1) resolved for the `processAlive`
  dead-pid unit test: S1 put it in `lock_unix_test.go` because "on Windows processAlive always returns true,
  so a no-build-tag dead-pid test would FAIL on Windows." The contract's "lock_test.go" placement is
  OVERRIDDEN by this hard CI-correctness requirement. lock_unix_test.go is the established home for Unix-specific
  dead-pid assertions + the processAlive tests the reaping tests exercise. See research §2.

  ⚠️ **THE fixture set — 4 planted `*.lock` files + the holder's own, each pinning one processAlive branch.**
  (1) `dead.lock`: pid=`math.MaxInt32` (2147483647), this hostname → `Kill(MaxInt32,0)`→ESRCH→dead→**REAPED**.
  (2) `live.lock`: pid=`os.Getpid()`, this hostname → alive→**SPARED**. (3) `foreign.lock`: pid=`os.Getpid()`,
  a bogus hostname → hostname-mismatch→conservative-true→**SPARED**. (4) `malformed.lock`: pid=`"not-a-number"`
  → `strconv.Atoi` error→`continue` (skip)→**SPARED**. Plus the holder's own `<sha256>.lock` (pid=`os.Getpid()`
  set by Acquire) → live→**SPARED**. `math.MaxInt32` is a guaranteed-dead pid (pid_t is int32; pid_max ≪ 2^31)
  — simpler than S1's fork+wait and the contract's choice. See research §4.

  ⚠️ **THE planting sequence — isolate XDG → resolve lockDir() → MkdirAll → write fakes → Acquire.** The fakes
  must be `*.lock` files in the SAME dir Acquire reaps (`filepath.Dir(path)`). Isolate via
  `t.Setenv("XDG_RUNTIME_DIR", t.TempDir())` + `t.Setenv("XDG_CACHE_HOME","")` (the TestRelease_RemovesLockFile
  pattern); resolve the dir via the package's own `lockDir()` (white-box); `os.MkdirAll(dir, 0o700)` before
  planting (Acquire's own MkdirAll is then a no-op). Filename collision with the holder's own `<64-hex>.lock`
  is impossible (fakes are named `dead.lock`/`live.lock`/etc.). See research §3.

  ⚠️ **THE read-before-Release composition (Issue 2).** `Release()` removes the holder's OWN lock file (Issue 2,
  landed). Assert `l.path` (the holder's own file) is present BEFORE `l.Release()`. The fake-file assertions
  (dead/live/foreign/malformed) are unaffected by Release. See research §6.

  ⚠️ **THE 3 contract scenarios → 2 tests.** (a)+(b) → `TestAcquire_ReapsDeadPidFile_SparesLive` (plant all 4
  fakes → Acquire → dead REAPED; live/foreign/malformed + own SPARED). (c) → `TestAcquire_ReapingIdempotent`
  (plant only survivors, no dead → Acquire+Release → Acquire again → all survivors STILL PRESENT; nothing
  re-reaped). See research §5.

  Deliverable: edits to ONE file — `internal/lock/lock_unix_test.go` (`//go:build !windows`) — adding a
  `writeLockFile` helper + 2 tests + import growth (`fmt`/`math`/`path/filepath`/`strconv`). NO production
  change, NO lock.go/lock_test.go/lock_unix.go/lock_windows.go edits, NO signal/main, NO docs, NO go.mod.
  INPUT = reapStaleLocks + processAlive + Acquire + lockDir (all LANDED). OUTPUT = test coverage proving
  stale files are reaped correctly and live files are never touched. DOCS = none (test-only).
---

## Goal

**Feature Goal**: Prove, via committed unit tests, that `Acquire`'s stale lock-FILE reaping is correct and
safe: a dead-pid orphan file is REMOVED; a live-pid file (this host), a foreign-hostname file, and a
malformed-pid file are all SPARED; the just-acquired holder's own file is SPARED; and reaping is idempotent
(a second Acquire with no new dead files reaps nothing).

**Deliverable** (edit to 1 existing file — `internal/lock/lock_unix_test.go`, `//go:build !windows`):
1. A `writeLockFile(t, path, pid, hostname)` helper that writes a minimal lock-file-shaped string.
2. `TestAcquire_ReapsDeadPidFile_SparesLive` — plants dead/live/foreign/malformed fakes, calls `Acquire`,
   asserts dead REAPED + live/foreign/malformed/own SPARED (contract a + b).
3. `TestAcquire_ReapingIdempotent` — plants only survivors, two `Acquire`s, asserts nothing re-reaped
   (contract c).
4. Import growth: add `fmt`, `math`, `path/filepath`, `strconv` to the existing `os`/`os/exec`/`testing`.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/` clean; `go test -race
./internal/lock/` green — the 2 new tests pass AND all existing lock tests stay green;
`GOOS=windows go test ./internal/lock/` green (the new tests are `//go:build !windows` → excluded on Windows,
so Windows CI is unaffected); `go test -race ./...` green (no regression). go.mod/go.sum unchanged; only
`internal/lock/lock_unix_test.go` touched.

## User Persona

**Target User**: The maintainer who needs confidence that the §18.5 reaping mechanism (landed by P1.M2.T1.S2)
actually reaps dead holders' orphaned files while NEVER touching a live holder's file (the FR52 "never
force-break" / "never reap a live pid" invariant). Transitively, every user whose lock directory would
otherwise accumulate litter.

**Use Case**: A user Ctrl-C's a run (or it crashes), orphaning a lock file with a now-dead pid; the next
`stagehand` run's `Acquire` reaps it. These tests prove that reaping happens (dead pid) and that it is safe
(live/foreign/malformed files are never touched).

**User Journey**: (test-only; no user surface) plant fixtures → `Acquire(repo)` → `reapStaleLocks` runs →
assert the dead-pid file is gone and every other file survives.

**Pain Points Addressed**: removes the "does the reaping actually work / could it ever delete a live holder's
file" uncertainty by pinning each `processAlive` branch with a dedicated fixture.

## Why

- **Closes the test-coverage gap for P1.M2.T1.S2.** S2 shipped `reapStaleLocks` + the Acquire wiring with NO
  committed tests (P1.M2.T3.S1 owns them — this task). The reaping logic is unverified until these land.
- **Pins the safety invariant.** The "never reap a live pid" guarantee is the keystone of safe reaping
  (unlinking a live holder's inode-bound-flock file would let a contender `O_CREATE`+flock a fresh inode and
  defeat FR52). Each `processAlive` branch (dead/live/foreign/malformed) gets a dedicated fixture, so a
  regression in any branch fails a specific test.
- **Idempotency matters.** Reaping runs on EVERY Acquire; a second Acquire must not re-reap (or somehow
  corrupt) surviving files. The idempotency test pins this.
- **No production/doc/dep change.** Test-only; DOCS: none. The production code (S1+S2) is frozen.

## What

Two committed tests + one helper in `internal/lock/lock_unix_test.go`. No production change, no new files,
no docs, no go.mod.

### Success Criteria

- [ ] `internal/lock/lock_unix_test.go` has `//go:build !windows` (already present from S1) + imports
      `fmt`, `math`, `os`, `os/exec`, `path/filepath`, `strconv`, `testing`.
- [ ] `writeLockFile(t, path, pid, hostname)` writes `pid=%s\nhostname=%s\nrepo=fake\ntimestamp=fake\nsnapshot=\n`.
- [ ] `TestAcquire_ReapsDeadPidFile_SparesLive`: isolates XDG; resolves `lockDir()`; MkdirAll; plants
      `dead.lock` (pid=`math.MaxInt32`, this host), `live.lock` (pid=`os.Getpid()`, this host),
      `foreign.lock` (pid=`os.Getpid()`, bogus host), `malformed.lock` (pid=`"not-a-number"`, this host);
      `Acquire(repo)`; asserts `dead.lock` GONE (`os.IsNotExist`); `live.lock`/`foreign.lock`/`malformed.lock`
      + the holder's `l.path` PRESENT; then `l.Release()`.
- [ ] `TestAcquire_ReapingIdempotent`: plants only `live.lock`/`foreign.lock`/`malformed.lock` (no dead);
      `Acquire`+`Release`; `Acquire` again; asserts all 3 survivors STILL PRESENT (nothing re-reaped);
      `l.Release()`.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/`, `go test -race ./internal/lock/`,
      `go test -race ./...` clean/green; `GOOS=windows go test ./internal/lock/` green (new tests excluded
      on Windows via the build tag); go.mod/go.sum unchanged; only `lock_unix_test.go` touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact production behavior
of `reapStaleLocks`/`processAlive` (quoted), the fixture table (pid/hostname/expected-outcome per branch),
the planting sequence (XDG isolate → lockDir → MkdirAll → write fakes → Acquire), the two test sketches
(with verbatim code), and the cross-platform placement rationale (lock_unix_test.go, NOT lock_test.go). No
PRD/git-internals knowledge beyond "flock auto-releases; orphaned files are reaped by pid-liveness."

### Documentation & References

```yaml
# MUST READ — the authoritative research (every fixture + the placement decision + the test sketches)
- docfile: plan/011_98cef660a41d/P1M2T3S1/research/reaping-tests-design.md
  why: the landed production code under test (§1), the cross-platform placement decision (§2 — WHY
       lock_unix_test.go not lock_test.go), the planting sequence + the writeLockFile helper (§3), the
       fixture table (§4 — math.MaxInt32 dead / os.Getpid live / foreign host / malformed pid), the 3
       contract scenarios → 2 tests (§5), the read-before-Release composition (§6), imports (§7).
  critical: §2 (the dead-pid-removed assertion is Unix-only; lock_test.go placement breaks Windows CI) and
       §4 (the exact pid/hostname values per fixture) are the things most likely to be done wrong.

# THE production code under test (LANDED — read, do NOT edit)
- file: internal/lock/lock.go
  section: reapStaleLocks(dir) (~L140) + the Acquire wiring (~L107, `reapStaleLocks(filepath.Dir(path))`)
       + parseContents (~L211) + writeContents (the `pid=%s\nhostname=%s\n...` format) + lockDir (~L165).
  why: the code these tests exercise. reapStaleLocks globs `*.lock`, parses c.Pid/c.Hostname, Atoi-continues
       on a malformed pid, and `os.Remove`s when processAlive is false. Acquire calls it after current.Store.
       lockDir resolves `<xdg>/stagehand/locks` — the dir to plant fakes in.
  pattern: the tests drive Acquire (the public entry point that triggers reaping) and assert on-disk state
       via os.Stat. They do NOT call reapStaleLocks directly (it is unexported — fine, white-box — but
       driving it through Acquire also covers the wiring).
  gotcha: parseContents reads `pid=`/`hostname=` lines; the fake files must use that exact key=value shape.
       Acquire MkdirAll's the lock dir, but planting needs it first → MkdirAll it yourself before writing fakes.

# THE processAlive semantics (the cross-platform keystone — S1, LANDED)
- file: internal/lock/lock_unix.go   (//go:build !windows)
  section: processAlive (S1).
  why: the Unix cascade — hostname==""||!=os.Hostname() → true; Kill(pid,0)==nil → true; EPERM → true;
       else (ESRCH) → false. This is what makes dead=MaxInt32 reapable and live=os.Getpid spared.
- file: internal/lock/lock_windows.go   (//go:build windows)
  section: processAlive (S1) — `return true` always.
  why: PROOF the dead-pid-removed assertion would FAIL on Windows (processAlive always true → dead-pid file
       NOT reaped). Hence the lock_unix_test.go placement.
  critical: do NOT place the dead-pid-removed assertion in a no-build-tag test file.

# THE test file being edited + the helpers/pattern to reuse
- file: internal/lock/lock_unix_test.go   (//go:build !windows — S1 created it for the 4 processAlive tests)
  why: the file you ADD the 2 reaping tests + writeLockFile to. Already has TestProcessAlive_* (incl. the
       dead-pid fork+wait test). Add the reaping tests alongside them (one home for dead-pid assertions).
  pattern: mirror S1's style (t.Helper(), t.Fatalf on setup, t.Errorf on assertions, t.Skip only on a fork
       failure — NOT used here since MaxInt32 needs no fork). Reuse resetCurrent(t) from lock_test.go.
  gotcha: add fmt/math/path/filepath/strconv to the imports (S1's file has os/os/exec/testing). os/exec stays
       (S1's dead-pid processAlive test uses it).
- file: internal/lock/lock_test.go   (no build tag — the helpers + pattern source; do NOT edit)
  section: resetCurrent(t) (~L9) + TestRelease_RemovesLockFile (~L190 — the XDG-isolation + read-before-
           Release pattern) + TestAcquireRelease_RoundTrip.
  why: the HELPERS/pattern the reaping tests reuse. resetCurrent(t) is visible to lock_unix_test.go on Unix
       (both compile). The XDG-isolation pattern (t.Setenv XDG_RUNTIME_DIR=t.TempDir() + clear XDG_CACHE_HOME)
       is mirrored verbatim. Read-before-Release (Issue 2 removes the holder's own file) is mirrored.
  gotcha: do NOT edit lock_test.go — the reaping tests go in lock_unix_test.go (cross-platform correctness;
       see research §2). resetCurrent is reused as-is.

# THE requirement + the reaping spec
- file: PRD.md §18.5 (h3.91) "Concurrency: the per-repo run lock (FR52)" — "Stale-file reaping (lock vs file)."
  why: the PRD source. flock auto-releases (the lock is never stale); the orphaned FILE is reaped by
       `kill(pid,0)`→ESRCH on Acquire; a LIVE pid is NEVER reaped (the "never force-break" guarantee);
       hostname-matching scopes reaping to this host; reaping by age/timestamp is rejected.
- file: plan/011_98cef660a41d/architecture/lock_reaping.md   "Fix 1: Stale-File Reaping in Acquire."
  why: the authoritative reapStaleLocks spec + the safety invariant ("a live pid is NEVER reaped …
       pid-liveness check is precisely what makes unlinking safe"). The tests pin this invariant per-branch.
```

### Current Codebase tree (relevant slice)

```bash
internal/lock/
  lock.go              # reapStaleLocks + Acquire wiring + parseContents + lockDir (LANDED, P1.M2.T1.S2) — NO edit
  lock_unix.go         # processAlive (LANDED, P1.M2.T1.S1) — NO edit
  lock_windows.go      # processAlive always-true stub (LANDED, P1.M2.T1.S1) — NO edit
  lock_test.go         # resetCurrent + TestRelease_RemovesLockFile + existing tests — NO edit (reuse helpers)
  lock_unix_test.go    # //go:build !windows — S1's 4 processAlive tests — EDIT (ADD writeLockFile + 2 reaping tests + imports)
go.mod / go.sum        # unchanged (stdlib only: fmt/math/os/os/exec/path/filepath/strconv/testing)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. ONE edit: internal/lock/lock_unix_test.go (+writeLockFile +2 tests +imports).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (cross-platform placement): the reaping tests go in lock_unix_test.go (//go:build !windows), NOT
// lock_test.go. The dead-pid-removed assertion requires Unix Kill→ESRCH; on Windows processAlive always
// returns true → the dead-pid file is NOT reaped → the assertion FAILS Windows CI. This mirrors S1's
// lock_unix_test.go placement for the processAlive dead-pid test. The contract's "lock_test.go" is overridden
// by this hard CI requirement. (resetCurrent + the XDG pattern from lock_test.go are still reused.)

// CRITICAL (the dead-pid fixture = math.MaxInt32, NOT fork+wait): pid=2147483647, this hostname. pid_t is
// int32; pid_max ≪ 2^31 → kill(MaxInt32,0) → ESRCH → dead → reaped. strconv.Atoi("2147483647") succeeds on
// 32- and 64-bit. Simpler than S1's fork+wait (no child to manage) and the contract's choice. Do NOT use
// os.Getpid() for the dead fixture (that's the LIVE one).

// CRITICAL (plant BEFORE Acquire, in the resolved lockDir): the fakes must be *.lock files in the dir Acquire
// reaps (filepath.Dir(path) = lockDir()). Isolate XDG → lockDir() → os.MkdirAll(dir,0o700) → write fakes →
// Acquire. Acquire's own MkdirAll is then a no-op. Filename collision with the holder's <64-hex>.lock is
// impossible (fakes are dead.lock/live.lock/etc.).

// CRITICAL (read-before-Release for the holder's own file): Release() removes l.path (Issue 2). Assert
// l.path is present BEFORE l.Release(). The fake-file assertions are unaffected by Release.

// GOTCHA (the malformed-pid fixture proves the Atoi-continue branch): pid="not-a-number" → strconv.Atoi
// errors → reapStaleLocks `continue`s (skips, no Remove). Assert malformed.lock is STILL PRESENT. This is
// the contract's (b) best-effort case. parseContents happily returns c.Pid="not-a-number" (it doesn't parse
// the value); the Atoi gate is what skips it.

// GOTCHA (the foreign-hostname fixture proves the conservative branch): a BOGUS hostname (e.g.
// "definitely-not-this-host-zzz") with an alive pid → processAlive returns true (hostname mismatch → don't
// reap). Assert foreign.lock STILL PRESENT. This pins "hostname-matching scopes reaping to this host."

// GOTCHA (lock_unix_test.go imports): S1's file imports os/os/exec/testing. ADD fmt (Sprintf), math (MaxInt32),
// path/filepath (Join), strconv (Itoa(os.Getpid())). os/exec STAYS (S1's dead-pid processAlive test uses it;
// removing it would be an unused-import error). All stdlib; go.mod unchanged.

// GOTCHA (do NOT call reapStaleLocks directly): drive reaping through Acquire (the public entry that wires
// it). This also covers the Acquire wiring (filepath.Dir(path)), not just the function in isolation.

// GOTCHA (resetCurrent is in lock_test.go, reusable on Unix): lock_test.go (no build tag) + lock_unix_test.go
// both compile on Unix → resetCurrent(t) is in scope. Do NOT redefine it. Call resetCurrent(t) at the top of
// each reaping test (the `current` singleton is process-global; existing lock tests are sequential, no Parallel).

// GOTCHA (no t.Parallel): the `current` singleton + the shared lock dir make these tests sequential. Mirror
// the existing lock tests (none call t.Parallel).
```

## Implementation Blueprint

### Data models and structure

No new types. One helper + two tests.

```go
// internal/lock/lock_unix_test.go — ADD to the existing file (S1's processAlive tests stay).
// Imports grow: fmt, math, os, os/exec, path/filepath, strconv, testing.

// writeLockFile writes a minimal lock file at path with the given pid/hostname — the two fields
// reapStaleLocks reads (via parseContents). The repo/timestamp/snapshot values are filler (parseContents
// reads them but reapStaleLocks ignores them). Used by the reaping tests to plant fixture orphan files.
func writeLockFile(t *testing.T, path, pid, hostname string) {
	t.Helper()
	content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=fake\ntimestamp=fake\nsnapshot=\n", pid, hostname)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: lock_unix_test.go — grow the imports + add writeLockFile
  - ADD "fmt", "math", "path/filepath", "strconv" to the import block (keep os, os/exec, testing).
  - ADD writeLockFile(t, path, pid, hostname) per the Data Models block.
  - GOTCHA: os/exec STAYS (S1's TestProcessAlive_DeadPID uses it). All stdlib; go.mod unchanged.

Task 2: lock_unix_test.go — ADD TestAcquire_ReapsDeadPidFile_SparesLive (contract a + b)
  - ADD the test per the sketch below: resetCurrent(t); isolate XDG; dir,_ := lockDir(); MkdirAll(dir);
    thisHost,_ := os.Hostname(); plant dead.lock (MaxInt32, thisHost) + live.lock (Getpid, thisHost) +
    foreign.lock (Getpid, bogus) + malformed.lock ("not-a-number", thisHost); l,_ := Acquire(repo); assert
    dead.lock GONE (os.IsNotExist); live/foreign/malformed + l.path PRESENT; l.Release().
  - ASSERTIONS: os.Stat + os.IsNotExist (reaped) / err==nil (present). Assert l.path BEFORE Release.
  - GOTCHA: math.MaxInt32 for dead; os.Getpid for live; bogus hostname for foreign; "not-a-number" for malformed.
    MkdirAll before planting. resetCurrent at top. No t.Parallel.

Task 3: lock_unix_test.go — ADD TestAcquire_ReapingIdempotent (contract c)
  - ADD the test per the sketch below: resetCurrent(t); isolate XDG; lockDir + MkdirAll; plant ONLY survivors
    (live/foreign/malformed, no dead); l1,_ := Acquire(repo); l1.Release(); snapshot the 3 survivor paths;
    l2,_ := Acquire(repo); assert all 3 survivors STILL PRESENT (nothing re-reaped); l2.Release().
  - GOTCHA: the first Acquire reaps nothing (no dead file); the second reaps nothing again. The assertion is
    "the survivor set is stable across two Acquires."

Task 4: VERIFY (no further edits)
  - RUN `gofmt -w internal/lock/lock_unix_test.go`; `go vet ./internal/lock/`; `go build ./...`;
    `go test -race ./internal/lock/ -v -run 'TestAcquire_Reap'`; `go test -race ./...`;
    `GOOS=windows go test ./internal/lock/` (new tests excluded → green).
  - go.mod/go.sum byte-unchanged. Only lock_unix_test.go touched. lock.go/lock_test.go/lock_unix.go/
    lock_windows.go/signal/main/docs byte-unchanged.
```

### Test Specs (lock_unix_test.go — 2 new tests + helper)

```go
// (imports — grow the existing block:)
import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// writeLockFile (the Data Models helper — see above).

// TestAcquire_ReapsDeadPidFile_SparesLive verifies §18.5 stale-FILE reaping (P1.M2.T1.S2): Acquire removes
// orphaned *.lock files whose recorded pid is DEAD, while SPARING live-pid files (this host), foreign-hostname
// files (conservative), malformed-pid files (Atoi-skip), and the just-acquired holder's own file. Each fixture
// pins one processAlive branch (P1.M2.T1.S1): dead=ESRCH→false; live=Kill-nil→true; foreign=hostname-mismatch→true;
// malformed=Atoi-error→continue. Unix-only (//go:build !windows) because the dead-pid-removed assertion
// requires Kill→ESRCH (Windows processAlive is always-true → the dead file would NOT be reaped → the
// assertion would fail Windows CI; mirrors S1's lock_unix_test.go placement for the processAlive dead-pid test).
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

	writeLockFile(t, deadPath, strconv.Itoa(math.MaxInt32), thisHost) // MaxInt32 ≫ pid_max → ESRCH → dead
	writeLockFile(t, livePath, strconv.Itoa(os.Getpid()), thisHost)  // self → alive
	writeLockFile(t, foreignPath, strconv.Itoa(os.Getpid()), "definitely-not-this-host-zzz-999") // alive, foreign host
	writeLockFile(t, malformedPath, "not-a-number", thisHost)        // Atoi error → skip

	repo := t.TempDir()
	l, err := Acquire(repo) // creates <hash>.lock (holder's own, live → spared) + triggers reapStaleLocks(dir)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// (a) dead-pid file REAPED; live/foreign/own SPARED.
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
	// The holder's own file is SPARED (its pid is os.Getpid, set by Acquire). Assert BEFORE Release (Issue 2
	// removes l.path on Release).
	if _, err := os.Stat(l.path); err != nil {
		t.Errorf("holder's own lock file should be PRESENT, missing: %v", err)
	}

	l.Release()
}

// TestAcquire_ReapingIdempotent verifies contract (c): a second Acquire on the same repo (after Release)
// with no new dead files does NOT re-reap anything — the surviving fixtures (live/foreign/malformed) are
// stable across two Acquire passes. Reaping runs on every Acquire; this pins that it is a no-op on the
// survivor set when nothing new has died. (Unix-only for cohesion with the dead-pid test; the outcome is
// cross-platform-safe but reaping is a Unix concept — Windows is a documented no-op.)
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
```

### Implementation Patterns & Key Details

```go
// THE fixture → processAlive-branch mapping (each fixture pins one branch):
//   dead.lock     pid=MaxInt32,  thisHost  → Kill→ESRCH→false          → REAPED   (the reaping trigger)
//   live.lock     pid=Getpid,    thisHost  → Kill→nil→true             → SPARED   (alive)
//   foreign.lock  pid=Getpid,    bogus     → hostname!=thisHost→true   → SPARED   (conservative)
//   malformed.lock pid="not-a-number",thisHost → Atoi error→continue   → SPARED   (best-effort skip)
//   holder's own  pid=Getpid (Acquire sets it), thisHost → true        → SPARED   (live)

// THE planting sequence (fakes must be in the dir Acquire reaps):
//   t.Setenv("XDG_RUNTIME_DIR", t.TempDir()); t.Setenv("XDG_CACHE_HOME", "")
//   dir, _ := lockDir()                       // → <tmpdir>/stagehand/locks (white-box)
//   os.MkdirAll(dir, 0o700)                   // before planting; Acquire's MkdirAll is then a no-op
//   writeLockFile(t, filepath.Join(dir,"dead.lock"), ...)  // key=value format parseContents reads
//   l, _ := Acquire(repo)                     // creates <hash>.lock + triggers reapStaleLocks(dir)

// WHY MaxInt32 (not fork+wait): pid_t is int32; pid_max ≪ 2^31 → kill(MaxInt32,0) is guaranteed ESRCH.
// strconv.Atoi("2147483647") succeeds. No child process to manage — simpler + deterministic. (S1's
// processAlive unit test used fork+wait; the reaping test uses MaxInt32 per the contract — both valid.)

// THE read-before-Release composition (Issue 2): Release removes l.path. Assert l.path present BEFORE Release.
// (The fake-file assertions are unaffected — Release only touches l.path.)
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — stdlib only (fmt/math/os/os/exec/path/filepath/strconv/testing).
      go mod tidy is a no-op.

PACKAGE EDGES: NONE. lock_unix_test.go is `package lock` (white-box), build-tag `!windows`. It reuses
      resetCurrent (lock_test.go) + lockDir/reapStaleLocks/processAlive/Acquire (the package).

FROZEN / NOT-EDITED:
  - internal/lock/lock.go (reapStaleLocks + Acquire wiring — P1.M2.T1.S2, LANDED).
  - internal/lock/lock_unix.go + lock_windows.go (processAlive — P1.M2.T1.S1, LANDED).
  - internal/lock/lock_test.go (resetCurrent + existing tests — reused, not edited).
  - internal/signal/* + cmd/stagehand/main.go (P1.M2.T2 exit-path release — different concern; P1.M2.T3.S2
    owns the signal-seam tests).
  - docs/* (DOCS: none — test-only).
  - go.mod / go.sum.

DOWNSTREAM / RELATED (do NOT implement here):
  - P1.M2.T3.S2 (exit-path release signal tests): injects a recording OnRescueExit, fakes the lock, asserts
    OnRescueExit fires before Exit on both branches. Different file (signal_test.go); different concern.

NO DATABASE / ROUTES / CONFIG / CLI / NEW FILES / DOCS / PRODUCTION CHANGE.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/lock/lock_unix_test.go
test -z "$(gofmt -l internal/lock/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/lock/   # catches an unused import (e.g. dropping os/exec) / a malformed helper.
go build ./...            # whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm the build tag is present (Unix-only — the dead-pid assertion requires it):
head -1 internal/lock/lock_unix_test.go   # → //go:build !windows
# Confirm only lock_unix_test.go changed:
git diff --name-only | grep -v '^internal/lock/lock_unix_test\.go$' && echo "UNEXPECTED file changed" || echo "only lock_unix_test.go changed (good)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 2 new reaping tests + S1's processAlive tests + the existing lock tests:
go test -race ./internal/lock/ -v -run 'TestAcquire_Reap'
# Expected PASS:
#   TestAcquire_ReapsDeadPidFile_SparesLive .... dead.lock REAPED; live/foreign/malformed + own SPARED.
#   TestAcquire_ReapingIdempotent ............... survivors stable across two Acquires.
go test -race ./internal/lock/    # full lock suite (S1's processAlive + existing lockDir/Acquire/Release/SetSnapshot).
go test -race ./...               # full module — no regression.
# Cross-platform gate — the new tests are //go:build !windows → excluded on Windows → Windows CI stays green:
GOOS=windows go vet ./internal/lock/ && echo "windows vet OK"
GOOS=windows go test ./internal/lock/ && echo "windows test OK (reaping tests excluded by build tag)"
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagehand ./cmd/stagehand && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm the production code + lock_test.go + platform files are byte-unchanged (test-only task):
git diff --exit-code -- internal/lock/lock.go internal/lock/lock_unix.go internal/lock/lock_windows.go internal/lock/lock_test.go internal/signal cmd/stagehand docs && echo "production + lock_test.go + signal/main/docs UNCHANGED (expected)"
# Confirm the 2 new tests + helper landed:
grep -n 'func writeLockFile\|func TestAcquire_ReapsDeadPidFile_SparesLive\|func TestAcquire_ReapingIdempotent' internal/lock/lock_unix_test.go
```

### Level 4: Creative & Domain-Specific Validation

```bash
# THE lint gate — the new tests call Acquire/writeLockFile/processAlive(via Acquire) → no unused:
make lint 2>&1 | grep -iE 'lock_unix_test|unused|U1000' && echo "BAD: flagged" || echo "not flagged (good)"
# Fixture audit — confirm each processAlive branch is pinned by exactly one fixture:
grep -c 'writeLockFile' internal/lock/lock_unix_test.go   # → 4 (dead/live/foreign/malformed) in the dead-pid test
# Safety-invariant audit — confirm the live/foreign/own files are asserted PRESENT (never reaped):
grep -c 'SPARED\|should be SPARED\|should be PRESENT\|should be SKIPPED' internal/lock/lock_unix_test.go
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1 clean: `gofmt -l internal/lock/`, `go vet ./internal/lock/`, `go build ./...`, `go mod tidy` no-op;
      `//go:build !windows` present; only `lock_unix_test.go` changed.
- [ ] Level 2 green: the 2 new tests pass; `go test -race ./internal/lock/` + `./...` green;
      `GOOS=windows go test ./internal/lock/` green (new tests excluded).
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; production + lock_test.go + signal/main/docs byte-unchanged.
- [ ] Level 4: `make lint` green (no unused); each processAlive branch pinned by a fixture.

### Feature Validation
- [ ] `TestAcquire_ReapsDeadPidFile_SparesLive`: dead.lock (MaxInt32) REAPED; live.lock (Getpid) SPARED;
      foreign.lock (bogus host) SPARED; malformed.lock ("not-a-number") SKIPPED; holder's own l.path SPARED.
- [ ] `TestAcquire_ReapingIdempotent`: survivor set (live/foreign/malformed) stable across two Acquires.
- [ ] Each `processAlive` branch (ESRCH-dead / Kill-nil-alive / hostname-mismatch / Atoi-skip) pinned by a fixture.

### Code Quality Validation
- [ ] Tests mirror S1's lock_unix_test.go style (t.Helper/Fatalf/Errorf, resetCurrent, XDG isolation, read-before-Release).
- [ ] Reuses `resetCurrent` (lock_test.go) + `lockDir()`/`Acquire` (the package); no re-implementation.
- [ ] `writeLockFile` is a focused helper (the exact key=value format parseContents reads).
- [ ] No scope creep into lock.go/lock_test.go/platform files/signal/main/docs/production code.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment
- [ ] No docs (DOCS: none — test-only; P1.M3 owns the changeset doc sync).
- [ ] go.mod/go.sum byte-unchanged; no new files; no production change.

---

## Anti-Patterns to Avoid

- ❌ **Don't put the dead-pid-removed assertion in `lock_test.go` (no build tag).** On Windows `processAlive`
  always returns `true` → the dead-pid file is NOT reaped → the assertion FAILS Windows CI. Put the reaping
  tests in `lock_unix_test.go` (`//go:build !windows`), mirroring S1. The contract's "lock_test.go" is
  overridden by this hard CI requirement. (resetCurrent + the XDG pattern from lock_test.go are still reused.)
- ❌ **Don't use `os.Getpid()` for the dead-pid fixture.** That's the LIVE one. Use `math.MaxInt32`
  (2147483647) — guaranteed ≫ pid_max → `kill(MaxInt32,0)`→ESRCH→dead. `strconv.Itoa(math.MaxInt32)` is the
  pid string to write.
- ❌ **Don't use fork+wait for the dead-pid fixture.** S1's processAlive unit test used fork+wait (valid),
  but the reaping test uses `math.MaxInt32` per the contract — simpler (no child to manage) + deterministic.
  Don't conflate the two.
- ❌ **Don't plant fakes in the wrong dir.** They must be `*.lock` in the dir Acquire reaps (`filepath.Dir(path)`
  = `lockDir()`). Isolate XDG → `lockDir()` → `os.MkdirAll` → plant → `Acquire`. Planting in `t.TempDir()`
  itself (not the `stagehand/locks` subdir) means reapStaleLocks never sees them.
- ❌ **Don't forget to `os.MkdirAll(dir, 0o700)` before planting.** Acquire MkdirAll's the dir, but planting
  needs it first. (Acquire's MkdirAll is then a no-op.)
- ❌ **Don't assert `l.path` present AFTER `l.Release()`.** Release removes the holder's OWN file (Issue 2).
  Assert `l.path` BEFORE Release. (Fake-file assertions are unaffected.)
- ❌ **Don't call `reapStaleLocks` directly.** Drive reaping through `Acquire` — it covers the wiring
  (`reapStaleLocks(filepath.Dir(path))`) too, not just the function in isolation.
- ❌ **Don't drop `os/exec` from the imports.** S1's `TestProcessAlive_DeadPID` (already in lock_unix_test.go)
  uses it; removing it is an unused-import error. ADD fmt/math/path/filepath/strconv; KEEP os/os/exec/testing.
- ❌ **Don't redefine `resetCurrent`.** It lives in lock_test.go (no build tag) and is visible to
  lock_unix_test.go on Unix. Reuse it.
- ❌ **Don't call `t.Parallel()`.** The `current` singleton + the shared isolated lock dir make these tests
  sequential (mirror the existing lock tests).
- ❌ **Don't edit production code.** This is test-only. reapStaleLocks/processAlive/Acquire are LANDED (S1+S2).
  Editing them overlaps P1.M2.T1. Do NOT "fix" the production code from a test task.
- ❌ **Don't touch lock_test.go, lock.go, lock_unix.go, lock_windows.go, signal/main, or docs.** The SOLE edit
  is `internal/lock/lock_unix_test.go`. resetCurrent is reused as-is.
- ❌ **Don't change go.mod/go.sum or add files.** One helper + two tests + imports, all in one existing file.
- ❌ **Don't skip `GOOS=windows go test ./internal/lock/`.** It proves the build tag excludes the new tests
  (Windows CI green). A no-build-tag dead-pid test would fail there — the build tag is the gate.

---

## Confidence Score

**9/10** — a test-only task against LANDED production code (reapStaleLocks + processAlive + the Acquire wiring
all verified in-tree), with each `processAlive` branch pinned by a dedicated fixture (the contract's exact
4-case design), a deterministic dead-pid (`math.MaxInt32` → guaranteed ESRCH, no fork), verbatim test
sketches, and a clearly-justified cross-platform placement (lock_unix_test.go, not lock_test.go — the one
real deviation from the contract, mandated by Windows CI correctness and S1's precedent). The -1 reserves for
the placement deviation itself (an implementer following the contract literally would put it in lock_test.go
and break Windows CI — the PRP's repeated call-outs mitigate this, but it's the one judgment call) and the
minor import-growth care (keeping os/exec).
