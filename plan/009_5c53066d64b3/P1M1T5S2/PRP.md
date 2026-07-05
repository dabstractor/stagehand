---
name: "P1.M1.T5.S2 — Confirm FUTURE_SPEC.md consistency (lossless multi-turn graduated; lossy chunking still rejected)"
description: |
  A Mode B **documentation-consistency confirmation** (not an expected edit). Open
  `FUTURE_SPEC.md` and verify it matches the shipped lossless multi-turn behavior in
  PRD §9.24 (FR-T1–T12): (a) the lossless multi-turn fallback is NOT listed as a
  rejected/deferred idea — it graduated into §9.24; (b) the lossy "chunk-summarize-
  combine" map-reduce form IS still listed as rejected, with its rationale (degrades
  message quality); (c) the disambiguating NOTE's language is non-contradictory with
  §9.24 (lossless, full diff across request-sized session turns, message-role only,
  gated on session_mode="append"). If consistent → NO edit (record the confirmation in
  the subtask result). If inconsistent → make the minimal corrective edit so the file
  matches §9.24.

  CONTRACT (from the work item — do not exceed):
  1. This is a CONFIRMATION. `plan/009_5c53066d64b3/delta_prd.md` line 79 states the
     file is ALREADY updated; no edit is expected. An edit is a fallback ONLY if a real
     inconsistency is found.
  2. INPUT = PRD §9.24 (shipped lossless behavior) + §10.5 (deferred/rejected ideas
     live in FUTURE_SPEC.md).
  3. OUTPUT = FUTURE_SPEC.md confirmed consistent with §9.24 (or minimally corrected).
  4. DOCS = this IS the doc task (Mode B). Ships no code, no CLI flags, no config keys.
  5. SCOPE = `FUTURE_SPEC.md` ONLY. The sibling task P1.M1.T5.S1 owns `README.md`; do
     not touch it. Do not touch `docs/`, source, PRD.md, or tasks.json.

  Deliverable: a verified `FUTURE_SPEC.md` (unchanged if consistent — the expected
  case) plus a confirmation note in the subtask result. Fallback: a minimal
  single-row corrective edit with the canonical wording supplied below.
---

## Goal

**Feature Goal**: Guarantee `FUTURE_SPEC.md` (Stagehand's deferred/rejected-ideas
registry, PRD §10.5) is **consistent** with the lossless multi-turn fallback that
shipped as PRD §9.24 (FR-T1–T12, built in P1.M1.T1–T4). Specifically: the lossless
multi-turn form must read as *graduated* (pointing at §9.24), and the lossy
chunk-summarize-combine form must remain *rejected* with its rationale — so a future
reader does not re-litigate either idea from scratch (FUTURE_SPEC's stated purpose).

**Deliverable**: `FUTURE_SPEC.md` left **byte-identical** to its current state IF the
three consistency conditions hold (the expected outcome — verified at PRP-authoring
time), OR a **minimal corrective edit** to the single relevant row
(`FUTURE_SPEC.md:99`, §3 Rejected table) IF an inconsistency is found. Either way, a
short **confirmation note** is added to the subtask result stating which path was
taken and why.

**Success Definition**:
- The three contract conditions (a/b/c below) are each verified PASS by the PRP's
  Level 1 grep suite, AND
- `git diff --stat -- FUTURE_SPEC.md` is **empty** (confirmation path — no edit), OR
  the diff is a **single-line edit** to row 99 that makes all three conditions pass
  (correction path), AND
- No file other than `FUTURE_SPEC.md` is modified by this task.

## User Persona (if applicable)

**Target User**: A future maintainer (human or agent) opening `FUTURE_SPEC.md` to
decide whether "large-diff chunking" or "multi-turn" is worth re-proposing. Transitively
PRD §7.1 "the plan-holder" / the spec's own audience.

**Use Case**: A future revision wants to add large-diff handling and checks
FUTURE_SPEC.md first to avoid re-litigating a closed decision.

**User Journey**: Opens FUTURE_SPEC.md → reads §3 Rejected → sees the "Large-diff
chunking — lossy map-reduce form" row → reads the NOTE that the *lossless* form
graduated to §9.24 → concludes: "lossy is permanently out; lossless already shipped,
go read §9.24." No ambiguity, no duplicate spec.

**Pain Points Addressed**: Without the graduation NOTE, a future reader would see
"Large-diff chunking … rejected" and wrongly conclude the *entire* chunking idea
(including the shipped lossless multi-turn) is off the table — or, conversely, would
re-propose the lossless form as novel and re-reject it as chunking. The NOTE prevents
both regressions.

## Why

- **FUTURE_SPEC.md exists to prevent re-litigation** (its header: *"so future
  revisions don't re-litigate them from scratch"*). The lossless multi-turn fallback
  (§9.24) and the lossy chunking rejection share a conceptual neighborhood
  ("handling large diffs"), so the file must crisply separate *rejected (lossy)* from
  *shipped (lossless)* — or it fails its purpose.
- **The feature already shipped (P1.M1.T1–T4 complete)**; §9.24 is in the PRD. If
  FUTURE_SPEC still described multi-turn as rejected/deferred, the two docs would
  contradict — a documentation defect that confuses every future reader.
- **Scope discipline (Mode B).** This is a read-mostly task. The contract forbids
  expanding into re-specifying §9.24 internals here (FUTURE_SPEC's own footer: *"an
  idea must live in exactly one of the two documents"*). The lossless *spec* stays in
  §9.24; only the lossy *rejection* + a disambiguating pointer live here.

## What

A **verification** of three conditions against `FUTURE_SPEC.md`, with a corrective
fallback:

1. **(a) Lossless multi-turn is NOT a rejected/deferred idea.** No row/entry in §1
   (Deferred), §2 (Blocked), or §3 (Rejected) lists the *lossless* multi-turn form as
   rejected or deferred. It appears only as a graduation NOTE pointing to §9.24.
2. **(b) Lossy map-reduce chunk-summarize-combine IS still rejected, with rationale.**
   The §3 row exists, its rejected form is precisely scoped to the lossy
   "summarize-each-chunk-then-combine" flavor, and the rationale ("degrades message
   quality", "permanently rejected") is present.
3. **(c) The NOTE's language is non-contradictory with §9.24.** The graduation note
   says "lossless … full diff delivered across request-sized session turns … graduated
   to the spec — see PRD §9.24 (FR-T1–T12) … applies only to the lossy form" — i.e. it
   does not misdescribe the shipped behavior as lossy/truncating/summarizing.

**Expected outcome (baseline verified at PRP-authoring time): all three PASS, file is
already consistent → NO edit.** An edit is made only if a condition fails.

### Success Criteria

- [ ] Condition (a) verified PASS: zero §1/§2/§3 entries titled "multi-turn" or an
      un-scoped "chunking"; the only chunking row is scoped to "lossy map-reduce form".
- [ ] Condition (b) verified PASS: §3 row present with "summarize-each-chunk-then-
      combine" + "degrades message quality" + "permanently rejected".
- [ ] Condition (c) verified PASS: the NOTE contains "graduated to the spec", a
      pointer to "PRD §9.24 (FR-T1–T12)", and scopes the rejection to "only to the
      lossy form"; nothing in the row contradicts §9.24.
- [ ] `git diff --stat -- FUTURE_SPEC.md` is empty (confirmation) OR a single-line
      corrective edit to row 99 that makes all conditions pass.
- [ ] No file other than `FUTURE_SPEC.md` is touched.
- [ ] A confirmation note is recorded in the subtask result (which path + why).

## All Needed Context

### Context Completeness Check

_Pass._ A writer with no prior knowledge of the repo can do this from: the exact line
to verify (`FUTURE_SPEC.md:99`), the exact §9.24 cross-reference (`PRD.md:491`), the
three conditions stated as grep checks, and (for the fallback) the canonical correct
row text. No Go toolchain, no build, no tests — it is a documentation read-verify
(with a single-line edit fallback).

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- file: FUTURE_SPEC.md
  section: "§3. Rejected — deliberate, with reasons" (the table; the relevant row is line 99)
  why: (1) This is the ONLY file in scope and the ONLY place chunking/multi-turn is mentioned.
       (2) The row at line 99 is the single artifact under verification. (3) Its header (§3) and
       the file's footer rule ("an idea must live in exactly one of the two documents") define
       what "consistent" means: the lossy rejection lives here; the lossless spec lives in §9.24;
       the NOTE is a pointer, not a second copy.
  pattern: each §3 row = `| **<feature> (<competitor>)** | <rationale> |` (2-column table).
  gotcha: the row is ONE logical markdown line (tables do not wrap). Any corrective edit must
          keep it a single line with exactly 2 cells (`| … | … |`) — no stray `|` inside the
          rationale cell, and the em-dashes (—) and curly quotes (“ ”) must be preserved verbatim
          if only part of the row is edited (they are part of the existing voice).

- file: FUTURE_SPEC.md
  section: "§1. Deferred" (lines ~13–48) and "§2. Blocked" (lines ~50–58)
  why: Condition (a) requires confirming NO deferred/blocked entry mentions multi-turn/chunking.
       These sections must be scanned (the PRP's grep does it programmatically). A false positive
       here (e.g. a stray "chunking" mention) would be an inconsistency to correct.

- prd: PRD.md §9.24 "Multi-turn generation fallback (lossless large-diff priming)" (line 491)
  why: This is the shipped spec the NOTE points at. It is the source of truth for "lossless",
       "full diff … request-sized session turns", "message role only" (FR-T10), and the
       session_mode="append" gate (FR-T1d/FR-T8). Cross-walk the NOTE's claims against these FRs.
  critical: the NOTE in FUTURE_SPEC need NOT re-state FR-T10 (message-role) or FR-T1d/FR-T8
            (session_mode gate) — those are §9.24 internals. The consistency check is for
            NON-CONTRADICTION, not re-statement. Do NOT "enrich" the NOTE by adding these;
            that would violate the footer's "exactly one document" rule and expand scope.

- prd: PRD.md §10.5 "Beyond this document" (line 545)
  why: Establishes that deferred/rejected ideas live in FUTURE_SPEC.md — i.e. this file is the
        canonical home for the lossy rejection, which is why it must stay correct.

- doc: plan/009_5c53066d64b3/delta_prd.md line 79 ("Confirm (not edit): FUTURE_SPEC.md already
       carries the lossless-multi-turn-graduation note … no edit expected.")
  why: The governing contract for this subtask — it declares the expected outcome is a
       CONFIRMATION, bounding the task to read-verify with a corrective fallback only.

- research: plan/009_5c53066d64b3/P1M1T5S2/research/future_spec_baseline.md
  why: The baseline audit captured at PRP-authoring time — proves all three conditions PASS on
       the current file, with the exact grep outputs and a claim-by-claim cross-walk to §9.24 FRs.
       The implementing agent re-runs the checks and compares to this baseline.
```

### Current Codebase tree (relevant slice)

```bash
FUTURE_SPEC.md            # VERIFY (§3 row at line 99); EDIT only if a condition fails
PRD.md                    # READ-ONLY — §9.24 (line 491) is the source of truth
plan/009_5c53066d64b3/delta_prd.md   # READ-ONLY — line 79 is the task contract
README.md                 # DO NOT TOUCH (sibling task P1.M1.T5.S1 owns it)
docs/                     # DO NOT TOUCH (out of scope for this subtask)
```

### Desired Codebase tree with files to be added

```bash
FUTURE_SPEC.md            # UNCHANGED (confirmation) OR +1 single-line corrective edit (fallback)
# No new files. No code. No tests. Mode B (doc-only).
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL: the EXPECTED outcome is NO edit. The file is already consistent (verified at PRP
     authoring — see research/future_spec_baseline.md). Do not "improve" the NOTE by adding
     §9.24 internals (N+1, message-role, session_mode) — that re-specifies §9.24 here and violates
     FUTURE_SPEC's own footer ("an idea must live in exactly one of the two documents"). The NOTE's
     job is to disambiguate lossy-rejected from lossless-shipped, nothing more. -->

<!-- CRITICAL: "consistent" means NON-CONTRADICTORY, not exhaustive. The NOTE correctly omits
     FR-T10 (message-role only) and FR-T1d/FR-T8 (session_mode="append" gate). Their absence is
     NOT an inconsistency — they are §9.24 internals. Only flag a condition-(c) failure if the
     NOTE actively MISDESCRIBES the shipped behavior (e.g. calls the lossless form "lossy",
     "truncating", or "summarizing"). -->

<!-- CRITICAL: the §3 row is a SINGLE markdown table line with exactly 2 cells. A corrective edit
     must preserve: (1) the title cell `**Large-diff chunking — lossy map-reduce form** (aicommits)`,
     (2) the 2-cell structure `| … | … |`, (3) the em-dash (—) and curly-quote (“ ”) typography.
     Do not split the row across lines or add a third column. -->

<!-- CRITICAL: scope = FUTURE_SPEC.md ONLY. Do NOT edit README.md (sibling S1), docs/, PRD.md,
     tasks.json, or any source. If you believe §9.24 itself is wrong, that is OUT OF SCOPE —
     record it as a finding and stop; do not edit PRD.md. -->

<!-- MINOR: `.markdownlint.json` has MD013 (line-length) OFF — the row is intentionally long and
     that is fine; do not wrap it. -->

<!-- MINOR: git baseline. At PRP-authoring time `git diff --stat -- FUTURE_SPEC.md` is EMPTY
     (the file is committed in its consistent state). The confirmation path leaves it empty. -->
```

## Implementation Blueprint

### Data models and structure

None. This is a documentation read-verify (with a single-line edit fallback). No data
models, no code.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY condition (a) — lossless multi-turn is NOT rejected/deferred
  - FILE: FUTURE_SPEC.md (READ-ONLY for this task).
  - RUN the Level 1 grep suite for condition (a):
      * §3 rows titled "multi-turn"   ⇒ expect 0
      * §3 rows titled "chunking"     ⇒ expect 1 (scoped to "lossy map-reduce form")
      * §1 Deferred multi-turn/chunking mentions ⇒ expect 0
      * §2 Blocked  multi-turn/chunking mentions ⇒ expect 0
  - PASS CRITERION: the ONLY chunking/multi-turn mention in the file is the §3 row at
    line 99, whose title is explicitly scoped to "lossy map-reduce form", and the
    lossless form appears ONLY inside that row's NOTE as a graduation pointer.
  - OUTPUT: record PASS/FAIL for (a). If FAIL, note exactly which stray entry exists.

Task 2: VERIFY condition (b) — lossy form rejected with rationale
  - RUN: grep line 99 for "summarize-each-chunk-then-combine" + "degrades message
    quality" + "permanently rejected". All three must be present on line 99.
  - PASS CRITERION: the §3 row scopes the rejection to the lossy
    "summarize-each-chunk-then-combine" flavor and states it is permanently rejected
    because it degrades message quality.
  - OUTPUT: record PASS/FAIL for (b).

Task 3: VERIFY condition (c) — NOTE non-contradictory with §9.24
  - RUN: grep line 99 for "graduated to the spec", "see PRD §9.24", "FR-T1–T12",
    "applies only to the lossy form", "lossless", "request-sized session turns".
  - CROSS-WALK the NOTE's claims against PRD §9.24 (line 491): lossless (FR-T2),
    full diff across request-sized session turns (FR-T2/T4/T6), graduated pointer
    (§9.24 exists), rejection scoped to lossy only (FR-T2 contrast).
  - PASS CRITERION: every NOTE claim is consistent with §9.24; NONE misdescribes the
    shipped behavior as lossy/truncating/summarizing. (Absence of FR-T10/FR-T1d/FR-T8
    is NOT a failure — see Gotchas.)
  - CONFIRM the §9.24 anchor exists: `grep -n "^### 9.24 Multi-turn generation
    fallback" PRD.md` ⇒ line 491.
  - OUTPUT: record PASS/FAIL for (c).

Task 4: DECIDE — confirm (no edit) vs. correct (minimal edit)
  - IF (a) AND (b) AND (c) all PASS:
      * Make NO edit. `git diff --stat -- FUTURE_SPEC.md` stays empty.
      * This is the EXPECTED path (baseline verified consistent).
      * Go to Task 6.
  - IF any condition FAILS:
      * Go to Task 5 (corrective edit). Record WHICH condition failed and the exact
        defect (stray entry / missing rationale / misdescribed NOTE).

Task 5 (FALLBACK, only if a condition failed): CORRECT FUTURE_SPEC.md:99 minimally
  - FILE: FUTURE_SPEC.md, §3 Rejected table, the "Large-diff chunking" row (line 99).
  - TARGET STATE — make the row match this canonical text EXACTLY (this is the
    verified-consistent wording; replace the entire row line with it):
      | **Large-diff chunking — lossy map-reduce form** (aicommits) | The *summarize-each-chunk-then-combine* flavor degrades message quality and is permanently rejected. NOTE: a **lossless** multi-turn priming form (full diff delivered across request-sized session turns) has graduated to the spec — see PRD §9.24 (FR-T1–T12). The rejection above applies only to the lossy form; the original premise ("agent contexts are 200k+; byte caps bound the payload") is withdrawn — a provider's per-request reliability ceiling can fall well below its advertised window, which is exactly what §9.24 addresses. |
  - MINIMAL-EDIT RULE: prefer the smallest edit that fixes the actual defect over a
    full-row replacement when the defect is local (e.g. if only the rationale phrase
    is missing, add just that phrase). Use the full-row replacement above only if the
    row has drifted broadly. Keep it ONE line, 2 cells, em-dashes/curly quotes intact.
  - IF the defect is a DUPLICATE/stray entry elsewhere (e.g. a second §1 or §3 row
    naming lossless multi-turn as rejected): DELETE that stray entry (the lossy row at
    line 99 is the canonical home). Do not leave two chunking rows.
  - DO NOT add §9.24 internals (N+1, message-role, session_mode) to the NOTE.
  - PRESERVE: every other row, the §1/§2/§3 structure, the footer rule line.

Task 6: RECORD the confirmation note in the subtask result
  - STATE which path was taken: "CONFIRMED consistent — no edit" (expected) or
    "CORRECTED — minimal edit to FUTURE_SPEC.md:99" (fallback).
  - LIST the three condition verdicts (a/b/c) with the grep evidence.
  - IF corrected, paste the single-line `git diff` for FUTURE_SPEC.md.
```

### Implementation Patterns & Key Details

```markdown
<!-- The canonical CONSISTENT row (FUTURE_SPEC.md:99). Use as the target state for any
     corrective edit; otherwise it is the row you are verifying. One markdown line, 2 cells. -->

| **Large-diff chunking — lossy map-reduce form** (aicommits) | The *summarize-each-chunk-then-combine* flavor degrades message quality and is permanently rejected. NOTE: a **lossless** multi-turn priming form (full diff delivered across request-sized session turns) has graduated to the spec — see PRD §9.24 (FR-T1–T12). The rejection above applies only to the lossy form; the original premise ("agent contexts are 200k+; byte caps bound the payload") is withdrawn — a provider's per-request reliability ceiling can fall well below its advertised window, which is exactly what §9.24 addresses. |

<!-- Claim-by-claim audit map (so a reviewer can verify condition (c) without re-reading §9.24):
  - "summarize-each-chunk-then-combine flavor … permanently rejected"  → the LOSSY form (rejected).
  - "lossless multi-turn priming form … graduated to the spec"          → the LOSSLESS form (shipped, §9.24).
  - "full diff delivered across request-sized session turns"            → FR-T2 (lossless) + FR-T4 (N+1) + FR-T6 (append session).
  - "see PRD §9.24 (FR-T1–T12)"                                         → the graduated pointer (anchor verified at PRD.md:491).
  - "applies only to the lossy form"                                    → scopes the rejection; lossless is NOT rejected.
  - "original premise … is withdrawn … per-request reliability ceiling" → §9.24 intro (169K-token one-shot failure vs 200K window).
  None of these contradict §9.24; none misdescribe the shipped behavior. → condition (c) PASS.
-->
```

### Integration Points

```yaml
DOCS (the only integration surface):
  - FUTURE_SPEC.md: UNCHANGED (confirmation) OR +1 single-line corrective edit to the §3
    "Large-diff chunking" row (fallback). Nothing else.

NO SOURCE / NO CLI / NO CONFIG SCHEMA / NO README / NO docs/ / NO PRD.md / NO tasks.json.
This is a doc-consistency confirmation. Mode B (doc-only).
```

## Validation Loop

### Level 1: Consistency checks (the core of this task)

```bash
# --- CONDITION (a): lossless multi-turn is NOT a rejected/deferred idea ---
# §3 rows whose title cell names "multi-turn" (expect 0 — only "chunking" appears, scoped to lossy):
awk '/^## 3\. Rejected/,/^---$/' FUTURE_SPEC.md | grep -cE '^\| \*\*[^|]*[Mm]ulti-turn'
# Expected: 0

# §3 rows whose title cell names "chunking" (expect 1 — scoped to "lossy map-reduce form"):
awk '/^## 3\. Rejected/,/^---$/' FUTURE_SPEC.md | grep -cE '^\| \*\*[^|]*[Cc]hunking'
# Expected: 1

# §1 Deferred mentioning multi-turn/chunking (expect 0):
awk '/^## 1\. Deferred/,/^## 2\. Blocked/' FUTURE_SPEC.md | grep -ic "multi-turn\|chunking"
# Expected: 0

# §2 Blocked mentioning multi-turn/chunking (expect 0):
awk '/^## 2\. Blocked/,/^## 3\. Rejected/' FUTURE_SPEC.md | grep -ic "multi-turn\|chunking"
# Expected: 0

# --- CONDITION (b): lossy form rejected with rationale (all on line 99) ---
grep -n "summarize-each-chunk-then-combine" FUTURE_SPEC.md   # Expected: 99:...
grep -n "degrades message quality"          FUTURE_SPEC.md   # Expected: 99:...
grep -n "permanently rejected"              FUTURE_SPEC.md   # Expected: 99:...

# --- CONDITION (c): NOTE non-contradictory with §9.24 (all on line 99) ---
grep -n "graduated to the spec"             FUTURE_SPEC.md   # Expected: 99:...
grep -n "see PRD §9.24"                     FUTURE_SPEC.md   # Expected: 99:...
grep -n "FR-T1–T12"                         FUTURE_SPEC.md   # Expected: 99:...
grep -n "applies only to the lossy form"    FUTURE_SPEC.md   # Expected: 99:...
grep -n "request-sized session turns"       FUTURE_SPEC.md   # Expected: 99:...

# Confirm the graduated-to anchor actually exists in the PRD:
grep -n "^### 9.24 Multi-turn generation fallback" PRD.md    # Expected: 491:...

# Expected: every printed line number is 99 (the single §3 row) and the PRD anchor is at 491.
# If any expected value differs, a condition FAILED → apply the Task 5 corrective edit, then re-run.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Not applicable — documentation-only confirmation; no code, no Go tests, no build.
echo "Level 2 N/A (documentation-only; no Go code or tests involved)."
```

### Level 3: Diff integrity (the acceptance gate)

```bash
# CONFIRMATION PATH (expected): the file is unchanged.
git diff --stat -- FUTURE_SPEC.md
# Expected: EMPTY (no output). If empty → confirmation succeeded; record it and stop.

# CORRECTION PATH (only if a condition failed): the diff is a single-line edit to row 99.
git diff --stat -- FUTURE_SPEC.md
# Expected: exactly "1 file changed, 1 insertion(+), 1 deletion(-)" (one table row replaced),
#           OR "1 file changed, 1 deletion(-)" if a stray duplicate entry was removed.
git diff -- FUTURE_SPEC.md | grep '^-' | grep -v '^---'
# Expected: at most the ONE old row line being replaced/removed — no other prose touched.

# Scope guard: ONLY FUTURE_SPEC.md changed by this task.
git diff --name-only
# Expected: at most FUTURE_SPEC.md. If README.md, docs/, PRD.md, or any source appears → STOP
#           and revert; those are out of scope (README.md is owned by sibling P1.M1.T5.S1).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# (a) Re-run the full Level 1 suite AFTER any corrective edit (or directly, on the confirmation
#     path) — all three conditions must now read PASS. This is the real acceptance check.
#     (Re-run the Level 1 block; every expected value must hold.)

# (b) Footer-rule sanity: FUTURE_SPEC's own footer ("an idea must live in exactly one of the two
#     documents") is not violated by an edit — i.e. the NOTE still POINTS to §9.24 rather than
#     re-specifying it.
grep -c "N+1\|message role\|session_mode" FUTURE_SPEC.md
# Expected: 0  (these are §9.24 internals; they must NOT appear in FUTURE_SPEC. If >0, the
#               NOTE was over-enriched — revert that addition; it violates the footer rule.)

# (c) Table still parses: the §3 row has exactly 2 pipe-delimited cells.
awk '/^## 3\. Rejected/,/^---$/' FUTURE_SPEC.md | grep -E 'Large-diff chunking' | \
  awk '{ n=gsub(/\|/,"|"); print n-1" cell-separators (expect 2)" }'
# Expected: 2  (i.e. "| a | b |"). If !=2, the row is malformed — fix before finishing.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: all three conditions (a/b/c) PASS; the §9.24 anchor exists at `PRD.md:491`.
- [ ] Level 3: `git diff --stat -- FUTURE_SPEC.md` is EMPTY (confirmation) OR a single-line
      edit to row 99 (correction); `git diff --name-only` shows at most `FUTURE_SPEC.md`.
- [ ] Level 4 (b): no §9.24 internals (`N+1`, `message role`, `session_mode`) leaked into the
      NOTE (footer rule respected); the §3 row still has exactly 2 cells.

### Feature Validation

- [ ] Condition (a): lossless multi-turn is not a rejected/deferred idea (0 stray entries).
- [ ] Condition (b): lossy chunk-summarize-combine is rejected with "degrades message quality".
- [ ] Condition (c): the NOTE points to §9.24 (FR-T1–T12) and scopes rejection to lossy only.
- [ ] Confirmation note recorded in the subtask result (path taken + per-condition verdicts).

### Code Quality Validation

- [ ] No file other than `FUTURE_SPEC.md` touched (README.md, docs/, PRD.md, source all intact).
- [ ] Corrective edit (if any) is minimal — smallest change fixing the actual defect; the row's
      em-dash/curly-quote typography and 2-cell structure preserved.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] FUTURE_SPEC.md still reads as a coherent registry: lossy rejected (here), lossless shipped
      (§9.24), no ambiguity for a future reader.
- [ ] No new env vars, flags, or config introduced (this task adds none).

---

## Anti-Patterns to Avoid

- ❌ Don't edit the file when it is already consistent. The EXPECTED outcome is a confirmation
  (no edit). Only edit if a Level 1 condition genuinely FAILS.
- ❌ Don't "enrich" the NOTE with §9.24 internals (`N+1`, `message role`/FR-T10,
  `session_mode`/FR-T1d/FR-T8). FUTURE_SPEC's footer requires an idea to live in exactly one
  document; the lossless *spec* lives in §9.24. The NOTE is a pointer, not a second copy.
- ❌ Don't treat the NOTE's omission of FR-T10/FR-T8 as an inconsistency. "Consistent" means
  non-contradictory, not exhaustive. Only flag (c) if the NOTE actively misdescribes the shipped
  behavior (e.g. calls lossless "lossy"/"truncating"/"summarizing").
- ❌ Don't touch `README.md` (owned by sibling P1.M1.T5.S1), `docs/`, `PRD.md`, `tasks.json`, or
  any source. Scope is `FUTURE_SPEC.md` only.
- ❌ Don't split the §3 row across lines, add a column, or alter its em-dash/curly-quote voice
  in a corrective edit. Keep it one line, 2 cells.
- ❌ Don't leave a duplicate/stray chunking row if you find one — delete the stray so the lossy
  row at line 99 is the single canonical home.
- ❌ Don't run a Go build/test as if this were code — it is a markdown read-verify; `make build`/
  `make test` are irrelevant and would mask a doc regression.
- ❌ Don't edit PRD.md if you believe §9.24 itself is wrong — that is out of scope; record the
  finding and stop.
