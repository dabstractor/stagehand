name: "P1.M1.T2.S1 — Export signal.Trigger(sig): the parent-death watchdog's programmatic rescue entry"
description: >
  Add ONE exported function `func Trigger(sig os.Signal)` to internal/signal/signal.go that routes a
  synthetic signal through the existing rescue/exit path (`active.Load()` → `h.handle(sig)`). It is the
  exact idiom of the file's other nil-safe package wrappers (RegisterChild/SetSnapshot/RestoreDefault),
  delegating to the signal-agnostic `handle()` — so the stopped-guard (first line of handle) makes it a
  no-op after RestoreDefault, and the `active==nil` check makes it a no-op in library use. The parent-death
  watchdog (P1.M2.T2.S1) will call `signal.Trigger(syscall.SIGTERM)` on detected parent death. Plus a
  godoc comment + 6 cross-platform unit tests. Purely additive — no handle/run/RestoreDefault changes.

---

## Goal

**Feature Goal**: Give code OUTSIDE the signal package (specifically the upcoming parent-death watchdog,
P1.M2.T2.S1) a programmatic way to invoke the rescue/exit path, because `handle()` is unexported. The
new `signal.Trigger(sig os.Signal)` is a nil-safe, stopped-guarded wrapper around `active.Load().handle(sig)`
that reuses the exact same forward-to-child → cancel-ctx → rescue-or-exit (+ lock-release) logic the
OS-signal path uses — with zero new rescue logic.

**Deliverable**:
1. **internal/signal/signal.go** — a new exported `func Trigger(sig os.Signal)` (≈3 lines + godoc), placed
   immediately after `Active()`, following the file's `if h := active.Load(); h != nil { ... }` idiom.
2. **internal/signal/signal_test.go** — 6 cross-platform unit tests proving Trigger routes through `handle`
   (SIGTERM→143, SIGINT→130), arms rescue (snapshot→3 + rescue print + OnRescueExit), forwards to the
   child group, is a no-op after `RestoreDefault` (the stopped guard), and is a nil-safe no-op with no
   handler installed (library use).
3. No external docs change (Mode A godoc only).

**Success Definition**:
- `signal.Trigger(syscall.SIGTERM)` with a handler installed + no snapshot → the injected `Exit` is called
  with **143** (proves the export reaches `handle`, not just the method existing).
- `signal.Trigger(syscall.SIGTERM)` with a snapshot armed → `Exit(3)` + rescue message printed +
  `OnRescueExit` fires (the FR52 lock-release seam fires through Trigger).
- `signal.Trigger(syscall.SIGTERM)` after `RestoreDefault()` → `Exit`/`Kill` **NOT** called (stopped guard).
- `signal.Trigger(syscall.SIGTERM)` when `active==nil` → no panic, no-op (library use).
- `go build ./...`, `GOOS=windows go build ./...`, `GOOS=linux go build ./...` all clean; `make test` +
  `make lint` pass; `gofmt -l` empty.

## User Persona (if applicable)

**Target User**: Stagecoach internals — specifically the parent-death watchdog (P1.M2.T2.S1), which runs
OUTSIDE the signal package and cannot call the unexported `handle()` directly. (End users never call Trigger.)

**Use Case**: The watchdog's getppid()-polling fallback detects that the launching terminal/IDE/lazygit
died (parent PID changed to 1 / init). It needs to tear the run down through the SAME rescue/lock-release
path a Ctrl-C uses — so it calls `signal.Trigger(syscall.SIGTERM)`. (The Linux `prctl(PR_SET_PDEATHSIG)`
fast path delivers a real SIGTERM that flows through `run`→`handle` naturally and does not need Trigger.)

**User Journey**: parent dies → watchdog detects → `signal.Trigger(SIGTERM)` → `active.handle(SIGTERM)` →
forward SIGTERM to child group → cancel ctx → rescue (if snapshot armed, exit 3) or exit 143 →
`OnRescueExit` releases the lock file before exit (no orphaned lock for §9.27 reaping).

**Pain Points Addressed**: Without `Trigger`, the watchdog would have to either duplicate the rescue logic
(violating the single-rescue-path invariant) or reach into unexported state (impossible from another
package). `Trigger` is the minimal, idiom-matching export that lets external code drive the existing path.

## Why

- **FR-K1 enabler / §9.27 / §18.4**: The orphaned-run lock-reclamation feature (P1) needs the watchdog to
  detect parent death and trigger rescue. The watchdog lives in a different package and cannot call the
  unexported `handle()` (signal.go:134). `Trigger` is the ONLY new public API the watchdog needs from the
  signal package (per architecture/signal_extension.md §"FR-K1 extension").
- **Single rescue path**: routing through `handle()` (not duplicating it) preserves the invariant that
  there is ONE forward→cancel→rescue→exit routine with ONE lock-release seam (`OnRescueExit`). A second
  ad-hoc teardown in the watchdog would risk diverging exit codes, missed lock release, or missed child
  forwarding. `Trigger` makes the watchdog reuse the proven path.
- **Bounded, decoupled scope**: this is a 3-line additive export + tests. It does NOT change `handle()`,
  `run()`, `RestoreDefault()`, the caught-signal set (that's parallel P1.M1.T1.S1), the watchdog (P1.M2),
  or the lock (P1.M3). It can land independently of all of them — it only depends on `active`/`handle`,
  which pre-date this whole milestone.

## What

**User-visible behavior**: None directly. (The watchdog that consumes Trigger is a later task.) Internally,
`signal.Trigger` becomes the programmatic entry point to the rescue path, idempotent and safe at every
lifecycle stage (no handler / handler active / handler stopped).

**Technical change (additive, ~3 lines):**
```go
// Trigger routes a synthetic signal through the rescue/exit path — the entry point the parent-death
// watchdog (P1.M2.T2.S1) uses to tear a run down when it detects the launcher died. It delegates to the
// same handle() the OS-signal goroutine uses, so forward-to-child / cancel / rescue-or-exit / lock-release
// are identical. Nil-safe (no-op when no handler is installed — library use) and stopped-guarded (no-op
// after RestoreDefault — handle()'s first line checks h.stopped), so it is safe to call at any lifecycle
// stage, including the update-ref window.
func Trigger(sig os.Signal) {
	if h := active.Load(); h != nil {
		h.handle(sig)
	}
}
```

### Success Criteria
- [ ] `Trigger(syscall.SIGTERM)` w/ handler, no snapshot → injected `Exit(143)`.
- [ ] `Trigger(os.Interrupt)` w/ handler, no snapshot → injected `Exit(130)` (signal-agnostic).
- [ ] `Trigger(syscall.SIGTERM)` w/ snapshot armed → `Exit(3)` + rescue printed + `OnRescueExit` fired once.
- [ ] `Trigger(syscall.SIGTERM)` w/ registered child → `Kill(pid, SIGTERM)` called.
- [ ] `Trigger(syscall.SIGTERM)` after `RestoreDefault()` → `Exit`/`Kill` NOT called (stopped guard).
- [ ] `Trigger(syscall.SIGTERM)` when `active==nil` → no panic (library use).
- [ ] `go build ./...`, `GOOS=windows/linux/darwin go build ./...` clean; `make test` + `make lint` pass.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact 3-line function, its placement anchor (after `Active()`), the idiom it mirrors (every
nil-safe wrapper), the proof that handle()/RestoreDefault need no changes (stopped guard), the 6 test cases
with the exact helper to reuse, the Windows-safety constraint on test signals, and the coordination note
with the parallel T1.S1 (which removes signal.go's syscall import — irrelevant to Trigger since Trigger
uses no syscall).

### Documentation & References

```yaml
- file: internal/signal/signal.go
  why: "The file to edit. var active @82; Active() @119 (placement anchor — add Trigger right after it);
        handle() @134 (the routine Trigger delegates to — signal-agnostic, NO change); the nil-safe
        wrappers @166-232 (the EXACT idiom to copy: 'if h := active.Load(); h != nil { h.<x> }');
        RestoreDefault @218 (sets stopped=true — the source of Trigger's no-op-after-Restore guarantee)."
  pattern: >
    Every wrapper follows:
        func RegisterChild(pid int) {
            if h := active.Load(); h != nil {
                h.childPID.Store(int64(pid))
            }
        }
    Trigger is identical in shape; it calls h.handle(sig) instead of a setter.
  critical: "Trigger takes os.Signal (NOT syscall.X) and references ONLY active + handle. It needs NO
             import changes. Do NOT add a syscall reference to signal.go (parallel task T1.S1 REMOVES the
             syscall import from signal.go; Trigger must remain syscall-free so it compiles after T1.S1)."

- file: internal/signal/signal_test.go
  why: "The UN-TAGGED cross-platform test file to extend. installTestHandler(t, opts) @13 (stores handler
        in active, t.Cleanup resets). The exact tests to mirror: ForwardsToChildGroup @33, RescueOnSignalWithSnapshot
        @64, Exit143SIGTERM @110, RestoreDefaultStopsForward @126 (the stopped-guard pattern), NilWrappersNoOp @150
        (the nil-safe pattern)."
  pattern: >
    h := installTestHandler(t, Options{Kill: ..., Exit: func(code int){exitCode=code}, Out: new(bytes.Buffer)})
    RegisterChild(1234)
    h.handle(syscall.SIGTERM)      // ← existing tests call the METHOD
    assert exitCode==143
    // Trigger tests call the PACKAGE-LEVEL function instead, proving active→handle delegation:
    //   Trigger(syscall.SIGTERM)
  critical: "signal_test.go is UN-TAGGED → compiles on Windows. Use ONLY syscall.SIGTERM and os.Interrupt
             (both exist cross-platform). NEVER syscall.SIGHUP (Windows-unsafe; that's T1.S1's signal)."

- docfile: plan/014_37208f58ffa2/architecture/signal_extension.md
  why: "The FR-K1 design. §'FR-K1 extension (signal.Trigger export)' gives the verbatim function, confirms
        the watchdog calls signal.Trigger(syscall.SIGTERM), and confirms the stopped-guard + OnRescueExit
        guarantees hold with zero changes to handle/RestoreDefault."
  section: "FR-K1 extension (signal.Trigger export)"

- docfile: plan/014_37208f58ffa2/P1M1T1S1/PRP.md
  why: "PARALLEL sibling that also edits signal.go. Its contract: swap line 103 to caughtSignals()...,
        REMOVE signal.go's syscall import, refresh comments. It does NOT touch active/Active/handle/wrappers.
        Trigger is syscall-free and additive → no conflict. Read it to confirm the non-overlap."

- docfile: plan/014_37208f58ffa2/P1M1T1S1/research/verification_deltas.md
  why: "Confirms T1.S1's exact signal.go edits (so you know which lines T1.S1 will have changed and that
        none of them are the Trigger function or its active/handle dependencies)."
```

### Current Codebase tree (relevant slice)

```bash
internal/signal/
  signal.go              # var active @82; Active() @119 ← ADD Trigger AFTER THIS; handle() @134 (no change);
                         # nil-safe wrappers @166-232 (the idiom); RestoreDefault @218 (stopped guard source)
  signal_unix.go         # build-tagged twins (T1.S1 adds caughtSignals here) — NOT touched by T2.S1
  signal_windows.go      #   "                                                              — NOT touched by T2.S1
  signal_test.go         # UN-TAGGED cross-platform tests — ADD the 6 Trigger tests here
  signal_integration_test.go  # //go:build !windows precedent — NOT needed for T2.S1 (tests are cross-platform)
cmd/stagecoach/main.go   # signal.Install(...) callsite — NO change (Trigger has no caller yet; watchdog is P1.M2)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/signal/signal.go        # MODIFY: +1 exported func Trigger(sig os.Signal) after Active() (≈3 lines + godoc)
internal/signal/signal_test.go   # MODIFY: +6 Trigger tests (reuse installTestHandler; call the package-level Trigger)
# (no new files; no other package touched)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (syscall-free): Trigger takes os.Signal and uses ONLY active + handle. Do NOT reference syscall
//   in signal.go for Trigger. Parallel task T1.S1 REMOVES signal.go's syscall import (line 103 was its only
//   user). If Trigger referenced syscall, it would either (a) re-add the import T1.S1 removed → merge churn,
//   or (b) break if T1.S1 lands first. os.Signal is already in scope (os is imported). Keep Trigger syscall-free.

// CRITICAL (reuse handle, do NOT duplicate): the ENTIRE point is one rescue path. Trigger MUST call h.handle(sig)
//   — NOT re-implement forward/cancel/rescue. A second teardown routine would diverge (exit codes, lock release,
//   child forwarding) and break the single-path invariant. handle() is signal-agnostic, so any os.Signal works.

// CRITICAL (the stopped guard is free — do NOT re-check): handle()'s FIRST line is `if h.stopped.Load(){return}`.
//   So Trigger(syscall.SIGTERM) after RestoreDefault() is ALREADY a no-op. Do NOT add a second stopped check in
//   Trigger (it would be redundant and misleading — implying the guard lives in Trigger). Test it, don't code it.

// CRITICAL (nil-safe is the wrapper idiom): use `if h := active.Load(); h != nil { h.handle(sig) }` — exactly
//   the RegisterChild/SetSnapshot/RestoreDefault shape. Do NOT call os.Exit / panic when active==nil; a no-op is
//   correct (library use of pkg/stagecoach never installs the handler — D4 library-safe).

// CRITICAL (test signal choice — Windows): signal_test.go is UN-TAGGED and compiles on Windows. Use syscall.SIGTERM
//   and os.Interrupt ONLY (both exist cross-platform). NEVER syscall.SIGHUP in these tests — it does not exist in
//   Go's Windows syscall pkg and would break `GOOS=windows go test`. (SIGHUP is T1.S1's signal, in signal_unix_*.)

// COORDINATION (parallel T1.S1): both tasks edit signal.go. Non-overlapping regions (T1.S1 = import block + line
//   103 + comments; T2.S1 = new func after Active()). If T1.S1 lands first, the Active() anchor may shift by a
//   few lines — place Trigger "immediately after the Active() function" by NAME, not by line number. If T2.S1
//   lands first, T1.S1's import removal is unaffected (Trigger adds no import). Either order is safe.
```

## Implementation Blueprint

### Data models and structure
None. No new types. One new exported function reusing the existing `Handler.handle` method and the
existing `active` singleton. The `Options`/`Handler` structs are unchanged.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/signal/signal.go — add Trigger after Active()
  - LOCATE func Active() (search "func Active()" — currently line 119). Insert Trigger IMMEDIATELY AFTER its
    closing brace, BEFORE the run() goroutine. (Anchor by symbol name, not line number — parallel T1.S1 may
    shift lines.)
  - ADD (verbatim, with the godoc):
        // Trigger routes a synthetic signal through the rescue/exit path. It is the programmatic entry point
        // the parent-death watchdog (PRD §9.27 FR-K1; P1.M2.T2.S1) uses to tear a run down when it detects the
        // launcher died: the watchdog calls signal.Trigger(syscall.SIGTERM) and the run unwinds exactly as if a
        // SIGTERM had arrived from the terminal — forward to the child process group, cancel the signal-aware
        // context, then rescue (exit 3, if a snapshot is armed) or plain exit (143 for SIGTERM), with
        // OnRescueExit releasing the lock file before exit on BOTH branches (FR52 §18.5).
        //
        // Trigger delegates to the same handle() the OS-signal goroutine uses, so there is exactly ONE rescue
        // path (no duplicated teardown). It is nil-safe (no-op when no handler is installed — library use of
        // pkg/stagecoach) and stopped-guarded (handle()'s first line checks h.stopped, so a call after
        // RestoreDefault — the update-ref window — is a no-op). Safe to call at any lifecycle stage.
        func Trigger(sig os.Signal) {
            if h := active.Load(); h != nil {
                h.handle(sig)
            }
        }
  - NAMING: Trigger (exported; PascalCase). Matches the exported-wrapper convention (Active, RegisterChild,
    SetSnapshot, RestoreDefault). Parameter name `sig` matches handle()'s parameter.
  - NO IMPORT CHANGES: Trigger uses os.Signal (os already imported) and active/handle (same package). Do NOT
    add syscall. DEPENDENCIES: none beyond the pre-existing active singleton + handle method.

Task 2: MODIFY internal/signal/signal_test.go — add 6 cross-platform Trigger tests
  - All tests reuse the existing installTestHandler(t, opts) helper (@13) and call the PACKAGE-LEVEL Trigger
    (not h.handle) — that is what proves the active→handle delegation path. Use syscall.SIGTERM / os.Interrupt only.
  - TEST A — TestTrigger_RoutesThroughHandle (table-driven, SIGTERM→143 + SIGINT→130):
        cases := []struct{name string; sig os.Signal; want int}{
            {"SIGTERM_143", syscall.SIGTERM, 143},
            {"SIGINT_130",  os.Interrupt,   130},
        }
        for each: installTestHandler(Exit recorder, Out bytes.Buffer); Trigger(tc.sig); assert exitCode==tc.want.
  - TEST B — TestTrigger_RescueOnSnapshot:
        h := installTestHandler(RescueFormat + Exit recorder + OnRescueExit recorder + Out buf)
        SetSnapshot("abc","def","cand"); Trigger(syscall.SIGTERM)
        assert exitCode==3 AND buf contains the rescue AND OnRescueExit called exactly once (lock-release seam fires).
  - TEST C — TestTrigger_ForwardsToChild:
        h := installTestHandler(Kill recorder + Exit recorder)
        RegisterChild(1234); Trigger(syscall.SIGTERM)
        assert Kill called with (1234, syscall.SIGTERM) — forwarding works through Trigger.
  - TEST D — TestTrigger_NoOpAfterRestoreDefault (THE stopped-guard guarantee):
        h := installTestHandler(Kill recorder (killCalled bool) + Exit recorder (exitCalled bool))
        RestoreDefault(); Trigger(syscall.SIGTERM)
        assert !killCalled AND !exitCalled (handle() returns at its first line; the update-ref window is safe).
  - TEST E — TestTrigger_NilSafeNoHandler (library use):
        active.Store(nil); Trigger(syscall.SIGTERM)   // must NOT panic
        (no assertions beyond not-panicking; mirror TestHandler_NilWrappersNoOp's spirit.)
  - (Optional merge: fold A's two rows into one func; keep B/C/D/E discrete like the existing file style.)
  - COVERAGE: every success criterion has a test. No new helpers needed. DEPENDENCIES: Task 1.

Task 3: VERIFY — build (native + cross), vet, format, targeted tests, full suite, grep guards
  - go build ./...
  - GOOS=windows go build ./... ; GOOS=linux go build ./...   (Trigger is cross-platform; must stay clean)
  - go vet ./internal/signal/... ; GOOS=windows go vet ./internal/signal/...
  - gofmt -l internal/signal/signal.go internal/signal/signal_test.go   # must list nothing
  - go test ./internal/signal/... -run Trigger -v
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the nil-safe package wrapper (the deliverable — copy RegisterChild's shape, call handle)
func Trigger(sig os.Signal) {
    if h := active.Load(); h != nil { // nil ⇒ library use ⇒ no-op (mirrors every wrapper in this file)
        h.handle(sig)                 // delegates to the ONE rescue/exit routine (stopped-guarded at its first line)
    }
}

// PATTERN: the Trigger test (clone of TestHandler_Exit143SIGTERM @110, but calls the EXPORT, not the method)
func TestTrigger_RoutesThroughHandle(t *testing.T) {
    var exitCode int
    installTestHandler(t, Options{
        Exit: func(code int) { exitCode = code },
        Out:  new(bytes.Buffer),
    })
    Trigger(syscall.SIGTERM) // package-level — proves active.Load() → handle() delegation
    if exitCode != 143 {
        t.Errorf("exitCode = %d, want 143 (Trigger must route SIGTERM through handle)", exitCode)
    }
}

// PATTERN: the stopped-guard test (clone of TestHandler_RestoreDefaultStopsForward @126)
func TestTrigger_NoOpAfterRestoreDefault(t *testing.T) {
    var killCalled, exitCalled bool
    installTestHandler(t, Options{
        Kill: func(int, os.Signal) error { killCalled = true; return nil },
        Exit: func(int) { exitCalled = true },
        Out:  new(bytes.Buffer),
    })
    RestoreDefault()               // handler stopped (the update-ref window)
    Trigger(syscall.SIGTERM)       // must be a no-op — handle() returns at `if h.stopped.Load()`
    if killCalled { t.Error("Kill called after RestoreDefault, want no-op") }
    if exitCalled { t.Error("Exit called after RestoreDefault, want no-op") }
}
```

### Integration Points

```yaml
NO database / config / routes / new types / import changes. One additive exported function + tests.

SIGNAL PACKAGE (internal/signal/signal.go):
  - +1 exported func Trigger(sig os.Signal) after Active(); delegates to active.handle(sig).

DOWNSTREAM (this subtask ENABLES but does NOT build):
  - P1.M2.T2.S1 (parent-death watchdog) — will `import ".../internal/signal"` and call
    `signal.Trigger(syscall.SIGTERM)` on detected parent death (getppid polling fallback). The Linux
    prctl(PR_SET_PDEATHSIG, SIGTERM) fast path needs NO Trigger call (real SIGTERM flows through run→handle).
  - NO production caller exists yet after this subtask — that is expected (grep guard confirms exactly 0
    callers outside the new tests; the watchdog lands in P1.M2).

LIFECYCLE SAFETY (unchanged model):
  - no handler installed (active==nil)      → no-op (library use, D4).
  - handler active, no snapshot             → exit exitCodeForSignal(sig) (130/143).
  - handler active, snapshot armed          → rescue print + exit 3 + OnRescueExit (lock release).
  - handler stopped (post-RestoreDefault)   → no-op (update-ref window) — handle()'s first line.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native build
go build ./...
# Cross-compile (Trigger must be cross-platform — the core guard):
GOOS=windows go build ./...
GOOS=linux   go build ./...
GOOS=darwin  go build ./...
# Expected: all clean. If GOOS=windows fails you referenced syscall.SIGHUP in an un-tagged file or added a
# syscall import to signal.go. If native fails, you likely typed something other than the 3-line function.

# Vet (native + Windows)
go vet ./internal/signal/...
GOOS=windows go vet ./internal/signal/...

# Format
gofmt -l internal/signal/signal.go internal/signal/signal_test.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new Trigger tests (targeted)
go test ./internal/signal/... -run Trigger -v
# Expected: all pass — SIGTERM→143, SIGINT→130, rescue-on-snapshot (exit 3 + print + OnRescueExit),
#           forward-to-child, no-op-after-RestoreDefault, nil-safe-no-handler.

# Full signal package (regression — existing handle/RestoreDefault/wrapper tests must stay green)
go test ./internal/signal/... -v

# Cross-platform test COMPILE check (the Trigger tests are in the un-tagged file; must compile on Windows):
GOOS=windows go test -c -o /dev/null ./internal/signal/
# Expected: compiles (no syscall.SIGHUP in the un-tagged file).

# Whole suite (race detector — project standard)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary (sanity — Trigger has no caller yet, but the package must still build into the binary)
make build

# Manual proof that Trigger reaches handle end-to-end is not possible without the watchdog caller (P1.M2.T2.S1).
# The unit tests (Level 2) are the within-scope proof: they call the EXPORTED signal.Trigger and observe the
# same exit codes / rescue / lock-release-seam the OS-signal path produces — which is the contract P1.M2.T2.S1
# will rely on. A full e2e (launcher dies → watchdog → Trigger → rescue + lock removed) is the deliverable of
# P1.M4.T1.S1 (e2e scenarios), NOT this subtask.

# Re-run the stopped-guard and nil-safe tests explicitly (the two lifecycle guarantees):
go test ./internal/signal/... -run 'Trigger_NoOpAfterRestoreDefault|Trigger_NilSafeNoHandler' -v
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: exactly one Trigger definition, in signal.go
grep -rn 'func Trigger' internal/signal/
# Expected: one hit — internal/signal/signal.go.

# Grep guard: Trigger is syscall-free in signal.go (no syscall import re-added; survives parallel T1.S1)
grep -n 'syscall' internal/signal/signal.go
# Expected: ZERO hits in signal.go after T1.S1 lands (T1.S1 removes the import). Trigger itself must not
# reference syscall. (Pre-T1.S1, signal.go has the syscall import at line ~24 used only at line 103 — that's
# T1.S1's to remove, NOT T2.S1's concern. Just ensure Trigger adds no syscall reference.)

# Grep guard: NO SIGHUP in the un-tagged test file (Windows-safety)
grep -n 'SIGHUP' internal/signal/signal_test.go
# Expected: empty.

# Grep guard: Trigger has exactly 0 production callers yet (the watchdog is P1.M2) — only the new tests
grep -rn 'Trigger(' --include='*.go' internal/ cmd/ pkg/
# Expected: hits only in internal/signal/signal.go (the def + h... no) and internal/signal/signal_test.go (the tests).
#           ZERO hits in internal/lock, internal/cmd, pkg/, or a watchdog package — those land in P1.M2+.

# Scope-boundary guard: handle()/run()/RestoreDefault()/wrappers UNCHANGED
git diff --stat -- internal/signal/signal.go   # should be a tiny additive diff (new func + comment + tests)
git diff internal/signal/signal.go | grep -E '^[+-]' | grep -vE 'Trigger|^\+\+\+|^---|func \(h \*Handler\) handle'
# Expected: only Trigger-related additions; NO edits inside handle()/run()/RestoreDefault()/the setters.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `GOOS=windows/linux/darwin go build ./...` clean
- [ ] `go vet ./internal/signal/...` + `GOOS=windows go vet ./internal/signal/...` clean
- [ ] `gofmt -l internal/signal/signal.go internal/signal/signal_test.go` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass, incl. the new Trigger tests

### Feature Validation
- [ ] `Trigger(syscall.SIGTERM)` w/ handler, no snapshot → `Exit(143)`
- [ ] `Trigger(os.Interrupt)` w/ handler, no snapshot → `Exit(130)` (signal-agnostic)
- [ ] `Trigger(syscall.SIGTERM)` w/ snapshot → `Exit(3)` + rescue printed + `OnRescueExit` once
- [ ] `Trigger(syscall.SIGTERM)` w/ registered child → `Kill(pid, SIGTERM)`
- [ ] `Trigger(syscall.SIGTERM)` after `RestoreDefault()` → no-op (stopped guard — THE key guarantee)
- [ ] `Trigger(syscall.SIGTERM)` when `active==nil` → no panic (library use)
- [ ] Existing handle/RestoreDefault/wrapper tests still pass unchanged

### Scope-Boundary Validation
- [ ] NO change to `handle()`, `run()`, `RestoreDefault()`, or the setter wrappers (Trigger delegates, not duplicates)
- [ ] NO change to the caught-signal set (parallel P1.M1.T1.S1 owns that)
- [ ] NO watchdog / no_parent_watchdog config / lock-status changes (P1.M2/P1.M3)
- [ ] NO production caller of Trigger added (the watchdog is P1.M2.T2.S1) — only tests call it
- [ ] NO README.md / docs/ change (external docs are P1.M4.T2) — only the godoc on Trigger

### Code Quality & Docs
- [ ] Trigger follows the `if h := active.Load(); h != nil { ... }` idiom of every other wrapper
- [ ] Trigger takes `os.Signal` and references no `syscall` (survives T1.S1's import removal; cross-platform)
- [ ] Godoc explains purpose (parent-death watchdog rescue trigger) + the nil-safe + stopped-guard guarantees
- [ ] Tests call the package-level `Trigger` (not `h.handle`) to prove the active→handle delegation
- [ ] Tests use only cross-platform signals (`syscall.SIGTERM`, `os.Interrupt`) — Windows-safe

---

## Anti-Patterns to Avoid

- ❌ Don't duplicate the rescue logic in Trigger — delegate to `h.handle(sig)`. A second teardown path would diverge (exit codes, lock release, child forwarding) and break the single-rescue-path invariant. handle() is signal-agnostic; reuse it.
- ❌ Don't add a second stopped check in Trigger. handle()'s FIRST line is `if h.stopped.Load(){return}` — the no-op-after-RestoreDefault guarantee is already there. Test it; don't re-code it (a redundant check would mislead readers into thinking the guard lives in Trigger).
- ❌ Don't reference `syscall` in signal.go for Trigger. Trigger takes `os.Signal` and needs no syscall. Parallel T1.S1 REMOVES signal.go's syscall import; a syscall reference in Trigger would cause merge churn or a break. Use `os.Signal`.
- ❌ Don't use `syscall.SIGHUP` in the Trigger tests. signal_test.go is UN-TAGGED (compiles on Windows); SIGHUP doesn't exist in Go's Windows syscall pkg. Use `syscall.SIGTERM` / `os.Interrupt` only. (SIGHUP is T1.S1's signal, tested in signal_unix_test.go.)
- ❌ Don't call `h.handle(sig)` in the tests and call it a Trigger test — call the package-level `Trigger(sig)`. The whole point is to prove the `active.Load()` → `handle()` delegation the watchdog will rely on; calling the method directly tests nothing new.
- ❌ Don't add a production caller of Trigger (a watchdog, a hook, anything) — that's P1.M2.T2.S1. This subtask adds the export + tests only; grep must show zero non-test callers.
- ❌ Don't touch `handle()`/`run()`/`RestoreDefault()`/the setters/`Install()`/`Options` — Trigger is purely additive. If you find yourself editing those, you've drifted out of scope.
- ❌ Don't panic or `os.Exit` when `active==nil` — a no-op is correct (library use never installs the handler; D4). Mirror the nil-safe wrapper idiom exactly.
- ❌ Don't anchor placement to a line number — parallel T1.S1 edits signal.go and may shift lines. Place Trigger "immediately after the `Active()` function" by NAME.

---

## Confidence Score: 10/10

One-pass success is essentially guaranteed: the deliverable is a 3-line additive function that copies an
idiom used by SIX existing wrappers in the same file, delegating to an already-signal-agnostic `handle()`.
Every test to clone is enumerated with line numbers, the helper to reuse (`installTestHandler`) is named,
the two lifecycle guarantees (nil-safe, stopped-guard) are proven by existing analog tests, and the only
non-obvious risk (Windows-safety of test signals / coordination with parallel T1.S1's syscall-import removal)
is explicitly fenced in the gotchas. No downstream caller is built here, so there is no integration risk —
the unit tests ARE the contract for P1.M2.T2.S1. The single judgment call (placement after Active() vs. in
the wrappers section) is cosmetic and explicitly noted as flexible.
