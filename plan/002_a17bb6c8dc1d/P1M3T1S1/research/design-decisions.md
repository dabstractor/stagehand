# Design Decisions — P1.M3.T1.S1 (RoleConfig + v2 Config fields + Defaults)

> The authoritative companion to `toml-tag-probe.md` (the empirical grounding) and `architecture/
> config_v2_delta.md §1` (the spec). Each § is a non-obvious call an implementer could get wrong; each cites
> its evidence. Numbered for cross-reference from the PRP's gotchas/tasks.

## §0 — Scope boundary: S1 is `config.go` ONLY (+ `config_test.go`)

- **S1 DECLARES the schema; it does not POPULATE it.** S1 adds: `type RoleConfig`, `const
  CurrentConfigVersion`, six `Config` fields, the `Defaults()` values, the doc comment, and the test
  assertions. That is the ENTIRE deliverable.
- **S2 (P1.M3.T1.S2) owns `file.go`:** the `fileRoleConfig` decode struct, the extended `fileGeneration`
  (+`max_commits`, +`binary_extensions`), `fileConfig.ConfigVersion`, and the `materialize()`/`overlay()`
  wiring that copies file values into the new `Config` fields. S1 only creates the TARGET fields S2 writes.
- **P1.M3.T2 owns `load.go`:** `STAGECOACH_<ROLE>_{PROVIDER,MODEL}` + `STAGECOACH_COMMITS` env,
  `--commits`/`--single`/`--max-commits`/`--<role>-*` flags, and `ResolveRoleModel`. S1 only creates the
  fields these set (`cfg.Commits`, `cfg.Single`, `cfg.Roles`).
- **Why split it this way:** adding the fields first means every later subtask compiles against a stable
  `Config` shape, and S1 is a tiny, reviewable, behavior-free change (no loader, no consumer touched). The
  fields are zero/nil by default, so existing code is unaffected (the item contract's "existing code
  compiles; new fields are zero-valued").

## §1 — The three-way toml tag split (the heart of S1)

The six new fields use THREE distinct tag strategies — getting this right IS the task:

| Class | Fields | Tag | Why |
|---|---|---|---|
| **(a) real §16.2 file keys** | `ConfigVersion`, `MaxCommits`, `BinaryExtensions` | `toml:"config_version"` / `toml:"max_commits"` / `toml:"binary_extensions"` | These ARE config-file keys (§16.2 shows `binary_extensions`, `max_commits`; §9.17 FR-B4 shows `config_version`). They appear in marshaled TOML and (in S2) decode from files. snake_case per §16.2. |
| **(b) loader-populated map** | `Roles` | `toml:"-"` | Mirrors `Providers map[string]map[string]any toml:"-"`. The `[role.<role>]` FILE tables decode into `fileConfig`'s map (S2) and materialize into this TYPED map — Config is NEVER directly TOML-decoded (the struct doc-comment invariant). `toml:"-"` excludes it from marshal/unmarshal on Config. |
| **(c) CLI/runtime-only** | `Commits`, `Single` | `toml:"-"` | Mirrors `NoColor toml:"-"`. Set by `--commits`/`--single` flags (P1.M3.T2/P4.M1.T1) at runtime; they MUST NEVER appear in a config file (a file cannot set a behavioral mode flag). |

A wrong tag is a silent bug: a real tag on `Commits` would leak a runtime flag into written configs
(misconfiguration); `toml:"-"` on `MaxCommits` would silently drop the file key (data loss). `TestConfig_V2TOMLTags`
exists to lock this in. See `toml-tag-probe.md §1`.

## §2 — `RoleConfig` is a plain struct; `Roles toml:"-"` mirrors `Providers`

- `RoleConfig{Provider, Model}` is intentionally NOT the provider `Manifest` type. `internal/config` must
  NOT import `internal/provider` (import cycle: provider's registry consumes `config.Providers`). The same
  decoupling already exists for `Providers` (carried as a raw `map[string]map[string]any`); `Roles` carries
  a typed `map[string]RoleConfig` but the principle is identical — config stays a LEAF package.
- The `toml:"provider"`/`toml:"model"` tags on `RoleConfig` are **documentary only** — because
  `Config.Roles` is `toml:"-"`, go-toml never marshals/unmarshals `RoleConfig` during Config round-trips
  (verified in `toml-tag-probe.md §1`: `Roles` never appears in marshal output). The tags match
  `config_v2_delta.md §1` and will be used by S2's SEPARATE `fileRoleConfig` decode struct (which DOES decode
  from the `[role.<role>]` tables). Including them is harmless and keeps the type self-documenting.
- `Roles == nil` means "no per-role overrides → every role inherits the global `[defaults]`" (FR-R2). On the
  single-commit path the only active role is `message`, so nil Roles is exactly v1-equivalent (back-compatible).

## §3 — Field placement follows the existing section grouping (clean diff)

The existing `Config` struct is grouped: `[defaults]` → `CLI/UI only` → `[generation]` → `Providers`. The new
fields slot INTO these groups by semantics; existing fields keep their EXACT current order (so the diff is
clean and reviewable):

- **`Commits` + `Single`** → the `CLI / UI only` group, immediately after `NoColor` (all three are
  `toml:"-"` runtime fields — they belong together).
- **`MaxCommits` + `BinaryExtensions`** → the `[generation]` group, after `StripCodeFence` (per §16.2 both
  are `[generation]` keys).
- **`Roles`** → immediately after `Providers` (both `toml:"-"` loader maps).
- **`ConfigVersion`** → a trailing group of its own (it is top-level metadata, not under any `[section]`;
  §16.1: "config_version is metadata, not a precedence layer").

Each new field gets an inline doc comment citing its FR. The Config struct doc comment gains a "V2 FIELDS"
paragraph (Mode A) listing them + which later subtask wires each.

## §4 — `Defaults()` gains NON-ZERO values; the existing marshal test is NOT broken

`Defaults()` sets `MaxCommits: 12` and `ConfigVersion: CurrentConfigVersion` (2) — both NON-zero. This is
correct (the shipped defaults: FR-M4's safety cap is 12; a Defaults() Config is always current-schema). The
other four new fields are zero/nil (`Commits: 0`, `Single: false`, `BinaryExtensions: nil`, `Roles: nil`).

**Why this does not break `TestTOMLMarshalKeysAndNoColorExclusion`:** that test checks PRESENCE of a fixed
key list (`provider`, `model`, …, `output`, `strip_code_fence`) via `strings.Contains(s, key+" =")` and
ABSENCE of `no_color`. Adding `config_version = 2`, `max_commits = 12`, and `binary_extensions = []` to the
marshal output is purely ADDITIVE — the presence loop and the no_color-absence check are unaffected
(reproduced against the modified struct: PASS — see `toml-tag-probe.md §4`). **Do not modify that test; ADD
`TestConfig_V2TOMLTags`.**

Note: a nil `BinaryExtensions` marshals as `binary_extensions = []` (NOT omitted — `toml-tag-probe.md §2`),
so marshaled `Defaults()` now visibly carries `binary_extensions = []`. Harmless and semantically correct
("no extra extensions"). List all six new fields in `Defaults()` explicitly (the existing style lists every
field; explicit `nil`/`0`/`false` documents intent — these are NOT accidental zero values).

## §5 — `CurrentConfigVersion` is an UNTYPED const

```go
const CurrentConfigVersion = 2
```
Untyped (not `const CurrentConfigVersion int = 2`). Idiomatic for a version constant; it assigns cleanly to
the `int` field (`ConfigVersion: CurrentConfigVersion`) and stays comparable to any integer type if ever
needed. Bumped on any breaking config change (§9.17 FR-B4). v2 = per-role models + multi-commit decomposition
+ binary filtering.

## §6 — Test strategy: per-field assertions + line-based tag proof; NO whole-Config DeepEqual

- **Extend `TestDefaults`** with the six new field assertions (`Commits==0`, `Single==false`,
  `MaxCommits==12`, `BinaryExtensions==nil`, `Roles==nil`, `ConfigVersion==CurrentConfigVersion`) + a
  `CurrentConfigVersion==2` const check. Mirrors the existing per-field `t.Errorf` style.
- **ADD `TestConfig_V2TOMLTags`** with the `hasKeyLine` helper proving the §1 tag split: file keys
  (`config_version`/`max_commits`/`binary_extensions`) present when set; `toml:"-"` fields
  (`commits`/`single`/`roles`) never leak even when populated.
- **Use `hasKeyLine`, NOT `strings.Contains(out, key)` / `strings.Contains(out, key+" =")`.** `commits` is a
  SUFFIX of `max_commits`, so both substring forms FALSE-POSITIVE on the legit `max_commits = …` line — this
  exact bug broke two test drafts in temp-module validation (`toml-tag-probe.md §3`). `hasKeyLine` (a trimmed
  line starting with `key =`) is robust against any key collision.
- **NO whole-Config `reflect.DeepEqual`.** None exists today (grep-verified); adding fields would make one
  brittle and would couple the test to field ORDER. Per-field assertions are the established style and are
  robust to field additions/reordering.
- **Do NOT modify `TestTOMLMarshalKeysAndNoColorExclusion`** — it still holds (§4). Only ADD the new test.
- `config_test.go` is `package config` (white-box) — references `strPtr`/`boolPtr`/`RoleConfig`/
  `CurrentConfigVersion` directly, no extra imports (`strings`/`testing`/`time`/`go-toml/v2` already present).
