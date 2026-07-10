# P2.M2.T2.S1 research findings — freeze-hardening docs sync (README + how-it-works)

> DOCS-ONLY task (Mode B — the changeset-level documentation sync for Phase 2 freeze hardening).
> No Go source, no tests, no config. Three surgical markdown edits across two files.

## §0 What this task IS (and is NOT)

- **IS**: the [Mode B] changeset-level docs sync that makes the freeze-hardening work from
  P2.M1 + P2.M2.T1 read coherently across README.md and docs/how-it-works.md. The freeze story
  must cover FR-M1e (defense-in-depth empty-index re-assertion) AND the improved FR-M1c error
  messages (concept-by-title + plain-language phrasing + remedy) as part of the reliability story.
- **IS NOT**: re-documenting FR-M1b/M1c/M1d from scratch (already documented), duplicating the
  FR-M1e Trigger note that P2.M1.T2.S1 already added (L55), changing any Go/test/config code, or
  writing a new user-facing feature page. The item is explicit: "Keep it concise — this is an
  internal hardening, not a user-facing feature."

## §1 Contract = ITEM DESCRIPTION (authoritative)

From the item (logic (a) + (b)):
- **(a) docs/how-it-works.md**: "ensure the freeze enforcement section covers FR-M1e
  (defense-in-depth re-check) and the improved FR-M1c error messages as part of the reliability
  story. The Mode A note from P2.M1.T2.S1 may be sufficient — verify and extend if needed."
- **(b) README.md**: "if there's a reliability/safety section, add a brief note that decompose
  re-asserts its preconditions as defense-in-depth. Keep it concise."
- OUTPUT: "Documentation covers the freeze hardening coherently."
- DOCS: "[Mode B] This IS the changeset-level documentation sync task for Phase 2."

## §2 The freeze-hardening work being documented (from P2.M1 + P2.M2.T1)

- **P2.M1.T1.S1** (Complete): added `StagedNames()` git primitive (lists staged paths). No doc
  surface — internal plumbing.
- **P2.M1.T2.S1** (Complete): FR-M1e — `Decompose()` re-asserts the empty-index precondition at
  entry (after the escape-hatch, before FreezeWorkingTree), failing loudly with an actionable
  error naming the staged paths + `git reset` / `stagecoach --single` remedies. **Mode A doc**:
  added the "Defense-in-depth (FR-M1e)" note to `docs/how-it-works.md` "### Trigger" (L55).
- **P2.M1.T3.S1** (Complete): amended FR-M1c — improved `verifyFreezeSubset` error messages:
  names the concept BY TITLE via `conceptTitle string` (%q render), replaces the opaque
  "not traceable to T_start" with "frozen working-tree snapshot" phrasing, and adds the remedy
  suffix "This indicates concurrent working-tree changes were picked up by the stager. Aborting
  to protect the freeze boundary." **No docs-surface change in that PRP** (item DOCS: "none —
  error message improvement") — so the improved error quality is UNDOCUMENTED today. This is a
  genuine gap this task closes.
- **P2.M2.T1.S1** (in-flight, parallel): TESTS-ONLY — adds 3 test edits (conceptTitle-wiring
  test, FR-M1e substring, e2e routing-boundary S8). NO docs change, NO conflict with this task.

## §3 Current state of the two docs files (verified at HEAD)

### docs/how-it-works.md (402 lines)
The freeze story currently lives in THREE places:
- **L47–55 "### Trigger"** — has the FR-M1e note (P2.M1.T2.S1 Mode A). SUFFICIENT for the FR-M1e
  *mention*. KEEP AS-IS.
- **L113–116 "### Key design points"**:
  - L113 "Start-of-run freeze (T_start)" bullet — the freeze-capture explanation. KEEP AS-IS.
  - L116 "Freeze enforcement" bullet — covers ONLY the per-staging-step subset verification
    (FR-M1c core) + hard abort. **GAP: no FR-M1e, no improved error quality.** This is THE
    "freeze enforcement section" the item names; it must be extended.
- **L132 "### Safety" → "Start-of-run freeze" bullet** — cites FR-M1c + FR-M1d ("the stager is
  verified... after each staging step (FR-M1c), and the arbiter — the third freeze surface —
  ... (FR-M1d)"). **GAP: omits FR-M1e.** After this changeset the three-layer freeze surface is
  incoherent here (entry re-assertion missing from the enumeration). Small surgical fix.

### README.md (391 lines)
The freeze/safety story lives in:
- **L64 Features table row "Multi-commit decomposition"** — already mentions "A start-of-run
  freeze means a concurrent edit during the run can never enter a commit — including across the
  leftover-reconciliation arbiter." Dense table cell. NO change (too dense for defense-in-depth).
- **L151 "### Multi-commit decomposition" prose paragraph** (under Quick start) — detailed: T_start
  freeze + arbiter + stager constraints. **GAP: no defense-in-depth re-assertion note.** This is
  where the item's (b) note belongs (the most contextual reliability paragraph; the FAQ
  "Will it corrupt my repo?" is about snapshot+lock, not decompose-internal freeze).
- **L189 FAQ "Will it corrupt my repo?"** — snapshot + per-repo lock + orphan reclamation. NOT the
  right home for a decompose-internal defense-in-depth clause (would bloat a corruption-focused
  answer). The L151 prose section is the right home.

## §4 Markdown style / linting (validation tooling)

- `.markdownlint.json` exists: `{"default": true, "MD013": false, "MD033": false, "MD060": false}`
  → no line-length limit (MD013 off), inline HTML allowed (MD033 off), no sibling-heading rule
  (MD060 off). So long paragraphs and the existing bold-lead-in bullet style are fine.
- **markdownlint is NOT on PATH** and is NOT wired into the Makefile (`make lint` = golangci-lint
  only) or any CI workflow (`.github/workflows/*` has zero markdown references — verified).
- **Therefore docs edits have NO automated gate.** Validation = grep confirms the new content is
  present + the markdown structure is intact (no broken headings) + `make build`/`make test`
  UNAFFECTED (docs-only changes cannot break the Go build/tests).

## §5 The exact edits (verified oldText strings — for precise edit instructions)

### Edit 1 — docs/how-it-works.md L116 (the "Freeze enforcement" bullet, the core of the task)
OLD (verbatim, the whole bullet paragraph):
```
**Freeze enforcement.** Because the stager is an external agent running `git` against the live tree, after each staging step stagecoach verifies the resulting tree is a content-subset of T_start (only T_start paths, T_start content). Any deviation — a concurrent change swept in, or a stager that ran a bare `git add -A` — is a hard abort (non-rescue; already-landed commits stand per FR-M12).
```
NEW: rename to "**Freeze enforcement (defense-in-depth).**", prepend the two-layer framing + FR-M1e
entry re-assertion (cross-linked to #trigger), keep the existing per-step verification sentence,
and append the improved-error-quality clause (concept by title + plain-language cause + intentional
protection). Single coherent paragraph. (Exact wording in PRP §Implementation Tasks Task 1.)

### Edit 2 — docs/how-it-works.md L132 (the "Start-of-run freeze" Safety bullet)
OLD (verbatim):
```
- **Start-of-run freeze** — T_start captures the full working-tree change set at decompose activation; concurrent edits never enter any commit. The stager is verified as a content-subset of T_start after each staging step (FR-M1c), and the arbiter — the third freeze surface — derives its gate, its diff, and every tree it commits strictly from T_start and tipTree, never a live re-read (FR-M1d).
```
NEW: insert the three-layer framing — "The freeze boundary is held at three layers: the
empty-index precondition is re-asserted at entry (FR-M1e); the stager is verified as a content-
subset of T_start after each staging step (FR-M1c); and the arbiter ... (FR-M1d)." Makes the
enumeration coherent. (Exact wording in PRP §Implementation Tasks Task 2.)

### Edit 3 — README.md L151 (the Multi-commit decomposition prose paragraph)
Insert ONE concise sentence after the arbiter clause "(a concurrent edit can never sneak into a
commit)." and before "The planner partitions changes per file.": the defense-in-depth re-assertion
note (FR-M1e — empty-index precondition re-asserted at entry; stale trigger fails loudly rather
than silently folding hand-staged content). Focused on FR-M1e per the item's (b) ask; the deeper
freeze-enforcement + error-quality story stays in docs/how-it-works.md. (Exact wording in PRP
§Implementation Tasks Task 3.)

## §6 What NOT to touch (scope boundaries)

- **L55 docs/how-it-works.md** (the FR-M1e Trigger note from P2.M1.T2.S1) — SUFFICIENT, KEEP. This
  task cross-LINKS to it (Edit 1) and cites its FR (Edit 2); it does NOT duplicate or reword it.
- **README L64 Features table row** — already covers the freeze at the right density; a table cell
  is too dense for a defense-in-depth clause. Leave it.
- **README L189 FAQ "Will it corrupt my repo?"** — snapshot+lock+orphan; not decompose-internal
  freeze. Leave it (would bloat a corruption-focused answer).
- **docs/how-it-works.md L113 "Start-of-run freeze (T_start)" bullet** — the freeze-CAPTURE
  explanation; not the enforcement story. Leave it.
- **docs/how-it-works.md "### Safety" other bullets** (Atomic, Frozen content, No index resets) —
  unrelated to freeze enforcement. Leave them.
- **ANY Go source / test / config / PRD / tasks.json / prd_snapshot** — READ-ONLY / out of scope.
- **docs/cli.md, docs/configuration.md, docs/providers.md, docs/README.md** — no freeze-enforcement
  surface that needs this sync (providers.md:111 already has a defense-in-depth mention for the
  HEAD-movement guard, which is a different layer). Leave them.

## §7 Validation (no automated markdown gate exists)

```bash
# 1. Grep confirms the new content landed in the right sections.
grep -n "FR-M1e" docs/how-it-works.md          # Expect ≥3 hits now (L55 Trigger + Edit1 + Edit2)
grep -n "defense-in-depth\|Defense-in-depth\|defense in depth" docs/how-it-works.md  # Edit1 title + body
grep -n "concept by title\|names the concept" docs/how-it-works.md  # Edit1 error-quality clause
grep -n "FR-M1e\|defense-in-depth\|re-assert" README.md  # Edit3 (≥1 hit)
# 2. Markdown structure intact — the bullets/paragraphs are still well-formed.
grep -c "^- \*\*" docs/how-it-works.md          # bullet count unchanged except where intended
# 3. Go build/test UNAFFECTED (docs-only — sanity, must still pass).
make build && make test
# 4. Scope guard — ONLY the two doc files changed.
git status --porcelain   # Expect M README.md AND M docs/how-it-works.md (nothing else)
```
