# P1.M4.T2.S1 — Signal Handler: Design Decisions & Findings

> Companion to `../PRP.md`. These are the non-obvious design calls an implementer must internalize
> before writing code. Every decision is justified against the codebase as it exists TODAY (post
> P1.M3 / mid P1.M4) and the frozen contracts listed in the work item.

## The contract (PRD §18.4 / §18.2 / §9.10 FR45 / FINDING 8)

On SIGINT/SIGTERM:
1. If a child (agent) is running → forward the signal to its **process group** (`Setpgid ⇒ PGID==PID`,
   so `-pid` addresses the whole tree). Grace period then SIGKILL escalation.
2. If the snapshot was taken (`treeSHA != ""`) → run the rescue path (`FormatRescue` + print + exit 3).
   Else → just exit.
3. **Restore the default signal handler BEFORE the final `update-ref`** so a Ctrl-C at the very last
   instant isn't mistaken for a failure (matches commit-pi's `trap - INT TERM` before commit).

Exit codes (§15.4 / §18.2 table): signal post-snapshot → **3** (rescue); pre-snapshot signal →
**130** (128+SIGINT, conventional) — "just exit."

---

## F1 — The child kill is ALREADY wired in the executor (via `cmd.Cancel`)

`internal/provider/procgroup_unix.go` (P1.M2.T5.S2) sets, on every child `*exec.Cmd`:
- `SysProcAttr.Setpgid = true` (child = own process-group leader; PGID==PID)
- `cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }` — fires when
  the **context is cancelled** (timeout OR signal-induced cancel)
- `cmd.WaitDelay = 3 * time.Second` — grace before Go escalates to SIGKILL

**Implication:** if the signal handler **cancels the context**, the executor's `cmd.Cancel` ALREADY
sends SIGTERM to the whole child group, and `WaitDelay` escalates to SIGKILL. So the child IS killed
by ctx-cancel alone. The handler's *direct* `KillProcessGroup` (FINDING 8) is **belt-and-suspenders /
immediate** — it does not wait for ctx-cancel propagation. We implement BOTH for robustness + to satisfy
the work item's explicit "forward via KillProcessGroup" requirement.

**Windows** (`procgroup_windows.go`) uses `CREATE_NEW_PROCESS_GROUP` + `GenerateConsoleCtrlEvent(
CTRL_BREAK_EVENT, pid)` in `cmd.Cancel`. Our `KillProcessGroup` mirrors this (see F6).

---

## F2 — There is NO standalone `KillProcessGroup` yet (work item names one that doesn't exist)

The work item says "INPUT: KillProcessGroup from P1.M2.T5.S2." But P1.M2.T5.S2 shipped
`func setupProcessGroup(cmd *exec.Cmd)` — the kill-to-group logic lives INSIDE its `cmd.Cancel` closure,
not as a reusable function. **This PRP therefore CREATES `KillProcessGroup`** as a new cross-platform
function in the new `internal/signal` package (Unix + Windows build-tag files). It is the single
mechanism the signal handler uses to forward a signal to a registered child's group.

- **Unix** (`signal_unix.go`, `//go:build !windows`): `syscall.Kill(-pgid, sig)`.
- **Windows** (`signal_windows.go`, `//go:build windows`): `GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, pgid)`
  via `syscall.NewLazyDLL("kernel32.dll")` — identical technique to `procgroup_windows.go`, stdlib-only
  (no `golang.org/x/sys` dependency; keeps `go.mod`/`go.sum` unchanged).

**Signature:** `func KillProcessGroup(pid int, sig os.Signal) error`. The caller passes the **positive**
child PID; each platform impl handles group addressing (Unix negates internally → `-pid`; Windows uses
`pid` directly as the console process-group id, since `CREATE_NEW_PROCESS_GROUP ⇒ PID==PGID`).

We do **NOT** refactor `procgroup_*.go`'s `cmd.Cancel` to call this (the `setupProcessGroup` signature is
FROZEN per P1.M2.T5.S2; leaving it untouched is lowest-risk). The kill one-liner is duplicated in spirit
but lives in two independent kill paths (ctx-cancel vs. signal-forward), both idempotent.

---

## F3 — Package dependency graph is CYCLE-FREE with this layout

```
internal/signal   → (stdlib only: os, os/signal, syscall, sync, sync/atomic, context, fmt, io)
internal/provider → internal/signal   (Execute calls signal.RegisterChild/ClearChild — nil-safe)
internal/generate → internal/signal   (CommitStaged calls signal.SetSnapshot/SetCandidate/RestoreDefault/ClearSnapshot)
pkg/stagecoach     → internal/signal + internal/generate   (runPipeline mirrors CommitStaged wiring)
cmd/stagecoach     → internal/signal + internal/generate + internal/cmd  (Install wires RescueFormat=FormatRescue)
```

**No cycle:** `signal` imports NO stagecoach package. The rescue message (`generate.FormatRescue`) is
passed to the handler as a **callback** (`Options.RescueFormat`) at `Install` time — wired by `main.go`,
which imports BOTH `signal` and `generate`. This is the key trick that lets the handler print the §18.3
rescue message WITHOUT `signal` importing `generate` (which would create signal↔generate cycle, since
`generate` calls `signal.SetSnapshot`).

---

## F4 — Singleton handler (`signal.Install` / `signal.Active`) — justified because signals are process-global

Signals are inherently process-global; there is exactly ONE signal-disposition per process. A package-level
singleton (`var active atomic.Pointer[Handler]`) is the pragmatic Go idiom (cf. `os/signal` itself, and
`signal.NotifyContext` which is also process-global).

- `signal.Install(parent ctx, opts) (ctx, *Handler)` — installs `signal.Notify` on a buffered channel,
  starts the handler goroutine, stores the handler in `active`, returns a signal-aware `ctx` (cancelled on
  signal). Called ONCE in `main.go`.
- `signal.Active() *Handler` — returns the current handler (nil if none installed, e.g. library use of
  `pkg/stagecoach` without the CLI).
- **Nil-safe package-level wrappers:** `signal.RegisterChild / ClearChild / SetSnapshot / SetCandidate /
  ClearSnapshot / RestoreDefault` — each does `if h := active.Load(); h != nil { h.<method>(...) }`. So
  `provider.Execute` and `generate.CommitStaged` call them unconditionally; when no handler is installed
  (library use), they are no-ops. **No call-site nil-checks needed.**

**Library-use safety:** a consumer of `pkg/stagecoach.GenerateCommit` who never calls `signal.Install` gets
the baseline behavior (their own ctx/signals; `Execute`'s `cmd.Cancel` still kills the group on their ctx
cancel). `provider` importing `signal` is harmless — `signal` has no `init()` side effects.

---

## F5 — `os.Exit` in the handler REQUIRES a subprocess integration test

The handler, on a signal with a non-empty snapshot, prints the rescue message and calls `os.Exit(3)`
(the AUTHORITATIVE rescue — it fires in ALL post-snapshot windows uniformly, including the
`CommitTree`/between-steps windows where the normal error-return path would NOT produce a `*RescueError`).
`os.Exit` terminates the test process, so the rescue+exit behavior CANNOT be unit-tested in-process.

**Resolution:** two test layers.
- **Unit tests** (`signal_test.go`, in-process): exercise the `Handler` state methods
  (`RegisterChild`/`ClearChild`/`SetSnapshot`/`SetCandidate`/`ClearSnapshot`/`RestoreDefault`) and the
  forward-kill logic by **injecting fakes** — `Options.Kill` (records the pid/sig instead of really
  killing), `Options.Exit` (records the code instead of exiting), `Options.RescueFormat`, `Options.Out`
  (a `*bytes.Buffer`). These run fast and deterministically.
- **Integration test** (`signal_integration_test.go`, `//go:build !windows`): drives the REAL stagecoach
  binary as a **subprocess** (`exec.Command("go","build","-o",bin,"./cmd/stagecoach")`, cached via
  `sync.Once` like `stubtest.Build`). Setup: temp git repo + staged file + a config pointing
  `[provider.stub] command = <stubagent>` with `STAGECOACH_STUB_SLEEP_MS=30000` (stub hangs). Start
  stagecoach, sleep ~800ms (snapshot+Execute guaranteed started; 800ms ≪ 30s stub sleep), send
  `SIGINT` to the stagecoach PID, `cmd.Wait()`, assert **exit code == 3**, stderr contains the §18.3
  rescue block (`Commit generation failed` + `Tree ID:` + `git commit-tree`), HEAD unchanged, and the
  stub child is dead.

---

## F6 — Why the handler prints rescue + `os.Exit(3)` directly (instead of only cancelling ctx)

The normal error-return path (`Execute → context.Canceled → CommitStaged → *RescueError →
default_action.handleGenError → FormatRescue + exit 3`) handles the rescue correctly ONLY when the
signal arrives **during `provider.Execute`**. But §18.4 says signals can arrive "any time post-snapshot."
The window **between** `WriteTree` and `UpdateRefCAS` includes `CommitTree` (a git subprocess). A signal
during `CommitTree` cancels the ctx → `CommitTree` fails with a wrapped git error (NOT
`context.Canceled`, and NOT a `*RescueError`) → `handleGenError` maps it to **exit 1** (generic), NOT
exit 3 + rescue. **That violates §18.2** (signal post-snapshot MUST be exit 3 + rescue).

The handler's **direct** rescue-print + `os.Exit(3)` fixes this uniformly: whenever a signal arrives
and the snapshot is registered (`treeSHA != ""`), the handler prints `FormatRescue` and exits 3 —
regardless of which step the main goroutine is blocked in. Because `os.Exit` is immediate, the normal
flow (still blocked in `Execute`/`CommitTree`) never gets to print, so there is **no double rescue
print** in practice (the only theoretical double-print is a sub-millisecond race if `handleGenError` is
mid-print exactly as the signal arrives — acceptable for v1; rescue is exit-3 either way).

**Pre-snapshot signal** (`treeSHA == ""`): handler kills the child (if any), cancels ctx, exits 130
(128+SIGINT). "Just exit" per §18.4 step 2.

---

## F7 — `RestoreDefault` before `update-ref` (the §18.4 step 3 / commit-pi `trap -` analog)

Right before `deps.Git.UpdateRefCAS(...)` in `CommitStaged` (step 8) AND in `pkg/stagecoach.runPipeline`,
call `signal.RestoreDefault()`. It does `signal.Stop(h.ch)` (stops delivering SIGINT/SIGTERM to our
channel → restores Go's default disposition) and closes the channel so the handler goroutine returns.

**Effect:** a Ctrl-C that arrives during `update-ref` (or after) uses Go's default behavior (exit),
NOT the rescue path. This is intentional: by the time we reach `update-ref`, the commit is essentially
done — a last-instant Ctrl-C is the user's explicit choice, not a generation failure. Printing a rescue
there would be misleading (the commit may even land via the CAS). This precisely mirrors commit-pi's
`trap - INT TERM` immediately before its `git commit`.

**v2 note (out of scope):** for v1 (single commit) the process exits after `update-ref`, so fully
stopping the handler is correct. v2's multi-commit loop (§11.3) would need to re-`Install` per
iteration — not our concern now.

---

## F8 — Wiring touches BOTH `generate.CommitStaged` AND `pkg/stagecoach.runPipeline`

The CLI default action calls `pkg/stagecoach.GenerateCommit`, which delegates to `CommitStaged` for the
common path (no DryRun, no SystemExtra) BUT runs its OWN pipeline (`runPipeline`) for DryRun/SystemExtra.
The **commit path** of `runPipeline` (SystemExtra set, not DryRun) ALSO does `WriteTree` + the generate
loop + `CommitTree` + `UpdateRefCAS` — so it has the SAME post-snapshot signal windows and MUST get the
identical wiring (`SetSnapshot` after `WriteTree`; `SetCandidate` in the loop; `RestoreDefault` before
`UpdateRefCAS`; `ClearSnapshot` after success). The DryRun branch of `runPipeline` does NOT snapshot
(no `WriteTree`) → no rescue wiring needed there.

Both wirings are the same 4-call pattern, so the PRP specifies both (DRY of intent; the two functions
already mirror each other per P1.M3.T5).

---

## F9 — The candidate message is propagated to the handler for the §18.3 note

§18.3 appends a candidate note ("A candidate message was produced but rejected: …") when a message was
in hand. For the **non-signal** rescue path this is handled by `*RescueError.Candidate` →
`FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate)` in `handleGenError`. For the **signal** rescue
path (handler), the handler reads `snapshot.candidate`. So the generate loop must update it:
`SetSnapshot(tree, parent, "")` right after `WriteTree`, then `SetCandidate(m)` after each successfully
parsed message `m` (before the dedupe check). On signal, the handler's `FormatRescue` uses the latest
registered candidate. This keeps the signal-rescue UX faithful to §18.3.

---

## D1..D9 — Decision summary (maps to PRP contract clauses)

- **D1 (new package):** `internal/signal` (stdlib-only core + Unix/Windows `KillProcessGroup`). Singleton
  via `atomic.Pointer[Handler]` (Go 1.19+, we're on 1.22).
- **D2 (injectable seams):** `Options.Kill`, `Options.Exit`, `Options.RescueFormat`, `Options.Out` — all
  defaulted; fakes enable in-process unit tests. `Exit` defaults to `os.Exit`; `Kill` to `KillProcessGroup`.
- **D3 (handler goroutine):** forward-to-child-group → cancel ctx → if snapshot set: print rescue +
  `Exit(3)`; else `Exit(130/143)`.
- **D4 (child PID):** `provider.Execute` calls `signal.RegisterChild(pid)` after `cmd.Start`, defers
  `signal.ClearChild()`. Nil-safe. (provider→signal; one-way; no cycle.)
- **D5 (snapshot state):** `generate.CommitStaged` + `pkg.runPipeline` call `signal.SetSnapshot` after
  `WriteTree`, `signal.SetCandidate` in the loop, `signal.RestoreDefault` before `UpdateRefCAS`,
  `signal.ClearSnapshot` after success.
- **D6 (CLI install):** `main.go` calls `signal.Install`, wires `RescueFormat: generate.FormatRescue`,
  `Out: os.Stderr`, passes the signal-aware ctx to `cmd.Execute`. (main.go's comment already reserved this.)
- **D7 (cross-platform kill):** `KillProcessGroup(pid, sig)` in build-tag files; stdlib-only on Windows.
- **D8 (testing):** unit (injectable fakes) + subprocess integration (real binary + hanging stub + SIGINT).
- **D9 (scope boundary):** no changes to `procgroup_*.go` (frozen `setupProcessGroup`), no changes to
  `root.go`/`config.go` (owned by P1.M4.T1 siblings, running in parallel), no change to the rescue
  message format (`FormatRescue` is frozen from P1.M3.T3.S1 — we only CALL it).
