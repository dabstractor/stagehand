---
name: "P1.M3.T1.S1 — Add RoleConfig type + new Config fields + Defaults() updates: the schema foundation for v2 per-role config, schema versioning, decompose flags, and binary filtering — PRD §16.4 / §9.17 / §9.14 / §9.1 (FR-R1–R5, FR-B4, FR-M2/M4, FR3a)"
description: |

  Land the FIRST subtask of Per-Role Config Schema & Model Resolution (P1.M3.T1): a purely-additive change
  to `internal/config/config.go` that extends the resolved `Config` struct with the v2 fields, defines the
  `RoleConfig` type and the `CurrentConfigVersion` constant, and updates `Defaults()`. It is the SCHEMA
  FOUNDATION: it adds the fields the later subtasks populate/consume — S2 (file.go decode + materialize +
  overlay) wires the file→Config plumbing; P1.M3.T2 (load.go) wires env/flags + ResolveRoleModel; P3/P4
  consume MaxCommits/BinaryExtensions/Commits/Single/Roles at runtime. S1 changes ONLY config.go (struct +
  type + const + Defaults + doc comment) and config_test.go (assertions). It touches NO other file.

  THE V2 FIELDS (from architecture/config_v2_delta.md §1 — AUTHORITATIVE; matches the item contract):
    type RoleConfig struct { Provider string `toml:"provider"`; Model string `toml:"model"` }
    const CurrentConfigVersion = 2
    Config gains:
      Roles            map[string]RoleConfig `toml:"-"`         // per-role overrides (§16.4); loader-populated (S2)
      ConfigVersion    int                   `toml:"config_version"` // schema metadata (§9.17 FR-B4)
      MaxCommits       int                   `toml:"max_commits"`        // decompose safety cap (§9.14 FR-M4); default 12
      BinaryExtensions []string              `toml:"binary_extensions"`  // extra binary exts (§9.1 FR3a); nil ⇒ denylist only
      Commits          int                   `toml:"-"`                  // --commits N (FR-M2); 0 = auto; CLI/runtime only
      Single           bool                  `toml:"-"`                  // --single/--no-decompose (FR-M2); CLI/runtime only

  Defaults() gains (explicit, matching the existing "list every field" style):
      Commits: 0, Single: false, MaxCommits: 12, BinaryExtensions: nil, Roles: nil,
      ConfigVersion: CurrentConfigVersion

  ⚠️ **THE #1 scope boundary — S1 is config.go ONLY.** S1 adds the FIELDS/TYPES/CONST + Defaults + doc
  comment + test assertions. It does NOT touch file.go (fileConfig/fileRoleConfig/fileGeneration decode
  structs + materialize + overlay — that is S2), load.go (env/flags/ResolveRoleModel — that is P1.M3.T2),
  or any consumer. The new Config fields are DECLARED in S1 and zero/nil by default; S2 wires the file→field
  population. Adding the fields now is what lets S2/P1.M3.T2/P3/P4 compile against them. See research
  design-decisions.md §0.

  ⚠️ **THE #2 design call — toml tags split three ways.** (a) `config_version`/`max_commits`/`binary_extensions`
  are real §16.2 file keys → snake_case toml tags (they WILL appear in marshaled TOML). (b) `Roles` is
  `toml:"-"` — it mirrors `Providers map[string]map[string]any toml:"-"`: a loader-populated map that is
  NEVER directly TOML-decoded on Config (the `[role.<role>]` FILE tables decode into fileConfig's map in S2
  and materialize into this typed map). (c) `Commits`/`Single` are `toml:"-"` — CLI/runtime-only, exactly
  like `NoColor` (set by `--commits`/`--single` flags in P4.M1.T1, never by a file). Getting these tags right
  is the whole point of S1; a wrong tag leaks a runtime flag into config files or hides a file key. See
  design-decisions.md §2 + research toml-tag-probe.md.

  ⚠️ **THE #3 design call — field placement follows the existing section grouping.** MaxCommits +
  BinaryExtensions go in the `[generation]` group (per §16.2 they live under [generation]); Commits + Single
  go in the `CLI / UI only` group next to NoColor (all three are `toml:"-"` runtime fields); Roles goes next
  to Providers (both `toml:"-"` loader maps); ConfigVersion is its own trailing `schema version` group
  (top-level metadata key, not under any [section]). Existing fields stay in their EXACT current order — new
  fields are appended into their logical groups for a clean, reviewable diff. See design-decisions.md §3.

  ⚠️ **THE #4 gotcha — Defaults() sets non-zero values for MaxCommits + ConfigVersion.** `MaxCommits: 12` and
  `ConfigVersion: CurrentConfigVersion` (2) are NON-zero defaults. This is correct (the shipped defaults) but
  means a marshaled Defaults() now emits `config_version = 2` and `max_commits = 12`. The existing
  `TestTOMLMarshalKeysAndNoColorExclusion` only checks PRESENCE of specific keys + ABSENCE of no_color, so it
  does NOT break — but S1 ADDS assertions that the `toml:"-"` fields (Roles/Commits/Single) never leak and
  that config_version/max_commits DO appear. See design-decisions.md §4.

  ⚠️ **THE #5 gotcha — NO new imports, NO go.mod change.** config.go currently imports ONLY `time`. Every new
  field is a plain type (`int`, `bool`, `[]string`, `map[string]RoleConfig`) or a same-package type
  (`RoleConfig`); the const is an untyped `2`. `go mod tidy` MUST be a no-op; `git diff --exit-code go.mod
  go.sum` MUST be empty.

  ⚠️ **THE #6 gotcha — CurrentConfigVersion is an UNTYPED const.** `const CurrentConfigVersion = 2` (not
  `const CurrentConfigVersion int = 2`). Untyped is idiomatic for a version constant and assigns cleanly to
  the `int` field `ConfigVersion: CurrentConfigVersion`. See design-decisions.md §5.

  Deliverable: MODIFIED `internal/config/config.go` — `RoleConfig` type + `CurrentConfigVersion` const + six
  new `Config` fields (with the exact toml tags above, in the grouped placement) + updated `Defaults()` +
  expanded Config-struct doc comment (Mode A). PLUS MODIFIED `internal/config/config_test.go` — the six new
  field assertions added to `TestDefaults` + a new `TestConfig_V2TOMLTags` proving the tag split (Roles/
  Commits/Single excluded; config_version/max_commits/binary_extensions present). NO other file touched.
  OUTPUT: Config has all v2 fields; Defaults() is v2-compatible; `go build ./... && go test ./...` green.

---

## Goal

**Feature Goal**: Extend the resolved `Config` struct (PRD §16.1) with the v2 schema — per-role overrides
(`Roles map[string]RoleConfig`), schema versioning (`ConfigVersion` + `CurrentConfigVersion`), the
auto-decompose safety cap (`MaxCommits`) and behavioral flags (`Commits`/`Single`), and binary-extension
filtering (`BinaryExtensions`) — so that the downstream subtasks (S2 file decode, P1.M3.T2 env/flags/
ResolveRoleModel, P3 decompose, P4 CLI) can compile against and populate them. This is the typed schema
foundation; it adds fields/types/const + Defaults + docs and validates them, changing no behavior.

**Deliverable** (all EDITS — no new files):
1. `internal/config/config.go`:
   - DEFINE `type RoleConfig struct { Provider string \`toml:"provider"\`; Model string \`toml:"model"\` }`
     + a doc comment (PRD §16.4, FR-R1–R5).
   - DEFINE `const CurrentConfigVersion = 2` + a doc comment (PRD §9.17 FR-B4).
   - ADD six fields to `Config` with the exact toml tags + grouped placement (see Implementation Blueprint):
     `Roles map[string]RoleConfig \`toml:"-"\``, `ConfigVersion int \`toml:"config_version"\``,
     `MaxCommits int \`toml:"max_commits"\``, `BinaryExtensions []string \`toml:"binary_extensions"\``,
     `Commits int \`toml:"-"\``, `Single bool \`toml:"-"\``.
   - UPDATE `Defaults()` to set all six (Commits: 0, Single: false, MaxCommits: 12, BinaryExtensions: nil,
     Roles: nil, ConfigVersion: CurrentConfigVersion).
   - EXPAND the `Config` struct doc comment (Mode A) to document the v2 fields, RoleConfig, and
     CurrentConfigVersion.
2. `internal/config/config_test.go`:
   - EXTEND `TestDefaults` with six assertions for the new Defaults() values.
   - ADD `TestConfig_V2TOMLTags` proving the tag split: marshaling a populated Config emits
     `config_version`/`max_commits`/`binary_extensions`; marshaling a Config with Roles/Commits/Single set
     does NOT leak them (toml:"-" honored) — mirroring the existing NoColor leak check.

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `Defaults()` returns a
Config whose v2 fields match the contract (ConfigVersion==2, MaxCommits==12, Commits==0, Single==false,
BinaryExtensions==nil, Roles==nil); the toml tag split is proven by `TestConfig_V2TOMLTags`; go.mod/go.sum
byte-unchanged; file.go/load.go/git.go and every non-config file byte-unchanged; config.go still imports
only `time`.

## User Persona

**Target User**: The downstream v2 subtasks that populate and consume these fields:
- **S2 (file.go)** — will add `fileRoleConfig`/extended `fileGeneration`/`fileConfig.ConfigVersion` decode
  structs and copy them into `Config.Roles`/`MaxCommits`/`BinaryExtensions`/`ConfigVersion` via `materialize`
  + field-merge them via `overlay`. S1 declares the TARGET fields S2 writes into.
- **P1.M3.T2 (load.go)** — will add `STAGECOACH_<ROLE>_{PROVIDER,MODEL}`, `STAGECOACH_COMMITS` env handling,
  `--commits`/`--single`/`--max-commits`/`--<role>-*` flags, and `ResolveRoleModel`. S1 declares the fields
  these set (`cfg.Commits`, `cfg.Single`, `cfg.Roles`).
- **P3 (decompose)** — reads `MaxCommits` (safety cap, FR-M4), `Commits`/`Single` (mode, FR-M2), `Roles`
  (per-role resolution).
- **P2.M1 (binary filtering)** — reads `BinaryExtensions` (FR3a denylist merge).
End-user persona is "the plan-holder"/"the multi-agent tinkerer" (PRD §7) who, once the full v2 lands, gets
per-role model routing + multi-commit decomposition + binary filtering — all gated on these fields existing.

**Use Case**: (internal schema, no end-user surface in S1) Every v2 feature reads `cfg.<V2Field>`. S1 makes
those fields exist with correct defaults so the feature subtasks compile and have a sane Layer-1 baseline
(e.g. decompose's safety cap is 12 even before any config file is read).

**User Journey**: (internal) `config.Defaults()` → Config{ConfigVersion:2, MaxCommits:12, ...} →
`config.Load` overlays file/env/flag (S2/P1.M3.T2) onto it → consumers read `cfg.Roles["planner"]`,
`cfg.MaxCommits`, `cfg.Commits`, etc. S1 is step zero (the typed shape + Layer-1 defaults).

**Pain Points Addressed**: Without these fields the v2 subtasks cannot compile (they reference fields that
don't exist). Without correct toml tags, file decoding (S2) would silently drop keys or leak runtime flags
into config files. S1 removes both blockers with one small, reviewable, behavior-free change.

## Why

- **Unblocks the entire v2 config + decompose track.** P1.M3 (per-role config), P2.M1 (binary filtering),
  P3 (decompose), P4 (CLI flags + public API) all reference these fields. S1 is the typed schema they build
  on — landing it first means every later subtask compiles against a stable `Config` shape.
- **Satisfies PRD §16.4 (FR-R1–R5), §9.17 (FR-B4), §9.14 (FR-M2/M4), §9.1 (FR3a), §16.2.** Roles =
  per-role overrides; ConfigVersion/CurrentConfigVersion = schema versioning; MaxCommits = safety cap;
  BinaryExtensions = binary denylist extension; Commits/Single = decompose mode flags. Each field maps to a
  cited FR.
- **Faithful to the existing design calls.** Config stays flat + plain-typed + RESOLVED (the struct doc
  comment's invariant). The new fields follow the EXACT patterns already in the file: `Roles` mirrors
  `Providers` (`toml:"-"` loader map); `Commits`/`Single` mirror `NoColor` (`toml:"-"` runtime field);
  `MaxCommits`/`BinaryExtensions` join the `[generation]` scalars; `ConfigVersion` is metadata (§16.1:
  "config_version is metadata, not a precedence layer").
- **Zero behavior change.** S1 adds fields + defaults + docs + tests. No loader, no consumer, no CLI is
  modified. Existing code that constructs `config.Config{}` (empty) or doesn't set the new fields gets zero
  values and behaves identically (the contract's "existing code compiles; new fields are zero-valued").

## What

A modified `internal/config/config.go` with one new type (`RoleConfig`), one new const
(`CurrentConfigVersion`), six new `Config` fields (with the exact toml tags and grouped placement), an
expanded `Defaults()` and struct doc comment; and a modified `internal/config/config_test.go` with the new
field assertions and a toml-tag proof test. No new files, no new imports, no logic change, no dependency
change. The schema is the deliverable.

### Success Criteria

- [ ] `RoleConfig` is defined: `type RoleConfig struct { Provider string \`toml:"provider"\`; Model string
      \`toml:"model"\` }` with a doc comment citing PRD §16.4 / FR-R1–R5.
- [ ] `CurrentConfigVersion` is defined: `const CurrentConfigVersion = 2` (UNtyped) with a doc comment
      citing PRD §9.17 FR-B4.
- [ ] `Config` has the six new fields with EXACT tags: `Roles map[string]RoleConfig \`toml:"-"\``,
      `ConfigVersion int \`toml:"config_version"\``, `MaxCommits int \`toml:"max_commits"\``,
      `BinaryExtensions []string \`toml:"binary_extensions"\``, `Commits int \`toml:"-"\``,
      `Single bool \`toml:"-"\``.
- [ ] Field placement: Commits+Single in the `CLI / UI only` group (next to NoColor); MaxCommits+
      BinaryExtensions in the `[generation]` group; Roles next to Providers; ConfigVersion as its own
      trailing group. Existing fields keep their current order.
- [ ] `Defaults()` returns `Commits: 0, Single: false, MaxCommits: 12, BinaryExtensions: nil, Roles: nil,
      ConfigVersion: CurrentConfigVersion` (the existing fields unchanged).
- [ ] The `Config` struct doc comment documents the v2 fields, RoleConfig, and CurrentConfigVersion (Mode A).
- [ ] `TestDefaults` asserts all six new Defaults() values.
- [ ] NEW `TestConfig_V2TOMLTags`: marshaling proves `config_version`/`max_commits`/`binary_extensions`
      appear and `Roles`/`Commits`/`Single` do NOT leak (toml:"-" honored).
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/config/` clean.
- [ ] go.mod/go.sum byte-unchanged; config.go imports ONLY `time`; file.go/load.go/git.go + every non-config
      file byte-unchanged; PRD.md byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact six fields + tags
(quoted verbatim above and in the Blueprint), the `RoleConfig`/`CurrentConfigVersion` definitions, the grouped
placement rule, the Defaults() additions, the architecture delta §1 (the authoritative source), the test
additions, and the LEAVE list (file.go/load.go are S2/P1.M3.T2). No decompose/git/prompt knowledge required
— S1 is six struct fields + a type + a const + defaults + docs + assertions.

### Documentation & References

```yaml
# MUST READ — the authoritative field definitions (matches the item contract verbatim)
- docfile: plan/002_a17bb6c8dc1d/architecture/config_v2_delta.md
  section: "§1. Config Struct Changes" — the EXACT new fields, toml tags, Defaults() additions, and the
           CurrentConfigVersion const. This is the spec S1 implements; the item description is a summary of it.
  critical: §1 is the source of truth for field names/types/tags/defaults. §2/§3/§4/§5 are OUT OF SCOPE for S1
       (they are file.go/load.go/bootstrap — S2, P1.M3.T2, P1.M4). Read §1 only for the schema; do NOT
       implement §2–§5 here.

# MUST READ — the design calls + the go-toml tag-behavior probe (validates the tag split)
- docfile: plan/002_a17bb6c8dc1d/P1M3T1S1/research/design-decisions.md
  why: the 6 non-obvious calls — scope boundary S1≠S2 (§0), the three-way toml tag split (§1), the per-role
       toml:"-" rationale mirroring Providers (§2), field placement/grouping (§3), the non-zero Defaults
       (MaxCommits/ConfigVersion) + why they don't break the existing marshal test (§4), untyped const (§5),
       test strategy (§6).
  critical: §0 (config.go ONLY — do NOT touch file.go/load.go), §1/§2 (the tag split is the whole point),
       §4 (Defaults gains non-zero values — add the V2TOMLTags test) are the things most likely to go wrong.

- docfile: plan/002_a17bb6c8dc1d/P1M3T1S1/research/toml-tag-probe.md
  why: empirically verifies go-toml/v2's behavior for the three tag classes on THIS struct — `toml:"key"`
       scalars emit the key; `toml:"-"` is fully excluded (never leaks, even when set); a nil `[]string` with
       a real tag marshals to an empty array line (so BinaryExtensions=nil ⇒ `binary_extensions = []` in the
       marshal output — harmless, asserted as "present when set"). Backs design-decisions §1/§4.
  critical: confirms `toml:"-"` fully excludes Roles/Commits/Single (the V2TOMLTags leak assertion is sound)
       and that adding config_version/max_commits/binary_extensions to the marshal output does NOT break
       TestTOMLMarshalKeysAndNoColorExclusion (it checks presence of specific keys + absence of no_color only).

# The PRD basis (already in your context as selected_prd_content)
- file: PRD.md (or plan/002_a17bb6c8dc1d/prd_snapshot.md)
  section: "16.4 Per-role provider/model configuration" (h3.68) — RoleConfig's semantics (4 roles, field-
           merge, "" ⇒ inherit global).
  section: "9.17 Config bootstrap & versioning" (h3.33) FR-B4 — config_version / CurrentConfigVersion
           advisory semantics ("metadata, not a precedence layer" per §16.1).
  section: "16.2 Full config file example" — confirms the snake_case file keys `config_version`,
           `max_commits`, `binary_extensions` live under [generation]/top-level (NOT under [defaults]).
  section: "9.14 Multi-commit decomposition" FR-M2/M4 — Commits/Single (mode) + MaxCommits (safety cap=12).
  section: "9.1 Diff capture" FR3a — BinaryExtensions (denylist extension).
  critical: §16.1 — "config_version is metadata, not a precedence layer" (why ConfigVersion is not resolved,
       just carried + compared). §16.2 — binary_extensions/max_commits are [generation] keys.

# The file being modified — READ FULLY before editing
- file: internal/config/config.go
  section: the Config struct + its doc comment + Defaults() + the boolPtr/strPtr helpers.
  why: the EXACT current state S1 edits. Note Output is `*string` and StripCodeFence is `*bool` (nil ⇒
       manifest/true). Note the section grouping ([defaults] / CLI-UI-only / [generation] / Providers) that
       the new fields slot into. Note config.go imports ONLY `time` (S1 adds no import).
  critical: the struct doc comment's "flat + plain-typed + RESOLVED ... NOT unmarshaled directly" invariant
       MUST be preserved/extended — the new fields follow it (Roles toml:"-" loader-populated like Providers;
       Commits/Single toml:"-" runtime like NoColor). Do NOT make Config directly TOML-decoded.

# The test file being modified
- file: internal/config/config_test.go
  section: TestDefaults (per-field assertions — EXTEND with the 6 new fields) + TestTOMLMarshalKeysAndNoColorExclusion
           (the marshal-key presence + NoColor-leak-absence pattern to MIRROR for the V2TOMLTags test).
  why: the test STYLE to follow — one t.Errorf per field, clear want messages; the leak-check pattern
       (marshal a Config with the field set, assert the key string is absent). config_test.go is `package
       config` (white-box) — same as every _test.go here.
  critical: TestTOMLMarshalKeysAndNoColorExclusion checks PRESENCE of a fixed key list + ABSENCE of no_color
       only — adding config_version/max_commits/binary_extensions to the marshal output does NOT break it
       (verified in toml-tag-probe.md). Do NOT add an "exactly these keys" assertion.

# The LEAVE files (S2 / P1.M3.T2 own them — do NOT edit in S1)
- file: internal/config/file.go   (fileConfig/fileDefaults/fileGeneration/materialize/overlay)
  why: S2 adds the [role.<role>] decode structs (fileRoleConfig), extends fileGeneration (+max_commits,
       +binary_extensions) and fileConfig (+config_version), and wires materialize()+overlay() to populate
       Config.Roles/MaxCommits/BinaryExtensions/ConfigVersion. S1 only DECLARES the target fields.
- file: internal/config/load.go   (Load/loadEnv/loadFlags)
  why: P1.M3.T2 adds STAGECOACH_<ROLE>_* / STAGECOACH_COMMITS env, --commits/--single/--max-commits/--<role>-*
       flags, and ResolveRoleModel. S1 only DECLARES Commits/Single/Roles.
- file: internal/config/git.go   (git-config reader)
  why: untouched by the per-role schema work in S1.

# The frozen plumbing (read-only reference — confirms no import-cycle / no breakage)
- file: internal/provider/manifest.go
  why: RoleConfig is intentionally a plain {Provider,Model} struct (NOT the Manifest type) so config does NOT
       import provider (no import cycle). The registry (P1.M2.T3) consumes Config.Providers (a raw map) for
       the same reason; Roles follows the same decoupling.
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go        # the Config struct + Defaults() + boolPtr/strPtr — EDIT (+RoleConfig +CurrentConfigVersion +6 fields +Defaults +doc)
  config_test.go   # TestDefaults + TestTOMLMarshalKeysAndNoColorExclusion — EDIT (+6 assertions +TestConfig_V2TOMLTags)
  file.go          # fileConfig/fileGeneration/materialize/overlay — UNCHANGED (S2's scope)
  load.go          # Load/loadEnv/loadFlags — UNCHANGED (P1.M3.T2's scope)
  git.go           # git-config reader — UNCHANGED
  file_test.go / load_test.go / git_test.go — UNCHANGED
go.mod / go.sum    # UNCHANGED (config.go stays `time`-only; no new dep)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. All edits are in-place modifications to internal/config/config.go + config_test.go.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — S1 is config.go ONLY): do NOT touch file.go (fileConfig/materialize/overlay — S2), load.go
// (loadEnv/loadFlags/ResolveRoleModel — P1.M3.T2), or git.go. S1 DECLARES the fields; S2/P1.M3.T2 POPULATE
// them. The new fields are zero/nil by default so existing code compiles unchanged.

// CRITICAL (#2 — the three-way toml tag split): (a) config_version/max_commits/binary_extensions are real
// §16.2 file keys → snake_case toml tags (they appear in marshaled TOML). (b) Roles is toml:"-" — it mirrors
// Providers (a loader-populated map NEVER directly TOML-decoded on Config; the [role.*] FILE tables decode
// into fileConfig's map in S2 then materialize into this typed map). (c) Commits/Single are toml:"-" —
// CLI/runtime-only, exactly like NoColor. A wrong tag leaks a runtime flag into files or hides a file key.

// CRITICAL (#3 — Defaults() gains NON-ZERO values): MaxCommits: 12 and ConfigVersion: CurrentConfigVersion
// (2) are non-zero. This is correct. It does NOT break TestTOMLMarshalKeysAndNoColorExclusion (that test
// checks presence of a fixed key list + absence of no_color — verified in toml-tag-probe.md). S1 ADDS
// TestConfig_V2TOMLTags to prove the tag split.

// CRITICAL (#4 — CurrentConfigVersion is UNTYPED): `const CurrentConfigVersion = 2` (not `... int = 2`).
// Untyped is idiomatic, assigns cleanly to the int field. Do NOT type it.

// GOTCHA (field placement follows existing sections): Commits+Single → CLI/UI-only group (next to NoColor);
// MaxCommits+BinaryExtensions → [generation] group; Roles → next to Providers; ConfigVersion → trailing
// schema-version group. Existing fields keep their EXACT current order (clean diff).

// GOTCHA (RoleConfig must NOT import provider): RoleConfig is a plain {Provider, Model} struct, NOT the
// Manifest type — so config stays a leaf (no import of internal/provider). Mirrors Providers' raw-map
// decoupling. The toml tags on RoleConfig are documentary (Config.Roles is toml:"-" so go-toml never
// touches RoleConfig during Config marshal/unmarshal) — include them per the delta doc; they're harmless.

// GOTCHA (no new imports): config.go imports ONLY `time` today. Every new field is a plain type or a
// same-package type; the const is untyped. `go mod tidy` MUST be a no-op.

// GOTCHA (existing Config{} literals are safe): the repo has empty `config.Config{}` literals
// (pkg/stagecoach error paths) and ONE keyed literal (pkg/stagecoach/stagecoach_test.go:599 — keyed, not
// positional). Adding fields breaks NEITHER (Go keyed/empty literals tolerate added fields). Verified.

// GOTCHA (TestDefaults is per-field, not whole-struct): TestDefaults asserts each field individually — it
// does NOT do a reflect.DeepEqual on the whole Config. So adding fields + Defaults values cannot break it;
// S1 EXTENDS it with the 6 new assertions. (No whole-Config DeepEqual exists anywhere — verified by grep.)

// GOTCHA (in-package white-box tests): config_test.go is `package config`, so it can reference the
// unexported strPtr/boolPtr helpers + the new RoleConfig/CurrentConfigVersion directly. No import needed.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/config.go — the FULL modified file (existing fields UNCHANGED in order/value; new
// artifacts marked // V2). config.go still imports ONLY "time".

package config

import (
	"time"
)

func boolPtr(b bool) *bool { return &b }

func strPtr(s string) *string { return &s }

// CurrentConfigVersion is the config-schema version this binary understands (PRD §9.17 FR-B4). Bumped on any
// breaking config change. On load, stagecoach compares a config file's config_version to this constant and
// emits an advisory staleness ("older") or ahead ("newer") warning pointing at `config upgrade` / `config
// init --force`; it is advisory only — no automatic migration (there are no existing users to migrate).
// config_version is metadata, NOT a precedence layer (PRD §16.1). v2 = per-role models + multi-commit
// decomposition + binary filtering.
const CurrentConfigVersion = 2

// RoleConfig holds a per-role provider/model override (PRD §16.4, §9.15 FR-R1–R5). A role is one of
// "planner", "stager", "message", "arbiter" (§13.6.2). Both fields "" ⇒ the role inherits the global
// [defaults] (FR-R2); a non-empty value overrides just that field (FR-R3 field-merge across layers). Model
// strings are provider-specific (FR-R5): a role's Model is interpreted by that role's resolved Provider's
// manifest, so changing a role's Provider without updating its Model is a configuration error stagecoach
// surfaces. For multi-provider agents (pi/opencode/agy) Provider is required when Model is set (FR-R5b).
//
// Config.Roles (below) carries the RESOLVED per-role table; it is toml:"-" because the [role.<role>] FILE
// tables decode into fileConfig's fileRoleConfig map (S2) and materialize/overlay into this typed map — the
// same raw-map→typed-field pattern Config.Providers uses.
type RoleConfig struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
}

// Config is the fully-resolved Stagecoach configuration: the single value produced by the 7-layer
// precedence resolver (PRD §16.1, FR34) and read by every consumer — the TOML/git/env/CLI loaders
// (P1.M1.T4.S2-S4, extended in v2 P1.M3.T1.S2 + P1.M3.T2), the provider registry (P1.M2.T3), and the
// generation/decompose pipeline (v1 §13 / v2 §13.6).
//
// DESIGN CALL: flat + plain-typed + RESOLVED. Every field holds a concrete value (Timeout is already
// a time.Duration). This struct is NOT unmarshaled directly from the §16.2 file: that file uses
// [defaults]/[generation]/[role.<role>]/[provider.<name>] subtables and string durations ("120s"). The
// loaders (file.go's fileConfig intermediate structs + materialize/overlay; load.go's env/flag layers)
// decode into their own intermediate structs and merge field-by-field INTO this plain Config. Keeping
// Config plain means consumers read cfg.Timeout / cfg.Roles["planner"].Model with zero dereferencing
// (except the deliberately-pointer Output/StripCodeFence, whose nil ⇒ "defer to the manifest"). The toml
// tags use §16.2 snake_case leaf names for the file-backed fields; toml:"-" marks loader-populated maps
// (Providers, Roles) and CLI/runtime-only fields (NoColor, Commits, Single) that NEVER appear in a file.
//
// V2 FIELDS (this subtask, P1.M3.T1.S1):
//   - Roles / ConfigVersion / MaxCommits / BinaryExtensions / Commits / Single (see inline comments).
//   - RoleConfig (above) + CurrentConfigVersion (above) are the supporting type/const.
//   - File→Config plumbing for Roles/ConfigVersion/MaxCommits/BinaryExtensions lands in S2 (file.go);
//     env/flag wiring for Commits/Single/Roles + ResolveRoleModel land in P1.M3.T2 (load.go).
type Config struct {
	// [defaults] (PRD §16.2)
	Provider     string        `toml:"provider"`       // "" => auto-detect (PRD §15.2)
	Model        string        `toml:"model"`          // "" => provider manifest default_model
	Timeout      time.Duration `toml:"timeout"`        // generation timeout; Defaults: 120s
	AutoStageAll bool          `toml:"auto_stage_all"` // git add -A when nothing staged (PRD §9.4)
	Verbose      bool          `toml:"verbose"`        // print resolved cmd, raw output, retries

	// CLI / UI only — NOT in the §16.2 config file (toml:"-"). Set by flags/env at runtime, never by a file.
	NoColor bool `toml:"-"` // --no-color / STAGECOACH_NO_COLOR / NO_COLOR; TTY-aware at runtime (UI layer)
	// V2 decompose mode flags (PRD §9.14 FR-M2) — set by --commits/--single (P1.M3.T2/P4.M1.T1), not files.
	Commits int  `toml:"-"` // --commits N (N≥2 forces exactly N commits); 0 = auto-decompose (planner decides); --commits 1 ⇒ Single
	Single  bool `toml:"-"` // --single/--no-decompose: bypass the planner entirely (v1 single-commit path)

	// [generation] (PRD §16.2)
	MaxDiffBytes        int     `toml:"max_diff_bytes"`        // byte cap on non-markdown diff section
	MaxMdLines          int     `toml:"max_md_lines"`          // per-file line cap for markdown diffs
	MaxDuplicateRetries int     `toml:"max_duplicate_retries"` // re-gen attempts on duplicate subject
	SubjectTargetChars  int     `toml:"subject_target_chars"`  // target subject length for truncation
	Output              *string `toml:"output"`                // nil ⇒ honor manifest (S2 bridge); non-nil ⇒ override
	StripCodeFence      *bool   `toml:"strip_code_fence"`      // strip ``` fences from agent output; nil ⇒ true
	// V2 generation tuning (PRD §16.2, §9.1 FR3a, §9.14 FR-M4) — decoded from [generation] in S2.
	MaxCommits       int      `toml:"max_commits"`       // safety cap on auto-decompose (default 12; FR-M4)
	BinaryExtensions []string `toml:"binary_extensions"` // extra non-text exts to filter (FR3a); nil ⇒ built-in denylist only

	// [provider.<name>] user-defined / override provider definitions (PRD §16.2, §12.8).
	// Carried as a RAW map: the provider MANIFEST type lives in internal/provider, so config must not import
	// it (import-cycle risk). The registry (P1.M2.T3) consumes this map — for each name it re-encodes the
	// entry to TOML and unmarshals into a Manifest, then field-merges with the built-in manifest per §16.1.
	// toml:"-" => excluded from flat marshal/unmarshal (Config is never decoded from §16.2; fileConfig is).
	// Populated by the file loaders (P1.M1.T4.S2); nil means "no user-defined providers".
	Providers map[string]map[string]any `toml:"-"`

	// V2 per-role provider/model overrides (PRD §16.4, §9.15 FR-R1–R5). Keyed by role name ("planner",
	// "stager", "message", "arbiter"). toml:"-" — populated by the file loaders (S2) from the [role.<role>]
	// tables (field-merged across layers exactly like Providers); nil means "no per-role overrides → every
	// role inherits the global [defaults]" (FR-R2). On the single-commit path the only active role is
	// "message", so a nil Roles is exactly equivalent to v1 (back-compatible).
	Roles map[string]RoleConfig `toml:"-"`

	// V2 schema version (PRD §9.17 FR-B4). Metadata, NOT a precedence layer (§16.1): on load it is compared
	// to CurrentConfigVersion for an advisory warning; it does not participate in value resolution. Decoded
	// from the top-level config_version key in S2; Defaults() pins it to CurrentConfigVersion.
	ConfigVersion int `toml:"config_version"`
}

// Defaults returns the built-in Layer-1 configuration (PRD §16.1): timeout 120s, auto_stage_all true,
// max_diff_bytes 300000, max_md_lines 100, max_duplicate_retries 3, subject_target_chars 50, max_commits 12.
// Output and StripCodeFence are nil (deferred to the manifest's Resolve() — §12.1). Provider and Model are
// "" (Layer 1 does not pin them): empty Provider => auto-detect (PRD §15.2); empty Model => the manifest
// default_model (§16.2). Verbose/NoColor/Single are false; Commits is 0 (auto-decompose). Roles and
// BinaryExtensions are nil (no per-role overrides → all roles use the global; binary filtering uses the
// built-in denylist only). ConfigVersion is pinned to CurrentConfigVersion (a Defaults() Config is always
// current-schema). NoColor is ultimately TTY-aware in the UI layer.
//
// Returned BY VALUE: Config is an immutable resolved snapshot after Load(); a value return avoids
// nil-pointer hazards and lets callers copy freely.
func Defaults() Config {
	return Config{
		Provider:            "",
		Model:               "",
		Timeout:             120 * time.Second,
		AutoStageAll:        true,
		Verbose:             false,
		NoColor:             false,
		Commits:             0, // auto-decompose (PRD §9.14 FR-M2); set by --commits in P1.M3.T2/P4.M1.T1
		Single:              false,
		MaxDiffBytes:        300000,
		MaxMdLines:          100,
		MaxDuplicateRetries: 3,
		SubjectTargetChars:  50,
		Output:              nil,
		StripCodeFence:      nil,
		MaxCommits:          12,      // §9.14 FR-M4 default safety cap on auto-decompose
		BinaryExtensions:    nil,     // nil ⇒ built-in denylist only (§9.1 FR3a)
		Providers:           nil,
		Roles:               nil,     // no per-role overrides → all roles use the global (§16.4 FR-R2)
		ConfigVersion:       CurrentConfigVersion,
	}
}
```

```go
// internal/config/config_test.go — ADDITIONS (the existing TestDefaults + TestTOMLMarshalKeysAndNoColorExclusion
// are EXTENDED/augmented; their existing assertions stay). package config (white-box), imports unchanged
// (strings, testing, time, go-toml/v2 already present).

// ... existing TestDefaults body: AFTER the existing StripCodeFence assertion, ADD:

	// V2 fields (P1.M3.T1.S1)
	if c.Commits != 0 {
		t.Errorf("Commits = %d, want 0 (auto-decompose)", c.Commits)
	}
	if c.Single {
		t.Errorf("Single = true, want false")
	}
	if c.MaxCommits != 12 {
		t.Errorf("MaxCommits = %d, want 12 (FR-M4 default)", c.MaxCommits)
	}
	if c.BinaryExtensions != nil {
		t.Errorf("BinaryExtensions = %v, want nil (built-in denylist only)", c.BinaryExtensions)
	}
	if c.Roles != nil {
		t.Errorf("Roles = %v, want nil (no per-role overrides)", c.Roles)
	}
	if c.ConfigVersion != CurrentConfigVersion {
		t.Errorf("ConfigVersion = %d, want CurrentConfigVersion (%d)", c.ConfigVersion, CurrentConfigVersion)
	}
	if CurrentConfigVersion != 2 {
		t.Errorf("CurrentConfigVersion = %d, want 2", CurrentConfigVersion)
	}

// hasKeyLine reports whether the marshaled TOML has a line whose trimmed form begins with `key =`.
// It is the ROBUST way to assert a key's presence/absence: "commits" is a SUFFIX of "max_commits", so
// both strings.Contains(out, "commits") AND strings.Contains(out, "commits =") FALSE-POSITIVE on the legit
// `max_commits = ...` line. A line-prefix check disambiguates (verified by temp-module validation).
func hasKeyLine(tomlText, key string) bool {
	prefix := key + " ="
	for _, line := range strings.Split(tomlText, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return true
		}
	}
	return false
}

// NEW test — proves the three-way toml tag split (design-decisions §1/§4, toml-tag-probe.md):
func TestConfig_V2TOMLTags(t *testing.T) {
	// (a) file-backed keys appear when set.
	c := Defaults()
	c.MaxCommits = 9
	c.BinaryExtensions = []string{"foo", "bar"}
	data, err := toml.Marshal(c)
	if err != nil {
		t.Fatalf("toml.Marshal err = %v", err)
	}
	s := string(data)
	for _, key := range []string{"config_version", "max_commits", "binary_extensions"} {
		if !hasKeyLine(s, key) {
			t.Errorf("marshaled TOML missing v2 key %q:\n%s", key, s)
		}
	}

	// (b) toml:"-" fields NEVER leak, even when populated (mirrors the NoColor leak check). MUST be
	// line-based (hasKeyLine): a bare strings.Contains on "commits"/"commits =" false-positives on the
	// legit `max_commits = ...` line (commits is a suffix of max_commits).
	leaky := Defaults()
	leaky.Commits = 5
	leaky.Single = true
	leaky.Roles = map[string]RoleConfig{
		"planner": {Provider: "agy", Model: "gemini-2.5-pro"},
	}
	data2, err := toml.Marshal(leaky)
	if err != nil {
		t.Fatalf("toml.Marshal(leaky) err = %v", err)
	}
	s2 := string(data2)
	for _, key := range []string{"commits", "single", "roles"} {
		if hasKeyLine(s2, key) {
			t.Errorf("toml:\"-\" field leaked into TOML as %q:\n%s", key, s2)
		}
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD RoleConfig + CurrentConfigVersion to config.go (before the Config struct)
  - DEFINE `type RoleConfig struct { Provider string `toml:"provider"`; Model string `toml:"model"` }` with
      the doc comment citing PRD §16.4 / FR-R1–R5/R5b. Place it above the Config struct (after strPtr).
  - DEFINE `const CurrentConfigVersion = 2` (UNtyped) with the doc comment citing §9.17 FR-B4 / §16.1.
  - GOTCHA: RoleConfig is a plain struct (NOT the Manifest type) — config must NOT import internal/provider.
  - GOTCHA: the const is UNTYPED (assigns cleanly to the int field).

Task 2: ADD the six v2 fields to the Config struct (grouped placement)
  - CLI/UI-only group (next to NoColor): `Commits int `toml:"-"`` + `Single bool `toml:"-"``.
  - [generation] group (after StripCodeFence): `MaxCommits int `toml:"max_commits"`` + `BinaryExtensions
      []string `toml:"binary_extensions"``.
  - After Providers: `Roles map[string]RoleConfig `toml:"-"``.
  - Trailing group: `ConfigVersion int `toml:"config_version"``.
  - EXACT tags per the Blueprint. Existing fields keep their current order/values. Each new field gets an
      inline doc comment citing its FR.
  - EXPAND the Config struct doc comment (Mode A): add a "V2 FIELDS" paragraph naming the six fields +
      RoleConfig + CurrentConfigVersion + which later subtask wires each. Preserve the existing
      "flat + plain-typed + RESOLVED ... NOT unmarshaled directly" invariant wording (extend, don't drop).

Task 3: UPDATE Defaults() (set all six new fields)
  - ADD to the returned Config literal (in struct order): Commits: 0, Single: false (CLI/UI group);
      MaxCommits: 12, BinaryExtensions: nil ([generation] group); Roles: nil (after Providers);
      ConfigVersion: CurrentConfigVersion (last).
  - PRESERVE every existing field's value. Match the existing "list every field explicitly" style.
  - EXPAND the Defaults() doc comment to name the v2 defaults (max_commits 12, ConfigVersion pinned, Roles/
      BinaryExtensions nil, Commits 0/Single false).

Task 4: EXTEND config_test.go (TestDefaults + new TestConfig_V2TOMLTags)
  - EXTEND TestDefaults: add the 6 new field assertions (Commits==0, Single==false, MaxCommits==12,
      BinaryExtensions==nil, Roles==nil, ConfigVersion==CurrentConfigVersion) + a CurrentConfigVersion==2
      const check. Place after the existing StripCodeFence assertion.
  - ADD TestConfig_V2TOMLTags: (a) marshal Defaults()+MaxCommits/BinaryExtensions set → assert config_version,
      max_commits, binary_extensions appear; (b) marshal a Config with Commits/Single/Roles set → assert
      none of commits/single/roles leak (toml:"-" honored), mirroring the NoColor leak check.
  - GOTCHA: do NOT add a whole-Config reflect.DeepEqual anywhere. Do NOT change the existing
      TestTOMLMarshalKeysAndNoColorExclusion assertions (they still hold — verified in toml-tag-probe.md).

Task 5: VERIFY (no further edits)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. file.go/load.go/git.go +
      their tests + every non-config file byte-unchanged. config.go imports ONLY `time`. All config tests
      green; whole-repo `go test ./...` green.
```

### Implementation Patterns & Key Details

```go
// THE three-way toml tag split (the heart of S1):
//   toml:"config_version" | toml:"max_commits" | toml:"binary_extensions"  → real §16.2 file keys (marshaled)
//   toml:"-"  on Roles     → loader-populated map, NEVER directly decoded (mirrors Providers)
//   toml:"-"  on Commits/Single → CLI/runtime-only, NEVER in a file (mirrors NoColor)

// THE const — UNTYPED (idiomatic; assigns to the int field):
const CurrentConfigVersion = 2

// THE type — plain struct, NO provider import (leaf-package invariant):
type RoleConfig struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
}

// THE Defaults additions — MaxCommits/ConfigVersion are NON-zero (correct; does not break the existing
// marshal test, which checks presence of fixed keys + absence of no_color only):
Commits:          0,
Single:           false,
MaxCommits:       12,
BinaryExtensions: nil,
Roles:            nil,
ConfigVersion:    CurrentConfigVersion,

// THE leak-check pattern (mirror the existing NoColor test) — proves toml:"-" excludes runtime fields.
// GOTCHA (caught by temp-module validation): MUST be LINE-BASED. "commits" is a SUFFIX of "max_commits",
// so strings.Contains(out, "commits") AND strings.Contains(out, "commits =") BOTH match the legit
// `max_commits = ...` line → false positive. Use hasKeyLine (a trimmed line starting with `key =`):
leaky := Defaults()
leaky.Commits = 5
leaky.Single = true
leaky.Roles = map[string]RoleConfig{"planner": {Provider: "agy", Model: "gemini-2.5-pro"}}
data, _ := toml.Marshal(leaky)
for _, key := range []string{"commits", "single", "roles"} {
	if hasKeyLine(string(data), key) {
		t.Errorf("toml:\"-\" runtime field %q leaked into TOML", key)
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): change NONE. config.go stays `time`-only; the new fields/types/const add NO
      import. `go mod tidy` MUST be a no-op. `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES: NONE added. config stays a LEAF (does not import internal/provider — RoleConfig is a plain
      struct, not Manifest; mirrors the Providers raw-map decoupling). No import changes anywhere.

DOWNSTREAM (NOT this task — S1 only DECLARES; these POPULATE/consume):
  - P1.M3.T1.S2 (file.go): adds fileRoleConfig + extends fileGeneration (+max_commits, +binary_extensions)
        + fileConfig (+config_version); materialize() copies them into Config; overlay() field-merges Roles.
  - P1.M3.T2 (load.go): adds STAGECOACH_<ROLE>_{PROVIDER,MODEL} + STAGECOACH_COMMITS env, --commits/--single/
        --max-commits/--<role>-* flags, and ResolveRoleModel(role, cfg).
  - P2.M1.T1 (internal/git/binary.go): merges cfg.BinaryExtensions with the built-in denylist (FR3a).
  - P3.M2/M4 (decompose): reads cfg.MaxCommits (FR-M4 cap), cfg.Commits/cfg.Single (FR-M2 mode), cfg.Roles.
  - P1.M4.T1 (config_version advisory): compares cfg.ConfigVersion to CurrentConfigVersion on load (FR-B4).

FROZEN/LEAVE (do NOT edit in S1):
  - internal/config/file.go (+_test.go), load.go (+_test.go), git.go (+_test.go).
  - internal/provider/*, internal/git/*, internal/prompt/*, internal/generate/*, internal/ui/*, cmd/*, pkg/*.
  - PRD.md, Makefile, providers/*.toml, docs/*.

NO NEW DATABASE / ROUTES / CLI COMMANDS / FILES.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/config/config.go internal/config/config_test.go
go vet ./internal/config/
# Confirm config.go still imports ONLY "time" (S1 adds no import):
head -8 internal/config/config.go   # → import ( "time" )
# Confirm go.mod/go.sum unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; config.go imports only time; go.mod/go.sum byte-unchanged.
```

### Level 2: Config-package unit tests (the new assertions + no regression)

```bash
go test ./internal/config/ -v
# Expected PASS — verify explicitly:
#   TestDefaults ............................ now incl. the 6 v2 field assertions + CurrentConfigVersion==2
#   TestConfig_V2TOMLTags ................... config_version/max_commits/binary_extensions present;
#                                               commits/single/roles do NOT leak (toml:"-" honored)
#   TestTOMLMarshalKeysAndNoColorExclusion .. STILL green (adding config_version/max_commits/binary_extensions
#                                               to the marshal output does not break its presence/no_color checks)
# If TestConfig_V2TOMLTags fails on the leak assertion, a toml tag is wrong (a runtime field got a real tag,
# or Roles/Commits/Single lost their toml:"-"). If it fails on presence, a file key got toml:"-".
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...     # Expect clean (the new fields are consumed by nothing yet — pure additive).
go test ./...      # Expect all PASS. (pkg/stagecoach's keyed Config literal + empty Config{} literals are
                   # unaffected — verified; no whole-Config DeepEqual exists anywhere.)
# Confirm the LEAVE files are byte-unchanged:
git diff --exit-code internal/config/file.go internal/config/load.go internal/config/git.go \
  internal/provider internal/git internal/prompt internal/generate internal/ui cmd pkg Makefile \
  go.mod go.sum PRD.md && echo "frozen files UNCHANGED (expected)"
# Straggler grep — confirm the 6 new fields exist ONLY in config.go (+ the test) and nowhere else yet:
grep -rn "MaxCommits\|ConfigVersion\|BinaryExtensions\|RoleConfig\|CurrentConfigVersion" --include=*.go \
  | grep -v "internal/config/config.go" | grep -v "internal/config/config_test.go"
# Expected: no output (no consumer references them yet — that is S2/P1.M3.T2/P2/P3/P4's job).
```

### Level 4: Schema-correctness reasoning (no runtime to start)

```bash
# S1 is pure schema — no server/DB/subprocess. The "integration" is the field/tag contract. Verify by
# reasoning + the tests:
#   1. Defaults() v2 values match the contract (TestDefaults): MaxCommits==12, ConfigVersion==2, Commits==0,
#      Single==false, BinaryExtensions==nil, Roles==nil.
#   2. The tag split is proven (TestConfig_V2TOMLTags): file keys marshal; runtime fields never leak.
#   3. Leaf-package invariant: config imports only `time` (RoleConfig is plain, not Manifest).
#   4. Back-compat: empty/keyed Config{} literals still compile; existing Defaults() consumers unaffected.
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/config/` clean.
- [ ] `go test ./...` PASS (config suite incl. the new assertions + no repo-wide regression).
- [ ] go.mod/go.sum byte-unchanged; config.go imports ONLY `time`.
- [ ] file.go/load.go/git.go (+tests) + every non-config file byte-unchanged; PRD.md byte-unchanged.

### Feature Validation
- [ ] `RoleConfig` defined (plain struct, toml:"provider"/toml:"model") + doc comment.
- [ ] `const CurrentConfigVersion = 2` (UNtyped) + doc comment.
- [ ] Config has the six new fields with EXACT tags + grouped placement (existing fields unchanged).
- [ ] `Defaults()` sets Commits:0, Single:false, MaxCommits:12, BinaryExtensions:nil, Roles:nil,
      ConfigVersion:CurrentConfigVersion (existing values unchanged).
- [ ] `TestDefaults` asserts all six + CurrentConfigVersion==2; `TestConfig_V2TOMLTags` proves the tag split.

### Code Quality Validation
- [ ] Follows existing conventions: section-grouped struct fields, inline doc comments citing FRs, Defaults()
      lists every field explicitly, white-box `package config` tests mirroring the NoColor leak-check pattern.
- [ ] Config struct doc comment expanded (Mode A) preserving the "flat + plain-typed + RESOLVED ... NOT
      unmarshaled directly" invariant.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (LEAVE files untouched).

### Documentation
- [ ] Config struct doc comment documents the v2 fields + RoleConfig + CurrentConfigVersion + which later
      subtask wires each. Defaults() doc comment names the v2 defaults.
- [ ] No new user-facing docs in S1 (PRD "DOCS: [Mode A] Update the Config struct doc comment" — internal).

---

## Anti-Patterns to Avoid

- ❌ **Don't touch file.go/load.go/git.go.** S1 is config.go (+ config_test.go) ONLY. The file→Config plumbing
  (materialize/overlay/fileRoleConfig) is S2; env/flags/ResolveRoleModel is P1.M3.T2. S1 DECLARES the fields;
  it does not populate them. (research design-decisions §0)
- ❌ **Don't get the toml tags wrong.** config_version/max_commits/binary_extensions are real file keys
  (snake_case tags); Roles/Commits/Single are `toml:"-"`. A wrong tag leaks a runtime flag into config files
  (silent misconfiguration) or hides a file key (silent data loss). The V2TOMLTags test exists to catch this.
  (research design-decisions §1/§2, toml-tag-probe.md)
- ❌ **Don't type `CurrentConfigVersion`.** It's `const CurrentConfigVersion = 2` (untyped), not
  `const CurrentConfigVersion int = 2`. Untyped is idiomatic and assigns cleanly to the int field. (§5)
- ❌ **Don't import internal/provider for RoleConfig.** RoleConfig is a plain {Provider, Model} struct (NOT
  Manifest) so config stays a leaf. Mirror the Providers raw-map decoupling. (design-decisions §2)
- ❌ **Don't reorder the existing Config fields.** New fields slot INTO the existing section groups (CLI-UI /
  [generation] / after-Providers / trailing-version); existing fields keep their exact order for a clean diff.
- ❌ **Don't add a whole-Config `reflect.DeepEqual` test.** None exists today (verified); adding fields would
  make one brittle. Use per-field assertions (the existing TestDefaults style). (design-decisions §6)
- ❌ **Don't omit the new fields from Defaults().** List all six explicitly (MaxCommits:12, ConfigVersion:
  CurrentConfigVersion, etc.) — the existing style lists every field, and explicit nil/0/false documents
  intent (these are NOT accidental zero values). (design-decisions §4)
- ❌ **Don't implement config_v2_delta.md §2–§5.** Those are file.go/load.go/bootstrap (S2, P1.M3.T2, P1.M4).
  S1 implements §1 (the struct changes) ONLY.
- ❌ **Don't change `TestTOMLMarshalKeysAndNoColorExclusion`'s existing assertions.** Adding
  config_version/max_commits/binary_extensions to the marshal output does not break them (verified); leave
  that test as-is and ADD TestConfig_V2TOMLTags for the v2 tag behavior.

---

## Confidence Score

**9/10** — A small, purely-additive schema change (one type, one const, six struct fields, six Defaults
entries, a doc-comment expansion, and two test additions) with the exact field names/types/tags/defaults
pinned verbatim by `architecture/config_v2_delta.md §1` (the authoritative source, matching the item
contract) and by the PRD §16.2/§16.4/§9.17 citations. The one genuine trap — the three-way toml tag split
(file keys vs `toml:"-"` loader map vs `toml:"-"` runtime flags) — is pinned by a dedicated
`TestConfig_V2TOMLTags` and empirically verified in `toml-tag-probe.md`, and the back-compat risks (existing
`Config{}` literals, the marshal-key test) are explicitly checked off (empty/keyed literals tolerate added
fields; the marshal test checks presence + no_color-absence only). The -1 reserves for human error on an
inline toml tag typo, which the V2TOMLTags leak assertion catches immediately.
