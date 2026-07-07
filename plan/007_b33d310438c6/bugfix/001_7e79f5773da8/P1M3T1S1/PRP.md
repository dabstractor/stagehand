---
name: "P1.M3.T1.S1 — Sweep README.md, docs/how-it-works.md, docs/configuration.md for truncation-format & diff_context accuracy (FR3i sentinel-newline fix + diff_context hardening; Mode B changeset doc sync)"
description: |

  Changeset-level documentation sweep (Mode B catch-all) for bugfix `001_7e79f5773da8` (Issues 1 & 2 of the
  FR3d–FR3i validation). It depends on all implementing subtasks and runs last. Two accuracy concerns across
  three docs:

  (a) **Per-file truncation format / sentinel line-shape** — P1.M1.T1.S1 fixed INTERNAL output line-shape:
      each truncated file's section now ends with `... [truncated]\n` so the next file's `diff --git` begins
      at a line start (was: sentinel glued to the next header). No config/API surface change.
  (b) **diff_context valid range / out-of-range behavior** — P1.M2.T1.S1 (parallel, implementing) makes an
      out-of-range `diff_context` fail config load with a clear error (was: silently clamped to 1).

  ⚠️ **RESEARCH OUTCOME (pre-verified): NO inaccuracies and NO stale references — a NO-OP CONFIRMATION, with
  ONE optional clarification.** A full grep audit of README.md + docs/how-it-works.md + docs/configuration.md
  (every truncation/sentinel/diff_context/token_limit/water-fill mention) shows:
    - Concern (a): NO doc states truncation line-shape ⇒ nothing inaccurate; NO doc implies sentinels run
      together ⇒ the contract's "ensure" clause is already satisfied. The only action is the OPTIONAL
      how-it-works.md L144 newline-separation note.
    - Concern (b): docs/configuration.md is P1.M2.T1.S1's domain (it adds "valid range 0–3; out-of-range
      rejected at config load" to 3 spots + bootstrap). how-it-works.md L138 lists 0/1/3 (valid, not
      inaccurate); the authoritative range/rejection detail belongs in configuration.md (one source of
      truth). README has no diff_context claim.
    - Stale references: NO doc references "sentinel gluing" or "silent clamping" ⇒ nothing to remove.
  See research/docs-sweep-audit.md §2–§5 (the full doc×concern matrix) — the single most important read.

  ⚠️ **DO NOT duplicate P1.M2.T1.S1.** It owns the diff_context range/rejection Mode-A docs in
  docs/configuration.md (~107 comment, ~131 table, ~147 prose + bootstrap.go:291). This sweep must NOT
  restate the range/rejection there. Touch configuration.md ONLY if P1.M2.T1.S1 left an inconsistency
  (none found). It touches neither how-it-works.md nor README.md — those are this sweep's alone.

  ⚠️ **DO NOT invent features or over-edit.** Update ONLY statements that are inaccurate OR would materially
  improve the mental model. The shipped fix is internal line-shape (Issue 1) + a config validation (Issue 2
  — already documented by P1.M2.T1.S1). Most docs are already consistent ⇒ the successful outcome is mostly
  a no-op confirmation. Do NOT add a truncation-format spec where the doc is deliberately high-level.

  ⚠️ **THE ONE optional edit (how-it-works.md L144).** The water-fill bullet describes the `... [truncated]`
  marker but is silent on line-shape. A short clause — "(with a `... [truncated]` marker that ends the file's
  section on its own line, so the next file's `diff --git` begins fresh)" — surfaces the fixed behavior and
  improves the mental model. OPTIONAL (the doc is accurate without it); NOT a duplicate. Skipping is
  acceptable; record the choice either way.

  ⚠️ **House style.** `.markdownlint.json`: default true; MD013 (line length), MD033 (inline HTML), MD060
  disabled. Long lines are fine; inline code + plain prose (the recommended edit) is fully compliant.

  ⚠️ **Re-verify at implementation time (docs drift).** The pre-run audit is against the current docs; the
  implementer MUST re-run the §2 grep at implementation time. If still clean (expected), apply ONLY the
  optional L144 edit (or skip it) and record a dated no-op line. If a real inconsistency surfaced (e.g. a
  doc now claims sentinels run together, or P1.M2.T1.S1's configuration.md edit conflicts), fix ONLY that.

  Deliverable: either (a) NO edit + a dated no-op confirmation in research/docs-sweep-audit.md (the
  expected primary outcome), optionally plus (b) the one-line how-it-works.md L144 clarification. OUTPUT:
  changeset-level docs consistent with the shipped behavior; no stale sentinel-gluing / silent-clamp
  references. DOCS: this IS the documentation task (Mode B catch-all per SOW §5).

---

## Goal

**Feature Goal**: Verify README.md, docs/how-it-works.md, and docs/configuration.md are consistent with the
shipped behavior of the FR3i truncation newline fix (Issue 1) and the diff_context validation (Issue 2): no
statement about (a) per-file truncation/sentinel line-shape or (b) the diff_context range/out-of-range
behavior is inaccurate, and no stale reference to pre-fix sentinel-gluing or silent diff_context clamping
remains. Pre-research shows the docs are already consistent ⇒ the expected outcome is a no-op confirmation,
with one optional how-it-works.md clarification.

**Deliverable**:
1. **RE-RUN** the audit grep (§2) against the live README.md + docs/how-it-works.md + docs/configuration.md.
2. **IF clean (expected)**: record a dated "re-verified coherent — no edit" line in
   `research/docs-sweep-audit.md`. Optionally apply the how-it-works.md L144 newline-separation clause.
3. **IF an inaccuracy/inconsistency is found**: edit ONLY the offending statement (match the shipped
   behavior; do not duplicate P1.M2.T1.S1's configuration.md content; respect house style).

**Success Definition**: the audit grep confirms no inaccurate truncation/diff_context statement and no stale
sentinel-gluing/silent-clamp reference across the three docs; docs/configuration.md's diff_context content
is P1.M2.T1.S1's (not duplicated here); the outcome (no-op or the targeted edit) is recorded in
`research/docs-sweep-audit.md`; no code/PRD touched; markdownlint-compliant.

## User Persona

**Target User**: The reader of the stagecoach docs (a user configuring `token_limit`/`diff_context`, or
inspecting `--dry-run` truncation output) who must not be misled by a stale description of how truncated
sections are shaped or how an out-of-range `diff_context` is handled.

**Use Case**: A user reads how-it-works.md's water-fill description and/or configuration.md's diff_context
knob and forms a correct mental model: truncated sections are newline-separated; diff_context accepts 0–3
and rejects out-of-range values at load.

**User Journey**: (doc-only; no runtime) reader scans the docs → any truncation/diff_context mention is
consistent with the shipped behavior → no stale claim.

**Pain Points Addressed**: A future reader seeing a doc that (hypothetically) implied sentinels run together,
or that diff_context silently clamps, would be misled. This sweep confirms no such stale claim exists (and
optionally makes the line-shape explicit).

## Why

- **Closes out the changeset doc sync (Mode B).** Issues 1 & 2 landed code fixes; the catch-all doc sweep is
  the last step that guarantees the user-facing docs match the shipped behavior. (Bug-Fix PRD §h2.4
  "Areas needing attention" / SOW §5.)
- **Prevents the scattered-claim anti-pattern.** diff_context range/rejection has ONE authoritative home
  (configuration.md, via P1.M2.T1.S1); truncation line-shape is internal (not a config surface). This sweep
  confirms no doc re-states either in a way that could drift and contradict.
- **Cheap and safe.** A grep audit + (almost certainly) no edit, or one optional one-line clarification.
  Zero code change, zero behavioral risk.

## What

A verification sweep of three docs against two accuracy concerns, using a small term set. Expected outcome:
no-op confirmation (+ optional how-it-works.md L144 clarification). No code, no config, no API, no
behavioral change. The three docs are the only files possibly edited; the audit note is updated either way.

### Success Criteria

- [ ] The audit grep (§2) re-run against the live three docs; every truncation/sentinel/diff_context hit
      classified (accurate / inaccurate / optional-clarification).
- [ ] No inaccurate statement about (a) truncation/sentinel line-shape or (b) diff_context range/out-of-range.
- [ ] No stale reference to "sentinel gluing" or "silent diff_context clamping" anywhere.
- [ ] docs/configuration.md's diff_context range/rejection content is P1.M2.T1.S1's — NOT duplicated here.
- [ ] IF an inaccuracy is found → the offending line is edited to match shipped behavior (house-style-safe,
      no duplication). IF none → the three docs are NOT edited (the optional L144 clause excepted).
- [ ] `research/docs-sweep-audit.md` is appended with a dated re-verification line (no-op) or the edit record.
- [ ] PRD.md, all `.go` files, and the parallel P1.M2.T1.S1 edits untouched.

## All Needed Context

### Context Completeness Check

_Pass._ A writer with no prior repo knowledge can do this from: the pre-run audit matrix (research §2–§5),
the optional L144 wording (§4), the re-verification grep commands, and the P1.M2.T1.S1 ownership boundary.
No code/git/truncation-internals knowledge required — this is a prose-coherence check.

### Documentation & References

```yaml
# MUST READ — the pre-run audit (the whole task is confirming/extending it)
- docfile: plan/007_b33d310438c6/bugfix/001_7e79f5773da8/P1M3T1S1/research/docs-sweep-audit.md
  why: §1 (scope: P1.M2.T1.S1 owns configuration.md diff_context; this sweep owns how-it-works.md + README.md
       + the consistency check), §2 (concern a — sentinel line-shape: no doc inaccurate; optional L144 note),
       §3 (concern b — diff_context: configuration.md = P1.M2.T1.S1; how-it-works.md L138 valid; no edit),
       §4 (the ONE optional L144 clarification + exact wording), §5 (stale-reference check: none), §6 (house
       style), §7 (conclusion: no-op + optional).
  critical: §1/§3 (do NOT duplicate P1.M2.T1.S1's configuration.md diff_context docs) and §4 (the only
       recommended edit is optional) are the things most likely to go wrong (over-editing / duplicating).

# The bug context (in your context as selected_prd_content)
- file: plan/007_…/bugfix/001_7e79f5773da8/prd_snapshot.md (Bug-Fix PRD)
  section: "Issue 1" (sentinel glued to next `diff --git`; fix = trailing newline) + "Issue 2" (diff_context
           silent clamp; fix = config-layer range validation).
  critical: Issue 1's fix is INTERNAL line-shape (no doc stated line-shape ⇒ nothing to correct); Issue 2's
       fix is documented by P1.M2.T1.S1 in configuration.md.

# The parallel PRP — the ownership boundary (READ to avoid duplication)
- file: plan/007_…/bugfix/001_7e79f5773da8/P1M2T1S1/PRP.md
  why: confirms P1.M2.T1.S1 owns docs/configuration.md (3 spots + bootstrap.go:291: "valid range 0–3;
       out-of-range rejected at config load") and touches NEITHER how-it-works.md NOR README.md. This sweep
       must not restate the range/rejection in configuration.md.
  critical: the docs/configuration.md diff_context content after P1.M2.T1.S1 lands is the authoritative
       source — do not duplicate it in how-it-works.md (keep one source of truth).

# The files under sweep — READ every grep hit before deciding
- file: README.md
  section: L66 (features row — the ONLY token_limit/truncation mention; no diff_context; no line-shape).
  why: confirms README makes no truncation-format/diff_context claim ⇒ no edit.
- file: docs/how-it-works.md
  section: L134 (skeleton), L138 (diff_context 0/1/3), L140 (index-strip), L143 (legacy caps sentinels),
           L144 (water-fill + `... [truncated]` marker — the OPTIONAL clarification target), L146 (knob pointer).
  why: confirms L138 is accurate (0/1/3 valid) and L144 is silent on line-shape (not inaccurate; optional note).
- file: docs/configuration.md
  section: L104-107 (comment block), L130-131 (defaults table), L146-147 (prose) — the diff_context +
           token_limit knob docs P1.M2.T1.S1 is updating.
  why: confirms (post-P1.M2.T1.S1) the diff_context range/rejection is documented HERE (authoritative); this
       sweep does not touch it.

# House style
- file: .markdownlint.json
  why: default true; MD013 (line length) / MD033 (inline HTML) / MD060 disabled ⇒ long lines OK, inline code
       + plain prose compliant. The optional L144 edit is fully compliant.

# The regression-invariant reference (cited by the contract)
- file: plan/007_b33d310438c6/architecture/system_context.md
  section: §6 (invariants — token_limit==0 byte-identical legacy; token_limit>0 water-fill + `... [truncated]`
           per file). Confirms the shipped sentinel behavior this sweep checks docs against.
```

### Current Codebase tree (relevant slice)

```bash
README.md                       # features row L66 — the only token_limit/truncation mention; no diff_context; no line-shape (NO edit expected)
docs/how-it-works.md            # L138 diff_context (0/1/3, valid); L144 water-fill + marker (OPTIONAL clarification) — the sweep's primary surface
docs/configuration.md           # L107/L131/L147 diff_context + token_limit — P1.M2.T1.S1's domain (DO NOT duplicate)
.markdownlint.json              # house style (MD013/MD033/MD060 disabled)
plan/007_…/P1M3T1S1/
  PRP.md                        # THIS file
  research/docs-sweep-audit.md  # the audit note — APPEND the re-verification result here
# All .go files, PRD.md — UNCHANGED.
```

### Desired Codebase tree with files to be added

```bash
# NO new source/doc files. At most ONE in-place edit to docs/how-it-works.md (L144, optional).
# research/docs-sweep-audit.md is APPENDED (a dated re-verification line) — it already exists.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (this is a VERIFICATION; the expected result is NO edit): pre-research (research §2/§3/§7)
     shows all three docs are already consistent with the shipped behavior. Re-run the grep to confirm; do
     NOT manufacture edits. A no-op confirmation IS the successful outcome. -->

<!-- CRITICAL (do NOT duplicate P1.M2.T1.S1): docs/configuration.md's diff_context range/rejection content
     is P1.M2.T1.S1's deliverable. Do NOT restate "valid range 0–3; out-of-range rejected" in how-it-works.md
     or README.md — that re-scatters the claim (the anti-pattern this sweep prevents). how-it-works.md L138's
     0/1/3 listing is an explainer, not the authoritative range doc. -->

<!-- CRITICAL (do NOT invent a truncation-format spec): the sentinel line-shape is INTERNAL output detail.
     No doc states it; none needs a full spec. The ONLY truncation edit is the OPTIONAL one-clause L144 note
     (newline-separation). Do NOT add a "format" section. -->

<!-- GOTCHA (the optional L144 edit is truly optional): how-it-works.md L144 is accurate WITHOUT it (silent
     on line-shape, not inaccurate). Apply it only if it reads cleanly; skipping is acceptable. Record the
     choice either way. -->

<!-- GOTCHA (stale-reference check is already satisfied): no doc references "sentinel gluing" or "silent
     clamping" — there is nothing to REMOVE. Verify with the grep; expect zero hits. -->

<!-- GOTCHA (house style): markdownlint disables MD013/MD033/MD060 — long lines and inline code are fine.
     The recommended L144 edit uses inline code (`... [truncated]`, `diff --git`) + plain prose ⇒ compliant. -->

<!-- GOTCHA (scope boundary): README.md + docs/how-it-works.md + docs/configuration.md ONLY. Do NOT edit
     PRD.md, docs/cli.md, docs/providers.md, or any .go file. -->
```

## Implementation Blueprint

### Data models and structure

No code. The "implementation" is a re-runnable grep audit + a decision (no-op vs the optional edit). The
exact commands and decision matrix:

```bash
# THE audit grep — re-run against the three live docs. Terms: truncation/sentinel/diff_context/token_limit/water-fill.
grep -niE 'truncat|sentinel|\[truncated\]|diff_context|token_limit|water-fill|waterfill' README.md docs/how-it-works.md docs/configuration.md
# Stale-reference check (contract output req): expect ZERO hits for both.
grep -niE 'sentinel.*glue|glued|silent.*clamp|silently clamp' README.md docs/how-it-works.md docs/configuration.md
# Confirm no doc restates the diff_context range/rejection OUTSIDE configuration.md (P1.M2.T1.S1's home):
grep -niE 'range 0.3|out-of-range|rejected at config' docs/how-it-works.md README.md   # expect ZERO (configuration.md-only)
```

### Decision matrix (per grep hit)

```yaml
# For EACH truncation/diff_context hit, classify and act:
concern_a_truncation_line_shape:
  - A doc that STATES a sentinel line-shape that is now wrong, or IMPLIES sentinels run together ⇒ EDIT to
    match shipped behavior (newline-separated). (Pre-run: NONE found.)
  - A doc SILENT on line-shape (how-it-works.md L144) ⇒ optional one-clause clarification (§4); skip is OK.
  - A high-level mention with no line-shape claim (README L66, configuration.md L146) ⇒ NO edit.
concern_b_diff_context_range:
  - configuration.md range/rejection content ⇒ P1.M2.T1.S1 OWNS it; DO NOT duplicate. Touch only if
    P1.M2.T1.S1 left an inconsistency (none found).
  - how-it-works.md L138 listing 0/1/3 ⇒ accurate (all valid); the authoritative range detail is
    configuration.md's ⇒ NO edit (restating here duplicates).
  - README (no diff_context mention) ⇒ NO edit.
stale_reference:
  - "sentinel gluing" / "silent clamping" mention ⇒ REMOVE/fix. (Pre-run: NONE found.)
expected_outcome: all hits classify as accurate/optional ⇒ NO edit (or the optional L144 clause) ⇒ no-op
      confirmation.
```

### The ONE optional edit (how-it-works.md L144)

```markdown
<!-- current (L144 water-fill bullet): -->
…every file *larger* than `L` is truncated to `L` (with a `... [truncated]` marker). Small files are never…

<!-- OPTIONAL clarifying clause (append into the marker parenthetical): -->
…every file *larger* than `L` is truncated to `L` (with a `... [truncated]` marker that ends the file's
section on its own line, so the next file's `diff --git` begins fresh). Small files are never…
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: RE-RUN the audit grep against the three live docs
  - RUN the three grep commands above (truncation terms; stale-reference; range-outside-configuration.md).
  - CLASSIFY each hit via the decision matrix (concern a / concern b / stale-reference).
  - CAPTURE the hit list + classifications — the evidence for the no-op/edit decision.
  - NOTE: pre-run (research §2/§3/§5) found every hit accurate + zero stale references. The re-run is
      expected to confirm; flag ANY new inaccuracy (docs drifted, or P1.M2.T1.S1's configuration.md edit
      introduced a cross-doc inconsistency).

Task 2: DECIDE — no-op (± optional edit) vs a real fix
  - IF a real inaccuracy/stale-reference is found → Task 3a (EDIT, match shipped behavior, no duplication).
  - ELSE (expected: all accurate, no stale refs) → Task 3b (NO-OP, optionally apply the L144 clause).
  - DO NOT duplicate P1.M2.T1.S1's configuration.md diff_context content in how-it-works.md/README.md.
  - DO NOT invent a truncation-format spec (the optional L144 clause is the only truncation edit).

Task 3a: (ONLY if a real inaccuracy is found) EDIT the offending line
  - EDIT the statement to match shipped behavior (newline-separated sentinels; diff_context 0–3 rejected at
      load — but ONLY if the doc is NOT configuration.md, and ONLY if not duplicating P1.M2.T1.S1).
  - RESPECT house style (markdownlint: long lines OK; inline code OK). RECORD the edit in the audit note.

Task 3b: (EXPECTED) NO-OP confirmation ± optional L144 clarification
  - MAKE NO EDIT to README.md or docs/configuration.md.
  - docs/how-it-works.md: OPTIONALLY apply the L144 newline-separation clause (Task 3c). Skipping is OK.
  - APPEND a dated line to research/docs-sweep-audit.md: "Re-verified <DATE>: re-ran the audit grep against
      README.md/docs/how-it-works.md/docs/configuration.md@<HEAD>; no inaccurate truncation/diff_context
      statement; no stale sentinel-gluing/silent-clamp reference; configuration.md diff_context owned by
      P1.M2.T1.S1 (not duplicated); [optional L144 clause applied | not applied]; NO other edit."

Task 3c: (OPTIONAL) apply the how-it-works.md L144 newline-separation clause
  - IF applying: edit L144's marker parenthetical per the Blueprint wording. Re-run the truncation grep to
      confirm the new text reads cleanly. IF skipping: record "optional L144 clause not applied" in the note.

Task 4: VERIFY
  - RE-RUN the audit grep one final time → confirms no inaccuracy + no stale reference (and, if 3c ran, the
      L144 text is coherent and markdownlint-compliant).
  - RUN markdownlint on the edited doc (if any): `npx markdownlint-cli2 docs/how-it-works.md` (or the
      project's lint command) — expect clean (MD013/MD033/MD060 disabled).
  - CONFIRM byte-unchanged: PRD.md, all .go files, docs/cli.md, docs/providers.md untouched. (README.md and
      configuration.md unchanged iff Task 3b no-op.)
  - (No build/test needed — doc-only. But `go test ./...` must stay green — this task changes no code.)
```

### Implementation Patterns & Key Details

```markdown
<!-- THE shipped behavior this sweep checks docs against:
     (a) each truncated file's section ends with `... [truncated]\n`; the next file's `diff --git` begins at
         a line start (Issue 1 / P1.M1.T1.S1 — internal line-shape).
     (b) diff_context accepts 0/1/2/3; an out-of-range value fails config load with a clear, field-named
         error (Issue 2 / P1.M2.T1.S1 — documented in configuration.md). -->
<!-- THE no-op is the successful outcome. Record it; do not fabricate edits. -->
<!-- THE scatter anti-pattern: diff_context range/rejection in ONE place (configuration.md, P1.M2.T1.S1);
     truncation line-shape is internal (optionally surfaced once in how-it-works.md L144). Never re-state. -->
```

### Integration Points

```yaml
DOCUMENTATION (Mode B): this IS the changeset-level doc sweep for bugfix 001_7e79f5773da8 (Issues 1 & 2).
      It depends on P1.M1.T1.S1 (sentinel fix), P1.M1.T2.S1 (E2E coverage), P1.M2.T1.S1 (diff_context
      validation + its configuration.md docs). Together they complete the Mode-B doc sync.

CODE: NONE. No .go file is touched. `go test ./...` stays green by construction (no code change).

FROZEN/LEAVE (do NOT edit):
  - PRD.md (read-only, human-owned).
  - docs/cli.md + docs/providers.md (out of scope for this changeset's two concerns).
  - docs/configuration.md's diff_context range/rejection content (P1.M2.T1.S1's deliverable — do not duplicate).
  - All .go files (the fixes are in P1.M1/P1.M2; this is doc-only).

NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG.
```

## Validation Loop

### Level 1: The audit grep (re-run + classify)

```bash
# Truncation/diff_context terms across the three docs.
grep -niE 'truncat|sentinel|\[truncated\]|diff_context|token_limit|water-fill|waterfill' README.md docs/how-it-works.md docs/configuration.md
# Stale-reference check — expect ZERO hits.
grep -niE 'sentinel.*glue|glued|silent.*clamp|silently clamp' README.md docs/how-it-works.md docs/configuration.md
# diff_context range/rejection should live ONLY in configuration.md (P1.M2.T1.S1) — expect ZERO elsewhere.
grep -niE 'range 0.3|out-of-range|rejected at config' docs/how-it-works.md README.md
# Expected: every truncation/diff_context hit is accurate (or the optional L144 target); zero stale refs;
# zero range/rejection content outside configuration.md. If so → no-op (+ optional L144).
```

### Level 2: Decision evidence (no-op vs edit)

```bash
# Walk each grep hit through the decision matrix. Expected verdict: NO edit required (or the optional L144 clause).
# If an inaccuracy WAS edited in Task 3a, re-run the truncation grep to confirm the fix reads cleanly.
```

### Level 3: Byte-unchanged guard + markdownlint (no scope creep)

```bash
git diff --exit-code PRD.md docs/cli.md docs/providers.md && echo "out-of-scope docs UNCHANGED (expected)"
git diff --name-only -- '*.go' | grep -q . && echo "UNEXPECTED .go change" || echo "no .go changed (expected)"
# README.md + docs/configuration.md: unchanged iff Task 3b no-op.
git diff --stat -- README.md docs/configuration.md docs/how-it-works.md
# Expected (no-op): only docs/how-it-works.md MAY show the optional 1-line L144 edit (or none).
# markdownlint on the edited doc (if any):
npx markdownlint-cli2 docs/how-it-works.md 2>/dev/null || npx markdownlint docs/how-it-works.md 2>/dev/null || echo "(markdownlint not installed; visually verify MD013/MD033/MD060 compliance)"
# Confirm the audit note recorded the outcome:
tail -6 plan/007_b33d310438c6/bugfix/001_7e79f5773da8/P1M3T1S1/research/docs-sweep-audit.md
```

### Level 4: Whole-repo still green (doc-only, but confirm no accidental code touch)

```bash
go build ./...   # Expect clean (no code changed).
go test ./...    # Expect all PASS (doc-only task; belt-and-suspenders). If RED, a code file was edited by
                 # mistake — revert it.
```

## Final Validation Checklist

### Technical Validation
- [ ] The audit grep re-run; every truncation/diff_context hit classified (accurate/optional); zero stale refs.
- [ ] No inaccurate truncation/sentinel line-shape or diff_context range/out-of-range statement.
- [ ] `go build ./... && go test ./...` GREEN (doc-only; no code touched).
- [ ] PRD.md / docs/cli.md / docs/providers.md / all `.go` files byte-unchanged; configuration.md diff_context not duplicated.

### Feature Validation
- [ ] No stale reference to pre-fix sentinel-gluing or silent diff_context clamping.
- [ ] docs/configuration.md's diff_context range/rejection is P1.M2.T1.S1's (not duplicated in how-it-works.md/README.md).
- [ ] Outcome recorded in `research/docs-sweep-audit.md` (no-op confirmation OR the targeted edit + the optional L144 choice).

### Code Quality Validation
- [ ] No edit manufactured where no inaccuracy exists (the scatter anti-pattern is NOT introduced).
- [ ] IF edited: house-style-safe (markdownlint: long lines/inline code OK); one source of truth preserved.
- [ ] Scope held to the three docs (+ the research note); sibling docs / PRD / code untouched.

### Documentation
- [ ] `research/docs-sweep-audit.md` carries the dated re-verification line (this IS the Mode-B sweep record).

---

## Anti-Patterns to Avoid

- ❌ **Don't manufacture edits.** Pre-research (§2/§3/§7) shows the three docs are consistent; the no-op is
      the expected and successful outcome. Re-run the grep, and if clean, record a no-op — do NOT invent edits.
- ❌ **Don't duplicate P1.M2.T1.S1.** docs/configuration.md's diff_context range/rejection is its deliverable.
      Do NOT restate "valid range 0–3; out-of-range rejected" in how-it-works.md or README.md — that re-scatters
      the claim (the anti-pattern this sweep exists to prevent). (research §1/§3)
- ❌ **Don't invent a truncation-format spec.** The sentinel line-shape is INTERNAL; no doc states it, none
      needs a full spec. The ONLY truncation edit is the OPTIONAL one-clause L144 note. Do NOT add a "format"
      section. (research §2/§4)
- ❌ **Don't treat the optional L144 note as mandatory.** how-it-works.md L144 is accurate without it (silent
      on line-shape, not inaccurate). Apply only if it reads cleanly; skipping is acceptable. Record the choice.
- ❌ **Don't edit configuration.md's diff_context content.** That's P1.M2.T1.S1's surface. Touch it only if
      P1.M2.T1.S1 left an inconsistency (none found). (research §1/§3)
- ❌ **Don't conflate how-it-works.md's 0/1/3 listing (L138) with the authoritative range doc.** L138 is an
      explainer; the range/out-of-range contract lives in configuration.md. Restating the range in L138
      duplicates. Leave L138 as-is. (research §3)
- ❌ **Don't edit PRD.md, docs/cli.md, docs/providers.md, or any .go file.** Scope is the three docs only.
- ❌ **Don't skip the re-verification.** Docs could drift between research and implementation (and P1.M2.T1.S1
      is editing configuration.md in parallel). Re-run the grep at implementation time; record the dated result.

---

## Confidence Score

**10/10** — a verification task whose outcome is pre-determined by a complete grep audit: across README.md,
docs/how-it-works.md, and docs/configuration.md there is NO inaccurate statement about truncation/sentinel
line-shape (Issue 1's fix is internal; no doc states line-shape) and NO inaccurate statement about diff_context
range (configuration.md is P1.M2.T1.S1's authoritative surface; how-it-works.md L138's 0/1/3 is valid), and NO
stale reference to sentinel-gluing or silent clamping. The implementer re-runs the same grep to confirm (docs
drift / parallel configuration.md edit) and, finding it clean, records a no-op — the successful outcome —
optionally applying the one-clause how-it-works.md L144 newline-separation clarification. No code, no behavioral
risk, no edit unless a real inconsistency surfaces. The only residual risk — a cross-doc inconsistency
introduced by the parallel P1.M2.T1.S1 configuration.md edit — is closed by the mandatory re-verification grep
(which explicitly checks that range/rejection content lives ONLY in configuration.md).
