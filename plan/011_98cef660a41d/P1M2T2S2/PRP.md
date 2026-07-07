---
name: "P1.M2.T2.S2 — Wire OnRescueExit: lock.ReleaseCurrent in main.go (FR52 §18.5 exit-path release)"
description: |
  THE WIRING half of §18.5's exit-path lock release (FR52). S1 (P1.M2.T2.S1, parallel) ships the SEAM —
  `lock.ReleaseCurrent()` (nil-safe wrapper twin of `SetSnapshot`) + `signal.Options.OnRescueExit func()`
  (injectable seam, no-op default, called before BOTH `os.Exit` branches in `handle()`). **This task is the
  one-line WIRING**: in `cmd/stagecoach/main.go` (the SOLE `signal.Install` caller) add the `internal/lock`
  import + set `OnRescueExit: lock.ReleaseCurrent` in the `signal.Options` literal. After this, a Ctrl-C /
  SIGTERM after the lock is acquired removes the lock file BEFORE `os.Exit` skips `defer locker.Release()`,
  so the signal-rescue path stops orphaning the lock file (the §18.5 "prevention"; reaping becomes the
  SIGKILL/crash backstop).

  This is a 2-line edit to ONE file (`cmd/stagecoach/main.go`). No new files, no new types, no logic change,
  no deps, no docs, no tests.

  ⚠️ **THE hard dependency — S1 MUST land first (STOP-and-flag guard).** The repo is currently MID-S1:
  `ReleaseCurrent` and `OnRescueExit` do NOT yet exist. If S1 hasn't landed, `go build` fails with
  `unknown field 'OnRescueExit'` + `undefined: lock.ReleaseCurrent`. **If those errors appear, STOP and
  flag — do NOT add the field/func yourself (S1's scope) and do NOT wire a closure that imports lock into
  signal (breaks signal's stdlib-only purity).** The orchestrator runs S1 → then S2.

  ⚠️ **THE design call — a direct function-value assignment, NOT a closure.** S1 defines `OnRescueExit
  func()` and `func ReleaseCurrent()`; the wiring is `OnRescueExit: lock.ReleaseCurrent` (no `()`, no
  wrapper). `ReleaseCurrent` is nil-safe (`current.Load()==nil → no-op`) + idempotent (`Release`'s
  `l.file==nil` guard), so a UNIVERSAL wire is correct — no "only wire if committing" conditional. main.go's
  `Install` runs for every invocation incl. read-only subcommands (which bypass `Acquire` → no lock held →
  `ReleaseCurrent` is a no-op). Safe on every path.

  ⚠️ **THE import-placement detail.** `gofmt` requires imports alphabetical within the group; `internal/lock`
  lands between `internal/generate` and `internal/signal`. No import cycle — `main` is the package root and
  `internal/lock` is a stdlib-only leaf (lock_reaping.md: "adding lock creates no cycle").

  ⚠️ **THE scope fences.** This task adds NOTHING to `internal/lock/lock.go` (S1 + P1.M2.T1.S2 own it),
  `internal/signal/signal.go` (S1 owns it), `internal/cmd/default_action.go` (read-only context), any test
  file (P1.M2.T3.S2 owns the signal-seam tests), any `docs/*.md` (item says "DOCS: none"), or go.mod/go.sum.

  Deliverable: MODIFIED `cmd/stagecoach/main.go` — (a) `"github.com/dustin/stagecoach/internal/lock"` added to
  the import block; (b) `OnRescueExit: lock.ReleaseCurrent` added to the `signal.Options` literal. OUTPUT:
  the signal-rescue exit removes the lock file instead of orphaning it (FR52 §18.5 prevention). INPUT = S1's
  `ReleaseCurrent` + `OnRescueExit` seam (assumed complete). DOCS = none.
---

## Goal

**Feature Goal**: Wire the FR52 §18.5 exit-path lock-release seam — set `OnRescueExit: lock.ReleaseCurrent`
in main.go's `signal.Install` call — so that on a SIGINT/SIGTERM after the lock is acquired, the signal
handler removes the lock file before `os.Exit` skips the deferred `Release`. One wiring site (main.go is the
sole `signal.Install` caller); two line additions.

**Deliverable** (edit to 1 existing file — `cmd/stagecoach/main.go`):
1. Add `"github.com/dustin/stagecoach/internal/lock"` to the import block (alphabetical, between
   `internal/generate` and `internal/signal`).
2. Add `OnRescueExit: lock.ReleaseCurrent,` to the `signal.Options{ ... }` literal (adjacent to
   `RescueFormat:`).

No new files, no new types, no logic change, no deps, no docs, no tests.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l cmd/stagecoach/main.go` clean; `go test
-race ./...` green (no regression — the wiring only activates on the signal path, which existing unit tests
don't exercise through main.go); `OnRescueExit: lock.ReleaseCurrent` is present and `internal/lock` is
imported; go.mod/go.sum unchanged; only `cmd/stagecoach/main.go` touched. (The end-to-end "Ctrl-C removes the
lock file" behavior is asserted by P1.M2.T3.S2's committed tests, not here.)

## User Persona

**Target User**: The user who Ctrl-C's a `stagecoach` run after the lock is acquired. Today `os.Exit` (post-
or pre-snapshot) skips `defer locker.Release()`, orphaning the lock *file* (`flock` still auto-releases —
only the file is orphaned). After this wiring, the signal handler removes the file before exiting.

**Use Case**: A user runs `stagecoach`, the lock is acquired (default_action.go:59), then they Ctrl-C during
generation. `signal.handle()` fires `OnRescueExit` (= `lock.ReleaseCurrent`) → removes the lock file → then
`os.Exit(3)`. The deferred `Release` (default_action.go:67) never runs (os.Exit skips it), but the file is
already gone.

**User Journey**: `stagecoach` → `lock.Acquire` → `defer Release` → snapshot armed → Ctrl-C → `handle()` →
print rescue → `OnRescueExit()` (release lock file) → `Exit(3)`. No orphaned file.

**Pain Points Addressed**: removes the most frequent stale-lock-*file* producer (signal-rescue `os.Exit`),
the §18.5 "Exit-path release (prevention)" — so reaping-on-Acquire (P1.M2.T1.S2) is a backstop for
SIGKILL/crash, not the hot path.

## Why

- **Closes the §18.5 "Exit-path release (prevention)" gap.** PRD §18.5: "The signal handler therefore
  releases the lock file immediately before exiting, via the same injected-seam used for the rescue formatter
  (the signal package stays stdlib-only and cannot import the lock package)." S1 built the seam; THIS task
  is the wiring that turns it on.
- **Prevention > backstop.** Reaping-on-Acquire catches SIGKILL/crash orphans, but the signal-rescue
  `os.Exit` is the COMMON producer — preventing it at the source is cleaner than reaping later.
- **Mirrors the proven injectable-seam pattern.** `RescueFormat` is already wired in main.go to
  `generate.FormatRescue` (so the stdlib-only signal package avoids a signal↔generate cycle). `OnRescueExit`
  is the identical pattern for the lock: wired in main.go to `lock.ReleaseCurrent`, avoiding a signal↔lock
  cycle. main.go is the natural wiring point (package root; imports both).
- **Zero risk + universal.** `ReleaseCurrent` is nil-safe + idempotent, so the wire needs no conditional —
  it's a no-op when no lock is held (read-only subcommands, pre-Acquire window, library use). S1's no-op
  default meant S1 alone changed nothing; S2 is the intended behavior flip.
- **No API/config/deps/docs change.** Two lines in main.go. go.mod unchanged.

## What

Add the `internal/lock` import + set `OnRescueExit: lock.ReleaseCurrent` in main.go's `signal.Options`
literal. No new files, no new types, no logic change, no docs, no tests.

### Success Criteria

- [ ] `cmd/stagecoach/main.go` imports `"github.com/dustin/stagecoach/internal/lock"` (alphabetical between
      `internal/generate` and `internal/signal`).
- [ ] The `signal.Options{ ... }` literal contains `OnRescueExit: lock.ReleaseCurrent,` (a function-value
      assignment — no `()`, no closure).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l cmd/stagecoach/main.go` clean — AND this is the S1-landed
      gate (the build proves `ReleaseCurrent` + `OnRescueExit` exist with matching signatures).
- [ ] `go test -race ./...` green (no regression — existing tests inject their own `Options`, not main.go's).
- [ ] go.mod/go.sum byte-unchanged; only `cmd/stagecoach/main.go` touched.
- [ ] `internal/lock/lock.go`, `internal/signal/signal.go`, `internal/cmd/default_action.go`, all test files,
      all `docs/*.md` byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the two verbatim edits (quoted
below), the "direct function-value, not a closure" note, the S1-must-land-first guard, the nil-safe/universal-
wire rationale, and the scope fences. No PRD/git internals beyond "os.Exit skips defers."

### Documentation & References

```yaml
# MUST READ — the authoritative research (the exact edits + the S1-dependency guard)
- docfile: plan/011_98cef660a41d/P1M2T2S2/research/wire-onrescueexit-main.md
  why: the 2 verbatim edits (§1), the S1-must-land-first STOP guard (§2 — the build errors you'll see if S1
       isn't done), the nil-safe/universal-wire rationale (§3), the scope fences (§4), validation (§5).
  critical: §2 (if `go build` errors with `unknown field OnRescueExit` / `undefined: lock.ReleaseCurrent`,
       S1 hasn't landed — STOP and flag) + §1 (direct function-value assignment, NOT a closure).

# The authoritative spec — lock_reaping.md Fix 2 (the wiring)
- docfile: plan/011_98cef660a41d/architecture/lock_reaping.md
  section: "Fix 2: Exit-Path Lock Release via Signal Seam" → "Wiring in cmd/stagecoach/main.go".
  why: the verbatim wiring block: `OnRescueExit: lock.ReleaseCurrent, // NEW` inside `signal.Install(...,
       signal.Options{...})`; "main already imports signal and generate; adding lock creates no cycle."
  critical: the wiring is EXACTLY `OnRescueExit: lock.ReleaseCurrent` (function value).

# The S1 contract (the seam this task wires — assume COMPLETE before this task)
- docfile: plan/011_98cef660a41d/P1M2T2S1/PRP.md
  why: S1 defines `func ReleaseCurrent()` (nil-safe twin of `SetSnapshot`: `if l := current.Load(); l != nil
       { l.Release() }`) + `signal.Options.OnRescueExit func()` (defaulted to `func(){}` in Install; called
       before BOTH Exit branches in handle()). This task CONSUMES those; it adds nothing to S1's files.
  critical: S1 ships the no-op default (byte-identical). S2 = the wiring that flips it on. If S1 isn't in the
       tree, this task won't compile — STOP (do NOT reimplement S1).

# The file to edit (READ + EDIT)
- file: cmd/stagecoach/main.go
  section: the import block (L9-17) + the signal.Install call's Options literal (L58-61).
  why: the SOLE signal.Install caller (verified: `grep -rn signal.Install` → 1 hit). Add the import + the
       OnRescueExit field here.
  pattern: mirror the existing wiring style — `RescueFormat: generate.FormatRescue` is a function-value seam
       wired from main.go; `OnRescueExit: lock.ReleaseCurrent` is the identical pattern for the lock.
  gotcha: the import must be alphabetical (gofmt) — `internal/lock` between `internal/generate` and
       `internal/signal`. The field is a function VALUE (`lock.ReleaseCurrent`, no `()`).

# The requirement
- file: PRD.md §18.5 (h3.91) "Concurrency: the per-repo run lock (FR52)" — "Exit-path release (prevention)."
  why: the PRD source: "The signal handler therefore releases the lock file immediately before exiting, via
       the same injected-seam used for the rescue formatter (the signal package stays stdlib-only and cannot
       import the lock package)." S1 = the seam; THIS task = the wiring.
- file: PRD.md §18.4 (h3.90) "Signal handling" — the two exit branches (post-snapshot rescue; pre-snapshot).
  why: context for why OnRescueExit fires on both branches (S1's concern, not this task's — but it explains
       the nil-safe/idempotent requirement that makes a universal wire safe).

# READ-ONLY context (do NOT edit)
- file: internal/lock/lock.go
  section: ReleaseCurrent (after SetSnapshot — S1 adds it) + Release (L157, idempotent l.file==nil guard).
  why: confirms ReleaseCurrent's signature (`func ReleaseCurrent()`) matches `OnRescueExit func()`, and that
       it's nil-safe + idempotent (so the universal wire is safe). This task does NOT edit lock.go.
- file: internal/signal/signal.go
  section: Options (L31) + Install defaults + handle()'s two Exit branches (S1 adds OnRescueExit).
  why: confirms OnRescueExit is a field (not an import) — signal stays stdlib-only. This task does NOT edit
       signal.go.
- file: internal/cmd/default_action.go
  section: L59 (lock.Acquire) + L67 (defer locker.Release()).
  why: confirms the lock is acquired BEFORE the snapshot is armed, so BOTH signal-exit branches can orphan
       the file (the gap this wiring closes). This task does NOT edit default_action.go.
```

### Current Codebase tree (relevant slice)

```bash
cmd/stagecoach/
  main.go                # the SOLE signal.Install caller — EDIT (import + OnRescueExit field)
internal/lock/
  lock.go                # ReleaseCurrent (S1 adds, after SetSnapshot @ L197) — NO edit (consume it)
  lock_unix.go / lock_windows.go  # processAlive (P1.M2.T1.S1, frozen) — NO edit
  lock_test.go           # P1.M2.T3.S1 adds reaping tests — NO edit
internal/signal/
  signal.go              # OnRescueExit field + Install default + 2 handle() calls (S1) — NO edit (consume it)
  signal_test.go         # P1.M2.T3.S2 adds exit-path-release tests — NO edit
internal/cmd/
  default_action.go      # L59 Acquire + L67 defer Release — READ ONLY (context)
go.mod / go.sum          # unchanged (no new dep; internal/lock already exists in-module)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. One edit: cmd/stagecoach/main.go (+internal/lock import + OnRescueExit field).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (S1 must land first): the repo is mid-S1 — ReleaseCurrent/OnRescueExit don't exist yet. If
// `go build ./cmd/stagecoach` fails with `unknown field 'OnRescueExit' in struct literal` AND/OR `undefined:
// lock.ReleaseCurrent`, S1 hasn't landed. STOP and flag — do NOT add the field/func (S1's scope), do NOT
// wire a closure that imports internal/lock into signal (breaks signal's stdlib-only purity).

// CRITICAL (direct function value, NOT a closure): the wiring is `OnRescueExit: lock.ReleaseCurrent` — no
// `()`, no `func(){ lock.ReleaseCurrent() }`. S1's `OnRescueExit func()` and `func ReleaseCurrent()` match
// exactly. A closure wrapper would compile but is needless noise.

// CRITICAL (universal wire — no conditional): main.go's Install runs for EVERY invocation incl. read-only
// subcommands (providers/config/--version) that bypass lock.Acquire. ReleaseCurrent is nil-safe
// (current.Load()==nil → no-op) + idempotent (Release's l.file==nil guard), so wiring it unconditionally is
// correct — no "only wire if committing" branching. Safe on every path.

// GOTCHA (import ordering — gofmt): `internal/lock` MUST be alphabetical between `internal/generate` and
// `internal/signal`. gofmt enforces this; a misplaced import = a gofmt diff. Let gofmt place it.

// GOTCHA (struct-literal alignment): adding a 3rd field to the Options literal changes gofmt's `:`
// alignment for all three fields. Do NOT hand-align — run `gofmt -w cmd/stagecoach/main.go` and let it align.

// GOTCHA (no docs/tests): the item says "DOCS: none." main.go (package main) has no direct unit tests. The
// signal-seam behavior tests are P1.M2.T3.S2. An inline trailing comment on the OnRescueExit line is the
// only "doc" — optional but recommended (the wiring's purpose is non-obvious from the field name).

// GOTCHA (behavior flip is intentional): S1's no-op default = byte-identical (zero risk). THIS task flips
// it on — that IS the FR52 §18.5 prevention. Do NOT add a flag/config to gate it; it's always on.
```

## Implementation Blueprint

### Data models and structure

No new types. Two line additions to one existing file.

```go
// cmd/stagecoach/main.go — IMPORT BLOCK (add the lock import, alphabetical):
import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/dustin/stagecoach/internal/cmd"
	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/generate"
	"github.com/dustin/stagecoach/internal/lock" // NEW — exit-path lock-release seam (FR52 §18.5)
	"github.com/dustin/stagecoach/internal/signal"
)

// cmd/stagecoach/main.go — THE signal.Install CALL (add OnRescueExit to the Options literal):
	ctx, _ := signal.Install(context.Background(), signal.Options{
		RescueFormat: generate.FormatRescue,
		OnRescueExit: lock.ReleaseCurrent, // NEW — FR52 §18.5: release the lock file before os.Exit orphans it
		Out:          os.Stderr,
	})
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: PRE-CHECK — confirm S1 has landed (STOP guard)
  - RUN: grep -n 'func ReleaseCurrent' internal/lock/lock.go && grep -c 'OnRescueExit' internal/signal/signal.go
  - EXPECT: 1 hit for ReleaseCurrent (after SetSnapshot) AND OnRescueExit count ≥4 (field + doc + Install
    default + 2 handle() calls). If EITHER is missing, S1 is not complete — STOP and flag (do NOT implement
    S1's seam here). Do not proceed to Task 2 until S1 is in the tree.

Task 2: EDIT cmd/stagecoach/main.go — add the import
  - ADD `"github.com/dustin/stagecoach/internal/lock"` to the import block, alphabetical between
    `internal/generate` and `internal/signal`. (gofmt will fix placement if you mis-order it.)
  - GOTCHA: no import cycle (main is root; lock is a stdlib-only leaf).

Task 3: EDIT cmd/stagecoach/main.go — wire OnRescueExit
  - ADD `OnRescueExit: lock.ReleaseCurrent,` to the signal.Options literal. Place it adjacent to
    `RescueFormat:` (both are wired function-value seams); `Out:` stays last.
  - It is a FUNCTION VALUE (`lock.ReleaseCurrent`, no `()` — NOT a closure).
  - An inline trailing comment ("FR52 §18.5: release the lock file before os.Exit orphans it") is recommended.

Task 4: VERIFY (no further edits)
  - RUN: gofmt -w cmd/stagecoach/main.go; go vet ./...; go build ./...; go test -race ./...
  - go build is the S1-landed gate (proves ReleaseCurrent + OnRescueExit exist + signatures match).
  - go.mod/go.sum byte-unchanged. Only cmd/stagecoach/main.go touched. NO tests added; NO docs edited.
```

### Implementation Patterns & Key Details

```go
// THE wiring (the entire "logic" of this task) — a function-value seam, mirroring RescueFormat:
	ctx, _ := signal.Install(context.Background(), signal.Options{
		RescueFormat: generate.FormatRescue,   // existing seam: signal ↔ generate (avoids the cycle)
		OnRescueExit: lock.ReleaseCurrent,     // NEW seam: signal ↔ lock (avoids the cycle); FR52 §18.5
		Out:          os.Stderr,
	})

// WHY a universal wire is safe (ReleaseCurrent mirrors SetSnapshot — nil-safe + idempotent):
//   func ReleaseCurrent() {
//       if l := current.Load(); l != nil { l.Release() }   // current==nil → no-op; Release is idempotent
//   }
//   - read-only subcommand (no Acquire) → current==nil → no-op.
//   - pre-Acquire window → current==nil → no-op.
//   - post-Acquire signal → current!=nil → Release removes the file BEFORE os.Exit skips the defer.
//   - double-call (S1 fires OnRescueExit on both exit branches; only one runs per signal) → idempotent.

// THE seam pattern (signal stays stdlib-only): main.go is the wiring point for EVERY injectable Options
// field (RescueFormat → generate.FormatRescue; OnRescueExit → lock.ReleaseCurrent; Kill/Exit default to
// stdlib). signal NEVER names a stagecoach package → no cycle. This task adds the lock wiring; it does NOT
// touch signal.go (the seam) or lock.go (the ReleaseCurrent func) — both are S1's.
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — internal/lock already exists in-module; main just imports it. `go mod
      tidy` is a no-op. `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES:
  - cmd/stagecoach (package main) → internal/lock : NEW direct import. Safe — main is the package root
        (already imports internal/cmd, internal/generate, internal/signal, internal/exitcode); lock is a
        stdlib-only leaf. NO cycle.
  - internal/signal → (stdlib only) : UNCHANGED. OnRescueExit is a func() field (the seam), NOT an import.
        signal still imports NO stagecoach package.

FROZEN / NOT-EDITED:
  - internal/lock/lock.go (S1's ReleaseCurrent + P1.M2.T1.S2's reapStaleLocks/doc-fixes).
  - internal/signal/signal.go (S1's OnRescueExit field + Install default + 2 handle() calls).
  - internal/cmd/default_action.go (L59 Acquire + L67 defer Release — READ-ONLY context).
  - all test files (P1.M2.T3.S1/S2 own the reaping + signal-seam tests).
  - all docs/*.md (item: "DOCS: none"; lock_reaping.md's doc-fixes are P1.M2.T1.S2).
  - go.mod / go.sum.

DOWNSTREAM CONSUMER (do NOT implement here):
  - P1.M2.T3.S2 ("Exit-path release signal tests"): injects a recording OnRescueExit, fakes the lock,
        asserts OnRescueExit fires before the recording Exit on BOTH branches, and (when wired to
        lock.ReleaseCurrent) that the lock file is removed. THIS task's wiring is what makes the
        lock.ReleaseCurrent variant of those tests meaningful; the tests themselves are S3's.

NO DATABASE / ROUTES / CONFIG / CLI / NEW FILES / COMMITTED TESTS / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w cmd/stagecoach/main.go
test -z "$(gofmt -l cmd/stagecoach/main.go)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./...                       # catches a bad import / wrong field type.
go build ./...                     # THE S1-landed gate: proves ReleaseCurrent + OnRescueExit exist + match.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm the wiring landed:
grep -n 'internal/lock' cmd/stagecoach/main.go                       # 1 hit (the import)
grep -n 'OnRescueExit: lock.ReleaseCurrent' cmd/stagecoach/main.go   # 1 hit (the wiring)
# Confirm only main.go changed:
git diff --name-only | grep -v '^cmd/stagecoach/main\.go$' && echo "UNEXPECTED file changed" || echo "only main.go changed (good)"
```

### Level 2: Unit Tests (Component Validation) — no-regression gate

```bash
go test -race ./...
# Expected: green throughout. main.go (package main) has no direct unit tests; the wiring only activates on
# the signal path, which existing unit tests do NOT exercise through main.go's Install (they inject their own
# Options). So NO regression. The committed signal-seam behavior tests are P1.M2.T3.S2 (not this task).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm the S1 seam is wired (the behavior flip is now live):
grep -c 'OnRescueExit: lock.ReleaseCurrent' cmd/stagecoach/main.go   # → 1
# Confirm S1's files + default_action.go + tests + docs are byte-unchanged (this task touches ONLY main.go):
git diff --exit-code -- internal/lock internal/signal internal/cmd/default_action.go docs/ && echo "S1 + default_action + docs UNCHANGED (expected)"
# (The end-to-end "Ctrl-C removes the lock file" behavior is asserted by P1.M2.T3.S2's committed tests, which
#  inject OnRescueExit=lock.ReleaseCurrent + fake the lock + call handle() on both branches. Not re-proven here.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# THE lint gate — the new import is used (lock.ReleaseCurrent is referenced); no unused-import / U1000:
make lint 2>&1 | grep -iE 'main\.go|lock|unused|U1000' && echo "BAD: wiring flagged" || echo "wiring not flagged (good)"
# Cross-platform build (main compiles on Windows too; lock_windows.go is the processAlive stub):
GOOS=windows go build ./cmd/stagecoach && echo "windows build OK" || echo "windows build FAILED"
# Seam audit — the wiring is the SOLE OnRescueExit reference outside internal/signal:
grep -rn 'OnRescueExit' --include='*.go' . | grep -v 'internal/signal/'
# Expected: exactly 1 hit — cmd/stagecoach/main.go:OnRescueExit: lock.ReleaseCurrent
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1 clean: `gofmt -l`, `go vet ./...`, `go build ./...` (the S1-landed gate), `go mod tidy` no-op.
- [ ] Level 2 green: `go test -race ./...` (no regression — wiring only activates on the signal path).
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only `cmd/stagecoach/main.go` changed.
- [ ] Level 4: `make lint` green (no unused-import/U1000); `GOOS=windows go build ./cmd/stagecoach` OK.

### Feature Validation
- [ ] `cmd/stagecoach/main.go` imports `"github.com/dustin/stagecoach/internal/lock"` (alphabetical).
- [ ] The `signal.Options` literal contains `OnRescueExit: lock.ReleaseCurrent,` (function value, not closure).
- [ ] S1's seam is now wired ON (the intended FR52 §18.5 behavior flip).

### Code Quality Validation
- [ ] The wiring mirrors the existing injectable-seam pattern (`RescueFormat: generate.FormatRescue`).
- [ ] Direct function-value assignment (no needless closure).
- [ ] No scope creep into S1's files (lock.go/signal.go), default_action.go, tests (P1.M2.T3.S2), docs, go.mod.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment
- [ ] Optional inline comment on the `OnRescueExit` line (FR52 §18.5 purpose). No other docs (item: "DOCS: none").
- [ ] go.mod/go.sum byte-unchanged; no new files; no committed tests.

---

## Anti-Patterns to Avoid

- ❌ **Don't proceed if S1 hasn't landed.** If `go build` errors with `unknown field 'OnRescueExit'` or
  `undefined: lock.ReleaseCurrent`, S1 (the seam) is not in the tree. STOP and flag — do NOT add the
  field/func yourself (S1's scope) and do NOT invent a workaround.
- ❌ **Don't use a closure wrapper.** `OnRescueExit: lock.ReleaseCurrent` is a direct function-value
  assignment (signatures match: `func()` ⇄ `func ReleaseCurrent()`). `func(){ lock.ReleaseCurrent() }`
  compiles but is needless noise.
- ❌ **Don't add a conditional ("only wire if committing").** main.go's Install runs for every invocation,
  incl. read-only subcommands. `ReleaseCurrent` is nil-safe + idempotent → a universal wire is correct and
  safe on every path (no-op when no lock is held).
- ❌ **Don't touch S1's files.** `internal/lock/lock.go` (ReleaseCurrent) + `internal/signal/signal.go`
  (OnRescueExit field/default/calls) are S1's scope. This task only CONSUMES them from main.go.
- ❌ **Don't add a flag/config to gate the wiring.** The behavior flip is the entire point (FR52 §18.5
  prevention). It's always on; S1's no-op default was the zero-risk staging step, not a permanent toggle.
- ❌ **Don't edit `internal/cmd/default_action.go`.** The lock `Acquire` (L59) + `defer Release` (L67) site is
  READ-ONLY context. The wiring lives in main.go (the sole `signal.Install` caller), not here.
- ❌ **Don't add tests or docs.** main.go has no direct unit tests; the signal-seam behavior tests are
  P1.M2.T3.S2; the item says "DOCS: none." An inline comment on the wiring line is the only "doc."
- ❌ **Don't change go.mod/go.sum or add files.** Two lines in one existing file.
- ❌ **Don't skip `go build ./...`.** It is the gate that proves S1 landed AND that the signatures match. A
  clean build is the single most important validation for this task.

---

## Confidence Score

**9.5/10** — a 2-line wiring (import + struct-literal field) to one file, fully specified verbatim by
`lock_reaping.md` Fix 2 + the S1 contract. The build is a hard gate that proves correctness (S1 landed +
signatures match). The only residual risk is the S1 ordering dependency (if S1 isn't merged first, the build
fails — but that's a clear, catchable error with a STOP guard, not a silent failure). The -0.5 reserves for
that sequencing dependency and the (unlikely) chance a `make lint` rule flags the new import ordering (gofmt
already enforces it, so gofmt-clean implies lint-clean).
