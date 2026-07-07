---
name: "P1.M2.T2.S1 — ReleaseCurrent + OnRescueExit signal seam in handle() (FR52 exit-path release)"
description: |
  The PREVENTION half of §18.5's stale-lock-file story (FR52). `flock` auto-releases on process death so the
  *lock* is never stale, but `signal.handle()` calls `os.Exit` on BOTH signal paths (post-snapshot rescue
  `Exit(3)` + pre-snapshot `Exit(130/143)`), which **skips `defer locker.Release()`** at
  `default_action.go:67`, orphaning the lock *file*. The lock is acquired at `default_action.go:59` BEFORE
  the snapshot is armed, so BOTH exit branches can orphan. Fix: add a `lock.ReleaseCurrent()` package-level
  nil-safe wrapper (mirrors `SetSnapshot`) + an `OnRescueExit func()` injectable seam on `signal.Options`
  (defaulted to a no-op, like `RescueFormat`/`Kill`/`Exit`) + call `h.opts.OnRescueExit()` immediately before
  BOTH `h.opts.Exit(...)` calls in `handle()`. The signal package stays STDLIB-ONLY (it cannot import
  internal/lock) — `OnRescueExit` is wired to `lock.ReleaseCurrent` in main.go by the NEXT task (P1.M2.T2.S2),
  not this one.

  ⚠️ **THE central design call — `ReleaseCurrent` mirrors `SetSnapshot` EXACTLY: `if l := current.Load();
  l != nil { l.Release() }`.** Nil-safe (current==nil → no-op) + idempotent (Release's `l.file==nil` guard).
  Place it immediately after the package-level `SetSnapshot` (its twin — both are `current.Load()` nil-safe
  bridges so library layers / the signal seam can act on the singleton without holding a pointer). Doc comment
  mirrors SetSnapshot's style + names the FR52 §18.5 exit-path-release purpose.

  ⚠️ **THE second design call — `OnRescueExit` is the injectable-seam pattern (signal stays stdlib-only).**
  `internal/signal` deliberately imports NO stagecoach packages (its doc says so; `RescueFormat` avoids a
  signal↔generate cycle the same way). `OnRescueExit func()` is a new `Options` field alongside
  `RescueFormat`/`Out`/`Kill`/`Exit`, defaulted to `func(){}` (no-op) in `Install`. main.go (P1.M2.T2.S2)
  wires `OnRescueExit: lock.ReleaseCurrent`; THIS task ships the no-op default → **byte-identical behavior**
  (the seam exists but does nothing until S2 wires it). S1 = the SEAM; S2 = the WIRING. S1 does NOT touch main.go.

  ⚠️ **THE third design call — call `OnRescueExit()` before BOTH exit branches in `handle()`.** The lock is
  acquired at `default_action.go:59` BEFORE the snapshot is armed (deep in CommitStaged/runPipeline via
  SetSnapshot), so a Ctrl-C in the PRE-snapshot window hits the `tree==""` branch (`Exit(130/143)`) while the
  lock IS held → orphan. A POST-snapshot Ctrl-C hits the `tree!=""` branch (`Exit(3)`) → same orphan. BOTH
  need `OnRescueExit()` immediately before the `h.opts.Exit(...)` call.

  ⚠️ **THE fourth design call — do NOT add `OnRescueExit` to `RestoreDefault`.** RestoreDefault runs before
  `update-ref` on the SUCCESS path; it does NOT call `os.Exit`, so `defer locker.Release()` at
  default_action.go:67 runs normally and removes the file. Adding OnRescueExit there would be a redundant
  double-release (harmless — Release is idempotent — but pointless and confusing). OnRescueExit is EXIT-PATH ONLY.

  ⚠️ **THE fifth design call — S1 ships NO committed tests (P1.M2.T3.S2 owns the signal-seam tests).** Mirrors
  the sibling P1.M2.T1.S2's discipline. `ReleaseCurrent` is exported (no `unused`/U1000 lint); `OnRescueExit`
  is a struct field defaulted + called in `handle()`. The no-op default = byte-identical behavior → existing
  tests stay green unchanged. P1.M2.T3.S2 ("Exit-path release signal tests") injects a recording OnRescueExit
  and asserts it fires before the recording Exit on BOTH branches. S1's Validation Loop includes a THROWAWAY
  (non-committed) end-to-end sanity check (Level 3).

  ⚠️ **Co-edit with the parallel P1.M2.T1.S2 (non-overlapping).** S2 (reapStaleLocks) adds `reapStaleLocks`
  (near parseContents) + an Acquire wiring line + 3 doc fixes to `lock.go`. S1 adds `ReleaseCurrent` (after
  SetSnapshot). Different functions, different regions → no textual merge conflict. S1 does NOT touch
  docs/how-it-works.md (S2 owns the reaping rewrites, which already mention "the signal path releases the file
  before exiting").

  Deliverable (edits to 2 existing files — NO new files, NO new deps, NO main.go): (1) `internal/lock/lock.go`
  — add `ReleaseCurrent()` after the package-level `SetSnapshot`; (2) `internal/signal/signal.go` — add
  `OnRescueExit func()` to `Options` + default it in `Install` + call it before both Exit calls in `handle()`.
  INPUT = lock.current singleton + Locker.Release + signal.Options/handle (all read from the live file).
  OUTPUT = the exit-path lock-release seam; after S2 wires it, signal-rescue no longer orphans the lock file.
  DOCS = Mode A doc comments on ReleaseCurrent + OnRescueExit. SCOPE: `internal/lock/lock.go` +
  `internal/signal/signal.go` ONLY.
---

## Goal

**Feature Goal**: Add the FR52 §18.5 "Exit-path release" seam — a `lock.ReleaseCurrent()` nil-safe wrapper
(mirroring `SetSnapshot`) + an `OnRescueExit func()` injectable seam on `signal.Options` — and call
`OnRescueExit()` before both `os.Exit` branches in `signal.handle()`, so that (once P1.M2.T2.S2 wires
`OnRescueExit: lock.ReleaseCurrent` in main.go) a signal-rescue exit removes the lock file instead of
orphaning it. The signal package stays stdlib-only; this task ships the seam (no-op default), not the wiring.

**Deliverable** (edits to 2 existing files):
1. **`internal/lock/lock.go`** — add `func ReleaseCurrent()` immediately after the package-level `SetSnapshot`
   (its twin): `if l := current.Load(); l != nil { l.Release() }`. Doc comment mirroring SetSnapshot + naming
   FR52 §18.5.
2. **`internal/signal/signal.go`** — (a) add `OnRescueExit func()` to the `Options` struct (alongside
   RescueFormat/Out/Kill/Exit); (b) default it to `func(){}` in `Install()` (after the other defaults);
   (c) in `handle()`, call `h.opts.OnRescueExit()` immediately before `h.opts.Exit(3)` (post-snapshot rescue)
   AND before `h.opts.Exit(exitCodeForSignal(sig))` (pre-snapshot 130/143).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l` clean; `go test -race ./...` green with
existing tests UNCHANGED (the no-op default = byte-identical behavior). `ReleaseCurrent` is nil-safe +
idempotent; `OnRescueExit` is defaulted + called on both exit branches; `RestoreDefault` is untouched; signal
imports NO stagecoach package; go.mod/go.sum unchanged; only `internal/lock/lock.go` + `internal/signal/signal.go`
touched.

## User Persona

**Target User**: The user who Ctrl-C's a `stagecoach` run. After S2 wires the seam, the signal-rescue exit
removes the lock file (today `os.Exit` skips `defer Release`, orphaning it). Transitively the FR52 contention
path + the reaping backstop (P1.M2.T1.S2): prevention (this seam) + backstop (reap-on-Acquire) together keep
the lock directory clean.

**Use Case**: A user Ctrl-C's a run after the snapshot → `handle()` prints the rescue + `Exit(3)`. Today the
lock file is orphaned (defer skipped). After this seam + S2's wiring, `OnRescueExit` (= `lock.ReleaseCurrent`)
removes the file BEFORE `os.Exit`.

**User Journey**: `stagecoach` → `lock.Acquire` (default_action.go:59) → `defer Release` (:67) → snapshot
armed → Ctrl-C → `handle()` → print rescue → **`OnRescueExit()`** (release lock file) → `Exit(3)`. The defer
never runs (os.Exit skips it), but `OnRescueExit` already removed the file.

**Pain Points Addressed**: removes the most frequent stale-file producer (signal-rescue `os.Exit`) — the
§18.5 "Exit-path release (prevention)" — so reaping (P1.M2.T1.S2) is a backstop for SIGKILL/crash, not the hot path.

## Why

- **Closes the §18.5 "Exit-path release (prevention)" gap.** The PRD specifies it: "The signal handler
  therefore releases the lock file immediately before exiting, via the same injected-seam used for the rescue
  formatter (the signal package stays stdlib-only and cannot import the lock package)." This task IS that seam.
- **Prevention > backstop.** Reaping-on-Acquire (P1.M2.T1.S2) catches SIGKILL/crash orphans, but the
  signal-rescue `os.Exit` is the COMMON producer — preventing it at the source is cleaner than reaping later.
- **Mirrors the proven injectable-seam pattern.** `RescueFormat`/`Kill`/`Exit` are already injectable Options
  seams (defaulted in Install, wired in main.go) so the stdlib-only signal package avoids import cycles.
  `OnRescueExit` is the identical pattern for the lock. `ReleaseCurrent` mirrors `SetSnapshot` (the existing
  `current.Load()` nil-safe bridge).
- **Zero risk.** The no-op default means S1 alone changes NO behavior (the seam is inert until S2 wires it).
  Existing tests stay byte-identical. The lock package stays a stdlib-only leaf.
- **No API/config/deps change.** One exported func + one Options field + two call insertions. go.mod unchanged.

## What

`lock.ReleaseCurrent()` (nil-safe wrapper over `Release`) + `signal.Options.OnRescueExit` (injectable seam,
no-op default) + two `OnRescueExit()` calls before the Exit branches in `handle()`. No main.go wiring (S2),
no RestoreDefault change, no committed tests (P1.M2.T3.S2), no docs/how-it-works.md (S2).

### Success Criteria

- [ ] `func ReleaseCurrent()` exists in `internal/lock/lock.go` immediately after the package-level `SetSnapshot`,
      body `if l := current.Load(); l != nil { l.Release() }`, doc comment mirroring SetSnapshot + naming FR52 §18.5.
- [ ] `OnRescueExit func()` is a field on `signal.Options` (alongside RescueFormat/Out/Kill/Exit).
- [ ] `Install()` defaults `OnRescueExit` to `func(){}` when nil (after the other `if opts.X == nil` defaults).
- [ ] `handle()` calls `h.opts.OnRescueExit()` immediately before `h.opts.Exit(3)` (post-snapshot rescue) AND
      before `h.opts.Exit(exitCodeForSignal(sig))` (pre-snapshot 130/143).
- [ ] `RestoreDefault` is UNCHANGED (no OnRescueExit there — success path uses `defer Release`).
- [ ] `internal/signal` imports NO stagecoach package (stdlib-only — OnRescueExit is the seam, not an import).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/ internal/signal/`, `go test -race ./...`
      clean/green; go.mod/go.sum unchanged; existing tests byte-unchanged; only the 2 files touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the three verbatim edits (quoted
below), the "both exit branches" rationale (lock acquired pre-snapshot), the RestoreDefault exclusion, the
stdlib-only-seam pattern, and the no-committed-tests discipline. No PRD/git internals beyond "os.Exit skips defers."

### Documentation & References

```yaml
# MUST READ — the authoritative research
- docfile: plan/011_98cef660a41d/P1M2T2S1/research/release-current-and-onrescueexit.md
  why: the three verbatim edits (§1), the both-branches rationale (§2 — lock acquired at default_action.go:59
       pre-snapshot), the injectable-seam pattern (§3 — signal stdlib-only; S1=seam, S2=wiring), the
       RestoreDefault exclusion (§4), the co-edit with P1.M2.T1.S2 (§5 — non-overlapping lock.go additions),
       the lock_reaping.md Fix 2 spec (§6), the no-committed-tests discipline (§7), scope fences (§8).
  critical: §2 (BOTH branches need it) + §4 (NOT RestoreDefault) + §3 (default no-op = byte-identical; S2 wires).

# The authoritative spec — lock_reaping.md Fix 2
- docfile: plan/011_98cef660a41d/architecture/lock_reaping.md
  section: "Fix 2: Exit-Path Lock Release via Signal Seam" → ReleaseCurrent + OnRescueExit + the both-branches rule.
  why: the verbatim ReleaseCurrent body + OnRescueExit spec + "Both branches need coverage: the lock is acquired
       at default_action.go:59 BEFORE the snapshot is armed."
  critical: ReleaseCurrent is `if l := current.Load(); l != nil { l.Release() }` — copy verbatim.

# The contract — what the parallel reapStaleLocks task does (co-edit, non-overlapping)
- docfile: plan/011_98cef660a41d/P1M2T1S2/PRP.md
  why: P1.M2.T1.S2 adds reapStaleLocks (near parseContents) + Acquire wiring + 3 doc fixes to lock.go. THIS task
       adds ReleaseCurrent (after SetSnapshot). Different functions/regions → no merge conflict. S2 owns
       docs/how-it-works.md (which already mentions "the signal path releases the file before exiting").
  critical: do NOT duplicate reapStaleLocks or the doc fixes; do NOT touch docs/how-it-works.md.

# The lock file (EDIT — add ReleaseCurrent)
- file: internal/lock/lock.go
  section: the package-level SetSnapshot (~L163 — the twin to mirror); current atomic.Pointer[Locker] (~L60);
           Release (~L123 — idempotent, l.file==nil guard, closes fd + os.Remove(path) + clears singleton).
  why: ReleaseCurrent is the `current.Load()` nil-safe bridge twin of SetSnapshot. Place it immediately after
       SetSnapshot. It calls Release (already idempotent + nil-safe on the Locker).
  pattern: copy SetSnapshot's shape: `func ReleaseCurrent() { if l := current.Load(); l != nil { l.Release() } }`.
  gotcha: ZERO new imports (lock.go is stdlib-only; ReleaseCurrent uses only current.Load + Release). The package
           must STAY stdlib-only (no stagecoach imports) — ReleaseCurrent is called via the seam, not by signal directly.

# The signal file (EDIT — add OnRescueExit + calls)
- file: internal/signal/signal.go
  section: Options struct (L31-47); Install defaults (the `if opts.X == nil` block, ~L85-99); handle() (~L113-141
           — the two Exit branches: `h.opts.Exit(3)` post-snapshot + `h.opts.Exit(exitCodeForSignal(sig))` pre-snapshot).
  why: the file you edit. OnRescueExit is a new Options field (the injectable-seam pattern); default it in Install;
           call it before BOTH Exit calls in handle().
  pattern: mirror the existing seams — `RescueFormat func(...)` / `Kill func(...)` / `Exit func(int)` are Options
           fields defaulted in Install + called in handle(). OnRescueExit is `func()`, same discipline.
  gotcha: signal is STDLIB-ONLY — do NOT add an `internal/lock` import (it would break the leaf purity + create a
           cycle). OnRescueExit IS the cycle-avoidance seam. The default `func(){}` keeps behavior byte-identical.

# The lock acquisition site (READ ONLY — confirms both branches need OnRescueExit)
- file: internal/cmd/default_action.go
  section: L59 (locker, lockErr := lock.Acquire(repoDir)) + L67 (defer locker.Release()).
  why: PROVES the lock is acquired BEFORE the snapshot is armed (the snapshot is set deep in CommitStaged/runPipeline
       via signal.SetSnapshot). So a pre-snapshot Ctrl-C (Exit 130/143) finds the lock HELD → os.Exit orphans it.
       BOTH exit branches need OnRescueExit.
  gotcha: do NOT edit default_action.go — the wiring (OnRescueExit: lock.ReleaseCurrent) is P1.M2.T2.S2 in main.go,
           not here. This site is read-only context.

# The requirement
- file: PRD.md §18.5 (h3.91) "Concurrency: the per-repo run lock (FR52)" — "Exit-path release (prevention)."
  why: the PRD is the source: "The signal handler therefore releases the lock file immediately before exiting,
       via the same injected-seam used for the rescue formatter (the signal package stays stdlib-only and cannot
       import the lock package)." This task IS that injected seam.
```

### Current Codebase tree (relevant slice)

```bash
internal/lock/
  lock.go              # current atomic.Pointer[Locker] (~L60) + SetSnapshot (~L163, the twin) + Release (~L123)  ← EDIT (add ReleaseCurrent after SetSnapshot)
  lock_unix.go         # processAlive — P1.M2.T1.S1 (frozen)                                                                                    ← (NO edit)
  lock_windows.go      # processAlive — P1.M2.T1.S1 (frozen)                                                                                   ← (NO edit)
  lock_test.go         # existing lock tests (P1.M2.T3.S1 adds reaping tests)                                                                   ← (NO edit; no committed tests here)
internal/signal/
  signal.go            # Options (L31) + Install defaults (~L85) + handle (~L113, two Exit branches)  ← EDIT (OnRescueExit field + default + 2 calls)
  signal_test.go       # existing signal tests (P1.M2.T3.S2 adds exit-path-release tests)                                                  ← (NO edit; no committed tests here)
internal/cmd/
  default_action.go    # L59 Acquire + L67 defer Release (READ ONLY — confirms both branches need the seam)                              ← (NO edit)
go.mod / go.sum        # unchanged (no new dep; both packages stay stdlib-only)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Edits: internal/lock/lock.go (ReleaseCurrent after SetSnapshot) + internal/signal/signal.go
# (OnRescueExit field + Install default + 2 handle() calls).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (both branches): the lock is acquired at default_action.go:59 BEFORE the snapshot is armed. So a
// pre-snapshot Ctrl-C (tree=="" → Exit(130/143)) finds the lock HELD → os.Exit skips defer Release → orphan.
// OnRescueExit MUST precede BOTH h.opts.Exit(3) AND h.opts.Exit(exitCodeForSignal(sig)). Missing either leaves
// a window where the file is orphaned.

// CRITICAL (stdlib-only signal): do NOT add an internal/lock import to signal.go. The signal package is
// deliberately stdlib-only (its doc says so; RescueFormat avoids a signal↔generate cycle the same way).
// OnRescueExit IS the cycle-avoidance seam. main.go (P1.M2.T2.S2) wires OnRescueExit: lock.ReleaseCurrent.

// CRITICAL (NOT RestoreDefault): do NOT add OnRescueExit to RestoreDefault. RestoreDefault runs before update-ref
// on the SUCCESS path (no os.Exit) → defer locker.Release() at default_action.go:67 runs → file removed. Adding
// OnRescueExit there is a redundant double-release (harmless but pointless + confusing). OnRescueExit is EXIT-PATH ONLY.

// CRITICAL (no-op default = byte-identical): Install defaults OnRescueExit to func(){}. So after S1 lands alone,
// behavior is UNCHANGED (handle() calls a no-op before exiting). The actual lock removal begins when S2 wires
// lock.ReleaseCurrent. S1 = the SEAM; S2 = the WIRING. Do NOT touch main.go.

// GOTCHA (mirror SetSnapshot verbatim): ReleaseCurrent is `if l := current.Load(); l != nil { l.Release() }`.
// Nil-safe (current==nil → no-op) + idempotent (Release's l.file==nil guard handles a double-call). Place it
// immediately after the package-level SetSnapshot (its twin).

// GOTCHA (no committed tests): S1 ships the seam only. ReleaseCurrent is exported (no unused-lint); OnRescueExit
// is a struct field defaulted + called in handle(). The no-op default → existing tests stay green unchanged.
// P1.M2.T3.S2 owns the signal-seam tests (inject a recording OnRescueExit; assert it fires before the recording
// Exit on BOTH branches).

// GOTCHA (co-edit with P1.M2.T1.S2): both tasks edit lock.go — S2 adds reapStaleLocks (near parseContents) +
// Acquire wiring + 3 doc fixes; S1 adds ReleaseCurrent (after SetSnapshot). Different functions/regions → no
// textual conflict. Do NOT touch docs/how-it-works.md (S2 owns it; it already mentions the signal-path release).

// GOTCHA: the signal package doc comment says "imports NO stagecoach packages (stdlib-only)" — keep it truthful.
// Adding the OnRescueExit seam does NOT violate it (the seam is a func() field, not an import). Do NOT edit the
// package doc unless to mention OnRescueExit is the lock-release seam (optional).
```

## Implementation Blueprint

### Data models and structure

No new types. One exported func + one Options field + two call insertions.

```go
// internal/lock/lock.go — ReleaseCurrent (place immediately after the package-level SetSnapshot):
// ReleaseCurrent releases the current lock holder, if any (nil-safe no-op when no lock is held). It is the
// exit-path seam for FR52 §18.5: signal.handle() cannot import internal/lock (the signal package is
// stdlib-only), so the signal handler calls an OnRescueExit callback (wired in main.go to ReleaseCurrent)
// immediately before os.Exit — removing the lock file that os.Exit's defer-skipping would otherwise orphan.
// Idempotent (Release's l.file==nil guard) and nil-safe (current==nil → no-op), exactly mirroring SetSnapshot.
func ReleaseCurrent() {
	if l := current.Load(); l != nil {
		l.Release()
	}
}
```

```go
// internal/signal/signal.go — Options: add OnRescueExit alongside the other seams:
type Options struct {
	// ... existing RescueFormat / Out / Kill / Exit fields ...
	// OnRescueExit is called immediately before the handler exits on BOTH signal paths (the post-snapshot
	// rescue exit 3 AND the pre-snapshot 130/143 exit). It is the exit-path lock-release seam: wired in main.go
	// to lock.ReleaseCurrent so the lock file is removed before os.Exit skips the deferred Release (FR52 §18.5).
	// Defaulted to a no-op here so the signal package stays stdlib-only (it cannot import internal/lock) and so
	// library use of pkg/stagecoach (no Install wiring) is unaffected.
	OnRescueExit func()
}

// internal/signal/signal.go — Install: default OnRescueExit (after the other `if opts.X == nil` defaults):
	if opts.OnRescueExit == nil {
		opts.OnRescueExit = func() {} // no-op default; main.go wires lock.ReleaseCurrent (FR52 exit-path release)
	}

// internal/signal/signal.go — handle(): call OnRescueExit before BOTH exit branches:
	if tree != "" {
		fmt.Fprintln(h.opts.Out, h.opts.RescueFormat(tree, parent, cand))
		h.opts.OnRescueExit()           // ← ADD: release the lock file before os.Exit orphans it
		h.opts.Exit(3)                  // §18.2: post-snapshot → exit 3
		return
	}
	h.opts.OnRescueExit()               // ← ADD: pre-snapshot exit too (lock held from default_action.go:59)
	h.opts.Exit(exitCodeForSignal(sig)) // 130 SIGINT / 143 SIGTERM
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: lock.go — add ReleaseCurrent (after the package-level SetSnapshot)
  - ADD `func ReleaseCurrent()` per the Data Models block, immediately after SetSnapshot (its twin).
  - BODY: `if l := current.Load(); l != nil { l.Release() }`.
  - DOC: mirror SetSnapshot's style; name FR52 §18.5; nil-safe + idempotent; the exit-path seam purpose.
  - GOTCHA: ZERO new imports (current.Load + Release are in-package). Place after SetSnapshot, NOT in Acquire.

Task 2: signal.go — add OnRescueExit to Options + default in Install
  - ADD `OnRescueExit func()` to the Options struct (after Exit), with the doc comment (FR52 §18.5 seam;
    stdlib-only; no-op default; wired in main.go).
  - ADD the default in Install: `if opts.OnRescueExit == nil { opts.OnRescueExit = func() {} }` (after the
    other `if opts.X == nil` defaults — RescueFormat/Out/Kill/Exit).
  - GOTCHA: do NOT import internal/lock (signal stays stdlib-only). The default keeps behavior byte-identical.

Task 3: signal.go — call OnRescueExit before BOTH exit branches in handle()
  - IN handle(), insert `h.opts.OnRescueExit()` immediately before `h.opts.Exit(3)` (the post-snapshot rescue
    branch, after the Fprintln rescue print) AND immediately before `h.opts.Exit(exitCodeForSignal(sig))`
    (the pre-snapshot branch).
  - GOTCHA: BOTH branches (the lock is held from default_action.go:59, pre-snapshot). Do NOT add OnRescueExit
    to RestoreDefault (success path uses defer Release).

Task 4: VERIFY (no further file change)
  - RUN `gofmt -w internal/lock/lock.go internal/signal/signal.go`; `go vet ./internal/lock/ ./internal/signal/`;
    `go build ./...`; `go test -race ./...` (no-op default = byte-identical → no regression).
  - go.mod/go.sum byte-unchanged. signal imports NO stagecoach package. RestoreDefault byte-unchanged.
    Only the 2 files touched. NO committed tests (P1.M2.T3.S2).
  - THROWAWAY end-to-end sanity check (Level 3, non-committed): inject OnRescueExit=lock.ReleaseCurrent,
    Acquire a lock, call handle(SIGINT) with a recording Exit, assert the lock file is gone.
```

### Implementation Patterns & Key Details

```go
// ReleaseCurrent — the verbatim SetSnapshot twin (nil-safe + idempotent):
func ReleaseCurrent() {
	if l := current.Load(); l != nil {
		l.Release()   // Release: l.file==nil guard (idempotent) → close fd + os.Remove(path) + clear singleton
	}
}

// The injectable-seam discipline (signal stays stdlib-only) — OnRescueExit mirrors RescueFormat/Kill/Exit:
//   Options field (func()) → defaulted in Install → called in handle() → WIRED in main.go (P1.M2.T2.S2).
//   signal NEVER names lock → no import → no cycle. The default func(){} = byte-identical behavior until S2.

// The both-branches rule (lock acquired pre-snapshot at default_action.go:59):
//   handle(): tree!="" → rescue print → OnRescueExit() → Exit(3)        // post-snapshot
//             tree=="" → OnRescueExit() → Exit(exitCodeForSignal(sig))  // pre-snapshot (lock already held)
//   BOTH need it; os.Exit skips defer Release in both windows.
```

```go
// Level 3 throwaway end-to-end sanity check (NOT committed — P1.M2.T3.S2 owns the signal tests):
// In a scratch test (//go:build ignore or a temp _test.go you delete):
//   - isolate XDG, Acquire a lock (file exists).
//   - h := &Handler{opts: Options{OnRescueExit: lock.ReleaseCurrent, Exit: func(int){}, Out: io.Discard,
//                              RescueFormat: func(...) string {return ""}}}
//   - h.handle(syscall.SIGINT)   // pre-snapshot branch (snapTree=="")
//   - ASSERT the lock file is GONE (OnRescueExit → ReleaseCurrent removed it); the recording Exit captured 130.
//   - repeat with snapTree set (post-snapshot) → rescue branch → Exit 3; file gone.
// This proves the seam end-to-end without adding a committed test.
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — no new dep; both packages stay stdlib-only. go mod tidy is a no-op.

PACKAGE EDGES:
  - internal/lock → (stdlib only). ReleaseCurrent adds NO import (current.Load + Release are in-package).
    The package stays a stdlib-only leaf.
  - internal/signal → (stdlib only). OnRescueExit is a func() field, NOT an import. signal does NOT import
    internal/lock (the seam IS the cycle-avoidance). NO new import.

FROZEN / NOT-EDITED:
  - internal/cmd/default_action.go (L59 Acquire + L67 defer Release — READ ONLY; the wiring is P1.M2.T2.S2
    in main.go, NOT here).
  - cmd/stagecoach/main.go (P1.M2.T2.S2 wires OnRescueExit: lock.ReleaseCurrent in the Install call).
  - internal/lock/{lock_unix,lock_windows,lock_test,lock_unix_test}.go (P1.M2.T1.S1/S2 + P1.M2.T3.S1).
  - internal/signal/signal_test.go (P1.M2.T3.S2 owns the exit-path-release tests).
  - docs/how-it-works.md (P1.M2.T1.S2 owns the reaping rewrites — already mention the signal-path release).
  - RestoreDefault (success path; defer Release handles cleanup — NO OnRescueExit there).

DOWNSTREAM CONSUMER (do NOT implement here):
  - P1.M2.T2.S2 (next): wires OnRescueExit: lock.ReleaseCurrent in main.go's Install call. After S2, the
    signal-rescue exit removes the lock file (this seam's purpose). S1 ships the no-op default (byte-identical).
  - P1.M2.T3.S2: the committed signal-seam tests (recording OnRescueExit; assert it fires before the recording
    Exit on BOTH branches; assert lock file removed when wired to ReleaseCurrent).

NO DATABASE / ROUTES / CONFIG / CLI / NEW FILES / COMMITTED TESTS / DOCS/*.MD.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/lock/lock.go internal/signal/signal.go
test -z "$(gofmt -l internal/lock/ internal/signal/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/lock/ ./internal/signal/   # catches a malformed func / unused field / wrong call site.
go build ./...                               # both packages compile; no caller breaks (additive seam).
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm ReleaseCurrent + OnRescueExit landed:
grep -n 'func ReleaseCurrent' internal/lock/lock.go                  # 1 hit (after SetSnapshot)
grep -c 'OnRescueExit' internal/signal/signal.go                     # ≥4: field + doc + Install default + 2 handle calls
# Confirm signal still imports NO stagecoach package (stdlib-only preserved):
grep -n 'dustin/stagecoach' internal/signal/signal.go && echo "BAD: signal imports a stagecoach pkg" || echo "signal stdlib-only (good)"
# Confirm OnRescueExit is NOT in RestoreDefault:
grep -A3 'func RestoreDefault' internal/signal/signal.go | grep 'OnRescueExit' && echo "BAD: in RestoreDefault" || echo "RestoreDefault clean (good)"
```

### Level 2: Unit Tests (Component Validation) — the no-op-default gate

```bash
go test -race ./internal/lock/ ./internal/signal/   # existing tests unchanged (the no-op default = byte-identical behavior).
go test -race ./...                                 # full module — no regression (additive seam; nothing wires it yet).
# Expected: green throughout. S1 adds NO committed test (P1.M2.T3.S2 owns the signal-seam tests). The no-op
#   default means handle() calls a func(){} before exiting — observably identical to today. Existing signal
#   tests (which inject their own Exit/RescueFormat) are unaffected UNLESS they assert the EXACT call sequence
#   in handle() — re-check any such test (it would need OnRescueExit in its injected Options; the default handles nil).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only the 2 files changed:
git diff --name-only | grep -Ev '^internal/lock/lock\.go$|^internal/signal/signal\.go$' \
  && echo "UNEXPECTED file changed" || echo "only lock.go + signal.go changed (good)"
# Confirm RestoreDefault + main.go + default_action.go byte-unchanged:
git diff --exit-code -- internal/cmd/default_action.go cmd/stagecoach/main.go && echo "default_action.go + main.go UNCHANGED (expected — wiring is S2)"
# THROWAWAY end-to-end sanity check (NOT committed — P1.M2.T3.S2 owns committed tests). Confirms the seam works
# once wired (this task ships the no-op default; the sanity check injects ReleaseCurrent to prove the mechanism):
cat > /tmp/seam_sanity_test.go <<'EOF'
//go:build ignore
package main
// Minimal: Acquire a lock (file exists); build a Handler with OnRescueExit=lock.ReleaseCurrent + a recording
// Exit; call handle(SIGINT) (pre-snapshot, snapTree==""); assert the lock file is GONE + Exit captured 130.
// Repeat with snapTree set (post-snapshot → rescue → Exit 3). Run as a throwaway `go run` against a temp XDG dir.
EOF
echo "throwaway sanity check sketched (run manually; P1.M2.T3.S2 adds the committed version)"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# THE lint gate — ReleaseCurrent is exported (no U1000); OnRescueExit is a field defaulted + called (no unused):
make lint 2>&1 | grep -iE 'ReleaseCurrent|OnRescueExit|unused|U1000' && echo "BAD: seam flagged" || echo "seam not flagged (good)"
# Seam-completeness audit: OnRescueExit appears in (a) Options struct, (b) Install default, (c) handle() pre-Exit(3),
# (d) handle() pre-Exit(exitCodeForSignal) — all 4 sites:
grep -n 'OnRescueExit' internal/signal/signal.go
# Stdlib-only audit: neither package gained a stagecoach import:
git diff internal/lock/lock.go internal/signal/signal.go | grep -E '^\+\s*"github.com/dustin' && echo "BAD: new stagecoach import" || echo "no new stagecoach imports (good)"
# Cross-platform build (signal compiles on Windows — OnRescueExit is a plain func() field; SIGTERM is a no-op there):
GOOS=windows go build ./internal/signal/ && echo "windows build OK" || echo "windows build FAILED"
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `gofmt -l`, `go vet ./...`, `go build ./...`, `go mod tidy` no-op; signal imports NO stagecoach pkg.
- [ ] Level 2 green: `go test -race ./...` (no-op default = byte-identical; existing tests unchanged; S1 adds none).
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only lock.go + signal.go changed; default_action.go + main.go byte-unchanged.
- [ ] Level 4: `make lint` green (no U1000/unused for ReleaseCurrent/OnRescueExit); `GOOS=windows go build ./internal/signal/` OK.

### Feature Validation

- [ ] `ReleaseCurrent()` exists after SetSnapshot; body `if l := current.Load(); l != nil { l.Release() }`; nil-safe + idempotent.
- [ ] `OnRescueExit func()` is an Options field; defaulted to `func(){}` in Install.
- [ ] `handle()` calls `h.opts.OnRescueExit()` before `Exit(3)` AND before `Exit(exitCodeForSignal(sig))`.
- [ ] `RestoreDefault` is UNCHANGED (no OnRescueExit).
- [ ] signal stays stdlib-only (OnRescueExit is the seam, not an import).

### Code Quality Validation

- [ ] `ReleaseCurrent` mirrors `SetSnapshot` (the `current.Load()` nil-safe bridge twin).
- [ ] `OnRescueExit` mirrors the existing injectable-seam pattern (RescueFormat/Kill/Exit).
- [ ] No scope creep into main.go wiring (P1.M2.T2.S2), RestoreDefault, committed tests (P1.M2.T3.S2),
      docs/how-it-works.md (P1.M2.T1.S2), or reapStaleLocks (P1.M2.T1.S2).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Mode-A doc comments on `ReleaseCurrent` (FR52 §18.5; nil-safe + idempotent; the exit-path seam) + `OnRescueExit`
      (the seam; stdlib-only; no-op default; wired in main.go).
- [ ] go.mod/go.sum byte-unchanged; no new files; no committed tests.

---

## Anti-Patterns to Avoid

- ❌ Don't import `internal/lock` from `internal/signal`. The signal package is deliberately stdlib-only (its
  doc says so; `RescueFormat` avoids a signal↔generate cycle the same way). `OnRescueExit func()` IS the
  cycle-avoidance seam — main.go (S2) wires it to `lock.ReleaseCurrent`. Adding the import breaks leaf purity.
- ❌ Don't call `OnRescueExit` in only ONE exit branch. The lock is acquired at `default_action.go:59` BEFORE
  the snapshot is armed, so a PRE-snapshot Ctrl-C (Exit 130/143) finds the lock held → orphan. BOTH
  `Exit(3)` (post-snapshot) AND `Exit(exitCodeForSignal(sig))` (pre-snapshot) need `OnRescueExit()` before them.
- ❌ Don't add `OnRescueExit` to `RestoreDefault`. RestoreDefault is the SUCCESS path (before update-ref; no
  os.Exit) → `defer locker.Release()` at default_action.go:67 runs → file removed. Adding OnRescueExit there is
  a redundant double-release (harmless but pointless + confusing). OnRescueExit is EXIT-PATH ONLY.
- ❌ Don't touch `main.go` to wire `OnRescueExit: lock.ReleaseCurrent`. That's P1.M2.T2.S2 (the next task).
  S1 ships the no-op default → byte-identical behavior. Wiring it here overlaps S2 + steals its scope.
- ❌ Don't deviate from the `ReleaseCurrent` body. It is `if l := current.Load(); l != nil { l.Release() }` —
  the verbatim `SetSnapshot` twin (lock_reaping.md Fix 2). Don't add a return value, an error, or a second
  guard — Release is already idempotent + nil-safe.
- ❌ Don't add a committed test. P1.M2.T3.S2 owns "Exit-path release signal tests (signal_test.go)." S1's seam
  is exercised by (a) the no-op default keeping existing tests green, and (b) P1.M2.T3.S2's recording-OnRescueExit
  tests. A throwaway Level-3 sanity check is fine (not committed).
- ❌ Don't touch `docs/how-it-works.md`. P1.M2.T1.S2 owns the reaping rewrites there (which already mention "the
  signal path releases the file before exiting"). S1's DOCS are Mode A code comments on ReleaseCurrent + OnRescueExit.
- ❌ Don't duplicate `reapStaleLocks` or the lock.go doc-fixes (P1.M2.T1.S2). Both tasks edit lock.go but add
  DIFFERENT functions (reapStaleLocks near parseContents vs ReleaseCurrent after SetSnapshot) — non-overlapping.
  Don't touch the 3 over-claim doc sites (lines 2/31/67) — S2 owns them.
- ❌ Don't change `RestoreDefault`, `default_action.go`, `lock_unix.go`/`lock_windows.go`, or any test file.
  RestoreDefault is success-path; default_action.go is read-only context; the platform files are S1(frozen);
  test files are P1.M2.T3.S1/S2.
- ❌ Don't change go.mod/go.sum or add files. One exported func + one Options field + two call insertions, in
  2 existing files.
- ❌ Don't skip `go vet`/`go build`/`go test -race ./...`/`make lint`. They catch a malformed func, a stagecoach
  import in signal (the stdlib-only audit), an unfired seam (the OnRescueExit count), and any regression from
  the (byte-identical) no-op default.

---

## Confidence Score

**9/10** — a small, verbatim (from lock_reaping.md Fix 2) seam: one nil-safe wrapper mirroring an existing
twin (`SetSnapshot`), one injectable Options field mirroring the existing seam pattern (`RescueFormat`/`Kill`/
`Exit`), and two call insertions before well-identified exit branches. The design is fully specified by the
authoritative architecture doc + the live signal.go structure, the no-op default makes it byte-identical/zero-
risk, and the scope fences (S2 wires main.go; P1.M2.T3.S2 tests; S2 owns how-it-works.md + reapStaleLocks) are
crisp. The -1 reserves for the throwaway sanity check (the only non-deterministic step — it proves the wired
seam end-to-end without a committed test) and the slight chance an existing signal test asserts handle()'s
exact call sequence (it would need OnRescueExit in its injected Options; the nil-default handles it).
