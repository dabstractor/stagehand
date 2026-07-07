---
name: "P1.M1.T3.S1 — Pure unit tests for closedLoopGate convergence + invariant (FR3j)"
description: |
  TEST-ONLY (1 point). Add the closedLoopGate convergence + invariant pure unit tests for FR3j (PRD
  §9.1 FR3j / §20.2). The gate — `closedLoopGate` (internal/git/tokengate.go:195, LANDED by S1) — is
  PURE (no git, no ctx, no I/O); the tests use SYNTHETIC measure callbacks (closures) and run in the
  standard CI unit-test pass. STATUS: S1 (P1.M1.T1.S1 — "Complete") shipped closedLoopGate AND its pure
  unit tests TOGETHER (tokengate_test.go:305-429), which ALREADY FULLY COVER three of the contract's
  four cases: (a) INVARIANT (`TestClosedLoopGate_OverBudget_RetrimmedToFit` :350), (c) NO-OP
  (`TestClosedLoopGate_NilMeasure_DelegatesToFirstCut` :317), (d) UNDER-BUDGET
  (`TestClosedLoopGate_WithinBudget_NoRetrim` :329). The ONE GENUINE GAP is contract (b)'s specific
  "grows with each trim" adversarial-drift variant — the existing `TestClosedLoopGate_MaxPassesBound`
  (:385) uses a CONSTANT measure (invariant to trimming) and asserts NO pass-count bound. This task
  ADDS ONE test — `TestClosedLoopGate_AdversarialDrift_GrowingMeasure` — with a stateful measure that
  GROWS each call (adversarial drift: trimming can never satisfy the invariant) AND COUNTS calls,
  asserting the three convergence properties: (1) TERMINATES (the call returns — a regression to an
  unbounded loop would hang the runner), (2) BOUNDED (`calls ≤ maxClosedLoopPasses+1` — the first
  pass-count bound assertion in the suite), (3) BEST ATTEMPT (`got == firstCut` — with a growing
  measure, trimming worsens the measure, so bestDiff stays the first-cut). VERIFY (do NOT duplicate)
  (a)/(c)/(d) by cross-referencing the existing tests. PURE (no repo). ONLY internal/git/tokengate_test.go
  touched. No production code, no docs. The parallel P1.M1.T2.S2 touches internal/decompose/* only — no
  conflict.
---

## Goal

**Feature Goal**: Complete the pure-unit-test coverage for the FR3j closed-loop gate's convergence
behavior — specifically the adversarial-drift convergence case (contract (b)) that the existing
constant-measure test does not cover, plus the `maxClosedLoopPasses` call-count bound (asserted nowhere
in the suite today). Confirm (don't duplicate) the three already-covered cases (a)/(c)/(d).

**Deliverable** (ONE new test appended to `internal/git/tokengate_test.go`):
- `TestClosedLoopGate_AdversarialDrift_GrowingMeasure` — a stateful measure that grows each call +
  counts calls, asserting: terminates, `calls ≤ maxClosedLoopPasses+1`, and `got == firstCut` (best
  attempt when trimming worsens the measure).

No other files touched. No production code. No docs.

**Success Definition**: `go build/vet/gofmt` clean; `go test -race ./internal/git/` green with the new
test passing (and the 5 existing closedLoopGate tests still green); the new test proves the loop
terminates within the bound and returns the first-cut (best attempt) under a growing adversarial
measure; the three contract cases (a)/(c)/(d) remain covered by their existing tests (unchanged);
`git diff --stat` shows ONLY `internal/git/tokengate_test.go`.

## User Persona

**Target User**: The contributor/maintainer who needs confidence that the FR3j closed-loop gate
TERMINATES and returns a sane best-effort result under any measure callback — including a pathological
"estimation drift" callback that reports larger values on each re-measurement (the worst case the
bounded loop must survive). Also the reviewer auditing FR3j's hard guarantee before release.

**Use Case**: A future change to `closedLoopGate` (e.g., altering the effectiveLimit formula, the slack,
or the best-attempt tracking) is regression-caught by this test if it either (a) hangs (unbounded loop),
(b) exceeds `maxClosedLoopPasses`, or (c) returns a WORSE attempt than the first-cut under a drifting
measure. The test pins the convergence contract.

**User Journey**: `go test -race ./internal/git/ -run TestClosedLoopGate` → the 5 existing tests +
the new growing-drift test all PASS, proving the gate's nil-safety, within-budget no-op, over-budget
re-trim invariant, constant-adversarial bound, AND the growing-drift bound + best-attempt behavior.

**Pain Points Addressed**: Without the growing-drift test, a regression that breaks the loop's
termination or best-attempt tracking under a non-monotonic (worsening) measure would ship undetected —
the existing constant-adversarial test cannot distinguish "trimming helps" from "trimming hurts" (the
measure is invariant to trimming). The growing-drift case is the one that exercises the
"trimming-makes-it-worse → keep the first-cut" path.

## Why

- **PRD §9.1 FR3j mandates the closed-loop guarantee** (`measure(assembledFullPrompt) ≤ token_limit`,
  always; best-effort + bounded on an adversarial estimator). §20.2 (Property/invariant tests) is the
  testing-strategy mandate for exactly this kind of hard guarantee. The convergence bound
  (`maxClosedLoopPasses`) is the termination proof — it must be regression-pinned.
- **The contract's (b) explicitly calls for "grows with each trim" drift.** The existing
  `TestClosedLoopGate_MaxPassesBound` uses a CONSTANT measure (`tokenLimit + 1000`), which is invariant
  to trimming. The growing-drift case is a DISTINCT adversarial pattern (trimming worsens the measure)
  that exercises a different best-attempt path (`bestDiff` stays the first-cut). It is not covered today.
- **No existing test asserts the `maxClosedLoopPasses` call-count bound.** The constant-adversarial test
  asserts the call RETURNS (termination) but not HOW MANY passes ran. The growing-drift test's
  call-counting measure asserts `calls ≤ maxClosedLoopPasses+1` — the first explicit bound assertion.
- **Three of the four contract cases are already done.** S1 shipped closedLoopGate + its tests together
  (the codebase pattern). (a)/(c)/(d) are fully covered with the exact assertions the contract specifies.
  This task's honest contribution is the ONE missing variant (b's growing drift) + the bound — not a
  redundant rewrite of the three existing tests.

## What

Append ONE pure unit test to `internal/git/tokengate_test.go`:

`TestClosedLoopGate_AdversarialDrift_GrowingMeasure` — uses a stateful measure closure that (1) GROWS
each call (`tokenLimit + 100 + calls*50` — always over budget, and larger each call) and (2) COUNTS
calls. Because the measure grows, trimming can never satisfy the invariant (each re-measure is worse
than the last), so the loop must run to the `maxClosedLoopPasses` bound and return the best attempt —
which, with a growing measure, is the FIRST-CUT (later trims are strictly worse, so `bestDiff` is never
updated). The test asserts: (1) the call returns (terminates — implicit), (2) `calls ≤ maxClosedLoopPasses+1`
(the bound), (3) `got == firstCut` (best attempt).

VERIFY (cross-reference, don't duplicate) that the existing tests cover (a)/(c)/(d):
- (a) `TestClosedLoopGate_OverBudget_RetrimmedToFit` (:350) — `measure(got) ≤ tokenLimit` + shorter.
- (c) `TestClosedLoopGate_NilMeasure_DelegatesToFirstCut` (:317) — `got == firstCut` when measure==nil.
- (d) `TestClosedLoopGate_WithinBudget_NoRetrim` (:329) — `got == firstCut` + `measure(got) ≤ tokenLimit`.

### Success Criteria

- [ ] `internal/git/tokengate_test.go` has `TestClosedLoopGate_AdversarialDrift_GrowingMeasure` (PURE,
      no repo/tempDir/I/O — uses `bodySection` + a synthetic measure closure).
- [ ] The new test's measure GROWS each call (always > tokenLimit) AND counts calls.
- [ ] The new test asserts `calls ≤ maxClosedLoopPasses+1` (the pass-count bound).
- [ ] The new test asserts `got == firstCut` (best attempt = first-cut when trimming worsens).
- [ ] The new test passes (the loop terminates — no hang).
- [ ] The existing 5 closedLoopGate tests (including (a)/(c)/(d)) remain GREEN and UNCHANGED.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] ONLY `internal/git/tokengate_test.go` modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim `closedLoopGate` body (so the implementer
understands the deterministic measure-call pattern: 1 first-cut + ≤maxClosedLoopPasses passes), the
complete new test body (copy-paste-ready), the `bodySection`/`tokenBudgetMargin`/`maxClosedLoopPasses`
helpers/constants it reuses, the exact names + line numbers of the 5 existing tests (to verify, not
duplicate), the gap analysis (constant vs growing drift), and the hard scope fence (tokengate_test.go
only; T2.S2 touches internal/decompose/*). No inference required.

### Documentation & References

```yaml
# MUST READ — the FR3j spec, the gate, and this task's research
- file: PRD.md
  why: "§9.1 FR3j (the closed-loop budget guarantee: measure(assembledFullPrompt) ≤ token_limit always;
        bounded loop; best-effort on an adversarial estimator). §20.2 (Property/invariant tests — the
        testing-strategy mandate for hard guarantees like this)."
  critical: "FR3j's 'bounded loop — the estimate is already close, so it converges in one or two passes'
             + 'the maxClosedLoopPasses bound guarantees termination and the best attempt is returned' IS
             the convergence contract this test pins."

- docfile: plan/011_98cef660a41d/architecture/fr3j_closed_loop.md
  why: "The architecture spec for closedLoopGate: the loop structure (measure → if over, reduce
        effectiveLimit by overshoot+slack → re-run applyWaterFillGate → repeat), the bound (~4 passes),
        the invariant, and the injection seam (MeasureAssembled). §'Out of Scope' confirms multi-turn
        re-capture is excluded."
  critical: "The loop's best-effort semantics ('an adversarial estimator that always reports over cannot
             be satisfied, but the maxClosedLoopPasses bound guarantees termination and the best attempt
             is returned') is EXACTLY what the growing-drift test exercises."

- docfile: plan/011_98cef660a41d/P1M1T3S1/research/closedloop_gate_tests.md
  why: "THIS subtask's research: §2 the verbatim gate + the deterministic measure-call pattern; §3 the
        gap analysis (which of (a)-(d) already exist, with test names + lines); §4 the verbatim new test;
        §5 why not duplicate (a)/(c)/(d); §6 decisions D1-D6. READ THIS FIRST."
  critical: "§3 (the gap table) and §4 (the verbatim test) are the core. §2's measure-call-pattern
             paragraph is WHY `calls ≤ maxClosedLoopPasses+1` is the correct bound assertion."

- docfile: plan/011_98cef660a41d/P1M1T2S2/PRP.md
  why: "The parallel sibling (wire MeasureAssembled at decompose consumer sites). Confirms it touches
        ONLY internal/decompose/{message,planner,decompose}.go — NOT internal/git/tokengate_test.go.
        No conflict with this task."
  critical: "T2.S2 wires the closures that ACTIVATE the gate in production; T3.S1 (this) tests the gate's
             convergence logic directly with synthetic measures. Distinct layers, no file overlap."

# The code under test + the existing tests (all READ-ONLY — do NOT edit production or the existing tests)
- file: internal/git/tokengate.go
  why: "READ-ONLY (the gate under test). closedLoopGate at :195 (verbatim body quoted in the research §2).
        maxClosedLoopPasses const at :83 (=4). closedLoopSlack, applyWaterFillGate (:135), EstimateTokens
        (tokens.go:25). Confirms the measure-call pattern: 1 first-cut + ≤maxClosedLoopPasses passes."
  pattern: "The loop tracks bestDiff/bestMeasured, updates ONLY when measured < bestMeasured (strictly
            less), floors effectiveLimit at 1, returns bestDiff on exit."
  gotcha: "The measure is called once BEFORE the loop (the first-cut) and once per pass. A stateful
           measure's call count is therefore ≤ maxClosedLoopPasses+1. Asserting `==` is fragile (early
           returns reduce it); assert `≤`."

- file: internal/git/tokengate_test.go
  why: "EDIT (append ONE test). The 5 existing closedLoopGate tests at :305-429 cover (a)/(c)/(d) +
        constant-(b) + the effectiveLimit-floor edge. bodySection helper at :27 (builds a diff section
        of KNOWN rune length so EstimateTokens is predictable). tokenBudgetMargin const referenced
        throughout. The file is PURE (no repo/tempDir/I/O) — `package git`, imports fmt/strings/testing."
  pattern: "Each closedLoopGate test: build a section via bodySection, compute firstCut via
            applyWaterFillGate, define a synthetic measure closure (func(gatedDiff string) int), call
            closedLoopGate(..., measure), assert structural properties (==, len, Contains)."
  gotcha: "Do NOT edit/re-add (a)/(c)/(d) — they exist (D1). Append ONLY the growing-drift test. Do NOT
           import internal/prompt (the synthetic measure models it; leaf-purity invariant)."

- file: internal/git/tokens.go
  why: "READ-ONLY. EstimateTokens(s) = ceil(runeCount(s)/4) at :25. The synthetic measures use it
        (prefixOverhead + EstimateTokens(gatedDiff)) to model the real assembled-prompt measurement."
```

### Current Codebase Tree (this task's scope)

```bash
stagecoach/
└── internal/git/
    ├── tokengate.go         # READ-ONLY (closedLoopGate :195, maxClosedLoopPasses :83)
    ├── tokengate_test.go    # EDIT (append TestClosedLoopGate_AdversarialDrift_GrowingMeasure)
    └── tokens.go            # READ-ONLY (EstimateTokens :25)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/git/
    └── tokengate_test.go    # +1 test (TestClosedLoopGate_AdversarialDrift_GrowingMeasure); existing 5 unchanged
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/tokengate_test.go` | MODIFY (append 1 test) | Add `TestClosedLoopGate_AdversarialDrift_GrowingMeasure` (stateful growing + counting measure; asserts terminates + bound + best-attempt). Verify (don't duplicate) (a)/(c)/(d). |

**Explicitly NOT touched**: `internal/git/tokengate.go` (the gate — read-only), `internal/git/tokens.go`
or any other `internal/git/*` production file, the 5 existing closedLoopGate tests (S1's territory —
cross-reference only), `internal/decompose/*` (T2.S2 — parallel), any other package, `docs/*` (P1.M3),
`PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — (a)/(c)/(d) ALREADY EXIST; do NOT duplicate): S1 (P1.M1.T1.S1, Complete) shipped
// closedLoopGate AND its pure unit tests together (tokengate_test.go:305-429). (a) is
// TestClosedLoopGate_OverBudget_RetrimmedToFit (:350); (c) is TestClosedLoopGate_NilMeasure_DelegatesToFirstCut
// (:317); (d) is TestClosedLoopGate_WithinBudget_NoRetrim (:329). All three assert EXACTLY what the contract
// specifies. Re-adding them is duplicate churn that crosses S1's territory. VERIFY by name+line; append ONLY
// the (b) growing-drift variant.

// CRITICAL (G2 — the existing (b) uses a CONSTANT measure; the gap is the GROWING-drift variant):
// TestClosedLoopGate_MaxPassesBound (:385) uses measure = func(_) int { return tokenLimit + 1000 } (CONSTANT
// — invariant to trimming). The contract's (b) explicitly wants "grows with each trim." A growing measure
// exercises a DIFFERENT best-attempt path: trimming worsens the measure ⇒ bestDiff stays the first-cut (the
// constant case never updates bestDiff either, but for a different reason — measured == bestMeasured, not
// measured > bestMeasured). The growing case is the stronger adversarial proof. ADD it; don't assume the
// constant case suffices.

// CRITICAL (G3 — assert the pass-count bound via a COUNTING measure; no existing test does): the measure is
// called once for the first-cut + once per pass ⇒ total calls ≤ maxClosedLoopPasses+1 (=5). No existing test
// asserts this bound (MaxPassesBound asserts termination + len, not call count). The growing-drift test's
// stateful counter asserts `calls ≤ maxClosedLoopPasses+1` — the first explicit bound assertion. Use `≤`
// (NOT `==`): early-return paths (invariant holds) call measure fewer times; only the never-satisfied
// adversarial path hits exactly maxClosedLoopPasses+1.

// CRITICAL (G4 — do NOT call measure(got) in an assertion): the growing-drift measure is STATEFUL (it
// increments `calls` and returns a larger value each call). Calling measure(got) in a post-loop assertion
// would re-increment `calls` (breaking the bound count) and return a value unrepresentative of the loop's
// internal measurement. Assert STRUCTURAL properties instead: `got == firstCut` (the best attempt) and
// `calls ≤ maxClosedLoopPasses+1` (the bound, counted during the loop).

// GOTCHA (G5 — closedLoopGate's measure-call pattern is DETERMINISTIC): the stateful counter is robust
// because the gate calls measure in a fixed order: (1) once for the first-cut (bestMeasured) before the loop,
// (2) once per pass in pass order. There is no concurrency, no reuse of the closure elsewhere. So `calls`
// is a faithful pass counter. (Research §2.)

// GOTCHA (G6 — the best attempt is the FIRST-CUT under a growing measure): because the measure grows each
// call, every pass's `measured` is LARGER than `bestMeasured` (the first-cut's measure) ⇒ `measured <
// bestMeasured` is FALSE ⇒ bestDiff is NEVER updated ⇒ the loop returns the first-cut. Assert `got ==
// firstCut`. (This is the distinguishing property from the constant case, where the first-cut is also kept
// but because measured == bestMeasured, not measured > bestMeasured.)

// GOTCHA (G7 — PURE test; no repo/tempDir/I/O): mirror tokengate_test.go's style. Use bodySection(path,
// bodyRunes) (:27) to build the diff section of known size; define the measure as a func(string) int
// closure; assert structural properties (==, ≤). Do NOT add a t.TempDir, initRepo, or exec.Command — this
// runs in the standard CI unit pass, not the e2e pass (that's T3.S2's job).

// GOTCHA (G8 — imports: the file already has fmt/strings/testing): tokengate_test.go imports fmt, strings,
// testing (and is package git). The new test uses bodySection, applyWaterFillGate, closedLoopGate,
// maxClosedLoopPasses, tokenBudgetMargin — all in-scope via package git. NO new imports needed. (EstimateTokens
// is NOT used by the growing-drift test — the measure is purely stateful; if you prefer a hybrid, it's
// already in scope too.)

// GOTCHA (G9 — scope: ONLY tokengate_test.go): the parallel T2.S2 wires MeasureAssembled closures in
// internal/decompose/{message,planner,decompose}.go — distinct files, no overlap. Do NOT touch tokengate.go
// (the gate — read-only), the existing 5 tests (S1's), or any other package.
```

## Implementation Blueprint

### Data models and structure

None. The test consumes existing symbols: `closedLoopGate`, `applyWaterFillGate`, `maxClosedLoopPasses`,
`tokenBudgetMargin`, `bodySection` (all in `package git`). No new production types, no new helpers.

### The new test (exact — append to `internal/git/tokengate_test.go`)

```go
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
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY the existing coverage (do NOT duplicate)
  - OPEN internal/git/tokengate_test.go; confirm the 5 closedLoopGate tests exist at :305-429:
      TestClosedLoopGate_NilMeasure_DelegatesToFirstCut (:317)        → contract (c)
      TestClosedLoopGate_WithinBudget_NoRetrim (:329)                 → contract (d)
      TestClosedLoopGate_OverBudget_RetrimmedToFit (:350)             → contract (a)
      TestClosedLoopGate_MaxPassesBound (:385)                        → contract (b), CONSTANT measure
      TestClosedLoopGate_EffectiveLimitFloor (:409)                   → edge case
  - CONFIRM (a)/(c)/(d) assert what the contract specifies (they do — see §3 of the research note).
  - DO NOT edit, re-add, or "strengthen" them — they are S1's territory and fully cover their cases. (G1.)

Task 2: ADD TestClosedLoopGate_AdversarialDrift_GrowingMeasure
  - FILE: internal/git/tokengate_test.go ; PACKAGE: git (white-box — same as the existing closedLoopGate tests).
  - PLACE: immediately AFTER TestClosedLoopGate_MaxPassesBound (:385) — they are the two convergence/bound
    tests and belong together (constant vs growing adversarial).
  - WRITE the test verbatim from §"The new test" above. (G2-G8.)
  - DO NOT: add a t.TempDir/initRepo/exec.Command (G7 — PURE); import internal/prompt (synthetic measure
    models it); call measure(got) in an assertion (G4 — stateful re-increment); edit the existing tests (G1).
  - VERIFY: go test -race ./internal/git/ -run TestClosedLoopGate_AdversarialDrift_GrowingMeasure -v → PASS.

Task 3: VALIDATE
  - RUN: gofmt -w internal/git/tokengate_test.go ; gofmt -l .
  - RUN: go vet ./... ; go build ./...
  - RUN: go test -race ./internal/git/ -run TestClosedLoopGate   # all 6 closedLoopGate tests (5 existing + 1 new) green
  - RUN: go test -race ./...                                      # whole repo green (additive test; T2.S2's decompose wiring unaffected)
  - RUN (scope): git diff --stat -- internal/ pkg/ cmd/ docs/ → EXPECT ONLY internal/git/tokengate_test.go.
```

### Implementation Patterns & Key Details

```go
// === Why the growing measure returns the first-cut (G6) ===
// closedLoopGate tracks bestDiff/bestMeasured and updates ONLY when `measured < bestMeasured` (strictly less).
// The growing measure returns a LARGER value each call, so every pass's `measured` > `bestMeasured` (the
// first-cut's measure, which is the smallest value the growing measure ever returns). The strict-less check
// is never true ⇒ bestDiff stays the first-cut ⇒ the loop returns it. This is the distinguishing property
// from the constant case (where measured == bestMeasured, also never strictly less, but for a different reason).

// === Why calls ≤ maxClosedLoopPasses+1 (G3) ===
// closedLoopGate calls measure exactly once before the loop (bestMeasured := measure(firstCut)) and at most
// once per loop pass (the for loop runs pass < maxClosedLoopPasses). So calls ≤ 1 + maxClosedLoopPasses.
// The growing-drift path never hits an early return (the invariant never holds), so it calls exactly
// maxClosedLoopPasses+1 times. Assert `≤` (not `==`) so an early-return path (if the measure ever dipped
// below tokenLimit) doesn't make the test brittle.

// === Why NOT to call measure(got) in the assertion (G4) ===
// The measure is stateful: each call increments `calls` and returns a larger value. Calling measure(got)
// AFTER the loop would (a) increment `calls` past the bound you just asserted, and (b) return a value
// unrepresentative of what the loop internally measured for `got`. Assert structural properties (got ==
// firstCut) instead. The loop's INTERNAL measurement of got is captured in bestMeasured, which the function
// already used to decide the return — you don't need to re-measure.

// === Why this is the ONE genuine gap (G1/G2) ===
// (a)/(c)/(d) are fully covered by S1's tests (with the exact contract assertions). The existing (b) uses a
// constant measure; the contract explicitly wants growing drift. The growing-drift case + the pass-count
// bound (asserted nowhere else) are this task's contribution. Adding more would duplicate S1's territory.
```

### Integration Points

```yaml
TESTS (internal/git/tokengate_test.go — append 1):
  - TestClosedLoopGate_AdversarialDrift_GrowingMeasure (growing + counting measure; terminates + bound + best-attempt)

CONSUMED (READ-ONLY — all in package git):
  - internal/git/tokengate.go: closedLoopGate (:195), applyWaterFillGate (:135), maxClosedLoopPasses (:83)
  - internal/git/tokengate_test.go: bodySection (:27), tokenBudgetMargin (const), the 5 existing tests (verify only)
  - internal/git/tokens.go: EstimateTokens (:25) — available but NOT used by the growing-drift measure (it's purely stateful)

GATE: go test -race ./internal/git/ -run TestClosedLoopGate → ALL 6 green ; git diff --stat → ONLY tokengate_test.go

NO-TOUCH (explicitly — owned by siblings):
  - internal/git/tokengate.go (the gate — read-only), internal/git/tokens.go (EstimateTokens — read-only)
  - the 5 existing closedLoopGate tests (S1's territory — verify, don't edit)
  - internal/decompose/* (T2.S2 — parallel wiring), internal/generate/* / pkg/stagecoach/* / internal/hook/* (T2.S1 message-role sites)
  - docs/* (P1.M3); PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOK (informational — owned by the SIBLING, NOT this task):
  - P1.M1.T3.S2: the E2E test for the assembled-prompt-≤-token_limit invariant (a real repo + the real
    MeasureAssembled closure). This T3.S1 is the PURE unit test (synthetic measures); T3.S2 is the
    integration test (real closure). Distinct — do NOT overlap.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/git/tokengate_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/git/...        # Expected: exit 0
go build ./...                   # Expected: exit 0

# Expected: zero errors. (No new imports needed — the test reuses in-scope package-git symbols.)
```

### Level 2: The New Test + the Existing closedLoopGate Suite

```bash
cd /home/dustin/projects/stagecoach

go test -race ./internal/git/ -v -run TestClosedLoopGate
# Expected: ALL 6 PASS:
#   TestClosedLoopGate_NilMeasure_DelegatesToFirstCut      (contract c — unchanged)
#   TestClosedLoopGate_WithinBudget_NoRetrim               (contract d — unchanged)
#   TestClosedLoopGate_OverBudget_RetrimmedToFit           (contract a — unchanged)
#   TestClosedLoopGate_MaxPassesBound                       (contract b constant — unchanged)
#   TestClosedLoopGate_EffectiveLimitFloor                  (edge — unchanged)
#   TestClosedLoopGate_AdversarialDrift_GrowingMeasure      (contract b growing drift — NEW)
#     asserts: terminates (returns), calls ≤ maxClosedLoopPasses+1, got == firstCut.
```

### Level 3: Whole-Repository Regression + Scope Discipline

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...    # Expected: ALL packages green (one additive test; no production change)
go vet ./...           # Expected: exit 0

# Scope: ONLY tokengate_test.go changed.
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/git/tokengate_test.go only.

# Sibling/production territory UNTOUCHED:
git diff --stat -- internal/git/tokengate.go internal/git/tokens.go internal/decompose/ internal/generate/ pkg/ internal/hook/ docs/
# Expected: EMPTY (the gate is read-only; T2.S2's decompose wiring is its own change, not this task's).
```

### Level 4: Behavioral Cross-Check (manual repro of the bound)

```bash
cd /home/dustin/projects/stagecoach

# The new test IS the proof (it asserts calls ≤ maxClosedLoopPasses+1 with a counting measure). For a
# manual cross-check, add a temporary t.Logf to print `calls` and confirm it equals maxClosedLoopPasses+1
# (5) on the growing-drift path (the never-satisfied adversarial path runs all 4 passes + 1 first-cut):
go test -race ./internal/git/ -v -run TestClosedLoopGate_AdversarialDrift_GrowingMeasure
# (Optional: temporarily add `t.Logf("calls=%d", calls)` and observe calls == 5 == maxClosedLoopPasses+1.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.
- [ ] `go test -race ./internal/git/ -v -run TestClosedLoopGate` — all 6 (5 existing + 1 new) PASS.

### Feature Validation
- [ ] `TestClosedLoopGate_AdversarialDrift_GrowingMeasure` exists, PURE (no repo/tempDir/I/O).
- [ ] Its measure GROWS each call (always > tokenLimit) AND counts calls.
- [ ] It asserts `calls ≤ maxClosedLoopPasses+1` (the pass-count bound — first in the suite).
- [ ] It asserts `got == firstCut` (best attempt when trimming worsens the measure).
- [ ] It passes (the loop terminates — no hang).
- [ ] The existing (a)/(c)/(d) tests remain GREEN and UNCHANGED.

### Scope Discipline Validation
- [ ] ONLY `internal/git/tokengate_test.go` modified (`git diff --stat`).
- [ ] Did NOT edit `tokengate.go` (the gate), `tokens.go`, or any production file.
- [ ] Did NOT edit/re-add the 5 existing closedLoopGate tests (S1's — verified, not duplicated).
- [ ] Did NOT touch `internal/decompose/*` (T2.S2), `internal/generate/*` / `pkg/` / `internal/hook/*` (T2.S1),
      docs (P1.M3), `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation
- [ ] The test mirrors tokengate_test.go's PURE style (bodySection + synthetic measure + structural asserts).
- [ ] The bound assertion uses `≤` (not `==`) so early-return paths don't make it brittle.
- [ ] The measure-call-pattern reasoning (1 first-cut + ≤maxClosedLoopPasses passes) is documented in the test.

---

## Anti-Patterns to Avoid

- ❌ Don't re-add (a)/(c)/(d). S1 shipped closedLoopGate + its tests together; (a)/(c)/(d) are fully covered
  by name at :350/:317/:329 with the exact contract assertions. Duplicating them is churn that crosses S1's
  territory (gotcha G1).
- ❌ Don't assume the existing constant-adversarial test covers (b). The contract explicitly wants "grows
  with each trim" drift — a DISTINCT adversarial pattern (trimming worsens the measure) from the constant
  case (invariant to trimming). The growing case exercises a different best-attempt path and is NOT covered
  (G2).
- ❌ Don't call `measure(got)` in an assertion. The growing measure is STATEFUL — re-calling it increments
  `calls` (breaking the bound you just asserted) and returns a value unrepresentative of the loop's internal
  measurement. Assert `got == firstCut` (structural) instead (G4).
- ❌ Don't assert `calls == maxClosedLoopPasses+1` (exact). Early-return paths (if the measure ever dipped
  below tokenLimit) call measure fewer times. Assert `≤` so the test is robust to the gate's early-exit
  branches (G3).
- ❌ Don't add a repo/tempDir/exec.Command. This is a PURE unit test (the gate is pure string/budget math);
  it runs in the standard CI unit pass. The e2e invariant test is T3.S2's job (G7).
- ❌ Don't import internal/prompt. The synthetic measure closure models the real assembled-prompt measurement
  (it's a func(string) int). Importing internal/prompt would violate the leaf-purity invariant (G7).
- ❌ Don't edit tokengate.go (the gate is read-only), the 5 existing tests (S1's), or internal/decompose/*
  (T2.S2's parallel wiring). This task is ONE additive test in tokengate_test.go (G9).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a single additive PURE unit test in one file, with the verbatim test body provided
(copy-paste-ready), the gate's deterministic measure-call pattern documented (so the `calls ≤
maxClosedLoopPasses+1` bound and the `got == firstCut` best-attempt assertions are provably correct),
and the gap analysis empirically grounded (the 5 existing tests were read in full at :305-429;
(a)/(c)/(d) confirmed covered; the constant-(b) confirmed distinct from the growing-drift (b)). Three
independent de-riskings: (1) the gate under test (closedLoopGate) is LANDED and its body is quoted
verbatim, so the test's claims about call pattern / best-attempt tracking are verified against the
actual code; (2) the test reuses only in-scope package-git symbols (bodySection, applyWaterFillGate,
maxClosedLoopPasses, tokenBudgetMargin) — no new imports, no helpers; (3) the parallel T2.S2 touches
internal/decompose/* only — zero file overlap. The two plausible mistakes — (G1) re-adding the three
existing tests, and (G4) calling the stateful measure in a post-loop assertion (re-incrementing the
counter) — are front-loaded as CRITICAL gotchas. The residual 0.5 uncertainty is the exact arithmetic
of the growing measure (the test should use values comfortably above tokenLimit on every call so the
"never satisfied" premise holds — the chosen `tokenLimit + 100 + calls*50` does, but the implementer
must keep `calls*50` from ever being subtracted into an under-budget value; it monotonically grows, so
it can't). No production-code risk (test-only); no parallel-edit risk (only tokengate_test.go, which no
sibling touches).
