# System Context — v2.9 Chrome-disable for every provider

## Delta scope

**From:** v2.8 (`plan/015_b461e4720495/prd_snapshot.md`)
**To:** v2.9 (`plan/016_e6bf7715e06e/prd_snapshot.md`)

The substantive delta is **§9.28 Chrome-disable for every provider** (FR-C1–C5, → G25) plus a
§12.7.1 conceptual extension (chrome as a separate axis from mutation safety).

**Sizing:** SMALL — a verification + documentation discipline layered on already-shipped provider
manifests. No commit/CAS/rescue/lock changes. No mutation-safety change. No manifest schema change.
No new CLI flags, env vars, or config keys.

## What "chrome" means

Chrome = agent features discovered/loaded from the user's environment and injected around the model
call that are NOT required for a commit-message prompt:

1. **Skills** — skill discovery and loading
2. **Extensions / prompt-templates** — extension and prompt-template discovery
3. **Context files** — AGENTS.md / CLAUDE.md-style context-file discovery
4. **MCP servers** — MCP-server discovery and spawn

Chrome excludes: the model itself, stagecoach's system prompt, and the diff payload.

A provider invocation is "chrome-less" when none of these are loaded or connected.

## Current codebase state (verified)

### Provider manifests — `internal/provider/builtin.go`

Seven built-in providers. Each is constructed via a `builtin*()` function with a comprehensive
doc-comment block. Key findings:

| Provider | Current bare_flags (chrome-relevant) | Chrome status |
|----------|-------------------------------------|---------------|
| **pi** | `--no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files --no-session` | Extensions, skills, prompt-templates, context-files **ALREADY disabled**. MCP gap: pi has no `--no-mcp` (only `--mcp-config`); `--no-tools` suppresses MCP tool *use* but servers may still connect. → Document (FR-C3). |
| **claude** | `--tools "" --setting-sources "" --no-session-persistence` | **Chrome covered**: `--tools ""` disables all tools (MCP surfaces as tools); `--setting-sources ""` blocks settings files where MCP/skills/extensions are configured. → Confirm + document (FR-C2/C3). |
| **agy** | `--mode plan` | Read-only constraint; no chrome switches set. agy v1.1.0 diverged from gemini-cli. → Verify `--help` for chrome switches; else document limitation (FR-C4). |
| **qwen-code** | `--approval-mode default` | Gemini-cli fork; experimental. → Verify `--help`; document (FR-C4). |
| **opencode** | `[]` (empty — `run` is inherently read-only/non-interactive) | No chrome switches set. → Verify `--help`/`run --help`; document (FR-C4). |
| **codex** | `--sandbox read-only --ephemeral` | Read-only constraint; no chrome switches set. → Verify `exec --help`; document (FR-C4). |
| **cursor** | `--mode ask --trust` | Read-only constraint; no chrome switches set. → Verify `agent --help`; document (FR-C4). |

### Manifest struct — `internal/provider/manifest.go`

The `Manifest` struct has **22 fields**. There is **no ChromeDisable field** — chrome-disable is
expressed via the existing `bare_flags` and `env` fields plus doc-comment notes. v2.9 adds nothing
structural to the schema.

### CHROME-DISABLE notes

**None exist yet.** Each provider's doc-comment block carries TOOLS-DISABLE discussions and
verification notes, but no CHROME-DISABLE paragraph. This is the primary code artifact to produce.

### Tests — `internal/provider/builtin_test.go`

Comprehensive test suite (21 tests) covering field assertions, decode parity, rendered commands,
freshness, and tooled mode. **No chrome-disable assertions exist.** The existing tests DO assert
bare_flags contents, so any additions to bare_flags would need test updates (but we expect few/no
flag additions — the work is mostly documentation).

### Documentation surfaces

1. **`docs/providers.md`** — Has "## The 7 built-in providers" table (7 rows, columns: Delivery,
   Print flag, Model flag, Default model, System prompt flag, Tool-disable approach, Stager?). Has
   "## Tools-disable asymmetry" section (two bullets: explicit switch vs read-only constraint).
   **No Chrome-disable column.**

2. **`docs/how-it-works.md`** — Has "### Safety invariant" paragraph (line ~197): "No provider
   mutates the repository (PRD §18.1)…" — covers mutation safety only. **No chrome mention.**

3. **`docs/README.md`** — Documentation index + capability index. **No chrome entry.**

4. **Top-level `README.md`** — Safety messaging embedded in feature descriptions (line 4 hero pitch,
   line 66 feature table). **No dedicated bare-mode/chrome bullet**, but "can never corrupt your repo"
   messaging exists.

5. **`providers/*.toml`** — Human-readable reference TOML files. `providers/pi.toml` was found to
   contain "chrome" references in comments. These mirror `builtin.go`.

### Prior research (verification input)

`plan/001_f1f80943ac34/architecture/external_deps.md` is the richest per-provider `--help` catalog.
Key chrome-relevant findings already captured:
- pi: `--no-extensions` (disable extension discovery), `--no-skills` (disable skills discovery),
  `--no-prompt-templates` (disable prompt template discovery), `--no-context-files` (disable
  AGENTS.md and CLAUDE.md discovery), `--no-session`, `--no-tools`
- claude: `--tools ""` (disable all tools), `--setting-sources <sources>` (load setting sources),
  `--no-session-persistence`
- pi has NO `--no-mcp` flag — only `--mcp-config <path>`
- The `--help` output for codex, cursor, opencode, agy, qwen-code was captured but predates the
  chrome focus — they record tool-disable flags, not chrome switches specifically

These catalogs predate the chrome focus, so verification genuinely re-reads them **for chrome
surfaces** — but the `--help` output is already captured; no new agent invocations are required to
author the notes.

## Files to touch

| File | Change | Task |
|------|--------|------|
| `internal/provider/builtin.go` | Add CHROME-DISABLE note to each of 7 provider doc-comment blocks; possibly add 1-2 missing chrome-disable flags if verification finds them | M1.T1 |
| `internal/provider/builtin_test.go` | Add chrome-disable contract assertions (pi/claude flag presence, note/flag consistency, read-only constraint presence) | M1.T2 |
| `docs/providers.md` | Add "Chrome-disable" column to 7-provider table; add "Chrome is a separate axis" bullet to asymmetry section | M2.T1 |
| `docs/how-it-works.md` | Extend safety paragraph (~line 197) with chrome sentence | M2.T1 |
| `docs/README.md` | Add one-line chrome-disable capability index entry | M2.T1 |
| `README.md` | Brief mention of chrome-less where appropriate in safety messaging | M2.T1 |
| `providers/*.toml` | Mirror any CHROME-DISABLE note additions in reference TOML headers (consistency with builtin.go) | M1.T1 |

## What is NOT changing

- No commit / CAS / rescue / lock / index behavior changes
- No mutation-safety change — read-only constraints and tool-disable switches stay as-is
- No manifest schema change — no new field
- No new CLI flags, env vars, config keys, or git-config keys
- The author rename (`dustin` → `dabstractor`) is already implemented — no tasks
- No removal of existing `experimental` flags on agy/qwen-code
