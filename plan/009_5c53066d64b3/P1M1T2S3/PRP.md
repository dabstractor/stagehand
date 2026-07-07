---
name: "P1.M1.T2.S3 — Config unit tests + Mode A docs/configuration.md [generation] table"
description: |
  Land the dedicated file/overlay unit tests for the two multi-turn config knobs (PRD §9.24 FR-T1c/FR-T3)
  and the Mode A docs/configuration.md `[generation]` surface. TWO deliverables: (1) NEW
  `internal/config/multiturn_test.go` (package config) — a table-driven
  `TestMaterializeOverlay_MultiTurn` mirroring the gold-standard `TestMaterializeOverlay_DiffContext_TokenLimit`,
  covering contract cases (b) int-override honored, (c) bool=true honored, (d) bool=false silently ignored
  (the ACCEPTED v1 limitation, pinned as known/tested behavior), (e) repo-file overrides global-file for
  chunk tokens — across materialize / overlay / loadTOML tiers. Case (a) Defaults==true/32000 is ALREADY
  pinned by S1's TestDefaults (LANDED) — NOT duplicated. (2) MODIFY `docs/configuration.md` — 3 Mode A
  edits: the commented `[generation]` template block +2 lines, the "Built-in defaults" table +2 rows, and
  a new "Multi-turn fallback" callout carrying the FR-T12 non-interaction note + the honest
  false-limitation. Consumes S1 (LANDED: Config fields + Defaults + TestDefaults) + S2 (in progress:
  fileGeneration + materialize + overlay). TESTS-ONLY code changes (one new test file) + DOCS-ONLY prose.
---

## Goal

**Feature Goal**: Prove — with dedicated unit tests that the existing suite lacked — that a user's
`[generation]`-table `multi_turn_fallback` and `multi_turn_chunk_tokens` keys flow correctly through the
file-decode layer (`fileGeneration` → `materialize` → `overlay`) into the resolved `Config` the downstream
multi-turn generate core (P1.M1.T3.S2 reads `cfg.MultiTurnChunkTokens`; P1.M1.T3.S3 reads
`cfg.MultiTurnFallback`) consumes; AND document both knobs + the accepted `multi_turn_fallback=false`
limitation honestly in docs/configuration.md so users are not surprised by it.

**Deliverable**:
1. **CREATE** `internal/config/multiturn_test.go` (`package config`) — one table-driven test,
   `TestMaterializeOverlay_MultiTurn`, mirroring `TestMaterializeOverlay_DiffContext_TokenLimit`, with
   materialize-only / overlay-chain / loadTOML tiers covering contract cases (b)–(e).
2. **MODIFY** `docs/configuration.md` — 3 Mode A edits: (D1) commented `[generation]` template block +2
   lines; (D2) "Built-in defaults" table +2 rows; (D3) new "Multi-turn fallback" callout (FR-T12 note +
   the false-limitation).

No production code touched. Case (a) is NOT re-tested (S1's `TestDefaults` already pins it).

**Success Definition**: `go build/vet/gofmt` clean; `go test ./internal/config/...` green with the new
`TestMaterializeOverlay_MultiTurn` passing all subtests (including the case-(d) limitation assertion that
pins `multi_turn_fallback=false` ⇒ resolved `true`); `go test ./...` green repo-wide; the docs table has
both rows, the template block has both lines, and the callout surfaces the limitation + FR-T12.

## User Persona

**Target User**: (1) End users who tune multi-turn via the `[generation]` table in their config file (they
need the docs to know the knobs exist, their defaults, and the false-limitation); (2) the contributor
implementing P1.M1.T3 (the generate core reads the resolved fields — these tests prove the file path
reaches them correctly); (3) future maintainers (the case-(d) test documents the limitation as intentional).

**Use Case**: A user on a 200K-context model raises the per-request chunk size for fewer turns:
`[generation]\nmulti_turn_chunk_tokens = 48000`. The test proves that value reaches
`cfg.MultiTurnChunkTokens == 48000`. Another user tries `[generation]\nmulti_turn_fallback = false` to
disable the fallback — the test + docs make clear this is silently ignored in this revision (and the docs
point to `session_mode = ""` as the real disable lever).

**User Journey**: TOML `[generation]` table → `fileConfig.fileGeneration` (Decode) → `materialize`
(file→Config) → `overlay` (Defaults → file merge) → resolved `Config` → read by the generate core. The
tests cover each rung; the docs describe the knobs + the limitation.

**Pain Points Addressed**: Without these tests, a silent regression in S2's file-decode plumbing (e.g. a
dropped materialize clause, a `> 0` vs `!= 0` typo, a clobbered overlay) would ship undetected — the
existing suite only pins the Defaults (S1), not the file→Config path. Without the docs, the
`multi_turn_fallback=false` limitation is a silent footgun.

## Why

- **PRD §9.24 FR-T1c/FR-T3 mandate both knobs as `[generation]`-table keys**, and §16.1 lists both in the
  built-in-defaults layer. §16.2 shows them in the `[generation]` table right after `max_duplicate_retries`.
  This task proves the file-decode path and documents the table — closing the config-surface loop for the
  multi-turn feature.
- **PRD §9.24 FR-T12 is a user-facing fact** (`multi_turn_chunk_tokens` does NOT interact with
  `token_limit`; multi-turn uses the UNTRUNCATED payload). The docs callout surfaces it so users do not
  expect the two to compose.
- **The `multi_turn_fallback=false` limitation is ACCEPTED but must be honest.** S2's contract chose the
  plain-bool `AutoStageAll` pattern (only-true-propagates); a file cannot disable the default-true fallback.
  The case-(d) test pins this as KNOWN/tested behavior (not a silent bug), and the docs callout tells users
  how to actually disable it (`session_mode = ""` on the provider). This is the §20 testing discipline:
  "every bug found in the wild becomes a scenario here" — including known-limitation pinning.
- **Closes P1.M1.T2.** S1 landed the resolved Config + Defaults; S2 plumbed the file-decode layer; S3 (this)
  proves the plumbing end-to-end and documents it. P1.M1.T3 (the generate core) then has a verified,
  documented config surface to read.

## What

Two deliverables, no production-code change:

1. **`internal/config/multiturn_test.go`** — a single table-driven `TestMaterializeOverlay_MultiTurn` with
   three tiers (mirror `TestMaterializeOverlay_DiffContext_TokenLimit`):
   - **materialize-only** (file→Config): proves `multi_turn_chunk_tokens` set ⇒ propagated; omitted ⇒ Go
     zero-value `0`; `multi_turn_fallback=true` ⇒ `true`; `false` INDISTINGUISHABLE from omitted (both
     `false`) — the root cause of the limitation.
   - **overlay chain** (Defaults → overlay(file)): proves the resolved values the generate core reads —
     omitted ⇒ Defaults (`true`/`32000`); `chunk=16000` ⇒ `16000`; `chunk=48000` ⇒ `48000`; `fallback=true`
     ⇒ `true`; **`fallback=false` ⇒ STILL `true`** (case (d), the ACCEPTED limitation).
   - **repo-overrides-global** (case (e)): `Defaults` → overlay(global chunk=48000) → overlay(repo
     chunk=16000) ⇒ `16000` (higher layer wins).
   - **end-to-end via loadTOML**: a real TOML `[generation] multi_turn_chunk_tokens = 16000` ⇒ resolved
     `16000`; a real TOML `multi_turn_fallback = false` ⇒ resolved STILL `true` (the limitation, end-to-end).
2. **`docs/configuration.md`** — 3 edits (D1 template block, D2 defaults table, D3 callout).

Case (a) — `Defaults()` returns `MultiTurnFallback==true && MultiTurnChunkTokens==32000` — is **already
pinned by S1's `TestDefaults`** (LANDED on disk, verified). S3 does NOT duplicate it.

### Success Criteria

- [ ] `internal/config/multiturn_test.go` exists, `package config`, `//go:build` NOT needed (plain test).
- [ ] `TestMaterializeOverlay_MultiTurn` has materialize-only, overlay-chain, repo-overrides-global, and
      loadTOML-end-to-end tiers.
- [ ] Case (b): `[generation] multi_turn_chunk_tokens = 16000` ⇒ resolved `cfg.MultiTurnChunkTokens == 16000`.
- [ ] Case (c): `[generation] multi_turn_fallback = true` ⇒ resolved `cfg.MultiTurnFallback == true`.
- [ ] Case (d): `[generation] multi_turn_fallback = false` ⇒ resolved `cfg.MultiTurnFallback == true`
      (ACCEPTED limitation; the assertion message names it as known/tested behavior).
- [ ] Case (e): repo-file `multi_turn_chunk_tokens` overrides global-file value.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean.
- [ ] `go test ./...` green (the new test file is additive).
- [ ] `docs/configuration.md`: `[generation]` template block has both commented lines; defaults table has
      both rows; the "Multi-turn fallback" callout surfaces FR-T12 + the false-limitation.
- [ ] NO production code, `internal/config/config.go`/`file.go`/`config_test.go`, or other packages touched.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP names the gold-standard test to mirror (`TestMaterializeOverlay_DiffContext_TokenLimit`),
enumerates the exact reusable helpers (`writeTempTOML`/`loadTOML`/`materialize`/`overlay`/`Defaults`/
`fileConfig`/`fileGeneration`) with their packages, gives the complete test body (all tiers + assertions,
copy-paste-ready), quotes the verbatim current text at all 3 docs edit sites with the exact target for
each, and states the dependency on S1 (LANDED) + S2 (in progress). No inference required.

### Documentation & References

```yaml
# MUST READ — the FR specs + the config model + the gold-standard test
- file: PRD.md
  why: "§9.24 FR-T1c (multi_turn_fallback default true — condition (c) of the FR-T1 trigger) and FR-T3
        (multi_turn_chunk_tokens default 32000 — per-request chunk sizing). §9.24 FR-T12 (multi-turn does
        NOT interact with token_limit — uses the UNTRUNCATED payload). §16.1 (built-in defaults list both).
        §16.2 (the [generation] table shows both right after max_duplicate_retries — IS the table ordering)."
  critical: "FR-T1c default true + FR-T3 default 32000 are the values S1 already set (LANDED); this task
             proves the file path reaches them + documents the table. FR-T12 + the false-limitation are the
             two user-facing facts the docs callout must carry."

- docfile: plan/009_5c53066d64b3/architecture/research-generate-config.md
  why: "§3b is the authoritative spec for the bool-sentinel DESIGN TENSION: MultiTurnFallback (default true)
        + the only-true-propagates bool guard (mirrors AutoStageAll) ⇒ false-in-file silently ignored.
        Recommends (a) accept the limitation over (b) *bool. §3b/§3c give the materialize/overlay guard
        templates S2 implemented (and S3 tests)."
  critical: "§3b's DESIGN TENSION is exactly what case (d) pins as a known/tested behavior. The docs callout
             surfaces it. Do NOT 'fix' it with *bool (S2's contract chose AutoStageAll; S3 documents)."

- docfile: plan/009_5c53066d64b3/P1M1T2S1/PRP.md
  why: "The S1 CONTRACT (LANDED). Config.MultiTurnFallback bool + Config.MultiTurnChunkTokens int +
        Defaults true/32000. CRITICALLY: S1's TestDefaults ALREADY pins both defaults (the contract's
        case (a)) — S3 must NOT re-add case (a)."
  critical: "Treat S1 as LANDED. Case (a) is covered by TestDefaults. S3 owns cases (b)–(e) only (the
             file/overlay tests S1 explicitly fenced to S3)."

- docfile: plan/009_5c53066d64b3/P1M1T2S2/PRP.md
  why: "The S2 CONTRACT (in progress — the code under test). fileGeneration gains MultiTurnFallback bool +
        MultiTurnChunkTokens int; materialize gains `if g.MultiTurnFallback { c.MultiTurnFallback = true }`
        + `if g.MultiTurnChunkTokens != 0 { c.MultiTurnChunkTokens = g.MultiTurnChunkTokens }`; overlay gains
        the matching src→dst clauses. S2 explicitly defers the dedicated file/overlay tests to S3."
  critical: "S3's tests assume S2 lands exactly as specified. If S2 hasn't landed, fileGeneration has no
             MultiTurn* fields → the tests won't compile (a clear signal, not a silent failure). The
             ACCEPTED false-in-file limitation originates in S2's bool-guard choice; S3 tests + documents it."

- docfile: plan/009_5c53066d64b3/P1M1T2S3/research/multiturn_config_tests_docs.md
  why: "THIS task's research: the materialize-vs-overlay distinction (§2 — omitted ⇒ Go zero-value at
        materialize time, Defaults applied via overlay; the root cause of the false-limitation); the
        gold-standard pattern to mirror (§3); the 3 docs edit sites with verbatim current→target (§4);
        decisions D1–D6."
  critical: "§2 (the materialize/overlay distinction) is THE insight: case (d) is an overlay-chain
             assertion, not a materialize-only one. §3 names the exact in-file test to mirror."

# The code under test + the patterns to mirror (READ-ONLY — do NOT edit production code)
- file: internal/config/file_test.go
  why: "THE pattern source. TestMaterializeOverlay_DiffContext_TokenLimit is the gold-standard 3-tier
        (materialize / overlay / loadTOML) table-driven test — copy its STRUCTURE for multi-turn.
        writeTempTOML(t,body) (line 16) writes a TOML file to t.TempDir(). TestLoadTOMLValid shows the
        loadTOML→assert idiom. TestOverlayPartial shows the Defaults→overlay partial-merge idiom."
  pattern: "materialize(&fileConfig{Generation: fileGeneration{<fields>}}, 0) → assert zero-value semantics;
            cfg := Defaults() → overlay(&cfg, materialize(...)) → assert resolved values; loadTOML(path)
            → Defaults() → overlay → assert end-to-end."
  gotcha: "loadTOML returns the MATERIALIZED file Config (NO Defaults overlay) — omit a key and the field
           is the Go zero-value (false/0), NOT the default. Defaults are applied by the SEPARATE overlay step.
           Mirror the DiffContext test's exact tier structure."

- file: internal/config/file.go
  why: "READ-ONLY (S2 edits it; S3 only tests). fileGeneration struct + materialize(fc,timeout) + overlay(dst,src)
        are the functions under test. Confirms materialize uses `g`/`c` and overlay uses `src`/`dst`; the
        bool guard `if g.X { c.X = true }`; the int guard `if g.X != 0 { c.X = g.X }`."
  gotcha: "Do NOT edit file.go (S2's territory). If a test fails because fileGeneration lacks a MultiTurn*
           field, S2 hasn't landed — wait/re-sync, do not edit file.go."

- file: internal/config/config.go
  why: "READ-ONLY (S1 LANDED). Config.MultiTurnFallback bool + Config.MultiTurnChunkTokens int + Defaults()
        sets true/32000. Confirms the resolved-field types + defaults the tests assert against."
- file: internal/config/config_test.go
  why: "READ-ONLY. TestDefaults (S1) ALREADY pins MultiTurnFallback==true + MultiTurnChunkTokens==32000 —
        that is contract case (a), already satisfied. Do NOT re-add it. (Also confirms the no-testify
        if/t.Errorf style.)"

- file: docs/configuration.md
  why: "THE docs edit target (3 spots). (D1) commented [generation] template block lines 104-113; insert
        after diff_context (108) and before exclude (109). (D2) 'Built-in defaults' table lines 132-135;
        insert 2 rows between max_duplicate_retries (134) and subject_target_chars (135). (D3) new
        'Multi-turn fallback' callout after the 'Token budget & diff context' callout (ends 149)."
  pattern: "Markdown table rows: `| key | default | source |`. Callouts: `> **Title.** … > - **key** …`.
            Match the surrounding line widths/comment style in the template block."

# External references
- url: https://pkg.go.dev/github.com/pelletier/go-toml/v2#readme-structs
  why: "Confirms go-toml/v2 decodes a struct field by matching the `toml:` tag to the TOML key, and that a
        key ABSENT from the TOML leaves the struct field at its Go zero-value (false/0). This is WHY the
        materialize-only tier asserts false/0 for omitted keys (not the Defaults)."
```

### Current Codebase Tree (relevant slice — S1 LANDED, S2 in progress)

```bash
stagecoach/
├── internal/config/
│   ├── config.go        # READ-ONLY (S1 LANDED — Config.MultiTurn* + Defaults true/32000)
│   ├── config_test.go   # READ-ONLY (S1 — TestDefaults already pins case (a))
│   ├── file.go          # S2 EDIT TARGET (in progress — fileGeneration + materialize + overlay)
│   └── file_test.go     # READ-ONLY pattern source (TestMaterializeOverlay_DiffContext_TokenLimit)
└── docs/
    └── configuration.md # EDIT TARGET (3 Mode A edits)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
├── internal/config/
│   └── multiturn_test.go   # NEW — package config; TestMaterializeOverlay_MultiTurn (cases b–e)
└── docs/
    └── configuration.md     # MODIFIED — [generation] template +2 lines, defaults table +2 rows, +1 callout
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/multiturn_test.go` | CREATE | `package config`. `TestMaterializeOverlay_MultiTurn` — materialize/overlay/loadTOML tiers covering cases (b)–(e); pins the case-(d) false-limitation as known/tested. |
| `docs/configuration.md` | MODIFY | (D1) `[generation]` template block +2 commented lines; (D2) defaults table +2 rows; (D3) "Multi-turn fallback" callout (FR-T12 + false-limitation). |

**Explicitly NOT touched**: `internal/config/config.go` (S1 LANDED), `internal/config/file.go` (S2),
`internal/config/config_test.go` (S1 — case (a) already there), `internal/config/file_test.go` (pattern
source), `internal/config/{git,load,bootstrap,migrate,role*}.go`, any other package, `PRD.md`,
`tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — case (a) is ALREADY DONE by S1; do NOT re-test it): S1's TestDefaults (config_test.go,
// LANDED) already asserts `c.MultiTurnFallback == true` and `c.MultiTurnChunkTokens == 32000`. The
// contract lists (a) as a case, but it was written before S1 landed. Re-adding it in multiturn_test.go is
// redundant + crosses S1's territory. Cross-reference TestDefaults instead.

// CRITICAL (G2 — case (d) is an OVERLAY-CHAIN assertion, not materialize-only): the false-limitation
// manifests at the overlay level, NOT materialize. At materialize time, `multi_turn_fallback = false` and
// "key omitted" are INDISTINGUISHABLE (both yield c.MultiTurnFallback == false — the bool guard
// `if g.MultiTurnFallback { c.MultiTurnFallback = true }` skips on false). The limitation bites because
// Defaults() sets MultiTurnFallback = true and overlay's only-true-propagates guard can never set it
// false. So case (d) MUST be: cfg := Defaults(); overlay(&cfg, loadTOML("multi_turn_fallback = false"));
// assert cfg.MultiTurnFallback is STILL true. Assert this EXPLICITLY with a t.Errorf message naming it
// "ACCEPTED v1 limitation" (the contract: "known, tested behavior, not a silent bug").

// CRITICAL (G3 — S2 is the code under test; do NOT edit file.go): S3 is TESTS-ONLY for code. If a test
// fails to compile because fileGeneration lacks MultiTurnFallback/MultiTurnChunkTokens, S2 hasn't landed —
// re-sync and re-run; do NOT edit file.go (S2's territory) to "fix" it.

// GOTCHA (G4 — loadTOML returns the MATERIALIZED file Config, NOT Defaults-merged): loadTOML(path) runs
// decode + materialize (file→Config) but does NOT overlay Defaults. So loadTOML of a file omitting
// multi_turn_fallback yields cfg.MultiTurnFallback == false (zero-value), NOT true. To get the RESOLVED
// value the generate core reads, do `dst := Defaults(); overlay(&dst, cfg)`. Mirror the DiffContext test's
// exact loadTOML→Defaults→overlay chain.

// GOTCHA (G5 — mirror the gold-standard test's 3-tier structure): TestMaterializeOverlay_DiffContext_TokenLimit
// (file_test.go) is the in-file precedent for a materialize/overlay table test. Copy its shape:
// (a) materialize-only subtests via materialize(&fileConfig{Generation: fileGeneration{...}}, 0);
// (b) overlay-chain subtests via cfg := Defaults() → overlay(&cfg, materialize(...));
// (c) loadTOML end-to-end subtests. Do NOT invent a new structure.

// GOTCHA (G6 — package config, reuse helpers, do NOT redeclare): multiturn_test.go is `package config`
// (white-box — same as file_test.go/config_test.go). writeTempTOML, loadTOML, materialize, overlay,
// Defaults, fileConfig, fileGeneration are ALL in scope. Redeclaring any is a compile error. Use plain
// bool/int literals in the table (no need for intPtr/strPtr/boolPtr — those are for pointer fields).

// GOTCHA (G7 — no testify): the config test suite uses plain if/t.Errorf (see TestDefaults,
// TestOverlayPartial). Mirror that; do not import testify.

// GOTCHA (G8 — docs: the callout must surface BOTH the FR-T12 note AND the false-limitation): the
// contract requires (1) a one-line FR-T12 note (multi_turn_chunk_tokens does NOT interact with token_limit;
// multi-turn uses the UNTRUNCATED payload) and (2) honest surfacing of the multi_turn_fallback=false
// limitation (cannot disable via file in this revision) + the workaround (set session_mode = "" on the
// provider). Put both in the new "Multi-turn fallback" callout (D3).

// GOTCHA (G9 — docs table ordering matches §16.2): insert the two defaults-table rows between
// max_duplicate_retries and subject_target_chars (matching §16.2's [generation] table order, which lists
// multi_turn_* right after max_duplicate_retries). Keep the template-block lines in the same relative
// position (after diff_context, before exclude).
```

## Implementation Blueprint

### Data models and structure

None. The test consumes existing types (`Config`, `fileConfig`, `fileGeneration`) and helpers. No new
production types. The docs edits are prose + table rows.

### The test file (exact — copy into `internal/config/multiturn_test.go`)

```go
// Package config — multi-turn config-knob tests (PRD §9.24 FR-T1c/FR-T3).
//
// TestMaterializeOverlay_MultiTurn proves the two multi-turn [generation] keys thread through the
// file-decode layer (fileGeneration → materialize → overlay) into the resolved Config the generate core
// (P1.M1.T3.S2 reads cfg.MultiTurnChunkTokens; P1.M1.T3.S3 reads cfg.MultiTurnFallback) consumes.
// Mirrors the gold-standard TestMaterializeOverlay_DiffContext_TokenLimit.
//
// Contract cases (P1.M1.T2.S3):
//   (b) multi_turn_chunk_tokens = 16000 in a file → resolved 16000 (int override honored)
//   (c) multi_turn_fallback = true in a file → resolved true (bool override honored)
//   (d) multi_turn_fallback = false in a file → resolved STILL true (ACCEPTED v1 limitation:
//       only-true-propagates, mirrors AutoStageAll; cannot disable via file)
//   (e) overlay precedence: repo-file chunk-tokens value overrides global-file value
//
// Case (a) — Defaults() returns true/32000 — is already pinned by TestDefaults (S1, LANDED); not duplicated.
package config

import "testing"

// TestMaterializeOverlay_MultiTurn is the load-bearing proof for the multi-turn config knobs across the
// materialize (file→Config) and overlay (Defaults → file) layers.
func TestMaterializeOverlay_MultiTurn(t *testing.T) {
	// ---- materialize-only: file → Config (the file→Config copy BEFORE any Defaults overlay) ----
	// materialize does NOT seed Defaults; an omitted key yields the Go zero-value (false/0).
	materializeCases := []struct {
		name         string
		fileFallback bool
		fileChunk    int
		wantFallback bool // omitted ⇒ false (Go zero-value); materialize does NOT seed the true default
		wantChunk    int  // omitted ⇒ 0
	}{
		{"both_omitted_zero_value", false, 0, false, 0},
		{"chunk_set_16000", false, 16000, false, 16000}, // (b) int honored at the file→Config copy
		{"fallback_set_true", true, 0, true, 0},         // (c) bool=true honored
		// The root cause of the limitation: false is INDISTINGUISHABLE from omitted at materialize time:
		{"fallback_false_same_as_omitted", false, 0, false, 0},
	}
	for _, tc := range materializeCases {
		tc := tc
		t.Run("materialize/"+tc.name, func(t *testing.T) {
			fc := &fileConfig{Generation: fileGeneration{
				MultiTurnFallback:    tc.fileFallback,
				MultiTurnChunkTokens: tc.fileChunk,
			}}
			c := materialize(fc, 0)
			if c.MultiTurnFallback != tc.wantFallback {
				t.Errorf("MultiTurnFallback = %v, want %v (materialize: omitted/false ⇒ Go zero-value false; it does NOT seed the default)", c.MultiTurnFallback, tc.wantFallback)
			}
			if c.MultiTurnChunkTokens != tc.wantChunk {
				t.Errorf("MultiTurnChunkTokens = %d, want %d (materialize: omitted ⇒ 0; set ⇒ propagated)", c.MultiTurnChunkTokens, tc.wantChunk)
			}
		})
	}

	// ---- overlay chain: Defaults() → overlay(file) — the RESOLVED value the generate core reads ----
	overlayCases := []struct {
		name         string
		fileFallback bool
		fileChunk    int
		wantFallback bool
		wantChunk    int
	}{
		{"omitted_keeps_defaults", false, 0, true, 32000},        // both omitted ⇒ Defaults win
		{"chunk_override_16000", false, 16000, true, 16000},       // (b) int override honored end-to-end
		{"chunk_override_48000", false, 48000, true, 48000},       // a larger value
		{"fallback_true_reasserts_true", true, 0, true, 32000},    // (c) bool=true honored (redundant w/ default)
		{"fallback_false_ignored_STAYS_true", false, 0, true, 32000}, // (d) THE limitation: false CANNOT disable
	}
	for _, tc := range overlayCases {
		tc := tc
		t.Run("overlay/"+tc.name, func(t *testing.T) {
			cfg := Defaults() // MultiTurnFallback=true, MultiTurnChunkTokens=32000
			g := materialize(&fileConfig{Generation: fileGeneration{
				MultiTurnFallback:    tc.fileFallback,
				MultiTurnChunkTokens: tc.fileChunk,
			}}, 0)
			overlay(&cfg, g)
			if cfg.MultiTurnFallback != tc.wantFallback {
				if tc.name == "fallback_false_ignored_STAYS_true" {
					t.Errorf("MultiTurnFallback = %v, want true — ACCEPTED v1 limitation: "+
						"multi_turn_fallback=false in a file is silently ignored (only-true-propagates, "+
						"mirrors auto_stage_all). This assertion deliberately PINS the limitation as "+
						"known/tested behavior, not a silent bug. To disable multi-turn for a provider, "+
						"set session_mode = \"\" on that provider (see docs/configuration.md).", cfg.MultiTurnFallback)
				} else {
					t.Errorf("MultiTurnFallback = %v, want %v (resolved value the generate core reads)", cfg.MultiTurnFallback, tc.wantFallback)
				}
			}
			if cfg.MultiTurnChunkTokens != tc.wantChunk {
				t.Errorf("MultiTurnChunkTokens = %d, want %d (resolved value the generate core reads)", cfg.MultiTurnChunkTokens, tc.wantChunk)
			}
		})
	}

	// ---- (e) overlay precedence: repo-file overrides global-file for chunk tokens ----
	t.Run("overlay/repo_overrides_global_chunk_tokens", func(t *testing.T) {
		cfg := Defaults() // 32000
		global := materialize(&fileConfig{Generation: fileGeneration{MultiTurnChunkTokens: 48000}}, 0)
		overlay(&cfg, global)
		if cfg.MultiTurnChunkTokens != 48000 {
			t.Fatalf("after global overlay: MultiTurnChunkTokens = %d, want 48000", cfg.MultiTurnChunkTokens)
		}
		repo := materialize(&fileConfig{Generation: fileGeneration{MultiTurnChunkTokens: 16000}}, 0)
		overlay(&cfg, repo) // higher layer (repo) wins
		if cfg.MultiTurnChunkTokens != 16000 {
			t.Errorf("after repo overlay: MultiTurnChunkTokens = %d, want 16000 (repo overrides global; higher layer wins)", cfg.MultiTurnChunkTokens)
		}
	})

	// ---- end-to-end via loadTOML (proves the TOML decode → resolved value path) ----
	t.Run("loadTOML/chunk_override_end_to_end", func(t *testing.T) {
		body := `
[generation]
multi_turn_chunk_tokens = 16000
`
		path := writeTempTOML(t, body)
		cfg, err := loadTOML(path)
		if err != nil || cfg == nil {
			t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
		}
		if cfg.MultiTurnChunkTokens != 16000 {
			t.Errorf("loadTOML MultiTurnChunkTokens = %d, want 16000 (materialized file value)", cfg.MultiTurnChunkTokens)
		}
		dst := Defaults()
		overlay(&dst, cfg)
		if dst.MultiTurnChunkTokens != 16000 {
			t.Errorf("after overlay: MultiTurnChunkTokens = %d, want 16000 (resolved)", dst.MultiTurnChunkTokens)
		}
		if !dst.MultiTurnFallback {
			t.Errorf("after overlay: MultiTurnFallback = false, want true (key omitted ⇒ default wins)")
		}
	})

	// ---- end-to-end: multi_turn_fallback = false is silently ignored (the ACCEPTED limitation) ----
	t.Run("loadTOML/fallback_false_ignored_end_to_end", func(t *testing.T) {
		body := `
[generation]
multi_turn_fallback = false
`
		path := writeTempTOML(t, body)
		cfg, err := loadTOML(path)
		if err != nil || cfg == nil {
			t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
		}
		// At materialize time the false decodes — INDISTINGUISHABLE from omitted (both false):
		if cfg.MultiTurnFallback {
			t.Errorf("loadTOML MultiTurnFallback = true, want false (materialized zero-value; false == omitted)")
		}
		// But after overlay on Defaults, the false CANNOT disable the default-true:
		dst := Defaults()
		overlay(&dst, cfg)
		if !dst.MultiTurnFallback {
			t.Errorf("after overlay: MultiTurnFallback = false, want true — ACCEPTED v1 limitation: "+
				"multi_turn_fallback=false in a config file is silently ignored (only-true-propagates, "+
				"mirrors auto_stage_all). Pinned here as known/tested behavior.", dst.MultiTurnFallback)
		}
	})
}
```

### The 3 docs edits (exact — current → target)

**Edit D1 — commented `[generation]` template block (`docs/configuration.md`, between lines 108 and 109).**

Current:
```
# diff_context          = 1       # 0 = changed-lines-only, 1 = one anchor (default), 3 = git default; FR3f; valid range 0–3 — out-of-range rejected at config load
# exclude               = []   # UNIONS across layers — see "Exclusion globs" below
```

Target (insert the two commented lines between them):
```
# diff_context          = 1       # 0 = changed-lines-only, 1 = one anchor (default), 3 = git default; FR3f; valid range 0–3 — out-of-range rejected at config load
# multi_turn_fallback     = true   # lossless multi-turn fallback on one-shot exhaustion (§9.24 FR-T1c); CANNOT disable via file (see "Multi-turn fallback" below)
# multi_turn_chunk_tokens = 32000  # per-turn chunk budget in tokens (§9.24 FR-T3); does NOT interact with token_limit (FR-T12)
# exclude               = []   # UNIONS across layers — see "Exclusion globs" below
```

**Edit D2 — "Built-in defaults" table (`docs/configuration.md`, between lines 134 and 135).**

Current:
```
| `max_duplicate_retries` | `3` | `config.Defaults()` |
| `subject_target_chars` | `50` | `config.Defaults()` |
```

Target (insert the two rows between them — matches §16.2 ordering):
```
| `max_duplicate_retries` | `3` | `config.Defaults()` |
| `multi_turn_fallback` | `true` | `config.Defaults()` (§9.24 FR-T1c) |
| `multi_turn_chunk_tokens` | `32000` | `config.Defaults()` (§9.24 FR-T3) |
| `subject_target_chars` | `50` | `config.Defaults()` |
```

**Edit D3 — new "Multi-turn fallback" callout (`docs/configuration.md`, immediately after the "Token budget & diff context" callout, which ends at line 149).**

Insert this new callout block right after the `diff_context` bullet (the last line of the existing callout):
```

> **Multi-turn fallback.** Two `[generation]` knobs control the lossless multi-turn fallback path (§9.24), which activates only after the one-shot retry loop exhausts on a large diff:
> - **`multi_turn_fallback`** (default `true`) — enables the fallback. **Limitation:** because this is a default-`true` boolean that uses the same only-true-propagates file pattern as `auto_stage_all`, you **cannot disable it by setting `multi_turn_fallback = false` in a config file** in this revision — the `false` is silently ignored (the resolved value stays `true`). To effectively disable multi-turn for a provider, set `session_mode = ""` on that provider (see [providers.md](providers.md#the-schema)); the shipped pi default is `"append"`.
> - **`multi_turn_chunk_tokens`** (default `32000`) — the per-request chunk size (tokens est.) the large diff is split into for multi-turn priming. **This does NOT interact with `token_limit`** (§9.24 FR-T12): `token_limit` truncates the one-shot payload, while multi-turn deliberately uses the **untruncated** payload, delivered in request-sized pieces — the two never compose for a single message.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/config/multiturn_test.go
  - FILE: internal/config/multiturn_test.go ; PACKAGE: config (white-box — same as file_test.go).
  - IMPORTS: testing only. (All helpers — writeTempTOML/loadTOML/materialize/overlay/Defaults,
    fileConfig/fileGeneration — are in-scope via package config; gotcha G6.)
  - WRITE the file verbatim from §"The test file" above (the package comment + TestMaterializeOverlay_MultiTurn).
  - DO NOT: re-add case (a) (G1 — S1's TestDefaults owns it); edit file.go (G3 — S2's territory);
    import testify (G7); redeclare any helper (G6).
  - VERIFY: go test ./internal/config/ -run TestMaterializeOverlay_MultiTurn -v → all subtests PASS.
    (If compile fails on fileGeneration.MultiTurn*, S2 hasn't landed — re-sync; do NOT edit file.go.)

Task 2: EDIT docs/configuration.md — Edit D1 (template block)
  - LOCATE the commented [generation] block (lines 104-113). Find the `# diff_context` line (108)
    followed by `# exclude` (109).
  - INSERT the two commented lines (multi_turn_fallback, multi_turn_chunk_tokens) between them, EXACTLY
    as in "Edit D1" target. Keep the surrounding comment alignment/width roughly consistent.
  - DO NOT: touch other template-block lines; reorder existing keys.

Task 3: EDIT docs/configuration.md — Edit D2 (defaults table)
  - LOCATE the "Built-in defaults" table. Find the `max_duplicate_retries` row (134) followed by the
    `subject_target_chars` row (135).
  - INSERT the two new rows between them, EXACTLY as in "Edit D2" target.
  - DO NOT: edit other table rows; change the table header.

Task 4: EDIT docs/configuration.md — Edit D3 (the callout)
  - LOCATE the "Token budget & diff context" callout (ends line 149 — the `diff_context` bullet).
  - INSERT the new "Multi-turn fallback" callout block immediately AFTER it, EXACTLY as in "Edit D3" target.
  - DO NOT: edit the existing callout; drop the FR-T12 note or the false-limitation (G8 — both required).

Task 5: VALIDATE
  - RUN: gofmt -l .            # must be empty (the test file; docs are not gofmt'd)
  - RUN: go build ./...        # must compile (S1 LANDED; S2 assumed landed)
  - RUN: go vet ./...
  - RUN: go test ./internal/config/... -v   # TestMaterializeOverlay_MultiTurn passes; TestDefaults (S1) still green
  - RUN: go test ./...          # whole repo green (additive test + docs-only prose)
  - RUN (docs wiring greps):
        grep -n 'multi_turn_fallback\|multi_turn_chunk_tokens' docs/configuration.md
        # EXPECT: ≥6 hits (2 template lines + 2 table rows + ≥2 callout mentions).
        grep -n 'FR-T12\|cannot disable\|session_mode' docs/configuration.md
        # EXPECT: the FR-T12 note + the limitation + the workaround all present in the callout.
  - RUN (scope):
        git diff --stat -- internal/config/file.go internal/config/config.go internal/config/config_test.go
        # EXPECT: EMPTY (no production code / S1 territory touched).
```

### Implementation Patterns & Key Details

```go
// === The 3-tier test structure (mirror TestMaterializeOverlay_DiffContext_TokenLimit) ===
// (1) materialize-only: proves the file→Config copy in isolation. Omitted keys ⇒ Go zero-value (false/0),
//     NOT Defaults. This tier isolates WHAT materialize does, independent of Defaults.
// (2) overlay-chain: cfg := Defaults() → overlay(&cfg, materialize(...)). Proves the RESOLVED value the
//     generate core reads — the Defaults + file merge. This is where the limitation manifests (case d).
// (3) loadTOML end-to-end: proves the TOML decode → materialize → overlay path with a real TOML string.
//     loadTOML returns the MATERIALIZED file Config (no Defaults); the test then overlays Defaults.

// === Why case (d) is an overlay-chain assertion, not materialize-only (G2) ===
// At materialize time, `multi_turn_fallback = false` and "key omitted" both yield c.MultiTurnFallback == false
// (the bool guard `if g.MultiTurnFallback { … }` skips on false). The limitation is that Defaults() sets the
// field true and overlay's only-true-propagates guard (`if src.MultiTurnFallback { dst.MultiTurnFallback = true }`)
// can never set it false. So the RESOLVED value (what the generate core reads) stays true regardless of a
// file's false. Asserting cfg.MultiTurnFallback == true AFTER `Defaults() → overlay(false-file)` PINS this.

// === Why the t.Errorf message for case (d) is verbose ===
// The contract: "assert this explicitly so it is a known, tested behavior, not a silent bug." A bare
// `want true` message would read like an accident. The named "ACCEPTED v1 limitation" + the workaround
// (session_mode = "") makes the PINNING intent unambiguous to a future reader who sees the test fail
// (which would mean someone fixed the limitation — they'd update the assertion).

// === Why no testify (G7) ===
// The config test suite is uniformly plain if/t.Errorf (TestDefaults, TestOverlayPartial,
// TestMaterializeOverlay_DiffContext_TokenLimit). Adding testify would diverge. Mirror the existing style.
```

### Integration Points

```yaml
CONFIG TESTS (internal/config/multiturn_test.go — NEW):
  - TestMaterializeOverlay_MultiTurn: materialize + overlay + loadTOML tiers; cases (b)-(e); case (d) limitation pin

CONSUMED (READ-ONLY — S1 LANDED + S2 in progress):
  - internal/config/config.go: Config.MultiTurnFallback bool + Config.MultiTurnChunkTokens int + Defaults true/32000
  - internal/config/file.go: fileGeneration.{MultiTurnFallback,MultiTurnChunkTokens} + materialize + overlay (S2)
  - internal/config/file_test.go: writeTempTOML + the gold-standard test pattern (read-only pattern source)
  - internal/config/config_test.go: TestDefaults (S1 — case (a) already pinned; do NOT duplicate)

DOCS (docs/configuration.md — 3 Mode A edits):
  - [generation] template block: +2 commented lines (multi_turn_fallback, multi_turn_chunk_tokens)
  - "Built-in defaults" table: +2 rows
  - "Multi-turn fallback" callout: FR-T12 non-interaction note + the false-limitation + the session_mode workaround

NO-TOUCH (explicitly — owned by siblings):
  - internal/config/config.go (S1 LANDED), config_test.go (S1 — case (a)), file.go (S2), file_test.go (pattern)
  - internal/config/{git,load,bootstrap,migrate,role*}.go (unaffected)
  - internal/provider/* (P1.M1.T1.S5), internal/generate/* (P1.M1.T3)
  - docs/{cli,providers,how-it-works,README}.md (S5 / other tasks), PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by LATER subtasks):
  - P1.M1.T3.S2: the N+1 turn protocol reads cfg.MultiTurnChunkTokens (this task PROVES the file path reaches it)
  - P1.M1.T3.S3: the FR-T1 trigger gate reads cfg.MultiTurnFallback (this task PROVES the file path reaches it)
  - P1.M1.T3.S4: the generate-core unit tests + Mode A how-it-works.md
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/config/multiturn_test.go   # Expected: empty (run gofmt -w if listed).
go vet ./internal/config/...                   # Expected: exit 0.
go build ./...                                 # Expected: exit 0 (the test file compiles; S1 LANDED; S2 assumed).

# Expected: zero errors. If `undefined: fileGeneration.MultiTurnFallback` (or .MultiTurnChunkTokens), S2
# has not landed its fileGeneration fields — re-sync (do NOT edit file.go to "fix" it; gotcha G3).
```

### Level 2: The New Unit Test (the deliverable)

```bash
cd /home/dustin/projects/stagecoach

go test ./internal/config/ -v -run TestMaterializeOverlay_MultiTurn
# Expected: ALL subtests PASS:
#   materialize/both_omitted_zero_value
#   materialize/chunk_set_16000                      ← case (b) at the file→Config copy
#   materialize/fallback_set_true                    ← case (c)
#   materialize/fallback_false_same_as_omitted       ← root cause of the limitation
#   overlay/omitted_keeps_defaults
#   overlay/chunk_override_16000                     ← case (b) end-to-end
#   overlay/chunk_override_48000
#   overlay/fallback_true_reasserts_true             ← case (c) end-to-end
#   overlay/fallback_false_ignored_STAYS_true        ← case (d) THE limitation (pinned)
#   overlay/repo_overrides_global_chunk_tokens       ← case (e)
#   loadTOML/chunk_override_end_to_end
#   loadTOML/fallback_false_ignored_end_to_end       ← case (d) end-to-end (pinned)

go test ./internal/config/ -v -run TestDefaults     # Expected: PASS (S1's case (a) still green; untouched).
go test ./internal/config/...                        # Expected: ok (whole config suite green).
```

### Level 3: Whole-Repository Regression + Scope Discipline

```bash
cd /home/dustin/projects/stagecoach

go test ./...    # Expected: ALL packages green (additive test file; docs-only prose).
go vet ./...     # Expected: exit 0.

# Confirm ONLY the intended files changed:
git status --porcelain
# Expected EXACTLY:
#   ?? internal/config/multiturn_test.go
#    M docs/configuration.md

# Confirm production code + S1/S2 territory UNTOUCHED:
git diff --stat -- internal/config/file.go internal/config/config.go internal/config/config_test.go internal/config/file_test.go internal/provider/ internal/generate/
# Expected: EMPTY.

# Docs wiring (the 3 edits landed):
grep -n 'multi_turn_fallback\|multi_turn_chunk_tokens' docs/configuration.md
# Expected: ≥6 hits across the template block (2), the defaults table (2), and the callout (≥2).
grep -n 'FR-T12\|cannot disable\|session_mode = ""' docs/configuration.md
# Expected: the FR-T12 note, the limitation, and the workaround all present in the new callout.
```

### Level 4: Behavioral Cross-Check (manual repro of the limitation)

```bash
cd /home/dustin/projects/stagecoach

# Reproduce the two resolved outcomes by hand (mirrors what the test asserts), to confirm the box behaves
# as expected. The authoritative proof is the unit test (Level 2); this is an optional sanity smoke.
cat > /tmp/sh_mt.go <<'EOF'
package main
import ("fmt";"os";"path/filepath";"time"
 "github.com/dustin/stagecoach/internal/config")
func main() {
	// chunk override honored
	dir, _ := os.MkdirTemp("","mt"); defer os.RemoveAll(dir)
	p := filepath.Join(dir, "c.toml")
	os.WriteFile(p, []byte("[generation]\nmulti_turn_chunk_tokens = 16000\n"), 0644)
	// loadTOML + overlay (mirrors the test's end-to-end tier; names are unexported — run via the test instead)
	_ = time.Second
	fmt.Println("see go test -run TestMaterializeOverlay_MultiTurn for the authoritative proof", p)
}
EOF
# NOTE: loadTOML/materialize/overlay are unexported (package config), so a /tmp main can't call them
# directly. The authoritative proof is the in-package unit test (Level 2). Delete the scratch file:
rm -f /tmp/sh_mt.go
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test ./...` — all packages green.
- [ ] `go test ./internal/config/ -v -run TestMaterializeOverlay_MultiTurn` — all subtests PASS.

### Feature Validation
- [ ] Case (b): `[generation] multi_turn_chunk_tokens = 16000` ⇒ resolved `cfg.MultiTurnChunkTokens == 16000`.
- [ ] Case (c): `[generation] multi_turn_fallback = true` ⇒ resolved `cfg.MultiTurnFallback == true`.
- [ ] Case (d): `[generation] multi_turn_fallback = false` ⇒ resolved `cfg.MultiTurnFallback == true` (ACCEPTED limitation; message names it).
- [ ] Case (e): repo-file `multi_turn_chunk_tokens` overrides global-file value.
- [ ] Case (a) NOT re-tested (S1's TestDefaults owns it).

### Docs Validation
- [ ] `[generation]` template block has both `multi_turn_fallback` and `multi_turn_chunk_tokens` commented lines.
- [ ] "Built-in defaults" table has both rows (between `max_duplicate_retries` and `subject_target_chars`).
- [ ] "Multi-turn fallback" callout surfaces: (1) FR-T12 non-interaction with `token_limit`; (2) the
      `false`-in-file limitation; (3) the `session_mode = ""` workaround.

### Scope Discipline Validation
- [ ] ONLY `internal/config/multiturn_test.go` (new) + `docs/configuration.md` (modified) changed.
- [ ] Did NOT touch `internal/config/file.go` (S2), `config.go` (S1), `config_test.go` (S1 — case (a)),
      `file_test.go` (pattern source), or any other package.
- [ ] Did NOT re-add case (a) Defaults pin (S1 owns it).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research).

### Code Quality Validation
- [ ] Test mirrors `TestMaterializeOverlay_DiffContext_TokenLimit`'s 3-tier structure (materialize / overlay / loadTOML).
- [ ] Plain if/t.Errorf (no testify); helpers reused (no redeclaration).
- [ ] Case-(d) assertion message names the limitation as "ACCEPTED v1 limitation" + the workaround.
- [ ] Docs rows/template/callout match the §16.2 ordering + the surrounding house style.

---

## Anti-Patterns to Avoid

- ❌ Don't re-add case (a) (`Defaults()` returns true/32000). S1's `TestDefaults` (LANDED) already pins it
  with those exact assertions. Duplicating it in `multiturn_test.go` is redundant + crosses S1's territory
  (gotcha G1).
- ❌ Don't test case (d) at the materialize level only. The limitation manifests at the OVERLAY level
  (`Defaults` true + only-true-propagates can't set false). At materialize time `false` and "omitted" are
  both `false` (indistinguishable) — asserting that is the root-cause tier, but the RESOLVED-behavior
  assertion (case d) MUST be `Defaults() → overlay(false-file) ⇒ still true` (gotcha G2).
- ❌ Don't edit `internal/config/file.go` (or `config.go`) to make a test pass. S2 (file.go) and S1
  (config.go) own those; if a test won't compile because `fileGeneration` lacks the fields, S2 hasn't
  landed — re-sync, do not "fix" production code (gotcha G3).
- ❌ Don't forget that `loadTOML` returns the MATERIALIZED file Config (no Defaults overlay). Asserting
  `cfg.MultiTurnFallback == true` on a bare `loadTOML` of an omitting file is WRONG (it's `false`). Always
  do `dst := Defaults(); overlay(&dst, cfg)` to get the resolved value (gotcha G4).
- ❌ Don't invent a new test structure — mirror `TestMaterializeOverlay_DiffContext_TokenLimit`'s 3 tiers
  (materialize / overlay / loadTOML). It's the in-file gold standard for this exact kind of test (gotcha G5).
- ❌ Don't import testify or redeclare `writeTempTOML`/`loadTOML`/`materialize`/`overlay`/`Defaults`/
  `fileConfig`/`fileGeneration` — they're all in scope via `package config` (gotchas G6/G7).
- ❌ Don't drop either the FR-T12 note OR the false-limitation from the docs callout — the contract
  requires both, plus the `session_mode = ""` workaround (gotcha G8).
- ❌ Don't reorder the docs defaults-table rows away from §16.2's ordering — the two new rows go between
  `max_duplicate_retries` and `subject_target_chars` (gotcha G9).
- ❌ Don't make the case-(d) `t.Errorf` message terse — the contract says "known, tested behavior, not a
  silent bug." Name it "ACCEPTED v1 limitation" + cite the workaround so a future reader who sees it fail
  knows it's a pinning assertion (not an accident).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a well-bounded tests + docs task. The test file mirrors an existing in-file gold
standard (`TestMaterializeOverlay_DiffContext_TokenLimit`) verbatim in structure, reuses all helpers (same
package, no redeclaration), and every assertion is pinned to the empirically-verified materialize/overlay
semantics (omitted ⇒ Go zero-value at materialize time; Defaults applied via overlay; the false-limitation
manifests at the overlay level). S1 is confirmed LANDED on disk (Config fields + Defaults + TestDefaults
case-(a) pins all present), so the resolved-value assertions have a stable target. The one dependency is
S2 (fileGeneration + materialize/overlay clauses), which is explicitly assumed-landed per the parallel
contract; if it hasn't, the test fails to compile with a clear `undefined: fileGeneration.MultiTurn*`
signal (not a silent failure). The 3 docs edits are quoted verbatim current→target at exact line anchors.
The two plausible mistakes — (a) re-adding case (a) (G1) and (b) testing case (d) at the materialize level
instead of the overlay level (G2) — are front-loaded as CRITICAL gotchas. The residual 0.5 uncertainty is
the docs line numbers shifting if a sibling edits configuration.md concurrently (P1.M1.T1.S5 touches
providers.md primarily, but configuration.md's session_mode note at line 24 suggests concurrent edits are
possible); the edit anchors are unique text strings (not line numbers), so they survive moderate drift.
No production-code risk (tests + docs only).
