---
name: "P1.M3.T1.S1 — CurrentConfigVersion → 3 + in-memory auto-migration on load (FR-B7): fold the removed default_provider into the model slash-prefix for multi-backend providers, in memory, before DecodeUserOverrides — PRD §9.17 FR-B7 / §16.1"
description: |

  Land the FIRST subtask of Config v3 migration (P1.M3.T1): (a) bump `CurrentConfigVersion` 2→3
  (`internal/config/config.go:18`); (b) add an IN-MEMORY migration step in `Load` that, when a loaded
  config predates v3, folds the removed `default_provider` field into a slash-PREFIX on `model` for
  multi-backend providers; (c) emit a ONE-TIME deprecation notice pointing at `config upgrade`. This is
  the load-side half of FR-B7 — the on-disk `config upgrade` rewrite is S2 (P1.M3.T1.S2).

  WHY MIGRATE: P1.M1.T1.S1 REMOVED `DefaultProvider` from the Manifest (the inference backend now rides
  the model slash-prefix, e.g. `zai/glm-5.2`, per FR-R5b). A v2 config file's `[provider.pi]
  default_provider = "zai"` flows as a RAW entry in `Config.Providers` (`config.go:93`,
  `map[string]map[string]any`). When a consumer calls `provider.DecodeUserOverrides(cfg.Providers)`
  (`registry.go:154`), it re-encodes the map to TOML and unmarshals into the v3 Manifest — which no
  longer has `default_provider` — so go-toml SILENTLY DROPS the value and pi's model is left BARE (an
  FR-R5b error). The migration must fold it BEFORE that, while it is still in the raw map.

  THE MIGRATION (PRD §9.17 FR-B7, AUTHORITATIVE):
    trigger:  fileLoaded && cfg.ConfigVersion < CurrentConfigVersion  (covers 0/1/2)
    step 0:   agent→provider terminology map — NO-OP in memory (see ⚠️ #3)
    step 1:   for each provider name with a non-empty raw default_provider="X" that is MULTI-BACKEND:
                delete cfg.Providers[name]["default_provider"];
                if cfg.Providers[name]["default_model"] is non-empty & bare → "X/<default_model>"
    step 2:   global: if cfg.Provider==name and cfg.Model non-empty & bare → cfg.Model = "X/<model>"
    step 3:   per-role: for each role r whose effective provider (Roles[r].Provider or cfg.Provider)
              == name and Roles[r].Model non-empty & bare → "X/<model>"
    then:     cfg.ConfigVersion = CurrentConfigVersion  (in-memory config is now v3-shaped)
    notice:   migrationNotice(originalVersion) → noticeOut (one-time; points at `config upgrade`)

  ⚠️ **#1 — multi-backend is classified WITHOUT importing internal/provider (decoupling invariant).**
  Config deliberately does NOT import provider (`Config.Providers` is a raw map so config need not know
  the Manifest type — `file_test.go` enforces this). In the v3 tree ONLY `builtinPi()` has a non-empty
  `ProviderFlag` (`builtin.go`: pi `strPtr("--provider")`; claude/gemini/agy/opencode/codex/cursor are all
  `strPtr("")`). opencode/agy route their backend via the model slash-prefix WITHOUT provider_flag and
  never carried a default_provider in v2. ⇒ multi-backend = a LOCAL `v2MultiBackendBuiltins = {"pi"}` set
  OR a user-defined provider whose raw map sets a non-empty `"provider_flag"`. Do NOT import provider.
  See research design-decisions.md §1.

  ⚠️ **#2 — fold keys off default_provider PRESENCE; idempotent; INVENTS NOTHING (FR-B7).** Only fold when
  the default_provider value X is non-empty AND the target model is non-empty AND bare
  (`!strings.Contains(model,"/")`). A bare model with no default_provider STAYS bare → FR-R5b error the
  user resolves. Single-backend providers are UNTOUCHED (a meaningless default_provider on one just has
  its dead key dropped, no fold). Re-running is a no-op (models already prefixed; key deleted). The
  raw-map model field is **`default_model`** (manifest.go:52), NOT `model` — the contract's "model" is
  generic shorthand. See design-decisions.md §2.

  ⚠️ **#3 — the "agent → provider" step is a DOCUMENTED IN-MEMORY NO-OP.** `fileConfig` (file.go:26-35) has
  NO `Agent` field and `loadTOML` uses `toml.Unmarshal` (file.go:134) which SILENTLY DROPS unknown
  `[agent.*]` tables — so no agent-keyed data ever reaches the typed Config (verified: `grep toml:"agent"
  | [agent | Agent map internal/` → empty). The in-memory agent step is therefore a no-op WITH a doc
  comment explaining why — do NOT chase non-existent data. The REAL `[agent.*]`→`[provider.*]` textual
  rewrite is S2's on-disk `config upgrade` (raw TOML, where `[agent.*]` survives). See §3.

  ⚠️ **#4 — migration runs INSIDE Load, BEFORE the caller's DecodeUserOverrides; then sets
  ConfigVersion=3.** DecodeUserOverrides would silently drop default_provider (Manifest field removed) —
  so the fold must happen while it is still in the raw map ⇒ inside Load, before `return &cfg`. After
  migrating, set `cfg.ConfigVersion = CurrentConfigVersion` (in-memory is v3-shaped; prevents re-trigger;
  accurate). Capture the ORIGINAL version for the notice first. See §4.

  ⚠️ **#5 — ONE deprecation notice, no double-notice.** Today Load calls `configVersionNotice` (load.go:152)
  which prints an "older" advisory. Restructure: the migration branch prints `migrationNotice(orig)`
  (FR-B7-specific) and `configVersionNotice` moves to an `else if` that now only fires for the AHEAD
  (version > current) case. `configVersionNotice`'s older/missing branches stay as pure tested utilities
  (no longer called by Load). See §5.

  ⚠️ **#6 — S1 MUST keep `go test ./...` GREEN: fix ALL bump+migration breakage.** Bumping 2→3 + the
  migration breaks ~44 test assertions that hardcode `config_version = 2` / `current is 2` / `supports up
  to 2` across internal/config/{load,bootstrap,file}_test.go (15) + internal/cmd/{config,default_action}
  _test.go (29). S1 fixes them: OUTPUT assertions → 3 (or vs `config.CurrentConfigVersion`); Load-notice
  tests → migration-notice expectations; KEEP INPUT v2 fixtures as "2" (they represent v2 files being
  migrated). S2 owns the `config upgrade` rewrite-BEHAVIOR tests. See §6 (full breakage map).

  ⚠️ **#7 — config upgrade --help text (Mode A DOCS): accurate + forward-compatible.** Update
  `runConfigUpgrade`'s `Long` (cmd/config.go:95) to note v3 folds default_provider into the model prefix
  and that older configs auto-migrate IN MEMORY on load (recommend `config upgrade` to persist). Word it
  to stay true after S2's on-disk rewrite. S1 does NOT implement the on-disk rewrite (S2).

  Deliverable: NEW `internal/config/migrate.go` (`migrateV2ToV3` + `isMultiBackend` + `migrationNotice` +
  `v2MultiBackendBuiltins`); MODIFIED `internal/config/config.go` (CurrentConfigVersion 2→3 + doc comment);
  MODIFIED `internal/config/load.go` (wire migration into Load + restructure the notice call); MODIFIED
  `internal/cmd/config.go` (`config upgrade` Long text); MODIFIED tests to green (the breakage map in §6);
  NEW `internal/config/migrate_test.go`. NO go.mod change (stdlib only — `fmt`+`strings`). OUTPUT: v2
  config files load correctly under v3 (models prefixed in memory, default_provider gone); a one-time
  deprecation notice is shown; `go build/vet/test ./...` green.

---

## Goal

**Feature Goal**: Implement the load-side of the v3 config migration (PRD §9.17 FR-B7): bump the schema
version to 3 and, when a loaded config file predates v3, transparently fold the removed `default_provider`
field into a `model` slash-prefix for multi-backend providers — in memory, before any consumer's
`DecodeUserOverrides` would silently drop it — across the global model, every per-role model, and the raw
provider-overrides map; emit a one-time deprecation notice pointing at `config upgrade`. A v2 file thus
loads correctly under v3 with no user action; nothing is invented (bare models stay bare → FR-R5b).

**Deliverable** (NEW + MODIFIED files, stdlib only):
1. **NEW `internal/config/migrate.go`** (`package config`, imports `fmt`+`strings`) —
   `func migrateV2ToV3(cfg *Config)`, `func isMultiBackend(name string, raw map[string]any) bool`,
   `func migrationNotice(originalVersion int) string`, `var v2MultiBackendBuiltins = map[string]bool{"pi": true}`.
2. **MODIFIED `internal/config/config.go`** — `const CurrentConfigVersion = 3` (+ refresh the const doc
   comment to describe v3 = inference-provider-folded-into-model).
3. **MODIFIED `internal/config/load.go`** — replace the `configVersionNotice` call site (L152) with the
   migration branch (trigger on `fileLoaded && cfg.ConfigVersion < CurrentConfigVersion`) + `else if`
   ahead-case notice; calls `migrateV2ToV3(&cfg)` + sets `cfg.ConfigVersion = CurrentConfigVersion` +
   writes `migrationNotice(orig)` to `noticeOut`.
4. **MODIFIED `internal/cmd/config.go`** — `runConfigUpgrade` `Long` text (Mode A: note v3 fold +
   in-memory auto-migration).
5. **NEW `internal/config/migrate_test.go`** — table-driven tests for `migrateV2ToV3` (global/per-role/
   raw-map fold; idempotency; single-backend untouched; empty/bare no-invent; empty-default_provider
   key-drop) + `migrationNotice` + `isMultiBackend`.
6. **MODIFIED tests to green** — the §6 breakage map (`internal/config/{load,bootstrap,file}_test.go`,
   `internal/cmd/{config,default_action}_test.go`).

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean;
`CurrentConfigVersion == 3`; loading a v2 file with `[provider.pi] default_provider = "zai"` + a bare pi
model yields `cfg.Model == "zai/<model>"` (global + per-role + raw map), `default_provider` deleted from
the raw map, `cfg.ConfigVersion == 3`, and a one-time deprecation notice; a single-backend provider
(claude) is untouched; a bare pi model with no default_provider stays bare; go.mod/go.sum byte-unchanged.

## User Persona

**Target User**: Any user loading a v2 (or unversioned) config under the v3 binary (P1.M1.T1.S1 removed
`DefaultProvider` from the Manifest). Transitively: the **provider registry** (`DecodeUserOverrides` +
`NewRegistry`), which consumes `cfg.Providers` AFTER Load — it must see folded models and no
`default_provider`, or pi routes to the wrong backend (the original FR-R5b bug). And the **decompose /
generate pipelines** whose `Render(model,…)` now expects the slash-prefix (FR-R5b, P1.M2.T1.S2).

**Use Case**: A user upgrades the stagecoach binary (v3) but keeps their v2 `~/.config/stagecoach/config.toml`
with `[provider.pi] default_provider = "zai"` and `[defaults] model = "glm-5.2"`. On load, stagecoach folds
in memory → `cfg.Model = "zai/glm-5.2"`, drops the dead key, prints one deprecation notice, and runs
correctly. The user later runs `config upgrade` (S2) to persist the rewrite to disk.

**User Journey**: `config.Load` → overlays resolve `cfg.ConfigVersion=2` → migration branch fires →
`migrateV2ToV3(&cfg)` (fold + delete key) → `cfg.ConfigVersion=3` → `migrationNotice(2)` printed →
`return &cfg` → consumer `DecodeUserOverrides(cfg.Providers)` sees clean v3 data → `Render("zai/glm-5.2",…)`.

**Pain Points Addressed**: Without migration, a v2 config under v3 silently loses pi's backend (the dropped
`default_provider`) → pi routes to the wrong upstream (the original FR-R5b misroute) or errors on a bare
model. The in-memory fold makes v2→v3 transparent; the notice guides the user to persist it.

## Why

- **Required by P1.M1.T1.S1's Manifest change.** Removing `DefaultProvider` from the Manifest (inference
  backend now in the model string, FR-R5b) makes every existing v2 config's `default_provider` a dead key
  that `DecodeUserOverrides` silently drops. The migration rescues that value (folds it) — otherwise v2
  users hit a hard regression on upgrade.
- **Satisfies PRD §9.17 FR-B7.** v3 "folds the inference provider into the model string … auto-migrated
  on load for older files … emit a one-time deprecation notice … no value is invented." This subtask IS
  the load-side of FR-B7.
- **In-memory (not on-disk) by design.** FR-B7 migrates in memory so a v2 file works immediately with no
  file mutation; the on-disk `config upgrade` (S2) persists it. Load is the right place: it owns the
  resolved `*Config` before any consumer touches `cfg.Providers`.
- **Preserves the decoupling invariant.** The migration classifies multi-backend LOCALLY (`{"pi"}` +
  raw `provider_flag`) so `internal/config` still imports nothing from `internal/provider` — no layering
  regression.

## What

A new `migrate.go` (3 funcs + 1 var), a one-line version bump + doc refresh, a Load restructure (migration
branch replaces the older/missing notice branches), a `config upgrade` --help touch, a new migration test
file, and the mechanical/behavioral test updates to keep the suite green. No new deps, no new types on
`Config`, no change to `file.go`/decode/overlay (the migration operates on the already-resolved `Config`).

### Success Criteria

- [ ] `internal/config/config.go`: `const CurrentConfigVersion = 3` (+ refreshed doc comment: v3 = inference
      provider folded into model string, FR-B7).
- [ ] `internal/config/migrate.go` exists, `package config`, imports EXACTLY `fmt`+`strings`: `migrateV2ToV3`,
      `isMultiBackend`, `migrationNotice`, `v2MultiBackendBuiltins`.
- [ ] `migrateV2ToV3` folds per §2: global `Model`, per-role `Roles[r].Model` (effective provider), raw
      `Providers[name]["default_model"]`; deletes `default_provider`; idempotent; no-invent; single-backend
      untouched; agent step is a documented no-op.
- [ ] `Load` runs `migrateV2ToV3(&cfg)` + sets `cfg.ConfigVersion = CurrentConfigVersion` + writes
      `migrationNotice(orig)` when `fileLoaded && cfg.ConfigVersion < CurrentConfigVersion`; the ahead-case
      still uses `configVersionNotice`; NO double notice.
- [ ] `migrationNotice(originalVersion)` is PURE; handles version 0 (no config_version) and 1/2 (older);
      points at `config upgrade`.
- [ ] `internal/cmd/config.go` `runConfigUpgrade` `Long` notes the v3 fold + in-memory auto-migration.
- [ ] `internal/config/migrate_test.go`: table-driven coverage of every migration branch + notice + classifier.
- [ ] ALL bump+migration test breakage fixed (§6 map); `go build ./... && go vet ./... && go test ./...` GREEN;
      `gofmt -l` clean; go.mod/go.sum byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact migration algorithm
(§2), the multi-backend classification rationale (§1), the agent-no-op finding (§3), the Load wiring + notice
restructure (§4/§5), the copy-ready `migrate.go` in the Blueprint, and the §6 breakage map. No decompose/
render/git knowledge required — the migration is a pure transformation on `*Config` + a Load restructure.

### Documentation & References

```yaml
# MUST READ — the design calls + the agent-no-op finding + the breakage map
- docfile: plan/003_6ce49c39466e/P1M3T1S1/research/design-decisions.md
  why: §1 (multi-backend WITHOUT importing provider; v2MultiBackendBuiltins={"pi"}), §2 (fold algorithm;
       default_model not model; idempotent; no-invent), §3 (agent step is an in-memory NO-OP — fileConfig
       has no Agent field; go-toml drops [agent.*]), §4 (inside Load before DecodeUserOverrides; set
       ConfigVersion=3), §5 (one notice, no double), §6 (the full test-breakage map), §7 (--help text).
  critical: §1 (do NOT import provider), §3 (do NOT chase agent data — it isn't there), §6 (fix every
       breakage to stay green) are the things most likely to derail one-pass success.

# MUST READ — the authoritative touchpoint map (config_version read/compare/write, DecodeUserOverrides path)
- docfile: plan/003_6ce49c39466e/architecture/scout_config_model.md
  section: "§(d) CurrentConfigVersion + config_version read/compare/write" (every site — the bump touches
           config.go:18; the notice is load.go:152/302; DecodeUserOverrides is registry.go:154).
  section: "§(f) [provider.<name>] default_provider decode/use path (RAW map)" — confirms default_provider
           flows as raw `any` in Config.Providers and DecodeUserOverrides unmarshals it into Manifest.
  critical: §(d) flags that configVersionNotice (load.go:302) is the v3 migrate trigger and that test
       fixtures hardcode "config_version = 2" (the §6 breakage). §(f) confirms default_provider is raw,
       not typed — so the migration reads/rewrites `cfg.Providers[name]["default_provider"]` directly.

# The PRD basis (in your context as selected_prd_content)
- file: PRD.md (or plan/003_6ce49c39466e/prd_snapshot.md)
  section: "9.17 Config bootstrap & versioning" FR-B7 (h3.33) — AUTHORITATIVE migration spec: the
           default_provider→model-prefix mapping, per-role + global, single-backend untouched, no-invent,
           agent/[agent.*] mapped first, auto-migrate in memory + one-time deprecation notice.
  section: "16.1 Resolution order" — "config_version is metadata, NOT a precedence layer" (why the
           migration transforms values but the version is advisory).
  critical: FR-B7's "No value is invented" + "Single-backend providers are untouched" are hard constraints
       the migration + tests must honor.

# The contract input — P1.M1.T1.S1 (DefaultProvider removed)
- file: internal/provider/manifest.go
  why: confirms DefaultProvider is GONE (only ProviderFlag remains, L58; Resolve defaults it to strPtr("")
       at L161-162). This is WHY the migration is needed (DecodeUserOverrides would drop default_provider).
  critical: do NOT re-add DefaultProvider. The migration folds its value into the model string instead.

# The file being modified
- file: internal/config/load.go
  section: Load (L76-160) — the overlay pipeline + the configVersionNotice call site (L152) that S1
           restructures into the migration branch + ahead-case else-if. configVersionNotice (L298-318).
  why: the EXACT wiring point. Migration runs after the Commits==1 normalize (L148), before `return &cfg`.
  critical: migration MUST be inside Load (before return) so it precedes the caller's DecodeUserOverrides.

- file: internal/config/config.go
  section: `const CurrentConfigVersion` (L18 → 3); `Config.Providers map[string]map[string]any` (L93);
           `Config.Roles map[string]RoleConfig` (L109); `Config.Model` (L64); `Config.Provider` (L63).
  why: the migration reads/writes these. Note Defaults() sets ConfigVersion=0 (UNSET) — leave that; the
       migration keys off the RESOLVED value.
  critical: do NOT change Config struct fields; only the const value + its doc comment.

- file: internal/config/file.go
  section: fileConfig (L26-35) — has NO Agent field; loadTOML uses toml.Unmarshal (L134, drops unknown keys).
  why: PROVES the agent step is an in-memory no-op (§3). Do NOT add an Agent field or raw-text re-read.

- file: internal/provider/builtin.go
  why: PROVES only builtinPi() has ProviderFlag non-empty (L51); all others strPtr("") — the basis for
       v2MultiBackendBuiltins={"pi"} (§1).
- file: internal/provider/registry.go
  section: DecodeUserOverrides (L154) — re-encodes raw map→TOML→Manifest; would silently drop default_provider.
  why: confirms the ordering constraint (migration must precede it) — but DecodeUserOverrides is NOT edited
       (it's a consumer, called after Load).

# The Mode-A doc touch
- file: internal/cmd/config.go
  section: runConfigUpgrade Long (L95-104) — S1 updates the --help text (v3 fold + in-memory auto-migration).
  critical: S1 edits ONLY the Long text; the on-disk rewrite LOGIC (upgradeConfigVersion L178) is S2.

# The parallel PRP (P1.M2.T1.S2) — confirms the model-prefix / FR-R5b consumer side
- file: plan/003_6ce49c39466e/P1M2T1S2/PRP.md
  why: ResolveRoles v3 enforces FR-R5b (bare-model-on-pi ⇒ error) at the decompose layer. The migration
       FEEDS that guard: a migrated config has prefixed models (passes); an unmigrated bare model fails.
       S1 and S2 are consistent — both treat `(backend, model)` as `backend/model`.
  critical: do NOT duplicate S2's work (ResolveRoles/Render/decompose). S1 is config-only.
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go       # CurrentConfigVersion (L18) + Config struct — EDIT (const 2→3 + doc)
  load.go         # Load (L76) + configVersionNotice (L298) — EDIT (migration branch + notice restructure)
  file.go         # fileConfig/loadTOML — UNCHANGED (proves agent no-op)
  migrate.go      # NEW (this subtask) ← migrateV2ToV3 + isMultiBackend + migrationNotice + v2MultiBackendBuiltins
  migrate_test.go # NEW (this subtask) ← table-driven migration/notice/classifier tests
  {load,bootstrap,file}_test.go — EDIT (bump+migration breakage, §6)
internal/cmd/
  config.go       # runConfigUpgrade Long — EDIT (--help text, Mode A)
  {config,default_action}_test.go — EDIT (bump breakage, §6)
internal/provider/  # UNCHANGED (manifest.go DefaultProvider already removed in P1.M1.T1.S1; DecodeUserOverrides is a consumer)
go.mod / go.sum     # UNCHANGED (stdlib fmt+strings only)
```

### Desired Codebase tree with files to be added

```bash
internal/config/migrate.go          # NEW — the v3 in-memory migration (FR-B7)
internal/config/migrate_test.go     # NEW — migration/notice/classifier tests
# All other changes are in-place edits (config.go const, load.go wiring, cmd/config.go help, tests).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — do NOT import internal/provider): config stays decoupled (Config.Providers is a raw map;
// file_test.go enforces this). Classify multi-backend LOCALLY via v2MultiBackendBuiltins={"pi"} + raw
// provider_flag. Only builtinPi() has ProviderFlag!="" in v3; opencode/agy use the model prefix WITHOUT
// provider_flag and never had default_provider in v2. (design-decisions §1)

// CRITICAL (#2 — fold is idempotent + no-invent; raw key is default_model): fold only when default_provider
// X!="" AND model!="" AND !strings.Contains(model,"/"). Single-backend ⇒ drop dead key, NO fold. The raw
// provider-map model field is "default_model" (manifest.go:52), NOT "model". (§2)

// CRITICAL (#3 — agent step is an in-memory NO-OP): fileConfig has NO Agent field; toml.Unmarshal drops
// [agent.*]. No agent data reaches cfg. The in-memory agent step is a no-op + doc comment — do NOT invent a
// data path. The real [agent.*]→[provider.*] rewrite is S2's on-disk config upgrade. (§3)

// CRITICAL (#4 — migration inside Load, BEFORE DecodeUserOverrides; then ConfigVersion=3): DecodeUserOverrides
// (registry.go:154) would silently drop default_provider (Manifest field gone). Migration must run while it's
// in the raw map ⇒ inside Load before return. After: cfg.ConfigVersion=CurrentConfigVersion (v3-shaped).

// CRITICAL (#5 — ONE notice; restructure Load): the migration branch prints migrationNotice(orig); move
// configVersionNotice to an else-if that now only handles the AHEAD case. No double notice. (§5)

// CRITICAL (#6 — fix ALL bump+migration test breakage; keep go test ./... green): ~44 sites hardcode
// "config_version = 2"/"current is 2"/"supports up to 2". OUTPUT assertions → 3; Load-notice tests →
// migration expectations; KEEP INPUT v2 fixtures as "2". S2 owns the upgrade rewrite-behavior tests. (§6)

// GOTCHA (map returns a copy): cfg.Roles[r] is a value type in the map — mutate a local copy and write it
// back (rc := cfg.Roles[r]; rc.Model = …; cfg.Roles[r] = rc). cfg.Providers[name] is map[string]any — mutate
// in place (delete/raw["default_model"]=…).
// GOTCHA (go-toml decodes TOML string → Go string): raw["default_provider"].(string) with comma-ok; absent ⇒
// ok=false; non-string ⇒ no fold (do NOT fmt.Sprintf a non-string).
// GOTCHA (noticeOut is the test-seam writer): migrationNotice is PURE (returns string); Load writes it to
// noticeOut (load.go:153) — mirrors configVersionNotice's testability. Do NOT do I/O inside migrationNotice.
// GOTCHA (ConfigVersion after migration): set to CurrentConfigVersion so TestLoad_ConfigVersion (load_test.go)
// must expect 3 for a v2-file load, and so the migration branch can't re-trigger on an already-migrated cfg.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/migrate.go
package config

import (
	"fmt"
	"strings"
)

// v2MultiBackendBuiltins names the v2 built-in providers whose manifests carried a default_provider (a
// non-empty provider_flag) — the only providers a v2 default_provider could meaningfully apply to. In the
// v3 tree ONLY builtinPi() has ProviderFlag != "" (internal/provider/builtin.go); opencode/agy route their
// inference backend via the model slash-prefix WITHOUT a provider_flag and never carried a default_provider
// in v2. A user-defined provider is multi-backend iff its raw cfg.Providers entry sets a non-empty
// "provider_flag" (isMultiBackend checks both). LOCAL to config so the v3 migration can classify providers
// WITHOUT importing internal/provider (the raw-map decoupling invariant). Migration shim — add a name here
// if a future built-in gains a provider_flag.
var v2MultiBackendBuiltins = map[string]bool{"pi": true}

// migrateV2ToV3 performs the PRD §9.17 FR-B7 IN-MEMORY migration on a resolved *Config whose ConfigVersion
// predates 3. It folds the removed default_provider field into a slash-PREFIX on model for multi-backend
// providers, in three places: the global Config.Model, each per-role Config.Roles[r].Model, and the raw
// Config.Providers[name]["default_model"] entry; then deletes the default_provider key. IDEMPOTENT and
// INVENTS NOTHING (FR-B7): folds only when default_provider X is non-empty AND the target model is
// non-empty and bare (no "/"). A bare model with no resolvable prefix STAYS bare (becomes an FR-R5b error
// the user resolves). Single-backend providers are UNTOUCHED (a meaningless default_provider just drops).
//
// Load calls this BEFORE the caller's provider.DecodeUserOverrides (registry.go): DecodeUserOverrides
// re-encodes Config.Providers to TOML and unmarshals into the v3 Manifest — which no longer has a
// default_provider field (removed in P1.M1.T1.S1) — so go-toml would SILENTLY DROP the value. The fold must
// happen first, while default_provider is still in the raw map.
//
// AGENT TERMINOLOGY (FR-B7 "first map agent/[agent.*] → provider/[provider.*]"): a NO-OP in memory.
// fileConfig (file.go) decodes only the [provider] table (no Agent field) and toml.Unmarshal silently DROPS
// unknown [agent.*] tables — so no agent-keyed data ever reaches the typed Config. The real textual rewrite
// is the on-disk `config upgrade` command's job (P1.M3.T1.S2), which reads raw TOML where [agent.*] survive.
func migrateV2ToV3(cfg *Config) {
	// (0) agent→provider: documented no-op (no agent data reaches cfg — see doc comment).

	// (1) Collect the default_provider prefix per multi-backend provider; drop the dead key; fold the
	// provider's own raw default_model.
	prefix := map[string]string{} // provider name -> former default_provider value X
	for name, raw := range cfg.Providers {
		dp, ok := raw["default_provider"]
		if !ok {
			continue
		}
		delete(raw, "default_provider") // the field is gone in v3 — drop the dead key regardless
		x, _ := dp.(string)             // go-toml decodes a TOML string to Go string
		if x == "" || !isMultiBackend(name, raw) {
			continue // empty value, or single-backend/unknown ⇒ no fold (FR-B7)
		}
		prefix[name] = x
		if dm, ok := raw["default_model"]; ok { // raw model field is "default_model" (manifest.go:52)
			if s, ok := dm.(string); ok && s != "" && !strings.Contains(s, "/") {
				raw["default_model"] = x + "/" + s
			}
		}
	}
	if len(prefix) == 0 {
		return // nothing to fold into global/per-role models
	}

	// (2) Global model — folds only if cfg.Provider is a multi-backend provider with a prefix.
	if cfg.Model != "" && !strings.Contains(cfg.Model, "/") {
		if x, ok := prefix[cfg.Provider]; ok {
			cfg.Model = x + "/" + cfg.Model
		}
	}

	// (3) Per-role models — effective provider = role override if set, else the global.
	for r, rc := range cfg.Roles {
		if rc.Model == "" || strings.Contains(rc.Model, "/") {
			continue
		}
		ep := rc.Provider
		if ep == "" {
			ep = cfg.Provider
		}
		if x, ok := prefix[ep]; ok {
			rc.Model = x + "/" + rc.Model
			cfg.Roles[r] = rc // map values are copies — write back
		}
	}
}

// isMultiBackend reports whether provider `name` is a multi-backend (provider_flag) provider per v3
// semantics, WITHOUT importing internal/provider. True if name is a known v2 built-in multi-backend
// (v2MultiBackendBuiltins) OR the raw provider map explicitly sets a non-empty "provider_flag". Used only
// by the v3 migration.
func isMultiBackend(name string, raw map[string]any) bool {
	if v2MultiBackendBuiltins[name] {
		return true
	}
	if pf, ok := raw["provider_flag"]; ok {
		if s, ok := pf.(string); ok && s != "" {
			return true
		}
	}
	return false
}

// migrationNotice returns the ONE-TIME PRD §9.17 FR-B7 deprecation notice emitted when a <v3 config is
// auto-migrated in memory. originalVersion is the file's declared version (0 ⇒ none declared). PURE (no
// I/O); Load writes it to noticeOut. Points the user at `config upgrade` to persist the migration.
func migrationNotice(originalVersion int) string {
	if originalVersion == 0 {
		return "stagecoach: config file has no config_version — treated as legacy and auto-migrated in " +
			"memory (the `default_provider` field was folded into the `model` slash-prefix, FR-B7). " +
			"Run 'stagecoach config upgrade' to persist this to the file.\n"
	}
	return fmt.Sprintf("stagecoach: config schema version %d (current %d) — auto-migrated in memory "+
		"(the `default_provider` field was folded into the `model` slash-prefix, FR-B7). "+
		"Run 'stagecoach config upgrade' to persist this to the file.\n",
		originalVersion, CurrentConfigVersion)
}
```

```go
// internal/config/load.go — REPLACE the configVersionNotice call site (L152) with:
//
// 	// v3 in-memory migration (PRD §9.17 FR-B7): when a loaded file predates v3, fold any v2
// 	// default_provider into the model slash-prefix BEFORE the caller's provider.DecodeUserOverrides
// 	// (which would silently drop the now-removed field). Idempotent; invents nothing. The in-memory
// 	// Config is then v3-shaped (ConfigVersion set to current). One-time deprecation notice below.
// 	if fileLoaded && cfg.ConfigVersion < CurrentConfigVersion {
// 		orig := cfg.ConfigVersion
// 		migrateV2ToV3(&cfg)
// 		cfg.ConfigVersion = CurrentConfigVersion
// 		fmt.Fprint(noticeOut, migrationNotice(orig))
// 	} else if msg := configVersionNotice(fileLoaded, cfg.ConfigVersion); msg != "" {
// 		// version > current (ahead) — the only remaining live configVersionNotice case in Load
// 		// (version==current ⇒ ""; the older/missing cases are handled by the migration branch above).
// 		fmt.Fprint(noticeOut, msg)
// 	}
//
// 	return &cfg, nil
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: BUMP CurrentConfigVersion 2→3 (config.go)
  - EDIT config.go: `const CurrentConfigVersion = 3`. Refresh its doc comment: v3 = inference provider
      folded into the model slash-prefix (FR-B7), auto-migrated on load; v2 = per-role/decompose/binary.
  - DO NOT change Defaults() (ConfigVersion stays 0 — the UNSET sentinel; the migration keys off resolved).
  - GOTCHA: this bump is what breaks the ~44 test assertions (Task 5).

Task 2: CREATE internal/config/migrate.go (migrateV2ToV3 + isMultiBackend + migrationNotice + var)
  - PACKAGE config; IMPORTS EXACTLY fmt, strings (NO internal/provider).
  - IMPLEMENT per the Blueprint. v2MultiBackendBuiltins = {"pi"}.
  - GOTCHA: agent step = documented no-op (no data path). Raw model key = "default_model". Map values are
      copies (write back Roles[r]). comma-ok on every raw.(string). migrationNotice is PURE.

Task 3: WIRE migration into Load (load.go) + restructure the notice
  - REPLACE the L152 configVersionNotice call site with the migration branch + else-if ahead-case (Blueprint).
  - Capture orig := cfg.ConfigVersion BEFORE migrate; set cfg.ConfigVersion = CurrentConfigVersion AFTER.
  - KEEP configVersionNotice (L298) intact as a pure utility (its ahead branch is still live; older/missing
      branches stay tested but are no longer Load's path).
  - GOTCHA: migration MUST be before `return &cfg` (precede the caller's DecodeUserOverrides).

Task 4: UPDATE config upgrade --help text (cmd/config.go, Mode A)
  - EDIT runConfigUpgrade's Long (L95): add that v3 folds default_provider into the model slash-prefix and
      that loading an older config auto-migrates IN MEMORY (run `config upgrade` to persist). Stay accurate
      post-S2. DO NOT edit upgradeConfigVersion (L178) — the on-disk rewrite is S2.

Task 5: FIX bump+migration test breakage (keep go test ./... green) — the §6 map
  - internal/config/load_test.go (10): TestConfigVersionNotice ("current is 2"→"3", "supports up to 2"→"3",
      or assert vs CurrentConfigVersion); TestLoad_ConfigVersionAdvisory_Older → now expects the MIGRATION
      notice ("auto-migrated in memory", "config upgrade"), not the generic "schema version 1/current is 2";
      TestLoad_ConfigVersion → cfg.ConfigVersion==3 for a v2-file load (post-migration); TestLoad_ConfigVersion
      _Advisory_Missing → migration notice (version 0 branch).
  - internal/config/bootstrap_test.go (4): "config_version = 2" OUTPUT assertions → 3 (or vs CurrentConfigVersion).
  - internal/config/file_test.go (1): round-trip config_version int64(2) → int64(3).
  - internal/cmd/config_test.go (28): OUTPUT assertions "config_version = 2" → 3; KEEP INPUT fixtures
      (e.g. L860 `input := "config_version = 2\n"`) as "2" if they feed upgrade/migrate with a v2 file.
  - internal/cmd/default_action_test.go (1, L1203): a v2 INPUT fixture — KEEP "2" if it exercises migration;
      else update. Inspect context.
  - RUN `go test ./...` after each file; every remaining failure is the bump/migration — fix per §6.

Task 6: CREATE internal/config/migrate_test.go (table-driven)
  - migrateV2ToV3 cases: (a) global pi model folded (cfg.Provider="pi", [provider.pi] default_provider="zai"
      ⇒ cfg.Model="zai/glm-5.2"); (b) per-role folded (Roles[planner].Provider="pi" + bare model ⇒ prefixed);
      (c) role inheriting global pi (Roles[message].Provider="" + cfg.Provider="pi" ⇒ prefixed); (d) raw map
      default_model folded + default_provider deleted; (e) idempotency (run twice ⇒ same; already-prefixed
      "zai/x" untouched); (f) single-backend claude UNTOUCHED (default_provider dropped, model NOT prefixed);
      (g) empty default_provider ⇒ key dropped, no fold; (h) bare pi model with NO default_provider ⇒ stays
      bare (no-invent); (i) v3 config (ConfigVersion>=3 not migrated here — but the fn itself is version-
      agnostic; Load gates it); (j) nil Providers/Roles ⇒ no panic.
  - migrationNotice cases: version 0 (mentions "no config_version" + "config upgrade"); version 2 (mentions
      "schema version 2" + "current 3" + "config upgrade"); \n-terminated; PURE (no I/O).
  - isMultiBackend cases: "pi"⇒true; "claude"⇒false; user-defined with raw provider_flag="--x"⇒true; "" ⇒false.
  - GOTCHA: build Config literals directly (white-box, package config) — set ConfigVersion explicitly per case.

Task 7: VERIFY
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. file.go/manifest.go/registry.go/
      render.go/decompose/* byte-unchanged. PRD.md byte-unchanged. configVersionNotice's pure tests still green.
```

### Implementation Patterns & Key Details

```go
// THE fold (idempotent, no-invent) — only when X!="" and model bare:
if x, ok := prefix[cfg.Provider]; ok && cfg.Model != "" && !strings.Contains(cfg.Model, "/") {
	cfg.Model = x + "/" + cfg.Model
}
// per-role: effective provider = role override or global; map value is a COPY → write back.
for r, rc := range cfg.Roles {
	ep := rc.Provider; if ep == "" { ep = cfg.Provider }
	if x, ok := prefix[ep]; ok && rc.Model != "" && !strings.Contains(rc.Model, "/") {
		rc.Model = x + "/" + rc.Model; cfg.Roles[r] = rc
	}
}
// raw map: delete dead key; fold default_model (NOT "model").
delete(raw, "default_provider")
if dm, ok := raw["default_model"]; ok { if s, ok := dm.(string); ok && s != "" && !strings.Contains(s, "/") { raw["default_model"] = x + "/" + s } }

// THE Load wiring (replaces the L152 notice call): migration branch + ahead-case else-if; ONE notice.
if fileLoaded && cfg.ConfigVersion < CurrentConfigVersion {
	orig := cfg.ConfigVersion
	migrateV2ToV3(&cfg)
	cfg.ConfigVersion = CurrentConfigVersion
	fmt.Fprint(noticeOut, migrationNotice(orig))
} else if msg := configVersionNotice(fileLoaded, cfg.ConfigVersion); msg != "" {
	fmt.Fprint(noticeOut, msg)
}

// THE classifier — LOCAL, no provider import:
func isMultiBackend(name string, raw map[string]any) bool {
	if v2MultiBackendBuiltins[name] { return true }
	if pf, ok := raw["provider_flag"]; ok { if s, ok := pf.(string); ok && s != "" { return true } }
	return false
}
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. migrate.go uses stdlib fmt+strings only. `go mod tidy` is a no-op.

PACKAGE EDGES: NONE added. config STILL does not import internal/provider (the raw-map invariant). migrate.go
      is package config; it reads Config.Providers (raw map) + Config.Roles + Config.Model directly.

UPSTREAM (the inputs — consume, do NOT edit):
  - P1.M1.T1.S1: Manifest.DefaultProvider REMOVED (manifest.go) — the reason the migration exists.
  - file.go: fileConfig/loadTOML/materialize/overlay produce the resolved *Config the migration transforms.

DOWNSTREAM (the consumers — not this task):
  - provider.DecodeUserOverrides(cfg.Providers) (registry.go:154): runs AFTER Load; sees folded models +
    no default_provider (the migration's whole point).
  - decompose ResolveRoles / Render (P1.M2.T1.S2): enforce FR-R5b on the (now prefixed) models.
  - S2 (P1.M3.T1.S2): the on-disk `config upgrade` rewrite (upgradeConfigVersion extension) + rewrite tests.

FROZEN/LEAVE (do NOT edit):
  - internal/provider/* (manifest.go DefaultProvider already gone; registry.go DecodeUserOverrides is a consumer).
  - internal/config/file.go (proves the agent no-op; the migration operates on resolved Config, not decode).
  - internal/decompose/*, internal/generate/*, internal/cmd/{root,default_action,providers}.go, pkg/stagecoach/*.
  - PRD.md, go.mod, Makefile, providers/*.toml.
  - configVersionNotice (L298) stays a pure utility (ahead-case still live; do not delete its older/missing
    branches — they remain unit-tested).
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/config/migrate.go internal/config/config.go internal/config/load.go internal/cmd/config.go
go vet ./internal/config/ ./internal/cmd/
head -8 internal/config/migrate.go   # → import ( "fmt" "strings" ) — NO internal/provider
grep -n "CurrentConfigVersion" internal/config/config.go | head -1   # → const CurrentConfigVersion = 3
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; migrate.go imports only fmt+strings; const = 3; go.mod/go.sum byte-unchanged.
```

### Level 2: Config-package unit tests (the migration + no regression)

```bash
go test ./internal/config/ -v -run 'TestMigrateV2ToV3|TestMigrationNotice|TestIsMultiBackend'
go test ./internal/config/ -v -run 'TestConfigVersionNotice|TestLoad_ConfigVersion'
go test ./internal/config/
# Expected PASS — verify:
#   TestMigrateV2ToV3/* ........ global/per-role/raw-map fold; idempotent; single-backend untouched;
#                                bare-no-default_provider stays bare; nil-safe
#   TestMigrationNotice ......... version 0 + version 2 branches; \n-terminated; PURE
#   TestIsMultiBackend .......... pi=true, claude=false, user provider_flag=true
#   TestConfigVersionNotice ..... "current is 3"/"supports up to 3" (updated for the bump)
#   TestLoad_ConfigVersion ...... cfg.ConfigVersion==3 for a v2-file load (post-migration)
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...     # Expect clean.
go test ./...      # Expect all PASS — this is the §6 breakage-fix gate. Every failure is a hardcoded "2"
                   # (→ OUTPUT assertion: 3; → INPUT v2 fixture: keep 2) or a Load-notice expectation.
# Confirm frozen files byte-unchanged:
git diff --exit-code internal/provider internal/config/file.go internal/decompose internal/generate \
  pkg cmd/root.go cmd/default_action.go cmd/providers.go go.mod go.sum PRD.md \
  && echo "frozen files UNCHANGED (expected)"
# Confirm config still does not import provider:
! grep -rq "stagecoach/internal/provider" internal/config/ && echo "config does NOT import provider (good)"
# Straggler grep — no remaining hardcoded version-2 OUTPUT assertions (INPUT v2 fixtures may legitimately keep "2"):
grep -rn 'current is 2\|supports up to 2' internal/ && echo "STALE v2 notice assertion — fix" || echo "notice assertions updated (good)"
```

### Level 4: Migration-correctness reasoning (no runtime to start)

```bash
# The migration is a pure *Config transform — no server/DB/subprocess. The "integration" is the FR-B7 contract.
# Verify by reasoning + migrate_test.go:
#   1. A v2 file ([provider.pi] default_provider="zai" + [defaults] model="glm-5.2") ⇒ cfg.Model="zai/glm-5.2",
#      raw map default_provider DELETED, default_model folded, cfg.ConfigVersion=3, one notice. (TestMigrateV2ToV3)
#   2. Single-backend (claude) default_provider ⇒ key dropped, model NOT prefixed. (FR-B7 "single-backend untouched")
#   3. Bare pi model + NO default_provider ⇒ stays bare (FR-B7 "no value invented") ⇒ FR-R5b error downstream.
#   4. Idempotent: re-running migrateV2ToV3 is a no-op (models already "/", key gone).
#   5. config does not import provider (decoupling invariant intact).
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on edited files.
- [ ] `go test ./...` GREEN (the §6 bump+migration breakage fully fixed).
- [ ] go.mod/go.sum byte-unchanged; config imports ONLY stdlib (no internal/provider).
- [ ] file.go/manifest.go/registry.go/render.go/decompose/* + cmd/{root,default_action,providers}.go byte-unchanged.

### Feature Validation
- [ ] `CurrentConfigVersion == 3`; `migrate.go` has `migrateV2ToV3`/`isMultiBackend`/`migrationNotice`/`v2MultiBackendBuiltins`.
- [ ] v2 pi config ⇒ global + per-role + raw-map models prefixed; default_provider deleted; ConfigVersion→3; one notice.
- [ ] Single-backend untouched; bare-no-default_provider stays bare (no-invent); idempotent; nil-safe.
- [ ] Load runs the migration before return (precedes DecodeUserOverrides); ONE notice (no double); ahead-case still advised.
- [ ] `config upgrade` --help notes the v3 fold + in-memory auto-migration.

### Code Quality Validation
- [ ] Follows conventions: PURE notice helper (mirrors configVersionNotice); white-box `package config` tests;
      comma-ok type assertions; map-copy write-back; doc comments cite FR-B7.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (LEAVE files untouched; upgrade rewrite is S2).

### Documentation
- [ ] `migrate.go` doc comments document the →v3 migration (FR-B7), the agent-no-op rationale, and the
      DecodeUserOverrides ordering constraint. `config.go` const doc describes v3.
- [ ] `config upgrade` Long text updated (Mode A). No docs/*.md edits (changeset doc sync is P4.M2.T1).

---

## Anti-Patterns to Avoid

- ❌ **Don't import internal/provider** to classify multi-backend. Use the LOCAL `v2MultiBackendBuiltins={"pi"}`
      + raw `provider_flag`. Config stays decoupled (the raw-map invariant; file_test.go enforces it). (§1)
- ❌ **Don't chase the "agent" data path.** fileConfig has no Agent field; go-toml drops `[agent.*]`. The
      in-memory agent step is a documented NO-OP. The real `[agent.*]`→`[provider.*]` rewrite is S2's on-disk
      `config upgrade`. (§3)
- ❌ **Don't invent a prefix.** Fold only when default_provider is non-empty AND the model is non-empty and
      bare. A bare model with no default_provider STAYS bare (FR-R5b error downstream). (§2)
- ❌ **Don't fold single-backend providers.** A default_provider on claude/gemini/etc. was meaningless in v2;
      drop the dead key, do NOT prefix the model. (§2)
- ❌ **Don't run the migration outside Load / after DecodeUserOverrides.** It must fold while default_provider
      is still in the raw map (inside Load, before return). DecodeUserOverrides would otherwise drop it. (§4)
- ❌ **Don't emit two notices.** The migration branch prints `migrationNotice`; `configVersionNotice` moves to
      an else-if for the AHEAD case only. (§5)
- ❌ **Don't leave `go test ./...` red.** The bump breaks ~44 hardcoded "2" assertions; fix them all (OUTPUT→3;
      keep INPUT v2 fixtures as "2"). S2 owns the upgrade rewrite-behavior tests, not the version-literal fixes. (§6)
- ❌ **Don't edit the raw-map "model" key — it's "default_model".** The manifest field is `DefaultModel
      toml:"default_model"` (manifest.go:52); the contract's "model" is generic shorthand. (§2)
- ❌ **Don't implement the on-disk `config upgrade` rewrite.** `upgradeConfigVersion`/`runConfigUpgrade` logic
      is S2. S1 touches only the `Long` (--help) text. (§0/§7)
- ❌ **Don't delete configVersionNotice's older/missing branches.** They stay as a pure tested utility; only
      Load's call site is restructured (the ahead-case else-if still uses it). (§5)

---

## Confidence Score

**9/10** — A well-bounded pure-transform migration (`migrateV2ToV3` on `*Config`) + a Load restructure, with
the algorithm pinned by FR-B7 (verbatim in the description), the multi-backend classification grounded in the
live v3 tree (only pi has a non-empty ProviderFlag — verified in builtin.go), and the two subtle traps
defused by research: (1) the "agent" step is an in-memory NO-OP (fileConfig has no Agent field — verified),
so the implementer won't chase non-existent data; (2) config must not import provider, so multi-backend is
classified locally. The `migrate.go` code is copy-ready and the Load wiring is a precise diff. The one
residual risk is the ~44-site test-breakage sweep (§6) — mechanical but voluminous; the breakage map
enumerates every file, and the INPUT-vs-OUTPUT fixture distinction is spelled out. The -1 reserves for a
missed test assertion or a Load-notice expectation that needs a second pass to land green.
