---
name: "P1.M3.T1.S2 — Sweep docs/how-it-works.md and docs/cli.md for cross-cutting coherence (FR52 run-lock qualification, Issue 1 changeset doc sync, Mode B)"
description: |

  Changeset-level documentation sweep (Mode B) for docs/how-it-works.md + docs/cli.md (+ a confirmation pass
  on docs/configuration.md), closing out the Issue-1 fix (FR52 per-repo run lock; Bug-Fix PRD §h2.2 Issue 1).
  P1.M1.T1.S2/S3 already QUALIFIED the two contention-behavior claims — docs/cli.md:379 and
  docs/how-it-works.md:155 — to scope the no-op fast path to the SINGLE-COMMIT (staged) path only (decompose
  accidental double-runs exit 5/Busy, never 0). This subtask verifies the REST of both files (and
  configuration.md) for any OTHER lock/no-op/exit-code reference that contradicts those qualified claims, and
  edits the ONE contradiction/gap found.

  ⚠️ **RESEARCH OUTCOME (pre-verified): exactly ONE edit — add a Busy(5) row to the how-it-works.md
  "Failure modes and exit codes" table.** A full grep audit (every contract term: run lock / FR52 / no-op /
  nothing to do / exit 0 / exit 5 / Busy / concurrent / decompose / snapshot / SetSnapshot, + the broader
  safety/atomic set + a bare-"exits 0" check) across all three docs shows:
    - docs/how-it-works.md: L155 (the "No-op fast path." subsection) is ALREADY correctly qualified by
      P1.M1.T1.S3 (the reference — do not edit). The "Failure modes and exit codes" table (L165-173) lists
      codes 1/2/3/124 but OMITS 5/Busy — even though the lock section ~8 lines above introduces exit 5, and
      cli.md's exit table (L374) already lists 5. Add a generic Busy(5) row for coherence (low-effort, in the
      lock changeset, explicitly endorsed by the contract). Every other hit is snapshot-mechanism / decompose-
      feature / freeze-enforcement / diff-noise (the "lock files" at L132 = package-lock.json, NOT the run
      lock) / rescue / hook-mode — NONE makes a contention claim, NONE contradicts L155.
    - docs/cli.md: the exit-code table rows 0 (L370) and 5 (L374) are GENERIC and consistent with the
      qualified L379 prose — the contract's "they are generic and should be fine" is CONFIRMED. The "no-op"
      hits at L50 (FR-R6 reasoning) and L113/121 (FR-H4 hook) are DIFFERENT concepts, not the lock no-op fast
      path. NO edit.
    - docs/configuration.md: the "Lock file location" section (L233-247) is pure location documentation (no
      behavior/exit claim) and AGREES with how-it-works.md L153 on the resolution order. NO edit.
  See research/docs-sweep-audit.md §1-§4 (the full hit-by-hit tables) — the single most important read.

  ⚠️ **THE task is a VERIFICATION + ONE small edit.** Because the docs could drift between this research and
  implementation, the implementer MUST re-run the audit grep against the live files at implementation time.
  If how-it-works.md's failure-modes table still lacks a Busy row (expected), make the §2 edit. Re-confirm
  cli.md and configuration.md are still no-ops. Do NOT invent additional edits where no contradiction exists
  (scattering the contention claim is the anti-pattern this sweep prevents).

  ⚠️ **DO NOT re-state the single-commit/decompose split in the Busy table row.** The path-specific 0-vs-5
  detail lives in ONE authoritative place per file (how-it-works.md:155, cli.md:379). The new table row is a
  GENERIC "another run holds the lock; wait and re-run" row that POINTS to the lock section — it must NOT
  duplicate the split (that would re-create the scattered-claim problem).

  ⚠️ **DO NOT conflate the diff-noise "lock files" (how-it-works.md:132) with the run lock.** L132's "lock
  files" = package-lock.json/yarn.lock/etc. (the FR3 diff-payload exclusion denylist), NOT the FR52 per-repo
  run lock. Leave L132 untouched. (research §1.)

  ⚠️ **DO NOT conflate the "no-op" hits in cli.md (L50/L113/L121) with the lock no-op fast path.** L50 =
  reasoning effort is a graceful no-op for providers without a thinking flag (FR-R6); L113/L121 = the hook
  passes through when a message source is present (FR-H4). Both are unrelated to the lock. Leave them.

  ⚠️ **Scope: docs/how-it-works.md (ONE edit) + cli.md/configuration.md (no-op confirmations).** Do NOT edit
  README.md (sibling sweep P1.M3.T1.S1), PRD.md (read-only), any `.go` file, or any other doc (providers.md,
  docs/README.md).

  Deliverable: (a) ONE in-place edit to docs/how-it-works.md (add the Busy(5) row to the failure-modes
  table); (b) dated no-op confirmations for docs/cli.md and docs/configuration.md appended to
  research/docs-sweep-audit.md. OUTPUT: docs/how-it-works.md and docs/cli.md coherent with the qualified
  claims (or confirmation no further edit needed). DOCS: this IS the changeset-level doc sweep for the docs/
  overview files (Mode B).

---

## Goal

**Feature Goal**: Verify docs/how-it-works.md, docs/cli.md, and docs/configuration.md are cross-cuttingly
coherent with the P1.M1.T1.S2/S3 qualification of the FR52 run-lock contention behavior (how-it-works.md:155
and cli.md:379): no other section re-introduces an unconditional "exits 0 on double-run", and no section
contradicts the single-commit→0/5 / decompose→5(Busy) split. Close the one coherence gap found: the
how-it-works.md "Failure modes and exit codes" table omits the Busy(5) code that the lock section above it
introduces.

**Deliverable**:
1. **RE-RUN** the audit grep (research §1/§3/§4) against the live docs.
2. **docs/how-it-works.md**: add ONE generic Busy(5) row to the "Failure modes and exit codes" table
   (after the "Nothing to commit" row, before "General error") — pointing to the lock section, NOT re-stating
   the split.
3. **docs/cli.md**: NO edit — re-confirm the exit-code table rows 0/5 are generic + consistent with L379.
4. **docs/configuration.md**: NO edit — re-confirm the lock-file-location section makes no behavior claim.
5. **APPEND** a dated re-verification line to `research/docs-sweep-audit.md` (the edit made + the no-ops
   confirmed) — this IS the Mode-B sweep record.

**Success Definition**: the audit grep confirms no non-L155/L379 contention discussion exists in either doc;
the how-it-works.md failure-modes table now includes a generic Busy(5) row consistent with L155; cli.md's
exit table (0/5) and configuration.md's lock-location are confirmed no-ops; PRD.md, README.md, and all `.go`
files byte-unchanged; `go test ./...` still green (no code touched).

## User Persona

**Target User**: A stagehand user reading the docs architecture overview (how-it-works.md) or CLI reference
(cli.md) who must form a correct mental model of the run-lock contention behavior (single-commit double-runs
can no-op exit 0; decompose double-runs exit Busy/5) and of the exit codes — and must not be confused by a
failure-modes table that silently omits the Busy(5) code discussed two paragraphs above it.

**Use Case**: A user scans how-it-works.md's "Failure modes and exit codes" table to see what exit 5 means;
they find the Busy row and follow the pointer to the lock section for the path-specific detail. A user reads
cli.md's exit-code table and the qualified L379 prose; both agree.

**User Journey**: (doc-only; no runtime) reader scans the failure-modes table → finds Busy(5) → follows the
link to the "Per-repo run lock" section → reads the qualified L155 split. No contradiction, no omission.

**Pain Points Addressed**: The how-it-works.md failure-modes table is the one place a reader looks up "exit
5" and currently finds nothing — even though exit 5 is a real, documented-by-the-lock-section code. Adding
the row closes the gap; the sweep confirms no other doc section contradicts the qualified claims.

## Why

- **Closes out the Issue-1 documentation fix across the docs/ overview files.** P1.M1.T1.S2/S3 qualified the
  two contention claims (cli.md:379, how-it-works.md:155); a cross-cutting sweep is the Mode-B doc-sync step
  that ensures the qualification isn't undercut by a stray redundant/contradictory claim or a glaring table
  omission elsewhere. (Bug-Fix PRD §h2.4 "Areas needing attention".)
- **Prevents the scattered-claim anti-pattern.** The path-specific contention split (single-commit→0/5;
  decompose→5) must live in ONE authoritative place per file (L155, L379). The new Busy table row is GENERIC
  + a pointer — it does NOT re-scatter the split. (Mirrors the README sweep's anti-scatter principle.)
- **Cheap and safe.** One markdown row + two confirmations. Zero code change, zero behavioral risk.

## What

A verification sweep of docs/how-it-works.md + docs/cli.md (+ docs/configuration.md confirmation) against
the qualified L155/L379 claims, plus ONE in-place markdown edit (the Busy row). No code, no config, no API,
no behavioral change. docs/how-it-works.md is the only file edited; the audit note is appended either way.

### Success Criteria

- [ ] The audit grep re-run against the live docs: every contract term accounted for; the lock/no-op/exit-0-5
      discussion is confirmed singleton at how-it-works.md:155 and cli.md:379 (both already qualified).
- [ ] docs/how-it-works.md: the "Failure modes and exit codes" table gains a generic Busy(5) row that points
      to the lock section and does NOT re-state the single-commit/decompose split.
- [ ] docs/cli.md: NO edit — exit-code table rows 0 (L370) and 5 (L374) confirmed generic + consistent with
      L379; the "no-op" hits at L50/L113/L121 confirmed as FR-R6/FR-H4 (not the lock).
- [ ] docs/configuration.md: NO edit — lock-file-location (L233-247) confirmed as pure location doc (no
      behavior/exit claim), agreeing with how-it-works.md L153.
- [ ] `research/docs-sweep-audit.md` is appended with a dated re-verification line (the edit + the no-ops).
- [ ] PRD.md, README.md, docs/cli.md, docs/configuration.md, and all `.go` files byte-unchanged (only
      docs/how-it-works.md is edited).

## All Needed Context

### Context Completeness Check

_Pass._ A writer with no prior repo knowledge can do this from: the pre-run audit tables (research §1-§4 —
every hit pre-classified), the authoritative qualified-claim quotes (research §1 L155, §3 L379), the exact
Busy-row markdown edit (Blueprint §1), the re-verification grep commands, and the edit/no-op decision rule.
No code/git/lock-internals knowledge required — this is a prose-coherence check + one table row.

### Documentation & References

```yaml
# MUST READ — the pre-run audit (the whole task is confirming/extending it)
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M3T1S2/research/docs-sweep-audit.md
  why: §1 (how-it-works.md hit-by-hit + the Busy-row decision §2), §3 (cli.md hit-by-hit — exit-table 0/5
       generic, "no-op" hits are FR-R6/FR-H4), §4 (configuration.md — pure location doc), §5 (scope/non-
       conflict), §6 (re-verification requirement).
  critical: §1/§2 (the ONE edit + why L132's "lock files" is diff-noise not the run lock), §3 (why cli.md is
       a no-op — do NOT conflate FR-R6/FR-H4 "no-op" with the lock), §4 (configuration.md is a no-op).

# The file under edit — READ the failure-modes table (L163-174) + L155 before editing
- file: docs/how-it-works.md
  section: L144-161 — the "Per-repo run lock (FR52)" section; L155 is the AUTHORITATIVE qualified claim
           (single-commit→0; decompose→5/Busy) — the reference, NOT a target.
  section: L163-174 — the "Failure modes and exit codes" table; the target of the ONE edit (add a Busy row).
  section: L132 — "lock files" in the binary-exclusion sentence = package-lock.json/yarn.lock (FR3 diff
           noise), NOT the run lock — leave it.
  why: confirms the table currently omits code 5 and that L155 is already qualified.
  critical: edit ONLY the table (add the row). Do NOT edit L155 (already correct). Do NOT touch L132.

# The file under confirmation (NO edit)
- file: docs/cli.md
  section: L366-375 — the exit-code TABLE (rows 0/1/2/3/5/124); rows 0 and 5 are generic, consistent with L379.
  section: L379 — the AUTHORITATIVE qualified contention prose (P1.M1.T1.S2) — the reference.
  section: L50 (FR-R6 reasoning "no-op"), L113/L121 (FR-H4 hook "no-op") — DIFFERENT concepts, not the lock.
  why: confirms no edit is needed (exit table 0/5 generic; "no-op" hits are reasoning/hook).
  critical: do NOT edit cli.md. The exit table already lists code 5; the prose at L379 is already qualified.

# The file under confirmation (NO edit)
- file: docs/configuration.md
  section: L233-247 — "Lock file location" (XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache; sha256 canonical path).
  why: confirms it is pure location documentation (no behavior/exit claim); AGREES with how-it-works.md L153.
  critical: do NOT edit configuration.md.

# The bug context (already in your context as selected_prd_content)
- file: plan/006_…/bugfix/001_624c8013d3b2/prd_snapshot.md (Bug-Fix PRD)
  section: "Issue 1" (h3.0/h2.2) — the original unconditional "exits 0" claim + the qualified fix
           (single-commit→0/5; decompose→5/Busy).
  critical: match the qualification exactly; the new Busy row is generic (no split) — consistent by construction.

# The prior fixes this sweep validates
- file: plan/006_…/bugfix/001_624c8013d3b2/P1M1T1S2/PRP.md   (qualified cli.md:379)
- file: plan/006_…/bugfix/001_624c8013d3b2/P1M1T1S3/PRP.md   (qualified how-it-works.md:155)
  why: read the exact qualified wording so the new Busy row is consistent (generic, points to the qualified prose).

# The sibling sweep (do NOT duplicate)
- file: plan/006_…/bugfix/001_624c8013d3b2/P1M3T1S1/PRP.md   (README.md sweep — singleton-at-L330)
  why: P1.M3.T1.S1 sweeps README.md ONLY. This task (S2) is how-it-works.md (+ cli/configuration
       confirmations) — do NOT touch README.md here.
```

### Current Codebase tree (relevant slice)

```bash
docs/
  how-it-works.md        # THE file under edit (add ONE Busy row to the L163-174 failure-modes table)
  cli.md                 # confirmation pass (NO edit) — exit table 0/5 generic, L379 qualified
  configuration.md       # confirmation pass (NO edit) — lock-file-location is pure location doc
  providers.md           # UNCHANGED (not in scope)
  README.md              # UNCHANGED (sibling sweep P1.M3.T1.S1)
plan/006_…/P1M3T1.S2/
  PRP.md                 # THIS file
  research/docs-sweep-audit.md   # the audit note — APPEND the re-verification result here
# PRD.md, README.md, all .go files — UNCHANGED.
```

### Desired Codebase tree with files to be added

```bash
# NO new source/doc files. At most ONE in-place edit to docs/how-it-works.md (the Busy row).
# research/docs-sweep-audit.md is APPENDED (a dated re-verification line) — it already exists.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the ONE edit is the Busy table row): docs/how-it-works.md's "Failure modes and exit codes"
     table (L163-174) omits code 5/Busy. Add a GENERIC row that POINTS to the lock section — do NOT re-state
     the single-commit/decompose split (that's the scatter anti-pattern; the split lives at L155). -->

<!-- CRITICAL (do NOT edit L155 or L379): those are the AUTHORITATIVE qualified claims (P1.M1.T1.S3 / S2).
     They are the references you check OTHER lines against, not targets. -->

<!-- CRITICAL (do NOT conflate the diff-noise "lock files" with the run lock): docs/how-it-works.md:132
     "Binary files, lock files, snapshots, sourcemaps …" = package-lock.json/yarn.lock (FR3 diff-payload
     exclusion), NOT the FR52 per-repo run lock. Leave L132 untouched. (research §1.) -->

<!-- CRITICAL (do NOT conflate cli.md's "no-op" hits with the lock): cli.md L50 = FR-R6 reasoning no-op;
     L113/L121 = FR-H4 hook pass-through no-op. Neither is the lock no-op fast path. Leave them. -->

<!-- GOTCHA (cli.md exit table is already complete): cli.md L374 ALREADY lists `5 → Busy`. Do NOT add a
     duplicate; the table is fine. The gap is ONLY in how-it-works.md's table. -->

<!-- GOTCHA (configuration.md agrees with how-it-works.md): both give the lock-file resolution order as
     XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache. No discrepancy; no edit. -->

<!-- GOTCHA (scope boundary): docs/how-it-works.md (edit) + cli.md/configuration.md (no-op confirmations)
     ONLY. Do NOT edit README.md (sibling P1.M3.T1.S1), PRD.md (read-only), any .go file, or other docs. -->

<!-- GOTCHA (no conflict with parallel work): P1.M3.T1.S1 is README.md ONLY; P1.M2.* are .go ONLY. This task
     is how-it-works.md (+ confirmations) — disjoint files. -->
```

## Implementation Blueprint

### Data models and structure

No code. The "implementation" is a re-runnable grep audit + ONE markdown table-row edit + two no-op
confirmations.

### §1. THE edit — add the Busy(5) row to docs/how-it-works.md

The "Failure modes and exit codes" table currently reads (L165-174):

```markdown
| Failure | Exit code | Recovery |
|---------|-----------|----------|
| Agent missing on `$PATH` | 1 (Error) | Check the `[provider.<name>] command` path; install the agent |
| Unresolved merge conflicts in the index | 1 (Error) | Resolve the conflicts, then re-run `stagehand` (caught before the snapshot) |
| Generation failed (parse/retry exhaustion) | 3 (Rescue) | Rescue message with tree SHA |
| Generation timed out | 124 (Timeout) | Rescue message with tree SHA |
| CAS failure (HEAD moved meanwhile) | 1 (Error) | HEAD-moved message |
| Nothing to commit (clean tree) | 2 (NothingToCommit) | Stage files and retry |
| General error | 1 (Error) | Inspect error message |
```

**Insert ONE row after the "Nothing to commit (clean tree)" row and before the "General error" row** (the
exact oldText → newText for a precise `edit`):

```markdown
| Nothing to commit (clean tree) | 2 (NothingToCommit) | Stage files and retry |
| Another stagehand run holds the per-repo lock | 5 (Busy) | Wait for the in-progress run to finish, then re-run (see [Per-repo run lock](#per-repo-run-lock-fr52)) |
| General error | 1 (Error) | Inspect error message |
```

**Why this wording:**
- GENERIC — "Another stagehand run holds the per-repo lock" matches cli.md L374 ("Busy — another stagehand
  run holds the per-repo lock; retry after it finishes") exactly in spirit; no path-specific claim.
- POINTS to the qualified prose — the link `#per-repo-run-lock-fr52` (the anchor for the "### Per-repo run
  lock (FR52)" heading at L144) takes the reader to L155's authoritative split. This keeps ONE source of
  truth for the 0-vs-5 detail (anti-scatter).
- Consistent with (does not contradict) the qualified L155 claim — a generic "exit 5 = busy, retry" row is
  true for BOTH paths (single-commit new-work AND decompose), so it cannot contradict.

### The audit grep (re-run + classify)

```bash
# Contention terms — across the three docs. Every hit ≠ how-it-works.md:155 / cli.md:379 must NOT be a
# CONTENTION_DISCUSSION (expected: all are feature/snapshot/diff-noise/hook/rescue/location).
grep -niE 'run.?lock|FR52|no-op|nothing to do|exit (0|5)|\bBusy\b|concurrent|double.?run|decompos|snapshot|SetSnapshot' \
  docs/how-it-works.md docs/cli.md docs/configuration.md
# Bare-"exits 0" check — every hit MUST be inside the qualified clause (how-it-works L155 / cli L379).
grep -niE 'exits? `?0`?|will exit 0|it exits 0' docs/how-it-works.md docs/cli.md
# Expected: contention terms hit how-it-works L144-161 (lock section, L155 qualified) + L163-174 (table, the
# edit target) + cli L379 (qualified) + cli L50/L113/L121 (FR-R6/FR-H4 "no-op", unrelated); bare-"exits 0"
# hits ONLY inside L155's qualified single-commit clause.
```

### Decision matrix (per grep hit outside the qualified lines)

```yaml
# For EACH hit (other than how-it-works.md:155 and cli.md:379), classify and act:
CONTENTION_DISCUSSION:  # mentions the run lock / no-op fast path / exit 0-or-5 / double-run BEHAVIOR
  - how-it-works.md failure-modes table (L163-174) missing code 5 ⇒ the ONE edit (§1).
  - any OTHER contention hit outside the qualified lines ⇒ would contradict; edit to match L155/L379
    (single-commit→0/5; decompose→5). [NONE found in audit — flag if a doc drifted.]
FEATURE_DESCRIPTION:    # decompose as a feature/pipeline (how-it-works L49/53/67/74/115; cli L14-18/30-36)
  ⇒ NO edit (no contention claim).
SNAPSHOT_MECHANISM:     # the write-tree snapshot / invariants / stage-while-generating (how-it-works L3-45/103-107)
  ⇒ NO edit.
FREEZE_ENFORCEMENT:     # "concurrent" = T_start content-subset check, FR-M1c (how-it-works L113/126)
  ⇒ NO edit (NOT the run lock).
DIFF_NOISE:             # "lock files" = package-lock.json/yarn.lock, FR3 (how-it-works L132/138/140)
  ⇒ NO edit (NOT the run lock).
REASONING_NOOP / HOOK_NOOP:  # cli L50 (FR-R6) / L113/L121 (FR-H4) "no-op"
  ⇒ NO edit (NOT the lock no-op fast path).
RESCUE:                 # exit 3/124, dry-run→1 (how-it-works L181-198)
  ⇒ NO edit.
UNRELATED_EXIT:         # cli L90/L243 exit 0 (hook uninstall / integration decline)
  ⇒ NO edit.
LOCK_LOCATION:          # configuration.md L233-247 (pure location doc); how-it-works L153
  ⇒ NO edit (no behavior/exit claim; the two agree).
expected_outcome: how-it-works.md gets the ONE Busy row; cli.md + configuration.md are no-ops.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: RE-RUN the audit grep against the live docs
  - RUN the contention-terms grep + the bare-"exits 0" grep across how-it-works.md, cli.md, configuration.md.
  - FOR EACH hit ≠ how-it-works.md:155 / cli.md:379, classify via the decision matrix.
  - CAPTURE the hit list + classifications (evidence for the edit/no-op decisions).
  - NOTE: the pre-run audit (research §1/§3/§4) found exactly one gap (the how-it-works.md table). The re-run
      is expected to confirm; flag ANY new contention hit (a doc drifted).

Task 2: docs/how-it-works.md — ADD the Busy(5) row
  - EDIT the "Failure modes and exit codes" table: insert the §1 row after "Nothing to commit (clean tree)",
      before "General error".
  - GOTCHA: the row is GENERIC + links to #per-repo-run-lock-fr52; do NOT re-state the single-commit/decompose
      split. Do NOT edit L155 (the qualified reference). Do NOT touch L132 (diff-noise "lock files").
  - RE-RUN the contention grep on how-it-works.md → confirm the table now shows code 5 and no contradiction.

Task 3: docs/cli.md — NO-OP confirmation
  - CONFIRM the exit-code table rows 0 (L370) and 5 (L374) are generic + consistent with L379.
  - CONFIRM the "no-op" hits (L50 FR-R6, L113/L121 FR-H4) are unrelated to the lock.
  - MAKE NO EDIT.

Task 4: docs/configuration.md — NO-OP confirmation
  - CONFIRM the "Lock file location" section (L233-247) makes no behavior/exit claim and agrees with
      how-it-works.md L153.
  - MAKE NO EDIT.

Task 5: RECORD the outcome
  - APPEND a dated re-verification line to research/docs-sweep-audit.md, e.g.:
      "Re-verified <DATE>: re-ran the audit grep. docs/how-it-works.md: added the generic Busy(5) row to the
       failure-modes table (points to #per-repo-run-lock-fr52; does not re-state the split). docs/cli.md:
       no-op — exit table 0/5 generic, consistent with L379; 'no-op' hits are FR-R6/FR-H4. docs/configuration.md:
       no-op — lock-file-location is pure location doc, agrees with how-it-works L153. No other contention
       discussion found outside L155/L379."
  - INCLUDE the captured hit list (or a reference) as evidence.

Task 6: VERIFY
  - RE-RUN the audit grep one final time → confirms the how-it-works.md table has code 5 and no non-L155/L379
      contention contradiction.
  - CONFIRM byte-unchanged: PRD.md, README.md, docs/cli.md, docs/configuration.md, all .go files. (Only
      docs/how-it-works.md is edited.)
  - `go build ./... && go test ./...` GREEN (doc-only; no code touched — belt-and-suspenders).
```

### Implementation Patterns & Key Details

```markdown
<!-- THE authoritative claims to match (do NOT edit; check against them):
     docs/how-it-works.md:155 — "No-op fast path." single-commit→0; decompose→5 (Busy).
     docs/cli.md:379 — "Code 5 (Busy) … two behaviors …" single-commit→0/5; decompose→5 (Busy). -->

<!-- THE edit: ONE generic table row. Generic = consistent with BOTH paths (no scatter). Points to L155. -->
| Another stagehand run holds the per-repo lock | 5 (Busy) | Wait for the in-progress run to finish, then re-run (see [Per-repo run lock](#per-repo-run-lock-fr52)) |

<!-- THE no-op is the successful outcome for cli.md and configuration.md. Record it; do not fabricate edits. -->
```

### Integration Points

```yaml
DOCUMENTATION (Mode B): this IS the changeset-level docs/ sweep for the Issue-1 (FR52 lock qualification)
      changeset. Sibling: P1.M3.T1.S1 sweeps README.md. Together they complete the Mode-B doc sync.

CODE: NONE. No .go file is touched. `go test ./...` stays green by construction (no code change).

FROZEN/LEAVE (do NOT edit):
  - PRD.md (read-only, human-owned).
  - README.md (sibling sweep P1.M3.T1.S1).
  - docs/cli.md + docs/configuration.md (this task's NO-OP confirmations).
  - how-it-works.md:155 (the authoritative qualified claim — the reference, not a target).
  - how-it-works.md:132 (diff-noise "lock files" — NOT the run lock).
  - cli.md:50/113/121 (FR-R6/FR-H4 "no-op" — NOT the lock).
  - All .go files; providers.md; docs/README.md.

NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG.
```

## Validation Loop

### Level 1: The audit grep (re-run + classify)

```bash
# Contention terms across the three docs.
grep -niE 'run.?lock|FR52|no-op|nothing to do|exit (0|5)|\bBusy\b|concurrent|double.?run|decompos|snapshot|SetSnapshot' \
  docs/how-it-works.md docs/cli.md docs/configuration.md
# Bare-"exits 0" check.
grep -niE 'exits? `?0`?|will exit 0|it exits 0' docs/how-it-works.md docs/cli.md
# Expected: contention hits at how-it-works L144-161 (L155 qualified) + L163-174 (table, edit target) + cli
# L379 (qualified) + cli L50/L113/L121 (FR-R6/FR-H4, unrelated); bare-"exits 0" only inside L155's clause.
```

### Level 2: The edit + no-op confirmations

```bash
# Confirm the how-it-works.md table now has a code-5 row (after the §1 edit):
grep -nE '\| 5 \(Busy\)' docs/how-it-works.md   # → the new row
# Confirm the row points to the lock section and does NOT re-state the split:
grep -n 'per-repo-run-lock-fr52' docs/how-it-works.md   # → the link in the new row
# Confirm cli.md exit table already lists 5 (NO edit — sanity):
grep -nE '\| `5` \| Busy' docs/cli.md   # → the existing generic row (untouched)
# Expected: how-it-works.md has the new row; cli.md unchanged; no path-specific split in the new row.
```

### Level 3: Byte-unchanged guard (no scope creep)

```bash
git diff --exit-code PRD.md README.md docs/cli.md docs/configuration.md && echo "frozen files UNCHANGED (expected)"
git diff --name-only -- '*.go' | grep -q . && echo "UNEXPECTED .go change" || echo "no .go changed (expected)"
# docs/how-it-works.md: exactly the ONE new table row.
git diff --stat -- docs/how-it-works.md
git diff -- docs/how-it-works.md | grep '^+' | grep -v '^+++'   # → exactly the Busy row
# Confirm the audit note recorded the outcome:
tail -6 plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M3T1S2/research/docs-sweep-audit.md
```

### Level 4: Whole-repo still green (doc-only, but confirm no accidental code touch)

```bash
go build ./...   # Expect clean (no code changed).
go test ./...    # Expect all PASS (doc-only task; belt-and-suspenders). If RED, a code file was edited by
                 # mistake — revert it.
```

## Final Validation Checklist

### Technical Validation
- [ ] The audit grep re-run; every hit classified (the how-it-works.md table gap → the ONE edit; all others
      feature/snapshot/freeze/diff-noise/hook/rescue/location/no-op-unrelated).
- [ ] `go build ./... && go test ./...` GREEN (doc-only; no code touched).
- [ ] PRD.md / README.md / docs/cli.md / docs/configuration.md / all `.go` files byte-unchanged.

### Feature Validation
- [ ] docs/how-it-works.md's "Failure modes and exit codes" table now lists code 5 (Busy) via a generic row
      that points to the lock section and does NOT re-state the single-commit/decompose split.
- [ ] docs/cli.md exit-code table rows 0/5 confirmed generic + consistent with the qualified L379 prose.
- [ ] docs/configuration.md lock-file-location confirmed as pure location doc (no behavior claim).
- [ ] No non-L155/L379 contention discussion contradicts the qualified claims.
- [ ] Outcome recorded in `research/docs-sweep-audit.md` (the edit + the no-op confirmations).

### Code Quality Validation
- [ ] The ONE edit is generic + a pointer (anti-scatter preserved; the split stays singleton at L155/L379).
- [ ] No edit manufactured where no contradiction exists (cli.md/configuration.md are honest no-ops).
- [ ] Scope held to docs/how-it-works.md (+ the audit note); README.md/PRD.md/code/other-docs untouched.

### Documentation
- [ ] `research/docs-sweep-audit.md` carries the dated re-verification line (this IS the Mode-B sweep record).

---

## Anti-Patterns to Avoid

- ❌ **Don't re-state the single-commit/decompose split in the Busy table row.** The split lives in ONE place
      per file (how-it-works.md:155, cli.md:379). The new row is GENERIC + a pointer — re-stating would
      re-create the scattered-claim problem this sweep prevents.
- ❌ **Don't edit L155 or L379.** Those are the AUTHORITATIVE qualified claims (P1.M1.T1.S3/S2) — the
      references, not targets.
- ❌ **Don't conflate the diff-noise "lock files" (how-it-works.md:132) with the run lock.** L132 =
      package-lock.json/yarn.lock (FR3); leave it. (research §1.)
- ❌ **Don't conflate cli.md's "no-op" hits (L50/L113/L121) with the lock no-op fast path.** They are FR-R6
      (reasoning) and FR-H4 (hook) — unrelated; leave them. (research §3.)
- ❌ **Don't edit docs/cli.md or docs/configuration.md.** cli.md's exit table already lists code 5 (L374) and
      its prose is already qualified (L379); configuration.md's lock-location is pure location doc. Both are
      confirmed no-ops.
- ❌ **Don't edit README.md, PRD.md, any `.go` file, or other docs.** README is the sibling sweep
      P1.M3.T1.S1; PRD is read-only; the code changes are P1.M2.* (already done, .go-only).
- ❌ **Don't manufacture edits where no contradiction exists.** The audit found ONE gap (the how-it-works.md
      table); cli.md and configuration.md are honest no-ops — record the confirmations, don't fabricate work.
- ❌ **Don't skip the re-verification.** Docs could drift between research and implementation. Re-run the grep
      at implementation time; record the dated result. (The edit + no-ops must be EARNED by re-verification.)

---

## Confidence Score

**9/10** — This is a verification task whose outcome is pre-determined by a complete grep audit: the lock/
no-op/exit-0-5 contention discussion is singleton at how-it-works.md:155 and cli.md:379 (both already
qualified), and every other relevant hit is a feature/snapshot/freeze/diff-noise/hook/rescue/location/
unrelated-no-op mention that makes NO contention claim. The ONE gap is the how-it-works.md failure-modes
table (L163-174) omitting code 5 — fixed by a single generic Busy row that points to the qualified prose
(anti-scatter preserved). cli.md's exit table already lists 5 (L374); configuration.md is pure location doc.
The implementer re-runs the same grep to confirm (docs could drift) and, finding it as audited, makes the one
edit + records the two no-ops. No code, no behavioral risk. The one residual risk — a doc drifting between
research and implementation — is closed by the mandatory re-verification grep + the edit/no-op decision
matrix. (The -1 vs a pure no-op task reserves for the one small markdown edit landing cleanly inside the
table without disturbing the pipe formatting.)
