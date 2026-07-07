# P1.M4.T3.S1 — Design Decisions & Findings

Scope: CREATE `internal/ui/output.go` (TTY detect + color helpers + Progress/Success/Error +
NO_COLOR) + `internal/ui/output_test.go`, and wire it into `internal/cmd/default_action.go` for the
safe, test-compatible surfaces. Mirrors the contract in the work item verbatim.

## Decisions

**D1 — Package = `internal/ui`, single file `output.go`.** PRD §14 package layout literally lists
`internal/ui/output.go` ("progress messages, color, TTY detect"). The sibling §14 slot
`internal/ui/exitcode.go` was implemented instead as `internal/exitcode/` (P1.M4.T1.S1, COMPLETE) —
so `internal/ui/` is created fresh with just `output.go`. No `internal/ui` dir exists yet (confirmed:
`ls internal/ui` → not found).

**D2 — Color gate = pure function `ResolveColor(noColor, isTTY bool) bool`.** Decouples the
untestable TTY check from the testable flag/env logic (the work item explicitly says "can't easily
test TTY in unit tests; test the flag/env logic"). Body: `!noColor && !noColorEnvSet() && isTTY`.
`noColor` is `cfg.NoColor` (already folds `--no-color` Layer 7 + `STAGECOACH_NO_COLOR` Layer 5).
`noColorEnvSet()` reads the bare `NO_COLOR` env (NOT yet handled by config — confirmed grep).
Caller passes `isTTY = IsTerminal(os.Stdout)`.

**D3 — TTY detection via stdlib `ModeCharDevice` (NO new dep).** Matches the project's stdlib-only
philosophy (`procgroup_windows.go` uses `syscall.NewLazyDLL`, not `golang.org/x/sys`). We do NOT add
`golang.org/x/term`. See external-research.md §2 for the heuristic + its accepted gotcha.

**D4 — `NO_COLOR` via `os.LookupEnv("NO_COLOR")` with `ok && v != ""`.** Byte-identical idiom to
`config/load.go` line 112 (`STAGECOACH_NO_COLOR`). Matches current no-color.org spec ("present and not
an empty string"). There is NO "force color" override in v1 (`CLICOLOR_FORCE` out of scope).

**D5 — `UI` type holds `stdout, stderr io.Writer` + resolved `color bool`.** Writers are injected
(cobra's `cmd.OutOrStdout()/ErrOrStderr()` in prod, `*bytes.Buffer` in tests). Color is a resolved
bool passed to `New` — tests construct `ui.New(&buf, &buf, true|false)` directly (no real TTY
needed). `IsTerminal(os.Stdout)` runs ONCE at construction in `runDefault`; the UI never re-checks
the TTY.

**D6 — Streams: Progress/Success/Error → STDERR (may color); stdout stays PLAIN.** FR51 + §15.5 +
`default_action_test.go` exact-equality assertions force this. `printCommitReport` /
`printDryRunMessage` (the DATA surface) stay plain on stdout and are NOT threaded through color
helpers. See external-research.md §5.

**D7 — Color palette: green=Success, red=Error, yellow=Progress-ish/warn.** ANSI SGR constants
`ansiRed/Green/Yellow/Reset` (see external-research.md §3). Helpers `Green/Red/Yellow(s) string`
return `code+s+reset` when color, else `s` unchanged.

**D8 — `Progress(msg)` writes `"↳ " + msg + "\n"` (↳ = U+21B3) to stderr.** The exact Appendix-B
prefix. `Success(msg)` writes the same `↳ ` prefix in GREEN to stderr (the "↳ Created …"
confirmation). `Error(msg)` writes `Red(msg)` to stderr. All three use `fmt.Fprintln` (trailing
newline). Ellipsis `…` is U+2026 (matches PRD; do NOT use "...").

**D9 — Wiring into `default_action.go` is SAFE vs the parallel P1.M4.T2.S1.** T2.S1 edits ONLY
`main.go`, `internal/provider/executor.go`, `internal/generate/generate.go`,
`pkg/stagecoach/stagecoach.go` (its PRP "SCOPE BOUNDARY" lists root.go/config.go/providers.go/
default_action.go as DO-NOT-TOUCH siblings). So editing `default_action.go` here cannot conflict.
We do NOT touch generate.go/executor.go/main.go/stagecoach.go (T2.S1 owns them).

**D10 — What stays UNCHANGED (frozen / out of scope):**
- `generate.FormatRescue` output (the §18.3 rescue block) — frozen, byte-for-byte; do NOT colorize the
  ❌ block (it's already printed plain to stderr by `handleGenError` + by the T2.S1 signal handler).
- `printCommitReport` text + stream (stdout, plain) — keep; test asserts `Contains("] feat: add login")`.
- `printDryRunMessage` (stdout, plain) — keep; test asserts `stdout == "feat: dry run"`.
- Verbose output (resolved command / raw output / retry details) — P1.M4.T3.S2, NOT this task.
- Exit codes / ExitError — already shipped in P1.M4.T1.S1 (`internal/exitcode`); S3 is a no-op/refine.

## Findings

**F1 — `cfg.NoColor` is already a resolved Config field (`config.go` line 24, `toml:"-"`).** It is set
by Layer 5 (`STAGECOACH_NO_COLOR`, `load.go:112`) and Layer 7 (`--no-color`, `loadFlags:151`). Its own
comment says: *"NoColor is ultimately TTY-aware in the UI layer"* and *"set by UI layer (P1.M4.T3.S1)"*
— i.e. THIS task is the designated owner of the final TTY/NO_COLOR resolution. The UI layer ADDS the
bare `NO_COLOR` env + the `isTTY(stdout)` check on top of `cfg.NoColor`.

**F2 — The bare `NO_COLOR` env var is NOT handled anywhere yet.** `grep -rn "NO_COLOR\b" internal/`
shows only `STAGECOACH_NO_COLOR` in config. The `NO_COLOR` comment in `config.go:23` is a forward
pointer to this task. ⇒ `internal/ui` owns `noColorEnvSet()`.

**F3 — `default_action_test.go` locks the stream contract.** Key assertions (must not break):
- Dry-run: `strings.TrimSpace(outBuf.String()) == "feat: dry run"` → **stdout must be exactly the
  message, plain, no decoration** (no "↳ Generating", no "(no commit created)" on stdout).
- Commit: `Contains(stdout, "] feat: add login")` + `Contains(stdout, "A  new.txt")` → the FR42
  report on stdout (plain; Contains survives but we keep it plain for pipe-safety).
- FR18: `Contains(stderr, "Nothing staged — staging all changes (2 files).")` → stderr; ANSI wrapping
  survives `Contains`, so colorizing the notice is SAFE.
⇒ The "↳ Generating…" Progress line goes to STDERR (errBuf), so it does NOT touch the stdout
equality. SAFE to add on both commit and dry-run paths.

**F4 — `runDefault` already has the writers + cfg in hand.** Lines: `stdout := cmd.OutOrStdout()`,
`stderr := cmd.ErrOrStderr()`, `cfg := Config()`. So constructing `u := ui.New(stdout, stderr,
ui.ResolveColor(cfg.NoColor, ui.IsTerminal(os.Stdout)))` is a 1-liner. TTY check uses `os.Stdout`
(the real file) — NOT `stdout` (which may be a `*bytes.Buffer` in tests and would then mis-detect).

**F5 — The resolved provider name is NOT known in `default_action.go`** (auto-detection happens deep
in the registry/generate pipeline; `cfg.Provider` is `""` when auto). So the "↳ Generating with pi
(glm-5.2)…" label is built defensively: append `" with <provider>"` only when `cfg.Provider != ""`,
and `" (<model>)"` only when `cfg.Model != ""`. Auto (the common case) → `"↳ Generating…"`. The full
"with <resolved-provider>" mid-pipeline Progress would need a pipeline callback (generate.go — owned
by the parallel T2.S1), so it is OUT OF SCOPE here; the pre-call generic label is the v1 surface.

**F6 — No import cycle.** `internal/ui` imports only stdlib (`fmt`, `io`, `os`). `internal/cmd`
imports `internal/ui` (one-way). `internal/ui` imports NOTHING from stagecoach — so any package may
import it freely (incl. future signal/provider packages) without a cycle. Mirrors `internal/exitcode`
(which imports only `internal/generate`) — but `ui` is even cleaner: stdlib-only.

## Test plan (mirrors `internal/cmd/root_test.go` conventions)

`internal/ui/output_test.go` (pure unit tests; no real TTY needed):
- `TestResolveColor_Logic`: matrix over (noColor, isTTY) → expected bool. (4 cases.)
- `TestResolveColor_NoColorEnv`: `t.Setenv("NO_COLOR", "1")` → false; `t.Setenv("NO_COLOR","")` →
  isTTY-gated (i.e. env empty does NOT disable); unset → isTTY-gated. Also assert `STAGECOACH_*`
  independence (NO_COLOR is a separate var).
- `TestColorHelpers_NoOpWhenColorOff`: `ui.New(&out,&err,false)` → `Green("x")=="x"`,
  `Progress("m")` writes `"↳ m\n"` plain, `Success`/`Error` plain.
- `TestColorHelpers_WrapWhenColorOn`: `ui.New(&out,&err,true)` → `Green("x")=="\x1b[32mx\x1b[0m"`,
  `Success("c")` writes `"\x1b[32m↳ c\x1b[0m\n"`, `Error("e")` writes `"\x1b[31me\x1b[0m\n"`.
- `TestProgress_PrefixAndStream`: assert Progress writes to the STDERR writer (not stdout) and the
  exact `"↳ <msg>\n"` bytes (↳ = U+21B3).
- `TestIsTerminal_Pipe`: `os.Pipe()` end is NOT a char device → `IsTerminal(r) == false` (the one
  deterministic TTY assertion possible without a pty). (A real TTY can't be opened portably in a unit
  test — that's the work item's acknowledged gap; we assert the negative case.)
