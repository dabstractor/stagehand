# Stagehand

> **Stagehand writes your commit messages using the AI agent you already pay for.**
> No API key. No per-token billing. It shells out to Claude Code, Codex, Gemini CLI, pi, opencode, or Cursor — whatever you already have installed — and spends your existing coding-plan quota instead. Stage while it thinks; it commits only what was staged when it started, atomically, and can never corrupt your repo.

A snapshot-based AI commit message generator that uses YOUR local CLI agent.

<!-- TODO: add LICENSE file and badge -->

![CI](https://github.com/dustin/stagehand/actions/workflows/ci.yml/badge.svg)

## 30-second demo

<!-- TODO: record asciinema demo; for now see the snapshot workflow diagram below -->

> [!NOTE]
> A recorded walkthrough is coming soon. See the [snapshot workflow](#the-snapshot-workflow) below for what you'll see.

## Why not opencommit/aicommits?

The incumbent tools — opencommit, aicommits — own the HTTP call to the model, so they can normalize providers, handle retries, and abstract auth. Once you own the HTTP call, you cannot use a coding-plan subscription, because that subscription is not reachable over the public API.

Stagehand inverts the architecture: it shells out to your installed CLI agent, trading provider normalization for quota reuse — the agent brings its own auth and billing. That trade-off — give up control of the model call in exchange for access to the user's existing quota — is the entire product.

| | **opencommit / aicommits** | **Stagehand** |
|---|---|---|
| Auth | API key required | None — uses your agent's existing auth |
| Architecture | Owns the HTTP call | Shells out to your CLI agent |
| Billing | Per-token | Your existing coding-plan quota |
| Stage while generating | No | Yes (snapshot-based) |

## Install

**Prerequisite:** a coding-agent CLI already installed and on `$PATH` (pi, Claude Code, Gemini CLI, opencode, Codex, or Cursor).

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

> [!NOTE] The install.sh script is published with the first release. Until then, use Homebrew, go install, or Scoop.

## Quick start

```bash
# 1. Stage your changes
git add feature/login.js

# 2. Run stagehand — it snapshots, generates, and commits atomically
stagehand
# [abc1234] feat: add login flow
# M  src/login.js

# 3. Stage everything and commit in one step
stagehand -a

# 4. Preview the real message (full pipeline: snapshot→generate→parse→dedupe→retry), no commit
stagehand --dry-run
```

> [!NOTE]
> If generation fails, `--dry-run` exits 1 with a short message — not the full recovery recipe or exit 3/124 — since no commit was ever intended.

### lazygit binding

```yaml
# From lazygit config.yml:
#   customCommands:
#     - key: '<c-a>'
#       command: 'stagehand'
#       loadingText: 'Generating commit message…'
#       output: 'none'
```

## Configure your agent

Stagehand auto-detects which agents are installed and uses the first one it finds (in preference order: pi, claude, gemini, opencode, codex, cursor). To see what's detected:

```bash
$ stagehand providers list
NAME      DETECTED  DEFAULT
claude    ✓
codex     ✓
cursor    ✓
gemini    ✓
opencode  ✓
pi        ✓         (default)
```

> [!NOTE]
> A provider whose command isn't on `$PATH` fails fast with exit 1 before any snapshot — no partial state, no rescue recipe.

Set a per-repo default with git config:

```bash
git config stagehand.provider pi
# Optionally pin a model:
git config stagehand.model glm-5.2
```

Or write a fully-commented global config file:

```bash
stagehand config init
# Wrote example config to ~/.config/stagehand/config.toml

# See where that lives:
stagehand config path
# ~/.config/stagehand/config.toml
```

> [!NOTE]
> The template also documents a `[generation]` section: `output` ("raw"|"json") and `strip_code_fence` are an **opt-in override** for how Stagehand parses agent output. When unset, the per-provider `[provider.<name>]` value is used (defaulting to `raw` / `true`); set them under `[generation]` only to force the value across ALL providers.

Point discovery at a specific file with `stagehand --config path/to/config.toml`. It is honored by every command — including the default commit action — so a provider declared under `[provider.<name>]` there is usable with `--provider <name>` directly. The path must exist: an explicit `--config` (or `STAGEHAND_CONFIG`) pointing at a missing file fails fast with exit 1 rather than silently falling back to auto-detection.

**Config precedence** (highest → lowest): CLI flags > `STAGEHAND_*` env vars > repo `git config` (`stagehand.*`) > repo `.stagehand.toml` > global config file > provider defaults > built-in defaults.

## The snapshot workflow

Stagehand creates commits against a frozen snapshot of your index — not the live state. This means you can keep staging more files while the message generates, and they will **never** be included in the current in-flight commit.

```text
Pane A (lazygit / shell)        Pane B (shell)
─────────────────────────       ───────────────────────
git add feature/login.js
stagehand                     ┐
  ↳ snapshotting…             │  (user is free to work here)
  ↳ generating with pi…       │  git add docs/login.md
  ↳ (10s pass)                │  git add tests/login.test.js
  ↳ created abc1234           │  (these stay staged — NOT in abc1234)
                              ┘
                                stagehand        # next run commits these
```

Generation time is no longer dead time. The in-flight commit only ever contains what was staged when it started, so you can stage the next batch freely while the current message generates — and a failed generation leaves your repo byte-for-byte unchanged.

## Full CLI and config reference

The **authoritative, always-available** reference lives in the binary itself:

```bash
stagehand --help          # every flag, subcommand, and option
stagehand config init     # writes a fully-commented config file (the canonical config reference)
stagehand config path     # shows where the global config lives
```

See the [docs/](docs/) for the full reference (growing).

## Adding a new agent

No recompilation needed — community agents land via a manifest in your config file. Drop a `[provider.<name>]` block into `~/.config/stagehand/config.toml` (or a repo-local `.stagehand.toml`):

```toml
# ~/.config/stagehand/config.toml
[provider.myagent]
command            = "/opt/myagent/bin/agent"
prompt_delivery    = "stdin"          # stdin | positional | flag
print_flag         = "--once"
model_flag         = "--model"
default_model      = "my-model-7b"
system_prompt_flag = "--system"
bare_flags         = ["--no-mcp", "--ephemeral"]
output             = "raw"            # raw | json
```

Verify it works with `providers show`:

```bash
stagehand providers show myagent
```

Then use it:

```bash
stagehand --provider myagent
```

For field reference, copy from the [shipped `providers/*.toml` files](providers/) in this repo — `providers/pi.toml` is the cleanest template.

## FAQ

### Stagehand is not for you if…

…you don't have (and don't want) a coding-agent CLI installed. Stagehand has no model of its own — it is a thin wrapper around *your* agent. If you just want an API-key-based commit generator, [opencommit](https://github.com/dlintw/opencommit) is the right tool.

### Will it corrupt my repo?

No. Stagehand uses `git write-tree` + `git commit-tree` + `git update-ref` (atomic snapshot commits). A failed generation leaves the repo byte-for-byte unchanged — it never touches the live index during generation.

### Does it send my code anywhere new?

No. It shells out to *your* agent under *your* existing auth and billing. Stagehand never opens an HTTP connection to any API — your agent does, exactly as it would if you ran it manually.

### Can it write multiple commits?

Not in v1 — v1 creates a single commit per invocation. Multi-commit hunk decomposition is planned for v2.

### How does it match my project's style?

It learns from the last 20 commits in your repo, with a prohibition on reusing their wording. It also guarantees that no generated subject duplicates one of the last 50 subjects. This means the messages improve over time as your project's style settles.

### Which agents are supported?

Six built-ins are auto-detected: **pi**, **claude**, **gemini**, **opencode**, **codex**, **cursor**. Any agent with a non-interactive CLI interface can be added via a `[provider.<name>]` manifest — see [Adding a new agent](#adding-a-new-agent).

### How do I see what command it runs?

```bash
stagehand --verbose
```

This prints the resolved provider command, raw agent output, and retry attempts.

---

## Contributing

Stagehand is built with Go. To build from source:

```bash
make build        # produces ./bin/stagehand
make test         # run the test suite
make help         # see all targets
```

See [providers/*.toml](providers/) for the manifest format if you want to add support for a new agent.
