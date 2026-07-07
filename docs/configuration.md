# Configuration

Full reference for the Stagehand configuration system: precedence order, file format, environment variables, git-config keys, built-in defaults, and paths. Matches the shipped `config init` template and the Go source in `internal/config/`.

## Precedence

```text
CLI flags  >  STAGEHAND_* env vars  >  repo git config (stagehand.*)  >
repo-local .stagehand.toml  >  global config file  >  provider defaults  >  built-in defaults
```

From lowest to highest:

1. **Built-in defaults** — hardcoded in `config.Defaults()` (Layer 1).
2. **Provider defaults** — the manifest's `default_model`, `provider_flag`, etc. (Layer 2).
3. **Global config file** — `$XDG_CONFIG_HOME/stagehand/config.toml` (Layer 3).
4. **Repo-local `.stagehand.toml`** — `./.stagehand.toml` in the repo root (Layer 4).
5. **Repo git config** — `stagehand.*` keys in `.git/config` (Layer 5).
6. **`STAGEHAND_*` env vars** — environment variables (Layer 6).
7. **CLI flags** — command-line arguments (Layer 7 — highest).

When a `[provider.<name>]` section appears in a config file, its fields are **merged onto** the built-in manifest of the same name (field-by-field: present values override, absent values inherit).

> **`session_mode` override.** `session_mode` is one such overridable field. An explicit `session_mode = ""` on a provider that ships `"append"` (pi) **disables the multi-turn fallback** for that provider (the run proceeds one-shot → rescue, unchanged); omitting the key inherits the built-in `"append"`. Setting `session_mode = "append"` on a provider that ships `""` is a user override at their own FR-T9 verification risk — the shipped default stays `""` until a reproducible append-turn rendering is confirmed (see [providers.md](providers.md#the-schema) and §9.24).

## Config file paths

| Scope | Path | Notes |
|-------|------|-------|
| Global | `$XDG_CONFIG_HOME/stagehand/config.toml` (default `~/.config/stagehand/config.toml`) | Written by `stagehand config init`; read as Layer 3. |
| Repo-local | `./.stagehand.toml` | Gitignored; read as Layer 4; overrides global. |

Use `stagehand config path` to print the resolved config path (override-aware: honors `--config` / `STAGEHAND_CONFIG`, else the global path).

### Bootstrap (`config init`)

`stagehand config init` writes a **populated, working config** to the global path by default. It:

1. Runs cascading provider detection (highest-priority installed built-in, in order: pi, opencode, cursor, agy, gemini, qwen-code, codex, claude).
2. Writes `[defaults] provider = "<detected>"` and that provider's per-role model defaults UNCOMMENTED (from the FR-D4 table) — EXCEPT for **pi**, whose per-role models are left EMPTY (pi is a multi-backend provider; set the model with an inference-provider prefix, e.g. `model = "zai/glm-5.2"`, FR-R5b). Pi's shipped per-role models are blank so you supply your own backend/model.
3. Writes other installed providers as commented-out `[role.*]` blocks (one-line uncomment to route a role to a different agent).
4. If no agent is detected, defaults to `"pi"` with an annotation.

The written path is always printed on success.

| Flag | Description |
|------|-------------|
| `--provider <name>` | Target a specific built-in provider instead of auto-detecting. Unknown names exit 1. |
| `--force` | Overwrite an existing config file. |
| `--template` | Write the inert all-commented reference config (v1 behavior) instead of a populated bootstrap. |

If a config file already exists, it is NOT overwritten unless `--force` is passed (exit code 1). Parent directories are created as needed.

`config init --interactive` runs a TTY-gated wizard: it lists detected providers (FR-D1 default highlighted), shows each role's curated default (FR-D4) for accept-or-edit, and — for multi-backend providers (pi, opencode) — prompts for the `inference/model` prefix on edited models (FR-D2/FR-R5b) rather than guessing. It writes the **same file** as plain `config init`. Non-TTY stdin exits 1 pointing at plain `config init` (which stays non-interactive for post-install/first-run use, FR-B3). Composes with `--force` (overwrites) and `--provider <name>` (pre-selects); mutually exclusive with `--template`.

### Schema versioning (`config upgrade`)

`stagehand config upgrade` rewrites an existing config's top-level `config_version` line to the current schema version (3) in place — For multi-backend providers, the former `default_provider` is folded into a slash-prefix on the model and the key is deleted. Every other line is preserved. Idempotent: running it twice leaves the file unchanged. No flags.

```text
$ stagehand config upgrade
# Already at version 3 →  "Config at <path> is already at version 3 (no changes)."
# Upgraded from v1  →  "Upgraded config at <path> to version 3."
# No file          →  "no config file at <path> (run 'stagehand config init' first)"  (exit 1)
# Not valid TOML   →  "config <path> is not valid TOML: <err>"  (exit 1, file untouched)
```

At load time, if `config_version` is missing or older, stagehand prints an advisory to stderr pointing at `config upgrade` (or `config init --force` to regenerate). The current schema (version 3) includes per-role models, reasoning levels (FR-R6), the inference-provider model-prefix (FR-R5b), multi-commit decomposition, and binary filtering.

> [!NOTE]
> Point discovery at a specific file with `--config <path>` (or the `STAGEHAND_CONFIG` env var). It overrides global and repo-local file discovery and is honored by every command — including the default commit action **and the `config init`, `config path`, and `config upgrade` subcommands** (e.g. `stagehand --config X config upgrade` upgrades file `X`; `config path` prints the resolved path) — so a provider declared under `[provider.<name>]` in that file is usable with `--provider <name>` directly. A missing explicit path (typo'd `--config` or `STAGEHAND_CONFIG`) fails fast with exit 1; only the discovery default tolerates a missing global file.

## File format

The config file uses TOML with several section groups. By default, `config init` writes a **populated config** with the detected provider and per-role models UNCOMMENTED so the tool works immediately. Use `config init --template` to get the inert all-commented reference (every line commented out).

**Populated config** (default `config init` output):

```toml
config_version = 3

[defaults]
provider = "claude"
reasoning = "off"   # off|low|medium|high; off by default for every role (FR-R6)
# model          = ""
# timeout        = "120s"
# auto_stage_all = true
# verbose        = false

# --- per-role models for the default provider "claude" (PRD §16.4, §9.15) ---

[role.planner]
model = "opus"

[role.stager]
model = "sonnet"

[role.message]
model = "haiku"

[role.arbiter]
model = "sonnet"

# [generation] — diff capture and output tuning (commented defaults)
# [generation]
# max_diff_bytes        = 300000  # ignored when token_limit is set (FR3d)
# max_md_lines          = 100     # ignored when token_limit is set (FR3d)
# token_limit           = 0       # holistic token budget (0 = unset ⇒ use the caps above); FR3d
# diff_context          = 1       # 0 = changed-lines-only, 1 = one anchor (default), 3 = git default; FR3f; valid range 0–3 — out-of-range rejected at config load
# multi_turn_fallback     = true   # lossless multi-turn fallback on one-shot exhaustion (§9.24 FR-T1c); CANNOT disable via file (see "Multi-turn fallback" below)
# multi_turn_chunk_tokens = 32000  # per-turn chunk budget in tokens (§9.24 FR-T3); does NOT interact with token_limit (FR-T12)
# exclude               = []   # UNIONS across layers — see "Exclusion globs" below
# format                = "auto"   # auto|conventional|gitmoji|plain; unknown = hard error (exit 1)
# locale                = ""       # free-form language name or BCP-47 tag; never validated
# template              = ""       # wrap every message; must contain literal $msg, e.g. "$msg (#205)"
# hook_timeout          = "10m"    # per-hook execution timeout (§9.25 FR-V6); file + default only
# no_verify             = false    # skip pre-commit and commit-msg hooks (§9.25 FR-V5; mirrors `git commit --no-verify`); CANNOT disable-via-file is N/A (default is already false)
# ...
```

**Inert template** (`config init --template`): all lines commented out, including `[defaults]`, `[generation]`, `[provider.*]`, and `[role.*]` sections — documents every available option without changing any defaults.

## Built-in defaults

These are the values when no config file, env var, git-config key, or flag sets them:

| Option | Default | Source |
|--------|---------|--------|
| `provider` | `""` (auto-detect) | `config.Defaults()` |
| `model` | `""` (manifest `default_model`) | `config.Defaults()` |
| `timeout` | `120s` | `config.Defaults()` |
| `auto_stage_all` | `true` | `config.Defaults()` |
| `verbose` | `false` | `config.Defaults()` |
| `max_diff_bytes` | `300000` | `config.Defaults()` |
| `max_md_lines` | `100` | `config.Defaults()` |
| `token_limit` | `0` | `config.Defaults()` (§9.1 FR3d — unset ⇒ legacy caps) |
| `diff_context` | `1` | `config.Defaults()` (§9.1 FR3f — `-U1`; range 0–3, out-of-range rejected at config load) |
| `max_duplicate_retries` | `3` | `config.Defaults()` |
| `multi_turn_fallback` | `true` | `config.Defaults()` (§9.24 FR-T1c) |
| `multi_turn_chunk_tokens` | `32000` | `config.Defaults()` (§9.24 FR-T3) |
| `subject_target_chars` | `50` | `config.Defaults()` |
| `output` | `"raw"` | provider manifest (§12.1) |
| `strip_code_fence` | `true` | provider manifest (§12.1) |
| `format` | `"auto"` | `config.Defaults()` (§9.19 FR-F1) |
| `locale` | `""` | `config.Defaults()` (§9.19 FR-F6) |
| `template` | `""` | `config.Defaults()` (§9.19 FR-F8) |
| `push` | `false` | `config.Defaults()` (§9.22 FR-P1) |
| `hook_timeout` | `10m` | `config.Defaults()` (§9.25 FR-V6 — per-hook execution timeout; file + default only) |
| `no_verify` | `false` | `config.Defaults()` (§9.25 FR-V5 — skip pre-commit/commit-msg hooks; mirrors `git commit --no-verify`) |

`NoColor` is TTY-aware at runtime (set by the UI layer); it is not a file field and has no config-file key.

> **Hook execution knobs.** Two `[generation]` knobs control the §9.25 hook-execution surface (pre-commit / prepare-commit-msg / commit-msg / post-commit):
> - **`hook_timeout`** (default `10m`) — bounds each hook invocation so a wedged hook cannot hang a commit (§9.25 FR-V6). A duration string (e.g. `"30s"`, `"10m"`); malformed values fail at config load. **File + default only** (no env var, no flag, no git-config key) — set it in a config file.
> - **`no_verify`** (default `false`) — the `--no-verify` bypass (§9.25 FR-V5): when true, skips `pre-commit` and `commit-msg` hooks (`prepare-commit-msg` and `post-commit` still run). It resolves through the full 5-layer precedence (`--no-verify` / `STAGEHAND_NO_VERIFY` / `stagehand.noVerify` / `[generation].no_verify`). The `[generation].no_verify` file key uses the same only-true-propagates limitation as `push`: a file setting `no_verify = false` is a no-op (false is already the default); use the flag/env layers to set it false explicitly.

The `output` and `strip_code_fence` settings apply to **parsing** of agent output. Setting `output = "json"` makes Stagehand parse the agent's stdout as JSON (extracting the `json_field` value) across all providers. These `[generation]` values are an **opt-in override**: when `[generation]` (and git-config) omit them, the per-provider `[provider.<name>]` value is honored, falling back to the §12.1 manifest defaults (`output = "raw"`, `strip_code_fence = true`). Set `output = "json"` here only to force JSON parsing across ALL providers.

> **Token budget & diff context.** Two `[generation]` knobs size and shape the diff payload:
> - **`token_limit`** (default `0` = unset) — a holistic token budget over the **whole** agent payload (system prompt + style examples + the concatenated diff). When set (e.g. `120000`), Stagehand reserves room for the prompt/examples and truncates the diff to fit using the ≈4 chars/token estimate; after truncation it assembles the actual full prompt, re-measures it, and re-trims until it fits — a closed-loop guarantee (§9.1 FR3j) that the payload never exceeds `token_limit`. The payload always fits your model's context window **without Stagehand maintaining a per-model context registry** (§9.1 FR3d). A non-zero `token_limit` **supersedes** the legacy per-section caps `max_diff_bytes` and `max_md_lines` for that run; the two modes are mutually exclusive. When `0`/unset, the legacy caps apply unchanged.
> - **`diff_context`** (default `1`) — unchanged context lines surrounding each diff hunk: `0` = changed lines only (maximal savings), `1` = one anchor line (default), `3` = git's default (§9.1 FR3f). Applies in every diff path (staged, multi-commit snapshot, per-concept tree diff). Valid range is 0–3; an out-of-range value is rejected at config load with a clear error (§9.1 FR3f).

> **Multi-turn fallback.** Two `[generation]` knobs control the lossless multi-turn fallback path (§9.24), which activates only after the one-shot retry loop exhausts on a large diff:
> - **`multi_turn_fallback`** (default `true`) — enables the fallback. **Limitation:** because this is a default-`true` boolean that uses the same only-true-propagates file pattern as `auto_stage_all`, you **cannot disable it by setting `multi_turn_fallback = false` in a config file** in this revision — the `false` is silently ignored (the resolved value stays `true`). To effectively disable multi-turn for a provider, set `session_mode = ""` on that provider (see [providers.md](providers.md#the-schema)); the shipped pi default is `"append"`.
> - **`multi_turn_chunk_tokens`** (default `32000`) — the per-request chunk size (tokens est.) the large diff is split into for multi-turn priming. **This does NOT interact with `token_limit`** (§9.24 FR-T12): `token_limit` truncates the one-shot payload, while multi-turn deliberately uses the **untruncated** payload, delivered in request-sized pieces — the two never compose for a single message.

## Environment variables

All `STAGEHAND_*` variables override the config file and are overridden by CLI flags:

| Variable | Mirrors flag | Description | Example |
|----------|-------------|-------------|---------|
| `STAGEHAND_PROVIDER` | `--provider` | Default provider/agent | `STAGEHAND_PROVIDER=claude stagehand` |
| `STAGEHAND_MODEL` | `--model` | Model override | `STAGEHAND_MODEL=sonnet stagehand` |
| `STAGEHAND_TIMEOUT` | `--timeout` | Generation timeout | `STAGEHAND_TIMEOUT=60s stagehand` |
| `STAGEHAND_CONFIG` | `--config` | Config file path | `STAGEHAND_CONFIG=./alt.toml stagehand` |
| `STAGEHAND_VERBOSE` | `--verbose` | Print resolved command and output | `STAGEHAND_VERBOSE=true stagehand` |
| `STAGEHAND_NO_COLOR` | `--no-color` | Disable color | `STAGEHAND_NO_COLOR=true stagehand` |
| `NO_COLOR` | `--no-color` | Universal color-disable (honored when set) | `NO_COLOR=1 stagehand` |
| `STAGEHAND_COMMITS` | `--commits` | Force N commits (0=auto, 1≡single) | `STAGEHAND_COMMITS=3 stagehand` |
| `STAGEHAND_PLANNER_PROVIDER` | `--planner-provider` | Per-role: planner provider | `STAGEHAND_PLANNER_PROVIDER=claude stagehand` |
| `STAGEHAND_PLANNER_MODEL` | `--planner-model` | Per-role: planner model | `STAGEHAND_PLANNER_MODEL=opus stagehand` |
| `STAGEHAND_STAGER_PROVIDER` | `--stager-provider` | Per-role: stager provider | `STAGEHAND_STAGER_PROVIDER=pi stagehand` |
| `STAGEHAND_STAGER_MODEL` | `--stager-model` | Per-role: stager model | `STAGEHAND_STAGER_MODEL=gpt-5.4-mini stagehand` |
| `STAGEHAND_MESSAGE_PROVIDER` | `--message-provider` | Per-role: message provider (env + config only) | `STAGEHAND_MESSAGE_PROVIDER=claude stagehand` |
| `STAGEHAND_MESSAGE_MODEL` | `--message-model` | Per-role: message model (env + config only) | `STAGEHAND_MESSAGE_MODEL=haiku stagehand` |
| `STAGEHAND_ARBITER_PROVIDER` | `--arbiter-provider` | Per-role: arbiter provider | `STAGEHAND_ARBITER_PROVIDER=claude stagehand` |
| `STAGEHAND_ARBITER_MODEL` | `--arbiter-model` | Per-role: arbiter model | `STAGEHAND_ARBITER_MODEL=sonnet stagehand` |
| `STAGEHAND_REASONING` | `--reasoning` | Global reasoning effort: off\|low\|medium\|high | `STAGEHAND_REASONING=high stagehand` |
| `STAGEHAND_PLANNER_REASONING` | `--planner-reasoning` | Per-role: planner reasoning | `STAGEHAND_PLANNER_REASONING=high stagehand` |
| `STAGEHAND_STAGER_REASONING` | `--stager-reasoning` | Per-role: stager reasoning | `STAGEHAND_STAGER_REASONING=low stagehand` |
| `STAGEHAND_MESSAGE_REASONING` | `--message-reasoning` | Per-role: message reasoning | `STAGEHAND_MESSAGE_REASONING=low stagehand` |
| `STAGEHAND_ARBITER_REASONING` | `--arbiter-reasoning` | Per-role: arbiter reasoning | `STAGEHAND_ARBITER_REASONING=low stagehand` |
| `STAGEHAND_FORMAT` | `--format` | Message format (auto\|conventional\|gitmoji\|plain; unknown = hard error) | `STAGEHAND_FORMAT=conventional stagehand` |
| `STAGEHAND_LOCALE` | `--locale` | Message language (free-form; never validated) | `STAGEHAND_LOCALE=ja stagehand` |
| `STAGEHAND_TEMPLATE` | `--template` | Message template; `$msg` = generated message; must contain `$msg` (hard error) | `STAGEHAND_TEMPLATE='$msg (#205)' stagehand` |
| `STAGEHAND_PUSH` | `--push` | Run `git push` after a fully-successful run (true = push; false = disable); on failure commits stand, exit 1 | `STAGEHAND_PUSH=1 stagehand` |

## Git-config keys

These keys live in `.git/config` (set with `git config --local` or `git config --global`):

```ini
[stagehand]
    provider = pi
    model = glm-5.2
    timeout = 120s
    auto_stage_all = true
```

| Key | Type | Reads with | Description |
|-----|------|-----------|-------------|
| `stagehand.provider` | string | `git config --get stagehand.provider` | Default provider |
| `stagehand.model` | string | `git config --get stagehand.model` | Model override |
| `stagehand.timeout` | string | `git config --get stagehand.timeout` | Generation timeout (duration string) |
| `stagehand.auto_stage_all` | bool | `git config --get --bool stagehand.auto_stage_all` | Auto-stage all when nothing staged |
| `stagehand.output` | string | `git config --get stagehand.output` | Agent output mode: `raw` \| `json` (overrides per-provider default) |
| `stagehand.stripCodeFence` | bool | `git config --get --bool stagehand.stripCodeFence` | Strip ``` fences from agent output (overrides per-provider default) |
| `stagehand.tokenLimit` | int | `git config --get stagehand.tokenLimit` | Holistic token budget for the whole payload; `0` = unset ⇒ legacy `max_diff_bytes`/`max_md_lines` caps (§9.1 FR3d). Supersedes both legacy caps when >0 (mutually exclusive). |
| `stagehand.diffContext` | int | `git config --get stagehand.diffContext` | Unchanged context lines per hunk: `0` = changed-lines-only, `1` = one anchor line (default), `3` = git default (§9.1 FR3f). An explicit `0` is honored (changed-lines-only is a first-class value). |
| `stagehand.format` | string | `git config --get stagehand.format` | Message format: `auto` \| `conventional` \| `gitmoji` \| `plain`. Unknown = hard error (exit 1). |
| `stagehand.locale` | string | `git config --get stagehand.locale` | Message language (free-form name or BCP-47 tag; never validated). |
| `stagehand.template` | string | `git config --get stagehand.template` | Message template; the literal `$msg` is replaced with the generated message. Must contain `$msg` (hard error, exit 1). |
| `stagehand.push` | bool | `git config --get --bool stagehand.push` | Run `git push` after a fully-successful run (§9.22 FR-P1). On failure the commits stand — git's stderr is shown verbatim, "commits created; push failed" prints, exit 1. |

> [!NOTE]
> The git-config layer has **no** per-role keys (`stagehand.role.*`), no `stagehand.commits`, and no `stagehand.max_commits`. Per-role configuration is available via CLI flags (`--planner-provider`, etc.), env vars (`STAGEHAND_PLANNER_*`), and config-file `[role.*]` blocks only. Decompose settings (`--commits`, `--single`, `--no-decompose`) are flag/env only; `--max-commits` also reads from the `[generation]` config-file section. There is also no `stagehand.exclude` git-config key and no `STAGEHAND_EXCLUDE` env var (deliberate — see [Exclusion globs](#exclusion-globs-generationexclude) below); exclusions are config-file + `--exclude`/`-x` only.

### Decompose config keys

| Setting | Flag | Env var | Config file | Default | Notes |
|---------|------|---------|-------------|---------|-------|
| Commit count | `--commits <N>` | `STAGEHAND_COMMITS` | — | `0` (auto) | 0=auto-decompose; ≥2=force N; 1≡`--single` |
| Single-commit | `--single` / `--no-decompose` | — | — | `false` | Bypass decompose → v1 single-commit |
| Max commits | `--max-commits <N>` | — | `[generation].max_commits` | `12` | Safety cap on auto-decompose count |

Per-role provider/model overrides (flag > env > `[role.<role>]` config > `[defaults]` > built-in): see [providers.md](providers.md#per-role-default-models-fr-d4) for the compiled-in defaults per provider. Every role (including message) exposes `--<role>-provider`/`--<role>-model`/`--<role>-reasoning` (FR-R3).

### Exclusion globs (`[generation].exclude`)

```toml
[generation]
exclude = ["*.min.js", "dist/*"]
```

`[generation].exclude` (config file, both global and repo-local) and the repeatable `--exclude <glob>` / `-x <glob>` CLI flag (§9.18 FR-X1) exclude matching files' **diff content** from the agent payload — a placeholder line stands in for the diff; the file is still captured and committed normally. Patterns are gitignore-style globs.

> [!IMPORTANT]
> This is the **one setting in the whole precedence system that UNIONS instead of overriding** (§16.1). Every other list-valued key (e.g. `[generation].binary_extensions`) REPLACES across layers — a higher layer's list wins outright. `exclude` instead **accumulates**: the resolved set is the global file's globs, followed by the repo file's globs, followed by every `--exclude`/`-x` occurrence, in that order. A repo cannot use its local config to un-exclude a glob a user set globally.
>
> There is deliberately **no** `STAGEHAND_EXCLUDE` environment variable and **no** `stagehand.exclude` git-config key — a colon/comma-joined env list is a well-known quoting trap for glob patterns containing those characters. Use the config file for persistent excludes and `--exclude`/`-x` for ad-hoc ones.

### `.stagehandignore`

A repo can place a `.stagehandignore` file at its root (alongside `.stagehand.toml`) containing one gitignore-style glob per line (§9.18 FR-X1b, FR-X2). Blank lines and `#` comment lines are ignored. The globs are **unioned** with `[generation].exclude` and `--exclude`/`-x` (see [Exclusion globs](#exclusion-globs-generationexclude) above).

> [!WARNING]
> **Negation (`!`) is NOT supported.** Git pathspec excludes have no re-include mechanism — a `!` line is silently skipped with a `--verbose` warning. This is intentional: the translated `:(exclude,glob)` pathspecs cannot un-exclude.

A missing `.stagehandignore` is a no-op (no warning, no error).

## Lock file location

The per-repo run lock (FR52) is stored outside the repository to avoid polluting `git status`, being committable, or being ambiguous across worktrees. The lock file location resolves in this order:

1. `$XDG_RUNTIME_DIR/stagehand/locks/<hash>.lock` — when `XDG_RUNTIME_DIR` is set and absolute
2. `$XDG_CACHE_HOME/stagehand/locks/<hash>.lock` — when `XDG_CACHE_HOME` is set and absolute
3. `~/.cache/stagehand/locks/<hash>.lock` — fallback via `os.UserHomeDir()`

Where `<hash>` is the sha256 hex digest of the repo's canonical absolute path (resolved via `filepath.EvalSymlinks` to handle symlinked paths). Relative XDG values are ignored (only absolute paths are honored). If no resolution path exists, stagehand exits with an error — it never falls back to the current working directory or the repo itself.

**Exclusions are payload-only:** excluded files are hidden from what the agent sees but are still captured and committed normally.

Example:

```
# .stagehandignore
*.min.js          # any-depth
/dist/            # root dist/ dir contents only
vendor/
```
