# Research Findings — P4.M2.T1.S1 (Public Decompose API)

## Task
Add `DecomposeOptions`, `DecomposeResult`, `RoleModel` types + `Decompose()` function to
`pkg/stagecoach/stagecoach.go`. The public wrapper delegates to `internal/decompose.Decompose`.

## Key findings

### 1. The internal contract being wrapped (`internal/decompose`)
- `decompose.Decompose(ctx context.Context, deps Deps) (decompose.DecomposeResult, error)` — top-level
  orchestrator (decompose.go:158). PRECONDITION (FR-M1): caller routed correctly (nothing staged +
  dirty tree). It does NOT re-check. Mode routing INSIDE: `deps.Config.Single || deps.Config.Commits==1`
  → `runSingleEscape` (AddAll → `generate.CommitStaged`).
- `decompose.Deps` (roles.go): `Git git.Git; Registry *provider.Registry; Config config.Config;
  Roles RoleManifests; Verbose *ui.Verbose; Out io.Writer; stager <unexported test seam>`.
- `decompose.ResolveRoles(cfg config.Config, reg *provider.Registry) (RoleManifests, RoleModels, error)`
  — resolves + install-checks all four roles; FR-D4 stager fallback (TooledFlags-less stager → first
  installed TooledFlags-capable provider, else error). Returns BOTH manifests (Deps.Roles) AND
  `(provider, model)` pairs (RoleModels). The CLI discards RoleModels; the PUBLIC wrapper NEEDS
  `RoleModels.Message` for result mapping.
- `decompose.DecomposeResult{ Commits []CommitResult; Amended int }` — NOTE: NO `Provider` field
  (the public `DecomposeResult` HAS one — the wrapper must ADD it from `RoleModels.Message.Provider`).
- `decompose.CommitResult{ SHA, Subject, Message string; Files []git.FileChange }` — maps to the
  public `Result` (SHA→CommitSHA; Subject→Subject; Message→Message; Provider/Model from RoleModels.Message).
- Error contract: planner/safety errors are NON-rescue (`ErrPlannerFailed`-wrapped); `*generate.RescueError`
  + `*generate.CASError` propagate DIRECTLY (errors.As-able); `*DecomposeRescueError` wraps a `*RescueError`
  field + Unwraps to it. Partial results returned with the error (FR-M12).

### 2. The existing public surface to extend (`pkg/stagecoach/stagecoach.go`)
- `package stagecoach`, "Stable as of v1.0". Existing exports: `Options`, `Result`, `GenerateCommit`.
- `Options` struct has MORE fields than the PRD §14.1 original (additive-only): `Provider, Model,
  SystemExtra, DryRun, Timeout, Verbose io.Writer, VerboseOn bool, Config *config.Config`.
- `Result{ CommitSHA, Subject, Message, Provider, Model string }`.
- Existing typed-error re-exports: `ErrNothingToCommit, ErrTimeout, ErrRescue, ErrCASFailed` (sentinels)
  + `RescueError = generate.RescueError` + `CASError = generate.CASError` (type aliases). The public
  Decompose reuses these (rescue/CAS propagate from the loop); NO new error exports needed.
- `resolveConfig(ctx, opts Options) (config.Config, string, error)` — reusable: loads config (or uses
  `opts.Config` if non-nil), applies `opts.Provider/Model/Timeout/VerboseOn` overrides (highest prec),
  returns `(cfg, repoDir, err)`. The wrapper extends this with DecomposeOptions-specific overrides.
- `buildDeps` (single-role resolver) is NOT reused by decompose (ResolveRoles does the four-role analog).

### 3. Mapping `DecomposeOptions` → `config.Config`
- `Options` (embedded) → `resolveConfig` applies Provider/Model/Timeout/VerboseOn. (DryRun applies to
  the single-delegation path; the multi-commit loop is NOT dry-run aware.)
- `Count int` (0=auto) → `cfg.Commits` (only override when !=0, since 0 == config default == auto).
- `Single bool` → `cfg.Single = true` when set.
- `MaxCommits int` (default 12) → `cfg.MaxCommits` (only when !=0; 0 == unset == inherit config default).
- `Planner/Stager/Arbiter RoleModel` → `cfg.Roles[role]` via field-merge (only override non-empty
  Provider/Model — "zero ⇒ global default" per PRD §14.1). NO Message field (message = global, FR-R2).

### 4. Import-cycle safety (verified)
- `internal/decompose` imports: internal/{generate,git,prompt,signal,config,provider,ui}. It does NOT
  import `pkg/stagecoach` or `internal/cmd`. So `pkg/stagecoach` → `internal/decompose` is cycle-free.

### 5. Signal handling
- The internal loop arms `signal.SetSnapshot`/`ClearSnapshot` per-concept (P3.M4.T1.S2) — these are
  nil-safe WITHOUT `signal.Install` (the CLI/main.go calls Install). The public wrapper does NOT call
  `signal.Install` (matching `GenerateCommit`, which also does not). For an external library consumer
  who didn't Install, the loop's arming is a no-op; the rescue is STILL returned as a typed error.

### 6. Test feasibility (critical constraint)
- The bare stub provider (stubtest) has NO `tooled_flags` → `ResolveRoles` FAILS at the stager-role
  fallback ("role \"stager\": provider \"stub\" cannot stage (tooled_flags empty) …"). This makes a
  FULL happy-path multi-commit test IMPOSSIBLE with the stub harness (same constraint as P4.M1.T1.S1).
- BUT it enables clean deterministic tests of the public wrapper:
  - **Single-delegation (full E2E)**: `opts.Single`/`Count==1` + STAGED change → delegates to
    `GenerateCommit` (which needs a staged index) → 1 commit; assert `DecomposeResult{Commits:1, Provider}`.
  - **Multi-commit entry (config error)**: bare stub + dirty/un-staged tree + `Decompose({})` →
    `ResolveRoles` stager error (proves: resolveDecomposeConfig + ResolveRoles + Deps construction ran,
    and the multi-commit path was entered, NOT delegated).
  - **Role-override proof**: `opts.Stager.Provider="nonexistent"` + dirty tree → ResolveRoles error
    "role \"stager\": unknown provider \"nonexistent\"" (proves the override flowed into cfg.Roles).
  - **Result mapping**: covered by the single-delegation test's assertions on DecomposeResult fields.

### 7. Coordination with parallel P4.M1.T1.S1 (CRITICAL SCOPE BOUNDARY)
- P4.M1.T1.S1 (running in parallel) EDITS `internal/cmd/default_action.go` to call
  `internal/decompose.Decompose` DIRECTLY (it cannot call `pkg/stagecoach.Decompose` — no dependency).
- To AVOID a parallel-edit conflict, THIS task MUST NOT touch `internal/cmd/`. It adds the public API
  to `pkg/stagecoach/stagecoach.go` ONLY. The CLI swap (internal/decompose.Decompose → pkg/stagecoach.Decompose)
  is a LATER follow-up, NOT this task. The P4.M1.T1.S1 PRP explicitly acknowledges this coordination point.

### 8. deps.Out sink decision
- The internal loop prints rescue/CAS recipes to `deps.Out`. The PRD `DecomposeOptions` struct (§14.1)
  has no dedicated `Out` field — only the embedded `Options.Verbose io.Writer`. The wrapper sets
  `deps.Out = opts.Verbose` (nil-safe: nil → loop skips printing; integrator still gets typed error).
  Rescue recipes are ALWAYS available via the returned `*RescueError`/`*CASError` regardless.

### 9. Validation gates (verified project commands)
- `go build ./... && go vet ./... && go test -race ./...`
- `golangci-lint run` (includes `unused`)
- `gofmt -l internal/ pkg/` (must be empty)
- `make coverage-gate` (>=85% on internal/{git,provider,generate,config}; pkg/stagecoach NOT gated)
- go.mod/go.sum UNCHANGED (no new deps — all imports already in stagecoach.go except internal/decompose, already vendored).
