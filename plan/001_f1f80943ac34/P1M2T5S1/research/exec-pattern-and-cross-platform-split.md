# Research: Provider Executor — canonical os/exec pattern, cross-platform split, error semantics

> Subtask P1.M2.T5.S1 — Subprocess executor (stdin pipe, stdout/stderr capture, env, timeout).
> This research is the SINGLE source of truth for the executor's design. Read it before writing
> `executor.go`, `procgroup_unix.go`, or `executor_test.go`.

---

## §1. The canonical os/exec pattern (Go 1.22+) — the authoritative reference

The PRD's own architecture doc `plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md §3`
("os/exec: Safe Subprocess Execution with Process Groups") is THE canonical pattern for this
task. critical_findings.md FINDING 8 + FINDING 10 cross-reference it. The work item itself says
"See …§3 (canonical os/exec pattern with cmd.Cancel + cmd.WaitDelay + SysProcAttr.Setpgid)."

The pattern delivers ALL FOUR requirements in the work-item LOGIC clause:

| Requirement (work item) | Mechanism (§3.1) |
|---|---|
| stdin pipe from `spec.Stdin` | `cmd.Stdin = strings.NewReader(spec.Stdin)` (only when non-empty) |
| capture stdout + stderr separately | two `bytes.Buffer`s on `cmd.Stdout` / `cmd.Stderr` |
| `cmd.Env = spec.Env` | `cmd.Env = spec.Env` (Render already built os.Environ()+manifest) |
| process-group kill on cancel | `SysProcAttr{Setpgid:true}` + `cmd.Cancel = syscall.Kill(-pid, SIGTERM)` |
| 3s grace before SIGKILL | `cmd.WaitDelay = 3 * time.Second` |
| configurable timeout | `ctx, cancel = context.WithTimeout(ctx, timeout)` when `timeout > 0` |

### 1.1 The exact field semantics (verified from Go 1.20 release notes + os/exec docs)

- **`cmd.Cancel`** (added Go 1.20): a `func() error` invoked by `exec.CommandContext` when the
  context is cancelled (timeout OR parent cancel). The DEFAULT Cancel sends SIGKILL to the direct
  child ONLY. We OVERRIDE it to send SIGTERM to the **whole process group** (`-pid`), which:
  - is gentler (SIGTERM lets the agent flush/cleanup), and
  - kills grandchildren (agents like pi/claude spawn helper processes).
  - `cmd.Process` is **guaranteed non-nil** inside Cancel — os/exec only calls it after `Start()`
    succeeds, so the closure may dereference `cmd.Process.Pid` safely.

- **`cmd.WaitDelay`** (added Go 1.20): after `Cancel` is called, the duration to wait for the
  process to exit before Go **forcibly sends SIGKILL**. `3 * time.Second` matches the work item
  ("3s grace before SIGKILL") and §3.1. This handles an agent that ignores SIGTERM.

- **`SysProcAttr.Setpgid = true`**: makes the child a NEW process-group leader. Its **PGID == PID**,
  so `syscall.Kill(-pid, sig)` addresses the entire group (child + all descendants). SIDE EFFECT
  (FINDING 8 / §3.5): because the child is in its OWN group, it does NOT receive the terminal's
  Ctrl-C — the PARENT must forward signals. That forwarding is the CLI signal handler's job
  (P1.M4.T2.S1), NOT the executor's. The executor's `cmd.Cancel` already covers context-cancellation
  (timeout + the parent's `ctx` being cancelled by the signal handler).

### 1.2 Why Start()+Wait(), NOT cmd.Run()/CombinedOutput()

`cmd.Run()` is Start()+Wait() combined, but it returns a single error and offers no way to return
the SEPARATE captured stdout/stderr buffers along with the error. The work-item signature is
`(stdout string, stderr string, err error)` — three return values, all populated even on error
(partial stdout/stderr are useful for the parser/rescue path). So we call `cmd.Start()` then
`cmd.Wait()` manually, reading the buffers afterward. (This is exactly §3.1's choice.)

---

## §2. The cross-platform delegation design (S1 / S2 split) — THE central design call

### 2.1 The problem

`SysProcAttr.Setpgid` and `syscall.Kill(-pid, sig)` are **Unix-only** (Linux/macOS/darwin). On
Windows the same effect needs Job Objects / `CREATE_NEW_PROCESS_GROUP` +
`GenerateConsoleCtrlEvent` (FINDING 10). The work item says:

> "Apply process-group setup (**delegate to S2's cross-platform abstraction**)."

S2 (P1.M2.T5.S2 — "Cross-platform process-group kill abstraction (Unix + Windows build tags)") OWNS
the abstraction. But S1 ships FIRST and MUST compile + pass tests independently on the Unix CI
matrix (Linux + macOS; PRD §20.4). So the delegation target must EXIST when S1 lands.

### 2.2 The resolution — build-tag-segregated files, S1 establishes the contract + Unix half

**S1 (this subtask) creates THREE files:**

1. `internal/provider/executor.go` — **NO build tag**. Platform-AGNOSTIC. Contains `Execute(...)`.
   Imports `bytes`, `context`, `fmt`, `os/exec`, `strings`, `time` ONLY — **no `syscall`**. It calls
   a single platform-specific helper: `setupProcessGroup(cmd)`.

2. `internal/provider/procgroup_unix.go` — build tag `//go:build !windows`. The Unix implementation
   of `setupProcessGroup(cmd *exec.Cmd)`. Imports `os/exec`, `syscall`, `time`. Sets `SysProcAttr`,
   `cmd.Cancel`, `cmd.WaitDelay`.

3. `internal/provider/executor_test.go` — **NO build tag** (CI is Unix). The test suite.

**S2 (future) creates ONE file:**

4. `internal/provider/procgroup_windows.go` — build tag `//go:build windows`. The Windows
   implementation of `setupProcessGroup(cmd *exec.Cmd)` — SAME signature. Imports the Windows
   syscall packages. S2 does NOT touch executor.go or procgroup_unix.go.

### 2.3 Why this is correct

- **S1 compiles + passes on Unix**: `procgroup_unix.go` (`!windows`) matches Linux/macOS, so
  `setupProcessGroup` is defined; `go build ./...` + `go test -race ./...` succeed on the dev
  machine and CI.
- **No merge collision with S2**: S1 and S2 write to DIFFERENT files (S1 = executor.go +
  procgroup_unix.go + test; S2 = procgroup_windows.go). Neither modifies the other's files.
- **executor.go is portable**: it has zero `syscall` references — only S2's Windows file adds the
  platform-specific bits. The delegation point (`setupProcessGroup(cmd)`) is the ONLY seam.
- **Cross-compiling to Windows before S2 fails** (`undefined: setupProcessGroup`) — which is
  CORRECT: Windows support isn't ready until S2 lands. (PRD CI matrix §20.4 lists Linux/macOS first.)
- **FROZEN contract for S2**: `func setupProcessGroup(cmd *exec.Cmd)` — signature pinned by S1. S2
  implements the identical signature. The doc comment in procgroup_unix.go states this explicitly.

### 2.4 The helper signature

```go
// setupProcessGroup configures cmd so that on context cancellation the entire child process
// tree is terminated (process-group kill). Sets cmd.SysProcAttr, cmd.Cancel, cmd.WaitDelay.
func setupProcessGroup(cmd *exec.Cmd)
```

No return value — it mutates the `*exec.Cmd` in place (sets three fields). This is the cleanest
seam: executor.go passes the cmd, the platform file configures it.

---

## §3. Error & timeout semantics (the return contract)

The work item: "Return stdout, stderr, and err (context.DeadlineExceeded → timeout)." And:
"err signals timeout/failure for the orchestrator's retry/rescue."

### 3.1 The four cases

| Case | When | `ctx.Err()` | Returned `err` | Orchestrator action (P1.M3.T4 / §18.2) |
|---|---|---|---|---|
| Success | exit 0 | nil | nil | parse stdout |
| Timeout | timeout fired | `context.DeadlineExceeded` | `context.DeadlineExceeded` | exit 124 + rescue |
| Signal/parent cancel | SIGINT/SIGTERM handler cancelled ctx | `context.Canceled` | `context.Canceled` | exit 3 + rescue |
| Non-zero exit | child exit ≠ 0 | nil | wrapped `*exec.ExitError` | retry, then rescue |
| Start failure | binary not on PATH | nil | wrapped LookPath err | "command not found", exit 1 |

### 3.2 The check ORDER (critical)

`exec.CommandContext`'s `cmd.Wait()` returns an error in BOTH the timeout case AND the non-zero-exit
case. The distinguishing signal is **`ctx.Err()`** (checked FIRST):

```go
if err := cmd.Wait(); err != nil {
    if ctxErr := ctx.Err(); ctxErr != nil {
        return out.String(), errb.String(), ctxErr  // timeout → DeadlineExceeded; signal → Canceled
    }
    return out.String(), errb.String(), fmt.Errorf("provider %q: %w", spec.Command, err)  // real exit error
}
return out.String(), errb.String(), nil
```

This matches §3.1 verbatim. Checking `ctx.Err()` first is essential — without it, a timeout would
be misreported as a generic exit error and the orchestrator could not detect exit-124.

### 3.3 Why return partial stdout/stderr on error?

The rescue path (§18.3) prints a candidate message if one was produced before failure, and verbose
mode (P1.M4.T3.S2) dumps raw output. Returning the captured buffers (even partial) on every error
path serves both. The work-item signature returns `(string, string, error)` for exactly this reason.

### 3.4 The `ctx` shadowing detail (gotcha)

```go
if timeout > 0 {
    var cancel context.CancelFunc
    ctx, cancel = context.WithTimeout(ctx, timeout)  // SHADOWS the param — intentional
    defer cancel()
}
```

Because `ctx` is shadowed, the LATER `ctx.Err()` reads the TIMEOUT context, which correctly returns
`DeadlineExceeded` on timeout and `Canceled` on parent-cancel. (If we read the parent ctx, a
timeout would show `nil`. The shadow is load-bearing — do not "fix" it by renaming.)

### 3.5 How the orchestrator distinguishes timeout from signal

`errors.Is(err, context.DeadlineExceeded)` → timeout (exit 124).
`errors.Is(err, context.Canceled)` → signal (exit 3).
Both trigger the rescue path (§18.2). The executor surfaces BOTH faithfully; the orchestrator maps
them. (The executor does NOT decide exit codes — that's the CLI layer, P1.M4.T3.S3.)

---

## §4. The CmdSpec contract (from P1.M2.T4.S1 — FROZEN, read render.go)

`Execute` consumes `provider.CmdSpec`, defined in `internal/provider/render.go` (landed by T4):

```go
type CmdSpec struct {
    Command string   // "pi", "agent", …  → exec.Command(spec.Command, spec.Args...)
    Args    []string // flags AFTER command (NOT including Command)
    Stdin   string   // payload to pipe; "" → no stdin (child gets /dev/null)
    Env     []string // os.Environ() + manifest Env as "KEY=VAL"
}
```

executor.go wiring (matches render.go's doc comment contract for the executor):
- `cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)`
- `if spec.Stdin != "" { cmd.Stdin = strings.NewReader(spec.Stdin) }` — empty → leave nil → os/exec
  gives the child `/dev/null` (the exact semantics Render's doc promised).
- `cmd.Stdout = &out; cmd.Stderr = &errb` (two separate `bytes.Buffer`s)
- `cmd.Env = spec.Env` — guarded by `if len(spec.Env) > 0` so a nil Env (hand-built test spec)
  inherits the parent env instead of getting an empty env. Render always populates Env, so the guard
  is harmless in production but protects tests.
- **No `cmd.Dir`** — the agent runs in the user's CWD (where they invoked stagecoach). Git operations
  use `-C repo`; the agent does not.

---

## §5. Test strategy — real binaries, matching codebase conventions

### 5.1 Convention (confirmed from committree_test.go)

The codebase tests git by shelling out to the **real git binary** (no mocks, no fakes). It uses
stdlib `testing`, `reflect.DeepEqual`, direct `t.Errorf`/`t.Fatalf`, and `t.Setenv` for env. There
is NO mocking framework. The executor tests follow the SAME philosophy: shell out to **real,
universally-present Unix binaries** (`cat`, `sleep`, `printenv`, `false`), with `exec.LookPath`
guards that `t.Skip` (not fail) if a binary is absent.

The work item explicitly sanctions this: "Mock: a stub binary (e.g. **/bin/cat** or a tiny Go test
helper) that echoes stdin to stdout; test: normal run, timeout kill, large stdin."

### 5.2 Binaries used + what each verifies

| Binary | Test | What it verifies |
|---|---|---|
| `cat` | normal run + large stdin | stdin pipe → stdout (echo round-trip); 1MiB stress |
| `cat /nonexistent` | stderr + non-zero exit | stderr captured separately; `*exec.ExitError` surfaced |
| `sleep 30` (timeout 200ms) | timeout kill | `context.DeadlineExceeded`; Execute returns promptly (not 30s) |
| `printenv VAR` | env propagation | `cmd.Env = spec.Env` wiring; a manifest env var reaches the child |
| `false` | non-zero exit (clean) | exit-1 error path; stdout/stderr still returned |
| `cat` with Stdin="" | no-stdin (positional/flag delivery) | Stdin empty → no pipe → child gets /dev/null → no hang |
| non-existent binary | command not found | Start() failure → wrapped error |
| parent-ctx cancel | signal/parent-cancel path | `context.Canceled` distinguished from timeout |

### 5.3 Availability guards

```go
func mustBin(t *testing.T, names ...string) {
    t.Helper()
    for _, n := range names {
        if _, err := exec.LookPath(n); err != nil {
            t.Skipf("required binary %q not in PATH: %v", n, err)
        }
    }
}
```
All of `cat`, `sleep`, `printenv`, `false` are present on ubuntu-latest + macos-latest runners and
the dev Linux box. The guard is defensive — it should never skip in CI.

### 5.4 Accepted limitation — no grandchild-kill test

Verifying that the process-group kill reaches GRANDCHILDREN requires spawning a grandchild process
(portable test is fiddly). We do NOT test that directly. Instead: (a) the timeout test proves the
KILL happens (Execute returns within seconds, not 30s — `sleep` dies on SIGTERM immediately), and
(b) the `Setpgid` + `Kill(-pid)` correctness is established by code review against the canonical
pattern (§3.1 / FINDING 8), which is battle-tested. This matches §3.4's "Key Safety Notes".

---

## §6. Import sets (per file) — verified against go 1.22 + existing deps

| File | Build tag | Imports |
|---|---|---|
| `executor.go` | (none) | `bytes`, `context`, `fmt`, `os/exec`, `strings`, `time` |
| `procgroup_unix.go` | `//go:build !windows` | `os/exec`, `syscall`, `time` |
| `executor_test.go` | (none) | `bytes`, `context`, `errors`, `os`, `os/exec`, `strings`, `testing`, `time` |

**No new module dependency.** All stdlib. `go.mod`/`go.sum` byte-unchanged. (cobra is NOT yet in
go.mod — the CLI lands in P1.M4; the executor doesn't need it.)

**Note on `executor.go` + `syscall`**: executor.go must NOT import `syscall` (that would make it
non-portable). All `syscall` use is confined to `procgroup_unix.go`. `gofmt`/`go vet` will catch a
stray import. (`fmt` is used only for the non-zero-exit and start-failure wraps.)

---

## §7. Integration points (downstream consumers — do NOT implement here)

- **P1.M3.T4 (CommitStaged orchestrator)**: `stdout, stderr, err := Execute(ctx, *spec,
  cfg.Timeout)`; on `errors.Is(err, context.DeadlineExceeded)` → exit 124 + rescue; on
  `context.Canceled` → exit 3 + rescue; on other err → retry then rescue; on nil → hand stdout to
  the parser (P1.M2.T6).
- **P1.M2.T6 (parser)**: consumes the returned `stdout` string (raw agent output).
- **P1.M4.T2.S1 (signal handler)**: cancels the SAME `ctx` passed to Execute on SIGINT/SIGTERM →
  triggers the `context.Canceled` path. (The executor does not install the signal handler — it only
  honors ctx cancellation, which `cmd.Cancel` already turns into a group SIGTERM.)
- **P1.M2.T5.S2 (cross-platform Windows)**: implements `setupProcessGroup` in
  `procgroup_windows.go` to the frozen signature. Does not touch executor.go.

The `Execute(ctx, spec CmdSpec, timeout time.Duration) (stdout, stderr string, err error)`
signature + the `setupProcessGroup(cmd)` contract are FROZEN after this subtask.
