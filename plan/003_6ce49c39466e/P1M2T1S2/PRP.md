---
name: "P1.M2.T1.S2 — ResolveRoles v3: model-prefix FR-R5b guard + RoleModels.Reasoning + 4 decompose callers + pkg/stagecoach RoleModel"
description: |
  Wire the per-role `reasoning` level (landed in config by P1.M2.T1.S1's 3-return `ResolveRoleModel`) into the
  multi-commit decompose pipeline and the public `pkg/stagecoach` API, AND re-add the FR-R5b guard in
  `ResolveRoles` using the new model-prefix semantics (the old `inferenceProvider`/`DefaultProvider` guard was
  REMOVED in P1.M1.T1.S1 and MUST be replaced). PRD spec: §9.15 FR-R5b/FR-R6, §12.2 (Render chokepoint), §16.4
  (per-role), §14.1 (pkg/stagecoach). Research: `architecture/scout_config_model.md` §(c), `scout_render_callsites.md`
  §A, and `research/s2_touchpoint_map.md` (the authoritative test-rework + wiring map).

  ⚠️ **THE central design call — the FR-R5b guard is now a MODEL-PREFIX check (not a DefaultProvider check).**
  `decompose/roles.go` previously had `inferenceProvider(m)` reading `*m.Resolve().DefaultProvider`; that field
  was REMOVED in P1.M1.T1.S1 (the inference backend now rides the model slash-prefix, e.g. `zai/glm-5.2`).
  S2 re-adds the guard as: `if isMultiProvider(m) && mdl != "" && !strings.Contains(mdl, "/")` → role-named
  error `model %q on %s must be inference/model, e.g. "zai/glm-5.2"`. `isMultiProvider(m)` STAYS (it tests
  `m.ProviderFlag != nil && *m.ProviderFlag != ""`, classifying pi vs single-backend); `inferenceProvider` is
  NOT re-added (dead). The guard mirrors Render's chokepoint (§12.2) exactly but fires EARLIER with a role
  name. Enforcement is therefore at BOTH layers: ResolveRoles (role-named, early) + Render (authoritative
  chokepoint, all paths). See `research/s2_touchpoint_map.md` §3.

  ⚠️ **THE second design call — reasoning threads TWO parallel paths (ResolveRoles + the 4 per-role callers),
  both via `config.ResolveRoleModel`.** (a) `ResolveRoles` captures the 3rd return (`rsn`) and stores it in
  `RoleModels.X.Reasoning` via `setRole` (so the 2nd return carries the full provider/model/reasoning triple —
  consumed by `mapDecomposeResult`/future work). (b) The 4 decompose Render callers (planner/message/arbiter/
  stager .go) ALSO re-derive `rsn` via `ResolveRoleModel("<role>", deps.Config)` and pass it as Render's 4th
  arg — consistent with how they ALREADY re-derive `mdl` (Deps carries RoleManifests, NOT RoleModels; the
  orchestrator retains RoleModels locally). Both call `ResolveRoleModel` with the SAME cfg ⇒ identical
  reasoning ⇒ no divergence. Do NOT thread RoleModels into Deps (orchestrator scope, P3.M4).

  ⚠️ **THE third design call — the 4 existing FR-R5b tests are tied to the DEAD DefaultProvider guard and MUST
  be reworked to model-prefix semantics.** `roles_test.go` has `TestResolveRoles_FR5b_*` (lines 252/309/337)
  asserting old fragments (`"no inference provider"`, `"[provider.pi] default_provider"`) and using the DEAD
  `withInferenceProvider("zai")` helper (sets DefaultProvider). S2 reworks them: bare-model-on-pi cases expect
  `"must be inference/model"`; the correct-config case uses a slash-prefix model (`"zai/gpt-5.4"`), no helper.
  `BareModelOnClaude_NoError` (276) STAYS as-is (claude is single-backend ⇒ no guard). Remove `withInferenceProvider`.
  `grep -n 'withInferenceProvider\|"no inference provider"\|default_provider' roles_test.go` MUST be empty after.
  See `research/s2_touchpoint_map.md` §1.

  ⚠️ **THE fourth design call — pkg/stagecoach: add `Reasoning` to `RoleModel` ONLY (NOT `Options`); thread
  through `applyRoleOverride` + the per-role gate.** `RoleModel{Provider,Model}` → `+ Reasoning string`
  (additive, zero-value ⇒ off, backward compatible — §14.1 "additive-only"). `applyRoleOverride` gains a
  `if rm.Reasoning != "" { rc.Reasoning = rm.Reasoning }` branch + early-return guard adds `&& rm.Reasoning == ""`.
  `resolveDecomposeConfig`'s per-role gate (stagecoach.go ~261) adds `|| opts.<X>.Reasoning != ""` so a
  reasoning-only override triggers the merge. `Options` has NO Reasoning field and the item does NOT add one —
  the single-commit path's reasoning = `cfg.Reasoning` from config.Load (S1's plumbing). Message has no RoleModel
  field (existing DecomposeOptions design; message reasoning = global cfg.Reasoning) — out of scope.

  ⚠️ **THE fifth design call — item (e) "verify the generate single-commit path passes global reasoning" is
  VERIFY-only.** `generate.go` CommitStaged + `pkg/stagecoach` runPipeline must pass `cfg.Reasoning` (NOT the
  literal `""`) as Render's 4th arg — that change is S1's Task 6. S2 VERIFIES both sites; if either is still
  the literal `""` (S1 mid-flight — the repo currently has a transient `root.go` redeclaration error from S1),
  flip it to `cfg.Reasoning` (one-liner; build stays green). Do NOT add Reasoning to Options.

  Deliverable (edits to existing files only — NO new files, NO new deps, NO go.mod change): `internal/decompose/
  {roles,planner,message,arbiter,stager}.go`, `internal/decompose/roles_test.go`, `pkg/stagecoach/stagecoach.go`,
  and (verify-only) `internal/generate/generate.go`. INPUT = S1's 3-return `ResolveRoleModel` + `RoleConfig.Reasoning`
  + `defaultRoleReasoning` + the single-commit `cfg.Reasoning` Render arg (S1) + P1.M1.T1.S1's Render (4th arg
  reasoning) + the model-prefix chokepoint guard. OUTPUT = every decompose Render call passes a real resolved
  reasoning; FR-R5b enforced at BOTH ResolveRoles (role-named) and Render (chokepoint); `pkg/stagecoach` RoleModel
  carries reasoning end-to-end. DOCS = Mode A (inline doc comments on ResolveRoles/RoleModel/DecomposeOptions ride
  with the code; NO docs/*.md edits — changeset doc sync is P4.M2.T1).

  SCOPE BOUNDARY vs neighbors: S1 = config reasoning field + ResolveRoleModel 3-return + single-commit Render
  reasoning + root.go flags + roles_test.go(config). P1.M3 = config version bump/migration. P2 = qwen-code. P3.M4
  = decompose orchestrator (Deps threading). S2 = ResolveRoles guard rework + RoleModels.Reasoning + the 4
  decompose Render reasoning args + pkg/stagecoach RoleModel.Reasoning. Do NOT touch root.go (S1 mid-flight), do
  NOT bump CurrentConfigVersion (P1.M3.T1.S1), do NOT edit render.go/manifest.go/role_defaults.go, do NOT edit
  docs/*.md.
---

## Goal

**Feature Goal**: Complete the v3 per-role reasoning + model-prefix story on the DECOMPOSE + public-LIBRARY
side: (1) re-add the FR-R5b guard in `decompose.ResolveRoles` using the model-prefix semantics (replacing the
removed DefaultProvider guard); (2) thread resolved per-role reasoning into the four decompose `Render` call
sites and into `RoleModels` (via `setRole`); (3) add `Reasoning` to the public `pkg/stagecoach.RoleModel` and
thread it through `applyRoleOverride` + the per-role merge gate; (4) verify the single-commit path forwards
`cfg.Reasoning` to Render. FR-R5b is then enforced at two layers (ResolveRoles role-named + Render chokepoint),
and every Render call site passes a real resolved reasoning value.

**Deliverable** (edits to existing files only):
1. **`internal/decompose/roles.go`** — (a) capture `rsn` from 3-return `ResolveRoleModel` in `ResolveRoles`;
   (b) re-add the FR-R5b model-prefix guard after the stager-fallback block (replace the `TODO(P1.M2.T1.S2)`
   marker): `isMultiProvider(m) && mdl != "" && !strings.Contains(mdl, "/")` → role-named error; (c) `setRole`
   gains an `rsn string` param and populates `config.RoleConfig{Provider, Model, Reasoning}`; (d) add `"strings"`
   import; (e) Mode A doc comments on the guard + `RoleModels` reasoning.
2. **`internal/decompose/{planner,message,arbiter,stager}.go`** — each: `_, mdl, _ := ResolveRoleModel(...)`
   → `_, mdl, rsn := …`; the `Render(mdl, sysPrompt, payload, "", mode)` call → `…, rsn, mode)`; refresh the
   "v3 FR-R5b … P1.M2 wires real per-role reasoning" comment block (the TODO is now resolved).
3. **`pkg/stagecoach/stagecoach.go`** — (a) `RoleModel` gains `Reasoning string` (+ Mode A doc); (b)
   `applyRoleOverride` gains the Reasoning field-merge branch + updated early-return guard; (c)
   `resolveDecomposeConfig`'s per-role gate adds `|| opts.<X>.Reasoning != ""`; (d) Mode A doc refresh on
   `DecomposeOptions`.
4. **`internal/generate/generate.go`** + **`pkg/stagecoach/stagecoach.go` (runPipeline)** — VERIFY the
   single-commit `Render(...)` 4th arg is `cfg.Reasoning` (not `""`); flip if still literal `""`.
5. **`internal/decompose/roles_test.go`** — rework the 3 dead FR-R5b tests to model-prefix semantics; keep
   `BareModelOnClaude_NoError`; remove `withInferenceProvider`; add reasoning-threading assertions
   (`rmodels.X.Reasoning`) + a positive slash-prefix case; ensure `TestIsMultiProvider` + stager-fallback tests
   stay green.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l` clean; `go test -race ./...` green; every
decompose Render call passes a resolved `rsn` (no remaining literal `""` 4th arg in decompose);
`ResolveRoles` rejects a bare model on a multi-provider with a role-named `"must be inference/model"` error and
accepts a slash-prefix model; `RoleModels.X.Reasoning` carries the resolved reasoning (planner="high" shipped
default, others ""/off via ResolveRoleModel); `pkg/stagecoach.RoleModel{Reasoning:"high"}` flows through
`applyRoleOverride` into `cfg.Roles[role].Reasoning`; the single-commit Render arg is `cfg.Reasoning`;
go.mod/go.sum byte-unchanged; no new files; no edits to root.go/render.go/manifest.go/role_defaults.go/docs.

## User Persona

**Target User**: (a) the **multi-commit decompose pipeline** — the planner/stager/message/arbiter agents now
receive a resolved reasoning level (FR-R6) per role, and a misconfigured bare-model-on-pi is caught early with
a role-named error (FR-R5b); (b) **library consumers** of `pkg/stagecoach.Decompose` who set per-role reasoning
via `DecomposeOptions{Planner: RoleModel{Reasoning:"high"}}`; (c) downstream **P3.M4 orchestrator** work that
consumes `RoleModels` + the validated manifests.

**Use Case**: `stagecoach --planner-reasoning high --planner-model zai/glm-5.2` (or
`DecomposeOptions{Planner: RoleModel{Provider:"pi", Model:"zai/glm-5.2", Reasoning:"high"}}`) routes the
planner to pi with high reasoning; a typo `--planner-model glm-5.2` (no slash on the multi-provider pi) is
rejected by `ResolveRoles` with `"role \"planner\": model \"glm-5.2\" on pi must be inference/model …"` BEFORE
any agent runs.

**User Journey**: flag/env/file (S1) → `Load()` → `ResolveRoles` (resolve+guard, THIS subtask) →
`RoleModels.X.Reasoning` + validated manifests → orchestrator → per-role `Render(mdl, sys, payload, rsn, mode)`
(THIS subtask) → reasoning tokens appended (FR-R6) / FR-R5b enforced (Render chokepoint).

**Pain Points Addressed**: closes "how does reasoning reach the decompose Render calls", "where is the FR-R5b
guard now that DefaultProvider is gone", and "can a library consumer set per-role reasoning" — for the
multi-commit path + the public API.

## Why

- **Restores FR-R5b on the decompose path.** P1.M1.T1.S1 removed `DefaultProvider` (folded into the model
  slash-prefix) and deleted `inferenceProvider`; without S2, `ResolveRoles` has NO early guard (only Render's
  chokepoint remains). The PRD (§9.15 FR-R5b, §16.4) requires a role-named early check too.
- **Makes reasoning reachable on the multi-commit path.** S1 landed the config plumbing + the single-commit
  Render arg; S2 is the decompose-side + public-API contract that makes per-role reasoning actually reach the
  four agents.
- **Completes the pkg/stagecoach v3 surface.** `RoleModel.Reasoning` is the last additive field the library API
  needs for FR-R6 per-role reasoning (§14.1 additive-only guarantee preserved).
- **Back-compatible.** Zero-value `RoleModel`/`RoleConfig` ⇒ reasoning "" ⇒ graceful no-op (FR-R6); a bare
  model on a single-backend provider is still fine (only pi-style multi-providers are guarded).

## What

A compiled decompose layer where `ResolveRoles` re-validates FR-R5b via the model prefix and carries resolved
reasoning in `RoleModels`; the four role Render calls pass that reasoning; and `pkg/stagecoach.RoleModel`
exposes reasoning end-to-end. No new types beyond the `Reasoning` field on `RoleModel`; no new files; no
dependency change.

### Success Criteria

- [ ] `internal/decompose/roles.go`: `ResolveRoles` captures `rsn` from `prov, mdl, rsn :=
      config.ResolveRoleModel(role, cfg)`; the FR-R5b guard `if isMultiProvider(m) && mdl != "" &&
      !strings.Contains(mdl, "/") { return …, fmt.Errorf("role %q: model %q on %s must be inference/model,
      e.g. \"zai/glm-5.2\"", role, mdl, m.Name) }` is present AFTER the stager-fallback block (replacing the
      `TODO(P1.M2.T1.S2)` marker); `setRole(&rm, &rmodels, role, m, prov, mdl, rsn)` is called and `setRole`
      builds `config.RoleConfig{Provider: prov, Model: mdl, Reasoning: rsn}`; `"strings"` is imported;
      `isMultiProvider` is unchanged; there is NO `inferenceProvider` function.
- [ ] The 4 decompose Render callers (`planner.go:96`, `message.go:127`, `arbiter.go:95`, `stager.go:87`)
      each capture `_, mdl, rsn := config.ResolveRoleModel("<role>", deps.Config)` and pass `rsn` as Render's
      4th arg (NO remaining literal `""` 4th arg in `internal/decompose/`).
- [ ] `pkg/stagecoach.RoleModel` has `Reasoning string` (after `Model`); `applyRoleOverride` has a
      `if rm.Reasoning != "" { rc.Reasoning = rm.Reasoning }` branch and its early-return guard is
      `if rm.Provider == "" && rm.Model == "" && rm.Reasoning == "" { return }`; `resolveDecomposeConfig`'s
      per-role gate includes `|| opts.Planner.Reasoning != "" || opts.Stager.Reasoning != "" ||
      opts.Arbiter.Reasoning != ""`.
- [ ] `generate.go` CommitStaged Render + `pkg/stagecoach` runPipeline Render pass `cfg.Reasoning` (4th arg),
      NOT `""`.
- [ ] `roles_test.go`: the 3 dead FR-R5b tests reworked to model-prefix semantics (assert `"must be
      inference/model"`); `BareModelOnClaude_NoError` kept; `withInferenceProvider` removed;
      `grep -n 'withInferenceProvider\|"no inference provider"\|default_provider' roles_test.go` empty;
      reasoning-threading assertions present (`rmodels.Planner.Reasoning == "high"` etc.); `TestIsMultiProvider`
      + the stager-fallback tests stay green.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l`, `go test -race ./...` all clean/green; go.mod/go.sum
      byte-unchanged; no edits to `internal/cmd/root.go`, `internal/provider/{render,manifest}.go`,
      `internal/config/role_defaults.go`, `docs/*.md`; no new files.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the guard predicate + error string
(quoted), the `setRole` signature change, the 4 exact call-site line numbers, the `applyRoleOverride`/gate
edits, the test-rework table, and the resolved-reasoning source (`config.ResolveRoleModel` 3rd return). No
git/generation internals beyond "Render takes reasoning as the 4th positional arg".

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/003_6ce49c39466e/P1M2T1S2/research/s2_touchpoint_map.md
  why: the AUTHORITATIVE wiring + test-rework map. §1 = the 4 FR-R5b tests (3 dead, rework table); §2 = the
       stager-fallback-on-pi edge case (guard runs on the FINAL pair); §3 = the guard predicate + error string;
       §4 = the two-path reasoning wiring (ResolveRoles + per-call re-derive); §5 = single-commit verify-only;
       §6 = applyRoleOverride + the per-role gate.
  critical: §1 + §3 — without reworking the FR-R5b tests the suite WILL fail (they assert dead fragments), and
       without the exact error string the guard won't match Render's chokepoint message.

- docfile: plan/003_6ce49c39466e/architecture/scout_config_model.md
  section: "(c) ResolveRoleModel signature + callers"
  why: the authoritative list of the 5 ResolveRoleModel callers (roles.go:96 + the 4 role files) + the note
       that the single-commit path reads cfg.Model/cfg.Reasoning DIRECTLY (not ResolveRoleModel).
  critical: §(c) confirms each decompose caller does `_, mdl := ResolveRoleModel(...)` (post-S1 `_, mdl, _`) —
       S2 flips `_` → `rsn` and passes it to Render. Do NOT change the single-commit path to use ResolveRoleModel.

- docfile: plan/003_6ce49c39466e/architecture/scout_render_callsites.md
  section: "A. Non-test Manifest.Render call sites" (#3–#6 are the decompose sites)
  why: confirms the 4 decompose Render sites pass model=`mdl`, reasoning=`""` (literal), and a mode. S2 changes
       only the reasoning positional (`""` → `rsn`); model/sys/payload/mode are unchanged.
  critical: §D confirms `ProviderFlag` is read by `isMultiProvider` (roles.go) AND by Render's FR-R5b guard —
       both keyed on the same field, so the two layers agree on "who is a multi-provider". `DefaultProvider` is
       gone (removed P1.M1.T1.S1); do NOT reference it.

- file: PRD.md
  section: "9.15 Per-role provider/model configuration" (h3.31) — esp. FR-R5b + FR-R6; "12.2 Command rendering
           algorithm" (h3.44); "16.4 Per-role provider/model configuration" (h3.69); "14.1 pkg/stagecoach" (h3.60)
  why: FR-R5b fixes the exact guard + error (`model %q on %s must be inference/model, e.g. zai/glm-5.2`) and
       the TWO-layer enforcement ("role resolution re-checks it earlier for a role-named error" + "Authoritative
       enforcement lives at Render"). FR-R6 fixes the graceful-no-op rule (reasoning never errors). §16.4 fixes
       per-role precedence. §14.1 fixes the additive-only RoleModel/DecomposeOptions guarantee.
  critical: FR-R5b "A model without a `/` on a `provider_flag` provider is a HARD configuration error … role
       resolution re-checks it earlier for a role-named error." That IS this subtask's ResolveRoles guard.

- file: internal/decompose/roles.go   (the file you EDIT — read it first)
  why: the current ResolveRoles (with the `// TODO(P1.M2.T1.S1.S2)` guard marker), `isMultiProvider`, `setRole`,
       `RoleModels` (4 × config.RoleConfig). S2 captures `rsn`, re-adds the guard at the TODO marker, and
       extends setRole.
  pattern: the stager-fallback block ends right before the TODO marker — place the guard THERE (after the
       fallback, so it validates the FINAL manifest+model). setRole is a 4-case switch; add `rsn` to its
       signature + the RoleConfig literal.
  gotcha: post-S1 the ResolveRoleModel call is `prov, mdl, _ := …` (S1's arity discard) — S2 renames `_`→`rsn`.

- file: internal/provider/render.go   (READ ONLY — the chokepoint whose message you mirror)
  why: Render's FR-R5b guard (render.go ~118) uses `*r.ProviderFlag != "" && modelToUse != ""` + a `strings.Index`
       slash check, error `"provider render %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\""`.
       ResolveRoles mirrors this on `mdl` with a `role %q:` prefix. Render's `reasoning` 4th param (render.go
       ~131) appends `r.ReasoningLevels[reasoning]` — the graceful no-op is Render's job, not ResolveRoles'.
  critical: do NOT edit render.go. The two layers must agree on the predicate (multi-provider + non-empty model
       + no slash) so a config ResolveRoles accepts, Render also accepts (and vice versa).

- file: internal/decompose/{planner,message,arbiter,stager}.go   (the 4 Render callers you EDIT)
  why: each has `_, mdl := config.ResolveRoleModel("<role>", deps.Config)` (post-S1 `_, mdl, _`) + a
       `deps.Roles.<Role>.Render(mdl, sysPrompt, payload, "", mode)` call. S2: `_, mdl, rsn := …` + `…, rsn, mode)`.
  pattern: identical edit in all 4 — capture the 3rd return, pass it as Render's 4th positional. Refresh the
       "P1.M2 wires real per-role reasoning" comment (the wiring is now done).
  gotcha: stager.go's Render call is `Render(mdl, "", task, "", provider.RenderTooled)` — sysPrompt is the
       literal `""` (2nd arg); reasoning is the 4th `""`. Change ONLY the 4th arg → `rsn`.

- file: pkg/stagecoach/stagecoach.go   (RoleModel ~74, applyRoleOverride ~274, resolveDecomposeConfig gate ~261)
  why: the public API surface. RoleModel gets Reasoning; applyRoleOverride field-merges it; the gate admits it.
  pattern: mirror the Provider/Model handling in applyRoleOverride + the gate exactly (`!= ""` non-zero merge).
  gotcha: DecomposeOptions has NO Message RoleModel (message reasoning = global cfg.Reasoning) — do NOT add one.

- file: internal/decompose/roles_test.go   (the tests you REWORK)
  why: the 4 `TestResolveRoles_FR5b_*` tests (252/276/309/337) + `TestIsMultiProvider` (502) + the
       `roleModel`/`roleManifest` helpers (525+) + `withInferenceProvider` (dead). See the rework table in
       research §1.
  critical: `withInferenceProvider` sets DefaultProvider (REMOVED) — it will not compile / is dead. Remove it.
       The 3 dead FR-R5b tests assert `"no inference provider"` — rework to `"must be inference/model"`.

- file: internal/config/roles.go   (READ ONLY — S1's ResolveRoleModel 3-return + defaultRoleReasoning)
  why: confirms the contract S2 consumes — `ResolveRoleModel(role, cfg) (provider, model, reasoning string)`,
       reasoning = per-role → global → `defaultRoleReasoning[role]` (planner="high"). S2 does NOT edit this;
       it just consumes the 3rd return.
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  roles.go             # ResolveRoleModel 3-return (S1) + defaultRoleReasoning{planner:high}  ← (INPUT; NO edit)
  role_defaults.go     # FR-D4 model table                                                                  ← (NO edit)
internal/provider/
  render.go            # Render(model,sys,user,reasoning,mode...) + FR-R5b chokepoint guard                  ← (INPUT; NO edit)
  manifest.go          # Manifest (ProviderFlag/ReasoningLevels/...)                                         ← (NO edit)
internal/decompose/
  roles.go             # ResolveRoles + isMultiProvider + setRole + RoleModels  ← EDIT (guard rework + rsn + setRole)
  planner.go           # callPlanner Render ( mdl, sys, payload, "", Bare)        ← EDIT (rsn)
  message.go           # generateMessage Render ( mdl, sys, payload, "", Bare)    ← EDIT (rsn)
  arbiter.go           # runArbiter Render ( mdl, sys, payload, "", Bare)        ← EDIT (rsn)
  stager.go            # stageConcept Render ( mdl, "",  task,  "", Tooled)      ← EDIT (rsn)
  roles_test.go        # TestResolveRoles_FR5b_* + TestIsMultiProvider + helpers ← EDIT (rework + assertions)
  *_test.go            # planner/message/arbiter/stager/chain/decompose tests   ← (VERIFY still green)
pkg/stagecoach/
  stagecoach.go         # RoleModel + applyRoleOverride + resolveDecomposeConfig + Decompose + runPipeline     ← EDIT
internal/generate/
  generate.go          # CommitStaged single-commit Render                                                     ← (VERIFY cfg.Reasoning)
internal/cmd/
  root.go              # flag registration (S1 mid-flight — transient redecl errors)                           ← (NO edit; S1 owns)
go.mod / go.sum        # unchanged (no new dep)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. All changes are EDITS to existing files (listed in Current Codebase tree above).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call #1): the FR-R5b guard is now a MODEL-PREFIX check. The old inferenceProvider(m)
// read *m.Resolve().DefaultProvider — that field is GONE (P1.M1.T1.S1 folded the backend into the model
// slash-prefix). The new guard: if isMultiProvider(m) && mdl != "" && !strings.Contains(mdl, "/") → error.
// isMultiProvider STAYS (tests ProviderFlag); inferenceProvider is NOT re-added. Mirror Render's chokepoint
// predicate exactly so the two layers agree. Add "strings" to roles.go imports.

// CRITICAL (design call #3): the 4 TestResolveRoles_FR5b_* tests assert DEAD fragments ("no inference
// provider", "[provider.pi] default_provider") and use the DEAD withInferenceProvider helper. They MUST be
// reworked (3 of them) or the suite fails to compile/assert. BareModelOnClaude_NoError stays (claude is
// single-backend). grep for 'withInferenceProvider|"no inference provider"|default_provider' must be empty.

// CRITICAL: the guard runs AFTER the stager fallback (on the FINAL manifest+model). If a stager fallback
// lands on pi (multi-provider) with a bare fallback model, the guard FIRES — correct per FR-R5b. The existing
// stager-fallback tests fall back to CLAUDE (single-backend) so they're unaffected, but RUN them to confirm.

// CRITICAL: the 4 Render callers RE-DERIVE reasoning via ResolveRoleModel (they already re-derive mdl this
// way — Deps has no RoleModels field). Do NOT thread RoleModels into Deps (P3.M4 orchestrator scope). Both
// ResolveRoles and the callers call ResolveRoleModel with the SAME cfg ⇒ identical reasoning ⇒ no divergence.

// CRITICAL: change ONLY the Render 4th positional arg in the 4 callers ("" → rsn). stager.go's Render is
// Render(mdl, "", task, "", RenderTooled) — the 2nd arg "" is sysPrompt (stager has none); change the 4th "".

// GOTCHA: pkg/stagecoach Options has NO Reasoning field and S2 does NOT add one. The single-commit path reads
// cfg.Reasoning (from config.Load, S1's plumbing). Item (e) is VERIFY-only — ensure generate.go + runPipeline
// pass cfg.Reasoning (flip any lingering literal "" — S1 is mid-flight, the repo currently has a transient
// root.go redeclaration error).

// GOTCHA: applyRoleOverride's early-return guard MUST include Reasoning, else a reasoning-only
// RoleModel{Reasoning:"high"} with empty Provider/Model returns early and the set is lost. The per-role gate
// in resolveDecomposeConfig MUST also admit Reasoning-only overrides (|| opts.X.Reasoning != ""), else
// applyRoleOverride is never called for a reasoning-only RoleModel.

// GOTCHA: do NOT edit root.go (S1 mid-flight — it currently has transient `flagPlannerProvider redeclared`
// errors while S1 adds the reasoning/message-* flags; that is S1's work, not S2's). Do NOT edit render.go,
// manifest.go, role_defaults.go, roles.go(config). Do NOT bump CurrentConfigVersion (P1.M3.T1.S1). Do NOT edit
// docs/*.md (Mode A inline docs only; changeset sync is P4.M2.T1).

// GOTCHA: setRole builds config.RoleConfig{Provider: prov, Model: mdl} — add Reasoning: rsn. RoleModels.X is
// a config.RoleConfig (S1 added the Reasoning field to it), so no struct change in decompose — just populate it.
```

## Implementation Blueprint

### Data models and structure

```go
// pkg/stagecoach/stagecoach.go — RoleModel gains Reasoning (additive, §14.1)
// A zero value ⇒ inherit global; a non-empty field overrides just that field (FR-R3 field-merge).
type RoleModel struct {
	Provider  string
	Model     string
	Reasoning string // off|low|medium|high (FR-R6); "" ⇒ inherit the global [defaults].reasoning ⇒ shipped default
}
```

```go
// internal/decompose/roles.go — the FR-R5b guard (replaces the TODO(P1.M2.T1.S2) marker, AFTER stager fallback)
import (
	// ...existing...
	"strings" // NEW — strings.Contains for the FR-R5b slash check
)

// inside ResolveRoles, after the stager-fallback block and BEFORE setRole:
prov, mdl, rsn := config.ResolveRoleModel(role, cfg) // (post-S1: was `prov, mdl, _`)
// ... auto-detect / Get / Validate / IsInstalled / stager-fallback (unchanged) ...

// FR-R5b (role-named early check; Render re-enforces at the chokepoint for ALL paths). A model PINNED on a
// multi-provider (provider_flag set, e.g. pi) MUST carry its inference backend as a slash-prefix
// ("zai/glm-5.2"); a bare model is an unroutable config error, never a silent bare --model. Mirrors Render's
// guard (render.go) but fires earlier with the role name. mdl=="" is NOT pinned ⇒ no guard (Render uses the
// manifest DefaultModel + its own guard). Single-backend providers (ProviderFlag=="") are never guarded.
if isMultiProvider(m) && mdl != "" && !strings.Contains(mdl, "/") {
	return RoleManifests{}, RoleModels{}, fmt.Errorf(
		"role %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\"", role, mdl, m.Name)
}

setRole(&rm, &rmodels, role, m, prov, mdl, rsn) // rsn threaded into RoleModels.X.Reasoning
```

```go
// internal/decompose/roles.go — setRole gains rsn; populates the full config.RoleConfig triple
func setRole(rm *RoleManifests, rmodels *RoleModels, role string, m provider.Manifest, prov, mdl, rsn string) {
	rc := config.RoleConfig{Provider: prov, Model: mdl, Reasoning: rsn}
	switch role {
	case "planner":
		rm.Planner, rmodels.Planner = m, rc
	case "stager":
		rm.Stager, rmodels.Stager = m, rc
	case "message":
		rm.Message, rmodels.Message = m, rc
	case "arbiter":
		rm.Arbiter, rmodels.Arbiter = m, rc
	}
}
```

```go
// pkg/stagecoach/stagecoach.go — applyRoleOverride gains Reasoning field-merge (+ updated early-return guard)
func applyRoleOverride(roles map[string]config.RoleConfig, role string, rm RoleModel) {
	if rm.Provider == "" && rm.Model == "" && rm.Reasoning == "" {
		return
	}
	rc := roles[role] // copy (zero value if absent)
	if rm.Provider != "" {
		rc.Provider = rm.Provider
	}
	if rm.Model != "" {
		rc.Model = rm.Model
	}
	if rm.Reasoning != "" {
		rc.Reasoning = rm.Reasoning
	}
	roles[role] = rc // REQUIRED write-back (Go maps return value copies); preserves sibling fields
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: roles.go — capture rsn + re-add the FR-R5b model-prefix guard + extend setRole
  - IMPORTS: add "strings" to internal/decompose/roles.go (for strings.Contains). Keep all existing imports.
  - ResolveRoles: change `prov, mdl, _ := config.ResolveRoleModel(role, cfg)` → `prov, mdl, rsn := …`
    (post-S1 the site is the `_` discard; if S1 left it 2-value, make it 3-value `prov, mdl, rsn`).
  - GUARD: replace the `// TODO(P1.M2.T1.S2): re-add the role-named FR-R5b guard …` marker (AFTER the stager-
    fallback block, BEFORE setRole) with: `if isMultiProvider(m) && mdl != "" && !strings.Contains(mdl, "/") {
    return RoleManifests{}, RoleModels{}, fmt.Errorf("role %q: model %q on %s must be inference/model, e.g.
    \"zai/glm-5.2\"", role, mdl, m.Name) }`.
  - setRole: add `rsn string` param; change `config.RoleConfig{Provider: prov, Model: mdl}` → add `Reasoning: rsn`.
  - setRole CALL: `setRole(&rm, &rmodels, role, m, prov, mdl)` → `…, m, prov, mdl, rsn)`.
  - DOC (Mode A): a comment on the guard (role-named early check; Render re-enforces; mdl=="" not pinned);
    refresh the RoleModels doc to note it carries resolved reasoning.
  - GOTCHA: isMultiProvider UNCHANGED; NO inferenceProvider function. The guard validates the FINAL (post-
    fallback) pair. `m.Name` in the error (mirrors Render's `m.name`).

Task 2: the 4 decompose Render callers — capture rsn + pass to Render
  - planner.go: `_, mdl := config.ResolveRoleModel("planner", deps.Config)` (post-S1 `_, mdl, _`) →
    `_, mdl, rsn := …`; `deps.Roles.Planner.Render(mdl, sysPrompt, payload, "", provider.RenderBare)` →
    `…, rsn, provider.RenderBare)`. Refresh the "P1.M2 wires real per-role reasoning" comment.
  - message.go: same pattern — `_, mdl, rsn := config.ResolveRoleModel("message", deps.Config)`;
    `deps.Roles.Message.Render(mdl, sysPrompt, payload, rsn, provider.RenderBare)`.
  - arbiter.go: `_, mdl, rsn := config.ResolveRoleModel("arbiter", deps.Config)`;
    `deps.Roles.Arbiter.Render(mdl, sysPrompt, payload, rsn, provider.RenderBare)`.
  - stager.go: `_, mdl, rsn := config.ResolveRoleModel("stager", deps.Config)`;
    `deps.Roles.Stager.Render(mdl, "", task, rsn, provider.RenderTooled)` — NOTE: only the 4th arg ("") → rsn;
    the 2nd arg "" (sysPrompt) is unchanged.
  - VERIFY: `grep -rn 'Render(mdl,' internal/decompose/*.go` shows NO remaining literal `""` 4th arg.

Task 3: pkg/stagecoach — RoleModel.Reasoning + applyRoleOverride + the per-role gate
  - RoleModel: add `Reasoning string` (after Model) with a one-line FR-R6 comment; refresh the doc to mention
    reasoning (Mode A); keep the "Stable as of v2.0" (additive-only — §14.1).
  - applyRoleOverride: early-return guard → `if rm.Provider == "" && rm.Model == "" && rm.Reasoning == ""`;
    add `if rm.Reasoning != "" { rc.Reasoning = rm.Reasoning }` (after the Model branch).
  - resolveDecomposeConfig gate: add `|| opts.Planner.Reasoning != "" || opts.Stager.Reasoning != "" ||
    opts.Arbiter.Reasoning != ""` to the existing per-role `if` (so a reasoning-only RoleModel triggers the merge).
  - DecomposeOptions doc: mention Planner/Stager/Arbiter RoleModel now carries reasoning (Mode A).
  - GOTCHA: NO Message RoleModel field (message reasoning = global cfg.Reasoning). NO Reasoning on Options.

Task 4: VERIFY (and fix-if-needed) the single-commit Render reasoning arg
  - internal/generate/generate.go (CommitStaged): confirm `deps.Manifest.Render(cfg.Model, sysPrompt, payload,
    cfg.Reasoning)` — the 4th arg is cfg.Reasoning, NOT "". If it is still "", flip it.
  - pkg/stagecoach/stagecoach.go (runPipeline, ~L461): confirm `deps.Manifest.Render(cfg.Model, sysPrompt,
    payload, cfg.Reasoning)`. If still "", flip it.
  - GOTCHA: read cfg.Reasoning DIRECTLY (these paths read cfg.Model directly, NOT ResolveRoleModel). This is
    S1's Task 6; S2 only verifies/ensures. If you flip a literal "" to cfg.Reasoning you are completing S1's
    declared work (allowed — keeps the build green); do NOT add Reasoning to Options.

Task 5: roles_test.go — rework the FR-R5b tests + add reasoning assertions
  - REMOVE `withInferenceProvider` (and any helper that sets DefaultProvider — the field is gone). `grep` clean.
  - TestResolveRoles_FR5b_BareModelOnPi: model="glm-5-turbo" (no /) on auto-detected pi → assert err != nil &&
    strings.Contains(err.Error(), "must be inference/model"). (Drop the old "no inference provider"/"default_provider" asserts.)
  - TestResolveRoles_FR5b_BareModelOnClaude_NoError: KEEP (claude single-backend ⇒ no guard). Optionally assert
    rmodels.Planner.Reasoning == "" (off, since nothing sets it). No provider/model assertion change.
  - TestResolveRoles_FR5b_ProviderSet_NoInferenceProvider → RENAME/reframe to ..._BareModelOnExplicitPi:
    Provider:"pi", Roles.planner.Model:"gpt-5.4" (no /) → assert err contains "must be inference/model".
  - TestResolveRoles_FR5b_InferenceProviderSet_NoError → RENAME/reframe to ..._SlashPrefixModel_NoError:
    Provider:"pi", Roles.planner.Model:"zai/gpt-5.4" (HAS /), plain goRegistry(t,[]string{"pi"},nil) → assert
    err == nil; rmodels.Planner.Model=="zai/gpt-5.4"; rmodels.Planner.Provider=="pi".
  - ADD reasoning assertions to TestResolveRoles_PerRoleOverrides (and a dedicated test if absent): with
    cfg.Roles["planner"].Reasoning unset + global unset, rmodels.Planner.Reasoning == "high" (shipped default
    via ResolveRoleModel); set cfg.Roles["message"].Reasoning="low" → rmodels.Message.Reasoning=="low".
  - KEEP TestIsMultiProvider unchanged (isMultiProvider is unchanged).
  - RUN the stager-fallback tests (143/193) — confirm they still pass (they fall back to claude ⇒ no guard).

Task 6: VERIFY (no further file change)
  - RUN the full Validation Loop. go.mod/go.sum byte-unchanged. No edits to root.go/render.go/manifest.go/
    role_defaults.go/docs. `go test -race ./internal/decompose/ ./pkg/stagecoach/ ./internal/generate/ ./...` green.
```

### Implementation Patterns & Key Details

```go
// The FR-R5b guard — mirrors Render's chokepoint (render.go ~118) but fires EARLIER with a role name.
// Predicate is IDENTICAL in spirit: multi-provider + pinned model + no slash ⇒ error.
if isMultiProvider(m) && mdl != "" && !strings.Contains(mdl, "/") {
	return RoleManifests{}, RoleModels{}, fmt.Errorf(
		"role %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\"", role, mdl, m.Name)
}
// mdl=="" ⇒ NOT user-pinned ⇒ no guard (Render uses manifest DefaultModel + its own guard). Correct: the
// early check is for USER misconfiguration; the manifest author owns DefaultModel correctness.

// The reasoning re-derive in each caller — identical to how mdl is already re-derived (Deps has no RoleModels):
_, mdl, rsn := config.ResolveRoleModel("planner", deps.Config) // was `_, mdl, _` post-S1
spec, rerr := deps.Roles.Planner.Render(mdl, sysPrompt, payload, rsn, provider.RenderBare) // 4th arg: "" → rsn

// applyRoleOverride — Reasoning follows Provider/Model (non-zero field-merge + write-back):
if rm.Reasoning != "" {
	rc.Reasoning = rm.Reasoning
}
roles[role] = rc // write-back REQUIRED (Go maps return value copies) — preserves sibling Provider/Model/Reasoning
```

```go
// roles_test.go — the reworked keystone FR-R5b test (bare model on pi ⇒ role-named error):
func TestResolveRoles_FR5b_BareModelOnPi(t *testing.T) {
	reg := goRegistry(t, []string{"pi"}, nil) // pi installed (multi-provider); auto-detect picks it
	cfg := config.Config{Roles: map[string]config.RoleConfig{"planner": {Model: "glm-5-turbo"}}}
	_, _, err := ResolveRoles(cfg, reg)
	if err == nil {
		t.Fatal("ResolveRoles nil error, want FR-R5b model-prefix error")
	}
	if !strings.Contains(err.Error(), "must be inference/model") || !strings.Contains(err.Error(), "planner") {
		t.Errorf("error = %q, want role-named must be inference/model", err)
	}
}

// roles_test.go — the reworked correct-config test (slash prefix ⇒ no error):
func TestResolveRoles_FR5b_SlashPrefixModel_NoError(t *testing.T) {
	reg := goRegistry(t, []string{"pi"}, nil)
	cfg := config.Config{Provider: "pi", Roles: map[string]config.RoleConfig{
		"planner": {Provider: "pi", Model: "zai/gpt-5.4"},
	}}
	rm, rmodels, err := ResolveRoles(cfg, reg)
	if err != nil { t.Fatalf("ResolveRoles: %v", err) }
	if rmodels.Planner.Model != "zai/gpt-5.4" || rm.Planner.Name != "pi" {
		t.Errorf("planner model/manifest = %q/%q, want zai/gpt-5.4/pi", rmodels.Planner.Model, rm.Planner.Name)
	}
}

// roles_test.go — reasoning threads into RoleModels (planner ships high when nothing is set):
func TestResolveRoles_ReasoningShippedDefault(t *testing.T) {
	reg := bogusRegistry(t, []string{"claude"}) // single-backend ⇒ no FR-R5b guard
	cfg := config.Config{}                       // nothing set
	_, rmodels, err := ResolveRoles(cfg, reg)
	if err != nil { t.Fatalf("ResolveRoles: %v", err) }
	if rmodels.Planner.Reasoning != "high" {
		t.Errorf("planner reasoning = %q, want high (FR-R6 shipped default)", rmodels.Planner.Reasoning)
	}
	if rmodels.Message.Reasoning != "" {
		t.Errorf("message reasoning = %q, want \"\" (off)", rmodels.Message.Reasoning)
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. No new dependency; reasoning is a string field, the guard is stdlib strings.Contains.
    `go mod tidy` MUST be a no-op. `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES:
  - internal/decompose → internal/config (ResolveRoleModel, RoleConfig), internal/provider (Manifest, Render).
    NO new import (strings is stdlib). 
  - pkg/stagecoach → internal/decompose (ResolveRoles, RoleModels), internal/config (RoleConfig). NO new import.

FROZEN / NOT-EDITED (do NOT touch):
  - internal/cmd/root.go — S1 mid-flight (transient redeclaration errors while S1 adds reasoning/message-* flags).
  - internal/provider/render.go + manifest.go — Render's reasoning param + FR-R5b chokepoint are the INPUT.
  - internal/config/roles.go + role_defaults.go — ResolveRoleModel 3-return + defaultRoleReasoning (S1) and the
    FR-D4 model table are INPUTS. defaultRoleReasoning lives in roles.go(config), NOT decompose.
  - internal/config/config.go const CurrentConfigVersion — P1.M3.T1.S1 bumps it.
  - docs/*.md — DOCS is Mode A (inline comments only); changeset doc sync is P4.M2.T1.
  - pkg/stagecoach.Options — NO Reasoning field (single-commit reasoning = cfg.Reasoning from config.Load).

DOWNSTREAM CONTRACT (hand-off):
  - P3.M4 orchestrator consumes RoleModels (now carrying Reasoning) + the validated RoleManifests; it threads
    Deps (RoleManifests only — RoleModels retained locally, per the existing contract). S2 does NOT change Deps.
  - Render's reasoning 4th arg is now exercised by every decompose path (FR-R6 reasoning tokens append via
    ReasoningLevels; absent ⇒ graceful no-op, Render's responsibility).

NO DATABASE / NO ROUTES / NO NEW FILES / NO CLI WIRING (root.go is S1's) / NO CONFIG VERSION BUMP.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/decompose/roles.go internal/decompose/planner.go internal/decompose/message.go \
  internal/decompose/arbiter.go internal/decompose/stager.go internal/decompose/roles_test.go \
  pkg/stagecoach/stagecoach.go
test -z "$(gofmt -l internal/ pkg/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./...          # Expect zero diagnostics (catches a missed call-site arg, an unused rsn, a stale helper).
go build ./...        # Whole module compiles. NOTE: if root.go still has S1's transient redeclaration errors,
                     #   that is S1's in-flight work, NOT S2's — do NOT fix root.go; re-run once S1 lands.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm the FR-R5b guard + rsn wiring landed, no dead refs remain:
grep -n 'must be inference/model' internal/decompose/roles.go          # the guard error (1 hit)
grep -rn 'Render(mdl,' internal/decompose/*.go | grep '", provider'    # MUST be empty — no literal "" 4th arg left
grep -n 'withInferenceProvider\|"no inference provider"\|default_provider' internal/decompose/roles_test.go  # MUST be empty
grep -n 'Reasoning' pkg/stagecoach/stagecoach.go                          # RoleModel.Reasoning + applyRoleOverride + gate
# Expected: clean. If `go build` fails ONLY on root.go redeclarations, that is S1 mid-flight — leave root.go.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./internal/decompose/ -v   # reworked FR-R5b tests + reasoning assertions + stager-fallback + isMultiProvider
go test -race ./pkg/stagecoach/ -v        # RoleModel.Reasoning flows via applyRoleOverride; Decompose single+multi
go test -race ./internal/generate/ -v    # single-commit passes cfg.Reasoning (verify)
go test -race ./...                      # Full suite — NO regressions (config/provider tests untouched stay green).
# Expected: all PASS. Key assertions: bare-model-on-pi ⇒ "must be inference/model" (role-named); slash-prefix ⇒
#   no error; claude+bare-model ⇒ no error (single-backend); rmodels.Planner.Reasoning=="high" (shipped),
#   rmodels.Message.Reasoning=="" (off); RoleModel{Reasoning:"high"} ⇒ cfg.Roles[role].Reasoning=="high".
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"   # (once S1's root.go lands)
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm no file outside the listed edits was touched (frozen-file gate):
git diff --name-only | grep -Ev 'internal/decompose/(roles|planner|message|arbiter|stager|roles_test)\.go|pkg/stagecoach/stagecoach\.go|internal/generate/generate\.go' \
  && echo "UNEXPECTED file changed (root.go = S1 mid-flight, OK; anything else = investigate)" || echo "only listed files changed (good)"
# Reasoning end-to-end smoke (throwaway, optional): build a DecomposeOptions{Planner: RoleModel{Reasoning:"high"}}
# + a stub registry, call resolveDecomposeConfig, assert cfg.Roles["planner"].Reasoning=="high"; then call
# ResolveRoles and assert rmodels.Planner.Reasoning=="high". (The in-package roles_test.go already covers this.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Two-layer FR-R5b agreement (belt-and-suspenders): for a pi manifest + a bare model "glm-5.2", BOTH
# ResolveRoles (role-named) and provider.Manifest.Render (chokepoint) must error with "must be inference/model".
# The roles_test.go table covers ResolveRoles; render_test.go (P1.M1.T1.S2) covers Render. No new test needed,
# but eyeball that the two error strings share the "must be inference/model" fragment (they do, by design).
# golangci-lint: `make lint` (project-wide gate).
# Property invariant (optional): for any role, if isMultiProvider(manifest) is false, ResolveRoles NEVER emits
# the FR-R5b error regardless of mdl (single-backend providers take bare models). A short loop over the 6
# built-ins × {bare, slash, empty} model asserts this.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/ pkg/`, `go mod tidy` no-op;
      `git diff --exit-code go.mod go.sum` empty. (root.go transient errors = S1 mid-flight, not S2.)
- [ ] Level 2 green: `go test -race ./...` (decompose reworked tests + reasoning assertions + pkg/stagecoach + generate).
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only listed files changed (root.go excepted as S1's).

### Feature Validation

- [ ] `ResolveRoles` has the FR-R5b model-prefix guard (`isMultiProvider(m) && mdl != "" && !strings.Contains(mdl,
      "/")`) AFTER the stager fallback, with the role-named `"must be inference/model"` error; `inferenceProvider`
      absent; `isMultiProvider` unchanged; `"strings"` imported.
- [ ] `ResolveRoles` captures `rsn` and `setRole` populates `RoleModels.X.Reasoning` (planner="high" shipped default).
- [ ] The 4 decompose Render callers pass `rsn` as Render's 4th arg (no literal `""` 4th arg in decompose).
- [ ] `pkg/stagecoach.RoleModel` has `Reasoning`; `applyRoleOverride` field-merges it; the per-role gate admits it.
- [ ] Single-commit Render (generate.go + runPipeline) passes `cfg.Reasoning`.
- [ ] The 3 dead FR-R5b tests reworked; `withInferenceProvider` removed; `BareModelOnClaude_NoError` kept;
      reasoning assertions present.

### Code Quality Validation

- [ ] Mirrors existing patterns: Render 4th-arg change is positional-only; applyRoleOverride mirrors Provider/Model;
      the guard mirrors Render's chokepoint predicate.
- [ ] No scope creep into S1 (config/root.go), P1.M3 (config version), P3.M4 (Deps/orchestrator), or Options.Reasoning.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Mode A doc comments on the ResolveRoles guard + RoleModels reasoning + RoleModel/DecomposeOptions reasoning.
- [ ] No docs/*.md edits (changeset doc sync is P4.M2.T1, Mode B).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't re-add `inferenceProvider` or reference `DefaultProvider`. The field is GONE (P1.M1.T1.S1). The new
  guard is a MODEL-PREFIX check (`strings.Contains(mdl, "/")`), keyed on `isMultiProvider(m)` (ProviderFlag).
- ❌ Don't fire the guard when `mdl == ""`. An empty model is NOT user-pinned — Render uses manifest DefaultModel
  + its own guard. The early check is for USER misconfiguration only. Predicate: `isMultiProvider(m) && mdl != "" && !strings.Contains(mdl,"/")`.
- ❌ Don't change the Render 2nd/3rd/5th args in the callers — only the 4th (reasoning). stager.go's
  `Render(mdl, "", task, "", RenderTooled)` has `""` as BOTH the 2nd (sysPrompt) and 4th (reasoning) — change
  ONLY the 4th.
- ❌ Don't thread `RoleModels` into `Deps` to pass reasoning. The callers RE-DERIVE it via `ResolveRoleModel`
  (same as they re-derive `mdl`) — Deps has no Models field by design (orchestrator retains RoleModels; P3.M4).
- ❌ Don't leave the 3 dead FR-R5b tests asserting `"no inference provider"` / using `withInferenceProvider`.
  Rework them to `"must be inference/model"` + slash-prefix fixtures, or the suite fails to compile/assert.
- ❌ Don't add `Reasoning` to `pkg/stagecoach.Options`. The item scopes it to `RoleModel` only. The single-commit
  path reads `cfg.Reasoning` (config.Load). Item (e) is VERIFY-only.
- ❌ Don't edit `internal/cmd/root.go` (S1 mid-flight — transient `flagPlannerProvider redeclared` errors are
  S1's, not S2's), `render.go`/`manifest.go`, `role_defaults.go`, `config/roles.go`, or `docs/*.md`.
- ❌ Don't forget the `applyRoleOverride` early-return guard + the per-role gate — both MUST include Reasoning,
  else a reasoning-only `RoleModel{Reasoning:"high"}` is silently dropped (early return / gate never opens).
- ❌ Don't forget the map-value-copy write-back in `applyRoleOverride` (`roles[role] = rc`) — without it the set
  is lost (Go maps return copies).
- ❌ Don't place the FR-R5b guard BEFORE the stager fallback — it must validate the FINAL (post-fallback)
  (manifest, model) pair (a fallback onto pi with a bare model is still an error). Place it at the TODO marker.
- ❌ Don't change go.mod/go.sum or add new files. Pure in-place edits + a stdlib (`strings`) import.
- ❌ Don't skip `go vet`/`go build`/`gofmt`/`go test -race ./...` — they catch a missed Render arg, an unused
  `rsn`, the stale `withInferenceProvider` helper, and formatting drift before the orchestrator (P3.M4) freezes
  on the `RoleModels` shape.
