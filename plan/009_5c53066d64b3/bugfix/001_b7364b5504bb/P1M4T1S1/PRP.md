---
name: "P1.M4.T1.S1 (Mode B docs sync) — Update docs/how-it-works.md hook-mode and multi-turn sections for multi-turn path coverage + the Issue-3 per-chunk estimate"
description: |

  Mode-B documentation sync. After P1.M2 (dry-run) and P1.M3 (hook) propagated the FR-T1 multi-turn gate
  to all three generation loops, `docs/how-it-works.md` is stale: its hook-mode section doesn't mention
  multi-turn at all, and the multi-turn section describes the feature purely in snapshot-flow terms. This
  subtask updates TWO sections of ONE file so the docs reflect that multi-turn now runs on every generation
  path (commit / `--dry-run` / hook) and that the fallback progress line reports the per-chunk token budget
  (Issue 3). No code, no tests, no other doc files.

  CONTRACT (item_description §3, three edits + one verify):
    (a) HOOK-MODE SECTION (how-it-works.md:300-324): add a concise (1-2 sentence) note that multi-turn
        fallback is now available in hook mode as an extra attempt before the never-block exit — on success
        the generated message is written to the commit-msg file; on any failure (turn error, parse empty,
        duplicate) the hook still exits 0 with the msg-file untouched (FR-H5 preserved).
    (b) MULTI-TURN SECTION (how-it-works.md:262-298): VERIFY the "`token_limit` does not apply (FR-T12)"
        paragraph still matches the corrected Issue-4 behavior (mtPayload is always rebuilt from the
        untruncated diff). It DOES — the docs are already accurate post-Issue-4; no change strictly needed.
    (c) (Optional) Add a one-line note that the fallback surface shows the per-chunk token estimate (Issue 3).
    Do NOT modify docs/configuration.md or docs/providers.md (already accurate).

  DELIVERABLE (1 file modified; nothing else):
    MODIFY `docs/how-it-works.md` — 3 concrete edits (a hook-mode note; a "three paths" coverage sentence in
    the multi-turn intro; a per-chunk-estimate sentence on the progress-line sentence) + 1 verify (the
    FR-T12 paragraph).

  SCOPE NOTE (the progress line is ALWAYS-ON, design §3): Issue 3 (P1.M1.T3.S1) put the per-chunk budget in
    the fallback progress line at `internal/generate/generate.go:340` — an UNCONDITIONAL `fmt.Fprintf(
    os.Stderr, …)` (every `↳ …` line is always-on per FR51b), NOT a `--verbose`-gated line. The separate
    `deps.Verbose.VerboseWarn(…)` (L343) is the verbose-gated trigger line; per-turn payload/raw-output
    detail lives inside `generate.Run`. So edit (c) is worded accurately: the chunk budget is in the
    always-on progress line; `--verbose` adds per-turn detail. (The task's "--verbose shows the per-chunk
    estimate" framing is slightly imprecise — the code shows it to everyone.)

  SCOPE NOTE (the "three paths" is real, design §4): all three generation loops now carry the FR-T1 gate —
    `internal/generate/generate.go:304` (CommitStaged), `pkg/stagecoach/stagecoach.go:555` (runPipeline /
    `--dry-run`), `internal/hook/exec.go:215` (hook.Run). Edit (a) covers the hook path; a one-sentence
    "runs on every generation path" note in the multi-turn intro makes the coverage explicit and
    cross-links to the hook section.

  SCOPE BOUNDARY (what this does NOT do): NO code changes; NO test changes; NO edits to
    docs/configuration.md, docs/providers.md, docs/cli.md, docs/README.md, or README.md (README features
    table is P1.M4.T1.S2's scope; the other docs are already accurate). This is a docs-only Mode-B sweep of
    ONE file.

  INPUT (upstream — already built): the multi-turn gate on all three paths (P1.M2.T1.S2 dry-run,
    P1.M3.T1.S2 hook); the Issue-3 per-chunk progress-line extension (P1.M1.T3.S1); the Issue-4 mtPayload
    rebuild (P1.M1.T2.S1). OUTPUT: `docs/how-it-works.md` reflects multi-turn across all three paths + the
    verbose/progress per-chunk estimate; no stale docs remain.

  ⚠️ Edit ONLY docs/how-it-works.md. Do NOT touch configuration.md / providers.md / README.md (task forbids;
     README is S2).
  ⚠️ Edit (c) wording: the per-chunk estimate is in the ALWAYS-ON progress line (generate.go:340 is an
     unconditional Fprintf), NOT verbose-only — word it accordingly.
  ⚠️ Edit (b) is a VERIFY/no-op — the FR-T12 paragraph is already accurate post-Issue-4; don't over-edit it.

  Deliverable: 1 modified file (docs/how-it-works.md); `go build ./... && go test ./...` green & unchanged
  (no code touched); `git status` shows ONLY that file.

---

## Goal

**Feature Goal**: Sync `docs/how-it-works.md` to the post-propagation reality: the FR-T1 multi-turn
fallback now runs on all three generation paths (snapshot commit / `--dry-run` / hook), and the fallback
progress line reports the per-chunk token budget (Issue 3). Close the two doc gaps the propagation opened —
the hook-mode section never mentioned multi-turn, and the multi-turn section described the feature as
snapshot-flow-only — so no stale documentation remains.

**Deliverable** (1 file modified):
- `docs/how-it-works.md` — 3 concrete edits + 1 verify:
  1. **(a) Hook-mode section** — a concise "Multi-turn fallback in hook mode" note (2 sentences).
  2. **(d) Multi-turn intro** — a one-sentence "runs on every generation path" coverage note (cross-links
     to the hook section). [Delivers the OUTPUT's "three paths" goal.]
  3. **(c) Progress-line sentence** — a one-sentence extension noting the per-chunk token budget +
     `--verbose` per-turn detail (FR-T11).
  4. **(b) FR-T12 paragraph** — VERIFY it's still accurate (it is); no change required.

**Success Definition**: the hook-mode section names multi-turn and its FR-H5 composition; the multi-turn
section states it runs on all three paths; the progress-line sentence mentions the per-chunk budget; the
FR-T12 paragraph is confirmed accurate; markdown anchors resolve; `git status` shows ONLY
docs/how-it-works.md; `go build ./... && go test ./...` green and unchanged (no code touched).

## User Persona

**Target User**: The reader of `docs/how-it-works.md` (PRD §7.1 "the plan-holder", and integrators) who
needs to know whether multi-turn helps them on THEIR path — a `git commit` via the installed hook, a
`stagecoach --dry-run`, or the snapshot `stagecoach` command. Today the hook-mode section implies multi-turn
is unimplemented there; after this sync the docs match the code.

**Use Case**: A user with a large diff + pi configured runs `git commit` (firing the hook) and wants to
know if multi-turn will help. They read the hook-mode section and see: yes, multi-turn is tried as an extra
attempt, and if it fails the commit still proceeds (never-block).

**User Journey**: user opens docs/how-it-works.md → reads the multi-turn section (now notes all three
paths) → reads the hook-mode section (now notes multi-turn composes with never-block) → knows the exact
behavior on their path → configures/trusts accordingly.

**Pain Points Addressed**: Stale docs — the hook-mode section listed the hook contract without multi-turn,
so a hook user would wrongly conclude multi-turn is unavailable; and the multi-turn section read as
snapshot-flow-only. Both gaps closed.

## Why

- **It IS the Mode-B docs half of the propagation changeset.** P1.M2 (dry-run) and P1.M3 (hook) wired the
  gate into the other two loops; the docs were not updated. This subtask closes that doc debt (the bug-fix
  PRD §h2.0/§h2.3 + research_hook_exec.md §7 explicitly call it out).
- **Prevents user confusion.** A hook user reading the old hook-mode section would believe multi-turn never
  fires on `git commit` — wrong post-propagation. The note (a) corrects this.
- **Captures the Issue-3 surface.** The per-chunk token budget now appears in the progress line; FR-T11 is
  satisfied. Documenting it (edit c) tells users the chunking budget is visible/verifiable.
- **Trivial, isolated, no-risk.** One markdown file; no code, no tests, no other docs.

## What

Three small markdown additions + one verification in `docs/how-it-works.md`, and nothing else. No code, no
tests, no config, no CLI, no other doc files.

### Success Criteria

- [ ] **(a)** The hook-mode section (`## Hook mode vs the snapshot-based flow`, L300) contains a
      "Multi-turn fallback in hook mode" note (≈2 sentences) placed after the Hook-mode bullets and before
      `### When to use which` (L320), stating: multi-turn is tried as an extra attempt before never-block;
      on success the message is written; on any failure (turn error / empty parse / duplicate) the hook
      exits 0 with the msg-file untouched (FR-H5).
- [ ] **(d)** The multi-turn section intro contains a one-sentence note that multi-turn runs on every
      generation path (snapshot commit / `--dry-run` / hook mode), cross-linking to the hook section.
- [ ] **(c)** The "Each turn is a separate provider invocation … surfaced on the progress line at fallback
      time." sentence is extended (or immediately followed by a sentence) noting the progress line reports
      the per-chunk token budget, and `--verbose` adds per-turn payload/raw-output detail (FR-T11).
- [ ] **(b)** The "`token_limit` does not apply (FR-T12)" paragraph is READ and confirmed accurate
      (mtPayload rebuilt from the untruncated diff — matches the Issue-4 fix). No change required; if a
      precision clause is added it stays user-facing (no retryInstr/preamble implementation detail).
- [ ] Markdown anchors resolve (`#multi-turn-generation-fallback`, `#hook-mode-vs-the-snapshot-based-flow`).
- [ ] `git status` shows ONLY `docs/how-it-works.md` modified; NO `.go`/test/other-doc file touched.
- [ ] `go build ./... && go test ./...` GREEN and unchanged (regression no-op — no code touched).

## All Needed Context

### Context Completeness Check

_Pass._ A technical writer (or developer) with no prior repo knowledge can implement this from: the 3
verbatim edits + their exact placement anchors (quoted below), the §3 accuracy note (progress line is
always-on), the §2 verify (FR-T12 already accurate), and the LEAVE list (don't touch other docs). No
git/Go/multi-turn-internals knowledge required — the edits are self-contained markdown.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M4T1S1/research/design-decisions.md
  why: the 6 decisions. §0 (scope: ONLY how-it-works.md), §1 (edit (a) verbatim text + placement), §2 (edit
       (b) is a VERIFY/no-op — FR-T12 already accurate post-Issue-4), §3 (edit (c) — progress line is
       ALWAYS-ON, word it accurately, not "verbose-only"), §4 (edit (d) — the "three paths" sentence), §5
       (Mode-B validation: docs review + git status + go build/test no-op).
  critical: §3 (the progress line at generate.go:340 is unconditional — the chunk budget is NOT verbose-
       only; word edit (c) to match), §0 (touch ONLY how-it-works.md; configuration.md/providers.md/README
       are forbidden/other-scope), §2 (don't over-edit the FR-T12 paragraph).

# MUST READ — the file being edited (the two sections, exact anchors)
- file: docs/how-it-works.md   (EDIT; the ONLY file)
  section: `## Multi-turn generation fallback` (L262) — intro paragraph (ends “…delivered in smaller
           pieces.”) is where edit (d) appends; the “Each turn is a separate provider invocation … surfaced
           on the progress line at fallback time.” sentence is where edit (c) appends; the
           “`token_limit` does not apply (FR-T12).” paragraph (end of section) is edit (b)’s verify target.
           `## Hook mode vs the snapshot-based flow` (L300) — the Hook-mode bullets block (ends with the
           “No rescue protocol” bullet “…the commit simply proceeds without an AI message.”) is where
           edit (a) inserts, before `### When to use which` (L320).
  why: the EXACT placement anchors for the 3 edits. Reference by CONTENT (the bullet/sentence text), not
       brittle line numbers (line numbers drift if anything above changes).
  critical: edit (a) goes AFTER the Hook-mode bullets, BEFORE `### When to use which`. Edit (d) appends to
       the multi-turn intro paragraph. Edit (c) appends to the "surfaced on the progress line" sentence.

# MUST READ — the hook gate contract (grounds edit (a)'s FR-H5 wording)
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M3T1S2/PRP.md
  section: the gate writes the msg-file ONLY on `cause==nil && ok2 && !duplicate`; every failure (turn
           error, empty final parse, duplicate subject) falls through to the exhaustion error → cmd
           neverBlock → exit 0 + untouched msg-file (or exit 1 if --strict).
  why: edit (a)'s "on any failure … exits 0 with the message file untouched (FR-H5 preserved)" must match
       this exactly. The hook has NO rescue (git owns the commit) — do NOT imply a rescue path.
  critical: edit (a) must say "exits 0 with the message file untouched" (FR-H5), NOT "enters rescue".

# The doc-debt this discharges (confirms edit (a) is the right call)
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/docs/architecture/research_hook_exec.md
  section: "## 7. Docs / FAQ on multi-turn in hook mode" — "the hook-mode section (how-it-works.md:300-324)
           should note that multi-turn is available in hook mode too (as an extra attempt, not a rescue — it
           composes with never-block). This is an accuracy update."
  why: the authoritative statement of the doc debt. Edit (a) is its resolution.

# Confirms configuration.md / providers.md are ALREADY accurate (do NOT touch)
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/docs/architecture/research_config_provider_docs.md
  section: "## 5. DOCS — multi-turn documentation surface" / §5b — docs/configuration.md (L110-111/137-138/
           155-157) and docs/providers.md (L29/40-49) document the multi-turn knobs + session_mode gate
           accurately and are NOT path-specific ⇒ need no change.
  why: the task forbids editing them; this confirms they're already correct, so the prohibition is safe.
  critical: do NOT edit configuration.md or providers.md.

# The accuracy anchor for edit (c) — the progress line is UNCONDITIONAL (always-on)
- file: internal/generate/generate.go   (READ ONLY — do NOT edit)
  section: L340 — `fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens),
           ~%dm total\n", turns, cfg.MultiTurnChunkTokens, totalMin)` is UNCONDITIONAL (no verbose gate).
           L343 `deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")` is the verbose-gated
           line. Per-turn payload/raw-output detail is emitted inside `generate.Run` (via provider.Execute).
  why: edit (c) MUST say the chunk budget is in the (always-on) progress line — visible to ALL users — and
       `--verbose` adds per-turn detail. The task's "--verbose shows the per-chunk estimate" is imprecise;
       the code shows it to everyone. Word edit (c) to match the code.
  critical: do NOT edit generate.go. Read L340 to confirm the always-on behavior before wording edit (c).

# Confirms the "three paths" claim (grounds edit (d))
- note: "the FR-T1 gate lives at internal/generate/generate.go:304 (CommitStaged), pkg/stagecoach/stagecoach.go:555
         (runPipeline / --dry-run), and internal/hook/exec.go:215 (hook.Run). All three paths have it ⇒ edit
         (d)'s 'runs on every generation path' is accurate."

- url: (Bug-Fix PRD §h2.0 Overview + §h2.3 Issue 2 + §h3.1 Issue 3 — in context as selected_prd_content;
       also plan/009…/bugfix/001_b7364b5504bb/prd_snapshot.md.)
  why: the bug context — multi-turn was only in CommitStaged; P1.M2/P1.M3 propagated it to dry-run + hook;
       Issue 3 added the per-chunk estimate. This subtask is the docs sync for that changeset.
```

### Current Codebase tree (relevant slice)

```bash
docs/
  how-it-works.md      # *** EDIT *** — multi-turn section (L262) + hook-mode section (L300). The ONLY file.
  configuration.md     # READ ONLY — multi-turn knobs already accurate (do NOT touch; task forbids).
  providers.md         # READ ONLY — session_mode subsection already accurate (do NOT touch; task forbids).
  cli.md               # READ ONLY (no multi-turn edit needed here).
  README.md            # READ ONLY (index; no edit).
README.md              # READ ONLY — the features table is P1.M4.T1.S2's scope (do NOT touch).
# NO .go / test / config / go.mod changes. Mode B (docs only).
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. ONE in-place edit: docs/how-it-works.md (3 markdown additions + 1 verify).
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (touch ONLY docs/how-it-works.md, design §0): the task explicitly forbids editing
     docs/configuration.md and docs/providers.md (both already accurate — research_config_provider_docs.md
     §5b confirmed). README.md is P1.M4.T1.S2's scope. `git status` must show ONLY how-it-works.md. -->

<!-- CRITICAL (edit (c) accuracy — progress line is ALWAYS-ON, design §3): the per-chunk token budget at
     generate.go:340 is an UNCONDITIONAL fmt.Fprintf(os.Stderr,…) (every `↳ …` line is always-on, FR51b) —
     NOT --verbose-gated. The VerboseWarn (L343) is the verbose-gated line. So edit (c) must say the chunk
     budget is in the always-on progress line (visible to everyone), and --verbose adds per-turn detail.
     Do NOT write "--verbose shows the per-chunk estimate" (imprecise — the code shows it to all users). -->

<!-- CRITICAL (edit (a) FR-H5 wording — no rescue in hook mode, design §1): the hook has NO rescue (git owns
     the commit). Edit (a) must say "exits 0 with the message file untouched (FR-H5 preserved)", NOT "enters
     rescue" or "prints a recovery command". Failure → never-block → empty editor. (P1.M3.T1.S2 contract.) -->

<!-- GOTCHA (reference anchors by CONTENT, not line number): line numbers (262/300/320/340) drift if
     anything above changes. Anchor each edit on the unique surrounding sentence/bullet text
     (e.g. the "No rescue protocol" bullet, the "surfaced on the progress line" sentence). -->

<!-- GOTCHA (markdown anchor slugs): GitHub lowercases + hyphenates + strips punctuation for headers.
     `## Multi-turn generation fallback` → `#multi-turn-generation-fallback`;
     `## Hook mode vs the snapshot-based flow` → `#hook-mode-vs-the-snapshot-based-flow`. Use these exact
     slugs in the cross-links (edits (a) and (d)). -->

<!-- GOTCHA (edit (b) is a VERIFY, design §2): the FR-T12 paragraph is ALREADY accurate post-Issue-4 (the
     fix made mtPayload rebuild from `diff`, so "delivers the untruncated diff" is now correct). Read it,
     confirm, leave it. The task's "captured ONCE and unmodified" is a paraphrase — that exact phrase is NOT
     in the file (grep confirmed; the only "captured" hit is an unrelated arbiter line at L119). Don't
     over-edit; if you add a precision clause, keep it user-facing (no retryInstr/preamble internals). -->

<!-- GOTCHA (edit (d) cross-link target): the "Hook mode" cross-link in the multi-turn intro must point to
     `#hook-mode-vs-the-snapshot-based-flow` (the section header slug), which EXISTS (L300). Verify it
     renders. -->
```

## Implementation Blueprint

### Data models and structure

```markdown
<!-- NO data models. This is a markdown-docs task. The "structure" is the 4 edits + their exact anchors. -->
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/how-it-works.md — edit (a): the hook-mode multi-turn note (the main edit)
  - LOCATE the "## Hook mode vs the snapshot-based flow" section (L300). Find the Hook-mode bullets block
      (Pre-commit hooks honored / No snapshot guarantees / Never-block contract / No rescue protocol). The
      "No rescue protocol" bullet ends with “…the commit simply proceeds without an AI message.”
  - INSERT a new paragraph IMMEDIATELY AFTER that bullets block and BEFORE "### When to use which" (L320):
      **Multi-turn fallback in hook mode.** The [multi-turn fallback](#multi-turn-generation-fallback) is
      available in hook mode too: on a large diff with an append-mode provider, the hook tries it as one
      extra attempt before the never-block exit. On success the generated message is written to the
      commit-message file; on any failure — a turn error, an empty final parse, or a duplicate subject —
      the hook still exits 0 with the message file untouched (FR-H5 preserved).
  - GOTCHA: this is the load-bearing accuracy fix (the section previously implied multi-turn is
      unimplemented in hook mode). Keep it to ~2 sentences (the task says "concise"). Anchor slug
      `#multi-turn-generation-fallback` must match the multi-turn header.

Task 2: EDIT docs/how-it-works.md — edit (d): the "three paths" coverage sentence in the multi-turn intro
  - LOCATE the multi-turn section intro paragraph (the one ending “…the model can handle the same content
      delivered in smaller pieces.”).
  - APPEND one sentence to that paragraph:
      Multi-turn runs on every generation path — the snapshot commit flow, `--dry-run`, and hook mode
      (where it composes with the never-block contract; see [Hook mode](#hook-mode-vs-the-snapshot-based-flow) below).
  - GOTCHA: this delivers the OUTPUT's "across all three paths" goal explicitly. Cross-link slug
      `#hook-mode-vs-the-snapshot-based-flow` must match the hook header (L300). One sentence only.

Task 3: EDIT docs/how-it-works.md — edit (c): the per-chunk-estimate note (Issue 3 / FR-T11)
  - LOCATE the sentence “Each turn is a separate provider invocation with its own timeout; total wall-clock
      ≈ `timeout × (N+1)`, surfaced on the progress line at fallback time.”
  - APPEND one sentence immediately after it:
      That progress line also reports the per-chunk token budget each chunk targets; with `--verbose`, each
      turn additionally prints its payload size and raw agent output (FR-T11).
  - GOTCHA: word it ACCURATELY — the chunk budget is in the ALWAYS-ON progress line (generate.go:340 is an
      unconditional Fprintf), NOT verbose-only. `--verbose` adds the per-turn detail. Do NOT write
      "--verbose shows the per-chunk estimate" (imprecise).

Task 4: VERIFY docs/how-it-works.md — edit (b): the FR-T12 paragraph (no change required)
  - LOCATE the “**`token_limit` does not apply (FR-T12).**” paragraph (end of the multi-turn section).
  - READ it and CONFIRM it is accurate: it says multi-turn “re-captures the diff with `token_limit`
      disabled and delivers the untruncated diff across the N+1 turns” and “the re-capture is skipped when
      `token_limit` is unset.” This MATCHES the Issue-4 fix (mtPayload is always rebuilt from `diff` via
      BuildUserPayload, never reused from the one-shot `payload`). NO change needed.
  - OPTIONAL (only if you want extra precision): append a short user-facing clause noting the multi-turn
      payload is rebuilt fresh from the captured diff. Do NOT introduce the retryInstr/preamble
      implementation detail (too deep for how-it-works.md). Default: leave the paragraph as-is.
  - GOTCHA: the task's "captured ONCE and unmodified" is a paraphrase — that exact phrase is NOT in the
      file. Don't go hunting for it; the paragraph is accurate.

Task 5: VERIFY (Mode B validation)
  - Open docs/how-it-works.md; confirm the 4 edits landed at the right anchors and render correctly
      (anchors resolve; no broken markdown).
  - `git status --porcelain` → ONLY docs/how-it-works.md modified.
  - `go build ./... && go test ./...` → GREEN and unchanged (regression no-op — no code touched).
  - Confirm docs/configuration.md, docs/providers.md, docs/cli.md, docs/README.md, README.md are
      byte-unchanged (`git diff --exit-code` on each).
```

### Implementation Patterns & Key Details

```markdown
<!-- THE 4 edits, summarized (anchor → action):
  (a) after Hook-mode bullets / before "### When to use which" → INSERT the "Multi-turn fallback in hook
      mode" paragraph (2 sentences; FR-H5 wording; cross-link to #multi-turn-generation-fallback).
  (d) end of multi-turn intro paragraph → APPEND the "runs on every generation path" sentence (cross-link
      to #hook-mode-vs-the-snapshot-based-flow).
  (c) after the "surfaced on the progress line at fallback time." sentence → APPEND the per-chunk-budget +
      --verbose-per-turn-detail sentence (FR-T11).
  (b) the FR-T12 paragraph → VERIFY (accurate; no change). Optional precision clause only if desired.

<!-- THE accuracy pins (don't get these wrong):
  - Hook failure = exit 0 + untouched msg-file (FR-H5 never-block). NOT a rescue. (edit a)
  - The chunk budget is in the ALWAYS-ON progress line (generate.go:340 unconditional Fprintf), not
    verbose-only. --verbose adds per-turn detail. (edit c)
  - "Three paths" = CommitStaged (generate.go:304) + runPipeline/dry-run (stagecoach.go:555) + hook.Run
    (exec.go:215). All three confirmed. (edit d)

<!-- THE anchor slugs (GitHub markdown):
  - `## Multi-turn generation fallback`        → #multi-turn-generation-fallback
  - `## Hook mode vs the snapshot-based flow`  → #hook-mode-vs-the-snapshot-based-flow
```

### Integration Points

```yaml
DOCS.FILE (the ONLY edit): docs/how-it-works.md — multi-turn section (L262) + hook-mode section (L300).

DOCS.LEFT-UNCHANGED (do NOT edit — task forbids / other-scope):
  - docs/configuration.md   # multi-turn knobs (L110-111/137-138/155-157) already accurate (§5b confirmed)
  - docs/providers.md       # session_mode subsection (L29/40-49) already accurate (§5b confirmed)
  - docs/cli.md, docs/README.md   # no multi-turn edit needed
  - README.md               # the features table is P1.M4.T1.S2's scope

CODE.LEFT-UNCHANGED: NO .go / test / config / go.mod / Makefile / PRD.md changes (Mode B — docs only).

UPSTREAM (the changeset this docs task reflects — already built, do NOT re-do):
  - P1.M2.T1.S2 (dry-run runPipeline gate at stagecoach.go:555) + P1.M3.T1.S2 (hook gate at exec.go:215).
  - P1.M1.T3.S1 (Issue 3: per-chunk budget in the progress line, generate.go:340).
  - P1.M1.T2.S1 (Issue 4: mtPayload rebuilt from diff ⇒ FR-T12 paragraph already accurate).

DOWNSTREAM: none. This is the docs end-state for the multi-turn propagation changeset (Mode B). P1.M4.T1.S2
      (README features table) is the only sibling docs task.
```

## Validation Loop

### Level 1: Markdown sanity

```bash
# Confirm the 4 edits landed at the right anchors:
grep -n "Multi-turn fallback in hook mode" docs/how-it-works.md                 # edit (a) present
grep -n "runs on every generation path" docs/how-it-works.md                   # edit (d) present
grep -n "per-chunk token budget each chunk targets" docs/how-it-works.md       # edit (c) present
grep -n "token_limit.*does not apply" docs/how-it-works.md                     # edit (b) paragraph still present
# Confirm the cross-link slugs match real headers (anchors resolve):
grep -nE "^## (Multi-turn generation fallback|Hook mode vs the snapshot-based flow)" docs/how-it-works.md
# Expected: all 4 edits present; the two headers exist (so #multi-turn-generation-fallback and
# #hook-mode-vs-the-snapshot-based-flow resolve). Read the rendered file (or a markdown previewer) to confirm.
```

### Level 2: Scope check (ONLY how-it-works.md changed)

```bash
git status --porcelain
# Expected: exactly ONE modified file — docs/how-it-works.md. NOTHING else.
git diff --exit-code docs/configuration.md docs/providers.md docs/cli.md docs/README.md README.md \
  && echo "other docs UNCHANGED (expected)"
```

### Level 3: Regression no-op (no code touched)

```bash
go build ./...   # Expect clean + unchanged (no .go file touched).
go test ./...    # Expect GREEN + unchanged (docs-only; no test touched).
git diff --exit-code go.mod go.sum Makefile PRD.md && echo "go.mod/Makefile/PRD UNCHANGED (expected)"
# Confirm NO source file changed:
! git diff --name-only | grep -E '\.go$' && echo "OK: no .go file modified (Mode B docs-only)"
```

### Level 4: Accuracy review (the 4 edits match the code reality)

```bash
# Verify the docs claims match the code (read-only checks):
#   1. edit (a) FR-H5: the hook writes ONLY on success; failure → exit 0 + untouched. (P1.M3.T1.S2 contract.)
grep -n "WriteMessageFile\|neverBlock" internal/hook/exec.go internal/cmd/hookexec.go | head
#   2. edit (c): the progress line is UNCONDITIONAL + includes the chunk budget.
grep -n 'falling back to multi-turn' internal/generate/generate.go   # → the unconditional Fprintf at ~L340
#   3. edit (d): all three paths have the gate.
grep -rn "MultiTurnFallback && " internal/generate/generate.go pkg/stagecoach/stagecoach.go internal/hook/exec.go
#   4. edit (b): the FR-T12 paragraph is accurate (mtPayload from diff — Issue 4 already landed).
grep -n "mtPayload := prompt.BuildUserPayload" internal/generate/generate.go
# Expected: each grep confirms the code behavior the docs now describe. (No edits to code — read-only proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean & unchanged; `go test ./...` GREEN & unchanged (docs-only — no code touched).
- [ ] `git status` shows EXACTLY ONE modified file: docs/how-it-works.md. No `.go`/test/other-doc file touched.
- [ ] go.mod/go.sum/Makefile/PRD.md byte-unchanged.

### Feature Validation
- [ ] **(a)** Hook-mode section has the "Multi-turn fallback in hook mode" note (2 sentences; FR-H5 wording:
      on success writes; on any failure exits 0 + msg-file untouched).
- [ ] **(d)** Multi-turn intro notes it runs on every generation path (commit / `--dry-run` / hook), with a
      cross-link to the hook section.
- [ ] **(c)** The progress-line sentence notes the per-chunk token budget (always-on) + `--verbose` per-turn
      detail (FR-T11), worded accurately (not "verbose-only").
- [ ] **(b)** The FR-T12 paragraph confirmed accurate (mtPayload rebuilt from diff — Issue 4); no stale claim.
- [ ] Markdown anchors resolve (`#multi-turn-generation-fallback`, `#hook-mode-vs-the-snapshot-based-flow`).

### Code Quality Validation
- [ ] The 4 edits are minimal and anchored on existing content (no wholesale rewrites); each is 1–2 sentences.
- [ ] Edit (a)/(d) cross-links use the correct GitHub anchor slugs.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (other docs / code frozen).

### Documentation
- [ ] docs/how-it-works.md now reflects multi-turn availability across all three paths + the per-chunk
      estimate; no stale documentation remains. (This IS the documentation task — Mode B.)

---

## Anti-Patterns to Avoid

- ❌ **Don't edit any file other than docs/how-it-works.md.** The task forbids docs/configuration.md and
  docs/providers.md (both already accurate — §5b confirmed); README.md is P1.M4.T1.S2's scope; no `.go`/test
  files (Mode B). `git status` must show exactly one file. (§0)
- ❌ **Don't word edit (c) as "verbose-only".** The per-chunk budget is in the ALWAYS-ON progress line
  (generate.go:340 is an unconditional `fmt.Fprintf`); `--verbose` adds per-turn detail. Writing "--verbose
  shows the per-chunk estimate" is imprecise — the code shows it to everyone. (§3)
- ❌ **Don't imply a rescue path in edit (a).** The hook has NO rescue (git owns the commit). Failure →
  never-block → exit 0 + untouched msg-file (FR-H5). Say "exits 0 with the message file untouched", NOT
  "enters rescue" or "prints a recovery command". (§1 / P1.M3.T1.S2 contract)
- ❌ **Don't over-edit the FR-T12 paragraph (edit b).** It's already accurate post-Issue-4 (the fix made
  mtPayload rebuild from `diff`, so "delivers the untruncated diff" is correct). Verify and leave it; if you
  add a precision clause, keep it user-facing (no retryInstr/preamble internals). (§2)
- ❌ **Don't anchor edits on line numbers.** Line numbers drift; anchor on the unique surrounding
  sentence/bullet text (e.g. the "No rescue protocol" bullet, the "surfaced on the progress line" sentence).
  (gotcha)
- ❌ **Don't invent markdown anchor slugs.** Use GitHub's lowercasing + hyphenation: `## Multi-turn
  generation fallback` → `#multi-turn-generation-fallback`; `## Hook mode vs the snapshot-based flow` →
  `#hook-mode-vs-the-snapshot-based-flow`. Verify they resolve. (gotcha)
- ❌ **Don't bloat edit (a).** The task says "concise (1-2 sentences)." Don't turn it into a sub-section;
  one short paragraph after the Hook-mode bullets is the right size. (§1)
- ❌ **Don't restate implementation details.** how-it-works.md is user-facing; edits (a)/(c)/(d) describe
  observable behavior, not code internals (no `cause==nil && ok2`, no `BuildUserPayload`, no `VerboseWarn`).
  (gotcha)
