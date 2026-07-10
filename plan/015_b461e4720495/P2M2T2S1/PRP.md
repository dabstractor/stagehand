name: "P2.M2.T2.S1 — Update README + docs overview for freeze hardening (FR-M1e + amended FR-M1c) (§9.14, Mode B docs sync)"
description: |
  A DOCS-ONLY task (Mode B — the changeset-level documentation sync for Phase 2 freeze hardening).
  No Go source, no tests, no config, no PRD/tasks/snapshot edits. THREE surgical markdown edits
  across TWO files make the freeze-hardening work from P2.M1 + P2.M2.T1 read COHERENTLY:
  (1) docs/how-it-works.md — extend the "Freeze enforcement" bullet (L116, in "### Key design
  points", THE "freeze enforcement section" the item names) to frame the freeze boundary as
  two-layer defense-in-depth (entry re-assertion FR-M1e + per-step stager verification FR-M1c) AND
  to document the improved FR-M1c error quality (names the concept by title + plain-language cause
  + intentional-protection remedy from P2.M1.T3.S1, currently UNDOCUMENTED);
  (2) docs/how-it-works.md — add FR-M1e to the "Start-of-run freeze" bullet in "### Safety" (L132),
  which today cites only FR-M1c + FR-M1d, so the three-layer freeze surface (entry re-assertion,
  per-step stager verification, arbiter parity) is enumerated coherently;
  (3) README.md — add ONE concise sentence to the "Multi-commit decomposition" prose paragraph
  (L151) noting decompose re-asserts its empty-index precondition at entry as defense-in-depth
  (FR-M1e), so a stale trigger fails loudly rather than silently folding hand-staged content. The
  FR-M1e Trigger note P2.M1.T2.S1 added at docs/how-it-works.md:55 is SUFFICIENT and is KEPT AS-IS
  (cross-linked, not duplicated). Validates via grep (no automated markdown gate exists) +
  `make build`/`make test` (must be UNAFFECTED) + scope guard.

---

## Goal

**Feature Goal**: The freeze-hardening work shipped in P2.M1 (FR-M1e empty-index re-assertion +
amended FR-M1c improved error messages) and exercised by P2.M2.T1 (tests) is documented COHERENTLY
across the user-facing README and the architecture doc — the "freeze enforcement section" of
docs/how-it-works.md names BOTH the FR-M1e defense-in-depth re-check AND the improved FR-M1c error
quality as part of the reliability story, the Safety section enumerates all three freeze layers,
and the README's decompose section carries a concise defense-in-depth note.

**Deliverable**: Three surgical markdown edits across two files:
1. `docs/how-it-works.md` — rewrite/extend the "Freeze enforcement" bullet (L116) in
   "### Key design points" (the freeze-enforcement section).
2. `docs/how-it-works.md` — extend the "Start-of-run freeze" bullet (L132) in "### Safety" to add
   FR-M1e.
3. `README.md` — add one concise sentence to the "Multi-commit decomposition" prose paragraph (L151).
No new files. No Go/test/config/PRD/tasks edits.

**Success Definition**:
- The "Freeze enforcement" bullet (docs/how-it-works.md L116) is re-titled "Freeze enforcement
  (defense-in-depth)", states the freeze boundary is guarded at two layers (FR-M1e entry
  re-assertion, cross-linked to the Trigger note at L55, + FR-M1c per-step subset verification),
  retains the existing hard-abort sentence, and ADDS a clause documenting the improved error
  quality (names the concept by title + plain-language cause + that the abort is intentional
  freeze-boundary protection).
- The "Start-of-run freeze" Safety bullet (L132) enumerates three layers — entry re-assertion
  (FR-M1e), per-step stager verification (FR-M1c), arbiter parity (FR-M1d).
- README's "Multi-commit decomposition" paragraph (L151) carries one concise sentence: decompose
  re-asserts its empty-index precondition at entry as defense-in-depth (FR-M1e), so a stale
  trigger fails loudly rather than silently folding hand-staged content.
- `make build` + `make test` PASS UNCHANGED (docs-only changes cannot break them, but confirm).
- `git status --porcelain` shows ONLY `README.md` and `docs/how-it-works.md`.
- grep confirms the new FR-M1e / defense-in-depth / error-quality content is present in the right
  sections.

## User Persona (if applicable)

**Target User**: A Stagecoach end user (or maintainer) reading the docs to understand whether a
concurrent editor/agent change during a decompose run can corrupt their commits, and what happens
if the trigger ever mis-routes a hand-staged index into decompose.
**Use Case**: Trust the decompose freeze. The user wants the reliability story in one coherent
place: the run commits only what existed when it began, the freeze is enforced at multiple layers
(entry + per-step + arbiter), and if a violation ever fires the error tells them what happened and
that the abort was intentional protection.
**Pain Points Addressed**: The improved FR-M1c error quality (concept by title, plain-language
cause) is currently UNDOCUMENTED (P2.M1.T3.S1 shipped it as code-only — item DOCS: "none"). The
freeze-enforcement bullet covers only the per-step check, not the entry re-assertion, so a reader
checking "is the freeze boundary re-asserted defense-in-depth?" finds no answer in the enforcement
section.

## Why

- **Close the Phase 2 docs milestone (Mode B):** P2.M1 shipped FR-M1e + amended FR-M1c as code;
  P2.M2.T1 added tests. P2.M2.T2 is the dedicated docs-sync milestone; this subtask IS that sync.
- **Document the improved error quality (the genuine gap):** P2.M1.T3.S1 deliberately made NO
  docs-surface change (its item said DOCS: "none — error message improvement"). The improved
  FR-M1c errors (concept-by-title + "frozen working-tree snapshot" phrasing + plain-language
  remedy) are therefore UNDOCUMENTED. This task closes that gap in the reliability story.
- **Coherence, not duplication:** the FR-M1e note already exists at docs/how-it-works.md:55
  (P2.M1.T2.S1 Mode A). The item says "verify and extend if needed" — it IS sufficient for the
  *mention*, but the "freeze enforcement section" (the L116 bullet) must cross-reference it and
  add the error-quality story so a reader gets the full picture in one place. No rewording of L55.
- **Concise README note:** the item is explicit — "Keep it concise — this is an internal hardening,
  not a user-facing feature." One sentence in the decompose section, not a new feature page.

## What

Three markdown edits. No code.

### Success Criteria
- [ ] docs/how-it-works.md L116 "Freeze enforcement" bullet is re-titled "Freeze enforcement
      (defense-in-depth)" and covers BOTH FR-M1e (entry re-assertion, cross-linked to `#trigger`)
      AND the improved FR-M1c error quality (concept by title + plain-language cause + intentional
      protection), while retaining the existing per-step verification + hard-abort + FR-M12 content.
- [ ] docs/how-it-works.md L132 "Start-of-run freeze" Safety bullet enumerates three freeze layers
      with FR citations: entry re-assertion (FR-M1e), per-step stager verification (FR-M1c), arbiter
      parity (FR-M1d).
- [ ] README.md L151 "Multi-commit decomposition" paragraph carries ONE concise sentence on the
      FR-M1e defense-in-depth re-assertion (stale trigger fails loudly, not silent fold-in).
- [ ] docs/how-it-works.md L55 (the FR-M1e Trigger note) is UNCHANGED (cross-linked, not duplicated).
- [ ] README.md L64 Features table row and L189 FAQ "Will it corrupt my repo?" are UNCHANGED.
- [ ] `make build` and `make test` PASS (docs-only; sanity confirmation).
- [ ] `git status --porcelain` shows ONLY `README.md` + `docs/how-it-works.md`.
- [ ] grep confirms new content present (see Validation Loop).

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact three edit locations with verbatim oldText strings, the exact
new wording for each, the markdown style in force (`.markdownlint.json`: MD013/033/060 OFF → long
paragraphs + bold-lead-in bullets are fine), the fact that NO automated markdown gate exists (so
validation is grep + build/test sanity + scope guard), the full provenance of what's being
documented (FR-M1e from P2.M1.T2.S1, improved errors from P2.M1.T3.S1), and the explicit
scope-don't-touch list.

### Documentation & References

```yaml
# MUST READ — the complete verified analysis for THIS item (current state, gaps, exact oldText,
# the no-automated-gate finding, scope boundaries).
- docfile: plan/015_b461e4720495/P2M2T2S1/research/findings.md
  why: "§2 what's being documented (P2.M1 + P2.M2.T1 provenance); §3 current state of BOTH doc
        files with the THREE freeze-story locations; §4 markdown style + the no-automated-gate
        finding; §5 the EXACT verbatim oldText strings for all 3 edits; §6 what NOT to touch; §7
        the validation commands."

# EDIT TARGET 1 + 2 — the architecture doc.
- file: docs/how-it-works.md
  why: "L116 the 'Freeze enforcement' bullet (in '### Key design points') — THE freeze-enforcement
        section to extend (Edit 1). L132 the 'Start-of-run freeze' bullet (in '### Safety') — add
        FR-M1e (Edit 2). L55 the FR-M1e Trigger note (P2.M1.T2.S1 Mode A) — KEEP, cross-link target
        (GitHub anchor '### Trigger' → '#trigger'). L113 the 'Start-of-run freeze (T_start)' capture
        bullet — KEEP (it is the capture explanation, not enforcement)."
  pattern: "Bold-lead-in bullet paragraphs ('**Term.** body...'). Long paragraphs are fine
            (MD013 OFF). The '### Key design points' bullets and '### Safety' bullets both use
            this style — match it."
  gotcha: "The '### Key design points' 'Freeze enforcement' bullet (L116) and the '### Safety'
           'Start-of-run freeze' bullet (L132) are DIFFERENT bullets in DIFFERENT sections — edit
           each independently with its own oldText. Do not conflate them."

# EDIT TARGET 3 — the README.
- file: README.md
  why: "L151 the '### Multi-commit decomposition' prose paragraph (under '## Quick start') — insert
        one concise FR-M1e sentence after the arbiter clause '(a concurrent edit can never sneak
        into a commit).' and before 'The planner partitions changes per file.' (Edit 3)."
  pattern: "Prose paragraph (not a bullet). Em-dashes (—) and backtick code spans (`git reset`,
            `--single`) are used throughout — match the surrounding style."
  gotcha: "Do NOT touch L64 (the Features table row 'Multi-commit decomposition') or L189 (the FAQ
           'Will it corrupt my repo?'). The table cell is too dense for a defense-in-depth clause;
           the FAQ is snapshot+lock+orphan, not decompose-internal freeze. L151 is the right home."

# CONTRACT — the P2.M1.T3.S1 work whose error quality is being documented (the genuine gap).
- docfile: plan/015_b461e4720495/P2M1T3S1/PRP.md
  why: "Defines the improved verifyFreezeSubset messages (the thing Edit 1 documents): concept named
        BY TITLE via conceptTitle (%q), 'frozen working-tree snapshot' phrasing (replacing 'not
        traceable to T_start'), and the remedy suffix 'This indicates concurrent working-tree
        changes were picked up by the stager. Aborting to protect the freeze boundary.' That PRP
        shipped code-only (item DOCS: 'none'); this task documents it. Treat it as implemented."

# CONTRACT — the P2.M1.T2.S1 work (FR-M1e) whose Trigger note (L55) is the cross-link target.
- docfile: plan/015_b461e4720495/P2M1T2S1/PRP.md
  why: "Defines FR-M1e (Decompose re-asserts the empty-index precondition at entry, after the
        escape-hatch, before FreezeWorkingTree; fails loudly naming staged paths + git reset /
        stagecoach --single remedies) and its Mode A doc note at docs/how-it-works.md:55. Edit 1
        cross-links to that note (anchor '#trigger'); Edit 2 cites FR-M1e alongside FR-M1c/FR-M1d."

# PARALLEL CONTEXT — P2.M2.T1.S1 (in-flight, TESTS-ONLY). NO conflict with this docs task.
- docfile: plan/015_b461e4720495/P2M2T1S1/PRP.md
  why: "Confirms the parallel sibling is TESTS-ONLY (internal/decompose/*_test.go +
        internal/e2e/scenarios_test.go) with NO docs change. Its edits do not touch README.md or
        docs/how-it-works.md, so there is NO merge/scope conflict with this task. Reference only."

# PRD provenance (read-only).
- docfile: plan/015_b461e4720495/prd_snapshot.md
  section: "§9.14 FR-M1c (freeze enforcement, defense-in-depth) + FR-M1e (the empty-index
            re-assertion); §13.6.1 (the trigger model); §13.6.7 (the one-paragraph safety proof)."
  why: "Establishes the three-layer freeze surface (FR-M1e entry / FR-M1c per-step / FR-M1d
        arbiter) the docs must present coherently."

# Markdown style in force (read-only).
- file: .markdownlint.json
  why: "Confirms MD013 (line length) OFF, MD033 (inline HTML) OFF, MD060 (no-sibling-headings) OFF,
        default=true otherwise. Long bold-lead-in bullet paragraphs (the existing style) are
        compliant. No automated gate runs this file (not on PATH, not in Makefile/CI — see §4 of
        findings)."
```

### Current Codebase tree (relevant slice)

```bash
# EDIT targets (DOCS only):
docs/how-it-works.md   # EDIT — L116 "Freeze enforcement" bullet (Edit 1) + L132 "Start-of-run freeze" Safety bullet (Edit 2)
README.md              # EDIT — L151 "Multi-commit decomposition" prose paragraph (Edit 3)

# READ-ONLY references:
plan/015_b461e4720495/P2M2T2S1/research/findings.md   # the full verified analysis (exact oldText strings)
plan/015_b461e4720495/P2M1T3S1/PRP.md                  # CONTRACT: the improved FR-M1c error messages (the gap)
plan/015_b461e4720495/P2M1T2S1/PRP.md                  # CONTRACT: FR-M1e + the L55 Trigger note (cross-link target)
plan/015_b461e4720495/P2M2T1S1/PRP.md                  # parallel sibling (TESTS-ONLY; no conflict)
.markdownlint.json                                     # style rules (MD013/033/060 OFF)
Makefile                                               # `make build` / `make test` / `make lint` (Go-only)
```

### Desired Codebase tree with files to be edited

```bash
docs/how-it-works.md   # EDIT — 2 bullets rewritten/extended (L116, L132)
README.md              # EDIT — 1 sentence added to L151 paragraph
# (no new files; no code/test/config/PRD/tasks/snapshot changes)
```

### Known Gotchas of our codebase & Library Quirks

```text
# CRITICAL (two DIFFERENT "Start-of-run freeze" bullets): docs/how-it-works.md has a
#   "Start-of-run freeze (T_start)." bullet at L113 (in "### Key design points" — the CAPTURE
#   explanation) AND a "- **Start-of-run freeze** —" bullet at L132 (in "### Safety" — the
#   enforcement/layers summary). Edit 2 targets the L132 SAFETY bullet, NOT the L113 capture
#   bullet. The L113 capture bullet is UNCHANGED. Match the exact oldText.

# CRITICAL (the "Freeze enforcement" bullet L116 is THE task's "freeze enforcement section"):
#   the item literally says "ensure the freeze enforcement section covers FR-M1e ... and the
#   improved FR-M1c error messages." That section = the "### Key design points" "Freeze
#   enforcement." bullet at L116. It currently covers ONLY the per-step subset verification
#   (FR-M1c core). Extend it — do NOT just append a stray sentence elsewhere.

# GOTCHA (NO automated markdown gate): markdownlint is NOT on PATH, NOT in the Makefile
#   (`make lint` = golangci-lint, Go-only), and NOT in any CI workflow. The .markdownlint.json
#   config exists but is never enforced automatically. So validation = grep (content present) +
#   structural sanity (bullets/paragraphs well-formed) + `make build`/`make test` UNAFFECTED
#   (docs-only). Do not invent a `make docs-lint` step.

# GOTCHA (GitHub anchor for the cross-link): "### Trigger" renders to the anchor "#trigger"
#   (lowercase, spaces→hyphens, punctuation stripped). Edit 1's cross-link is
#   "[Trigger](#trigger)". Verify with the existing in-doc anchors if unsure (e.g. the README
#   links docs/how-it-works.md#multi-commit-decomposition — same algorithm).

# GOTCHA (em-dash — is used throughout both docs): both the oldText and newText use "—" (U+2014),
#   not "--" or "-". When copying oldText for the edit, preserve the exact em-dash bytes or the
#   match will fail.

# GOTCHA (FR-M1e Trigger note L55 is SUFFICIENT — do NOT reword it): the item says "The Mode A
#   note from P2.M1.T2.S1 may be sufficient — verify and extend if needed." It IS sufficient for
#   the FR-M1e mention. This task CROSS-LINKS to it (Edit 1 → "#trigger") and CITES its FR
#   (Edit 2) — it does NOT duplicate or reword L55. Rewording L55 risks conflicting with the
#   detailed entry-point behavior + remedies P2.M1.T2.S1 carefully wrote.

# GOTCHA (P2.M2.T1.S1 runs in PARALLEL but is TESTS-ONLY): the sibling subtask edits
#   internal/decompose/*_test.go and internal/e2e/scenarios_test.go — NO docs files. There is
#   ZERO scope/merge conflict with this task. Do not coordinate around it; the two touch
#   disjoint files.

# GOTCHA (do NOT add a per-step/error-quality clause to the README): the item's (b) asks for a
#   "brief note that decompose re-asserts its preconditions as defense-in-depth" — singular,
#   focused on FR-M1e. The deeper freeze-enforcement + improved-error story belongs in
#   docs/how-it-works.md (Edit 1), NOT the README. Adding it to README would violate "Keep it
#   concise — this is an internal hardening, not a user-facing feature."
```

## Implementation Blueprint

### Data models and structure
None — no code/data. The "data" is three markdown paragraph/bullet rewrites.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1 (Edit 1 — the core): EDIT docs/how-it-works.md — extend the "Freeze enforcement" bullet (L116)
  - LOCATE the bullet in "### Key design points" (immediately after the L113 "Start-of-run freeze
    (T_start)." bullet). Its verbatim current text (match EXACTLY, incl. em-dashes + backticks):
        **Freeze enforcement.** Because the stager is an external agent running `git` against the live tree, after each staging step stagecoach verifies the resulting tree is a content-subset of T_start (only T_start paths, T_start content). Any deviation — a concurrent change swept in, or a stager that ran a bare `git add -A` — is a hard abort (non-rescue; already-landed commits stand per FR-M12).
  - REPLACE with (re-title to "Freeze enforcement (defense-in-depth)."; prepend the two-layer
    framing + FR-M1e entry re-assertion cross-linked to #trigger; KEEP the per-step verification +
    hard-abort + FR-M12 sentence; APPEND the improved-error-quality clause):
        **Freeze enforcement (defense-in-depth).** The freeze boundary is guarded at two layers, neither trusting its caller nor the external stager. At entry, decompose re-asserts its empty-index precondition (FR-M1e) — a stale or buggy trigger that routes here with a non-empty index fails loudly, naming the offending staged paths, instead of silently folding them into T_start (see [Trigger](#trigger)). Then, because the stager is an external agent running `git` against the live tree, after each staging step stagecoach verifies the resulting tree is a content-subset of T_start (only T_start paths, T_start content). Any deviation — a concurrent change swept in, or a stager that ran a bare `git add -A` — is a hard abort (non-rescue; already-landed commits stand per FR-M12), with an error that names the concept by title and explains the cause in plain language (concurrent working-tree changes were picked up by the stager; the abort is intentional freeze-boundary protection), so the user knows the run was protected and can re-run from a clean tree.
  - WHY this wording: (a) "two layers ... neither trusting its caller nor the external stager" is
    the defense-in-depth framing the item asks for; (b) the FR-M1e clause + "#trigger" cross-link
    satisfies "covers FR-M1e (defense-in-depth re-check)" without duplicating L55; (c) the
    appended error-quality clause satisfies "the improved FR-M1c error messages as part of the
    reliability story" — it documents the concept-by-title + plain-language-cause + intentional-
    protection that P2.M1.T3.S1 shipped (currently undocumented); (d) the existing per-step +
    hard-abort + FR-M12 content is preserved verbatim mid-paragraph.
  - PRESERVE: the L113 "Start-of-run freeze (T_start)." capture bullet ABOVE it UNCHANGED.

Task 2 (Edit 2 — coherence in the Safety section): EDIT docs/how-it-works.md — extend the
"Start-of-run freeze" SAFETY bullet (L132) to enumerate FR-M1e
  - LOCATE the bullet in "### Safety" (the "- **Start-of-run freeze** —" bullet; NOT the L113
    "### Key design points" capture bullet). Verbatim current text:
        - **Start-of-run freeze** — T_start captures the full working-tree change set at decompose activation; concurrent edits never enter any commit. The stager is verified as a content-subset of T_start after each staging step (FR-M1c), and the arbiter — the third freeze surface — derives its gate, its diff, and every tree it commits strictly from T_start and tipTree, never a live re-read (FR-M1d).
  - REPLACE with (insert the "three layers" framing so FR-M1e is enumerated alongside FR-M1c/M1d):
        - **Start-of-run freeze** — T_start captures the full working-tree change set at decompose activation; concurrent edits never enter any commit. The freeze boundary is held at three layers: the empty-index precondition is re-asserted at entry (FR-M1e); the stager is verified as a content-subset of T_start after each staging step (FR-M1c); and the arbiter — the third freeze surface — derives its gate, its diff, and every tree it commits strictly from T_start and tipTree, never a live re-read (FR-M1d).
  - WHY: after this changeset the Safety bullet would otherwise cite FR-M1c + FR-M1d but omit
    FR-M1e — an incoherence. The "three layers" framing (entry re-assertion / per-step stager /
    arbiter parity) matches the PRD's three-surface model (FR-M1e/M1c/M1d) and the "coherently"
    goal in the OUTPUT spec. Minimal, surgical.
  - PRESERVE: the rest of the "### Safety" bullets (Atomic, Frozen content, No index resets) and
    the trailing per-role-timeout paragraph UNCHANGED.

Task 3 (Edit 3 — the README note): EDIT README.md — add ONE concise FR-M1e sentence to the
"Multi-commit decomposition" paragraph (L151)
  - LOCATE the sentence boundary in the paragraph under "### Multi-commit decomposition" (under
    "## Quick start"). The clause to split AFTER is "(a concurrent edit can never sneak into a
    commit)." and the sentence to keep AFTER is "The planner partitions changes per file and leans
    toward a soft count target...". Insert the new sentence BETWEEN them.
  - FIND (the exact text spanning the insertion point):
        and that holds across the leftover-reconciliation arbiter too (a concurrent edit can never sneak into a commit). The planner partitions changes per file and leans toward a soft count target,
  - REPLACE with (insert one defense-in-depth sentence between the two):
        and that holds across the leftover-reconciliation arbiter too (a concurrent edit can never sneak into a commit). As defense-in-depth, decompose also re-asserts its empty-index precondition at entry (FR-M1e), so a stale trigger that reaches it with a staged index fails loudly rather than silently folding that hand-staged content into the run. The planner partitions changes per file and leans toward a soft count target,
  - WHY: satisfies item (b) — "add a brief note that decompose re-asserts its preconditions as
    defense-in-depth." ONE sentence, focused on FR-M1e (the item's specific ask), concise (the
    deeper per-step verification + improved-error story lives in docs/how-it-works.md via Edit 1,
    linked from this section's existing "See [How Stagecoach works — Multi-commit decomposition]"
    footer). Placed in the most contextual reliability paragraph, not the dense Features table cell
    (L64) or the corruption-focused FAQ (L189).
  - PRESERVE: the L64 Features table row and L189 FAQ UNCHANGED (do not bloat a table cell or a
    snapshot+lock answer with decompose-internal freeze detail).

Task 4: VALIDATE — grep content checks + build/test sanity + scope guard
  - grep -n "FR-M1e" docs/how-it-works.md          # expect ≥3 hits (L55 + Edit1 + Edit2)
  - grep -n "defense-in-depth\|Defense-in-depth" docs/how-it-works.md README.md  # Edit1 title/body + Edit3
  - grep -n "concept by title\|names the concept" docs/how-it-works.md   # Edit1 error-quality clause
  - grep -n "three layers\|at three layers" docs/how-it-works.md         # Edit2
  - grep -n "FR-M1e\|defense-in-depth" README.md                          # Edit3 (≥1 hit)
  - make build && make test            # docs-only; MUST still pass (sanity)
  - git status --porcelain             # ONLY README.md + docs/how-it-works.md
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN (Edit 1 — the "freeze enforcement section" becomes the two-layer defense-in-depth +
     improved-error reliability story; bold-lead-in bullet, long paragraph OK per MD013 OFF) -->
**Freeze enforcement (defense-in-depth).** The freeze boundary is guarded at two layers, neither
trusting its caller nor the external stager. At entry, decompose re-asserts its empty-index
precondition (FR-M1e) — a stale or buggy trigger that routes here with a non-empty index fails
loudly, naming the offending staged paths, instead of silently folding them into T_start (see
[Trigger](#trigger)). Then, because the stager is an external agent running `git` against the live
tree, after each staging step stagecoach verifies the resulting tree is a content-subset of T_start
(only T_start paths, T_start content). Any deviation — a concurrent change swept in, or a stager
that ran a bare `git add -A` — is a hard abort (non-rescue; already-landed commits stand per
FR-M12), with an error that names the concept by title and explains the cause in plain language
(concurrent working-tree changes were picked up by the stager; the abort is intentional
freeze-boundary protection), so the user knows the run was protected and can re-run from a clean tree.

<!-- PATTERN (Edit 2 — the Safety bullet enumerates all three freeze layers coherently) -->
- **Start-of-run freeze** — T_start captures the full working-tree change set at decompose
  activation; concurrent edits never enter any commit. The freeze boundary is held at three layers:
  the empty-index precondition is re-asserted at entry (FR-M1e); the stager is verified as a
  content-subset of T_start after each staging step (FR-M1c); and the arbiter — the third freeze
  surface — derives its gate, its diff, and every tree it commits strictly from T_start and
  tipTree, never a live re-read (FR-M1d).

<!-- PATTERN (Edit 3 — README: ONE concise FR-M1e defense-in-depth sentence, prose not bullet) -->
... and that holds across the leftover-reconciliation arbiter too (a concurrent edit can never
sneak into a commit). As defense-in-depth, decompose also re-asserts its empty-index precondition
at entry (FR-M1e), so a stale trigger that reaches it with a staged index fails loudly rather than
silently folding that hand-staged content into the run. The planner partitions changes per file ...
```

### Integration Points

```yaml
DOCS (docs/how-it-works.md):
  - EDIT (Task 1): the "Freeze enforcement." bullet → "Freeze enforcement (defense-in-depth)." in
    "### Key design points" (L116). Adds FR-M1e cross-link + improved-error clause.
  - EDIT (Task 2): the "- **Start-of-run freeze** —" bullet in "### Safety" (L132). Adds the
    three-layer framing with FR-M1e/M1c/M1d.
README (README.md):
  - EDIT (Task 3): one sentence inserted in the "### Multi-commit decomposition" prose paragraph
    (L151), between the arbiter clause and "The planner partitions changes per file."
CROSS-LINKS:
  - Edit 1 links to "#trigger" (the existing FR-M1e note at docs/how-it-works.md:55). The README's
    existing "See [How Stagecoach works — Multi-commit decomposition]" footer (already in L151's
    section) routes depth-seekers to the Edit 1 content — no new README→docs link needed.
NO code / tests / config / migrations / env vars / new files / PRD / tasks.json / prd_snapshot /
  delta_prd edits. NO new doc files.
```

## Validation Loop

### Level 1: Content presence (grep — the primary gate, since no automated markdown gate exists)

```bash
cd /home/dustin/projects/stagecoach
# Edit 1 landed in the freeze-enforcement section:
grep -n "Freeze enforcement (defense-in-depth)" docs/how-it-works.md   # 1 hit (the re-titled bullet)
grep -n "concept by title" docs/how-it-works.md                        # 1 hit (the error-quality clause)
grep -n 'see \[Trigger\](#trigger)' docs/how-it-works.md               # 1 hit (the cross-link)
# Edit 2 landed in the Safety section:
grep -n "held at three layers" docs/how-it-works.md                    # 1 hit
# Edit 3 landed in the README:
grep -n "As defense-in-depth, decompose also re-asserts" README.md     # 1 hit
# FR-M1e is now referenced in BOTH files, multiple places:
grep -cn "FR-M1e" docs/how-it-works.md   # ≥3 (L55 Trigger + Edit1 + Edit2)
grep -cn "FR-M1e" README.md              # ≥1 (Edit3)
# Expected: every grep above prints ≥1 line. Any zero-hit grep = an edit did not land; re-check.
```

### Level 2: Markdown structural sanity (no broken headings / bullets)

```bash
cd /home/dustin/projects/stagecoach
# The "### Key design points" and "### Safety" sections still exist and their bullet counts are sane.
grep -n "^### " docs/how-it-works.md | grep -E "Key design points|Safety|Trigger"   # 3 section headers intact
# Bullet count under Safety unchanged except the one rewritten bullet (still a single "- **...**" line):
grep -c "^- \*\*Start-of-run" docs/how-it-works.md   # 2 (L113 capture bullet + L132 safety bullet — both intact)
# README heading + paragraph structure intact:
grep -n "^### Multi-commit decomposition" README.md   # 1 hit (heading unchanged)
# Expected: all greps hit. No heading was accidentally merged/broken.
```

### Level 3: Build/test sanity (docs-only — MUST be unaffected)

```bash
cd /home/dustin/projects/stagecoach
make build   # compiles the binary; docs changes cannot affect it
# Expected: success (produces ./bin/stagecoach).

make test    # the Go test suite; docs changes cannot affect it
# Expected: PASS (identical to pre-edit). If ANY test fails, it is NOT caused by this task (docs-only)
#           — but STOP and investigate before claiming done, since a coincidental break must be understood.
```

### Level 4: Scope guard (ONLY the two doc files changed)

```bash
cd /home/dustin/projects/stagecoach
git status --porcelain
# Expected: EXACTLY two lines:
#   M README.md
#   M docs/how-it-works.md

# Guard: no out-of-scope / forbidden files touched.
git status --porcelain | grep -vE '^ M (README\.md|docs/how-it-works\.md)$' | grep -E '\.(go|md|toml|json)$|PRD\.md|tasks\.json|prd_snapshot|delta_prd' && echo "FAIL: out-of-scope file touched" || echo "OK: scope clean"
# Expected: "OK: scope clean". If anything other than README.md + docs/how-it-works.md is listed,
#           STOP — this is a docs-only task.
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1 greps all hit (Edit 1/2/3 content present; FR-M1e in both files)
- [ ] Level 2 structural greps hit (headings/bullets intact)
- [ ] `make build` succeeds (docs-only; sanity)
- [ ] `make test` passes (docs-only; sanity — identical to pre-edit)
- [ ] Level 4 scope guard prints "OK: scope clean" (ONLY README.md + docs/how-it-works.md)

### Feature Validation
- [ ] docs/how-it-works.md L116 "Freeze enforcement (defense-in-depth)" bullet covers BOTH FR-M1e
      (entry re-assertion, cross-linked to `#trigger`) AND the improved FR-M1c error quality
      (concept by title + plain-language cause + intentional protection)
- [ ] docs/how-it-works.md L132 "Start-of-run freeze" Safety bullet enumerates three layers
      (FR-M1e entry / FR-M1c per-step / FR-M1d arbiter)
- [ ] README.md L151 carries ONE concise FR-M1e defense-in-depth sentence
- [ ] The freeze-hardening story reads COHERENTLY end-to-end (Trigger L55 → Key design points L116 →
      Safety L132; README L151 → links to docs)

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY `README.md` + `docs/how-it-works.md`
- [ ] docs/how-it-works.md L55 (the FR-M1e Trigger note from P2.M1.T2.S1) UNCHANGED
- [ ] docs/how-it-works.md L113 "Start-of-run freeze (T_start)." capture bullet UNCHANGED
- [ ] README.md L64 Features table row UNCHANGED
- [ ] README.md L189 FAQ "Will it corrupt my repo?" UNCHANGED
- [ ] docs/cli.md / configuration.md / providers.md / docs/README.md UNCHANGED
- [ ] NO Go source, test, config, PRD.md, tasks.json, prd_snapshot.md, or delta_prd.md edit

### Documentation Quality
- [ ] Follows existing markdown style (bold-lead-in bullets, em-dashes, backtick code spans)
- [ ] No duplication of the L55 Trigger note (cross-linked, not reworded)
- [ ] Concise per the item ("Keep it concise — this is an internal hardening, not a user-facing
      feature") — README gets ONE sentence; depth lives in docs/how-it-works.md
- [ ] Cross-link `#trigger` uses the correct GitHub anchor algorithm

---

## Anti-Patterns to Avoid

- ❌ Don't reword the FR-M1e Trigger note at docs/how-it-works.md:55 — the item says it "may be
  sufficient — verify and extend if needed." It IS sufficient. Cross-LINK to it (Edit 1) and CITE
  its FR (Edit 2); do not duplicate or rewrite it (risks conflicting with P2.M1.T2.S1's carefully
  worded entry-point behavior + remedies).
- ❌ Don't conflate the two "Start-of-run freeze" bullets — L113 (in "### Key design points", the
  CAPTURE explanation, KEEP) and L132 (in "### Safety", the layers summary, EDIT via Task 2). They
  are different bullets in different sections. Match each oldText exactly.
- ❌ Don't add the per-step verification / improved-error story to the README — the item's (b) asks
  for a BRIEF note that decompose "re-asserts its preconditions as defense-in-depth" (singular,
  FR-M1e). The depth belongs in docs/how-it-works.md (Edit 1). Bloating the README violates "Keep
  it concise — this is an internal hardening, not a user-facing feature."
- ❌ Don't touch the README Features table row (L64) or the FAQ "Will it corrupt my repo?" (L189) —
  the table cell is too dense for a defense-in-depth clause, and the FAQ is snapshot+lock+orphan,
  not decompose-internal freeze. L151's prose paragraph is the right (and only) README home.
- ❌ Don't invent a `make docs-lint` / markdownlint step — markdownlint is NOT on PATH, NOT in the
  Makefile, and NOT in CI. The `.markdownlint.json` config exists but is never auto-enforced.
  Validation is grep + build/test sanity + scope guard.
- ❌ Don't edit any Go/test/config/PRD/tasks/snapshot file — this is a DOCS-ONLY (Mode B) task. If
  `git status` shows anything other than README.md + docs/how-it-works.md, you are out of scope.
- ❌ Don't drop or weaken the existing content in the L116 bullet — the per-step subset
  verification, the "hard abort (non-rescue; already-landed commits stand per FR-M12)" sentence,
  and the "concurrent change swept in, or a stager that ran a bare `git add -A`" examples must all
  be PRESERVED. Edit 1 EXTENDS the bullet, it does not replace its substance.
- ❌ Don't coordinate around P2.M2.T1.S1 — it runs in parallel but is TESTS-ONLY (touches
  internal/decompose/*_test.go + internal/e2e/scenarios_test.go). Zero file overlap with this task.

---

## Confidence Score

**One-pass success likelihood: 9/10.** The deliverable is three surgical, well-specified markdown
edits against verified oldText strings in two files I read in full. The wording for each edit is
given verbatim in the Implementation Tasks. The two things that keep it from 10/10: (1) the em-dash
bytes in oldText must be copied exactly (U+2014, not "--") or the text match fails — called out
explicitly; (2) the two "Start-of-run freeze" bullets (L113 vs L132) are easy to conflate — Task 2
pins the L132 SAFETY bullet with its exact oldText. There is no automated markdown gate, so the
"does it render/look right" judgment is human (the structural greps in Level 2 catch gross breakage).
The parallel sibling (P2.M2.T1.S1) touches disjoint files, so there is no merge/scope conflict.
