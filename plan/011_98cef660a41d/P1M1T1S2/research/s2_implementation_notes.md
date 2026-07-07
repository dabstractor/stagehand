# S2 Implementation Notes — Wire closedLoopGate into StagedDiff/TreeDiff/WorkingTreeDiff

> Scope: P1.M1.T1.S2 — a mechanical 3-site change in `internal/git/git.go`: replace the direct
> `applyWaterFillGate(...)` call in each diff function's `>0` branch with `closedLoopGate(..., opts.MeasureAssembled)`.
> **S1 is landed** (MeasureAssembled field git.go:81 + closedLoopGate tokengate.go:195). Verified 2026-07-06.

## 0. S1 state + baseline (confirmed)

- `StagedDiffOptions.MeasureAssembled func(gatedDiff string) int` EXISTS (git.go:81, nil-safe). **S1 landed.**
- `closedLoopGate(mdDiffs []string, nmDiff, skeleton string, tokenLimit, promptReserve int, measure func(string) int) string`
  EXISTS (tokengate.go:195). Its signature is `applyWaterFillGate`'s + a trailing `measure func(string) int`.
  When `measure == nil` it delegates to `applyWaterFillGate` (no loop — behavior unchanged). **S1 landed.**
- The 3 diff sites STILL call `applyWaterFillGate` directly (git.go:1026, 1524, 1697) → S2's gap.

### ⚠️ Baseline is CURRENTLY RED — but it's S1's bug, NOT S2's
`go test ./internal/git/` FAILS on `TestClosedLoopGate_OverBudget_RetrimmedToFit` (tokengate_test.go:365):
"measure(got)=2962 > tokenLimit=2524" — S1's `closedLoopGate` has a convergence defect (the re-trim doesn't
fully reduce the assembled prompt under tokenLimit). This is entirely in `tokengate.go`/`tokengate_test.go`
(S1's territory). **S2 does NOT touch tokengate.go** — S2's 3-site wiring is independent of the convergence
bug. S2's own gate is `go build ./...` green + the existing diff-function tests green (the wiring is
behavior-preserving when MeasureAssembled==nil). The full `go test ./internal/git/` goes green once S1 fixes
the convergence bug; S2 neither causes nor fixes that failure.

## 1. The 3 sites — IDENTICAL structure, mechanical change

All three `>0` branches are byte-identical (verified by sed):
```go
if opts.TokenLimit > 0 {
    // FR3d/FR3i: the gate emits md + non-md (BOTH) under ONE shared water-fill budget; it returns the
    // recomposed body (skeleton was already written above). The legacy byte cap is SUPPERSeded.
    b.WriteString(applyWaterFillGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens))
    return b.String(), nil
}
```
- StagedDiff: git.go:1026
- TreeDiff: git.go:1524
- WorkingTreeDiff: git.go:1697

The locals `mdDiffs` (declared `var mdDiffs []string` at 970/1471/...), `nmDiff`, `skeleton` are all in
scope at each site (they're already the applyWaterFillGate args). The change at each site:
```go
// FR3d/FR3i/FR3j: closedLoopGate does the first-cut water-fill (applyWaterFillGate) AND, when
// opts.MeasureAssembled is provided, re-measures the ASSEMBLED prompt and re-trims if over tokenLimit
// (FR3j hard guarantee). nil MeasureAssembled ⇒ first-cut only (behavior unchanged).
gatedBody := closedLoopGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens, opts.MeasureAssembled)
b.WriteString(gatedBody)
return b.String(), nil
```

## 2. Why this is safe + behavior-preserving

- `closedLoopGate`'s `measure == nil` path returns `applyWaterFillGate(...)` with NO loop (S1's nil-safe
  contract). All current callers leave `opts.MeasureAssembled == nil` (the closures are wired in P1.M1.T2,
  a later subtask). So after S2, every existing diff-function test (TestStagedDiff_*, TestTreeDiff_*,
  TestWorkingTreeDiff_*) produces byte-identical output to today — the wiring is a pure no-op until T2
  injects a closure.
- The signature lines up exactly: `closedLoopGate(mdDiffs, nmDiff, skeleton, tokenLimit, promptReserve,
  measure)` — the current call's 5 args `(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens)`
  + the new 6th `opts.MeasureAssembled`. No reordering, no type change.

## 3. What S2 does NOT do

- NOT `tokengate.go` / `tokengate_test.go` (S1 — owns closedLoopGate + its tests; S1 must fix the
  convergence bug for the full suite to go green).
- NOT the 6 consumer closure wirings (P1.M1.T2.S1/S2 — generate/pkg/hook + decompose planner/message/arbiter).
- NOT the closed-loop invariant tests / e2e (P1.M1.T3.S1/S2).
- NOT any skeleton/md-capture/nm-capture logic — the locals are RETAINED unchanged (mdDiffs/nmDiff/skeleton
  stay in scope so the re-run preserves FR3i water-fill fairness; flatly re-cutting the gated string would
  lose it — the contract is explicit on this).
- NOT PRD.md / tasks.json / prd_snapshot.md / plan/*.

## 4. The gate for S2

S2 is a mechanical wiring change. Its gate is NOT "full git suite green" (that depends on S1 fixing
closedLoopGate's convergence bug, which is S1's responsibility). S2's gate:
- `go build ./...` green (closedLoopGate exists; signature matches; the 3 sites compile).
- The existing diff-function tests green (TestStagedDiff_*, TestTreeDiff_*, TestWorkingTreeDiff_* — prove
  behavior-preservation under nil MeasureAssembled). Run them by name to skip S1's failing closedLoopGate test.
- `grep` confirms exactly 3 `closedLoopGate(` call sites in git.go and ZERO remaining `applyWaterFillGate(`
  direct calls in the diff functions.

## 5. Sources

- `architecture/fr3j_closed_loop.md` (the closed-loop design; the 3 diff-function gate sites).
- `P1M1T1S1/PRP.md` (S1 — MeasureAssembled field + closedLoopGate pure function; the nil-safe contract).
- PRD §9.1 FR3j (closed-loop budget guarantee); FR3i (water-fill the closed loop re-applies).
- `internal/git/git.go` (the 3 sites :1026/:1524/:1697) + `tokengate.go` (closedLoopGate :195).
