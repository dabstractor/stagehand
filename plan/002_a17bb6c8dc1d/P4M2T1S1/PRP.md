---
name: "P4.M2.T1.S1 — Add DecomposeOptions / DecomposeResult / RoleModel types + Decompose() function to pkg/stagecoach/stagecoach.go (the v2 public library surface for multi-commit decomposition)"
description: |

  EDIT `pkg/stagecoach/stagecoach.go` (ADD three exported types + one exported function + two unexported
  helpers + the `internal/decompose` import) and EDIT `pkg/stagecoach/stagecoach_test.go` (ADD four
  focused tests: single-delegation E2E, single-delegation-via-Count, multi-commit-entry config error,
  stager-role-override proof + result-mapping assertions).

  CONTRACT (P4.M2.T1.S1, verbatim from the work item):
    1. RESEARCH NOTE: §14.1 specifies the public API surface. DecomposeOptions embeds Options
       (Provider/Model/DryRun/Timeout for the MESSAGE role), adds Count (int, 0=auto), Single (bool),
       MaxCommits (int, default 12), and Planner/Stager/Arbiter RoleModel. DecomposeResult has
       Commits []Result, Amended int, Provider string. Decompose() delegates to
       internal/decompose.Decompose. It's a NO-OP (delegates to GenerateCommit) when Single or Count==1.
       Caller must ensure nothing is staged (CLI gates on HasStagedChanges).
    2. INPUT: the internal decompose.Decompose from P3.M4.T1.S1, the existing Options/Result types
       from pkg/stagecoach/stagecoach.go.
    3. LOGIC: In pkg/stagecoach/stagecoach.go: add `type RoleModel struct { Provider, Model string }`,
       `type DecomposeOptions struct { Options; Count int; Single bool; MaxCommits int; Planner, Stager,
       Arbiter RoleModel }`, `type DecomposeResult struct { Commits []Result; Amended int; Provider string }`.
       Implement `func Decompose(ctx context.Context, opts DecomposeOptions) (DecomposeResult, error)`:
       if opts.Single || opts.Count==1 → delegate to GenerateCommit (wrap in DecomposeResult with one
       commit). Else: resolve config (reuse resolveConfig pattern), build decompose.Deps (reuse
       ResolveRoles), call decompose.Decompose, map internal result to public DecomposeResult.
    4. OUTPUT: pkg/stagecoach exports Decompose, DecomposeOptions, DecomposeResult, RoleModel. External
       integrators can call Decompose() for multi-commit. CLI (P4.M1.T1.S1) calls this.
    5. DOCS: [Mode A] Add doc comments to all new exported types and the Decompose function, marking
       'Stable as of v2.0' per the additive-only convention.

  ───────────────────────────────────────────────────────────────────────────────────────────────────
  CRITICAL SCOPE BOUNDARY — PARALLEL COORDINATION WITH P4.M1.T1.S1 (READ THIS FIRST):
  ───────────────────────────────────────────────────────────────────────────────────────────────────
  P4.M1.T1.S1 is being implemented IN PARALLEL and EDITS `internal/cmd/default_action.go` to call
  `internal/decompose.Decompose` DIRECTLY (it cannot depend on / call `pkg/stagecoach.Decompose` — the
  tasks.json dependency graph gives P4.M1.T1.S1 NO edge to P4.M2.T1.S1). To AVOID a parallel-edit
  conflict on `internal/cmd/`, THIS task touches ONLY `pkg/stagecoach/`. It does NOT swap the CLI call
  site. The swap (internal/decompose.Decompose → pkg/stagecoach.Decompose in default_action.go) is a
  LATER follow-up, explicitly deferred by the P4.M1.T1.S1 PRP ("P4.M2.T1.S1 will later add
  pkg/stagecoach.Decompose as a thin wrapper ... and SWAP this one CLI call site to use it").
  Encroaching into internal/cmd here would race/merge-conflict with the parallel task.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/decompose/*.go (Decompose, DecomposeResult, CommitResult, Deps, ResolveRoles,
      RoleManifests, RoleModels, DecomposeRescueError) — CONSUMED READ-ONLY. Produced by P3.M4.T1.S1/S2.
    - internal/cmd/* — OWNED BY P4.M1.T1.S1 (parallel). DO NOT TOUCH.
    - internal/config/* (Config, RoleConfig, Defaults, ResolveRoleModel) — CONSUMED.
    - internal/git/git.go (git.New, FileChange) — CONSUMED.
    - internal/provider/{registry.go,builtin.go} (NewRegistry, DecodeUserOverrides) — CONSUMED.
    - internal/ui/verbose.go (NewVerbose) — CONSUMED.
    - internal/generate/{rescue.go,generate.go} (RescueError, CASError, sentinels) — CONSUMED.
    - cmd/stagecoach/main.go — UNCHANGED (signal.Install already wired; not this task's concern).
    - PRD.md, tasks.json, .gitignore — NEVER modify (research/PRP agent; this task edits code only).

  DELIVERABLES (2 file EDITS — no new files):
    EDIT pkg/stagecoach/stagecoach.go       — +RoleModel +DecomposeOptions +DecomposeResult +Decompose
                                            +resolveDecomposeConfig +mapDecomposeResult (helpers) +
                                            import "github.com/dustin/stagecoach/internal/decompose".
    EDIT pkg/stagecoach/stagecoach_test.go  — +TestDecompose_SingleDelegates +TestDecompose_Count1Delegates
                                            +TestDecompose_MultiEntry_RoleError +TestDecompose_StagerOverride
                                            (reuse the existing setupTestRepo/setupScriptedRepo stub harness).

  SUCCESS: `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; `make coverage-gate` PASS (pkg/stagecoach is NOT gated, so no risk);
  go.mod/go.sum UNCHANGED; `Decompose`, `DecomposeOptions`, `DecomposeResult`, `RoleModel` exported with
  "Stable as of v2.0" doc comments; Single/Count==1 delegates to GenerateCommit (1 commit, full E2E
  green); the multi-commit path enters ResolveRoles (stager-role error with a bare stub); role overrides
  flow into cfg.Roles (Stager.Provider="nonexistent" → "unknown provider" error); the returned
  DecomposeResult.Provider and per-Commit Provider/Model are populated from RoleModels.Message.

---

## Goal

**Feature Goal**: Expose the v2 multi-commit decomposition pipeline through Stagecoach's intentionally-tiny
public library surface (`pkg/stagecoach`, PRD §14.1 / §13.6 / §9.14). Add three additive-only exported
types — `RoleModel`, `DecomposeOptions`, `DecomposeResult` — and the `Decompose` function, as a thin
wrapper over the already-shipped `internal/decompose.Decompose` orchestrator. The wrapper: (1) is a
NO-OP that delegates to the existing `GenerateCommit` when `opts.Single || opts.Count == 1` (the v1
single-commit path); (2) otherwise resolves the 7-layer config (reusing the `resolveConfig` pattern,
extended with the decompose-specific overrides), builds `decompose.Deps` via `decompose.ResolveRoles`
(four-role resolution), calls `decompose.Decompose`, and maps the internal `decompose.DecomposeResult`
to the public `DecomposeResult` (converting each `CommitResult` to a `Result` and populating the
`Provider` field from the resolved MESSAGE role). External integrators (a git GUI, a pre-commit hook, a
CI step) gain a single call for "dirty tree → N coherent commits" without shelling out to the CLI.

**Deliverable** (2 file EDITS — no new files):
1. `pkg/stagecoach/stagecoach.go` (EDIT) — add the `internal/decompose` import; add `type RoleModel struct`,
   `type DecomposeOptions struct` (embeds `Options` + `Count`/`Single`/`MaxCommits`/`Planner`/`Stager`/
   `Arbiter`), `type DecomposeResult struct` (`Commits []Result`/`Amended int`/`Provider string`); add
   `func Decompose`; add two unexported helpers `resolveDecomposeConfig` + `mapDecomposeResult`. All four
   new exported symbols carry "Stable as of v2.0" doc comments.
2. `pkg/stagecoach/stagecoach_test.go` (EDIT) — four new tests reusing the existing `setupTestRepo` /
   `setupScriptedRepo` stub harness.

**Success Definition**:
- **API surface**: `pkg/stagecoach` exports `Decompose`, `DecomposeOptions`, `DecomposeResult`, `RoleModel`
  (and only these new symbols — no new error sentinels; rescue/CAS reuse the already-aliased
  `RescueError`/`CASError`/`ErrRescue`/`ErrTimeout`/`ErrCASFailed`/`ErrNothingToCommit`). Each carries a
  "Stable as of v2.0" doc comment (additive-only convention).
- **Single delegation (NO-OP)**: `Decompose(ctx, DecomposeOptions{Single: true})` on a repo with a STAGED
  change delegates to `GenerateCommit` → exactly ONE new commit, returns `DecomposeResult{Commits: [one
  Result], Amended: 0, Provider: <resolved>}`, `err == nil`. `Count == 1` behaves identically.
- **Multi-commit entry**: `Decompose(ctx, DecomposeOptions{})` on a repo with a dirty, UN-STAGED working
  tree (bare stub provider, no `tooled_flags`) enters the multi-commit path: `ResolveRoles` fails the
  stager role → returns a non-nil error whose message contains "stager" and "tooled" (config error, NOT a
  rescue). This proves `resolveDecomposeConfig` + `ResolveRoles` + `Deps` construction ran and the
  delegation short-circuit did NOT fire.
- **Role override plumbing**: `Decompose(ctx, DecomposeOptions{Stager: RoleModel{Provider: "nonexistent"}})`
  on a dirty/un-staged tree returns a `ResolveRoles` error containing "stager" and "unknown provider"
  (distinct from the tooled_flags error) — proving `opts.Stager` flowed into `cfg.Roles["stager"]` and was
  consumed by `ResolveRoles`.
- **Result mapping**: the single-delegation test asserts `DecomposeResult.Provider` and each `Result`'s
  `Provider`/`Model` are populated (mapping logic verified end-to-end).
- `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; `make coverage-gate` PASS; go.mod/go.sum UNCHANGED.

## User Persona

**Target User**: an external integrator — a git GUI (lazygit customCommand), a pre-commit hook, or a CI
step (PRD §14.1: "let an integrator call the core without reimplementing it") — importing
`github.com/dustin/stagecoach/pkg/stagecoach`. Transitive: the in-module CLI (which will later swap its
internal/decompose.Decompose call to this public wrapper — a follow-up, NOT this task).

**Use Case**: the integrator has a dirty, un-staged working tree spanning several concerns and wants N
logically-coherent commits via a single library call — OR wants the v1 single-commit behavior via the same
entry point with `Single: true` / `Count: 1`.

**User Journey**: (1) integrator calls `stagecoach.Decompose(ctx, stagecoach.DecomposeOptions{Count: 3,
Provider: "pi"})`; (2) stagecoach resolves config + the four roles, runs the planner→stager→message→arbiter
pipeline; (3) on success returns `DecomposeResult{Commits: [N Result], Amended, Provider}`; (4) on a
per-concept failure returns a `*RescueError`/`*CASError` (same typed errors as `GenerateCommit`) plus the
partial commits that landed.

**Pain Points Addressed**: (a) integrators no longer need to shell out to the CLI or reimplement the
planner→stager→message→arbiter pipeline; (b) a single coherent API for both single and multi-commit paths.

## Why

- **Business value**: this is the public-library half of the v2 core (PRD §10.3 / G11 / §14.1). The
  decompose pipeline (P3) is unreachable from outside the module until this task ships the `pkg/stagecoach`
  entry point. It completes the "intentionally tiny" public surface promised by §14.1 alongside the
  existing `GenerateCommit`.
- **Integration with existing features**: consumes `internal/decompose.Decompose` + `ResolveRoles` +
  `Deps`/`DecomposeResult`/`CommitResult`/`RoleManifests`/`RoleModels` (P3.M4.T1.S1/S2), `config.Config` +
  `config.RoleConfig` + `config.ResolveRoleModel` (P1.M3), and `provider.{NewRegistry,
  DecodeUserOverrides}` (mirrors the existing `buildDeps`). It reuses the `resolveConfig` helper already
  in `pkg/stagecoach` for the 7-layer config + Options overrides, extending it with the decompose-specific
  overrides. The typed errors (`*RescueError`/`*CASError`) are the SAME aliases already exported, so exit
  mapping is uniform across both entry points.
- **Problems this solves and for whom**: lets a git GUI / pre-commit hook / CI step call decomposition
  programmatically (PRD §14.1's stated purpose). The `Single`/`Count==1` delegation keeps the two entry
  points behaviorally consistent (one call site, both paths).

## What

**User-visible behavior** (library API):
- `Decompose(ctx, DecomposeOptions{Single: true})` or `{Count: 1}` → delegates to `GenerateCommit` (one
  commit from the staged index; honors `Options.DryRun`). Returns `DecomposeResult{Commits: [Result],
  Amended: 0, Provider}`.
- `Decompose(ctx, DecomposeOptions{})` (nothing staged, dirty tree) → the full multi-commit pipeline:
  `DecomposeResult{Commits: [N Result], Amended, Provider}`. `Provider` is the resolved MESSAGE-role
  provider; each `Result` carries that provider + the resolved MESSAGE-role model.
- `Decompose` honors `Options.Provider`/`Model`/`Timeout`/`Verbose`/`VerboseOn`/`Config` for the MESSAGE
  role (inherited via embedding + `resolveConfig`). Per-role overrides via `Planner`/`Stager`/`Arbiter`
  `RoleModel` (field-merge: zero value ⇒ inherit the global default, FR-R2/FR-R3).
- PRECONDITION (FR-M1): the caller must ensure NOTHING is staged. `Decompose` does NOT re-check
  `HasStagedChanges` (it trusts the caller, exactly like the internal orchestrator). Staging + calling
  `Decompose` is undefined behavior. Document this prominently in the doc comment.
- `Options.DryRun` is honored ONLY on the single-delegation path (the multi-commit loop always commits;
  decompose is not dry-run aware). Document this.

**Technical requirements**: 3 exported types + 1 exported function + 2 unexported helpers in
`pkg/stagecoach/stagecoach.go`; the `internal/decompose` import; doc comments marked "Stable as of v2.0".

### Success Criteria

- [ ] `pkg/stagecoach` exports `Decompose`, `DecomposeOptions`, `DecomposeResult`, `RoleModel` — and ONLY
      these new symbols (no new error sentinels; reuse `RescueError`/`CASError`/`Err*` aliases).
- [ ] Each new exported symbol has a doc comment ending in / containing "Stable as of v2.0".
- [ ] `RoleModel` == `struct { Provider, Model string }`; `DecomposeOptions` embeds `Options` and adds
      `Count int; Single bool; MaxCommits int; Planner, Stager, Arbiter RoleModel` (NO `Message` field —
      message = global, FR-R2). `DecomposeResult` == `struct { Commits []Result; Amended int; Provider string }`.
- [ ] `Decompose(ctx, {Single: true})` on a staged-change repo → 1 new commit, `err==nil`,
      `DecomposeResult{Commits: 1, Provider: <resolved>}` (TestDecompose_SingleDelegates green).
- [ ] `Decompose(ctx, {Count: 1})` behaves identically (TestDecompose_Count1Delegates green).
- [ ] `Decompose(ctx, {})` on a dirty/un-staged tree + bare stub → multi-commit path entered; error
      contains "stager" + "tooled" (TestDecompose_MultiEntry_RoleError green).
- [ ] `Decompose(ctx, {Stager: RoleModel{Provider: "nonexistent"}})` on a dirty tree → error contains
      "stager" + "unknown provider" (TestDecompose_StagerOverride green — proves role override applied).
- [ ] `resolveDecomposeConfig` reuses `resolveConfig` then applies Count/Single/MaxCommits/role field-merge.
- [ ] `mapDecomposeResult` converts each `decompose.CommitResult` → `Result` and sets `DecomposeResult.Provider`
      from `RoleModels.Message.Provider`.
- [ ] `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
      `gofmt -l internal/ pkg/` empty; `make coverage-gate` PASS; go.mod/go.sum UNCHANGED.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ — YES. Every consumed symbol is named with its exact signature + file; the exact struct
field lists (from PRD §14.1, verified against the live code) are given; the exact `resolveConfig` reuse
point, the field-merge override semantics, the `Deps` literal fields, the `ResolveRoles` 2-return usage
(manifests → Deps, models → result mapping), the `decompose.CommitResult` → `Result` field mapping, the
test harness (`setupTestRepo`/`setupScriptedRepo` + the bare-stub-makes-ResolveRoles-fail mechanism), and
the validation gates are all specified below. The subtle points — that `resolveConfig` is reused as-is
(only the decompose overrides are layered on top), that `Count==0`/`MaxCommits==0` mean "inherit" (don't
clobber the file default), that `deps.Out = opts.Verbose` (nil-safe; no dedicated Out field on the PRD
struct), that the wrapper does NOT call `signal.Install` (matching `GenerateCommit`), that the internal
`decompose.DecomposeResult` lacks `Provider` so the wrapper must inject it, and that THIS task must NOT
touch `internal/cmd/` (parallel coordination with P4.M1.T1.S1) — are all explained, not just named.

### Documentation & References

```yaml
# MUST READ — include these in your context window
- url: PRD.md §14.1 (the public library surface) — the EXACT API contract to implement
  why: "Authoritative struct field lists + doc-comment intents for DecomposeOptions/DecomposeResult/RoleModel
        + the Decompose function signature + the 'NO-OP when Single/Count==1' + 'Caller must ensure nothing
        is staged' precondition. This is a verbatim implementation spec."
  critical: "DecomposeOptions embeds Options (NOT Options + a Message field — message = global, FR-R2).
        DecomposeResult.Commits is []Result (the PUBLIC Result), Amended int, Provider string. The embedded
        Options applies to the MESSAGE role. Doc comments: 'Stable as of v2.0' (additive-only convention)."

- url: PRD.md §13.6 (multi-commit decomposition) + §13.6.6 (failure handling / FR-M12)
  why: "The flow + the partial-result-on-failure contract. The wrapper must return partial commits + error
        when the loop aborts mid-decompose (FR-M12) — mapDecomposeResult runs BEFORE checking the error."
  critical: "FR-M12: on a per-concept failure, decompose.Decompose returns (DecomposeResult{Commits: 0..i-1},
        err). The wrapper returns (mapped partial DecomposeResult, err) — do NOT swallow the partial commits."

- url: PRD.md §9.14 (FR-M1 … FR-M4) + §9.15 (FR-R1–R5) + §16.4 (per-role config)
  why: "FR-M1 (precondition: nothing staged — caller's responsibility), FR-M2 (modes: Single/Count==1 ⇒
        single; Count≥2 ⇒ forced; Count==0 ⇒ auto), FR-M4 (max_commits safety cap, default 12). FR-R2 (a
        zero RoleModel ⇒ inherit global), FR-R3 (field-merge across layers), FR-R5 (models are provider-specific)."
  critical: "FR-M1: Decompose does NOT re-check HasStagedChanges (the internal orchestrator doesn't either —
        it trusts the caller). Document the precondition. Do NOT add a defensive HasStagedChanges check (it
        would deviate from the contract + the internal orchestrator's behavior)."

# CODEBASE FILES — pattern sources + consumed dependencies (all verified, paths exact)
- file: pkg/stagecoach/stagecoach.go   # EDIT TARGET + pattern source
  why: "This IS the file to edit. It already has: `package stagecoach` (Stable as of v1.0); `Options`,
        `Result`, `GenerateCommit`; typed-error re-exports (ErrNothingToCommit/ErrTimeout/ErrRescue/
        ErrCASFailed + RescueError/CASError aliases); the `resolveConfig(ctx, opts Options) (config.Config,
        string, error)` helper (loads 7-layer config OR uses opts.Config; applies opts.Provider/Model/
        Timeout/VerboseOn overrides); imports config/generate/git/prompt/provider/signal/ui."
  pattern: "REUSE `resolveConfig(ctx, opts.Options)` for the base config + Options overrides, then layer
        the decompose-specific overrides (Count/Single/MaxCommits/roles) on top in a new `resolveDecomposeConfig`.
        Mirror `GenerateCommit`'s doc-comment style ('Stable as of vX.Y'). Add the `internal/decompose` import.
        Place the new types near the existing Options/Result; place Decompose after GenerateCommit."
  gotcha: "The existing Options has MORE fields than PRD §14.1's original snippet (Verbose io.Writer,
        VerboseOn bool, Config *config.Config — additive-only). DecomposeOptions embeds the FULL current
        Options (embedding gives access to all). Do NOT redeclare any Options field."

- file: pkg/stagecoach/stagecoach_test.go   # EDIT TARGET + test-pattern source
  why: "The test harness: `setupTestRepo(t, stubtest.Options{Out: ...})` builds a temp repo + a repo-local
        .stagecoach.toml registering the BARE stub provider + chdir into it (GenerateCommit uses os.Getwd()).
        `setupScriptedRepo(t, headSubject, responses)` for per-call scripted responses. `writeFile`,
        `stageFile`, `commitRaw`, `headSHA`, `gitOut`, `shaRe`, `boolPtr` helpers exist. Tests use
        config/stubtest/exitcode imports."
  pattern: "For single-delegation tests: setupTestRepo (stub Out = 'feat: ...') → writeFile + stageFile a
        change (STAGED, since GenerateCommit needs a staged index) → call Decompose(ctx, {Single: true}) →
        assert 1 new commit (headSHA changed) + DecomposeResult fields. For multi-entry/override tests:
        setupTestRepo → writeFile an UN-STAGED change (do NOT git add) → call Decompose(ctx, opts) → assert
        the ResolveRoles error (message contains the expected substring)."
  gotcha: "The bare stub provider has NO tooled_flags → a full happy-path multi-commit Decompose test is
        IMPOSSIBLE (the real stager needs a tooled agent that runs git; the stub is BARE). Cover: single
        delegation (full E2E via GenerateCommit), multi-commit ENTRY (the stager-role ResolveRoles error
        proves entry), and role-override PLUMBING (Stager.Provider='nonexistent' → 'unknown provider'). The
        pipeline itself is tested in internal/decompose (stager seam). Do NOT attempt a full E2E multi test."

- file: internal/decompose/decompose.go   # CONSUMED — Decompose signature + DecomposeResult/CommitResult
  why: "`Decompose(ctx context.Context, deps Deps) (DecomposeResult, error)` (line ~158).
        `DecomposeResult{ Commits []CommitResult; Amended int }` — NOTE: NO Provider field (the wrapper adds it).
        `CommitResult{ SHA, Subject, Message string; Files []git.FileChange }` — maps to public Result.
        Decompose's PRECONDITION (FR-M1): caller routed correctly (nothing staged + dirty tree); it does NOT
        re-check. Mode routing INSIDE: Config.Single||Config.Commits==1 → runSingleEscape — but the PUBLIC
        wrapper short-circuits Single/Count==1 to GenerateCommit BEFORE calling internal Decompose, so the
        internal escape-hatch is only a defensive backstop."
  pattern: "Call `decompose.Decompose(ctx, deps)`; on error return `(mapDecomposeResult(ires, roleModels), err)`
        (partial commits + error — FR-M12); on success return `(mapDecomposeResult(ires, roleModels), nil)`."
  gotcha: "Decompose REQUIRES deps.Roles populated (ResolveRoles) and assumes correct routing (nothing
        staged). The wrapper builds Deps via ResolveRoles and trusts the caller on the staging precondition."

- file: internal/decompose/roles.go   # CONSUMED — Deps + ResolveRoles + RoleManifests + RoleModels
  why: "`Deps{ Git git.Git; Registry *provider.Registry; Config config.Config; Roles RoleManifests;
        Verbose *ui.Verbose; Out io.Writer; stager <unexported> }`. `ResolveRoles(cfg config.Config, reg
        *provider.Registry) (RoleManifests, RoleModels, error)` — resolves + install-checks all four roles;
        FR-D4 stager fallback (TooledFlags-less stager → first installed TooledFlags-capable provider, else
        error 'role \"stager\": provider … cannot stage (tooled_flags empty)'). Returns BOTH RoleManifests
        (→ Deps.Roles) AND RoleModels (→ result mapping). `RoleModels{ Planner, Stager, Message, Arbiter
        config.RoleConfig }` where RoleConfig is {Provider, Model}. `RoleModels.Message` is the resolved
        message-role (provider, model) — use it for DecomposeResult.Provider + each Result.Provider/Model."
  pattern: "overrides,_:=provider.DecodeUserOverrides(cfg.Providers); reg:=provider.NewRegistry(overrides);
        roleManifests, roleModels, err:=decompose.ResolveRoles(cfg, reg); deps:=decompose.Deps{Git:git.New(repoDir),
        Registry:reg, Config:cfg, Roles:roleManifests, Verbose:ui.NewVerbose(opts.Verbose, cfg.Verbose),
        Out:opts.Verbose}. The CLI (P4.M1.T1.S1) discards RoleModels; the PUBLIC wrapper USES it for mapping."
  gotcha: "ResolveRoles takes a config.Config VALUE (not *pointer) and a *provider.Registry. The bare stub
        (no tooled_flags) → ResolveRoles fails at the stager — this is the deterministic multi-entry test
        signal. `deps.Out = opts.Verbose` (nil-safe: the loop guards nil; rescue/CAS recipes skipped if nil,
        but the typed error is still returned)."

- file: internal/config/config.go   # CONSUMED — Config struct + RoleConfig + Defaults
  why: "`Config{ ... Commits int; Single bool; MaxCommits int; ... Roles map[string]RoleConfig; ... }`.
        `RoleConfig{ Provider, Model string }`. `Commits` 0 = auto; `MaxCommits` default 12; `Roles` nil ⇒
        all roles inherit global (FR-R2). `config.Defaults()` returns a Config with these set."
  pattern: "Layer decompose overrides onto the resolved cfg: `if opts.Count!=0 { cfg.Commits=opts.Count }`;
        `if opts.Single { cfg.Single=true }`; `if opts.MaxCommits!=0 { cfg.MaxCommits=opts.MaxCommits }`;
        field-merge each non-zero Planner/Stager/Arbiter RoleModel into cfg.Roles (init map if nil)."
  gotcha: "Count==0 and MaxCommits==0 are the 'inherit / unset' sentinels (they EQUAL the config default),
        so use `!= 0` guards — do NOT unconditionally assign, or you'd clobber a file-set value with 0.
        Single==false is the 'inherit' sentinel — use `if opts.Single`."

- file: internal/provider/registry.go   # CONSUMED — NewRegistry + DecodeUserOverrides
  why: "`provider.DecodeUserOverrides(cfg.Providers map[string]map[string]any) (map[string]Manifest, error)`;
        `provider.NewRegistry(map[string]Manifest) *Registry`. Mirror the existing buildDeps + the CLI's runDecompose."

- file: internal/git/git.go   # CONSUMED — git.New + FileChange
  why: "`git.New(workDir string) Git` (line 190). `git.FileChange` (the type behind CommitResult.Files)."

- file: internal/ui/verbose.go   # CONSUMED — NewVerbose
  why: "`ui.NewVerbose(w io.Writer, on bool) *Verbose` — the deps.Verbose type. `ui.NewVerbose(opts.Verbose,
        cfg.Verbose)` mirrors GenerateCommit."

- file: internal/generate/{rescue.go,generate.go}   # CONSUMED — RescueError, CASError (already aliased)
  why: "The rescue/CAS typed errors the loop propagates are ALREADY aliased in pkg/stagecoach
        (`RescueError = generate.RescueError`, `CASError = generate.CASError`). No new aliases needed.
        *decompose.DecomposeRescueError wraps a *RescueError field + Unwraps to it, so errors.As(&re) matches."
```

### Current Codebase tree (relevant subset)

```bash
pkg/stagecoach/
  stagecoach.go          # EDIT — +RoleModel +DecomposeOptions +DecomposeResult +Decompose
                        #         +resolveDecomposeConfig +mapDecomposeResult + import internal/decompose
  stagecoach_test.go     # EDIT — +4 Decompose tests (reuse setupTestRepo/setupScriptedRepo)
internal/decompose/     # CONSUMED (read-only) — Decompose, DecomposeResult, CommitResult, Deps,
                        #   ResolveRoles, RoleManifests, RoleModels, DecomposeRescueError (P3.M4.T1.S1/S2)
internal/config/        # CONSUMED — Config, RoleConfig, Defaults, ResolveRoleModel
internal/git/git.go     # CONSUMED — git.New, FileChange
internal/provider/      # CONSUMED — NewRegistry, DecodeUserOverrides
internal/ui/verbose.go  # CONSUMED — NewVerbose
internal/generate/      # CONSUMED — RescueError, CASError, sentinels (already aliased in pkg/stagecoach)
internal/cmd/           # OWNED BY P4.M1.T1.S1 (parallel) — DO NOT TOUCH
cmd/stagecoach/main.go   # CONSUMED (unchanged)
```

### Desired Codebase tree (files this task EDITS — no new files)

```bash
pkg/stagecoach/stagecoach.go        # +3 exported types + Decompose + 2 unexported helpers + import internal/decompose
pkg/stagecoach/stagecoach_test.go   # +TestDecompose_SingleDelegates +TestDecompose_Count1Delegates
                                  # +TestDecompose_MultiEntry_RoleError +TestDecompose_StagerOverride
```

### Known Gotchas of our codebase & Library Quirks

```go
// G-NO-CLI-EDIT (CRITICAL): This task runs IN PARALLEL with P4.M1.T1.S1, which EDITS
//   internal/cmd/default_action.go to call internal/decompose.Decompose directly. To avoid a
//   parallel-edit conflict, this task touches ONLY pkg/stagecoach/. The CLI swap to pkg/stagecoach.Decompose
//   is a LATER follow-up (deferred by the P4.M1.T1.S1 PRP). Do NOT edit internal/cmd/* here.

// G-INTERNAL-RESULT-HAS-NO-PROVIDER (CRITICAL): internal decompose.DecomposeResult is
//   { Commits []CommitResult; Amended int } — NO Provider field. The PRD public DecomposeResult HAS
//   Provider string. The wrapper injects it from roleModels.Message.Provider. Also: internal CommitResult
//   is {SHA, Subject, Message, Files} — it has NO Provider/Model; mapDecomposeResult pulls those from
//   roleModels.Message for each Result.

// G-RESOLVEROLES-TWO-RETURNS: ResolveRoles returns (RoleManifests, RoleModels, error). Deps.Roles takes
//   the RoleManifests (1st); the result mapping takes RoleModels.Message (2nd). The CLI discards the 2nd;
//   the PUBLIC wrapper USES it. Do NOT discard it.

// G-COUNT-MAXCOMMITS-ZERO-IS-INHERIT: opts.Count==0 and opts.MaxCommits==0 mean "inherit the config value"
//   (0 == the config default == auto/12). Apply them with `!= 0` guards so you don't clobber a file-set
//   value with 0. opts.Single==false means "inherit" — apply with `if opts.Single`.

// G-ROLE-FIELD-MERGE: opts.Planner/Stager/Arbiter are RoleModel{Provider, Model}. A zero RoleModel means
//   "inherit global" (FR-R2). Apply only the NON-EMPTY fields onto cfg.Roles[role] (FR-R3 field-merge):
//   init cfg.Roles if nil; copy the existing RoleConfig; set Provider/Model only when non-empty; write back.

// G-DEPS-OUT-SINK: The internal loop prints rescue/CAS recipes to deps.Out. The PRD DecomposeOptions
//   struct has NO dedicated Out field (only the embedded Options.Verbose io.Writer). Set deps.Out = opts.Verbose
//   (nil-safe: the loop guards nil → skips printing; the integrator still gets the typed *RescueError/*CASError
//   via the return). Do NOT hardcode os.Stderr (bad form for a library). Rescue is ALWAYS available via the error.

// G-NO-SIGNAL-INSTALL: The wrapper does NOT call signal.Install (matching GenerateCommit). The internal
//   loop arms signal.SetSnapshot/ClearSnapshot per-concept, which are nil-safe WITHOUT Install. For an
//   external consumer who didn't Install, arming is a no-op; the rescue is still returned as a typed error.
//   Do NOT add signal handling in the wrapper.

// G-SINGLE-DELEGATION-BEFORE-RESOLVE: The `if opts.Single || opts.Count==1` short-circuit calls
//   GenerateCommit(ctx, opts.Options) and returns IMMEDIATELY — it does NOT resolve decompose config or
//   call ResolveRoles. This is the contract's "NO-OP (delegates to GenerateCommit)". GenerateCommit honors
//   opts.DryRun (the multi-commit path does not). Single-delegation tests stage a change first.

// G-PRECONDITION-NOT-RECHECKED: Decompose does NOT re-check HasStagedChanges (the internal orchestrator
//   doesn't either; the CLI gates on it). Document the precondition in the doc comment. Do NOT add a
//   defensive check (deviates from the contract + the internal contract).

// G-PARTIAL-RESULT-ON-ERROR: On a mid-loop failure (FR-M12), decompose.Decompose returns (partial, err).
//   The wrapper returns (mapDecomposeResult(partial, roleModels), err) — do NOT swallow the partial commits.

// G-BARE-STUB-CANNOT-RUN-STAGER: The stubtest provider is BARE (no tooled_flags, no tools). A full
//   happy-path multi-commit Decompose test is impossible (the real stager runs git). The multi-entry test
//   asserts the ResolveRoles STAGER error (unique to the multi path). The pipeline is tested in internal/decompose.

// G-EMBEDDED-OPTIONS-VISIBLE: DecomposeOptions embeds Options, so opts.Options, opts.Provider, opts.Model,
//   opts.DryRun, opts.Timeout, opts.Verbose, opts.VerboseOn, opts.Config are all directly addressable on a
//   DecomposeOptions value. Pass opts.Options (the embedded struct) to resolveConfig/GenerateCommit.

// G-IMPORT-CYCLE-FREE (verified): internal/decompose imports only internal/{generate,git,prompt,signal,
//   config,provider,ui} — NOT pkg/stagecoach, NOT internal/cmd. So pkg/stagecoach → internal/decompose is safe.
```

## Implementation Blueprint

### Data models and structure

Add three exported types (verbatim field lists from PRD §14.1) and two unexported helpers. No new data
models beyond these (everything else is consumed verbatim):

```go
// RoleModel is a per-role provider/model override for DecomposeOptions (PRD §14.1, §16.4, FR-R1–R5).
// A zero value ⇒ the role inherits the global default (FR-R2); a non-empty field overrides just that
// field (FR-R3 field-merge). Models are provider-specific (FR-R5).
//
// Stable as of v2.0.
type RoleModel struct {
	Provider string
	Model    string
}

// DecomposeOptions configures the multi-commit pipeline (PRD §14.1, §13.6). The embedded Options
// (Provider/Model/DryRun/Timeout/Verbose/VerboseOn/Config) apply to the MESSAGE role. Count 0 ⇒
// auto-decompose (planner decides); >0 ⇒ force exactly Count commits. Single true ⇒ bypass the planner
// (delegate to GenerateCommit, the v1 single-commit path). MaxCommits 0 ⇒ the config default (12);
// >0 ⇒ override the safety cap. Planner/Stager/Arbiter are per-role overrides (zero ⇒ global default).
//
// Stable as of v2.0.
type DecomposeOptions struct {
	Options                  // embedded: Provider/Model/DryRun/Timeout/… apply to the MESSAGE role
	Count    int             // 0 ⇒ auto-decompose (planner decides); >0 ⇒ force exactly Count commits
	Single   bool            // true ⇒ bypass planner, force one GenerateCommit (--single)
	MaxCommits int           // 0 ⇒ config default (12); >0 ⇒ override the auto-decompose safety cap
	Planner  RoleModel       // planner role provider/model (zero ⇒ global default)
	Stager   RoleModel       // stager role provider/model (zero ⇒ global default)
	Arbiter  RoleModel       // arbiter role provider/model (zero ⇒ global default)
}

// DecomposeResult is the outcome of Decompose: the ordered commits created this run (PRD §14.1).
// Commits is one Result per concept that produced a commit (empty concepts skipped). Amended is the
// number of commits the arbiter folded leftovers into (0 if the arbiter did not run or made a new commit).
// Provider is the resolved MESSAGE-role provider (for display).
//
// Stable as of v2.0.
type DecomposeResult struct {
	Commits  []Result // one per concept that produced a commit (empty concepts skipped)
	Amended  int      // number of commits the arbiter folded leftovers into
	Provider string   // resolved MESSAGE provider (for display)
}

// Decompose turns a dirty, un-staged working tree into N logically-coherent commits (PRD §14.1, §13.6).
// It is a NO-OP that delegates to GenerateCommit when opts.Single is true or opts.Count == 1; otherwise it
// activates the planner→stager→message→arbiter pipeline (via internal/decompose.Decompose).
// PRECONDITION (FR-M1): the caller must ensure NOTHING is staged — Decompose does NOT re-check
// HasStagedChanges (the CLI gates on it). Options.DryRun is honored ONLY on the single-delegation path
// (the multi-commit pipeline always commits). On a per-concept failure (FR-M12) the already-landed commits
// are returned alongside a *RescueError / *CASError (the same typed errors GenerateCommit returns).
//
// Stable as of v2.0.
func Decompose(ctx context.Context, opts DecomposeOptions) (DecomposeResult, error)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT pkg/stagecoach/stagecoach.go — add the import + the three exported types
  - ADD import "github.com/dustin/stagecoach/internal/decompose" to the import block (alphabetical:
    after "github.com/dustin/stagecoach/internal/config" and before "internal/generate").
  - ADD the three exported types (RoleModel, DecomposeOptions, DecomposeResult) with the exact field lists
    + doc comments shown in "Data models and structure" above. PLACE them near the existing Options/Result
    types (e.g. after Result, before the typed-error re-export block) for cohesion.
  - VERIFY the field lists match PRD §14.1 EXACTLY: DecomposeOptions has NO Message field; DecomposeResult
    has Provider string; RoleModel has Provider + Model.
  - FOLLOW pattern: the existing Options/Result doc comments ("Stable as of v1.0" → these say v2.0).
  - NAMING: exported, exported-field Capitalized, no json tags (this package uses none).
  - PLACEMENT: pkg/stagecoach/stagecoach.go.

Task 2: EDIT pkg/stagecoach/stagecoach.go — add Decompose + resolveDecomposeConfig + mapDecomposeResult
  - ADD func Decompose(ctx context.Context, opts DecomposeOptions) (DecomposeResult, error):
        // (1) NO-OP delegation (PRD §14.1): Single or Count==1 → GenerateCommit, wrapped in a 1-commit
        //     DecomposeResult. Honors opts.DryRun (the single path is dry-run aware).
        if opts.Single || opts.Count == 1 {
            r, err := GenerateCommit(ctx, opts.Options)
            if err != nil {
                return DecomposeResult{}, err
            }
            return DecomposeResult{Commits: []Result{r}, Amended: 0, Provider: r.Provider}, nil
        }
        // (2) Multi-commit path: resolve config → ResolveRoles → build Deps → decompose.Decompose → map.
        cfg, repoDir, err := resolveDecomposeConfig(ctx, opts)
        if err != nil {
            return DecomposeResult{}, err
        }
        overrides, err := provider.DecodeUserOverrides(cfg.Providers)
        if err != nil {
            return DecomposeResult{}, fmt.Errorf("decompose: provider overrides: %w", err)
        }
        reg := provider.NewRegistry(overrides)
        roleManifests, roleModels, err := decompose.ResolveRoles(cfg, reg)
        if err != nil {
            return DecomposeResult{}, fmt.Errorf("decompose: %w", err)
        }
        deps := decompose.Deps{
            Git:      git.New(repoDir),
            Registry: reg,
            Config:   cfg,
            Roles:    roleManifests,
            Verbose:  ui.NewVerbose(opts.Verbose, cfg.Verbose),
            Out:      opts.Verbose, // nil-safe rescue/CAS sink (G-DEPS-OUT-SINK); no dedicated field on the PRD struct
        }
        ires, derr := decompose.Decompose(ctx, deps)
        return mapDecomposeResult(ires, roleModels), derr // partial + error on FR-M12 (G-PARTIAL-RESULT-ON-ERROR)
  - ADD func resolveDecomposeConfig(ctx context.Context, opts DecomposeOptions) (config.Config, string, error):
        cfg, repoDir, err := resolveConfig(ctx, opts.Options) // reuse the base 7-layer config + Options overrides
        if err != nil {
            return config.Config{}, "", err
        }
        // Decompose-specific overrides (highest precedence — explicit intent wins over file/env/git-config).
        // G-COUNT-MAXCOMMITS-ZERO-IS-INHERIT: 0 means "inherit the config value" (don't clobber).
        if opts.Count != 0 {
            cfg.Commits = opts.Count
        }
        if opts.Single {
            cfg.Single = true // (already short-circuited above, but set for consistency/safety)
        }
        if opts.MaxCommits != 0 {
            cfg.MaxCommits = opts.MaxCommits
        }
        // Per-role field-merge (G-ROLE-FIELD-MERGE): zero RoleModel ⇒ inherit global (FR-R2).
        if opts.Planner.Provider != "" || opts.Planner.Model != "" ||
            opts.Stager.Provider != "" || opts.Stager.Model != "" ||
            opts.Arbiter.Provider != "" || opts.Arbiter.Model != "" {
            if cfg.Roles == nil {
                cfg.Roles = map[string]config.RoleConfig{}
            }
            applyRoleOverride(cfg.Roles, "planner", opts.Planner)
            applyRoleOverride(cfg.Roles, "stager", opts.Stager)
            applyRoleOverride(cfg.Roles, "arbiter", opts.Arbiter)
        }
        return cfg, repoDir, nil
  - ADD func applyRoleOverride(roles map[string]config.RoleConfig, role string, rm RoleModel):
        if rm.Provider == "" && rm.Model == "" {
            return
        }
        rc := roles[role] // copy (zero value if absent)
        if rm.Provider != "" {
            rc.Provider = rm.Provider
        }
        if rm.Model != "" {
            rc.Model = rm.Model
        }
        roles[role] = rc
  - ADD func mapDecomposeResult(ires decompose.DecomposeResult, roleModels decompose.RoleModels) DecomposeResult:
        commits := make([]Result, len(ires.Commits))
        for i, c := range ires.Commits {
            commits[i] = Result{
                CommitSHA: c.SHA,
                Subject:   c.Subject,
                Message:   c.Message,
                Provider:  roleModels.Message.Provider,
                Model:     roleModels.Message.Model,
            }
        }
        return DecomposeResult{
            Commits:  commits,
            Amended:  ires.Amended,
            Provider: roleModels.Message.Provider, // G-INTERNAL-RESULT-HAS-NO-PROVIDER
        }
  - FOLLOW pattern: resolveConfig (reuse), GenerateCommit's doc-comment style, the existing fmt.Errorf
    wrapping idiom. The three unexported helpers (resolveDecomposeConfig, applyRoleOverride,
    mapDecomposeResult) are siblings to resolveConfig/buildDeps/buildSysPrompt.
  - PRESERVE: GenerateCommit, resolveConfig, buildDeps, buildSysPrompt, runPipeline, all typed-error
    re-exports UNCHANGED.
  - NAMING: exported Decompose; unexported resolveDecomposeConfig/applyRoleOverride/mapDecomposeResult.
  - PLACEMENT: pkg/stagecoach/stagecoach.go (Decompose after GenerateCommit; helpers after resolveConfig).

Task 3: EDIT pkg/stagecoach/stagecoach_test.go — single-delegation tests (full E2E)
  - ADD TestDecompose_SingleDelegates:
        setupTestRepo(t, stubtest.Options{Out: "feat: decompose single"}) // bare stub, repo-local .toml
        writeFile(t, ".", "a.txt", "change\n") // repo is CWD (setupTestRepo chdir'd into it)
        stageFile(t, ".", "a.txt")             // STAGE — GenerateCommit needs a staged index
        before := headSHA(t, ".")
        res, err := stagecoach.Decompose(context.Background(), stagecoach.DecomposeOptions{Single: true})
        // assertions:
        assert err == nil
        assert headSHA(t, ".") != before              // exactly 1 new commit
        assert len(res.Commits) == 1
        assert res.Amended == 0
        assert res.Provider != ""                     // mapping populated Provider
        assert res.Commits[0].Provider != "" && res.Commits[0].CommitSHA != ""
        assert strings.HasPrefix(res.Commits[0].Subject, "feat: decompose single") // (after fence-strip)
  - ADD TestDecompose_Count1Delegates:
        identical to the above but with DecomposeOptions{Count: 1} (no Single) — proves Count==1 delegates.
  - GOTCHA: setupTestRepo writes .stagecoach.toml + initial commit + chdir's into the temp repo. "." is the
    repo dir. The stub Out must be a VALID single-line message (the stub returns it as the commit message;
    setupTestRepo uses STAGECOACH_STUB_OUT mode). Use stageFile to stage (GenerateCommit reads the staged
    diff). headSHA/gitOut/writeFile/stageFile/shaRe are the existing helpers.
  - COVERAGE: the single-delegation branch + the DecomposeResult wrapping + mapDecomposeResult (via the
    Provider/CommitSHA/Subject assertions).

Task 4: EDIT pkg/stagecoach/stagecoach_test.go — multi-commit entry + role-override tests
  - ADD TestDecompose_MultiEntry_RoleError:
        setupTestRepo(t, stubtest.Options{Out: "feat: x"}) // bare stub (NO tooled_flags)
        writeFile(t, ".", "b.txt", "un-staged change\n")   // dirty working tree, NOT staged
        // (do NOT stageFile — the precondition is nothing staged + dirty tree)
        res, err := stagecoach.Decompose(context.Background(), stagecoach.DecomposeOptions{})
        // assertions:
        assert err != nil
        assert strings.Contains(err.Error(), "stager")   // ResolveRoles stager-role error
        assert strings.Contains(err.Error(), "tooled")   // the FR-D4 fallback failure message
        assert res.Commits == nil || len(res.Commits) == 0  // nothing landed (ResolveRoles failed pre-loop)
  - ADD TestDecompose_StagerOverride:
        setupTestRepo(t, stubtest.Options{Out: "feat: x"})
        writeFile(t, ".", "b.txt", "un-staged change\n")
        res, err := stagecoach.Decompose(context.Background(), stagecoach.DecomposeOptions{
            Stager: stagecoach.RoleModel{Provider: "nonexistent"},
        })
        // assertions: the override flowed into cfg.Roles["stager"] → ResolveRoles fails with
        // "unknown provider" (distinct from the tooled_flags error), proving the override was applied.
        assert err != nil
        assert strings.Contains(err.Error(), "stager")
        assert strings.Contains(err.Error(), "unknown provider")
        assert res.Commits == nil || len(res.Commits) == 0
  - GOTCHA: these tests PROVE the multi-commit path was entered (resolveDecomposeConfig + ResolveRoles +
    Deps construction ran; the single short-circuit did NOT fire) WITHOUT needing a tooled stager. The
    bare stub's lack of tooled_flags is the deterministic entry signal. The "unknown provider" substring
    in the override test is UNIQUE to having set Stager.Provider (proves the field-merge).
  - COVERAGE: multi-commit entry + role-override plumbing. (A full happy-path multi test is impossible
    with the bare stub — covered in internal/decompose via the stager seam.)
```

### Implementation Patterns & Key Details

```go
// The Decompose wrapper — the whole task in one function:
//
//   func Decompose(ctx context.Context, opts DecomposeOptions) (DecomposeResult, error) {
//       // (1) NO-OP delegation: Single/Count==1 → GenerateCommit (honors DryRun).
//       if opts.Single || opts.Count == 1 {
//           r, err := GenerateCommit(ctx, opts.Options)
//           if err != nil { return DecomposeResult{}, err }
//           return DecomposeResult{Commits: []Result{r}, Amended: 0, Provider: r.Provider}, nil
//       }
//       // (2) Multi-commit: resolve → ResolveRoles → Deps → internal Decompose → map.
//       cfg, repoDir, err := resolveDecomposeConfig(ctx, opts)
//       if err != nil { return DecomposeResult{}, err }
//       overrides, err := provider.DecodeUserOverrides(cfg.Providers)
//       if err != nil { return DecomposeResult{}, fmt.Errorf("decompose: provider overrides: %w", err) }
//       reg := provider.NewRegistry(overrides)
//       roleManifests, roleModels, err := decompose.ResolveRoles(cfg, reg)
//       if err != nil { return DecomposeResult{}, fmt.Errorf("decompose: %w", err) }
//       deps := decompose.Deps{
//           Git: git.New(repoDir), Registry: reg, Config: cfg, Roles: roleManifests,
//           Verbose: ui.NewVerbose(opts.Verbose, cfg.Verbose), Out: opts.Verbose,
//       }
//       ires, derr := decompose.Decompose(ctx, deps)
//       return mapDecomposeResult(ires, roleModels), derr
//   }
//
// Why resolveConfig is reused (not reimplemented): it already does the 7-layer config.Load (or the
// opts.Config fast-path) + applies opts.Provider/Model/Timeout/VerboseOn. The only NEW logic is the
// decompose-specific overrides (Count/Single/MaxCommits/roles) — layered on top in resolveDecomposeConfig.
//
// Why roleModels.Message (not roleManifests.Message): the manifest carries command/flags (for Render);
// the resolved (provider, model) PAIR lives in RoleModels.Message. CommitResult has no provider/model,
// so mapDecomposeResult sources both DecomposeResult.Provider and each Result.Provider/Model from
// roleModels.Message — the resolved MESSAGE role, which every concept's message uses.
//
// Why deps.Out = opts.Verbose (not os.Stderr / not a new field): the PRD DecomposeOptions struct has no
// dedicated Out field (it's fixed by §14.1). opts.Verbose is the integrator's io.Writer sink; nil is
// safe (the loop guards nil → skips printing; the typed error is still returned). A library must not
// hardcode os.Stderr. The rescue recipe is ALWAYS in the returned *RescueError/*CASError regardless.
```

### Integration Points

```yaml
PUBLIC API (pkg/stagecoach/stagecoach.go):
  - add to: pkg/stagecoach (the existing package)
  - pattern: mirror GenerateCommit's doc-comment style + the Options/Result struct shape; "Stable as of v2.0"
INTERNAL DECOMPOSE: CONSUMED — Decompose, ResolveRoles, Deps, RoleManifests, RoleModels, CommitResult,
  DecomposeResult (internal — no Provider field). The wrapper maps internal→public.
CONFIG: CONSUMED — Config.{Commits,Single,MaxCommits,Roles}, RoleConfig, Defaults. No config changes
  (all fields already shipped in P1.M3). The wrapper LAYERS opts onto the resolved cfg (does NOT re-load).
PROVIDER: CONSUMED — NewRegistry, DecodeUserOverrides (mirror buildDeps + the CLI's runDecompose).
GIT: CONSUMED — git.New(repoDir). No new git methods.
UI: CONSUMED — ui.NewVerbose(opts.Verbose, cfg.Verbose). No changes.
GENERATE: CONSUMED — RescueError, CASError, sentinels (ALREADY aliased in pkg/stagecoach — no new aliases).
SIGNAL: CONSUMED (no-op) — the wrapper does NOT call signal.Install; the loop's arming is nil-safe. No changes.
CLI (internal/cmd): NOT TOUCHED — owned by parallel P4.M1.T1.S1. The swap to pkg/stagecoach.Decompose is a follow-up.
MAIN (cmd/stagecoach/main.go): UNCHANGED.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after the edits — fix before proceeding.
go build ./...                       # compile (3 types + Decompose + 2 helpers + the internal/decompose import)
go vet ./...                         # shadowed vars, printf, unkeyed literals
gofmt -l internal/ pkg/              # MUST print nothing
golangci-lint run                    # repo linter (Makefile `make lint`) — includes `unused`

# Scope-specific quick check:
go build ./pkg/stagecoach/... && go vet ./pkg/stagecoach/...

# Expected: zero errors. Verify: the new internal/decompose import is USED (Decompose references
# decompose.Decompose/ResolveRoles/Deps/DecomposeResult/RoleModels) — else `unused import`. Verify the
# three new types have "Stable as of v2.0" doc comments. Verify NO Message field on DecomposeOptions.
# Verify gofmt did not reorder the DecomposeOptions fields (gofmt is field-order-neutral, but keep the
# PRD §14.1 order: Options, Count, Single, MaxCommits, Planner, Stager, Arbiter).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The four new tests:
go test -race ./pkg/stagecoach/ -run 'TestDecompose_SingleDelegates|TestDecompose_Count1Delegates' -v
go test -race ./pkg/stagecoach/ -run 'TestDecompose_MultiEntry_RoleError|TestDecompose_StagerOverride' -v

# Full pkg/stagecoach suite (regression — existing GenerateCommit tests stay green):
go test -race ./pkg/stagecoach/...

# Whole suite (Makefile `make test`):
go test -race ./...

# Expected: all pass. If TestDecompose_SingleDelegates makes 0 commits, GenerateCommit wasn't reached
# (check the staged change was actually staged + the short-circuit fired). If the multi-entry test gets
# err==nil, the short-circuit leaked (Single/Count falsy check) OR the stub has tooled_flags (it doesn't).
# If the override test says "tooled" instead of "unknown provider", the Stager override wasn't applied
# (check applyRoleOverride + the != "" guards). If `unused` fires, a helper is unreferenced.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the module + a binary smoke (the public API is library-only; the binary is unchanged):
go build ./...
go build -o /tmp/stagecoach ./cmd/stagecoach   # smoke (no CLI behavior change from this task)

# Verify the public API compiles for an external-style import + the symbols are exported:
go doc github.com/dustin/stagecoach/pkg/stagecoach.Decompose        # prints the Decompose doc comment
go doc github.com/dustin/stagecoach/pkg/stagecoach.DecomposeOptions # prints the options doc comment
go doc github.com/dustin/stagecoach/pkg/stagecoach.DecomposeResult
go doc github.com/dustin/stagecoach/pkg/stagecoach.RoleModel
# Expected: each prints a doc comment containing "Stable as of v2.0".

# Coverage gate (pkg/stagecoach is NOT gated, but internal/ packages ARE — confirm no regression):
make coverage-gate      # enforces >=85% on internal/{git,provider,generate,config}

# Expected: PASS. This task edits only pkg/stagecoach (not gated) + its test; it consumes (does not change)
# internal/* symbols, so no gated package's coverage is impacted. go.mod/go.sum MUST be unchanged (all
# imports — including internal/decompose — are already module-internal / vendored).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# API-shape invariant (go doc is authoritative; the tests encode the behavior):
#   DecomposeOptions embeds Options; adds Count/Single/MaxCommits/Planner/Stager/Arbiter; NO Message field.
#   DecomposeResult { Commits []Result; Amended int; Provider string }.
#   RoleModel { Provider, Model string }.
#   Decompose(ctx, opts) (DecomposeResult, error) — NO-OP delegates when Single||Count==1.

# Behavior invariant (the 4 tests encode this):
#   Single==true (staged change) → GenerateCommit → 1 commit, Provider set, err==nil.
#   Count==1 (staged change)     → GenerateCommit → 1 commit (identical to Single).
#   {} (dirty/un-staged + bare stub) → ResolveRoles stager error ("stager"+"tooled"); 0 commits.
#   {Stager:{Provider:"nonexistent"}} (dirty tree) → ResolveRoles "unknown provider" error; 0 commits.

# Scope invariant (CRITICAL): diff internal/cmd/ — it MUST be unchanged by this task (parallel P4.M1.T1.S1
# owns it). `git diff --stat internal/cmd/` prints nothing attributable to P4.M2.T1.S1.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` compiles (3 types + Decompose + 2 helpers + the `internal/decompose` import).
- [ ] `go vet ./...` clean.
- [ ] `go test -race ./...` green (Makefile `make test`).
- [ ] `golangci-lint run` clean (Makefile `make lint`) — `unused` does NOT fire on the new helpers/types.
- [ ] `gofmt -l internal/ pkg/` empty.
- [ ] `make coverage-gate` PASS (no gated-package regression — pkg/stagecoach edits only).
- [ ] go.mod/go.sum UNCHANGED (config/decompose/git/provider/ui/generate all already imported/vendored).

### Feature Validation

- [ ] `pkg/stagecoach` exports `Decompose`, `DecomposeOptions`, `DecomposeResult`, `RoleModel` (and ONLY these
      new symbols — no new error sentinels).
- [ ] Each new exported symbol has a "Stable as of v2.0" doc comment (verified via `go doc`).
- [ ] `DecomposeOptions` field list matches PRD §14.1 EXACTLY (embedded Options + Count/Single/MaxCommits/
      Planner/Stager/Arbiter; NO Message field).
- [ ] `Decompose(ctx, {Single: true})` + staged change → 1 commit, Provider set, err==nil (TestDecompose_SingleDelegates).
- [ ] `Decompose(ctx, {Count: 1})` behaves identically (TestDecompose_Count1Delegates).
- [ ] `Decompose(ctx, {})` + dirty/un-staged tree + bare stub → "stager"+"tooled" error (TestDecompose_MultiEntry_RoleError).
- [ ] `Decompose(ctx, {Stager:{Provider:"nonexistent"}})` + dirty tree → "stager"+"unknown provider" error
      (TestDecompose_StagerOverride — proves role-override field-merge).
- [ ] `resolveDecomposeConfig` reuses `resolveConfig` (no config re-load) and layers Count/Single/MaxCommits/roles.
- [ ] `mapDecomposeResult` converts `decompose.CommitResult`→`Result` and injects `Provider` from
      `roleModels.Message.Provider` (internal result has no Provider field).
- [ ] Partial commits returned with the error on FR-M12 (mapDecomposeResult runs before the error check).

### Code Quality Validation

- [ ] Follows existing pkg/stagecoach patterns (resolveConfig reuse, GenerateCommit doc-comment style, fmt.Errorf idiom).
- [ ] File placement matches the desired tree (edits to stagecoach.go + stagecoach_test.go ONLY).
- [ ] Anti-patterns avoided (see below): no CLI encroachment, no defensive HasStagedChanges check, no signal.Install.
- [ ] The `internal/decompose` import is USED (no `unused import`); the new unexported helpers are USED.
- [ ] The decompose-specific overrides use `!= 0`/`if opts.Single` guards (zero = inherit, don't clobber).
- [ ] Decompose does NOT touch internal/cmd (parallel coordination with P4.M1.T1.S1).

### Documentation & Deployment

- [ ] Mode-A doc comments on all four new exported symbols ("Stable as of v2.0"; cite PRD §14.1/§13.6/FR-M1/FR-M12).
- [ ] The Decompose doc comment states the PRECONDITION (caller ensures nothing is staged — FR-M1) and that
      DryRun is honored only on the single-delegation path.
- [ ] A code comment at the single-delegation branch + the deps.Out assignment noting the design rationale
      (NO-OP delegation; no dedicated Out field on the PRD struct → opts.Verbose).
- [ ] No new environment variables, config keys, or CLI flags (all already shipped via config/Defaults).
- [ ] The changeset-level README/docs update is P4.M3.T1.S1 (NOT this task — do not edit README.md).

---

## Anti-Patterns to Avoid

- ❌ Don't edit `internal/cmd/` — it is owned by the PARALLEL P4.M1.T1.S1 task (which calls
  internal/decompose.Decompose directly). The CLI swap to pkg/stagecoach.Decompose is a LATER follow-up.
  Encroaching here causes a parallel-edit race/merge-conflict.
- ❌ Don't add a `Message` field to DecomposeOptions — PRD §14.1 has none (message = global, FR-R2). The
  MESSAGE role inherits opts.Provider/opts.Model via the embedded Options + resolveConfig.
- ❌ Don't discard `ResolveRoles`'s 2nd return (RoleModels) — the PUBLIC wrapper USES
  `roleModels.Message.Provider`/`.Model` for result mapping (the CLI discards it; the wrapper must not).
- ❌ Don't forget to inject `DecomposeResult.Provider` — the internal `decompose.DecomposeResult` has NO
  Provider field; the public one does. `mapDecomposeResult` sets it from `roleModels.Message.Provider`.
- ❌ Don't unconditionally assign `cfg.Commits = opts.Count` / `cfg.MaxCommits = opts.MaxCommits` — 0 means
  "inherit the config value" (0 == the default). Use `!= 0` guards so a file-set value isn't clobbered.
- ❌ Don't add a defensive `HasStagedChanges` check — the contract + the internal orchestrator trust the
  caller (FR-M1). Document the precondition; don't re-check it.
- ❌ Don't call `signal.Install` — the wrapper matches GenerateCommit (no signal install). The loop's
  SetSnapshot/ClearSnapshot are nil-safe without Install; the rescue is still returned as a typed error.
- ❌ Don't hardcode `os.Stderr` for `deps.Out` — bad form for a library. Use `opts.Verbose` (the integrator's
  writer; nil-safe). The rescue recipe is ALWAYS in the returned `*RescueError`/`*CASError` regardless.
- ❌ Don't reimplement `resolveConfig` — reuse it and layer the decompose overrides on top. Reimplementing
  would desync from the 7-layer precedence + the opts.Config fast-path.
- ❌ Don't swallow partial commits on FR-M12 — `decompose.Decompose` returns `(partial, err)`; the wrapper
  returns `(mapDecomposeResult(partial, roleModels), err)`. The landed commits are real and stand.
- ❌ Don't add new error sentinels/aliases — rescue/CAS reuse the already-aliased `RescueError`/`CASError`/
  `ErrRescue`/`ErrTimeout`/`ErrCASFailed`. Planner/stager/arbiter errors surface as generic wrapped errors.
- ❌ Don't attempt a full happy-path multi-commit test with the bare stub — it has no tools/tooled_flags; the
  real stager runs git. Cover single-delegation (E2E), multi-ENTRY (ResolveRoles stager error), and
  role-override plumbing. The pipeline is tested in internal/decompose (stager seam).
- ❌ Don't touch GenerateCommit / resolveConfig / buildDeps / runPipeline / the typed-error re-exports —
  only ADD the new types + Decompose + helpers. Regressing GenerateCommit breaks the single path + the
  delegation tests.
