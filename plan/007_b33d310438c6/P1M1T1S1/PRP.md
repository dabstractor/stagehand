---
name: "P1.M1.T1.S1 — Config struct fields + Defaults() + fileGeneration struct (token_limit + diff_context)"
description: |
  Pure scaffolding (FR3d/FR3f config plumbing, step 1 of 4). Add two flat int fields to `config.Config`
  (`TokenLimit toml:"token_limit"`, `DiffContext toml:"diff_context"`) next to MaxDiffBytes/MaxMdLines;
  add the SAME two plain-int fields to the `fileGeneration` decode struct; seed `Defaults()` with
  `TokenLimit: 0` (FR3d unset ⇒ legacy caps) and `DiffContext: 1` (FR3f -U1 default). NOTHING reads them
  yet — materialize/overlay is S2, git-config keys S3, bootstrap/docs S4. Config.DiffContext stays a plain
  int in S1 (the fileGeneration `*int` 0-vs-unset disambiguation is S2's job). No behavior change; existing
  tests stay green. No docs in S1.
---

## Goal

**Feature Goal**: Land the first of four config-plumbing steps for the diff-payload optimizations
(FR3d token-budget overlay + FR3f reduced diff context): add the two new flat int fields to the resolved
`config.Config`, the `[generation]` decode struct (`fileGeneration`), and seed their PRD-mandated defaults
in `Defaults()`. The fields exist and are correctly TOML-tagged and defaulted but are **not yet consumed**
(materialize/overlay/git-config/bootstrap all land in S2–S4).

**Deliverable** (3 production edits in 2 files + 1 recommended test assertion):
1. `internal/config/config.go` — `Config` struct: add `TokenLimit int \`toml:"token_limit"\`` and
   `DiffContext int \`toml:"diff_context"\`` immediately after `MaxMdLines` (line 78).
2. `internal/config/file.go` — `fileGeneration` struct: add the SAME two plain-int fields after `MaxMdLines`
   (line 51).
3. `internal/config/config.go` — `Defaults()`: add `TokenLimit: 0,` (FR3d unset) and `DiffContext: 1,`
   (FR3f -U1) after `MaxMdLines: 100,` (line 169).
4. *(Recommended)* `internal/config/config_test.go` `TestDefaults`: add `TokenLimit==0` and `DiffContext==1`
   assertions (pins the seed values; matches the convention that every Defaults field is asserted).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green
with NO edits to any existing test (the new fields are unconsumed → no behavior change). The two fields
exist on both structs with correct TOML tags and the PRD defaults. No other package is touched; no docs.

## User Persona

**Target User**: The contributor implementing the immediately-following config-plumbing subtasks (S2
materialize/overlay with the `*int` DiffContext, S3 git-config keys, S4 bootstrap + docs) and the eventual
diff-capture consumers (P1.M1.T2 StagedDiffOptions, P1.M2+ the diff functions).

**Use Case**: Every downstream subtask references `cfg.TokenLimit` / `cfg.DiffContext`. S1 lands the field
definitions + defaults so those subtasks have stable struct members to wire.

**Pain Points Addressed**: Removes the "where do token_limit / diff_context live on Config, and what are
their defaults?" gap that would otherwise block S2/S3/S4 and the diff-function consumers.

## Why

- **First step of a 4-step config-plumbing chain.** The diff-payload optimizations (FR3d–FR3i) need two new
  config knobs. S1 is the struct/defaults foundation; S2 wires file→Config, S3 adds git-config keys, S4 the
  bootstrap template + docs. Splitting them keeps each subtask minimal and reviewable.
- **Pure scaffolding = lowest risk.** Nothing reads the fields yet, so there is literally no behavior
  change. The fields are dead until S2. Existing tests are field-specific (not exhaustive DeepEqual), so
  they stay green.
- **PRD-mandated defaults.** §16.1 layer-1 pins `token_limit 0` (unset ⇒ legacy caps, FR3d) and
  `diff_context 1` (-U1, FR3f). Defaults() must seed exactly these so the "unset" / reduced-context
  semantics hold before any consumer reads them.
- **Flat fields, not a nested struct.** Confirmed by `diff_capture_touchmap.md §3`: config fields are FLAT
  on `config.Config`; the `[generation]` block decodes into `fileGeneration`. The new fields mirror the
  existing `MaxDiffBytes`/`MaxMdLines` pair exactly.

## What

Three struct/seed additions in two files (plus a recommended test assertion). No logic, no consumers, no
docs. The new fields are unconsumed after S1 — `Config.DiffContext` always reads `1` (Defaults) and
`Config.TokenLimit` always reads `0` until S2 wires the file→Config path.

### Success Criteria

- [ ] `config.Config` has `TokenLimit int \`toml:"token_limit"\`` and `DiffContext int \`toml:"diff_context"\``
      immediately after `MaxMdLines` (flat, not nested).
- [ ] `fileGeneration` has the SAME two plain-int fields (NOT `*int` — that's S2) after `MaxMdLines`.
- [ ] `Defaults()` seeds `TokenLimit: 0` (FR3d unset) and `DiffContext: 1` (FR3f -U1), each commented with
      its FR citation.
- [ ] `TestDefaults` asserts `TokenLimit==0` and `DiffContext==1` (recommended — pins the seeds).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] NO edits to `materialize`/`overlay` (S2), git-config (S3), bootstrap/docs (S4), any consumer, or any
      other package.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact current `Config` fields (lines 77-80), the exact
`fileGeneration` struct (lines 49-62), the exact `Defaults()` block (lines 168-182), the precise insertion
points (after `MaxMdLines` in each), the verbatim new lines (with TOML tags + FR-citation comments), and
the `gofmt`-alignment note. The `*int`-vs-`int` boundary between S1 and S2 is stated explicitly so the
implementer doesn't pre-empt S2's design.

### Documentation & References

```yaml
# MUST READ — the config-field handoff (do not re-derive the placement)
- docfile: plan/007_b33d310438c6/architecture/diff_capture_touchmap.md
  why: "§3 states config fields are FLAT on config.Config (not a nested struct); the [generation] block decodes into fileGeneration. Confirms MaxDiffBytes/MaxMdLines is the existing pair to mirror."
  critical: "§3 is the authority that the new fields go flat on Config next to MaxDiffBytes, and that fileGeneration is the decode struct. The materialize/overlay wiring is OUT of S1 (S2)."

# The two files under edit
- file: internal/config/config.go
  why: "EDIT (2 spots). Config struct: insert TokenLimit + DiffContext after MaxMdLines (line 78), before MaxDuplicateRetries. Defaults(): insert TokenLimit:0 + DiffContext:1 after MaxMdLines:100 (line 169)."
  pattern: "Mirror the MaxDiffBytes/MaxMdLines pair exactly: flat int + toml snake_case tag + trailing //-comment citing the FR. Fields are column-aligned; gofmt realigns after the edit."
  gotcha: "Keep Config.DiffContext a PLAIN int in S1 (default 1). Do NOT make it *int — the 0-vs-unset disambiguation for the FILE struct is S2's job (fileGeneration.DiffContext becomes *int in S2). Config holds the resolved concrete int."

- file: internal/config/file.go
  why: "EDIT (1 spot). fileGeneration struct (lines 49-62): insert TokenLimit + DiffContext after MaxMdLines (line 51), before MaxDuplicateRetries."
  pattern: "Same two fields as Config, plain int, same toml tags. The struct already has MaxDiffBytes/MaxMdLines as the pair to mirror."
  gotcha: "Do NOT add materialize/overlay lines for these fields — that is S2 (P1.M1.T1.S2). S1 only adds the struct fields; they are decoded by go-toml but not yet copied into Config."

- file: internal/config/config_test.go
  why: "EDIT (recommended). TestDefaults (line 11) currently asserts MaxDiffBytes==300000, MaxMdLines==100, etc. ADD TokenLimit==0 and DiffContext==1 assertions — matches the convention that every Defaults field is pinned."
  pattern: "Field-specific `if c.X != want { t.Errorf(...) }` — the same idiom as the existing MaxDiffBytes/MaxMdLines assertions (lines 34-38). NOT an exhaustive DeepEqual."

# Read-only refs (do NOT edit in S1)
- docfile: plan/007_b33d310438c6/P1M1T1S1/research/s1_config_fields.md
  why: "Distilled S1 findings: exact edit targets with verbatim new lines, the Defaults values (FR3d/FR3f + §16.1), the S1-vs-S2 *int boundary, the no-test-breaks proof, and the gofmt note."

- file: internal/config/file.go # materialize (lines ~210-220) + overlay (lines ~312-322)
  why: "READ-ONLY ref — shows the `if g.MaxDiffBytes != 0 { c.MaxDiffBytes = g.MaxDiffBytes }` pattern S2 will replicate for TokenLimit (and, with *int, for DiffContext). S1 does NOT touch these."

# PRD authority (already in the selected content)
- prd: PRD.md §9.1 FR3d (token_limit, default 0 = unset) + FR3f (diff_context, default 1, range 0–3); §16.1 layer-1 (token_limit 0, diff_context 1).
  why: "The authoritative defaults + semantics. FR3d: 0/unset ⇒ legacy caps apply unchanged. FR3f: 1 = -U1 default; 0 = changed-lines-only; 3 = git default."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/config/
    ├── config.go        # EDIT: Config struct (+2 fields) + Defaults() (+2 seeds)
    ├── config_test.go   # EDIT (recommended): TestDefaults (+2 assertions)
    └── file.go          # EDIT: fileGeneration struct (+2 fields)
```

### Desired Codebase Tree After S1

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/config/config.go        # Config +TokenLimit +DiffContext; Defaults() +TokenLimit:0 +DiffContext:1
    internal/config/config_test.go   # TestDefaults +TokenLimit==0 +DiffContext==1 (recommended)
    internal/config/file.go          # fileGeneration +TokenLimit +DiffContext
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/config.go` | MODIFY | `Config` struct +2 flat int fields; `Defaults()` +2 seeds (0 and 1). |
| `internal/config/file.go` | MODIFY | `fileGeneration` struct +2 plain-int fields (decode surface). |
| `internal/config/config_test.go` | MODIFY (recommended) | `TestDefaults` +2 assertions pinning the seeds. |

**Explicitly NOT touched**: `materialize`/`overlay` (S2 = P1.M1.T1.S2), git-config resolver (S3 =
P1.M1.T1.S3), bootstrap template + `docs/CONFIGURATION.md` (S4 = P1.M1.T1.S4), `StagedDiffOptions`/call
sites (P1.M1.T2), the diff functions (P1.M2+), any other package, `PRD.md`, `tasks.json`,
`prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (S1/S2 boundary): Config.DiffContext is a PLAIN int in S1 (default 1). Do NOT make it *int.
// fileGeneration.DiffContext is ALSO a plain int in S1 (the contract: "the SAME two fields"). S2
// (P1.M1.T1.S2) changes fileGeneration.DiffContext to *int to disambiguate 0 (changed-lines-only) from
// unset — because for diff_context, 0 IS meaningful (unlike max_diff_bytes where 0 is never valid, so the
// existing `!= 0` materialize/overlay guard works). Pre-empting S2's *int design here crosses the boundary.

// CRITICAL (no consumers yet): S1 adds the fields but NOTHING reads them. materialize does NOT copy
// fileGeneration.{TokenLimit,DiffContext} into Config; overlay does NOT merge them. So after S1,
// Config.TokenLimit is always 0 and Config.DiffContext is always 1 (from Defaults), regardless of what a
// config file says. This is the intended "no behavior change yet" state — S2 wires the file→Config path.

// GOTCHA (gofmt alignment): the struct fields are column-aligned (type column padded to the longest name
// in the block, ~MaxDuplicateRetries/SubjectTargetChars = 18 chars). TokenLimit/DiffContext are shorter;
// RUN `gofmt -w internal/config/config.go internal/config/file.go` after the edits — gofmt realigns the
// whole block automatically. Do NOT hand-align.

// GOTCHA (TOML tags): snake_case, matching §16.2 (`token_limit`, `diff_context`). The toml tag is the
// [generation]-block key the user writes; Config field name is Go-CamelCase.

// GOTCHA (Defaults ordering): insert the two seeds between MaxMdLines:100 and MaxDuplicateRetries:3 to
// keep the diff-capture caps contiguous (MaxDiffBytes/MaxMdLines/TokenLimit/DiffContext grouped, before
// the retry/subject knobs). gofmt does NOT reorder struct-literal fields, so place them where you want them.

// GOTCHA (no test breaks): TestDefaults + file_test are field-specific (c.MaxDiffBytes != 300000), NOT
// exhaustive DeepEqual. Adding fields does not break them. (Verified: `go test ./internal/config/` green.)
// Still RECOMMENDED to add TokenLimit==0 / DiffContext==1 assertions to TestDefaults (convention).
```

## Implementation Blueprint

### Data models and structure

No new types — two flat int fields on two existing structs, plus two seeds. The relevant existing pair
(unchanged) is the precedent:

```go
// config.Config (EXISTING pair — the model to mirror)
MaxDiffBytes int `toml:"max_diff_bytes"` // byte cap on non-markdown diff section
MaxMdLines   int `toml:"max_md_lines"`   // per-file line cap for markdown diffs

// fileGeneration (EXISTING pair — the decode-side mirror)
MaxDiffBytes int `toml:"max_diff_bytes"`
MaxMdLines   int `toml:"max_md_lines"`
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: config.go — add the two fields to the Config struct
  - LOCATE: internal/config/config.go, the MaxDiffBytes/MaxMdLines pair (lines 77-78), immediately before
    MaxDuplicateRetries (line 79).
  - INSERT after MaxMdLines (line 78), before MaxDuplicateRetries:
        TokenLimit          int `toml:"token_limit"`           // FR3d holistic token cap (0 = unset ⇒ legacy caps); consumed by S2/S4
        DiffContext         int `toml:"diff_context"`          // FR3f reduced diff context (0–3; default 1); consumed by S2/S4
  - TOML TAGS: exactly `token_limit` / `diff_context` (snake_case, §16.2).
  - TYPE: plain `int` on BOTH (do NOT use *int on Config — see gotcha).
  - COMMENT: cite FR3d / FR3f on each (matches the existing field-comment style).
  - DO NOT: touch any other Config field; nest the fields; or add materialize/overlay logic.

Task 2: file.go — add the SAME two fields to the fileGeneration struct
  - LOCATE: internal/config/file.go, fileGeneration struct, the MaxDiffBytes/MaxMdLines pair (lines 50-51),
    immediately before MaxDuplicateRetries (line 52).
  - INSERT after MaxMdLines (line 51), before MaxDuplicateRetries:
        TokenLimit          int      `toml:"token_limit"`   // FR3d — plumbed in S2 (materialize/overlay)
        DiffContext         int      `toml:"diff_context"`  // FR3f — becomes *int in S2 (0-vs-unset); plain int here
  - TYPE: plain `int` on BOTH in S1 (the *int refactor of DiffContext is S2's job).
  - DO NOT: add materialize/overlay lines (S2); make DiffContext *int (S2).

Task 3: config.go — seed Defaults()
  - LOCATE: internal/config/config.go, Defaults(), the MaxDiffBytes:300000 / MaxMdLines:100 pair (lines
    168-169), immediately before MaxDuplicateRetries:3 (line 170).
  - INSERT after MaxMdLines:100 (line 169), before MaxDuplicateRetries:
        TokenLimit:          0, // FR3d: 0 = unset ⇒ legacy per-section caps (max_diff_bytes/max_md_lines) apply unchanged
        DiffContext:         1, // FR3f: reduced context (-U1) default; 0 = changed-lines-only, 3 = git default
  - VALUES: TokenLimit MUST be 0 (FR3d / §16.1); DiffContext MUST be 1 (FR3f / §16.1). Comment each.
  - DO NOT: seed any non-PRD value; reorder surrounding seeds.

Task 4 (RECOMMENDED): config_test.go — pin the seeds in TestDefaults
  - LOCATE: internal/config/config_test.go, TestDefaults (line 11), near the MaxDiffBytes/MaxMdLines
    assertions (lines 34-38).
  - ADD (same idiom):
        if c.TokenLimit != 0 {
            t.Errorf("TokenLimit = %d, want 0 (unset ⇒ legacy caps)", c.TokenLimit)
        }
        if c.DiffContext != 1 {
            t.Errorf("DiffContext = %d, want 1 (-U1 default)", c.DiffContext)
        }
  - WHY: matches the convention that every Defaults() field is asserted; pins the seeds so a future edit
    can't silently change them. NOT required by the contract but consistent with the codebase.
  - DO NOT: add materialize/overlay/git-config tests (S2/S3).

Task 5: VALIDATE
  - RUN: gofmt -w internal/config/config.go internal/config/file.go internal/config/config_test.go
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test -race ./...   # all green (new fields unconsumed; TestDefaults assertions pass)
  - FIX-FORWARD: read failures, fix, re-run. Do NOT skip.
```

### Implementation Patterns & Key Details

```go
// === config.go — Config struct (the edited region) ===
	MaxDiffBytes        int `toml:"max_diff_bytes"`        // byte cap on non-markdown diff section
	MaxMdLines          int `toml:"max_md_lines"`          // per-file line cap for markdown diffs
	TokenLimit          int `toml:"token_limit"`           // FR3d holistic token cap (0 = unset ⇒ legacy caps); consumed by S2/S4
	DiffContext         int `toml:"diff_context"`          // FR3f reduced diff context (0–3; default 1); consumed by S2/S4
	MaxDuplicateRetries int `toml:"max_duplicate_retries"` // re-gen attempts on duplicate subject
	SubjectTargetChars  int `toml:"subject_target_chars"`  // target subject length for truncation
```

```go
// === file.go — fileGeneration struct (the edited region) ===
type fileGeneration struct {
	MaxDiffBytes        int      `toml:"max_diff_bytes"`
	MaxMdLines          int      `toml:"max_md_lines"`
	TokenLimit          int      `toml:"token_limit"`   // FR3d — plumbed in S2 (materialize/overlay)
	DiffContext         int      `toml:"diff_context"`  // FR3f — becomes *int in S2 (0-vs-unset); plain int here
	MaxDuplicateRetries int      `toml:"max_duplicate_retries"`
	SubjectTargetChars  int      `toml:"subject_target_chars"`
	// ... rest unchanged ...
}
```

```go
// === config.go — Defaults() (the edited region) ===
		MaxDiffBytes:        300000,
		MaxMdLines:          100,
		TokenLimit:          0, // FR3d: 0 = unset ⇒ legacy per-section caps (max_diff_bytes/max_md_lines) apply unchanged
		DiffContext:         1, // FR3f: reduced context (-U1) default; 0 = changed-lines-only, 3 = git default
		MaxDuplicateRetries: 3,
		SubjectTargetChars:  50,
```

### Integration Points

```yaml
CONFIG STRUCT (internal/config/config.go):
  - field added: "TokenLimit int `toml:\"token_limit\"`"   # FR3d; default 0 (unset)
  - field added: "DiffContext int `toml:\"diff_context\"`" # FR3f; default 1 (-U1)

FILE DECODE STRUCT (internal/config/file.go fileGeneration):
  - field added: TokenLimit int (plain)   # decoded by go-toml; copied to Config in S2
  - field added: DiffContext int (plain)  # becomes *int in S2 (0-vs-unset); plain in S1

DEFAULTS (internal/config/config.go Defaults):
  - seed: TokenLimit: 0   # FR3d unset ⇒ legacy caps
  - seed: DiffContext: 1  # FR3f -U1

NO-TOUCH (explicitly — owned by sibling/later subtasks):
  - internal/config/file.go materialize/overlay      # S2 (P1.M1.T1.S2): wire fileGeneration→Config; DiffContext → *int
  - internal/config/git.go                            # S3 (P1.M1.T1.S3): stagecoach.tokenLimit / stagecoach.diffContext keys
  - internal/config/bootstrap.go + docs/CONFIGURATION.md  # S4 (P1.M1.T1.S4): template + docs
  - internal/git StagedDiffOptions + 6 call sites     # P1.M1.T2
  - the 3 diff functions (buildDiffArgs, -M/-U, index strip)  # P1.M2+
  - any other package; PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — S2/S3/S4 own):
  - S2: add `if g.TokenLimit != 0 { c.TokenLimit = g.TokenLimit }` to materialize + overlay; change
        fileGeneration.DiffContext to *int and copy non-nil into Config.DiffContext (resolving the 0-vs-unset).
  - S3: read stagecoach.tokenLimit / stagecoach.diffContext git-config keys into Config.
  - S4: add `token_limit` / `diff_context` to the bootstrap template + docs/CONFIGURATION.md.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/config/config.go internal/config/file.go internal/config/config_test.go   # realign the struct fields
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/config/...     # Expected: exit 0
go build ./...                   # Expected: exit 0 (new fields compile; nothing reads them yet)
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

# TestDefaults now also checks TokenLimit==0 / DiffContext==1 (Task 4)
go test -race -run TestDefaults ./internal/config/ -v

# Full config suite (proves materialize/overlay/file tests still green — fields unconsumed)
go test -race ./internal/config/ -v

# Expected: TestDefaults PASSES with the two new assertions; every other config test UNCHANGED (green).
```

### Level 3: Whole-Repository Regression (no collateral)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages green (S1 adds dead fields; no behavior change)
go vet ./...                     # Expected: exit 0

# Confirm ONLY the 3 intended config files changed
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/config/{config,file,config_test}.go only. No other files.
```

### Level 4: Dead-Field Confirmation (the new fields are unconsumed)

```bash
cd /home/dustin/projects/stagecoach

# Nothing reads cfg.TokenLimit / cfg.DiffContext yet (S2 wires them). Confirm:
grep -rn "\.TokenLimit\|\.DiffContext" --include="*.go" internal/ pkg/ cmd/ | grep -v "_test.go" | grep -v "/plan/"
# Expected: only the struct field declarations + Defaults() seeds in config.go (no consumer reads).
# (A config file setting [generation] token_limit=9999 still has NO effect after S1 — by design. S2 changes that.)

# Confirm materialize/overlay were NOT edited (S2's job)
git diff -- internal/config/file.go | grep -E '^\+.*TokenLimit|^\+.*DiffContext' | grep -v 'toml' || echo "OK: only struct fields added in file.go (no materialize/overlay lines)"
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green (incl. TestDefaults with the new assertions).

### Feature Validation

- [ ] `Config` has `TokenLimit int \`toml:"token_limit"\`` + `DiffContext int \`toml:"diff_context"\`` (flat, after MaxMdLines).
- [ ] `fileGeneration` has the same two plain-int fields (after MaxMdLines).
- [ ] `Defaults()` seeds `TokenLimit: 0` + `DiffContext: 1`, each commented with FR3d/FR3f.
- [ ] `TestDefaults` asserts `TokenLimit==0` + `DiffContext==1` (recommended).

### Scope Discipline Validation

- [ ] ONLY `internal/config/{config,file,config_test}.go` modified (git diff --stat confirms).
- [ ] Did NOT edit `materialize`/`overlay` (S2), git-config (S3), bootstrap/docs (S4).
- [ ] Did NOT make `Config.DiffContext` or `fileGeneration.DiffContext` a `*int` (S2's job).
- [ ] Did NOT touch any consumer (`StagedDiffOptions`, call sites, diff functions).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Fields mirror the MaxDiffBytes/MaxMdLines pair (flat int + snake_case toml + FR-citation comment).
- [ ] `gofmt -w` run (struct fields column-aligned automatically).
- [ ] Defaults values match PRD §16.1 exactly (token_limit 0, diff_context 1).

---

## Anti-Patterns to Avoid

- ❌ Don't make `Config.DiffContext` a `*int` — it's a plain int in S1 (the resolved concrete value, default
  1). The `*int` 0-vs-unset disambiguation belongs on `fileGeneration.DiffContext` and is S2's job (S1 keeps
  it plain int too). Pre-empting S2's design crosses the subtask boundary.
- ❌ Don't add `materialize`/`overlay` lines for the new fields — that's S2's whole subtask ("materialize +
  overlay field-merge with DiffContext *int pointer"). S1 fields are decoded but NOT copied into Config.
- ❌ Don't seed non-PRD defaults. `TokenLimit` MUST be 0 (FR3d unset ⇒ legacy caps); `DiffContext` MUST be 1
  (FR3f -U1, §16.1). A different seed would silently change behavior once S2 wires them.
- ❌ Don't nest the fields into a sub-struct — config fields are FLAT on `Config` (touchmap §3); the new
  fields mirror MaxDiffBytes/MaxMdLines.
- ❌ Don't hand-align the struct columns — run `gofmt -w`; it realigns the whole block.
- ❌ Don't add git-config keys, bootstrap-template lines, or docs — those are S3/S4. S1 is internal struct
  fields only (contract point 5).
- ❌ Don't worry about breaking existing tests — they're field-specific (not exhaustive DeepEqual), verified
  green. (But DO add the TestDefaults assertions to pin the new seeds — it's the codebase convention.)
- ❌ Don't touch any consumer (StagedDiffOptions, the 6 call sites, the diff functions) — those are
  P1.M1.T2 / P1.M2+; S1 is config-layer-only.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a 3-spot scaffolding edit (Config struct + fileGeneration struct + Defaults()) with the
exact current code quoted verbatim, the precise insertion points (after MaxMdLines in each), the verbatim
new lines (TOML tags + FR-citation comments + PRD-mandated seeds 0 and 1), and the gofmt-alignment note.
Three independent confirmations de-risk it: (1) the touchmap §3 pins the flat-on-Config + fileGeneration
placement; (2) the existing tests are field-specific (no exhaustive DeepEqual) so they stay green, verified
by `go test ./internal/config/` passing; (3) the S1/S2 `*int`-vs-`int` boundary is stated explicitly so the
implementer doesn't pre-empt S2. The only residual uncertainty (not 10/10) is whether the implementer adds
the recommended TestDefaults assertions (a convention, not a contract requirement) — either way the build
is green. The fields are dead (unconsumed) after S1, so there is literally no behavior change to regress.
