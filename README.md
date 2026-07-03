# stagehand

[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](#license)

> Conventional, AI-friendly Git commits, staged from natural language.

> **Stagehand writes your commit messages using the AI agent you already pay for.**
> No API key. No per-token billing. It shells out to Claude Code, Codex, Gemini CLI, pi, opencode, or Cursor — whatever you already have installed — and spends your existing coding-plan quota instead. Stage while it thinks; it commits only what was staged when it started, atomically, and can never corrupt your repo.

`stagehand` is a single static Go binary that turns a staging area into a [conventional commit](https://www.conventionalcommits.org). It never calls a provider HTTP API and never holds an API key — it runs the same coding-agent CLI you already run, and it commits through `git`'s own plumbing so the message can generate while you keep staging the next batch.

---

## Table of contents

1. [Hero](#stagehand) &nbsp;·&nbsp; [30-second demo](#30-second-demo) &nbsp;·&nbsp; [Why not opencommit / aicommits?](#why-not-opencommit--aicommits)
2. [Install](#install) &nbsp;·&nbsp; [Quick start](#quick-start) &nbsp;·&nbsp; [Configure your agent](#configure-your-agent)
3. [The snapshot workflow (stage while it thinks)](#the-snapshot-workflow-stage-while-it-thinks)
4. [CLI + config reference](#cli--config-reference) &nbsp;·&nbsp; [Adding a new agent](#adding-a-new-agent) &nbsp;·&nbsp; [FAQ](#faq--stagehand-is-not-for-you-if)

---

## 30-second demo

<!-- TODO: record a 30s asciinema cast of: stage a few files -> `stagehand` -> it generates -> atomically commits -> user keeps staging in a second pane during generation -> `stagehand` again commits only the new batch. -->

```
coming soon — a 30-second asciinema demo (stage → stagehand → commit, with
stage-while-generating shown across two panes).
```

---

## Why not opencommit / aicommits?

[opencommit](https://github.com/di-sukharev/opencommit) and [aicommits](https://github.com/Nutlope/aicommits) own the HTTP call to the model, which means they need an API key and bill you per token on top of the coding plan you already pay for. Stagehand gives up control of that call and shells out to the agent you already have installed instead — strictly worse for provider normalization, strictly better for quota reuse. The result: no API key, no new billing relationship, and a commit message that spends the plan you already bought.

---

## Install

**Prerequisite:** a supported coding-agent CLI installed and authenticated on your `$PATH` — Claude Code (`claude`), Codex (`codex`), Gemini CLI (`gemini`), `pi`, `opencode`, or Cursor (`agent`). Stagehand never manages credentials; if your agent runs, stagehand runs.

Pick one path (all four resolve to the same binary):

```bash
# Homebrew (macOS / Linuxbrew)
brew install dustin/tap/stagehand

# Go install (anywhere with Go 1.22+)
go install github.com/dustin/stagehand/cmd/stagehand@latest

# Direct binary (curl | sh)
curl -fsSL https://github.com/dustin/stagehand/raw/main/install.sh | bash

# Windows (Scoop)
scoop install dustin/stagehand
```

Verify it landed:

```bash
stagehand --version
# stagehand version <tag>
```

---

## Quick start

From inside a git repository with some changes staged:

```bash
# Commit whatever is staged, using your default detected agent.
stagehand

# See the message it would write, without committing anything.
stagehand --dry-run

# Quick checkpoint: stage everything and commit in one shot.
stagehand -a
```

With no subcommand, `stagehand` resolves the provider + config (flag → env → git-config → file → defaults), optionally auto-stages everything when nothing is staged, asks the resolved agent for a conventional commit message, and creates the commit atomically. Generation failures leave your repo byte-for-byte unchanged (see [the snapshot workflow](#the-snapshot-workflow-stage-while-it-thinks)).

---

## Configure your agent

Stagehand ships six built-in providers: **pi**, **claude** (Claude Code), **gemini** (Gemini CLI), **opencode**, **codex**, and **cursor**. See which are installed and which is the resolved default:

```bash
stagehand providers list
```

That prints one row per provider with a `detected` / `not detected` status and the resolved default provider + model. Set a per-repo default (persisted in the repo's git config):

```bash
git config stagehand.provider pi
git config stagehand.model glm-5.2
```

…or pick an agent and model for a single commit:

```bash
stagehand --provider claude --model sonnet
```

First-time setup? Scaffold a fully-commented config file you can edit, then point stagehand at it:

```bash
stagehand config init        # writes a documented no-op config to the global path
stagehand config path        # prints where stagehand looks for it
```

The full precedence chain (flag → env → git-config → file → default), every `STAGEHAND_*` env var, and every `stagehand.*` git-config key live in **[docs/CONFIGURATION.md](docs/CONFIGURATION.md)**.

---

## The snapshot workflow (stage while it thinks)

Stagehand never calls `git commit`. It freezes the index with `git write-tree`, generates the message, then creates the commit with `git commit-tree` and advances `HEAD` with the two-argument `git update-ref` (a compare-and-swap that refuses to move `HEAD` if it has changed underneath us). The committed content is exactly what was staged when `write-tree` ran — nothing more.

That means generation time is no longer dead time. You can keep staging the next batch in another pane while the current message generates:

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

If generation fails, stagehand never reaches `update-ref` — your `HEAD` and index are untouched, and the rescue protocol prints the frozen tree SHA plus the exact manual recovery commands. The only artifacts left behind are harmless objects that `git gc` reaps. The in-flight commit can only ever contain what was staged when it started, and a failure can never corrupt your repo.

---

## CLI + config reference

- **[docs/CONFIGURATION.md](docs/CONFIGURATION.md)** — the precedence chain, every `STAGEHAND_*` env var, the `stagehand.*` git-config keys, the `.stagehand.toml` schema, the full CLI flag table, and the exit codes (`0` success, `1` error, `2` nothing to commit, `3` rescue).
- **[docs/PROVIDERS.md](docs/PROVIDERS.md)** — `stagehand providers list` / `providers show`, the built-in provider table, and the field-merge rules for overriding a manifest.

---

## Adding a new agent

You do not need to fork or recompile stagehand to drive an agent it does not know about. Drop a `[provider.<name>]` block into a config file and point stagehand at it. (Git-config **cannot** express `[provider.<name>]` tables, so user-defined providers are file-only.)

```toml
# ~/.config/stagehand/config.toml
[provider.myagent]
command = "/opt/myagent/bin/agent"
prompt_delivery = "stdin"
print_flag = "--once"
model_flag = "--model"
default_model = "my-model-7b"
system_prompt_flag = "--system"
bare_flags = ["--no-mcp", "--ephemeral"]
output = "raw"
```

Then:

```bash
stagehand --provider myagent
```

…or set `stagehand.provider = myagent` in the TOML `[defaults]` table. `stagehand providers show myagent` prints the fully-resolved manifest (the exact command stagehand will render), which is the debugging surface when an agent does not behave as expected.

---

## FAQ / "Stagehand is not for you if…"

**I don't have any coding-agent CLI installed and I don't want one.**
Then stagehand is useless to you — it spends the quota of a coding-plan subscription you already have, and there is no way to reach that subscription over the public API. If you want the paste-an-API-key path, [opencommit](https://github.com/di-sukharev/opencommit) is the right tool. The README says so plainly to avoid disappointing installs.

**Does stagehand split a big staging batch into multiple commits?**
Not in v1. Every invocation produces exactly **one** commit. Multi-commit decomposition (partitioning a batch into logically coherent commits) is the headline v2 feature — it reuses the snapshot/atomic-commit foundation v1 is built on. If you need hunk-splitting today, opencommit does it well.

**Will stagehand ever take an `--api-key` flag?**
No — that is a deliberate, permanent architectural boundary, not a limitation to be lifted later. Stagehand will never own the HTTP call to a model. It also will never be an AI coding assistant, code reviewer, or PR-description writer. It writes commit messages; the narrow scope is the product.

---

## License

MIT (see [`.goreleaser.yaml`](.goreleaser.yaml)). A top-level `LICENSE` file ships with releases.

> **Note for the maintainer:** no `LICENSE` file is committed to the repo root yet. `.goreleaser.yaml` sets `license: MIT` and `archives.files: [LICENSE*, README*]`, so a missing `LICENSE` will warn "no files matched" on each release. Add a standard MIT `LICENSE` (out of scope for this task).
