package git

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

// E2E (temp-repo) tests for the FR3d/FR3i token-limit gate wired into the three diff functions
// (P1.M4.T3.S1). Reuse the repo helpers from stagediff_test.go/committree_test.go (initRepo /
// writeFile / stageFile / writeTreeOf — same package, accessible). These are NOT pure — they
// materialize a temp git repo — but they pin the END-TO-END behavior the pure tests cannot:
// (a) the ==0 regression anchor at the GATE level (the legacy byte/line-cap sentinels still appear);
// (b) the >0 water-fill: a huge file is CAPPED (the `... [truncated]` sentinel), a small file is
// WHOLE (its unique marker survives), the FR3g skeleton is present, the legacy `at N bytes` sentinel
// is ABSENT, and EstimateTokens(out) fits the budget; (c) FR3c parity — the gate is wired into all
// three diff functions (StagedDiff / TreeDiff / WorkingTreeDiff).
//
// Patterns mirror stagediff_test.go (initRepo + writeFile + stageFile + New(repo) + StagedDiff(ctx,…))
// and treediff_test.go (writeTreeOf for the two-tree domain).

// commitAllowEmpty runs `git commit --allow-empty` in dir (to establish a baseline HEAD so a working-
// tree diff has something to diff against, and so fileStatuses/detectBinaryFiles have a base). Helper.
func commitAllowEmpty(t *testing.T, dir, msg string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", msg)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit --allow-empty failed: %v\n%s", err, out)
	}
}

// embeddedDiffLiteralBody builds a non-markdown file body whose CONTENT contains `blocks`
// documented `diff --git ` literals — a realistic test-fixture / golden-snapshot / .patch file.
// Each block is a sample diff with a distinct sample filename (fa..fz cycling). Used by the
// content-embedded-literal regression tests to prove splitDiffSections does NOT fragment one real
// file into bogus sections (bugfix 002 Issue 1).
func embeddedDiffLiteralBody(blocks int) string {
	var sb strings.Builder
	for i := 0; i < blocks; i++ {
		c := string(rune('a' + (i % 26)))
		sb.WriteString("diff --git a/f" + c + " b/f" + c + "\n@@ -1 +1 @@\n-old\n+new\n\n")
	}
	return sb.String()
}

// assertContentEmbeddedLiteralNotFragmented asserts the bugfix-002 Issue-1 fix holds for a
// content-embedded-`diff --git `-literal water-fill output: (a) the payload FITS the token budget
// (FR3d), (b) the single large file is TRUNCATED (exactly 1 sentinel), (c) it is NOT fragmented
// (exactly ONE real `diff --git a/<file>` header — the bug produced hundreds of bogus sections),
// (d) the FR3g numstat skeleton is present, (e) the legacy `at N bytes/lines` sentinels are
// absent. Shared by the three ContentEmbeddedDiffGitLiteral tests.
func assertContentEmbeddedLiteralNotFragmented(t *testing.T, out string, tokenLimit int, file string) {
	t.Helper()
	// (a) FR3d contract: the payload fits the budget (skeleton/header overhead absorbed by 2× margin).
	if got := EstimateTokens(out); got > tokenLimit+2*tokenBudgetMargin {
		t.Errorf("content-embedded: payload %d tokens overflows token_limit %d (FR3d contract broken); out tail=\n%s", got, tokenLimit, tail(out, 200))
	}
	// (b) Truncation occurred: exactly 1 sentinel for the single large file.
	if c := strings.Count(out, "... [truncated]"); c != 1 {
		t.Errorf("content-embedded: expected exactly 1 truncation sentinel (the single large file capped), got %d; out tail=\n%s", c, tail(out, 200))
	}
	// (c) NOT fragmented: the single real file appears as exactly ONE section header. The bug
	//     fragmented one file into ~N bogus sections (one per embedded `diff --git ` literal).
	realHeader := "diff --git a/" + file + " b/" + file
	if c := strings.Count(out, realHeader); c != 1 {
		t.Errorf("content-embedded: expected exactly 1 real %q header (not fragmented), got %d; out tail=\n%s", realHeader, c, tail(out, 200))
	}
	// (d) The FR3g numstat skeleton is present (completeness floor).
	if !strings.Contains(out, "Change summary (numstat") {
		t.Errorf("content-embedded: FR3g skeleton missing; out=\n%s", out)
	}
	// (e) The legacy 'at N bytes/lines' sentinels are ABSENT on the token_limit>0 path.
	if strings.Contains(out, "diff truncated at") {
		t.Errorf("content-embedded: legacy 'diff truncated at N' sentinel must be ABSENT; out=\n%s", out)
	}
}

// === ==0 REGRESSION (the gate-level anchor) ==============================================

// TestStagedDiff_TokenLimitZero_LegacyCaps pins the ==0 path at the GATE level: with TokenLimit unset
// (0), the legacy per-section caps + their `... [diff truncated at N bytes/lines]` sentinels apply
// BYTE-IDENTICALLY to pre-M4 (system_context §6 invariant 1 — the regression anchor).
func TestStagedDiff_TokenLimitZero_LegacyCaps(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// A markdown file exceeding the line cap.
	writeFile(t, repo, "big.md", sdManyLines(50))
	stageFile(t, repo, "big.md")
	// A non-markdown file exceeding the byte cap.
	writeFile(t, repo, "big.go", "package main\n"+strings.Repeat("// line\n", 2000))
	stageFile(t, repo, "big.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{
		MaxMDLines:   10,
		MaxDiffBytes: 100,
		TokenLimit:   0, // UNSET ⇒ legacy caps (the regression anchor)
	})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	// The legacy line-cap sentinel (markdown) MUST be present.
	if !strings.Contains(out, "... [diff truncated at 10 lines]") {
		t.Errorf("==0: expected the legacy line-cap sentinel, got:\n%s", out)
	}
	// The legacy byte-cap sentinel (non-markdown) MUST be present.
	if !strings.Contains(out, "... [diff truncated at 100 bytes]") {
		t.Errorf("==0: expected the legacy byte-cap sentinel, got:\n%s", out)
	}
	// The water-fill sentinel MUST NOT appear on the ==0 path (system_context §6 inv 1).
	if strings.Contains(out, "... [truncated]") {
		t.Errorf("==0: the water-fill sentinel must NOT appear; got:\n%s", out)
	}
}

// === >0 WATER-FILL (the item's e2e) ======================================================

// TestStagedDiff_TokenLimitGt0_WaterFill is the item's named e2e: ONE HUGE file + ONE SMALL file,
// token_limit set small. Asserts: small file WHOLE (unique marker), huge file CAPPED
// (`... [truncated]`), FR3g skeleton present, legacy `at N bytes` sentinel ABSENT, total ≤ budget.
func TestStagedDiff_TokenLimitGt0_WaterFill(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// HUGE file: 20000 runes of generated content (a deterministic block).
	writeFile(t, repo, "huge.go", "package main\n"+strings.Repeat("// generated payload line xxxxxxxx\n", 1000))
	stageFile(t, repo, "huge.go")
	// SMALL file: a 1-line tweak with a UNIQUE marker to assert wholeness.
	writeFile(t, repo, "small.go", "package main\n// SMALL_FILE_UNIQUE_MARKER_TWEAK\n")
	stageFile(t, repo, "small.go")

	g := New(repo)
	// tokenLimit small enough that the huge file MUST be capped. The skeleton + reserve + margin are
	// subtracted; bodies share the remainder. PromptReserveTokens=0 so bodyBudget = tokenLimit − skel − margin.
	const tokenLimit = 4000
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{
		TokenLimit:          tokenLimit,
		PromptReserveTokens: 0,
	})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}

	// (a) The small file's UNIQUE marker survives ⇒ small file WHOLE.
	if !strings.Contains(out, "SMALL_FILE_UNIQUE_MARKER_TWEAK") {
		t.Errorf(">0: small file marker MISSING (small file should be whole); out=\n%s", out)
	}
	// (b) The huge file is CAPPED — the water-fill sentinel is present.
	if !strings.Contains(out, "... [truncated]") {
		t.Errorf(">0: expected the water-fill sentinel (huge file capped); out=\n%s", out)
	}
	// (c) The FR3g skeleton is present (completeness floor — prepended in BOTH branches).
	if !strings.Contains(out, "Change summary (numstat") {
		t.Errorf(">0: FR3g skeleton MISSING; out=\n%s", out)
	}
	// (d) The legacy `at N bytes/lines` sentinels are ABSENT on the >0 path (system_context §6 inv 2).
	if strings.Contains(out, "diff truncated at") {
		t.Errorf(">0: legacy 'diff truncated at N' sentinel must be ABSENT; out=\n%s", out)
	}
	// (e) The total payload fits the budget (bodies ≤ bodyBudget; skeleton/headers are overhead).
	// EstimateTokens(out) ≤ tokenLimit + a small slack for the skeleton + header overhead the gate
	// does NOT count against bodyBudget (the margin absorbs some, but the skeleton can exceed it).
	outTokens := EstimateTokens(out)
	if outTokens > tokenLimit+2*tokenBudgetMargin {
		t.Errorf(">0: EstimateTokens(out)=%d exceeds tokenLimit+2×margin=%d (bodies should fit the budget); out tail=\n%s",
			outTokens, tokenLimit+2*tokenBudgetMargin, tail(out, 200))
	}
}

// TestStagedDiff_TokenLimitGt0_AllFits covers the common case: tokenLimit LARGE ⇒ no truncation,
// every file whole (the water-fill level ≥ every size).
func TestStagedDiff_TokenLimitGt0_AllFits(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "a.go", "package main\n// a\n")
	stageFile(t, repo, "a.go")
	writeFile(t, repo, "b.go", "package lib\n// b\n")
	stageFile(t, repo, "b.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{
		TokenLimit: 100000, // huge ⇒ everything whole
	})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	if strings.Count(out, "... [truncated]") != 0 {
		t.Errorf(">0 all-fits: expected 0 sentinels, got %d; out=\n%s", strings.Count(out, "... [truncated]"), out)
	}
	if !strings.Contains(out, "package main") || !strings.Contains(out, "package lib") {
		t.Errorf(">0 all-fits: a body missing; out=\n%s", out)
	}
}

// TestStagedDiff_TokenLimitGt0_HugeMarkdown covers the markdown path under the gate: a huge .md file
// is collected UNCAPPED into mdDiffs and capped via the shared water-fill (the line cap is superseded).
func TestStagedDiff_TokenLimitGt0_HugeMarkdown(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	writeFile(t, repo, "big.md", strings.Repeat("# heading line xxxxxxxx\n", 500))
	stageFile(t, repo, "big.md")
	writeFile(t, repo, "tiny.md", "# TINY_MD_UNIQUE_MARKER\n")
	stageFile(t, repo, "tiny.md")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{
		TokenLimit: 3000, // small ⇒ big.md capped
	})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	// tiny.md whole (marker survives); big.md capped (sentinel).
	if !strings.Contains(out, "TINY_MD_UNIQUE_MARKER") {
		t.Errorf(">0 md: tiny.md marker missing (should be whole); out=\n%s", out)
	}
	if strings.Count(out, "... [truncated]") != 1 {
		t.Errorf(">0 md: expected exactly 1 sentinel (big.md capped), got %d; out=\n%s", strings.Count(out, "... [truncated]"), out)
	}
	// The legacy line-cap sentinel must NOT appear (the line cap is superseded).
	if strings.Contains(out, "diff truncated at") {
		t.Errorf(">0 md: legacy line-cap sentinel must be ABSENT; out=\n%s", out)
	}
}

// === FR3c PARITY — the gate is wired into TreeDiff and WorkingTreeDiff ====================

// TestTreeDiff_TokenLimitGt0 proves the gate is wired into TreeDiff (FR3c parity): two trees, a huge
// file diff capped under token_limit, a small file whole.
func TestTreeDiff_TokenLimitGt0(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// treeA: a baseline.
	writeFile(t, repo, "base.go", "package main\n")
	stageFile(t, repo, "base.go")
	treeA := writeTreeOf(t, repo)

	// treeB: add a HUGE file + a SMALL file.
	writeFile(t, repo, "huge.go", "package main\n"+strings.Repeat("// generated payload line xxxxxxxx\n", 800))
	stageFile(t, repo, "huge.go")
	writeFile(t, repo, "small.go", "package main\n// TREE_SMALL_UNIQUE_MARKER\n")
	stageFile(t, repo, "small.go")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{
		TokenLimit: 3000,
	})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "TREE_SMALL_UNIQUE_MARKER") {
		t.Errorf("TreeDiff >0: small file marker missing (should be whole); out=\n%s", out)
	}
	if !strings.Contains(out, "... [truncated]") {
		t.Errorf("TreeDiff >0: expected the water-fill sentinel (huge file capped); out=\n%s", out)
	}
	if !strings.Contains(out, "Change summary (numstat") {
		t.Errorf("TreeDiff >0: FR3g skeleton missing; out=\n%s", out)
	}
	if strings.Contains(out, "diff truncated at") {
		t.Errorf("TreeDiff >0: legacy sentinel must be ABSENT; out=\n%s", out)
	}
}

// assertMultiSectionTruncationFormat asserts the Issue-1 fix holds for a multi-section water-fill
// output (TWO files both truncated): no glued sentinel, the fixed sentinel+newline+diff--git join,
// both file headers survive, the FR3g skeleton is present, legacy sentinels are absent, and exactly
// 2 truncation sentinels appear (both files capped). Shared by the three MultiSection tests.
func assertMultiSectionTruncationFormat(t *testing.T, out string) {
	t.Helper()
	// (a) Bug signature GONE: the sentinel is NEVER immediately followed by `diff --git`.
	if strings.Contains(out, "[truncated]diff --git") {
		t.Errorf("multi-section: sentinel glued to next diff --git (Issue 1 regressed); out=\n%s", out)
	}
	// (b) The fixed form: a truncated section's sentinel is followed by a newline then the next diff --git.
	if !strings.Contains(out, "... [truncated]\ndiff --git") {
		t.Errorf("multi-section: expected '... [truncated]\\ndiff --git' (sentinel on own line, next header at line start); out=\n%s", out)
	}
	// (c) Both files' diff --git section headers survive.
	if !strings.Contains(out, "diff --git a/a.go b/a.go") {
		t.Errorf("multi-section: a.go diff --git header missing; out=\n%s", out)
	}
	if !strings.Contains(out, "diff --git a/b.go b/b.go") {
		t.Errorf("multi-section: b.go diff --git header missing; out=\n%s", out)
	}
	// (d) The FR3g numstat skeleton is present (completeness floor).
	if !strings.Contains(out, "Change summary (numstat") {
		t.Errorf("multi-section: FR3g skeleton missing; out=\n%s", out)
	}
	// (e) The legacy 'at N bytes/lines' sentinels are ABSENT on the token_limit>0 path.
	if strings.Contains(out, "diff truncated at") {
		t.Errorf("multi-section: legacy 'diff truncated at N' sentinel must be ABSENT; out=\n%s", out)
	}
	// Both files truncated ⇒ exactly 2 water-fill sentinels.
	if c := strings.Count(out, "... [truncated]"); c != 2 {
		t.Errorf("multi-section: expected 2 sentinels (both files capped), got %d; out=\n%s", c, out)
	}
}

// === MULTI-SECTION TRUNCATION (Issue-1 regression: TWO files both truncated) ===================
//
// The single-section tests above cap only ONE file, so the truncated section is the LAST section →
// its sentinel is never followed by another `diff --git` header, and the Issue-1 glue defect
// (... [truncated]diff --git) is invisible. These three tests stage/create TWO large files so BOTH
// are capped under a small token_limit → the FIRST section's sentinel is followed by the SECOND's
// `diff --git` header (the exact join the bug corrupted). The shared helper asserts the no-glue
// property across all three diff entry points.

// TestStagedDiff_TokenLimitGt0_MultiSection_BothTruncated pins the multi-section no-glue property
// for StagedDiff: TWO large non-markdown files, BOTH staged, BOTH truncated under a small budget.
func TestStagedDiff_TokenLimitGt0_MultiSection_BothTruncated(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	// TWO large non-markdown files, BOTH staged. Under a small token_limit BOTH are capped by the
	// water-fill → the first section's sentinel is followed by the second's diff --git (the
	// multi-section join the single-section tests never exercise — the gap that hid Issue 1).
	writeFile(t, repo, "a.go", "package main\n"+strings.Repeat("// generated payload line xxxxxxxx\n", 1000))
	stageFile(t, repo, "a.go")
	writeFile(t, repo, "b.go", "package lib\n"+strings.Repeat("// other payload line yyyyyyyy\n", 1000))
	stageFile(t, repo, "b.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{
		TokenLimit:          4000,
		DiffContext:         1,
		PromptReserveTokens: 0,
	})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	assertMultiSectionTruncationFormat(t, out)
}

// TestTreeDiff_TokenLimitGt0_MultiSection_BothTruncated pins the multi-section no-glue property for
// TreeDiff: treeA (base.go) vs treeB (base + TWO large files), BOTH new files truncated.
func TestTreeDiff_TokenLimitGt0_MultiSection_BothTruncated(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	// treeA: a baseline (base.go only).
	writeFile(t, repo, "base.go", "package main\n")
	stageFile(t, repo, "base.go")
	treeA := writeTreeOf(t, repo)
	// treeB: add TWO large files.
	writeFile(t, repo, "a.go", "package main\n"+strings.Repeat("// generated payload line xxxxxxxx\n", 1000))
	stageFile(t, repo, "a.go")
	writeFile(t, repo, "b.go", "package lib\n"+strings.Repeat("// other payload line yyyyyyyy\n", 1000))
	stageFile(t, repo, "b.go")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{
		TokenLimit:          4000,
		DiffContext:         1,
		PromptReserveTokens: 0,
	})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	assertMultiSectionTruncationFormat(t, out)
}

// TestWorkingTreeDiff_TokenLimitGt0_MultiSection_BothTruncated pins the multi-section no-glue
// property for WorkingTreeDiff: a tracked baseline commit then unstaged bloat of TWO tracked files.
func TestWorkingTreeDiff_TokenLimitGt0_MultiSection_BothTruncated(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	// Baseline: commit base.go + a.go + b.go (small, TRACKED) so working-tree changes appear in
	// `git diff`. Untracked files do NOT appear in `git diff`.
	writeFile(t, repo, "base.go", "package main\n")
	stageFile(t, repo, "base.go")
	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	writeFile(t, repo, "b.go", "package lib\n")
	stageFile(t, repo, "b.go")
	commitAllowEmpty(t, repo, "baseline")
	// UNSTAGED working-tree bloat of BOTH tracked files → both large → both capped under the budget.
	writeFile(t, repo, "a.go", "package main\n"+strings.Repeat("// unstaged payload line xxxxxxxx\n", 1000))
	writeFile(t, repo, "b.go", "package lib\n"+strings.Repeat("// other unstaged line yyyyyyyy\n", 1000))

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{
		TokenLimit:          4000,
		DiffContext:         1,
		PromptReserveTokens: 0,
	})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	assertMultiSectionTruncationFormat(t, out)
}

// === CONTENT-EMBEDDED `diff --git ` LITERAL (bugfix 002 Issue-1 regression) ===================
//
// The existing tests stage content free of `diff --git ` literals, so the fragmentation defect was
// never exercised E2E: a single non-markdown file whose BODY embeds many `diff --git ` literals (a
// realistic fixture / golden-snapshot / .patch file) was torn into bogus sections by the un-anchored
// split → sized tiny → truncated nothing → payload SILENTLY overflowed token_limit (~7× measured at
// token_limit=2000). The S1 fix (line-anchored `(?m)^diff --git ` regex slice in truncatediff.go:55)
// keeps that file ONE section, sized, and truncated to fit. These three tests PIN the fix across all
// three diff entry points by staging ONE such file under a small token_limit and asserting (via the
// shared helper) fits-budget + truncated + not-fragmented + skeleton + no-legacy.

// TestStagedDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral pins the Issue-1 fix for StagedDiff:
// ONE non-md file whose content embeds many `diff --git ` literals, staged, under a small token_limit.
func TestStagedDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	// ONE non-md file whose CONTENT embeds many `diff --git ` literals (a fixture/snapshot/.patch).
	// Under a small token_limit the file MUST be capped as ONE section (the un-anchored split
	// fragmented it into bogus sections, silently overflowing the budget — bugfix 002 Issue 1).
	writeFile(t, repo, "fixtures.diff", embeddedDiffLiteralBody(300))
	stageFile(t, repo, "fixtures.diff")

	g := New(repo)
	const tokenLimit = 2000
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{
		TokenLimit:          tokenLimit,
		PromptReserveTokens: 0,
	})
	if err != nil {
		t.Fatalf("StagedDiff err = %v, want nil", err)
	}
	assertContentEmbeddedLiteralNotFragmented(t, out, tokenLimit, "fixtures.diff")
}

// TestTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral pins the Issue-1 fix for TreeDiff:
// treeA (base.go) vs treeB (base + the embedded-literal fixture), under a small token_limit.
func TestTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	// treeA: a baseline (base.go only).
	writeFile(t, repo, "base.go", "package main\n")
	stageFile(t, repo, "base.go")
	treeA := writeTreeOf(t, repo)
	// treeB: add the embedded-literal fixture.
	writeFile(t, repo, "fixtures.diff", embeddedDiffLiteralBody(300))
	stageFile(t, repo, "fixtures.diff")
	treeB := writeTreeOf(t, repo)

	g := New(repo)
	const tokenLimit = 2000
	out, err := g.TreeDiff(context.Background(), treeA, treeB, StagedDiffOptions{
		TokenLimit:          tokenLimit,
		PromptReserveTokens: 0,
	})
	if err != nil {
		t.Fatalf("TreeDiff err = %v, want nil", err)
	}
	assertContentEmbeddedLiteralNotFragmented(t, out, tokenLimit, "fixtures.diff")
}

// TestWorkingTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral pins the Issue-1 fix for
// WorkingTreeDiff: a tracked baseline commit then unstaged bloat of the tracked fixture with the
// embedded-literal body, under a small token_limit.
func TestWorkingTreeDiff_TokenLimitGt0_ContentEmbeddedDiffGitLiteral(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	// Baseline: commit base.go + fixtures.diff (small, TRACKED) so working-tree changes appear in
	// `git diff`. Untracked files do NOT appear in `git diff`.
	writeFile(t, repo, "base.go", "package main\n")
	stageFile(t, repo, "base.go")
	writeFile(t, repo, "fixtures.diff", "seed\n")
	stageFile(t, repo, "fixtures.diff")
	commitAllowEmpty(t, repo, "baseline")
	// UNSTAGED working-tree bloat of the tracked fixture with the embedded-literal body.
	writeFile(t, repo, "fixtures.diff", embeddedDiffLiteralBody(300))

	g := New(repo)
	const tokenLimit = 2000
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{
		TokenLimit:          tokenLimit,
		PromptReserveTokens: 0,
	})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	assertContentEmbeddedLiteralNotFragmented(t, out, tokenLimit, "fixtures.diff")
}

// TestWorkingTreeDiff_TokenLimitGt0 proves the gate is wired into WorkingTreeDiff (FR3c parity): a
// baseline commit + an unstaged huge + small change, token_limit set.
func TestWorkingTreeDiff_TokenLimitGt0(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// Baseline: commit base.go + the two target files as TRACKED, so working-tree changes show up in
	// `git diff` (working-tree-vs-index). Untracked files do NOT appear in `git diff`.
	writeFile(t, repo, "base.go", "package main\n")
	stageFile(t, repo, "base.go")
	writeFile(t, repo, "huge.go", "package main\n")
	stageFile(t, repo, "huge.go")
	writeFile(t, repo, "small.go", "package main\n")
	stageFile(t, repo, "small.go")
	commitAllowEmpty(t, repo, "baseline") // commit the staged base (all three files tracked now)

	// Now make UNSTAGED working-tree changes to the tracked files: bloat huge.go, tweak small.go.
	writeFile(t, repo, "huge.go", "package main\n"+strings.Repeat("// unstaged payload line xxxxxxxx\n", 800))
	writeFile(t, repo, "small.go", "package main\n// WT_SMALL_UNIQUE_MARKER\n")

	g := New(repo)
	out, err := g.WorkingTreeDiff(context.Background(), StagedDiffOptions{
		TokenLimit: 3000,
	})
	if err != nil {
		t.Fatalf("WorkingTreeDiff err = %v, want nil", err)
	}
	if !strings.Contains(out, "WT_SMALL_UNIQUE_MARKER") {
		t.Errorf("WorkingTreeDiff >0: small file marker missing (should be whole); out=\n%s", out)
	}
	if !strings.Contains(out, "... [truncated]") {
		t.Errorf("WorkingTreeDiff >0: expected the water-fill sentinel (huge file capped); out=\n%s", out)
	}
	if !strings.Contains(out, "Change summary (numstat") {
		t.Errorf("WorkingTreeDiff >0: FR3g skeleton missing; out=\n%s", out)
	}
	if strings.Contains(out, "diff truncated at") {
		t.Errorf("WorkingTreeDiff >0: legacy sentinel must be ABSENT; out=\n%s", out)
	}
}
