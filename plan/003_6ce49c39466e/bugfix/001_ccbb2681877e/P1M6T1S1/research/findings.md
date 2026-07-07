# Research Findings — P1.M6.T1.S1 (Sync README.md / docs/cli.md / docs/providers.md for the reasoning changeset)

> Documentation-only changeset-level sync (Mode B). No code, no unit tests. The reasoning feature
> (`--reasoning`, `planner=high`, every `--<role>-reasoning`) was made FUNCTIONAL for **pi** and
> **claude** by the now-complete implementing subtasks (P1.M1.T1.S1/S2 + P1.M2.* + P1.M3.* + P1.M4.*).
> This task makes the **docs** match the now-working reality, scopes the `--reasoning` claim honestly,
> and verifies no stale flag names remain.

## §1 — What "activation" means now (the verified tokens, authoritative)

From `architecture/external_deps.md` (research performed 2026-07-02 against the live `--help` output):

| Provider | Reasoning flag | Verified values | Stagecoach mapping |
|----------|----------------|-----------------|-------------------|
| `pi` | `--thinking` | off, minimal, low, medium, high, xhigh | high→`["--thinking","high"]`, medium→medium, low→low, off→no tokens |
| `claude` | `--effort` | low, medium, high | high→`["--effort","high"]`, medium→medium, low→low, off→no tokens |
| gemini/agy/qwen-code/opencode/codex/cursor | (none verified) | — | nil → **graceful no-op** (FR-R6 honest per-provider no-op; no error) |

**CRITICAL**: the PRD Issue 1 "Suggested Fix" GUESSED `--thinking-effort` for claude — that is WRONG.
The verified flag is **`--effort`**. Confirmed (§5): `grep -rn "thinking-effort"` across `README.md
docs/ providers/` returns NOTHING today. The docs already use the correct `--effort`; this task must
KEEP it that way and add a guard so a future edit doesn't reintroduce the wrong name.

After the implementing subtasks, the shipped default `planner = high` and `--reasoning high` emit real
tokens for pi/claude (and are an honest no-op elsewhere). Before them, every provider's
`ReasoningLevels` was `nil` and the whole feature was inert — that is what the README/cli.md were
describing (inertly) and must now stop describing.

## §2 — README.md current state (the only edit point)

`grep -n reasoning README.md` → exactly **one** site: **lines 121–122**, inside the `bash` code block
of the **"Multi-commit decomposition"** section:

```
121:# Use reasoning for deeper analysis on the planner
122:stagecoach --reasoning high
```

Surrounding context (the code block runs lines ~113–130, then a blank line, then line 132 begins
`See [How Stagecoach works…`):

```
… (code block)
# Keep the v1 single-commit behavior
stagecoach --single

# Route planning to a bigger model (per-repo .stagecoach.toml):
# [role.planner]
# provider = "claude"
# model = "opus"
```                              ← code fence closes (~line 130)
(blank)
See [How Stagecoach works — Multi-commit decomposition](docs/how-it-works.md#…) …
```

**README house note-callout pattern**: the README already uses `> [!NOTE]`, `> [!TIP]` blocks (e.g.
lines 31, 79, 100) to attach prose qualifications to a preceding code block. So the idiomatic, low-risk
edit is to KEEP lines 121–122 verbatim (the contract forbids removing the example) and append a
`> [!NOTE]` callout AFTER the code fence closes (between the closing ``` and the `See` line). This adds
the provider-dependence qualifier without disturbing the runnable example block or any line above it.

Net: README.md gets ONE new `> [!NOTE]` block (~3–4 lines) inserted after the multi-commit code block.
Lines 121–122 are untouched.

## §3 — docs/cli.md current state (two sites; one is the contract's line 43)

`grep -n reasoning docs/cli.md` → 7 hits in two tables + the Examples block. The contract names
**line 43** (the Global-flags table row) and the example at **lines 212–213**.

**Site A — line 43 (the Global flags table row, the contract's primary target):**
```
| `--reasoning <level>` | string | "" (off; planner: high) | `STAGECOACH_REASONING` | `stagecoach.reasoning` | Global reasoning effort: off|low|medium|high |
```
- The default cell `"" (off; planner: high)` is **now accurate** — `planner = high` (the shipped
  default in `internal/config/roles.go` `defaultRoleReasoning`) now emits `--thinking`/`--effort` for
  pi/claude. **Keep it.**
- The Description cell `Global reasoning effort: off|low|medium|high` is generic and silent about
  provider-dependence. Append a concise qualifier: provider-dependent (engages for pi via `--thinking`
  and claude via `--effort`; graceful no-op otherwise; FR-R6). This satisfies the contract's
  "optionally add a note about provider-dependent support".

**Site B — lines 212–213 (Examples block):**
```
# Use reasoning for deeper analysis
stagecoach --reasoning high
```
- Already a valid, now-functional example. Leave the command; optionally tighten the comment to name the
  provider scope. Minimal-touch: keep as-is OR add a short parenthetical. The table-row note (Site A)
  carries the full qualifier, so this example need not repeat it — but a 1-line inline note keeps the
  Examples block self-consistent.

The **Flag↔env↔git-config map table** (lines 164–170) lists only the flag/env/git-key triples and has
no Description column — it needs NO edit (it's a pure mapping table; nothing is "claimed" about effect).
Leave it.

## §4 — docs/providers.md current state (already Mode-A-updated; verify-only)

`grep -n "reasoning\|effort\|thinking" docs/providers.md` → the decisive hit is **line 35**, the
`reasoning_levels` row of the 19-field schema table:

```
| `reasoning_levels` | table | nil (none) | Per-level reasoning-effort token lists (off/low/medium/high);
nil/empty ⇒ graceful no-op (FR-R6). Appended after the model flag at render. pi populates high/medium/low
via `--thinking` (verified `pi --help`); claude via `--effort` (verified `claude --help`); all other
built-ins are nil (graceful no-op). |
```

This was ALREADY updated by **Mode A** of P1.M1.T1.S1/S2 (the implementing subtasks' bundled doc line).
It already states the correct, verified flags (`--thinking` for pi, `--effort` for claude) and the
graceful-no-op policy for the other six built-ins. It is **fully consistent** with the verified tokens
in §1. Line 59 (the "Command rendering" paragraph) also correctly says reasoning tokens are appended
after the model flag when present, else silent no-op.

**Therefore (C) is a VERIFY step, not an edit.** The PRP must direct the implementer to re-read line 35
+ line 59, confirm they match §1, and leave the file byte-unchanged if they already do (they do). The
only justified edit would be if a future diff to providers.md introduced an inconsistency — none exists
today. Keep the file's edit risk at zero unless an inconsistency is found; if it IS found, fix ONLY that.

## §5 — The `--thinking-effort` anti-guard (contract requirement D)

`grep -rn "thinking-effort\|thinking_effort" README.md docs/ providers/` → **NO matches today.** The
docs already use the correct `--effort` for claude (§1, §4). Requirement (D) is therefore a
**regression guard**: after editing, re-run the same grep and assert it STILL returns nothing, so a
future edit never reintroduces the PRD's wrong guess. Add it as a hard Validation-Gate step.

## §6 — Validation approach (documentation-only; NO unit tests)

- There is **no** test harness for prose. Per the contract: "TEST: no unit test — documentation-only.
  Verify by reading the updated sections for consistency with the verified tokens."
- **markdownlint is available**: `npx markdownlint-cli2 README.md docs/cli.md providers/` runs
  (markdownlint-cli2 v0.23.0 / markdownlint v0.41.0, installed in the node_modules cache; `--no-install`
  succeeds). Repo config `.markdownlint.json` = `{default:true, MD013:false, MD033:false, MD060:false}`
  (line-length, inline-HTML, and nested-list rules OFF; everything else ON). So this IS a real,
  project-configured gate — run it on the 3 touched files.
- The Go build/test suites are UNAFFECTED (no `.go` file changes) — but run `go build ./...` as a cheap
  smoke check that nothing was accidentally touched (it must stay green and is byte-identical).
- `git status --porcelain` must show **exactly** README.md + docs/cli.md (+ docs/providers.md ONLY if an
  inconsistency forced an edit — expected NONE).

## §7 — Confidence, risks, scope boundary

- **Confidence: 9/10.** Pure prose edits to 2 well-understood files + a verify-only third. No schema,
  no code, no test surface. The only risk is markdown-table formatting (the cli.md row is a 6-column
  table — keep `|` column count identical when editing the Description cell) and not breaking the
  markdownlint config.
- **Parallel coordination**: P1.M5.T1.S1 (the other in-flight subtask) edits `internal/config/file.go` +
  `migrate.go` + `file_test.go` — ZERO overlap with the docs this task touches. No merge conflict
  possible. This task edits NO `.go` file, so it cannot perturb P1.M5.T1.S1's `go test` gate.
- **Scope boundary (frozen — do NOT touch)**: `internal/*` (all source), `providers/*.toml` (the
  shipped reference manifests — already carry `[reasoning_levels]` from P1.M1.T1.S1/S2), `PRD.md` /
  `prd_snapshot.md` / `tasks.json` (owned by humans/orchestrator), `docs/how-it-works.md` / other
  docs (out of scope; this changeset is README + cli + providers only). The bootstrap config header
  (P1.M4.T1.S1) is a SOURCE file (`internal/config/bootstrap.go`), not user-facing prose — out of scope
  here.
