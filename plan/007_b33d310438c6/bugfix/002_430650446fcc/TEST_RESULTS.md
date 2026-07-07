# Bug Fix Requirements

## Overview

Creative end-to-end validation of the **FR3d–FR3i diff-payload optimization** delta (PRD §9.1) in
plan `007_b33d310438c6`, as a *second* adversarial pass after bugfix `001_7e79f5773da8` (whose two
findings — sentinel-glued-to-next-section and out-of-range `diff_context` — are confirmed fixed in
commits `a004b12` and `b7f723d`).

Testing approach: I read the implementation end-to-end (`tokens.go`, `waterfill.go`, `tokengate.go`,
`truncatediff.go`, `numstat.go`, `skeleton.go`, the three diff functions in `git.go`, the config-layer
`*int` plumbing, and the six call sites' `PromptReserveTokens` measurement), confirmed the full existing
`internal/git` test suite passes, then wrote **~16 new adversarial integration probes** against a real
temp git repo plus targeted unit probes, exercising code paths the existing tests miss.

**Overall assessment:** The implementation remains high quality. Verified working (probes PASS): the
`token_limit==0` legacy regression anchor; the `>0` water-fill fairness (small file whole, large files
capped at a shared level, `body==allotment` boundary passes through); the `minBodyTokens` degenerate
floor; multi-section truncation boundaries (the prior sentinel-gluing fix is complete across md→md,
md→non-md, and non-md→non-md); true `-M` rename collapse (1 skeleton row + `rename from`/`rename to`);
new-file / deleted-file index-line stripping; `diff_context` 0-vs-unset `*int` semantics; and the
`PromptReserveTokens` worst-case measurement at all six call sites.

However, **one systemic Major bug** was found in the FR3i section-splitting primitive
(`splitDiffSections`), affecting all three diff functions (StagedDiff / TreeDiff / WorkingTreeDiff). It
is reproducible 100% of the time for its trigger, and it **silently defeats the `token_limit` truncation**
— the exact contract FR3d exists to uphold ("the payload always fits your model's context window").

## Critical Issues (Must Fix)

None. The default `token_limit==0` flow and the core commit-generation flow are unaffected; no bug
prevents a commit from being produced. The Major issue below is scoped to the opt-in `token_limit > 0`
mode interacting with a realistic class of file content.

## Major Issues (Should Fix)

### Issue 1: `splitDiffSections` fragments files whose content contains the literal `diff --git `, defeating `token_limit` truncation and overflowing the context window

**Severity**: Major
**PRD Reference**: §9.1 FR3d ("the payload always fits **without stagecoach maintaining a per-model context registry**"; "When set … the payload always fits"); §9.1 FR3i ("the aggregate non-markdown diff is split on `diff --git` boundaries to apply the per-file level without extra `git` invocations"); §9.1 FR3c (parity — the bug is replicated across all three diff paths); system_context.md §6 invariant 2 (the `>0` truncation contract).
**Affected code**: `internal/git/truncatediff.go` → `splitDiffSections` (the `strings.Split(diff, "diff --git ")` call); consumed by `applyWaterFillGate` (`internal/git/tokengate.go`) in the `opts.TokenLimit > 0` branch of `StagedDiff` (git.go:883), `TreeDiff` (git.go:1360), and `WorkingTreeDiff` (git.go:1533).

**Root cause**: `splitDiffSections` splits the captured non-markdown aggregate on the **un-anchored** substring `"diff --git "`:

```go
// internal/git/truncatediff.go
parts := strings.Split(diff, "diff --git ")   // <-- splits on ANY occurrence, not just line starts
```

A **content line** inside a file's diff body — e.g. an added line documenting or fixture-ing a git diff
(`+diff --git a/foo b/foo`) — contains the literal `diff --git ` and is therefore treated as a section
boundary. One real file is fragmented into many bogus tiny sections. (The sibling helpers in the same
file are correctly **line-anchored** and are immune: `diffSectionHeaderRe` is `(?m)^diff --git a/(.*) b/(.*)$`
and `atAtRe` is `(?m)^@@`. Only `splitDiffSections` is un-anchored.)

**Expected Behavior**: Under `token_limit > 0`, when a non-markdown file's body is larger than its
water-fill allotment, that file's body is truncated to its first `allotment×4` runes + the
`... [truncated]` sentinel, and the **total payload fits within `token_limit`** (the FR3d contract). The
section split must identify only REAL file boundaries (`diff --git ` at column 0 / line start), never a
`diff --git ` that appears inside an added/removed/context line of another file.

**Actual Behavior**: When any changed non-markdown file's content contains the substring `diff --git `
(with the trailing space), the file is fragmented into bogus sections. Because each fragment is tiny, the
water-fill sizes them as tiny, concludes the change set fits the body budget, and **truncates nothing**.
The un-truncated full content is then re-emitted (the split→re-prefix round-trip is accidentally
lossless when nothing is truncated), so the payload **silently overflows `token_limit`** — the model's
context window — by an unbounded factor.

**Measured overflows** (integration probes against a real `git init` temp repo, `token_limit=2000` /
`1500`, `PromptReserveTokens=0`, single non-markdown file whose content is N documented diff blocks):

| Scenario | token_limit | actual payload tokens | overflow | sentinels emitted |
|---|---|---|---|---|
| 500-block content file | 2000 | **14543** | **+12543 (~7×)** | 0 (should be ≥1) |
| mixed: small `.go` + 300-block `.diff` | 1500 | **3458** | **+1958** | 0 |
| TreeDiff (FR3c parity), 300-block file | 1500 | **3422** | **+1922** | 0 |

In every case the water-fill emitted **zero** `... [truncated]` sentinels despite the payload vastly
exceeding the limit. The small legitimate file in the mixed case correctly survived whole, but the large
file was not truncated at all. The TreeDiff result confirms FR3c parity replicates the bug across all
three diff functions.

**Scope / blast radius**:
- Triggered only when `token_limit > 0` (the default `0`/unset path uses the legacy whole-string
  byte-cap and never calls `splitDiffSections`, so it is **unaffected**).
- Triggered only by **non-markdown** files. Markdown files are collected per-file into `mdDiffs` and are
  **immune** (they are not passed through `splitDiffSections`; their truncation uses the line-anchored
  `diffSectionPath`/`atAtRe`).
- Realistic triggers (non-markdown files whose content contains `diff --git `): **test fixtures / golden
  snapshots that embed sample diffs, documentation showing git diffs, vendored `.patch`/`.diff` files,
  the source of git/diff-related tooling, and changelogs quoting diffs.** None of these are exotic.

**Impact**:
- **Breaks FR3d's core promise** ("payload always fits your context window") precisely for the users who
  opt into `token_limit` because their model has a small context window — the population for whom an
  overflow is most costly.
- When the overflowing payload is fed to the agent, the agent may **error out, refuse to process the
  request, or silently truncate its own input** (losing the tail of the diff), any of which yields a
  degraded or failed commit message. (The committed content is still safe — all transforms are
  payload-only — so this is a quality/reliability defect, not data corruption.)
- It is a **silent** failure: no error, no sentinel, no diagnostic — the payload just ships oversized.

**Steps to Reproduce** (drop-in test against a real temp repo; ~10 lines):

```go
func TestRepro_SplitDiffSections_DefatsTokenLimit(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo) // existing helper
	// A non-markdown file whose CONTENT contains the literal "diff --git " (a documented diff).
	var sb strings.Builder
	for i := 0; i < 500; i++ {
		c := string(rune('a' + (i % 26)))
		sb.WriteString("diff --git a/f" + c + " b/f" + c + "\n@@ -1 +1 @@\n-old\n+new\n\n")
	}
	writeFile(t, repo, "fixtures.diff", sb.String())
	stageFile(t, repo, "fixtures.diff")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{
		TokenLimit: 2000, PromptReserveTokens: 0,
	})
	if err != nil { t.Fatal(err) }
	got := EstimateTokens(out)
	// FAILS today: got=14543 >> 2000; strings.Count(out, "... [truncated]")==0.
	if got > 2000+tokenBudgetMargin {
		t.Errorf("payload %d tokens overflows token_limit 2000 (FR3d contract broken)", got)
	}
}
```

Unit-level confirmation of the root cause (one real file → two sections):

```go
func TestRepro_SplitDiffSections_Unit(t *testing.T) {
	nmDiff := "diff --git a/real.txt b/real.txt\n@@ -0,0 +1,1 @@\n+diff --git a/foo b/foo\n"
	secs := splitDiffSections(nmDiff) // returns 2 sections; want 1
	// section[0] = the real file, BODY CUT mid-content at "+"
	// section[1] = bogus "diff --git a/foo b/foo" section
}
```

**Suggested Fix**: Make `splitDiffSections` split only on `diff --git ` at a **line start**, matching the
line-anchored approach already used by its siblings `diffSectionHeaderRe`/`atAtRe`. Two viable shapes:

1. Split on `"\ndiff --git "` (the newline preceding a real header) and treat the leading section
   specially (it has no preceding newline; today's code already special-cases the leading element —
   re-prefix the first real section and drop a truly-empty prefix).
2. Use `regexp.MustCompile("(?m)^diff --git ").FindAllStringIndex(diff, -1)` to slice at real boundaries
   only; a content line `+diff --git a/foo b/foo` does not match `^diff --git` (it starts with `+`).

Either makes content-embedded `diff --git ` literals inert, so each real file is one section, sized and
truncated as a unit. Then add a regression test that stages a non-markdown file whose content contains
`diff --git ` literals under a truncating `token_limit` and asserts (a) the payload fits the budget and
(b) exactly one `... [truncated]` sentinel is emitted for that single large file. Apply the fix once in
`splitDiffSections` so all three diff functions (and the shared gate) are covered. (Markdown files need
no change — they are already immune.)

## Minor Issues (Nice to Fix)

None rise to the Minor bar after this pass. Two observations recorded for the implementer's awareness,
neither a defect requiring action:

- **Binary / excluded placeholders are not subtracted from the body budget.** They are written to the
  payload between the skeleton and the gate output but are not counted in
  `body_budget = token_limit − skeleton − reserve − margin`. This is **by design** — the
  `tokenBudgetMargin` (1024) explicitly absorbs them (tokengate.go comment item (c)) — and a typical
  commit's handful of placeholders is far below that margin. Only a commit with a very large number of
  binary/excluded files (hundreds) could approach the margin; negligible in practice.

- **`firstNRunes` rune-safety** was re-verified correct (it iterates `for i := range s`, yielding rune
  start bytes, so it never splits a UTF-8 codepoint). No action required — noted only because the prior
  round carried it as an open item.

## Testing Summary

- **Total tests performed**: ~16 new adversarial integration probes (StagedDiff / TreeDiff /
  WorkingTreeDiff × {multi-file truncation boundaries, true rename collapse, rename-with-edits, new-file
  index-line strip, deleted-file, multi-markdown truncation, degenerate `minBodyTokens`, water-fill
  fairness, content-with-`diff --git `-literal × {no-truncation, under-truncation, large-overflow,
  mixed-files, TreeDiff-parity}}) + 1 unit-level `splitDiffSections` probe + full re-run of the existing
  `internal/git` suite.
- **Passing**: All existing unit + golden tests; `token_limit==0` legacy regression; multi-section
  truncation boundaries (prior fix confirmed complete); true `-M` rename collapse; new-file/deleted-file
  index-line stripping; water-fill fairness and boundary; degenerate floor; config `*int` 0-vs-unset.
- **Failing**: 1 systemic issue (Issue 1 — `splitDiffSections` fragmentation defeats truncation;
  reproduced in StagedDiff, TreeDiff, and WorkingTreeDiff).
- **Areas with good coverage**: pure solver (`waterFillLevel`/`allocByWaterFill`), token estimator, gate
  budget arithmetic, config plumbing, single- and multi-section truncation formatting, legacy regression.
- **Areas needing more attention**: **`splitDiffSections` robustness against content that resembles git
  diff headers** (the gap that hid Issue 1) — the existing `truncatediff_test.go` `splitDiffSections`
  cases use only clean multi-file aggregates where every `diff --git ` is a real header; add a case with
  a content-embedded `diff --git ` literal under a truncating budget. This is the regression net for the
  fix above.
