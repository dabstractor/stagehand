---
name: "P1.M2.T1.S1 (bugfix Issue 2) — Fix generate.CommitStaged and runPipeline to use ResolveRoleModel(\"message\")"
description: |
  Bugfix for Issue 2 (PRD §9.15 FR-R3): per-role `message` overrides (`--message-model` /
  `--message-reasoning` / `--message-provider` / `[role.message]`) are silently IGNORED on the
  single-commit path. The loaders correctly write them into `cfg.Roles["message"]` and the flags are
  registered, but BOTH single-commit Render call sites pass the **global** `cfg.Model`/`cfg.Reasoning`
  directly instead of resolving the `message` role. Fix: resolve `config.ResolveRoleModel("message", cfg)`
  and pass its (model, reasoning) to `Render` (mirroring the already-correct decompose path at
  `internal/decompose/message.go:103`). With no message override, ResolveRoleModel returns
  `(cfg.Provider, cfg.Model, cfg.Reasoning)` — **back-compatible** (the common case is byte-identical).

  ⚠️ **THE central design call — resolve the message role ONCE before each loop, feed model+reasoning to
  Render, and propagate model to Result.Model.** Two call sites:
  (A) `internal/generate/generate.go` `CommitStaged` — add `_, msgModel, msgReasoning :=
  config.ResolveRoleModel("message", cfg)` before the dedupe loop (near the existing `resolved`/
  `retryInstr` setup); change L196 `Render(cfg.Model, …, cfg.Reasoning)` → `Render(msgModel, …,
  msgReasoning)`; change step-10 `model := cfg.Model` → `model := msgModel` (keep the
  `if model == "" { model = *resolved.DefaultModel }` fallback so Result.Model reports the concrete model).
  (B) `pkg/stagecoach/stagecoach.go` `runPipeline` — add the same ResolveRoleModel call before the loop;
  change the `model` local var (~L447) `model := cfg.Model` → `model := msgModel` (keep the DefaultModel
  fallback); change L467 `Render(cfg.Model, …, cfg.Reasoning)` → `Render(msgModel, …, msgReasoning)`. The
  two `Result.Model` returns (~L529 dryRun, ~L566 commit) already reference the `model` var — NO separate
  edit (they pick up msgModel automatically).

  ⚠️ **THE second design call — DISCARD the resolved provider with `_`, NOT `msgProv`.** Contract (B)
  wrote `msgProv, msgModel, msgReasoning := …` but `msgProv` is unused in runPipeline (provider→manifest
  selection is P1.M2.T2.S1's job in buildDeps) ⇒ Go **"declared and not used" compile error**. Use
  `_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)` in BOTH sites (generate.go's
  contract (A) already uses `_`). The manifest reaching these functions is `deps.Manifest`, selected
  upstream by buildDeps — unchanged by this task. Do NOT wire msgProv into manifest selection here
  (that overlaps with P1.M2.T2.S1).

  ⚠️ **THE third design call — observability for the test.** `provider.Manifest` is a CONCRETE struct
  (`Deps.Manifest provider.Manifest`) and `Render` is a value receiver, so a per-instance "recording
  Render" is impossible. `cmd/stubagent` ignores its argv (stdin+env only). To faithfully assert "Render
  received model=haiku, reasoning=high" end-to-end, add a TINY `STAGECOACH_STUB_ARGSFILE` env knob to
  cmd/stubagent (write `os.Args` to a file, mirroring the existing `STAGECOACH_STUB_MARKER` file-write at
  stubagent main.go:~40) + an `ArgsFile` field on `stubtest.Options`. Then tests read that file and assert
  the rendered argv contains `--model haiku` + the reasoning token. MODEL is ALSO observable via
  `Result.Model` (deterministic, regression-catching on its own). See research note for the no-stub-change
  minimum (Result.Model + direct `spec.Args` assertion via the realagent_test.go:83 pattern).

  SCOPE: edits to `internal/generate/generate.go`, `pkg/stagecoach/stagecoach.go`, `cmd/stubagent/main.go`
  (+ `internal/stubtest/stubtest.go` for the ArgsFile option), and tests in `internal/generate/generate_test.go`
  + `pkg/stagecoach/*_test.go`. NO config-layer changes (the loaders/flags already work), NO render.go/
  manifest.go, NO default_action.go, NO docs (README/cli.md already document the flags — they just start
  working). INPUT = `cfg config.Config` (already received by both funcs) carrying `cfg.Roles["message"]`.
  OUTPUT = `--message-*` / `[role.message]` now drive the single-commit Render (both CommitStaged and
  runPipeline). DOCS: none (FR-R3 behavior was already documented; this makes it true).
---

## Goal

**Feature Goal**: Make the single-commit generation path honor per-role `message` overrides by resolving
the `message` role via `config.ResolveRoleModel("message", cfg)` and feeding its (model, reasoning) to
`provider.Manifest.Render` — in BOTH `generate.CommitStaged` and `pkg/stagecoach.runPipeline` — with no
behavior change when no message override is set (back-compatible global fallback).

**Deliverable** (edits to existing files; one tiny test-infra addition):
1. **`internal/generate/generate.go`** (`CommitStaged`) — add `_, msgModel, msgReasoning :=
   config.ResolveRoleModel("message", cfg)` before the dedupe loop; change the Render call to use
   `msgModel`/`msgReasoning`; change step-10 `model := cfg.Model` → `model := msgModel`.
2. **`pkg/stagecoach/stagecoach.go`** (`runPipeline`) — add the same ResolveRoleModel call before the loop;
   change the `model` local var to start from `msgModel`; change the Render call to use
   `msgModel`/`msgReasoning` (the two `Result.Model` returns inherit `model` automatically).
3. **`cmd/stubagent/main.go`** + **`internal/stubtest/stubtest.go`** — add a `STAGECOACH_STUB_ARGSFILE` knob
   (write `os.Args` join-by-NUL or newline to a file) + `stubtest.Options.ArgsFile` so tests can observe
   the exact rendered argv end-to-end.
4. **`internal/generate/generate_test.go`** + **`pkg/stagecoach/*_test.go`** — regression tests:
   (a) message override (model+reasoning) reaches Render/Result; (b) no-override case is unchanged.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l` clean; `go test -race ./...` green
with the new regression tests passing (and all existing tests unchanged). Concretely: with
`cfg.Roles["message"] = {Model:"haiku", Reasoning:"high"}` and `cfg.Model==""`/`cfg.Reasoning==""`,
`CommitStaged` and `runPipeline` Render with model="haiku"/reasoning="high" and `Result.Model=="haiku"`;
with `cfg.Roles` empty, behavior is byte-identical to today (Render gets cfg.Model/cfg.Reasoning;
Result.Model == cfg.Model or manifest default). go.mod/go.sum unchanged.

## User Persona

**Target User**: Any user who configures the message role on the single-commit (default) path —
`stagecoach --message-model haiku`, `STAGECOACH_MESSAGE_REASONING=high`, or `[role.message] model=…` in
config. Transitively PRD §9.15 FR-R3 ("every role exposes all three flags, including message").

**Use Case**: `stagecoach --message-model haiku` (with something staged) must generate the commit with the
haiku model. Today it silently uses the global/manifest default.

**User Journey**: flag/env/file → `Load()` writes `cfg.Roles["message"]` → single-commit path resolves
`ResolveRoleModel("message", cfg)` → `Render(msgModel, …, msgReasoning)` → Execute → commit. (Today the
resolve step is missing, so the override is dropped.)

**Pain Points Addressed**: removes "I set `--message-model` and nothing changed" (FR-R3 violation) without
disturbing the common no-override path.

## Why

- **Fixes a P0/Major FR-R3 violation.** The flags/env/config keys are documented and wired into cfg; only
  the Render call site was wrong. This is the minimal, surgical fix.
- **Parity with the decompose path.** `internal/decompose/message.go:103` already resolves the message
  role correctly; the single-commit path is the lone holdout. This makes the two paths consistent.
- **Back-compatible.** With no message override, `ResolveRoleModel("message", cfg)` returns the globals —
  identical to today. Only explicit message overrides (the FR-R3 use case) change behavior.
- **No new surface.** README/cli.md already document `--message-*`; this just makes them truthful.

## What

Both single-commit Render call sites resolve the `message` role before rendering and pass its
model+reasoning to `Render`; `Result.Model` reflects the resolved model. A tiny stub knob lets tests
observe the rendered argv. No config-layer, render, manifest, CLI-routing, or doc changes.

### Success Criteria

- [ ] `generate.go` `CommitStaged`: `_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)`
      before the loop; `Render(msgModel, sysPrompt, payload, msgReasoning)`; step-10 `model := msgModel`.
- [ ] `stagecoach.go` `runPipeline`: `_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)`
      before the loop; `model := msgModel` (DefaultModel fallback kept); `Render(msgModel, …, msgReasoning)`.
- [ ] Neither site uses `msgProv` (discard with `_`) — provider→manifest selection stays in buildDeps
      (P1.M2.T2.S1). No "declared and not used" compile error.
- [ ] `cmd/stubagent` writes `os.Args` to `STAGECOACH_STUB_ARGSFILE` when set; `stubtest.Options.ArgsFile`
      threads it into the Env map.
- [ ] New tests: message override (model+reasoning) reaches Render (`ARGSFILE` contains `--model haiku` +
      reasoning token) and `Result.Model == "haiku"`; no-override case unchanged (regression-safe).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l`, `go test -race ./...` clean/green; go.mod/go.sum
      unchanged; no files outside the listed edits.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the two exact call sites (quoted
below), the reference pattern (`message.go:103`), the `msgProv`-unused gotcha, and the stub-knob test
approach. No provider/config internals beyond "ResolveRoleModel returns (provider, model, reasoning)".

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/P1M2T1S1/research/message_role_single_path.md
  why: the verified two-call-site fix, the msgProv-unused compile GOTCHA (use `_`), the reference pattern,
       the test-observability constraint + STAGECOACH_STUB_ARGSFILE approach, and the back-comat proof.
  critical: contract (B)'s literal `msgProv, ...` WILL NOT COMPILE (unused var). Discard with `_` in both
       sites. Provider/manifest selection is P1.M2.T2.S1 — out of scope.

- file: internal/decompose/message.go   (L103 + L125-130)
  why: the ALREADY-CORRECT reference pattern on the decompose path. `_, mdl, rsn := config.ResolveRoleModel
       ("message", deps.Config)` then `Render(mdl, sysPrompt, payload, rsn, provider.RenderBare)`. Mirror it
       (single-commit uses default mode — no RenderMode arg).
  pattern: resolve once before the loop; feed model+reasoning to Render in the loop body.

- file: internal/config/roles.go   (ResolveRoleModel, L41)
  why: the function you call. Signature `func ResolveRoleModel(role string, cfg Config) (provider, model,
       reasoning string)`. Per-role → global → shipped-default. No message override ⇒ (cfg.Provider,
       cfg.Model, cfg.Reasoning) — back-compatible.
  gotcha: it does NOT consult any manifest; provider="" ⇒ registry auto-detect (not your concern here).

- file: internal/generate/generate.go   (CommitStaged; Render at L196; Result.Model at L287-289)
  why: call site (A). `resolved := deps.Manifest.Resolve()` + `retryInstr := …` are computed before the
       loop (~L178-180) — add the ResolveRoleModel call right there. `resolved` is in scope at step 10.
  pattern: see Implementation Blueprint (A).

- file: pkg/stagecoach/stagecoach.go   (runPipeline L401; model var L447-449; Render L467; Result L529,L566)
  why: call site (B). The `model` var feeds BOTH Result returns — change its initializer to msgModel and
       the two returns inherit it (no edit at L529/L566). Render L467 uses cfg.Model/cfg.Reasoning today.
  pattern: see Implementation Blueprint (B).

- file: internal/stubtest/stubtest.go   + cmd/stubagent/main.go
  why: the test harness. stubtest.Manifest builds a real provider.Manifest backed by the stub binary;
       stubtest.Options→STAGECOACH_STUB_* env. stubagent drains stdin, reads env, writes canned stdout
       (IGNORES argv). Add STAGECOACH_STUB_ARGSFILE (write os.Args) + Options.ArgsFile to observe the
       rendered argv end-to-end.
  pattern: mirror the STAGECOACH_STUB_MARKER file-write at stubagent main.go:~40 (`os.WriteFile(marker,…)`).
  gotcha: provider.Manifest is a STRUCT — you cannot substitute a recording mock via Deps.Manifest; the
       ARGSFILE knob is how you observe Render's output (the rendered command) through the real Execute seam.

- file: internal/generate/realagent_test.go   (L83: `args := strings.Join(spec.Args, " ")`)
  why: the existing pattern for inspecting a rendered CmdSpec.Args directly. Use it for a no-stub-change
       minimum assertion that `manifest.Render("haiku", …, "high")` ⇒ Args contains the reasoning token.
  pattern: call Render directly, assert on spec.Args (proves Render translates reasoning→token; pair with
       Result.Model for the CommitStaged-routing proof).

- file: PRD.md (bug report) §9.15 FR-R3 — "Every role exposes all three flags, including message — no role
       is a special case that omits a flag."
  why: the requirement this fixes. The flag/env/file wiring already exists; only the Render call site was
       wrong.
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/generate.go        # CommitStaged — Render L196, Result.Model L287-289   ← EDIT (A)
pkg/stagecoach/stagecoach.go           # runPipeline — model var L447, Render L467, Result L529/L566  ← EDIT (B)
cmd/stubagent/main.go                # stub binary (ignores argv today)                      ← EDIT (ARGSFILE knob)
internal/stubtest/stubtest.go        # Options→STAGECOACH_STUB_* env                         ← EDIT (Options.ArgsFile)
internal/generate/generate_test.go   # CommitStaged integration tests (stubtest harness)    ← EDIT (regression tests)
pkg/stagecoach/*_test.go              # GenerateCommit/runPipeline tests                     ← EDIT (regression tests)
internal/config/roles.go             # ResolveRoleModel (3-return) — INPUT, NO edit
internal/decompose/message.go        # reference pattern (already correct) — NO edit
internal/cmd/default_action.go       # runGenerate — NO edit (provider routing is P1.M2.T2.S1)
internal/provider/{render,manifest}.go  # Render/Manifest — INPUT, NO edit
go.mod / go.sum                      # unchanged
```

### Desired Codebase tree with files to be added

```bash
# NO new files. All changes are EDITS to the files listed above.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: contract (B)'s `msgProv, msgModel, msgReasoning := …` is an UNUSED-VARIABLE compile error in
// runPipeline (msgProv is not used — provider→manifest selection is P1.M2.T2.S1's buildDeps job). Use
// `_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)` in BOTH sites. generate.go's
// contract (A) already uses `_`.

// CRITICAL: keep the DefaultModel fallback on the Result.Model path. Render receives the RAW msgModel
// (may be "" ⇒ Render/manifest uses its default); Result.Model must report the CONCRETE model, so keep
// `if model == "" { model = *resolved.DefaultModel }` after `model := msgModel`. (resolved is in scope:
// generate.go computes it before the loop; stagecoach.go computes it at ~L446.)

// CRITICAL: stagecoach.go's two Result.Model returns (dryRun ~L529, commit ~L566) reference the `model`
// LOCAL VAR, not cfg.Model. So changing `model := cfg.Model` → `model := msgModel` (one line, ~L447)
// propagates to BOTH returns — do NOT edit L529/L566 separately (you'd risk divergence).

// CRITICAL (test): provider.Manifest is a CONCRETE struct; Deps.Manifest is not an interface. You CANNOT
// substitute a per-instance recording Render. cmd/stubagent ignores argv. To observe "Render received
// model=haiku, reasoning=high" end-to-end, add STAGECOACH_STUB_ARGSFILE (stub writes os.Args) and read it
// in the test. Result.Model is the no-stub-change minimum observable for MODEL.

// GOTCHA: ResolveRoleModel("message", cfg) with NO message override returns (cfg.Provider, cfg.Model,
// cfg.Reasoning) — so the common case is byte-identical to today. Do NOT add special-casing for "no
// override"; the resolution IS the back-compatible fallback.

// GOTCHA: do NOT wire msgProv into manifest selection (buildDeps) here — that is P1.M2.T2.S1 and would
// overlap. The manifest reaching runPipeline/CommitStaged is deps.Manifest (unchanged).

// GOTCHA: stubagent MUST drain stdin BEFORE writing the args file (the existing deadlock guard: the
// executor pipes the payload via a bounded ~64 KiB pipe; not draining first can deadlock parent+child).
// Write ARGSFILE after the existing io.Copy(io.Discard, os.Stdin), near the MARKER write.

// GOTCHA: encode os.Args in the args file safely — join with NUL ("\x00") or newline; tests split on the
// same delimiter. (A flat space-join would break on flag values containing spaces; NUL is safest.)
```

## Implementation Blueprint

### Data models and structure

No new types. The only structural addition is a stub env knob:

```go
// cmd/stubagent/main.go — after the stdin drain + MARKER write, near the existing STAGECOACH_STUB_MARKER block:
if argsFile := os.Getenv("STAGECOACH_STUB_ARGSFILE"); argsFile != "" {
	// Join argv with NUL so flag values with spaces survive; tests split on "\x00".
	_ = os.WriteFile(argsFile, []byte(strings.Join(os.Args, "\x00")), 0o644)
}

// internal/stubtest/stubtest.go — add to Options:
type Options struct {
	// ... existing fields ...
	ArgsFile string // STAGECOACH_STUB_ARGSFILE (writes the stub's os.Args to this path — observe rendered argv)
}
// and in optsEnvMap: if o.ArgsFile != "" { m["STAGECOACH_STUB_ARGSFILE"] = o.ArgsFile }
```

```go
// internal/generate/generate.go — CommitStaged (before the dedupe loop, near `resolved`/`retryInstr`):
// v3 FR-R3: the single-commit path's active role is "message" — resolve its model/reasoning per-field
// (flag > env > [role.message] > global [defaults]) so --message-model / [role.message] drive Render.
// No message override ⇒ (cfg.Provider, cfg.Model, cfg.Reasoning) — back-compatible. The provider is
// unused here: the manifest is deps.Manifest, selected upstream by buildDeps (P1.M2.T2.S1 wires the
// message provider into that selection).
_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
// ... inside the loop:
spec, rerr := deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)
// ... step 10:
model := msgModel
if model == "" {
	model = *resolved.DefaultModel
}

// pkg/stagecoach/stagecoach.go — runPipeline (replace the L447-449 block):
resolved := deps.Manifest.Resolve()
_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg) // FR-R3: honor [role.message] (back-compat fallback)
model := msgModel
if model == "" {
	model = *resolved.DefaultModel
}
// ... inside the loop (L467):
spec, rerr := deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)
// (Result returns at L529/L566 already use `model` — no edit needed there.)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: generate.go CommitStaged — resolve message role for Render + Result.Model
  - ADD before the dedupe loop (right after `retryInstr := *resolved.RetryInstruction`):
    `_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)` (+ the FR-R3/back-compat comment).
  - CHANGE L196: `deps.Manifest.Render(cfg.Model, sysPrompt, payload, cfg.Reasoning)` →
    `deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)`.
  - CHANGE step-10 (L287): `model := cfg.Model` → `model := msgModel` (keep the `if model == ""` DefaultModel fallback).
  - GOTCHA: use `_` for the provider (NOT msgProv) — unused-var compile error otherwise.

Task 2: stagecoach.go runPipeline — resolve message role for Render + the model var
  - ADD before the loop (right after `resolved := deps.Manifest.Resolve()`): the same
    `_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)` line.
  - CHANGE the model var (L447): `model := cfg.Model` → `model := msgModel` (keep DefaultModel fallback).
  - CHANGE L467: `deps.Manifest.Render(cfg.Model, sysPrompt, payload, cfg.Reasoning)` →
    `deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)`.
  - GOTCHA: do NOT edit L529/L566 — they reference `model` and inherit msgModel. Use `_` for the provider.

Task 3: stub argv-observation knob (test infra)
  - cmd/stubagent/main.go: after the stdin drain + MARKER write, add the STAGECOACH_STUB_ARGSFILE block
    (os.WriteFile of strings.Join(os.Args, "\x00")).
  - internal/stubtest/stubtest.go: add `ArgsFile string` to Options; in optsEnvMap add
    `if o.ArgsFile != "" { m["STAGECOACH_STUB_ARGSFILE"] = o.ArgsFile }`.
  - WHY: provider.Manifest is a struct (no mock Render); the stub ignores argv today. This knob lets a
    test read the exact rendered argv end-to-end (model + reasoning token).

Task 4: regression tests
  - internal/generate/generate_test.go:
      * TestCommitStaged_MessageRoleOverride: build a manifest with ModelFlag="--model",
        ReasoningLevels={"high":["--thinking","high"]}, Command=stub, Output="raw"; cfg.Roles["message"]=
        {Model:"haiku",Reasoning:"high"}, cfg.Model="", cfg.Reasoning=""; stubtest Options{Out:"feat: x",
        ArgsFile:<tmpfile>}. CommitStaged → read argsfile, assert it contains "--model","haiku" and
        "--thinking","high"; assert Result.Model=="haiku".
      * TestCommitStaged_NoMessageOverride_Regression: cfg.Roles=nil, cfg.Model="openrouter/gpt-5.4";
        assert argsfile has "--model","openrouter/gpt-5.4" and NO "--thinking"; Result.Model==
        "openrouter/gpt-5.4" (mirrors the existing L428 fixture — regression-safe).
  - pkg/stagecoach/*_test.go:
      * TestRunPipeline_MessageRoleOverride (via GenerateCommit, DryRun:true): cfg.Roles["message"]=
        {Model:"haiku"}; assert Result.Model=="haiku" (the contract's dryRun assertion).
      * TestRunPipeline_NoMessageOverride_Regression: cfg.Roles=nil, cfg.Model=<X>; Result.Model==<X>.
  - MINIMUM (if Task 3 is deferred): drop the argsfile assertions; keep Result.Model=="haiku" (model) +
    a direct `manifest.Render("haiku", …, "high")` → spec.Args contains "--thinking" (reasoning, via the
    realagent_test.go:83 pattern) + the no-override Result.Model regression.

Task 5: VERIFY (no further file change)
  - RUN the full Validation Loop. go.mod/go.sum byte-unchanged. No files outside the listed edits.
    Existing generate/stagecoach tests stay green (no-override path is byte-identical).
```

### Implementation Patterns & Key Details

```go
// The single ResolveRoleModel call feeds BOTH Render inputs + Result.Model — one source of truth:
_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg) // before the loop
// ... in the loop:
spec, rerr := deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)
// ... Result.Model (concrete, with manifest-default fallback):
model := msgModel
if model == "" {
	model = *resolved.DefaultModel
}

// stagecoach.go: the `model` var is shared by BOTH Result returns — change one initializer, both update:
model := msgModel
if model == "" {
	model = *resolved.DefaultModel
}
// ... later, unchanged:
return Result{…, Model: model}, nil   // dryRun (L529) AND commit (L566) both reference `model`
```

```go
// generate_test.go — the override test (model + reasoning observable end-to-end via ARGSFILE):
func TestCommitStaged_MessageRoleOverride(t *testing.T) {
	bin := stubtest.Build(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add login", ArgsFile: argsFile})
	m.ModelFlag = strPtr("--model")                       // emit --model <model>
	m.ReasoningLevels = map[string][]string{"high": {"--thinking", "high"}}
	cfg := config.Defaults()
	cfg.Roles = map[string]config.RoleConfig{"message": {Model: "haiku", Reasoning: "high"}}
	// cfg.Model == "" && cfg.Reasoning == "" (Defaults)
	… // stage a file in a temp repo, run CommitStaged(ctx, Deps{Git: git.New(repo), Manifest: m}, cfg)
	raw, _ := os.ReadFile(argsFile)
	args := strings.Split(string(raw), "\x00")
	if !sliceContains(args, "--model") || !sliceContains(args, "haiku") {
		t.Errorf("Render did not receive message model haiku; args=%v", args)
	}
	if !sliceContains(args, "--thinking") || !sliceContains(args, "high") {
		t.Errorf("Render did not receive message reasoning high; args=%v", args)
	}
	if res.Model != "haiku" {
		t.Errorf("Result.Model=%q want haiku", res.Model)
	}
}
// (sliceContains + strPtr are trivial local helpers; strPtr = func(s string)*string{ return &s }.)
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — no new dep. go mod tidy MUST be a no-op.

FROZEN / NOT-EDITED:
  - internal/config/* (loaders + flags already populate cfg.Roles["message"] correctly — the bug is NOT there).
  - internal/provider/{render,manifest}.go (Render already takes reasoning; the 4th arg).
  - internal/decompose/message.go (the already-correct reference — mirror, don't edit).
  - internal/cmd/default_action.go (runGenerate; provider→manifest routing is P1.M2.T2.S1).
  - README.md / docs/cli.md (the flags are already documented; they just start working — DOCS: none).

DOWNSTREAM / NEXT (do NOT implement here):
  - P1.M2.T2.S1: use the message role's PROVIDER in buildDeps to select the manifest (the msgProv this
    task discards with `_`). After that task, a `--message-provider` override selects a different agent
    manifest; this task only routes model+reasoning to Render on the already-selected manifest.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/generate/generate.go pkg/stagecoach/stagecoach.go cmd/stubagent/main.go \
  internal/stubtest/stubtest.go internal/generate/generate_test.go pkg/stagecoach/*_test.go
test -z "$(gofmt -l internal/ pkg/ cmd/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./...        # Catches the msgProv unused-var if you forgot `_`.
go build ./...      # Whole module compiles (stubagent rebuilds in tests via stubtest.Build).
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: clean. If `go vet`/`go build` flags an unused `msgProv`, switch to `_` (see gotcha).
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./internal/generate/ -v -run 'CommitStaged|MessageRole'   # override + no-override regression
go test -race ./pkg/stagecoach/ -v      -run 'Pipeline|GenerateCommit|MessageRole'
go test -race ./internal/stubtest/ ./...   # stub knob + full suite (no regressions)
# Expected: PASS. Key assertions: with cfg.Roles["message"]={Model:"haiku",Reasoning:"high"} + cfg.Model=""
#   → Render argv has --model haiku + the reasoning token AND Result.Model=="haiku"; with cfg.Roles empty
#   → Render argv has --model <cfg.Model> + no reasoning token AND Result.Model==cfg.Model (regression-safe).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only the listed files changed:
git diff --name-only | grep -Ev 'internal/generate/generate\.go|pkg/stagecoach/stagecoach\.go|cmd/stubagent/main\.go|internal/stubtest/stubtest\.go|internal/generate/generate_test\.go|pkg/stagecoach/.*_test\.go' \
  && echo "UNEXPECTED file changed" || echo "only listed files changed (good)"
# Functional smoke (requires an installed agent OR rely on the stub-backed unit tests above): the unit
# tests with the stub manifest ARE the integration proof (real Execute seam, real Render, real ResolveRoleModel).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Back-compat property (optional throwaway): for cfg with cfg.Roles empty, ResolveRoleModel("message", cfg)
# returns exactly (cfg.Provider, cfg.Model, cfg.Reasoning) — assert equality in a quick test to pin the
# no-regression invariant. (roles_test.go already covers ResolveRoleModel; this is belt-and-suspenders for
# the single-commit wiring.) golangci-lint: `make lint` (project-wide gate).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/ pkg/ cmd/`, `go mod tidy` no-op.
- [ ] Level 2 green: `go test -race ./...` (new regression tests + no regressions).
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only listed files changed.

### Feature Validation

- [ ] `generate.go` CommitStaged: ResolveRoleModel("message") before loop; Render uses msgModel/msgReasoning;
      Result.Model uses msgModel (+ DefaultModel fallback).
- [ ] `stagecoach.go` runPipeline: same; the shared `model` var updated (both Result returns inherit it).
- [ ] Neither site uses `msgProv` (discarded with `_`).
- [ ] New tests prove message override (model+reasoning) reaches Render AND Result.Model; no-override case
      is unchanged (regression-safe).

### Code Quality Validation

- [ ] Mirrors the decompose `message.go:103` reference pattern; uses the existing stubtest harness + a
      minimal ARGSFILE knob consistent with the stub's env design.
- [ ] DefaultModel fallback preserved on Result.Model; stdin-drain-before-ARGSFILE-write honored in stub.
- [ ] No scope creep into P1.M2.T2.S1 (provider→manifest selection) or the config layer.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs edits (DOCS: none — the flags were already documented; this makes them truthful).
- [ ] go.mod/go.sum byte-unchanged; no new files outside cmd/stubagent's existing role.

---

## Anti-Patterns to Avoid

- ❌ Don't write `msgProv, msgModel, msgReasoning := …` and leave msgProv unused — Go won't compile
  ("declared and not used"). Discard the provider with `_` in both sites; provider→manifest selection is
  P1.M2.T2.S1 (buildDeps), not this task.
- ❌ Don't drop the `if model == "" { model = *resolved.DefaultModel }` fallback on Result.Model. Render
  takes the raw msgModel (may be ""); Result.Model must report the CONCRETE model. resolved is in scope.
- ❌ Don't edit stagecoach.go's two Result.Model returns (L529/L566) separately — they reference the shared
  `model` var; change the var's initializer (one line) and both inherit it. Editing them separately risks
  divergence.
- ❌ Don't add config-layer changes. The loaders/flags already populate cfg.Roles["message"] correctly
  (the bug report confirmed this); the ONLY defect is the Render call site reading the globals.
- ❌ Don't wire msgProv into buildDeps/manifest selection here — that is P1.M2.T2.S1 and would overlap.
- ❌ Don't special-case "no message override". ResolveRoleModel("message", cfg) ALREADY returns the globals
  in that case — that IS the back-compatible fallback. Adding an `if cfg.Roles["message"] == …` branch
  would duplicate the resolution logic and risk divergence.
- ❌ Don't write the stub ARGSFILE before draining stdin — the executor pipes the payload via a bounded
  ~64 KiB pipe; writing first (or sleeping) can deadlock parent+child. Write ARGSFILE after the existing
  `io.Copy(io.Discard, os.Stdin)`, near the MARKER write.
- ❌ Don't space-join os.Args in the args file — a flag value with a space would corrupt the parse. Use
  NUL ("\x00") and split on the same in tests.
- ❌ Don't edit render.go/manifest.go, default_action.go, decompose/message.go, README, or docs — those are
  out of scope (Render already takes reasoning; the decompose path is already correct; docs already list
  the flags).
- ❌ Don't change go.mod/go.sum. This is a pure in-place bugfix + a tiny test-infra knob; no new dependency.
- ❌ Don't skip `go vet`/`go build`/`go test -race ./...` — they catch the msgProv unused-var, a malformed
  stub edit, and any regression in the no-override path.
