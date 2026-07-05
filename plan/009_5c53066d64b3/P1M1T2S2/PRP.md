---
name: "P1.M1.T2.S2 — Add fields to fileGeneration struct + materialize + overlay"
description: |
  Thread the two multi-turn config knobs through the FILE-decode layer in `internal/config/file.go`
  (PRD §9.24 FR-T1c/FR-T3; the file-based precedence rung of §16.1). Three surgical edits, all in
  `internal/config/file.go`: (1) add `MultiTurnFallback bool` (toml `multi_turn_fallback`) +
  `MultiTurnChunkTokens int` (toml `multi_turn_chunk_tokens`) to the `fileGeneration` struct between
  `MaxDuplicateRetries` and `SubjectTargetChars` (~line 54); (2) in `materialize` (~line 229) add two
  clauses mirroring the house guard templates — `if g.MultiTurnFallback { c.MultiTurnFallback = true }`
  (bool, only-true-propagates, mirrors AutoStageAll) and `if g.MultiTurnChunkTokens != 0 {
  c.MultiTurnChunkTokens = g.MultiTurnChunkTokens }` (int, mirrors TokenLimit); (3) in `overlay`
  (~line 343) add the same two clauses (`src`→`dst`). ACCEPTED limitation (per the contract + research):
  `multi_turn_fallback = false` in a file is silently ignored — the same v1 limitation `AutoStageAll`
  carries (S3 documents it). Consumes S1 (LANDED: `Config.MultiTurnFallback` + `Config.MultiTurnChunkTokens`
  + Defaults true/32000). NO new test (S3 owns the dedicated file/overlay tests + docs). NO CLI flags/env
  (out of scope per the delta). NO git.go edit (contract LOGIC is file.go only). Behavior-free: existing
  suite stays green (new fileGeneration fields default to false/0 → skipped → Defaults values win).
---

## Goal

**Feature Goal**: Make a user's `[generation]`-table `multi_turn_fallback` and `multi_turn_chunk_tokens`
keys (in a global or repo `.stagehand.toml` / `config.toml`) flow through the file-decode layer into the
resolved `Config` that the downstream multi-turn generate core (P1.M1.T3.S2 reads `cfg.MultiTurnChunkTokens`;
P1.M1.T3.S3 reads `cfg.MultiTurnFallback`) consumes. After this task, a TOML file setting either key
propagates through `materialize` (file→Config) and `overlay` (global→repo field-by-field merge) to the
resolved config; the resolved-`Config` fields S1 landed are now actually reachable from a file.

**Deliverable**: ONE file modified — `internal/config/file.go` — with three surgical edits:
1. `fileGeneration` struct: +`MultiTurnFallback bool` + `MultiTurnChunkTokens int` (between
   `MaxDuplicateRetries` and `SubjectTargetChars`).
2. `materialize`: +two copy clauses (bool only-true-propagates; int `!= 0`) between the
   `MaxDuplicateRetries` and `SubjectTargetChars` clauses.
3. `overlay`: +two merge clauses (same guards, `src`→`dst`) between the same two neighbors.

No other file touched. No new test (S3), no docs (S3), no CLI flags/env, no git.go edit.

**Success Definition**: a TOML `[generation]` block with `multi_turn_chunk_tokens = 48000` resolves to
`cfg.MultiTurnChunkTokens == 48000`; `multi_turn_fallback = true` resolves to `cfg.MultiTurnFallback == true`;
omitting both keys resolves to the S1 defaults (`32000` / `true`) — verified by the existing suite staying
green (the additions are skipped when the file omits them); `go build/vet/gofmt` clean; `go test ./...`
green; `git diff --stat` shows ONLY `internal/config/file.go`.

## User Persona

**Target User**: (1) The contributor implementing P1.M1.T3 (the multi-turn generate core that reads
`cfg.MultiTurnFallback`/`cfg.MultiTurnChunkTokens`); (2) end users who tune multi-turn via the
`[generation]` table in their config file.

**Use Case**: A user on a 200K-context model raises the per-request chunk size for fewer turns:
`[generation]\nmulti_turn_chunk_tokens = 48000`. After S2, that file value flows through `materialize` →
resolved `cfg.MultiTurnChunkTokens == 48000`, which P1.M1.T3.S2's chunk sizer reads.

**User Journey**: TOML `[generation]` table → `fileConfig.fileGeneration` (Decode) → `materialize`
(file→Config copy) → `overlay` (global→repo merge, field-by-field) → resolved `Config` → read by the
generate core. This task is the file→Config + merge rungs.

**Pain Points Addressed**: Without these edits, the resolved-`Config` fields S1 landed are unreachable
from a file — a user setting `multi_turn_chunk_tokens` in their config would see it silently ignored
(zero-value fileGeneration field → materialize skips → Defaults 32000 wins regardless). This task wires
the file-decode path so the knobs are actually configurable.

## Why

- **PRD §9.24 FR-T1c/FR-T3 mandate both knobs as `[generation]`-table keys.** FR-T1c (`multi_turn_fallback`
  default true) and FR-T3 (`multi_turn_chunk_tokens` default 32000). §16.1 lists both in the built-in-defaults
  layer (S1); §16.2 shows them in the `[generation]` table right after `max_duplicate_retries`. This task is
  the file-decode layer that makes the §16.2 keys reachable.
- **The resolved Config is the single source the generate core reads.** `cfg.MultiTurnFallback` /
  `cfg.MultiTurnChunkTokens` (S1, LANDED) must be populated from the file layers — `materialize` (file→Config)
  and `overlay` (global→repo merge) are the two functions that do that for every other `[generation]` scalar.
- **Lowest-risk seam-threading.** Two new fields on `fileGeneration` + four guard clauses (two in
  `materialize`, two in `overlay`) mirroring existing templates. The new fields default to Go zero-values
  (`false`/`0`), so every existing config-test fixture (TOML without `multi_turn_*`) resolves identically —
  no behavior change, the existing suite stays green.
- **Accepts the documented v1 bool limitation.** `multi_turn_fallback` (default true) uses the
  only-true-propagates bool pattern (`AutoStageAll`/`Verbose`/`Push`), so `multi_turn_fallback = false` in a
  file is silently ignored. This is the same limitation `AutoStageAll` (also default-true) carries; the
  contract explicitly accepts "follow whatever pattern auto_stage_all uses." S3 surfaces it in
  `docs/configuration.md`. (The `*bool` fix is deliberately NOT taken — it widens scope and diverges from
  the delta; `DiffContext *int` is the precedent for the *pointer* pattern, not applicable here.)
- **No user-facing/docs surface (contract point 5).** The `[generation]` table doc + the limitation note
  ride with P1.M1.T2.S3.

## What

Three edits in `internal/config/file.go`, all mirroring existing house-style templates:

1. **`fileGeneration` struct** — add `MultiTurnFallback bool` + `MultiTurnChunkTokens int` (with TOML tags
   `multi_turn_fallback` / `multi_turn_chunk_tokens`) between `MaxDuplicateRetries` and `SubjectTargetChars`.
2. **`materialize`** — add `if g.MultiTurnFallback { c.MultiTurnFallback = true }` (bool template) and
   `if g.MultiTurnChunkTokens != 0 { c.MultiTurnChunkTokens = g.MultiTurnChunkTokens }` (int template)
   between the `MaxDuplicateRetries` and `SubjectTargetChars` clauses.
3. **`overlay`** — add `if src.MultiTurnFallback { dst.MultiTurnFallback = true }` and
   `if src.MultiTurnChunkTokens != 0 { dst.MultiTurnChunkTokens = src.MultiTurnChunkTokens }` between the
   same two neighbors.

No new test (S3 owns dedicated file/overlay tests), no docs (S3), no CLI flags/env (out of scope per the
delta), no git.go edit (contract LOGIC is file.go only), no config.go edit (S1 LANDED — read-only).

### Success Criteria

- [ ] `fileGeneration` has `MultiTurnFallback bool` (toml `multi_turn_fallback`) + `MultiTurnChunkTokens int`
      (toml `multi_turn_chunk_tokens`), between `MaxDuplicateRetries` and `SubjectTargetChars`.
- [ ] `materialize` has the bool-guard clause (`if g.MultiTurnFallback { c.MultiTurnFallback = true }`) and
      the int-guard clause (`if g.MultiTurnChunkTokens != 0 { … }`), between the two neighbor clauses.
- [ ] `overlay` has the matching `src`→`dst` clauses (same guards), between the two neighbor clauses.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean.
- [ ] `go test ./...` green — existing suite unchanged (new fields default false/0 → skipped → Defaults win).
- [ ] `git diff --stat` shows ONLY `internal/config/file.go`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim current code at all three edit sites (fileGeneration
struct lines 54-55; materialize lines 229-234; overlay lines 343-348) and gives the exact target for each
(copy-paste-ready, with gofmt re-alignment noted). It states the two guard templates (bool
only-true-propagates via AutoStageAll; int `!= 0` via TokenLimit) with in-file precedents, confirms S1 is
LANDED (the resolved fields exist), confirms the fields are ABSENT in file.go (genuine add), and explains
why the change is behavior-free for existing tests. No inference required.

### Documentation & References

```yaml
# MUST READ — the FR specs + the config model
- file: PRD.md
  why: "§9.24 FR-T1c (multi_turn_fallback default true — condition (c) of the FR-T1 trigger) and FR-T3
        (multi_turn_chunk_tokens default 32000 — per-request chunk sizing). §16.1 (built-in defaults list both).
        §16.2 (the [generation] table shows both right after max_duplicate_retries — IS the field ordering)."
  critical: "FR-T1c default true + FR-T3 default 32000 are the Defaults() values S1 already set; this task
             makes the [generation]-table keys reachable. §16.2's ordering (after max_duplicate_retries) IS
             the fileGeneration struct-field placement. The TOML key names are multi_turn_fallback /
             multi_turn_chunk_tokens verbatim."

- docfile: plan/009_5c53066d64b3/architecture/research-generate-config.md
  why: "§3 is the authoritative spec for THIS edit: §3a fileGeneration (line 44; the two fields go after
        MaxDuplicateRetries line 54); §3b materialize (line 193; the int-guard template at :219/229 and the
        bool-guard template at :213/275); §3c overlay (line 294; the int template at :332/343 and bool at
        :318/327). §3b's DESIGN TENSION block is the bool-limitation decision (recommend (a) accept, NOT (b) *bool)."
  critical: "§3b's bool-guard template (AutoStageAll/Push: only-true-propagates) is EXACTLY what
             MultiTurnFallback mirrors; §3b's int-guard template (TokenLimit/MaxDuplicateRetries: != 0) is
             what MultiTurnChunkTokens mirrors. The DESIGN TENSION note (default-true + only-true-propagates
             ⇒ false-in-file ignored) is the ACCEPTED limitation — do NOT 'fix' it with *bool."

- docfile: plan/009_5c53066d64b3/P1M1T2S1/PRP.md
  why: "The S1 CONTRACT (LANDED). Specifies the resolved-Config fields this task copies INTO: Config.MultiTurnFallback
        bool (config.go:84) + Config.MultiTurnChunkTokens int (config.go:85) + Defaults true/32000 (config.go:179-180)
        + TestDefaults pins. S1's scope explicitly fences file.go to THIS task (S2)."
  critical: "Treat S1 as LANDED. The resolved fields already exist with the right types (plain bool/int) and
             defaults. This task (S2) is the file-decode layer that populates them from a TOML file. Do NOT
             edit config.go (S1's territory)."

- docfile: plan/009_5c53066d64b3/P1M1T2S2/research/file_generation_materialize_overlay_notes.md
  why: "THIS task's research: the verbatim current code at all 3 edit sites (with exact line numbers), the
        exact target for each (copy-paste-ready), the two guard templates with in-file precedents, the
        ACCEPTED bool-limitation decision, the behavior-free regression argument, and decisions D1–D6."
  critical: "§2 (the three current→target edits) and §3 (why existing tests stay green) are the implementation
             spec. §4 (do-NOT-do) fences S3 (tests+docs), git.go, env/flags, and the *bool temptation."

- file: internal/config/file.go
  why: "THE edit target (3 spots). fileGeneration struct (lines 48-62; insert at 54-55). materialize
        (line 193; insert between the MaxDuplicateRetries clause 229-231 and SubjectTargetChars 232-234).
        overlay (line 294; insert between MaxDuplicateRetries 343-345 and SubjectTargetChars 346-348).
        materialize uses `g` (fileGeneration) + `c` (*Config); overlay uses `src`+`dst` (both *Config)."
  pattern: "Int guard: `if g.X != 0 { c.X = g.X }` / `if src.X != 0 { dst.X = src.X }` (TokenLimit,
            MaxDuplicateRetries, MaxCommits, SubjectTargetChars). Bool guard: `if g.X { c.X = true }` /
            `if src.X { dst.X = true }` (AutoStageAll, Verbose, Push — only-true-propagates)."
  gotcha: "Use `!= 0` for MultiTurnChunkTokens (NOT `> 0`) — matches every existing int field; for the
           positive default 32000 and positive user values they're equivalent. Use only-true-propagates for
           MultiTurnFallback (NOT `*bool`) — matches AutoStageAll; the false-in-file limitation is ACCEPTED."

# Read-only cross-refs (do NOT edit)
- file: internal/config/config.go
  why: "READ-ONLY (S1 LANDED). Config.MultiTurnFallback bool (line 84) + Config.MultiTurnChunkTokens int
        (line 85) + Defaults true/32000 (lines 179-180). Confirms the resolved-field types this task copies into."
- file: internal/config/load.go
  why: "READ-ONLY. The precedence resolver that calls Defaults() → materialize → overlay across global→repo
        layers. Adding the overlay clauses is SUFFICIENT to thread the fields through file-based precedence;
        load.go itself needs no edit (it already calls overlay for every layer)."
- file: internal/config/git.go
  why: "READ-ONLY. The stagehand.* git-config resolver. NOT edited here (contract LOGIC is file.go only).
        Plan 009 has no git-config subtask for multi-turn → file-only by design. The overlay clauses this
        task adds would let a future git-config resolver compose, but adding the keys is out of scope."

# External references
- url: https://pkg.go.dev/github.com/pelletier/go-toml/v2#readme-structs
  why: "Confirms go-toml/v2 decodes into struct fields by matching the `toml:` tag (snake_case) to the TOML
        key, and that a struct field absent from the TOML keeps its Go zero-value (false/0). This is WHY the
        new fileGeneration fields default to false/0 when a file omits them (the behavior-free guarantee)."
```

### Current Codebase Tree (relevant slice — S1 LANDED)

```bash
stagehand/
└── internal/config/
    ├── config.go        # READ-ONLY (S1 LANDED — Config.MultiTurn* fields + Defaults)
    ├── file.go          # EDIT TARGET — fileGeneration struct + materialize + overlay (3 edits)
    ├── config_test.go   # READ-ONLY (S1's TestDefaults already pins true/32000; S3 adds file/overlay tests)
    └── load.go, git.go  # READ-ONLY (precedence resolver; git-config resolver — unaffected)
```

### Desired Codebase Tree After This Subtask

```bash
stagehand/
└── (only one existing file modified — no new files)
    internal/config/file.go   # fileGeneration +2 fields; materialize +2 clauses; overlay +2 clauses
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/file.go` | MODIFY | (1) fileGeneration: +`MultiTurnFallback bool` + `MultiTurnChunkTokens int`; (2) materialize: +bool-guard +int-guard clauses; (3) overlay: +matching src→dst clauses. |

**Explicitly NOT touched**: `internal/config/config.go` (S1 LANDED), `internal/config/config_test.go`
(S3 owns the dedicated file/overlay tests), `internal/config/load.go` + `git.go` (unaffected / out of scope),
`docs/*` (S3 — the `[generation]` table + the false-limitation note), `internal/provider/*` (P1.M1.T1.S5),
`internal/generate/*` (P1.M1.T3), any other package, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — bool guard is only-true-propagates, NOT *bool): MultiTurnFallback mirrors AutoStageAll/
// Verbose/Push — `if g.MultiTurnFallback { c.MultiTurnFallback = true }`. Because Defaults() sets it true,
// a file CANNOT disable it (false-in-file is ignored). This is the ACCEPTED v1 limitation (research §3b
// recommends (a) accept over (b) *bool; the delta says "follow auto_stage_all"). Do NOT make it *bool
// (DiffContext *int is the pointer precedent, but the contract chose the plain-bool AutoStageAll pattern).
// S3 documents the limitation in docs/configuration.md.

// CRITICAL (G2 — int guard is != 0, NOT > 0): MultiTurnChunkTokens mirrors TokenLimit/MaxDuplicateRetries/
// MaxCommits — `if g.MultiTurnChunkTokens != 0 { c.MultiTurnChunkTokens = g.MultiTurnChunkTokens }`. For the
// positive default (32000) and any positive user value, != 0 and > 0 are equivalent; the contract says use
// != 0 to match house style. A file value of 0 is treated as "unset" → keeps the 32000 default (same as
// every other int field).

// CRITICAL (G3 — three edits, all between MaxDuplicateRetries and SubjectTargetChars): the struct field
// order, the materialize clause order, and the overlay clause order must ALL stay in lockstep (matches S1's
// resolved-Config order + §16.2's [generation] table order). Insert the two new entries between those two
// neighbors in all three sites. Do NOT append at the end of the struct/function.

// GOTCHA (G4 — gofmt re-aligns the struct): fileGeneration's field block is gofmt-aligned on the type column
// + the TOML-tag column. Adding a `bool` and an `int` field (different widths) triggers a re-align of the
// whole block. Run `gofmt -w internal/config/file.go`; do NOT hand-align.

// GOTCHA (G5 — materialize uses `g`/`c`; overlay uses `src`/`dst`): materialize reads `g` (the fileGeneration
// var) and writes `c` (*Config); overlay reads `src` and writes `dst` (both *Config). The bool/int guard
// SHAPE is identical in both — only the receiver names differ. Do not mix them.

// GOTCHA (G6 — behavior-free; existing tests stay green): the new fileGeneration fields default to false/0
// (Go zero-value) when a TOML file omits them. materialize/overlay skip them (false → skip; 0 → skip), so
// c.MultiTurnFallback/c.MultiTurnChunkTokens keep the Defaults() values (true/32000). Every existing config
// fixture resolves identically → TestDefaults (S1) still passes → go test ./... green. If any existing test
// changes, something beyond the 3 edits was touched — re-check scope.

// GOTCHA (G7 — no new test in S2): the dedicated file/overlay unit tests for these fields are P1.M1.T2.S3
// ("Config unit tests + Mode A docs"). Adding them here overlaps S3. S2's validation is the existing suite
// staying green + the grep gates (Level 2/3). Do NOT add a test.

// GOTCHA (G8 — scope fence: NO config.go, NO git.go, NO env/flags, NO docs): S1 owns config.go (LANDED);
// the git-config resolver (git.go) is NOT in the contract LOGIC (file-only by design); env/flags are
// explicitly out of scope ("no new CLI flags/env per the delta"); docs ride with S3. Only file.go changes.
```

## Implementation Blueprint

### Data models and structure

No new types. Two fields on the existing `fileGeneration` struct (the FILE-decode twin — only `file.go`
decodes into it). The "model" facts are the type choices (`bool` + `int`, mirroring `AutoStageAll` +
`MaxDuplicateRetries`) and the two guard templates (only-true-propagates; `!= 0`).

### The three edits (exact — current → target)

**Edit 1 — `fileGeneration` struct (`internal/config/file.go:54-55`).**

Current:
```go
	MaxDuplicateRetries int      `toml:"max_duplicate_retries"`
	SubjectTargetChars  int      `toml:"subject_target_chars"`
```

Target (insert the two fields between them):
```go
	MaxDuplicateRetries  int      `toml:"max_duplicate_retries"`   // re-gen attempts on duplicate subject
	MultiTurnFallback    bool     `toml:"multi_turn_fallback"`     // §9.24 FR-T1c multi-turn fallback (default true); only-true-propagates (mirrors AutoStageAll)
	MultiTurnChunkTokens int      `toml:"multi_turn_chunk_tokens"` // §9.24 FR-T3 per-request chunk size in tokens (default 32000); != 0 guard (mirrors TokenLimit)
	SubjectTargetChars   int      `toml:"subject_target_chars"`
```

**Edit 2 — `materialize` (`internal/config/file.go:229-234`).**

Current:
```go
	if g.MaxDuplicateRetries != 0 {
		c.MaxDuplicateRetries = g.MaxDuplicateRetries
	}
	if g.SubjectTargetChars != 0 {
		c.SubjectTargetChars = g.SubjectTargetChars
	}
```

Target (insert the two clauses between them):
```go
	if g.MaxDuplicateRetries != 0 {
		c.MaxDuplicateRetries = g.MaxDuplicateRetries
	}
	// §9.24 FR-T1c — multi_turn_fallback (bool; only-true-propagates, mirrors AutoStageAll —
	// cannot disable via file, same v1 limitation; S3 documents this in docs/configuration.md).
	if g.MultiTurnFallback {
		c.MultiTurnFallback = true
	}
	// §9.24 FR-T3 — multi_turn_chunk_tokens (int; != 0, mirrors TokenLimit/MaxDuplicateRetries).
	if g.MultiTurnChunkTokens != 0 {
		c.MultiTurnChunkTokens = g.MultiTurnChunkTokens
	}
	if g.SubjectTargetChars != 0 {
		c.SubjectTargetChars = g.SubjectTargetChars
	}
```

**Edit 3 — `overlay` (`internal/config/file.go:343-348`).**

Current:
```go
	if src.MaxDuplicateRetries != 0 {
		dst.MaxDuplicateRetries = src.MaxDuplicateRetries
	}
	if src.SubjectTargetChars != 0 {
		dst.SubjectTargetChars = src.SubjectTargetChars
	}
```

Target (insert the two clauses between them):
```go
	if src.MaxDuplicateRetries != 0 {
		dst.MaxDuplicateRetries = src.MaxDuplicateRetries
	}
	// §9.24 FR-T1c — multi_turn_fallback (bool; only-true-propagates, mirrors AutoStageAll/Push —
	// cannot disable via file, same v1 limitation).
	if src.MultiTurnFallback {
		dst.MultiTurnFallback = true
	}
	// §9.24 FR-T3 — multi_turn_chunk_tokens (int; != 0, mirrors TokenLimit/MaxCommits).
	if src.MultiTurnChunkTokens != 0 {
		dst.MultiTurnChunkTokens = src.MultiTurnChunkTokens
	}
	if src.SubjectTargetChars != 0 {
		dst.SubjectTargetChars = src.SubjectTargetChars
	}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/file.go — Edit 1 (fileGeneration struct fields)
  - FILE: internal/config/file.go
  - LOCATE fileGeneration (line 47). Find `MaxDuplicateRetries int` (line 54) followed by
    `SubjectTargetChars int` (line 55).
  - INSERT the two fields between them, EXACTLY as in "Edit 1" target above.
  - TYPES: MultiTurnFallback bool, MultiTurnChunkTokens int (PLAIN; gotcha G1 — NOT *bool/*int).
  - TOML TAGS: `multi_turn_fallback`, `multi_turn_chunk_tokens` (verbatim from §16.2).
  - DO NOT: place them elsewhere (G3 — immediately after MaxDuplicateRetries); make them pointers.
  - RUN: gofmt -w internal/config/file.go  (re-aligns the struct block; G4)
  - VERIFY: go build ./internal/config/  → exit 0.

Task 2: EDIT internal/config/file.go — Edit 2 (materialize clauses)
  - LOCATE materialize (line 193). Find the `if g.MaxDuplicateRetries != 0 { … }` clause (line 229)
    followed by `if g.SubjectTargetChars != 0 { … }` (line 232).
  - INSERT the bool-guard clause + the int-guard clause between them, EXACTLY as in "Edit 2" target.
  - GUARDS: bool `if g.MultiTurnFallback { c.MultiTurnFallback = true }` (G1); int `!= 0` (G2).
  - DO NOT: use `> 0` for the int; use `*bool` for the bool; mix src/dst names (materialize is g/c — G5).
  - RUN: gofmt -w internal/config/file.go
  - VERIFY: go build ./internal/config/  → exit 0.

Task 3: EDIT internal/config/file.go — Edit 3 (overlay clauses)
  - LOCATE overlay (line 294). Find `if src.MaxDuplicateRetries != 0 { … }` (line 343) followed by
    `if src.SubjectTargetChars != 0 { … }` (line 346).
  - INSERT the two src→dst clauses between them, EXACTLY as in "Edit 3" target.
  - GUARDS: identical shape to Edit 2 but src→dst (G5).
  - DO NOT: change any other overlay clause; edit load.go (it already calls overlay for every layer).
  - RUN: gofmt -w internal/config/file.go
  - VERIFY: go build ./internal/config/  → exit 0.

Task 4: VALIDATE (no new test — S3 owns the dedicated tests; existing suite green is the regression proof)
  - RUN: gofmt -l .            # must be empty
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test ./...          # whole repo green (new fields default false/0 → skipped → Defaults win)
  - RUN targeted: go test ./internal/config/ -v   # TestDefaults (S1) still pins true/32000; all config tests pass
  - RUN (wiring grep — the contract's headline check):
        grep -n 'MultiTurnFallback\|MultiTurnChunkTokens' internal/config/file.go
        # EXPECT: 2 struct-field hits + 2 materialize-clause hits + 2 overlay-clause hits = 6 total.
  - RUN (scope grep):
        git diff --stat -- internal/config/config.go internal/config/git.go internal/config/load.go docs/
        # EXPECT: EMPTY (only file.go changed).
  - FIX-FORWARD: a compile failure = a field-name/type typo or a mixed receiver name; a test failure =
                 something beyond the 3 edits was touched. Read + fix.
```

### Implementation Patterns & Key Details

```go
// === The bool-guard template (only-true-propagates) — MultiTurnFallback mirrors AutoStageAll ===
// materialize: `if g.MultiTurnFallback { c.MultiTurnFallback = true }`
// overlay:     `if src.MultiTurnFallback { dst.MultiTurnFallback = true }`
// Because Defaults() sets MultiTurnFallback = true, a file can only RE-ASSERT true, never set false.
// This is the documented v1 limitation AutoStageAll (also default-true) carries. Accepted per the contract;
// S3 documents it. The *bool alternative (DiffContext *int is the pointer precedent) is deliberately NOT
// taken — it widens scope and diverges from the delta's "follow auto_stage_all" instruction.

// === The int-guard template (!= 0) — MultiTurnChunkTokens mirrors TokenLimit/MaxDuplicateRetries ===
// materialize: `if g.MultiTurnChunkTokens != 0 { c.MultiTurnChunkTokens = g.MultiTurnChunkTokens }`
// overlay:     `if src.MultiTurnChunkTokens != 0 { dst.MultiTurnChunkTokens = src.MultiTurnChunkTokens }`
// For the positive default (32000) and any positive user value, != 0 ≡ > 0. A file value of 0 = "unset"
// → keeps the 32000 default (same semantics as every other int field). Use != 0 (NOT > 0) for house style.

// === Why all three sites insert between MaxDuplicateRetries and SubjectTargetChars ===
// Keeps the [generation] scalar cluster in lockstep across (1) the decode struct, (2) the file→Config copy,
// and (3) the field-merge — matching S1's resolved-Config struct order and §16.2's [generation] table order
// (multi_turn_* right after max_duplicate_retries). A reader can then trace a field top-to-bottom through
// the three layers without hunting.

// === Why this is behavior-free (the regression argument) ===
// The new fileGeneration fields default to false/0 (Go zero-value) when a TOML file omits them. Both guards
// skip on the zero-value (false → bool-guard skips; 0 → != 0 skips), so c.MultiTurnFallback/ChunkTokens keep
// the Defaults() values (true/32000) for every existing fixture. TestDefaults (S1) still pins true/32000;
// no existing config test asserts the field set of fileGeneration. Hence go test ./... stays green by
// construction — the only NEW behavior (a file SETTING a key now propagates) has no existing test (S3 adds it).
```

### Integration Points

```yaml
FILE DECODE (internal/config/file.go):
  - fileGeneration struct: +MultiTurnFallback bool (toml multi_turn_fallback) + MultiTurnChunkTokens int (toml multi_turn_chunk_tokens)
  - materialize (file→Config copy): +bool-guard clause + int-guard clause (g → c)
  - overlay (global→repo field-by-field merge): +matching src→dst clauses

CONSUMED (READ-ONLY — S1 LANDED):
  - internal/config/config.go: Config.MultiTurnFallback bool + Config.MultiTurnChunkTokens int + Defaults true/32000

PRECEDENCE CHAIN (informational — no edit needed):
  - load.go calls Defaults() → materialize (per file layer) → overlay (across global→repo). Adding the
    overlay clauses is SUFFICIENT to thread the fields through all file-based layers. The git-config
    resolver (git.go) does NOT read multi-turn keys (file-only by design; no plan-009 subtask for it).

NO-TOUCH (explicitly — contract or sibling ownership):
  - internal/config/config.go, config_test.go   # S1 (LANDED) + S3 (dedicated file/overlay tests)
  - internal/config/git.go, load.go             # git-config resolver NOT in contract LOGIC; load.go unaffected
  - docs/* (the [generation] table + false-limitation note = S3)
  - internal/provider/* (P1.M1.T1.S5), internal/generate/* (P1.M1.T3)
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by LATER subtasks):
  - P1.M1.T2.S3: dedicated config unit tests (file-set multi_turn_chunk_tokens → resolved value; overlay
    global-vs-repo) + docs/configuration.md [generation] table rows + the false-limitation note
  - P1.M1.T3.S2: the N+1 turn protocol reads cfg.MultiTurnChunkTokens (chunk sizing)
  - P1.M1.T3.S3: the FR-T1 trigger gate reads cfg.MultiTurnFallback (condition (c))
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand

gofmt -l internal/config/file.go   # Expected: empty (run gofmt -w if listed — re-aligns the struct block)
go vet ./internal/config/...        # Expected: exit 0
go build ./...                      # Expected: exit 0 (the new fields + clauses compile)

# Expected: Zero errors. A compile error most likely = a typo in a field name or a mixed receiver (g/src/c/dst).
```

### Level 2: Wiring + Regression (existing suite stays green — no new test in S2)

```bash
cd /home/dustin/projects/stagehand

# The wiring is present (2 struct fields + 2 materialize clauses + 2 overlay clauses = 6 hits):
grep -n 'MultiTurnFallback\|MultiTurnChunkTokens' internal/config/file.go
# Expected: 6 matches total — 2 in fileGeneration, 2 in materialize, 2 in overlay.

# The TOML keys are the §16.2 names:
grep -n 'multi_turn_fallback\|multi_turn_chunk_tokens' internal/config/file.go
# Expected: the 2 snake_case TOML tags in fileGeneration.

# The guard shapes are correct (bool only-true-propagates; int != 0):
grep -n 'if g.MultiTurnFallback {\|if src.MultiTurnFallback {\|if g.MultiTurnChunkTokens != 0\|if src.MultiTurnChunkTokens != 0' internal/config/file.go
# Expected: 4 clause lines (bool + int × materialize + overlay).

# Existing suite green (behavior-free — new fields default false/0 → skipped → Defaults win):
go test ./internal/config/ -v   # TestDefaults (S1) still pins true/32000; all config tests pass.
go test ./...                   # Expected: ALL packages green.
```

### Level 3: Scope Discipline (only file.go changed)

```bash
cd /home/dustin/projects/stagehand

# ONLY internal/config/file.go changed.
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: only internal/config/file.go.

# S1's territory (config.go) + the git-config resolver (git.go) + load.go + docs are UNTOUCHED:
git diff --stat -- internal/config/config.go internal/config/config_test.go internal/config/git.go internal/config/load.go docs/
# Expected: EMPTY.

# Confirm the diff is exactly the 3 intended hunks (eyeball the patch):
git diff -- internal/config/file.go
# Expected: three changed hunks — (1) fileGeneration +2 fields, (2) materialize +2 clauses, (3) overlay +2 clauses.
```

### Level 4: Behavioral Cross-Check (a file value propagates; absence keeps the default)

```bash
cd /home/dustin/projects/stagehand

# S3 will add the dedicated file/overlay unit test; here we cross-check via a throwaway that materialize +
# overlay actually propagate a set value (and skip an unset one). Requires S1's resolved fields.
cat > /tmp/sh_mt_check.go <<'EOF'
package main
import ("fmt";"time"
 "github.com/dustin/stagehand/internal/config")
func main() {
	// A file that SETS multi_turn_chunk_tokens (and re-asserts multi_turn_fallback = true):
	toml := `[generation]
multi_turn_chunk_tokens = 48000
multi_turn_fallback = true
`
	fc, err := config.DecodeTOMLString(toml) // or whichever helper the suite exposes; see file.go exports
	if err != nil { fmt.Println("decode:", err); return }
	c := config.Materialize(fc, 120*time.Second) // materialize entry; adjust to the exported name
	fmt.Printf("chunk=%d want 48000; fallback=%v want true\n", c.MultiTurnChunkTokens, c.MultiTurnFallback)
	// Absent keys → Defaults:
	toml2 := `[generation]\nmax_diff_bytes = 1000\n`
	fc2, _ := config.DecodeTOMLString(toml2)
	c2 := config.Materialize(fc2, 120*time.Second)
	fmt.Printf("absent: chunk=%d want 32000; fallback=%v want true\n", c2.MultiTurnChunkTokens, c2.MultiTurnFallback)
}
EOF
# NOTE: the exact exported decode/materialize names live in internal/config (file.go) — the implementing
# agent should run the equivalent via the package's test helpers OR defer the behavioral proof to S3's
# dedicated unit test. The authoritative S2 gate is Level 2 (wiring greps) + Level 3 (scope) + green suite.
go run /tmp/sh_mt_check.go 2>/dev/null && rm -f /tmp/sh_mt_check.go || rm -f /tmp/sh_mt_check.go
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test ./...` — all packages green (behavior-free; TestDefaults still pins true/32000).

### Feature Validation
- [ ] `fileGeneration` has `MultiTurnFallback bool` + `MultiTurnChunkTokens int` (toml `multi_turn_fallback`/`multi_turn_chunk_tokens`), between `MaxDuplicateRetries` and `SubjectTargetChars`.
- [ ] `materialize` has the bool-guard clause (`if g.MultiTurnFallback { c.MultiTurnFallback = true }`) + int-guard clause (`if g.MultiTurnChunkTokens != 0 { … }`), between the two neighbor clauses.
- [ ] `overlay` has the matching `src`→`dst` clauses (same guards), between the two neighbor clauses.
- [ ] Guard shapes: bool = only-true-propagates (NOT `*bool`); int = `!= 0` (NOT `> 0`).

### Scope Discipline Validation
- [ ] ONLY `internal/config/file.go` modified (`git diff --stat` confirms).
- [ ] Did NOT edit `internal/config/config.go` (S1 LANDED), `config_test.go` (S3), `git.go`/`load.go` (out of scope/unaffected).
- [ ] Did NOT edit `docs/*` (S3), `internal/provider/*` (S5), `internal/generate/*` (T3).
- [ ] Did NOT add CLI flags / env vars (out of scope per the delta).
- [ ] Did NOT add a test (S3 owns the dedicated file/overlay tests + docs).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research note).

### Code Quality Validation
- [ ] Field placement is in lockstep across struct + materialize + overlay (all between MaxDuplicateRetries ↔ SubjectTargetChars).
- [ ] Bool mirrors `AutoStageAll`/`Verbose`/`Push`; int mirrors `TokenLimit`/`MaxDuplicateRetries`/`MaxCommits`.
- [ ] TOML tags match §16.2 verbatim (`multi_turn_fallback`, `multi_turn_chunk_tokens`).
- [ ] gofmt re-aligns the struct; no hand-alignment.
- [ ] The ACCEPTED false-in-file limitation is noted in the comments (S3 surfaces it in docs).

---

## Anti-Patterns to Avoid

- ❌ Don't make `MultiTurnFallback` a `*bool` on `fileGeneration`. The contract says "follow auto_stage_all"
  (plain `bool`, only-true-propagates). `DiffContext *int` is the pointer-pattern precedent but is NOT the
  chosen pattern here. The false-in-file limitation is ACCEPTED; S3 documents it (gotcha G1).
- ❌ Don't use `> 0` for `MultiTurnChunkTokens` — use `!= 0` to match every existing int field
  (TokenLimit/MaxDuplicateRetries/MaxCommits/SubjectTargetChars). For the positive default + positive user
  values they're equivalent; house style is `!= 0` (gotcha G2).
- ❌ Don't place the fields/clauses anywhere other than between `MaxDuplicateRetries` and `SubjectTargetChars`.
  All three sites (struct + materialize + overlay) stay in lockstep with S1's resolved-Config order and
  §16.2's table order (gotcha G3).
- ❌ Don't mix the receiver names — `materialize` uses `g`/`c`; `overlay` uses `src`/`dst`. The guard SHAPE
  is identical; only the names differ (gotcha G5).
- ❌ Don't add a test in S2. The dedicated file/overlay unit tests are P1.M1.T2.S3. S2's validation is the
  existing suite staying green + the wiring greps (gotcha G7).
- ❌ Don't edit `config.go` (S1 LANDED), `git.go` (the git-config resolver is NOT in the contract LOGIC —
  multi-turn is file-only by design), `load.go` (it already calls overlay for every layer), or `docs/*` (S3)
  (gotcha G8).
- ❌ Don't add CLI flags or env vars — the delta explicitly says "no new CLI flags/env" (out of scope).
- ❌ Don't hand-align the struct — run `gofmt -w`; it re-aligns after the bool/int insert (gotcha G4).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a minimal, fully-prescribed change — two struct fields + two materialize clauses + two
overlay clauses — with every edit site quoted verbatim from the live tree (fileGeneration lines 54-55,
materialize lines 229-234, overlay lines 343-348) and the target for each copy-paste-ready. S1 is confirmed
LANDED (the resolved-`Config` fields + Defaults + TestDefaults pins all exist), and the fields are confirmed
ABSENT in file.go (genuine add). The two guard templates (bool only-true-propagates via AutoStageAll; int
`!= 0` via TokenLimit) are in-file precedents — eliminating the two plausible over-edits: making
`MultiTurnFallback` a `*bool` (the contract chose the AutoStageAll plain-bool pattern; the false-in-file
limitation is ACCEPTED and documented) and using `> 0` for the int (house style is `!= 0`). The change is
behavior-free by construction (new fileGeneration fields default to false/0 → both guards skip → Defaults
win for every existing fixture), so `go test ./...` staying green IS the regression proof. The residual 0.5
uncertainty is the Level-4 cross-check's dependence on the package's exported decode/materialize helper
names (the implementing agent may need to defer the behavioral proof to S3's dedicated unit test if the
helpers aren't conveniently exported) — mitigated by the deterministic Level 2 wiring greps (6 hits) + the
green suite. The S3 (tests+docs), T3 (generate-core), and S5 (provider) boundaries are cleanly fenced
downstream consumers that cannot be broken by adding two file-decode fields + four guard clauses.
