# System Context — Stagecoach v3 Changeset Bugfixes

## What Stagecoach Is

Stagecoach is a Go CLI (`module github.com/dustin/stagecoach`, Go 1.22) that generates
AI-authored git commits. Two paths:

1. **Single-commit path** (`pkg/stagecoach.GenerateCommit` → `internal/generate.CommitStaged`
   or `runPipeline`): one commit from staged changes. The **message role** is the only
   active role.
2. **Multi-commit decompose path** (`pkg/stagecoach.Decompose` → `internal/decompose.Decompose`):
   planner → stager → message → arbiter pipeline turning a dirty tree into N coherent commits.

## Architecture Layers (relevant to the 5 issues)

```
internal/cmd/          CLI (cobra): flags, env, dispatch
  ├── root.go          flag registration (--reasoning, --message-model, etc.)
  ├── default_action.go  runDefault → GenerateCommit (single path)
  └── ...
internal/config/       Config loading + resolution
  ├── load.go          loadFlags/loadEnv → setRole{Model,Provider,Reasoning} → cfg.Roles[role]
  ├── roles.go         ResolveRoleModel(role, cfg) → (provider, model, reasoning); defaultRoleReasoning
  ├── file.go          loadTOML → fileConfig (go-toml/v2 decode) → materialize
  ├── migrate.go       migrateV2ToV3 (in-memory config migration)
  └── bootstrap.go     bootstrapHeader (generated config template header)
internal/provider/     Provider manifests + Render
  ├── manifest.go      Manifest struct (incl. ReasoningLevels map[string][]string)
  ├── builtin.go       BuiltinManifests(): 8 providers (pi, claude, gemini, agy, qwen-code, ...)
  ├── render.go        Manifest.Render(model, sys, payload, reasoning) → CmdSpec
  ├── merge.go         MergeManifest (user overrides)
  └── registry.go      Registry: Get(name), IsInstalled, DefaultProvider
internal/generate/     Single-commit orchestration
  └── generate.go      CommitStaged(ctx, deps, cfg) — Render + dedupe + commit
internal/decompose/    Multi-commit orchestration
  ├── decompose.go     Decompose: FreezeWorkingTree → planner → runOneFileShortcut / runSingleShortcut / runLoop
  ├── message.go       publishCommit, generateMessage (uses ResolveRoleModel("message"))
  ├── roles.go         decompose.Deps, ResolveRoles (per-role manifest selection)
  └── ...
pkg/stagecoach/         Public library API
  └── stagecoach.go     GenerateCommit → resolveConfig → buildDeps → CommitStaged/runPipeline
                       Decompose → ResolveRoles → internal.Decompose
internal/git/          Git primitives (ReadTree, CommitTree, UpdateRefCAS, FreezeWorkingTree, ...)
```

## Key Invariants & Data Flows

### Provider Manifest Selection
- **Single path**: `buildDeps(cfg, repoDir)` (`pkg/stagecoach/stagecoach.go:316`) selects ONE
  manifest from `cfg.Provider` (global). Called ONLY from `GenerateCommit`. The resulting
  `generate.Deps{Manifest: m}` is passed to `CommitStaged`/`runPipeline`.
- **Multi path**: `decompose.ResolveRoles(cfg, reg)` resolves a per-role manifest set
  (`decompose.RoleManifests`). Each role gets its own manifest.
- **buildDeps is single-path-only**: confirmed by grep — the only production caller is
  `GenerateCommit` at `stagecoach.go:136`. `Decompose` (multi) uses `ResolveRoles` instead.

### Role Resolution (FR-R3)
- `config.ResolveRoleModel(role, cfg) (provider, model, reasoning)` — precedence:
  `cfg.Roles[role]` (per-field) → global `cfg.Provider/Model/Reasoning` → `defaultRoleReasoning[role]`.
- **Single path bug (Issue 2)**: `CommitStaged` (generate.go:196) and `runPipeline`
  (stagecoach.go:467) pass `cfg.Model`/`cfg.Reasoning` directly to `Render`, bypassing
  `ResolveRoleModel("message")`. The multi path resolves correctly (message.go:103).

### Reasoning Feature (FR-R6)
- `Manifest.Render` guard (`render.go:126`): `if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0`.
- Config resolves `planner = "high"` by default (`defaultRoleReasoning`). The `reasoning`
  value reaches `Render` on all paths. But **all 8 built-in providers ship
  `ReasoningLevels = nil`**, so the guard is always false → the feature is inert.

### Freeze-Based Decompose (FR-M1)
- `FreezeWorkingTree(baseTree)` = `AddAll` → `WriteTree` (=tStart) → `ReadTree(baseTree)`.
  The final `ReadTree(baseTree)` resets the index to baseTree for the per-concept stager.
- `runOneFileShortcut` commits tStart directly, then re-syncs index via `ReadTree(tStart)`.
- **`runSingleShortcut` bug (Issue 3)**: commits tStart via `publishCommit` but does NOT
  call `ReadTree(tStart)` → index stays at baseTree ≠ HEAD.tree(tStart) → dirty `git status`.
