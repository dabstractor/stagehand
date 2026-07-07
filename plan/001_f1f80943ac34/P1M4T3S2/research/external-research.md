# External Research — P1.M4.T3.S2 Verbose Mode

Scope: `--verbose` / `-v` / `STAGECOACH_VERBOSE=1` prints (1) the resolved provider command,
(2) the raw agent stdout, (3) each retry attempt — to **stderr** — using a `DEBUG:` prefix
(per the work-item contract note: "commit-pi uses VERBOSE=1 with DEBUG: prefix lines").

---

## 1. The `DEBUG:` prefix convention (authoritative for this task)

The work-item CONTRACT explicitly states: *"commit-pi uses VERBOSE=1 with DEBUG: prefix lines."*
PRD §2.1 names commit-pi as the originating tool, and Appendix C is a "Line-by-line porting map
from commit-pi." Therefore Stagecoach's verbose lines use a `DEBUG: ` prefix — this is the single
most important formatting decision and it is CONTRACT-DRIVEN, not a stylistic choice.

**Resolved line formats (deterministic, testable):**

| Function                | Writes to stderr (only when verbose ON)                                  |
| ----------------------- | ------------------------------------------------------------------------ |
| `VerboseCommand(cmd)`   | `DEBUG: command: <cmd>\n` — `<cmd>` = argv joined by spaces              |
| `VerboseRawOutput(out)` | `DEBUG: raw output:\n<out>` (trailing `\n` ensured)                      |
| `VerboseRetry(n,reason)`| `DEBUG: attempt <n>: <reason>\n` — `<n>` is 1-based                      |

**Why `DEBUG:` and NOT the `↳` (U+21B3) progress prefix:** `↳` is owned by P1.M4.T3.S1's
`internal/ui` Progress/Success/Error helpers — those are ALWAYS-SHOWN progress lines
(Appendix B.1: `↳ Snapshotting…`, `↳ Generating…`, `↳ Created…`). Verbose lines are
ONLY shown with `-v`. Using a distinct `DEBUG:` prefix (a) matches commit-pi, (b) keeps the
two output streams visually separable, (c) avoids any edit to S1's `output.go` (parallel-safe).

> Appendix B.4 (`stagecoach -v`) shows `↳ Attempt 1: …` lines. That is the PRD's *illustrative*
> rendering. The CONTRACT's `DEBUG:` convention is authoritative for the implementation; the
> `↳ Attempt` wording is reflected in the *reason text*, not the prefix.

## 2. PRD §19 security boundary (line 1203 — load-bearing)

> *"Logs in `--verbose` print the command and flags but **never stdin contents** unless
> `STAGECOACH_VERBOSE=2`."*

And §19 line (No secret handling): *"Stagecoach never reads, logs, or transmits the agent's
credentials… Stagecoach only spawns it with the inherited environment (plus any manifest-declared
`[env]` additions)."*

**Implications for `VerboseCommand`:**
- **Log argv ONLY** (`spec.Command` + `spec.Args`). The display string is
  `strings.Join(append([]string{spec.Command}, spec.Args...), " ")`.
- **NEVER log `spec.Env`** — `spec.Env` = `os.Environ()` + manifest env, which routinely
  carries credentials (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, bearer tokens). Logging it would
  leak secrets to stderr / shell scrollback / CI logs. This is the #1 security gotcha.
- **stdin protection**: for `stdin`-delivery providers (pi, the default), the prompt payload is
  in `spec.Stdin` (NOT logged at VERBOSE=1 — only at the future VERBOSE=2). For
  `positional`/`flag`-delivery providers, the payload is in `spec.Args` and IS logged as part of
  "the command and flags" — which §19 explicitly permits at VERBOSE=1. So logging argv is
  compliant for all delivery modes; logging Stdin is NOT (until VERBOSE=2).

## 3. VERBOSE=2 is OUT OF SCOPE (future) — and currently un-parseable

The contract: *"STAGECOACH_VERBOSE=2 could print stdin contents (§19 notes)."* The word "could"
marks it future. Concretely, `config.Verbose` is a **`bool`**, and `config/load.go` parses it via
`strconv.ParseBool` — which accepts `1/true/0/false` but **rejects `"2"`** (`ParseBool("2")` →
error → config load fails). Supporting VERBOSE=2 would require changing `Config.Verbose` to an
`int` (a cross-cutting config change owned by P1.M1.T4, NOT this task). **S2 implements VERBOSE=1
semantics only** (`-v` / `STAGECOACH_VERBOSE=1` → bool true). VERBOSE=2/stdin is documented as a
deferred future enhancement; do NOT implement it.

## 4. Library-vs-CLI stream discipline (why verbose is an injectable writer, not os.Stderr)

Stagecoach has a public library surface (`pkg/stagecoach.GenerateCommit`) used both by the CLI and
by 3rd-party Go integrators. A library function writing directly to `os.Stderr` is an anti-pattern
(it hijacks the host process's stderr). The correct pattern: thread an **`io.Writer`** (the
diagnostic sink) through the pipeline; `nil` ⇒ silent. The CLI supplies its `cmd.ErrOrStderr()`
stderr; a library consumer supplies a writer of its choice (or `nil`). This mirrors how Go's
`log` package takes an `io.Writer` and how the existing `internal/ui.UI` (P1.M4.T3.S1) takes
injectable writers.

**Stream safety proof (why `TestRunDefault_DryRun` stays green):** that test asserts
`strings.TrimSpace(stdout) == "feat: dry run"` EXACTLY (default_action_test.go:272). Verbose
writes go to the **stderr** writer (the `*ui.Verbose.w` field), never the stdout writer. In that
test `cfg.Verbose == false` (no `-v`), so the `*ui.Verbose` is a no-op anyway. Double-safe: even
if a future test flips verbose on, the DEBUG lines land in `errBuf`, leaving `outBuf` pristine.

## 5. Go testing pattern — capturing verbose output via an injected writer

The `*ui.Verbose` writer is injectable (`NewVerbose(w io.Writer, on bool)`), so unit tests capture
output with a plain `*bytes.Buffer` — no subprocess, no pty, no `os.Stderr` redirection:

```go
func TestVerbose_CommandWhenOn(t *testing.T) {
    var buf bytes.Buffer
    v := ui.NewVerbose(&buf, true)            // on
    v.VerboseCommand("pi --model x")
    want := "DEBUG: command: pi --model x\n"
    if buf.String() != want { t.Errorf(...) }
}
func TestVerbose_NoOpWhenOff(t *testing.T) {
    var buf bytes.Buffer
    v := ui.NewVerbose(&buf, false)           // off
    v.VerboseCommand("pi")
    if buf.Len() != 0 { t.Errorf("wrote %q when off", buf.String()) }
}
```

For executor/orchestrator-level tests, the same buffer is passed as `deps.Verbose` /
the `Execute` `vb` param. This matches `internal/cmd/root_test.go`'s `bytes.Buffer` +
`rootCmd.SetErr(&errBuf)` capture style already used across the repo.

## 6. Layering: core packages MAY import `internal/ui` (leaf, stdlib-only)

`internal/ui` imports only the Go stdlib (`fmt`, `io`, `os`, `strings`) — confirmed by S1's design.
Therefore `internal/provider` and `internal/generate` importing `internal/ui` introduces **no import
cycle** (ui has zero stagecoach imports). The existing codebase already has cross-cutting internal
imports (`internal/signal` is imported by BOTH `provider` and `generate`), so a `verbose` sink
following the same shape is consistent with project conventions. (A purist could invert this via an
interface defined in core + implemented by ui, but that adds a type for ~3 methods — rejected as
over-engineering for v1; see design-decisions.md D4.)

## 7. References

- **Work-item contract** — "commit-pi uses VERBOSE=1 with DEBUG: prefix lines" (the authoritative
  formatting rule for this task).
- **PRD.md:318 (FR50)** — "--verbose / -v / STAGECOACH_VERBOSE=1 — print the resolved provider
  command, the raw agent stdout, and each retry attempt to stderr."
- **PRD.md:946 (§15.2)** — `--verbose, -v | STAGECOACH_VERBOSE | — | false | Print resolved command,
  raw output, retries.`
- **PRD.md:1203 (§19)** — "Logs in --verbose print the command and flags but never stdin contents
  unless STAGECOACH_VERBOSE=2." (the security boundary + VERBOSE=2 deferral).
- **PRD Appendix B.4** — illustrative `-v` session (retry wording source; prefix per contract).
- **P1.M4.T3.S1 PRP** — defines `internal/ui/output.go` (the `↳` Progress/color layer S2 must NOT
  edit; S2 adds `internal/ui/verbose.go` as a sibling).
