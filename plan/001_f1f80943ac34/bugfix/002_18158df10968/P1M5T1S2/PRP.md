---
name: "Reconcile docs/how-it-works.md overview (failure/exit-code + rescue sections) with the merge-conflict and dry-run fixes (Mode B, bugfix-002)"
work_item: P1.M5.T1.S2 (bugfix-002)
kind: documentation (Mode B — changeset-level overview reconciliation of docs/how-it-works.md)
changeset: bugfix-002 (002_18158df10968) — second-pass QA bugfix
depends_on:
  - P1.M3.T1.S1 (Issue 3: clean single-line "resolve merge conflicts" message; exit 1; pre-generation)  ✅ Complete (commit a9055fa — NO doc touched; deferred here)
  - P1.M4.T1.S1 (Issue 4: --dry-run generation failure → exit 1 + short message, no rescue recipe)       ✅ Complete (commit 04508d3 — Mode-A updated docs/cli.md ONLY)
mode_a_docs_already_synced:
  - docs/cli.md        (Issue 4: --dry-run flag row line 26, exit-code note line 86 — the SOURCE OF TRUTH for wording)
  - docs/configuration.md (Issue 2 — opt-in override; not this task's concern)
  - docs/providers.md  (Issue 2 — manifest output/strip semantics; not this task's concern)
  - README.md          (S1/P1.M5.T1.S1 — done, commit 78e6bdd; DO NOT EDIT here)
---

## Goal

**Feature Goal**: Make `docs/how-it-works.md`'s **§"Failure modes and exit codes"** and
**§"Rescue protocol"** overview consistent with the shipped behavior of bugfix-002 **Issues 3 & 4**.
The implementing subtasks deferred all `how-it-works.md` edits to this Mode-B task (Issue 3's commit
`a9055fa` touched no docs; Issue 4's commit `04508d3` touched only `docs/cli.md`). Two reconciliations
are required: (a) **Issue 3** — the failure-modes table omits merge conflicts; add the now-clean
"resolve merge conflicts first" pre-generation exit-1 row. (b) **Issue 4** — the §"Rescue protocol"
over-claims that *every* post-snapshot generation failure prints the full recovery recipe (exit 3/124);
under `--dry-run`, generation failures now exit **1** with a short message and **omit** the recipe.

**Deliverable**: An edited `docs/how-it-works.md` (prose + one table row — **no `.go` file touched
anywhere**), verified by (1) `markdownlint-cli2` clean, (2) coherence greps showing the rescue/exit
wording agrees with the already-synced `docs/cli.md` (source of truth) and the binary, and (3) a
`go build/vet/test` sanity pass. Changes are accurate and minimal — if a section already matches, it
is left alone.

**Success Definition**:

- `npx markdownlint-cli2 docs/how-it-works.md` → `0 error(s)`.
- The failure-modes table lists **merge conflicts → 1 (Error)** with the clean "resolve … then re-run"
  recovery text (Issue 3).
- The §"Rescue protocol" no longer implies the recovery recipe is printed for `--dry-run`; it states
  the recipe + codes 3/124 apply to a real commit, and that `--dry-run` generation failures exit 1
  with a short message (Issue 4).
- Every behavioral claim in `docs/how-it-works.md` agrees with `docs/cli.md` (the already-synced
  source of truth).
- Only `docs/how-it-works.md` is modified (`git status --short` shows nothing else).

## Why

- `docs/how-it-works.md` is the cross-cutting **architecture overview** that ties together the git
  plumbing, rescue protocol, and exit-code story. Once the bugfix-002 changeset shipped, two overview
  claims drifted from the binary: the failure table omitted the (now clean) merge-conflict outcome,
  and the rescue protocol described the recovery recipe as the universal generation-failure outcome.
- A user reading "When generation fails after the snapshot is taken (exit 3 or 124) … prints a
  recovery block" and then running `stagecoach --dry-run` into a timeout gets **exit 1 + a short line**
  — contradicting the doc. Likewise, a user hitting a merge conflict now sees a clean one-liner, but
  the doc never mentions that failure mode at all.
- This is the **only** documentation task scoped to `docs/how-it-works.md`. S1 owns README (done).
Issues 1 & 2 have no how-it-works.md surface (owned by cli/config/providers docs, already Mode-A
synced). Stay within `docs/how-it-works.md`.

## What

A verification pass + up to three targeted prose/table edits. Each maps to a **shipped fix** and is
anchored to an exact current line. The table records what Mode-A **already did** (do not duplicate)
vs. what this task adds.

### Mode-A coverage map (DO NOT duplicate — these are done)

| Issue | File | Mode-A edit (already landed) | Commit |
|-------|------|------------------------------|--------|
| 3 | — | **NONE** — implementing commit `a9055fa` touched only `internal/git/git.go` (message-only); docs deferred to this task | a9055fa |
| 4 | `docs/cli.md:26` | `--dry-run` flag row: "… If generation fails … exits **1** with a short stderr message instead of exit 3/124 + the full recovery recipe (since no commit was ever intended)" | 04508d3 |
| 4 | `docs/cli.md:86` | exit-code note: "With `--dry-run`, generation failures … report exit **1** with a short stderr message (not 3/124 + the recovery recipe)" | 04508d3 |

### This task's edits (all in docs/how-it-works.md)

| # | Section (line) | Issue | Drift | Type |
|---|----------------|-------|-------|------|
| A | §"Failure modes and exit codes" table (~57-62) | 3 | Table OMITS merge conflicts entirely (PRD §18.2 has the row); contract OUTPUT requires "note that merge conflicts produce a clean 'resolve merge conflicts first' message (exit 1)" | **REQUIRED** (add one row) |
| B | §"Rescue protocol" intro + tail (~68, ~83) | 4 | Line 68 over-claims the recovery recipe applies to *every* post-snapshot generation failure; under `--dry-run` failure → exit 1 + short message, no recipe | **REQUIRED** |
| C | Note under the failure-modes table (~64) | 4 | The table's 3/124 rows read as universal; contract names this section | RECOMMENDED (one cross-ref line) |

> **Scope discipline (contract verbatim):** "Keep changes accurate and minimal; if a section already
> matches, leave it. Do NOT touch docs/cli.md/configuration.md/providers.md (already Mode-A updated)."
> Do NOT edit README.md (S1 owns it), `internal/cmd/config.go`, `PRD.md`, `tasks.json`, or any `.go` file.

## All Needed Context

### Context Completeness Check

**Pass** — this PRP quotes the exact current text of every edit target (with line numbers), the exact
shipped wording from the Go source (so the doc is accurate, not paraphrased), the canonical wording
from the already-synced `docs/cli.md`, the lint command, and the gotchas. An agent who has never seen
this repo can complete it from this file + `docs/how-it-works.md` + `docs/cli.md`.

### Documentation & References

```yaml
# MUST READ — the file being edited
- file: docs/how-it-works.md
  why: THE edit target. Read §"Safety and the rescue protocol" fully before editing.
  sections:
    - lines 53-64: "### Failure modes and exit codes" (the table + the "See cli.md" line)
    - lines 66-83: "### Rescue protocol" (intro at 68, the ```text block at 70-82, tail at 83)

# MUST READ — source of truth for wording (already synced in bugfix-002 Mode A)
- file: docs/cli.md
  why: the authoritative CLI reference; the canonical dry-run exit-1 wording lives here. Mirror it; do not contradict it.
  sections:
    - line 26: --dry-run row ends "… If generation fails (timeout or parse/duplicate-check exhaustion),
      exits **1** with a short stderr message instead of exit 3/124 + the full recovery recipe (since no
      commit was ever intended)"  → mirror substance for Edit B
    - line 86: "With `--dry-run`, generation failures (timeout or parse/duplicate-check exhaustion)
      report exit **1** with a short stderr message (not 3/124 + the recovery recipe) — codes 3 and 124
      remain the non-dry-run (commit-path) semantics."  → mirror substance for Edit B/C
  critical: docs/cli.md is the source of truth. The how-it-works.md wording must be a strict subset of it.

# MUST READ — the shipped behavior (quote these EXACT strings in the doc, do not paraphrase loosely)
- file: internal/git/git.go
  why: Issue 3's clean merge-conflict error message (line 230) — the accurate wording to reflect.
  string: |
    "unresolved merge conflicts in the index — resolve them first, then re-run stagecoach"
  semantics: exit 1, PRE-generation (WriteTree is step 3, before the model runs), HEAD/index untouched, no snapshot, no rescue.

- file: internal/cmd/default_action.go
  why: Issue 4's dry-run short messages (lines 181 & 183) + the exit-1 mapping (line 185).
  strings: |
    rescue (parse/dup exhaustion): "could not generate a commit message; run without --dry-run to see retries and the recovery recipe"
    timeout:                      "generation timed out; run without --dry-run to see the recovery recipe"
    → exitcode.New(exitcode.Error, nil)   // exit 1, NO FormatRescue recovery recipe printed
  semantics: the library (pkg/stagecoach) is UNCHANGED (still returns *RescueError 3/124); only the
    CLI rendering special-cases dry-run. The recipe + codes 3/124 remain the real-commit (default action) semantics.

# SUPPORTING — root cause + why each fix was made
- file: plan/001_f1f80943ac34/bugfix/002_18158df10968/architecture/issue_analysis.md
  why: verified file:line evidence + Mode-A doc notes for Issues 3 & 4. The Mode-A notes confirm how-it-works.md was DEFERRED here.
  sections: "## ISSUE 3 (Minor)" (WriteTree fix + "Documentation (Mode A)" note), "## ISSUE 4 (Minor)" (handleGenError fix + "Documentation (Mode A)" note), "## Cross-cutting: changeset-level documentation (Mode B — final task)"

- file: plan/001_f1f80943ac34/bugfix/002_18158df10968/architecture/system_context.md
  why: "UX wording (Issues 3 & 4)" — confirms Issue 3 is a WriteTree-message-only change and Issue 4 is a CLI-layer rendering change (library unchanged).

- file: plan/001_f1f80943ac34/bugfix/002_18158df10968/P1M5T1S2/research/mode-a-coverage-and-drift.md
  why: the git-diff audit confirming how-it-works.md is untouched by bugfix-002 and the resulting drift table.

- file: plan/001_f1f80943ac34/bugfix/002_18158df10968/P1M5T1S1/PRP.md
  why: the sibling Mode-B task (README). Its "minimal/accurate, mirror docs/cli.md, do not pad" discipline applies here. It explicitly defers Issue 3 to THIS task.
```

### Current Codebase tree (documentation surface only)

```bash
docs/
├── how-it-works.md       # ← THE edit target (Mode B, bugfix-002) — §"Failure modes" + §"Rescue protocol"
├── cli.md                # SOURCE OF TRUTH (Mode-A synced for Issue 4) — DO NOT EDIT here
├── configuration.md      # Mode-A synced (Issue 2) — DO NOT EDIT here
├── providers.md          # Mode-A synced (Issue 2) — DO NOT EDIT here
└── README.md             # docs index — VERIFY ONLY
README.md                 # ← DONE by S1 — DO NOT EDIT
internal/cmd/config.go    # config init template — DO NOT EDIT (out of scope)
internal/git/git.go       # Issue 3 source (line 230) — READ ONLY (quote the string)
internal/cmd/default_action.go  # Issue 4 source (lines 178-185) — READ ONLY (quote the strings)
.markdownlint.json        # lint config: default=true, MD013/MD033/MD060 off (MD060 non-standard → ignored)
.github/workflows/ci.yml  # CI (markdownlint is NOT wired in here)
```

### Desired Codebase tree

```bash
# No files added, moved, or deleted.
docs/how-it-works.md      # ← EDITED (Edit A row + Edit B dry-run exception + optional Edit C note)
# everything else untouched
```

### Known Gotchas of our codebase & library quirks

```text
# CRITICAL: markdownlint is NOT wired into the Makefile or CI (.github/workflows/ci.yml).
# The ONLY way to validate is the manual command:
#   npx markdownlint-cli2 docs/how-it-works.md
# (markdownlint-cli2 v0.22.1 / markdownlint v0.40.0 is cached and available via npx.)
# Current baseline: docs/how-it-works.md → 0 errors. Preserve it. Do NOT assume CI catches markdown errors.

# CRITICAL: .markdownlint.json disables MD013 (line-length) and MD033 (inline HTML), and lists
# "MD060" (NOT a standard markdownlint rule — silently ignored, no effect). Long lines and the
# existing ```text fenced blocks are fine; keep that style.

# CRITICAL: This is a PROSE-ONLY task. There are no Go tests for docs content. Validation is:
# (1) markdownlint clean, (2) coherence greps vs docs/cli.md (source of truth), (3) build sanity.
# Do NOT "improve" code, the config init template, README, or other docs.

# CRITICAL — quote the EXACT shipped strings, do not loosely paraphrase:
#   Issue 3 (git.go:230): "unresolved merge conflicts in the index — resolve them first, then re-run stagecoach"
#   Issue 4 dry-run messages (default_action.go:181/183): use the docs/cli.md substance (exit 1 + short
#   message, no recovery recipe). The how-it-works.md prose may be slightly more compact than cli.md but
#   must NOT contradict it.

# GOTCHA — Issue 3 is PRE-generation / PRE-snapshot. Merge conflicts make `git write-tree` (step 3)
# fail BEFORE the model is invoked and BEFORE any snapshot object is written. So the merge-conflict row
# is a 1 (Error) like "Agent missing", NOT a 3 (Rescue). Do NOT imply a snapshot/rescue is involved.
# (Do NOT touch Snapshot invariant #3 at line ~41 — it is correctly scoped to POST-snapshot failures.)

# GOTCHA — Issue 4's dry-run change does NOT alter the pipeline: --dry-run STILL runs the full
# snapshot→generate→parse→dedupe→retry pipeline (bugfix-001 Issue 2/6). The ONLY change is the FAILURE
# outcome (exit 1 + short message instead of 3/124 + recovery recipe). So the line "When generation
# fails after the snapshot is taken" is still literally true (the snapshot IS taken in dry-run) — the
# drift is that it then says "… prints a recovery block", which is now false for dry-run.

# GOTCHA — the library API (pkg/stagecoach) still returns *RescueError (3/124) for dry-run; only the
# CLI layer (handleGenError) special-cases it to exit 1. The doc should describe the USER-VISIBLE
# behavior (exit 1, no recipe), not the internal return type. Do not mention *RescueError in the doc.

# GOTCHA — docs/cli.md is the SOURCE OF TRUTH and was Mode-A synced for Issue 4. Mirror its wording
# (line 26 + line 86). Do not introduce a different exit-code story than cli.md tells.

# GOTCHA — DO NOT edit docs/cli.md, docs/configuration.md, docs/providers.md (Mode-A done),
# README.md (S1 done), internal/cmd/config.go, PRD.md, tasks.json, or any .go file.
```

## Implementation Blueprint

### Implementation Tasks (ordered — table first, then rescue protocol)

```yaml
Task 1 (REQUIRED, Edit A): add a merge-conflict row to the failure-modes table  (Issue 3)
  FILE: docs/how-it-works.md, "### Failure modes and exit codes" table (lines 55-62).
  WHERE: insert a new row. Place it right AFTER the "Agent missing on $PATH" row (line 57) — both are
         pre-generation exit-1 failures — OR adjacent to "Nothing to commit"; either grouping reads
         fine. Keep the three-column format: | Failure | Exit code | Recovery |.
  DRIFT: the table omits merge conflicts entirely (PRD §18.2 includes them). The contract OUTPUT
         requires noting "merge conflicts produce a clean 'resolve merge conflicts first' message
         (exit 1)".
  ADD (one row, wording mirrors git.go:230 substance — keep it tight):
         | Unresolved merge conflicts in the index | 1 (Error) | Resolve the conflicts, then re-run `stagecoach` (caught before the snapshot) |
  GOTCHA: merge conflicts are PRE-generation / PRE-snapshot → 1 (Error), NOT 3 (Rescue). Do NOT imply a
          snapshot or rescue is involved. The "caught before the snapshot" phrase pairs this row with
          the existing "Agent missing on $PATH" pre-generation framing.

Task 2 (REQUIRED, Edit B): scope the rescue recipe to real commits; add the dry-run exception  (Issue 4)
  FILE: docs/how-it-works.md, "### Rescue protocol" (lines 66-83).
  WHERE: (i) refine the intro at line 68 to scope the recipe to a real commit; (ii) add a dry-run
         exception paragraph at the END of the section (after line 83, before "## Prompt engineering").
  CURRENT intro (line 68, verbatim):
         "When generation fails after the snapshot is taken (exit 3 or 124), Stagecoach prints a recovery
         block to stderr with the frozen tree SHA and the exact `git commit-tree` command to commit manually:"
  CHANGE (minimal — append a scoping clause; keep the ```text block lines 70-82 UNCHANGED):
         "When generation fails after the snapshot is taken on a real commit (exit 3 or 124), Stagecoach
         prints a recovery block to stderr with the frozen tree SHA and the exact `git commit-tree`
         command to commit manually:"
    (Only addition: the words "on a real commit".)
  ADD (new paragraph at the end of the section, after line 83; mirror docs/cli.md line 86 substance):
         "Under `--dry-run`, the full pipeline still runs and the snapshot is still taken, but a generation
         failure (timeout or parse/duplicate-check exhaustion) exits **1** with a short stderr message and
         omits this recovery recipe — no commit was ever intended. The recipe and exit codes 3/124 apply
         to a real `stagecoach` commit."
  GOTCHA: do NOT weaken the existing "after the snapshot is taken" accuracy (the snapshot IS taken in
          dry-run). Only scope the RECIPE. Do NOT change the ```text rescue-block example (lines 70-82)
          or the line-83 rejected-candidate note — those describe the real-commit recipe and are still
          correct. Mention only user-visible behavior (exit 1), NOT the internal *RescueError type.

Task 3 (RECOMMENDED, Edit C): one-line note under the failure-modes table  (Issue 4)
  FILE: docs/how-it-works.md, the line right after the table, currently line 64:
         "See [cli.md](cli.md#exit-codes) for the full exit-code table."
  WHERE: add ONE short sentence (before or after the existing "See cli.md" line) noting the dry-run
         divergence, with a cross-ref to the Rescue protocol section.
  DRIFT: the table's "Generation failed → 3 (Rescue)" and "Generation timed out → 124 (Timeout)" rows
         read as universal, but under --dry-run those failures exit 1.
  ADD (one line; keep compact):
         "The rescue (3) and timeout (124) rows are the real-commit path; under `--dry-run`, a generation
         failure reports exit 1 instead — see [Rescue protocol](#rescue-protocol)."
  GOTCHA: this is a cross-reference to Edit B's substance, NOT a duplicate explanation. Keep it to one
          line. If it reads as padding, SKIP it (Edit B covers the substance) — do not force a worse
          phrasing. The contract says "minimal".

Task 4 (VERIFY-ONLY pass): confirm no OTHER overview drift exists
  - Read docs/how-it-works.md end-to-end.
  - Confirm Snapshot invariant #3 (line ~41) and the "Plumbing alternative" write-tree description
    (line ~18-24) are internally consistent with the merge-conflict pre-flight (pre-snapshot) — they
    are; do NOT touch them.
  - Confirm the ```text rescue-block example (lines 70-82) wording still matches the real-commit
    FormatRescue output — do NOT edit it (out of scope; unchanged).
  - If you find drift beyond A/B/C, fix it minimally (mirror docs/cli.md). If none, do not invent edits.
```

### Implementation Patterns & Key Details

```markdown
# PATTERN: docs/cli.md is the source of truth. Every behavioral claim added to how-it-works.md must
# agree with it (line 26 + line 86 for dry-run; the exit-code table at line 76). Do not contradict.

# PATTERN: keep the three-column table format exactly (| Failure | Exit code | Recovery |) and the
# existing ```text fenced-block style. markdownlint allows them (MD031/MD040 are satisfied; MD013 off).

# CRITICAL: quote the shipped strings accurately. Issue 3's message is "resolve them first, then
# re-run stagecoach" (git.go:230); Issue 4's dry-run outcome is "exit 1 + short message, no recipe"
# (cli.md:26/86). Do not invent wording the binary doesn't produce.

# CRITICAL: do NOT change the rescue-block ```text example (lines 70-82) or line 83. They describe the
# real-commit recipe, which is unchanged. Only ADD scope ("on a real commit") and a dry-run exception.
```

### Integration Points

```yaml
NONE — this is a single-file prose/table edit. No config, routes, database, or code integration.
The only "integration" is textual consistency with the already-synced docs/cli.md (source of truth).
```

## Validation Loop

> This is a documentation task. No Go tests, no build step, no server. Validation is lint +
> coherence + integrity. **markdownlint is NOT in CI** — run the commands below manually.

### Level 1: Markdown lint (the primary gate)

```bash
# From the repo root. MUST show "Summary: 0 error(s)".
npx markdownlint-cli2 docs/how-it-works.md

# (Optional) lint all docs together to confirm you didn't accidentally need another doc change:
npx markdownlint-cli2 'docs/**/*.md'
# Expected: 0 error(s). If docs/cli.md/configuration.md/providers.md "fail", that is NOT your task
# (they are Mode-A done) — flag it, do not fix.
```

### Level 2: Coherence check vs the source of truth (docs/cli.md) + contract checks

```bash
# (1) merge conflicts → clean message, exit 1, pre-generation (Issue 3) — the new table row exists:
grep -n "merge conflict\|resolve.*conflict\|caught before the snapshot\|before the snapshot" docs/how-it-works.md
# Expected: the new row (and only there).

# (2) dry-run generation failure → exit 1, no recovery recipe (Issue 4) — the rescue protocol + table note:
grep -n "dry-run\|exit 1\|exit \*\*1\*\*\|recovery recipe\|no commit was ever intended\|3/124" docs/how-it-works.md docs/cli.md
# Expected: how-it-works.md now states dry-run failures exit 1 + omit the recipe; agrees with docs/cli.md.

# (3) the rescue-block example and Snapshot invariant #3 are UNCHANGED (you didn't over-edit):
grep -n "Commit generation failed\|Tree ID:\|orphan tree/commit objects" docs/how-it-works.md
# Expected: still present, verbatim.
# Expected: docs agree in substance. Resolve any disagreement by aligning how-it-works.md to docs/cli.md.
```

### Level 3: Integrity — only how-it-works.md changed, no scope creep

```bash
# Confirm only docs/how-it-works.md changed (no cli/configuration/providers, no README, no .go):
git status --short
# Expected: M docs/how-it-works.md   (only this). MUST NOT include README.md, docs/cli.md,
# docs/configuration.md, docs/providers.md, internal/cmd/config.go, PRD.md, tasks.json, or any *.go.

# Confirm the Mode-A cli.md wording is intact (you mirrored it, not mutated it):
git show 04508d3:docs/cli.md | sed -n '26p'   # the --dry-run row — still present verbatim
git show 04508d3:docs/cli.md | sed -n '86p'   # the dry-run exit-code note — still present verbatim
```

### Level 4: Build sanity (no code touched — confirm you didn't break the build by accident)

```bash
# If you ONLY edited docs/how-it-works.md, this is a no-op confirmation. Run it to be safe.
go build ./... && go vet ./... && go test ./... 2>&1 | tail -5
# Expected: clean build/vet and all tests pass. (You should not have touched any .go file.)
```

## Final Validation Checklist

### Technical / Lint

- [ ] `npx markdownlint-cli2 docs/how-it-works.md` → `0 error(s)`
- [ ] `git status --short` shows ONLY `docs/how-it-works.md`
- [ ] `go build ./... && go vet ./... && go test ./...` still pass (sanity — no code touched)

### Coherence (the two contract reconciliations)

- [ ] **A (REQUIRED, Issue 3):** the failure-modes table lists merge conflicts → 1 (Error), pre-snapshot,
      with the clean "resolve … then re-run" recovery text
- [ ] **B (REQUIRED, Issue 4):** the §"Rescue protocol" scopes the recovery recipe to a real commit and
      states `--dry-run` generation failures exit 1 with a short message (no recipe)
- [ ] **C (RECOMMENDED, if done, Issue 4):** the note under the table cross-refs the dry-run exception
- [ ] The rescue-block ```text example (lines 70-82) and the line-83 rejected-candidate note are UNCHANGED
- [ ] Snapshot invariant #3 and the write-tree "Plumbing alternative" description are UNCHANGED
- [ ] Every behavioral claim in how-it-works.md agrees with `docs/cli.md` (source of truth)

### Integrity / Discipline

- [ ] No edits to `docs/cli.md`, `docs/configuration.md`, `docs/providers.md` (Mode-A done),
      `README.md` (S1 done), `internal/cmd/config.go`, `PRD.md`, `tasks.json`, or any `.go` file
- [ ] Minimal edits only — no padding, no whole-section rewrites (contract: "Keep changes accurate and minimal")
- [ ] Quotes the EXACT shipped strings (git.go:230 merge-conflict message; cli.md:26/86 dry-run outcome)
- [ ] Reuses the existing three-column table format and ```text fenced-block style; no new format introduced

---

## Anti-Patterns to Avoid

- ❌ Don't edit `docs/cli.md`, `docs/configuration.md`, `docs/providers.md` — they are already Mode-A
  synced (Issues 1, 2, 4). This task owns ONLY `docs/how-it-works.md`.
- ❌ Don't edit `README.md` (S1 owns it, done), `internal/cmd/config.go`, `PRD.md`, `tasks.json`, or any `.go` file.
- ❌ Don't imply merge conflicts trigger a rescue/snapshot — they fail PRE-generation at `write-tree`
  (step 3), exit 1, no snapshot object. The row is 1 (Error), not 3 (Rescue).
- ❌ Don't change the rescue-block ```text example (lines 70-82) or line 83 — they describe the real-commit
  recipe, which is unchanged. Only scope the intro and add the dry-run exception.
- ❌ Don't weaken the "the snapshot is still taken under --dry-run" accuracy — bugfix-002 did NOT change
  the pipeline, only the failure exit code/rendering. Only the RECIPE is omitted under dry-run.
- ❌ Don't mention the internal `*RescueError` type or the library/CLI split — the doc describes
  user-visible behavior (exit 1, no recipe), not internals.
- ❌ Don't invent wording the binary doesn't produce — quote git.go:230 and mirror docs/cli.md:26/86.
- ❌ Don't skip the manual `npx markdownlint-cli2 docs/how-it-works.md` run assuming CI will catch it —
  markdownlint is NOT in CI.
- ❌ Don't pad — the contract says "Keep changes accurate and minimal; if a section already matches, leave it."
