// FR3d/FR3i model-agnostic token estimator (PRD §9.1 FR3d/FR3i; architecture/git_diff_semantics.md §5).
//
// Stagehand never loads a tokenizer (it shells out to an arbitrary agent CLI), so it estimates tokens with
// the standard "~4 chars ≈ 1 token" heuristic: ceil(runeCount / 4), rounded UP. Rune-based (not byte-based)
// so multi-byte UTF-8 (CJK, emoji) does not over-count. This is the SINGLE estimator used by BOTH the
// prompt-reserve measurement (P1.M4.T1.S2) and the FR3i water-fill sizing/truncation (P1.M4.T2) — they call
// the same function so the budget arithmetic (body_budget = token_limit − skeleton − prompt − margin) is
// measured in consistent units. The FR3d/FR3i safety `margin` (applied in M4.T2) absorbs the
// code-vs-prose density gap; this estimator is the consistent MEASURE, not the safety MECHANISM.
//
// The contract pins chars/4 (NOT the architecture doc's chars/3 ceiling-recommendation); the chars/3 gap is
// reconciled by the separate `margin`, not by changing this formula. See P1.M4.T1.S1/research/design-decisions.md §2.

package git

import "unicode/utf8"

// EstimateTokens returns a model-agnostic token-count estimate for s using the standard "~4 chars ≈ 1 token"
// heuristic (PRD §9.1 FR3d; architecture/git_diff_semantics.md §5): ceil(runeCount / 4), rounded UP. Rune-based
// (utf8.RuneCountInString, NOT byte-based len(s)) so a 4-rune/12-byte CJK string estimates as 1 token, not 3.
//
// This is the SINGLE estimator consumed by the prompt-reserve measurement (P1.M4.T1.S2) and the FR3i
// water-fill sizing/truncation (P1.M4.T2). The FR3d/FR3i safety `margin` (applied in M4.T2) is the safety
// buffer; this function is the consistent measure, not the safety mechanism — do not "improve" it to chars/3.
func EstimateTokens(s string) int {
	return ceilDiv(utf8.RuneCountInString(s), 4)
}

// EstimateTokensBytes is the []byte form of EstimateTokens (same ceil(runes/4) formula). Convenience for
// callers that hold a []byte (e.g. a diff body buffer); allocation-free via utf8.RuneCount. See EstimateTokens
// for the heuristic and the single-estimator rationale.
func EstimateTokensBytes(b []byte) int {
	return ceilDiv(utf8.RuneCount(b), 4)
}

// ceilDiv returns the ceiling of n/d for n≥0, d>0: (n+d-1)/d. Yields 0 for n=0 (no special-case) and rounds
// UP for n>0 (e.g. ceilDiv(5,4)=2). The ceiling is what makes "any non-empty string ≥ 1 token" hold.
func ceilDiv(n, d int) int { return (n + d - 1) / d }
