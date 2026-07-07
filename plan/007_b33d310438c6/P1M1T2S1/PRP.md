---
name: "P1.M1.T2.S1 — StagedDiffOptions fields (TokenLimit, DiffContext, PromptReserveTokens)"
description: |
  Add three fields to the `StagedDiffOptions` struct in `internal/git/git.go` (lines 36-44):
  `TokenLimit int` (FR3d holistic token budget; 0 = unset ⇒ legacy per-section caps), `DiffContext int`
  (FR3f unified-context line count 0–3; the RESOLVED value — 0 is valid = -U0), and
  `PromptReserveTokens int` (FR3i stable prompt-portion cost measured upstream for the water-fill).
  Each documented with its FR citation + source/consumer; a prominent guard-note that DiffContext==0 is
  VALID (-U0) and callers must pass the resolved value (default 1) explicitly. Pure struct-field
  addition — the fields are UNREAD by the three diff functions until M2 (DiffContext) and M4
  (TokenLimit/PromptReserveTokens). No behavior change; no test (unread fields have nothing to assert).
  Threads the v2.1 diff-overlay seam so T2.S2 can map cfg → opts at the 6 call sites in one pass.
---

## Goal

**Feature Goal**: Extend `StagedDiffOptions` (`internal/git/git.go:36-44`) with the three v2.1
diff-payload-optimization fields — `TokenLimit`, `DiffContext`, `PromptReserveTokens` — so the seam
between the config layer (S1/S2, landed: `cfg.TokenLimit int` + `cfg.DiffContext *int`) and the git
diff functions is in place for the downstream consumption tasks (M2 reads `DiffContext`; M4 reads
`TokenLimit`/`PromptReserveTokens`) and the call-site mapping task (T2.S2) can thread all three from
config in a single pass.

**Deliverable**: ONE struct edit — three new fields added to `StagedDiffOptions` in
`internal/git/git.go`, each with an FR-cited doc comment, grouped under a brief v2.1 header that
explains why they are (intentionally) unread yet, plus the prominent `DiffContext==0` is valid
(-U0) guard-note. No other file touched. No test added.

**Success Definition**: `StagedDiffOptions` has the three fields with correct types (all plain `int`)
and doc comments; the struct still compiles; the existing ~57 test call sites and 6 production call
sites compile unchanged (new fields default to 0); `go build ./...`, `go vet ./...`, `gofmt -l .`, and
`go test ./...` are all green. No diff-function behavior changes (the fields are unread).

## User Persona

**Target User**: The contributors implementing the downstream diff-payload tasks — T2.S2 (map cfg → opts at the 6 call sites), M2.T2 (inject `-U<DiffContext>` via the flag helper), and M4.T2/T4 (the water-fill + token-limit gate that reads `TokenLimit`/`PromptReserveTokens`).

**Use Case**: When those tasks land, they reference `opts.TokenLimit` / `opts.DiffContext` / `opts.PromptReserveTokens` knowing the fields exist, are correctly typed, and carry the resolved values. This task puts the fields in place first so the downstream tasks have a single, stable struct to read.

**User Journey**: config fields (`cfg.TokenLimit`, `cfg.DiffContext`) → T2.S2 maps them into `StagedDiffOptions{...}` (resolving `*cfg.DiffContext` with default-1 fallback) → StagedDiff/TreeDiff/WorkingTreeDiff read `opts.DiffContext` (M2) / `opts.TokenLimit`+`opts.PromptReserveTokens` (M4). This task is the struct-field prerequisite.

**Pain Points Addressed**: Avoids a coupled mega-edit where the struct, the 6 call sites, and the consumption logic all land at once. By threading the seam now (struct fields only, unread), T2.S2 and M2/M4 become independent, smaller, parallelizable changes.

## Why

- **PRD §9.1 FR3d/FR3f/FR3i mandate the three knobs.** FR3d (`token_limit` holistic overlay), FR3f (`diff_context` reduced `-U<n>`), FR3i (dynamic water-fill truncation with `body_budget = token_limit − skeleton − promptReserve`). All three ride on `StagedDiffOptions` (the single options struct consumed by all three diff paths — FR3c parity), so the struct is the natural home.
- **The config seam is already landed (S1/S2 Complete).** `config.TokenLimit int` + `config.DiffContext *int` (+ Defaults + materialize + overlay + git-config keys) exist. This task is the git-side counterpart: the struct fields that receive those resolved values. Without them, T2.S2 has nowhere to map.
- **Decouples the struct from its consumers.** Adding unread fields is the lowest-risk way to thread a seam: no behavior can change (Go zero-value), every existing caller still compiles, and the downstream M2/M4 tasks can independently write their consumption logic against a stable field set.
- **`PromptReserveTokens` is the clean upstream/downstream boundary (system_context.md §5).** The git layer owns the diff body + numstat sizing; the prompt portion is measured upstream (prompt/generate layers) and passed in via this field. This keeps the git layer free of `internal/prompt` imports and prompt-construction concerns.
- **No user-facing/docs surface (contract: "DOCS: none").** Pure internal struct field.

## What

Three new fields appended to `StagedDiffOptions` in `internal/git/git.go`, grouped under a brief v2.1
header comment, each with an FR-cited inline comment and (for `DiffContext`) the prominent guard-note.
All three are plain `int`. The three diff functions (StagedDiff/TreeDiff/WorkingTreeDiff) do NOT read
them yet — they are consumed by M2 (`DiffContext`) and M4 (`TokenLimit`/`PromptReserveTokens`). No
behavior change; no test.

### Success Criteria

- [ ] `StagedDiffOptions` has `TokenLimit int`, `DiffContext int`, `PromptReserveTokens int` (all plain `int`).
- [ ] Each field's doc comment cites its FR (FR3d / FR3f / FR3i) and names its config source + downstream consumer.
- [ ] `DiffContext`'s doc comment includes the guard-note: `0` is VALID (-U0); callers must pass the resolved value (default 1) explicitly (NOT a "0 means unset" sentinel).
- [ ] A brief group header explains the three fields are intentionally unread until M2/M4 (the seam-threading rationale).
- [ ] The three diff functions (StagedDiff/TreeDiff/WorkingTreeDiff) are UNCHANGED — they do not read the new fields.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test ./...` green (existing tests unaffected).
- [ ] No file other than `internal/git/git.go` is modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the current `StagedDiffOptions` struct verbatim (4 fields, lines
36-44), gives the exact target struct (3 new fields + comments, ready to paste), explains the subtle
type choice (plain `int` here vs `*int` at the config layer for DiffContext), states the FR semantics
for each field, names the config source and downstream consumer, and confirms the fields are absent
today (genuine add) + the baseline is green. No inference required.

### Documentation & References

```yaml
# MUST READ — the binding knob specs
- file: PRD.md
  why: "§9.1 FR3d (token_limit holistic overlay; 0=unset⇒legacy caps; mutually exclusive with per-section caps), FR3f (diff_context reduced -U<n>, 0–3, default 1), FR3i (dynamic water-fill: body_budget = token_limit − skeleton − prompt − margin). These three FRs ARE the field semantics."
  critical: "FR3d's '0/unset ⇒ legacy caps' is why TokenLimit is a plain int with 0 as its unset sentinel. FR3f's '0 = changed lines only' is why DiffContext==0 is VALID and must not be treated as unset. FR3i's 'body_budget = token_limit − skeleton − prompt − margin' is what PromptReserveTokens feeds."

- docfile: plan/007_b33d310438c6/architecture/diff_capture_touchmap.md
  why: "§2 is the authoritative spec for THIS edit: the verbatim current StagedDiffOptions struct (4 fields, git.go:36-44), the instruction 'Add TokenLimit int and DiffContext int here', the 6 production call sites that will map cfg→opts (T2.S2), and the PromptReserveTokens seam rationale (§5: 'the git layer owns the diff body + numstat sizing; the prompt portion is measured upstream and passed in')."
  critical: "§2 confirms the struct is the single home for the three knobs (consumed by all three diff paths, FR3c parity). §5 confirms PromptReserveTokens is the upstream/downstream seam. The 'Open coupling question' in §5 is exactly what PromptReserveTokens resolves (worst-case reserve measured upstream, passed in)."

- docfile: plan/007_b33d310438c6/architecture/system_context.md
  why: "§5 defines the PromptReserveTokens seam: the git layer owns the diff body + numstat sizing; the prompt portion is measured upstream and passed in. Confirms the field's role as the boundary that keeps internal/git free of internal/prompt imports."
  critical: "This is the architectural justification for PromptReserveTokens being a plain int field on StagedDiffOptions (received, not computed, by the git layer)."

- docfile: plan/007_b33d310438c6/P1M1T2S1/research/stageddiffoptions_fields_notes.md
  why: "EMPIRICALLY VERIFIED (live code, 2026-07-04): the 3 fields are ABSENT (genuine add); the config layer has cfg.TokenLimit (int) + cfg.DiffContext (*int, default intPtr(1)); baseline git tests GREEN; the type-choice rationale (plain int here vs *int at config for DiffContext); why no test; the placement/style decision; decisions D1–D6. READ THIS FIRST."
  critical: "§2 (the three fields' semantics + type + source + consumer) and §4 (why no test) prevent the two most likely over-edits: (a) making DiffContext a *int here (it must be plain int — the resolved value), and (b) adding a tautological test for unread fields."

- file: internal/git/git.go
  why: "THE edit target. The StagedDiffOptions struct (lines 36-44) is where the 3 fields go. The three diff functions (StagedDiff:642, TreeDiff:1094, WorkingTreeDiff:1228) consume opts but are UNCHANGED by this task (they do not read the new fields). The struct's existing style is concise inline comments."
  pattern: "Inline `// comment` per field (MaxDiffBytes/MaxMdLines/Excludes/BinaryExtensions). A multi-line comment block follows BinaryExtensions explaining the denylist sourcing. The new fields append after that block, grouped under a brief v2.1 header."
  gotcha: "Do NOT edit the three diff functions, the Git interface, or any other part of git.go. This task is the struct fields ONLY. gofmt re-aligns the struct if needed — run gofmt -w."

- file: internal/config/config.go
  why: "READ-ONLY ref (the config source, S1 landed). config.TokenLimit int (line 81, plain; 0=unset) and config.DiffContext *int (line 82, pointer; nil=unset, non-nil incl. *0=explicit; default intPtr(1) at line 175). Confirms the field names + the *int-vs-int distinction that the DiffContext doc comment must explain."
  pattern: "The config layer's *int for DiffContext exists to distinguish 'user omitted the key' (nil → default 1) from 'user set 0' (*0 → -U0). StagedDiffOptions takes the RESOLVED value (plain int) — the call site (T2.S2) dereferences with a default-1 fallback."

- docfile: plan/007_b33d310438c6/P1M1T1S4/PRP.md
  why: "The parallel sibling (bootstrap config template + docs/CONFIGURATION.md). Confirms NO internal/git/git.go overlap — S4 touches only config/docs files, this task touches only the git struct. No conflict."
  critical: "S4 may add commented `token_limit`/`diff_context` lines to the bootstrap template; that is config-side documentation and does not interact with this struct-field addition."

# External references
- url: https://go.dev/ref/spec#Struct_types
  why: "Confirms adding fields to a struct is backward-compatible: existing struct literals (named or positional) continue to compile; new fields take their zero value. This is WHY the ~57 test call sites and 6 production call sites need no changes for this task."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/git/
    └── git.go    # EDIT TARGET — StagedDiffOptions struct (lines 36-44); 3 diff functions unchanged
# (internal/config/* is READ-ONLY — the config source, S1/S2 landed)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only one existing file modified — no new files)
    internal/git/git.go   # +3 fields on StagedDiffOptions (+ doc comments + group header)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY (StagedDiffOptions struct only) | Add `TokenLimit int`, `DiffContext int`, `PromptReserveTokens int` with FR-cited doc comments + the DiffContext guard-note + a brief group header. **Only edit.** |

**Explicitly NOT touched**: the three diff functions (StagedDiff/TreeDiff/WorkingTreeDiff — they do not
read the new fields), the `Git` interface, any other `internal/git/*` file, `internal/config/*`
(S1/S2/S4), `internal/generate/*`, `internal/decompose/*`, `internal/hook/*`, `pkg/stagecoach/*`
(the 6 call sites = T2.S2), any docs, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — DiffContext is PLAIN int here, NOT *int): the config layer uses *int (config.DiffContext)
// to distinguish "user omitted the key" (nil → default 1) from "user set 0" (*0 → -U0). StagedDiffOptions
// takes the RESOLVED value — plain int — because the git layer has no "unset" state for context: 0 means
// -U0 (valid). The *int→int dereference (nil→1, *0→0) is the CALL SITE's job (T2.S2), NOT this struct's.
// Do NOT make StagedDiffOptions.DiffContext a *int — it would push config-resolution logic into the git
// layer and violate the "git takes resolved values" seam.

// CRITICAL (G2 — DiffContext==0 is VALID, document the guard-note): 0 means -U0 (changed lines only),
// NOT "unset". The field's doc comment MUST say so prominently, and MUST say callers pass the resolved
// value (default 1) explicitly. Without this note, a future consumer might write `if opts.DiffContext > 0`
// (skipping -U0) or treat 0 as "use git default" — both wrong. The contract requires this guard-note.

// CRITICAL (G3 — TokenLimit==0 is the unset sentinel, by design): FR3d says 0/unset ⇒ legacy per-section
// caps. Unlike DiffContext, 0 here genuinely means "unset" (there is no meaningful "explicit 0 token
// budget"). So TokenLimit is a plain int with 0=unset. Do NOT add a *int for TokenLimit (the config layer
// uses plain int too — they match). The M4 gate reads `if opts.TokenLimit > 0` to switch modes.

// GOTCHA (G4 — the fields are UNREAD; do NOT wire consumption): this task adds the struct fields ONLY.
// The three diff functions (StagedDiff:642, TreeDiff:1094, WorkingTreeDiff:1228) must NOT be edited to
// read them — that is M2 (DiffContext → -U<n>) and M4 (TokenLimit/PromptReserveTokens → gate + water-fill).
// Wiring consumption now would couple this task to M2/M4 and risk behavior change. The contract: "No
// behavior change yet."

// GOTCHA (G5 — no test for unread fields): adding fields that no code reads has no observable behavior.
// A "struct has these fields" test is tautological. The real coverage lands in M2/M4. Do NOT add a test
// here — the existing suite staying green IS the validation. (The contract: "No behavior change yet.")

// GOTCHA (G6 — gofmt re-aligns the struct): adding fields may shift the column alignment of the existing
// inline comments. Run `gofmt -w internal/git/git.go` after the edit; do NOT hand-align (gofmt is
// authoritative). The existing fields' comment text is unchanged; only the whitespace column may shift.

// GOTCHA (G7 — scope discipline): ONLY the StagedDiffOptions struct. T2.S2 owns the 6 call-site mappings
// (cfg → opts); M2 owns -U<n> injection; M4 owns the gate + water-fill; S4 owns bootstrap config. Editing
// any of those here crosses the subtask boundary and risks conflicting with parallel/landed work.
```

## Implementation Blueprint

### Data models and structure

No new types. The three fields are plain `int` additions to the existing `StagedDiffOptions` struct.
The "model" facts are the type choices (all plain int) and the semantics each carries (FR3d/FR3f/FR3i).

### The struct edit (exact — current → target)

**Current** (`internal/git/git.go:36-44`):
```go
// StagedDiffOptions configures staged-diff capture (commit-pi parity, PRD §9.1 / FINDING 7).
// The T3.S1 (StagedDiff) implementation consumes these.
type StagedDiffOptions struct {
	MaxDiffBytes     int      // byte cap on the non-markdown section (commit-pi default 300000); 0 = unlimited
	MaxMDLines       int      // per-file line cap for markdown files (commit-pi default 100); 0 = unlimited
	Excludes         []string // pathspec magic-prefix excludes, e.g. []string{":!*.lock", ":!vendor/*"}
	BinaryExtensions []string // extra non-text extensions to filter beyond the built-in denylist
	// (png jpg … woff2 in internal/git/binary.go); nil ⇒ built-in denylist only.
	// Entries are dot-tolerant + case-insensitive (PRD §9.1 FR3a).
	// Sourced from config `binary_extensions`.
}
```

**Target** (append the v2.1 field group after the BinaryExtensions comment block):
```go
// StagedDiffOptions configures staged-diff capture (commit-pi parity, PRD §9.1 / FINDING 7).
// The T3.S1 (StagedDiff) implementation consumes these.
type StagedDiffOptions struct {
	MaxDiffBytes     int      // byte cap on the non-markdown section (commit-pi default 300000); 0 = unlimited
	MaxMDLines       int      // per-file line cap for markdown files (commit-pi default 100); 0 = unlimited
	Excludes         []string // pathspec magic-prefix excludes, e.g. []string{":!*.lock", ":!vendor/*"}
	BinaryExtensions []string // extra non-text extensions to filter beyond the built-in denylist
	// (png jpg … woff2 in internal/git/binary.go); nil ⇒ built-in denylist only.
	// Entries are dot-tolerant + case-insensitive (PRD §9.1 FR3a).
	// Sourced from config `binary_extensions`.

	// --- v2.1 diff-payload optimization (PRD §9.1 FR3d/FR3f/FR3i) ---
	// These overlay the legacy per-section caps above. UNREAD by the three diff functions until M2
	// (DiffContext) and M4 (TokenLimit/PromptReserveTokens) — added now to thread the seam so the 6
	// call sites (T2.S2) can map cfg → opts in one pass. No behavior change from this field set.

	// FR3d: holistic token budget over the WHOLE payload (system prompt + style examples + diff).
	// 0 = unset ⇒ the legacy MaxDiffBytes/MaxMDLines per-section caps apply unchanged (the two modes
	// are mutually exclusive: a non-zero TokenLimit supersedes both). Sourced from config `token_limit`
	// (a plain int — 0 IS its unset sentinel; no meaningful "explicit 0"). Read by the M4 gate + water-fill.
	TokenLimit int

	// FR3f: unified-context line count for `git diff -U<n>` (0–3). Reduces git's -U3 default to cut
	// unchanged-context noise.
	// ⚠️ 0 is VALID (-U0 = changed lines only) — this is a PLAIN int (not *int) because the git layer
	// takes the RESOLVED value: callers MUST pass the resolved context (default 1 when the user omits
	// it) explicitly, NEVER a "0 means unset" sentinel. Sourced from config `diff_context` (a *int at
	// the config layer to distinguish unset from explicit 0; the call site dereferences with a
	// default-1 fallback before constructing this struct). Read by M2's flag helper.
	DiffContext int

	// FR3i: token cost of the STABLE prompt portion — system-prompt header + style examples (FR11) +
	// user instruction + worst-case rejection block + margin — measured UPSTREAM (prompt/generate
	// layers) and passed in so the git layer can compute body_budget = token_limit − skeleton −
	// promptReserve for the dynamic water-fill. 0 = unset (no reserve subtracted); only meaningful
	// when TokenLimit > 0. The git layer RECEIVES this (it does not compute it — keeps internal/git
	// free of internal/prompt imports). Read by the M4 water-fill. See system_context.md §5.
	PromptReserveTokens int
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/git/git.go — append the v2.1 field group to StagedDiffOptions
  - FILE: internal/git/git.go
  - LOCATE: the StagedDiffOptions struct (lines 36-44). Find the closing of the BinaryExtensions comment
    block (the line `// Sourced from config `binary_extensions`.`) immediately before the struct's
    closing `}`.
  - INSERT (between that comment line and the closing `}`): the v2.1 group header + the three fields
    with their doc comments, EXACTLY as in "The struct edit" target above.
  - NAMING: `TokenLimit`, `DiffContext`, `PromptReserveTokens` (all plain `int`). The struct field names
    match the config field names (TokenLimit, DiffContext) so the call-site mapping is symmetric.
  - DOC COMMENTS: each field cites its FR (FR3d/FR3f/FR3i), names its config source, and names its
    downstream consumer (M2/M4). The DiffContext comment carries the prominent guard-note (⚠️ 0 is
    VALID = -U0; callers pass the resolved value). The group header explains the fields are unread
    until M2/M4 (the seam rationale).
  - DO NOT: edit the three diff functions (StagedDiff/TreeDiff/WorkingTreeDiff), the Git interface, or
    any other part of git.go. DO NOT add consumption logic. DO NOT make DiffContext a *int.
  - RUN: gofmt -w internal/git/git.go   (re-aligns the struct's comment column if needed)
  - VERIFY: go build ./internal/git/  → exit 0.

Task 2: VALIDATE (the fields are unread; the gates confirm no behavior change)
  - RUN: go build ./...     # the struct change compiles across the repo (all call sites still valid)
  - RUN: go vet ./...       # exit 0
  - RUN: gofmt -l .         # empty
  - RUN: go test ./...      # ALL green — the ~57 StagedDiffOptions test literals + 6 production call
                             # sites compile unchanged (new fields default to 0); no behavior change.
  - RUN targeted: go test ./internal/git/   # the diff-function tests stay green (fields unread).
  - FIX-FORWARD: a compile failure would mean a typo in a field name/type — read the error, fix, re-run.
```

### Implementation Patterns & Key Details

```go
// === Why all three are plain int (and DiffContext is NOT *int here) ===
// TokenLimit: matches config (plain int); 0 = unset sentinel (FR3d — no meaningful "explicit 0").
// DiffContext: PLAIN int here even though config uses *int. The git layer takes the RESOLVED value —
//   0 means -U0 (valid), never "unset". The *int→int dereference (nil→1, *0→0) is the call site's job.
//   Making it *int here would push config-resolution into the git layer and violate the seam.
// PromptReserveTokens: a measured count received from upstream; 0 = unset (no reserve subtracted).

// === Why the fields are unread (the seam-threading pattern) ===
// Adding unread fields is the lowest-risk way to thread a seam: Go zero-value, every existing caller
// still compiles, no behavior can change. T2.S2 (call-site mapping) and M2/M4 (consumption) become
// independent downstream tasks against a stable field set. This is the standard "land the type, then
// land the consumers" decomposition.

// === Why no test (unread fields have nothing to assert) ===
// An unread struct field has no observable behavior. The existing diff tests stay green because the
// diff functions are unchanged. A "struct has field X" test is tautological. M2 adds -U<n> argv
// assertions; M4 adds gate + water-fill assertions. This task's validation is: builds + existing suite green.

// === Why the DiffContext guard-note matters ===
// Without "0 is valid (-U0); callers pass the resolved value explicitly", a future consumer might:
//   (a) write `if opts.DiffContext > 0` — silently dropping -U0 (a legitimate FR3f value); or
//   (b) treat 0 as "use git default -U3" — wrong (0 means changed-lines-only).
// The guard-note + the plain-int type force the call site to resolve explicitly (default 1).
```

### Integration Points

```yaml
STRUCT (internal/git/git.go:StagedDiffOptions):
  - +TokenLimit int          (FR3d; source: cfg.TokenLimit; consumer: M4 gate + water-fill)
  - +DiffContext int         (FR3f; source: *cfg.DiffContext resolved at call site; consumer: M2 flag helper)
  - +PromptReserveTokens int (FR3i; source: measured upstream; consumer: M4 water-fill)
  - the three diff functions UNCHANGED (do not read the new fields)

NO-TOUCH (explicitly — owned by sibling/downstream subtasks):
  - internal/git/git.go diff functions (StagedDiff/TreeDiff/WorkingTreeDiff)   # consumption = M2/M4
  - internal/git/* other files (binary.go, etc.)                               # unaffected
  - internal/config/* (config.go, file.go, git.go, bootstrap.go)              # S1/S2/S4 (landed/parallel)
  - the 6 production call sites (generate/hook/stagecoach/decompose)           # T2.S2 (the mapping task)
  - internal/prompt/*                                                         # M4.T1.S2 measures PromptReserveTokens here
  - any docs (README.md, docs/*)                                              # contract: no docs surface
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by LATER subtasks, NOT this one):
  - P1.M1.T2.S2: maps cfg.TokenLimit/cfg.DiffContext (+ measured PromptReserveTokens) into the 6 call-site struct literals
  - P1.M2.T2 (FR3f): the flag helper reads opts.DiffContext → injects `-U<opts.DiffContext>` into the diff argv
  - P1.M4.T3 (FR3d): the token-limit gate reads opts.TokenLimit → switches off legacy caps when >0
  - P1.M4.T2 (FR3i): the water-fill reads opts.TokenLimit + opts.PromptReserveTokens → body_budget = token_limit − skeleton − promptReserve
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/git/git.go     # Expected: empty (run gofmt -w if listed — re-aligns the comment column)
go vet ./internal/git/...        # Expected: exit 0
go build ./internal/git/         # Expected: exit 0
go build ./...                   # Expected: exit 0 (the struct change compiles across all call sites)

# Expected: Zero errors. The build passing across the repo confirms every StagedDiffOptions literal
# (test + production) still compiles with the new fields defaulting to 0.
```

### Level 2: Unit Tests (no behavior change — existing suite stays green)

```bash
cd /home/dustin/projects/stagecoach

go test ./internal/git/          # Expected: ok — the diff-function tests are unchanged (fields unread)
go test ./...                    # Expected: ALL packages green (no call site changed; fields are zero-value)

# Expected: all green. The ~57 StagedDiffOptions test literals and 6 production call sites compile
# unchanged. No new test is added (unread fields have nothing to assert — see Gotcha G5).
```

### Level 3: Whole-Repository Regression (no collateral)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages green (only a struct field was added)
go vet ./...                     # Expected: exit 0

# Confirm ONLY internal/git/git.go changed
git diff --stat -- internal/ pkg/ cmd/
# Expected: only internal/git/git.go (the StagedDiffOptions struct). No other file.

# Confirm the config layer (S1/S2) and the diff functions are UNTOUCHED
git diff --stat -- internal/config/ internal/git/binary.go
# Expected: EMPTY (config is S1/S2's landed territory; binary.go is unrelated).
git diff -- internal/git/git.go | grep -E '^\+.*func .*Staged|^\+.*func .*TreeDiff|^\+.*func .*WorkingTreeDiff' || echo "OK: diff functions unchanged"
```

### Level 4: Field-Presence Cross-Check (prove the seam is threaded)

```bash
cd /home/dustin/projects/stagecoach

# The three fields exist on the struct with the right types (plain int), are cited by FR, and the
# DiffContext guard-note is present.
grep -n 'TokenLimit int\|DiffContext int\|PromptReserveTokens int' internal/git/git.go
# Expected: three matches inside StagedDiffOptions (all plain `int`, no `*int`).

grep -n 'FR3d\|FR3f\|FR3i' internal/git/git.go
# Expected: the three FR citations in the new field doc comments.

grep -n '0 is VALID (-U0)\|resolved value' internal/git/git.go
# Expected: the DiffContext guard-note is present.

# The diff functions do NOT read the new fields (unread — consumption is M2/M4).
for f in StagedDiff TreeDiff WorkingTreeDiff; do
  echo "--- $f reads of the new fields (expect none):"
  awk "/func \\(g \\*gitRunner\\) $f\\(/,/^}/" internal/git/git.go | grep -n 'opts\.TokenLimit\|opts\.DiffContext\|opts\.PromptReserveTokens' || echo "  (none — $f does not read the new fields; correct)"
done
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test ./...` (and `go test -race ./...`) — all packages green.

### Feature Validation

- [ ] `StagedDiffOptions` has `TokenLimit int`, `DiffContext int`, `PromptReserveTokens int` (all plain `int`).
- [ ] Each field's doc comment cites its FR (FR3d / FR3f / FR3i) and names its config source + downstream consumer.
- [ ] `DiffContext`'s doc comment includes the guard-note: `0` is VALID (-U0); callers pass the resolved value (default 1) explicitly.
- [ ] A group header explains the three fields are intentionally unread until M2/M4.
- [ ] The three diff functions (StagedDiff/TreeDiff/WorkingTreeDiff) do NOT read the new fields (Level 4 check).

### Scope Discipline Validation

- [ ] ONLY `internal/git/git.go` modified, and ONLY the `StagedDiffOptions` struct (`git diff --stat` confirms).
- [ ] Did NOT edit the three diff functions, the `Git` interface, or any other `internal/git/*` file.
- [ ] Did NOT make `DiffContext` a `*int` (it is plain `int` — the resolved value).
- [ ] Did NOT wire consumption (M2/M4 territory) or add a test (unread fields have nothing to assert).
- [ ] Did NOT touch `internal/config/*` (S1/S2/S4), the 6 call sites (T2.S2), any docs, `PRD.md`, `tasks.json`,
      `prd_snapshot.md`, or anything under `plan/` (except this PRP + research note).

### Code Quality Validation

- [ ] Field names match the config field names (TokenLimit, DiffContext) for symmetric call-site mapping.
- [ ] Doc comments match the struct's existing concise inline style (with the necessary guard-note detail).
- [ ] gofmt re-aligns the struct; no hand-alignment.
- [ ] The seam-threading rationale (group header) explains why unread fields exist — prevents future "cleanup."

---

## Anti-Patterns to Avoid

- ❌ Don't make `DiffContext` a `*int` on `StagedDiffOptions` — the git layer takes the RESOLVED value (plain
  int). The config layer's `*int` distinguishes unset from explicit 0; the call site (T2.S2) dereferences
  with a default-1 fallback. Pushing `*int` into the git layer violates the seam (gotcha G1).
- ❌ Don't omit the `DiffContext==0 is VALID (-U0)` guard-note — without it a future consumer may write
  `if opts.DiffContext > 0` (dropping -U0) or treat 0 as "git default" (wrong). The contract requires it (G2).
- ❌ Don't make `TokenLimit` a `*int` — FR3d says 0/unset ⇒ legacy caps; 0 IS the unset sentinel (no
  meaningful "explicit 0"). It matches the config layer's plain `int`. The M4 gate reads `if > 0` (G3).
- ❌ Don't wire consumption (don't edit the diff functions to read the fields) — that is M2 (DiffContext →
  -U<n>) and M4 (TokenLimit/PromptReserveTokens → gate + water-fill). This task is the struct fields ONLY.
  The contract: "No behavior change yet" (G4).
- ❌ Don't add a test — unread struct fields have no observable behavior; a "struct has field X" test is
  tautological. M2/M4 add the real coverage. Existing suite green IS the validation (G5).
- ❌ Don't hand-align the struct — run `gofmt -w`; it re-aligns the comment column (G6).
- ❌ Don't edit `internal/config/*` (S1/S2/S4), the 6 call sites (T2.S2), `internal/prompt/*` (M4.T1.S2),
  any docs, or any other package — this task is the `StagedDiffOptions` struct only (G7).
- ❌ Don't cite the wrong FR or omit the FR citation — each field's semantics IS its FR (FR3d/FR3f/FR3i);
  the citation is how a reader finds the rationale.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a minimal, fully-prescribed struct-field addition — three plain-`int` fields with
doc comments — to a struct quoted verbatim (current 4 fields → target 7 fields, ready to paste). The
fields are confirmed ABSENT today (genuine add), the baseline is GREEN, and the config source fields
(`cfg.TokenLimit int`, `cfg.DiffContext *int`) are verified landed (S1/S2 Complete). Adding unread
fields to a Go struct is provably backward-compatible (zero-value; every existing literal compiles), so
the ~57 test call sites and 6 production call sites need no changes — `go build ./...` passing IS the
regression proof. The two most likely over-edits — making `DiffContext` a `*int` here (it must be plain
int; the resolved value) and wiring consumption into the diff functions (this task is fields-only) — are
front-loaded as CRITICAL gotchas (G1, G4), and the DiffContext guard-note (0 is valid = -U0) is
explicitly required and checked (G2, Level 4). The "no test" decision is explained (G5) so the
implementer does not add tautological churn. The prior parallel PRP (S4) is bootstrap config with zero
`internal/git` overlap. The residual 0.5 uncertainty is purely gofmt column re-alignment and
doc-comment wording taste, both caught by the deterministic `gofmt -l .` + `go test ./...` gates and
both cosmetic. T2.S2 (the call-site mapping) and M2/M4 (consumption) are cleanly fenced downstream
consumers that cannot be broken by adding unread fields.
