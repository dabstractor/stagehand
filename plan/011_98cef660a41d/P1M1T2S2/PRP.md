---
name: "P1.M1.T2.S2 — Wire MeasureAssembled at decompose-role consumer sites (planner, message, arbiter)"
description: |
  Wire the FR3j closed-loop `MeasureAssembled` closure at the 3 decompose-role `TreeDiff` consumer
  sites, mirroring the S1 (P1.M1.T2.S1) pattern at the message-role `StagedDiff` sites. At each site,
  when `deps.Config.TokenLimit != 0`, add a closure that measures the ACTUAL assembled prompt
  (role-sysPrompt + role-payload-builder(gatedDiff)) via `git.EstimateTokens`, so `closedLoopGate`
  (T1.S2, LANDED) can re-trim until it fits `token_limit`. The 3 sites:
  (4) internal/decompose/message.go:~91 `generateMessage` (MESSAGE) →
      `git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, deps.Config.Context, nil))`;
  (5) internal/decompose/planner.go:~80 `callPlanner` (PLANNER) →
      `git.EstimateTokens(sysPrompt + prompt.BuildPlannerUserPayload(gatedDiff, deps.Config.Context, forcedCount))`
      ⚠️ the live variable is `sysPrompt`, NOT the contract's `plannerSys`;
  (6) internal/decompose/decompose.go:~663 `runArbiterPhase` (ARBITER) →
      `git.EstimateTokens(arbiterSys + prompt.BuildArbiterUserPayload(arbiterCommits, gatedDiff))`
      (diff is the builder's 2nd arg). When `TokenLimit==0`, the closure is nil (byte-identical legacy
      path — closedLoopGate is nil-safe). Each decompose role has exactly ONE TreeDiff (no multi-turn
      re-capture calls to avoid, unlike S1). No new imports, no tests (P1.M1.T3), no docs. Baseline GREEN.
---

## Goal

**Feature Goal**: Complete the FR3j closed-loop wiring at the 3 decompose-role `TreeDiff` consumer sites
so the closed-loop gate (`closedLoopGate`, wired by T1.S2) can measure each role's ACTUAL assembled
prompt and re-trim until it fits `token_limit` — extending the FR3j hard guarantee
(`EstimateTokens(assembledFullPrompt) ≤ token_limit`, always) to the planner, message, and arbiter roles
of the decompose pipeline, not just the single-commit message role (S1).

**Deliverable** (3 edits in 3 files — one conditional closure + one struct-field addition each):
1. `internal/decompose/message.go` (~line 91, `generateMessage`): +`MeasureAssembled` closure (message role).
2. `internal/decompose/planner.go` (~line 80, `callPlanner`): +`MeasureAssembled` closure (planner role).
3. `internal/decompose/decompose.go` (~line 663, `runArbiterPhase`): +`MeasureAssembled` closure (arbiter role).

**Success Definition**: When `deps.Config.TokenLimit != 0`, each of the 3 `TreeDiff` calls passes a
non-nil `MeasureAssembled` closure measuring the role's actual assembled prompt; when `TokenLimit == 0`,
the closure is nil (the gate doesn't run; byte-identical legacy path). `go build/vet/gofmt` clean;
`go test ./...` green. The message-role `StagedDiff` sites (S1) and `internal/git/*` (T1.S1/S2) are untouched.

## User Persona

**Target User**: The user who sets `token_limit` and runs `stagecoach` in decompose mode (multiple
commits), relying on the FR3j guarantee that NONE of the role prompts (planner, per-concept message,
arbiter) ever exceeds `token_limit`. Also the contributor implementing P1.M1.T3 (the closed-loop
invariant E2E tests).

**Use Case**: A user sets `token_limit = 120000` and runs decompose on a large changeset. The planner's
working-tree diff, each concept's tree-to-tree diff, and the arbiter's leftover diff are each captured,
water-fill-truncated (FR3i), then re-measured by the role's `MeasureAssembled` closure and re-trimmed
if the assembled prompt landed over 120000 (FR3j). This task provides the closures that activate the
guarantee on all three decompose roles.

**User Journey**: decompose activates → planner `TreeDiff(baseTree, T_start)` (closure measures
plannerSys + BuildPlannerUserPayload) → loop: message `TreeDiff(tree[i-1], tree[i])` (closure measures
sysPrompt + BuildUserPayload) → arbiter `TreeDiff(tipTree, T_start)` (closure measures arbiterSys +
BuildArbiterUserPayload). Each gates against `token_limit`.

**Pain Points Addressed**: After T1.S1 (field) + T1.S2 (gate) + S1 (message-role sites), the closed-loop
is structurally wired for the single-commit path but the 3 decompose roles still pass `nil` for
`MeasureAssembled` → the gate delegates to the open-loop first-cut (no re-measurement) on decompose.
This task injects the role-specific closures, activating the FR3j guarantee across the whole decompose pipeline.

## Why

- **FR3j mandates the closed-loop guarantee for EVERY role that routes through the gate.** PRD §9.1 FR3j:
  *"The guarantee holds for every role that routes through the gate (message, planner, arbiter)."* S1
  covered the message-role StagedDiff sites; this task covers the 3 decompose-role TreeDiff sites.
- **The seam (T1.S1) and the gate (T1.S2) are LANDED.** `StagedDiffOptions.MeasureAssembled`
  (git.go:81) + `closedLoopGate` called inside `TreeDiff` (git.go:1527) exist and are nil-safe. The only
  missing piece is the 3 decompose consumers INJECTING the closure. Without this task, `MeasureAssembled`
  is nil at every decompose site → the closed-loop is dead code on decompose.
- **3 mechanical edits mirroring S1.** Each site already has its sysPrompt (`sysPrompt`/`arbiterSys`) +
  reserve in scope, the role's payload builder imported, and `git.EstimateTokens` reachable. The edit is
  a conditional closure + a struct-field addition.
- **No behavioral change when TokenLimit==0.** The closure is nil → `closedLoopGate`'s nil-safe path
  delegates to `applyWaterFillGate` → the legacy path is byte-identical. Existing decompose tests (which
  run with `TokenLimit==0`) pass unchanged.

## What

A conditional `MeasureAssembled` closure added to 3 `TreeDiff` `StagedDiffOptions` literals. When
`deps.Config.TokenLimit != 0`, the closure measures the role's actual assembled prompt; when
`TokenLimit == 0`, the closure is nil (omitted). No signature changes, no new imports, no docs, no tests.

### Success Criteria

- [ ] Each of the 3 decompose `TreeDiff` calls (message.go:~91, planner.go:~80, decompose.go:~663) passes
      a non-nil `MeasureAssembled` closure when `deps.Config.TokenLimit != 0`.
- [ ] message closure: `func(gatedDiff string) int { return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, deps.Config.Context, nil)) }`.
- [ ] planner closure: `func(gatedDiff string) int { return git.EstimateTokens(sysPrompt + prompt.BuildPlannerUserPayload(gatedDiff, deps.Config.Context, forcedCount)) }`
      (uses the live variable `sysPrompt`, NOT `plannerSys`).
- [ ] arbiter closure: `func(gatedDiff string) int { return git.EstimateTokens(arbiterSys + prompt.BuildArbiterUserPayload(arbiterCommits, gatedDiff)) }`
      (diff is the builder's 2nd arg).
- [ ] When `deps.Config.TokenLimit == 0`, the closure is nil.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test ./...` green.
- [ ] The message-role `StagedDiff` sites (S1) and `internal/git/*` (T1.S1/S2) are UNTOUCHED.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim current code at all 3 TreeDiff sites (with line
numbers + the in-scope sysPrompt/reserve variables), gives the exact closure body per role (copy-paste-
ready), the conditional pattern, the insertion point (between `reserve :=` and the `TreeDiff` call), and
front-loads the one real trap — the planner variable is `sysPrompt`, not the contract's `plannerSys`
(undefined → compile error). T1.S1's field + T1.S2's nil-safe gate are confirmed LANDED. No inference.

### Documentation & References

```yaml
# MUST READ — the FR3j spec + the wiring architecture
- file: PRD.md
  why: "§9.1 FR3j (closed-loop budget guarantee): 'after the water-fill produces the gated diff, stagecoach
        assembles the actual full prompt — the system prompt plus [the role's payload builder](gatedDiff) —
        measures it with the SAME EstimateTokens used for sizing, and if it exceeds token_limit, reduces the
        body budget by the overshoot and re-applies the per-file truncation. Invariant:
        EstimateTokens(assembledFullPrompt) ≤ token_limit, always. The guarantee holds for EVERY role that
        routes through the gate (message, planner, arbiter).'"
  critical: "FR3j's 'every role (message, planner, arbiter)' IS this task's mandate — S1 did message; THIS
             task does planner + message(decompose) + arbiter. The 'same EstimateTokens' = single-estimator
             rule (use git.EstimateTokens, the chars/4 estimator)."

- docfile: plan/011_98cef660a41d/architecture/fr3j_closed_loop.md
  why: "§'The 6 Consumer Wiring Sites' table: sites 4/5/6 are THIS task (message.go generateMessage,
        planner.go callPlanner, decompose.go runArbiterPhase); sites 1/2/3 are S1 (message-role StagedDiff).
        §'nil-safe: MeasureAssembled == nil OR TokenLimit == 0 → behave exactly as today' is the regression
        guarantee. The closure shape (EstimateTokens(sysPrompt + role_payload_builder(gatedDiff))) is the spec."
  critical: "The 6-site table confirms the message-role (S1) vs decompose-role (S2) split. The closure shape
             + the nil-safe rationale are the implementation spec. NOTE the doc may use 'plannerSys' as a
             label — the LIVE variable is 'sysPrompt' (see D1 / gotcha G1)."

- docfile: plan/011_98cef660a41d/P1M1T2S1/PRP.md
  why: "The S1 CONTRACT (parallel) — the message-role pattern this task mirrors. S1 wires the 3 StagedDiff
        sites with `func(gatedDiff string) int { return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, cfg.Context, nil)) }`,
        conditional on `cfg.TokenLimit != 0`. This task applies the same shape at the 3 TreeDiff sites with
        role-specific builders + `deps.Config`. Confirms S1 does NOT touch decompose files → no conflict."
  critical: "Mirror S1's conditional (`var measureAssembled func(string) int` + `if <cfg>.TokenLimit != 0`)
             + the `nil`-rejected rationale. The difference: decompose uses `deps.Config` (not `cfg`) and
             role-specific payload builders."

- docfile: plan/011_98cef660a41d/P1M1T1S2/PRP.md
  why: "The T1.S2 sibling (LANDED) — wired closedLoopGate into StagedDiff/TreeDiff/WorkingTreeDiff.
        Confirms TreeDiff (git.go:1527) calls closedLoopGate(... opts.MeasureAssembled) and is nil-safe."
  critical: "T1.S2 is LANDED — the gate is live in TreeDiff. This task's closures are consumed by it.
             nil MeasureAssembled ⇒ first-cut applyWaterFillGate only (byte-identical legacy)."

- docfile: plan/011_98cef660a41d/P1M1T2S2/research/measure_assembled_decompose_notes.md
  why: "THIS task's research: the verbatim current code at all 3 TreeDiff sites (with exact line numbers +
        in-scope variables), the per-role closure bodies, the plannerSys→sysPrompt CORRECTION (D1), the
        arbiter 2nd-arg diff placement (D4), the 'no multi-turn re-capture in decompose' finding (D5),
        and decisions D1–D6. READ THIS FIRST."
  critical: "§2 (the 3 sites' current code + closure per role) and §3 (the edit pattern) are copy-paste-ready.
             §6 D1 (sysPrompt, not plannerSys) is the one compile-blocking trap."

# The edit targets (verbatim current state)
- file: internal/decompose/message.go
  why: "EDIT (generateMessage, ~line 91). The TreeDiff literal is at ~91-99. `sysPrompt` at :84; `reserve`
        at :87. Config is `deps.Config`. Insert the closure var between reserve and TreeDiff; add
        `MeasureAssembled: measureAssembled,` to the literal."
  pattern: "Same shape as S1's message-role closure but with `deps.Config` (not `cfg`). Uses
            prompt.BuildUserPayload(gatedDiff, deps.Config.Context, nil)."
  gotcha: "Only ONE TreeDiff in this file (line 91) — no multi-turn re-capture to avoid."

- file: internal/decompose/planner.go
  why: "EDIT (callPlanner, ~line 80). The TreeDiff literal is at ~80-88. `sysPrompt` at :72 (from
        BuildPlannerSystemPrompt); `reserve` at :74; `forcedCount` is a callPlanner param (in scope).
        Insert the closure var between reserve and TreeDiff; add the field to the literal."
  pattern: "Uses prompt.BuildPlannerUserPayload(gatedDiff, deps.Config.Context, forcedCount)."
  gotcha: "CRITICAL: capture `sysPrompt` (the LIVE variable at :72), NOT `plannerSys` (the contract/doc
           label — undefined in source → compile error). See gotcha G1."

- file: internal/decompose/decompose.go
  why: "EDIT (runArbiterPhase, ~line 663). The TreeDiff literal is at ~663-671. `arbiterSys` at :655;
        `arbiterCommits` at ~:654 (type []ArbiterCommit); `reserve` at :656. Insert the closure var
        between reserve and TreeDiff; add the field to the literal."
  pattern: "Uses prompt.BuildArbiterUserPayload(arbiterCommits, gatedDiff) — diff is the 2ND arg."
  gotcha: "BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string) — the gated diff replaces
           leftoverDiff (2nd arg), NOT commits. arbiterCommits is captured (in scope). See gotcha G3."

# Read-only refs (do NOT edit)
- file: internal/git/git.go
  why: "READ-ONLY. StagedDiffOptions.MeasureAssembled field at :81 (T1.S1 LANDED). TreeDiff calls
        closedLoopGate(... opts.MeasureAssembled) at :1527 (T1.S2 LANDED, nil-safe)."
- file: internal/prompt/payload.go
  why: "READ-ONLY. `func BuildUserPayload(diff, context string, rejected []string) string` (line 97)."
- file: internal/prompt/planner.go
  why: "READ-ONLY. `func BuildPlannerUserPayload(diff, context string, forcedCount int) string` (line 175)."
- file: internal/prompt/arbiter.go
  why: "READ-ONLY. `func BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string) string`
        (line 92) — diff is the SECOND argument."
- file: internal/git/tokens.go
  why: "READ-ONLY. `func EstimateTokens(s string) int` — the chars/4 estimator. The closure uses the SAME
        estimator the water-fill sizing uses (single-estimator rule, FR3j)."

# External references
- url: https://go.dev/ref/spec#Function_types
  why: "Confirms a `func(string) int` field on a struct accepts a closure literal and that an uninitialized
        `var measureAssembled func(string) int` is nil (the nil-safe path). The conditional assignment keeps
        the closure nil when TokenLimit==0."
```

### Current Codebase Tree (relevant slice — T1.S1 + T1.S2 LANDED; S1 parallel)

```bash
stagecoach/
└── internal/decompose/
    ├── message.go      # EDIT (site 4): generateMessage TreeDiff (~line 91)  [MESSAGE role]
    ├── planner.go      # EDIT (site 5): callPlanner TreeDiff (~line 80)       [PLANNER role]
    └── decompose.go    # EDIT (site 6): runArbiterPhase TreeDiff (~line 663)  [ARBITER role]
# (internal/git/git.go MeasureAssembled field + closedLoopGate = T1.S1/S2 LANDED, read-only)
# (internal/generate/generate.go, pkg/stagecoach/stagecoach.go, internal/hook/exec.go = S1, parallel)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/decompose/message.go    # +MeasureAssembled closure (message role, BuildUserPayload)
    internal/decompose/planner.go    # +MeasureAssembled closure (planner role, BuildPlannerUserPayload)
    internal/decompose/decompose.go  # +MeasureAssembled closure (arbiter role, BuildArbiterUserPayload)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/decompose/message.go` | MODIFY | Add the conditional MeasureAssembled closure to `generateMessage`'s TreeDiff StagedDiffOptions (message role). |
| `internal/decompose/planner.go` | MODIFY | Add the closure to `callPlanner`'s TreeDiff StagedDiffOptions (planner role). |
| `internal/decompose/decompose.go` | MODIFY | Add the closure to `runArbiterPhase`'s TreeDiff StagedDiffOptions (arbiter role). |

**Explicitly NOT touched**: `internal/git/git.go` + `tokengate.go` (T1.S1 field + T1.S2 gate — LANDED),
the 3 message-role `StagedDiff` sites (`internal/generate/generate.go`, `pkg/stagecoach/stagecoach.go`,
`internal/hook/exec.go` — S1, parallel), any tests (P1.M1.T3), any docs, `PRD.md`, `tasks.json`,
`prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — planner closure MUST capture `sysPrompt`, NOT `plannerSys`): the contract and the
// architecture doc label the planner's system-prompt variable `plannerSys`, but the LIVE code names it
// `sysPrompt` (planner.go:72: `sysPrompt := prompt.BuildPlannerSystemPrompt(...)`). Writing `plannerSys`
// in the closure is an `undefined: plannerSys` COMPILE ERROR. Sites (4) and (6) match the contract labels
// (`sysPrompt` and `arbiterSys` respectively — both exist in the source). Only site (5) needs the correction.

// CRITICAL (G2 — Config is `deps.Config`, not `cfg`): all 3 decompose sites access config via the `Deps`
// struct field `deps.Config` (there is no local `cfg`). The conditional is `if deps.Config.TokenLimit != 0`
// and the closure references `deps.Config.Context`. (S1's message-role sites used a local `cfg` — do not
// copy that name into decompose.)

// CRITICAL (G3 — arbiter builder takes the diff as its SECOND argument): `BuildArbiterUserPayload(commits
// []ArbiterCommit, leftoverDiff string)` (arbiter.go:92). The closure is
// `prompt.BuildArbiterUserPayload(arbiterCommits, gatedDiff)` — `arbiterCommits` (captured, in scope at
// decompose.go:~654) FIRST, `gatedDiff` SECOND. Do NOT put gatedDiff first. `arbiterCommits` is
// []ArbiterCommit (from convertArbiterCommits) — matches the builder's first param exactly.

// CRITICAL (G4 — conditional on deps.Config.TokenLimit != 0): when TokenLimit is 0, the gate branch in
// TreeDiff doesn't run, so MeasureAssembled is never called. Omitting the closure (nil) keeps the legacy
// path byte-identical (closedLoopGate is nil-safe — T1.S2). Use a local `var measureAssembled func(string) int`
// + conditional assignment, exactly as S1 does.

// GOTCHA (G5 — the closure uses the SAME git.EstimateTokens as the sizing): FR3j's single-estimator rule.
// Do NOT use a different estimator (e.g. a real tokenizer). git.EstimateTokens is already imported at all
// 3 sites (they call prompt.*ReserveTokens(..., git.EstimateTokens)).

// GOTCHA (G6 — `nil` for rejected in the message closure): the diff is captured ONCE before the dedupe
// loop; at capture time rejected is empty. PromptReserveTokens (already computed at each site) covers
// worst-case rejection growth. Matches S1's rationale. Do NOT pass a `rejected` variable.

// GOTCHA (G7 — no multi-turn re-capture TreeDiff calls in decompose): unlike S1's message-role sites
// (which each have a 2nd multi-turn re-capture StagedDiff with TokenLimit: 0 to AVOID), each decompose
// role has exactly ONE TreeDiff. There is no second call to skip. Confirmed: grep TreeDiff in the 3 files
// → one call each; no TokenLimit: 0 re-capture calls.

// GOTCHA (G8 — no new imports): `git` and `prompt` are already imported at all 3 sites (they call
// git.TreeDiff, git.EstimateTokens, prompt.*ReserveTokens, prompt.BuildPlannerSystemPrompt, etc.). The
// builders BuildUserPayload / BuildPlannerUserPayload / BuildArbiterUserPayload are in the already-imported
// `prompt` package. Adding the closure introduces no new import.

// GOTCHA (G9 — behavior-free when TokenLimit==0): the conditional leaves measureAssembled nil when
// deps.Config.TokenLimit == 0. closedLoopGate (T1.S2) is nil-safe → first-cut applyWaterFillGate only →
// legacy path byte-identical. Every existing decompose test (TokenLimit==0 unless explicitly set) passes
// unchanged → go test ./... green by construction.
```

## Implementation Blueprint

### Data models and structure

No new types. The `MeasureAssembled func(gatedDiff string) int` field already exists on
`StagedDiffOptions` (T1.S1, git.go:81). This task injects a role-specific closure at each of the 3
decompose consumer sites.

### The edit pattern (same shape at all 3 sites; role-specific closure body)

At each site, insert between the `reserve :=` line and the `TreeDiff` call:
```go
	// FR3j closed-loop: when token_limit is set, the gate re-measures the ACTUAL assembled prompt
	// (<sys> + <role payload builder>(gatedDiff)) after water-fill truncation and re-trims until it
	// fits token_limit. nil when TokenLimit==0 (the gate branch doesn't run; byte-identical legacy path).
	var measureAssembled func(string) int
	if deps.Config.TokenLimit != 0 {
		measureAssembled = func(gatedDiff string) int {
			return git.EstimateTokens(<SYS> + prompt.<BUILDER>(<BUILDER ARGS with gatedDiff>))
		}
	}
```
then add `MeasureAssembled: measureAssembled,` to the `StagedDiffOptions` literal (after
`PromptReserveTokens: reserve,`).

**Per-site `<SYS>` + `<BUILDER>(<ARGS>)`:**

| Site | Role | `<SYS>` | `prompt.<BUILDER>(<ARGS>)` |
|---|---|---|---|
| (4) message.go:~91 | message | `sysPrompt` | `BuildUserPayload(gatedDiff, deps.Config.Context, nil)` |
| (5) planner.go:~80 | planner | `sysPrompt` ⚠️ (NOT `plannerSys`) | `BuildPlannerUserPayload(gatedDiff, deps.Config.Context, forcedCount)` |
| (6) decompose.go:~663 | arbiter | `arbiterSys` | `BuildArbiterUserPayload(arbiterCommits, gatedDiff)` ⚠️ diff is 2nd arg |

### The 3 concrete edits (exact)

**Site (4) — `internal/decompose/message.go` (`generateMessage`, insert after `reserve :=` at line 87):**
```go
	var measureAssembled func(string) int
	if deps.Config.TokenLimit != 0 {
		measureAssembled = func(gatedDiff string) int {
			return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, deps.Config.Context, nil))
		}
	}
```
+ `MeasureAssembled: measureAssembled,` in the TreeDiff literal (after `PromptReserveTokens: reserve,`).

**Site (5) — `internal/decompose/planner.go` (`callPlanner`, insert after `reserve :=` at line 74):**
```go
	var measureAssembled func(string) int
	if deps.Config.TokenLimit != 0 {
		measureAssembled = func(gatedDiff string) int {
			return git.EstimateTokens(sysPrompt + prompt.BuildPlannerUserPayload(gatedDiff, deps.Config.Context, forcedCount))
		}
	}
```
+ `MeasureAssembled: measureAssembled,` in the TreeDiff literal. **⚠️ `sysPrompt` (line 72), NOT `plannerSys`.**

**Site (6) — `internal/decompose/decompose.go` (`runArbiterPhase`, insert after `reserve :=` at line 656):**
```go
	var measureAssembled func(string) int
	if deps.Config.TokenLimit != 0 {
		measureAssembled = func(gatedDiff string) int {
			return git.EstimateTokens(arbiterSys + prompt.BuildArbiterUserPayload(arbiterCommits, gatedDiff))
		}
	}
```
+ `MeasureAssembled: measureAssembled,` in the TreeDiff literal. **⚠️ `BuildArbiterUserPayload(arbiterCommits, gatedDiff)` — diff is the 2nd arg.**

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/decompose/message.go — generateMessage (site 4, message role)
  - FILE: internal/decompose/message.go
  - LOCATE `reserve := prompt.MessageReserveTokens(...)` (line 87) followed by the TreeDiff call (line 91).
  - INSERT (between reserve and TreeDiff): the `var measureAssembled` + conditional closure (site 4 body above).
  - ADD `MeasureAssembled: measureAssembled,` to the StagedDiffOptions literal (after `PromptReserveTokens: reserve,`).
  - DO NOT: rename sysPrompt; touch any other function; add imports.
  - RUN: gofmt -w internal/decompose/message.go; go build ./internal/decompose/

Task 2: EDIT internal/decompose/planner.go — callPlanner (site 5, planner role)
  - FILE: internal/decompose/planner.go
  - LOCATE `reserve := prompt.PlannerReserveTokens(...)` (line 74) followed by the TreeDiff call (~line 80).
  - INSERT the `var measureAssembled` + conditional closure (site 5 body above).
  - ⚠️ Capture `sysPrompt` (the variable at line 72), NOT `plannerSys` (gotcha G1 — undefined).
  - ADD `MeasureAssembled: measureAssembled,` to the literal.
  - RUN: gofmt -w internal/decompose/planner.go; go build ./internal/decompose/

Task 3: EDIT internal/decompose/decompose.go — runArbiterPhase (site 6, arbiter role)
  - FILE: internal/decompose/decompose.go
  - LOCATE `reserve := prompt.ArbiterReserveTokens(...)` (line 656) followed by the TreeDiff call (~line 663).
  - INSERT the `var measureAssembled` + conditional closure (site 6 body above).
  - ⚠️ `BuildArbiterUserPayload(arbiterCommits, gatedDiff)` — commits FIRST, gatedDiff SECOND (gotcha G3).
  - ADD `MeasureAssembled: measureAssembled,` to the literal.
  - RUN: gofmt -w internal/decompose/decompose.go; go build ./internal/decompose/

Task 4: VALIDATE (no new test — P1.M1.T3 owns the closed-loop invariant tests; existing suite green is the proof)
  - RUN: gofmt -l .
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test ./...     # full suite green (closure nil when TokenLimit==0 → byte-identical legacy)
  - RUN (wiring grep — the contract's headline check):
        grep -n 'measureAssembled' internal/decompose/message.go internal/decompose/planner.go internal/decompose/decompose.go
        # EXPECT: 3 var decls + 3 conditionals + 3 literal fields = 9 total (3 per site).
  - RUN (role-specific builders present):
        grep -n 'BuildUserPayload(gatedDiff\|BuildPlannerUserPayload(gatedDiff\|BuildArbiterUserPayload(arbiterCommits, gatedDiff' internal/decompose/*.go
        # EXPECT: one match per site (3 total).
  - RUN (scope grep):
        git diff --stat -- internal/git/ internal/generate/ pkg/stagecoach/ internal/hook/
        # EXPECT: EMPTY (only the 3 decompose files changed).
  - FIX-FORWARD: `undefined: plannerSys` = used the contract label instead of `sysPrompt` (G1);
                 `cannot use gatedDiff as []ArbiterCommit` = swapped the arbiter builder args (G3).
```

### Implementation Patterns & Key Details

```go
// === The MeasureAssembled closure (FR3j closed-loop re-measurement, decompose roles) ===
// The gate (closedLoopGate, T1.S2) calls opts.MeasureAssembled(gatedDiff) AFTER water-fill truncation
// to measure the ACTUAL assembled prompt. If it exceeds token_limit, the gate reduces the body budget
// and re-applies truncation (bounded loop, converges in 1-2 passes).
//
// Per role, the closure measures the role's OWN assembled prompt:
//   message: EstimateTokens(sysPrompt + BuildUserPayload(gatedDiff, Context, nil))
//   planner: EstimateTokens(sysPrompt + BuildPlannerUserPayload(gatedDiff, Context, forcedCount))
//   arbiter: EstimateTokens(arbiterSys  + BuildArbiterUserPayload(arbiterCommits, gatedDiff))
// EstimateTokens is the SAME chars/4 estimator the water-fill sizing uses (single-estimator rule, FR3j).

// === Why conditional on deps.Config.TokenLimit != 0 ===
// When TokenLimit is 0, the gate branch in TreeDiff doesn't run; closedLoopGate is never called. The
// closure is nil → harmless (closedLoopGate nil-safe → applyWaterFillGate first-cut only). The conditional
// avoids constructing the closure (which captures sysPrompt/arbiterCommits) when it will never be called.

// === Why decompose is simpler than S1 (no multi-turn re-capture to avoid) ===
// S1's message-role sites each have a 2nd multi-turn re-capture StagedDiff (TokenLimit: 0) that must NOT
// get a MeasureAssembled. The decompose roles have exactly ONE TreeDiff each — no second call to skip.
```

### Integration Points

```yaml
PRODUCTION (3 files, 1 edit each):
  - internal/decompose/message.go:    +MeasureAssembled closure to generateMessage's TreeDiff (message role)
  - internal/decompose/planner.go:    +MeasureAssembled closure to callPlanner's TreeDiff (planner role)
  - internal/decompose/decompose.go:  +MeasureAssembled closure to runArbiterPhase's TreeDiff (arbiter role)

CONSUMED (READ-ONLY):
  - internal/git/git.go:81: StagedDiffOptions.MeasureAssembled field (T1.S1 LANDED)
  - internal/git/git.go:1527: TreeDiff calls closedLoopGate(... opts.MeasureAssembled) (T1.S2 LANDED, nil-safe)
  - internal/prompt/payload.go:97: BuildUserPayload(diff, context, rejected)
  - internal/prompt/planner.go:175: BuildPlannerUserPayload(diff, context, forcedCount)
  - internal/prompt/arbiter.go:92: BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string)
  - internal/git/tokens.go: EstimateTokens(s) — the chars/4 estimator

GATE: go build ./... → OK; go test ./... green (closure nil when TokenLimit==0)

NO-TOUCH (explicitly):
  - internal/git/git.go, tokengate.go (T1.S1 field + T1.S2 gate — LANDED)
  - internal/generate/generate.go, pkg/stagecoach/stagecoach.go, internal/hook/exec.go (S1 — parallel message-role sites)
  - tests (P1.M1.T3), docs, PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational):
  - P1.M1.T3.S2: the E2E test asserting EstimateTokens(assembledFullPrompt) ≤ token_limit for every role
    (message, planner, arbiter) — consumes the closures this task wires.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/decompose/   # Expected: empty (run gofmt -w if listed)
go vet ./internal/decompose/... # Expected: exit 0
go build ./...                  # Expected: exit 0 (closure field; no signature change)

# Expected: Zero errors. `undefined: plannerSys` = used the contract label instead of `sysPrompt` (G1).
# `cannot use gatedDiff (string) as []ArbiterCommit` = swapped BuildArbiterUserPayload args (G3).
```

### Level 2: Wiring + Regression (existing suite stays green — no new test in S2)

```bash
cd /home/dustin/projects/stagecoach

# The wiring is present (3 var decls + 3 conditionals + 3 literal fields = 9 hits):
grep -n 'measureAssembled' internal/decompose/message.go internal/decompose/planner.go internal/decompose/decompose.go
# Expected: 9 matches total — 3 per site (var + conditional-assignment line is inside the if; count the
#           var decl, the closure-assignment, and the literal field). At minimum 3 literal-field hits
#           (`MeasureAssembled: measureAssembled,`).

# The role-specific builders are present (one per site):
grep -n 'BuildUserPayload(gatedDiff\|BuildPlannerUserPayload(gatedDiff\|BuildArbiterUserPayload(arbiterCommits, gatedDiff' internal/decompose/*.go
# Expected: 3 matches (one per file).

# The conditionals gate on deps.Config.TokenLimit:
grep -n 'if deps.Config.TokenLimit != 0' internal/decompose/message.go internal/decompose/planner.go internal/decompose/decompose.go
# Expected: 3 matches (one per site).

# Existing suite green (behavior-free — closure nil when TokenLimit==0):
go test ./internal/decompose/ -v   # all decompose tests pass (TokenLimit==0 → nil closure → legacy path).
go test ./...                      # Expected: ALL packages green.
```

### Level 3: Scope Discipline (only the 3 decompose files changed)

```bash
cd /home/dustin/projects/stagecoach

# ONLY the 3 decompose files changed.
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/decompose/message.go + planner.go + decompose.go only.

# T1's territory (git.go/tokengate.go) + S1's territory (message-role StagedDiff sites) UNTOUCHED:
git diff --stat -- internal/git/ internal/generate/generate.go pkg/stagecoach/stagecoach.go internal/hook/exec.go
# Expected: EMPTY.

# Confirm the diff is exactly the 3 intended hunks (eyeball the patch):
git diff -- internal/decompose/
# Expected: 3 changed hunks — one per file (the var+conditional closure + the literal field).
```

### Level 4: Closed-Loop Activation (the FR3j guarantee lights up for decompose roles)

```bash
cd /home/dustin/projects/stagecoach

# When TokenLimit != 0, the closure is non-nil → closedLoopGate (T1.S2) re-measures the role's assembled
# prompt and re-trims if over. When TokenLimit == 0, the closure is nil → first-cut only (nil-safe).
# The authoritative behavioral proof is P1.M1.T3.S2's E2E invariant test; here we cross-check the shape:
grep -A3 'var measureAssembled' internal/decompose/message.go internal/decompose/planner.go internal/decompose/decompose.go
# Expected: 3 `if deps.Config.TokenLimit != 0 {` blocks, each with the role-specific closure body.

# Cross-check the role closure bodies match the contract (modulo the plannerSys→sysPrompt correction):
grep -n 'sysPrompt + prompt.BuildUserPayload\|sysPrompt + prompt.BuildPlannerUserPayload\|arbiterSys + prompt.BuildArbiterUserPayload' internal/decompose/*.go
# Expected: one match per file (message/planner use `sysPrompt`; arbiter uses `arbiterSys`).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test ./...` — all packages green (closure nil when TokenLimit==0 → byte-identical legacy).

### Feature Validation
- [ ] All 3 decompose `TreeDiff` calls pass a non-nil `MeasureAssembled` when `deps.Config.TokenLimit != 0`.
- [ ] message closure uses `sysPrompt + prompt.BuildUserPayload(gatedDiff, deps.Config.Context, nil)`.
- [ ] planner closure uses `sysPrompt` (NOT `plannerSys`) + `prompt.BuildPlannerUserPayload(gatedDiff, deps.Config.Context, forcedCount)`.
- [ ] arbiter closure uses `arbiterSys + prompt.BuildArbiterUserPayload(arbiterCommits, gatedDiff)` (diff is 2nd arg).
- [ ] When `deps.Config.TokenLimit == 0`, the closure is nil.

### Scope Discipline Validation
- [ ] ONLY `internal/decompose/{message,planner,decompose}.go` modified (`git diff --stat` confirms).
- [ ] Did NOT touch `internal/git/*` (T1.S1/S2 LANDED), the message-role `StagedDiff` sites (S1), tests (P1.M1.T3), docs.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research note).

### Code Quality Validation
- [ ] Closure uses the SAME `git.EstimateTokens` as sizing (single-estimator rule, FR3j).
- [ ] `nil` for the message closure's `rejected` arg (capture-time state).
- [ ] The conditional (`if deps.Config.TokenLimit != 0`) keeps the TokenLimit==0 path clean.
- [ ] gofmt re-aligns; no hand-alignment; no new imports.

---

## Anti-Patterns to Avoid

- ❌ Don't use `plannerSys` in the planner closure — the live variable is `sysPrompt` (planner.go:72).
  `plannerSys` is undefined → compile error. The contract/doc used `plannerSys` as a label; the source
  names it `sysPrompt`. Sites (4) `sysPrompt` and (6) `arbiterSys` match the contract as-is (gotcha G1).
- ❌ Don't use `cfg` for config — decompose sites use `deps.Config` (the Deps struct field). The conditional
  is `if deps.Config.TokenLimit != 0` and the closure references `deps.Config.Context` (gotcha G2).
- ❌ Don't swap the arbiter builder's args — `BuildArbiterUserPayload(commits, leftoverDiff)` takes the diff
  SECOND. Write `BuildArbiterUserPayload(arbiterCommits, gatedDiff)`, not the reverse (gotcha G3).
- ❌ Don't pass a `rejected` variable in the message closure — use `nil`. The diff is captured ONCE before
  the dedupe loop; PromptReserveTokens covers worst-case rejection growth (gotcha G6).
- ❌ Don't omit the `deps.Config.TokenLimit != 0` conditional — when TokenLimit is 0 the gate branch doesn't
  run; a nil closure keeps the legacy path byte-identical (gotcha G4).
- ❌ Don't use a different estimator than `git.EstimateTokens` — FR3j's single-estimator rule (gotcha G5).
- ❌ Don't touch `internal/git/*` (T1.S1/S2 LANDED), the message-role `StagedDiff` sites (S1, parallel),
  tests (P1.M1.T3), or docs. This task is the 3 decompose `TreeDiff` sites only.
- ❌ Don't look for a multi-turn re-capture TreeDiff to skip — decompose roles have exactly ONE TreeDiff
  each (gotcha G7, unlike S1).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is 3 mechanical edits mirroring S1's landed pattern — each a conditional closure + a
struct-field addition — with every site's current code quoted verbatim from the live source (message.go:91,
planner.go:80, decompose.go:663), the in-scope variables confirmed (`sysPrompt`/`arbiterSys`, `reserve`,
`forcedCount`, `arbiterCommits`, `deps.Config`), the per-role closure body copy-paste-ready, and the
`MeasureAssembled` field + nil-safe `closedLoopGate` confirmed LANDED (T1.S1 git.go:81 + T1.S2 git.go:1527).
The one compile-blocking trap — the planner variable is `sysPrompt`, not the contract's `plannerSys` — is
front-loaded as CRITICAL gotcha G1 (and would surface as an `undefined: plannerSym` build error immediately
if missed). The arbiter's 2nd-arg diff placement is front-loaded as G3. When `TokenLimit==0`, the closure is
nil → `closedLoopGate`'s nil-safe path → byte-identical legacy path, so `go test ./...` staying green IS the
regression proof (no decompose test sets TokenLimit unless it explicitly tests the gate; P1.M1.T3 owns those).
The decompose sites are simpler than S1's (exactly ONE TreeDiff each — no multi-turn re-capture to avoid).
The residual 0.5 uncertainty is line-number drift from the parallel S1 work (if it shifted the reserve/TreeDiff
lines) — mitigated by the "grep for `PromptReserveTokens: reserve` to locate the literal" guidance. The
T1 (LANDED), S1 (parallel message-role), and P1.M1.T3 (invariant tests) boundaries are cleanly fenced.
