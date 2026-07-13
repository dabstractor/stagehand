# External Dependencies — Chrome-disable research (v2.9, FR-C1–C5)

## Verification sources

The per-provider `--help` findings were captured in prior research sessions. The primary catalog is
`plan/001_f1f80943ac34/architecture/external_deps.md`. Later re-verifications exist in
`plan/010_*, plan/012_*, plan/014_*/architecture/external_deps.md`. These catalogs predate the
chrome focus (they record tool-disable flags, not chrome switches specifically), so verification
re-reads them **for chrome surfaces** — but the `--help` output is already captured.

## Per-provider chrome surface inventory

### pi (§12.3) — VERIFIED, BAR-SETTER

pi's `--help` (external_deps.md §pi) explicitly documents chrome-disable switches:

| Surface | Flag | Status |
|---------|------|--------|
| Extensions | `--no-extensions` / `-ne` | **SET** in bare_flags ✓ |
| Skills | `--no-skills` / `-ns` | **SET** in bare_flags ✓ |
| Prompt-templates | `--no-prompt-templates` / `-np` | **SET** in bare_flags ✓ |
| Context files (AGENTS.md/CLAUDE.md) | `--no-context-files` / `-nc` | **SET** in bare_flags ✓ |
| Session | `--no-session` | **SET** in bare_flags ✓ |
| Tools (general) | `--no-tools` / `-nt` | **SET** in bare_flags ✓ |
| MCP servers | `--no-mcp` | **DOES NOT EXIST** — pi has only `--mcp-config <path>`. `--no-tools` suppresses MCP tool *use* but servers may still be discovered/connected at startup. → **Documented limitation (FR-C3)**. |

**Verdict:** pi sets the bar for chrome-disable. Four of five chrome surfaces are disabled. The MCP
gap (no `--no-mcp` flag exists) is a documented, tracked limitation, not an assumption.

### claude (§12.4) — VERIFIED, CHROME-COVERED

claude's `--help` (external_deps.md §claude) shows:

| Surface | Flag | Status |
|---------|------|--------|
| All tools (incl. MCP surfaces) | `--tools ""` | **SET** in bare_flags ✓ |
| Settings files (where MCP/skills/extensions are configured) | `--setting-sources ""` | **SET** in bare_flags ✓ |
| Session persistence | `--no-session-persistence` | **SET** in bare_flags ✓ |

**Verdict:** claude covers chrome via two mechanisms: `--tools ""` disables all built-in tools (MCP
surfaces as tools), and `--setting-sources ""` blocks the settings files where MCP servers,
skills, and extensions are configured. Both are already set. This is the claude equivalent of
chrome-less.

### agy (§12.5.1) — VERIFIED 2026-07-08 (agy v1.1.0), READ-ONLY CONSTRAINT

agy v1.1.0 has **diverged** from the gemini-cli lineage. Its `--help` shows:

| Surface | Flag | Status |
|---------|------|--------|
| Read-only constraint | `--mode plan` | **SET** in bare_flags ✓ |
| Chrome surfaces (skills/extensions/context/MCP) | No per-surface disable switch found | **Documented limitation (FR-C4)** |

**Verdict:** agy exposes `--mode plan` (read-only, never-ask) but has no per-surface chrome-disable
switches. The limitation is documented: chrome may load, but the call stays read-only and
never-mutate. agy stays `experimental = true` pending §12.5.1.1 item 4 (stager flags).

### qwen-code (§12.5.2) — EXPERIMENTAL, GEMINI-CLI FORK

qwen-code is a fork of gemini-cli. Its flag surface mirrors the gemini-cli lineage. Flag surface
assembled from docs (NOT yet `--help`-verified).

| Surface | Flag | Status |
|---------|------|--------|
| Read-only constraint | `--approval-mode default` | **SET** in bare_flags ✓ (# TO CONFIRM) |
| Chrome surfaces | Unknown — `# TO CONFIRM` per FR-D5 | **Documented limitation (FR-C4)** |

**Verdict:** qwen-code's chrome surface is unverified (experimental). The limitation is documented.
Chrome-disable documentation mirrors the agy pattern.

### opencode (§12.6) — VERIFIED, READ-ONLY BY DESIGN

opencode's `run` subcommand is inherently non-interactive and read-only.

| Surface | Flag | Status |
|---------|------|--------|
| Read-only constraint | `run` subcommand (inherent) | **SET** (empty bare_flags = run is already bare) ✓ |
| Chrome surfaces | No per-surface disable switch on `run` | **Documented limitation (FR-C4)** |

**Verdict:** opencode's `run` is read-only by design. No chrome-disable switches are available on
`run`. The limitation is documented.

### codex (§12.7) — VERIFIED, READ-ONLY CONSTRAINT

codex `exec` (external_deps.md §codex, verified 2026-07-08, codex-cli 0.143.0):

| Surface | Flag | Status |
|---------|------|--------|
| Read-only constraint | `--sandbox read-only` | **SET** in bare_flags ✓ |
| Session clean | `--ephemeral` | **SET** in bare_flags ✓ |
| Chrome surfaces (MCP/AGENTS.md/skills) | No per-surface disable switch found on `exec` | **Documented limitation (FR-C4)** |

**Verdict:** codex has no global tool-disable switch. `--sandbox read-only` constrains it to
read-only. No per-surface chrome switches are documented on `exec`. The limitation is documented.

### cursor (§12.7) — VERIFIED, READ-ONLY CONSTRAINT

cursor agent (external_deps.md §cursor):

| Surface | Flag | Status |
|---------|------|--------|
| Read-only constraint | `--mode ask` | **SET** in bare_flags ✓ |
| Workspace trust skip | `--trust` | **SET** in bare_flags ✓ |
| Chrome surfaces | No per-surface disable switch found | **Documented limitation (FR-C4)** |

**Verdict:** cursor has no global tool-disable switch. `--mode ask` constrains to read-only Q&A.
No per-surface chrome switches. The limitation is documented.

## Summary table

| Provider | Chrome-disable mechanism | Gap |
|----------|-------------------------|-----|
| pi | `--no-extensions/--no-skills/--no-prompt-templates/--no-context-files` | MCP: no `--no-mcp` exists (documented) |
| claude | `--tools ""` + `--setting-sources ""` | None — covers all surfaces |
| agy | Read-only constraint only (`--mode plan`) | No per-surface switches (documented) |
| qwen-code | Read-only constraint only (`--approval-mode default`) | No per-surface switches (documented) |
| opencode | Read-only by design (`run` subcommand) | No per-surface switches (documented) |
| codex | Read-only constraint only (`--sandbox read-only --ephemeral`) | No per-surface switches (documented) |
| cursor | Read-only constraint only (`--mode ask --trust`) | No per-surface switches (documented) |

**Expected flag additions:** Near-zero. The existing bare_flags already set every chrome-disable
flag that exists. The work is documentation (CHROME-DISABLE notes) and test assertions, not flag
changes.
