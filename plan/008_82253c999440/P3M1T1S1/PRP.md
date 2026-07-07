---
name: "P3.M1.T1.S1 — README.md: surface v2.2 decompose improvements in the feature list (PRD §21.5 marketing surface; §5 hero pitch; §9.14 FR-M1d/M3/M4)"
description: |

  A MODE-B DOCS-ONLY edit to ONE file: `README.md`. Surface the v2.2 multi-commit-decomposition
  improvements (P1 arbiter freeze parity COMPLETE; P2 planner per-file + soft target COMPLETE/IMPLEMENTING)
  in the README feature list. The v2.2 changes are INTERNAL-QUALITY (no new flags/keys — confirmed: P1 and
  P2 add zero CLI flags and zero config keys), so the surfacing is a concise marketing-surface refresh,
  NOT a flag/config reference (those live in docs/cli.md + docs/configuration.md).

  CONTRACT (P3.M1.T1.S1, verbatim from the work item):
    1. RESEARCH NOTE: README.md is the marketing surface (PRD §21.5). The hero pitch (PRD §5) must stay
       intact. The v2.2 improvements are internal-quality (arbiter fully freeze-safe — a concurrent edit
       never enters a commit; planner partitions are per-file and count-guided by a soft target). No new
       flags/keys to document.
    2. INPUT: The implemented v2.2 changes from all of P1.M1.T2/T3 and P2.M1.T1/T2/T3.
    3. LOGIC: Add/refresh the decompose feature blurb in README.md's feature list: the multi-commit
       decomposition now commits strictly from the frozen start-of-run snapshot (a concurrent edit during
       the run can never enter a commit, including across the leftover-reconciliation arbiter), and the
       planner partitions per-file with a soft count target. Keep it concise; do NOT duplicate per-key
       config reference or per-flag CLI reference (those live in docs/). Do NOT alter the hero pitch.
       Mock: none (docs edit).
    4. OUTPUT: README.md feature list reflects v2.2; hero pitch intact.
    5. DOCS: [Mode B] This subtask IS the README changeset-level doc update. Depends on every implementing
       subtask so it runs last and summarizes the whole delta.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `docs/how-it-works.md` — the SIBLING task P3.M1.T1.S2 ("holistic decompose-section reconciliation").
      README links INTO its `#multi-commit-decomposition` anchor; S2 owns the target's content.
    - `docs/cli.md`, `docs/configuration.md` — the authoritative per-flag/per-key reference. The README
      Features row LINKS to them; it must NOT duplicate their content.
    - Any `.go` file / go.mod / go.sum — UNCHANGED (docs-only; zero code impact).
    - PRD.md / tasks.json / prd_snapshot.md — READ-ONLY (never modify).
    - README hero pitch (lines 3-5) + line-6 version tagline — FROZEN / out of scope.

  DELIVERABLES (1 file MODIFIED, 0 new):
    EDIT README.md — (A) ADD a "Multi-commit decomposition" row to the `## Features` capability table
      (currently MISSING the decompose row entirely) as its FIRST data row; (B) LIGHTLY refresh the
      `### Multi-commit decomposition` narrative freeze sentence to be arbiter-inclusive + add a concise
      per-file/soft-target clause.

  SUCCESS: README `## Features` table has a decompose row surfacing both v2.2 improvements (freeze incl.
  arbiter; per-file planner + soft target) with a docs link and NO per-flag/per-key enumeration; the
  narrative freeze clause is arbiter-inclusive; the hero pitch blockquote (lines 3-5) is byte-unchanged;
  no docs/ or .go file is touched; `git status --short` shows ONLY `M README.md`.

---

## Goal

**Feature Goal**: Surface the v2.2 multi-commit-decomposition improvements in README.md's feature list so
the marketing surface reflects the current quality of the decompose pipeline — (1) the start-of-run freeze
is now complete: a file a concurrent process edits during the run can never enter ANY commit, including
across the leftover-reconciliation arbiter (FR-M1d, P1); (2) the planner partitions changes per-file
(FR-M3/M3b, P2) and leans toward a soft count target of `max_commits / 2` (FR-M4, P2). Both are
internal-quality improvements with NO new user-facing flags or config keys, so the surfacing is a concise
blurb + link, not a reference dump.

**Deliverable** (1 file MODIFIED, 0 new): `README.md` with two coordinated edits —
- **Edit A (primary)**: a new "Multi-commit decomposition" row in the `## Features` capability table (the
  table currently omits decompose entirely — the gap). Placed as the FIRST data row (headline v2 capability).
- **Edit B (supporting)**: a light refresh of the `### Multi-commit decomposition` narrative's freeze
  sentence to state the arbiter-inclusive guarantee and add a per-file + soft-target clause.

**Success Definition**:
- The `## Features` table contains a "Multi-commit decomposition" row whose description names BOTH the
  freeze (concurrent edits excluded from every commit, including across the arbiter) AND the per-file
  planner with a soft count target, and links to `docs/how-it-works.md#multi-commit-decomposition` (anchor
  verified to exist).
- The Features row contains NO `--commits` / `--single` / `[role.` / `max_commits` tokens (no per-flag /
  per-key duplication — those live in docs/cli.md + docs/configuration.md + the how-to subsection).
- The `### Multi-commit decomposition` narrative's freeze clause is arbiter-inclusive and mentions per-file
  + soft target; the four-role-pipeline opener, the stager-constraint sentence, and "Stagecoach owns every
  commit via git plumbing" are byte-unchanged.
- The hero pitch blockquote (README lines 3-5) is BYTE-UNCHANGED.
- `git status --short` shows EXACTLY `M README.md` — no docs/ file, no `.go` file, no go.mod.

## User Persona

**Target User**: a developer reading the README to decide whether Stagecoach is safe to run on a busy
working tree — the persona from PRD §7.1 ("the plan-holder") who keeps coding while stagecoach runs. They
skim the Features table and the decompose blurb.

**Use Case**: the user has a messy working tree, is running stagecoach auto-decompose, and is ALSO running
an editor / a second coding agent / a formatter that writes files mid-run. They want to know: will that
concurrent edit get swept into a commit? (v2.2 answer: no — never, including the arbiter's reconciliation.)

**Pain Points Addressed**: the current README understates the freeze (it says mid-run edits are excluded
"from every commit" but doesn't note the arbiter loophole that v2.2 closed) and doesn't mention the
per-file planner or the soft target. A reader cannot tell from the feature list that decompose is
concurrency-hardened end-to-end. This edit makes that visible in the one place buyers look.

## Why

- **Closes the README half of the v2.2 changeset-level doc sync (Mode B).** system_context.md line 89:
  "README surfaces v2.2 decompose improvements (arbiter fully freeze-safe; planner per-file + …)". The
  implementing subtasks (P1.M1.* arbiter freeze parity; P2.M1.* planner files + soft target) are COMPLETE
  or IMPLEMENTING; this task runs LAST and summarizes the whole delta on the marketing surface (PRD §21.5).
- **The freeze guarantee is the headline safety property, and v2.2 completed it.** v2.0-v2.1 had a real
  loophole: the arbiter gate read live `git status --porcelain` and the resolution ran `git add -A` against
  the live tree, so a concurrent change during the planner call could silently land in an arbiter commit.
  FR-M1d (P1) closes it. The README's safety FAQ ("Will it corrupt my repo?" / the snapshot narrative)
  trades on the never-corrupt / never-sweep promise — the feature list should reflect that the promise now
  holds across the whole decompose pipeline, not just the stager loop.
- **The Features table is missing decompose entirely.** Every other v2.x capability (payload exclusions,
  message shaping, hook mode, integrations, --edit/--push, discovery) has a row; the flagship v2 feature
  does not. Adding it surfaces both the v2.2 improvements AND fills the pre-existing gap.
- **Concise by design.** The work item explicitly forbids duplicating the per-flag CLI reference or the
  per-key config reference (those live in docs/). So the surfacing is a one-row blurb + a narrative clause,
  both linking into the authoritative docs — not a re-statement of `--commits`/`--single`/`[role.planner]`.

## What

A pure markdown edit to `README.md`. Two changes, both within the file:

### Edit A — new row in the `## Features` capability table

INSERT as the FIRST data row (right after the `| Capability | Description |\n|---|---|\n` header, before
the existing `| Payload exclusions | ...` row). The row has exactly two cells (`| Capability | Description |`),
matching the table schema. The description is one concise sentence covering both v2.2 improvements, with a
docs link in the established `([label](docs/...#anchor) · [label](docs/...))` format. NO flag/config tokens.

### Edit B — refresh the `### Multi-commit decomposition` narrative freeze sentence

REPLACE the single sentence beginning "A start-of-run freeze (T_start) captures your entire change set up
front, so files you change mid-run are excluded from every commit — the run only ever commits what existed
when it started." with a refreshed version that (a) makes the freeze arbiter-inclusive and (b) adds a
concise per-file + soft-target clause. The preceding sentence (the four-role-pipeline opener) and the
following sentences (the stager-constraint details; "Stagecoach owns every commit via git plumbing") are
byte-unchanged.

### Success Criteria

- [ ] The `## Features` table has a "Multi-commit decomposition" row (first data row) whose Description
      names BOTH (a) the start-of-run freeze excluding concurrent edits from every commit (arbiter-
      inclusive) AND (b) the per-file planner leaning toward a soft count target, and links to
      `docs/how-it-works.md#multi-commit-decomposition`.
- [ ] The Features row contains NONE of: `--commits`, `--single`, `[role.`, `max_commits`, `--reasoning`
      (no per-flag / per-key / per-role duplication).
- [ ] The `### Multi-commit decomposition` narrative's freeze clause is arbiter-inclusive ("...across the
      arbiter too" or equivalent) and adds a per-file + soft-target clause; the four-role-pipeline opener,
      the stager-constraint sentence, and "Stagecoach owns every commit via git plumbing" are unchanged.
- [ ] The hero pitch blockquote (README lines 3-5) is BYTE-UNCHANGED (verify by diff).
- [ ] `git status --short` shows EXACTLY `M README.md` — no docs/ file, no `.go` file, no go.mod/go.sum.

## All Needed Context

### Context Completeness Check

_Pass._ A technical writer (or developer) with no prior repo knowledge can implement this from: the exact
two edits with verbatim replacement text (§"Implementation Blueprint" — copy/paste), the gap analysis
(findings §1 — the Features table is missing the decompose row; the narrative understates the freeze), the
exact v2.2 changeset to surface (findings §2 — arbiter freeze parity FR-M1d, per-file planner FR-M3, soft
target FR-M4; NO new flags/keys), the link-format precedent (the existing Features rows), the validated
link target (`docs/how-it-works.md#multi-commit-decomposition` exists, line 47), the scope fence
(findings §5 — README only; how-it-works.md is the sibling S2; hero pitch frozen), and the validation
approach (findings §4 — docs-only, no build/test; markdown table integrity + link + grep checks). No Go /
git / freeze-implementation knowledge required — the code is already COMPLETE/IMPLEMENTING; this task only
documents it on the marketing surface.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE findings (gap analysis + exact edits + scope fence)
- docfile: plan/008_82253c999440/P3M1T1S1/research/findings.md
  why: §1 the FOUR README surfaces mentioning decompose + the GAP (Features table has NO decompose row);
       §2 the EXACT v2.2 changeset to surface (FR-M1d arbiter freeze parity; FR-M3 per-file planner; FR-M4
       soft target; NO new flags/keys); §3 the TWO edits (A: add Features row first; B: refresh narrative
       freeze sentence) with what NOT to touch; §4 the docs-only validation approach; §5 the scope fence.
  critical: §1 (the Features table is missing decompose — that's the gap); §2 (NO new flags/keys — do NOT
            enumerate --commits/--single/[role.] in the Features row); §5 (README ONLY — how-it-works.md is
            the sibling S2 task; hero pitch lines 3-5 FROZEN).

# MUST READ — the FILE TO EDIT
- file: README.md   (EDIT — the ONLY file this task touches)
  section: hero pitch blockquote (L3-5 — FROZEN, byte-compare before/after); L6 "v2.1 adds..." tagline
           (OUT OF SCOPE — leave it); `## Features` table (L59-71 — INSERT the decompose row as the FIRST
           data row, right after the `|---|---|` separator before `| Payload exclusions |`); `### Multi-
           commit decomposition` narrative (L142 — REPLACE the one freeze sentence; keep the rest).
  why: the two edits live here. The Features table is the "feature list" named in the work item; it is
       missing the decompose row (findings §1). The narrative is the detailed blurb whose freeze claim
       v2.2 strengthens (arbiter parity) and extends (per-file + soft target).
  pattern: mirror the EXISTING Features-row link format exactly — `([label](docs/...#anchor))` or
           `([label](docs/...) · [label](docs/...))`. Match the 2-cell `| Capability | Description |`
           schema. The narrative uses mid-sentence backticks for T_start / git commands — keep that style.
  gotcha: do NOT touch the hero pitch blockquote (L3-5), the L6 tagline, the comparison table (L32), or the
          code examples in the decompose subsection (those --commits/--single/[role.planner] examples ARE
          the per-flag reference and stay). Do NOT edit any docs/ or .go file.

# MUST READ — the validated link target (READ-ONLY — owned by the sibling S2 task)
- file: docs/how-it-works.md   (READ-ONLY — confirms the anchor; do NOT edit)
  section: L47 `## Multi-commit decomposition` → GitHub anchor `#multi-commit-decomposition`. The Features
           row links here. (The sibling P3.M1.T1.S2 owns this file's decompose-section reconciliation.)
  why: confirms the Features-row link target resolves (no broken link). Do NOT edit this file.

# MUST READ — PRD §5 (hero pitch — FROZEN) + §9.14 (the v2.2 FRs being surfaced) + §21.5 (README = marketing surface)
- docfile: PRD.md
  section: §5 (the UVP + the verbatim hero-pitch candidate — the hero pitch must stay byte-intact); §9.14
           FR-M1d (arbiter freeze parity — "a file created or modified after T_start ... cannot enter any
           arbiter commit"), FR-M3/FR-M3b (planner per-file partition), FR-M4 (soft target = max_commits/2);
           §21.5 (README is the marketing surface).
  why: §5 defines what the hero pitch IS (so "do not alter" is unambiguous); §9.14 is the authoritative
       source for the v2.2 guarantees the blurb must reflect (exact wording for "concurrent edit never
       enters a commit, including across the arbiter" + "per-file" + "soft count target").
  critical: §9.14 FR-M1d — the arbiter-inclusive freeze is the v2.2 HEADLINE; the blurb MUST surface it
            (the pre-v2.2 loophole is exactly what makes this a changeset-level doc update).

# MUST READ — system_context (confirms this task's charter)
- docfile: plan/008_82253c999440/docs/architecture/system_context.md
  section: L88-89 — "T1 — README.md + docs/how-it-works.md cross-cutting sweep ... README surfaces v2.2
           decompose improvements (arbiter fully freeze-safe; planner per-file + ...)".
  why: confirms README surfacing of (arbiter freeze-safe + planner per-file) is exactly this task.

# READ — the v2.2 implementing PRPs (for the exact guarantee wording; do NOT restate their internals)
- docfile: plan/008_82253c999440/P1M1T2S1/PRP.md   (arbiter freeze parity — FR-M1d — COMPLETE)
  why: the authoritative description of the arbiter-inclusive freeze (gate/diff/trees all from T_start +
       tipTree, never live). The README blurb summarizes this at the marketing level — no internals.
- docfile: plan/008_82253c999440/P2M1T2S1/PRP.md   (planner per-file + soft target — COMPLETE)
  why: confirms FR-M3 (per-file partition) + FR-M4 (soft target max_commits/2, guidance not enforcement).
- docfile: plan/008_82253c999440/P2M1T3S1/PRP.md   (stager files block — IMPLEMENTING in parallel)
  why: confirms the stager files block is INTERNAL prompt construction (no user-facing surface) — it does
       NOT need a README mention. Its narrative is the sibling S2's how-it-works.md concern. Do NOT mention
       stager prompt internals in README.
```

### Current Codebase tree (relevant slice)

```bash
README.md                          # EDIT (the ONLY file this task touches):
                                   #   (A) ADD a "Multi-commit decomposition" row to the `## Features` table
                                   #       (first data row — the table currently omits decompose);
                                   #   (B) refresh the `### Multi-commit decomposition` narrative freeze
                                   #       sentence (arbiter-inclusive + per-file/soft-target clause).
docs/how-it-works.md               # READ-ONLY (sibling P3.M1.T1.S2 owns it; L47 anchor is the link target).
docs/cli.md, docs/configuration.md # READ-ONLY (authoritative per-flag/per-key reference; README links in).
(go.mod, go.sum, *.go)             # UNCHANGED (docs-only task; zero code impact).
```

### Desired Codebase tree with files to be added/modified

No NEW files. One file MODIFIED:

```bash
README.md   # MODIFY — two edits:
            #   Edit A: new first data row in the `## Features` capability table:
            #     | Multi-commit decomposition | <concise v2.2 blurb> ([how it works](docs/how-it-works.md#multi-commit-decomposition) · [flags](docs/cli.md)) |
            #   Edit B: refreshed freeze sentence in the `### Multi-commit decomposition` narrative
            #     (arbiter-inclusive + per-file/soft-target clause; surrounding sentences unchanged).
# NO other file changes. NO docs/ edits. NO .go edits. NO go.mod/go.sum edits.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the Features table is MISSING the decompose row — findings §1): the `## Features` capability
     table (README L59-71) lists Payload exclusions, Payload optimization, Message shaping, Git hook mode,
     Tool integrations, --edit/--push, Discovery — but NOT decompose. The work item's "Add/refresh the
     decompose feature blurb in README.md's feature list" resolves to: ADD the missing row (this is an
     "Add", the work item's "Add/refresh" covers both). Place it as the FIRST data row (headline v2 cap). -->

<!-- CRITICAL (NO per-flag / per-key duplication — work item + findings §2/§3): the Features row description
     must NOT contain --commits, --single, [role., max_commits, --reasoning, or any config key. Those live
     in docs/cli.md + docs/configuration.md AND in the `### Multi-commit decomposition` how-to subsection's
     code blocks (which STAY). The Features row is a concise blurb + link, not a reference. -->

<!-- CRITICAL (hero pitch FROZEN — work item): the blockquote at README L3-5 ("> **Stagecoach writes your
     commit messages...**") must be BYTE-UNCHANGED. Byte-compare before/after (diff L3-5). The L6 "v2.1
     adds..." tagline is OUT OF SCOPE (not the feature list; not requested) — leave it, even though it
     names v2.1 not v2.2. -->

<!-- GOTCHA (markdown table integrity): the new row must have EXACTLY two cells delimited by `|`,
     matching `| Capability | Description |`. A missing trailing `|` or an extra `|` inside the
     description (e.g. a literal pipe in prose) breaks the table render. The description contains no pipe
     chars; the link syntax `([label](url))` is pipe-safe. -->

<!-- GOTCHA (link format precedent): existing Features rows use `([docs](docs/...#anchor))` or
     `([how it works](docs/...) · [knobs](docs/...))`. Match that exactly (lowercase labels; `·` middle dot
     with spaces for multi-link). The anchor `#multi-commit-decomposition` is the GitHub-slug of the L47
     `## Multi-commit decomposition` heading (verified). -->

<!-- GOTCHA (Edit B is SURGICAL — findings §3): replace ONLY the one freeze sentence in the L142 narrative.
     Keep the four-role-pipeline opener ("...planner → stager → message → arbiter). Each concept becomes
     its own commit.") and the FOLLOWING stager-constraint sentence ("The stager is constrained to staging
     operations: claude via ... git plumbing.") byte-unchanged — they are accurate and out of scope. A
     common mistake is rewriting the whole paragraph; don't. -->

<!-- GOTCHA (scope fence — findings §5): touch ONLY README.md. docs/how-it-works.md is the sibling S2 task;
     docs/cli.md + docs/configuration.md are the authoritative reference (link, don't duplicate). If `git
     status --short` shows anything beyond `M README.md`, you've gone out of scope. -->

<!-- GOTCHA (v2.2 is INTERNAL-quality — no flag/key to document): P1 and P2 add ZERO CLI flags and ZERO
     config keys. So the README edit is purely a quality/correctness surfacing — there is nothing new to
     "document" in the reference sense. Do not invent flags/keys. -->
```

## Implementation Blueprint

### Edit A — the new `## Features` table row (EXACT text — insert as the FIRST data row)

Insert immediately after the table separator line `|---|---|` and before the existing
`| Payload exclusions | ...` row (README ~L63). One row, two cells, matching the schema:

```markdown
| Multi-commit decomposition | Auto-decompose a dirty, un-staged tree into N logical commits (planner → stager → message → arbiter). A start-of-run freeze means a concurrent edit during the run can never enter a commit — including across the leftover-reconciliation arbiter; the planner partitions per file and leans toward a soft count target ([how it works](docs/how-it-works.md#multi-commit-decomposition) · [flags](docs/cli.md)). |
```

> Rationale per cell:
> - **Capability**: "Multi-commit decomposition" — matches the comparison-table + hero-pitch term.
> - **Description**: one sentence covering BOTH v2.2 headlines (freeze incl. arbiter; per-file + soft
>   target), preceded by the one-line "what it is" (auto-decompose dirty tree → N commits, four-role
>   pipeline). NO `--commits`/`--single`/`[role.]`/`max_commits` tokens (per-flag/per-key reference lives
>   in docs/cli.md + docs/configuration.md). Two doc links in the established `·`-joined format.
>
> If a shorter row is preferred, this alternative keeps both v2.2 headlines and drops the pipeline list:
> ```markdown
> | Multi-commit decomposition | Auto-decompose a dirty tree into N logical commits. A start-of-run freeze excludes concurrent edits from every commit — including across the arbiter; the planner partitions per file toward a soft count target ([how it works](docs/how-it-works.md#multi-commit-decomposition)). |
> ```
> Either is acceptable; the primary (longer) row is preferred because it carries the four-role-pipeline
> term that appears elsewhere in the README, keeping the surface consistent.

### Edit B — refresh the `### Multi-commit decomposition` narrative freeze sentence (EXACT text)

Locate the sentence in the L142 paragraph (it is the THIRD sentence). Replace ONLY this sentence:

> OLD: `A start-of-run freeze (T_start) captures your entire change set up front, so files you change mid-run are excluded from every commit — the run only ever commits what existed when it started.`

with:

> NEW: `A start-of-run freeze (T_start) captures your entire change set up front, so files you change mid-run are excluded from every commit — the run only ever commits what existed when it started, and that holds across the leftover-reconciliation arbiter too (a concurrent edit can never sneak into a commit). The planner partitions changes per file and leans toward a soft count target, so a typical mixed tree lands at or below half the cap.`

> Rationale:
> - The first clause (T_start captures the change set; mid-run edits excluded) is preserved verbatim, then
>   EXTENDED with the arbiter-inclusive guarantee (FR-M1d — the v2.2 headline).
> - The NEW second sentence adds the per-file planner (FR-M3) + soft target (FR-M4) in one concise clause.
> - The preceding sentence (the four-role-pipeline opener "...planner → stager → message → arbiter). Each
>   concept becomes its own commit.") and the following stager-constraint sentence ("The stager is
>   constrained to staging operations: ... git plumbing.") are BYTE-UNCHANGED.

### Implementation Tasks (ordered)

```yaml
Task 1: EDIT README.md — Edit A (add the Features table row)
  - LOCATE the `## Features` table header: `| Capability | Description |` followed by `|---|---|` (README
    ~L61-63). The first data row is currently `| Payload exclusions | ...` (~L65).
  - INSERT the new "Multi-commit decomposition" row (§"Edit A" primary text) as the FIRST data row —
    immediately after `|---|---|`, before `| Payload exclusions |`.
  - VERIFY: the row has exactly two `|`-delimited cells; no literal `|` inside the description; the two
    markdown links are well-formed; the anchor `#multi-commit-decomposition` matches the how-it-works.md
    L47 heading slug.
  - VERIFY: the row contains NONE of `--commits`, `--single`, `[role.`, `max_commits`, `--reasoning`.
  - PLACEMENT: first data row of the Features table.

Task 2: EDIT README.md — Edit B (refresh the narrative freeze sentence)
  - LOCATE the `### Multi-commit decomposition` paragraph (README L142).
  - REPLACE the single OLD freeze sentence (§"Edit B" OLD) with the NEW two-sentence version (§"Edit B"
    NEW). Use an exact-text edit (the OLD sentence is unique in the file).
  - PRESERVE byte-for-byte: the preceding "With a dirty working tree ... planner → stager → message →
    arbiter). Each concept becomes its own commit." opener AND the following "The stager is constrained to
    staging operations: ... Either way, Stagecoach owns every commit via git plumbing." sentence.
  - VERIFY: the em-dash "—" and the arrows "→" in the preserved opener are intact (UTF-8); the backticks
    around `T_start` / git commands are intact.

Task 3: VERIFY (docs-only validation — findings §4)
  - Hero-pitch byte-compare: `git diff README.md` — the ONLY changes are the new Features row + the
    refreshed sentence; README L3-5 (the blockquote) show NO diff hunks.
  - Markdown table render-check: the Features table still renders as a table (the new row is a row, not
    loose text). Eyeball the rendered table.
  - Link target: confirm `docs/how-it-works.md` L47 is `## Multi-commit decomposition` (anchor resolves).
  - No-flag/no-key duplication: `grep -nE '\-\-commits|\-\-single|\[role\.|max_commits|\-\-reasoning'`
    inside the new Features row ⇒ no matches (the how-to subsection's code blocks MAY still match — those
    are fine and stay).
  - v2.2 surfaced: the Features row mentions "arbiter" (or "across the arbiter") and "soft count target"
    (or "soft target"); the narrative mentions "across the leftover-reconciliation arbiter" and
    "soft count target".
  - Scope fence: `git status --short` ⇒ EXACTLY `M README.md` (no docs/ file, no .go file, no go.mod).
```

### Integration Points

```yaml
DATABASE: none.
CONFIG:  none (no new flags/keys — v2.2 is internal-quality; P1/P2 add zero CLI flags and zero config keys).
ROUTES:  none.
DOCS:
  - This task IS the README changeset-level doc update (Mode B). It depends on every implementing
    subtask (P1.* arbiter freeze parity COMPLETE; P2.* planner files + soft target COMPLETE/IMPLEMENTING)
    and runs LAST to summarize the whole v2.2 delta on the marketing surface (PRD §21.5).
  - The README Features row LINKS INTO docs/how-it-works.md#multi-commit-decomposition (the sibling
    P3.M1.T1.S2 task owns that target's content) and docs/cli.md (the per-flag reference). It does NOT
    duplicate either.
BUILD/TEST: none — docs-only edit; zero Go code change. `go build`/`go test` are unaffected (do NOT need
  to run for this task, though a no-op `go test ./...` confirms nothing else moved).
```

## Validation Loop

### Level 1: Markdown Integrity (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

# 1a. Hero pitch BYTE-UNCHANGED: the diff must show NO hunk on the blockquote (README L3-5).
git diff README.md                       # eyeball: only the new Features row + the refreshed sentence.

# 1b. Features table still a table: the new row has 2 cells, matching the header.
grep -n "^| Multi-commit decomposition |" README.md     # the new row exists, starts with `| `, ends with ` |`.
grep -c "^| " README.md                                  # row count rose by exactly 1 vs. the baseline.

# 1c. No per-flag / per-key leakage in the new Features row (the how-to code blocks MAY still match — OK).
grep -nE '\-\-commits|\-\-single|\[role\.|max_commits|\-\-reasoning' README.md
# Expected: matches ONLY inside the `### Multi-commit decomposition` code blocks (~L150-170), NOT in the
# new Features row (~L63). If the Features row matches, remove the token (it's reference duplication).

# Expected: zero unintended changes. Fix any before proceeding.
```

### Level 2: Link + Content Validation

```bash
# 2a. The Features-row link target EXISTS (the sibling S2 owns the file; this task only links in).
grep -n "^## Multi-commit decomposition" docs/how-it-works.md    # L47 ⇒ anchor #multi-commit-decomposition.

# 2b. v2.2 actually surfaced in the Features row.
grep -n "Multi-commit decomposition |" README.md | grep -iE "arbiter|soft"   # the row names both headlines.

# 2c. v2.2 surfaced in the narrative (arbiter-inclusive freeze + per-file + soft target).
grep -n "leftover-reconciliation arbiter\|across the .* arbiter" README.md    # the refreshed freeze clause.
grep -n "soft count target" README.md                                        # the soft-target clause.

# Expected: all present. If any is absent, the edit is incomplete.
```

### Level 3: Scope & Regression (System Validation)

```bash
# 3a. Scope fence: ONLY README.md changed.
git status --short                # Expected: EXACTLY "M README.md". No docs/ file, no .go file, no go.mod.

# 3b. No code impact (docs-only): the build/test suite is unaffected (sanity, not required).
go build ./... && go test ./...   # Expected: GREEN (no code moved; this is a belt-and-suspenders check).

# 3c. The docs/how-it-works.md, docs/cli.md, docs/configuration.md are UNTOUCHED (sibling/reference files).
git diff --name-only | grep -E "^docs/" && echo "VIOLATION: a docs/ file was edited" || echo "OK: no docs/ file edited"
```

### Level 4: Render & Readability (Domain-Specific Validation)

```bash
# 4a. Render the README locally (if you have a markdown viewer / GitHub preview) and confirm:
#   - The Features table renders the new "Multi-commit decomposition" row as a normal table row.
#   - The two links in the row resolve (how-it-works + cli).
#   - The narrative paragraph reads naturally after the sentence swap (no doubled words, no broken
#     sentence boundary at the swap point).
#
# 4b. Read the Features table top-to-bottom: decompose is now the FIRST row (headline), followed by the
#   v2.1-era capabilities — a coherent prominence order.
#
# 4c. Read the safety FAQ + the snapshot-workflow section: the v2.2 "concurrent edit never enters a
#   commit, including across the arbiter" claim in the Features row is CONSISTENT with the existing
#   "never corrupt your repo" / snapshot narrative (no contradiction).
#
# Expected: renders cleanly; reads naturally; internally consistent.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 (markdown integrity): the new Features row is a valid 2-cell table row; no literal `|` in
      the description; links well-formed.
- [ ] Level 2 (link + content): the `#multi-commit-decomposition` anchor exists; the Features row names
      BOTH v2.2 headlines (freeze incl. arbiter; per-file + soft target); the narrative names the
      arbiter-inclusive freeze + soft target.
- [ ] Level 3 (scope): `git status --short` = EXACTLY `M README.md`; no docs/ or .go file touched;
      `go build ./... && go test ./...` green (no-op confirmation).

### Feature Validation

- [ ] The `## Features` table contains a "Multi-commit decomposition" row (first data row) surfacing both
      v2.2 improvements with a docs link.
- [ ] The Features row contains NO `--commits` / `--single` / `[role.` / `max_commits` / `--reasoning`
      tokens (no per-flag / per-key duplication).
- [ ] The `### Multi-commit decomposition` narrative freeze clause is arbiter-inclusive and adds a
      per-file + soft-target clause; the surrounding sentences are byte-unchanged.
- [ ] The hero pitch blockquote (README L3-5) is BYTE-UNCHANGED (`git diff` shows no hunk there).
- [ ] The surface is internally consistent (the Features-row freeze claim matches the FAQ / snapshot
      narrative's never-corrupt promise).

### Code Quality Validation

- [ ] Markdown style matches the existing Features rows (link format, cell schema, mid-sentence backticks).
- [ ] Em-dash "—" and arrows "→" in preserved sentences are intact (UTF-8 preserved by the surgical edit).
- [ ] Anti-patterns avoided (no whole-paragraph rewrite of the narrative; no flag/config enumeration in
      the Features row; no docs/ or .go edit; hero pitch untouched).

### Documentation & Deployment

- [ ] The Features row links to the authoritative docs (how-it-works + cli), not duplicating them.
- [ ] The v2.2 changeset is summarized on the marketing surface (PRD §21.5): arbiter fully freeze-safe;
      planner per-file + soft target.
- [ ] No new flags/keys documented (there are none — v2.2 is internal-quality).

---

## Anti-Patterns to Avoid

- ❌ Don't alter the hero pitch blockquote (README L3-5) — FROZEN (work item). Byte-compare before/after.
- ❌ Don't edit the L6 "v2.1 adds..." tagline — out of scope (not the feature list; not requested).
- ❌ Don't enumerate `--commits` / `--single` / `[role.planner]` / `max_commits` / `--reasoning` in the
  Features row — that's per-flag/per-key reference duplication (forbidden by the work item; it lives in
  docs/cli.md + docs/configuration.md + the how-to subsection's code blocks).
- ❌ Don't rewrite the whole `### Multi-commit decomposition` narrative paragraph — Edit B is SURGICAL
  (replace the ONE freeze sentence; keep the opener and the stager-constraint sentence byte-unchanged).
- ❌ Don't edit docs/how-it-works.md (the sibling P3.M1.T1.S2 owns it), docs/cli.md, or
  docs/configuration.md (the authoritative reference — link, don't duplicate).
- ❌ Don't edit any `.go` file, go.mod, or go.sum — docs-only task; zero code impact.
- ❌ Don't mention the stager files block / guardrails wording (P2.M1.T3.S1) in the README — that's
  INTERNAL prompt construction with no user-facing surface; its narrative is the sibling S2's concern.
- ❌ Don't invent new flags, keys, or version numbers — v2.2 is internal-quality (P1/P2 add zero of each).
- ❌ Don't break the Features table render (the new row must have exactly 2 `|`-delimited cells; no stray
  `|` in the description; the link syntax is pipe-safe).
- ❌ Don't change the comparison table (L32) — it's accurate; the v2.2 delta is improvement-level, not a
  new yes/no capability.

---

**Confidence Score: 9/10** — This is a low-risk, two-edit markdown change to a single file with no code
impact. The gap is unambiguous (the Features table is missing the decompose row; the narrative understates
the freeze), the v2.2 changeset is precisely identified (FR-M1d arbiter freeze parity + FR-M3 per-file
planner + FR-M4 soft target — all COMPLETE or IMPLEMENTING, with zero new flags/keys), the exact
replacement text is given verbatim for both edits, the link target is verified to exist, and the scope
fence is tight (README only; how-it-works.md is the sibling S2; hero pitch frozen). The only residual
uncertainty is editorial (which of the two Edit-A row wordings the implementer picks, and the exact clause
shape for Edit B) — both are pinned with verbatim primary text + a shorter alternative, and any reasonable
choice satisfying the success criteria (both headlines surfaced, no flag/key tokens, hero pitch intact,
README-only) is correct. The parallel P2.M1.T3.S1 (stager files block) does not touch the README, so there
is no merge conflict.
