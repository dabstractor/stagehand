---
name: "P1.M2.T5.S1 — Provider Executor (subprocess: stdin pipe, stdout/stderr capture, env, timeout, process-group kill) — PRD §9.5/FR25 + §18.2/§18.4"
description: |

  Land the FIRST subtask of Provider Executor (P1.M2.T5): `internal/provider/executor.go` with
  `func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration) (stdout, stderr string,
  err error)` that runs a provider `CmdSpec` (produced by P1.M2.T4.S1's `Manifest.Render`) as a
  subprocess with FULL control over I/O, environment, timeout, and process-group cleanup — per PRD
  §9.5/FR25 (configurable generation timeout, default 120s; on timeout kill the agent and enter
  rescue), §18.2 (timeout → exit 124 + rescue; SIGINT/SIGTERM → exit 3 + rescue), and §18.4
  (propagate SIGTERM to the child's process group). Implements the canonical Go 1.20+ pattern from
  architecture/go_ecosystem_patterns.md §3 + critical_findings.md FINDING 8/10: `SysProcAttr.Setpgid`
  (child = own process-group leader, PGID==PID) + `cmd.Cancel = syscall.Kill(-pid, SIGTERM)` (kill
  the whole tree) + `cmd.WaitDelay = 3s` (grace before Go's SIGKILL).

  INPUT: the `CmdSpec` from P1.M2.T4.S1 (`Render`) — `{Command, Args, Stdin, Env}` (FROZEN; read
  `internal/provider/render.go`). The `timeout` from P1.M1.T4's resolved `Config.Timeout`
  (`time.Duration`, default `120 * time.Second`).

  OUTPUT: `stdout` (the agent's raw output, for parser P1.M2.T6), `stderr` (for verbose/rescue
  diagnostics), and `err`. `err == context.DeadlineExceeded` ⇒ timeout; `context.Canceled` ⇒
  signal/parent-cancel; a wrapped `*exec.ExitError` ⇒ non-zero exit; a wrapped LookPath error ⇒
  command not found. `stdout`/`stderr` are ALWAYS returned (even partial, on error) so the rescue
  path (§18.3) can salvage a candidate message and verbose mode (P1.M4.T3.S2) can dump raw output.

  ⚠️ **THE central design call — build-tag-segregated CROSS-PLATFORM SPLIT.** `SysProcAttr.Setpgid`
  and `syscall.Kill(-pid, sig)` are Unix-only (FINDING 10); Windows needs Job Objects. The work item
  says "Apply process-group setup (delegate to S2's cross-platform abstraction)." S2
  (P1.M2.T5.S2) OWNS the abstraction but ships AFTER S1, and S1 MUST compile + pass tests on the
  Unix CI matrix independently. Resolution: S1 establishes the abstraction CONTRACT + the Unix
  implementation; S2 later adds the Windows implementation to the SAME frozen signature. Concretely:
  (a) `executor.go` is platform-AGNOSTIC (NO build tag, NO `syscall` import) and calls ONE helper
  `setupProcessGroup(cmd)`; (b) S1 creates `procgroup_unix.go` (`//go:build !windows`) implementing
  that helper (Setpgid + Cancel + WaitDelay); (c) S2 (future) creates `procgroup_windows.go`
  (`//go:build windows`) with the identical signature — touching NEITHER executor.go NOR
  procgroup_unix.go. This yields zero merge collision, a portable executor, and a self-contained S1.

  ⚠️ **THE second design call — `ctx.Err()` checked FIRST to disambiguate timeout vs exit error.**
  `exec.CommandContext`'s `cmd.Wait()` returns an error in BOTH the timeout case and the non-zero-exit
  case; the distinguishing signal is `ctx.Err()`. Check it first: non-nil ⇒ return it
  (`DeadlineExceeded` for timeout / `Canceled` for parent/signal cancel); else wrap the `*exec.ExitError`
  as a real exit failure. Without this order, a timeout would be misreported as a generic exit error
  and the orchestrator could not fire exit-124 + rescue.

  ⚠️ **THE third design call — `context.WithTimeout` SHADOWS the `ctx` param (load-bearing).**
  `ctx, cancel = context.WithTimeout(ctx, timeout)` shadows so the later `ctx.Err()` reads the TIMEOUT
  context (→ `DeadlineExceeded` on timeout, `Canceled` on parent-cancel). Do not "fix" the shadow by
  renaming — a non-shadowed read would report `nil` on timeout. Only derive when `timeout > 0`
  (timeout ≤ 0 ⇒ no timeout; the parent ctx still applies).

  ⚠️ **THE fourth design call — Start()+Wait(), NOT cmd.Run()/CombinedOutput().** The work-item
  signature is `(stdout, stderr string, err error)` — three values, all populated even on error.
  `cmd.Run()`/`CombinedOutput` cannot return separate buffers alongside an error. So call
  `cmd.Start()` then `cmd.Wait()` manually, reading the two `bytes.Buffer`s afterward (exactly §3.1).

  ⚠️ **THE fifth design call — Stdin semantics follow the CmdSpec contract; empty Env is guarded.**
  `if spec.Stdin != "" { cmd.Stdin = strings.NewReader(spec.Stdin) }` — empty ⇒ leave `cmd.Stdin`
  nil ⇒ os/exec gives the child `/dev/null` (the exact semantics `render.go`'s doc promised: "Stdin=''
  means no stdin pipe"). `cmd.Env = spec.Env` guarded by `if len(spec.Env) > 0` so a nil Env
  (hand-built test spec) inherits the parent env rather than receiving an empty env (which would break
  the child). `Render` always populates Env, so the guard is harmless in production but protects tests.
  NO `cmd.Dir` is set — the agent runs in the user's CWD (git uses `-C repo`; the agent does not).

  ⚠️ **THE sixth design call — tests shell out to REAL binaries, matching codebase conventions.**
  committree_test.go tests git via the REAL git binary (no mocks). The executor tests follow suit:
  real, universally-present Unix binaries (`cat`, `sleep`, `printenv`, `false`) with `exec.LookPath`
  guards that `t.Skip` if absent. The work item explicitly sanctions `/bin/cat`. Cases: normal run
  (cat echoes stdin), large stdin (1MiB stress), timeout kill (sleep 30 + 200ms timeout ⇒
  DeadlineExceeded + returns promptly), stderr capture + non-zero exit (cat nonexistent), env
  propagation (printenv), no-stdin path (Stdin="" ⇒ no hang), command-not-found, parent-ctx-cancel.

  Deliverable: `internal/provider/executor.go` (`package provider`, no build tag, imports
  `bytes`/`context`/`fmt`/`os/exec`/`strings`/`time`) — the `Execute` function; `internal/provider/
  procgroup_unix.go` (`package provider`, `//go:build !windows`, imports `os/exec`/`syscall`/`time`)
  — the Unix `setupProcessGroup(cmd)`; and `internal/provider/executor_test.go` (`package provider`,
  no build tag) — ~8 test groups. Touches ONLY the three new files — NO go.mod/go.sum change (stdlib
  only), NO edit to any frozen file (render.go's CmdSpec is a CONTRACT; manifest/builtin/registry are
  untouched). OUTPUT = the executed agent output the parser (P1.M2.T6) reads and the err the
  orchestrator (P1.M3.T4) maps to retry/rescue/exit-code.

---

## Goal

**Feature Goal**: Implement the provider executor — the function that runs a provider `CmdSpec` as a
subprocess with piped stdin, separately-captured stdout/stderr, a controlled environment, a
configurable timeout, and whole-process-group killing on cancel. It is the third quarter of the
provider system's "produce a concrete command line, then RUN it" mission (§12): manifests (T1/T2/T3)
describe the agents; the renderer (T4) composes the invocation; **the executor (T5.S1) runs it**;
the parser (T6) reads stdout. On timeout/signal it kills the agent tree and surfaces a sentinel
error so the orchestrator can enter the rescue path (§18.2/§18.4).

**Deliverable**:
1. **CREATE** `internal/provider/executor.go` (`package provider`, no build tag) —
   `func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration) (stdout, stderr string,
   err error)`: derive timeout context (if `timeout > 0`) → `exec.CommandContext(ctx, spec.Command,
   spec.Args...)` → wire stdin/stdout/stderr/env → `setupProcessGroup(cmd)` → `Start()` + `Wait()` →
   return `(stdout, stderr, err)` with `ctx.Err()`-first error disambiguation.
2. **CREATE** `internal/provider/procgroup_unix.go` (`package provider`, `//go:build !windows`) —
   `func setupProcessGroup(cmd *exec.Cmd)`: `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`;
   `cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }`;
   `cmd.WaitDelay = 3 * time.Second`. (The frozen CONTRACT for S2's Windows file.)
3. **CREATE** `internal/provider/executor_test.go` (`package provider`, no build tag) — the ~8 test
   groups below, all passing.

No other files touched. **No go.mod/go.sum change** (stdlib only). NO edit to `render.go` (CmdSpec is
FROZEN), `manifest.go`/`merge.go`/`builtin.go`/`registry.go` or their tests.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/` clean;
`go test -race ./...` green (all existing provider/config/git tests STILL green + new executor tests
green); the timeout test returns `context.DeadlineExceeded` and completes within seconds (proving the
kill fires, not a 30s hang); the env test proves a manifest env var reaches the child; the
`cat`-echo and 1MiB-large-stdin tests prove stdin piping; stderr + non-zero-exit are captured
separately; go.mod/go.sum and every frozen file byte-unchanged. Cross-compiling to Windows is allowed
to fail (`undefined: setupProcessGroup`) until S2 lands procgroup_windows.go — that is correct and
expected (the CI matrix is Linux/macOS; PRD §20.4).

## User Persona

**Target User**: The generate orchestrator (P1.M3.T4 — calls `Execute(ctx, spec, cfg.Timeout)` and
maps the returned `err` to retry/rescue/exit-code) and transitively the parser (P1.M2.T6 — consumes
the returned `stdout`). End-user persona is "the plan-holder" / "the API-key refusenik" (PRD §7)
running any of the 6 verified agent CLIs (pi, claude, gemini, opencode, codex, cursor) via Stagecoach.

**Use Case**: The generate flow resolves the provider manifest (P1.M2.T3), renders the `CmdSpec`
(P1.M2.T4), then calls `Execute(ctx, *spec, cfg.Timeout)`. If the agent finishes in time, its stdout
goes to the parser. If it times out (FR25), the executor kills the agent's process group (SIGTERM →
3s → SIGKILL) and returns `context.DeadlineExceeded`; the orchestrator enters rescue (§18.2, exit 124).
If the user hits Ctrl-C (§18.4), the signal handler cancels `ctx`; the executor's `cmd.Cancel` forwards
SIGTERM to the group; the orchestrator enters rescue (exit 3).

**User Journey**: (internal API, no end-user surface yet) `Manifest.Render(...)` → `*CmdSpec` →
`Execute(ctx, *spec, timeout)` → `(stdout, stderr, err)` → parser reads stdout / orchestrator maps err.

**Pain Points Addressed**: Removes "how do we run an arbitrary agent CLI safely / how do we kill it
and its grandchildren on timeout / how do we distinguish timeout from a normal failure / how do we
pipe a prompt over stdin and capture only the agent's stdout" ambiguity by landing one tested executor
now — the single site that owns PRD §9.5 generation execution + §18.2/§18.4 kill semantics.

## Why

- **The executor is the bridge from "rendered command" to "agent output" (§12 / §9.5 FR24).** Without
  it the renderer's `CmdSpec` is inert data. FR24 ("Capture the agent's stdout") and FR25 ("configurable
  generation timeout; on timeout, kill the agent process and enter the rescue path") are THIS subtask.
- **Implements the safety invariant (§18.1 / §18.4).** On timeout or signal, the WHOLE process tree
  must die (no orphaned grandchildren leaking model API calls). `Setpgid` + `Kill(-pid, SIGTERM)` +
  `WaitDelay` is the battle-tested way (FINDING 8 / §3). Getting this wrong = leaked processes + cost.
- **Surfaces the timeout/signal sentinel the rescue path depends on.** `context.DeadlineExceeded`
  (exit 124) and `context.Canceled` (exit 3) must be distinguishable from a plain non-zero exit so the
  orchestrator (P1.M3.T4) and CLI (P1.M4.T3.S3) can fire the right exit code + rescue (§18.2 table).
- **Establishes the cross-platform seam for S2.** By freezing the `setupProcessGroup(cmd)` signature +
  landing the Unix half, S2 (Windows) becomes a clean single-file addition with no merge collision.
- **No user-facing surface change** (PRD "DOCS: none — internal executor"). Verbose-mode raw-output
  docs come with P1.M4.T3.S2.
- **No new dependency.** Stdlib only. go.mod/go.sum unchanged.

## What

A compiled `internal/provider` package exporting `Execute` + `setupProcessGroup` (the latter
platform-specific via build tags), layered on T4's `CmdSpec`. No parsing, no CLI, no config edits,
no signal-handler installation (the executor only HONORS ctx cancellation, which `cmd.Cancel` already
turns into a group SIGTERM).

### Success Criteria

- [ ] `internal/provider/executor.go` exists, `package provider`, NO build tag, imports EXACTLY
      `bytes`, `context`, `fmt`, `os/exec`, `strings`, `time`. It does NOT import `syscall`
      (that lives in `procgroup_unix.go`), `internal/config`, or `internal/git`.
- [ ] `func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration) (stdout, stderr string,
      err error)` — derives a timeout context when `timeout > 0`; builds `exec.CommandContext(ctx,
      spec.Command, spec.Args...)`; pipes stdin when `spec.Stdin != ""`; captures stdout+stderr to two
      `bytes.Buffer`s; sets `cmd.Env` when non-empty; calls `setupProcessGroup(cmd)`; runs Start()+Wait();
      returns `(stdout, stderr, err)` with `ctx.Err()`-first disambiguation.
- [ ] `internal/provider/procgroup_unix.go` exists, `package provider`, build tag `//go:build !windows`,
      imports `os/exec`, `syscall`, `time`. Defines `func setupProcessGroup(cmd *exec.Cmd)` that sets
      `SysProcAttr{Setpgid:true}`, `cmd.Cancel` = group SIGTERM, `cmd.WaitDelay = 3*time.Second`.
- [ ] Timeout (e.g. `sleep 30` + 200ms) ⇒ `err` IS `context.DeadlineExceeded` (`errors.Is` true) AND
      Execute returns within ~3s (not 30s) — proving the kill fired.
- [ ] Parent-context cancel ⇒ `err` IS `context.Canceled`.
- [ ] Non-zero exit (e.g. `false`, `cat /nonexistent`) ⇒ `err` non-nil (wrapped), stdout+stderr still
      returned; stderr non-empty for the `cat /nonexistent` case (separate capture verified).
- [ ] `cat` echoes stdin to stdout verbatim; 1MiB stdin round-trips byte-for-byte.
- [ ] `printenv VAR` returns the manifest/env value ⇒ `cmd.Env` wiring verified.
- [ ] Empty `spec.Stdin` (`""`) ⇒ no hang (child gets /dev/null), exit 0, stdout "".
- [ ] Command not found ⇒ wrapped Start() error returned.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` (diff-empty) clean;
      `go test -race ./...` green.
- [ ] go.mod/go.sum byte-unchanged; every file outside the three new files byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the canonical
os/exec pattern + the four design calls (research §1, §3), the cross-platform build-tag split +
frozen helper signature (research §2), the `CmdSpec` contract (research §4 — read render.go), the test
strategy with exact binaries + cases (research §5), the per-file import sets (research §6), and the ~8
test specs. No parsing/generate/CLI knowledge required — the executor is a self-contained function over
an already-landed `CmdSpec` type, exec'ing real binaries in tests.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M2T5S1/research/exec-pattern-and-cross-platform-split.md
  why: the SINGLE most important read — the canonical os/exec pattern + field semantics (§1), the
       cross-platform build-tag split + frozen setupProcessGroup signature (§2 — THE central design
       call), the error/timeout return contract + check order (§3), the CmdSpec wiring (§4), the test
       strategy with exact binaries + cases (§5), the per-file import sets (§6).
  critical: §2 (the S1/S2 split — do NOT inline syscall into executor.go; do NOT create the Windows
       file) and §3.2 (ctx.Err() checked FIRST) are the things most likely to be implemented wrong.

- file: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md   (READ §3; do NOT copy wholesale)
  section: "3. os/exec: Safe Subprocess Execution with Process Groups" (h3 — §3.1 canonical Run, §3.4
           key safety notes, §3.5 Setpgid/signal interaction).
  why: the AUTHORITATIVE pattern this executor implements (cmd.Cancel = Kill(-pid, SIGTERM) +
       WaitDelay = 3s + Setpgid + Start/Wait + separate buffers). §3.1's `Run` returns a Result struct;
       our Execute returns (string,string,error) per the work item — the PATTERN (Start+Wait, separate
       buffers, ctx.Err()-first) is reused, the SIGNATURE differs. Do NOT import §3.1's hypothetical
       execrun package; implement inline in executor.go.
  critical: §3.4 confirms Setpgid is Unix-only (→ build-tag split). §3.5 confirms the child will NOT
       get terminal Ctrl-C directly → the PARENT must forward signals; that is the CLI signal handler's
       job (P1.M4.T2.S1), NOT the executor's (the executor only honors ctx cancellation via cmd.Cancel).

- file: plan/001_f1f80943ac34/architecture/critical_findings.md   (READ FINDING 8 + FINDING 10)
  section: "FINDING 8: Signal handling requires process-group kill (Setpgid)" + "FINDING 10: Windows
           SysProcAttr.Setpgid is Unix-only".
  why: FINDING 8 pins the cmd.Cancel + WaitDelay + Setpgid + Kill(-pid) recipe and the §18.4 rescue
       flow. FINDING 10 pins the build-tag-segregated file layout (exec_unix.go !windows +
       exec_windows.go windows) — the executor applies the SAME approach with file names
       procgroup_unix.go / procgroup_windows.go inside internal/provider (the work item mandates
       internal/provider/executor.go).
  critical: FINDING 10 names a hypothetical internal/executil/ location, but the WORK ITEM mandates
       internal/provider/executor.go. Keep the abstraction in the SAME package (internal/provider) so
       executor.go can call setupProcessGroup with no import — do NOT create a new executil package.

- file: internal/provider/render.go   (P1.M2.T4.S1 — COMPLETE; read, do NOT edit)
  why: the EXACT `CmdSpec` type Execute consumes — `type CmdSpec struct { Command string; Args
       []string; Stdin string; Env []string }`. The doc comment STATES the executor contract: "the
       executor runs exec.Command(spec.Command, spec.Args...) with cmd.Stdin/cmd.Env derived from
       Stdin/Env" and "Stdin='' → executor uses os.DevNull". Execute fulfills that contract.
  critical: CmdSpec is FROZEN (downstream T6/M3.T4/M4.T3.S2 depend on it). Do NOT edit render.go. The
       Stdin/Env semantics Execute implements are EXACTLY what render.go's doc promised.

- file: internal/git/git.go   (P1.M1.T2/T3 — read for the os/exec + error-handling IDIOM; do NOT edit)
  section: the private `run()` / `runWithInput()` helpers (LookPath → CommandContext → separate
           buffers → errors.As(ExitError)).
  why: the codebase's established os/exec idiom — separate stdout/stderr buffers, LookPath for the
       binary, context-aware error handling. The executor follows the SAME conventions (separate
       buffers, Start+Wait, ctx.Err() check). NOTE one DELIBERATE difference: git's run() treats
       non-zero exits as (stdout,exitCode,nil) — a semantic signal; the AGENT executor treats non-zero
       exit as a wrapped error (an agent that exits non-zero FAILED; the orchestrator retries/rescues).
       Do not copy git's exit-code-as-signal convention here.
  critical: the agent executor is NOT a git plumbing call — its error contract is "timeout/cancel vs
       exit-failure vs success", not git's exit-code semantics. Mirror the STRUCTURE (buffers,
       CommandContext, ctx check), not the exit-code return.

- file: internal/git/committree_test.go   (read for the TEST IDIOM; do NOT edit)
  section: fixture helpers (setIdentityConfig/writeFile/stageFile) + the Test* functions.
  why: the codebase tests git by shelling out to the REAL git binary with stdlib testing, direct
       t.Fatalf/t.Errorf, t.Setenv — NO mocking framework. The executor tests follow suit with real
       Unix binaries (cat/sleep/printenv/false) + exec.LookPath guards. Copy the assertion STYLE.
  critical: no mocking framework exists; do NOT introduce one. Use real binaries + t.Skip on absence.

- file: internal/config/config.go   (P1.M1.T4 — read, do NOT edit; do NOT import)
  why: confirms `Config.Timeout` is a `time.Duration` (default `120 * time.Second` via Defaults()).
       The orchestrator passes cfg.Timeout to Execute. Execute takes a `time.Duration` directly (NOT a
       *Config) — it does NOT import internal/config (avoids a cycle; the executor is provider-layer).
  critical: Execute's `timeout time.Duration` param is the wired value; ≤ 0 ⇒ no timeout.

- file: plan/001_f1f80943ac34/P1M2T4S1/PRP.md   (the PREVIOUS item — the input contract)
  section: "DOWNSTREAM CONTRACTS" — "P1.M2.T5 (executor): exec.Command(spec.Command, spec.Args...);
           cmd.Stdin = (spec.Stdin != "") ? strings.NewReader(spec.Stdin) : <os.DevNull>;
           cmd.Env = spec.Env."
  why: this is the EXACT contract T4's PRP documented for the executor. Execute implements it
       verbatim. (Parallel-execution note: T4 is being implemented concurrently — treat its CmdSpec
       as FROZEN per its PRP; it already exists in render.go.)
  critical: the CmdSpec shape + the Stdin/Env wiring rules are dictated by T4; do not diverge.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; require go-toml/v2 + pflag  (UNCHANGED — Execute adds NO dep)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — untouched; do NOT import (cycle)
  git/                          # P1.M1.T2/T3 — untouched (read for idiom only)
  provider/                     # S1..T4 created; this subtask adds executor + procgroup + test
    manifest.go                 # S1 — Manifest + Validate + Resolve + ...        (CONTRACT — do NOT edit)
    manifest_test.go            # S1                                                  (do NOT edit)
    merge.go / merge_test.go    # S2                                                  (do NOT edit)
    builtin.go / builtin_test.go# S2/S3                                              (do NOT edit)
    registry.go / registry_test.go # P1.M2.T3                                        (do NOT edit)
    render.go                   # T4 — CmdSpec + Manifest.Render  (FROZEN CONTRACT — do NOT edit)
    render_test.go              # T4                                                  (do NOT edit)
    executor.go                 # NEW (this subtask) ← Execute (no build tag; stdlib: bytes,context,fmt,os/exec,strings,time)
    procgroup_unix.go           # NEW (this subtask) ← setupProcessGroup (//go:build !windows; os/exec,syscall,time)
    executor_test.go            # NEW (this subtask) ← ~8 test groups (no build tag)
cmd/stagecoach/main.go           # `package main; func main(){}` stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  provider/
    executor.go                 # NEW — func Execute(ctx, spec CmdSpec, timeout) (stdout, stderr string, err error)
    procgroup_unix.go           # NEW — func setupProcessGroup(cmd *exec.Cmd)  [//go:build !windows]
    executor_test.go            # NEW — ~8 test groups, package provider
# render.go (T4) CmdSpec + Manifest.Render FROZEN. manifest/merge/builtin/registry UNCHANGED.
# go.mod/go.sum UNCHANGED. Every other file UNCHANGED.
# FUTURE (P1.M2.T5.S2): procgroup_windows.go [//go:build windows] — same setupProcessGroup signature.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call 1 — do NOT inline syscall into executor.go): executor.go must be platform-
// AGNOSTIC (no build tag, no `syscall` import). ALL syscall use lives in procgroup_unix.go
// (//go:build !windows). The single seam is setupProcessGroup(cmd). If you import syscall in
// executor.go, the package won't be portable and go vet/gofmt will fight you. S2's Windows file
// (procgroup_windows.go) will implement the SAME signature — do NOT create that file (it's S2's job).

// CRITICAL (design call 2 — check ctx.Err() BEFORE wrapping the exit error): cmd.Wait() errors in
// BOTH the timeout case and the non-zero-exit case. The ONLY reliable disambiguator is ctx.Err():
//   if err := cmd.Wait(); err != nil {
//       if ctxErr := ctx.Err(); ctxErr != nil { return out, errb, ctxErr }  // timeout/cancel FIRST
//       return out, errb, fmt.Errorf("provider %q: %w", spec.Command, err)   // real exit error
//   }
// Without this order a timeout is misreported as a generic exit error → orchestrator can't fire 124.

// CRITICAL (design call 3 — the ctx SHADOW is load-bearing): inside the `if timeout > 0` block,
// `ctx, cancel = context.WithTimeout(ctx, timeout)` SHADOWS the param. The later `ctx.Err()` reads
// the TIMEOUT ctx → DeadlineExceeded on timeout / Canceled on parent-cancel. Do NOT rename to avoid
// the shadow — a non-shadowed read returns nil on timeout. Only derive when timeout > 0.

// CRITICAL (design call 4 — Start()+Wait(), not Run()/CombinedOutput): the (stdout, stderr string,
// err error) signature needs the two buffers returned ALONGSIDE the error. cmd.Run() returns only an
// error; CombinedOutput merges streams. Use Start() then Wait() and read the buffers after (§3.1).

// CRITICAL (design call 5 — Stdin/Env wiring follows the CmdSpec contract):
//   if spec.Stdin != "" { cmd.Stdin = strings.NewReader(spec.Stdin) }   // "" → nil → /dev/null
//   if len(spec.Env) > 0 { cmd.Env = spec.Env }                          // nil → inherit parent
// Do NOT set cmd.Dir (agent runs in user CWD; git uses -C, the agent does not).

// CRITICAL (cmd.Cancel closure dereferences cmd.Process — safe by os/exec contract): Cancel is only
// invoked AFTER Start() succeeds, so cmd.Process is non-nil. The closure may read cmd.Process.Pid.
//   cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
// (If the process already died, Kill returns ESRCH; os/exec logs but does not fail Wait — leave as-is.)

// GOTCHA (WaitDelay units): cmd.WaitDelay = 3 * time.Second (a Duration), NOT 3 or "3s". Import time.

// GOTCHA (build-tag file MUST declare `package provider` AND start with the constraint line): the
// //go:build line is the FIRST line; a blank line; then the package clause. gofmt enforces this. A
// missing/old-style `// +build` is NOT required for go 1.22 (//go:build is sufficient).

// GOTCHA (no grandchild-kill test): verifying the process-group kill reaches GRANDCHILDREN needs a
// portable grandchild-spawning test (fiddly). We do NOT test it directly. The timeout test proves the
// kill FIRES (Execute returns within ~3s, not 30s — sleep dies on SIGTERM at once); the Setpgid +
// Kill(-pid) correctness is established by code review vs the canonical pattern (research §5.4).

// GOTCHA (tests use REAL binaries — must be in PATH on the runner): cat/sleep/printenv/false are
// present on ubuntu-latest + macos-latest + the dev Linux box. Guard with exec.LookPath + t.Skip so a
// missing binary degrades to a skip, not a hard failure. Do NOT introduce a mocking framework.

// GOTCHA (executor does NOT install the signal handler): §18.4's signal.Notify is the CLI layer's
// job (P1.M4.T2.S1). The executor only HONORS ctx cancellation — when the handler cancels ctx,
// cmd.Cancel fires and SIGTERMs the group. Do NOT call signal.Notify in executor.go.

// GOTCHA (agent non-zero exit is a FAILURE, unlike git's exit-code-as-signal): git's run() returns
// (stdout, exitCode, nil) because git uses exit codes as semantic signals (1=has-staged, 128=unborn).
// The AGENT executor is different: a non-zero agent exit is a FAILURE (wrap it as an error) so the
// orchestrator retries/rescues. Do NOT copy git's exit-code-as-nil-error convention into Execute.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/provider/executor.go
package provider

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Execute runs a provider CmdSpec as a subprocess and returns its captured stdout, captured stderr,
// and a result error. It is the third stage of the provider pipeline (PRD §9.5/FR24–FR25): manifests
// (T1–T3) describe the agent; Render (T4) composes the CmdSpec; Execute (T5.S1) runs it; the parser
// (T6) reads stdout. The returned stdout is the agent's raw output for the parser; err signals
// timeout/failure for the orchestrator's retry/rescue (§18.2).
//
// I/O: spec.Stdin (non-empty) is piped to the child's stdin; "" leaves stdin nil so os/exec gives the
// child /dev/null (the CmdSpec contract from render.go). stdout and stderr are captured to SEPARATE
// buffers and returned even on error (partial output is useful for the rescue path §18.3 + verbose
// mode). cmd.Env = spec.Env when non-empty (Render builds os.Environ()+manifest env); nil Env ⇒ the
// child inherits the parent env. cmd.Dir is NOT set — the agent runs in the user's CWD.
//
// Timeout: when timeout > 0 a context.WithTimeout is derived (shadowing ctx — load-bearing: the later
// ctx.Err() reads the timeout ctx). The Config.Timeout default is 120s (PRD FR25). timeout ≤ 0 ⇒ no
// timeout (the parent ctx still applies).
//
// Kill semantics (PRD §18.2/§18.4, FINDING 8): the child runs as its own process-group leader
// (Setpgid ⇒ PGID==PID). On ctx cancellation (timeout OR parent/signal cancel) cmd.Cancel sends
// SIGTERM to the WHOLE group (-pid), killing grandchildren too; cmd.WaitDelay (3s) is the grace
// before Go escalates to SIGKILL. The platform specifics live in procgroup_unix.go (this file is
// platform-agnostic — setupProcessGroup is the single seam).
//
// Error contract (check ctx.Err() FIRST):
//   - timeout        ⇒ err IS context.DeadlineExceeded  (orchestrator: exit 124 + rescue)
//   - signal/parent  ⇒ err IS context.Canceled          (orchestrator: exit 3 + rescue)
//   - non-zero exit  ⇒ wrapped *exec.ExitError           (orchestrator: retry, then rescue)
//   - start failure  ⇒ wrapped LookPath/start error      (orchestrator: "command not found", exit 1)
//   - success        ⇒ err == nil
func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration) (stdout string, stderr string, err error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout) // SHADOW — see doc; do not rename
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	if spec.Stdin != "" {
		cmd.Stdin = strings.NewReader(spec.Stdin) // "" ⇒ nil ⇒ /dev/null (CmdSpec contract)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out // separate capture
	cmd.Stderr = &errb
	if len(spec.Env) > 0 {
		cmd.Env = spec.Env // Render populates; nil ⇒ inherit parent env
	}
	setupProcessGroup(cmd) // platform seam (procgroup_*.go): Setpgid + Cancel + WaitDelay

	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("provider %q: start: %w", spec.Command, err)
	}

	if werr := cmd.Wait(); werr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return out.String(), errb.String(), ctxErr // timeout → DeadlineExceeded; cancel → Canceled
		}
		return out.String(), errb.String(), fmt.Errorf("provider %q: %w", spec.Command, werr) // exit failure
	}
	return out.String(), errb.String(), nil
}
```

```go
// internal/provider/procgroup_unix.go
//go:build !windows

package provider

import (
	"os/exec"
	"syscall"
	"time"
)

// setupProcessGroup configures cmd so that on context cancellation (timeout or parent/signal cancel)
// the ENTIRE child process tree is terminated, preventing orphaned grandchildren (PRD §18.4, FINDING 8,
// go_ecosystem_patterns §3). It sets three fields on cmd:
//
//   - SysProcAttr.Setpgid = true → the child is a new process-group leader; its PGID == its PID, so
//     -pid addresses the whole group (child + all descendants).
//   - cmd.Cancel → on ctx cancel, send SIGTERM to the group (-pid). Gentler than the default SIGKILL
//     (lets the agent flush), and reaches grandchildren (the default only kills the direct child).
//     cmd.Process is guaranteed non-nil inside Cancel (os/exec invokes it only after Start()).
//   - cmd.WaitDelay = 3s → after Cancel, wait 3s for exit before Go forcibly SIGKILLs (handles an
//     agent that ignores SIGTERM).
//
// CONTRACT (FROZEN for P1.M2.T5.S2): the Windows build (procgroup_windows.go, //go:build windows) MUST
// implement the IDENTICAL signature `func setupProcessGroup(cmd *exec.Cmd)` using Job Objects /
// CREATE_NEW_PROCESS_GROUP. Do NOT change this signature without coordinating with S2.
//
// Unix implementation (Linux/macOS/darwin). The CI matrix targets Linux + macOS (PRD §20.4).
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) // -pid ⇒ whole process group
	}
	cmd.WaitDelay = 3 * time.Second
}
```

> **gofmt note:** run `gofmt -w internal/provider/executor.go internal/provider/procgroup_unix.go
> internal/provider/executor_test.go`. The `//go:build !windows` line MUST be the first line of
> procgroup_unix.go, followed by a blank line, then `package provider` (gofmt enforces this).
>
> **Imports:** executor.go = `bytes, context, fmt, os/exec, strings, time` (NO `syscall`).
> procgroup_unix.go = `os/exec, syscall, time`. executor_test.go = `bytes, context, errors, os,
> os/exec, strings, testing, time`. `go vet` flags unused imports — do not add what you don't use.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/provider/procgroup_unix.go — the Unix setupProcessGroup
  - FIRST LINE: `//go:build !windows` then blank line then `package provider`.
  - IMPLEMENT func setupProcessGroup(cmd *exec.Cmd) per the Data Models block: SysProcAttr{Setpgid:
      true}; cmd.Cancel = group SIGTERM; cmd.WaitDelay = 3*time.Second.
  - IMPORTS: "os/exec", "syscall", "time".
  - GOTCHA: -cmd.Process.Pid (negative ⇒ process group). cmd.Process non-nil in Cancel (os/exec
      contract). Do NOT create procgroup_windows.go (S2's job).

Task 2: CREATE internal/provider/executor.go — the platform-agnostic Execute
  - NO build tag. `package provider`. IMPORTS: "bytes","context","fmt","os/exec","strings","time".
  - IMPLEMENT func Execute(ctx, spec CmdSpec, timeout) (stdout, stderr string, err error) per the
      Data Models block: timeout context derivation (shadow + defer cancel) → CommandContext →
      stdin/stdout/stderr/env wiring → setupProcessGroup(cmd) → Start()+Wait() → ctx.Err()-first
      disambiguation.
  - GOTCHA: NO syscall import. ctx.Err() checked BEFORE the exit-error wrap. Stdin/Env guards
      (non-empty). No cmd.Dir. Do NOT import internal/config or internal/git.

Task 3: CREATE internal/provider/executor_test.go — the ~8 test groups (see Test Specs)
  - NO build tag. `package provider`. IMPORTS: "bytes","context","errors","os","os/exec","strings",
      "testing","time".
  - MIRROR repo test style (committree_test.go): stdlib testing, real binaries, t.Fatalf/t.Errorf,
      reflect.DeepEqual / bytes.Equal, t.Setenv for env. Add a mustBin(t, names...) LookPath+skip helper.
  - THE KEYSTONE: TestExecute_TimeoutKillsProcess (sleep 30 + 200ms ⇒ DeadlineExceeded + returns
      within seconds — proves the kill fires).

Task 4: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. render.go +
      manifest/merge/builtin/registry files MUST be byte-unchanged. `go test -race ./...` green.
```

### Test Specs (executor_test.go — ~8 groups)

```go
// All tests shell out to REAL Unix binaries (cat/sleep/printenv/false), guarded by mustBin(t, ...).
// CI matrix is Linux/macOS where these are universal; mustBin t.Skip's if absent.

// mustBin skips the test if any named binary is not resolvable on PATH.
func mustBin(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			t.Skipf("required binary %q not in PATH: %v", n, err)
		}
	}
}

// 1. Normal run: `cat` echoes stdin to stdout verbatim.
func TestExecute_CatEchoesStdin(t *testing.T) {
	mustBin(t, "cat")
	spec := CmdSpec{Command: "cat", Args: nil, Stdin: "hello world\n", Env: os.Environ()}
	out, _, err := Execute(context.Background(), spec, 0)
	if err != nil { t.Fatalf("Execute: err = %v, want nil", err) }
	if out != "hello world\n" { t.Errorf("stdout = %q, want %q", out, "hello world\n") }
}

// 2. Large stdin: 1MiB round-trips byte-for-byte (proves stdin piping, not a tiny buffer).
func TestExecute_LargeStdin(t *testing.T) {
	mustBin(t, "cat")
	payload := strings.Repeat("x", 1<<20) // 1 MiB
	spec := CmdSpec{Command: "cat", Stdin: payload, Env: os.Environ()}
	out, _, err := Execute(context.Background(), spec, 30*time.Second)
	if err != nil { t.Fatalf("Execute: err = %v, want nil", err) }
	if out != payload { t.Errorf("large stdin: len(stdout) = %d, want %d", len(out), len(payload)) }
}

// 3. THE KEYSTONE — timeout kills the process: `sleep 30` + 200ms timeout ⇒ DeadlineExceeded,
//    AND Execute returns within seconds (not 30s) — proving cmd.Cancel fired and killed the group.
func TestExecute_TimeoutKillsProcess(t *testing.T) {
	mustBin(t, "sleep")
	spec := CmdSpec{Command: "sleep", Args: []string{"30"}, Stdin: "", Env: os.Environ()}
	start := time.Now()
	_, _, err := Execute(context.Background(), spec, 200*time.Millisecond)
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Execute took %v; the process-group kill should have fired within ~3s (WaitDelay)", elapsed)
	}
}

// 4. Parent-context cancel ⇒ context.Canceled (distinguished from timeout).
func TestExecute_ParentContextCancel(t *testing.T) {
	mustBin(t, "sleep")
	ctx, cancel := context.WithCancel(context.Background())
	spec := CmdSpec{Command: "sleep", Args: []string{"30"}, Env: os.Environ()}
	go func() { time.Sleep(150 * time.Millisecond); cancel() }()
	_, _, err := Execute(ctx, spec, 0) // no timeout; rely on parent cancel
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// 5. stderr captured SEPARATELY + non-zero exit surfaced: `cat /nonexistent` writes to stderr, exits 1.
func TestExecute_StderrCaptureAndNonZeroExit(t *testing.T) {
	mustBin(t, "cat")
	spec := CmdSpec{Command: "cat", Args: []string{"/nonexistent/path/xyz"}, Env: os.Environ()}
	out, errb, err := Execute(context.Background(), spec, 5*time.Second)
	if err == nil { t.Fatal("err = nil, want non-nil (non-zero exit)") }
	if errb == "" { t.Errorf("stderr = empty, want a 'No such file' message") }
	if out != "" { t.Errorf("stdout = %q, want empty", out) }
}

// 6. Env propagation: a manifest/env var reaches the child (`printenv VAR` prints its value).
func TestExecute_EnvPropagation(t *testing.T) {
	mustBin(t, "printenv")
	env := append(os.Environ(), "STAGECOACH_TEST_VAR=s3cr3t")
	spec := CmdSpec{Command: "printenv", Args: []string{"STAGECOACH_TEST_VAR"}, Env: env}
	out, _, err := Execute(context.Background(), spec, 5*time.Second)
	if err != nil { t.Fatalf("Execute: err = %v, want nil", err) }
	if strings.TrimSpace(out) != "s3cr3t" { t.Errorf("env var: stdout = %q, want %q", out, "s3cr3t") }
}

// 7. No stdin (positional/flag delivery): Stdin="" ⇒ no pipe ⇒ child gets /dev/null ⇒ cat exits 0
//    immediately with empty output (and does NOT hang).
func TestExecute_NoStdinDoesNotHang(t *testing.T) {
	mustBin(t, "cat")
	spec := CmdSpec{Command: "cat", Stdin: "", Env: os.Environ()}
	out, _, err := Execute(context.Background(), spec, 3*time.Second)
	if err != nil { t.Fatalf("Execute: err = %v, want nil", err) }
	if out != "" { t.Errorf("stdout = %q, want empty (no stdin)", out) }
}

// 8. Command not found ⇒ wrapped Start() error.
func TestExecute_CommandNotFound(t *testing.T) {
	spec := CmdSpec{Command: "definitely-not-a-real-binary-xyz-stagecoach", Env: os.Environ()}
	_, _, err := Execute(context.Background(), spec, 3*time.Second)
	if err == nil { t.Fatal("err = nil, want non-nil (command not found)") }
	if !strings.Contains(err.Error(), "start") && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "executable") {
		t.Errorf("err = %v, want it to mention start/not found/executable", err)
	}
}

// (optional) 9. timeout=0 with a clean run: `false` exits 1 ⇒ non-nil wrapped error (exit-failure path).
func TestExecute_NonZeroExit(t *testing.T) {
	mustBin(t, "false")
	spec := CmdSpec{Command: "false", Env: os.Environ()}
	out, _, err := Execute(context.Background(), spec, 0)
	if err == nil { t.Fatal("err = nil, want non-nil (`false` exits 1)") }
	if out != "" { t.Errorf("stdout = %q, want empty", out) }
}
```

> **Note on Test 4 (parent cancel):** a background goroutine cancels the parent ctx after 150ms. The
> `Execute(ctx, spec, 0)` call (no timeout) sees ctx cancelled → cmd.Cancel SIGTERMs the group →
> `sleep` dies → `cmd.Wait()` errors → `ctx.Err()` is `context.Canceled`. This proves the
> signal-handler integration path (P1.M4.T2.S1 will cancel the SAME ctx on SIGINT).

### Implementation Patterns & Key Details

```go
// THE ctx.Err()-first disambiguation — the heart of the error contract. Without this order a timeout
// is misreported as a generic exit error and the orchestrator can't fire exit-124 + rescue.
if werr := cmd.Wait(); werr != nil {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return out.String(), errb.String(), ctxErr // timeout → DeadlineExceeded; cancel → Canceled
	}
	return out.String(), errb.String(), fmt.Errorf("provider %q: %w", spec.Command, werr)
}

// THE timeout-context derivation (shadow is load-bearing — the later ctx.Err() reads THIS ctx).
if timeout > 0 {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()
}

// THE process-group seam — executor.go stays portable; all syscall lives in the build-tagged file.
setupProcessGroup(cmd)
//   (procgroup_unix.go) cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
//                       cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
//                       cmd.WaitDelay = 3 * time.Second
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. Execute + setupProcessGroup use ONLY stdlib. `go mod tidy` MUST be a no-op.
        `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - internal/provider → (stdlib: bytes, context, fmt, os/exec, strings, time) ONLY for executor.go;
        + (os/exec, syscall, time) for procgroup_unix.go. It does NOT import internal/config (cycle;
        the orchestrator passes cfg.Timeout as a Duration) or internal/git (different layer).

BUILD TAGS (NEW to the codebase — this subtask establishes the pattern):
  - procgroup_unix.go: `//go:build !windows` (matches Linux + macOS + darwin — the entire CI matrix).
  - executor.go + executor_test.go: NO build tag (portable; tests run on Unix CI).
  - Cross-compile to Windows (`GOOS=windows go build ./...`) WILL fail with `undefined:
        setupProcessGroup` until P1.M2.T5.S2 lands procgroup_windows.go — this is CORRECT and expected
        (Windows support is S2's deliverable; CI is Linux/macOS per PRD §20.4).

FROZEN FILES (do NOT edit):
  - internal/provider/render.go + render_test.go (T4): CmdSpec {Command, Args, Stdin, Env} is a
        CONTRACT (Execute consumes it; the parser/orchestrator/verbose-CLI depend on it downstream).
  - internal/provider/manifest.go/merge.go/builtin.go/registry.go + their tests (S1–T3): untouched.
  - internal/config/*, internal/git/*, cmd/stagecoach/main.go, Makefile.

DOWNSTREAM CONTRACTS (do NOT implement here — just honor the shapes they will consume):
  - P1.M2.T6 (parser): consumes the returned stdout string (raw agent output) — Execute's stdout is
        the parser's input.
  - P1.M3.T4 (CommitStaged orchestrator): stdout, stderr, err := Execute(ctx, *spec, cfg.Timeout);
        errors.Is(err, context.DeadlineExceeded) ⇒ exit 124 + rescue; errors.Is(err,
        context.Canceled) ⇒ exit 3 + rescue; other err ⇒ retry then rescue; nil ⇒ parse stdout.
  - P1.M4.T2.S1 (signal handler): installs signal.Notify for SIGINT/SIGTERM; on receipt cancels the
        SAME ctx passed to Execute → triggers Execute's context.Canceled path + cmd.Cancel group kill.
        (The executor does NOT install the handler — it only honors ctx cancellation.)
  - P1.M4.T3.S2 (verbose mode): prints the resolved command + the raw stdout/stderr for debugging.
  => Execute(...) + setupProcessGroup(cmd) signatures are FROZEN after this subtask. Do not change.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after creating the files — fix before proceeding
gofmt -w internal/provider/executor.go internal/provider/procgroup_unix.go internal/provider/executor_test.go
go vet ./internal/provider/                                            # catches unused imports (e.g. stray "syscall" in executor.go)
go build ./...                                                         # compiles the whole module (Linux/macOS host)

# go.mod/go.sum MUST be unchanged (stdlib only)
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"

# Build-tag sanity: the //go:build !windows line is present and first in procgroup_unix.go
head -1 internal/provider/procgroup_unix.go   # → "//go:build !windows"

# Expected: Zero errors. If `go vet` flags an unused import, remove it. If `go build` fails with
# `undefined: setupProcessGroup`, procgroup_unix.go's build tag or function name is wrong.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The executor + its tests
go test -race ./internal/provider/ -run 'Execute' -v

# The full provider suite (S1/S2/S3 + registry + render + new executor tests — all must stay green)
go test -race ./internal/provider/ -v

# Expected: all green. THE KEYSTONE TestExecute_TimeoutKillsProcess MUST return
# context.DeadlineExceeded AND complete in <5s (proving the group kill fired). If it hangs ~30s, the
# cmd.Cancel/WaitDelay/Setpgid wiring is wrong (re-read research §1 + §3). If the env test fails, the
# cmd.Env guard swallowed a populated Env (re-read design call 5).
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-module test suite (provider + config + git + cmd) — nothing else may regress
go test -race ./...

# Coverage for the executor (informational; the Makefile coverage gate ≥85% is a P1.M5 concern)
go test -coverprofile=coverage.out ./internal/provider/ && go tool cover -func=coverage.out | grep -E 'executor.go|Execute'

# Expected: `go test -race ./...` green; executor.go Execute covered (happy path + timeout + cancel +
# non-zero-exit + start-failure all exercised by the ~8 test groups). procgroup_unix.go's
# setupProcessGroup is exercised transitively by every Execute test.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Manual kill-semantics reasoning check (the grandchild-kill is NOT unit-tested — research §5.4):
#   - Setpgid:true ⇒ child PGID == PID.
#   - cmd.Cancel ⇒ syscall.Kill(-PID, SIGTERM) ⇒ the NEGATIVE pid targets the whole group (child +
#     grandchildren). Verified against go_ecosystem_patterns §3.4 + critical_findings FINDING 8.
#   - cmd.WaitDelay = 3s ⇒ if the group ignores SIGTERM, Go SIGKILLs after 3s.
# (The TestExecute_TimeoutKillsProcess test empirically confirms the kill fires within ~3s.)

# Cross-platform seam check (DO NOT fix this failure — it is correct until S2 lands):
GOOS=windows go build ./internal/provider/ 2>&1 | grep -q 'undefined: setupProcessGroup' \
  && echo "Windows build correctly defers to S2 (procgroup_windows.go) ✓" \
  || echo "UNEXPECTED: Windows build succeeded without procgroup_windows.go — investigate"
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed; `go test -race ./...` green.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/` clean.
- [ ] `go mod tidy` is a no-op; `git diff --exit-code go.mod go.sum` empty (stdlib only).
- [ ] Every frozen file byte-unchanged (`git diff --exit-code internal/provider/render.go
      internal/provider/manifest.go internal/provider/merge.go internal/provider/builtin.go
      internal/provider/registry.go` and their `_test.go`).
- [ ] `//go:build !windows` is the first line of procgroup_unix.go; executor.go has NO build tag and
      NO `syscall` import.

### Feature Validation

- [ ] All success criteria from "What" section met.
- [ ] `TestExecute_TimeoutKillsProcess` passes — `context.DeadlineExceeded` AND completes <5s.
- [ ] `TestExecute_ParentContextCancel` passes — `context.Canceled` (signal/parent path).
- [ ] `TestExecute_CatEchoesStdin` + `TestExecute_LargeStdin` pass — stdin piping verified.
- [ ] `TestExecute_StderrCaptureAndNonZeroExit` passes — separate stderr + wrapped exit error.
- [ ] `TestExecute_EnvPropagation` passes — manifest env var reaches the child.
- [ ] `TestExecute_NoStdinDoesNotHang` + `TestExecute_CommandNotFound` pass.

### Code Quality Validation

- [ ] Follows existing codebase patterns (separate buffers, CommandContext, ctx-aware errors — see
      internal/git/git.go's run(); stdlib testing with real binaries — see committree_test.go).
- [ ] File placement matches the desired codebase tree (executor.go + procgroup_unix.go + test).
- [ ] executor.go is platform-agnostic (no syscall); the cross-platform seam is setupProcessGroup.
- [ ] Anti-patterns avoided (no mocking framework; no cmd.Run()/CombinedOutput; no inline syscall in
      executor.go; no exit-code-as-nil-error like git's run(); no signal.Notify in the executor).

### Documentation & Deployment

- [ ] One doc comment per exported identifier (`Execute`) + a contract note on `setupProcessGroup`
      (citing PRD §9.5/§18.2/§18.4 + the frozen signature for S2).
- [ ] No new environment variables or config (timeout comes from the resolved Config.Timeout).
- [ ] No user-facing docs (PRD "DOCS: none — internal executor"); verbose-mode docs come with P1.M4.T3.S2.

---

## Anti-Patterns to Avoid

- ❌ Don't inline `syscall` into executor.go — it must stay portable; the seam is `setupProcessGroup`.
- ❌ Don't check the exit error BEFORE `ctx.Err()` — a timeout would be misreported (design call 2).
- ❌ Don't rename the shadowed `ctx` in the timeout block — the shadow is load-bearing (design call 3).
- ❌ Don't use `cmd.Run()` / `CombinedOutput()` — they can't return separate buffers + error (design call 4).
- ❌ Don't copy git's "non-zero exit ⇒ nil error" convention — an agent non-zero exit is a FAILURE (wrap it).
- ❌ Don't install `signal.Notify` in the executor — that's the CLI signal handler's job (P1.M4.T2.S1).
- ❌ Don't create `procgroup_windows.go` — that's S2's deliverable (creating it now steals S2's scope).
- ❌ Don't introduce a mocking framework — the codebase tests with real binaries (committree_test.go).
- ❌ Don't set `cmd.Dir` — the agent runs in the user's CWD (git uses `-C repo`; the agent does not).
- ❌ Don't skip the timeout test's elapsed-time assertion — it's the proof the group kill actually fired.

---

## Confidence Score

**9/10** for one-pass implementation success. The canonical os/exec pattern is pre-verified in the
repo's own architecture doc (§3) + critical_findings (FINDING 8/10); the CmdSpec input contract is
FROZEN and already landed (render.go); the cross-platform split is unambiguous (build-tag files, one
frozen signature); the test strategy uses universally-present binaries matching the codebase's real-
binary testing idiom. The one residual risk — the grandchild-kill is verified by code review + the
prompt-kill timing assertion rather than a direct grandchild test — is documented and accepted
(research §5.4).
