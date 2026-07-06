---
name: "P1.M4.T1.S1 (Mode B docs sync) — how-it-works.md headline rewrite (plumbing path now honors hooks) + README.md feature mention — PRD §9.25 (FR-V1–V8) / §9.20 FR-H7"
description: |

  Mode-B documentation sync, RUNS LAST (depends on all of M1–M3). The v2.4 changeset made the snapshot-
  based plumbing path run the repository's standard commit hooks itself (§9.25, FR-V1–V8). That makes two
  docs surfaces FALSE: `docs/how-it-works.md`'s "Hook mode vs the snapshot-based flow" comparison still
  says the snapshot flow "**Bypasses pre-commit hooks** … do NOT run on the generated commit", and frames
  hook mode as the way to get hooks; and `README.md`'s "Does it run my pre-commit hooks?" FAQ still says
  "pre-commit hooks do not run … bypassed". Rewrite both so no stale "bypasses" framing remains, and add
  a one-line feature mention to the README Features table. The detailed feature subsection ("## Commit
  hooks on the plumbing path", added by M3.T2.S1) already exists ABOVE the comparison and explicitly flags
  it as "being reconciled in the v2.4 docs rewrite" — THIS task IS that reconciliation.

  CONTRACT (item_description §3, two files):
    (a) docs/how-it-works.md — REWRITE the comparison section:
        - the Snapshot-based-flow bullet "Bypasses pre-commit hooks" → "Honors pre-commit hooks … scoped
          to the frozen snapshot … `--no-verify` skips pre-commit + commit-msg (mirrors git)".
        - the Hook-mode block: reframe — no longer "the way to get hooks"; it remains the bridge for plain
          `git commit` from IDEs (hooks honored there too, but no snapshot guarantees; latency inside commit).
        - "When to use which": the two modes COMPOSE (a user who always invokes `stagehand` gets hook
          coverage WITHOUT installing hook mode; hook mode covers `git commit` from an IDE). Cross-link the
          M3.T2.S1 subsection (#commit-hooks-on-the-plumbing-path) and §9.25.
    (b) README.md — rewrite the stale FAQ + add a one-line feature mention: hooks run on every `stagehand`
          commit (atomic + stage-while-generating preserved); `--no-verify` mirrors git.
    VERIFY: grep the whole docs/ tree ( + README.md) for bypass/Bypasses/"pre-commit hooks do NOT run" and
    confirm every STALE claim is updated (distinguishing the correct unrelated "bypass" hits — §2).

  DELIVERABLE (2 files modified; nothing else):
    MODIFY `docs/how-it-works.md` — 4 edits (Snapshot-flow bullet; Hook-mode bullet; When-to-use-which; the
    M3.T2.S1 subsection's now-stale "is being reconciled" parenthetical).
    MODIFY `README.md` — 2 edits (rewrite the FAQ; add one Features-table row).

  SCOPE NOTE (the task's line numbers are STALE, design §0): the contract cites how-it-works.md:312/316/
    327-329, but M3.T2.S1's subsection was inserted ABOVE, shifting everything down. The ACTUAL anchors
    (verified): how-it-works.md L337 (Bypasses bullet), L344 (Hook-mode bullet), L353-357 (When-to-use-
    which), L325-326 (the "is being reconciled" parenthetical); README.md L368 (FAQ), L70 (Git hook mode
    Features row). Anchor on CONTENT, not line numbers.

  SCOPE NOTE (the VERIFY grep is a starting point, NOT a fix-list, design §2): grep surfaces BOTH the real
    stale claims (how-it-works.md L337/344/355 + README.md L368) AND ~6 CORRECT unrelated "bypass" hits:
    cli.md:34 (--single "Bypass decomposition"), cli.md:42 (--edit "bypasses the duplicate check"),
    cli.md:44 (--no-verify "Bypass pre-commit and commit-msg hooks" — THIS IS the bypass flag, correct),
    configuration.md:155/234 (no_verify / --single), how-it-works.md:115 (planner "bypassed"). Do NOT touch
    those — they are accurate. Fix ONLY the 4 STALE entries.

  SCOPE BOUNDARY (what this does NOT do): NO code; NO tests; NO edits to docs/cli.md, docs/configuration.md,
    docs/providers.md (their --no-verify / hook_timeout / no_verify content is already accurate — added by
    M1.T1/M1.T2). This is a docs-only Mode-B sweep of TWO files.

  INPUT (upstream — already built): the full §9.25 feature (M1 config/CLI + M2 git primitives + M3 hooks
    runner + wiring into CommitStaged/runPipeline/decompose). OUTPUT: docs/how-it-works.md + README.md
    reflect that the plumbing path honors hooks; no stale "bypasses" framing remains.

  ⚠️ Anchor edits on CONTENT, not the task's stale line numbers (M3.T2.S1 shifted them). (§0)
  ⚠️ The VERIFY grep has ~6 CORRECT "bypass" hits (--single/--edit/--no-verify/planner) — do NOT touch them.
     Fix ONLY the 4 STALE entries (how-it-works.md L337/344/355 + README.md L368). (§2)
  ⚠️ Preserve the "### Trade-off inversion (FR-H7)" header (README L70 links #trade-off-inversion-fr-h7) —
     reframe the CONTENT, don't rename the header. (§4)
  ⚠️ Update the M3.T2.S1 subsection's "is being reconciled" parenthetical (it's stale once this lands). (§3)

  Deliverable: 2 modified files (docs/how-it-works.md, README.md); `go build ./... && go test ./...` green
  & unchanged (no code touched); `git status` shows ONLY those two files.

---

## Goal

**Feature Goal**: Reconcile `docs/how-it-works.md`'s "Hook mode vs the snapshot-based flow" comparison and
`README.md`'s hooks FAQ with the v2.4 reality (§9.25): the snapshot-based plumbing path now runs the
repository's commit hooks itself, scoped to the frozen snapshot. Eliminate every stale "bypasses pre-commit
hooks" / "hooks do not run" claim, reframe hook mode as the bridge for plain `git commit` (not the way to
get hooks), and add a one-line feature mention to the README — so the two docs surfaces match the shipped
behavior and the M3.T2.S1 feature subsection above them.

**Deliverable** (2 files modified; nothing else):
1. `docs/how-it-works.md` — 4 edits: (1) Snapshot-flow "Bypasses" bullet → "Honors"; (2) Hook-mode bullet
   reframed (bridge for `git commit`, not the hooks source); (3) "When to use which" reframed (the modes
   compose); (4) the M3.T2.S1 subsection's "is being reconciled" parenthelial → a clean cross-link.
2. `README.md` — 2 edits: (1) rewrite the "Does it run my pre-commit hooks?" FAQ (the stale claim); (2) add
   one Features-table row for "Commit hooks on every `stagehand` commit".

**Success Definition**: no occurrence of "Bypasses pre-commit hooks" / "pre-commit hooks do not run" /
"bypassed" (in the hooks sense) remains in how-it-works.md or README.md; the comparison section states the
snapshot flow honors hooks (scoped to the snapshot; `--no-verify` mirrors git) and that hook mode is the
`git commit` bridge; the two-modes-compose point is made; README's FAQ says "yes" and the Features table
has the new row; the 6 correct unrelated "bypass" hits (--single/--edit/--no-verify/planner) are
byte-unchanged; `git status` shows ONLY the two files; `go build ./... && go test ./...` green & unchanged.

## User Persona

**Target User**: The reader deciding whether `stagehand` will run their hooks (husky, lint-staged,
conventional-commit lint, a formatter, post-commit notifications). Pre-§9.25 the docs told them hooks were
bypassed on the `stagehand` path and hook mode was the only fix — wrong now. After this sync the docs match
the code: hooks run on every `stagehand` commit (atomic + stage-while-generating preserved), and hook mode
is only for when they commit via plain `git commit`.

**Use Case**: A husky/lint-staged user reads the README FAQ or the how-it-works comparison to decide
whether `stagehand` is safe for their workflow. They see: yes, hooks run; `--no-verify` for a one-off; hook
mode optional (only for IDE `git commit`).

**User Journey**: user opens README → Features row "Commit hooks on every `stagehand` commit" → FAQ "Does
it run my pre-commit hooks?" → "Yes (v2.4)…" → (optional) how-it-works comparison + the M3.T2.S1 subsection
for the full scope/freeze/failure detail.

**Pain Points Addressed**: Stale/misleading docs — a hooks-reliant user would wrongly conclude they MUST
install hook mode (forfeiting atomicity) to get hooks, or that `stagehand` silently skips their lint. Both
gaps closed.

## Why

- **It IS the Mode-B docs half of the §9.25 changeset.** M1–M3 shipped the feature (plumbing path runs
  hooks); the docs were not reconciled. codebase_reality.md §8 names this as "the headline Mode B rewrite."
- **Prevents a false impression that costs users the atomic path.** The old framing ("bypasses hooks → use
  hook mode") pushed hook-reliant users toward hook mode (no snapshot/atomicity) when `stagehand` itself now
  covers them.
- **Keeps the docs self-consistent.** The M3.T2.S1 subsection (above the comparison) documents the feature
  and flags the comparison as "being reconciled" — leaving that unresolved makes the doc contradict itself.
- **Trivial, isolated, no-risk.** Two markdown files; no code, no tests, no other docs.

## What

Six small markdown edits across two files, and nothing else. No code, no tests, no config, no CLI, no other
doc files.

### Success Criteria

- [ ] **how-it-works.md** Snapshot-flow bullet: "Bypasses pre-commit hooks … do NOT run" is GONE, replaced
      by a "Honors pre-commit hooks" bullet stating the repo's pre-commit → prepare-commit-msg → commit-msg
      → post-commit run around every `stagehand` commit, scoped to the frozen snapshot (freeze holds), and
      `--no-verify` skips pre-commit + commit-msg (mirrors `git commit --no-verify`).
- [ ] **how-it-works.md** Hook-mode bullet: the "Pre-commit hooks honored: the commit flows through the
      standard `git commit` path" framing is GONE; reframed to: hook mode is the bridge for plain
      `git commit` from an IDE/tool (hooks honored there via real `git commit`, but no snapshot guarantees,
      no stage-while-generating, latency inside the commit). The Never-block / No-rescue bullets stay.
- [ ] **how-it-works.md** "When to use which": reframed — `stagehand` directly is the day-to-day default
      (atomic + stage-while-generating + now hooks); hook mode only for plain `git commit` from an IDE; the
      two COMPOSE (§9.25 covers `stagehand`, hook mode covers `git commit`); cross-link
      `#commit-hooks-on-the-plumbing-path`.
- [ ] **how-it-works.md** M3.T2.S1 subsection trailing parenthetical: "is being reconciled in the v2.4 docs
      rewrite" is GONE; replaced by a clean forward cross-link to the (now-reconciled) comparison section.
- [ ] **README.md** FAQ "Does it run my pre-commit hooks?": "do not run / bypassed" is GONE; rewritten to
      "Yes — as of v2.4 …" (hooks run scoped to the snapshot; `--no-verify`; hook mode for `git commit`;
      cross-link `#commit-hooks-on-the-plumbing-path`).
- [ ] **README.md** Features table: ONE new row "Commit hooks on every `stagehand` commit" added (after the
      "Git hook mode" row), cross-linking `#commit-hooks-on-the-plumbing-path`. The existing "Git hook mode"
      row stays (complementary).
- [ ] VERIFY grep: `grep -rniE "bypass|pre-commit hooks do NOT run" docs/ README.md` shows NO stale hooks
      claim; the 6 correct unrelated hits (--single/--edit/--no-verify/planner) are byte-unchanged.
- [ ] `git status` shows ONLY docs/how-it-works.md + README.md; NO `.go`/test/other-doc file touched;
      `go build ./... && go test ./...` GREEN & unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A technical writer with no prior repo knowledge can implement this from: the 6 verbatim edits +
their content anchors (below), the §2 stale-vs-correct distinction (so the VERIFY grep isn't mis-applied),
the §3/§4 parenthetical + header-preservation notes, and the LEAVE list. No git/Go/§9.25-internals
knowledge required — the edits are self-contained markdown, and the authoritative behavior is quoted from
PRD §9.25/§9.20 (in context).

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions
- docfile: plan/010_49117f1f30ab/P1M4T1S1/research/design-decisions.md
  why: the 6 decisions. §0 (scope: 2 files; the task's line numbers are STALE — anchor on content), §1 (the
       reframing logic: snapshot now honors hooks; hook mode = `git commit` bridge; compose), §2 (THE VERIFY
       grep distinction — 4 STALE vs 6 CORRECT unrelated "bypass" hits; do NOT touch the correct ones),
       §3 (the M3.T2.S1 "is being reconciled" parenthetical becomes stale — update it), §4 (PRESERVE the
       "### Trade-off inversion (FR-H7)" header — README L70 links its anchor), §5 (README FAQ rewrite +
       Features-row add), §6 (no conflict with parallel M3.T3.S1).
  critical: §0 (content anchors, not line numbers), §2 (don't "fix" the correct --no-verify/--single/--edit
       hits), §4 (don't orphan the README cross-link by renaming the header).

# MUST READ — the file being edited (the comparison section + the M3.T2.S1 subsection above it)
- file: docs/how-it-works.md   (EDIT; one of two files)
  section: "## Commit hooks on the plumbing path" (L303, M3.T2.S1) — its trailing parenthetical (L325-326)
           is edit (4). "## Hook mode vs the snapshot-based flow" (L328) → "### Trade-off inversion (FR-H7)"
           (L330, KEEP this header) → the Snapshot-based-flow bullets (the "Bypasses pre-commit hooks"
           bullet at L337 is edit 1) → the Hook-mode bullets (the "Pre-commit hooks honored" bullet at L344
           is edit 2) → "### When to use which" (L353-357, edit 3).
  why: the EXACT placement anchors for the 4 edits. Reference by CONTENT (the bullet text), not line
       numbers (they shifted when M3.T2.S1's subsection was inserted).
  critical: edit (1) replaces the L337 bullet VERBATIM (the task gives the target text); edit (2) reframes
       L344 (hook mode ≠ the hooks source); KEEP the "### Trade-off inversion (FR-H7)" header (L330).

# MUST READ — the other file being edited (Features table + the FAQ)
- file: README.md   (EDIT; the other file)
  section: "## Features" table (L59-71) — add ONE row after the "Git hook mode" row (L70). The FAQ
           "### Does it run my pre-commit hooks?" (L366-374) — rewrite the answer (L368, the stale claim).
  why: the EXACT anchors for the 2 README edits. The Features row cross-links
       docs/how-it-works.md#commit-hooks-on-the-plumbing-path (the M3.T2.S1 subsection anchor); the FAQ
       cross-links the same.
  critical: the existing "Git hook mode" row (L70) cross-links #trade-off-inversion-fr-h7 — KEEP that
       anchor valid (don't rename the how-it-works header it points at). Add the NEW row; don't muddy the
       existing one.

# MUST READ — the authoritative behavior (quote the hooks/freeze/no-verify semantics from here)
- file: PRD.md (or plan/010_49117f1f30ab/prd_snapshot.md)
  section: "9.25 Hook execution on the commit path" (FR-V1–V8) + the updated "9.20 Git hook mode" (FR-H7)
           — in your context as selected_prd_content `h3.41`/`h3.36`.
  why: the SOURCE OF TRUTH for the rewrite. FR-V1 (hooks run in git's order around every plumbing commit),
       FR-V3 (pre-commit scoped to T_start — freeze holds), FR-V5 (--no-verify skips pre-commit + commit-msg
       only), FR-V8d (hook mode unchanged = bridge for `git commit`; the two compose). The docs edits must
       match these.
  critical: the rewrite must say hooks run "scoped to the frozen snapshot" (FR-V3 — NOT the live index);
       `--no-verify` skips ONLY pre-commit + commit-msg (FR-V5 — prepare-commit-msg + post-commit still run);
       hook mode = `git commit` bridge (FR-H7/FR-V8d).

# The doc-debt this discharges (confirms this is the headline Mode B rewrite)
- docfile: plan/010_49117f1f30ab/architecture/codebase_reality.md
  section: "## 8. The docs that contradict the feature (Mode B headline)" — names how-it-works.md as the
           headline rewrite (the "Bypasses pre-commit hooks" claim + the "When to use which" reframe).
  why: confirms the scope. (cli.md/configuration.md Mode-A updates already rode with M1.T1/M1.T2 — not this task.)

# The M3.T2.S1 subsection whose parenthetical is edit (4) (read its exact trailing text)
- file: docs/how-it-works.md   (the "## Commit hooks on the plumbing path" subsection, L303-326)
  section: the closing sentence (L325-326): "See PRD §9.25 (FR-V1–V8) for the full specification. (The
           'Hook mode vs the snapshot-based flow' framing below is being reconciled in the v2.4 docs
           rewrite — hook mode remains the bridge for plain `git commit` from IDEs, and the two modes now
           compose.)"
  why: edit (4) — once the comparison is rewritten, "is being reconciled in the v2.4 docs rewrite" is
       self-referential/stale. Replace with a clean cross-link.

# Confirms the parallel task is code-only (no docs overlap)
- docfile: plan/010_49117f1f30ab/P1M3T3S1/PRP.md
  section: the header — it wires RunCommitHooks into decompose.publishCommit/arbiter (CODE in
           internal/decompose + internal/hooks). It does NOT touch how-it-works.md or README.md.
  why: confirms no parallel-edit conflict. This task edits ONLY those two docs files.
```

### Current Codebase tree (relevant slice)

```bash
docs/
  how-it-works.md   # *** EDIT *** — the comparison section (L328-357) + the M3.T2.S1 subsection's parenthetical (L325-326).
  cli.md            # READ ONLY — the --no-verify row (L44) is CORRECT (added by M1.T2.S1). Do NOT touch.
  configuration.md  # READ ONLY — no_verify/hook_timeout (L155/~145) are CORRECT (added by M1.T1). Do NOT touch.
  providers.md      # READ ONLY (no hooks content).
  README.md (index) # READ ONLY.
README.md           # *** EDIT *** — the Features table (L59-71) + the FAQ (L366-374).
# NO .go / test / config / go.mod / Makefile / PRD.md changes. Mode B (docs only).
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. TWO in-place edits: docs/how-it-works.md (4 edits) + README.md (2 edits).
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the task's line numbers are STALE, design §0): the contract cites how-it-works.md:312/316/
     327-329, but M3.T2.S1's subsection was inserted ABOVE, shifting the comparison down to L337/344/353.
     Anchor EVERY edit on the unique surrounding bullet/sentence CONTENT (e.g. the "Bypasses pre-commit
     hooks" bullet, the "Pre-commit hooks honored" bullet), NOT line numbers. -->

<!-- CRITICAL (the VERIFY grep is a starting point, NOT a fix-list, design §2): grep -rniE
     "bypass|pre-commit hooks do NOT run" docs/ README.md surfaces 4 STALE entries (how-it-works.md
     L337/344/355 + README.md L368) AND ~6 CORRECT unrelated hits: cli.md:34 (--single "Bypass
     decomposition"), cli.md:42 (--edit "bypasses the duplicate check"), cli.md:44 (--no-verify "Bypass
     pre-commit and commit-msg hooks" — THIS IS the bypass flag, correct), configuration.md:155/234
     (no_verify / --single), how-it-works.md:115 (planner "bypassed"). Do NOT touch the correct ones — fix
     ONLY the 4 stale entries. -->

<!-- CRITICAL (PRESERVE the "### Trade-off inversion (FR-H7)" header, design §4): README.md:70 (the "Git
     hook mode" Features row) cross-links #trade-off-inversion-fr-h7 — the slug of that header (L330).
     REFRAME the section's bullet CONTENT but KEEP the header verbatim, or you orphan the README link.
     ("Trade-off inversion" is still apt: hook mode inverts the atomicity/snapshot trade-off; hooks are no
     longer the differentiator.) If you MUST rename it, update README.md:70's link too — but default: keep it. -->

<!-- CRITICAL (update the M3.T2.S1 "is being reconciled" parenthetical, design §3): once the comparison is
     rewritten, the subsection's closing "(… is being reconciled in the v2.4 docs rewrite …)" is stale
     (self-referential). Replace it with a clean forward cross-link to the now-reconciled comparison. -->

<!-- GOTCHA (the freeze wording, FR-V3): the rewrite must say hooks run "scoped to the frozen snapshot"
     (pre-commit sees a throwaway index primed from T_start, NOT the live index) — that's what preserves
     stage-while-generating. Do NOT say "hooks run against the staged index" (that would imply the live index). -->

<!-- GOTCHA (--no-verify scope, FR-V5): --no-verify skips ONLY pre-commit + commit-msg; prepare-commit-msg
     and post-commit STILL run (git does not gate them on --no-verify). Do NOT write "skips all hooks". -->

<!-- GOTCHA (markdown anchor slugs): GitHub lowercases + hyphenates + strips punctuation.
     "## Commit hooks on the plumbing path" → #commit-hooks-on-the-plumbing-path.
     "## Hook mode vs the snapshot-based flow" → #hook-mode-vs-the-snapshot-based-flow.
     "### Trade-off inversion (FR-H7)" → #trade-off-inversion-fr-h7. Verify the cross-links resolve. -->

<!-- GOTCHA (the README "Git hook mode" row STAYS, design §5): it documents hook mode (the `git commit`
     bridge). The NEW row documents hooks on the `stagehand` path. They are complementary — do not merge or
     delete the existing row. -->
```

## Implementation Blueprint

### Data models and structure

```markdown
<!-- NO data models. A markdown-docs task. The "structure" is the 6 edits + their content anchors. -->
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/how-it-works.md — edit (1): the Snapshot-flow "Bypasses" bullet → "Honors"
  - LOCATE the "## Hook mode vs the snapshot-based flow" section, "### Trade-off inversion (FR-H7)",
      "Snapshot-based flow" bullets. Find the bullet: "- **Bypasses pre-commit hooks**: because the commit
      is built via plumbing (not `git commit`), tools like husky, lint-staged, and `.pre-commit-config.yaml`
      do NOT run on the generated commit."
  - REPLACE that bullet (verbatim target, from the task):
      - **Honors pre-commit hooks**: the repository's pre-commit → prepare-commit-msg → commit-msg →
        post-commit hooks run around every stagehand commit, scoped to the frozen snapshot (so the
        stage-while-generating freeze holds). `--no-verify` skips pre-commit + commit-msg (mirrors `git
        commit --no-verify`). See [Commit hooks on the plumbing path](#commit-hooks-on-the-plumbing-path).
  - GOTCHA: keep the bullet's place in the Snapshot-flow list. The other Snapshot bullets (Atomic /
      Stage-while-generating / Rescue protocol) stay.

Task 2: EDIT docs/how-it-works.md — edit (2): the Hook-mode "Pre-commit hooks honored" bullet → reframed
  - LOCATE the Hook-mode block. Find the bullet: "- **Pre-commit hooks honored**: the commit flows through
      the standard `git commit` path, so husky, lint-staged, and any other `pre-commit` hooks run normally."
  - REPLACE (reframe — hook mode is NO LONGER "the way to get hooks"; it's the `git commit` bridge):
      - **The bridge for plain `git commit`**: hook mode covers the case where you commit via `git commit`
        from an IDE or another tool instead of invoking `stagehand`. Hooks run there too (real `git
        commit`), but there is no snapshot, no atomicity guarantee, and no stage-while-generating —
        generation latency happens inside the commit.
  - GOTCHA: the other Hook-mode bullets (No snapshot guarantees / Never-block contract / No rescue protocol)
      stay. The "Multi-turn fallback in hook mode" paragraph (added by an earlier changeset) stays.

Task 3: EDIT docs/how-it-works.md — edit (3): "When to use which" reframed (the modes compose)
  - LOCATE "### When to use which" (3 bullets). REPLACE the 3 bullets with:
      - Use **`stagehand` directly** (the snapshot flow) for day-to-day commits: it's atomic,
        stage-while-generating, and — as of v2.4 — honors your repository's hooks (`--no-verify` for a
        one-off skip).
      - Install **hook mode** only if you commit via plain `git commit` from an IDE or lazygit instead of
        invoking `stagehand` — it fills the message without blocking, with hooks honored but no snapshot
        guarantees.
      - The two **compose**: [Commit hooks on the plumbing path](#commit-hooks-on-the-plumbing-path) (§9.25)
        covers `stagehand` commits; hook mode covers `git commit` commits.
  - GOTCHA: the thrust flips — `stagehand` is the day-to-day default (it now has hooks); hook mode is the
      opt-in for the `git commit` case. KEEP the "### When to use which" header.

Task 4: EDIT docs/how-it-works.md — edit (4): the M3.T2.S1 subsection's "is being reconciled" parenthetical
  - LOCATE the closing sentence of "## Commit hooks on the plumbing path": "See PRD §9.25 (FR-V1–V8) for the
      full specification. (The 'Hook mode vs the snapshot-based flow' framing below is being reconciled in
      the v2.4 docs rewrite — hook mode remains the bridge for plain `git commit` from IDEs, and the two
      modes now compose.)"
  - REPLACE the whole sentence with: "See PRD §9.25 (FR-V1–V8) for the full specification, and [Hook mode
      vs the snapshot-based flow](#hook-mode-vs-the-snapshot-based-flow) below for how the two modes compose."
  - GOTCHA: this removes the now-stale "is being reconciled in the v2.4 docs rewrite" clause (the
      reconciliation is Task 1-3). Keeps a clean cross-link to the comparison section.

Task 5: EDIT README.md — edit (5): rewrite the "Does it run my pre-commit hooks?" FAQ
  - LOCATE "### Does it run my pre-commit hooks?" and its answer (the "pre-commit hooks do not run …
      bypassed … install hook mode" text).
  - REPLACE the answer paragraph(s) with:
      Yes. As of v2.4, the default `stagehand` command runs your repository's standard commit hooks
      (`pre-commit` → `prepare-commit-msg` → `commit-msg` → `post-commit`) around every commit, scoped to
      the frozen snapshot — so atomicity and stage-while-generating are preserved (a `pre-commit`
      formatter's fixes are included; a hook that stages brand-new content aborts the run). `--no-verify`
      skips `pre-commit` and `commit-msg` only, mirroring `git commit --no-verify`. **Hook mode**
      (`stagehand hook install`) remains for when you commit via plain `git commit` from an IDE — the two
      compose (§9.25 covers `stagehand`; hook mode covers `git commit`). See [Commit hooks on the plumbing
      path](docs/how-it-works.md#commit-hooks-on-the-plumbing-path).
  - GOTCHA: this fixes the stale "do not run / bypassed" claim (the VERIFY target). Keep the "### Does it
      run my pre-commit hooks?" header. If the old answer had a `stagehand hook install` code block, you may
      keep a trimmed one-liner or drop it (the FAQ now leads with "Yes").

Task 6: EDIT README.md — edit (6): add ONE Features-table row
  - LOCATE the "## Features" table. Find the "Git hook mode" row (cross-links #trade-off-inversion-fr-h7).
  - INSERT a new row IMMEDIATELY AFTER it:
      | Commit hooks on every `stagehand` commit | As of v2.4, your repo's `pre-commit` → `prepare-commit-msg` → `commit-msg` → `post-commit` hooks run around every `stagehand` commit, scoped to the frozen snapshot (atomic + stage-while-generating preserved); `--no-verify` mirrors git ([how it works](docs/how-it-works.md#commit-hooks-on-the-plumbing-path)). |
  - GOTCHA: keep the existing "Git hook mode" row (complementary). The new row cross-links the M3.T2.S1
      subsection anchor. One row (the task says "a sentence or two").

Task 7: VERIFY (Mode B validation)
  - VERIFY grep: `grep -rniE "bypass|pre-commit hooks do NOT run|do not run on|hooks.*not.*run" docs/
      README.md` → confirm the 4 STALE entries are gone (how-it-works.md Snapshot-flow bullet; the Hook-mode
      bullet; the When-to-use-which framing; README FAQ) AND the 6 CORRECT unrelated hits (--single/--edit/
      --no-verify/planner) are byte-unchanged.
  - Render-check: the cross-links resolve (`#commit-hooks-on-the-plumbing-path`,
      `#hook-mode-vs-the-snapshot-based-flow`, `#trade-off-inversion-fr-h7`).
  - `git status --porcelain` → ONLY docs/how-it-works.md + README.md.
  - `go build ./... && go test ./...` → GREEN & unchanged (no code touched).
  - `git diff --exit-code` on docs/cli.md, docs/configuration.md, docs/providers.md, go.mod, Makefile, PRD.md
      → all unchanged.
```

### Implementation Patterns & Key Details

```markdown
<!-- THE 6 edits, summarized (anchor → action):
  how-it-works.md:
    (1) Snapshot-flow "Bypasses pre-commit hooks" bullet → "Honors pre-commit hooks" (scoped to snapshot;
        --no-verify; cross-link #commit-hooks-on-the-plumbing-path).
    (2) Hook-mode "Pre-commit hooks honored" bullet → "The bridge for plain `git commit`" (hooks honored
        there too, but no snapshot/atomicity/stage-while-generating; latency inside commit).
    (3) "When to use which" 3 bullets → reframed (`stagehand` = day-to-day default w/ hooks; hook mode =
        `git commit` opt-in; compose; cross-link).
    (4) M3.T2.S1 subsection trailing "is being reconciled" parenthetical → clean cross-link to the
        comparison section.
  README.md:
    (5) "Does it run my pre-commit hooks?" FAQ → "Yes (v2.4)…" (scoped to snapshot; --no-verify; hook mode
        for git commit; cross-link).
    (6) Features table: + one "Commit hooks on every `stagehand` commit" row (after "Git hook mode").

<!-- THE accuracy pins (don't get these wrong):
  - Hooks run "scoped to the FROZEN snapshot" (FR-V3) — pre-commit sees a throwaway index from T_start,
    NOT the live index. That's what preserves stage-while-generating.
  - `--no-verify` skips ONLY pre-commit + commit-msg (FR-V5); prepare-commit-msg + post-commit STILL run.
  - Hook mode = the bridge for plain `git commit` (FR-H7/FR-V8d); NOT "the way to get hooks" anymore.

<!-- THE preserve/don't-break list:
  - KEEP the "### Trade-off inversion (FR-H7)" header (README L70 links #trade-off-inversion-fr-h7).
  - KEEP the README "Git hook mode" Features row (complementary to the new row).
  - KEEP the 6 correct "bypass" hits (--single/--edit/--no-verify/planner) byte-unchanged.
  - KEEP the how-it-works "Multi-turn fallback in hook mode" paragraph + the other Snapshot/Hook bullets.

<!-- THE anchor slugs (GitHub markdown):
  - "## Commit hooks on the plumbing path"       → #commit-hooks-on-the-plumbing-path
  - "## Hook mode vs the snapshot-based flow"     → #hook-mode-vs-the-snapshot-based-flow
  - "### Trade-off inversion (FR-H7)"             → #trade-off-inversion-fr-h7
```

### Integration Points

```yaml
DOCS.FILES (the ONLY edits): docs/how-it-works.md (4 edits) + README.md (2 edits).

DOCS.LEFT-UNCHANGED (do NOT edit — already accurate / other-scope):
  - docs/cli.md            # the --no-verify row (L44) is CORRECT (M1.T2.S1).
  - docs/configuration.md  # no_verify / hook_timeout (L155/~145) are CORRECT (M1.T1.S1).
  - docs/providers.md, docs/README.md (index)  # no hooks content.
  - PRD.md, go.mod, Makefile  # never.

CODE.LEFT-UNCHANGED: NO .go / test / config changes (Mode B — docs only).

UPSTREAM (the changeset this docs task reflects — already built, do NOT re-do):
  - M1 (config NoVerify/HookTimeout + --no-verify flag) + M2 (git primitives) + M3 (hooks runner + wiring
    into CommitStaged/runPipeline/decompose). The M3.T2.S1 feature subsection (how-it-works.md:303) is the
    detailed doc; THIS task reconciles the comparison + README to match it.

DOWNSTREAM: none. This is the docs end-state for the §9.25 changeset (Mode B).

CROSS-LINKS (must resolve): #commit-hooks-on-the-plumbing-path (the M3.T2.S1 subsection);
  #hook-mode-vs-the-snapshot-based-flow (the comparison section); #trade-off-inversion-fr-h7 (unchanged
  header, linked from README L70).
```

## Validation Loop

### Level 1: Markdown sanity + the edits landed

```bash
# Confirm the 6 edits landed at the right content anchors:
grep -n "Honors pre-commit hooks" docs/how-it-works.md                       # edit (1)
grep -n "bridge for plain" docs/how-it-works.md                              # edit (2)
grep -n "stagehand.*directly.*day-to-day\|two.*compose" docs/how-it-works.md # edit (3)
grep -n "is being reconciled" docs/how-it-works.md || echo "OK: stale parenthetical removed (edit 4)"
grep -n "As of v2.4.*standard commit hooks\|Yes.*pre-commit hooks" README.md # edit (5)
grep -n "Commit hooks on every.*stagehand.*commit" README.md                 # edit (6)
# Confirm the cross-link targets exist (anchors resolve):
grep -nE "^## (Commit hooks on the plumbing path|Hook mode vs the snapshot-based flow)|^### Trade-off inversion" docs/how-it-works.md
# Expected: all 6 edits present; the 3 headers exist (so the 3 cross-link slugs resolve). Read the rendered
# files (or a markdown previewer) to confirm the links work.
```

### Level 2: The VERIFY grep (the stale-claim audit)

```bash
# The task's VERIFY step:
grep -rniE "bypass|pre-commit hooks do NOT run|do not run on|hooks.*not.*run" docs/ README.md
# EXPECTED: the ONLY remaining hits are the 6 CORRECT unrelated ones —
#   docs/cli.md:34 (--single "Bypass decomposition")
#   docs/cli.md:42 (--edit "bypasses the duplicate check")
#   docs/cli.md:44 (--no-verify "Bypass pre-commit and commit-msg hooks" — the bypass flag, correct)
#   docs/configuration.md:155 (no_verify "the --no-verify bypass")
#   docs/configuration.md:234 (--single "Bypass decompose")
#   docs/how-it-works.md:115 (planner "bypassed entirely")
# The 4 STALE hooks-claim entries (how-it-works.md old Snapshot/Hook/When-to-use + README FAQ) must be GONE.
# If any stale hooks claim remains, fix it. If a "correct" hit was wrongly edited, revert it.
```

### Level 3: Scope check + regression no-op (no code touched)

```bash
git status --porcelain
# Expected: exactly TWO modified files — docs/how-it-works.md + README.md. NOTHING else.
git diff --exit-code docs/cli.md docs/configuration.md docs/providers.md go.mod go.sum Makefile PRD.md \
  && echo "other docs + go.mod/Makefile/PRD UNCHANGED (expected)"
go build ./...   # Expect clean & unchanged (no .go file touched).
go test ./...    # Expect GREEN & unchanged (docs-only; no test touched).
! git diff --name-only | grep -E '\.go$' && echo "OK: no .go file modified (Mode B docs-only)"
```

### Level 4: Accuracy review (the docs match PRD §9.25)

```bash
# Verify the rewritten claims match the PRD semantics (read-only checks against the docs text):
#   1. "scoped to the frozen snapshot" (FR-V3 — NOT the live index):
grep -niE "scoped to the frozen snapshot|frozen snapshot" docs/how-it-works.md README.md
#   2. --no-verify skips ONLY pre-commit + commit-msg (FR-V5):
grep -niE "no-verify.*pre-commit.*commit-msg|skips.*pre-commit.*commit-msg" docs/how-it-works.md README.md
#   3. hook mode = the `git commit` bridge (FR-H7/FR-V8d), NOT "the way to get hooks":
grep -niE "bridge for plain|git commit.*IDE" docs/how-it-works.md README.md
#   4. the two modes compose:
grep -ni "compose" docs/how-it-works.md README.md
# Expected: each grep confirms the rewritten docs state the §9.25 behavior. (No code edits — read-only proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean & unchanged; `go test ./...` GREEN & unchanged (docs-only — no code touched).
- [ ] `git status` shows EXACTLY TWO modified files: docs/how-it-works.md + README.md. No other file touched.
- [ ] go.mod/go.sum/Makefile/PRD.md byte-unchanged; docs/cli.md/configuration.md/providers.md byte-unchanged.

### Feature Validation
- [ ] **(1)** Snapshot-flow bullet is "Honors pre-commit hooks" (scoped to frozen snapshot; `--no-verify`
      skips pre-commit + commit-msg; cross-link).
- [ ] **(2)** Hook-mode bullet is reframed ("the bridge for plain `git commit`"; hooks honored there too,
      but no snapshot/atomicity/stage-while-generating).
- [ ] **(3)** "When to use which" reframed (`stagehand` = day-to-day default w/ hooks; hook mode = `git
      commit` opt-in; compose; cross-link).
- [ ] **(4)** M3.T2.S1 subsection's "is being reconciled" parenthetical replaced with a clean cross-link.
- [ ] **(5)** README FAQ rewritten ("Yes (v2.4)…"; scoped to snapshot; `--no-verify`; hook mode for `git
      commit`; cross-link).
- [ ] **(6)** README Features table has the new "Commit hooks on every `stagehand` commit" row (existing
      "Git hook mode" row kept).
- [ ] VERIFY grep: the 4 stale hooks claims are GONE; the 6 correct unrelated "bypass" hits are unchanged.
- [ ] Cross-links resolve (`#commit-hooks-on-the-plumbing-path`, `#hook-mode-vs-the-snapshot-based-flow`,
      `#trade-off-inversion-fr-h7`).

### Code Quality Validation
- [ ] The 6 edits are minimal and anchored on existing content (no wholesale rewrites beyond the targeted
      bullets/FAQ); each reframed bullet is 1-3 sentences.
- [ ] The "### Trade-off inversion (FR-H7)" header preserved (README L70 link stays valid).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (cli.md/configuration.md/providers.md frozen).

### Documentation
- [ ] docs/how-it-works.md + README.md reflect that the plumbing path honors hooks; no stale "bypasses"
      framing remains. (This IS the documentation task — Mode B.)

---

## Anti-Patterns to Avoid

- ❌ **Don't anchor edits on the task's line numbers (312/316/327).** They're stale — M3.T2.S1's subsection
  shifted the comparison down to L337/344/353. Anchor on the unique bullet/sentence CONTENT. (§0)
- ❌ **Don't "fix" the correct unrelated "bypass" hits.** The VERIFY grep surfaces 6 CORRECT usages (--single
  "Bypass decomposition"; --edit "bypasses the duplicate check"; --no-verify "Bypass pre-commit and
  commit-msg hooks" [the bypass flag itself]; no_verify; planner "bypassed"). Touch ONLY the 4 stale
  hooks-claim entries. (§2)
- ❌ **Don't rename "### Trade-off inversion (FR-H7)".** README.md:70 links `#trade-off-inversion-fr-h7`.
  Reframe the CONTENT; keep the header (or also update the README link — but default: keep it). (§4)
- ❌ **Don't say hooks run "against the staged/live index".** FR-V3: pre-commit runs against a throwaway
  index primed from the FROZEN snapshot (T_start), never the live index — that's what preserves
  stage-while-generating. Say "scoped to the frozen snapshot". (gotcha)
- ❌ **Don't say `--no-verify` "skips all hooks".** FR-V5: it skips ONLY pre-commit + commit-msg;
  prepare-commit-msg and post-commit STILL run. (gotcha)
- ❌ **Don't frame hook mode as "the way to get hooks".** Post-§9.25 it's the bridge for plain `git commit`
  from an IDE; `stagehand` itself honors hooks. (§1)
- ❌ **Don't delete the README "Git hook mode" Features row.** It's complementary to the new row (hook mode
  = `git commit`; new row = `stagehand`). Keep both. (§5)
- ❌ **Don't leave the M3.T2.S1 "is being reconciled" parenthetical.** Once the comparison is rewritten,
  it's self-referential/stale. Replace with a clean cross-link. (§3)
- ❌ **Don't touch docs/cli.md / docs/configuration.md / docs/providers.md.** Their --no-verify /
  hook_timeout / no_verify content is already accurate (M1.T1/M1.T2). (scope boundary)
- ❌ **Don't restate implementation internals.** how-it-works/README are user-facing; describe observable
  behavior (hooks run, scoped to the snapshot, `--no-verify`, hook mode composes) — no throwaway-index /
  T_start / CAS internals beyond "scoped to the frozen snapshot". (gotcha)
