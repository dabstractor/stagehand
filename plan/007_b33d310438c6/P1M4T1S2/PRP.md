---
name: "P1.M4.T1.S2 — Measure PromptReserveTokens upstream at the 6 call sites (PRD §9.1 FR3d/FR3i): the FR3i coupling seam — measure the stable prompt portion's worst-case token count BEFORE each diff call and thread it into opts.PromptReserveTokens"
description: |

  FR3i's water-fill budget is `body_budget = token_limit − skeleton − prompt − margin`. The git layer
  owns the diff body + the numstat sizing; the "prompt" term (system prompt + style examples + the user-
  payload stable part) is built AFTER the diff returns today, so its size is unknown inside the diff
  function. This task closes that seam: at each of the 6 diff call sites, build the system prompt BEFORE
  the diff call, measure its worst-case token count (system prompt + worst-case user-payload overhead +
  safety margin) via the S1 estimator `git.EstimateTokens`, and set `opts.PromptReserveTokens`. M4.T3's
  gate consumes it; M4.T2's water-fill subtracts it. When `token_limit == 0` the reserve is unused but
  measured anyway (uniform call sites).

  CONTRACT (item_description §3, verbatim intent): provide a small SHARED helper to avoid 6× duplication.
  Concretely:
    - MESSAGE path (sites 1,2,3,5 — generate.go, hook/exec.go, pkg/stagecoach.go, decompose/message.go):
      reserve = EstimateTokens(sysPrompt) + EstimateTokens(worstCaseRejectionOverhead) + margin, where the
      worst case is the REJECTION path (userInstructionReject + context + rejectionPreamble +
      maxDuplicateRetries rejected-subject lines + rejectionEpilogue). The reserve is a WORST-CASE ceiling,
      stable across retry attempts (StagedDiff is called ONCE before the dedupe loop).
    - PLANNER path (site 4 — decompose/planner.go): reserve = EstimateTokens(plannerSystemPrompt) +
      EstimateTokens(forced-count line + plannerUserInstruction + context + PlannerRetryInstruction) + margin.
      No growing block (single retry, fixed retry instruction).
    - ARBITER path (site 6 — decompose/decompose.go runArbiterPhase): reserve = EstimateTokens(arbiterSystemPrompt)
      + EstimateTokens(commits + headers overhead) + margin. Single call, no growing block.

  DELIVERABLES (1 NEW helper file + 1 NEW test file + 6 surgical EDITS; nothing under internal/git touched
  — S1 owns EstimateTokens, M4.T2/T3 own the diff-function consumption):
    1. CREATE `internal/prompt/reserve.go`     — `package prompt` (LEAF-PURE: NO internal imports; the
       estimator is INJECTED as a `TokenEstimator` func param). Exports `TokenEstimator` type +
       `MessageReserveTokens` + `PlannerReserveTokens` + `ArbiterReserveTokens` + the unexported
       `measureReserve` core + `reserveSafetyMargin` (256) const.
    2. CREATE `internal/prompt/reserve_test.go` — `package prompt`. Pure table tests: worst-case stability
       across attempts, empty examples, monotonic in maxDuplicateRetries, the margin is included, the
       empty-diff overhead trick.
    3. EDIT 6 call sites — at each: (a) REORDER the system-prompt build to precede the diff call,
       (b) measure the role-appropriate reserve (passing `git.EstimateTokens`), (c) set
       `PromptReserveTokens: <reserve>` in the `git.StagedDiffOptions{...}` literal.

  THE 6 SITES (P1.M1.T2.S2, LANDED — each literal already carries TokenLimit + DiffContext; PromptReserveTokens
  is the ZERO this task wires):
    1. internal/generate/generate.go:163      StagedDiff  (cfg)            → MESSAGE role
    2. internal/hook/exec.go:104              StagedDiff  (cfg)            → MESSAGE role
    3. pkg/stagecoach/stagecoach.go:423         StagedDiff  (cfg)            → MESSAGE role (+ systemExtra)
    4. internal/decompose/planner.go:69       TreeDiff    (deps.Config)   → PLANNER role
    5. internal/decompose/message.go:71       TreeDiff    (deps.Config)   → MESSAGE role
    6. internal/decompose/decompose.go:608    TreeDiff    (deps.Config)   → ARBITER role

  SCOPE NOTE (the contract's "planner path" is a simplification, design §1): the contract lumps sites
  4/5/6 under "planner path", but the LIVE code shows site 5 (message.go) is the MESSAGE role (its dedupe
  loop appends `rejected` — message.go:121 `prompt.BuildUserPayload(diff, deps.Config.Context, rejected)`)
  and site 6 (decompose.go) is the ARBITER role (BuildArbiterSystemPrompt zero-arg). Applying the planner
  formula to site 5 would UNDER-COUNT (omit the rejection block) ⇒ payload could exceed token_limit. This
  PRP applies the contract's INTENT (measure stable prompt + worst-case variable + margin) FAITHFULLY per
  role: MessageReserveTokens (1,2,3,5), PlannerReserveTokens (4), ArbiterReserveTokens (6).

  SCOPE NOTE (leaf-pure prompt via INJECTED estimator, design §0): the helper needs `git.EstimateTokens`
  AND the prompt package's unexported constants/builders. Importing internal/git into prompt would compile
  (acyclic — internal/git imports nothing internal) but would BREAK prompt's STATED "zero internal
  dependencies" value (planner.go). Instead the helper takes `est TokenEstimator` (call sites pass
  `git.EstimateTokens`). prompt stays leaf-pure, uses its constants natively, no new package, honors the
  contract's name (`promptReserveTokens`). S1's "single estimator" rule is enforced by this PRP (all 6
  sites pass `git.EstimateTokens`).

  SCOPE BOUNDARY (what this does NOT do): NO water-fill solver/truncation (M4.T2); NO token-limit gate
  (M4.T3 — it reads opts.PromptReserveTokens + opts.TokenLimit and switches off legacy caps when >0); NO
  change to the diff functions or StagedDiffOptions struct (S1/M4.T2/T3 territory); NO config/CLI/docs
  surface (contract: "DOCS: none — internal seam"); NO edit to internal/git (S1's EstimateTokens is the
  frozen INPUT). This task MEASURES upstream and SETS the field — nothing reads it yet (behavior-free).

  INPUT (upstream — READ-ONLY contracts): `git.EstimateTokens(s string) int` (S1, parallel — ceil(runes/4),
  EXPORTED). The 6 call-site literals (P1.M1.T2.S2, LANDED — each has TokenLimit + DiffContext; PromptReserveTokens
  is zero). The prompt builders: `BuildSystemPrompt`/`BuildFallbackPrompt` (system.go), `BuildUserPayload`
  (payload.go), `BuildPlannerSystemPrompt`/`BuildPlannerUserPayload`/`PlannerRetryInstruction` (planner.go),
  `BuildArbiterSystemPrompt`/`BuildArbiterUserPayload`/`ArbiterCommit` (arbiter.go). `convertArbiterCommits`
  (decompose/arbiter.go, unexported, in-package for site 6). `config.Config` fields MaxDuplicateRetries /
  SubjectTargetChars / Context / Format / Locale.

  OUTPUT (downstream consumer): M4.T3 (the gate) reads `opts.PromptReserveTokens` + `opts.TokenLimit`;
  when TokenLimit > 0 it passes the reserve into M4.T2's `body_budget = token_limit − skeleton − reserve − margin`.

  ⚠️ REORDER every site's system-prompt build to PRECEDE the diff call (design §4). The empty-diff path
     then fetches RecentMessages before the empty-check returns — a rare, cheap, accepted wasted call.
  ⚠️ The reserve is a WORST-CASE ceiling measured ONCE (StagedDiff is called once, before the dedupe loop);
     it does NOT grow per attempt. The message path synthesizes maxDuplicateRetries rejected subjects (§3).
  ⚠️ ALWAYS measure + set (even when token_limit==0) — uniform call sites, no `if TokenLimit>0` branching (§5).
  ⚠️ Do NOT import internal/git into internal/prompt — inject the estimator (§0). Do NOT edit internal/git
     (S1 owns EstimateTokens; M4.T2/T3 own the diff-function consumption of PromptReserveTokens).

  Deliverable: 2 NEW files + 6 surgical edits. `go build/vet/gofmt` clean; `go test ./...` green (the
  field is unread until M4.T3 — behavior-free, §7); `git.EstimateTokens` consumed at all 6 sites.

---

## Goal

**Feature Goal**: Land the FR3i coupling seam — measure the stable prompt portion's worst-case token count
UPSTREAM (before each diff call) at all 6 diff call sites and thread it into `opts.PromptReserveTokens`,
via a single shared helper family (`MessageReserveTokens` / `PlannerReserveTokens` / `ArbiterReserveTokens`)
that uses the S1 estimator `git.EstimateTokens`. After this task, `opts.PromptReserveTokens` carries a safe
worst-case upper bound on the non-diff prompt size at every diff call, ready for M4.T3's gate and M4.T2's
water-fill (`body_budget = token_limit − skeleton − reserve − margin`).

**Deliverable** (1 NEW helper + 1 NEW test + 6 surgical edits; NO edits under internal/git):
1. `internal/prompt/reserve.go` — `package prompt`, LEAF-PURE (no internal imports; estimator injected).
   `type TokenEstimator func(s string) int`; `func MessageReserveTokens(sysPrompt string, maxDuplicateRetries,
   subjectTargetChars int, context string, est TokenEstimator) int`; `func PlannerReserveTokens(sysPrompt
   string, forcedCount int, context string, est TokenEstimator) int`; `func ArbiterReserveTokens(sysPrompt
   string, commits []ArbiterCommit, est TokenEstimator) int`; unexported `measureReserve` + `reserveSafetyMargin`.
2. `internal/prompt/reserve_test.go` — `package prompt`. Pure table tests (no git, no I/O).
3. EDIT 6 sites — reorder the system-prompt build before the diff call + measure the role-appropriate
   reserve (passing `git.EstimateTokens`) + set `PromptReserveTokens` in the `git.StagedDiffOptions{...}` literal.

**Success Definition**: all 6 literals carry a non-zero `PromptReserveTokens` computed from the role-
appropriate stable prompt inputs via `git.EstimateTokens`; the message-path reserve is a stable worst-case
(≥ every per-attempt payload's non-diff overhead); `go build/vet/gofmt` clean; `go test ./...` green (the
field is unread until M4.T3, so diff output is byte-identical — existing tests pass unchanged); only the 2
new files + the 6 edited call-site files differ.

## User Persona

**Target User**: The downstream subtasks M4.T3 (the token-limit gate) and M4.T2 (the FR3i water-fill),
which consume `opts.PromptReserveTokens`. Transitively: every user who sets `token_limit` (PRD §9.1 FR3d)
so a large diff fits their model's context window — the reserve is the "prompt" term that, subtracted from
`token_limit`, leaves the `body_budget` the water-fill allocates across files.

**Use Case**: A user sets `token_limit = 120000`. At each diff call site, stagecoach measures the prompt
it's ABOUT to send (system prompt + style examples + worst-case rejection block + margin) ≈ a few thousand
tokens, sets `opts.PromptReserveTokens`, and the diff layer (M4.T2/T3) gives the REMAINDER to the diff body
via water-fill. Without this seam, the diff layer wouldn't know how much budget the prompt consumes.

**User Journey**: (internal) call site builds the system prompt → `prompt.<Role>ReserveTokens(sysPrompt, …,
git.EstimateTokens)` → an int → set on `opts.PromptReserveTokens` → the diff function (unread until M4.T3).

**Pain Points Addressed**: Closes the FR3i timing gap — the prompt is built after the diff today, so its
size was unknowable inside the diff function. A shared, role-aware helper avoids 6× duplication and the
divergence that would make `body_budget` incoherent.

## Why

- **It IS the FR3i prompt term.** `body_budget = token_limit − skeleton − prompt − margin`; "prompt" must
  be measured upstream (the diff layer can't see it). This task is that measurement, at every call site.
- **One shared estimator, one shared helper.** S1 landed `git.EstimateTokens` (chars/4) as the SINGLE
  measure; M4.T2's water-fill sizing uses the same. This task's helper feeds the SAME measure into the
  prompt term, so a "token" upstream == a "token" downstream (no second formula, no drift).
- **Worst-case ceiling, measured once.** StagedDiff is called ONCE before the dedupe loop; the rejection
  list grows per attempt. A worst-case reserve (maxDuplicateRetries slots) measured once is a safe upper
  bound for every attempt — no per-attempt re-measurement, no mid-run budget reset.
- **Behavior-free, minimal blast radius.** The diff functions don't read `PromptReserveTokens` until M4.T3.
  Setting it + reordering the prompt build changes no diff output → existing tests stay green (§7).

## What

A `reserve.go` with a `TokenEstimator` type + three role-specific exported functions over a shared
`measureReserve` core, and a `reserve_test.go` with pure table tests. The three functions exploit the fact
that the payload builders append the diff as the verbatim TAIL — calling each with an EMPTY diff yields the
exact non-diff overhead (single source of truth — no hand-rebuilt constant concatenation). At the 6 call
sites, the system-prompt build is moved before the diff call, the role-appropriate reserve is measured
(passing `git.EstimateTokens`), and `PromptReserveTokens` is set on the struct literal.

### Success Criteria

- [ ] `internal/prompt/reserve.go` exists, `package prompt`, imports ONLY stdlib (`strings`) — NO internal
      imports (the estimator is the `est TokenEstimator` param). Defines `type TokenEstimator func(s string) int`.
- [ ] `MessageReserveTokens(sysPrompt, maxDuplicateRetries, subjectTargetChars, context, est)` returns
      `measureReserve(sysPrompt, BuildUserPayload("", context, worstRejected), est)` where worstRejected has
      `max(maxDuplicateRetries,0)` entries each `strings.Repeat("x", max(subjectTargetChars,1))`.
- [ ] `PlannerReserveTokens(sysPrompt, forcedCount, context, est)` returns `measureReserve(sysPrompt,
      PlannerRetryInstruction + "\n\n" + BuildPlannerUserPayload("", context, forcedCount), est)`.
- [ ] `ArbiterReserveTokens(sysPrompt, commits, est)` returns `measureReserve(sysPrompt,
      BuildArbiterUserPayload(commits, ""), est)`.
- [ ] `measureReserve(sysPrompt, overhead, est) = est(sysPrompt) + est(overhead) + reserveSafetyMargin`
      (reserveSafetyMargin = 256; documented as DISTINCT from FR3i's body_budget margin).
- [ ] All 6 production literals carry a non-zero `PromptReserveTokens` set from the role-appropriate helper:
      sites 1,2,3,5 → Message; site 4 → Planner; site 6 → Arbiter. Each passes `git.EstimateTokens`.
- [ ] Each site's system-prompt build is REORDERED to precede its diff call (so the reserve can be measured).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test ./...` green (behavior-free — the field
      is unread until M4.T3; diff output byte-identical).
- [ ] No edit to `internal/git/*` (S1's EstimateTokens is the frozen input), `StagedDiffOptions`, the diff
      functions, or any config/CLI/docs surface.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact helper signatures
+ the copy-ready `reserve.go` skeleton (below), the empty-diff trick (§2), the per-site role mapping + the
verbatim current code at each of the 6 sites (quoted below + in research/design-decisions.md), the S1
`git.EstimateTokens` contract (chars/4 ceiling), the leaf-pure-via-injection decision (§0), and the
reorder rationale (§4). No water-fill/gate/diff-function knowledge required (those are explicitly M4.T2/T3).

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions
- docfile: plan/007_b33d310438c6/P1M4T1S2/research/design-decisions.md
  why: the 10 decisions. §0 (placement internal/prompt/reserve.go + INJECTED estimator — leaf-pure; rejected
       prompt→git import + new diffbudget pkg), §1 (3 roles — the contract's "planner path" lumping of sites
       5/6 is a simplification; per-role mapping table), §2 (the empty-diff trick — payload builders take
       diff as the verbatim tail ⇒ empty diff = exact overhead; single source of truth), §3 (worst-case
       stable across attempts — the contract's test; maxDuplicateRetries synthetic subjects), §4 (REORDER
       every site's prompt build before the diff; the rare wasted RecentMessages call is accepted), §5
       (uniform — always measure+set, no token_limit>0 branching), §6 (TWO margins — reserveSafetyMargin=256
       HERE vs FR3i's body_budget margin in M4.T2), §7 (behavior-free — field unread until M4.T3), §8 (site 6
       uses in-package convertArbiterCommits for exact arbiter overhead), §9 (site 3 measures AFTER the
       systemExtra append), §10 (max() builtin; guard strings.Repeat).
  critical: §0 (inject the estimator — do NOT import git into prompt), §1 (site 5 is MESSAGE not planner),
       §2 (empty-diff trick — don't hand-rebuild the constants), §4 (reorder — the prompt must exist before
       the diff call), §6 (256 is the RESERVE margin, distinct from FR3i's margin).

# MUST READ — the S1 INPUT contract (the estimator this task consumes)
- docfile: plan/007_b33d310438c6/P1M4T1S1/PRP.md
  section: "Data models" (internal/git/tokens.go: `func EstimateTokens(s string) int` = ceilDiv(utf8.RuneCountInString(s), 4);
       EXPORTED; ceiling; rune-based; the SINGLE estimator).
  why: this task calls `git.EstimateTokens` at all 6 sites (passed as the `est` param). S1 is PARALLEL
       (being implemented) — treat its `EstimateTokens(s string) int` signature as a FROZEN contract. Do NOT
       introduce a second estimator or a chars/3 variant (S1's "single formula" rule).
  critical: the helper's `est TokenEstimator` param receives `git.EstimateTokens` at every call site. Do NOT
       reimplement chars/4 inside prompt (that would violate S1's single-estimator rule + break leaf-purity).

# MUST READ — the 6 call-site contract (PromptReserveTokens was left ZERO for THIS task)
- docfile: plan/007_b33d310438c6/P1M1T2S2/PRP.md
  section: the 6-site table + the literal edits (TokenLimit + DiffContext mapped; "PromptReserveTokens is NOT
       set at any site (left zero; M4.T1.S2 wires it)").
  why: confirms the 6 sites, the two variable shapes (`cfg` for 1,2,3; `deps.Config` for 4,5,6), and that
       PromptReserveTokens is the zero this task fills. The existing 4 fields (MaxDiffBytes/MaxMDLines/
       BinaryExtensions/Excludes) + TokenLimit + DiffContext stay byte-identical; this task APPENDS
       PromptReserveTokens + reorders the prompt build.
  critical: do NOT touch TokenLimit/DiffContext (T2.S2 wired them) or the existing 4 fields. Only ADD
       PromptReserveTokens + the reorder.

# MUST READ — the FR3i coupling seam (the authoritative architecture)
- docfile: plan/007_b33d310438c6/architecture/system_context.md
  section: "## 5. The FR3i coupling seam (prompt-reserve)".
  why: documents the seam: PromptReserveTokens on StagedDiffOptions; each call site measures the STABLE
       prompt portion (system-prompt header + RecentMessages examples + user-instruction + worst-case
       rejection-block = maxDuplicateRetries × ~max-rejection-line-len + safety margin) BEFORE the diff
       call, using chars/4. "When token_limit == 0, PromptReserveTokens is ignored and the legacy caps apply."
  critical: the "measure BEFORE calling the diff function" + "worst-case rejection-block" + "stable prompt
       portion" framing IS this task's spec.

- docfile: plan/007_b33d310438c6/architecture/diff_capture_touchmap.md
  section: "## 5. Payload consumption + token estimation" + "## 2. StagedDiffOptions struct + ALL call sites".
  why: §5's "Open coupling question" flags that StagedDiff is called ONCE before the retry loop and the
       rejection list grows per-attempt ⇒ needs a worst-case reserve (this task). §2 is the 6-site map.
  critical: §5 confirms "The cleanest seam is a PromptReserveTokens field on StagedDiffOptions measured
       upstream from BuildUserPayload's constants + RecentMessages output" — exactly this task.

# MUST READ — the prompt builders this task's helper calls (with empty diff)
- file: internal/prompt/payload.go   (READ ONLY)
  section: `BuildUserPayload(diff, context string, rejected []string) string` — appends `diff` VERBATIM as
       the tail. Constants `userInstruction`/`userInstructionReject`/`rejectionPreamble`/`rejectionEpilogue`
       (all UNEXPORTED). The rejection path: userInstructionReject + context + rejectionPreamble + per-subject
       `- <s>\n` + rejectionEpilogue, then diff.
  why: MessageReserveTokens calls `BuildUserPayload("", context, worstRejected)` — the empty diff isolates
       the non-diff overhead (the worst-case rejection block). NO need to hand-rebuild the constants.
  gotcha: the constants are unexported ⇒ the helper MUST live in package prompt (it does). Don't export them.

- file: internal/prompt/planner.go   (READ ONLY)
  section: `BuildPlannerUserPayload(diff, context string, forcedCount int) string` (diff verbatim tail) +
       `PlannerRetryInstruction` (EXPORTED const — prepended on the 1 retry).
  why: PlannerReserveTokens calls `BuildPlannerUserPayload("", context, forcedCount)` + prepends
       `PlannerRetryInstruction + "\n\n"` (the worst case includes the retry preamble).

- file: internal/prompt/arbiter.go   (READ ONLY)
  section: `BuildArbiterSystemPrompt() string` (zero-arg) + `BuildArbiterUserPayload(commits []ArbiterCommit,
       leftoverDiff string) string` (leftoverDiff verbatim tail) + `type ArbiterCommit struct{SHA, Subject
       string; Files []string}`.
  why: ArbiterReserveTokens calls `BuildArbiterUserPayload(commits, "")` — empty leftoverDiff isolates the
       commits+headers overhead. BuildArbiterSystemPrompt() is zero-arg (trivial to call before the diff).

- file: internal/prompt/system.go   (READ ONLY)
  section: `BuildSystemPrompt(examples, hasMultiline, subjectTarget, format, locale)` + `BuildFallbackPrompt
       (subjectTarget, format, locale)`.
  why: confirms the system-prompt builders the 4 message-role sites call (via their local builders). The
       helper takes the ALREADY-BUILT sysPrompt string, so anything in it (header + examples + format
       scaffold + locale line + systemExtra at site 3) is automatically measured.

# MUST READ — the 6 call sites (verbatim current code + role per site)
- file: internal/generate/generate.go   (EDIT — site 1, MESSAGE role, cfg)
  section: CommitStaged. StagedDiff literal at ~:163; `buildSystemPrompt(ctx, deps.Git, cfg, isUnborn)` at
       ~:189 (Step 4, AFTER WriteTree). Empty-check `if diff == ""` at ~:172.
  why: REORDER — move the `sysPrompt, err := buildSystemPrompt(...)` block to BEFORE StagedDiff; measure
       `prompt.MessageReserveTokens(sysPrompt, cfg.MaxDuplicateRetries, cfg.SubjectTargetChars, cfg.Context,
       git.EstimateTokens)`; set `PromptReserveTokens:` in the literal. The empty-check stays after StagedDiff.
  gotcha: buildSystemPrompt needs `isUnborn` (from RevParseHEAD, Step 1 — already before StagedDiff). ✓
       recentSubjects (separate, for dedupe) stays where it is — NOT needed for the reserve.

- file: internal/hook/exec.go   (EDIT — site 2, MESSAGE role, cfg)
  section: Run. StagedDiff literal at ~:104; `hookSystemPrompt(ctx, deps.Git, cfg, isUnborn)` at ~:131
       (which does CommitCount + RecentMessages + BuildSystemPrompt). Empty-check `if diff == ""` → ErrNoOp.
  why: REORDER hookSystemPrompt before StagedDiff; measure MessageReserveTokens; set PromptReserveTokens.

- file: pkg/stagecoach/stagecoach.go   (EDIT — site 3, MESSAGE role, cfg + systemExtra)
  section: runPipeline. StagedDiff literal at ~:423; `buildSysPrompt(...)` at ~:447; `sysPrompt += "\n\n" +
       systemExtra` at ~:452.
  why: REORDER buildSysPrompt + the systemExtra append to BEFORE StagedDiff; measure MessageReserveTokens
       AFTER the systemExtra append (so the reserve includes systemExtra, design §9); set PromptReserveTokens.
  gotcha: pkg/stagecoach already imports internal/git (uses git.StagedDiffOptions/git.New) — git.EstimateTokens
       resolves with no new import.

- file: internal/decompose/planner.go   (EDIT — site 4, PLANNER role, deps.Config)
  section: callPlanner. TreeDiff literal at ~:69; `plannerExamples(ctx, deps.Git, isUnborn)` +
       `BuildPlannerSystemPrompt(examples, deps.Config.Format, deps.Config.Locale)` at ~:86.
  why: REORDER plannerExamples + BuildPlannerSystemPrompt to BEFORE TreeDiff; measure
       `prompt.PlannerReserveTokens(sysPrompt, forcedCount, deps.Config.Context, git.EstimateTokens)`;
       set PromptReserveTokens.
  gotcha: callPlanner's signature already has `forcedCount` — pass it to PlannerReserveTokens (the forced-
       count directive line is part of the worst-case payload).

- file: internal/decompose/message.go   (EDIT — site 5, MESSAGE role, deps.Config)
  section: generateMessage. TreeDiff literal at ~:71; `messageSystemPrompt(ctx, deps.Git, deps.Config,
       isUnborn)` at ~:95 (AFTER RevParseHEAD at ~:89). Dedupe loop appends `rejected` (message.go:121).
  why: REORDER messageSystemPrompt to BEFORE TreeDiff (it needs isUnborn — derive it BEFORE the TreeDiff via
       RevParseHEAD, which generateMessage already does at ~:89; move BOTH before TreeDiff); measure
       `prompt.MessageReserveTokens(sysPrompt, deps.Config.MaxDuplicateRetries, deps.Config.SubjectTargetChars,
       deps.Config.Context, git.EstimateTokens)`; set PromptReserveTokens.
  gotcha: site 5 is the MESSAGE role (NOT planner) — its dedupe loop grows `rejected`, so the message worst-
       case (with the rejection block) is correct here (design §1).

- file: internal/decompose/decompose.go   (EDIT — site 6, ARBITER role, deps.Config)
  section: runArbiterPhase (signature: `commits []CommitInfo, chainData []ChainEntry, tStart string`).
       TreeDiff literal at ~:608; `runArbiter(ctx, deps, commits, leftoverDiff)` at ~:620 (which calls
       `convertArbiterCommits(commits)` at arbiter.go:84 + BuildArbiterSystemPrompt zero-arg at arbiter.go:88).
  why: BEFORE the TreeDiff, call `arbiterCommits, _ := convertArbiterCommits(commits)` (in-package,
       design §8) + `arbiterSys := prompt.BuildArbiterSystemPrompt()`; measure
       `prompt.ArbiterReserveTokens(arbiterSys, arbiterCommits, git.EstimateTokens)`; set PromptReserveTokens.
  gotcha: convertArbiterCommits returns `(arbiterCommits, validSHAs)` — ignore validSHAs for the reserve.
       runArbiter continues to call convertArbiterCommits internally (UNCHANGED) — double-conversion is a
       negligible pure-struct map.

# External references
- url: https://pkg.go.dev/unicode/utf8#RuneCountInString
  why: (informational — already used by S1's EstimateTokens) confirms rune-counting for token estimation.
       This task does NOT call it directly (it calls git.EstimateTokens); noted for context only.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/tokens.go             # P1.M4.T1.S1 (parallel) — EstimateTokens(s) int = ceil(runes/4). READ ONLY (the INPUT).
internal/prompt/
  system.go                        # BuildSystemPrompt / BuildFallbackPrompt. READ ONLY (called by sites' local builders).
  payload.go                       # BuildUserPayload + unexported rejection constants. READ ONLY (helper calls with empty diff).
  planner.go                       # BuildPlannerUserPayload + PlannerRetryInstruction. READ ONLY.
  arbiter.go                       # BuildArbiterSystemPrompt (zero-arg) + BuildArbiterUserPayload + ArbiterCommit. READ ONLY.
  format.go                        # withLocale / formatScaffoldBody (folded into the built sysPrompt). READ ONLY.
  reserve.go                       # *** CREATE *** — TokenEstimator + Message/Planner/ArbiterReserveTokens + measureReserve + margin.
  reserve_test.go                  # *** CREATE *** — pure table tests.
internal/generate/generate.go      # EDIT (site 1) — reorder buildSystemPrompt before StagedDiff + set PromptReserveTokens.
internal/hook/exec.go              # EDIT (site 2) — reorder hookSystemPrompt before StagedDiff + set PromptReserveTokens.
pkg/stagecoach/stagecoach.go         # EDIT (site 3) — reorder buildSysPrompt(+systemExtra) before StagedDiff + set PromptReserveTokens.
internal/decompose/planner.go      # EDIT (site 4) — reorder plannerExamples+BuildPlannerSystemPrompt before TreeDiff + set PromptReserveTokens.
internal/decompose/message.go      # EDIT (site 5) — reorder messageSystemPrompt before TreeDiff + set PromptReserveTokens.
internal/decompose/decompose.go    # EDIT (site 6) — convertArbiterCommits+BuildArbiterSystemPrompt before TreeDiff + set PromptReserveTokens.
internal/decompose/arbiter.go      # READ ONLY — convertArbiterCommits (in-package, site 6 reuses it).
go.mod / go.sum                    # UNCHANGED (stdlib strings only; no new deps).
```

### Desired Codebase tree with files to be added/changed

```bash
internal/prompt/reserve.go          # NEW — TokenEstimator type + Message/Planner/ArbiterReserveTokens + measureReserve + reserveSafetyMargin.
internal/prompt/reserve_test.go     # NEW — pure table tests (worst-case stability, empty examples, monotonic, margin, empty-diff trick).
internal/generate/generate.go       # MODIFIED — site 1: reorder + MessageReserveTokens + PromptReserveTokens.
internal/hook/exec.go               # MODIFIED — site 2: reorder + MessageReserveTokens + PromptReserveTokens.
pkg/stagecoach/stagecoach.go          # MODIFIED — site 3: reorder (incl systemExtra) + MessageReserveTokens + PromptReserveTokens.
internal/decompose/planner.go       # MODIFIED — site 4: reorder + PlannerReserveTokens + PromptReserveTokens.
internal/decompose/message.go       # MODIFIED — site 5: reorder + MessageReserveTokens + PromptReserveTokens.
internal/decompose/decompose.go     # MODIFIED — site 6: convertArbiterCommits + ArbiterReserveTokens + PromptReserveTokens.
# go.mod/go.sum UNCHANGED. internal/git UNCHANGED (S1's territory). StagedDiffOptions/diff-functions UNCHANGED (M4.T2/T3).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (inject the estimator; do NOT import internal/git into prompt, design §0): prompt is a LEAF
//   package by stated design (planner.go: "zero internal dependencies"). The helper needs git.EstimateTokens
//   AND prompt's unexported constants. Resolution: take `est TokenEstimator` as a PARAM (call sites pass
//   git.EstimateTokens). prompt stays leaf-pure, uses its constants natively, no new package. Do NOT add
//   `import "github.com/dustin/stagecoach/internal/git"` to reserve.go.

// CRITICAL (site 5 is MESSAGE, not planner, design §1): the contract lumps decompose sites 4/5/6 under
//   "planner path", but message.go:generateMessage is the MESSAGE role (its loop appends `rejected` —
//   message.go:121). Use MessageReserveTokens for site 5 (with the rejection block). Using the planner
//   formula would under-count ⇒ the payload could exceed token_limit on a duplicate-retry.

// CRITICAL (the empty-diff trick, design §2): BuildUserPayload/BuildPlannerUserPayload/BuildArbiterUserPayload
//   append the diff as the VERBATIM tail. Calling each with diff="" yields the EXACT non-diff overhead. USE
//   THIS — do NOT hand-rebuild "userInstructionReject + ... + rejectionPreamble + ...". The builders are the
//   single source of truth; if §17.3/§17.5/§17.7 wording changes, the reserve tracks it automatically.

// CRITICAL (worst-case, measured ONCE, design §3): StagedDiff/TreeDiff is called ONCE before the dedupe loop;
//   the rejection list grows per attempt. The reserve is the WORST CASE (maxDuplicateRetries rejected
//   subjects), measured once before the diff call. It is STABLE across attempts (a ceiling, not per-attempt).
//   This is the contract's explicit test assertion.

// CRITICAL (REORDER every site, design §4): each site builds its system prompt AFTER the diff today. Move it
//   BEFORE the diff so the reserve can be measured. The empty-diff path then runs RecentMessages before the
//   empty-check returns — a rare, cheap, accepted wasted call (gated upstream by HasStagedChanges).

// CRITICAL (uniform call sites, design §5): ALWAYS measure + set PromptReserveTokens, even when token_limit==0
//   (the reserve is unused there but measuring keeps the 6 sites mechanically identical). NO `if cfg.TokenLimit>0`
//   branching at the sites. M4.T3's gate decides whether to consume it.

// CRITICAL (TWO margins — don't conflate, design §6): reserveSafetyMargin (256, THIS task) inflates the prompt
//   estimate to a worst-case upper bound (absorbs over-length subjects + FR29 retryInstr + chars/4-vs-chars/3
//   density). FR3i's body_budget margin (M4.T2) is a SEPARATE term. This task does NOT touch FR3i's margin.

// GOTCHA (site 3 measures AFTER systemExtra, design §9): runPipeline appends systemExtra to sysPrompt AFTER
//   buildSysPrompt. Measure the reserve AFTER the append so systemExtra is included. The helper takes the
//   already-built sysPrompt string, so this is just ordering the measure call after the append.

// GOTCHA (site 6 uses in-package convertArbiterCommits, design §8): runArbiterPhase is in package decompose,
//   so it can call the unexported convertArbiterCommits(commits) before the TreeDiff to get []ArbiterCommit.
//   runArbiter keeps its own convertArbiterCommits call (UNCHANGED) — the double-conversion is negligible.

// GOTCHA (max() builtin + strings.Repeat guard, design §10): go 1.22 ⇒ builtin max/min available. Use
//   max(subjectTargetChars, 1) before strings.Repeat (negative count panics; 0 yields "" which is fine but
//   clamp to 1 for a meaningful worst-case subject). Clamp maxDuplicateRetries to ≥0.

// GOTCHA (behavior-free, design §7): the diff functions do NOT read PromptReserveTokens until M4.T3. Setting
//   it + reordering the prompt build changes NO diff output. Existing golden tests + pipeline tests pass
//   UNCHANGED. The reorder changes git-operation ORDER (RecentMessages before StagedDiff) but not results.

// GOTCHA (recentSubjects ≠ RecentMessages): each site fetches recentSubjects (for dedupe) SEPARATELY from
//   the style examples (RecentMessages, inside the system-prompt builder). Only the system-prompt build
//   moves; recentSubjects stays where it is (it's not needed for the reserve).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/prompt/reserve.go
package prompt

import "strings"

// TokenEstimator estimates the token count of a string. The canonical implementation is git.EstimateTokens
// (ceil(runes/4), P1.M4.T1.S1) — the SINGLE model-agnostic estimator. It is INJECTED (not imported) so the
// prompt package stays dependency-free (its stated design value); every call site passes git.EstimateTokens.
type TokenEstimator func(s string) int

// reserveSafetyMargin is the FR3d/FR3i prompt-reserve safety buffer (in tokens). It inflates the prompt
// estimate to a worst-case upper bound so body_budget = token_limit − skeleton − reserve − margin (FR3i,
// M4.T2) never under-reserves the prompt. It absorbs: over-length rejected subjects (the worst-case block
// uses SubjectTargetChars; real subjects can exceed it), the FR29 retryInstruction preamble (~15 tokens,
// prepended on parse-fail retries and not separately enumerated), the chars/4-vs-chars/3 code-density gap,
// and any small per-attempt overhead. DISTINCT from FR3i's body_budget `margin` (M4.T2's separate term).
const reserveSafetyMargin = 256

// measureReserve is the shared core: est(sysPrompt) + est(overhead) + reserveSafetyMargin. The three
// role-specific helpers build their worst-case `overhead` string (the non-diff user-payload portion) and
// delegate here. `overhead` is the user payload MINUS the diff (the diff is the variable part the water-fill
// allocates; the reserve bounds everything ELSE).
func measureReserve(sysPrompt, overhead string, est TokenEstimator) int {
	return est(sysPrompt) + est(overhead) + reserveSafetyMargin
}

// MessageReserveTokens computes the worst-case prompt reserve for the MESSAGE role (FR3d/FR3i). sysPrompt is
// the ALREADY-BUILT system prompt (BuildSystemPrompt/BuildFallbackPrompt output — header + style examples +
// format scaffold + locale line; at pkg/stagecoach it includes the appended SystemExtra). The message user
// payload's worst case is the REJECTION path (FR32): it grows per dedupe attempt up to maxDuplicateRetries
// rejected subjects. Because the diff is captured ONCE before the dedupe loop, the reserve is the WORST CASE
// (all maxDuplicateRetries slots filled), measured once — stable across attempts (a ceiling, not per-attempt).
//
// The overhead is built via BuildUserPayload with an EMPTY diff (the builder appends diff as the verbatim
// tail ⇒ empty diff isolates the exact non-diff overhead) and a worst-case rejected slice (maxDuplicateRetries
// subjects at ~subjectTargetChars each). The normal path (no rejection) is smaller, so this is a safe upper
// bound for every attempt. reserveSafetyMargin absorbs over-length subjects + the FR29 retryInstruction.
//
// Consumers: generate.CommitStaged, hook.Run, pkg/stagecoach.runPipeline, decompose.generateMessage.
func MessageReserveTokens(sysPrompt string, maxDuplicateRetries, subjectTargetChars int, context string, est TokenEstimator) int {
	n := maxDuplicateRetries
	if n < 0 {
		n = 0
	}
	subjLen := max(subjectTargetChars, 1)
	worstRejected := make([]string, n)
	for i := range worstRejected {
		worstRejected[i] = strings.Repeat("x", subjLen)
	}
	overhead := BuildUserPayload("", context, worstRejected) // empty diff ⇒ pure overhead (worst-case rejection path)
	return measureReserve(sysPrompt, overhead, est)
}

// PlannerReserveTokens computes the worst-case prompt reserve for the PLANNER role (FR3d/FR3i). sysPrompt is
// BuildPlannerSystemPrompt's output. The planner has ONE retry (PlannerRetryInstruction prepended on parse
// failure) and NO growing rejection block, so the worst case = the retry preamble + the (possibly forced-
// count) instruction + context. The overhead is built via BuildPlannerUserPayload with an empty diff, with
// PlannerRetryInstruction prepended (the worst case includes the retry).
//
// Consumer: decompose.callPlanner.
func PlannerReserveTokens(sysPrompt string, forcedCount int, context string, est TokenEstimator) int {
	overhead := PlannerRetryInstruction + "\n\n" + BuildPlannerUserPayload("", context, forcedCount)
	return measureReserve(sysPrompt, overhead, est)
}

// ArbiterReserveTokens computes the worst-case prompt reserve for the ARBITER role (FR3d/FR3i). sysPrompt is
// BuildArbiterSystemPrompt()'s output (zero-arg §17.7 constant). The arbiter runs ONCE (no growing block);
// its non-diff overhead is the commits + headers block (BuildArbiterUserPayload with an empty leftoverDiff
// isolates it). `commits` is the converted []ArbiterCommit (the caller obtains it via convertArbiterCommits).
//
// Consumer: decompose.runArbiterPhase.
func ArbiterReserveTokens(sysPrompt string, commits []ArbiterCommit, est TokenEstimator) int {
	overhead := BuildArbiterUserPayload(commits, "") // empty leftoverDiff ⇒ pure overhead (commits + headers)
	return measureReserve(sysPrompt, overhead, est)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/prompt/reserve.go (the helper family — leaf-pure, injected estimator)
  - FILE: NEW internal/prompt/reserve.go. PACKAGE: `package prompt`. IMPORT: `strings` ONLY (NO internal imports).
  - DEFINE: `type TokenEstimator func(s string) int`; `const reserveSafetyMargin = 256`; unexported
      `func measureReserve(sysPrompt, overhead string, est TokenEstimator) int`; exported
      `MessageReserveTokens` / `PlannerReserveTokens` / `ArbiterReserveTokens` — EXACTLY as in "Data models".
  - DOC COMMENTS on every exported symbol cite FR3d/FR3i, the worst-case rationale, the empty-diff trick,
      the single-estimator rule (pass git.EstimateTokens), and the reserveSafetyMargin-vs-FR3i-margin
      distinction (so future readers don't conflate them or import git into prompt).
  - GOTCHA: do NOT `import "github.com/dustin/stagecoach/internal/git"` — the estimator is the `est` param.
      Use `max(n, 0)` / `max(subjectTargetChars, 1)` (go 1.22 builtin). The helpers call the SAME-PACKAGE
      builders (BuildUserPayload/BuildPlannerUserPayload/BuildArbiterUserPayload) with empty diff.
  - RUN: gofmt -w ; go build ./internal/prompt/ → exit 0.

Task 2: CREATE internal/prompt/reserve_test.go (pure table tests — no git, no I/O)
  - FILE: NEW internal/prompt/reserve_test.go. PACKAGE: `package prompt` (white-box — can use the unexported
      reserveSafetyMargin + measureReserve). IMPORT: `testing` (+ stdlib). Use a LOCAL estimator
      `est := func(s string) int { return (len([]rune(s)) + 3) / 4 }` (a chars/4 ceiling, mirroring S1 — do
      NOT import internal/git here; the test is pure). Hardcode expected ints (don't derive from the helper).
  - CASES:
      * TestMessageReserveTokens_StableWorstCase: with (sysPrompt="S", maxDup=3, target=50, ctx="") the
        result R satisfies R == est("S") + est(BuildUserPayload("", "", threeSyntheticSubjects)) + margin.
        Assert R does NOT depend on any "current attempt" (there is no such param — it's inherent). Assert
        R(maxDup=3) > R(maxDup=0) (more retries ⇒ larger worst-case). Assert R(maxDup=0) == est("S") +
        est(BuildUserPayload("", "", nil)) + margin (normal instruction overhead only).
      * TestMessageReserveTokens_GrowsWithContext: R(ctx="") < R(ctx="some context") (context block adds).
      * TestMessageReserveTokens_NegativeClamp: maxDup=-1 behaves like maxDup=0 (no panic, no negative slice).
      * TestPlannerReserveTokens: R(forced=0) < R(forced=3) (the forced-count line adds). Assert R includes
        PlannerRetryInstruction in the overhead (R == est(sys) + est(PlannerRetryInstruction + "\n\n" +
        BuildPlannerUserPayload("", ctx, forced)) + margin for a constructed case).
      * TestArbiterReserveTokens: R(commits=nil) < R(commits=[2 commits]) (the commits block adds). Assert
        R == est(sys) + est(BuildArbiterUserPayload(commits, "")) + margin.
      * TestMeasureReserve_MarginIncluded: measureReserve("A", "B", est) == est("A") + est("B") + 256.
  - COVERAGE: every helper + the worst-case stability (the contract's test) + the empty-diff trick parity +
      the margin. Fast (pure arithmetic).

Task 3: EDIT site 1 — internal/generate/generate.go (MESSAGE role, cfg)
  - LOCATE CommitStaged. Current order: RevParseHEAD(:155) → StagedDiff(:163) → empty-check(:172) →
      WriteTree → signal.SetSnapshot → buildSystemPrompt(:189) → recentSubjects → loop.
  - REORDER: move `sysPrompt, err := buildSystemPrompt(ctx, deps.Git, cfg, isUnborn)` (+ its error return)
      to BEFORE the StagedDiff literal (after RevParseHEAD). The empty-check stays after StagedDiff.
  - MEASURE + SET: add `reserve := prompt.MessageReserveTokens(sysPrompt, cfg.MaxDuplicateRetries,
      cfg.SubjectTargetChars, cfg.Context, git.EstimateTokens)` and append `PromptReserveTokens: reserve,`
      to the StagedDiffOptions literal (keep TokenLimit/DiffContext/the existing 4 fields byte-identical).
  - UPDATE comments: the "Step 4: system prompt" numbering shifts (the prompt build is now before StagedDiff).
      Re-number/annotate but preserve the logic. buildSystemPrompt needs isUnborn (available from RevParseHEAD). ✓
  - RUN: gofmt -w ; go build ./internal/generate/ → exit 0.

Task 4: EDIT site 2 — internal/hook/exec.go (MESSAGE role, cfg)
  - LOCATE Run. Current: StagedDiff(:104) → empty-check(→ErrNoOp) → … → hookSystemPrompt(:131).
  - REORDER hookSystemPrompt (which does CommitCount + RecentMessages + BuildSystemPrompt) to BEFORE StagedDiff.
      hookSystemPrompt needs isUnborn — verify the caller has it before StagedDiff (if not, derive via
      RevParseHEAD first; mirror site 1's pattern).
  - MEASURE + SET: `reserve := prompt.MessageReserveTokens(sysPrompt, cfg.MaxDuplicateRetries,
      cfg.SubjectTargetChars, cfg.Context, git.EstimateTokens)`; append `PromptReserveTokens: reserve,`.
  - RUN: gofmt -w ; go build ./internal/hook/ → exit 0.

Task 5: EDIT site 3 — pkg/stagecoach/stagecoach.go (MESSAGE role, cfg + systemExtra)
  - LOCATE runPipeline. Current: StagedDiff(:423) → empty-check → WriteTree → buildSysPrompt(:447) →
      `sysPrompt += "\n\n" + systemExtra`(:452) → loop.
  - REORDER: move buildSysPrompt + the systemExtra append to BEFORE StagedDiff. MEASURE the reserve AFTER
      the systemExtra append (so it includes systemExtra, design §9): `reserve := prompt.MessageReserveTokens(
      sysPrompt, cfg.MaxDuplicateRetries, cfg.SubjectTargetChars, cfg.Context, git.EstimateTokens)`.
  - SET: append `PromptReserveTokens: reserve,` to the StagedDiffOptions literal.
  - GOTCHA: runPipeline also has a DryRun path — the reserve is measured+set uniformly (DryRun still calls
      StagedDiff to capture the diff). pkg/stagecoach already imports internal/git ⇒ git.EstimateTokens resolves.
  - RUN: gofmt -w ; go build ./pkg/stagecoach/ → exit 0.

Task 6: EDIT site 4 — internal/decompose/planner.go (PLANNER role, deps.Config)
  - LOCATE callPlanner. Current: TreeDiff(:69) → plannerExamples(:82) + BuildPlannerSystemPrompt(:86).
  - REORDER plannerExamples + `sysPrompt := prompt.BuildPlannerSystemPrompt(examples, deps.Config.Format,
      deps.Config.Locale)` to BEFORE the TreeDiff literal. plannerExamples needs isUnborn (callPlanner's
      signature has it).
  - MEASURE + SET: `reserve := prompt.PlannerReserveTokens(sysPrompt, forcedCount, deps.Config.Context,
      git.EstimateTokens)`; append `PromptReserveTokens: reserve,` to the TreeDiff StagedDiffOptions literal.
  - RUN: gofmt -w ; go build ./internal/decompose/ → exit 0.

Task 7: EDIT site 5 — internal/decompose/message.go (MESSAGE role, deps.Config)
  - LOCATE generateMessage. Current: TreeDiff(:71) → empty-check → RevParseHEAD(:89) → messageSystemPrompt(:95).
  - REORDER: move RevParseHEAD + `sysPrompt, err := messageSystemPrompt(ctx, deps.Git, deps.Config, isUnborn)`
      to BEFORE the TreeDiff literal (messageSystemPrompt needs isUnborn). The empty-check stays after TreeDiff.
  - MEASURE + SET: `reserve := prompt.MessageReserveTokens(sysPrompt, deps.Config.MaxDuplicateRetries,
      deps.Config.SubjectTargetChars, deps.Config.Context, git.EstimateTokens)`; append `PromptReserveTokens:
      reserve,`.
  - GOTCHA: site 5 is the MESSAGE role (its loop appends `rejected` — message.go:121) ⇒ use MessageReserveTokens
      (NOT the planner formula). design §1.
  - RUN: gofmt -w ; go build ./internal/decompose/ → exit 0.

Task 8: EDIT site 6 — internal/decompose/decompose.go (ARBITER role, deps.Config)
  - LOCATE runArbiterPhase(:603). Current: derive tipTree → TreeDiff(:608) → runArbiter(:620).
  - BEFORE the TreeDiff: `arbiterCommits, _ := convertArbiterCommits(commits)` (in-package, arbiter.go;
      design §8) + `arbiterSys := prompt.BuildArbiterSystemPrompt()` (zero-arg). MEASURE: `reserve :=
      prompt.ArbiterReserveTokens(arbiterSys, arbiterCommits, git.EstimateTokens)`. SET: append
      `PromptReserveTokens: reserve,` to the TreeDiff StagedDiffOptions literal.
  - GOTCHA: convertArbiterCommits returns (arbiterCommits, validSHAs) — discard validSHAs for the reserve.
      runArbiter keeps its own convertArbiterCommits call (UNCHANGED). git.EstimateTokens: internal/decompose
      already imports internal/git (uses git.StagedDiffOptions) ⇒ resolves with no new import.
  - RUN: gofmt -w ; go build ./internal/decompose/ → exit 0.

Task 9: VALIDATE (run all gates; fix before declaring done)
  - gofmt -w internal/prompt/reserve.go internal/prompt/reserve_test.go internal/generate/generate.go
      internal/hook/exec.go pkg/stagecoach/stagecoach.go internal/decompose/planner.go
      internal/decompose/message.go internal/decompose/decompose.go
  - go vet ./... && go build ./...
  - go test ./...   (ALL green — behavior-free: the field is unread until M4.T3; diff output byte-identical.)
  - go test ./internal/prompt/ -run 'Reserve' -v   (the new helper tests)
  - git grep -n 'PromptReserveTokens:' internal/generate internal/hook pkg/stagecoach internal/decompose
      (expect: 6 matches — one per site, each set from the role-appropriate helper.)
  - git grep -n 'EstimateTokens' internal/generate internal/hook pkg/stagecoach internal/decompose
      (expect: 6 matches — each site passes git.EstimateTokens to the helper.)
  - ! grep -q 'stagecoach/internal/git' internal/prompt/reserve.go   (confirm leaf-pure: NO internal import.)
  - git diff --stat → expect ONLY the 2 new files + the 6 edited call-site files. internal/git UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the empty-diff trick — the payload builders take diff as the verbatim tail, so diff="" isolates
// the non-diff overhead. This is the SINGLE SOURCE OF TRUTH for the prompt topology (no hand-rebuilt
// constants). If §17.3/§17.5/§17.7 change, the reserve tracks automatically.
overhead := BuildUserPayload("", context, worstRejected)       // message: worst-case rejection path
overhead := PlannerRetryInstruction + "\n\n" + BuildPlannerUserPayload("", context, forcedCount)  // planner
overhead := BuildArbiterUserPayload(commits, "")               // arbiter: commits + headers

// PATTERN: worst-case measured ONCE (StagedDiff is called once, before the dedupe loop). The message
// reserve synthesizes maxDuplicateRetries rejected subjects — a ceiling, stable across attempts. This is
// the contract's test assertion ("reserve is stable across retry attempts").
worstRejected := make([]string, max(maxDuplicateRetries,0))
for i := range worstRejected { worstRejected[i] = strings.Repeat("x", max(subjectTargetChars,1)) }

// PATTERN: leaf-pure prompt via INJECTED estimator. Do NOT import internal/git into prompt.
type TokenEstimator func(s string) int
// call sites: prompt.MessageReserveTokens(sysPrompt, …, git.EstimateTokens)

// PATTERN: uniform call sites — ALWAYS measure + set (even when token_limit==0). No branching.
// reserve := prompt.<Role>ReserveTokens(sysPrompt, …, git.EstimateTokens)
// diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{ …, PromptReserveTokens: reserve })

// GOTCHA: REORDER — build the system prompt BEFORE the diff call (so the reserve can be measured). The
// empty-diff path then runs RecentMessages before the empty-check returns (rare/cheap/accepted).

// GOTCHA: reserveSafetyMargin (256, HERE) ≠ FR3i's body_budget margin (M4.T2). The reserve (incl. its 256)
// IS the `prompt` term in body_budget = token_limit − skeleton − prompt − margin.

// GOTCHA: behavior-free — the diff functions don't read PromptReserveTokens until M4.T3. Setting it +
// reordering the prompt build changes NO diff output. Existing tests stay green (the regression proof).
```

### Integration Points

```yaml
PROMPT.PACKAGE (internal/prompt/reserve.go):
  - +type TokenEstimator func(s string) int   (injected estimator; call sites pass git.EstimateTokens)
  - +const reserveSafetyMargin = 256           (the reserve's safety buffer; DISTINCT from FR3i's margin)
  - +func measureReserve(sysPrompt, overhead, est) int   (shared core)
  - +func MessageReserveTokens / PlannerReserveTokens / ArbiterReserveTokens
  - leaf-pure: imports ONLY stdlib `strings` (NO internal imports)

CALL SITES (6 — each: reorder prompt build before diff + measure + set PromptReserveTokens):
  - internal/generate/generate.go:163   StagedDiff  (cfg)          → MessageReserveTokens
  - internal/hook/exec.go:104           StagedDiff  (cfg)          → MessageReserveTokens
  - pkg/stagecoach/stagecoach.go:423      StagedDiff  (cfg)          → MessageReserveTokens (after systemExtra)
  - internal/decompose/planner.go:69    TreeDiff    (deps.Config)  → PlannerReserveTokens
  - internal/decompose/message.go:71    TreeDiff    (deps.Config)  → MessageReserveTokens
  - internal/decompose/decompose.go:608 TreeDiff    (deps.Config)  → ArbiterReserveTokens (convertArbiterCommits)

NOT TOUCHED (explicitly — owned by sibling/completed subtasks):
  - internal/git/* (S1's EstimateTokens is the frozen INPUT; M4.T2/T3 own the diff-function consumption)   # READ ONLY
  - internal/git/git.go StagedDiffOptions + the 3 diff functions   # S1/M4.T2/T3 territory
  - internal/config/* (TokenLimit/DiffContext/etc. — P1.M1.T1/T2 COMPLETE)   # READ ONLY
  - any docs (contract: "DOCS: none — internal seam")
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM CONSUMERS (informational — owned by LATER subtasks, NOT this one):
  - P1.M4.T3 (FR3d gate): reads opts.TokenLimit + opts.PromptReserveTokens → switches off legacy caps when >0
  - P1.M4.T2 (FR3i water-fill): body_budget = token_limit − skeleton − opts.PromptReserveTokens − margin
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/prompt/reserve.go internal/prompt/reserve_test.go internal/generate/generate.go \
       internal/hook/exec.go pkg/stagecoach/stagecoach.go internal/decompose/planner.go \
       internal/decompose/message.go internal/decompose/decompose.go
# Expected: empty (run gofmt -w on any listed file — it re-aligns the struct literals).

go vet ./...
# Expected: exit 0. (A "declared and not used" for `reserve` means a site measured it but forgot to set the field.)

go build ./...
# Expected: exit 0. Confirms reserve.go compiles (leaf-pure — strings only) + all 6 edits type-check
# (TokenEstimator matches git.EstimateTokens's `func(string) int`).

# Confirm reserve.go is leaf-pure (NO internal import — the estimator is injected):
! grep -q 'stagecoach/internal/git' internal/prompt/reserve.go && echo "OK: reserve.go is leaf-pure"
```

### Level 2: Unit Tests (the helper family + behavior-free regression)

```bash
cd /home/dustin/projects/stagecoach

# The new helper family (pure table tests):
go test ./internal/prompt/ -run 'Reserve' -v
# Expected PASS — worst-case stability, empty examples, monotonic in maxDuplicateRetries, margin included,
# empty-diff trick parity. If MessageReserveTokens(maxDup=3) <= MessageReserveTokens(maxDup=0), the worst-case
# isn't growing with retries (fix the synthesis). If measureReserve doesn't include +256, the margin was dropped.

# The 6 call-site packages — existing suites unchanged (PromptReserveTokens is unread until M4.T3):
go test ./internal/generate/ ./internal/hook/ ./internal/decompose/ ./pkg/stagecoach/ ./internal/prompt/
# Expected: ALL green. No existing test alters (the diff functions do not read the field yet).

go test ./...
# Expected: ALL packages green (behavior-free — §7).
```

### Level 3: Whole-Repository Regression (no collateral)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...     # Expected: ALL green.
go vet ./...            # Expected: exit 0.

# Confirm all 6 sites set PromptReserveTokens (6 matches):
git grep -n 'PromptReserveTokens:' internal/generate/generate.go internal/hook/exec.go \
    pkg/stagecoach/stagecoach.go internal/decompose/planner.go internal/decompose/message.go \
    internal/decompose/decompose.go | wc -l
# Expected: 6.

# Confirm each site passes git.EstimateTokens (6 matches — the single estimator is wired everywhere):
git grep -n 'git.EstimateTokens' internal/generate/generate.go internal/hook/exec.go \
    pkg/stagecoach/stagecoach.go internal/decompose/planner.go internal/decompose/message.go \
    internal/decompose/decompose.go | wc -l
# Expected: 6.

# Confirm the role mapping: Message at 4 sites, Planner at 1, Arbiter at 1:
git grep -c 'MessageReserveTokens' internal/generate/generate.go internal/hook/exec.go \
    pkg/stagecoach/stagecoach.go internal/decompose/message.go   # expect 1 each (4 total)
git grep -c 'PlannerReserveTokens' internal/decompose/planner.go                       # expect 1
git grep -c 'ArbiterReserveTokens' internal/decompose/decompose.go                     # expect 1

# Confirm ONLY the 2 new + 6 edited files changed; internal/git UNCHANGED:
git diff --stat -- internal/prompt/ internal/generate/ internal/hook/ internal/decompose/ pkg/stagecoach/
# Expected: reserve.go + reserve_test.go (new) + the 6 call-site files (modified). Nothing else.
git diff --stat -- internal/git/   # Expected: EMPTY (S1's EstimateTokens is the frozen input; untouched).
```

### Level 4: Worst-Case Correctness Cross-Check (the reserve IS an upper bound)

```bash
cd /home/dustin/projects/stagecoach

# The correctness contract: the reserve must be a WORST-CASE upper bound on the non-diff prompt portion at
# EVERY attempt. The helper tests (Level 2) pin this arithmetically. This is the human-readable reasoning:
go test ./internal/prompt/ -run 'TestMessageReserveTokens_StableWorstCase|TestMeasureReserve_MarginIncluded' -v
# Expected: PASS. Verify by reasoning:
#   1. WORST-CASE: MessageReserveTokens synthesizes maxDuplicateRetries subjects (every rejection slot filled).
#      A real run fills ≤ that many ⇒ the reserve ≥ every per-attempt payload's non-diff overhead. ✓
#   2. STABLE: the helper takes maxDuplicateRetries (not a per-attempt count) ⇒ the reserve is constant across
#      the dedupe loop. ✓ (the contract's test assertion)
#   3. UPPER BOUND: reserve = est(sysPrompt) + est(worstOverhead) + 256. Even if real subjects are 2× the
#      target, 3×100=300 chars=75 tokens ≪ 256-token margin. ✓
#   4. SINGLE ESTIMATOR: every site passes git.EstimateTokens (Level 3 grep = 6) ⇒ upstream/downstream agree. ✓

# Confirm no second estimator / no chars/3 variant crept in:
! grep -rn '/ 3\|chars.*3' internal/prompt/reserve.go && echo "OK: no second formula (S1's single-estimator rule honored)"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0; `go vet ./...` exits 0; `gofmt -l .` reports nothing.
- [ ] `go test ./...` (and `go test -race ./...`) — all packages green.
- [ ] `reserve.go` imports ONLY `strings` (leaf-pure); `TokenEstimator` is the injected estimator seam.

### Feature Validation
- [ ] `TokenEstimator` + `MessageReserveTokens` + `PlannerReserveTokens` + `ArbiterReserveTokens` +
      `measureReserve` + `reserveSafetyMargin (256)` exist in `package prompt`.
- [ ] All 6 production literals carry `PromptReserveTokens` set from the role-appropriate helper (Message at
      1,2,3,5; Planner at 4; Arbiter at 6), each passing `git.EstimateTokens`.
- [ ] Each site's system-prompt build is REORDERED to precede its diff call (the reserve is measured from
      the built system prompt).
- [ ] The message reserve is a stable worst-case (synthesizes maxDuplicateRetries subjects) — pinned by
      `TestMessageReserveTokens_StableWorstCase`.
- [ ] Site 3 measures AFTER the systemExtra append; site 6 uses `convertArbiterCommits` for exact arbiter overhead.
- [ ] `reserveSafetyMargin` (256) is documented as DISTINCT from FR3i's body_budget margin (M4.T2).

### Scope Discipline Validation
- [ ] ONLY the 2 new files + the 6 edited call-site files changed (`git diff --stat`).
- [ ] Did NOT edit `internal/git/*` (S1's EstimateTokens is the frozen input; M4.T2/T3 own diff consumption).
- [ ] Did NOT change `StagedDiffOptions`, the diff functions, or the `Git` interface.
- [ ] Did NOT import `internal/git` into `internal/prompt` (leaf-pure via injected `TokenEstimator`).
- [ ] Did NOT add config/CLI/docs surface (contract: "DOCS: none — internal seam").
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research).

### Code Quality Validation
- [ ] The helper family uses the empty-diff trick (single source of truth — no hand-rebuilt constants).
- [ ] `TokenEstimator` is injected (preserves prompt's leaf-purity); all 6 sites pass the SAME estimator.
- [ ] The reserve is a documented worst-case ceiling (stable across attempts; absorbs over-length via the margin).
- [ ] The reorder is behavior-free (existing tests unchanged — the field is unread until M4.T3).
- [ ] Doc comments cite FR3d/FR3i, the worst-case rationale, the single-estimator rule, and the margin distinction.

---

## Anti-Patterns to Avoid

- ❌ Don't import `internal/git` into `internal/prompt`. prompt is leaf-pure by stated design (planner.go).
  Inject the estimator: `est TokenEstimator`, call sites pass `git.EstimateTokens`. (§0)
- ❌ Don't apply the planner formula to site 5 (message.go). Site 5 is the MESSAGE role (its loop appends
  `rejected` — message.go:121). Use `MessageReserveTokens` (with the rejection block) or the reserve
  under-counts and the payload can exceed token_limit on a duplicate-retry. (§1)
- ❌ Don't hand-rebuild the rejection-block constants. The payload builders append diff as the verbatim tail
  ⇒ call them with `""` to isolate the exact overhead (single source of truth). (§2)
- ❌ Don't measure the reserve per-attempt. StagedDiff is called ONCE before the dedupe loop; the reserve is
  the WORST CASE (maxDuplicateRetries slots), measured once — stable across attempts. (§3)
- ❌ Don't skip the reorder. The system prompt must exist BEFORE the diff call to be measured. Build it first,
  measure, then call the diff with `PromptReserveTokens` set. (§4)
- ❌ Don't branch on `token_limit > 0` at the call sites. ALWAYS measure + set (uniform sites; M4.T3's gate
  decides whether to consume it). (§5)
- ❌ Don't conflate `reserveSafetyMargin` (256, this task) with FR3i's body_budget `margin` (M4.T2). The
  reserve (incl. its 256) IS the `prompt` term. (§6)
- ❌ Don't edit `internal/git/*`, `StagedDiffOptions`, or the diff functions. S1 owns `EstimateTokens`;
  M4.T2/T3 own the diff-function consumption of `PromptReserveTokens`. This task MEASURES + SETS only. (scope)
- ❌ Don't introduce a second estimator or a chars/3 variant. S1's `git.EstimateTokens` (chars/4) is the
  SINGLE measure — pass it at every site so upstream/downstream budget arithmetic is coherent. (S1's rule)
- ❌ Don't move `recentSubjects` (the dedupe fetch). Only the system-prompt build (style examples) moves before
  the diff; `recentSubjects` is separate and stays where it is (not needed for the reserve). (§4 gotcha)
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: the helper family is small and fully specified (copy-ready `reserve.go` skeleton + the empty-diff
trick that eliminates constant duplication). The 6 call sites are quoted verbatim with their current order,
role, variable shape, and exact edit (reorder + measure + set one field). S1's `git.EstimateTokens(s string)
int` signature is the frozen INPUT (parallel, but its contract is fixed); P1.M1.T2.S2 is LANDED (the 6
literals already exist with TokenLimit/DiffContext, PromptReserveTokens at zero). The task is behavior-free
by construction (the field is unread until M4.T3), so `go test ./...` staying green IS the regression proof.
The three residual risks — (a) the contract's "planner path" lumping of sites 5/6 vs. the per-role reality
(resolved explicitly in §1: site 5 = message, site 6 = arbiter), (b) the reorder touching each site's
step-numbering/comments (cosmetic; caught by gofmt + build), and (c) site 6's `convertArbiterCommits` reuse
(in-package, §8) — are all addressed in the design decisions and pinned by the Level 3/4 gates (6
PromptReserveTokens matches, 6 git.EstimateTokens matches, role-mapping grep, leaf-pure grep). The downstream
consumers (M4.T2 water-fill, M4.T3 gate) are cleanly fenced and cannot be broken by setting a field they will
later read.
