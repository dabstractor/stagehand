# S2 Touchpoint & Test-Rework Map (P1.M2.T1.S2)

> Backs the `decompose/roles.go` FR-R5b rework + reasoning wiring. Read alongside
> `../../architecture/scout_config_model.md` §(c) and `../../architecture/scout_render_callsites.md` §A.

## 1. The 4 existing FR-R5b tests are tied to the DEAD DefaultProvider guard — they MUST be reworked

`internal/decompose/roles_test.go` has 4 tests asserting the OLD `inferenceProvider(m)` guard (which
read `*m.Resolve().DefaultProvider`, REMOVED in P1.M1.T1.S1). They currently assert error fragments
that no longer exist:

| Test (line) | Current (DEAD) assertion | S2 rework |
|---|---|---|
| `TestResolveRoles_FR5b_BareModelOnPi` (252) | err contains `"no inference provider"` + `"[provider.pi] default_provider"` | model=`"glm-5-turbo"` (no `/`) on pi (auto-detected, multi-provider) → err contains `"must be inference/model"`. Role-named. |
| `TestResolveRoles_FR5b_BareModelOnClaude_NoError` (276) | claude + `"haiku"` → no error | **STILL VALID as-is** — claude `ProviderFlag==""` ⇒ `isMultiProvider` false ⇒ no guard fires. Keep; minor comment refresh only. |
| `TestResolveRoles_FR5b_ProviderSet_NoInferenceProvider` (309) | err contains `"no inference provider"` | model=`"gpt-5.4"` (no `/`) on explicit provider `"pi"` → err contains `"must be inference/model"`. |
| `TestResolveRoles_FR5b_InferenceProviderSet_NoError` (337) | uses DEAD `withInferenceProvider("zai")` helper (sets DefaultProvider) | rework: model=`"zai/gpt-5.4"` (HAS `/`) on pi, plain `goRegistry(t,[]string{"pi"},nil)` → no error. **Remove the `withInferenceProvider` helper** (DefaultProvider is gone). |

ADD: a positive FR-R5b slash-prefix test (model=`"zai/glm-5.2"` on pi → no error) if not already
covered by reworking #4. The model-prefix is now THE field — there is no `default_provider` to set.

`grep -n 'withInferenceProvider\|"no inference provider"\|default_provider' internal/decompose/roles_test.go`
MUST return nothing after S2 (all dead references removed).

## 2. The guard runs on the FINAL (post-stager-fallback) (manifest, model) pair — verify the stager-fallback tests

S2 places the FR-R5b guard AFTER the FR-D4 stager fallback (same position as the current TODO marker),
so it validates the FINAL resolved pair. **Edge case:** if a stager fallback lands on a multi-provider
(pi) and the fallback model (`config.DefaultModelsForProvider(fb)["stager"]`) lacks a `/`, the guard
FIRES. The existing stager-fallback tests (`TestResolveRoles_StagerFallback` 143,
`TestResolveRoles_StagerFallback_PiNotInstalled_FallsToClaude` 193) fall back to **claude**
(single-backend ⇒ `isMultiProvider` false ⇒ no guard), so they are unaffected — but S2 MUST run them
to confirm. If any stager-fallback fixture lands on pi with a bare model, that fixture (or
`internal/config/role_defaults.go` — OUT OF SCOPE for S2; do not edit) needs a slash-prefix model.
The guard is correct either way: a bare model on a multi-provider is ALWAYS a config error (FR-R5b).

## 3. Guard semantics (matches Render's chokepoint exactly)

PRD §12.2: Render fires when `*r.ProviderFlag != "" && modelToUse != "" && !contains(modelToUse,"/")`.
`ResolveRoles` mirrors it on the USER-resolved `mdl`: `if isMultiProvider(m) && mdl != "" &&
!strings.Contains(mdl, "/")`. When `mdl == ""` the model is NOT user-pinned → no guard (Render uses
manifest DefaultModel and its OWN guard handles that). Error message (role-named, mirrors Render's):
```
fmt.Errorf("role %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\"", role, mdl, m.Name)
```
`isMultiProvider(m)` STAYS (checks `m.ProviderFlag != nil && *m.ProviderFlag != ""`); `inferenceProvider`
is NOT re-added (it was DefaultProvider-based and is dead). Add `"strings"` import to roles.go.

## 4. Reasoning wiring — the 4 Render callers RE-DERIVE (consistent with how they re-derive model)

Each decompose caller already does `_, mdl := config.ResolveRoleModel("<role>", deps.Config)` (post-S1:
`_, mdl, _`) because `Deps` carries `RoleManifests` but NOT `RoleModels` (the orchestrator retains
RoleModels locally). S2 follows the SAME pattern for reasoning: `_, mdl, rsn := …` then
`Render(mdl, sysPrompt, payload, rsn, mode)`. Do NOT thread RoleModels into Deps (that is orchestrator
scope, P3.M4). ResolveRoles separately populates `RoleModels.X.Reasoning` (via `setRole`) so the
2nd return carries the full triple — consumed by `mapDecomposeResult`/future work. Both paths
(ResolveRoles + per-call re-derive) call `config.ResolveRoleModel` with the SAME cfg ⇒ identical
reasoning ⇒ no divergence.

## 5. Single-commit path: reasoning comes from cfg.Reasoning (Options has NO Reasoning field)

`pkg/stagecoach.Options` (Provider/Model/SystemExtra/DryRun/Timeout/Verbose/VerboseOn/Config) has NO
`Reasoning` field — and the item does NOT ask S2 to add one (only `RoleModel` gets Reasoning). So the
single-commit path's reasoning = `cfg.Reasoning` from `config.Load` (S1's file/env/flag plumbing) →
`generate.go`/`runPipeline` pass `cfg.Reasoning` as Render's 4th arg (S1's Task 6). **Item (e) is
VERIFY-only:** ensure those two sites are NOT still the literal `""` (S1 may have left them mid-flight
— the repo currently has a transient `root.go` redeclaration error from S1). If a site is still `""`,
flip it to `cfg.Reasoning` (one-liner; keeps the build green). Do NOT add Reasoning to Options.

## 6. applyRoleOverride + the per-role gate must gain Reasoning

`resolveDecomposeConfig`'s per-role field-merge gate (stagecoach.go ~261) currently checks only
`Provider`/`Model` for planner/stager/arbiter. S2 adds `|| opts.<X>.Reasoning != ""` so a
reasoning-only `RoleModel{Reasoning:"high"}` triggers the merge. `applyRoleOverride` gains a
`if rm.Reasoning != "" { rc.Reasoning = rm.Reasoning }` branch + the early-return guard becomes
`if rm.Provider == "" && rm.Model == "" && rm.Reasoning == "" { return }`. (Message has NO RoleModel
field — its reasoning is the global `cfg.Reasoning`; this is the existing DecomposeOptions design and
is out of scope.)
