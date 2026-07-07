# Render Call-Site Census — signature change planning

Planned change: drop the separate `provider` param from `Manifest.Render` (it folds into a slash-prefix on `model`), and add a new `reasoning` param.

Current signature (`internal/provider/render.go:124`):
```go
func (m Manifest) Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode) (*CmdSpec, error)
```

---

## A. Non-test `Manifest.Render(` call sites (5 total)

| # | file:line | model | provider | sysPrompt | userPayload | mode |
|---|-----------|-------|----------|-----------|-------------|------|
| 1 | `internal/generate/generate.go:196` | `cfg.Model` (`config.Config.Model`, string; may be `""` → fallback to `*resolved.DefaultModel` via `Resolve()` at generate.go:187/`resolved`) | `""` (literal; FR37a — Render resolves sub-provider from manifest `DefaultProvider`) | `sysPrompt` (built once by `buildSystemPrompt(...)`, generate.go:171) | `payload` (`prompt.BuildUserPayload(diff, rejected)` mutated each loop attempt; `parseFail` prepends `retryInstr+"\n\n"`) | omitted (variadic → `RenderBare`) |
| 2 | `pkg/stagecoach/stagecoach.go:461` | `cfg.Model` | `""` (literal) | `sysPrompt` (built via `runPipeline`-local `buildSystemPrompt`) | `payload` (same builder + retryInstruction prepend) | omitted (→ `RenderBare`) |
| 3 | `internal/decompose/planner.go:98` | `mdl` (`config.ResolveRoleModel("planner", deps.Config)` → model, planner.go:62) | `""` (literal) | `sysPrompt` (`prompt.BuildPlannerSystemPrompt(examples)`, planner.go:95) | `payload` (`basePayload` = `prompt.BuildPlannerUserPayload(diff, forcedCount)`; retry prepends `prompt.PlannerRetryInstruction`) | `provider.RenderBare` |
| 4 | `internal/decompose/message.go:129` | `mdl` (`config.ResolveRoleModel("message", deps.Config)` → model, message.go:103) | `""` (literal) | `sysPrompt` (`messageSystemPrompt(...)`, message.go:91) | `payload` (`prompt.BuildUserPayload(diff, rejected)` + retryInstruction prepend) | `provider.RenderBare` |
| 5 | `internal/decompose/arbiter.go:97` | `mdl` (`config.ResolveRoleModel("arbiter", deps.Config)` → model, arbiter.go:82) | `""` (literal) | `sysPrompt` (`prompt.BuildArbiterSystemPrompt()`, arbiter.go:88) | `payload` (`prompt.BuildArbiterUserPayload(arbiterCommits, leftoverDiff)`, arbiter.go:89) | `provider.RenderBare` |
| 6 | `internal/decompose/stager.go:86` | `mdl` (`config.ResolveRoleModel("stager", deps.Config)` → model, stager.go:78) | `""` (literal) | `""` (literal — empty system prompt; the task IS the payload) | `task` (`prompt.BuildStagerTask(concept.Title, concept.Description)`, stager.go:81) | `provider.RenderTooled` (the ONLY tooled call site) |

Note: in every non-test call site the `provider` argument is the literal `""`. The actual sub-provider is always resolved inside Render from `m.DefaultProvider` after `Resolve()`. So the `provider` param is effectively dead at call sites — confirming the change is low-risk.

### `ResolveRoleModel` — supplies `mdl` for call sites #3–#6
`internal/config/roles.go:28`
```go
func ResolveRoleModel(role string, cfg Config) (provider, model string)
```
Returns `(provider, model)`; the `provider` return is intentionally DISCARDED (`_,`) at every decompose call site — only `model` (`mdl`) is passed to Render. This makes removing the Render `provider` param trivial for these sites: nothing currently uses the provider half.

---

## B. Executor that consumes `CmdSpec` (1)

`internal/provider/executor.go` — `func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout string, stderr string, err error)`

Consumers of `Execute(ctx, *spec, <timeout>, <verbose>)`:
- `internal/generate/generate.go:201` — timeout `cfg.Timeout`, verbose `deps.Verbose`
- `pkg/stagecoach/stagecoach.go:466` — timeout `cfg.Timeout`, verbose `deps.Verbose`
- `internal/decompose/planner.go:103` — timeout `deps.Config.Timeout`, verbose `deps.Verbose`
- `internal/decompose/message.go:134` — timeout `deps.Config.Timeout`, verbose `deps.Verbose`
- `internal/decompose/arbiter.go:103` — timeout `deps.Config.Timeout`, verbose `deps.Verbose`
- `internal/decompose/stager.go:92` — timeout `deps.Config.Timeout`, verbose `deps.Verbose`

`CmdSpec` is pure data (render.go:10–25): `Command string; Args []string; Stdin string; Env []string`. Render is the ONLY producer; Execute is the ONLY non-test consumer. Test consumer: `internal/stubtest/stubtest_test.go` (executes via the real `provider.Execute` seam). The planned change is internal to Render — `CmdSpec` is unaffected, so Execute needs NO changes.

---

## C. `generate.CommitStaged` signature + how it calls Render

`internal/generate/generate.go:135`
```go
func CommitStaged(ctx context.Context, deps Deps, cfg config.Config) (Result, error)
```
- `Deps` (generate.go:26): `{ Git git.Git; Manifest provider.Manifest; Verbose *ui.Verbose }`
- Calls Render once per loop iteration inside the generation+dedupe loop (generate.go:188–203), passing `(cfg.Model, "", sysPrompt, payload)` with NO mode (→ `RenderBare`). `cfg.Model` may be `""`; Render falls back to `*resolved.DefaultModel`. `resolved := deps.Manifest.Resolve()` at generate.go:187.
- After success, the model reported in `Result.Model` (generate.go:290–292) re-applies the same `cfg.Model`/`DefaultModel` fallback.
- Delegated to by `pkg/stagecoach.GenerateCommit` (stagecoach.go:141) for the common (non-dry-run, non-SystemExtra) path.

---

## D. References to `DefaultProvider` / `DefaultModel` / `ProviderFlag` in NON-TEST source

### `DefaultProvider` field usage (non-test)
- `internal/provider/manifest.go:62` — field declaration `DefaultProvider *string`
- `internal/provider/manifest.go` (Resolve) — nil → `strPtr("")` default
- `internal/provider/render.go:113-116` — `providerToUse = *r.DefaultProvider` when provider param == `""`
- `internal/provider/render.go:131` — used in the FR-R5b backstop guard (`*r.ProviderFlag != "" && modelToUse != "" && providerToUse == ""` → error)
- `internal/provider/render.go:139` — emitted as `(*r.ProviderFlag, providerToUse)` when both set
- `internal/decompose/roles.go:188-190` — `mergedDefaultProvider(m)` returns `*m.Resolve().DefaultProvider` (used by `ResolveRoles` FR-R5b guard)
- `internal/generate/generate.go:194-195` — comment only (Render resolves from DefaultProvider)
- `internal/decompose/arbiter.go:93-94`, `message.go:125-126`, `planner.go:94-95`, `stager.go:77-78` — comments only
- `internal/config/bootstrap.go:67` — `DefaultModelsForProvider(name)` (DIFFERENT function — per-provider role table; not the field)

### `DefaultModel` field usage (non-test)
- `internal/provider/manifest.go:54` — field declaration `DefaultModel *string`
- `internal/provider/manifest.go` (Resolve) — nil → `strPtr("")` default
- `internal/provider/render.go:104-107` — `modelToUse = *r.DefaultModel` when model param == `""`
- `internal/generate/generate.go:290-291` — `model = *resolved.DefaultModel` when `cfg.Model == ""` (for `Result.Model`)
- `pkg/stagecoach/stagecoach.go:443` — same fallback in `runPipeline` (`model = *resolved.DefaultModel`); NOTE: this local `model` var is used later but the Render call at :461 still passes raw `cfg.Model` (a minor inconsistency worth noting — Render re-derives from DefaultModel internally, so behavior is identical)

### `ProviderFlag` field usage (non-test)
- `internal/provider/manifest.go:60` — field declaration `ProviderFlag *string`
- `internal/provider/manifest.go` (Resolve) — nil → `strPtr("")` default
- `internal/provider/render.go:131` — `*r.ProviderFlag != ""` (gate for emitting `--provider <p>` AND for the FR-R5b backstop)
- `internal/provider/render.go:139` — emit `(*r.ProviderFlag, providerToUse)`
- `internal/decompose/roles.go:178-182` — `isMultiProvider(m)`: `return m.ProviderFlag != nil && *m.ProviderFlag != ""` (classifies pi vs single-backend agents; used by stager fallback)
- `internal/decompose/roles.go:25` (comment), `:79`, `:86`, `:138-139` — comments / doc

### Method-level `Registry.DefaultProvider(installed)` (DIFFERENT — a registry auto-detect, NOT the field)
- `internal/provider/registry.go:102-118` — `func (r *Registry) DefaultProvider(installed []string) string`
- `internal/provider/registry.go:13` (doc)
- Callers (non-test): `pkg/stagecoach/stagecoach.go:326`, `internal/config/bootstrap.go:25`, `internal/cmd/providers.go:142`, `internal/decompose/roles.go:99`, `internal/config/roles.go:20` (doc)

---

## E. Test `Manifest.Render(` call sites — COUNT and files

Total test call sites: **41** across 4 files (will need mechanical updating if the Render signature changes arity/positions):

| file | approx count | signature used |
|------|--------------|----------------|
| `internal/provider/render_test.go` | ~22 | mixed: 4-arg `Render(model, provider, sys, user)` and 5-arg `Render(..., RenderMode)`; exercises provider default fallback, FR-R5b, mode ternary |
| `internal/provider/builtin_test.go` | 2 | 5-arg `Render("model", "provider", "<sys>", "<user>", RenderTooled)` — tooled-mode assertions for pi + claude |
| `internal/generate/realagent_test.go` | 1 | 4-arg `m.Render(cfg.Model, "", "<system prompt>", "<staged diff>")` (line 78) — real-agent end-to-end |
| `internal/stubtest/stubtest_test.go` | ~10 | 4-arg `m.Render("", "", "", "...")` — stub manifest exercises the Execute seam |

Plus `internal/generate/invariants_test.go` exercises `CommitStaged(...)` (which calls Render internally) ~6 times — not direct Render calls.

---

## F. Implications for the planned signature change

New signature (proposed): `func (m Manifest) Render(model, sysPrompt, userPayload string, reasoning <T>, mode ...RenderMode) (*CmdSpec, error)` — where `model` carries any `provider/model` slash-prefix and `reasoning` is the new param.

Required edits:
1. **`internal/provider/render.go:124`** — signature; remove `provider` param, add `reasoning`. Inside the body:
   - `modelToUse` logic stays (lines 104-107).
   - Replace `providerToUse` derivation (lines 109-116): parse the sub-provider from a `/`-split of `model` (fall back to `*r.DefaultProvider` when no slash).
   - FR-R5b backstop (lines 120-129) and the `--provider` emit (lines 137-139): now driven by the parsed slash-provider + `*r.ProviderFlag`.
   - Add reasoning-flag handling (wherever the manifest carries reasoning config — likely a new field/flag; check whether a `reasoning`/`reasoning_flag` field exists on `Manifest` — currently it does NOT; this is a new addition).
2. **5 non-test call sites (Section A)** — all pass literal `""` for provider today, so the call becomes `Render(mdl, sysPrompt, payload, reasoning, mode...)`. The slash-prefix fold means call sites that want to pin a sub-provider must build `"provider/model"` strings; today NONE do (all rely on `DefaultProvider`), so the fold is transparent for current callers. The only thing each call site must add is the new `reasoning` argument.
   - Need to decide where `reasoning` is sourced at each call site: most likely from `cfg` / `deps.Config` (a new `Config.Reasoning` field) or from the role manifest. This is the main design question.
3. **`Execute` / `CmdSpec`** — no changes (pure data contract; reasoning, if it becomes a CLI flag, gets folded into `CmdSpec.Args` by Render).
4. **~41 test call sites** — mechanical arity fix: insert the new `reasoning` arg and drop the provider arg. The 4-arg v1 calls (`Render(model, "", sys, user)`) become 4-arg `Render(model, sys, user, reasoning)` — same arity, different positions, so tests will compile-break clearly and need each arg list reordered.
5. **Field additions** (likely): `Manifest` probably needs a `ReasoningFlag *string` (and maybe a default) so Render knows the flag spelling; `Resolve()` must default it. Check whether slash-prefix parsing for `model` conflicts with opencode/agy combined-form providers (role_defaults.go:66 notes `"openai/gpt-5.4"` is already provider-prefixed for opencode whose `ProviderFlag` is empty) — the parser must treat the slash as the sub-provider separator ONLY when `*r.ProviderFlag != ""`.

Open question for the implementer: does `reasoning` come from config (per-invocation) or from the manifest (per-provider default)? The census shows `model` is sourced from `cfg.Model` (v1 path) and `ResolveRoleModel(role, cfg).Model` (decompose path) — a parallel `cfg.Reasoning` / role-level reasoning is the consistent choice.
