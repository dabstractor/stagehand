# System Context — Stagehand V2.0 Delta

## Current State (V1 — Fully Implemented)

The Stagehand v1 single-commit core is **fully implemented and tested** across these packages:

| Package | Location | Purpose |
|---|---|---|
| `internal/git` | `git.go` | Git plumbing wrapper (`Git` interface, `gitRunner` impl): RevParseHEAD, WriteTree, CommitTree, UpdateRefCAS, DiffTree, StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll, StagedFileCount |
| `internal/provider` | `manifest.go`, `render.go`, `executor.go`, `parse.go`, `registry.go`, `builtin.go`, `merge.go` | Provider manifest system: Manifest struct (pointer scalars), Render→CmdSpec, Execute (subprocess with process-group kill), ParseOutput (raw/json pipeline), Registry (name→manifest), 6 built-in providers |
| `internal/config` | `config.go`, `file.go`, `git.go`, `load.go` | 7-layer config precedence: defaults → global TOML → repo TOML → git config → env → CLI flags |
| `internal/generate` | `generate.go`, `dedupe.go`, `rescue.go` | CommitStaged orchestrator: snapshot→generate→parse→dedupe→commit-tree→update-ref CAS |
| `internal/prompt` | `system.go`, `payload.go` | System prompt (style learning, anti-reuse) + user payload construction |
| `internal/cmd` | `root.go`, `default_action.go`, `providers.go`, `config.go` | Cobra CLI: root flags, default action, providers list/show, config init/path |
| `internal/signal` | `signal.go`, `*_unix.go`, `*_windows.go` | SIGINT/SIGTERM handling + rescue path |
| `internal/ui` | `output.go`, `verbose.go` | Progress messages, color, verbose diagnostics |
| `internal/exitcode` | `exitcode.go` | Canonical exit codes (0/1/2/3/124) |
| `internal/stubtest` | `stubtest.go` | Fake agent for integration tests (cmd/stubagent binary) |
| `pkg/stagehand` | `stagehand.go` | Public API: `GenerateCommit(ctx, Options) (Result, error)` |
| `cmd/stubagent` | `main.go` | Test-only stub agent binary |
| `cmd/stagehand` | `main.go` | CLI entrypoint |

## V2 Delta — What Must Be Added/Changed

### 1. Provider System (Manifest Extensions)
- **`Manifest` struct**: Add `TooledFlags []string` and `Experimental *bool` fields
- **`MergeManifest`**: Add TooledFlags to the replace-wholesale regime (like BareFlags)
- **`Render`**: Add `mode` parameter ("bare"|"tooled") to select BareFlags vs TooledFlags
- **`builtin.go`**: Add `agy` provider; update pi defaults (remove glm-5-turbo); update preferredBuiltins order
- **`providers/agy.toml`**: New reference manifest file

### 2. Config System (Per-Role + Bootstrap)
- **`Config` struct**: Add `Roles` (map of role→provider/model), `ConfigVersion`, `MaxCommits`, `BinaryExtensions`
- **`file.go`**: Add `[role.<role>]` decode structs, `config_version`, `max_commits`, `binary_extensions`
- **`load.go`**: Add per-role env vars, per-role flags
- **Role resolution**: `ResolveRoleModel(role, cfg) → (provider, model)` with 5-layer precedence
- **Config init**: Rewrite to populated bootstrap (FR-B1/B2)
- **Config upgrade**: New command (FR-B5)

### 3. Git Plumbing V2 (New Methods)
- `RevParseTree(ctx)` — `git rev-parse HEAD^{tree}` (base tree for concept diffs)
- `TreeDiff(ctx, treeA, treeB)` — tree-to-tree diff (concept diffs, never index-vs-HEAD)
- `ReadTree(ctx, tree)` — `git read-tree` (for chain rebuild in arbiter)
- `StatusPorcelain(ctx)` — `git status --porcelain` (arbiter trigger)
- `WorkingTreeDiff(ctx, opts)` — unstaged diff (planner input)

### 4. Binary/Non-Text Filtering (FR3a-c)
- `internal/git/binary.go`: detectNonText via numstat + extension denylist; placeholder line
- Integrate into StagedDiff, WorkingTreeDiff, TreeDiff

### 5. Decomposition Pipeline (`internal/decompose/`)
- `roles.go` — resolve all four agent roles
- `planner.go` — planner agent call + JSON parse/retry
- `stager.go` — tooled stager agent + snapshot scheduling
- `arbiter.go` — arbiter agent + amend/new/rebuild resolution
- `chain.go` — linear-chain rebuild for mid-chain amend
- `decompose.go` — orchestrator

### 6. Decomposition Prompts (`internal/prompt/`)
- `planner.go` — planner system prompt + JSON contract (§17.5)
- `stager.go` — stager task prompt (§17.6)
- `arbiter.go` — arbiter prompt + JSON contract (§17.7)

### 7. CLI Integration
- New flags: `--commits`, `--single`/`--no-decompose`, `--max-commits`, per-role flags
- Default action routing: decompose when nothing staged
- Config init → populated bootstrap; config upgrade command

### 8. Public API
- `DecomposeOptions`, `DecomposeResult`, `RoleModel` types
- `Decompose()` function

## Key Architectural Patterns (Must Follow)

### Manifest Pointer-Scalar Design
The Manifest struct uses `*string`/`*bool` for optional fields. A nil pointer = "absent" (inherit base on merge); a non-nil pointer (even `*""` or `*false`) = "explicit override." This is the ONLY way go-toml/v2 can distinguish absent from explicitly-empty. New fields (`TooledFlags` as `[]string`, `Experimental` as `*bool`) must follow this convention.

### Git Interface Pattern
All git operations go through the `Git` interface in `internal/git/git.go`. Methods are added to the interface AND the `gitRunner` struct. Tests use real git binaries in temp directories. The `run()` helper execs git with `[]string` args (never `sh -c`), targets repo via `-C` flag, and returns `(stdout, stderr, exitCode, err)` where non-zero exits have `err==nil`.

### Config Overlay Pattern
Config resolution uses `overlay(dst, src *Config)` which copies non-zero scalars from src→dst. Provider blocks merge field-by-field (FR37a). Per-role config must follow the same pattern.

### Render Signature
Current: `Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error)`
V2 must add mode: `Render(model, provider, sysPrompt, userPayload string, mode ...string) (*CmdSpec, error)` or explicit parameter. All existing callers pass bare mode.

### Subprocess Execution
`provider.Execute(ctx, spec, timeout, verbose)` handles process-group creation, signal forwarding, timeout via context, and stdout/stderr capture. Decompose roles reuse this — the planner/message/arbiter use bare mode, the stager uses tooled mode. Both produce CmdSpec via Render.

## Dependency Graph

```
P1 (Manifest + Config V2) ─────┐
                               │
P2 (Binary Filter + Git V2) ──┼──▶ P3 (Decompose) ──▶ P4 (CLI/API) ──▶ P5 (Docs)
                               │
P1.M3 (Per-Role Config) ──────┘
```

P1 and P2 are independent and can proceed in parallel. P3 depends on both. P4 depends on P3. P5 is the final sweep.
