# Stagecoach

> **Stagecoach writes your commit messages using the AI agent you already pay for.**
> No API key. No per-token billing. It shells out to Claude Code, Codex, pi, opencode, agy, qwen-code, or Cursor — whatever you already have installed — and spends your existing coding-plan quota instead. Stage while it thinks; it commits only what was staged when it started, atomically, and can never corrupt your repo. With a dirty working tree and nothing staged, it automatically decomposes your changes into a sequence of logically-coherent commits.

A snapshot-based AI commit message generator that uses YOUR local CLI agent. v2.1 adds payload exclusions, message shaping, git hook mode, editor/git integrations, `--edit`/`--push`, and model discovery — see [Features](#features) below.

<!-- TODO: add LICENSE file and badge -->

![CI](https://github.com/dabstractor/stagecoach/actions/workflows/ci.yml/badge.svg)

## 30-second demo

<!-- TODO: record asciinema demo; for now see the snapshot workflow diagram below -->

> [!NOTE]
> A recorded walkthrough is coming soon. See the [snapshot workflow](#the-snapshot-workflow) below for what you'll see.

## Why not opencommit/aicommits?

The incumbent tools — opencommit, aicommits — own the HTTP call to the model, so they can normalize providers, handle retries, and abstract auth. Once you own the HTTP call, you cannot use a coding-plan subscription, because that subscription is not reachable over the public API. Not every plan is locked down this way — a few are permissive (Opencode's, for one) — but the most popular ones gate their quota to the official harness (Anthropic, Google Antigravity, Cursor), and Z.ai even subsidizes harness use with free tokens. The quota lives behind your agent's CLI either way, which is exactly why stagecoach shells out to that CLI instead of opening its own connection.

Stagecoach inverts the architecture: it shells out to your installed CLI agent, trading provider normalization for quota reuse — the agent brings its own auth and billing. That trade-off — give up control of the model call in exchange for access to the user's existing quota — is the entire product.

| | **opencommit / aicommits** | **Stagecoach** |
|---|---|---|
| Auth | API key required | None — uses your agent's existing auth |
| Architecture | Owns the HTTP call | Shells out to your CLI agent |
| Billing | Per-token | Your existing coding-plan quota |
| Stage while generating | No | Yes (snapshot-based) |
| Multi-commit decomposition | No | Yes (auto-decompose dirty tree into N logical commits) |
| Per-role model routing | No | Yes (planner/stager/message/arbiter — right model for the right job) |

<details>
<summary><em>Which coding plans actually gate their quota?</em></summary>

How strictly a coding plan's quota is tied to its official harness varies by provider. A few are
permissive; the popular ones are not — and that distinction is the whole reason stagecoach shells
out to your agent rather than calling an API.

- **Anthropic (Claude Code)** — strict. Plan quota is gated to the Claude Code harness and isn't
  reachable over the public API.
- **Google Antigravity** — strict (and newly arriving). Quota is reserved for the harness.
- **Cursor** — has explicit policies against use outside its own harness.
- **Z.ai** — permissive in principle, but actively pro-harness: it hands subscribers free tokens
  for using the Z.ai harness, steering (rewarding) harness use rather than locking it.
- **Opencode (Opencode Go plan)** — permissive; the notable exception that doesn't gate quota to
  a harness.

Net: almost every provider cares about keeping you on their harness, whether by lock (Anthropic,
Antigravity, Cursor) or by incentive (Z.ai). Opencode is the outlier.

</details>

> [!NOTE]
> What we deliberately didn't build is tracked in [FUTURE_SPEC.md](FUTURE_SPEC.md).

## Features

Stagecoach does one thing — commit messages — and a few things around them.

| Capability | Description |
|---|---|
| Multi-commit decomposition | Auto-decompose a dirty, un-staged tree into N logical commits (planner → stager → message → arbiter). A start-of-run freeze means a concurrent edit during the run can never enter a commit — including across the leftover-reconciliation arbiter; the planner partitions per file and leans toward a soft count target ([how it works](docs/how-it-works.md#multi-commit-decomposition) · [flags](docs/cli.md)). |
| Payload exclusions | `.stagecoachignore` / `--exclude` hide a file's diff from the model — never from the commit ([docs](docs/configuration.md#exclusion-globs-generationexclude)). |
| Payload optimization | The diff sent to your agent is trimmed and budgeted — rename-aware (`-M`), reduced-context (`-U1`), led by a compact file skeleton, and optionally capped to your model's context window via `token_limit` — a closed-loop guarantee that the assembled prompt never exceeds the limit ([how it works](docs/how-it-works.md#diff-capture-pipeline) · [knobs](docs/configuration.md#built-in-defaults)). |
| Multi-turn fallback | Lossless multi-turn fallback: when a one-shot generation of a large diff fails, stagecoach re-delivers the full diff across session turns so the message still lands — no truncation, no extra commits ([how it works](docs/how-it-works.md#multi-turn-generation-fallback) · [knobs](docs/configuration.md#built-in-defaults)). |
| Message shaping | `--format` (auto, conventional, gitmoji, plain), `--locale`, `--context`, `--template` ([docs](docs/how-it-works.md#format-modes-and-locale)). |
| Git hook mode | `stagecoach hook install` fills the message on `git commit` — pre-commit hooks honored, never blocks ([docs](docs/how-it-works.md#trade-off-inversion-fr-h7)). |
| Commit hooks on every `stagecoach` commit | As of v2.4, your repo's `pre-commit` → `prepare-commit-msg` → `commit-msg` → `post-commit` hooks run around every `stagecoach` commit, scoped to the frozen snapshot (atomic + stage-while-generating preserved); `--no-verify` mirrors git ([how it works](docs/how-it-works.md#commit-hooks-on-the-plumbing-path)). |
| Tool integrations | `stagecoach integrate install git-alias lazygit` wires `git stagecoach` and a lazygit keybind ([docs](docs/cli.md#integrate-install-target)). |
| `--edit` / `--push` | Review in `$EDITOR` before the atomic commit; push after a clean run ([docs](docs/cli.md)). |
| Discovery | `stagecoach models` and `config init --interactive` for guided setup ([docs](docs/cli.md#models-provider)). |

<!-- Multi-turn fallback (Features row above): intentionally generic — "stagecoach" re-delivers, NOT
     "the commit path". Multi-turn runs on EVERY generation path (snapshot commit, `--dry-run`, hook
     mode); the per-path detail lives in docs/how-it-works.md#multi-turn-generation-fallback (linked
     from the row), so this high-level row deliberately does NOT enumerate paths. "no extra commits"
     is an anti-misconception note (one message/commit, not N), accurate on all three paths. Do not
     narrow this row. (P1.M4.T1.S2.) -->

## Install

**Prerequisite:** a coding-agent CLI already installed and on `$PATH` (pi, Claude Code, opencode, Codex, Cursor, agy, or qwen-code).

> [!NOTE]
> Stagecoach is pre-release and still being tested locally — **build from source** is the only working install method today. The package-managed channels below are coming with the first public release.

### Build from source (works today)

Requires [Go](https://go.dev) 1.22+:

```bash
git clone https://github.com/dabstractor/stagecoach.git
cd stagecoach
make install          # installs the binary to $GOPATH/bin
```

Ensure `$GOPATH/bin` (usually `~/go/bin`) is on your `$PATH`, then verify:

```bash
stagecoach --version   # stagecoach version dev
```

> [!TIP]
> If you keep your user binaries elsewhere (e.g. `~/.local/bin`), symlink it instead of editing `$PATH`: `ln -s "$(go env GOPATH)/bin/stagecoach" ~/.local/bin/stagecoach`. `make install` overwrites the target in place, so the link stays valid across rebuilds.

### Coming soon

These will land with the first release, once the tap/bucket repos are published:

- **Homebrew** (macOS / Linuxbrew) — `brew install dabstractor/tap/stagecoach`
- **Scoop** (Windows) — `scoop install dabstractor/stagecoach`
- **`go install`** — `go install github.com/dabstractor/stagecoach/cmd/stagecoach@latest`
- **Direct binary** (curl​|​sh one-liner) — `curl -fsSL https://github.com/dabstractor/stagecoach/raw/main/install.sh | bash`

## Quick start

```bash
# 1. Stage your changes
git add feature/login.js

# 2. Run stagecoach — it snapshots, generates, and commits atomically
stagecoach
# [abc1234] feat: add login flow
# M  src/login.js

# 3. Stage everything and commit in one step
stagecoach -a

# 4. Preview the real message (full pipeline: snapshot→generate→parse→dedupe→retry), no commit
stagecoach --dry-run
```

> [!NOTE]
> If generation fails, `--dry-run` exits 1 with a short message — not the full recovery recipe or exit 3/124 — since no commit was ever intended.

### More options

```bash
stagecoach --push                 # commit + push after a clean run
stagecoach --edit                 # review in $EDITOR before the atomic commit
stagecoach --format conventional  # force conventional-commit style
stagecoach --exclude '*.snap'     # hide snapshot diffs from the model (still committed)
```

See [Features](#features) above and the [CLI reference](docs/cli.md) for the rest.

### Multi-commit decomposition

With a dirty working tree and nothing staged, `stagecoach` automatically decomposes your changes into a sequence of logically-coherent commits using a four-role agent pipeline (planner → stager → message → arbiter). Each concept becomes its own commit. A start-of-run freeze (T_start) captures your entire change set up front, so files you change mid-run are excluded from every commit — the run only ever commits what existed when it started, and that holds across the leftover-reconciliation arbiter too (a concurrent edit can never sneak into a commit). As defense-in-depth, decompose also re-asserts its empty-index precondition at entry (FR-M1e), so a stale trigger that reaches it with a staged index fails loudly rather than silently folding that hand-staged content into the run. The planner partitions changes per file and leans toward a soft count target, so a typical mixed tree lands at or below half the cap. The stager is constrained to staging operations: claude via a staging-only git allowlist (`git add`/`apply`/`status`/`diff`); pi instructionally (its task prompt) plus a HEAD-movement guard that aborts the run if the stager moves a ref. Either way, Stagecoach owns every commit via git plumbing.

```bash
# Auto-decompose — planner decides the count and grouping
stagecoach
# Decomposes into N commits automatically

# Use reasoning for deeper analysis on the planner
stagecoach --reasoning high

# Force exactly 3 commits
stagecoach --commits 3

# Keep the v1 single-commit behavior
stagecoach --single

# Route planning to a bigger model (per-repo .stagecoach.toml):
# [role.planner]
# provider = "claude"
# model = "opus"
# timeout = "600s"   # per-role generation timeout (FR-R7); the planner defaults to 480s
```

> [!NOTE]
> `--reasoning` is provider-dependent: it engages deeper reasoning for **pi** (`--thinking`) and
> **claude** (`--effort`). Other providers treat it as a graceful no-op (no error) per FR-R6. It
> applies to any role via `--<role>-reasoning` or `[role.*] reasoning`.

See [How Stagecoach works — Multi-commit decomposition](docs/how-it-works.md#multi-commit-decomposition) for the pipeline architecture and [CLI reference](docs/cli.md) for all decompose and per-role flags.

### lazygit & git alias

```bash
stagecoach integrate install lazygit      # default key <c-a>; --key '<c-s>' to customize
stagecoach integrate install git-alias     # enables `git stagecoach` everywhere
stagecoach integrate list                  # see what's installed / detected
```

> [!NOTE]
> gitui isn't supported — see [FUTURE_SPEC.md](FUTURE_SPEC.md) §1.2.

<details>
<summary><em>Manual install (no <code>stagecoach integrate</code>)</em></summary>

If you prefer to paste the YAML yourself, add this to your lazygit `config.yml` (see [docs/cli.md#lazygit-target](docs/cli.md#lazygit-target) for the canonical block):

```yaml
customCommands:
  - key: '<c-a>'                       # stagecoach-integration
    context: 'files'
    command: 'stagecoach'
    loadingText: 'Generating commit message…'
    output: 'none'
    description: 'stagecoach: AI commit'
```

</details>

## Configure your agent

Stagecoach auto-detects which agents are installed and uses the first one it finds (in preference order: pi, opencode, cursor, agy, qwen-code, codex, claude). To see what's detected:

```bash
$ stagecoach providers list
NAME       DETECTED  DEFAULT
agy        ✓
claude     ✓
codex      ✓
cursor     ✓
opencode   ✓
pi         ✓         (default)
qwen-code  ✗
```

> [!NOTE]
> A provider whose command isn't on `$PATH` fails fast with exit 1 before any snapshot — no partial state, no rescue recipe.

Set a per-repo default with git config:

```bash
git config stagecoach.provider claude
# Optionally pin a model (single-backend providers use a bare model):
git config stagecoach.model sonnet

# For pi (multi-backend), prefix the model with the inference backend:
git config stagecoach.provider pi
git config stagecoach.model zai/glm-5.2
```

> [!NOTE]
> `pi` is a multi-backend provider: the inference backend is a slash-prefix on the model
> (`zai/glm-5.2`). A bare model (no `/`) on pi is a config error (FR-R5b). Set
> `git config stagecoach.model zai/glm-5.2` (or `[defaults] model = "zai/glm-5.2"` in your config).
> See [Provider manifests](docs/providers.md) for the full schema.

Or bootstrap a **populated, working config** (auto-detects your agent and writes per-role model defaults — for **pi**, the default, per-role models are left empty so you can supply your own inference-backend/model prefix; set `model = "zai/glm-5.2"` to pin a specific backend):

```bash
stagecoach config init
# Wrote config to ~/.config/stagecoach/config.toml

# See where that lives:
stagecoach config path
# ~/.config/stagecoach/config.toml

# Upgrade a v1 config to the current schema:
stagecoach config upgrade
```

> [!NOTE]
> The template also documents a `[generation]` section: `output` ("raw"|"json") and `strip_code_fence` are an **opt-in override** for how Stagecoach parses agent output. When unset, the per-provider `[provider.<name>]` value is used (defaulting to `raw` / `true`); set them under `[generation]` only to force the value across ALL providers.

Point discovery at a specific file with `stagecoach --config path/to/config.toml`. It is honored by every command — including the default commit action **and the `config init`, `config path`, and `config upgrade` subcommands** (e.g. `stagecoach --config X config upgrade` upgrades file `X`, and `config path` prints the resolved path) — so a provider declared under `[provider.<name>]` there is usable with `--provider <name>` directly. The path must exist: an explicit `--config` (or `STAGECOACH_CONFIG`) pointing at a missing file fails fast with exit 1 rather than silently falling back to auto-detection.

**Config precedence** (highest → lowest): CLI flags > `STAGECOACH_*` env vars > repo `git config` (`stagecoach.*`) > repo `.stagecoach.toml` > global config file > provider defaults > built-in defaults.

## The snapshot workflow

Stagecoach creates commits against a frozen snapshot of your index — not the live state. This means you can keep staging more files while the message generates, and they will **never** be included in the current in-flight commit.

```text
Pane A (lazygit / shell)        Pane B (shell)
─────────────────────────       ───────────────────────
git add feature/login.js
stagecoach                     ┐
  ↳ snapshotting…             │  (user is free to work here)
  ↳ generating with pi…       │  git add docs/login.md
  ↳ (10s pass)                │  git add tests/login.test.js
  ↳ created abc1234           │  (these stay staged — NOT in abc1234)
                              ┘
                                stagecoach        # next run commits these
```

Generation time is no longer dead time. The in-flight commit only ever contains what was staged when it started, so you can stage the next batch freely while the current message generates — and a failed generation leaves your repo byte-for-byte unchanged.

## Full CLI and config reference

The **authoritative, always-available** reference lives in the binary itself:

```bash
stagecoach --help          # every flag, subcommand, and option
stagecoach config init     # bootstraps a populated working config (auto-detects your agent)
stagecoach config upgrade  # upgrades an existing config to the current schema version
stagecoach config path     # shows where the global config lives
stagecoach lock status     # inspect the per-repo run lock holder (path, pid, liveness, orphan status)
```

See the [docs/](docs/) for the full reference (growing).

## Adding a new agent

No recompilation needed — community agents land via a manifest in your config file. Drop a `[provider.<name>]` block into `~/.config/stagecoach/config.toml` (or a repo-local `.stagecoach.toml`):

```toml
# ~/.config/stagecoach/config.toml
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
stagecoach providers show myagent
```

Then use it:

```bash
stagecoach --provider myagent
```

For field reference, copy from the [shipped `providers/*.toml` files](providers/) in this repo — `providers/pi.toml` is the cleanest template.

## FAQ

### Stagecoach is not for you if…

…you don't have (and don't want) a coding-agent CLI installed. Stagecoach has no model of its own — it is a thin wrapper around *your* agent. If you just want an API-key-based commit generator, [opencommit](https://github.com/dlintw/opencommit) is the right tool.

### Will it corrupt my repo?

No. Stagecoach uses `git write-tree` + `git commit-tree` + `git update-ref` (atomic snapshot commits). A failed generation leaves the repo byte-for-byte unchanged — it never touches the live index during generation.

**Safe to run twice.** A per-repo run lock prevents two concurrent commit-producing runs from racing on HEAD. On the **single-commit path** (changes staged), an accidental double-invoke exits `0` if nothing new has been staged since the in-progress run began (*nothing to do — an in-progress run already covers your staged changes*), or exits `5` (Busy) if genuinely new work is staged (your changes stay staged to re-run). On the **decompose path** (nothing staged, dirty working tree), an accidental double-run exits `5` (Busy) rather than `0` — the in-progress run publishes a working-tree snapshot a contender can't reproduce without the lock, so it conservatively refuses and leaves your working tree untouched. (On a shared filesystem across hosts the lock can't help — the atomic `update-ref` CAS is the never-clobber-HEAD guarantee there.)

If the launcher closed without killing stagecoach — you closed the lazygit TUI, quit your IDE, or detached the terminal mid-run — the orphaned run **self-exits** via a parent-death watchdog (FR-K1) and releases the lock, so it never strands. `stagecoach lock status` (FR-K4) shows the holder's path and liveness so you can decide whether to `kill`/`rm` yourself; it never force-breaks a live lock.

### Does it send my code anywhere new?

No. It shells out to *your* agent under *your* existing auth and billing. Stagecoach never opens an HTTP connection to any API — your agent does, exactly as it would if you ran it manually.

### Can it write multiple commits?

Yes. Run `stagecoach` with a dirty working tree and nothing staged, and it automatically decomposes the changes into a sequence of logically-coherent commits. Force a count with `--commits 3`, or keep the one-commit behavior with `--single`. See [Multi-commit decomposition](#multi-commit-decomposition) and the [pipeline architecture](docs/how-it-works.md#multi-commit-decomposition).

### How does it match my project's style?

It learns from the last 20 commits in your repo, with a prohibition on reusing their wording. It also guarantees that no generated subject duplicates one of the last 50 subjects. This means the messages improve over time as your project's style settles.

### Which agents are supported?

Seven built-ins are auto-detected: **pi**, **opencode**, **cursor**, **agy** *(experimental)*, **qwen-code** *(experimental)*, **codex**, **claude**. (Google's `gemini` / Gemini CLI is **no longer shipped** — it was superseded by **agy**, the Antigravity CLI, on 2026-06-18.) Any agent with a non-interactive CLI interface can be added via a `[provider.<name>]` manifest — see [Adding a new agent](#adding-a-new-agent).

**End-to-end verification status** (this build): **pi**, **agy**, **codex**, **opencode**, and **claude** have each been driven through a real commit-generation run. **cursor is NOT yet verified end-to-end** — its manifest is assembled from `agent --help` and ships untested here. If you're a paying **Cursor** subscriber and would be willing to spend a few `agent` runs helping confirm the cursor path (the `--mode ask --trust -p` read-only combo), please open an issue — cursor is the one provider the maintainer can't validate without an account.

### How do I see what command it runs?

```bash
stagecoach --verbose
```

This prints the resolved provider command, raw agent output, and retry attempts.

### Does it run my pre-commit hooks?

Yes. As of v2.4, the default `stagecoach` command runs your repository's standard commit hooks (`pre-commit` → `prepare-commit-msg` → `commit-msg` → `post-commit`) around every commit, scoped to the frozen snapshot — so atomicity and stage-while-generating are preserved (a `pre-commit` formatter's fixes are included; a hook that stages brand-new content aborts the run). `--no-verify` skips `pre-commit` and `commit-msg` only, mirroring `git commit --no-verify`. **Hook mode** (`stagecoach hook install`) remains for when you commit via plain `git commit` from an IDE — the two compose (§9.25 covers `stagecoach`; hook mode covers `git commit`). See [Commit hooks on the plumbing path](docs/how-it-works.md#commit-hooks-on-the-plumbing-path).

### What about PR generation, editor extensions, a GitHub Action, API-key providers?

Stagecoach writes commit messages — nothing else. Ideas we considered but deferred or rejected — VS Code/neovim extensions, a GitHub Action, gitui integration, API-key HTTP providers, generate-N-and-pick, diff chunking, self-update, and more — each with its reason — live in [FUTURE_SPEC.md](FUTURE_SPEC.md).

---

## Contributing

Stagecoach is built with Go. To build from source:

```bash
make build        # produces ./bin/stagecoach
make test         # run the test suite
make help         # see all targets
```

See [providers/*.toml](providers/) for the manifest format if you want to add support for a new agent.
