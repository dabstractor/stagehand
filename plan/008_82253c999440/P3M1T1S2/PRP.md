---
name: "P3.M1.T1.S2 — docs/how-it-works.md: holistic decompose-section reconciliation + verify cli.md/configuration.md untouched (Mode B docs sync; PRD §9.14 FR-M1d, §13.6.5)"
description: |

  A MODE B DOCS-ONLY edit to ONE file: `docs/how-it-works.md`. Reconcile the multi-commit-decompose
  section AS A WHOLE so the two Mode A edits that already landed (P1.M1.T2.S2 arbiter freeze narrative;
  P2.M1.T2.S2 planner mode-conditional/files/soft-target) cohere, NO stale pre-freeze references remain
  in the section, and the Safety bullets reflect FR-M1d (the arbiter is the third freeze surface). Then
  VERIFY `docs/cli.md` + `docs/configuration.md` require NO changes (FR-M4 soft target is derived from
  the existing `max_commits`; FR-M3 `files` is automatic planner output — zero new flags/keys) and LEAVE
  THEM UNTOUCHED. Zero code, zero new files.

  CONTRACT (P3.M1.T1.S2, verbatim from the work item):
    1. RESEARCH NOTE: The Mode A edits landed in P1.M1.T2.S2 (arbiter freeze narrative) and P2.M1.T2.S2
       (planner mode-conditional/files/soft-target). This task reconciles the docs/how-it-works.md
       decompose section as a WHOLE so the two edits cohere and the 'Safety' bullets (line ~121-127) are
       consistent with FR-M1d (e.g. 'Start-of-run freeze' bullet should name the arbiter as a freeze
       surface, not just stager).
    2. INPUT: The Mode A edits from P1.M1.T2.S2 and P2.M1.T2.S2, plus the implemented behavior from
       P1.M1.T2.S1 / P2.M1.T1.S1 / P2.M1.T2.S1 / P2.M1.T3.S1.
    3. LOGIC: Reconcile the full decompose narrative in docs/how-it-works.md so the arbiter-freeze and
       planner-files/soft-target descriptions are internally consistent (no stale 'git status --porcelain'
       references anywhere in the decompose section; the 'Safety' bullets reflect that the arbiter now
       derives strictly from T_start). Cross-check the data-flow diagram caption if present. Then VERIFY
       docs/cli.md and docs/configuration.md require NO changes (FR-M4 soft target is derived from existing
       max_commits; FR-M3 files is automatic — no new flags/keys) and leave them untouched. Mock: none
       (docs review/edit).
    4. OUTPUT: A coherent, stale-free docs/how-it-works.md decompose section; cli.md/configuration.md
       confirmed unchanged.
    5. DOCS: [Mode B] This subtask IS the how-it-works changeset-level reconciliation. Depends on the Mode
       A edits + implementing subtasks so it runs last.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `docs/cli.md`, `docs/configuration.md` — VERIFY-ONLY (grep-confirmed: no new flag/key needed; the
      soft target is derived from max_commits, `files` is automatic). LEAVE BYTE-UNCHANGED.
    - `README.md` — the sibling P3.M1.T1.S1 owns it (it links INTO this file's decompose anchor).
    - `docs/providers.md` — unaffected (the stager-scoping paragraph there is unchanged).
    - Any `.go` file / go.mod / go.sum — UNCHANGED (docs-only; zero code impact).
    - PRD.md / tasks.json / prd_snapshot.md — READ-ONLY (never modify).
    - The non-decompose sections of how-it-works.md (single-commit, diff-capture, run lock, rescue,
      prompt engineering) — out of scope.

  DELIVERABLES (1 file MODIFIED, 0 new):
    EDIT docs/how-it-works.md — 3 surgical edits within the decompose section (~lines 47-130):
      (1) diagram planner-input label: "full working-tree diff" → "T_start diff" (coherence with the
          freeze narrative);
      (2) diagram single-shortcut arrow: "git add -A → CommitStaged (one call) → done" → "commit T_start
          (planner's message) → done" (stale pre-freeze wording — FR-M11 commits T_start directly);
      (3) Safety "Start-of-run freeze" bullet: add the arbiter as the third freeze surface (FR-M1d).

  SUCCESS: the decompose section is internally consistent (diagram + narrative + Safety bullets all
  reflect the freeze; no stale `git add -A`-as-commit-path / `git status --porcelain` references); the
  Safety "Start-of-run freeze" bullet names BOTH the stager (FR-M1c) and the arbiter (FR-M1d); cli.md +
  configuration.md are byte-unchanged; `git status --short` shows ONLY `M docs/how-it-works.md`.

---

## Goal

**Feature Goal**: Reconcile the `docs/how-it-works.md` multi-commit-decompose section into a single
coherent, stale-free narrative after two separately-landed Mode A edits (P1.M1.T2.S2 arbiter freeze
narrative; P2.M1.T2.S2 planner files + mode-conditional + soft target). Both edits are mutually consistent
and internally correct, but each deliberately skipped spots the other owned — leaving (a) two pre-freeze
remnants in the pipeline data-flow diagram (a stale `git add -A → CommitStaged` single-shortcut arrow and
an imprecise "full working-tree diff" planner-input label) and (b) a Safety bullet ("Start-of-run freeze")
that names only the stager as a freeze surface, not the arbiter (FR-M1d). This task closes those three
gaps so a reader skimming the diagram OR the Safety bullets gets the same arbiter-inclusive freeze
guarantee the narrative paragraphs already state. It also VERIFY-ONLY confirms `docs/cli.md` and
`docs/configuration.md` need no changes (the v2.2 improvements add zero flags/keys).

**Deliverable** (1 file MODIFIED, 0 new): `docs/how-it-works.md` with 3 surgical edits inside the
decompose section — 2 in the pipeline diagram (code-fence lines 1 + 5), 1 in the Safety bullets (line 128).

**Success Definition**:
- The pipeline diagram's planner-input label reads "T_start diff (binary placeholders)" (not "full
  working-tree diff"), matching the freeze narrative ("The planner partitions T_start's diff").
- The pipeline diagram's single-shortcut arrow reads "commit T_start (planner's message) → done" (not
  "git add -A → CommitStaged (one call) → done"), matching FR-M11 (commit T_start directly) and FR-M10's
  null path.
- The Safety "Start-of-run freeze" bullet names BOTH freeze surfaces: the stager (content-subset check,
  FR-M1c) AND the arbiter (gate + diff + trees from T_start + tipTree, never live, FR-M1d).
- No stale `git status --porcelain` reference and no stale `git add -A`-as-a-commit-path reference remain
  anywhere in the decompose section (the `git add -A` in the "Freeze enforcement" paragraph STAYS — it
  describes a stager misbehavior that is aborted, not a commit path).
- `docs/cli.md` and `docs/configuration.md` are BYTE-UNCHANGED (`git diff --exit-code` ⇒ empty for both).
- `git status --short` shows EXACTLY `M docs/how-it-works.md` — no other file.

## User Persona

**Target User**: a developer reading `docs/how-it-works.md` to understand decompose's concurrency safety
before running it on a busy working tree (the PRD §7.1 "plan-holder" who keeps coding while stagecoach
runs). They skim the pipeline diagram and the Safety bullets.

**Use Case**: the user wants to confirm that a file a concurrent process (editor save, another agent)
writes mid-run can never enter a commit. They look at the diagram (does any arrow read `git add -A`
against the live tree?) and the Safety bullets (does "Start-of-run freeze" cover the whole pipeline?).

**Pain Points Addressed**: today the diagram's single-shortcut arrow reads `git add -A → CommitStaged` (a
live-tree commit path — the pre-freeze behavior) and the Safety "Start-of-run freeze" bullet names only
the stager. A careful reader could conclude the single shortcut or the arbiter is a freeze hole. This
reconciliation makes the diagram and the Safety bullet say what the narrative already guarantees: the
freeze is complete across planner, stager, arbiter, and all shortcuts.

## Why

- **Closes the how-it-works half of the v2.2 changeset-level doc sync (Mode B).** system_context.md
  frames T1 as "README.md + docs/how-it-works.md cross-cutting sweep." The sibling P3.M1.T1.S1 owns the
  README; THIS task owns the how-it-works reconciliation. Both run last and depend on every implementing
  subtask (P1.*/P2.* are COMPLETE).
- **The two Mode A edits were scoped narrowly and left gaps by design.** P1.M1.T2.S2's PRP explicitly
  says "No edits to: … the Safety bullets"; P2.M1.T2.S2 touched only the table + the mode-conditional
  paragraph. Neither touched the diagram's single-shortcut arrow or planner-input label. A holistic pass
  — the explicit purpose of THIS task — catches the inconsistencies a per-edit PRP scope would miss.
- **The diagram is the most-scanned artifact.** Readers skim the ASCII pipeline before reading prose. A
  diagram arrow that reads `git add -A → CommitStaged` directly contradicts the freeze narrative
  ("never a fresh re-read of the live tree") one paragraph below it. Reconciling it is the highest-value
  edit in the section.
- **FR-M1d makes the arbiter the third freeze surface; the Safety bullet must say so.** The work item is
  explicit: "'Start-of-run freeze' bullet should name the arbiter as a freeze surface, not just stager."
  The narrative paragraphs (lines 67 + 75) already do; the Safety bullet — the scanned summary — does not.

## What

A pure markdown edit to `docs/how-it-works.md`, 3 surgical changes inside the decompose section. No new
files. No code. Specifically:

- **Edit 1 (diagram, planner-input label):** `full working-tree diff (binary placeholders)` →
  `T_start diff (binary placeholders)`.
- **Edit 2 (diagram, single-shortcut arrow):** `single? ──yes──▶ git add -A → CommitStaged (one call)
  → done` → `single? ──yes──▶ commit T_start (planner's message) → done`.
- **Edit 3 (Safety bullet):** the "Start-of-run freeze" bullet gains an arbiter clause naming it the
  third freeze surface (gate + diff + trees from T_start + tipTree, never live — FR-M1d), alongside the
  existing stager content-subset clause (FR-M1c).

### Success Criteria

- [ ] The diagram planner-input label reads "T_start diff (binary placeholders)".
- [ ] The diagram single-shortcut arrow reads "commit T_start (planner's message) → done" (no `git add -A`,
      no `CommitStaged`).
- [ ] The Safety "Start-of-run freeze" bullet names BOTH the stager (FR-M1c content-subset) AND the
      arbiter (FR-M1d: gate/diff/trees from T_start + tipTree, never live).
- [ ] No `git status --porcelain` and no `git add -A`-as-commit-path reference remains in the decompose
      section (the "Freeze enforcement" paragraph's `git add -A` STAYS — it is a stager-misbehavior
      description, not a commit path).
- [ ] `docs/cli.md` + `docs/configuration.md` are byte-unchanged; `git status --short` = ONLY
      `M docs/how-it-works.md`.

## All Needed Context

### Context Completeness Check

_Pass._ A technical writer with no prior repo knowledge can implement this from: the exact 3 edits with
verbatim OLD/NEW text (§"Implementation Blueprint" — copy/paste); what each Mode A edit already did + the
explicit gaps they left (findings §1); the spots CONSIDERED and left unchanged so they don't over-edit
(findings §3 — the "Freeze enforcement" `git add -A` stays; "No index resets" is accurate; the table +
arbiter box are already correct); the cli.md/configuration.md verification result (findings §4 — grep-
confirmed no new flag/key; leave byte-unchanged); the diagram ASCII-alignment care (findings §6); and the
scope fence (findings §5 — how-it-works ONLY). No Go/git/freeze-implementation knowledge required — the
code is COMPLETE; this task only reconciles its documentation.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE findings (the 3 edits + the gap analysis + the verification)
- docfile: plan/008_82253c999440/P3M1T1S2/research/findings.md
  why: §1 what the two Mode A edits already did (so this task does NOT duplicate them) + the explicit
       gaps (P1.M1.T2.S2's PRP said "No edits to: … the Safety bullets"; neither edit touched the
       diagram's single-shortcut arrow or planner-input label); §2 the THREE edits with exact OLD/NEW
       text + rationale; §3 the spots CONSIDERED and LEFT UNCHANGED (the "Freeze enforcement" git add -A
       STAYS — it is a stager-misbehavior description; "No index resets" is accurate; the four-roles
       table + arbiter box already updated/correct); §4 the cli.md/configuration.md grep-confirmed
       verification (no new flag/key — leave byte-unchanged); §5 scope fence; §6 diagram ASCII care.
  critical: §2 (the 3 exact edits); §3 (do NOT over-edit — the "Freeze enforcement" git add -A is NOT
            stale; "No index resets" is NOT contradicted by the arbiter's index sync); §4 (cli.md +
            configuration.md need NO changes — verify, don't edit).

# MUST READ — the FILE TO EDIT
- file: docs/how-it-works.md   (EDIT — the ONLY file this task touches)
  section: the decompose section (~lines 47-130). The 3 edits: (1) the pipeline diagram code-fence
           (~line 70) planner-input label, line 1 of the fence; (2) the same diagram's single-shortcut
           arrow (~line 74), line 5 of the fence; (3) the Safety "Start-of-run freeze" bullet (~line 128).
  why: the 3 reconciliation targets live here. The diagram is a ```text code fence (preserve ASCII
       alignment — see findings §6). The Safety bullet is the 4th bullet under "### Safety".
  pattern: the diagram uses box-drawing chars (┌─┐│└┘▼◀═) + arrows (→ ▶ ◀). The labels are to the RIGHT
           of / BELOW the boxes. Edit ONLY the label text; do NOT move box chars. The Safety bullets use
           `- **<bold lead>** — <prose>` form with mid-sentence backticks for code tokens (T_start, tipTree).
  gotcha: do NOT touch any other paragraph in the section (Trigger, four-roles table, Overlapped staging,
          Stage-while-editing, Frozen tree snapshots, Tree-to-tree diffs, Serialized publication, Start-of-
          run freeze PARAGRAPH (line 67 — already correct), Freeze enforcement (line 69 — its git add -A
          STAYS), One-file short-circuit, Mode-conditional planner rules, Arbiter leftover reconciliation,
          the other 3 Safety bullets). Do NOT touch the non-decompose sections.

# MUST READ — the two Mode A edit PRPs (to confirm what they touched + explicitly skipped)
- docfile: plan/008_82253c999440/P1M1T2S2/PRP.md
  section: its "3 edits" (arbiter paragraph L75; Start-of-run freeze PARAGRAPH L67; diagram GATE label L93)
           + its explicit "No edits to: … the Safety bullets" (its PRP L90).
  why: confirms P1.M1.T2.S2 did NOT touch the Safety bullets or the diagram's non-gate lines — those are
       THIS task's gaps. Also documents the diagram ASCII-alignment care (its PRP L208) — same discipline.
- docfile: plan/008_82253c999440/P2M1T2S2/PRP.md
  section: its 2 edits (four-roles table planner JSON +files L59; Mode-conditional planner rules L73).
  why: confirms P2.M1.T2.S2 did NOT touch the diagram or the Safety bullets.

# MUST READ — the sibling README PRP (to avoid collision + confirm the anchor link target)
- docfile: plan/008_82253c999440/P3M1T1S1/PRP.md
  section: it edits README.md ONLY; it lists docs/how-it-works.md as READ-ONLY (owned by THIS task); its
           Features row links to docs/how-it-works.md#multi-commit-decomposition.
  why: confirms no collision (README is the sibling's file; how-it-works is THIS task's file) + confirms
       the decompose section heading "## Multi-commit decomposition" (the link target) MUST stay intact
       (do NOT rename the heading).

# READ — the authoritative FRs the reconciliation reflects
- docfile: PRD.md   (READ-ONLY)
  section: §9.14 FR-M1c (stager freeze enforcement — content-subset after each staging step), FR-M1d
           (arbiter freeze parity — gate/diff/trees from T_start + tipTree, never live; the THIRD freeze
           surface), FR-M11 (single shortcut commits T_start directly with the planner's message), FR-M10
           (null path = "commit T_start directly"); §13.6.5 (the arbiter's frozen-leftover gate).
  why: FR-M1d is the authoritative source for the arbiter-inclusive Safety-bullet wording; FR-M11/FR-M10
       for the diagram's single-shortcut arrow ("commit T_start directly", not "git add -A → CommitStaged").
  critical: §9.14 FR-M1d — the arbiter is the THIRD freeze surface; the Safety bullet MUST name it.

# READ — system_context (this task's charter)
- docfile: plan/008_82253c999440/docs/architecture/system_context.md   (path may be …/architecture/…)
  section: the T1 line — "README.md + docs/how-it-works.md cross-cutting sweep … how-it-works reconciles
           the decompose section (arbiter freeze + planner files/soft-target coherence); cli.md/
           configuration.md verified untouched."
  why: confirms the how-it-works reconciliation + the cli.md/configuration.md no-op verification are
       exactly this task.
```

### Current Codebase tree (relevant slice)

```bash
docs/how-it-works.md        # EDIT (the ONLY file this task touches): 3 surgical edits in the decompose
                            #   section — diagram planner-input label (fence L1), diagram single-shortcut
                            #   arrow (fence L5), Safety "Start-of-run freeze" bullet (~L128).
docs/cli.md                 # READ-ONLY / VERIFY-ONLY (grep-confirmed: no new flag/key; byte-unchanged).
docs/configuration.md       # READ-ONLY / VERIFY-ONLY (grep-confirmed: no new key; byte-unchanged).
docs/providers.md           # UNCHANGED (the stager-scoping paragraph is unaffected).
README.md                   # UNCHANGED (sibling P3.M1.T1.S1 owns it).
(go.mod, go.sum, *.go)      # UNCHANGED (docs-only; zero code impact).
```

### Desired Codebase tree with files to be modified

No NEW files. One file MODIFIED:

```bash
docs/how-it-works.md   # MODIFY — 3 edits within the decompose section:
                       #   Edit 1 (diagram L1):   "full working-tree diff (binary placeholders)"
                       #                          → "T_start diff (binary placeholders)"
                       #   Edit 2 (diagram L5):   "git add -A → CommitStaged (one call) → done"
                       #                          → "commit T_start (planner's message) → done"
                       #   Edit 3 (Safety ~L128): the "Start-of-run freeze" bullet gains an arbiter
                       #                          clause (FR-M1d) alongside the stager clause (FR-M1c).
# NO other file changes. cli.md/configuration.md byte-unchanged. NO .go edits. NO go.mod/go.sum edits.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the "Freeze enforcement" git add -A STAYS — findings §3): the paragraph at line 69 says
     "a stager that ran a bare `git add -A` … is a hard abort." That `git add -A` is a description of a
     stager MISBEHAVIOR the enforcement catches — it is NOT a commit-path reference. Do NOT remove it.
     The ONLY stale `git add -A` is the diagram's single-shortcut arrow (Edit 2). A common mistake is to
     "purge all git add -A" and delete the enforcement paragraph's correct reference. -->

<!-- CRITICAL (the Safety bullet must name BOTH freeze surfaces — work item): the "Start-of-run freeze"
     bullet currently names only the stager ("Each staging step is verified…"). FR-M1d makes the arbiter
     the THIRD freeze surface. Edit 3 adds the arbiter clause. Do NOT delete the stager clause (FR-M1c is
     still live) — ADD the arbiter alongside it. -->

<!-- CRITICAL (cli.md + configuration.md are VERIFY-ONLY — findings §4): grep confirms neither file needs
     a new flag/key (the FR-M4 soft target is max_commits/2 — DERIVED from the existing max_commits knob;
     the FR-M3 `files` field is automatic planner output). Leave BOTH byte-unchanged. `git diff --exit-code
     docs/cli.md docs/configuration.md` must be empty. If `git status --short` shows either as modified,
     you've gone out of scope. -->

<!-- GOTCHA (diagram ASCII alignment — findings §6): the pipeline diagram is a ```text code fence with
     box-drawing chars (┌─┐│└┘▼◀═). Edits 1 + 2 change ONLY the label text to the right of / below the
     boxes; they do NOT move any box-drawing char. Edit 1's new label is SHORTER than the old (more
     trailing space — fine). Edit 2's arrow glyphs (│ ── ▶ →) are PRESERVED; only the text after ▶ changes.
     Eyeball the rendered diagram after the edit (the boxes still connect; the arrows still point the
     right way). -->

<!-- GOTCHA (the single-shortcut arrow is FR-M11, NOT the escape-hatch): the diagram's "single? ──yes"
     branch is the planner returning single:true (FR-M11 single-call shortcut). It commits T_start with
     the PLANNER'S message — it neither runs `git add -A` nor calls `CommitStaged` (the v1 message-
     regenerating function). The escape-hatch (--single) bypasses the planner entirely (routed before the
     diagram) and is NOT this arrow. Edit 2's wording "commit T_start (planner's message)" is unambiguous. -->

<!-- GOTCHA (do NOT rename the "## Multi-commit decomposition" heading — L47): the sibling README's
     Features row links to #multi-commit-decomposition (the GitHub slug of that heading). Renaming it
     breaks the link. The heading is OUT OF SCOPE anyway. -->

<!-- GOTCHA (the "No index resets" Safety bullet is NOT stale — findings §3): it describes the LOOP's
     accumulate-never-reset invariant (a safety property for overlapped staging). The arbiter's post-loop
     index-sync to T_start is a separate step (covered in the arbiter paragraph L75). After the arbiter,
     HEAD.tree == T_start and index == T_start, so "the index is clean relative to HEAD" still holds. Do
     NOT edit this bullet — it is accurate for its scope. -->

<!-- GOTCHA (scope fence — findings §5): touch ONLY docs/how-it-works.md, and within it ONLY the 3 spots.
     The four-roles table (already +files), the Start-of-run freeze PARAGRAPH (L67, already arbiter-
     inclusive), the Arbiter leftover reconciliation paragraph (L75, already freeze-safe), the diagram
     gate label (L93, already "frozen leftover empty?"), and the other 3 Safety bullets are ALL already
     correct (fixed by the Mode A edits). Editing them is duplication / scope creep. -->
```

## Implementation Blueprint

### Edit 1 — diagram planner-input label (code-fence line 1)

Locate the first line inside the ```` ```text ```` pipeline diagram (under `### Pipeline flow`), which
currently reads:

> OLD: `            ┌────────────┐   full working-tree diff (binary placeholders)`

Replace ONLY the trailing label (keep the box `┌────────────┐` and its leading spaces byte-identical):

> NEW: `            ┌────────────┐   T_start diff (binary placeholders)`

Rationale: post-freeze the planner receives `TreeDiff(baseTree, T_start)` — the frozen tree-to-tree diff,
not a live working-tree read. This matches the freeze narrative (L67: "The planner partitions T_start's
diff (never a fresh re-read of the live tree)"). The `(binary placeholders)` clause is preserved (FR3c).

### Edit 2 — diagram single-shortcut arrow (code-fence line 5)

Locate the line immediately after the planner's JSON output line in the same diagram:

> OLD: `                  │ single? ──yes──▶ git add -A → CommitStaged (one call) → done`

Replace with:

> NEW: `                  │ single? ──yes──▶ commit T_start (planner's message) → done`

Rationale: this branch is the FR-M11 single-call shortcut (planner returned `single:true` + a message).
Per FR-M11 (updated) + FR-M10's null path, the shortcut commits the frozen `T_start` directly with the
planner's message — it neither runs a live `git add -A` nor calls `CommitStaged` (the v1 message-
regenerating path). The implemented `runSingleShortcut` does `treePrime := tStart` → `publishCommit`. The
`│`, `──yes──▶`, and `→` glyphs are preserved (only the text after `▶` changes).

### Edit 3 — Safety "Start-of-run freeze" bullet (~line 128)

Locate the 4th bullet under `### Safety` (the "Start-of-run freeze" bullet). It currently reads:

> OLD: `- **Start-of-run freeze** — T_start captures the full working-tree change set at decompose activation; concurrent edits never enter any commit. Each staging step is verified as a content-subset of T_start.`

Replace with:

> NEW: `- **Start-of-run freeze** — T_start captures the full working-tree change set at decompose activation; concurrent edits never enter any commit. The stager is verified as a content-subset of T_start after each staging step (FR-M1c), and the arbiter — the third freeze surface — derives its gate, its diff, and every tree it commits strictly from T_start and tipTree, never a live re-read (FR-M1d).`

Rationale: the work item requires the bullet name the arbiter as a freeze surface. FR-M1c (stager
content-subset) is preserved; FR-M1d (arbiter gate/diff/trees from T_start + tipTree) is added. This
makes the bullet consistent with the narrative paragraphs (L67 "the arbiter (its gate, its diff, and its
leftover staging)"; L75 "The live working tree is never consulted for the gate"). The other 3 Safety
bullets (Atomic and safe / Frozen content / No index resets) are byte-unchanged.

### Implementation Tasks (ordered)

```yaml
Task 1: EDIT docs/how-it-works.md — Edit 1 (diagram planner-input label)
  - LOCATE the ```text pipeline diagram under "### Pipeline flow"; its FIRST line is the planner box with
    the trailing label "full working-tree diff (binary placeholders)".
  - REPLACE the label text "full working-tree diff" → "T_start diff" (keep "(binary placeholders)"; keep
    the box chars + leading spaces byte-identical).
  - VERIFY: the box `┌────────────┐` is intact; the line still renders inside the code fence.
  - PLACEMENT: diagram code-fence line 1.

Task 2: EDIT docs/how-it-works.md — Edit 2 (diagram single-shortcut arrow)
  - LOCATE the diagram line "│ single? ──yes──▶ git add -A → CommitStaged (one call) → done" (the branch
    right after the planner's JSON output line).
  - REPLACE the post-▶ text "git add -A → CommitStaged (one call) → done" → "commit T_start (planner's
    message) → done". PRESERVE the leading "│ single? ──yes──▶" glyph sequence.
  - VERIFY: the arrow still reads as a branch off the planner box; the `→` before "done" is intact.
  - PLACEMENT: diagram code-fence line 5.

Task 3: EDIT docs/how-it-works.md — Edit 3 (Safety Start-of-run freeze bullet)
  - LOCATE the "### Safety" bullet list; the 4th bullet is "- **Start-of-run freeze** — …".
  - REPLACE the OLD bullet (§"Edit 3" OLD) with the NEW bullet (§"Edit 3" NEW) — exact-text edit (the OLD
    bullet is unique in the file).
  - PRESERVE byte-for-byte: the 3 OTHER Safety bullets (Atomic and safe / Frozen content / No index
    resets), the "### Safety" heading, the preceding intro sentence, and the trailing "See
    [configuration.md]… [cli.md]…" sentence.
  - VERIFY: the FR-M1c + FR-M1d references are present; the em-dash "—" is intact (UTF-8); the backticks
    around T_start / tipTree are intact.
  - PLACEMENT: Safety section, 4th bullet.

Task 4: VERIFY cli.md + configuration.md are untouched (the work item's verify step)
  - GREP docs/cli.md: confirm `--max-commits` is documented (it is, ~L36) and there is NO `soft target`/
    `soft-target`/`files field`/`per-file` knob (grep returns none). The soft target is max_commits/2
    (derived); `files` is automatic. ⇒ NO edit needed.
  - GREP docs/configuration.md: confirm `[generation].max_commits` is documented (~L217) and there is NO
    new key for the soft target or `files` (grep returns none). ⇒ NO edit needed.
  - VERIFY `git diff --exit-code docs/cli.md docs/configuration.md` ⇒ empty (byte-unchanged).

Task 5: VERIFY (docs-only validation — findings §5/§6)
  - `git diff docs/how-it-works.md` — the ONLY changes are the 3 edits (2 diagram labels + 1 Safety
    bullet); no other line in the file moved.
  - Diagram render-check: the pipeline diagram still renders correctly (boxes connect; arrows point the
    right way; alignment intact — eyeball it).
  - Stale-reference sweep: `grep -nE 'git status --porcelain|status --porcelain' docs/how-it-works.md`
    ⇒ no matches in the decompose section; `grep -n 'git add -A' docs/how-it-works.md` ⇒ matches ONLY in
    the "Freeze enforcement" paragraph (the stager-misbehavior description — correct) and NONE in the
    diagram.
  - Scope fence: `git status --short` ⇒ EXACTLY `M docs/how-it-works.md` (no cli.md, no configuration.md,
    no README.md, no .go file, no go.mod).
```

### Integration Points

```yaml
DATABASE: none.
CONFIG:  none (no new flags/keys — the soft target is derived from max_commits; `files` is automatic
         planner output). cli.md + configuration.md are VERIFY-ONLY (byte-unchanged).
ROUTES:  none.
DOCS:
  - This task IS the how-it-works changeset-level reconciliation (Mode B). It depends on the two Mode A
    edits (P1.M1.T2.S2 + P2.M1.T2.S2, both COMPLETE) and the implementing subtasks (P1.*/P2.* COMPLETE),
    and runs last to make the section cohere.
  - The sibling P3.M1.T1.S1 (README) links INTO this file's #multi-commit-decomposition anchor; do NOT
    rename that heading.
BUILD/TEST: none — docs-only edit; zero Go code change. `go build`/`go test` are unaffected (a no-op
  confirmation only).
```

## Validation Loop

### Level 1: Markdown + Diagram Integrity (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

# 1a. The diff is EXACTLY the 3 edits (2 diagram labels + 1 Safety bullet) — no other line moved.
git diff docs/how-it-works.md          # eyeball: only the planner-input label, the single-shortcut
                                       # arrow, and the Start-of-run freeze bullet changed.

# 1b. Diagram still a coherent code fence (boxes connect; arrows intact).
sed -n '/### Pipeline flow/,/### Key design points/p' docs/how-it-works.md   # eyeball the rendered diagram.

# 1c. The "## Multi-commit decomposition" heading is UNCHANGED (the README link target).
grep -n "^## Multi-commit decomposition" docs/how-it-works.md                # still present (L47).

# Expected: zero unintended changes. Fix any before proceeding.
```

### Level 2: Content Validation (the reconciliation actually landed)

```bash
# 2a. Edit 1: planner-input label now "T_start diff".
grep -n "T_start diff (binary placeholders)" docs/how-it-works.md            # the diagram label.

# 2b. Edit 2: single-shortcut arrow no longer says git add -A / CommitStaged.
grep -n "commit T_start (planner's message)" docs/how-it-works.md            # the new arrow text.
! grep -n "git add -A → CommitStaged" docs/how-it-works.md                   # the stale arrow is GONE.

# 2c. Edit 3: Safety bullet names BOTH freeze surfaces (stager FR-M1c + arbiter FR-M1d).
grep -n "Start-of-run freeze" docs/how-it-works.md | grep "FR-M1c"
grep -n "Start-of-run freeze" docs/how-it-works.md | grep "FR-M1d"           # both present.

# Expected: all present. If any is absent, the edit is incomplete.
```

### Level 3: Stale-Reference Sweep + Scope Fence (System Validation)

```bash
# 3a. No stale git status --porcelain anywhere in the decompose section.
! grep -nE 'git status --porcelain|status --porcelain' docs/how-it-works.md  # none (already true; confirm).

# 3b. The ONLY git add -A references are the CORRECT ones (Freeze enforcement stager-misbehavior;
#     none in the diagram commit path).
grep -n "git add -A" docs/how-it-works.md
# Expected: matches ONLY in the "Freeze enforcement" paragraph (~L69). NONE in the diagram.

# 3c. cli.md + configuration.md BYTE-UNCHANGED (the verify step).
git diff --exit-code docs/cli.md docs/configuration.md && echo "OK: both byte-unchanged"

# 3d. Scope fence: ONLY docs/how-it-works.md changed.
git status --short                # Expected: EXACTLY "M docs/how-it-works.md". No cli.md, no
                                  # configuration.md, no README.md, no .go file, no go.mod.

# 3e. No code impact (docs-only): the build/test suite is unaffected (sanity, not required).
go build ./... && go test ./...   # Expected: GREEN (no code moved; belt-and-suspenders).
```

### Level 4: Render & Readability (Domain-Specific Validation)

```bash
# 4a. Render the decompose section (markdown preview / GitHub view) and confirm:
#   - The pipeline diagram renders cleanly: the planner box label reads "T_start diff"; the single-
#     shortcut arrow reads "commit T_start (planner's message) → done"; boxes still align; arrows point
#     the right way.
#   - The Safety bullet list renders the refreshed "Start-of-run freeze" bullet with both FR-M1c (stager)
#     and FR-M1d (arbiter) clauses; the other 3 bullets are unchanged.
#
# 4b. Read the decompose section top-to-bottom for COHERENCE:
#   - The diagram's "T_start diff" planner input matches L67's "The planner partitions T_start's diff".
#   - The diagram's "commit T_start" single-shortcut matches L73's "the one-file/single shortcuts …
#     commit T_start directly" (FR-M11/M10) — no arrow suggests a live git add -A commit path.
#   - The Safety "Start-of-run freeze" bullet matches L67 + L75 (arbiter = third freeze surface).
#   - No paragraph contradicts another.
#
# Expected: renders cleanly; reads naturally; internally consistent end-to-end.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 (markdown/diagram integrity): the diff is exactly the 3 edits; the diagram still renders;
      the `## Multi-commit decomposition` heading is unchanged.
- [ ] Level 2 (content): Edit 1 label, Edit 2 arrow, Edit 3 bullet all landed (the 3 grep checks pass).
- [ ] Level 3 (stale sweep + scope): no `git status --porcelain`; the only `git add -A` is the Freeze-
      enforcement stager-misbehavior one; cli.md + configuration.md byte-unchanged; `git status --short`
      = ONLY `M docs/how-it-works.md`; `go build && go test` green (no-op).
- [ ] Level 4 (render): the diagram + Safety list render cleanly; the section reads coherently.

### Feature Validation

- [ ] The diagram planner-input label reads "T_start diff (binary placeholders)".
- [ ] The diagram single-shortcut arrow reads "commit T_start (planner's message) → done" (no `git add -A`,
      no `CommitStaged`).
- [ ] The Safety "Start-of-run freeze" bullet names BOTH the stager (FR-M1c) AND the arbiter (FR-M1d).
- [ ] The decompose section is internally consistent (diagram ↔ narrative ↔ Safety bullets all reflect
      the arbiter-inclusive freeze; no stale commit-path references).
- [ ] cli.md + configuration.md are byte-unchanged (the verify step — no new flag/key was needed).

### Code Quality Validation

- [ ] Markdown style matches the existing diagram (box-drawing chars + label placement) and the existing
      Safety bullets (`- **lead** — prose` form, mid-sentence backticks, em-dashes).
- [ ] Anti-patterns avoided (no over-edit — the "Freeze enforcement" git add -A STAYS; "No index resets"
      untouched; the already-correct paragraphs untouched; no cli.md/configuration.md edit; no .go edit).
- [ ] The diagram's ASCII alignment is preserved (boxes connect; arrows point correctly).

### Documentation & Deployment

- [ ] The decompose section is the coherent, stale-free authoritative narrative for v2.2 decompose.
- [ ] The Safety bullet reflects FR-M1d (arbiter = third freeze surface) — the v2.2 headline guarantee.
- [ ] cli.md/configuration.md confirmed (grep) to need no changes — no new flags/keys to document.

---

## Anti-Patterns to Avoid

- ❌ Don't edit any file other than `docs/how-it-works.md` — cli.md + configuration.md are VERIFY-ONLY
  (grep-confirmed no new flag/key); README.md is the sibling P3.M1.T1.S1's file; providers.md is unaffected.
- ❌ Don't remove the "Freeze enforcement" paragraph's `git add -A` (line 69) — it describes a stager
  MISBEHAVIOR that is aborted, NOT a commit path. The only stale `git add -A` is the diagram's single-
  shortcut arrow (Edit 2). "Purge all git add -A" would delete a correct reference.
- ❌ Don't delete the stager clause when adding the arbiter to the Safety bullet — FR-M1c (stager content-
  subset) is still live; ADD the arbiter (FR-M1d) alongside it.
- ❌ Don't over-edit — the four-roles table, the Start-of-run freeze PARAGRAPH (L67), the Arbiter
  leftover reconciliation paragraph (L75), the diagram gate label (L93), and the other 3 Safety bullets
  are ALL already correct (fixed by the Mode A edits). Touching them is duplication / scope creep.
- ❌ Don't rename the `## Multi-commit decomposition` heading (L47) — the sibling README's Features row
  links to its `#multi-commit-decomposition` anchor; renaming breaks the link.
- ❌ Don't move any box-drawing character in the diagram — Edits 1 + 2 change ONLY label text (right of /
  below the boxes); preserve the `│ ── ▶ → ┌─┐│└┘▼◀═` alignment. Eyeball the rendered diagram.
- ❌ Don't edit any `.go` file, go.mod, or go.sum — docs-only task; zero code impact.
- ❌ Don't invent new flags, keys, or version numbers — v2.2 is internal-quality (P1/P2 add zero flags/
  keys); the soft target is derived from `max_commits`, `files` is automatic.
- ❌ Don't touch the "No index resets" Safety bullet — it accurately describes the LOOP's accumulate-
  never-reset invariant; the arbiter's post-loop index-sync is covered in the arbiter paragraph (L75).
- ❌ Don't conflate the diagram's "single?" arrow (FR-M11 planner shortcut) with the escape-hatch
  (--single, which bypasses the planner entirely). Edit 2's wording "commit T_start (planner's message)"
  is unambiguous.

---

**Confidence Score: 9/10** — This is a low-risk, 3-edit markdown reconciliation to a single file with no
code impact. The gap analysis is precise (the two Mode A edits deliberately skipped the Safety bullets +
the diagram's non-gate lines — their PRPs say so explicitly), the 3 edits are given verbatim with
OLD/NEW text, the "do not over-edit" list (findings §3) prevents the obvious mistake of purging the
correct `git add -A` in the Freeze-enforcement paragraph, and the cli.md/configuration.md verification is
a grep-confirmed no-op. The only mechanical care is preserving the diagram's ASCII alignment (Edits 1 + 2
change only label text, not box chars — eyeball after). The sibling P3.M1.T1.S1 (README) does not touch
how-it-works.md (confirmed READ-ONLY in its PRP), so there is no merge conflict. The implementing
subtasks (P1.*/P2.*) are COMPLETE, so the behavior being documented is frozen.
