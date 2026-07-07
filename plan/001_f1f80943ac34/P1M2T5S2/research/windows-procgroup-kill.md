# Research: Cross-platform process-group kill — the Windows half

> Subtask P1.M2.T5.S2 — Cross-platform process-group kill abstraction (Unix + Windows build tags).
> This file is the SINGLE source of truth for the Windows `setupProcessGroup` implementation and the
> S1↔S2 contract reconciliation. Read it before writing `procgroup_windows.go`.

---

## §0. The S1↔S2 contract reconciliation (READ FIRST — avoids a duplicate abstraction)

The WORK-ITEM DESCRIPTION (written before S1 was planned) sketches a DIFFERENT shape than what S1
actually shipped. **S2 must follow S1's contract (the live, being-implemented one), NOT the stale
work-item text.** Side-by-side:

| Aspect | Work-item text (STALE) | S1 PRP (LIVE CONTRACT — shipped in parallel) |
|---|---|---|
| Package location | `internal/provider/executil/` | **`internal/provider/`** (same package as executor.go) |
| API surface | `SetProcessGroup(cmd)` + `KillProcessGroup(pid, sig)` (two fns) | **`setupProcessGroup(cmd *exec.Cmd)`** (ONE fn, mutates cmd in place) |
| Files | `executil.go` + `exec_unix.go` + `exec_windows.go` | `executor.go` (no tag) + `procgroup_unix.go` (`!windows`) + **`procgroup_windows.go` (`windows`) ← S2** |
| Who calls the kill | executor + signal handler call `KillProcessGroup` directly | **`cmd.Cancel` closure** (set inside `setupProcessGroup`) — invoked by os/exec on ctx cancel. The signal handler just cancels ctx (uniform, cross-platform). |

**S1 already created `internal/provider/procgroup_unix.go`** with the FROZEN signature
`func setupProcessGroup(cmd *exec.Cmd)` and the Unix mechanism (`SysProcAttr{Setpgid:true}` +
`cmd.Cancel = syscall.Kill(-pid, SIGTERM)` + `cmd.WaitDelay = 3s`). S1's executor.go calls
`setupProcessGroup(cmd)` with no import (same package). S2 adds ONLY the Windows file to the SAME
frozen signature — touching NEITHER executor.go NOR procgroup_unix.go.

**Do NOT create an `executil` package, and do NOT add `SetProcessGroup`/`KillProcessGroup`.** That
would be a duplicate abstraction conflicting with S1's. The platform kill is encapsulated in
`cmd.Cancel` (set by `setupProcessGroup`), invoked uniformly via ctx cancellation — so a separate
`KillProcessGroup` is unnecessary: the signal handler (P1.M4.T2) cancels ctx → os/exec fires
`cmd.Cancel` → group kill. This is cleaner and fully cross-platform.

The "consumed by the executor (P1.M2.T5.S1) and signal handler (P1.M4.T2)" clause in the work item
is satisfied TRANSITIVELY under S1's design:
- Executor: already calls `setupProcessGroup(cmd)` inside `Execute` (landed by S1).
- Signal handler: cancels the same ctx → `cmd.Cancel` → group kill (no direct call needed).

---

## §1. The Windows mechanism — what replaces `Setpgid` + `syscall.Kill(-pid, sig)`

Windows has **no** POSIX process groups, no `Setpgid`, and `syscall.Kill(-pid, ...)` does not exist.
The documented Windows analog (critical_findings FINDING 10 + work-item LOGIC §3) is:

1. **`CREATE_NEW_PROCESS_GROUP` (`0x00000200`)** in `SysProcAttr.CreationFlags` → the child becomes a
   *console process-group* leader. **Its PID == its process-group ID** — the exact parallel of Unix's
   `Setpgid ⇒ PGID==PID`. So the group is addressed by the child's PID.
2. **`GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, childPID)`** in `cmd.Cancel` → signals the WHOLE
   group on ctx cancel (timeout or signal). CTRL_BREAK (not CTRL_C) — see §2.
3. **`cmd.WaitDelay = 3 * time.Second`** → if the child ignores CTRL_BREAK, os/exec escalates after 3s
   (on Windows it `TerminateProcess`es the direct child).

This maps 1:1 onto the Unix `setupProcessGroup` shape (set SysProcAttr; set Cancel; set WaitDelay) —
which is why S1's frozen signature works for BOTH platforms without any executor.go change.

### §1.1 Exact field semantics (verified from Go 1.22–1.26 stdlib + Microsoft docs)

- **`syscall.SysProcAttr` (GOOS=windows)** has field `CreationFlags uint32` (verified in GOROOT
  `src/syscall/exec_windows.go`). Set it to `syscall.CREATE_NEW_PROCESS_GROUP`. Do NOT also set
  `CREATE_NEW_CONSOLE` — see §3 (console-sharing gotcha).
- **`cmd.Cancel`** (Go 1.20+, platform-agnostic field) is `func() error`, invoked by
  `exec.CommandContext` when the ctx is cancelled. Inside it, `cmd.Process` is guaranteed non-nil
  (os/exec only calls it after `Start()` succeeds). Default Windows Cancel `TerminateProcess`es the
  DIRECT child only; we override to signal the GROUP.
- **`cmd.WaitDelay`** (Go 1.20+, platform-agnostic) — after Cancel, wait this long then forcibly
  kill. Same `3 * time.Second` as Unix.

---

## §2. Why CTRL_BREAK_EVENT, not CTRL_C_EVENT (the single most important Windows nuance)

From the authoritative Microsoft doc (https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent):

> "Generates a CTRL+C signal. This signal **cannot be limited to a specific process group**. If
> dwProcessGroupId is nonzero, this function will succeed, but the CTRL+C signal will not be received
> by those processes."

i.e. `GenerateConsoleCtrlEvent(CTRL_C_EVENT, pid)` is effectively a **broadcast to every process
sharing the caller's console** — it would also interrupt Stagecoach itself and unrelated processes.
Whereas **`CTRL_BREAK_EVENT` CAN be limited to a specific process group** (`dwProcessGroupId` honored).
→ The work item is correct: use **`CTRL_BREAK_EVENT`** (`syscall.CTRL_BREAK_EVENT == 1`).

The `dwProcessGroupId` argument = the child's PID (== its group id, post `CREATE_NEW_PROCESS_GROUP`).

---

## §3. The console-sharing requirement (the silent failure mode)

`GenerateConsoleCtrlEvent` only delivers to processes **attached to the SAME console as the caller**
(verified — Microsoft "Console Process Groups" doc + multiple SO/Groups threads). Consequences:

- **DO set `CREATE_NEW_PROCESS_GROUP`** (puts the child in its own *group*, still sharing the console).
- **Do NOT set `CREATE_NEW_CONSOLE`** (that would give the child its own console → it would NEVER
  receive our CTRL_BREAK → the kill silently no-ops). The default (no CREATE_NEW_CONSOLE) lets the
  child inherit Stagecoach's console → CTRL_BREAK reaches it. ✓
- **Honest limitation (document, do not fix in v1):** if a child agent detaches from the console
  (e.g. its own `CREATE_NEW_CONSOLE`, or a GUI daemon) or installs a handler that swallows
  CTRL_BREAK, grandchildren may survive escalation. os/exec's WaitDelay escalation on Windows only
  `TerminateProcess`es the direct child. The robust fix is **Job Objects** (golang/go issue #17608),
  which guarantee whole-tree kill on job close. Job Objects are explicitly **deferred beyond v1**
  (FINDING 10 names them only as a parenthetical alternative; the work item's primary specified
  approach is `CREATE_NEW_PROCESS_GROUP + GenerateConsoleCtrlEvent`). This matches PRD §12.7.2's
  "document limitations honestly" principle.

---

## §4. Dependency decision — stdlib `syscall.LazyProc`, NOT `golang.org/x/sys/windows`

**The problem:** `GenerateConsoleCtrlEvent` is NOT exported by Go's stdlib `syscall` package
(verified: `grep -rn GenerateConsoleCtrlEvent $GOROOT/src/syscall/` → NOT FOUND). It lives in
`golang.org/x/sys/windows`. The constants `CREATE_NEW_PROCESS_GROUP`, `CTRL_BREAK_EVENT`,
`CTRL_C_EVENT` ARE in stdlib syscall, but the function call is not.

**Two options:**

| Option | Mechanism | go.mod impact | Verdict |
|---|---|---|---|
| A | `import "golang.org/x/sys/windows"` → `windows.GenerateConsoleCtrlEvent(...)` | **MODIFIES go.mod + go.sum** (go mod tidy resolves imports across ALL build constraints, so a build-tagged `//go:build windows` import STILL lands in the module graph) | ✗ breaks S1's "stdlib-only, go.mod byte-unchanged" principle |
| B | `syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent").Call(...)` (stdlib) | **NONE** — zero deps, go.mod/go.sum byte-unchanged | ✓ **chosen** |

**Decision: Option B (stdlib `syscall.LazyProc`).** Rationale:
1. **Matches S1's principle.** S1 shipped executor.go + procgroup_unix.go with stdlib only and go.mod
   byte-unchanged. S2 should not balloon a 1-file Windows shim into a new transitive dependency.
2. **`syscall.NewLazyDLL` / `(*LazyProc).Call` ARE stable stdlib APIs** (GOOS=windows), verified in
   GOROOT `src/syscall/dll_windows.go` (line 316 `func NewLazyDLL`, line 374 `func (*LazyProc) Call`).
   The Go stdlib itself uses this pattern for Windows calls. It resolves `kernel32!GenerateConsoleCtrlEvent`
   lazily on first `Call`.
3. **Minimal blast radius.** S2 touches exactly one new source file; no `go.mod`/`go.sum` churn, no
   `go mod tidy`/`go mod download` step for CI.
4. The constants (`CREATE_NEW_PROCESS_GROUP`, `CTRL_BREAK_EVENT`) are already stdlib → no magic numbers.

The `Call` returns `(r1, r2 uintptr, lastErr error)`. `GenerateConsoleCtrlEvent` returns 0 on failure
(then `lastErr` = GetLastError). Check `r1 == 0` → return `lastErr`; else `nil`.

> **Fallback note for the implementer:** if a reviewer strongly prefers the typed `golang.org/x/sys`
> wrappers, Option A is a valid alternative — but it MUST be a conscious decision that adds the
> dependency to go.mod/go.sum (run `go mod tidy`), documented as such. The default is Option B.

---

## §5. The reference implementation (copy-ready, drop-in for S1's Unix file)

```go
// internal/provider/procgroup_windows.go
//go:build windows

package provider

import (
	"os/exec"
	"syscall"
	"time"
)

// procGenerateConsoleCtrlEvent resolves kernel32!GenerateConsoleCtrlEvent lazily. GenerateConsoleCtrlEvent
// is not in Go's stdlib syscall package; we resolve it via syscall.LazyProc (stdlib, GOOS=windows) so
// NO module dependency is added — go.mod/go.sum stay byte-unchanged (matching procgroup_unix.go).
var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")

// setupProcessGroup is the Windows implementation of the cross-platform seam declared in executor.go
// (FROZEN by P1.M2.T5.S1: `func setupProcessGroup(cmd *exec.Cmd)`). Identical signature to
// procgroup_unix.go's setupProcessGroup — only the platform mechanism differs (FINDING 10,
// go_ecosystem_patterns §3.4).
//
// Windows has no POSIX process groups, Setpgid, or syscall.Kill(-pid). The analog of "kill the whole
// child tree on ctx cancel" is:
//   - CREATE_NEW_PROCESS_GROUP (0x00000200) in CreationFlags: the child becomes a console process-
//     group leader; its PID == its process-group id (the exact parallel of Unix Setpgid ⇒ PGID==PID).
//   - cmd.Cancel: on ctx cancel, call GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, pid) to signal the
//     whole group. CTRL_BREAK (not CTRL_C) because CTRL_C is broadcast to ALL processes sharing the
//     caller's console and cannot be limited to one group; CTRL_BREAK honors dwProcessGroupId.
//   - cmd.WaitDelay = 3s: if the child ignores CTRL_BREAK, os/exec escalates after 3s (on Windows it
//     TerminateProcess'es the direct child).
//
// CONSOLE GOTCHA: GenerateConsoleCtrlEvent reaches ONLY processes attached to the caller's console.
// We set CREATE_NEW_PROCESS_GROUP but NOT CREATE_NEW_CONSOLE so the child inherits Stagecoach's
// console. If a child detaches from the console or swallows CTRL_BREAK, grandchildren may survive
// escalation; the robust fix is Job Objects (golang/go #17608), deferred beyond v1 (PRD §12.7.2).
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP, // child = console process-group leader; PID==PGID
	}
	cmd.Cancel = func() error {
		// cmd.Process is guaranteed non-nil inside Cancel (os/exec calls it only after Start()).
		// CTRL_BREAK_EVENT (1) targets the specific group == child PID.
		r1, _, err := procGenerateConsoleCtrlEvent.Call(
			uintptr(syscall.CTRL_BREAK_EVENT),
			uintptr(cmd.Process.Pid),
		)
		if r1 == 0 {
			return err // signal failed (e.g. child already dead); WaitDelay escalates to TerminateProcess.
		}
		return nil
	}
	cmd.WaitDelay = 3 * time.Second
}
```

---

## §6. Validation strategy (dev box is Linux — the Windows file is NOT compiled locally)

The dev machine and the Linux/macOS CI legs never compile `procgroup_windows.go` (build tag). So S2's
validation is **cross-compilation + the Windows CI leg (PRD §20.4)**, NOT local unit tests:

| Check | Command | What it proves | Where it runs |
|---|---|---|---|
| Cross-build (amd64) | `GOOS=windows GOARCH=amd64 go build ./...` | procgroup_windows.go compiles + provides `setupProcessGroup` → **the `undefined: setupProcessGroup` failure S1 documented is now RESOLVED** | Linux dev box (cross-compile) |
| Cross-build (arm64) | `GOOS=windows GOARCH=arm64 go build ./...` | arm64 Windows builds (PRD §21.2) | Linux dev box |
| Cross-vet | `GOOS=windows go vet ./internal/provider/` | incl. the windows test file compiles | Linux dev box |
| Local suite unchanged | `go test -race ./...` | procgroup_windows.go excluded on Linux → no regressions; S1 tests still green | Linux dev box |
| go.mod clean | `git diff --exit-code go.mod go.sum` | **Option B kept deps byte-unchanged** | Linux dev box |
| Windows CI leg | CI runs `go build ./...` + `go test ./internal/provider/` on windows-2022 | the package builds on real Windows; S1's build-tag-less `executor_test.go` runs + exercises the Windows `setupProcessGroup` via `Execute` (binaries `cat`/`sleep`/`printenv`/`false` may be absent on Windows → `mustBin` skips those; the structural test below is the deterministic Windows-side check) | GitHub Actions windows-2022 |

**KEYSTONE for S2:** `GOOS=windows GOARCH=amd64 go build ./...` SUCCEEDS. (S1's PRP asserted this
FAILED until S2 landed — that assertion is now inverted: S2 is the thing that flips it green.)

### §6.1 A structural Windows test (optional but recommended — `//go:build windows`)

S1's `executor_test.go` has no build tag, so it RUNS on the Windows CI leg, but most of its cases
shell out to Unix binaries (`cat`, `sleep`, …) absent on Windows → `mustBin` skips them, leaving the
Windows `setupProcessGroup` weakly exercised. A small `//go:build windows` test file gives the
Windows leg a deterministic, dependency-free check that the wiring is correct (SysProcAttr set to
CREATE_NEW_PROCESS_GROUP; Cancel non-nil; WaitDelay == 3s). It compiles via `GOOS=windows go vet` and
runs only on Windows CI. See PRP Task 3.

### §6.2 Accepted limitation — no grandchild-kill test on Windows (same honesty as S1 §5.4)

Verifying that CTRL_BREAK reaches a child's GRANDCHILDREN on Windows needs a portable
grandchild-spawning test (fiddly, and undevelopable on a Linux box). NOT done. Correctness is
established by (a) code review vs §1/§5 and the Microsoft docs, (b) the structural test, and (c)
CI compiling the Windows file. Matches S1's accepted limitation (research §5.4).

---

## §7. Downstream contracts (do NOT implement here — just honor)

- **executor.go (S1, FROZEN):** calls `setupProcessGroup(cmd)` with no import (same package). S2
  must not change executor.go — it already works on Windows once procgroup_windows.go exists.
- **procgroup_unix.go (S1, FROZEN):** untouched. S2 and S1 are in different files (`windows` vs
  `!windows`) → zero merge collision.
- **P1.M4.T2 (signal handler):** cancels the same ctx passed to Execute on SIGINT/SIGTERM → os/exec
  fires `cmd.Cancel` → `GenerateConsoleCtrlEvent` on Windows / `syscall.Kill(-pid)` on Unix. The
  signal handler needs NO platform-specific kill code — S1/S2's `cmd.Cancel` encapsulates it.
- **Error contract (platform-agnostic, from S1):** on ctx cancel `cmd.Wait()` errors + `ctx.Err() ==
  context.Canceled` → Execute returns `context.Canceled` → orchestrator exit 3 + rescue (§18.2).
  Same on both platforms because executor.go is platform-agnostic.

## §8. Sources

- Microsoft, *GenerateConsoleCtrlEvent function* — https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent
- Microsoft, *Console Process Groups* — https://learn.microsoft.com/en-us/windows/console/console-process-groups
- golang/go issue #17608, *syscall: add support for Windows job objects* — https://github.com/golang/go/issues/17608
- Go stdlib `syscall` (GOOS=windows): `dll_windows.go` (NewLazyDLL / LazyProc.Call), `exec_windows.go`
  (SysProcAttr.CreationFlags), `types_windows.go` (CREATE_NEW_PROCESS_GROUP=0x200, CTRL_C_EVENT=0,
  CTRL_BREAK_EVENT=1) — verified in GOROOT.
- critical_findings.md FINDING 8 (Setpgid/Cancel/WaitDelay recipe) + FINDING 10 (Windows build-tag split).
- go_ecosystem_patterns.md §3.4 (Setpgid Unix-only → Job Objects on Windows) + §3.5 (signal forwarding).
