# closedLoopGate convergence + invariant pure tests — P1.M1.T3.S1 Research

> Verified against the live repo (module `github.com/dustin/stagecoach`, 2026-07-07). No files modified —
> research only. This is a TEST-ONLY task (1 point) for the FR3j closed-loop gate.

## 1. What this task is (and is not)

Add the closedLoopGate convergence + invariant pure unit tests called for by the contract (FR3j, PRD
§9.1 FR3j / §20.2). The gate under test — `closedLoopGate` (internal/git/tokengate.go:195) — is PURE
(no git, no ctx, no I/O): it wraps `applyWaterFillGate` with a measure→reduce→rerun loop bounded at
`maxClosedLoopPasses` (tokengate.go:83, =4). The tests use SYNTHETIC measure callbacks (closures) to
model the real "measure = EstimateTokens(sysPrompt + payload)" behavior without importing
internal/prompt (the leaf-purity invariant).

**Scope fence (the plan):**
- **This T3.S1** = the PURE unit tests in `internal/git/tokengate_test.go` (convergence + invariant).
  PURE (no repo) → runs in the standard CI unit-test pass.
- **P1.M1.T3.S2** = the E2E test for the assembled-prompt-≤-token_limit invariant (a real repo +
  assembled-prompt measurement). DISTINCT — do NOT duplicate.
- **P1.M1.T2.S2 (parallel)** = wires `MeasureAssembled` closures at the 3 decompose consumer sites
  (`internal/decompose/{message,planner,decompose}.go`). Touches NO `internal/git/` test files → NO conflict.

## 2. The gate under test — `closedLoopGate` (tokengate.go:195, LANDED by S1)

```go
func closedLoopGate(mdDiffs []string, nmDiff, skeleton string, tokenLimit, promptReserve int,
	measure func(string) int) string {
	if measure == nil {
		return applyWaterFillGate(mdDiffs, nmDiff, skeleton, tokenLimit, promptReserve) // nil-safe seam
	}
	gatedDiff := applyWaterFillGate(mdDiffs, nmDiff, skeleton, tokenLimit, promptReserve) // first cut
	bestDiff := gatedDiff
	bestMeasured := measure(gatedDiff)                                                    // call #1
	for pass := 0; pass < maxClosedLoopPasses; pass++ {                                   // ≤ 4 passes
		if bestMeasured <= tokenLimit { return bestDiff }                                 // invariant holds
		overshoot := bestMeasured - tokenLimit
		effectiveLimit := tokenLimit - overshoot - closedLoopSlack
		if effectiveLimit < 1 { effectiveLimit = 1 }                                      // floor
		gatedDiff = applyWaterFillGate(mdDiffs, nmDiff, skeleton, effectiveLimit, promptReserve)
		measured := measure(gatedDiff)                                                    // call #2..#5
		if measured < bestMeasured { bestDiff = gatedDiff; bestMeasured = measured }      // track best
		if measured <= tokenLimit { return gatedDiff }                                    // invariant holds
	}
	return bestDiff // best effort after maxClosedLoopPasses (adversarial estimator can always lie)
}
```

**Measure call pattern (deterministic — load-bearing for the stateful test):** measure is called
**once** for the first-cut (before the loop) + **once per pass** (≤ maxClosedLoopPasses). So the total
call count is ≤ `maxClosedLoopPasses + 1` (= 5). Early `return` paths (invariant holds) call it fewer
times; the adversarial never-satisfied path calls it exactly `maxClosedLoopPasses + 1` times.

**The invariant (FR3j):** `measure(gatedDiff) ≤ tokenLimit` when achievable; on an adversarial
estimator that always reports over, the bound guarantees termination and the BEST (smallest measured)
attempt is returned.

## 3. What ALREADY EXISTS (S1 shipped closedLoopGate + its tests together)

`internal/git/tokengate_test.go:305-429` already contains 5 closedLoopGate pure tests (added by S1,
P1.M1.T1.S1 — "Complete"). Mapping to the contract's four required cases:

| Contract case | Existing test (tokengate_test.go) | Covered? |
|---|---|---|
| **(c) NO-OP** (measure==nil → first-cut byte-identical) | `TestClosedLoopGate_NilMeasure_DelegatesToFirstCut` (:317) | ✅ **FULLY** — asserts `got == firstCut` |
| **(d) UNDER-BUDGET** (first-cut fits → zero passes, unchanged) | `TestClosedLoopGate_WithinBudget_NoRetrim` (:329) | ✅ **FULLY** — asserts `got == firstCut` AND `measure(got) ≤ tokenLimit` |
| **(a) INVARIANT** (drift → over budget → re-trim until `measure(got) ≤ tokenLimit`) | `TestClosedLoopGate_OverBudget_RetrimmedToFit` (:350) | ✅ **FULLY** — uses `prefixOverhead + EstimateTokens(gatedDiff)` measure; asserts `measure(got) ≤ tokenLimit` AND `len(got) < len(firstCut)`; has a premise sanity check (`measure(firstCut) > tokenLimit`) |
| **(b) CONVERGENCE** (adversarial drift → converges within maxClosedLoopPasses) | `TestClosedLoopGate_MaxPassesBound` (:385) | ⚠️ **PARTIAL** — uses a CONSTANT adversarial measure (`tokenLimit + 1000`); asserts termination + `len(got) ≤ len(firstCut)`. Does NOT cover "grows with each trim" drift; does NOT assert the pass-count bound. |
| (edge: effectiveLimit < 1 floor) | `TestClosedLoopGate_EffectiveLimitFloor` (:409) | ✅ (bonus, not in contract) |

**The gap:** the contract's (b) explicitly calls for *"adversarial drift (e.g., measure returns a value
that GROWS with each trim)"* — a measure that WORSENS as the loop trims. The existing `MaxPassesBound`
uses a CONSTANT measure (invariant to trimming). The growing-drift case is a DISTINCT adversarial
pattern: trimming makes the measure worse, so the best-attempt tracking keeps the FIRST-CUT (not a
trimmed later attempt). AND no existing test asserts the loop respects the `maxClosedLoopPasses` call
bound (a stateful call-counting measure).

## 4. The deliverable — ONE new test (the (b) growing-drift convergence variant)

A stateful measure that (a) GROWS each call (adversarial drift — trimming can never satisfy the
invariant) and (b) COUNTS calls (to assert the bound). With a growing measure:

- first-cut: `bestDiff = firstCut`, `bestMeasured = measure(firstCut)` (call #1; the smallest value the
  growing measure ever returns).
- each pass: `measured = measure(gatedDiff)` is LARGER than `bestMeasured` (it grew) → `measured <
  bestMeasured` is FALSE → `bestDiff` stays the first-cut; `measured ≤ tokenLimit` is FALSE → no early
  return. The loop runs all `maxClosedLoopPasses` passes and returns the first-cut (the best attempt).

**Assertions (the three convergence properties):**
1. **TERMINATES** — the call returns (an unbounded loop would hang the test runner; the test completing
   is the proof).
2. **BOUNDED** — `calls ≤ maxClosedLoopPasses + 1` (the loop respected the bound: 1 first-cut measure +
   ≤ maxClosedLoopPasses pass measures).
3. **BEST ATTEMPT** — `got == firstCut` (with a growing measure, trimming worsens the measure, so the
   best attempt is the untrimmed first-cut; the loop never updates `bestDiff`).

```go
// TestClosedLoopGate_AdversarialDrift_GrowingMeasure (FR3j contract (b) — the "grows with each trim"
// drift variant): an adversarial measure that GROWS each call models estimation drift where re-measurement
// reports LARGER on smaller inputs (or accumulates overhead each pass). Trimming can never satisfy the
// invariant (each re-measure is worse than the last), so the loop must TERMINATE at the maxClosedLoopPasses
// bound and return the best attempt (the first-cut — later trims are strictly worse). Distinct from
// TestClosedLoopGate_MaxPassesBound (constant adversarial): there the measure is invariant to trimming;
// here trimming makes it worse, exercising the best-attempt-keeps-first-cut path.
func TestClosedLoopGate_AdversarialDrift_GrowingMeasure(t *testing.T) {
	section := bodySection("src/big.go", 4000) // a sizeable body so the gate trims on re-runs
	tokenLimit := tokenBudgetMargin + 200
	firstCut := applyWaterFillGate(nil, section, "", tokenLimit, 0)

	// Stateful adversarial drift: GROWS each call AND counts calls. Every value > tokenLimit, so the
	// invariant can never hold (the loop must hit the maxClosedLoopPasses bound).
	var calls int
	measure := func(gatedDiff string) int {
		calls++
		return tokenLimit + 100 + calls*50 // call#1: +150; #2: +200; #3: +250; … (all > tokenLimit)
	}

	got := closedLoopGate(nil, section, "", tokenLimit, 0, measure)

	// (1) TERMINATES: the call returned (implicit — a regression to an unbounded loop hangs the runner).
	// (2) BOUNDED: measure is called once for the first-cut + ≤ maxClosedLoopPasses passes ⇒ calls ≤ maxClosedLoopPasses+1.
	if calls > maxClosedLoopPasses+1 {
		t.Errorf("loop exceeded maxClosedLoopPasses: measure called %d times, want ≤ %d", calls, maxClosedLoopPasses+1)
	}
	// (3) BEST ATTEMPT: with a growing measure, trimming worsens the measure, so the best attempt is the
	// first-cut (the loop never updates bestDiff). got == firstCut, byte-identical.
	if got != firstCut {
		t.Errorf("growing-drift: best attempt must be the first-cut (trimming worsens the measure);\nfirstCut=\n%s\ngot=\n%s", firstCut, got)
	}
}
```

**Why a stateful measure is acceptable here:** closedLoopGate's measure call pattern is DETERMINISTIC
(documented in §2: one first-cut call + one per pass, in order). The stateful counter is robust to that
deterministic order. The test does NOT call `measure(got)` in an assertion (that would re-increment
`calls` and return a misleading value); it asserts `got == firstCut` (structural) + `calls ≤ bound`.

## 5. Why NOT duplicate (a)/(c)/(d)

The contract lists four tests (a)-(d), but it was authored against a state where S1 had not yet shipped
the closedLoopGate tests. S1 (P1.M1.T1.S1 — "Complete") shipped the function AND its pure unit tests
together (the codebase pattern: the function subtask includes its smoke tests — cf. plan/008
OverlayTreePaths S1). Three of the four contract cases — (a) `OverBudget_RetrimmedToFit`, (c)
`NilMeasure_DelegatesToFirstCut`, (d) `WithinBudget_NoRetrim` — are already FULLY covered (with the
exact assertions the contract specifies). Re-adding them would be duplicate churn that crosses S1's
territory. The honest deliverable is the ONE missing variant — (b)'s growing-drift convergence test
(§4) — plus confirming the three existing tests cover their cases (cross-reference, don't duplicate).

## 6. Decisions log

- **D1** (a)/(c)/(d) are ALREADY covered by S1's tests — VERIFY (cross-reference by name + line), do NOT
  duplicate. The contract predates S1's test shipping.
- **D2** the genuine gap is (b)'s "grows with each trim" drift variant (the existing `MaxPassesBound`
  uses a CONSTANT measure) + the `maxClosedLoopPasses` call-count bound (asserted nowhere). ADD ONE test
  covering both.
- **D3** the growing-drift measure is STATEFUL (grows + counts). Acceptable because closedLoopGate's call
  pattern is deterministic (§2). Do NOT call `measure(got)` in the assertion (re-increments the counter);
  assert `got == firstCut` + `calls ≤ maxClosedLoopPasses + 1`.
- **D4** the test asserts the THREE convergence properties: terminates (implicit), bounded
  (`calls ≤ maxClosedLoopPasses+1`), best-attempt (`got == firstCut`).
- **D5** PURE test (no repo, no t.TempDir, no I/O) — matches tokengate_test.go's established style
  (bodySection helper + synthetic measures + structural assertions). Runs in the standard CI unit pass.
- **D6** scope: ONLY `internal/git/tokengate_test.go` (append ONE test). No production code, no other
  test files. The parallel T2.S2 touches `internal/decompose/*` — no conflict.
