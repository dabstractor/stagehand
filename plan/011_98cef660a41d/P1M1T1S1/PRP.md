---
name: "P1.M1.T1.S1 — Add MeasureAssembled field + closedLoopGate pure function"
description: |
  FR3j closed-loop token-budget guarantee, step 1 of 3. Add `MeasureAssembled func(gatedDiff string) int`
  to `StagedDiffOptions` (git.go) — the injection seam for the closed-loop assembly measurement (nil-safe).
  Implement a PURE `closedLoopGate` function in `tokengate.go` that calls `applyWaterFillGate` for the
  first-cut, measures the assembled prompt via the injected callback, and if over `tokenLimit`, reduces
  the effective limit by the overshoot+slack and re-runs the gate (bounded at ~4 passes). The function is
  PURE (no git, no ctx, no I/O) so it is exhaustively unit-testable without a repo. Consumed by S2
  (the 3 diff functions) and S3 (the invariant tests). No external docs (internal API seam).
---

## Goal

**Feature Goal**: Land the FR3j closed-loop budget guarantee mechanism — a pure `closedLoopGate` function
that iteratively re-applies the water-fill gate until the assembled prompt (system prompt + payload) fits
within `token_limit`, plus the `MeasureAssembled` injection seam on `StagedDiffOptions` that lets the
prompt/generate layers provide the "assemble and measure" callback without creating an import edge.

**Deliverable** (1 struct field + 1 pure function + pure unit tests):
1. `internal/git/git.go` `StagedDiffOptions`: add `MeasureAssembled func(gatedDiff string) int` (nil-safe).
2. `internal/git/tokengate.go`: add `closedLoopGate(mdDiffs []string, nmDiff, skeleton string, tokenLimit, promptReserve int, measure func(string) int) string` — the bounded iterative re-trim.
3. `internal/git/tokengate_test.go`: add pure unit tests for closedLoopGate (convergence, nil-measure no-op, within-budget no-op, overshoot correction, maxPasses bound).

**Success Definition**: `closedLoopGate` returns a gated diff string that, when assembled and measured by
the injected callback, satisfies `measure(gatedDiff) ≤ tokenLimit` (the FR3j invariant). When `measure==nil`,
it delegates to `applyWaterFillGate` with no loop (behavior unchanged). `go build/vet/gofmt` clean;
`go test -race ./...` green. Consumed unchanged by S2/S3.

## Why

- **FR3j's hard invariant.** The current `applyWaterFillGate` is an open-loop first-cut: it subtracts
  *estimates* (promptReserve, skeletonTokens, margin) from `tokenLimit`. Estimation drift (chars/4 vs real
  density; worst-case rejection block vs actual framing; skeleton measured in isolation) can let the
  assembled prompt land slightly over `tokenLimit`. FR3j closes the loop: measure the *actual* assembled
  prompt and re-trim if needed. Invariant: `EstimateTokens(assembledFullPrompt) ≤ tokenLimit`, always.
- **Leaf-purity preserved.** `internal/git` cannot import `internal/prompt` (the leaf-purity invariant).
  The `MeasureAssembled` callback follows the existing `TokenEstimator` injection pattern
  (`internal/prompt/reserve.go:19`) in reverse: the caller injects a closure that captures `sysPrompt` +
  the role-specific builder, so `internal/git` measures without importing the builder. No new import edge.
- **Pure function = exhaustively testable.** `closedLoopGate` is pure (no git, no ctx, no I/O) — it can be
  tested with string-literal inputs and a synthetic `measure` callback, exactly like `applyWaterFillGate`.
- **Foundation for S2/S3.** S2 wires `closedLoopGate` into the 3 diff functions; S3 adds the invariant
  tests + the e2e guarantee test. S1 provides the function they call.

## What

A new struct field (the injection seam) + a new pure function (the closed-loop algorithm) + pure unit tests.
No wiring (S2), no e2e tests (S3), no external docs (internal API seam only).

### Success Criteria

- [ ] `StagedDiffOptions` has `MeasureAssembled func(gatedDiff string) int` (nil-safe, documented).
- [ ] `closedLoopGate(mdDiffs, nmDiff, skeleton, tokenLimit, promptReserve, measure)` exists in `tokengate.go`.
- [ ] When `measure == nil`: delegates to `applyWaterFillGate(...)` with no loop (behavior unchanged).
- [ ] When `measure(gatedDiff) <= tokenLimit`: returns the first-cut (invariant holds, no re-trim).
- [ ] When over: reduces `effectiveLimit = tokenLimit - (assembled - tokenLimit) - slack`, re-runs the gate,
      re-measures, bounded at `maxClosedLoopPasses` (~4). Returns the final result.
- [ ] Pure function: no git, no ctx, no I/O, no imports beyond what `tokengate.go` already has.
- [ ] Pure unit tests pass (convergence, nil-no-op, within-budget-no-op, overshoot-correction, maxPasses-bound).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.

## All Needed Context

### Documentation & References

```yaml
- docfile: plan/011_98cef660a41d/architecture/fr3j_closed_loop.md
  why: "The authoritative design. Documents the open-loop drift sources (chars/4 vs real density, worst-case framing vs actual, skeleton-in-isolation), the closed-loop algorithm (first-cut → measure → if-over-reduce-and-rerun, bounded ~4 passes), the injection seam (MeasureAssembled callback, mirroring reserve.go's TokenEstimator pattern in reverse), the 3 diff-function gate call sites (where the loop will live in S2), and the 6 consumer wiring sites (where the closures are built in S2's sibling tasks)."
  critical: "The closed loop MUST live INSIDE the >0 branches of the 3 diff functions (S2) because only there are mdDiffs/nmDiff/skeleton in scope. S1 provides the closedLoopGate function that S2 wraps. The measure callback captures sysPrompt + the role-specific builder. nil-safe: measure==nil OR TokenLimit==0 → first-cut only, behavior unchanged."

- file: internal/git/git.go
  why: "EDIT — StagedDiffOptions struct (line ~38). Add MeasureAssembled after PromptReserveTokens. Document the FR3j closed-loop seam + nil-safe semantics."
  pattern: "Mirror PromptReserveTokens' doc-comment style: a multi-line comment explaining what it is, who provides it, the nil-safe default. The field type is func(gatedDiff string) int."

- file: internal/git/tokengate.go
  why: "EDIT — add closedLoopGate after applyWaterFillGate (~line 155). The function delegates to applyWaterFillGate for each pass, measures via the callback, and re-trims if over."
  pattern: "applyWaterFillGate is the existing first-cut allocator (line 121). closedLoopGate wraps it with the measure→reduce→rerun loop. Add a const maxClosedLoopPasses = 4 and const closedLoopSlack = 64 (tokens) near the existing tokenBudgetMargin/minBodyTokens constants. Reuse EstimateTokens, applyWaterFillGate — NO new imports (all already in tokengate.go)."

- file: internal/git/tokengate_test.go
  why: "EDIT — add pure unit tests for closedLoopGate. Use the existing table-driven style + synthetic measure callbacks (lambdas that return a pre-set int for a given input, to simulate drift)."
  pattern: "TestClosedLoopGate_NilMeasure_DelegatesToFirstCut: measure==nil → same output as applyWaterFillGate. TestClosedLoopGate_WithinBudget_NoRetrim: measure returns ≤ tokenLimit → same as first-cut. TestClosedLoopGate_OverBudget_RetrimmedToFit: measure returns > tokenLimit on first cut but ≤ after re-trim. TestClosedLoopGate_MaxPassesBound: measure always returns over → bounded at maxClosedLoopPasses, returns the last attempt (doesn't infinite-loop)."

- file: internal/git/tokens.go # EstimateTokens
  why: "READ-ONLY — the single estimator (ceil(runes/4)). closedLoopGate delegates to the injected measure callback, which in production calls EstimateTokens(sysPrompt + payload). The unit tests use synthetic measures that don't need EstimateTokens."

- file: internal/prompt/reserve.go # TokenEstimator (:19)
  why: "READ-ONLY — the precedent for the injection pattern. reserve.go accepts a func(string) int to keep internal/prompt from importing internal/git for estimation. MeasureAssembled is the reverse direction (internal/git accepts a func to avoid importing internal/prompt for assembly). Same design call."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/git/
    ├── git.go           # EDIT: + MeasureAssembled field on StagedDiffOptions
    ├── tokengate.go     # EDIT: + closedLoopGate function + constants
    └── tokengate_test.go # EDIT: + TestClosedLoopGate_* pure unit tests
```

### Known Gotchas

```go
// CRITICAL (nil-safe): when measure == nil OR tokenLimit == 0, closedLoopGate MUST return the exact
// same output as applyWaterFillGate — NO loop, NO re-trim. This is the "behavior unchanged when the
// seam is not used" guarantee that S2's consumers rely on (a consumer that doesn't wire the closure
// gets the first-cut, exactly as today).

// CRITICAL (slack prevents oscillation): when reducing effectiveLimit, subtract the overshoot PLUS
// a slack (closedLoopSlack = 64 tokens). Without slack, the second pass might land exactly at the
// boundary and oscillate (measure says 1 over → trim 1 → measure says 1 over again). The slack
// ensures each pass makes strictly more progress than the overshoot.

// CRITICAL (maxPasses bound): the loop MUST terminate even if the estimator is adversarial (always
// reports over). maxClosedLoopPasses = 4 ensures termination. On exit, return the best attempt
// (the one with the smallest measured value seen). The invariant is "best effort" — an adversarial
// estimator can always lie; the bound prevents infinite looping.

// GOTCHA (the first-cut is ALWAYS applyWaterFillGate): closedLoopGate does NOT re-implement the
// water-fill — it DELEGATES to applyWaterFillGate for every pass, just at a reduced tokenLimit.
// This ensures the FR3i water-fill fairness (per-file level, no static cap) is preserved across
// every pass — it's the SAME allocator, just with a tighter budget.

// GOTCHA (measure measures the ASSEMBLED prompt, not just the diff): the callback's job is to
// return EstimateTokens(sysPrompt + BuildUserPayload(gatedDiff, ...)) — the WHOLE prompt, not just
// the diff body. This is why PromptReserveTokens (the first-cut estimate) is insufficient for the
// hard guarantee — it's an estimate of the non-diff prefix; the closed loop measures the ACTUAL
// assembled prompt. The unit tests simulate this with a synthetic measure that models the prefix
// as a constant added to the gated diff's token count.
```

## Implementation Blueprint

### Implementation Tasks

```yaml
Task 1: git.go — add MeasureAssembled field to StagedDiffOptions
  - LOCATE: StagedDiffOptions, after the PromptReserveTokens field (the last FR3i field).
  - ADD:
        // FR3j: closed-loop budget guarantee. When non-nil AND TokenLimit > 0, the diff functions
        // (S2) call closedLoopGate which measures the ASSEMBLED prompt (system prompt +
        // BuildUserPayload(gatedDiff)) via this callback and re-trims if over tokenLimit.
        // nil ⇒ first-cut only (applyWaterFillGate, behavior unchanged). The callback is injected
        // by the consumer (generate/prompt layers) to avoid an internal/git → internal/prompt import
        // (mirrors internal/prompt/reserve.go's TokenEstimator injection in reverse). See FR3j.
        MeasureAssembled func(gatedDiff string) int

Task 2: tokengate.go — add constants + closedLoopGate function
  - ADD constants near the existing tokenBudgetMargin/minBodyTokens:
        const maxClosedLoopPasses = 4  // FR3j: bounded loop (the estimate is already close → 1-2 passes typical)
        const closedLoopSlack = 64    // FR3j: extra tokens shaved per overshoot to prevent oscillation at the boundary

  - ADD after applyWaterFillGate (~line 155):
        // closedLoopGate is the FR3j closed-loop budget guarantee. It calls applyWaterFillGate for the
        // first-cut, measures the assembled prompt via the injected measure callback, and if over
        // tokenLimit, reduces the effective limit by the overshoot + slack and re-runs the gate.
        // Bounded at maxClosedLoopPasses. When measure == nil, delegates to applyWaterFillGate with
        // no loop (behavior unchanged — the nil-safe seam). PURE: no git, no ctx, no I/O.
        //
        // The measure callback captures sysPrompt + the role-specific payload builder (injected by
        // the consumer) so internal/git can measure the WHOLE assembled prompt without importing
        // internal/prompt (the leaf-purity invariant). See architecture/fr3j_closed_loop.md.
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
                    return bestDiff // invariant holds
                }
                overshoot := bestMeasured - tokenLimit
                effectiveLimit := tokenLimit - overshoot - closedLoopSlack
                if effectiveLimit < 1 {
                    effectiveLimit = 1 // floor: don't go below 1 (applyWaterFillGate handles minBodyTokens)
                }
                gatedDiff = applyWaterFillGate(mdDiffs, nmDiff, skeleton, effectiveLimit, promptReserve)
                measured := measure(gatedDiff)
                if measured < bestMeasured {
                    bestDiff = gatedDiff
                    bestMeasured = measured
                }
                if measured <= tokenLimit {
                    return gatedDiff // invariant holds
                }
            }
            return bestDiff // best effort after maxPasses (an adversarial estimator can always lie)
        }

Task 3: tokengate_test.go — add pure unit tests for closedLoopGate
  - ADD (pure table/func tests, same package, synthetic measure callbacks):
  - TestClosedLoopGate_NilMeasure_DelegatesToFirstCut:
        measure == nil → output == applyWaterFillGate(...) (identical).
  - TestClosedLoopGate_WithinBudget_NoRetrim:
        measure returns ≤ tokenLimit on first cut → output == first-cut (no loop).
  - TestClosedLoopGate_OverBudget_RetrimmedToFit:
        measure returns tokenLimit + 100 on first cut, then ≤ tokenLimit after the effectiveLimit
        reduction → output is the trimmed version (shorter than first-cut). Assert measure(final) ≤ tokenLimit.
  - TestClosedLoopGate_MaxPassesBound:
        measure ALWAYS returns tokenLimit + 1000 (adversarial — never fits) → returns the bestDiff
        after maxClosedLoopPasses iterations. Assert no infinite loop (the test terminates).
  - TestClosedLoopGate_EffectiveLimitFloor:
        effectiveLimit drops below 1 → clamped to 1 (applyWaterFillGate's minBodyTokens path fires).
  - SYNTHETIC MEASURE PATTERN: use a closure that captures a "prefix overhead" constant and adds it
    to EstimateTokens(gatedDiff):
        prefixOverhead := 500 // simulates sysPrompt + framing tokens
        measure := func(gatedDiff string) int {
            return prefixOverhead + EstimateTokens(gatedDiff)
        }
    This models the real behavior (measure = the whole assembled prompt) without importing internal/prompt.

Task 4: VALIDATE
  - RUN: gofmt -w internal/git/git.go internal/git/tokengate.go internal/git/tokengate_test.go
  - RUN: go build ./... ; go vet ./...
  - RUN: go test -race ./internal/git/ -v -run TestClosedLoopGate
  - RUN: go test -race ./...
```

### Implementation Patterns & Key Details

```go
// === closedLoopGate — the bounded iterative re-trim (the complete function) ===
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
			return bestDiff // invariant holds
		}
		overshoot := bestMeasured - tokenLimit
		effectiveLimit := tokenLimit - overshoot - closedLoopSlack
		if effectiveLimit < 1 {
			effectiveLimit = 1
		}
		gatedDiff = applyWaterFillGate(mdDiffs, nmDiff, skeleton, effectiveLimit, promptReserve)
		measured := measure(gatedDiff)
		if measured < bestMeasured {
			bestDiff = gatedDiff
			bestMeasured = measured
		}
		if measured <= tokenLimit {
			return gatedDiff
		}
	}
	return bestDiff
}
```

```go
// === synthetic measure pattern for unit tests ===
prefixOverhead := 500 // simulates sysPrompt + framing tokens
measure := func(gatedDiff string) int {
    return prefixOverhead + EstimateTokens(gatedDiff)
}
```

### Integration Points

```yaml
STRUCT FIELD (internal/git/git.go StagedDiffOptions):
  - added: "MeasureAssembled func(gatedDiff string) int"  # nil-safe FR3j seam

PURE FUNCTION (internal/git/tokengate.go):
  - added: "func closedLoopGate(mdDiffs, nmDiff, skeleton, tokenLimit, promptReserve, measure) string"
  - constants: maxClosedLoopPasses=4, closedLoopSlack=64
  - delegates to applyWaterFillGate for every pass (NO re-implementation of water-fill)

TESTS (internal/git/tokengate_test.go):
  - +TestClosedLoopGate_NilMeasure_DelegatesToFirstCut
  - +TestClosedLoopGate_WithinBudget_NoRetrim
  - +TestClosedLoopGate_OverBudget_RetrimmedToFit
  - +TestClosedLoopGate_MaxPassesBound
  - +TestClosedLoopGate_EffectiveLimitFloor

NO-TOUCH (owned by S2/S3):
  - internal/git/git.go StagedDiff/TreeDiff/WorkingTreeDiff (S2 = P1.M1.T1.S2: wire closedLoopGate into the >0 branches)
  - the 6 consumer wiring sites (P1.M1.T2.S1/S2: wire the MeasureAssembled closures)
  - invariant/e2e tests (P1.M1.T3.S1/S2)
  - any other package; docs; PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS:
  - S2 (P1.M1.T1.S2): replace `applyWaterFillGate(...)` with `closedLoopGate(..., opts.MeasureAssembled)`
    in the >0 branches of StagedDiff/TreeDiff/WorkingTreeDiff (3 call sites).
  - S2 (P1.M1.T2.S1/S2): wire the 6 MeasureAssembled closures (message/planner/arbiter × generate/pkg/hook/decompose).
  - S3 (P1.M1.T3.S1/S2): pure invariant tests + e2e "assembled ≤ token_limit" test.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach
gofmt -w internal/git/git.go internal/git/tokengate.go internal/git/tokengate_test.go
gofmt -l .            # Expected: empty
go vet ./internal/git/...  # Expected: exit 0
go build ./...        # Expected: exit 0
```

### Level 2: Unit Tests

```bash
cd /home/dustin/projects/stagecoach
go test -race ./internal/git/ -v -run TestClosedLoopGate   # all 5 cases pass
go test -race ./internal/git/ -v                           # full git suite green
```

### Level 3: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagecoach
go test -race ./...   # Expected: ALL packages green
git diff --stat       # Expected: internal/git/{git,tokengate,tokengate_test}.go only
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.

### Feature Validation
- [ ] `StagedDiffOptions.MeasureAssembled func(gatedDiff string) int` exists (nil-safe).
- [ ] `closedLoopGate` delegates to `applyWaterFillGate` when `measure==nil`.
- [ ] `closedLoopGate` returns the first-cut when `measure(gatedDiff) <= tokenLimit`.
- [ ] `closedLoopGate` reduces effectiveLimit and re-runs when over (bounded at maxClosedLoopPasses).
- [ ] Pure function (no git, no ctx, no I/O, no new imports).
- [ ] 5 pure unit tests pass.

### Scope Discipline
- [ ] ONLY `internal/git/{git,tokengate,tokengate_test}.go` modified.
- [ ] Did NOT wire into StagedDiff/TreeDiff/WorkingTreeDiff (S2); wire closures (P1.M1.T2); e2e tests (P1.M1.T3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Anti-Patterns to Avoid

- ❌ Don't re-implement the water-fill inside closedLoopGate — DELEGATE to `applyWaterFillGate` for every
  pass. Re-implementing it would duplicate the FR3i algorithm and risk divergence.
- ❌ Don't forget the nil-safe path — `measure == nil` MUST return `applyWaterFillGate(...)` with no loop.
  This is the "behavior unchanged when the seam isn't used" guarantee.
- ❌ Don't skip the slack (`closedLoopSlack`). Without it, a boundary oscillation (measure says 1 over →
  trim 1 → measure says 1 over) can waste passes. The slack ensures strictly-more-progress-per-pass.
- ❌ Don't skip the `maxClosedLoopPasses` bound. An adversarial estimator (always reports over) must not
  cause an infinite loop. The bound + best-effort return is the guarantee.
- ❌ Don't forget the `effectiveLimit < 1` floor. A pathological overshoot could make effectiveLimit
  negative; clamp to 1 (applyWaterFillGate's minBodyTokens path handles it).
- ❌ Don't import `internal/prompt` — the whole point of the `MeasureAssembled` callback is to keep
  `internal/git` a leaf (no prompt import). The callback is injected by the CONSUMER.

---

## Confidence Score

**9/10** — the arch `fr3j_closed_loop.md` supplies the algorithm (first-cut → measure → if-over-reduce-and-rerun,
bounded ~4 passes), the injection-seam design (MeasureAssembled callback, mirroring reserve.go's TokenEstimator
in reverse), the nil-safe semantics, and the 3+6 downstream wiring sites. The function is pure (testable with
synthetic measure callbacks), delegates to the existing tested `applyWaterFillGate` for every pass, and the
constants (maxPasses=4, slack=64) are conservative. The one residual uncertainty (not 10/10) is the synthetic
measure callback's fidelity to the real assembled-prompt measurement — mitigated by the fact that the unit
test's purpose is to verify the LOOP LOGIC (convergence, nil-safety, maxPasses bound), not the estimator
accuracy (which is S3's e2e test's job). The wiring (S2) and e2e invariant (S3) are cleanly fenced.
