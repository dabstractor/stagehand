# Provider manifests

Full reference for Stagecoach's provider manifest system: the 22-field schema, command-rendering algorithm, the 7 built-in providers, the tools-disable asymmetry, adding a new agent, and output parsing. Matches the Go source in `internal/provider/` and the shipped `providers/*.toml` files.

## What a manifest is

A manifest describes one AI provider's CLI interface — how to invoke it, deliver the prompt, and parse its output. Seven providers are compiled in as built-ins (zero config needed). Users can override built-in fields or define brand-new providers via `[provider.<name>]` sections in their config file.

See the [shipped `providers/*.toml` files](../providers/) for human-readable reference manifests — `providers/pi.toml` is the cleanest template.

## The schema

Each manifest has 22 fields (matching the TOML tags in `internal/provider/manifest.go`):

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `name` | string | (required) | Provider identity; set from the `[provider.<name>]` table key. |
| `detect` | string | `command` | Binary to probe on `$PATH` for auto-detection. |
| `command` | string | (required) | The executable to run. |
| `list_models_command` | list of string | `[]` (none) | Full argv that asks the agent CLI to list its reachable models (e.g. `["opencode", "models"]`), used by `stagecoach models`. Empty/nil ⇒ stagecoach prints its curated per-role tier table instead (FR-L1). Populated only for providers whose CLI exposes a verified listing (opencode, pi, agy, cursor); never an HTTP call (§6.2 N2). |
| `subcommand` | list of string | `[]` (none) | Inserted between command and flags (e.g. `["run"]`, `["exec"]`). |
| `prompt_delivery` | string | `"stdin"` | How to deliver the prompt: `stdin`, `positional`, or `flag`. |
| `prompt_flag` | string | `""` | Flag used when `prompt_delivery` is `"flag"`. |
| `print_flag` | string | `""` | Non-interactive print-mode flag (always appended last, after `bare_flags`). |
| `model_flag` | string | `""` | Flag for model selection (e.g. `"--model"`, `"-m"`). |
| `default_model` | string | `""` | Model used when the user specifies none. |
| `system_prompt_flag` | string | `""` | Flag for the system prompt. When `""`, the system prompt is prepended to the payload instead. |
| `provider_flag` | string | `""` | Flag for sub-provider selection (e.g. `"--provider"`). |
| `session_mode` | string | `""` | Multi-turn fallback capability (§9.24). `""` (default) = the provider cannot append turns across one-shot calls → multi-turn unavailable; `"append"` = re-invoking the same session id appends a recallable turn. **Only pi ships `"append"`** (VERIFIED 2026-07-05; FR-T9). Requires a verified, reproducible append-turn rendering — see below. |
| `bare_flags` | list of string | `[]` (none) | Extra flags appended verbatim before `print_flag` in bare mode. |
| `tooled_flags` | list of string | `nil` (none) | Flags for tooled/stager mode — tools ON, git-scoped, non-interactive. `nil`/empty ⇒ not stager-capable. |
| `output` | string | `"raw"` | Agent output mode: `"raw"` or `"json"`. |
| `json_field` | string | `""` | Field to extract when `output` is `"json"`. |
| `strip_code_fence` | bool | `true` | Strip one layer of `` ``` `` / `~~~` fences from agent output. |
| `retry_instruction` | string | `"Output ONLY the commit message. No preamble, no markdown, no quotes."` | Prepended to the payload on a parse-failure retry. |
| `env` | table | `nil` (none) | Environment variables set only for the subprocess (as `KEY=VAL`). |
| `reasoning_levels` | table | nil (none) | Per-level reasoning-effort token lists (off/low/medium/high); nil/empty ⇒ graceful no-op (FR-R6). Appended after the model flag at render. pi populates high/medium/low via `--thinking` (verified `pi --help`); claude via `--effort` (verified `claude --help`); all other built-ins are nil (graceful no-op). |
| `experimental` | bool | false | Marks a provider experimental (agy, qwen-code) — surfaced in `providers list`/`show`. Absent/false ⇒ stable. |

### Multi-turn capability (`session_mode`)

A provider supports Stagecoach's **lossless multi-turn fallback** (§9.24 — used when a one-shot generation repeatedly fails on a diff too large for a single reliable request) if and only if re-invoking the SAME session id appends a turn the model can recall. The `session_mode` manifest field declares this:

- `"append"` — re-invoking the same session id appends a recallable turn (multi-turn available).
- `""` (default) — the provider cannot append turns across one-shot calls (multi-turn unavailable; the run proceeds one-shot → rescue, unchanged).

**Only `pi` ships `session_mode = "append"` today** — VERIFIED 2026-07-05 via a live run (`pi --session-id X <isolation-flags-minus-no-session> -p "remember BANANA"`, then a same-`--session-id` recall turn returning "BANANA"). Every other built-in (claude, opencode, codex, cursor, agy, qwen-code) ships `""`.

**FR-T9 verification bar.** A manifest MUST NOT declare `"append"` speculatively. Setting it requires a verified, reproducible append-turn rendering — the exact flag set confirmed per provider (analogous to FR-D5's model-token verification duty). Until a provider's append mechanism is verified, its `session_mode` stays `""` and multi-turn is silently skipped for it. See §9.24 (FR-T8/FR-T9) for the full contract.

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

In **tooled mode** (the stager role), `tooled_flags` replaces `bare_flags`; tooled mode with empty `tooled_flags` errors — that provider cannot serve as a stager.

For a multi-backend provider (one whose manifest sets `provider_flag` — pi today), the model is `inference/model` (e.g. `zai/glm-5.2`): Render splits it on the first `/` and emits `--provider <prefix> --model <rest>` (FR-R5b). A model with no `/` on such a provider is a HARD configuration error, never a silent bare `--model`. Single-backend providers take the model verbatim. When a `reasoning` level resolves to a non-empty token list in `reasoning_levels`, those tokens are appended after the model flag (FR-R6); absent/empty ⇒ silent no-op.

## The 7 built-in providers

Auto-detection order (first installed = default): **pi, opencode, cursor, agy, qwen-code, codex, claude**. User-defined providers are never auto-selected.

| Provider | Delivery | Print flag | Model flag | Default model | System prompt flag | Tool-disable approach | Chrome-disable | Stager? |
|----------|----------|-----------|-----------|----------------|-------------------|----------------------|----------------|--------|
| `pi` | stdin | `-p` | `--model` | "" (user must set) | `--system-prompt` | Explicit `--no-*` flags | extensions/skills/templates/context off (`--no-*`); MCP use suppressed (servers may connect — tracked limitation) | ✓ yes |
| `claude` | stdin | `-p` | `--model` | `sonnet` | `--system-prompt` | Explicit `--tools ""` + settings flags | via `--tools ""` + `--setting-sources ""` | ✓ yes |
| `opencode` | positional | (none) | `-m` | (user must set) | (prepended) | Read-only constraint (`run` subcommand) | no per-surface switch; read-only by design — documented limitation | — no |
| `codex` | stdin | (none) | `-m` | (user must set) | (prepended) | Read-only constraint (`--sandbox read-only --ephemeral`) | no per-surface switch; read-only constraint only — documented limitation | — no |
| `cursor` | positional | `-p` | `--model` | (user must set) | (prepended) | Read-only constraint (`--mode ask --trust`) | no per-surface switch; read-only constraint only — documented limitation | — no |
| `agy` | stdin | (none) | `--model` | `Gemini 3.5 Flash (Low)` | (prepended) | Read-only constraint (`--mode plan`) | no per-surface switch; read-only constraint only — documented limitation | — no |
| `qwen-code` | stdin | `-p` | `-m` | `qwen3-coder-plus` ⚠️ | (prepended) | Read-only constraint (`--approval-mode default`) | no per-surface switch; read-only constraint only — documented limitation | — no ⚠️ |

Note: cursor is the only provider where `detect` and `command` differ from `name` — the binary is `agent`, not `cursor`. `agy` is **experimental** (PRD §12.5.1) pending the remaining §12.5.1.1 checklist items (the non-TTY stdout drop, issue #76, no longer reproduces as of **2026-07-08**) and cannot serve as a stager (empty `tooled_flags`). `qwen-code` is **experimental** (PRD §12.5.2) — a Gemini-CLI fork for Qwen3-Coder via DashScope — and cannot serve as a stager (empty `tooled_flags`).

## Tools-disable asymmetry

The seven built-in providers achieve tool-safety via two distinct mechanisms (PRD §12.7.1):

- **Explicit switch** (pi, claude): The manifest passes literal flags that **disable tools** (pi: `--no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files --no-session`; claude: `--tools "" --setting-sources "" --no-session-persistence`). This is the cleanest approach — the agent runs as a pure text-in/text-out process.

- **Read-only constraint** (codex, cursor): The manifest passes flags that **constrain the agent to a read-only, never-ask profile** (codex: `--sandbox read-only --ephemeral`; cursor: `--mode ask --trust`). opencode's `run` subcommand is inherently non-interactive and read-only.

Both approaches satisfy the §18.1 safety invariant: no provider can mutate the repository.

- **Chrome is a separate axis** (all providers): Mutation safety says nothing about agent chrome (skills, extensions, context files, MCP servers). Providers that expose a per-surface disable switch set it (pi, claude); providers that do not document the limitation honestly (codex, cursor, opencode, agy, qwen-code) — the call stays read-only and never-mutate regardless. See the **Chrome-disable** column above and the CHROME-DISABLE notes in each provider manifest (FR-C1–C5, §9.28).

## Tooled mode and the stager role

The v2 manifest system has two invocation modes (PRD §11.5):

- **Bare mode** (default): tools off, session-less, chrome-less, ephemeral. Serves the planner, message, and arbiter roles, and the entire v1 single-commit path. Uses `bare_flags`.

- **Tooled mode** (stager only): tools on, git-scoped, non-interactive. Serves **only** the stager role — the per-concept agent that runs `git add` and applies hunks. Uses `tooled_flags`. A provider with nil/empty `tooled_flags` **cannot** serve as a stager (render errors at invocation time); FR-D4 falls back to the next stager-capable provider.

The stager's safety is enforced by three layers (PRD §12.7.1):

1. **`tooled_flags`** — claude is **structurally** scoped via a staging-only git allowlist (`--allowed-tools Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit`) that makes `git commit`/`push`/`update-ref`/`reset`/`rebase` unreachable. pi is **not** flag-scoped — its tooled profile enables tools with chrome stripped (no git-scoped allowlist), so a misbehaving pi stager CAN run arbitrary Bash. pi's safety is therefore **instructional** (the §17.6 stager task prompt) + a **best-effort HEAD-movement guard** (HEAD is snapshotted before each stager call; the run aborts if HEAD moved), not structural.
2. **Stagecoach's ref-mutation monopoly** — the orchestrator alone runs `git commit`, `git update-ref`, and `git push` (§13.6.2/§19). This is a defense-in-depth layer: for claude, the structural allowlist makes ref-mutating commands unreachable; for pi, the HEAD-movement guard (Layer 1) is the actual safety net since pi lacks flag-scoping.
3. **The stager task prompt** (§17.6) — instructs the agent to stage only concept[i]'s subset and never commit/update-ref/push.

## Per-role default models (FR-D4)

Out of the box, each agent role is assigned a model sized to its job (PRD §9.16 FR-D3):

| Role | Tier | Rationale |
|------|------|-----------|
| **planner** | flagship / smart | Needs the strongest model for task decomposition and architecture reasoning. |
| **stager** | mid | Needs tool use + competence, but not the flagship — cost-effective for git staging. |
| **message** | fast | Commit-message generation is a short-text task — the cheapest/fastest tier suffices. |
| **arbiter** | mid | Needs reasoning to evaluate diffs, but not the flagship — mid-tier balances quality and cost. |

The compiled-in per-provider table (PRD §9.16 FR-D4) lives in `internal/config/role_defaults.go`. The config bootstrap (`config init`) uses these defaults — EXCEPT for **pi**, whose per-role models are written EMPTY in BOTH the active `[role.*]` block AND the commented-out pi block (pi needs an inference-provider prefix on the model, FR-R5b; its shipped per-role models are blank so you supply backend/model, e.g. `zai/gpt-5.4`). The pi row below is the compiled-in default, not the bootstrap output. Model names are 2026-07 baselines — FR-D5 mandates periodic re-verification per provider.

| Provider | planner | stager | message | arbiter |
|----------|---------|--------|---------|--------|
| `pi` | `gpt-5.4` | `gpt-5.4-mini` | `gpt-5.4-nano` | `gpt-5.4-mini` |
| `claude` | `opus` | `sonnet` | `haiku` | `sonnet` |
| `agy` | `Gemini 3.5 Flash (High)` | *(cannot)* | `Gemini 3.5 Flash (Low)` | `Gemini 3.5 Flash (Medium)` |
| `opencode` | `openai/gpt-5.4` | *(cannot)* | `openai/gpt-5.4-nano` | `openai/gpt-5.4-mini` |
| `codex` | `gpt-5.1-codex-max` | *(cannot)* | `gpt-5.4-nano` | `gpt-5.1-codex-mini` |
| `cursor` | `gpt-5.4` ⚠️ | *(cannot)* | `gpt-5.4-nano` ⚠️ | `gpt-5.4-mini` ⚠️ |
| `qwen-code` | `qwen3-coder-plus` ⚠️ | *(cannot)* | `qwen3-coder-flash` ⚠️ | `qwen3-coder-plus` ⚠️ |

*⚠️ cursor models are PRD tier-names (flagship/mid/nano) resolved to best-guess OpenAI tokens — FR-D5: verify against `agent --help`.*
*⚠️ qwen-code models are # TO CONFIRM per FR-D5 (Alibaba Qwen3-Coder via DashScope; no live CLI lookup this pass).*

**Stager column:** A value of *(cannot)* means the provider lacks `tooled_flags` in its manifest and cannot serve as the stager. When the detected provider cannot be the stager, the bootstrap falls back to the next stager-capable provider (FR-D4 fallback — currently pi or claude).

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
stagecoach providers show myagent
```

Use it:

```bash
stagecoach --provider myagent
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

A `[generation] output` / `strip_code_fence` value in the config file or git-config is an **opt-in override**: when unset, the per-provider manifest value above is what `parseOutput` uses (so `providers show` and parsing agree). Set it only to force a value across all providers (see [configuration.md](configuration.md)).
