---
name: "P1.M1.T2.S1 — Add MultiTurnFallback + MultiTurnChunkTokens to resolved Config struct + Defaults()"
description: |
  Config-layer scalars for the multi-turn generation fallback (PRD §9.24 FR-T1c/FR-T3). Add two fields to
  the resolved `Config` struct in `internal/config/config.go` immediately after `MaxDuplicateRetries`:
  `MultiTurnFallback bool` (toml `multi_turn_fallback`, default `true`, FR-T1c) and
  `MultiTurnChunkTokens int` (toml `multi_turn_chunk_tokens`, default `32000`, FR-T3). Add the matching
  defaults to `Defaults()` and pin them in the existing `TestDefaults`. Plain types (NOT pointers — the
  resolved Config is flat/plain-typed; the *bool/*int pointer pattern is S2's file-decode layer). No docs
  (the [generation] table doc rides with P1.M1.T2.S3). Consumed by P1.M1.T3.S2 (protocol reads
  cfg.MultiTurnChunkTokens) + P1.M1.T3.S3 (trigger gate reads cfg.MultiTurnFallback). Baseline GREEN; fields absent.
---

## Goal

**Feature Goal**: Land the two resolved-`Config` scalars that the multi-turn generation fallback (§9.24)
reads — `MultiTurnFallback` (the FR-T1c enable gate, default true) and `MultiTurnChunkTokens` (the FR-T3
per-request chunk size, default 32000) — so the downstream generate-core tasks (P1.M1.T3.S2/S3) have a
stable config surface to consume, and the file-decode layer (P1.M1.T2.S2) has the resolved-Config fields
to copy into.

**Deliverable** (2 production edits + 1 test edit, all in `internal/config/`):
1. `internal/config/config.go` `Config` struct — add `MultiTurnFallback bool` + `MultiTurnChunkTokens int`
   immediately after `MaxDuplicateRetries` (line 83), with TOML tags + FR-cited comments.
2. `internal/config/config.go` `Defaults()` — add `MultiTurnFallback: true` + `MultiTurnChunkTokens: 32000`
   after the `MaxDuplicateRetries: 3` entry (line 176).
3. `internal/config/config_test.go` `TestDefaults` — add 2 assertions pinning the new defaults (after the
   MaxDuplicateRetries check, line 48).

**Success Definition**: `Config.MultiTurnFallback` defaults to `true` and `Config.MultiTurnChunkTokens`
defaults to `32000` (verified by `TestDefaults`); the resolved Config still compiles; the existing
`config_test.go` (which constructs `Defaults()` literals) compiles unchanged; `go build/vet/gofmt` clean;
`go test ./...` green. No file outside `internal/config/config.go` + `config_test.go` touched.

## User Persona

**Target User**: The contributors implementing the downstream multi-turn tasks — P1.M1.T2.S2 (fileGeneration/materialize/overlay copy the new fields), P1.M1.T3.S2 (the N+1 turn protocol reads `cfg.MultiTurnChunkTokens`), P1.M1.T3.S3 (the trigger gate reads `cfg.MultiTurnFallback`).

**Use Case**: A user opts into multi-turn fallback (default on); the generate-core reads `cfg.MultiTurnFallback` to decide whether the FR-T1 trigger condition (c) holds, and `cfg.MultiTurnChunkTokens` to size each request chunk. This task puts the two resolved fields in place first.

**Pain Points Addressed**: Unblocks P1.M1.T3 (the generate core) with a stable config surface; the fields default correctly (true / 32000) so multi-turn works out-of-the-box once the protocol lands.

## Why

- **PRD §9.24 mandates both knobs.** FR-T1c (`multi_turn_fallback` default true — condition (c) of the FR-T1 trigger) and FR-T3 (`multi_turn_chunk_tokens` default 32000 — the chunk sizing). §16.1 lists both in the built-in-defaults layer; §16.2 shows them in the `[generation]` table right after `max_duplicate_retries`.
- **The resolved Config is the single source consumed by the generate core.** `cfg.MultiTurnFallback` / `cfg.MultiTurnChunkTokens` are read directly by P1.M1.T3.S2/S3 — these fields must exist (with correct defaults) before those tasks land.
- **Lowest-risk seam-threading.** Adding two plain-typed fields with defaults to a struct is provably backward-compatible (Go zero-value; every existing `Defaults()` literal + test continues to compile). The file-decode-layer plumbing (`fileGeneration` + `materialize` + `overlay`) is S2; this task is the resolved-Config layer only.
- **No user-facing/docs surface (contract point 5).** The `[generation]` table doc rides with P1.M1.T2.S3.

## What

Two new fields appended to the `Config` struct after `MaxDuplicateRetries`, the matching defaults in
`Defaults()`, and two assertions in `TestDefaults`. Both plain-typed (`bool`, `int`). No file-decode-layer
changes, no docs, no other package.

### Success Criteria

- [ ] `Config` struct has `MultiTurnFallback bool` (toml `multi_turn_fallback`) + `MultiTurnChunkTokens int`
      (toml `multi_turn_chunk_tokens`), immediately after `MaxDuplicateRetries`.
- [ ] Each field's doc comment cites its FR (§9.24 FR-T1c / FR-T3) and names its downstream consumer.
- [ ] `Defaults()` sets `MultiTurnFallback: true` + `MultiTurnChunkTokens: 32000` (with FR-ref inline comments).
- [ ] `TestDefaults` asserts `c.MultiTurnFallback == true` + `c.MultiTurnChunkTokens == 32000`.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test ./...` green.
- [ ] No file outside `internal/config/config.go` + `internal/config/config_test.go` modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim current struct lines (config.go:83-84) + Defaults()
lines (config.go:176-177) + TestDefaults lines (config_test.go:46-51), gives the exact target for each
(copy-paste-ready, with gofmt re-alignment noted), states the type rationale (plain bool/int vs the S2
pointer pattern, with the AutoStageAll/MaxDuplicateRetries precedents), and confirms the fields are absent
+ baseline green. No inference required.

### Documentation & References

```yaml
# MUST READ — the FR specs + the config model
- file: PRD.md
  why: "§9.24 FR-T1c (multi_turn_fallback default true — condition (c) of the FR-T1 trigger gate) and FR-T3 (multi_turn_chunk_tokens default 32000 — per-request chunk sizing). §16.1 (built-in defaults list both). §16.2 (the [generation] table shows both right after max_duplicate_retries)."
  critical: "FR-T1c default true and FR-T3 default 32000 ARE the values Defaults() must set. §16.2's ordering (right after max_duplicate_retries) IS the struct-field placement."

- docfile: plan/009_5c53066d64b3/architecture/research-generate-config.md
  why: "§2 confirms TokenLimit at config.go:81 + MaxDuplicateRetries at :83, and that new keys go 'next to TokenLimit / MaxDuplicateRetries'. Also flags the MISNAMING: the field-merge is overlay() at file.go:294 + materialize() at file.go:193 (NOT a 'merge' function) — that is S2's territory, NOT this task."
  critical: "Confirms the exact insertion point (after MaxDuplicateRetries :83) and the S1/S2 boundary: S1 = the resolved Config struct + Defaults(); S2 = fileGeneration + materialize + overlay (the file-decode copy)."

- docfile: plan/009_5c53066d64b3/P1M1T1S5/PRP.md
  why: "The parallel sibling (Render unit tests in internal/provider + Mode A docs in docs/). Confirms NO internal/config edit — S5 is test+docs in internal/provider + docs/. No file overlap → no conflict."
  critical: "S5 does NOT touch internal/config. This task (S1) owns internal/config/config.go + config_test.go exclusively within P1.M1.T2."

- docfile: plan/009_5c53066d64b3/P1M1T2S1/research/multiturn_config_fields_notes.md
  why: "EMPIRICALLY VERIFIED (live code, 2026-07-05): both fields ABSENT (genuine add); baseline GREEN (1.657s); the exact insertion points (struct :83, Defaults :176, TestDefaults :46-51) with verbatim current→target; the type rationale (plain bool/int — AutoStageAll/MaxDuplicateRetries precedents); the TestDefaults-pin decision; decisions D1–D6. READ THIS FIRST."
  critical: "§3 is copy-paste-ready (struct + Defaults + TestDefaults, verbatim). §2 (the type rationale) prevents the over-edit of making these *bool/*int pointers (that's S2's file-decode concern)."

# The edit targets
- file: internal/config/config.go
  why: "EDIT (2 spots). (a) Config struct: the generation scalars at lines 78-96; insert the two fields between MaxDuplicateRetries (:83) and SubjectTargetChars (:84). (b) Defaults() (~:161): insert the two defaults between MaxDuplicateRetries: 3 (:176) and SubjectTargetChars: 50 (:177)."
  pattern: "Plain-typed scalars with snake_case TOML tags + inline // comments. The struct is gofmt-aligned (type column + tag column); adding bool/int fields triggers a re-align — run gofmt -w. Defaults() is a single struct literal returned by value."
  gotcha: "Use PLAIN bool/int, NOT *bool/*int. The resolved Config docstring (config.go:47-61) says 'flat, resolved, plain-typed'. The *bool/*int pointer pattern (DiffContext *int, StripCodeFence *bool) is for the file-decode layer (fileGeneration in file.go) to distinguish absent-vs-explicit-zero/false during overlay — that is S2's concern. AutoStageAll (bool, default true) is the exact precedent for MultiTurnFallback."

- file: internal/config/config_test.go
  why: "EDIT (1 spot). TestDefaults (:11) enumerates every scalar default sequentially. Add the two assertions between the MaxDuplicateRetries check (:46-48) and the SubjectTargetChars check (:49-51) — matches the struct-field order."
  pattern: "Plain if/t.Errorf (NO testify). The existing checks: `if c.MaxDuplicateRetries != 3 { t.Errorf(...) }`. Mirror for the two new fields (`!c.MultiTurnFallback` for the bool; `!= 32000` for the int)."

# Read-only refs (do NOT edit)
- file: internal/config/file.go
  why: "READ-ONLY. fileGeneration struct + materialize() (~:193) + overlay() (~:294) are the file-decode layer that will copy the new fields — that is P1.M1.T2.S2, NOT this task. Confirms the S1/S2 boundary."
- file: internal/config/defaults.go
  why: "READ-ONLY (none — Defaults() lives in config.go in this codebase, NOT a separate defaults.go; §16.1's 'internal/config/defaults.go' is a doc simplification). The actual Defaults() is config.go:161."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/config/
    ├── config.go        # EDIT: Config struct (2 fields) + Defaults() (2 entries)
    └── config_test.go   # EDIT: TestDefaults (2 assertions)
# (internal/config/file.go is READ-ONLY — fileGeneration/materialize/overlay = P1.M1.T2.S2)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/config/config.go        # +MultiTurnFallback bool + MultiTurnChunkTokens int (struct + Defaults)
    internal/config/config_test.go   # +2 TestDefaults assertions
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/config.go` | MODIFY | Add the two fields to the `Config` struct (after MaxDuplicateRetries) + the two defaults to `Defaults()` (after MaxDuplicateRetries: 3). **Only production file.** |
| `internal/config/config_test.go` | MODIFY | Pin the two new defaults in `TestDefaults` (after the MaxDuplicateRetries check). |

**Explicitly NOT touched**: `internal/config/file.go` (fileGeneration/materialize/overlay = P1.M1.T2.S2),
`internal/config/git.go`/`load.go` (unaffected), `docs/*` (the [generation] table doc = P1.M1.T2.S3),
`internal/provider/*` (P1.M1.T1.S5 — parallel), `internal/generate/*` (P1.M1.T3), any other package,
`PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — plain bool/int, NOT pointers): MultiTurnFallback is `bool`, MultiTurnChunkTokens is `int`.
// The resolved Config is "flat, resolved, plain-typed" (config.go:47-61). The *bool/*int pointer pattern
// (DiffContext *int, StripCodeFence *bool) exists for the FILE-DECODE layer (fileGeneration) to distinguish
// "key absent" from "explicit false/0" during overlay — that is S2's concern. AutoStageAll (bool, true) is
// the exact precedent for MultiTurnFallback; MaxDuplicateRetries (int, 3) for MultiTurnChunkTokens. Do NOT
// make these *bool/*int — it would push file-decode concerns into the resolved layer + mismatch the contract.

// GOTCHA (G2 — gofmt re-aligns the struct): the generation-scalar block is gofmt-aligned on the type column
// + the TOML-tag column. Adding `bool` + `int` fields (different widths than the surrounding `int`) triggers
// a re-align of the whole block. Run `gofmt -w internal/config/config.go`; do NOT hand-align.

// GOTCHA (G3 — place fields immediately after MaxDuplicateRetries, NOT at the end of the struct): the
// contract specifies "immediately after MaxDuplicateRetries (line ~83)". This keeps the generation-scalar
// cluster contiguous + matches §16.2's [generation] table ordering. SubjectTargetChars (line 84) shifts down.

// GOTCHA (G4 — TOML tags are documentation parity, not decode keys): the resolved Config is NEVER directly
// TOML-decoded (fileConfig / fileGeneration is). The snake_case tags are kept for documentation parity
// (so the struct reads like the [generation] table). Use the exact §16.2 names: `multi_turn_fallback`,
// `multi_turn_chunk_tokens`.

// GOTCHA (G5 — Defaults() return is BY VALUE): Defaults() returns `Config{...}` by value (config.go:161
// docstring: "Returned BY VALUE"). Adding two fields to the literal is a local edit; no caller needs to
// change (every caller reads fields off the returned value).

// GOTCHA (G6 — TestDefaults enumerates ALL scalar defaults; add the two): leaving the new defaults
// unverified in TestDefaults is inconsistent with the file's pattern (it pins MaxDiffBytes/MaxMdLines/
// TokenLimit/DiffContext/MaxDuplicateRetries/SubjectTargetChars/…). Add the two assertions (6 lines).
// S3 owns the file/overlay tests — no overlap.

// GOTCHA (G7 — scope): ONLY config.go + config_test.go. file.go (S2), docs (S3), internal/provider (S5),
// internal/generate (T3) are out of scope. Editing them crosses the subtask boundary.
```

## Implementation Blueprint

### Data models and structure

No new types. Two plain scalars on the existing `Config` struct. The "model" facts are the type choices
(plain `bool`/`int` — matching `AutoStageAll`/`MaxDuplicateRetries`) and the defaults (`true`/`32000`).

### The edits (exact — current → target)

**Config struct** (`internal/config/config.go:83-84`):
```go
// CURRENT
	MaxDuplicateRetries int  `toml:"max_duplicate_retries"` // re-gen attempts on duplicate subject
	SubjectTargetChars  int  `toml:"subject_target_chars"`  // target subject length for truncation

// TARGET (insert the two fields between them; gofmt re-aligns the columns)
	MaxDuplicateRetries  int  `toml:"max_duplicate_retries"`   // re-gen attempts on duplicate subject
	MultiTurnFallback    bool `toml:"multi_turn_fallback"`     // §9.24 FR-T1c multi-turn fallback (lossless large-diff priming); default true; consumed by P1.M1.T3.S3 trigger gate
	MultiTurnChunkTokens int  `toml:"multi_turn_chunk_tokens"` // §9.24 FR-T3 per-request chunk size (tokens est) for multi-turn; default 32000; consumed by P1.M1.T3.S2 protocol
	SubjectTargetChars   int  `toml:"subject_target_chars"`    // target subject length for truncation
```

**Defaults()** (`internal/config/config.go:176-177`):
```go
// CURRENT
		MaxDuplicateRetries: 3,
		SubjectTargetChars:  50,

// TARGET (insert the two defaults between them)
		MaxDuplicateRetries:   3,
		MultiTurnFallback:     true,  // §9.24 FR-T1c default (multi-turn fallback enabled)
		MultiTurnChunkTokens:  32000, // §9.24 FR-T3 default (per-request chunk size, tokens est)
		SubjectTargetChars:    50,
```

**TestDefaults** (`internal/config/config_test.go:46-51`):
```go
// CURRENT
	if c.MaxDuplicateRetries != 3 {
		t.Errorf("MaxDuplicateRetries = %d, want 3", c.MaxDuplicateRetries)
	}
	if c.SubjectTargetChars != 50 {

// TARGET (insert the two assertions between them)
	if c.MaxDuplicateRetries != 3 {
		t.Errorf("MaxDuplicateRetries = %d, want 3", c.MaxDuplicateRetries)
	}
	if !c.MultiTurnFallback {
		t.Errorf("MultiTurnFallback = false, want true (§9.24 FR-T1c)")
	}
	if c.MultiTurnChunkTokens != 32000 {
		t.Errorf("MultiTurnChunkTokens = %d, want 32000 (§9.24 FR-T3)", c.MultiTurnChunkTokens)
	}
	if c.SubjectTargetChars != 50 {
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/config.go — add the two Config struct fields
  - FILE: internal/config/config.go
  - LOCATE the generation-scalar block (lines 78-96). Find `MaxDuplicateRetries int` (line 83) followed by
    `SubjectTargetChars int` (line 84).
  - INSERT (between them) the two fields EXACTLY as in "Config struct" target above.
  - TYPES: MultiTurnFallback bool, MultiTurnChunkTokens int (PLAIN — not pointers; gotcha G1).
  - TOML TAGS: `multi_turn_fallback`, `multi_turn_chunk_tokens` (gotcha G4).
  - COMMENTS: cite §9.24 FR-T1c / FR-T3 + name the downstream consumer (P1.M1.T3.S3 / S2).
  - DO NOT: make them *bool/*int; place them elsewhere (G3 — immediately after MaxDuplicateRetries).
  - RUN: gofmt -w internal/config/config.go  (re-aligns the block; G2)
  - VERIFY: go build ./internal/config/  → exit 0.

Task 2: EDIT internal/config/config.go — add the two Defaults() entries
  - LOCATE Defaults() (~line 161). Find `MaxDuplicateRetries: 3,` (line 176) followed by
    `SubjectTargetChars: 50,` (line 177).
  - INSERT (between them) `MultiTurnFallback: true,` + `MultiTurnChunkTokens: 32000,` with the FR-ref
    inline comments, EXACTLY as in "Defaults()" target above.
  - DO NOT: change any other default; edit the return type (it stays `Config` by value; G5).
  - RUN: gofmt -w internal/config/config.go
  - VERIFY: go build ./internal/config/  → exit 0.

Task 3: EDIT internal/config/config_test.go — pin the two new defaults in TestDefaults
  - FILE: internal/config/config_test.go
  - LOCATE TestDefaults (line 11). Find the MaxDuplicateRetries check (lines 46-48) followed by the
    SubjectTargetChars check (lines 49-51).
  - INSERT (between them) the two assertions EXACTLY as in "TestDefaults" target above.
  - STYLE: plain if/t.Errorf (NO testify); mirror the existing `c.MaxDuplicateRetries != 3` shape.
  - DO NOT: relax/edit the existing assertions; add testify; edit other tests.
  - VERIFY: go test ./internal/config/ -run TestDefaults -v  → PASS (the 2 new assertions hold).

Task 4: VALIDATE
  - RUN: gofmt -l .            # must be empty
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test ./...          # whole repo green (the resolved-Config change is additive)
  - RUN targeted: go test ./internal/config/ -run TestDefaults -v
  - FIX-FORWARD: a compile failure = a field-name/type typo; a TestDefaults failure = a wrong default value. Read + fix.
```

### Implementation Patterns & Key Details

```go
// === Why plain bool/int (the resolved-Config discipline) ===
// The resolved Config (config.go:47-61) is "flat, resolved, plain-typed" — it carries FINAL values, never
// "unset" sentinels. The *bool/*int pointer pattern is the file-decode layer's (fileGeneration) mechanism
// for distinguishing "user omitted the key" from "user set false/0" DURING overlay. That distinction is
// resolved AWAY before reaching Config. So Config.MultiTurnFallback is a plain bool (default true, set by
// Defaults(); the overlay in S2 copies fileGeneration.MultiTurnFallback over it when the user sets it).
// AutoStageAll (bool, true) is the exact precedent.

// === Why the fields sit after MaxDuplicateRetries ===
// The contract: "immediately after MaxDuplicateRetries (line ~83)". This keeps the [generation] scalar
// cluster contiguous and matches §16.2's [generation] table ordering (multi_turn_* right after
// max_duplicate_retries). SubjectTargetChars shifts down two lines.

// === Why TOML tags on a never-directly-decoded struct ===
// The resolved Config is decoded from NOTHING — fileConfig/fileGeneration is the decode target. The tags
// exist for documentation parity (the struct reads like the [generation] table). S2 adds the matching
// tags to fileGeneration; S3 documents the [generation] table. Here the tags are cosmetic-but-consistent.
```

### Integration Points

```yaml
CONFIG (internal/config/config.go):
  - Config struct: +MultiTurnFallback bool (toml multi_turn_fallback) + MultiTurnChunkTokens int (toml multi_turn_chunk_tokens)
  - Defaults(): +MultiTurnFallback: true + MultiTurnChunkTokens: 32000
  - both immediately after MaxDuplicateRetries

TESTS (internal/config/config_test.go):
  - TestDefaults: +2 assertions (c.MultiTurnFallback==true, c.MultiTurnChunkTokens==32000)

CONSUMED BY (informational — downstream tasks):
  - P1.M1.T2.S2: fileGeneration struct + materialize() + overlay() copy the two fields (file-decode layer)
  - P1.M1.T3.S2: the N+1 turn protocol reads cfg.MultiTurnChunkTokens (chunk sizing)
  - P1.M1.T3.S3: the FR-T1 trigger gate reads cfg.MultiTurnFallback (condition (c))

NO-TOUCH (explicitly):
  - internal/config/file.go (fileGeneration/materialize/overlay = S2)
  - internal/config/{git,load}.go (unaffected)
  - docs/* (the [generation] table doc = S3)
  - internal/provider/* (S5 — parallel), internal/generate/* (T3)
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by LATER subtasks):
  - P1.M1.T2.S2: fileGeneration gains the same 2 fields; materialize copies them; overlay merges them (plain bool/int copy semantics)
  - P1.M1.T2.S3: docs/configuration.md [generation] table gains the 2 rows; dedicated config unit tests (file/overlay) land
  - P1.M1.T3.S2/S3: the generate core reads the two cfg fields
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/config/   # Expected: empty (run gofmt -w if listed — re-aligns the struct/Defaults columns)
go vet ./internal/config/... # Expected: exit 0
go build ./...               # Expected: exit 0 (adding fields + defaults breaks nothing)

# Expected: Zero errors. The build across the repo confirms every Defaults() caller still compiles.
```

### Level 2: Unit Tests (TestDefaults pins the new defaults)

```bash
cd /home/dustin/projects/stagecoach

go test ./internal/config/ -v -run TestDefaults

# Expected: PASS. The 2 new assertions hold: MultiTurnFallback==true, MultiTurnChunkTokens==32000.
# Every existing TestDefaults assertion still holds (the change is purely additive).

go test ./internal/config/   # Expected: ok (the whole config suite is green)
```

### Level 3: Whole-Repository Regression (no collateral)

```bash
cd /home/dustin/projects/stagecoach

go test ./...    # Expected: ALL packages green (only the resolved Config gained 2 fields + defaults)
go vet ./...     # Expected: exit 0

# Confirm ONLY the 2 intended files changed
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/config/config.go + internal/config/config_test.go only. Nothing else.
```

### Level 4: Field-Presence + Default Cross-Check

```bash
cd /home/dustin/projects/stagecoach

# The two fields exist on the struct with the right types (plain bool/int) + TOML tags + FR citations.
grep -n 'MultiTurnFallback\b\|MultiTurnChunkTokens\b' internal/config/config.go
# Expected: 2 struct-field matches + 2 Defaults() matches (4 total). No *bool/*int.

grep -n 'multi_turn_fallback\|multi_turn_chunk_tokens' internal/config/config.go
# Expected: the 2 snake_case TOML tags.

# Defaults() sets the documented PRD values.
grep -A0 'MultiTurnFallback:.*true\|MultiTurnChunkTokens:.*32000' internal/config/config.go
# Expected: both default entries (true / 32000).
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test ./...` — all packages green.

### Feature Validation

- [ ] `Config` struct has `MultiTurnFallback bool` (toml `multi_turn_fallback`) + `MultiTurnChunkTokens int` (toml `multi_turn_chunk_tokens`), immediately after `MaxDuplicateRetries`.
- [ ] Each field's comment cites §9.24 FR-T1c / FR-T3 + names its downstream consumer.
- [ ] `Defaults()` sets `MultiTurnFallback: true` + `MultiTurnChunkTokens: 32000`.
- [ ] `TestDefaults` asserts both defaults (true / 32000).

### Scope Discipline Validation

- [ ] ONLY `internal/config/config.go` + `internal/config/config_test.go` modified (`git diff --stat` confirms).
- [ ] Did NOT make the fields `*bool`/`*int` (plain types — the pointer pattern is S2's file-decode layer).
- [ ] Did NOT edit `internal/config/file.go` (S2), `docs/*` (S3), `internal/provider/*` (S5), or any other package.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research note).

### Code Quality Validation

- [ ] Plain `bool`/`int` types match the resolved-Config discipline (AutoStageAll/MaxDuplicateRetries precedents).
- [ ] TOML tags match §16.2 verbatim (`multi_turn_fallback`, `multi_turn_chunk_tokens`).
- [ ] gofmt re-aligns the struct/Defaults columns; no hand-alignment.
- [ ] TestDefaults assertions mirror the existing `c.MaxDuplicateRetries != 3` idiom (plain if/t.Errorf, no testify).

---

## Anti-Patterns to Avoid

- ❌ Don't make the fields `*bool`/`*int`. The resolved Config is flat/plain-typed (config.go:47-61). The
  pointer pattern is the file-decode layer's (fileGeneration) mechanism for absent-vs-explicit-zero/false
  during overlay — that is S2's concern. AutoStageAll (bool, true) is the precedent. (gotcha G1)
- ❌ Don't place the fields anywhere other than immediately after `MaxDuplicateRetries`. The contract pins
  the location; §16.2's [generation] table orders them right after max_duplicate_retries. (G3)
- ❌ Don't hand-align the struct/Defaults columns — run `gofmt -w`; it re-aligns after the bool/int insert. (G2)
- ❌ Don't invent TOML tag names — use §16.2's exact `multi_turn_fallback` / `multi_turn_chunk_tokens`. (G4)
- ❌ Don't skip the TestDefaults assertions. The existing TestDefaults pins every scalar default; leaving the
  new ones unverified is inconsistent + fragile. S3 owns the file/overlay tests (no overlap). (G6)
- ❌ Don't edit `internal/config/file.go` (S2 = fileGeneration/materialize/overlay), `docs/*` (S3),
  `internal/provider/*` (S5), or `internal/generate/*` (T3). This task is the resolved Config only. (G7)
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a minimal, fully-prescribed change — two plain-typed struct fields + two Defaults()
entries + two TestDefaults assertions — with every detail pinned to verified live state (the struct lines
config.go:83-84 and Defaults lines :176-177 quoted verbatim, the TestDefaults lines :46-51 quoted verbatim,
the target for each copy-paste-ready). The fields are confirmed ABSENT (genuine add), the baseline is GREEN,
and the type choice is dictated by the resolved-Config discipline (plain bool/int, with AutoStageAll and
MaxDuplicateRetries as the in-file precedents — eliminating the one plausible over-edit of using *bool/*int
pointers, which is S2's file-decode concern). Adding fields + defaults to a Go struct is provably
backward-compatible (zero-value; every `Defaults()` caller and test continues to compile), so `go build ./...`
passing IS the regression proof. The prior parallel PRP (S5) is internal/provider + docs with zero
internal/config overlap. The residual 0.5 uncertainty is purely gofmt column re-alignment (cosmetic, gated
by `gofmt -l .`). The S2 (file-decode), S3 (docs + dedicated tests), and T3 (generate-core) boundaries are
cleanly fenced downstream consumers that cannot be broken by adding two resolved-Config fields.
