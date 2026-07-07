---
name: "P1.M3.T2.S2 — ResolveRoleModel(): a pure function resolving a role's (provider, model) with the 5-layer precedence (flag/env/file already merged into cfg.Roles → global cfg.Provider/cfg.Model → manifest-default sentinel) — PRD §9.15 FR-R3 / §16.4 / architecture config_v2_delta.md §4"
description: |

  Land the SECOND subtask of Per-Role Config Schema & Model Resolution's resolution half (P1.M3.T2): the
  READ-SIDE counterpart to S1's write-side. S1 (DONE) made loadEnv/loadFlags populate `cfg.Roles` with the
  per-role overrides (flag/env layers) on top of file.go's file/git layers (M3.T1.S2, DONE); the registry
  resolves the global `cfg.Provider`/`cfg.Model`. THIS subtask is the PURE function that collapses all that
  into one answer for a given role. The item contract (point #1/#3) and delta §4 are crystal clear: by the
  time this runs, all 7 precedence layers are ALREADY merged into `cfg`; the function only checks
  `cfg.Roles[role]` (per-role, per-field) then falls back to `cfg.Provider`/`cfg.Model` (global). The manifest
  default (lowest layer) is deliberately NOT handled here — `("", "")` is the sentinel the downstream consumer
  (registry/Render, P3.M2.T1.S1 / the CLI) interprets as "use manifest `default_model` / auto-detect provider".

  ONE NEW FILE (the function) + ONE NEW FILE (its tests). NO EDITS to anything else:
    1. internal/config/roles.go     — CREATE: `func ResolveRoleModel(role string, cfg Config) (provider, model string)`.
    2. internal/config/roles_test.go — CREATE: white-box `package config` tests for all resolution cases.

  WHY A NEW `roles.go` (not config.go, not load.go):
    - config.go is FROZEN (owned by P1.M3.T1.S1, SHIPPED). A sibling task must not edit it.
    - load.go is the parallel sibling P1.M3.T2.S1's territory (DONE). Putting ResolveRoleModel in load.go
      creates a merge-conflict surface; a new roles.go is 100% disjoint → zero merge risk.
    - Cohesion: ResolveRoleModel is the role-resolution READ logic; load.go is the WRITE side. A dedicated
      roles.go is where role-resolution helpers naturally live.

  THE FUNCTION (authoritative source: architecture/config_v2_delta.md §4 — quoted verbatim in the Blueprint):
    ```go
    // ResolveRoleModel returns the (provider, model) for a role, applying the precedence
    // flag/env/[role.*] config (all already merged into cfg.Roles) > [defaults] global
    // (cfg.Provider/cfg.Model) > manifest-default sentinel.
    func ResolveRoleModel(role string, cfg Config) (provider, model string) {
        if rc, ok := cfg.Roles[role]; ok {
            if rc.Provider != "" { provider = rc.Provider }
            if rc.Model    != "" { model    = rc.Model    }
        }
        if provider == "" { provider = cfg.Provider }
        if model    == "" { model    = cfg.Model    }
        return provider, model
    }
    ```
  Provider and Model are resolved INDEPENDENTLY (per-field — FR-R3/FR37a merge at the role→global boundary):
  a model-only role override inherits the global provider, a provider-only override inherits the global model.
  `("", "")` means "use manifest defaults" (model="" ⇒ manifest default_model; provider="" ⇒ auto-detect).

  ⚠️ **THE #1 scope boundary — NEW roles.go + roles_test.go ONLY.** Do NOT touch config.go (frozen),
  load.go/load_test.go (P1.M3.T2.S1, DONE), file.go/git.go (M3.T1.S2 + others), or ANY other file. The
  function is purely additive — it reads cfg and returns two strings. (design-decisions §0/§6)

  ⚠️ **THE #2 design call — pure function, VALUE semantics.** `cfg Config` by value (NOT `*Config`); named
  returns `(provider, model string)`; NO error return. Every input has a defined output (worst case
  `("", "")`). This is a RESOLUTION function, not a validation function — FR-R5/FR-R5b validation is the
  registry's job at manifest-resolution time. Do NOT import internal/provider (import-cycle risk; provider
  consumes config). (design-decisions §1/§6)

  ⚠️ **THE #3 design call — the 7-layer precedence COLLAPSES to a 2-level check (correct, not a simplification).**
  The loaders (load.go + file.go) ALREADY merged all 7 layers into cfg: cfg.Roles[role] holds the highest-
  precedence per-role value across flag/env/file/git (per-field merged), cfg.Provider/cfg.Model hold the
  resolved global. So checking cfg.Roles[role] then falling back to cfg.Provider/cfg.Model IS the full
  precedence. Do NOT re-walk the layers; do NOT reach into the manifest. (design-decisions §2/§4)

  ⚠️ **THE #4 design call — independent PER-FIELD resolution.** The two `if rc.X != ""` are independent and
  the two `if x == ""` fallbacks are independent. NEVER couple them (`if rc.Provider != "" { provider=...;
  model=rc.Model }` would drop a model-only override whose provider is empty). A test pins each of the four
  combinations. (design-decisions §3)

  ⚠️ **THE #5 design call — `("", "")` is the manifest-default SENTINEL; do NOT resolve the manifest here.**
  The lowest precedence layer ("built-in manifest default" + auto-detect) is the registry/Render's job
  (model="" ⇒ manifest default_model; provider="" ⇒ Registry.DefaultProvider, FR-D1). ResolveRoleModel
  returns ("","") when nothing set a value, and that is the intended signal. Document it in the doc comment
  so consumers (P3.M2.T1.S1 decompose/roles.go, the CLI) know. (design-decisions §4)

  ⚠️ **THE #6 coordination note — roles_test.go MAY reference `roleNames` (load.go, DONE) for one canonical-
  roles iteration test, but the explicit per-case tests hardcode their role strings** so they are self-
  documenting and do not silently change if roleNames is reordered. (design-decisions §7)

  Deliverable: NEW internal/config/roles.go (ResolveRoleModel, ~6-line body + doc comment) + NEW
  internal/config/roles_test.go (8 cases). NO other file touched. NO new dependency (roles.go needs NO
  imports; roles_test.go uses only `testing`). OUTPUT: `ResolveRoleModel(role, cfg)` resolves per-role config
  with correct precedence; `go build ./... && go vet ./... && go test ./...` green; go.mod/go.sum unchanged.

---

## Goal

**Feature Goal**: Provide the pure read-side function that turns the fully-resolved `Config` (built by the
loaders across all 7 precedence layers) into a single `(provider, model)` answer for a given agent role
(planner/stager/message/arbiter), with correct per-field precedence: per-role override (highest) → global
default → manifest-default sentinel (lowest, returned as `("", "")` for the registry/Render to interpret).
This is the function the decompose pipeline (P3.M2.T1.S1 `decompose/roles.go`) and the CLI call to learn
which provider+model to use for each role before invoking the registry to build the manifest.

**Deliverable** (both NEW files — no edits to anything):
1. `internal/config/roles.go` — `func ResolveRoleModel(role string, cfg Config) (provider, model string)`
   with a doc comment citing PRD §16.4 / §9.15 FR-R3 and the `("", "")` manifest-default sentinel semantics.
2. `internal/config/roles_test.go` — white-box `package config` tests covering: global fallback (Roles nil &
   role absent), per-role full override, per-role MODEL-only override (provider inherits global), per-role
   PROVIDER-only override (model inherits global), both-empty ⇒ `("", "")`, unknown/non-canonical role ⇒
   global fallback, and a canonical-roles iteration test.

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `ResolveRoleModel("planner",
cfg)` with `cfg.Roles["planner"]={agy,gemini-2.5-pro}` & global `{pi,""}` returns `(agy, gemini-2.5-pro)`;
with `cfg.Roles["message"]={"",gpt-5.4-nano}` & global `{pi,""}` returns `(pi, gpt-5.4-nano)` (model-only,
provider inherits global); with `cfg.Roles["stager"]={agy,""}` & global `{pi,gpt-5.4}` returns
`(agy, gpt-5.4)` (provider-only, model inherits global); with everything empty returns `("", "")` (sentinel);
an unknown role returns the global; go.mod/go.sum byte-unchanged; config.go + load.go + file.go + git.go +
every non-target file byte-unchanged.

## User Persona

**Target User**: The downstream consumers that call `ResolveRoleModel`:
- **P3.M2.T1.S1 (`internal/decompose/roles.go`)** — for each of the four roles it calls
  `provider, model := config.ResolveRoleModel(role, cfg)` then asks the registry to resolve/build the manifest
  for that `(provider, model)` (model="" ⇒ the manifest's `default_model`; provider="" ⇒ the registry's
  `DefaultProvider` auto-detect, FR-D1). This subtask provides that single-call entry point.
- **The CLI** (single-commit path) — resolves the `message` role; with no per-role override it returns
  `(cfg.Provider, cfg.Model)`, which is exactly v1 (back-compatible).
End-user persona is "the multi-agent tinkerer" (PRD §7.3) who routes roles to different agents
(`stagecoach --planner-model gemini-2.5-pro --planner-provider agy`, §16.4); this function is what makes that
routing resolve correctly per-role.

**Use Case**: A user configures `[role.planner] provider="agy" model="gemini-2.5-pro"` and leaves everything
else on the global `[defaults] provider="pi"`. At run time the decompose orchestrator calls
`ResolveRoleModel("planner", cfg)` → `(agy, gemini-2.5-pro)` (planner on Antigravity); `ResolveRoleModel(
"message", cfg)` → `(pi, "")` (message on pi, model="" ⇒ pi's manifest default_model); `ResolveRoleModel(
"stager", cfg)` → `(pi, "")`. Each role resolves independently and correctly.

**User Journey**: (internal) `Load()` (loaders merge all layers into cfg) → orchestrator calls
`ResolveRoleModel(role, cfg)` per role → `(provider, model)` → registry resolves manifest (model="" ⇒
default_model) → Render builds the command → agent runs.

**Pain Points Addressed**: (1) No single function to answer "what provider+model does role X use?" — consumers
would each re-derive the precedence (duplicated, divergence-prone logic). (2) Per-role overrides being
ignored on the model-only/provider-only edge cases if a consumer couples the two fields. This pure function
centralizes the resolution with correct per-field semantics.

## Why

- **Completes the per-role resolution contract.** S1 wired the env/flag WRITE side into `cfg.Roles`; M3.T1.S2
  wired the file layer. THIS subtask is the READ side — the function that consumes `cfg.Roles`. Without it,
  downstream consumers have no entry point and would each re-implement (and re-bug) the precedence.
- **Satisfies PRD §9.15 FR-R3 (per-role precedence) and §16.4 (per-role resolution).** The function IS
  FR-R3's resolution: flag/env/[role.*] (all in cfg.Roles) > global (cfg.Provider/Model) > manifest default
  (the `("", "")` sentinel the registry interprets).
- **Faithful to the authoritative delta §4.** The implementation is quoted verbatim from
  `architecture/config_v2_delta.md` §4 — no invention, no deviation.
- **Zero behavior change for v1.** On the single-commit path the only role is `message`, and with no
  per-role override `ResolveRoleModel("message", cfg)` returns `(cfg.Provider, cfg.Model)` — identical to
  v1 reading those fields directly. Back-compatible by construction.

## What

NEW `internal/config/roles.go` (the function + doc comment) and NEW `internal/config/roles_test.go` (the
tests). No new dependency, no edits to any existing file, no manifest lookup, no flag registration, no
validation logic.

### Success Criteria

- [ ] `roles.go` defines `func ResolveRoleModel(role string, cfg Config) (provider, model string)` — takes
      `cfg` BY VALUE, named return values, NO error return, NO imports (pure map access + string assignment).
- [ ] The body: read `cfg.Roles[role]` (per-field: `rc.Provider != ""` → provider, `rc.Model != ""` → model,
      independently); then per-field global fallback (`provider == ""` → `cfg.Provider`, `model == ""` →
      `cfg.Model`); return both. (Exactly delta §4.)
- [ ] The doc comment cites PRD §16.4 / §9.15 FR-R3, states the precedence (flag/env/[role.*] in cfg.Roles >
      global > manifest-default sentinel), and documents `("", "")` = "use manifest defaults" (model="" ⇒
      manifest default_model; provider="" ⇒ auto-detect).
- [ ] `roles_test.go` is `package config` (white-box) and covers: Roles-nil global fallback; role-absent
      global fallback; per-role full override; per-role MODEL-only override (provider inherits global);
      per-role PROVIDER-only override (model inherits global); both-empty ⇒ `("", "")`; unknown/non-canonical
      role ⇒ global; a `roleNames`-iteration test over all four canonical roles.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/` clean.
- [ ] go.mod/go.sum byte-unchanged; config.go + load.go + load_test.go + file.go + git.go + every non-target
      file byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact function body (quoted
verbatim from delta §4 in the Blueprint), the `Config` struct fields it reads (enumerated below), the S1
contract (cfg.Roles is a fully-merged `map[string]RoleConfig`), the 8 required test cases (each spelled out
with inputs/expected outputs), and the LEAVE list (config.go/load.go/file.go/git.go frozen). No manifest/
registry/git/prompt knowledge required — this is a ~6-line pure function over two cfg fields and a map.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE function spec (the implementation is quoted verbatim from here)
- docfile: plan/002_a17bb6c8dc1d/architecture/config_v2_delta.md
  section: "§4. Role Resolution Function" — the EXACT ResolveRoleModel signature, body, and the two-sentence
           comment. This PRP implements §4 AS-WRITTEN (no deviation).
  critical: §4 is the single source of truth for the function. The body is ~6 lines; do not embellish. §4's
       comment names the precedence layers correctly; expand it in the doc comment to also state the ("","")
       manifest-default sentinel semantics (§4's comment says 'provider=="" means "use global/default";
       model=="" means "use manifest default_model"' — keep that meaning, make it explicit in the doc).

# MUST READ — the design calls (the non-obvious decisions)
- docfile: plan/002_a17bb6c8dc1d/P1M3T2S2/research/design-decisions.md
  why: the 9 non-obvious calls — new roles.go placement + why not config.go/load.go (§0), value semantics +
       no error return + no internal/provider import (§1/§6), the 7-layer collapse to 2-level check (§2),
       independent per-field resolution (§3), the ("","") manifest-default sentinel (§4), unknown-role global
       fallback (§5), scope boundaries — no manifest/FR-R5/FR-R5b validation (§6), roleNames reuse in tests
       (§7), no new deps (§8), the 8 test cases (§9).
  critical: §2 (the collapse — do NOT re-walk layers or touch the manifest), §3 (per-field independence — the
       most error-prone logic), §4 (the sentinel — do NOT resolve the manifest here), §6 (scope — keep it
       pure) are the things most likely to go wrong if not read.

# MUST READ — the S1/M3.T1 CONTRACT (the Config fields this function READS; config.go is FROZEN — do NOT edit)
- file: internal/config/config.go   (read for the field/types; do NOT edit)
  section: `RoleConfig` (struct {Provider string; Model string}); the Config fields `Roles map[string]RoleConfig
           toml:"-"`, `Provider string`, `Model string`. Also `Defaults()` (returns Config BY VALUE; Roles nil;
           Provider/Model "").
  why: confirms Roles is `map[string]RoleConfig` (so `cfg.Roles[role]` returns a value `RoleConfig` + an `ok`
       bool — the delta §4 idiom), and that Provider/Model are plain strings (the global fallback). Confirms
       `Defaults()` returns by value — the function's value-semantic signature mirrors it.
  critical: do NOT edit config.go. The function only READS these fields.

# MUST READ — the WRITE-SIDE counterpart (confirms cfg.Roles is fully merged by the time this reads it)
- file: internal/config/load.go   (DONE — P1.M3.T2.S1; do NOT edit)
  section: `var roleNames = []string{"planner","stager","message","arbiter"}` (package-level; reusable in
           roles_test.go via the same package); `setRoleProvider`/`setRoleModel` (map-value-copy write into
           cfg.Roles); the loadEnv/loadFlags per-role loops (flag/env write DIRECTLY into cfg.Roles, the
           highest layers). Also `Load()` orchestrates Defaults → file/git overlays → loadEnv → loadFlags.
  why: confirms the precedence-collapse argument (§2 of design-decisions): by the time ResolveRoleModel runs,
       flag/env/file/git have ALL been merged into cfg.Roles (per-field) and cfg.Provider/cfg.Model (global).
       So the 2-level check is complete. Also confirms `roleNames` exists for the test iteration case.
  critical: do NOT edit load.go. Read it to confirm cfg.Roles is the fully-resolved per-role table (not raw
       layer data) — that is WHY ResolveRoleModel does not re-walk layers.

# MUST READ — the FILE-layer merge (confirms the per-field merge idiom at the file layer too)
- file: internal/config/file.go   (DONE — M3.T1.S2; do NOT edit)
  section: `overlay()` — `for role, rc := range src.Roles { existing := dst.Roles[role]; if rc.Provider != ""
           { existing.Provider = rc.Provider }; if rc.Model != "" { existing.Model = rc.Model }; dst.Roles[role]
           = existing }`; `materialize()` converts fileConfig.Role → c.Roles.
  why: confirms the file layer ALSO does per-field merge into cfg.Roles (same independence as ResolveRoleModel's
       read side). This is the symmetry that makes the 2-level collapse correct: every layer merges per-field,
       so cfg.Roles[role] is the per-field-maximum across all layers.
  critical: do NOT edit file.go. Read it to internalize that cfg.Roles is per-field-merged across ALL layers.

# THE TEST STYLE TO MIRROR — white-box package config, construct Config directly, one t.Errorf per assertion
- file: internal/config/config_test.go   (read for style; do NOT edit)
  section: `TestDefaults` (constructs `c := Defaults()`, one `t.Errorf` per assertion, clear want-messages).
  why: the test STYLE — `package config`, build the Config under test directly (here: `cfg := Defaults()` then
       set `cfg.Roles`/`cfg.Provider`/`cfg.Model` by hand), assert with descriptive `t.Errorf`. Do NOT route
       through `Load()` (that obscures the pure function under test).
  critical: do NOT edit config_test.go. Mirror its assertion style in the new roles_test.go.

# The PRD basis (already in your context as selected_prd_content)
- file: PRD.md (or plan/002_a17bb6c8dc1d/prd_snapshot.md)
  section: "9.15 Per-role provider/model configuration" (h3.31) — FR-R1 (4 roles), FR-R3 (precedence: flag >
           env > per-role config > global config > built-in manifest default), FR-R4 (one model for everything
           = global covers all), FR-R5 (model strings are provider-specific — validation is downstream, NOT
           this function).
  section: "16.4 Per-role provider/model configuration" (h3.68) — "Resolution for a role's provider/model
           (highest wins): CLI flag → env → [role.<role>] config → [defaults] config (the global) → built-in
           manifest default_model." And the worked example (planner overridden to agy, message/stager/arbiter
           inherit [defaults] pi).
  critical: §9.15 FR-R3 IS the precedence this function implements; §16.4's resolution sentence is the spec.
       Note FR-R5 validation ("switching a role's provider without updating its model is a config error
       stagecoach surfaces") is the REGISTRY's job, NOT this pure function (design-decisions §6).
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go        # M3.T1.S1 SHIPPED (FROZEN) — RoleConfig + Roles/Provider/Model. READ for fields; do NOT edit.
  config_test.go   # M3.T1.S1 SHIPPED. READ for test style; do NOT edit.
  file.go          # M3.T1.S2 SHIPPED — overlay()/materialize() per-field-merge Roles into cfg. READ; do NOT edit.
  file_test.go     # M3.T1.S2. UNCHANGED.
  git.go           # git-config reader. UNCHANGED.
  git_test.go      # UNCHANGED.
  load.go          # P1.M3.T2.S1 DONE — roleNames + setRole* + loadEnv/loadFlags per-role loops + Load(). READ; do NOT edit.
  load_test.go     # P1.M3.T2.S1 DONE. UNCHANGED.
  roles.go         # *** CREATE *** — ResolveRoleModel (this subtask).
  roles_test.go    # *** CREATE *** — the tests (this subtask).
go.mod / go.sum    # UNCHANGED (roles.go needs NO imports; roles_test.go uses only `testing`).
```

### Desired Codebase tree with files to be added

```bash
internal/config/
  roles.go         # NEW — `func ResolveRoleModel(role string, cfg Config) (provider, model string)` + doc comment.
  roles_test.go    # NEW — white-box package config tests (8 cases).
# (NO other file changes.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — scope: NEW roles.go + roles_test.go ONLY): do NOT touch config.go (FROZEN, M3.T1.S1),
//   load.go/load_test.go (P1.M3.T2.S1, DONE), file.go/git.go (M3.T1.S2 + others), or ANY other file. The
//   function is purely additive — a new file with a pure function. (design-decisions §0/§6)

// CRITICAL (#2 — pure function, VALUE semantics, NO error return): signature is EXACTLY
//   `func ResolveRoleModel(role string, cfg Config) (provider, model string)` — cfg BY VALUE (not *Config),
//   named returns, no error. Every input has a defined output (worst case ("","")). Do NOT add an error
//   return "for validation" — FR-R5/FR-R5b validation is the registry's job (it has the manifest knowledge;
//   config must not import provider — import cycle). (design-decisions §1/§6)

// CRITICAL (#3 — the 7-layer precedence COLLAPSES to a 2-level check; do NOT re-walk layers): the loaders
//   (load.go + file.go) ALREADY merged flag/env/file/git into cfg.Roles (per-field) and cfg.Provider/
//   cfg.Model (global). So `if rc,ok := cfg.Roles[role]; ok { per-field } ; if x=="" { x = cfg.X }` IS the
//   full precedence. Do NOT re-read env/files/git; do NOT reach into the manifest. (design-decisions §2/§4)

// CRITICAL (#4 — independent PER-FIELD resolution; do NOT couple provider & model): the two `if rc.X != ""`
//   are independent and the two `if x == ""` fallbacks are independent. A model-only role override MUST keep
//   the global provider; a provider-only override MUST keep the global model. NEVER write
//   `if rc.Provider != "" { provider = rc.Provider; model = rc.Model }` (drops a model-only override).
//   A test pins each of the four combinations. (design-decisions §3)

// CRITICAL (#5 — ("","") is the manifest-default SENTINEL; do NOT resolve the manifest here): the lowest
//   layer ("built-in manifest default" + provider auto-detect) is the registry/Render's job — model="" ⇒
//   manifest default_model; provider="" ⇒ Registry.DefaultProvider (FR-D1). ResolveRoleModel returns
//   ("","") when nothing set a value. Document this in the doc comment. Do NOT import internal/provider.
//   (design-decisions §4)

// GOTCHA (unknown role ⇒ global fallback, via map zero-value): `cfg.Roles[role]` for a non-key returns the
//   zero RoleConfig{} with ok==false → falls through to global. Correct & intended (an unknown role inherits
//   global). No special handling. Add a test (non-canonical name). (design-decisions §5)

// GOTCHA (roles.go needs NO imports): the body is pure map access + string assignment. `go mod tidy` MUST be
//   a no-op. roles_test.go uses only `testing`. (design-decisions §8)

// GOTCHA (roles_test.go is white-box `package config`): same package as roles.go/config.go/load.go, so it can
//   reference `roleNames` (load.go) for one canonical-roles iteration test. The explicit per-case tests
//   hardcode their role strings (self-documenting; immune to roleNames reorder). (design-decisions §7)
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/roles.go — NO new types. ONE new function. NO imports.
// (RoleConfig + Config.Roles/Provider/Model are already defined in config.go, M3.T1.S1 — read-only here.)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/config/roles.go — ResolveRoleModel
  - WRITE the file with `package config`, NO imports, and:
      * The doc comment (cite PRD §16.4 / §9.15 FR-R3; state the precedence flag/env/[role.*] in cfg.Roles >
        global cfg.Provider/cfg.Model > manifest-default sentinel; document ("","") ⇒ manifest defaults:
        model="" ⇒ manifest default_model, provider="" ⇒ auto-detect).
      * `func ResolveRoleModel(role string, cfg Config) (provider, model string)` — body EXACTLY delta §4:
          provider, model = "", ""
          if rc, ok := cfg.Roles[role]; ok {
              if rc.Provider != "" { provider = rc.Provider }
              if rc.Model    != "" { model    = rc.Model    }
          }
          if provider == "" { provider = cfg.Provider }
          if model    == "" { model    = cfg.Model    }
          return provider, model
  - GOTCHA: cfg BY VALUE (not *Config); named returns; NO error return; NO imports.
  - GOTCHA: per-field independence (the two rc checks + two fallbacks are independent — do NOT couple).

Task 2: CREATE internal/config/roles_test.go — the 8 cases (white-box package config)
  - WRITE the file with `package config`, import only `"testing"`. Construct Config via Defaults() + manual
    field sets (do NOT route through Load). One t.Errorf per assertion (mirror config_test.go's TestDefaults).
  - CASES (each a distinct Test function):
      * TestResolveRoleModel_GlobalFallbackRolesNil — cfg.Roles nil, global {pi, gpt-5.4} → (pi, gpt-5.4).
      * TestResolveRoleModel_GlobalFallbackRoleAbsent — cfg.Roles has OTHER roles but not the queried one →
        global (same code path as Roles-nil; also covers the unknown-role case).
      * TestResolveRoleModel_FullOverride — cfg.Roles["planner"]={agy,gemini-2.5-pro}, global {pi,""} →
        (agy, gemini-2.5-pro).
      * TestResolveRoleModel_ModelOnlyOverride — cfg.Roles["message"]={"",gpt-5.4-nano}, global {pi,""} →
        (pi, gpt-5.4-nano)  [provider inherits global].
      * TestResolveRoleModel_ProviderOnlyOverride — cfg.Roles["stager"]={agy,""}, global {pi,gpt-5.4} →
        (agy, gpt-5.4)  [model inherits global].
      * TestResolveRoleModel_BothEmptyManifestSentinel — global {"",""}, Roles nil → ("","").
      * TestResolveRoleModel_UnknownRoleFallsBackToGlobal — query "palnner" (typo), global {pi,gpt-5.4} →
        (pi, gpt-5.4)  [non-canonical name → global via map zero-value].
      * TestResolveRoleModel_AllCanonicalRoles — for each role in roleNames: with a per-role override it wins;
        without it, global wins. (Iterates roleNames — proves all four resolve. References load.go's roleNames.)
  - GOTCHA: white-box `package config`; reference roleNames ONLY in the iteration test; hardcode role strings
    in the explicit cases. Do NOT call Load() — construct cfg directly.

Task 3: VERIFY (no further edits)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. config.go + load.go + file.go +
    git.go + every non-target file byte-unchanged. `go build ./... && go test ./...` green.
```

### Implementation Patterns & Key Details

```go
// === internal/config/roles.go — THE FUNCTION (body is delta §4 verbatim) ===

package config // no imports needed

// ResolveRoleModel returns the (provider, model) for a single agent role (PRD §16.4, §9.15 FR-R1–R3),
// applying the precedence:
//
//   CLI flag > env > [role.<role>] config   (all already merged into cfg.Roles by the loaders)
//   > [defaults] global                     (cfg.Provider / cfg.Model)
//   > built-in manifest default             (the ("","") sentinel — see below)
//
// By the time this runs, Load() has already overlaid every precedence layer into cfg: the per-role flag/env/
// file/git values are per-field-merged into cfg.Roles[role], and the global layers into cfg.Provider/
// cfg.Model. So this function only checks the per-role entry, then falls back to the global for any field
// still empty. It does NOT re-walk the layers and does NOT consult any manifest.
//
// Provider and Model are resolved INDEPENDENTLY (per-field, FR-R3/FR37a): a role that sets only its Model
// inherits the global Provider, and vice versa. A role absent from cfg.Roles inherits the global entirely.
//
// The returned ("", "") is the "use manifest defaults" sentinel for the downstream consumer (the registry /
// Render): model == ""  => use the resolved provider manifest's default_model; provider == "" => the registry
// applies auto-detection (Registry.DefaultProvider, FR-D1). ResolveRoleModel deliberately does NOT resolve
// the manifest layer itself — that is the registry's job (config must not import internal/provider).
//
// On the single-commit path the only active role is "message"; with no per-role override this returns
// (cfg.Provider, cfg.Model) — exactly v1 (back-compatible).
//
// role is an arbitrary string (one of "planner","stager","message","arbiter" in practice); a non-canonical
// name simply misses the cfg.Roles lookup and inherits the global (no error).
func ResolveRoleModel(role string, cfg Config) (provider, model string) {
	if rc, ok := cfg.Roles[role]; ok {
		if rc.Provider != "" {
			provider = rc.Provider
		}
		if rc.Model != "" {
			model = rc.Model
		}
	}
	if provider == "" {
		provider = cfg.Provider
	}
	if model == "" {
		model = cfg.Model
	}
	return provider, model
}
```

```go
// === internal/config/roles_test.go — white-box package config; construct Config directly; one t.Errorf/assertion ===

package config

import "testing"

func TestResolveRoleModel_GlobalFallbackRolesNil(t *testing.T) {
	cfg := Defaults()              // Roles == nil, Provider/Model == ""
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	p, m := ResolveRoleModel("message", cfg)
	if p != "pi" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(message) = (%q,%q), want (pi,gpt-5.4) [global fallback, Roles nil]", p, m)
	}
}

func TestResolveRoleModel_GlobalFallbackRoleAbsent(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	cfg.Roles = map[string]RoleConfig{"planner": {Provider: "agy"}} // other roles set, but not "message"
	p, m := ResolveRoleModel("message", cfg)
	if p != "pi" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(message) = (%q,%q), want (pi,gpt-5.4) [role absent ⇒ global]", p, m)
	}
}

func TestResolveRoleModel_FullOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi" // global
	cfg.Roles = map[string]RoleConfig{
		"planner": {Provider: "agy", Model: "gemini-2.5-pro"},
	}
	p, m := ResolveRoleModel("planner", cfg)
	if p != "agy" || m != "gemini-2.5-pro" {
		t.Errorf("ResolveRoleModel(planner) = (%q,%q), want (agy,gemini-2.5-pro) [full override]", p, m)
	}
}

func TestResolveRoleModel_ModelOnlyOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi" // global provider
	cfg.Roles = map[string]RoleConfig{
		"message": {Provider: "", Model: "gpt-5.4-nano"}, // model-only override
	}
	p, m := ResolveRoleModel("message", cfg)
	if p != "pi" || m != "gpt-5.4-nano" {
		t.Errorf("ResolveRoleModel(message) = (%q,%q), want (pi,gpt-5.4-nano) [model-only: provider inherits global]", p, m)
	}
}

func TestResolveRoleModel_ProviderOnlyOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4" // global model
	cfg.Roles = map[string]RoleConfig{
		"stager": {Provider: "agy", Model: ""}, // provider-only override
	}
	p, m := ResolveRoleModel("stager", cfg)
	if p != "agy" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(stager) = (%q,%q), want (agy,gpt-5.4) [provider-only: model inherits global]", p, m)
	}
}

func TestResolveRoleModel_BothEmptyManifestSentinel(t *testing.T) {
	cfg := Defaults() // Roles nil, Provider/Model ""
	p, m := ResolveRoleModel("planner", cfg)
	if p != "" || m != "" {
		t.Errorf("ResolveRoleModel(planner) = (%q,%q), want (\"\",\"\") [manifest-default sentinel]", p, m)
	}
}

func TestResolveRoleModel_UnknownRoleFallsBackToGlobal(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	p, m := ResolveRoleModel("palnner", cfg) // typo / non-canonical name
	if p != "pi" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(palnner) = (%q,%q), want (pi,gpt-5.4) [unknown role ⇒ global]", p, m)
	}
}

func TestResolveRoleModel_AllCanonicalRoles(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	// Override only planner + stager; leave message + arbiter on the global.
	cfg.Roles = map[string]RoleConfig{
		"planner": {Provider: "agy", Model: "gemini-2.5-pro"},
		"stager":  {Provider: "agy", Model: "gemini-2.5-flash"},
	}
	want := map[string][2]string{
		"planner": {"agy", "gemini-2.5-pro"},   // overridden
		"stager":  {"agy", "gemini-2.5-flash"}, // overridden
		"message": {"pi", "gpt-5.4"},           // global
		"arbiter": {"pi", "gpt-5.4"},           // global
	}
	for _, role := range roleNames { // roleNames is load.go's package-level canonical list (same package)
		p, m := ResolveRoleModel(role, cfg)
		w := want[role]
		if p != w[0] || m != w[1] {
			t.Errorf("ResolveRoleModel(%s) = (%q,%q), want (%q,%q)", role, p, m, w[0], w[1])
		}
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): change NONE. roles.go needs NO imports (pure map access + string assignment);
      roles_test.go imports only "testing". `go mod tidy` MUST be a no-op. `git diff --exit-code go.mod
      go.sum` MUST be empty.

PACKAGE EDGES: NONE added. config stays a leaf. Do NOT import internal/provider (import-cycle risk — provider
      consumes config; and manifest resolution is the registry's job, not this pure function's). roles.go has
      zero imports; roles_test.go has only "testing".

UPSTREAM CONTRACT (the inputs — already SHIPPED, just READ):
  - M3.T1.S1 (config.go): RoleConfig{Provider,Model}, Config.Roles map[string]RoleConfig, Config.Provider/
        Model. FROZEN — read only.
  - M3.T1.S2 (file.go): overlay()/materialize() per-field-merge the [role.<role>] file tables + git config into
        cfg.Roles. DONE.
  - P1.M3.T2.S1 (load.go): setRoleProvider/setRoleModel + loadEnv/loadFlags per-role loops merge the flag/env
        layers into cfg.Roles (the highest layers); Load() orchestrates all layers. DONE. Also exports
        package-level `roleNames` (reusable in roles_test.go).

DOWNSTREAM CONTRACTS (the consumers — do NOT implement here, just honor the return semantics):
  - P3.M2.T1.S1 (internal/decompose/roles.go, FUTURE): for each role calls
        `provider, model := config.ResolveRoleModel(role, cfg)` then asks the registry to resolve/build the
        manifest for that (provider, model). model=="" ⇒ manifest default_model; provider=="" ⇒ the registry's
        DefaultProvider auto-detect (FR-D1).
  - The CLI (single-commit path): resolves the "message" role; with no per-role override returns
        (cfg.Provider, cfg.Model) — v1 back-compatible.

FROZEN/LEAVE (do NOT edit):
  - internal/config/config.go (+_test.go) — M3.T1.S1 SHIPPED.
  - internal/config/file.go (+_test.go), git.go (+_test.go) — M3.T1.S2 + others.
  - internal/config/load.go (+_test.go) — P1.M3.T2.S1 DONE.
  - internal/provider/*, internal/git/*, internal/prompt/*, internal/generate/*, internal/ui/*,
    internal/cmd/*, pkg/*, cmd/*.
  - PRD.md, Makefile, providers/*.toml, docs/*.

NO NEW DATABASE / ROUTES / CLI COMMANDS (the two NEW files are a pure function + its tests).
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/config/roles.go internal/config/roles_test.go
go vet ./internal/config/
# Confirm roles.go has NO imports (pure) and defines ResolveRoleModel:
head -5 internal/config/roles.go   # expect: package config  + blank line + doc comment (NO import block)
grep -n 'func ResolveRoleModel' internal/config/roles.go
# Confirm go.mod/go.sum unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; roles.go has no import block; ResolveRoleModel present; go.mod/go.sum byte-unchanged.
```

### Level 2: Config-package unit tests (the new roles_test.go + no regression)

```bash
go test ./internal/config/ -run 'ResolveRoleModel' -v
# Expected PASS — verify explicitly:
#   TestResolveRoleModel_GlobalFallbackRolesNil ...... Roles nil ⇒ (cfg.Provider, cfg.Model)
#   TestResolveRoleModel_GlobalFallbackRoleAbsent .... role absent in cfg.Roles ⇒ global
#   TestResolveRoleModel_FullOverride ................ per-role {agy,gemini-2.5-pro} beats global
#   TestResolveRoleModel_ModelOnlyOverride ........... model-only override; provider inherits global
#   TestResolveRoleModel_ProviderOnlyOverride ........ provider-only override; model inherits global
#   TestResolveRoleModel_BothEmptyManifestSentinel ... everything empty ⇒ ("","")
#   TestResolveRoleModel_UnknownRoleFallsBackToGlobal  non-canonical name ⇒ global
#   TestResolveRoleModel_AllCanonicalRoles ........... all four roles resolve (overrides win; others global)
# Then the FULL config suite (no regression in Defaults/loadEnv/loadFlags/file/git tests):
go test ./internal/config/ -v
# Expected: all PASS — the existing ~30 tests stay green (this subtask only ADDS a file + tests; touches nothing).
# If TestResolveRoleModel_ModelOnlyOverride fails (provider != "pi"), the two rc checks are coupled — make
# them independent (design-decisions §3).
# If TestResolveRoleModel_BothEmptyManifestSentinel fails, the function is reaching into a manifest/default
# instead of returning ("","") — it must NOT (design-decisions §4).
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...     # Expect clean (purely additive: one new function, no new imports).
go test ./...      # Expect all PASS — nothing else depends on ResolveRoleModel yet, so no downstream effect.
# Confirm the LEAVE files are byte-unchanged:
git diff --exit-code internal/config/config.go internal/config/config_test.go internal/config/file.go \
  internal/config/file_test.go internal/config/git.go internal/config/git_test.go \
  internal/config/load.go internal/config/load_test.go \
  internal/provider internal/git internal/prompt internal/generate internal/ui \
  internal/cmd cmd pkg Makefile go.mod go.sum PRD.md \
  && echo "frozen files UNCHANGED (expected)"
# Confirm ONLY the two NEW files appear (untracked), nothing modified:
git status --porcelain internal/config/
# Expected: two untracked files (roles.go, roles_test.go); NO modified files anywhere.
```

### Level 4: Correctness reasoning (no runtime to start)

```bash
# This subtask is a pure function over cfg — no server/DB/subprocess/git. Verify by reasoning + the tests:
#   1. Precedence collapse: cfg.Roles[role] holds the per-field-merged flag/env/file/git values (load.go +
#      file.go merged them); cfg.Provider/cfg.Model hold the global. So checking Roles then falling back to
#      global IS the full 5-layer precedence (manifest default = the ("","") sentinel). [design-decisions §2]
#   2. Per-field independence: provider and model each resolve independently — a model-only override keeps the
#      global provider, a provider-only override keeps the global model. [Test_ModelOnly/ProviderOnly]
#   3. ("","") sentinel: when nothing is set, returns ("","") for the registry/Render to interpret (model="" ⇒
#      manifest default_model; provider="" ⇒ auto-detect). The function does NOT resolve the manifest.
#      [Test_BothEmptyManifestSentinel]
#   4. Unknown role: non-canonical name misses the map ⇒ global fallback (no error). [Test_UnknownRole]
#   5. Back-compat: "message" with no override ⇒ (cfg.Provider, cfg.Model) = v1. [Test_GlobalFallback*]
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/` clean.
- [ ] `go test ./...` PASS (config suite incl. the 8 new ResolveRoleModel tests + no repo-wide regression).
- [ ] go.mod/go.sum byte-unchanged.
- [ ] config.go + load.go + file.go + git.go (+tests) + every non-target file byte-unchanged; PRD.md byte-unchanged.

### Feature Validation
- [ ] `ResolveRoleModel(role, cfg)` exists with signature `(role string, cfg Config) (provider, model string)`
      — cfg by value, named returns, no error, no imports.
- [ ] Per-role full override beats global; model-only override keeps global provider; provider-only override
      keeps global model.
- [ ] `("", "")` returned when nothing is set (manifest-default sentinel); the function does NOT resolve the
      manifest (no internal/provider import).
- [ ] Unknown/non-canonical role ⇒ global fallback (no error).
- [ ] All four canonical roles resolve correctly (override wins; others inherit global).
- [ ] The 8 new tests pass; the existing ~30 config tests stay green.

### Code Quality Validation
- [ ] Follows existing conventions: value-semantic signature (mirrors `Defaults()`); white-box
      `package config` tests mirroring `TestDefaults` (construct Config directly, one t.Errorf/assertion);
      doc comment cites PRD §16.4 / §9.15 FR-R3 and the precedence + sentinel semantics.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (LEAVE files untouched); no manifest/import-
      cycle creep.

### Documentation
- [ ] roles.go doc comment cites PRD §16.4 / §9.15 FR-R3, states the 3-level precedence (per-role in cfg.Roles
      > global > manifest-default sentinel), and documents `("", "")` ⇒ manifest defaults (model="" ⇒
      default_model; provider="" ⇒ auto-detect) so consumers (P3.M2.T1.S1, the CLI) know the contract.

---

## Anti-Patterns to Avoid

- ❌ **Don't edit config.go / load.go / file.go / git.go.** config.go is M3.T1.S1 (FROZEN); load.go is
  P1.M3.T2.S1 (DONE); file.go/git.go are M3.T1.S2+. ResolveRoleModel is a NEW file (roles.go). (§0)
- ❌ **Don't take cfg by pointer or add an error return.** The signature is `(role string, cfg Config) (provider,
  model string)` — value in, two named strings out, no error. Every input has a defined output. (§1)
- ❌ **Don't re-walk the 7 precedence layers or reach into the manifest.** The loaders already merged them into
  cfg.Roles (per-field) and cfg.Provider/cfg.Model. Checking Roles then global IS the full precedence. The
  manifest layer is the `("", "")` sentinel for the registry, NOT resolved here. (§2/§4)
- ❌ **Don't couple provider and model resolution.** The two `if rc.X != ""` are independent; the two
  `if x == ""` fallbacks are independent. Coupling them drops model-only/provider-only overrides. (§3)
- ❌ **Don't import internal/provider.** That risks an import cycle (provider consumes config) and pushes
  manifest/validation logic into the wrong package. FR-R5/FR-R5b validation is the registry's job. (§6)
- ❌ **Don't route the tests through Load().** Construct Config directly via `Defaults()` + manual field sets —
  ResolveRoleModel is a pure function of cfg; Load() obscures the unit under test. (§9)
- ❌ **Don't add a third-party import (or any import) to roles.go.** The body is pure map access + string
  assignment. `go mod tidy` must be a no-op. (§8)
- ❌ **Don't special-case unknown roles or canonical-role validation.** An unknown role simply misses the map
  and inherits global — no error, no validation. Canonical-role enforcement is the caller's concern. (§5)
