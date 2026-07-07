# Research Note — P1.M1.T4.S2: TOML file loading

> Companion to `../PRP.md`. Captures the reasoning behind the one significant deviation from the
> item-contract signatures, plus the evidence trail. Read this if you wonder why `loadTOML` returns
> `*fileConfig` instead of `*Config`.

## 1. The core tension

The item contract sketches:

```
loadTOML(path string) (*Config, error)          // "toml.Unmarshal into Config"
overlay(dst, src *Config)                       // "merges non-zero fields"
```

But **P1.M1.T4.S1 shipped `config.Config` as flat + plain-typed + frozen** (`internal/config/config.go`,
verified in-repo): 12 scalar fields, `Timeout time.Duration`, NO `[defaults]`/`[generation]`
subtables, NO provider map, `NoColor toml:"-"`. S1's PRP explicitly states this struct is *"NOT
unmarshaled directly from the §16.2 file"* and that *"the loaders (S2-S4) decode into their own
intermediate structs (pointer or map-based — see arch §5.4) and merge field-by-field INTO this plain
Config."*

So the contract sketch was written against the **arch doc §2.2 NESTED `Config{Defaults, Provider map,
Generation}` with `Timeout string`** — a shape S1 deliberately did NOT ship. Two consequences:

1. **`Config` cannot be the decode target.** The §16.2 file has `[defaults]`/`[generation]` sections
   and `timeout = "120s"` (a string). go-toml/v2 looks for keys at the table path named in the tag,
   so a flat `Provider string toml:"provider"` would NOT pick up `[defaults].provider`; and even if
   it did, the string `"120s"` cannot unmarshal into `time.Duration` (no `TextUnmarshaler` for
   `Duration`). Verified against go-toml/v2 semantics + S1's gotchas.

2. **A flat `*Config` cannot carry "field present" info.** This is FINDING 5
   (`architecture/critical_findings.md`): go-toml/v2 has no `omitempty`, so for plain types you
   *"cannot distinguish 'field not present in TOML' from 'field = zero value'… Use pointer types
   (`*bool` for `auto_stage_all`, etc.) or decode to `map[string]any` first."*

## 2. Why a non-zero `overlay(*Config, *Config)` is an FR34 bug (not a harmless simplification)

Consider `Defaults()`: `AutoStageAll == true`, `StripCodeFence == true`, `MaxDuplicateRetries == 3`.
A user's §16.2 file can legitimately set any of these to its **zero value**:

| Field              | Default | User file value | Non-zero overlay result | FR34 (file > defaults) |
|--------------------|---------|-----------------|-------------------------|------------------------|
| `auto_stage_all`   | `true`  | `false`         | `false` skipped → stays `true` | ❌ VIOLATED |
| `strip_code_fence` | `true`  | `false`         | `false` skipped → stays `true` | ❌ VIOLATED |
| `max_duplicate_retries` | `3` | `0`           | `0` skipped → stays `3`        | ❌ VIOLATED |

FR34 (PRD §9.8, P0) requires the file to beat built-in defaults. A faithful implementation of the
contract's "non-zero fields" overlay on a flat `*Config` silently ignores these three overrides. The
higher presence-aware layers (env: `STAGECOACH_AUTO_STAGE_ALL=false`; CLI flag) would still work, but
the **file layer specifically** would be broken — and that is exactly what S2 owns.

## 3. The resolution (what the PRP specifies)

Introduce an **unexported, nested, pointer-bearing `fileConfig`** as the sole decode target, and make
`overlay` consume it:

```go
loadTOML(path string) (*fileConfig, error)   // presence-preserving decode; (nil,nil) if missing
overlay(dst *Config, src *fileConfig)         // merges PRESENT (non-nil) fields into resolved Config
```

- Pointer fields (`*string`/`*bool`/`*int`) make "key present" (even at `false`/`0`/`""`) distinct
  from "key absent". `overlay` checks `!= nil`, not non-zero. **A present `false` now correctly
  overrides the default `true`** (TestOverlay_PresentFalseOverridesDefault).
- `Timeout` is `*string`; `loadTOML` validates it (`time.ParseDuration`, fail-fast), `overlay` parses
  the validated string.
- The deviation is **fully internal**: `fileConfig` is unexported; the `Load()` orchestrator (S4)
  lives in the same package (`internal/config`) and sees it directly. The only outward contract
  (S4 → consumers) remains the resolved `config.Config`. No public API changes.

This is the design the arch doc §5.4 Option A and FINDING 5 both prescribe for exactly this layer,
and the one S1's PRP explicitly deferred to "the MERGE/overlay layer (S2–S4)."

## 4. The provider-map scoping decision

The contract says overlay should "merge the Provider map key-by-key." But S1 froze `Config` with
**NO provider map** ("provider manifests are P1.M2.T1 — out of scope"), and the typed `Manifest`
struct does not exist yet (P1.M2.T1 is PLANNED). So:

- `[provider.X]` is decoded into `fileConfig.Provider map[string]map[string]any` (generic; arch §5.5
  shows nested tables decode into maps natively) — nothing is lost.
- `overlay` merges **only** `[defaults]` + `[generation]` scalars into `Config` (no provider field).
- A generic `mergeProviderMaps(dst, src)` helper (key-by-key, deep field-merge, src wins) is provided
  + tested, honoring the contract's "merge key-by-key" as a utility for combining global + repo
  provider overrides.
- The typed manifest-vs-builtin FIELD merge (PRD §16.1: "a user override that sets only
  `default_model` leaves all other fields intact") is **M2.T3's** job (it owns `Manifest`).

The contract's stated tests ("parse valid config, missing file, merge partial override") do **not**
exercise provider-map-into-Config merging, which corroborates treating it as a carried/deferred
concern rather than a Config field.

## 5. Path correction (arch doc is stale)

PRD §16.1 Layer 4 + the item contract both say the repo-local file is **`./.stagecoach.toml`**. The
arch doc §2.1/§2.8 says `./.stagecoach/config.toml` — that is a stale error in the arch example
(written for the nested-Config sketch). PRD + item contract win → `repoLocalConfigPath()` returns
`".stagecoach.toml"`. Global: `$XDG_CONFIG_HOME/stagecoach/config.toml`, default
`~/.config/stagecoach/config.toml` (empty/unset XDG → home fallback).

## 6. Confidence & evidence

- `internal/config/config.go` read in-repo → confirms S1's flat Config exactly as S1's PRP specifies.
- `go.mod` read in-repo → `require github.com/pelletier/go-toml/v2 v2.4.2` already present (S1); no
  dep addition needed in S2.
- `architecture/critical_findings.md` FINDING 5 + `go_ecosystem_patterns.md` §2.4/§5.4/§5.5/§5.8 read
  → confirm pointer-presence pattern, overlay shape, map decoding, deep-merge shape.
- `internal/git/git_test.go` read → confirms test conventions (white-box, `t.TempDir`, `t.Setenv`,
  `t.Errorf`, no table boilerplate).
- PRD §16.1/§16.2 + §9.8 (FR34) read → confirm layer order, repo filename, provider field-merge rule.

**One-pass-success confidence: 9/10.** The only residual risk is an implementer "correcting" the
`*fileConfig` signatures back to `*Config` to match the contract sketch — the PRP's design-call #3
and the Anti-Patterns section exist specifically to prevent that.
