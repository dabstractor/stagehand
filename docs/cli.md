# CLI reference

Full reference for the `stagehand` command, its flags, subcommands, exit codes, and examples. Matches the shipped binary (`stagehand --help`) and the Go source in `internal/cmd/`.

## Synopsis

```text
stagehand [flags]
stagehand <command> [flags]
```

With no subcommand, `stagehand` runs the **default action**. The routing depends on repo state:

- **Something staged** → single-commit path (snapshot the staged diff, generate, commit).
- **Nothing staged, dirty tree, auto-stage on (default), not opted out** → **multi-commit decomposition** (planner → stager → message → arbiter pipeline; see [how-it-works.md](how-it-works.md#multi-commit-decomposition)).
- **Nothing staged, clean tree** → exit 2 "Nothing to commit."
- `--single` / `--no-decompose` / `--commits 1` → force the single-commit path.
- `--dry-run` → force the single-commit preview (decompose commits, so dry-run honors the single preview).

## Global flags

| Flag | Type | Default | Env var | Git config | Description |
|------|------|---------|---------|------------|-------------|
| `--provider <name>` | string | "" (auto-detect) | `STAGEHAND_PROVIDER` | `stagehand.provider` | Provider/agent to use |
| `--model <name>` | string | "" (manifest default) | `STAGEHAND_MODEL` | `stagehand.model` | Model override |
| `--config <path>` | string | "" | `STAGEHAND_CONFIG` | — | Path to a config file, overrides discovery. A path pointing at a **missing** file fails fast with exit 1 (like a malformed or directory path), rather than falling back to discovery. |
| `--timeout <dur>` | string | "120s" | `STAGEHAND_TIMEOUT` | `stagehand.timeout` | Generation timeout (e.g. `"120s"` or `120`) |
| `--verbose`, `-v` | bool | false | `STAGEHAND_VERBOSE` | — | Print resolved command, raw output, retries |
| `--no-color` | bool | TTY-aware | `STAGEHAND_NO_COLOR` | — | Disable color (also honors `NO_COLOR`) |
| `--all`, `-a` | bool | false | — | — | Run `git add -A` before snapshotting, even if something is staged |
| `--no-auto-stage` | bool | false | — | — | If nothing is staged, exit instead of auto-staging |
| `--dry-run` | bool | false | — | — | Run the full snapshot→generate→parse→duplicate-check pipeline (same as a real commit, including the write-tree snapshot and retry) and print the message; do not commit. If generation fails (timeout or parse/duplicate-check exhaustion), exits **1** with a short stderr message instead of exit 3/124 + the full recovery recipe (since no commit was ever intended) |
| `--commits <N>` | int | 0 (auto) | `STAGEHAND_COMMITS` | — | Force exactly N commits when nothing is staged (0 = auto-decompose; ≥2 = force N; 1 ≡ `--single`) |
| `--single` | bool | false | — | — | Bypass decomposition; force the single-commit auto-stage-all behavior (alias: `--no-decompose`) |
| `--no-decompose` | bool | false | — | — | Alias for `--single` |
| `--max-commits <N>` | int | 12 | — | — | Safety cap on auto-decompose commit count (also `[generation].max_commits` in config) |
| `--planner-provider <name>` | string | "" | `STAGEHAND_PLANNER_PROVIDER` | — | Per-role provider override for the decomposition planner |
| `--planner-model <name>` | string | "" | `STAGEHAND_PLANNER_MODEL` | — | Per-role model override for the decomposition planner |
| `--stager-provider <name>` | string | "" | `STAGEHAND_STAGER_PROVIDER` | — | Per-role provider override for the (tooled) staging agent |
| `--stager-model <name>` | string | "" | `STAGEHAND_STAGER_MODEL` | — | Per-role model override for the (tooled) staging agent |
| `--arbiter-provider <name>` | string | "" | `STAGEHAND_ARBITER_PROVIDER` | — | Per-role provider override for the leftover arbiter |
| `--arbiter-model <name>` | string | "" | `STAGEHAND_ARBITER_MODEL` | — | Per-role model override for the leftover arbiter |
| `--version` | — | — | — | — | Print the build version (`"dev"` for a local build; the release tag for a released binary) |
| `--help`, `-h` | — | — | — | — | Print help |

The `--config` flag is a path override for config-file discovery — it is not itself a `Config` field. An explicit `--config` (or `STAGEHAND_CONFIG`) pointing at a missing file errors with `config: config file not found: <path>` (exit 1) instead of silently falling back to provider auto-detection. Only the discovery default (no `--config` or `STAGEHAND_CONFIG`) tolerates a missing global file. The behavioral flags (`--all`, `--no-auto-stage`, `--dry-run`) have no env-var or git-config analogs. `--config` is honored by every command — including the default commit action, so a user-defined provider declared under `[provider.<name>]` in that file is usable with `--provider <name>` on `stagehand` directly (not just the `providers`/`config` subcommands).

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

`✓` = the provider's command is found on `$PATH`. `(default)` marks the provider selected by auto-detection (first installed built-in in preference order: pi, opencode, cursor, agy, gemini, codex, claude).

### `providers show <name>`

Print the fully-merged manifest for a provider as TOML. Exits 1 if the provider is unknown:

```bash
stagehand providers show pi
```

### `config init`

Bootstrap a **populated, working config** to the global config path. Auto-detects the highest-priority installed built-in agent (order: pi, opencode, cursor, agy, gemini, codex, claude) and writes `config_version = 2`, `[defaults] provider = "<detected>"`, and that provider's per-role model defaults UNCOMMENTED so the tool works immediately. Other installed providers appear as commented-out `[role.*]` blocks. If no agent is detected, defaults to `"pi"`. Creates parent directories as needed. **Refuses to overwrite** an existing file (exit 1) unless `--force` is passed:

```bash
stagehand config init
# Wrote config to ~/.config/stagehand/config.toml

# Target a specific provider:
stagehand config init --provider claude

# Overwrite existing config:
stagehand config init --force

# Write the inert all-commented reference (v1 behavior):
stagehand config init --template
```

| Flag | Description |
|------|-------------|
| `--provider <name>` | Target a specific built-in provider instead of auto-detecting |
| `--force` | Overwrite an existing config file |
| `--template` | Write the inert all-commented reference config (v1 behavior) |

### `config upgrade`

Upgrade an existing config's `config_version` to the current schema version (2) in place. Every line except the top-level `config_version` is preserved byte-for-byte. Idempotent — running it twice leaves the file unchanged. No flags.

```bash
stagehand config upgrade
# Already at version 2 →  "Config at ~/.config/stagehand/config.toml is already at version 2 (no changes)."
# Upgraded from v1  →  "Upgraded config at ~/.config/stagehand/config.toml to version 2."
# No file          →  "no config file at <path> (run 'stagehand config init' first)"  (exit 1)
```

At load time, a missing or outdated `config_version` triggers an advisory pointing at `config upgrade` or `config init --force`.

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
| `1` | General error (generation failed, parse failed after retries, **provider command missing on `$PATH` (checked before the snapshot)**, CAS, usage). |
| `2` | Nothing to commit (clean tree after auto-stage, or nothing staged with `--no-auto-stage`). |
| `3` | Rescue condition (snapshot taken, commit not created — manual recovery printed). |
| `124` | Timeout (generation exceeded `--timeout`). |

Exit codes mirror the constants in `internal/exitcode/exitcode.go`. A timeout is reported as `124` (matching GNU `timeout`), not `3`. With `--dry-run`, generation failures (timeout or parse/duplicate-check exhaustion) report exit **1** with a short stderr message (not 3/124 + the recovery recipe) — codes 3 and 124 remain the non-dry-run (commit-path) semantics.

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
| `--commits` | `STAGEHAND_COMMITS` | — |
| `--single` | — | — |
| `--no-decompose` | — | — |
| `--max-commits` | — | — (also `[generation].max_commits` in config) |
| `--planner-provider` | `STAGEHAND_PLANNER_PROVIDER` | — |
| `--planner-model` | `STAGEHAND_PLANNER_MODEL` | — |
| `--stager-provider` | `STAGEHAND_STAGER_PROVIDER` | — |
| `--stager-model` | `STAGEHAND_STAGER_MODEL` | — |
| `--arbiter-provider` | `STAGEHAND_ARBITER_PROVIDER` | — |
| `--arbiter-model` | `STAGEHAND_ARBITER_MODEL` | — |

> [!NOTE]
> The **message role** has no CLI flag (`--message-provider`/`--message-model` do not exist). It is reachable via `STAGEHAND_MESSAGE_PROVIDER`/`STAGEHAND_MESSAGE_MODEL` env vars and the `[role.message]` config block only. When unset, it inherits the global `--provider`/`--model`.

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

# Multi-commit decomposition — auto-split a dirty tree
stagehand
# Decomposes into N logically-coherent commits automatically

# Force exactly 3 commits
stagehand --commits 3

# Keep v1 single-commit behavior
stagehand --single

# Route planning to a bigger model
stagehand --planner-provider claude --planner-model opus

# Per-repo per-role config (.stagehand.toml)
# [role.planner]
# provider = "claude"
# model = "opus"
```
