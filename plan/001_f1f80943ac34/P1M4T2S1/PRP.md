---
name: "P1.M4.T2.S1 — Signal handler: intercept SIGINT/SIGTERM, forward to child process group, rescue, restore before commit — PRD §18.4 / §18.2 / §9.10 FR45"
description: |

  Add a signal-handling module (`internal/signal`) that intercepts SIGINT/SIGTERM, forwards the signal
  to the currently-running child agent's PROCESS GROUP (FINDING 8 — `Setpgid` isolates the child, so
  terminal Ctrl-C does NOT reach it; the parent MUST forward via `syscall.Kill(-pgid, sig)`), runs the
  §18.3 rescue path if the snapshot was taken, and restores the default signal disposition right before
  the final `update-ref` (matching commit-pi's `trap - INT TERM`). The module is wired into the generate
  flow (`generate.CommitStaged` + `pkg/stagecoach.runPipeline`) and the CLI (`cmd/stagecoach/main.go`).

  CONTRACT (PRD §18.4 / §18.2 / §9.10 FR45, FINDING 8):
    On SIGINT/SIGTERM:
      1. If a child PID is registered (set by the executor after `cmd.Start`), forward the signal to the
         child's process group via `KillProcessGroup` (grace period + SIGKILL escalation already wired in
         the executor's `cmd.Cancel`/`WaitDelay` — belt-and-suspenders/immediacy here).
      2. Cancel the signal-aware context (so `provider.Execute` unwinds; the executor's `cmd.Cancel` also
         SIGTERMs the group — idempotent).
      3. If the snapshot state's `treeSHA` is non-empty → print `FormatRescue(treeSHA, parentSHA,
         candidate)` to STDERR + exit 3 (the AUTHORITATIVE rescue — fires in ALL post-snapshot windows,
         including `CommitTree`, where the normal error-return path would wrongly map to exit 1). Else
         (pre-snapshot) → exit 130 (128+SIGINT), "just exit" per §18.4 step 2.
      4. `RestoreDefaultHandler()` is called right before `update-ref` so a Ctrl-C at the very last
         instant isn't mistaken for a failure (Go default disposition, NOT rescue).

  INPUT (upstream — all EXIST, READ/CONSUME only, do NOT modify their behavior):
    - `generate.FormatRescue(treeSHA, parentSHA, candidateMsg) string`  — P1.M3.T3.S1 (internal/generate/rescue.go). FROZEN. Passed to the
      handler as a CALLBACK (`Options.RescueFormat`) at Install time (avoids signal→generate import cycle).
    - The executor's `setupProcessGroup(cmd)` (internal/provider/procgroup_unix.go / _windows.go) — P1.M2.T5.S2. It ALREADY wires `cmd.Cancel =
      syscall.Kill(-pid, SIGTERM)` + `WaitDelay = 3s`, so cancelling the ctx kills the child group. We do
      NOT refactor it (signature FROZEN). We ADD a direct forward-kill via the handler for immediacy +
      contract fidelity.

  ⚠️ NOTE ON "KillProcessGroup from P1.M2.T5.S2": P1.M2.T5.S2 shipped `func setupProcessGroup(cmd
     *exec.Cmd)` — the kill-to-group logic lives INSIDE its `cmd.Cancel` closure, NOT as a standalone
     function. This task therefore CREATES `KillProcessGroup` (cross-platform, Unix + Windows build-tag
     files) in the new `internal/signal` package. It is the single mechanism the handler uses.

  DELIVERABLES (5 NEW files + 4 EDITS):
    NEW internal/signal/signal.go            — Handler type, Install/Active, nil-safe package wrappers,
                                              the handler goroutine (forward→cancel→rescue/exit).
    NEW internal/signal/signal_unix.go       — //go:build !windows: KillProcessGroup + exitCodeForSignal.
    NEW internal/signal/signal_windows.go    — //go:build windows: KillProcessGroup + exitCodeForSignal.
    NEW internal/signal/signal_test.go       — in-process unit tests (state methods + injectable Kill/Exit).
    NEW internal/signal/signal_integration_test.go — //go:build !windows: subprocess SIGINT→rescue→exit 3.
    EDIT internal/provider/executor.go       — after cmd.Start: signal.RegisterChild(pid); defer
                                              signal.ClearChild(). (+ import internal/signal.)
    EDIT internal/generate/generate.go       — SetSnapshot after WriteTree; SetCandidate in loop;
                                              RestoreDefault before UpdateRefCAS; ClearSnapshot after.
    EDIT pkg/stagecoach/stagecoach.go          — identical wiring in runPipeline (commit path only).
    EDIT cmd/stagecoach/main.go               — Install handler (RescueFormat=generate.FormatRescue,
                                              Out=os.Stderr); pass signal-aware ctx to cmd.Execute.

  SCOPE BOUNDARY (owned by siblings — do NOT implement or edit):
    - root.go / config.go / providers.go / default_action.go — P1.M4.T1 siblings (running in PARALLEL;
      do NOT touch root.go, config.go, providers.go, default_action.go, or main.go's version plumbing).
      The ONLY main.go change is swapping `context.Background()` for the signal-aware ctx + Install
      (main.go's comment explicitly reserves this for P1.M4.T2).
    - FormatRescue / RescueError / the §18.3 message text — P1.M3.T3.S1 / P1.M3.T4 (FROZEN; we only CALL
      FormatRescue).
    - procgroup_*.go (setupProcessGroup) — P1.M2.T5.S2 (FROZEN; do NOT refactor).
    - UI/color/TTY (P1.M4.T3) — the handler writes plain text to os.Stderr; P1.M4.T3 may colorize later.
    - double-Ctrl-C force-exit polish (go_ecosystem_patterns §4.4) — OPTIONAL future; v1 exits on 1st signal.

  DEPENDENCY GRAPH (CYCLE-FREE — see research/design-decisions.md F3):
    signal → (stdlib only). provider → signal. generate → signal. pkg/stagecoach → signal + generate.
    cmd/stagecoach → signal + generate + cmd. The rescue message reaches the handler via a CALLBACK
    (Options.RescueFormat, wired in main.go), so signal never imports generate (no signal↔generate cycle).

  Deliverable: 5 NEW files + 4 EDITS. `make build` → a real Ctrl-C during generation (post-snapshot)
  kills the agent tree, prints the §18.3 rescue block to stderr, and exits 3 with HEAD byte-for-byte
  unchanged. A Ctrl-C before the snapshot exits 130. A Ctrl-C during `update-ref` (after RestoreDefault)
  uses the default disposition (NOT rescue). `go test -race ./internal/signal/ -v` green; `go test -race
  ./...` no regression; `go vet ./...` clean; `gofmt -l internal/signal/ internal/provider/
  internal/generate/ pkg/stagecoach/ cmd/stagecoach/` empty.

---

## Goal

**Feature Goal**: Ship Stagecoach's SIGINT/SIGTERM safety net (PRD §18.4 / §9.10 FR45 / FINDING 8): a
signal-handling module that intercepts Ctrl-C / SIGTERM, forwards the signal to the running child
agent's whole process group (so no orphaned grandchildren survive), runs the §18.3 rescue protocol if
the snapshot was taken (print TREE_SHA + manual recovery command, exit 3), or exits cleanly (130)
pre-snapshot, and restores the default signal disposition immediately before the final atomic
`update-ref` so a last-instant Ctrl-C isn't misreported as a failure.

**Deliverable** (5 NEW files + 4 EDITS):
1. NEW `internal/signal/signal.go` — `Handler` type + `Install(parent ctx, Options) (ctx, *Handler)` +
   `Active()` + nil-safe package wrappers (`RegisterChild`/`ClearChild`/`SetSnapshot`/`SetCandidate`/
   `ClearSnapshot`/`RestoreDefault`) + the handler goroutine (forward → cancel → rescue-or-exit).
2. NEW `internal/signal/signal_unix.go` (`//go:build !windows`) — `KillProcessGroup` (`syscall.Kill(-pgid,
   sig)`) + `exitCodeForSignal` (SIGINT→130, SIGTERM→143).
3. NEW `internal/signal/signal_windows.go` (`//go:build windows`) — `KillProcessGroup`
   (`GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, pid)` via kernel32 LazyProc, stdlib-only) +
   `exitCodeForSignal`.
4. NEW `internal/signal/signal_test.go` — in-process unit tests (state methods + forward-kill + rescue
   via injectable `Options.Kill`/`Options.Exit`/`Options.RescueFormat`/`Options.Out`).
5. NEW `internal/signal/signal_integration_test.go` (`//go:build !windows`) — subprocess test: build the
   real binary, hang a stub agent, send SIGINT, assert exit 3 + rescue printed + child killed + HEAD
   unchanged.
6. EDIT `internal/provider/executor.go` — after `cmd.Start`: `signal.RegisterChild(cmd.Process.Pid)`;
   `defer signal.ClearChild()`. (+ `import "github.com/dustin/stagecoach/internal/signal"`.)
7. EDIT `internal/generate/generate.go` — after `WriteTree`: `signal.SetSnapshot(treeSHA, parentSHA, "")`;
   in the generate loop after each parsed `m`: `signal.SetCandidate(m)`; right before `UpdateRefCAS`:
   `signal.RestoreDefault()`; after success (before return): `signal.ClearSnapshot()`.
8. EDIT `pkg/stagecoach/stagecoach.go` — identical wiring in `runPipeline` (the commit path, i.e. the
   `!dryRun` branch that does `WriteTree`); the DryRun branch is UNCHANGED (no snapshot).
9. EDIT `cmd/stagecoach/main.go` — replace `context.Background()` with `signal.Install(ctx, …)`,
   `Options{RescueFormat: generate.FormatRescue, Out: os.Stderr}`; pass the returned ctx to
   `cmd.Execute`.

**Success Definition**: `make build` then, in a real git repo with staged changes using a slow/hanging
agent: pressing Ctrl-C mid-generation (after the snapshot) kills the agent subprocess (and any
grandchildren), prints the exact §18.3 rescue block (❌ notice, 60-`-` separators, `Tree ID: <sha>`,
the `git commit-tree … | xargs git update-ref HEAD` command, the first-commit hint) to **stderr**, and
exits **3**, with `git rev-parse HEAD` UNCHANGED and the staged index byte-for-byte intact. Pressing
Ctrl-C BEFORE the snapshot exits 130 and prints no rescue. Pressing Ctrl-C during `update-ref` (after
`RestoreDefault`) uses the default disposition (no rescue). `go test -race ./internal/signal/` green;
`go test -race ./...` no regression; `go vet ./...` clean; only the 9 listed files changed.

## User Persona

**Target User**: The Stagecoach CLI user (PRD §7 "the plan-holder") who kicks off a generation that may
take tens of seconds and legitimately needs to abort it (wrong provider, wrong staged files, changed
their mind) WITHOUT losing their staged work or leaving an orphaned agent process running in the
background.

**Use Case**: `git add -A && stagecoach` → realize mid-generation you staged the wrong thing → press
Ctrl-C → get a clean rescue message + the exact manual-recovery command, exit 3, repo untouched.

**User Journey**: user runs stagecoach → generation starts (agent subprocess) → user presses Ctrl-C →
agent is killed (no lingering process) → user sees the §18.3 rescue block with their `TREE_SHA` → user
re-stages correctly and re-runs (or uses the printed `git commit-tree` command to commit the snapshot
manually).

**Pain Points Addressed**: (1) orphaned agent grandchildren after Ctrl-C (Setpgid isolates the child —
without explicit forwarding they survive) — solved by `KillProcessGroup`; (2) "did my staged work
survive the abort?" — solved by the rescue message + the frozen-TREE_SHA invariant (§18.1); (3) a Ctrl-C
at the very last instant looking like a failure — solved by `RestoreDefault` before `update-ref`.

## Why

- **Closes PRD §18.4 / §9.10 FR45 (P0 safety).** Without this, a Ctrl-C during generation leaves the
  agent subprocess (and any tool grandchildren) running (FINDING 8: Setpgid means the terminal's SIGINT
  does NOT reach the child), and the user has no recovery path — violating the §18.1 invariant
  ("every path that doesn't reach update-ref leaves the repo byte-for-byte unchanged") in spirit (the
  repo IS unchanged, but the user doesn't KNOW that, and orphaned processes litter their machine).
- **The rescue must fire in ALL post-snapshot windows (F6).** The normal error-return path only produces
  a `*RescueError` when the signal lands during `provider.Execute`. A signal during `CommitTree` would
  wrongly map to exit 1. The handler's direct rescue-print + `os.Exit(3)` fixes this uniformly.
- **Reuses frozen upstream (zero new domain logic).** `FormatRescue` (P1.M3.T3.S1) assembles the exact
  message; the executor's `cmd.Cancel`/`WaitDelay` (P1.M2.T5.S2) already kill the group on ctx-cancel.
  This task wires them together + adds the intercept/forward/restore glue.
- **Library-safe (D4).** `signal` is opt-in (`Install`). A `pkg/stagecoach` consumer who never installs
  it gets baseline behavior (their own ctx/signals; `cmd.Cancel` still kills the group). No behavior
  change for library use.

## What

A new `internal/signal` package exposing:
- `type Options struct { RescueFormat func(treeSHA, parentSHA, candidate string) string; Out io.Writer; Kill func(pid int, sig os.Signal) error; Exit func(int) }` — all defaulted (`Kill`→`KillProcessGroup`, `Exit`→`os.Exit`, `Out`→`os.Stderr`, `RescueFormat`→a base formatter if nil); fakes enable in-process unit tests.
- `func Install(parent context.Context, opts Options) (context.Context, *Handler)` — `signal.Notify` on a buffered channel for `os.Interrupt` + `syscall.SIGTERM`; starts the handler goroutine; stores the handler in a package-level `atomic.Pointer[Handler]`; returns a child ctx cancelled on signal.
- `func Active() *Handler` — current handler or nil (library use).
- Nil-safe package wrappers: `RegisterChild(pid int)`, `ClearChild()`, `SetSnapshot(treeSHA, parentSHA, candidate string)`, `SetCandidate(candidate string)`, `ClearSnapshot()`, `RestoreDefault()` — each no-ops when `Active()==nil`.
- The handler goroutine: on signal → if registered child PID > 0, `opts.Kill(pid, sig)` (forward to group); `cancel()`; read snapshot under lock; if `treeSHA != ""` → `fmt.Fprintln(opts.Out, opts.RescueFormat(tree, parent, candidate))` + `opts.Exit(3)`; else `opts.Exit(exitCodeForSignal(sig))`.
- `KillProcessGroup(pid int, sig os.Signal) error` + `exitCodeForSignal(sig os.Signal) int` — build-tag files (Unix `syscall.Kill(-pid, sig)` / Windows `GenerateConsoleCtrlEvent`).

Wiring (4 edits): `provider.Execute` registers/clears the child PID; `generate.CommitStaged` and
`pkg/stagecoach.runPipeline` register/update/clear the snapshot + `RestoreDefault` before `update-ref`;
`main.go` installs the handler and passes the signal-aware ctx.

### Success Criteria

- [ ] `internal/signal/signal.go` exists, `package signal`, stdlib-only imports (+ no stagecoach imports).
- [ ] `Install` returns a ctx that is cancelled when SIGINT/SIGTERM is delivered; `Active()` returns the
      handler; the package wrappers are nil-safe when no handler is installed.
- [ ] The handler goroutine, on signal, forwards to the registered child PID's group (via the injectable
      `Kill`), cancels the ctx, and either prints `RescueFormat(tree,parent,candidate)` to `Out` + exits
      3 (snapshot set) or exits 130/143 (snapshot empty).
- [ ] `RestoreDefault()` stops signal delivery (`signal.Stop` + close channel) so a later signal uses the
      default disposition.
- [ ] `provider.Execute` calls `signal.RegisterChild(cmd.Process.Pid)` after `cmd.Start` and defers
      `signal.ClearChild()`; existing executor behavior UNCHANGED (cmd.Cancel/WaitDelay intact).
- [ ] `generate.CommitStaged` calls `signal.SetSnapshot` after `WriteTree`, `signal.SetCandidate` after
      each parsed message, `signal.RestoreDefault` immediately before `UpdateRefCAS`, `signal.ClearSnapshot`
      before returning success. (Rescue/CAS/error return paths UNCHANGED.)
- [ ] `pkg/stagecoach.runPipeline` gets the identical wiring on its commit (`!dryRun`) branch; DryRun
      branch UNCHANGED.
- [ ] `main.go` installs the handler with `RescueFormat: generate.FormatRescue`, `Out: os.Stderr`, and
      passes the signal-aware ctx to `cmd.Execute`. main.go's version plumbing + error-print/exit logic
      UNCHANGED.
- [ ] `go test -race ./internal/signal/ -v` green (unit + integration); `go test -race ./...` NO
      regression; `go vet ./...` clean; `gofmt -l` empty for the changed trees. Only the 9 listed files
      changed (`git status`); root.go/config.go/providers.go/default_action.go/procgroup_*.go/rescue.go
      UNCHANGED (`git diff` empty for each).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact upstream
signatures (all quoted below + in research/design-decisions.md F1–F9), the frozen contracts (FormatRescue,
setupProcessGroup, the executor's cmd.Cancel/WaitDelay), the dependency-graph reasoning (F3), the
go_ecosystem_patterns §3/§4 canonical patterns, the copy-ready skeletons in the Implementation Blueprint,
and the test conventions to mirror (`stubtest.Build`, `generate_test.go`'s `initRepo`/`commitRaw`/
`writeFile`/`stageFile`). No CLI/UI/dry-run/color knowledge required (explicitly out of scope).

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M4T2S1/research/design-decisions.md
  why: the 9 decisions (D1–D9) + 9 findings (F1–F9) SPECIFIC to this subtask. F1 (executor ALREADY kills
       the group via cmd.Cancel on ctx-cancel — our forward-kill is belt-and-suspenders), F2 (there is NO
       KillProcessGroup yet — we CREATE it), F3 (the dependency graph is cycle-free BECAUSE RescueFormat is
       a callback, not an import), F4 (singleton justified; nil-safe wrappers), F5 (os.Exit ⇒ subprocess
       integration test + injectable Kill/Exit for unit tests), F6 (WHY the handler prints rescue directly
       — the CommitTree window), F7 (RestoreDefault semantics), F8 (wire BOTH CommitStaged AND runPipeline),
       F9 (candidate propagation via SetCandidate).
  critical: F3 (the callback trick prevents the signal↔generate cycle), F6 (WHY direct os.Exit rescue),
       F5 (the two test layers), F2 (KillProcessGroup is NEW, not pre-existing).

- docfile: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md
  section: "§3 os/exec: Safe Subprocess Execution with Process Groups" (3.1 Canonical Pattern, 3.4 Key
       Safety Notes, 3.5 Setpgid and Signal Forwarding Interaction) AND "§4 signal.Notify: Intercept,
       Forward, Cleanup" (4.1 Complete Pattern, 4.2 Integration, 4.3 signal.NotifyContext, 4.5 Signal Flow).
  why: §3.5 explains WHY Setpgid means the child does NOT receive the terminal's Ctrl-C (the parent MUST
       forward via -pid). §4.1/4.2 is the canonical handler pattern (buffered channel, forward-to-group,
       cancel, cleanup). §4.5 is the signal-flow diagram. Our handler follows §4.1's shape but uses a
       singleton + callback (per F3/F4) instead of a returned Handler threaded through every signature.
  pattern: buffered `chan os.Signal` (size 1+); `signal.Notify(ch, os.Interrupt, syscall.SIGTERM)`;
       goroutine reads the channel; on signal: read child PGID under lock → Kill(-pgid, sig) → cancel →
       cleanup → exit. `signal.Stop(ch)` to restore default.
  gotcha: §4.2's "do NOT set cmd.Cancel" advice does NOT apply to us — our executor ALREADY sets cmd.Cancel
       (P1.M2.T5.S2) and we do NOT change it. Both kill paths coexist (idempotent SIGTERM to the group).

- docfile: plan/001_f1f80943ac34/architecture/critical_findings.md
  section: "FINDING 8 — Signal handling requires process-group kill (Setpgid)" + "FINDING 10 — Windows
       SysProcAttr.Setpgid is Unix-only".
  why: FINDING 8 is the AUTHORITY for this task (forward via syscall.Kill(-pid, sig); restore default
       before update-ref). FINDING 10 mandates the build-tag abstraction for KillProcessGroup
       (Unix syscall.Kill vs Windows GenerateConsoleCtrlEvent) — already implemented in procgroup_*.go;
       our signal_unix.go/signal_windows.go mirror it.
  critical: the grace-then-SIGKILL escalation is the executor's `WaitDelay` (3s) — do NOT reimplement it
       in the handler. The handler sends ONE SIGTERM (forward) + relies on WaitDelay for escalation.

- file: internal/provider/executor.go   (P1.M2.T5.S1 — Execute; S1 EDITS this additively)
  section: `func Execute(ctx, spec CmdSpec, timeout) (stdout, stderr string, err error)` — the body around
       `cmd := exec.CommandContext(...)` → `setupProcessGroup(cmd)` → `if err := cmd.Start(); err != nil`
       → `if werr := cmd.Wait(); werr != nil`. The cmd.Start success point is WHERE we register the PID.
  why: S1 adds `signal.RegisterChild(cmd.Process.Pid)` immediately after the successful `cmd.Start()` (the
       `cmd.Process` is guaranteed non-nil there — same guarantee procgroup_unix.go's cmd.Cancel relies on)
       and `defer signal.ClearChild()` so the registration is cleared whether Wait succeeds or fails. This
       is the ONLY change to Execute (2 lines + import). Execute's error contract (ctx.Err() FIRST) is
       UNCHANGED.
  pattern: insert right after the `if err := cmd.Start(); err != nil { … }` block:
       `signal.RegisterChild(cmd.Process.Pid); defer signal.ClearChild()`.
  gotcha: do NOT touch setupProcessGroup(cmd) or cmd.Cancel/WaitDelay. do NOT change Execute's signature
       or return contract. The `defer signal.ClearChild()` must run before Execute returns so a stale PID
       isn't left registered (a later signal would kill a recycled PID).

- file: internal/provider/procgroup_unix.go   (P1.M2.T5.S2 — READ; do NOT edit)
  section: `func setupProcessGroup(cmd *exec.Cmd)` sets `SysProcAttr.Setpgid=true`, `cmd.Cancel =
       syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)`, `cmd.WaitDelay = 3*time.Second`.
  why: this PROVES the child runs as its own process-group leader (PGID==PID) and that cancelling the ctx
       already SIGTERMs the whole group + escalates to SIGKILL after 3s. Our `signal_unix.go`
       `KillProcessGroup(pid, sig)` does the SAME `syscall.Kill(-pid, sig)` (negates internally) — so the
       handler's forward-kill is byte-identical to the executor's escalation mechanism. Read this file to
       copy the exact syscall idiom.
  gotcha: signature FROZEN — do NOT refactor setupProcessGroup to call signal.KillProcessGroup (would be a
       provider→signal import on the hot path + touches a frozen file). Leave it inline. The kill
       one-liner is intentionally duplicated across two independent kill paths.

- file: internal/provider/procgroup_windows.go   (P1.M2.T5.S2 — READ; do NOT edit)
  section: `var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc(...)` +
       `setupProcessGroup` using `CREATE_NEW_PROCESS_GROUP` + `GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT,
       cmd.Process.Pid)`.
  why: this is the EXACT technique `signal_windows.go`'s `KillProcessGroup` must mirror (stdlib-only
       LazyProc, no golang.org/x/sys; CTRL_BREAK not CTRL_C because CTRL_C can't be limited to one group).
       Copy the LazyProc resolution + Call + r1==0 error handling verbatim.
  gotcha: Windows KillProcessGroup takes the POSITIVE pid (GenerateConsoleCtrlEvent's dwProcessGroupId ==
       the child's PID, since CREATE_NEW_PROCESS_GROUP ⇒ PID==PGID). Do NOT negate. sig is effectively
       ignored (always CTRL_BREAK for graceful; force-escalation is the executor's WaitDelay/
       TerminateProcess, not our job).

- file: internal/generate/rescue.go   (P1.M3.T3.S1 — FormatRescue; READ, do NOT edit)
  section: `func FormatRescue(treeSHA, parentSHA, candidateMsg string) string` — pure string assembler
       (3 strings in, 1 string out, no I/O). Returns the §18.3 block with NO trailing newline (the handler
       adds it via fmt.Fprintln). Omits `-p <parentSHA>` when parentSHA=="" (root commit).
  why: THIS is the function the handler calls (via the Options.RescueFormat callback, wired in main.go to
       generate.FormatRescue). The handler passes (snapshot.tree, snapshot.parent, snapshot.candidate).
       Signature FROZEN — call it, don't reimplement it.
  gotcha: FormatRescue has NO trailing newline — the handler MUST use fmt.Fprintln (not Fprint) to add one.
       The "(interrupted)" variant is NOT in scope (FormatRescue always produces the §18.3 base form) — a
       signal rescue is indistinguishable in message from any other rescue, which is fine (the §18.3
       message is generic; the user knows they pressed Ctrl-C).

- file: internal/generate/generate.go   (P1.M3.T4.S2 — CommitStaged; S1 EDITS this additively)
  section: `func CommitStaged(ctx, deps Deps, cfg config.Config) (Result, error)` — the 10-step pipeline.
       The EDIT points: (a) right after `treeSHA, err := deps.Git.WriteTree(ctx)` succeeds (the
       `*** SNAPSHOT TAKEN ***` comment) → `signal.SetSnapshot(treeSHA, parentSHA, "")`; (b) inside the
       generate loop, after `m, ok, _ := provider.ParseOutput(...)` succeeds (`if !ok {…}` else-branch,
       before `subject := ExtractSubject(m)`) → `signal.SetCandidate(m)`; (c) right before
       `deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld)` → `signal.RestoreDefault()`; (d) before
       the final `return Result{…}, nil` (success) → `signal.ClearSnapshot()`.
  why: these are the exact post-snapshot windows §18.4 governs. SetSnapshot arms the rescue; SetCandidate
       keeps the §18.3 candidate note current; RestoreDefault neuters the handler for the update-ref
       window; ClearSnapshot is belt-and-suspenders cleanup on success. The RescueError/CASError/error
       returns are UNCHANGED (they handle NON-signal failures; a signal makes the handler os.Exit first).
  pattern: `signal.SetSnapshot(treeSHA, parentSHA, "")` (nil-safe; no-op when no handler). For (c), place
       RestoreDefault on its own line immediately before the UpdateRefCAS call (mirroring commit-pi's
       `trap - INT TERM` immediately before `git commit`).
  gotcha: do NOT add signal calls to the DryRun path (CommitStaged has none; runPipeline's DryRun branch
       skips WriteTree). do NOT change the RescueError/CASError construction. RestoreDefault must be BEFORE
       UpdateRefCAS, not after (the §18.4 "before commit" semantics).

- file: pkg/stagecoach/stagecoach.go   (P1.M3.T5.S1 — runPipeline; S1 EDITS this additively)
  section: `func runPipeline(ctx, deps, cfg, systemExtra string, dryRun bool) (Result, error)` — mirrors
       CommitStaged for DryRun/SystemExtra. The commit path (`!dryRun`) does WriteTree + the loop +
       CommitTree + UpdateRefCAS. Apply the SAME 4 edit points as CommitStaged (SetSnapshot after WriteTree;
       SetCandidate in the loop; RestoreDefault before UpdateRefCAS; ClearSnapshot before success return).
  why: the CLI default action goes through GenerateCommit → CommitStaged (common path), but a library user
       with SystemExtra (or the DryRun-with-commit edge) goes through runPipeline. Both commit paths have
       the SAME post-snapshot signal windows, so both need the wiring (F8). The DryRun branch is UNCHANGED.
  gotcha: runPipeline's loop already updates `candidate` — add `signal.SetCandidate(m)` next to where
       `candidate = m` is set. Do NOT wire the DryRun early-return branch.

- file: cmd/stagecoach/main.go   (P1.M4.T1.S1 — main; S1 EDITS this)
  section: `func main()` — currently `err := cmd.Execute(context.Background())`. The comment on `var
       version`/context says "P1.M4.T2 will replace context.Background() with a signal-aware context."
  why: THIS is the install point. Replace with:
       `ctx, _ := signal.Install(context.Background(), signal.Options{RescueFormat: generate.FormatRescue,
       Out: os.Stderr})` then `err := cmd.Execute(ctx)`. (Discard the *Handler — Active() + the wrappers are
       all that's needed downstream.) Add imports: internal/signal, internal/generate.
  gotcha: keep `cmd.Version = version`, the `exitcode.For(err)` mapping, and the `err.Error() != ""`
       stderr-print UNCHANGED. The handler's os.Exit bypasses exitcode.For (correct — a signal rescue IS
       exit 3, hardcoded). Install must be BEFORE cmd.Execute so the handler is active for the whole run.

- file: internal/generate/generate_test.go   (P1.M3.T4.S2 — READ; reuse test helpers, do NOT edit)
  section: `initRepo(t, dir)`, `commitRaw(t, repo, msg)`, `writeFile(t, repo, name, body)`,
       `stageFile(t, repo, name)`, `headSHA(t, repo)`, `gitOut(t, repo, args...)` + the
       `TestCommitStaged_Timeout` pattern (stub with `SleepMS: 2000`, asserts RescueError + HEAD
       unchanged).
  why: signal_integration_test.go builds the REAL binary (not in-process) so it can't reuse these helpers
       directly — BUT it replicates the SAME repo setup sequence (init, identity, initial commit, write+
       stage a file) via git CLI calls on a t.TempDir(), and uses `stubtest.Build(t)` for the hanging stub
       (SleepMS: 30000). Read this file to copy the exact setup recipe + the timeout test's assertions.
  pattern: for the integration test, replicate initRepo/commitRaw/writeFile/stageFile as local helpers
       using `exec.Command("git", "-C", repo, …)`; build stagecoach via `sync.Once` like stubtest.Build.

- url: https://pkg.go.dev/os/signal   (stdlib — signal.Notify, signal.Stop, signal.Reset)
  section: "Notify", "Stop", "NotifyContext". os.Interrupt == SIGINT (cross-platform). syscall.SIGTERM is
       Unix-deliverable (defined as a const on Windows too, but Windows can't send it — the SIGTERM branch
       simply never fires there; harmless).
  why: the canonical API. `signal.Notify(ch, os.Interrupt, syscall.SIGTERM)` compiles cross-platform (no
       build tag needed for the Notify line). `signal.Stop(ch)` restores the default disposition. Use a
       BUFFERED channel (size 1+) so a signal sent before the goroutine is reading isn't dropped.
  critical: Notify + Stop + a buffered channel + a goroutine — that's the whole interception mechanism.
       Do NOT use signal.Reset on the package-global signals (it affects other goroutines); use
       signal.Stop(ourChannel) to stop delivery to OUR channel only.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                              # module github.com/dustin/stagecoach ; go 1.22 (atomic.Pointer ✓) ; UNCHANGED
cmd/stagecoach/main.go               # P1.M4.T1.S1 — os.Exit(exitcode.For(cmd.Execute(context.Background())))  (S1 EDITS: Install + signal-aware ctx)
internal/
  provider/executor.go              # P1.M2.T5.S1 — Execute (cmd.Start/Wait)  (S1 EDITS: +RegisterChild/ClearChild)
  provider/procgroup_unix.go        # P1.M2.T5.S2 — setupProcessGroup (Setpgid+cmd.Cancel+WaitDelay)  (UNCHANGED — frozen)
  provider/procgroup_windows.go     # P1.M2.T5.S2 — Windows setupProcessGroup  (UNCHANGED — frozen)
  generate/generate.go              # P1.M3.T4.S2 — CommitStaged  (S1 EDITS: +SetSnapshot/SetCandidate/RestoreDefault/ClearSnapshot)
  generate/rescue.go                # P1.M3.T3.S1 — FormatRescue  (UNCHANGED — frozen; consumed via callback)
  generate/generate_test.go         # P1.M3.T4.S2 — test helpers + Timeout pattern  (READ; reuse recipe)
  stubtest/stubtest.go              # P1.M3.T4.S1 — Build(t) stubagent + Manifest(Options{SleepMS})  (READ; reuse)
  cmd/{root,config,providers,default_action}.go  # P1.M4.T1 siblings  (UNCHANGED — parallel; do NOT touch)
  exitcode/exitcode.go              # P1.M4.T1.S1 — For/New/ExitError  (UNCHANGED — handler bypasses it via os.Exit)
pkg/stagecoach/stagecoach.go          # P1.M3.T5.S1 — GenerateCommit + runPipeline  (S1 EDITS: runPipeline commit-path wiring)
Makefile                            # build/test(-race)/coverage/lint/clean  (UNCHANGED)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/signal/signal.go                  # NEW — Handler, Install, Active, nil-safe wrappers, goroutine.
internal/signal/signal_unix.go             # NEW — //go:build !windows: KillProcessGroup + exitCodeForSignal.
internal/signal/signal_windows.go          # NEW — //go:build windows: KillProcessGroup + exitCodeForSignal.
internal/signal/signal_test.go             # NEW — in-process unit tests (injectable Kill/Exit/Out/RescueFormat).
internal/signal/signal_integration_test.go # NEW — //go:build !windows: subprocess SIGINT→rescue→exit 3.
internal/provider/executor.go              # EDIT — +signal.RegisterChild(pid) / defer signal.ClearChild() (+ import).
internal/generate/generate.go              # EDIT — +SetSnapshot/SetCandidate/RestoreDefault/ClearSnapshot in CommitStaged.
pkg/stagecoach/stagecoach.go                 # EDIT — identical wiring in runPipeline (commit path only).
cmd/stagecoach/main.go                      # EDIT — Install handler + signal-aware ctx (+ imports).
# All other files UNCHANGED. root.go/config.go/providers.go/default_action.go UNCHANGED.
# procgroup_*.go / rescue.go / exitcode.go UNCHANGED (frozen / consumed-only).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the signal↔generate cycle, F3): signal MUST NOT import internal/generate. FormatRescue lives
// in generate, but generate calls signal.SetSnapshot (generate→signal). If signal also imported generate
// (for FormatRescue) that's a cycle. FIX: pass FormatRescue to the handler as Options.RescueFormat — a
// func value — wired in main.go (which imports both). signal stays stdlib-only (no stagecoach imports).

// CRITICAL (the child does NOT receive terminal Ctrl-C, FINDING 8 / go_ecosystem_patterns §3.5): because
// the executor sets SysProcAttr.Setpgid=true, the child is in its OWN process group. The terminal sends
// SIGINT to STAGECOACH's process group only — the child never sees it. Stagecoach MUST forward via
// syscall.Kill(-pid, sig) (Unix) / GenerateConsoleCtrlEvent (Windows). Cancelling the ctx ALSO works (the
// executor's cmd.Cancel does exactly this Kill), but we forward directly too for immediacy + contract.

// CRITICAL (os.Exit in the handler ⇒ subprocess integration test, F5): the handler calls os.Exit(3) on a
// post-snapshot signal. That kills the test process, so the rescue+exit behavior CANNOT be tested
// in-process. Use injectable Options.Exit (records the code) for UNIT tests of the logic, and a SUBPROCESS
// integration test (build the real binary, hang a stub, SIGINT, assert exit 3) for the real behavior.

// CRITICAL (RestoreDefault BEFORE update-ref, not after, F7/§18.4 step 3): call signal.RestoreDefault()
// on its OWN line immediately before deps.Git.UpdateRefCAS(...). After it, a signal uses Go's default
// disposition (exit), NOT rescue — intentional (the commit is essentially done). This mirrors commit-pi's
// `trap - INT TERM` immediately before `git commit`. Do NOT place it after UpdateRefCAS.

// GOTCHA (KillProcessGroup is NEW — the work item names a function that doesn't exist yet, F2): P1.M2.T5.S2
// shipped setupProcessGroup(cmd), with the kill logic INSIDE cmd.Cancel. There is no standalone
// KillProcessGroup. This task CREATES it (internal/signal/signal_{unix,windows}.go). Do NOT try to call a
// pre-existing one.

// GOTCHA (KillProcessGroup pid sign convention): caller passes the POSITIVE child PID. Unix impl negates
// internally (syscall.Kill(-pid, sig)). Windows impl uses pid as-is (GenerateConsoleCtrlEvent group id).
// Do NOT negate at the call site.

// GOTCHA (FormatRescue has NO trailing newline): the handler MUST fmt.Fprintln(opts.Out, msg) (adds \n),
// NOT fmt.Fprint. (rescue.go deliberately omits the trailing newline; the CLI layer adds it.)

// GOTCHA (defer signal.ClearChild() in Execute): place it immediately after the successful cmd.Start so
// the PID is cleared whether Wait succeeds or fails. A stale registered PID would make a LATER signal kill
// a RECYCLED pid (kill the wrong process) — ClearChild prevents that.

// GOTCHA (buffered signal channel): make(chan os.Signal, 1) (or larger). An UNbuffered channel drops a
// signal delivered while the goroutine isn't blocked on receive — the user's Ctrl-C would be silently lost.

// GOTCHA (singleton + atomic.Pointer): `var active atomic.Pointer[Handler]` (Go 1.19+; we're 1.22). Load/
// Store are atomic. The nil-safe wrappers do `if h := active.Load(); h != nil { h.method(...) }`. When no
// handler is installed (library use of pkg/stagecoach), every wrapper is a no-op — Execute/CommitStaged call
// them unconditionally with no call-site nil-check.

// GOTCHA (do NOT change the executor's error contract): Execute returns ctx.Err() FIRST on Wait failure
// (DeadlineExceeded for timeout, Canceled for signal/parent-cancel). The signal handler cancelling the ctx
// makes Execute return context.Canceled → CommitStaged wraps it in *RescueError{Kind: ErrRescue}. That
// normal-flow rescue is REDUNDANT with the handler's direct rescue (the handler os.Exits first), but it's
// harmless and stays as the path for NON-signal cancels. Do NOT remove it.

// GOTCHA (Windows SIGTERM is a no-op path): signal.Notify(ch, os.Interrupt, syscall.SIGTERM) compiles on
// Windows, but Windows cannot deliver SIGTERM. The SIGTERM branch of the handler simply never fires on
// Windows — only SIGINT (Ctrl-C) does. This is expected; do NOT add build tags to the Notify line.

// GOTCHA (provider→signal import is one-way and fine): Execute importing internal/signal for
// RegisterChild/ClearChild does NOT create a cycle (signal imports no stagecoach package). procgroup_*.go is
// NOT changed (its cmd.Cancel stays inline syscall.Kill — frozen). The kill one-liner is intentionally
// duplicated across the ctx-cancel path (executor) and the signal-forward path (handler); both idempotent.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/signal/signal.go
package signal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

// Options configures Install. All fields are OPTIONAL — zero values are defaulted in Install. The
// injectable seams (Kill, Exit, RescueFormat, Out) exist so unit tests can exercise the handler logic
// in-process without really killing a process or exiting (see F5).
type Options struct {
	// RescueFormat assembles the §18.3 rescue message. Wired in main.go to generate.FormatRescue.
	// If nil, Install substitutes a minimal base formatter (the handler still prints SOMETHING).
	RescueFormat func(treeSHA, parentSHA, candidate string) string

	// Out is where the rescue message is written (default os.Stderr — matches default_action.go).
	Out io.Writer

	// Kill forwards a signal to a child process group (default KillProcessGroup). Tests inject a recorder.
	Kill func(pid int, sig os.Signal) error

	// Exit terminates the process (default os.Exit). Tests inject a recorder so os.Exit doesn't kill them.
	Exit func(int)
}

// Handler is the installed signal handler (singleton — see F4). Created by Install; accessed elsewhere
// via the nil-safe package wrappers (RegisterChild/SetSnapshot/RestoreDefault/…), NOT by holding the ptr.
type Handler struct {
	opts   Options
	ch     chan os.Signal   // buffered; signal.Notify delivers SIGINT/SIGTERM here
	cancel context.CancelFunc // cancels the signal-aware ctx (→ Execute unwinds + cmd.Cancel kills group)

	childPID atomic.Int64 // registered child PID (0 = none); Setpgid ⇒ PGID==PID, so Kill(pid) ⇒ whole group

	mu           sync.Mutex
	snapTree     string // "" = no snapshot armed (pre-snapshot signal → exit 130, no rescue)
	snapParent   string // parentSHA ("" on root commit — FormatRescue omits -p)
	snapCandidate string // last parsed message (for the §18.3 candidate note); "" if none

	stopped atomic.Bool // RestoreDefault sets this; goroutine exits after draining
}

// active is the process-global singleton (signals are process-global — see F4). nil when no handler is
// installed (library use of pkg/stagecoach); all package wrappers are nil-safe then.
var active atomic.Pointer[Handler]

// Install sets up SIGINT/SIGTERM interception and returns a context cancelled on signal. Stores the
// handler in `active` so the package wrappers (RegisterChild/SetSnapshot/…) reach it. Call ONCE in main,
// BEFORE cmd.Execute. opts fields are defaulted if zero.
func Install(parent context.Context, opts Options) (context.Context, *Handler) {
	// default the injectable seams
	if opts.RescueFormat == nil {
		opts.RescueFormat = func(tree, parent, cand string) string {
			return "❌ Commit generation failed.\nTree ID: " + tree // minimal fallback (main wires the real one)
		}
	}
	if opts.Out == nil {
		opts.Out = os.Stderr
	}
	if opts.Kill == nil {
		opts.Kill = KillProcessGroup // build-tag func (signal_unix.go / signal_windows.go)
	}
	if opts.Exit == nil {
		opts.Exit = os.Exit
	}

	ctx, cancel := context.WithCancel(parent)
	h := &Handler{opts: opts, cancel: cancel, ch: make(chan os.Signal, 1)}
	signal.Notify(h.ch, os.Interrupt, syscall.SIGTERM) // SIGTERM is a no-op path on Windows (harmless)
	active.Store(h)
	go h.run()
	return ctx, h
}

// Active returns the installed handler, or nil if Install was never called (library use).
func Active() *Handler { return active.Load() }

// run is the handler goroutine. One signal ⇒ forward-to-group → cancel → rescue-or-exit. (v1 exits on
// the first signal; the double-Ctrl-C force-exit polish of go_ecosystem_patterns §4.4 is future work.)
func (h *Handler) run() {
	for sig := range h.ch {
		// 1. Forward the signal to the child's process group (if one is registered). Belt-and-suspenders
		//    with the executor's cmd.Cancel (which also SIGTERMs the group on the ctx cancel below).
		if pid := h.childPID.Load(); pid > 0 {
			_ = h.opts.Kill(int(pid), sig) // -pid handled inside KillProcessGroup (Unix) / pid as-is (Windows)
		}
		// 2. Cancel the signal-aware ctx → Execute returns context.Canceled → CommitStaged unwinds.
		h.cancel()
		// 3. Rescue (snapshot armed) or plain exit (pre-snapshot). Snapshot read under the lock.
		h.mu.Lock()
		tree, parent, cand := h.snapTree, h.snapParent, h.snapCandidate
		h.mu.Unlock()
		if tree != "" {
			fmt.Fprintln(h.opts.Out, h.opts.RescueFormat(tree, parent, cand)) // Fprintln adds the trailing \n
			h.opts.Exit(3)                                                    // §18.2: SIGINT/SIGTERM post-snapshot → exit 3
			return
		}
		h.opts.Exit(exitCodeForSignal(sig)) // 130 SIGINT / 143 SIGTERM — "just exit" (§18.4 step 2)
		return
	}
}

// ---- Nil-safe package wrappers (called by provider.Execute / generate.CommitStaged / runPipeline) ----
// Each no-ops when no handler is installed (Active()==nil), so callers need no nil-checks.

// RegisterChild records the running child's PID so a signal can be forwarded to its process group.
// Called by provider.Execute after cmd.Start. Setpgid ⇒ PGID==PID, so Kill(pid) addresses the whole tree.
func RegisterChild(pid int) {
	if h := active.Load(); h != nil {
		h.childPID.Store(int64(pid))
	}
}

// ClearChild clears the registered child PID. Called by provider.Execute (deferred) so a later signal
// can't kill a recycled PID. Idempotent.
func ClearChild() {
	if h := active.Load(); h != nil {
		h.childPID.Store(0)
	}
}

// SetSnapshot arms the rescue path: after this, a signal prints FormatRescue + exits 3. Called by
// generate.CommitStaged / runPipeline immediately after WriteTree succeeds.
func SetSnapshot(treeSHA, parentSHA, candidate string) {
	if h := active.Load(); h != nil {
		h.mu.Lock()
		h.snapTree, h.snapParent, h.snapCandidate = treeSHA, parentSHA, candidate
		h.mu.Unlock()
	}
}

// SetCandidate updates the candidate message (for the §18.3 note) without touching tree/parent. Called
// in the generate loop after each parsed message.
func SetCandidate(candidate string) {
	if h := active.Load(); h != nil {
		h.mu.Lock()
		h.snapCandidate = candidate
		h.mu.Unlock()
	}
}

// ClearSnapshot disarms the rescue path. Belt-and-suspenders on success (RestoreDefault already neuters
// the handler before update-ref). Called before the success return.
func ClearSnapshot() {
	if h := active.Load(); h != nil {
		h.mu.Lock()
		h.snapTree, h.snapParent, h.snapCandidate = "", "", ""
		h.mu.Unlock()
	}
}

// RestoreDefault stops signal delivery to our channel, restoring Go's default disposition. Called right
// before update-ref (§18.4 step 3) so a last-instant Ctrl-C isn't mistaken for a failure. Idempotent.
func RestoreDefault() {
	if h := active.Load(); h != nil {
		if h.stopped.CompareAndSwap(false, true) {
			signal.Stop(h.ch) // stop delivering SIGINT/SIGTERM to h.ch → default disposition restored
			close(h.ch)       // let the goroutine's `range` exit
		}
	}
}
```

```go
// internal/signal/signal_unix.go
//go:build !windows

package signal

import (
	"os"
	"syscall"
)

// KillProcessGroup sends sig to the child's entire process group. The caller passes the POSITIVE child
// PID; Setpgid ⇒ PGID==PID, so -pid addresses the whole tree (child + all grandchildren). This is the
// SAME idiom as procgroup_unix.go's cmd.Cancel (F2); duplicated intentionally so procgroup_*.go stays
// frozen. The grace-then-SIGKILL escalation is the executor's cmd.WaitDelay (3s) — NOT our job.
func KillProcessGroup(pid int, sig os.Signal) error {
	return syscall.Kill(-pid, sig.(syscall.Signal)) // -pid ⇒ whole group
}

// exitCodeForSignal returns the conventional 128+signum exit code for an aborted run (§18.4 step 2
// "else just exit"). Used only for PRE-snapshot signals (post-snapshot is hardcoded exit 3).
func exitCodeForSignal(sig os.Signal) int {
	switch sig {
	case os.Interrupt, syscall.SIGINT:
		return 130 // 128 + 2
	case syscall.SIGTERM:
		return 143 // 128 + 15
	default:
		return 1
	}
}
```

```go
// internal/signal/signal_windows.go
//go:build windows

package signal

import (
	"os"
	"syscall"
)

// procGenerateConsoleCtrlEvent resolves kernel32!GenerateConsoleCtrlEvent lazily (stdlib-only — no
// golang.org/x/sys dependency, matching procgroup_windows.go). CTRL_BREAK (not CTRL_C) because CTRL_C
// can't be limited to one console process group.
var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")

// KillProcessGroup is the Windows analog (FINDING 10): CREATE_NEW_PROCESS_GROUP ⇒ the child's PID is its
// console process-group id, so GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, pid) signals the whole group.
// sig is effectively ignored (always CTRL_BREAK for graceful; force-escalation is the executor's
// WaitDelay/TerminateProcess). The caller passes the POSITIVE pid (do NOT negate — Windows has no -pid).
func KillProcessGroup(pid int, sig os.Signal) error {
	r1, _, err := procGenerateConsoleCtrlEvent.Call(
		uintptr(syscall.CTRL_BREAK_EVENT),
		uintptr(pid),
	)
	if r1 == 0 {
		return err // non-fatal: the executor's WaitDelay escalation handles a stubborn child
	}
	return nil
}

// exitCodeForSignal (Windows). SIGINT via Ctrl-C → 130. SIGTERM is not deliverable on Windows but is
// defined as a const; map it to 143 for consistency with Unix (the branch won't fire in practice).
func exitCodeForSignal(sig os.Signal) int {
	switch sig {
	case os.Interrupt, syscall.SIGINT:
		return 130
	case syscall.SIGTERM:
		return 143
	default:
		return 1
	}
}
```

```go
// internal/provider/executor.go   (EDIT — additive; the ONLY change to Execute)
// … existing imports …
import "github.com/dustin/stagecoach/internal/signal"
// …

func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration) (stdout string, stderr string, err error) {
	// … existing timeout + cmd build + setupProcessGroup(cmd) …
	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("provider %q: start: %w", spec.Command, err)
	}
	signal.RegisterChild(cmd.Process.Pid) // NEW — arm signal forwarding (Setpgid ⇒ PGID==PID)
	defer signal.ClearChild()             // NEW — clear before return so a later signal can't kill a recycled PID
	// … existing cmd.Wait() + error-contract return …
}
```

```go
// internal/generate/generate.go   (EDIT — 4 additive call sites in CommitStaged)
// … existing imports …
import "github.com/dustin/stagecoach/internal/signal"
// …
func CommitStaged(ctx context.Context, deps Deps, cfg config.Config) (Result, error) {
	// … steps 1–2 …
	treeSHA, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return Result{}, err
	}
	signal.SetSnapshot(treeSHA, parentSHA, "") // NEW — arm rescue (§18.4)
	// … step 4 (sysPrompt, recent) …
	// … step 5 loop …
	for attempt := 0; …; attempt++ {
		// … payload, render, Execute …
		m, ok, _ := provider.ParseOutput(out, deps.Manifest)
		if !ok {
			parseFail = true; candidate = m; continue
		}
		signal.SetCandidate(m) // NEW — keep the §18.3 candidate note current
		parseFail = false
		// … subject/dedupe …
	}
	// … step 7 CommitTree …
	signal.RestoreDefault() // NEW — §18.4 step 3: default disposition for the update-ref window
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
		// … existing CAS handling UNCHANGED …
	}
	// … step 9 DiffTree …
	signal.ClearSnapshot() // NEW — belt-and-suspenders disarm on success
	return Result{…}, nil
}
```

```go
// cmd/stagecoach/main.go   (EDIT — install the handler)
import (
	"context"
	"fmt"
	"os"

	"github.com/dustin/stagecoach/internal/cmd"
	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/generate"
	"github.com/dustin/stagecoach/internal/signal"
)

func main() {
	cmd.Version = version
	ctx, _ := signal.Install(context.Background(), signal.Options{
		RescueFormat: generate.FormatRescue, // §18.3 message (callback — avoids signal→generate import)
		Out:          os.Stderr,             // rescue to stderr (matches default_action.go's handleGenError)
	})
	err := cmd.Execute(ctx)
	code := exitcode.For(err)
	if err != nil && err.Error() != "" {
		fmt.Fprintf(os.Stderr, "stagecoach: %v\n", err)
	}
	os.Exit(code)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/signal/signal.go (Handler + Install/Active + wrappers + goroutine)
  - FILE: NEW internal/signal/signal.go. PACKAGE: `package signal`. Follow "Data models" skeleton.
  - IMPORTS (stdlib ONLY — no stagecoach): context, fmt, io, os, os/signal, sync, sync/atomic, syscall.
  - DEFINE: type Options (RescueFormat/Out/Kill/Exit); type Handler (opts/ch/cancel/childPID atomic.Int64/
      mu+snap fields/stopped atomic.Bool); var active atomic.Pointer[Handler]; func Install(parent, opts)
      (ctx, *Handler); func Active() *Handler; func (h *Handler) run(); the nil-safe wrappers
      RegisterChild/ClearChild/SetSnapshot/SetCandidate/ClearSnapshot/RestoreDefault.
  - GOTCHA: signal imports NO stagecoach package (F3). RescueFormat is a callback. Kill defaults to
      KillProcessGroup (defined in the build-tag files Task 2/3 — same package, always compiled).
  - GOTCHA: signal.Notify(h.ch, os.Interrupt, syscall.SIGTERM) — NO build tag (compiles on Windows; the
      SIGTERM branch just never fires there). Buffered channel (size 1).
  - GOTCHA: run() does forward → cancel → (snapshot? rescue+Exit(3) : Exit(exitCodeForSignal)). Uses
      fmt.Fprintln (FormatRescue has NO trailing newline). RestoreDefault does signal.Stop + close(ch).

Task 2: CREATE internal/signal/signal_unix.go (//go:build !windows)
  - FILE: NEW. PACKAGE signal. BUILD TAG: `//go:build !windows` (first line).
  - DEFINE: func KillProcessGroup(pid int, sig os.Signal) error { return syscall.Kill(-pid,
      sig.(syscall.Signal)) }; func exitCodeForSignal(sig os.Signal) int (SIGINT→130, SIGTERM→143, else 1).
  - GOTCHA: caller passes POSITIVE pid; negate internally (-pid ⇒ whole group). Mirror procgroup_unix.go's
      cmd.Cancel idiom EXACTLY (read it first). Do NOT refactor procgroup_unix.go.

Task 3: CREATE internal/signal/signal_windows.go (//go:build windows)
  - FILE: NEW. PACKAGE signal. BUILD TAG: `//go:build windows` (first line).
  - DEFINE: var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc(...); func
      KillProcessGroup(pid int, sig os.Signal) error (GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, pid); r1==0
      ⇒ return err); func exitCodeForSignal (130/143/1).
  - GOTCHA: copy the LazyProc + Call + r1==0 error handling VERBATIM from procgroup_windows.go. POSITIVE
      pid (do NOT negate). sig ignored (always CTRL_BREAK). Verify it compiles via `GOOS=windows go vet
      ./internal/signal/` (no Windows host needed).

Task 4: CREATE internal/signal/signal_test.go (in-process unit tests)
  - FILE: NEW. PACKAGE signal (`package signal` — white-box; access unexported fields).
  - PATTERN: each test installs a Handler with INJECTED fakes so os.Exit/real-kill never fire:
      * opts.Kill = func(pid int, sig os.Signal) error { killedPid = pid; killedSig = sig; return nil }
      * opts.Exit = func(code int) { exitCode = code; exited = true } (record, don't exit)
      * opts.Out = &buf (*bytes.Buffer); opts.RescueFormat = generate.FormatRescue (or a stub).
    Install via a LOCAL constructor (or call Install then active.Store(nil) in t.Cleanup to reset the
    singleton — CRITICAL: reset `active` in t.Cleanup or tests poison each other + trip -race). Provide a
    test helper `installTestHandler(t, opts) *Handler` that t.Cleanup-resets active.
  - CASES:
      * TestHandler_ForwardsToChildGroup: RegisterChild(1234); deliver a signal (send on h.ch or use
        os.Process.Signal(self)? — simpler: close/h.ch <- syscall.SIGINT after Install, with a fake Kill
        recording pid+sig); assert Kill called with (1234, SIGINT) and ctx cancelled.
        NOTE: to deliver a signal deterministically in-process WITHOUT os.Exit, the fake Exit records the
        code; send the signal via `h.ch <- syscall.SIGINT` (the channel is buffered size 1 — but the goroutine
        reads it; use a tiny poll/wait). Alternatively call a test-only `h.handle(sig)` extracted from run().
        RECOMMENDED: factor the per-signal body into `func (h *Handler) handle(sig os.Signal)` and have run()
        call it; tests call h.handle directly (no goroutine timing). Do this.
      * TestHandler_RescueOnSignalWithSnapshot: SetSnapshot("abc","def","cand"); h.handle(SIGINT); assert
        exitCode==3 AND buf contains FormatRescue("abc","def","cand") output (the ❌ + Tree ID + commit-tree).
      * TestHandler_Exit130PreSnapshot: (no SetSnapshot); h.handle(SIGINT); assert exitCode==130 AND buf empty.
      * TestHandler_RestoreDefaultStopsForward: RestoreDefault(); then h.handle(SIGINT) — assert Kill NOT
        called (stopped flag short-circuits) OR (if handle checks stopped) exitCode reflects default. Define
        the contract: after RestoreDefault, handle is a no-op (the goroutine has exited; default disposition
        applies). Test that stopped==true and a second handle does nothing.
      * TestHandler_NilWrappersNoOp: with active=nil (after reset), RegisterChild/SetSnapshot/etc do not
        panic (nil-safe).
      * TestHandler_SetCandidateUpdates: SetSnapshot("t","p",""); SetCandidate("new"); assert snapCandidate=="new".
      * TestKillProcessGroup_Unix (//go:build !windows): kill a real child (e.g. `exec.Command("sleep","30")`
        with Setpgid; start; KillProcessGroup(pid, SIGTERM); assert it exits). Or assert
        syscall.Kill(-pid,SIGTERM) semantics with a stub. Keep it light.
  - COVERAGE: forward-kill, rescue-print+exit3, exit130-pre-snapshot, RestoreDefault, nil-safety,
      SetCandidate. Use the injectable seams exclusively (no real os.Exit).

Task 5: CREATE internal/signal/signal_integration_test.go (//go:build !windows — real subprocess)
  - FILE: NEW. PACKAGE signal. BUILD TAG: `//go:build !windows`. Skips short: `if testing.Short()
      { t.Skip(…) }`.
  - GOAL: drive the REAL stagecoach binary as a subprocess, send SIGINT mid-generation, assert exit 3 +
      rescue printed + child killed + HEAD unchanged. (The os.Exit path can ONLY be tested this way — F5.)
  - BUILD: cache the stagecoach binary via sync.Once (mirror stubtest.Build): `exec.Command("go","build",
      "-o", bin, "./cmd/stagecoach")`. Build the stub via stubtest.Build(t).
  - SETUP (replicate generate_test.go's initRepo recipe via git CLI on t.TempDir()):
      git init; git config user.email/name; commit an initial file; write+`git add` a second file (so
      WriteTree produces a non-empty treeSHA). Write a config TOML to a temp path:
        [defaults]
        provider = "stub"
        [provider.stub]
        command = "<stubpath>"   # the stubagent binary from stubtest.Build
        prompt_delivery = "stdin"
        output = "raw"
  - ENV for the stagecoach subprocess: os.Environ() + "STAGECOACH_STUB_SLEEP_MS=30000" (stub hangs after
      draining stdin) + HOME/XDG isolated to a temp dir (so no real global config interferes) +
      "GIT_CONFIG_NOSYSTEM=1". (The stub inherits STAGECOACH_STUB_SLEEP_MS because the manifest has no Env
      override → cmd.Env is nil → child inherits parent env. Verify by reading executor.go.)
  - RUN: cmd := exec.Command(bin, "--config", cfgPath); cmd.Dir = repo; cmd.Env = …; capture
      stdout/stderr via pipes (or CombinedOutput). cmd.Start().
  - TIMING: poll until the stub child is running (best-effort: sleep 200ms, then retry up to ~2s). A
      simple robust approach: sleep 800ms (snapshot+Execute guaranteed started; 800ms ≪ 30s stub sleep).
      Optional: detect the stub PID by scanning cmd.Process.Pid's children (Linux /proc) — but timing is
      sufficient for v1.
  - SIGNAL: cmd.Process.Signal(syscall.SIGINT).
  - ASSERT: err := cmd.Wait(); var ee *exec.ExitError; errors.As(err, &ee); ee.ExitCode() == 3.
      CombinedOutput/stderr contains "Commit generation failed" AND "Tree ID:" AND "git commit-tree" AND
      "update-ref HEAD". `git -C repo rev-parse HEAD` UNCHANGED vs before. The stub child is dead (send
      SIGCONT/0 to its PID → ESRCH, or it's gone from the process list). The snapshot tree object exists:
      `git -C repo cat-file -t <treeSHA>` → "tree" (parse the Tree ID from stderr).
  - GOTCHA: the signal MUST arrive AFTER WriteTree (snapshot armed) or the handler exits 130 (no rescue)
      and the test fails. Generous sleep + the 30s stub window make this reliable. If flaky, increase the
      sleep or add child-PID detection.

Task 6: EDIT internal/provider/executor.go (register/clear child PID)
  - FILE: internal/provider/executor.go. ADD import "github.com/dustin/stagecoach/internal/signal".
  - INSERT (2 lines) immediately after the `if err := cmd.Start(); err != nil { … }` block, before
      `if werr := cmd.Wait(); werr != nil`:
        signal.RegisterChild(cmd.Process.Pid)
        defer signal.ClearChild()
  - GOTCHA: cmd.Process is guaranteed non-nil there (same guarantee procgroup_unix.go's cmd.Cancel relies
      on). The defer runs on EVERY return path (success/error). Do NOT touch setupProcessGroup/cmd.Cancel/
      WaitDelay/error-contract. Verify with `git diff internal/provider/executor.go` showing ONLY the 2
      lines + import.

Task 7: EDIT internal/generate/generate.go (CommitStaged wiring — 4 call sites)
  - FILE: internal/generate/generate.go. ADD import "github.com/dustin/stagecoach/internal/signal".
  - 4 ADDITIVE call sites in CommitStaged (see skeleton): (a) signal.SetSnapshot(treeSHA, parentSHA, "")
      right after WriteTree succeeds; (b) signal.SetCandidate(m) after a successful ParseOutput (before
      ExtractSubject); (c) signal.RestoreDefault() on its own line immediately before UpdateRefCAS; (d)
      signal.ClearSnapshot() before the success return.
  - GOTCHA: RestoreDefault BEFORE UpdateRefCAS (not after). Do NOT wire DryRun (CommitStaged has none).
      Do NOT change RescueError/CASError construction. `git diff` shows ONLY the 4 lines + import.

Task 8: EDIT pkg/stagecoach/stagecoach.go (runPipeline wiring — commit path only)
  - FILE: pkg/stagecoach/stagecoach.go. ADD import "github.com/dustin/stagecoach/internal/signal".
  - Apply the SAME 4 call sites in runPipeline's commit path (the `!dryRun` branch that does WriteTree +
      loop + CommitTree + UpdateRefCAS): SetSnapshot after WriteTree; SetCandidate in the loop (next to
      `candidate = m`); RestoreDefault before UpdateRefCAS; ClearSnapshot before success return.
  - GOTCHA: the DryRun early-return branch is UNCHANGED (no WriteTree → no snapshot). runPipeline mirrors
      CommitStaged — the wiring is identical. `git diff` shows ONLY the 4 lines + import.

Task 9: EDIT cmd/stagecoach/main.go (install handler + signal-aware ctx)
  - FILE: cmd/stagecoach/main.go. ADD imports internal/signal + internal/generate.
  - REPLACE `err := cmd.Execute(context.Background())` with:
        ctx, _ := signal.Install(context.Background(), signal.Options{
            RescueFormat: generate.FormatRescue,
            Out:          os.Stderr,
        })
        err := cmd.Execute(ctx)
  - GOTCHA: keep cmd.Version=version, exitcode.For(err), and the err.Error()!="" stderr-print UNCHANGED.
      Install BEFORE cmd.Execute (handler active for the whole run). Discard the *Handler (Active() +
      wrappers are all that's needed). `git diff` shows ONLY the Install block + imports.

Task 10: VALIDATE (run all gates; fix before declaring done)
  - `make build` → ./bin/stagecoach exists.
  - `go test -race ./internal/signal/ -v` → green (unit + integration). The integration test exits 3 from
      the subprocess (expected); the test asserts it.
  - `go test -race ./...` → green (NO regression — provider/generate/pkg tests unaffected by the additive
      signal calls; they don't Install a handler so the wrappers are no-ops).
  - `GOOS=windows go vet ./internal/signal/` → clean (verifies the Windows build file compiles).
  - `go vet ./...` clean; `gofmt -l internal/signal/ internal/provider/ internal/generate/ pkg/stagecoach/
      cmd/stagecoach/` empty.
  - `git status` shows: 5 new internal/signal/* + modified executor.go/generate.go/stagecoach.go/main.go.
      Verify UNCHANGED: `git diff internal/cmd/root.go internal/cmd/config.go internal/cmd/providers.go
      internal/cmd/default_action.go internal/provider/procgroup_unix.go internal/provider/procgroup_windows.go
      internal/generate/rescue.go internal/exitcode/exitcode.go` = ALL empty.
```

### Implementation Patterns & Key Details

```go
// PATTERN: factor the per-signal body into a testable method (Task 4 recommends this).
//   func (h *Handler) run() { for sig := range h.ch { h.handle(sig); /* v1: return after first */ } }
//   func (h *Handler) handle(sig os.Signal) { … forward → cancel → rescue/exit … }
// Tests call h.handle(sig) directly (no goroutine timing, no real os.Exit via injected Exit).

// PATTERN: nil-safe singleton wrappers (F4). Callers (Execute/CommitStaged/runPipeline) call the package
//   funcs unconditionally; they no-op when Active()==nil (library use). No call-site nil-checks.
//   func SetSnapshot(tree, parent, cand string) { if h := active.Load(); h != nil { h.mu.Lock(); …; h.mu.Unlock() } }

// PATTERN: RestoreDefault before update-ref (§18.4 step 3 / F7).
//   signal.RestoreDefault()              // ← own line, IMMEDIATELY before:
//   if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil { … }

// PATTERN: install once in main, pass ctx down (the ctx already flows main→cmd.Execute→rootCmd.SetContext
//   →cmd.Context()→GenerateCommit→CommitStaged→Execute — no new plumbing needed).
//   ctx, _ := signal.Install(context.Background(), signal.Options{RescueFormat: generate.FormatRescue, Out: os.Stderr})
//   err := cmd.Execute(ctx)

// GOTCHA: the rescue reaches the handler via a CALLBACK, not an import (F3). signal never imports
//   generate. main.go (which imports both) wires Options.RescueFormat = generate.FormatRescue.

// GOTCHA: cmd.Process.Pid is valid after cmd.Start (guaranteed non-nil) — same guarantee the executor's
//   cmd.Cancel relies on. RegisterChild there; defer ClearChild.

// GOTCHA: a buffered signal channel (size 1) — an unbuffered one drops a signal delivered while the
//   goroutine isn't reading (user's Ctrl-C silently lost).

// GOTCHA: KillProcessGroup pid sign — caller passes POSITIVE pid; Unix negates internally; Windows uses
//   pid as-is. Do NOT negate at the call site.
```

### Integration Points

```yaml
CONTEXT (main.go → root.go → Execute → CommitStaged → Execute — ALREADY flows, no new plumbing):
  - flow: "main.go calls signal.Install(parentCtx) → returns signalAwareCtx → cmd.Execute(ctx) →
    rootCmd.SetContext(ctx) (root.go line 112, UNCHANGED) → PersistentPreRunE/cmd.Context() →
    default_action.runDefault reads cmd.Context() → stagecoach.GenerateCommit(ctx,…) → CommitStaged(ctx,…)
    → provider.Execute(ctx,…). The handler's cancel() cancels THIS ctx everywhere. No new ctx plumbing."

CHILD.PID (provider.Execute → signal.RegisterChild):
  - register: "Execute, after cmd.Start succeeds: signal.RegisterChild(cmd.Process.Pid). Setpgid ⇒
    PGID==PID, so Kill(pid) addresses the whole child tree. defer signal.ClearChild() clears it before
    return (prevents killing a recycled PID on a later signal)."
  - gotcha: "do NOT touch setupProcessGroup/cmd.Cancel/WaitDelay (frozen). The executor's ctx-cancel
    kill path coexists with the handler's forward-kill (both SIGTERM the group — idempotent)."

SNAPSHOT.STATE (generate.CommitStaged + pkg.runPipeline → signal.SetSnapshot/SetCandidate):
  - arm: "immediately after WriteTree succeeds: signal.SetSnapshot(treeSHA, parentSHA, ''). A signal now
    prints FormatRescue + exits 3."
  - candidate: "in the generate loop after a successful ParseOutput: signal.SetCandidate(m). Keeps the
    §18.3 candidate note current for the signal-rescue message."
  - restore: "signal.RestoreDefault() on its own line IMMEDIATELY before UpdateRefCAS (§18.4 step 3)."
  - clear: "signal.ClearSnapshot() before the success return (belt-and-suspenders; RestoreDefault already
    neutered the handler for the update-ref window)."

RESCUE.MESSAGE (handler → generate.FormatRescue via callback):
  - callback: "main.go wires Options.RescueFormat = generate.FormatRescue. The handler calls
    opts.RescueFormat(snapshot.tree, snapshot.parent, snapshot.candidate) and fmt.Fprintln's it to
    opts.Out (os.Stderr). FormatRescue is FROZEN (P1.M3.T3.S1) — call it, don't reimplement."

EXIT.CODES (handler bypasses exitcode.For):
  - signal rescue: "handler calls os.Exit(3) directly (§18.2). Bypasses exitcode.For — correct, a signal
    rescue IS exit 3. exitcode.For is UNCHANGED and still handles the NON-signal rescue path
    (RescueError → 3, timeout → 124, CAS → 1) for normal failures."
  - pre-snapshot: "handler calls os.Exit(exitCodeForSignal(sig)) = 130 (SIGINT) / 143 (SIGTERM)."
  - default-action success: "UNCHANGED — exit 0 via exitcode.For(nil)."
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After each file — fix before proceeding
gofmt -w internal/signal/ internal/provider/executor.go internal/generate/generate.go pkg/stagecoach/stagecoach.go cmd/stagecoach/main.go
go vet ./internal/signal/ ./internal/provider/ ./internal/generate/ ./pkg/stagecoach/ ./cmd/stagecoach/

# Cross-platform compile check (no Windows host needed)
GOOS=windows go vet ./internal/signal/      # verifies signal_windows.go + the !windows-tagged parts exclude correctly
GOOS=linux   go vet ./internal/signal/      # verifies signal_unix.go

# Project-wide
go vet ./...
gofmt -l internal/ pkg/ cmd/

# Expected: Zero errors. The build-tag split (signal_unix.go vs signal_windows.go) must vet clean on BOTH
# GOOS values. If GOOS=windows vet fails, the Windows KillProcessGroup/LazyProc idiom is wrong — re-read
# procgroup_windows.go and copy it verbatim.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The signal package unit tests (injectable Kill/Exit — no real os.Exit)
go test -race ./internal/signal/ -run 'TestHandler|TestKillProcessGroup' -v

# The integration test (subprocess; may be slow — builds the real binary + hangs a stub 30s-less)
go test -race ./internal/signal/ -run TestSignalIntegration -v
# (or all signal tests together:)
go test -race ./internal/signal/ -v

# Confirm the additive wiring didn't regress the dependent packages (they don't Install a handler,
# so the wrappers are no-ops — these must stay green unchanged)
go test -race ./internal/provider/ -v
go test -race ./internal/generate/ -v
go test -race ./pkg/stagecoach/ -v
go test -race ./internal/cmd/ -v

# Expected: All green. If a dependent-package test regressed, the signal calls were placed wrong
# (e.g. RestoreDefault after UpdateRefCAS, or ClearSnapshot missing causing a test's ctx to be cancelled).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the real binary
make build

# Set up a throwaway repo with a slow/hanging fake agent and exercise the FULL signal path by hand.
# (The automated equivalent is signal_integration_test.go's scenario.)
TMP=$(mktemp -d) && cd "$TMP"
git init -q && git config user.email t@t && git config user.name t
printf 'a\n' > f.txt && git add f.txt && git commit -qm init
printf 'b\n' > f.txt && git add f.txt          # staged change → WriteTree will produce a treeSHA

# Fake "agent" that hangs forever (stands in for a real slow CLI agent):
cat > /usr/local/bin/fakeagent 2>/dev/null <<'EOF'   # or any dir on $PATH; or use the built stubagent
#!/bin/sh
cat > /dev/null   # drain the prompt (the executor pipes stdin)
sleep 3600        # hang — simulates a slow/stuck agent
EOF
# (better: use cmd/stubagent with STAGECOACH_STUB_SLEEP_MS=3600000)

# Configure stagecoach to use it, then run in the FOREGROUND and press Ctrl-C mid-generation:
cat > .stagecoach.toml <<EOF
[defaults]
provider = "stub"
[provider.stub]
command = "$(command -v stubagent || echo /path/to/stubagent)"
prompt_delivery = "stdin"
output = "raw"
EOF
STAGECOACH_STUB_SLEEP_MS=3600000 ./bin/stagecoach --config .stagecoach.toml &
SP=$!
sleep 1              # let the snapshot be taken + the agent start
kill -INT $SP        # ← simulate Ctrl-C
wait $SP; echo "EXIT=$?"   # EXPECT: EXIT=3

# Expected output (to STDERR):
#   ❌ Commit generation failed.
#   ------------------------------------------------------------ (60 '-')
#   Your staged files were safely snapshotted before generation.
#   Tree ID: <sha>
#
#   To commit the originally staged files manually:
#     git commit-tree -p <parent> -m "Your message" <tree> | xargs git update-ref HEAD
#
#   (omit "-p <PARENT_SHA>" if this is the repository's first commit)
#   ------------------------------------------------------------ (60 '-')
# EXPECT: EXIT=3; `git rev-parse HEAD` UNCHANGED; the fakeagent child is dead (ps aux | grep fakeagent).
# The Tree ID is a real git tree object: `git cat-file -t <Tree ID>` → "tree".

# Pre-snapshot Ctrl-C test (signal before WriteTree — e.g. during config/gather): exit 130, no rescue.
# Update-ref-window test: harder to hit by hand (RestoreDefault → default disposition); covered by unit tests.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Orphan-prevention check: confirm NO stubagent/fakeagent grandchildren survive the Ctrl-C.
# (FINDING 8 — the whole point of process-group kill.)
pgrep -P $SP stubagent && echo "ORPHAN SURVIVED (BUG)" || echo "no orphans (correct)"
# Also check grandchildren (a real agent may spawn tool subprocesses): pstree -p $SP should be empty after exit.

# Race-detector confidence (the handler goroutine + main goroutine share childPID/snap via atomics+mutex)
go test -race -count=3 ./internal/signal/    # run 3× to shake out goroutine races

# Library-use regression (pkg/stagecoach with NO handler installed — wrappers must be no-ops)
go test -race ./pkg/stagecoach/ -v            # existing tests must pass UNCHANGED (no Install in them)

# Windows compile + vet (no Windows host needed — cross-vet)
GOOS=windows GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=arm64 go vet ./...

# Expected: no orphans; -race clean across 3 runs; pkg/stagecoach green unchanged; Windows vet clean.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go test -race ./internal/signal/ -v` green (unit + integration).
- [ ] `go test -race ./...` green (NO regression in provider/generate/pkg/cmd).
- [ ] `go vet ./...` clean; `GOOS=windows go vet ./internal/signal/` clean.
- [ ] `gofmt -l internal/signal/ internal/provider/ internal/generate/ pkg/stagecoach/ cmd/stagecoach/` empty.
- [ ] Only the 9 listed files changed (`git status`); root.go/config.go/providers.go/default_action.go/
      procgroup_*.go/rescue.go/exitcode.go UNCHANGED (`git diff` empty for each).

### Feature Validation

- [ ] Ctrl-C mid-generation (post-snapshot) kills the agent tree (no orphan grandchildren), prints the
      exact §18.3 rescue block to STDERR, exits 3, HEAD + staged index byte-for-byte unchanged.
- [ ] Ctrl-C before the snapshot exits 130, prints no rescue.
- [ ] Ctrl-C during/after `update-ref` (post-RestoreDefault) uses the default disposition (no rescue).
- [ ] The rescue message's Tree ID is a real git tree object (`git cat-file -t` → "tree").
- [ ] The candidate note appears in the rescue when a message was parsed before the signal (SetCandidate).
- [ ] Library use of `pkg/stagecoach` WITHOUT `signal.Install` is unchanged (wrappers are no-ops;
      `go test ./pkg/stagecoach/` green).
- [ ] Windows builds + vets clean (KillProcessGroup via GenerateConsoleCtrlEvent).

### Code Quality Validation

- [ ] `internal/signal` imports NO stagecoach package (stdlib-only + the build-tag syscall files).
- [ ] The rescue reaches the handler via the `Options.RescueFormat` callback (no signal→generate import).
- [ ] Injectable seams (`Kill`/`Exit`/`Out`/`RescueFormat`) enable in-process unit tests (no real os.Exit).
- [ ] The singleton (`atomic.Pointer[Handler]`) is reset in test t.Cleanup (no -race poisoning).
- [ ] `RestoreDefault` is placed BEFORE `UpdateRefCAS` (not after) in BOTH CommitStaged and runPipeline.
- [ ] `defer signal.ClearChild()` in Execute clears the PID on every return path.
- [ ] Buffered signal channel (size 1+); `signal.Stop` (not a blanket `signal.Reset`) in RestoreDefault.

### Documentation & Deployment

- [ ] Code is self-documenting (doc comments on Handler/Install/each wrapper + KillProcessGroup explaining
      the Setpgid/-pid/PGID==PID reasoning + the §18.4 step each maps to).
- [ ] The FROZEN upstream (FormatRescue, setupProcessGroup) is CALLED, not duplicated or modified.
- [ ] No new dependencies added to go.mod/go.sum (Windows uses stdlib syscall.LazyProc, not golang.org/x/sys).

---

## Anti-Patterns to Avoid

- ❌ Don't make `internal/signal` import `internal/generate` (cycle — generate imports signal). Pass
  `FormatRescue` as the `Options.RescueFormat` callback; wire it in main.go.
- ❌ Don't refactor `procgroup_*.go` (its `setupProcessGroup` signature is FROZEN). Leave `cmd.Cancel`
  inline; the duplicated kill idiom is intentional (two independent, idempotent kill paths).
- ❌ Don't place `RestoreDefault` AFTER `UpdateRefCAS` (it must be BEFORE — §18.4 step 3 semantics).
- ❌ Don't forget `defer signal.ClearChild()` in Execute (a stale PID → a later signal kills a recycled PID).
- ❌ Don't use an unbuffered signal channel (drops the Ctrl-C if the goroutine isn't reading).
- ❌ Don't `os.Exit` in a unit test (use injectable `Options.Exit`; reserve real os.Exit for the
  subprocess integration test).
- ❌ Don't reimplement the grace/SIGKILL escalation in the handler (that's the executor's `WaitDelay`).
- ❌ Don't touch root.go/config.go/providers.go/default_action.go (P1.M4.T1 siblings, running in parallel).
- ❌ Don't add the DryRun branch of `runPipeline` to the snapshot wiring (DryRun skips WriteTree — no rescue).
- ❌ Don't negate the pid at the `KillProcessGroup` call site (Unix negates internally; Windows uses as-is).
