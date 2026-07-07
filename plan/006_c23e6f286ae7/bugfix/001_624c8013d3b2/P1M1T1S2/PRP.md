---
name: "P1.M1.T1.S2 — Qualify the docs/cli.md contention-behavior prose (line 379)"
description: |
  Documentation-only fix (Issue 1, Major — doc-vs-behavior mismatch, second of three sibling doc fixes).
  `docs/cli.md:379` currently describes the FR52 no-op fast path **generically**: "if a contending run's
  staged changes are already covered by the in-progress run's published snapshot, it exits **0**". This
  is **false on the decompose path**: the holder publishes a working-tree snapshot (`T_start`) a lock-free
  contender cannot reproduce from an index `write-tree` (it returns `baseTree` == `HEAD^{tree}`), so the
  contender always exits `5` (Busy), never 0. Rewrite line 379's paragraph to scope exit-0 to the
  **single-commit (staged) path** (keeping BOTH its outcomes: 0 nothing-to-do AND 5 Busy-if-new-work) and
  add that the **decompose path** exits `5` (Busy) on an accidental double-run. Preserve the first
  sentence ("Code 5 is distinct…") and the last sentence ("Stagecoach never force-breaks the lock.")
  verbatim. Do NOT edit the exit-code TABLE (lines 368-375) — it does not over-claim. NO code changes.
  README.md:330 (S1) and docs/how-it-works.md:155 (S3) are sibling subtasks.
---

## Goal

**Feature Goal**: Make the `docs/cli.md` Exit-codes contention-behavior prose **honest** by scoping the
exit-0 (no-op fast path) promise to the path where it actually holds — the **single-commit (staged)**
path — and explicitly documenting that on the **decompose path** an accidental double-run exits `5`
(Busy) because the holder publishes a working-tree snapshot (`T_start`) a lock-free contender cannot
reproduce from the index alone.

**Deliverable**: The rewritten contention-behavior paragraph at `docs/cli.md:379` (the paragraph
beginning "Code `5` (Busy) is distinct…"), scoped per-path. No other line of `docs/cli.md` changes —
the exit-code table (368-375) and the "Exit codes mirror the constants…" explanation paragraph are
unchanged. No code changes anywhere.

**Success Definition**: A reader of the `docs/cli.md` Exit-codes section knows the single-commit path
can exit `0` (nothing to do) or `5` (Busy if new work is staged), and the decompose path exits `5`
(Busy) on an accidental double-run — and why (working-tree `T_start` vs index `write-tree` snapshot-axis
mismatch). The first and last sentences of the paragraph are unchanged; the exit-code table is untouched;
markdown is valid; no code/test/other-doc files are touched; `go test ./...` is unaffected.

## User Persona

**Target User**: The Stagecoach user (or script author) reading the `docs/cli.md` Exit-codes section to
understand code `5` (Busy) — especially someone binding `stagecoach` to a keybind (lazygit/double-tap)
who needs to know what an accidental double-run does on each commit path, and a reviewer auditing the
docs for accuracy against the implemented FR52 behavior.

**Use Case**: A user double-invokes `stagecoach` on a dirty, un-staged working tree (the decompose
trigger) and consults `docs/cli.md` to learn the exit code. The current prose implies exit 0 is
reachable; the fixed prose tells them it exits Busy (5) on the decompose path and why.

**Pain Points Addressed**: Removes a false claim. The current text sets an expectation (decompose
double-run can reach exit 0) the binary provably does not meet (it exits 5), which erodes trust in the
rest of the exit-code documentation once a user observes the mismatch.

## Why

- **Honesty in the concurrency documentation.** The exit-code section is the authoritative reference for
  scripting around `stagecoach`. A generic "exits 0" contention clause that is false on an entire commit
  path (decompose) undermines the reference the moment a user double-taps a keybind on a dirty tree.
- **Doc-only is the PRD-recommended fix (Option 1).** `issue_analysis.md` (Issue 1) evaluated two
  options: (1) qualify the documentation (lowest-risk, makes docs honest) vs (2) a more-invasive
  holder/contender snapshot-axis code change (non-trivial, risks false no-ops, violates the
  index-read-only-without-the-lock invariant). Option 1 — this subtask — is recommended. The behavior
  (decompose → Busy) is correct and safe; only the doc over-promises.
- **Root cause is structural, not a bug.** The holder publishes a **working-tree** snapshot (`T_start`,
  `internal/decompose/decompose.go:169`); the contender can only compute an **index** snapshot
  (`git write-tree`) without the lock. With nothing staged (decompose's activation condition, FR-M1) the
  index tree == `baseTree` == `HEAD^{tree}` ≠ `T_start`. So `contenderTree == snap` is always false on
  decompose → Busy(5). This is the designed defense-in-depth behavior; the doc must describe it as such.
- **Aligns the three doc surfaces.** S1 (README:330), S2 (this — cli.md:379), and S3 (how-it-works.md:155)
  carry one consistent per-path story. S2 is the cli.md reference-surface piece.

## What

A single-paragraph rewrite of the contention-behavior paragraph at `docs/cli.md:379`. The generic
"exits 0" clause becomes path-scoped (single-commit only, keeping both its outcomes); a new clause
documents the decompose path exits Busy. The first sentence ("Code `5` (Busy) is distinct…") and the
last sentence ("Stagecoach never force-breaks the lock.") are preserved verbatim. Everything else in the
Exit-codes section (the table 368-375 and the "Exit codes mirror…" explanation paragraph) is unchanged.

### Success Criteria

- [ ] `docs/cli.md:379` scopes the exit-0 (no-op fast path) to the **single-commit (staged) path**.
- [ ] The single-commit path still documents BOTH outcomes: exit **0** (nothing to do) AND exit **5** (Busy if genuinely new work is staged).
- [ ] The paragraph states the **decompose path** (nothing staged, dirty working tree) exits **5** (Busy) on an accidental double-run, and why (working-tree snapshot `T_start` a lock-free contender cannot reproduce from the index alone).
- [ ] The first sentence ("Code `5` (Busy) is distinct from the commit-failure codes so scripts can tell 'busy, retry' from 'failed.'") is UNCHANGED.
- [ ] The last sentence ("Stagecoach never force-breaks the lock.") is UNCHANGED.
- [ ] The "nothing to do — an in-progress run already covers your staged changes" string is preserved.
- [ ] The exit-code TABLE (lines 368-375) is UNCHANGED.
- [ ] The "Exit codes mirror the constants in `internal/exitcode/exitcode.go`…" explanation paragraph is UNCHANGED.
- [ ] NO code files, NO test files, NO other doc files are touched (README.md = S1; docs/how-it-works.md = S3; PRD.md / tasks.json / prd_snapshot.md / plan/* untouched).
- [ ] `go test ./...` still passes (doc-only sanity check).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the EXACT current paragraph (verbatim, `docs/cli.md:379`), the
EXACT target paragraph (ready to paste), the exact sentences to preserve (first + last), and the exact
lines that must NOT change (the table 368-375 + the "Exit codes mirror…" paragraph). The issue root cause
(working-tree `T_start` vs index `baseTree` snapshot-axis mismatch) is stated so the implementer
understands the *why* behind the wording, not just the *what*.

### Documentation & References

```yaml
# MUST READ — the issue + root cause
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/issue_analysis.md
  why: "Issue 1 documents the doc-vs-behavior mismatch: the no-op fast path (exit 0) never fires on the decompose path because the holder publishes T_start (working-tree, decompose.go:169) while the contender computes baseTree (index) via write-tree (index empty at decompose activation → write-tree returns HEAD^{tree}). Recommends Option 1 (qualify the docs) as the lowest-risk fix. Gives the empirical repro (snapshot=c0f5cf74… vs write-tree=d7a57d28…==HEAD^{tree})."
  critical: "Confirms the behavior (decompose → Busy) is CORRECT and safe — only the doc over-promises. This subtask is the doc fix (Option 1); do NOT attempt Option 2 (holder/contender snapshot-axis code change) — out of scope and riskier. NO code change."

- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/system_context.md
  why: "Line 29 lists docs/cli.md (line 379) as the 'Contention-behavior prose under exit-code table' surface, scope 1 — confirms this is S2's exact target and that the table is a separate surface (not in scope)."

- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M1T1S1/PRP.md
  why: "The README.md sibling (S1). Carries the SAME per-path semantics (single-commit → 0-or-5; decompose → 5) in the README's voice. S2 mirrors it for docs/cli.md's reference voice. S1 explicitly defers docs/cli.md to S2 — different files, no conflict. Use S1's target paragraph as the coherence reference for the 'nothing to do…' string and the working-tree-snapshot rationale."

# The file under edit
- file: docs/cli.md
  why: "EDIT line 379 ONLY (the contention-behavior paragraph beginning 'Code `5` (Busy) is distinct…'). It is ONE logical markdown line. The exit-code TABLE (lines 368-375) and the 'Exit codes mirror the constants…' explanation paragraph (immediately above 379) are UNCHANGED — they are correct and do not over-claim."
  pattern: "The current paragraph is one long markdown line with: first sentence (Code 5 distinct…), a 'two behaviors' clause with two exit-code cases (0 / 5), and a last sentence (Stagecoach never force-breaks the lock.). The rewrite keeps that shape, splits the cases by PATH (single-commit vs decompose), preserves the first+last sentences, and adds the decompose-path Busy clause."
  gotcha: "Do NOT edit the exit-code table (368-375). Do NOT edit the 'Exit codes mirror…' paragraph. Keep the paragraph as ONE logical markdown line (the file already uses long single-line paragraphs and .markdownlint.json is configured — do not hard-wrap). Preserve the first and last sentences verbatim."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── docs/cli.md          # EDIT line 379 (the contention-behavior paragraph)
```

### Desired Codebase Tree After S2

```bash
stagecoach/
└── docs/cli.md          # line 379 rewritten (path-scoped exit-0 + decompose→Busy); rest unchanged
```

| Path | Action | Responsibility |
|---|---|---|
| `docs/cli.md` | MODIFY (line 379 only) | Scope the exit-0 no-op fast path to the single-commit path; document decompose→Busy; preserve the first + last sentences. |

**Explicitly NOT touched**: the `docs/cli.md` exit-code table (lines 368-375) + the "Exit codes
mirror…" explanation paragraph, `README.md` (S1), `docs/how-it-works.md` (S3), any `.go`/test file
(no code change), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```markdown
<!-- CRITICAL: edit ONLY the contention-behavior paragraph at docs/cli.md:379 (begins "Code `5` (Busy)
     is distinct…"). The exit-code TABLE (368-375) is generic and does NOT over-claim — leave it.
     The "Exit codes mirror the constants in internal/exitcode/exitcode.go…" paragraph above 379 is
     correct and unrelated — leave it. -->

<!-- CRITICAL: preserve the FIRST sentence verbatim — "Code `5` (Busy) is distinct from the commit-failure
     codes so scripts can tell "busy, retry" from "failed."" — and the LAST sentence verbatim —
     "Stagecoach never force-breaks the lock." The contract requires both. -->

<!-- GOTCHA: keep BOTH single-commit outcomes (exit 0 nothing-to-do AND exit 5 Busy-if-new-work-staged).
     Do NOT drop the Busy case for the single-commit path — only the *unconditional/generic* "exits 0"
     framing is wrong; the single-commit path still exits Busy when genuinely new work is staged. -->

<!-- GOTCHA: the paragraph is ONE logical markdown line. .markdownlint.json is configured (root) and the
     existing line 379 is already a long single line, so the project allows long lines. Keep the rewrite
     as ONE logical line (do NOT hard-wrap into multiple lines). -->

<!-- GOTCHA: this is doc-only. `go test ./...` is a sanity check that nothing was accidentally clobbered —
     it does NOT validate prose. The prose check is: accurate, coherent, path-scoped, first+last
     sentences preserved, table untouched. -->

<!-- GOTCHA: do NOT touch README.md (S1 owns it) or docs/how-it-works.md (S3 owns it). Those carry the
     same per-path semantics in their own voices; S2 is docs/cli.md ONLY. -->
```

## Implementation Blueprint

### Data models and structure

N/A — documentation-only. No types, no code.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: REWRITE the contention-behavior paragraph (docs/cli.md:379)
  - LOCATE: docs/cli.md, the paragraph beginning "Code `5` (Busy) is distinct from the commit-failure
    codes…" (line 379), under the "## Exit codes" heading (line 366), immediately after the "Exit codes
    mirror the constants…" explanation paragraph and immediately before the "## Flag ↔ env ↔
    git-config map" heading (line 381).
  - PRESERVE VERBATIM: the exit-code TABLE (lines 368-375) and the "Exit codes mirror the constants in
    `internal/exitcode/exitcode.go`…" explanation paragraph. Do not touch them.
  - REPLACE the current paragraph (the single long line at 379) with the path-scoped version. Target
    (ONE logical markdown line — paste verbatim):
        Code `5` (Busy) is distinct from the commit-failure codes so scripts can tell "busy, retry" from "failed." Contention on the per-repo run lock (FR52) has two behaviors. On the single-commit path (changes staged): if a contending run's staged changes are already covered by the in-progress run's published index snapshot, it exits **0** ("nothing to do — an in-progress run already covers your staged changes"); if genuinely new work is staged, it exits **5** with the holder's pid/host and leaves the new changes staged for a re-run. On the decompose path (nothing staged, working tree dirty): an accidental double-run exits **5** (Busy) rather than 0 — the holder publishes a working-tree snapshot (`T_start`) that a lock-free contender cannot reproduce from the index alone, so it conservatively refuses. Stagecoach never force-breaks the lock.
  - PRESERVED in that target: first sentence (Code 5 distinct…), last sentence (Stagecoach never
    force-breaks the lock.), the "nothing to do…" string, and BOTH single-commit outcomes (0 + 5).
  - ADDED: path scoping ("On the single-commit path" / "On the decompose path"), the decompose Busy
    clause with the `T_start` rationale, and "index snapshot" precision on the single-commit side.
  - VOICE: reference-doc tone (matches docs/cli.md's existing exit-code prose). Terms: "single-commit
    path" / "decompose path"; exit codes `0`/`5` in backticks/bold consistent with the paragraph's
    existing style.
  - DO NOT: change the table, the explanation paragraph, any heading, or any other doc/code/test file.

Task 2: VERIFY (doc-only validation)
  - RUN: go build ./... && go test ./...     # sanity — no file accidentally clobbered (doc change)
  - RUN (if markdownlint available): markdownlint docs/cli.md  OR  npx -y markdownlint-cli2 docs/cli.md
    (.markdownlint.json is configured). Expected: no NEW errors (keep the paragraph one logical line;
    backticks/bold/quotes balanced).
  - READ CHECK (sed -n '366,381p' docs/cli.md): the Exit-codes section reads coherently — table →
    "Exit codes mirror…" → "Code 5 is distinct…" (now path-scoped, single-commit 0-or-5 + decompose
    Busy) → next heading. The exit-0 promise is scoped to the single-commit path; the decompose path is
    documented as Busy.
  - git diff --stat → ONLY docs/cli.md changed (1 file, +1/-1-ish line).
```

### Implementation Patterns & Key Details

```markdown
<!-- === CURRENT docs/cli.md:379 (the line to replace) === -->
Code `5` (Busy) is distinct from the commit-failure codes so scripts can tell "busy, retry" from "failed." Contention on the per-repo run lock (FR52) has two behaviors: if a contending run's staged changes are already covered by the in-progress run's published snapshot, it exits **0** ("nothing to do — an in-progress run already covers your staged changes"); if genuinely new work is staged, it exits **5** with the holder's pid/host and leaves the new changes staged for a re-run. Stagecoach never force-breaks the lock.

<!-- === TARGET docs/cli.md:379 (path-scoped; ONE logical markdown line; first+last sentences preserved) === -->
Code `5` (Busy) is distinct from the commit-failure codes so scripts can tell "busy, retry" from "failed." Contention on the per-repo run lock (FR52) has two behaviors. On the single-commit path (changes staged): if a contending run's staged changes are already covered by the in-progress run's published index snapshot, it exits **0** ("nothing to do — an in-progress run already covers your staged changes"); if genuinely new work is staged, it exits **5** with the holder's pid/host and leaves the new changes staged for a re-run. On the decompose path (nothing staged, working tree dirty): an accidental double-run exits **5** (Busy) rather than 0 — the holder publishes a working-tree snapshot (`T_start`) that a lock-free contender cannot reproduce from the index alone, so it conservatively refuses. Stagecoach never force-breaks the lock.
```

```markdown
<!-- === UNCHANGED context (for reference — DO NOT edit) === -->
<!-- line 366:  ## Exit codes -->
<!-- lines 368-375: the exit-code TABLE (| Code | Meaning | … | `124` | Timeout |) -->
<!-- the "Exit codes mirror the constants in internal/exitcode/exitcode.go…" explanation paragraph (above 379) -->
<!-- line 381:  ## Flag ↔ env ↔ git-config map -->
```

### Integration Points

```yaml
DOC (docs/cli.md): line 379 rewritten ONLY — scope exit-0 to single-commit path; add decompose→Busy; preserve first+last sentences

NO-TOUCH (explicitly):
  - docs/cli.md exit-code table (368-375) + "Exit codes mirror…" paragraph   # correct, do not over-claim-edit
  - README.md                                # P1.M1.T1.S1 (sibling — same semantics, README voice)
  - docs/how-it-works.md                     # P1.M1.T1.S3 (sibling — "No-op fast path" subsection)
  - any .go file / test file                 # doc-only change
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — sibling/later subtasks):
  - S1 (P1.M1.T1.S1): scopes README.md:330 "Safe to run twice" (same per-path semantics).
  - S3 (P1.M1.T1.S3): qualifies docs/how-it-works.md:155 "No-op fast path" subsection.
  - P1.M1.T2.S1: adds e2e scenario F (decompose accidental double-run → Busy) that pins the documented exit code.
  - P1.M3.T1 (Mode B): final docs/cli.md + how-it-works.md + README coherence sweep.
```

## Validation Loop

### Level 1: Doc Lint & Sanity (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

# (markdownlint is configured — .markdownlint.json at root)
markdownlint docs/cli.md 2>/dev/null || npx -y markdownlint-cli2 docs/cli.md 2>/dev/null || echo "markdownlint not runnable; visual check only"
# Expected: no NEW errors (keep the paragraph ONE logical line; backticks/bold/quotes balanced).

# Sanity: only docs/cli.md changed
git diff --stat
# Expected: docs/cli.md only (1 file).
```

### Level 2: Build/Test Sanity (no code changed → must stay green)

```bash
cd /home/dustin/projects/stagecoach

go build ./...     # Expected: exit 0 (doc change; confirms no accidental source clobber)
go test ./...      # Expected: all green (doc change; confirms nothing else touched)
```

### Level 3: Prose Accuracy Check (the real validation)

```bash
cd /home/dustin/projects/stagecoach

# Render the Exit-codes section and verify each claim against the implemented behavior:
sed -n '366,381p' docs/cli.md

# CHECK each of these is TRUE in the rewritten paragraph:
#   1. exit 0 is scoped to the SINGLE-COMMIT path (not generic/unconditional).
#   2. The single-commit path still shows BOTH exit 0 (nothing-to-do) AND exit 5 (Busy-if-new-work).
#   3. The DECOMPOSE path is documented as exit 5 (Busy), with the "working-tree snapshot (T_start) a
#      lock-free contender cannot reproduce from the index alone" rationale.
#   4. The FIRST sentence ("Code `5` (Busy) is distinct from the commit-failure codes…") is UNCHANGED.
#   5. The LAST sentence ("Stagecoach never force-breaks the lock.") is UNCHANGED.
#   6. The exit-code TABLE (368-375) and the "Exit codes mirror…" paragraph are UNCHANGED.
```

### Level 4: Cross-Doc Coherence (light — full sweep is P1.M3.T1)

```bash
cd /home/dustin/projects/stagecoach

# Confirm the per-path semantics are consistent in spirit with the sibling docs (S1/S3 fully align;
# here just flag gross contradictions). The "nothing to do — an in-progress run already covers your
# staged changes" string should be recognizably the same notion across README/cli.md/how-it-works.md,
# and all three should scope exit-0 to the single-commit path + document decompose→Busy.
grep -n "nothing to do\|single-commit\|decompose path\|exits .0.\|Busy" README.md docs/cli.md docs/how-it-works.md | head
# (No assertion required in S2 — just ensure docs/cli.md no longer makes the unconditional claim.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` + `go test ./...` green (doc-only sanity check).
- [ ] `git diff --stat` shows ONLY `docs/cli.md` changed.

### Feature Validation

- [ ] `docs/cli.md:379` scopes exit 0 to the **single-commit (staged) path**.
- [ ] The single-commit path documents BOTH exit **0** (nothing to do) AND exit **5** (Busy if new work staged).
- [ ] The **decompose path** is documented as exit **5** (Busy) with the working-tree-`T_start` rationale.
- [ ] The first sentence ("Code `5` (Busy) is distinct…") is UNCHANGED.
- [ ] The last sentence ("Stagecoach never force-breaks the lock.") is UNCHANGED.
- [ ] The exit-code table (368-375) and the "Exit codes mirror…" paragraph are UNCHANGED.

### Scope Discipline Validation

- [ ] ONLY `docs/cli.md` modified (git diff --stat confirms).
- [ ] Did NOT touch the exit-code table (368-375) or the "Exit codes mirror…" paragraph.
- [ ] Did NOT touch `README.md` (S1), `docs/how-it-works.md` (S3), or any `.go`/test file.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Paragraph is ONE logical markdown line (matches docs/cli.md's paragraph style + markdownlint config).
- [ ] Inline markup balanced: backticks (`` `5` ``, `` `0` ``, `` `T_start` ``), bold (`**0**`, `**5**`), quotes.
- [ ] Voice matches the surrounding Exit-codes reference prose (direct, technical).

---

## Anti-Patterns to Avoid

- ❌ Don't describe the exit-0 fast path generically/unconditionally — that's the exact false claim being
  fixed. Scope it to the single-commit (staged) path.
- ❌ Don't drop the single-commit path's Busy case. Only the *generic* "exits 0" framing is wrong; the
  single-commit path still exits Busy when genuinely new work is staged — keep both outcomes.
- ❌ Don't edit the exit-code TABLE (368-375) or the "Exit codes mirror…" paragraph — they are correct and
  do not over-claim. Only the contention-behavior paragraph (379) over-promises.
- ❌ Don't change the first sentence ("Code `5` (Busy) is distinct…") or the last sentence ("Stagecoach
  never force-breaks the lock.") — the contract requires both verbatim.
- ❌ Don't attempt Option 2 (the holder/contender snapshot-axis code change). `issue_analysis.md`
  recommends Option 1 (this doc fix) as lowest-risk; the behavior is correct, only the doc over-promises.
  Code changes are out of scope.
- ❌ Don't edit `README.md` (S1) or `docs/how-it-works.md` (S3) — those are sibling subtasks carrying the
  same per-path semantics in their own voices; editing them here crosses the subtask boundary.
- ❌ Don't hard-wrap the paragraph into multiple lines — docs/cli.md uses single-line markdown paragraphs
  and `.markdownlint.json` is configured; keep the rewrite as ONE logical line.
- ❌ Don't introduce a claim that isn't backed by the behavior (e.g. "decompose exits 0 if…"). Every
  exit-code claim must match what the binary does (single-commit: 0 or 5; decompose: 5).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a single-paragraph, doc-only edit with the exact current text quoted verbatim, the
exact target paragraph provided ready-to-paste, the exact sentences to preserve (first + last) called
out, and the exact lines that must NOT change (the table 368-375 + the "Exit codes mirror…" paragraph)
named. The root cause (working-tree `T_start` vs index `baseTree` snapshot-axis mismatch) is documented
so the wording's "why" is grounded. `issue_analysis.md` explicitly recommends this doc-qualification
(Option 1) over the riskier code change, and the contract supplies the authoritative target phrasing.
S1 (README) is the structural template and confirms the per-path semantics. The only residual uncertainty
(not 10/10) is minor voice/wording taste that P1.M3.T1's Mode-B coherence sweep will finalize — the
PRP's target paragraph is a faithful, ready-to-paste starting point. No code, no tests, no other docs
are in scope, so the blast radius is one line of one file.
