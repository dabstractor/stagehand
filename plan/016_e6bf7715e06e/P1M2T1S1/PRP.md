name: "P1.M2.T1.S1 — Add Chrome-disable column and asymmetry bullet to docs/providers.md (FR-C5, §9.28/§12.7.1)"
description: >
  Pure Mode-B documentation task. Two edits to docs/providers.md: (1) add a "Chrome-disable" column to
  the "## The 7 built-in providers" table (inserted between "Tool-disable approach" and "Stager?"),
  with one short phrase per provider recording the verified chrome status; (2) add a "Chrome is a
  separate axis" bullet to the "## Tools-disable asymmetry" section. The per-provider column values are
  the ones specified in the work-item contract and are VERIFIED against the CHROME-DISABLE notes in
  internal/provider/builtin.go (written by the completed P1.M1.T1.S1) — this doc faithfully records
  that verified manifest state. NO Go code, NO tests, NO other doc files (how-it-works.md / docs/README.md
  / README.md are the separate P1.M2.T1.S2 subtask; providers/*.toml already mirror builtin.go).

---

## Goal

**Feature Goal**: Make docs/providers.md honestly surface the chrome-disable story (FR-C1–C5, §9.28) that
the manifests already implement: a per-provider Chrome-disable column in the 7-provider table and a
"chrome is a separate axis from mutation safety" bullet in the asymmetry section, so a reader can see at
a glance which providers switch chrome off (pi, claude) and which document the read-only limitation
instead (codex, cursor, opencode, agy, qwen-code).

**Deliverable**: Two edits to **`docs/providers.md`** only:
1. A new **Chrome-disable** column in the "## The 7 built-in providers" table (header + separator + all 7
   data rows), placed after "Tool-disable approach" and before "Stager?".
2. A new **"Chrome is a separate axis"** bullet appended to the "## Tools-disable asymmetry" section
   (after the "Both approaches satisfy the §18.1 safety invariant…" concluding sentence).

**Success Definition**:
- The 7-provider table renders with **9 columns** (the 8 existing + Chrome-disable), every row has
  exactly 9 cells, and the Chrome-disable cell appears between "Tool-disable approach" and "Stager?".
- The Chrome-disable values match the contract (pi: extensions/skills/templates/context off + MCP
  caveat; claude: `--tools ""` + `--setting-sources ""`; the five read-only providers: "no per-surface
  switch; … — documented limitation", with opencode's variant reading "read-only by design").
- The asymmetry section has a new bullet stating chrome is a separate axis from mutation safety,
  cross-referencing the Chrome-disable column and the CHROME-DISABLE manifest notes (FR-C1–C5, §9.28).
- No other file changed; the existing "Tool-disable approach"/"Stager?" columns and the two existing
  asymmetry bullets + their concluding sentence are preserved.

## User Persona (if applicable)

**Target User**: A developer reading docs/providers.md to understand provider safety/isolation —
specifically what each agent does about agent chrome (skills, extensions, context files, MCP servers)
on a one-shot commit-message call.

**Use Case**: "I'm evaluating codex/cursor/opencode — do they load my MCP servers and skills when
stagecoach calls them? Is that safe / deterministic / costly?" The Chrome-disable column answers per
provider; the asymmetry bullet explains the axis split.

**User Journey**: Open docs/providers.md → the 7-provider table now shows a Chrome-disable column →
scan pi/claude (explicit switches) vs the read-only providers (documented limitation) → read the
asymmetry section's new bullet for the conceptual framing (chrome ≠ mutation safety).

**Pain Points Addressed**: FR-C5's "tracking duty" — chrome status must be documented, not hidden. Today
the table has no chrome column and the asymmetry section covers mutation safety only; a reader cannot
see the chrome story without reading manifest source comments.

## Why

- **FR-C5 / §9.28 / §12.7.1**: the chrome-disable verification (P1.M1.T1) added CHROME-DISABLE notes to
  every manifest; FR-C5 mandates the §12.7.1 asymmetry table gain a "Chrome-disable" column and that
  limitations be documented, not assumed. This task is the docs half of that duty.
- **Honesty**: chrome is a separate axis from mutation safety. The existing asymmetry section honestly
  covers mutation safety (explicit switch vs read-only constraint); it must equally honestly state that
  read-only ≠ chrome-less, and name which providers switch chrome off vs document the gap.
- **Bounded scope**: two edits to one markdown file. No code, no tests, no schema change. The source of
  truth (builtin.go CHROME-DISABLE notes) already exists and is verified; this doc records it.

## What

**User-visible behavior**: docs/providers.md gains a Chrome-disable table column and a chrome-axis
asymmetry bullet. No runtime behavior change.

**Technical change** (two markdown edits; verbatim content in the Implementation Blueprint):

1. **Table** (lines 78-86): insert a Chrome-disable column. New header row:
   `| Provider | Delivery | Print flag | Model flag | Default model | System prompt flag | Tool-disable approach | Chrome-disable | Stager? |`
   …+ a separator dash column, + one Chrome-disable cell in each of the 7 data rows (values in §Blueprint).
2. **Asymmetry section** (after line 98): append a new bullet:
   `- **Chrome is a separate axis** (all providers): Mutation safety says nothing about agent chrome …`

### Success Criteria
- [ ] Table header has 9 columns incl. `Chrome-disable` between `Tool-disable approach` and `Stager?`
- [ ] Table separator has 9 dash-columns
- [ ] All 7 data rows have exactly 9 cells (Chrome-disable cell inserted before Stager?)
- [ ] Chrome-disable values match the contract (pi/claude explicit; 5 read-only providers documented limitation; opencode "by design")
- [ ] Asymmetry section has the new "Chrome is a separate axis" bullet after the concluding sentence
- [ ] The two existing asymmetry bullets and the "Both approaches satisfy the §18.1 safety invariant…" sentence are preserved
- [ ] Only docs/providers.md changed; no other file touched

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact current table (header/separator/7 rows with line numbers), the exact new header, the
exact per-row Chrome-disable cell values (in the table's actual row order), the exact asymmetry bullet
text and placement, the markdown gotchas (em-dashes/double-quotes/backticks fine; pipes break cells;
cell-count must be 9; MD013 off so wide rows are fine), the value-verification table proving the cells
match builtin.go, and the explicit scope fences (only docs/providers.md).

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim edits + value verification + gotchas)
- docfile: plan/016_e6bf7715e06e/P1M2T1S1/research/findings.md
  why: "§1 has the exact table header/separator/per-row cell values in the TABLE's row order
        (pi,claude,opencode,codex,cursor,agy,qwen-code — NOT the contract's listing order); §2 the
        value-verification table (contract vs builtin.go); §3 the exact asymmetry bullet + placement;
        §4 scope fences; §6 the markdown/validation approach."
  critical: "§1: map the contract's per-provider values to the ACTUAL table row order. §1 gotchas: every
             row MUST have exactly 9 cells; never put a pipe inside a cell; MD013 is OFF so wide rows are
             fine — do NOT hand-align the long column."

# MUST READ — the file being edited
- file: docs/providers.md
  why: "The '## The 7 built-in providers' table is at lines 78-86 (header 78, separator 79, rows 80-86);
        the '## Tools-disable asymmetry' section is at lines 90-98 (concluding sentence at 98). Both
        edits land here. LOCATE by content via grep (line numbers are advisory)."
  pattern: "The table is a standard markdown pipe table (8 cols → 9 cols). The asymmetry section is a
            2-bullet list + a concluding sentence; the new chrome bullet is appended after the conclusion."
  gotcha: "Line numbers may drift if a parallel task touched the file — locate the table header with
           `grep -n 'Tool-disable approach' docs/providers.md` and the asymmetry section with
           `grep -n '## Tools-disable asymmetry' docs/providers.md`."

# MUST READ — the source of truth this doc records (CHROME-DISABLE notes, written by completed P1.M1.T1.S1)
- file: internal/provider/builtin.go
  why: "Each provider's CHROME-DISABLE note (pi @43, claude @120, agy @212, qwen-code @266, opencode @310,
        codex @363, cursor @407) is the verified per-provider chrome status. The column values MUST agree
        with these (findings §2 confirms they do). Read them if any value is questioned."
  critical: "Do NOT edit builtin.go. It is the source of truth this doc RECORDS (Mode B). If a value seems
             wrong, the fix is in the manifest (a different task), not here — but the contract values are
             already verified-correct."

# MUST READ — the PRD spec for chrome-disable (the requirement this doc surfaces)
- docfile: plan/016_e6bf7715e06e/prd_snapshot.md
  why: "§9.28 (FR-C1–C5) defines chrome surfaces and the disable/document discipline; §12.7.1 consequence
        #4 is the 'chrome is a separate axis' statement the new bullet encodes. Cite FR-C1–C5/§9.28 in
        the bullet."
  section: "§9.28 Chrome-disable for every provider; §12.7.1 The tools-disable asymmetry"

# CONTEXT — the parallel sibling (no overlap; confirms the same values)
- docfile: plan/016_e6bf7715e06e/P1M1T2S1/PRP.md
  why: "Parallel sibling adds TestBuiltinManifests_ChromeDisableContract to internal/provider/builtin_test.go
        — Go assertions on BareFlags. It does NOT touch any docs file. It confirms the SAME BareFlags
        values this doc records, so the two tasks are consistent. No overlap, no conflict."

# CONTEXT — the architecture overview (what changes, what doesn't)
- docfile: plan/016_e6bf7715e06e/architecture/system_context.md
  why: "The 'Documentation surfaces' section names docs/providers.md as the target (7-provider table +
        asymmetry section, 'No Chrome-disable column' today — this task adds it). The 'Files to touch'
        table maps docs/providers.md → M2.T1 (this task)."
  critical: "It confirms docs/how-it-works.md, docs/README.md, and top-level README.md are SEPARATE
             surfaces (the M2.T1.S2 sibling), NOT this task."
```

### Current Codebase tree (relevant slice)

```bash
docs/
  providers.md        # EDIT — +Chrome-disable table column (lines 78-86) + chrome asymmetry bullet (after line 98)
internal/provider/
  builtin.go          # READ-ONLY — CHROME-DISABLE notes are the source of truth (pi@43 claude@120 agy@212 qwen@266 opencode@310 codex@363 cursor@407)
  builtin_test.go     # READ-ONLY — parallel P1.M1.T2.S1 adds chrome-contract tests here (no overlap)
.markdownlint.json    # READ-ONLY — {MD013:false (line length OFF), MD033:false, MD060:false}; no make target for docs
Makefile              # READ-ONLY — `make lint` is golangci-lint (Go only); docs not linted by make
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
docs/providers.md     # +1 table column (Chrome-disable) across header/separator/7 rows; +1 asymmetry bullet
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (cell count = 9): after the edit every table row — header, separator, and all 7 data rows —
     must have exactly 9 cells (8 content columns). A missing/extra cell breaks GitHub table rendering.
     Count the pipes: a 9-column row has 10 pipes (| ... | ... |). -->

<!-- CRITICAL (no pipes inside cells): none of the Chrome-disable values may contain a literal '|'
     (it would split the cell). The contract values contain em-dashes (—), parentheses, and double-quotes
     ("") — all safe in markdown table cells. Backticks around flags (--no-*, --tools, --setting-sources)
     are also safe and match the existing table's flag-backtick convention. -->

<!-- CRITICAL (MD013 is OFF; do NOT hand-align): .markdownlint.json disables MD013 (line length), and there
     is NO markdownlint make target. The Chrome-disable column makes rows wide — that's fine; markdown
     renders regardless of source spacing. Do NOT waste effort hand-aligning the long column; just keep
     the pipe count consistent. -->

<!-- GOTCHA (row order ≠ contract order): the contract lists providers as pi,claude,agy,qwen-code,opencode,
     codex,cursor — but the TABLE's actual row order is pi,claude,opencode,codex,cursor,agy,qwen-code.
     Map each Chrome-disable value to the correct row by provider NAME, not by listing position.
     (4 read-only providers share identical text; opencode is the "by design" variant.) -->

<!-- GOTCHA (preserve the existing asymmetry block): keep the two mutation-safety bullets AND the
     "Both approaches satisfy the §18.1 safety invariant…" concluding sentence intact. The new chrome
     bullet is APPENDED after that sentence (with a blank line before it) — it is a NEW axis, not a
     replacement for the mutation-safety summary. -->

<!-- GOTCHA (Mode B — record, don't change the source): if a Chrome-disable value seems to disagree with
     builtin.go, the discrepancy is fixed in the manifest (a different task), not by editing the doc to
     diverge further. The contract values are already verified against builtin.go (findings §2). -->
```

## Implementation Blueprint

### Data models and structure
None. Pure markdown. No types, no code.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/providers.md — add the Chrome-disable column to the 7-provider table
  - LOCATE the table: `grep -n 'Tool-disable approach' docs/providers.md` → the header row (line ~78).
    The table spans header(78) + separator(79) + 7 data rows(80-86).
  - REPLACE the header row with (9 columns — Chrome-disable inserted before Stager?):
      | Provider | Delivery | Print flag | Model flag | Default model | System prompt flag | Tool-disable approach | Chrome-disable | Stager? |
  - REPLACE the separator row with (one more dash column):
      |----------|----------|-----------|-----------|----------------|-------------------|----------------------|----------------|--------|
  - IN EACH of the 7 data rows, insert the Chrome-disable cell IMMEDIATELY BEFORE the Stager? cell.
    Values (by provider NAME — the table's row order is pi, claude, opencode, codex, cursor, agy, qwen-code):
      pi        → `extensions/skills/templates/context off (`--no-*`); MCP use suppressed (servers may connect — tracked limitation)`
      claude    → `via `--tools ""` + `--setting-sources ""``
      opencode  → `no per-surface switch; read-only by design — documented limitation`
      codex     → `no per-surface switch; read-only constraint only — documented limitation`
      cursor    → `no per-surface switch; read-only constraint only — documented limitation`
      agy       → `no per-surface switch; read-only constraint only — documented limitation`
      qwen-code → `no per-surface switch; read-only constraint only — documented limitation`
  - PRESERVE every other cell in every row (Delivery/Print flag/Model flag/Default model/System prompt
    flag/Tool-disable approach/Stager? unchanged). Keep the existing "Note:" paragraph below the table.
  - VERIFY each row has exactly 9 cells (10 pipes).

Task 2: EDIT docs/providers.md — append the "Chrome is a separate axis" bullet to the asymmetry section
  - LOCATE the section: `grep -n '## Tools-disable asymmetry' docs/providers.md` (line ~90). Find the
    concluding sentence `Both approaches satisfy the §18.1 safety invariant: no provider can mutate the
    repository.` (line ~98).
  - INSERT, AFTER that concluding sentence (with a blank line before the new bullet):
      - **Chrome is a separate axis** (all providers): Mutation safety says nothing about agent chrome (skills, extensions, context files, MCP servers). Providers that expose a per-surface disable switch set it (pi, claude); providers that do not document the limitation honestly (codex, cursor, opencode, agy, qwen-code) — the call stays read-only and never-mutate regardless. See the **Chrome-disable** column above and the CHROME-DISABLE notes in each provider manifest (FR-C1–C5, §9.28).
  - PRESERVE the two existing bullets ("Explicit switch" / "Read-only constraint") and the concluding
    sentence verbatim.

Task 3: VERIFY — grep guards + render check + scope guard
  - grep guards + manual render (see Validation Loop)
  - git diff --name-only  → only docs/providers.md
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: the new header (9 cols) -->
| Provider | Delivery | Print flag | Model flag | Default model | System prompt flag | Tool-disable approach | Chrome-disable | Stager? |
|----------|----------|-----------|-----------|----------------|-------------------|----------------------|----------------|--------|

<!-- PATTERN: a data row before/after (pi shown) -->
BEFORE: | `pi` | stdin | `-p` | `--model` | "" (user must set) | `--system-prompt` | Explicit `--no-*` flags | ✓ yes |
AFTER:  | `pi` | stdin | `-p` | `--model` | "" (user must set) | `--system-prompt` | Explicit `--no-*` flags | extensions/skills/templates/context off (`--no-*`); MCP use suppressed (servers may connect — tracked limitation) | ✓ yes |

<!-- PATTERN: the asymmetry bullet (appended after the concluding sentence, blank line before it) -->

Both approaches satisfy the §18.1 safety invariant: no provider can mutate the repository.

- **Chrome is a separate axis** (all providers): Mutation safety says nothing about agent chrome …
```

### Integration Points

```yaml
NO code / tests / schema / config / routes. ONE markdown file edited (docs/providers.md).

DOCS (docs/providers.md):
  - table: +Chrome-disable column (header + separator + 7 rows), between Tool-disable approach and Stager?.
  - asymmetry section: +"Chrome is a separate axis" bullet after the concluding sentence.

SCOPE FENCES: NO Go code; NO tests; NO builtin.go/builtin_test.go (source of truth + parallel sibling);
  NO providers/*.toml (already mirror builtin.go); NO docs/how-it-works.md, docs/README.md, README.md
  (the separate P1.M2.T1.S2 subtask); NO PRD.md/tasks.json/prd_snapshot.md (read-only).
```

## Validation Loop

> Docs are NOT linted by `make` (`.markdownlint.json` exists but there is no markdownlint Makefile
> target; `make lint` is golangci-lint for Go only). MD013 (line length) is OFF, so wide rows are fine.
> Validation is manual (render) + grep guards.

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Scope guard: only docs/providers.md changed.
git diff --name-only
# Expected: docs/providers.md (only).

# Confirm no Go file changed (this is a docs-only task).
git diff --stat -- '*.go'
# Expected: empty.

# (Optional) markdownlint if the tool is installed locally — baseline is green except known pre-existing.
npx --no-install markdownlint-cli2 docs/providers.md 2>/dev/null || echo "(markdownlint not installed / not in make — skip; MD013 is off so wide rows are fine)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# No tests are authored by this task. Run the Go suite ONLY to prove the working tree is otherwise clean
# (a docs edit cannot break Go tests, but parallel work may be in flight).
make test
# Expected: green (race detector). Unaffected by this docs edit.
```

### Level 3: Integration Testing (System Validation)

```bash
# Manual render check — the table must render with 9 columns and no broken rows.
# (Use any markdown renderer; glow is common.)
glow docs/providers.md 2>/dev/null | sed -n '/The 7 built-in/,/Note: cursor/p' || sed -n '74,88p' docs/providers.md
# Expected: the table renders with a Chrome-disable column between Tool-disable approach and Stager?,
#           all 7 rows intact, the "Note:" paragraph below unchanged.

# If glow isn't available, eyeball the raw markdown: every row from the header through qwen-code must
# have the same number of pipes (10 pipes = 9 columns).
awk 'NR>=78 && NR<=86 { n=gsub(/\|/,"|"); print NR": "n" pipes" }' docs/providers.md
# Expected: every line prints "10 pipes" (9 columns). A row with !=10 pipes is a broken cell count — fix it.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1: the Chrome-disable header column is present exactly once.
grep -c '| Chrome-disable |' docs/providers.md
# Expected: 1.

# Grep guard 2: every data row has a Chrome-disable cell — assert the load-bearing values are present.
grep -c 'documented limitation' docs/providers.md     # the 5 read-only providers (5 in the column + possibly the bullet)
grep -c 'MCP use suppressed' docs/providers.md        # pi's MCP caveat
grep -c -- '--tools "" + `--setting-sources ""`' docs/providers.md   # claude
grep -c 'read-only by design' docs/providers.md       # opencode variant
# Expected: each ≥1 (the column is populated for all 7 rows).

# Grep guard 3: the asymmetry bullet is present.
grep -c 'Chrome is a separate axis' docs/providers.md
# Expected: 1.

# Grep guard 4: the existing columns/sections are PRESERVED (the edit added, not replaced).
grep -c 'Tool-disable approach' docs/providers.md     # still the column header
grep -c 'Stager?' docs/providers.md                   # still the last column
grep -c 'Both approaches satisfy the §18.1 safety invariant' docs/providers.md  # concluding sentence intact
grep -c 'Explicit switch' docs/providers.md           # bullet 1 intact
grep -c 'Read-only constraint' docs/providers.md      # bullet 2 intact
# Expected: each ≥1.

# Grep guard 5: scope — no other doc/Go file changed.
git diff --name-only | grep -v 'docs/providers.md'
# Expected: empty (no output).

# Grep guard 6: no stray pipe inside a Chrome-disable cell broke a row (cell count consistency).
awk 'NR>=78 && NR<=86 { n=gsub(/\|/,"|"); if (n!=10) print "BAD row "NR": "n" pipes" }' docs/providers.md
# Expected: no "BAD row" output (all rows have 10 pipes / 9 columns).
```

## Final Validation Checklist

### Technical Validation
- [ ] `git diff --name-only` == only `docs/providers.md`
- [ ] `git diff --stat -- '*.go'` empty (docs-only)
- [ ] `make test` green (working tree otherwise clean — no behavioral regression)

### Feature Validation
- [ ] Table has 9 columns; every row (header/separator/7 data) has exactly 10 pipes
- [ ] Chrome-disable column sits between "Tool-disable approach" and "Stager?"
- [ ] Per-provider values match the contract (pi/claude explicit; 5 read-only documented limitation; opencode "by design")
- [ ] Asymmetry section has the new "Chrome is a separate axis" bullet after the concluding sentence
- [ ] Table renders correctly (glow/preview): no broken rows

### Scope-Boundary Validation
- [ ] Only docs/providers.md changed
- [ ] The two existing asymmetry bullets + "Both approaches satisfy the §18.1 safety invariant…" sentence preserved
- [ ] The "Note:" paragraph below the table preserved
- [ ] No Go code / test / builtin.go / providers/*.toml change
- [ ] No docs/how-it-works.md, docs/README.md, README.md change (those are P1.M2.T1.S2)

### Code Quality & Docs
- [ ] Values verified against builtin.go CHROME-DISABLE notes (Mode B records the verified state)
- [ ] Flag tokens back-ticked for consistency with the existing table style
- [ ] The bullet cross-references the Chrome-disable column + CHROME-DISABLE notes + cites FR-C1–C5/§9.28

---

## Anti-Patterns to Avoid

- ❌ Don't hand-align the wide Chrome-disable column. MD013 (line length) is OFF and there's no markdown
  `make` target; markdown renders regardless of source spacing. Hand-aligning 9 columns of varying width
  is unreadable and fragile. Just keep the pipe count consistent (10 pipes/row).
- ❌ Don't get the cell count wrong. Every row — header, separator, and all 7 data rows — must have
  exactly 9 columns (10 pipes). A missing Chrome-disable cell on any row breaks GitHub's table rendering
  for that row. The Level 3/4 awk guard (10 pipes/row) catches this.
- ❌ Don't map the contract's provider order to the table rows by position. The contract lists
  pi,claude,agy,qwen-code,opencode,codex,cursor; the TABLE's actual row order is
  pi,claude,opencode,codex,cursor,agy,qwen-code. Match each Chrome-disable value to its row by provider
  NAME. (4 read-only providers share identical text; opencode is the "by design" variant — don't mix them up.)
- ❌ Don't put a literal `|` inside a Chrome-disable cell. It would split the cell and break the row. The
  contract values contain em-dashes/parens/double-quotes (all safe) but no pipes — keep it that way.
- ❌ Don't replace or rewrite the existing asymmetry bullets or the concluding sentence. The new chrome
  bullet is APPENDED (after the "Both approaches satisfy the §18.1 safety invariant…" sentence, with a
  blank line before it). The mutation-safety discussion stays coherent as its own block.
- ❌ Don't edit any file other than docs/providers.md. The other doc surfaces (how-it-works.md,
  docs/README.md, README.md) are the separate P1.M2.T1.S2 subtask; builtin.go/builtin_test.go are the
  source of truth + the parallel sibling's tests; providers/*.toml already mirror builtin.go.
- ❌ Don't "fix" a Chrome-disable value that seems to disagree with builtin.go by diverging in the doc.
  The contract values are verified against the CHROME-DISABLE notes (findings §2). If a real discrepancy
  existed, the fix would be in the manifest (a different task) — but there is none; they agree.
- ❌ Don't add a column anywhere other than between "Tool-disable approach" and "Stager?". The contract
  specifies "after Tool-disable approach"; placing it elsewhere breaks the cross-reference in the
  asymmetry bullet ("See the Chrome-disable column above").

---

## Confidence Score: 10/10

This is a two-edit markdown change to one file, with the exact new header, the exact per-row cell values
(in the table's actual row order), the exact asymmetry bullet text and placement, a value-verification
table proving the cells match the builtin.go source of truth, explicit markdown gotchas (cell count = 9,
no pipes in cells, MD013 off, don't hand-align), and an awk-based guard that mechanically verifies every
row has 10 pipes. No code, no tests, no schema. The only judgment call (bullet placement after the
concluding sentence vs between the bullets) is resolved with a stated rationale (keep the mutation-safety
block coherent, then introduce chrome as a separate axis — mirrors PRD §12.7.1 structure). One-pass
success is essentially guaranteed.
