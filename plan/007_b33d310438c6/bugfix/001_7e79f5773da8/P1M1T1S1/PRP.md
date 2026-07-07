---
name: "P1.M1.T1.S1 — Apply sentinel-newline fix + pure unit regression in truncateByWaterFill"
description: |
  Bugfix (PRD Issue 1 — Major). In `internal/git/truncatediff.go` `truncateByWaterFill`, the truncation
  branch writes `firstNRunes(body, allotment*4) + "\n" + truncatedSentinel` with NO trailing newline, so a
  truncated NON-LAST section's sentinel glues to the next section's `diff --git` header
  (`... [truncated]diff --git a/b.go b/b.go`). Fix: append `+ "\n"` AFTER the sentinel (sentinel-only — do
  NOT touch within-budget / path-miss / pure-rename pass-through branches; the byte-identical guarantee
  must hold). Update THREE existing `HasSuffix(out, "\n... [truncated]")` assertions (lines 287/333/352 —
  the contract named only line 287, but 333 + 352 break identically) to `"\n... [truncated]\n"`. Add 4
  pure unit subtests that truncate a NON-LAST section (the gap that hid the bug) asserting
  `!Contains(out, "[truncated]diff --git")` + the next `diff --git` at a line start + sentinel count.
  Signature FROZEN. No docs in S1.
---

## Goal

**Feature Goal**: Eliminate the FR3i truncation-formatting defect where a truncated non-last diff
section's `... [truncated]` sentinel is glued to the following section's `diff --git` header on one line.
After the fix, every truncated section ends with `... [truncated]\n` and the next section's `diff --git`
begins at a line start — uniform across `StagedDiff` / `TreeDiff` / `WorkingTreeDiff` (the fix lands once
in the shared `truncateByWaterFill`), with byte-identical pass-through for within-budget / path-miss /
pure-rename sections preserved.

**Deliverable** (1 one-line production fix + 3 fixture-delta assertion updates + 4 new pure subtests):
1. `internal/git/truncatediff.go` `truncateByWaterFill`: change the truncation-branch body replacement to
   `firstNRunes(body, allotment*4) + "\n" + truncatedSentinel + "\n"` (append one trailing `\n`).
2. `internal/git/truncatediff_test.go`: update the THREE `!strings.HasSuffix(out, "\n... [truncated]")`
   assertions (lines 287, 333, 352) to `!strings.HasSuffix(out, "\n... [truncated]\n")`.
3. `internal/git/truncatediff_test.go`: add 4 new `t.Run` subtests in `TestTruncateByWaterFill` that
   truncate a NON-LAST section (non-md→non-md, md→non-md, non-md→md, both-truncated) and assert the
   sentinel is followed by `\n` (not `diff --git`), the next `diff --git` is at a line start, and the
   sentinel count equals the number of truncated files.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
No output contains `[truncated]diff --git`; every truncated section ends `... [truncated]\n`; the next
section's `diff --git` is at a line start. The within-budget / path-miss / pure-rename pass-through tests
remain byte-identical (the fix is sentinel-only). `truncateByWaterFill`'s signature is unchanged. No other
package, no docs.

## User Persona

**Target User**: The agent (and human `--dry-run` reader) consuming the recomposed diff payload — the fix
makes truncated multi-file diffs parse correctly (each `diff --git` section header at a line start) instead
of a malformed run-together line. Also the contributor implementing the E2E regression (S2).

**Use Case**: A repo with two large files staged under a small `token_limit`; both are truncated. Today the
first file's sentinel runs into the second's `diff --git` header; after the fix each is on its own line.

**Pain Points Addressed**: The glued `... [truncated]diff --git a/b.go b/b.go` line obscures the section
boundary (the model may mis-attribute hunks to the wrong file or drop the glued line) and breaks the
visual completeness floor; it's also a regression vs. the legacy `token_limit==0` path (which always
inserts the separating newline).

## Why

- **Major systemic bug, 100% reproducible.** Affects all three diff functions (`StagedDiff`/`TreeDiff`/
  `WorkingTreeDiff`) because they share `truncateByWaterFill` via `applyWaterFillGate`. Reproduced
  empirically whenever `token_limit > 0` truncates a non-last section.
- **Single-point fix.** `truncateByWaterFill` is the one shared recomposition chokepoint. Adding the
  trailing `\n` there covers all three diff functions AND the markdown per-file path in one edit.
- **Sentinel-only = lowest risk.** The fix touches ONLY the truncation branch. The pass-through branches
  (within-budget / path-miss / pure-rename) are byte-identical-pinned by 6 existing tests; the fix never
  fires in those branches, so the invariants hold mechanically.
- **Restores parity with the legacy path.** The `token_limit==0` markdown path already enforces
  `if !strings.HasSuffix(fileDiff, "\n") { b.WriteByte('\n') }` (git.go:859/1338/1511); the fix gives the
  water-fill path the same line-shape discipline.

## What

A one-line production change (append `+ "\n"` after the sentinel in the truncation branch), three
assertion fixture-deltas (the existing single-section suffix assertions now end with `\n`), and four new
pure unit subtests covering the non-last-section truncation case that hid the bug. No signature change,
no other package, no docs.

### Success Criteria

- [ ] `truncateByWaterFill`'s truncation branch writes `firstNRunes(body, allotment*4) + "\n" + truncatedSentinel + "\n"`.
- [ ] NO output of `truncateByWaterFill` ever contains the substring `[truncated]diff --git` (verified by the
      new subtests and the existing `item_3file...` test continuing to pass).
- [ ] Every truncated section ends with `... [truncated]\n`; the next section's `diff --git` is at a line start.
- [ ] The 3 existing `HasSuffix(out, "\n... [truncated]")` assertions (lines 287/333/352) are updated to
      `HasSuffix(out, "\n... [truncated]\n")` and pass.
- [ ] 4 new pure subtests (non-md→non-md, md→non-md, non-md→md, both-truncated) pass.
- [ ] The 6 pass-through/within-budget invariant tests stay byte-identical (sentinel-only fix).
- [ ] `truncateByWaterFill` signature is UNCHANGED.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact buggy line + the exact fixed line, the exact 3 assertion
lines to update (with line numbers + the before/after strings), and the 4 new subtests' shapes + assertions
(section literals, allotments, `Contains`/`Count` checks). The critical #1 one-pass trap — that the
contract named only ONE of the THREE breaking assertions — is surfaced with the grep evidence.

### Documentation & References

```yaml
# MUST READ — the bug report + authoritative fix
- docfile: plan/007_b33d310438c6/bugfix/001_7e79f5773da8/TEST_RESULTS.md
  why: "Issue 1: the exact buggy expression, the empirical repro, the captured glued output ('... [truncated]diff --git a/b.go b/b.go'), and the prescribed fix ('append a trailing newline after the sentinel'). Notes the existing tests missed it because they truncate only the LAST/only section."
  critical: "States the fix must land ONCE in truncateByWaterFill (covers all 3 diff functions + markdown path) and that a regression test truncating a NON-LAST section is required. This subtask IS that fix + the pure unit regression."

# The single production file under edit
- file: internal/git/truncatediff.go
  why: "EDIT (1 line). truncateByWaterFill's truncation branch: `body = firstNRunes(body, allotment*4) + \"\\n\" + truncatedSentinel` → append `+ \"\\n\"`. const truncatedSentinel (\"... [truncated]\") is UNCHANGED. Signature FROZEN."
  pattern: "The branch is `if EstimateTokens(body) > allotment { body = ... }`. Append `+ \"\\n\"` to the RHS only. Do NOT touch the surrounding `b.WriteString(headerBlock); b.WriteString(body)` or the pass-through branches."
  gotcha: "SENTINEL-ONLY. Do NOT add `\\n` to within-budget bodies, path-miss sections (`!ok || !found || allotment <= 0`), or pure-rename sections (`loc == nil`). Those must stay byte-identical (6 existing invariant tests pin them)."

- file: internal/git/truncatediff_test.go
  why: "EDIT (3 assertion updates + 4 new subtests). Lines 287/333/352: `!HasSuffix(out, \"\\n... [truncated]\")` → `!HasSuffix(out, \"\\n... [truncated]\\n\")`. Add 4 t.Run subtests in TestTruncateByWaterFill that truncate a NON-LAST section."
  pattern: "Existing pure style: string-literal sections (`diff --git a/x b/x\\n--- a/x\\n+++ b/x\\n@@ -1 +1 @@\\n...`), t.Run subtests, build big bodies via inline loops with the shared `itoa` helper, no t.TempDir/IO/testify. Reuse `itoa` (int→string) and `tail` (last n bytes) already in the file."
  gotcha: "The contract names ONLY line 287 (item_sentinel_on_its_own_line). Lines 333 (multi_hunk...) and 352 (markdown_per_file...) use the IDENTICAL assertion and break IDENTICALLY. Update ALL THREE or 2 tests fail."

# Read-only refs (do NOT edit in S1)
- file: internal/git/tokengate.go # applyWaterFillGate
  why: "READ-ONLY — the caller. Assembles `sections = mdDiffs + splitDiffSections(nmDiff)` and `allotments` (numstat destination path → token allotment), then calls truncateByWaterFill. The fix is internal to truncateByWaterFill; callers are UNCHANGED (signature frozen)."
- file: internal/git/git.go # StagedDiff/TreeDiff/WorkingTreeDiff token_limit>0 branches (~883/1360/1533)
  why: "READ-ONLY — the legacy token_limit==0 markdown path at git.go:859/1338/1511 (`if !strings.HasSuffix(fileDiff, \"\\n\") { b.WriteByte('\\n') }`) is the parity reference the fix restores. The fix is in the shared helper, not these functions."

- docfile: plan/007_b33d310438c6/bugfix/001_7e79f5773da8/P1M1T1S1/research/s1_sentinel_newline.md
  why: "Distilled S1 findings: the exact bug locus + fix, the byte-identical pass-through proof (6 invariant tests stay green), the THREE-assertion blast-radius table (grep-confirmed lines 287/333/352), the no-other-test-file-breaks proof, and the 4 new subtests' concrete shapes."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/git/
    ├── truncatediff.go      # EDIT (1 line: truncation branch + "\n")
    └── truncatediff_test.go # EDIT (3 assertion updates + 4 new t.Run subtests)
```

### Desired Codebase Tree After S1

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/git/truncatediff.go      # truncation branch: sentinel + "\n"
    internal/git/truncatediff_test.go # 3 HasSuffix updates + 4 non-last-section subtests
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/truncatediff.go` | MODIFY (1 line) | Append `+ "\n"` after the sentinel in the truncation branch. **Only production edit.** |
| `internal/git/truncatediff_test.go` | MODIFY | Update 3 `HasSuffix` assertions (287/333/352) for the trailing `\n`; add 4 non-last-section regression subtests. |

**Explicitly NOT touched**: `internal/git/tokengate.go` / `git.go` / `numstat.go` / `skeleton.go` /
`tokens.go` / `waterfill.go` (callers + siblings — signature frozen, fix is internal), any other package,
E2E integration tests `difftokenlimit_test.go` (S2 = P1.M1.T2.S1), `diff_context` validation (Issue 2 =
P1.M2.T1.S1), docs (P1.M3.T1.S1), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (#1 one-pass trap): the contract names ONLY `item_sentinel_on_its_own_line` (line 287) for the
// assertion update. But THREE subtests use the identical `!strings.HasSuffix(out, "\n... [truncated]")`:
// lines 287 (item_sentinel_on_its_own_line), 333 (multi_hunk_truncated_first_atat_kept_second_cut),
// 352 (markdown_per_file_section_same_code_path). ALL THREE break (the output now ends with `... [truncated]\n`).
// Update ALL THREE to `!HasSuffix(out, "\n... [truncated]\n")` or two tests fail. Grep-confirmed.

// CRITICAL (sentinel-only): the fix appends `+ "\n"` ONLY in the truncation branch
// (`if EstimateTokens(body) > allotment`). Do NOT add `\n` to within-budget bodies, path-miss sections,
// or pure-rename/no-@@ sections — 6 existing invariant tests pin them byte-identical
// (all_within_budget_byte_identical_no_sentinels, path_miss_pass_through_verbatim, recompose_in_input_order,
// pure_rename_no_hunk_verbatim, zero_or_negative_allotment_verbatim, body_exactly_at_allotment_no_truncation).
// Those branches never enter the truncation path → the added `\n` never fires → they stay green.

// CRITICAL (signature FROZEN): `func truncateByWaterFill(sections []string, allotments map[string]int) string`
// is consumed unchanged by applyWaterFillGate (tokengate.go) and transitively StagedDiff/TreeDiff/
// WorkingTreeDiff. Do NOT change its signature, its return type, or its purity (stdlib regexp + strings only).

// GOTCHA (no double-newline): after the fix, a truncated section ends `... [truncated]\n` and the next
// section starts `diff --git ...` → the join is exactly `... [truncated]\ndiff --git` (ONE newline). Do
// NOT also add a `\n` at the start of the next section or after the header block — that would double it.

// GOTCHA (trailing newline at end of output): if the LAST section is truncated, the recomposed output now
// ends with `... [truncated]\n` (before the fix it ended with `... [truncated]`, no trailing newline). This
// is CORRECT and consistent with the legacy token_limit==0 path (which enforces a trailing `\n` per section).
// The 3 updated HasSuffix assertions account for it (they now require the trailing `\n`).

// GOTCHA (test style): pure unit tests — string-literal sections with explicit \n, t.Run subtests, no
// t.TempDir, no I/O, no testify. Reuse the shared `itoa` (int→string, avoids strconv import) and `tail`
// (last n bytes, for error messages) helpers at the bottom of truncatediff_test.go. Build "truncated"
// bodies via an inline loop (~60 lines with itoa) + a small allotment (10); "within-budget" = the
// canonical 1-line body + a large allotment (100000).
```

## Implementation Blueprint

### Data models and structure

No data-model change — a single string-concatenation fix. The relevant existing constants/shape (unchanged):

```go
// internal/git/truncatediff.go (EXISTING — unchanged)
const truncatedSentinel = "... [truncated]"
// canonical section shape (from the tests):
// "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n"
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: FIX internal/git/truncatediff.go — append trailing "\n" after the sentinel (sentinel-only)
  - LOCATE: truncateByWaterFill, the truncation branch:
        if EstimateTokens(body) > allotment {
            body = firstNRunes(body, allotment*4) + "\n" + truncatedSentinel
        }
  - CHANGE the RHS to append a trailing newline:
        body = firstNRunes(body, allotment*4) + "\n" + truncatedSentinel + "\n"
  - PRESERVE the surrounding `b.WriteString(headerBlock); b.WriteString(body)` and ALL pass-through branches.
  - DO NOT: touch the `truncatedSentinel` const; add `\n` to within-budget / path-miss / pure-rename bodies;
    change the signature; or import anything new (regexp/strings already imported).
  - VERIFY: `go build ./internal/git/` compiles.

Task 2: UPDATE the THREE existing HasSuffix assertions in truncatediff_test.go
  - LOCATE (grep-confirmed lines): 287, 333, 352 — all are
        if !strings.HasSuffix(out, "\n... [truncated]") {
  - CHANGE each to:
        if !strings.HasSuffix(out, "\n... [truncated]\n") {
  - WHY: the output now ends with `... [truncated]\n` (trailing newline); the old suffix no longer matches.
    This is the EXPECTED fixture delta, not a regression. The sentinel is still on its own line (preceded
    by `\n`); it now also ends its line (followed by `\n`).
  - The three subtests: item_sentinel_on_its_own_line (287, contract-named),
    multi_hunk_truncated_first_atat_kept_second_cut (333), markdown_per_file_section_same_code_path (352).
  - DO NOT: touch the Contains/Count assertions in those or other tests (they still hold).

Task 3: ADD 4 new pure subtests in TestTruncateByWaterFill (the non-last-section regression)
  - PLACEMENT: inside TestTruncateByWaterFill (append after the existing t.Run blocks, before the closing
    brace). Use the existing style: t.Run subtests, string-literal sections, inline body loops with itoa.
  - Define/inline a "big body" builder (~60 content lines via itoa) for truncated sections; use the
    canonical 1-line body for within-budget sections. Allotments: truncated → 10; within-budget → 100000.
  - (a) t.Run("nonmd_truncated_then_nonmd_within_budget", ...):
        sectionA = "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n" + bigBody(60)   (truncated; path a.go, allotment 10)
        sectionB = "diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1 +1 @@\n-b\n+b2\n"  (within budget; path b.go, allotment 100000)
        out := truncateByWaterFill([]string{sectionA, sectionB}, map[string]int{"a.go":10,"b.go":100000})
        ASSERT: !strings.Contains(out, "[truncated]diff --git")                         // no glue
        ASSERT: strings.Contains(out, "... [truncated]\ndiff --git a/b.go b/b.go")      // next header at line start
        ASSERT: strings.Count(out, "... [truncated]") == 1                              // only A truncated
        ASSERT: strings.Contains(out, sectionB)                                         // B verbatim
  - (b) t.Run("md_truncated_then_nonmd", ...):
        sectionA = "diff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md\n" + bigBody(60)  (truncated; path README.md, allotment 10)
        sectionB = canonical b.go within budget (allotment 100000)
        out := truncateByWaterFill([]string{sectionA, sectionB}, map[string]int{"README.md":10,"b.go":100000})
        ASSERT: !Contains(out, "[truncated]diff --git"); Contains(out, "[truncated]\ndiff --git a/b.go b/b.go"); Count==1.
  - (c) t.Run("nonmd_truncated_then_md", ...):
        sectionA = "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n" + bigBody(60)  (truncated; a.go, allotment 10)
        sectionB = "diff --git a/NOTES.md b/NOTES.md\n--- a/NOTES.md\n+++ b/NOTES.md\n@@ -1 +1 @@\n-x\n+y\n"  (within budget; NOTES.md, allotment 100000)
        out := truncateByWaterFill([]string{sectionA, sectionB}, map[string]int{"a.go":10,"NOTES.md":100000})
        ASSERT: !Contains(out, "[truncated]diff --git"); Contains(out, "[truncated]\ndiff --git a/NOTES.md b/NOTES.md"); Count==1.
  - (d) t.Run("both_nonmd_truncated", ...):
        sectionA = a.go big body (allotment 10); sectionB = b.go big body (allotment 10)
        out := truncateByWaterFill([]string{sectionA, sectionB}, map[string]int{"a.go":10,"b.go":10})
        ASSERT: !Contains(out, "[truncated]diff --git")
        ASSERT: Contains(out, "... [truncated]\ndiff --git a/b.go b/b.go")   // A's sentinel → B's header
        ASSERT: strings.Count(out, "... [truncated]") == 2                  // both truncated
        ASSERT: strings.HasSuffix(out, "... [truncated]\n")                  // last section truncated → trailing \n
  - (body builder): inline, e.g.:
        bigBody := func(n int) string {
            var sb strings.Builder
            sb.WriteString("@@ -1,100 +1,100 @@\n")
            for i := 0; i < n; i++ {
                sb.WriteString("-old content line payload " + itoa(i) + " here\n")
                sb.WriteString("+new content line payload " + itoa(i) + " here\n")
            }
            return sb.String()
        }
    (Define it once inside TestTruncateByWaterFill near the existing unused `makeBody` closure, or per-case.)
  - DO NOT: use t.TempDir / I/O / testify / testify; add E2E/integration tests (S2); change the function.

Task 4: VALIDATE
  - RUN: gofmt -w internal/git/truncatediff.go internal/git/truncatediff_test.go
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test -race ./internal/git/   # the 3 updated + 4 new subtests pass; ALL pass-through invariants green
  - RUN: go test -race ./...             # full suite green (fix is internal to the shared helper)
  - FIX-FORWARD: read failures, fix, re-run.
```

### Implementation Patterns & Key Details

```go
// === truncatediff.go — the ONE-line fix (truncation branch only) ===
		if EstimateTokens(body) > allotment {
			// allotment tokens ⟺ allotment×4 runes. Rune-boundary slice + sentinel on its own line,
			// with a trailing "\n" so the next section's `diff --git` begins at a line start (FR3i).
			body = firstNRunes(body, allotment*4) + "\n" + truncatedSentinel + "\n"
		}
```

```go
// === truncatediff_test.go — the 3 assertion updates (lines 287/333/352) ===
		// BEFORE: if !strings.HasSuffix(out, "\n... [truncated]") {
		// AFTER:
		if !strings.HasSuffix(out, "\n... [truncated]\n") {
			t.Errorf("output must end with '\\n... [truncated]\\n' (own line, trailing newline); got tail=%q",
				tail(out, 40))
		}
```

```go
// === new subtest assertion shape (the regression that catches the bug) ===
		// No glue: the sentinel is NEVER immediately followed by `diff --git`.
		if strings.Contains(out, "[truncated]diff --git") {
			t.Errorf("sentinel glued to next diff --git (no newline separator); out=\n%s", out)
		}
		// The next section's header begins at a line start (preceded by \n).
		if !strings.Contains(out, "... [truncated]\ndiff --git a/b.go b/b.go") {
			t.Errorf("next diff --git not at a line start after the sentinel; out=\n%s", out)
		}
		// Exactly one sentinel per truncated file.
		if c := strings.Count(out, "... [truncated]"); c != 1 { /* == 2 for the both-truncated case */
			t.Errorf("sentinel count = %d, want %d; out=\n%s", c, 1, out)
		}
```

### Integration Points

```yaml
PRODUCTION (internal/git/truncatediff.go truncateByWaterFill):
  - truncation branch: sentinel + "\n"  (the ONLY production change)
  - signature UNCHANGED: func(sections []string, allotments map[string]int) string
  - const truncatedSentinel UNCHANGED

TESTS (internal/git/truncatediff_test.go):
  - 3 HasSuffix assertions (287/333/352): "\n... [truncated]" → "\n... [truncated]\n"
  - +4 t.Run subtests (non-md→non-md, md→non-md, non-md→md, both-truncated)

NO-TOUCH (explicitly):
  - internal/git/tokengate.go (applyWaterFillGate — caller; signature frozen)
  - internal/git/git.go (StagedDiff/TreeDiff/WorkingTreeDiff — consume via the gate; legacy token_limit==0 path is the parity ref)
  - internal/git/{numstat,skeleton,tokens,waterfill}.go   # siblings
  - difftokenlimit_test.go (E2E integration regression = S2 / P1.M1.T2.S1)
  - diff_context validation (Issue 2 = P1.M2.T1.S1)
  - docs (P1.M3.T1.S1); PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks):
  - S2 (P1.M1.T2.S1): E2E multi-section truncation regression across StagedDiff/TreeDiff/WorkingTreeDiff
    (difftokenlimit_test.go) — asserts the same no-glue property end-to-end via real git repos.
  - P1.M2.T1.S1: diff_context ∈ [0,3] validation (Issue 2).
  - P1.M3.T1.S1: docs sweep (confirm docs/how-it-works.md's `... [truncated]` mention needs no line-shape update).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/git/truncatediff.go internal/git/truncatediff_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/git/...        # Expected: exit 0
go build ./...                   # Expected: exit 0
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

# The truncation-application tests (3 updated subtests + 4 new subtests + all pass-through invariants)
go test -race -run TestTruncateByWaterFill ./internal/git/ -v

# Expected: ALL subtests pass — the 3 HasSuffix-updated subtests, the 4 new non-last-section subtests,
# and every pass-through/within-budget invariant (byte-identical preserved).
```

### Level 3: Whole-Repository Regression (no collateral)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages green (fix is internal to the shared helper)
go vet ./...                     # Expected: exit 0

# Confirm ONLY the 2 intended files changed
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/git/truncatediff.go + internal/git/truncatediff_test.go only.
```

### Level 4: The Bug-Is-Gone Check (direct assertion)

```bash
cd /home/dustin/projects/stagecoach

# The 4 new subtests directly assert no glue. Cross-check the property holds for the shared helper by
# running the whole git suite (which includes tokengate + difftokenlimit integration via real git):
go test -race ./internal/git/ -v 2>&1 | grep -E "truncated|PASS|FAIL" | head

# Expected: no FAIL; the glued shape "[truncated]diff --git" is asserted absent by the new subtests.
# (The dedicated E2E no-glue regression across the 3 diff functions lands in S2 / P1.M1.T2.S1.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.

### Feature Validation

- [ ] `truncateByWaterFill` truncation branch writes `... [truncated]\n` (trailing newline).
- [ ] No `truncateByWaterFill` output contains `[truncated]diff --git` (new subtests + existing pass).
- [ ] Every truncated section ends `... [truncated]\n`; the next `diff --git` is at a line start.
- [ ] The 3 `HasSuffix` assertions (287/333/352) updated to `"\n... [truncated]\n"` and pass.
- [ ] 4 new non-last-section subtests (non-md→non-md, md→non-md, non-md→md, both-truncated) pass.
- [ ] Pass-through/within-budget invariant tests stay byte-identical (sentinel-only fix).
- [ ] `truncateByWaterFill` signature UNCHANGED.

### Scope Discipline Validation

- [ ] ONLY `internal/git/{truncatediff,truncatediff_test}.go` modified (git diff --stat confirms).
- [ ] Did NOT edit `tokengate.go`/`git.go`/siblings (signature frozen; fix is internal).
- [ ] Did NOT add E2E/integration tests (S2 = P1.M1.T2.S1) or `diff_context` validation (P1.M2.T1.S1).
- [ ] Did NOT edit docs (P1.M3.T1.S1).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Sentinel-only fix (no `\n` added to pass-through branches).
- [ ] New subtests follow the existing pure style (string literals, t.Run, itoa/tail, no I/O/testify).
- [ ] Assertions check BOTH the negative (`![truncated]diff --git`) and the positive (`[truncated]\ndiff --git`).

---

## Anti-Patterns to Avoid

- ❌ Don't update only the contract-named assertion (line 287). THREE subtests use the identical
  `HasSuffix(out, "\n... [truncated]")` (287/333/352) and ALL break. Update ALL THREE or two tests fail.
- ❌ Don't add `\n` to the pass-through branches. The fix is SENTINEL-ONLY (the truncation branch). Within-
  budget / path-miss / pure-rename sections must stay byte-identical (6 invariant tests pin them). Adding a
  blanket "ensure each section ends with `\n`" would break the byte-identical guarantee.
- ❌ Don't change `truncateByWaterFill`'s signature, return type, or purity — it's FROZEN, consumed
  unchanged by `applyWaterFillGate` and all three diff functions.
- ❌ Don't touch the `truncatedSentinel` const — the fix is a trailing `\n` AFTER it, not a change to it.
- ❌ Don't introduce a double newline. The join is `... [truncated]\n` + `diff --git ...` = ONE separator.
  Do NOT also add a `\n` after the header block or at the start of the next section.
- ❌ Don't worry about the trailing `\n` at end-of-output when the LAST section is truncated — that's
  correct and matches the legacy `token_limit==0` path; the 3 updated assertions require it.
- ❌ Don't add E2E/integration tests or `diff_context` validation here — those are S2 / P1.M2.
- ❌ Don't truncate only the LAST section in the new tests — the bug is specifically about NON-LAST
  sections (the existing last-section tests passed even with the bug). Each new subtest MUST have a
  truncated section FOLLOWED by another section.
- ❌ Don't use `t.TempDir`, real git, I/O, or testify in the new subtests — `truncateByWaterFill` is a PURE
  function; the tests are pure string-arithmetic table cases (match the existing style).

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a one-line production fix (append `+ "\n"` after the sentinel) with the exact buggy line
and exact fixed line quoted verbatim, plus the precise 3-assertion update list (with grep-confirmed line
numbers 287/333/352) and 4 fully-specified new subtests (section literals, allotments, assertions). Two
independent de-riskings: (1) the sentinel-only nature of the fix is proven against the 6 byte-identical
pass-through invariant tests (none enter the truncation branch, so they stay green mechanically); (2) the
blast-radius grep confirms EXACTLY 3 `HasSuffix` assertions break and NO other test file (difftokenlimit/
tokengate/stagediff/treediff/workingtreediff use only Contains/Count) is affected. The one residual
uncertainty (not 10/10) is the #1 trap the contract itself set up: it named only ONE of the THREE breaking
assertions — this PRP surfaces all three explicitly with evidence, which is the deciding factor for one-pass
success. The E2E regression (S2) and `diff_context` (P1.M2) are cleanly fenced and untouched.
