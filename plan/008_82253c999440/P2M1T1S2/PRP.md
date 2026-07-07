---
name: "P2.M1.T1.S2 — FR-M3b deterministic non-fatal planner coverage check (logs unclaimed paths)"
description: |

  Implement PRD §9.14 FR-M3b: after the planner returns a multi-concept partition, union all `concept.Files`
  and compare against the frozen changed-path set `DiffTreeNames(baseTree, tStart)`; log (verbose) each path
  the planner left unclaimed as a likely arbiter leftover. PURELY DIAGNOSTIC — never aborts, never
  hard-constrains the stager (FR-M1c/`verifyFreezeSubset` in runLoop stays the SOLE content guarantee).

  CONTRACT (P2.M1.T1.S2, verbatim):
    1. RESEARCH NOTE: the check belongs in internal/decompose/decompose.go immediately after callPlanner
       returns successfully (current line ~199) and before the `if out.Single` short-circuit (line ~204).
       The frozen changed-path set is DiffTreeNames(baseTree, tStart) (baseTree/tStart in scope).
       deps.Verbose.VerboseRawOutput is nil-receiver-safe (internal/ui/verbose.go:54).
    2. INPUT: concept.Files from P2.M1.T1.S1 (PlannerCommit.Files), the frozen baseTree/tStart, and
       deps.Git.DiffTreeNames.
    3. LOGIC: Implement FR-M3b. When !out.Single && len(out.Commits) > 0: union all concept.Files into a
       set; compute changed := deps.Git.DiffTreeNames(ctx, baseTree, tStart); for each path in changed not
       in the union, call deps.Verbose.VerboseRawOutput(fmt.Sprintf("decompose: path %q not claimed by any
       concept (likely leftover for the arbiter)", p)). On a DiffTreeNames error, log 'coverage check
       skipped: <err>' and CONTINUE (best-effort, never fail the run). NEVER abort, NEVER hard-constrain the
       stager. Consider extracting a small helper checkPlannerCoverage(deps, baseTree, tStart, concepts) for
       testability. TDD: a test that drives a stub planner JSON whose concepts' files deliberately omit one
       changed path; assert the run SUCCEEDS AND a capturing Verbose writer received a line naming the
       unclaimed path. Mock: stubtest.Manifest stub planner returning canned JSON; real git for DiffTreeNames.
    4. OUTPUT: A diagnostic-only coverage log (no behavior change to staging/loop) for FR-M3b.
    5. DOCS: none — diagnostic-only, no user-facing surface.

  ⚠️ §1 — DIAGNOSTIC-ONLY (load-bearing). FR-M3b MUST NOT abort the run and MUST NOT hard-constrain the
  stager. FR-M1c (verifyFreezeSubset in runLoop) is the SOLE content guarantee. The coverage check is about
  planner PRECISION (did it account for every path?), not correctness; it logs and is forgotten. Do NOT
  return an error, do NOT mutate state, do NOT feed the loop/arbiter.

  ⚠️ §2 — VerboseRawOutput is the contract-mandated sink (NOT VerboseWarn). It prepends "DEBUG: raw
  output:\n" (designed for agent stdout), which is slightly odd for a diagnostic, but the contract is
  explicit. The test asserts the path string appears in the captured buffer regardless of prefix.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `PlannerCommit.Files` field + `internal/prompt/planner*.go` → P2.M1.T1.S1 (in progress). This task
      only READS `concept.Files`.
    - `validatePlannerOutput`, `callPlanner`, `ParsePlannerOutput` → UNCHANGED (coverage ≠ validation).
    - `runLoop`, `verifyFreezeSubset`, the arbiter → UNCHANGED (the check never feeds them).
    - planner/stager/arbiter system prompts, docs/how-it-works.md → P2.M1.T2/T3 + P3. Item §5: NO DOCS.
    - internal/git/* → UNCHANGED (DiffTreeNames already exists).

  DELIVERABLES (0 NEW files, 2 EDITED files):
    EDIT internal/decompose/decompose.go       — add the `checkPlannerCoverage` helper + the 4-line guarded
                                                 call between callPlanner success and `if out.Single`.
    EDIT internal/decompose/decompose_test.go  — ADD a stub-planner-driven test (real git + capturing
                                                 Verbose) asserting the run succeeds AND the unclaimed path
                                                 is logged.

  SUCCESS: a multi-concept planner partition that omits a changed path produces a verbose line naming it
  (`decompose: path "c.txt" not claimed by any concept (likely leftover for the arbiter)`); the run still
  SUCCEEDS (no error); a DiffTreeNames failure logs `coverage check skipped: <err>` and CONTINUES; FR-M1c /
  the loop / the arbiter are byte-unchanged; `go build/vet/test ./...` green; go.mod/go.sum unchanged; the 2
  files above are the ONLY changes.

---

## Goal

**Feature Goal**: Implement PRD §9.14 FR-M3b — a deterministic, NON-FATAL coverage check that runs
immediately after the planner returns a multi-concept partition. It unions every `concept.Files` the
planner declared and compares against the frozen changed-path set `DiffTreeNames(baseTree, tStart)`; any
frozen path the planner left unclaimed is logged (verbose) as a likely arbiter leftover. The check is
diagnostic only: it never aborts the run and never hard-constrains the stager (FR-M1c freeze-subset
verification stays the sole content guarantee). Its purpose is planner PRECISION visibility, not correctness.

**Deliverable** (0 NEW + 2 EDITED):
1. **EDIT `internal/decompose/decompose.go`** — add a `checkPlannerCoverage(ctx, deps, baseTree, tStart,
   concepts)` helper (void, best-effort, nil-safe verbose) + a 4-line guarded call inserted between the
   `callPlanner` err-check and the `if out.Single` short-circuit.
2. **EDIT `internal/decompose/decompose_test.go`** — ADD a full-`Decompose` test using a `stubtest`
   planner (canned JSON whose concepts omit one changed path) + real git + a capturing `lockedBuffer`
   verbose writer; assert the run SUCCEEDS and the buffer contains the unclaimed-path line.

**Success Definition**:
- A planner partition of 2 concepts claiming `[a.txt]` + `[b.txt]` over a 3-file frozen change set
  `[a.txt, b.txt, c.txt]` ⇒ `Decompose` returns NO error and the captured verbose buffer contains
  ``decompose: path "c.txt" not claimed by any concept (likely leftover for the arbiter)``.
- If `DiffTreeNames` fails, the helper logs ``coverage check skipped: <err>`` and RETURNS (the run is
  unaffected — no error propagated).
- The check is SKIPPED when `out.Single` (single-shortcut path) or `len(out.Commits) == 0`.
- `runLoop`, `verifyFreezeSubset`, the arbiter, `callPlanner`, `validatePlannerOutput`, and all prompts/docs
  are byte-unchanged.
- `go build ./... && go vet ./... && go test ./...` GREEN; go.mod/go.sum unchanged; EXACTLY 2 files change.

## User Persona

**Target User**: a developer running `stagecoach --verbose` (or a library consumer threading a `*ui.Verbose`
writer) who wants to see, at a glance, whether the planner accounted for every changed path — so an
unexpected arbiter commit ("why did a leftover get its own commit?") is explainable after the fact.

**Use Case**: the planner partitions a 5-file changeset into 2 concepts but silently drops one path from
both `files` lists. Today that path surfaces only as a surprise arbiter commit. After S2, `--verbose`
emits a `decompose: path "x.go" not claimed by any concept (likely leftover for the arbiter)` line the
instant the planner returns, making the drop visible BEFORE the loop runs.

**Pain Points Addressed**: today a planner that omits a path from its `files` lists produces an opaque
arbiter commit with no early signal. The coverage check gives a deterministic, pre-loop diagnostic.

## Why

- **Closes PRD §9.14 FR-M3b at the orchestrator layer.** The check is the deterministic counterweight to
  the planner's model-judgment `files`: it catches omissions the planner makes, cheaply and deterministically.
- **Precision, not correctness.** `files`' real job is guiding each concept's stager (FR-M5). The coverage
  check surfaces omissions so the user understands the arbiter's later leftover commit — it does NOT replace
  FR-M1c (the freeze-subset hard guarantee) and does NOT feed the loop.
- **Non-fatal by design.** A diagnostic that could abort the run would duplicate/violate FR-M1c and change
  the failure semantics. The helper is void + best-effort: a `DiffTreeNames` error is logged and swallowed.
- **Low-risk, surgical.** One small helper + a 4-line guarded call + one test. No new types, no interface
  change, no import change, no behavior change to staging/the loop/the arbiter.

## What

A diagnostic-only post-planner check: union `concept.Files`, diff against `DiffTreeNames(baseTree, tStart)`,
log unclaimed paths via `VerboseRawOutput`. No structural changes, no state mutation, no control-flow change
beyond the 4-line guarded call.

### Success Criteria

- [ ] `checkPlannerCoverage(ctx, deps, baseTree, tStart, concepts []prompt.PlannerCommit)` exists in
      `internal/decompose/decompose.go`, is VOID (best-effort), and is nil-receiver-safe on `deps.Verbose`.
- [ ] The helper: unions `concept.Files` into a set; calls `DiffTreeNames(ctx, baseTree, tStart)`; on error
      logs ``coverage check skipped: <err>`` via `VerboseRawOutput` and returns; for each changed path NOT in
      the union, logs ``decompose: path "<p>" not claimed by any concept (likely leftover for the arbiter)``.
- [ ] `Decompose` calls the helper between the `callPlanner` err-check and `if out.Single`, guarded by
      `!out.Single && len(out.Commits) > 0`.
- [ ] The run still SUCCEEDS when the planner omits a path (no error); the captured verbose buffer names it.
- [ ] `validatePlannerOutput`, `callPlanner`, `runLoop`, `verifyFreezeSubset`, the arbiter, all prompts, and
      docs are UNCHANGED. `PlannerCommit.Files` (S1) is only READ.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; go.mod/go.sum unchanged; EXACTLY 2 files change.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact insertion point
(§1 — between callPlanner's err-check and `if out.Single`), the helper signature + body (§5 — void,
nil-safe, union + DiffTreeNames + log), the verbose sink choice (§4 — contract-mandated VerboseRawOutput),
the data sources (§2 — `concept.Files` from S1 + `DiffTreeNames` clean paths), the excludes non-issue (§3 —
consistent with the arbiter gate), the exact test pattern to copy (§7 — the verbose-capture decompose test
at line ~2395), the M3b-vs-M1c distinction (§9 — diagnostic, never a hard constraint), and the scope fence
(§8). No git internals / provider / prompt-builder knowledge required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE findings (insertion point + helper + test pattern + the M3b-vs-M1c fence)
- docfile: plan/008_82253c999440/P2M1T1S2/research/findings.md
  why: §1 (the EXACT insertion point + the baseTree/tStart scope); §2 (concept.Files from S1 + DiffTreeNames
       clean-path return); §3 (DiffTreeNames does NOT apply excludes — consistent with the arbiter gate;
       use it directly per the contract); §4 (VerboseRawOutput is nil-safe + contract-mandated; the semantic
       note); §5 (the helper signature incl. ctx + the verbatim body); §6 (the 4-line guarded call-site
       edit); §7 (the test scenario + the existing pattern to copy at decompose_test.go:~2395); §8 (scope
       fence); §9 (M3b ≠ M1c — diagnostic, never a hard constraint).
  critical: §1 (insert between callPlanner err-check and `if out.Single`, NOT elsewhere), §5 (VOID helper,
       best-effort, log+return on DiffTreeNames error — NEVER propagate), §9 (do NOT mutate state / feed the
       loop / abort — FR-M1c stays the sole content guarantee).

# MUST READ — S1's PRP (the CONTRACT for PlannerCommit.Files, which this task READS)
- docfile: plan/008_82253c999440/P2M1T1S1/PRP.md
  section: the PlannerCommit.Files field (json:"files"; guidance, not a constraint). S1 ADDS it; S2 CONSUMES
       it (out.Commits[i].Files). Do NOT re-add or redeclare it.
  critical: Files is populated for free by ParsePlannerOutput's generic json.Unmarshal (S1); S2 just reads
       it. Files is GUIDANCE — never validated (FR-M1c is the content guarantee) — S2's check is diagnostic,
       not validation.

# MUST READ — the FILE TO EDIT: Decompose + the insertion point
- file: internal/decompose/decompose.go   (EDIT)
  section: `Decompose` — the `out, err := callPlanner(...)` block + its `if err != nil { return ... }`
       (the insertion point is the blank line BETWEEN that err-check and the `// (4) FR-M11 single-SHORTCUT`
       / `if out.Single {` block). baseTree + tStart are both in scope here.
  why: this is where the 4-line guarded call goes. `out.Commits` is `[]prompt.PlannerCommit`; `out.Commits[i]
       .Files` is the S1 field. `checkPlannerCoverage` (new helper) is also added to this file.
  pattern: mirror the existing diagnostic-log idiom in this file: `deps.Verbose.VerboseRawOutput(fmt.Sprintf(
       "decompose: ...", ...))` (already used ~line 230 for the reread-final-commits best-effort log).
  gotcha: the helper is VOID and the call site does NOT assign/compare its return — it is pure side-effect.
       Guard with `!out.Single && len(out.Commits) > 0`.

# MUST READ — the verbose sink (nil-safe; contract-mandated method)
- file: internal/ui/verbose.go   (READ-ONLY)
  section: `VerboseRawOutput(output string)` (L54 — `if v == nil || v.w == nil || !v.on { return }`; then
       prints "DEBUG: raw output:\n" + output + trailing-newline-ensured).
  why: confirms nil-receiver safety ⇒ the helper needs NO nil guard on deps.Verbose. Confirms the prefix.
  gotcha: VerboseRawOutput prepends "DEBUG: raw output:\n" (it is for agent stdout). VerboseWarn ("DEBUG:
       <msg>") fits a diagnostic better, BUT THE CONTRACT MANDATES VerboseRawOutput — use it verbatim. The
       test matches on the path substring regardless of prefix.

# MUST READ — DiffTreeNames (the frozen changed-path primitive)
- file: internal/git/git.go   (READ-ONLY)
  section: `DiffTreeNames(ctx, treeA, treeB) (paths []string, err error)` (L288 — `git diff-tree -r --name-only
       --no-commit-id`; clean paths NO status prefix; identical trees ⇒ (nil, nil); exit 128 ⇒ wrapped err).
  why: the changed-path set. Clean paths ⇒ directly comparable to concept.Files paths (both are repo-relative).
  gotcha: DiffTreeNames does NOT apply excludes — an excluded-but-changed file (e.g. *.lock) shows as
       unclaimed. This is ACCEPTABLE + consistent with the arbiter gate (which also uses DiffTreeNames
       without excludes). Do NOT apply excludes in the check (the contract mandates DiffTreeNames directly).

# MUST READ — the Deps struct (the helper's deps param)
- file: internal/decompose/roles.go   (READ-ONLY)
  section: `type Deps struct` (L55 — `Git git.Git`, `Verbose *ui.Verbose`, ...). The helper uses deps.Git.
       DiffTreeNames + deps.Verbose.VerboseRawOutput.
  why: confirms the two fields the helper needs are on Deps (so `checkPlannerCoverage(ctx, deps, ...)`).
  gotcha: deps.Verbose may be nil in library use — VerboseRawOutput is nil-safe, so no guard needed.

# MUST READ — the test file to EXTEND + the pattern to copy
- file: internal/decompose/decompose_test.go   (EDIT — ADD a test)
  section: the verbose-capture decompose test at ~L2395-2428 (stubtest.Manifest planner with Out:plannerJSON
       + dcmDeps + dcmStagerSeam + `var lb lockedBuffer; deps.Verbose = ui.NewVerbose(&lb, true)` +
       Decompose + `lb.String()` substring assert). `lockedBuffer` is defined in this file — REUSE it.
  why: this is the exact pattern for "stub planner + real git + capturing verbose". Copy its setup; change
       plannerJSON so the concepts omit one path; assert the run succeeds AND lb.String() contains the
       unclaimed-path line.
  pattern: plannerJSON concepts carry `"files":[...]`; the stager seam stages the CLAIMED files; the omitted
       file flows to the arbiter (arbiter stub `{"target": null}` ⇒ new commit; message script needs a 3rd
       entry). The run succeeds (N loop commits + 1 arbiter); the coverage line is already in the buffer.
  gotcha: reuse `lockedBuffer` + `dcmDeps` + `dcmStagerSeam` + the `piShape`/`tooledStubManifest` helpers —
       do NOT redefine them. The message script must have enough entries for loop + arbiter commits.

# The PRD basis (in your context as selected_prd_content)
- file: PRD.md
  section: §9.14 FR-M3b (h3.30 — "After the planner returns, stagecoach unions the files declared across all
       concepts and compares against the frozen changed-path set (DiffTreeNames(baseTree, T_start)). Any path
       the planner left unclaimed is logged (verbose) as a likely leftover … This is a diagnostic only: it
       never aborts the run and does not hard-constrain the stager (FR-M1c remains the sole content guarantee)");
       §13.6 (h3.66 — the pipeline: planner → loop → arbiter, the freeze surfaces); §17.5 (h3.81 — the planner
       JSON contract with "files").
  critical: FR-M3b's own text pins (a) the EXACT primitive (DiffTreeNames(baseTree, T_start)), (b) the
       DIAGNOSTIC-ONLY semantics (never abort, never constrain), (c) the M3b-vs-M1c distinction.
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go       # EDIT: add checkPlannerCoverage helper + the 4-line guarded call in Decompose.
  decompose_test.go  # EDIT: ADD the stub-planner verbose-capture test (reuse lockedBuffer/dcmDeps/dcmStagerSeam).
  planner.go         # READ-ONLY: callPlanner + validatePlannerOutput (UNCHANGED; coverage ≠ validation).
  roles.go           # READ-ONLY: Deps struct (Git + Verbose fields the helper uses).
  {arbiter,chain,stager,message,roles}_*.go  # UNCHANGED.
internal/prompt/
  planner.go         # READ-ONLY (S1): PlannerCommit.Files — the field this task READS.
internal/git/
  git.go             # READ-ONLY: DiffTreeNames (the frozen changed-path primitive; clean paths).
internal/ui/
  verbose.go         # READ-ONLY: VerboseRawOutput (nil-safe; contract-mandated sink).
internal/stubtest/
  stubtest.go        # READ (test only): Manifest/Options/Env for the stub planner.
go.mod / go.sum      # UNCHANGED (no new import — fmt + prompt already in the decompose package).
```

### Desired Codebase tree with files to be MODIFIED

```bash
internal/decompose/decompose.go       # EDIT. + checkPlannerCoverage(ctx, deps, baseTree, tStart, concepts)
                                      #   helper (void, best-effort, nil-safe verbose) near Decompose;
                                      #   + the 4-line guarded call between callPlanner's err-check and
                                      #   `if out.Single`. NO other change to Decompose / runLoop / arbiter.
internal/decompose/decompose_test.go  # EDIT. + TestDecompose_PlannerCoverageLogsUnclaimed (stub planner
                                      #   JSON omitting one path; real git; capturing lockedBuffer verbose;
                                      #   assert run SUCCEEDS + buffer contains the unclaimed-path line).
# go.mod/go.sum UNCHANGED. validatePlannerOutput/callPlanner/runLoop/arbiter UNCHANGED. prompts/docs UNCHANGED.
# PlannerCommit.Files (S1) only READ. internal/git/*, internal/prompt/*, internal/ui/* UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (§9 — DIAGNOSTIC ONLY, never a hard constraint): FR-M3b MUST NOT abort the run and MUST NOT
// hard-constrain the stager. FR-M1c (verifyFreezeSubset in runLoop) is the SOLE content guarantee. The
// coverage check is void + best-effort: a DiffTreeNames error is logged + swallowed; the loop/arbiter are
// untouched. Do NOT return an error, do NOT mutate state, do NOT feed the loop/arbiter.

// CRITICAL (§4 — VerboseRawOutput is contract-mandated, NOT VerboseWarn): deps.Verbose.VerboseRawOutput is
// nil-receiver-safe (verbose.go:54 — `if v==nil || v.w==nil || !v.on {return}`). It prepends "DEBUG: raw
// output:\n" (designed for agent stdout) — slightly odd for a diagnostic, but the contract is explicit. The
// test matches on the path substring, so the prefix is irrelevant. Use VerboseRawOutput for BOTH the
// unclaimed line and the skip line.

// CRITICAL (§1 — insertion point): the call goes BETWEEN `callPlanner`'s `if err != nil { return ... }` and
// the `// (4) FR-M11 single-SHORTCUT` / `if out.Single {` block. NOT inside callPlanner; NOT after the loop.
// baseTree + tStart are in scope here (no plumbing needed).

// GOTCHA (§5 — helper is VOID): checkPlannerCoverage returns nothing. The call site does NOT assign or
// compare its return — it is pure side-effect (verbose logging). On DiffTreeNames error: log "coverage check
// skipped: <err>" and return; NEVER propagate.

// GOTCHA (§3 — DiffTreeNames does NOT apply excludes): an excluded-but-changed file (e.g. *.lock) is in
// DiffTreeNames but the planner never saw it ⇒ it logs as unclaimed. ACCEPTABLE + consistent with the
// arbiter gate (which also uses DiffTreeNames without excludes). Do NOT apply excludes in the check — the
// contract mandates DiffTreeNames(baseTree, tStart) directly.

// GOTCHA (paths are directly comparable): DiffTreeNames returns clean repo-relative paths (NO status prefix,
// via --name-only). concept.Files are repo-relative paths from the planner JSON. So `claimed[p]` set lookup
// is exact — no path normalization needed.

// GOTCHA (guard the call): `if !out.Single && len(out.Commits) > 0 { checkPlannerCoverage(...) }`. The
// !out.Single guard avoids running it on the single-shortcut path (which short-circuits to one commit).
// len(out.Commits)==0 avoids a vacuous union (defensive; validatePlannerOutput already ensures ≥1 when
// !single, but the guard is cheap insurance).

// GOTCHA (reuse test helpers — do NOT redefine): lockedBuffer, dcmDeps, dcmStagerSeam, piShape,
// tooledStubManifest are defined in decompose_test.go. Reuse them. The message script must have enough
// entries for loop commits + the arbiter's null→new commit (one entry per commit message generated).

// GOTCHA (no new import): decompose.go's package already imports fmt + prompt (planner.go imports prompt).
// checkPlannerCoverage adds no import. go.mod/go.sum byte-unchanged.
```

## Implementation Blueprint

### Data models and structure

No new TYPES. The helper consumes existing types: `Deps` (roles.go), `prompt.PlannerCommit` (S1's `Files`
field), and `git.DiffTreeNames`'s `[]string`. The coverage set is a local `map[string]bool`.

```go
// === internal/decompose/decompose.go — ADD checkPlannerCoverage (near Decompose) ===

// checkPlannerCoverage implements PRD §9.14 FR-M3b: a deterministic, NON-FATAL planner coverage check.
// It unions the paths declared across all concept.Files and compares against the frozen changed-path set
// DiffTreeNames(baseTree, tStart); each frozen path the planner left unclaimed is logged (verbose) as a
// likely arbiter leftover.
//
// DIAGNOSTIC ONLY (FR-M3b): this NEVER aborts the run and NEVER hard-constrains the stager. FR-M1c
// (verifyFreezeSubset in runLoop) is the SOLE content guarantee; this check is about planner PRECISION
// (did it account for every path?), not correctness. On any error it logs "coverage check skipped: <err>"
// and returns — best-effort, void.
//
// DiffTreeNames does NOT apply excludes (consistent with the arbiter gate, which also uses the full frozen
// path-set): an excluded-but-changed file legitimately shows as unclaimed and flows to the arbiter.
func checkPlannerCoverage(ctx context.Context, deps Deps, baseTree, tStart string, concepts []prompt.PlannerCommit) {
	claimed := make(map[string]bool)
	for _, c := range concepts {
		for _, f := range c.Files {
			claimed[f] = true
		}
	}
	changed, err := deps.Git.DiffTreeNames(ctx, baseTree, tStart)
	if err != nil {
		deps.Verbose.VerboseRawOutput(fmt.Sprintf("coverage check skipped: %v", err))
		return // best-effort — NEVER propagate
	}
	for _, p := range changed {
		if !claimed[p] {
			deps.Verbose.VerboseRawOutput(fmt.Sprintf("decompose: path %q not claimed by any concept (likely leftover for the arbiter)", p))
		}
	}
}
```

```go
// === internal/decompose/decompose.go — EDIT Decompose: insert the guarded call ===
// Locate the block:
//	out, err := callPlanner(ctx, deps, deps.Config.Commits, isUnborn, baseTree, tStart)
//	if err != nil {
//		return DecomposeResult{}, err
//	}
//
//	// (4) FR-M11 single-SHORTCUT: planner judged N=1 + supplied a message.
//	if out.Single {
//		return runSingleShortcut(...)
//	}
//
// Insert BETWEEN the err-check and the (4) comment:
//
//	out, err := callPlanner(ctx, deps, deps.Config.Commits, isUnborn, baseTree, tStart)
//	if err != nil {
//		return DecomposeResult{}, err
//	}
//
//	// FR-M3b: deterministic, NON-FATAL planner coverage check. Unions concept.Files and logs (verbose) any
//	// frozen changed-path the planner left unclaimed — a likely arbiter leftover. Diagnostic ONLY: never
//	// aborts, never hard-constrains the stager (FR-M1c/verifyFreezeSubset remains the sole content guarantee).
//	if !out.Single && len(out.Commits) > 0 {
//		checkPlannerCoverage(ctx, deps, baseTree, tStart, out.Commits)
//	}
//
//	// (4) FR-M11 single-SHORTCUT: planner judged N=1 + supplied a message.
//	if out.Single {
//		return runSingleShortcut(...)
//	}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: internal/decompose/decompose.go — ADD the checkPlannerCoverage helper
  - ADD the helper per the Blueprint above (void; union concept.Files; DiffTreeNames; log unclaimed via
    VerboseRawOutput; log+return "coverage check skipped: <err>" on DiffTreeNames error).
  - GOTCHA: VOID return — best-effort, never propagate. Nil-safe on deps.Verbose (VerboseRawOutput guards).
  - GOTCHA: use VerboseRawOutput (contract-mandated), NOT VerboseWarn, for BOTH log lines.
  - NAMING/PLACEMENT: checkPlannerCoverage; place it near Decompose in decompose.go (e.g. just below the
    Decompose function, or alongside the other decompose.go helpers).
  - GOTCHA: no new import (fmt + prompt already in the package).

Task 2: internal/decompose/decompose.go — INSERT the 4-line guarded call in Decompose
  - INSERT the call (Blueprint above) between callPlanner's `if err != nil { return ... }` and the
    `// (4) FR-M11 single-SHORTCUT` / `if out.Single` block.
  - GOTCHA: guard `!out.Single && len(out.Commits) > 0`. The call is a statement (no assignment) — pure
    side-effect.
  - GOTCHA: do NOT alter any other line of Decompose / runLoop / the arbiter.

Task 3: internal/decompose/decompose_test.go — ADD TestDecompose_PlannerCoverageLogsUnclaimed
  - COPY the setup pattern from the verbose-capture test at ~L2395-2428 (stubtest.Manifest planner with
    Out:plannerJSON; dcmDeps; dcmStagerSeam; `var lb lockedBuffer; deps.Verbose = ui.NewVerbose(&lb, true)`;
    Decompose(ctx, deps)).
  - SCENARIO: a repo with 3 changed files (a.txt, b.txt, c.txt) vs base. plannerJSON = 2 concepts claiming
    only a.txt + b.txt (c.txt deliberately omitted):
    `{"count":2,"single":false,"commits":[{"title":"A","description":"d1","files":["a.txt"]},{"title":"B","description":"d2","files":["b.txt"]}]}`
  - STAGER SEAM: c1→["a.txt"], c2→["b.txt"]. MESSAGE script: ≥3 entries (2 loop + 1 arbiter null→new).
    ARBITER: `{"target": null}` (c.txt → new commit).
  - ASSERT: `err == nil` (run SUCCEEDS — the omitted path is NOT fatal; the arbiter commits it) AND
    `strings.Contains(lb.String(), "decompose: path \"c.txt\" not claimed by any concept (likely leftover for the arbiter)")`.
  - GOTCHA: reuse lockedBuffer/dcmDeps/dcmStagerSeam/piShape/tooledStubManifest — do NOT redefine. Ensure
    the message script has enough entries (loop + arbiter).
  - PLACEMENT: internal/decompose/decompose_test.go (append).

Task 4: VERIFY (run all gates; fix before declaring done)
  - gofmt -w internal/decompose/decompose.go internal/decompose/decompose_test.go
  - go build ./... && go vet ./internal/decompose/
  - go test -race ./internal/decompose/ -run "TestDecompose_PlannerCoverageLogsUnclaimed" -v
  - go test ./...   # full regression — no other test should change (the check is diagnostic-only, no behavior change)
  - git diff --exit-code go.mod go.sum → empty.
  - git status → EXACTLY 2 files (decompose.go, decompose_test.go). validatePlannerOutput/callPlanner/
    runLoop/arbiter/prompts/docs byte-unchanged.
```

### Implementation Patterns & Key Details

```go
// PATTERN (diagnostic-log idiom already in this file): decompose.go already uses
//   deps.Verbose.VerboseRawOutput(fmt.Sprintf("decompose: ...", ...)) for the reread-final-commits
//   best-effort log (~line 230). Mirror that exact shape.

// PATTERN (nil-safe verbose): VerboseRawOutput's `if v==nil || v.w==nil || !v.on {return}` means the helper
//   needs NO nil guard on deps.Verbose — call it unconditionally. Library callers pass nil; the CLI passes
//   a stderr writer; tests pass a capturing buffer. All work.

// CRITICAL (M3b ≠ M1c — §9): the helper is VOID + best-effort. It NEVER returns an error, NEVER mutates
//   state, NEVER feeds runLoop/the arbiter. FR-M1c (verifyFreezeSubset) is the SOLE content guarantee. If
//   you find yourself adding a hard abort or a stager constraint here, STOP — that duplicates/violates M1c.

// CRITICAL (DiffTreeNames path comparability): DiffTreeNames returns clean repo-relative paths (--name-only,
//   no status prefix). concept.Files are repo-relative paths from the planner JSON. So `claimed[p]` is an
//   exact lookup — no normalization. (Renames in T_start are emitted by --name-only as the destination path,
//   matching how the planner names files; verified consistent with the arbiter gate's use of DiffTreeNames.)

// PATTERN (test = the existing verbose-capture decompose test): copy the ~L2395 setup. The only differences
//   are (a) plannerJSON concepts omit one path, (b) the assertion targets the coverage line in lb.String().
//   The run succeeds because the arbiter commits the leftover (the realistic M3b scenario).
```

### Integration Points

```yaml
DECOMPOSE ORCHESTRATOR (internal/decompose/decompose.go — EDIT):
  - add: "checkPlannerCoverage(ctx, deps, baseTree, tStart, concepts) helper (void, best-effort) + a 4-line
          guarded call between callPlanner's err-check and `if out.Single`."
  - effect: "after a multi-concept planner partition, any frozen path not in the union of concept.Files is
          logged (verbose) as a likely arbiter leftover. NO effect on runLoop/the arbiter/FR-M1c."

VERBOSE SINK (internal/ui/verbose.go — CONSUME, no edit):
  - use: "deps.Verbose.VerboseRawOutput(fmt.Sprintf(...)) — nil-receiver-safe; contract-mandated (NOT
          VerboseWarn). Both the unclaimed line and the skip line use it."

GIT (internal/git/git.go — CONSUME, no edit):
  - use: "deps.Git.DiffTreeNames(ctx, baseTree, tStart) — the frozen changed-path set; clean paths; does NOT
          apply excludes (consistent with the arbiter gate)."

GO MODULE (go.mod/go.sum): change NONE. fmt + prompt already imported in the decompose package.

UPSTREAM (consume, do NOT edit): PlannerCommit.Files (S1 — READ only); callPlanner, validatePlannerOutput
      (UNCHANGED); DiffTreeNames, VerboseRawOutput (UNCHANGED).

DOWNSTREAM (consumers — not this task): the coverage log is read by humans via --verbose (and by the test's
      capturing buffer). No downstream code consumes it — it is terminal diagnostics.

FROZEN/LEAVE (do NOT edit):
  - PlannerCommit.Files + internal/prompt/* (S1's domain).
  - validatePlannerOutput, callPlanner, ParsePlannerOutput (coverage ≠ validation).
  - runLoop, verifyFreezeSubset, the arbiter (the check never feeds them).
  - internal/git/*, internal/ui/*, internal/stubtest/*.
  - planner/stager/arbiter system prompts, docs/how-it-works.md (T2/T3/P3; item §5 = NO DOCS).
  - go.mod/go.sum, Makefile, PRD.md.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach
gofmt -w internal/decompose/decompose.go internal/decompose/decompose_test.go
go vet ./internal/decompose/
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; gofmt -l empty; go.mod/go.sum unchanged.
```

### Level 2: Unit Tests (the new coverage test + regression)

```bash
# The new test (stub planner omitting a path; real git; capturing verbose):
go test -race ./internal/decompose/ -run "TestDecompose_PlannerCoverageLogsUnclaimed" -v
# Expected: PASS. The run SUCCEEDS (err==nil) AND lb.String() contains:
#   `decompose: path "c.txt" not claimed by any concept (likely leftover for the arbiter)`

# Full decompose suite (regression — the check is diagnostic-only, so no existing test changes behavior):
go test -race ./internal/decompose/ -v

# Full module:
go test ./...
```

### Level 3: Integration / Behavioral Proof

```bash
# Empirically confirm the diagnostic fires and is non-fatal. The Go test above IS the integration proof
# (real git for DiffTreeNames + the freeze; stub planner for the JSON; the full Decompose pipeline incl. the
# arbiter committing the leftover). To see the line at the CLI (manual):
make build
cd /tmp && rm -rf cov && mkdir cov && cd cov && git init -q . && git config user.email t@t && git config user.name t
printf 'a\n' > a.txt; printf 'b\n' > b.txt; printf 'c\n' > c.txt; git add . && git commit -qm base
printf 'a2\n' > a.txt; printf 'b2\n' > b.txt; printf 'c2\n' > c.txt   # 3 modified, nothing staged ⇒ decompose
# (Run stagecoach with a planner that omits c.txt — the verbose line appears pre-loop. The Go test pins this
#  deterministically without needing a real agent.)
cd /; rm -rf /tmp/cov
```

### Level 4: Regression & Audit

```bash
cd /home/dustin/projects/stagecoach
go build ./...                 # whole module compiles
go test ./...                  # FULL regression — the check is diagnostic-only ⇒ no behavior change
git status --short             # Expected: EXACTLY 2 modified files:
                               #   M internal/decompose/decompose.go
                               #   M internal/decompose/decompose_test.go
# Expected: build + full test green; only 2 files; go.mod/go.sum unchanged; validatePlannerOutput /
#   callPlanner / runLoop / arbiter / prompts / docs byte-unchanged.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l internal/` empty; `go vet ./internal/decompose/` clean.
- [ ] Level 2: `go test -race ./internal/decompose/ -run TestDecompose_PlannerCoverageLogsUnclaimed -v`
      green; the full decompose suite green (no regression).
- [ ] Level 3: the test proves the diagnostic fires (buffer names the omitted path) AND the run succeeds.
- [ ] Level 4: `go build ./...` + `go test ./...` green; `git status` shows EXACTLY 2 files; go.mod/go.sum
      unchanged.

### Feature Validation

- [ ] `checkPlannerCoverage(ctx, deps, baseTree, tStart, concepts []prompt.PlannerCommit)` exists, is VOID,
      and is nil-safe on `deps.Verbose`.
- [ ] It unions `concept.Files`, calls `DiffTreeNames(ctx, baseTree, tStart)`, logs each unclaimed path via
      `VerboseRawOutput`, and logs `"coverage check skipped: <err>"` + returns on a DiffTreeNames error.
- [ ] `Decompose` calls it between callPlanner's err-check and `if out.Single`, guarded by `!out.Single &&
      len(out.Commits) > 0`.
- [ ] A planner partition omitting a path ⇒ the run SUCCEEDS (arbiter commits the leftover) AND the verbose
      buffer names the omitted path.
- [ ] The check NEVER aborts / NEVER constrains the stager (FR-M1c is the sole content guarantee).

### Code Quality Validation

- [ ] The helper mirrors the existing `deps.Verbose.VerboseRawOutput(fmt.Sprintf("decompose: ...", ...))` idiom.
- [ ] It is VOID + best-effort (no error return; log+return on DiffTreeNames failure).
- [ ] It uses `VerboseRawOutput` (contract-mandated), not `VerboseWarn`.
- [ ] No state mutation, no control-flow change beyond the 4-line guarded call; runLoop/arbiter/M1c untouched.
- [ ] Reuses `lockedBuffer`/`dcmDeps`/`dcmStagerSeam` (no test-helper redefinition).
- [ ] Anti-patterns avoided (see below): no hard abort, no stager constraint, no VerboseWarn, no excludes.

### Documentation & Deployment

- [ ] The helper has a doc comment naming FR-M3b + the DIAGNOSTIC-ONLY rationale + the M3b-vs-M1c distinction.
- [ ] The guarded call has a brief comment (FR-M3b; non-fatal; FR-M1c is the content guarantee).
- [ ] N/A — no user-facing docs (item contract: "DOCS: none — diagnostic-only").
- [ ] Implementation summary records: the insertion point, the void helper, the test scenario, the M3b-vs-M1c fence.

---

## Anti-Patterns to Avoid

- ❌ **Don't make the check fatal.** FR-M3b is DIAGNOSTIC ONLY. It MUST NOT abort the run and MUST NOT
  hard-constrain the stager. FR-M1c (`verifyFreezeSubset` in runLoop) is the SOLE content guarantee. The
  helper is VOID + best-effort: on a `DiffTreeNames` error it logs `"coverage check skipped: <err>"` and
  returns. Returning an error / adding a hard abort duplicates and violates M1c.
- ❌ **Don't use `VerboseWarn` (use `VerboseRawOutput`).** The contract explicitly mandates
  `deps.Verbose.VerboseRawOutput(...)`. `VerboseWarn` fits a diagnostic better semantically, but the contract
  owns the choice. The test matches on the path substring regardless of the `"DEBUG: raw output:\n"` prefix.
- ❌ **Don't apply excludes to the changed-path set.** The contract mandates
  `DiffTreeNames(baseTree, tStart)` (no excludes). An excluded-but-changed file legitimately shows as
  unclaimed and flows to the arbiter — this is consistent with the arbiter gate (which also uses DiffTreeNames
  without excludes). Applying excludes here would diverge from the freeze/arbiter path-set semantics.
- ❌ **Don't insert the call in the wrong place.** It goes BETWEEN `callPlanner`'s `if err != nil { return ...
  }` and the `// (4) FR-M11 single-SHORTCUT` / `if out.Single` block — NOT inside `callPlanner`, NOT after the
  loop. `baseTree` + `tStart` are in scope at that exact point (no plumbing needed).
- ❌ **Don't feed the loop / arbiter.** The check logs and is forgotten. It does NOT pass data to `runLoop`,
  the arbiter, or `verifyFreezeSubset`. Its only output is verbose text. Any wiring into the loop is scope
  creep + a behavior change that violates "no behavior change to staging/loop."
- ❌ **Don't touch `validatePlannerOutput` / `callPlanner` / `PlannerCommit`.** Coverage ≠ validation.
  `PlannerCommit.Files` is S1's field (this task only READS it). `validatePlannerOutput` must NOT enforce
  non-empty Files (Files is guidance; FR-M1c is the content guarantee).
- ❌ **Don't forget the `!out.Single && len(out.Commits) > 0` guard.** Without `!out.Single`, the check would
  run on the single-shortcut path (which short-circuits to one commit — no concepts to cover). The
  `len(out.Commits) > 0` guard is defensive insurance against a vacuous union.
- ❌ **Don't redefine `lockedBuffer` / `dcmDeps` / `dcmStagerSeam`.** They are defined in
  `decompose_test.go` — reuse them. The message script must have enough entries (loop commits + the arbiter's
  null→new commit); under-provisioning makes the test flake.
- ❌ **Don't add imports, types, interface methods, or docs.** No new import (`fmt` + `prompt` already in the
  package); no new types; no `Git`/`Verbose` interface change; item §5 says NO DOCS. go.mod/go.sum unchanged.
- ❌ **Don't edit `internal/prompt/*`, `internal/git/*`, `internal/ui/*`, or the system prompts/docs.** Those
  are S1's / upstream's / T2/T3/P3's domains. This task is `internal/decompose/decompose.go` +
  `decompose_test.go` ONLY.
