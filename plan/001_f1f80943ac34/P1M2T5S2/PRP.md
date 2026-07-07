---
name: "P1.M2.T5.S2 — Cross-platform process-group kill abstraction: the Windows half (//go:build windows) of setupProcessGroup — PRD §18.4 / §20.4 / §21.2 + critical_findings FINDING 10"
description: |

  Land the SECOND subtask of Provider Executor (P1.M2.T5): the Windows implementation of the
  cross-platform `setupProcessGroup(cmd *exec.Cmd)` seam that S1 (P1.M2.T5.S1) froze. S1 created
  `internal/provider/procgroup_unix.go` (`//go:build !windows`) with `func setupProcessGroup(cmd
  *exec.Cmd)` using `SysProcAttr{Setpgid:true}` + `cmd.Cancel = syscall.Kill(-pid, SIGTERM)` +
  `cmd.WaitDelay = 3s`, and a platform-AGNOSTIC `executor.go` that calls it with no import (same
  package). S1's PRP explicitly asserted `GOOS=windows go build ./...` FAILS (`undefined:
  setupProcessGroup`) UNTIL S2 lands procgroup_windows.go. **S2 IS the file that flips that failure
  to success.** This implements critical_findings FINDING 10 (Windows `Setpgid` is Unix-only) so the
  process-group kill (PRD §18.4 — SIGINT/SIGTERM → kill the whole child tree) works on Windows
  amd64+arm64 (PRD §21.2), and the CI matrix (PRD §20.4: linux/macos/windows × amd64/arm64) can
  build+test the Windows leg.

  INPUT: the FROZEN seam `func setupProcessGroup(cmd *exec.Cmd)` declared+used in
  `internal/provider/executor.go` (S1) and implemented in `internal/provider/procgroup_unix.go` (S1).
  Treat S1's PRP + the procgroup_unix.go signature as a CONTRACT.

  OUTPUT: `internal/provider/procgroup_windows.go` (`//go:build windows`) implementing the IDENTICAL
  signature via the Windows analog of process-group kill: `CREATE_NEW_PROCESS_GROUP`
  (`syscall.SysProcAttr.CreationFlags`) → child = console process-group leader (PID==PGID); `cmd.Cancel`
  → `GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, cmd.Process.Pid)` signals the whole group on ctx
  cancel; `cmd.WaitDelay = 3s` → os/exec escalation. Consumed transitively by the executor (S1's
  `Execute` calls `setupProcessGroup`) and the signal handler (P1.M4.T2 cancels ctx → cmd.Cancel).

  ⚠️ **THE central reconciliation — follow S1's LIVE contract, NOT the stale work-item text.** The
  work-item DESCRIPTION sketches `internal/provider/executil/` with TWO functions
  (`SetProcessGroup` + `KillProcessGroup`). S1 (being implemented in parallel) DIVERGED and shipped
  ONE function `setupProcessGroup(cmd *exec.Cmd)` in `internal/provider/` (same package as executor.go),
  with the platform kill encapsulated in `cmd.Cancel` (not a separate KillProcessGroup). S2 must
  implement S1's signature — do NOT create an `executil` package or `SetProcessGroup`/`KillProcessGroup`
  (that would duplicate/conflict with S1). See research §0.

  ⚠️ **THE second call — CTRL_BREAK_EVENT, NOT CTRL_C_EVENT.** `GenerateConsoleCtrlEvent(CTRL_C_EVENT,
  pid)` is a BROADCAST to every process sharing the caller's console (cannot be limited to one group —
  Microsoft docs); it would interrupt Stagecoach itself. `CTRL_BREAK_EVENT` honors `dwProcessGroupId`
  → targets ONLY the child's group. The work item correctly specifies CTRL_BREAK_EVENT. See research §2.

  ⚠️ **THE third call — CREATE_NEW_PROCESS_GROUP but NOT CREATE_NEW_CONSOLE.** `GenerateConsoleCtrlEvent`
  reaches ONLY processes attached to the caller's console. The default (no CREATE_NEW_CONSOLE) lets the
  child inherit Stagecoach's console → CTRL_BREAK reaches it. Setting CREATE_NEW_CONSOLE would silently
  break the kill. See research §3.

  ⚠️ **THE fourth call — stdlib `syscall.LazyProc`, NOT `golang.org/x/sys/windows`.**
  `GenerateConsoleCtrlEvent` is not in stdlib `syscall` (verified). Option A (`x/sys/windows`) would
  MODIFY go.mod/go.sum even for a build-tagged file (`go mod tidy` resolves across all build configs).
  Option B (`syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent").Call(...)`) is
  stdlib-only → go.mod/go.sum byte-unchanged, matching S1's stdlib-only principle. The constants
  `CREATE_NEW_PROCESS_GROUP`/`CTRL_BREAK_EVENT` ARE stdlib. CHOOSE Option B. See research §4.

  ⚠️ **THE fifth call — validation is CROSS-COMPILE + Windows CI, NOT local unit tests.** The dev box
  is Linux; `procgroup_windows.go` is never compiled locally (build tag). The keystone check is
  `GOOS=windows GOARCH=amd64 go build ./...` SUCCEEDS (was failing pre-S2). Plus `GOARCH=arm64`, `GOOS=
  windows go vet`, unchanged local `go test -race ./...`, byte-unchanged go.mod/go.sum. See research §6.

  Deliverable: `internal/provider/procgroup_windows.go` (`package provider`, `//go:build windows`,
  imports `os/exec`/`syscall`/`time`) — the Windows `setupProcessGroup`. OPTIONALLY
  `internal/provider/procgroup_windows_test.go` (`//go:build windows`) — a deterministic structural
  check for the Windows CI leg (S1's build-tag-less executor_test.go skips most cases on Windows due
  to absent Unix binaries). Touches ONLY these new file(s) — NO go.mod/go.sum change (stdlib only),
  NO edit to executor.go, procgroup_unix.go, or any frozen file.

---

## Goal

**Feature Goal**: Close the cross-platform gap S1 deliberately left open: provide the Windows
implementation of the `setupProcessGroup(cmd *exec.Cmd)` seam so the whole-child-tree kill on ctx
cancel (PRD §18.4) works on Windows, `GOOS=windows go build ./...` succeeds (today it fails with
`undefined: setupProcessGroup`), and the Windows legs of the CI matrix (PRD §20.4: windows ×
amd64/arm64) can build and test the provider package. This is the Windows analog of S1's Unix
`Setpgid`+`syscall.Kill(-pid)` recipe, expressed with Windows console process groups
(`CREATE_NEW_PROCESS_GROUP` + `GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT)`).

**Deliverable**:
1. **CREATE** `internal/provider/procgroup_windows.go` (`package provider`, `//go:build windows`,
   imports `os/exec`, `syscall`, `time`) — `func setupProcessGroup(cmd *exec.Cmd)`: set
   `cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}`; set
   `cmd.Cancel` to call `GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, cmd.Process.Pid)` via a package-level
   `syscall.LazyProc` (kernel32); set `cmd.WaitDelay = 3 * time.Second`. Identical signature to
   procgroup_unix.go.
2. **CREATE (recommended)** `internal/provider/procgroup_windows_test.go` (`package provider`,
   `//go:build windows`, imports `os`, `os/exec`, `syscall`, `testing`, `time`) — a structural test
   asserting the wiring (SysProcAttr.CreationFlags == CREATE_NEW_PROCESS_GROUP; cmd.Cancel != nil;
   cmd.WaitDelay == 3s) so the Windows CI leg has a deterministic, dependency-free check.

No other files touched. **No go.mod/go.sum change** (stdlib `syscall.LazyProc` only — Option B). NO
edit to `executor.go` (already calls `setupProcessGroup`; works on Windows once this file exists),
`procgroup_unix.go`, `render.go`, or any frozen manifest/config/git file.

**Success Definition**: `GOOS=windows GOARCH=amd64 go build ./...` and `GOOS=windows GOARCH=arm64 go
build ./...` both SUCCEED (the `undefined: setupProcessGroup` error is gone); `GOOS=windows go vet
./internal/provider/` clean (incl. the windows test file); on the Linux dev box `go test -race ./...`
remains fully green (procgroup_windows.go is excluded by build tag → zero regressions) and
`gofmt -l internal/provider/` is clean; go.mod/go.sum byte-unchanged (`git diff --exit-code` empty);
every frozen file byte-unchanged; the Windows CI leg builds the provider package and the structural
test passes.

## User Persona

**Target User**: The provider executor (P1.M2.T5.S1 — `Execute` calls `setupProcessGroup(cmd)`, which
on Windows now resolves to THIS file) and transitively the signal handler (P1.M4.T2 — cancels ctx →
os/exec fires `cmd.Cancel` → `GenerateConsoleCtrlEvent`) and the generate orchestrator (P1.M3.T4 —
maps the returned `context.Canceled`/`DeadlineExceeded` to exit 3/124 + rescue). End-user persona is
"the plan-holder" / "the API-key refusenik" on Windows (PRD §7, §21.2 Scoop install path) running any
of the 6 verified agent CLIs (pi, claude, gemini, opencode, codex, cursor) via Stagecoach.

**Use Case**: On Windows, when an agent run times out (PRD FR25) or the user hits Ctrl-C (§18.4), the
signal handler / timeout cancels the ctx passed to `Execute`; os/exec invokes `cmd.Cancel`; THIS file's
Cancel closure sends `CTRL_BREAK_EVENT` to the child's console process group, terminating the child
and its grandchildren (no orphaned model-API-calling helpers leaking cost). The orchestrator then
enters the rescue path (§18.2: exit 124 timeout / exit 3 signal).

**User Journey**: (internal API, no new end-user surface) `Execute(ctx, *spec, timeout)` → (Windows)
`setupProcessGroup(cmd)` sets CreationFlags + Cancel + WaitDelay → on ctx cancel → `cmd.Cancel` →
`GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, pid)` → child group dies → `cmd.Wait()` errors →
`ctx.Err()==context.Canceled` → Execute returns `context.Canceled` → orchestrator exit 3 + rescue.

**Pain Points Addressed**: Removes the Windows "orphaned grandchildren on timeout/Ctrl-C" hazard and
the "package won't cross-compile to Windows" gap S1 intentionally deferred. Windows users get the same
whole-tree kill guarantee as Unix users (modulo the documented console-attachment limitation, research §3).

## Why

- **Completes the cross-platform seam S1 froze.** S1 landed `setupProcessGroup` for Unix only and
  documented that `GOOS=windows go build ./...` fails until S2 ships (S1 PRP "Integration Points" +
  "Level 4"). S2 is precisely the file that resolves it — without it, Windows is a dead build.
- **Satisfies PRD §21.2 (Windows × amd64/arm64 binaries) and §20.4 (Windows in the CI matrix).** The
  goreleaser archives list `windows × amd64/arm64`; the CI matrix builds+tests on `windows`. Both are
  unreachable until the provider package compiles on Windows — which requires this abstraction's
  Windows half (FINDING 10).
- **Implements the safety invariant (§18.4) on Windows.** "Kill the child and its descendants on
  signal/timeout" must work cross-platform or Windows users leak agent subprocesses (cost + zombies).
- **Encapsulates the platform kill in `cmd.Cancel` — no per-platform signal-handler code.** Because
  the kill lives in `cmd.Cancel` (set by `setupProcessGroup`), the future signal handler (P1.M4.T2)
  just cancels the ctx uniformly on both platforms; it needs no `if runtime.GOOS == "windows"` branch.
- **No new user-facing surface** (PRD "DOCS: none — internal abstraction"). No new dependency (stdlib
  LazyProc, Option B) — go.mod/go.sum stay byte-unchanged, matching S1.

## What

A compiled `internal/provider` package whose `setupProcessGroup` is now defined on BOTH Unix
(`procgroup_unix.go`, S1) and Windows (`procgroup_windows.go`, S2) via mutually-exclusive build tags
(`!windows` vs `windows`), each implementing the identical `func setupProcessGroup(cmd *exec.Cmd)`
signature. `executor.go` (platform-agnostic, S1) is unchanged and resolves the call to the right file
per GOOS at compile time. No new exports, no new types, no parsing, no CLI, no config edits.

### Success Criteria

- [ ] `internal/provider/procgroup_windows.go` exists, `package provider`, FIRST LINE
      `//go:build windows` (then blank line, then package clause — gofmt-enforced), imports EXACTLY
      `os/exec`, `syscall`, `time`. Defines `func setupProcessGroup(cmd *exec.Cmd)` setting
      `SysProcAttr{CreationFlags: CREATE_NEW_PROCESS_GROUP}`, a `cmd.Cancel` that calls
      `GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, cmd.Process.Pid)` via `syscall.LazyProc`, and
      `cmd.WaitDelay = 3 * time.Second`.
- [ ] `GOOS=windows GOARCH=amd64 go build ./...` SUCCEEDS (the keystone — was failing pre-S2).
- [ ] `GOOS=windows GOARCH=arm64 go build ./...` SUCCEEDS (PRD §21.2 arm64).
- [ ] `GOOS=windows go vet ./internal/provider/` clean (compiles incl. the windows test file).
- [ ] `gofmt -l internal/provider/` clean; on Linux `go test -race ./...` fully green (no regressions —
      the windows file is build-tagged out on Linux).
- [ ] go.mod/go.sum byte-unchanged (`git diff --exit-code go.mod go.sum` empty) — Option B (stdlib).
- [ ] Every frozen file byte-unchanged: `executor.go`, `procgroup_unix.go`, `render.go`,
      `manifest.go`/`merge.go`/`builtin.go`/`registry.go` + tests, `internal/config/*`, `internal/git/*`.
- [ ] (recommended) `internal/provider/procgroup_windows_test.go` exists (`//go:build windows`) and
      asserts the structural wiring (CreationFlags == CREATE_NEW_PROCESS_GROUP; Cancel != nil;
      WaitDelay == 3s); compiles via `GOOS=windows go vet`.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the S1↔S2
contract reconciliation (research §0 — the most important read), the Windows mechanism + the 5 design
calls (research §1–§4 + the copy-ready reference in §5), the validation strategy for a build-tagged
file that can't run locally (research §6), and the frozen `setupProcessGroup` signature in S1's
procgroup_unix.go (research §0 / the file itself). No parsing/generate/CLI/signal-handler knowledge
required — S2 is a single platform-specific file implementing an already-frozen one-function seam.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M2T5S2/research/windows-procgroup-kill.md
  why: the SINGLE most important read — the S1↔S2 contract reconciliation (§0 — why S2 follows S1's
       setupProcessGroup signature, NOT the work-item's SetProcessGroup/KillProcessGroup sketch), the
       Windows mechanism (§1), CTRL_BREAK vs CTRL_C (§2), the console-sharing gotcha (§3), the stdlib-
       LazyProc dependency decision (§4), the COPY-READY reference implementation (§5), and the
       cross-compile validation strategy (§6).
  critical: §0 (do NOT create executil/ or SetProcessGroup/KillProcessGroup — follow S1) and §2
       (CTRL_BREAK not CTRL_C) and §4 (stdlib LazyProc not x/sys/windows) are the things most likely
       to be implemented wrong.

- file: plan/001_f1f80943ac34/P1M2T5S1/PRP.md   (the PREVIOUS item — S1 is the contract; read, do NOT edit)
  section: "Implementation Blueprint → Data models" (the procgroup_unix.go block) + "Integration Points"
           (BUILD TAGS — the FROZEN signature + the assertion that GOOS=windows build FAILS until S2).
  why: S1 froze `func setupProcessGroup(cmd *exec.Cmd)` and documented that the Windows build is
       S2's deliverable. S2 implements the IDENTICAL signature. The procgroup_unix.go block is the
       EXACT pattern to mirror (signature, doc-comment style, field-by-field: SysProcAttr → Cancel →
       WaitDelay) with the Windows mechanism substituted.
  critical: the signature is FROZEN. `executor.go` (platform-agnostic, no syscall) calls
       `setupProcessGroup(cmd)` — do NOT change executor.go or procgroup_unix.go; S2 adds ONE file.

- file: internal/provider/procgroup_unix.go   (S1 — read for the EXACT signature + doc-comment style; do NOT edit)
  why: the file S2 parallels. Its `func setupProcessGroup(cmd *exec.Cmd)` sets three cmd fields
       (SysProcAttr{Setpgid:true}; cmd.Cancel = group SIGTERM; cmd.WaitDelay = 3s). S2's file sets the
       same three fields with the Windows mechanism (SysProcAttr{CreationFlags:CREATE_NEW_PROCESS_GROUP};
       cmd.Cancel = GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, pid); cmd.WaitDelay = 3s).
  critical: the SIGNATURE (`func setupProcessGroup(cmd *exec.Cmd)`) and the WAITDELAY value (3s) must
       match S1 exactly so behavior is uniform across platforms. The first line must be the build tag.

- file: internal/provider/executor.go   (S1 — read to confirm the call site; do NOT edit)
  section: the `setupProcessGroup(cmd)` call inside `Execute` (platform-agnostic; no syscall import).
  why: confirms executor.go needs NO change — it already calls setupProcessGroup; on a Windows build
       the linker resolves it to procgroup_windows.go. This is the proof S2 is a clean single-file add.
  critical: do NOT edit executor.go. If the Windows build still fails after adding procgroup_windows.go,
       the signature/build-tag/package-clause in the new file is wrong (re-check §5).

- url: https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent
  why: the AUTHORITATIVE semantics — "Generates a CTRL+C signal. This signal cannot be limited to a
       specific process group" (→ use CTRL_BREAK for a targeted group); dwProcessGroupId = the child's
       PID (== its group id post CREATE_NEW_PROCESS_GROUP); returns 0 on failure (GetLastError).
  critical: confirms CTRL_BREAK_EVENT (not CTRL_C) and that the second arg is the group id == child PID.

- url: https://learn.microsoft.com/en-us/windows/console/console-process-groups
  why: confirms a process created with CREATE_NEW_PROCESS_GROUP is a console process-group leader and
       that GenerateConsoleCtrlEvent delivers ONLY to processes sharing the caller's console (→ do NOT
       add CREATE_NEW_CONSOLE; the child must inherit Stagecoach's console).
  critical: the console-sharing requirement is the silent failure mode (research §3).

- url: https://github.com/golang/go/issues/17608
  why: golang/go #17608 "syscall: add support for Windows job objects" — the robust whole-tree-kill
       mechanism (Job Objects kill all descendants on job close, even if they ignore CTRL_BREAK or
       detach from the console). Documented as the FUTURE hardening beyond v1; S2 uses the work-item's
       specified CREATE_NEW_PROCESS_GROUP + GenerateConsoleCtrlEvent approach for v1.
  critical: cite this in the doc comment's "known limitation" so a future contributor knows the upgrade
       path and the v1 limitation is honestly recorded (PRD §12.7.2).

- file: plan/001_f1f80943ac34/architecture/critical_findings.md   (READ FINDING 8 + FINDING 10)
  section: "FINDING 8: Signal handling requires process-group kill (Setpgid)" + "FINDING 10: Windows
           SysProcAttr.Setpgid is Unix-only".
  why: FINDING 10 pins the build-tag split (exec_unix.go !windows + exec_windows.go windows — here
       realized as procgroup_unix.go / procgroup_windows.go per S1's in-package decision). FINDING 8
       pins the cmd.Cancel + WaitDelay + group-kill recipe S1 implemented for Unix; S2 provides the
       Windows analog.
  critical: FINDING 10 names a hypothetical internal/executil/ location, but S1 (the live contract)
       keeps the abstraction in internal/provider (same package as executor.go). Follow S1.

- file: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md   (READ §3.4 + §3.5)
  section: "3.4 Key Safety Notes" (the cross-platform row: Setpgid Unix-only → Windows needs Job
           Objects) + "3.5 Setpgid and Signal Forwarding Interaction".
  why: §3.4 states the Windows Job-Objects alternative (v2 hardening); §3.5 explains why the parent
       must forward signals (the child in its own group won't get terminal Ctrl-C) — under S1's design
       that forwarding happens via cmd.Cancel on ctx cancel, not a direct kill call.
  critical: do NOT take §3's Unix code wholesale — it inlines syscall into a single file. S1 already
       split it (executor.go platform-agnostic + procgroup_*.go build-tagged). S2 adds only the windows file.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; require go-toml/v2 + pflag  (UNCHANGED — S2 adds NO dep: stdlib LazyProc)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — untouched
  git/                          # P1.M1.T2/T3 — untouched
  provider/                     # S1..T5.S1 created; this subtask adds the Windows half of the seam
    manifest.go / manifest_test.go          # S1                  (CONTRACT — do NOT edit)
    merge.go / merge_test.go                # S2 (P1.M2.T2)       (do NOT edit)
    builtin.go / builtin_test.go            # P1.M2.T2/S3         (do NOT edit)
    registry.go / registry_test.go          # P1.M2.T3            (do NOT edit)
    render.go / render_test.go              # T4 — CmdSpec + Render  (FROZEN CONTRACT — do NOT edit)
    executor.go                             # T5.S1 — Execute (platform-agnostic; no syscall)  (do NOT edit)
    procgroup_unix.go                       # T5.S1 — setupProcessGroup [//go:build !windows]  (FROZEN — do NOT edit)
    executor_test.go                        # T5.S1 — ~8 test groups (no build tag; runs on Unix CI)  (do NOT edit)
    procgroup_windows.go                    # NEW (this subtask) ← setupProcessGroup [//go:build windows]
    procgroup_windows_test.go               # NEW (this subtask, recommended) ← structural test [//go:build windows]
cmd/stagecoach/main.go           # stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  provider/
    procgroup_windows.go         # NEW — func setupProcessGroup(cmd *exec.Cmd)  [//go:build windows]
    procgroup_windows_test.go    # NEW (recommended) — structural wiring test    [//go:build windows]
# executor.go, procgroup_unix.go, executor_test.go (all S1) UNCHANGED.
# render.go + manifest/merge/builtin/registry UNCHANGED. go.mod/go.sum UNCHANGED. Every other file UNCHANGED.
# After S2: `GOOS=windows go build ./...` succeeds (the seam is now satisfied on both platforms).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (reconciliation — do NOT create executil/ or SetProcessGroup/KillProcessGroup): S1 (the
// LIVE contract, being implemented in parallel) froze `func setupProcessGroup(cmd *exec.Cmd)` in
// internal/provider/ (same package as executor.go), with the platform kill in cmd.Cancel. The work-
// item DESCRIPTION's sketch of internal/provider/executil/ + SetProcessGroup + KillProcessGroup is
// STALE. S2 implements S1's signature in procgroup_windows.go. Creating the old sketch would DUPLICATE
// the abstraction and conflict with S1. See research §0.

// CRITICAL (CTRL_BREAK not CTRL_C): GenerateConsoleCtrlEvent(CTRL_C_EVENT, pid) broadcasts to ALL
// processes sharing the caller's console (Microsoft docs) — it would interrupt Stagecoach itself and
// cannot be limited to the child's group. CTRL_BREAK_EVENT honors dwProcessGroupId → targets ONLY the
// child's group. Use syscall.CTRL_BREAK_EVENT (== 1). See research §2.

// CRITICAL (console-sharing — the silent failure): GenerateConsoleCtrlEvent reaches ONLY processes
// attached to the caller's console. Set CREATE_NEW_PROCESS_GROUP (child = group leader, still sharing
// the console) but do NOT set CREATE_NEW_CONSOLE (that would give the child its own console → CTRL_BREAK
// never reaches it → kill silently no-ops). The child inherits Stagecoach's console by default. See §3.

// CRITICAL (dependency — stdlib LazyProc, NOT x/sys/windows): GenerateConsoleCtrlEvent is NOT in stdlib
// syscall (verified). Importing golang.org/x/sys/windows WOULD modify go.mod/go.sum even in a build-
// tagged file (go mod tidy resolves across all build configs). Use syscall.NewLazyDLL("kernel32.dll").
// NewProc("GenerateConsoleCtrlEvent").Call(...) (stdlib, GOOS=windows) → go.mod byte-unchanged, matching
// S1's stdlib-only principle. The constants CREATE_NEW_PROCESS_GROUP / CTRL_BREAK_EVENT ARE stdlib.
// See research §4.

// GOTCHA (LazyProc.Call return contract): Call returns (r1, r2 uintptr, lastErr error). GenerateConsoleCtrlEvent
// returns 0 on failure (then lastErr = GetLastError). Check `if r1 == 0 { return err }`. On success return nil.
// (A failure here — e.g. child already dead — is non-fatal: WaitDelay escalates to TerminateProcess.)

// GOTCHA (cmd.Process is non-nil inside Cancel): os/exec invokes cmd.Cancel ONLY after Start() succeeds,
// so cmd.Process.Pid is safe to dereference in the closure. Mirrors S1's Unix Cancel closure exactly.

// GOTCHA (build-tag file layout): the //go:build line is the FIRST line; a blank line; then the package
// clause. gofmt enforces this. A missing/old-style `// +build` is NOT required for go 1.22. The package
// clause is `package provider` (NOT a new executil package).

// GOTCHA (validation is CROSS-COMPILE, not local test): procgroup_windows.go is never compiled on the
// Linux dev box (build tag). Local `go test -race ./...` MUST stay green but does NOT exercise this file.
// The keystone is `GOOS=windows GOARCH=amd64 go build ./...` succeeding. See research §6.

// GOTCHA (PID == process-group id): with CREATE_NEW_PROCESS_GROUP the child's PID IS its console
// process-group id — the exact parallel of Unix Setpgid (PGID==PID). So GenerateConsoleCtrlEvent(..., pid)
// targets the child's group; pass cmd.Process.Pid (NOT a negative pid — Windows has no -pid convention).

// GOTCHA (no grandchild-kill test): verifying CTRL_BREAK reaches GRANDCHILDREN needs a portable
// grandchild-spawning test undevelopable on Linux. NOT done (matches S1's accepted limitation). The
// structural test + cross-compile + code-review-vs-MS-docs establish correctness. See research §6.2.

// GOTCHA (Job Objects are the v2 hardening, NOT v1): a child that detaches from the console or swallows
// CTRL_BREAK can leave orphaned grandchildren (os/exec's WaitDelay escalation only TerminateProcess'es
// the direct child on Windows). Job Objects (golang/go #17608) fix this. DEFERRED beyond v1 — document
// honestly in the doc comment (PRD §12.7.2). The work item's specified v1 approach is CREATE_NEW_PROCESS_GROUP
// + GenerateConsoleCtrlEvent; implement that.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/provider/procgroup_windows.go
//go:build windows

package provider

import (
	"os/exec"
	"syscall"
	"time"
)

// procGenerateConsoleCtrlEvent resolves kernel32!GenerateConsoleCtrlEvent lazily. The function is not
// exported by Go's stdlib syscall package; resolving it via syscall.LazyProc (stdlib, GOOS=windows)
// keeps go.mod/go.sum byte-unchanged (no golang.org/x/sys dependency) — matching procgroup_unix.go's
// stdlib-only principle. Resolved on first Call; kernel32 is always present on Windows.
var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")

// setupProcessGroup is the Windows implementation of the cross-platform seam declared in executor.go
// and FROZEN by P1.M2.T5.S1 (`func setupProcessGroup(cmd *exec.Cmd)`). Identical signature to
// procgroup_unix.go's setupProcessGroup — only the platform mechanism differs (critical_findings
// FINDING 10; go_ecosystem_patterns §3.4). executor.go (platform-agnostic, no syscall) calls this with
// no import (same package); on a Windows build the linker selects THIS file.
//
// Windows has no POSIX process groups, no Setpgid, and no syscall.Kill(-pid). The analog of "kill the
// whole child tree on ctx cancel" (PRD §18.4) is the console process-group mechanism:
//
//   - CREATE_NEW_PROCESS_GROUP (0x00000200) in SysProcAttr.CreationFlags: the child becomes a console
//     process-group leader; its PID == its process-group id (the exact parallel of Unix Setpgid ⇒
//     PGID==PID). Do NOT also set CREATE_NEW_CONSOLE — the child must inherit the caller's console or
//     GenerateConsoleCtrlEvent will not reach it.
//   - cmd.Cancel: on ctx cancel (timeout OR signal/parent cancel) call GenerateConsoleCtrlEvent(
//     CTRL_BREAK_EVENT, childPID) to signal the WHOLE group. CTRL_BREAK (not CTRL_C) because
//     CTRL_C is broadcast to every process sharing the caller's console and cannot be limited to one
//     group (Microsoft GenerateConsoleCtrlEvent docs); CTRL_BREAK honors dwProcessGroupId.
//   - cmd.WaitDelay = 3 * time.Second: if the child ignores CTRL_BREAK, os/exec escalates after 3s
//     (on Windows it TerminateProcess'es the direct child).
//
// ERROR CONTRACT (platform-agnostic, inherited from executor.go): on ctx cancel cmd.Wait() errors and
// ctx.Err() == context.Canceled (parent/signal cancel) or context.DeadlineExceeded (timeout); Execute
// returns that sentinel → orchestrator exit 3 / 124 + rescue (§18.2). No change to executor.go.
//
// KNOWN LIMITATION (v1; PRD §12.7.2 — document honestly): if a child detaches from the console
// (e.g. CREATE_NEW_CONSOLE of its own) or installs a handler that swallows CTRL_BREAK, grandchildren
// may survive escalation (os/exec's WaitDelay escalation TerminateProcess'es only the direct child).
// The robust whole-tree kill is Job Objects (golang/go #17608), deferred beyond v1. The work item's
// specified v1 approach is CREATE_NEW_PROCESS_GROUP + GenerateConsoleCtrlEvent (this implementation).
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP, // child = console process-group leader; PID==PGID
	}
	cmd.Cancel = func() error {
		// cmd.Process is guaranteed non-nil inside Cancel (os/exec calls it only after Start() succeeds).
		// CTRL_BREAK_EVENT (1) targets the specific group == child PID (NOT a negative pid; Windows has no -pid).
		r1, _, err := procGenerateConsoleCtrlEvent.Call(
			uintptr(syscall.CTRL_BREAK_EVENT),
			uintptr(cmd.Process.Pid),
		)
		if r1 == 0 {
			// GenerateConsoleCtrlEvent failed (e.g. child already exited) — non-fatal: WaitDelay escalates
			// to TerminateProcess on the direct child. Return err so os/exec logs it (does not fail Wait).
			return err
		}
		return nil
	}
	cmd.WaitDelay = 3 * time.Second
}
```

```go
// internal/provider/procgroup_windows_test.go   (recommended; //go:build windows)
//go:build windows

package provider

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestSetupProcessGroup_Wiring is the deterministic Windows-side structural check. S1's
// executor_test.go has no build tag and runs on the Windows CI leg, but most of its cases shell out to
// Unix binaries (cat/sleep/printenv/false) absent on windows-2022 → mustBin skips them, leaving the
// Windows setupProcessGroup weakly exercised. This test asserts the wiring directly with no external
// binary — it runs only on Windows (build tag) and compiles via `GOOS=windows go vet`.
func TestSetupProcessGroup_Wiring(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "exit", "0") // cmd.exe is always present on Windows
	setupProcessGroup(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("cmd.SysProcAttr == nil; want non-nil")
	}
	if cmd.SysProcAttr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Errorf("CreationFlags = %#x; want CREATE_NEW_PROCESS_GROUP (%#x) bit set",
			cmd.SysProcAttr.CreationFlags, syscall.CREATE_NEW_PROCESS_GROUP)
	}
	if cmd.Cancel == nil {
		t.Error("cmd.Cancel == nil; want the GenerateConsoleCtrlEvent(CTRL_BREAK) closure")
	}
	if cmd.WaitDelay != 3*time.Second {
		t.Errorf("cmd.WaitDelay = %v; want 3s (match procgroup_unix.go)", cmd.WaitDelay)
	}
}
```

> **gofmt note:** run `gofmt -w internal/provider/procgroup_windows.go internal/provider/procgroup_windows_test.go`.
> The `//go:build windows` line MUST be the first line of each file, followed by a blank line, then
> `package provider` (gofmt enforces this).
>
> **Imports:** procgroup_windows.go = `os/exec`, `syscall`, `time` (NO `context`, NO `fmt`, NO `os`).
> procgroup_windows_test.go = `os/exec`, `syscall`, `testing`, `time` (add `os` only if used). Unused
> imports fail `GOOS=windows go vet` — add only what you use.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/provider/procgroup_windows.go — the Windows setupProcessGroup
  - FIRST LINE: `//go:build windows` then blank line then `package provider`.
  - DECLARE the package-level `var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").
      NewProc("GenerateConsoleCtrlEvent")` (stdlib LazyProc — NO x/sys dependency).
  - IMPLEMENT func setupProcessGroup(cmd *exec.Cmd) per the Data Models block: SysProcAttr{
      CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}; cmd.Cancel = the GenerateConsoleCtrlEvent(
      CTRL_BREAK_EVENT, cmd.Process.Pid) closure (check r1==0 → return err); cmd.WaitDelay = 3*time.Second.
  - IMPORTS: "os/exec", "syscall", "time".
  - GOTCHA: CTRL_BREAK_EVENT (not CTRL_C). Do NOT set CREATE_NEW_CONSOLE. PID (not -pid). The signature
      must EXACTLY match procgroup_unix.go's `func setupProcessGroup(cmd *exec.Cmd)`.

Task 2: CREATE internal/provider/procgroup_windows_test.go — the structural Windows check (recommended)
  - FIRST LINE: `//go:build windows` then blank line then `package provider`.
  - IMPLEMENT TestSetupProcessGroup_Wiring per the Data Models block: build a cmd (cmd.exe /c exit 0),
      call setupProcessGroup, assert CreationFlags has CREATE_NEW_PROCESS_GROUP bit, Cancel != nil,
      WaitDelay == 3*time.Second.
  - IMPORTS: "os/exec", "syscall", "testing", "time".
  - GOTCHA: this file is NEVER compiled on Linux (build tag) — verify it compiles via
      `GOOS=windows go vet ./internal/provider/`. It runs only on the Windows CI leg.

Task 3: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–4). go.mod/go.sum MUST be byte-unchanged (Option B).
      executor.go / procgroup_unix.go / render.go + manifest/merge/builtin/registry MUST be byte-unchanged.
      Local `go test -race ./...` MUST stay green. `GOOS=windows GOARCH={amd64,arm64} go build ./...`
      MUST succeed (the keystone — was failing pre-S2).
```

### Implementation Patterns & Key Details

```go
// THE package-level LazyProc (resolved once, lazily — stdlib only, no x/sys dependency).
var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")

// THE three-field wiring — mirrors procgroup_unix.go field-for-field (SysProcAttr → Cancel → WaitDelay),
// only the mechanism differs.
cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
cmd.Cancel = func() error {
	r1, _, err := procGenerateConsoleCtrlEvent.Call(uintptr(syscall.CTRL_BREAK_EVENT), uintptr(cmd.Process.Pid))
	if r1 == 0 { return err }
	return nil
}
cmd.WaitDelay = 3 * time.Second

// THE reconciliation — S1's executor.go is UNCHANGED; it already calls setupProcessGroup(cmd). On a
// Windows build the linker selects procgroup_windows.go; on Unix it selects procgroup_unix.go. The
// build tags are mutually exclusive (windows vs !windows) so exactly one definition is compiled per GOOS.
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. setupProcessGroup uses stdlib syscall (LazyProc + constants) ONLY. `go mod tidy` MUST
        be a no-op. `git diff --exit-code go.mod go.sum` MUST be empty. (Option B — do NOT import
        golang.org/x/sys/windows; that WOULD modify go.mod/go.sum even in a build-tagged file.)

PACKAGE EDGES (import graph):
  - procgroup_windows.go → (stdlib: os/exec, syscall, time) ONLY. It does NOT import internal/config,
        internal/git, context, fmt, or golang.org/x/sys. It is in package provider (same as executor.go)
        so the setupProcessGroup call needs no import.

BUILD TAGS (the mutually-exclusive pair — completes the pattern S1 established):
  - procgroup_unix.go:     `//go:build !windows`   (S1 — Linux/macOS/darwin; the entire Unix CI matrix).
  - procgroup_windows.go:  `//go:build windows`    (S2 — Windows amd64 + arm64; PRD §21.2/§20.4).
  - Exactly ONE definition of setupProcessGroup is compiled per GOOS. executor.go (no tag) resolves the
        call to whichever file the build selects. This is why S2 is a clean single-file add with zero
        merge collision vs S1.

FROZEN FILES (do NOT edit):
  - internal/provider/executor.go (S1): platform-agnostic; calls setupProcessGroup(cmd). Works on Windows
        unchanged once procgroup_windows.go exists.
  - internal/provider/procgroup_unix.go (S1): the !windows half — untouched.
  - internal/provider/executor_test.go (S1): runs on BOTH Unix and Windows CI (no build tag); on Windows
        most cases skip via mustBin (Unix binaries absent) — Task 2's structural test covers the gap.
  - internal/provider/render.go + manifest/merge/builtin/registry + tests, internal/config/*, internal/git/*.

DOWNSTREAM CONTRACTS (do NOT implement here — just honor):
  - P1.M4.T2 (signal handler): cancels the same ctx passed to Execute on SIGINT/SIGTERM → os/exec fires
        cmd.Cancel → on Windows, GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT) kills the group; on Unix,
        syscall.Kill(-pid, SIGTERM). The handler needs NO platform branch — the kill is encapsulated in
        cmd.Cancel by this seam. (This is why S1/S2's cmd.Cancel design supersedes the work-item's
        separate KillProcessGroup sketch.)
  - P1.M3.T4 (orchestrator): errors.Is(err, context.Canceled) ⇒ exit 3 + rescue; context.DeadlineExceeded
        ⇒ exit 124 + rescue — uniform across platforms (executor.go is platform-agnostic).
  => The `func setupProcessGroup(cmd *exec.Cmd)` signature is FROZEN after S1+S2. Do not change.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the new files (the //go:build line must be first; gofmt enforces it)
gofmt -w internal/provider/procgroup_windows.go internal/provider/procgroup_windows_test.go

# Cross-vet the provider package FOR WINDOWS (compiles the windows files incl. the test — the only
# way to lint them from a Linux box, since gofmt/vet only parse the host-build files by default).
GOOS=windows go vet ./internal/provider/

# Confirm the build-tag file structure
head -1 internal/provider/procgroup_windows.go        # → "//go:build windows"
head -1 internal/provider/procgroup_windows_test.go   # → "//go:build windows"

# go.mod/go.sum MUST be unchanged (Option B — stdlib only)
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"

# Expected: Zero errors. If `GOOS=windows go vet` flags an unused import, remove it. If it reports
# `undefined: procGenerateConsoleCtrlEvent` or a type error, re-check the LazyProc usage (research §4/§5).
```

### Level 2: Cross-Build (THE KEYSTONE — was failing before S2)

```bash
# Cross-compile to Windows amd64 — MUST now succeed (S1 asserted this FAILED until S2 landed)
GOOS=windows GOARCH=amd64 go build ./...   && echo "windows/amd64 build ✓ (setupProcessGroup resolved)"

# Cross-compile to Windows arm64 — MUST succeed (PRD §21.2 arm64 target)
GOOS=windows GOARCH=arm64 go build ./...   && echo "windows/arm64 build ✓"

# Whole-module cross-build sanity
GOOS=windows go build ./...

# Expected: all succeed with no `undefined: setupProcessGroup`. If amd64/arm64 differ in outcome, a
# build-tag or an arch-specific syscall is wrong (neither should be — kernel32/LazyProc are arch-neutral).
```

### Level 3: Local Suite Unchanged (Linux dev box)

```bash
# procgroup_windows.go is excluded on Linux (build tag) → the host suite MUST be unchanged + green
go test -race ./...

# Confirm the windows file is NOT part of the Linux build (it should not appear in the compile set)
go list -f '{{.GoFiles}} {{.TestGoFiles}}' ./internal/provider/   # procgroup_windows*.go absent on Linux

# gofmt check across the package
gofmt -l internal/provider/   # → empty (no files listed)

# Expected: `go test -race ./...` green (S1 executor tests + all config/git/provider tests). No new
# failures. The windows file neither helps nor hurts the Linux build.
```

### Level 4: Windows CI Leg (runs in GitHub Actions, PRD §20.4 — not on the dev box)

```bash
# On windows-2022 the CI matrix runs:
go build ./...                                  # provider package builds; setupProcessGroup resolves to procgroup_windows.go
go test ./internal/provider/                    # S1's executor_test.go runs (most cases skip via mustBin — Unix binaries absent);
                                                #   Task 2's TestSetupProcessGroup_Wiring PASSES (the deterministic Windows-side check)
go vet ./...                                    # clean

# Manual structural reasoning check (the grandchild-kill is NOT unit-tested — research §6.2):
#   - CREATE_NEW_PROCESS_GROUP ⇒ child PID == its console process-group id.
#   - cmd.Cancel ⇒ GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, childPID) ⇒ signals the whole group that
#     shares the caller's console (child + descendants that didn't detach). Verified vs MS docs (§2/§3).
#   - cmd.WaitDelay = 3s ⇒ if the group ignores CTRL_BREAK, os/exec TerminateProcess'es the direct child.
# (Job Objects, the robust whole-tree kill, are deferred beyond v1 — golang/go #17608; documented in the
#  doc comment per PRD §12.7.2.)
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed; `GOOS=windows GOARCH=amd64 go build ./...` AND `GOARCH=arm64` succeed.
- [ ] `GOOS=windows go vet ./internal/provider/` clean (incl. procgroup_windows_test.go compiles).
- [ ] `gofmt -l internal/provider/` empty; `//go:build windows` is the first line of each new file.
- [ ] `go mod tidy` is a no-op; `git diff --exit-code go.mod go.sum` empty (Option B — stdlib LazyProc).
- [ ] Local `go test -race ./...` green (no regressions — windows file excluded on Linux).
- [ ] Every frozen file byte-unchanged (`git diff --exit-code internal/provider/executor.go
      internal/provider/procgroup_unix.go internal/provider/executor_test.go
      internal/provider/render.go internal/provider/manifest.go internal/provider/merge.go
      internal/provider/builtin.go internal/provider/registry.go` and their `_test.go`).

### Feature Validation

- [ ] `func setupProcessGroup(cmd *exec.Cmd)` defined in procgroup_windows.go with the IDENTICAL
      signature to procgroup_unix.go (S1) — `//go:build windows`.
- [ ] SysProcAttr.CreationFlags sets CREATE_NEW_PROCESS_GROUP (and does NOT set CREATE_NEW_CONSOLE).
- [ ] cmd.Cancel calls GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, cmd.Process.Pid) (NOT CTRL_C_EVENT).
- [ ] cmd.WaitDelay == 3 * time.Second (matches procgroup_unix.go for uniform cross-platform grace).
- [ ] (recommended) TestSetupProcessGroup_Wiring asserts CreationFlags/Cancel/WaitDelay wiring.

### Code Quality Validation

- [ ] Follows S1's established pattern (same package, same signature, same three-field wiring, same doc-
      comment rigor) — only the platform mechanism differs.
- [ ] File placement matches the desired codebase tree (procgroup_windows.go + optional _test.go).
- [ ] Anti-patterns avoided: no `executil` package; no `SetProcessGroup`/`KillProcessGroup`; no
      `golang.org/x/sys` dependency; no CREATE_NEW_CONSOLE; no CTRL_C_EVENT; no edit to executor.go or
      procgroup_unix.go; no negative-pid convention (Windows has none).
- [ ] The v1 limitation (grandchildren surviving if the child detaches/swallows CTRL_BREAK) is honestly
      documented in the doc comment with the Job Object upgrade path (PRD §12.7.2).

### Documentation & Deployment

- [ ] Doc comment on `setupProcessGroup` cites PRD §18.4 + critical_findings FINDING 10 + the CTRL_BREAK
      rationale + the console-sharing gotcha + the Job Object (#17608) v2 path.
- [ ] No new environment variables, config, or user-facing docs (PRD "DOCS: none — internal abstraction").

---

## Anti-Patterns to Avoid

- ❌ Don't create `internal/provider/executil/` or `SetProcessGroup`/`KillProcessGroup` — S1 froze
      `setupProcessGroup(cmd)` in package `provider` with the kill in `cmd.Cancel`. Follow S1 (research §0).
- ❌ Don't use `CTRL_C_EVENT` — it broadcasts to the whole console and cannot target one group (research §2).
- ❌ Don't set `CREATE_NEW_CONSOLE` — it gives the child its own console and `GenerateConsoleCtrlEvent`
      silently fails to reach it (research §3).
- ❌ Don't import `golang.org/x/sys/windows` — it modifies go.mod/go.sum even in a build-tagged file. Use
      stdlib `syscall.LazyProc` (Option B) to keep deps byte-unchanged (research §4).
- ❌ Don't edit `executor.go` or `procgroup_unix.go` — S2 is a clean single(+1)-file add; the build tags
      are mutually exclusive so there's nothing to reconcile in the existing files.
- ❌ Don't use a negative pid — Windows has no `-pid` convention; the group id is the child's PID (positive).
- ❌ Don't try to unit-test the grandchild kill on a Linux box — it can't be compiled or run locally.
      Validate via cross-build + `GOOS=windows go vet` + the Windows CI leg (research §6).
- ❌ Don't omit the Job-Object limitation from the doc comment — PRD §12.7.2 requires honest limitation docs.

---

## Confidence Score

**9/10** for one-pass implementation success. The Windows mechanism is pre-verified against the
authoritative Microsoft docs (CTRL_BREAK semantics, console-sharing requirement) and the Go stdlib
(`syscall.LazyProc` + the `CREATE_NEW_PROCESS_GROUP`/`CTRL_BREAK_EVENT` constants confirmed in GOROOT);
the S1 contract is FROZEN and documented to the field (identical signature, same three-field wiring);
the copy-ready reference implementation (research §5) is drop-in; the validation is cross-compile +
cross-vet (executable on the Linux dev box) + the Windows CI leg. The two residual risks — (a) the
grandchild-kill correctness is verified by code review + structural test + CI compile rather than a
direct grandchild test, and (b) the Windows CI leg is not exercisable on the dev box — are documented
and accepted, matching S1's own accepted limitations (S1 research §5.4).
