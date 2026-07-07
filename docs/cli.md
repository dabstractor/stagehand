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
| `--exclude <glob>`, `-x` | string (repeatable) | — | — | — | Exclude matching files from the agent payload (placeholder line instead of the diff; never excluded from the commit itself). Unions with `.stagehandignore` and `[generation].exclude` — repeat the flag to add more than one glob; it does not override the config-file set |
| `--format <mode>` | string | `auto` | `STAGEHAND_FORMAT` | `stagehand.format` | Message format: `auto` (style learning) \| `conventional` \| `gitmoji` \| `plain`. An unknown mode is a hard error (exit 1). Also `[generation].format`. |
| `--locale <lang>` | string | "" | `STAGEHAND_LOCALE` | `stagehand.locale` | Write the message in this language (free-form name or BCP-47 tag; never validated). Also `[generation].locale`. |
| `--template <tpl>` | string | "" | `STAGEHAND_TEMPLATE` | `stagehand.template` | Wrap every commit message: `$msg` is replaced with the generated message, e.g. `"$msg (#205)"`. Must contain the literal `$msg` (else hard error, exit 1). Applies to every commit in a run. Also `[generation].template`. Distinct from `config init --template`. |
| `--context <text>` | string | "" | — | — | Extra authoritative context appended to the message and planner payloads (e.g. `"hotfix for #812"`). Flag only — per-invocation; no env var, git-config, or config-file key. |
| `--edit` | bool | false | — | — | Open your editor (`$GIT_EDITOR` via `git var GIT_EDITOR`) on the generated message before committing. The EDITMSG file includes the tree SHA + a diff-tree name-status summary; comment lines (`#`) are stripped on close. An empty result aborts (exit 1, not a rescue). The edited message bypasses the duplicate check (git parity). In decompose mode each commit is gated. Ignored with `--dry-run`; not valid with `hook exec`. (§9.22 FR-E1) |
| `--push` | bool | false | `STAGEHAND_PUSH` | `stagehand.push` | Run plain `git push` (no arguments, streaming its output) after a fully-successful run. Never prompts. On push failure the commits stand — git's stderr is shown verbatim (including the no-upstream hint; stagehand does NOT auto-`--set-upstream`), "commits created; push failed" prints, and stagehand exits 1. Skipped on `--dry-run`, the nothing-to-commit exit, and any rescue/CAS abort. Also `[generation].push`. (§9.22 FR-P1) |
| `--no-verify` | bool | false | `STAGEHAND_NO_VERIFY` | `stagehand.noVerify` | Bypass `pre-commit` and `commit-msg` hooks for this commit (mirrors `git commit --no-verify`; `prepare-commit-msg` and `post-commit` still run). §9.25, FR-V5. |
| `--planner-provider <name>` | string | "" | `STAGEHAND_PLANNER_PROVIDER` | — | Per-role provider override for the decomposition planner |
| `--planner-model <name>` | string | "" | `STAGEHAND_PLANNER_MODEL` | — | Per-role model override for the decomposition planner |
| `--stager-provider <name>` | string | "" | `STAGEHAND_STAGER_PROVIDER` | — | Per-role provider override for the (tooled) staging agent |
| `--stager-model <name>` | string | "" | `STAGEHAND_STAGER_MODEL` | — | Per-role model override for the (tooled) staging agent |
| `--arbiter-provider <name>` | string | "" | `STAGEHAND_ARBITER_PROVIDER` | — | Per-role provider override for the leftover arbiter |
| `--arbiter-model <name>` | string | "" | `STAGEHAND_ARBITER_MODEL` | — | Per-role model override for the leftover arbiter |
| `--reasoning <level>` | string | "" (off) | `STAGEHAND_REASONING` | `stagehand.reasoning` | Global reasoning effort: off\|low\|medium\|high. Provider-dependent: engages for pi (`--thinking`) and claude (`--effort`); other providers are a graceful no-op (FR-R6). |
| `--planner-reasoning <level>` | string | "" | `STAGEHAND_PLANNER_REASONING` | — | Per-role reasoning for the planner |
| `--stager-reasoning <level>` | string | "" | `STAGEHAND_STAGER_REASONING` | — | Per-role reasoning for the stager |
| `--message-provider <name>` | string | "" | `STAGEHAND_MESSAGE_PROVIDER` | — | Per-role provider override for the message composer |
| `--message-model <name>` | string | "" | `STAGEHAND_MESSAGE_MODEL` | — | Per-role model override for the message composer |
| `--message-reasoning <level>` | string | "" | `STAGEHAND_MESSAGE_REASONING` | — | Per-role reasoning for the message composer |
| `--arbiter-reasoning <level>` | string | "" | `STAGEHAND_ARBITER_REASONING` | — | Per-role reasoning for the arbiter |
| `--version` | — | — | — | — | Print the build version (`"dev"` for a local build; the release tag for a released binary) |
| `--help`, `-h` | — | — | — | — | Print help |

The `--config` flag is a path override for config-file discovery — it is not itself a `Config` field. An explicit `--config` (or `STAGEHAND_CONFIG`) pointing at a missing file errors with `config: config file not found: <path>` (exit 1) instead of silently falling back to provider auto-detection. Only the discovery default (no `--config` or `STAGEHAND_CONFIG`) tolerates a missing global file. The behavioral flags (`--all`, `--no-auto-stage`, `--dry-run`) have no env-var or git-config analogs. `--config` is honored by every command — including the default commit action **and the `config init`, `config path`, and `config upgrade` subcommands** (e.g. `stagehand --config X config upgrade` upgrades file `X`, and `config path` prints the resolved path) — so a user-defined provider declared under `[provider.<name>]` in that file is usable with `--provider <name>` on `stagehand` directly.

## Subcommands

### `hook install`

Install stagehand's `prepare-commit-msg` hook in the current repo. Writes an executable (0755) script containing the marker `# stagehand prepare-commit-msg hook v1` at the repo's hooks directory. Re-running overwrites an existing stagehand hook (idempotent — reports "Installed" on first run, "Updated" on subsequent).

The hook script calls `stagehand hook exec "$@"` (runtime lands in P1.M3.T2.S1 — not yet shipped).

```bash
stagehand hook install              # write the hook
stagehand hook install              # → "Updated stagehand prepare-commit-msg hook." (idempotent)
stagehand hook install --strict    # bake --strict into the script body
stagehand hook install --print     # print the script to stdout, no disk write (works outside a repo)
```

| Flag | Description |
|------|-------------|
| `--strict` | Bake `--strict` into the hook so generation failures abort the commit (default: never block) |
| `--print` | Write the hook script to stdout instead of installing it |

**Foreign-hook policy (never-clobber, FR-H2):** If a `prepare-commit-msg` already exists WITHOUT stagehand's marker, `install` refuses (exit 1) and prints the one-line manual invocation you can add to your existing hook. There is **no `--force`** — this is by design. Stagehand will never overwrite someone else's hook.

### `hook uninstall`

Remove stagehand's `prepare-commit-msg` hook. Only removes the file when the marker is present. If no hook exists, prints an informational note and exits 0 (idempotent). A foreign hook is refused (exit 1, untouched).

```bash
stagehand hook uninstall            # → "Removed stagehand prepare-commit-msg hook."
stagehand hook uninstall            # (no hook) → "No stagehand prepare-commit-msg hook to remove." (exit 0)
```

### `hook status`

Report the current state of the repo's `prepare-commit-msg` hook. Prints exactly one line:

| Output | Meaning |
|--------|---------|
| `none` | No `prepare-commit-msg` file exists |
| `stagehand (v1)` | A stagehand-owned hook is installed (marker present) |
| `foreign` | A `prepare-commit-msg` exists WITHOUT stagehand's marker (never touched by install/uninstall) |

```bash
stagehand hook status              # → "none"
stagehand hook install
stagehand hook status              # → "stagehand (v1)"
```

### `hook exec`

Generate a commit message into git's `prepare-commit-msg` file. Called by stagehand's installed hook — not by users directly. When `git commit` fires the hook, stagehand generates a message for the staged diff and writes it at the **top** of `<msg-file>`, preserving git's comment block beneath.

**Source-gated no-op (FR-H4):** exits 0 having done nothing when a message source is present (`message`/`template`/`merge`/`squash`/`commit`) or nothing is staged. This means `git commit -m "x"`, `git commit -t template`, merge commits, squash commits, and `--amend` all pass through unchanged — the explicit message wins.

**Never-block (FR-H5):** any generation failure (agent missing, timeout, parse failure, duplicate exhaustion) leaves `<msg-file>` byte-identical to before and exits 0 (so the commit proceeds to an empty editor). With `--strict` (baked into the script by `hook install --strict`), the same failure exits non-zero (aborts the commit).

**Message-role resolution (FR-H6):** resolves provider/model/reasoning exactly like the single-commit path (`--message-*` flags, `[role.message]` config, env vars). Never decomposes.

```bash
stagehand hook exec <msg-file>                    # normal invocation (source absent → proceed)
stagehand hook exec <msg-file> message              # source=message → no-op (exit 0)
stagehand hook exec --strict <msg-file>             # abort on failure (exit 1)
```

| Arg | Description |
|-----|-------------|
| `<msg-file>` | Path to git's `prepare-commit-msg` file (e.g. `.git/COMMIT_EDITMSG`) |
| `<source>` | Source of the message (absent/empty = proceed; `message`/`template`/`merge`/`squash`/`commit` = no-op) |
| `<sha>` | Commit SHA (present only when source=`commit`) |

| Flag | Description |
|------|-------------|
| `--strict` | Abort the commit on generation failure (default: never block — exit 0 and leave the message empty) |

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

`✓` = the provider's command is found on `$PATH`. `(default)` marks the provider selected by auto-detection (first installed built-in in preference order: pi, opencode, cursor, agy, gemini, qwen-code, codex, claude).

### `providers show <name>`

Print the fully-merged manifest for a provider as TOML. Exits 1 if the provider is unknown:

```bash
stagehand providers show pi
```

### `config init`

Bootstrap a **populated, working config** to the resolved config path (override-aware: honors `--config` / `STAGEHAND_CONFIG`, defaulting to the global path). Auto-detects the highest-priority installed built-in agent (order: pi, opencode, cursor, agy, gemini, qwen-code, codex, claude) and writes `config_version = 3`, `[defaults] provider = "<detected>"`, and that provider's per-role model defaults — EXCEPT for **pi** (the default), whose per-role models are left EMPTY so pi picks its own backend model (set the model with an inference-provider prefix (e.g. model = "zai/glm-5.2") to pin a backend (FR-R5b)). Other detected providers get their per-role models UNCOMMENTED. Other installed providers appear as commented-out `[role.*]` blocks. If no agent is detected, defaults to `"pi"`. Creates parent directories as needed. **Refuses to overwrite** an existing file (exit 1) unless `--force` is passed:

```bash
stagehand config init
# Wrote config to ~/.config/stagehand/config.toml

# Target a specific provider:
stagehand config init --provider claude

# Overwrite existing config:
stagehand config init --force

# Write the inert all-commented reference (v1 behavior):
stagehand config init --template

# Guided TTY wizard — pick a provider, accept or edit per-role models:
stagehand config init --interactive

# Pre-select a provider, then edit its models:
stagehand config init --interactive --provider pi
```

| Flag | Description |
|------|-------------|
| `--provider <name>` | Target a specific built-in provider instead of auto-detecting |
| `--force` | Overwrite an existing config file |
| `--template` | Write the inert all-commented reference config (v1 behavior) |
| `--interactive` | Guided TTY wizard: pick a detected provider, accept or edit per-role models; prompts for the inference/ prefix on multi-backend providers (pi, opencode). Writes the same file as plain `config init`. Non-TTY → exit 1 (use plain `config init`). |

`--interactive` runs a three-step wizard: (1) pick a provider from the detected set (default highlighted), (2) accept or edit each per-role model default, (3) for multi-backend providers (pi, opencode), prompts for the `inference/model` prefix on any edited model. Writes the **same** file as plain `config init` — the wizard is a TTY front-end. Composes with `--force` (overwrites) and `--provider <name>` (pre-selects, skipping the provider prompt). Mutually exclusive with `--template` (exit 1). Non-TTY stdin exits 1 pointing at plain `config init`.

### `config upgrade`

Upgrade an existing config's `config_version` to the current schema version (3) in place. For multi-backend providers, the former `default_provider` is folded into a slash-prefix on the model (`default_provider = "X"` + `model = "Y"` → `model = "X/Y"`) and the `default_provider` key is deleted. Every other line is preserved. Idempotent — running it twice leaves the file unchanged. No flags.

```bash
stagehand config upgrade
# Already at version 3 →  "Config at ~/.config/stagehand/config.toml is already at version 3 (no changes)."
# Upgraded from v1  →  "Upgraded config at ~/.config/stagehand/config.toml to version 3."
# No file          →  "no config file at <path> (run 'stagehand config init' first)"  (exit 1)
```

At load time, a missing or outdated `config_version` triggers an advisory pointing at `config upgrade` or `config init --force`.

### `config path`

Print the resolved config path (override-aware: honors `--config` / `STAGEHAND_CONFIG`, falling back to the global path):

```bash
stagehand config path
# ~/.config/stagehand/config.toml
```

### `integrate list`

List all integration targets with detection status, integration state, and config path:

```text
TARGET      DETECTED  STATUS         CONFIG
git-alias   ✓         installed      ~/.gitconfig
lazygit     ✓         not installed  —
```

- **TARGET**: the integration name (the `<target>` argument for install/remove)
- **DETECTED**: ✓ if the tool is on `$PATH`, ✗ otherwise
- **STATUS**: `not installed`, `installed`, or `foreign` (a conflicting entry exists)
- **CONFIG**: the resolved config file path the integration edits (— if the tool is absent or the path cannot be determined)

Supported targets are `git-alias` and `lazygit`. (gitui is blocked upstream — see FUTURE_SPEC.md.)

Detection gating (FR-I2): a target whose tool is absent is still listed (DETECTED=✗) but `install`/`remove` for it prints a note and exits 1.

### `integrate install <target>…`

Install one or more stagehand integrations. Targets are explicit (at least one required; there is no "install all" default). Each target runs the no-mangle protocol (see below) independently. Multiple targets may be named; if any target fails (detection gate, install error, or unknown target), the remaining targets are still attempted (best-effort), and the command exits 1.

| Flag | Description |
|------|-------------|
| `--yes` | Skip the y/N confirmation prompt and apply changes directly (for scripts and CI) |

Detection gating (FR-I2): if a named target's tool is not on `$PATH`, the target is skipped with a note to stderr and marked as failed. `git-alias` requires only `git` (always present for stagehand); `lazygit` requires `lazygit` on `$PATH`.

Decline and no-change outcomes (user answered N, or the integration is already applied) are reported on stdout and are NOT errors (exit 0).

```bash
stagehand integrate install git-alias lazygit    # install both
stagehand integrate install --yes git-alias     # skip confirmation
```

### `integrate remove <target>…`

Remove one or more stagehand integrations. Same semantics as `install`: explicit targets, detection gating, best-effort batch, and `--yes` to skip confirmation.

```bash
stagehand integrate remove lazygit     # remove lazygit integration
stagehand integrate remove --yes git-alias lazygit
```

#### `git-alias` target

Registers `git stagehand` as a git alias in the **global** gitconfig (`git config --global alias.stagehand '!stagehand'`). After installation, `git stagehand` runs stagehand from any git repo — no PATH configuration needed.

The `.gitconfig` write is delegated to `git config` itself (FR-I4), so the no-mangle protocol (unified-diff preview, backup, re-parse validation) does **not** apply. Instead, git-alias shows the exact command and resulting usage, then asks for confirmation (same `y/N` / `--yes` mechanics).

| Flag | On | Description |
|------|----|-------------|
| `--alias-name <name>` | `install`, `remove` | Override the alias name (default: `stagehand`). Manages `alias.<name>` instead of `alias.stagehand`. |

**Conflicting alias behavior:**

- **Install:** if `alias.<name>` already exists with a value other than `!stagehand` (a *foreign* alias), the current value is shown in the preview with a warning. After confirmation, the alias is overwritten (outcome: *Updated*). Use `--yes` to skip the prompt.
- **Remove:** if the alias is foreign (not stagehand's), `remove` **refuses** to unset it and prints a note (outcome: *NoChange* — the alias is never silently removed). `remove` only unsets when the value is `!stagehand`.

**`integrate list` shows:**

- **DETECTED:** ✓ (git-alias needs only git, which is always present for stagehand)
- **STATUS:** `not installed` / `installed` / `foreign` (a conflicting alias exists at `alias.<name>`)
- **CONFIG:** the resolved global gitconfig path (`$GIT_CONFIG_GLOBAL` if set, else `$HOME/.gitconfig`)

```bash
stagehand integrate install git-alias        # install `git stagehand`
stagehand integrate install git-alias --yes   # skip confirmation
stagehand integrate install git-alias --alias-name ci   # install as `git ci`
stagehand integrate remove git-alias         # remove the alias
stagehand integrate remove git-alias --yes --alias-name ci  # remove `git ci`
```

#### No-mangle protocol

Every file edit by an integration runs the no-mangle protocol (PRD §9.21 FR-I3): a unified-diff preview is shown, the user is asked to confirm (`y/N`; use `--yes` to skip), a timestamped backup is written before modification, and the file is re-parsed after writing with automatic restore on validation failure. This guarantee is enforced by the protocol engine — it is not a convention each target follows independently. The `git-alias` target does **not** use this protocol (it delegates the write to `git config`). The `lazygit` target uses it for all edits.

#### `lazygit` target

Adds a `customCommands` entry to lazygit's `config.yml` via a **comment-preserving YAML round-trip** (`gopkg.in/yaml.v3` Node API). Press `<c-a>` in lazygit's files panel to run `stagehand` and generate an AI commit message — `output: 'none'` keeps you in the UI (US8).

```yaml
customCommands:
  - key: '<c-a>'                       # stagehand-integration
    context: 'files'
    command: 'stagehand'
    loadingText: 'Generating commit message…'
    output: 'none'
    description: 'stagehand: AI commit'
```

| Field | Default | Description |
|-------|---------|-------------|
| `key` | `<c-a>` | Key binding in lazygit |
| `context` | `files` | Panel context (files panel) |
| `command` | `stagehand` | Command to run |
| `loadingText` | `Generating commit message…` | Spinner text while running |
| `output` | `none` | Suppress output (stay in UI) |
| `description` | `stagehand: AI commit` | Menu description |

| Flag | On | Description |
|------|----|-------------|
| `--key <k>` | `install`, `remove` | Override the key binding (default: `<c-a>`). Remove targets the **marked stagehand entry** (by the `# stagehand-integration` marker), not by key — so you can install with `--key '<c-s>'` and remove with the default. |

**Config discovery order:**

1. `lazygit --print-config-dir` output + `/config.yml` (when lazygit is installed)
2. Platform default: `$XDG_CONFIG_HOME/lazygit/config.yml` (Linux), `~/Library/Application Support/lazygit/config.yml` (macOS), `%AppData%/lazygit/config.yml` (Windows)

**No-mangle behavior:** The full protocol applies: a unified-diff preview is shown before writing, a timestamped backup (`.stagehand-backup.<ts>`) is created for existing files, and the output is re-parsed after writing with automatic restore on validation failure. A corrupt `config.yml` is **hard-refused** — nothing is written, and the error is surfaced. Hand-maintained comments, other `customCommands` entries, and all other config blocks are preserved.

**Idempotency:** The entry is identified by its `# stagehand-integration` marker comment (not the key binding). Re-running `install` on an already-installed entry reports "No changes" (replace, never duplicate). `remove` deletes only the stagehand entry — other entries and config blocks are untouched.

**Conflicting key behavior:** Because `customCommands` is a YAML *sequence*, lazygit permits two entries to share a key binding. If an **unmarked** entry already binds your target key (e.g. `<c-a>`), `install` prints a `WARNING` to stderr noting that a duplicate `customCommands` entry will be created, then proceeds through the normal no-mangle preview/confirm flow (outcome: *Updated*). Use `--key '<other>'` to install under a different binding instead. (`integrate list` reports this pre-existing state as `foreign`.) Unlike the `git-alias` target — where a foreign alias is **overwritten** — the lazygit target cannot overwrite (a sequence key is not unique), so it **appends** and surfaces the resulting duplicate for you to resolve.

**`integrate list` shows:**

- **DETECTED:** ✓ if `lazygit` is on `$PATH`, ✗ otherwise
- **STATUS:** `not installed` / `installed` / `foreign` (an unmarked entry binds our key)
- **CONFIG:** the resolved `config.yml` path

```bash
stagehand integrate install lazygit              # install with default key (<c-a>)
stagehand integrate install lazygit --yes        # skip confirmation
stagehand integrate install lazygit --key '<c-s>' # custom binding
stagehand integrate remove lazygit               # remove stagehand entry
stagehand integrate remove lazygit --yes          # skip confirmation
```

### `models [<provider>]`

List the models reachable by a provider's CLI. Source of truth, in order:

1. **(a) Live list** — if the provider manifest defines a `list_models_command`, it is run as a subprocess (inherited env, bounded timeout) and its stdout is printed under a provider heading.
2. **(b) Curated table** — if the `list_models_command` is absent **or** the command fails (non-zero exit, timeout, or not found), Stagehand's curated per-role tier table (FR-D4) is printed, annotated with its verification date and a "consult `<command> --help`" hint.

Stagehand never makes an HTTP call to list models (§6.2 N2) — the agent CLI is the only model authority.

With no argument, the **resolved default provider** is shown. With `--all`, every **detected** provider (command on `$PATH`) is shown, one block at a time. An unknown or undetected named provider exits 1.

```bash
stagehand models                # show the default provider's models
stagehand models claude         # show claude's models
stagehand models --all          # show all detected providers
stagehand models --help          # see the models-scoped --all text
```

| Flag | Description |
|------|-------------|
| `--all`, `-a` | List models for every detected provider (default: the resolved default provider) |

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success (commit created, or dry-run message printed). |
| `1` | General error (generation failed, parse failed after retries, **provider command missing on `$PATH` (checked before the snapshot)**, CAS, usage). |
| `2` | Nothing to commit (clean tree after auto-stage, or nothing staged with `--no-auto-stage`). |
| `3` | Rescue condition (snapshot taken, commit not created — manual recovery printed). |
| `5` | Busy — another stagehand run holds the per-repo lock; retry after it finishes. |
| `124` | Timeout (generation exceeded `--timeout`). |

Exit codes mirror the constants in `internal/exitcode/exitcode.go`. A timeout is reported as `124` (matching GNU `timeout`), not `3`. With `--dry-run`, generation failures (timeout or parse/duplicate-check exhaustion) report exit **1** with a short stderr message (not 3/124 + the recovery recipe) — codes 3 and 124 remain the non-dry-run (commit-path) semantics.

Code `5` (Busy) is distinct from the commit-failure codes so scripts can tell "busy, retry" from "failed." Contention on the per-repo run lock (FR52) has two behaviors. On the single-commit path (changes staged): if a contending run's staged changes are already covered by the in-progress run's published index snapshot, it exits **0** ("nothing to do — an in-progress run already covers your staged changes"); if genuinely new work is staged, it exits **5** with the holder's pid/host and leaves the new changes staged for a re-run. On the decompose path (nothing staged, working tree dirty): an accidental double-run exits **5** (Busy) rather than 0 — the holder publishes a working-tree snapshot (`T_start`) that a lock-free contender cannot reproduce from the index alone, so it conservatively refuses. Stagehand never force-breaks the lock.

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
| `--exclude`, `-x` | — (no env var; deliberate — see [configuration.md](configuration.md)) | — (also `[generation].exclude` in config, UNIONS rather than overrides) |
| `--planner-provider` | `STAGEHAND_PLANNER_PROVIDER` | — |
| `--planner-model` | `STAGEHAND_PLANNER_MODEL` | — |
| `--stager-provider` | `STAGEHAND_STAGER_PROVIDER` | — |
| `--stager-model` | `STAGEHAND_STAGER_MODEL` | — |
| `--arbiter-provider` | `STAGEHAND_ARBITER_PROVIDER` | — |
| `--arbiter-model` | `STAGEHAND_ARBITER_MODEL` | — |
| `--format` | `STAGEHAND_FORMAT` | `stagehand.format` |
| `--locale` | `STAGEHAND_LOCALE` | `stagehand.locale` |
| `--template` | `STAGEHAND_TEMPLATE` | `stagehand.template` |
| `--reasoning` | `STAGEHAND_REASONING` | `stagehand.reasoning` |
| `--planner-reasoning` | `STAGEHAND_PLANNER_REASONING` | — |
| `--stager-reasoning` | `STAGEHAND_STAGER_REASONING` | — |
| `--message-provider` | `STAGEHAND_MESSAGE_PROVIDER` | — |
| `--message-model` | `STAGEHAND_MESSAGE_MODEL` | — |
| `--message-reasoning` | `STAGEHAND_MESSAGE_REASONING` | — |
| `--arbiter-reasoning` | `STAGEHAND_ARBITER_REASONING` | — |

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

# Use reasoning for deeper analysis (pi: --thinking, claude: --effort; others no-op)
stagehand --reasoning high

# Per-repo per-role config (.stagehand.toml)
# [role.planner]
# provider = "claude"
# model = "opus"
```
