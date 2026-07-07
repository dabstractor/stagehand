---
name: "P1.M1.T1.S2 — Wire closedLoopGate into StagedDiff/TreeDiff/WorkingTreeDiff"
description: |
  Mechanical 3-site wiring in `internal/git/git.go`: replace the direct `applyWaterFillGate(...)` call in
  each diff function's `opts.TokenLimit > 0` branch with `closedLoopGate(..., opts.MeasureAssembled)`.
  S1 landed the `MeasureAssembled` field (git.go:81) + the `closedLoopGate` pure function (tokengate.go:195,
  nil-safe: measure==nil ⇒ delegates to applyWaterFillGate). The 3 sites are byte-identical; the change is
  a rename + add `opts.MeasureAssembled` as the 6th arg. Behavior-preserving for all current callers
  (MeasureAssembled is nil until P1.M1.T2 wires the closures). NOTE: the baseline git suite is currently
  RED on S1's `TestClosedLoopGate_OverBudget_RetrimmedToFit` (a closedLoopGate convergence bug in
  tokengate.go — S1's territory, NOT S2's); S2 does not touch tokengate.go and neither causes nor fixes
  that failure. S2's gate = `go build ./...` green + the existing diff-function tests green. No new tests
  (behavior-preservation is proven by existing tests; the invariant/e2e tests are P1.M1.T3's job).
---

## Goal

**Feature Goal**: Wire the FR3j closed-loop gate into the three diff-producing functions (`StagedDiff`,
`TreeDiff`, `WorkingTreeDiff`) so that — once a consumer injects a `MeasureAssembled` closure (P1.M1.T2) —
the gated diff is re-measured against the *actual* assembled prompt and re-trimmed until it fits
`token_limit`. S1 provided the `closedLoopGate` function and the `MeasureAssembled` seam; S2 is the
3-call-site wiring that makes the diff functions actually call it.

**Deliverable**: Three identical edits in `internal/git/git.go` — each `>0` branch's
`b.WriteString(applyWaterFillGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens))`
becomes `gatedBody := closedLoopGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens, opts.MeasureAssembled); b.WriteString(gatedBody)`.
No other file changes.

**Success Definition**: The three diff functions call `closedLoopGate` (passing `opts.MeasureAssembled`);
when `MeasureAssembled == nil` (every current caller), output is byte-identical to today (closedLoopGate's
nil-safe path delegates to applyWaterFillGate); `go build ./...` is green; the existing diff-function tests
(`TestStagedDiff_*` / `TestTreeDiff_*` / `TestWorkingTreeDiff_*`) pass; `grep` confirms exactly 3
`closedLoopGate(` sites and zero direct `applyWaterFillGate(` calls remain in the diff functions.

## User Persona

**Target User**: The contributor wiring P1.M1.T2 (the 6 `MeasureAssembled` closures at the consumer call
sites) and P1.M1.T3 (the closed-loop invariant tests), and the end user who sets `token_limit` and relies
on the FR3j "never delivered over budget" guarantee.

**Use Case**: A user sets `token_limit = 120000`. The message-role diff is captured, gated by the water-fill
(FR3i), then — once T2 wires the closure — `closedLoopGate` assembles the real prompt (sysPrompt + payload),
measures it, and re-trims if it landed over 120000. S2 is the wiring that makes the diff functions invoke
that loop.

**Pain Points Addressed**: After S1, the `closedLoopGate` function exists but the three diff functions still
call the open-loop `applyWaterFillGate` directly — the closed-loop guarantee is dead code until S2 wires it.

## Why

- **Closes the S1 wiring gap.** S1 landed the field + the pure function; the three diff functions are the
  only callers of `applyWaterFillGate` in the `>0` path, and they're the sites FR3j specifies the loop lives
  (only there are `mdDiffs`/`nmDiff`/`skeleton` in scope). S2 is the literal 3-site call swap.
- **Pure mechanical change, behavior-preserving.** `closedLoopGate`'s nil-measure path is `applyWaterFillGate`
  with no loop (S1's contract). Every current caller leaves `MeasureAssembled == nil` (closures land in
  P1.M1.T2). So S2 changes nothing observable until T2 — zero risk to existing behavior.
- **FR3j's loop MUST live here.** The closed loop re-runs the water-fill at a reduced budget; it needs the
  raw `mdDiffs`/`nmDiff`/`skeleton` (NOT the already-concatenated body) so each re-cut preserves FR3i's
  per-file fairness. These locals exist only inside each diff function — so the `closedLoopGate` call must
  be at these 3 sites, not downstream.

## What

Three byte-for-byte identical edits: in each diff function's `opts.TokenLimit > 0` branch, rename the
`applyWaterFillGate` call to `closedLoopGate` and append `opts.MeasureAssembled` as the 6th argument;
capture the result in `gatedBody` then `b.WriteString(gatedBody)`. Update the inline comment to cite FR3j.
No new tests (behavior-preservation is proven by the existing diff-function tests; the closed-loop
invariant + e2e tests are P1.M1.T3's job).

### Success Criteria

- [ ] `StagedDiff` `>0` branch (git.go:~1026) calls `closedLoopGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens, opts.MeasureAssembled)`.
- [ ] `TreeDiff` `>0` branch (git.go:~1524) calls `closedLoopGate(...)` (identical).
- [ ] `WorkingTreeDiff` `>0` branch (git.go:~1697) calls `closedLoopGate(...)` (identical).
- [ ] Each site retains `mdDiffs`/`nmDiff`/`skeleton` as the args (NOT a re-cut of the assembled body).
- [ ] `go build ./...` green; existing `TestStagedDiff_*`/`TestTreeDiff_*`/`TestWorkingTreeDiff_*` pass (byte-identical output under nil MeasureAssembled).
- [ ] `grep -n "applyWaterFillGate(" internal/git/git.go` → ZERO matches (all 3 swapped to closedLoopGate).
- [ ] `grep -n "closedLoopGate(" internal/git/git.go` → exactly 3 matches (the 3 diff sites).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact current code at each of the 3 byte-identical sites, the
exact `closedLoopGate` signature (tokengate.go:195 = `applyWaterFillGate`'s args + a trailing `measure
func(string) int`), the verbatim replacement, the in-scope locals, the nil-safe behavior-preservation
guarantee, and the baseline caveat (S1's closedLoopGate convergence test is currently red — S1's bug, not
S2's). The change is a rename + 1 added arg, repeated 3×.

### Documentation & References

```yaml
# MUST READ — the closed-loop design + the 3 sites
- docfile: plan/011_98cef660a41d/architecture/fr3j_closed_loop.md
  why: "The authoritative design. Documents that the closed loop MUST live INSIDE the >0 branches of the 3 diff functions (only there are mdDiffs/nmDiff/skeleton in scope), the nil-safe semantics (measure==nil OR TokenLimit==0 → first-cut only), and that the loop DELEGATES to applyWaterFillGate (preserving FR3i water-fill fairness across every pass)."
  critical: "The 3 sites are the ONLY place the closed loop can live — the raw mdDiffs/nmDiff/skeleton locals are needed so each re-cut re-applies the per-file water-fill (flatly re-cutting the assembled body would lose FR3i fairness)."

- docfile: plan/011_98cef660a41d/P1M1T1S1/PRP.md
  why: "S1's contract: the MeasureAssembled field (git.go:81) + the closedLoopGate pure function (tokengate.go:195). Gives the exact signature: `closedLoopGate(mdDiffs, nmDiff, skeleton, tokenLimit, promptReserve, measure)`. Confirms nil-safe: measure==nil ⇒ applyWaterFillGate, no loop."
  critical: "S1 is LANDED (verified: field at git.go:81, function at tokengate.go:195). S2 consumes them as-is — it does NOT re-edit tokengate.go. S1's nil-safe contract is what makes S2 behavior-preserving for all current (nil-MeasureAssembled) callers."

- docfile: plan/011_98cef660a41d/P1M1T1S2/research/s2_implementation_notes.md
  why: "Distilled S2 findings: the 3 byte-identical sites, the exact replacement, the in-scope locals, the nil-safe behavior-preservation proof, the baseline caveat (S1's closedLoopGate convergence test is red — S1's territory), and S2's gate (build green + existing diff tests green, NOT full-suite-green which depends on S1)."

# The file under edit
- file: internal/git/git.go
  why: "EDIT (3 spots). The >0 branches of StagedDiff (~1026), TreeDiff (~1524), WorkingTreeDiff (~1697). Each is byte-identical: `b.WriteString(applyWaterFillGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens))` + `return b.String(), nil`."
  pattern: "Rename applyWaterFillGate → closedLoopGate; append `opts.MeasureAssembled` as the 6th arg; capture in `gatedBody` then WriteString. Update the inline comment FR3d/FR3i → FR3d/FR3i/FR3j."
  gotcha: "RETAIN mdDiffs/nmDiff/skeleton as the args — do NOT replace them with the already-concatenated body. The closed loop re-runs applyWaterFillGate at a reduced budget on each pass; it needs the RAW sections to preserve FR3i per-file water-fill fairness (flatly re-cutting the assembled string would lose it). The locals are already in scope (declared earlier in each function)."

# Read-only refs (do NOT edit in S2)
- file: internal/git/tokengate.go   # S1 LANDED: closedLoopGate (:195) + applyWaterFillGate (:135) + constants
  why: "closedLoopGate's signature and nil-safe contract. No edit — S1 owns it (and must fix its convergence test)."
- file: internal/git/git.go (StagedDiffOptions.MeasureAssembled :81)   # S1 LANDED
  why: "The field S2 passes through (opts.MeasureAssembled). No edit."

# PRD authority (already in the selected content)
- prd: PRD.md §9.1 FR3j (closed-loop budget guarantee: assemble → measure → re-trim if over, bounded) + FR3i (water-fill the loop re-applies each pass).
  why: "FR3j is WHY the loop exists; FR3i is WHY it must reuse the raw mdDiffs/nmDiff/skeleton (fairness)."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/git/
    ├── git.go           # EDIT (3 spots): the >0 branches of StagedDiff/TreeDiff/WorkingTreeDiff
    ├── tokengate.go     # READ-ONLY (S1 landed: closedLoopGate + applyWaterFillGate + constants)
    └── tokengate_test.go # READ-ONLY (S1 — owns the closedLoopGate unit tests)
```

### Desired Codebase Tree After S2

```bash
stagecoach/
└── (only git.go modified — 3 spots, no new files)
    internal/git/git.go   # 3 sites: applyWaterFillGate(...) → closedLoopGate(..., opts.MeasureAssembled)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY | 3 `>0` branches call `closedLoopGate` (passing `opts.MeasureAssembled`). **Only file touched.** |

**Explicitly NOT touched**: `tokengate.go` / `tokengate_test.go` (S1 — closedLoopGate + its tests; S1 must
fix the convergence bug), the 6 consumer closure wirings (P1.M1.T2.S1/S2), the closed-loop invariant/e2e
tests (P1.M1.T3.S1/S2), any other package, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (retain the raw sections): pass mdDiffs/nmDiff/skeleton to closedLoopGate — NOT the assembled
// body. The closed loop re-runs applyWaterFillGate at a reduced budget on each pass; it needs the RAW
// per-file sections to preserve FR3i's per-file water-fill fairness. Flatly re-cutting the already-
// concatenated body would lose the fairness guarantee. The locals are already in scope at each site.

// CRITICAL (nil-safe = behavior-preserving): closedLoopGate's measure==nil path returns applyWaterFillGate(...)
// with NO loop (S1's contract). Every current caller leaves opts.MeasureAssembled == nil (closures land in
// P1.M1.T2). So after S2, all existing diff-function tests produce byte-identical output — the wiring is a
// pure no-op until T2 injects a closure. This is why S2 is safe and adds no new tests.

// CRITICAL (baseline caveat — S1's bug, not S2's): `go test ./internal/git/` is CURRENTLY RED on S1's
// TestClosedLoopGate_OverBudget_RetrimmedToFit (tokengate_test.go:365 — a closedLoopGate convergence
// defect: measure(got)=2962 > tokenLimit=2524 after re-trim). This is in tokengate.go/tokengate_test.go
// (S1's territory). S2 does NOT touch tokengate.go and neither causes nor fixes that failure. S2's gate is
// `go build ./...` green + the existing diff-function tests green (run by name to skip S1's failing test).
// The full suite goes green once S1 fixes the convergence bug.

// GOTCHA (signature lines up exactly): closedLoopGate is applyWaterFillGate's signature + a trailing
// `measure func(string) int`. The current call's 5 args are unchanged; just append opts.MeasureAssembled
// as the 6th. No reordering, no type change.

// GOTCHA (3 identical sites): the three >0 branches are byte-for-byte identical (verified). Apply the SAME
// edit at all three. Do not diverge them (e.g. don't add the closed loop to one and not another).
```

## Implementation Blueprint

### Data models and structure

No schema change — S1 landed `closedLoopGate` (tokengate.go:195) and `MeasureAssembled` (git.go:81). S2 is
a 3-site call swap. The relevant existing function signature (unchanged):

```go
// internal/git/tokengate.go (S1 LANDED — the function S2 calls)
func closedLoopGate(mdDiffs []string, nmDiff, skeleton string, tokenLimit, promptReserve int,
    measure func(string) int) string   // measure==nil ⇒ applyWaterFillGate, no loop (nil-safe)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: git.go StagedDiff — swap applyWaterFillGate → closedLoopGate
  - LOCATE: the `if opts.TokenLimit > 0 {` branch in StagedDiff (~git.go:1023-1027).
  - REPLACE:
        b.WriteString(applyWaterFillGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens))
        return b.String(), nil
    WITH:
        // FR3d/FR3i/FR3j: closedLoopGate does the first-cut water-fill (applyWaterFillGate) AND, when
        // opts.MeasureAssembled is non-nil, re-measures the ASSEMBLED prompt and re-trims if over
        // tokenLimit (FR3j hard guarantee). nil MeasureAssembled ⇒ first-cut only (behavior unchanged).
        gatedBody := closedLoopGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens, opts.MeasureAssembled)
        b.WriteString(gatedBody)
        return b.String(), nil
  - RETAIN mdDiffs/nmDiff/skeleton as the args (they are in-scope locals).

Task 2: git.go TreeDiff — identical swap (~git.go:1521-1525)
  - Same edit as Task 1, in TreeDiff's >0 branch.

Task 3: git.go WorkingTreeDiff — identical swap (~git.go:1694-1698)
  - Same edit as Task 1, in WorkingTreeDiff's >0 branch.

Task 4: VALIDATE (S2's gate — build + existing diff tests green; full-suite greenness depends on S1)
  - RUN: gofmt -w internal/git/git.go
  - RUN: go build ./...        # green (closedLoopGate exists; signature matches)
  - RUN: go test -race ./internal/git/ -run 'TestStagedDiff|TestTreeDiff|TestWorkingTreeDiff'   # green (behavior-preserving)
  - RUN: go vet ./...
  - GREP: confirm exactly 3 closedLoopGate sites, zero applyWaterFillGate direct calls in the diff funcs:
        grep -n "closedLoopGate(" internal/git/git.go        # → 3 matches
        grep -n "applyWaterFillGate(" internal/git/git.go    # → ZERO (all swapped)
  - NOTE: `go test ./internal/git/` (full) is red until S1 fixes TestClosedLoopGate_OverBudget_RetrimmedToFit
    (tokengate.go convergence bug — S1's territory). S2 does not touch that file.
  - FIX-FORWARD: if a diff-function test fails, the swap diverged (e.g. wrong arg order) — re-check the signature.
```

### Implementation Patterns & Key Details

```go
// === git.go — the SAME edit at all 3 sites (StagedDiff / TreeDiff / WorkingTreeDiff >0 branch) ===
// BEFORE (current, byte-identical at :1026 / :1524 / :1697):
	if opts.TokenLimit > 0 {
		// FR3d/FR3i: the gate emits md + non-md (BOTH) under ONE shared water-fill budget; it returns the
		// recomposed body (skeleton was already written above). The legacy byte cap is SUPPERSeded.
		b.WriteString(applyWaterFillGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens))
		return b.String(), nil
	}

// AFTER:
	if opts.TokenLimit > 0 {
		// FR3d/FR3i/FR3j: closedLoopGate does the first-cut water-fill (applyWaterFillGate) AND, when
		// opts.MeasureAssembled is non-nil, re-measures the ASSEMBLED prompt and re-trims if over
		// tokenLimit (FR3j hard guarantee). nil MeasureAssembled ⇒ first-cut only (behavior unchanged).
		gatedBody := closedLoopGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens, opts.MeasureAssembled)
		b.WriteString(gatedBody)
		return b.String(), nil
	}
```

### Integration Points

```yaml
DIFF FUNCTIONS (internal/git/git.go):
  - StagedDiff >0 branch (~1026): applyWaterFillGate → closedLoopGate(..., opts.MeasureAssembled)
  - TreeDiff >0 branch (~1524): same
  - WorkingTreeDiff >0 branch (~1697): same

CONSUMED (read-only — S1 landed):
  - tokengate.go closedLoopGate (:195) — the function S2 calls (nil-safe)
  - git.go StagedDiffOptions.MeasureAssembled (:81) — the field S2 passes through

NO-TOUCH (explicitly — owned by sibling/later subtasks):
  - internal/git/tokengate.go / tokengate_test.go   # S1: closedLoopGate + tests (S1 must fix the convergence bug)
  - the 6 MeasureAssembled closure wirings           # P1.M1.T2.S1/S2 (generate/pkg/hook + decompose roles)
  - closed-loop invariant/e2e tests                  # P1.M1.T3.S1/S2
  - any other package; PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S2):
  - P1.M1.T2.S1/S2: wire the MeasureAssembled closures at the 6 consumer sites → activates the closed loop.
  - P1.M1.T3.S1/S2: pure invariant tests + e2e "assembled ≤ token_limit" test.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/git/git.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/git/...        # Expected: exit 0
go build ./...                   # Expected: exit 0 (closedLoopGate exists; signature matches; 3 sites compile)
```

### Level 2: Existing Diff-Function Tests (behavior-preservation proof)

```bash
cd /home/dustin/projects/stagecoach

# The 3 diff functions' tests — prove byte-identical output under nil MeasureAssembled (every current caller).
go test -race ./internal/git/ -v -run 'TestStagedDiff|TestTreeDiff|TestWorkingTreeDiff'

# Expected: ALL PASS. closedLoopGate's nil-measure path == applyWaterFillGate (S1's contract), so the
# existing golden/byte-count assertions are unchanged.
```

### Level 3: The Wiring Gate (grep — the 3 sites swapped)

```bash
cd /home/dustin/projects/stagecoach

grep -n "closedLoopGate(" internal/git/git.go        # Expected: exactly 3 matches (the 3 diff sites)
grep -n "applyWaterFillGate(" internal/git/git.go    # Expected: ZERO (all 3 swapped to closedLoopGate)

# Confirm S2 touched only git.go
git diff --stat -- internal/git/
# Expected: only internal/git/git.go. (tokengate.go/tokengate_test.go are S1's — NOT touched by S2.)
```

### Level 4: Scope + Baseline-Caveat Check

```bash
cd /home/dustin/projects/stagecoach

# S2 does NOT touch tokengate.go (S1's territory — owns the closedLoopGate convergence bug)
git diff --stat -- internal/git/tokengate.go internal/git/tokengate_test.go   # Expected: EMPTY (S2 untouched)

# Baseline caveat: the full git suite is red on S1's TestClosedLoopGate_OverBudget_RetrimmedToFit until S1
# fixes the convergence defect. S2 neither causes nor fixes it. Confirm S2's own contribution is clean:
go test ./internal/git/ 2>&1 | grep -E "FAIL.*StagedDiff|FAIL.*TreeDiff|FAIL.*WorkingTreeDiff" || echo "OK: no diff-function test fails (S2 is behavior-preserving)"
# (The only failure is TestClosedLoopGate_OverBudget_RetrimmedToFit — S1's, in tokengate_test.go.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `TestStagedDiff*` / `TestTreeDiff*` / `TestWorkingTreeDiff*` pass (behavior-preserving under nil MeasureAssembled).

### Feature Validation

- [ ] `StagedDiff` `>0` branch calls `closedLoopGate(..., opts.MeasureAssembled)`.
- [ ] `TreeDiff` `>0` branch calls `closedLoopGate(..., opts.MeasureAssembled)`.
- [ ] `WorkingTreeDiff` `>0` branch calls `closedLoopGate(..., opts.MeasureAssembled)`.
- [ ] Each site retains `mdDiffs`/`nmDiff`/`skeleton` as the args (raw sections, not the assembled body).
- [ ] `grep "closedLoopGate(" internal/git/git.go` → 3 matches; `grep "applyWaterFillGate(" internal/git/git.go` → 0.

### Scope Discipline Validation

- [ ] ONLY `internal/git/git.go` modified by S2 (git diff --stat confirms).
- [ ] Did NOT edit `tokengate.go`/`tokengate_test.go` (S1 — owns closedLoopGate + the convergence bug).
- [ ] Did NOT wire the 6 MeasureAssembled closures (P1.M1.T2) or add invariant/e2e tests (P1.M1.T3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] The 3 edits are byte-for-byte identical (no divergence between the diff functions).
- [ ] The inline comment cites FR3d/FR3i/FR3j (the closed loop is FR3j).
- [ ] The raw sections (mdDiffs/nmDiff/skeleton) are retained (FR3i fairness preserved across re-trim passes).

---

## Anti-Patterns to Avoid

- ❌ Don't pass the assembled body to closedLoopGate — pass the RAW `mdDiffs`/`nmDiff`/`skeleton`. The closed
  loop re-runs `applyWaterFillGate` at a reduced budget each pass; it needs the per-file sections to preserve
  FR3i water-fill fairness. Re-cutting the concatenated body would lose it.
- ❌ Don't edit `tokengate.go` or `tokengate_test.go` — that's S1's territory (closedLoopGate + its tests).
  The baseline `TestClosedLoopGate_OverBudget_RetrimmedToFit` failure is S1's convergence bug to fix, not
  S2's. S2 is the 3-site wiring in git.go only.
- ❌ Don't diverge the 3 sites — they are byte-identical today and must receive the identical edit. Don't add
  the closed loop to one diff function and not another.
- ❌ Don't reorder or retype the args — `closedLoopGate` is `applyWaterFillGate`'s signature + a trailing
  `measure func(string) int`. Keep the existing 5 args in order; append `opts.MeasureAssembled` as the 6th.
- ❌ Don't add new tests — S2 is behavior-preserving (proven by the existing diff-function tests under nil
  MeasureAssembled). The closed-loop invariant + e2e tests are P1.M1.T3's job; the closure wiring is
  P1.M1.T2's. Adding a "wired" test here crosses the boundary.
- ❌ Don't gate S2 on the full git suite being green — that depends on S1 fixing closedLoopGate's convergence
  bug. S2's gate is `go build ./...` green + the existing diff-function tests green + the grep wiring check.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a mechanical, byte-for-byte-identical 3-site call swap (rename `applyWaterFillGate` →
`closedLoopGate` + append `opts.MeasureAssembled` as the 6th arg) in `git.go` only. Four independent
de-riskings: (1) S1 is CONFIRMED landed — `closedLoopGate` exists at tokengate.go:195 with the exact
signature (`applyWaterFillGate`'s args + trailing `measure func(string) int`), and `MeasureAssembled` exists
at git.go:81; (2) the three sites are verified byte-identical (sed) and the locals (`mdDiffs`/`nmDiff`/
`skeleton`) are confirmed in scope; (3) the nil-safe contract makes the change behavior-preserving for every
current caller (MeasureAssembled is nil until P1.M1.T2) — so the existing diff-function tests stay green;
(4) the grep gates (3 closedLoopGate sites, 0 applyWaterFillGate direct calls) deterministically confirm
the wiring. The one residual uncertainty (not 10/10) is OUTSIDE S2's control: the baseline git suite is
currently red on S1's `TestClosedLoopGate_OverBudget_RetrimmedToFit` (a closedLoopGate convergence bug in
tokengate.go — S1's territory). S2 neither causes nor fixes it; S2's gate is build-green + diff-tests-green
+ grep-wiring, all of which are deterministic and independent of S1's bug. The closure wiring (P1.M1.T2)
and invariant tests (P1.M1.T3) are cleanly fenced and untouched.
