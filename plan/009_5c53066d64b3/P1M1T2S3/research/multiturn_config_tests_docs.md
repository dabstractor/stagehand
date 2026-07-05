# Research: Config unit tests + Mode A docs for multi-turn knobs (P1.M1.T2.S3)

> Pin the exact test design for the multi-turn config knobs (PRD §9.24 FR-T1c/FR-T3) through the
> file-decode layer, and the docs/configuration.md `[generation]` updates. Verification: live code
> read 2026-07-05.

## 1. Dependency state — S1 LANDED, S2 in progress (the code under test)

- **S1 (LANDED, on disk):** `Config.MultiTurnFallback bool` + `Config.MultiTurnChunkTokens int`
  (config.go), `Defaults()` sets `true`/`32000`, AND **`TestDefaults` already pins both** (config_test.go):
  ```go
  if !c.MultiTurnFallback { t.Errorf("MultiTurnFallback = false, want true (§9.24 FR-T1c)") }
  if c.MultiTurnChunkTokens != 32000 { t.Errorf("MultiTurnChunkTokens = %d, want 32000 (§9.24 FR-T3)", c.MultiTurnChunkTokens) }
  ```
  ⇒ **Contract case (a) "Defaults() returns true/32000" is ALREADY satisfied by S1.** S3 does NOT
  duplicate it (would be redundant + cross S1's territory). S3 owns cases (b)–(e): the FILE/OVERLAY tests.

- **S2 (in progress — the code under test):** adds `fileGeneration.MultiTurnFallback bool` +
  `fileGeneration.MultiTurnChunkTokens int` + the materialize guard clauses
  (`if g.MultiTurnFallback { c.MultiTurnFallback = true }` bool only-true-propagates;
  `if g.MultiTurnChunkTokens != 0 { c.MultiTurnChunkTokens = g.MultiTurnChunkTokens }` int `!= 0`) +
  the matching overlay `src→dst` clauses. **S3's tests assume S2 lands exactly as specified** — if
  S2 hasn't landed, `fileGeneration` has no such fields and the tests won't compile (clear signal).

## 2. The materialize-vs-overlay distinction (THE key test-design insight)

`materialize(fc, timeout)` is the file→Config copy. It does NOT seed Defaults — it creates a fresh
`Config` where omitted fields are **Go zero-values** (`false`/`0`/`nil`). Defaults are applied
separately by the precedence resolver via `Defaults() → overlay(file)`.

Verified against `TestMaterializeOverlay_DiffContext_TokenLimit` (file_test.go): `materialize(fc, 0)`
with an omitted key yields the zero-value (`c.DiffContext == nil`, `c.TokenLimit == 0`), NOT the default.

**Implication for the multi-turn knobs:**
- `materialize` level (file→Config):
  - `multi_turn_chunk_tokens` omitted → `c.MultiTurnChunkTokens == 0`
  - `multi_turn_chunk_tokens = 16000` → `c.MultiTurnChunkTokens == 16000` (int `!= 0` propagates)
  - `multi_turn_fallback` omitted → `c.MultiTurnFallback == false`
  - `multi_turn_fallback = true` → `c.MultiTurnFallback == true` (bool guard propagates)
  - `multi_turn_fallback = false` → `c.MultiTurnFallback == false` (**INDISTINGUISHABLE from omitted** —
    the root cause of the limitation)
- `overlay` level (Defaults → overlay(file)) — the resolved value the generate core reads:
  - omitted → Defaults win (`true`/`32000`)
  - `chunk = 16000` → `16000` (override honored end-to-end)
  - `fallback = true` → `true` (redundant with default, but honored)
  - **`fallback = false` → STILL `true`** (the ACCEPTED v1 limitation: overlay's `if src.MultiTurnFallback
    { dst.MultiTurnFallback = true }` can only set true, never false; mirrors `AutoStageAll`)

This is why contract case (d) — "multi_turn_fallback=false leaves the resolved value at the default
true" — is an **overlay-chain** assertion (`Defaults()` → `overlay`), not a materialize-only one.
The test pins this as KNOWN/tested behavior (the `t.Errorf` message says so explicitly).

## 3. The gold-standard test pattern to mirror

`TestMaterializeOverlay_DiffContext_TokenLimit` (file_test.go) is the exact template — a table-driven
test with three tiers:
1. **materialize-only** — `materialize(&fileConfig{Generation: fileGeneration{…}}, 0)`, assert zero-value
   semantics.
2. **overlay chain** — `cfg := Defaults()` → `overlay(&cfg, materialize(...))`, assert resolved values.
3. **end-to-end via loadTOML** — `loadTOML(writeTempTOML(t, body))` then `Defaults()` → `overlay`, proving
   the TOML decode → resolved value path.

Reusable helpers (all `package config` — do NOT redeclare):
- `writeTempTOML(t, body) string` (file_test.go) — writes TOML to t.TempDir(), returns path.
- `loadTOML(path) (*Config, error)` (file.go) — file → materialized Config (NO Defaults overlay).
- `materialize(fc *fileConfig, timeout time.Duration) *Config` (file.go) — file→Config copy.
- `overlay(dst, src *Config)` (file.go) — field-by-field merge.
- `Defaults() Config` (config.go) — the baseline.
- `fileConfig` / `fileGeneration` types (file.go).

S3's new file `internal/config/multiturn_test.go` (package config) mirrors this structure with a single
table-driven `TestMaterializeOverlay_MultiTurn` covering cases (b)–(e).

## 4. The docs/configuration.md edit sites (3 edits, all Mode A)

**Edit D1 — commented `[generation]` template block (lines 104–113).** Insert the two commented lines
between `diff_context` (line 108) and `exclude` (line 109):
```
# multi_turn_fallback     = true   # lossless multi-turn fallback on one-shot exhaustion (§9.24 FR-T1c); CANNOT disable via file (see note below)
# multi_turn_chunk_tokens = 32000  # per-turn chunk budget in tokens (§9.24 FR-T3); does NOT interact with token_limit (FR-T12)
```

**Edit D2 — "Built-in defaults" table (lines 132–135).** Insert two rows between `max_duplicate_retries`
(line 134) and `subject_target_chars` (line 135) — matches §16.2 ordering:
```
| `multi_turn_fallback` | `true` | `config.Defaults()` (§9.24 FR-T1c) |
| `multi_turn_chunk_tokens` | `32000` | `config.Defaults()` (§9.24 FR-T3) |
```

**Edit D3 — new "Multi-turn fallback" callout** immediately after the "Token budget & diff context"
callout (ends line 149). Carries the FR-T12 non-interaction note AND the honest false-limitation:
```
> **Multi-turn fallback.** Two `[generation]` knobs control the lossless multi-turn fallback path (§9.24)…
> - **`multi_turn_fallback`** (default `true`) … **Limitation:** … you cannot disable it by setting
>   `multi_turn_fallback = false` in a config file in this revision — the `false` is silently ignored
>   (only-true-propagates, mirrors `auto_stage_all`). To disable multi-turn for a provider, set
>   `session_mode = ""` on that provider (see providers.md).
> - **`multi_turn_chunk_tokens`** (default `32000`) … **does NOT interact with `token_limit`** (FR-T12):
>   multi-turn uses the UNTRUNCATED payload, delivered in request-sized pieces.
```

## 5. Decisions log

| # | Point | Decision | Why |
|---|---|---|---|
| D1 | New test file vs append to file_test.go | NEW `multiturn_test.go` | Keeps multi-turn tests isolated; file_test.go is already 32KB; matches the per-feature test-file convention. |
| D2 | Case (a) Defaults pin | DO NOT re-add — S1's TestDefaults already pins it | S1 LANDED the exact assertions; duplicating crosses S1's territory + is redundant. Cross-reference TestDefaults. |
| D3 | Mirror pattern | `TestMaterializeOverlay_DiffContext_TokenLimit` (3 tiers: materialize / overlay / loadTOML) | The in-file gold standard for file/overlay tests; same helpers, same idiom. |
| D4 | Case (d) the limitation | Assert as overlay-chain (`Defaults→overlay` stays `true`); t.Errorf names it "ACCEPTED v1 limitation" | Contract: "assert this explicitly so it is a known, tested behavior, not a silent bug." The materialize-level indistinguishability (false==omitted) is the root cause documented in the materialize tier. |
| D5 | Docs: FR-T12 + limitation placement | New "Multi-turn fallback" callout after "Token budget & diff context" | Keeps the multi-turn docs together; the FR-T12 non-interaction + the false-limitation are the two user-facing facts. |
| D6 | Test-only bool-sentinel: NO `*bool` fix | The limitation is ACCEPTED (mirrors AutoStageAll); docs surface it | S2's contract chose the plain-bool AutoStageAll pattern; S3 documents, does not fix. |
