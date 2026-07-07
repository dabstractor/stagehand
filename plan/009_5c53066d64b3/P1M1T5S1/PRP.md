---
name: "P1.M1.T5.S1 — README.md: add multi-turn fallback feature blurb"
description: |
  Add ONE row to the `## Features` capabilities table in `README.md` describing the
  lossless multi-turn fallback (PRD §9.24), the feature built in P1.M1.T1–T4. This is
  the marketing-surface mention required by PRD §21.5 (README structure, item 1: the
  feature surface). Mode B (this task IS the documentation; it ships no code, no CLI
  flags, no config keys into the README body).

  CONTRACT (from the work item — do not exceed):
  1. ONE sentence. A single new row in the Features table. Do not expand scope — no
     new section, no paragraph, no callout, no FAQ entry.
  2. Wording (adapt to the README's existing voice; verbatim candidate is in the
     Implementation block): "Lossless multi-turn fallback: when a one-shot generation
     of a large diff fails, stagecoach re-delivers the full diff across session turns
     so the message still lands — no truncation, no extra commits."
  3. NO CLI flags or config details in the README body beyond a POINTER to docs. The
     feature is internally triggered; its only user surfaces are the progress line
     and the two config keys (`multi_turn_fallback`, `multi_turn_chunk_tokens`),
     which are already documented in `docs/configuration.md`. The blurb links to docs
     only; it does not name the keys or the progress-line text.
  4. INPUT = the complete, tested feature from P1.M1.T1–T4 (PRD §9.24 FR-T1–T12).
     Sibling task P1.M1.T5.S2 owns FUTURE_SPEC.md consistency — do NOT touch
     FUTURE_SPEC.md here.

  Deliverable: `README.md` with exactly one new row added to the Features table,
  positioned immediately after the "Payload optimization" row. No other file changes.
---

## Goal

**Feature Goal**: Surface the lossless multi-turn fallback (PRD §9.24, built in
P1.M1.T1–T4) as a one-line entry in the README's Features table so a first-time
reader of the marketing surface learns the capability exists and can follow a link
to the full explanation.

**Deliverable**: A single new table row in `README.md`'s `## Features` capabilities
table (lines 61–72), placed directly after the "Payload optimization" row, that
describes multi-turn fallback in one sentence and links to the two docs that own its
user surfaces (`how-it-works.md` for the progress-line/trigger, `configuration.md`
for the two config keys).

**Success Definition**:
- `README.md` contains exactly one new row whose first cell is `Multi-turn fallback`.
- The row body is one sentence and contains **no** CLI flags and **no** config-key
  names (only the allowed docs links).
- Both links resolve to real, on-disk anchors (`#multi-turn-generation-fallback`,
  `#built-in-defaults`).
- The table still parses (every row has the same column count: 3 cells `| … | … |`).
- `git diff --stat` shows **only** `README.md` changed.

## User Persona

**Target User**: A developer reading the README to decide whether stagecoach handles
their large diffs. (Transitively: PRD §7.1 primary persona "the plan-holder"; §9.24
→ G21.)

**Use Case**: Scanning the Features table to see what stagecoach does. They have a
repo with very large commits where one-shot generation occasionally fails on a big
diff, and they want to know whether stagecoach recovers or just bails to the rescue
message.

**User Journey**: README → Features table → reads "Multi-turn fallback" row → clicks
"how it works" → lands on `docs/how-it-works.md#multi-turn-generation-fallback` →
understands the lossless N+1-turn priming → optionally clicks "knobs" to see the
config keys.

**Pain Points Addressed**: Without this row, a reader has no idea the fallback exists
(it is internally triggered — there is no flag to discover). They'd assume a large
diff that fails one-shot just produces a rescue message; in fact stagecoach silently
recovers losslessly when the provider supports it (pi).

## Why

- **§21.5 makes the README the marketing surface, and the Features table is its
  feature list.** A shipped, tested capability (P1.M1.T1–T4 complete) that is
  invisible in the README is a marketing gap. This row closes it.
- **The feature is internally triggered, so it is undiscoverable by flags.** The
  README row is the only place a casual reader would learn it exists. The progress
  line ("falling back to multi-turn…") only appears at fallback time, after a
  failure — too late to be the discovery surface.
- **Anchored to existing docs, not new prose.** `docs/how-it-works.md` (line 262) and
  `docs/configuration.md` (lines 137–138, 155–157) already contain the full §9.24
  treatment. The README row is a one-line pointer, not a second copy of the spec.
- **Scope discipline.** The contract forbids expanding into CLI flags / config keys
  / a new section. One row, one sentence, two links. This keeps the README scannable
  and avoids duplicating docs that already exist (DRY).

## What

Exactly **one** new row added to the Features table in `README.md`, immediately after
the "Payload optimization" row. The row mirrors the voice and link style of the
"Payload optimization" row (the closest analog — both are large-diff mechanisms).

No other change: no new section, no edit to the hero/why/install/quick-start/FAQ
blocks, no change to any other file (no docs/, no FUTURE_SPEC.md, no source).

### Success Criteria

- [ ] `README.md` Features table has one new row; first cell `Multi-turn fallback`.
- [ ] Row body is a single sentence (with the trailing clause folded in via em-dash,
      matching sibling rows); no CLI flags; no config-key names in the body.
- [ ] Row links to `docs/how-it-works.md#multi-turn-generation-fallback` and
      `docs/configuration.md#built-in-defaults` using the `[how it works](…) · [knobs](…)`
      pattern already used by the "Payload optimization" row.
- [ ] Table column count unchanged (3 cells per row); the new row parses.
- [ ] Both anchor targets exist on disk (verified below).
- [ ] `git diff --stat` ⇒ only `README.md`.

## All Needed Context

### Context Completeness Check

_Pass._ A writer with no prior knowledge of the repo can do this from: the exact row
to add (verbatim, in the Implementation block), the exact insertion point (after the
"Payload optimization" row), and the two verified anchors. No code knowledge, no Go
toolchain, no build step needed — it is a one-line markdown edit.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- file: README.md
  section: "## Features" (lines 59–72) — the capabilities table this task edits.
  why: (1) Confirms the table voice/format each row uses (`| Capability | Description + links |`).
       (2) The "Payload optimization" row (line 67) is the EXACT template to mirror — same link
       pair style `[how it works](…) · [knobs](docs/configuration.md#built-in-defaults)`.
       (3) Confirms insertion point: new row goes immediately AFTER line 67 (Payload optimization),
       BEFORE line 68 (Message shaping) — clusters the two large-diff rows together.
  pattern: each row = `| <Capability name> | <one-sentence description> ([how it works](anchor) · [knobs](anchor)). |`
  gotcha: every row MUST have exactly 3 cells (`| … | … |`) — a missing trailing pipe or an extra
          `|` inside the cell breaks the table render. The description cell may contain markdown
          links (inline `[text](url)`) but NOT a pipe — escape any literal `|` as `\|` (none needed here).

- file: docs/how-it-works.md
  section: "## Multi-turn generation fallback" (line 262)
  why: PROVES the anchor `#multi-turn-generation-fallback` exists and is the right "how it works"
       target. This section already contains the full §9.24 treatment (trigger, lossless priming,
       N+1 turns, failure→rescue, token_limit non-interaction) — so the README row only needs to
       POINT here, never re-explain.
  critical: the README body must NOT duplicate this section's content. One sentence + link only.

- file: docs/configuration.md
  section: "## Built-in defaults" (line 121) — contains the multi-turn rows at lines 137–138
           (`multi_turn_fallback` default `true`, `multi_turn_chunk_tokens` default `32000`)
           and the explanatory note at lines 155–157.
  why: PROVES the anchor `#built-in-defaults` is the correct "knobs" target — it is where the
       feature's two config keys (its only non-progress-line user surface) are documented. The
       README row's `[knobs](…)` link points here so the contract's "pointer to docs" is satisfied
       WITHOUT naming the keys in the README body.

- prd: PRD.md §9.24 (FR-T1–T12, esp. FR-T1 trigger gate, FR-T2 "lossless", FR-T5 progress line)
       and §21.5 (README structure — item 1 is the feature surface)
  why: §9.24 is the feature spec the blurb summarizes; §21.5 mandates the README is the marketing
       surface whose feature list this row joins. FR-T2 ("lossless — full diff, request-sized
       chunks… no truncation, no summarization") is the source of the "no truncation, no extra
       commits" clause; FR-T10 (one commit, role = message) is the source of "no extra commits".
```

### Current Codebase tree (relevant slice)

```bash
README.md                 # EDIT — add one row to the Features table (after the Payload optimization row)
docs/how-it-works.md      # READ-ONLY — owns anchor #multi-turn-generation-fallback (line 262)
docs/configuration.md     # READ-ONLY — owns anchor #built-in-defaults (multi-turn keys at 137–138, 155–157)
FUTURE_SPEC.md            # DO NOT TOUCH (sibling task P1.M1.T5.S2 owns its consistency)
```

### Desired Codebase tree with files to be added

```bash
README.md                 # MODIFIED — +1 row in the Features table (no new files)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL: this is a ONE-ROW, ONE-SENTENCE edit. Do not add a section, a callout, a FAQ entry,
     or a "How it works" paragraph. The contract forbids scope expansion. -->

<!-- CRITICAL: do NOT name config keys or CLI flags in the README body. The contract allows ONLY a
     pointer to docs. Write "([how it works](…) · [knobs](docs/configuration.md#built-in-defaults))"
     — the word "knobs" + the link IS the pointer; never write `multi_turn_fallback` /
     `multi_turn_chunk_tokens` / `session_mode` / `--*` in the row body. -->

<!-- CRITICAL: keep the table valid. The row must have exactly 3 pipe-delimited cells
     (`| Capability | Description |`) and NO bare `|` inside the description cell. The suggested
     wording contains no pipe, so no escaping is needed — but double-check after pasting. -->

<!-- CRITICAL: place the row AFTER "Payload optimization" (line 67), NOT at the end of the table and
     NOT before "Multi-commit decomposition". Rationale: both Payload optimization and Multi-turn are
     large-diff mechanisms, so they cluster; placing it elsewhere breaks the semantic grouping a
     reader expects (diff handling → message shaping → integrations → conveniences → discovery). -->

<!-- CRITICAL: anchor slugs. GitHub/markdownlint slugify headings by lowercasing and replacing
     spaces with `-`. "## Multi-turn generation fallback" → `#multi-turn-generation-fallback`
     (verified on disk, line 262). "## Built-in defaults" → `#built-in-defaults` (line 121).
     Do NOT invent anchors like `#multiturn` or `#multi-turn-fallback`. -->

<!-- MINOR: `.markdownlint.json` has MD013 (line-length) OFF, MD033 (inline HTML) OFF, MD060 OFF,
     default true. So a long table row is fine; no need to wrap. The new row will be long (mirrors
     the Payload optimization row) and that is acceptable. -->

<!-- MINOR: do NOT touch FUTURE_SPEC.md. Its lossy-chunking rejection (line 99) already notes the
     lossless multi-turn form "graduated to the spec — see PRD §9.24". That consistency note is
     owned by the SIBLING task P1.M1.T5.S2. This task edits README.md ONLY. -->
```

## Implementation Blueprint

### Data models and structure

None. This is a documentation edit — no data models, no code.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT README.md — insert one row into the Features table
  - FILE: README.md (root of repo).
  - LOCATE: the "## Features" section; the table whose header is `| Capability | Description |`.
  - INSERT POINT: immediately AFTER the "Payload optimization" row (the row beginning
    `| Payload optimization |` — currently the 3rd data row, around line 67) and BEFORE the
    "Message shaping" row (`| Message shaping |`).
  - ADD exactly this row (verbatim — wording finalized to match the table voice; em-dash
    folds in the contract's parenthetical "(no truncation, no extra commits)"; link pair
    mirrors the "Payload optimization" row):

      | Multi-turn fallback | Lossless multi-turn fallback: when a one-shot generation of a large diff fails, stagecoach re-delivers the full diff across session turns so the message still lands — no truncation, no extra commits ([how it works](docs/how-it-works.md#multi-turn-generation-fallback) · [knobs](docs/configuration.md#built-in-defaults)). |

  - NAMING/VOICE: first cell `Multi-turn fallback` (matches the `Multi-*` / two-word capability
    naming of sibling rows like `Multi-commit decomposition`, `Payload optimization`).
  - PRESERVE: every other row, the table header + separator, and all surrounding sections.
    Do NOT reflow or reword any existing row.

Task 2: VERIFY (no further file change)
  - RUN the Validation Loop (Level 1 grep checks + optional markdownlint + render check).
  - `git diff --stat` ⇒ only README.md; the diff is a pure insertion of one line.
```

### Implementation Patterns & Key Details

```markdown
<!-- The exact row to add (copy verbatim). It is one logical line (markdown tables do not wrap
     across lines); keep it on a single source line like every other row in the table. -->

| Multi-turn fallback | Lossless multi-turn fallback: when a one-shot generation of a large diff fails, stagecoach re-delivers the full diff across session turns so the message still lands — no truncation, no extra commits ([how it works](docs/how-it-works.md#multi-turn-generation-fallback) · [knobs](docs/configuration.md#built-in-defaults)). |

<!-- Why this wording (traces each clause to the spec, so a reviewer can audit it):
  - "Lossless multi-turn fallback:"         → FR-T2 ("Lossless — full diff, request-sized chunks").
  - "when a one-shot generation of a large  → FR-T1 (fallback, not default; activates only AFTER
    diff fails,"                               the one-shot retry loop exhausts; cond a + payload>chunk).
  - "stagecoach re-delivers the full diff    → FR-T2 (the SAME captured payload, unmodified — no
    across session turns"                     summarization, no truncation) + FR-T4 (N+1 turn protocol).
  - "so the message still lands"            → FR-T7 (best-effort; on failure falls to rescue — never
                                              worse than one-shot-exhausted). "lands" = success case.
  - "— no truncation, no extra commits"     → FR-T2 ("no truncation") + FR-T10 (multi-turn produces
                                              ONE commit, role = message; distinct from decomposition
                                              §9.14 which makes N commits).
  - "[how it works](…)"                     → the progress line + trigger + lossless priming live here.
  - "[knobs](…#built-in-defaults)"          → the two config keys (the only non-progress user surface).
-->
```

### Integration Points

```yaml
DOCS (the only integration surface):
  - README.md Features table: +1 row (Task 1).
  - docs/how-it-works.md: UNCHANGED (already has §9.24 at line 262; the README links TO it).
  - docs/configuration.md: UNCHANGED (already has the two keys at lines 137–138 + note 155–157;
    the README links TO #built-in-defaults).

NO SOURCE / NO CLI / NO CONFIG SCHEMA / NO FUTURE_SPEC / NO tasks.json / NO PRD.md.
This is a 1-line markdown insertion. Mode B (doc-only).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# (a) The new row + capability name are present, exactly once.
grep -c '^| Multi-turn fallback |' README.md
# Expected: 1   (exactly one row whose first cell is "Multi-turn fallback")

# (b) Both anchors are referenced by the new row.
grep -F 'docs/how-it-works.md#multi-turn-generation-fallback' README.md
grep -F 'docs/configuration.md#built-in-defaults' README.md
# Expected: each prints the new row (the "Payload optimization" row also references the config
# anchor, so the second grep may print 2 lines — that is fine; the point is the new row is among them).

# (c) The contract's forbidden tokens do NOT appear in the README body
#     (no CLI flags, no config-key names in prose).
! grep -nE 'multi_turn_fallback|multi_turn_chunk_tokens|session_mode|--no-session|--session-id' README.md \
  && echo "OK: no config keys/flags leaked into README" \
  || echo "FAIL: a config key or flag leaked into the README body"
# Expected: OK (the keys live only in docs/, which this task does not edit).

# (d) The anchor targets actually exist on disk.
grep -q '^## Multi-turn generation fallback$' docs/how-it-works.md && echo "anchor how-it-works OK"
grep -q '^## Built-in defaults$'            docs/configuration.md && echo "anchor configuration OK"
# Expected: both print OK.

# (e) Optional — markdownlint if the runner has it (Makefile has no md target; .markdownlint.json exists).
npx --yes markdownlint-cli2 README.md 2>/dev/null && echo "markdownlint clean" \
  || echo "markdownlint-cli2 unavailable or reported issues — review manually (MD013/MD033/MD060 are OFF in .markdownlint.json; a long table row is allowed)."
# Expected: clean, OR the fallback message (a long row is not a violation under the configured rules).
```

### Level 2: Unit Tests (Component Validation)

```bash
# Not applicable — this is a markdown documentation edit with no code, no tests, no build step.
# The "unit test" is the Level 1 grep assertions + the render check below.
echo "Level 2 N/A (documentation-only change; no Go code or tests involved)."
```

### Level 3: Integration Testing (System Validation)

```bash
# (a) The table still parses: every data row has exactly 3 pipe-delimited cells
#     (header + separator + each data row). Count cells per row in the Features table.
awk '
  /^## Features/      { in_feat=1; next }
  /^## / && in_feat   { in_feat=0 }
  in_feat && /^\|/    { n=gsub(/\|/,"|"); print n-1" cells: "$0 }
' README.md
# Expected: every printed row shows "2" (=> 3 cells: | a | b |). If any row shows !=2, the table is broken.

# (b) The diff is a pure single-line insertion (no other row edited).
git diff --stat -- README.md
# Expected: only README.md; the hunks are an insertion (green +1 line), no red removals of existing rows.
git diff -- README.md | grep '^-' | grep -v '^---'
# Expected: EMPTY (no existing lines removed). If anything prints, an existing row was altered — revert it.

# (c) Render check — confirm the row renders as a table row, not a paragraph.
#     (Pipe the README into any markdown renderer you have, or eyeball that the new row sits between
#      "Payload optimization" and "Message shaping" with the same indentation/pipes as its neighbors.)
sed -n '/^## Features/,/^## /p' README.md | grep -E 'Payload optimization|Multi-turn fallback|Message shaping'
# Expected order: Payload optimization … THEN Multi-turn fallback … THEN Message shaping.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# (a) Voice consistency — the new row's link style matches the "Payload optimization" row exactly
#     (same `[how it works](…) · [knobs](…)` pattern; no new convention introduced).
grep -E '\[how it works\]\(.*\) · \[knobs\]\(docs/configuration\.md#built-in-defaults\)' README.md
# Expected: prints at least 2 rows (Payload optimization + Multi-turn fallback) — same convention.

# (b) Scope guard — no new H2/H3 section, no callout (> [!NOTE]), no FAQ entry was added for multi-turn.
grep -cE '^#{2,3} .*[Mm]ulti-turn' README.md
# Expected: 0  (the feature is a table ROW, not a section). If >0, scope was expanded — revert.

# (c) Cross-link sanity — the two docs the README points at still describe multi-turn
#     (guards against the anchors being moved/renamed in a parallel doc edit).
grep -q 'multi_turn_fallback' docs/configuration.md && echo "config keys still present"
grep -q 'Multi-turn generation fallback' docs/how-it-works.md && echo "how-it-works section still present"
# Expected: both OK. (docs/ are read-only for this task; this just confirms the links aren't dangling.)
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `grep -c '^| Multi-turn fallback |' README.md` ⇒ `1`; both anchors referenced;
      no config-key/flag tokens leaked into README; both anchor targets exist on disk.
- [ ] Level 3: every Features-table row has 3 cells (cell-count == 2 per the awk); the new row sits
      between "Payload optimization" and "Message shaping".
- [ ] `git diff --stat` ⇒ only `README.md`; `git diff | grep '^-'` (minus the `---` header) is empty.

### Feature Validation

- [ ] One new row; first cell `Multi-turn fallback`.
- [ ] Row body is a single sentence; no CLI flags; no config-key names (only the `[knobs]` link).
- [ ] Both links resolve (`#multi-turn-generation-fallback`, `#built-in-defaults`).
- [ ] Wording matches the contract's intent (lossless / one-shot-fails / re-delivers across turns /
      message still lands / no truncation / no extra commits).
- [ ] Row placed immediately after "Payload optimization".

### Code Quality Validation

- [ ] Mirrors the existing "Payload optimization" row's voice and `[how it works](…) · [knobs](…)` link
      style — no new convention.
- [ ] No other README section, row, or file touched.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] The README row is a pointer to (not a duplicate of) the existing docs treatment — no DRY violation.
- [ ] No new env vars, flags, or config introduced (this task adds none; it only documents an existing
      feature's existence on the marketing surface).

---

## Anti-Patterns to Avoid

- ❌ Don't expand scope — one row, one sentence. No new `##`/`###` section, no `> [!NOTE]` callout, no
  FAQ entry, no "How it works" paragraph. The contract is explicit: keep it to one sentence.
- ❌ Don't name config keys (`multi_turn_fallback`, `multi_turn_chunk_tokens`) or flags (`--session-id`,
  `--no-session`) or `session_mode` in the README body. The `[knobs](…)` link is the only allowed pointer.
- ❌ Don't invent anchors. Use the verified on-disk slugs `#multi-turn-generation-fallback` and
  `#built-in-defaults` — not `#multiturn`, `#multi-turn-fallback`, or `#multi_turn`.
- ❌ Don't place the row at the end of the table or before "Multi-commit decomposition". It goes right
  after "Payload optimization" (both are large-diff mechanisms — cluster them).
- ❌ Don't duplicate the §9.24 spec into the README. `docs/how-it-works.md` (line 262) already has the
  full treatment; the README row links to it.
- ❌ Don't touch `FUTURE_SPEC.md` — its lossy-chunking-vs-lossless-multiturn note (line 99) is owned by
  the sibling task P1.M1.T5.S2.
- ❌ Don't edit `docs/how-it-works.md` or `docs/configuration.md` — they already contain everything the
  README links to; this task only adds the README pointer.
- ❌ Don't reword or reflow existing README rows — the diff must be a pure single-line insertion.
- ❌ Don't run a Go build/test as if this were code — it's a markdown edit; `make build`/`make test` are
  irrelevant here and would only mask a doc regression.
