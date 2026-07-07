---
name: "P1.M5.T1.S1 (Mode B docs) — Sweep README.md and docs/how-it-works.md for hooks-feature accuracy: add the empty-message-after-hooks abort to the detailed hooks section (Issue 4); confirm no stale key/argc/newline refs (Issues 1/2/3)"
description: |

  Mode B changeset-level documentation sync, DEPENDENT ON all 4 implementing subtasks (P1.M1.T1 argc=1,
  P1.M1.T2 trailing newline, P1.M2.T1 stagehand.noVerify, P1.M3.T1 empty-message guard — all COMPLETE).
  The per-file docs (docs/cli.md:44, docs/configuration.md:155) + config.go:130 were ALREADY corrected in
  P1.M2.T1.S1 (Mode A). THIS task sweeps ONLY the overview/README-level docs that the per-file fix didn't
  touch: README.md:71, README.md:367-369, docs/how-it-works.md:310-324, docs/how-it-works.md:337.

  RESEARCH VERDICT (grep-confirmed, all 4 locations read in full):
    - Check (a) stale `stagehand.no_verify` key: NOT PRESENT in either overview doc (grep count 0). The
      stale key was only in docs/cli.md:44 + docs/configuration.md:155 + config.go:130 — all already fixed
      in P1.M2.T1.S1. NO CHANGE.
    - Check (b) 2-args/argc claim: NOT PRESENT (overview docs never mention argv). NO CHANGE.
    - Check (c) trailing-newline implication: NOT PRESENT (overview docs never describe message-file
      format). NO CHANGE.
    - Check (d) empty-message abort omission: FOUND in ONE place — docs/how-it-works.md "Commit hooks on
      the plumbing path" (L310-324). That DETAILED section enumerates hook failure modes (rescue exit 3 on
      hook failure/non-zero/timeout) but OMITS the Issue-4 behavior (a `prepare-commit-msg`/`commit-msg`
      that empties the message file → abort exit 1, no commit). ADD it there for completeness, mirroring
      the `--edit` abort documented at docs/cli.md:42 ("An empty result aborts (exit 1, not a rescue)").
    - README.md (both L71 feature-table row + L367-369 FAQ): high-level, clean, appropriately abstract →
      REVIEWED, NO CHANGE (the contract: "likely for README which is high-level").

  CONTRACT (item_description §1–§5; Bug-Fix PRD §h2.2 Issue 4):
    3. LOGIC: "Review README.md:71, README.md:367-369, docs/how-it-works.md:310-324, and docs/how-it-works.md:337.
       Check whether any content: (a) references the old invalid key name 'stagehand.no_verify' … (b) claims
       prepare-commit-msg runs with 2 args … (c) implies message files have no trailing newline … or (d)
       omits the empty-message abort behavior (should mention it for completeness, like the --edit abort).
       If any of these are found, update the doc text … If none are found (likely for README which is
       high-level), document that the docs were reviewed and no changes were needed."
    4. OUTPUT: "README.md and docs/how-it-works.md accurately reflect the git-parity behavior of the hooks
       feature after the 4 bug fixes. No stale references to the invalid key name or incorrect argc."
    5. DOCS: "[Mode B] This IS the changeset-level documentation sync task."

  DELIVERABLE (1 file MODIFIED; no code, no tests):
    MODIFY docs/how-it-works.md — in the `## Commit hooks on the plumbing path` section, add ONE sentence
    describing the empty-message-after-hooks abort (exit 1, no commit; mirrors git + the --edit path),
    placed immediately after the existing "rescue (exit code 3)" sentence and before "`post-commit` is
    best-effort".

  SCOPE BOUNDARY (do NOT edit):
    - README.md — REVIEWED, NO CHANGE required (high-level; both locations clean). Do NOT add failure-mode
      detail to the feature-table row or rewrite the FAQ.
    - docs/cli.md, docs/configuration.md, internal/config/config.go — the `stagehand.noVerify` key fix is
      ALREADY DONE (P1.M2.T1.S1). Re-touching is out of scope and risks churn.
    - docs/how-it-works.md other sections + the L337 feature-list bullet — NOT in scope for the abort detail.
    - ANY source code / tests — Mode B docs-only.

  SUCCESS: docs/how-it-works.md "Commit hooks on the plumbing path" documents the empty-message-after-hooks
  abort (exit 1, no commit) alongside the existing rescue (exit 3) behavior; NO stale `stagehand.no_verify`/
  argc/trailing-newline references exist in either overview doc (grep-confirmed 0); README.md reviewed and
  unchanged; markdown well-formed; only docs/how-it-works.md modified.

---

## Goal

**Feature Goal**: Sync the overview-level hooks documentation to the post-bugfix git-parity behavior.
Concretely: (1) confirm by grep + full read that README.md and docs/how-it-works.md contain NO stale
`stagehand.no_verify` key name, NO incorrect argc/2-args claim, and NO trailing-newline misstatement (the
stale technical details were per-file/code, already fixed in P1.M2.T1.S1); and (2) close the ONE real
accuracy gap — the detailed "Commit hooks on the plumbing path" section enumerates hook failure modes but
omits the Issue-4 empty-message-after-hooks abort, so add it for completeness, mirroring the `--edit`
empty-result abort.

**Deliverable** (1 modified file; no code, no tests):
- `docs/how-it-works.md` — one sentence added to `## Commit hooks on the plumbing path` describing the
  empty-message-after-hooks abort (exit 1, no commit; mirrors git + `--edit`), placed after the existing
  rescue (exit 3) sentence.

**Success Definition**:
- A reader of the detailed hooks section sees BOTH abort behaviors: hook failure/non-zero/timeout → rescue
  (exit 3); hook empties the message → exit 1, no commit (the new sentence).
- The new prose is accurate to the shipped Issue-4 fix: "exit 1, not a rescue", no commit, HEAD/index
  untouched (no `update-ref` ran).
- `grep -cE 'no_verify|argc|two arg|2 arg' README.md docs/how-it-works.md` → 0 / 0 (no stale references
  introduced or previously present).
- README.md is reviewed and UNCHANGED (both cited locations are high-level and clean).
- Markdown is well-formed; only `docs/how-it-works.md` is modified.

## User Persona

**Target User**: the stagehand user (or contributor) reading "How it works" to understand what happens when
a hook misbehaves — specifically the user whose `commit-msg`/`prepare-commit-msg` hook rejects a message by
emptying the file (a common force-re-edit / rejection pattern). They want to know stagehand aborts cleanly
(exit 1, no commit) just like `git commit`.

**Use Case**: user has a `commit-msg` hook that empties `$1` to reject a message → runs `stagehand` → reads
how-it-works.md to confirm the behavior → sees the empty-message abort documented (exit 1, no commit, clean
repo) alongside the rescue behavior.

**User Journey**: reader scans "Commit hooks on the plumbing path" → sees the rescue (exit 3) sentence →
sees the new empty-message abort sentence (exit 1) → understands the two distinct abort paths and that both
leave the repo clean.

**Pain Points Addressed**: the detailed section documented ONE abort path (rescue) but not the other
(empty-message); a user with a rejecting hook had no doc confirming stagehand matches git's "Aborting commit
due to empty commit message." behavior. This closes that gap.

## Why

- **It IS the P1.M5.T1.S1 contract.** Mode B changeset-level docs sync: the overview docs must reflect the
  4 bugfixes' git-parity behavior. The contract names exactly these 4 locations and 4 checks.
- **Closes the one real accuracy gap (Issue 4).** The detailed hooks section enumerates hook failure modes
  but omits the empty-message abort. The bugfix changeset's whole theme is git parity; documenting the
  abort that the Issue-4 fix added is the sync this section needs.
- **Documents shipped behavior, not a promise.** The empty-message guard is LANDED (P1.M3.T1.S1 Complete)
  in all 3 commit paths. The docs describe existing behavior.
- **Confirms the per-file fix covered the stale key.** The `stagehand.no_verify` → `stagehand.noVerify`
  rename was done in P1.M2.T1.S1 for the per-file docs + code. This task CONFIRMS (grep + read) that the
  overview docs never carried the stale key — so no dangling reference remains anywhere user-facing.
- **Cheap and isolated.** One sentence in one docs file + a documented review. No code, no tests, no
  cross-file churn (README, cli.md, configuration.md all untouched).

## What

A single-sentence addition to `docs/how-it-works.md`, plus a documented review confirming the other three
checks are clean.

**The edit**: in `## Commit hooks on the plumbing path`, immediately after the sentence ending
"…the rescue recipe is printed." and before the sentence "`post-commit` is best-effort…", insert one
sentence:

> A hook that empties the message file (a rejection or force-re-edit pattern) aborts with **exit 1** and no
> commit created — mirroring `git commit`'s "Aborting commit due to empty commit message." and the `--edit`
> path's empty-result abort (exit 1, not a rescue). HEAD and the index are untouched at that point (no
> `update-ref` has run).

**The review (documented, no edits)**: README.md:71 (feature-table row), README.md:367-369 (FAQ), and
docs/how-it-works.md:337 (feature-list bullet) were checked for stale `stagehand.no_verify` / 2-args /
trailing-newline content. None found (`grep -cE 'no_verify|argc|two arg|2 arg'` = 0 for both files). These
locations are appropriately high-level (the feature-table row and the feature-list bullet enumerate no
failure modes; the FAQ lists one abort example already) and require no change.

### Success Criteria

- [ ] `docs/how-it-works.md` "Commit hooks on the plumbing path" contains a sentence documenting the
      empty-message-after-hooks abort: exit 1, no commit, mirrors git + `--edit`, HEAD/index untouched.
- [ ] The new sentence is placed AFTER the rescue (exit 3) sentence and BEFORE the `post-commit`
      best-effort sentence.
- [ ] The new prose is accurate: "exit 1, not a rescue" (NOT exit 3 / rescue); "no commit"; "HEAD and the
      index untouched" (no `update-ref` ran).
- [ ] `grep -cE 'no_verify|argc|two arg|2 arg' README.md docs/how-it-works.md` → `0` and `0` (no stale
      references, none introduced).
- [ ] README.md is UNCHANGED (reviewed, no edit required); docs/cli.md, docs/configuration.md, config.go
      UNCHANGED (already fixed in P1.M2.T1.S1).
- [ ] Well-formed markdown; ONLY `docs/how-it-works.md` modified.

## All Needed Context

### Context Completeness Check

_Pass._ A technical writer with no prior repo knowledge can implement this from: the exact insertion point
(after "…the rescue recipe is printed.", before "`post-commit` is best-effort"), the ready-to-paste prose
(quoted), the accuracy constraint ("exit 1, not a rescue"), the grep check confirming no stale refs, and
the LEAVE list (README, cli.md, configuration.md, config.go, all code/tests). No code reading is required.

### Documentation & References

```yaml
# MUST READ — THE decisive doc (the verdict per location + the exact edit)
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/P1M5T1S1/research/findings.md
  why: §1 the VERDICT per location (checks a/b/c = NOT PRESENT, grep count 0; check d = FOUND in how-it-works.md
       L310-324 only); §2 the exact insertion point + recommended prose (mirrors the rescue sentence + the
       cli.md:42 --edit parallel); §3 confirms the Issue-4 fix is LANDED (docs describe shipped behavior);
       §4 scope (README reviewed/no-change; cli.md/configuration.md/config.go already fixed — LEAVE);
       §5 validation (grep checks).
  critical: §1 (a/b/c are CLEAN — don't invent stale refs to "fix"; d is the ONLY edit), §2 (the exact prose
       + insertion point + the "exit 1, not a rescue" accuracy pin).

# MUST READ — the file being EDITED (the detailed hooks section)
- file: docs/how-it-works.md   (EDIT — add ONE sentence to `## Commit hooks on the plumbing path`)
  section: `## Commit hooks on the plumbing path` (L306-324). The insertion neighborhood:
       "… A hook that exits non-zero or times out aborts the run as a **rescue** (exit code 3) — no commit is
       created, HEAD and the index are byte-for-byte unchanged, and the rescue recipe is printed. `post-commit`
       is best-effort: …"
       INSERT the new sentence BETWEEN "…the rescue recipe is printed." and "`post-commit` is best-effort".
  why: this is THE edit. The section already enumerates ONE abort path (rescue exit 3); the empty-message
       abort is the parallel second path that Issue 4 added. Grouping them is the natural place a reader looks.
  pattern: mirror the existing sentence's structure — bold the exit code (`**exit 1**`), state "no commit is
       created", state HEAD/index disposition. Match the file's prose style (plain sentences, `**bold**` for
       the key token, inline code for literals like `--edit`, `update-ref`).
  gotcha: the abort is "exit 1, NOT a rescue" (Issue 4 returns a non-rescue error). Do NOT call it a rescue
       or exit 3. Do NOT imply HEAD moved (no `update-ref` ran). The `--edit` parallel (cli.md:42) is
       "An empty result aborts (exit 1, not a rescue)" — mirror that framing.

# MUST READ — the --edit parallel (pin the wording; do NOT edit)
- file: docs/cli.md   (READ ONLY — L42)
  section: the `--edit` flag row: "… An empty result aborts (exit 1, not a rescue). …"
  why: the contract says the empty-message abort "should mention it for completeness, like the --edit abort".
       cli.md:42 is that --edit abort, documented verbatim — mirror its "exit 1, not a rescue" framing so the
       two docs are consistent.
  gotcha: do NOT edit cli.md (it's already correct and was part of P1.M2.T1.S1's surface). READ ONLY.

# READ — the cited README locations (confirm high-level + clean; do NOT edit)
- file: README.md   (READ ONLY — L71 feature-table row + L367-369 FAQ)
  section: L71 "Commit hooks on every `stagehand` commit" row (mentions only the hook chain + `--no-verify`;
       enumerates NO failure modes). L367-369 "Does it run my pre-commit hooks?" FAQ (lists one abort example:
       "a hook that stages brand-new content aborts the run").
  why: confirms README is appropriately high-level and contains no stale key/argc/newline refs. The contract
       explicitly anticipates "likely for README which is high-level" → no change.
  gotcha: do NOT add the empty-message abort to README — the feature-table row enumerates no failure modes
       (adding one would be inconsistent), and the FAQ already lists an abort example and answers a different
       question ("does it run my hooks", not "enumerate aborts"). The detailed treatment belongs in how-it-works.md.

# READ — confirms the underlying behavior is shipped (the docs describe real behavior)
- file: internal/generate/generate.go   (READ ONLY)
  section: CommitStaged — after RunCommitHooks returns, the empty-message guard (P1.M3.T1.S1) returns a
       non-rescue error when the finalized message is empty (after trimming) → exit 1, no CommitTree, no
       update-ref. (Same guard in pkg/stagehand.runPipeline and internal/decompose/message.go publishCommit.)
  why: confirms the empty-message abort IS the shipped behavior the new sentence describes. (Issue 4 fix =
       Complete; no code change needed for this docs task.)

- url: (PRD internal) Bug-Fix PRD §h2.2 Issue 4 (in context as selected_prd_content h3.3) — the AUTHORITATIVE
       statement of the empty-message abort: git aborts "Aborting commit due to empty commit message." (exit 1,
       no commit); stagehand must do the same. The new sentence documents this parity.
```

### Current Codebase tree (relevant slice)

```bash
docs/
  how-it-works.md   # EDIT — `## Commit hooks on the plumbing path`: +1 sentence (empty-message abort, Issue 4).
  cli.md            # READ ONLY — L42 the --edit "exit 1, not a rescue" parallel (wording source). NOT edited.
  configuration.md  # READ ONLY — stagehand.noVerify already fixed (P1.M2.T1.S1). NOT edited.
README.md           # READ ONLY — L71 + L367-369 reviewed, high-level + clean. NOT edited.
internal/generate/generate.go   # READ ONLY — CommitStaged empty-message guard (P1.M3.T1.S1, shipped).
internal/hooks/runner.go        # READ ONLY — Issues 1/2 fixes (argc=1, trailing newline) shipped.
internal/config/config.go       # READ ONLY — stagehand.noVerify comment already fixed (P1.M2.T1.S1).
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. 1 MODIFIED docs file only:
docs/how-it-works.md   # +1 sentence in `## Commit hooks on the plumbing path` (empty-message-after-hooks abort).
# NO code changes. NO tests. README.md / docs/cli.md / docs/configuration.md UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the abort is "exit 1, NOT a rescue"): the Issue-4 empty-message guard returns a NON-rescue
     error → exit 1 (mirroring git + the --edit path). Do NOT call it a rescue or cite exit 3. The rescue
     (exit 3) is the SEPARATE hook-failure/timeout path the existing sentence already documents. The two
     aborts are distinct: hook fails/non-zero/times out → rescue exit 3; hook empties the message → exit 1.
     (findings §2; cli.md:42 "exit 1, not a rescue".) -->

<!-- CRITICAL (HEAD/index UNTOUCHED on the empty-message abort): the guard fires AFTER RunCommitHooks but
     BEFORE CommitTree/update-ref, so no ref mutation occurred. State this (it is the user's main worry —
     "did this corrupt my repo?"). It parallels the rescue sentence's "HEAD and the index are byte-for-byte
     unchanged". (findings §2.) -->

<!-- CRITICAL (checks a/b/c are CLEAN — do NOT invent stale refs to "fix"): grep confirms ZERO
     `no_verify`/`argc`/`two arg`/`2 arg` references in README.md and docs/how-it-works.md. The stale
     `stagehand.no_verify` key was ONLY in docs/cli.md:44 + docs/configuration.md:155 + config.go:130 — all
     already corrected to `stagehand.noVerify` in P1.M2.T1.S1. Do NOT "fix" a non-existent reference, and do
     NOT re-touch those already-fixed files. The only edit is check (d). (findings §1/§4.) -->

<!-- GOTCHA (placement — group the two aborts): insert the new sentence IMMEDIATELY AFTER the rescue
     sentence ("…the rescue recipe is printed.") and BEFORE "`post-commit` is best-effort". This keeps the
     two abort behaviors adjacent (the reader scanning for "what happens if a hook misbehaves" finds both).
     Do NOT append it at the end of the section or near the `--no-verify` sentence. -->

<!-- GOTCHA (README is REVIEW-ONLY): the contract says "likely for README which is high-level" → no change.
     Do NOT add failure-mode detail to the L71 feature-table row (it enumerates none) or rewrite the L367-369
     FAQ (it answers "does it run my hooks" and already lists one abort example). The detailed treatment
     belongs ONLY in how-it-works.md. -->

<!-- GOTCHA (mirror the file's prose style): bold the exit code (`**exit 1**`), use inline code for literals
     (`--edit`, `update-ref`, `prepare-commit-msg`/`commit-msg`), plain sentences. Match the rescue sentence's
     rhythm ("A hook that … aborts with … — mirroring … . HEAD and the index …"). -->

<!-- GOTCHA (do NOT over-link): the section already ends with "See PRD §9.25 (FR-V1–V8) for the full
     specification". Don't add another link/cross-ref in the new sentence; keep it self-contained prose. -->
```

## Implementation Blueprint

### Data models and structure

```markdown
<!-- No data models — this is a one-sentence prose addition. The "structure" is: locate the rescue sentence
     in `## Commit hooks on the plumbing path`, insert the new sentence right after it (before the
     `post-commit` best-effort sentence). -->
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: REVIEW the 4 cited locations + confirm checks (a)/(b)/(c) are clean (READ/grep, no edit)
  - RUN: `grep -cE 'no_verify|argc|two arg|2 arg' README.md docs/how-it-works.md` → expect `0` and `0`.
      (Confirms NO stale key name, NO argc/2-args claim in either overview doc.)
  - READ README.md:71 (feature-table row) → confirm it mentions only the hook chain + `--no-verify` and
      enumerates NO failure modes → appropriately high-level, NO change.
  - READ README.md:367-369 (FAQ) → confirm it lists at most one abort example ("a hook that stages brand-new
      content aborts the run") and answers "does it run my hooks" → NO change.
  - READ docs/how-it-works.md:337 (feature-list bullet) → confirm it is a terse bullet enumerating no failure
      modes → NO change.
  - READ docs/how-it-works.md:310-324 (the DETAILED section) → confirm it enumerates the rescue (exit 3) but
      OMITS the empty-message abort → this is the ONE edit (Task 2).
  - WHY: the contract requires a documented review; the grep pins the (a/b/c) verdict to evidence so the
      implementer doesn't invent stale refs to "fix".
  - GOTCHA: if the grep returns non-zero, STOP and re-read the matching line — there may be a stale reference
      after all (the findings said 0; verify before proceeding). (It is 0.)

Task 2: EDIT docs/how-it-works.md — add the empty-message-after-hooks abort sentence (THE deliverable)
  - FILE: docs/how-it-works.md, section `## Commit hooks on the plumbing path`.
  - LOCATE the rescue sentence (it ends "…the rescue recipe is printed.") immediately followed by
      "`post-commit` is best-effort: …".
  - INSERT between them ONE sentence (adapt this proven prose — keep the accuracy pins):
      "A hook that empties the message file (a rejection or force-re-edit pattern) aborts with **exit 1** and
      no commit created — mirroring `git commit`'s \"Aborting commit due to empty commit message.\" and the
      `--edit` path's empty-result abort (exit 1, not a rescue). HEAD and the index are untouched at that
      point (no `update-ref` has run)."
  - WHY: closes the check-(d) gap. The detailed section now documents BOTH abort paths (rescue exit 3 on hook
      failure; exit 1 on empty message), accurate to the shipped Issue-4 fix.
  - GOTCHA: the abort is "exit 1, NOT a rescue" — do NOT conflate it with the rescue (exit 3). State HEAD/index
      untouched (no update-ref ran). Mirror the rescue sentence's `**bold**`-exit-code + inline-code style.
      Place it ADJACENT to the rescue sentence (not at section end).

Task 3: VERIFY (docs-only gates)
  - RUN the grep checks in "Validation Loop → Level 1" (the empty-message abort is present; no stale refs;
      README/cli.md/configuration.md unchanged).
  - VISUAL review: the section reads coherently — rescue (exit 3) → empty-message abort (exit 1) →
      post-commit best-effort, as three distinct hook-outcome behaviors.
  - MARKDOWN sanity: valid sentence, balanced quotes around the git message, valid inline code.
  - `git status --porcelain` → ONLY docs/how-it-works.md modified.
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: mirror the existing rescue sentence. It is:
     "A hook that exits non-zero or times out aborts the run as a **rescue** (exit code 3) — no commit is
      created, HEAD and the index are byte-for-byte unchanged, and the rescue recipe is printed."
     The new sentence mirrors this rhythm: "A hook that empties the message file … aborts with **exit 1** and
     no commit created — mirroring … . HEAD and the index are untouched …". -->

<!-- PATTERN: the --edit abort parallel (cli.md:42): "An empty result aborts (exit 1, not a rescue)." The new
     prose reuses the exact "exit 1, not a rescue" framing so the two docs are consistent. -->

<!-- CRITICAL (accuracy): the two aborts are DISTINCT:
       - hook fails / non-zero / times out  → RESCUE, exit 3 (existing sentence).
       - hook EMPTIES the message           → exit 1, NOT a rescue (new sentence).
     Do NOT merge them, do NOT call the empty-message case a rescue, do NOT cite exit 3 for it. -->

<!-- CRITICAL (HEAD/index clean): the empty-message guard fires after RunCommitHooks, before CommitTree/
     update-ref → no ref mutation. State "HEAD and the index are untouched (no update-ref has run)" — it is
     the user's main worry and parallels the rescue sentence's "byte-for-byte unchanged". -->

<!-- GOTCHA (scope): ONLY docs/how-it-works.md is edited. README.md is REVIEWED (no change). cli.md /
     configuration.md / config.go were fixed in P1.M2.T1.S1 — LEAVE them. No code, no tests. -->
```

### Integration Points

```yaml
DOCS.HOW_IT_WORKS (the only file edited):
  - section: "## Commit hooks on the plumbing path"
  - insert: "ONE sentence after the rescue (exit 3) sentence, before the post-commit best-effort sentence"
  - content: "empty-message-after-hooks abort: exit 1, no commit, mirrors git + --edit, HEAD/index untouched"

DOCS.CLI (READ ONLY — the --edit parallel wording source):
  - reference: "docs/cli.md:42 — 'An empty result aborts (exit 1, not a rescue).'"
  - do-not-edit: "cli.md is correct and was part of P1.M2.T1.S1's surface."

README (READ ONLY — reviewed, no change):
  - L71 feature-table row: "high-level, enumerates no failure modes — NO change."
  - L367-369 FAQ: "answers 'does it run my hooks', lists one abort example already — NO change."

FROZEN/LEAVE (do NOT edit):
  - docs/configuration.md, internal/config/config.go (stagehand.noVerify fixed in P1.M2.T1.S1).
  - docs/how-it-works.md other sections + the L337 feature-list bullet.
  - ALL source code and tests (Mode B docs-only; the 4 bugfixes are the INPUT).
```

## Validation Loop

### Level 1: Content & accuracy checks (docs-only — the real gate)

```bash
# 1. The empty-message-after-hooks abort is now documented in the detailed hooks section:
grep -nE 'empty.*message|Aborting commit|exit 1' docs/how-it-works.md
# Expected: a match in `## Commit hooks on the plumbing path` describing the abort (exit 1, no commit).

# 2. NO stale key name / argc / 2-args / trailing-newline references exist (checks a/b/c clean):
grep -cE 'no_verify|argc|two arg|2 arg' README.md docs/how-it-works.md
# Expected: README.md:0  and  docs/how-it-works.md:0  (none present, none introduced).

# 3. The new prose is accurate (NOT a rescue; exit 1):
grep -n 'exit 1, not a rescue' docs/how-it-works.md   # expect the new sentence (mirrors cli.md:42's framing)

# 4. The --edit parallel wording is unchanged in cli.md (READ ONLY — not edited):
grep -n 'empty result aborts' docs/cli.md   # expect the existing L42 row, unchanged

# Expected: the abort documented; zero stale refs; "exit 1, not a rescue" present; cli.md unchanged.
```

### Level 2: Markdown well-formedness

```bash
# If a markdown linter is available:
markdownlint docs/how-it-works.md 2>/dev/null && echo "markdownlint clean" || echo "(no markdownlint — visual review)"
# Visual review checklist:
#   - The new sentence is a single sentence within the existing paragraph (or a short follow-on sentence).
#   - Inline code balanced: `git commit`, `--edit`, `update-ref`, `prepare-commit-msg`/`commit-msg`.
#   - The quoted git string "Aborting commit due to empty commit message." uses straight quotes, balanced.
#   - `**exit 1**` bolds correctly; no stray markdown.
#   - No broken cross-links introduced (the section's existing "See PRD §9.25" / anchor links stay valid).
```

### Level 3: Scope & review audit

```bash
# ONLY docs/how-it-works.md changed (README reviewed/no-change; no code; no other docs):
git status --porcelain
# Expected: exactly one entry — docs/how-it-works.md (modified). Nothing under internal/, README.md, or other docs.

# Confirm the already-fixed files are NOT re-touched (P1.M2.T1.S1 owns them):
git diff --exit-code README.md docs/cli.md docs/configuration.md internal/config/config.go go.mod go.sum \
  && echo "README / cli.md / configuration.md / config.go UNCHANGED (expected)"

# Confirm the new sentence sits in the right place (after the rescue sentence, before post-commit):
sed -n '/## Commit hooks on the plumbing path/,/## /p' docs/how-it-works.md | \
  grep -nE 'rescue \(exit code 3\)|empty.*message.*aborts|post-commit.*best-effort'
# Expected (in order): the rescue sentence, THEN the new empty-message sentence, THEN the post-commit sentence.
```

### Level 4: Accuracy spot-check against the shipped behavior (confidence, no file change)

```bash
# Confirm the empty-message guard is shipped (the docs describe real behavior, not a promise). READ ONLY:
grep -rn 'empty' internal/generate/generate.go | grep -iE 'message|abort'   # the CommitStaged guard (P1.M3.T1.S1)
grep -rn 'empty' internal/decompose/message.go | grep -iE 'message|abort'   # the publishCommit guard (P1.M3.T1.S1)
# Expected: the guards exist (the docs accurately describe shipped behavior). No code change in this task.

# Reasoning check (the two aborts are distinct):
#   hook fails / non-zero / times out  → rescue, exit 3 (the existing sentence — unchanged).
#   hook empties the message file      → exit 1, NOT a rescue, no commit (the NEW sentence).
# The docs now match the implementation's two distinct post-hook failure paths.
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1: the empty-message abort is documented in `## Commit hooks on the plumbing path`; `grep -cE
      'no_verify|argc|two arg|2 arg'` = 0 for both files; "exit 1, not a rescue" present; cli.md unchanged.
- [ ] Level 2: markdown well-formed (balanced inline code/quotes, valid `**exit 1**` bold, no broken links).
- [ ] Level 3: ONLY `docs/how-it-works.md` modified; README/cli.md/configuration.md/config.go/code unchanged.
- [ ] Level 4: the empty-message guard is shipped (docs describe real behavior); the two aborts are distinct.
- [ ] No source code or test files touched; no other docs files touched.

### Feature Validation
- [ ] `docs/how-it-works.md` "Commit hooks on the plumbing path" documents the empty-message-after-hooks
      abort (exit 1, no commit, mirrors git + `--edit`, HEAD/index untouched) alongside the rescue (exit 3).
- [ ] The new sentence is placed after the rescue sentence, before the `post-commit` best-effort sentence.
- [ ] Accuracy: "exit 1, NOT a rescue" (not exit 3); "no commit"; "HEAD and the index untouched".
- [ ] Checks (a)/(b)/(c) confirmed clean (grep 0); the stale `stagehand.no_verify` key lives nowhere in the
      overview docs (it was per-file/code, already fixed in P1.M2.T1.S1).
- [ ] README.md reviewed and unchanged (high-level + clean at both cited locations).

### Code Quality Validation
- [ ] The new sentence mirrors the existing rescue sentence's structure + the cli.md:42 `--edit` framing.
- [ ] No duplication of README/cli.md/configuration.md content; no cross-link churn.
- [ ] Anti-patterns avoided (see below); focused one-sentence addition + documented review.

### Documentation & Deployment
- [ ] User-facing prose; the abort behavior is described in the same detail level as the rescue behavior.
- [ ] No new knobs/flags/keys invented; describes existing shipped behavior only.
- [ ] The documented review (README clean, checks a/b/c clean) is recorded in the implementation summary.

---

## Anti-Patterns to Avoid

- ❌ **Don't call the empty-message abort a "rescue" or cite exit 3.** It is a NON-rescue error → exit 1
  (Issue 4 mirrors git + the `--edit` path). The rescue (exit 3) is the SEPARATE hook-failure path the next
  sentence over documents. Keep the two distinct. (gotcha)
- ❌ **Don't omit the HEAD/index disposition.** The user's main worry is "did this corrupt my repo?" State
  "HEAD and the index are untouched (no `update-ref` has run)" — paralleling the rescue sentence's
  "byte-for-byte unchanged". (gotcha)
- ❌ **Don't edit README.md.** The contract says README is "likely high-level" → no change. The feature-table
  row (L71) enumerates no failure modes; the FAQ (L367-369) answers "does it run my hooks" and already lists
  one abort example. Adding failure-mode detail there is inconsistent. REVIEW ONLY. (gotcha)
- ❌ **Don't re-touch docs/cli.md, docs/configuration.md, or config.go.** The `stagehand.noVerify` rename was
  completed in P1.M2.T1.S1 (Mode A). Re-editing is out of scope and risks churn. (scope)
- ❌ **Don't invent stale references to "fix".** Checks (a)/(b)/(c) are CLEAN (grep count 0). Do not add a
  `stagehand.noVerify` mention to the overview docs "for completeness" — these docs intentionally stay
  high-level and mention only the `--no-verify` flag. The only edit is check (d). (gotcha)
- ❌ **Don't place the new sentence at the section end or near `--no-verify`.** Group it with the rescue
  sentence (immediately after it, before `post-commit` best-effort) so the two abort behaviors sit together.
  (gotcha)
- ❌ **Don't edit code or tests.** This is Mode B docs-only. The 4 bugfixes are the INPUT; the empty-message
  guard is already shipped (P1.M3.T1.S1). (scope)
