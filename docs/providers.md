# Provider manifests

Full reference for Stagehand's provider manifest system: the 18-field schema, command-rendering algorithm, the 6 built-in providers, the tools-disable asymmetry, adding a new agent, and output parsing. Matches the Go source in `internal/provider/` and the shipped `providers/*.toml` files.

## What a manifest is

A manifest describes one AI provider's CLI interface — how to invoke it, deliver the prompt, and parse its output. Six providers are compiled in as built-ins (zero config needed). Users can override built-in fields or define brand-new providers via `[provider.<name>]` sections in their config file.

See the [shipped `providers/*.toml` files](../providers/) for human-readable reference manifests — `providers/pi.toml` is the cleanest template.

## The schema

Each manifest has 18 fields (matching the TOML tags in `internal/provider/manifest.go`):

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `name` | string | (required) | Provider identity; set from the `[provider.<name>]` table key. |
| `detect` | string | `command` | Binary to probe on `$PATH` for auto-detection. |
| `command` | string | (required) | The executable to run. |
| `subcommand` | list of string | `[]` (none) | Inserted between command and flags (e.g. `["run"]`, `["exec"]`). |
| `prompt_delivery` | string | `"stdin"` | How to deliver the prompt: `stdin`, `positional`, or `flag`. |
| `prompt_flag` | string | `""` | Flag used when `prompt_delivery` is `"flag"`. |
| `print_flag` | string | `""` | Non-interactive print-mode flag (always appended last, after `bare_flags`). |
| `model_flag` | string | `""` | Flag for model selection (e.g. `"--model"`, `"-m"`). |
| `default_model` | string | `""` | Model used when the user specifies none. |
| `system_prompt_flag` | string | `""` | Flag for the system prompt. When `""`, the system prompt is prepended to the payload instead. |
| `provider_flag` | string | `""` | Flag for sub-provider selection (e.g. `"--provider"`). |
| `default_provider` | string | `""` | Default sub-provider when the user specifies none. |
| `bare_flags` | list of string | `[]` (none) | Extra flags appended verbatim before `print_flag`. |
| `output` | string | `"raw"` | Agent output mode: `"raw"` or `"json"`. |
| `json_field` | string | `""` | Field to extract when `output` is `"json"`. |
| `strip_code_fence` | bool | `true` | Strip one layer of `` ``` `` / `~~~` fences from agent output. |
| `retry_instruction` | string | `"Output ONLY the commit message. No preamble, no markdown, no quotes."` | Prepended to the payload on a parse-failure retry. |
| `env` | table | `nil` (none) | Environment variables set only for the subprocess (as `KEY=VAL`). |

## Command rendering

The renderer assembles the command invocation from the manifest fields and the resolved model, provider, system prompt, and user payload. Token order (per PRD §12.2):

```text
args = [subcommand...]
+ (provider_flag, provider)           if provider_flag != "" && provider != ""
+ (model_flag,    model)              if model_flag    != "" && model    != ""
+ (system_prompt_flag, system_prompt)  if system_prompt_flag != "" && system_prompt != ""
+ bare_flags...
+ print_flag                          if print_flag != ""       (always LAST)
+ payload                             per prompt_delivery:
    stdin     → piped to stdin
    positional → trailing argument
    flag      → (prompt_flag, payload)
```

When `system_prompt_flag` is empty, the system prompt is **prepended** to the payload (delimited by `\n\n`) instead of being delivered via a flag.

## The 6 built-in providers

Auto-detection order (first installed = default): **pi**, claude, gemini, opencode, codex, cursor. User-defined providers are never auto-selected.

| Provider | Delivery | Print flag | Model flag | Default model | System prompt flag | Tool-disable approach |
|----------|----------|-----------|-----------|----------------|-------------------|----------------------|
| `pi` | stdin | `-p` | `--model` | `glm-5-turbo` | `--system-prompt` | Explicit `--no-*` flags |
| `claude` | stdin | `-p` | `--model` | `sonnet` | `--system-prompt` | Explicit `--tools ""` + settings flags |
| `gemini` | stdin | (none) | `-m` | `gemini-2.5-pro` | (prepended) | Read-only constraint (`--approval-mode default`) |
| `opencode` | positional | (none) | `-m` | (user must set) | (prepended) | Read-only constraint (`run` subcommand) |
| `codex` | stdin | (none) | `-m` | (user must set) | (prepended) | Read-only constraint (`--sandbox read-only --ephemeral`) |
| `cursor` | positional | `-p` | `--model` | (user must set) | (prepended) | Read-only constraint (`--mode ask --trust`) |

Note: cursor is the only provider where `detect` and `command` differ from `name` — the binary is `agent`, not `cursor`.

## Tools-disable asymmetry

The six providers achieve tool-safety via two distinct mechanisms (PRD §12.7.1):

- **Explicit switch** (pi, claude): The manifest passes literal flags that **disable tools** (pi: `--no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files --no-session`; claude: `--tools "" --setting-sources "" --no-session-persistence`). This is the cleanest approach — the agent runs as a pure text-in/text-out process.

- **Read-only constraint** (codex, cursor, gemini): The manifest passes flags that **constrain the agent to a read-only, never-ask profile** (codex: `--sandbox read-only --ephemeral`; cursor: `--mode ask --trust`; gemini: `--approval-mode default`). opencode's `run` subcommand is inherently non-interactive and read-only.

Both approaches satisfy the §18.1 safety invariant: no provider can mutate the repository.

## Adding a new agent

Define a `[provider.<name>]` block in your config file (global or repo-local). You only need to set the fields that differ from the defaults — omitted fields inherit the built-in values (for a known name) or the schema defaults.

Example — add a provider called `myagent`:

```toml
[provider.myagent]
command            = "/opt/myagent/bin/agent"
prompt_delivery    = "stdin"
print_flag         = "--once"
model_flag         = "--model"
default_model      = "my-model-7b"
system_prompt_flag = "--system"
bare_flags         = ["--no-mcp", "--ephemeral"]
output             = "raw"
```

Verify the merged manifest:

```bash
stagehand providers show myagent
```

Use it:

```bash
stagehand --provider myagent
```

## Output parsing

The output parser processes the agent's stdout in five steps (PRD §12.9):

1. **Trim** — remove leading and trailing whitespace.
2. **Strip code fence** (if `strip_code_fence` is true) — remove a leading `` ``` `` or `~~~` opener line and everything from the last matching closer onward.
3. **Mode switch**:
   - `raw` — the trimmed output is the commit message.
   - `json` — parse as JSON and extract `json_field`; on failure, try a brace-balanced substring; on any failure, fall back to raw.
4. **Normalize newlines** — `\r\n` → `\n`; collapse 3+ consecutive `\n` to 2.
5. **Final trim** — if the result is empty, the orchestrator retries with `retry_instruction` prepended.

The v1 default is `output = "raw"` — the agent's stdout, after cleanup, is the commit message verbatim.
