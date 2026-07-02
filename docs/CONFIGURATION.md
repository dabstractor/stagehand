# Configuration reference

Stagehand resolves its settings through a layered precedence chain (PRD §16.1,
FR34). This document is the single source of truth for that chain: the order of
the layers, the environment variables, the `git config` keys, and the
`.stagehand.toml` file keys. The resolved value is produced by
[`config.Load`](../internal/config/load.go) and consumed read-only by every
downstream package.

> **Cross-reference.** The CLI flag reference (the `--provider`, `--model`,
> `--config`, `--timeout`, `--verbose`/`-v`, `--no-color` flags, plus the
> `--all`/`--no-auto-stage` action flags) is documented in this same file by
> the CLI task (P1.M7.T2.S1). This document owns everything *below* the CLI
> flag layer.

## 1. Precedence order (FR34)

Settings are resolved **lowest → highest**, and **higher wins**. At each layer,
only the fields the source actually sets override the layers below — an unset
field never clobbers a value from a lower layer. A field set to its zero value
(e.g. `model = ""` or `verbose = false`) **does** count as "set" and overrides
lower layers (the *present-but-zero* rule).

| #   | Layer                          | Source                                                                 |
| --- | ------------------------------ | ---------------------------------------------------------------------- |
| 1   | Built-in defaults              | `internal/config/defaults.go` (`Default()`)                            |
| 2   | Built-in provider manifests    | `internal/provider/builtin.go` (`Builtins()`) — injected by registry   |
| 3   | Global config file             | `$XDG_CONFIG_HOME/stagehand/config.toml` (default `~/.config/stagehand/config.toml`) |
| 4   | Per-repo config file           | `./.stagehand.toml`                                                     |
| 5   | Per-repo git config            | `stagehand.*` keys (read via `git config --get`)                        |
| 6   | Environment variables          | `STAGEHAND_*`                                                           |
| 7   | CLI flags                      | `--provider`, `--model`, `--config`, `--timeout`, `--verbose`, `--no-color` |

Notes:

- **Built-in provider manifests (layer 2)** are *not* layered into the scalar
  `Config`; they are injected by
  `provider.NewRegistry(provider.Builtins(), cfg.ProviderOverrides)`, which
  field-merges each user override onto its matching built-in (see
  [§5](#5-provider-override-field-merge)).
- **`--config` / `STAGEHAND_CONFIG`** (a layer-7/6 value) *overrides discovery*:
  when set, that single file **replaces** both the global-file (3) and repo-file
  (4) layers. The git-config (5), env (6), and flag (7) layers still apply
  normally. A CLI flag always beats its matching env var (flag > env).
- `auto_stage_all` and the generation caps (`max_diff_bytes`, `max_md_lines`,
  `max_duplicate_retries`, `subject_target_chars`, `output`,
  `strip_code_fence`) have **no** environment variable and **no** CLI flag —
  they are file/git-config only. `auto_stage_all` is additionally toggled at
  runtime by the `--all`/`--no-auto-stage` action flags in the CLI.

### Built-in defaults table

| Setting                 | Default        |
| ----------------------- | -------------- |
| `timeout`               | `120s`         |
| `auto_stage_all`        | `true`         |
| `max_diff_bytes`        | `300000`       |
| `max_md_lines`          | `100`          |
| `max_duplicate_retries` | `3`            |
| `output`                | `raw`          |
| `strip_code_fence`      | `true`         |
| `subject_target_chars`  | `50`           |

`provider` and `model` default to the empty string (resolved from each
manifest's `default_model`/the registry); `verbose` and `no_color` default to
`false`.

## 2. Environment variables (FR35)

There are exactly **six** `STAGEHAND_*` environment variables, each mapping to
a CLI flag of the same meaning. The CLI layer reads them into the `Env` layer
(FR34 layer 6); a CLI flag always overrides its env var.

| Variable              | CLI flag      | Maps to config field | Notes                                            |
| --------------------- | ------------- | -------------------- | ------------------------------------------------ |
| `STAGEHAND_PROVIDER`  | `--provider`  | `provider`           |                                                  |
| `STAGEHAND_MODEL`     | `--model`     | `model`              |                                                  |
| `STAGEHAND_TIMEOUT`   | `--timeout`   | `timeout`            | Parsed as a duration (`120s`, `90`).             |
| `STAGEHAND_CONFIG`    | `--config`    | —                    | Path to a config file (overrides discovery).     |
| `STAGEHAND_VERBOSE`   | `--verbose`   | `verbose`            |                                                  |
| `STAGEHAND_NO_COLOR`  | `--no-color`  | `no_color`           | Note the **underscore** in `NO_COLOR`.           |

## 3. Git-config keys (§16.3)

An alternative to a `.stagehand.toml` file for users who keep config with the
repo. Read with `git config --get stagehand.<key>`. **Keys are camelCase**, not
the snake_case used in the TOML file. Booleans must be read with
`git config --bool`. Git-config **cannot** express `[provider.<name>]` tables,
so there is no key for provider overrides here.

```ini
[stagehand]
    provider = pi
    model = glm-5.2
    timeout = 90
    autoStageAll = true
```

| Key                    | Type      | Maps to config field     |
| ---------------------- | --------- | ------------------------ |
| `stagehand.provider`   | string    | `provider`               |
| `stagehand.model`      | string    | `model`                  |
| `stagehand.timeout`    | duration  | `timeout` (`90` or `90s`) |
| `stagehand.autoStageAll` | bool (\*) | `auto_stage_all`       |
| `stagehand.verbose`    | bool (\*) | `verbose`                |
| `stagehand.noColor`    | bool (\*) | `no_color`               |
| `stagehand.maxDiffBytes` | int     | `max_diff_bytes`         |
| `stagehand.maxMdLines` | int       | `max_md_lines`           |
| `stagehand.maxDuplicateRetries` | int | `max_duplicate_retries` |
| `stagehand.subjectTargetChars` | int | `subject_target_chars`  |
| `stagehand.output`     | string    | `output` (`raw`/`json`)  |
| `stagehand.stripCodeFence` | bool (\*) | `strip_code_fence`   |

(\*) Read via `git config --bool`. This composes naturally with `git config
--local` vs `--global`.

## 4. `.stagehand.toml` keys (§16.2)

The TOML file splits scalars across two tables, `[defaults]` and `[generation]`,
and supports `[provider.<name>]` tables for overrides (see
[§5](#5-provider-override-field-merge)). The global file lives at
`$XDG_CONFIG_HOME/stagehand/config.toml` (default
`~/.config/stagehand/config.toml`); the per-repo file is `./.stagehand.toml`.

### `[defaults]`

| Key             | Type     | Notes                                        |
| --------------- | -------- | -------------------------------------------- |
| `provider`      | string   | Default agent.                               |
| `model`         | string   | `""` → use the manifest's `default_model`.   |
| `timeout`       | string   | TOML string (`"120s"`).                      |
| `auto_stage_all`| bool     |                                              |
| `verbose`       | bool     |                                              |
| `no_color`      | bool     |                                              |

### `[generation]`

| Key                     | Type   | Notes                          |
| ----------------------- | ------ | ------------------------------ |
| `max_diff_bytes`        | int    | Total staged-diff byte cap.    |
| `max_md_lines`          | int    | Per-markdown-file line cap.    |
| `max_duplicate_retries` | int    | Outer duplicate-retry budget.  |
| `output`                | string | `raw` \| `json`.               |
| `strip_code_fence`      | bool   |                                |
| `subject_target_chars`  | int    | Target subject-line length.    |

### `[provider.<name>]`

Overrides (or defines) a provider manifest. Keys are the snake_case form of the
`provider.Manifest` schema (PRD §12.1): `command`, `detect`, `subcommand`,
`prompt_delivery`, `prompt_flag`, `print_flag`, `model_flag`, `default_model`,
`system_prompt_flag`, `provider_flag`, `default_provider`, `bare_flags`,
`output`, `json_field`, `strip_code_fence`, `retry_instruction`, `env`.

### Golden example (§16.2)

```toml
# ~/.config/stagehand/config.toml

[defaults]
provider = "pi"            # default agent
model   = ""               # "" → use the manifest's default_model
timeout = "120s"
auto_stage_all = true
verbose = false

[generation]
max_diff_bytes      = 300000
max_md_lines        = 100
max_duplicate_retries = 3
output              = "raw"     # raw | json
strip_code_fence    = true
subject_target_chars = 50

# Override a built-in provider (field-merged with the built-in manifest).
[provider.pi]
default_model = "glm-5.2"
default_provider = "zai"

# Define a brand-new provider (§12.8).
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

## 5. Provider-override field-merge

When multiple `[provider.<name>]` tables appear across the global and repo
files, they are merged **per-key shallow**: a higher layer's whole
`[provider.<name>]` entry **replaces** a lower layer's same-named entry, while
*different*-named providers from lower layers survive. So a global
`[provider.alpha]` and a repo `[provider.beta]` both reach the registry, but a
repo `[provider.pi]` replaces a global `[provider.pi]`.

Each surviving override is then **field-merged onto its matching built-in
manifest** inside the registry (`NewRegistry`): an override that sets only
`default_model` leaves the built-in `command`, `bare_flags`, `print_flag`,
`model_flag`, `prompt_delivery`, etc. intact. An override naming a provider that
is not built-in is added as a brand-new provider, used as-is (§12.8). This
field-merge happens exactly once, over the built-in — two user overrides are
never field-merged together.

## 6. Repo-local config trust notice (§19)

A repo-local `.stagehand.toml` could be committed by an attacker to change a
user's provider — but it can only redirect commit generation to another
*installed* agent the user already trusts; it cannot exfiltrate credentials or
run arbitrary commands (manifests specify a `command` + flags, not arbitrary
shell). For visibility, stagehand prints a one-line notice to stderr when a
**repo-local** source (the repo `.stagehand.toml` file **or** a
`stagehand.provider` git-config key) sets the provider:

```
stagehand: repo-local config changed provider to <name>
```

`<name>` is the final resolved provider. The notice is **not** printed when only
the global file, an environment variable, a CLI flag, or an explicit
`--config`/`STAGEHAND_CONFIG` path sets the provider — those are user-chosen,
not attacker-committable. (Hardening planned for v1.1: restrict repo-local
overrides to non-`command` fields unless `STAGEHAND_TRUST_REPO_CONFIG=1`.)
