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

// closedLoopGate pure unit tests (FR3j). The gate under test is PURE (no git, no ctx, no I/O): it wraps
// applyWaterFillGate with a measure→reduce→rerun loop. The tests use SYNTHETIC measure callbacks
// (lambdas) to model the real "measure = EstimateTokens(sysPrompt + payload)" behavior without importing
// internal/prompt: a closure that adds a fixed `prefixOverhead` (the simulated sysPrompt + framing)
// to EstimateTokens(gatedDiff). This models the real assembled-prompt measurement faithfully enough to
// exercise the LOOP LOGIC (convergence, nil-safety, within-budget no-op, overshoot correction, maxPasses
// bound) — the accuracy of the real estimator is S3's e2e invariant test's job.

// TestClosedLoopGate_NilMeasure_DelegatesToFirstCut verifies the nil-safe seam: when measure == nil,
// closedLoopGate returns the EXACT same output as applyWaterFillGate (no loop, no re-trim). This is the
// "behavior unchanged when the seam is not used" guarantee that S2's consumers rely on — a consumer that
// does not wire the closure gets the first-cut, byte-identical to today.
func TestClosedLoopGate_NilMeasure_DelegatesToFirstCut(t *testing.T) {
	section := bodySection("src/big.go", 4000) // a sizeable body so the gate actually trims
	firstCut := applyWaterFillGate(nil, section, "", tokenBudgetMargin+100, 0)
	got := closedLoopGate(nil, section, "", tokenBudgetMargin+100, 0, nil)
	if got != firstCut {
		t.Errorf("nil measure must delegate to first-cut byte-identically;\nfirstCut=\n%s\ngot=\n%s", firstCut, got)
	}
}

// TestClosedLoopGate_WithinBudget_NoRetrim verifies the invariant holds when the first-cut already fits:
// measure(gatedDiff) ≤ tokenLimit ⇒ the first-cut is returned unchanged (no re-trim, no loop body runs).
// The measure adds a fixed prefixOverhead; tokenLimit is set large enough that prefixOverhead + body fits.
func TestClosedLoopGate_WithinBudget_NoRetrim(t *testing.T) {
	section := bodySection("src/small.go", 40) // ~10 tokens of body — well within budget
	prefixOverhead := 100                      // simulated sysPrompt + framing
	tokenLimit := tokenBudgetMargin + 1000     // bodyBudget ~1000 ≫ body+prefix; first-cut is the full body
	firstCut := applyWaterFillGate(nil, section, "", tokenLimit, 0)
	measure := func(gatedDiff string) int {
		return prefixOverhead + EstimateTokens(gatedDiff)
	}
	got := closedLoopGate(nil, section, "", tokenLimit, 0, measure)
	if got != firstCut {
		t.Errorf("within-budget must return the first-cut unchanged;\nfirstCut=\n%s\ngot=\n%s", firstCut, got)
	}
	if measure(got) > tokenLimit {
		t.Errorf("invariant violated: measure(got)=%d > tokenLimit=%d", measure(got), tokenLimit)
	}
}

// TestClosedLoopGate_OverBudget_RetrimmedToFit verifies the closed loop converges: when the first-cut's
// assembled prompt EXCEEDS tokenLimit, the loop reduces the effective limit and re-runs the gate until
// the trimmed result measures ≤ tokenLimit. Asserts (a) the result measures within budget and (b) the
// result is SHORTER than the first-cut (the re-trim actually trimmed).
func TestClosedLoopGate_OverBudget_RetrimmedToFit(t *testing.T) {
	// A LARGE body (~1020 tokens). prefixOverhead models the stable prompt portion the first-cut did not
	// account for densely — so the assembled prompt (prefix + body) overruns tokenLimit on the first cut.
	// tokenLimit is set so the first-cut's bodyBudget is generous (the body comes back whole) but the
	// assembled measure (prefix + body) overruns. After the loop reduces the effective limit, the body is
	// trimmed until prefix + trimmedBody ≤ tokenLimit.
	//
	// Empirically verified convergence (the loop reaches the invariant in ≤ maxClosedLoopPasses):
	//   section ≈ 1020 tokens; first-cut whole ⇒ measure(fc) = 1200 + 1022 = 2222 > 2024 (over);
	//   after re-trim measure(got) = 1960 ≤ 2024 (fits); len(got)=3040 < len(fc)=4088 (shorter).
	section := bodySection("src/big.go", 4000) // ~1020 tokens of body
	prefixOverhead := 1200                     // simulated sysPrompt + framing
	tokenLimit := tokenBudgetMargin + 1000     // bodyBudget = 1000 < 1020 ⇒ ... but margin: 2024−1024=1000, body ~1020 ⇒ near-whole; the first-cut returns the body whole (water-fill at the boundary)
	firstCut := applyWaterFillGate(nil, section, "", tokenLimit, 0)
	measure := func(gatedDiff string) int {
		return prefixOverhead + EstimateTokens(gatedDiff)
	}
	// Sanity: the premise holds — the first-cut is genuinely over budget (else the loop is a no-op).
	if fc := measure(firstCut); fc <= tokenLimit {
		t.Fatalf("test premise broken: first-cut already fits (measure(fc)=%d ≤ tokenLimit=%d); pick larger prefixOverhead or smaller tokenLimit", fc, tokenLimit)
	}
	got := closedLoopGate(nil, section, "", tokenLimit, 0, measure)
	if m := measure(got); m > tokenLimit {
		t.Errorf("invariant violated after re-trim: measure(got)=%d > tokenLimit=%d;\ngot=\n%s", m, tokenLimit, got)
	}
	if len(got) >= len(firstCut) {
		t.Errorf("re-trim should produce a SHORTER result; len(got)=%d >= len(firstCut)=%d;\nfirstCut=\n%s\ngot=\n%s", len(got), len(firstCut), firstCut, got)
	}
}

// TestClosedLoopGate_MaxPassesBound verifies the loop terminates (no infinite loop) against an
// adversarial estimator that ALWAYS reports over budget, regardless of how small the gated diff gets.
// The loop must run at most maxClosedLoopPasses iterations and return the best attempt seen. We assert
// the call returns (the test would hang on a regression) and that the result is at least as small as
// the first-cut (the loop never makes things worse — it tracks the best attempt).
func TestClosedLoopGate_MaxPassesBound(t *testing.T) {
	section := bodySection("src/big.go", 4000)
	tokenLimit := tokenBudgetMargin + 200
	firstCut := applyWaterFillGate(nil, section, "", tokenLimit, 0)
	// Adversarial: ALWAYS reports tokenLimit + 1000, no matter the input.
	measure := func(gatedDiff string) int {
		return tokenLimit + 1000
	}
	// If the loop were unbounded this would hang the test runner; the bounded loop returns.
	got := closedLoopGate(nil, section, "", tokenLimit, 0, measure)
	// The best attempt is tracked: with a constant adversarial measure, the "best" is the smallest gated
	// diff, which closedLoopGate produces by reducing the effective limit each pass. The returned diff is
	// never LARGER than the first-cut (the first pass's effective limit is below tokenLimit).
	if len(got) > len(firstCut) {
		t.Errorf("adversarial measure: result must not exceed the first-cut; len(got)=%d > len(firstCut)=%d;\nfirstCut=\n%s\ngot=\n%s", len(got), len(firstCut), firstCut, got)
	}
}

// TestClosedLoopGate_AdversarialDrift_GrowingMeasure (FR3j contract (b) — the "grows with each trim"
// drift variant): an adversarial measure that GROWS each call models estimation drift where re-measurement
// reports LARGER on each pass (the worst case the bounded loop must survive). Trimming can never satisfy
// the invariant (each re-measure is worse than the last), so the loop must TERMINATE at the maxClosedLoopPasses
// bound and return the best attempt — the FIRST-CUT (later trims are strictly worse, so bestDiff is never
// updated). Distinct from TestClosedLoopGate_MaxPassesBound (constant adversarial): there the measure is
// invariant to trimming; here trimming makes it strictly worse. Also the FIRST test to assert the
// maxClosedLoopPasses call-count bound (via a counting measure).
func TestClosedLoopGate_AdversarialDrift_GrowingMeasure(t *testing.T) {
	section := bodySection("src/big.go", 4000) // a sizeable body so the gate trims on re-runs
	tokenLimit := tokenBudgetMargin + 200
	firstCut := applyWaterFillGate(nil, section, "", tokenLimit, 0)

	// Stateful adversarial drift: GROWS each call (always > tokenLimit ⇒ invariant can never hold) AND
	// counts calls (to assert the pass bound). The closedLoopGate measure-call pattern is deterministic
	// (1 first-cut call + ≤ maxClosedLoopPasses pass calls), so `calls` is a faithful pass counter.
	var calls int
	measure := func(gatedDiff string) int {
		calls++
		return tokenLimit + 100 + calls*50 // call#1: +150; #2: +200; #3: +250; #4: +300; #5: +350 (all > tokenLimit)
	}

	got := closedLoopGate(nil, section, "", tokenLimit, 0, measure)

	// (1) TERMINATES: the call returned (implicit — a regression to an unbounded loop hangs the runner).
	// (2) BOUNDED: measure is called once for the first-cut + ≤ maxClosedLoopPasses passes ⇒ calls ≤ maxClosedLoopPasses+1.
	if calls > maxClosedLoopPasses+1 {
		t.Errorf("loop exceeded maxClosedLoopPasses: measure called %d times, want ≤ %d (1 first-cut + maxClosedLoopPasses passes)", calls, maxClosedLoopPasses+1)
	}
	// (3) BEST ATTEMPT: with a growing measure, trimming worsens the measured value, so bestDiff is never
	// updated and the loop returns the first-cut (the smallest measured value seen) byte-identical.
	if got != firstCut {
		t.Errorf("growing-drift: best attempt must be the first-cut (trimming worsens the measure, so bestDiff is never updated);\nfirstCut=\n%s\ngot=\n%s", firstCut, got)
	}
}

// TestClosedLoopGate_EffectiveLimitFloor verifies the effectiveLimit < 1 clamp: a pathological overshoot
// (measure reports vastly over tokenLimit) drives effectiveLimit negative, which MUST be clamped to 1 so
// applyWaterFillGate's bodyBudget ≤ 0 / minBodyTokens path fires (a strictly-positive limit is required —
// effectiveLimit ≤ 0 would still compute bodyBudget ≤ 0 the same way, but the clamp documents the floor
// and guards against a future negative-limit surprise). The loop must still terminate and return a
// best-effort result that is SMALLER than the untrimmed first-cut.
func TestClosedLoopGate_EffectiveLimitFloor(t *testing.T) {
	section := bodySection("src/big.go", 4000)
	// tokenLimit tiny + prefixOverhead huge ⇒ overshoot ≫ tokenLimit ⇒ effectiveLimit goes negative and is
	// clamped to 1. The measure still reports over (the prefix alone exceeds tokenLimit), so the loop runs
	// to maxClosedLoopPasses and returns the best (smallest) attempt — a minBodyTokens sliver.
	tokenLimit := tokenBudgetMargin + 50
	prefixOverhead := 100000
	firstCut := applyWaterFillGate(nil, section, "", tokenLimit, 0)
	measure := func(gatedDiff string) int {
		return prefixOverhead + EstimateTokens(gatedDiff)
	}
	got := closedLoopGate(nil, section, "", tokenLimit, 0, measure)
	// The loop converged to the smallest attempt (the clamped-to-1 effectiveLimit trims hardest).
	if len(got) >= len(firstCut) {
		t.Errorf("effectiveLimit floor: result must be SMALLER than the first-cut; len(got)=%d >= len(firstCut)=%d;\nfirstCut=\n%s\ngot=\n%s", len(got), len(firstCut), firstCut, got)
	}
	// Sanity: the best-effort result still has the section header (truncation preserves headers).
	if !strings.Contains(got, "diff --git a/src/big.go b/src/big.go") {
		t.Errorf("effectiveLimit floor: section header missing; out=\n%s", got)
	}
}
