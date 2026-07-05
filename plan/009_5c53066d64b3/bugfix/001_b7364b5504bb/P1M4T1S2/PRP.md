---
name: "P1.M4.T1.S2 (Mode B docs sync) — Update README.md Features-table 'Multi-turn fallback' row for multi-turn path coverage"
description: |

  Mode-B documentation sync, README half. After P1.M2 (dry-run) and P1.M3 (hook) propagated the FR-T1
  multi-turn gate to all three generation loops, the README.md Features-table "Multi-turn fallback" row
  (line 68) is reviewed for whether it implies commit-path-only. This is a CONDITIONAL task: the contract
  (item_description §3) says "If the existing wording is already generic enough to cover all paths (it
  says 'stagehand' generically, not 'the commit path'), no change may be needed — document that decision
  in a brief comment and leave the row unchanged. If the wording implies commit-path-only, update it."

  DECISION (research/design-decisions.md §0–§3): the row is ALREADY generic enough. Its subject is
  "stagehand" (the tool), not "the commit path"; it never says "on the commit path / when committing /
  snapshot / commit-tree / HEAD". The one commit-flavored phrase — "no extra commits" — is a UNIVERSALLY-
  TRUE anti-misconception note (multi-turn yields ONE message/commit delivered across turns, not N),
  accurate on all three paths (commit / --dry-run / hook). The authoritative docs-sync analysis
  (research_config_provider_docs.md §5c) does NOT flag the README row. And the row LINKS to
  docs/how-it-works.md#multi-turn-generation-fallback, which the parallel sibling (P1.M4.T1.S1) is updating
  to explicitly state all three paths — so the README (a deliberate high-level overview, "not a path-by-
  path reference") should stay generic and point there.

  THEREFORE the PRIMARY deliverable is: leave the row byte-unchanged + add a brief Markdown `<!-- -->`
  comment (in the gap before `## Install`) documenting the decision for future maintainers. A minimal
  4-word fallback edit (" on any generation path" after "lands") is specified IF a reviewer judges the
  current wording commit-path-only.

  DELIVERABLE (1 file modified — README.md; nothing else):
    - PRIMARY: add a `<!-- ... -->` comment in the blank line between the Features table and `## Install`,
      recording that the row is intentionally generic + where the path detail lives. The rendered table is
      byte-identical (the comment is invisible in rendered GitHub markdown).
    - FALLBACK (only if the reviewer disagrees): a 4-word insertion in the row (" on any generation path"
      after "lands") + the same comment. Do NOT enumerate paths in the row; do NOT add a sentence.

  CONTRACT (item_description §3–§5):
    - §3: review README.md:68; if generic enough (says "stagehand" not "the commit path"), no change —
      document in a brief comment; if commit-path-only, update to cover commit / --dry-run / hook. Keep
      changes minimal; README is high-level, not path-by-path. Do NOT add new sections.
    - §4 OUTPUT: "README.md features row accurately reflects multi-turn coverage. If no change was needed,
      that decision is documented."
    - §5: "This IS the documentation task (Mode B). No further docs subtask needed."

  SCOPE NOTE (the row is generic; §0/§1): the contract's own no-change criterion ("it says 'stagehand'
    generically, not 'the commit path'") is SATISFIED — the row's subject is "stagehand". research §5c
    (the authoritative docs-sync implications list) names how-it-works.md + configuration.md but NOT
    README.md ⇒ the auditor did not consider the row stale.

  SCOPE NOTE (README = pointer, how-it-works.md = detail, §2): the row links to
    docs/how-it-works.md#multi-turn-generation-fallback. P1.M4.T1.S1 adds the "runs on every generation
    path" sentence THERE. Duplicating the path enumeration in the README row would violate "not a path-by-
    path reference" and create a second sync site. Keep the row high-level.

  SCOPE BOUNDARY (what this does NOT do): NO code; NO tests; NO edits to docs/how-it-works.md (S1's scope),
    docs/configuration.md, docs/providers.md, docs/cli.md, docs/README.md (all already accurate per §5b/§5c
    or out of scope); NO new README sections; NO path enumeration in the row. This is a one-file,
    minimal-edit Mode-B docs task.

  INPUT (upstream — already built): the multi-turn gate on all three paths (P1.M2.T1.S2 dry-run at
    pkg/stagehand/stagehand.go:555; P1.M3.T1.S2 hook at internal/hook/exec.go:215; CommitStaged at
    internal/generate/generate.go:304). P1.M4.T1.S1 (parallel) updates docs/how-it-works.md to state the
    three-paths coverage explicitly. OUTPUT: the README features row is accurate (generic + the decision
    documented); no stale README claim remains.

  ⚠️ PRIMARY path = NO row-text change (the criterion is met, §0). Add ONLY the `<!-- -->` comment. Do NOT
     "improve" the row by enumerating paths — that violates the contract's "not a path-by-path reference".
  ⚠️ The comment CANNOT go inside the Markdown table (it would terminate the table and break rendering).
     Place it in the blank line between the Features table and `## Install`.
  ⚠️ Edit ONLY README.md. Do NOT touch docs/* (S1 owns how-it-works.md; others are accurate) or any .go file.

  Deliverable: 1 modified file (README.md); the rendered Features table byte-identical (primary) or a
  4-word row insertion (fallback); `go build ./... && go test ./...` green & unchanged (no code touched).

---

## Goal

**Feature Goal**: Ensure the README.md Features-table "Multi-turn fallback" row accurately reflects that
multi-turn runs on every generation path (snapshot commit / `--dry-run` / git hook) — by applying the
contract's conditional decision: the row is ALREADY generic enough (subject = "stagehand", not "the commit
path"), so leave the row text byte-unchanged and document the decision in a brief source comment. The
path-by-path detail is carried by the linked docs/how-it-works.md section (which P1.M4.T1.S1 updates to
state the three paths explicitly).

**Deliverable** (1 file modified — README.md):
- **PRIMARY**: a `<!-- … -->` comment placed in the blank line between the Features table and `## Install`,
  recording that the multi-turn row is intentionally generic, that multi-turn covers all three paths, that
  the path detail lives in the linked how-it-works.md section, and that "no extra commits" is an accurate
  anti-misconception note. The rendered Features table is byte-identical to before.
- **FALLBACK** (only if the reviewer judges the row implies commit-path-only): a 4-word insertion in the
  row — ` on any generation path` immediately after "lands" — PLUS the same comment. No path enumeration,
  no new sentence, no link changes.

**Success Definition**: the rendered Features table either (primary) is unchanged or (fallback) shows only
the 4-word broadening; the decision comment is present in the table→`## Install` gap; the two row links
resolve (`docs/how-it-works.md#multi-turn-generation-fallback` and `docs/configuration.md#built-in-defaults`);
`git status` shows ONLY README.md modified; NO `.go`/test/other-doc file touched; `go build ./... &&
go test ./...` green and unchanged (Mode B — no code touched).

## User Persona

**Target User**: The README reader (PRD §7.1 "the plan-holder") scanning the Features table to decide
whether multi-turn helps them — on a `stagehand` snapshot commit, a `stagehand --dry-run`, or a `git commit`
via the installed hook. After this sync (primary path), the row reads generically ("stagehand") and points
to a how-it-works.md section that explicitly covers all three paths; no reader concludes multi-turn is
commit-path-only.

**Use Case**: A user with a large diff + pi (append-mode) reads the Features row, clicks "how it works",
and lands on the multi-turn section (post-S1) which states it runs on commit / `--dry-run` / hook.

**User Journey**: README Features table → "Multi-turn fallback" row (generic, points to how-it-works.md) →
clicks the link → how-it-works.md multi-turn section (post-S1: "runs on every generation path") → knows the
behavior on their path.

**Pain Points Addressed**: Closes the doc-debt surface of the multi-turn propagation changeset at the
README (the most visible entry point). The row was already generic; this task CONFIRMS that and documents
it so it stays generic (a future maintainer won't accidentally narrow it or re-litigate).

## Why

- **It IS the README half of the Mode-B docs sync.** P1.M2/P1.M3 propagated multi-turn to dry-run + hook;
  P1.M4.T1.S1 syncs docs/how-it-works.md; this subtask syncs (or, per the decision, confirms) the README
  Features row. The bug-fix PRD §h2.0/§h2.3 + research §5b call out the README row as the surface to review.
- **Avoids over-editing a high-level doc.** The contract explicitly says the README is "a high-level
  overview, not a path-by-path reference." Enumerating paths in the row would duplicate how-it-works.md and
  create a second sync site. The generic row + the linked detail page is the correct division.
- **Documents the decision so it sticks.** A source comment records WHY the row is generic and WHERE the
  path detail lives — so a future maintainer doesn't narrow the row or re-open the question.
- **Trivial, isolated, no-risk.** One markdown file; no code, no tests, no other docs.

## What

Either (primary) a single Markdown comment added in the Features-table→`## Install` gap with the row text
unchanged, or (fallback) the same comment plus a 4-word row insertion. No code, no tests, no other doc
files, no new README sections, no path enumeration in the row.

### Success Criteria

- [ ] **PRIMARY**: the "Multi-turn fallback" row text (README.md:68) is byte-identical to before; a
      `<!-- … -->` comment is present in the blank line between the Features table and `## Install`.
- [ ] **FALLBACK** (if taken): the ONLY row-text change is the insertion of ` on any generation path`
      immediately after "lands" (4 words); no other row text, link, or table structure changed; the
      comment is also present.
- [ ] The comment states: the row is intentionally generic ("stagehand", not "the commit path"); multi-turn
      covers all three paths (snapshot commit / `--dry-run` / hook); the per-path detail lives in the linked
      docs/how-it-works.md section; "no extra commits" is an accurate anti-misconception note.
- [ ] The comment is NOT inside the Markdown table (it would break rendering) — it is in the table→`## Install` gap.
- [ ] The two row links still resolve: `docs/how-it-works.md#multi-turn-generation-fallback` and
      `docs/configuration.md#built-in-defaults`.
- [ ] `git status` shows ONLY README.md modified; NO `.go`/test/other-doc file touched.
- [ ] `go build ./... && go test ./...` GREEN and unchanged (regression no-op — no code touched).

## All Needed Context

### Context Completeness Check

_Pass._ A technical writer (or developer) with no prior repo knowledge can implement this from: the
verbatim current row text (quoted below), the decision (primary = comment only, no row change) + its
rationale (§0–§2), the exact comment text + placement (cannot go inside the table), the 4-word fallback
wording, and the LEAVE list (don't touch other docs / code). No multi-turn-internals knowledge required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M4T1S2/research/design-decisions.md
  why: the 6 decisions. §0 (the row says "stagehand" generically ⇒ the contract's no-change criterion is
       MET — verbatim row + per-phrase path-specificity table), §1 (research §5c does NOT flag the README
       row ⇒ the auditor didn't consider it stale), §2 (README = high-level POINTER; how-it-works.md carries
       the path detail — S1 adds the "three paths" sentence THERE), §3 (PRIMARY: comment only, exact text +
       placement in the table→## Install gap), §4 (FALLBACK: 4-word " on any generation path" insertion,
       ceiling of acceptable change), §5 (Mode B validation), §6 (coordination with S1 — different files,
       no conflict).
  critical: §0 (the decision criterion is met — do NOT change the row text on the primary path), §2 (do NOT
       enumerate paths in the row — that's how-it-works.md's job), §3 (the comment CANNOT go inside the
       Markdown table — place it in the table→## Install gap).

# MUST READ — the authoritative docs-sync analysis (confirms the README row is NOT flagged)
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/docs/architecture/research_config_provider_docs.md
  section: "## 5. DOCS — multi-turn documentation surface" / §5b (quotes the README:68 row verbatim) / §5c
       ("Docs-sync implications" — lists how-it-works.md + configuration.md; does NOT mention README.md).
  why: §5b is the verbatim source of the current row text (cross-check your edit against it). §5c's SILENCE
       on README.md is independent corroboration that the row was already accurate/generic (the auditor
       documented it in §5b but did not flag it for change in §5c).
  critical: §5c does NOT list README.md among the sync needs ⇒ primary path (no row change) is correct.

# MUST READ — the parallel sibling (P1.M4.T1.S1) contract (different file; composes with this task)
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M4T1S1/PRP.md
  section: edit (d) — the "Multi-turn runs on every generation path — the snapshot commit flow, `--dry-run`,
           and hook mode" sentence appended to the how-it-works.md multi-turn intro; edit (a) — the hook-mode
           multi-turn note.
  why: S1 edits docs/how-it-works.md ONLY; this task edits README.md ONLY — NO conflict (§6). S1's edit (d)
       is what makes the README row's generic pointer land on a page that DOES state the three-paths
       coverage. Do NOT duplicate S1's path enumeration in the README row (§2).
  critical: do NOT touch docs/how-it-works.md (S1's scope). The README row LINKS to S1's section — that's
       the composition.

# MUST READ — the file being edited (the verbatim row + its placement)
- file: README.md   (EDIT; the ONLY file)
  section: the "## Features" table, row 2 ("Multi-turn fallback", ~line 68). Verbatim current text:
           "| Multi-turn fallback | Lossless multi-turn fallback: when a one-shot generation of a large diff
           fails, stagehand re-delivers the full diff across session turns so the message still lands — no
           truncation, no extra commits ([how it works](docs/how-it-works.md#multi-turn-generation-fallback)
           · [knobs](docs/configuration.md#built-in-defaults)). |"
           The table ends at the "| Discovery | … |" row, followed by a BLANK line, then "## Install".
  why: the EXACT row text (cross-check byte-identity on the primary path) + the placement anchor for the
       comment (the blank line between the table and `## Install`).
  critical: the comment goes in that blank-line gap, NOT inside the table (a `<!-- -->` line between table
           rows terminates the table). On the primary path the row text is byte-identical; on the fallback
           path the only change is ` on any generation path` inserted after "lands".

# Confirms the three-paths reality (grounds the comment's coverage claim)
- note: "the FR-T1 multi-turn gate lives at internal/generate/generate.go:304 (CommitStaged),
         pkg/stagehand/stagehand.go:555 (runPipeline / --dry-run), and internal/hook/exec.go:215 (hook.Run).
         All three paths have it ⇒ the row's generic 'stagehand' wording covers all three."

- url: (Bug-Fix PRD §h2.0 Overview + §h2.3 Minor Issues / Issue 2 — in context as selected_prd_content;
       also plan/009…/bugfix/001_b7364b5504bb/prd_snapshot.md.)
  why: the bug context — multi-turn was only in CommitStaged; P1.M2/P1.M3 propagated it to dry-run + hook.
       This task is the README review for that changeset (Mode B).
```

### Current Codebase tree (relevant slice)

```bash
README.md              # *** EDIT *** — the Features-table "Multi-turn fallback" row (~L68) + the comment in
                       #                the table→## Install gap. The ONLY file.
docs/how-it-works.md   # READ ONLY — P1.M4.T1.S1's scope (the multi-turn + hook-mode sections). Do NOT touch.
docs/configuration.md  # READ ONLY — multi-turn knobs already accurate (§5b; do NOT touch).
docs/providers.md      # READ ONLY — session_mode subsection already accurate (§5b; do NOT touch).
docs/cli.md            # READ ONLY (no multi-turn edit needed).
docs/README.md         # READ ONLY (docs index; no multi-turn capability line — §5b).
# NO .go / test / config / go.mod changes. Mode B (docs only).
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. ONE in-place edit: README.md.
# PRIMARY: add a <!-- --> comment in the Features-table→## Install gap; row text byte-identical.
# FALLBACK: same comment + a 4-word insertion (" on any generation path" after "lands") in the row.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (PRIMARY path = NO row-text change, design §0/§3): the contract's no-change criterion ("it
     says 'stagehand' generically, not 'the commit path'") is MET — the row's subject is "stagehand". Do NOT
     change the row text on the primary path. Add ONLY the <!-- --> comment. "Improving" the row by
     enumerating paths violates "not a path-by-path reference" (§2). -->

<!-- CRITICAL (the comment CANNOT go inside the Markdown table, design §3): a <!-- --> line placed between
     table rows TERMINATES the table and breaks rendering. Place the comment in the BLANK line between the
     Features table (ends at the | Discovery | row) and "## Install". -->

<!-- CRITICAL (touch ONLY README.md, design §5/§6): docs/how-it-works.md is P1.M4.T1.S1's scope (parallel);
     docs/configuration.md / providers.md are already accurate (§5b); no .go/test files (Mode B). `git
     status` must show exactly ONE file. -->

<!-- GOTCHA (anchor the comment by CONTENT, not line number): line 68 drifts if anything above changes.
     Anchor on the "Multi-turn fallback" row text + the "| Discovery |" row + the "## Install" header. -->

<!-- GOTCHA (the row links must keep resolving): the two row links — docs/how-it-works.md#multi-turn-
     generation-fallback and docs/configuration.md#built-in-defaults — are correct and must be preserved
     byte-for-byte. (how-it-works.md's anchor is reinforced by S1's edit (d); it resolves.) -->

<!-- GOTCHA ("no extra commits" is NOT a scoping claim, design §0): it is a universally-true anti-
     misconception note (multi-turn yields ONE message/commit across turns, NOT N) — accurate on commit
     (one), dry-run (zero), and hook (one via git). Do NOT delete it thinking it scopes the row to commits. -->

<!-- GOTCHA (GitHub renders <!-- --> comments as invisible): the comment is visible in the README SOURCE
     (for maintainers) but NOT in the rendered Features table — so the primary path leaves the rendered
     user-facing table byte-identical. -->
```

## Implementation Blueprint

### Data models and structure

```markdown
<!-- NO data models. This is a one-row markdown-docs review + a comment (or a 4-word row insertion). -->
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: REVIEW README.md:68 (the "Multi-turn fallback" row) — apply the decision
  - OPEN README.md; locate the "## Features" table; find the "Multi-turn fallback" row (~line 68). Read its
      full text (quoted verbatim in "Documentation & References" / research §5b).
  - APPLY the contract's criterion (§3): does the row say "stagehand" generically (not "the commit path")?
      — YES. Does it enumerate or imply a single path? — NO (the subject is "stagehand"; "no extra commits"
      is an anti-misconception note, not a scoping claim — §0). ⇒ PRIMARY path: NO row-text change.
  - DECISION: take the PRIMARY path (Task 2). Take the FALLBACK (Task 3) ONLY if you (or a reviewer) judge
      "so the message still lands — no extra commits" reads as commit-path-only despite §0.

Task 2: PRIMARY — add the decision comment in the Features-table→## Install gap (row unchanged)
  - LOCATE the blank line BETWEEN the end of the Features table (the "| Discovery | … |" row) and the
      "## Install" header.
  - INSERT (in that blank-line gap) this comment, verbatim:
      <!-- Multi-turn fallback (Features row above): intentionally generic — "stagehand" re-delivers, NOT
           "the commit path". Multi-turn runs on EVERY generation path (snapshot commit, `--dry-run`, hook
           mode); the per-path detail lives in docs/how-it-works.md#multi-turn-generation-fallback (linked
           from the row), so this high-level row deliberately does NOT enumerate paths. "no extra commits"
           is an anti-misconception note (one message/commit, not N), accurate on all three paths. Do not
           narrow this row. (P1.M4.T1.S2.) -->
  - KEEP the "Multi-turn fallback" row byte-identical (do NOT change its text, links, or table structure).
  - GOTCHA: the comment goes in the table→## Install gap, NOT between table rows (a comment line inside the
      table terminates it). Verify the table still renders (the `| Discovery |` row is the last row, then
      the blank line + comment, then `## Install`).
  - SKIP Task 3 (the fallback). Go to Task 4.

Task 3: FALLBACK (only if the reviewer judges the row commit-path-only) — 4-word row insertion + comment
  - EDIT the "Multi-turn fallback" row: insert " on any generation path" IMMEDIATELY after "lands":
      BEFORE: …stagehand re-delivers the full diff across session turns so the message still lands — no truncation, no extra commits…
      AFTER:  …stagehand re-delivers the full diff across session turns so the message still lands on any generation path — no truncation, no extra commits…
  - ADD the Task 2 comment (the decision documentation applies either way).
  - GOTCHA: this is the CEILING of acceptable change. Do NOT enumerate "commit / --dry-run / hook" in the
      row (that's how-it-works.md's job — §2). Do NOT add a sentence. Do NOT touch the links. The 4-word
      phrase " on any generation path" is the only row-text delta.

Task 4: VERIFY (Mode B validation)
  - OPEN README.md; confirm the Features table renders correctly and the "Multi-turn fallback" row text is
      either (primary) byte-identical to research §5b's quote or (fallback) differs ONLY by the 4-word phrase.
  - Confirm the `<!-- … -->` comment is present in the table→## Install gap and is invisible in the rendered
      table (GitHub hides HTML comments).
  - Confirm the two row links resolve: docs/how-it-works.md#multi-turn-generation-fallback (exists — S1's
      edit (d) reinforces it) and docs/configuration.md#built-in-defaults.
  - `git status --porcelain` → ONLY README.md modified.
  - `go build ./... && go test ./...` → GREEN and unchanged (regression no-op — no code touched).
  - Confirm docs/how-it-works.md, docs/configuration.md, docs/providers.md, docs/cli.md, docs/README.md are
      byte-unchanged (`git diff --exit-code` on each).
```

### Implementation Patterns & Key Details

```markdown
<!-- THE decision (one sentence): the row already says "stagehand" generically ⇒ the contract's no-change
     criterion is met ⇒ PRIMARY = comment only, row unchanged. The path detail is in the LINKED
     how-it-works.md section (S1's edit (d) states all three paths there). -->

<!-- THE comment (primary deliverable), placed in the Features-table→## Install gap (NOT in the table):
     <!-- Multi-turn fallback (Features row above): intentionally generic — "stagehand" re-delivers, NOT
          "the commit path". Multi-turn runs on EVERY generation path (snapshot commit, `--dry-run`, hook
          mode); the per-path detail lives in docs/how-it-works.md#multi-turn-generation-fallback (linked
          from the row), so this high-level row deliberately does NOT enumerate paths. "no extra commits"
          is an anti-misconception note (one message/commit, not N), accurate on all three paths. Do not
          narrow this row. (P1.M4.T1.S2.) --> -->

<!-- THE fallback (only if the reviewer disagrees), 4 words, no path enumeration:
     "so the message still lands" → "so the message still lands on any generation path" -->

<!-- THE accuracy pins:
  - "stagehand" is the row's subject (generic) — NOT "the commit path". (decision criterion)
  - "no extra commits" is accurate on all 3 paths (commit: one; dry-run: zero; hook: one via git). (§0)
  - The row links to how-it-works.md#multi-turn-generation-fallback, which (post-S1) states all 3 paths. (§2)
  - The comment is invisible in rendered GitHub markdown (HTML comment). -->
```

### Integration Points

```yaml
README.md (the ONLY edit):
  - PRIMARY: +a <!-- --> comment in the Features-table→## Install gap; "Multi-turn fallback" row byte-unchanged.
  - FALLBACK: +the same comment + " on any generation path" (4 words) inserted after "lands" in the row.

DOCS.LEFT-UNCHANGED (do NOT edit — other-scope / already accurate):
  - docs/how-it-works.md   # P1.M4.T1.S1's scope (parallel) — the multi-turn + hook-mode sections
  - docs/configuration.md  # multi-turn knobs already accurate (§5b)
  - docs/providers.md      # session_mode subsection already accurate (§5b)
  - docs/cli.md, docs/README.md   # no multi-turn edit needed (§5b: docs/README.md has no multi-turn line)

CODE.LEFT-UNCHANGED: NO .go / test / config / go.mod / Makefile / PRD.md changes (Mode B — docs only).

UPSTREAM (the changeset this docs task reflects — already built, do NOT re-do):
  - P1.M2.T1.S2 (dry-run runPipeline gate, stagehand.go:555) + P1.M3.T1.S2 (hook gate, exec.go:215) +
    CommitStaged gate (generate.go:304) — all three paths carry the FR-T1 multi-turn gate.
  - P1.M4.T1.S1 (parallel) — adds the "runs on every generation path" sentence + hook-mode note to
    docs/how-it-works.md (the page the README row links to).

DOWNSTREAM: none. This is the README end-state for the multi-turn propagation changeset (Mode B).
```

## Validation Loop

### Level 1: Markdown sanity

```bash
cd /home/dustin/projects/stagehand

# Confirm the comment landed in the table→## Install gap (NOT inside the table):
grep -n "Multi-turn fallback (Features row above)" README.md   # → the comment, AFTER the | Discovery | row
# Confirm the Features table still has exactly its rows (the comment did NOT terminate it mid-table):
grep -nE '^\| (Multi-turn fallback|Discovery|Payload optimization|Message shaping|Git hook mode)' README.md
# Expected: the rows are intact and contiguous; the comment sits AFTER the last row (| Discovery |).

# PRIMARY path: confirm the row text is byte-identical to research §5b's quote:
grep -n "stagehand re-delivers the full diff across session turns so the message still lands" README.md
# Expected (primary): the line ends "…so the message still lands — no truncation…" (NO "on any generation path").
# Expected (fallback): the line reads "…so the message still lands on any generation path — no truncation…".

# Confirm the two row links are preserved byte-for-byte:
grep -n 'how-it-works.md#multi-turn-generation-fallback' README.md   # → present in the row
grep -n 'configuration.md#built-in-defaults' README.md               # → present in the row
```

### Level 2: Scope check (ONLY README.md changed)

```bash
git status --porcelain
# Expected: exactly ONE modified file — README.md. NOTHING else.
git diff --exit-code docs/how-it-works.md docs/configuration.md docs/providers.md docs/cli.md docs/README.md \
  && echo "other docs UNCHANGED (expected)"
# Expected: "other docs UNCHANGED" — S1's how-it-works.md is NOT touched by this task.
```

### Level 3: Regression no-op (no code touched)

```bash
go build ./...   # Expect clean + unchanged (no .go file touched).
go test ./...    # Expect GREEN + unchanged (docs-only; no test touched).
git diff --exit-code go.mod go.sum Makefile PRD.md && echo "go.mod/Makefile/PRD UNCHANGED (expected)"
# Confirm NO source file changed:
! git diff --name-only | grep -E '\.go$' && echo "OK: no .go file modified (Mode B docs-only)"
```

### Level 4: Accuracy review (the decision + the row match the code reality)

```bash
# Verify the comment's coverage claim — all three paths carry the FR-T1 gate:
grep -rn "MultiTurnFallback && " internal/generate/generate.go pkg/stagehand/stagehand.go internal/hook/exec.go
# Expected: 3 matches (CommitStaged + runPipeline/dry-run + hook.Run) ⇒ "every generation path" is accurate.

# Confirm the README row's link target exists (and is reinforced by S1's edit (d)):
grep -nE "^## Multi-turn generation fallback" docs/how-it-works.md   # → the section header (anchor resolves)
# Expected: the header exists ⇒ #multi-turn-generation-fallback resolves.
# (If S1 has already landed, grep for S1's "runs on every generation path" sentence too:
grep -n "runs on every generation path" docs/how-it-works.md || echo "S1 not yet landed (expected if parallel)")
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean & unchanged; `go test ./...` GREEN & unchanged (docs-only — no code touched).
- [ ] `git status` shows EXACTLY ONE modified file: README.md. No `.go`/test/other-doc file touched.
- [ ] go.mod/go.sum/Makefile/PRD.md byte-unchanged.

### Feature Validation
- [ ] **PRIMARY**: the "Multi-turn fallback" row text is byte-identical to before (research §5b's quote); a
      `<!-- … -->` comment documenting the decision is present in the Features-table→`## Install` gap.
- [ ] **FALLBACK** (if taken): the ONLY row-text change is ` on any generation path` after "lands"; the
      comment is also present.
- [ ] The comment states the row is intentionally generic, that multi-turn covers all three paths, that the
      per-path detail is in the linked how-it-works.md section, and that "no extra commits" is accurate.
- [ ] The comment is NOT inside the Markdown table (it is in the table→`## Install` gap); the table renders.
- [ ] The two row links resolve: `docs/how-it-works.md#multi-turn-generation-fallback` + `docs/configuration.md#built-in-defaults`.

### Code Quality Validation
- [ ] The edit is minimal (primary: a comment, zero row-text change; fallback: a 4-word insertion) — no new
      section, no path enumeration in the row, no wholesale rewrite.
- [ ] The decision criterion (the row says "stagehand" generically) is applied and documented.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (other docs / code frozen).

### Documentation
- [ ] The README Features row is accurate (generic + the decision documented); the path detail lives in the
      linked docs/how-it-works.md section (P1.M4.T1.S1). (This IS the documentation task — Mode B.)

---

## Anti-Patterns to Avoid

- ❌ **Don't change the row text on the PRIMARY path.** The contract's no-change criterion ("says 'stagehand'
  generically, not 'the commit path'") is MET (§0); research §5c does NOT flag the README row (§1). Add ONLY
  the `<!-- -->` comment. The 4-word fallback is the ceiling, taken only if a reviewer disagrees. (§0/§3)
- ❌ **Don't enumerate paths in the row.** "commit / --dry-run / hook" belongs in docs/how-it-works.md (S1's
  edit (d) puts it there). The README is "a high-level overview, not a path-by-path reference" (contract §3);
  duplicating the enumeration creates a second sync site. (§2)
- ❌ **Don't put the comment inside the Markdown table.** A `<!-- -->` line between table rows TERMINATES the
  table and breaks rendering. Place it in the blank line between the Features table and `## Install`. (§3)
- ❌ **Don't edit any file other than README.md.** docs/how-it-works.md is P1.M4.T1.S1's scope (parallel);
  docs/configuration.md / providers.md are already accurate (§5b); no `.go`/test files (Mode B). (§5/§6)
- ❌ **Don't delete "no extra commits" thinking it scopes the row to commits.** It is a universally-true
  anti-misconception note (multi-turn yields ONE message/commit, not N) — accurate on commit (one), dry-run
  (zero), and hook (one via git). It does NOT imply commit-path-only. (§0)
- ❌ **Don't anchor on line numbers.** Line 68 drifts; anchor on the "Multi-turn fallback" row text + the
  "| Discovery |" row + the "## Install" header. (gotcha)
- ❌ **Don't add new README sections or sentences.** The contract says "only adjust the existing row if
  needed" + "Do NOT add new sections." The primary path adds a COMMENT (not a section); the fallback adds 4
  words to the existing row. (contract §3)
- ❌ **Don't restate implementation details.** The README is user-facing; the comment describes the coverage
  intent + where the detail lives, not code internals (no `FR-T1`, no `exec.go:215`, no `Run pipeline`).
  (gotcha)

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: this is a single-file, minimal-edit Mode-B docs task with a clear, decided primary path (no row-
text change + a comment). The current row text is quoted VERBATIM (from the file + cross-checked against
research §5b), the decision criterion is applied explicitly (the row says "stagehand" generically ⇒ no
change), the exact comment text + its safe placement (the table→`## Install` gap, NOT inside the table) are
specified, and the 4-word fallback is provided with exact before/after. The authoritative docs-sync
analysis (§5c) independently corroborates the no-row-change decision by NOT flagging the README. The one
residual judgment call — whether a reviewer reads "so the message still lands — no extra commits" as
commit-path-only — is resolved with a decisive primary recommendation (no change, §0) AND a fully-specified
fallback (the 4-word insertion, §4), so either path is a one-pass success. The parallel sibling (S1) edits a
DIFFERENT file (docs/how-it-works.md), so there is no merge hazard, and S1's "three paths" sentence is
exactly what makes the README row's generic pointer land on a page that states the coverage — the two tasks
compose cleanly. Validation is deterministic: `git status` = one file, the row links resolve, the table
renders, `go build/test` is a no-op.
