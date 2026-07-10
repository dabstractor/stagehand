# P1.M2.T2.S1 Research Findings — Watchdog package (prctl + getppid poll, Windows no-op)

Source: codebase reading + `architecture/watchdog_config.md`, `architecture/external_deps.md`,
`architecture/critical_findings.md`, the P1.M1.T2.S1 / P1.M2.T1.S1 PRPs, Go stdlib sources.

## 0. Contract recap (the public + file surface)

- PUBLIC API (consumed by P1.M2.T2.S2): `watchdog.Arm(ctx context.Context, interval time.Duration)`
  + `watchdog.Stop()`. Package `internal/watchdog`. Imports `internal/signal` (one-directional).
- FILE LAYOUT (build tags per the item description):
  - `watchdog.go` — `Arm`/`Stop` orchestration (records ppid, starts poll lifecycle, Linux prctl best-effort via armImpl).
  - `arm_unix.go` (`//go:build !windows`) — `armImpl(originalPpid int, interval time.Duration, notify chan struct{})`: getppid poll + armPdeathsig (Linux best-effort via `runtime.GOOS`).
  - `arm_windows.go` (`//go:build windows`) — `armImpl` no-op (FR-K7).
  - `pdeathsig_linux.go` (`//go:build linux`) — `armPdeathsig(sig syscall.Signal) error` via LockOSThread + SYS_PRCTL.
  - `pdeathsig_nonlinux.go` (`//go:build !linux`) — `armPdeathsig` no-op stub.
- DETECTION = parent-pid CHANGE (`getppid() != originalPpid`), NOT `getppid()==1` (wrong under subreapers: systemd-run/docker/supervisord). PRD §9.27 FR-K2.

## 1. Verified Go stdlib facts (critical for the prctl file)

- `syscall.SYS_PRCTL` EXISTS on Linux (Go's own `src/syscall/exec_linux.go:359,533,550` uses it).
- `syscall.PR_SET_PDEATHSIG = 0x1` IS EXPORTED on every Linux arch (`zerrors_linux_amd64.go:709`, arm64/arm/mips/…). So the
  implementer MAY use the named constant `syscall.PR_SET_PDEATHSIG` (== 1) for readability; the contract's literal `1` also works.
- The syscall form (from Go's own exec_linux.go:550): `RawSyscall6(SYS_PRCTL, PR_SET_PDEATHSIG, uintptr(sig), 0,0,0,0)`.
  `sig` is a VALUE (not a pointer). For us: `syscall.Syscall6(syscall.SYS_PRCTL, uintptr(PR_SET_PDEATHSIG), uintptr(sig), 0,0,0,0)`.
- `syscall.AllThreadsSyscall(SYS_PRCTL, …)` ALSO exists (`src/syscall/syscall_linux.go:1123`) — the arguably-more-correct way
  to set a per-thread flag on ALL threads. NOT used here: the CONTRACT specifies the `LockOSThread` + `Syscall6` approach.

## 2. The LockOSThread gotcha (why the poll MUST always run)

`prctl(PR_SET_PDEATHSIG)` is PER-THREAD. The contract uses `runtime.LockOSThread()` + `defer runtime.UnlockOSThread()`.
**Gotcha**: after `UnlockOSThread`, the runtime MAY retire that thread → the prctl setting is LOST (no signal delivered on
parent death). Therefore prctl is genuinely BEST-EFFORT / time-limited. **The polling goroutine ALWAYS runs (even on Linux)
as the reliable detector** — it covers (a) the fork→prctl race window, (b) thread retirement, (c) non-Linux Unix. This is the
"combined best-practice pattern" the contract names. Document this prominently: prctl = latency optimization (kernel SIGTERM
with no 1s poll delay when it works); getppid poll = the trustworthy primary.

## 3. The signal-package APIs consumed (exact signatures, from internal/signal/signal.go)

```go
func Active() *Handler          // nil when Install never called (library use) — Trigger no-ops then
func Trigger(sig os.Signal)     // programmatic rescue entry: if active != nil { h.handle(sig) }
// handle() is stopped-guarded (no-op after RestoreDefault) and calls OnRescueExit (=lock.ReleaseCurrent) on BOTH branches.
```
- `Trigger(syscall.SIGTERM)` → `handle()` → forward to child group → cancel ctx → rescue (exit 3, snapshot armed) OR plain
  exit 143 (SIGTERM pre-snapshot). `OnRescueExit` releases the lock before exit on both branches (FR52 §18.5). FREE for us.
- The watchdog passes `syscall.SIGTERM` (an `os.Signal`). `signal` package imports nothing from us (one-directional; no cycle).

## 4. Leak-free poll design (honoring `armImpl(originalPpid, interval, notify)`)

`notify chan struct{}` = the watchdog lifecycle channel. Closed for ANY reason (detection OR cancel), idempotent via `sync.Once`.
- `watchdog.go Arm`: records `originalPpid := osGetppid()`, makes `armCtx, cancel := context.WithCancel(ctx)` (stored for Stop),
  creates `notify` + `*sync.Once`, calls `armImpl(originalPpid, interval, notify, once)`, runs ONE orchestrator goroutine:
  `<-armCtx.Done() → once.Do(close(notify))` (stops the poll cleanly on process exit / Stop).
- `arm_unix.go armImpl`: best-effort `armPdeathsig(SIGTERM)` (Linux only), then a poll goroutine:
  `for { select { case <-notify: return (stopped); case <-ticker.C: if osGetppid()!=originalPpid { signal.Trigger(SIGTERM); once.Do(close(notify)); return } } }`.
  So detection calls Trigger DIRECTLY (matches contract: "the polling goroutine … calls signal.Trigger on change").
- `arm_windows.go armImpl`: no-op (no poll goroutine). The orchestrator's ctx-cancel still closes notify harmlessly.
- This is leak-free + test-poison-free: every goroutine exits on detection OR ctx cancel.

## 5. The osGetppid test seam

`watchdog.go` declares `var osGetppid = os.Getppid` (package-level, default real). Tests swap it to simulate parent death
(return one ppid at Arm time, a different ppid during the poll). Keeps the PUBLIC `Arm(ctx, interval)` signature clean while
making the detection path unit-testable (you can't reparent a test process for real).

## 6. Consumer wiring (P1.M2.T2.S2 — NOT this task; for the contract note)

`internal/cmd/default_action.go`, after `defer locker.Release()` (~line 79):
`if !cfg.NoParentWatchdog { watchdog.Arm(ctx, 1*time.Second) }` where `ctx := cmd.Context()` (the signal-aware ctx from
main.go's signal.Install → dies when the process exits). Gated by `cfg.NoParentWatchdog` (FR-K6, lands in P1.M2.T1.S1).

## 7. Build-tag / unused-func gotcha (the windows armPdeathsig edge)

With the contract's literal tags, on a WINDOWS build: `arm_windows.go` (no-op) never calls `armPdeathsig`, but
`pdeathsig_nonlinux.go` (`!linux`, which includes windows) DEFINES it → staticcheck U1000 (unused unexported func) on a
native-windows lint. NOTE: (a) `GOOS=windows go build` still PASSES (build ≠ lint; unused funcs are a lint, not compile,
error); (b) `make lint` runs NATIVELY on this linux dev box where arm_unix.go references armPdeathsig → used → clean.
So the contract's tags pass every gate in the validation loop here. ROBUST FIX if CI ever cross-lints windows: make
`pdeathsig_nonlinux.go` `//go:build !linux && !windows` (then windows defines no armPdeathsig and arm_windows.go never
references it). Flagged as an optional refinement in the PRP.

## 8. Validation commands (verified from Makefile)

- `make test` = `go test -race ./...` (race detector). Packages isolate in SEPARATE binaries → the signal `active` singleton
  is fresh per package → no cross-package test poisoning.
- `make lint` = golangci-lint (staticcheck/gosimple/govvet/errcheck/ineffassign/unused). Runs native (linux here).
- `make coverage-gate` = ≥85% on internal/{git,provider,generate,config} ONLY — internal/watchdog is NOT in the gate set.
- `go build ./...` ; `GOOS=windows go build ./...` ; `GOOS=linux go build ./...` ; `gofmt -l internal/watchdog/*.go`.

## 9. Test plan (clone the signal-package injectable-Exit idiom)

- `watchdog_test.go` (cross-platform): `TestArm_NegativeIntervalDefaults`, `TestArm_StopWithoutArmIsNoOp`,
  `TestArm_StopCancelsGoroutine` (Arm a canceled ctx → no leak/hang), `TestArm_NilSignalHandlerIsNoOp`
  (Arm with no signal.Install → poll fires Trigger no-op → process does not exit → ctx cancel cleans up).
- `watchdog_unix_test.go` (`!windows`): `TestArm_FiresTriggerOnParentDeath` — install a real signal handler via
  `signal.Install(ctx, Options{Exit: recorder, Out: buf})`; set `osGetppid = orig`; `watchdog.Arm(ctx, 5*time.Millisecond)`;
  swap `osGetppid = changed`; poll for the recorder exit code == 143 (SIGTERM pre-snapshot) within 2s; RestoreDefault in
  Cleanup. Mirrors signal_test.go's `installTestHandler` + fake-Exit assertion idiom.
