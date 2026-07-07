# Research: tooled_flags for pi + claude (stager-capable providers) — P1.M2.T2.S2

> This subtask POPULATES the `TooledFlags []string` field (added in P1.M1.T1.S1, frozen) on the TWO
> "explicit tool-disable switch" providers — **pi** and **claude** — so they can serve the **stager** role
> (v2 §11.5: tools ON, git-scoped, non-interactive — the only role that mutates the index). The other five
> providers (gemini, agy, opencode, codex, cursor) keep `TooledFlags == nil` → they cannot stager.
>
> Sources: PRD §11.5 (h3.42), §12.1 (h3.43 — the `tooled_flags` field), §12.7.1 (the tools-disable
> asymmetry + stager inversion), §12.3 (pi), §12.4 (claude); item-spec CONTRACT (exact flag values);
> `architecture/external_deps.md` §pi/§claude; the current source (`internal/provider/*.go`).

---

## §1 — The exact TooledFlags values (from the item-spec CONTRACT)

### pi (§12.3) — tooled = bare MINUS `--no-tools`

`TooledFlags = []string{"--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session"}`

Rationale (item + §12.7.1): pi's tool-disable is a literal switch (`--no-tools`). Tooled mode = the same
bare invocation with `--no-tools` REMOVED → pi's native tool system is ON. Everything else stays off
(extensions/skills/prompt-templates/context-files/session) so the call is still chrome-less + ephemeral.

⚠️ **pi has NO git-scoped allowlist flag.** `pi --help` (external_deps.md §pi) shows only the all-or-nothing
`--no-tools`/`-nt`; there is no `--allowed-tools`/`--tools` equivalent. So pi's tooled profile turns ALL
tools on — the §12.7.1 stager safety ("tools scoped to staging, never commit/update-ref/push") for pi is
therefore enforced by the **stager task prompt (§17.6)** + stagecoach's monopoly on ref mutations (§13.6.2,
§19), NOT by `tooled_flags` scoping. This is acceptable for v1 (the field expresses pi's idiom honestly:
"tooled = tools on, no chrome"). Do NOT invent a non-existent allowlist flag.

### claude (§12.4) — tooled = tools ENABLED with a git/read/edit allowlist

`TooledFlags = []string{"--allowed-tools", "Bash(git:*),Read,Edit", "--setting-sources", "", "--no-session-persistence"}`

Rationale (item + §12.7.1): claude's bare mode disables ALL tools via `--tools ""`. Tooled mode INVERTS
this — it enables tools but RESTRICTS them via an allowlist to `Bash(git:*)` (git only) + `Read` + `Edit`.
`--setting-sources ""` (clean slate) + `--no-session-persistence` (ephemeral) carry over from bare. So
claude's stager CAN run `git add` and apply file edits but cannot (via the allowlist) do arbitrary shell.

⚠️ **`--allowed-tools` vs `--tools` discrepancy — TO CONFIRM at integration.** `external_deps.md §claude`
(verified `claude --help` 2026-06-29) records the flag as `--tools <tools...>` ("Use \"\" to disable all
tools", "default", or names). The item-spec CONTRACT says `--allowed-tools`. The codebase's synthetic tooled
fixture (`render_test.go` `dualModeManifest`) ALREADY uses `--allowed-tools`, and
`TestRender_TooledModeAppendsTooledFlags` asserts that token. **Decision: honor the item CONTRACT
(`--allowed-tools`)** — it is the authoritative instruction for this work item, matches the codebase's
mental model, and modern Claude Code does expose `--allowed-tools` (the explicit-enable allow-list flag;
`--tools` is the broader enable/disable surface). **Carry a `# TO CONFIRM` note** in the manifest doc
comment + providers/claude.toml so the first real stager run (P3.M2.T3) catches a wrong flag — exactly the
§12.7.2 "progressive verification" discipline. The value is verbatim per the item spec.

### The other five — STAY nil (not stager-capable)

| Provider | TooledFlags | Why |
|---|---|---|
| gemini | **nil** (unchanged) | read-only-constrained; no verified tooled combo (PRD §12.5 / Appendix E) |
| agy | **nil** (unchanged) | experimental + non-TTY stdout bug (§12.5.1.1); AgyFields already asserts `TooledFlags == nil` |
| opencode | **nil** (unchanged) | `run` is non-interactive read-only; no tooled surface |
| codex | **nil** (unchanged) | read-only sandbox is its bare idiom; no verified tooled combo |
| cursor | **nil** (unchanged) | `--mode ask` is read-only; no verified tooled combo |

**Do NOT touch these five.** Their builtin constructors already omit `TooledFlags` (→ nil); their TOML
literals (geminiTOML, etc.) and `providers/*.toml` have no `tooled_flags` key (→ nil on decode); so
`reflect.DeepEqual` parity holds (nil ⇄ nil). Editing them is out-of-scope churn.

---

## §2 — THE 4-WAY PARITY CHAIN (the #1 trap — mirrors S1/S2(plan001) discipline)

`TooledFlags` is a plain `[]string`. go-toml/v2 decodes (per `P1.M2T1S1/research/go-toml-pointer-behavior.md`
FINDING C/D): a **present** array → non-nil slice; an **absent** key → `nil`; a **present empty** `[]` →
non-nil empty. After this change, pi + claude carry a NON-EMPTY slice, so the TOML side MUST write the array
EXPLICITLY — or `reflect.DeepEqual(built-in, decode(TOML))` fails (non-nil ≠ nil).

`TooledFlags` is now parity-checked across FOUR artifacts per provider:

```
builtinPi().TooledFlags  (Go literal)        ⇄  piTOML  (builtin_test.go)        ⇄  providers/pi.toml
builtinClaude().TooledFlags                  ⇄  claudeTOML                       ⇄  providers/claude.toml
```

Two `reflect.DeepEqual` oracles enforce it:
- `TestBuiltinManifests_DecodeParity/{pi,claude}` — builtin ⇄ TOML literal (builtin_test.go).
- `TestProviderReferenceFiles_DecodeParity/{pi,claude}` (referencefiles_test.go) — builtin ⇄ providers/*.toml.

**Mechanical rule: set the IDENTICAL `tooled_flags = [...]` array in all FOUR places** (builtin.go Go literal,
builtin_test.go TOML literal, providers/pi.toml, providers/claude.toml). One stale/omitted entry fails
both oracles. The array ELEMENTS must match byte-for-byte (esp. `"Bash(git:*),Read,Edit"` — the commas are
inside the TOML string, NOT array separators; keep it ONE string element).

**Insertion point in the TOML literals:** after the `bare_flags = [...]` array, before `output = "raw"`
(matching the field order in manifest.go + the PRD §12.1 schema, where `tooled_flags` sits between
`bare_flags` and `output`).

---

## §3 — S1 (P1.M2.T2.S1) coupling: target POST-S1 state, preserve S1's edits

S1 (reorder preferredBuiltins + pi `default_model=""`) edits the SAME files this task touches. The current
repo is MID-S1: `builtin.go` already has `builtinPi().DefaultModel = strPtr("")` (FR-D2 applied), but
`providers/pi.toml` and `builtin_test.go` (`piTOML`, `PiFields`, render tests) still show `glm-5-turbo`
(S1 not yet applied there). **This task runs AFTER S1 completes** — assume the full post-S1 state:

| File | Post-S1 state (S1's edit) | This task ADDS (do not undo S1) |
|---|---|---|
| `builtin.go` `builtinPi()` | `DefaultModel = strPtr("")` | `TooledFlags: [5 bare-minus-... flags]` + doc-comment note |
| `builtin_test.go` `piTOML` | `default_model = ""` | `tooled_flags = [5 flags]` block + (PiFields: `TooledFlags` assertion) |
| `builtin_test.go` `PiFields` | asserts `DefaultModel == ""` | + `TooledFlags` DeepEqual assertion |
| `builtin_test.go` render tests | split into ShippedDefault + PersonalOverride | + a tooled-mode render test (Render, RenderTooled) |
| `providers/pi.toml` | `default_model = ""` + rendered-command placeholders | `tooled_flags = [5 flags]` block + tooled-render comment |
| `providers/claude.toml` | (unchanged by S1) | `tooled_flags = [5 flags]` block + tooled-render comment |

**Critical:** this task's edits are ADDITIVE on top of S1's. Do NOT revert `default_model=""`, do NOT
un-split the pi render tests, do NOT touch `preferredBuiltins` or `registry.go` (that is S1's entire scope).
If S1 has NOT fully landed (piTOML still shows `glm-5-turbo`), STOP and flag it — this task depends on S1.

(Sequential safety: S1 and S2 both edit builtin.go/builtin_test.go/providers/pi.toml, so they CANNOT run
concurrently — the orchestrator runs S1 to completion, then S2. The parallel_execution_context is about
RESEARCH concurrency only.)

---

## §4 — Test strategy (extend the existing tests; do NOT create new files)

### MUST update (parity oracles + field assertions) — else they fail once TooledFlags is set

- `builtin_test.go` `piTOML`: ADD `tooled_flags = [ ... 5 flags ... ]` block (after bare_flags).
- `builtin_test.go` `claudeTOML`: ADD `tooled_flags = [ ... 5 flags ... ]` block (after bare_flags).
- `builtin_test.go` `PiFields` (Test 3): ADD `TooledFlags` assertion
  (`reflect.DeepEqual(m.TooledFlags, []string{"--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session"})`).
- `builtin_test.go` `ClaudeFields` (Test 4): ADD `TooledFlags` assertion
  (`reflect.DeepEqual(m.TooledFlags, []string{"--allowed-tools","Bash(git:*),Read,Edit","--setting-sources","","--no-session-persistence"})`).
- `providers/pi.toml`: ADD `tooled_flags = [ ... ]` block + a tooled-render comment.
- `providers/claude.toml`: ADD `tooled_flags = [ ... ]` block + a tooled-render comment.

### ADD (new, valuable, low-cost) — pin the tooled argv

- `builtin_test.go`: ADD `TestBuiltinManifests_RenderedCommand_Pi_Tooled` — call the REAL
  `builtinPi().Render("glm-5-turbo", "zai", "<sys>", "<user>", RenderTooled)` and assert `spec.Args`
  (tooled_flags appended, NO `--no-tools`) + `spec.Stdin == "<user>"` (sys via flag). Proves (a) RenderTooled
  does NOT error (TooledFlags non-empty), (b) the tooled argv is exactly bare-without-`--no-tools`.
- `builtin_test.go`: ADD `TestBuiltinManifests_RenderedCommand_Claude_Tooled` —
  `builtinClaude().Render("sonnet", "", "<sys>", "<user>", RenderTooled)`; assert `spec.Args` uses
  `--allowed-tools Bash(git:*),Read,Edit` (NOT `--tools ""`) + `spec.Stdin == "<user>"`.

  (Use the real `Render`, NOT the local `renderArgs` helper — `renderArgs` is bare-only, has no mode param,
  and S1's render tests depend on its signature. The real `Render` exercises the actual tooled branch.)

### LEAVE (auto-cover, unaffected, or out of scope — do NOT edit)

- `TestBuiltinManifests_DecodeParity` (Test 6) — the table-driven loop auto-covers pi/claude once piTOML/
  claudeTOML carry tooled_flags. No edit needed beyond the TOML-literal blocks above.
- `TestBuiltinManifests_Validate` (Test 5) + `NameMatchesKey` (Test 2) — iterate the whole map; pi/claude
  still Validate() (TooledFlags has no Validate rule). No edit.
- `TestBuiltinManifests_AgyFields` (Test 17) — already asserts `agy.TooledFlags == nil`; keep (agy stays nil).
- `TestBuiltinManifests_GeminiFields`/`OpenCodeFields`/`CodexFields`/`CursorFields` — do NOT currently
  assert TooledFlags; they STAY nil so no edit is needed. (Optionally add `assertTooledNil` for symmetry —
  optional, low value; the item only requires pi/claude non-empty.)
- `render_test.go` — the tooled tests (`TestRender_TooledModeAppendsTooledFlags`,
  `TestRender_TooledModeEmptyFlagsErrors`, etc.) use the SYNTHETIC `dualModeManifest`; they test Render's
  mode LOGIC, not the built-ins. **Unaffected — do NOT touch.**
- `merge_test.go` — `TooledFlags` merge (regime 2 wholesale-replace) is already tested with synthetic
  fixtures. **Unaffected — do NOT touch.**
- `manifest_test.go` — Resolve's "TooledFlags left as-is (nil)" slice-regime test uses a synthetic manifest.
  **Unaffected.**
- The five non-stager providers' constructors + TOML + tests — **untouched** (TooledFlags stays nil).
- `render.go` / `merge.go` / `manifest.go` / `registry.go` — **FROZEN** (no logic change; TooledFlags plumbing
  already exists from P1.M1). go.mod/go.sum unchanged (no new dep).

---

## §5 — DOCS scope: docs/providers.md only (Appendix D is PRD-owned → OUT OF SCOPE)

The item says "DOCS: [Mode A] Update docs/providers.md and Appendix D quick-reference". **Appendix D lives
inside PRD.md (§h2.27), which is human-owned/read-only — do NOT modify PRD.md.** Scope the docs work to
`docs/providers.md` (exists, 7.8KB). It currently has NO `tooled_flags` field in its 18-field schema table,
NO bare/tooled mode in its "Command rendering" section, and NO stager mention. Add:

1. **Schema table** (`## The schema`): ADD a `tooled_flags` row (list of string; default `nil`; purpose:
   "flags for tooled/stager mode — tools ON, git-scoped, non-interactive. nil/empty ⇒ not stager-capable.").
   (Also note `experimental` is missing from the table — agy is experimental — but that is P1.M1 docs debt;
   optionally add the row, but it is NOT this task's focus. Mention as optional.)
2. **Command rendering section** (`## Command rendering`): ADD the bare/tooled mode ternary — after the
   `bare_flags...` line, note "in tooled mode (stager role), `tooled_flags` replaces `bare_flags`; tooled
   mode with empty `tooled_flags` is an error (that provider cannot serve as a stager)."
3. **NEW subsection** (`## Tooled mode and the stager role` after "Tools-disable asymmetry"): explain the
   two invocation modes (bare = tools off, planner/message/arbiter; tooled = tools on + git-scoped, stager
   ONLY) per §11.5, and LIST which providers are stager-capable:
   - **Stager-capable (non-empty `tooled_flags`):** pi (tools on, no chrome), claude (git/read/edit allowlist).
   - **NOT stager-capable (empty `tooled_flags`):** gemini, agy (experimental), opencode, codex, cursor.
   - Note the safety model (§12.7.1): the stager's safety is "tools scoped to staging, never
     commit/update-ref/push" — enforced by `tooled_flags` + stagecoach's ref-mutation monopoly + the stager
     task prompt; FR-D4 falls back to the next stager-capable provider when the chosen one can't stager.
4. **Provider table** (`## The 7 built-in providers`): optionally ADD a "Stager?" column
   (pi=yes, claude=yes, others=no). This directly satisfies "show which providers are stager-capable".

⚠️ **Pre-existing doc staleness (NOT this task's scope):** `docs/providers.md` still shows pi
`default_model = "glm-5-turbo"` (FR-D2 made it `""`; S1 did not touch this doc, and P4.M3.T1.S1 owns the
batched v2 docs sync). Do NOT fix that here unless trivially adjacent — note it as known doc debt. Focus
this task's doc edits on `tooled_flags` + the stager.

---

## §6 — Files touched / frozen / dependencies

- **TOUCH (5):** `internal/provider/builtin.go` (add TooledFlags to builtinPi + builtinClaude + doc comments),
  `internal/provider/builtin_test.go` (piTOML/claudeTOML +tooled_flags; PiFields/ClaudeFields +TooledFlags
  assertion; +2 tooled render tests), `providers/pi.toml` (+tooled_flags block + comment),
  `providers/claude.toml` (+tooled_flags block + comment), `docs/providers.md` (schema row + mode ternary +
  stager subsection + capability marker).
- **FROZEN (do NOT edit):** `manifest.go` (the TooledFlags field — already exists), `merge.go`, `render.go`,
  `registry.go`, `manifest_test.go`, `merge_test.go`, `render_test.go`, `referencefiles_test.go`,
  `internal/config/*`, `internal/git/*`, `cmd/stagecoach/main.go`, `Makefile`, `go.mod`, `go.sum`, the five
  non-stager provider constructors + their TOML/tests, **PRD.md** (Appendix D — human-owned).
- **IMPORTS:** `builtin.go` stays ZERO-import (literal `[]string` + strPtr/boolPtr only — same as the other
  fields). `builtin_test.go` already imports testing+reflect+toml; the tooled render tests use the REAL
  `Render` method (no new import). go.mod/go.sum UNCHANGED.
- **DEPENDENCY:** P1.M1.T1.S1 (TooledFlags field) COMPLETE; P1.M1.T2.S1 (Render mode param) COMPLETE;
  P1.M2.T1.S1 (agy) COMPLETE; **P1.M2.T2.S1 (reorder + pi default_model) assumed COMPLETE before this task.**
- **DOWNSTREAM:** the stager role (P3.M2.T3) calls `manifest.Render(..., RenderTooled)`; pi/claude now
  succeed (non-empty TooledFlags); the other five error at render time ("tooled mode requires non-empty
  tooled_flags") and FR-D4 falls back to a stager-capable provider.
