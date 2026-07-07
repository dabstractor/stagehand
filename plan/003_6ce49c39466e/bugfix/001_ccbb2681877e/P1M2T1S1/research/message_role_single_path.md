# Research: fix single-commit path to use ResolveRoleModel("message") (bugfix Issue 2, P1.M2.T1.S1)

Verified against the live codebase. Source of truth for the two-call-site fix + the test approach.

## The bug (Issue 2)

The single-commit path renders the **global** `cfg.Model`/`cfg.Reasoning` instead of the resolved
**message** role, so `--message-model` / `--message-reasoning` / `[role.message]` are silently dropped
(PRD §9.15 FR-R3 violation). The loaders DO populate `cfg.Roles["message"]` (load.go setRoleModel/
setRoleReasoning) and the flags ARE registered (root.go); the bug is purely the Render call site.

## The two call sites (exact)

- `internal/generate/generate.go:196` — inside `CommitStaged`'s dedupe loop:
  `spec, rerr := deps.Manifest.Render(cfg.Model, sysPrompt, payload, cfg.Reasoning)`
  - Result.Model derived at step 10 (~L287-289): `model := cfg.Model; if model == "" { model = *resolved.DefaultModel }`.
- `pkg/stagecoach/stagecoach.go:467` — inside `runPipeline`'s loop (identical call).
  - `model` local var at ~L447-449: `model := cfg.Model; if model == "" { model = *resolved.DefaultModel }`,
    used by BOTH Result returns (dryRun ~L529 `Model: model,` and commit ~L566 `Model: model,`).

## The reference pattern (already correct on the decompose path)

`internal/decompose/message.go:103`: `_, mdl, rsn := config.ResolveRoleModel("message", deps.Config)`
then `deps.Roles.Message.Render(mdl, sysPrompt, payload, rsn, provider.RenderBare)`. Mirror this.

`config.ResolveRoleModel(role, cfg)` (`roles.go:41`) returns `(provider, model, reasoning string)`:
per-role `[role.<role>]` → global `[defaults]` → shipped default (planner=high). With NO message
override it returns `(cfg.Provider, cfg.Model, cfg.Reasoning)` — **back-compatible** (the common case
is unchanged; only an explicit message override now takes effect).

## ⚠️ GOTCHA: contract (B)'s `msgProv` is an unused-variable COMPILE ERROR

Contract (B) writes `msgProv, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)` in
runPipeline, then says "The resolved provider (msgProv) is handled separately by P1.M2.T2.S1 (buildDeps
manifest selection)." But msgProv is NOT used in runPipeline this task ⇒ Go **"declared and not used"**
compile error. FIX: discard the provider with `_` in BOTH sites:
`_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)`. (generate.go's contract (A)
already uses `_`.) Provider-based manifest selection is P1.M2.T2.S1 — out of scope here. The manifest
reaching runPipeline is `deps.Manifest`, selected upstream by buildDeps (unchanged).

## Result.Model propagation

- generate.go: change step-10 `model := cfg.Model` → `model := msgModel` (keep the
  `if model == "" { model = *resolved.DefaultModel }` fallback). `resolved` is in scope (computed before
  the loop).
- stagecoach.go: change the `model` var (~L447) `model := cfg.Model` → `model := msgModel` (keep the
  DefaultModel fallback). The two Result returns (L529, L566) already reference `model`, so they need NO
  separate edit — they pick up the new value automatically.

## Imports — NONE to add

Both files already import `github.com/dustin/stagecoach/internal/config` (generate.go:14,
stagecoach.go via its existing config import). `cfg` is `config.Config` in both signatures.

## Test observability — the constraint + the faithful approach

`provider.Manifest` is a CONCRETE STRUCT (`Deps.Manifest provider.Manifest`), and `Render` is a value
receiver — so a per-instance "recording Render" is impossible (can't substitute a mock type via Deps).
`cmd/stubagent` IGNORES its argv (drains stdin, reads STAGECOACH_STUB_* env, writes canned stdout) — so
the rendered `--model`/reasoning tokens are NOT observable through the stub today.

Therefore the faithful end-to-end observation of "Render received model=haiku, reasoning=high" requires a
TINY addition to the stub: a `STAGECOACH_STUB_ARGSFILE` env knob that writes `os.Args` to a file (3 lines
in cmd/stubagent, mirrors the existing STAGECOACH_STUB_MARKER file-write pattern at stubagent main.go:~40).
Then a test reads that file and asserts the rendered argv contains `--model haiku` + the reasoning token.
This is the realization of the contract's "stub Manifest whose Render records (model, reasoning)".

Minimum (no stub change): observe MODEL via `Result.Model` (deterministic, regression-catching — fails
before the fix because cfg.Model="" → manifest DefaultModel; passes after because msgModel="haiku").
Reasoning-through-CommitStaged is then proven by code symmetry (the SAME ResolveRoleModel line feeds both
`Render(msgModel, …, msgReasoning)`) + a direct `manifest.Render(...)` → `spec.Args` assertion (the
realagent_test.go:83 pattern: `args := strings.Join(spec.Args, " ")`).

Recommended: do the STAGECOACH_STUB_ARGSFILE knob — it gives a genuine end-to-end regression test for BOTH
model and reasoning (the whole point of the bug) and matches the stub's env-knob design.

## Regression safety

- No message override (`cfg.Roles` empty/nil): ResolveRoleModel("message", cfg) → (cfg.Provider,
  cfg.Model, cfg.Reasoning). Render gets the SAME values as before. Result.Model = cfg.Model (or manifest
  default). Byte-identical to current behavior. Existing tests (e.g. generate_test.go:428
  cfg.Model="openrouter/gpt-5.4") stay green.

## Scope boundary (do NOT touch)

- P1.M2.T2.S1 owns provider→manifest selection in buildDeps (the msgProv consumer). Discard it here.
- P1.M1.T1.S2 (parallel) populates pi ReasoningLevels in internal/provider/builtin.go — independent file.
- default_action.go runGenerate / GenerateCommit Options.Model (stagecoach.go:230 cfg.Model=opts.Model):
  NO change — the public-API model sets the global, which ResolveRoleModel("message") inherits when no
  per-role override exists (back-compatible).
