---
name: "P1.M1.T1.S1 — Scope the README 'Safe to run twice' exit-0 claim to the single-commit path"
description: |
  Documentation-only fix (Issue 1, Major — doc-vs-behavior mismatch). README.md:330 currently claims
  unconditionally that an accidental double-run "exits `0`" when nothing new is staged. On the decompose
  path (nothing staged, dirty working tree) this is FALSE: the holder publishes a working-tree snapshot
  (T_start) the contender cannot reproduce from a lock-free index `write-tree` (it returns baseTree =
  HEAD^{tree}), so the contender always exits `5` (Busy), never 0. Rewrite the "Safe to run twice."
  paragraph so the exit-0 promise is scoped to the STAGED (single-commit) path and add that the decompose
  path exits `5` (Busy). Preserve the per-host/CAS caveat sentence and the line 328 "No. Stagecoach uses
  git write-tree…" sentence unchanged. NO code changes. docs/cli.md:379 and docs/how-it-works.md:155 are
  sibling subtasks (S2/S3) — NOT this one.
---

## Goal

**Feature Goal**: Make the README's headline "Safe to run twice" safety claim *honest* by scoping its
exit-0 (no-op fast path) promise to the path where it actually holds — the **single-commit (staged)**
path — and explicitly documenting that on the **decompose** path an accidental double-run exits `5`
(Busy) because the holder publishes a working-tree snapshot a lock-free contender cannot reproduce.

**Deliverable**: The rewritten `**Safe to run twice.**` paragraph at `README.md:330`, adapted to the
README's voice, scoped per-path. No other line of README.md changes (line 328 unchanged; heading line 326
unchanged; the per-host/CAS parenthetical preserved). No code changes anywhere.

**Success Definition**: The README no longer promises exit 0 unconditionally for "nothing new is staged";
a reader knows the single-commit path can exit 0 (nothing to do) or 5 (Busy if new work is staged), and
the decompose path exits 5 (Busy) on an accidental double-run. The surrounding FAQ answer remains
coherent; markdown is valid; no code/test files are touched; `go test ./...` is unaffected.

## User Persona

**Target User**: The Stagecoach user reading the README's "Will it corrupt my repo?" FAQ — especially
someone binding `stagecoach` to a keybind (lazygit/double-tap) who relies on the "safe to run twice"
claim, and a reviewer auditing the README for accuracy against the implemented FR52 behavior.

**Use Case**: A user double-invokes `stagecoach` on a dirty, un-staged working tree (the decompose
trigger) and wants to know what happens. The current README implicitly promises exit 0; the fixed README
tells them it exits Busy (5) and why.

**Pain Points Addressed**: Removes a false safety/UX claim. The current text sets an expectation
(decompose double-run → exit 0) the binary provably does not meet (it exits 5), which erodes trust in
the rest of the safety documentation once a user observes the mismatch.

## Why

- **Honesty in the headline safety claim.** "Safe to run twice" is the README's marquee concurrency
  assurance. An unconditional "exits 0" sub-clause that is false on an entire commit path (decompose)
  undermines the whole pitch the moment a user double-taps a keybind on a dirty tree.
- **Doc-only is the PRD-recommended fix (Option 1).** The issue_analysis.md (Issue 1) evaluated two
  options: (1) qualify the documentation (lowest-risk, makes docs honest) vs (2) a more-invasive holder/
  contender snapshot-axis change (non-trivial, risks false no-ops). Option 1 — this subtask — is
  recommended. The behavior (decompose → Busy) is correct and safe; only the doc over-promises.
- **Root cause is structural, not a bug.** The holder publishes a **working-tree** snapshot (T_start);
  the contender can only compute an **index** snapshot (`write-tree`) without the lock. With nothing
  staged the index tree == baseTree == HEAD^{tree} ≠ T_start (the tree has changes, else decompose
  wouldn't activate). So `contenderTree == snap` is always false on decompose → Busy. This is the
  designed defense-in-depth behavior; the doc must describe it as such.
- **Pins the behavior for downstream.** A later e2e scenario (P1.M1.T2.S1) asserts the documented exit
  code, so the README wording chosen here is the contract that test pins.

## What

A single-paragraph rewrite of the `**Safe to run twice.**` paragraph at `README.md:330`. The exit-0
promise becomes path-scoped (single-commit only); a new clause documents the decompose path exits Busy.
Everything else in the FAQ answer (line 328's "No. Stagecoach uses `git write-tree`…" and the per-host/CAS
parenthetical) is preserved.

### Success Criteria

- [ ] README.md:330's "Safe to run twice" paragraph scopes exit 0 to the **single-commit (staged) path**.
- [ ] The paragraph states the **decompose path** (nothing staged, dirty working tree) exits `5` (Busy)
      on an accidental double-run, and why (working-tree snapshot a lock-free contender cannot reproduce).
- [ ] The single-commit path still documents BOTH outcomes: exit `0` (nothing to do) AND exit `5` (Busy
      if genuinely new work is staged).
- [ ] The per-host/shared-filesystem CAS caveat sentence is preserved (unchanged or lightly carried over).
- [ ] Line 328 ("No. Stagecoach uses `git write-tree`…") is UNCHANGED.
- [ ] The FAQ heading line 326 ("### Will it corrupt my repo?") is UNCHANGED.
- [ ] NO code files, NO test files, NO other doc files are touched (docs/cli.md:379 = S2;
      docs/how-it-works.md:155 = S3; PRD.md / tasks.json / prd_snapshot.md / plan/* untouched).
- [ ] `go test ./...` still passes (it's a doc-only change, but confirms no accidental file clobber).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the EXACT current paragraph (verbatim, README.md:330), the
EXACT sentences to preserve (lines 326 + 328 + the CAS caveat), and a complete target paragraph adapted
to the README's voice. The issue root cause (working-tree vs index snapshot axis mismatch) is stated so
the implementer understands the *why* behind the wording, not just the *what*.

### Documentation & References

```yaml
# MUST READ — the issue + root cause
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/issue_analysis.md
  why: "Issue 1 documents the doc-vs-behavior mismatch: the no-op fast path (exit 0) never fires on the decompose path because the holder publishes T_start (working-tree) while the contender computes baseTree (index) via write-tree. Recommends Option 1 (qualify the docs) as the lowest-risk fix. Gives the empirical repro (snapshot=c0f5cf74… vs write-tree=d7a57d28…==HEAD^{tree})."
  critical: "Confirms the behavior (decompose → Busy) is CORRECT and safe — only the doc over-promises. This subtask is the doc fix (Option 1); do NOT attempt Option 2 (holder/ contender snapshot-axis change) — that is out of scope and riskier."

- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/system_context.md
  why: "The 'snapshot publish axis mismatch' note explains structurally why the contender cannot reproduce T_start from a lock-free write-tree (index empty at decompose activation → write-tree returns baseTree). Underpins the wording's 'why'."

# The file under edit
- file: README.md
  why: "EDIT line 330 ONLY (the '**Safe to run twice.**' paragraph). Lines 326 (heading) and 328 ('No. Stagecoach uses git write-tree…') are UNCHANGED. The per-host/CAS parenthetical at the end of line 330 is PRESERVED."
  pattern: "The current paragraph is one long line (markdown) with bold lead-in '**Safe to run twice.**', two exit-code cases (0 / 5), and a trailing '(On a shared filesystem…)' caveat. The rewrite keeps that shape but splits the exit-0 case to the single-commit path and adds the decompose-path Busy clause."
  gotcha: "Do NOT change line 328. Do NOT edit the heading (326). Keep the CAS caveat sentence. Do NOT touch docs/cli.md:379 or docs/how-it-works.md:155 — those are sibling subtasks (S2/S3)."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── README.md          # EDIT line 330 (the 'Safe to run twice' paragraph)
```

### Desired Codebase Tree After S1

```bash
stagecoach/
└── README.md          # line 330 rewritten (path-scoped exit-0 + decompose→Busy); rest unchanged
```

| Path | Action | Responsibility |
|---|---|---|
| `README.md` | MODIFY (line 330 only) | Scope the exit-0 claim to the single-commit path; document decompose→Busy; preserve the CAS caveat. |

**Explicitly NOT touched**: `docs/cli.md` (line 379 = P1.M1.T1.S2), `docs/how-it-works.md` (line 155 =
P1.M1.T1.S3), any `.go` file (no code change), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```markdown
<!-- CRITICAL: edit ONLY the "**Safe to run twice.**" paragraph (README.md:330). Line 328 ("No. Stagecoach
     uses `git write-tree` + `git commit-tree` + `git update-ref`…") is the FAQ's lead answer and MUST
     stay verbatim — it is correct and unrelated. The heading (326) stays. -->

<!-- CRITICAL: preserve the per-host/shared-filesystem CAS caveat (the trailing "(On a shared filesystem
     across hosts the lock can't help — the atomic `update-ref` CAS is the never-clobber-HEAD guarantee
     there.)"). It is true for BOTH paths and belongs in the scoped paragraph. -->

<!-- GOTTA: keep BOTH single-commit outcomes (exit 0 nothing-to-do AND exit 5 Busy-if-new-work-staged).
     Do NOT drop the Busy case for the single-commit path — only the unconditional "exits 0" framing is
     wrong; the single-commit path still exits Busy when genuinely new work is staged. -->

<!-- GOTTA: the README uses single-line markdown paragraphs (no hard wraps within a paragraph). Keep the
     rewritten paragraph as ONE logical line (matching line 330's style), not multi-line. -->

<!-- GOTTA: this is doc-only. `go test ./...` is a sanity check that nothing was accidentally clobbered —
     it does NOT validate prose. The prose check is: accurate, coherent, scoped. -->
```

## Implementation Blueprint

### Data models and structure

N/A — documentation-only. No types, no code.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: REWRITE the '**Safe to run twice.**' paragraph (README.md:330)
  - LOCATE: README.md, the line beginning "**Safe to run twice.**" (line 330), under the
    "### Will it corrupt my repo?" heading (line 326).
  - PRESERVE VERBATIM: line 328 ("No. Stagecoach uses `git write-tree`…byte-for-byte unchanged…") and the
    heading (line 326). Do not touch them.
  - REPLACE the current paragraph (the single long line) with a path-scoped version. Adapt the contract's
    suggested phrasing to the README's voice; keep the bold lead-in, the two single-commit outcomes, add
    the decompose-path clause, and carry over the per-host/CAS caveat. Target (one logical markdown line):
        **Safe to run twice.** A per-repo run lock prevents two concurrent commit-producing runs from
        racing on HEAD. On the **single-commit path** (changes staged), an accidental double-invoke exits
        `0` if nothing new has been staged since the in-progress run began (*nothing to do — an in-progress
        run already covers your staged changes*), or exits `5` (Busy) if genuinely new work is staged
        (your changes stay staged to re-run). On the **decompose path** (nothing staged, dirty working
        tree), an accidental double-run exits `5` (Busy) rather than `0` — the in-progress run publishes a
        working-tree snapshot a contender can't reproduce without the lock, so it conservatively refuses
        and leaves your working tree untouched. (On a shared filesystem across hosts the lock can't help —
        the atomic `update-ref` CAS is the never-clobber-HEAD guarantee there.)
  - VOICE: match the README's existing tone (direct, "your changes", parenthetical asides). Keep the
    italic "nothing to do…" string consistent with docs/cli.md / how-it-works.md (those are S2/S3 to
    align; minor wording drift here is fine — S3 sweeps coherence).
  - NAMING/TERMS: "single-commit path" / "decompose path"; exit codes `0` and `5` (Busy) in backticks.
  - DO NOT: change line 328, the heading, or any other FAQ entry. DO NOT touch code/test/other docs.

Task 2: VERIFY (doc-only validation)
  - RUN: go test ./...          # sanity — no file accidentally clobbered (doc change shouldn't affect it)
  - RUN (if markdownlint is configured): npx markdownlint README.md  OR  markdownlint README.md
    (project has .markdownlint.json). Expected: no new lint errors (keep the paragraph one logical line).
  - READ CHECK: the FAQ reads coherently — "Will it corrupt my repo?" → "No. …write-tree…" → "Safe to run
    twice." (now path-scoped) → [next FAQ]. The exit-0 promise is scoped to the single-commit path; the
    decompose path is documented as Busy.
  - git diff --stat → ONLY README.md changed (1 file, +1/-1-ish line).
```

### Implementation Patterns & Key Details

```markdown
<!-- === CURRENT README.md:330 (the line to replace) === -->
**Safe to run twice.** A per-repo run lock prevents two concurrent commit-producing runs from racing on HEAD, so an accidental double-invoke degrades gracefully: if nothing new is staged it exits `0` (*nothing to do — an in-progress run already covers your staged changes*); if genuinely new work is staged it exits `5` (Busy) and leaves your changes staged to re-run. (On a shared filesystem across hosts the lock can't help — the atomic `update-ref` CAS is the never-clobber-HEAD guarantee there.)

<!-- === TARGET README.md:330 (path-scoped; one logical markdown line; CAS caveat preserved) === -->
**Safe to run twice.** A per-repo run lock prevents two concurrent commit-producing runs from racing on HEAD. On the **single-commit path** (changes staged), an accidental double-invoke exits `0` if nothing new has been staged since the in-progress run began (*nothing to do — an in-progress run already covers your staged changes*), or exits `5` (Busy) if genuinely new work is staged (your changes stay staged to re-run). On the **decompose path** (nothing staged, dirty working tree), an accidental double-run exits `5` (Busy) rather than `0` — the in-progress run publishes a working-tree snapshot a contender can't reproduce without the lock, so it conservatively refuses and leaves your working tree untouched. (On a shared filesystem across hosts the lock can't help — the atomic `update-ref` CAS is the never-clobber-HEAD guarantee there.)
```

```markdown
<!-- === UNCHANGED context (for reference — DO NOT edit) === -->
<!-- line 326:  ### Will it corrupt my repo? -->
<!-- line 328:  No. Stagecoach uses `git write-tree` + `git commit-tree` + `git update-ref` (atomic snapshot commits). A failed generation leaves the repo byte-for-byte unchanged — it never touches the live index during generation. -->
```

### Integration Points

```yaml
DOC (README.md): line 330 rewritten ONLY — scope exit-0 to single-commit path; add decompose→Busy; preserve CAS caveat

NO-TOUCH (explicitly):
  - README.md lines 326 (heading) + 328 (lead answer)   # correct + unrelated
  - docs/cli.md ~line 379                                 # P1.M1.T1.S2 (sibling)
  - docs/how-it-works.md ~line 155                        # P1.M1.T1.S3 (sibling)
  - any .go file / test file                              # doc-only change
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — sibling/later subtasks):
  - S2 (P1.M1.T1.S2): qualify docs/cli.md:379 contention-behavior prose to match this scoping.
  - S3 (P1.M1.T1.S3): qualify docs/how-it-works.md:155 "No-op fast path" subsection.
  - P1.M1.T2.S1: add e2e scenario F (decompose accidental double-run → Busy) that pins the documented exit.
  - P1.M3.T1 (Mode B): final README coherence sweep after all fixes.
```

## Validation Loop

### Level 1: Doc Lint & Sanity (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

# (if markdownlint available — project has .markdownlint.json)
markdownlint README.md 2>/dev/null || npx -y markdownlint-cli2 README.md 2>/dev/null || echo "markdownlint not installed; skip (visual check only)"
# Expected: no NEW errors (keep the paragraph one logical markdown line; backticks balanced; italics balanced).

# Sanity: only README.md changed
git diff --stat
# Expected: README.md only (1 file).
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

# Read the FAQ paragraph and verify each claim against the implemented behavior:
sed -n '326,332p' README.md

# CHECK each of these is TRUE in the rendered paragraph:
#   1. exit 0 is scoped to the SINGLE-COMMIT path (not unconditional).
#   2. The single-commit path still shows BOTH exit 0 (nothing-to-do) AND exit 5 (Busy-if-new-work).
#   3. The DECOMPOSE path is documented as exit 5 (Busy), with the "working-tree snapshot a contender
#      can't reproduce" rationale.
#   4. The per-host/shared-filesystem CAS caveat is present.
#   5. Lines 326 (heading) and 328 ("No. Stagecoach uses git write-tree…") are UNCHANGED.
```

### Level 4: Cross-Doc Coherence (light — full sweep is S3)

```bash
cd /home/dustin/projects/stagecoach

# Confirm the exit codes/wording are consistent in spirit with the sibling docs (S2/S3 will fully align;
# here just flag gross contradictions). The "nothing to do — an in-progress run already covers your
# staged changes" string should be recognizably the same notion across README/cli.md/how-it-works.md.
grep -n "nothing to do\|Safe to run twice\|exits .0.\|Busy" README.md docs/cli.md docs/how-it-works.md | head
# (No assertion required in S1 — just ensure the README no longer makes the unconditional claim.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` + `go test ./...` green (doc-only sanity check).
- [ ] `git diff --stat` shows ONLY `README.md` changed.

### Feature Validation

- [ ] README.md:330 scopes exit 0 to the **single-commit (staged) path**.
- [ ] The single-commit path documents BOTH exit 0 (nothing to do) AND exit 5 (Busy if new work staged).
- [ ] The **decompose path** is documented as exit 5 (Busy) with the working-tree-snapshot rationale.
- [ ] The per-host/CAS caveat sentence is preserved.
- [ ] Lines 326 (heading) and 328 ("No. Stagecoach uses `git write-tree`…") are UNCHANGED.

### Scope Discipline Validation

- [ ] ONLY `README.md` modified (git diff --stat confirms).
- [ ] Did NOT touch `docs/cli.md` (S2), `docs/how-it-works.md` (S3), or any `.go`/test file.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Paragraph is one logical markdown line (matches README's paragraph style).
- [ ] Backticks (`` `0` ``, `` `5` ``, `` `update-ref` ``) and italics (`*nothing to do…*`) are balanced.
- [ ] Voice matches the surrounding FAQ (direct, "your changes", parenthetical asides).

---

## Anti-Patterns to Avoid

- ❌ Don't promise exit 0 unconditionally — that's the exact false claim being fixed. Scope it to the
  single-commit (staged) path.
- ❌ Don't drop the single-commit path's Busy case. Only the *unconditional* "exits 0" framing is wrong;
  the single-commit path still exits Busy when genuinely new work is staged — keep both outcomes.
- ❌ Don't change line 328 ("No. Stagecoach uses `git write-tree`…") or the heading (326). They are correct
  and unrelated to the lock contention behavior.
- ❌ Don't drop the per-host/shared-filesystem CAS caveat — it's true for both paths and belongs in the
  scoped paragraph.
- ❌ Don't attempt Option 2 (the holder/contender snapshot-axis code change). The issue_analysis.md
  recommends Option 1 (this doc fix) as lowest-risk; the behavior is correct, only the doc over-promises.
  Code changes are out of scope for this subtask.
- ❌ Don't edit `docs/cli.md` or `docs/how-it-works.md` — those are sibling subtasks (S2/S3) and will be
  aligned to the same scoping; editing them here crosses the subtask boundary.
- ❌ Don't hard-wrap the paragraph into multiple lines — the README uses single-line markdown paragraphs;
  keep the rewrite as one logical line.
- ❌ Don't introduce a claim that isn't backed by the behavior (e.g. "decompose exits 0 if…"). Every
  exit-code claim must match what the binary does (single-commit: 0 or 5; decompose: 5).

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a single-paragraph, doc-only edit with the exact current text quoted verbatim, the
exact lines to preserve (326, 328, CAS caveat) called out, and a complete target paragraph (adapted to
the README's voice) provided. The root cause (working-tree T_start vs index baseTree snapshot-axis
mismatch) is documented so the wording's "why" is grounded, not arbitrary. The issue_analysis.md
explicitly recommends this doc-qualification (Option 1) over the riskier code change. The only residual
uncertainty (not 10/10) is minor voice/wording taste that S3's Mode-B coherence sweep will finalize —
the PRP's target paragraph is a faithful, ready-to-paste starting point. No code, no tests, no other
docs are in scope, so the blast radius is one line of one file.
