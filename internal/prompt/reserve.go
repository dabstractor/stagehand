// Package prompt: FR3d/FR3i prompt-reserve helpers (P1.M4.T1.S2).
//
// This file closes the FR3i coupling seam (PRD §9.1 FR3d/FR3i; architecture/system_context.md §5): each
// diff call site builds its system prompt BEFORE the diff call, measures the WORST-CASE token count of the
// stable prompt portion (system prompt + worst-case user-payload overhead + a safety margin), and threads
// the result into git.StagedDiffOptions.PromptReserveTokens. M4.T3's gate reads it; M4.T2's water-fill
// subtracts it via body_budget = token_limit − skeleton − reserve − margin.
//
// LEAF-PURE via an INJECTED estimator (research design-decisions.md §0): the prompt package has zero
// internal dependencies by stated design (planner.go). The helper needs BOTH the same-package builders
// (BuildUserPayload / BuildPlannerUserPayload / BuildArbiterUserPayload) AND git.EstimateTokens (S1's
// SINGLE chars/4 estimator). Rather than importing internal/git (which would compile — it's acyclic —
// but erode the leaf-pure invariant), the estimator is INJECTED as the `est TokenEstimator` parameter.
// Every call site passes git.EstimateTokens, so a "token" upstream (this reserve) == a "token" downstream
// (M4.T2's water-fill sizing) — no second formula, no drift (S1's single-estimator rule).
//
// The empty-diff trick (design-decisions.md §2): the three payload builders append the diff as the
// VERBATIM tail, so calling each with an EMPTY diff ("") yields EXACTLY the non-diff overhead. This is
// the SINGLE SOURCE OF TRUTH for the prompt topology — no hand-rebuilt constants that could drift from
// payload.go / planner.go / arbiter.go if §17.3 / §17.5 / §17.7 wording changes.

package prompt

import "strings"

// TokenEstimator estimates the token count of a string. The canonical implementation is
// git.EstimateTokens (ceil(runes/4), P1.M4.T1.S1) — the SINGLE model-agnostic estimator. It is INJECTED
// (not imported) so the prompt package stays dependency-free (its stated design value); every call site
// passes git.EstimateTokens. S1's "single formula" rule is enforced by this seam — do not introduce a
// second estimator or a chars/3 variant (it would make upstream/downstream budget arithmetic incoherent).
type TokenEstimator func(s string) int

// reserveSafetyMargin is the FR3d/FR3i prompt-reserve safety buffer (in tokens). It inflates the prompt
// estimate to a worst-case upper bound so body_budget = token_limit − skeleton − reserve − margin (FR3i,
// M4.T2) never under-reserves the prompt. It absorbs: over-length rejected subjects (the worst-case block
// uses SubjectTargetChars; real subjects can exceed it), the FR29 retryInstruction preamble (~15 tokens,
// prepended on parse-fail retries and not separately enumerated in the message path), the chars/4-vs-chars/3
// code-density gap, and any small per-attempt overhead the worst-case block does not explicitly enumerate.
//
// DISTINCT from FR3i's body_budget `margin` (M4.T2's separate term): the reserve (including this 256)
// IS the `prompt` term in body_budget = token_limit − skeleton − prompt − margin. Do NOT conflate them.
const reserveSafetyMargin = 256

// measureReserve is the shared core: est(sysPrompt) + est(overhead) + reserveSafetyMargin. The three
// role-specific helpers build their worst-case `overhead` string (the non-diff user-payload portion) and
// delegate here. `overhead` is the user payload MINUS the diff (the diff is the variable part the
// FR3i water-fill allocates across files; the reserve bounds everything ELSE so it is excluded).
func measureReserve(sysPrompt, overhead string, est TokenEstimator) int {
	return est(sysPrompt) + est(overhead) + reserveSafetyMargin
}

// MessageReserveTokens computes the worst-case prompt reserve for the MESSAGE role (FR3d/FR3i). sysPrompt
// is the ALREADY-BUILT system prompt (BuildSystemPrompt / BuildFallbackPrompt output — header + style
// examples + format scaffold + locale line; at pkg/stagecoach it includes the appended SystemExtra). The
// message user payload's worst case is the REJECTION path (FR32): it grows per dedupe attempt up to
// maxDuplicateRetries rejected subjects. Because the diff is captured ONCE before the dedupe loop, the
// reserve is the WORST CASE (all maxDuplicateRetries slots filled), measured once — STABLE across
// attempts (a ceiling, not per-attempt). This stability is the contract's explicit test assertion.
//
// The overhead is built via BuildUserPayload with an EMPTY diff (the builder appends diff as the verbatim
// tail ⇒ empty diff isolates the exact non-diff overhead — the single source of truth) and a worst-case
// rejected slice (maxDuplicateRetries subjects at ~subjectTargetChars each). The normal path (no
// rejection) is smaller, so this is a safe upper bound for every attempt. reserveSafetyMargin absorbs
// over-length subjects + the FR29 retryInstruction preamble.
//
// Consumers: generate.CommitStaged, hook.Run, pkg/stagecoach.runPipeline, decompose.generateMessage.
func MessageReserveTokens(sysPrompt string, maxDuplicateRetries, subjectTargetChars int, context string, est TokenEstimator) int {
	n := maxDuplicateRetries
	if n < 0 { // clamp: a negative config ⇒ no rejection slots ⇒ normal-instruction overhead only.
		n = 0
	}
	subjLen := max(subjectTargetChars, 1) // guard strings.Repeat (negative panics; clamp to ≥1 for a meaningful subject)
	worstRejected := make([]string, n)
	for i := range worstRejected {
		worstRejected[i] = strings.Repeat("x", subjLen)
	}
	overhead := BuildUserPayload("", context, worstRejected) // empty diff ⇒ pure overhead (worst-case rejection path)
	return measureReserve(sysPrompt, overhead, est)
}

// PlannerReserveTokens computes the worst-case prompt reserve for the PLANNER role (FR3d/FR3i). sysPrompt
// is BuildPlannerSystemPrompt's output. The planner has ONE retry (PlannerRetryInstruction prepended on
// parse failure) and NO growing rejection block, so the worst case = the retry preamble + the (possibly
// forced-count) instruction + context. The overhead is built via BuildPlannerUserPayload with an empty
// diff (isolates the non-diff overhead — single source of truth), with PlannerRetryInstruction prepended
// (the worst case includes the retry).
//
// Consumer: decompose.callPlanner.
func PlannerReserveTokens(sysPrompt string, forcedCount int, context string, est TokenEstimator) int {
	overhead := PlannerRetryInstruction + "\n\n" + BuildPlannerUserPayload("", context, forcedCount)
	return measureReserve(sysPrompt, overhead, est)
}

// ArbiterReserveTokens computes the worst-case prompt reserve for the ARBITER role (FR3d/FR3i). sysPrompt
// is BuildArbiterSystemPrompt()'s output (zero-arg §17.7 constant). The arbiter runs ONCE (no growing
// block); its non-diff overhead is the commits + headers block. BuildArbiterUserPayload with an empty
// leftoverDiff isolates it (single source of truth). `commits` is the converted []ArbiterCommit (the
// caller obtains it via decompose.convertArbiterCommits — design-decisions.md §8).
//
// Consumer: decompose.runArbiterPhase.
func ArbiterReserveTokens(sysPrompt string, commits []ArbiterCommit, est TokenEstimator) int {
	overhead := BuildArbiterUserPayload(commits, "") // empty leftoverDiff ⇒ pure overhead (commits + headers)
	return measureReserve(sysPrompt, overhead, est)
}
