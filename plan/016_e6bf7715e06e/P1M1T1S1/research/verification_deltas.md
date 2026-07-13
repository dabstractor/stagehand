# Research Notes — P1.M1.T1.S1 (CHROME-DISABLE notes for all 7 built-in providers)

Verification against the CURRENT working tree (2026-07-13). The architecture docs
(`plan/016_*/architecture/external_deps.md` + the PRD §9.28 FR-C1–C5 / §12.7.1) are accurate and
complete. These notes record the exact per-provider content for the CHROME-DISABLE note and confirm
the FR-C2 "any missing flags?" verification outcome.

## TASK SHAPE — documentation-only (no struct/flag change expected)

- The Manifest struct has NO ChromeDisable field (manifest.go: only BareFlags []string at :70 +
  TooledFlags at :76). Chrome-disable is expressed via existing `bare_flags` PLUS a doc-comment note.
- S1 = add a CHROME-DISABLE note paragraph to each of the 7 provider doc-comment blocks in
  internal/provider/builtin.go. NO code/flag changes unless the FR-C2 verification finds a missed flag
  (it does NOT — see DELTA 1).
- The note must be "consumable by P1.M1.T2.S1" — i.e. each claim of the form "flag X disables surface Y"
  must name the exact flag token so the test can assert `BareFlags` contains it.

## VERIFIED bare_flags (per provider) — the input to the notes

| Provider | func (line) | BareFlags | Chrome surfaces disabled | Gap |
|----------|-------------|-----------|--------------------------|-----|
| pi | builtinPi (42) | `--no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files --no-session` | extensions, skills, prompt-templates, context-files (AGENTS.md/CLAUDE.md), session; --no-tools also suppresses MCP tool *use* | MCP: NO `--no-mcp` flag exists (only `--mcp-config`); servers may still connect at startup → documented limitation (FR-C3) |
| claude | builtinClaude (111) | `--tools ""` `--setting-sources ""` `--no-session-persistence` | all tools (MCP surfaces as tools) via --tools ""; settings files where MCP/skills/extensions are configured via --setting-sources "" | none — chrome covered |
| agy | builtinAgy (198) | `--mode plan` | none per-surface (read-only constraint only) | no per-surface chrome switch exists → documented limitation (FR-C4) |
| qwen-code | builtinQwenCode (246) | `--approval-mode default` | none per-surface (read-only constraint only) | unknown/unverified (# TO CONFIRM) → documented limitation (FR-C4) |
| opencode | builtinOpenCode (285) | `[]string{}` (empty) | none per-surface (`run` is read-only by design) | no per-surface chrome switch on `run` → documented limitation (FR-C4) |
| codex | builtinCodex (332) | `--sandbox read-only` `--ephemeral` | none per-surface (read-only constraint only) | no per-surface chrome switch on exec → documented limitation (FR-C4) |
| cursor | builtinCursor (370) | `--mode ask` `--trust` | none per-surface (read-only constraint only) | no per-surface chrome switch → documented limitation (FR-C4) |

NOTE: codex's `--ephemeral` IS present in the code (builtin.go:344-347) — the external_deps.md summary
table was accurate. No discrepancy.

## DELTA 1 — FR-C2 verification outcome: ZERO flag additions needed

The task says "VERIFY against plan/001_f1f80943ac34/architecture/external_deps.md whether any of the 5
read-only-constrained providers actually DO expose a chrome-disable switch currently unset in
bare_flags." Re-read both catalogs:
- plan/001 catalog is tool-disable-focused (predates the chrome focus). It CONFIRMS pi's full flag set
  (`--no-tools/-nt, --no-extensions/-ne, --no-skills/-ns, --no-prompt-templates/-np,
  --no-context-files/-nc, --no-session`) and the ABSENCE of `--no-mcp`. It does NOT surface any chrome
  switch on agy/qwen-code/opencode/codex/cursor.
- plan/016 external_deps.md already did the per-surface chrome verification: "No per-surface disable
  switch found" for all 5 read-only providers.

CONCLUSION: no bare_flags additions. Every chrome-disable flag the CLIs expose is already set. The work
is purely the doc-comment notes + (T2.S1) the test assertions. Do NOT invent flags (FR-C4 is explicit).

## DELTA 2 — Doc-comment structure (where each note goes)

Each builtinXxx() has a multi-paragraph doc-comment block immediately preceding its `func` line:
- pi:    doc block lines 30-41, func at 42
- claude: doc block 102-110, func at 111
- agy:   doc block 154-197, func at 198
- qwen-code: doc block 221-245, func at 246
- opencode: doc block 268-284, func at 285
- codex: doc block 305-331, func at 332
- cursor: doc block 354-369, func at 370

The CHROME-DISABLE note is a NEW paragraph appended to each block (before the `func`, after the
existing verification/notes paragraphs). Use `//` line comments consistent with the existing style. The
first line of each block already starts with the function name (godoc convention) — appending a later
paragraph does NOT break that.

The PRD FR-C5 says the note sits "alongside the existing TOOLS-DISABLE CATEGORY note" — note: the actual
headers use prose like "§12.7.1 'read-only constraint'" / "explicit tool-disable switch" rather than a
literal "TOOLS-DISABLE CATEGORY" heading. The CHROME-DISABLE note should start with a recognizable
marker line, e.g. `// CHROME-DISABLE (FR-C5, §9.28):` so it is grep-able and T2.S1/S2 (providers/*.toml)
can locate/mirror it.

## DELTA 3 — Test pattern T2.S1 will consume (so structure the notes to match)

builtin_test.go asserts BareFlags with `reflect.DeepEqual(m.BareFlags, wantBare)` (exact order) AND
existing tests check specific tokens. T2.S1 will add chrome-disable contract assertions that check a
flag token is PRESENT in BareFlags (e.g. `slices.Contains(m.BareFlags, "--no-extensions")` or the
existing contains-helpers). So each CHROME-DISABLE note must NAME the exact flag token for each surface
it claims disabled — e.g. "extensions: disabled by --no-extensions" — so the test can assert that token.
For the read-only providers, the note states "no per-surface chrome switch exists" (nothing to assert
beyond the read-only constraint flag already tested).

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T1.S2**: mirror the CHROME-DISABLE notes into providers/*.toml reference files (the TOML
  comment blocks). S1 is builtin.go ONLY.
- **P1.M1.T2.S1**: add chrome-disable contract assertions to builtin_test.go (the tests that consume
  these notes). S1 is the notes; T2.S1 is the tests.
- **P1.M2.T1.*** : docs/providers.md (Chrome-disable column + asymmetry bullet), docs/how-it-works.md,
  README.md. Do NOT touch external docs in S1.
- DOCS: [Mode A per task] the CHROME-DISABLE note IS the per-provider documentation artifact — it lives
  in builtin.go. No separate docs subtask for S1.
