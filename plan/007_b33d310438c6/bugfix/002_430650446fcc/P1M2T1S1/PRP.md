---
name: "P1.M2.T1.S1 (bugfix 002 Mode B) — Sweep README.md, docs/how-it-works.md, docs/configuration.md for token_limit / truncation accuracy"
description: |
  Mode-B changeset-level documentation sweep (the FINAL task, depending on P1.M1.T1.S1 + P1.M1.T2.S1).
  Verify the three user-facing doc surfaces that mention `token_limit` / the FR3i water-fill truncation /
  the FR3d "payload always fits your context window" contract are CONSISTENT with the now-restored shipped
  behavior, and edit ONLY if a statement is genuinely stale or misleading. The fix (P1.M1.T1.S1) line-
  anchored `splitDiffSections` so a non-markdown file whose CONTENT contains the literal `diff --git `
  (test fixtures, golden snapshots, .patch/.diff files, docs quoting diffs) is ONE section — sized and
  truncated as a unit — restoring the FR3d contract that the docs already describe. **The EXPECTED outcome
  is a NO-OP CONFIRMATION**: the docs describe the *intended* (now-restored) behavior, not new behavior,
  and do NOT reference the internal split mechanism (system_context.md §7.2). Documentation editing only;
  no source code, no tests, no PRD.

  ⚠️ **THE central design call — this is VERIFICATION-FIRST with a no-op default; do NOT invent edits.**
  Read each surface, check every statement about (a) `token_limit` / water-fill truncation / the
  payload-fits-context-window contract, and (b) how the non-markdown aggregate becomes per-file sections,
  against the now-restored shipped behavior. The default and expected verdict for EACH surface is
  "accurate — no edit": the docs were never made inaccurate by describing new behavior; the fix makes
  reality match the docs (a latent gap existed only for the trigger class, closed from the implementation
  side). The deliverable is the VERIFICATION RECORD (a per-surface verdict table) + an empty `git diff` for
  the three files. Edit ONLY if a statement is genuinely stale or misleading (see the strict edit criteria).

  ⚠️ **THE second design call — do NOT leak the internal split mechanism into user-facing docs.** The fix
  is a correctness defect in an INTERNAL pure function (`splitDiffSections`). Adding a note like "files
  containing `diff --git` text in their content are handled correctly" would (a) describe an internal
  robustness/mechanism detail the contract FORBIDS in user-facing docs, and (b) be a non-feature (it is
  just "the documented cap now actually holds"). It would NOT materially improve the user's mental model —
  the contract statement already covers it. The docs describe the CONTRACT (output line-shape, "always
  fits", "every file larger than L is truncated to L"); they correctly do NOT describe HOW sections are
  split. Leave that discipline intact.

  ⚠️ **THE third design call — user-facing docs ONLY; the internal Mode-A godoc is already done.** The
  `splitDiffSections` godoc rewrite (internal/git/truncatediff.go ~59-73) was the Mode-A doc that rode
  WITH the fix in P1.M1.T1.S1 (Complete). This task touches ONLY README.md (~63), docs/how-it-works.md
  (~138-146), and docs/configuration.md (~107/131/146). Do NOT touch internal godocs, source code, the
  PRD, or the already-present `diff_context` range notes (those are from bugfix 001's Mode-A work).

  SCOPE: read + verify 3 user-facing doc surfaces; record a per-surface verdict; edit minimally IFF
  genuinely stale/misleading (expected: no edit). Respect markdownlint house style (`.markdownlint.json`).
  INPUT = the shipped behavior (FR3d contract restored) + the 3 docs as they stand. OUTPUT =
  changeset-level docs confirmed consistent with shipped behavior (most likely a no-op confirmation,
  recorded in the task verification). MOCKING: none (documentation editing only).
---

## Goal

**Feature Goal**: Confirm the three user-facing documentation surfaces that mention `token_limit` / the
FR3i water-fill truncation / the FR3d "payload always fits your context window" contract are consistent
with the now-restored shipped behavior (after the P1.M1.T1.S1 line-anchoring fix), editing ONLY where a
statement is genuinely stale or misleading — and making NO edit where the docs already describe the
intended contract (the expected case).

**Deliverable**:
1. **A per-surface verification record** (in the task verification / commit message): for each of
   README.md (~63 "Payload optimization"), docs/how-it-works.md (~138-146 "Size budget / water-fill"),
   docs/configuration.md (~107 comment / ~131 defaults table / ~146 "Token budget" prose), state the
   verdict — "accurate, no edit" (expected) or "edited: <one-line reason>".
2. **Conditionally**, a MINIMAL edit to any surface that is genuinely stale or misleading, passing
   markdownlint and leaking no internal-mechanism detail. (Expected: zero such edits.)

**Success Definition**: Each of the three surfaces is verified against the shipped behavior. In the
expected (no-op) case: `git diff --exit-code README.md docs/how-it-works.md docs/configuration.md` is
EMPTY and each surface's verdict is "accurate — the fix restores the documented contract". If an edit was
warranted: the edit is minimal, passes `markdownlint-cli2`, does not describe the split mechanism, and the
per-surface record explains why it was necessary. No source code, internal godoc, or PRD touched.

## User Persona

**Target User**: Maintainers + users reading the docs to understand `token_limit`'s guarantee. Transitively
the FR3d contract ("the payload always fits your model's context window without a per-model registry").

**Use Case**: A user reads docs/configuration.md ~146 ("the payload always fits your model's context
window") and trusts it when they set `token_limit` for a small-context model. The fix makes that trust
well-placed for ALL file content (including files that embed `diff --git` literals). The docs need no
change — they stated the contract correctly all along.

**User Journey**: (doc reading only) docs/configuration.md → `token_limit` prose → "always fits" → trust.
The fix ensures reality matches; the docs are confirmed consistent.

**Pain Points Addressed**: Closes the (latent, trigger-class-only) doc-vs-reality gap FROM THE
IMPLEMENTATION SIDE. No doc edit is needed because the docs described the intended behavior, not the bug.

## Why

- **Required by SOW §5 (Mode-B catch-all).** The final changeset-level doc sweep MUST exist and run last,
  even when the most likely outcome is "no edit needed" — the implementing agent CONFIRMS consistency
  rather than assuming it.
- **Prevents doc rot by verification, not by reflexive editing.** Editing accurate docs to "mention the
  fix" would leak internal mechanism detail and add a non-feature; verifying they already match is the
  correct discipline.
- **Zero risk.** A read-and-confirm sweep with a no-op default cannot regress code or behavior. Any
  conditional edit is minimal and markdownlint-gated.
- **Closes the loop on the FR3d contract.** The docs promised "always fits"; the bug broke it for one
  trigger class; the fix restores it; this task confirms the docs now describe reality.

## What

A documented verification of three user-facing doc surfaces against the now-restored shipped behavior,
with a strict no-op default. No source code, no tests, no internal godocs, no PRD. Conditional minimal
doc edits only if a statement is genuinely stale/misleading (expected: none).

### Success Criteria

- [ ] README.md ~63 ("Payload optimization" row) read and verdict recorded (expected: "accurate —
      'optionally capped to your model's context window via `token_limit`' is the FR3d contract, now
      restored; no mechanism referenced; no edit").
- [ ] docs/how-it-works.md ~138-146 ("Size budget / water-fill") read and verdict recorded (expected:
      "accurate — describes the water-fill contract + output line-shape ('next file's `diff --git` begins
      fresh'); does NOT specify the split mechanism or imply content-embedded `diff --git` is a problem;
      no edit").
- [ ] docs/configuration.md ~107 / ~131 / ~146 (`token_limit` comment, defaults row, "Token budget"
      prose) read and verdict recorded (expected: "accurate — 'always fits' is the restored contract; no
      edit"). The already-present `diff_context` range notes are NOT this task's concern.
- [ ] In the expected no-op case: `git diff --exit-code README.md docs/how-it-works.md docs/configuration.md`
      is EMPTY. (If an edit was warranted: it is minimal, passes markdownlint, leaks no mechanism detail,
      and the record explains it.)
- [ ] No edit describes the internal split mechanism or invents a feature; no source code / internal godoc
      / PRD touched; `.markdownlint.json` house style respected for any edit.

## All Needed Context

### Context Completeness Check

_Pass._ A technical writer with no prior knowledge can do this from: the shipped-behavior statement
below, the per-surface verdict table (the expected outcome), the strict edit criteria, and the markdownlint
config. No Go/git-internals knowledge required — this is a doc-accuracy verification against a one-line
behavioral contract.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/007_b33d310438c6/bugfix/002_430650446fcc/P1M2T1S1/research/doc_sweep_token_limit.md
  why: the per-surface audit (verbatim quotes + verdicts), the no-op rationale (docs describe the intended
       contract, which the fix restores — not new behavior), the markdownlint config, and the strict edit
       criteria.
  critical: the EXPECTED outcome is NO EDIT on all three surfaces. Do NOT add "files with `diff --git`
       content are handled correctly" — that is internal-mechanism leakage the contract forbids.

- docfile: plan/007_b33d310438c6/bugfix/002_430650446fcc/architecture/system_context.md
  section: "7. Documentation surface (per SOW §5)" (7.1 Mode A, 7.2 Mode B)
  why: §7.1 confirms "No user-facing / config / API surface change … the docs are not made accurate by the
       bug and do not need a Mode-A edit to describe new behavior" and that the internal `splitDiffSections`
       godoc was the Mode-A doc (rode with P1.M1.T1.S1, already done). §7.2 lists the three Mode-B surfaces
       and states "the most likely outcome is 'no edit needed' — the implementing agent confirms."
  critical: §7.2 names the exact surfaces (README.md:66, docs/how-it-works.md:144, docs/configuration.md:146)
       and the verdict for each ("verify it does not imply anything now-incorrect (it won't)").

- file: README.md
  section: ~63, the "Payload optimization" row of the Features table
  why: surface 1. Quote: "…optionally capped to your model's context window via `token_limit`…".
  pattern: it states the FR3d contract at a high level; references docs/how-it-works.md + docs/configuration.md.
  gotcha: do NOT add mechanism detail here — it is a one-row marketing surface.

- file: docs/how-it-works.md
  section: ~138-146, "Size budget (FR3d / FR3i)" — the Legacy-caps + Holistic-token-budget bullets
  why: surface 2. Describes the water-fill CONTRACT ("every file larger than L is truncated to L") and the
       OUTPUT line-shape ("the next file's `diff --git` begins fresh"). Does NOT specify the split mechanism.
  pattern: contract-level description; the right level of abstraction for a user-facing doc.
  gotcha: the phrase "the next file's `diff --git` begins fresh" describes OUTPUT shape (clean section
       boundaries), NOT the internal split — it is accurate and must not be "clarified" into mechanism detail.

- file: docs/configuration.md
  section: ~107 ([generation] comment block), ~131 (Built-in defaults table), ~146 ("Token budget & diff
           context" prose blockquote)
  why: surface 3. The ~146 prose is the FR3d "always fits" contract statement.
  pattern: config-key doc with a prose blockquote explaining the contract.
  gotcha: the `diff_context` "range 0–3 — out-of-range rejected at config load" notes at ~107/131/146 are
       ALREADY PRESENT (bugfix 001 Mode-A). Do NOT touch them — they are not this task's concern and are
       already accurate.

- file: .markdownlint.json
  why: house style — `{"default": true, "MD013": false, "MD033": false, "MD060": false}` (line-length off,
       inline-HTML off, pseudo-headings off; all else default). Any conditional edit MUST pass markdownlint.
  pattern: in the expected no-op case the docs are untouched and already lint clean.

- docfile: plan/007_b33d310438c6/bugfix/002_430650446fcc/P1M1T2S1/PRP.md
  why: confirms the parallel task is TEST-ONLY ("Test-only — no production/docs change") in
       internal/git/difftokenlimit_test.go → ZERO overlap with this doc sweep.
  critical: this task is the ONLY doc-editing task in the changeset; do not duplicate or conflict.
```

### Current Codebase tree (relevant slice)

```bash
README.md                       # ~63 "Payload optimization" row — VERIFY (expected: no edit)
docs/how-it-works.md            # ~138-146 "Size budget / water-fill" — VERIFY (expected: no edit)
docs/configuration.md           # ~107 / ~131 / ~146 token_limit surface — VERIFY (expected: no edit)
.markdownlint.json              # house style (any conditional edit must pass)
# NO source code, NO internal godocs (splitDiffSections godoc was Mode-A, done in P1.M1.T1.S1),
# NO PRD, NO tests touched.
```

### Desired Codebase tree with files to be added

```bash
# NO new files. AT MOST a minimal edit to one of the 3 doc files IF genuinely stale (expected: none).
# The primary deliverable is the VERIFICATION RECORD (per-surface verdict table) in the task verification.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL: the EXPECTED outcome is NO EDIT on all three surfaces. The docs describe the FR3d CONTRACT
     ("payload always fits"; "every file larger than L is truncated to L"; "the next file's diff --git
     begins fresh"), which the fix RESTORES. The docs were never inaccurate about new behavior — reality
     was inaccurate about the documented behavior. Do not edit accurate docs. -->

<!-- CRITICAL: do NOT add a note that "files containing literal `diff --git ` text are handled correctly".
     That is INTERNAL-MECHANISM detail (the splitDiffSections line-anchoring) forbidden in user-facing
     docs, AND it is a non-feature (just "the cap now holds"). The contract statement already covers it. -->

<!-- CRITICAL: the docs must NOT describe HOW the non-markdown aggregate is split into per-file sections.
     The current docs correctly describe the OUTPUT line-shape ("next file's diff --git begins fresh"),
     not the split mechanism. Preserve that abstraction level — do not "clarify" downward into mechanism. -->

<!-- CRITICAL: user-facing docs ONLY. The internal splitDiffSections godoc (internal/git/truncatediff.go
     ~59-73) was the Mode-A doc and rode WITH the fix in P1.M1.T1.S1 (Complete). Do NOT touch it, do NOT
     touch any source code, do NOT touch the PRD. -->

<!-- GOTCHA: the `diff_context` "range 0–3 — out-of-range rejected at config load" notes already present in
     docs/configuration.md (~107/131/146) are from bugfix 001's Mode-A work. They are accurate and NOT this
     task's concern — leave them. This task is scoped to the token_limit / truncation accuracy only. -->

<!-- GOTCHA (markdownlint): .markdownlint.json disables MD013 (line length), MD033 (inline HTML), MD060
     (pseudo-headings). Any conditional edit must pass `markdownlint-cli2 README.md docs/*.md`. The docs
     table rows (README.md) and blockquote prose (configuration.md) are valid under this config — keep any
     edit in the existing shape. -->
```

## Implementation Blueprint

### Data models and structure

N/A — documentation verification only. No types, no code. The "data" is the per-surface verdict table:

| Surface | Location | Statement checked | Expected verdict |
|---|---|---|---|
| README.md | ~63 "Payload optimization" row | "optionally capped to your model's context window via `token_limit`" | accurate — FR3d contract, now restored; no mechanism; **no edit** |
| docs/how-it-works.md | ~138-146 "Size budget" | water-fill "every file larger than L is truncated to L" + "next file's `diff --git` begins fresh" | accurate — contract + output shape; no mechanism; **no edit** |
| docs/configuration.md | ~107 comment / ~131 table / ~146 prose | `token_limit` "the payload always fits your model's context window" | accurate — restored contract; **no edit** (diff_context notes already present, not this task) |

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: READ + VERIFY README.md (~63, "Payload optimization" row)
  - READ the row. CONFIRM "optionally capped to your model's context window via `token_limit`" is the FR3d
    contract — now TRUE again after the fix. CONFIRM it references no section-splitting mechanism.
  - RECORD verdict: "accurate — restored contract; no edit" (expected).
  - EDIT ONLY IF: the row implied something now-incorrect (it does not). If it did (it won't), make a
    minimal, markdownlint-clean edit that leaks no mechanism detail.

Task 2: READ + VERIFY docs/how-it-works.md (~138-146, "Size budget (FR3d / FR3i)")
  - READ the Legacy-caps + Holistic-token-budget bullets. CONFIRM the water-fill contract ("every file
    larger than L is truncated to L"; "small files never penalized"; "nothing is wasted"; "the common case
    is left untouched") and the OUTPUT line-shape ("the next file's `diff --git` begins fresh") are now TRUE.
  - CONFIRM it does NOT specify the section-splitting mechanism and does NOT imply content-embedded
    `diff --git` literals are a problem.
  - RECORD verdict: "accurate — contract + output shape; no mechanism; no edit" (expected).
  - EDIT ONLY IF: a statement is genuinely stale/misleading (none expected). Do NOT add a content-embedded-
    diff --git note — that is forbidden mechanism leakage.

Task 3: READ + VERIFY docs/configuration.md (~107 / ~131 / ~146)
  - READ the [generation] `token_limit` comment (~107), the Built-in-defaults `token_limit` row (~131),
    and the "Token budget & diff context" blockquote prose (~146). CONFIRM "the payload always fits your
    model's context window" (the FR3d contract) is now TRUE.
  - NOTE the already-present `diff_context` "range 0–3 — out-of-range rejected" notes (bugfix 001 Mode-A)
    are accurate and OUT OF SCOPE — leave them.
  - RECORD verdict: "accurate — restored contract; no edit" (expected).
  - EDIT ONLY IF: a token_limit/truncation statement is genuinely stale (none expected).

Task 4: CONDITIONAL minimal edit (only if Task 1/2/3 found a genuinely stale statement)
  - IF (and only if) a surface met the strict edit criteria, make the SMALLEST edit that fixes the
    inaccuracy, WITHOUT describing the split mechanism or inventing a feature. Run markdownlint on the edit.
  - EXPECTED: this task is a no-op (no surface meets the bar).

Task 5: RECORD the verification + VERIFY
  - WRITE the per-surface verdict table into the task verification / commit message.
  - RUN `git diff --exit-code README.md docs/how-it-works.md docs/configuration.md` — in the expected case
    it is EMPTY (the no-op confirmation). If an edit was made, confirm it is minimal + markdownlint-clean.
  - RUN `markdownlint-cli2 README.md docs/how-it-works.md docs/configuration.md` (if available) — clean.
```

### Implementation Patterns & Key Details

```markdown
<!-- The verification discipline — default no-op, edit only on genuine staleness:
     1. QUOTE the exact statement from the doc.
     2. STATE the shipped behavior (FR3d contract restored: each real file is one section; payload fits).
     3. VERDICT: does the statement match? (Expected: YES — the docs describe the intended contract.)
     4. IF YES → "no edit". IF NO (genuinely stale/misleading) → minimal edit, no mechanism detail.

     The trap to AVOID: "the fix handled content-embedded diff --git, so let me mention that in the docs."
     That is mechanism leakage. The contract statement ("always fits"; "every file larger than L is
     truncated to L") ALREADY covers the user-visible guarantee. The HOW (line-anchored split) is internal. -->
```

### Integration Points

```yaml
DEPENDS ON (must be complete before this task runs):
  - P1.M1.T1.S1 (Complete): line-anchored splitDiffSections + its Mode-A godoc rewrite. The shipped
    behavior this task verifies against.
  - P1.M1.T2.S1 (parallel → completes first): E2E regression coverage pinning the fix. Test-only; no
    doc overlap.

FROZEN / NOT-EDITED:
  - All source code (internal/git/*, internal/config/*, …).
  - internal/git/truncatediff.go splitDiffSections godoc (Mode-A, done in P1.M1.T1.S1).
  - The already-present docs/configuration.md `diff_context` range notes (bugfix 001 Mode-A).
  - PRD.md, tasks.json, prd_snapshot.md (read-only).

NO DATABASE / NO ROUTES / NO CONFIG CODE / NO TESTS / NO NEW FILES.
```

## Validation Loop

> This is a documentation-verification task. The usual build/test gates do not apply; the gates are
> doc-consistency + markdownlint + the no-op (or minimal-edit) confirmation.

### Level 1: Doc Consistency (Immediate Feedback)

```bash
# Confirm the 3 surfaces exist and read them.
sed -n '60,68p' README.md                       # "Payload optimization" row
sed -n '136,148p' docs/how-it-works.md          # "Size budget" bullets
sed -n '104,148p' docs/configuration.md         # [generation] comment + defaults table + Token-budget prose
# Expected: each statement matches the shipped FR3d contract (restored). Record the per-surface verdict.
```

### Level 2: No-Op / Minimal-Edit Confirmation (Component Validation)

```bash
# THE primary gate. In the expected (no-op) case this is EMPTY — the confirmation:
git diff --exit-code README.md docs/how-it-works.md docs/configuration.md && echo "no doc edits needed (expected no-op)"
# If an edit was warranted (not expected), this is NON-empty — confirm it is minimal + targeted.
git diff --stat README.md docs/how-it-works.md docs/configuration.md
```

### Level 3: House Style (System Validation)

```bash
# markdownlint house style (.markdownlint.json: default true, MD013/MD033/MD060 off).
# In the no-op case the docs are untouched and already clean. If edited, re-lint:
npx markdownlint-cli2 README.md docs/how-it-works.md docs/configuration.md 2>/dev/null \
  || markdownlint-cli2 README.md docs/how-it-works.md docs/configuration.md 2>/dev/null \
  || echo "(markdownlint not installed locally; CI gate runs it — any edit must be markdownlint-clean by construction)"
# Expected: clean (no violations). If a violation appears on an edited file, fix the edit (not the config).
```

### Level 4: Mechanism-Leakage Audit (Domain-Specific Validation)

```bash
# CRITICAL audit: no user-facing doc should describe the internal split mechanism or invent a feature.
# After the (expected no-op) sweep, grep the 3 docs for mechanism leakage:
grep -nE 'splitDiffSections|split on|line-anchor|content.embedded|diff --git.*literal|handled correctly' \
  README.md docs/how-it-works.md docs/configuration.md
# Expected: NO matches (the docs describe the CONTRACT, not the mechanism). If a match appears in an edit,
# remove it — that is forbidden mechanism leakage. (A pre-existing benign mention of `diff --git` as an
# OUTPUT-shape description in how-it-works.md is FINE — it is not the split mechanism.)
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: all 3 surfaces read; per-surface verdict recorded.
- [ ] Level 2: `git diff --exit-code README.md docs/how-it-works.md docs/configuration.md` is EMPTY (no-op,
      the expected outcome) — OR a minimal edit is present and justified in the record.
- [ ] Level 3: markdownlint clean on any edited file (no-op case: docs untouched, already clean).
- [ ] Level 4: no mechanism-leakage / no invented feature in any edit.

### Feature Validation

- [ ] README.md ~63 verdict recorded (expected: accurate, no edit).
- [ ] docs/how-it-works.md ~138-146 verdict recorded (expected: accurate, no edit).
- [ ] docs/configuration.md ~107/131/146 verdict recorded (expected: accurate, no edit; diff_context notes
      left as-is).
- [ ] The FR3d contract statement ("payload always fits your model's context window") is confirmed TRUE
      against the shipped behavior in every surface that makes it.

### Code Quality Validation

- [ ] No edit describes the internal split mechanism (`splitDiffSections` / line-anchoring / "content with
      `diff --git` handled correctly") — that is forbidden in user-facing docs.
- [ ] Any conditional edit is minimal, in the existing doc shape, and markdownlint-clean.
- [ ] No scope creep into source code, internal godocs, the PRD, or the diff_context range notes.

### Documentation & Deployment

- [ ] The per-surface verification record is written (task verification / commit message).
- [ ] No new files; at most a minimal edit to one of the 3 existing doc files (expected: none).

---

## Anti-Patterns to Avoid

- ❌ Don't edit accurate docs to "mention the fix." The docs describe the FR3d CONTRACT, which the fix
  RESTORES — they were never inaccurate about new behavior. The default and expected outcome is NO EDIT.
- ❌ Don't add "files containing `diff --git` text in their content are handled correctly" (or any equivalent).
  That is INTERNAL-MECHANISM leakage the contract forbids in user-facing docs, AND it is a non-feature (just
  "the documented cap now holds"). The contract statement already covers the user-visible guarantee.
- ❌ Don't "clarify" docs/how-it-works.md's "the next file's `diff --git` begins fresh" into a description of
  the split mechanism. That phrase describes the OUTPUT line-shape (clean section boundaries) — the correct
  abstraction level for a user-facing doc. Leave it.
- ❌ Don't touch the internal `splitDiffSections` godoc (`internal/git/truncatediff.go` ~59-73). That was the
  Mode-A doc and rode WITH the fix in P1.M1.T1.S1 (Complete). This task is USER-FACING docs only.
- ❌ Don't touch the already-present `diff_context` "range 0–3 — out-of-range rejected" notes in
  docs/configuration.md. Those are bugfix 001's Mode-A work, already accurate, and out of scope.
- ❌ Don't touch source code, tests, the PRD, tasks.json, or prd_snapshot.md. This is a doc-verification task.
- ❌ Don't skip the verification just because the outcome is a no-op. SOW §5 requires the Mode-B sweep to
  exist and CONFIRM consistency — record the per-surface verdict even (especially) when no edit is made.
- ❌ Don't invent features or future behavior. The fix restores an existing contract; the docs reflect that
  contract. No new capability is documented.
- ❌ Don't violate markdownlint (`.markdownlint.json`: MD013/MD033/MD060 off, else default). Any conditional
  edit must pass; in the no-op case the docs are untouched.
- ❌ Don't conflate "the docs could mention X" with "the docs are stale." The edit bar is GENUINELY STALE OR
  MISLEADING — a mere opportunity to add detail is NOT a reason to edit (especially when the detail is
  internal mechanism).
