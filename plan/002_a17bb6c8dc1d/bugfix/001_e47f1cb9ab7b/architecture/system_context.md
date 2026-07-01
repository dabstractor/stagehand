# System Context — Stagehand v2.0 QA Bug Fixes

## Project Overview

Stagehand is a Go-based CLI tool (module `github.com/dustin/stagehand`, Go 1.22) that generates
AI-assisted git commit messages. It has two pipelines:

1. **Single-commit path** (`internal/generate/generate.go` → `CommitStaged`): snapshot the staged
   index into a frozen tree, generate a message via an agent subprocess, build the commit via git
   plumbing (`commit-tree` + `update-ref` CAS), and print the FR42 success report.

2. **Multi-commit decomposition** (`internal/decompose/decompose.go` → `Decompose`): for an
   un-staged dirty tree, run a four-agent pipeline (planner → stager → message → arbiter) to
   produce N logically-coherent commits with 1-deep-overlapped staging/generation and serialized
   CAS publication.

## Architecture Layers (top-down)

| Layer | Package | Responsibility |
|-------|---------|----------------|
| CLI | `internal/cmd` (root.go, config.go, default_action.go, providers.go) | cobra command tree, flag parsing, config.Load, exit-code mapping, FR42 report printing |
| Config | `internal/config` (load.go, file.go, config.go, roles.go, role_defaults.go, bootstrap.go, git.go) | 7-layer config resolution, TOML parsing, bootstrap generation, role-model resolution |
| Generate | `internal/generate` (generate.go, dedupe.go, rescue.go) | Single-commit orchestrator (snapshot → generate → CAS) |
| Decompose | `internal/decompose` (decompose.go, roles.go, planner.go, stager.go, message.go, arbiter.go, chain.go) | Multi-commit orchestrator (planner → stager → message → arbiter) |
| Provider | `internal/provider` (builtin.go, manifest.go, merge.go, render.go, registry.go, executor.go, parse.go) | Provider manifests, command rendering, subprocess execution, output parsing |
| Git | `internal/git` (git.go, binary.go) | Git plumbing wrapper (rev-parse, write-tree, commit-tree, update-ref CAS, diff-tree) |
| Prompt | `internal/prompt` (system.go, payload.go, planner.go, stager.go, arbiter.go) | System-prompt and payload builders |
| Signal | `internal/signal` | Signal handler for Ctrl-C rescue |
| UI | `internal/ui` | Progress display, verbose logging |
| ExitCode | `internal/exitcode` | Exit-code mapping |
| Stubtest | `internal/stubtest` | Test-only fake agent binary + Manifest factory |
| Public API | `pkg/stagehand` | (Not yet shipped — P4.M2) |

## Key Architectural Patterns

### Provider Manifest Resolution Flow
1. `config.Load` resolves `cfg.Provider` (the manifest/agent name, e.g. "pi") + `cfg.Model`.
2. `provider.NewRegistry(overrides)` merges user `[provider.X]` overrides onto built-in manifests
   (`internal/provider/builtin.go`) via `MergeManifest`.
3. `registry.Get(providerName)` returns the merged-but-unresolved manifest.
4. `manifest.Render(model, provider, sysPrompt, payload, mode...)` renders a `CmdSpec` — **this is
   where Issue 1 lives**: the `provider` param is treated as the sub-provider, but callers pass the
   manifest name.

### Render's Provider Fallback (CRITICAL for Issue 1)
In `render.go`, `Render`'s `provider` param defaults to `*r.DefaultProvider` when empty:
```go
providerToUse := provider
if providerToUse == "" {
    providerToUse = *r.DefaultProvider
}
```
The fix for Issue 1: callers should pass `""` so this fallback fires and uses the merged
`DefaultProvider` (the sub-provider, e.g. "openrouter"), NOT the manifest name (e.g. "pi").

### Config Layer Resolution (`internal/config/load.go`)
- Layer order: built-in Defaults → global TOML → repo-local TOML → repo git config → env vars → CLI flags
- `ConfigPathOverride` (--config flag) → `STAGEHAND_CONFIG` env → `globalConfigPath()` discovery
- FR37a preserves `default_provider` across merge layers into the manifest's `DefaultProvider` field

### Decompose Arbiter Resolution (`internal/decompose/chain.go`)
After the per-concept loop, if the working tree has leftovers (`StatusPorcelain != ""`):
1. `runArbiter` decides: `null` (new commit) or `&sha` (amend target).
2. `resolveArbiter` executes: `resolveNewCommit` (AddAll→WriteTree→generateMessage→CAS),
   `resolveTipAmend` (AddAll→WriteTree→CommitTree→CAS), or `resolveMidChain` (chain rebuild).
3. SHAs change after amend/rebuild; a new commit is added on the null path.

### Test Conventions
- Tests use **real git** (`git.New(repo)` against temp repos) — NO mock Git interface implementations.
- Agent calls use `internal/stubtest` (`stubtest.Build(t)` compiles `cmd/stubagent`; `stubtest.Manifest(bin, opts)`
  creates a fake manifest; `stubtest.NewScript(t, bin, []string)` creates a multi-response script).
- Tests are co-located (`foo_test.go` next to `foo.go`), follow table-driven patterns, and use
  `t.Setenv` / `t.TempDir` for isolation.
- `go test ./...` is green; `go build ./...` is green.

## Issue Summary

| # | Severity | Title | Key Files |
|---|----------|-------|-----------|
| 1 | Critical | Provider/sub-provider conflation | `generate.go`, `decompose/{planner,stager,message,arbiter}.go`, `render.go` |
| 2 | Major | Stager toolset not actually scoped | `provider/builtin.go`, `providers/{pi,claude}.toml`, `decompose/stager.go` |
| 3 | Major | Post-arbiter stale SHAs / missing commits | `decompose/decompose.go`, `cmd/default_action.go`, `git/git.go` |
| 4 | Major | Config subcommands ignore --config override | `cmd/config.go`, `config/load.go`, `config/file.go` |
| 5 | Minor | Bootstrap config not functional for pi | `config/bootstrap.go`, `config/role_defaults.go` |

## Dependency Chain

- **Issue 1 (Critical)** is foundational — it breaks the default provider (pi) entirely.
- **Issue 5 (Minor)** is latent and only surfaces **after Issue 1 is fixed** (currently masked by
  the bogus `--provider pi` erroring first). Issue 5 depends on Issue 1.
- **Issues 2, 3, 4** are independent of each other and of Issue 1.
- All implementing subtasks are dependencies of the final documentation-sync task.
