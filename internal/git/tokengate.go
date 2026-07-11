// FR3d/FR3i token-limit GATE (PRD §9.1 FR3d/FR3i; architecture/system_context.md §5 the FR3i coupling
// seam + §6 the regression invariants).
//
// The gate chooses the truncation STRATEGY for the three sibling diff functions (StagedDiff/TreeDiff/
// WorkingTreeDiff in git.go) based on opts.TokenLimit:
//
//   - ==0 (unset): the EXISTING legacy per-section caps (per-file markdown line-cap `max_md_lines` +
//     non-markdown aggregate byte-cap `max_diff_bytes` + their `... [diff truncated at N bytes/lines]`
//     sentinels) apply UNCHANGED — byte-identical to pre-M4 (system_context §6 invariant 1 — the
//     regression anchor; the FR3e/-M, FR3f/-U<n>, FR3g skeleton-prepend, FR3h index-strip transforms from
//     M2/M3 still apply around them).
//   - >0  (set):   a dynamic water-fill REPLACES both caps (FR3d: "a non-zero token_limit supersedes both
//     legacy caps"). body_budget = max(0, token_limit − EstimateTokens(skeleton) − promptReserve − margin);
//     each file's body is sized with EstimateTokens; allocByWaterFill allots the budget; truncateByWaterFill
//     applies the per-file level (emitting the shorter `... [truncated]` sentinel per truncated file —
//     system_context §6 invariant 2). The FR3g numstat skeleton is prepended in BOTH branches (it runs at
//     capture, upstream of the gate; the gate RECEIVES the skeleton string ONLY to size it — it is not
//     re-emitted by the gate).
//
// This file holds the PURE helper that implements the >0 branch's budget arithmetic + assembly. It is a
// PURE string/budget-arithmetic function — no git, no ctx, no I/O — so it is exhaustively unit-testable
// without a repo (tokengate_test.go mirrors truncatediff_test.go's pure table-driven style). The git.go
// >0 branches are then trivial wiring: capture uncapped text → delegate to applyWaterFillGate.
//
// COHERENCE (design-decisions D2/D8): sizing and enforcement BOTH use EstimateTokens over the SAME body.
// sectionBody splits a section at its first `@@` via the SIBLING's `atAtRe` — the EXACT same split
// truncateByWaterFill uses to cut the body. So the water-fill's "file > L" condition (on sizes) and
// truncateByWaterFill's "EstimateTokens(body) > allotment" enforcement AGREE exactly ⇒ the FR3i fairness
// guarantees (a–d: every file represented, small files whole, large files capped at a shared water level,
// headers preserved) hold. numstatRows is the dual-use SKELETON (FR3g) + path identity — the body token
// estimate uses the captured body, NOT numstat line counts.
//
// Composition (no new domain logic): sizing = EstimateTokens (tokens.go, in-package); allocation =
// allocByWaterFill (waterfill.go, the consumer-facing allocator — waterFillLevel is the solver's internal
// detail); application = truncateByWaterFill (truncatediff.go, which emits the `... [truncated]` sentinel
// and preserves headers). The gate only computes the budget + assembles the inputs.

package git

// tokenBudgetMargin is the FR3d/FR3i safety buffer subtracted from body_budget (PRD §9.1 FR3i:
// body_budget = token_limit − skeleton − prompt − margin). It absorbs (a) the chars/4 vs actual-
// tokenization density gap (the estimator is conservative; code is ~3-4 chars/token, prose ~4-5), (b)
// the `diff --git`/`---`/`+++` header blocks truncateByWaterFill PRESERVES but that are NOT counted in
// body sizing (they sit before the first `@@`), and (c) the `[binary]`/`[excluded]` placeholders. The
// user's token_limit already carries implicit slack (set below the hard context window); this is the
// deterministic floor. Tunable — raise it for noisier repos; lower it to spend more of the budget on
// bodies.
const tokenBudgetMargin = 1024

// sectionBody returns the BODY of a diff section: the substring from the first hunk-header line (`@@`,
// detected via the sibling's atAtRe — the SAME regex truncateByWaterFill splits on) onward. A section
// with no `@@` (pure rename / mode-only) has an empty body. PURE.
//
// Used by applyWaterFillGate to SIZE each file's body with EstimateTokens — using the SAME body split as
// the enforcement ⇒ the water-fill's "file > L" condition (on sizes) and truncateByWaterFill's
// "EstimateTokens(body) > allotment" AGREE exactly (coherence; design-decisions D2/D8). Do NOT invent a
// different body-split.
func sectionBody(section string) string {
	loc := atAtRe.FindStringIndex(section)
	if loc == nil {
		return "" // no hunk → no body → size 0 → never truncated (matches truncateByWaterFill's pass-through)
	}
	return section[loc[0]:]
}

// minBodyTokens is the per-file body-token floor applied when the token budget is exhausted
// (bodyBudget ≤ 0 but tokenLimit > 0). Rather than passing over-budget bodies through VERBATIM (the old
// D10 behavior — which silently sent the full untruncated payload, violating the documented "payload
// always fits your context window" contract for small token_limit values), each file's body is cut to a
// minimal sliver of this many tokens (≈ minBodyTokens×4 runes) + the `... [truncated]` sentinel. This is
// a BEST-EFFORT fit: when token_limit is below the effective floor (skeleton + prompt reserve + margin),
// NOTHING can truly fit, but truncating to a sliver is strictly better than sending the full diff — a
// smaller payload overflows the window by less, and the sentinel makes the truncation VISIBLE to the
// model (vs. the silent no-op). Tunable; kept small so the aggregate stays minimal.
const minBodyTokens = 8

// maxClosedLoopPasses is the FR3j closed-loop bound: the maximum number of re-trim passes
// closedLoopGate will perform after the first-cut. The water-fill estimate is already close (the
// promptReserve + tokenBudgetMargin subtract a conservative upper bound), so convergence typically takes
// 1–2 passes; 4 is a safety ceiling against a hostile estimator. Beyond it the loop returns the best
// attempt seen so far (best-effort — an adversarial estimator can always lie; the bound prevents an
// infinite loop).
const maxClosedLoopPasses = 4

// closedLoopSlack is the FR3j extra tokens shaved per overshoot when closedLoopGate reduces the
// effective limit. Without it the second pass could land exactly at the boundary and oscillate (measure
// says 1 over → trim 1 → measure says 1 over again, burning a pass each time). The slack ensures each
// pass makes strictly more progress than the raw overshoot, so the loop converges instead of hovering.
const closedLoopSlack = 64

// IrreducibleFloor returns the minimum token_limit below which the assembled prompt cannot possibly fit,
// so the FR3j closed-loop "payload never exceeds token_limit" invariant cannot be honored (Issue 4). It is
// the non-body minimum: the numstat skeleton (EstimateTokens) + the system-prompt reserve (promptReserve,
// measured upstream) + the tokenBudgetMargin framing buffer. Diff BODIES are truncatable (down to
// minBodyTokens slivers); these three terms are not — they are the floor. The three git.go diff entry
// points (StagedDiff/TreeDiff/WorkingTreeDiff) reject token_limit < IrreducibleFloor(...) with a clear
// error instead of silently returning an over-budget best-effort payload from closedLoopGate. PURE
// (mirrors applyWaterFillGate): no git/ctx/I/O — it composes only EstimateTokens (tokens.go, in-package)
// and the unexported tokenBudgetMargin const.
func IrreducibleFloor(skeleton string, promptReserve int) int {
	return EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin
}

// applyWaterFillGate is the FR3d/FR3i token-limit gate (PRD §9.1 FR3d/FR3i; system_context.md §5/§6). It
// replaces the legacy max_md_lines/max_diff_bytes caps with a dynamic water-fill over ALL diff bodies
// (markdown + non-markdown) sharing ONE body_budget (FR3i: "across files" — one shared budget, NOT two
// separate md/non-md budgets; design-decisions D3). PURE: no git, no ctx, no I/O — it composes only
// splitDiffSections, EstimateTokens, allocByWaterFill, truncateByWaterFill, diffSectionPath, sectionBody
// (all pure / in-package).
//
// Inputs:
//   - mdDiffs:      the per-file markdown diffs, each a self-contained `diff --git` section, already
//     captured UNCAPPED + FR3h-index-stripped + -M/-U<n>-shaped upstream (the >0 branch in git.go
//     appends stripIndexLines(fileDiff) without the legacy line-cap).
//   - nmDiff:       the non-markdown aggregate, captured UNCAPPED + index-stripped (the >0 branch skips
//     the legacy byte-cap).
//   - skeleton:     the already-prepended FR3g numstat skeleton string. USED ONLY TO SIZE — NOT re-emitted
//     (the skeleton is written to the builder BEFORE the gate runs; the gate receives the string so it can
//     subtract EstimateTokens(skeleton) from the budget).
//   - tokenLimit:   opts.TokenLimit (the caller has already branched on >0; this is the resolved value).
//   - promptReserve: opts.PromptReserveTokens (measured upstream, P1.M4.T1.S2; the stable prompt-portion
//     cost so body_budget = token_limit − skeleton − promptReserve).
//
// Algorithm:
//  1. skeletonTokens := EstimateTokens(skeleton).
//  2. bodyBudget := max(0, tokenLimit − skeletonTokens − promptReserve − tokenBudgetMargin).
//  3. sections = mdDiffs + splitDiffSections(nmDiff)  (ALL files, one shared budget — FR3i "across files").
//  4. sizes[i] = EstimateTokens(sectionBody(sections[i]))  (body tokens; the SAME body the enforcement cuts).
//  5. allocs := allocByWaterFill(sizes, bodyBudget)  (parallel to sections; preserves input order).
//  6. allotments[path] = allocs[i]  (keyed by diffSectionPath — the SAME key truncateByWaterFill looks up).
//  7. return truncateByWaterFill(sections, allotments)  (recomposes in input order; emits `... [truncated]`
//     per over-budget file; within-budget files byte-identical; the `at N bytes/lines` sentinels NEVER
//     appear — §6 invariant 2).
//
// Coherence: sizing + enforcement both use EstimateTokens over the SAME body (sectionBody via atAtRe ≡
// truncateByWaterFill's split) ⇒ the water-fill's fairness guarantees (FR3i a–d) are EXACT.
//
// Degenerate bodyBudget≤0 (token_limit too small for skeleton+reserve+margin): the contract "payload
// always fits" still requires a best-effort truncation. Each over-budget file's body is cut to a
// minBodyTokens sliver + sentinel (NOT passed through whole — the old D10 behavior silently sent the
// full untruncated payload, violating the documented contract for small token_limit values). A
// minBodyTokens floor is used because truncateByWaterFill treats allotment≤0 as path-miss (pass-through),
// so a strictly-positive floor is required to force truncation. This is a BEST-EFFORT fit: when
// token_limit is below the effective floor nothing can truly fit, but a sliver is strictly smaller than
// the full diff and the sentinel makes the truncation VISIBLE.
//
// Called by StagedDiff/TreeDiff/WorkingTreeDiff in their opts.TokenLimit>0 branch.
func applyWaterFillGate(mdDiffs []string, nmDiff, skeleton string, tokenLimit, promptReserve int) string {
	skeletonTokens := EstimateTokens(skeleton)
	bodyBudget := tokenLimit - skeletonTokens - promptReserve - tokenBudgetMargin
	budgetExhausted := bodyBudget <= 0 // token_limit below the effective floor; force minimal truncation.
	if bodyBudget < 0 {
		bodyBudget = 0
	}

	nmSections := splitDiffSections(nmDiff)
	sections := make([]string, 0, len(mdDiffs)+len(nmSections))
	sections = append(sections, mdDiffs...)
	sections = append(sections, nmSections...)
	if len(sections) == 0 {
		return ""
	}

	sizes := make([]int, len(sections))
	for i, sec := range sections {
		sizes[i] = EstimateTokens(sectionBody(sec))
	}
	allocs := allocByWaterFill(sizes, bodyBudget)

	allotments := make(map[string]int, len(sections))
	for i, sec := range sections {
		if path, ok := diffSectionPath(sec); ok {
			allot := allocs[i]
			// Budget-exhausted path: a 0 allotment would make truncateByWaterFill treat the file as a
			// path-miss and pass the FULL body through verbatim. Force a minimal strictly-positive floor
			// so the body is truncated to a sliver + sentinel (best-effort fit; visible truncation).
			if budgetExhausted && allot <= 0 {
				allot = minBodyTokens
			}
			allotments[path] = allot // keyed by destination — matches truncateByWaterFill's lookup (D11)
		}
	}
	return truncateByWaterFill(sections, allotments)
}

// closedLoopGate is the FR3j closed-loop budget guarantee (PRD §9.1 FR3j; architecture/
// fr3j_closed_loop.md). It calls applyWaterFillGate for the first-cut, measures the ASSEMBLED prompt
// via the injected measure callback, and if over tokenLimit, reduces the effective limit by the
// overshoot + slack and re-runs the gate. Bounded at maxClosedLoopPasses. When measure == nil, it
// delegates to applyWaterFillGate with NO loop (behavior unchanged — the nil-safe seam that lets a
// consumer who does not wire the closure get the first-cut, exactly as today).
//
// PURE: no git, no ctx, no I/O — it composes only applyWaterFillGate + the injected measure callback
// (and the input strings). The measure callback captures sysPrompt + the role-specific payload
// builder (injected by the consumer in S2) so internal/git can measure the WHOLE assembled prompt
// without importing internal/prompt (the leaf-purity invariant — mirrors internal/prompt/reserve.go's
// TokenEstimator injection in reverse).
//
// Invariant: measure(gatedDiff) ≤ tokenLimit, where gatedDiff is the returned string. The loop is
// best-effort — an adversarial estimator that always reports over cannot be satisfied, but the
// maxClosedLoopPasses bound guarantees termination and the best attempt (smallest measured value
// seen) is returned on exit. The slack (closedLoopSlack) is subtracted alongside the overshoot to
// prevent boundary oscillation (measure says 1 over → trim 1 → measure says 1 over again); it ensures
// each pass makes strictly more progress than the raw overshoot so the loop converges.
//
// Called by StagedDiff/TreeDiff/WorkingTreeDiff in their opts.TokenLimit>0 branch (S2 wiring) when
// opts.MeasureAssembled is non-nil.
func closedLoopGate(mdDiffs []string, nmDiff, skeleton string, tokenLimit, promptReserve int,
	measure func(string) int) string {
	if measure == nil {
		return applyWaterFillGate(mdDiffs, nmDiff, skeleton, tokenLimit, promptReserve)
	}

	gatedDiff := applyWaterFillGate(mdDiffs, nmDiff, skeleton, tokenLimit, promptReserve)
	bestDiff := gatedDiff
	bestMeasured := measure(gatedDiff)

	for pass := 0; pass < maxClosedLoopPasses; pass++ {
		if bestMeasured <= tokenLimit {
			return bestDiff // invariant holds — the assembled prompt fits within tokenLimit
		}
		overshoot := bestMeasured - tokenLimit
		effectiveLimit := tokenLimit - overshoot - closedLoopSlack
		if effectiveLimit < 1 {
			effectiveLimit = 1 // floor: never below 1 (applyWaterFillGate's minBodyTokens path handles tiny limits)
		}
		gatedDiff = applyWaterFillGate(mdDiffs, nmDiff, skeleton, effectiveLimit, promptReserve)
		measured := measure(gatedDiff)
		if measured < bestMeasured {
			bestDiff = gatedDiff
			bestMeasured = measured
		}
		if measured <= tokenLimit {
			return gatedDiff // invariant holds after this pass
		}
	}
	return bestDiff // best effort after maxClosedLoopPasses (an adversarial estimator can always lie)
}
