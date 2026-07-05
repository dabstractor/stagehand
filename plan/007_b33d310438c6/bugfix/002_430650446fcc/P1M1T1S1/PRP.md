---
name: "P1.M1.T1.S1 — Line-anchor splitDiffSections via (?m)^diff --git regex slice + pure unit regression"
description: |
  Bugfix (PRD Issue 1 — Major). `splitDiffSections` (`internal/git/truncatediff.go:76`) splits on the
  UN-ANCHORED substring `"diff --git "`, so a content line `+diff --git a/foo b/foo` (a fixture/doc/
  snapshot/.patch embedding a sample diff) is torn into a bogus section boundary. One real file fragments
  into many tiny bogus sections → water-fill sizes them tiny → truncates nothing → the payload SILENTLY
  overflows `token_limit` (~7× measured), breaking FR3d's "always fits" contract. Fix: Shape B — add a
  `(?m)^diff --git ` line-anchored regex (mirroring the sibling diffSectionHeaderRe/atAtRe style) and
  rewrite splitDiffSections to slice at `FindAllStringIndex` offsets (the slice begins AT the header, so
  the prefix is naturally present — NO re-prefix; byte-identical round-trip preserved). Rewrite the now-
  wrong godoc (it defended the un-anchored split). Add 2 pure table cases to TestSplitDiffSections
  (content-embedded literal → 1 section; 2-file aggregate with embedded literal → 2 sections). Signature
  FROZEN. Default token_limit==0 path & markdown unaffected. No external docs in S1.
---

## Goal

**Feature Goal**: Eliminate the FR3i section-splitting defect where a non-markdown file whose content
contains the literal `diff --git ` is fragmented into bogus sections, silently defeating `token_limit`
truncation and overflowing the model's context window. Make `splitDiffSections` split ONLY on real
file-section headers (`diff --git ` at a line start / column 0), so each real file is one section — sized
and truncated as a unit — restoring FR3d's "the payload always fits" contract.

**Deliverable** (one regex + one function rewrite + one doc-comment correction + two test cases):
1. `internal/git/truncatediff.go`: add `var diffSectionBoundaryRe = regexp.MustCompile("(?m)^diff --git ")`
   (sibling-style, near `diffSectionHeaderRe`/`atAtRe`).
2. `internal/git/truncatediff.go`: rewrite `splitDiffSections` to slice at `diffSectionBoundaryRe.FindAllStringIndex`
   offsets (Shape B — no re-prefix; byte-identical round-trip). Signature UNCHANGED.
3. `internal/git/truncatediff.go`: correct the `splitDiffSections` godoc (it defended the un-anchored
   trailing-space split — now factually wrong) to the line-anchoring rationale.
4. `internal/git/truncatediff_test.go`: add 2 pure table cases to `TestSplitDiffSections` — (i) a single
   real file whose body contains `+diff --git a/foo b/foo` → 1 section byte-identical to input;
   (ii) a 2-file aggregate where the first file's body embeds a `diff --git ` literal → 2 sections.

**Success Definition**: `splitDiffSections` returns one section per REAL `diff --git` header at a line
start; a content-embedded `diff --git ` literal is inert. All 6 existing `TestSplitDiffSections` fixtures
pass byte-identically (the rewrite is non-regressing); the 2 new cases pass. `go build/vet/gofmt` clean
and `go test -race ./...` green. Signature unchanged. Default `token_limit==0` path and markdown handling
unaffected.

## User Persona

**Target User**: The user who opts into `token_limit` (because their model has a small context window)
AND stages a non-markdown file whose content contains a sample git diff — test fixtures/golden snapshots,
documentation showing diffs, vendored `.patch`/`.diff` files, the source of git/diff tooling, changelogs
quoting diffs. Also the contributor implementing the E2E regression (S2).

**Use Case**: `stagehand` (or `--dry-run`) with `token_limit = 2000` on a repo with a large `fixtures.diff`
whose content is 500 documented diff blocks. Today the payload ships ~14543 tokens (7× over, zero
sentinels); after the fix the single large file is one section, sized, and truncated to fit.

**Pain Points Addressed**: The silent overflow breaks FR3d's core promise for the exact population that
needs it most (small-context-window models), with no error/sentinel/diagnostic — the payload just ships
oversized and the agent errors/refuses/silently-truncates its own input.

## Why

- **Restores FR3d's headline contract.** "The payload always fits your model's context window" is the
  entire purpose of `token_limit`. The fragmentation makes that a silent lie for a realistic class of
  file content.
- **Single-point fix covers all three diff functions.** `splitDiffSections` is the one shared primitive,
  consumed by `applyWaterFillGate` in the `TokenLimit>0` branch of `StagedDiff`/`TreeDiff`/`WorkingTreeDiff`
  (FR3c parity). Fixing it once fixes all three.
- **Matches the established convention.** Every sibling helper in the same file is already `(?m)^`
  line-anchored (`diffSectionHeaderRe`, `diffSectionPlusPlusRe`, `atAtRe`). `splitDiffSections` is the
  lone un-anchored outlier; the fix brings it in line.
- **Shape B is the lowest-risk rewrite.** Regex `FindAllStringIndex` slicing at header offsets consumes
  nothing, so each section is byte-identical to its source slice (no re-prefix, no newline bookkeeping).
  Arch §4 verified Shape B byte-identical against all 6 existing fixtures. Shape A (`"\ndiff --git "`
  split) consumes the preceding newline and breaks the PREAMBLE fixture.
- **Default path & markdown unaffected.** The default `token_limit==0` path never calls
  `splitDiffSections` (legacy whole-string byte-cap); markdown files bypass it. No regression risk there.

## What

A line-anchored rewrite of one pure function (`splitDiffSections`) + its godoc + 2 pure unit test cases.
No signature change, no caller change, no other package, no external docs. The function remains PURE
(stdlib `regexp` + `strings` only).

### Success Criteria

- [ ] `diffSectionBoundaryRe = regexp.MustCompile("(?m)^diff --git ")` added (sibling-style).
- [ ] `splitDiffSections` slices at `diffSectionBoundaryRe.FindAllStringIndex` offsets (Shape B); each
      section begins at its header offset (NO re-prefix); byte-identical round-trip preserved.
- [ ] A content line `+diff --git a/foo b/foo` is INERT (does not create a section boundary).
- [ ] Empty/whitespace-only input → `nil` (UNCHANGED).
- [ ] All 6 existing `TestSplitDiffSections` fixtures pass byte-identically (non-regressing).
- [ ] 2 new table cases pass: (i) embedded literal in a single file → 1 section byte-identical to input;
      (ii) 2-file aggregate with first body embedding the literal → 2 sections.
- [ ] `splitDiffSections` signature UNCHANGED; `regexp`/`strings` the only imports (no new import).
- [ ] `splitDiffSections` godoc corrected (line-anchoring rationale, not the trailing-space defense).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact buggy line, the verbatim Shape-B rewrite (regex + new
function body) from arch §4, the byte-identical verification table for all 6 existing fixtures, the 2 new
test cases (verbatim inputs + wants), and the godoc-correction requirement. The #1 trap — that the regex
slice must NOT re-prefix (unlike the old `strings.Split`) — is called out explicitly.

### Documentation & References

```yaml
# MUST READ — the authoritative fix (Shape B, verbatim + byte-identical verification)
- docfile: plan/007_b33d310438c6/bugfix/002_430650446fcc/architecture/system_context.md
  why: "§1 root cause (the un-anchored split + why it defeats token_limit); §2 blast radius (ONE caller applyWaterFillGate; only token_limit>0 + non-md); §3 the sibling-anchoring inconsistency table; §4 the RECOMMENDED Shape-B rewrite (verbatim regex + function body) + the byte-identical verification table against all 6 existing fixtures; §3 the WRONG godoc defense that must be corrected."
  critical: "§4 gives the exact Shape-B code (diffSectionBoundaryRe + FindAllStringIndex slice) and confirms it is byte-identical for every existing fixture. §4 explains why Shape A is error-prone (consumes the preceding \\n, breaks PREAMBLE). Use Shape B verbatim."

- docfile: plan/007_b33d310438c6/bugfix/002_430650446fcc/TEST_RESULTS.md
  why: "PRD Issue 1: the measured overflows (~7× at token_limit=2000), the realistic triggers (fixtures/snapshots/docs/.patch), the repro, and the prescribed line-anchored fix."

# The single production file under edit
- file: internal/git/truncatediff.go
  why: "EDIT (3 spots). (a) Add `var diffSectionBoundaryRe = regexp.MustCompile(\"(?m)^diff --git \")` near diffSectionHeaderRe (line 27) / atAtRe (line 40). (b) Rewrite splitDiffSections (lines 76-95) to the Shape-B regex slice (NO re-prefix). (c) Correct the splitDiffSections godoc (lines ~59-73) — the trailing-space defense is now wrong."
  pattern: "Mirror the sibling regex style: package-level `var xRe = regexp.MustCompile(\"(?m)^…\")` with a doc comment citing the (?m)^ line-anchor + the sibling names. splitDiffSections stays PURE (regexp + strings only); signature FROZEN."
  gotcha: "Shape-B slices AT the header offset → the `diff --git ` prefix is NATURALLY part of each section. Do NOT re-prefix (the old strings.Split code re-prefixed because it consumed the separator; the regex slice consumes nothing). Re-prefixing would DOUBLE the prefix and break byte-identity."

- file: internal/git/truncatediff_test.go
  why: "EDIT (2 new table cases). Add to the `tests` slice in TestSplitDiffSections (~line 24): (i) single file body containing `+diff --git a/foo b/foo` → want 1 section byte-identical to input; (ii) 2-file aggregate with first body embedding the literal → want 2 sections. Assert len + per-section byte equality (the existing idiom)."
  pattern: "Existing table-driven pure style: `tests := []struct{desc,in string; want []string}{…}`, `t.Run(tc.desc, …)`, `len(got)==len(want)` + `got[i]==want[i]`. No t.TempDir/IO/testify. String-literal sections with explicit \\n."

# Read-only refs (do NOT edit in S1)
- file: internal/git/tokengate.go # applyWaterFillGate:129
  why: "READ-ONLY — the ONE production caller. `nmSections := splitDiffSections(nmDiff)` then sizes/water-fills/truncates. The fix is internal to splitDiffSections; the caller + signature are UNCHANGED."
- file: internal/git/git.go # StagedDiff:883 / TreeDiff:1360 / WorkingTreeDiff:1533
  why: "READ-ONLY — the TokenLimit>0 branch calls applyWaterFillGate (→ splitDiffSections). The default TokenLimit==0 path NEVER calls splitDiffSections (regression anchor). Markdown files bypass it. All unaffected by the internal fix."

- docfile: plan/007_b33d310438c6/bugfix/002_430650446fcc/P1M1T1S1/research/s1_line_anchor_split.md
  why: "Distilled S1 findings: the verbatim Shape-B rewrite, the 6-fixture byte-identical table, the 2 new test cases (verbatim), the godoc-correction requirement, and the S1/S2/P1.M2 scope boundary."
```

### Current Codebase Tree (relevant slice)

```bash
stagehand/
└── internal/git/
    ├── truncatediff.go      # EDIT: + diffSectionBoundaryRe; rewrite splitDiffSections; correct godoc
    └── truncatediff_test.go # EDIT: +2 table cases in TestSplitDiffSections
```

### Desired Codebase Tree After S1

```bash
stagehand/
└── (only existing files modified — no new files)
    internal/git/truncatediff.go      # +regex; splitDiffSections line-anchored (Shape B); godoc corrected
    internal/git/truncatediff_test.go # +2 content-embedded-literal table cases
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/truncatediff.go` | MODIFY | + `diffSectionBoundaryRe`; rewrite `splitDiffSections` (Shape B, no re-prefix); correct godoc. **Only production file.** |
| `internal/git/truncatediff_test.go` | MODIFY | +2 pure table cases in `TestSplitDiffSections`. |

**Explicitly NOT touched**: `internal/git/tokengate.go` / `git.go` / `numstat.go` / `skeleton.go` /
`tokens.go` / `waterfill.go` (callers + siblings — signature frozen, fix is internal), E2E integration
tests `difftokenlimit_test.go` (S2 = P1.M1.T2.S1), docs (P1.M2.T1.S1), any other package, `PRD.md`,
`tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (no re-prefix): Shape-B slices AT the header offset → the `diff --git ` prefix is NATURALLY
// part of each section. Do NOT re-prefix (the old `strings.Split` code did `"diff --git " + p` because
// Split consumed the separator; FindAllStringIndex consumes NOTHING). Re-prefixing would double the
// prefix and break the byte-identical round-trip + all existing fixtures.

// CRITICAL (use Shape B, NOT Shape A): Shape A (`strings.Split(diff, "\ndiff --git ")`) consumes the
// preceding newline, so the PREAMBLE fixture ("PREAMBLE\ndiff --git …") loses its trailing \n and would
// need special-casing. Shape B (regex slice) consumes nothing → byte-identical for all 6 existing
// fixtures (arch §4 table). Use Shape B verbatim.

// CRITICAL (signature FROZEN): `func splitDiffSections(diff string) []string` — consumed unchanged by
// applyWaterFillGate + the 3 diff functions. Do NOT change the signature, return type, or purity.

// GOTCHA (regex import): `regexp` is ALREADY imported (used by diffSectionHeaderRe/diffSectionPlusPlusRe/
// atAtRe). `strings` stays (TrimSpace). NO new import. NO new dependency.

// GOTCHA (the godoc is now WRONG): the current comment (lines ~59-73) defends the trailing-space split
// as "the faithful section boundary that distinguishes the header from a content line that happens to
// start with diff --git". That defense only covers a content line WITHOUT the trailing space. Rewrite it
// to the line-anchoring rationale: every content line is diff-marker-prefixed (+/-/space/\), so
// (?m)^diff --git matches only real headers at column 0.

// GOTCHA (whitespace-only → nil): preserve `if strings.TrimSpace(diff) == "" { return nil }` as the
// FIRST check (UNCHANGED). Do NOT TrimSpace the whole input otherwise — that would strip the trailing
// "\n" of the last section and break byte-identical round-tripping.

// GOTCHA (no matches → single blob): if FindAllStringIndex returns no matches (no line starts with
// `diff --git `), return `[]string{diff}` (the non-empty input as a single verbatim blob). This should
// not occur for a clean non-md aggregate (which always starts with `diff --git`) but is the safe default.

// GOTCHA (test style): the new cases are PURE table cases in the existing `tests` slice — string-literal
// sections with explicit \n, assert len + per-section byte equality. No t.TempDir, no I/O, no testify.
// The content-embedded literal line MUST start with a diff marker (`+`) so (?m)^diff --git does NOT match.
```

## Implementation Blueprint

### Data models and structure

No data-model change — one new package-level regex + one pure-function rewrite. The relevant existing
sibling regexes (the style to mirror, unchanged):

```go
// internal/git/truncatediff.go (EXISTING siblings — the pattern to mirror)
var diffSectionHeaderRe  = regexp.MustCompile(`(?m)^diff --git a/(.*) b/(.*)$`)
var diffSectionPlusPlusRe = regexp.MustCompile(`(?m)^\+\+\+ b/(.*)$`)
var atAtRe               = regexp.MustCompile(`(?m)^@@`)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD diffSectionBoundaryRe (internal/git/truncatediff.go)
  - PLACEMENT: near the sibling regexes (after diffSectionHeaderRe/diffSectionPlusPlusRe/atAtRe, lines
    27-40), before truncatedSentinel. Mirror their style exactly.
  - ADD:
        // diffSectionBoundaryRe matches a real file-section header at a LINE START. (?m)^ anchors at
        // column 0, so a content line carrying the literal `diff --git ` (always prefixed with a diff
        // marker +/-/space/\) does NOT match. Mirrors the line-anchored siblings
        // diffSectionHeaderRe/atAtRe. Pure; compiled once.
        var diffSectionBoundaryRe = regexp.MustCompile(`(?m)^diff --git `)
  - IMPORTS: `regexp` already imported — NO new import.
  - DO NOT: change the sibling regexes.

Task 2: REWRITE splitDiffSections (internal/git/truncatediff.go:76-95) — Shape B regex slice
  - REPLACE the body (the `strings.Split(diff, "diff --git ")` loop) with the Shape-B slice (verbatim
    from arch §4):
        func splitDiffSections(diff string) []string {
            if strings.TrimSpace(diff) == "" {
                return nil
            }
            matches := diffSectionBoundaryRe.FindAllStringIndex(diff, -1)
            if len(matches) == 0 {
                // No section boundary → the non-empty input is a single blob (should not occur for a
                // clean non-md aggregate, which always starts with `diff --git`). Preserve verbatim.
                return []string{diff}
            }
            var sections []string
            // Leading content before the first boundary: drop if empty (TrimSpace), preserve if non-empty
            // (defensive — a stray placeholder/comment would be preserved, not lost).
            if first := matches[0][0]; first > 0 {
                if leading := diff[:first]; strings.TrimSpace(leading) != "" {
                    sections = append(sections, leading)
                }
            }
            // Each section is sliced AT its header offset, so the `diff --git ` prefix is naturally
            // present (NO re-prefixing — the regex consumes nothing). Byte-identical split→join round-trip.
            for i, m := range matches {
                start := m[0]
                end := len(diff)
                if i+1 < len(matches) {
                    end = matches[i+1][0]
                }
                sections = append(sections, diff[start:end])
            }
            return sections
        }
  - PRESERVE the leading `if strings.TrimSpace(diff) == "" { return nil }` (UNCHANGED — first check).
  - DO NOT: re-prefix the sections; TrimSpace the whole input; change the signature; use Shape A.

Task 3: CORRECT the splitDiffSections godoc (internal/git/truncatediff.go ~lines 59-73)
  - The current godoc defends the un-anchored trailing-space split ("the faithful section boundary that
    distinguishes the header from a content line that happens to start with `diff --git`"). That defense
    is now WRONG. Rewrite to the line-anchoring rationale:
    - Each returned section is self-contained (begins with `diff --git a/<p> b/<p>`) because sections are
      SLICED AT the header offset (the regex consumes nothing → no re-prefix → byte-identical round-trip).
    - Line-anchoring: `(?m)^diff --git ` matches only at column 0; every diff CONTENT line is prefixed
      with a marker (+/-/space/\), so a content line carrying the literal `diff --git ` is INERT.
    - Leading element: drop if empty, preserve if non-empty; empty/whitespace-only input → nil; the
      trailing "\n" of the last section is NOT stripped (byte-identical round-trip).
  - DO NOT: leave the stale trailing-space defense in place.

Task 4: ADD 2 pure table cases to TestSplitDiffSections (internal/git/truncatediff_test.go ~line 24)
  - PLACEMENT: in the `tests` slice, alongside the existing 6 cases.
  - CASE (i) — single real file, body embeds the literal:
        {
            desc: "content-embedded diff --git literal → ONE section (line-anchored, inert)",
            in:   "diff --git a/real.txt b/real.txt\n@@ -0,0 +1,1 @@\n+diff --git a/foo b/foo\n",
            want: []string{"diff --git a/real.txt b/real.txt\n@@ -0,0 +1,1 @@\n+diff --git a/foo b/foo\n"},
        }
    → the `+diff --git...` content line starts with `+` → not a match → 1 section byte-identical to input.
  - CASE (ii) — 2-file aggregate, first body embeds the literal:
        {
            desc: "first file body embeds diff --git literal → TWO sections (only real headers are boundaries)",
            in: "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,2 @@\n-old\n+diff --git a/embed b/embed\n" +
                "diff --git a/b.go b/b.go\n--- b/a.go\n+++ b/b.go\n@@ -1 +1 @@\n-b\n+b2\n",
            want: []string{
                "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,2 @@\n-old\n+diff --git a/embed b/embed\n",
                "diff --git a/b.go b/b.go\n--- b/a.go\n+++ b/b.go\n@@ -1 +1 @@\n-b\n+b2\n",
            },
        }
    → the embedded content line is inert; only the 2 real headers at line start are boundaries → 2 sections.
  - ASSERTIONS: the existing loop (`len(got)==len(want)` + per-section `got[i]==want[i]`) covers BOTH new
    cases unchanged — no new assertion code needed.
  - DO NOT: add E2E/integration tests (S2); change the assertion loop; use t.TempDir/IO/testify.

Task 5: VALIDATE
  - RUN: gofmt -w internal/git/truncatediff.go internal/git/truncatediff_test.go
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test -race -run TestSplitDiffSections ./internal/git/ -v   # 6 existing + 2 new cases pass
  - RUN: go test -race ./...                                           # full suite green
  - FIX-FORWARD: read failures, fix, re-run.
```

### Implementation Patterns & Key Details

```go
// === the new sibling-style regex (near diffSectionHeaderRe/atAtRe) ===
var diffSectionBoundaryRe = regexp.MustCompile(`(?m)^diff --git `)

// === splitDiffSections — Shape B (the slice begins AT the header; NO re-prefix) ===
	matches := diffSectionBoundaryRe.FindAllStringIndex(diff, -1)
	if len(matches) == 0 {
		return []string{diff}
	}
	// … leading[:first] if TrimSpace-non-empty …
	for i, m := range matches {
		start := m[0]
		end := len(diff)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		sections = append(sections, diff[start:end]) // prefix naturally present — DO NOT re-prefix
	}
```

```go
// === why a content-embedded literal is inert (the line-anchor rationale) ===
// input line: "+diff --git a/foo b/foo"  (an added line documenting a diff)
// (?m)^diff --git  → the line starts with "+", not "diff" → NO MATCH at that line.
// Only real headers ("diff --git a/real.txt b/real.txt" at column 0) match → one section per real file.
```

### Integration Points

```yaml
PRODUCTION (internal/git/truncatediff.go):
  - added: var diffSectionBoundaryRe = regexp.MustCompile(`(?m)^diff --git `)
  - rewritten: splitDiffSections (Shape B regex slice; NO re-prefix; byte-identical round-trip)
  - corrected: splitDiffSections godoc (line-anchoring rationale)
  - signature UNCHANGED: func(diff string) []string
  - imports UNCHANGED: regexp + strings (both already imported)

TESTS (internal/git/truncatediff_test.go):
  - +2 table cases in TestSplitDiffSections (content-embedded literal: 1-file→1-section, 2-file→2-sections)
  - existing 6 fixtures pass byte-identically (non-regressing)

NO-TOUCH (explicitly):
  - internal/git/tokengate.go (applyWaterFillGate — the ONE caller; signature frozen)
  - internal/git/git.go (StagedDiff/TreeDiff/WorkingTreeDiff — consume via the gate; token_limit==0 path never calls splitDiffSections)
  - internal/git/{numstat,skeleton,tokens,waterfill}.go   # siblings
  - internal/git/difftokenlimit_test.go (E2E integration regression = S2 / P1.M1.T2.S1)
  - docs (P1.M2.T1.S1); PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks):
  - S2 (P1.M1.T2.S1): E2E content-embedded-literal regression across the 3 diff functions
    (difftokenlimit_test.go) — asserts the payload FITS token_limit + exactly one sentinel for the single
    large file, under a real git repo.
  - P1.M2.T1.S1: docs sweep (confirm docs/configuration.md's token_limit "always fits" wording is now
    accurate; no edit expected — the fix RESTORES the documented behavior).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand

gofmt -w internal/git/truncatediff.go internal/git/truncatediff_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/git/...        # Expected: exit 0
go build ./...                   # Expected: exit 0
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagehand

# The split-primitive tests (6 existing fixtures + 2 new content-embedded-literal cases)
go test -race -run TestSplitDiffSections ./internal/git/ -v

# Expected: ALL 8 cases pass — the 6 existing byte-identically (non-regressing), the 2 new ones (1-section
# and 2-section). The content-embedded literal is inert under the line-anchored regex.
```

### Level 3: Whole-Repository Regression (no collateral)

```bash
cd /home/dustin/projects/stagehand

go test -race ./...              # Expected: ALL packages green (fix is internal to the shared primitive)
go vet ./...                     # Expected: exit 0

# Confirm ONLY the 2 intended files changed
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/git/truncatediff.go + internal/git/truncatediff_test.go only.
```

### Level 4: The Bug-Is-Gone Check (unit-level, direct)

```bash
cd /home/dustin/projects/stagehand

# The new case (i) IS the direct bug-repro at the unit level. Cross-check the property directly:
go test -race -run 'TestSplitDiffSections/content-embedded' ./internal/git/ -v

# Expected: PASS. Before the fix this input returned 2 sections (one bogus); after, 1 section byte-identical.
# (The end-to-end "payload fits token_limit" assertion lands in S2 / P1.M1.T2.S1's difftokenlimit_test.go.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.

### Feature Validation

- [ ] `diffSectionBoundaryRe = regexp.MustCompile("(?m)^diff --git ")` added (sibling-style).
- [ ] `splitDiffSections` uses Shape B (FindAllStringIndex slice, NO re-prefix); signature unchanged.
- [ ] A content line `+diff --git a/foo b/foo` is inert (no bogus section).
- [ ] Empty/whitespace-only input → `nil`; no-matches non-empty input → `[]{diff}`.
- [ ] All 6 existing `TestSplitDiffSections` fixtures pass byte-identically.
- [ ] 2 new cases pass (1-file→1-section byte-identical; 2-file→2-sections).
- [ ] `splitDiffSections` godoc corrected (line-anchoring rationale; stale trailing-space defense gone).

### Scope Discipline Validation

- [ ] ONLY `internal/git/{truncatediff,truncatediff_test}.go` modified (git diff --stat confirms).
- [ ] Did NOT edit `tokengate.go`/`git.go`/siblings (signature frozen; fix is internal).
- [ ] Did NOT add E2E/integration tests (S2 = P1.M1.T2.S1) or docs (P1.M2.T1.S1).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Regex mirrors the sibling style (`var xRe = regexp.MustCompile("(?m)^…")` + doc comment).
- [ ] Shape-B slice preserves byte-identical round-tripping (no re-prefix, no whole-input TrimSpace).
- [ ] New test cases are pure table entries (string literals, existing assertion loop, no I/O/testify).
- [ ] Embedded-literal lines start with a diff marker (`+`) so the line-anchor correctly excludes them.

---

## Anti-Patterns to Avoid

- ❌ Don't re-prefix the sections. Shape-B slices AT the header offset, so the `diff --git ` prefix is
  naturally present. The old `strings.Split` re-prefixed because it CONSUMED the separator; the regex
  slice consumes NOTHING. Re-prefixing doubles the prefix and breaks byte-identity.
- ❌ Don't use Shape A (`strings.Split(diff, "\ndiff --git ")`). It consumes the preceding newline, so the
  PREAMBLE fixture loses its trailing `\n` and needs special-casing. Use Shape B (regex slice) — arch §4
  verified it byte-identical against all 6 existing fixtures.
- ❌ Don't change `splitDiffSections`'s signature, return type, or purity. It's FROZEN, consumed unchanged
  by `applyWaterFillGate` and all three diff functions.
- ❌ Don't add a new import or dependency — `regexp` + `strings` are already imported.
- ❌ Don't leave the stale godoc. The current comment DEFENDS the un-anchored trailing-space split — that
  defense is now factually wrong (it only covers a content line WITHOUT the trailing space). Rewrite it to
  the line-anchoring rationale (content lines are marker-prefixed → `(?m)^diff --git` matches only real
  headers at column 0).
- ❌ Don't TrimSpace the whole input (only the leading element, when deciding to drop it). Stripping the
  trailing `\n` of the last section breaks byte-identical round-tripping.
- ❌ Don't forget the `len(matches)==0 → []string{diff}` branch — a non-empty input with no `diff --git`
  header line must return a single verbatim section (safe default; should not occur for a clean aggregate).
- ❌ Don't make the embedded-literal test line start with `diff` (no marker) — it MUST start with a diff
  marker (`+`/`-`/space) so `(?m)^diff --git` correctly does NOT match it. Otherwise the test doesn't
  exercise the bug.
- ❌ Don't add E2E/integration tests or docs here — those are S2 (P1.M1.T2.S1) / P1.M2.T1.S1. S1 is the
  pure primitive fix + pure unit cases only.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a single-function pure rewrite with the verbatim Shape-B code (regex + function body)
supplied by arch §4, which ALSO verified it byte-identical against all 6 existing fixtures — so the
non-regression proof is already done. Three independent de-riskings: (1) the bug is the lone un-anchored
op in a file where every sibling is `(?m)^`-anchored, so the fix is "bring it in line with the convention";
(2) Shape B (regex slice) consumes nothing → byte-identical round-trip with zero newline bookkeeping,
unlike Shape A which breaks the PREAMBLE fixture; (3) the blast radius is exactly ONE caller
(applyWaterFillGate) and the default token_limit==0 path never calls the function, so collateral is nil.
The #1 implementation trap — re-prefixing the sections after slicing (carried over from the old
strings.Split habit) — is called out four times. The one residual uncertainty (not 10/10) is the godoc
rewriting (subjective phrasing), which has no functional effect. The E2E regression (S2) and docs sweep
(P1.M2) are cleanly fenced and untouched.
