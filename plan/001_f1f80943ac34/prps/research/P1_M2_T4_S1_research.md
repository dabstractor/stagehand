# Research — P1.M2.T4.S1: internal/provider/executor.go — Executor.Run

## Goal of this note
Fix the exact os/exec + syscall mechanics (process-group kill, grace SIGKILL,
ctx-driven timeout vs cancel, typed errors) BEFORE writing the PRP, and prove
every behavior on the host (go1.26.4, linux/amd64) so the PRP's gates and
pseudo-code are verified-correct, not speculative.

---

## 1. Authority sources (all agree)

| Source | Statement |
|---|---|
| **PRD §12.2** | `cmd = exec.Command(m.command, args...)`; `cmd.Stdin = stdin?bytes.NewReader:user(/dev/null)`; `cmd.Env = os.Environ()+m.env`. NO sh -c. |
| **PRD FR24** | Capture the agent's stdout. |
| **PRD FR25** | Configurable timeout (default 120s); on timeout KILL the agent + rescue. |
| **PRD §18.4** | `SysProcAttr.Setpgid = true` so we can kill the whole tree; on signal SIGTERM then SIGKILL after a grace period via the process group. |
| **PRD §19** | Commands built as `[]string`, run via `exec.Command` directly — NEVER `sh -c`. Diff via stdin, never interpolated into an arg. |
| **decisions.md §5** | "Set `SysProcAttr.Setpgid=true` for process-group kill on signal (§18.4)." Env = os.Environ()+m.Env. |
| **Work item LOGIC** | render→exec.Command(m.Command, rendered.Args...); cmd.Dir=repo root; cmd.Stdin from rendered.StdinPayload if DeliverViaStdin else nil; cmd.Env=os.Environ()+m.Env; SysProcAttr{Setpgid:true}; capture stdout; goroutine + select on done vs ctx.Done(); on cancel/timeout → kill GROUP (SIGTERM to -PGID then SIGKILL after ~2s); return ErrTimeout(ctx deadline) or context-cancel error; non-zero exit → ErrAgent w/ short stderr excerpt. |

All consistent. **NO sh -c** is non-negotiable (§19). The diff is delivered via
stdin, never interpolated into an arg.

## 2. The consumption seam (Rendered, from P1.M2.T1.S1)

`Executor.Run` consumes the completed sibling type `Manifest.Render` output:
```go
r, err := m.Render(model, provider, sys, payload)   // P1.M2.T1.S1, PURE
cmd := exec.Command(m.Command, r.Args...)            // Args EXCLUDE the command token
if r.DeliverViaStdin {
    cmd.Stdin = bytes.NewReader([]byte(r.StdinPayload))
} // else nil ⇒ os/exec gives the child /dev/null automatically
cmd.Env = append(os.Environ(), r.Env...)             // r.Env = sorted "K=V" additions
```
The M2.T1.S1 integration-seam note ALSO assigns DEFAULT resolution to the
executor: **`if model == "" { model = m.DefaultModel }; if provider == "" { provider = m.DefaultProvider }` BEFORE Render** (Render is pure and does NOT resolve defaults). This is the cohesive choice — confirmed verbatim in the completed sibling PRP.

## 3. Verified mechanics on host (go1.26.4 linux/amd64)

A full prototype was run in /tmp/exectest and PASSED every case:
- **stdin feed + stdout capture:** `cmd.Stdout=&buf; cmd.Stdin=bytes.NewReader(...)` ⇒ `cat` echoes the stdin verbatim, `echo hi` yields `"hi\n"`. ✅
- **Setpgid + process-group kill:** `cmd.SysProcAttr=&syscall.SysProcAttr{Setpgid:true}`; child PID == PGID (process becomes its own group leader). `syscall.Kill(-pgid, syscall.SIGTERM)` kills the whole group (the hanging `sleep 30` died in ~80ms). ✅
- **ctx-driven timeout vs cancel:** `select{case <-done: case <-ctx.Done():}` ⇒ on `context.WithTimeout` expiry `ctx.Err()==context.DeadlineExceeded`; on `context.WithCancel` `ctx.Err()==context.Canceled`. ✅
- **grace SIGKILL:** SIGTERM → `select{case <-done: case <-time.After(2s): SIGKILL; <-done}` — SIGTERM almost always suffices (the 2s branch is a safety net). ✅
- **non-zero exit + stderr:** `cmd.Stderr=&buf2`; `exit 7` ⇒ Wait returns `*exec.ExitError`; stderr buffer holds the agent's stderr for the ErrAgent excerpt. ✅

All mechanics use ONLY stdlib (`context`, `os/exec`, `syscall`, `bytes`, `time`,
`fmt`, `strings`). **No new module deps.** `go build ./internal/provider/` and
`go vet` will be clean; no go.mod/go.sum change.

## 4. Platform constraint: SysProcAttr.Setpgid is Unix-only

`syscall.SysProcAttr{Setpgid: true}` exists on linux/darwin/freebsd/etc.
(the `//go:build unix` set) but NOT on windows. The host is linux and the
validation gate `go build ./internal/provider/` runs on linux ⇒ inline use in
executor.go compiles + works here. PRD §20.4 lists windows in the CI matrix,
but windows process-group semantics are a Unix-shaped concept (there is no
PGID); a windows port needs `CREATE_NEW_PROCESS_GROUP`/`taskkill /T` — that is
a deferred porting concern, explicitly out of scope for this task. **Decision
for THIS task:** inline `&syscall.SysProcAttr{Setpgid: true}` in executor.go
(host gate passes); document the unix constraint in a gotcha so a future
windows task (or build-tag split `executor_unix.go`+`executor_other.go`) can
land it without rework. This is the minimal, scope-faithful choice (the work
item names a single file `executor.go`).

## 5. The context.Context is the signal-handler seam

Work item: "Expose a way for the signal handler (M6.T2) to share/observe this
context." The M6.T2.S1 contract confirms the mechanism: CommitStaged owns a
cancellable `context.Context`; it passes the CONTEXT to `Executor.Run` and the
CANCEL FUNC to the signal handler's `installSignalHandler(ctxCancel func(),
rescueFn func())`. On SIGINT/SIGTERM the handler calls `ctxCancel()` ⇒
`ctx.Done()` fires inside Run ⇒ Run kills the process group. **Therefore `Run(ctx
context.Context, ...)` accepting ctx and reacting to ctx.Done() IS the
observation seam — NO extra Executor field or accessor is required.** Over-
building (a stored context, a public Cancel method, an observe channel) would
add surface with no caller (M6.T1/M6.T2 never need it). Keep Run signature
exactly `Run(ctx, m Manifest, model, provider, sys, payload string) (string,
error)`.

## 6. Timeout ownership: caller applies it via ctx (NOT a Run param)

FR25's configurable timeout (cfg.Timeout, default 120s) is applied by the
CALLER — CommitStaged does `ctx, cancel := context.WithTimeout(parent,
cfg.Timeout)` and passes `ctx` to Run. Run has NO timeout parameter (the work-
item signature has none). Run distinguishes the two ctx failure modes:
`ctx.Err()==context.DeadlineExceeded` ⇒ typed `*TimeoutError` (FR25 rescue
trigger); `ctx.Err()==context.Canceled` ⇒ plain context-cancel error (signal-
driven). Tests trigger the timeout by wrapping ctx with `context.WithTimeout
(ctx, 80ms)` + a `/bin/sleep 30` stub.

## 7. Typed error shapes (consumed by generate M6.T1 + rescue M6.T1.S3)

Generate needs `errors.As` to route the rescue path (timeout/agent-fail →
rescue). Define two exported error types (NOT bare sentinels) so the caller can
extract data:
```go
// TimeoutError: the generation ctx deadline elapsed and the group was killed.
type TimeoutError struct{ Deadline time.Time }
func (e *TimeoutError) Error() string { return "provider: generation timed out" }
func (e *TimeoutError) Unwrap() error { return context.DeadlineExceeded }

// AgentError: the agent exited non-zero (bad flags, missing config, crash).
type AgentError struct {
    Name    string // m.Name, for a clear message
    Command string // m.Command
    Code    int    // exit code (best-effort; -1 if signaled-but-unexpected)
    Stderr  string // short excerpt (last ~500 chars), §“short stderr excerpt”
}
func (e *AgentError) Error() string { ... }
```
A cancelled (non-timeout) run returns `ctx.Err()` directly (a
`*context.Canceled`) — generate treats any Run error as rescue per decisions §3.

## 8. Mocking strategy (PRD §20.1 layer 1/2; no real LLM)

Tests use REAL host binaries as the fake agent (NO stub binary generation, NO
go-binaries helper — keeps the test stdlib-only and cross-host):
- `/bin/echo hi`            ⇒ clean exit, stdout="hi\n" (capture test).
- `/bin/cat`                ⇒ echoes stdin verbatim (stdin-feed-exactly test).
- `/bin/sh -c 'echo X >&2; exit 7'` ⇒ non-zero exit + stderr (ErrAgent test).
  NOTE: `/bin/sh -c` is used ONLY inside the TEST (as the fake agent's own
  body), NOT in Run — Run itself NEVER uses sh -c (§19). This is the
  difference: the product execs `m.Command` directly with `[]string`; the test
  happens to choose `/bin/sh` as `m.Command` and `-c '...'` as its arg, which
  is a legitimate manifest (some real agents ARE `sh -c`-style). The §19
  prohibition is on the PRODUCT interpolating into a shell — Run never does.
- `/bin/sleep 30` + `ctx.WithTimeout(80ms)` ⇒ timeout kills the group,
  `*TimeoutError` returned, elapsed ≈ 80ms (proves SIGTERM killed it fast).
- `/bin/sleep 30` + `ctx.WithCancel` ⇒ `context.Canceled` returned.
Each test builds a tiny Manifest literal (Command=/bin/echo etc.,
PromptDelivery="stdin") and a zero-value or Dir="" Executor; cmd.Dir="" means
"inherit stagehand's cwd" which is fine for these fakes. (Repo-root Dir wiring
is exercised in the generate integration tests M6.T3.)

To assert the PROCESS-GROUP kill actually felled a child (not just the
leader), one test runs `/bin/sh -c 'trap "" TERM; sleep 30'` under a timeout:
the `trap "" TERM` makes the child IGNORE SIGTERM, so the 2s-grace SIGKILL is
required to kill it — proving both the group targeting (-PGID) AND the grace
escalation. (Keep this test's effective wait under ~2.5s.)

## 9. Validation gates (verified valid on host)

`go build ./internal/provider/`, `go vet ./internal/provider/`,
`test -z "$(gofmt -l internal/provider/)"`, `go test ./internal/provider/`,
`go test ./...`. All single-command (no &&/heredoc/for). The executor tests
spawn real processes (echo/cat/sleep/sh) — fast (<1s total), deterministic,
no network, no real LLM. Scope: ONLY create `internal/provider/executor.go`
+ `internal/provider/executor_test.go`; do NOT touch manifest.go/parse.go/
registry.go/builtin.go, main.go, Makefile, go.mod/go.sum; do NOT run
`go mod tidy` (stdlib-only additions).

## 10. Executor struct shape (minimal)

`Executor` is a struct so it can hold the repo-root Dir for cmd.Dir (the work
item says `cmd.Dir = repo root`). For THIS task only `Dir string` is needed;
future fields (verbose writer, logger) land when their callers exist (avoid
speculative fields). `NewExecutor(dir string) *Executor` constructor for
ergonomics. Run is a method: `func (e *Executor) Run(ctx context.Context, m
Manifest, model, provider, sys, payload string) (string, error)`.
