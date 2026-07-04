# Bug Fix Requirements

## Overview

Creative end-to-end validation of the **FR3d–FR3i diff-payload optimization** delta (PRD §9.1) implemented in plan `007_b33d310438c6`. The delta adds six diff transforms across `StagedDiff` / `TreeDiff` / `WorkingTreeDiff` in `internal/git/`: a holistic `token_limit` overlay (FR3d), deterministic `-M` rename detection (FR3e), reduced `-U<diff_context>` context (FR3f), a compact `--numstat` change skeleton (FR3g), index-line stripping (FR3h), and dynamic water-fill truncation (FR3i).

Testing approach: I read the implementation end-to-end (`tokens.go`, `waterfill.go`, `tokengate.go`, `truncatediff.go`, `numstat.go`, `skeleton.go`, the three diff functions in `git.go`, the config-layer `*int` plumbing, and the six call sites' `PromptReserveTokens` measurement), confirmed the full existing test suite passes, then wrote **adversarial integration probes** against a real temp git repo to exercise code paths the unit tests miss.

**Overall assessment:** The implementation is high quality — the config-layer `*int` 0-vs-unset disambiguation is correct, the legacy `token_limit==0` path is byte-identical (regression invariants hold), rename detection and index-line stripping work for added/deleted/modified files, binary placeholders survive truncation, the skeleton's completeness floor holds, the water-fill boundary handling is exact (`>` strict, body==allotment passes through), and the `PromptReserveTokens` empty-diff-trick measurement is consistent across all six call sites.

However, **one systemic Major bug** was found in the FR3i truncation output formatting, affecting all three diff functions. It is reproducible 100% of the time and produces a malformed payload sent to the agent.

## Critical Issues (Must Fix)

None. The core generation flow is functional; no bug prevents a commit from being produced.

## Major Issues (Should Fix)

### Issue 1: Truncated sections are glued to the next `diff --git` with no newline separator

**Severity**: Major
**PRD Reference**: §9.1 FR3i ("each file's `diff --git`/hunk headers are always preserved alongside its (possibly truncated) body"; "Recompose the sections in original order"); system_context.md §6 invariant 2 (the `... [truncated]` sentinel shape)
**Affected code**: `internal/git/truncatediff.go` → `truncateByWaterFill`; consumed by `applyWaterFillGate` (tokengate.go) in the `opts.TokenLimit > 0` branch of `StagedDiff`, `TreeDiff`, and `WorkingTreeDiff` (git.go lines ~883, ~1360, ~1533).
**Expected Behavior**: When `token_limit > 0` and the water-fill caps a file's body, the file's section ends with `... [truncated]` on its own line, and the **next** file's section begins on a fresh line starting with `diff --git a/... b/...`. The same line-shape discipline that the existing unit test `item_sentinel_on_its_own_line` asserts for the LAST section (`\n... [truncated]` suffix) should hold for every intermediate truncated section too. The legacy `token_limit==0` markdown path already enforces this (`if !strings.HasSuffix(fileDiff, "\n") { b.WriteByte('\n') }`).
**Actual Behavior**: `truncateByWaterFill` writes `headerBlock + (firstNRunes(body, allotment*4) + "\n" + truncatedSentinel)` — the sentinel has **no trailing newline**, and the loop immediately appends the next section. The result is that a truncated section's sentinel is glued to the next section's `diff --git` header on a single line:

```
... [truncated]diff --git a/b.go b/b.go
```

Concrete captured output (StagedDiff, two non-markdown files both capped under a small `token_limit`):

```
"...\n@@ -1,2 +1,2 @@\n package main\n-v\n... [truncated]diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1,2 +1,2 @@\n package main\n-v\n... [truncated]"
```

The same defect occurs for a truncated **markdown** file followed by any other file, and for a truncated non-markdown file followed by another file. It is **not** visible in the existing `truncatediff_test.go` cases because every one of them truncates only the *last* (or only) section — the intermediate-section concatenation is never exercised. The `item_3file_one_over_budget_truncated_others_byte_identical` test truncates only B (the middle section) and asserts on `Contains`/`Count`, which both pass even though B's sentinel is glued to C's header.

**Steps to Reproduce**:
1. In any repo, stage two files whose combined diff bodies exceed a small `token_limit`, e.g.:
   ```go
   opts := git.StagedDiffOptions{
       MaxDiffBytes: 300000, MaxMDLines: 100,
       TokenLimit: 600, DiffContext: 1, PromptReserveTokens: 0,
   }
   // repo has a.go and b.go both modified with large bodies, both staged
   out, _ := g.StagedDiff(ctx, opts)
   ```
2. Observe `out` contains the literal substring `... [truncated]diff --git`.
3. The identical defect reproduces via `TreeDiff(treeA, treeB, opts)` and `WorkingTreeDiff(ctx, opts)` — the gate is shared.

**Impact**:
- The agent receives a malformed line `... [truncated]diff --git a/b.go b/b.go` where the next file's section header is no longer at a line start. Depending on the agent's diff parsing, the section boundary is obscured and the model may mis-attribute hunks to the wrong file or drop the glued-together line.
- It defeats the visual completeness floor: a human inspecting `--dry-run` output sees run-together text.
- It is a regression vs. the legacy `token_limit==0` path, which always inserts the separating newline.

**Suggested Fix**: In `truncateByWaterFill` (`internal/git/truncatediff.go`), when a body is truncated, append a trailing newline after the sentinel before the next section is written — e.g. write `firstNRunes(body, allotment*4) + "\n" + truncatedSentinel + "\n"`. Alternatively (and more uniformly), ensure each recomposed section ends with a newline before appending the next. Then add a regression test that truncates a *non-last* section and asserts the sentinel is followed by `\n` (not by `diff --git`), e.g. `!strings.Contains(out, "[truncated]diff --git")` and that the next section's `diff --git` line begins at a line start. The fix should be applied once in `truncateByWaterFill` so all three diff functions and the markdown path are covered by a single change.

## Minor Issues (Nice to Fix)

### Issue 2: `diff_context` out-of-range values are silently clamped rather than validated at the config layer

**Severity**: Minor
**PRD Reference**: §9.1 FR3f ("integer `0`–`3`")
**Expected Behavior**: An out-of-range `diff_context` (e.g. `4`, `-1`) is either rejected with a clear config error or at minimum documented as clamped.
**Actual Behavior**: The config layer (`internal/config`) accepts any integer; `buildDiffArgs` (`internal/git/git.go` line ~689) silently clamps out-of-range values to `1`. There is no diagnostic. A user who sets `diff_context = 5` (a typo for, say, a future value) gets `-U1` with no feedback.
**Steps to Reproduce**: Set `diff_context = 5` in config; observe the diff uses `-U1` with no warning.
**Suggested Fix**: Either validate `diff_context ∈ [0,3]` at config materialize/overlay time and return an error, or emit a one-line stderr warning when clamping fires. Low priority — the clamp is defensive and the value space is small.

### Issue 3: `firstNRunes` truncation can split a UTF-8 codepoint's *logical* representation mid-hunk on byte-aligned content

**Severity**: Minor (theoretical — rune-boundary-safe in practice)
**PRD Reference**: §9.1 FR3i
**Expected Behavior**: Truncation preserves valid UTF-8 boundaries.
**Actual Behavior**: `firstNRunes` correctly iterates rune boundaries (it uses `for i := range s`), so it does NOT split a rune. This is actually correct. *Self-correction during review: no bug here.* Leaving the note only to record that the rune-safety was explicitly verified — no action required.

## Testing Summary

- **Total tests performed**: ~15 new adversarial integration probes (StagedDiff / TreeDiff / WorkingTreeDiff × markdown / non-markdown / mixed / deletion / new-file / spaces-in-name / binary / rename / extreme-small-token-limit / boundary / coherence), plus full re-run of the existing `internal/git` suite and a build of `./...`.
- **Passing**: All existing unit + golden tests; legacy `token_limit==0` regression; rename detection; index-line stripping (added/deleted/modified); binary placeholder survival under truncation; skeleton completeness floor; water-fill boundary (`body==allotment` passes through); extreme-small `token_limit` (minBodyTokens floor path); config `*int` 0-vs-unset semantics (verified by code reading); docs/configuration.md + bootstrap template updated.
- **Failing**: 1 (Issue 1 — sentinel-abuts-next-section, reproduced in all three diff functions).
- **Areas with good coverage**: pure-function solver (`waterFillLevel`/`allocByWaterFill`), token estimator, gate budget arithmetic, config plumbing, single-section truncation, legacy regression.
- **Areas needing more attention**: **multi-section concatenation in `truncateByWaterFill`** (the gap that hid Issue 1) — add a test that truncates a non-last section. Also worth adding: a test that exercises the markdown `mdDiffs` path under `token_limit` with >1 markdown file (today's tests cover markdown-only-uncapped-collect indirectly but not multi-md truncation ordering).
