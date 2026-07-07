# P1.M3.T1.S2 — Design Decisions ([role.*] decode structs + materialize + overlay)

Ground truth read before writing this note:
- **PRD §16.4** (per-role config — h3.68), **§16.2** (full config example — h3.66, the file keys), **§16.1**
  (resolution order + "config_version is metadata, not a precedence layer" — h3.65), **§9.15** FR-R1–R5b
  (h3.31), **§9.17** FR-B4 (config versioning — h3.33), **§9.14** FR-M4 (max_commits — h3.30), **§9.1** FR3a
  (binary_extensions — h3.17). All in-context as `selected_prd_content`.
- **plan/002_a17bb6c8dc1d/architecture/config_v2_delta.md §2** (the AUTHORITATIVE file.go spec — the
  decode structs, materialize() updates, overlay() updates). §2 is the source of truth for this subtask.
- **The ACTUAL `internal/config/config.go` on disk** — S1 is SHIPPED (staged): `RoleConfig{Provider,Model}`,
  `const CurrentConfigVersion = 2`, and the six new `Config` fields (`Roles toml:"-"`, `ConfigVersion
  toml:"config_version"`, `MaxCommits toml:"max_commits"`, `BinaryExtensions toml:"binary_extensions"`,
  `Commits toml:"-"`, `Single toml:"-"`) all exist. S2 CONSUMES these.
- **The ACTUAL `internal/config/file.go` on disk** — the file S2 modifies: `fileConfig`, `fileDefaults`,
  `fileGeneration`, `materialize()`, `overlay()` (with the per-`Provider` field-merge — the pattern to
  mirror for `Roles`).
- **`internal/config/load.go`** — `Load()` = `cfg := Defaults()` then `overlay(&cfg, layer)` per layer.
  Confirms the overlay is onto a Defaults() baseline (ConfigVersion=2, MaxCommits=12).
- **`internal/config/file_test.go`** — the test style (white-box `package config`, `writeTempTOML` helper,
  `TestOverlayPartial`, `TestOverlayProvidersFieldMerge` — the field-merge test to MIRROR for Roles).
- **`internal/cmd/config.go`** — `exampleConfigTemplate` (the inert commented template; the DOCS Mode A
  target). Verified: `config_test.go:136` asserts the written file EQUALS the constant, so editing the
  constant stays green.
- Verified at research time: `go build ./...` ✓, `go test ./internal/config/` ✓ (S1 green).

The item contract is binding. S2 touches THREE files: `internal/config/file.go` (decode structs +
materialize + overlay), `internal/config/file_test.go` (tests), and `internal/cmd/config.go`
(exampleConfigTemplate Mode-A docs). It touches NEITHER `config.go` (S1, done) NOR `load.go` (P1.M3.T2).

---

## §0 — Scope: file.go (decode + materialize + overlay) + file_test.go + internal/cmd/config.go

**Decision:** S2 modifies `file.go` (the file→Config plumbing S1 deferred), adds tests to `file_test.go`,
and adds Mode-A reference sections to `exampleConfigTemplate` in `internal/cmd/config.go`.

**Frozen (do NOT touch):**
- `config.go` — S1 SHIPPED the `Config` fields/types/const/Defaults. S2 only WRITES INTO them.
- `load.go` — P1.M3.T2 owns env/flags (`STAGECOACH_<ROLE>_*`, `--commits`/`--single`/`--<role>-*`) +
  `ResolveRoleModel`. S2 does not touch the resolver; it only makes the FILE layer populate the fields.
- `git.go`, provider/*, prompt/*, generate/*, ui/*, pkg/*, root.go.

**Why:** the item contract is explicit ("In file.go: add ... ; In materialize(): ... ; In overlay(): ... ;
DOCS: ... exampleConfigTemplate in internal/cmd/config.go"). The v2 Config fields exist (S1) but are
zero/nil because nothing decodes/merges them yet. S2 is that plumbing.

---

## §1 — fileConfig: add `ConfigVersion` (top-level) + `Role` map; match delta §2 ordering

**Decision:** match the delta §2 target `fileConfig` exactly:
```go
type fileConfig struct {
	ConfigVersion int                       `toml:"config_version"` // V2 — top-level metadata key
	Defaults      fileDefaults              `toml:"defaults"`
	Generation    fileGeneration            `toml:"generation"`
	Role          map[string]fileRoleConfig `toml:"role"`  // V2 — [role.<role>] subtables
	Provider      map[string]map[string]any `toml:"provider"`
}
```
`ConfigVersion` first (top-level metadata key, mirrors §16.2 where it is a top-level key); `Role` between
`Generation` and `Provider` (groups the typed subtables). go-toml matches by tag, so order is functional-
neutral — but matching the authoritative delta §2 makes the struct self-documenting and reviewable.

**`[role.<role>]` decodes into `map[string]fileRoleConfig`** EXACTLY as `[provider.<name>]` decodes into
`map[string]map[string]any` today: TOML `[role.planner]` → go-toml → `fc.Role["planner"]`. Confirmed by
the existing Provider decode (same nested-map pattern). No special handling needed.

---

## §2 — fileRoleConfig: a plain {Provider, Model} decode struct (mirrors RoleConfig)

**Decision:**
```go
type fileRoleConfig struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
}
```
This is the FILE-side decode twin of `config.RoleConfig` (S1). It exists because `Config.Roles` is
`toml:"-"` (S1 design-decisions §2: a loader-populated map NEVER directly TOML-decoded on Config, mirroring
`Providers`). The `[role.*]` FILE tables decode into `fileRoleConfig` here, then `materialize` converts to
the typed `RoleConfig`. The two structs are intentionally separate (decode shape vs resolved shape) — the
same split as `fileGeneration.Output string` → `Config.Output *string`.

---

## §3 — fileGeneration: add `MaxCommits` + `BinaryExtensions`

**Decision:** append two fields (delta §2):
```go
MaxCommits       int      `toml:"max_commits"`       // V2 — safety cap on auto-decompose (FR-M4)
BinaryExtensions []string `toml:"binary_extensions"` // V2 — extra non-text exts to filter (FR3a)
```
They join the existing `[generation]` scalars. Place after `StripCodeFence` (the current last field) for a
clean append (existing fields keep their order).

---

## §4 — materialize(): copy the 4 new values (non-zero / non-empty semantics, matching every existing field)

**Decision:** add four blocks to `materialize()`, each following the EXISTING per-field non-zero pattern:

```go
// V2 [generation] scalars (non-zero wins, like every existing materialize field):
if g.MaxCommits != 0 {
	c.MaxCommits = g.MaxCommits
}
if len(g.BinaryExtensions) > 0 {
	c.BinaryExtensions = g.BinaryExtensions
}
// V2 top-level metadata (non-zero wins — see §5 for why this is correct + the advisory subtlety):
if fc.ConfigVersion != 0 {
	c.ConfigVersion = fc.ConfigVersion
}
// V2 per-role table: convert fileRoleConfig → RoleConfig, copying every present role.
if len(fc.Role) > 0 {
	c.Roles = make(map[string]RoleConfig, len(fc.Role))
	for role, frc := range fc.Role {
		c.Roles[role] = RoleConfig{Provider: frc.Provider, Model: frc.Model}
	}
}
```

**Why non-zero for ConfigVersion:** UNIFORMITY with the existing materialize (every field is a non-zero
copy). It is functionally identical to an unconditional copy here, because `overlay` also skips zero (§5) —
a file omitting `config_version` produces `fc.ConfigVersion == 0`, which materialize leaves at the fresh
`Config`'s zero, which overlay then skips, so `Defaults().ConfigVersion (2)` survives. Keep materialize
uniform; do not special-case ConfigVersion.

**Why copy ALL roles (no empty-role filtering):** mirrors how `Providers` materializes (`c.Providers =
fc.Provider` — copies the whole map including sparse entries). An all-empty `[role.X]` table →
`RoleConfig{"",""}` = "inherit global" (FR-R2), which is harmless and self-documenting. Filtering would add
complexity for no behavioral gain.

---

## §5 — overlay(): Roles FIELD-MERGE (mirror Providers) + 3 scalar non-zero-wins

**Decision:** add to `overlay()`, after the existing `[provider.X]` field-merge block:

```go
// V2 [role.<role>] — per-role field-merge across config layers (PRD §16.4 FR-R3). A field a higher layer
// sets overrides that one field only; fields the higher layer omits survive from lower layers. Mirrors the
// [provider.X] field-merge above (PRD §9.8 FR37a).
if len(src.Roles) > 0 {
	if dst.Roles == nil {
		dst.Roles = make(map[string]RoleConfig, len(src.Roles))
	}
	for role, rc := range src.Roles {
		existing := dst.Roles[role]
		if rc.Provider != "" {
			existing.Provider = rc.Provider
		}
		if rc.Model != "" {
			existing.Model = rc.Model
		}
		dst.Roles[role] = existing
	}
}
// V2 scalars — non-zero wins (matching every existing overlay field).
if src.ConfigVersion != 0 {
	dst.ConfigVersion = src.ConfigVersion
}
if src.MaxCommits != 0 {
	dst.MaxCommits = src.MaxCommits
}
if len(src.BinaryExtensions) > 0 {
	dst.BinaryExtensions = src.BinaryExtensions
}
```

**Roles field-merge is the load-bearing correctness property (FR-R3 / mirror of FR37a).** A repo
`[role.planner] model = "X"` must NOT erase a global `[role.planner] provider = "agy"`. The per-field
`existing := dst.Roles[role]; if rc.X != "" { existing.X = rc.X }; dst.Roles[role] = existing` loop is the
exact analog of the `[provider.X]` field loop. (Test it the same way `TestOverlayProvidersFieldMerge` tests
the provider case: lower-layer field survives a higher-layer partial.)

---

## §6 — ConfigVersion overlay is non-zero-wins; the "missing version" advisory is NOT S2's job

**Subtlety (document, do not "fix"):** `Defaults()` pins `ConfigVersion = CurrentConfigVersion = 2`, and
`overlay` skips zero. So a config file that OMITS `config_version` leaves the RESOLVED
`cfg.ConfigVersion == 2` (== CurrentConfigVersion) — indistinguishable from a file that explicitly sets
`config_version = 2`. Therefore the §9.17 FR-B4 advisory ("file's version is missing/older → warn") CANNOT
be implemented by comparing only the resolved `cfg.ConfigVersion` to `CurrentConfigVersion`; it must inspect
the RAW file's `config_version` presence (P1.M4.T1.S1's job, at the `loadTOML`/materialize seam).

**S2's responsibility is ONLY to plumb `config_version` through materialize+overlay with non-zero-wins** so
that an EXPLICIT version (e.g. an older `config_version = 1` or a newer `= 3`) DOES propagate to the
resolved Config for whatever advisory logic later reads it. Do NOT implement the advisory warning here; do
NOT special-case "missing == warn". (This is why non-zero-wins is correct and sufficient for S2.)

---

## §7 — BinaryExtensions overlay is REPLACE (non-empty wins), NOT append/merge

**Decision:** `if len(src.BinaryExtensions) > 0 { dst.BinaryExtensions = src.BinaryExtensions }` — a higher
config layer's `binary_extensions` REPLACES the lower layer's list, it does not concatenate.

**Why replace, not append:** (a) the item contract + delta §2 both specify non-zero/non-empty-wins (= replace);
(b) PRD §16.2's comment "merges with built-in denylist" refers to the RUNTIME merge in P2.M1
(`internal/git/binary.go` appends the user's extensions to the BUILT-IN denylist), NOT to cross-layer config
merging. Cross-layer, the highest layer's list wins outright (consistent with how every other overlay field
is a whole-value replace). Append-across-layers would surprise users (a repo list + a global list silently
concatenated) and is not what the contract says. The denylist merge is P2.M1's concern; S2 just carries the
list with standard overlay semantics.

---

## §8 — exampleConfigTemplate (Mode A docs): add commented `[role.*]` + `[generation]` lines

**Decision:** append two things to `exampleConfigTemplate` in `internal/cmd/config.go`, ALL COMMENTED OUT
(the template is INERT by design — every option line is `#`):

1. **In the existing `[generation]` section**, after the `strip_code_fence` line, add:
   ```
   # max_commits         = 12      # safety cap on auto-decompose (PRD §9.14 FR-M4); default 12
   # binary_extensions   = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
   ```

2. **A new `[role.<role>]` section** (after the `[provider.<name>]` section), modeled on PRD §16.4, showing
   all four roles with a header explaining FR-R1–R4:
   ```
   # ---------------------------------------------------------------------------
   # [role.<role>] — per-role provider/model overrides (PRD §16.4, §9.15 FR-R1–R5)
   # ---------------------------------------------------------------------------
   # The four agent roles — planner, stager, message, arbiter — each resolve their
   # provider/model independently. A single [defaults] covers all roles; a [role.*]
   # table overrides it for the roles you care about. Both fields "" -> inherit [defaults].
   # Precedence (highest wins): flag > env > [role.*] config > [defaults] > manifest default.
   #
   # [role.planner]
   # provider = "agy"
   # model    = "gemini-2.5-pro"
   #
   # [role.stager]
   # provider = "agy"
   # model    = "gemini-2.5-flash"
   #
   # [role.message]
   # provider = ""   # "" -> inherit [defaults].provider
   # model    = ""   # "" -> inherit [defaults].model
   #
   # [role.arbiter]
   # provider = ""
   # model    = ""
   ```

**Gotcha (test safety):** `internal/cmd/config_test.go:136` asserts the file written by `config init` EQUALS
`exampleConfigTemplate` (`got != exampleConfigTemplate`). Editing the CONSTANT keeps that test green (it
compares against the same constant). Do NOT add a test that pins specific template content beyond
"all-lines-commented" — the template will be REWRITTEN to a populated bootstrap by P1.M4.T2.S1 (FR-B1), so
S2's additions are intentionally provisional reference text. Keep every added line `#`-commented so the
template stays INERT (a future P1.M4.T2 test may assert "no uncommented key" — don't break that invariant).

---

## §9 — No new imports; go.mod UNCHANGED

**Decision:** S2 adds NO import anywhere. `file.go` already imports `fmt`/`io`/`os`/`path/filepath`/`time`/
`go-toml/v2`; the new fields/types use only same-package types (`RoleConfig`, `fileRoleConfig`) and plain Go
(map iteration, struct conversion). `internal/cmd/config.go` already imports `config`; the template edit is a
string change only. `go mod tidy` MUST be a no-op; `git diff --exit-code go.mod go.sum` MUST be empty.

---

## §10 — Test strategy (mirror the existing field-merge + partial-merge tests)

**Decision:** add to `file_test.go` (`package config` white-box), mirroring the EXISTING test idioms:

1. **`TestLoadTOML_V2Fields`** — decode a TOML carrying `config_version`, `[generation] max_commits`/
   `binary_extensions`, and `[role.planner]`/`[role.stager]` tables; assert materialize populates
   `cfg.ConfigVersion`, `cfg.MaxCommits`, `cfg.BinaryExtensions`, and `cfg.Roles` (incl. a partial role
   whose Provider is "" — proves the field-level decode, not a whole-block). Mirrors `TestLoadTOMLValid`.
2. **`TestOverlayRolesFieldMerge`** — MIRROR `TestOverlayProvidersFieldMerge`: dst has
   `Roles["planner"]={provider:"pi",model:"A"}`; src sets `Roles["planner"]={model:"B"}`; after overlay,
   `planner.provider == "pi"` SURVIVES and `planner.model == "B"` is overridden; a src-only role is ADDED;
   an untouched dst role is preserved. (This is the FR-R3 regression guard, exactly as the provider test is
   the FR37a guard.)
3. **`TestOverlay_V2Scalars`** — dst=Defaults() (MaxCommits=12, ConfigVersion=2, BinaryExtensions=nil);
   (a) src sets all three → dst overridden to src's values; (b) src omits them (zero/nil) → dst PRESERVES
   12/2/nil (partial-merge correctness for the new scalars, mirroring `TestOverlayPartial`).

Pure unit tests: no subprocess, no git. `writeTempTOML` helper already exists for the decode test.

---

## Summary table (the 10 calls at a glance)

| § | Decision | Source |
|---|----------|--------|
| 0 | file.go + file_test.go + internal/cmd/config.go ONLY; config.go (S1) + load.go (P1.M3.T2) frozen | item contract |
| 1 | fileConfig += ConfigVersion (first) + Role map (after Generation); match delta §2 | delta §2 |
| 2 | fileRoleConfig = plain {Provider, Model} (decode twin of RoleConfig) | delta §2, S1 |
| 3 | fileGeneration += MaxCommits + BinaryExtensions (append after StripCodeFence) | delta §2 |
| 4 | materialize: 4 blocks, non-zero/non-empty (uniform w/ existing fields); copy all roles | delta §2, file.go pattern |
| 5 | overlay: Roles FIELD-MERGE (mirror Providers) + 3 scalar non-zero-wins | delta §2, FR-R3/FR37a |
| 6 | ConfigVersion non-zero-wins; "missing version" advisory is P1.M4.T1 (file-level), NOT S2 | §16.1, FR-B4 |
| 7 | BinaryExtensions overlay = REPLACE (runtime denylist merge is P2.M1) | contract, §16.2, FR3a |
| 8 | exampleConfigTemplate += commented [generation] lines + commented [role.*] section | item DOCS, §16.4 |
| 9 | NO new imports; go.mod UNCHANGED | file.go/cmd.go imports |
| 10 | tests mirror TestOverlayProvidersFieldMerge + TestOverlayPartial + TestLoadTOMLValid | file_test.go |
