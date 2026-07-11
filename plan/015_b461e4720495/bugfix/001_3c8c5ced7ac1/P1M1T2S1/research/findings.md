# Research: P1.M1.T2.S1 — Post-bootstrap ValidateModel regression net (all target×installed combos)

**Scope**: A new test file `internal/config/bootstrap_validate_test.go` that, for every (target,
installed) combination, generates a bootstrap config, parses the ACTIVE `[role.*]` blocks into a
`Config`, resolves each role's (provider, model) via `ResolveRoleModel`, and calls
`Manifest.ValidateModel` — failing on any bare model on a provider_flag provider (pi). Catches the
Issue 1 bug class automatically. All line numbers verified against the working tree (2026-07-11).

## 1. ⚠️ CRITICAL FINDING: the item's `package config_test` recommendation is WRONG (use `package config`)

The item + architecture/test_patterns.md claim an invariant: "`internal/config` MUST NOT import
`internal/provider`." **This invariant is already violated by production code:**

```
internal/config/bootstrap.go:9:	"github.com/dustin/stagecoach/internal/provider"
```

`bootstrap.go` (non-test, `package config`) imports `internal/provider` (for `provider.NewRegistry`
in `GenerateBootstrapConfigWithOverrides`). I verified **NO import cycle** exists
(`grep -rn 'stagecoach/internal/config' internal/provider/` → empty; provider does NOT import config).
So the "decoupling invariant" in test_patterns.md is STALE/ASPIRATIONAL, and a `package config` test
file importing `internal/provider` compiles fine — exactly as bootstrap.go already does.

**Why `package config` (internal) is REQUIRED, not `package config_test` (external):**
1. The item's point 3 requires testing `installed = nil` AND `installed = [all providers]`. Only the
   UNEXPORTED `buildBootstrapConfig(target, installed, overrides)` (bootstrap.go:143) accepts an
   `installed` arg. The exported `GenerateBootstrapConfig(target)` (bootstrap.go:22) AUTO-DETECTS
   `installed` via the `$PATH` registry → non-deterministic, and CANNOT express nil/[all].
   `package config_test` CANNOT call unexported `buildBootstrapConfig`.
2. `Config.Roles` is `toml:"-"` (config.go — never decoded directly). So `toml.Unmarshal` into a
   `*Config` leaves `Roles` nil. The production path is `toml.Unmarshal → fileConfig → materialize()`
   (file.go:219, UNEXPORTED). `package config_test` CANNOT call unexported `materialize`, and there is
   NO exported "TOML string → Config" helper (the only exported parse is `Load`, which needs a file
   path + ctx + does I/O).
3. `package config` CAN import `internal/provider` (bootstrap.go precedent; no cycle).

⇒ **Decision: `package config` (internal test package).** This is a deliberate, evidence-based
deviation from the item's literal `config_test` recommendation. The PRP must state this prominently
so the implementer doesn't follow the item's `config_test` advice, hit two walls (can't call
buildBootstrapConfig; can't populate Config.Roles), and stall.

## 2. The exact parse → resolve → validate pipeline (the test's core loop)

```
content := buildBootstrapConfig(target, installed, nil)        // deterministic (bootstrap.go:143)
var fc fileConfig                                              // file.go — the TOML decode target
toml.Unmarshal([]byte(content), &fc)                           // ACTIVE blocks only; commented blocks are inert
cfg, err := materialize(&fc, 120*time.Second, 10*time.Minute)  // file.go:219 — fileConfig → *Config (populates Roles)
for _, role := range []string{"planner","stager","message","arbiter"} {
    prov, model, _ := ResolveRoleModel(role, *cfg)             // roles.go:46 — per-role provider/model (falls back to cfg.Provider)
    if model == "" { continue }                                // blank is valid (FR-D2 — user fills it in)
    m := provider.BuiltinManifests()[prov]                     // builtin.go:18 — map[name]Manifest
    if err := m.ValidateModel(model); err != nil { t.Errorf(...) } // manifest.go:136 — FR-R5b bare-model check
}
```

**Why each step is right:**
- `buildBootstrapConfig` (not `GenerateBootstrapConfig`): deterministic + controls `installed`.
- `fileConfig` + `materialize`: the EXACT production load path (`loadTOML` at file.go:162 does this).
  Faithfully reproduces "what does a real `stagecoach --config <bootstrapped file>` load see?"
- Commented blocks (`# [role.x]`, `# provider = ...`) are TOML COMMENTS → they do NOT decode into
  `fileConfig` → they are NEVER validated. (Item point 4: validate ACTIVE roles only. Issue 2's
  commented-block bug is P1.M2.T1's scope, fenced out.)
- `materialize(&fc, timeout, hookTimeout)`: the timeout/hookTimeout params only set the GLOBAL
  `cfg.Timeout`/`cfg.HookTimeout` scalars — irrelevant to role MODEL validation. Pass Defaults values
  (120s, 10m) or (0, 0); either works.
- `ResolveRoleModel(role, *cfg)` returns the per-role provider if `[role.<role>].provider` is set,
  else falls back to `cfg.Provider` (the `[defaults] provider = <target>` line). For target=agy:
  planner/message/arbiter→agy, stager→pi (the fallback). For target=claude: all→claude.
- `ValidateModel(model)` (manifest.go:136): if the provider's `ProviderFlag != ""` (pi — the only
  multi-backend provider) AND model != "" AND model has no "/", → error. Blank → OK. Single-backend
  providers (agy/claude/opencode/qwen-code/codex/cursor — ProviderFlag empty) → always OK.

## 3. The regression property (why this test exists)

For target ∈ {agy, opencode, qwen-code, codex, cursor} (stager-fallback to pi):
- **Pre-S1 (buggy)**: stager model = bare `"gpt-5.4-mini"` on pi → `ValidateModel` errors → **test FAILS**.
- **Post-S1 (fixed)**: stager model = `""` (blanked) → skipped → **test PASSES**.

For target=pi: all role models blanked (piBlanked) → all skipped → passes.
For target=claude: models like "opus"/"sonnet"/"haiku" on claude (single-backend) → ValidateModel OK → passes.

So the test is GREEN on the fixed tree and RED on the pre-S1 tree — a true regression guard for the
Issue 1 bug class (any future revert that re-introduces a bare pi model in an active role block).

## 4. The (target, installed) table

Mirror `TestBuildBootstrapConfig_ValidTOML` (bootstrap_test.go:143) + expand to satisfy item point 3
("installed = nil AND installed = [all providers]"):

```go
var allBuiltins = []string{"pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"} // mirrors preferredBuiltins (bootstrap.go:16)
cases := []struct{ target string; installed []string }{
    {pi, [pi]}, {pi, [pi,claude]}, {claude, [claude]}, {claude, [claude,pi]}, {agy, [agy,pi,claude]}, // the ValidTOML cases
}
for _, tgt := range allBuiltins {
    cases = append(cases, {tgt, nil})          // no-detection case
    cases = append(cases, {tgt, allBuiltins})  // everything-detected case
}
```

`preferredBuiltins` (bootstrap.go:16, unexported, accessible in `package config`) IS the canonical
all-providers list — use it directly (or the literal above). Map-iteration of `BuiltinManifests()`
keys is non-deterministic; `preferredBuiltins` is ordered + stable. `installed` order only affects
which COMMENTED blocks appear (we don't validate those), so order is cosmetically irrelevant here —
but use `preferredBuiltins` for consistency.

## 5. Anchors (verified)

| Symbol | Location | Notes |
|---|---|---|
| `buildBootstrapConfig(target, installed, overrides)` | bootstrap.go:143 | UNEXPORTED — deterministic core; call with `nil` overrides |
| `GenerateBootstrapConfig(prov)` | bootstrap.go:22 | EXPORTED but auto-detects installed (non-deterministic) — DON'T use |
| `StagerFallback` | bootstrap.go:76 | exported; routes stager→pi for empty-tooled_flags targets |
| `materialize(fc, timeout, hookTimeout)` | file.go:219 | UNEXPORTED; fileConfig → *Config (populates Roles) |
| `fileConfig` (with `Role map[string]fileRoleConfig toml:"role"`) | file.go:38 | the TOML decode target |
| `Config.Roles map[string]RoleConfig toml:"-"` | config.go | NEVER decoded directly → MUST go via materialize |
| `ResolveRoleModel(role, cfg) (provider, model, reasoning)` | roles.go:46 | per-role → cfg.Provider fallback |
| `Manifest.ValidateModel(model) error` | manifest.go:136 | FR-R5b bare-model check |
| `provider.BuiltinManifests() map[string]Manifest` | builtin.go:18 | manifest lookup by name |
| `preferredBuiltins` | bootstrap.go:16 | UNEXPORTED canonical provider order |
| `TestBuildBootstrapConfig_ValidTOML` | bootstrap_test.go:143 | the table to mirror (`package config`) |
| `TestValidateModel_BareModelOnProviderFlagProvider_Errors` | manifest_test.go:318 | the ValidateModel pattern (provider pkg) |

## 6. What this task does NOT do (scope fences)

- Does NOT modify production code (bootstrap.go fix = S1, complete; test-only here).
- Does NOT validate COMMENTED role blocks (Issue 2 = P1.M2.T1 — commented blocks are inert TOML).
- Does NOT duplicate S2's stager-fallback table test (S2 asserts the SUBSTRING shape `model = ""` /
  no gpt-5.4; T2.S1 asserts the SEMANTIC validity via ValidateModel — complementary, not overlapping).
- Does NOT touch docs (item point 5: test-only).
- Does NOT add a `config upgrade` backup test (Issue 3 = P1.M2.T2).

## 7. Validation commands

- `go build ./...` (test file compiles — imports config + provider + toml).
- `go vet ./internal/config/...`.
- `gofmt -l internal/config/bootstrap_validate_test.go` → empty.
- `go test ./internal/config/ -v -run TestBootstrapValidateModels` (the item's exact command).
- `make test && make lint`.
- Grep guards: `grep -n 'package config$' internal/config/bootstrap_validate_test.go` (internal pkg);
  `grep -n 'internal/provider' internal/config/bootstrap_validate_test.go` (the import — justified by bootstrap.go precedent);
  `grep -n 'ValidateModel' internal/config/bootstrap_validate_test.go` (the core assertion).
