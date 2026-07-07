---
name: "P3.M2.T1.S1 — Implement internal/decompose/roles.go: resolve four role manifests from config (PRD §13.6.2, §9.15 FR-R1–R5b, §9.16 FR-D4)"
description: |

  CREATE ONE NEW PACKAGE `internal/decompose/` and ONE NEW FILE `roles.go` in it, PLUS ONE SMALL
  ADDITIVE METHOD on `internal/provider/registry.go`. roles.go is the role-resolution half of the
  multi-commit decomposition pipeline (PRD §13.6.2): it turns the fully-resolved `config.Config`
  (the 7-layer precedence already applied) + a `*provider.Registry` into four resolved
  `provider.Manifest`s (planner/stager/message/arbiter) + four resolved `(provider, model)` pairs
  (`config.RoleConfig` each), applying the stager TooledFlags fallback (FR-D4) and the FR-R5b
  model-without-provider guard. The decompose orchestrator (P3.M4.T1.S1) calls `ResolveRoles` to
  build the injectable `Deps`.

  CONTRACT (P3.M2.T1.S1, verbatim from the work item):
    1. RESEARCH NOTE: Each of the four roles independently resolves its provider+model via
       config.ResolveRoleModel (P1.M3.T2.S2), then looks up the manifest from the Registry, validates
       it, and checks IsInstalled. For bare roles (planner/message/arbiter) the manifest's BareFlags
       are used (at Render time). For the stager, TooledFlags must be non-empty — if the resolved
       stager provider has empty TooledFlags, fall back to the next-priority provider that CAN
       (FR-D4 note). FR-R5b: for multi-provider agents, if model is set but provider is empty, surface
       a config error. The Deps struct carries Git, the Registry, Config, resolved RoleManifests, and
       Verbose — injectable for testing with stub manifests.
    2. INPUT: config.ResolveRoleModel from P1.M3.T2.S2, provider.Registry from existing code.
    3. LOGIC: Create internal/decompose/roles.go. Define `type RoleManifests struct { Planner, Stager,
       Message, Arbiter provider.Manifest }` and `type RoleModels struct { Planner, Stager, Message,
       Arbiter config.RoleConfig }`. Define `type Deps struct { Git git.Git; Registry *provider.Registry;
       Config config.Config; Roles RoleManifests; Verbose *ui.Verbose }`. Implement
       `func ResolveRoles(cfg config.Config, reg *provider.Registry) (RoleManifests, RoleModels, error)`
       — for each role: call config.ResolveRoleModel, get manifest from reg.Get, Validate, IsInstalled
       check, stager tooled_flags check (fall back if empty). Return error on missing provider,
       uninstalled command, or model-without-provider for multi-provider agents.
    4. OUTPUT: decompose/roles.go exports Deps, RoleManifests, RoleModels, and ResolveRoles(). The
       decompose orchestrator (P3.M4.T1.S1) calls ResolveRoles to build Deps.
    5. DOCS: none — internal resolution logic.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/config/roles.go (ResolveRoleModel) — CONSUMED as-is; its ("","") sentinel = "manifest
      defaults" is the load-bearing contract.
    - internal/provider/registry.go — ADD ONE additive method only (FirstTooledProvider); do NOT change
      any existing signature (DefaultProvider/Get/List/IsInstalled/NewRegistry/DecodeUserOverrides).
    - internal/provider/{manifest,merge,builtin,render}.go — CONSUMED (Manifest fields, MergeManifest
      override semantics, RenderBare/RenderTooled); UNCHANGED.
    - internal/git/git.go, internal/ui/verbose.go — CONSUMED for the Deps struct field TYPES only;
      UNCHANGED.
    - internal/decompose/{planner,stager,message,arbiter,chain,decompose}.go — DO NOT EXIST YET. This
      task creates ONLY roles.go (+ roles_test.go). P3.M2.T2/T3/T4 + P3.M3.* + P3.M4.* own the rest.
    - pkg/stagecoach — UNCHANGED (the public Decompose API is P4.M2.T1.S1).

  DELIVERABLES (3 new files, 1 tiny additive method, 0 breaking changes):
    CREATE internal/decompose/roles.go — package `decompose`; package doc; the 3 structs (RoleManifests,
      RoleModels, Deps); ResolveRoles(cfg, reg) (RoleManifests, RoleModels, error); private helpers
      (computeInstalled, isMultiProvider, the 4-role resolution loop, the stager fallback).
    CREATE internal/decompose/roles_test.go — ResolveRoles table tests (happy 4-role resolve; stager
      TooledFlags fallback; FR-R5b bare-model guard; missing-provider; unknown-provider; uninstalled;
      no-stager-capable fallback) using Command="go" override manifests for deterministic IsInstalled.
    ADD func (r *Registry) FirstTooledProvider(installed []string) string to internal/provider/registry.go
      — additive; mirrors DefaultProvider; walks preferredBuiltins for the first name in `installed`
      whose manifest has non-empty TooledFlags, else "".

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass; the 4 built-in
  roles resolve; the stager falls back from a TooledFlags-less provider to a capable one (FR-D4); a bare
  model on a multi-provider (pi) agent surfaces a config error (FR-R5b); Deps/RoleManifests/RoleModels
  are exactly as the contract specifies.

---

## Goal

**Feature Goal**: Implement the per-role manifest/model resolution layer for multi-commit decomposition
(PRD §13.6.2 four-agent pipeline) as a self-contained module `internal/decompose/roles.go`. ResolveRoles
turns `(config.Config, *provider.Registry)` into four resolved `provider.Manifest`s + four resolved
`config.RoleConfig` pairs (one per role: planner/stager/message/arbiter), independently resolving each
role's provider+model via `config.ResolveRoleModel`, validating + IsInstalled-checking each manifest,
applying the stager TooledFlags fallback (FR-D4 — a TooledFlags-less stager provider falls back to the
next-priority installed provider that CAN stage), and surfacing the FR-R5b misconfiguration (a model set
with no provider on a multi-provider agent). It is the 4-role generalization of v1's
`pkg/stagecoach.buildDeps` (one role: message).

**Deliverable** (2 new files in a new package + 1 tiny additive method):
1. `internal/decompose/roles.go` (package `decompose`) — `RoleManifests` struct (4 Manifests),
   `RoleModels` struct (4 RoleConfig), `Deps` struct (Git/Registry/Config/Roles/Verbose), and
   `ResolveRoles(cfg config.Config, reg *provider.Registry) (RoleManifests, RoleModels, error)`.
2. `internal/decompose/roles_test.go` — table + edge-case tests.
3. `internal/provider/registry.go` — ADD `FirstTooledProvider(installed []string) string` (additive).

**Success Definition**:
- `ResolveRoles` with a config whose provider resolves to "pi" (or auto-detects "pi") returns
  RoleManifests with all 4 manifests == pi's manifest (when no per-role override) and RoleModels all
  `{Provider:"pi", Model:<resolved>}`; nil error.
- Stager fallback: when the resolved stager manifest has empty `TooledFlags`, ResolveRoles switches
  Stager to the first installed preferred provider with non-empty TooledFlags (pi→claude if pi lacked
  them; in practice gemini/agy stager → pi/claude), AND switches Stager's model to that provider's
  FR-D4 stager model (`config.DefaultModelsForProvider(fb)["stager"]`). If none is installed+capable →
  non-nil error.
- FR-R5b: a role whose `ResolveRoleModel` returns `(provider=="", model!="")` AND whose auto-detected
  manifest is multi-provider (`*Manifest.ProviderFlag != ""`, i.e. pi) → non-nil config error. A role
  with provider set (the config-init default) → no error even with a bare model (out-of-scope sub-
  provider conflation — documented).
- Missing provider (`ResolveRoleModel` provider=="" AND auto-detect yields "") → non-nil error.
  Unknown provider (`reg.Get` miss) → non-nil error. Uninstalled command (`IsInstalled`==false) →
  non-nil error.
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 3 changes (2 new
  files + the registry.go method), nothing else.

## User Persona

**Target User**: the decompose orchestrator (`internal/decompose/decompose.go`, P3.M4.T1.S1) and, by
extension, the end user running `stagecoach` on an un-staged working tree to get N logically-coherent
commits. roles.go is internal plumbing — NOT user-facing CLI text. The user configures per-role
provider/model (`--planner-model`, `[role.stager].provider`, …) or relies on the config-init defaults;
ResolveRoles is the layer that turns that config into concrete, validated, install-checked manifests.

**Use Case**: before the planner→stager→message→arbiter pipeline runs, the orchestrator calls
`ResolveRoles(cfg, reg)` ONCE to (a) validate every role's provider is known + installed + valid
(fail-fast BEFORE any git mutation — same pre-flight discipline as buildDeps), (b) resolve the stager's
tooled manifest with the FR-D4 fallback, and (c) hand back the four manifests + four (provider,model)
pairs the loop consumes. A misconfiguration (uninstalled provider, ambiguous bare model, no stager-
capable provider) surfaces as a clear error here, not as a confusing mid-run failure.

**Pain Points Addressed**: (a) the four roles must resolve independently (FR-R1) so a user can run a
flagship planner on a cheap message agent; (b) the stager REQUIRES a tooled provider but only pi+claude
are tooled-capable today — silent misconfiguration (stager on agy) would produce a Render-time error
deep in the pipeline; ResolveRoles fails fast with the FR-D4 fallback instead; (c) a bare model on pi
(glm-5-turbo with no provider) silently misroutes to the wrong upstream (FR-R5b) — ResolveRoles surfaces
it as a config error.

## Why

- **Closes the role-resolution half of PRD §13.6.2 / §9.15 FR-R1–R5b / §9.16 FR-D4.** The four roles
  (planner/stager/message/arbiter) each resolve independently (FR-R1). This task is the literal
  resolution+validation implementation — the bridge between the already-resolved `config.Config`
  (7-layer precedence done by the loaders) and the `provider.Manifest`s the decompose loop consumes.
  With it, the decompose pipeline has its Deps builder; P3.M2.T2/T3/T4 (planner/stager/message) can
  assume a validated, install-checked set of role manifests.
- **Generalizes the proven v1 pattern (buildDeps → ResolveRoles).** `pkg/stagecoach.buildDeps`
  (stagecoach.go) already does this for ONE role (message): overrides→registry→auto-detect→Get→Validate→
  IsInstalled→error-wrapping. ResolveRoles is the same algorithm × 4 roles, plus the stager fallback and
  the FR-R5b guard — no new architectural concept. The installed computation, the auto-detect path, and
  the error wording SHOULD mirror buildDeps for cohesion (findings §7).
- **Unblocks the decompose pipeline (P3.M2–P3.M4).** Every decompose subtask (planner.go, stager.go,
  message loop, arbiter.go, the orchestrator) consumes `Deps` + the resolved manifests/models. None can
  run until ResolveRoles exists. This is the foundation file for the entire `internal/decompose/`
  package.
- **Lowest-risk, maximal-reuse, backward-compatible.** Two NEW files in a NEW package + one ADDITIVE
  method (no existing signature changes → no existing caller/test breaks). go.mod/go.sum untouched
  (stdlib + config/provider/git/ui, all already imported elsewhere). No import cycle (config does NOT
  import provider — roles.go bridges the strings↔manifests boundary, exactly as buildDeps does).

## What

One new package `internal/decompose` with one file `roles.go` exporting three structs and one function,
plus private helpers; one new test file `roles_test.go`; and one additive method on
`internal/provider/registry.go`. No new dependencies. No caller wiring (that is P3.M4.T1.S1).
Specifically:

- **`RoleManifests`** (exported struct): `{Planner, Stager, Message, Arbiter provider.Manifest}`. Each
  field is the MERGED-but-unresolved manifest from `reg.Get` (Resolve happens later inside Render, which
  calls Validate+Resolve itself — same as buildDeps storing the unresolved manifest in Deps.Manifest).
  The orchestrator calls `RoleManifests.X.Render(...)` per role.
- **`RoleModels`** (exported struct): `{Planner, Stager, Message, Arbiter config.RoleConfig}`. Each field
  carries the RESOLVED `(Provider, Model)` pair for that role (post-auto-detect, post-stager-fallback).
  The orchestrator passes `RoleModels.X.Model` + `RoleModels.X.Provider` as the Render model/provider
  params. (config.RoleConfig is `{Provider, Model string}` — the natural carrier.)
- **`Deps`** (exported struct): `{Git git.Git; Registry *provider.Registry; Config config.Config; Roles
  RoleManifests; Verbose *ui.Verbose}`. Injectable for testing with stub manifests — a test sets
  `Deps.Roles` directly and skips ResolveRoles, exactly as generate tests set `Deps.Manifest`.
- **`ResolveRoles(cfg config.Config, reg *provider.Registry) (RoleManifests, RoleModels, error)`**:
  computes `installed` once (iterate `reg.List()` + `reg.IsInstalled`), then for each of the four roles:
  (1) `prov, mdl := config.ResolveRoleModel(role, cfg)`; capture `bareModelNoProvider := prov=="" &&
  mdl!=""`; (2) if prov=="" → `prov = reg.DefaultProvider(installed)`; (3) if prov=="" → error "missing
  provider"; (4) `m, ok := reg.Get(prov)`; if !ok → error "unknown provider"; (5) `m.Validate()`; (6)
  `if !reg.IsInstalled(m)` → error "command not found"; (7) FR-R5b: if `bareModelNoProvider &&
  isMultiProvider(m)` → error; (8) stager fallback: if role=="stager" && `len(m.TooledFlags)==0` →
  `fb := reg.FirstTooledProvider(installed)`; if fb=="" → error "no stager-capable provider"; else set
  prov=fb, m=reg.Get(fb), mdl=config.DefaultModelsForProvider(fb)["stager"] ("" if none); (9) store into
  the right RoleManifests/RoleModels field. Returns the two structs + nil, or a wrapped non-nil error.
- **`isMultiProvider(m provider.Manifest) bool`** (private): `m.ProviderFlag != nil && *m.ProviderFlag
  != ""`. Only pi today (findings §6). Documented decision.
- **`computeInstalled(reg) []string`** (private): iterate `reg.List()`, append `m.Name` where
  `reg.IsInstalled(m)` — mirrors buildDeps.
- **`Registry.FirstTooledProvider(installed []string) string`** (ADDITIVE, in registry.go): identical
  structure to `DefaultProvider` — build a `present` set from `installed`, walk `preferredBuiltins`,
  return the first name that is present AND whose `reg.Get(name)` manifest has `len(TooledFlags) > 0`,
  else "". Drift-free (reuses the unexported `preferredBuiltins`).

### Success Criteria

- [ ] `internal/decompose/roles.go` is package `decompose`, has a package doc comment citing PRD §13.6.2,
      and defines `RoleManifests`, `RoleModels`, `Deps`, `ResolveRoles` EXACTLY as the contract (field
      names Planner/Stager/Message/Arbiter; types provider.Manifest / config.RoleConfig / etc.).
- [ ] `ResolveRoles` resolves all 4 roles; with a pi-config + no per-role override, all 4 manifests ==
      pi's merged manifest and all 4 RoleModels.Provider == "pi" (Model per the table/global).
- [ ] Stager fallback: a TooledFlags-less stager provider is replaced by the first installed preferred
      provider with non-empty TooledFlags, with Stager.Provider/Model updated accordingly; if none,
      ResolveRoles returns a non-nil error naming the role.
- [ ] FR-R5b: `ResolveRoleModel`→("", non-empty model) on a multi-provider (ProviderFlag!="") manifest →
      non-nil config error; a provider-bearing role with a bare model → NO error (documented limitation).
- [ ] Missing provider (no auto-detect hit), unknown provider (Get miss), and uninstalled command
      (IsInstalled false) each yield a distinct, role-named, wrapped non-nil error.
- [ ] `FirstTooledProvider` is additive on registry.go (DefaultProvider/Get/List/IsInstalled signatures
      unchanged) and returns the priority-first installed TooledFlags-capable built-in.
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; only 3 git changes (roles.go, roles_test.go, the
      registry.go method).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the contract (findings §1);
ResolveRoleModel's ("","") sentinel + per-field semantics (findings §2); the Registry surface + the
unexported-preferredBuiltins fact driving FirstTooledProvider (findings §3); the Manifest field model
(TooledFlags/ProviderFlag on the merged-but-unresolved reg.Get manifest — findings §4); the stager
fallback algorithm + the model-switch rule (findings §5); the FR-R5b decision + the ProviderFlag signal
+ the documented out-of-scope sub-provider conflation (findings §6); the buildDeps pattern to mirror
(findings §7); the Deps/RoleModels cohesion note for the orchestrator (findings §8); the no-cycle import
graph (findings §9); the Command="go" test-determinism trick + cross-package pointer construction
(findings §10); the validation gates (findings §11). No prior decompose knowledge required — the module
is fully self-contained at the resolution layer.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (contract + 11 sections of load-bearing facts)
- docfile: plan/002_a17bb6c8dc1d/P3M2T1S1/research/findings.md
  why: §1 the verbatim contract + scope boundary; §2 ResolveRoleModel's ("","") sentinel + per-field
       (provider/model independent) semantics — the upstream contract; §3 the Registry surface + the
       CRITICAL "preferredBuiltins is UNEXPORTED" fact (drives the FirstTooledProvider decision); §4 the
       Manifest field model on the merged-but-unresolved reg.Get manifest (TooledFlags nil/empty check;
       ProviderFlag nil-safe multi-provider signal; Validate is safe pre-Resolve; Resolve is Render's
       job, NOT ResolveRoles'); §5 the stager fallback algorithm + the model-switch-via-
       DefaultModelsForProvider rule; §6 the FR-R5b DECISION (ProviderFlag signal = only pi; WHY
       opencode/agy are excluded; the bareModelNoProvider capture; the out-of-scope sub-provider
       conflation); §7 buildDeps — the pattern to mirror (installed computation, auto-detect, error
       wording); §8 the Deps/RoleModels cohesion note; §9 no import cycle; §10 test determinism; §11 gates.
  critical: §3 (preferredBuiltins unexported → MUST add FirstTooledProvider to registry.go OR you cannot
            respect priority order; Reg.List() is alphabetical = WRONG priority); §6 (the FR-R5b check
            MUST use the ProviderFlag signal, NOT the literal pi/opencode/agy list, or it FALSE-POSITIVES
            on opencode/agy's normal empty-provider usage and breaks defaults); §4 (check TooledFlags +
            ProviderFlag on the reg.Get manifest directly — it is merged but NOT Resolve'd; Resolve is
            Render's job).

# MUST READ — the CLOSEST PATTERN: buildDeps (the 1-role analog to mirror for cohesion)
- file: pkg/stagecoach/stagecoach.go
  section: buildDeps(cfg, repoDir) (generate.Deps, error) — the v1 single-commit manifest resolver.
           Steps: DecodeUserOverrides(cfg.Providers) → NewRegistry(overrides); name:=cfg.Provider;
           if name=="" compute installed (reg.List+IsInstalled) → reg.DefaultProvider(installed);
           if name=="" → "no provider configured" error; reg.Get(name) → "unknown provider" error;
           m.Validate(); pre-flight reg.IsInstalled(m) → "command not found" error; apply
           cfg.Output/StripCodeFence.
  why: ResolveRoles is buildDeps × 4 roles + stager fallback + FR-R5b. Mirror the installed computation,
       the auto-detect path, and the error wording (role-named). buildDeps stores the UNRESOLVED merged
       manifest in Deps.Manifest (Render Validate+Resolves later) — ResolveRoles does the SAME.
  pattern: compute installed ONCE (all 4 roles share it); for each role: ResolveRoleModel → (if prov=="")
           DefaultProvider → Get → Validate → IsInstalled → (stager) TooledFlags fallback → store. Wrap
           every error with the role name. Return the merged-but-unresolved manifest (Render Resolves).
  gotcha: buildDeps's "no provider" error hardcodes a 6-name list — reuse it or derive from
          preferredBuiltins for the multi-role version. Do NOT call Resolve() in ResolveRoles (Render
          owns that; storing an unresolved manifest matches buildDeps + keeps "providers show" truthful).

# MUST READ — ResolveRoleModel: the upstream resolver (the ("","") sentinel contract)
- file: internal/config/roles.go
  section: func ResolveRoleModel(role string, cfg Config) (provider, model string) — reads cfg.Roles[role]
           then falls back to cfg.Provider/cfg.Model for empty fields; returns ("","") sentinel ("use
           manifest defaults"); provider/model resolved INDEPENDENTLY; does NOT consult manifests.
  why: ResolveRoles calls this 4× (one per role). The ("","") sentinel is the load-bearing contract:
       provider=="" ⇒ ResolveRoles auto-detects via reg.DefaultProvider; model=="" ⇒ Render uses the
       manifest DefaultModel. Capture the PRE-auto-detect (prov,mdl) for the FR-R5b bareModelNoProvider
       check (prov=="" && mdl!=""), BEFORE overwriting prov with the auto-detected name.
  gotcha: ResolveRoleModel returns MANIFEST-LEVEL provider (pi/claude), NOT a sub-provider (zai/openai).
          config has no separate sub-provider field (cfg.Provider conflates both — a v1 quirk). The
          FR-R5b check can only catch "model set, NO provider anywhere, multi-provider manifest" — it
          CANNOT detect a wrong sub-provider when provider IS set. Documented out-of-scope (findings §6).

# MUST READ — the Registry surface + the unexported preferredBuiltins fact
- file: internal/provider/registry.go
  section: preferredBuiltins (line 14, UNEXPORTED); NewRegistry; Get; List (sorted by Name); IsInstalled
           (exec.LookPath on DetectCommand); DefaultProvider(installed) (walks preferredBuiltins for the
           first in `installed`).
  why: ResolveRoles uses Get/IsInstalled/DefaultProvider/List. The stager fallback needs PRIORITY order
           among TooledFlags-capable providers; preferredBuiltins is unexported and List() is
           alphabetical (wrong priority) ⇒ you MUST add FirstTooledProvider (a DefaultProvider twin that
           also filters on non-empty TooledFlags). Additive only — do NOT touch existing signatures.
  pattern: FirstTooledProvider mirrors DefaultProvider EXACTLY: build a present-set from `installed`,
           range preferredBuiltins, return the first present name whose reg.Get(name).TooledFlags is
           non-empty, else "". Reuses the unexported preferredBuiltins (drift-free; a test already pins
           its order — registry_test.go:29-35).

# MUST READ — the Manifest field model (TooledFlags + ProviderFlag on the reg.Get manifest)
- file: internal/provider/manifest.go
  section: the Manifest struct (TooledFlags []string; ProviderFlag *string; Validate; Resolve); the
           pointer-scalar design comment.
  why: from reg.Get the manifest is MERGED but NOT Validate'd/Resolve'd. ResolveRoles calls Validate
       (safe pre-Resolve — nil-tolerant on optionals, strict on Name+Command) but does NOT Resolve
       (Render Validate+Resolves later, like buildDeps). Check TooledFlags directly (len==0 ⇒ cannot
       stage; Resolve does not touch TooledFlags). Check ProviderFlag nil-safe (m.ProviderFlag != nil &&
       *m.ProviderFlag != "") for isMultiProvider.
  gotcha: Resolve() fills nil pointers — but ResolveRoles must NOT call it (storing unresolved matches
          buildDeps + Render re-Validates+Resolves). The TooledFlags/ProviderFlag checks work on the
          unresolved manifest directly (they are plain slice / the pointer is non-nil for every built-in).

# MUST READ — the builtin TooledFlags/ProviderFlag ground truth (verify at implementation)
- file: internal/provider/builtin.go
  section: builtinPi (TooledFlags non-empty; ProviderFlag "--provider"); builtinClaude (TooledFlags
           non-empty; ProviderFlag ""); builtinGemini/builtinAgy/builtinOpenCode/builtinCodex/
           builtinCursor (TooledFlags nil; ProviderFlag "").
  why: confirms ONLY pi+claude are stager-capable (non-empty TooledFlags) today, and ONLY pi is
       multi-provider by the ProviderFlag signal (ProviderFlag "--provider"). This is the ground truth
       isMultiProvider + the stager fallback rely on. VERIFY it has not drifted at implementation.

# MUST READ — MergeManifest override semantics (for test determinism)
- file: internal/provider/merge.go
  section: the THREE merge regimes (scalar pointer non-nil WINS; slice len>0 REPLACES; nil slice override
           PRESERVES base).
  why: roles_test.go builds Registries via NewRegistry(overrides) to drive deterministic IsInstalled +
       TooledFlags states. A `{Command: &"go"}` override on a built-in name → Command="go" (installed),
       all else inherited (incl. TooledFlags). An empty/nil slice override does NOT clear TooledFlags
       (len 0 = not overridden) — to get a TooledFlags-LESS provider in a test, use a built-in that is
       already nil (gemini/agy/...), not an empty override. Cross-package pointers: provider.strPtr is
       unexported — use `cmd := "go"; &cmd` (stubtest.go's local-strPtr pattern).

# MUST READ — config.DefaultModelsForProvider (the stager-fallback model source)
- file: internal/config/role_defaults.go
  section: DefaultModelsForProvider(name) map[string]string — returns a COPY of the FR-D4 provider×role
           column, or nil if name is not a built-in.
  why: on stager fallback the model MUST switch to the fallback provider's stager model (models are
       provider-specific — FR-R5). mdl = DefaultModelsForProvider(fb)["stager"] ("gpt-5.4-mini" for pi,
       "sonnet" for claude; "" if nil → manifest DefaultModel at Render). Read the value, do NOT mutate.

# MUST READ — the types Deps references (Git / Verbose — field TYPES only; UNCHANGED)
- file: internal/git/git.go
  section: the `Git` interface (the boundary Deps.Git's type). ResolveRoles does NOT call any Git method
           — Git is in Deps for the orchestrator. Read only the interface declaration for the type.
- file: internal/ui/verbose.go
  section: the `Verbose` struct (NewVerbose; nil-safe). Deps.Verbose's type; ResolveRoles does NOT use it.

# MUST READ — the design reference (the 4-role table + Deps shape)
- docfile: plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md
  section: "The Four Agent Roles" table (planner/stager bare; stager tooled; per-role provider/model);
           "Decompose Deps Structure (injectable for testing)" (the Deps + RoleManifests shapes).
  why: confirms the Deps shape (Roles RoleManifests) + that stager is the ONLY tooled role (TooledFlags)
       + that Deps is injectable for testing with stub manifests. ResolveRoles BUILDS Deps.Roles; it does
       NOT run the pipeline.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §13.6.2 (the four agent roles: planner/stager/message/arbiter; only stager tooled)
  why: the role set + the stager-tooled-only invariant ResolveRoles encodes (TooledFlags check is on the
       stager ALONE).
- url: PRD.md §9.15 FR-R1–R5b (per-role independent resolution; FR-R5b model-without-provider guard)
  why: FR-R1 (4 roles, independent) + FR-R5b (the guard). findings §6 is the faithful FR-R5b reading.
- url: PRD.md §9.16 FR-D4 (per-provider default-model table + the stager TooledFlags fallback note)
  why: the fallback contract ("a provider whose tooled_flags is empty cannot serve as the stager; the
       stager role falls back to the next-priority provider that can").
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  roles.go             # READ (CONSUMED): ResolveRoleModel(role, cfg) → (provider,model); ("","") sentinel.
  role_defaults.go     # READ (CONSUMED): DefaultModelsForProvider(name) → stager-fallback model source.
  config.go            # READ: Config + RoleConfig {Provider, Model} types; Roles map[string]RoleConfig.
internal/provider/
  registry.go          # READ + ADD ONE METHOD: Get/List/IsInstalled/DefaultProvider; preferredBuiltins
                       #   (UNEXPORTED); ADD FirstTooledProvider(installed). Existing sigs UNCHANGED.
  manifest.go          # READ (CONSUMED): Manifest (TooledFlags, ProviderFlag, Validate, Resolve).
  merge.go             # READ (test determinism): override merge regimes (scalar/slice/env).
  builtin.go           # READ (ground truth): only pi+claude have TooledFlags; only pi has ProviderFlag.
  render.go            # READ (understanding): RenderBare/RenderTooled; ProviderFlag governs --provider.
pkg/stagecoach/
  stagecoach.go         # READ (CLOSEST PATTERN): buildDeps — the 1-role analog to mirror (cohesion).
internal/git/git.go    # READ (TYPE only): the Git interface (Deps.Git). UNCHANGED.
internal/ui/verbose.go # READ (TYPE only): the Verbose struct (Deps.Verbose). UNCHANGED.
internal/decompose/    # DOES NOT EXIST YET — THIS TASK CREATES IT (roles.go + roles_test.go).
go.mod / go.sum        # UNCHANGED (go 1.22; stdlib + config/provider/git/ui, already imported).
.golangci.yml          # READ: errcheck/gosimple/govet/ineffassign/staticcheck/unused.
```

### Desired Codebase tree with files to be added

```bash
internal/decompose/roles.go          # NEW — package `decompose`; the role-resolution module:
                                     #   type RoleManifests { Planner, Stager, Message, Arbiter provider.Manifest }
                                     #   type RoleModels     { Planner, Stager, Message, Arbiter config.RoleConfig }
                                     #   type Deps           { Git git.Git; Registry *provider.Registry;
                                     #                         Config config.Config; Roles RoleManifests; Verbose *ui.Verbose }
                                     #   func ResolveRoles(cfg config.Config, reg *provider.Registry) (RoleManifests, RoleModels, error)
                                     #   func computeInstalled(reg *provider.Registry) []string                 (private)
                                     #   func isMultiProvider(m provider.Manifest) bool                          (private)
internal/decompose/roles_test.go     # NEW — ResolveRoles table + edge cases (happy 4-role; stager fallback;
                                     #   FR-R5b; missing/unknown/uninstalled; no-capable-fallback). Command="go"
                                     #   override manifests for deterministic IsInstalled.
internal/provider/registry.go        # ADD func (r *Registry) FirstTooledProvider(installed []string) string
                                     #   (additive — DefaultProvider twin filtering on non-empty TooledFlags).
                                     #   NO existing signature changes.
# go.mod/go.sum UNCHANGED. config/provider/git/ui/cmd/stagecoach/pkg/stagecoach all UNCHANGED (except the 1 additive method).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (preferredBuiltins is UNEXPORTED — findings §3/§5): internal/provider/registry.go defines
//   `var preferredBuiltins = []string{"pi","opencode","cursor","agy","gemini","codex","claude"}` (lowercase
//   = unexported). The stager fallback NEEDS this priority order, but it lives in a DIFFERENT package
//   (decompose) and Reg.List() is ALPHABETICAL (claude before pi = WRONG priority). THE RULE: add
//   `Registry.FirstTooledProvider(installed []string) string` to registry.go (it is in package provider,
//   so it CAN see preferredBuiltins) — a DefaultProvider twin that ALSO filters on non-empty TooledFlags.
//   Do NOT hardcode the priority list in roles.go (drift risk; a test already pins preferredBuiltins).

// CRITICAL (FR-R5b MUST use the ProviderFlag signal, NOT the PRD's pi/opencode/agy list — findings §6):
//   PRD §9.15 + config.go name pi/opencode/agy as "multi-provider". But the actionable FR-R5b mechanism
//   ("emits --provider <p> whenever it emits --model <m>") is governed by the manifest's ProviderFlag
//   (render.go:102: `if *r.ProviderFlag != "" && providerToUse != ""`). Among built-ins ONLY pi has a
//   non-empty ProviderFlag; opencode/agy have ProviderFlag="" (opencode encodes provider IN the model
//   string; agy is single-backend). If you error on opencode/agy's NORMAL empty-provider usage you
//   FALSE-POSITIVE and break defaults. THE RULE: isMultiProvider(m) = m.ProviderFlag != nil &&
//   *m.ProviderFlag != "" (only pi). Document WHY opencode/agy are excluded (no --provider flag to omit).

// CRITICAL (capture bareModelNoProvider BEFORE auto-detect — findings §6): ResolveRoleModel returns
//   provider=="" when NEITHER the role NOR the global default set a provider. ResolveRoles then
//   overwrites prov via DefaultProvider. The FR-R5b check needs the PRE-overwrite state, so capture
//   `bareModelNoProvider := prov=="" && mdl != ""` IMMEDIATELY after ResolveRoleModel, before the
//   `if prov == "" { prov = reg.DefaultProvider(installed) }` line. Then: if bareModelNoProvider &&
//   isMultiProvider(resolvedManifest) → error. (The resolved manifest is known only AFTER auto-detect+Get,
//   so the check is deferred to after Get, using the captured bool.)

// CRITICAL (FR-R5b is NON-BREAKING by construction — findings §6): config init writes `[defaults]
//   provider = "pi"` + per-role models, so ResolveRoleModel returns provider="pi" (NON-empty) ⇒
//   bareModelNoProvider=false ⇒ NO error. The check fires ONLY when a model is configured with NO provider
//   anywhere AND the auto-detected provider is pi. Do NOT also error when provider IS set but the sub-
//   provider (zai) is not — config has no sub-provider field (cfg.Provider conflates manifest-name +
//   Render provider param, a v1 quirk); detecting a wrong sub-provider is impossible at resolution time
//   and is OUT OF SCOPE (Render-layer / future). Document this limitation.

// CRITICAL (do NOT call Resolve() in ResolveRoles — findings §4/§7): the manifest from reg.Get is
//   MERGED but NOT Validate'd/Resolve'd. ResolveRoles calls Validate() (safe — nil-tolerant on optionals)
//   but does NOT Resolve() — store the unresolved manifest in RoleManifests, EXACTLY as buildDeps stores
//   the unresolved manifest in Deps.Manifest (Render calls Validate+Resolve itself). Calling Resolve here
//   would double-work and risk divergence from "providers show" (which displays the registry manifest).

// GOTCHA (stager fallback MUST switch the MODEL too — findings §5): on fallback the stager provider
//   changes (e.g. gemini→pi). Models are provider-specific (FR-R5), so the original model is now WRONG
//   for the fallback provider. Set mdl = config.DefaultModelsForProvider(fb)["stager"] ("gpt-5.4-mini"
//   for pi, "sonnet" for claude; "" if the table has no entry → Render uses the manifest DefaultModel).
//   Do NOT keep the original (provider-specific) model — it would emit e.g. `pi --model gemini-3.5-flash`.

// GOTCHA (the TooledFlags check is on the stager ALONE — findings §4/architecture): only the stager is
//   tooled (PRD §13.6.2); planner/message/arbiter are bare (Render with RenderBare uses BareFlags). The
//   `len(m.TooledFlags)==0` fallback check runs ONLY when role=="stager". Do NOT run it for the other 3.

// GOTCHA (test determinism — IsInstalled is exec.LookPath — findings §10): reg.IsInstalled probes
//   m.DetectCommand() via exec.LookPath; built-ins use Command "pi"/"claude"/… which may be ABSENT on the
//   test machine. registry_test.go tests IsInstalled with Command="go" (always on PATH in CI). In
//   roles_test.go, build Registries via NewRegistry(OVERRIDES on built-in names) with Command/Detect="go"
//   (installed) + controlled TooledFlags. MergeManifest (merge.go): a `{Command: &"go"}` override sets
//   Command; nil-slice override PRESERVES base TooledFlags (so `{Command:&"go"}` on "gemini" keeps gemini
//   nil TooledFlags = not-capable; on "pi" keeps pi non-empty = capable). To make a normally-capable
//   provider NOT-capable in a test is awkward (empty slice override = not overridden) — instead drive the
//   fallback with gemini/agy (already nil TooledFlags) as the stager and pi/claude as the fallback target.
//   Cross-package pointers: provider.strPtr is UNEXPORTED — use `cmd := "go"; provider.Manifest{Command: &cmd}`
//   (stubtest.go's local-strPtr pattern).

// GOTCHA (Deps carries RoleManifests only; the orchestrator also needs RoleModels — findings §8): the
//   contract Deps = {Git, Registry, Config, Roles RoleManifests, Verbose} has NO RoleModels field.
//   ResolveRoles returns RoleModels as its 2nd value; the orchestrator (P3.M4.T1.S1) retains it locally
//   for the Render model/provider params (or may later extend Deps with a `Models RoleModels` field).
//   THIS task defines Deps per the contract + returns RoleModels; it does NOT wire the orchestrator.

// GOTCHA (package decompose is NEW — roles.go is the first file): add a package doc comment (package
//   decompose — the multi-commit decomposition pipeline; roles.go resolves the 4 role manifests). No
//   other decompose/*.go exists yet (P3.M2.T2/T3/T4 + P3.M3.* + P3.M4.* own them) → zero merge friction.

// GOTCHA (no import cycle — findings §9): decompose imports config/provider/git/ui. config does NOT
//   import provider (ResolveRoleModel returns strings precisely to avoid that cycle). roles.go BRIDGES
//   config strings ↔ provider manifests — the same boundary buildDeps spans. No new go.mod dependency.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/decompose/roles.go — package decompose

// RoleManifests holds the four resolved (merged-but-unresolved — Render Validates+Resolves) provider
// manifests for the decomposition pipeline (PRD §13.6.2). Built by ResolveRoles; consumed by the
// orchestrator (P3.M4.T1.S1) via RoleManifests.X.Render(...). The stager field carries the TOOLED
// manifest post-FR-D4 fallback (non-empty TooledFlags guaranteed); the other three are bare manifests.
type RoleManifests struct {
	Planner  provider.Manifest // bare
	Stager   provider.Manifest // tooled (TooledFlags non-empty after fallback)
	Message  provider.Manifest // bare
	Arbiter  provider.Manifest // bare
}

// RoleModels holds the four resolved (provider, model) pairs (one config.RoleConfig per role) produced
// by ResolveRoles. Post-auto-detect and post-stager-fallback. The orchestrator passes RoleModels.X.Model
// + RoleModels.X.Provider as the Render model/provider params. config.RoleConfig is {Provider, Model}.
type RoleModels struct {
	Planner  config.RoleConfig
	Stager   config.RoleConfig
	Message  config.RoleConfig
	Arbiter  config.RoleConfig
}

// Deps carries the runtime collaborators the decompose orchestrator (P3.M4.T1.S1) threads through the
// pipeline. Injectable for testing with stub manifests: a test sets Deps.Roles directly and skips
// ResolveRoles (mirrors generate.Deps{Manifest: stub}). The orchestrator ALSO retains RoleModels
// (ResolveRoles's 2nd return value) locally for Render params (Deps carries RoleManifests only, per the
// contract — P3.M4.T1.S1 may extend Deps with a Models field if preferred).
type Deps struct {
	Git      git.Git            // the git boundary (real *gitRunner via git.New(repo); stub in tests)
	Registry *provider.Registry // the merged-manifest registry (real NewRegistry(overrides); stub in tests)
	Config   config.Config      // the fully-resolved 7-layer config snapshot
	Roles    RoleManifests      // the 4 resolved manifests (Deps carries manifests; models returned separately)
	Verbose  *ui.Verbose        // nil-safe --verbose diagnostics sink
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD func (r *Registry) FirstTooledProvider(installed []string) string to internal/provider/registry.go
  - PLACE: immediately after DefaultProvider (same file, adjacent — they are twins).
  - BODY: identical structure to DefaultProvider — build `present := make(map[string]struct{}, len(installed))`
    from `installed`; `for _, name := range preferredBuiltins { if _, ok := present[name]; !ok { continue };
    m, ok := r.Get(name); if !ok { continue }; if len(m.TooledFlags) > 0 { return name } }`; return "".
  - DOC COMMENT: cite FR-D4 ("the stager role falls back to the next-priority provider that can"); note
    it mirrors DefaultProvider + the TooledFlags filter; note only pi+claude are tooled-capable today
    (builtin.go); note it returns "" when no installed preferred built-in can stage (caller errors).
  - GOTCHA: this is the ONLY reason roles.go can respect priority — preferredBuiltins is unexported and
    Reg.List() is alphabetical. Do NOT change DefaultProvider's signature (additive only).
  - GOTCHA: reuses the unexported preferredBuiltins (same package) — drift-free (registry_test.go pins it).

Task 2: CREATE internal/decompose/roles.go — package doc + the 3 structs (RoleManifests, RoleModels, Deps)
  - ADD a package doc comment for `package decompose` (cite PRD §13.6.2; explain roles.go resolves the 4
    role manifests/models — the foundation file; note the package will grow planner.go/stager.go/etc.).
  - DEFINE RoleManifests, RoleModels, Deps EXACTLY as in "Data models and structure" above. Rich doc
    comments citing the PRD § + the FR + the contract (see the struct comments).
  - GOTCHA: field order Planner/Stager/Message/Arbiter (alphabetical within the pipeline order is fine;
    match the contract verbatim). Deps.Roles is RoleManifests (NOT RoleModels) per the contract.

Task 3: CREATE internal/decompose/roles.go — private helpers (computeInstalled, isMultiProvider)
  - DEFINE `func computeInstalled(reg *provider.Registry) []string`: `var installed []string; for _, m :=
    range reg.List() { if reg.IsInstalled(m) { installed = append(installed, m.Name) } }; return installed`.
    Doc: mirrors pkg/stagecoach.buildDeps's installed computation; computed ONCE per ResolveRoles call
    (shared by all 4 roles + FirstTooledProvider).
  - DEFINE `func isMultiProvider(m provider.Manifest) bool { return m.ProviderFlag != nil && *m.ProviderFlag
    != "" }`. Doc: cite FR-R5b + findings §6 — the ProviderFlag signal (only pi today); explain WHY
    opencode/agy (ProviderFlag="") are excluded (no separate --provider to omit; opencode encodes provider
    in the model string; agy is single-backend).
  - GOTCHA: ProviderFlag is non-nil for every built-in (pi="--provider", rest=""), but check nil-safe for
    a hypothetical user override that left it nil (MergeManifest preserves the base, so in practice never
    nil — defensive nil-guard anyway).

Task 4: CREATE internal/decompose/roles.go — ResolveRoles(cfg, reg) (RoleManifests, RoleModels, error)
  - SIGNATURE: `func ResolveRoles(cfg config.Config, reg *provider.Registry) (RoleManifests, RoleModels, error)`.
  - BODY:
      var rm RoleManifests
      var rmodels RoleModels
      installed := computeInstalled(reg)
      for _, role := range []string{"planner", "stager", "message", "arbiter"} {
          prov, mdl := config.ResolveRoleModel(role, cfg)
          bareModelNoProvider := prov == "" && mdl != ""            // capture PRE-auto-detect (FR-R5b)
          if prov == "" {
              prov = reg.DefaultProvider(installed)                 // auto-detect (mirrors buildDeps)
          }
          if prov == "" {
              return RoleManifests{}, RoleModels{}, fmt.Errorf(
                  "role %q: no provider configured and none of the preferred built-ins are installed", role)
          }
          m, ok := reg.Get(prov)
          if !ok {
              return RoleManifests{}, RoleModels{}, fmt.Errorf("role %q: unknown provider %q", role, prov)
          }
          if err := m.Validate(); err != nil {
              return RoleManifests{}, RoleModels{}, fmt.Errorf("role %q: provider %q: %w", role, prov, err)
          }
          if !reg.IsInstalled(m) {
              return RoleManifests{}, RoleModels{}, fmt.Errorf(
                  "role %q: provider %q: command %q not found. Is the agent installed?",
                  role, prov, m.DetectCommand())
          }
          // FR-R5b: model set with NO provider, on a multi-provider agent → ambiguous → error.
          if bareModelNoProvider && isMultiProvider(m) {
              return RoleManifests{}, RoleModels{}, fmt.Errorf(
                  "role %q: model %q is set without a provider; %q is a multi-provider agent and needs an "+
                  "explicit provider so the model is routed correctly (set stagecoach.provider or --%s-provider)",
                  role, mdl, prov, role)
          }
          // FR-D4 stager fallback: a TooledFlags-less stager cannot stage → fall back to a capable one.
          if role == "stager" && len(m.TooledFlags) == 0 {
              fb := reg.FirstTooledProvider(installed)
              if fb == "" {
                  return RoleManifests{}, RoleModels{}, fmt.Errorf(
                      "role %q: provider %q cannot stage (tooled_flags empty) and no other installed "+
                      "provider is stager-capable", role, prov)
              }
              fbm, ok := reg.Get(fb)
              if !ok {
                  return RoleManifests{}, RoleModels{}, fmt.Errorf(      // defensive — FirstTooledProvider only returns Get-able names
                      "role %q: stager fallback provider %q not found", role, fb)
              }
              prov = fb
              m = fbm
              if col := config.DefaultModelsForProvider(fb); col != nil {
                  mdl = col["stager"]                                   // provider-specific stager model (FR-R5); "" if absent → manifest default
              }
          }
          // store into the right field (helper or a switch — see Implementation Patterns).
          setRole(&rm, &rmodels, role, m, prov, mdl)
      }
      return rm, rmodels, nil
  - GOTCHA: import "fmt" + the 4 internal packages. `setRole` is a private helper that switch-cases on
    `role` to assign the right struct field (a 4-case switch on "planner"/"stager"/"message"/"arbiter").
    Do NOT use reflection. Default-case = defensive no-op (or error) — the 4 roles are the only callers.
  - GOTCHA: every error is wrapped with the role name (cohesion with buildDeps's role-agnostic wording;
    the role name is the new diagnostic dimension for the 4-role path). Return zero-value structs on error.
  - DOC COMMENT: cite PRD §13.6.2/§9.15/§9.16; diagram the per-role pipeline (ResolveRoleModel →
    DefaultProvider → Get → Validate → IsInstalled → FR-R5b → stager fallback → store); note it is the
    4-role generalization of pkg/stagecoach.buildDeps; note the merged-but-unresolved manifest (Render
    Resolves); note the stager fallback switches provider+model; note FR-R5b's narrow non-breaking scope.

Task 5: CREATE internal/decompose/roles_test.go — table + edge-case tests
  - IMPORTS: "fmt"; "strings"; "testing"; "github.com/dustin/stagecoach/internal/config";
    "github.com/dustin/stagecoach/internal/provider". Package: `decompose` (internal test — unexported
    computeInstalled/isMultiProvider visible).
  - ADD a local `strPtr`/`boolPtr` helper (provider.strPtr is unexported — different package) OR use the
    `cmd := "go"; &cmd` inline form (stubtest.go precedent).
  - ADD a helper `func goRegistry(t *testing.T, overrides map[string]provider.Manifest) *provider.Registry`:
    wraps provider.NewRegistry(overrides) — tests pass overrides that set Command/Detect to "go" on the
    built-ins they want "installed". (MergeManifest: {Command:&"go"} overrides Command, inherits the rest.)
  - ADD TestResolveRoles_HappyPath_AllPi: cfg with cfg.Provider="pi" (no per-role override); a registry
    where pi's Command is overridden to "go" (installed). Expect: nil error; all 4 manifests ==
    reg.Get("pi"); all 4 RoleModels.Provider=="pi"; Stager.TooledFlags non-empty (pi is capable — no
    fallback). (Model assertions: RoleModels.X.Model reflects cfg.Model / ResolveRoleModel's result.)
  - ADD TestResolveRoles_StagerFallback: cfg.Roles["stager"]={Provider:"gemini"}; override gemini +
    pi + claude all with Command="go". gemini's TooledFlags is nil (inherited from builtinGemini) →
    stager falls back. Expect: rm.Stager == reg.Get("pi") (priority-first capable); rmodels.Stager.Provider
    =="pi"; rmodels.Stager.Model == config.DefaultModelsForProvider("pi")["stager"] ("gpt-5.4-mini").
    Assert the OTHER 3 roles are UNAFFECTED (planner/message/arbiter stay gemini if that's the global, or
    whatever ResolveRoleModel returned). Variant: if pi were NOT installed, fallback → claude.
  - ADD TestResolveRoles_NoStagerCapable: override gemini (stager, Command="go", nil TooledFlags) + make
    pi/claude NOT installed (leave their Command as the built-in "pi"/"claude" — absent on most machines —
    OR override Detect to a bogus name). Expect: non-nil error containing "cannot stage" + "stager-capable".
  - ADD TestResolveRoles_FR5b_BareModelOnPi: cfg with NO defaults provider, cfg.Roles["planner"]={Model:
    "glm-5-turbo"} (provider empty); override pi Command="go" (installed → auto-detected). Expect: non-nil
    error containing "without a provider" + "multi-provider". Variant: same but the auto-detected provider
    is claude (make pi NOT installed, claude Command="go") → NO error (claude is not multi-provider by the
    ProviderFlag signal); planner resolves to claude. (Proves opencode/agy/claude don't false-positive.)
  - ADD TestResolveRoles_FR5b_ProviderSet_NoError: cfg.Provider="pi", cfg.Roles["planner"]={Model:
    "gpt-5.4"} (provider inherits global pi). Expect: nil error (provider is set → bareModelNoProvider
    false). This is the config-init default case — MUST NOT error.
  - ADD TestResolveRoles_MissingProvider: cfg with no provider anywhere; a registry where NO built-in is
    installed (all Detect=bogus). Expect: non-nil error containing "no provider configured".
  - ADD TestResolveRoles_UnknownProvider: cfg.Provider="nope"; expect non-nil error "unknown provider".
  - ADD TestResolveRoles_Uninstalled: cfg.Provider="pi"; leave pi's Command="pi" (likely absent); expect
    non-nil error "command" + "not found".
  - ADD TestFirstTooledProvider (in package provider OR decompose — since it's a Registry method, prefer a
    provider-package test provider/registry_test.go append OR a decompose test calling it). Cases: installed
    has pi (capable) → "pi"; installed has gemini+claude (only claude capable) → "claude"; installed empty
    → ""; installed has only gemini (not capable) → "". Prefer adding to internal/provider/registry_test.go
    (same package, next to TestDefaultProvider) for cohesion.
  - GOTCHA: IsInstalled probes the REAL $PATH. To make a built-in "installed" deterministically, override
    Command/Detect to "go" (always on PATH in CI). To make it NOT installed, override Detect to a bogus
    name (e.g. "definitely-not-real-xyz"). Do NOT rely on pi/claude being present on the dev machine.
  - GOTCHA: do NOT redeclare strPtr/boolPtr if a sibling decompose test file exists (none does yet — but
    when P3.M2.T2+ add test files, coordinate; for now roles_test.go owns it). Use distinct helper names.

Task 6: VERIFY — build, vet, lint, format, full test suite
  - RUN: `go build ./...` (compiles the new package + the additive method); `go vet ./...`;
    `golangci-lint run`; `gofmt -l internal/ pkg/` (must be empty — run `gofmt -w` on the new files);
    `go test ./...` (the new tests pass AND no existing test regressed — the additive FirstTooledProvider
    changes no existing signature).
  - GOTCHA: errcheck is enabled — check every error return (Validate, Get's ok, etc.). Wrap errors with
    %w where appropriate. unused/staticcheck: no unused helpers (computeInstalled/isMultiProvider must be
    called). Confirm `git status --short` shows exactly 3 changes (roles.go, roles_test.go, registry.go).
```

### Implementation Patterns & Key Details

```go
// PATTERN — the per-role resolution loop (mirror buildDeps's single-role path, ×4):
func ResolveRoles(cfg config.Config, reg *provider.Registry) (RoleManifests, RoleModels, error) {
	installed := computeInstalled(reg) // ONCE — shared by all 4 roles + FirstTooledProvider
	var rm RoleManifests
	var rmodels RoleModels
	for _, role := range []string{"planner", "stager", "message", "arbiter"} {
		prov, mdl := config.ResolveRoleModel(role, cfg)
		bareModelNoProvider := prov == "" && mdl != "" // FR-R5b — capture PRE-auto-detect
		if prov == "" {
			prov = reg.DefaultProvider(installed) // auto-detect (buildDeps parity)
		}
		m, err := resolveOneRole(role, prov, mdl, bareModelNoProvider, reg, installed)
		if err != nil {
			return RoleManifests{}, RoleModels{}, err
		}
		// stager fallback handled inside resolveOneRole OR inline here (Task 4 shows inline)
		setRole(&rm, &rmodels, role, m.manifest, m.provider, m.model)
	}
	return rm, rmodels, nil
}

// PATTERN — setRole: a 4-case switch to assign the right struct field (no reflection):
func setRole(rm *RoleManifests, rmodels *RoleModels, role string, m provider.Manifest, prov, mdl string) {
	rc := config.RoleConfig{Provider: prov, Model: mdl}
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

// PATTERN — isMultiProvider: the ProviderFlag signal (findings §6 — only pi today):
func isMultiProvider(m provider.Manifest) bool {
	return m.ProviderFlag != nil && *m.ProviderFlag != "" // nil-safe; non-empty only for pi
}

// PATTERN — FirstTooledProvider (DefaultProvider twin + the TooledFlags filter):
func (r *Registry) FirstTooledProvider(installed []string) string {
	present := make(map[string]struct{}, len(installed))
	for _, name := range installed {
		present[name] = struct{}{}
	}
	for _, name := range preferredBuiltins { // unexported — same package; drift-free
		if _, ok := present[name]; !ok {
			continue
		}
		m, ok := r.Get(name)
		if !ok {
			continue
		}
		if len(m.TooledFlags) > 0 { // the ONLY difference from DefaultProvider
			return name
		}
	}
	return ""
}

// CRITICAL: store the UNRESOLVED merged manifest (reg.Get result). Do NOT call m.Resolve() — Render
// Validate+Resolves later (buildDeps parity). The TooledFlags/ProviderFlag checks work on the unresolved
// manifest directly (TooledFlags is a plain slice; ProviderFlag is non-nil for every built-in).

// CRITICAL: stager fallback switches BOTH provider AND model (models are provider-specific — FR-R5):
//   mdl = config.DefaultModelsForProvider(fb)["stager"]  // "gpt-5.4-mini" / "sonnet"; "" → manifest default
```

### Integration Points

```yaml
PACKAGE (NEW):
  - create: "internal/decompose/ (package decompose); roles.go is the first file"
  - doc: "package doc comment citing PRD §13.6.2"

PROVIDER (ADDITIVE METHOD):
  - add to: internal/provider/registry.go (immediately after DefaultProvider)
  - method: "func (r *Registry) FirstTooledProvider(installed []string) string"
  - pattern: "DefaultProvider twin + the `len(m.TooledFlags) > 0` filter; reuses preferredBuiltins"
  - PRESERVE: "every existing signature (DefaultProvider/Get/List/IsInstalled/NewRegistry/...)"

CONSUMER (NOT THIS TASK — P3.M4.T1.S1):
  - the decompose orchestrator calls ResolveRoles(cfg, reg) to build Deps; retains RoleModels for Render.
  - NO caller wiring in this task (do NOT touch cmd/ or pkg/stagecoach/).

NO DATABASE / NO CONFIG-FILE / NO ROUTE changes. go.mod/go.sum UNCHANGED.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after each file creation — fix before proceeding.
gofmt -w internal/decompose/roles.go internal/decompose/roles_test.go internal/provider/registry.go
go vet ./internal/decompose/... ./internal/provider/...
go build ./...

# Expected: zero errors. If errors exist, READ the output and fix before proceeding (the most likely
# failure is a field-name/type mismatch against the contract — re-check RoleManifests/RoleModels/Deps).
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test the new package + the additive method as created.
go test ./internal/decompose/... -v
go test ./internal/provider/... -run FirstTooledProvider -v   # the additive method's test (registry_test.go)

# Expected: all new tests pass. The IsInstalled-dependent cases MUST use Command="go" overrides for
# determinism (a flaky test relying on pi/claude being on $PATH is a bug). If a fallback/FR-R5b case
# fails, re-check findings §5/§6 (the model-switch rule; the ProviderFlag signal vs the PRD list).
```

### Level 3: Integration (No-Regressions Validation)

```bash
# The additive FirstTooledProvider must NOT break any existing provider test (no signature change).
go test ./internal/provider/... -v
# The new package + the whole module must be green.
go test ./...
go vet ./...
golangci-lint run        # .golangci.yml: errcheck/gosimple/govet/ineffassign/staticcheck/unused
gofmt -l internal/ pkg/  # MUST be empty

# Confirm scope: exactly 3 changes, go.mod/go.sum untouched.
git status --short
git diff --stat          # expect roles.go (+), roles_test.go (+), registry.go (FirstTooledProvider only)

# Expected: the whole module builds + tests green; only 3 files changed; no existing behavior altered.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# (This task is pure resolution logic — no agent runs, no git mutations, no network. Level 4 is a
# design-coherence check, not a runtime check.)

# Confirm the contract surface by grepping the exported symbols (the orchestrator depends on these names):
rg -n 'type (RoleManifests|RoleModels|Deps) struct|func ResolveRoles' internal/decompose/roles.go
rg -n 'func \(r \*Registry\) FirstTooledProvider' internal/provider/registry.go

# Confirm Deps matches the architecture doc's shape (Roles RoleManifests; Git/Registry/Config/Verbose):
rg -n 'type Deps struct' -A 6 internal/decompose/roles.go

# Expected: all 4 exported symbols present; Deps has exactly the 5 contract fields; FirstTooledProvider
# is additive (DefaultProvider unchanged).
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go build ./...` succeeds (new package compiles + additive method compiles).
- [ ] `go test ./...` GREEN (new tests pass + NO existing test regressed).
- [ ] `go vet ./...` clean.
- [ ] `golangci-lint run` clean (errcheck: every error checked; unused: no dead helpers).
- [ ] `gofmt -l internal/ pkg/` empty.
- [ ] go.mod / go.sum UNCHANGED.

### Feature Validation

- [ ] All success criteria from "What" met (4 structs + ResolveRoles + FirstTooledProvider).
- [ ] Happy path: 4 roles resolve to the global/auto-detected provider; nil error.
- [ ] Stager fallback (FR-D4): TooledFlags-less stager → priority-first capable provider + switched model.
- [ ] FR-R5b: bare model + no provider + multi-provider (pi) → config error; provider-set + bare model → NO error.
- [ ] Missing/unknown/uninstalled provider → distinct role-named errors.
- [ ] Deps carries RoleManifests (per contract); RoleModels returned separately (orchestrator retains it).
- [ ] FirstTooledProvider is additive (no existing signature changed).

### Code Quality Validation

- [ ] Follows existing conventions (mirrors buildDeps; errcheck-clean; role-named wrapped errors).
- [ ] File placement matches the desired tree (internal/decompose/roles.go + roles_test.go; registry.go method).
- [ ] Anti-patterns avoided (no reflection in setRole; no hardcoded priority list; no Resolve() in ResolveRoles;
      no false-positive FR-R5b on opencode/agy; no model left stale on stager fallback).
- [ ] No import cycle (config does not import provider; roles.go bridges strings↔manifests).
- [ ] Doc comments cite the PRD § + the FR for every exported symbol.

### Documentation & Deployment

- [ ] Package doc comment + per-symbol doc comments are self-documenting (cite PRD §13.6.2/§9.15/§9.16).
- [ ] The FR-R5b ProviderFlag-vs-PRD-list decision + the out-of-scope sub-provider limitation are documented
      in code (isMultiProvider doc + the ResolveRoles doc) so future maintainers understand the narrow scope.
- [ ] No new env vars / config keys (this task is pure resolution logic over existing config).

---

## Anti-Patterns to Avoid

- ❌ Don't call `m.Resolve()` in ResolveRoles — store the merged-but-unresolved manifest; Render Validate+Resolves
  later (buildDeps parity; keeps "providers show" truthful).
- ❌ Don't use the PRD's literal pi/opencode/agy list for the FR-R5b check — use the `ProviderFlag != ""`
  signal, or opencode/agy's normal empty-provider usage false-positives and breaks the config-init defaults.
- ❌ Don't hardcode the preferred-provider priority list in roles.go — preferredBuiltins is unexported for a
  reason; add FirstTooledProvider to the provider package (drift-free) instead.
- ❌ Don't keep the original (provider-specific) model on a stager fallback — switch it to the fallback
  provider's FR-D4 stager model (DefaultModelsForProvider) or you emit `pi --model gemini-3.5-flash`.
- ❌ Don't change any existing provider.Registry signature — FirstTooledProvider is ADDITIVE only.
- ❌ Don't rely on pi/claude being on $PATH in tests — override Command/Detect to "go" (deterministic).
- ❌ Don't use reflection to assign role fields — a 4-case switch is clearer, faster, and vet-friendly.
- ❌ Don't wire the orchestrator (cmd/, pkg/stagecoach/) — this task ONLY builds Deps; P3.M4.T1.S1 consumes it.
- ❌ Don't swallow errors (errcheck is on) — check every Get `ok`, Validate error, etc.; wrap with the role name.

---

## Confidence Score

**9/10** — one-pass success is highly likely. The task is a well-scoped 4-role generalization of the
PROVEN v1 `buildDeps` pattern (same algorithm × 4 + 2 additions). The two non-trivial decisions are
fully resolved in the findings with explicit rationale: (1) the FR-R5b multi-provider signal is the
manifest's `ProviderFlag` (only pi) — documented WHY opencode/agy are excluded, and WHY it is
non-breaking (the config-init default sets a provider → bareModelNoProvider=false); (2) the stager
fallback needs priority order, which requires the additive `FirstTooledProvider` (preferredBuiltins is
unexported). The one residual uncertainty is test determinism around `IsInstalled` (exec.LookPath) —
mitigated by the documented `Command="go"` override pattern (the same trick registry_test.go already
uses). No import cycle, no new dependency, no existing-signature change → blast radius is tiny.
