# P1.M3.T1.S2 — docs/how-it-works.md + docs/cli.md + docs/configuration.md Sweep Audit

Cross-cutting coherence sweep for the FR52 run-lock qualification (Issue 1) changeset. The QUALIFIED claims
already in place (by the sibling P1.M1.T1 tasks):
- **docs/how-it-works.md:155** — the "**No-op fast path.**" subsection (qualified by P1.M1.T1.S3): single-
  commit path → contender exits 0; **decompose path → exits 5 (Busy)** because the holder publishes a
  working-tree `T_start` a lock-free contender can't reproduce.
- **docs/cli.md:379** — the "Code 5 (Busy) … Contention on the per-repo run lock (FR52) has two behaviors"
  paragraph (qualified by P1.M1.T1.S2): single-commit → 0 (covered) / 5 (new work); **decompose → 5 (Busy)**.

This sweep checks the REST of both files (and docs/configuration.md) for any OTHER lock/no-op/exit-code
reference that contradicts those qualified claims. Terms grepped (contract list): `run lock`, `FR52`,
`no-op`, `nothing to do`, `exit 0`, `exit 5`, `Busy`, `concurrent`, `decompose`, `snapshot`, `SetSnapshot`
(+ the broader safety/atomic set and a bare-"exits 0" check).

Verified at audit time against the live files (HEAD). Conclusion: **ONE edit** (add a Busy(5) row to the
how-it-works.md failure-modes table for completeness); **cli.md and configuration.md need NO edit**
(confirmations below).

---

## §1. docs/how-it-works.md — hit-by-hit

| Line | Text (gist) | Class | Verdict / Action |
|---|---|---|---|
| 3,5,15,19,23,24,26,30,37,40,45 | snapshot-based flow / invariants / stage-while-generating | SNAPSHOT_MECHANISM | NO edit. About the commit snapshot, not the run lock. |
| 49,53,67,74 | decompose FEATURE description + activation + ASCII flow | FEATURE_DESCRIPTION | NO edit. Describes the pipeline/trigger, NO contention/exit claim. |
| 103,105,107 | `--edit` freeze / frozen tree snapshots / tree-to-tree diffs | SNAPSHOT_MECHANISM | NO edit. |
| 113 | "Freeze enforcement … a concurrent change swept in … is a hard abort" | FREEZE_ENFORCEMENT | NO edit. "concurrent" here = the T_start content-subset check (FR-M1c), NOT the FR52 run lock. Different mechanism. |
| 115 | One-file short-circuit (FR-M2b) | FEATURE_DESCRIPTION | NO edit. |
| 121,124,126 | decompose snapshot invariants / T_start / "concurrent edits never enter any commit" | FREEZE_ENFORCEMENT | NO edit. "concurrent" = freeze enforcement, not the run lock. |
| 128 | pointer to configuration.md / cli.md | POINTER | NO edit. |
| 132 | "Binary files, **lock files**, snapshots, sourcemaps, vendor … excluded from every diff payload" | DIFF_NOISE | NO edit. "**lock files**" here = package-lock.json/yarn.lock/etc. (FR3 diff-noise denylist), NOT the FR52 run lock. Different concept — do not conflate. |
| 138,140 | payload-only exclusion guarantee + denylist union | DIFF_NOISE | NO edit. |
| 144 | "### Per-repo run lock (FR52)" | LOCK_SECTION_HEADER | NO edit. |
| 146 | "two-stage defense against concurrent runs" | LOCK_CONTENTION | NO edit. Accurate; no exit claim. |
| 148 | per-repo run lock, auto-release on process death | LOCK_MECHANISM | NO edit. |
| 149 | §13.5 CAS second guarantee | LOCK_MECHANISM | NO edit. |
| 151 | per-host limit | LOCK_MECHANISM | NO edit. |
| 153 | never-in-repo location (XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache) | LOCK_LOCATION | NO edit. AGREES with configuration.md L238-240. |
| **155** | **"No-op fast path."** single-commit→0 / decompose→5(Busy) | **QUALIFIED_CLAIM** | **NO edit (already qualified by P1.M1.T1.S3). This is the reference.** |
| 157 | Auto-release (flock; Windows no-op stub) | LOCK_MECHANISM | NO edit. |
| 163-174 | **"Failure modes and exit codes" TABLE** — rows: Agent missing(1), Merge conflicts(1), Generation failed(3), Timed out(124), CAS failure(1), Nothing to commit(2), General error(1) | **TABLE_GAP** | **EDIT: add a Busy(5) row.** The table omits code 5 even though the lock section 8 lines above introduces it, and cli.md's exit table (L374) already lists 5. See §2. |
| 181,186,189,198 | rescue protocol (exit 3/124; dry-run → 1) | RESCUE | NO edit. Correct, unrelated to the lock. |
| 230 | user payload + rejection list | PAYLOAD | NO edit. |
| 241-264 | hook mode vs snapshot flow | HOOK_MODE | NO edit. No lock claim. |

**how-it-works.md OUTCOME: exactly ONE edit (the Busy row in the failure-modes table).** No other hit is a
lock-contention discussion, and no hit contradicts the qualified L155 claim.

### §2. The how-it-works.md Busy(5) row — decision: ADD IT

**Decision: add a generic Busy(5) row to the failure-modes table.** Rationale:
1. **Coherence:** the table is titled "Failure modes and exit codes" and sits ~8 lines under the lock section
   that introduces exit 5 (Busy). A reader scanning the table for "exit 5" currently finds nothing.
2. **Parity:** cli.md's exit-code table (L374) already lists `5 → Busy`. how-it-works.md's table is the
   outlier.
3. **In-scope + low-effort:** the contract explicitly offers this ("decide whether to add a Busy row for
   completeness (low-effort, improves coherence) … do NOT widen scope beyond the lock changeset"). A Busy row
   IS the lock changeset (it documents the lock's exit code). Endorsed.
4. **No scatter:** the row is GENERIC ("another run holds the lock; wait and re-run") and POINTS to the lock
   section for the path-specific detail — it does NOT re-state the single-commit/decompose split. That keeps
   one authoritative place for the split (L155), matching the anti-scatter principle from the README sweep.

**Exact row to insert** (placed after the "Nothing to commit (clean tree) | 2" row, before "General error"):
```
| Another stagecoach run holds the per-repo lock | 5 (Busy) | Wait for the in-progress run to finish, then re-run (see [Per-repo run lock](#per-repo-run-lock-fr52)) |
```
Consistency check: matches cli.md L374 ("Busy — another stagecoach run holds the per-repo lock; retry after it
finishes") and is consistent with (does not contradict) the qualified L155 claim. The path-specific 0-vs-5
detail stays at L155 + cli.md:379 (single source of truth); the row is the table-index pointer.

---

## §3. docs/cli.md — hit-by-hit

| Line | Text (gist) | Class | Verdict / Action |
|---|---|---|---|
| 3 | reference for the command/flags/exit codes | META | NO edit. |
| 14-18 | default-action routing (staged→single; nothing-staged-dirty→decompose; clean→exit 2; --single/--dry-run) | PATH_SELECTION | NO edit. Path selection, not contention. |
| 30-36 | flags table (--all/--no-auto-stage/--dry-run/--commits/--single/--no-decompose/--max-commits) | FLAGS | NO edit. No contention claim. |
| 42 | `--edit` | FLAG | NO edit. |
| 50 | `--reasoning` "graceful **no-op** (FR-R6)" | REASONING_NOOP | NO edit. "no-op" = reasoning effort is a no-op for providers without a thinking flag (FR-R6), NOT the lock no-op fast path. Different concept. |
| 90 | `hook uninstall` → "(exit 0)" | HOOK_UNINSTALL | NO edit. Unrelated exit 0. |
| 111-133 | `hook exec` — "Source-gated **no-op** (FR-H4)" / "message … → no-op (exit 0)" | HOOK_NOOP | NO edit. "no-op" = the hook passes through when a message source is present (FR-H4), NOT the lock. Different concept. |
| 243 | integration decline/no-change → "exit 0" | INTEGRATION | NO edit. Unrelated exit 0. |
| **366-375** | **Exit-code TABLE** — `0` Success / `1` General error / `2` Nothing to commit / `3` Rescue / `5` Busy / `124` Timeout | **EXIT_TABLE** | **NO edit.** Rows 0 and 5 are GENERIC and consistent with the qualified L379 prose. The contract: "they are generic and should be fine" — CONFIRMED. (Row 5 = "Busy — another stagecoach run holds the per-repo lock; retry after it finishes." Row 0 = "Success (commit created, or dry-run message printed)." Neither makes a path-specific contention claim.) |
| 377 | exit codes mirror constants; timeout=124; dry-run → 1 | EXIT_META | NO edit. |
| **379** | **"Code 5 (Busy) … two behaviors"** — single-commit→0/5, decompose→5(Busy) | **QUALIFIED_CLAIM** | **NO edit (already qualified by P1.M1.T1.S2). This is the reference.** |

**cli.md OUTCOME: NO edit.** The exit-code table rows 0 and 5 are generic and consistent with the qualified
L379 contention prose. No other lock/no-op/exit hit contradicts the qualified claim. (The "no-op" hits at
L50/L113/L121 are FR-R6 reasoning and FR-H4 hook concepts, NOT the lock — explicitly distinct.)

---

## §4. docs/configuration.md — hit-by-hit

| Line | Text (gist) | Class | Verdict / Action |
|---|---|---|---|
| 233-247 | "## Lock file location" — XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache resolution; sha256 of canonical path; relative XDG ignored; never falls back to CWD/repo | LOCK_LOCATION | **NO edit.** Makes NO behavior claim about contention/no-op/exit codes — purely WHERE the lock file lives. The contract: "it makes no behavior claim and should need no edit, but confirm" — CONFIRMED. |
| (cross-check) | L238-240 resolution order | — | AGREES with how-it-works.md L153 (same XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache order). No discrepancy. |

**configuration.md OUTCOME: NO edit.** Pure location documentation; no contention/exit claim to contradict.

---

## §5. Scope & non-conflict

- **This task (S2) edits:** docs/how-it-works.md (ONE row) ONLY. docs/cli.md and docs/configuration.md are
  NO-OP confirmations (recorded here, no file edit).
- **Frozen / do NOT edit:** README.md (sibling sweep P1.M3.T1.S1 — singleton-at-L330, its own no-op/edit),
  PRD.md (read-only), all `.go` files, every other doc (providers.md, docs/README.md).
- **No conflict with P1.M3.T1.S1:** that task is README.md ONLY. This task is how-it-works.md (+ cli/
  configuration confirmations). Disjoint files.
- **No conflict with P1.M2.* (the lock-hardening code tasks):** those are `.go`-only; this is markdown-only.

---

## §6. Re-verification requirement

The docs COULD drift between this research and implementation. The implementer MUST re-run the §1/§3/§4 grep
at implementation time:
- If how-it-works.md's failure-modes table still lacks a Busy row → make the §2 edit.
- If cli.md's exit-code table rows 0/5 are still generic + L379 still qualified → record the no-op.
- If configuration.md's lock-location section still makes no behavior claim → record the no-op.
- Flag ANY new contention hit (a doc drifted) and classify it before acting.

Record the dated re-verification result (edit made + no-ops confirmed) in this file (append a line) — this IS
the Mode-B sweep record.

---

## Summary

| File | Outcome | Action |
|---|---|---|
| docs/how-it-works.md | ONE edit | Add a generic Busy(5) row to the "Failure modes and exit codes" table (§2). L155 already qualified — leave it. |
| docs/cli.md | NO edit (confirmation) | Exit-code table rows 0/5 generic + consistent with L379; "no-op" hits are FR-R6/FR-H4, not the lock. |
| docs/configuration.md | NO edit (confirmation) | Lock-file-location is pure location doc; no behavior claim. |

---

## Re-verification (2026-07-04)

Re-verified 2026-07-04: re-ran the §1/§3/§4 audit grep (contention terms + bare-"exits 0") against the
live docs. No drift from the audit-time findings.

- **docs/how-it-works.md: EDIT MADE.** Added the generic Busy(5) row to the "Failure modes and exit codes"
  table (now at L173, between "Nothing to commit (clean tree)" and "General error"): `| Another stagecoach
  run holds the per-repo lock | 5 (Busy) | Wait for the in-progress run to finish, then re-run (see
  [Per-repo run lock](#per-repo-run-lock-fr52)) |`. The row is generic and points to the qualified L155
  prose via the `#per-repo-run-lock-fr52` anchor; it does NOT re-state the single-commit/decompose split
  (anti-scatter preserved). L155 (the authoritative qualified claim) and L132 (the diff-noise "lock files" =
  package-lock.json/yarn.lock, FR3) were left untouched. `git diff --stat` = 1 insertion, 1 file.
- **docs/cli.md: NO edit (no-op confirmed).** Exit-code table rows 0 (L370) and 5 (L374) are generic and
  consistent with the qualified L379 contention prose; the "no-op" hits at L50 (FR-R6 reasoning), L113/L121
  (FR-H4 hook pass-through) are unrelated to the lock no-op fast path. No contradiction found.
- **docs/configuration.md: NO edit (no-op confirmed).** The "Lock file location" section (L233-247) is pure
  location documentation (XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache; sha256 of canonical path) and makes
  no behavior/exit claim; it agrees with how-it-works.md L153.

No other contention discussion found outside how-it-works.md:155 and cli.md:379. The bare-"exits 0" hits
(how-it-works.md L60 stager tool contract / L258 hook never-block; cli.md L86/L90 hook uninstall / L113/L115
FR-H4 / L133 `--strict` / L243 integration decline / L433 dry-run preview) are all unrelated to the run
lock. Byte-unchanged guard: PRD.md, README.md, docs/cli.md, docs/configuration.md, and all `.go` files
unchanged; only docs/how-it-works.md edited (1 row). `go build ./... && go test ./...` GREEN (doc-only).
