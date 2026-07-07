---
name: "P1.M1.T2.S2 — Mode A doc edit: docs/how-it-works.md arbiter freeze narrative"
description: |
  Pure documentation edit (Mode A — rides with P1.M1.T2.S1). Bring docs/how-it-works.md's arbiter
  narrative into line with the freeze-safe arbiter (FR-M1d/M9/M10) that S1 lands. Three surgical edits
  in the decompose section: (1) REWRITE the "Arbiter leftover reconciliation" paragraph (line ~117) —
  the gate is the frozen leftover `diff-names(tipTree, T_start)`, NOT live `git status --porcelain`;
  the arbiter runs iff it is non-empty; a concurrent working-tree change cannot trigger it; the diff +
  resolution are frozen/tree-only-from-T_start; index synced to T_start. (2) REFINE the "Start-of-run
  freeze (T_start)" paragraph (line ~111) to name the arbiter's gate, diff, AND staging (FR-M1d's three
  freeze surfaces — current text names only "leftover staging"). (3) REFINE the pipeline-diagram gate
  label (line ~91) "git status clean?" → "frozen leftover empty?" so the diagram does not contradict
  the rewritten paragraph. No new flags/keys; do NOT touch docs/cli.md or docs/configuration.md. No
  code, no tests, no mocks. Docs-only.
---

## Goal

**Feature Goal**: Make `docs/how-it-works.md`'s arbiter narrative consistent with the freeze-safe
arbiter behavior (FR-M1d — the "third freeze surface"): the arbiter's **gate**, its **diff**, and its
**committed trees** all derive from frozen `T_start` / `tipTree`, never from a live `git status` read
or live `git add`. After this edit, the doc no longer claims the arbiter gates on `git status --porcelain`
(the v2.0–v2.1 loophole S1 closes), and the freeze property — a file written after `T_start` capture
cannot trigger the arbiter or enter any arbiter commit — is stated plainly.

**Deliverable**: ONE file modified — `docs/how-it-works.md` — with three surgical edits in the
decompose section:
1. **Rewrite** the "Arbiter leftover reconciliation" paragraph (~line 117): frozen gate/diff/staging.
2. **Refine** the "Start-of-run freeze (T_start)" paragraph (~line 111): name the arbiter's gate+diff+staging.
3. **Refine** the pipeline-diagram gate label (~line 91): "git status clean?" → "frozen leftover empty?".

No other file touched. No code, no tests, no config keys.

**Success Definition**: `docs/how-it-works.md` no longer references `git status --porcelain` anywhere;
the arbiter paragraph states the frozen-leftover gate, the freeze property, the frozen diff, and the
tree-only resolution + index sync; line 111 names all three freeze surfaces; the diagram gate label is
consistent with the paragraph; `git diff --stat` shows ONLY `docs/how-it-works.md`; `docs/cli.md` and
`docs/configuration.md` are untouched.

## User Persona

**Target User**: The reader of `docs/how-it-works.md` — a stagecoach user (or contributor) trying to
understand the decompose pipeline's concurrency safety. They want to know whether running `stagecoach`
while another tool (editor save, concurrent coding agent) writes to the working tree can contaminate
the commits.

**Use Case**: A user reads the "Key design points" / "Safety" subsections to decide whether decompose
is safe to run alongside their editor. The stale `git status --porcelain` gate suggests a concurrent
change could trigger the arbiter and land in a commit; the corrected narrative states the opposite
(and true) behavior: the frozen gate excludes concurrent changes.

**User Journey**: open `docs/how-it-works.md` → read the decompose "Pipeline flow" diagram + "Key
design points" → the arbiter paragraph + the Start-of-run freeze bullet together convey that the whole
run (planner + stager + arbiter) commits exactly the working-tree state captured at `T_start`.

**Pain Points Addressed**: Removes the doc/code drift where the doc described a known-closed loophole
(FR-M1d names it explicitly), which would mislead users about decompose's concurrency safety.

## Why

- **FR-M1d is the mandate.** PRD §9.14 FR-M1d names the arbiter as the "third freeze surface," held to
  the identical invariant as the stager: its gate, its diff, and its committed trees are all derived
  from `T_start`/`tipTree` (frozen SHAs), never a live working-tree read. S1 (P1.M1.T2.S1) implements
  exactly that. This doc edit is the Mode A ride-with-the-work documentation of it (SOW §5).
- **The doc currently describes the loophole.** Line ~117 gates the arbiter on `git status --porcelain`
  — the precise v2.0–v2.1 behavior FR-M1d closes ("In v2.0–v2.1 the arbiter gate read live
  `git status --porcelain` … so a concurrent change during the planner call was silently swept into an
  arbiter commit; FR-M1d closes that loophole"). Doc/code drift of this kind misleads users about the
  safety property that is the whole point of the freeze.
- **Consistency within the doc.** Line 111 (Start-of-run freeze) already claims the arbiter draws from
  `T_start`; line 117 contradicts it by gating on live status. The edit makes the two paragraphs (and
  the diagram) internally consistent and aligned with FR-M1d's three-surfaces framing.
- **No code, no flags, no config.** Pure narrative correction. Mode A = the doc edit rides WITH the
  work (S1); there is no separate docs subtask beyond this one.

## What

Three surgical edits to `docs/how-it-works.md` (decompose section), all in present tense (by landing,
S1 IS the behavior):

1. **Rewrite** the "Arbiter leftover reconciliation" paragraph: the gate is the frozen leftover
   `diff-names(tipTree, T_start)`; the arbiter runs iff it is non-empty; the live working tree is never
   consulted (not `git status --porcelain`), so a post-`T_start` file cannot trigger it or enter any
   arbiter commit; the arbiter is shown `TreeDiff(tipTree, T_start)` and decides amend-vs-new;
   stagecoach performs all git from frozen trees and syncs the index to `T_start`.
2. **Refine** the "Start-of-run freeze (T_start)" paragraph's arbiter clause: "the arbiter's leftover
   staging" → "the arbiter (its gate, its diff, and its leftover staging)".
3. **Refine** the pipeline-diagram gate label: "git status clean?" → "frozen leftover empty?".

No edits to: the four-roles table, the format-modes paragraph, the Safety bullets, the lock paragraph,
`docs/cli.md`, `docs/configuration.md`, any code, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Success Criteria

- [ ] `docs/how-it-works.md` contains ZERO occurrences of `git status --porcelain` (the stale gate).
- [ ] The "Arbiter leftover reconciliation" paragraph names the frozen gate (`diff-names(tipTree, T_start)`),
      the freeze property (concurrent change can't trigger/enter), the frozen diff (`TreeDiff(tipTree, T_start)`),
      and tree-only resolution + index sync to `T_start`.
- [ ] The "Start-of-run freeze (T_start)" paragraph names the arbiter's gate, diff, AND staging.
- [ ] The pipeline-diagram gate label reads "frozen leftover empty?" (not "git status clean?").
- [ ] `git diff --stat` shows ONLY `docs/how-it-works.md` changed.
- [ ] `docs/cli.md` and `docs/configuration.md` are UNCHANGED (`git diff --stat` empty for both).
- [ ] No code files changed; no new flags/keys/FR citations beyond the existing bullets.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the three current passages verbatim (with exact line numbers)
and gives the exact target text for each (ready to paste), plus the S1 behavior contract (gate/diff/
staging all frozen; index synced) and the scope fence (which other passages to leave alone). A docs
edit needs no toolchain — only the three quoted replacements. No inference required.

### Documentation & References

```yaml
# MUST READ — the behavior contract + the authoritative PRD narrative
- file: PRD.md
  why: "§9.14 FR-M1d (arbiter freeze parity — the third freeze surface: gate+diff+trees from T_start/tipTree,
        NOT live status/add; closes the v2.0–v2.1 loophole), FR-M9 (arbiter runs iff frozen leftover non-empty;
        shown TreeDiff(tipTree, T_start)), FR-M10 (resolution: tree-only-from-T_start; index synced via read-tree T_start).
        §13.6.5 is the full plain-English arbiter narrative the how-it-works edit mirrors."
  critical: "FR-M1d's '(1) The gate is the frozen leftover diff-names(tipTree, T_start), not git status --porcelain'
             is the single sentence this doc edit propagates into how-it-works.md. The doc must NOT cite live
             git status as the gate."

- docfile: plan/008_82253c999440/P1M1T2S1/PRP.md
  why: "The S1 CONTRACT (parallel; assume landed). Specifies the freeze-safe arbiter this doc must describe:
        gate = DiffTreeNames(tipTree, tStart); diff already frozen (TreeDiff); paths A/B treePrime:=tStart;
        path C OverlayTreePaths(tree[j], tStart, leftoverPaths); ReadTree(tStart) index sync after each path;
        NO StatusPorcelain/AddAll/Add in any arbiter path. S1's scope explicitly fences docs/* to THIS task (S2)."
  critical: "Treat S1 as landed. Write the doc in PRESENT tense — by the time this Mode A edit lands, S1 IS
             the behavior. Do NOT say 'will' or 'planned'."

- docfile: plan/008_82253c999440/P1M1T2S2/research/how_it_works_arbiter_notes.md
  why: "THIS task's research: the verbatim current text of all three spots (line 117 stale paragraph,
        line 111 imprecise clause, line 91 diagram label), the exact target text for each, the 'not stale'
        inventory (line 62/236/124/126/173 — leave alone), and decisions D1–D5. READ THIS FIRST."
  critical: "§3 (the three current→target edits, copy-paste-ready) and §4 (the do-NOT-do scope list) are
             the implementation spec."

- file: docs/how-it-works.md
  why: "THE edit target. The decompose section spans ~lines 55-128. The three spots: line ~91 (diagram
        gate label), line ~111 (Start-of-run freeze paragraph), line ~117 (Arbiter leftover reconciliation
        paragraph). `git status --porcelain` appears EXACTLY ONCE in this file — at line 117 (the stale gate)."
  pattern: "Plain-English bullets with moderate code-symbol use (T_start, tree[i], diff(tree[i-1], tree[i]),
            write-tree, commit-tree). Each 'Key design point' is a bold-led 1-3 sentence paragraph. Match
            that voice/density in the rewrite."
  gotcha: "The diagram is an ASCII code fence (```text). Preserve alignment when changing the gate label —
           the `──yes──▶ done` and `│ no / ▼` branches must stay aligned with the new label width."

# Read-only cross-refs (do NOT edit)
- file: docs/cli.md
  why: "READ-ONLY scope check. The contract forbids touching it. Its decompose/arbiter flag surface
        (--commits, --single, etc.) is unaffected by the freeze-safe rewrite (no new flags)."
- file: docs/configuration.md
  why: "READ-ONLY scope check. The contract forbids touching it. No new config keys come from FR-M1d."

# External references
- url: https://git-scm.com/docs/git-diff#_named_output
  why: "(context) `git diff --name-only` is the frozen-leftover gate basis (DiffTreeNames wraps
        `git diff --name-only treeA treeB`). Confirms it compares two tree SHAs — not the working tree."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── docs/
    ├── how-it-works.md   # EDIT TARGET — 3 edits in the decompose section (lines ~91, ~111, ~117)
    ├── cli.md            # READ-ONLY (do NOT touch — contract)
    ├── configuration.md  # READ-ONLY (do NOT touch — contract)
    └── providers.md      # unaffected (no arbiter narrative)
# (no code, no tests touched — docs-only)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only one existing file modified — no new files)
    docs/how-it-works.md   # arbiter narrative rewritten to the freeze-safe behavior (FR-M1d)
```

| Path | Action | Responsibility |
|---|---|---|
| `docs/how-it-works.md` | MODIFY | 3 edits: rewrite the arbiter paragraph (frozen gate/diff/staging); refine the Start-of-run freeze clause (gate+diff+staging); refine the diagram gate label. |

**Explicitly NOT touched**: `docs/cli.md`, `docs/configuration.md`, `docs/providers.md` (contract:
no new flags/keys; those files are unaffected), any Go source / tests (S1 owns the code; this is the
Mode A doc ride), `README.md` (P3.M1.T1.S1 owns the README changeset sync), `PRD.md`, `tasks.json`,
`prd_snapshot.md`, `plan/*`.

### Known Gotchas of our codebase & toolchain

```markdown
<!-- CRITICAL (G1 — the stale gate is `git status --porcelain`, and it appears EXACTLY ONCE): the line-117
     paragraph is the ONLY place in how-it-works.md that gates the arbiter on live status. Replacing it with
     the frozen-leftover gate removes the doc/code drift. After the edit, `grep porcelain docs/how-it-works.md`
     must return NOTHING. (If it returns a hit, the rewrite left the stale phrase or added a new one.) -->

<!-- CRITICAL (G2 — write PRESENT tense; S1 is the behavior): Mode A rides with the work. By the time this
     edit lands, S1's freeze-safe arbiter IS the live behavior. Do NOT write "will gate on" or "once S1 lands"
     — write "gates on" / "runs iff". Past-tense/conditional wording would re-introduce drift the next time
     someone reads it. -->

<!-- GOTCHA (G3 — preserve the diagram's ASCII alignment): the pipeline diagram is a ```text code fence.
     Changing the gate label "git status clean?" → "frozen leftover empty?" shifts the label width by -1 char
     (19 → 22 chars... actually "git status clean?" = 17, "frozen leftover empty?" = 22). Re-align the
     `──yes──▶ done` arrow and the `│ no / ▼` branch under the new label so the box art stays rectangular.
     The "leftover diff" arrow label into the arbiter box is UNCHANGED. -->

<!-- GOTCHA (G4 — line 111's clause must cover gate+diff+staging, not just staging): FR-M1d names three
     freeze surfaces. The current "the arbiter's leftover staging" names only one (staging). Refine to
     "the arbiter (its gate, its diff, and its leftover staging)" and generalize "stage content drawn strictly
     from T_start" → "draw strictly from T_start" so the gate/diff reads are covered too. -->

<!-- GOTCHA (G5 — do NOT over-specify; how-it-works.md is the plain-English doc): do NOT name OverlayTreePaths,
     do NOT cite FR numbers in the prose (the existing bullets don't), do NOT paste §13.6.5 verbatim. Keep the
     existing voice: concise bold-led paragraphs, moderate symbols (T_start, tree[j], diff-names, TreeDiff).
     The primitive mechanics live in the PRD / the code; this doc states the USER-VISIBLE safety property. -->

<!-- GOTCHA (G6 — leave the accurate passages alone): the four-roles table (line 62), the format-modes paragraph
     (line 236), the Safety bullets (line 124/126 — "concurrent edits never enter any commit" is now MORE true),
     and the lock no-op-fast-path paragraph (line 173) are NOT stale. Editing them is scope creep. Only the three
     spots in §research §2 are stale/imprecise. -->

<!-- GOTCHA (G7 — scope fence: cli.md + configuration.md are READ-ONLY): the contract explicitly forbids
     touching them. FR-M1d adds no flags and no config keys, so those files need no change. `git diff --stat`
     must show ONLY docs/how-it-works.md. -->
```

## Implementation Blueprint

### Data models and structure

None. This is a prose edit to one Markdown file. The "model" is the three current→target passages below.

### The three edits (exact — current → target)

**Edit 1 — rewrite the "Arbiter leftover reconciliation" paragraph (~line 117).**

Current:
```markdown
**Arbiter leftover reconciliation.** After all N concepts are committed, if `git status --porcelain` shows remaining changes, the arbiter decides whether they belong to an existing commit (amend) or warrant a new (N+1)th commit.
```

Target:
```markdown
**Arbiter leftover reconciliation.** After all N concepts are committed, stagecoach computes the **frozen leftover** = `diff-names(tipTree, T_start)` — the `T_start` content no stager claimed (`tipTree` is the last committed tree) — and runs the arbiter **iff it is non-empty**. The live working tree is never consulted for the gate (not `git status --porcelain`), so a file written after `T_start` was captured cannot trigger the arbiter or enter any arbiter commit. Given `TreeDiff(tipTree, T_start)`, the arbiter decides whether the leftovers belong to an existing commit (a plumbing amend that rebuilds the chain from the frozen per-concept `tree[j]` and `T_start`) or warrant a new (N+1)th commit (committing `T_start` directly); stagecoach performs all git from frozen trees, then syncs the index to `T_start`, and the arbiter only decides.
```

**Edit 2 — refine the "Start-of-run freeze (T_start)" paragraph's arbiter clause (~line 111).**

Current clause (within the longer paragraph):
```markdown
... every stager, the arbiter's leftover staging, and the one-file/single shortcuts stage content drawn strictly from T_start. ...
```

Target clause:
```markdown
... every stager, the arbiter (its gate, its diff, and its leftover staging), and the one-file/single shortcuts draw strictly from T_start. ...
```

**Edit 3 — refine the pipeline-diagram gate label (~line 91).**

Current:
```text
         git status clean? ──yes──▶ done
                  │ no
                  ▼
```

Target (re-aligned so the arrow + branch sit under the new label):
```text
     frozen leftover empty? ──yes──▶ done
                  │ no
                  ▼
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/how-it-works.md — Edit 1 (rewrite the arbiter paragraph, ~line 117)
  - FILE: docs/how-it-works.md
  - LOCATE the "Arbiter leftover reconciliation" paragraph (the one starting "**Arbiter leftover
    reconciliation.** After all N concepts are committed, if `git status --porcelain` ...").
  - REPLACE the entire paragraph with the Edit-1 target text above.
  - VERIFY after: `grep -n 'git status --porcelain' docs/how-it-works.md` → NO output (the stale gate is gone).
  - DO NOT: touch the four-roles table, the diagram, the Start-of-run freeze paragraph, or any other paragraph.

Task 2: EDIT docs/how-it-works.md — Edit 2 (refine the Start-of-run freeze clause, ~line 111)
  - LOCATE the "Start-of-run freeze (T_start)." paragraph. Find the clause "every stager, the arbiter's
    leftover staging, and the one-file/single shortcuts stage content drawn strictly from T_start."
  - REPLACE that clause with the Edit-2 target clause above (names gate+diff+staging; "draw strictly from").
  - KEEP the rest of the paragraph (T_start capture, planner partitions T_start's diff, file-after-T_start
    invisible) byte-identical.
  - DO NOT: change "T_start" spelling, add FR citations, or alter the paragraph's other sentences.

Task 3: EDIT docs/how-it-works.md — Edit 3 (diagram gate label, ~line 91)
  - LOCATE the ```text pipeline diagram; the gate box "git status clean? ──yes──▶ done".
  - REPLACE the label "git status clean?" with "frozen leftover empty?" and re-align the `──yes──▶ done`
    arrow and the `│ no / ▼` branch under the new label (gotcha G3).
  - KEEP the rest of the diagram (the loop, the arbiter box, the "leftover diff" arrow label) unchanged.
  - DO NOT: alter the diagram's box-drawing characters elsewhere or the "stagecoach does all git" caption.

Task 4: VALIDATE (editorial + scope — no compile/test for docs)
  - RUN: grep -n 'git status --porcelain' docs/how-it-works.md            → expect NO output
  - RUN: grep -n 'frozen leftover' docs/how-it-works.md                    → expect ≥2 hits (paragraph + diagram label)
  - RUN: grep -n 'diff-names(tipTree, T_start)' docs/how-it-works.md       → expect 1 hit (the new paragraph)
  - RUN: grep -n 'its gate, its diff, and its leftover staging' docs/how-it-works.md → expect 1 hit (Edit 2)
  - RUN: git diff --stat -- docs/                                           → expect ONLY docs/how-it-works.md
  - RUN: git diff --stat -- docs/cli.md docs/configuration.md              → expect EMPTY (untouched)
  - RUN: git diff --stat -- internal/ pkg/ cmd/ README.md                  → expect EMPTY (no code/docs-elsewhere)
  - READ-THROUGH: the decompose section (~lines 85-128) end-to-end once — confirm the diagram, the
    Start-of-run freeze bullet, and the Arbiter paragraph are mutually consistent (all say frozen gate).
```

### Implementation Patterns & Key Details

```markdown
<!-- === Why the paragraph rewrite leads with the gate (the freeze property is the news) ===
     The user-facing safety property is "a concurrent change cannot contaminate an arbiter commit." That
     follows DIRECTLY from "the gate is the frozen leftover, not git status." So the rewrite states the
     gate first, then the consequence ("so a file written after T_start ... cannot trigger ... or enter").
     The diff/resolution/index-sync details are secondary specificity, kept to one trailing sentence. -->

<!-- === Why the diagram label must change too (consistency, not scope creep) ===
     If the paragraph says "frozen leftover" but the diagram says "git status clean?", the doc contradicts
     itself. The diagram is part of the SAME arbiter narrative; its gate label must track the paragraph.
     This is a one-label edit (no structural diagram change) — the minimal consistency fix. -->

<!-- === Why line 111 needs "gate, diff, and staging" (FR-M1d's three surfaces) ===
     FR-M1d: "its gate, the diff it is shown, and the trees it commits are all derived from T_start and
     tipTree." The current "the arbiter's leftover staging" names only the third. A reader who matches
     "staging" against the rewritten paragraph's "gate" + "diff" would notice the gap. Naming all three
     makes the bullet self-consistent and aligned with the PRD. -->

<!-- === Voice match (how-it-works.md vs PRD §13.6.5) ===
     how-it-works.md is plainer and shorter than the PRD. It uses `T_start`/`tree[i]`/`diff(...)` but NOT
     `OverlayTreePaths` or FR numbers. The rewrite mirrors that: names the gate (`diff-names(tipTree, T_start)`),
     the diff (`TreeDiff(tipTree, T_start)`), and the resolution ("rebuilds the chain from frozen tree[j]
     and T_start" / "committing T_start directly") WITHOUT the primitive's mechanics or FR citations. -->
```

### Integration Points

```yaml
DOCS (docs/how-it-works.md):
  - line ~91:  diagram gate label "git status clean?" → "frozen leftover empty?" (re-aligned)
  - line ~111: "Start-of-run freeze (T_start)" arbiter clause → names gate+diff+staging
  - line ~117: "Arbiter leftover reconciliation" paragraph → frozen gate/diff/staging + index sync

NO-TOUCH (explicitly — contract or sibling ownership):
  - docs/cli.md, docs/configuration.md, docs/providers.md   # contract: no new flags/keys; unaffected
  - README.md                                                # P3.M1.T1.S1 (changeset sync)
  - internal/*, pkg/*, cmd/* (any .go)                       # S1 owns the code; this is docs-only
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

CROSS-DOC CONSISTENCY (informational — verify, do not edit):
  - PRD §13.6.5 / FR-M1d/M9/M10 are the authoritative source the rewrite mirrors (plain-English condensation).
  - docs/cli.md's decompose flags (--commits, --single, --max-commits) are unchanged by FR-M1d (no new flags).
  - docs/configuration.md's decompose keys are unchanged by FR-M1d (no new config keys).

DOWNSTREAM HOOKS (informational):
  - P3.M1.T1.S2 (holistic how-it-works.md reconciliation) will review the whole decompose section later;
    this edit's three spots must be consistent by then (the edit makes them so).
```

## Validation Loop

> Docs-only edit — no compile/test/lint gates apply. Validation is editorial (the stale phrase is gone,
> the new phrasing is present, the section is internally consistent) + scope (`git diff --stat`).

### Level 1: Markdown Sanity (no broken structure)

```bash
cd /home/dustin/projects/stagecoach

# The edits are prose within existing paragraphs + one ASCII label inside a ```text fence. Confirm no
# accidental heading/structure breakage:
grep -n '^#' docs/how-it-works.md | head   # the heading list is unchanged (no new/removed headings)
# Expected: the same heading sequence as before the edit (eyeball: no '#' mid-paragraph from a stray edit).

# Diagram fence still balanced (the ```text ... ``` block around the pipeline):
awk '/^```text/{f=1} f{print} /^```$/{if(f){f=0; print "---END---"}}' docs/how-it-works.md | head -40
# Expected: the diagram renders with the new "frozen leftover empty?" label and a single closing fence.
```

### Level 2: Content Assertions (the stale gate is gone; the freeze narrative is present)

```bash
cd /home/dustin/projects/stagecoach

# THE headline check: the stale live-status gate is GONE from the doc.
grep -n 'git status --porcelain' docs/how-it-works.md
# Expected: NO output. (Before the edit this returned exactly one hit at ~line 117.)

# The frozen-gate phrasing is present (paragraph + diagram label).
grep -n 'frozen leftover' docs/how-it-works.md
# Expected: ≥2 hits — the rewritten paragraph ("computes the **frozen leftover**") + the diagram label
#           ("frozen leftover empty?").

# The three freeze surfaces are named in the Start-of-run freeze bullet.
grep -n 'its gate, its diff, and its leftover staging' docs/how-it-works.md
# Expected: exactly 1 hit (Edit 2).

# The frozen diff + tree-only resolution are stated.
grep -n 'diff-names(tipTree, T_start)\|TreeDiff(tipTree, T_start)' docs/how-it-works.md
# Expected: ≥1 hit (the rewritten paragraph names both).
```

### Level 3: Scope Discipline (only the one doc changed)

```bash
cd /home/dustin/projects/stagecoach

# ONLY docs/how-it-works.md changed.
git diff --stat -- docs/
# Expected: only docs/how-it-works.md.

# The contract-forbidden docs are untouched.
git diff --stat -- docs/cli.md docs/configuration.md
# Expected: EMPTY.

# No code, no README, no other docs touched.
git diff --stat -- internal/ pkg/ cmd/ README.md docs/providers.md
# Expected: EMPTY.

# Confirm the diff is exactly the 3 intended edits (eyeball the patch).
git diff -- docs/how-it-works.md
# Expected: three changed hunks — (1) the diagram label line, (2) the one clause in the Start-of-run
# freeze paragraph, (3) the Arbiter leftover reconciliation paragraph. Nothing else.
```

### Level 4: Cross-Reference Consistency (the section tells one story)

```bash
cd /home/dustin/projects/stagecoach

# Read the decompose section end-to-end and confirm the gate story is consistent across the three surfaces:
sed -n '85,128p' docs/how-it-works.md
# Expected (eyeball): the diagram gate ("frozen leftover empty?"), the Start-of-run freeze bullet
# ("the arbiter (its gate, its diff, and its leftover staging) ... draw strictly from T_start"), and the
# Arbiter paragraph ("frozen leftover = diff-names(tipTree, T_start) ... not git status --porcelain")
# all agree that the arbiter gate is FROZEN. None references live `git status`.

# Cross-check against the PRD's authoritative phrasing (do NOT edit the PRD — just confirm alignment):
grep -n 'frozen leftover\|diff-names(tipTree, T_start)\|not.*git status' PRD.md | head
# Expected: PRD §13.6.5 / FR-M1d/M9 use the same frozen-gate framing the doc now mirrors.
```

## Final Validation Checklist

### Technical Validation
- [ ] `grep 'git status --porcelain' docs/how-it-works.md` → no output (Level 2).
- [ ] `grep 'frozen leftover' docs/how-it-works.md` → ≥2 hits (paragraph + diagram label).
- [ ] `grep 'diff-names(tipTree, T_start)' docs/how-it-works.md` → 1 hit.
- [ ] `grep 'its gate, its diff, and its leftover staging' docs/how-it-works.md` → 1 hit (Edit 2).
- [ ] Markdown structure intact (no broken headings/fences) — Level 1 eyeball.

### Feature Validation
- [ ] The Arbiter paragraph names the frozen gate, the freeze property (concurrent change can't trigger/enter),
      the frozen diff, and tree-only resolution + index sync to `T_start`.
- [ ] The Start-of-run freeze bullet names the arbiter's gate, diff, AND staging.
- [ ] The diagram gate label reads "frozen leftover empty?" and the arrow/branch are re-aligned.
- [ ] The narrative is in PRESENT tense (no "will"/"once S1 lands" — S1 is the behavior).

### Scope Discipline Validation
- [ ] `git diff --stat -- docs/` shows ONLY `docs/how-it-works.md`.
- [ ] `git diff --stat -- docs/cli.md docs/configuration.md` is EMPTY (contract: do not touch).
- [ ] `git diff --stat -- internal/ pkg/ cmd/ README.md` is EMPTY (no code/README).
- [ ] The diff is exactly 3 hunks (diagram label + line-111 clause + line-117 paragraph).
- [ ] Did NOT edit the four-roles table, format-modes paragraph, Safety bullets, or lock paragraph.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation (editorial)
- [ ] The rewrite matches the existing voice (concise bold-led paragraphs; moderate symbols; no FR citations).
- [ ] No raw `OverlayTreePaths` primitive name in the prose (mechanics live in PRD §13.6.5 / code).
- [ ] The diagram's box-drawing alignment is preserved under the new label width.

---

## Anti-Patterns to Avoid

- ❌ Don't leave `git status --porcelain` in the arbiter paragraph — that IS the stale loophole FR-M1d
  closes; the whole point of the edit is to replace it with the frozen-leftover gate (gotcha G1).
- ❌ Don't write the rewrite in future/conditional tense ("will gate", "once S1 lands"). Mode A rides with
  the work — S1 IS the behavior when this lands. Present tense only (gotcha G2).
- ❌ Don't break the diagram's ASCII alignment when changing the gate label — re-align the `──yes──▶ done`
  arrow and the `│ no / ▼` branch under "frozen leftover empty?" (gotcha G3).
- ❌ Don't leave line 111 naming only "the arbiter's leftover staging." FR-M1d names three surfaces
  (gate, diff, staging) — refine the clause to name all three and generalize "stage content" → "draw"
  (gotcha G4).
- ❌ Don't over-specify — no `OverlayTreePaths`, no FR numbers in the prose, no §13.6.5 verbatim paste.
  how-it-works.md is the plain-English companion; match its voice (gotcha G5).
- ❌ Don't edit the accurate passages (four-roles table line 62, format modes line 236, Safety bullets
  line 124/126, lock paragraph line 173) — none are stale; editing them is scope creep (gotcha G6).
- ❌ Don't touch `docs/cli.md` or `docs/configuration.md` — the contract explicitly forbids it; FR-M1d
  adds no flags and no config keys (gotcha G7).
- ❌ Don't edit code, tests, `README.md`, `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`. This is
  a single-file docs edit.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a minimal, fully-prescribed three-edit prose change to one Markdown file. Every edit
is quoted verbatim (current → target, ready to paste), the line numbers are verified against the live
tree, and the stale phrase (`git status --porcelain`) is confirmed to appear exactly once in the file
(the rewrite removes the only hit). The behavior contract (S1's freeze-safe arbiter) is exceptionally
well-specified in both the PRD (FR-M1d/M9/M10 + §13.6.5) and S1's PRP, so the target phrasing is
unambiguous. The two most likely mistakes — leaving the stale `git status --porcelain` reference (G1)
and writing in future tense (G2) — are front-loaded as CRITICAL gotchas and caught by the deterministic
`grep` gates (Level 2: zero `porcelain` hits; present-tense wording). The one residual uncertainty
(not 10/10) is the ASCII-diagram re-alignment under the new label width (G3) — a cosmetic spacing detail
the implementer must eyeball (Level 1 fence check + Level 4 read-through). Scope is cleanly fenced
(`git diff --stat` proves only `docs/how-it-works.md`; `cli.md`/`configuration.md` explicitly read-only).
No code, no tests, no toolchain — the gates are editorial greps and a scope check, all deterministic.
