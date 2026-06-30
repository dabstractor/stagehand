# Stagehand documentation

Stagehand writes your commit messages using the AI agent you already have installed. It auto-detects pi, Claude Code, Gemini CLI, opencode, Codex, or Cursor, snapshots your staged changes atomically via git plumbing, and commits only what was staged when it started — so you can keep staging while it thinks.

See the [README](../README.md) for the quick start, feature overview, and FAQ.

> [!NOTE]
> The `docs/` directory is new in v1.0. It ships the full reference that the README's "see docs/" link promises. If anything here disagrees with `stagehand --help`, the binary is authoritative — open an issue.

## Install

```bash
# Homebrew (macOS / Linuxbrew)
brew install dustin/tap/stagehand

# Go install (anywhere with Go)
go install github.com/dustin/stagehand/cmd/stagehand@latest

# Direct binary (curl|sh one-liner from GitHub Releases)
curl -fsSL https://github.com/dustin/stagehand/raw/main/install.sh | bash

# Windows (Scoop)
scoop install dustin/stagehand
```

> [!NOTE]
> The `install.sh` script is published with the first release. Until then, use Homebrew, `go install`, or Scoop. See the README for details.

## Documentation index

| Page | Description |
|------|-------------|
| [CLI reference](cli.md) | Synopsis, all 11 global flags, subcommands, exit codes, examples, and the flag↔env↔git-config map. |
| [Configuration](configuration.md) | 7-layer precedence, config file format, environment variables, git-config keys, built-in defaults, and paths. |
| [Provider manifests](providers.md) | 18-field manifest schema, command rendering, the 6 built-in providers, and adding a new agent. |
| [How Stagehand works](how-it-works.md) | Snapshot-based architecture, stage-while-generating, the safety and rescue protocol, and prompt engineering. |

## Product specification

The [PRD](../PRD.md) is the authoritative product and technical specification (read-only). These docs are derived from it and from the shipped binary — refer to the PRD for the canonical requirements and design rationale.

## Contributing

See the [README](../README.md#contributing) for build instructions. For the manifest format, see [providers/*.toml](../providers/) in the repo root — `providers/pi.toml` is the cleanest template.
