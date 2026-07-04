package git

import (
	"fmt"
	"strings"
	"testing"
)

// Pure table tests for the FR3d/FR3i token-limit gate (applyWaterFillGate / sectionBody /
// tokenBudgetMargin). Mirror truncatediff_test.go's style: table-driven, HARDCODED expectations,
// t.Run subtests, PURE (no git repo, no t.TempDir, no I/O). Every case is a string literal — the
// gate under test is pure string/budget arithmetic. (PRP Task 2 / truncatediff_test.go pattern.)
//
// All section literals use explicit \n — the index-stripped captured-section shape (no `index` line;
// e.g. `diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n`).
//
// Sizing determinism: to make body sizes predictable, build bodies of KNOWN rune length. EstimateTokens
// is ceil(runes/4), so a body of N runes estimates at ceil(N/4) tokens. We assert STRUCTURE
// (Contains/Count of the `... [truncated]` sentinel, body-content survival) rather than exact bytes
// where truncation is involved, because the cutoff is allotment×4 runes and the exact prefix length is
// not the property under test — the property is that the LARGE file is capped and the SMALL file is whole.

// bodySection builds a self-contained `diff --git a/<path> b/<path>` section whose BODY (from the first
// @@ onward) is at least `bodyRunes` runes of filler content. The filler is ASCII so 1 rune = 1 byte.
// Used to manufacture sections with a known-large EstimateTokens(body) for deterministic sizing. The
// exact body token count is not asserted; only the over/under-budget relations, which the cases pin.
func bodySection(path string, bodyRunes int) string {
	hdr := "diff --git a/" + path + " b/" + path + "\n--- a/" + path + "\n+++ b/" + path + "\n@@ -1 +1 @@\n"
	if bodyRunes <= len("@@ -1 +1 @@\n") {
		return hdr // body is just the @@ header; small but non-zero EstimateTokens
	}
	// The filler is on content lines (8 'x' runes + newline per line ≈ a known block). The body rune count
	// is hdr-body + filler ≥ bodyRunes, so EstimateTokens(body) is large and predictable.
	fill := strings.Repeat("xxxxxxxx\n", (bodyRunes-len("@@ -1 +1 @@\n"))/9+1)
	return hdr + fill
}

// TestSectionBody covers the pure body-extractor: @@ onward, no-@@ (rename) → "", exact substring.
func TestSectionBody(t *testing.T) {
	tests := []struct {
		desc    string
		section string
		want    string
	}{
		{
			desc:    "body starts at the first @@ line",
			section: "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n",
			want:    "@@ -1 +1 @@\n-old\n+new\n",
		},
		{
			desc:    "no @@ (pure rename) → empty body",
			section: "diff --git a/old b/new\nsimilarity index 100%\nrename from old\nrename to new\n",
			want:    "",
		},
		{
			desc:    "empty section → empty body",
			section: "",
			want:    "",
		},
		{
			desc:    "multi-hunk: body is the FIRST @@ onward (includes later hunks)",
			section: "diff --git a/x b/x\n@@ -1 +1 @@\n-a\n+a2\n@@ -5 +5 @@\n-b\n+b2\n",
			want:    "@@ -1 +1 @@\n-a\n+a2\n@@ -5 +5 @@\n-b\n+b2\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			if got := sectionBody(tc.section); got != tc.want {
				t.Errorf("sectionBody(%q) = %q, want %q", tc.section, got, tc.want)
			}
		})
	}
}

// TestApplyWaterFillGate covers the FR3d/FR3i gate: budget arithmetic (skeleton + reserve + margin
// subtracted), sizing, allocation, truncation integration, fairness, shared md/non-md budget, and the
// degenerate + edge cases. Pure; no I/O.
func TestApplyWaterFillGate(t *testing.T) {
	t.Run("Empty_no_sections_returns_empty", func(t *testing.T) {
		if got := applyWaterFillGate(nil, "", "", 4000, 0); got != "" {
			t.Errorf("applyWaterFillGate(nil,'','',…) = %q, want %q", got, "")
		}
		if got := applyWaterFillGate(nil, "", "skeleton", 4000, 0); got != "" {
			t.Errorf("applyWaterFillGate(nil,'','skeleton',…) = %q, want %q", got, "")
		}
	})

	t.Run("AllWithinBudget_no_truncation_byte_identical", func(t *testing.T) {
		// Two small sections; tokenLimit large ⇒ both whole ⇒ no sentinel ⇒ both verbatim.
		s1 := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-a\n+a2\n"
		s2 := "diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1 +1 @@\n-b\n+b2\n"
		// nmDiff = two sections concatenated.
		nmDiff := s2
		out := applyWaterFillGate([]string{s1}, nmDiff, "", 4000, 0)
		if strings.Count(out, "... [truncated]") != 0 {
			t.Errorf("expected 0 sentinels (all within budget), got %d; out=\n%s", strings.Count(out, "... [truncated]"), out)
		}
		if !strings.Contains(out, s1) {
			t.Errorf("s1 not verbatim in output; missing %q", s1)
		}
		if !strings.Contains(out, s2) {
			t.Errorf("s2 not verbatim in output; missing %q", s2)
		}
	})

	t.Run("OneLargeCapped_fairness_small_files_whole", func(t *testing.T) {
		// A and C small (a unique marker line, ~3 tokens). B LARGE (4000 runes ⇒ ~450 tokens).
		// tokenLimit set so bodyBudget is between (sizeA+sizeC) and sizeB ⇒ A,C whole, B capped.
		sA := "diff --git a/a/A.go b/a/A.go\n--- a/a/A.go\n+++ b/a/A.go\n@@ -1 +1 @@\n+MARKER_A_UNIQUE\n"
		sB := bodySection("b/B.go", 4000)
		sC := "diff --git a/c/C.go b/c/C.go\n--- a/c/C.go\n+++ b/c/C.go\n@@ -1 +1 @@\n+MARKER_C_UNIQUE\n"
		// Skeleton "" + reserve 0 ⇒ bodyBudget = tokenLimit − margin.
		// Pick tokenLimit so bodyBudget = 100 (tokenLimit = 100 + margin). 100 > ~6 (A+C whole),
		// 100 < ~450 (B capped). The unique markers of A/C survive; B's section header survives + sentinel.
		tokenLimit := tokenBudgetMargin + 100
		out := applyWaterFillGate([]string{sA}, sB+sC, "", tokenLimit, 0)

		// A and C bodies WHOLE: their unique markers survive (not truncated).
		if !strings.Contains(out, "MARKER_A_UNIQUE") {
			t.Errorf("A body marker missing (A should be whole); out=\n%s", out)
		}
		if !strings.Contains(out, "MARKER_C_UNIQUE") {
			t.Errorf("C body marker missing (C should be whole); out=\n%s", out)
		}
		// Assert A and C section headers present.
		if !strings.Contains(out, "diff --git a/a/A.go b/a/A.go") {
			t.Errorf("A header missing; out=\n%s", out)
		}
		if !strings.Contains(out, "diff --git a/c/C.go b/c/C.go") {
			t.Errorf("C header missing; out=\n%s", out)
		}
		// Exactly ONE sentinel (only B capped).
		if c := strings.Count(out, "... [truncated]"); c != 1 {
			t.Errorf("sentinel count = %d, want 1 (only B capped); out=\n%s", c, out)
		}
		// B's headers preserved.
		if !strings.Contains(out, "diff --git a/b/B.go b/b/B.go") {
			t.Errorf("B header NOT preserved; out=\n%s", out)
		}
		if !strings.Contains(out, "--- a/b/B.go") || !strings.Contains(out, "+++ b/b/B.go") {
			t.Errorf("B ---/+++ headers NOT preserved; out=\n%s", out)
		}
		// The legacy N-bearing sentinel must NOT appear.
		if strings.Contains(out, "[diff truncated at") {
			t.Errorf("legacy N-bearing sentinel leaked; out=\n%s", out)
		}
	})

	t.Run("SharedBudget_md_and_nm_one_budget", func(t *testing.T) {
		// mdDiffs=[small md section]; nmDiff=[one LARGE nm section]. ONE shared budget ⇒ the LARGE nm
		// section is capped even though the md section is separate. Proves D3 (not two budgets).
		mdSmall := "diff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md\n@@ -1 +1 @@\n-old md\n+new md tiny tweak here\n"
		nmLarge := bodySection("src/big.go", 4000) // ~1000 tokens
		// bodyBudget = 50 (tokenLimit = 50 + margin). mdSmall (~6 tokens) whole; nmLarge (1000) capped.
		tokenLimit := tokenBudgetMargin + 50
		out := applyWaterFillGate([]string{mdSmall}, nmLarge, "", tokenLimit, 0)

		// The md section is small ⇒ WHOLE (its unique marker survives).
		if !strings.Contains(out, "new md tiny tweak here") {
			t.Errorf("md small section NOT whole (marker missing); out=\n%s", out)
		}
		// Exactly one sentinel (the nm large section capped).
		if c := strings.Count(out, "... [truncated]"); c != 1 {
			t.Errorf("sentinel count = %d, want 1 (the nm large section); out=\n%s", c, out)
		}
	})

	t.Run("Skeleton_subtracted_smaller_budget", func(t *testing.T) {
		// Two runs, same bodies; run1 skeleton="" run2 skeleton=<big>. run2's bodyBudget is smaller ⇒ a
		// body that was WHOLE in run1 is CAPPED in run2. Demonstrates EstimateTokens(skeleton) subtracted.
		bodyRunes := 800 // ~200 tokens
		section := bodySection("p.go", bodyRunes)
		// run1: tokenLimit large enough that bodyBudget ≥ 200 ⇒ no truncation.
		run1Limit := tokenBudgetMargin + 300 // bodyBudget = 300 > 200 ⇒ whole
		out1 := applyWaterFillGate(nil, section, "", run1Limit, 0)
		if strings.Count(out1, "... [truncated]") != 0 {
			t.Errorf("run1: expected no truncation (budget ≥ size), got sentinel; out=\n%s", out1)
		}
		// run2: SAME tokenLimit but a large skeleton eats the budget ⇒ bodyBudget < 0 → clamped 0 ⇒
		// budget-exhausted path ⇒ the body is truncated to a minBodyTokens sliver + sentinel (NOT passed
		// through whole — the contract "payload always fits" requires a best-effort truncation). The
		// DIFFERENCE from run1 proves the skeleton was subtracted: run1 had budget for the body (whole),
		// run2 does not (sliver + sentinel). Assert the section header survives and exactly one sentinel.
		bigSkeleton := strings.Repeat("s", 8000) // ~2000 tokens > run1Limit-margin ⇒ bodyBudget clamps to 0
		out2 := applyWaterFillGate(nil, section, bigSkeleton, run1Limit, 0)
		if !strings.Contains(out2, "diff --git a/p.go b/p.go") {
			t.Errorf("run2: section header missing (should survive truncation); out=\n%s", out2)
		}
		if c := strings.Count(out2, "... [truncated]"); c != 1 {
			t.Errorf("run2: expected the body CAPPED to a sliver under degenerate budget (sentinel=1), got %d; out=\n%s", c, out2)
		}
		if strings.Contains(out2, "[diff truncated at") {
			t.Errorf("run2: legacy sentinel leaked; out=\n%s", out2)
		}
		// Now a MIDDLING skeleton: enough to shrink the budget below 200 but above 0 ⇒ truncation.
		// skeleton ~100 tokens ⇒ bodyBudget = 300 − 100 = 200 ⇒ exactly at size ⇒ NOT > ⇒ verbatim.
		// Pick skeleton ~150 tokens (600 runes) ⇒ bodyBudget = 150 < 200 ⇒ truncated.
		midSkeleton := strings.Repeat("s", 600) // ~150 tokens
		// Need bodyBudget between 0 and 200: raise tokenLimit so bodyBudget = 300 − 150(skel) = 150.
		// tokenLimit = 150(skel-budget) + 150(skel) + margin = 300 + margin.
		out3 := applyWaterFillGate(nil, section, midSkeleton, tokenBudgetMargin+300, 0)
		if c := strings.Count(out3, "... [truncated]"); c != 1 {
			t.Errorf("run3: expected the body CAPPED now that the skeleton ate budget (sentinel=1), got %d; out=\n%s", c, out3)
		}
	})

	t.Run("PromptReserve_subtracted_smaller_budget", func(t *testing.T) {
		// Same idea with promptReserve. run1 reserve=0 ⇒ whole; run2 reserve=large ⇒ capped.
		bodyRunes := 800 // ~200 tokens
		section := bodySection("p.go", bodyRunes)
		tokenLimit := tokenBudgetMargin + 250 // bodyBudget = 250 (reserve 0) > 200 ⇒ whole
		out1 := applyWaterFillGate(nil, section, "", tokenLimit, 0)
		if strings.Count(out1, "... [truncated]") != 0 {
			t.Errorf("reserve=0: expected no truncation, got sentinel; out=\n%s", out1)
		}
		// reserve=large (300) ⇒ bodyBudget = 250 − 300 = −50 → clamped 0 ⇒ budget-exhausted path ⇒ the body
		// is truncated to a minBodyTokens sliver + sentinel (best-effort fit; NOT passed through whole).
		out2 := applyWaterFillGate(nil, section, "", tokenLimit, 300)
		if !strings.Contains(out2, "diff --git a/p.go b/p.go") {
			t.Errorf("reserve=large: section header missing (should survive truncation); out=\n%s", out2)
		}
		if c := strings.Count(out2, "... [truncated]"); c != 1 {
			t.Errorf("reserve=large: expected CAPPED to a sliver under degenerate budget (sentinel=1), got %d; out=\n%s", c, out2)
		}
		// reserve=100 ⇒ bodyBudget = 250 − 100 = 150 < 200 ⇒ truncated.
		out3 := applyWaterFillGate(nil, section, "", tokenLimit, 100)
		if c := strings.Count(out3, "... [truncated]"); c != 1 {
			t.Errorf("reserve=100: expected CAPPED (sentinel=1), got %d; out=\n%s", c, out3)
		}
	})

	t.Run("BodyBudget_clamped_zero_truncates_to_sliver", func(t *testing.T) {
		// tokenLimit tiny (< skeleton+reserve+margin) ⇒ bodyBudget ≤ 0 ⇒ budget-exhausted path. The old
		// behavior passed the full body through verbatim (silent no-op), violating the documented "payload
		// always fits" contract. The fix truncates each over-budget body to a minBodyTokens sliver +
		// sentinel — a best-effort fit that is strictly smaller than the full diff and makes the truncation
		// VISIBLE. The section header is preserved; exactly one sentinel is emitted.
		section := bodySection("p.go", 4000)                        // ~1000 tokens
		out := applyWaterFillGate(nil, section, "skeleton", 100, 0) // bodyBudget = 100 − ~2 − margin < 0 → 0
		if c := strings.Count(out, "... [truncated]"); c != 1 {
			t.Errorf("degenerate budget≤0 must truncate to a sliver (sentinel=1), got %d; out=\n%s", c, out)
		}
		if !strings.Contains(out, "diff --git a/p.go b/p.go") {
			t.Errorf("degenerate budget: section header missing (should survive truncation); out=\n%s", out)
		}
		// The truncated body must be SMALLER than the full 4000-rune body: assert the sentinel appears and
		// the output length is well under the untruncated section length.
		if len(out) >= len(section) {
			t.Errorf("degenerate budget: output not smaller than the full body (len=%d, full=%d); out=\n%s", len(out), len(section), out)
		}
	})

	t.Run("PureRename_not_truncated", func(t *testing.T) {
		// A section with no @@ (rename only) ⇒ sectionBody "" ⇒ size 0 ⇒ never truncated even at tiny budget.
		rename := "diff --git a/old b/new\nsimilarity index 100%\nrename from old\nrename to new\n"
		// Tiny tokenLimit ⇒ bodyBudget 0 ⇒ all-0 allotments ⇒ pass-through for everything (incl. rename).
		out := applyWaterFillGate(nil, rename, "", tokenBudgetMargin, 0)
		if strings.Count(out, "... [truncated]") != 0 {
			t.Errorf("pure rename must NOT be truncated; got sentinel; out=\n%s", out)
		}
		if !strings.Contains(out, rename) {
			t.Errorf("pure rename not verbatim; out=\n%s", out)
		}
	})

	t.Run("PathKeying_via_diffSectionPath", func(t *testing.T) {
		// A normal section whose path resolves via diffSectionPath is keyed into the allotments map ⇒
		// truncateByWaterFill finds it ⇒ truncates when over budget. (Confirms the map is keyed by the
		// SAME path truncateByWaterFill looks up — D11.)
		large := bodySection("src/big.go", 4000) // ~1000 tokens
		tokenLimit := tokenBudgetMargin + 50     // bodyBudget 50 < 1000 ⇒ capped
		out := applyWaterFillGate(nil, large, "", tokenLimit, 0)
		if c := strings.Count(out, "... [truncated]"); c != 1 {
			t.Errorf("path-keyed section should be capped (sentinel=1), got %d; out=\n%s", c, out)
		}
	})

	t.Run("SmallTokenLimit_truncates_not_noop", func(t *testing.T) {
		// REGRESSION (report Bug 2): a small-but-nonzero token_limit (e.g. 100, 500, 1000) used to be
		// silently a no-op — the full untruncated payload was sent because bodyBudget clamped to 0 and the
		// degenerate path passed bodies through verbatim. The documented contract ("payload always fits
		// your context window") was violated: a smaller token_limit yielded a LARGER (untruncated) payload.
		// The fix truncates each over-budget body to a minBodyTokens sliver + sentinel regardless of how
		// small the (positive) token_limit is. For each small token_limit, assert (a) the section header
		// survives, (b) exactly one sentinel, and (c) the output is strictly smaller than the full body.
		large := bodySection("src/big.go", 4000) // ~1000 tokens; full section ~4100 bytes
		for _, tokenLimit := range []int{1, 100, 500, 1000, 1024} {
			tokenLimit := tokenLimit
			t.Run(fmt.Sprintf("tokenLimit=%d", tokenLimit), func(t *testing.T) {
				out := applyWaterFillGate(nil, large, "", tokenLimit, 0)
				if !strings.Contains(out, "diff --git a/src/big.go b/src/big.go") {
					t.Errorf("section header missing (should survive truncation); out=\n%s", out)
				}
				if c := strings.Count(out, "... [truncated]"); c != 1 {
					t.Errorf("small token_limit must STILL truncate (sentinel=1), got %d; out=\n%s", c, out)
				}
				if len(out) >= len(large) {
					t.Errorf("output not smaller than the full body (len=%d, full=%d); out=\n%s", len(out), len(large), out)
				}
			})
		}
	})
}
