---
name: "P1.M3.T1.S1 — Sweep README.md for cross-cutting coherence (FR52 run-lock qualification, Issue 1 changeset doc sync, Mode B)"
description: |

  Changeset-level documentation sweep (Mode B) for README.md, closing out the Issue-1 fix (FR52 per-repo run
  lock; Bug-Fix PRD §h2.2 Issue 1). P1.M1.T1.S1 already QUALIFIED the "Safe to run twice" FAQ paragraph
  (README.md:330) to scope the no-op fast path (exit 0 on accidental double-run) to the SINGLE-COMMIT
  (staged) path only — on the DECOMPOSE path an accidental double-run exits 5 (Busy), never 0, because the
  in-progress run publishes a working-tree snapshot a contender can't reproduce without the lock. This
  subtask verifies that NO OTHER README section re-introduces the unconditional "exits 0" claim or otherwise
  contradicts the qualified L330 claim, and edits any contradiction found (or, if none, records a no-op
  confirmation).

  ⚠️ **RESEARCH OUTCOME (pre-verified): README.md is COHERENT — this is a NO-OP CONFIRMATION.** A full grep
  audit (every contract term: run lock / FR52 / Safe to run / nothing to do / exit 0 / exit 5 / Busy /
  concurrent / double-run / decompose / no-op fast path / accidental, plus the broader atomic/never-corrupt
  safety set) shows the run-lock / no-op-fast-path / exit-0-or-5 / "Safe to run twice" discussion exists in
  EXACTLY ONE place — L330 — which is already correctly qualified. Every other "decompose" hit (L4 tagline,
  L31 features table, L139-168 pipeline section, L338 FAQ) describes the FEATURE/PIPELINE, not contention,
  and makes NO double-run/exit claim; the other exit mentions (L126 dry-run, L216 provider-missing, L253
  config-missing — all "exit 1") are unrelated to the lock. No contradiction exists. See
  research/readme-sweep-audit.md §2 (the full hit-by-hit table) — the single most important read.

  ⚠️ **THE task is a VERIFICATION, not a guaranteed edit.** Because README.md could drift between this
  research and implementation, the implementer MUST re-run the §2 audit grep against the live README.md at
  implementation time. If it is still clean (expected), make NO README edit and record the no-op in
  research/readme-sweep-audit.md (append a dated "re-verified" line). If a contradiction IS found, edit the
  offending line to match L330's qualified claim (single-commit path → 0/5; decompose path → 5/Busy). Do NOT
  invent edits where no contradiction exists (scattering the claim is the anti-pattern this sweep prevents).

  ⚠️ **DO NOT re-introduce the unconditional "exits 0" claim anywhere.** The bug P1.M1.T1.S1 fixed was an
  unqualified "if nothing new is staged it exits 0" that held for both paths. Any edit made here must MATCH
  L330's qualification — never a bare "exits 0". (Audit confirmed: the only "exit 0" in README is inside
  L330's qualified single-commit clause — research §4.)

  ⚠️ **DO NOT add a redundant Busy(5) caveat to the decompose pipeline section (L139-168) or the
  features/tagline (L4/L31).** Those describe the FEATURE, not contention. The contention behavior correctly
  lives in ONE place (the FAQ, L330), immediately qualified in the same paragraph. Adding the caveat
  elsewhere would re-create the scattered-claim problem this sweep exists to prevent. (research §3.)

  ⚠️ **DO NOT edit PRD.md.** And do NOT edit docs/cli.md or docs/how-it-works.md — those are the SIBLING
  sweep P1.M3.T1.S2. This task is README.md ONLY.

  ⚠️ **No conflict with the parallel work item.** P1.M2.T4.S1 (Issue 4b, contention-message empty-field
  guard) touches ONLY Go code (`internal/cmd/default_action.go`, `internal/cmd/lock_contention_test.go`,
  `internal/exitcode/exitcode.go`, `internal/lock/lock.go`, `internal/lock/lock_unix.go`) — no markdown. This
  task touches ONLY README.md. Zero overlap. (research §5.)

  Deliverable: either (a) NO README.md edit + a dated no-op confirmation appended to
  research/readme-sweep-audit.md (EXPECTED), or (b) a targeted edit to a contradicting line (if the re-audit
  finds one). OUTPUT: README.md coherent with the qualified L330 claim across all sections. DOCS: this IS the
  changeset-level doc sweep for README.md (Mode B).

---

## Goal

**Feature Goal**: Verify README.md is cross-cuttingly coherent with the P1.M1.T1.S1 qualification of the
FR52 run-lock "Safe to run twice" claim (L330): no other section re-introduces an unconditional "exits 0"
on double-run, and no section contradicts the single-commit→0/5 / decompose→5(Busy) split. Pre-research
shows README is already coherent, so the expected outcome is a NO-OP confirmation; the implementer re-runs
the audit to confirm and only edits if a real contradiction is found.

**Deliverable**:
1. **RE-RUN** the §2 audit grep against the live `README.md` (every contract term).
2. **IF clean (expected)**: make NO README edit; append a dated "re-verified coherent — no edit" line to
   `plan/006_…/P1M3T1S1/research/readme-sweep-audit.md`.
3. **IF a contradiction is found**: edit the offending line(s) to match L330's qualified claim
   (single-commit path → exit 0/5; decompose path → exit 5/Busy). One authoritative place (L330); never
   re-scatter, never a bare "exits 0".

**Success Definition**: the audit grep confirms the run-lock/no-op/exit-0-5/"Safe to run twice" discussion
is singleton at L330 and no other section contradicts it; README.md (edited only if needed) is coherent;
the no-op (or the targeted edit) is recorded in `research/readme-sweep-audit.md`; PRD.md untouched;
docs/cli.md + docs/how-it-works.md untouched (sibling sweep P1.M3.T1.S2).

## User Persona

**Target User**: The README reader (a new or prospective stagecoach user) reading any section that mentions
decompose, safety, or running the tool — who must not be misled by a stale, unconditional "exits 0 on
double-run" claim into thinking the decompose path no-ops on an accidental double-tap (it exits Busy/5).

**Use Case**: A user reads the decompose section or the FAQ and forms a correct mental model of the run-lock
contention behavior: single-commit double-runs can no-op (exit 0); decompose double-runs always exit Busy
(5). No section contradicts this.

**User Journey**: (doc-only; no runtime) reader scans README → any decompose/lock mention is consistent with
L330 → no false "exits 0" expectation on the decompose path.

**Pain Points Addressed**: The Issue-1 doc/behavior mismatch (README promised exit 0 unconditionally; the
decompose path exits 5). P1.M1.T1.S1 fixed L330; this sweep guarantees no OTHER section still carries the
old unqualified claim.

## Why

- **Closes out the Issue-1 documentation fix.** P1.M1.T1.S1 qualified L330; a cross-cutting sweep is the
  Mode-B doc-sync step that ensures the qualification isn't undercut by a stray redundant claim elsewhere
  in the same file. (Bug-Fix PRD §h2.4 "Areas needing attention".)
- **Prevents the scattered-claim anti-pattern.** Lock/contention behavior must live in ONE authoritative
  place (the FAQ, L330). If another section restated the old "exits 0" claim, a future edit to L330 wouldn't
  propagate and the docs would silently contradict again. This sweep confirms single-source-of-truth.
- **Cheap and safe.** A grep audit + (almost certainly) no edit. Zero code change, zero behavioral risk.

## What

A verification sweep of `README.md` against the qualified L330 claim, using the contract's term list.
Expected outcome: no-op confirmation (README is coherent). If the re-audit finds a contradiction, a targeted
edit to match L330. No code, no config, no API, no behavioral change. README.md is the only file possibly
edited; the audit note is updated either way.

### Success Criteria

- [ ] The §2 audit grep re-run against the live README.md: every contract term (run lock, FR52, Safe to run,
      nothing to do, exit 0, exit 5, Busy, concurrent, double-run, decompose, no-op fast path, accidental)
      accounted for; the run-lock/no-op/exit-0-5 discussion is confirmed singleton at L330.
- [ ] No section outside L330 makes an unconditional "exits 0 on double-run" claim or contradicts the
      single-commit→0/5 / decompose→5(Busy) split.
- [ ] IF a contradiction is found → the offending line is edited to match L330's qualified claim (single
      authoritative place; no bare "exits 0"). IF none → README.md is NOT edited.
- [ ] `research/readme-sweep-audit.md` is appended with a dated "re-verified" line (no-op) or a record of
      the edit made.
- [ ] PRD.md, docs/cli.md, docs/how-it-works.md, and all `.go` files are byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A writer with no prior repo knowledge can do this from: the pre-run audit table (research §2 — every
hit pre-classified), the authoritative L330 quote (research §1), the re-verification grep commands, and the
edit/no-op decision rule. No code/git/lock-internals knowledge required — this is a prose-coherence check.

### Documentation & References

```yaml
# MUST READ — the pre-run audit (the whole task is confirming/extending it)
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M3T1S1/research/readme-sweep-audit.md
  why: §1 (the authoritative L330 qualified claim, verbatim), §2 (the FULL hit-by-hit grep table — every
       contract term → every line → verdict), §3 (why the decompose section needs NO caveat), §4 (no bare
       "exits 0" anywhere), §5 (no conflict with P1.M2.T4.S1), §6 (conclusion: no-op).
  critical: §2 is the audit you re-run; §3/§4 are the "do NOT invent edits" guardrails (don't add a redundant
       Busy caveat to the decompose section; don't re-introduce a bare "exits 0").

# The file under sweep — READ L330 + every grep hit before deciding
- file: README.md
  section: L330 — the FAQ "Safe to run twice." paragraph (the AUTHORITATIVE qualified claim; the reference).
  section: L4 (tagline), L31 (features table), L139-168 (decompose section), L338 (FAQ "multiple commits") —
           the non-L330 decompose mentions; all describe the feature/pipeline, NONE the contention behavior.
  section: L126 / L216 / L253 — unrelated "exit 1" mentions (dry-run / provider-missing / config-missing).
  why: confirms (re-verification) that no section outside L330 discusses the lock/no-op/exit-0-5.
  critical: L330 is the ONLY place the run lock + exit 0/5 + "Safe to run twice" appear. Do NOT edit it (it
       is already correct); edit only a DIFFERENT line if it contradicts L330.

# The bug context (already in your context as selected_prd_content)
- file: plan/006_…/bugfix/001_624c8013d3b2/prd_snapshot.md (Bug-Fix PRD)
  section: "Issue 1" (h3.0/h2.2) — the original unconditional "exits 0" README claim + the qualified fix.
  critical: the qualification is single-commit→0/5, decompose→5(Busy); match it exactly if any edit is made.

# The prior fix this sweep validates (READ its exact wording of L330)
- file: plan/006_…/bugfix/001_624c8013d3b2/P1M1T1S1/PRP.md
  why: P1.M1.T1.S1 wrote the qualified L330. This sweep confirms nothing else in README contradicts it.

# The sibling sweep (do NOT duplicate)
- file: plan/006_…/bugfix/001_624c8013d3b2/P1M3T1S2/PRP.md (when it exists)
  why: P1.M3.T1.S2 sweeps docs/how-it-works.md + docs/cli.md. This task (S1) is README.md ONLY — do not
       touch those docs here.

# The parallel code task (no conflict)
- file: plan/006_…/bugfix/001_624c8013d3b2/P1M2T4S1/PRP.md
  why: confirms P1.M2.T4.S1 touches only .go files (no markdown) ⇒ zero overlap with this README sweep.
```

### Current Codebase tree (relevant slice)

```bash
README.md                                  # THE file under sweep (read; edit ONLY if a contradiction is found)
plan/006_…/P1M3T1.S1/
  PRP.md                                   # THIS file
  research/readme-sweep-audit.md           # the audit note — APPEND the re-verification result here
# All .go files, PRD.md, docs/cli.md, docs/how-it-works.md — UNCHANGED.
```

### Desired Codebase tree with files to be added

```bash
# NO new source/doc files. At most ONE in-place edit to README.md (only if the re-audit finds a contradiction).
# research/readme-sweep-audit.md is APPENDED (a dated re-verification line) — it already exists.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (this is a VERIFICATION; the expected result is NO edit): pre-research (research §2/§6) shows
     README.md is already coherent — the lock/no-op/exit-0-5 discussion is singleton at L330. Re-run the grep
     to confirm; do NOT manufacture an edit. A no-op confirmation IS the successful outcome. -->

<!-- CRITICAL (do NOT re-introduce a bare "exits 0"): the only "exit 0" in README is inside L330's qualified
     single-commit clause. Any edit must mirror L330's qualification (single-commit→0/5; decompose→5). Never
     write an unqualified "exits 0" / "will exit 0" / "if nothing new is staged it exits 0" anywhere. -->

<!-- CRITICAL (do NOT add a Busy(5) caveat to the decompose pipeline section L139-168, the tagline L4, or the
     features table L31): those describe the FEATURE/pipeline, not contention. Contention lives in ONE place
     (FAQ L330), already qualified in-paragraph. Re-scattering the claim is the anti-pattern this sweep
     prevents. (research §3.) -->

<!-- GOTCHA (the other "exit" mentions are unrelated): L126 (--dry-run exit 1), L216 (provider-missing exit 1),
     L253 (config-missing exit 1) are NOT contention/lock exits — leave them. Only exit 0/5 in a double-run
     context is in scope, and that's only at L330. -->

<!-- GOTCHA (scope boundary): README.md ONLY. Do NOT edit PRD.md (read-only), and do NOT edit docs/cli.md or
     docs/how-it-works.md — those are the sibling sweep P1.M3.T1.S2. -->

<!-- GOTCHA (no conflict with P1.M2.T4.S1): the parallel task edits only .go files (default_action.go,
     lock.go, exitcode.go, lock_contention_test.go, lock_unix.go) — no markdown. README.md is yours alone. -->
```

## Implementation Blueprint

### Data models and structure

No code. The "implementation" is a re-runnable grep audit + a decision (edit vs no-op). The exact commands
and the decision matrix:

```bash
# THE audit grep — re-run against the live README.md. Terms from the contract (+ the broader safety set).
grep -niE 'run.?lock|FR52|safe to run|nothing to do|exit (0|5)|\bBusy\b|concurrent|double.?run|decompos|no-op fast path|accidental' README.md
# plus the broader safety/atomic set (to catch any unconditional-safety claim that implies double-run safety):
grep -niE 'atomic|never corrupt|byte-for-byte|clobber|race' README.md
# plus a targeted check that NO bare/unconditional "exits 0" exists outside L330's qualified clause:
grep -niE 'exits? `?0`?|will exit 0|it exits 0' README.md   # every hit MUST be inside L330's qualified clause
```

### Decision matrix (per grep hit outside L330)

```yaml
# For EACH line the grep returns (other than L330), classify and act:
hit_classifies_as:
  - CONTENTION_DISCUSSION: mentions the run lock / no-op fast path / exit 0-or-5 / "safe to run twice" /
      double-run BEHAVIOR. If OUTSIDE L330 ⇒ the only case that may need an edit. Compare to L330; if it
      contradicts (e.g. a bare "exits 0" for decompose, or an unqualified "safe to run twice") ⇒ EDIT to
      match L330 (single-commit→0/5; decompose→5). If it merely DUPLICATES L330 faithfully ⇒ collapse to a
      pointer ("see FAQ") to keep one source of truth — but ONLY if a duplicate exists (none found in audit).
  - FEATURE_DESCRIPTION: mentions decompose as a feature (tagline L4, table L31, pipeline L139-168, FAQ L338)
      with NO contention/lock/exit claim ⇒ NO edit (do not add a Busy caveat — research §3).
  - UNRELATED_EXIT: an "exit 1" for dry-run/provider-missing/config-missing (L126/L216/L253) ⇒ NO edit.
  - SAFETY_MECHANISM: "atomic"/"never corrupt"/"byte-for-byte" about the snapshot/CAS (L4/L113/L274/L328/L358/
      L366) with NO double-run claim ⇒ NO edit (these remain true; the CAS is the never-clobber guarantee).
expected_outcome: ALL hits classify as FEATURE_DESCRIPTION / UNRELATED_EXIT / SAFETY_MECHANISM or are L330
      itself ⇒ NO edit ⇒ no-op confirmation.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: RE-RUN the audit grep against the live README.md
  - RUN the three grep commands above (contention terms; safety set; bare-"exits 0" check).
  - FOR EACH hit ≠ L330, classify it via the decision matrix (CONTENTION_DISCUSSION / FEATURE_DESCRIPTION /
      UNRELATED_EXIT / SAFETY_MECHANISM).
  - CAPTURE the result (the hit list + classifications) — this is the evidence for the no-op/edit decision.
  - NOTE: the pre-run audit (research §2) found every non-L330 hit is FEATURE_DESCRIPTION/UNRELATED_EXIT/
      SAFETY_MECHANISM. The re-run is expected to confirm this; flag ANY new contention hit (README drifted).

Task 2: DECIDE — edit vs no-op
  - IF any non-L330 hit classifies as CONTENTION_DISCUSSION and contradicts L330 (e.g. a bare "exits 0" for
      decompose, or an unqualified "safe to run twice") → Task 3a (EDIT).
  - ELSE (expected: all non-L330 hits are feature/unrelated/safety, or faithful L330 duplicates) → Task 3b
      (NO-OP).
  - DO NOT invent an edit for a FEATURE_DESCRIPTION hit (no Busy caveat on the decompose section — research §3).
  - DO NOT edit L330 itself (it is already correct — the reference).

Task 3a: (ONLY if a contradiction is found) EDIT the offending README line
  - EDIT the contradicting line to match L330's qualified claim: single-commit path → exit 0 (nothing new) /
      5 (Busy); decompose path → exit 5 (Busy). Keep ONE authoritative place (L330); prefer collapsing a
      redundant restatement to a pointer ("see the FAQ 'Safe to run twice'") rather than re-stating.
  - NEVER write a bare/unconditional "exits 0". Re-run the bare-"exits 0" grep to confirm the fix.
  - RECORD the edit (line, before/after, reason) in research/readme-sweep-audit.md.

Task 3b: (EXPECTED) NO-OP confirmation
  - MAKE NO EDIT to README.md.
  - APPEND a dated line to research/readme-sweep-audit.md, e.g.:
      "Re-verified <DATE>: re-ran the §2 audit grep against README.md@<HEAD>; the run-lock/no-op/exit-0-5
       discussion remains singleton at L330; no non-L330 hit classifies as CONTENTION_DISCUSSION; no
       contradiction found; NO README edit required (no-op confirmation)."
  - INCLUDE the captured hit list (or a reference to it) as evidence.

Task 4: VERIFY
  - RE-RUN the audit grep one final time → confirms singleton-at-L330 (and, if Task 3a ran, the bare-
      "exits 0" check is clean outside L330).
  - CONFIRM byte-unchanged: PRD.md, docs/cli.md, docs/how-it-works.md, all .go files untouched. (README.md
      unchanged iff Task 3b.)
  - (No build/test needed — doc-only. But `go test ./...` must still be green — this task changes no code.)
```

### Implementation Patterns & Key Details

```markdown
<!-- THE authoritative claim to match (L330, verbatim — do not edit, match against it):
     "Safe to run twice. A per-repo run lock … On the SINGLE-COMMIT PATH (changes staged), an accidental
     double-invoke exits 0 if nothing new has been staged … or exits 5 (Busy) if genuinely new work is
     staged … On the DECOMPOSE PATH (nothing staged, dirty working tree), an accidental double-run exits 5
     (Busy) rather than 0 …" -->

<!-- THE no-op is the successful outcome. Record it; do not fabricate an edit. -->
<!-- THE scatter anti-pattern: lock/contention behavior in ONE place (L330). Never re-state it in the
     decompose section / tagline / features table. If a faithful duplicate exists, collapse to a pointer. -->
```

### Integration Points

```yaml
DOCUMENTATION (Mode B): this IS the changeset-level README sweep for the Issue-1 (FR52 lock qualification)
      changeset. Sibling: P1.M3.T1.S2 sweeps docs/how-it-works.md + docs/cli.md. Together they complete the
      Mode-B doc sync for the changeset.

CODE: NONE. No .go file is touched. `go test ./...` stays green by construction (no code change).

FROZEN/LEAVE (do NOT edit):
  - PRD.md (read-only, human-owned).
  - docs/cli.md + docs/how-it-works.md (sibling sweep P1.M3.T1.S2).
  - L330 itself (already the correct qualified claim — the reference, not a target).
  - All .go files; the parallel P1.M2.T4.S1 code changes are independent.

NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG.
```

## Validation Loop

### Level 1: The audit grep (re-run + classify)

```bash
# Contention terms — every hit ≠ L330 must NOT be a CONTENTION_DISCUSSION (expected: all are feature/unrelated/safety).
grep -niE 'run.?lock|FR52|safe to run|nothing to do|exit (0|5)|\bBusy\b|concurrent|double.?run|decompos|no-op fast path|accidental' README.md
# Safety set — confirm no unconditional double-run-safety claim.
grep -niE 'atomic|never corrupt|byte-for-byte|clobber|race' README.md
# Bare-"exits 0" check — every hit MUST be inside L330's qualified single-commit clause.
grep -niE 'exits? `?0`?|will exit 0|it exits 0' README.md
# Expected: the contention terms hit ONLY at L330 (the qualified claim); "decompos" hits at L4/L31/L139-168/
# L330/L338 (feature descriptions, no contention); bare-"exits 0" hits ONLY inside L330. If so → no-op.
```

### Level 2: Decision evidence (edit vs no-op)

```bash
# Confirm no non-L330 CONTENTION_DISCUSSION contradiction exists:
# (manual: walk each grep hit ≠ 330 through the decision matrix in the Blueprint.)
# Expected verdict: NO edit required (README coherent).
# If a contradiction WAS edited in Task 3a, re-run the bare-"exits 0" grep to confirm the fix:
grep -niE 'exits? `?0`?|will exit 0|it exits 0' README.md   # only L330's qualified clause may match.
```

### Level 3: Byte-unchanged guard (no scope creep)

```bash
git diff --exit-code PRD.md docs/cli.md docs/how-it-works.md && echo "frozen docs UNCHANGED (expected)"
git diff --name-only -- '*.go' | grep -q . && echo "UNEXPECTED .go change" || echo "no .go changed (expected)"
# README.md: unchanged iff Task 3b (no-op); one targeted edit iff Task 3a.
git diff --stat -- README.md
# Expected (no-op): empty. Expected (edit): exactly the one contradicting line.
# Confirm the audit note recorded the outcome:
tail -5 plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M3T1S1/research/readme-sweep-audit.md
```

### Level 4: Whole-repo still green (doc-only, but confirm no accidental code touch)

```bash
go build ./...   # Expect clean (no code changed).
go test ./...    # Expect all PASS (doc-only task; this is a belt-and-suspenders check that nothing was
                 # accidentally touched). If RED, a code file was edited by mistake — revert it.
```

## Final Validation Checklist

### Technical Validation
- [ ] The §2 audit grep re-run; every non-L330 hit classified (feature/unrelated/safety, expected).
- [ ] No non-L330 CONTENTION_DISCUSSION contradiction; no bare/unconditional "exits 0" outside L330.
- [ ] `go build ./... && go test ./...` GREEN (doc-only; no code touched).
- [ ] PRD.md / docs/cli.md / docs/how-it-works.md / all `.go` files byte-unchanged.

### Feature Validation
- [ ] README.md's run-lock/no-op/exit-0-5/"Safe to run twice" discussion is singleton at L330 (qualified).
- [ ] No other section contradicts L330 (single-commit→0/5; decompose→5/Busy) or needs a Busy(5) caveat.
- [ ] Outcome recorded in `research/readme-sweep-audit.md` (no-op confirmation OR the targeted edit).

### Code Quality Validation
- [ ] No edit manufactured where no contradiction exists (the scatter anti-pattern is NOT re-introduced).
- [ ] IF edited: the edit matches L330's qualification exactly; one authoritative place preserved.
- [ ] Scope held to README.md (+ the research note); sibling docs / PRD / code untouched.

### Documentation
- [ ] `research/readme-sweep-audit.md` carries the dated re-verification line (this IS the Mode-B sweep record).

---

## Anti-Patterns to Avoid

- ❌ **Don't manufacture an edit.** Pre-research (§2/§6) shows README is coherent; the no-op is the expected
      and successful outcome. Re-run the grep to confirm, and if clean, record a no-op — do NOT invent edits.
- ❌ **Don't re-introduce a bare/unconditional "exits 0".** The bug P1.M1.T1.S1 fixed was exactly that. Any
      edit must mirror L330's qualification (single-commit→0/5; decompose→5). The only "exit 0" in README is
      inside L330's qualified clause — keep it that way.
- ❌ **Don't add a Busy(5) caveat to the decompose section (L139-168), tagline (L4), or features table (L31).**
      Those describe the FEATURE/pipeline, not contention. Contention lives in ONE place (FAQ L330), already
      qualified in-paragraph. Re-scattering is the anti-pattern this sweep prevents. (research §3.)
- ❌ **Don't edit L330 itself.** It is the AUTHORITATIVE qualified claim (P1.M1.T1.S1) — the reference you
      check OTHER lines against, not a target. Only a DIFFERENT contradicting line is editable.
- ❌ **Don't edit PRD.md, docs/cli.md, or docs/how-it-works.md.** PRD is read-only; the two docs are the
      sibling sweep P1.M3.T1.S2. This task is README.md ONLY.
- ❌ **Don't conflate unrelated exit-code mentions with the lock.** L126 (dry-run exit 1), L216
      (provider-missing exit 1), L253 (config-missing exit 1) are NOT contention exits — leave them.
- ❌ **Don't skip the re-verification.** README could drift between research and implementation. Re-run the
      grep at implementation time; record the dated result. (The no-op must be EARNED by re-verification,
      not assumed.)
- ❌ **Don't touch the parallel P1.M2.T4.S1's files.** It edits only `.go` (default_action.go/lock.go/etc.);
      README.md is yours alone — but do not edit those `.go` files either.

---

## Confidence Score

**10/10** — This is a verification task whose outcome is pre-determined by a complete grep audit: the
run-lock / no-op-fast-path / exit-0-or-5 / "Safe to run twice" discussion is singleton at README.md:330
(already correctly qualified by P1.M1.T1.S1), and every other relevant hit (decompose feature/section at
L4/L31/L139-168/L338; unrelated exit-1s at L126/L216/L253; snapshot/CAS safety at L113/L274/L328/L358/L366)
is a feature/unrelated/safety mention that makes NO contention claim and so neither contradicts nor needs a
Busy(5) caveat. The implementer re-runs the same grep to confirm (README could drift) and, finding it clean,
records a no-op — the successful outcome. No code, no behavioral risk, no edit unless a real contradiction
surfaces. The only residual risk — README drifting between research and implementation — is closed by the
mandatory re-verification grep + the edit/no-op decision matrix.
