---
name: "P1.M2.T3.S2 — Exit-path release signal tests (signal_test.go): OnRescueExit fires before Exit on both signal branches, skipped after RestoreDefault"
description: |
  TEST-ONLY subtask. Verifies the `OnRescueExit` exit-path-release seam landed by **P1.M2.T2.S1** (and wired by
  P1.M2.T2.S2) on top of P1.M2.T1's lock package. The production code is ALREADY IN THE TREE (`internal/signal/
  signal.go`: `Options.OnRescueExit func()` field + defaulted to `func(){}` in Install + `h.opts.OnRescueExit()`
  called immediately before BOTH `h.opts.Exit(3)` (post-snapshot rescue) and `h.opts.Exit(exitCodeForSignal(sig))`
  (pre-snapshot 130/143); `RestoreDefault` sets `h.stopped` so `handle()` early-returns and never reaches the seam).
  This task adds ONLY tests — no production change, no docs, no go.mod, no new files.

  ⚠️ **THE central design call — drive `h.handle(os.Interrupt)` DIRECTLY (no goroutine / no real signal).**
  `handle()` was extracted from `run()` precisely so unit tests can call it synchronously. The contract is
  explicit: "call h.handle(os.Interrupt) directly (no goroutine timing needed — handle() is extracted for direct
  testing)." Reuse the existing `installTestHandler(t, opts)` helper (it Stores the handler in the `active`
  singleton + resets it on Cleanup). Do NOT send a real signal to the process; do NOT wait on the `run()` goroutine.
  This is how EVERY existing unit test in signal_test.go works. See research §4.

  ⚠️ **THE ordering check — "OnRescueExit sets a flag that Exit checks" (verbatim from the contract).** Because
  `handle()` is synchronous within one goroutine, a flag set by `OnRescueExit` and READ by `Exit` reliably proves
  ordering with NO goroutine timing: `var rescueFired bool`; `OnRescueExit` sets `rescueFired = true`; `Exit`
  captures `exitSawRescueFired = rescueFired`. If ordering were reversed, `rescueFired` would still be false when
  Exit runs → assertion fails with a clear message. Plus a `rescueCalls int` counter for the "called exactly once"
  assertion (contract a/b). See research §3.

  ⚠️ **THE 3 contract scenarios → 3 tests (additive — NO existing test covers OnRescueExit).** (a)
  `TestHandler_OnRescueExit_PostSnapshot`: `SetSnapshot("tree","parent","cand")` → recording recorders →
  `h.handle(os.Interrupt)` → assert `rescueCalls==1` + `exitCode==3` + `exitSawRescueFired==true`. (b)
  `TestHandler_OnRescueExit_PreSnapshot`: NO SetSnapshot (tree=="") → same recorders → `h.handle(os.Interrupt)` →
  assert `rescueCalls==1` + `exitCode==130` + `exitSawRescueFired==true`. (c)
  `TestHandler_OnRescueExit_SkippedAfterRestoreDefault`: `RestoreDefault()` → `h.handle(os.Interrupt)` → assert
  `rescueCalls==0` (handler stopped; default disposition applies) AND `exitCalled==false`. "No real lock needed
  — the seam is a recorder." See research §2.

  ⚠️ **NO new imports, NO build tag, NO helper changes.** signal_test.go already imports `bytes`, `context`, `os`,
  `syscall`, `testing` — my tests use only `os` (os.Interrupt), `bytes` (new(bytes.Buffer) for Out), `context`
  (installTestHandler → Install(context.Background(), …)), `testing`. Zero import growth. Zero new helpers
  (existing `installTestHandler` suffices; I assert on recorders, NOT rescue output text, so `contains` is unused).
  signal_test.go has NO build tag and is cross-platform — the pre-snapshot `Exit(130)` assertion already works on
  Windows via signal_windows.go's `exitCodeForSignal` (the existing `TestHandler_Exit130PreSnapshot` proves it).
  See research §6.

  Deliverable: append 3 tests to ONE existing file — `internal/signal/signal_test.go`. NO production change, NO
  new files, NO docs, NO go.mod. INPUT = `OnRescueExit` seam + `handle()` + `SetSnapshot`/`RestoreDefault` +
  `installTestHandler` (all LANDED). OUTPUT = test coverage proving OnRescueExit fires before Exit on both signal
  branches, exactly once, and is skipped after RestoreDefault. DOCS = none (test-only).

---

## Goal

**Feature Goal**: Prove, via committed unit tests, that the `OnRescueExit` exit-path-release seam (P1.M2.T2.S1)
behaves correctly on all three `handle()` branches: it fires EXACTLY ONCE and BEFORE `Exit` on both the
post-snapshot (exit 3) and pre-snapshot (exit 130) signal paths, and it is SKIPPED entirely after
`RestoreDefault` (the handler is stopped; default disposition applies — no exit-path release needed).

**Deliverable** (edit to 1 existing file — `internal/signal/signal_test.go`): three new tests appended to the
existing file:
1. `TestHandler_OnRescueExit_PostSnapshot` (contract a) — snapshot armed → `handle(os.Interrupt)` → OnRescueExit
   called once + Exit(3) + OnRescueExit-before-Exit.
2. `TestHandler_OnRescueExit_PreSnapshot` (contract b) — no snapshot → `handle(os.Interrupt)` → OnRescueExit
   called once + Exit(130) + OnRescueExit-before-Exit.
3. `TestHandler_OnRescueExit_SkippedAfterRestoreDefault` (contract c) — `RestoreDefault` → `handle(os.Interrupt)`
   → OnRescueExit NOT called + Exit NOT called.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/signal/` clean; `go test -race
./internal/signal/` green — the 3 new tests pass AND all existing signal tests stay green; `go test -race ./...`
green (no regression); `GOOS=windows go test ./internal/signal/` green (signal_test.go is cross-platform); go.mod
/go.sum unchanged; only `internal/signal/signal_test.go` touched; zero new imports / zero new helpers.

## User Persona

**Target User**: The maintainer who needs confidence that the FR52 §18.5 "Exit-path release (prevention)" seam
actually fires at the right time — before `os.Exit` orphans the lock file on BOTH signal branches, and never
firing spuriously after the handler is stopped (RestoreDefault). Transitively every user whose Ctrl-C would
otherwise leave a dangling lock file (before this seam, `os.Exit` skipped `defer locker.Release()`).

**Use Case**: A user Ctrl-C's a run AFTER the snapshot (post-snapshot → rescue → `Exit(3)`) OR BEFORE it
(pre-snapshot → `Exit(130)`) — in both cases the lock is held (acquired at default_action.go:59, pre-snapshot),
so `OnRescueExit` (= `lock.ReleaseCurrent`, wired by P1.M2.T2.S2) must remove the file before `os.Exit` skips the
defer. These tests prove the seam fires on both branches, in the right order, exactly once — without requiring a
real lock or a real signal.

**User Journey**: (test-only; no user surface) `installTestHandler` → inject recording `OnRescueExit`/`Exit` →
arm/disarm snapshot or stop the handler → `h.handle(os.Interrupt)` → assert call counts, exit codes, and ordering.

**Pain Points Addressed**: removes the "does OnRescueExit actually fire before Exit, on BOTH branches, and get
skipped when the handler is stopped" uncertainty by pinning each branch with a dedicated test.

## Why

- **Closes the test-coverage gap for P1.M2.T2.S1.** S1 shipped the `OnRescueExit` seam + the `handle()` calls
  with NO committed tests (P1.M2.T3.S2 owns them — this task). The seam is unverified until these land.
- **Pins the ordering invariant.** "OnRescueExit fires BEFORE Exit" is the whole point: `os.Exit` skips defers,
  so the lock release MUST precede it. A regression that reordered (or removed) the `OnRescueExit()` call would
  re-orphan the lock file. The flag-check technique pins this per-branch.
- **Pins the both-branches rule.** The lock is acquired at `default_action.go:59` BEFORE the snapshot is armed,
  so a PRE-snapshot Ctrl-C (Exit 130/143) finds the lock HELD → must also release. Both branches get a test; a
  regression dropping the call from either branch fails a specific test.
- **Pins the RestoreDefault skip.** After RestoreDefault the handler is stopped (success path; `defer Release`
  runs normally) — OnRescueExit must NOT fire there (it would be a redundant double-release, harmless but a sign
  of a logic error). Contract (c) pins this.
- **No production/doc/dep change.** Test-only; DOCS: none. The production code (S1 seam + S2 wiring) is frozen.

## What

Three committed tests appended to `internal/signal/signal_test.go`. No production change, no new files, no docs,
no go.mod, no new imports, no new helpers.

### Success Criteria

- [ ] `internal/signal/signal_test.go` gains `TestHandler_OnRescueExit_PostSnapshot`, `TestHandler_OnRescueExit_PreSnapshot`,
      and `TestHandler_OnRescueExit_SkippedAfterRestoreDefault`.
- [ ] Each test uses `installTestHandler(t, Options{OnRescueExit: …, Exit: …, Out: new(bytes.Buffer)})` and calls
      `h.handle(os.Interrupt)` DIRECTLY (no goroutine, no real signal).
- [ ] Post-snapshot test: `SetSnapshot("tree","parent","cand")` first; asserts `rescueCalls==1`, `exitCode==3`,
      `exitSawRescueFired==true`.
- [ ] Pre-snapshot test: NO `SetSnapshot`; asserts `rescueCalls==1`, `exitCode==130`, `exitSawRescueFired==true`.
- [ ] RestoreDefault test: calls `RestoreDefault()` first; asserts `rescueCalls==0` AND `exitCalled==false`.
- [ ] Ordering is verified by OnRescueExit setting a `rescueFired` flag that `Exit` reads into `exitSawRescueFired`.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/signal/`, `go test -race ./internal/signal/`,
      `go test -race ./...`, `GOOS=windows go test ./internal/signal/` clean/green; go.mod/go.sum unchanged;
      imports byte-unchanged; only `signal_test.go` touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact `handle()` behavior under
test (quoted), the ordering-verification technique (verbatim code), the three test sketches (with verbatim code),
the direct-`handle()` invocation rationale, and the zero-import/zero-build-tag finding. No PRD/git-internals
knowledge beyond "os.Exit skips defers; the seam releases the lock before Exit."

### Documentation & References

```yaml
# MUST READ — the authoritative research (the ordering technique + the 3-test mapping + the no-import finding)
- docfile: plan/011_98cef660a41d/P1M2T3S2/research/onrescueexit-seam-tests.md
  why: the landed production code under test (§1 — handle()'s two OnRescueExit-before-Exit branches + the
       RestoreDefault early-return), the 3 contract scenarios → 3 tests (§2), the ordering flag-check technique
       (§3 — "OnRescueExit sets a flag that Exit checks"), direct handle() invocation (§4), os.Interrupt choice
       (§5), the ZERO new imports / NO build tag finding (§6), the singleton/no-Parallel note (§7), the
       RestoreDefault-then-handle detail (§8), scope fences (§9), verified validation commands (§10).
  critical: §3 (the ordering technique — a flag set by OnRescueExit, read by Exit) and §6 (zero imports — do NOT
       add io/syscall; reuse os+bytes+context already imported) are the things most likely to be done wrong.

# THE production code under test (LANDED by P1.M2.T2.S1 — read, do NOT edit)
- file: internal/signal/signal.go
  section: Options.OnRescueExit field (~L73) + Install default `func(){}` (~L96-97) + handle()'s two call sites
           (L149 OnRescueExit→L150 Exit(3) post-snapshot; L153 OnRescueExit→L154 Exit(exitCodeForSignal) pre-snapshot)
           + the RestoreDefault early-return (`if h.stopped.Load() { return }` at the top of handle()).
  why: the code these tests exercise. handle() is extracted from run() for direct testing — call h.handle(sig)
       directly. OnRescueExit fires before Exit on BOTH branches; RestoreDefault sets h.stopped → handle() returns
       before reaching the seam.
  pattern: the tests drive handle() (the extracted method) and assert on the recording closures (call count +
           exit code + ordering flag). They do NOT send a real signal or touch the lock.
  gotcha: OnRescueExit is defaulted to func(){} — so a test that OMITS it from Options still compiles + runs
           (the no-op fires). The 3 new tests MUST inject their own recording OnRescueExit to assert on it.

# THE exit-code mapping (proves pre-snapshot 130; cross-platform)
- file: internal/signal/signal_unix.go   (//go:build !windows)
  section: exitCodeForSignal — os.Interrupt/SIGINT→130, SIGTERM→143, else 1.
  why: PROVES the pre-snapshot branch calls Exit(130) for os.Interrupt (contract b's assertion).
- file: internal/signal/signal_windows.go   (//go:build windows)
  section: the Windows exitCodeForSignal equivalent (must exist — the existing TestHandler_Exit130PreSnapshot
           asserts 130 with no build tag, so the Windows build of the package produces 130 too).
  why: PROVES signal_test.go (no build tag) compiles + passes on Windows — the pre-snapshot 130 assertion is
       cross-platform. Do NOT add a build tag to signal_test.go.

# THE test file being edited + the helper/pattern to reuse
- file: internal/signal/signal_test.go   (NO build tag — cross-platform)
  why: the file you APPEND the 3 tests to. Has `installTestHandler(t, opts)` (Stores handler in `active` +
       resets on Cleanup), `contains` helper (unused by these tests), and 10 existing tests. The new tests
       follow the existing style: `installTestHandler(t, Options{...})` → `h.handle(os.Interrupt)` → assertions.
  pattern: mirror TestHandler_Exit130PreSnapshot (opts with Exit recorder + Out buffer; direct handle() call;
       assert exitCode) and TestHandler_RestoreDefaultStopsForward (RestoreDefault → handle() → assert no-op).
       The NEW element is the OnRescueExit recorder + the ordering flag (none of the existing tests assert on it).
  gotcha: ZERO import changes. signal_test.go imports bytes/context/os/syscall/testing — the new tests use only
           os (os.Interrupt)/bytes (Out buffer)/context (installTestHandler)/testing. Do NOT add io, sync, etc.

# THE seam's upstream spec (read-only — confirms the both-branches + RestoreDefault-exclusion design)
- docfile: plan/011_98cef660a41d/architecture/lock_reaping.md
  section: "Fix 2: Exit-Path Lock Release via Signal Seam" → OnRescueExit before BOTH exit branches; NOT in
           RestoreDefault (success path uses defer Release).
  why: the authoritative design rationale for what these tests pin: "Both branches need coverage: the lock is
       acquired at default_action.go:59 BEFORE the snapshot is armed." And RestoreDefault is success-path (no
       os.Exit → defer Release runs) → OnRescueExit is exit-path ONLY (test c pins the skip).

# THE requirement + the seam's purpose
- file: PRD.md §18.5 (h3.91) "Concurrency: the per-repo run lock (FR52)" — "Exit-path release (prevention)."
  why: the PRD source. "The signal handler therefore releases the lock file immediately before exiting, via the
       same injected-seam used for the rescue formatter (the signal package stays stdlib-only and cannot import
       the lock package)." These tests verify that injected seam's firing contract.

# THE previous (parallel) task's PRP — the contract for what exists when implementation begins
- docfile: plan/011_98cef660a41d/P1M2T3S1/PRP.md
  why: P1.M2.T3.S1 adds reaping tests to internal/lock/lock_unix_test.go (a DIFFERENT file, DIFFERENT package,
       DIFFERENT concern). NON-OVERLAPPING with this task (signal_test.go). Read it to confirm no collision +
       to mirror the test-only PRP structure. Do NOT duplicate its work.
```

### Current Codebase tree (relevant slice)

```bash
internal/signal/
  signal.go                  # OnRescueExit field + Install default + handle() 2 call sites (LANDED, P1.M2.T2.S1) — NO edit
  signal_unix.go             # exitCodeForSignal (130/143) (LANDED) — NO edit
  signal_windows.go          # exitCodeForSignal equivalent (LANDED) — NO edit
  signal_test.go             # installTestHandler + 10 existing tests + contains helper — EDIT (APPEND 3 OnRescueExit tests)
  signal_integration_test.go # //go:build !windows — real-binary SIGINT tests (NOT this task) — NO edit
cmd/stagecoach/main.go        # OnRescueExit: lock.ReleaseCurrent wiring (LANDED, P1.M2.T2.S2) — NO edit
internal/lock/*              # ReleaseCurrent + lock package (LANDED, P1.M2.T1/T2) — NO edit
go.mod / go.sum              # unchanged (stdlib only: bytes/context/os/syscall/testing — all already imported)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. ONE edit: internal/signal/signal_test.go (+3 tests appended; ZERO import changes).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (drive handle() DIRECTLY — no goroutine, no real signal): handle() was extracted from run() so unit
// tests call it synchronously. Use installTestHandler(t, opts) → h.handle(os.Interrupt). Do NOT send a real
// signal to the process and do NOT wait on the run() goroutine. This is how every existing signal unit test works.

// CRITICAL (the ordering check = a flag OnRescueExit sets that Exit reads): handle() is synchronous in one
// goroutine, so a flag is a reliable ordering probe. var rescueFired bool; OnRescueExit: rescueCalls++; rescueFired=true;
// Exit: exitCode=code; exitSawRescueFired=rescueFired. If Exit ran first, rescueFired would still be false →
// exitSawRescueFired==false → fail with "OnRescueExit must fire BEFORE Exit". (rescueCalls gives the "exactly once".)

// CRITICAL (BOTH branches need OnRescueExit): the lock is acquired at default_action.go:59 BEFORE the snapshot
// is armed, so a PRE-snapshot Ctrl-C (Exit 130) finds the lock HELD. Both the post-snapshot (Exit 3) and the
// pre-snapshot (Exit 130) branches must fire OnRescueExit before Exit. Test a AND test b each pin one branch.

// CRITICAL (OnRescueExit is SKIPPED after RestoreDefault, NOT removed): RestoreDefault sets h.stopped=true;
// handle()'s FIRST line is `if h.stopped.Load() { return }` — so after RestoreDefault, handle() returns before
// reaching Kill/cancel/OnRescueExit/Exit. Test c asserts OnRescueExit NOT called (rescueCalls==0) AND Exit NOT
// called. OnRescueExit is deliberately NOT in RestoreDefault itself (success path uses defer Release).

// GOTCHA (os.Interrupt, not syscall.SIGINT — match the contract): exitCodeForSignal handles both identically
// (case os.Interrupt, syscall.SIGINT). os.Interrupt==syscall.SIGINT on Unix. Use os.Interrupt to match the
// contract verbatim. os is already imported.

// GOTCHA (OnRescueExit MUST be injected — it defaults to a no-op): Install defaults OnRescueExit to func(){} so
// omitting it still runs (the no-op fires). The 3 new tests MUST inject their own recording OnRescueExit to
// assert call count / ordering. A test that forgets to inject it will see rescueCalls==0 even on the exit paths
// (the no-op fires but isn't recorded) — that's a test bug, not a production bug.

// GOTCHA (Out is required for the post-snapshot branch): the post-snapshot branch calls fmt.Fprintln(h.opts.Out,
// RescueFormat(...)). Install defaults Out to os.Stderr, so omitting it WRITES TO THE REAL STDERR during the test
// (noisy but harmless). Inject Out: new(bytes.Buffer) to keep test output clean (mirror the existing tests).

// GOTCHA (RescueFormat defaults are fine for these tests): the post-snapshot branch uses RescueFormat only to
// produce the rescue text (written to Out). These tests do NOT assert on rescue text — they assert on the
// OnRescueExit/Exit recorders. So OMIT RescueFormat (Install defaults it); no need to inject a stub.

// GOTCHA (no t.Parallel): the `active` singleton is process-global; installTestHandler Stores it + resets on
// Cleanup. Existing tests are sequential (none call t.Parallel). Follow suit.

// GOTCHA (signal_test.go has NO build tag and is cross-platform): do NOT add //go:build !windows. The existing
// TestHandler_Exit130PreSnapshot asserts Exit(130) with no build tag and passes on Windows (signal_windows.go
// has the exitCodeForSignal equivalent). Adding a build tag would needlessly exclude these tests from Windows CI.

// GOTCHA (do NOT send a real SIGINT or build a real binary): that's signal_integration_test.go's job (it's
// //go:build !windows + builds cmd/stagecoach). These are pure in-process unit tests via the recording seam.
```

## Implementation Blueprint

### Data models and structure

No new types, no new helpers. Three test functions appended to the existing file. The shared recorder pattern
(declared inside each test — they are independent):

```go
// internal/signal/signal_test.go — APPEND to the existing file. ZERO import changes (bytes/context/os/testing
// already imported; the new tests use only those). The recorders below are declared per-test (independent state).

// Shared shape used by tests (a) and (b):
//   var rescueCalls int          // "called int" counter — contract a/b ("exactly once")
//   var rescueFired bool         // flag OnRescueExit sets
//   var exitCode int             // code recorder (Exit)
//   var exitSawRescueFired bool  // what Exit observed — false ⇒ OnRescueExit did NOT fire first (ordering)
//   opts := Options{
//       OnRescueExit: func() { rescueCalls++; rescueFired = true },
//       Exit:         func(code int) { exitCode = code; exitSawRescueFired = rescueFired },
//       Out:          new(bytes.Buffer),
//   }
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: signal_test.go — APPEND TestHandler_OnRescueExit_PostSnapshot (contract a)
  - APPEND the test per the sketch below: installTestHandler with the shared recorder Options; SetSnapshot
    ("tree","parent","cand") to arm the rescue path; h.handle(os.Interrupt).
  - ASSERTIONS: rescueCalls == 1; exitCode == 3; exitSawRescueFired == true.
  - GOTCHA: SetSnapshot arms snapTree (tree != "" → post-snapshot branch). Out: new(bytes.Buffer) keeps the
    rescue print off real stderr. RescueFormat omitted (Install defaults it — fine, we don't assert on text).

Task 2: signal_test.go — APPEND TestHandler_OnRescueExit_PreSnapshot (contract b)
  - APPEND the test per the sketch below: installTestHandler with the shared recorder Options; NO SetSnapshot
    (snapTree == "" → pre-snapshot branch); h.handle(os.Interrupt).
  - ASSERTIONS: rescueCalls == 1; exitCode == 130; exitSawRescueFired == true.
  - GOTCHA: NO SetSnapshot → tree=="" → pre-snapshot branch → Exit(exitCodeForSignal(os.Interrupt))==130.
    os.Interrupt (not syscall.SIGINT) per the contract.

Task 3: signal_test.go — APPEND TestHandler_OnRescueExit_SkippedAfterRestoreDefault (contract c)
  - APPEND the test per the sketch below: installTestHandler with a recorder Options (rescueCalls counter +
    exitCalled bool); RestoreDefault(); h.handle(os.Interrupt).
  - ASSERTIONS: rescueCalls == 0 (OnRescueExit NOT called — handler stopped); exitCalled == false (Exit NOT called).
  - GOTCHA: RestoreDefault sets h.stopped → handle() returns at its first line (if h.stopped.Load()). Neither
    Kill, cancel, OnRescueExit, nor Exit runs. Assert BOTH rescueCalls==0 AND exitCalled==false for completeness.

Task 4: VERIFY (no further edits)
  - RUN `gofmt -w internal/signal/signal_test.go`; `go vet ./internal/signal/`; `go build ./...`;
    `go test -race ./internal/signal/ -v -run 'TestHandler_OnRescueExit'`; `go test -race ./internal/signal/`;
    `go test -race ./...`; `GOOS=windows go test ./internal/signal/`.
  - go.mod/go.sum byte-unchanged. Imports byte-unchanged. Only signal_test.go touched. signal.go/signal_unix.go/
    signal_windows.go/signal_integration_test.go/main.go/lock/* /docs byte-unchanged.
```

### Test Specs (signal_test.go — 3 new tests appended; ZERO import changes)

```go
// (NO import changes — the new tests use only os (os.Interrupt), bytes (Out), context (installTestHandler),
//  testing — all already imported by signal_test.go. Append these three tests anywhere after installTestHandler;
//  conventionally near the related existing tests, e.g. after TestHandler_NoChildKill, before the `contains` helper.)

// TestHandler_OnRescueExit_PostSnapshot verifies the FR52 §18.5 exit-path-release seam (P1.M2.T2.S1): on the
// POST-SNAPSHOT signal branch (snapshot armed → Exit 3), OnRescueExit fires EXACTLY ONCE and BEFORE Exit. The
// ordering is proven by OnRescueExit setting a flag (rescueFired) that Exit reads — since handle() is synchronous
// in one goroutine, if Exit ran first the flag would still be false. (Contract a.)
func TestHandler_OnRescueExit_PostSnapshot(t *testing.T) {
	var rescueCalls int
	var rescueFired bool
	var exitCode int
	var exitSawRescueFired bool

	h := installTestHandler(t, Options{
		OnRescueExit: func() {
			rescueCalls++
			rescueFired = true
		},
		Exit: func(code int) {
			exitCode = code
			exitSawRescueFired = rescueFired // Exit "checks" the flag OnRescueExit set
		},
		Out: new(bytes.Buffer), // keep the rescue print off real stderr
	})

	SetSnapshot("tree", "parent", "cand") // arm the rescue path (snapTree != "" → post-snapshot branch)
	h.handle(os.Interrupt)                // direct call — no goroutine timing (handle() is extracted for testing)

	if rescueCalls != 1 {
		t.Errorf("OnRescueExit calls = %d, want 1 (post-snapshot exit path)", rescueCalls)
	}
	if exitCode != 3 {
		t.Errorf("Exit code = %d, want 3 (post-snapshot rescue)", exitCode)
	}
	if !exitSawRescueFired {
		t.Error("Exit observed rescueFired=false, want true — OnRescueExit must fire BEFORE Exit (FR52 exit-path release)")
	}
}

// TestHandler_OnRescueExit_PreSnapshot verifies the seam on the PRE-SNAPSHOT branch (no snapshot → Exit 130):
// OnRescueExit fires EXACTLY ONCE and BEFORE Exit. The lock is acquired at default_action.go:59 BEFORE the
// snapshot is armed, so a pre-snapshot Ctrl-C finds the lock HELD → the release must fire here too (both branches
// need it). Same ordering flag technique as the post-snapshot test. (Contract b.)
func TestHandler_OnRescueExit_PreSnapshot(t *testing.T) {
	var rescueCalls int
	var rescueFired bool
	var exitCode int
	var exitSawRescueFired bool

	h := installTestHandler(t, Options{
		OnRescueExit: func() {
			rescueCalls++
			rescueFired = true
		},
		Exit: func(code int) {
			exitCode = code
			exitSawRescueFired = rescueFired
		},
		Out: new(bytes.Buffer),
	})

	// NO SetSnapshot — snapTree == "" → pre-snapshot branch → Exit(exitCodeForSignal(os.Interrupt)) == 130.
	h.handle(os.Interrupt)

	if rescueCalls != 1 {
		t.Errorf("OnRescueExit calls = %d, want 1 (pre-snapshot exit path — lock held since default_action.go:59)", rescueCalls)
	}
	if exitCode != 130 {
		t.Errorf("Exit code = %d, want 130 (SIGINT, pre-snapshot)", exitCode)
	}
	if !exitSawRescueFired {
		t.Error("Exit observed rescueFired=false, want true — OnRescueExit must fire BEFORE Exit (FR52 exit-path release)")
	}
}

// TestHandler_OnRescueExit_SkippedAfterRestoreDefault verifies that after RestoreDefault the handler is STOPPED:
// handle() returns at its first line (if h.stopped.Load()) and never reaches OnRescueExit or Exit. This is correct
// — RestoreDefault is the SUCCESS path (before update-ref; no os.Exit) → defer locker.Release() runs normally, so
// the exit-path seam must NOT fire (it would be a redundant double-release). Asserts OnRescueExit NOT called AND
// Exit NOT called. (Contract c. "No real lock needed — the seam is a recorder.")
func TestHandler_OnRescueExit_SkippedAfterRestoreDefault(t *testing.T) {
	var rescueCalls int
	var exitCalled bool

	h := installTestHandler(t, Options{
		OnRescueExit: func() { rescueCalls++ },
		Exit:         func(int) { exitCalled = true },
		Out:          new(bytes.Buffer),
	})

	RestoreDefault() // stop signal delivery — handler is now inert (h.stopped == true)
	h.handle(os.Interrupt)

	if rescueCalls != 0 {
		t.Errorf("OnRescueExit calls = %d, want 0 (handler stopped after RestoreDefault; success path uses defer Release)", rescueCalls)
	}
	if exitCalled {
		t.Error("Exit was called after RestoreDefault, want no-op (handler stopped; default disposition applies)")
	}
}
```

### Implementation Patterns & Key Details

```go
// THE ordering probe — a flag OnRescueExit sets, Exit reads (handle() is synchronous in one goroutine):
//   var rescueFired bool
//   OnRescueExit: func() { rescueCalls++; rescueFired = true }
//   Exit:         func(code int) { exitCode = code; exitSawRescueFired = rescueFired }
//   ... h.handle(os.Interrupt) ...
//   if !exitSawRescueFired { t.Error("OnRescueExit must fire BEFORE Exit") }
// Reliability: handle() calls OnRescueExit then Exit sequentially in the SAME goroutine. If Exit ran first,
// rescueFired would be false when Exit reads it → exitSawRescueFired==false → fail. (No goroutine timing needed —
// that's WHY handle() was extracted from run().)

// THE 3 branches → which assertions hold:
//   POST-SNAPSHOT  (tree != "")   → OnRescueExit ONCE, Exit(3),       exitSawRescueFired TRUE.
//   PRE-SNAPSHOT   (tree == "")   → OnRescueExit ONCE, Exit(130),     exitSawRescueFired TRUE.
//   RESTOREDEFAULT (h.stopped)    → OnRescueExit ZERO, Exit NOT called (handle() early-returns).

// WHY direct handle() (not a real signal): handle() is extracted from run() for exactly this — synchronous,
// deterministic, in-process. installTestHandler(t, opts) returns *Handler; call h.handle(os.Interrupt). The
// run() goroutine is irrelevant (it just ranges the signal channel). Every existing signal unit test does this.

// WHY os.Interrupt: exitCodeForSignal maps os.Interrupt/SIGINT→130 (and SIGTERM→143). os.Interrupt==SIGINT on
// Unix. The contract says os.Interrupt; use it. (signal_windows.go's equivalent also yields 130 — cross-platform.)

// WHY no RescueFormat/Out complexity: the post-snapshot branch writes RescueFormat(tree,parent,cand) to Out.
// These tests assert on the OnRescueExit/Exit recorders, NOT the rescue text — so omit RescueFormat (Install
// defaults it) and pass Out: new(bytes.Buffer) to keep test output clean.
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — stdlib only (bytes/context/os/syscall/testing — all already imported).
      go mod tidy is a no-op.

PACKAGE EDGES: NONE. signal_test.go is `package signal` (white-box, NO build tag — cross-platform). It reuses
      installTestHandler (same file) + SetSnapshot/RestoreDefault/handle/Options (the package). It touches NO
      other package (no internal/lock import — the seam is a func() recorder, exactly as designed).

FROZEN / NOT-EDITED:
  - internal/signal/signal.go (OnRescueExit seam + handle() — P1.M2.T2.S1, LANDED).
  - internal/signal/signal_unix.go + signal_windows.go (exitCodeForSignal — LANDED).
  - internal/signal/signal_integration_test.go (real-binary SIGINT tests — //go:build !windows; NOT this task).
  - cmd/stagecoach/main.go (OnRescueExit: lock.ReleaseCurrent wiring — P1.M2.T2.S2, LANDED).
  - internal/lock/* (ReleaseCurrent + lock package + P1.M2.T3.S1's reaping tests — LANDED / parallel, different file).
  - docs/* (DOCS: none — test-only; P1.M3 owns the changeset doc sync).
  - go.mod / go.sum.

NO DATABASE / ROUTES / CONFIG / CLI / NEW FILES / DOCS / PRODUCTION CHANGE / NEW IMPORTS / NEW HELPERS.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/signal/signal_test.go
test -z "$(gofmt -l internal/signal/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/signal/   # catches a malformed test / unused var / wrong Options field.
go build ./...              # whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm imports are byte-unchanged (ZERO new imports — bytes/context/os/syscall/testing only):
git diff internal/signal/signal_test.go | grep -E '^\+\s*"bytes"|^\+\s*"context"|^\+\s*"os"|^\+\s*"syscall"|^\+\s*"testing"|^\+\s*"io"' \
  && echo "BAD: a NEW import was added" || echo "imports unchanged (good — no new imports)"
# Confirm the 3 new tests landed:
grep -n 'func TestHandler_OnRescueExit_PostSnapshot\|func TestHandler_OnRescueExit_PreSnapshot\|func TestHandler_OnRescueExit_SkippedAfterRestoreDefault' internal/signal/signal_test.go
# Confirm only signal_test.go changed:
git diff --name-only | grep -v '^internal/signal/signal_test\.go$' && echo "UNEXPECTED file changed" || echo "only signal_test.go changed (good)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 3 new seam tests:
go test -race ./internal/signal/ -v -run 'TestHandler_OnRescueExit'
# Expected PASS:
#   TestHandler_OnRescueExit_PostSnapshot ........... rescueCalls==1, exitCode==3, OnRescueExit-before-Exit.
#   TestHandler_OnRescueExit_PreSnapshot ............ rescueCalls==1, exitCode==130, OnRescueExit-before-Exit.
#   TestHandler_OnRescueExit_SkippedAfterRestoreDefault ... rescueCalls==0, exitCalled==false.
go test -race ./internal/signal/    # full signal suite (the 10 existing tests + the 3 new) — no regression.
go test -race ./...                 # full module — no regression.
# Cross-platform gate — signal_test.go has NO build tag, so the pre-snapshot 130 assertion must pass on Windows too
# (signal_windows.go's exitCodeForSignal yields 130; the existing TestHandler_Exit130PreSnapshot proves this):
GOOS=windows go vet ./internal/signal/ && echo "windows vet OK"
GOOS=windows go test ./internal/signal/ && echo "windows test OK (signal_test.go is cross-platform)"
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm the production code + the platform files + main.go + the lock package + integration test are byte-unchanged:
git diff --exit-code -- internal/signal/signal.go internal/signal/signal_unix.go internal/signal/signal_windows.go internal/signal/signal_integration_test.go cmd/stagecoach/main.go internal/lock docs && echo "production + platform + main + lock + integration-test + docs UNCHANGED (expected — test-only)"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# THE lint gate — the recorders (rescueCalls/rescueFired/exitCode/exitSawRescueFired/exitCalled) are all READ by
# the assertions → no unused-variable / U1000 errors:
make lint 2>&1 | grep -iE 'signal_test|unused|U1000' && echo "BAD: flagged" || echo "not flagged (good)"
# Seam-coverage audit — OnRescueExit is asserted in exactly the 3 contract branches:
grep -c 'func TestHandler_OnRescueExit' internal/signal/signal_test.go   # → 3
# Ordering-technique audit — the flag is set by OnRescueExit and read by Exit in tests a + b:
grep -c 'exitSawRescueFired = rescueFired' internal/signal/signal_test.go   # → 2 (post + pre snapshot)
# Direct-handle() audit — all 3 tests call h.handle directly (no goroutine, no real signal):
grep -c 'h.handle(os.Interrupt)' internal/signal/signal_test.go   # → 3 (one per new test)
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1 clean: `gofmt -l internal/signal/`, `go vet ./internal/signal/`, `go build ./...`, `go mod tidy`
      no-op; ZERO new imports; only `signal_test.go` changed.
- [ ] Level 2 green: the 3 new tests pass; `go test -race ./internal/signal/` + `./...` green;
      `GOOS=windows go test ./internal/signal/` green (signal_test.go is cross-platform).
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; production + platform + main + lock + integration-test +
      docs byte-unchanged.
- [ ] Level 4: `make lint` green (no unused); the 3 OnRescueExit branches each pinned by a test.

### Feature Validation
- [ ] `TestHandler_OnRescueExit_PostSnapshot`: SetSnapshot armed → OnRescueExit called ONCE, Exit(3), OnRescueExit-before-Exit.
- [ ] `TestHandler_OnRescueExit_PreSnapshot`: no snapshot → OnRescueExit called ONCE, Exit(130), OnRescueExit-before-Exit.
- [ ] `TestHandler_OnRescueExit_SkippedAfterRestoreDefault`: RestoreDefault → OnRescueExit NOT called, Exit NOT called.
- [ ] Each of the 3 `handle()` branches (post-snapshot / pre-snapshot / stopped) pinned by a dedicated test.

### Code Quality Validation
- [ ] Tests mirror the existing signal_test.go style (installTestHandler, direct h.handle(), t.Errorf assertions).
- [ ] Reuses `installTestHandler` (same file) + `SetSnapshot`/`RestoreDefault`/`handle`/`Options` (the package); no re-impl.
- [ ] Ordering verified by the contract's specified technique (flag OnRescueExit sets, Exit reads).
- [ ] No scope creep into signal.go/platform files/main.go/lock package/integration test/docs/production code.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment
- [ ] No docs (DOCS: none — test-only; P1.M3 owns the changeset doc sync).
- [ ] go.mod/go.sum byte-unchanged; no new files; no production change; no new imports; no new helpers.

---

## Anti-Patterns to Avoid

- ❌ **Don't send a REAL signal or build a real binary.** That's `signal_integration_test.go`'s job (`//go:build
  !windows`, builds cmd/stagecoach). These are pure in-process unit tests: `installTestHandler(t, opts)` →
  `h.handle(os.Interrupt)` directly. handle() was extracted from run() for exactly this (no goroutine timing).
- ❌ **Don't assert ordering with a timer / sleep / channel.** handle() is synchronous in one goroutine — use the
  contract's flag technique: OnRescueExit sets `rescueFired`, Exit reads it into `exitSawRescueFired`. A reversed
  order leaves the flag false → a clear failure message. No timing flakiness.
- ❌ **Don't test only ONE exit branch.** The lock is acquired at `default_action.go:59` BEFORE the snapshot is
  armed, so a pre-snapshot Ctrl-C (Exit 130) finds the lock HELD too. BOTH the post-snapshot (Exit 3) and
  pre-snapshot (Exit 130) branches need an OnRescueExit-before-Exit test. Missing either leaves a window uncovered.
- ❌ **Don't forget to inject `OnRescueExit`.** Install defaults it to `func(){}` — a test that omits it still
  runs (the no-op fires) but records nothing → `rescueCalls` stays 0 → a false failure you'd misread as a
  production bug. The 3 new tests MUST inject their own recording OnRescueExit.
- ❌ **Don't inject `OnRescueExit` only in tests (a)/(b) and forget (c) needs a counter too.** Contract (c)
  asserts OnRescueExit is NOT called after RestoreDefault — inject a counter and assert `rescueCalls == 0`.
- ❌ **Don't add a build tag to signal_test.go.** It is cross-platform today (the existing
  `TestHandler_Exit130PreSnapshot` asserts 130 with no build tag and passes on Windows via signal_windows.go's
  `exitCodeForSignal`). Adding `//go:build !windows` would needlessly exclude these tests from Windows CI.
- ❌ **Don't add new imports.** signal_test.go already imports bytes/context/os/syscall/testing. The new tests use
  only those. Adding `io` (for io.Discard) or `sync` is unnecessary — use `new(bytes.Buffer)` for `Out` (matches
  the existing tests) and plain vars for the recorders.
- ❌ **Don't omit `Out`.** The post-snapshot branch writes the rescue message to `Out`. Install defaults `Out` to
  `os.Stderr` — omitting it spams the real stderr during the test (noisy, harmless, but sloppy). Pass
  `Out: new(bytes.Buffer)`.
- ❌ **Don't inject a custom `RescueFormat`.** These tests assert on the OnRescueExit/Exit recorders, NOT the
  rescue text. Install's default RescueFormat is fine. (Injecting one isn't wrong, just unnecessary.)
- ❌ **Don't use `syscall.SIGINT` when the contract says `os.Interrupt`.** Both map to 130 in `exitCodeForSignal`
  (and are identical on Unix), but the contract literally says `os.Interrupt` — use it for fidelity. `os` is imported.
- ❌ **Don't call `t.Parallel()`.** The `active` singleton is process-global; `installTestHandler` Stores it +
  resets on Cleanup. Existing tests are sequential — follow suit (no Parallel).
- ❌ **Don't edit production code.** This is test-only. The OnRescueExit seam (signal.go) + its wiring (main.go) +
  the lock package are all LANDED (P1.M2.T1/T2). Editing them overlaps other work items. The SOLE edit is
  `internal/signal/signal_test.go`.
- ❌ **Don't touch signal_integration_test.go, signal.go, signal_unix.go, signal_windows.go, main.go, lock/*, or
  docs.** The SOLE edit is `internal/signal/signal_test.go`. `installTestHandler` (same file) is reused as-is.
- ❌ **Don't change go.mod/go.sum or add files.** Three tests appended to one existing file; zero imports; zero helpers.
- ❌ **Don't duplicate P1.M2.T3.S1's work.** S1 adds reaping tests to `internal/lock/lock_unix_test.go` (a
  different package, different file, different concern — the lock-FILE reaping on Acquire). This task is the
  signal SEAM tests (OnRescueExit firing contract). Non-overlapping. Don't touch lock_unix_test.go.

---

## Confidence Score

**9/10** — a test-only task against LANDED production code (the OnRescueExit seam is verified in-tree: field +
Install default + both handle() call sites + the RestoreDefault early-return), with a deterministic ordering
probe (the flag technique the contract specifies, reliable because handle() is synchronous), direct handle()
invocation (no goroutine timing — handle() was extracted for this), verbatim test sketches for all 3 contract
branches, and a confirmed ZERO-import / NO-build-tag finding (the new tests reuse only os/bytes/context/testing
already imported; signal_test.go is cross-platform today). The -1 reserves for the minor care items the call-outs
mitigate: (1) remembering to inject OnRescueExit in all 3 tests (it defaults to a no-op, so omitting it records
nothing), and (2) keeping Out as new(bytes.Buffer) so the rescue print stays off real stderr.
