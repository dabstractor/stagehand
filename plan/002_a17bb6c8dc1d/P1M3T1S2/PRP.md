---
name: "P1.M3.T1.S2 — [role.<role>] decode structs + materialize() + overlay(): the FILE→Config plumbing that makes S1's v2 Config fields actually populate from TOML — PRD §16.2/§16.4/§16.1/§9.17/§9.14/§9.1"
description: |

  Land the SECOND subtask of Per-Role Config Schema & Model Resolution (P1.M3.T1): wire the FILE layer so
  PRD §16.2 config files carrying `config_version`, `[generation] max_commits`/`binary_extensions`, and
  `[role.<role>]` tables actually DECODE and MERGE into the `Config` fields S1 declared. S1 (SHIPPED, staged
  in config.go) declared `RoleConfig`, `CurrentConfigVersion=2`, and six new `Config` fields
  (`Roles toml:"-"`, `ConfigVersion toml:"config_version"`, `MaxCommits toml:"max_commits"`,
  `BinaryExtensions toml:"binary_extensions"`, `Commits toml:"-"`, `Single toml:"-"`) — but they are
  zero/nil because NOTHING decodes them yet. S2 IS that plumbing. It is the CONSUMER of the S1 Config struct
  and the PRODUCER of populated `Config.Roles`/`MaxCommits`/`BinaryExtensions`/`ConfigVersion` for
  `Load()` (which overlays file layers onto `Defaults()`).

  THREE FILES, all EDITS (no new files):
    1. internal/config/file.go — the file→Config plumbing.
    2. internal/config/file_test.go — decode + field-merge + partial-merge tests.
    3. internal/cmd/config.go — the Mode-A `exampleConfigTemplate` reference sections (item DOCS).

  THE file.go CHANGES (authoritative source: architecture/config_v2_delta.md §2 — matches the item contract):
    A. `fileConfig` += `ConfigVersion int `toml:"config_version"`` (top-level metadata) + `Role
       map[string]fileRoleConfig `toml:"role"`` ([role.<role>] subtables). Order: ConfigVersion, Defaults,
       Generation, Role, Provider (match delta §2).
    B. NEW `type fileRoleConfig struct { Provider string `toml:"provider"`; Model string `toml:"model"` }`
       — the FILE decode twin of `config.RoleConfig` (S1). `[role.planner]` decodes into
       `fc.Role["planner"]` EXACTLY as `[provider.pi]` decodes into `fc.Provider["pi"]` today.
    C. `fileGeneration` += `MaxCommits int `toml:"max_commits"`` + `BinaryExtensions []string
       `toml:"binary_extensions"`` (append after StripCodeFence).
    D. `materialize()` += four blocks (each NON-ZERO/non-empty, matching every existing materialize field):
       ConfigVersion (non-zero), MaxCommits (non-zero), BinaryExtensions (len>0), and Roles (convert
       map[string]fileRoleConfig → map[string]RoleConfig, copying every present role).
    E. `overlay()` += Roles FIELD-MERGE (mirror the existing `[provider.X]` field-merge — FR-R3 is the
       per-role analog of FR37a) + ConfigVersion/MaxCommits (non-zero wins) + BinaryExtensions (len>0 =
       REPLACE, not append). Nil-safe.

  THE exampleConfigTemplate CHANGES (Mode A — internal/cmd/config.go): append COMMENTED reference lines to
  the inert template — (a) `# max_commits = 12` + `# binary_extensions = []` in the existing [generation]
  section; (b) a new commented `[role.*]` section (planner/stager/message/arbiter) with an FR-R1–R4 header.
  ALL lines stay `#`-commented (the template is INERT by design). P1.M4.T2.S1 later rewrites this to a
  populated bootstrap (FR-B1); S2 only adds provisional reference text now.

  ⚠️ **THE #1 scope boundary — config.go (S1, DONE) + load.go (P1.M3.T2) are FROZEN.** S2 writes INTO the
  S1 Config fields but does NOT edit config.go. It does NOT touch load.go (env/flags/ResolveRoleModel is
  P1.M3.T2). It does NOT implement the config_version advisory (that is P1.M4.T1 — see §gotcha below). See
  research design-decisions.md §0.

  ⚠️ **THE #2 design call — Roles overlay is a FIELD-MERGE (mirror Providers), NOT a whole-block replace.**
  A repo `[role.planner] model = "X"` must NOT erase a global `[role.planner] provider = "agy"`. The
  per-field loop `existing := dst.Roles[role]; if rc.X != "" { existing.X = rc.X }; dst.Roles[role] =
  existing` is the EXACT analog of the `[provider.X]` field loop already in overlay() (FR37a). FR-R3 is the
  per-role statement of the same correction. Test it the same way (TestOverlayRolesFieldMerge mirrors
  TestOverlayProvidersFieldMerge). See design-decisions.md §5.

  ⚠️ **THE #3 design call — BinaryExtensions overlay is REPLACE (non-empty wins), NOT append.** A higher
  config layer's `binary_extensions` REPLACES the lower layer's list. The PRD §16.2 "merges with built-in
  denylist" is a RUNTIME merge in P2.M1 (internal/git/binary.go appends user exts to the BUILT-IN denylist),
  NOT a cross-layer config concatenation. See design-decisions.md §7.

  ⚠️ **THE #4 gotcha — ConfigVersion is non-zero-wins; the "missing version" advisory is NOT S2's job.**
  Defaults() pins ConfigVersion=2 and overlay skips 0, so a file OMITTING config_version leaves the resolved
  cfg.ConfigVersion==2 (==CurrentConfigVersion). Therefore the §9.17 FR-B4 advisory ("missing/older → warn")
  CANNOT be done on the resolved Config alone — it must inspect the RAW file's config_version presence at
  the loadTOML/materialize seam. That is P1.M4.T1.S1. S2 ONLY plumbs the value through (so an explicit older
  `config_version = 1` or newer `= 3` DOES propagate). Do NOT implement the warning; do NOT special-case
  "missing". See design-decisions.md §6.

  ⚠️ **THE #5 gotcha — NO new imports; go.mod UNCHANGED.** file.go already imports fmt/io/os/path/filepath/
  time/go-toml/v2; the new fields/types use only same-package types (RoleConfig, fileRoleConfig) and plain
  Go. internal/cmd/config.go already imports config; the template edit is a string change. `go mod tidy`
  MUST be a no-op.

  Deliverable: MODIFIED internal/config/file.go (fileConfig + fileRoleConfig + fileGeneration + materialize
  + overlay) + MODIFIED internal/config/file_test.go (TestLoadTOML_V2Fields + TestOverlayRolesFieldMerge +
  TestOverlay_V2Scalars) + MODIFIED internal/cmd/config.go (exampleConfigTemplate reference sections). NO
  other file touched. OUTPUT: config files with [role.*], [generation] max_commits/binary_extensions, and
  top-level config_version decode + overlay correctly across layers; `go build ./... && go test ./...` green.

---

## Goal

**Feature Goal**: Wire the FILE layer (PRD §16.2) so that TOML config files carrying the v2 keys —
top-level `config_version` (§9.17 FR-B4), `[generation] max_commits` (§9.14 FR-M4) and `binary_extensions`
(§9.1 FR3a), and `[role.<role>]` per-role provider/model tables (§16.4 FR-R1–R5) — DECODE into the
intermediate `fileConfig`/`fileGeneration` structs, MATERIALIZED into the typed `Config` fields S1 declared,
and FIELD-MERGED across config layers by `overlay()` (highest layer wins per-field, exactly like the existing
`[provider.X]` merge). After S2, `Load()`'s file layers populate `cfg.Roles`/`cfg.MaxCommits`/
`cfg.BinaryExtensions`/`cfg.ConfigVersion`; downstream consumers (P1.M3.T2 ResolveRoleModel, P2.M1 binary
filter, P3 decompose, P1.M4.T1 advisory) can read them.

**Deliverable** (all EDITS — no new files):
1. `internal/config/file.go`:
   - EXTEND `fileConfig` with `ConfigVersion int `toml:"config_version"`` (first) and `Role
     map[string]fileRoleConfig `toml:"role"`` (between Generation and Provider).
   - DEFINE `type fileRoleConfig struct { Provider string `toml:"provider"`; Model string `toml:"model"` }`.
   - EXTEND `fileGeneration` with `MaxCommits int `toml:"max_commits"`` and `BinaryExtensions []string
     `toml:"binary_extensions"`` (append after StripCodeFence).
   - EXTEND `materialize()` with four non-zero/non-empty copy blocks (ConfigVersion, MaxCommits,
     BinaryExtensions, Roles map-conversion).
   - EXTEND `overlay()` with the Roles per-role field-merge (mirror the `[provider.X]` block) + three
     scalar non-zero/non-empty blocks (ConfigVersion, MaxCommits, BinaryExtensions).
2. `internal/config/file_test.go`:
   - ADD `TestLoadTOML_V2Fields` (decode + materialize of all v2 keys, incl. a partial role).
   - ADD `TestOverlayRolesFieldMerge` (FR-R3 regression guard — mirrors TestOverlayProvidersFieldMerge).
   - ADD `TestOverlay_V2Scalars` (ConfigVersion/MaxCommits/BinaryExtensions non-zero-wins + partial-merge
     preservation — mirrors TestOverlayPartial).
3. `internal/cmd/config.go`:
   - EXTEND `exampleConfigTemplate` with commented `# max_commits` / `# binary_extensions` in the
     [generation] section and a new commented `[role.*]` reference section (all `#`-commented, inert).

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; a TOML file with
`config_version`, `[generation] max_commits`/`binary_extensions`, and `[role.planner]`/`[role.stager]`
tables decodes and materializes into the correct `Config` fields; `overlay()` field-merges Roles
(a higher layer's `[role.planner].model` does NOT erase a lower layer's `[role.planner].provider`) and
non-zero-wins the three scalars; the config init template shows the new sections commented-out; go.mod/
go.sum byte-unchanged; config.go (S1) + load.go + git.go and every other file byte-unchanged.

## User Persona

**Target User**: The downstream v2 consumers that read these populated fields:
- **P1.M3.T2 (load.go)** — `ResolveRoleModel(role, cfg)` reads `cfg.Roles[role]` (per-role config layer)
  then falls back to `cfg.Provider`/`cfg.Model` (global). S2 makes the FILE populate `cfg.Roles`.
- **P2.M1.T1 (internal/git/binary.go)** — merges `cfg.BinaryExtensions` with the built-in denylist (FR3a).
  S2 makes the FILE populate `cfg.BinaryExtensions`.
- **P3.M2/M4 (decompose)** — reads `cfg.MaxCommits` (FR-M4 safety cap). S2 makes the FILE populate it.
- **P1.M4.T1.S1 (config_version advisory)** — compares the FILE's `config_version` to `CurrentConfigVersion`
  at the loadTOML seam. S2 makes the FILE decode `config_version` into `fileConfig` (the seam P1.M4.T1 reads).
End-user persona is "the plan-holder"/"the multi-agent tinkerer" (PRD §7) who, once the full v2 lands, gets
per-role model routing + multi-commit decomposition + binary filtering — all gated on these file keys
decoding correctly.

**Use Case**: A user writes `[role.planner] provider = "agy"; model = "gemini-2.5-pro"` in their global
config and `[role.planner] model = "gemini-3.5-pro"` in a repo-local `.stagecoach.toml`. After S2,
`Load()` decodes both, field-merges them (planner.provider="agy" from global survives; planner.model=
"gemini-3.5-pro" from repo wins), and the resolved `cfg.Roles["planner"]` is `{agy, gemini-3.5-pro}` —
exactly the FR-R3 field-merge guarantee, and the same correctness property FR37a already enforces for
`[provider.X]`.

**User Journey**: (internal) `config file` → `loadTOML` → `toml.Unmarshal` into `fileConfig` (now incl.
`ConfigVersion`/`Role`/`Generation.MaxCommits`/`Generation.BinaryExtensions`) → `materialize` → partial
`*Config` → `overlay(&cfg, layer)` per layer (Roles field-merged; scalars non-zero-wins) → resolved `cfg`
read by P1.M3.T2/P2/P3/P1.M4.T1.

**Pain Points Addressed**: Without S2, the v2 file keys silently decode into NOTHING (S1's fields stay
zero/nil) — a user's `[role.planner]` table is ignored, `max_commits`/`binary_extensions` are dropped, and
`config_version` is invisible. Worse, a naive "whole-block replace" Roles overlay would erase cross-layer
per-role pins (the same class of bug FR37a fixed for providers). S2 makes the keys work AND field-merges
Roles correctly.

## Why

- **Activates the v2 Config schema S1 declared.** S1 added the fields; S2 makes them populate from files.
  Without S2, every v2 file key is a no-op. S2 is the bridge between the typed schema (S1) and the
  resolver/consumers (P1.M3.T2/P2/P3/P1.M4.T1).
- **Satisfies PRD §16.4 (FR-R3), §16.2, §9.17 (FR-B4), §9.14 (FR-M4), §9.1 (FR3a).** Roles field-merge =
  FR-R3 (the per-role statement of FR37a); config_version decode = the seam FR-B4's advisory reads;
  max_commits/binary_extensions = the §16.2 [generation] keys.
- **Faithful to the existing file.go design.** The `[role.<role>]` decode mirrors `[provider.<name>]`
  (nested map); the Roles field-merge mirrors the Providers field-merge; the scalar non-zero-wins mirrors
  every existing overlay field. S2 adds NO new pattern — it extends the existing ones.
- **Zero behavior change for v1 files.** A v1 config file (no v2 keys) decodes identically (the new
  fileConfig fields are zero/nil → materialize/overlay skip them → resolved Config is unchanged). Back-
  compatible by construction.

## What

Modified `internal/config/file.go` (extended decode structs + materialize + overlay), modified
`internal/config/file_test.go` (three new tests), and modified `internal/cmd/config.go` (extended
`exampleConfigTemplate`). No new files, no new imports, no dependency change, no resolver/consumer change.

### Success Criteria

- [ ] `fileConfig` has `ConfigVersion int `toml:"config_version"`` (first field) and `Role
      map[string]fileRoleConfig `toml:"role"`` (between Generation and Provider), matching delta §2.
- [ ] `fileRoleConfig` is defined: `type fileRoleConfig struct { Provider string `toml:"provider"`;
      Model string `toml:"model"` }`.
- [ ] `fileGeneration` has `MaxCommits int `toml:"max_commits"`` and `BinaryExtensions []string
      `toml:"binary_extensions"`` (appended after StripCodeFence).
- [ ] `materialize()` copies (non-zero/non-empty): `fc.ConfigVersion→c.ConfigVersion`,
      `g.MaxCommits→c.MaxCommits`, `g.BinaryExtensions→c.BinaryExtensions`, and converts `fc.Role→c.Roles`
      (map[string]fileRoleConfig → map[string]RoleConfig, every present role).
- [ ] `overlay()` FIELD-MERGES Roles (per-role: a higher layer's non-empty Provider/Model overrides that
      one field only; lower-layer fields survive; new roles added; untouched roles preserved — mirror of the
      `[provider.X]` block) AND non-zero/non-empty-wins ConfigVersion, MaxCommits, BinaryExtensions.
- [ ] `exampleConfigTemplate` shows commented `# max_commits` + `# binary_extensions` in [generation] and a
      commented `[role.*]` section (planner/stager/message/arbiter); every added line is `#`-commented.
- [ ] `TestLoadTOML_V2Fields` decodes + materializes all v2 keys (incl. a partial role with Provider="").
- [ ] `TestOverlayRolesFieldMerge` proves the field-merge (lower-layer provider survives a higher-layer
      model-only override) — mirrors TestOverlayProvidersFieldMerge.
- [ ] `TestOverlay_V2Scalars` proves non-zero-wins + partial-merge preservation for the three scalars.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/` clean.
- [ ] go.mod/go.sum byte-unchanged; config.go (S1) + load.go + git.go + every non-{file.go, file_test.go,
      internal/cmd/config.go} file byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact decode-struct
additions + materialize + overlay blocks (quoted verbatim in the Blueprint), the authoritative delta §2
(the source of truth — quoted in research design-decisions.md §1–§7), the S1 Config fields (the targets
S2 writes into — enumerated), the existing file.go materialize/overlay patterns (the templates to extend),
the existing file_test.go idioms (the tests to mirror), the exampleConfigTemplate Mode-A additions, and the
LEAVE list (config.go/load.go frozen). No decompose/git/prompt knowledge required — S2 is decode structs +
field-copy + field-merge + template text.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE file.go spec (matches the item contract verbatim)
- docfile: plan/002_a17bb6c8dc1d/architecture/config_v2_delta.md
  section: "§2. File Decode Changes (file.go)" — the EXACT new fileConfig (ConfigVersion + Role),
           fileRoleConfig, fileGeneration (+MaxCommits, +BinaryExtensions), materialize() updates, and
           overlay() updates (Roles field-merge + the 3 scalar blocks). This is the spec S2 implements.
  critical: §2 is the source of truth for struct fields/tags and the overlay field-merge code. §1 is S1
       (DONE — do not re-implement). §3/§4/§5/§6 are load.go/bootstrap/advisory (P1.M3.T2 / P1.M4 — OUT OF
       SCOPE for S2). Read §2 only for the file.go work.

# MUST READ — the design calls (the non-obvious decisions)
- docfile: plan/002_a17bb6c8dc1d/P1M3T1S2/research/design-decisions.md
  why: the 10 non-obvious calls — scope (file.go+file_test.go+cmd/config.go ONLY; §0), fileConfig ordering
       matching delta §2 (§1), fileRoleConfig as RoleConfig's decode twin (§2), fileGeneration appends
       (§3), materialize's four non-zero blocks + copy-all-roles (§4), overlay Roles FIELD-MERGE mirroring
       Providers (§5), ConfigVersion non-zero-wins + why the advisory is NOT S2 (§6), BinaryExtensions
       REPLACE not append (§7), the exampleConfigTemplate Mode-A additions (§8), no new imports (§9), the
       test strategy (§10).
  critical: §5 (Roles field-merge is the load-bearing correctness property — mirror Providers exactly),
       §6 (ConfigVersion subtlety — do NOT implement the advisory), §7 (BinaryExtensions is REPLACE, the
       denylist merge is P2.M1), §0 (config.go/load.go frozen) are the things most likely to go wrong.

# MUST READ — the S1 CONTRACT (the Config fields S2 writes into; S1 is SHIPPED, staged)
- file: internal/config/config.go   (S1 — read for the field/type/const targets; do NOT edit)
  section: `RoleConfig` (struct {Provider,Model}), `const CurrentConfigVersion = 2`, and the Config fields
           `Roles map[string]RoleConfig toml:"-"`, `ConfigVersion int toml:"config_version"`,
           `MaxCommits int toml:"max_commits"`, `BinaryExtensions []string toml:"binary_extensions"`,
           `Commits int toml:"-"`, `Single bool toml:"-"`; `Defaults()` sets ConfigVersion=2, MaxCommits=12.
  why: S2 WRITES INTO these fields. Confirms `RoleConfig` is the conversion target for materialize; confirms
       `ConfigVersion`/`MaxCommits`/`BinaryExtensions` are the scalar materialize targets; confirms `Commits`/
       `Single` are `toml:"-"` runtime-only (NOT decoded from file — do NOT add them to fileConfig).
  critical: do NOT edit config.go. `Commits`/`Single` are runtime-only (toml:"-") — they are NEVER file keys,
       so do NOT add them to fileConfig/fileGeneration. Only ConfigVersion/Roles/MaxCommits/BinaryExtensions
       are file-decoded.

# THE FILE BEING MODIFIED — READ FULLY before editing
- file: internal/config/file.go
  section: fileConfig/fileDefaults/fileGeneration (decode structs), materialize() (non-zero copy into fresh
           *Config), overlay() (non-zero field-merge incl. the `[provider.X]` per-provider field-merge loop
           — the EXACT pattern to mirror for Roles).
  why: the EXACT current state S2 edits. Note `materialize` starts `c := &Config{Timeout: timeout}` and every
       field is a `if x != 0/"" { c.X = x }` copy. Note `overlay`'s `[provider.X]` block: `for name, entry :=
       range src.Providers { ... for k,v := range entry { dst.Providers[name][k] = v } }` — the Roles merge is
       the typed (Provider/Model) analog. Note `c.Providers = fc.Provider` (nil-safe whole-map copy) — the
       Roles materialize is the typed analog.
  critical: mirror the existing non-zero/non-empty semantics EXACTLY. The Roles overlay field-merge must
       preserve lower-layer per-role fields (do NOT do `dst.Roles[role] = rc` — that's a whole-entry replace
       that would drop the lower-layer provider; use the existing := / per-field pattern).

# THE TEST FILE BEING MODIFIED — mirror its idioms
- file: internal/config/file_test.go
  section: writeTempTOML (helper), TestLoadTOMLValid (decode+materialize assertions), TestOverlayPartial
           (partial-merge: src sets 2 fields, everything else untouched), TestOverlayProvidersFieldMerge
           (the FR37a field-merge regression test — THE template for TestOverlayRolesFieldMerge),
           TestOverlayNilSrc (nil-safety).
  why: the test STYLE — white-box `package config`, grouped doc-comments, one t.Errorf per assertion, the
       field-merge "lower-layer field SURVIVES a higher-layer partial" assertion pattern.
  critical: TestOverlayProvidersFieldMerge is the EXACT model for the Roles test (swap Providers→Roles,
       map[string]map[string]any→map[string]RoleConfig, the field-merge logic is identical in shape).

# THE DOCS FILE BEING MODIFIED — the Mode-A template
- file: internal/cmd/config.go
  section: `const exampleConfigTemplate` (the inert commented config written by `config init`). The [defaults]
           / [generation] / [provider.<name>] sections; the `#`-commented-everywhere convention.
  why: the template S2 extends with [generation] max_commits/binary_extensions lines + a [role.*] section.
       Confirms every option line is `#`-commented (INERT) and the style (a header comment block per section).
  critical: keep EVERY added line `#`-commented. config_test.go:136 asserts the written file EQUALS the
       constant (editing the constant is safe — it compares to itself). Do NOT add uncommented keys. P1.M4.T2
       later rewrites this template to populated — S2's additions are provisional reference text.

# THE LOAD FLOW (read-only — confirms overlay is onto Defaults())
- file: internal/config/load.go   (P1.M3.T2's file — do NOT edit)
  section: `Load()` — `cfg := Defaults()` (Layer 1, ConfigVersion=2/MaxCommits=12) then `overlay(&cfg, g)`
           per file layer.
  why: confirms the overlay is onto a Defaults() baseline, so non-zero-wins overlay is correct (a file
       omitting a key leaves Defaults' value). Confirms S2 does NOT touch Load/loadEnv/loadFlags.
  critical: do NOT edit load.go. The ConfigVersion advisory must be added at the loadTOML seam by P1.M4.T1,
       NOT here — see design-decisions.md §6.

# The PRD basis (already in your context as selected_prd_content)
- file: PRD.md (or plan/002_a17bb6c8dc1d/prd_snapshot.md)
  section: "16.4 Per-role provider/model configuration" (h3.68) — Roles semantics (4 roles, field-merge, ""
           ⇒ inherit global, FR-R1–R5).
  section: "16.2 Full config file example" (h3.66) — confirms the snake_case file keys config_version (top-
           level), max_commits/binary_extensions ([generation]).
  section: "16.1 Resolution order" (h3.65) — "config_version is metadata, NOT a precedence layer."
  section: "9.17 FR-B4" (h3.33) — config_version advisory semantics.
  section: "9.14 FR-M4" (h3.30) — max_commits (safety cap, default 12). "9.1 FR3a" (h3.17) — binary_extensions.
  critical: §16.1 — config_version is metadata (non-zero-wins overlay is correct; the advisory is separate).
       §16.4 — per-role field-merge (FR-R3) is the analog of the provider field-merge (FR37a).
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go        # S1 SHIPPED (staged) — RoleConfig + CurrentConfigVersion + 6 Config fields. UNCHANGED by S2.
  config_test.go   # S1 SHIPPED. UNCHANGED by S2.
  file.go          # fileConfig/fileGeneration/materialize/overlay — EDIT (S2's core)
  file_test.go     # decode + overlay tests — EDIT (S2 adds 3 tests)
  load.go          # Load/loadEnv/loadFlags — UNCHANGED (P1.M3.T2's scope)
  git.go           # git-config reader — UNCHANGED
internal/cmd/
  config.go        # configCmd + exampleConfigTemplate — EDIT (Mode-A template)
  config_test.go   # config init/path tests — UNCHANGED (the equality test stays green)
go.mod / go.sum    # UNCHANGED (no new import)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. All edits are in-place modifications to internal/config/file.go + file_test.go + internal/cmd/config.go.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — scope: file.go + file_test.go + internal/cmd/config.go ONLY): do NOT touch config.go (S1
//   SHIPPED the fields — S2 only WRITES INTO them), load.go (P1.M3.T2 owns env/flags/ResolveRoleModel), or
//   git.go. Commits/Single are toml:"-" runtime-only (set by flags in P1.M3.T2/P4) — they are NEVER file
//   keys, so do NOT add them to fileConfig/fileGeneration. Only ConfigVersion/Roles/MaxCommits/
//   BinaryExtensions are file-decoded. (design-decisions §0)

// CRITICAL (#2 — Roles overlay is a FIELD-MERGE, mirror Providers): do NOT write `dst.Roles[role] = rc`
//   (whole-entry replace — drops the lower-layer provider). Use the per-field pattern:
//     existing := dst.Roles[role]
//     if rc.Provider != "" { existing.Provider = rc.Provider }
//     if rc.Model != ""    { existing.Model = rc.Model }
//     dst.Roles[role] = existing
//   This is the typed analog of the [provider.X] field loop. FR-R3 is the per-role statement of FR37a.
//   (design-decisions §5)

// CRITICAL (#3 — BinaryExtensions overlay is REPLACE, not append): `if len(src.BinaryExtensions) > 0 {
//   dst.BinaryExtensions = src.BinaryExtensions }` — a higher layer REPLACES the lower layer's list. The
//   PRD §16.2 "merges with built-in denylist" is a RUNTIME merge in P2.M1 (internal/git/binary.go), NOT a
//   cross-layer config concatenation. Do NOT append. (design-decisions §7)

// CRITICAL (#4 — ConfigVersion: non-zero-wins; advisory is NOT S2): materialize `if fc.ConfigVersion != 0`
//   and overlay `if src.ConfigVersion != 0`. Defaults() pins ConfigVersion=2 and overlay skips 0, so a file
//   omitting config_version leaves resolved cfg.ConfigVersion==2. The §9.17 FR-B4 "missing/older → warn"
//   advisory CANNOT be done on the resolved Config — it inspects the raw file at the loadTOML seam
//   (P1.M4.T1.S1). S2 ONLY plumbs the value so an explicit version propagates. Do NOT implement the warning.
//   (design-decisions §6)

// GOTCHA (materialize non-zero uniformity): every existing materialize field is a `if x != 0/""` copy.
//   Match that for ConfigVersion/MaxCommits (non-zero) and BinaryExtensions (len>0). Do NOT special-case
//   ConfigVersion (unconditional copy is functionally identical because overlay skips 0 — keep it uniform).

// GOTCHA (Roles materialize copies ALL present roles): `if len(fc.Role) > 0 { c.Roles = make(...); for role,
//   frc := range fc.Role { c.Roles[role] = RoleConfig{Provider: frc.Provider, Model: frc.Model} } }`. Do NOT
//   filter all-empty roles (an empty [role.X] ⇒ RoleConfig{"",""} = "inherit global", harmless; mirrors how
//   Providers copies the whole map).

// GOTCHA (fileConfig ordering matches delta §2): place ConfigVersion FIRST and Role between Generation and
//   Provider. go-toml matches by tag (order is functional-neutral), but matching the authoritative delta §2
//   makes the struct reviewable. Existing Defaults/Generation/Provider shift down by the insertions.

// GOTCHA (exampleConfigTemplate: keep ALL lines #): the template is INERT by design. Every added line (the
//   [generation] max_commits/binary_extensions lines and the whole [role.*] section) MUST be `#`-commented.
//   config_test.go:136 compares the written file to the CONSTANT (editing it is safe), but a future
//   P1.M4.T2 test may assert "no uncommented key" — don't break that invariant.

// GOTCHA (no new imports): file.go already imports fmt/io/os/path/filepath/time/go-toml/v2; the new fields
//   use same-package types (RoleConfig, fileRoleConfig) + plain Go. internal/cmd/config.go already imports
//   config; the template edit is a string change. `go mod tidy` MUST be a no-op.

// GOTCHA (v1 files are back-compatible): a v1 config (no v2 keys) → the new fileConfig fields are zero/nil →
//   materialize/overlay skip them → resolved Config unchanged. No special v1 handling needed.
```

## Implementation Blueprint

### Data models and structure

```go
// === internal/config/file.go — the modified decode structs (existing fields unchanged; new fields marked V2) ===

// fileConfig is the §16.2 file decode target: NESTED (matches [defaults]/[generation]/[role.X]/
// [provider.X]), with Timeout as a STRING ("120s") because go-toml/v2 cannot decode "120s" into
// time.Duration and the resolved Config is flat/plain (S1). loadTOML materializes this into a *Config.
// UNEXPORTED: only file.go decodes into these.
//
// V2 (P1.M3.T1.S2): ConfigVersion (top-level metadata, §9.17 FR-B4) + Role ([role.<role>] per-role
// provider/model tables, §16.4 FR-R1–R5). [role.<role>] decodes into the Role map EXACTLY as [provider.X]
// decodes into the Provider map.
type fileConfig struct {
	ConfigVersion int                       `toml:"config_version"` // V2 — top-level metadata key (§9.17 FR-B4)
	Defaults      fileDefaults              `toml:"defaults"`
	Generation    fileGeneration            `toml:"generation"`
	Role          map[string]fileRoleConfig `toml:"role"`  // V2 — [role.<role>] per-role tables (§16.4)
	Provider      map[string]map[string]any `toml:"provider"` // nil if the file has no [provider] table
}

// fileRoleConfig is the FILE decode twin of config.RoleConfig (§16.4). A [role.planner] table decodes into
// fc.Role["planner"] EXACTLY as a [provider.pi] table decodes into fc.Provider["pi"]. materialize converts
// each to a typed RoleConfig. Both fields "" ⇒ the role inherits the global [defaults] (FR-R2).
type fileRoleConfig struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
}

// fileGeneration is unchanged above; the two V2 fields are APPENDED after StripCodeFence.
type fileGeneration struct {
	MaxDiffBytes        int    `toml:"max_diff_bytes"`
	MaxMdLines          int    `toml:"max_md_lines"`
	MaxDuplicateRetries int    `toml:"max_duplicate_retries"`
	SubjectTargetChars  int    `toml:"subject_target_chars"`
	Output              string `toml:"output"`
	StripCodeFence      *bool  `toml:"strip_code_fence"`
	MaxCommits       int      `toml:"max_commits"`       // V2 — safety cap on auto-decompose (§9.14 FR-M4)
	BinaryExtensions []string `toml:"binary_extensions"` // V2 — extra non-text exts to filter (§9.1 FR3a)
}
```

```go
// === internal/config/file.go — materialize() additions (append before the final `c.Providers = fc.Provider`) ===

	// V2 [generation] scalars — non-zero/non-empty copy (matches every existing materialize field).
	if g.MaxCommits != 0 {
		c.MaxCommits = g.MaxCommits
	}
	if len(g.BinaryExtensions) > 0 {
		c.BinaryExtensions = g.BinaryExtensions
	}
	// V2 top-level metadata — non-zero copy (see overlay: Defaults() pins 2; a file's explicit version
	// propagates; an omitted one leaves Defaults' value — the §9.17 advisory is P1.M4.T1's job, not here).
	if fc.ConfigVersion != 0 {
		c.ConfigVersion = fc.ConfigVersion
	}
	// V2 per-role table — convert map[string]fileRoleConfig → map[string]RoleConfig, copying every present
	// role (an all-empty [role.X] ⇒ "inherit global", harmless — mirrors Providers' whole-map copy).
	if len(fc.Role) > 0 {
		c.Roles = make(map[string]RoleConfig, len(fc.Role))
		for role, frc := range fc.Role {
			c.Roles[role] = RoleConfig{Provider: frc.Provider, Model: frc.Model}
		}
	}
	c.Providers = fc.Provider // nil-safe: nil if no [provider] table  (existing line — UNCHANGED)
```

```go
// === internal/config/file.go — overlay() additions (append AFTER the existing [provider.X] field-merge block) ===

	// V2 [role.<role>] — per-role FIELD-MERGE across config layers (PRD §16.4 FR-R3). A field a higher layer
	// sets overrides that one field only; fields the higher layer omits survive from lower layers. This is
	// the typed (Provider/Model) analog of the [provider.X] field-merge above (PRD §9.8 FR37a): a repo
	// [role.planner] model="X" must NOT erase a global [role.planner] provider="agy". Nil-safe.
	if len(src.Roles) > 0 {
		if dst.Roles == nil {
			dst.Roles = make(map[string]RoleConfig, len(src.Roles))
		}
		for role, rc := range src.Roles {
			existing := dst.Roles[role] // zero value if absent — fine (inherit-global sentinel)
			if rc.Provider != "" {
				existing.Provider = rc.Provider
			}
			if rc.Model != "" {
				existing.Model = rc.Model
			}
			dst.Roles[role] = existing
		}
	}
	// V2 scalars — non-zero/non-empty wins (matches every existing overlay field).
	if src.ConfigVersion != 0 {
		dst.ConfigVersion = src.ConfigVersion
	}
	if src.MaxCommits != 0 {
		dst.MaxCommits = src.MaxCommits
	}
	if len(src.BinaryExtensions) > 0 {
		dst.BinaryExtensions = src.BinaryExtensions // REPLACE, not append (runtime denylist merge is P2.M1)
	}
```

```go
// === internal/config/file_test.go — ADDITIONS (package config white-box; imports unchanged) ===

// TestLoadTOML_V2Fields proves the v2 file keys decode + materialize: config_version, [generation]
// max_commits/binary_extensions, and [role.<role>] tables (incl. a PARTIAL role whose Provider is "" — the
// field-level decode, not a whole-block). Mirrors TestLoadTOMLValid.
func TestLoadTOML_V2Fields(t *testing.T) {
	body := `
config_version = 2

[generation]
max_commits = 5
binary_extensions = ["foo", "bar"]

[role.planner]
provider = "agy"
model = "gemini-2.5-pro"

[role.stager]
model = "gemini-2.5-flash"
`
	path := writeTempTOML(t, body)
	cfg, err := loadTOML(path)
	if err != nil || cfg == nil {
		t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
	}
	if cfg.ConfigVersion != 2 {
		t.Errorf("ConfigVersion=%d want 2", cfg.ConfigVersion)
	}
	if cfg.MaxCommits != 5 {
		t.Errorf("MaxCommits=%d want 5", cfg.MaxCommits)
	}
	if len(cfg.BinaryExtensions) != 2 || cfg.BinaryExtensions[0] != "foo" || cfg.BinaryExtensions[1] != "bar" {
		t.Errorf("BinaryExtensions=%v want [foo bar]", cfg.BinaryExtensions)
	}
	if len(cfg.Roles) != 2 {
		t.Fatalf("Roles len=%d want 2", len(cfg.Roles))
	}
	if rc := cfg.Roles["planner"]; rc.Provider != "agy" || rc.Model != "gemini-2.5-pro" {
		t.Errorf("Roles[planner]=%+v want {agy gemini-2.5-pro}", rc)
	}
	// Partial role: only model set → Provider decodes "" (field-level, not whole-block).
	if rc := cfg.Roles["stager"]; rc.Provider != "" || rc.Model != "gemini-2.5-flash" {
		t.Errorf("Roles[stager]=%+v want {\"\" gemini-2.5-flash}", rc)
	}
}

// TestOverlayRolesFieldMerge is the FR-R3 regression guard — MIRRORS TestOverlayProvidersFieldMerge. A
// higher layer setting only [role.planner].model must NOT erase a lower layer's [role.planner].provider
// (the per-role analog of the FR37a provider field-merge). Plus: a src-only role is added; an untouched
// dst role survives.
func TestOverlayRolesFieldMerge(t *testing.T) {
	dst := &Config{
		Roles: map[string]RoleConfig{
			"planner": {Provider: "agy", Model: "gemini-2.5-pro"},
			"message": {Provider: "pi", Model: "gpt-5.4-nano"},
		},
	}
	src := &Config{
		Roles: map[string]RoleConfig{
			"planner": {Model: "gemini-3.5-pro"}, // higher layer sets MODEL only
			"arbiter": {Provider: "codex", Model: "gpt-5.1-codex-mini"}, // new role
		},
	}
	overlay(dst, src)

	// planner.provider SURVIVES (lower-layer field not clobbered by a higher-layer partial):
	if rc := dst.Roles["planner"]; rc.Provider != "agy" {
		t.Errorf("planner.provider=%q want agy (field-merge must preserve lower-layer provider)", rc.Provider)
	}
	// planner.model OVERRIDDEN by the higher layer:
	if rc := dst.Roles["planner"]; rc.Model != "gemini-3.5-pro" {
		t.Errorf("planner.model=%q want gemini-3.5-pro (higher layer wins)", rc.Model)
	}
	// new role added:
	if rc, ok := dst.Roles["arbiter"]; !ok {
		t.Errorf("arbiter role missing (src-only role must be added)")
	} else if rc.Provider != "codex" || rc.Model != "gpt-5.1-codex-mini" {
		t.Errorf("arbiter=%+v want {codex gpt-5.1-codex-mini}", rc)
	}
	// untouched dst role survives:
	if rc := dst.Roles["message"]; rc.Provider != "pi" || rc.Model != "gpt-5.4-nano" {
		t.Errorf("message=%+v want {pi gpt-5.4-nano} (untouched role must survive)", rc)
	}
}

// TestOverlay_V2Scalars proves non-zero-wins + partial-merge preservation for ConfigVersion/MaxCommits/
// BinaryExtensions — mirrors TestOverlayPartial. (a) src sets all three → dst overridden; (b) src omits
// them (zero/nil) → Defaults() baseline PRESERVED.
func TestOverlay_V2Scalars(t *testing.T) {
	// (a) src sets all three → overridden.
	dst := Defaults() // ConfigVersion=2, MaxCommits=12, BinaryExtensions=nil
	src := &Config{ConfigVersion: 3, MaxCommits: 7, BinaryExtensions: []string{"x", "y"}}
	overlay(&dst, src)
	if dst.ConfigVersion != 3 {
		t.Errorf("ConfigVersion=%d want 3 (src non-zero wins)", dst.ConfigVersion)
	}
	if dst.MaxCommits != 7 {
		t.Errorf("MaxCommits=%d want 7 (src non-zero wins)", dst.MaxCommits)
	}
	if len(dst.BinaryExtensions) != 2 {
		t.Errorf("BinaryExtensions=%v want [x y] (src non-empty wins = REPLACE)", dst.BinaryExtensions)
	}

	// (b) src OMITS them (zero/nil) → Defaults() baseline preserved (partial merge, no clobber).
	dst = Defaults()
	src = &Config{Provider: "pi"} // none of the v2 scalars set
	overlay(&dst, src)
	if dst.ConfigVersion != CurrentConfigVersion {
		t.Errorf("ConfigVersion=%d want CurrentConfigVersion (nil src must not clobber)", dst.ConfigVersion)
	}
	if dst.MaxCommits != 12 {
		t.Errorf("MaxCommits=%d want 12 (zero src must not clobber)", dst.MaxCommits)
	}
	if dst.BinaryExtensions != nil {
		t.Errorf("BinaryExtensions=%v want nil (empty src must not clobber)", dst.BinaryExtensions)
	}
}
```

```go
// === internal/cmd/config.go — exampleConfigTemplate additions (Mode A). Insert the two [generation] lines
//     after the existing "# strip_code_fence = true ..." / NOTE line; append the [role.*] section at the
//     end of the template string. EVERY added line is # -commented (template stays INERT). ===

// (1) In the [generation] section, AFTER the existing lines:
//     # strip_code_fence      = true    # strip ` fences from agent output (all providers)
//     # NOTE: [generation] output/strip_code_fence override any per-provider [provider.<name>] values.
//   ADD:
# max_commits           = 12      # safety cap on auto-decompose (PRD §9.14 FR-M4); default 12
# binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)

// (2) At the END of the template (after the [provider.<name>] section), ADD:
# ---------------------------------------------------------------------------
# [role.<role>] — per-role provider/model overrides (PRD §16.4, §9.15 FR-R1–R5)
# ---------------------------------------------------------------------------
# The four agent roles — planner, stager, message, arbiter — each resolve their provider/model
# independently. A single [defaults] (above) covers ALL roles; a [role.*] table overrides it for the
# roles you care about. Both fields "" -> inherit [defaults]. Precedence (highest wins):
#   flag > STAGECOACH_<ROLE>_* env > [role.*] config > [defaults] > provider manifest default.
#
# [role.planner]
# provider = "agy"
# model    = "gemini-2.5-pro"
#
# [role.stager]            # tooled agent that runs git; needs tooled_flags in its provider manifest
# provider = "agy"
# model    = "gemini-2.5-flash"
#
# [role.message]           # bare commit-message agent — inherits [defaults] (omit to inherit)
# provider = ""            # "" -> inherit [defaults].provider
# model    = ""            # "" -> inherit [defaults].model
#
# [role.arbiter]           # bare leftover arbiter — inherits [defaults]
# provider = ""
# model    = ""
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/file.go — decode structs (fileConfig + fileRoleConfig + fileGeneration)
  - ADD `ConfigVersion int `toml:"config_version"`` as the FIRST field of fileConfig.
  - ADD `Role map[string]fileRoleConfig `toml:"role"`` to fileConfig BETWEEN Generation and Provider.
  - DEFINE `type fileRoleConfig struct { Provider string `toml:"provider"`; Model string `toml:"model"` }`
      (place it right after fileGeneration, before fileConfig, or alongside — keep decode structs grouped).
  - APPEND `MaxCommits int `toml:"max_commits"`` + `BinaryExtensions []string `toml:"binary_extensions"``
      to fileGeneration (after StripCodeFence).
  - GOTCHA: do NOT add Commits/Single to any decode struct (they are toml:"-" runtime-only).
  - GOTCHA: ordering matches delta §2 (ConfigVersion first; Role between Generation and Provider).

Task 2: EDIT internal/config/file.go — materialize() (four copy blocks)
  - ADD (before the final `c.Providers = fc.Provider`): MaxCommits (non-zero), BinaryExtensions (len>0),
      ConfigVersion (non-zero), Roles (convert map, copy every present role).
  - GOTCHA: match the existing non-zero/non-empty per-field pattern. Do NOT special-case ConfigVersion.
  - GOTCHA: copy ALL present roles (no empty-role filtering).

Task 3: EDIT internal/config/file.go — overlay() (Roles field-merge + 3 scalars)
  - ADD (AFTER the existing [provider.X] field-merge block): the Roles per-role field-merge (existing :=
      dst.Roles[role]; per-field non-zero override; dst.Roles[role] = existing), then ConfigVersion/MaxCommits
      (non-zero), then BinaryExtensions (len>0 = REPLACE).
  - GOTCHA: Roles is a FIELD-MERGE — never `dst.Roles[role] = rc` (whole-entry replace drops lower-layer
      fields). Mirror the [provider.X] loop shape exactly.
  - GOTCHA: BinaryExtensions is REPLACE, not append.

Task 4: EDIT internal/config/file_test.go — three tests
  - ADD TestLoadTOML_V2Fields (decode+materialize of all v2 keys; partial role with Provider="").
  - ADD TestOverlayRolesFieldMerge (FR-R3 guard — mirror TestOverlayProvidersFieldMerge).
  - ADD TestOverlay_V2Scalars (non-zero-wins + partial-merge preservation).
  - GOTCHA: white-box `package config`; use the existing writeTempTOML helper; one t.Errorf per assertion.

Task 5: EDIT internal/cmd/config.go — exampleConfigTemplate (Mode A)
  - ADD the two commented [generation] lines (max_commits, binary_extensions) after the existing
      strip_code_fence/NOTE lines.
  - APPEND the commented [role.*] section (planner/stager/message/arbiter + FR-R1–R4 header) at the end.
  - GOTCHA: keep EVERY added line # -commented (template stays INERT).

Task 6: VERIFY (no further edits)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. config.go (S1) + load.go +
      git.go + every non-target file byte-unchanged. `go build ./... && go test ./...` green.
```

### Implementation Patterns & Key Details

```go
// THE Roles overlay FIELD-MERGE (the load-bearing correctness property — FR-R3 / mirror of FR37a):
if len(src.Roles) > 0 {
	if dst.Roles == nil {
		dst.Roles = make(map[string]RoleConfig, len(src.Roles))
	}
	for role, rc := range src.Roles {
		existing := dst.Roles[role]        // zero value if absent (inherit-global sentinel — fine)
		if rc.Provider != "" {
			existing.Provider = rc.Provider // per-field override; lower-layer fields SURVIVE
		}
		if rc.Model != "" {
			existing.Model = rc.Model
		}
		dst.Roles[role] = existing
	}
}

// THE Roles materialize (typed conversion of the decode twin):
if len(fc.Role) > 0 {
	c.Roles = make(map[string]RoleConfig, len(fc.Role))
	for role, frc := range fc.Role {
		c.Roles[role] = RoleConfig{Provider: frc.Provider, Model: frc.Model}
	}
}

// THE scalar overlay (non-zero/non-empty wins — BinaryExtensions is REPLACE):
if src.ConfigVersion != 0 { dst.ConfigVersion = src.ConfigVersion }
if src.MaxCommits != 0    { dst.MaxCommits = src.MaxCommits }
if len(src.BinaryExtensions) > 0 { dst.BinaryExtensions = src.BinaryExtensions }
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): change NONE. file.go uses same-package types + plain Go; internal/cmd/config.go
      template edit is a string change. `go mod tidy` MUST be a no-op. `git diff --exit-code go.mod go.sum`
      MUST be empty.

PACKAGE EDGES: NONE added. config stays a leaf (RoleConfig is plain, not Manifest). cmd already imports config.

DOWNSTREAM (NOT this task — S2 only POPULATES; these CONSUME):
  - P1.M3.T2 (load.go): ResolveRoleModel(role, cfg) reads cfg.Roles[role] then falls back to cfg.Provider/
        cfg.Model. Also adds STAGECOACH_<ROLE>_* env + --<role>-* flags (which set cfg.Roles directly).
  - P2.M1.T1 (internal/git/binary.go): merges cfg.BinaryExtensions with the built-in denylist (FR3a runtime).
  - P3.M2/M4 (decompose): reads cfg.MaxCommits (FR-M4 cap).
  - P1.M4.T1.S1 (config_version advisory): reads fileConfig.ConfigVersion at the loadTOML seam to detect
        missing/older/newer (FR-B4) — S2 makes the value AVAILABLE; it does not implement the warning.

UPSTREAM (the S1 contract S2 consumes — do NOT edit):
  - config.RoleConfig{Provider,Model}, config.CurrentConfigVersion=2, config.Config fields Roles/
        ConfigVersion/MaxCommits/BinaryExtensions (S1 SHIPPED, staged).

FROZEN/LEAVE (do NOT edit):
  - internal/config/config.go (+_test.go), load.go (+_test.go), git.go (+_test.go).
  - internal/provider/*, internal/git/*, internal/prompt/*, internal/generate/*, internal/ui/*,
    internal/cmd/root.go (+_test.go), internal/cmd/config_test.go, pkg/*, cmd/*.
  - PRD.md, Makefile, providers/*.toml, docs/*.

NO NEW DATABASE / ROUTES / CLI COMMANDS / FILES.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/config/file.go internal/config/file_test.go internal/cmd/config.go
go vet ./internal/config/ ./internal/cmd/
# Confirm go.mod/go.sum unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm file.go gained the new decode types/fields (no Commits/Single — those are runtime-only):
grep -n "fileRoleConfig\|ConfigVersion\|MaxCommits\|BinaryExtensions" internal/config/file.go
# Expected: go vet clean; go.mod/go.sum byte-unchanged; the four new file.go artifacts present.
```

### Level 2: Config-package unit tests (the new tests + no regression)

```bash
go test ./internal/config/ -v
# Expected PASS — verify explicitly:
#   TestLoadTOML_V2Fields .......... config_version/max_commits/binary_extensions/[role.*] decode + materialize
#   TestOverlayRolesFieldMerge ..... FR-R3: lower-layer planner.provider SURVIVES a higher-layer model-only override
#   TestOverlay_V2Scalars .......... ConfigVersion/MaxCommits/BinaryExtensions non-zero-wins + partial-merge preserve
#   TestLoadTOMLValid / TestOverlayPartial / TestOverlayProvidersFieldMerge ... STILL green (no regression)
# If TestOverlayRolesFieldMerge fails on "provider SURVIVES", the Roles merge is a whole-entry replace (bug) —
# use the per-field existing := dst.Roles[role] pattern. If TestLoadTOML_V2Fields fails, a toml tag is wrong.
```

### Level 3: Whole-repo build/test + cmd-package tests + frozen-file check

```bash
go build ./...     # Expect clean (S2 only populates fields S1 declared).
go test ./...      # Expect all PASS — incl. internal/cmd (the exampleConfigTemplate equality test stays green
                   # because it compares the written file to the constant; the template is still all-commented).
# Confirm the LEAVE files are byte-unchanged:
git diff --exit-code internal/config/config.go internal/config/config_test.go internal/config/load.go \
  internal/config/load_test.go internal/config/git.go internal/config/git_test.go \
  internal/provider internal/git internal/prompt internal/generate internal/ui \
  internal/cmd/root.go internal/cmd/config_test.go cmd pkg Makefile go.mod go.sum PRD.md \
  && echo "frozen files UNCHANGED (expected)"
# Confirm ONLY the three target files changed:
git diff --name-only | grep -E 'internal/config/file\.go|internal/config/file_test\.go|internal/cmd/config\.go' \
  && echo "target files modified (expected)"
```

### Level 4: Correctness reasoning (no runtime to start)

```bash
# S2 is pure decode + field-copy + field-merge + template text — no server/DB/subprocess. Verify by reasoning
# + the tests:
#   1. Decode: [role.planner]/[role.stager] → fc.Role (mirrors [provider.X] → fc.Provider); config_version →
#      fc.ConfigVersion; [generation] max_commits/binary_extensions → fc.Generation.* (TestLoadTOML_V2Fields).
#   2. Materialize: the four copy blocks populate Config.Roles/MaxCommits/BinaryExtensions/ConfigVersion with
#      non-zero/non-empty semantics (TestLoadTOML_V2Fields asserts the materialized values).
#   3. Overlay Roles field-merge: a higher layer's partial role does NOT clobber a lower layer's fields
#      (TestOverlayRolesFieldMerge — the FR-R3 guard, mirror of the FR37a provider guard).
#   4. Overlay scalars: non-zero-wins + partial-merge preservation (TestOverlay_V2Scalars).
#   5. Template: every added line is # -commented (INERT); the cmd equality test stays green.
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/` clean.
- [ ] `go test ./...` PASS (config suite incl. the 3 new tests + no repo-wide regression; cmd suite green).
- [ ] go.mod/go.sum byte-unchanged.
- [ ] config.go (S1) + load.go + git.go (+tests) + every non-target file byte-unchanged; PRD.md byte-unchanged.

### Feature Validation
- [ ] `fileConfig` has `ConfigVersion` (first) + `Role` map (between Generation and Provider); `fileRoleConfig`
      defined; `fileGeneration` has `MaxCommits` + `BinaryExtensions`.
- [ ] `materialize()` copies ConfigVersion/MaxCommits (non-zero), BinaryExtensions (non-empty), and converts
      Role → Roles (every present role).
- [ ] `overlay()` FIELD-MERGES Roles (lower-layer per-role fields survive a higher-layer partial) + non-zero/
      non-empty-wins ConfigVersion/MaxCommits/BinaryExtensions.
- [ ] `exampleConfigTemplate` shows commented `# max_commits` + `# binary_extensions` in [generation] and a
      commented `[role.*]` section; all added lines `#`-commented.
- [ ] `TestLoadTOML_V2Fields` / `TestOverlayRolesFieldMerge` / `TestOverlay_V2Scalars` pass.

### Code Quality Validation
- [ ] Follows existing conventions: non-zero/non-empty materialize+overlay semantics; the `[provider.X]`
      field-merge pattern mirrored for `[role.<role>]`; white-box `package config` tests mirroring
      TestOverlayProvidersFieldMerge/TestOverlayPartial/TestLoadTOMLValid; commented-everywhere template.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (LEAVE files untouched).

### Documentation
- [ ] Decode-struct doc comments cite PRD §16.2/§16.4/§9.17/§9.14/§9.1 and the FRs. materialize/overlay doc
      comments cite FR-R3/FR37a (the field-merge) + the ConfigVersion-advisory-is-P1.M4.T1 note.
- [ ] The exampleConfigTemplate sections document the v2 keys (Mode A — the template IS the user-facing
      config reference until P1.M4.T2 rewrites it to a populated bootstrap).

---

## Anti-Patterns to Avoid

- ❌ **Don't touch config.go / load.go / git.go.** config.go is S1 (SHIPPED — S2 only writes INTO its fields);
  load.go is P1.M3.T2 (env/flags/ResolveRoleModel); git.go is untouched. Commits/Single are `toml:"-"`
  runtime-only — NEVER file keys, so do NOT add them to any decode struct. (design-decisions §0)
- ❌ **Don't whole-block-replace Roles in overlay.** `dst.Roles[role] = rc` drops the lower-layer provider.
  Use the per-field `existing := dst.Roles[role]; if rc.X != "" { existing.X = rc.X }; dst.Roles[role] =
  existing` pattern — the typed analog of the `[provider.X]` field loop. FR-R3 is the per-role FR37a.
  (design-decisions §5)
- ❌ **Don't append BinaryExtensions across layers.** Overlay REPLACES (non-empty wins). The "merges with
  built-in denylist" is a P2.M1 RUNTIME merge, not a cross-layer config concatenation. (design-decisions §7)
- ❌ **Don't implement the config_version advisory.** S2 only plumbs config_version through (non-zero-wins).
  The §9.17 FR-B4 "missing/older → warn" must inspect the raw file at the loadTOML seam (P1.M4.T1.S1) — it
  CANNOT be done on the resolved Config (Defaults() pins 2). (design-decisions §6)
- ❌ **Don't special-case ConfigVersion in materialize.** Use the same non-zero check as every other field
  (unconditional copy is functionally identical because overlay skips 0 — keep materialize uniform).
- ❌ **Don't filter empty roles in materialize.** Copy every present role (an empty `[role.X]` ⇒
  RoleConfig{"",""} = "inherit global", harmless — mirrors Providers' whole-map copy).
- ❌ **Don't add uncommented lines to exampleConfigTemplate.** Every added line MUST be `#`-commented (the
  template is INERT). config_test.go:136's equality test stays green (compares to the constant), but a future
  P1.M4.T2 test may assert "no uncommented key". (design-decisions §8)
- ❌ **Don't reorder existing fileGeneration/fileConfig fields arbitrarily.** Match delta §2 (ConfigVersion
  first; Role between Generation and Provider; MaxCommits/BinaryExtensions appended after StripCodeFence).
- ❌ **Don't add a whole-Config `reflect.DeepEqual` test.** Use per-field/per-role assertions (the existing
  file_test.go style).
- ❌ **Don't implement config_v2_delta.md §3/§4/§5/§6.** Those are load.go (env/flags/ResolveRoleModel),
  bootstrap (config init populated/upgrade/first-run), and the advisory — P1.M3.T2 / P1.M4. S2 implements
  §2 (file decode) ONLY.

---

## Confidence Score

**9/10** — A focused, pattern-following change to three files. The decode-struct additions mirror the
existing `[provider.X]` nested-map decode verbatim; the Roles overlay field-merge is the typed analog of the
in-file `[provider.X]` field-merge loop (read in full); the scalar non-zero-wins matches every existing
overlay field; the tests mirror the three existing file_test.go idioms (TestLoadTOMLValid /
TestOverlayProvidersFieldMerge / TestOverlayPartial). The authoritative spec (architecture/config_v2_delta.md
§2) gives the exact struct fields/tags and the overlay code; the S1 Config fields (the targets) are already
on disk (read + confirmed). The four non-obvious calls — Roles field-merge not whole-block (§5),
BinaryExtensions replace not append (§7), ConfigVersion advisory is NOT S2 (§6), Commits/Single are never
file keys (§0) — are each pinned by a dedicated test row or a frozen-file guard. The one residual risk — an
implementer writing `dst.Roles[role] = rc` (whole-block) — is caught by TestOverlayRolesFieldMerge's
"planner.provider SURVIVES" assertion. The template edit is mechanical (commented lines) and the cmd equality
test is self-referential (compares to the constant), so it cannot regress.
