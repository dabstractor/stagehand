# Providers reference (`stagehand providers`)

Stagehand can drive several AI coding agents. The `stagehand providers` command
tree is the discovery + debugging surface for those agents (PRD §15.3, FR46–FR48,
US10). It has two subcommands:

- **`stagehand providers list`** — list every built-in and user-defined provider,
  mark which ones are installed on `$PATH`, and show the resolved default
  provider and model (FR46).
- **stagehand providers show `<name>`** — print a provider's fully-resolved
  manifest (built-in field-merged with any user override) as TOML, so you can
  see the exact command stagehand will render (FR47).

> **Cross-reference.** The configuration keys that select the default provider
> and model, and the `[provider.<name>]` override table syntax, are documented in
> [CONFIGURATION.md](./CONFIGURATION.md). This document owns the *output format*
> of the `providers` commands themselves (the Mode A provider reference surface).

## `stagehand providers list` (FR46)

`providers list` resolves the active configuration (defaults → global config file
→ repo config file → repo git-config) and prints one row per provider in sorted
name order, marking each as detected or not detected on `$PATH`.

```
$ stagehand providers list
PROVIDER       STATUS         DEFAULT MODEL
claude          detected       sonnet
codex           detected       (unset)
cursor          detected       (unset)
gemini          detected       gemini-2.5-pro
opencode        detected       (unset)
pi              detected       glm-5-turbo (default)

default provider: pi (model: glm-5-turbo)
```

Each row carries:

| Column          | Meaning                                                                 |
| --------------- | ----------------------------------------------------------------------- |
| `PROVIDER`      | The provider name. A ` (default)` marker follows the resolved default.  |
| `STATUS`        | `detected` if the provider's executable is on `$PATH`, else `not detected`. |
| `DEFAULT MODEL` | The provider manifest's `default_model`, or `(unset)` when the manifest leaves it empty. |

The trailing line names the **resolved default provider and model**:

- The default **provider** is the configured `provider` value; if none is set,
  stagehand auto-resolves to the first `detected` provider in (sorted) list
  order. If nothing is detected, the line reads
  `default provider: (none detected)`.
- The default **model** is the configured `model` value; if none is set, the
  resolved provider's `default_model`. If that is also empty, the line reads
  `(model: (unset))`.

A provider that is **not installed** is still listed but marked `not detected`
and never chosen as the auto-resolved default.

> **Note.** The environment-variable and CLI-flag precedence layers
> (`STAGEHAND_PROVIDER`/`--provider`, `STAGEHAND_MODEL`/`--model`) are wired by
> the CLI layer. Until then, `providers list` reflects the file/git-config
> resolution only. See [CONFIGURATION.md](./CONFIGURATION.md) §1.

## `stagehand providers show <name>` (FR47)

`providers show <name>` prints the provider's **fully-resolved manifest** — the
built-in manifest with any `[provider.<name>]` user overrides field-merged onto
it (decisions.md §6) — as TOML. This is the exact manifest stagehand uses to
render the agent command, so it is the debugging surface for "why is stagehand
invoking the agent *this* way?" (US10).

```
$ stagehand providers show pi
name = 'pi'
detect = 'pi'
command = 'pi'
subcommand = []
prompt_delivery = 'stdin'
prompt_flag = ''
print_flag = '-p'
model_flag = '--model'
default_model = 'glm-5-turbo'
system_prompt_flag = '--system-prompt'
provider_flag = '--provider'
default_provider = ''
bare_flags = ['--no-tools', '--no-extensions', '--no-skills', '--no-prompt-templates', '--no-context-files', '--no-session']
output = 'raw'
json_field = ''
strip_code_fence = true
retry_instruction = 'Output ONLY the commit message. No preamble, no markdown, no quotes.'
```

The emitted TOML keys map 1:1 to the provider manifest fields (PRD §12.1):

| TOML key             | Field             | Meaning                                                              |
| -------------------- | ----------------- | -------------------------------------------------------------------- |
| `name`               | `name`            | Provider identifier (e.g. `pi`).                                     |
| `detect`             | `detect`          | Command looked up on `$PATH` to decide "installed" (falls back to `command`). |
| `command`            | `command`         | Executable to run.                                                   |
| `subcommand`         | `subcommand`      | Tokens inserted between `command` and the flags (e.g. opencode's `run`). |
| `prompt_delivery`    | `prompt_delivery` | How the prompt reaches the agent: `stdin`, `positional`, or `flag`.  |
| `prompt_flag`        | `prompt_flag`     | Flag carrying the prompt (used only when `prompt_delivery = "flag"`). |
| `print_flag`         | `print_flag`      | Flag that puts the agent into non-interactive "print and exit" mode. |
| `model_flag`         | `model_flag`      | Flag carrying the model name.                                        |
| `default_model`      | `default_model`   | Model used when the user specifies none.                             |
| `system_prompt_flag` | `system_prompt_flag` | Flag carrying the system prompt (empty → system prompt is prepended to the payload). |
| `provider_flag`      | `provider_flag`   | Flag selecting a sub-provider for multi-backend agents (e.g. pi's `--provider`). |
| `default_provider`   | `default_provider`| Sub-provider used when the user specifies none.                      |
| `bare_flags`         | `bare_flags`      | Flags appended verbatim to make the call tool-less/session-less/chrome-less. |
| `output`             | `output`          | How stdout is interpreted: `raw` or `json`.                          |
| `json_field`         | `json_field`      | Field extracted when `output = "json"`.                              |
| `strip_code_fence`   | `strip_code_fence`| Remove one layer of ``` / ~~~ fencing from stdout when true.         |
| `retry_instruction`  | `retry_instruction` | Prepended to the payload on a parse retry.                        |

An unknown name exits 1 with a pointer back to `providers list`:

```
$ stagehand providers show nope
Error: unknown provider "nope": run 'stagehand providers list' to see available providers
```

## Field-merge: overriding a provider (FR48)

A user override applies **field-by-field** onto the matching built-in
(decisions.md §6): setting only `default_model` leaves `bare_flags`,
`print_flag`, `model_flag`, `prompt_delivery`, and every other field intact. So
you can tune one knob without redeclaring the whole manifest. For example, with
this repo-local `.stagehand.toml`:

```toml
[provider.pi]
default_model = "my-overridden-model"
```

`providers show pi` reflects the override while preserving the built-in
`bare_flags`:

```
$ stagehand providers show pi
...
default_model = 'my-overridden-model'
bare_flags = ['--no-tools', '--no-extensions', '--no-skills', '--no-prompt-templates', '--no-context-files', '--no-session']
...
```

Slices and maps (`subcommand`, `bare_flags`, `env`) are replaced **wholesale**
when an override sets them — they are not appended or deep-merged. A
`[provider.<name>]` entry whose name does not match a built-in is added as a
brand-new provider used as-is. See [CONFIGURATION.md](./CONFIGURATION.md) §5 for
the override table syntax and the full precedence chain.

## Built-in providers

Stagehand ships six built-in providers, all verified against live `--help` on
2026-06-30 (see `plan/001_f1f80943ac34/architecture/external_deps.md` §B):

| Name      | Command   | Delivery   | Default model    |
| --------- | --------- | ---------- | ---------------- |
| `pi`      | `pi`      | `stdin`    | `glm-5-turbo`    |
| `claude`  | `claude`  | `stdin`    | `sonnet`         |
| `gemini`  | `gemini`  | `positional` | `gemini-2.5-pro` |
| `opencode`| `opencode`| `positional` | *(unset)*        |
| `codex`   | `codex`   | `stdin`    | *(unset)*        |
| `cursor`  | `agent`   | `positional` | *(unset)*        |

> **Roadmap.** The reference manifest files (`providers/*.toml`) will be emitted
> to disk by the M8.T1.S1 task; this document is the human-facing reference they
> will complement.
