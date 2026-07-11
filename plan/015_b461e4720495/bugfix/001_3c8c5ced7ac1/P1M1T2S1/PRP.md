name: "P1.M1.T2.S1 — Post-bootstrap ValidateModel regression net for all (target, installed) combinations"
description: >
  A new test file internal/config/bootstrap_validate_test.go (package config) that, for every
  (target, installed) combination, generates a bootstrap config, parses the ACTIVE [role.*] blocks
  into a Config via the production fileConfig→materialize path, resolves each role's (provider, model)
  via ResolveRoleModel, and calls Manifest.ValidateModel — failing on any bare model on a provider_flag
  provider (pi). This is the permanent regression net for the Issue 1 bug class (a bare pi stager model
  that breaks decomposition on first run). Test-only — no production code, no docs. Uses package config
  (internal) NOT package config_test, because it needs the unexported buildBootstrapConfig (deterministic
  installed control) + materialize (Config.Roles is toml:"-") — and the "config must not import provider"
  invariant is already violated by bootstrap.go:9 (no cycle; verified).

---

## Goal

**Feature Goal**: Provide a permanent, automatic regression net that catches the Issue 1 bug class
(any ACTIVE role model in a `config init` bootstrap output that violates FR-R5b — i.e. a bare model on
a `provider_flag`/multi-backend provider like pi). For every `(target, installed)` combination the
bootstrap supports, the test generates the config, loads it the way a real run does, resolves each
active role's effective `(provider, model)`, and asserts `ValidateModel` passes (blank models skipped).
A future revert of S1's stager-fallback blanking (or the same bug class on any new path) turns this
test red before a release ships a broken first-run config.

**Deliverable**: ONE new test file `internal/config/bootstrap_validate_test.go` in **`package config`**
(internal test package — see CRITICAL deviation note below), containing `TestBootstrapValidateModels`:
a table-driven test over `(target, installed)` (every built-in target × {nil, all} + the existing
`TestBuildBootstrapConfig_ValidTOML` cases), that runs the generate→parse→resolve→ValidateModel pipeline
for all four roles. Plus a small `installedLabel` helper for readable subtest names.

**Success Definition**:
- For every `(target, installed)` row, every ACTIVE role model in the bootstrapped config either is
  blank (skipped) or passes `ValidateModel` on its resolved provider's manifest.
- The test is GREEN on the current (post-S1) tree: the agy/opencode/qwen-code stager models are blanked
  → skipped; claude's "opus"/"sonnet"/"haiku" are single-backend → OK.
- The test would FAIL on the pre-S1 tree (where the stager model was a bare `gpt-5.4-mini` on pi →
  `ValidateModel` returns the FR-R5b error). This is the load-bearing regression-guard property.
- `go test ./internal/config/ -v -run TestBootstrapValidateModels` passes; `make test` + `make lint` pass.
- No production file touched; no docs touched.

## User Persona (if applicable)

**Target User**: Stagecoach maintainers / CI (this is a regression-guard test; no user-facing surface).

**Use Case**: A maintainer refactors `buildBootstrapConfig`, `StagerFallback`, `roleDefaults`, or the
pi-blanking logic. This test fails loudly if any active role model in any bootstrap output becomes a
bare model on a `provider_flag` provider — before a release ships a `config init` output that errors on
the first decompose.

**User Journey**: PR introduces a regression (e.g. re-adds a bare pi stager model) → CI runs
`TestBootstrapValidateModels` → it fails with `ValidateModel("gpt-5.4-mini" on "pi") = ...must be
inference/model...` → maintainer fixes before merge.

**Pain Points Addressed**: Issue 1 shipped because the ONE test exercising the stager-fallback path
PINNED the buggy value and no test ever called `ValidateModel` on bootstrap output. This test closes
that gap permanently — it validates the FR-R5b INVARIANT (via `ValidateModel`), not a pinned string.

## Why

- **Issue 1 (Critical)**: `config init --provider agy` (or opencode/qwen-code) wrote a bare pi stager
  model → FR-R5b hard error → decomposition (the DEFAULT action) failed on the first run. S1
  (P1.M1.T1.S1, Complete) fixed the emission (blanks the model). S2 (P1.M1.T1.S2, Implementing) adds
  substring-shape tests (`model = ""`, no `gpt-5.4`). **This task (T2.S1) adds the SEMANTIC regression
  net**: it calls `ValidateModel` (the actual FR-R5b enforcer) on every active role model in every
  bootstrap output, so the bug class — not just the one pinned string — is caught forever. PRD §9.15
  FR-R5b, §9.17 FR-B1.
- **Complementary to S1/S2, non-overlapping**: S1 = the code fix; S2 = substring-shape assertions
  (`model = ""`, `no gpt-5.4`, guidance present); T2.S1 = the `ValidateModel` invariant sweep across
  ALL `(target, installed)` combos. S2 asserts the FIX's surface; T2.S1 asserts the INVARIANT. Both
  belong; neither replaces the other.
- **The architecture doc asked for it**: `bootstrap_pi_model_bug.md §Post-Bootstrap ValidateModel
  Regression Net` specifies this exact approach (generate → parse → resolve → ValidateModel).

## What

**User-visible behavior**: None (test-only; item point 5: "DOCS: none").

**Technical change (one new test file):**
- `TestBootstrapValidateModels` iterates `(target, installed)` over: the `ValidTOML` cases +
  every built-in target × `{nil, allBuiltins}` (the item's "installed = nil and installed = [all
  providers]" requirement). For each row: `buildBootstrapConfig(target, installed, nil)` →
  `toml.Unmarshal` into `fileConfig` → `materialize(&fc, 120s, 10m)` → for each of the 4 roles,
  `ResolveRoleModel` → if model != "", `BuiltinManifests[provider].ValidateModel(model)` must be nil.

### Success Criteria
- [ ] `TestBootstrapValidateModels` covers every built-in target (`pi`, `claude`, `agy`, `opencode`, `qwen-code`, `codex`, `cursor`) with `installed=nil` AND `installed=allBuiltins`, plus the `ValidTOML` cases.
- [ ] Every ACTIVE role model in every row either is blank (skipped) or passes `ValidateModel`.
- [ ] The test is GREEN on the post-S1 tree (stager blanked → skipped; claude single-backend → OK).
- [ ] The test would FAIL on the pre-S1 tree (bare pi stager → `ValidateModel` FR-R5b error). *[verify by reasoning: pre-S1 stager model was `gpt-5.4-mini` on pi, no `/` → ValidateModel errors]*
- [ ] Commented role blocks are NOT validated (they are inert TOML comments → not in `fileConfig`).
- [ ] `go test ./internal/config/ -v -run TestBootstrapValidateModels` passes; `make test`/`make lint` pass; no production/docs touched.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact package to use (and WHY not the item's recommended one), the exact generate→parse→
resolve→validate pipeline with verified signatures, the table to mirror, the regression property to
reason about, the "Config.Roles is toml:'-'" gotcha, the "commented blocks are inert" gotcha, and the
scope fences against S1/S2/P1.M2.T1 are all enumerated below.

### Documentation & References

```yaml
- file: internal/config/bootstrap.go
  why: "buildBootstrapConfig(target, installed, overrides) @143 — the UNEXPORTED deterministic core to
        call (NOT the exported GenerateBootstrapConfig @22, which auto-detects installed non-deterministically).
        preferredBuiltins @16 — the canonical all-providers ordered list (use as `installed=allBuiltins`).
        bootstrap.go:9 imports internal/provider — PROOF the 'config must not import provider' invariant
        is already relaxed for this package (so a `package config` test importing provider compiles)."
  pattern: "buildBootstrapConfig(target, installed, nil) returns the TOML string deterministically."
  critical: "Call buildBootstrapConfig (unexported), NOT GenerateBootstrapConfig. The item's point 3a says
             'GenerateBootstrapConfig(target)' but that can't express installed=nil/[all]; buildBootstrapConfig
             can. This is only callable from `package config` (internal) — see the CRITICAL deviation note."

- file: internal/config/file.go
  why: "THE parse path. fileConfig (with Role map[string]fileRoleConfig toml:\"role\" @38) is the TOML
        decode target. materialize(fc, timeout, hookTimeout) @219 (UNEXPORTED) is fileConfig → *Config —
        the ONLY way to populate Config.Roles (which is toml:\"-\"). loadTOML @162 is the production caller
        of this exact pair (toml.Unmarshal at @177)."
  pattern: >
    var fc fileConfig
    if err := toml.Unmarshal([]byte(content), &fc); err != nil { t.Fatalf(...) }
    cfg, err := materialize(&fc, 120*time.Second, 10*time.Minute)
    if err != nil { t.Fatalf(...) }
  critical: "Do NOT toml.Unmarshal into *Config directly — Config.Roles is toml:\"-\" so Roles stays nil and
             ResolveRoleModel finds no roles. You MUST go fileConfig → materialize. materialize is unexported
             → another reason this test is `package config`, not `package config_test`. The timeout/hookTimeout
             params only set cfg.Timeout/HookTimeout scalars (irrelevant to role-model validation) — pass
             Defaults values (120s, 10m) or (0,0); either works. BurntSushi/toml ignores unknown keys, so the
             bootstrap's header docs + [provider.*] tables decode cleanly into fileConfig."

- file: internal/config/roles.go
  why: "ResolveRoleModel(role, cfg) @46 returns (provider, model, reasoning). Uses cfg.Roles[role] if
        present (per-role override), else falls back to cfg.Provider (the [defaults] provider line). This
        is the 'resolve each role's effective (provider, model)' step."
  pattern: "prov, model, _ := ResolveRoleModel(role, *cfg)"
  critical: "The provider returned is the per-role provider if [role.<role>].provider is set (e.g. stager=pi
             for an agy target), else cfg.Provider (the target). For target=agy: planner/message/arbiter→agy,
             stager→pi. For target=claude: all→claude. The reasoning return is unused here."

- file: internal/provider/manifest.go
  why: "Manifest.ValidateModel(model) @136 — the FR-R5b enforcer. Returns nil if OK; returns an error
        'provider render NAME: model MODEL on NAME must be inference/model...' if the provider's
        ProviderFlag != \"\" (pi is the only one) AND model != \"\" AND model has no '/'. Blank model → OK.
        Single-backend providers (ProviderFlag empty) → always OK."
  pattern: "if err := manifests[prov].ValidateModel(model); err != nil { t.Errorf(...) }"
  critical: "ValidateModel is the SEMANTIC check (the real FR-R5b rule). This is what makes T2.S1 a stronger
             guard than S2's substring 'no gpt-5.4' check: it catches ANY bare model on pi, not just
             gpt-5.4-mini. Blank models are valid (FR-D2) — skip them, don't error."

- file: internal/provider/builtin.go
  why: "provider.BuiltinManifests() @18 (EXPORTED) returns map[string]Manifest — the manifest lookup by
        provider name. Every role provider in a bootstrap output is a built-in (pi/claude/agy/opencode/
        qwen-code/codex/cursor) so the lookup always hits."
  pattern: "manifests := provider.BuiltinManifests()  // then manifests[prov]"

- file: internal/config/bootstrap_test.go
  why: "The pattern to mirror. TestBuildBootstrapConfig_ValidTOML @143 — the (target, installed) table
        (cases: {pi,[pi]}, {pi,[pi,claude]}, {claude,[claude]}, {claude,[claude,pi]}, {agy,[agy,pi,claude]}).
        It is `package config` and calls the unexported buildBootstrapConfig — the precedent for T2.S1's
        package choice. assertContains helper (~227) and the toml.Unmarshal-into-map validity idiom."
  pattern: "cases := []struct{ target string; installed []string }{...}; for _, tc := range cases { t.Run(...) }"
  critical: "T2.S1 is a SEPARATE file (bootstrap_validate_test.go), also `package config`, mirroring this
             table shape but adding the ValidateModel sweep + the every-target×{nil,all} expansion. Do NOT
             edit bootstrap_test.go (S2 is implementing in parallel there — avoid a merge conflict)."

- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/bootstrap_pi_model_bug.md
  why: "§Post-Bootstrap ValidateModel Regression Net specifies this approach verbatim and notes the
        config/provider-import constraint (which this PRP resolves — see deviation note)."
  section: "Post-Bootstrap ValidateModel Regression Net"

- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M1T1S1/PRP.md
  why: "S1 is the code-fix CONTRACT (Complete): it blanked the stager-fallback pi model. T2.S1 consumes
        the fixed output. Read it to confirm the post-fix bootstrap emits model=\"\" for the stager on
        non-pi stager-fallback targets (so the test SKIPS that role, not errors)."

- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M1T1S2/PRP.md
  why: "S2 (Implementing in parallel) adds substring-shape tests (model=\"\", no gpt-5.4, guidance) for
        opencode/qwen-code in bootstrap_test.go. T2.S1 is COMPLEMENTARY (semantic ValidateModel sweep),
        in a SEPARATE file — no overlap, no conflict. Read it to confirm the non-overlap."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  bootstrap.go                  # buildBootstrapConfig:143 (call this); GenerateBootstrapConfig:22 (DON'T); preferredBuiltins:16; imports provider:9
  file.go                       # fileConfig (Role toml:"role":38); materialize:219 (fileConfig→*Config); loadTOML:162 (the production parse precedent)
  roles.go                      # ResolveRoleModel:46
  bootstrap_test.go             # TestBuildBootstrapConfig_ValidTOML:143 (the table to mirror; package config) — S2 editing in parallel, DON'T touch
internal/provider/
  manifest.go                   # Manifest.ValidateModel:136 (FR-R5b enforcer)
  builtin.go                    # provider.BuiltinManifests():18 (EXPORTED map[name]Manifest)
# NEW file (this subtask):
#   internal/config/bootstrap_validate_test.go   # package config — TestBootstrapValidateModels
```

### Desired Codebase tree with files to be added

```bash
internal/config/bootstrap_validate_test.go   # NEW — `package config`; TestBootstrapValidateModels + installedLabel helper
# (no other file touched — test-only)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (package choice — DEVIATION from the item): use `package config` (internal), NOT
//   `package config_test` (external). The item + architecture/test_patterns.md recommend config_test
//   citing a "config must not import provider" invariant. THAT INVARIANT IS STALE: bootstrap.go:9
//   (production, package config) ALREADY imports internal/provider, and there is NO import cycle
//   (verified: internal/provider does not import internal/config). Two hard reasons require `package config`:
//   (1) the item's "test installed=nil and installed=[all]" needs the UNEXPORTED buildBootstrapConfig
//       (the exported GenerateBootstrapConfig auto-detects installed via $PATH — non-deterministic, and
//       can't express nil/[all]); (2) Config.Roles is toml:"-", so populating Roles needs the UNEXPORTED
//       materialize (no exported string→Config parser exists). config_test can access NEITHER. This is
//   the single most important decision in this PRP — following the item's config_test advice verbatim
//   will stall at both walls.

// CRITICAL (Config.Roles is toml:"-"): toml.Unmarshal into *Config leaves Roles NIL. You MUST parse into
//   fileConfig then call materialize(&fc, ...) — the production loadTOML path. ResolveRoleModel then finds
//   the roles in cfg.Roles. Skipping materialize is the #2 one-pass failure mode.

// CRITICAL (commented blocks are inert): the bootstrap emits commented `# [role.x]` blocks for other
//   installed providers. These are TOML COMMENTS → toml.Unmarshal does NOT decode them into fileConfig →
//   they are NEVER validated. This is CORRECT (item point 4: validate ACTIVE roles only). Issue 2's
//   commented-block bug is P1.M2.T1's scope — fenced out. Do NOT try to parse/validate commented blocks.

// CRITICAL (blank model is VALID — skip, don't error): FR-D2 ships pi's models BLANK ("there is no
//   universally-correct inference backend"). ValidateModel("") returns nil (blank is OK). So the test
//   MUST `if model == "" { continue }` — a blank is the intended state for pi roles, not a bug. Asserting
//   ValidateModel on "" is harmless (returns nil) but skipping is clearer about intent.

// CRITICAL (call buildBootstrapConfig, NOT GenerateBootstrapConfig): GenerateBootstrapConfig(target)
//   (bootstrap.go:22) creates a registry, runs $PATH detection, and calls buildBootstrapConfig(target,
//   detectedInstalled, nil). Its `installed` is whatever's on $PATH → NON-DETERMINISTIC across machines
//   (CI vs dev) → flaky test + can't express the item's nil/[all] cases. buildBootstrapConfig(target,
//   installed, nil) is deterministic. This requires `package config` (it's unexported) — see above.

// GOTCHA (materialize timeout params): materialize(&fc, timeout, hookTimeout) — the two durations only
//   set cfg.Timeout and cfg.HookTimeout (global scalars). They do NOT affect role-model resolution or
//   validation. Pass (120*time.Second, 10*time.Minute) [the Defaults values] for realism, or (0, 0); the
//   test's outcome is identical either way.

// GOTCHA (BuiltinManifests lookup always hits): every provider name that appears in a bootstrap [role.*]
//   block is a built-in (pi/claude/agy/opencode/qwen-code/codex/cursor). provider.BuiltinManifests()[prov]
//   is always non-nil. A defensive `if m, ok := manifests[prov]; !ok { t.Errorf(...) }` is fine but the
//   !ok branch is unreachable in practice.

// SCOPE: do NOT edit bootstrap_test.go (S2 is Implementing it in parallel — merge conflict). do NOT
//   validate commented blocks (Issue 2 = P1.M2.T1). do NOT touch production code (S1's fix is Complete).
//   do NOT add docs (item point 5). do NOT add a config-upgrade backup test (Issue 3 = P1.M2.T2).
```

## Implementation Blueprint

### Data models and structure
None. Pure test addition. No types, no production code. One table-driven test + one tiny helper. The
test reuses the production types (`fileConfig`, `Config`, `RoleConfig`, `provider.Manifest`) and the
production parse path (`toml.Unmarshal` + `materialize`).

### Implementation Tasks (ordered by dependencies)

> **Prerequisite**: S1 (P1.M1.T1.S1) is Complete — `buildBootstrapConfig` blanks the stager-fallback pi
> model. CONFIRM (it is): the test's agy/opencode/qwen-code rows must SKIP the stager (model=""), not
> error. If S1 were reverted, those rows would fail with the FR-R5b error (the regression-guard property).

```yaml
Task 1: CREATE internal/config/bootstrap_validate_test.go (`package config`)
  - FILE HEADER: `package config` (NOT config_test — see CRITICAL deviation note in Gotchas).
  - IMPORTS:
        import (
            "strings"
            "testing"
            "time"

            "github.com/BurntSushi/toml"
            "github.com/dustin/stagecoach/internal/provider"
        )
    (github.com/dustin/stagecoach/internal/provider is the import path — confirm via `head -15
     internal/config/bootstrap.go` which already imports it. BurntSushi/toml is the project's TOML
     lib — see bootstrap_test.go's existing import.)
  - ADD the all-builtins list (mirror preferredBuiltins at bootstrap.go:16):
        // allBuiltins mirrors internal preferredBuiltins (bootstrap.go:16) — the canonical ordered set
        // of built-in provider names. Used as the `installed=[all]` case.
        var bootstrapValidateAllBuiltins = []string{"pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"}
    (Name it bootstrapValidateAllBuiltins to avoid colliding with the unexported preferredBuiltins
     which is already in package config. OR just use preferredBuiltins directly since it's accessible.)
  - ADD the test:
        // TestBootstrapValidateModels is the post-bootstrap FR-R5b regression net (PRD §9.15 FR-R5b,
        // §9.17 FR-B1; architecture/bootstrap_pi_model_bug.md §Post-Bootstrap ValidateModel Regression Net).
        // For every (target, installed) combination it generates the bootstrap config, parses the ACTIVE
        // [role.*] blocks into a Config via the production fileConfig→materialize path (commented blocks
        // are inert TOML comments and are NOT parsed), resolves each role's effective (provider, model)
        // via ResolveRoleModel, and calls Manifest.ValidateModel. A bare model on a provider_flag provider
        // (pi) fails; a blank model is skipped (FR-D2 — the user fills it in). This would have caught
        // Issue 1 (a bare pi stager model) immediately. GREEN on the post-S1 tree; RED on the pre-S1 tree.
        //
        // NOTE: this is `package config` (not config_test) because it needs the UNEXPORTED
        // buildBootstrapConfig (for deterministic installed=nil/[all] control — GenerateBootstrapConfig
        // auto-detects installed via $PATH, which is non-deterministic) and the UNEXPORTED materialize
        // (Config.Roles is toml:"-", so direct toml.Unmarshal into *Config leaves Roles nil). The
        // "config must not import provider" invariant is already relaxed — bootstrap.go:9 imports it
        // (no cycle: provider does not import config).
        func TestBootstrapValidateModels(t *testing.T) {
            manifests := provider.BuiltinManifests()
            roles := []string{"planner", "stager", "message", "arbiter"}

            type tc struct{ target string; installed []string }
            cases := []tc{
                {"pi", []string{"pi"}},
                {"pi", []string{"pi", "claude"}},
                {"claude", []string{"claude"}},
                {"claude", []string{"claude", "pi"}},
                {"agy", []string{"agy", "pi", "claude"}},
            }
            for _, tgt := range bootstrapValidateAllBuiltins {
                cases = append(cases, tc{tgt, nil})              // no-detection case
                cases = append(cases, tc{tgt, bootstrapValidateAllBuiltins}) // everything-detected case
            }

            for _, tc := range cases {
                tc := tc
                t.Run(tc.target+"_installed_"+installedLabel(tc.installed), func(t *testing.T) {
                    content := buildBootstrapConfig(tc.target, tc.installed, nil)

                    // Parse the ACTIVE blocks (fileConfig) and materialize into a Config (populates Roles).
                    // Commented `# [role.*]` blocks are TOML comments → not decoded → not validated.
                    var fc fileConfig
                    if err := toml.Unmarshal([]byte(content), &fc); err != nil {
                        t.Fatalf("buildBootstrapConfig(%q,%v): invalid TOML: %v\n%s", tc.target, tc.installed, err, content)
                    }
                    cfg, err := materialize(&fc, 120*time.Second, 10*time.Minute)
                    if err != nil {
                        t.Fatalf("materialize: %v", err)
                    }

                    for _, role := range roles {
                        prov, model, _ := ResolveRoleModel(role, *cfg)
                        if model == "" {
                            continue // blank is valid (FR-D2 — user fills it in); skip
                        }
                        m, ok := manifests[prov]
                        if !ok {
                            t.Errorf("target=%s installed=%v role=%s: provider %q has no built-in manifest", tc.target, tc.installed, role, prov)
                            continue
                        }
                        if err := m.ValidateModel(model); err != nil {
                            t.Errorf("target=%s installed=%v role=%s: ValidateModel(%q on %q) = %v",
                                tc.target, tc.installed, role, model, prov, err)
                        }
                    }
                })
            }
        }

        func installedLabel(installed []string) string {
            if len(installed) == 0 {
                return "nil"
            }
            return strings.Join(installed, ",")
        }
  - PLACE: new file internal/config/bootstrap_validate_test.go (keeps it isolated from S2's in-flight
    bootstrap_test.go edits — no merge conflict).
  - DEPENDENCIES: S1 merged (buildBootstrapConfig blanks the pi stager — otherwise the agy/opencode/
    qwen-code rows fail with the FR-R5b error). S1 IS Complete.

Task 2: VERIFY build + vet + format + the exact item test command + regression property
  - go build ./...
  - go vet ./internal/config/...
  - gofmt -l internal/config/bootstrap_validate_test.go   # must list nothing
  - go test ./internal/config/ -v -run TestBootstrapValidateModels   # the item's EXACT command
  - go test ./internal/config/... -v                        # full package (no regressions)
  - make test && make lint
  - REGRESSION-PROPERTY CHECK (by reasoning, no code needed): confirm that for target=agy the stager
    row resolves to provider="pi", model="" (blanked by S1) → skipped → the row passes. On the pre-S1
    tree the stager model would be "gpt-5.4-mini" (bare, no "/") on pi → ValidateModel errors → the
    row fails. This red-on-old/green-on-new property is the test's reason to exist.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the generate → parse → resolve → validate pipeline (the test's core)
content := buildBootstrapConfig(tc.target, tc.installed, nil)   // deterministic (NOT GenerateBootstrapConfig)
var fc fileConfig
toml.Unmarshal([]byte(content), &fc)                            // ACTIVE blocks only (comments inert)
cfg, _ := materialize(&fc, 120*time.Second, 10*time.Minute)     // fileConfig → *Config (populates Roles; toml:"-" otherwise nil)
for _, role := range []string{"planner", "stager", "message", "arbiter"} {
    prov, model, _ := ResolveRoleModel(role, *cfg)              // per-role provider/model (falls back to cfg.Provider)
    if model == "" {
        continue                                                 // FR-D2: blank is valid (pi ships blank)
    }
    if err := provider.BuiltinManifests()[prov].ValidateModel(model); err != nil {
        t.Errorf("target=%s role=%s: ValidateModel(%q on %q) = %v", tc.target, role, model, prov, err)
    }
}

// PATTERN: the (target, installed) table — mirror ValidTOML + expand to every target × {nil, all}
cases := []tc{
    {"pi", []string{"pi"}}, {"pi", []string{"pi", "claude"}},
    {"claude", []string{"claude"}}, {"claude", []string{"claude", "pi"}},
    {"agy", []string{"agy", "pi", "claude"}},
}
for _, tgt := range allBuiltins {
    cases = append(cases, tc{tgt, nil}, tc{tgt, allBuiltins})   // item's "installed = nil and [all]"
}
```

### Integration Points

```yaml
NO database / routes / config-struct / production-code / docs changes. One new test file.

NEW TEST FILE:
  - internal/config/bootstrap_validate_test.go (`package config`) — TestBootstrapValidateModels + installedLabel.

CONSUMED (read-only):
  - internal/config: buildBootstrapConfig (bootstrap.go:143), materialize (file.go:219), fileConfig
    (file.go), ResolveRoleModel (roles.go:46), preferredBuiltins (bootstrap.go:16).
  - internal/provider: BuiltinManifests (builtin.go:18), Manifest.ValidateModel (manifest.go:136).
  - github.com/BurntSushi/toml (the project's TOML lib — see bootstrap_test.go).

RELATION TO SIBLINGS:
  - S1 (Complete): the code fix this test consumes (stager-fallback pi model blanked).
  - S2 (Implementing): substring-shape tests in bootstrap_test.go (model="", no gpt-5.4, guidance).
    T2.S1 is COMPLEMENTARY (semantic ValidateModel sweep), in a SEPARATE file — no overlap, no conflict.
  - P1.M2.T1 (Issue 2, commented pi block): T2.S1 does NOT validate commented blocks (they're inert).

UNCHANGED (do NOT touch): bootstrap.go (S1); bootstrap_test.go (S2 in parallel); role_defaults.go;
  manifest.go/builtin.go; the commented-block loop (Issue 2 = P1.M2.T1); docs (item point 5).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the test imports config + provider + toml; must compile)
go build ./...
# Vet the package
go vet ./internal/config/...
# Format check
gofmt -l internal/config/bootstrap_validate_test.go
# Expected: nothing listed. If listed: gofmt -w it.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The item's EXACT command — runs TestBootstrapValidateModels (all target×installed subtests)
go test ./internal/config/ -v -run TestBootstrapValidateModels
# Expected: every subtest PASSES. For target=agy/opencode/qwen-code/codex/cursor the stager resolves to
#           (pi, "") → skipped (blanked by S1). For target=claude the models (opus/sonnet/haiku) are
#           single-backend → ValidateModel OK. For target=pi all models blanked → all skipped.

# Full config package (regression — S1/S2 tests + existing bootstrap tests must stay green)
go test ./internal/config/... -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Test-only subtask — no binary behavior change to smoke. The within-scope proof is the unit test.
# Optional manual confirmation that the regression property holds (the test catches a bare pi model):
#   temporarily revert S1's guard (rm the `if stagerName == "pi" && stagerName != target { stagerModel = "" }`
#   line in bootstrap.go), re-run `go test ./internal/config/ -v -run TestBootstrapValidateModels`,
#   observe the agy/opencode/qwen-code rows FAIL with "model \"gpt-5.4-mini\" on pi must be inference/model",
#   then restore the guard. (This proves the test is a real guard, not a no-op. Optional — reasoning suffices.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: prove the test is `package config` (internal, not config_test) — required for buildBootstrapConfig + materialize
grep -n '^package config' internal/config/bootstrap_validate_test.go
# Expected: `package config` (NOT `package config_test`). This is the deliberate deviation — see Gotchas.

# Grep guard: prove the test imports provider (justified by bootstrap.go:9 precedent; no cycle)
grep -n 'internal/provider' internal/config/bootstrap_validate_test.go
# Expected: the import line.

# Grep guard: prove the core ValidateModel sweep is present
grep -n 'ValidateModel\|ResolveRoleModel\|materialize\|buildBootstrapConfig' internal/config/bootstrap_validate_test.go
# Expected: all four — the generate→parse→resolve→validate pipeline.

# Grep guard: prove the table covers nil + all installed (the item's requirement)
grep -n 'nil\|bootstrapValidateAllBuiltins\|allBuiltins' internal/config/bootstrap_validate_test.go
# Expected: the {tgt, nil} and {tgt, allBuiltins} case append loop.

# Scope-boundary guard: NO production file touched by this subtask
git diff --stat -- internal/config/bootstrap.go internal/config/role_defaults.go internal/provider/ docs/
# Expected: empty (test-only subtask; S1 owns bootstrap.go).

# Scope-boundary guard: bootstrap_test.go NOT touched (S2 owns it — avoid parallel-merge conflict)
git diff --stat -- internal/config/bootstrap_test.go
# Expected: empty (this subtask adds a NEW file; it does not edit bootstrap_test.go).

# Scope-boundary guard: commented blocks NOT validated (Issue 2 = P1.M2.T1)
grep -n 'commented\|Commented\|writeCommentedRoleBlock' internal/config/bootstrap_validate_test.go
# Expected: at most a comment explaining commented blocks are inert; no code that parses them.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/config/...` clean
- [ ] `gofmt -l internal/config/bootstrap_validate_test.go` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass
- [ ] `go test ./internal/config/ -v -run TestBootstrapValidateModels` passes (the item's exact command)

### Feature Validation
- [ ] Every built-in target × {nil, allBuiltins} is covered, plus the ValidTOML cases
- [ ] Every ACTIVE role model in every row is blank (skipped) or passes ValidateModel
- [ ] target=agy/opencode/qwen-code stager resolves to (pi, "") → skipped (post-S1)
- [ ] target=claude models (opus/sonnet/haiku) pass ValidateModel (single-backend)
- [ ] target=pi all models blanked → all skipped
- [ ] The test would FAIL on the pre-S1 tree (regression-guard property — verify by reasoning)

### Scope-Boundary Validation
- [ ] `package config` (internal), NOT `package config_test` (deliberate deviation — documented)
- [ ] NO production file touched (bootstrap.go, role_defaults.go, manifest.go, builtin.go unchanged)
- [ ] NO edit to bootstrap_test.go (S2 owns it; this subtask adds a SEPARATE file)
- [ ] Commented role blocks NOT validated (Issue 2 = P1.M2.T1)
- [ ] NO docs change (item point 5)
- [ ] NO config-upgrade backup test (Issue 3 = P1.M2.T2)

### Code Quality
- [ ] The package-config-vs-config_test deviation is explained in a test-file comment (so a future reader
      understands why it's not config_test despite the stale architecture note)
- [ ] Failure messages include target + installed + role + model + provider for fast triage
- [ ] The table is table-driven and uses t.Run subtests (mirrors TestBuildBootstrapConfig_ValidTOML)
- [ ] Blank models are skipped (not errored) — FR-D2 intent

---

## Anti-Patterns to Avoid

- ❌ Don't use `package config_test` (external) — the item recommends it but it CANNOT call the unexported `buildBootstrapConfig` (needed for deterministic `installed=nil/[all]`) NOR the unexported `materialize` (needed because `Config.Roles` is `toml:"-"`). Use `package config` (internal). The "config must not import provider" invariant it cites is stale — `bootstrap.go:9` already imports provider (no cycle, verified). This is the #1 one-pass failure mode; the PRP's Gotchas explain it fully.
- ❌ Don't call `GenerateBootstrapConfig(target)` — it auto-detects `installed` via `$PATH` (non-deterministic; varies CI vs dev) and can't express the item's nil/[all] cases. Call the UNEXPORTED `buildBootstrapConfig(target, installed, nil)` (requires `package config`).
- ❌ Don't `toml.Unmarshal` into `*Config` directly — `Config.Roles` is `toml:"-"` so Roles stays nil and `ResolveRoleModel` finds no roles (the test passes vacuously, catching nothing). Parse into `fileConfig` then `materialize(&fc, ...)` — the production loadTOML path.
- ❌ Don't validate COMMENTED role blocks — they're TOML comments (`# [role.x]`), not decoded into `fileConfig`, and validating them is Issue 2's scope (P1.M2.T1). The test validates ACTIVE `[role.*]` blocks only, which is exactly what `fileConfig`/`materialize` give you for free.
- ❌ Don't error on a blank model — FR-D2 ships pi's models BLANK intentionally ("no universally-correct inference backend"). `ValidateModel("")` returns nil, but `if model == "" { continue }` is clearer about intent. Erroring on blank would make every pi-target row fail.
- ❌ Don't edit `bootstrap_test.go` — S2 (P1.M1.T1.S2) is Implementing it in parallel (adding opencode/qwen-code substring tests). Editing it risks a merge conflict AND duplicates S2's work. Add a NEW file (`bootstrap_validate_test.go`).
- ❌ Don't duplicate S2's substring-shape assertions (model="", no gpt-5.4, guidance) — S2 owns those. T2.S1 is the SEMANTIC `ValidateModel` sweep; complementary, not overlapping.
- ❌ Don't modify production code (S1's bootstrap.go fix is Complete; manifest.go/builtin.go are read-only here). This subtask is TEST-ONLY (item point 5).
- ❌ Don't construct "minimal provider.Manifest" instances by hand — use `provider.BuiltinManifests()[prov]` (the real manifests). Hand-constructing risks diverging from the real ProviderFlag values (the whole point of ValidateModel is to use the REAL FR-R5b rule).
- ❌ Don't forget the regression-property check — confirm (by reasoning or a temporary local revert) that the test FAILS on the pre-S1 tree. A test that passes on both old and new code is not a guard.
- ❌ Don't pass a non-deterministic `installed` (e.g. from `BuiltinManifests()` map keys — unordered) when you need the `[all]` case — map iteration order is non-deterministic. Use the ordered `preferredBuiltins` (or the literal list mirroring it) so the `[all]` case is stable. (Order only affects commented-block appearance, which isn't validated — but stability is good hygiene.)

---

## Confidence Score: 9/10

One-pass success is high: the pipeline (generate→fileConfig→materialize→ResolveRoleModel→ValidateModel)
uses the production load path verbatim, the table mirrors an existing test, and the regression property
is clear (blank→skip=green; bare→error=red). The -1 is for the one genuine judgment call this PRP
resolves against the item's literal text: **the item says `package config_test`, but that is impossible
given the item's OWN requirements** (installed=nil/[all] needs unexported buildBootstrapConfig; Roles
population needs unexported materialize). An implementer who follows the item's package recommendation
without reading this PRP's deviation note will stall at two compile walls. The PRP makes the
`package config` decision explicit and evidence-backed (bootstrap.go:9 precedent; no cycle), which is
the single thing that unlocks one-pass success. Everything else is a mechanical mirror of existing patterns.
