---
name: "P1.M4.T3.S1 — Progress messages, TTY detection, and color (NO_COLOR support) — PRD §9.13 FR50/FR51, §15.2, Appendix B"
description: |

  Create the `internal/ui` output package (`internal/ui/output.go` + tests) that owns ALL CLI
  rendering: (1) TTY detection on `os.Stdout` (stdlib `ModeCharDevice` heuristic — NO new dep, matching
  `procgroup_windows.go`'s stdlib-only philosophy); (2) a pure color-gate `ResolveColor(noColor, isTTY)`
  that folds `cfg.NoColor` (already resolved from `--no-color`/`STAGECOACH_NO_COLOR` by config.Load) +
  the bare `NO_COLOR` env var (https://no-color.org — NOT yet handled anywhere in the codebase) + the
  TTY check; (3) ANSI color helpers `Green/Red/Yellow` that no-op when color is off; (4) `Progress(msg)`
  writing the Appendix-B `↳ ` (U+21B3) prefix to STDERR (FR51: stdout stays clean for piping);
  `Success(msg)`/`Error(msg)` (green/red) also to STDERR. Then WIRE it into `internal/cmd/default_action.go`
  for the safe, test-compatible surfaces: construct the UI, emit the `↳ Generating…` progress line before
  generation, and colorize the FR18 stderr notice + generic error line. stdout DATA stays PLAIN.

  CONTRACT (PRD §9.13 FR50/FR51, §15.2, Appendix B; work-item spec):
    - FR51: "Color output when stdout is a TTY; disable with `--no-color` or `NO_COLOR`. Progress
      messages go to stderr so stdout stays clean for piping."
    - §15.2 `--no-color`: default "TTY-aware"; env `STAGECOACH_NO_COLOR`; "Respects `NO_COLOR`."
    - Appendix B: every progress line is prefixed `↳ ` (U+21B3), e.g. `↳ Generating with pi (glm-5.2)…`.
    - Work item: "Implement color helpers (green, red, yellow) that no-op when !TTY or NoColor.
      Implement `Progress(msg)` writing to os.Stderr with the ↳ prefix. Implement `Success(msg)` and
      `Error(msg)` helpers. Respect NO_COLOR env var and --no-color flag. Mock: unit tests for color
      on/off logic (can't easily test TTY in unit tests; test the flag/env logic)."

  INPUT (upstream — all EXIST, READ/CONSUME only, do NOT modify their behavior):
    - `config.Config.NoColor bool` (`toml:"-"`) — P1.M1.T4.S1. ALREADY resolved by `config.Load`:
      Layer 5 `STAGECOACH_NO_COLOR` (load.go:112) + Layer 7 `--no-color` (loadFlags:151). Its own
      comment says "set by UI layer (P1.M4.T3.S1)" — THIS task is the designated owner of the final
      TTY/NO_COLOR resolution. We ADD the bare `NO_COLOR` env + the `os.Stdout` TTY check on top.
    - `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` — cobra writers (P1.M4.T1.S1). Used as the UI's
      writers (so tests with `rootCmd.SetOut(&buf)` capture output).
    - `cfg.Provider` / `cfg.Model` — for the "↳ Generating with <provider> (<model>)…" label.

  OUTPUT: the `internal/ui` package, consumed by `internal/cmd` (P1.M4.T1) and available to any future
  package (stdlib-only → no import-cycle risk).

  DELIVERABLES (2 NEW files + 1 EDIT):
    NEW internal/ui/output.go        — `package ui`: IsTerminal, noColorEnvSet, ResolveColor, UI type
                                       (New/Color/Green/Red/Yellow), Progress/Success/Error methods.
    NEW internal/ui/output_test.go   — unit tests for color on/off logic + stream/prefix assertions
                                       (no real TTY required — D2 decouples the env/flag logic).
    EDIT internal/cmd/default_action.go — construct the UI from cfg + streams; emit the `↳ Generating…`
                                       Progress line before `stagecoach.GenerateCommit`; route the FR18
                                       auto-stage notice + generic error through the color helpers.
                                       stdout DATA (`printCommitReport`/`printDryRunMessage`) UNCHANGED.

  SCOPE BOUNDARY (owned by siblings — do NOT implement or edit):
    - `main.go`, `internal/provider/executor.go`, `internal/generate/generate.go`,
      `pkg/stagecoach/stagecoach.go` — being edited IN PARALLEL by P1.M4.T2.S1 (signal handler). DO NOT
      touch them (its PRP lists default_action.go as a do-not-touch sibling; the reverse holds too).
    - Verbose output (resolved command / raw output / retry details) — P1.M4.T3.S2.
    - Exit codes / ExitError — already shipped P1.M4.T1.S1 (`internal/exitcode`); S3 refine-only.
    - `generate.FormatRescue` / the §18.3 rescue block — FROZEN, byte-for-byte; do NOT colorize the ❌
      block (it's printed plain to stderr by `handleGenError` + by the T2.S1 signal handler).
    - Mid-pipeline progress that fires INSIDE `generate.CommitStaged`/`runPipeline` (e.g. a precise
      "↳ Generating with <resolved-provider>…" after registry auto-detect, "↳ Snapshotting…", "↳ Attempt
      N…") — requires a pipeline callback into generate.go (owned by parallel T2.S1). OUT OF SCOPE here;
      the pre-call generic Progress line is the v1 surface.

  DEPENDENCY GRAPH (CYCLE-FREE): `internal/ui` → stdlib ONLY (fmt, io, os). `internal/cmd` →
  `internal/ui` (one-way). No other stagecoach package is imported by `ui`, so any package may import it.

  Deliverable: 2 NEW files + 1 EDIT. `go build ./...` green; `go test -race ./internal/ui/ -v` green;
  `go test -race ./...` NO regression (default_action_test.go stream assertions still pass — Progress
  goes to stderr, stdout stays plain); `go vet ./...` clean; `gofmt -l internal/ui/ internal/cmd/` empty.

---

## Goal

**Feature Goal**: Ship Stagecoach's CLI rendering layer (PRD §9.13 FR51, §15.2, Appendix B): a
stdlib-only `internal/ui` package that detects whether `os.Stdout` is a terminal, resolves whether to
emit ANSI color from that + `--no-color`/`STAGECOACH_NO_COLOR` (via `cfg.NoColor`) + the bare `NO_COLOR`
env var (https://no-color.org), and provides color-aware helpers (`Green/Red/Yellow`,
`Progress`/`Success`/`Error`) where **progress goes to STDERR** so **stdout stays byte-clean for
piping** (FR51). Then wire it into the default action so the `↳ Generating…` progress line actually
appears and stderr notices pick up color.

**Deliverable** (2 NEW files + 1 EDIT):
1. NEW `internal/ui/output.go` — `package ui`:
   - `func IsTerminal(f *os.File) bool` — stdlib `ModeCharDevice` heuristic.
   - `func ResolveColor(noColor bool, isTTY bool) bool` — pure color gate (folds `cfg.NoColor` + `NO_COLOR` env + TTY).
   - `func New(stdout, stderr io.Writer, color bool) *UI` + `func (u *UI) Color() bool`.
   - `func (u *UI) Green/Red/Yellow(s string) string` — ANSI-wrap iff color on, else plain.
   - `func (u *UI) Progress(msg string)` — writes `"↳ " + msg + "\n"` (↳ = U+21B3) to STDERR.
   - `func (u *UI) Success(msg string)` — writes `Green("↳ " + msg)` to STDERR.
   - `func (u *UI) Error(msg string)` — writes `Red(msg)` to STDERR.
2. NEW `internal/ui/output_test.go` — unit tests for the color on/off logic + stream/prefix bytes.
3. EDIT `internal/cmd/default_action.go` — construct the UI; emit `Progress("Generating…")` before
   `stagecoach.GenerateCommit`; route the FR18 notice + generic-error line through the color helpers.

**Success Definition**: `go test -race ./internal/ui/ -v` green (color-gate matrix + NO_COLOR env +
ANSI-wrap/no-op + Progress prefix/stream + IsTerminal-on-a-pipe); `go test -race ./...` green with
**zero** change to `default_action_test.go`'s stream assertions (dry-run `stdout == "feat: dry run"`
exact-equality still holds because Progress goes to stderr; commit `Contains(stdout,"] feat: add login")`
holds because stdout stays plain). `make build` then `./bin/stagecoach --dry-run` in a real repo prints
`↳ Generating…` to stderr (colored on a TTY, plain under `NO_COLOR=1` or a pipe) and the bare message to
stdout. `go vet ./...` clean; `gofmt -l` empty for the touched trees.

## User Persona

**Target User**: The Stagecoach CLI user (PRD §7 "the plan-holder") running `stagecoach` interactively in
a terminal — they want live progress feedback (`↳ Generating…`) and color-coded success/error — AND the
same user piping output (`stagecoach --dry-run --no-color | tee /tmp/msg.txt`, §15.5) who needs stdout
clean. The single design must serve both; FR51 is the rule that reconciles them.

**Use Case**: `git add -A && stagecoach` → see `↳ Generating…` (colored on a TTY) → see the FR42 report.
Then `stagecoach --dry-run | git commit -F -` → stdout is the message only, no ANSI, no progress noise.

**User Journey**: user runs stagecoach → `↳ Generating…` appears on stderr → generation completes → the
commit report appears on stdout (plain, pipeable) → on error, a red notice on stderr. When the user
redirects/pipes, `NO_COLOR`/non-TTY auto-disables color so downstream tools never see ANSI.

**Pain Points Addressed**: (1) no progress feedback during the (slow) generation → the `↳ ` progress
line; (2) ANSI codes leaking into piped output (`git commit -F <(stagecoach --dry-run)`) → stdout never
carries color; (3) respecting the universal `NO_COLOR` convention → implemented.

## Why

- **Closes PRD §9.13 FR51 (P1) + the §15.2 `--no-color` contract.** Without this, color is impossible
  (no TTY/NO_COLOR logic exists) and progress would have to go to stdout (breaking the §15.5 pipe use
  case). This task is the explicit owner (the `Config.NoColor` comment names "P1.M4.T3.S1").
- **The bare `NO_COLOR` env is NOT yet handled** (`grep -rn "NO_COLOR\b" internal/` finds only
  `STAGECOACH_NO_COLOR`). The UI layer adds the convention so Stagecoach behaves like every other
  well-mannered CLI.
- **Stdlib-only, zero new deps.** `IsTerminal` uses `os.FileStat` + `ModeCharDevice`; color is plain ANSI
  SGR constants. Matches `procgroup_windows.go`'s no-`golang.org/x/sys` philosophy (project invariant).
- **Library-safe.** `internal/ui` imports only stdlib → any package can import it with no cycle risk.

## What

A new `internal/ui` package (PRD §14 layout) exposing a pure color gate + a small `UI` type:

```go
func IsTerminal(f *os.File) bool                                     // stdlib ModeCharDevice heuristic
func ResolveColor(noColor bool, isTTY bool) bool                     // !noColor && !NO_COLOR && isTTY
func New(stdout, stderr io.Writer, color bool) *UI                   // writers injectable for tests
func (u *UI) Color() bool
func (u *UI) Green(s string) string                                  // "\x1b[32m"+s+"\x1b[0m" iff color
func (u *UI) Red(s string) string
func (u *UI) Yellow(s string) string
func (u *UI) Progress(msg string)                                    // stderr: "↳ "+msg+"\n"   (↳ = U+21B3)
func (u *UI) Success(msg string)                                     // stderr: Green("↳ "+msg)+"\n"
func (u *UI) Error(msg string)                                       // stderr: Red(msg)+"\n"
```

Wiring (1 edit, `internal/cmd/default_action.go`): build the UI from `cfg.NoColor` + the cobra writers
(TTY check on the real `os.Stdout`), emit the `↳ Generating…` progress line before generation, and route
the FR18 stderr notice + generic-error stderr line through the color helpers. The stdout DATA surface
(`printCommitReport`, `printDryRunMessage`) is UNCHANGED — it stays plain.

### Success Criteria

- [ ] `internal/ui/output.go` exists, `package ui`, **stdlib-only** imports (`fmt`, `io`, `os`). No
      stagecoach imports (F6) and no new `go.mod` dependencies.
- [ ] `IsTerminal(*os.File) bool` returns true for a char device, false for a pipe/file (stat-error→false).
- [ ] `ResolveColor(noColor, isTTY)` returns true iff `!noColor && !noColorEnvSet() && isTTY`;
      `noColorEnvSet()` reads `NO_COLOR` via `os.LookupEnv` with `ok && v != ""` (matches `config/load.go:112`).
- [ ] `Green/Red/Yellow` wrap with ANSI (`\x1b[3{1,2,3}m…\x1b[0m`) when `color` is true; return the
      string unchanged when false (zero ANSI bytes).
- [ ] `Progress` writes exactly `"↳ " + msg + "\n"` to the **stderr** writer; `Success` writes
      `Green("↳ "+msg)+"\n"` to stderr; `Error` writes `Red(msg)+"\n"` to stderr. (`↳` = U+21B3; `…` = U+2026.)
- [ ] `default_action.go` constructs `ui.New(stdout, stderr, ui.ResolveColor(cfg.NoColor, ui.IsTerminal(os.Stdout)))`
      and emits `u.Progress(<generating-label>)` immediately before `stagecoach.GenerateCommit(...)`; the
      FR18 notice + generic-error line use the UI's color helpers. `printCommitReport`/`printDryRunMessage`
      are UNCHANGED (stdout plain).
- [ ] `go test -race ./internal/ui/ -v` green; `go test -race ./...` green (NO `default_action_test.go`
      edits; its stdout-equality + stderr-Contains assertions still pass); `go vet ./...` clean;
      `gofmt -l internal/ui/ internal/cmd/` empty; only the 3 listed files changed (`git status`).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact upstream
seams (all quoted below + in research/design-decisions.md D1–D10 / F1–F6), the governing stream contract
(stdout plain / stderr colored, locked by `default_action_test.go` — quoted in F3), the NO_COLOR spec
(external-research.md §1), the copy-ready `output.go` skeleton in the Implementation Blueprint, and the
test conventions to mirror (`root_test.go`'s `bytes.Buffer` capture + `t.Setenv`). No signal/generate/
verbose/exit-code knowledge required (all explicitly out of scope — D10).

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M4T3S1/research/design-decisions.md
  why: the 10 decisions (D1–D10) + 6 findings (F1–F6) SPECIFIC to this subtask. D2 (the pure
       ResolveColor decouples untestable TTY from testable flag/env logic — the work item's exact ask),
       D3 (stdlib ModeCharDevice, NO new dep), D4 (NO_COLOR idiom matches config/load.go:112), D6 (only
       stderr carries ANSI — forced by default_action_test.go's exact stdout equality), D9 (default_action.go
       is safe to edit vs the parallel T2.S1), D10 (frozen/out-of-scope list).
  critical: D2, D6, D9, D10 (the scope + stream boundaries); F1 (cfg.NoColor already resolved, we add
       NO_COLOR+TTY), F2 (bare NO_COLOR NOT handled yet), F3 (the locked test assertions), F5 (provider
       name unknown pre-call → defensive label).

- docfile: plan/001_f1f80943ac34/P1M4T3S1/research/external-research.md
  why: the NO_COLOR spec (§1 — "present and not an empty string"), the stdlib TTY heuristic + its
       accepted gotcha (§2), the ANSI SGR table (§3), the ↳ U+21B3 glyph + … U+2026 (§4), and the stream
       discipline proof (§5 — why stdout MUST stay plain).
  critical: §1 (the exact `os.LookupEnv` idiom), §2 (IsTerminal body — copy verbatim), §5 (the
       Contains-vs-exact-equality reasoning that makes stderr-coloring safe and stdout-coloring illegal).

- docfile: plan/001_f1f80943ac34/P1M4T2S1/PRP.md
  why: the PARALLEL sibling. Its "SCOPE BOUNDARY" lists default_action.go as do-not-touch (so our edit
       there cannot conflict) and it edits ONLY main.go/executor.go/generate.go/stagecoach.go (which we
       do NOT touch). Its signal handler writes the §18.3 rescue PLAIN to os.Stderr ("P1.M4.T3 may
       colorize later" — we deliberately do NOT, to keep the frozen message byte-for-byte).
  critical: do NOT edit main.go/generate.go/executor.go/stagecoach.go (T2.S1 owns them this cycle).

- file: internal/cmd/default_action.go   (P1.M4.T1.S2 — runDefault + isolated print helpers; S1 EDITS this)
  section: `func runDefault(cmd, args) error` — the lines `stdout := cmd.OutOrStdout()`,
       `stderr := cmd.ErrOrStderr()`, `cfg := Config()` (the UI's inputs), the `stagecoach.GenerateCommit(...)`
       call (the Progress insert point — right BEFORE it), the FR18 `fmt.Fprintf(stderr, "Nothing staged —
       staging all changes (%d files).\n", n)` (color-route target), and `handleGenError`'s generic
       `return exitcode.New(exitcode.Error, err)` tail (the Error insert point).
  why: S1 adds (a) the UI construction 1-liner after `cfg := Config()`; (b) `u.Progress(<label>)` right
       before `stagecoach.GenerateCommit`; (c) color-wraps the FR18 notice string; (d) routes the generic
       error through `u.Error`. `printCommitReport`/`printDryRunMessage` are UNCHANGED (stdout plain).
  pattern: see "Integration Points" + Implementation Blueprint for the exact edits (oldText→newText).
  gotcha: TTY check uses `os.Stdout` (the real file), NOT the `stdout` writer (which is a `*bytes.Buffer`
       in tests → would mis-detect as non-TTY; actually that's the DESIRED test behavior, but the prod
       path must check the real fd). do NOT colorize the rescue path in `handleGenError` (frozen §18.3).

- file: internal/cmd/default_action_test.go   (P1.M4.T1.S2 — READ; the stream-contract LOCK)
  section: `TestRunDefault_DryRun` asserts `strings.TrimSpace(outBuf.String()) == "feat: dry run"` (stdout
       EXACT, plain); `TestRunDefault_HappyPath` asserts `Contains(stdout, "] feat: add login")` +
       `Contains(stdout, "A  new.txt")`; `TestRunDefault_AutoStageAll` asserts
       `Contains(stderr, "Nothing staged — staging all changes (2 files).")`.
  why: PROVES (a) stdout must stay plain + undecorated (the dry-run equality); (b) the Progress line MUST
       go to stderr (else it'd land in outBuf and break `== "feat: dry run"`); (c) ANSI-wrapping the FR18
       notice is SAFE (Contains survives wrapping). Read this before editing default_action.go.
  gotcha: do NOT edit this file. If a test fails after your edit, your Progress line leaked to stdout or
       you colored stdout — fix the implementation, not the test.

- file: internal/config/config.go   (P1.M1.T4.S1 — Config.NoColor; READ, do NOT edit)
  section: `NoColor bool \`toml:"-"\`` (line 24) + its comment "TTY-aware at runtime; set by UI layer
       (P1.M4.T3.S1)". And `Defaults()` sets `NoColor: false`.
  why: THIS is the field `ResolveColor` consumes. It's already resolved (Layer 5 STAGECOACH_NO_COLOR +
       Layer 7 --no-color). The UI layer is the "P1.M4.T3.S1" the comment names — we add TTY + NO_COLOR.
  gotcha: do NOT move the NO_COLOR logic into config (config is the resolved snapshot; the bare NO_COLOR
       env + TTY are runtime/IO concerns that belong in `internal/ui`). Keep `config.NoColor` as-is.

- file: internal/config/load.go   (P1.M1.T4.S4 — the env/flag idiom to COPY; READ, do NOT edit)
  section: line 112 `if v, ok := os.LookupEnv("STAGECOACH_NO_COLOR"); ok && v != "" {` — the EXACT idiom
       `internal/ui/noColorEnvSet()` must mirror for the bare `NO_COLOR` var.
  why: consistency — `NO_COLOR` and `STAGECOACH_NO_COLOR` use the same `ok && v != ""` rule, so a user's
       mental model transfers. Copy the idiom verbatim (just change the var name).
  gotcha: NO_COLOR is a SEPARATE var from STAGECOACH_NO_COLOR. config handles STAGECOACH_+--; ui handles
       the bare NO_COLOR. Both OR into "disable color".

- file: internal/cmd/root_test.go   (P1.M4.T1.S1 — READ; mirror its test conventions)
  section: `bytes.Buffer` capture (`var outBuf, errBuf bytes.Buffer; rootCmd.SetOut(&outBuf);
       rootCmd.SetErr(&errBuf)`), `t.Setenv(...)` for env tests, `saveRootState`/`restoreRootState` +
       `resetFlags` (pflag doesn't reset between Parses).
  why: `internal/ui/output_test.go` is SIMPLER (pure unit tests — no cobra), but mirror the buffer-capture
       + `t.Setenv` style. The UI's `New(&out, &err, bool)` makes capturing trivial (no cobra needed).

- url: https://no-color.org
  section: the single-paragraph spec ("present and not an empty string").
  why: the AUTHORITATIVE rule for `noColorEnvSet()`. Quoted verbatim in external-research.md §1.
  critical: `NO_COLOR=""` does NOT disable; any non-empty value DOES. (Current spec, post-2023 update.)

- url: https://pkg.go.dev/os#FileStat  and  https://pkg.go.dev/os#pkg-constants
  section: `ModeCharDevice`.
  why: the stdlib TTY heuristic — `(stat.Mode() & os.ModeCharDevice) != 0`. No `golang.org/x/term` dep.
  critical: stat-error → return false (treat as non-TTY → color off → safe default).
```

### Current Codebase tree (relevant slice)

```bash
go.mod                              # module github.com/dustin/stagecoach ; go 1.22 ; UNCHANGED (no new deps)
internal/
  cmd/default_action.go             # P1.M4.T1.S2 — runDefault + printCommitReport/printDryRunMessage/handleGenError  (S1 EDITS: +UI construct, +Progress, color notices)
  cmd/default_action_test.go        # P1.M4.T1.S2 — stream-contract assertions (READ; do NOT edit)
  cmd/root.go                       # P1.M4.T1.S1 — flags incl. --no-color (flagNoColor)  (UNCHANGED)
  config/config.go                  # P1.M1.T4.S1 — Config.NoColor (toml:"-")  (UNCHANGED — consumed)
  config/load.go                    # P1.M1.T4.S4 — STAGECOACH_NO_COLOR + --no-color idiom  (UNCHANGED — READ to copy)
  generate/rescue.go                # P1.M3.T3.S1 — FormatRescue (frozen §18.3)  (UNCHANGED — do NOT colorize)
  exitcode/exitcode.go              # P1.M4.T1.S1 — For/ExitError  (UNCHANGED)
  ui/                               # ← DOES NOT EXIST YET (S1 creates it)
cmd/stagecoach/main.go               # P1.M4.T1.S1 — main  (UNCHANGED — T2.S1 owns main.go this cycle)
Makefile                            # build/test(-race)/coverage/lint/clean  (UNCHANGED)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/ui/output.go               # NEW — package ui: IsTerminal, ResolveColor, UI(New/Color/Green/Red/Yellow/Progress/Success/Error).
internal/ui/output_test.go          # NEW — unit tests (color matrix + NO_COLOR env + ANSI wrap/no-op + Progress prefix/stream + IsTerminal-on-pipe).
internal/cmd/default_action.go      # EDIT — construct UI; emit Progress before GenerateCommit; color-route FR18 notice + generic error. stdout DATA unchanged.
# All other files UNCHANGED. root.go/config*.go/rescue.go/exitcode.go/main.go UNCHANGED.
# generate.go/executor.go/stagecoach.go UNCHANGED (parallel T2.S1 owns them).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (stdout MUST stay plain — FR51 + default_action_test.go exact equality): NEVER colorize
// printCommitReport or printDryRunMessage, and NEVER route Progress/Success/Error to the stdout writer.
// TestRunDefault_DryRun asserts TrimSpace(stdout) == "feat: dry run" EXACTLY — a single ANSI byte or a
// stray "↳ Generating" on stdout breaks it. ALL color + ALL "↳ " progress → STDERR only.

// CRITICAL (the bare NO_COLOR env is the UI layer's job): grep confirms config handles only
// STAGECOACH_NO_COLOR. The Config.NoColor comment explicitly delegates the final TTY/NO_COLOR resolution
// to "P1.M4.T3.S1". Implement noColorEnvSet() in internal/ui (do NOT edit config).

// CRITICAL (TTY check on os.Stdout, not the cobra writer): in tests rootCmd.SetOut(&buf) makes the
// stdout WRITER a *bytes.Buffer. IsTerminal must take *os.File — pass os.Stdout (the real fd) at the
// runDefault call site. (In tests os.Stdout is the runner's fd — usually non-TTY → color off → plain,
// which is exactly why the stream tests pass without a pty.)

// GOTCHA (NO_COLOR empty-string semantics): use `v, ok := os.LookupEnv("NO_COLOR"); ok && v != ""`.
// NO_COLOR="" (set without a value) does NOT disable color. This is the current no-color.org spec AND
// matches config/load.go:112's STAGECOACH_NO_COLOR idiom — copy it verbatim.

// GOTCHA (IsTerminal is a heuristic, not a true isatty): ModeCharDevice != ioctl TCGETS. A rare
// non-terminal char device could enable color. Accepted (--no-color/NO_COLOR remain authoritative) to
// AVOID adding golang.org/x/term (project stdlib-only invariant; procgroup_windows.go sets the precedent).
// The signature IsTerminal(*os.File) is stable — swap the body later if precision ever matters.

// GOTCHA (↳ is U+21B3, … is U+2026): write the literal Unicode chars in the Go source (Go source is
// UTF-8). Do NOT use three ASCII dots for the ellipsis, and do NOT escape ↳ as \u21b3 (a literal is
// clearer and matches the PRD Appendix B bytes). The Progress prefix is "↳ " (glyph + ONE space).

// GOTCHA (colorize the FR18 notice is SAFE, but keep the text verbatim): default_action_test.go asserts
// Contains(stderr, "Nothing staged — staging all changes (2 files)."). ANSI-wrapping that string
// (e.g. Yellow("Nothing staged — …")) leaves the plaintext substring intact → Contains still passes. But
// the em-dash "—" (U+2014) and the "(2 files)." must stay byte-identical. Do NOT change the wording.

// GOTCHA (do NOT touch the rescue path): handleGenError prints generate.FormatRescue(...) to stderr. That
// message is FROZEN (§18.3, byte-for-byte; the ❌ is U+274C). Do NOT wrap it in Red(). It is intentionally
// plain. (The parallel T2.S1 signal handler also writes it plain.) Colorizing would corrupt the frozen text.

// GOTCHA (do NOT edit generate.go / executor.go / main.go / stagecoach.go): P1.M4.T2.S1 is editing those
// files IN PARALLEL this cycle. default_action.go is the ONLY file both this task edits AND that is safe
// (T2.S1's scope boundary lists it as a do-not-touch sibling). Keep your diff to internal/ui/* + default_action.go.

// GOTCHA (provider name unknown pre-call): cfg.Provider is "" in the common auto-detect case (the real
// name is resolved deep in the registry/generate pipeline). Build the Progress label defensively:
// "Generating" + (" with "+provider if != "") + ((" ("+model+")") if != "") + "…". Auto → "↳ Generating…".
// (The precise "with <resolved-provider>" mid-pipeline line needs a generate.go callback — out of scope.)
```

## Implementation Blueprint

### Data models and structure

```go
// internal/ui/output.go
package ui

import (
	"fmt"
	"io"
	"os"
)

// ANSI SGR codes (Select Graphic Rendition). Emitted ONLY when u.color is true.
const (
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
)

// progressPrefix is the Appendix-B "↳" glyph (U+21B3) + one space, prefixing every progress line.
const progressPrefix = "↳ "

// IsTerminal reports whether f is a terminal (character device). Stdlib-only TTY heuristic: a real
// terminal/pty is a char device; a pipe, file, or redirect is not. Sufficient for FR51 color gating.
// stat-error → false (treat as non-TTY → color off → the safe default). NOT a true isatty ioctl;
// --no-color / NO_COLOR remain the authoritative overrides. Signature is stable for a future
// golang.org/x/term swap (out of v1 scope — project stays dep-free; see procgroup_windows.go).
func IsTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// noColorEnvSet reports whether the NO_COLOR convention (https://no-color.org) disables color: the var
// is present AND not an empty string. Byte-identical idiom to config/load.go's STAGECOACH_NO_COLOR
// handling (line 112) — kept consistent so a user's mental model transfers between the two vars.
func noColorEnvSet() bool {
	v, ok := os.LookupEnv("NO_COLOR")
	return ok && v != ""
}

// ResolveColor decides whether ANSI color should be emitted (PRD §9.13 FR51, §15.2). Color is ON iff:
// cfg.NoColor is false (--no-color / STAGECOACH_NO_COLOR, already resolved by config.Load) AND the bare
// NO_COLOR env var is unset/empty (https://no-color.org) AND stdout is a terminal. The TTY check is
// PASSED IN (isTTY) so the untestable IO is decoupled from the testable flag/env logic (work item:
// "can't easily test TTY in unit tests; test the flag/env logic"). Callers pass isTTY = IsTerminal(os.Stdout).
func ResolveColor(noColor bool, isTTY bool) bool {
	if noColor {
		return false
	}
	if noColorEnvSet() {
		return false
	}
	return isTTY
}

// UI renders Stagecoach's CLI output with optional ANSI color (PRD §9.13, Appendix B). Progress/Success/
// Error go to STDERR (FR51: stdout stays clean for piping); the actual RESULT data (commit report,
// dry-run message) stays PLAIN on stdout via the caller's own print path — never thread it through here.
// Writers are injectable (cobra's cmd.OutOrStdout/ErrOrStderr in prod, *bytes.Buffer in tests); color is
// a resolved bool passed to New (tests pass true/false directly — no real TTY needed).
type UI struct {
	stdout io.Writer
	stderr io.Writer
	color  bool
}

// New constructs a UI writing to the given streams with the given color resolution. From the CLI:
// ui.New(stdout, stderr, ui.ResolveColor(cfg.NoColor, ui.IsTerminal(os.Stdout))). From tests:
// ui.New(&outBuf, &errBuf, true|false). nil writers default to os.Stdout/os.Stderr.
func New(stdout, stderr io.Writer, color bool) *UI {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return &UI{stdout: stdout, stderr: stderr, color: color}
}

// Color reports whether color is enabled (for conditional callers / tests).
func (u *UI) Color() bool { return u.color }

// Green/Red/Yellow wrap s in the ANSI color iff color is enabled; otherwise return s unchanged (zero
// ANSI bytes — keeps `git commit -F <(stagecoach --dry-run)` and `| tee` clean).
func (u *UI) Green(s string) string  { return u.colorize(ansiGreen, s) }
func (u *UI) Red(s string) string    { return u.colorize(ansiRed, s) }
func (u *UI) Yellow(s string) string { return u.colorize(ansiYellow, s) }

func (u *UI) colorize(code, s string) string {
	if !u.color {
		return s
	}
	return code + s + ansiReset
}

// Progress writes a progress line to STDERR with the Appendix-B "↳ " prefix (FR51: stdout stays clean
// for piping). Example: Progress("Generating with pi (glm-5.2)…") -> "↳ Generating with pi (glm-5.2)…\n".
func (u *UI) Progress(msg string) {
	fmt.Fprintln(u.stderr, progressPrefix+msg)
}

// Success writes a success notice to STDERR in green (when color): the Appendix-B "↳ " prefix + msg.
// Example: Success("Created abc1234") -> green "↳ Created abc1234". (The data report itself stays plain
// on stdout via the caller's print path.)
func (u *UI) Success(msg string) {
	fmt.Fprintln(u.stderr, u.Green(progressPrefix+msg))
}

// Error writes an error notice to STDERR in red (when color). Example: Error("generation failed").
// (The frozen §18.3 rescue block is NOT routed through here — it stays plain; see handleGenError.)
func (u *UI) Error(msg string) {
	fmt.Fprintln(u.stderr, u.Red(msg))
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/ui/output.go (package ui, stdlib-only)
  - FILE: NEW internal/ui/output.go. PACKAGE: `package ui`. Copy the "Data models" skeleton VERBATIM.
  - IMPORTS (stdlib ONLY): fmt, io, os. NO stagecoach imports (F6) — guarantees no import cycle.
  - DEFINE: the 4 ANSI consts + progressPrefix ("↳ ", U+21B3); IsTerminal(*os.File); noColorEnvSet();
      ResolveColor(noColor, isTTY bool) bool; type UI{stdout,stderr io.Writer; color bool}; New();
      Color(); Green/Red/Yellow(); colorize(); Progress/Success/Error().
  - GOTCHA: ↳ is the literal U+21B3 char in source (UTF-8); … is literal U+2026. Do NOT use \u escapes
      or ASCII "...". Progress prefix = "↳ " (glyph + ONE space).
  - GOTCHA: noColorEnvSet() uses `v, ok := os.LookupEnv("NO_COLOR"); ok && v != ""` (copy load.go:112's
      idiom). NO_COLOR="" does NOT disable.
  - GOTCHA: ResolveColor is a PURE function (no IO except noColorEnvSet) — testable without a TTY.

Task 2: CREATE internal/ui/output_test.go (pure unit tests; no real TTY needed)
  - FILE: NEW internal/ui/output_test.go. PACKAGE: `package ui` (internal tests — can call unexported
      noColorEnvSet). Mirror root_test.go's bytes.Buffer + t.Setenv style (but NO cobra needed).
  - TESTS:
      • TestResolveColor_Logic: matrix (noColor,isTTY) → {(T,*)->F, (F,T)->T, (F,F)->F}. NO_COLOR unset.
      • TestResolveColor_NoColorEnv: t.Setenv("NO_COLOR","1")→F; t.Setenv("NO_COLOR","")→ isTTY-gated
        (env empty does NOT disable); unset → isTTY-gated. Assert independence from STAGECOACH_NO_COLOR.
      • TestColorHelpers_NoOpWhenColorOff: New(&out,&err,false) → Green("x")=="x"; Progress("m") writes
        "↳ m\n" PLAIN to err (not out); Success/Error plain.
      • TestColorHelpers_WrapWhenColorOn: New(&out,&err,true) → Green("x")=="\x1b[32mx\x1b[0m";
        Success("c") writes "\x1b[32m↳ c\x1b[0m\n" to err; Error("e") writes "\x1b[31me\x1b[0m\n".
      • TestProgress_PrefixAndStream: Progress writes ONLY to the stderr writer (out buf empty); exact
        bytes "↳ Generating…\n" (↳=U+21B3, …=U+2026).
      • TestIsTerminal_Pipe: r,w := os.Pipe(); IsTerminal(r)==false (the deterministic non-TTY case).
        (A real TTY can't be opened portably in a unit test — the work item's acknowledged gap.)
  - GOTCHA: t.Setenv auto-restores on cleanup — no manual unset needed. Each subtest sets its own env.
  - COVERAGE: every exported symbol + noColorEnvSet. Assert BOTH the colored AND plain byte sequences.

Task 3: EDIT internal/cmd/default_action.go (wire the UI — 4 small changes)
  - FILE: EDIT internal/cmd/default_action.go. ADD import "github.com/dustin/stagecoach/internal/ui".
  - CHANGE A (construct): right after `cfg := Config() { … }` (the block that returns on cfg==nil),
      add:
        u := ui.New(stdout, stderr, ui.ResolveColor(cfg.NoColor, ui.IsTerminal(os.Stdout)))
      (stdout/stderr are already `cmd.OutOrStdout()`/`cmd.ErrOrStderr()` from the top of runDefault.)
  - CHANGE B (Progress before generation): immediately before `res, err := stagecoach.GenerateCommit(...)`,
      add a defensive label + Progress call:
        label := "Generating"
        if cfg.Provider != "" {
            label += " with " + cfg.Provider
            if cfg.Model != "" {
                label += " (" + cfg.Model + ")"
            }
        }
        label += "…"
        u.Progress(label)
      (Both commit and dry-run paths emit "↳ Generating…" on stderr — matches Appendix B.1/B.3. SAFE:
      stderr only; doesn't touch the dry-run stdout equality.)
  - CHANGE C (colorize FR18 notice): the `fmt.Fprintf(stderr, "Nothing staged — staging all changes
      (%d files).\n", n)` line → wrap the formatted string in u.Yellow(...) (or keep fmt.Fprintf but
      write u.Yellow(fmt.Sprintf(...))). Keep the text BYTE-IDENTICAL (em-dash —, "(%d files).").
      SAFE: Contains(stderr, "Nothing staged — staging all changes (2 files).") survives ANSI wrapping.
  - CHANGE D (route generic error): in handleGenError's final `return exitcode.New(exitcode.Error, err)`,
      the err.Error() is currently printed by main as "stagecoach: <msg>". Option: leave main's printing
      as-is (simplest, no main.go edit) OR, for a colored notice, this is OPTIONAL — main.go is owned by
      T2.S1 this cycle, so DO NOT edit main.go. Instead, if you want the generic error colored, print it
      from handleGenError via `u.Error(err.Error())` and return a SILENT exitcode.New(exitcode.Error, nil)
      so main doesn't double-print. (This mirrors the rescue/CAS silent pattern already in handleGenError.)
      If unsure, SKIP CHANGE D (leave the generic path to main's plain print) — it is non-essential.
  - PRESERVE: printCommitReport, printDryRunMessage UNCHANGED (stdout plain). handleGenError's rescue
      (FormatRescue) + CAS paths UNCHANGED (frozen). The --no-auto-stage / clean-tree returns UNCHANGED.
  - GOTCHA: do NOT touch main.go (T2.S1 owns it). do NOT colorize the rescue block. do NOT route anything
      to the stdout writer.
  - GOTCHA: re-run default_action_test.go after editing — if TestRunDefault_DryRun fails with stdout
      containing "↳", your Progress leaked to stdout (fix: ensure u writes to `stderr`, the cobra ErrOrStderr).
```

### Implementation Patterns & Key Details

```go
// The color gate is the heart of this task. Keep it PURE (testable):
func ResolveColor(noColor bool, isTTY bool) bool {
	if noColor {        // --no-color / STAGECOACH_NO_COLOR (cfg.NoColor, already resolved by config.Load)
		return false
	}
	if noColorEnvSet() { // bare NO_COLOR env (https://no-color.org) — THIS task adds it
		return false
	}
	return isTTY         // os.Stdout is a char device (stdlib ModeCharDevice)
}

// runDefault wiring (the safe edit). After `cfg := Config()`:
u := ui.New(stdout, stderr, ui.ResolveColor(cfg.NoColor, ui.IsTerminal(os.Stdout)))

// Before GenerateCommit (defensive label — provider name unknown pre-call when auto):
label := "Generating"
if cfg.Provider != "" {
	label += " with " + cfg.Provider
	if cfg.Model != "" {
		label += " (" + cfg.Model + ")"
	}
}
label += "…" // U+2026
u.Progress(label) // → stderr: "↳ Generating…"  (↳ U+21B3)

// FR18 notice colorization (text BYTE-IDENTICAL; only optionally wrapped):
fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d files).", n)))
```

### Integration Points

```yaml
PACKAGE LAYOUT (PRD §14):
  - create: internal/ui/output.go   # §14 literally lists "internal/ui/output.go # progress messages, color, TTY detect"
  - note: the §14 "internal/ui/exitcode.go" slot was implemented as internal/exitcode/ (P1.M4.T1.S1) — NOT duplicated here.

CONFIG (consumed, NOT modified):
  - field: config.Config.NoColor (toml:"-") — already resolved by config.Load (Layer 5 STAGECOACH_NO_COLOR
    + Layer 7 --no-color). ResolveColor reads it; do NOT move NO_COLOR logic into config.

CLI (wired):
  - file: internal/cmd/default_action.go
  - point: after `cfg := Config()` → construct UI; before `stagecoach.GenerateCommit` → u.Progress(label);
    FR18 notice → u.Yellow(...); (optional) generic error → u.Error(...)+silent exitcode.
  - preserve: printCommitReport / printDryRunMessage (stdout plain); handleGenError rescue/CAS (frozen).

ENV VARS (handled by internal/ui, NEW):
  - NO_COLOR: present-and-non-empty → disable color (https://no-color.org). Separate from STAGECOACH_NO_COLOR.

PARALLEL COORDINATION (P1.M4.T2.S1 — do NOT touch its files):
  - main.go / internal/provider/executor.go / internal/generate/generate.go / pkg/stagecoach/stagecoach.go
    are being edited by the signal-handler sibling THIS cycle. default_action.go is the shared-safe file
    (T2.S1's scope boundary lists it as do-not-touch; it does not edit CLI output).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After creating output.go — fix before proceeding.
gofmt -w internal/ui/output.go internal/ui/output_test.go
go vet ./internal/ui/

# After editing default_action.go
gofmt -w internal/cmd/default_action.go
go vet ./internal/cmd/

# Expected: zero errors. gofmt -l should be empty for both trees.
gofmt -l internal/ui/ internal/cmd/
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test the UI package in isolation (the core deliverable).
go test -race ./internal/ui/ -v

# Expected: all green. Covers ResolveColor matrix, NO_COLOR env, ANSI wrap/no-op, Progress prefix+stream,
# IsTerminal-on-a-pipe. NO real TTY required (D2 decouples the env/flag logic).

# Full CLI suite — proves the default_action.go wiring didn't break the stream contract.
go test -race ./internal/cmd/ -v

# Expected: TestRunDefault_DryRun still asserts stdout == "feat: dry run" (Progress went to stderr);
# TestRunDefault_HappyPath still Contains "] feat: add login" + "A  new.txt" (stdout plain);
# TestRunDefault_AutoStageAll still Contains the FR18 notice (ANSI wrap survives Contains).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the real binary.
make build

# In a scratch git repo with a staged change + a stub/fake provider (or --dry-run with a configured agent):
cd /tmp && rm -rf ui-smoke && git init ui-smoke && cd ui-smoke &&
  git config user.email t@t.co && git config user.name t &&
  echo hi > a.txt && git add a.txt &&
  NO_COLOR=1 ./path/to/bin/stagecoach --dry-run --no-color 2>err.txt 1>out.txt

# Assertions:
#   - out.txt == the generated message ONLY (no "↳", no ANSI, no "(no commit created)").  [FR51 / §15.5]
#   - err.txt contains "↳ Generating…" (the Progress line, PLAIN under NO_COLOR=1 / --no-color).
#   - `cat -v err.txt` shows NO "^[[32m"/"^[[31m" escapes when NO_COLOR=1 or piped (color off).
#   - Run WITHOUT NO_COLOR on a real TTY → err.txt (via a pty) shows ANSI escapes; piped → plain.

# TTY/color toggle sanity (interactive terminal):
#   stagecoach --dry-run            # color ON (TTY), "↳ Generating…" on stderr (green/yellow)
#   stagecoach --dry-run --no-color # color OFF, plain text
#   NO_COLOR=1 stagecoach --dry-run # color OFF (the convention)
#   stagecoach --dry-run | cat      # color OFF (stdout is a pipe → !isTTY), message clean on stdout

# Expected: stdout ALWAYS clean; stderr colored only on a real TTY with no disable signal.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Pipe-safety proof (the FR51 motivating use case — §15.5):
./path/to/bin/stagecoach --dry-run --no-color 2>/dev/null | tee /tmp/msg.txt
# /tmp/msg.txt must be the bare message with ZERO ANSI bytes:
grep -P '\x1b' /tmp/msg.txt && echo "FAIL: ANSI leaked into stdout" || echo "PASS: stdout is clean"

# git-commit-from-stdin proof:
#   git commit -F <(./path/to/bin/stagecoach --dry-run --no-color 2>/dev/null)
# (commit succeeds — the message has no control codes.)

# Race + full regression (the gate):
go test -race ./...
go vet ./...
gofmt -l internal/ pkg/ cmd/

# Expected: all green; only internal/ui/output.go, internal/ui/output_test.go, internal/cmd/default_action.go
# changed (git status). generate.go/executor.go/main.go/stagecoach.go UNCHANGED (parallel T2.S1 owns them).
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go test -race ./internal/ui/ -v` green (the core deliverable).
- [ ] `go test -race ./...` green — NO regression; `default_action_test.go` UNCHANGED and still passing.
- [ ] `go vet ./...` clean; `gofmt -l internal/ui/ internal/cmd/` empty.
- [ ] No new `go.mod` dependencies (stdlib-only — `gofmt`/`grep` confirm no `golang.org/x/term`).

### Feature Validation

- [ ] `IsTerminal` detects char-device vs pipe; `ResolveColor` folds cfg.NoColor + NO_COLOR + TTY.
- [ ] `NO_COLOR` env (present + non-empty) disables color; `--no-color`/`STAGECOACH_NO_COLOR` disable color.
- [ ] `Green/Red/Yellow` emit ANSI iff color on; return plain iff off (zero ANSI bytes when off).
- [ ] `Progress` writes `"↳ " + msg + "\n"` (↳ U+21B3) to STDERR; `Success`/`Error` to STDERR (green/red).
- [ ] `default_action.go` emits `↳ Generating…` before generation; FR18 notice optionally colored; stdout
      DATA (commit report / dry-run message) stays PLAIN.
- [ ] Pipe-safety: `stagecoach --dry-run --no-color | tee` yields a clean message with zero ANSI bytes.

### Code Quality Validation

- [ ] `internal/ui` imports only stdlib (no stagecoach package — F6; no import-cycle risk).
- [ ] Follows existing patterns: `config/load.go:112` NO_COLOR idiom; `root_test.go` buffer/t.Setenv style.
- [ ] File placement matches the desired tree (`internal/ui/output.go` per PRD §14).
- [ ] Anti-patterns avoided (see below): no stdout color, no new dep, no rescue colorization, no main.go edit.

### Documentation & Deployment

- [ ] Code is self-documenting (doc comments on every exported symbol; the NO_COLOR/TTY rationale inlined).
- [ ] No new env vars beyond the standard `NO_COLOR` (documented in the §15.2 flag help already).

---

## Anti-Patterns to Avoid

- ❌ Don't colorize stdout — it breaks `stagecoach --dry-run | tee` / `git commit -F <(stagecoach --dry-run)`
  (§15.5) AND `default_action_test.go`'s exact `stdout == "feat: dry run"` equality. Color → STDERR only.
- ❌ Don't add `golang.org/x/term` (or any dep) for TTY detection — the project is stdlib-only
  (`procgroup_windows.go` sets the precedent). Use `os.FileStat` + `ModeCharDevice`.
- ❌ Don't colorize the §18.3 rescue block (`generate.FormatRescue`) — it's FROZEN byte-for-byte; the ❌
  is intentional. Leave `handleGenError`'s rescue path plain.
- ❌ Don't edit `main.go` / `generate.go` / `executor.go` / `stagecoach.go` — P1.M4.T2.S1 owns them this
  cycle (parallel). `default_action.go` is the only shared-safe edit.
- ❌ Don't move the bare `NO_COLOR` handling into `config` — config is the resolved snapshot; the bare
  NO_COLOR env + the TTY check are runtime/IO concerns that belong in `internal/ui` (per the
  `Config.NoColor` comment delegating to "P1.M4.T3.S1").
- ❌ Don't conflate `NO_COLOR` with `STAGECOACH_NO_COLOR` — they're separate vars that OR together to
  disable; both use the `ok && v != ""` idiom but live in different packages.
- ❌ Don't make `ResolveColor` do IO directly that defeats testing — keep the TTY check as a PASSED-IN
  bool (`isTTY`) so the flag/env logic is unit-testable without a pty (the work item's explicit ask).
- ❌ Don't use ASCII "..." for the ellipsis or `\u21b3` for ↳ — use the literal Unicode chars (… U+2026,
  ↳ U+21B3) to match PRD Appendix B bytes exactly.
