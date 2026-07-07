# Research — P1.M2.T2.S1: ReleaseCurrent + OnRescueExit signal seam in handle() (FR52 exit-path release)

> Scope: the PREVENTION half of §18.5's stale-lock-file story (FR52). `flock` auto-releases on process death
> so the *lock* is never stale, but `os.Exit` (called by `signal.handle()` on both rescue and pre-snapshot
> paths) **skips `defer locker.Release()`** in `default_action.go:67`, orphaning the lock *file*. This task
> adds the seam that lets `handle()` remove the lock file immediately before exiting — WITHOUT the stdlib-only
> signal package importing `internal/lock`. The BACKSTOP half (reaping on Acquire) is the parallel P1.M2.T1.S2.

---

## 1. The three edits (verbatim from lock_reaping.md Fix 2)

### (a) `lock.ReleaseCurrent()` — package-level, mirrors `SetSnapshot` (lock.go)

```go
// ReleaseCurrent releases the current lock holder (nil-safe no-op when none is held).
// It is the exit-path seam: signal.handle() cannot import internal/lock (the signal package is
// stdlib-only), so the signal handler calls OnRescueExit (defaulted to a no-op; wired to
// lock.ReleaseCurrent in main.go) immediately before os.Exit — removing the lock file that os.Exit's
// defer-skipping would otherwise orphan (FR52 §18.5 "Exit-path release"). Idempotent (Release's
// l.file==nil guard) and nil-safe (current==nil → no-op), exactly mirroring SetSnapshot.
func ReleaseCurrent() {
	if l := current.Load(); l != nil {
		l.Release()
	}
}
```
PLACE immediately after the package-level `SetSnapshot` (its twin — both are `current.Load()` nil-safe
bridges so library layers / the signal seam can act on the singleton without holding a pointer).

### (b) `signal.Options.OnRescueExit func()` — new injectable seam (signal.go)

Add to the `Options` struct (alongside `RescueFormat`/`Out`/`Kill`/`Exit`):
```go
	// OnRescueExit is called immediately before the handler exits on BOTH signal paths (the post-snapshot
	// rescue exit 3 AND the pre-snapshot 130/143 exit). It is the exit-path lock-release seam: wired in
	// main.go to lock.ReleaseCurrent so the lock file is removed before os.Exit skips the deferred Release
	// (FR52 §18.5). Defaulted to a no-op here so the signal package stays stdlib-only (it cannot import
	// internal/lock) and so library use of pkg/stagehand (no Install wiring) is unaffected.
	OnRescueExit func()
```
DEFAULT in `Install()` (after the other `if opts.X == nil` defaults):
```go
	if opts.OnRescueExit == nil {
		opts.OnRescueExit = func() {} // no-op default; main.go wires lock.ReleaseCurrent (FR52 exit-path release)
	}
```

### (c) `handle()` calls `OnRescueExit()` before BOTH exit branches (signal.go)

```go
	if tree != "" {
		fmt.Fprintln(h.opts.Out, h.opts.RescueFormat(tree, parent, cand))
		h.opts.OnRescueExit()           // ← ADD: release the lock file before os.Exit orphans it
		h.opts.Exit(3)                  // §18.2: post-snapshot → exit 3
		return
	}
	h.opts.OnRescueExit()               // ← ADD: pre-snapshot exit too (lock is held from default_action.go:59)
	h.opts.Exit(exitCodeForSignal(sig)) // 130 SIGINT / 143 SIGTERM
```

---

## 2. WHY both exit branches need it (the lock is acquired pre-snapshot)

`default_action.go:59`: `locker, lockErr := lock.Acquire(repoDir)` → `defer locker.Release()` at :67. The
lock is acquired BEFORE the snapshot is armed (the snapshot is armed deep in `CommitStaged`/`runPipeline`
via `signal.SetSnapshot`). So:
- **Pre-snapshot Ctrl-C** → `handle()`'s `tree==""` branch → `Exit(130/143)`. The lock IS held (acquired at
  :59) but `os.Exit` skips the `defer Release` at :67 → the file is orphaned. OnRescueExit fixes this.
- **Post-snapshot Ctrl-C** → `handle()`'s `tree!=""` branch → rescue print + `Exit(3)`. Same orphaning.
- **Success path** → `RestoreDefault()` runs before `update-ref`; the function returns normally → `defer
  locker.Release()` at :67 runs → file removed. OnRescueExit is NOT needed here (and is NOT added to
  RestoreDefault — see §4).

So BOTH exit branches can orphan; BOTH need OnRescueExit. This is the lock_reaping.md Fix 2 rationale:
"Both branches need coverage: the lock is acquired at default_action.go:59 BEFORE the snapshot is armed."

---

## 3. The injectable-seam pattern (signal stays stdlib-only)

`internal/signal` is **deliberately stdlib-only** — its package doc: *"This package imports NO stagehand
packages (stdlib-only). The rescue message reaches the handler via the Options.RescueFormat callback (wired
in main.go), avoiding a signal↔generate import cycle."* `OnRescueExit` is the SAME pattern: a `func()` field
on `Options`, defaulted to a no-op in `Install`, **wired in `main.go`** (by P1.M2.T2.S2, NOT this task) to
`lock.ReleaseCurrent`. The signal package never names `lock` → no import → no cycle. This is the established
discipline: `RescueFormat` (→ generate), `Kill` (→ process group), `Exit` (→ os.Exit), and now `OnRescueExit`
(→ lock) are ALL injectable seams.

### S1 = the SEAM; S2 (P1.M2.T2.S2) = the WIRING
After S1 lands, `OnRescueExit` defaults to `func(){}` → **behavior is byte-identical to today** (the signal
handler calls a no-op before exiting; the lock file is still orphaned on signal-rescue). The actual lock
removal begins only when S2 wires `OnRescueExit: lock.ReleaseCurrent` in main.go's `Install` call. S1 ships
the seam + the lock-side `ReleaseCurrent`; S2 connects them. (This split mirrors how `RescueFormat` was
landed: the seam in signal, the wiring in main.) **S1 does NOT touch main.go.**

---

## 4. RestoreDefault is EXCLUDED (success path uses defer Release)

The contract: "Do NOT add OnRescueExit to RestoreDefault (that runs before update-ref on the success path;
the defer Release handles cleanup there)." `RestoreDefault()` restores the default signal disposition
before `update-ref` so a last-instant Ctrl-C isn't misreported as a failure (§18.4 step 3) — it does NOT
call `os.Exit`. The function returns normally, so `defer locker.Release()` at default_action.go:67 runs and
removes the file. Adding OnRescueExit there would be a redundant (double) release — harmless (Release is
idempotent) but pointless and confusing. OnRescueExit is ONLY for the two os.Exit paths in `handle()`.

---

## 5. Co-edit with the parallel P1.M2.T1.S2 (non-overlapping lock.go additions)

P1.M2.T1.S2 (reapStaleLocks, in flight) edits `internal/lock/lock.go`:
- adds `reapStaleLocks(dir)` (near `parseContents` / after `Acquire`).
- wires `reapStaleLocks(filepath.Dir(path))` into `Acquire` (after `current.Store(l)`).
- fixes 3 over-claim doc comments (lines 2/31/67).

S1 (THIS task) edits the SAME file (`lock.go`) but adds a DIFFERENT function:
- `ReleaseCurrent()` — placed immediately after the package-level `SetSnapshot` (its twin).

The two additions do NOT overlap textually (`reapStaleLocks` is an unexported helper near Acquire/parseContents;
`ReleaseCurrent` is an exported package-level function next to `SetSnapshot`). Both are additive. If both
tasks land together, `git` merges them cleanly (different regions of the file). S2's doc fixes (lines 2/31/67)
are separate from S1's ReleaseCurrent doc comment. **No conflict.** (S1 does NOT touch docs/how-it-works.md —
S2 owns the how-it-works.md reaping rewrites, which already mention "the signal path releases the file before
exiting" per S2's PRP.)

---

## 6. lock_reaping.md Fix 2 — the authoritative spec (verbatim)

```
### lock.ReleaseCurrent() — package-level (mirrors SetSnapshot)
func ReleaseCurrent() {
    if l := current.Load(); l != nil { l.Release() }
}
Idempotent + nil-safe. Called from signal.handle() via the injected seam.

### signal.Options.OnRescueExit func() — new seam
Added to Options alongside existing Kill, Exit, RescueFormat, Out. Defaulted to func(){} (no-op) in Install
— preserves stdlib-only leaf purity.

Called in handle() immediately before BOTH exit branches:
- Before h.opts.Exit(3) (post-snapshot rescue)
- Before h.opts.Exit(exitCodeForSignal(sig)) (pre-snapshot 130/143)
```
This is the contract; the implementation copies it verbatim.

---

## 7. Tests — S1 ships NO committed tests (P1.M2.T3.S2 owns the signal tests)

Mirroring the sibling P1.M2.T1.S2's discipline ("S2 ships NO committed tests — P1.M2.T3.S1 owns the reaping
tests"), S1 ships the SEAM only:
- `ReleaseCurrent` is EXPORTED → no `unused`/U1000 lint (it has a caller once S2 wires it; until then it's
  an exported API symbol, which `unused` does not flag).
- `OnRescueExit` is a struct field (not a function) → no unused-func lint; defaulted + called in `handle()`.
- The `handle()` calls are exercised by P1.M2.T3.S2's "Exit-path release signal tests (signal_test.go)"
  (inject a recording OnRescueExit; assert it's called before the recording Exit on BOTH branches).
- `ReleaseCurrent`'s nil-safety + idempotency is exercised IMPLICITLY by the existing `Release` tests
  (ReleaseCurrent is a one-line `current.Load()` → `Release()` wrapper) + P1.M2.T3.S2's wired-seam test.

A THROWAWAY (non-committed) sanity check (Level 3) confirms: inject `OnRescueExit = lock.ReleaseCurrent`,
Acquire a lock, call `handle(SIGINT)`, assert the lock file is gone (proving the seam end-to-end) — without
adding a committed test file (P1.M2.T3.S2's scope).

---

## 8. Scope fences (NOT this task)

- **NOT main.go wiring** (P1.M2.T2.S2): `OnRescueExit: lock.ReleaseCurrent` in the `Install` call. S1 ships
  the no-op default; S2 connects it. S1 does NOT touch main.go.
- **NOT the reaping backstop** (P1.M2.T1.S2): `reapStaleLocks` + Acquire wiring + how-it-works.md. S1 is the
  PREVENTION (exit-path); S2 is the BACKSTOP (reap-on-Acquire). Both ship in the same milestone.
- **NOT RestoreDefault**: the success path's `defer Release` handles cleanup; OnRescueExit is exit-only.
- **NOT committed tests** (P1.M2.T3.S2): the signal-seam tests. S1 = seam only.
- **NOT the lock.go doc over-claim fixes** (P1.M2.T1.S2's lines 2/31/67). S1 adds ONLY the ReleaseCurrent doc.
- **NOT signal_test.go / lock_test.go**: existing tests stay green (the no-op default = byte-identical behavior).

---

## 9. Validation commands

```bash
gofmt -w internal/lock/lock.go internal/signal/signal.go
go vet ./internal/lock/ ./internal/signal/
go build ./...
go test -race ./...          # the no-op default = byte-identical behavior → NO regression.
git diff --exit-code go.mod go.sum
# Confirm ReleaseCurrent + OnRescueExit landed:
grep -n 'func ReleaseCurrent' internal/lock/lock.go
grep -n 'OnRescueExit' internal/signal/signal.go        # struct field + Install default + 2 handle() calls (≥4 hits)
# Confirm signal still imports NO stagehand package (stdlib-only):
grep -n 'dustin/stagehand' internal/signal/signal.go && echo "BAD: signal imports a stagehand pkg" || echo "signal stdlib-only (good)"
# Confirm OnRescueExit is NOT in RestoreDefault (success path uses defer Release):
grep -n 'OnRescueExit' internal/signal/signal.go | grep -i restore && echo "BAD: OnRescueExit in RestoreDefault" || echo "RestoreDefault clean (good)"
```
