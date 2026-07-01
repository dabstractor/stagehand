---
name: "P3.M2.T2.S1 — Implement internal/decompose/planner.go: planner agent call + JSON parse/retry + single-shortcut (PRD §13.6.2/§13.6.4/§13.6.6, §9.14 FR-M3/M4/M11)"
description: |

  CREATE ONE NEW FILE `internal/decompose/planner.go` (package `decompose`, the 2nd file after the
  already-shipped roles.go) and ONE NEW TEST FILE `planner_test.go`. planner.go is the planner half of
  the multi-commit decomposition pipeline (PRD §13.6.2): `callPlanner` captures the full working-tree
  diff, builds the planner prompt (§17.5), Renders the planner manifest in BARE mode, Executes the
  agent, parses the JSON `PlannerOutput` with one retry on parse/contract failure, enforces the
  single⇔message contract (FR-M11 single-shortcut) and the auto-mode safety cap (FR-M4), and returns
  the parsed partition (or single-shortcut message). It is the decompose analogue of v1's
  generate.CommitStaged generation loop (one bare agent call + bounded retry), specialized to the
  planner's JSON output contract. Consumed by the orchestrator (P3.M4.T1.S1); NO caller wiring here.

  CONTRACT (P3.M2.T2.S1, verbatim from the work item):
    1. RESEARCH NOTE: The planner (§13.6.2, FR-M3) is BARE, receives the full working-tree diff
       (WorkingTreeDiff from P2.M2.T2.S2) + style examples (RecentMessages). Output is JSON
       (PlannerOutput from prompt/planner.go P3.M1.T1.S1). Modes: auto-decompose (planner decides
       count+partition), forced-count (--commits N, planner only partitions). Single-call shortcut
       (FR-M11): if planner returns single==true, use planner's message directly. Retry once on
       unparseable JSON. Planner failure (§13.6.6): no commits made yet, surface error, exit non-rescue.
    2. INPUT: Deps with Planner manifest from P3.M2.T1.S1, prompt/planner.go from P3.M1.T1.S1,
       WorkingTreeDiff from P2.M2.T2.S2.
    3. LOGIC: Create internal/decompose/planner.go. Implement
       `func callPlanner(ctx, deps Deps, forcedCount int, isUnborn bool) (prompt.PlannerOutput, error)`:
       capture working-tree diff via deps.Git.WorkingTreeDiff; build system prompt (style examples);
       build user payload (forced-count prefix if forcedCount>0); Render with bare mode; Execute;
       ParsePlannerOutput with one retry on parse failure. If single==true, the output carries a Message
       field for the single-shortcut. Enforce safety cap: if output.Count > deps.Config.MaxCommits and
       forcedCount==0, return error 'planner proposed N commits; exceeds max_commits (M); use --commits
       or --max-commits'.
    4. OUTPUT: callPlanner returns a parsed PlannerOutput (concepts list or single-shortcut). Consumed
       by the orchestrator (P3.M4.T1.S1).
    5. DOCS: none — internal agent call.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/decompose/roles.go — SHIPPED by P3.M2.T1.S1 (RUNNING IN PARALLEL). Defines Deps
      {Git, Registry, Config, Roles RoleManifests, Verbose}, RoleManifests, RoleModels, ResolveRoles,
      computeInstalled, isMultiProvider, setRole. CONSUMED VERBATIM. Deps has NO Models field (callPlanner
      derives the planner model from deps.Config — see findings §3 / Gotchas).
    - internal/prompt/planner.go — SHIPPED by P3.M1.T1.S1. CONSUMED: BuildPlannerSystemPrompt,
      BuildPlannerUserPayload, ParsePlannerOutput, PlannerRetryInstruction, PlannerOutput, PlannerCommit.
    - internal/provider/{render,executor}.go — CONSUMED: Manifest.Render(model,provider,sys,payload,mode...),
      provider.Execute(ctx, spec, timeout, vb) → (stdout, stderr, err), RenderBare, CmdSpec.
    - internal/git/git.go — CONSUMED: WorkingTreeDiff(ctx, StagedDiffOptions), RecentMessages(ctx, n),
      StagedDiffOptions{MaxDiffBytes, MaxMdLines, BinaryExtensions}.
    - internal/config/{config,roles}.go — CONSUMED: Config (MaxCommits=12, MaxDiffBytes, MaxMdLines,
      BinaryExtensions, Timeout), ResolveRoleModel(role, cfg) → (provider, model), config.Defaults().
    - internal/decompose/{stager,message,arbiter,chain,decompose}.go — DO NOT EXIST YET. This task
      creates ONLY planner.go (+ planner_test.go). Other tasks own the rest.
    - cmd/, pkg/stagehand/ — UNCHANGED (the orchestrator P3.M4.T1.S1 wires callPlanner; NOT this task).

  DELIVERABLES (2 new files, 0 edits to existing files, 0 breaking changes):
    CREATE internal/decompose/planner.go — package `decompose`; callPlanner (the exported-by-package-
      convention entry point), ErrPlannerFailed sentinel, validatePlannerOutput (private), and a small
      private helper for the style examples (RecentMessages short-circuit on unborn).
    CREATE internal/decompose/planner_test.go — stubtest + real-git integration tests (happy multi-commit;
      single-shortcut; forced-count; parse-retry-then-success; safety-cap error; single-without-message;
      unparseable-after-retry; timeout; unborn nil-examples).

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass; callPlanner
  returns a parsed PlannerOutput on valid JSON; retries once on parse/contract failure then errors;
  enforces single⇔message (FR-M11) and the auto-mode safety cap (FR-M4) with the exact message; all
  callPlanner errors are non-rescue (no snapshot during planning, §13.6.6).

---

## Goal

**Feature Goal**: Implement the planner agent invocation for multi-commit decomposition (PRD §13.6.2 /
FR-M3) as a self-contained module `internal/decompose/planner.go`. `callPlanner(ctx, deps, forcedCount,
isUnborn)` is the decompose analogue of v1's generate.CommitStaged generation loop (one bare agent call
+ a bounded retry), specialized to the planner's structured-JSON output contract: it captures the full
working-tree diff, assembles the §17.5 planner prompt (system + forced/normal user payload), Renders the
resolved planner manifest in BARE mode, Executes the agent, parses `prompt.PlannerOutput` with one retry
on parse/contract failure, enforces the single⇔message contract (FR-M11 single-shortcut: single==true ⇒
Message present) and the auto-decompose safety cap (FR-M4: Count ≤ MaxCommits), and returns the parsed
partition. It generalizes the proven v1 generate loop to the planner role's JSON output + the
single-shortcut + the safety cap — no new architectural concept (the snapshot/CAS machinery is NOT
touched; planning precedes all staging, so callPlanner performs ZERO git mutations).

**Deliverable** (2 new files in the existing `decompose` package):
1. `internal/decompose/planner.go` — `ErrPlannerFailed` sentinel; `callPlanner(ctx, deps, forcedCount,
   isUnborn) (prompt.PlannerOutput, error)`; `validatePlannerOutput` + the style-examples helper (private).
2. `internal/decompose/planner_test.go` — stubtest-driven integration tests against a real temp git repo.

**Success Definition**:
- Happy multi-commit: a stub emitting valid `{"count":3,"single":false,"commits":[...]}` ⇒ callPlanner
  returns that PlannerOutput verbatim; nil error.
- Single-shortcut (FR-M11): stub emitting `{"count":1,"single":true,"commits":[...],"message":"feat: …"}`
  ⇒ returns Single=true, Message="feat: …"; nil error.
- Forced-count: `forcedCount=3` ⇒ the rendered payload carries the "Produce EXACTLY 3 commits…"
  directive (BuildPlannerUserPayload prepends it); callPlanner still parses the response normally.
- Parse retry: stub script `[invalidJSON, validJSON]` ⇒ first call parse-fails, second (with
  PlannerRetryInstruction prepended) parses ⇒ returns the valid output; nil error (one retry consumed).
- Contract failure retry: stub emitting `{"count":1,"single":true,"commits":[...]}` (single, NO message)
  then a valid single-with-message ⇒ retried once, succeeds; OR `[malformed, malformed]` ⇒ ErrPlannerFailed.
- Safety cap (FR-M4, auto mode): stub emitting `{"count":15,...}` with `cfg.MaxCommits=12` and
  `forcedCount=0` ⇒ returns the EXACT error `planner proposed 15 commits; exceeds max_commits (12); use
  --commits or --max-commits` (no retry, no ErrPlannerFailed wrap). Same count with `forcedCount>0` ⇒ NO
  cap error (forced mode trusts the user's --commits).
- Timeout: stub `SleepMS > cfg.Timeout` ⇒ Execute returns DeadlineExceeded ⇒ callPlanner returns an
  ErrPlannerFailed-wrapped error, NON-RESCUE (no snapshot taken).
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 2 new files, nothing else.

## User Persona

**Target User**: the decompose orchestrator (`internal/decompose/decompose.go`, P3.M4.T1.S1) and, by
extension, the end user running `stagehand` on an un-staged working tree (the default action routes to
decompose per FR-M1, P4.M1.T1.S1). planner.go is internal plumbing — NOT user-facing CLI text. The user
controls commit granularity via `--commits N` (forced) vs default (auto-decompose) vs `--single`
(bypass); callPlanner is the layer that turns the working-tree diff into a structured partition the
loop consumes.

**Use Case**: once the orchestrator has built `Deps` (via ResolveRoles, P3.M2.T1.S1) and confirmed the
decompose trigger (nothing staged + working tree dirty, FR-M1), it calls `callPlanner(ctx, deps,
forcedCount, isUnborn)` ONCE. callPlanner runs the bare planner agent over the full working-tree diff +
style examples, parses the JSON partition, and hands back the concepts[] (or the single-shortcut
message). The orchestrator then drives stager→snapshot→message→commit per concept. A planner failure
(unparseable even after retry, timeout, cap exceeded) surfaces as a non-rescue error BEFORE any staging —
no commits made, nothing to recover.

**Pain Points Addressed**: (a) the planner is a SINGLE bare agent round-trip that decides the whole
partition (vs N round-trips) — callPlanner keeps it one call + one corrective retry; (b) a runaway
planner proposing dozens of micro-commits is capped by FR-M4 with an actionable remediation; (c) the
trivial case (one commit suffices) is the single-call shortcut (FR-M11) — the planner emits the message
in the same call, no separate message-agent round-trip; (d) an unparseable/contract-violating planner
output is retried once (cheap correction) then surfaced as a clean non-rescue error (no half-made history).

## Why

- **Closes the planner half of PRD §13.6.2 / §13.6.4 / §13.6.6 / §9.14 FR-M3/M4/M11.** The planner is the
  first of the four decompose roles; it decides the count (unless forced) and the partition, and — when
  one commit suffices — emits the message for free. This task is the literal invocation+parse+retry+
  shortcut+cap implementation. With it, the orchestrator has its planner entry point; P3.M2.T3/T4
  (stager/message) and P3.M3.* (arbiter) can assume `callPlanner` exists.
- **Generalizes the proven v1 generation loop (generate.CommitStaged → callPlanner).** v1 already does
  "Render bare → Execute → parse → retry once on parse failure → accept/dedupe" for ONE role (message,
  raw-text output). callPlanner is the SAME algorithm with THREE planner-specific deltas: (1) JSON output
  via `prompt.ParsePlannerOutput` (not raw-text `provider.ParseOutput`); (2) the single⇔message contract
  (FR-M11); (3) the auto-mode safety cap (FR-M4). No new concept — the snapshot/CAS machinery is
  untouched (planning precedes all staging).
- **Unblocks the decompose pipeline (P3.M2–P3.M4).** Every downstream step (stager per concept, message
  per concept, arbiter) consumes the concepts[] callPlanner returns (or the single-shortcut). The
  orchestrator cannot run until callPlanner exists. This is the second foundation file of the
  `internal/decompose/` package (roles.go is the first).
- **Lowest-risk, maximal-reuse, backward-compatible.** Two NEW files in an EXISTING package (decompose);
  ZERO edits to any shipped file (roles.go, prompt/planner.go, provider/*, git/*, config/* all CONSUMED).
  go.mod/go.sum untouched (stdlib + config/git/prompt/provider, all already imported by roles.go). No
  import cycle (decompose already imports all four). callPlanner is consumed later (P3.M4.T1.S1); no
  caller wiring here → zero merge friction.

## What

One new file `internal/decompose/planner.go` in the existing `decompose` package exporting one sentinel
+ one function (+ private helpers), and one new test file. No new dependencies. No caller wiring (that
is P3.M4.T1.S1). Specifically:

- **`ErrPlannerFailed`** (exported package-level sentinel): `errors.New("decompose: planner failed")`.
  Wrap ALL genuine planner failures (parse-after-retry, single⇔message-after-retry, exec-non-zero-after-
  retry, timeout, canceled, render error) with `%w` so `errors.Is(err, ErrPlannerFailed)` is true. The
  orchestrator treats ANY callPlanner error as NON-RESCUE (no snapshot during planning, §13.6.6). The
  safety-cap error is distinct (NOT wrapped — actionable remediation) but also non-rescue.
- **`callPlanner(ctx context.Context, deps Deps, forcedCount int, isUnborn bool) (prompt.PlannerOutput,
  error)`**: the planner agent invocation. Pipeline: (1) derive planner (provider, model) via
  `config.ResolveRoleModel("planner", deps.Config)`; (2) capture `WorkingTreeDiff` (caps from cfg); (3)
  build style examples (RecentMessages(20), short-circuited on isUnborn); (4) build system prompt
  (BuildPlannerSystemPrompt) + user payload (BuildPlannerUserPayload(diff, forcedCount)); (5) the retry
  loop (≤2 attempts): Render(bare) → Execute → ParsePlannerOutput → validatePlannerOutput; on parse OR
  contract failure, retry once with PlannerRetryInstruction prepended; on timeout/cancel, return
  ErrPlannerFailed immediately; (6) on an accepted output, enforce the safety cap (auto mode only) with
  the exact message; (7) return the parsed PlannerOutput.
- **`validatePlannerOutput(out prompt.PlannerOutput) error`** (private): count≥1; single⇔message
  (single⇒message present; the FR-M11 load-bearing check); non-single ⇒ ≥1 commit. Drives the retry on
  violation (same budget as a parse failure).
- **private style-examples helper** (or inline): `RecentMessages(ctx, 20)` short-circuited to nil on
  isUnborn (mirrors generate.buildSystemPrompt / recentSubjects); fed to BuildPlannerSystemPrompt.

### Success Criteria

- [ ] `internal/decompose/planner.go` is package `decompose`, has a file doc comment citing PRD
      §13.6.2/§13.6.4/§13.6.6 + FR-M3/M4/M11, and defines `ErrPlannerFailed` + `callPlanner` EXACTLY as
      the contract (signature `callPlanner(ctx, deps Deps, forcedCount int, isUnborn bool)`).
- [ ] callPlanner uses the planner manifest from `deps.Roles.Planner` (BARE mode) and derives the
      planner (provider, model) via `config.ResolveRoleModel("planner", deps.Config)` (Deps has no Models
      field — findings §3).
- [ ] callPlanner captures the diff via `deps.Git.WorkingTreeDiff(StagedDiffOptions{MaxDiffBytes,
      MaxMdLines, BinaryExtensions})` (from cfg) and builds the §17.5 prompt (BuildPlannerSystemPrompt +
      BuildPlannerUserPayload with forcedCount).
- [ ] callPlanner Executes via `provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)` and
      parses via `prompt.ParsePlannerOutput`; it retries ONCE on parse failure (PlannerRetryInstruction
      prepended) and ONCE on validatePlannerOutput failure (shared budget); timeout/cancel return
      immediately (no retry).
- [ ] single⇔message (FR-M11): a single==true output with an empty Message is treated as a contract
      failure (validatePlannerOutput) → retried once → ErrPlannerFailed if it persists. A single==true
      output WITH a Message is returned as-is (the shortcut).
- [ ] Safety cap (FR-M4): accepted output with `forcedCount==0 && Count > deps.Config.MaxCommits` returns
      the EXACT error `planner proposed N commits; exceeds max_commits (M); use --commits or --max-commits`
      (N=output.Count, M=deps.Config.MaxCommits); NOT wrapped in ErrPlannerFailed; no retry. Same count
      with forcedCount>0 ⇒ NO cap error.
- [ ] ALL genuine planner failures are wrapped with `ErrPlannerFailed` (`errors.Is` true); the
      safety-cap error is NOT wrapped. None of callPlanner's errors involve a snapshot (non-rescue).
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; only 2 git changes (planner.go, planner_test.go).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the contract + scope
boundary (findings §1/§13); the NOW-SHIPPED Deps shape (no Models field) and the model-derivation
decision (findings §2/§3); the prompt/planner.go API surface (findings §4); the exact execution+retry
pattern to mirror from generate.CommitStaged (findings §5); the single⇔message validation (findings §6);
the safety-cap exact message + non-wrap rule (findings §7); the ErrPlannerFailed sentinel + non-rescue
semantics (findings §8); the isUnborn/diff handling (findings §9); the config fields (findings §10); the
stubtest + real-git test pattern (findings §11); the no-deps/no-cycle/zero-friction facts (findings §12).
No prior decompose knowledge beyond roles.go's Deps is required — callPlanner is fully self-contained.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (contract + 13 sections of load-bearing facts)
- docfile: plan/002_a17bb6c8dc1d/P3M2T2S1/research/findings.md
  why: §1 the verbatim contract + scope boundary; §2 the SHIPPED Deps shape (NO Models field — callPlanner
       must derive the planner model); §3 the KEY DESIGN DECISION (derive via ResolveRoleModel("planner",
       deps.Config) — WHY options B/C are forbidden, WHY it's correct for all reachable cases); §4 the
       prompt/planner.go API (BuildPlannerSystemPrompt/UserPayload, ParsePlannerOutput,
       PlannerRetryInstruction, PlannerOutput/PlannerCommit fields — CONSUMED VERBATIM); §5 the execution
       + retry pattern to mirror from generate.CommitStaged (Render bare, Execute's 3-tuple, execErr
       handling, the ≤2-attempt retry); §6 the single⇔message + light validation (the caller-owned
       contract); §7 the safety-cap EXACT message + non-wrap + auto-mode-only rule; §8 ErrPlannerFailed
       + non-rescue semantics; §9 isUnborn/diff handling; §10 config fields; §11 the test pattern; §12
       no-deps/no-cycle; §13 scope boundary.
  critical: §3 (Deps has NO Models field — do NOT add one (owned by the shipped parallel task); derive
            the model via ResolveRoleModel); §5 (mirror generate's loop: Render→Execute→Parse, retry on
            parse/contract, immediate return on timeout/cancel); §6 (callPlanner OWNS single⇔message);
            §7 (the cap message is EXACT and NOT wrapped in ErrPlannerFailed, auto-mode only).

# MUST READ — the SHIPPED Deps/RoleManifests (P3.M2.T1.S1) — callPlanner's input contract
- file: internal/decompose/roles.go
  section: type Deps struct { Git git.Git; Registry *provider.Registry; Config config.Config; Roles
           RoleManifests; Verbose *ui.Verbose } — the injectable collaborators. RoleManifests{
           Planner, Stager, Message, Arbiter provider.Manifest} — the planner manifest is
           deps.Roles.Planner (BARE). ResolveRoles(cfg, reg) builds Deps; the orchestrator (P3.M4) calls
           callPlanner with it.
  why: confirms Deps has Config + Roles but NO Models/RoleModels field (findings §2/§3). callPlanner
       reads deps.Roles.Planner (the bare manifest) and derives the (provider, model) from deps.Config.
       Do NOT edit this file (shipped by the parallel task; editing = conflict).
  gotcha: RoleManifests.Planner is the merged-but-UNRESOLVED manifest from reg.Get (Render Validate+
          Resolves later — same as buildDeps/generate.Deps.Manifest). Render is safe to call directly.

# MUST READ — the prompt API (P3.M1.T1.S1) — CONSUMED VERBATIM
- file: internal/prompt/planner.go
  section: BuildPlannerSystemPrompt(examples []string) — examples = RecentMessages(20); NO
           DetectMultiline/SubjectTargetChars (§17.5 omits both); nil-safe. BuildPlannerUserPayload(diff,
           forcedCount) — forcedCount<=0 ⇒ normal; >0 ⇒ prepends "Produce EXACTLY N commits…".
           ParsePlannerOutput(raw) (PlannerOutput, error) — whole-string Unmarshal then brace-balanced
           fallback; returns error on parse failure (callPlanner retries); does NOT validate
           single⇔message ("the caller (decompose/planner.go) owns that decision"). PlannerRetryInstruction
           (the retry prepend). PlannerOutput{Count, Single, Commits []PlannerCommit, Message};
           PlannerCommit{Title, Description}.
  why: these are the EXACT symbols callPlanner calls. The single⇔message contract is callPlanner's
       responsibility (ParsePlannerOutput's doc says so) — encode it in validatePlannerOutput.
  pattern: BuildPlannerSystemPrompt(examples) + BuildPlannerUserPayload(diff, forcedCount); on retry,
           payload = PlannerRetryInstruction + "\n\n" + payload (mirror generate's retryInstr prepend).
  gotcha: do NOT pass DetectMultiline/SubjectTargetChars to the planner prompt — §17.5 has neither; the
          planner uses ONLY RecentMessages (the same SOURCE as v1, but its OWN builder).

# MUST READ — the execution loop to mirror (the proven v1 generation loop)
- file: internal/generate/generate.go
  section: CommitStaged step 5 (the GENERATION LOOP) — the pattern callPlanner generalizes.
           `resolved := deps.Manifest.Resolve()`; `retryInstr := *resolved.RetryInstruction`; the loop:
           `payload := prompt.BuildUserPayload(diff, rejected); if parseFail { payload = retryInstr +
           "\n\n" + payload }`; `spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt,
           payload)`; `out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)`;
           execErr handling (DeadlineExceeded ⇒ immediate; Canceled ⇒ immediate; non-zero exit ⇒ fall
           through to parse); `m, ok, _ := provider.ParseOutput(out, deps.Manifest)`; VerboseRetry on
           retry. Also buildSystemPrompt (the isUnborn short-circuit + RecentMessages precedent) +
           recentSubjects (isUnborn short-circuit).
  why: callPlanner is this loop with 3 planner deltas (JSON parse, single⇔message, safety cap) and NO
       dedupe/duplicate-retry (the planner has no duplicate-rejection concept) and NO snapshot/commit
       (planning precedes staging). Mirror the retry-instruction prepend, the execErr handling, the
       VerboseRetry call, and the isUnborn short-circuit on RecentMessages.
  pattern: `out, _, execErr := provider.Execute(...)`; `if errors.Is(execErr, context.DeadlineExceeded)
           { return ...ErrTimeout-wrapped }` (callPlanner returns ErrPlannerFailed instead — non-rescue);
           `if errors.Is(execErr, context.Canceled) { return ... }`; non-zero exit ⇒ lastCause=execErr,
           fall through to parse. The retry counter: generate loops `attempt := 0; attempt <=
           cfg.MaxDuplicateRetries`; callPlanner uses a fixed maxAttempts=2 (1 + 1 retry).
  gotcha: generate wraps timeout/cancel in *RescueError (post-snapshot). callPlanner does NOT — planning
          precedes the snapshot, so there is NO tree to rescue; return a plain ErrPlannerFailed-wrapped
          error (non-rescue). Do NOT import or reuse generate.RescueError.

# MUST READ — Render (bare mode) + Execute (the provider seam)
- file: internal/provider/render.go
  section: `func (m Manifest) Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode)
           (*CmdSpec, error)` — mode defaults to RenderBare (variadic); callPlanner passes
           provider.RenderBare (the planner IS the bare role, §13.6.2). Render calls Validate+Resolve
           internally (safe on the unresolved deps.Roles.Planner manifest). Token order + the
           system-prompt-prepend fallback are Render's concern, not callPlanner's.
- file: internal/provider/executor.go
  section: `func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout,
           stderr string, err error)` — runs the agent; returns (stdout, stderr, err). err is
           context.DeadlineExceeded on timeout, context.Canceled on parent cancel, wrapped *exec.ExitError
           on non-zero exit, wrapped LookPath/start error on start failure. Execute internally calls
           vb.VerboseCommand + vb.VerboseRawOutput.
  why: callPlanner's ONLY provider calls. Pass `*spec` (deref the Render pointer) and deps.Config.Timeout.
       Handle execErr exactly as generate does (findings §5): DeadlineExceeded/Canceled ⇒ immediate
       ErrPlannerFailed; non-zero exit ⇒ fall through to ParsePlannerOutput (stdout may be partial).

# MUST READ — the git methods callPlanner consumes
- file: internal/git/git.go
  section: `WorkingTreeDiff(ctx, StagedDiffOptions) (diff string, err error)` — the unstaged working-tree
           diff (working-tree-vs-INDEX, NOT --cached) with FR3c binary filtering; "" on a clean tree.
           `RecentMessages(ctx, n) ([]string, error)` — up to n most-recent FULL messages for style
           examples; (nil, nil) on an unborn repo (exit 128). `StagedDiffOptions{MaxDiffBytes, MaxMdLines,
           BinaryExtensions}`.
  why: callPlanner calls WorkingTreeDiff for the diff payload and RecentMessages(20) for the style
       examples. Short-circuit RecentMessages on isUnborn (mirror generate) — pass nil examples to
       BuildPlannerSystemPrompt (nil-safe). WorkingTreeDiff values come from deps.Config (mirror
       CommitStaged's StagedDiff call).
  gotcha: WorkingTreeDiff OMITS untracked files (git diff never lists them); the tooled stager discovers
          untracked files itself (FR-M5). callPlanner does NOT gate on empty diff — the orchestrator
          gates (FR-M1). Documented assumption.

# MUST READ — ResolveRoleModel (the planner model derivation) + Config fields
- file: internal/config/roles.go
  section: `func ResolveRoleModel(role string, cfg Config) (provider, model string)` — reads cfg.Roles[role]
           then falls back to cfg.Provider/cfg.Model for empty fields; returns ("","") sentinel. callPlanner
           calls ResolveRoleModel("planner", deps.Config) to derive the planner (provider, model) for Render.
  why: Deps has no Models field (findings §2/§3); this is the consistent, self-contained way to get the
       per-role planner model. It is the SAME function ResolveRoles uses (reads the same cfg) → honors
       per-role overrides (FR-R1/D3). FR-R5b guards the dangerous bare-model-no-provider-on-pi case at
       ResolveRoles time (BEFORE callPlanner runs), so the derivation is correct for every reachable case.
- file: internal/config/config.go
  section: type Config — MaxCommits (default 12, FR-M4), MaxDiffBytes (300000), MaxMdLines (100),
           BinaryExtensions (nil), Timeout (120s). config.Defaults() populates all.
  why: callPlanner reads deps.Config.MaxCommits (the cap), the diff caps, and Timeout. All consumed
       read-only.

# MUST READ — the test infrastructure (stubtest) + the test-repo fixture pattern
- file: internal/stubtest/stubtest.go
  section: `Build(t)` (compiles cmd/stubagent ONCE, cached); `Manifest(bin, Options{Out, Exit, SleepMS,
           Script, Counter, Output, StripCodeFence})` (single-response manifest); `NewScript(t, bin,
           []string{...})` (call-varying: responses[0]=call1 stdout, etc.; blank ⇒ empty). Options.Out is
           the stub's single-response stdout; the stub emits it verbatim.
  why: planner_test.go builds Deps{Git: git.New(repo), Config: config.Defaults(), Roles: RoleManifests{
       Planner: stubManifest}, Verbose: nil} (NO ResolveRoles). The stub emits JSON on stdout;
       ParsePlannerOutput parses it (the stub's Output mode is IRRELEVANT — callPlanner does not call
       provider.ParseOutput). NewScript drives the retry (responses[0]=bad, responses[1]=good).
  gotcha: stubtest.Manifest passes Render's Validate+Resolve (generate_test proves it); the stub manifest
          leaves BareFlags nil (Render appends nil → no-op). The stub does NOT care about the model flag.

# MUST READ — the test-repo fixture helpers (copy into planner_test.go — git's _test.go helpers are unimportable)
- file: internal/generate/generate_test.go
  section: the fixture helpers (initRepo, writeFile, stageFile, commitRaw, headSHA, runGit, gitOut) +
           TestCommitStaged_Success (the canonical stubtest+real-repo integration test pattern).
  why: planner_test.go needs a real git repo with UNSTAGED working-tree changes (write files WITHOUT
       staging) so WorkingTreeDiff is non-empty, plus a few commits (commitRaw) for style examples (or
       unborn for the nil-examples path). Copy the fixture helpers verbatim (they are in package generate
       — unimportable from package decompose; generate_test owns its own copies for the same reason).
  pattern: repo := t.TempDir(); initRepo(t, repo); commitRaw(t, repo, "initial"); writeFile(t, repo,
           "a.txt", "..."); // UNSTAGED → WorkingTreeDiff non-empty. bin := stubtest.Build(t); m :=
           stubtest.Manifest(bin, Options{Out: `<json>`}); deps := Deps{Git: git.New(repo), Config:
           config.Defaults(), Roles: RoleManifests{Planner: m}, Verbose: nil}.

# MUST READ — the design reference (the planner role + single-shortcut + failure handling)
- docfile: plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md
  section: "The Four Agent Roles" (planner = bare, JSON output); "Single-Commit Shortcut (§13.6.4)"
           (single==true ⇒ planner's message, no separate message-agent); "Failure Handling" (Planner
           unparseable/fails ⇒ surface error, nothing snapshotted, exit non-rescue).
  why: confirms the planner is bare, the single-shortcut semantics, and the non-rescue failure contract.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §13.6.2 (planner role: bare, JSON {count,single,commits,message?})
- url: PRD.md §13.6.4 (single-commit shortcut: single==true ⇒ planner's message, FR-M11)
- url: PRD.md §13.6.6 (planner failure: no commits yet, surface error, exit non-rescue)
- url: PRD.md §9.14 FR-M3 (planner agent bare, JSON contract) / FR-M4 (safety cap, max_commits default 12)
       / FR-M11 (single-call shortcut)
- url: PRD.md §17.5 (planner prompt — the §17.5 text committed verbatim in prompt/planner.go)
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  roles.go            # SHIPPED (P3.M2.T1.S1) — READ (CONSUMED): Deps, RoleManifests, RoleModels,
                      #   ResolveRoles. Deps has NO Models field (callPlanner derives the model).
  planner.go          # DOES NOT EXIST YET — THIS TASK CREATES IT.
internal/prompt/
  planner.go          # SHIPPED (P3.M1.T1.S1) — READ (CONSUMED): BuildPlannerSystemPrompt/UserPayload,
                      #   ParsePlannerOutput, PlannerRetryInstruction, PlannerOutput, PlannerCommit.
  system.go           # READ (precedent): BuildSystemPrompt (the v1 style-example builder the planner
                      #   REUSES THE SOURCE of — RecentMessages — but NOT the builder itself).
internal/generate/
  generate.go         # READ (CLOSEST PATTERN): CommitStaged step 5 (the loop callPlanner mirrors) +
                      #   buildSystemPrompt/recentSubjects (the isUnborn short-circuit precedent).
internal/provider/
  render.go           # READ (CONSUMED): Manifest.Render(model,provider,sys,payload,mode...), RenderBare.
  executor.go         # READ (CONSUMED): provider.Execute(ctx, spec, timeout, vb) → (stdout, stderr, err).
internal/git/
  git.go              # READ (CONSUMED): WorkingTreeDiff, RecentMessages, StagedDiffOptions.
internal/config/
  config.go           # READ (CONSUMED): Config (MaxCommits, diff caps, Timeout), config.Defaults().
  roles.go            # READ (CONSUMED): ResolveRoleModel(role, cfg) → (provider, model).
internal/stubtest/
  stubtest.go         # READ (test infra): Build, Manifest, NewScript (the stub agent seam).
internal/generate/
  generate_test.go    # READ (test pattern): fixture helpers + TestCommitStaged_Success (copy helpers).
go.mod / go.sum       # UNCHANGED (go 1.22; stdlib + config/git/prompt/provider — all already in roles.go).
.golangci.yml         # READ: errcheck/gosimple/govet/ineffassign/staticcheck/unused.
```

### Desired Codebase tree with files to be added

```bash
internal/decompose/planner.go          # NEW — package `decompose` (2nd file); the planner agent call:
                                       #   var ErrPlannerFailed = errors.New("decompose: planner failed")
                                       #   func callPlanner(ctx, deps Deps, forcedCount int, isUnborn bool) (prompt.PlannerOutput, error)
                                       #   func validatePlannerOutput(out prompt.PlannerOutput) error          (private)
                                       #   func plannerExamples(ctx, g git.Git, isUnborn bool) ([]string, error) (private — RecentMessages short-circuit)
internal/decompose/planner_test.go     # NEW — stubtest + real-git integration tests (fixture helpers
                                       #   copied from generate_test.go). Cases: happy multi-commit;
                                       #   single-shortcut; forced-count; parse-retry-then-success;
                                       #   safety-cap; single-without-message; unparseable-after-retry;
                                       #   timeout; unborn nil-examples.
# go.mod/go.sum UNCHANGED. roles.go + prompt/* + provider/* + git/* + config/* + cmd/* + pkg/stagehand all UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (Deps has NO Models field — findings §2/§3): the SHIPPED internal/decompose/roles.go defines
//   Deps {Git, Registry, Config, Roles RoleManifests, Verbose} — there is NO Models/RoleModels field.
//   (RoleModels is ResolveRoles's 2nd return value; the orchestrator retains it locally.) callPlanner
//   takes ONLY Deps, so it CANNOT read a pre-resolved planner (provider, model). THE RULE: derive it via
//   `prov, mdl := config.ResolveRoleModel("planner", deps.Config)` — the SAME function ResolveRoles uses.
//   Do NOT add a Models field to Deps (it is owned by the shipped parallel task — editing roles.go is a
//   conflict). Do NOT add a model param to callPlanner (the contract signature is fixed). The derivation
//   is correct for every reachable case (FR-R5b guards the dangerous bare-model-no-provider-on-pi case at
//   ResolveRoles time, BEFORE the orchestrator calls callPlanner; config-init always sets a provider).

// CRITICAL (the planner is BARE — findings §5): Render the planner manifest with provider.RenderBare
//   (the planner is the bare role per §13.6.2; only the STAGER is tooled). deps.Roles.Planner.Render(mdl,
//   prov, sysPrompt, payload, provider.RenderBare). Render defaults to bare when mode is omitted, so
//   omitting it also works — but pass RenderBare explicitly to document intent.

// CRITICAL (callPlanner OWNS the single⇔message contract — findings §4/§6): prompt.ParsePlannerOutput's
//   doc explicitly states "It does NOT validate the single⇔message contract — the caller
//   (decompose/planner.go) owns that decision." So validatePlannerOutput MUST enforce single==true ⇒
//   Message != "" (the FR-M11 shortcut is unusable without a message). A violation triggers the ONE retry
//   (same budget as a parse failure); if it persists → ErrPlannerFailed.

// CRITICAL (the safety-cap message is EXACT and NOT wrapped — findings §7): on an accepted output with
//   forcedCount==0 && Count > deps.Config.MaxCommits, return EXACTLY:
//     "planner proposed %d commits; exceeds max_commits (%d); use --commits or --max-commits"
//   (N=output.Count, M=deps.Config.MaxCommits). Do NOT wrap it in ErrPlannerFailed (it is a distinct,
//   actionable remediation, not a "planner failed"). Do NOT retry (a reasoning decision won't change).
//   Auto-mode ONLY (forcedCount==0); forced mode trusts --commits (the CLI/orchestrator validated it).

// CRITICAL (planner failure is NON-RESCUE — findings §8/§13.6.6): planning PRECEDES all staging, so NO
//   snapshot/tree exists when callPlanner fails. Do NOT import or reuse generate.RescueError. Wrap genuine
//   failures in ErrPlannerFailed; the orchestrator treats ANY callPlanner error as non-rescue (exit, print
//   the message — no recovery command). Timeout/cancel return ErrPlannerFailed IMMEDIATELY (no retry —
//   mirror generate: the agent was killed).

// GOTCHA (the retry budget is ONE retry = ≤2 attempts — findings §5): callPlanner does at most 2 Execute
//   calls (1 initial + 1 retry). The retry is triggered by ParsePlannerOutput error OR
//   validatePlannerOutput error (shared budget). On the retry, prepend PlannerRetryInstruction + "\n\n" to
//   the payload (mirror generate's retryInstr prepend). VerboseRetry on the retry. If the 2nd attempt also
//   fails (parse or contract) → ErrPlannerFailed.

// GOTCHA (timeout/cancel do NOT retry — findings §5): Execute returns context.DeadlineExceeded (timeout)
//   or context.Canceled (parent cancel). Return ErrPlannerFailed-wrapped IMMEDIATELY (no parse, no retry)
//   — mirror generate.CommitStaged (the agent was killed; a retry won't help). A non-zero EXIT
//   (*exec.ExitError) is different: fall through to ParsePlannerOutput (stdout may be partial/valid),
//   record the cause, and retry if parse/contract fails.

// GOTCHA (do NOT use provider.ParseOutput — findings §4/§5): the planner's output is JSON parsed by
//   prompt.ParsePlannerOutput (which does its own whole-string + brace-balanced extraction). v1's
//   provider.ParseOutput is for RAW-text commit messages (it consults the manifest's Output field).
//   callPlanner MUST call prompt.ParsePlannerOutput(out), NOT provider.ParseOutput. (This also means the
//   stub manifest's Output mode is irrelevant in tests — the stub just emits JSON on stdout.)

// GOTCHA (style examples = RecentMessages ONLY — findings §4/§9): the §17.5 planner prompt has NEITHER a
//   hasMultiline rule NOR a subject-target line (unlike §17.1). So callPlanner passes ONLY
//   RecentMessages(ctx, 20) to BuildPlannerSystemPrompt — NOT DetectMultiline, NOT SubjectTargetChars.
//   (The contract's "same as v1 prompt: RecentMessages/DetectMultiline/BuildSystemPrompt" refers to the
//   style-example SOURCE — RecentMessages — which the planner reuses; the BUILDER is the planner's own.)
//   Short-circuit RecentMessages on isUnborn (return nil) — mirrors generate.buildSystemPrompt.
//   BuildPlannerSystemPrompt(nil) is nil-safe (planner prompt + a blank line, no "---" lines).

// GOTCHA (callPlanner does NOT gate on empty diff — findings §9): the orchestrator gates (FR-M1:
//   decomposition activates iff nothing staged AND the working tree has changes). callPlanner assumes a
//   non-empty diff. Do NOT add an empty-diff branch (that would duplicate the orchestrator's gate). If
//   WorkingTreeDiff returns "" (shouldn't happen when called correctly), the planner just gets an empty
//   payload — its problem, not callPlanner's.

// GOTCHA (package decompose is NEW-ish — planner.go is the 2nd file): roles.go is the 1st file (shipped).
//   planner.go adds to the SAME package (no `package` re-declaration issue — same `package decompose`).
//   Add a file doc comment (cite §13.6.2/§13.6.4/§13.6.6 + FR-M3/M4/M11). Do NOT redeclare helpers that
//   roles.go owns (computeInstalled/isMultiProvider/setRole) — callPlanner does not need them. No import
//   cycle (decompose already imports config/git/prompt/provider via roles.go).

// GOTCHA (test fixture helpers must be COPIED — findings §11): git's _test.go helpers are unimportable
//   (different package), and generate_test.go's helpers are in package generate (also unimportable from
//   package decompose). Copy initRepo/writeFile/stageFile/commitRaw/runGit/gitOut into planner_test.go
//   verbatim (generate_test owns its own copies for the same reason). Use UNSTAGED working-tree files
//   (write WITHOUT staging) for WorkingTreeDiff; commitRaw a few messages for style examples (or leave
//   unborn for the nil-examples path).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/decompose/planner.go — package decompose (the 2nd file after roles.go)

// ErrPlannerFailed is the sentinel for planner-agent failures (unparseable output after the one retry,
// single⇔message contract violation after the retry, agent non-zero exit after the retry, timeout,
// cancellation, or a render error). It is wrapped (%w) around the underlying cause so errors.Is works.
// The orchestrator (P3.M4.T1.S1) treats ANY callPlanner error as NON-RESCUE: planning precedes all
// staging, so no snapshot/tree exists when the planner fails (PRD §13.6.6 — "no commits have been made
// yet; surface the error and exit non-rescue"). The auto-mode safety-cap error is NOT wrapped in this
// sentinel (it is a distinct, actionable remediation) but is likewise non-rescue.
var ErrPlannerFailed = errors.New("decompose: planner failed")

// (callPlanner + validatePlannerOutput + plannerExamples — see Implementation Tasks)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/decompose/planner.go — package doc + imports + ErrPlannerFailed
  - FILE DOC: cite PRD §13.6.2 (planner role, bare, JSON) / §13.6.4 (single-shortcut) / §13.6.6 (planner
    failure = non-rescue) + §9.14 FR-M3/M4/M11. Note planner.go is the planner agent invocation
    (callPlanner), the decompose analogue of generate.CommitStaged's generation loop specialized to the
    planner's JSON output + the single-shortcut + the safety cap. Note it performs ZERO git mutations
    (planning precedes all staging).
  - IMPORTS: "context"; "errors"; "fmt"; "github.com/dustin/stagehand/internal/config";
    "github.com/dustin/stagehand/internal/git"; "github.com/dustin/stagehand/internal/prompt";
    "github.com/dustin/stagehand/internal/provider". (All already imported by roles.go — no new dep.)
  - DEFINE `var ErrPlannerFailed = errors.New("decompose: planner failed")` with the doc comment above.

Task 2: CREATE internal/decompose/planner.go — plannerExamples (private style-examples helper)
  - DEFINE `func plannerExamples(ctx context.Context, g git.Git, isUnborn bool) ([]string, error)`:
    if isUnborn { return nil, nil } (short-circuit — mirrors generate.buildSystemPrompt/recentSubjects);
    else `return g.RecentMessages(ctx, 20)`. Doc: the §17.5 style examples are RecentMessages(20) ONLY
    (no DetectMultiline/SubjectTargetChars — §17.5 omits both); nil on unborn (BuildPlannerSystemPrompt
    is nil-safe).
  - GOTCHA: do NOT call DetectMultiline or read SubjectTargetChars here — the planner prompt has neither.

Task 3: CREATE internal/decompose/planner.go — validatePlannerOutput (private; the single⇔message owner)
  - DEFINE `func validatePlannerOutput(out prompt.PlannerOutput) error`:
      if out.Count < 1 { return errors.New("planner output: count < 1") }
      if out.Single {
          if out.Message == "" { return errors.New("planner output: single==true but message is empty") }
      } else {
          if len(out.Commits) == 0 { return errors.New("planner output: single==false but no commits") }
      }
      return nil
  - DOC: cite FR-M11 + the ParsePlannerOutput doc ("the caller owns the single⇔message contract"). Explain
    the load-bearing check is single⇒message-present (the shortcut is unusable otherwise). A violation is
    treated as a contract failure that drives the ONE retry (same budget as a parse failure). Lenient on
    single==false + non-empty Message (harmless; orchestrator ignores it).

Task 4: CREATE internal/decompose/planner.go — callPlanner (the entry point)
  - SIGNATURE: `func callPlanner(ctx context.Context, deps Deps, forcedCount int, isUnborn bool) (prompt.PlannerOutput, error)`.
  - BODY (mirror generate.CommitStaged step 5; see Implementation Patterns for the exact loop):
      // 1. Derive the planner (provider, model) — Deps has no Models field (findings §3).
      prov, mdl := config.ResolveRoleModel("planner", deps.Config)
      // 2. Capture the working-tree diff (caps from cfg; mirror CommitStaged's StagedDiff call).
      diff, err := deps.Git.WorkingTreeDiff(ctx, git.StagedDiffOptions{
          MaxDiffBytes:     deps.Config.MaxDiffBytes,
          MaxMdLines:       deps.Config.MaxMdLines,
          BinaryExtensions: deps.Config.BinaryExtensions,
      })
      if err != nil { return prompt.PlannerOutput{}, fmt.Errorf("%w: working-tree diff: %v", ErrPlannerFailed, err) }
      // 3. Build the system prompt (style examples) + the base user payload.
      examples, err := plannerExamples(ctx, deps.Git, isUnborn)
      if err != nil { return prompt.PlannerOutput{}, fmt.Errorf("%w: recent messages: %v", ErrPlannerFailed, err) }
      sysPrompt := prompt.BuildPlannerSystemPrompt(examples)
      basePayload := prompt.BuildPlannerUserPayload(diff, forcedCount)
      // 4. The retry loop (≤2 attempts: 1 initial + 1 retry on parse/contract failure).
      const maxAttempts = 2
      var lastErr error
      for attempt := 0; attempt < maxAttempts; attempt++ {
          payload := basePayload
          if attempt > 0 {
              payload = prompt.PlannerRetryInstruction + "\n\n" + payload  // mirror generate's retryInstr prepend
              deps.Verbose.VerboseRetry(attempt, "planner output unparseable or contract-invalid")
          }
          spec, rerr := deps.Roles.Planner.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)
          if rerr != nil { return prompt.PlannerOutput{}, fmt.Errorf("%w: render: %v", ErrPlannerFailed, rerr) }
          out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
          if execErr != nil {
              if errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execErr, context.Canceled) {
                  return prompt.PlannerOutput{}, fmt.Errorf("%w: %v", ErrPlannerFailed, execErr) // non-rescue; no retry
              }
              lastErr = execErr // non-zero exit — fall through to parse (stdout may be partial)
          } else {
              lastErr = nil
          }
          parsed, perr := prompt.ParsePlannerOutput(out)
          if perr != nil { lastErr = perr; continue }            // parse failure → retry
          if verr := validatePlannerOutput(parsed); verr != nil { lastErr = verr; continue } // contract → retry
          // 5. Accepted output — enforce the safety cap (auto mode only) BEFORE returning.
          if forcedCount == 0 && parsed.Count > deps.Config.MaxCommits {
              return prompt.PlannerOutput{}, fmt.Errorf(
                  "planner proposed %d commits; exceeds max_commits (%d); use --commits or --max-commits",
                  parsed.Count, deps.Config.MaxCommits)
          }
          return parsed, nil                                       // SUCCESS
      }
      return prompt.PlannerOutput{}, fmt.Errorf("%w: %v", ErrPlannerFailed, lastErr)
  - DOC COMMENT: cite PRD §13.6.2/§13.6.4/§13.6.6 + FR-M3/M4/M11; diagram the pipeline (derive model →
    WorkingTreeDiff → system prompt + payload → Render bare → Execute → ParsePlannerOutput + validate →
    retry once → safety cap → return); note it is the decompose analogue of generate.CommitStaged's loop;
    note it performs ZERO git mutations (non-rescue on failure); note the model-derivation (findings §3);
    note the single⇔message ownership (validatePlannerOutput); note the safety cap is auto-mode-only.
  - GOTCHA: Render's mode param is variadic — pass `provider.RenderBare` explicitly (5th arg). ResolveRoleModel
    returns (provider, model); Render takes (model, provider) — pass (mdl, prov). `*spec` derefs the Render
    pointer. deps.Config.Timeout is the per-attempt timeout (Execute derives the context). deps.Verbose may
    be nil (ui.Verbose methods are nil-safe — confirmed by generate using deps.Verbose unconditionally).
  - GOTCHA: the safety-cap check is INSIDE the loop, after a successful parse+validate, and RETURNS
    immediately (no retry, no continue). It is checked on EVERY accepted output (attempt 0 or 1), which is
    correct — if the first attempt is unparseable (retry) and the second is valid-but-over-cap, the cap
    error fires. Good.

Task 5: CREATE internal/decompose/planner_test.go — fixture helpers (copied from generate_test.go)
  - IMPORTS: "context"; "errors"; "os"; "os/exec"; "strings"; "testing"; "time";
    "github.com/dustin/stagehand/internal/config"; "github.com/dustin/stagehand/internal/git";
    "github.com/dustin/stagehand/internal/prompt"; "github.com/dustin/stagehand/internal/stubtest".
    Package: `decompose` (internal test — callPlanner/validatePlannerOutput/plannerExamples visible).
  - COPY the fixture helpers from generate_test.go verbatim: initRepo, writeFile, stageFile, commitRaw,
    headSHA, runGit, gitOut (they are unimportable from package decompose; generate_test owns its own
    copies for the same reason). Use distinct names IF needed to avoid collisions (none exist yet in
    decompose — but when P3.M2.T3+ add test files, coordinate; for now planner_test.go owns them).
  - ADD a helper `func plannerDeps(t *testing.T, repo string, m provider.Manifest) Deps` that builds
    `Deps{Git: git.New(repo), Config: config.Defaults(), Roles: RoleManifests{Planner: m}, Verbose: nil}`.
    (provider import needed for the RoleManifests literal type — add it.)

Task 6: CREATE internal/decompose/planner_test.go — the test cases
  - TestCallPlanner_HappyMultiCommit: repo with a commit + UNSTAGED working-tree files; stub emits valid
    `{"count":3,"single":false,"commits":[{"title":"a","description":".."},{"title":"b",..},{"title":"c",..}]}`
    via stubtest.Manifest(bin, Options{Out: json}). Assert: nil error; out.Count==3; !out.Single;
    len(out.Commits)==3; out.Message=="".
  - TestCallPlanner_SingleShortcut (FR-M11): stub emits `{"count":1,"single":true,"commits":[{..}],"message":
    "feat: all in one"}`. Assert: out.Single==true; out.Message=="feat: all in one"; nil error.
  - TestCallPlanner_ForcedCount: callPlanner(ctx, deps, 3, false) with a stub emitting valid 3-commit JSON.
    To assert the forced directive reached the payload, capture the stub's stdin: extend the stub or use a
    stub variant that echoes stdin back — OR (simpler) assert callPlanner still parses normally (the
    directive is BuildPlannerUserPayload's job, already unit-tested in prompt/). Prefer a stdin-echo check
    if cheap; else assert the happy parse + note BuildPlannerUserPayload is covered by prompt tests.
  - TestCallPlanner_ParseRetryThenSuccess: stubtest.NewScript(t, bin, []string{invalidJSON, validJSON}).
    Assert: nil error; out == the valid JSON's parsed form. (Confirms the one retry with retry instruction.)
  - TestCallPlanner_SingleWithoutMessage_RetryThenSuccess: NewScript [`{"count":1,"single":true,"commits":
    []}` (no message — contract fail), `{"count":1,"single":true,"commits":[],"message":"feat: x"}`].
    Assert: nil error; out.Single==true; out.Message=="feat: x". (Confirms the single⇔message retry.)
  - TestCallPlanner_UnparseableAfterRetry: NewScript [bad, bad]. Assert: errors.Is(err, ErrPlannerFailed).
  - TestCallPlanner_SingleWithoutMessage_AfterRetry: NewScript [single-no-msg, single-no-msg]. Assert:
    errors.Is(err, ErrPlannerFailed). (Contract violation persists → ErrPlannerFailed.)
  - TestCallPlanner_SafetyCap_Auto (FR-M4): cfg := config.Defaults(); cfg.MaxCommits = 12; stub emits
    `{"count":15,"single":false,"commits":[15 entries]}`; callPlanner(ctx, deps, 0, false). Assert: non-nil
    err; err.Error() contains "planner proposed 15 commits" AND "exceeds max_commits (12)" AND "--commits";
    errors.Is(err, ErrPlannerFailed) == FALSE (distinct error).
  - TestCallPlanner_SafetyCap_ForcedSkips: same stub/count but callPlanner(ctx, deps, 15, false) (forcedCount
    = 15 > 0). Assert: nil error (forced mode trusts --commits; no cap). (Set cfg.MaxCommits=12; the count
    15 > 12 but forcedCount>0 ⇒ no cap.)
  - TestCallPlanner_Timeout: stub with Options{SleepMS: 2000}; cfg := config.Defaults(); cfg.Timeout =
    100*time.Millisecond; callPlanner. Assert: errors.Is(err, ErrPlannerFailed); the underlying cause is
    context.DeadlineExceeded (errors.Is(err, context.DeadlineExceeded) — the %w chain reaches it).
  - TestCallPlanner_UnbornNilExamples: repo with NO commits (isUnborn=true via RevParseHEAD, or just pass
    isUnborn=true) + unstaged file; stub emits valid single-shortcut JSON. Assert: nil error (the
    nil-examples path through BuildPlannerSystemPrompt(nil) does not panic; plannerExamples short-circuits).
    (Use isUnborn=true directly to exercise the short-circuit without needing a true unborn repo.)
  - GOTCHA: for tests that need a non-empty WorkingTreeDiff, create UNSTAGED files (writeFile WITHOUT
    stageFile). commitRaw a message or two first for a mature repo (style examples non-nil), or pass
    isUnborn=true for the nil path. The stub emits JSON regardless of the diff content.
  - GOTCHA: stubtest.Manifest's default Output is "raw" — fine (callPlanner uses ParsePlannerOutput, not
    provider.ParseOutput). The stub emits Options.Out verbatim on stdout. For JSON, set Out to the JSON
    string. NewScript's responses join with "\n" — ensure each response is a complete JSON object (the
    stub reads one line per call via the counter; a multi-line JSON won't work with NewScript — use
    single-line compact JSON for each response).

Task 7: VERIFY — build, vet, lint, format, full test suite
  - RUN: `go build ./...` (the new file compiles); `go vet ./...`; `golangci-lint run`;
    `gofmt -l internal/ pkg/` (must be empty — run `gofmt -w` on the new files); `go test ./...` (the new
    tests pass AND no existing test regressed — callPlanner adds no exported symbol that existing code
    imports, and edits no existing file).
  - GOTCHA: errcheck is enabled — check every error return (Render, Execute's execErr, WorkingTreeDiff,
    RecentMessages). unused/staticcheck: no unused helpers (validatePlannerOutput/plannerExamples must be
    called). Confirm `git status --short` shows exactly 2 changes (planner.go, planner_test.go).
```

### Implementation Patterns & Key Details

```go
// PATTERN — callPlanner's retry loop (mirror generate.CommitStaged step 5, with 3 planner deltas):
func callPlanner(ctx context.Context, deps Deps, forcedCount int, isUnborn bool) (prompt.PlannerOutput, error) {
	prov, mdl := config.ResolveRoleModel("planner", deps.Config) // Deps has no Models field (findings §3)

	diff, err := deps.Git.WorkingTreeDiff(ctx, git.StagedDiffOptions{
		MaxDiffBytes:     deps.Config.MaxDiffBytes,
		MaxMdLines:       deps.Config.MaxMdLines,
		BinaryExtensions: deps.Config.BinaryExtensions,
	})
	if err != nil {
		return prompt.PlannerOutput{}, fmt.Errorf("%w: working-tree diff: %v", ErrPlannerFailed, err)
	}

	examples, err := plannerExamples(ctx, deps.Git, isUnborn) // RecentMessages(20), nil on unborn
	if err != nil {
		return prompt.PlannerOutput{}, fmt.Errorf("%w: recent messages: %v", ErrPlannerFailed, err)
	}
	sysPrompt := prompt.BuildPlannerSystemPrompt(examples)
	basePayload := prompt.BuildPlannerUserPayload(diff, forcedCount)

	const maxAttempts = 2 // 1 initial + 1 retry on parse/contract failure
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		payload := basePayload
		if attempt > 0 {
			payload = prompt.PlannerRetryInstruction + "\n\n" + payload
			deps.Verbose.VerboseRetry(attempt, "planner output unparseable or contract-invalid")
		}

		spec, rerr := deps.Roles.Planner.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)
		if rerr != nil {
			return prompt.PlannerOutput{}, fmt.Errorf("%w: render: %v", ErrPlannerFailed, rerr)
		}

		out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execErr, context.Canceled) {
				return prompt.PlannerOutput{}, fmt.Errorf("%w: %v", ErrPlannerFailed, execErr) // non-rescue; no retry
			}
			lastErr = execErr // non-zero exit — stdout may be partial; fall through to parse
		} else {
			lastErr = nil
		}

		parsed, perr := prompt.ParsePlannerOutput(out)
		if perr != nil {
			lastErr = perr
			continue // parse failure → retry
		}
		if verr := validatePlannerOutput(parsed); verr != nil {
			lastErr = verr
			continue // single⇔message / count contract → retry (same budget)
		}

		// Accepted output — enforce the safety cap (auto mode only, FR-M4). No retry; exact message.
		if forcedCount == 0 && parsed.Count > deps.Config.MaxCommits {
			return prompt.PlannerOutput{}, fmt.Errorf(
				"planner proposed %d commits; exceeds max_commits (%d); use --commits or --max-commits",
				parsed.Count, deps.Config.MaxCommits)
		}
		return parsed, nil // SUCCESS
	}
	return prompt.PlannerOutput{}, fmt.Errorf("%w: %v", ErrPlannerFailed, lastErr)
}

// PATTERN — validatePlannerOutput (the single⇔message contract owner — findings §6):
func validatePlannerOutput(out prompt.PlannerOutput) error {
	if out.Count < 1 {
		return errors.New("planner output: count < 1")
	}
	if out.Single {
		if out.Message == "" {
			return errors.New("planner output: single==true but message is empty") // FR-M11 load-bearing
		}
	} else if len(out.Commits) == 0 {
		return errors.New("planner output: single==false but no commits")
	}
	return nil
}

// PATTERN — plannerExamples (style examples; nil on unborn — mirrors generate.buildSystemPrompt):
func plannerExamples(ctx context.Context, g git.Git, isUnborn bool) ([]string, error) {
	if isUnborn {
		return nil, nil // short-circuit (RecentMessages would exit 128 on unborn)
	}
	return g.RecentMessages(ctx, 20) // §17.5 uses RecentMessages ONLY (no DetectMultiline/SubjectTargetChars)
}

// CRITICAL: the safety-cap error is NOT wrapped in ErrPlannerFailed (distinct, actionable remediation);
//   it is auto-mode ONLY (forcedCount==0). The orchestrator surfaces it; it is non-rescue like all
//   callPlanner errors, but its message is the exact remediation the user acts on.
// CRITICAL: timeout/cancel return ErrPlannerFailed IMMEDIATELY (no retry) — mirror generate.CommitStaged
//   (the agent was killed; a retry won't help). A non-zero EXIT falls through to ParsePlannerOutput.
// CRITICAL: do NOT call provider.ParseOutput — the planner's JSON is parsed by prompt.ParsePlannerOutput.
```

### Integration Points

```yaml
PACKAGE (EXISTING — additive file):
  - create: "internal/decompose/planner.go (package decompose; 2nd file after roles.go)"
  - doc: "file doc comment citing PRD §13.6.2/§13.6.4/§13.6.6 + FR-M3/M4/M11"

CONSUMER (NOT THIS TASK — P3.M4.T1.S1):
  - the decompose orchestrator calls callPlanner(ctx, deps, forcedCount, isUnborn) once after building
    Deps (ResolveRoles) and confirming the decompose trigger (FR-M1).
  - callPlanner returns prompt.PlannerOutput; the orchestrator branches: Single==true ⇒ single-shortcut
    (git add -A → snapshot → commit-tree → update-ref with out.Message, FR-M11); else loop concepts[].
  - ANY callPlanner error ⇒ the orchestrator surfaces it and exits NON-RESCUE (no snapshot taken).
  - NO caller wiring in this task (do NOT touch cmd/ or pkg/stagehand/ or decompose.go).

NO DATABASE / NO CONFIG-FILE / NO ROUTE / NO git-mutation changes. go.mod/go.sum UNCHANGED. ZERO edits
to any shipped file (roles.go, prompt/*, provider/*, git/*, config/*, cmd/*, pkg/stagehand all CONSUMED).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after creating planner.go — fix before proceeding.
gofmt -w internal/decompose/planner.go internal/decompose/planner_test.go
go vet ./internal/decompose/...
go build ./...

# Expected: zero errors. The most likely failure is a signature/type mismatch (re-check the callPlanner
# signature against the contract; re-check Render's (model, provider) arg order vs ResolveRoleModel's
# (provider, model) return; re-check Execute's 3-tuple). If `decompose` fails to compile, confirm roles.go
# (Deps/RoleManifests) is present and unchanged.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test the new file as created.
go test ./internal/decompose/... -v

# Expected: all new planner_test.go cases pass. If a retry/cap/contract case fails, re-check findings
# §5/§6/§7 (the retry budget; the single⇔message validation; the exact cap message + non-wrap). If the
# stub emits multi-line JSON via NewScript, switch to single-line compact JSON (NewScript joins responses
# with "\n" and the stub reads one line per call). If a timeout test is flaky, ensure cfg.Timeout <
# SleepMS and that the %w chain reaches context.DeadlineExceeded.
```

### Level 3: Integration (No-Regressions Validation)

```bash
# The new file adds no exported symbol that existing code imports, and edits no existing file — but verify.
go test ./...
go vet ./...
golangci-lint run        # .golangci.yml: errcheck/gosimple/govet/ineffassign/staticcheck/unused
gofmt -l internal/ pkg/  # MUST be empty

# Confirm scope: exactly 2 changes, go.mod/go.sum untouched, roles.go untouched.
git status --short
git diff --stat          # expect planner.go (+), planner_test.go (+); nothing else

# Expected: the whole module builds + tests green; only 2 files changed; roles.go (the parallel task's
# output) is byte-for-byte unchanged; no existing behavior altered.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# (callPlanner performs ZERO git mutations — no real agent run needed for correctness. Level 4 is a
# contract-surface + design-coherence check.)

# Confirm the exported symbols the orchestrator (P3.M4.T1.S1) will consume:
rg -n 'var ErrPlannerFailed|func callPlanner|func validatePlannerOutput|func plannerExamples' internal/decompose/planner.go

# Confirm callPlanner is BARE (RenderBare) and uses prompt.ParsePlannerOutput (NOT provider.ParseOutput):
rg -n 'provider\.RenderBare|prompt\.ParsePlannerOutput|provider\.ParseOutput' internal/decompose/planner.go
# Expected: RenderBare present; ParsePlannerOutput present; provider.ParseOutput ABSENT.

# Confirm the model derivation (ResolveRoleModel) and the exact safety-cap message:
rg -n 'ResolveRoleModel|exceeds max_commits' internal/decompose/planner.go

# Confirm callPlanner does NOT touch generate.RescueError / git mutations (non-rescue):
rg -n 'RescueError|WriteTree|CommitTree|UpdateRef|AddAll' internal/decompose/planner.go
# Expected: NO matches (planning precedes all staging — callPlanner is read-only w.r.t. git refs/index,
#           except the read-only WorkingTreeDiff/RecentMessages).

# Expected: all symbols present; the planner is bare; no git mutations; no RescueError reuse.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go build ./...` succeeds (the new file compiles against the shipped roles.go).
- [ ] `go test ./...` GREEN (new tests pass + NO existing test regressed).
- [ ] `go vet ./...` clean.
- [ ] `golangci-lint run` clean (errcheck: every error checked; unused: no dead helpers).
- [ ] `gofmt -l internal/ pkg/` empty.
- [ ] go.mod / go.sum UNCHANGED.

### Feature Validation

- [ ] All success criteria from "What" met (ErrPlannerFailed + callPlanner + validatePlannerOutput +
      plannerExamples).
- [ ] Happy multi-commit: valid JSON partition ⇒ returned verbatim; nil error.
- [ ] Single-shortcut (FR-M11): single==true + Message ⇒ returned; nil error.
- [ ] Forced-count: forcedCount>0 ⇒ BuildPlannerUserPayload prepends the directive; callPlanner parses.
- [ ] Parse retry: bad-then-good ⇒ one retry ⇒ success.
- [ ] single⇔message: single-without-message ⇒ retry; persists ⇒ ErrPlannerFailed.
- [ ] Safety cap (FR-M4): auto-mode Count>MaxCommits ⇒ exact message, NOT wrapped, no retry; forced ⇒ no cap.
- [ ] Timeout: DeadlineExceeded ⇒ ErrPlannerFailed (non-rescue); the %w chain reaches context.DeadlineExceeded.
- [ ] All genuine failures wrapped in ErrPlannerFailed; the cap error is distinct; none involve a snapshot.

### Code Quality Validation

- [ ] Follows existing conventions (mirrors generate.CommitStaged's loop; errcheck-clean; %w wrapping).
- [ ] File placement matches the desired tree (internal/decompose/planner.go + planner_test.go).
- [ ] Anti-patterns avoided (no RescueError reuse; no provider.ParseOutput; no Deps edit; no model param;
      no DetectMultiline for the planner; no empty-diff gate; no git mutations).
- [ ] No import cycle (decompose already imports config/git/prompt/provider via roles.go).
- [ ] Doc comments cite the PRD § + the FR for every exported symbol.

### Documentation & Deployment

- [ ] File doc + per-symbol doc comments are self-documenting (cite §13.6.2/§13.6.4/§13.6.6 + FR-M3/M4/M11).
- [ ] The model-derivation decision (ResolveRoleModel — Deps has no Models field) + the non-rescue
      semantics + the single⇔message ownership are documented in code so future maintainers understand.
- [ ] No new env vars / config keys (callPlanner is pure invocation logic over existing config/manifests).

---

## Anti-Patterns to Avoid

- ❌ Don't add a `Models`/`RoleModels` field to `Deps` — it is owned by the shipped `roles.go` (parallel
  task P3.M2.T1.S1); editing roles.go is a conflict. Derive the planner model via `ResolveRoleModel`.
- ❌ Don't add a model/provider param to `callPlanner` — the contract signature is fixed
  (`callPlanner(ctx, deps Deps, forcedCount int, isUnborn bool)`); the model comes from `deps.Config`.
- ❌ Don't call `provider.ParseOutput` — the planner's JSON is parsed by `prompt.ParsePlannerOutput` (the
  provider parser is for raw-text commit messages; callPlanner must not use it).
- ❌ Don't reuse `generate.RescueError` / the snapshot machinery — planning precedes all staging; a planner
  failure is NON-RESCUE (no tree to rescue). Wrap failures in `ErrPlannerFailed`, not RescueError.
- ❌ Don't pass `DetectMultiline`/`SubjectTargetChars` to the planner prompt — §17.5 has neither; the
  planner uses `RecentMessages` only (its own builder, not §17.1's).
- ❌ Don't retry on timeout/cancel — return `ErrPlannerFailed` immediately (the agent was killed; mirror
  generate). Only parse failures and contract (single⇔message) failures consume the one retry.
- ❌ Don't wrap the safety-cap error in `ErrPlannerFailed` — it is a distinct, actionable remediation with
  an exact message; return it unwrapped (still non-rescue).
- ❌ Don't enforce the safety cap in forced mode (`forcedCount>0`) — the user explicitly set `--commits`;
  the cap is auto-mode-only. Don't retry on a cap violation (a reasoning decision won't change).
- ❌ Don't gate callPlanner on an empty diff — the orchestrator gates (FR-M1). Don't add a duplicate gate.
- ❌ Don't perform ANY git mutation (WriteTree/CommitTree/UpdateRef/AddAll) in callPlanner — planning is
  read-only w.r.t. refs/index (only the read-only WorkingTreeDiff/RecentMessages).
- ❌ Don't wire the orchestrator (cmd/, pkg/stagehand/, decompose.go) — this task ONLY implements
  callPlanner; P3.M4.T1.S1 consumes it.
- ❌ Don't swallow errors (errcheck is on) — check Render, Execute's execErr, WorkingTreeDiff,
  RecentMessages; wrap genuine failures with `%w` + ErrPlannerFailed.

---

## Confidence Score

**9/10** — one-pass success is highly likely. callPlanner is a well-scoped generalization of the PROVEN
v1 `generate.CommitStaged` generation loop (Render bare → Execute → parse → retry once → accept), with
three planner-specific deltas that are each fully resolved in the findings: (1) JSON parse via
`prompt.ParsePlannerOutput` (a shipped, tested function); (2) the single⇔message contract (the caller-
owned validation, encoded in `validatePlannerOutput`); (3) the auto-mode safety cap (exact message,
non-wrap, no-retry). The one design decision with a moving part — deriving the planner model from
`deps.Config` via `ResolveRoleModel` (because Deps has no Models field) — is forced by the shipped
parallel task's frozen Deps and is correct for every reachable case (FR-R5b guards the dangerous
misconfiguration at ResolveRoles time, before callPlanner runs; config-init always sets a provider). No
new dependency, no import cycle, ZERO edits to any shipped file (additive file in an existing package),
no caller wiring → blast radius is tiny. The one residual uncertainty is stubtest JSON test determinism
(NewScript joins responses with "\n" — use single-line compact JSON per response), mitigated by the
documented compact-JSON guidance.
