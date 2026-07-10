name: "P1.M2.T2.S1 — Watchdog package: prctl fast path (Linux) + getppid polling (Unix), Windows no-op (FR-K1/K2/K7)"
description: >
  Create a NEW leaf package `internal/watchdog` (5 build-tagged files + tests) that detects launcher death and tears the
  run down via the existing signal rescue/exit path. PUBLIC API consumed by P1.M2.T2.S2: `watchdog.Arm(ctx, interval)` +
  `watchdog.Stop()`. The package imports `internal/signal` (one-directional — signal never imports back; no cycle) and calls
  `signal.Trigger(syscall.SIGTERM)` on parent death, which routes through the signal handler's `handle()` → forward-to-child-
  group → cancel ctx → rescue(exit 3, if snapshot armed) OR plain exit(143 SIGTERM), with `OnRescueExit` (= lock.ReleaseCurrent)
  releasing the lock before exit on BOTH branches (FR52 §18.5) — FREE for us, no lock-import in this package.
  DETECTION = parent-pid CHANGE (`getppid() != originalPpid` captured at Arm time), NOT `getppid()==1` (WRONG under subreapers
  like systemd-run/docker/supervisord — PRD §9.27 FR-K2). The polling goroutine ALWAYS runs (even on Linux) as the reliable
  detector; prctl(PR_SET_PDEATHSIG) is a Linux-only BEST-EFFORT kernel fast path (it is per-thread; the runtime can retire the
  LockOSThread-pinned thread after UnlockOSThread, so the setting may be lost — the poll covers that race + thread retirement +
  the fork→prctl gap). Windows is a no-op (FR-K7). Nil-safe: if `signal.Active()==nil` (library use of pkg/stagecoach),
  `signal.Trigger` is a no-op and the goroutine exits cleanly on ctx cancel. NO consumer wiring here (P1.M2.T2.S2 adds the
  `if !cfg.NoParentWatchdog { watchdog.Arm(ctx, 1*time.Second) }` call); NO config-field change (P1.M2.T1.S1 owns that);
  NO signal-package change (P1.M1.T2.S1 already exported `Trigger`). Stdlib-only + one stagecoach import (internal/signal).

---

## Goal

**Feature Goal**: Create `internal/watchdog`, a leaf package that arms a parent-death watchdog. When the launching process
(lazygit TUI, an IDE, a detaching terminal — the orphaned-run case of §9.27 / §18.5) dies without sending SIGINT/SIGTERM,
the child stagecoach is reparented to init/subreaper; the watchdog detects the parent-pid CHANGE and tears the run down by
calling `signal.Trigger(syscall.SIGTERM)`, reusing the exact same rescue + lock-release exit path a real SIGTERM takes. This
closes the §18.5 gap where a holder whose launcher "closed without killing it" leaves the lock file orphaned indefinitely.

**Deliverable** (5 build-tagged source files + 2 test files; one new package; zero edits to existing files):
1. **`internal/watchdog/watchdog.go`** — cross-platform: package doc + `Arm(ctx, interval)` + `Stop()` + the `osGetppid`
   test seam + the `notifier`/`once` lifecycle glue + the cancel-watcher goroutine.
2. **`internal/watchdog/arm_unix.go`** (`//go:build !windows`) — `armImpl(originalPpid, interval, notify, once)`: best-effort
   `armPdeathsig(SIGTERM)` on Linux (via `runtime.GOOS=="linux"`) + the getppid polling goroutine that calls
   `signal.Trigger(syscall.SIGTERM)` on a ppid change.
3. **`internal/watchdog/arm_windows.go`** (`//go:build windows`) — `armImpl` no-op (FR-K7).
4. **`internal/watchdog/pdeathsig_linux.go`** (`//go:build linux`) — `armPdeathsig(sig)` via `runtime.LockOSThread()` +
   `syscall.Syscall6(syscall.SYS_PRCTL, uintptr(syscall.PR_SET_PDEATHSIG), uintptr(sig), 0,0,0,0)`.
5. **`internal/watchdog/pdeathsig_nonlinux.go`** (`//go:build !linux`) — `armPdeathsig(sig)` no-op stub.
6. **`internal/watchdog/watchdog_test.go`** — cross-platform tests (Stop, interval default, nil-safety, cancel cleanup).
7. **`internal/watchdog/watchdog_unix_test.go`** (`//go:build !windows`) — the detection-path test (ppid change → Trigger → exit 143).

**Success Definition**:
- `watchdog.Arm(ctx, 1*time.Second)` starts the watchdog; on parent-pid change it calls `signal.Trigger(syscall.SIGTERM)`
  (proven by a test that installs a signal handler with a fake `Exit`, swaps the `osGetppid` seam, and asserts exit code 143).
- `watchdog.Stop()` (or `ctx` cancel / process exit) stops every watchdog goroutine with no leak (proven by the cancel test +
  `-race`). Safe to call when never armed (no-op) and multiple times (idempotent).
- Nil-safe: `Arm` with no `signal.Install` (`signal.Active()==nil`) → `Trigger` is a no-op, process does NOT exit, goroutine
  exits on ctx cancel (proven by `TestArm_NilSignalHandlerIsNoOp`).
- Windows build: `armImpl` is a no-op, no poll goroutine, no prctl (FR-K7) — `GOOS=windows go build ./...` clean.
- `go build ./...`, `GOOS={linux,windows} go build ./...`, `make test` (race), `make lint`, `gofmt -l` all clean.
- Package imports: stdlib + `github.com/dustin/stagecoach/internal/signal` ONLY (no lock, no config, no third-party).
- ZERO production callers of `watchdog.Arm` after this subtask (the consumer is P1.M2.T2.S2; only the new tests call it).

## User Persona (if applicable)

**Target User**: A developer who launches stagecoach from a short-lived or fragile parent — closing the lazygit TUI, quitting
an IDE window, or a detaching terminal — exactly the orphaned-run scenarios in §9.27.

**Use Case**: The launcher exits without sending SIGINT/SIGTERM (the §18.5 bug case). The watchdog detects the resulting
reparent and tears the run down cleanly (rescue message if mid-generation, lock released) instead of orphaning the lock file.

**User Journey**: `stagecoach` (launched by lazygit) → user quits lazygit → kernel reparents stagecoach to init → watchdog's
poll sees the ppid change → `signal.Trigger(SIGTERM)` → rescue-or-exit + `OnRescueExit` removes the lock file → next run is
not blocked by a stale lock. (For intentional detach — `nohup`/`setsid`/`systemd-run` — the user sets
`no_parent_watchdog`/`STAGECOACH_NO_PARENT_WATCHDOG`, P1.M2.T1.S1 + P1.M2.T2.S2, to opt out.)

**Pain Points Addressed**: FR-K1/K2 — the §18.5 "launcher closed without killing it" lock-orphan gap.

## Why

- **FR-K1/K2 / §9.27**: the parent-death watchdog is the detection mechanism for orphaned-run lock reclamation. This subtask
  builds the watchdog PACKAGE itself; its arming + config gate land in P1.M2.T2.S2 (consumer) and P1.M2.T1.S1 (config).
- **Reuses the single rescue path**: by calling `signal.Trigger(SIGTERM)` (the P1.M1.T2.S1 export), the watchdog gets
  forward-to-child-group + cancel ctx + rescue(exit 3) + `OnRescueExit`(lock release) for free — no duplicated teardown, no
  `internal/lock` import in this package (keeping lock stdlib-only).
- **Bounded scope**: a self-contained new leaf package with a tiny public surface (`Arm`/`Stop`) and one directional import.
  No edits to existing files; no new third-party deps (stdlib + internal/signal only).

## What

**User-visible behavior**: None directly (the consumer is a later task). Internally, the package provides the watchdog that
P1.M2.T2.S2 will arm after `lock.Acquire`, sharing the signal-aware `ctx` from main.go so it dies when the process exits.

**Technical change**: 5 new build-tagged source files + 2 test files in a new `internal/watchdog/` package. See the
Implementation Blueprint for verbatim code + build tags + gotchas.

### Success Criteria
- [ ] `internal/watchdog/` package exists with `Arm(ctx, interval)` + `Stop()` exported, godoc'd, citing §9.27 FR-K1/K2/K7.
- [ ] `watchdog.go` declares `var osGetppid = os.Getppid` (test seam) and the `notifier`/`once` lifecycle glue.
- [ ] `arm_unix.go` polls `osGetppid()` and on a change calls `signal.Trigger(syscall.SIGTERM)`; Linux also best-effort `armPdeathsig`.
- [ ] `arm_windows.go` `armImpl` is a no-op (no poll goroutine, no prctl) — FR-K7.
- [ ] `pdeathsig_linux.go` `armPdeathsig` uses `runtime.LockOSThread()` + `syscall.Syscall6(syscall.SYS_PRCTL, …PR_SET_PDEATHSIG…)`.
- [ ] `pdeathsig_nonlinux.go` `armPdeathsig` is a no-op returning nil.
- [ ] Detection is ppid CHANGE (`!= originalPpid`), NEVER `== 1` (subreaper-safe — PRD §9.27 FR-K2).
- [ ] Nil-safe: `Arm` with `signal.Active()==nil` does not exit the process; goroutine exits on ctx cancel.
- [ ] Leak-free: `Stop()`/ctx cancel stops every goroutine (no test hang, `-race` clean).
- [ ] 5-6 tests pass (Stop no-op, Stop cancels, interval default, nil-safety, detection→exit 143, [windows no-op]).
- [ ] `go build ./...` + `GOOS=windows` + `GOOS=linux` clean; `make test`(race) + `make lint` clean; `gofmt -l` empty.
- [ ] Package imports = stdlib + `internal/signal` ONLY. ZERO production `watchdog.Arm` callers (consumer is P1.M2.T2.S2).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — verbatim code for all 5 files + 2 test files (with build tags and imports), the exact `signal.Trigger`/`signal.Active`
signatures consumed, the verified Go-stdlib facts for the prctl syscall (`syscall.SYS_PRCTL` + `syscall.PR_SET_PDEATHSIG=0x1`
both exist on Linux; Go's own exec_linux.go:550 uses this exact pattern), the LockOSThread gotcha (why prctl is best-effort and
the poll ALWAYS runs), the leak-free `notify`+`sync.Once` lifecycle design, the `osGetppid` test seam, the consumer's exact call
site (default_action.go post-Acquire, ctx = cmd.Context()), and the test idiom to clone (signal_test.go's injectable `Exit`).
Build tags, import lists, naming, and the unused-func-on-windows edge case are all spelled out.

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim code + gotchas for every file + the leak-free design)
- docfile: plan/014_37208f58ffa2/P1M2T2S1/research/findings.md
  why: "§0 contract recap; §1 verified stdlib facts (SYS_PRCTL + PR_SET_PDEATHSIG exist); §2 the LockOSThread gotcha
        (why prctl is best-effort and the poll ALWAYS runs); §3 the exact signal.Trigger/Active APIs consumed; §4 the
        leak-free notify+once design; §5 the osGetppid test seam; §7 the windows armPdeathsig unused-func edge; §8-9
        validation commands + the test plan."
  critical: "§2: after LockOSThread+defer UnlockOSThread the runtime MAY retire the pinned thread → prctl setting LOST.
             So prctl is a best-effort latency optimization; the getppid poll is the reliable primary detector and MUST
             always run. Do NOT remove the poll 'because prctl handles Linux'."

# MUST READ — the watchdog design + the combined prctl+poll best-practice pattern
- docfile: plan/014_37208f58ffa2/architecture/watchdog_config.md
  why: "'The parent-death watchdog (FR-K1/K2/K7)' section: the 5-file layout, the Arm function contract, the parent-pid-CHANGE
        detection (NOT getppid()==1), the prctl constraints (per-thread, LockOSThread, race window), and the arming point."
  critical: "Confirms: new internal/watchdog leaf (NOT in lock or signal — both stay stdlib-only); one-directional import of
             internal/signal; prctl is best-effort; poll always runs; the watchdog rides the OnRescueExit seam for lock release."

# MUST READ — the external-deps + verified stdlib facts (the prctl syscall, getppid, build-tag table)
- docfile: plan/014_37208f58ffa2/architecture/external_deps.md
  why: "'Go stdlib APIs used' + 'Platform build tags' sections: the exact Syscall6 form, the LockOSThread requirement, the
        getppid CHANGE detection rationale (subreapers), and the 5-row build-tag table for the watchdog files."
  critical: "PR_SET_PDEATHSIG=1 (hardcoded per the contract; syscall.PR_SET_PDEATHSIG==0x1 is ALSO exported on Linux — either
             works); sig is a VALUE not a pointer; verify os.Getppid()==originalPpid after arming is the poll's job."

# MUST READ — the signal package APIs the watchdog consumes (read these BEFORE writing arm_unix.go)
- file: internal/signal/signal.go
  why: "The exported Trigger(sig) (~line 138) and Active() (~line 110) the watchdog calls; handle() (~line 143) shows the exact
        forward→cancel→rescue/exit path Trigger reuses (stopped-guarded; OnRescueExit on BOTH branches). Confirms Trigger is
        nil-safe (active.Load()==nil → no-op) and stopped-guarded — so calling it after RestoreDefault (the update-ref window)
        is a harmless no-op."
  pattern: "Trigger delegates to the SAME handle() the OS-signal goroutine uses → exactly one rescue path (no duplicated teardown)."
  gotcha: "Do NOT import internal/lock here — the lock release rides signal's OnRescueExit seam (wired in main.go). Keep signal
           one-directional (signal never imports watchdog)."

# MUST READ — the test idiom to clone (injectable Exit/Kill/Out via signal.Install)
- file: internal/signal/signal_test.go
  why: "installTestHandler (~line 14) + TestHandler_Exit143SIGTERM (~line 119) are the template for TestArm_FiresTriggerOnParentDeath:
        install a handler with Options{Exit: func(code int){exitCode=code}, Out: new(bytes.Buffer)}, drive the detection, assert
        exitCode==143. Packages isolate in separate test binaries under `go test ./...`, so the signal `active` singleton is fresh
        per package — no cross-package poisoning."
  pattern: "Fake Exit records the code WITHOUT exiting (so the test process survives); assert the recorded code."
  gotcha: "Call signal.RestoreDefault() in t.Cleanup so the installed handler's run goroutine exits (it ranges on the channel;
           RestoreDefault closes it). Restoring is belt-and-suspenders; within this package each test re-installs anyway."

# CONTEXT — the consumer (LANDS LATER in P1.M2.T2.S2; read to confirm the call site + ctx source)
- file: internal/cmd/default_action.go
  why: "runDefault (~line 37): ctx := cmd.Context() (the signal-aware ctx from main.go's signal.Install). lock.Acquire at ~line 71;
        `defer locker.Release()` at ~line 79. P1.M2.T2.S2 adds `if !cfg.NoParentWatchdog { watchdog.Arm(ctx, 1*time.Second) }`
        immediately after the defer. The watchdog shares that ctx → dies when the process exits."
  critical: "Do NOT add this consumer call now. After this subtask, grep must show ZERO production watchdog.Arm callers."

# CONTEXT — the config gate (lands in P1.M2.T1.S1; parallel sibling, no overlap)
- docfile: plan/014_37208f58ffa2/P1M2T1S1/PRP.md
  why: "Adds cfg.NoParentWatchdog (the FR-K6 opt-out). NO file overlap with this task (that PRP edits internal/config/* only;
        this PRP creates internal/watchdog/* only). The consumer in P1.M2.T2.S2 reads both."

# CONTEXT — main.go ctx + OnRescueExit wiring (the lock-release seam the watchdog rides for free)
- file: cmd/stagecoach/main.go
  why: "signal.Install(context.Background(), Options{OnRescueExit: lock.ReleaseCurrent, …}) (~line 59) → the ctx flows to
        cmd.Execute → cmd.Context() in default_action.go. Confirms the watchdog's ctx is signal-aware and OnRescueExit releases
        the lock before exit on BOTH rescue(3) and plain(143) paths."
```

### Current Codebase tree (relevant slice)

```bash
internal/signal/                 # READ-ONLY dependency — exports Trigger(sig) + Active(); we import it
  signal.go                      # Trigger (~138), Active (~110), handle (~143) — the rescue/exit path we ride
  signal_unix.go / signal_windows.go   # build-tag reference for our own arm_*.go split
internal/cmd/
  default_action.go              # READ-ONLY — consumer site (P1.M2.T2.S2 adds the Arm call post-Acquire)
cmd/stagecoach/main.go           # READ-ONLY — signal.Install + OnRescueExit wiring (ctx source)
go.mod                           # go 1.22; deps: cobra, go-toml/v2, pflag, yaml. NO golang.org/x/sys (keep it that way)
Makefile                         # test = go test -race ./...; lint = golangci-lint; coverage-gate = the 4 core pkgs only
# internal/watchdog/             # DOES NOT EXIST YET — this task creates it (5 files + 2 tests)
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
internal/watchdog/               # NEW leaf package (PRD §9.27 FR-K1/K2/K7). Imports: stdlib + internal/signal ONLY.
  watchdog.go                    # CREATE — package doc; Arm(ctx,interval)+Stop(); osGetppid seam; notifier/once lifecycle; cancel-watcher goroutine
  arm_unix.go        (!windows)  # CREATE — armImpl: Linux best-effort armPdeathsig + getppid poll → signal.Trigger(SIGTERM) on change
  arm_windows.go     (windows)   # CREATE — armImpl no-op (FR-K7; no poll, no prctl)
  pdeathsig_linux.go (linux)     # CREATE — armPdeathsig: runtime.LockOSThread + syscall.Syscall6(SYS_PRCTL, PR_SET_PDEATHSIG, sig, 0,0,0,0)
  pdeathsig_nonlinux.go (!linux) # CREATE — armPdeathsig no-op stub (covers darwin + windows)
  watchdog_test.go               # CREATE — cross-platform tests (Stop no-op/cancel, interval default, nil-safety)
  watchdog_unix_test.go (!windows) # CREATE — detection-path test (ppid change → Trigger → exit 143) via osGetppid seam + fake Exit
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (prctl is PER-THREAD + best-effort): prctl(PR_SET_PDEATHSIG) sets the death-signal on the CALLING thread only.
// The contract pattern runtime.LockOSThread() + defer runtime.UnlockOSThread() pins the goroutine to one OS thread for the
// syscall, but AFTER defer UnlockOSThread the runtime MAY retire that thread → the prctl setting is LOST (no kernel SIGTERM on
// parent death). There is also a fork→prctl race window (parent can die before prctl runs). THEREFORE prctl is a best-effort
// latency optimization ONLY. The getppid() poll MUST ALWAYS run (even on Linux) as the reliable detector. Do NOT remove it.

// CRITICAL (detection = ppid CHANGE, NOT == 1): record originalPpid := os.Getppid() at Arm time; fire when os.Getppid() !=
// originalPpid. NEVER use getppid()==1 — wrong under subreapers (systemd-run/docker/supervisord reparent to a non-init pid).
// PRD §9.27 FR-K2 is explicit on this.

// CRITICAL (the value passed to prctl is a SIGNAL VALUE, not a pointer): Syscall6 arg2 = uintptr(sig), NOT a pointer to sig.
// Go's own exec_linux.go:550 uses RawSyscall6(SYS_PRCTL, PR_SET_PDEATHSIG, uintptr(sys.Pdeathsig), 0,0,0,0) — value form.
// syscall.PR_SET_PDEATHSIG == 0x1 is EXPORTED on every Linux arch (zerrors_linux_*.go) — use it for readability, or the literal 1.

// CRITICAL (reuse the single rescue path — do NOT reimplement teardown): the watchdog's ONLY effect is to call
// signal.Trigger(syscall.SIGTERM). That reuses handle(): forward-to-child-group → cancel ctx → rescue(exit 3 if snapshot armed)
// OR plain exit(143), with OnRescueExit(=lock.ReleaseCurrent) releasing the lock before exit on BOTH branches (FR52 §18.5).
// Do NOT import internal/lock in this package. Do NOT print a rescue message yourself. Trigger does it all.

// CRITICAL (nil-safety is a hard requirement): signal.Trigger is nil-safe (active.Load()==nil → no-op) — so Arm with no
// signal.Install (library use of pkg/stagecoach) must NOT crash and must NOT exit the process; the poll goroutine fires the
// (no-op) Trigger and returns. The goroutine ALWAYS exits cleanly on ctx cancel (no leak).

// GOTCHA (build-tag the files EXACTLY — a missing/mismatched tag = duplicate-symbol or "unused on platform"):
//   watchdog.go            (no tag, cross-platform)
//   arm_unix.go            //go:build !windows
//   arm_windows.go         //go:build windows
//   pdeathsig_linux.go     //go:build linux
//   pdeathsig_nonlinux.go  //go:build !linux
// Each //go:build line MUST be followed by a blank line, then `package watchdog`.

// GOTCHA (Windows unused-func edge): with the tags above, on a WINDOWS build arm_windows.go (no-op) never calls armPdeathsig
// while pdeathsig_nonlinux.go (`!linux`, includes windows) DEFINES it → staticcheck U1000 on a native-windows lint. NOTE:
// `GOOS=windows go build` still PASSES (build≠lint; unused funcs are lint, not compile), and `make lint` runs NATIVELY on this
// linux box where arm_unix.go references armPdeathsig (used). So all validation gates pass here. IF CI cross-lints windows,
// change pdeathsig_nonlinux.go to `//go:build !linux && !windows` (then windows defines no armPdeathsig and never references it).

// GOTCHA (leak-free poll requires a stop channel, not just a detection channel): a poll goroutine blocked on time.Ticker leaks
// (and can fire Trigger during a LATER test → test poisoning). Use the notify+sync.Once lifecycle design: the poll goroutine
// selects on ticker.C AND notify; it exits when notify is closed (by detection OR by the cancel-watcher on ctx cancel). See
// the Implementation Blueprint — do NOT write a poll that only returns on detection.

// GOTCHA (test packages isolate per-binary): `go test ./...` runs each PACKAGE in a separate binary, so the signal `active`
// singleton is fresh per package. Within THIS package, each test re-installs its own handler + RestoreDefault in Cleanup.
```

## Implementation Blueprint

### Data models and structure

No external types. The package owns:
- `var osGetppid = os.Getppid` — package-level seam (default real; tests swap it to simulate reparenting).
- `type notifier struct { once sync.Once; ch chan struct{} }` — the idempotent lifecycle channel (closed once for ANY reason:
  detection OR cancel). `fire()` does `once.Do(func(){ close(ch) })`; `done()` returns `<-chan struct{}`.
- a package-level `currentMu sync.Mutex` + `currentCancel context.CancelFunc` so `Stop()` can cancel the live watchdog.
- `armImpl(originalPpid int, interval time.Duration, n *notifier)` — build-tagged per platform.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/watchdog/watchdog.go (cross-platform orchestration)
  - PACKAGE doc comment: "Package watchdog implements the §9.27 FR-K1/K2 parent-death watchdog. When the launching process
    dies without sending a signal (the lazygit/IDE/detaching-terminal case — §18.5's orphaned-run gap), the child is
    reparented; the watchdog detects the parent-pid change and calls signal.Trigger(SIGTERM), reusing the signal handler's
    rescue/lock-release exit path. Detection is ppid CHANGE (subreaper-safe — NOT getppid()==1). prctl(PR_SET_PDEATHSIG) is a
    Linux best-effort kernel fast path; the getppid poll ALWAYS runs as the reliable detector. Windows is a no-op (FR-K7)."
  - IMPORTS: "context", "os", "sync", "time". (NO syscall, NO internal/signal in THIS file — those live in arm_unix.go so
    this cross-platform file stays free of unix-only refs. watchdog.go records ppid via osGetppid and owns Stop/lifecycle.)
  - IMPLEMENT:
      var osGetppid = os.Getppid   // test seam (tests swap to simulate parent death)
      var ( currentMu sync.Mutex; currentCancel context.CancelFunc )   // for Stop()
      type notifier struct { once sync.Once; ch chan struct{} }
      func newNotifier() *notifier { return &notifier{ch: make(chan struct{})} }
      func (n *notifier) fire() { n.once.Do(func(){ close(n.ch) }) }
      func (n *notifier) done() <-chan struct{} { return n.ch }
      // Arm: godoc citing FR-K1/K2/K7, the poll-always-runs guarantee, nil-safety, and "Linux also arms prctl best-effort".
      func Arm(ctx context.Context, interval time.Duration) {
          if interval <= 0 { interval = time.Second }   // FR-K2 default ~1s
          originalPpid := osGetppid()
          armCtx, cancel := context.WithCancel(ctx)
          currentMu.Lock(); currentCancel = cancel; currentMu.Unlock()
          n := newNotifier()
          armImpl(originalPpid, interval, n)   // build-tagged: Unix polls + (Linux) prctl; Windows no-op
          go func() { <-armCtx.Done(); n.fire() }()   // cancel-watcher: on process exit / Stop, stop the poll
      }
      // Stop: godoc — cancels the live watchdog (tests/library use); no-op if never armed; idempotent.
      func Stop() { currentMu.Lock(); c := currentCancel; currentCancel = nil; currentMu.Unlock(); if c != nil { c() } }
  - FOLLOW pattern: the cancel-watcher goroutine mirrors how signal.go's run() exits on channel close (lifecycle goroutine).
  - NAMING: Arm/Stop (exported, verb); osGetppid/notifier/currentCancel (unexported).
  - PLACEMENT: internal/watchdog/watchdog.go.

Task 2: CREATE internal/watchdog/arm_unix.go (`//go:build !windows`) — detection + Linux prctl
  - BUILD TAG: first line `//go:build !windows`, blank line, `package watchdog`.
  - IMPORTS: "runtime", "syscall", "time", "github.com/dustin/stagecoach/internal/signal".
  - IMPLEMENT armImpl:
      // armImpl arms Unix parent-death detection: a getppid() poll goroutine (reliable, subreaper-safe — FR-K2) PLUS, on
      // Linux, a best-effort prctl(PR_SET_PDEATHSIG) kernel fast path. On a parent-pid CHANGE it calls
      // signal.Trigger(SIGTERM) (the single rescue/exit path) and fires the notifier. The poll ALWAYS runs (even on Linux):
      // prctl is per-thread and the runtime can retire the LockOSThread-pinned thread, so the poll covers the race, thread
      // retirement, and the fork→prctl gap. n.fire() stops the poll and wakes the cancel-watcher.
      func armImpl(originalPpid int, interval time.Duration, n *notifier) {
          if runtime.GOOS == "linux" {
              _ = armPdeathsig(syscall.SIGTERM)   // best-effort; failure is non-fatal (the poll is the reliable detector)
          }
          go func() {
              t := time.NewTicker(interval)
              defer t.Stop()
              for {
                  select {
                  case <-n.done():                 // stopped (detection fired elsewhere OR cancel-watcher fired on ctx cancel)
                      return
                  case <-t.C:
                      if osGetppid() != originalPpid {   // parent-pid CHANGE (reparented to init/subreaper) — NOT == 1
                          signal.Trigger(syscall.SIGTERM) // routes through handle(): forward→cancel→rescue/exit + lock release
                          n.fire()                        // idempotent; wakes cancel-watcher + stops this goroutine's next tick
                          return
                      }
                  }
              }
          }()
      }
  - GOTCHA: the select MUST include `<-n.done()` so the goroutine stops on ctx cancel (leak-free). Detection calls Trigger THEN
    fires the notifier (the cancel-watcher also fires it on cancel — sync.Once makes both safe).
  - FOLLOW pattern: the injectable osGetppid seam + the signal.Trigger delegation (one rescue path).
  - PLACEMENT: internal/watchdog/arm_unix.go.

Task 3: CREATE internal/watchdog/arm_windows.go (`//go:build windows`) — no-op (FR-K7)
  - BUILD TAG: `//go:build windows`, blank line, `package watchdog`.
  - IMPORTS: "time" (only for the interval param type).
  - IMPLEMENT:
      // armImpl is a no-op on Windows (FR-K7): Windows has no controlling-terminal-hangup analog and no init-reparenting,
      // so there is no parent-death concept to watch. The notifier is never fired here; the cancel-watcher in watchdog.go
      // still fires it harmlessly on ctx cancel. No poll goroutine, no prctl.
      func armImpl(originalPpid int, interval time.Duration, n *notifier) {
          _ = originalPpid
          _ = interval
          _ = n
      }
  - NAMING/PLACEMENT: identical signature to arm_unix.go (the orchestrator calls one or the other per build tag).
  - GOTCHA: keep the underscore-blank-identifier assignments so `unused-parameter` linters (and `go vet`) stay quiet; OR name
    the params and rely on the no-op body (params ARE used by the `_ =` lines). Either is fine — pick the `_ =` form for clarity.

Task 4: CREATE internal/watchdog/pdeathsig_linux.go (`//go:build linux`) — the prctl syscall
  - BUILD TAG: `//go:build linux`, blank line, `package watchdog`.
  - IMPORTS: "runtime", "syscall".
  - IMPLEMENT:
      // armPdeathsig best-effort arms the kernel to deliver sig to THIS process when its parent dies (PR_SET_PDEATHSIG). It is
      // the Linux fast path for FR-K1/K2: when it works, the kernel sends a real SIGTERM with no 1s-poll latency, and SIGTERM
      // is already in the caught set so it flows through signal.Notify → handle() naturally. prctl is PER-THREAD, so we pin the
      // goroutine with runtime.LockOSThread() for the syscall (the runtime would otherwise migrate it to another thread). The
      // deferred Unlock means the runtime MAY later retire this thread → the setting can be lost; the getppid poll covers that.
      // Best-effort: returns the errno (caller ignores it); never fatal.
      func armPdeathsig(sig syscall.Signal) error {
          runtime.LockOSThread()
          defer runtime.UnlockOSThread()
          _, _, errno := syscall.Syscall6(syscall.SYS_PRCTL,
              uintptr(syscall.PR_SET_PDEATHSIG), // == 0x1 (exported on every Linux arch); the contract's literal 1 also works
              uintptr(sig),                      // VALUE, not a pointer (matches Go's exec_linux.go:550)
              0, 0, 0, 0)
          if errno != 0 {
              return errno
          }
          return nil
      }
  - GOTCHA: arg2 is uintptr(sig) — a VALUE (passing &sig would be wrong). syscall.PR_SET_PDEATHSIG is exported (==0x1); if for
    some reason a linter dislikes it, the literal uintptr(1) is equivalent and contract-compliant.
  - PLACEMENT: internal/watchdog/pdeathsig_linux.go.

Task 5: CREATE internal/watchdog/pdeathsig_nonlinux.go (`//go:build !linux`) — no-op stub
  - BUILD TAG: `//go:build !linux`, blank line, `package watchdog`.
  - IMPORTS: "syscall" (for the syscall.Signal param type).
  - IMPLEMENT:
      // armPdeathsig is a no-op on non-Linux (darwin/BSDs have no PR_SET_PDEATHSIG equivalent in this stdlib form; Windows has
      // no parent-death concept at all — FR-K7). The getppid poll (arm_unix.go) is the detector on these platforms. This stub
      // exists so arm_unix.go's reference to armPdeathsig compiles on darwin (the runtime.GOOS=="linux" gate means it is never
      // executed there, but the compiler still needs the symbol).
      func armPdeathsig(sig syscall.Signal) error { _ = sig; return nil }
  - GOTCHA (the Windows unused-func edge, see Known Gotchas): with `!linux` this file is ALSO compiled on windows, where
    arm_windows.go never calls armPdeathsig → staticcheck U1000 on a native-windows lint. All gates pass on this linux dev box.
    IF cross-linting windows in CI, change this tag to `//go:build !linux && !windows` (see Anti-Patterns).

Task 6: CREATE internal/watchdog/watchdog_test.go (cross-platform tests)
  - IMPORTS: "context", "testing", "time", "github.com/dustin/stagecoach/internal/signal".
  - TESTS:
      TestArm_NegativeIntervalDefaults: Arm with interval=0 should not panic and should use 1s internally — assert via Stop()
        cleaning up without hang (Arm a canceled ctx, Stop, done).
      TestArm_StopWithoutArmIsNoOp: call Stop() before any Arm() — must not panic (currentCancel==nil → guarded).
      TestArm_StopCancelsGoroutine: Arm(ctx, 1*time.Millisecond); time.Sleep(20ms) to let it start; Stop(); assert the package
        has no leaked goroutine by re-Arming on a new ctx and confirming a fresh start (or simply: no test hang under -race).
      TestArm_NilSignalHandlerIsNoOp: do NOT call signal.Install (so signal.Active()==nil). Arm(ctx, 5ms); swap osGetppid to a
        changed value; assert the process did NOT exit (the test function continues) and Stop() cleans up. Restores osGetppid
        in t.Cleanup. (signal.Trigger is a no-op with no handler; the goroutine fires the no-op Trigger and returns.)
  - PATTERN: each test swaps osGetppid via `old := osGetppid; osGetppid = func() int {...}; t.Cleanup(func(){ osGetppid = old })`.
  - GOTCHA: because signal.Active() may be set by a prior test in this same binary, for the nil-safety test set up cleanly OR
    rely on the fact that Trigger's nil-check is on active.Load() (if a prior test left a handler, RestoreDefault it first).
  - PLACEMENT: internal/watchdog/watchdog_test.go.

Task 7: CREATE internal/watchdog/watchdog_unix_test.go (`//go:build !windows`) — detection → exit 143
  - BUILD TAG: `//go:build !windows` (the detection path only exists on Unix; arm_windows is a no-op).
  - IMPORTS: "bytes", "context", "runtime", "testing", "time", "github.com/dustin/stagecoach/internal/signal",
    "github.com/dustin/stagecoach/internal/watchdog".
  - TEST TestArm_FiresTriggerOnParentDeath (clone the signal_test.go fake-Exit idiom):
      var exitCode int; buf := new(bytes.Buffer)
      ctx, _ := signal.Install(context.Background(), signal.Options{ Exit: func(c int){ exitCode = c }, Out: buf })
      t.Cleanup(func(){ signal.RestoreDefault() })   // stop the installed handler's run goroutine (closes its channel)
      const orig, reparented = 11111, 99999
      osGetppid = func() int { return orig }; t.Cleanup(func(){ osGetppid = os.Getppid })
      watchdog.Arm(ctx, 5*time.Millisecond)          // captures orig as originalPpid
      osGetppid = func() int { return reparented }   // simulate parent death (poll will see the change)
      // poll for the fake Exit to record 143 (SIGTERM pre-snapshot) within 2s
      deadline := time.Now().Add(2*time.Second)
      for time.Now().Before(deadline) && exitCode == 0 { runtime.Gosched() }
      if exitCode != 143 { t.Errorf("exitCode = %d, want 143 (SIGTERM pre-snapshot via Trigger)", exitCode) }
      watchdog.Stop()   // belt-and-suspenders cleanup
  - PATTERN: mirrors signal_test.go's installTestHandler + TestHandler_Exit143SIGTERM (fake Exit records without exiting).
  - GOTCHA: osGetppid is an UNEXPORTED package var — so this test MUST be in `package watchdog` (white-box, same package as
    watchdog.go), NOT `package watchdog_test`. Same for watchdog_test.go. (Both files use `package watchdog`.)
  - PLACEMENT: internal/watchdog/watchdog_unix_test.go.

Task 8: VERIFY — build (native+cross), vet, format, focused + full tests, lint, grep guards
  - go build ./... ; GOOS=windows go build ./... ; GOOS=linux go build ./...
  - go vet ./internal/watchdog/...
  - gofmt -l internal/watchdog/*.go   # must be empty
  - go test ./internal/watchdog/ -v                       # all tests
  - GOOS=windows go vet ./internal/watchdog/...           # the windows no-op armImpl compiles + vets
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the leak-free lifecycle (notify + sync.Once). The poll goroutine and the cancel-watcher BOTH can fire the
// notifier; sync.Once makes it safe. The poll exits on EITHER detection (it fired) OR cancel (the watcher fired).
func Arm(ctx context.Context, interval time.Duration) {
    if interval <= 0 { interval = time.Second }
    originalPpid := osGetppid()
    armCtx, cancel := context.WithCancel(ctx)
    currentMu.Lock(); currentCancel = cancel; currentMu.Unlock()
    n := newNotifier()
    armImpl(originalPpid, interval, n)
    go func() { <-armCtx.Done(); n.fire() }()   // process exit / Stop → stop the poll
}

// PATTERN: the Unix poll goroutine (detection calls Trigger directly; poll always runs).
func armImpl(originalPpid int, interval time.Duration, n *notifier) {
    if runtime.GOOS == "linux" { _ = armPdeathsig(syscall.SIGTERM) }   // best-effort
    go func() {
        t := time.NewTicker(interval); defer t.Stop()
        for {
            select {
            case <-n.done(): return                                  // stopped (detection OR cancel)
            case <-t.C:
                if osGetppid() != originalPpid {                     // ppid CHANGE — NOT == 1 (subreaper-safe)
                    signal.Trigger(syscall.SIGTERM)                  // the ONE rescue/exit path (nil-safe + stopped-guarded)
                    n.fire(); return
                }
            }
        }
    }()
}

// PATTERN: the prctl syscall (per-thread → LockOSThread; sig is a VALUE; best-effort errno ignored by caller).
func armPdeathsig(sig syscall.Signal) error {
    runtime.LockOSThread(); defer runtime.UnlockOSThread()
    _, _, errno := syscall.Syscall6(syscall.SYS_PRCTL,
        uintptr(syscall.PR_SET_PDEATHSIG), uintptr(sig), 0, 0, 0, 0)   // PR_SET_PDEATHSIG == 0x1
    if errno != 0 { return errno }
    return nil
}
```

### Integration Points

```yaml
PACKAGE BOUNDARY:
  - NEW package internal/watchdog (leaf). Imports: stdlib + github.com/dustin/stagecoach/internal/signal ONLY.
  - ONE-DIRECTIONAL: internal/signal MUST NOT import internal/watchdog (signal stays stdlib-only; no cycle).
  - Does NOT import internal/lock (the lock release rides signal's OnRescueExit seam wired in main.go).
  - Does NOT import internal/config (the cfg.NoParentWatchdog gate is read by the consumer, P1.M2.T2.S2).

DATABASE / MIGRATION / ROUTES / CONFIG: none. Pure new package; zero edits to existing files.

DOWNSTREAM (this subtask ENABLES but does NOT build):
  - P1.M2.T2.S2 (arming): adds `if !cfg.NoParentWatchdog { watchdog.Arm(ctx, 1*time.Second) }` in default_action.go
    immediately after `defer locker.Release()` (~line 79), where ctx := cmd.Context() (the signal-aware ctx from main.go).
  - NO production caller of watchdog.Arm exists after this subtask (expected; grep guard confirms 0 outside the new tests).

SCOPE FENCES: NO edit to internal/signal/* (Trigger already exported by P1.M1.T2.S1); NO edit to internal/config/* (the
  NoParentWatchdog field is P1.M2.T1.S1, a parallel sibling with zero file overlap); NO edit to internal/cmd/* (the consumer
  is P1.M2.T2.S2); NO new third-party dep (stdlib + internal/signal only); NO README/docs change (P1.M4.T2 owns the docs sync).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross build (the build tags must produce a valid package on every platform).
go build ./...
GOOS=linux   go build ./...
GOOS=windows go build ./...
# Expected: all clean. If GOOS=windows fails, a non-windows file references a windows-absent symbol (e.g. a unix-only syscall
# outside a build-tagged file). If GOOS=linux fails, the prctl file isn't tagged `linux` correctly.

# Vet (native + windows — the no-op arm_windows.go must vet too).
go vet ./internal/watchdog/...
GOOS=windows go vet ./internal/watchdog/...
# Expected: clean.

# Format.
gofmt -l internal/watchdog/*.go
# Expected: empty. If a file is listed: gofmt -w internal/watchdog/<file>.

# Lint (native; runs staticcheck/gosimple/govvet/errcheck/ineffassign/unused).
make lint
# Expected: zero errors. `unused` (U1000) would fire only if a func is unreferenced on THIS (linux) build — armPdeathsig IS
#           referenced by arm_unix.go on linux, so it's clean. (The windows-unused edge is latent; see Known Gotchas.)

# Scope guard: only new internal/watchdog files exist.
git status --porcelain
# Expected: 7 new files under internal/watchdog/ (5 source + 2 tests). ZERO modified files elsewhere.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new tests (focused).
go test ./internal/watchdog/ -v
# Expected: PASS — TestArm_NegativeIntervalDefaults, TestArm_StopWithoutArmIsNoOp, TestArm_StopCancelsGoroutine,
#           TestArm_NilSignalHandlerIsNoOp (cross-platform) + TestArm_FiresTriggerOnParentDeath (Unix only).

# Race detector (the package spawns goroutines — the race detector is the leak/poison guard).
go test -race ./internal/watchdog/ -v
# Expected: green. A leaked poll goroutine firing Trigger in a later test would surface here as a race or unexpected Exit.

# Full suite regression (the signal package's own tests must be untouched — we only ADDED an importer).
make test
# Expected: green. (internal/signal is unchanged; internal/watchdog is additive.)

# NOTE: make coverage-gate covers ONLY internal/{git,provider,generate,config} — internal/watchdog is NOT gated, but the
# tests above still provide real coverage of Arm/Stop/the poll/the prctl-error-path.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary (sanity — the new package links into the binary).
make build

# There is no production caller yet (P1.M2.T2.S2 wires it), so there is no end-to-end CLI behavior to observe from THIS
# subtask alone. The within-scope proof is Level 2 (the detection test fires Trigger → exit 143 end-to-end through the real
# signal.Install path) plus a clean `make build`. The full orphaned-lock e2e scenarios are P1.M4.T1.S1.

# Sanity: the package compiles into the binary and the binary still runs (no import-cycle / link error).
./bin/stagecoach --version
# Expected: prints the version (proves the new package links cleanly; an import cycle would fail at build/link).
```

> **Note**: this subtask is a pure new package with no production caller. The detection→rescue path is proven at the unit
> level (TestArm_FiresTriggerOnParentDeath drives a real `signal.Install` → fake-Exit through the watchdog's poll). The
> end-to-end "launcher dies → lock released" scenario is P1.M4.T1.S1 (it needs the P1.M2.T2.S2 consumer wiring first).

### Level 4: Creative & Domain-Specific Validation

```bash
# Scope guard 1: only new files under internal/watchdog/; ZERO edits elsewhere.
git diff --name-only $(git merge-base HEAD HEAD~1 2>/dev/null || echo HEAD~1) -- internal/ cmd/ pkg/ 2>/dev/null
git status --porcelain
# Expected: 7 new files under internal/watchdog/; no modified files in internal/signal, internal/config, internal/cmd, etc.

# Scope guard 2: the package imports ONLY stdlib + internal/signal.
go list -deps ./internal/watchdog | grep -E 'github.com/dustin/stagecoach' | grep -v '^github.com/dustin/stagecoach/internal/signal$'
# Expected: EMPTY (the only stagecoach dep is internal/signal). Critically NO internal/lock, NO internal/config.

# Scope guard 3: build tags are present + correct on every file.
for f in internal/watchdog/*.go; do echo "== $f =="; head -2 "$f"; done
# Expected: arm_unix.go → !windows; arm_windows.go → windows; pdeathsig_linux.go → linux; pdeathsig_nonlinux.go → !linux;
#           watchdog.go / *_test.go → (no tag or !windows for the unix test). Each //go:build line followed by a blank line.

# Scope guard 4: detection is ppid CHANGE, never == 1.
grep -rn 'Getppid()' internal/watchdog/
# Expected: a comparison `!= originalPpid` (or `osGetppid() != originalPpid`), NEVER `== 1`.

# Scope guard 5: prctl uses a VALUE (uintptr(sig)), not a pointer.
grep -n 'SYS_PRCTL' internal/watchdog/pdeathsig_linux.go
# Expected: syscall.Syscall6(syscall.SYS_PRCTL, uintptr(syscall.PR_SET_PDEATHSIG) [or 1], uintptr(sig), 0,0,0,0) — NO &sig.

# Scope guard 6: LockOSThread is used (prctl is per-thread).
grep -n 'LockOSThread' internal/watchdog/pdeathsig_linux.go
# Expected: runtime.LockOSThread() (+ defer runtime.UnlockOSThread()).

# Scope guard 7: the ONLY stagecoach call in the poll is signal.Trigger (no reimplemented teardown).
grep -rn 'Trigger\|lock\.\|Release' internal/watchdog/
# Expected: signal.Trigger(syscall.SIGTERM) in arm_unix.go. NO internal/lock reference anywhere (grep for 'lock"' → empty).

# Scope guard 8: ZERO production callers of watchdog.Arm (consumer is P1.M2.T2.S2).
grep -rn 'watchdog.Arm\|watchdog\.Stop' --include='*.go' internal/ cmd/ pkg/ | grep -v '_test.go'
# Expected: ZERO hits outside internal/watchdog/*_test.go. (default_action.go's `watchdog.Arm` lands in P1.M2.T2.S2.)

# Scope guard 9: godoc present on the package + Arm + Stop + each build-tagged helper (PRD Mode A: per-file godoc).
grep -rn 'FR-K' internal/watchdog/*.go
# Expected: each file's godoc cites the FR-K requirement it implements (FR-K1/K2 for Arm/poll; FR-K7 for the Windows no-op).

# Scope guard 10: cross-platform build still clean after all edits.
GOOS=windows go build ./... && GOOS=linux go build ./... && echo OK
# Expected: OK.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=linux go build` + `GOOS=windows go build` clean
- [ ] `go vet ./internal/watchdog/...` + `GOOS=windows go vet` clean
- [ ] `gofmt -l internal/watchdog/*.go` empty
- [ ] `make lint` zero errors (no `unused` on the native linux build)
- [ ] `make test` (race) green, incl. the 5-6 new tests
- [ ] `go test -race ./internal/watchdog/ -v` green (no leak, no poisoning)

### Feature Validation
- [ ] `watchdog.Arm(ctx, interval)` starts the poll; on a ppid CHANGE it calls `signal.Trigger(syscall.SIGTERM)` (test asserts
      exit 143 through a real `signal.Install` + fake `Exit`)
- [ ] Detection is ppid CHANGE (`!= originalPpid`), NEVER `== 1` (subreaper-safe — FR-K2)
- [ ] `watchdog.Stop()` (or ctx cancel) stops every goroutine with no leak; no-op if never armed; idempotent
- [ ] Nil-safe: `Arm` with `signal.Active()==nil` does NOT exit the process; goroutine exits on ctx cancel
- [ ] Linux best-effort prctl arms PR_SET_PDEATHSIG (LockOSThread + Syscall6, sig as VALUE); failure is non-fatal
- [ ] Windows `armImpl` is a no-op (no poll, no prctl) — FR-K7

### Scope-Boundary Validation
- [ ] `git status` shows ONLY new files under `internal/watchdog/` (7 files); ZERO modified files elsewhere
- [ ] `go list -deps ./internal/watchdog` → the only stagecoach dep is `internal/signal` (no internal/lock, no internal/config)
- [ ] ZERO production `watchdog.Arm` callers (consumer is P1.M2.T2.S2; grep guard #8)
- [ ] NO edit to internal/signal/* (Trigger already exported), internal/config/* (P1.M2.T1.S1), internal/cmd/* (P1.M2.T2.S2)
- [ ] NO new third-party dependency (go.mod unchanged; stdlib + internal/signal only)

### Code Quality & Docs
- [ ] Package doc + Arm/Stop + each helper has a godoc comment citing §9.27 and the FR-K it implements (PRD Mode A)
- [ ] Build tags exactly: `!windows` / `windows` / `linux` / `!linux` (each followed by a blank line before `package watchdog`)
- [ ] Tests are white-box (`package watchdog`) so they can swap the unexported `osGetppid` seam
- [ ] The poll ALWAYS runs on Unix (Linux prctl is best-effort; the poll is the reliable detector) — documented, not "optimized away"

---

## Anti-Patterns to Avoid

- ❌ Don't remove the getppid poll "because prctl handles Linux." prctl(PR_SET_PDEATHSIG) is PER-THREAD; after
  `defer runtime.UnlockOSThread()` the runtime MAY retire the pinned thread → the setting is LOST. There is also a
  fork→prctl race window. The poll ALWAYS runs (even on Linux) as the reliable detector. prctl is only a latency optimization.
- ❌ Don't detect with `getppid() == 1`. Under subreapers (systemd-run, docker, supervisord, some shells) the reparent pid is
  NOT 1. Detect a CHANGE from the `originalPpid` captured at Arm time (`os.Getppid() != originalPpid`). PRD §9.27 FR-K2.
- ❌ Don't pass `&sig` (a pointer) to prctl. The second prctl arg is the signal VALUE. Use `uintptr(sig)` (matches Go's own
  exec_linux.go:550). A pointer would set the wrong value and silently fail.
- ❌ Don't reimplement the rescue/lock-release teardown. The watchdog's ONLY effect is `signal.Trigger(syscall.SIGTERM)` — that
  reuses `handle()` (forward→cancel→rescue/exit + `OnRescueExit`=lock.ReleaseCurrent on BOTH branches). Do NOT import
  internal/lock, do NOT print a rescue message yourself, do NOT call os.Exit.
- ❌ Don't import internal/lock or internal/config in this package. The lock release rides signal's OnRescueExit seam (wired in
  main.go); the config gate is read by the consumer (P1.M2.T2.S2). Keep this package stdlib + internal/signal only. Importing
  internal/lock would break the stdlib-only invariant of lock (lock must not be pulled into a signal-importing package's graph
  in a way that... ) — simplest rule: this package imports ONLY internal/signal.
- ❌ Don't write a poll goroutine that only returns on detection. It would leak (blocked on `time.Ticker`) and could fire
  `signal.Trigger` during a LATER test in the same binary → test poisoning. Always `select` on the notifier's `done()` channel
  too, so ctx cancel / Stop stops the goroutine (leak-free).
- ❌ Don't make `Arm` exit the process when no signal handler is installed. Nil-safety is a hard requirement: with
  `signal.Active()==nil`, `signal.Trigger` is a no-op and the process must continue (library use of pkg/stagecoach). The
  goroutine exits on ctx cancel. Do NOT add an `os.Exit` fallback "just in case."
- ❌ Don't wire the consumer. `watchdog.Arm(ctx, …)` is called by P1.M2.T2.S2 in default_action.go (gated by
  `cfg.NoParentWatchdog`). After THIS subtask, grep must show ZERO production `watchdog.Arm` callers.
- ❌ Don't edit existing files. This is a pure new package. internal/signal already exports `Trigger` (P1.M1.T2.S1);
  internal/config already gets `NoParentWatchdog` (P1.M2.T1.S1, a parallel sibling with zero file overlap); internal/cmd gets
  the consumer later (P1.M2.T2.S2). Touching any of them here = scope creep + merge conflict.
- ❌ Don't add `golang.org/x/sys` or any third-party dep for the prctl syscall. `syscall.SYS_PRCTL` + `syscall.PR_SET_PDEATHSIG`
  (=0x1) are in the Go stdlib on Linux (verified: zerrors_linux_*.go). The whole point of the new package is stdlib-only.
- ❌ Don't forget the `//go:build` line on the 4 platform files — a missing tag = duplicate symbol
  (e.g. two `armImpl` on Linux) or "unused on platform." Each tag line must be followed by a blank line before `package watchdog`.
- ❌ Don't write the tests as `package watchdog_test` (black-box). The tests swap the unexported `osGetppid` seam, so they MUST
  be `package watchdog` (white-box, same package). Same-package tests are the established idiom here (see internal/signal's
  signal_test.go using the unexported `active`/`contains`).

---

## Confidence Score: 9/10

This is a self-contained new leaf package with a tiny public surface (`Arm`/`Stop`), verbatim code for all 5 source files +
2 test files (build tags + imports included), the exact `signal.Trigger`/`signal.Active` APIs it consumes (read from the real
signal.go), the verified Go-stdlib facts (`syscall.SYS_PRCTL` + `syscall.PR_SET_PDEATHSIG=0x1` exist; Go's own exec_linux.go:550
uses this exact Syscall6 pattern), the LockOSThread gotcha spelled out (why prctl is best-effort and the poll ALWAYS runs), the
leak-free `notify`+`sync.Once` lifecycle design (no goroutine leak / no test poisoning), the `osGetppid` test seam, the test
idiom to clone (signal_test.go's injectable `Exit` → assert exit 143), the consumer's exact future call site, and 10 grep
guards. The one residual uncertainty is the latent Windows unused-func (U1000) edge for `armPdeathsig` under a hypothetical
cross-platform lint — fully analyzed (passes every native-linux gate here; the `!linux && !windows` refinement is documented
if CI ever cross-lints). No edits to existing files, no new third-party deps, no consumer wiring, no import cycle. One-pass
success is highly likely; the -2 from 10/10 reflects that prctl-on-Go edge cases (thread retirement, container pid-namespace
behavior) are inherently best-effort and the PRP correctly treats them as such rather than over-promising.
