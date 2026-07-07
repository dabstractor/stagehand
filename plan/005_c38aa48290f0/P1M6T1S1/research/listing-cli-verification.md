# `list_models_command` — Live CLI Verification (2026-07-03)

> Source of truth for populating `Manifest.ListModelsCommand` across the 8 built-ins.
> Per the work-item contract + external_deps.md §9: "populate ONLY where verified against the
> live CLI in-task; record the verification date in a source comment." This file records that
> verification. FR-D5: the IMPLEMENTING AGENT must **re-confirm** each entry at implementation
> time (model CLIs iterate fast) and stamp the date.

## Verification method

For each installed provider, ran `<cli> --help` (grep for `models`/`list-models`) AND executed the
candidate command capturing exit code + first lines. "Verified" = the command exists AND exits 0
producing a model list without auth. "Exists-requires-auth" = the subcommand is recognized but exits
non-zero demanding credentials (still a valid listing surface for an authed user). "Not found" = no
model-listing surface in `--help`.

Installed at verification time: pi, opencode, claude, codex, cursor(`agent`), agy, gemini.
**NOT installed: qwen-code** (cannot verify → left empty).

## Results

### ✅ VERIFIED — exit 0, lists models, no auth needed → POPULATE

| Provider | argv (`list_models_command`) | live evidence |
|----------|------------------------------|---------------|
| **opencode** | `["opencode", "models"]` | `opencode models` → exit 0; prints `opencode/big-pickle`, `opencode/deepseek-v4-flash-free`, … (PRD-cited known-good; `opencode models --help` confirms subcommand `list all available models`) |
| **pi** | `["pi", "--list-models"]` | `pi --list-models` → exit 0; prints a TUI-style table (`provider model context max-out thinking images`, e.g. `deepseek deepseek-v4-flash 1M 384K yes no`). **FLAG form, not a subcommand** — argv uses `--list-models`. |
| **agy** | `["agy", "models"]` | `agy models` → exit 0; prints `Gemini 3.5 Flash (Medium)`, `Gemini 3.5 Flash (High)`, … |

### ⚠️ EXISTS — recognized subcommand, but requires auth → POPULATE with auth note

| Provider | argv | live evidence |
|----------|------|---------------|
| **cursor** | `["agent", "models"]` | `agent models` → **exit 1**: `Error: Authentication required. Run 'agent login'...`. The `models` subcommand IS valid (`agent --help` lists `models  List available models for this account`; also a `--list-models` flag exists). For an authed cursor user it works; for an unauthed one FR-L1's "(b) if it fails, print the curated table" fallback covers it. **Binary is `agent`** (manifest Detect/Command="agent", ≠ Name "cursor") → argv uses `agent`, not `cursor`. |

### ❌ NOT FOUND — no model-listing surface → LEAVE EMPTY (fall back to FR-D4 table)

| Provider | why empty |
|----------|-----------|
| **claude** | `claude --help` shows only selection flags `--model <model>`, `--fallback-model <model>`. No listing subcommand. |
| **codex** | `codex --help` shows only `-m, --model <MODEL>` + `-c model="..."` config. No listing subcommand. |
| **gemini** | `gemini --help` shows `gemini gemma` (manage LOCAL Gemma routing — not a general model list) + `-m, --model`. No general listing command. |
| **qwen-code** | **Not installed** — cannot verify. Ships experimental as the gemini-twin; MAY have a `models` subcommand (it's a Gemini-CLI fork), but unverified = empty per the contract. |

## Contract delta

The work-item contract's "expected" line said: `opencode → ["opencode", "models"]` (only).
external_deps.md §9 said "only opencode's `opencode models` is the known listing CLI."
**In-task live verification found 3 more** (pi, agy, cursor) — exactly the "verify each CLI's
`--help`, populate only confirmed" instruction the contract makes mandatory. The richer result is the
correct one: 4 built-ins populated, 4 left empty. FR-L1's fallback guarantees empty entries still
produce useful output (the curated FR-D4 tier table, via `DefaultModelsForProvider`).

## argv form gotchas (carry into builtin.go)

1. **pi is a FLAG, not a subcommand**: `["pi", "--list-models"]`, NOT `["pi", "models"]`.
   The `--list-models` flag also takes an optional `[search]` positional (fuzzy) — OMIT it; bare
   `--list-models` lists everything, which is what FR-L1 wants.
2. **cursor's binary is `agent`**: `["agent", "models"]`, NOT `["cursor", "models"]`. Matches the
   manifest's Detect/Command. Record the auth requirement in the source comment.
3. **agy/opencode are subcommands**: `["agy", "models"]` / `["opencode", "models"]`.
4. `list_models_command` is the FULL argv (binary + args) per PRD §12.1 schema example
   `["opencode", "models"]` — NOT relative to the manifest's `command`. (S2's `stagecoach models` will
   run it via `exec.Command(argv[0], argv[1:]...)`; FR-L1 says "run it, print its stdout".)

## Scope boundary (do NOT do here)

- S1 adds the field + wires merge + populates built-ins/TOMLs + docs. It does NOT implement the
  `stagecoach models` command (that is **P1.M6.T1.S2**, the consumer of `Manifest.ListModelsCommand`).
- S1 does NOT touch `config init --interactive` (P1.M6.T2.S1).
- User-defined providers set `list_models_command` via `[provider.<name>]` blocks **for free** through
  the existing `DecodeUserOverrides` → `MergeManifest` path (slice regime 2) — no config-layer change.
