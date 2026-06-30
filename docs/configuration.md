# Configuration

Full reference for the Stagehand configuration system: precedence order, file format, environment variables, git-config keys, built-in defaults, and paths. Matches the shipped `config init` template and the Go source in `internal/config/`.

## Precedence

```text
CLI flags  >  STAGEHAND_* env vars  >  repo git config (stagehand.*)  >
repo-local .stagehand.toml  >  global config file  >  provider defaults  >  built-in defaults
```

From lowest to highest:

1. **Built-in defaults** — hardcoded in `config.Defaults()` (Layer 1).
2. **Provider defaults** — the manifest's `default_model`, `default_provider`, etc. (Layer 2).
3. **Global config file** — `$XDG_CONFIG_HOME/stagehand/config.toml` (Layer 3).
4. **Repo-local `.stagehand.toml`** — `./.stagehand.toml` in the repo root (Layer 4).
5. **Repo git config** — `stagehand.*` keys in `.git/config` (Layer 5).
6. **`STAGEHAND_*` env vars** — environment variables (Layer 6).
7. **CLI flags** — command-line arguments (Layer 7 — highest).

When a `[provider.<name>]` section appears in a config file, its fields are **merged onto** the built-in manifest of the same name (field-by-field: present values override, absent values inherit).

## Config file paths

| Scope | Path | Notes |
|-------|------|-------|
| Global | `$XDG_CONFIG_HOME/stagehand/config.toml` (default `~/.config/stagehand/config.toml`) | Written by `stagehand config init`; read as Layer 3. |
| Repo-local | `./.stagehand.toml` | Gitignored; read as Layer 4; overrides global. |

Use `stagehand config path` to print the resolved global path. Use `stagehand config init` to write a fully-commented example to the global path.

> [!NOTE]
> Point discovery at a specific file with `--config <path>` (or the `STAGEHAND_CONFIG` env var). It overrides global and repo-local file discovery and is honored by every command — including the default commit action — so a provider declared under `[provider.<name>]` in that file is usable with `--provider <name>` directly. A missing explicit path (typo'd `--config` or `STAGEHAND_CONFIG`) fails fast with exit 1; only the discovery default tolerates a missing global file.

## File format

The config file uses TOML with three section groups. Every line in the `config init` template is commented out, so the file is inert until you uncomment lines you want to use:

```toml
# [defaults] — top-level Stagehand behavior
[defaults]
# provider       = "pi"
# model          = ""
# timeout        = "120s"
# auto_stage_all = true
# verbose        = false

# [generation] — diff capture and output tuning
[generation]
# max_diff_bytes        = 300000
# max_md_lines          = 100
# max_duplicate_retries = 3
# subject_target_chars  = 50
# output                = "raw"
# strip_code_fence      = true

# [provider.<name>] — override a built-in or define a new provider
# [provider.pi]
# default_model    = "glm-5.2"
# default_provider = "zai"
```

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
| `max_duplicate_retries` | `3` | `config.Defaults()` |
| `subject_target_chars` | `50` | `config.Defaults()` |
| `output` | `"raw"` | `config.Defaults()` |
| `strip_code_fence` | `true` | `config.Defaults()` |

`NoColor` is TTY-aware at runtime (set by the UI layer); it is not a file field and has no config-file key.

The `output` and `strip_code_fence` settings apply to **parsing** of agent output. Setting `output = "json"` makes Stagehand parse the agent's stdout as JSON (extracting the `json_field` value) across all providers. These `[generation]` values override any per-provider `[provider.<name>]` defaults — the broader layer wins.

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
