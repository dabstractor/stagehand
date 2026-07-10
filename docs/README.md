# Stagecoach documentation

Stagecoach writes your commit messages using the AI agent you already have installed. It auto-detects pi, Claude Code, opencode, Codex, Cursor, agy, or qwen-code, snapshots your staged changes atomically via git plumbing, and commits only what was staged when it started — so you can keep staging while it thinks.

See the [README](../README.md) for the quick start, feature overview, and FAQ.

> [!NOTE]
> The `docs/` directory tracks the shipped binary. If anything here disagrees with `stagecoach --help`, the binary is authoritative — open an issue.

## Install

```bash
# Homebrew (macOS / Linuxbrew)
brew install dustin/tap/stagecoach

# Go install (anywhere with Go)
go install github.com/dustin/stagecoach/cmd/stagecoach@latest

# Direct binary (curl|sh one-liner from GitHub Releases)
curl -fsSL https://github.com/dustin/stagecoach/raw/main/install.sh | bash

# Windows (Scoop)
scoop install dustin/stagecoach
```

> [!NOTE]
> The `install.sh` script is published with the first release. Until then, use Homebrew, `go install`, or Scoop. See the README for details.

## Documentation index

| Page | Description |
|------|-------------|
| [CLI reference](cli.md) | Synopsis, all global flags (incl. decompose + per-role), subcommands, exit codes, examples, and the flag↔env↔git-config map. **v2.1 additions:** hook (install/uninstall/status/exec), integrate (git-alias/lazygit + no-mangle protocol), models, and the global flags `--exclude`, `--format`, `--locale`, `--context`, `--template`, `--edit`, `--push`. |
| [Configuration](configuration.md) | 7-layer precedence, config file format, environment variables, git-config keys, built-in defaults, and paths. **v2.1 additions:** exclusion globs + `.stagecoachignore`, `[generation]` shaping keys (format/locale/template), `STAGECOACH_PUSH`, and `config init --interactive` (guided setup). |
| [Provider manifests](providers.md) | 22-field manifest schema, command rendering, the 7 built-in providers (incl. agy and qwen-code), and adding a new agent. |
| [How Stagecoach works](how-it-works.md) | Snapshot-based architecture, multi-commit decomposition pipeline, stage-while-generating, the safety and rescue protocol, binary filtering, and prompt engineering. **v2.1 additions:** payload exclusions, format modes & locale, the hook-vs-snapshot trade-off (FR-H7), and stage-while-editing (`--edit`). **Lock reclamation (FR-K1–K7):** the parent-death watchdog, `SIGHUP`, `lock status`, and the `no_parent_watchdog` opt-out. |

## Capability index

Each v2.1 capability maps to a specific doc anchor:

- **Payload exclusions** → [configuration.md#exclusion-globs-generationexclude](configuration.md#exclusion-globs-generationexclude) · [how-it-works.md#payload-exclusions-stagecoachignore](how-it-works.md#payload-exclusions-stagecoachignore)
- **Message shaping** → [how-it-works.md#format-modes-and-locale](how-it-works.md#format-modes-and-locale)
- **Git hook mode** → [how-it-works.md#trade-off-inversion-fr-h7](how-it-works.md#trade-off-inversion-fr-h7) · [cli.md#hook-install](cli.md#hook-install)
- **Tool integrations** → [cli.md#integrate-install-target](cli.md#integrate-install-target)
- **`--edit` / `--push`** → [cli.md](cli.md) (global flags)
- **Discovery** → [cli.md#models-provider](cli.md#models-provider) · [cli.md#config-init](cli.md#config-init)
- **Concurrency & lock reclamation** → [how-it-works.md#per-repo-run-lock-fr52](how-it-works.md#per-repo-run-lock-fr52) · [cli.md#lock-status](cli.md#lock-status) · [configuration.md#environment-variables](configuration.md#environment-variables) (`no_parent_watchdog`)

## Product specification

The [PRD](../PRD.md) is the authoritative product and technical specification (read-only). These docs are derived from it and from the shipped binary — refer to the PRD for the canonical requirements and design rationale.

The [FUTURE_SPEC.md](../FUTURE_SPEC.md) lists deferred and rejected ideas — each with its reason.

## Contributing

See the [README](../README.md#contributing) for build instructions. For the manifest format, see [providers/*.toml](../providers/) in the repo root — `providers/pi.toml` is the cleanest template.
