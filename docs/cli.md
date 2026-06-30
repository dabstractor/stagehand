# CLI reference

Full reference for the `stagehand` command, its flags, subcommands, exit codes, and examples. Matches the shipped binary (`stagehand --help`) and the Go source in `internal/cmd/`.

## Synopsis

```text
stagehand [flags]
stagehand <command> [flags]
```

With no subcommand, `stagehand` runs the **default action**: it snapshots your staged changes, generates a commit message using the configured AI agent, and commits atomically. If nothing is staged and auto-stage is on (the default), it runs `git add -A` first. See [how-it-works.md](how-it-works.md) for the snapshot architecture.

## Global flags

| Flag | Type | Default | Env var | Git config | Description |
|------|------|---------|---------|------------|-------------|
| `--provider <name>` | string | "" (auto-detect) | `STAGEHAND_PROVIDER` | `stagehand.provider` | Provider/agent to use |
| `--model <name>` | string | "" (manifest default) | `STAGEHAND_MODEL` | `stagehand.model` | Model override |
| `--config <path>` | string | "" | `STAGEHAND_CONFIG` | — | Path to a config file, overrides discovery |
| `--timeout <dur>` | string | "120s" | `STAGEHAND_TIMEOUT` | `stagehand.timeout` | Generation timeout (e.g. `"120s"` or `120`) |
| `--verbose`, `-v` | bool | false | `STAGEHAND_VERBOSE` | — | Print resolved command, raw output, retries |
| `--no-color` | bool | TTY-aware | `STAGEHAND_NO_COLOR` | — | Disable color (also honors `NO_COLOR`) |
| `--all`, `-a` | bool | false | — | — | Run `git add -A` before snapshotting, even if something is staged |
| `--no-auto-stage` | bool | false | — | — | If nothing is staged, exit instead of auto-staging |
| `--dry-run` | bool | false | — | — | Generate and print the message; do not commit |
| `--version` | — | — | — | — | Print the build version (`"dev"` for a local build; the release tag for a released binary) |
| `--help`, `-h` | — | — | — | — | Print help |

The `--config` flag is a path override for config-file discovery — it is not itself a `Config` field. The behavioral flags (`--all`, `--no-auto-stage`, `--dry-run`) have no env-var or git-config analogs.

## Subcommands

### `providers list`

List all known providers with detection status:

```text
NAME      DETECTED  DEFAULT
claude    ✓
codex     ✓
cursor    ✓
gemini    ✓
opencode  ✓
pi        ✓         (default)
```

`✓` = the provider's command is found on `$PATH`. `(default)` marks the provider selected by auto-detection (first installed built-in in preference order: pi, claude, gemini, opencode, codex, cursor).

### `providers show <name>`

Print the fully-merged manifest for a provider as TOML. Exits 1 if the provider is unknown:

```bash
stagehand providers show pi
```

### `config init`

Write a fully-commented example config to the global config path. Creates parent directories as needed. **Refuses to overwrite** an existing file (exit 1) — delete it first to regenerate:

```bash
stagehand config init
# Wrote example config to ~/.config/stagehand/config.toml
```

### `config path`

Print the resolved global config path:

```bash
stagehand config path
# ~/.config/stagehand/config.toml
```

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success (commit created, or dry-run message printed). |
| `1` | General error (generation failed, parse failed after retries, agent missing, CAS, usage). |
| `2` | Nothing to commit (clean tree after auto-stage, or nothing staged with `--no-auto-stage`). |
| `3` | Rescue condition (snapshot taken, commit not created — manual recovery printed). |
| `124` | Timeout (generation exceeded `--timeout`). |

Exit codes mirror the constants in `internal/exitcode/exitcode.go`. A timeout is reported as `124` (matching GNU `timeout`), not `3`.

## Flag ↔ env ↔ git-config map

Config-backed flags can also be set via environment variables or git-config keys. This table shows the mapping (highest to lowest precedence: CLI flag > env var > git config):

| Flag | Env var | Git config key |
|------|---------|----------------|
| `--provider` | `STAGEHAND_PROVIDER` | `stagehand.provider` |
| `--model` | `STAGEHAND_MODEL` | `stagehand.model` |
| `--timeout` | `STAGEHAND_TIMEOUT` | `stagehand.timeout` |
| `--config` | `STAGEHAND_CONFIG` | — |
| `--verbose` | `STAGEHAND_VERBOSE` | — |
| `--no-color` | `STAGEHAND_NO_COLOR` (also honors `NO_COLOR`) | — |
| `--all` | — | — |
| `--no-auto-stage` | — | — |
| `--dry-run` | — | — |

## Examples

```bash
# Happy path — stage, generate, commit
git add feature/login.js
stagehand
# [abc1234] feat: add login flow
# M  src/login.js

# Use a specific provider and model
stagehand --provider claude --model sonnet

# Persist provider choice per-repo with git config
git config stagehand.provider pi

# Preview the message without committing (exit 0)
stagehand --dry-run

# Force staging everything (including untracked)
stagehand -a

# Pipe the dry-run message
stagehand --dry-run --no-color | tee /tmp/msg.txt

# See what command is being run
stagehand --verbose
```
