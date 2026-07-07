# FR3j Closed-Loop Token-Budget Guarantee — Architecture

## The Open-Loop Problem (current state)

`applyWaterFillGate` (internal/git/tokengate.go:121) computes body budget:

```
bodyBudget = tokenLimit − skeletonTokens − promptReserve − tokenBudgetMargin
```

Where:
- `skeletonTokens = EstimateTokens(skeleton)` — measured from the actual numstat skeleton (accurate)
- `promptReserve` — a WORST-CASE estimate via the empty-diff trick (`MessageReserveTokens`/`PlannerReserveTokens`/`ArbiterReserveTokens` in prompt/reserve.go). This measures `est(sysPrompt) + est(BuildUserPayload("", context, worstRejected)) + reserveSafetyMargin(256)`.
- `tokenBudgetMargin = 1024` — deterministic safety buffer

**Drift sources that let the assembled prompt exceed token_limit:**
1. `chars/4` estimator vs real token density (code ~3-4 chars/token, prose ~4-5)
2. Worst-case rejection block (reserve's ceiling) vs the actual attempt's framing
3. Skeleton measured in isolation from its final placement in the builder

## The Closed-Loop Solution (FR3j)

After the first-cut `applyWaterFillGate` produces the gated diff, the closed loop:

1. Calls `MeasureAssembled(gatedDiff)` — an injected callback that returns `EstimateTokens(sysPrompt + role_payload_builder(gatedDiff))`
2. If the result ≤ `tokenLimit`: done (the invariant holds)
3. If over: reduce `effectiveLimit = tokenLimit - (assembled - tokenLimit) - slack`, re-run `applyWaterFillGate(mdDiffs, nmDiff, skeleton, effectiveLimit, promptReserve)` at the reduced limit
4. Repeat (bounded ~4 passes — the estimate is already close, converges in 1-2 passes)

**Invariant:** `EstimateTokens(assembledFullPrompt) ≤ token_limit`, always. Going under is fine.

## The Injection Seam

`internal/git` cannot import `internal/prompt` (leaf-purity invariant). The `MeasureAssembled`
callback follows `reserve.go`'s existing `TokenEstimator` injection pattern in reverse:

```go
// New field on git.StagedDiffOptions (git.go:38):
MeasureAssembled func(gatedDiff string) int
```

Each consumer provides a closure capturing `sysPrompt` + role-specific context:
```go
MeasureAssembled: func(gatedDiff string) int {
    return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, cfg.Context, nil))
},
```

**nil-safe:** `MeasureAssembled == nil` OR `TokenLimit == 0` → behave exactly as today (first-cut only).

## The 3 Diff Functions (where the loop lives)

The closed-loop loop must live INSIDE the `>0` branches of the 3 diff functions because only
there are `mdDiffs []string`, `nmDiff string`, and `skeleton string` in scope (retaining the
raw sections is critical — re-running the gate over them preserves FR3i water-fill fairness;
flatly re-cutting the gated string would lose it).

| Function | File | Gate call line |
|---|---|---|
| `StagedDiff` | git.go | ~1018 |
| `TreeDiff` | git.go | ~1516 |
| `WorkingTreeDiff` | git.go | ~1689 |

Each currently: `b.WriteString(applyWaterFillGate(...)); return b.String(), nil`

After FR3j: run the closed-loop wrapper that re-measures and re-trims.

## The 6 Consumer Wiring Sites

| # | Site | File | Role | Builder |
|---|---|---|---|---|
| 1 | `CommitStaged` | generate.go:~213 | message | `BuildUserPayload` |
| 2 | `runPipeline` | stagecoach.go:~437 | message | `BuildUserPayload` |
| 3 | `hook.Run` | hook/exec.go:~123 | message | `BuildUserPayload` |
| 4 | `generateMessage` | decompose/message.go:~88 | message (decompose) | `BuildUserPayload` |
| 5 | `callPlanner` | decompose/planner.go:~79 | planner | `BuildPlannerUserPayload` |
| 6 | `runArbiterPhase` | decompose.go:~663 | arbiter | `BuildArbiterUserPayload` |

Each closure captures: `sysPrompt` (built before the diff), `context`, and `rejected`/`forcedCount`/`commits`
as appropriate. Uses `git.EstimateTokens` (single-estimator rule).

## Out of Scope

- **Multi-turn (FR-T12):** deliberately re-captures diff with `TokenLimit=0`; lossless delivery
  supersedes the per-request budget. Do NOT thread `MeasureAssembled` into the multi-turn re-capture.
- **Stager role:** no diff to truncate (tooled index-mutator). Does not route through the gate.
- **`token_limit == 0` path:** legacy per-section caps apply. No closed-loop needed (no holistic budget).
