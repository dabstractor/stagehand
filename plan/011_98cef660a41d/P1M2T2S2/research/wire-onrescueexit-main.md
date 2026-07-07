# Research: Wire OnRescueExit → lock.ReleaseCurrent in main.go (P1.M2.T2.S2)

> THE WIRING half of §18.5's exit-path lock release (FR52). S1 (P1.M2.T2.S1, parallel) ships the SEAM:
> `lock.ReleaseCurrent()` (nil-safe wrapper twin of `SetSnapshot`) + `signal.Options.OnRescueExit func()`
> (injectable seam, no-op default). **This task (S2) is the one-line WIRING**: set
> `OnRescueExit: lock.ReleaseCurrent` in main.go's `signal.Install` call so the signal handler removes the
> lock file before `os.Exit` orphans it.
>
> Sources: `architecture/lock_reaping.md` Fix 2 (the wiring spec), the S1 PRP (the seam contract), the live
> `cmd/stagecoach/main.go`, PRD §18.4/§18.5.

---

## §1 — The exact change (2 line additions to ONE file)

`cmd/stagecoach/main.go` is the sole `signal.Install` caller (verified: `grep -rn signal.Install` → 1 hit).
Two edits:

### Edit A — add the import (alphabetical, between `internal/generate` and `internal/signal`)

```go
import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/dustin/stagecoach/internal/cmd"
	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/generate"
	"github.com/dustin/stagecoach/internal/lock"   // NEW — exit-path lock-release seam (FR52 §18.5)
	"github.com/dustin/stagecoach/internal/signal"
)
```

`gofmt` enforces alphabetical order within the group; `lock` lands between `generate` and `signal`. No import
cycle: `main` is the package root (it already imports `cmd`/`generate`/`signal`); `internal/lock` is a
stdlib-only leaf. `lock_reaping.md`: "main already imports signal and generate; adding lock creates no cycle."

### Edit B — add `OnRescueExit` to the `signal.Options` literal

```go
	ctx, _ := signal.Install(context.Background(), signal.Options{
		RescueFormat: generate.FormatRescue,
		OnRescueExit: lock.ReleaseCurrent, // NEW — FR52 §18.5: release the lock file before os.Exit orphans it
		Out:          os.Stderr,
	})
```

Place `OnRescueExit` adjacent to `RescueFormat` (both are wired function-value seams: `generate.FormatRescue`
and `lock.ReleaseCurrent`); `Out` is the `io.Writer` (different kind). `gofmt` re-aligns the `:` column for
the three fields — do not hand-align. The trailing comment is optional but recommended (the wiring's purpose
is non-obvious from `OnRescueExit` alone); the existing `RescueFormat`/`Out` lines have no comment, so matching
that style exactly is also acceptable.

**Signature match**: S1 defines `OnRescueExit func()` and `func ReleaseCurrent()` — so `OnRescueExit:
lock.ReleaseCurrent` is a direct function-value assignment (NOT a call). No `()`, no wrapper closure needed.

---

## §2 — THE hard dependency: S1 must land FIRST (STOP-and-flag guard)

The current repo is MID-S1: `grep 'func ReleaseCurrent' internal/lock/lock.go` → no hit; `grep 'OnRescueExit'
internal/signal/signal.go` → no hit. **If S1 has not landed, this task's edit does not compile:**

- `OnRescueExit: lock.ReleaseCurrent` → **`unknown field 'OnRescueExit' in struct literal of type signal.Options`**
  (field doesn't exist yet) AND **`undefined: lock.ReleaseCurrent`** (func doesn't exist yet).

`go build ./cmd/stagecoach` is the gate. **If those errors appear, S1 is not complete — STOP and flag; do NOT
invent a workaround (e.g. do NOT add the field/func yourself — that is S1's scope; do NOT wire a closure that
imports lock into signal — that breaks signal's stdlib-only purity).** The orchestrator runs S1 to completion,
then S2; this task assumes S1 is in the tree.

---

## §3 — Why the wiring is universal + safe (no conditional needed)

main.go's `signal.Install` runs for EVERY CLI invocation — including read-only subcommands (`providers`,
`config`, `--version`) that **bypass `lock.Acquire`** (§18.5: "Read-only subcommands bypass it — they never
mutate refs"). So on a read-only subcommand, no lock is held → `current.Load() == nil` → `ReleaseCurrent` is a
nil-safe **no-op**. The wiring therefore needs NO conditional ("only wire if committing") — `ReleaseCurrent`'s
nil-safety (mirroring `SetSnapshot`: `if l := current.Load(); l != nil { l.Release() }`) makes a universal wire
correct. It is also **idempotent** (`Release`'s `l.file==nil` guard handles a double-call), so firing on both
signal-exit branches (S1's two `OnRescueExit()` calls in `handle()`) is safe.

The behavior change is INTENTIONAL and is the entire point: S1 shipped a no-op default (byte-identical, zero
risk); S2 flips it on. After S2, a Ctrl-C/SIGTERM after `lock.Acquire` (default_action.go:59) removes the lock
file via `OnRescueExit → ReleaseCurrent` before `os.Exit` skips `defer locker.Release()` (default_action.go:67).
This is the §18.5 "Exit-path release (prevention)" — reaping (P1.M2.T1.S2) becomes the backstop for
SIGKILL/crash, not the hot path.

---

## §4 — What NOT to touch (scope fences)

- **`internal/lock/lock.go`** — S1 adds `ReleaseCurrent`; P1.M2.T1.S2 adds `reapStaleLocks` + the 3 doc-fixes.
  This task adds NOTHING to lock.go (it only CONSUMES `lock.ReleaseCurrent`).
- **`internal/signal/signal.go`** — S1 adds the `OnRescueExit` field + default + the two `handle()` calls. This
  task adds NOTHING to signal.go (it only SETS the field from main.go).
- **`internal/cmd/default_action.go`** — the lock `Acquire` (L59) + `defer Release` (L67) site is READ-ONLY
  context; the wiring lives in main.go, not here.
- **`docs/how-it-works.md`** / any `docs/*.md` — the item says "DOCS: none — no user-facing surface change."
  `lock_reaping.md`'s "Doc-Comment Corrections" are P1.M2.T1.S2's scope; the `ReleaseCurrent`/`OnRescueExit`
  doc comments are S1's. S2 adds NO docs (an inline comment on the wiring line is the only "doc").
- **Test files** — main.go (`package main`) has no direct unit tests (it's the binary entrypoint). The
  signal-seam behavior tests are P1.M2.T3.S2 (recording `OnRescueExit`; assert it fires before the recording
  `Exit` on both branches + removes the lock file when wired to `ReleaseCurrent`). This task adds NO tests.
- **go.mod / go.sum** — no new dep (`internal/lock` is already a package in this module; `main` just imports it).

---

## §5 — Validation

- `gofmt -l cmd/stagecoach/main.go` clean (import ordering + struct alignment).
- `go vet ./...` / `go build ./...` clean — AND this is the S1-landed gate (the build proves `ReleaseCurrent`
  + `OnRescueExit` exist with matching signatures).
- `go test -race ./...` green — NO regression. The wiring only activates on the signal path; existing unit
  tests inject their own `Options` (they don't go through main.go's `Install`), so they're unaffected.
- Smoke (Level 3): `go build -o /tmp/stagecoach ./cmd/stagecoach`; confirm `OnRescueExit` is wired
  (`grep 'OnRescueExit: lock.ReleaseCurrent' cmd/stagecoach/main.go`); confirm `internal/lock` is imported.
- The end-to-end behavior (Ctrl-C removes the lock file) is asserted by P1.M2.T3.S2's committed tests, NOT here.
