---
name: "P3.M2.T3.S1 — Implement internal/decompose/stager.go: tooled stageConcept + freezeSnapshot (PRD §13.6.2/§13.6.3, §9.14 FR-M5/M6/M8)"
description: |

  CREATE ONE NEW FILE `internal/decompose/stager.go` (package `decompose`, the 3rd file after the
  shipped roles.go and the parallel planner.go) and ONE NEW TEST FILE `stager_test.go`. stager.go is
  the stager half of the multi-commit decomposition pipeline (PRD §13.6.2): `stageConcept` runs the
  TOOLED stager agent (the ONLY tooled role) with a concept's title+description as its task, Rending in
  RenderTooled mode, Executing once, and returning nil on success or an ErrStagerFailed-wrapped error on
  any failure (render/exec-non-zero/timeout/cancel). It is the tooled, no-retry, no-parse counterpart of
  callPlanner: NO retry loop (the orchestrator P3.M4.T1.S1 owns the FR-M8 "retry once then treat as
  empty"), NO output parsing (the stager returns free-form text; the index is the truth source), and it
  mutates the INDEX only (git add / git apply --cached) — NEVER refs (stagecoach owns all ref mutations).
  `freezeSnapshot` is the §13.6.3 invariant-1 primitive: a thin, documented `deps.Git.WriteTree` wrapper
  that freezes the current index into an immutable tree SHA, which the orchestrator calls synchronously
  after stageConcept returns and BEFORE the next stageConcept starts. Consumed by the orchestrator
  (P3.M4.T1.S1); NO caller wiring here.

  CONTRACT (P3.M2.T3.S1, verbatim from the work item):
    1. RESEARCH NOTE: The stager (§13.6.2, FR-M5) is the ONLY tooled role. It receives a concept's
       title+description as a task (prompt/stager.go from P3.M1.T1.S2), runs with tools ON via
       RenderTooled mode (P1.M1.T2.S1). It stages changes via git add and hunk-staging. It MUST NOT
       commit/amend/push — stagecoach owns all ref mutations. After stager[i] returns, the orchestrator
       FREEZES tree[i] = write-tree BEFORE stager[i+1] starts (§13.6.3 invariant 1). Stager failure:
       retry once, then treat as empty (FR-M8). The stager mutates the INDEX only (not refs).
    2. INPUT: Deps with Stager manifest (tooled_flags non-empty) from P3.M2.T1.S1, prompt/stager.go
       from P3.M1.T1.S2, Render with RenderTooled mode from P1.M1.T2.S1.
    3. LOGIC: Create internal/decompose/stager.go. Implement
       `func stageConcept(ctx, deps Deps, concept prompt.PlannerCommit) error`: build stager task prompt
       (BuildStagerTask), Render with RenderTooled mode, Execute. Non-zero exit: return error (caller
       retries once, then treats as empty). On success, the index holds the concept's changes. Implement
       `func freezeSnapshot(ctx, deps Deps) (treeSHA string, err error)`: calls deps.Git.WriteTree to
       freeze the current index into an immutable tree. This MUST be called synchronously after
       stageConcept returns and BEFORE the next stageConcept starts.
    4. OUTPUT: stageConcept stages one concept's changes; freezeSnapshot returns the frozen tree SHA.
       The orchestrator (P3.M4.T1.S1) interleaves these with message generation.
    5. DOCS: none — internal agent call + git operation.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/decompose/roles.go — SHIPPED by P3.M2.T1.S1 (RUNNING IN PARALLEL). Defines Deps
      {Git, Registry, Config, Roles RoleManifests, Verbose}, RoleManifests, RoleModels, ResolveRoles,
      computeInstalled, isMultiProvider, setRole. CONSUMED VERBATIM. Deps has NO Models field
      (stageConcept derives the stager model — see findings §2).
    - internal/decompose/planner.go — SHIPPED by P3.M2.T2.S1 (RUNNING IN PARALLEL). Defines callPlanner
      + ErrPlannerFailed. CONSUMED (the orchestrator drives stager per planner concept). Do NOT edit;
      do NOT rely on planner_test.go's test helpers existing (parallel hazard — findings §9).
    - internal/prompt/stager.go — SHIPPED by P3.M1.T1.S2. CONSUMED: BuildStagerTask(title, description).
    - internal/provider/{render,executor}.go — CONSUMED: Manifest.Render(..., mode...RenderTooled),
      provider.Execute(ctx, spec, timeout, vb), provider.RenderTooled, CmdSpec.
    - internal/git/git.go — CONSUMED: WriteTree(ctx) → (sha, err). (TreeDiff/RevParseTree/ReadTree are
      the message/arbiter roles' concern — P3.M2.T4/P3.M3 — NOT this task.)
    - internal/config/roles.go — CONSUMED: ResolveRoleModel("stager", cfg) → (provider, model).
    - internal/decompose/{message,arbiter,chain,decompose}.go — DO NOT EXIST YET. This task creates
      ONLY stager.go (+ stager_test.go).
    - cmd/, pkg/stagecoach/ — UNCHANGED (the orchestrator P3.M4.T1.S1 wires stageConcept/freezeSnapshot).

  DELIVERABLES (2 new files, 0 edits to existing files, 0 breaking changes):
    CREATE internal/decompose/stager.go — package `decompose`; ErrStagerFailed sentinel;
      stageConcept (the tooled, no-retry stager invocation), freezeSnapshot (the §13.6.3 invariant-1
      WriteTree wrapper).
    CREATE internal/decompose/stager_test.go — stubtest (tooledStubManifest helper) + real-git
      integration tests (exit-code→error mapping for stageConcept; immutability for freezeSnapshot via
      ls-tree). Fixture helpers use DISTINCT stg*-prefixed names (parallel-safe vs planner_test.go).

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass; stageConcept
  returns nil on stub-exit-0 and ErrStagerFailed on stub-exit-nonzero/timeout; stageConcept uses
  RenderTooled (empty-TooledFlags manifest → ErrStagerFailed-wrapped render error); freezeSnapshot
  returns a non-empty tree SHA and the frozen tree is IMMUTABLE (ls-tree of tree1 excludes files staged
  AFTER the freeze — §13.6.3 invariant 1); all stageConcept errors wrap ErrStagerFailed; freezeSnapshot
  propagates WriteTree errors verbatim.

---

## Goal

**Feature Goal**: Implement the stager agent invocation + snapshot freeze for multi-commit decomposition
(PRD §13.6.2 / §13.6.3 / FR-M5/M6/M8) as a self-contained module `internal/decompose/stager.go`.
`stageConcept(ctx, deps, concept)` is the TOOLED, no-retry, no-parse counterpart of the parallel
`callPlanner`: it derives the stager (provider, model) via ResolveRoleModel, builds the §17.6 stager task
from the concept's title+description, Rends the resolved stager manifest in RenderTooled mode (the stager
is the ONLY tooled role), Executes the agent exactly once, and returns nil on success or an
ErrStagerFailed-wrapped error on any failure (render error, non-zero exit, timeout, cancel). It mutates
the INDEX only (git add / git apply —cached); it NEVER commits, amends, or moves refs. The
retry-once-then-empty (FR-M8/M12) is the CALLER's job (the orchestrator P3.M4.T1.S1) — stageConcept has
NO retry loop and NO output parsing (the stager returns free-form text; the index is the truth source).
`freezeSnapshot(ctx, deps)` is the §13.6.3 invariant-1 primitive: a thin, documented
`deps.Git.WriteTree` wrapper that freezes the current index into an immutable tree SHA, which the
orchestrator calls synchronously after stageConcept returns and BEFORE the next stageConcept starts.

**Deliverable** (2 new files in the existing `decompose` package):
1. `internal/decompose/stager.go` — `ErrStagerFailed` sentinel; `stageConcept(ctx, deps, concept) error`;
   `freezeSnapshot(ctx, deps) (string, error)`.
2. `internal/decompose/stager_test.go` — stubtest-driven (tooledStubManifest helper) + real-git
   integration tests against a real temp git repo (fixture helpers with DISTINCT stg* names).

**Success Definition**:
- stageConcept success (stub exit 0): returns nil. The render used RenderTooled (proved by the
  empty-TooledFlags test erroring).
- stageConcept non-zero exit (stub Exit:1): returns non-nil err; `errors.Is(err, ErrStagerFailed) == true`.
- stageConcept timeout (stub SleepMS > cfg.Timeout): returns non-nil err; `errors.Is(err, ErrStagerFailed)
  == true` AND `errors.Is(err, context.DeadlineExceeded) == true` (the %w chain reaches it).
- stageConcept uses RenderTooled (NOT RenderBare): a Deps whose Stager manifest has EMPTY TooledFlags
  (the raw stubtest.Manifest) → stageConcept returns an ErrStagerFailed-wrapped error mentioning "tooled"
  (RenderTooled errors on empty TooledFlags; RenderBare would silently succeed).
- freezeSnapshot immutability (§13.6.3 invariant 1): stage file A → freezeSnapshot → tree1; stage file B
  → freezeSnapshot → tree2; `tree1 != tree2`; `git ls-tree --name-only tree1` lists ONLY a.txt (NOT
  b.txt — tree1 was frozen despite the later staging); `git ls-tree --name-only tree2` lists BOTH. This
  proves the freeze is the immutable snapshot the overlapped pipeline relies on.
- freezeSnapshot error propagation: on an index with unresolved merge conflicts, freezeSnapshot returns a
  non-nil err whose message mentions "merge conflicts".
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 2 new files, nothing else.

## User Persona

**Target User**: the decompose orchestrator (`internal/decompose/decompose.go`, P3.M4.T1.S1) and, by
extension, the end user running `stagecoach` on an un-staged working tree (the default action routes to
decompose per FR-M1, P4.M1.T1.S1). stager.go is internal plumbing — NOT user-facing CLI text. The user
never invokes the stager directly; the orchestrator calls stageConcept once per concept from the planner's
partition, then freezeSnapshot, then the message agent, interleaving them.

**Use Case**: once the orchestrator has the planner's `concepts[]` (via callPlanner, P3.M2.T2.S1), it
loops: for each concept[i] it calls `stageConcept(ctx, deps, concepts[i])` (which runs the tooled agent
that stages exactly that concept's changes via git add/hunk-staging), then immediately
`tree[i], err := freezeSnapshot(ctx, deps)` (freezing the accumulated index BEFORE stager[i+1] starts),
then starts the message agent on `TreeDiff(tree[i-1], tree[i])` (possibly overlapped with stager[i+1]).
A stageConcept failure is retried once by the orchestrator, then treated as empty (tree[i]==tree[i-1],
skip commit[i] — FR-M8). stageConcept/freezeSnapshot never touch HEAD.

**Pain Points Addressed**: (a) the stager is the ONLY role that mutates the repo, so it needs the tooled
manifest (tools on, git-scoped) — stageConcept is the single seam that uses RenderTooled; (b) the
overlapped pipeline (stager[i+1] ∥ message[i]) is safe ONLY because tree[i] is frozen before stager[i+1]
starts — freezeSnapshot is exactly that freeze, and its immutability is the load-bearing guarantee; (c) a
mis-behaving/timeout stager is contained: stageConcept returns an error, the orchestrator retries once
then treats as empty — one bad concept cannot poison the run, and no ref is ever moved by the stager.

## Why

- **Closes the stager half of PRD §13.6.2 / §13.6.3 / §9.14 FR-M5/M6/M8.** The stager is the second of
  the four decompose roles and the ONLY tooled one: it runs with tools on, scoped to git staging, and
  mutates the index per concept. freezeSnapshot is the §13.6.3 invariant-1 primitive that makes the
  overlapped pipeline safe. With these, the orchestrator (P3.M4.T1.S1) has its stager + freeze entry
  points; P3.M2.T4 (message) and P3.M3.* (arbiter) can assume they exist.
- **The tooled counterpart of the proven v1 generate loop (generate.CommitStaged → stageConcept).** v1
  already does "Render → Execute → handle execErr" for ONE role (message, bare, with retry+parse).
  stageConcept is the SAME execute-and-handle pattern with TWO stager deltas: (1) TOOLED mode
  (RenderTooled, not RenderBare); (2) NO retry/parse (exit code is the signal; the caller owns retry).
  No new concept — the snapshot/CAS machinery is NOT touched by stageConcept/freezeSnapshot (refs move
  only at P3.M2.T4's UpdateRefCAS).
- **Unblocks the decompose pipeline (P3.M2–P3.M4).** Every downstream step (message per concept via
  tree-to-tree diff, arbiter) consumes the frozen tree[i] that freezeSnapshot produces, and the staged
  index that stageConcept leaves. The orchestrator cannot run until these exist. This is the third
  foundation file of the `internal/decompose/` package (roles.go, planner.go, stager.go).
- **Lowest-risk, maximal-reuse, backward-compatible.** Two NEW files in an EXISTING package (decompose);
  ZERO edits to any shipped file (roles.go, planner.go, prompt/*, provider/*, git/*, config/* all
  CONSUMED). go.mod/go.sum untouched (stdlib + config/git/prompt/provider, all already imported by
  roles.go). No import cycle (decompose already imports all four). stageConcept/freezeSnapshot are
  consumed later (P3.M4.T1.S1); no caller wiring here → zero merge friction.

## What

One new file `internal/decompose/stager.go` in the existing `decompose` package exporting one sentinel +
two functions, and one new test file. No new dependencies. No caller wiring (that is P3.M4.T1.S1).
Specifically:

- **`ErrStagerFailed`** (exported package-level sentinel): `errors.New("decompose: stager failed")`.
  Wrap ALL genuine stager failures (render error, non-zero exit, timeout, cancel) with `%w` so
  `errors.Is(err, ErrStagerFailed)` is true. The orchestrator detects stager failures via errors.Is to
  apply the FR-M8/M12 retry-once-then-empty. Mirrors callPlanner's ErrPlannerFailed (P3.M2.T2.S1).
  Non-rescue: stageConcept mutates the index only (never refs); its failures are not
  generate.RescueError scenarios.
- **`stageConcept(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error`**: the tooled
  stager invocation. Pipeline: (1) derive the stager (provider, model) via
  `config.ResolveRoleModel("stager", deps.Config)`; (2) build the §17.6 task via
  `prompt.BuildStagerTask(concept.Title, concept.Description)`; (3) Render the stager manifest in
  TOOLED mode (system prompt = ""); (4) Execute once; (5) execErr == nil → return nil (index mutated by
  the agent); execErr != nil → return ErrStagerFailed-wrapped error. A render error is likewise wrapped.
  NO retry loop, NO output parse (the contract: "return error (caller retries once, then treats as empty)").
- **`freezeSnapshot(ctx context.Context, deps Deps) (string, error)`**: the §13.6.3 invariant-1 freeze.
  Calls `deps.Git.WriteTree(ctx)`, returns the tree SHA on success or the WriteTree error verbatim on
  failure (WriteTree's errors are already descriptive — e.g. "unresolved merge conflicts in the index").
  Read-only w.r.t. refs/index (WriteTree writes a tree object to the object store but touches neither
  the index nor HEAD — §13.2). The orchestrator calls it synchronously after stageConcept returns and
  BEFORE the next stageConcept starts.

### Success Criteria

- [ ] `internal/decompose/stager.go` is package `decompose`, has a file doc comment citing PRD
      §13.6.2/§13.6.3 + FR-M5/M6/M8, and defines `ErrStagerFailed` + `stageConcept` + `freezeSnapshot`
      EXACTLY as the contract (signatures `stageConcept(ctx, deps Deps, concept prompt.PlannerCommit) error`
      and `freezeSnapshot(ctx, deps Deps) (string, error)`).
- [ ] stageConcept uses the stager manifest from `deps.Roles.Stager` in TOOLED mode
      (`deps.Roles.Stager.Render(mdl, prov, "", task, provider.RenderTooled)`) and derives the stager
      (provider, model) via `config.ResolveRoleModel("stager", deps.Config)` (Deps has no Models field).
- [ ] stageConcept builds the task via `prompt.BuildStagerTask(concept.Title, concept.Description)` and
      Executes via `provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)`.
- [ ] stageConcept returns nil on exit-0 and an ErrStagerFailed-wrapped error on ANY failure (render
      error, non-zero exit, timeout, cancel). NO retry loop, NO output parse.
- [ ] freezeSnapshot calls `deps.Git.WriteTree(ctx)` and returns `(treeSHA, nil)` on success or
      `("", err)` propagating the WriteTree error verbatim. It is read-only w.r.t. refs/index.
- [ ] ALL stageConcept failures are wrapped with `ErrStagerFailed` (`errors.Is` true). freezeSnapshot
      propagates WriteTree errors verbatim (no sentinel — infrastructural, aborts the run).
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; only 2 git changes (stager.go, stager_test.go).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the contract + scope
boundary (findings §1/§13); the SHIPPED Deps shape (no Models field) and the model-derivation decision
(findings §2); the stageConcept pipeline (findings §3); the tooled-mode rendering + empty-system-prompt
(findings §4); the Execute error contract + the NO-retry design (caller owns retry — findings §5); the
ErrStagerFailed sentinel + non-rescue semantics (findings §6); freezeSnapshot = thin WriteTree wrapper +
immutability (findings §7); the stub tooled-mode testing challenge (findings §8); the test-fixture
name-collision hazard (findings §9 — use stg*-prefixed names); the fixture helpers + the
freeze-immutability test (findings §10); the config fields (findings §11); the no-deps/no-cycle/zero-
friction facts (findings §12). No prior decompose knowledge beyond roles.go's Deps is required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (contract + 13 sections of load-bearing facts)
- docfile: plan/002_a17bb6c8dc1d/P3M2T3S1/research/findings.md
  why: §1 the verbatim contract + scope boundary; §2 the SHIPPED Deps shape (NO Models field — stageConcept
       derives the stager model via ResolveRoleModel); §3 the stageConcept pipeline (5 steps: derive → task
       → RenderTooled → Execute → error-or-nil); §4 the tooled-mode rendering (RenderTooled, empty
       TooledFlags errors, system prompt = ""); §5 Execute's error contract + the NO-retry design (caller
       owns the FR-M8 retry); §6 ErrStagerFailed + non-rescue semantics; §7 freezeSnapshot = thin WriteTree
       wrapper + the immutability that makes overlapped staging safe; §8 the stub tooled-mode testing
       challenge (manifest needs TooledFlags; stub is a git no-op); §9 the test-fixture name-collision
       hazard (use stg*-prefixed names); §10 the fixture helpers + the freeze-immutability test; §11 config
       fields; §12 no-deps/no-cycle; §13 the one-paragraph design summary.
  critical: §2 (Deps has NO Models field — derive via ResolveRoleModel("stager", deps.Config); do NOT add a
            field/param); §3 (stageConcept is ONE Execute call — NO retry, NO parse — the caller owns retry);
            §4 (pass provider.RenderTooled explicitly; system prompt is "" — the stager task IS the payload);
            §8 (stubtest.Manifest has nil TooledFlags → use a local tooledStubManifest helper; the stub is a
            git no-op so stageConcept tests assert exit-code→error only); §9 (stager_test.go MUST use
            stg*-prefixed fixture names to avoid colliding with the parallel planner_test.go).

# MUST READ — the SHIPPED Deps/RoleManifests (P3.M2.T1.S1) — stageConcept/freezeSnapshot's input contract
- file: internal/decompose/roles.go
  section: type Deps struct { Git git.Git; Registry *provider.Registry; Config config.Config; Roles
           RoleManifests; Verbose *ui.Verbose } — the injectable collaborators. RoleManifests{Planner,
           Stager, Message, Arbiter provider.Manifest} — the stager manifest is deps.Roles.Stager (TOOLED,
           TooledFlags non-empty after the FR-D4 fallback). ResolveRoles(cfg, reg) builds Deps; the
           orchestrator (P3.M4) calls stageConcept/freezeSnapshot with it.
  why: confirms Deps has Config + Roles but NO Models/RoleModels field (findings §2). stageConcept reads
       deps.Roles.Stager (the tooled manifest) and derives the (provider, model) from deps.Config via
       ResolveRoleModel. freezeSnapshot uses deps.Git.WriteTree. Do NOT edit this file (shipped by the
       parallel task; editing = conflict).
  gotcha: RoleManifests.Stager is the merged-but-UNRESOLVED manifest from reg.Get, with TooledFlags
          GUARANTEED non-empty (the FR-D4 fallback). Render is safe to call directly (it Validate+Resolves
          internally).

# MUST READ — the stager task prompt (P3.M1.T1.S2) — CONSUMED VERBATIM
- file: internal/prompt/stager.go
  section: `func BuildStagerTask(title, description string) string` — assembles the §17.6 task: the
           instruction line + the concept's title + description + the 5-line git-instructions/guardrails
           block. The stager has NO system-prompt constant (the task IS the user payload; §17.6) and NO
           JSON contract / NO parse function (the stager returns free-form text; the index is the truth
           source; failure is exit-code-driven).
  why: stageConcept calls `prompt.BuildStagerTask(concept.Title, concept.Description)` and passes the
       result as the userPayload (4th Render arg) with system prompt "" (3rd arg). No parsing — stageConcept
       discards stdout (Execute logs it via VerboseRawOutput internally).
  pattern: `task := prompt.BuildStagerTask(concept.Title, concept.Description); spec, err := deps.Roles.
           Stager.Render(mdl, prov, "", task, provider.RenderTooled)`.
  gotcha: do NOT build a system prompt for the stager — §17.6 has none. Pass "" as the sys arg. With
          SystemPromptFlag set (e.g. claude), "" sys → no flag emitted; with it unset, the prepend
          fallback is guarded by `sys != ""` → no-op. Either way "" is clean.

# MUST READ — Render (tooled mode) + Execute (the provider seam)
- file: internal/provider/render.go
  section: `func (m Manifest) Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode)
           (*CmdSpec, error)` — mode defaults to RenderBare (variadic); stageConcept MUST pass
           provider.RenderTooled (5th arg) — the stager is the ONLY tooled role (§13.6.2). RenderTooled
           appends TooledFlags; RenderTooled on a manifest with nil/empty TooledFlags returns an error
           ("tooled mode requires non-empty tooled_flags") — ResolveRoles guarantees deps.Roles.Stager
           has non-empty TooledFlags, but wrap the render error in ErrStagerFailed defensively. Render
           calls Validate+Resolve internally. Token order + the system-prompt-prepend fallback are Render's
           concern. ARG ORDER: Render takes (model, provider, sys, payload, mode) — pass (mdl, prov, "",
           task, RenderTooled).
- file: internal/provider/executor.go
  section: `func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout,
           stderr string, err error)` — runs the agent; returns (stdout, stderr, err). err is
           context.DeadlineExceeded on timeout, context.Canceled on parent cancel, wrapped *exec.ExitError
           on non-zero exit, wrapped LookPath/start error on start failure. Execute internally calls
           vb.VerboseCommand + vb.VerboseRawOutput (so the stager's free-form output is logged in verbose
           mode WITHOUT stageConcept capturing/returning stdout).
  why: stageConcept's ONLY provider calls. Pass `*spec` (deref the Render pointer) and deps.Config.Timeout.
       Handle execErr uniformly: ANY execErr → return ErrStagerFailed-wrapped error (stageConcept does NOT
       distinguish timeout vs non-zero-exit vs cancel — all are "stager failed"; the caller retries).
       Wrapping with `%w` preserves the chain so the orchestrator CAN still errors.Is(err,
       context.Canceled) if it wants to abort-on-signal.

# MUST READ — WriteTree (the freeze primitive) + the Git interface
- file: internal/git/git.go
  section: `WriteTree(ctx) (sha string, err error)` — `git write-tree`: serializes the CURRENT index into a
           tree object, returns its SHA. READ-ONLY w.r.t. refs/index (writes a tree object to the object
           store; does NOT modify the index or HEAD — §13.2). Fails (exit 128) on unresolved merge
           conflicts → returns "unresolved merge conflicts in the index — resolve them first". freezeSnapshot
           wraps this.
  why: freezeSnapshot IS WriteTree (thin wrapper). The returned SHA is an IMMUTABLE record of "what was
       staged at time T" — the §13.6.3 invariant-1 guarantee. Whatever the NEXT stageConcept does to the
       live index afterward CANNOT reach the frozen tree.
  gotcha: do NOT add any index mutation to freezeSnapshot — it is the pure freeze. Do NOT pass options
          (WriteTree takes none). The orchestrator sequences it (after stageConcept, before the next); the
          function itself is stateless.

# MUST READ — ResolveRoleModel (the stager model derivation) + Config fields
- file: internal/config/roles.go
  section: `func ResolveRoleModel(role string, cfg Config) (provider, model string)` — reads cfg.Roles[role]
           then falls back to cfg.Provider/cfg.Model for empty fields; returns the resolved pair.
           stageConcept calls ResolveRoleModel("stager", deps.Config) to derive the stager (provider, model).
  why: Deps has no Models field (findings §2); this is the consistent, self-contained way to get the
       per-role stager model. It is the SAME function ResolveRoles uses (reads the same cfg) → honors
       per-role overrides (FR-R1/D3) AND the FR-D4 stager-fallback model. FR-R5b guards the dangerous
       bare-model-no-provider-on-pi case at ResolveRoles time (BEFORE stageConcept runs), so the derivation
       is correct for every reachable case.
- file: internal/config/config.go
  section: type Config — Timeout (120s default). config.Defaults() populates it.
  why: stageConcept reads deps.Config.Timeout (passed to Execute). freezeSnapshot reads nothing from Config.
       All consumed read-only.

# MUST READ — the test infrastructure (stubtest) + the test-repo fixture pattern
- file: internal/stubtest/stubtest.go
  section: `Build(t)` (compiles cmd/stubagent ONCE, cached); `Manifest(bin, Options{Out, Exit, SleepMS,
           Stderr, Script, Counter, Output, StripCodeFence})` (single-response manifest). The stub agent
           (cmd/stubagent) reads stdin → /dev/null, emits Options.Out on stdout, exits Options.Exit. It
           IGNORES its argv entirely.
  why: stager_test.go builds Deps{Git: git.New(repo), Config: config.Defaults(), Roles: RoleManifests{
       Stager: tooledStubManifest(...)}, Verbose: nil} (NO ResolveRoles). The stub simulates the agent's
       exit code (0 → nil; non-zero → ErrStagerFailed; SleepMS > Timeout → DeadlineExceeded).
  gotcha: stubtest.Manifest sets TooledFlags=nil → RenderTooled ERRORS on it. stager_test.go MUST use a
          LOCAL tooledStubManifest helper (wrap stubtest.Manifest + set m.TooledFlags = []string{"..."}).
          The stub is a GIT NO-OP — it cannot run git add — so stageConcept success tests assert ONLY the
          exit-code → error mapping (NOT "the index holds the changes"). The staging/immutability behavior
          is tested via freezeSnapshot + real git (findings §8/§10).

# MUST READ — the test-repo fixture helpers (copy into stager_test.go with stg* prefix — findings §9)
- file: internal/generate/generate_test.go
  section: the fixture helpers (initRepo, writeFile, stageFile, commitRaw, headSHA, runGit, gitOut) +
           TestCommitStaged_Success (the canonical stubtest+real-repo integration test pattern).
  why: stager_test.go needs a real git repo to test freezeSnapshot's immutability (stage files via git add,
       freeze, stage more, assert via ls-tree). Copy the fixture helpers VERBATIM but RENAME with a `stg`
       prefix (stgInitRepo, stgWriteFile, stgStageFile, stgCommitRaw, stgRunGit, stgGitOut) to avoid
       colliding with the parallel planner_test.go's helpers (which own the un-prefixed names in package
       decompose — a duplicate declaration is a compile error).
  pattern: repo := t.TempDir(); stgInitRepo(t, repo); stgCommitRaw(t, repo, "initial"); stgWriteFile(t,
           repo, "a.txt", "a\n"); stgStageFile(t, repo, "a.txt"); tree1, _ := freezeSnapshot(ctx, deps);
           stgWriteFile(t, repo, "b.txt", "b\n"); stgStageFile(t, repo, "b.txt"); tree2, _ :=
           freezeSnapshot(ctx, deps); assert tree1 != tree2; assert stgGitOut(...ls-tree tree1...) has
           a.txt NOT b.txt; stgGitOut(...ls-tree tree2...) has both.

# MUST READ — the planner PRP (P3.M2.T2.S1) — the parallel sibling + the pattern to mirror
- docfile: plan/002_a17bb6c8dc1d/P3M2T2S1/PRP.md
  section: callPlanner's design (the bare counterpart of stageConcept) — the model derivation via
           ResolveRoleModel, the Render→Execute→handle pattern, the ErrPlannerFailed sentinel.
  why: stageConcept mirrors callPlanner's structure (derive model → build prompt → Render → Execute →
       error handling) with TWO deltas: TOOLED mode (RenderTooled, not RenderBare) and NO retry/parse (the
       stager has no JSON contract; the caller owns retry). The ErrStagerFailed sentinel mirrors
       ErrPlannerFailed. The model-derivation decision (ResolveRoleModel, because Deps has no Models field)
       is IDENTICAL — confirmed by reading the planner PRP's findings §2/§3.

# MUST READ — the design reference (the stager role + the invariant-1 freeze + failure handling)
- docfile: plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md
  section: "The Four Agent Roles" (stager = tooled, runs git in the repo, mutates index, exits 0 — output
           is free-form); "Pipeline Flow" (snapshot[i] = write-tree FROZEN before stager[i+1] — invariant 1);
           "Three Safety Invariants" (tree[i] frozen before stager[i+1]; WriteTree is pure/ref-index-
           read-only); "Failure Handling" (stager exits non-zero → retry once → treat as empty).
  why: confirms the stager is tooled, the freeze semantics, the retry-once-then-empty contract, and the
       index-only mutation rule.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §13.6.2 (stager role: tooled, git-scoped, mutates index, no commit/amend/push)
- url: PRD.md §13.6.3 (invariant 1: tree[i] frozen before stager[i+1]; the concept diff is tree-to-tree)
- url: PRD.md §9.14 FR-M5 (stager agent tooled: git add + hunk-staging; must not commit/move refs/push)
       / FR-M6 (per-concept snapshot: freeze tree[i]=write-tree BEFORE stager[i+1]) / FR-M8 (empty-concept
       skip: tree[i]==tree[i-1] → skip; the orchestrator compares, not stager.go) / FR-M12 (stager non-zero
       → retry once → treat as empty)
- url: PRD.md §17.6 (stager task prompt — committed verbatim in prompt/stager.go)
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  roles.go            # SHIPPED (P3.M2.T1.S1) — READ (CONSUMED): Deps, RoleManifests, RoleModels,
                      #   ResolveRoles. Deps has NO Models field (stageConcept derives the model).
  planner.go          # SHIPPED (P3.M2.T2.S1, parallel) — READ (CONSUMED): callPlanner, ErrPlannerFailed.
  stager.go           # DOES NOT EXIST YET — THIS TASK CREATES IT.
internal/prompt/
  stager.go           # SHIPPED (P3.M1.T1.S2) — READ (CONSUMED): BuildStagerTask(title, description).
                      #   NO system-prompt constant; NO JSON contract; NO parse.
  planner.go          # SHIPPED (P3.M1.T1.S1) — READ (CONSUMED): PlannerCommit{Title, Description} (the
                      #   input type to stageConcept), PlannerOutput.
internal/generate/
  generate.go         # READ (CLOSEST PATTERN): CommitStaged's Render→Execute→handle-execErr pattern
                      #   (the v1 loop stageConcept generalizes — minus the retry/parse/snapshot).
internal/provider/
  render.go           # READ (CONSUMED): Manifest.Render(model,provider,sys,payload,mode...),
                      #   RenderTooled (errors on empty TooledFlags), RenderBare (the default).
  executor.go         # READ (CONSUMED): provider.Execute(ctx, spec, timeout, vb) → (stdout, stderr, err).
internal/git/
  git.go              # READ (CONSUMED): WriteTree(ctx) → (sha, err) — the freeze primitive. (TreeDiff/
                      #   RevParseTree/ReadTree are the message/arbiter roles' concern — NOT this task.)
internal/config/
  config.go           # READ (CONSUMED): Config (Timeout), config.Defaults().
  roles.go            # READ (CONSUMED): ResolveRoleModel("stager", cfg) → (provider, model).
internal/stubtest/
  stubtest.go         # READ (test infra): Build, Manifest (TooledFlags=nil → needs the local tooled helper).
internal/generate/
  generate_test.go    # READ (test pattern): fixture helpers (copy + stg*-rename) + TestCommitStaged_Success.
go.mod / go.sum       # UNCHANGED (go 1.22; stdlib + config/git/prompt/provider — all already in roles.go).
.golangci.yml         # READ: errcheck/gosimple/govet/ineffassign/staticcheck/unused.
```

### Desired Codebase tree with files to be added

```bash
internal/decompose/stager.go          # NEW — package `decompose` (3rd file); the tooled stager + freeze:
                                      #   var ErrStagerFailed = errors.New("decompose: stager failed")
                                      #   func stageConcept(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error
                                      #   func freezeSnapshot(ctx context.Context, deps Deps) (string, error)
internal/decompose/stager_test.go     # NEW — stubtest (tooledStubManifest helper) + real-git integration
                                      #   tests (fixture helpers with stg*-prefixed names). Cases: stageConcept
                                      #   success/non-zero/timeout/render-tooled-error; freezeSnapshot
                                      #   immutability (ls-tree) + merge-conflict error.
# go.mod/go.sum UNCHANGED. roles.go + planner.go + prompt/* + provider/* + git/* + config/* + cmd/* +
# pkg/stagecoach all UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (Deps has NO Models field — findings §2): the SHIPPED internal/decompose/roles.go defines
//   Deps {Git, Registry, Config, Roles RoleManifests, Verbose} — there is NO Models/RoleModels field.
//   stageConcept takes ONLY Deps, so it CANNOT read a pre-resolved stager (provider, model). THE RULE:
//   derive it via `prov, mdl := config.ResolveRoleModel("stager", deps.Config)` — the SAME function
//   ResolveRoles uses. Do NOT add a Models field to Deps (owned by the shipped parallel task — editing
//   roles.go is a conflict). Do NOT add a model param to stageConcept (the contract signature is fixed).
//   The derivation is correct for every reachable case (ResolveRoles guarantees deps.Roles.Stager has
//   non-empty TooledFlags via the FR-D4 fallback AND switches the model; FR-R5b guards the dangerous
//   bare-model-no-provider-on-pi case BEFORE stageConcept runs).

// CRITICAL (the stager is TOOLED — findings §4): Render the stager manifest with provider.RenderTooled
//   (the stager is the ONLY tooled role per §13.6.2). deps.Roles.Stager.Render(mdl, prov, "", task,
//   provider.RenderTooled). Render's mode param is VARIADIC and defaults to RenderBare — OMITTING it would
//   silently render BARE (tools off) — WRONG for a stager. Pass RenderTooled explicitly (5th positional arg).
//   RenderTooled on a manifest with nil/empty TooledFlags returns an error — wrap it in ErrStagerFailed
//   defensively (ResolveRoles guarantees non-empty TooledFlags in prod, but a misconfigured test Deps hits it).

// CRITICAL (stageConcept has NO retry / NO parse — findings §3/§5): stageConcept is ONE Execute call. The
//   contract is explicit: "return error (caller retries once, then treats as empty)." Do NOT add a retry
//   loop (the orchestrator P3.M4.T1.S1 owns the FR-M8 retry-once-then-empty). Do NOT parse the stager's
//   stdout (the stager has NO JSON contract; the index is the truth source; Execute logs stdout via
//   VerboseRawOutput internally). Discard stdout; return nil on exit-0 or ErrStagerFailed on any failure.

// CRITICAL (system prompt is EMPTY — findings §4): pass "" as the 3rd Render arg. The stager has NO
//   system-prompt constant (prompt/stager.go; §17.6 — the task IS the user payload). With "" sys, Render's
//   system-prompt-flag emit (guarded by `sys != ""`) and its prepend fallback (guarded by `sys != ""`) are
//   both no-ops. Do NOT build a system prompt for the stager.

// CRITICAL (freezeSnapshot = thin WriteTree wrapper — findings §7): freezeSnapshot calls deps.Git.WriteTree
//   and returns (treeSHA, nil) or ("", err) verbatim. WriteTree is READ-ONLY w.r.t. refs/index (it writes a
//   tree object to the object store). Do NOT add any index mutation, any verbose logging (the verbose API has
//   no generic log method; WriteTree is silent in generate.go too), or any sentinel (WriteTree's errors are
//   already descriptive). The VALUE of the named function is documenting the §13.6.3 invariant-1 semantics
//   and giving the orchestrator a single seam — keep it minimal.

// CRITICAL (the stub is a GIT NO-OP — findings §8): cmd/stubagent reads stdin → /dev/null, emits canned
//   output, exits. It does NOT run git. So stageConcept success tests (stub exit 0) do NOT actually stage
//   anything — the index is unchanged. Assert ONLY the exit-code → error mapping (nil on exit 0; ErrStagerFailed
//   on non-zero/timeout). The staging/immutability behavior is tested via freezeSnapshot + real git (stage
//   files in the test, freeze, stage more, assert via ls-tree — findings §10).

// GOTCHA (stubtest.Manifest has nil TooledFlags — findings §8): RenderTooled ERRORS on empty TooledFlags.
//   stager_test.go MUST define a LOCAL tooledStubManifest helper that wraps stubtest.Manifest and sets
//   m.TooledFlags = []string{"--tooled-stub-flag"} (any non-empty slice; the stub ignores argv). Use this for
//   the happy/error/timeout stageConcept tests. For the "RenderTooled mode" assertion test, use the RAW
//   stubtest.Manifest (nil TooledFlags) and assert the render error surfaces (proving stageConcept uses
//   RenderTooled, not RenderBare — RenderBare would silently succeed on nil BareFlags).

// GOTCHA (test fixture name collision — findings §9): planner.go (P3.M2.T2.S1) is RUNNING IN PARALLEL and
//   its PRP instructs planner_test.go to COPY fixture helpers (initRepo, writeFile, stageFile, commitRaw,
//   runGit, gitOut) into package decompose. stager_test.go is ALSO package decompose — a DUPLICATE
//   declaration is a COMPILE ERROR. stager_test.go MUST use DISTINCT stg*-prefixed names (stgInitRepo,
//   stgWriteFile, stgStageFile, stgCommitRaw, stgRunGit, stgGitOut) — copied verbatim from
//   internal/generate/generate_test.go (those helpers are unimportable from package decompose). This makes
//   stager_test.go self-contained and parallel-safe.

// GOTCHA (Render's arg order vs ResolveRoleModel's return order — findings §3): ResolveRoleModel returns
//   (provider, model); Render takes (model, provider, sys, payload, mode). So pass (mdl, prov, "", task,
//   RenderTooled). `*spec` derefs the Render pointer. deps.Config.Timeout is the per-attempt timeout
//   (Execute derives the context). deps.Verbose may be nil (ui.Verbose methods are nil-safe — confirmed by
//   generate using deps.Verbose unconditionally).

// GOTCHA (wrapping context.Canceled preserves the chain — findings §5/§6): wrapping execErr (which may be
//   context.Canceled or context.DeadlineExceeded) with `fmt.Errorf("%w: %v", ErrStagerFailed, execErr)`
//   means errors.Is(err, ErrStagerFailed) AND errors.Is(err, context.Canceled) are BOTH true. The orchestrator
//   can still distinguish "user hit Ctrl-C" from "stager crashed" if it wants. So wrapping is non-lossy.

// GOTCHA (package decompose is growing — stager.go is the 3rd file): roles.go is 1st (shipped), planner.go
//   is 2nd (parallel), stager.go is 3rd. Same `package decompose`. Add a file doc comment (cite
//   §13.6.2/§13.6.3 + FR-M5/M6/M8). Do NOT redeclare helpers that roles.go owns (computeInstalled/
//   isMultiProvider/setRole) — stageConcept does not need them. No import cycle (decompose already imports
//   config/git/prompt/provider via roles.go).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/decompose/stager.go — package decompose (the 3rd file after roles.go + planner.go)

// ErrStagerFailed is the sentinel for stager-agent failures (render error, non-zero exit, timeout,
// cancellation). It is wrapped (%w) around the underlying cause so errors.Is works. The orchestrator
// (P3.M4.T1.S1) detects stager failures via errors.Is to apply the FR-M8/M12 retry-once-then-empty.
// Non-rescue: stageConcept mutates the INDEX only (the agent runs git add / git apply --cached); it NEVER
// commits, amends, or moves refs (stagecoach owns all ref mutations — §13.6.2/§19). Its failures are NOT
// generate.RescueError scenarios (no snapshot-then-CAS here; refs move only at P3.M2.T4's UpdateRefCAS).
var ErrStagerFailed = errors.New("decompose: stager failed")

// (stageConcept + freezeSnapshot — see Implementation Tasks)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/decompose/stager.go — package doc + imports + ErrStagerFailed
  - FILE DOC: cite PRD §13.6.2 (stager role: tooled, mutates index, no ref mutation) / §13.6.3 (invariant 1:
    tree[i] frozen before stager[i+1]) + §9.14 FR-M5/M6/M8. Note stager.go is the tooled stager invocation
    (stageConcept) + the snapshot freeze (freezeSnapshot). Note stageConcept is the tooled, no-retry, no-parse
    counterpart of callPlanner (it mutates the INDEX only; the orchestrator owns the FR-M8 retry). Note
    freezeSnapshot is the §13.6.3 invariant-1 WriteTree wrapper (read-only w.r.t. refs/index).
  - IMPORTS: "context"; "errors"; "fmt"; "github.com/dustin/stagecoach/internal/config";
    "github.com/dustin/stagecoach/internal/prompt"; "github.com/dustin/stagecoach/internal/provider".
    (NOTE: "git" is NOT needed by stager.go IF freezeSnapshot calls deps.Git.WriteTree directly — deps.Git is
    the git.Git interface from roles.go's Deps; WriteTree takes no options. So the import set is
    context/errors/fmt/config/prompt/provider. Verify at compile: if freezeSnapshot needs no git-package
    symbol, omit the import — do NOT add an unused import (golangci/unused + go vet reject it).)
    (All these are already imported by roles.go except possibly none new — confirm at Task 7.)
  - DEFINE `var ErrStagerFailed = errors.New("decompose: stager failed")` with the doc comment above.

Task 2: CREATE internal/decompose/stager.go — stageConcept (the tooled, no-retry stager invocation)
  - SIGNATURE: `func stageConcept(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error`.
  - BODY (see Implementation Patterns for the exact code):
      // 1. Derive the stager (provider, model) — Deps has no Models field (findings §2).
      prov, mdl := config.ResolveRoleModel("stager", deps.Config)
      // 2. Build the §17.6 stager task from the concept's title + description (findings §4).
      task := prompt.BuildStagerTask(concept.Title, concept.Description)
      // 3. Render the stager manifest in TOOLED mode (system prompt empty — the task IS the payload).
      spec, rerr := deps.Roles.Stager.Render(mdl, prov, "", task, provider.RenderTooled)
      if rerr != nil {
          return fmt.Errorf("%w: render: %v", ErrStagerFailed, rerr) // e.g. empty TooledFlags (defensive)
      }
      // 4. Execute once. NO retry (the orchestrator owns FR-M8); NO parse (no JSON contract).
      _, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
      if execErr != nil {
          return fmt.Errorf("%w: %v", ErrStagerFailed, execErr) // non-zero exit / timeout / cancel
      }
      // 5. Success — the agent mutated the index (git add / git apply --cached). The index now holds the
      //    concept's changes; the orchestrator freezes via freezeSnapshot BEFORE the next stageConcept.
      return nil
  - DOC COMMENT: cite PRD §13.6.2/§13.6.3 + FR-M5/M6/M8; note it is the tooled counterpart of callPlanner
    (RenderTooled, no retry, no parse); note it mutates the INDEX only (never refs); note the model
    derivation (findings §2 — Deps has no Models field); note the orchestrator owns the retry-once-then-empty.
  - GOTCHA: Render's (model, provider) arg order vs ResolveRoleModel's (provider, model) return — pass (mdl,
    prov). `*spec` derefs. deps.Config.Timeout → Execute. deps.Verbose may be nil (nil-safe).
  - GOTCHA: do NOT capture/return stdout — Execute logs it via VerboseRawOutput; stageConcept's contract
    returns only error.

Task 3: CREATE internal/decompose/stager.go — freezeSnapshot (the §13.6.3 invariant-1 freeze)
  - SIGNATURE: `func freezeSnapshot(ctx context.Context, deps Deps) (string, error)`.
  - BODY:
      treeSHA, err := deps.Git.WriteTree(ctx)
      if err != nil {
          return "", err // WriteTree's error is descriptive (e.g. "unresolved merge conflicts")
      }
      return treeSHA, nil
  - DOC COMMENT: cite PRD §13.6.3 invariant 1 ("tree[i] is frozen before stager[i+1] starts"). Note it is a
    thin wrapper over deps.Git.WriteTree (read-only w.r.t. refs/index — §13.2; writes a tree object to the
    object store). Note the returned SHA is an IMMUTABLE record of the index at call time — whatever the next
    stageConcept does to the live index afterward CANNOT reach it (the safety basis for stager[i+1] ∥
    message[i]). Note the orchestrator MUST call it synchronously after stageConcept returns and BEFORE the
    next stageConcept starts. Note WriteTree fails (exit 128) on unresolved merge conflicts → the error
    propagates verbatim and aborts the run.
  - GOTCHA: do NOT add verbose logging (the verbose API has no generic log method; WriteTree is silent in
    generate.go). Do NOT add a sentinel (WriteTree's errors are descriptive). Do NOT mutate the index. Keep
    it minimal — the value is the named seam + the §13.6.3 documentation.

Task 4: CREATE internal/decompose/stager_test.go — package + imports + tooledStubManifest helper
  - PACKAGE: `decompose` (internal test — stageConcept/freezeSnapshot/ErrStagerFailed visible).
  - IMPORTS: "context"; "errors"; "os"; "os/exec"; "path/filepath"; "strings"; "testing"; "time";
    "github.com/dustin/stagecoach/internal/config"; "github.com/dustin/stagecoach/internal/git";
    "github.com/dustin/stagecoach/internal/provider"; "github.com/dustin/stagecoach/internal/prompt";
    "github.com/dustin/stagecoach/internal/stubtest".
    (NOTE: only import what each test uses; drop unused imports. prompt is needed for prompt.PlannerCommit
    literals; git for git.New + the Deps.Git type; provider for provider.Manifest + RenderTooled.)
  - DEFINE `func tooledStubManifest(t *testing.T, bin string, o stubtest.Options) provider.Manifest`:
      m := stubtest.Manifest(bin, o)
      m.TooledFlags = []string{"--tooled-stub-flag"} // non-empty so RenderTooled succeeds; the stub ignores argv
      return m
    (findings §8 — stubtest.Manifest has nil TooledFlags; RenderTooled errors on it.)
  - DEFINE `func stagerDeps(t *testing.T, repo string, m provider.Manifest) Deps`:
      return Deps{Git: git.New(repo), Config: config.Defaults(), Roles: RoleManifests{Stager: m}, Verbose: nil}
    (NO ResolveRoles — the test injects the manifest directly, mirroring generate_test's deps.Manifest.)

Task 5: CREATE internal/decompose/stager_test.go — the stg*-prefixed fixture helpers (findings §9/§10)
  - COPY the fixture helpers from internal/generate/generate_test.go VERBATIM but RENAME with the `stg`
    prefix: stgInitRepo, stgWriteFile, stgStageFile, stgCommitRaw, stgRunGit, stgGitOut. (They are
    unimportable from package decompose — generate_test owns them in package generate; and renaming avoids
    colliding with the parallel planner_test.go's un-prefixed copies in package decompose.)
  - stgInitRepo(t, repo): git init + set user.name/user.email (so commit-tree works). Use the minGitEnv()
    pattern from generate_test.go if present; otherwise set GIT_AUTHOR_*/GIT_COMMITTER_* via -c flags or env.
  - stgWriteFile(t, repo, name, body): os.WriteFile(filepath.Join(repo, name), ...). UNstaged by default.
  - stgStageFile(t, repo, name): git -C repo add name (the stager's job, simulated by the test).
  - stgCommitRaw(t, repo, msg): git -C repo add -A && git -C repo commit -m msg (a parent commit).
  - stgRunGit(t, repo, args...) (string): exec git -C repo with the test env; return combined output.
  - stgGitOut(t, repo, args...) (string): same, trimmed — for ls-tree/status assertions.
  - GOTCHA: read generate_test.go's actual helper bodies before copying — match their env handling exactly
    (user.name/user.email + GIT_* env) so commit-tree/commit work in CI without a global git identity.

Task 6: CREATE internal/decompose/stager_test.go — the stageConcept test cases
  - TestStageConcept_Success: repo (any state); bin := stubtest.Build(t); m := tooledStubManifest(t, bin,
    stubtest.Options{Out: "staged a.txt"}); deps := stagerDeps(t, repo, m); concept := prompt.PlannerCommit{
    Title: "Add a", Description: "a.txt"}; err := stageConcept(ctx, deps, concept). Assert: err == nil.
    (The stub exits 0 → stageConcept returns nil. The stub is a git no-op, so do NOT assert index state.)
  - TestStageConcept_NonZeroExit: m := tooledStubManifest(t, bin, stubtest.Options{Exit: 1}); err :=
    stageConcept(ctx, deps, concept). Assert: err != nil; errors.Is(err, ErrStagerFailed) == true.
  - TestStageConcept_Timeout: cfg := config.Defaults(); cfg.Timeout = 100*time.Millisecond; m :=
    tooledStubManifest(t, bin, stubtest.Options{SleepMS: 2000}); deps := stagerDeps(...) BUT with cfg
    (build Deps manually: Deps{Git: git.New(repo), Config: cfg, Roles: RoleManifests{Stager: m}, Verbose:
    nil}); err := stageConcept(ctx, deps, concept). Assert: err != nil; errors.Is(err, ErrStagerFailed) ==
    true; errors.Is(err, context.DeadlineExceeded) == true (the %w chain reaches it).
  - TestStageConcept_RenderTooledMode (proves TOOLED, not BARE): m := stubtest.Manifest(bin,
    stubtest.Options{Out: "x"}) (RAW — nil TooledFlags); deps := stagerDeps(t, repo, m); err :=
    stageConcept(ctx, deps, concept). Assert: err != nil; errors.Is(err, ErrStagerFailed) == true;
    strings.Contains(err.Error(), "tooled") == true. (RenderTooled errors on empty TooledFlags; RenderBare
    would silently succeed — so the error PROVES the tooled path. The happy path (tooledStubManifest +
    exit 0 → nil) is the complement, covered by TestStageConcept_Success.)
  - TestStageConcept_BuildStagerTaskCalled (light): optional — assert stageConcept passes concept.Title +
    concept.Description to BuildStagerTask. Since the stub ignores stdin, the cleanest check is a code-level
    rg (Level 4) OR skip (BuildStagerTask is unit-tested in prompt/stager_test.go). Prefer the rg check in
    Level 4 over a convoluted stdin-capture test.

Task 7: CREATE internal/decompose/stager_test.go — the freezeSnapshot test cases (findings §10)
  - TestFreezeSnapshot_Success_Immutable (§13.6.3 invariant 1 — the KEY test):
      repo := t.TempDir(); stgInitRepo(t, repo); stgCommitRaw(t, repo, "initial")
      stgWriteFile(t, repo, "a.txt", "a\n"); stgStageFile(t, repo, "a.txt")
      tree1, err := freezeSnapshot(ctx, stagerDeps(t, repo, provider.Manifest{}))  // manifest unused by freeze
      Assert: err == nil; tree1 != ""
      stgWriteFile(t, repo, "b.txt", "b\n"); stgStageFile(t, repo, "b.txt")
      tree2, err := freezeSnapshot(ctx, stagerDeps(t, repo, provider.Manifest{}))
      Assert: err == nil; tree2 != ""; tree1 != tree2 (B was added)
      // IM MUTABILITY: tree1 was frozen BEFORE staging b.txt → ls-tree tree1 lists ONLY a.txt
      ls1 := stgGitOut(t, repo, "ls-tree", "--name-only", tree1)
      Assert: strings.Contains(ls1, "a.txt"); !strings.Contains(ls1, "b.txt")
      ls2 := stgGitOut(t, repo, "ls-tree", "--name-only", tree2)
      Assert: strings.Contains(ls2, "a.txt") && strings.Contains(ls2, "b.txt")
    (deps.Roles.Stager is unused by freezeSnapshot — pass a zero provider.Manifest{}; only deps.Git matters.)
  - TestFreezeSnapshot_MergeConflict: repo with an unresolved merge conflict (adapt writetree_test.go's
    makeMergeConflict pattern, or replicate inline: commit base, branch side, conflict, attempt merge →
    unmerged index). freezeSnapshot → err != nil; strings.Contains(err.Error(), "merge conflict").
    (Copy the makeMergeConflict sequence from internal/git/writetree_test.go into a stg* helper if needed;
    it's in package git — unimportable — so replicate the sequence inline or as a stgMakeMergeConflict helper.)
  - TestFreezeSnapshot_EmptyIndex: stgInitRepo (no commits, empty index) → freezeSnapshot → treeSHA ==
    git.EmptyTreeSHA ("4b825dc642cb6eb9a060e54bf8d69288fbee4904"), err == nil. (WriteTree on an empty index
    returns the well-known empty tree — the unborn-repo base case, §13.6.3. Optional but valuable.)

Task 8: VERIFY — build, vet, lint, format, full test suite
  - RUN: `go build ./...` (the new file compiles against the shipped roles.go + the parallel planner.go);
    `go vet ./...`; `golangci-lint run`; `gofmt -l internal/ pkg/` (must be empty — run `gofmt -w` on the
    new files); `go test ./...` (the new tests pass AND no existing test regressed — stageConcept/freezeSnapshot
    add no exported symbol that existing code imports, and edit no existing file).
  - GOTCHA: if stager.go imports "git" but uses no git-package symbol (freezeSnapshot calls deps.Git.WriteTree
    via the interface — no git.* reference), `go vet`/unused will reject the import. Omit "git" from
    stager.go's imports unless a git-package symbol is referenced (it likely is NOT — deps.Git is typed via
    roles.go's Deps). Confirm at Task 8; remove the unused import if present.
  - GOTCHA: errcheck is enabled — check every error return (Render, Execute's execErr, WriteTree). unused/
    staticcheck: no unused helpers (stageConcept/freezeSnapshot must be called by the tests). Confirm
    `git status --short` shows exactly 2 changes (stager.go, stager_test.go).
```

### Implementation Patterns & Key Details

```go
// PATTERN — stageConcept (the tooled, no-retry, no-parse counterpart of callPlanner):
func stageConcept(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error {
	// 1. Derive the stager (provider, model) — Deps has no Models field (findings §2).
	prov, mdl := config.ResolveRoleModel("stager", deps.Config)

	// 2. Build the §17.6 stager task from the concept's title + description (findings §4).
	task := prompt.BuildStagerTask(concept.Title, concept.Description)

	// 3. Render the stager manifest in TOOLED mode (system prompt empty — the task IS the payload).
	spec, rerr := deps.Roles.Stager.Render(mdl, prov, "", task, provider.RenderTooled)
	if rerr != nil {
		return fmt.Errorf("%w: render: %v", ErrStagerFailed, rerr) // e.g. empty TooledFlags (defensive)
	}

	// 4. Execute once. NO retry (the orchestrator owns FR-M8); NO parse (no JSON contract).
	if _, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose); execErr != nil {
		return fmt.Errorf("%w: %v", ErrStagerFailed, execErr) // non-zero exit / timeout / cancel
	}

	// 5. Success — the agent mutated the index (git add / git apply --cached). The orchestrator freezes
	//    via freezeSnapshot BEFORE the next stageConcept (§13.6.3 invariant 1).
	return nil
}

// PATTERN — freezeSnapshot (the §13.6.3 invariant-1 WriteTree wrapper):
func freezeSnapshot(ctx context.Context, deps Deps) (string, error) {
	treeSHA, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return "", err // WriteTree's error is descriptive (e.g. "unresolved merge conflicts")
	}
	return treeSHA, nil // IMMUTABLE record of the index at call time — safe vs. concurrent staging
}

// CRITICAL: stageConcept is ONE Execute call — NO retry loop, NO output parse. The orchestrator (P3.M4.T1.S1)
//   owns the FR-M8 retry-once-then-empty. Do NOT add a loop.
// CRITICAL: pass provider.RenderTooled explicitly (5th Render arg) — the stager is the ONLY tooled role.
//   Omitting mode would silently render BARE (tools off). System prompt is "" (the stager has no sys prompt).
// CRITICAL: wrapping execErr with %w preserves the chain — errors.Is(err, ErrStagerFailed) AND errors.Is(err,
//   context.DeadlineExceeded)/context.Canceled are all true. The orchestrator can distinguish if needed.
// CRITICAL: freezeSnapshot is read-only w.r.t. refs/index (WriteTree writes a tree object only). Do NOT add
//   a sentinel/verbose/index-mutation — keep it minimal; the value is the named seam + §13.6.3 docs.
```

### Integration Points

```yaml
PACKAGE (EXISTING — additive file):
  - create: "internal/decompose/stager.go (package decompose; 3rd file after roles.go + planner.go)"
  - doc: "file doc comment citing PRD §13.6.2/§13.6.3 + FR-M5/M6/M8"

CONSUMER (NOT THIS TASK — P3.M4.T1.S1):
  - the decompose orchestrator calls, per concept[i] (after callPlanner returns the partition):
        err := stageConcept(ctx, deps, concepts[i])
        if err != nil {
            err = stageConcept(ctx, deps, concepts[i]) // FR-M8: retry once
            if err != nil { /* treat as empty (tree[i] == tree[i-1]); log; continue */ }
        }
        tree[i], err := freezeSnapshot(ctx, deps) // FROZEN before stager[i+1]
        if err != nil { /* abort the run */ }
        // then start message[i] on TreeDiff(tree[i-1], tree[i]); stager[i+1] may overlap.
  - stageConcept/freezeSnapshot NEVER touch HEAD (refs move only at P3.M2.T4's UpdateRefCAS).
  - NO caller wiring in this task (do NOT touch cmd/ or pkg/stagecoach/ or decompose.go).

NO DATABASE / NO CONFIG-FILE / NO ROUTE / NO ref-mutation changes. go.mod/go.sum UNCHANGED. ZERO edits
to any shipped file (roles.go, planner.go, prompt/*, provider/*, git/*, config/*, cmd/*, pkg/stagecoach all
CONSUMED read-only).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after creating stager.go — fix before proceeding.
gofmt -w internal/decompose/stager.go internal/decompose/stager_test.go
go vet ./internal/decompose/...
go build ./...

# Expected: zero errors. The most likely failure is an unused "git" import in stager.go (freezeSnapshot
# calls deps.Git.WriteTree via the interface — no git-package symbol — so omit "git" from the imports).
# If `decompose` fails to compile, confirm roles.go (Deps/RoleManifests) + planner.go are present and
# unchanged, and that stager_test.go's stg*-prefixed helpers do NOT collide with planner_test.go's
# un-prefixed helpers (a duplicate declaration is a compile error — findings §9).
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test the new file as created.
go test ./internal/decompose/... -v

# Expected: all new stager_test.go cases pass. If a stageConcept case fails, re-check findings §3/§4/§5
# (RenderTooled mode; the empty-system-prompt; the no-retry error wrapping). If freezeSnapshot's
# immutability test fails, re-check findings §7/§10 (the ls-tree assertion that tree1 excludes b.txt).
# If the merge-conflict test is flaky, replicate writetree_test.go's makeMergeConflict sequence exactly.
```

### Level 3: Integration (No-Regressions Validation)

```bash
# The new file adds no exported symbol that existing code imports, and edits no existing file — but verify.
go test ./...
go vet ./...
golangci-lint run        # .golangci.yml: errcheck/gosimple/govet/ineffassign/staticcheck/unused
gofmt -l internal/ pkg/  # MUST be empty

# Confirm scope: exactly 2 changes, go.mod/go.sum untouched, roles.go + planner.go untouched.
git status --short
git diff --stat          # expect stager.go (+), stager_test.go (+); nothing else

# Expected: the whole module builds + tests green; only 2 files changed; roles.go + planner.go (the
# parallel tasks' outputs) are byte-for-byte unchanged; no existing behavior altered.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# (stageConcept mutates the index via the agent; freezeSnapshot is read-only — no real agent run needed
# for correctness. Level 4 is a contract-surface + design-coherence check.)

# Confirm the exported symbols the orchestrator (P3.M4.T1.S1) will consume:
rg -n 'var ErrStagerFailed|func stageConcept|func freezeSnapshot' internal/decompose/stager.go

# Confirm stageConcept is TOOLED (RenderTooled) and uses BuildStagerTask + Execute (NOT ParseOutput,
# NOT a retry loop, NOT RenderBare):
rg -n 'provider\.RenderTooled|BuildStagerTask|provider\.Execute|provider\.RenderBare|ParseOutput|for attempt' internal/decompose/stager.go
# Expected: RenderTooled present; BuildStagerTask present; provider.Execute present; RenderBare ABSENT;
#           ParseOutput ABSENT (no JSON contract); "for attempt" ABSENT (no retry loop).

# Confirm freezeSnapshot is a thin WriteTree wrapper (no sentinel, no verbose, no index mutation):
rg -n 'WriteTree|ErrFreeze|VerboseRetry|ReadTree|AddAll' internal/decompose/stager.go
# Expected: WriteTree present; ErrFreeze ABSENT (no sentinel); VerboseRetry ABSENT; ReadTree/AddAll ABSENT.

# Confirm the model derivation (ResolveRoleModel) and the ErrStagerFailed wrapping:
rg -n 'ResolveRoleModel|ErrStagerFailed' internal/decompose/stager.go

# Confirm stageConcept/freezeSnapshot do NOT touch HEAD/refs (non-rescue; refs move only at P3.M2.T4):
rg -n 'CommitTree|UpdateRef|RescueError' internal/decompose/stager.go
# Expected: NO matches (stageConcept mutates the index via the agent; freezeSnapshot is read-only w.r.t.
#           refs; neither commits nor moves HEAD — that is P3.M2.T4/P3.M4's job).

# Confirm stager_test.go uses stg*-prefixed fixture helpers (no collision with planner_test.go):
rg -n 'func stg(InitRepo|WriteFile|StageFile|CommitRaw|RunGit|GitOut)' internal/decompose/stager_test.go
# Expected: the 6 stg* helpers present (parallel-safe vs planner_test.go's un-prefixed copies).

# Expected: all symbols present; the stager is tooled; no retry/parse/HEAD-touch; freezeSnapshot is a
# thin WriteTree wrapper; test fixtures are stg*-prefixed.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go build ./...` succeeds (the new file compiles against the shipped roles.go + parallel planner.go).
- [ ] `go test ./...` GREEN (new tests pass + NO existing test regressed — including the parallel planner_test.go).
- [ ] `go vet ./...` clean.
- [ ] `golangci-lint run` clean (errcheck: every error checked — Render, Execute's execErr, WriteTree;
      unused: no dead symbols, no unused "git" import).
- [ ] `gofmt -l internal/ pkg/` empty.
- [ ] go.mod / go.sum UNCHANGED.

### Feature Validation

- [ ] All success criteria from "What" met (ErrStagerFailed + stageConcept + freezeSnapshot).
- [ ] stageConcept success (stub exit 0): returns nil; RenderTooled used.
- [ ] stageConcept non-zero exit (stub Exit:1): returns ErrStagerFailed.
- [ ] stageConcept timeout (stub SleepMS > Timeout): returns ErrStagerFailed; chain reaches DeadlineExceeded.
- [ ] stageConcept uses RenderTooled (empty-TooledFlags manifest → ErrStagerFailed-wrapped render error
      mentioning "tooled"; proves the tooled path vs the silent RenderBare).
- [ ] freezeSnapshot immutability (§13.6.3 invariant 1): tree1 != tree2; ls-tree tree1 excludes files
      staged AFTER the first freeze; ls-tree tree2 includes both.
- [ ] freezeSnapshot merge-conflict error propagation.
- [ ] ALL stageConcept failures wrap ErrStagerFailed; freezeSnapshot propagates WriteTree errors verbatim.

### Code Quality Validation

- [ ] Follows existing conventions (mirrors callPlanner/generate.CommitStaged's Render→Execute pattern;
      errcheck-clean; %w wrapping).
- [ ] File placement matches the desired tree (internal/decompose/stager.go + stager_test.go).
- [ ] Anti-patterns avoided (no retry loop in stageConcept; no output parse; no RescueError reuse; no
      Deps edit; no model param; no system prompt for the stager; no sentinel/verbose/mutation in
      freezeSnapshot; no git import if unused).
- [ ] No import cycle (decompose already imports config/git/prompt/provider via roles.go).
- [ ] Doc comments cite the PRD § + the FR for every exported symbol.

### Documentation & Deployment

- [ ] File doc + per-symbol doc comments are self-documenting (cite §13.6.2/§13.6.3 + FR-M5/M6/M8).
- [ ] The model-derivation decision (ResolveRoleModel — Deps has no Models field) + the no-retry design
      (caller owns FR-M8) + the non-rescue semantics (index-only mutation) + the freeze immutability
      (§13.6.3 invariant 1) are documented in code so future maintainers understand.
- [ ] No new env vars / config keys (stageConcept/freezeSnapshot are pure invocation + git over existing
      config/manifests).

---

## Anti-Patterns to Avoid

- ❌ Don't add a retry loop to `stageConcept` — the contract is explicit ("return error (caller retries
  once, then treats as empty"). The orchestrator (P3.M4.T1.S1) owns the FR-M8 retry-once-then-empty.
  stageConcept is ONE Execute call.
- ❌ Don't parse the stager's output — the stager has NO JSON contract (free-form text; the index is the
  truth source). Do NOT call `provider.ParseOutput` or `prompt.ParsePlannerOutput`. Discard stdout; the
  exit code is the signal.
- ❌ Don't render the stager in BARE mode — the stager is the ONLY tooled role (§13.6.2). Pass
  `provider.RenderTooled` explicitly (5th Render arg). Omitting mode silently renders BARE (tools off).
- ❌ Don't build a system prompt for the stager — §17.6 has none. Pass "" as the 3rd Render arg.
- ❌ Don't add a `Models`/`RoleModels` field to `Deps` or a model/provider param to `stageConcept` —
  owned by the shipped `roles.go`; the contract signature is fixed. Derive via `ResolveRoleModel`.
- ❌ Don't reuse `generate.RescueError` / the snapshot-then-CAS machinery — stageConcept/freezeSnapshot
  never move HEAD (refs move only at P3.M2.T4's UpdateRefCAS). stageConcept failures are non-rescue
  (index-only mutation; the orchestrator retries-or-treats-as-empty). Wrap in `ErrStagerFailed`.
- ❌ Don't add a sentinel, verbose logging, or index mutation to `freezeSnapshot` — it is a thin
  `WriteTree` wrapper (read-only w.r.t. refs/index). WriteTree's errors are descriptive; the verbose API
  has no generic log method; keep it minimal (the value is the named seam + §13.6.3 docs).
- ❌ Don't distinguish timeout vs non-zero-exit vs cancel in `stageConcept` — ALL are "stager failed";
  wrap uniformly in `ErrStagerFailed`. The `%w` chain preserves the underlying cause so the orchestrator
  CAN still `errors.Is(err, context.Canceled)` if it wants to abort-on-signal.
- ❌ Don't use the raw `stubtest.Manifest` for the stageConcept happy/error/timeout tests — it has nil
  `TooledFlags` → `RenderTooled` errors. Use the LOCAL `tooledStubManifest` helper (sets TooledFlags).
  (The raw manifest IS correct for the "RenderTooled mode" assertion test — its render error PROVES the
  tooled path.)
- ❌ Don't assume the stub stages anything — `cmd/stubagent` is a GIT NO-OP (it emits canned output, never
  runs git). stageConcept success tests assert ONLY the exit-code → error mapping. Test staging/immutability
  via `freezeSnapshot` + real git (`ls-tree` assertions).
- ❌ Don't use un-prefixed fixture helper names (`initRepo`, `writeFile`, etc.) in `stager_test.go` —
  they collide with the parallel `planner_test.go`'s copies in package `decompose` (duplicate-declaration
  compile error). Use `stg`*-prefixed names (`stgInitRepo`, …).
- ❌ Don't import `git` in `stager.go` if no `git`-package symbol is referenced — `freezeSnapshot` calls
  `deps.Git.WriteTree` via the interface (typed by `roles.go`'s `Deps`), so `stager.go` likely needs NO
  `git` import. An unused import fails `go vet`/`unused`.
- ❌ Don't wire the orchestrator (cmd/, pkg/stagecoach/, decompose.go) — this task ONLY implements
  stageConcept/freezeSnapshot; P3.M4.T1.S1 consumes them.
- ❌ Don't swallow errors (errcheck is on) — check Render, Execute's execErr, WriteTree; wrap stageConcept
  failures with `%w` + `ErrStagerFailed`; propagate freezeSnapshot's WriteTree error verbatim.

---

## Confidence Score

**9/10** — one-pass success is highly likely. stageConcept is the TOOLED, no-retry, no-parse counterpart of
the PROVEN pattern (v1 `generate.CommitStaged` + the parallel `callPlanner` both do Render→Execute→handle),
with two fully-resolved deltas: (1) TOOLED mode (`provider.RenderTooled`, errors on empty TooledFlags —
ResolveRoles guarantees non-empty in prod); (2) NO retry/parse (the contract is explicit; the orchestrator
owns FR-M8; the stager has no JSON contract). `freezeSnapshot` is a thin, documented `WriteTree` wrapper —
the §13.6.3 invariant-1 primitive — whose immutability is directly testable with real git + `ls-tree`. The
one design decision with a moving part — deriving the stager model from `deps.Config` via `ResolveRoleModel`
(because Deps has no Models field) — is IDENTICAL to callPlanner's resolved decision (confirmed by reading
the planner PRP) and is correct for every reachable case. No new dependency, no import cycle, ZERO edits to
any shipped file (additive file in an existing package), no caller wiring → blast radius is tiny. The two
residual uncertainties are both mitigated by documented guidance: (a) the stub tooled-mode test (stubtest
has nil TooledFlags → use the local tooledStubManifest helper; the stub is a git no-op → assert exit-code
mapping only); (b) the parallel test-fixture name collision (use stg*-prefixed helpers). The freeze
immutability test is the load-bearing correctness proof and is concrete (ls-tree excludes the post-freeze
file).
