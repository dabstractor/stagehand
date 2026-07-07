---
name: "P1.M1.T1.S3 — Qualify the docs/how-it-works.md 'No-op fast path' subsection (line 155)"
description: |
  Documentation-only fix (Issue 1, Major — doc-vs-behavior mismatch, third of three sibling doc fixes).
  `docs/how-it-works.md:155` (the `**No-op fast path.**` paragraph under `### Per-repo run lock (FR52)`
  at line 144) states unconditionally: "A contender with nothing new staged since that snapshot can exit
  0 immediately (no-op fast path)." This is **false on the decompose path**: the holder publishes a
  working-tree snapshot (`T_start`) that a lock-free contender cannot reproduce from an index
  `write-tree` (it returns `baseTree` == `HEAD^{tree}`), so the contender always exits `5` (Busy),
  never 0. Rewrite the paragraph to scope exit 0 to the **single-commit (staged) path** (precision:
  "index-tree SHA") and note that on the **decompose path** an accidental double-run exits `5` (Busy)
  — the holder publishes a working-tree snapshot (`T_start`) a lock-free contender cannot reproduce
  from the index, so it conservatively refuses. Keep it concise (one logical markdown line) to match
  the subsection's bold-lead-in style (siblings: Per-host limit L151, Never-in-repo location L153,
  Auto-release L157). Do NOT edit the "Failure modes and exit codes" table (lines 163-173) — it does
  not list code 5/Busy at all, which is a **pre-existing gap unrelated to this bug**; do NOT expand
  scope to fix it. NO code changes. README.md:330 (S1) and docs/cli.md:379 (S2) are sibling subtasks.
---

## Goal

**Feature Goal**: Make the `docs/how-it-works.md` **No-op fast path** subsection *honest* by scoping
its exit-0 (no-op fast path) promise to the path where it actually holds — the **single-commit
(staged)** path — and explicitly documenting that on the **decompose path** an accidental double-run
exits `5` (Busy) because the holder publishes a working-tree snapshot (`T_start`) a lock-free contender
cannot reproduce from the index alone.

**Deliverable**: The rewritten `**No-op fast path.**` paragraph at `docs/how-it-works.md:155`, scoped
per-path and kept concise (one logical markdown line, `**No-op fast path.**` lead-in preserved, matching
the subsection's bold-lead-in sibling paragraphs). No other line of `docs/how-it-works.md` changes — the
`### Per-repo run lock (FR52)` heading (line 144), the other bold-lead-in subsections (Per-host limit
L151, Never-in-repo location L153, Auto-release L157), the **"Failure modes and exit codes" table
(lines 163-173)**, and the rescue/pointer notes (L175, L177) are all UNCHANGED. No code changes anywhere.

**Success Definition**: A reader of the `### Per-repo run lock (FR52)` section knows the single-commit
path can exit `0` (a byte-identical staged snapshot), and the decompose path exits `5` (Busy) on an
accidental double-run — and why (the holder publishes a working-tree `T_start` a lock-free contender
cannot reproduce from the index). The exit-code table (163-173) and the other subsections are untouched;
markdown is valid; no code/test/other-doc files are touched; `go test ./...` is unaffected.

## User Persona

**Target User**: The Stagecoach user (or contributor) reading `docs/how-it-works.md`'s
"### Per-repo run lock (FR52)" architecture section to understand the no-op fast path — especially
someone binding `stagecoach` to a keybind (lazygit/double-tap) who needs to know what an accidental
double-run does on each commit path, and a reviewer auditing the architecture docs for accuracy against
the implemented FR52 behavior.

**Use Case**: A user double-invokes `stagecoach` on a dirty, un-staged working tree (the decompose
trigger) and consults `docs/how-it-works.md` to learn the exit code. The current prose implies exit 0 is
reachable on that path; the fixed prose tells them it exits Busy (5) and why.

**Pain Points Addressed**: Removes a false claim. The current text sets an expectation (decompose
double-run can reach exit 0) the binary provably does not meet (it exits 5), which erodes trust in the
rest of the run-lock architecture documentation once a user observes the mismatch.

## Why

- **Honesty in the architecture documentation.** The "No-op fast path" subsection is the how-it-works
  narrative of the FR52 contention no-op optimization. An unconditional "can exit 0 immediately" clause
  that is false on an entire commit path (decompose) undermines the architecture section the moment a
  user double-taps a keybind on a dirty tree.
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
- **Completes the three-surface doc fix.** S1 (README:330), S2 (cli.md:379), and S3 (this —
  how-it-works.md:155) carry one consistent per-path story. S3 is the how-it-works architecture-narrative
  piece.

## What

A single-paragraph rewrite of the `**No-op fast path.**` paragraph at `docs/how-it-works.md:155`. The
unconditional "can exit 0 immediately" becomes path-scoped (single-commit only); a new clause documents
that the decompose path exits Busy. The `**No-op fast path.**` bold lead-in is preserved (subsection
identity). Everything else in the `### Per-repo run lock (FR52)` section — the other bold-lead-in
subsections and the **exit-code table** — is unchanged.

### Success Criteria

- [ ] `docs/how-it-works.md:155` scopes the exit-0 (no-op fast path) to the **single-commit (staged) path**.
- [ ] The paragraph states the **decompose path** (nothing staged, dirty working tree) exits `5` (Busy)
      on an accidental double-run, and why (the holder publishes a working-tree snapshot `T_start` a
      lock-free contender cannot reproduce from the index alone, so it conservatively refuses).
- [ ] The `**No-op fast path.**` bold lead-in is preserved.
- [ ] The paragraph is **concise** — one logical markdown line, matching the subsection's bold-lead-in
      sibling paragraphs (Per-host limit, Never-in-repo location, Auto-release). No hard-wrapping.
- [ ] The **"Failure modes and exit codes" table (lines 163-173)** is UNCHANGED. (It does not list code
      5/Busy at all — this is a pre-existing gap unrelated to this bug; do NOT add a row.)
- [ ] The other bold-lead-in subsections (Per-host limit L151, Never-in-repo location L153, Auto-release
      L157), the `### Per-repo run lock (FR52)` heading (L144), and the rescue/pointer notes (L175, L177)
      are UNCHANGED.
- [ ] NO code files, NO test files, NO other doc files are touched (README.md = S1; docs/cli.md:379 = S2;
      PRD.md / tasks.json / prd_snapshot.md / plan/* untouched).
- [ ] `go test ./...` still passes (doc-only sanity check).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the EXACT current paragraph (verbatim, `docs/how-it-works.md:155`),
the EXACT target paragraph (ready to paste, supplied by the item contract), the surrounding subsection
style (bold-lead-in single-line paragraphs — siblings quoted), the markdownlint config (MD013 disabled),
and the exact lines that must NOT change (the exit-code table 163-173 + the other subsections + the
heading). The issue root cause (working-tree `T_start` vs index `baseTree` snapshot-axis mismatch) is
stated so the implementer understands the *why* behind the wording, not just the *what*.

### Documentation & References

```yaml
# MUST READ — the issue + root cause
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/issue_analysis.md
  why: "Issue 1 documents the doc-vs-behavior mismatch: the no-op fast path (exit 0) never fires on the decompose path because the holder publishes T_start (working-tree, decompose.go:169) while the contender computes baseTree (index) via write-tree (index empty at decompose activation → write-tree returns HEAD^{tree}). Recommends Option 1 (qualify the docs) as the lowest-risk fix. Lists docs/how-it-works.md:155 as one of the three doc surfaces to qualify."
  critical: "Confirms the behavior (decompose → Busy) is CORRECT and safe — only the doc over-promises. This subtask is the doc fix (Option 1); do NOT attempt Option 2 (holder/contender snapshot-axis code change) — out of scope and riskier. NO code change."

- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/system_context.md
  why: "Line 30 lists docs/how-it-works.md:155 ('No-op fast path' subsection) as scope-1 surface. The 'Snapshot publish axis mismatch' table (the root of Issue 1) gives the authoritative per-path table: single-commit = index tree both sides (can match); decompose = holder T_start (working-tree) vs contender baseTree = HEAD^{tree} (never matches → Busy). Underpins the wording's 'why'."
  critical: "The snapshot-axis table is the precise justification for the rewritten paragraph's 'index-tree SHA' vs 'working-tree snapshot (T_start)' distinction — use that exact framing."

- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M1T1S1/PRP.md
  why: "The README.md sibling (S1). Carries the SAME per-path semantics (single-commit → 0-or-5; decompose → 5) in the README's voice. S3 mirrors it for how-it-works.md's architecture-narrative voice. S1 explicitly defers docs/how-it-works.md to S3 — different files, no conflict. Use S1's per-path semantics as the coherence reference."

- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M1T1S2/PRP.md
  why: "The docs/cli.md sibling (S2). Same per-path semantics in the cli.md reference voice. Confirms the three-surface alignment story and the 'nothing to do…' string lives in README/cli.md (NOT in how-it-works.md:155, so S3 need not reproduce it)."

# The file under edit
- file: docs/how-it-works.md
  why: "EDIT line 155 ONLY (the '**No-op fast path.**' paragraph). The surrounding bold-lead-in subsections (Per-host limit L151, Never-in-repo location L153, Auto-release L157), the heading (L144), and the 'Failure modes and exit codes' table (163-173) are UNCHANGED."
  pattern: "The paragraph is ONE logical markdown line with a '**<lead-in>.**' bold prefix (matching the sibling subsections Per-host limit / Never-in-repo location / Auto-release). The rewrite keeps that shape: '**No-op fast path.**' lead-in, then path-scoped prose (single-commit → exit 0 on byte-identical staged snapshot; decompose → exit 5 Busy)."
  gotcha: "Do NOT edit the 'Failure modes and exit codes' table (163-173) — it does NOT list code 5/Busy, which is a pre-existing gap unrelated to this bug (do NOT add a Busy row). Do NOT edit the other bold-lead-in subsections. Keep the paragraph ONE logical markdown line (.markdownlint.json disables MD013, so long lines are expected)."

# markdownlint config (confirms long single-line paragraphs are allowed)
- file: .markdownlint.json
  why: "Config: { \"default\": true, \"MD013\": false, \"MD033\": false, \"MD060\": false }. MD013 (line-length) is DISABLED → the existing bold-lead-in subsections (L151/L153/L155/L157) are each ONE long markdown line, and the rewrite must match (no hard-wrapping)."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── docs/how-it-works.md    # EDIT line 155 (the '**No-op fast path.**' paragraph)
└── .markdownlint.json      # MD013 disabled → single-line paragraphs allowed
```

### Desired Codebase Tree After S3

```bash
stagecoach/
└── docs/how-it-works.md    # line 155 rewritten (path-scoped exit-0 + decompose→Busy); rest unchanged
```

| Path | Action | Responsibility |
|---|---|---|
| `docs/how-it-works.md` | MODIFY (line 155 only) | Scope the exit-0 no-op fast path to the single-commit path; document decompose→Busy with the working-tree-`T_start` rationale; keep the `**No-op fast path.**` lead-in; stay one concise logical markdown line. |

**Explicitly NOT touched**: `docs/how-it-works.md` heading L144 + other bold-lead-in subsections
(L151 Per-host limit, L153 Never-in-repo location, L157 Auto-release) + **exit-code table (L163-173)**
+ rescue/pointer notes (L175, L177), `README.md` (S1), `docs/cli.md` (S2), any `.go`/test file
(no code change), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```markdown
<!-- CRITICAL: edit ONLY the '**No-op fast path.**' paragraph (docs/how-it-works.md:155). The 'Failure
     modes and exit codes' table (lines 163-173) does NOT list code 5/Busy at all — the item contract
     explicitly says this is a PRE-EXISTING GAP unrelated to this bug; do NOT add a Busy row, do NOT
     expand scope. Leave the table untouched. -->

<!-- CRITICAL: preserve the '**No-op fast path.**' bold lead-in (subsection identity). The sibling
     subsections all follow the '**<lead-in>.** <prose>' shape (Per-host limit, Never-in-repo location,
     Auto-release). Keep the same shape. -->

<!-- CRITICAL: the rewrite must be CONCISE (one logical markdown paragraph) to match the subsection's
     style. The contract's suggested phrasing is 3 sentences ≈ the density of Auto-release (L157). Do
     not balloon it. -->

<!-- GOTCHA: keep the paragraph ONE logical markdown line. .markdownlint.json DISABLES MD013
     (line-length) and the existing L151/L153/L155/L157 are each one long line. Do NOT hard-wrap the
     rewrite into multiple physical lines. -->

<!-- GOTCHA: the snapshot-axis distinction is the heart of the fix. Use the precise framing from the
     system_context.md table: on the single-commit path the holder publishes an INDEX-tree SHA
     (same axis as the contender's write-tree → can match → exit 0); on the decompose path the holder
     publishes a WORKING-TREE snapshot (T_start) the contender cannot reproduce from the index alone
     (it gets baseTree = HEAD^{tree}) → never matches → Busy(5). -->

<!-- GOTCHA: this is doc-only. `go test ./...` is a sanity check that nothing was accidentally clobbered
     — it does NOT validate prose. The prose check is: accurate, concise, path-scoped, lead-in kept,
     table + other subsections untouched. -->

<!-- GOTCHA: do NOT touch README.md (S1) or docs/cli.md (S2) — those carry the same per-path semantics
     in their own voices; S3 is docs/how-it-works.md ONLY. -->
```

## Implementation Blueprint

### Data models and structure

N/A — documentation-only. No types, no code.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: REWRITE the '**No-op fast path.**' paragraph (docs/how-it-works.md:155)
  - LOCATE: docs/how-it-works.md, the line beginning "**No-op fast path.**" (line 155), under the
    "### Per-repo run lock (FR52)" heading (line 144), between the "**Never-in-repo location.**"
    subsection (L153) and the "**Auto-release.**" subsection (L157).
  - PRESERVE VERBATIM: the heading (L144), the other bold-lead-in subsections (Per-host limit L151,
    Never-in-repo location L153, Auto-release L157), the "Failure modes and exit codes" table
    (L163-173), and the rescue/pointer notes (L175, L177). Do not touch them.
  - REPLACE the current paragraph (the single long line at 155) with the path-scoped version. Target
    (ONE logical markdown line — paste verbatim from the item contract's suggested phrasing):
        **No-op fast path.** On the single-commit path (changes staged), the holder publishes its frozen index-tree SHA via `SetSnapshot()`, and a contender whose staged snapshot is byte-identical to it exits 0 immediately. On the decompose path (nothing staged, dirty working tree), an accidental double-run exits **5 (Busy)** instead — the holder publishes a working-tree snapshot (`T_start`) that a lock-free contender cannot reproduce from the index, so it conservatively refuses.
  - PRESERVED in that target: the `**No-op fast path.**` bold lead-in, the "exit 0 immediately" notion
    (now scoped to the single-commit path), and the "SetSnapshot()" reference.
  - ADDED: path scoping ("On the single-commit path" / "On the decompose path"), the decompose Busy(5)
    clause with the working-tree `T_start` rationale, and "index-tree SHA" precision on the
    single-commit side (the snapshot-axis distinction that is the heart of the fix).
  - VOICE: architecture-narrative tone (matches docs/how-it-works.md's existing run-lock subsections —
    direct, technical, third-person, backticked identifiers: `SetSnapshot()`, `T_start`). Terms:
    "single-commit path" / "decompose path"; exit code `0` and `5 (Busy)`.
  - DO NOT: change the exit-code table, the other subsections, the heading, the rescue/pointer notes,
    or any other doc/code/test file.

Task 2: VERIFY (doc-only validation)
  - RUN: go build ./... && go test ./...     # sanity — no file accidentally clobbered (doc change)
  - RUN (if markdownlint available): markdownlint docs/how-it-works.md  OR
    npx -y markdownlint-cli2 docs/how-it-works.md
    (.markdownlint.json is configured; MD013 disabled). Expected: no NEW errors (keep the paragraph
    ONE logical line; backticks/bold balanced).
  - READ CHECK (sed -n '144,177p' docs/how-it-works.md): the "### Per-repo run lock (FR52)" section
    reads coherently — Per-host limit → Never-in-repo location → **No-op fast path** (now path-scoped:
    single-commit → exit 0 on byte-identical staged snapshot; decompose → Busy 5) → Auto-release.
    The exit-0 promise is scoped to the single-commit path; the decompose path is documented as Busy;
    the exit-code table is unchanged.
  - git diff --stat → ONLY docs/how-it-works.md changed (1 file, +1/-1-ish line).
```

### Implementation Patterns & Key Details

```markdown
<!-- === CURRENT docs/how-it-works.md:155 (the line to replace) === -->
**No-op fast path.** When a lock is held, the holder publishes its frozen tree SHA via `SetSnapshot()`. A contender with nothing new staged since that snapshot can exit 0 immediately (no-op fast path).

<!-- === TARGET docs/how-it-works.md:155 (path-scoped; ONE logical markdown line; lead-in preserved;
     supplied verbatim by the item contract's suggested phrasing) === -->
**No-op fast path.** On the single-commit path (changes staged), the holder publishes its frozen index-tree SHA via `SetSnapshot()`, and a contender whose staged snapshot is byte-identical to it exits 0 immediately. On the decompose path (nothing staged, dirty working tree), an accidental double-run exits **5 (Busy)** instead — the holder publishes a working-tree snapshot (`T_start`) that a lock-free contender cannot reproduce from the index, so it conservatively refuses.
```

```markdown
<!-- === UNCHANGED context (for reference — DO NOT edit) === -->
<!-- line 144:  ### Per-repo run lock (FR52) -->
<!-- line 151:  **Per-host limit.** … -->
<!-- line 153:  **Never-in-repo location.** … -->
<!-- line 157:  **Auto-release.** … -->
<!-- lines 163-173: the "Failure modes and exit codes" TABLE (no code 5/Busy row — pre-existing gap,
     explicitly OUT OF SCOPE; do NOT add a Busy row) -->
<!-- line 175: The rescue (3) and timeout (124) rows … note -->
<!-- line 177: See [cli.md](cli.md#exit-codes) for the full exit-code table. -->
```

### Integration Points

```yaml
DOC (docs/how-it-works.md): line 155 rewritten ONLY — scope exit-0 to single-commit path; add
  decompose→Busy with the working-tree-T_start rationale; preserve the '**No-op fast path.**' lead-in;
  stay one concise logical markdown line.

NO-TOUCH (explicitly):
  - docs/how-it-works.md heading L144 + bold-lead-in siblings (Per-host limit L151,
    Never-in-repo location L153, Auto-release L157)   # correct + unrelated
  - docs/how-it-works.md "Failure modes and exit codes" table (L163-173)              # pre-existing
    gap (no Busy row) — OUT OF SCOPE, do NOT add a row
  - docs/how-it-works.md rescue/pointer notes (L175, L177)                            # unrelated
  - README.md                                # P1.M1.T1.S1 (sibling — same semantics, README voice)
  - docs/cli.md                              # P1.M1.T1.S2 (sibling — contention-behavior prose)
  - any .go file / test file                 # doc-only change
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — sibling/later subtasks):
  - S1 (P1.M1.T1.S1): scopes README.md:330 "Safe to run twice" (same per-path semantics).
  - S2 (P1.M1.T1.S2): qualifies docs/cli.md:379 contention-behavior prose.
  - P1.M1.T2.S1: adds e2e scenario F (decompose accidental double-run → Busy) that pins the documented
    exit code.
  - P1.M3.T1 (Mode B): final how-it-works.md + cli.md + README coherence sweep.
```

## Validation Loop

### Level 1: Doc Lint & Sanity (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

# (markdownlint is configured — .markdownlint.json at root; MD013 line-length disabled)
markdownlint docs/how-it-works.md 2>/dev/null || npx -y markdownlint-cli2 docs/how-it-works.md 2>/dev/null || echo "markdownlint not runnable; visual check only"
# Expected: no NEW errors (keep the paragraph ONE logical line; backticks/bold balanced).

# Sanity: only docs/how-it-works.md changed
git diff --stat
# Expected: docs/how-it-works.md only (1 file).
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

# Render the "### Per-repo run lock (FR52)" section and verify each claim against the implemented behavior:
sed -n '144,177p' docs/how-it-works.md

# CHECK each of these is TRUE in the rewritten paragraph (line 155):
#   1. exit 0 is scoped to the SINGLE-COMMIT path (not unconditional).
#   2. The single-commit path's snapshot is described as an "index-tree SHA" (the snapshot-axis that
#      lets the contender's write-tree match it).
#   3. The DECOMPOSE path is documented as exit 5 (Busy), with the "working-tree snapshot (T_start) a
#      lock-free contender cannot reproduce from the index" rationale.
#   4. The '**No-op fast path.**' bold lead-in is preserved.
#   5. The paragraph is ONE concise logical markdown line (matches the sibling subsections).
#   6. The "Failure modes and exit codes" TABLE (163-173) is UNCHANGED (still no Busy row — out of scope).
#   7. The other bold-lead-in subsections (Per-host limit, Never-in-repo location, Auto-release) and
#      the heading (L144) are UNCHANGED.
```

### Level 4: Cross-Doc Coherence (light — full sweep is P1.M3.T1)

```bash
cd /home/dustin/projects/stagecoach

# Confirm the per-path semantics are consistent in spirit with the sibling docs (S1/S2 fully align;
# here just flag gross contradictions). All three surfaces should scope exit-0 to the single-commit
# path and document decompose→Busy.
grep -n "No-op fast path\|single-commit\|decompose path\|exits .0.\|Busy\|T_start" docs/how-it-works.md docs/cli.md README.md | head
# (No assertion required in S3 — just ensure docs/how-it-works.md no longer makes the unconditional claim.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` + `go test ./...` green (doc-only sanity check).
- [ ] `git diff --stat` shows ONLY `docs/how-it-works.md` changed.

### Feature Validation

- [ ] `docs/how-it-works.md:155` scopes exit 0 to the **single-commit (staged) path**.
- [ ] The single-commit path's snapshot is described as an **index-tree SHA** (the matchable axis).
- [ ] The **decompose path** is documented as exit `5` (Busy) with the working-tree-`T_start` rationale.
- [ ] The `**No-op fast path.**` bold lead-in is preserved.
- [ ] The paragraph is concise — ONE logical markdown line matching the subsection's bold-lead-in style.
- [ ] The "Failure modes and exit codes" table (163-173) is UNCHANGED (no Busy row added — out of scope).
- [ ] The other bold-lead-in subsections (Per-host limit, Never-in-repo location, Auto-release) and the
      heading (L144) are UNCHANGED.

### Scope Discipline Validation

- [ ] ONLY `docs/how-it-works.md` modified (git diff --stat confirms).
- [ ] Did NOT edit the "Failure modes and exit codes" table (163-173) or add a Busy row (pre-existing gap).
- [ ] Did NOT edit the other bold-lead-in subsections, the heading, or the rescue/pointer notes.
- [ ] Did NOT touch `README.md` (S1), `docs/cli.md` (S2), or any `.go`/test file.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Paragraph is ONE logical markdown line (matches docs/how-it-works.md's bold-lead-in style + markdownlint MD013-disabled config).
- [ ] Inline markup balanced: backticks (`` `SetSnapshot()` ``, `` `T_start` ``), bold (`**No-op fast path.**`, `**5 (Busy)**`).
- [ ] Voice matches the surrounding run-lock architecture prose (direct, technical, third-person, backticked identifiers).

---

## Anti-Patterns to Avoid

- ❌ Don't describe the exit-0 fast path unconditionally — that's the exact false claim being fixed. Scope
  it to the single-commit (staged) path.
- ❌ Don't edit the **"Failure modes and exit codes" table** (163-173). It does NOT list code 5/Busy, and
  the item contract explicitly says that is a **pre-existing gap unrelated to this bug** — do NOT add a
  Busy row. Only the `**No-op fast path.**` paragraph (155) over-promises; fix that and nothing else.
- ❌ Don't edit the other bold-lead-in subsections (Per-host limit L151, Never-in-repo location L153,
  Auto-release L157), the heading (L144), or the rescue/pointer notes (L175, L177) — they are correct and
  unrelated.
- ❌ Don't drop the `**No-op fast path.**` bold lead-in — it is the subsection's identity (the section is a
  stack of bold-lead-in paragraphs).
- ❌ Don't balloon the paragraph. The subsection style is concise (siblings are 2-3 sentences); the
  contract's suggested phrasing is ~3 sentences and matches Auto-release's density — use it as-is.
- ❌ Don't blur the snapshot-axis distinction. Use the precise framing: single-commit path = holder
  publishes an **index-tree** SHA (matchable); decompose path = holder publishes a **working-tree**
  snapshot `T_start` (the contender's index `write-tree` returns `baseTree` = `HEAD^{tree}`, never matches).
- ❌ Don't attempt Option 2 (the holder/contender snapshot-axis code change). `issue_analysis.md` recommends
  Option 1 (this doc fix) as lowest-risk; the behavior is correct, only the doc over-promises. Code changes
  are out of scope.
- ❌ Don't edit `README.md` (S1) or `docs/cli.md` (S2) — those are sibling subtasks carrying the same
  per-path semantics in their own voices; editing them here crosses the subtask boundary.
- ❌ Don't hard-wrap the paragraph into multiple physical lines — the docs/how-it-works.md subsections use
  single-line markdown paragraphs and `.markdownlint.json` disables MD013; keep the rewrite as ONE logical line.
- ❌ Don't introduce a claim that isn't backed by the behavior (e.g. "decompose exits 0 if…"). Every
  exit-code claim must match what the binary does (single-commit: 0; decompose: 5).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a single-paragraph, doc-only edit with the EXACT current text quoted verbatim
(`docs/how-it-works.md:155`), the EXACT target paragraph provided ready-to-paste (supplied verbatim by
the item contract's suggested phrasing — no authoring required), the `**No-op fast path.**` lead-in to
preserve called out, and the exact lines that must NOT change (the exit-code table 163-173 + the other
bold-lead-in subsections + the heading) named. The surrounding subsection style (bold-lead-in, single
logical line) is quoted from the verified siblings (Per-host limit L151, Never-in-repo location L153,
Auto-release L157), and the markdownlint config (MD013 disabled) confirms long single-line paragraphs
are expected. The root cause (working-tree `T_start` vs index `baseTree` snapshot-axis mismatch) is
documented so the wording's "why" is grounded — notably the "index-tree SHA" precision on the
single-commit side, which is the snapshot-axis distinction at the heart of the fix. `issue_analysis.md`
explicitly recommends this doc-qualification (Option 1) over the riskier code change, and the contract
supplies the authoritative target phrasing. S1 (README) and S2 (cli.md) are the structural templates and
confirm the per-path semantics. The only residual uncertainty (not 10/10) is minor voice/wording taste
that P1.M3.T1's Mode-B coherence sweep will finalize — the PRP's target paragraph is a faithful,
ready-to-paste starting point. No code, no tests, no other docs are in scope, so the blast radius is one
line of one file.
