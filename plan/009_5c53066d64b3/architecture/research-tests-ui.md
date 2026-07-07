# Code Context — Test/Integration Patterns + Verbose/Progress UI for Multi-Turn Generation (PRD §9.24, FR-T4/T5/T11)

Scout scope: extend the existing `internal/stubtest` stub-agent pattern for multi-turn fallback tests. All paths below are EXACT file paths + line numbers. No files were modified.

---

## Files Retrieved

1. `internal/stubtest/stubtest.go` (lines 1-148) — the reusable stub-agent helpers: `Build`, `Options`, `Manifest`, `NewScript`, `Env`, `optsEnvMap`. This is the seam to extend.
2. `internal/stubtest/stubtest_test.go` (lines 1-180) — exercises every stub knob including call-varying (`TestStub_ScriptCallVarying`, `TestStub_ScriptBlankIsParseFailure`).
3. `cmd/stubagent/main.go` (lines 1-145) — the tiny fake-agent binary itself. STDLIB ONLY. Drains stdin, writes marker/argsfile, sleeps, emits stderr, then writes stdout (single-response `OUT` or script-indexed line), then `os.Exit`.
4. `internal/provider/manifest.go` (lines 1-235) — `Manifest` struct + `Render`/`Resolve`/`Validate`/`DetectCommand`. `Env map[string]string` is the test-only knob injection point.
5. `internal/provider/render.go` (lines 1-200) — `Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error)` + `CmdSpec{Command, Args, Stdin, Env}` + `RenderMode{RenderBare, RenderTooled}`. THIS is where a multi-turn variant (drop `--no-session`, add `--session-id`) lives.
6. `internal/provider/render_test.go` (lines 1-450) — golden/table-driven render unit tests. `TestRender_GoldenPerProvider` + `TestRender_Pi_ByteForByteCommitPi` are the templates.
7. `internal/provider/builtin.go` (lines 46-100) — `builtinPi()`. `BareFlags` contains `"--no-session"` (line 60); `TooledFlags` also has `"--no-session"` (line 92). The multi-turn render variant must drop `--no-session` and add a `--session-id` token.
8. `internal/generate/generate.go` (lines 1-330) — `CommitStaged` pipeline + `Deps{Git, Manifest, Verbose, Excludes, Progress}` + `Result{CommitSHA, Subject, Message, Provider, Model, Changes}`. The generate→parse→dedupe loop is at lines 150-205 (the bounded `for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++`). This is the loop to extend for multi-turn fallback.
9. `internal/generate/generate_test.go` (lines 1-400) — the in-process `CommitStaged` integration tests. `TestCommitStaged_Success`, `TestCommitStaged_DedupeRetryThenSuccess`, `TestCommitStaged_ParseFailRescue` are the templates. Has its own `initRepo`/`writeFile`/`stageFile`/`headSHA`/`commitRaw`/`gitOut`/`runGit`/`shaRe` fixture helpers.
10. `internal/provider/executor.go` (lines 1-95) — `Execute(ctx, spec, timeout, vb *ui.Verbose) (stdout, stderr string, err error)`. Already calls `vb.VerboseCommand`, `vb.VerbosePayload(len(spec.Stdin))`, `vb.VerboseRawOutput`, `vb.VerboseStderr` per invocation. FR-T11 (per-turn payload size + raw stdout/stderr) is satisfied by simply calling `provider.Execute` per turn.
11. `internal/ui/verbose.go` (lines 1-200) — `Verbose` sink, nil-safe. `VerboseCommand`, `VerboseRawOutput`, `VerboseStderr`, `VerbosePayload`, `VerboseWarn`, `VerboseRetry`, `VerboseRoles`.
12. `internal/ui/verbose_test.go` (lines 1-200) — table-driven verbose tests; the pattern for any new verbose method.
13. `internal/ui/output.go` (lines 1-200) — `UI` with `Progress`, `Success`, `Error`, plus pure helper `ProgressLabel(verb, model, provider string)`. The `"↳ "` prefix (line 19, `progressPrefix`) + `Progress` writing to STDERR (lines 121-125) is the FR-T5 seam.
14. `internal/ui/output_test.go` (lines 200-290) — `TestProgress_PrefixAndStream`, `TestProgressLabel`. Byte-exact assertion pattern for any new progress line.
15. `internal/cmd/default_action.go` (lines 160-205) — where the CLI emits the `"↳ Generating…"` line: `u.Progress(ui.ProgressLabel("Generating", labelModel, labelProvider))` (line 181) BEFORE `stagecoach.GenerateCommit`.
16. `internal/cmd/hookexec.go` (line 136) — hook path: `Progress: func() { u.Progress(ui.ProgressLabel("Generating", msgModel, labelProvider)) }`.
17. `internal/e2e/harness_test.go` (lines 1-260) — the e2e subprocess harness: `buildStagecoach`, `buildStub` (delegates to `stubtest.Build`), `writeStubConfig`, `stubEnv`, `runStagecoach`, `newRepo`, `seedCommit`, `waitForMarker`.
18. `internal/e2e/scenarios_test.go` (lines 1-120) — the e2e scenarios. `S2_OneFile_NoPlannerCall` shows the dual-mode stub+real pattern + the canary-marker trick.

---

## Key Code

### 1. The stub-agent mechanism (extend this for multi-turn)

**The binary** — `cmd/stubagent/main.go` — is a tiny Go binary compiled once per test process (`stubtest.Build`, `sync.Once`-cached). Behavior is driven ENTIRELY by `STAGECOACH_STUB_*` env vars that a test sets via the manifest's `Env` map. Critical flow (lines 27-95):

```go
// 1. Drain stdin FIRST (deadlock guard)
if sf := os.Getenv("STAGECOACH_STUB_STDINFILE"); sf != "" { ... tee ... } else { io.Copy(io.Discard, os.Stdin) }
// 1b. readiness marker (STAGECOACH_STUB_MARKER)
// 1c. argv capture (STAGECOACH_STUB_ARGSFILE)
// 2. sleep (STAGECOACH_STUB_SLEEP_MS) AFTER drain
// 3. stderr (STAGECOACH_STUB_STDERR)
// 4. stdout: OUT (single) OR script-indexed line
out := os.Getenv("STAGECOACH_STUB_OUT")
if scriptFile := os.Getenv("STAGECOACH_STUB_SCRIPT"); scriptFile != "" {
    out = selectScripted(scriptFile)
}
fmt.Fprint(os.Stdout, out)
os.Exit(envInt("STAGECOACH_STUB_EXIT", 0))
```

**Call-varying mode** (`selectScripted`, lines 99-115) reads a script file line-by-line and advances a file-backed counter (`STAGECOACH_STUB_COUNTER`) so each fresh process gets the next line; after the list is exhausted it CLAMPS TO THE LAST line:

```go
index := 0
if counterFile := os.Getenv("STAGECOACH_STUB_COUNTER"); counterFile != "" {
    index = readCounter(counterFile)
    writeCounter(counterFile, index+1) // serial callers — no race
}
if index < 0 || index >= len(lines) { index = len(lines) - 1 } // clamp to last
return lines[index]
```

This means **a multi-turn stub can ALREADY vary per invocation** — `NewScript(t, bin, []string{"turn1-out", "turn2-out", ...})` returns successive outputs. BUT: the counter is global to the script, keyed only on call-order. It does NOT key on a session id.

**KEY GAP for "recalls prior-turn content by session id"**: the stub has no concept of `--session-id`. The `STAGECOACH_STUB_ARGSFILE` knob already writes the rendered argv to a file (NUL-joined, lines 33-37), so a test can OBSERVE that the multi-turn render variant emitted `--session-id <value>` — but the stub does not currently echo prior-turn content based on it. Two extension options (delta says "extend the existing pattern"):
  - **(a) Cheap / recommended:** keep `NewScript` as the per-turn-output driver (it already returns 'ok' on priming turns then the message on the final turn — exactly the multi-turn test shape), AND add an assertion via `STAGECOACH_STUB_ARGSFILE` that the rendered argv for turn N contains `--session-id <stable-value>`. The stub does not need to recall content; the orchestrator's prompt builder does.
  - **(b) If true session recall is needed in the stub:** add a new `STAGECOACH_STUB_SESSION_DIR` knob to `cmd/stubagent/main.go` keyed off the `--session-id` argv token; the stub writes each turn's stdin payload to `<dir>/<session-id>/<turn>` and prepends prior turns' content to stdout. This is more invasive.

**`stubtest.Options`** (`stubtest.go` lines 17-29) is the single struct to extend if a new knob is added:

```go
type Options struct {
    Out            string // STAGECOACH_STUB_OUT
    Exit           int    // STAGECOACH_STUB_EXIT
    SleepMS        int    // STAGECOACH_STUB_SLEEP_MS
    Stderr         string // STAGECOACH_STUB_STDERR
    Script         string // STAGECOACH_STUB_SCRIPT path
    Counter        string // STAGECOACH_STUB_COUNTER path
    Output         string
    StripCodeFence *bool
    ArgsFile       string // STAGECOACH_STUB_ARGSFILE — observe rendered argv
}
```

`optsEnvMap` (lines 67-90) is the single source of truth translating Options → env vars; `Manifest(bin, o)` (lines 105-128) wraps it as a `provider.Manifest` with `Command` pointed at the stub.

### 2. CommitStaged end-to-end integration template

`TestCommitStaged_Success` (`generate_test.go` lines 70-115) is the canonical template:

```go
bin := stubtest.Build(t)
repo := t.TempDir()
initRepo(t, repo)
commitRaw(t, repo, "initial")
writeFile(t, repo, "new.txt", "hello world")
stageFile(t, repo, "new.txt")

m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add login"})
cfg := config.Defaults()

res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
```

Then asserts `res.CommitSHA` matches `shaRe` (`^[0-9a-f]{7,64}$`), `res.Subject`/`res.Message`/`res.Provider`, `len(res.Changes)>0`, `headSHA(t, repo) == res.CommitSHA`, and `gitOut(t, repo, "log", "--format=%B", "-n1", res.CommitSHA)` round-trips the message.

**Multi-turn-relevant sibling**: `TestCommitStaged_DedupeRetryThenSuccess` (lines 118-145) uses `stubtest.NewScript(t, bin, []string{"feat: existing", "feat: fresh"})` on a repo whose HEAD subject is `"feat: existing"` — exactly the per-turn-output shape the multi-turn test wants (priming turns → final message). It asserts the second scripted output won.

**Rescue / idempotent-index template**: `TestCommitStaged_ParseFailRescue` (lines 162-215) + `TestCommitStaged_IdempotentIndexOnFailure` (lines 318-365). The idempotent-index invariant (HEAD + staged diff unchanged after a rescue) is asserted via:
```go
beforeHEAD := headSHA(t, repo)
beforeIndex := gitOut(t, repo, "diff", "--cached", "--name-only")
... CommitStaged returns *RescueError ...
afterIndex := gitOut(t, repo, "diff", "--cached", "--name-only")
// assert unchanged
```

**Verbose-capture template** (useful for FR-T11): `TestCommitStaged_ResolvesSubProviderFromManifest` (lines 376-409) builds `Deps{..., Verbose: ui.NewVerbose(&buf, true)}` and asserts on `buf.String()` (the rendered command). For multi-turn, the same buffer captures every turn's `DEBUG: command:`, `DEBUG: payload: N bytes`, `DEBUG: raw output:`, `DEBUG: stderr:` lines (because `provider.Execute` emits all four per call — see below).

**Rendered-argv capture template** (useful to assert `--session-id`): `TestCommitStaged_MessageRoleOverride` (lines 339-376) sets `stubtest.Options{Out:..., ArgsFile: argsFile}` then reads `os.ReadFile(argsFile)` and splits on `"\x00"`:
```go
raw, _ := os.ReadFile(argsFile)
args := strings.Split(string(raw), "\x00")
if !sliceContains(args, "--model") || !sliceContains(args, "haiku") { ... }
```

### 3. `--verbose` per-turn logging (FR-T11) — already wired

`provider.Execute` (`executor.go` lines 71-92) emits four verbose lines EVERY call, passed a `*ui.Verbose`:

```go
vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), " "))
vb.VerbosePayload(len(spec.Stdin)) // size only — never contents
... cmd.Start(); cmd.Wait() ...
vb.VerboseRawOutput(out.String())
vb.VerboseStderr(errb.String()) // even on success
```

So **a multi-turn loop that calls `provider.Execute` once per turn inherits per-turn payload size + raw stdout/stderr logging for free.** No new verbose plumbing is needed for FR-T11. The `Verbose` sink (`internal/ui/verbose.go`) is nil-safe — every method early-returns on `v==nil || v.w==nil || !v.on`. Test pattern: `ui.NewVerbose(&buf, true)` (on) or `ui.NewVerbose(io.Discard, false)` (off).

`generate.go` also calls `deps.Verbose.VerboseRetry(attempt+1, reason)` inside its loop (lines 178, 195) — the natural seam for a "falling back to multi-turn" verbose line if one is desired in addition to the progress line.

### 4. Progress "↳" line (FR-T5) — the seam

`ui.Progress` (`output.go` lines 121-125) writes one line to STDERR with the `"↳ "` prefix:
```go
const progressPrefix = "↳ "
func (u *UI) Progress(msg string) { fmt.Fprintln(u.stderr, progressPrefix+msg) }
```

`ui.ProgressLabel(verb, model, provider string)` (lines 143-152) is the pure helper that builds the body: returns `"<verb> with <model> in <provider>…"` (or `"<verb>…"` when nothing is resolved).

**For FR-T5's "falling back to multi-turn: N+1 turns, ~Mm total" one-line notice**, the natural approach is a NEW one-line progress call. Two options consistent with existing style:
  - Add a dedicated helper alongside `ProgressLabel` (e.g. `MultiTurnFallbackLabel(turns int, est string) string`) — pure, trivially unit-testable like `ProgressLabel`.
  - Or call `u.Progress(fmt.Sprintf("falling back to multi-turn: %d turns, ~%s total", turns, est))` directly at the fallback site inside the generate loop.

The CLI/hook call sites are `internal/cmd/default_action.go:181` and `internal/cmd/hookexec.go:136`. NOTE: `CommitStaged` itself does NOT print progress directly — `Deps.Progress` is a `func()` callback (generate.go lines 24-28) invoked only by the hook layer; the default-action CLI prints the line itself before calling `stagecoach.GenerateCommit`. So a fallback progress line should either (a) be emitted by `CommitStaged` via a NEW `Deps` callback, or (b) be surfaced through `Result`/error and printed by the CLI. The existing `Deps.Progress func()` is a `nil`-safe single-shot callback; a per-turn or fallback callback would be a new field.

### 5. Render unit-test pattern (for the multi-turn render variant)

`internal/provider/render_test.go` is golden/table-driven. `TestRender_GoldenPerProvider` (lines 36-90) is the keystone: a `cases` slice of `{name, m, model, wantCmd, wantArgs, wantStdin}` run under `t.Run(tc.name, ...)`, asserting `spec.Command`, `reflect.DeepEqual(spec.Args, tc.wantArgs)`, `spec.Stdin`. `TestRender_Pi_ByteForByteCommitPi` (lines 94-108) is the byte-exact single-case form.

For the multi-turn render variant (drop `--no-session`, add `--session-id <id>`), the test shape is:
```go
spec, err := m.Render(model, sys, payload, reasoning, <new RenderMultiTurn mode or session param>)
wantArgs := []string{..., "--session-id", sessionID, ...} // no "--no-session"
if !reflect.DeepEqual(spec.Args, wantArgs) { ... }
```
Helpers `containsPair`/`containsToken` (lines 396-416) assert a flag-value pair or single token is present. `TestRender_DoesNotMutateManifest` (lines 311-322) is the non-mutation regression guard.

The current `Render` signature (`render.go` line 65):
```go
func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error)
```
`mode` is VARIADIC (default `RenderBare`) so adding a `RenderMultiTurn` constant keeps every existing caller byte-identical — same pattern that added `RenderTooled`. A `--session-id` value would need a new parameter (or a new `RenderOption` struct) since `Render` currently has no session argument.

---

## Architecture

```
Test
 │ stubtest.Build(t) ── compiles cmd/stubagent ONCE (sync.Once cache)
 │ stubtest.Manifest(bin, Options{...}) or NewScript(t, bin, []string{...})
 ▼
provider.Manifest{Env: STAGECOACH_STUB_*}
 │ m.Render(model, sys, payload, reasoning, mode...) ── render.go
 ▼
provider.CmdSpec{Command, Args, Stdin, Env}
 │ provider.Execute(ctx, spec, timeout, vb) ── executor.go
 │   ├─ vb.VerboseCommand / VerbosePayload   (pre-spawn)
 │   ├─ cmd.Start + signal.RegisterChild
 │   ├─ cmd.Wait
 │   └─ vb.VerboseRawOutput / VerboseStderr  (post-wait, both success+fail)
 ▼
(stdout, stderr, err) ── consumed by generate.CommitStaged's loop
```

`generate.CommitStaged` (`generate.go` lines 130-205) is the bounded loop caller. It:
1. captures parent + builds system prompt (once)
2. StagedDiff (empty → `ErrNothingToCommit`)
3. WriteTree (snapshot — `signal.SetSnapshot`)
4. RecentSubjects (once)
5. **LOOP** (`for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++`):
   - `prompt.BuildUserPayload(diff, rejected)` (+ retry-instruction prepend on parse-fail)
   - `deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)` → spec
   - `provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)` → out, _, err
   - timeout → `*RescueError{Kind: ErrTimeout}` (immediate)
   - `provider.ParseOutput` → not ok → `VerboseRetry`, continue
   - `FinalizeMessage` + `IsDuplicate` → duplicate → `VerboseRetry`, continue
   - else success, break
6. not success → `*RescueError{Kind: ErrRescue}`
7. EditMessage gate → CommitTree → UpdateRefCAS → DiffTree → Result

The multi-turn fallback plugs INTO this loop (or as a follow-on phase after it exhausts): per-turn `Render` with a multi-turn flag-set + per-turn `Execute` (which gives free per-turn verbose). The `Verbose` and `Progress` UI layers are downstream sinks; `CommitStaged` threads `*ui.Verbose` via `Deps`.

---

## Start Here

Open **`internal/stubtest/stubtest.go`** first — `Options` (line 17) and `optsEnvMap` (line 67) are where any new stub knob is added, and `NewScript` (line 133) already provides the per-turn-output mechanism the multi-turn test needs. Then open **`cmd/stubagent/main.go`** (lines 27-95) to see exactly how env vars become stub behavior — this is the binary to extend IF the stub must truly recall prior-turn content by session id.

Second, open **`internal/generate/generate_test.go:70` (`TestCommitStaged_Success`)** and **`:118` (`TestCommitStaged_DedupeRetryThenSuccess`)** — copy either as the integration-test skeleton for the multi-turn path. The fixture helpers (`initRepo`/`writeFile`/`stageFile`/`headSHA`/`commitRaw`/`gitOut`/`runGit`/`shaRe`) are defined in the same file (lines 24-67).

Third, for the FR-T5 progress line, open **`internal/ui/output.go:121` (`Progress`) + `:143` (`ProgressLabel`)** and mirror the pattern; for FR-T11 verbose, no new plumbing is needed — `provider.Execute` already emits payload-size + raw stdout/stderr per turn (`executor.go:71-92`).

---

## Notes / Open Questions for the parent

- **Stub session-recall**: `NewScript` already varies output per-invocation (call-order-indexed), which is enough to simulate "ok on priming turns, message on final turn". True recall-by-session-id requires a new stub knob (`STAGECOACH_STUB_SESSION_DIR`) — confirm whether the test contract needs the stub to ECHO prior-turn content, or only needs the orchestrator to RE-SEND prior-turn content (in which case `NewScript` + an `ArgsFile` assertion on `--session-id` suffices).
- **`Render` signature change**: adding `--session-id` requires either a new variadic option or a `RenderMultiTurn` mode + a session-value parameter. The variadic `mode` pattern (`RenderTooled`) keeps existing callers byte-identical and is the established convention.
- **Progress emission site**: `CommitStaged` does not print progress itself (only `Deps.Progress`, a nil-safe hook callback). A fallback notice either needs a new `Deps` callback field or must be surfaced through `Result`/error and printed by the CLI (`default_action.go:181` / `hookexec.go:136`).
- **`--no-session` lives in BOTH `BareFlags` and `TooledFlags`** of `builtinPi()` (`builtin.go:60` and `:92`). The multi-turn variant must drop it from whichever flag-set the multi-turn render path selects, and add `--session-id`.

```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "Read-only scout task: no scope widened, no implementation changes proposed beyond the assigned research brief. Mapped all 5 requested areas (stubtest mechanism, CommitStaged integration template, --verbose per-turn logging, ↳ progress seam, render unit-test pattern) with exact file paths, line numbers, and code excerpts. Identified the key gap (stub has no --session-id concept) and 2 extension options without committing to a design."
    }
  ],
  "changedFiles": [],
  "testsAddedOrUpdated": [],
  "commandsRun": [
    {
      "command": "ls internal/{stubtest,generate,ui,e2e,provider} cmd",
      "result": "passed",
      "summary": "Enumerated target directories + cmd/stubagent."
    },
    {
      "command": "grep -n 'ProgressLabel|\\.Progress\\(|Generating|session-id|--session-id|--no-session'",
      "result": "passed",
      "summary": "Located progress-line call sites (default_action.go:181, hookexec.go:136) and --no-session occurrences in builtin.go/render_test.go."
    }
  ],
  "validationOutput": [
    "No build/test commands run — scout/research role only; acceptance is the report content itself, written to /home/dustin/projects/stagecoach/plan/009_5c53066d64b3/architecture/research-tests-ui.md."
  ],
  "residualRisks": [
    "none — no files modified; the report's open questions (stub session-recall design, Render signature change shape, progress emission site) are decision points for the parent, not risks introduced by this scout."
  ],
  "noStagedFiles": true,
  "diffSummary": "No diff. Only the research report file was written to plan/009_5c53066d64b3/architecture/research-tests-ui.md; no source files were touched.",
  "reviewFindings": [
    "no blockers — read-only scout; findings are descriptive with exact line numbers for the implementing agent."
  ],
  "manualNotes": "Key actionable findings: (1) NewScript already provides per-turn output variation via call-order indexing — likely sufficient for the multi-turn test without a new stub knob; (2) provider.Execute already emits per-turn VerbosePayload/VerboseRawOutput/VerboseStderr, so FR-T11 needs no new verbose plumbing; (3) Render's variadic mode pattern is the established way to add a multi-turn flag-set without breaking callers; (4) --no-session appears in BOTH pi BareFlags and TooledFlags (builtin.go:60,92); (5) CommitStaged does not print progress itself — a fallback notice needs a new Deps callback or Result/error surfacing."
}
```
