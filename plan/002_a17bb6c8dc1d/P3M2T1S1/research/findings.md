# P3.M2.T1.S1 Research Findings — decompose/roles.go

Empirical findings from reading the codebase (2026-07-01). Every claim below is backed by a file:line
reference. These are the load-bearing facts the PRP encodes.

## 1. The contract (verbatim from the work item)

Create `internal/decompose/roles.go`:
- `type RoleManifests struct { Planner, Stager, Message, Arbiter provider.Manifest }`
- `type RoleModels struct { Planner, Stager, Message, Arbiter config.RoleConfig }`
- `type Deps struct { Git git.Git; Registry *provider.Registry; Config config.Config; Roles RoleManifests; Verbose *ui.Verbose }`
- `func ResolveRoles(cfg config.Config, reg *provider.Registry) (RoleManifests, RoleModels, error)`

For each role: ResolveRoleModel → reg.Get → Validate → IsInstalled → stager tooled_flags check (fall
back if empty). Return error on: missing provider, uninstalled command, or model-without-provider for
multi-provider agents. This is the FIRST file in `internal/decompose/` (package `decompose`).

## 2. config.ResolveRoleModel — the upstream resolver (internal/config/roles.go)

Signature: `func ResolveRoleModel(role string, cfg Config) (provider, model string)`.

- Reads `cfg.Roles[role]` (per-field merge already done by the loaders), then falls back to
  `cfg.Provider`/`cfg.Model` (global) for any field still empty. Returns the `("","")` SENTINEL =
  "use manifest defaults" (provider=="" → registry auto-detects; model=="" → manifest DefaultModel).
- Does NOT consult manifests (config must not import provider — no cycle). Provider/Model resolved
  INDEPENDENTLY (per-field). A role absent from cfg.Roles inherits global entirely.
- `config.RoleConfig` is `{Provider, Model string}` (config.go). So RoleModels is 4× RoleConfig.

CRITICAL: ResolveRoleModel returns MANIFEST-LEVEL provider (pi/claude/...), NOT a sub-provider
(zai/openai). This matches how v1 buildDeps uses cfg.Provider both as the reg.Get key AND the Render
provider param. There is no separate sub-provider config field. See §6 (FR-R5b nuance).

## 3. provider.Registry — the manifest store (internal/provider/registry.go)

- `NewRegistry(userOverrides map[string]Manifest)` — seeds BuiltinManifests, overlays each override via
  MergeManifest (built-in match) or adds verbatim (§12.8 new name). Stored manifests are MERGED but NOT
  Validate()'d or Resolve()'d (lifecycle: decode→merge→store→[Validate→Resolve at consume]).
- `Get(name) (Manifest, bool)`, `List() []Manifest` (sorted by Name), `IsInstalled(m) bool`
  (exec.LookPath on m.DetectCommand()), `DefaultProvider(installed []string) string`.
- `preferredBuiltins = []string{"pi","opencode","cursor","agy","gemini","codex","claude"}` is UNEXPORTED
  (registry.go:14). DefaultProvider walks it for the first in `installed`. A test enforces it matches
  BuiltinManifests keys + pi-first + the exact order (registry_test.go:29-35).

For the stager fallback I need priority order among TooledFlags-capable providers. preferredBuiltins is
unexported and Reg.List() is alphabetical (wrong priority). DECISION (§5): add a small exported
`Registry.FirstTooledProvider(installed []string) string` mirroring DefaultProvider.

## 4. provider.Manifest — the field model (internal/provider/manifest.go)

Pointer scalars (*string/*bool) + plain slices. From reg.Get the manifest is merged-but-unresolved, so:
- `TooledFlags []string` — nil/empty = provider CANNOT be a stager. Check `len(m.TooledFlags) == 0`
  directly on the reg.Get manifest (Resolve does not touch TooledFlags). Built-in state (builtin.go):
  pi + claude have non-empty TooledFlags; gemini/agy/opencode/codex/cursor have nil. ONLY pi+claude can stage.
- `ProviderFlag *string` — non-nil for ALL built-ins (either "--provider" for pi, or "" for the rest).
  Check nil-safe: `m.ProviderFlag != nil && *m.ProviderFlag != ""`. Non-empty ONLY for pi today.
- `Validate()` is nil-tolerant on optionals; strict on Name + Command. Safe on a reg.Get manifest.
- `Resolve()` fills nil pointers to defaults; Render calls Validate+Resolve itself, so ResolveRoles does
  NOT need to Resolve (it stores the merged-but-unresolved manifest; Render will Resolve at call time —
  same as buildDeps stores the unresolved m in Deps.Manifest).

## 5. The stager TooledFlags fallback (FR-D4 note)

If the resolved stager manifest has empty TooledFlags, fall back to the next-priority provider that CAN
stage (non-empty TooledFlags) AND is installed. Because models are provider-specific (FR-R5), the
fallback MUST also switch the stager MODEL to the fallback provider's stager model via
`config.DefaultModelsForProvider(fb)["stager"]` (role_defaults.go — returns a COPY or nil). If NO
installed provider has TooledFlags → hard error (decompose requires a tooled stager).

priority order: walk preferredBuiltins via a NEW `Registry.FirstTooledProvider(installed []string) string`
(additive method on registry.go — reuses the unexported preferredBuiltins, mirrors DefaultProvider's
signature; drift-free). FirstTooledProvider returns the first preferredBuiltins name that is in
`installed` AND whose manifest has non-empty TooledFlags, else "".

## 6. FR-R5b — "model without provider for multi-provider agents" (the nuanced one)

PRD §9.15 FR-R5b + config.go docstring name pi/opencode/agy as "multi-provider". But the actionable
mechanism FR-R5b describes — "emits --provider <p> whenever it emits --model <m>" — is governed by the
manifest's ProviderFlag (render.go:102: `if *r.ProviderFlag != "" && providerToUse != ""`). Among
built-ins, ProviderFlag != "" is TRUE ONLY for pi. opencode/agy have ProviderFlag="" (opencode encodes
its provider IN the model string "openai/gpt-5.4"; agy is single-backend Gemini) → there is no separate
--provider to omit, so the bare-model misroute mechanism does not apply to them at the flag level.

DECISION: define "multi-provider agent" for THIS check as `*m.ProviderFlag != ""` (only pi today). This:
(a) uses the only signal available in the manifest; (b) is non-breaking — opencode/agy with their normal
empty provider do NOT error (they would under the literal pi/opencode/agy list); (c) catches pi's
bare-model misroute (the FR-R5b motivating case).

THE CHECK: capture the PRE-auto-detect result of ResolveRoleModel. If it returned `provider=="" &&
model!=""` (a model is configured with NO provider at role OR global level → would auto-detect) AND the
auto-detected manifest is multi-provider (ProviderFlag!="") → return a config error. This is narrow and
correct:
- Defaults (config init writes `[defaults] provider=pi` + per-role models): ResolveRoleModel returns
  provider="pi" (non-empty) → bareModelNoProvider=false → NO error. Defaults work.
- Dangerous case (`[role.planner] model=glm-5-turbo`, no provider anywhere): ResolveRoleModel → ("",
  "glm-5-turbo") → auto-detect pi → ProviderFlag="--provider" → ERROR. Correct.
- opencode/agy auto-detected with bare model: ProviderFlag="" → NOT multi-provider → no error. Correct.

OUT OF SCOPE (documented, NOT enforced here): when provider IS set to the manifest name "pi" but the
SUB-provider (zai) is not — config has no separate sub-provider field (cfg.Provider conflates manifest
name + Render provider param, a v1 design quirk). stagecoach cannot detect "glm-5-turbo ≠ pi's default
upstream" at resolution time. That is a Render-layer / future concern. ResolveRoles surfaces only the
resolvable misconfiguration (bare model, no provider, multi-provider manifest).

## 7. The closest existing pattern — pkg/stagecoach.buildDeps (pkg/stagecoach/stagecoach.go)

buildDeps(cfg, repoDir) is the v1 single-commit analog of ResolveRoles (one role: message). It:
1. `overrides, _ := provider.DecodeUserOverrides(cfg.Providers)` → `reg := provider.NewRegistry(overrides)`.
2. `name := cfg.Provider; if name == "" { compute installed; name = reg.DefaultProvider(installed) }`.
3. `if name == "" { error "no provider configured and none of the built-ins installed" }`.
4. `m, ok := reg.Get(name); if !ok { error "unknown provider" }`.
5. `m.Validate()`; pre-flight `if !reg.IsInstalled(m) { error "command not found" }`.
6. applies cfg.Output/cfg.StripCodeFence onto m (generation-layer override).

ResolveRoles is buildDeps generalized to 4 roles + the stager fallback + the FR-R5b check. The installed
computation + the error wording + the auto-detect path should mirror buildDeps for cohesion.

The installed computation (buildDeps): iterate `reg.List()`, `if reg.IsInstalled(m) { append installed,
m.Name }`. ResolveRoles computes this ONCE (all 4 roles share it). The error message hardcodes a provider
list — reuse the same 6-name list or, better, derive from preferredBuiltins.

## 8. Deps / RoleModels cohesion (for P3.M4.T1.S1)

Contract Deps = {Git, Registry, Config, Roles RoleManifests, Verbose}. ResolveRoles returns
(RoleManifests, RoleModels, error). The orchestrator (P3.M4.T1.S1) needs BOTH: RoleManifests.X to call
Render, and RoleModels.X.Model/Provider as the Render model/provider params. Per the contract Deps
carries only RoleManifests; the orchestrator retains RoleModels locally (or P3.M4.T1.S1 may extend Deps
with a `Models RoleModels` field). THIS task defines Deps per the contract and returns RoleModels; it
does NOT wire the orchestrator. Flagged for cohesion.

## 9. Import graph (no cycle)

decompose imports: config, provider, git, ui. None of those import decompose (P3 is the first consumer).
config does NOT import provider (ResolveRoleModel returns strings to avoid the cycle — roles.go BRIDGES
config strings ↔ provider manifests, exactly the boundary buildDeps spans). go.mod is `go 1.22`, stdlib +
toml only — no new dependency.

## 10. Test determinism (the IsInstalled challenge)

reg.IsInstalled does exec.LookPath(m.DetectCommand()) — environment-dependent. Built-ins use Command
"pi"/"claude"/... which may be absent on the test machine. registry_test.go solves this by testing
IsInstalled with Command="go" (always on PATH in CI). For roles_test.go:
- Construct Registries via NewRegistry with OVERRIDE manifests on built-in names whose Command/Detect =
  "go" (installed) and controlled TooledFlags. MergeManifest (merge.go): scalar pointer non-nil wins;
  slice len>0 replaces; nil slice override preserves base. So:
  - `{Command: &"go"}` on "pi" → pi is "installed"; pi's builtin TooledFlags survive (non-empty → capable).
  - `{Command: &"go"}` on "gemini" → gemini "installed"; gemini's TooledFlags stays nil → NOT capable.
  - `{Command: &"go", TooledFlags: []string{"x"}}` on "claude" → installed + capable (stager fallback target).
- Cross-package pointer construction: provider.strPtr is unexported; use `cmd := "go"; &cmd` (stubtest.go
  does exactly this — its own local strPtr/boolPtr).
- FirstTooledProvider + DefaultProvider both take `installed []string` — so tests control the installed
  set directly without exec at all for THOSE calls, but ResolveRoles computes installed via IsInstalled,
  hence the Command="go" override approach for end-to-end ResolveRoles tests.

## 11. Validation gates (verified Go project)

`go build ./...` · `go test ./internal/decompose/... -v` · `go test ./...` (no breakage) ·
`go vet ./...` · `golangci-lint run` (.golangci.yml: errcheck/gosimple/govet/ineffassign/staticcheck/
unused; errcheck excluded on some test paths) · `gofmt -l internal/ pkg/` (must be empty). All additive
(new file + one additive registry method) — no existing test should change.
