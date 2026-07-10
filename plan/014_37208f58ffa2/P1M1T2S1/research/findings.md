# Research: P1.M1.T2.S1 — Exported `signal.Trigger(sig)` wrapper

**Scope**: Add one exported function `func Trigger(sig os.Signal)` to internal/signal/signal.go +
godoc + unit tests. All line numbers verified against the working tree (2026-07-09).

## 1. The deliverable (1 function, ~3 lines)

```go
func Trigger(sig os.Signal) {
    if h := active.Load(); h != nil {
        h.handle(sig)
    }
}
```

- GREENFIELD: `grep -rn 'func Trigger' internal/ cmd/ pkg/` → EMPTY. No collision.
- It is the **exact same idiom** as every nil-safe package wrapper in this file
  (`RegisterChild`, `ClearChild`, `SetSnapshot`, `SetCandidate`, `ClearSnapshot`, `RestoreDefault`):
  `if h := active.Load(); h != nil { h.<something> }`. The only difference: it calls the (unexported)
  `handle(sig)` method — the rescue/exit routine — instead of a setter.

## 2. Why this is enough (no handle/RestoreDefault changes)

- `handle(sig)` (signal.go:134) is **signal-agnostic and already the single rescue/exit routine**:
  stopped-guard → forward-to-child-group → cancel ctx → rescue-or-exit (with OnRescueExit on both branches).
  `Trigger` reuses it verbatim.
- The **stopped guard** is the FIRST line of `handle()`: `if h.stopped.Load() { return }`. So a `Trigger`
  call after `RestoreDefault()` (which sets `stopped=true`) is a no-op FOR FREE. This is the
  "update-ref window" guarantee the item requires — zero extra code.
- Nil-safe FOR FREE: `active.Load()==nil` (library use, no Install) → the `if h != nil` guard skips → no-op.
  Mirrors `TestHandler_NilWrappersNoOp` (signal_test.go:150).

## 3. Exact current anchors in signal.go

| Symbol | Line | Relevant to |
|---|---|---|
| `var active atomic.Pointer[Handler]` | 82 | the singleton Trigger reads |
| `func Active() *Handler` | 119 | exported singleton access (already exists; Trigger's natural neighbor) |
| `func (h *Handler) handle(sig os.Signal)` | 134 | the routine Trigger delegates to (signal-agnostic; no change) |
| `// ---- Nil-safe package wrappers ----` | 166 | the idiom group Trigger joins |
| `func RestoreDefault()` | 218 | sets stopped=true (Trigger's no-op-after-Restore guarantee source) |

**Placement**: add `Trigger` **immediately after `Active()` (line 119)**, before `run()`. Rationale: both
are exported entry points to the `active` singleton — `Active()` exposes the handler; `Trigger()` drives it
programmatically. (Equally valid: top of the nil-safe-wrappers section at line 166. Either is fine — the
function is purely additive; pick after-Active for the read order.)

## 4. COORDINATION WITH PARALLEL TASK P1.M1.T1.S1 (CRITICAL)

P1.M1.T1.S1 is being implemented in parallel and ALSO edits signal.go. Its contract (from its PRP):
- Swaps `signal.Notify(h.ch, os.Interrupt, syscall.SIGTERM)` (line 103) → `signal.Notify(h.ch, caughtSignals()...)`.
- **REMOVES the `"syscall"` import from signal.go** (line 103 was its only user).
- Refreshes doc comments. Does NOT touch `active`, `Active()`, `handle()`, or the nil-safe wrappers.

**Why there is NO conflict for T2.S1**: `Trigger` takes `os.Signal` and references only `active`/`handle` —
it does **NOT** import or reference `syscall`. So after T1.S1 removes the syscall import, `Trigger` still
compiles cleanly. The two changes touch non-overlapping regions of signal.go (T1.S1 = import block + line 103
+ comments; T2.S1 = a new function near line 119). The implementer does NOT need syscall for Trigger.

(If for some reason T1.S1 has NOT landed when T2.S1 implements, that's also fine — Trigger is independent of
the caught-signals work. T1.S1 and T2.S1 are decoupled; T2.S1 only needs `active`/`handle`, which pre-date T1.S1.)

## 5. Test patterns to mirror (internal/signal/signal_test.go — UN-TAGGED, cross-platform)

| Existing test | Line | What it proves | Mirror for Trigger |
|---|---|---|---|
| `installTestHandler(t, opts)` helper | 13 | stores handler in `active`, `t.Cleanup` resets to nil | reuse as-is |
| `TestHandler_ForwardsToChildGroup` | 33 | `h.handle(SIGINT)` → Kill(pid,sig) + exit 130 | `Trigger(SIGTERM)` variant |
| `TestHandler_RescueOnSignalWithSnapshot` | 64 | snapshot + handle → exit 3 + rescue print | `Trigger` + snapshot |
| `TestHandler_Exit143SIGTERM` | 110 | handle(SIGTERM) → 143 | `Trigger(SIGTERM)` → 143 |
| `TestHandler_RestoreDefaultStopsForward` | 126 | after RestoreDefault, handle is no-op | **the stopped-guard test for Trigger** |
| `TestHandler_NilWrappersNoOp` | 150 | active==nil → wrappers no-op, no panic | **the nil-safe test for Trigger** |

**KEY distinction from the existing handle-tests**: those call `h.handle(sig)` (the method). Trigger tests
must call the **package-level** `Trigger(sig)` (the export) — that's what proves the singleton-lookup +
delegation path the watchdog will use. The handler is installed via `installTestHandler` (which sets
`active`), then `Trigger(...)` finds it through `active.Load()`.

**Windows-safety (must hold)**: signal_test.go is UN-TAGGED (compiles on Windows). Existing tests use
`syscall.SIGINT`/`syscall.SIGTERM` and `os.Interrupt` only — all exist on Windows. Trigger tests MUST use
the SAME cross-platform signals (`syscall.SIGTERM`, `os.Interrupt`). Do **NOT** use `syscall.SIGHUP`
(that's T1.S1's domain and does not exist on Windows → would break `GOOS=windows go test`).

## 6. Test cases to implement (all go in the un-tagged signal_test.go)

1. `TestTrigger_RoutesThroughHandle_SIGTERM` — install (no snapshot); `Trigger(syscall.SIGTERM)`; assert exitCode==143.
2. `TestTrigger_RoutesThroughHandle_SIGINT` — `Trigger(os.Interrupt)`; assert exitCode==130 (signal-agnostic proof).
3. `TestTrigger_RescueOnSnapshot` — `SetSnapshot(...)`; `Trigger(syscall.SIGTERM)`; assert exitCode==3 + rescue printed + OnRescueExit fired (the lock-release seam fires through Trigger too).
4. `TestTrigger_ForwardsToChild` — `RegisterChild(1234)`; `Trigger(syscall.SIGTERM)`; assert Kill called with (1234, SIGTERM).
5. `TestTrigger_NoOpAfterRestoreDefault` — `RestoreDefault()`; `Trigger(syscall.SIGTERM)`; assert Exit NOT called, Kill NOT called (the stopped-guard — the item's key guarantee).
6. `TestTrigger_NilSafeNoHandler` — `active.Store(nil)`; `Trigger(syscall.SIGTERM)`; no panic (library use).

(Cases 1+2 can be a single table-driven test; 3/4/5/6 discrete — match the file's mixed style.)

## 7. Downstream consumer (NOT built here — just the contract)

P1.M2.T2.S1 (the parent-death watchdog) will call `signal.Trigger(syscall.SIGTERM)` when it detects parent
death via getppid() polling (the `prctl(PR_SET_PDEATHSIG)` Linux fast path delivers a REAL SIGTERM that
flows through `run`→`handle` naturally and needs no Trigger call). So `Trigger` is the fallback-path entry
point. The architecture note (`signal_extension.md` §"FR-K1 extension") confirms `signal.Trigger(syscall.SIGTERM)`
is the exact call and that it respects the stopped guard + fires OnRescueExit (lock release).

## 8. Validation commands (project conventions)

- `go build ./...` — native.
- `GOOS=windows go build ./...` + `GOOS=linux go build ./...` — cross-compile (Trigger is cross-platform; must stay clean).
- `go vet ./internal/signal/...` and `GOOS=windows go vet ./internal/signal/...`.
- `gofmt -l internal/signal/signal.go internal/signal/signal_test.go` → must list nothing.
- `go test ./internal/signal/... -run Trigger -v`.
- `make test` + `make lint`.
- Grep guard: `grep -rn 'func Trigger' internal/signal/` → exactly one hit (signal.go); `grep -rn 'signal\.Trigger\|Trigger(' internal/signal/signal_test.go` → the new tests only (no production caller yet — the watchdog is P1.M2).
