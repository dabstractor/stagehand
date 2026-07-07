# P1.M4.T1.S2 — Design Decisions

Measure `PromptReserveTokens` upstream at the 6 diff call sites (FR3d/FR3i coupling seam). The git
layer owns the diff body + the numstat sizing; the PROMPT portion (system prompt + style examples +
user-payload stable part) is built AFTER the diff today, so its size is unknown inside the diff
function. This task measures the stable prompt portion's WORST-CASE token count BEFORE each diff call
and threads it into `opts.PromptReserveTokens` (the field P1.M1.T2.S2 left at zero). M4.T3's gate
consumes it; M4.T2's water-fill subtracts it (`body_budget = token_limit − skeleton − prompt − margin`).

All signatures quoted VERBATIM from on-disk contracts: `git.EstimateTokens` (S1, parallel), the 6
call sites (P1.M1.T2.S2, LANDED), the prompt builders (`internal/prompt/{system,payload,planner,
arbiter,format}.go`).

---

## §0 — Placement: `internal/prompt/reserve.go`, leaf-pure via INJECTED estimator

The helper needs (a) `git.EstimateTokens` (S1's single estimator) and (b) the prompt constants/builders
(`userInstructionReject`, `rejectionPreamble`, `BuildUserPayload`, etc. — most UNEXPORTED). Three homes
considered:

1. **`internal/prompt/reserve.go` importing `internal/git`** — uses unexported constants natively, but
   BREAKS prompt's stated "zero internal dependencies" value (planner.go: "The prompt package has zero
   internal dependencies; this keeps it that way."). prompt→git IS acyclic (verified: internal/git
   imports nothing internal), so it would compile — but it erodes a stated design invariant.
2. **A NEW `internal/diffbudget` package** (imports git + prompt) — keeps prompt leaf-pure, but a new
   package for 3 small functions is over-engineering (anti-pattern: "don't create new patterns").
3. **`internal/prompt/reserve.go` with an INJECTED estimator** — the helper takes `est func(string) int`
   (call sites pass `git.EstimateTokens`). prompt stays LEAF-PURE (no internal imports), uses the
   unexported constants/builders natively, no new package, and honors the contract's name
   (`promptReserveTokens`). **CHOSEN.**

The contract names the helper `promptReserveTokens(...)` ⇒ package prompt. Injecting the estimator
makes S1's "estimateTokens from S1" an EXPLICIT input (not a hidden import), preserves prompt's
independence, and keeps the "single estimator" rule enforceable by the PRP (all 6 sites pass
`git.EstimateTokens`). A named type `TokenEstimator func(s string) int` self-documents the seam.

## §1 — Three role-specific helpers (the contract's "planner path" lumping is a simplification)

The contract says: message path (generate.go, hook/exec.go, pkg/stagecoach.go) = systemPrompt + worst-
case rejection block + margin; "planner path" (decompose/planner.go, message.go, decompose.go) =
plannerSystemPrompt + examples + instruction + margin. But the LIVE code shows the decompose sites use
THREE different roles:

| Site | File:func | Role | Prompt builder | Growing block? |
|------|-----------|------|----------------|----------------|
| 1 | generate.go:CommitStaged | **message** | BuildSystemPrompt / BuildFallbackPrompt | YES — rejection list (FR32) |
| 2 | hook/exec.go:Run | **message** | hookSystemPrompt → BuildSystemPrompt | YES — rejection list |
| 3 | pkg/stagecoach.go:runPipeline | **message** | buildSysPrompt → BuildSystemPrompt (+ systemExtra) | YES — rejection list |
| 4 | decompose/planner.go:callPlanner | **planner** | BuildPlannerSystemPrompt | NO (1 retry, fixed retry-instr) |
| 5 | decompose/message.go:generateMessage | **message** | messageSystemPrompt → BuildSystemPrompt | YES — rejection list |
| 6 | decompose/decompose.go:runArbiterPhase | **arbiter** | BuildArbiterSystemPrompt (zero-arg) | NO (single call) |

So the CORRECT per-role reserve (the contract's INTENT — measure stable prompt + worst-case variable +
margin — applied faithfully): **MessageReserveTokens** (sites 1,2,3,5), **PlannerReserveTokens** (site 4),
**ArbiterReserveTokens** (site 6). The contract's "planner path" label for sites 5/6 was brevity; site 5
IS the message role (its dedupe loop appends `rejected` — message.go:121) and MUST use the message
formula (with the rejection block) or the reserve under-counts and the payload can exceed token_limit.

## §2 — The empty-diff trick: the payload builders take the diff as the verbatim TAIL

`BuildUserPayload(diff, context, rejected)`, `BuildPlannerUserPayload(diff, context, forcedCount)`, and
`BuildArbiterUserPayload(commits, leftoverDiff)` ALL append the diff as the verbatim final bytes (verified
in payload.go / planner.go / arbiter.go: "diff is the exact tail"). Therefore calling each with an EMPTY
diff (`""`) yields EXACTLY the non-diff overhead. The helpers exploit this:

- `MessageReserveTokens`: overhead = `BuildUserPayload("", context, worstRejected)` where worstRejected =
  `maxDuplicateRetries` synthetic subjects at ~`subjectTargetChars` each (the worst case — see §3).
- `PlannerReserveTokens`: overhead = `PlannerRetryInstruction + "\n\n" + BuildPlannerUserPayload("", context, forcedCount)`
  (the retry instruction is prepended on the 1 retry — include it as the worst case).
- `ArbiterReserveTokens`: overhead = `BuildArbiterUserPayload(commits, "")` (the commits/header block;
  the leftoverDiff is the diff tail).

This uses the REAL builders as the single source of truth for the prompt topology — NO constant
duplication (no re-deriving `userInstructionReject + "\n\n" + rejectionPreamble + ...` by hand). If the
PRD's §17.3/§17.5/§17.7 wording ever changes, the reserve tracks it automatically.

## §3 — The message reserve is a WORST-CASE, stable across retry attempts (the contract's test)

The message role's user payload grows per dedupe attempt: `BuildUserPayload(diff, context, rejected)`
appends one `- <subject>\n` line per rejected subject, up to `cfg.MaxDuplicateRetries` (default 3). But
`StagedDiff` is called ONCE, before the retry loop (generate.go:163 StagedDiff → :189 buildSystemPrompt →
the loop). So the reserve CANNOT be per-attempt — it must be the WORST CASE (all `maxDuplicateRetries`
slots filled), measured ONCE. `MessageReserveTokens` synthesizes `maxDuplicateRetries` subjects (each
`strings.Repeat("x", max(subjectTargetChars,1))`) and measures the rejection-path overhead. The result is
STABLE across attempts (it's a ceiling, not the current attempt's size) — this is the contract's explicit
test assertion ("reserve is stable across retry attempts"). The normal path (no rejection) is SMALLER, so
the worst-case reserve is a safe upper bound for every attempt.

Worst-case subject length: `subjectTargetChars` (the configured target the model aims for; default 50).
Real subjects can exceed it, but the `reserveSafetyMargin` (256 tokens ≈ 1024 chars, §6) absorbs
over-length subjects (even 3 × 2× target = 300 chars = 75 tokens ≪ 256) AND the FR29 `retryInstr`
preamble (~15 tokens, prepended on parse-fail retries) AND the chars/4-vs-chars/3 density gap.

## §4 — REORDER: build the system prompt BEFORE the diff call at every site

Today every site builds its system prompt (which fetches `RecentMessages` via git) AFTER the diff call:
- Site 1 generate.go: StagedDiff(:163) → … → buildSystemPrompt(:189).
- Site 2 hook/exec.go: StagedDiff(:104) → … → hookSystemPrompt(:131).
- Site 3 pkg/stagecoach.go: StagedDiff(:423) → … → buildSysPrompt(:447) → append systemExtra(:452).
- Site 4 planner.go: TreeDiff(:69) → plannerExamples + BuildPlannerSystemPrompt(:86).
- Site 5 message.go: TreeDiff(:71) → … → messageSystemPrompt(:95).
- Site 6 decompose.go: TreeDiff(:608) → runArbiter builds BuildArbiterSystemPrompt (zero-arg, arbiter.go:88).

To measure the reserve, the system prompt must exist BEFORE the diff call. So MOVE the system-prompt
build (examples fetch + builder) to precede the diff call, then measure the reserve, then call the diff
with `PromptReserveTokens` set. The contract explicitly endorses this: "REORDER so the fetch happens
before StagedDiff."

COST: on the empty-diff path (diff == ""), the examples fetch now runs before the empty-check returns
(generate.go:172 / hook/exec.go:114 / message.go etc.). This is a wasted `RecentMessages` call on the
nothing-to-commit path. It is (a) RARE — `CommitStaged`/`Run`/`generateMessage` are called after
`HasStagedChanges` already confirmed staged changes exist (the empty case is all-files-excluded etc.),
(b) CHEAP — `RecentMessages` is one `git log`, and (c) ACCEPTED by the contract ("measuring it anyway is
harmless"). No correctness impact: the empty-check still returns `ErrNothingToCommit`/`ErrNoOp`/etc.

The `recentSubjects` fetch (for dedupe, separate from the style-examples fetch) stays where it is — it is
NOT needed for the reserve. Only the system-prompt build moves.

## §5 — Uniform call sites: ALWAYS measure + set (even when token_limit == 0)

The contract: "When token_limit == 0 the reserve is unused (the legacy caps apply) — measuring it anyway
is harmless and keeps the call sites uniform." So NO `if cfg.TokenLimit > 0` branching at the 6 sites —
the reserve is unconditionally measured and assigned to `opts.PromptReserveTokens`. M4.T3's gate (the
NEXT subtask) decides whether to consume it (it's ignored when `token_limit == 0`). This keeps the 6 call
sites mechanically identical (measure → set), which is the whole point of the shared helper.

## §6 — TWO distinct margins — do NOT conflate

1. **`reserveSafetyMargin` (THIS task, = 256 tokens)** — the reserve helper's INTERNAL safety buffer.
   Inflates the prompt estimate to a worst-case upper bound. Absorbs: over-length rejected subjects, the
   FR29 `retryInstr` preamble, the chars/4-vs-chars/3 code-density gap, and any small per-attempt
   overhead the worst-case block doesn't explicitly enumerate. Named `reserveSafetyMargin` (unexported
   const in reserve.go) to distinguish it from…
2. **FR3i's body_budget `margin` (M4.T2, NOT this task)** — a SEPARATE term in
   `body_budget = token_limit − skeleton − prompt − margin`, applied in the diff layer's water-fill.
   This task does NOT touch it. The reserve (incl. its own 256 margin) IS the `prompt` term.

## §7 — Behavior-free (like P1.M1.T2.S2): existing tests stay green

The diff functions (`StagedDiff`/`TreeDiff`/`WorkingTreeDiff`) do NOT read `PromptReserveTokens` until
M4.T3's gate. Setting it now + reordering the system-prompt build changes ZERO diff output (the diff is
captured identically; the reserve flows into an unread field). So every golden diff test
(stagediff/treediff/workingtreediff) and every pipeline test (generate/hook/decompose/stagecoach) passes
UNCHANGED. The reorder changes the ORDER of git operations (`RecentMessages` before `StagedDiff`) but not
their RESULTS (real git, not a call-order-asserting mock) — so tests that assert on outcomes (not call
order) are unaffected. `go test ./...` staying green IS the regression proof.

## §8 — Site 6 (arbiter): use the in-package `convertArbiterCommits` to measure the exact overhead

`runArbiterPhase` (decompose.go:603) holds `commits []CommitInfo`. `runArbiter` (arbiter.go:79) converts
them via the unexported `convertArbiterCommits(commits)` → `([]ArbiterCommit, validSHAs)`. Since both
live in package `decompose`, `runArbiterPhase` CAN call `convertArbiterCommits(commits)` before the
TreeDiff to obtain `[]ArbiterCommit`, then `prompt.ArbiterReserveTokens(BuildArbiterSystemPrompt(),
arbiterCommits, git.EstimateTokens)`. `runArbiter` continues to call `convertArbiterCommits` internally
(UNCHANGED) — the double-conversion is a negligible pure-struct map (no git, no I/O). This gives the
EXACT arbiter overhead (commits + headers) rather than a guessed bound — important because the arbiter's
commits-info block can be substantial (each commit's message + diff-tree file list × up to maxCommits).

`BuildArbiterSystemPrompt()` is zero-arg (a fixed §17.7 constant) — trivial to call before the TreeDiff.

## §9 — Site 3 (pkg/stagecoach runPipeline): measure AFTER the systemExtra append

runPipeline appends the integrator's `SystemExtra` to the system prompt AFTER `buildSysPrompt`
(stagecoach.go:452: `sysPrompt += "\n\n" + systemExtra`). The reserve must include systemExtra (it's part
of the prompt the agent receives), so the measurement must happen AFTER the append. Order at site 3:
`buildSysPrompt` → `sysPrompt += "\n\n" + systemExtra` → `reserve := MessageReserveTokens(sysPrompt, …)`
→ `StagedDiff(… PromptReserveTokens: reserve)`. (The helper takes the already-built `sysPrompt` string,
so anything appended to it — systemExtra — is automatically included.)

## §10 — `max()` builtin: go 1.22 ⇒ available; guard `strings.Repeat` against ≤0

The module is `go 1.22` (S1 PRP), so the builtin `max(a,b)`/`min(a,b)` (Go 1.21+) is available. Use
`max(subjectTargetChars, 1)` before `strings.Repeat("x", …)` — `strings.Repeat` with a count ≤0 panics
(count==0 is allowed since Go 1.11, but negative panics; guard defensively against a misconfigured
negative `subjectTargetChars`). `maxDuplicateRetries < 0` ⇒ clamp to 0 (no rejection slots ⇒ normal
instruction overhead only).
