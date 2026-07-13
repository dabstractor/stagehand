# Research: P1.M2.T1.S1 — Add Chrome-disable column + asymmetry bullet to docs/providers.md

Pure documentation task (Mode B — this IS the doc task). Two edits to `docs/providers.md`:
1. Add a **Chrome-disable** column to the 7-provider table (after "Tool-disable approach", before "Stager?").
2. Add a **"Chrome is a separate axis"** bullet to the "## Tools-disable asymmetry" section.

Source of truth = the CHROME-DISABLE notes in `internal/provider/builtin.go` (written by the completed
P1.M1.T1.S1). All 7 contract-specified column values verified against those notes (see §2).

---

## 1. THE TABLE — `docs/providers.md` lines 78-86

Current header (line 78) — 8 columns:
```
| Provider | Delivery | Print flag | Model flag | Default model | System prompt flag | Tool-disable approach | Stager? |
```
Separator (line 79):
```
|----------|----------|-----------|-----------|----------------|-------------------|----------------------|--------|
```
Data rows (lines 80-86), in THIS order: pi, claude, opencode, codex, cursor, agy, qwen-code.

### The edit
Insert a **Chrome-disable** column between "Tool-disable approach" and "Stager?" — in the header, the
separator, and ALL 7 data rows. New header (9 columns):
```
| Provider | Delivery | Print flag | Model flag | Default model | System prompt flag | Tool-disable approach | Chrome-disable | Stager? |
```
New separator (one more dash column; width is cosmetic — markdown renders regardless):
```
|----------|----------|-----------|-----------|----------------|-------------------|----------------------|----------------|--------|
```

### Per-row Chrome-disable cell values (in the TABLE's row order)

(Map the contract values to the actual row order pi, claude, opencode, codex, cursor, agy, qwen-code.
4 of the 5 read-only providers share identical text; opencode is "by design".)

| Row | Provider | Chrome-disable cell (insert before Stager?) |
|-----|----------|----------------------------------------------|
| 80 | `pi` | `extensions/skills/templates/context off (`--no-*`); MCP use suppressed (servers may connect — tracked limitation)` |
| 81 | `claude` | `via `--tools ""` + `--setting-sources ""`` |
| 82 | `opencode` | `no per-surface switch; read-only by design — documented limitation` |
| 83 | `codex` | `no per-surface switch; read-only constraint only — documented limitation` |
| 84 | `cursor` | `no per-surface switch; read-only constraint only — documented limitation` |
| 85 | `agy` | `no per-surface switch; read-only constraint only — documented limitation` |
| 86 | `qwen-code` | `no per-surface switch; read-only constraint only — documented limitation` |

### Markdown gotchas
- **Em-dash (—)** in cells: fine (the doc already uses them; MD013 is off).
- **Double-quotes** in the claude cell (`--tools ""`): fine in a markdown table cell (no escaping needed;
  pipes are the only char that breaks cells, and none of the values contain a pipe).
- **Backticks** around flag tokens (`--no-*`, `--tools`, `--setting-sources`): use them for consistency
  with the existing table (which backticks `--model`, `-p`, etc.).
- **Cell count**: every row must have exactly 9 cells (8 `|`-separated content cells). A missing/extra
  cell breaks GitHub's table rendering — count the pipes.
- **Column width / alignment**: markdown renders regardless of source spacing; do NOT hand-align the long
  Chrome-disable column (it would be unreadable). Just keep the pipe count consistent. (MD013 line-length
  is OFF in `.markdownlint.json`, so the wide rows are fine.)

---

## 2. VALUE VERIFICATION — contract vs builtin.go CHROME-DISABLE notes

Every contract value matches the verified source of truth (FR-C5 notes in `internal/provider/builtin.go`):

| Provider | Contract column value | builtin.go CHROME-DISABLE (verified) | Match |
|----------|-----------------------|--------------------------------------|-------|
| pi | extensions/skills/templates/context off; MCP use suppressed (servers may connect — tracked limitation) | extensions/skills/prompt-templates/context-files disabled by `--no-*`; MCP NOT disabled (no `--no-mcp`; `--no-tools` suppresses MCP tool USE, servers may connect) — LIMITATION FR-C3 | ✓ |
| claude | via `--tools ""` + `--setting-sources ""` | `--tools ""` disables all tools (MCP surfaces as tools); `--setting-sources ""` blocks settings files (MCP/skills/extensions). No per-surface gap | ✓ |
| agy | no per-surface switch; read-only constraint only — documented limitation | NO per-surface chrome switch; `--mode plan` is read-only CONSTRAINT not chrome; Chrome MAY load — LIMITATION FR-C4 | ✓ |
| qwen-code | no per-surface switch; read-only constraint only — documented limitation | NO known per-surface chrome switch; `--approval-mode default` is read-only CONSTRAINT not chrome; unverified — LIMITATION FR-C4 | ✓ |
| opencode | no per-surface switch; read-only by design — documented limitation | `run` inherently read-only by design, NO per-surface switch; bare_flags empty; Chrome MAY load — LIMITATION FR-C4 | ✓ ("by design") |
| codex | no per-surface switch; read-only constraint only — documented limitation | NO per-surface chrome switch; `--sandbox read-only`+`--ephemeral` are read-only CONSTRAINT not chrome — LIMITATION FR-C4 | ✓ |
| cursor | no per-surface switch; read-only constraint only — documented limitation | NO per-surface chrome switch; `--mode ask`+`--trust` are read-only Q&A CONSTRAINT not chrome — LIMITATION FR-C4 | ✓ |

All 7 consistent. The docs (Mode B) faithfully record the verified manifest state.

---

## 3. THE ASYMMETRY SECTION — `docs/providers.md` lines 90-98

Current structure:
```
## Tools-disable asymmetry

The seven built-in providers achieve tool-safety via two distinct mechanisms (PRD §12.7.1):

- **Explicit switch** (pi, claude): The manifest passes literal flags that disable tools (...). ...
- **Read-only constraint** (codex, cursor): The manifest passes flags that constrain the agent (...). opencode's `run` subcommand is inherently non-interactive and read-only.

Both approaches satisfy the §18.1 safety invariant: no provider can mutate the repository.
```

### The edit — add a "Chrome is a separate axis" bullet AFTER the concluding sentence (line 98)

Placement rationale: keep the two mutation-safety bullets + their "Both approaches..." summary intact as
one coherent block, THEN introduce chrome as a separate axis (mirrors PRD §12.7.1 consequence #4, which
is also a separate item after the mutation-safety consequences). Add a blank line, then the new bullet:

```
- **Chrome is a separate axis** (all providers): Mutation safety says nothing about agent chrome (skills, extensions, context files, MCP servers). Providers that expose a per-surface disable switch set it (pi, claude); providers that do not document the limitation honestly (codex, cursor, opencode, agy, qwen-code) — the call stays read-only and never-mutate regardless. See the **Chrome-disable** column above and the CHROME-DISABLE notes in each provider manifest (FR-C1–C5, §9.28).
```

(The contract provides this wording; I added the §9.28/FR-C1–C5 reference + bolded "Chrome-disable" to
cross-link the new column. Em-dashes are consistent with the doc.)

---

## 4. SCOPE FENCES (what NOT to touch)

- **ONLY `docs/providers.md`.** The other doc surfaces (docs/how-it-works.md safety paragraph,
  docs/README.md index, top-level README.md) are P1.M2.T1.S2 — a SEPARATE subtask. Do NOT edit them.
- **NO Go code, NO tests.** This is Mode B documentation. The CHROME-DISABLE notes (builtin.go) and the
  contract tests (builtin_test.go, parallel P1.M1.T2.S1) are the source of truth this doc RECORDS — do
  not modify them.
- **NO providers/*.toml.** Those reference TOML files already mirror the builtin.go notes (P1.M1.T1.S2);
  this task is the providers.md table column only.
- **NO PRD.md / tasks.json / prd_snapshot.md** (read-only).

---

## 5. PARALLEL-OVERLAP CHECK

Parallel sibling **P1.M1.T2.S1** adds `TestBuiltinManifests_ChromeDisableContract` to
`internal/provider/builtin_test.go` — Go test assertions on BareFlags. It does NOT touch any docs file.
No overlap with this docs task. No conflict regardless of order. (It also confirms the same BareFlags
values this doc records — so the two tasks are consistent.)

---

## 6. Validation (docs aren't in `make`; manual + grep)

- `.markdownlint.json`: `{"default":true, "MD013":false, "MD033":false, "MD060":false}` — line length OFF,
  so the wide Chrome-disable rows are fine. There is NO markdownlint target in the Makefile (`make lint`
  is golangci-lint for Go only). So docs validation is:
  - Manual: render the table (e.g. `glow docs/providers.md` or a GitHub preview) — the table must render
    with 9 columns, no broken rows.
  - Grep guards: every row has 9 cells (8 pipes of content); the Chrome-disable header is present once;
    the chrome bullet is present; the existing "Tool-disable approach" / "Stager?" columns are intact.
- `make test` / `make lint` are unaffected (no Go change) — run only to confirm the working tree is clean.
