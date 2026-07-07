# Research: Wire MeasureAssembled at the 3 decompose-role TreeDiff sites

> **Purpose:** Pin the exact edits for P1.M1.T2.S2 — wiring the FR3j closed-loop `MeasureAssembled`
> closure at the 3 decompose-role `TreeDiff` consumer sites (message, planner, arbiter). Mirrors the
> S1 (P1.M1.T2.S1) pattern at the message-role `StagedDiff` sites, but with each role's sysPrompt
> variable + role-specific payload builder. Built on T1.S1 (LANDED: `MeasureAssembled` field) +
> T1.S2 (LANDED: `closedLoopGate` wired into the 3 diff functions, nil-safe). All line numbers live on 2026-07-07.

---

## 1. Baseline state (verified)

### 1.1 T1.S1 + T1.S2 LANDED — the field + the gate exist
- `StagedDiffOptions.MeasureAssembled func(gatedDiff string) int` — **git.go:81** (T1.S1 LANDED).
- `closedLoopGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens, opts.MeasureAssembled)`
  is called inside StagedDiff/TreeDiff/WorkingTreeDiff (git.go:1027/1527/1702 — T1.S2 LANDED).
- **Nil-safe:** `MeasureAssembled == nil` OR `TokenLimit == 0` → first-cut water-fill only (legacy path,
  byte-identical). So wiring a nil closure when `TokenLimit==0` changes nothing.

### 1.2 S1 (P1.M1.T2.S1, parallel) — the pattern to mirror
S1 wires the 3 message-role `StagedDiff` sites (generate.go, stagecoach.go, exec.go) with:
```go
var measureAssembled func(string) int
if cfg.TokenLimit != 0 {
    measureAssembled = func(gatedDiff string) int {
        return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, cfg.Context, nil))
    }
}
// ... StagedDiffOptions{ ..., MeasureAssembled: measureAssembled }
```
This task mirrors that at the 3 decompose-role `TreeDiff` sites — same shape, role-specific sysPrompt +
payload builder, and `deps.Config` (not `cfg`).

### 1.3 No MeasureAssembled at decompose sites yet — genuine add
`grep MeasureAssembled internal/decompose/{message,planner,decompose}.go` → no matches. Net-new wiring.

### 1.4 No multi-turn re-capture TreeDiff calls in decompose (simpler than S1)
Unlike the message-role sites (which each have a 2nd multi-turn re-capture `StagedDiff` with
`TokenLimit: 0` that S1 must AVOID), each decompose role has **exactly ONE `TreeDiff` call**. Confirmed:
message.go:91, planner.go:~80, decompose.go:~663 are the only TreeDiff calls in those files; no
`TokenLimit: 0` re-capture calls exist. So there is no "avoid the second call" concern here.

## 2. The 3 sites — exact current code (verified)

### Site (4) — `generateMessage` (internal/decompose/message.go:~88-99) — MESSAGE role
In scope: `sysPrompt` (line 84, from `messageSystemPrompt`), `reserve` (line 87), `deps.Config`,
`git.EstimateTokens`, `prompt.BuildUserPayload`. Config is `deps.Config` (not `cfg`).
```go
	sysPrompt, err := messageSystemPrompt(ctx, deps.Git, deps.Config, isUnborn)   // :84
	...
	reserve := prompt.MessageReserveTokens(sysPrompt, deps.Config.MaxDuplicateRetries, deps.Config.SubjectTargetChars, deps.Config.Context, git.EstimateTokens)  // :87

	diff, err := deps.Git.TreeDiff(ctx, treeA, treeB, git.StagedDiffOptions{   // :91
		MaxDiffBytes:        deps.Config.MaxDiffBytes,
		...
		TokenLimit:          deps.Config.TokenLimit,
		DiffContext:         deps.Config.DiffContextValue(),
		PromptReserveTokens: reserve,
	})
```
**Closure (matches the contract verbatim):** `func(gatedDiff string) int { return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, deps.Config.Context, nil)) }`
— uses `BuildUserPayload(diff, context, rejected)` (payload.go:97); `nil` for rejected (capture-time).

### Site (5) — `callPlanner` (internal/decompose/planner.go:~72-83) — PLANNER role
In scope: **`sysPrompt`** (line 72, from `prompt.BuildPlannerSystemPrompt`), `reserve` (line 74),
`forcedCount` (callPlanner param), `deps.Config`.
```go
	sysPrompt := prompt.BuildPlannerSystemPrompt(examples, deps.Config.Format, deps.Config.Locale, forcedCount, deps.Config.MaxCommits)  // :72
	reserve := prompt.PlannerReserveTokens(sysPrompt, forcedCount, deps.Config.Context, git.EstimateTokens)  // :74

	diff, err := deps.Git.TreeDiff(ctx, baseTree, tStart, git.StagedDiffOptions{   // :~80
		...
		PromptReserveTokens: reserve,
	})
```
**⚠️ VARIABLE-NAME CORRECTION (the one real gotcha):** the contract wrote `plannerSys` in the closure
body, but the **live variable is `sysPrompt`** (planner.go:72). Using `plannerSys` would be an
`undefined: plannerSys` compile error. The closure MUST capture `sysPrompt`:
`func(gatedDiff string) int { return git.EstimateTokens(sysPrompt + prompt.BuildPlannerUserPayload(gatedDiff, deps.Config.Context, forcedCount)) }`
— uses `BuildPlannerUserPayload(diff, context, forcedCount)` (planner.go:175).

### Site (6) — `runArbiterPhase` (internal/decompose/decompose.go:~650-665) — ARBITER role
In scope: **`arbiterSys`** (line 655, from `prompt.BuildArbiterSystemPrompt()`), `arbiterCommits`
(line ~654, from `convertArbiterCommits(commits)` — type `[]ArbiterCommit`), `reserve` (line 656),
`deps.Config`.
```go
	arbiterCommits, _ := convertArbiterCommits(commits)   // :~654
	arbiterSys := prompt.BuildArbiterSystemPrompt()       // :655
	reserve := prompt.ArbiterReserveTokens(arbiterSys, arbiterCommits, git.EstimateTokens)  // :656
	tipTree := chainData[len(chainData)-1].Tree           // :~662
	leftoverDiff, err := deps.Git.TreeDiff(ctx, tipTree, tStart, git.StagedDiffOptions{   // :~663
		...
		PromptReserveTokens: reserve,
	})
```
**Closure (matches the contract; note the diff is the SECOND arg):** `func(gatedDiff string) int {
return git.EstimateTokens(arbiterSys + prompt.BuildArbiterUserPayload(arbiterCommits, gatedDiff)) }`
— `BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string)` (arbiter.go:92) takes the diff
as its SECOND argument; the gated diff replaces `leftoverDiff`. `arbiterCommits` is captured (in scope).

## 3. The edit pattern (identical shape at all 3 sites; role-specific closure body)

At each site, insert between the `reserve :=` line and the `TreeDiff` call:
```go
	// FR3j closed-loop: when token_limit is set, the gate re-measures the ACTUAL assembled prompt
	// (sysPrompt + role-payload-builder(gatedDiff)) after water-fill truncation and re-trims until it
	// fits token_limit. nil when TokenLimit==0 (the gate branch doesn't run; byte-identical legacy path).
	var measureAssembled func(string) int
	if deps.Config.TokenLimit != 0 {
		measureAssembled = func(gatedDiff string) int {
			return git.EstimateTokens(<SYS> + prompt.<BUILDER>(<ARGS with gatedDiff>))
		}
	}
```
then add `MeasureAssembled: measureAssembled,` to the `StagedDiffOptions` literal (after `PromptReserveTokens: reserve,`).

Per-site `<SYS>` + `<BUILDER>(<ARGS>)`:
- (4) message:  `sysPrompt + prompt.BuildUserPayload(gatedDiff, deps.Config.Context, nil)`
- (5) planner:  `sysPrompt + prompt.BuildPlannerUserPayload(gatedDiff, deps.Config.Context, forcedCount)`  ← `sysPrompt`, NOT `plannerSys`
- (6) arbiter:  `arbiterSys + prompt.BuildArbiterUserPayload(arbiterCommits, gatedDiff)`  ← diff is 2nd arg

## 4. Why this is behavior-free when TokenLimit==0 (regression guarantee)

The conditional leaves `measureAssembled` nil when `deps.Config.TokenLimit == 0`. T1.S2's `closedLoopGate`
is nil-safe (`MeasureAssembled == nil` OR `TokenLimit == 0` → first-cut `applyWaterFillGate` only → legacy
path). So every existing decompose test (which runs with `TokenLimit == 0` unless it explicitly sets it)
resolves byte-identically → `go test ./...` stays green. The only NEW behavior (a non-nil closure
re-measuring when `TokenLimit != 0`) has no existing test exercising it (P1.M1.T3 owns the closed-loop
invariant tests).

## 5. Scope boundaries (do NOT do)

- Do NOT touch the message-role `StagedDiff` sites (generate.go/stagecoach.go/exec.go) — that is S1
  (P1.M1.T2.S1, parallel). This task is the 3 decompose `TreeDiff` sites only.
- Do NOT touch `internal/git/git.go` or `tokengate.go` (T1.S1 field + T1.S2 gate wiring — LANDED).
- Do NOT add new imports — `git` + `prompt` are already imported at all 3 sites (they call
  `git.TreeDiff`, `git.EstimateTokens`, `prompt.*ReserveTokens`, etc.).
- Do NOT add tests (P1.M1.T3 owns the closed-loop invariant tests). Existing suite green is the proof.
- Do NOT edit `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`, docs, or any other package.

## 6. Decisions log

| # | Point | Decision | Why |
|---|---|---|---|
| D1 | planner closure uses `plannerSys` (contract) or `sysPrompt` (live)? | **`sysPrompt`** (live) | The contract's `plannerSys` does not exist in the source — `callPlanner` names the variable `sysPrompt` (planner.go:72). Using `plannerSys` is a compile error. Sites (4) `sysPrompt` and (6) `arbiterSys` match the contract as-is. |
| D2 | Config reference | `deps.Config` (not `cfg`) | All 3 decompose sites access config via `deps.Config` (Deps struct field); there is no local `cfg`. The conditional is `deps.Config.TokenLimit != 0`. |
| D3 | `rejected` arg in message closure | `nil` | The diff is captured ONCE before the dedupe loop; at capture time rejected is empty. Matches S1's rationale; PromptReserveTokens covers worst-case rejection growth. |
| D4 | Arbiter builder arg order | `BuildArbiterUserPayload(arbiterCommits, gatedDiff)` | Signature is `(commits []ArbiterCommit, leftoverDiff string)` — diff is the 2nd arg. The gated diff replaces leftoverDiff. arbiterCommits is captured (in scope at decompose.go:~654). |
| D5 | Multi-turn re-capture calls to avoid? | None | Each decompose role has exactly ONE TreeDiff; no `TokenLimit: 0` re-capture calls exist in decompose (unlike the message-role sites). |
| D6 | Variable name `measureAssembled` | local `var measureAssembled func(string) int` per site | Matches S1's naming; function-local (no cross-site collision). |
