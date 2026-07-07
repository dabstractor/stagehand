# System Context: stagehand → stagecoach Rename

## Project State

**Repository:** `git@github.com:dabstractor/stagecoach` (GitHub remote already renamed to stagecoach)
**Working directory:** `/home/dustin/projects/stagehand` (local dir still named stagehand)
**Module path:** `github.com/dustin/stagehand` (go.mod — MUST be renamed)
**Codebase maturity:** Fully implemented, 63,008 lines of Go across ~200 files. The v1 single-commit core (§9, §13.1–§13.5) and all v2.x features (decomposition, per-role models, hooks, multi-turn fallback, work-description mode, closed-loop token gate, etc.) are implemented with comprehensive test coverage.

## What Exists

The project is a complete Go CLI tool that generates git commit messages by shelling out to AI coding agents (pi, Claude Code, Gemini CLI, etc.). It uses git plumbing (write-tree, commit-tree, update-ref CAS) for atomic, snapshot-based commits.

### Package Layout (current — all under `github.com/dustin/stagehand/`)

| Package | Purpose | Rename surface |
|---------|---------|---------------|
| `cmd/stagehand/` | Main entrypoint binary | Directory + file rename |
| `cmd/stubagent/` | Test-only stub agent binary | Import path only |
| `pkg/stagehand/` | Public library API | Directory + file + package rename |
| `internal/cmd/` | CLI commands (cobra) | String literals, identifiers |
| `internal/config/` | Config loading, precedence, bootstrap | String literals (env vars, git config keys, paths) |
| `internal/provider/` | Provider manifests, executor, parser | Import paths, string literals |
| `internal/git/` | Git plumbing wrapper | Import paths |
| `internal/generate/` | Single-commit orchestrator | String literals (error messages) |
| `internal/decompose/` | Multi-commit pipeline | Import paths |
| `internal/hook/` | Git hook mode | String literals (marker, script) |
| `internal/hooks/` | Hook execution on commit path | Import paths |
| `internal/integrate/` | Tool integrations | Import paths |
| `internal/lock/` | Per-repo run lock | String literals (lock dir path) |
| `internal/exclude/` | .stagehandignore loader | String literals + identifier |
| `internal/prompt/` | Prompt construction | Import paths |
| `internal/signal/` | Signal handling | Import paths |
| `internal/ui/` | Output, color, verbosity | Import paths |
| `internal/exitcode/` | Exit codes | String literals |
| `internal/stubtest/` | Test stub helpers | String literals + import paths |
| `internal/e2e/` | End-to-end test harness | String literals + import paths |

### Non-Go Files Needing Rename

- `Makefile` — binary names, build targets, install paths
- `.goreleaser.yaml` — project name, build IDs, formula/scoop/AUR names, URLs
- `.github/workflows/ci.yml` — module paths in coverage gates
- `README.md` — title, hero text, badges, install instructions, examples
- `docs/*.md` (5 files) — comprehensive documentation
- `providers/*.toml` (8 files) — reference manifests
- `.gitignore` — binary name, config file name
- `FUTURE_SPEC.md` — deferred/rejected features doc

## Key Architectural Constraint

The rename is **mechanical but pervasive**. There are no backward-compatibility concerns because:
1. The project has no public releases yet (goreleaser comment: "before the first REAL tag")
2. The git remote is already renamed on GitHub (`dabstractor/stagecoach`)
3. No external consumers import the module

The rename must be **complete and atomic** — no `stagehand` references should remain in any tracked file after completion.

## Dependencies (external)

| Dependency | Version | Impact of rename |
|-----------|---------|-----------------|
| `github.com/spf13/cobra` | v1.10.2 | None (external) |
| `github.com/spf13/pflag` | v1.0.10 | None (external) |
| `github.com/pelletier/go-toml/v2` | v2.4.2 | None (external) |
| `gopkg.in/yaml.v3` | v3.0.1 | None (external) |
| Go toolchain | 1.22+ | Module path must be valid |
| `git` binary | ≥2.20 | Shells out at runtime — unaffected |

No external dependency references "stagehand" — the rename is self-contained.
