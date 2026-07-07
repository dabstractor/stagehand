---
name: "P3.M4.T1.S1 — Implement Decompose() orchestrator (full pipeline + safety cap + planner failure): internal/decompose/decompose.go (PRD §13.6, §11.4, §9.14 FR-M1/M2/M4/M11, §13.6.6)"
description: |

  CREATE `internal/decompose/decompose.go` + `internal/decompose/decompose_test.go`, and EDIT
  `internal/decompose/roles.go` (add ONE unexported test-seam field to Deps). decompose.go is the
  TOP-LEVEL orchestrator that composes the four-role multi-commit pipeline into one entry point
  (PRD §13.6 / §11.4 / §9.14): it routes by mode, runs the planner, takes the single-call shortcut
  when the planner judges N=1, runs the 1-deep-overlapped per-concept loop (stage→freeze→generate→
  publish), and wires the arbiter for leftover reconciliation. It is the single place overlap
  goroutine scheduling lives — every sibling (callPlanner/stageConcept/generateMessage/publishCommit/
  runArbiter/resolveArbiter) is a SIGNAL-FREE synchronous primitive consumed here.

  CONTRACT (P3.M4.T1.S1, verbatim from the work item):
    1. RESEARCH NOTE: Decompose (§13.6, §11.4) is the top-level orchestrator. Trigger (FR-M1):
       activates iff nothing staged AND working tree has changes. Modes (FR-M2): auto-decompose
       (planner decides), forced-count (--commits N), single (--single bypasses planner). Safety cap
       (FR-M4): max_commits default 12. Single-shortcut (FR-M11): planner returns single==true → use
       planner's message directly via git add -A → CommitStaged. Planner failure (§13.6.6): nothing
       snapshotted yet, surface error, exit non-rescue. The orchestrator composes: ResolveRoles →
       callPlanner → [single-shortcut OR loop(stageConcept→freezeSnapshot→generateMessage→CommitTree→
       UpdateRefCAS)] → [StatusPorcelain empty? done : runArbiter→resolveArbiter].
    2. INPUT: callPlanner from P3.M2.T2.S1, the loop from P3.M2.T4.S1, runArbiter+resolveArbiter from
       P3.M3, ResolveRoles from P3.M2.T1.S1.
    3. LOGIC: Create internal/decompose/decompose.go. Implement
       `func Decompose(ctx context.Context, deps Deps) (DecomposeResult, error)`. Logic:
       (1) if deps.Config.Single or deps.Config.Commits==1 → delegate to v1 path (AddAll →
       generate.CommitStaged).
       (2) ResolveRoles (or receive pre-resolved).
       (3) callPlanner(forcedCount=deps.Config.Commits).
       (4) If planner output single==true → AddAll → CommitStaged with planner's message (fall back to
       standard message agent on duplicate failure).
       (5) Safety cap check.
       (6) Run the loop (P3.M2.T4.S1 logic) producing []CommitResult.
       (7) StatusPorcelain check.
       (8) If non-empty → runArbiter → resolveArbiter.
       (9) Return DecomposeResult{Commits, Amended}. Define DecomposeResult struct matching pkg/stagecoach.
    4. OUTPUT: Decompose() is the entry point for the multi-commit pipeline. Returns the ordered
       commits created this run.
    5. DOCS: none — internal orchestrator.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/decompose/{planner,stager,message,arbiter}.go — SHIPPED. CONSUMED VERBATIM:
      callPlanner, stageConcept, freezeSnapshot, generateMessage, publishCommit, runArbiter,
      CommitInfo, all the Err*Failed sentinels. DO NOT EDIT.
    - internal/decompose/chain.go + chain_test.go — PARALLEL-OWNED (P3.M3.T2.S1). CONSUMED (assume
      exists when this task runs): resolveArbiter(ctx,deps,target *string,commits []CommitInfo,
      chainData []ChainEntry) error; ChainEntry{SHA,Tree,Message,Parent}; findTargetIndex;
      ErrArbiterResolutionFailed. DO NOT EDIT (merge conflict with the running parallel task).
    - internal/git/git.go — PARALLEL-OWNED (P3.M3.T2.S1 adds `Add`). DO NOT EDIT THIS CYCLE. Consume
      the shipped methods only (RevParseHEAD/RevParseTree/WriteTree/CommitTree/UpdateRefCAS/DiffTree/
      StatusPorcelain/HasStagedChanges/AddAll, EmptyTreeSHA, ErrCASFailed). CANNOT add a SHA-list
      method (would conflict) — see DecomposeResult gap (§G-RESULT).
    - internal/decompose/roles.go — EDIT (this task): add ONE unexported field `stager` (test seam) to
      Deps. Safe — parallel task owns chain.go+git.go, NOT roles.go.
    - internal/generate/generate.go — CONSUMED (read-only): generate.CommitStaged (single escape-hatch
      delegation), generate.Deps, generate.Result, generate.ExtractSubject, generate.RescueError,
      generate.CASError, generate.ErrNothingToCommit.
    - internal/prompt/planner.go — CONSUMED (read-only): PlannerOutput{Count,Single,Commits,Message},
      PlannerCommit{Title,Description}.
    - internal/cmd/root.go, pkg/stagecoach/ — UNCHANGED. CLI routing (FR-M1 trigger) + the public
      Decompose API are P4 (P4.M1.T1.S1, P4.M2.T1.S1).

  DELIVERABLES (3 git changes):
    CREATE internal/decompose/decompose.go — package `decompose`; `DecomposeResult` + `CommitResult`
      types; `Decompose(ctx, deps) (DecomposeResult, error)`; the single escape-hatch (→CommitStaged);
      the FR-M11 single-shortcut (dup-check + message-agent fallback); the 1-deep-overlap loop with
      FR-M8 empty-skip + serialized CAS publication; the arbiter wiring (StatusPorcelain gate →
      runArbiter → resolveArbiter); private helpers (runLoop, runSingleShortcut, invokeStager,
      dupCheckMessage, computeAmended, buildCommitResult, drainMsg).
    CREATE internal/decompose/decompose_test.go — dcm*-prefixed fixtures (DISTINCT from arb*/chn*/
      msg*/stg*/planner); integration tests covering all routing branches + the overlapped happy path
      (via the stager seam) + FR-M8 empty-skip + planner failure + safety cap + single-shortcut dup
      fallback + arbiter wiring + error propagation (RescueError/CASError/stager error).
    EDIT internal/decompose/roles.go — add unexported `stager func(context.Context, Deps,
      prompt.PlannerCommit) error` field to Deps (nil → orchestrator uses package stageConcept). A
      2-line addition (field + doc comment); NO behavior change for production (CLI builds Deps without it).

  SUCCESS: `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; `make coverage-gate` unaffected (decompose is NOT in the 4 gated
  packages); go.mod/go.sum UNCHANGED (generate/prompt/config/git/provider/ui all already imported by
  siblings); all existing tests still pass; Decompose(--single) delegates to CommitStaged (1 commit,
  ErrNothingToCommit/RescueError/CASError propagated); Decompose(auto, planner single==true) uses the
  planner's message verbatim when not a duplicate (NO separate message-agent call) and falls back to the
  message agent on duplicate; Decompose(auto, planner N concepts) creates N commits in order via the
  overlapped loop (stage[i] ∥ message[i-1]), HEAD advances N times via serialized CAS, no goroutine
  leak; FR-M8 empty concepts are skipped (no empty commits, commit count ≤ N); planner failure +
  safety-cap return a NON-RESCUE error (no *RescueError); a non-empty StatusPorcelain after the loop
  triggers runArbiter→resolveArbiter (tree ends clean); DecomposeResult.Commits is ordered oldest-first
  and accurate on the happy path; DecomposeResult.Amended reflects the arbiter's rewrite count.

---

## Goal

**Feature Goal**: Implement `Decompose(ctx, deps) (DecomposeResult, error)` — the top-level orchestrator
that turns an un-staged, dirty working tree into an ordered sequence of logically-coherent commits by
composing the four-role pipeline (planner → stager → message → arbiter) shipped in P3.M2/P3.M3. It is
the single entry point for the multi-commit pipeline (PRD §13.6 / §11.4 / §9.14): it routes by mode
(single escape-hatch / auto / forced), runs the planner, takes the FR-M11 single-call shortcut when the
planner judges N=1, drives the 1-deep-overlapped per-concept loop (stage→freeze→generate→publish) with
FR-M8 empty-skip and serialized CAS publication, and wires the arbiter (runArbiter→resolveArbiter) when
leftovers remain. It owns the ONLY concurrency in the package (the `stager[i+1] ∥ message[i]` overlap);
every sibling function is a synchronous primitive it calls.

**Deliverable** (3 git changes):
1. `internal/decompose/decompose.go` (NEW) — `DecomposeResult` + `CommitResult` types; `Decompose` entry
   point; `runLoop` (1-deep overlap); `runSingleShortcut` (FR-M11); `runSingleEscape` (→CommitStaged);
   private `invokeStager`, `dupCheckMessage`, `computeAmended`, `buildCommitResult`, `drainMsg`.
2. `internal/decompose/decompose_test.go` (NEW) — dcm*-prefixed integration tests (real temp git repo +
   stub agents + the stager seam).
3. `internal/decompose/roles.go` (EDIT) — add the unexported `stager` test-seam field to `Deps`.

**Success Definition**:
- **Single escape-hatch** (`Config.Single || Config.Commits==1`): Decompose calls `AddAll` then
  `generate.CommitStaged` with a `generate.Deps{Git, Manifest: Roles.Message, Verbose}`; on a repo with
  2 untracked files + stub message agent returning "feat: all", it creates exactly 1 commit whose
  subject == "feat: all" and whose tree contains both files; CommitStaged's typed errors
  (ErrNothingToCommit / *RescueError / *CASError) propagate verbatim (errors.As-able from Decompose's
  return).
- **Single-shortcut** (auto mode, planner returns `{Single:true, Message:"feat: one"}`): on a clean
  subject, Decompose creates 1 commit using the planner's message VERBATIM with ZERO separate
  message-agent Execute calls (verified by a call-counting stub); on a DUPLICATE subject, it falls back
  to `generateMessage` (the message agent) and uses the regenerated message.
- **Auto multi-commit** (planner returns N≥2 concepts, stager seam stages real files per concept):
  Decompose creates exactly N commits in order; `git log --oneline` shows the N messages from the stub
  message agent; HEAD advanced N times; the first concept's commit is a child of the pre-run HEAD (or a
  root commit on an unborn repo) and each subsequent commit's parent is the previous one's SHA;
  `runLoop` launches message[i] in a goroutine that overlaps stage[i+1] (verified by a stager seam that
  asserts a goroutine is in flight — or by timing); no goroutine leaks (`go test -race` clean, no
  blocked senders — channels are buffered(1)).
- **FR-M8 empty-skip**: if the stager seam stages nothing for concept i (tree[i]==tree[i-1]),
  Decompose skips commit i — the repo ends with ≤ N commits, never an empty one.
- **Planner failure / safety cap** (auto mode): a planner stub returning unparseable JSON (after
  callPlanner's one retry) makes Decompose return an error wrapping `ErrPlannerFailed`; a planner
  proposing Count=20 > MaxCommits(12) makes Decompose return the safety-cap error; in BOTH cases the
  error is NOT a `*generate.RescueError` (nothing was snapshotted — non-rescue, §13.6.6) and NO commit
  was created.
- **Arbiter wiring**: if the stager seam leaves a file un-staged, `git status --porcelain` is non-empty
  after the loop → Decompose calls `runArbiter` then `resolveArbiter`; afterward `git status --porcelain`
  is "" (clean). If the stager claimed everything, the arbiter does NOT run.
- **DecomposeResult**: `Commits` is ordered oldest-first, length == number of non-skipped concepts
  created in the loop, each `{SHA, Subject, Message, Files}` accurate; `Amended` == 0 when the arbiter
  didn't run (or made a new commit), == 1 for a tip amend, == (N - i) for a mid-chain rebuild at index i.

## User Persona

**Target User**: the CLI default-action router (P4.M1.T1.S1) and the public `pkg/stagecoach` API
(P4.M2.T1.S1), and through them the end user running `stagecoach` on an un-staged, dirty working tree.
decompose.go is internal orchestration — NOT user-facing text. The user invokes `stagecoach` (no
subcommand); the router sees nothing staged + a dirty tree (FR-M1) and calls Decompose.

**Use Case**: the user has made several unrelated changes and runs `stagecoach`; Decompose partitions
them into N coherent commits via the four-agent pipeline, publishes them in order, and reconciles any
leftovers — leaving a clean tree and a tidy linear history.

**User Journey**: (1) user edits files across concerns; (2) runs `stagecoach`; (3) router checks
`HasStagedChanges` (false) + dirty tree → Decompose; (4) Decompose runs planner → loop → (arbiter);
(5) user sees N `[<sha>] <subject>` lines + per-commit file-lists (FR42).

**Pain Points Addressed**: (a) one `stagecoach` invocation produces a coherent multi-commit history
without manual `git add` per concept; (b) the overlapped pipeline (stage[i+1] ∥ message[i]) keeps the
run fast without sacrificing the snapshot safety guarantee; (c) leftovers are never orphaned — the
arbiter folds them in or makes a new commit; (d) a misbehaving planner (runaway count / unparseable
output) fails fast and clean (non-rescue) before any commit is attempted.

## Why

- **Business value**: Decompose is the keystone of v2 (PRD §10.3 / G11) — it makes the four-role
  pipeline usable as a single command. Every sibling task (P3.M1 prompts, P3.M2 planner/stager/message,
  P3.M3 arbiter) is a primitive Decompose composes; without it the pipeline has no entry point.
- **Integration with existing features**: consumes callPlanner (P3.M2.T2.S1), stageConcept+freezeSnapshot
  (P3.M2.T3.S1), generateMessage+publishCommit (P3.M2.T4.S1), runArbiter (P3.M3.T1.S1), resolveArbiter
  (P3.M3.T2.S1 — parallel), ResolveRoles (P3.M2.T1.S1), and generate.CommitStaged (the v1 single-commit
  primitive, reused for the escape-hatch). It is the composition layer §11.3/§11.4 promised.
- **Problems this solves and for whom**: the plan-holder persona (§7.1) wants a clean history from a
  messy working tree in one command. Decompose delivers it while preserving the §13.6.7 safety proof
  (frozen trees, tree-to-tree diffs, serialized CAS refs, stager scoped to staging).

## What

**User-visible behavior**: `stagecoach` on an un-staged dirty tree produces N commits (or 1 via the
shortcut/escape-hatch), each with a generated subject and its file-list, in strict order, leaving a
clean tree. Modes: `--single`/`--commits 1` → one v1 commit; `--commits N` → exactly N; default → the
planner decides. A runaway planner (Count > 12) or a planner failure aborts before any commit with a
clear non-rescue error.

**Technical requirements**: `Decompose` is a package-level EXPORTED function (the eventual public-API
delegate) in `internal/decompose/decompose.go`. It (a) routes by mode; (b) delegates the single
escape-hatch to `generate.CommitStaged`; (c) calls `callPlanner`; (d) takes the FR-M11 shortcut; (e)
runs the 1-deep-overlap loop via `runLoop`; (f) gates the arbiter on `StatusPorcelain != ""`; (g) returns
`DecomposeResult`. It propagates `*generate.RescueError` / `*generate.CASError` UNWRAPPED (so the CLI's
`errors.As` works) and wraps infra failures in a new `ErrDecomposeFailed` sentinel. It is SIGNAL-FREE
for the loop/shortcut paths in S1 (signal arming in the loop is S2 — see §scope); the single escape-hatch
gets signal handling for free via `generate.CommitStaged`'s internal `signal.SetSnapshot`.

### Success Criteria

- [ ] `Decompose(ctx, deps)` with `Config.Single==true` (or `Commits==1`) calls `AddAll` then
      `generate.CommitStaged`; produces 1 commit; propagates CommitStaged's typed errors verbatim.
- [ ] In auto mode, if `callPlanner` returns `Single==true` with a non-duplicate `Message`, Decompose
      creates 1 commit using that message with ZERO separate message-agent calls (call-counted stub).
- [ ] In auto mode, if the planner's `Message` IS a duplicate subject, Decompose falls back to
      `generateMessage` and uses the regenerated message.
- [ ] In auto/forced mode with N≥2 concepts, `runLoop` creates N commits in order (oldest→newest), each
      a child of the previous; HEAD advances via N serialized CAS `update-ref`s.
- [ ] `runLoop` overlaps `stageConcept[i+1]` with `generateMessage[i]` (1-deep); channels are
      buffered(1); on ANY loop error the in-flight message goroutine is drained (no leak, `-race` clean).
- [ ] FR-M8: a concept whose staged tree equals the previous tree is skipped (no empty commit; final
      commit count ≤ N).
- [ ] Planner failure (unparseable after retry) and safety-cap (Count > MaxCommits) both return a
      NON-RESCUE error (NOT `*RescueError`) and create NO commit.
- [ ] After the loop, if `StatusPorcelain != ""`, Decompose calls `runArbiter` then `resolveArbiter`;
      the tree ends clean. If `StatusPorcelain == ""`, neither is called.
- [ ] `DecomposeResult.Commits` is ordered oldest-first, accurate on the happy path; `.Amended` is 0
      (no arbiter / new commit), 1 (tip amend), or N-i (mid-chain at i).
- [ ] `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
      `gofmt -l internal/ pkg/` empty; `make coverage-gate` still PASS (decompose not gated); go.mod/go.sum UNCHANGED.

## All Needed Context

### Context Completeness Check

_Before writing this PRP, validate: "If someone knew nothing about this codebase, would they have
everything needed to implement this successfully?"_ — YES. Every consumed symbol (with its exact
signature, file, and error contract), the 1-deep-overlap algorithm (with the drain-on-error rule), the
FR-M11 single-shortcut (with the dup-check + fallback), the DecomposeResult shape, the test-seam design,
and the validation gates are all specified below with exact file paths and line-level patterns. The
subtle points — overlap serialization, FR-M8 prevTree tracking, the single-shortcut≠escape-hatch
distinction, and the DecomposeResult-after-arbiter gap — are explained, not just named.

### Documentation & References

```yaml
# MUST READ — include these in your context window
- url: PRD.md §13.6.3 (the pipeline: sequential staging, overlapped generation)
  why: "The exact loop order + the THREE invariants (tree[i] frozen before stager[i+1]; concept diff is
        tree-to-tree; update-refs serialize). This IS the runLoop algorithm."
  critical: "Overlap is 1-DEEP (stager[i+1] ∥ message[i]), NOT unbounded. message[i] uses
        diff(tree[i-1], tree[i]) — never index-vs-HEAD. publish order is strictly i before i+1 (CAS chain)."

- url: PRD.md §13.6.4 (single-commit shortcut) + §9.14 FR-M11
  why: "When planner returns single==true + Message, use the planner's message directly (AddAll→snapshot
        →commit-tree→update-ref); fall back to the message agent ONLY if the message is a duplicate."
  critical: "The single-shortcut is NOT generate.CommitStaged (CommitStaged regenerates a message). It is
        a small inline path using publishCommit + a dup-check. Distinct from the --single ESCAPE-HATCH
        (planner bypassed → CommitStaged)."

- url: PRD.md §13.6.6 (failure handling within the loop) + §9.14 FR-M4/M12
  why: "Planner fails → exit NON-RESCUE (nothing snapshotted). Safety cap (FR-M4) → refuse >max_commits.
        Per-concept failure isolation (FR-M12) is S2 — S1 propagates structurally."
  critical: "S1's loop returns errors directly: stager err→wrapped ErrStagerFailed/ErrDecomposeFailed;
        message err→*RescueError; CAS err→*CASError. S2 wraps these in FR-M12 isolation. Do NOT implement
        retry-once-then-empty or rescue-for-concept-i here (S2)."

- url: PRD.md §13.6.5 (the arbiter) + §9.14 FR-M9
  why: "After the loop, if git status --porcelain is non-empty, run the arbiter. The arbiter DECIDES;
        resolveArbiter (chain.go, parallel) RESOLVES. Stagecoach does ALL git."
  critical: "Gate on StatusPorcelain() != ''. The arbiter does not run on the perfect run (clean tree).
        runArbiter takes (commits []CommitInfo, leftoverDiff string); resolveArbiter takes (target *string,
        commits, chainData []ChainEntry). Decompose unwraps runArbiter's ArbiterOutput.Target."

- url: PRD.md §9.14 FR-M1/M2 (trigger + modes)
  why: "FR-M1: decompose activates iff nothing staged + dirty tree (the ROUTER's job — P4). FR-M2 modes:
        auto (Commits==0), forced (Commits≥2), single (Single==true || Commits==1)."
  critical: "Decompose does NOT re-check FR-M1 (the router owns it) — it assumes the caller routed
        correctly. It DOES branch on Config.Single/Commits for mode routing. callPlanner's forcedCount =
        Config.Commits (0=auto, ≥2=forced; ==1 never reaches callPlanner — the escape-hatch catches it)."

# CODEBASE FILES — pattern sources + consumed dependencies (all verified, paths exact)
- file: internal/decompose/planner.go
  why: "CONSUMED: callPlanner(ctx, deps, forcedCount, isUnborn) (PlannerOutput, error). It ALREADY enforces
        the FR-M4 safety cap in auto mode (forcedCount==0 && Count>MaxCommits → error). ErrPlannerFailed
        sentinel. All callPlanner errors are NON-RESCUE (planning precedes all staging)."
  pattern: "Render→Execute→ParsePlannerOutput→validate→(safety cap)→return. The retry-once-on-parse-failure
        is INSIDE callPlanner — Decompose does NOT retry."
  gotcha: "callPlanner needs isUnborn (for plannerExamples short-circuit). Decompose derives it via
        RevParseHEAD BEFORE calling callPlanner. forcedCount = Config.Commits."

- file: internal/decompose/message.go
  why: "CONSUMED: generateMessage(ctx, deps, treeA, treeB) (string, error) [single-shortcut fallback +
        loop message gen] + publishCommit(ctx, deps, tree, parentSHA, msg) (string, error) [single-shortcut
        + loop publication]. generateMessage derives parent+isUnborn INTERNALLY + returns *RescueError
        DIRECTLY. publishCommit takes EXPLICIT parentSHA (the CAS expected-old) + returns newSHA or
        *CASError DIRECTLY."
  pattern: "Both are SIGNAL-FREE synchronous primitives. Decompose launches generateMessage in a goroutine
        (the overlap); publishCommit is called synchronously in publication order."
  gotcha: "publishCommit's parentSHA is the CAS expected-old = newSHA[i-1] (or preRunHEAD for i==0, or
        all-zeros for a root commit). Decompose tracks this across the loop. Do NOT re-read HEAD for the
        CAS expected-old (the re-read is ONLY for the §13.5 Actual after a CAS failure — publishCommit
        does that internally)."

- file: internal/decompose/stager.go
  why: "CONSUMED: stageConcept(ctx, deps, PlannerCommit) error [loop staging — via invokeStager seam] +
        freezeSnapshot(ctx, deps) (string, error) [loop snapshot]. stageConcept is TOOLED, no-retry,
        no-parse; mutates INDEX only. ErrStagerFailed sentinel."
  pattern: "stageConcept(concept) → freezeSnapshot() → tree[i]. The orchestrator MUST freeze BEFORE the
        next stageConcept (§13.6.3 invariant 1)."
  gotcha: "stageConcept has NO retry (the orchestrator owns FR-M8 retry-once-then-empty — S2). In S1 a
        stager error propagates. invokeStager(deps, concept) dispatches to deps.stager if non-nil else
        stageConcept (the test seam)."

- file: internal/decompose/arbiter.go
  why: "CONSUMED: runArbiter(ctx, deps, commits []CommitInfo, leftoverDiff string) (ArbiterOutput, error)
        + CommitInfo{SHA, Subject, Files []git.FileChange}. ErrArbiterFailed sentinel. runArbiter OWNS
        'when in doubt null' (returns {Target:nil} with nil error on indecision)."
  pattern: "runArbiter takes the []CommitInfo (the loop's commits) + the leftover diff (WorkingTreeDiff).
        Decompose unwraps ArbiterOutput.Target → resolveArbiter(ctx, deps, out.Target, commits, chainData)."
  gotcha: "runArbiter does ZERO git reads — commits + leftoverDiff are PARAMETERS. Decompose pre-computes
        leftoverDiff via deps.Git.WorkingTreeDiff (same caps as the planner). Build []CommitInfo from the
        loop's published commits (SHA + Subject + DiffTree files)."

- file: internal/decompose/chain.go   # PARALLEL (P3.M3.T2.S1) — assume it exists when this task runs
  why: "CONSUMED: resolveArbiter(ctx, deps, target *string, commits []CommitInfo, chainData []ChainEntry)
        error + ChainEntry{SHA,Tree,Message,Parent} + findTargetIndex(sha, chainData) int + sentinel
        ErrArbiterResolutionFailed. resolveArbiter reconciles leftovers → clean tree; returns ONLY error."
  pattern: "resolveArbiter dispatches: nil/not-found→new commit; tip→amend; earlier→mid-chain rebuild.
        Decompose builds []ChainEntry as it publishes each loop commit (it holds tree[i], msg[i], newSHA[i],
        parent) and passes it to resolveArbiter."
  gotcha: "resolveArbiter returns ONLY an error — NOT the new/amended SHAs. So Decompose CANNOT know the
        post-arbiter SHAs without a git re-read (and it can't add a SHA-list git method this cycle — git.go
        is parallel-owned). See §G-RESULT: Commits = loop result; Amended = count; document the gap."

- file: internal/decompose/roles.go   # EDIT TARGET (add the stager test-seam field)
  why: "EDIT: add ONE unexported field to Deps — `stager func(context.Context, Deps, prompt.PlannerCommit)
        error`. Deps{Git, Registry, Config, Roles RoleManifests, Verbose}. ResolveRoles(cfg, reg) populates
        Roles. Deps is the injectable-collaborators struct (mirrors generate.Deps{Manifest: stub})."
  pattern: "Add the field with a doc comment. NO other change. Production CLI builds Deps without it (nil).
        invokeStager: `if deps.stager != nil { return deps.stager(ctx, deps, c) }; return stageConcept(ctx, deps, c)`."
  gotcha: "roles.go is SHIPPED (P3.M2.T1.S1) but NOT parallel-owned this cycle (parallel owns chain.go +
        git.go). Editing roles.go is safe. The field is unexported (lowercase) — not part of any public API."

- file: internal/generate/generate.go
  why: "CONSUMED: generate.CommitStaged(ctx, generate.Deps{Git, Manifest, Verbose}, cfg) (Result, error)
        [single escape-hatch]. generate.Deps{Git git.Git; Manifest provider.Manifest; Verbose *ui.Verbose}.
        generate.Result{CommitSHA, Subject, Message, Provider, Model, Changes []git.FileChange}. Also:
        generate.ExtractSubject(msg), generate.RescueError, generate.CASError, generate.ErrNothingToCommit."
  pattern: "Map decompose.Deps → generate.Deps{Git: deps.Git, Manifest: deps.Roles.Message, Verbose:
        deps.Verbose}; pass deps.Config. Map Result → CommitResult{SHA:CommitSHA, Subject, Message,
        Files:Changes}. CommitStaged uses cfg.Model/cfg.Provider (GLOBAL — correct for v1 escape-hatch)."
  gotcha: "CommitStaged DOES NOT call AddAll (it assumes the index is staged) — Decompose MUST AddAll
        before delegating (contract step 1: 'AddAll → generate.CommitStaged'). CommitStaged arms signal
        internally (nil-safe if signal.Install wasn't called). CommitStaged uses StagedDiff (index-vs-HEAD)
        — after AddAll the index has everything, so it captures all changes."

- file: internal/prompt/planner.go
  why: "CONSUMED (read-only): PlannerOutput{Count int; Single bool; Commits []PlannerCommit; Message
        string} + PlannerCommit{Title, Description string}. Message is present iff Single==true."
  pattern: "out.Commits is the concept partition for the loop; out.Message is the single-shortcut message;
        out.Single routes to runSingleShortcut."

- file: internal/config/config.go
  why: "CONSUMED (read-only): Config fields — Single bool, Commits int (0/1/≥2), MaxCommits int (12),
        Timeout, MaxDiffBytes/MaxMdLines/BinaryExtensions, MaxDuplicateRetries. All flow through to
        callPlanner/generateMessage (caps) + mode routing."
  pattern: "Mode routing: `if deps.Config.Single || deps.Config.Commits == 1 { return runSingleEscape(...) }`.
        forcedCount for callPlanner = deps.Config.Commits."

- file: internal/git/git.go   # CONSUMED (do NOT edit — parallel-owned this cycle)
  why: "CONSUMED: RevParseHEAD()→(sha,isUnborn,err); RevParseTree(ref)→(tree,err) ['' on unborn];
        WriteTree(); CommitTree(tree,parents,msg); UpdateRefCAS(ref,new,expected); DiffTree(sha,isRoot)
        →[]FileChange; StatusPorcelain()→(string,err); HasStagedChanges()→(bool,err); AddAll();
        WorkingTreeDiff(ctx, opts)→(string,err). Consts: git.EmptyTreeSHA, git.ErrCASFailed."
  pattern: "baseTree = isUnborn ? git.EmptyTreeSHA : RevParseTree('HEAD'). preRunHEAD from RevParseHEAD.
        isRoot for DiffTree = (i==0 && isUnborn)."
  gotcha: "RevParseTree returns ('', nil) on unborn (NOT an error). StatusPorcelain returns '' on a clean
        tree (compare to ''). Do NOT add any method to git.go this cycle (parallel conflict)."

- file: internal/decompose/arbiter_test.go   # PATTERN for decompose_test.go fixtures
  why: "PATTERN: arb*-prefixed fixture helpers (arbInitRepo/arbWriteFile/arbStageFile/arbCommitRaw/
        arbRunGit/arbCommits) + arbDeps(Deps{Git,Config,Roles:RoleManifests{...}}). Real temp git repo
        (t.TempDir); repo-local identity; stubtest.Build + stubtest.Manifest for agents; assert via
        arbRunGit (log/rev-parse/status)."
  pattern: "decompose_test.go MUST use dcm*-prefixed helpers (dcmInitRepo, dcmWriteFile, ...) copied from
        arbiter_test.go and renamed — DISTINCT from arb*/chn*/msg*/stg*/planner. See §G-PREFIX."
  gotcha: "Tooled stub manifest: stubtest.Manifest + `m.TooledFlags = []string{\"--tooled\"}` (see
        tooledStubManifest in stager_test.go:76). The 4-role Deps needs all 4 RoleManifests populated."

- file: internal/decompose/message_test.go   # PATTERN for tree-building + script stubs
  why: "PATTERN: msgGitOut(repo,'write-tree') to freeze trees; stubtest.NewScript(t,bin,[]string{...}) for
        call-varying agent responses (e.g. dup-then-fresh). messageDeps(repo, m) builds Deps."
  pattern: "For the single-shortcut dup-fallback test: a script returning [dupSubject, freshSubject] —
        generateMessage (the fallback) consumes them in order."

- file: internal/stubtest/stubtest.go   # the test harness
  why: "stubtest.Build(t) → bin path; stubtest.Manifest(bin, Options{Out,Exit,SleepMS,Script,Counter,...})
        → provider.Manifest pointing at the stub; stubtest.NewScript(t,bin,responses) → call-varying
        manifest. The stub CANNOT run git (it prints canned output) — hence the stager seam (§G-SEAM)."
  pattern: "planner stub: Options{Out: `{\"count\":2,\"single\":false,\"commits\":[...]}`} (valid JSON).
        message stub: Options{Out: 'feat: x'} or NewScript([...]). arbiter stub: Options{Out:
        '{\"target\":null}'} or '{\"target\":\"<sha>\"}'. stager stub: real staging via deps.stager seam."
```

### Current Codebase tree (relevant subset)

```bash
internal/
  decompose/
    roles.go        # EDIT (this task): add unexported `stager` field to Deps
    planner.go      # SHIPPED — callPlanner, ErrPlannerFailed (CONSUMED)
    stager.go       # SHIPPED — stageConcept, freezeSnapshot, ErrStagerFailed (CONSUMED)
    message.go      # SHIPPED — generateMessage, publishCommit, ErrMessageFailed (CONSUMED)
    arbiter.go      # SHIPPED — runArbiter, CommitInfo, ErrArbiterFailed (CONSUMED)
    chain.go        # PARALLEL (P3.M3.T2.S1) — resolveArbiter, ChainEntry (CONSUMED, assume exists)
    decompose.go    # ← NEW (this task): Decompose, DecomposeResult, runLoop, runSingleShortcut, ...
    decompose_test.go # ← NEW (this task): dcm* fixtures + integration tests
  git/git.go        # CONSUMED (do NOT edit — parallel-owned)
  generate/generate.go # CONSUMED — CommitStaged, Result, ExtractSubject, RescueError, CASError
  prompt/planner.go # CONSUMED — PlannerOutput, PlannerCommit
  config/config.go  # CONSUMED — Config{Single,Commits,MaxCommits,...}
  stubtest/stubtest.go # test harness (Build, Manifest, NewScript)
pkg/stagecoach/stagecoach.go # UNCHANGED (P4.M2.T1.S1 adds the public Decompose API later)
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
internal/decompose/decompose.go     # NEW — DecomposeResult + CommitResult types; Decompose() entry point
                                     #   (mode routing → single escape-hatch / single-shortcut / loop →
                                     #   arbiter wiring); runLoop (1-deep overlap + FR-M8 empty-skip +
                                     #   serialized CAS); runSingleShortcut (FR-M11 dup-check + fallback);
                                     #   runSingleEscape (→CommitStaged); private invokeStager,
                                     #   dupCheckMessage, computeAmended, buildCommitResult, drainMsg.
internal/decompose/decompose_test.go # NEW — dcm* fixtures + integration tests for every routing branch,
                                     #   the overlapped happy path (stager seam), FR-M8, planner failure,
                                     #   safety cap, single-shortcut dup fallback, arbiter wiring, errors.
internal/decompose/roles.go         # EDIT — add unexported `stager` test-seam field to Deps (+ doc comment).
```

### Known Gotchas of our codebase & Library Quirks

```go
// G-OVERLAP (CRITICAL): the overlap is 1-DEEP — stager[i+1] ∥ message[i] — NOT unbounded. At most ONE
//   message goroutine is in flight at a time. The pipeline: stage[i] (msg[i-1] in flight) → freeze
//   tree[i] → drain+publish msg[i-1] (CAS) → launch msg[i]. Publication is STRICTLY ordered (commit[i-1]
//   before commit[i]) because each CAS requires HEAD==newSHA[i-1]. Channels MUST be buffered(1) so the
//   goroutine never blocks on send (it sends exactly once then exits). On ANY loop error, drain the
//   in-flight channel (`<-ch`) before returning to avoid a goroutine leak (the goroutine has already sent
//   to the buffered channel, so the receive is non-blocking; but receiving documents intent + lets you
//   inspect a message error before returning a stage error if S2 wishes).

// G-EMPTY-SKIP (FR-M8): a concept is skipped iff tree[i] == prevTree (the stager staged nothing new).
//   prevTree starts at baseTree and advances to tree[i] ONLY for non-skipped concepts. On skip: do NOT
//   launch a message, do NOT publish; prevTree is unchanged (it already == tree[i]). BUT the pending
//   message[i-1] (if any) MUST still be drained+published — the empty concept[i] does not cancel it.
//   Final commit count ≤ N (skips reduce it). NOTE: this is FR-M8 part (a) — the tree-comparison skip.
//   Part (b) (stager non-zero-exit → retry once → treat as empty) is S2 (FR-M12); in S1 a stager error
//   propagates (it does NOT silently skip).

// G-SHORTCUT-VS-ESCAPE (CRITICAL): TWO different single-commit paths — do not conflate.
//   (1) SINGLE ESCAPE-HATCH (Config.Single==true || Config.Commits==1): planner is BYPASSED entirely.
//       AddAll → generate.CommitStaged (v1 behavior). CommitStaged GENERATES its own message.
//   (2) SINGLE-SHORTCUT (auto mode, planner returned Single==true + Message): the planner ALREADY ran and
//       produced a message. Use it DIRECTLY (dup-check first; fallback to generateMessage only on dup).
//       AddAll → WriteTree → dup-check Message → publishCommit. ZERO separate message-agent call on a
//       clean subject (the shortcut's whole point — one agent round-trip).

// G-DUP-CHECK (FR-M11 fallback): the single-shortcut checks IsDuplicate(ExtractSubject(planner.Message),
//   RecentSubjects(50)). NOT a duplicate → use planner.Message verbatim. IS a duplicate → call
//   generateMessage(ctx, deps, baseTree, treePrime) to regenerate (it runs the full message-agent loop);
//   if THAT returns *RescueError, propagate it DIRECTLY. This is the ONLY place Decompose calls
//   generateMessage outside the loop. Use generate.IsDuplicate + generate.ExtractSubject (same as
//   generateMessage internally) for consistency.

// G-PLANNER-NONRESCUE (CRITICAL): ANY callPlanner error (ErrPlannerFailed wrap OR the safety-cap error)
//   is NON-RESCUE — planning precedes ALL staging, so NO snapshot/tree exists when the planner fails
//   (§13.6.6). Decompose returns the error directly; it is NOT a *RescueError and no commit was created.
//   Do NOT wrap callPlanner errors in *RescueError. (The CLI maps these to exit 1, not 3/124.)

// G-SAFETY-CAP: callPlanner ALREADY enforces FR-M4 in auto mode (forcedCount==0 && Count>MaxCommits →
//   error). In forced mode (Commits≥2) the user asserted the count → no cap (callPlanner skips the check
//   because forcedCount!=0). Decompose does NOT need a redundant cap check — it trusts callPlanner. The
//   cap error is a distinct, actionable error (NOT wrapped in ErrPlannerFailed) and is non-rescue.

// G-CAS-CHAIN: publishCommit's parentSHA (CAS expected-old) is newSHA[i-1] for i>0, preRunHEAD for i==0,
//   and all-zeros (strings.Repeat("0",40)) for a root commit (i==0 on an unborn repo). Decompose threads
//   this as `prevSHA` across the loop. publishCommit returns *generate.CASError DIRECTLY on CAS mismatch
//   (errors.As-able); in S1 the loop returns it (S2 adds abort-with-recovery-message). NEVER force-update.

// G-RESULT (CRITICAL — the post-arbiter gap): resolveArbiter returns ONLY an error (it's parallel-owned;
//   cannot change its signature). So after the arbiter runs, Decompose CANNOT know the post-arbiter SHAs
//   (tip amend / mid-chain rebuild rewrite SHAs; null adds an (N+1)-th) without a git re-read — and it
//   CANNOT add a SHA-list git method this cycle (git.go is parallel-owned). S1 DECISION:
//     - DecomposeResult.Commits = the loop's []CommitResult (original SHAs/messages/files) — ACCURATE on
//       the happy path (arbiter doesn't run) and for the loop's N commits generally.
//     - DecomposeResult.Amended = the arbiter's rewrite COUNT (0 if arbiter didn't run or made a new
//       commit; 1 for tip amend; N-i for mid-chain at index i), computed via findTargetIndex on chainData
//       (same package — accessible) BEFORE calling resolveArbiter.
//   Document: when Amended>0, the last `Amended` entries' SHAs are pre-amend (superseded); the null-path
//   (N+1)-th commit is created (tree ends clean) but not in Commits. P4 (public API) re-reads git for
//   final display. This is an honest, scoped S1 limitation.

// G-SEAM (testability): the stub agent (stubtest) CANNOT run git — so a stub stager stages nothing →
//   tree[i]==tree[i-1] → every concept is skipped → no commits (useless happy-path test). SOLUTION: the
//   unexported `stager` field on Deps (this task adds it). invokeStager(deps, concept) = deps.stager if
//   non-nil else stageConcept. Production builds Deps without it (nil → real stageConcept via the tooled
//   agent). Tests inject a stager that runs `git add` for the concept's files → full N-commit happy path.
//   SAFE to edit roles.go (parallel owns chain.go+git.go, NOT roles.go).

// G-ARBITER-INPUTS: runArbiter needs (a) []CommitInfo — build from the loop's published commits:
//   CommitInfo{SHA: newSHA, Subject: ExtractSubject(msg), Files: DiffTree(newSHA, isRoot)}. (b) leftoverDiff
//   = deps.Git.WorkingTreeDiff(ctx, caps) — the SAME caps as the planner (MaxDiffBytes/MaxMdLines/
//   BinaryExtensions). Gate the WHOLE arbiter block on StatusPorcelain() != "" (FR-M9). Build []ChainEntry
//   (parallel to []CommitInfo) for resolveArbiter as you publish: ChainEntry{SHA:newSHA, Tree:tree[i],
//   Message:msg, Parent:parentSHA}.

// G-PREFIX: decompose_test.go MUST use dcm*-prefixed fixture helpers (dcmInitRepo, dcmWriteFile,
//   dcmStageFile, dcmCommitRaw, dcmRunGit, dcmDeps) — DISTINCT from arb* (arbiter), chn* (chain),
//   stg* (stager), msg* (message), and un-prefixed (planner). The `decompose` test package shares ONE
//   package scope; duplicate helper names = compile errors.

// G-SIGNAL-FREE (S1 scope): Decompose's loop + single-shortcut paths are SIGNAL-FREE in S1 (no
//   internal/signal import). signal.RestoreDefault is one-shot + loop-signal is owned by S2 (P3.M4.T1.S2).
//   The single ESCAPE-HATCH gets signal handling for FREE via generate.CommitStaged's internal
//   signal.SetSnapshot (nil-safe if Install wasn't called). S2 will add loop signal arming + the
//   multi-commit rescue variant. Do NOT import internal/signal in decompose.go.

// G-MODE-FORCEDCOUNT: callPlanner's forcedCount = Config.Commits. The escape-hatch catches Commits==1
//   BEFORE callPlanner, so callPlanner only ever sees forcedCount==0 (auto) or ≥2 (forced). Never pass 1.

// G-ISUNBORN-DERIVE: callPlanner + the loop both need isUnborn + preRunHEAD. Derive ONCE at the top of
//   Decompose (after the escape-hatch check, before callPlanner) via RevParseHEAD. baseTree = isUnborn ?
//   git.EmptyTreeSHA : RevParseTree("HEAD"). isRoot for DiffTree = (conceptIndex==0 && isUnborn).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/decompose/decompose.go

// CommitResult is one commit produced by a Decompose run (mirrors generate.Result's commit-relevant
// fields). Ordered oldest-first in DecomposeResult.Commits. On the happy path (arbiter did not run) the
// SHA/Subject/Message/Files are the published commit's. When the arbiter amended/rebuilt (Amended>0) the
// last `Amended` entries' SHAs are PRE-amend (superseded by the rebuild) — see §G-RESULT.
type CommitResult struct {
	SHA     string           // the published commit SHA (newSHA[i])
	Subject string           // ExtractSubject(Message) — for the "[<short-sha>] <subject>" line (FR42)
	Message string           // the full commit message committed verbatim
	Files   []git.FileChange // DiffTree(newSHA, isRoot) — the "what landed" file-list (FR42)
}

// DecomposeResult is the outcome of Decompose. Commits is ordered oldest-first. Amended is the number of
// commits the arbiter rewrote (tip amend=1, mid-chain at index i = N-i); 0 if the arbiter did not run
// (clean tree) or made a new commit (null target). Designed to mirror the future pkg/stagecoach public
// DecomposeResult (P4.M2.T1.S1).
type DecomposeResult struct {
	Commits []CommitResult // the ordered commits created this run (oldest first)
	Amended int            // arbiter rewrite count (0/1/N-i); see §G-RESULT for the post-arbiter gap
}

// ErrDecomposeFailed is the sentinel for orchestrator-level INFRA failures not owned by a sibling
// sentinel (e.g. baseTree derivation, WorkingTreeDiff for the arbiter, AddAll before the escape-hatch).
// callPlanner/stager/message/arbiter/resolveArbiter failures carry their OWN sentinels (propagated).
// *generate.RescueError (message gen) and *generate.CASError (CAS) propagate DIRECTLY (not wrapped).
var ErrDecomposeFailed = errors.New("decompose: orchestrator failed")
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/decompose/roles.go — add the unexported stager test-seam field to Deps
  - ADD to the Deps struct (after the Verbose field):
      // stager is an OPTIONAL test seam. When non-nil, the orchestrator (decompose.go) calls it instead
      // of the package-level stageConcept (the real tooled-agent invocation). nil in production (the CLI
      // builds Deps without it). Lets orchestrator tests inject a stager that actually stages files via
      // git (the stubtest agent cannot run git), exercising the full happy-path loop end-to-end. The
      // signature matches stageConcept exactly. See decompose.invokeStager.
      stager func(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error
  - ADD the import "github.com/dustin/stagecoach/internal/prompt" if not already present (roles.go imports
    config/git/provider/ui but NOT prompt today — verify before adding; if prompt is unused elsewhere,
    add it solely for this field's type).
  - FOLLOW pattern: Deps is the injectable-collaborators struct; adding an unexported optional field is
    the same pattern generate.Deps uses (injectable Manifest). NO behavior change (zero value = nil).
  - PRESERVE: every other field + ResolveRoles + all helpers UNCHANGED.

Task 2: CREATE internal/decompose/decompose.go — Decompose + the routing + runLoop + runSingleShortcut
  - IMPLEMENT (in order): ErrDecomposeFailed; CommitResult; DecomposeResult; Decompose (entry point);
    runSingleEscape; runSingleShortcut; runLoop; invokeStager; dupCheckMessage; computeAmended;
    buildCommitResult; drainMsg.
  - FOLLOW pattern: internal/decompose/message.go (sibling sentinel + direct publishCommit/generateMessage
    calls + *RescueError/*CASError returned directly) + internal/generate/generate.go (CommitStaged's
    step-structure + Result mapping). Package doc-comment style (doc-first, every symbol commented).
  - NAMING: Decompose (exported — the eventual public-API delegate); runLoop/runSingleShortcut/
    runSingleEscape/invokeStager/dupCheckMessage/computeAmended/buildCommitResult/drainMsg (unexported).
  - PLACEMENT: internal/decompose/decompose.go.

Task 3: CREATE internal/decompose/decompose_test.go — dcm* fixtures + integration tests
  - IMPLEMENT: dcm*-prefixed fixtures (copied from arbiter_test.go, renamed); dcmDeps(repo, RoleManifests);
    TestDecompose_SingleEscape (→CommitStaged, 1 commit, errors propagated); TestDecompose_SingleShortcut
    _CleanMessage (planner msg used verbatim, 0 message-agent calls via counter stub);
    TestDecompose_SingleShortcut_DuplicateFallback (dup → generateMessage regenerates);
    TestDecompose_AutoMultiCommit_HappyPath (N concepts via stager seam → N ordered commits, HEAD chain);
    TestDecompose_Overlap (stage[i+1] ∥ message[i] — timing or in-flight assertion);
    TestDecompose_EmptyConceptSkip (FR-M8: stager stages nothing for concept i → skipped, ≤N commits);
    TestDecompose_PlannerFailure (non-rescue, no commit); TestDecompose_SafetyCap (Count>MaxCommits →
    non-rescue error, no commit); TestDecompose_ArbiterWiring (leftover → runArbiter→resolveArbiter, clean
    tree); TestDecompose_ArbiterSkippedOnCleanTree; TestDecompose_ErrorPropagation (stager err / message
    RescueError / CAS CASError propagated); TestDecompose_UnbornRepo (root commit for concept 0).
  - FOLLOW pattern: internal/decompose/arbiter_test.go (arbDeps-style Deps; stubtest.Build+Manifest; real
    temp git repo; dcmRunGit assertions) + message_test.go (NewScript for call-varying responses).
  - NAMING: dcm* helpers + TestDecompose_*.
  - COVERAGE: every routing branch + overlap + FR-M8 + planner failure + safety cap + single-shortcut
    (clean + dup) + arbiter (run + skip) + all 3 error types + unborn.
  - PLACEMENT: internal/decompose/decompose_test.go.
```

### Decompose — the entry point (PRD §13.6, findings §1/§4)

```go
// Decompose is the top-level orchestrator for the multi-commit pipeline (PRD §13.6 / §11.4 / §9.14). It
// turns an un-staged, dirty working tree into an ordered sequence of logically-coherent commits by
// composing the four-role pipeline (planner → stager → message → arbiter).
//
// PRECONDITION (FR-M1, owned by the CLI router — P4.M1.T1.S1): the caller routed here because NOTHING is
// staged (HasStagedChanges false) AND the working tree has changes. Decompose does NOT re-check this; it
// assumes correct routing. (Defensively, the single escape-hatch's AddAll→CommitStaged handles a staged
// index too, and the planner's WorkingTreeDiff handles an empty tree by returning "" — but the contract
// is that the router enforces FR-M1.)
//
// MODE ROUTING (FR-M2): Config.Single==true || Config.Commits==1 → single ESCAPE-HATCH (planner bypassed
// → AddAll → generate.CommitStaged, v1 behavior). Else → callPlanner(forcedCount=Config.Commits). If the
// planner returns Single==true → FR-M11 single-SHORTCUT (use planner's message, dup-check first). Else →
// runLoop (1-deep overlap, N concepts).
//
// Error contract: planner failure + safety cap are NON-RESCUE (nothing snapshotted — §13.6.6); returned
// directly (NOT *RescueError). *generate.RescueError (message gen) and *generate.CASError (CAS) propagate
// DIRECTLY (errors.As-able). Other infra wraps ErrDecomposeFailed. SIGNAL-FREE for loop/shortcut in S1
// (signal arming is S2); the escape-hatch gets signal via CommitStaged internally.
//
// Decompose REQUIRES deps.Roles to be populated (by ResolveRoles, called by the CLI/P4 before Decompose).
// The optional deps.stager seam (nil in production) lets tests inject a staging-capable stager.
func Decompose(ctx context.Context, deps Deps) (DecomposeResult, error) {
	// (1) Mode routing: single ESCAPE-HATCH (planner bypassed) → v1 path.
	if deps.Config.Single || deps.Config.Commits == 1 {
		return runSingleEscape(ctx, deps)
	}

	// (2) Derive isUnborn + preRunHEAD + baseTree ONCE (callPlanner + the loop both need them).
	preRunHEAD, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil {
		return DecomposeResult{}, fmt.Errorf("%w: rev-parse head: %w", ErrDecomposeFailed, err)
	}
	baseTree := git.EmptyTreeSHA
	if !isUnborn {
		baseTree, err = deps.Git.RevParseTree(ctx, "HEAD")
		if err != nil {
			return DecomposeResult{}, fmt.Errorf("%w: rev-parse head^{tree}: %w", ErrDecomposeFailed, err)
		}
	}

	// (3) Planner (forcedCount = Config.Commits: 0=auto, ≥2=forced; ==1 caught above). NON-RESCUE on error.
	//     callPlanner ALREADY enforces the FR-M4 safety cap in auto mode (findings §1).
	out, err := callPlanner(ctx, deps, deps.Config.Commits, isUnborn)
	if err != nil {
		return DecomposeResult{}, err // ErrPlannerFailed wrap OR safety-cap error — both non-rescue (§G-PLANNER-NONRESCUE)
	}

	// (4) FR-M11 single-SHORTCUT: planner judged N=1 + supplied a message.
	if out.Single {
		return runSingleShortcut(ctx, deps, out.Message, preRunHEAD, isUnborn, baseTree)
	}

	// (5) Safety cap is enforced inside callPlanner (auto mode). (Forced mode: user asserted N — no cap.)

	// (6) The loop (1-deep overlap, FR-M8 empty-skip, serialized CAS).
	commits, chainData, err := runLoop(ctx, deps, out.Commits, baseTree, preRunHEAD, isUnborn)
	if err != nil {
		return DecomposeResult{}, err // *RescueError / *CASError / wrapped — propagated
	}

	// (7)+(8) Arbiter gate: StatusPorcelain != "" → runArbiter → resolveArbiter.
	amended := 0
	status, err := deps.Git.StatusPorcelain(ctx)
	if err != nil {
		return DecomposeResult{}, fmt.Errorf("%w: status: %w", ErrDecomposeFailed, err)
	}
	if status != "" && len(commits) > 0 {
		amended, err = runArbiterPhase(ctx, deps, commits, chainData)
		if err != nil {
			return DecomposeResult{}, err // resolveArbiter errors propagated (incl. *RescueError/*CASError)
		}
	}

	// (9) Return.
	return DecomposeResult{Commits: commits, Amended: amended}, nil
}
```

### runSingleEscape — the single escape-hatch (→ CommitStaged)

```go
// runSingleEscape is the v1 single-commit path (Config.Single || Commits==1): planner is BYPASSED
// entirely. AddAll → generate.CommitStaged. CommitStaged GENERATES its own message + arms signal
// internally. Its typed errors (ErrNothingToCommit / *RescueError / *CASError) propagate verbatim.
func runSingleEscape(ctx context.Context, deps Deps) (DecomposeResult, error) {
	if err := deps.Git.AddAll(ctx); err != nil {
		return DecomposeResult{}, fmt.Errorf("%w: add -A: %w", ErrDecomposeFailed, err)
	}
	res, err := generate.CommitStaged(ctx, generate.Deps{
		Git:      deps.Git,
		Manifest: deps.Roles.Message, // the bare message role == the §13.1–§13.5 agent
		Verbose:  deps.Verbose,
	}, deps.Config)
	if err != nil {
		return DecomposeResult{}, err // typed errors propagated verbatim (errors.As-able)
	}
	return DecomposeResult{
		Commits: []CommitResult{{
			SHA: res.CommitSHA, Subject: res.Subject, Message: res.Message, Files: res.Changes,
		}},
		Amended: 0,
	}, nil
}
```

### runSingleShortcut — FR-M11 (dup-check + message-agent fallback)

```go
// runSingleShortcut (FR-M11): the planner ALREADY ran and returned Single==true + a Message. Use the
// planner's message DIRECTLY (AddAll → WriteTree → publish), dup-checking it first. If it's a duplicate,
// fall back to generateMessage (the message agent) to regenerate. ZERO separate message-agent call on a
// clean subject (the shortcut's whole point — one agent round-trip). Distinct from the escape-hatch
// (which bypasses the planner and regenerates via CommitStaged). SIGNAL-FREE in S1.
func runSingleShortcut(ctx context.Context, deps Deps, plannerMsg, preRunHEAD string, isUnborn bool, baseTree string) (DecomposeResult, error) {
	if err := deps.Git.AddAll(ctx); err != nil {
		return DecomposeResult{}, fmt.Errorf("%w: add -A: %w", ErrDecomposeFailed, err)
	}
	treePrime, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return DecomposeResult{}, fmt.Errorf("%w: write-tree: %w", ErrDecomposeFailed, err)
	}

	// Dup-check the planner's message. Fallback to the message agent ONLY on a duplicate (FR-M11).
	msg := plannerMsg
	if dupCheckMessage(ctx, deps, plannerMsg, isUnborn) {
		msg, err = generateMessage(ctx, deps, baseTree, treePrime) // the message agent regenerates
		if err != nil {
			return DecomposeResult{}, err // *RescueError — propagate DIRECTLY
		}
	}

	// Publish (parentSHA = preRunHEAD; root if unborn). publishCommit returns *CASError DIRECTLY on CAS.
	newSHA, err := publishCommit(ctx, deps, treePrime, preRunHEAD, msg)
	if err != nil {
		return DecomposeResult{}, err
	}
	cr, err := buildCommitResult(ctx, deps, newSHA, msg, isUnborn)
	if err != nil {
		return DecomposeResult{}, err
	}
	return DecomposeResult{Commits: []CommitResult{cr}, Amended: 0}, nil
}

// dupCheckMessage reports whether msg's subject exactly matches one of the last 50 commit subjects
// (FR32-style). nil/vacuous on an unborn repo (no dup possible). Reuses generate.IsDuplicate +
// generate.ExtractSubject (same as generateMessage internally) for consistency.
func dupCheckMessage(ctx context.Context, deps Deps, msg string, isUnborn bool) bool {
	if isUnborn {
		return false
	}
	recent, err := deps.Git.RecentSubjects(ctx, 50)
	if err != nil {
		return false // best-effort: treat a read failure as "not a duplicate" (fall through to the planner msg)
	}
	return generate.IsDuplicate(generate.ExtractSubject(msg), recent)
}
```

### runLoop — the 1-deep-overlap per-concept loop (§13.6.3, findings §4) — CRITICAL: overlap + drain

```go
// runLoop drives the per-concept pipeline with 1-DEEP overlap (stager[i+1] ∥ message[i]) + FR-M8
// empty-skip + serialized CAS publication (PRD §13.6.3). It returns the ordered []CommitResult (oldest
// first) + the parallel []ChainEntry (for resolveArbiter).
//
// Algorithm (findings §4): for each concept i — stage[i] (msg[i-1] in flight) → freeze tree[i] → FR-M8
// skip check → drain+publish msg[i-1] (CAS) → launch msg[i]. Final: drain+publish msg[N-1].
//
// Safety: message[i] uses diff(prevTree, tree[i]) — frozen, immune to the live index (§13.6.3 inv. 2).
// Publication is strictly ordered (CAS chain). Channels are buffered(1) so goroutines never block on send.
// On ANY error, drainMsg the in-flight channel before returning (no leak). In S1 errors propagate
// structurally; S2 (FR-M12) wraps the stage/message/publish seams with isolation.
func runLoop(ctx context.Context, deps Deps, concepts []prompt.PlannerCommit, baseTree, preRunHEAD string, isUnborn bool) ([]CommitResult, []ChainEntry, error) {
	type msgOut struct {
		conceptIdx int
		treeA      string
		treeB      string
		msg        string
		err        error
	}

	var commits []CommitResult
	var chainData []ChainEntry
	prevTree := baseTree
	prevSHA := preRunHEAD // CAS expected-old + parent for the next commit (all-zeros handled by publishCommit for "")

	// launch runs generateMessage for one concept in a goroutine, returning a buffered result channel.
	launch := func(i int, treeA, treeB string) chan msgOut {
		ch := make(chan msgOut, 1) // buffered(1) — goroutine sends once + exits; never blocks
		go func() {
			m, e := generateMessage(ctx, deps, treeA, treeB)
			ch <- msgOut{conceptIdx: i, treeA: treeA, treeB: treeB, msg: m, err: e}
		}()
		return ch
	}

	// publish drains a message channel + publishes the commit in order. Returns the newSHA + updated chain.
	publish := func(ch chan msgOut) error {
		if ch == nil {
			return nil
		}
		res := <-ch
		if res.err != nil {
			return res.err // *RescueError — propagate DIRECTLY (S2: rescue-for-concept-i)
		}
		newSHA, err := publishCommit(ctx, deps, res.treeB, prevSHA, res.msg) // parentSHA = prevSHA (CAS expected-old)
		if err != nil {
			return err // *CASError DIRECTLY (S2: abort-with-recovery)
		}
		cr, err := buildCommitResult(ctx, deps, newSHA, res.msg, res.conceptIdx == 0 && isUnborn)
		if err != nil {
			return fmt.Errorf("%w: diff-tree[%d]: %w", ErrDecomposeFailed, res.conceptIdx, err)
		}
		commits = append(commits, cr)
		chainData = append(chainData, ChainEntry{SHA: newSHA, Tree: res.treeB, Message: res.msg, Parent: prevSHA})
		prevSHA = newSHA
		return nil
	}

	var inflight chan msgOut // the in-flight message goroutine (concept i-1); nil if none
	for i, concept := range concepts {
		// stage[i] (msg[i-1] runs concurrently in `inflight` — the overlap). S1: propagate; S2: retry-once.
		if err := invokeStager(ctx, deps, concept); err != nil {
			drainMsg(inflight) // avoid goroutine leak (S1: discard; the stage error wins)
			return nil, nil, err
		}
		treeI, err := freezeSnapshot(ctx, deps)
		if err != nil {
			drainMsg(inflight)
			return nil, nil, fmt.Errorf("%w: freeze snapshot[%d]: %w", ErrDecomposeFailed, i, err)
		}

		// FR-M8 empty-skip: stager staged nothing new → skip commit i (no message, no publish).
		skipped := treeI == prevTree

		// Publish the PREVIOUS concept's commit (drain msg[i-1]) — serialized, in order.
		if err := publish(inflight); err != nil {
			return nil, nil, err
		}
		inflight = nil

		// Launch msg[i] (overlaps stage[i+1] in the NEXT iteration) unless this concept was skipped.
		if !skipped {
			inflight = launch(i, prevTree, treeI)
			prevTree = treeI
		}
		// skipped: prevTree unchanged (== treeI); no message launched.
	}

	// Drain + publish the final pending message.
	if err := publish(inflight); err != nil {
		return nil, nil, err
	}
	return commits, chainData, nil
}

// drainMsg receives-and-discards a buffered(1) message channel's result to avoid a goroutine leak when
// the loop aborts with a message goroutine in flight. Non-blocking (the goroutine sends exactly once to
// the buffered channel); but the receive also lets a future S2 inspect a message error before returning
// a stage error. nil-safe.
func drainMsg(ch interface{}) { // accept chan msgOut via interface to avoid exporting msgOut; or make msgOut unexported package-level
	if ch == nil {
		return
	}
	// (Implement with the concrete unexported chan type — see Task 2. A select with default is safe but
	// a blocking receive is ALSO safe because the goroutine ALWAYS sends exactly once to a buffered(1)
	// channel before exiting, so the receive is guaranteed to complete. Prefer the blocking receive.)
	_ = ch // placeholder — real impl: `<-ch` (concrete type) or a select{case r := <-ch: _ = r}
}
```

> **NOTE on drainMsg's type**: export `msgOut` as an unexported package-level type (or keep it local to
> runLoop and make drainMsg a closure inside runLoop). The cleanest is to inline the drain at each error
> site as `<-inflight` (inflight is already the concrete `chan msgOut`), since the only drain callers are
> inside runLoop. Prefer inlining `<-inflight` (3 sites) over a helper to avoid type-juggling.

### Arbiter phase + helpers

```go
// runArbiterPhase runs the arbiter + resolution when the working tree is non-empty after the loop
// (PRD §13.6.5 / FR-M9). Returns the arbiter's rewrite COUNT for DecomposeResult.Amended (0 if the arbiter
// made a new commit; 1 for tip amend; N-i for mid-chain at index i). resolveArbiter returns ONLY an error;
// the count is computed from the target via findTargetIndex (same package) BEFORE calling resolveArbiter.
func runArbiterPhase(ctx context.Context, deps Deps, commits []CommitInfo, chainData []ChainEntry) (int, error) {
	// Build the arbiter input: []CommitInfo (already built) + the leftover diff.
	leftoverDiff, err := deps.Git.WorkingTreeDiff(ctx, git.StagedDiffOptions{
		MaxDiffBytes:     deps.Config.MaxDiffBytes,
		MaxMdLines:       deps.Config.MaxMdLines,
		BinaryExtensions: deps.Config.BinaryExtensions,
	})
	if err != nil {
		return 0, fmt.Errorf("%w: leftover diff: %w", ErrDecomposeFailed, err)
	}

	out, err := runArbiter(ctx, deps, commits, leftoverDiff)
	if err != nil {
		return 0, err // ErrArbiterFailed wrap (render error) — rare
	}

	amended := computeAmended(out.Target, chainData) // BEFORE resolveArbiter (it doesn't return the count)

	if err := resolveArbiter(ctx, deps, out.Target, commits, chainData); err != nil {
		return 0, err // *RescueError / *CASError / ErrArbiterResolutionFailed — propagated
	}
	return amended, nil
}

// computeAmended returns the arbiter's rewrite count from its target decision: nil → 0 (new commit);
// tip (last entry) → 1; earlier commit at index i → N-i; not-found → 0 (defensive null). Uses
// findTargetIndex (chain.go, same package). See §G-RESULT.
func computeAmended(target *string, chainData []ChainEntry) int {
	if target == nil {
		return 0
	}
	N := len(chainData)
	idx := findTargetIndex(*target, chainData)
	if idx < 0 {
		return 0 // defensive: not-found → treat as null/new (matches resolveArbiter's defensive null)
	}
	if idx == N-1 {
		return 1 // tip amend
	}
	return N - idx // mid-chain rebuild at index idx
}

// buildCommitResult builds a CommitResult from a published commit (DiffTree for the FR42 file-list).
// isRoot is true ONLY for concept 0 on an unborn repo.
func buildCommitResult(ctx context.Context, deps Deps, sha, msg string, isRoot bool) (CommitResult, error) {
	files, err := deps.Git.DiffTree(ctx, sha, isRoot)
	if err != nil {
		return CommitResult{}, err
	}
	return CommitResult{SHA: sha, Subject: generate.ExtractSubject(msg), Message: msg, Files: files}, nil
}

// invokeStager is the test seam: deps.stager if non-nil (tests inject a git-staging stager), else the
// real package-level stageConcept (the tooled agent). Production builds Deps without deps.stager (nil).
func invokeStager(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error {
	if deps.stager != nil {
		return deps.stager(ctx, deps, concept)
	}
	return stageConcept(ctx, deps, concept)
}
```

### Integration Points

```yaml
DECOMPOSE PACKAGE (internal/decompose/decompose.go):
  - new file: DecomposeResult, CommitResult, ErrDecomposeFailed, Decompose + runLoop + runSingleShortcut
    + runSingleEscape + runArbiterPhase + computeAmended + buildCommitResult + dupCheckMessage + invokeStager
  - consumed (no change): callPlanner, stageConcept, freezeSnapshot, generateMessage, publishCommit,
    runArbiter, CommitInfo (arbiter.go); resolveArbiter, ChainEntry, findTargetIndex (chain.go — parallel);
    Deps, RoleManifests (roles.go)

ROLES (internal/decompose/roles.go):
  - edit: add unexported `stager` field to Deps (+ doc comment; + prompt import if needed)

GENERATE (internal/generate): consumed read-only — CommitStaged, Deps, Result, ExtractSubject, IsDuplicate,
  RescueError, CASError, ErrNothingToCommit. NOT edited.

GIT (internal/git/git.go): consumed read-only — do NOT edit (parallel-owned). RevParseHEAD/RevParseTree/
  WriteTree/CommitTree/UpdateRefCAS/DiffTree/StatusPorcelain/AddAll/WorkingTreeDiff/RecentSubjects +
  EmptyTreeSHA + ErrCASFailed.

CONFIG / PROMPT: consumed read-only. No new config keys; no prompt changes.

CALLER WIRING (P4 — NOT this task): the CLI router (P4.M1.T1.S1) enforces FR-M1 (HasStagedChanges +
dirty tree) + builds Deps via ResolveRoles + calls Decompose; the public API (P4.M2.T1.S1) wraps it.
No caller wiring in this task.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after each file change — fix before proceeding.
go build ./...                       # compile (catches the new Decompose + Deps.stager wiring)
go vet ./...                         # go vet (catches shadowed vars, printf issues, unkeyed struct lits)
gofmt -l internal/ pkg/              # MUST print nothing (empty = formatted)
golangci-lint run                    # the repo's linter (Makefile `make lint`)

# Scope-specific quick check:
go build ./internal/decompose/... && go vet ./internal/decompose/...

# Expected: zero errors. If gofmt prints file names, run `gofmt -w internal/ pkg/` and re-check.
# CRITICAL: verify roles.go still compiles with the new unexported field + that the prompt import (if
# added) is actually used (unused import = compile error under golangci-lint's goimports).
```

### Level 2: Unit & Integration Tests (Component Validation)

```bash
# The new orchestrator (internal/decompose):
go test -race ./internal/decompose/... -run 'TestDecompose' -v

# Full package (regression — siblings unchanged):
go test -race ./internal/decompose/...

# The whole suite (Makefile `make test`):
go test -race ./...

# Expected: all pass. If TestDecompose_AutoMultiCommit_HappyPath fails with "0 commits", the stub stager
# staged nothing — verify the test injects deps.stager (§G-SEAM) and that invokeStager dispatches to it.
# If TestDecompose_Overlap fails on -race / goroutine-leak, verify channels are buffered(1) + drainMsg is
# called on every error path (§G-OVERLAP). If TestDecompose_EmptyConceptSkip creates an empty commit, the
# FR-M8 comparison is wrong (compare tree[i] vs prevTree, the LAST NON-SKIPPED tree — §G-EMPTY-SKIP).
```

### Level 3: Integration Testing (System Validation)

```bash
# Coverage gate — confirms this task did NOT regress the 4 gated packages (decompose is NOT gated, but
# editing roles.go is in internal/decompose which isn't gated either; run to be safe):
make coverage-gate      # enforces >=85% on internal/{git,provider,generate,config}

# Manual happy-path sanity (real repo) — optional; the integration tests cover this. To eyeball:
#   build a repo with 3 unrelated file groups, inject a stager seam that stages each group per concept,
#   run Decompose, then: git log --oneline (3 ordered commits) + git status (clean) + each commit's
#   parent is the previous one's SHA.

# Expected: coverage-gate PASS (unchanged); manual history shows N ordered commits with a clean tree.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Overlap invariant (TestDecompose_Overlap): assert that stage[i+1] runs WHILE message[i] is in flight.
# Two ways: (a) timing — a message stub with SleepMS=200 + a stager seam that records a timestamp on
# entry; assert the stager's timestamp is BEFORE the message goroutine completes (overlap occurred). Or
# (b) a synchronization primitive — the stager seam asserts an in-flight flag set by the message goroutine.
# Prefer (a) for simplicity; allow timing slack.

# Single-shortcut zero-call invariant (TestDecompose_SingleShortcut_CleanMessage): use a message-agent
# stub whose Execute INCREMENTS a counter (or t.Fatal if called). Assert the planner's message is used
# verbatim AND the message agent was NEVER invoked (counter==0). The dup-fallback test (counter==1) is
# the inverse: planner msg is a duplicate → generateMessage runs once.

# Planner-non-rescue invariant (TestDecompose_PlannerFailure + TestDecompose_SafetyCap): assert the
# returned error is NOT *generate.RescueError (errors.As(err, &*generate.RescueError{}) == false) AND
# that NO commit was created (git rev-list --count HEAD unchanged). This is §13.6.6's "exit non-rescue".

# Arbiter wiring (TestDecompose_ArbiterWiring): the stager seam leaves a file un-staged → after the loop
# StatusPorcelain != "" → runArbiter + resolveArbiter run → git status --porcelain == "" (clean). Assert
# DecomposeResult.Amended matches the arbiter stub's target (0 for null, 1 for tip, N-i for mid-chain).

# Error propagation (TestDecompose_ErrorPropagation): inject (a) a stager seam returning an error →
# Decompose returns it (errors.Is ErrStagerFailed or the seam's error); (b) a message stub that times out
# (SleepMS > Timeout) → Decompose returns *generate.RescueError (errors.As-able); (c) move HEAD externally
# before publishCommit's CAS → *generate.CASError (errors.As-able). For (c) the stager seam or a test hook
# must inject the HEAD move between tree-build and CAS — or test publishCommit's CAS path at the message
# layer (already covered by message_test); here, assert Decompose propagates it.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` compiles (Decompose + Deps.stager wired; no import cycle; msgOut typed cleanly).
- [ ] `go vet ./...` clean.
- [ ] `go test -race ./...` green (Makefile `make test`) — NO goroutine leaks / races in runLoop.
- [ ] `golangci-lint run` clean (Makefile `make lint`).
- [ ] `gofmt -l internal/ pkg/` empty.
- [ ] `make coverage-gate` PASS (the 4 gated packages unaffected; roles.go edit doesn't lower coverage).
- [ ] go.mod/go.sum UNCHANGED (generate/prompt/config/git/provider/ui already imported by siblings).

### Feature Validation

- [ ] `Decompose(Config.Single)` / `Decompose(Config.Commits==1)` → AddAll → CommitStaged → 1 commit; typed errors propagated verbatim.
- [ ] `Decompose(auto, planner Single==true, clean msg)` → planner's message used VERBATIM, ZERO message-agent calls (counter stub).
- [ ] `Decompose(auto, planner Single==true, DUP msg)` → generateMessage regenerates; regenerated message committed.
- [ ] `Decompose(auto/forced, N concepts via stager seam)` → N ordered commits; HEAD chain intact (each parent == previous SHA); concept-0 root on unborn repo.
- [ ] runLoop overlaps stage[i+1] ∥ message[i] (1-deep); channels buffered(1); drained on every error path.
- [ ] FR-M8: a concept whose tree == prevTree is skipped (commit count ≤ N; never an empty commit).
- [ ] Planner failure (unparseable) + safety cap (Count>MaxCommits) → NON-RESCUE error (NOT *RescueError); NO commit created.
- [ ] StatusPorcelain != "" after loop → runArbiter → resolveArbiter; tree ends clean. == "" → arbiter skipped.
- [ ] DecomposeResult.Commits ordered oldest-first, accurate on the happy path; .Amended = 0 (no arbiter/new) / 1 (tip) / N-i (mid-chain).
- [ ] *RescueError (message), *CASError (CAS), and stager errors propagate from Decompose (errors.As/Is-able).

### Code Quality Validation

- [ ] decompose.go follows the package's sentinel convention (ErrDecomposeFailed) + direct generateMessage/publishCommit calls (message.go pattern) + *RescueError/*CASError returned DIRECTLY.
- [ ] Decompose is SIGNAL-FREE for loop/shortcut (no internal/signal import); escape-hatch gets signal via CommitStaged.
- [ ] refs move ONLY at publishCommit's CAS (serialized, in order); NEVER the 2-arg force update-ref.
- [ ] runLoop's overlap is 1-deep (one in-flight message goroutine at a time); every error path drains it.
- [ ] decompose_test.go uses dcm*-prefixed fixtures (no collision with arb*/chn*/stg*/msg*/planner).
- [ ] planner.go/stager.go/message.go/arbiter.go/chain.go/git.go UNCHANGED (only 3 git changes: decompose.go, decompose_test.go, roles.go).
- [ ] The stager seam (Deps.stager) is unexported + nil in production (no behavior change); documented as a test seam.

### Documentation & Deployment

- [ ] Doc comments on Decompose, runLoop, runSingleShortcut, runSingleEscape, runArbiterPhase, computeAmended, buildCommitResult, dupCheckMessage, invokeStager, CommitResult, DecomposeResult, ErrDecomposeFailed (the package's doc-first style; every symbol commented, including the §G-RESULT gap + S2 seams).
- [ ] No new environment variables or config keys.
- [ ] The S1/S2 boundary is documented in code comments (FR-M12 isolation + signal arming + rescue variant = S2).

---

## Anti-Patterns to Avoid

- ❌ Don't conflate the single SHORTCUT (FR-M11 — planner ran, use its message) with the single ESCAPE-HATCH (--single — planner bypassed, CommitStaged regenerates). They are two different paths (§G-SHORTCUT-VS-ESCAPE).
- ❌ Don't make the overlap UNBOUNDED — it is strictly 1-deep (stager[i+1] ∥ message[i]). At most ONE message goroutine in flight; publish before launching the next (§G-OVERLAP).
- ❌ Don't use an UNbuffered channel for the message goroutine — it would block the goroutine on send if the orchestrator is slow to receive, risking a leak on error. Buffered(1) (the goroutine sends exactly once then exits).
- ❌ Don't forget to drain the in-flight message channel on EVERY runLoop error path (stage err, freeze err, publish err) — a missed drain leaks a goroutine (§G-OVERLAP; -race may not catch it but leakcheck would).
- ❌ Don't compare tree[i] to tree[i-1] for FR-M8 — compare to `prevTree` (the LAST NON-SKIPPED tree), else a sequence of empty concepts mis-skips (§G-EMPTY-SKIP).
- ❌ Don't wrap callPlanner errors (planner failure / safety cap) in *generate.RescueError — they are NON-RESCUE (nothing was snapshotted; §G-PLANNER-NONRESCUE / §13.6.6).
- ❌ Don't re-implement the safety cap in Decompose — callPlanner ALREADY enforces FR-M4 in auto mode (forced mode is user-asserted; §G-SAFETY-CAP).
- ❌ Don't re-read HEAD for publishCommit's CAS expected-old — pass `prevSHA` (newSHA[i-1] / preRunHEAD / all-zeros) explicitly (§G-CAS-CHAIN). publishCommit does the §13.5 Actual re-read internally on failure.
- ❌ Don't edit git.go to add a SHA-list method (for accurate post-arbiter Commits) — git.go is parallel-owned this cycle; it WILL conflict (§G-RESULT). Accept the documented Commits/Amended limitation in S1.
- ❌ Don't edit chain.go (resolveArbiter / ChainEntry / findTargetIndex) — it's parallel-owned (P3.M3.T2.S1); consume it verbatim.
- ❌ Don't import internal/signal in decompose.go — the loop/shortcut are SIGNAL-FREE in S1 (signal arming is S2; §G-SIGNAL-FREE). The escape-hatch gets signal via CommitStaged.
- ❌ Don't implement FR-M12 per-concept isolation (stager retry-once-then-empty, message rescue-for-concept-i, CAS abort-with-recovery) — that's S2 (P3.M4.T1.S2). S1 propagates structurally; structure the seams so S2 can wrap them.
- ❌ Don't run the arbiter when StatusPorcelain == "" (the perfect run) — gate on `status != ""` (§G-ARBITER-INPUTS / FR-M9).
- ❌ Don't build the arbiter's []CommitInfo lazily — build it from the loop's published commits (SHA + ExtractSubject(msg) + DiffTree files) as you go; it's PARALLEL to chainData (§G-ARBITER-INPUTS).
- ❌ Don't pass forcedCount==1 to callPlanner — the escape-hatch catches Commits==1 first (§G-MODE-FORCEDCOUNT).
- ❌ Don't use t.Parallel() in decompose_test.go if any test mutates a package-level var — but this design uses the Deps.stager SEAM (per-Deps), so t.Parallel() is safe; the dcm* fixtures use t.TempDir() (isolated repos).
