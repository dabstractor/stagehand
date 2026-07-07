---
name: "P1.M4.T3.S2 — Verbose mode (resolved command, raw output, retries to stderr) — PRD §9.13 FR50, §15.2, §19, Appendix B.4"
description: |

  Ship Stagecoach's `--verbose` / `-v` / `STAGECOACH_VERBOSE=1` diagnostics (PRD §9.13 FR50, §15.2,
  §19): when verbose is on, print to STDERR (1) the resolved provider COMMAND (argv only — NEVER
  env, per §19 secret-handling), (2) the raw agent STDOUT, and (3) each RETRY attempt — each line
  prefixed `DEBUG: ` (the commit-pi convention named in the work-item contract). Implemented as a
  new nil-safe `*ui.Verbose` sink in a NEW file `internal/ui/verbose.go` (sibling to P1.M4.T3.S1's
  `output.go` — zero merge-conflict), threaded through `generate.Deps` and a new nil-safe param on
  `provider.Execute`, exposed additively on the public `pkg/stagecoach.Options` as `Verbose
  io.Writer`, and wired into the CLI via one line in `runDefault`.

  CONTRACT (work-item spec — verbatim):
    1. RESEARCH: "PRD FR50 / §15.2. --verbose/-v / STAGECOACH_VERBOSE=1 prints: the resolved
       provider command, the raw agent stdout, and each retry attempt to stderr. commit-pi uses
       VERBOSE=1 with DEBUG: prefix lines. STAGECOACH_VERBOSE=2 could print stdin contents (§19
       notes)."
    3. LOGIC: "Add verbose logging functions: `VerboseCommand(cmd string)`,
       `VerboseRawOutput(output string)`, `VerboseRetry(attempt int, reason string)` — all write to
       stderr only when verbose is true. Wire these into the executor (log the rendered command) and
       the orchestrator (log each attempt/retry). Mock: unit test that verbose output appears on
       stderr when enabled."
    4. OUTPUT: "Verbose logging wired into the executor (P1.M2.T5) and orchestrator (P1.M3.T4)."

  INPUT (upstream — all EXIST; CONSUME only, do NOT change their behavior):
    - `config.Config.Verbose bool` (config.go:21) — ALREADY resolved by every loader layer
      (Defaults/TOML/git-config/STAGECOACH_VERBOSE env/--verbose flag; see design-decisions.md F1).
      This task is the designated CONSUMER of that bool (config is the resolved snapshot).
    - `internal/ui` package from P1.M4.T3.S1 — owns the always-on `↳` Progress/color helpers in
      `output.go`. S2 ADDS a sibling `verbose.go`; it does NOT edit `output.go` (parallel-safe).
    - `provider.CmdSpec{Command, Args, Stdin, Env}` (render.go) — Render produces it; Execute
      consumes it. `VerboseCommand` logs `Command`+`Args` (argv), never `Env`.

  OUTPUT: the `internal/ui/verbose.go` sink + wiring through `generate.Deps`,
  `provider.Execute`, `pkg/stagecoach.Options`, and `internal/cmd/default_action.go`.

  DELIVERABLES (2 NEW files + 5 EDITS):
    NEW  internal/ui/verbose.go         — `package ui`: `type Verbose`, `NewVerbose`, nil-safe
                                          `VerboseCommand/VerboseRawOutput/VerboseRetry`.
    NEW  internal/ui/verbose_test.go    — unit tests: on→writes DEBUG lines to the injected writer;
                                          off/nil→zero bytes; exact formats.
    EDIT internal/provider/executor.go  — add nil-safe `vb *ui.Verbose` param to `Execute`; log
                                          argv (VerboseCommand) before Start + raw stdout
                                          (VerboseRawOutput) after Wait (success AND error paths).
    EDIT internal/generate/generate.go  — add `Verbose *ui.Verbose` to `Deps`; pass `deps.Verbose`
                                          to `Execute`; call `VerboseRetry` at the 2 retry sites.
    EDIT pkg/stagecoach/stagecoach.go     — add `Verbose io.Writer` to `Options`; set
                                          `deps.Verbose = ui.NewVerbose(opts.Verbose, cfg.Verbose)`;
                                          pass to the 2 `Execute` calls + add `VerboseRetry` in the
                                          runPipeline loop.
    EDIT internal/cmd/default_action.go — add `Verbose: stderr` to the `stagecoach.Options{}` call.
    EDIT internal/provider/executor_test.go — append `nil` to the 9 `Execute(...)` calls
                                          (compiler-driven; nil-safe).

  SCOPE BOUNDARY (owned by siblings — do NOT implement or edit):
    - `internal/config/*` — `cfg.Verbose` is fully resolved; do NOT change the type (bool) or
      loaders (design-decisions.md F1). VERBOSE=2/stdin is deferred (D9) — would need int config.
    - `internal/ui/output.go` — owned by P1.M4.T3.S1 (the `↳` Progress/color layer). S2 writes a
      SIBLING `verbose.go`; never edits `output.go`.
    - `generate.FormatRescue` / the §18.3 rescue block — FROZEN byte-for-byte; do NOT verbose-ize it.
    - The always-on `↳ Snapshotting…` / `↳ Attempt N … accepted` progress lines — progress/S1
      domain (S1 deferred them to a future generate.go callback). Verbose adds only `DEBUG:` lines.
    - Colorizing verbose output — verbose is plain `DEBUG:` text; color is S1's separate concern.

  DEPENDENCY GRAPH (CYCLE-FREE): `internal/ui` → stdlib only. `internal/provider` → `internal/ui`
  + `internal/signal`. `internal/generate` → `internal/{config,git,prompt,provider,signal,ui}`.
  `pkg/stagecoach` → `internal/{...}` incl. `ui`. No cycle (ui is a stdlib-only leaf).

  Deliverable: 2 NEW + 5 EDITS. `go build ./...` green; `go test -race ./internal/ui/ ./internal/provider/
  ./internal/generate/ ./internal/cmd/ ./pkg/stagecoach/ -v` green (new verbose tests pass; existing
  tests unchanged in behavior — executor_test.go gets `nil` args, default_action_test.go's
  stdout-exact assertions stay green because verbose → stderr + off in those tests); `go vet ./...`
  clean; `gofmt -l` empty for all touched trees.

---

## Goal

**Feature Goal**: Implement PRD §9.13 FR50 / §15.2 / §19 verbose diagnostics for Stagecoach: when
`cfg.Verbose` is true, emit to STDERR — with a `DEBUG: ` prefix — the resolved provider command
(argv only, never env), the raw agent stdout, and each retry attempt. Provide a clean, nil-safe,
injectable sink (`*ui.Verbose`) so the LIBRARY (`pkg/stagecoach`) stays side-effect-free by default
(writes only to a caller-supplied writer) while the CLI opts in by passing its stderr.

**Deliverable** (2 NEW files + 5 EDITS — see the description block for the full list):
1. NEW `internal/ui/verbose.go` — `package ui`:
   - `type Verbose struct { w io.Writer; on bool }`
   - `func NewVerbose(w io.Writer, on bool) *Verbose`
   - `func (v *Verbose) VerboseCommand(cmd string)` → `"DEBUG: command: " + cmd + "\n"` (nil-safe)
   - `func (v *Verbose) VerboseRawOutput(output string)` → `"DEBUG: raw output:\n" + output` (+ trailing `\n`) (nil-safe)
   - `func (v *Verbose) VerboseRetry(attempt int, reason string)` → `"DEBUG: attempt <n>: <reason>\n"` (nil-safe)
2. NEW `internal/ui/verbose_test.go` — on/off/nil matrix + exact byte assertions.
3. EDIT `internal/provider/executor.go` — `Execute` gains `vb *ui.Verbose`; logs argv + raw stdout.
4. EDIT `internal/generate/generate.go` — `Deps.Verbose *ui.Verbose`; pass to Execute; retry logging.
5. EDIT `pkg/stagecoach/stagecoach.go` — `Options.Verbose io.Writer`; construct sink; wire runPipeline.
6. EDIT `internal/cmd/default_action.go` — `Options{..., Verbose: stderr}`.
7. EDIT `internal/provider/executor_test.go` — append `nil` to the 9 `Execute(...)` calls.

**Success Definition**: `go test -race ./internal/ui/ -v` green (on writes DEBUG lines, off/nil are
zero-byte, exact formats); `go test -race ./internal/provider/ -v` green incl. a NEW
`TestExecute_Verbose` proving DEBUG command+raw-output lines land in the injected buffer; `go test
-race ./internal/generate/ -v` green incl. a NEW test proving `VerboseRetry` fires (1-based) on a
duplicate via `stubtest.NewScript`; `go test -race ./...` green with **zero** behavioral change to
existing tests (`default_action_test.go`'s `stdout == "feat: dry run"` still holds — verbose is off
there AND writes to stderr); `make build` then `./bin/stagecoach -v --dry-run` in a scratch repo
prints `DEBUG: command: …` + `DEBUG: raw output:` to **stderr** while stdout stays the bare message.

## User Persona

**Target User**: the Stagecoach CLI user (PRD §7 "the plan-holder") debugging a misbehaving agent —
"why did it generate *that*?" / "which command did it actually run?" / "why did it retry?" — and the
contributor porting/tuning a provider manifest who needs to see the rendered argv. Secondary: a Go
integrator of `pkg/stagecoach` who wants diagnostics routed to their own writer.

**Use Case**: `stagecoach -v` → see the exact `pi --model …` (or stub) command, the agent's raw
stdout (pre-parse, pre-fence-strip), and each `DEBUG: attempt N: …` retry on stderr; stdout keeps
the clean commit report (or `--dry-run` message). Then `stagecoach -v --dry-run 2>debug.log` to
capture a full diagnostic trace without polluting the piped message.

**User Journey**: user runs `stagecoach -v` → `DEBUG: command: <argv>` on stderr (confirms the
resolved provider+flags) → agent runs → `DEBUG: raw output:` on stderr (the unparsed message) → if
duplicate: `DEBUG: attempt 1: subject "…" matches an existing commit` → `DEBUG: raw output:` for
attempt 2 → commit report on stdout. With no `-v`: identical behavior, zero DEBUG output.

**Pain Points Addressed**: (1) no visibility into the rendered command (which provider/model/flags
were *actually* used after auto-detect + manifest merge) → `DEBUG: command:`; (2) no visibility
into what the agent returned before parsing/stripping → `DEBUG: raw output:`; (3) silent retries
("it just retried, why?") → `DEBUG: attempt N: <reason>`.

## Why

- **Closes PRD §9.13 FR50 (P1) + the §15.2 `--verbose`/`-v`/`STAGECOACH_VERBOSE` contract.** Without
  this, the verbose flag is wired into config (`cfg.Verbose` resolves correctly — F1) but produces
  NO output anywhere (F2: zero `DEBUG:`/`Verbose*` matches in the codebase). This task is the sole
  owner of the verbose *output*.
- **Respects §19 secret-handling.** The §19 line "Logs in --verbose print the command and flags but
  never stdin contents unless STAGECOACH_VERBOSE=2" + "never reads, logs, or transmits the agent's
  credentials" is enforced by logging **argv only, never `spec.Env`** (which carries `*_API_KEY`).
  This is the #1 security gotcha and is non-negotiable (D6).
- **Library-clean by construction.** Threading an `io.Writer` (not writing `os.Stderr` directly)
  keeps `pkg/stagecoach` side-effect-free for 3rd-party integrators; the CLI opts in via its stderr.
  Matches the injectable-writer pattern already used by S1's `internal/ui.UI` (D8).
- **Parallel-safe with P1.M4.T3.S1.** A NEW `internal/ui/verbose.go` (sibling file) means S2 never
  touches S1's `output.go` → no merge conflict whether S1 is in-flight or done (D2).

## What

A new `internal/ui/verbose.go` exposing a nil-safe `*ui.Verbose` sink:

```go
func NewVerbose(w io.Writer, on bool) *Verbose                     // w nil ⇒ all methods no-op
func (v *Verbose) VerboseCommand(cmd string)                       // "DEBUG: command: "+cmd+"\n"
func (v *Verbose) VerboseRawOutput(output string)                  // "DEBUG: raw output:\n"+out (+trailing \n)
func (v *Verbose) VerboseRetry(attempt int, reason string)         // "DEBUG: attempt <n>: "+reason+"\n"
```

Wiring (5 edits):
- **executor** (`provider.Execute`): new `vb *ui.Verbose` param; `vb.VerboseCommand(<argv>)` before
  `cmd.Start()`; `vb.VerboseRawOutput(out.String())` after `cmd.Wait()` on BOTH success and error
  returns (partial output aids diagnosis). argv = `strings.Join(append([]string{spec.Command}, spec.Args...), " ")`; **never** `spec.Env`.
- **orchestrator** (`generate.CommitStaged` + `pkg/stagecoach.runPipeline`): `deps.Verbose` passed to
  `Execute`; `deps.Verbose.VerboseRetry(attempt+1, <reason>)` at the 2 retry `continue` sites
  (parse-fail, duplicate) in EACH loop.
- **public API** (`pkg/stagecoach.Options`): additive `Verbose io.Writer`; `GenerateCommit` sets
  `deps.Verbose = ui.NewVerbose(opts.Verbose, cfg.Verbose)`.
- **CLI** (`runDefault`): `stagecoach.Options{..., Verbose: stderr}` (one line).

### Success Criteria

- [ ] `internal/ui/verbose.go` exists, `package ui`, **stdlib-only** imports (`fmt`, `io`, `os`,
      `strings`). No stagecoach imports (leaf) and no new `go.mod` deps.
- [ ] `NewVerbose(w, on)`; the 3 methods are **nil-safe** (`v==nil || v.w==nil || !v.on` → no-op, zero
      bytes/allocs); exact byte formats per the table above.
- [ ] `provider.Execute` signature is `Execute(ctx, spec, timeout, vb *ui.Verbose)`; it logs argv
      (NEVER Env) before Start and raw stdout after Wait (both paths); `vb` nil-safe.
- [ ] `generate.Deps` has `Verbose *ui.Verbose`; `CommitStaged` passes it to Execute and calls
      `VerboseRetry` (1-based) at the parse-fail + duplicate retry sites.
- [ ] `pkg/stagecoach.runPipeline` mirrors the wiring (Execute vb arg + the 2 retry logs); `Options`
      gains additive `Verbose io.Writer`; `deps.Verbose` constructed from `opts.Verbose`+`cfg.Verbose`.
- [ ] `runDefault` passes `Verbose: stderr` (cmd.ErrOrStderr()) in `stagecoach.Options`.
- [ ] `executor_test.go`'s 9 `Execute(...)` calls append `nil` (compiler-driven); a NEW
      `TestExecute_Verbose` proves DEBUG command+raw-output land in an injected buffer.
- [ ] `go test -race ./...` green; `default_action_test.go` UNCHANGED and still passing (stdout exact);
      `go vet ./...` clean; `gofmt -l` empty for touched trees; only the listed files changed.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact
upstream seams (all quoted in F1–F8 + the doc comments referenced), the deterministic DEBUG formats
(external-research.md §1 table — copy verbatim), the §19 security rule (log argv, never env),
the copy-ready `verbose.go` skeleton (Implementation Blueprint), the exact edit sites (Integration
Points + Implementation Tasks list oldText→newText guidance), and the test conventions to mirror
(`root_test.go` buffer capture + `stubtest.NewScript` for duplicate-driven retries). No
signal/color/config/dry-run internals required (all explicitly out of scope — D9/F1).

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M4T3S2/research/design-decisions.md
  why: the 8 findings (F1–F8) + 9 decisions (D1–D9) SPECIFIC to this subtask. F1 (cfg.Verbose already
       resolved — consume only), F3 (Execute's exact call sites — compiler-driven), F4 (TWO code paths
       both need wiring), F5 (Deps is the injection point), F7 (the stdout-exact test invariant), F8
       (T2.S1 is complete — executor/generate ARE editable now).
  critical: F1, F3, F4, F7, F8 (the seams + the test lock); D1 (DEBUG prefix), D2 (new verbose.go),
       D3 (nil-safe), D5 (Execute gains vb param), D6 (never log Env), D8 (Options.Verbose io.Writer).

- docfile: plan/001_f1f80943ac34/P1M4T3S2/research/external-research.md
  why: §1 (the exact DEBUG line formats — copy verbatim into verbose.go), §2 (the §19 security
       boundary: argv only, never Env, stdin deferred), §3 (VERBOSE=2 is un-parseable + out of scope),
       §4 (library-vs-CLI stream discipline + the stdout-exact safety proof), §5 (the bytes.Buffer
       test pattern), §6 (no import cycle — ui is a leaf).
  critical: §1 (formats), §2 (security), §4 (why TestRunDefault_DryRun stays green).

- docfile: plan/001_f1f80943ac34/P1M4T3S1/PRP.md
  why: the SIBLING that creates `internal/ui/output.go` (the `↳` Progress/color layer). Its scope
       boundary lists verbose as "P1.M4.T3.S2" (owned by THIS task). Confirms `internal/ui` is
       stdlib-only (so provider/generate may import it) and that S1 does NOT touch
       executor.go/generate.go/stagecoach.go/default_action.go (no overlap with S2's edits).
  critical: do NOT edit `internal/ui/output.go` (S1 owns it); create `internal/ui/verbose.go` instead.

- file: internal/provider/executor.go   (P1.M2.T5.S1 — Execute; S2 EDITS the signature + adds 2 logs)
  section: `func Execute(ctx, spec, timeout) (stdout, stderr string, err error)` — the cmd build block
       (`cmd := exec.CommandContext(...)`; `var out, errb bytes.Buffer`; `cmd.Stdout=&out`;
       `setupProcessGroup(cmd)`), the `cmd.Start()` call (VerboseCommand goes RIGHT BEFORE it), and the
       `cmd.Wait()` block (VerboseRawOutput goes at the TOP of both the `werr != nil` branch and the
       success tail — log `out.String()` once per path).
  why: S2 adds (a) `vb *ui.Verbose` as the 4th param; (b) `vb.VerboseCommand(argv)` before Start;
       (c) `vb.VerboseRawOutput(out.String())` after Wait (success + error). argv joins Command+Args;
       NEVER Env (§19). All calls nil-safe (the methods guard nil).
  pattern: see Implementation Blueprint for the exact edits. out is already a bytes.Buffer → out.String().
  gotcha: log raw output on the ERROR path too (partial stdout aids rescue/diagnosis). On Start failure
       (returns before Wait) there is no captured stdout → log ONLY the command there (it's already
       logged before Start), no VerboseRawOutput. Do NOT log spec.Env.

- file: internal/provider/executor_test.go   (P1.M2.T5.S1 — 9 Execute calls; S2 appends `nil` + adds 1 test)
  section: every `Execute(context.Background(), spec, <dur>)` (and the one `Execute(ctx, spec, 0)`) —
       append a 4th arg `nil` (non-verbose). Add `TestExecute_Verbose` using `mustBin(t,"cat")` + a
       `bytes.Buffer` + `ui.NewVerbose(&buf, true)` → assert buf Contains "DEBUG: command: cat" AND
       "DEBUG: raw output:".
  why: the signature change is compiler-driven — `go test ./internal/provider/` lists every call to fix.
  gotcha: import `github.com/dustin/stagecoach/internal/ui` in the test (provider pkg test → ui pkg).
       The existing 9 tests pass `nil` so their behavior/assertions are unchanged.

- file: internal/generate/generate.go   (P1.M3.T4.S2 — CommitStaged + Deps; S2 EDITS)
  section: `type Deps struct { Git git.Git; Manifest provider.Manifest }` (add `Verbose *ui.Verbose`);
       the generation loop `for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++` — the
       `provider.Execute(ctx, *spec, cfg.Timeout)` call (add `deps.Verbose` arg) and the two `continue`
       sites: the parse-fail `if !ok { parseFail=true; candidate=m; continue }` and the duplicate
       `if IsDuplicate(subject, recent) { rejected=append(...); candidate=m; continue }` (add a
       `deps.Verbose.VerboseRetry(attempt+1, <reason>)` call IMMEDIATELY before each `continue`).
  why: S2 wires retry logging (1-based attempt) + passes the sink to Execute. Both retry sites must log
       (parse-fail + duplicate) — that IS "each retry attempt" (FR50).
  pattern: reasons — parse-fail: `"parse failed (no valid commit message)"`; duplicate:
       `fmt.Sprintf("subject %q matches an existing commit", subject)`. attempt+1 ⇒ 1-based (B.4 "Attempt 1").
  gotcha: do NOT log a retry for the SUCCESSFUL/final attempt (no VerboseRetry on the `break` path).
       Deps.Verbose nil-safe → existing generate_test.go Deps literals (no Verbose field) keep working.

- file: pkg/stagecoach/stagecoach.go   (P1.M3.T5.S1 — GenerateCommit + runPipeline; S2 EDITS)
  section: `type Options struct {...}` (add `Verbose io.Writer` — additive, stdlib type, doc'd
       additive-only); `GenerateCommit` after `buildDeps(...)` returns `deps` (set
       `deps.Verbose = ui.NewVerbose(opts.Verbose, cfg.Verbose)`); `runPipeline`'s dry-run
       `provider.Execute(ctx, *spec, cfg.Timeout)` (line ~257) and the loop's `provider.Execute(...)`
       (line ~295) (add `deps.Verbose` arg) + the 2 retry `continue` sites in the loop (add
       `deps.Verbose.VerboseRetry(attempt+1, <reason>)`, mirroring generate.go).
  why: the SECOND code path (DryRun/SystemExtra) must also be verbose-wired (F4) or `--dry-run -v`
       stays silent. Options.Verbose is io.Writer (no internal/ui leak into the public surface); the
       *ui.Verbose is constructed internally.
  pattern: import `github.com/dustin/stagecoach/internal/ui`. The retry reasons match generate.go exactly.
  gotcha: set deps.Verbose AFTER buildDeps (don't change buildDeps's signature — minimize churn).
       cfg.Verbose is the bool; opts.Verbose is the writer; nil writer ⇒ silent (library default).

- file: internal/cmd/default_action.go   (P1.M4.T1.S2 — runDefault; S2 EDITS 1 line)
  section: the `stagecoach.GenerateCommit(ctx, stagecoach.Options{Provider:..., Model:..., Timeout:...,
       DryRun: flagDryRun})` call — add `Verbose: stderr` (stderr is already `cmd.ErrOrStderr()` from
       the top of runDefault). This is the ONLY CLI change.
  why: opts in to verbose diagnostics on the CLI's stderr. cfg.Verbose (resolved by config) gates
       whether anything is actually written; the writer is where it goes.
  gotcha: do NOT touch printCommitReport/printDryRunMessage (stdout stays plain). do NOT touch the
       rescue/CAS paths in handleGenError (frozen). This is a ONE-LINE additive field in the Options literal.

- file: internal/config/config.go   (P1.M1.T4.S1 — Config.Verbose; READ, do NOT edit)
  section: `Verbose bool \`toml:"verbose"\`` (line 21) + comment "print resolved cmd, raw output,
       retries". Already resolved by all 7 layers (F1).
  why: THIS is the bool NewVerbose consumes. Do NOT change its type (bool) or add VERBOSE=2 parsing
       (D9 — would need int config, owned by P1.M1.T4).
  gotcha: ParseBool("2") errors → STAGECOACH_VERBOSE=2 currently fails config load; that's a KNOWN
       future-enhancement boundary, not a bug to fix here.

- file: internal/cmd/default_action_test.go   (P1.M4.T1.S2 — READ; the stream-contract LOCK)
  section: `TestRunDefault_DryRun` asserts `strings.TrimSpace(outBuf.String()) == "feat: dry run"`
       (stdout EXACT). Other tests assert Contains on stdout/stderr.
  why: PROVES verbose must write to the STDERR writer (errBuf), never stdout. Also: cfg.Verbose is
       false in every existing test (no -v) → verbose is a no-op there, so assertions are untouched.
  gotcha: do NOT edit this file. Optionally ADD a `TestRunDefault_Verbose` (sets rootCmd --verbose via
       `rootCmd.SetArgs([]string{"--verbose", ...})`) asserting errBuf Contains "DEBUG: command:".

- file: internal/cmd/root_test.go   (P1.M4.T1.S1 — READ; mirror test conventions)
  section: `bytes.Buffer` capture + `rootCmd.SetOut/SetErr` + `saveRootState`/`restoreRootState`/
       `resetFlags`. For a CLI verbose test, `rootCmd.SetArgs([]string{"--verbose","--dry-run"})`.
  why: the verbose_test.go (ui pkg) is simpler (pure unit, no cobra); but any CLI-level verbose test
       mirrors root_test.go's buffer-capture + SetArgs style.

- file: internal/stubtest/stubtest.go   (P1.M3.T4.S1 — READ; the duplicate-retry test driver)
  section: `stubtest.NewScript(t, bin, responses)` — call-varying mode: responses[0]=call1 stdout,
       responses[1]=call2, etc.; blank ⇒ ParseOutput ok=false. Used to drive a duplicate-then-valid
       sequence for the generate verbose-retry test.
  why: `TestCommitStaged_VerboseRetries` uses NewScript(["dup-subject","good-subject"]) + a prior
       commit with subject "dup-subject" so attempt 1 is a duplicate → VerboseRetry fires.
  gotcha: the stub is `stdin`-delivery (PromptDelivery "stdin") → its payload is in spec.Stdin (NOT
       logged at VERBOSE=1 — correct per §19); the logged argv is just the stub path.

- url: (PRD internal) PRD.md:318 (FR50), :946 (§15.2 flag table), :1203 (§19 security/VERBOSE=2)
  why: the three authoritative PRD anchors. FR50 = the feature; §15.2 = the flag/env contract;
       §19 = the "command and flags but never stdin unless VERBOSE=2" security rule.
  critical: §19 — log argv (command+flags), never Env, never stdin (until the future VERBOSE=2).
```

### Current Codebase tree (relevant slice)

```bash
go.mod                              # module github.com/dustin/stagecoach ; go 1.22 ; UNCHANGED (no new deps)
internal/
  ui/                               # created by P1.M4.T3.S1 (output.go) — S2 ADDS verbose.go here (sibling)
  provider/executor.go              # P1.M2.T5.S1 — Execute (S2 EDITS: +vb param, +2 logs)
  provider/executor_test.go         # P1.M2.T5.S1 — 9 Execute calls (S2: append nil + add TestExecute_Verbose)
  generate/generate.go              # P1.M3.T4.S2 — CommitStaged + Deps (S2 EDITS: +Verbose field, +Execute arg, +2 retry logs)
  generate/generate_test.go         # P1.M3.T4.S2 — stub-driven tests (S2: add TestCommitStaged_VerboseRetries; existing untouched)
  config/config.go                  # P1.M1.T4.S1 — Config.Verbose (READ; already resolved)
  cmd/default_action.go             # P1.M4.T1.S2 — runDefault (S2 EDITS 1 line: +Verbose: stderr)
  cmd/default_action_test.go        # P1.M4.T1.S2 — stream-contract assertions (READ; do NOT edit)
  cmd/root_test.go                  # P1.M4.T1.S1 — test conventions (READ; mirror)
  stubtest/stubtest.go              # P1.M3.T4.S1 — NewScript duplicate driver (READ; use)
pkg/stagecoach/stagecoach.go          # P1.M3.T5.S1 — GenerateCommit + runPipeline (S2 EDITS: +Options.Verbose, +deps.Verbose, +Execute arg, +2 retry logs)
Makefile                            # build/test(-race)/vet/coverage/lint/clean (UNCHANGED)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/ui/verbose.go              # NEW — package ui: type Verbose, NewVerbose, VerboseCommand/VerboseRawOutput/VerboseRetry (nil-safe, DEBUG: prefix, stdlib-only).
internal/ui/verbose_test.go         # NEW — on/off/nil matrix + exact byte assertions.
internal/provider/executor.go       # EDIT — Execute gains vb *ui.Verbose; log argv (pre-Start) + raw stdout (post-Wait, both paths).
internal/provider/executor_test.go  # EDIT — append nil to 9 Execute calls; add TestExecute_Verbose.
internal/generate/generate.go       # EDIT — Deps.Verbose; pass to Execute; VerboseRetry at 2 retry sites.
pkg/stagecoach/stagecoach.go          # EDIT — Options.Verbose io.Writer; deps.Verbose=NewVerbose(...); wire runPipeline (Execute arg + 2 retry logs).
internal/cmd/default_action.go      # EDIT — Options{..., Verbose: stderr} (1 line).
# All other files UNCHANGED. config/*.go, internal/ui/output.go, generate/rescue.go, exitcode/*, signal/* UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (§19 SECURITY — never log Env): spec.Env = os.Environ() + manifest env, which routinely
// carries ANTHROPIC_API_KEY / OPENAI_API_KEY / bearer tokens. VerboseCommand logs
// strings.Join(append([]string{spec.Command}, spec.Args...), " ") — ARGV ONLY. NEVER fmt.Sprint(spec.Env),
// NEVER loop over spec.Env, NEVER include env in the command display. This is PRD §19 line 1203.

// CRITICAL (stdout MUST stay plain — FR50/§15.2 + default_action_test.go:272): Verbose writes ONLY to
// the *ui.Verbose.w writer (the CLI's stderr). NEVER write DEBUG lines to a stdout writer. In every
// existing test cfg.Verbose==false → Verbose is a no-op, so the stdout-exact assertions are safe; but
// the design must make stdout leakage structurally impossible (Verbose has no stdout field at all).

// CRITICAL (TWO code paths — F4): BOTH generate.CommitStaged AND pkg/stagecoach.runPipeline contain a
// Render→Execute→dedupe loop. Wire verbose into BOTH (Execute vb arg + the 2 retry logs each) or
// `stagecoach -v --dry-run` stays silent (runPipeline is the DryRun path). Forgetting runPipeline is the
// #1 completeness bug.

// GOTCHA (nil-safety is the threading mechanism): every *ui.Verbose method starts with
// `if v == nil || v.w == nil || !v.on { return }`. Callers pass deps.Verbose (may be nil) and call
// methods UNCONDITIONALLY — no `if deps.Verbose != nil { ... }` guards in the pipeline. Existing Deps
// literals without a Verbose field (generate_test, stagecoach_test) keep working (nil → no-op).

// GOTCHA (Execute signature change is compiler-driven — F3): Execute gains `vb *ui.Verbose` as the 4th
// param. `go build ./... && go test ./internal/provider/` will list EVERY call site. Prod calls pass
// deps.Verbose; the 9 executor_test calls append nil. You cannot miss one (the compiler won't let you).

// GOTCHA (DEBUG prefix, NOT ↳): verbose lines use "DEBUG: " (the commit-pi convention from the
// contract). The "↳" (U+21B3) prefix is S1's always-on Progress layer — do NOT reuse it for verbose
// (it would imply always-shown). Distinct prefixes keep the streams separable. (Appendix B.4's "↳ Attempt"
// is illustrative; the contract's DEBUG: is authoritative — D1.)

// GOTCHA (1-based attempt numbering): the loop var `attempt` is 0-based; VerboseRetry(attempt+1, reason)
// makes it 1-based to match Appendix B.4 "Attempt 1". Log ONLY at the 2 failure `continue` sites
// (parse-fail, duplicate) — NOT on the successful `break` (the success is the normal flow / FR42 report).

// GOTCHA (log raw output on the ERROR path too): Execute returns captured stdout EVEN on error (partial
// output). VerboseRawOutput(out.String()) should fire after cmd.Wait() in BOTH the werr!=nil branch and
// the success tail — verbose exists to diagnose failures, so seeing partial output on a non-zero exit /
// timeout is the point. On Start failure (pre-Wait) there's no stdout → log only the command.

// GOTCHA (do NOT implement VERBOSE=2 / stdin — D9): Config.Verbose is a bool; ParseBool("2") errors.
// Supporting VERBOSE=2 needs Config.Verbose → int (a cross-cutting config change owned by P1.M1.T4).
// S2 implements VERBOSE=1 only. VerboseRawOutput logs STDOUT (FR50 "raw agent stdout"); stdin is deferred.
// Add a code comment noting the future VERBOSE=2 hook so the next contributor finds it.

// GOTCHA (do NOT edit internal/ui/output.go): S1 (P1.M4.T3.S1) owns it. Create internal/ui/verbose.go
// as a SIBLING file in package ui. Both files contribute to package ui with zero merge conflict.

// GOTCHA (public API stays additive + stdlib-typed): Options.Verbose is `io.Writer` (stdlib), NOT
// `*ui.Verbose` — internal/ui must NOT leak into the public pkg/stagecoach surface. The *ui.Verbose is
// constructed INSIDE GenerateCommit. nil Options.Verbose ⇒ silent (library default; a library has no
// business writing to os.Stderr — D8).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/ui/verbose.go
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Verbose is Stagecoach's --verbose diagnostics sink (PRD §9.13 FR50, §15.2, §19). When ON, it prints
// the resolved provider command, the raw agent stdout, and each retry attempt to a writer (the CLI's
// stderr) with a "DEBUG: " prefix (the commit-pi convention named in the work-item contract). When OFF
// (the default), or when the receiver is nil, or when the writer is nil, EVERY method is a no-op
// (zero bytes, zero allocations) — so callers thread a *Verbose (possibly nil) and call methods
// unconditionally with no nil guards.
//
// SECURITY (PRD §19): VerboseCommand logs ARGV ONLY (Command+Args). It NEVER logs spec.Env (which
// carries *_API_KEY credentials). Stdin contents are NOT logged at VERBOSE=1 (deferred to a future
// VERBOSE=2 — see D9; Config.Verbose is a bool, so VERBOSE=2 is currently un-parseable and out of scope).
//
// The writer is INJECTABLE: the CLI passes cmd.ErrOrStderr() (stderr); a library consumer of
// pkg/stagecoach passes its own writer or nil. This keeps the library side-effect-free by default
// (it never writes to os.Stderr directly). Sibling to output.go (P1.M4.T3.S1's ↳/color layer); this
// file owns ONLY verbose diagnostics.
type Verbose struct {
	w  io.Writer // destination (stderr in prod, *bytes.Buffer in tests); nil ⇒ no-op
	on bool      // cfg.Verbose — resolved by config.Load across all 7 layers
}

// NewVerbose constructs a Verbose sink. on=false (the common case) ⇒ every method is a no-op. w may be
// nil ⇒ every method is a no-op (the library default: a caller that supplies no writer gets silence
// even if cfg.Verbose is true). From the CLI: ui.NewVerbose(stderr, cfg.Verbose). From tests:
// ui.NewVerbose(&buf, true|false).
func NewVerbose(w io.Writer, on bool) *Verbose {
	return &Verbose{w: w, on: on}
}

// VerboseCommand prints the resolved provider command (PRD §9.13 FR50). cmd is the rendered argv
// (Command + Args, space-joined) — the CALLER builds it from CmdSpec; this method never touches Env.
// Format: "DEBUG: command: <cmd>\n". No-op when v==nil, v.w==nil, or !v.on.
func (v *Verbose) VerboseCommand(cmd string) {
	if v == nil || v.w == nil || !v.on {
		return
	}
	fmt.Fprintln(v.w, "DEBUG: command: "+cmd)
}

// VerboseRawOutput prints the raw agent stdout (PRD §9.13 FR50 — "the raw agent stdout"), pre-parse
// and pre-fence-strip, so a user can see exactly what the model returned. Format: "DEBUG: raw output:\n"
// followed by the output verbatim, with a trailing newline ensured (so the next DEBUG line is clean).
// No-op when v==nil, v.w==nil, or !v.on.
func (v *Verbose) VerboseRawOutput(output string) {
	if v == nil || v.w == nil || !v.on {
		return
	}
	fmt.Fprint(v.w, "DEBUG: raw output:\n")
	fmt.Fprint(v.w, output)
	if !strings.HasSuffix(output, "\n") {
		fmt.Fprint(v.w, "\n")
	}
}

// VerboseRetry prints a retry attempt and its reason (PRD §9.13 FR50 — "each retry attempt"). attempt
// is 1-based (matches Appendix B.4 "Attempt 1"). Format: "DEBUG: attempt <n>: <reason>\n". No-op when
// v==nil, v.w==nil, or !v.on.
func (v *Verbose) VerboseRetry(attempt int, reason string) {
	if v == nil || v.w == nil || !v.on {
		return
	}
	fmt.Fprintf(v.w, "DEBUG: attempt %d: %s\n", attempt, reason)
}

// (ResolveColor/IsTerminal/UI/New/Progress/... live in output.go — P1.M4.T3.S1. This file adds ONLY
//  the Verbose type. Do not duplicate them.)
var _ = os.Stdout // keep "os" import meaningful if a future helper defaults to os.Stderr; remove the
                  // blank-var line if "os" ends up unused (preferably drop the "os" import entirely if
                  // unused — see Task 1 note).
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/ui/verbose.go (package ui, stdlib-only, nil-safe)
  - FILE: NEW internal/ui/verbose.go. PACKAGE: `package ui`. Copy the "Data models" skeleton.
  - IMPORTS (stdlib ONLY): fmt, io, strings. (Drop "os" if unused — the blank-var line in the skeleton
      is a placeholder; a clean implementation that never references os.Stdout needs only fmt/io/strings.
      If you keep no os reference, OMIT the os import and the `var _ = os.Stdout` line.)
  - DEFINE: type Verbose{w io.Writer; on bool}; NewVerbose(w, on); VerboseCommand/VerboseRawOutput/
      VerboseRetry — each guarded by `if v == nil || v.w == nil || !v.on { return }`.
  - FORMATS (copy EXACTLY — tests assert byte-for-byte):
      VerboseCommand  → fmt.Fprintln(v.w, "DEBUG: command: "+cmd)
      VerboseRawOutput → fmt.Fprint(v.w, "DEBUG: raw output:\n"); fmt.Fprint(v.w, output);
                         if !strings.HasSuffix(output,"\n") { fmt.Fprint(v.w,"\n") }
      VerboseRetry    → fmt.Fprintf(v.w, "DEBUG: attempt %d: %s\n", attempt, reason)
  - GOTCHA: nil-safety on the RECEIVER (v==nil) is mandatory — callers pass deps.Verbose which may be nil.
  - GOTCHA: do NOT edit output.go; this is a sibling file. Do NOT redefine UI/Progress/etc.

Task 2: CREATE internal/ui/verbose_test.go (pure unit tests; bytes.Buffer capture)
  - FILE: NEW internal/ui/verbose_test.go. PACKAGE: `package ui` (internal — can construct Verbose
      directly). Mirror root_test.go's bytes.Buffer style (but no cobra needed).
  - TESTS:
      • TestVerbose_CommandWhenOn: NewVerbose(&buf,true); VerboseCommand("pi --model x"); buf=="DEBUG: command: pi --model x\n".
      • TestVerbose_RawOutputWhenOn: NewVerbose(&buf,true); VerboseRawOutput("feat: x\n"); buf=="DEBUG: raw output:\nfeat: x\n".
        AND a no-trailing-newline variant: VerboseRawOutput("feat: x"); buf=="DEBUG: raw output:\nfeat: x\n" (trailing \n added).
      • TestVerbose_RetryWhenOn: NewVerbose(&buf,true); VerboseRetry(1,"subject \"x\" matches"); buf=="DEBUG: attempt 1: subject \"x\" matches\n".
      • TestVerbose_NoOpWhenOff: NewVerbose(&buf,false); call all 3; buf.Len()==0.
      • TestVerbose_NilSafeReceiver: var v *Verbose = nil; v.VerboseCommand("x"); v.VerboseRawOutput("y"); v.VerboseRetry(1,"z"); // no panic, nothing written.
      • TestVerbose_NilWriterNoOp: NewVerbose(nil,true); call all 3; // no panic, nothing written (library default).
      • TestVerbose_MultipleLinesAccumulate: call Command then RawOutput; assert both substrings present in order.
  - COVERAGE: every method × {on, off, nil-receiver, nil-writer}. Assert EXACT bytes for the on cases.

Task 3: EDIT internal/provider/executor.go (Execute gains vb param + 2 logs)
  - FILE: EDIT internal/provider/executor.go. ADD import "github.com/dustin/stagecoach/internal/ui"
      (strings is already imported).
  - CHANGE A (signature): `func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration) (...)`
      → `func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (...)`.
      Update the doc comment to note vb is a nil-safe verbose sink (logs argv pre-Start, raw stdout
      post-Wait); nil ⇒ no diagnostics.
  - CHANGE B (VerboseCommand before Start): right BEFORE `if err := cmd.Start(); err != nil {`, add:
        vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), " "))
      (argv ONLY — NEVER spec.Env. vb is nil-safe so no guard needed.)
  - CHANGE C (VerboseRawOutput after Wait — BOTH paths): at the TOP of the `if werr := cmd.Wait();
      werr != nil {` body (before the ctx.Err() check) add `vb.VerboseRawOutput(out.String())`; AND in
      the success tail (right before `return out.String(), errb.String(), nil`) add
      `vb.VerboseRawOutput(out.String())`. (Log partial stdout on error too — verbose diagnoses failures.)
      Do NOT add VerboseRawOutput on the cmd.Start() failure path (no captured stdout there).
  - GOTCHA: out is already a *bytes.Buffer → out.String(). errb (stderr) is NOT logged (FR50 = stdout).
  - GOTCHA: re-run executor_test.go — the compiler flags all 9 Execute calls; fix in Task 6.

Task 4: EDIT internal/generate/generate.go (Deps.Verbose + Execute arg + 2 retry logs)
  - FILE: EDIT internal/generate/generate.go. ADD import "github.com/dustin/stagecoach/internal/ui".
  - CHANGE A (Deps field): add `Verbose *ui.Verbose // nil-safe --verbose diagnostics sink (P1.M4.T3.S2);
      logs retries here + passed to provider.Execute for command/raw-output logging` to the Deps struct.
  - CHANGE B (Execute arg): `out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout)` → append
      `deps.Verbose` → `provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)`.
  - CHANGE C (retry log — parse-fail): in the loop, the `m, ok, _ := provider.ParseOutput(...)` block's
      `if !ok {` body, BEFORE `parseFail = true; candidate = m; continue`, add:
        deps.Verbose.VerboseRetry(attempt+1, "parse failed (no valid commit message)")
  - CHANGE D (retry log — duplicate): in the `if IsDuplicate(subject, recent) {` body, BEFORE
      `rejected = append(rejected, subject); candidate = m; continue`, add:
        deps.Verbose.VerboseRetry(attempt+1, fmt.Sprintf("subject %q matches an existing commit", subject))
  - GOTCHA: do NOT add VerboseRetry on the success `break` path. attempt+1 ⇒ 1-based.
  - GOTCHA: Deps.Verbose nil-safe → existing generate_test.go Deps literals (no Verbose) keep working.

Task 5: EDIT pkg/stagecoach/stagecoach.go (Options.Verbose + deps.Verbose + runPipeline wiring)
  - FILE: EDIT pkg/stagecoach/stagecoach.go. ADD import "github.com/dustin/stagecoach/internal/ui".
  - CHANGE A (Options field): add `Verbose io.Writer // optional; when set AND cfg.Verbose, diagnostics
      (resolved command, raw output, retries) are written here (the CLI passes stderr). nil ⇒ silent.
      Additive-only (PRD §14.1).` to the Options struct (after Timeout).
  - CHANGE B (construct sink): in GenerateCommit, AFTER `deps, err := buildDeps(cfg, repoDir)` succeeds,
      add: `deps.Verbose = ui.NewVerbose(opts.Verbose, cfg.Verbose)`. (BEFORE the CommitStaged/runPipeline
      calls so both paths get it.) Do NOT change buildDeps's signature.
  - CHANGE C (runPipeline Execute args): the dry-run `out, _, execErr := provider.Execute(ctx, *spec,
      cfg.Timeout)` (~line 257) and the loop's `provider.Execute(ctx, *spec, cfg.Timeout)` (~line 295) →
      append `deps.Verbose` to both.
  - CHANGE D (runPipeline retry logs): in runPipeline's loop, mirror generate.go's two retry sites —
      parse-fail `if !ok {` body: `deps.Verbose.VerboseRetry(attempt+1, "parse failed (no valid commit
      message)")` before continue; duplicate `if generate.IsDuplicate(...)` body:
      `deps.Verbose.VerboseRetry(attempt+1, fmt.Sprintf("subject %q matches an existing commit", subject))`
      before continue. (runPipeline is the DryRun/SystemExtra path — F4: it MUST be wired too.)
  - GOTCHA: Options.Verbose is io.Writer (stdlib) — do NOT expose *ui.Verbose publicly.
  - GOTCHA: nil opts.Verbose + cfg.Verbose=true ⇒ NewVerbose(nil,true) ⇒ silent (library default — D8).

Task 6: EDIT internal/provider/executor_test.go (append nil to 9 calls + add TestExecute_Verbose)
  - FILE: EDIT internal/provider/executor_test.go. ADD import "github.com/dustin/stagecoach/internal/ui"
      + "bytes" (bytes likely already imported; check).
  - CHANGE A: every `Execute(<ctx>, <spec>, <dur>)` call → append `, nil` (4th arg). There are 9 (the
      compiler lists them on `go test`). Non-verbose existing tests pass nil → behavior unchanged.
  - CHANGE B (NEW test): add
        func TestExecute_Verbose(t *testing.T) {
            mustBin(t, "cat")
            var buf bytes.Buffer
            vb := ui.NewVerbose(&buf, true)
            spec := CmdSpec{Command: "cat", Stdin: "feat: hello\n", Env: os.Environ()}
            out, _, err := Execute(context.Background(), spec, 5*time.Second, vb)
            // assertions: err==nil; out=="feat: hello\n"; buf contains "DEBUG: command: cat" AND
            // "DEBUG: raw output:\nfeat: hello\n"; AND buf must NOT contain any env var (e.g. "PATH=")
            // proving Env is never logged.
        }
  - GOTCHA: assert the env-leak guard (`!strings.Contains(buf.String(), "PATH")` or similar) — this is
      the §19 security regression test.

Task 7: EDIT internal/cmd/default_action.go (Options.Verbose: stderr — 1 line)
  - FILE: EDIT internal/cmd/default_action.go. In the `stagecoach.GenerateCommit(ctx, stagecoach.Options{...})`
      literal, add `Verbose: stderr,` (stderr = cmd.ErrOrStderr(), already bound at the top of runDefault).
  - PRESERVE: everything else. printCommitReport/printDryRunMessage (stdout plain), handleGenError
      rescue/CAS (frozen), the auto-stage state machine — all UNCHANGED.
  - GOTCHA: this is the ONLY CLI change. cfg.Verbose (from config) gates actual output; stderr is where.

Task 8 (optional but recommended): ADD generate retry-verbose + CLI verbose tests
  - internal/generate/generate_test.go — add TestCommitStaged_VerboseRetries: initRepo + commit an
      empty commit "feat: dup" (so it's recent); stage a change; deps := Deps{Git: git.New(dir),
      Manifest: stubtest.NewScript(t, stubtest.Build(t), []string{"feat: dup","feat: good"}),
      Verbose: ui.NewVerbose(&buf, true)}; cfg.MaxDuplicateRetries=1; CommitStaged → assert err==nil AND
      buf contains `DEBUG: attempt 1: subject "feat: dup" matches an existing commit` AND a second
      `DEBUG: raw output:` for attempt 2. (Mirror an existing duplicate test in generate_test.go.)
  - internal/cmd/default_action_test.go (or root_test.go) — OPTIONAL TestRunDefault_Verbose: set up a
      stub provider + `rootCmd.SetArgs([]string{"--verbose","--dry-run"})` + SetErr(&errBuf); assert
      errBuf Contains "DEBUG: command:" and stdout still == the bare message (verbose→stderr).
```

### Implementation Patterns & Key Details

```go
// The nil-safe sink — the heart of this task. Every method no-ops on nil receiver / nil writer / off:
func (v *Verbose) VerboseCommand(cmd string) {
	if v == nil || v.w == nil || !v.on {
		return
	}
	fmt.Fprintln(v.w, "DEBUG: command: "+cmd)
}

// executor wiring (Task 3). argv ONLY — NEVER Env (§19):
vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), " "))
// ... cmd.Start() ... cmd.Wait() ...
vb.VerboseRawOutput(out.String()) // log captured stdout on BOTH success and error paths

// orchestrator retry wiring (Task 4 + 5). 1-based, at the FAILURE site, before `continue`:
deps.Verbose.VerboseRetry(attempt+1, fmt.Sprintf("subject %q matches an existing commit", subject))

// public API sink construction (Task 5). on = cfg.Verbose (single source of truth); w = opts.Verbose:
deps.Verbose = ui.NewVerbose(opts.Verbose, cfg.Verbose)

// CLI opt-in (Task 7). One field:
stagecoach.GenerateCommit(ctx, stagecoach.Options{Provider: cfg.Provider, Model: cfg.Model,
	Timeout: cfg.Timeout, DryRun: flagDryRun, Verbose: stderr})
```

### Integration Points

```yaml
PACKAGE LAYOUT (PRD §14):
  - create: internal/ui/verbose.go   # sibling to output.go (S1); package ui owns all CLI rendering.

CONFIG (consumed, NOT modified):
  - field: config.Config.Verbose (toml:"verbose") — already resolved by config.Load (all 7 layers).
    NewVerbose reads it as the `on` bool. Do NOT change the type (bool) or add VERBOSE=2 parsing.

CLI (wired):
  - file: internal/cmd/default_action.go
  - point: the stagecoach.Options literal in runDefault → add `Verbose: stderr`.

PUBLIC API (additive):
  - field: pkg/stagecoach.Options.Verbose io.Writer — additive-only (PRD §14.1); stdlib type (no
    internal/ui leak). nil ⇒ silent (library default).

EXECUTOR (signature change — compiler-driven):
  - func: provider.Execute(ctx, spec, timeout, vb *ui.Verbose) — vb nil-safe; logs argv + raw stdout.

ORCHESTRATOR (Deps field + retry logs):
  - struct: generate.Deps gains `Verbose *ui.Verbose`.
  - loops: generate.CommitStaged AND pkg/stagecoach.runPipeline both pass deps.Verbose to Execute and
    log VerboseRetry at the parse-fail + duplicate continue sites.

SECURITY (PRD §19 — load-bearing):
  - NEVER log spec.Env (carries *_API_KEY). VerboseCommand logs Command+Args only.
  - NEVER log stdin at VERBOSE=1 (deferred to future VERBOSE=2; Config.Verbose is bool so VERBOSE=2 is
    currently un-parseable — out of scope).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After creating verbose.go — fix before proceeding.
gofmt -w internal/ui/verbose.go internal/ui/verbose_test.go
go vet ./internal/ui/

# After editing executor.go / generate.go / stagecoach.go / default_action.go / executor_test.go
gofmt -w internal/provider/ internal/generate/ pkg/stagecoach/ internal/cmd/
go vet ./internal/provider/ ./internal/generate/ ./pkg/stagecoach/ ./internal/cmd/

# Expected: zero errors. gofmt -l should be empty for all touched trees.
gofmt -l internal/ pkg/ cmd/
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test the ui.Verbose sink in isolation (the core deliverable).
go test -race ./internal/ui/ -v
# Expected: all green — on/off/nil matrix, exact byte formats, nil-receiver + nil-writer safety.

# Test the executor verbose wiring (Task 3 + 6).
go test -race ./internal/provider/ -v
# Expected: the 9 existing Execute tests pass (nil arg, unchanged behavior); TestExecute_Verbose proves
# DEBUG command+raw-output land in the buffer AND no env var leaks (§19 guard).

# Test the orchestrator retry verbose wiring (Task 4 + 8).
go test -race ./internal/generate/ -v
# Expected: existing tests pass (Deps.Verbose nil → no-op); TestCommitStaged_VerboseRetries proves
# DEBUG attempt 1 duplicate line + a 2nd raw output on the retry.

# Full suite — proves no regression (verbose off in every existing test + writes to stderr).
go test -race ./...
# Expected: all green. default_action_test.go UNCHANGED: TestRunDefault_DryRun still asserts
# stdout == "feat: dry run" (verbose is off there AND writes to stderr, never stdout).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the real binary.
make build

# Smoke test in a scratch repo with the stub provider (or any installed agent) + --verbose --dry-run.
cd /tmp && rm -rf vb-smoke && git init vb-smoke && cd vb-smoke &&
  git config user.email t@t.co && git config user.name t &&
  echo hi > a.txt && git add a.txt &&
  STAGECOACH_PROVIDER=stub ./path/to/bin/stagecoach -v --dry-run 2>debug.log 1>out.txt ||
  ./path/to/bin/stagecoach -v --dry-run 2>debug.log 1>out.txt

# Assertions:
#   - out.txt == the generated message ONLY (no "DEBUG:", no "↳").           [FR50 / §15.5 pipe]
#   - debug.log contains "DEBUG: command: " (the resolved argv).             [FR50]
#   - debug.log contains "DEBUG: raw output:" (the raw agent stdout).        [FR50]
#   - debug.log must NOT contain "PATH" / "API_KEY" / "=" env lines (§19).   [SECURITY]
#   - `cat -v debug.log` shows NO ANSI escapes (verbose is plain text).      [D1]

# Toggle sanity:
#   stagecoach --dry-run            # NO debug output (verbose off), message on stdout
#   stagecoach -v --dry-run         # DEBUG lines on STDERR, message still clean on stdout
#   STAGECOACH_VERBOSE=1 stagecoach --dry-run   # same as -v (env-driven)
#   stagecoach -v --dry-run 2>/dev/null | tee /tmp/msg.txt   # msg.txt is clean (stdout only)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# §19 secret-leak regression (the critical security check): force a provider whose env would carry a
# fake secret, run -v, and assert the secret NEVER appears on stderr.
cd /tmp/vb-smoke &&
  SECRET_KEY=d0ntleakme ./path/to/bin/stagecoach -v --dry-run 2>debug.log 1>/dev/null || true
grep -q "d0ntleakme" debug.log && echo "FAIL: secret leaked to stderr (§19 violation)" || echo "PASS: no secret leak"

# Pipe-safety proof (FR51/§15.5 — verbose must not corrupt stdout):
./path/to/bin/stagecoach -v --dry-run 2>/dev/null | tee /tmp/msg.txt
grep -P '\x1b|DEBUG' /tmp/msg.txt && echo "FAIL: stdout polluted" || echo "PASS: stdout is the bare message"

# Duplicate-retry verbose proof (Appendix B.4): set up a repo whose recent subject will collide, run -v,
# and confirm "DEBUG: attempt 1: subject ... matches an existing commit" on stderr followed by a 2nd
# "DEBUG: raw output:" for the retry. (Use the stub script or a real agent prompted to duplicate.)

# Race + full regression (the gate):
go test -race ./...
go vet ./...
gofmt -l internal/ pkg/ cmd/

# Expected: all green; only the 7 listed files changed (git status). config/*, internal/ui/output.go,
# generate/rescue.go, exitcode/*, signal/* UNCHANGED.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go test -race ./internal/ui/ -v` green (the core sink deliverable).
- [ ] `go test -race ./...` green — NO regression; `default_action_test.go` UNCHANGED and still passing
      (TestRunDefault_DryRun stdout == "feat: dry run").
- [ ] `go vet ./...` clean; `gofmt -l internal/ pkg/ cmd/` empty.
- [ ] No new `go.mod` dependencies (stdlib-only — `internal/ui` imports fmt/io/strings only).

### Feature Validation

- [ ] `VerboseCommand` writes `"DEBUG: command: <argv>\n"`; `VerboseRawOutput` writes
      `"DEBUG: raw output:\n<out>"` (+ trailing `\n`); `VerboseRetry` writes
      `"DEBUG: attempt <n>: <reason>\n"` — all ONLY when on, all nil-safe (nil receiver / nil writer / off).
- [ ] `provider.Execute` logs argv (pre-Start) + raw stdout (post-Wait, success AND error); NEVER Env.
- [ ] `generate.CommitStaged` AND `pkg/stagecoach.runPipeline` both pass `deps.Verbose` to Execute and log
      `VerboseRetry` (1-based) at the parse-fail + duplicate retry sites.
- [ ] `Options.Verbose io.Writer` (additive); CLI passes stderr; library nil ⇒ silent.
- [ ] `stagecoach -v --dry-run` prints DEBUG command + raw output to STDERR; stdout stays the bare message.
- [ ] §19: no env var / secret / stdin contents appear in verbose output (the security regression test passes).

### Code Quality Validation

- [ ] `internal/ui/verbose.go` imports only stdlib (no stagecoach package; no import-cycle risk).
- [ ] Follows existing patterns: injectable-writer (S1's UI), DI struct (generate.Deps), bytes.Buffer tests.
- [ ] File placement matches the desired tree; `output.go` (S1) UNCHANGED.
- [ ] Anti-patterns avoided (see below): no stdout verbose, no Env logging, no os.Stderr in library, no
      VERBOSE=2, no edit to S1's output.go or frozen rescue.

### Documentation & Deployment

- [ ] Code is self-documenting (doc comments on Verbose + each method; §19 security rationale inlined;
      the future VERBOSE=2 hook noted in a comment).
- [ ] No new env vars beyond the existing `STAGECOACH_VERBOSE` (already in §15.2 help).

---

## Anti-Patterns to Avoid

- ❌ Don't log `spec.Env` — it carries `*_API_KEY` credentials. VerboseCommand logs argv (Command+Args)
  ONLY. This is PRD §19 (line 1203) and is the #1 security rule (D6).
- ❌ Don't write verbose output to stdout — it breaks `stagecoach --dry-run | tee` / `git commit -F <(...)`
  (§15.5) AND `default_action_test.go:272`'s exact `stdout == "feat: dry run"` equality. Verbose → the
  stderr writer ONLY (F7).
- ❌ Don't write `os.Stderr` directly from `pkg/stagecoach` — it's a library; thread an `io.Writer`
  instead (nil ⇒ silent). The CLI opts in by passing its stderr (D8).
- ❌ Don't edit `internal/ui/output.go` (P1.M4.T3.S1 owns it) — create `internal/ui/verbose.go` as a
  sibling (D2). Editing output.py risks a merge conflict with the parallel S1 work.
- ❌ Don't wire only `generate.CommitStaged` and forget `pkg/stagecoach.runPipeline` — runPipeline is the
  DryRun/SystemExtra path; leaving it silent breaks `stagecoach -v --dry-run` (F4).
- ❌ Don't implement VERBOSE=2 / stdin logging — `Config.Verbose` is a `bool` (`ParseBool("2")` errors);
  supporting it needs a cross-cutting int-config change owned by P1.M1.T4. S2 = VERBOSE=1 only (D9).
- ❌ Don't reuse the `↳` (U+21B3) prefix for verbose — that's S1's always-on Progress layer. Verbose uses
  `DEBUG: ` (the commit-pi convention; D1) so the two streams stay visually distinct.
- ❌ Don't scatter `if verbose != nil { ... }` guards — make the `*Verbose` methods nil-safe (nil
  receiver / nil writer / off ⇒ no-op) and call them unconditionally (D3).
- ❌ Don't log a retry on the SUCCESSFUL/final attempt — `VerboseRetry` fires ONLY at the parse-fail +
  duplicate `continue` sites (1-based `attempt+1`); the success is the normal FR42 flow.
- ❌ Don't change `buildDeps`'s signature to thread verbose — set `deps.Verbose` AFTER `buildDeps`
  returns (minimize churn; the sink is a runtime collaborator, not a construction-time dep).
- ❌ Don't colorize verbose lines — verbose is plain `DEBUG:` text; color is S1's separate concern.
