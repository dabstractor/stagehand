---
name: "P3.M2.T1.S1 — Freeze enforcement: verify tree[i] is a content-subset of T_start after each staging step (hard abort on violation, FR-M1c) (PRD §13.6.1 FR-M1c, §9.14 FR-M1c, §20.2)"
description: |

  ADD freeze enforcement (FR-M1c) to the per-concept loop: after each staging step (invokeStagerRetry →
  freezeSnapshot → tree[i]), VERIFY tree[i] is a CONTENT-SUBSET of T_start (every path changed in
  diff(baseTree, tree[i]) must be present in diff(baseTree, T_start) AND agree on blob content). Any
  deviation — a concurrent working-tree change the stager swept in, OR a mis-behaving stager that ran a
  bare `git add -A` staging a path/content not traceable to T_start — is a HARD ABORT via a new sentinel
  ErrFreezeViolation (NON-RESCUE; already-landed commits stand, mirroring the ErrStagerMovedHEAD guard).
  The orchestrator owns the freeze boundary, NOT the stager. This consumes P3.M1.T1.S1's DiffTreeNames
  (the ONLY primitive needed — no new git method) + P3.M1.T1.S2's threaded T_start.

  CONTRACT (P3.M2.T1.S1, verbatim from the work item):
    1. RESEARCH NOTE: scout_decompose_freeze.md §(c)/(d). freezeSnapshot (stager.go:108) is the
       per-concept WriteTree wrapper; the orchestrator calls it after invokeStagerRetry (decompose.go:386).
       FR-M1c: the stager is an EXTERNAL agent running git against the live tree — do NOT trust it. The
       orchestrator owns the freeze boundary (mirroring the HEAD-movement guard, §19). ErrStagerMovedHEAD
       (stager.go) is the existing hard-abort precedent.
    2. INPUT: P3.M1.T1.S2 (T_start captured + threaded) + P3.M1.T1.S1 (DiffTreeNames).
    3. LOGIC: After each staging step (invokeStagerRetry → freezeSnapshot → tree[i]), VERIFY tree[i] is a
       CONTENT-SUBSET of T_start: every path changed in diff(baseTree, tree[i]) must be present in
       diff(baseTree, T_start) AND agree on blob content (tree[i]'s blob == T_start's blob for that path).
       Implement via DiffTreeNames(baseTree, tree[i]) ⊆ DiffTreeNames(baseTree, T_start) for the path
       check, plus a content check (e.g. `git diff tree[i] T_start -- <tree[i] changed paths>` is empty ⇒
       tree[i]'s changes are all traceable to T_start). Any deviation ... is a HARD ABORT: new sentinel
       ErrFreezeViolation (NON-RESCUE; already-landed commits 0..i-1 stand, mirroring the HEAD-movement
       guard philosophy). The orchestrator owns the freeze boundary, NOT the stager.
    4. OUTPUT: a concurrent change or mis-behaving stager can NEVER enter a commit; the run aborts cleanly
       with already-landed commits intact. Covered by the §20.2 'start-of-run freeze' invariant test.
    5. DOCS: [Mode A] decompose.go/stager.go doc comment (FR-M1c enforcement: the orchestrator owns the
       freeze boundary).

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - git.go — UNCHANGED. DiffTreeNames (git.go:258/1240) is CONSUMED as-is (the ONLY primitive this task
      needs — findings §4 proves no new git method is required; the git.Git interface has NO path-restricted
      diff, and the content check is done via the DiffTreeNames intersection trick, NOT a pathspec).
    - planner.go / message.go / arbiter.go / chain.go / roles.go — UNCHANGED. (chain.go's arbiter staging
      is a SEPARATE freeze surface — see Cohesion §"Why"/findings §8; NOT this task per contract point 3.)
    - go.mod / go.sum — UNCHANGED (stdlib only; +strings already transitively present).

  DELIVERABLES (2 files MODIFIED + their tests; 0 NEW files; 0 git.go changes):
    MODIFY internal/decompose/stager.go — ADD `var ErrFreezeViolation` (next to ErrStagerMovedHEAD) +
      ADD `func verifyFreezeSubset(...)` (next to freezeSnapshot) + a private pathSet helper; +strings.
    MODIFY internal/decompose/decompose.go — runLoop: +tStart param; compute tStartPaths once before the
      loop; insert the verifyFreezeSubset call after freezeSnapshot; +tStart at the runLoop call site;
      doc comments (FR-M1c).
    MODIFY internal/decompose/stager_test.go (or decompose_test.go) — verifyFreezeSubset unit tests
      (happy / path-violation / content-violation / empty-staging-no-false-positive).
    MODIFY internal/decompose/decompose_test.go — TestDecompose_StagerFreezeViolation (rogue stager
      sweeps a post-freeze sentinel → ErrFreezeViolation; HEAD unchanged; sentinel in no commit).

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; a mis-behaving stager (or a concurrent change
  it sweeps in) triggers ErrFreezeViolation and aborts BEFORE any commit containing it; a well-behaved
  stager (the existing happy-path tests) is NOT false-positive'd; already-landed commits stand.

---

## Goal

**Feature Goal**: Add the FR-M1c freeze-enforcement guard to the decompose per-concept loop: after each
staging step produces `tree[i]`, verify `tree[i]` is a CONTENT-SUBSET of the frozen `T_start` (only paths
present in T_start, with T_start's blob content), using ONLY the existing `DiffTreeNames` primitive. Any
deviation — a concurrent working-tree change the external stager swept in, or a stager that ran a bare
`git add -A` staging a path/content not traceable to T_start — is a HARD, NON-RESCUE abort via a new
sentinel `ErrFreezeViolation`, with already-landed commits intact (mirroring the `ErrStagerMovedHEAD`
guard). The orchestrator owns the freeze boundary, not the stager.

**Deliverable** (2 files MODIFIED + tests; 0 NEW files; 0 git.go changes):
1. `internal/decompose/stager.go` — `ErrFreezeViolation` sentinel (next to `ErrStagerMovedHEAD`) + the
   `verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, treeI)` helper (next to
   `freezeSnapshot`) implementing the two-part path+content subset check via `DiffTreeNames`, plus a
   private `pathSet` helper.
2. `internal/decompose/decompose.go` — `runLoop` gains a `tStart` param, computes `tStartPaths` once
   before the loop, and calls `verifyFreezeSubset` after each `freezeSnapshot` (before the empty-skip
   check); the `runLoop` call site gains `tStart`; FR-M1c doc comments.
3. Tests — `verifyFreezeSubset` unit tests + a `TestDecompose_StagerFreezeViolation` integration test.

**Success Definition**:
- `verifyFreezeSubset` returns `nil` when `tree[i]`'s changed paths are all in `T_start` with matching
  blobs; returns an `ErrFreezeViolation`-wrapped error naming the offending path(s) on a path-not-in-
  T_start OR a content-mismatch; returns an `ErrDecomposeFailed`-wrapped error on a git failure.
- `runLoop` calls `verifyFreezeSubset` after every `freezeSnapshot`, BEFORE the `skipped := treeI ==
  prevTree` check; on violation it does `drainMsg(inflight); return commits, nil, err` (partial commits
  stand) — the exact pattern of the HEAD-movement guard and the freezeSnapshot-error path.
- A rogue stager seam that stages a sentinel file written AFTER the freeze → `Decompose` returns
  `ErrFreezeViolation`; HEAD is unchanged; the sentinel appears in no commit.
- The existing happy-path decompose tests (well-behaved stagers) still pass — the check does NOT
  false-positive on legitimate per-concept staging.
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED.

## User Persona

**Target User**: the end user running `stagecoach` on an un-staged working tree while another tool (an
editor auto-save, a concurrent coding agent) may also be writing files. The freeze enforcement is NOT a
user-facing flag; it is an internal safety guarantee (defense-in-depth, PRD §9.14 FR-M1c).

**Use Case**: the stager is the ONLY external agent that runs git (tooled mode). A mis-behaving or
flag-un-scopable stager (e.g. pi's tooled profile has no tool allowlist — Issue 2 / PRD §19) could run a
bare `git add -A` and sweep a concurrent change into the index, or stage content it fabricated. Without
enforcement, that content would land in a commit the user never intended. The guard catches it at the
freeze boundary (right after `tree[i]` is materialized) and aborts cleanly BEFORE any commit containing it.

**Pain Points Addressed**: the v2 decompose trust hole — the external stager mutates the index, and
stagecoach cannot assume it honors `T_start`. The HEAD-movement guard (ErrStagerMovedHEAD) already catches
ref mutations; THIS guard catches INDEX/CONTENT violations (the stager staged a path or blob not in
T_start). Together they make the external stager fully untrusted on both axes (refs + content), closing
the §22.1 "Stager mutates the working tree/index" risk to a clean abort.

## Why

- **Closes the enforcement half of PRD §13.6.1 FR-M1c / §9.14 FR-M1c.** FR-M1c: "After each staging step,
  stagecoach verifies the resulting tree is a subset of T_start — only paths present in T_start, with
  T_start's content. Any staged path or content not traceable to T_start ... is a hard error: stagecoach
  aborts the run (non-rescue; already-landed commits stand, per FR-M12) rather than letting a concurrent
  change into a commit. The orchestrator owns the freeze boundary, not the stager." This task is the
  literal enforcement. P3.M1.T1.S2 captured T_start + froze the diff INPUTS; THIS task verifies every
  staged tree against it.
- **Mirrors the proven ErrStagerMovedHEAD guard (lowest-risk pattern).** The HEAD-movement guard
  (stager.go sentinel + decompose.go invokeStagerRetry logic) already established the exact shape: a
  non-rescue sentinel, a pre/post comparison, drainMsg+return-partial on violation, errors.Is test
  assertions. ErrFreezeViolation is the content-axis twin (same sentinel location, same abort semantics,
  same test shape). No new architectural concept.
- **Needs NO new git primitive (findings §4 — the load-bearing insight).** The content-subset check is
  expressible with ONLY `DiffTreeNames`: path check = `DiffTreeNames(baseTree, treeI) ⊆
  DiffTreeNames(baseTree, tStart)`; content check = `DiffTreeNames(baseTree, treeI) ∩
  DiffTreeNames(treeI, tStart) == ∅` (proven equivalent to the contract's path-restricted `git diff
  treeI tStart -- <paths>` without needing a pathspec, which the git.Git interface does not offer).
- **Unblocks the §20.2 'start-of-run freeze' invariant for the loop path.** The §20.2 test (a sentinel
  written after decompose begins appears in no commit) is satisfied for the external-stager path by this
  guard. (P3.M1.T1.S2's wiring froze the planner/shortcut/arbiter DIFF inputs; THIS guard freezes the
  stager's STAGING output.)

## What

MODIFY 2 production files (`stager.go`, `decompose.go`) + their tests. No new files. No new types (just
the sentinel + the helper). No new deps. No git.go change. Specifically:

- **`ErrFreezeViolation`** (stager.go, next to `ErrStagerMovedHEAD`): `var ErrFreezeViolation =
  errors.New("decompose: freeze violation")`. Rich doc: FR-M1c; the orchestrator owns the freeze
  boundary (not the stager); NON-RESCUE (no snapshot to restore — the violation is detected at tree[i],
  before its commit; already-landed commits 0..i-1 stand via the same partial-return as the HEAD guard);
  produced by `verifyFreezeSubset`; wrapped with `%w` so `errors.Is` works.
- **`verifyFreezeSubset(ctx, deps Deps, baseTree, tStart string, tStartPaths []string, i int, treeI string)
  error`** (stager.go, next to `freezeSnapshot`): implements the two-part check (findings §4):
  1. `changedTreeI := DiffTreeNames(baseTree, treeI)`; PATH check — every path in `changedTreeI` must be
     in `tStartPaths` (a precomputed set); else `ErrFreezeViolation` naming the extra path(s).
  2. CONTENT check — `delta := DiffTreeNames(treeI, tStart)`; if `changedTreeI ∩ delta` is non-empty,
     `ErrFreezeViolation` naming the content-mismatch path(s).
  Returns `ErrDecomposeFailed`-wrapped error on a DiffTreeNames git failure. `nil` if the subset holds.
- **`pathSet(paths []string) map[string]struct{}`** (stager.go, private helper): builds a set for the
  membership lookups.
- **`runLoop`** (decompose.go): signature `+tStart string` (after `baseTree`); compute `tStartPaths, err
  := deps.Git.DiffTreeNames(ctx, baseTree, tStart)` ONCE before the loop (error → `ErrDecomposeFailed`
  wrap, `return nil, nil, err`); insert `if err := verifyFreezeSubset(ctx, deps, baseTree, tStart,
  tStartPaths, i, treeI); err != nil { drainMsg(inflight); return commits, nil, err }` AFTER
  `freezeSnapshot` and BEFORE `skipped := treeI == prevTree`. The `runLoop` call site (decompose.go:185)
  gains `tStart`.
- **Doc comments:** stager.go `ErrFreezeViolation` + `verifyFreezeSubset` (FR-M1c: the orchestrator owns
  the freeze boundary); decompose.go `runLoop` doc + the freeze-boundary note (the loop verifies tree[i]
  ⊆ T_start after each staging step).

### Success Criteria

- [ ] `ErrFreezeViolation` is defined in stager.go (next to ErrStagerMovedHEAD) with a non-rescue doc;
      `errors.Is(err, ErrFreezeViolation)` is true for violations.
- [ ] `verifyFreezeSubset` returns nil for a content-subset tree; ErrFreezeViolation (naming the path/s)
      for a path-not-in-T_start OR a content-mismatch; ErrDecomposeFailed for a git failure.
- [ ] `runLoop` takes `tStart`, computes `tStartPaths` once, and calls `verifyFreezeSubset` after every
      `freezeSnapshot` (before the empty-skip check); on violation it `drainMsg(inflight)` + returns
      partial commits + the error (mirroring the HEAD guard + freezeSnapshot-error paths).
- [ ] A rogue stager that stages a post-freeze sentinel → `Decompose` returns `ErrFreezeViolation`; HEAD
      unchanged; the sentinel is in no commit.
- [ ] The existing happy-path decompose tests (well-behaved stagers) still pass — no false positive.
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact loop insertion
point + the drainMsg+return-partial pattern (findings §2); the ErrStagerMovedHEAD precedent to mirror
(§3); the content-subset MATH + why DiffTreeNames alone suffices + the worked proof (§4/§5 — the
load-bearing insight); the exact changes per file (§6); the test seam + the rogue-stager test pattern
(§7); the arbiter-staging scope decision (§8); the overlap subtlety on what "stands" (§9); the gates (§10).
The scout note (§c/§d) anchors freezeSnapshot/invokeStagerRetry. DiffTreeNames + FreezeWorkingTree already
exist; T_start is already threaded (P3.M1.T1.S2 on disk). No external research needed.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (the math + the exact changes + the precedent)
- docfile: plan/003_6ce49c39466e/P3M2T1S1/research/findings.md
  why: §1 the verbatim contract; §2 the loop anchor (the EXACT insertion point: after freezeSnapshot,
       before `skipped := treeI == prevTree`; the drainMsg+return-commits-nil-err pattern); §3 the
       ErrStagerMovedHEAD precedent (sentinel in stager.go, guard logic in decompose.go, non-rescue,
       partial-stands, errors.Is test); §4 THE MATH — the content-subset check done with ONLY
       DiffTreeNames (path check = changedTreeI ⊆ changedTStart; content check = changedTreeI ∩
       DiffTreeNames(treeI, tStart) == ∅; PROVEN equivalent to the contract's path-restricted `git diff
       treeI tStart -- <paths>`; why content alone is complete but keep both for diagnostics; NO new git
       primitive); §5 the worked example proving §4; §6 the exact per-file diff; §7 the test seam +
       rogue-stager pattern; §8 the arbiter-staging scope decision; §9 the overlap subtlety.
  critical: §4 (the content check is `changedTreeI ∩ DiffTreeNames(treeI, tStart) == ∅` — do NOT try to
            use a pathspec; the git.Git interface has NONE; the intersection trick IS the path-restricted
            content check); §2 (insert AFTER freezeSnapshot, BEFORE the empty-skip check; reuse the
            drainMsg+return-partial pattern verbatim); §3 (ErrFreezeViolation is the content-axis twin of
            ErrStagerMovedHEAD — same location, same semantics, same test shape).

# MUST READ — the parallel task's PRP (the T_start CONTRACT this task consumes — T_start is ALREADY threaded on disk)
- docfile: plan/003_6ce49c39466e/P3M1T1S2/PRP.md
  section: Decompose() captures tStart via FreezeWorkingTree(baseTree) (decompose.go:164) + threads it to
           callPlanner/runSingleShortcut/runArbiterPhase; runLoop is UNCHANGED in that task (does NOT take
           tStart) — THIS task adds tStart to runLoop. The freeze is index-idempotent on failure.
  why: confirms T_start is in scope in Decompose() (line 164) and available to pass to runLoop. THIS task
       is the +tStart-to-runLoop change + the verify call. Do NOT re-capture T_start (it is already
       captured once in Decompose and must be threaded, not re-derived).
  critical: runLoop currently does NOT take tStart (P3.M1.T1.S2 left it unchanged). THIS task adds the
            `tStart` param to runLoop's signature + the call site. baseTree is already a runLoop param.

# MUST READ — the scout research note (the anchor map)
- docfile: plan/003_6ce49c39466e/architecture/scout_decompose_freeze.md
  section: §(c) stageConcept (stager.go:58) + freezeSnapshot (stager.go:108 — thin WriteTree wrapper);
           §(d) how the stager reads the working tree (the TOOLED agent runs git add; the orchestrator
           owns retry + freezeSnapshot); §(b) the loop anchors (invokeStagerRetry decompose.go:353,
           freezeSnapshot decompose.go:386).
  why: confirms freezeSnapshot is the freeze primitive + that the orchestrator (not the stager) owns the
       freeze boundary (FR-M1c). The verify call goes right after freezeSnapshot in runLoop.

# MUST READ — DiffTreeNames: the ONLY primitive this task needs (UNCHANGED)
- file: internal/git/git.go
  section: DiffTreeNames interface doc (git.go:242–258) + impl (git.go:1240–1271).
  why: DiffTreeNames(treeA, treeB) returns the SORTED, DEDUPED changed-path set via `git diff-tree -r
       --name-only --no-commit-id`; identical trees ⇒ (nil, nil); read-only w.r.t. refs+index; NOT
       unborn-aware (caller passes EmptyTreeSHA). verifyFreezeSubset calls it 3 ways: (baseTree, treeI),
       (baseTree, tStart) [once in runLoop], (treeI, tStart).
  gotcha: DiffTreeNames returns a nil slice for identical trees (len 0) — `range nil` is a safe no-op, and
          the empty-staging case (treeI == baseTree ⇒ changedTreeI nil ⇒ both checks trivially pass ⇒ no
          false positive on FR-M8 empty-skip). Do NOT add a new git method — the intersection trick (§4)
          is how you do the content check.

# MUST READ — the FILES TO MODIFY
- file: internal/decompose/stager.go
  section: ErrStagerMovedHEAD (line 46 — the sentinel to mirror) + freezeSnapshot (line 108 — where
           verifyFreezeSubset goes, as its enforcement sibling).
  why: ErrFreezeViolation goes next to ErrStagerMovedHEAD (same stager-safety-sentinel grouping);
       verifyFreezeSubset goes next to freezeSnapshot (freeze primitive + freeze enforcement, cohesive).
       Both take (ctx, deps) and are CALLED by the orchestrator (decompose.go) — symmetric with
       freezeSnapshot's call site.
  pattern: mirror ErrStagerMovedHEAD's doc style (FR citation + non-rescue + partial-stands + %w wrap);
           verifyFreezeSubset mirrors freezeSnapshot's signature shape (ctx, deps, ...) + a rich doc.
  gotcha: stager.go currently imports context/errors/fmt/config/prompt/provider — ADD "strings" (for the
          Join in the error messages). Add a private pathSet helper.

- file: internal/decompose/decompose.go
  section: runLoop signature (line 291) + the loop body (lines 380–410: invokeStagerRetry → freezeSnapshot
           → [INSERT HERE] → skipped → publish → launch) + the runLoop call site (line 185) +
           ErrDecomposeFailed (line 38) + drainMsg (the partial-return helper).
  why: ALL the decompose.go changes live here. The insertion is between freezeSnapshot and `skipped :=`.
  pattern: signature `func runLoop(ctx, deps Deps, concepts []prompt.PlannerCommit, baseTree, tStart,
           preRunHEAD string, isUnborn bool)`; before the loop: `tStartPaths, err := deps.Git.DiffTreeNames(
           ctx, baseTree, tStart); if err != nil { return nil, nil, fmt.Errorf("%w: freeze baseline
           diff-tree-names: %w", ErrDecomposeFailed, err) }`; after freezeSnapshot: `if err :=
           verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, treeI); err != nil {
           drainMsg(inflight); return commits, nil, err }`; call site +tStart (after baseTree).
  gotcha: the verify call MUST go BEFORE `skipped := treeI == prevTree` (an empty-staging treeI == prevTree
          is still a valid subset — changedTreeI is empty ⇒ both checks pass ⇒ no false positive on the
          FR-M8 skip). Reuse `err` carefully (it is in scope from freezeSnapshot; `if err := ...` shadows
          it in a new block — fine). The call site: `runLoop(ctx, deps, out.Commits, baseTree, tStart,
          preRunHEAD, isUnborn)`.

# MUST READ — the ErrStagerMovedHEAD precedent (the guard to mirror) + its test
- file: internal/decompose/decompose.go
  section: invokeStagerRetry (lines 353–378 — the HEAD pre/post-snapshot guard; `drainMsg`+return-partial;
           errors.Is short-circuits bypass retry-once-then-empty).
- file: internal/decompose/decompose_test.go
  section: TestDecompose_StagerMovedHEAD (line 573) + TestDecompose_StagerGuardHappyPath (line 623) — the
           rogue-stager + well-behaved-stager test pair to mirror for ErrFreezeViolation.
  why: ErrFreezeViolation's test is the content-axis twin of TestDecompose_StagerMovedHEAD: a rogue
       deps.stager closure that stages a post-freeze sentinel → assert errors.Is(err, ErrFreezeViolation)
       + HEAD unchanged + sentinel in no commit. The happy-path test proves no false positive.

# MUST READ — the test seam + helpers (REUSE — do NOT redefine)
- file: internal/decompose/roles.go
  section: the Deps.stager OPTIONAL test seam (the `stager func(ctx, deps Deps, concept
           prompt.PlannerCommit) error` field) — invokeStager dispatches to it when non-nil.
- file: internal/decompose/decompose_test.go
  section: the test helpers — dcmInitRepo / dcmCommitRaw / dcmWriteFile / dcmRunGit / dcmPlannerManifest /
           dcmMessageScriptManifest / dcmAllRoles / dcmDeps / dcmLogCount / dcmStagerSeam (all package
           `decompose` — visible, REUSE).
  why: the verifyFreezeSubset unit tests + the rogue-stager integration test use these. The unit test can
       call git.New(repo) directly to set up baseTree/tStart/treeI.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §9.14 FR-M1c (the freeze-enforcement contract: "after each staging step, stagecoach verifies
       the resulting tree is a subset of T_start ... hard error ... non-rescue; already-landed commits
       stand ... The orchestrator owns the freeze boundary, not the stager — mirroring the HEAD-movement
       guard (§19)").
- url: PRD.md §13.6.1 FR-M1c (the freeze-enforcement cross-reference) + §20.2 "Start-of-run freeze (v2)"
       (the sentinel-appears-in-no-commit invariant test).
  why: FR-M1c is the authoritative requirement; §20.2 is the test contract (a sentinel written after
       decompose begins appears in no commit — this guard satisfies it for the loop/stager path).
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  stager.go              # MODIFY: +ErrFreezeViolation sentinel (next to ErrStagerMovedHEAD) +
                         #   +verifyFreezeSubset helper (next to freezeSnapshot) +pathSet; +strings import.
  decompose.go           # MODIFY: runLoop +tStart param + tStartPaths once-before-loop + the verify call
                         #   (after freezeSnapshot, before skipped) + the runLoop call site +tStart; docs.
  planner.go             # READ/UNCHANGED: callPlanner already diffs frozen tStart (P3.M1.T1.S2).
  message.go             # READ/UNCHANGED: generateMessage already tree-to-tree.
  arbiter.go             # READ/UNCHANGED: runArbiter takes leftoverDiff as a param.
  chain.go               # READ/UNCHANGED (arbiter staging is a SEPARATE freeze surface — findings §8).
  roles.go               # READ/UNCHANGED: Deps.stager test seam (consumed by tests).
  stager_test.go         # MODIFY: +verifyFreezeSubset unit tests.
  decompose_test.go      # MODIFY: +TestDecompose_StagerFreezeViolation (rogue sentinel stager).
internal/git/git.go      # READ/UNCHANGED: DiffTreeNames (:258/1240) + FreezeWorkingTree consumed as-is.
go.mod / go.sum          # UNCHANGED (stdlib only; no new deps).
```

### Desired Codebase tree with files to be modified

```bash
internal/decompose/stager.go          # MODIFIED — +var ErrFreezeViolation (non-rescue sentinel, mirrors
                                      #   ErrStagerMovedHEAD); +func verifyFreezeSubset(ctx, deps, baseTree,
                                      #   tStart, tStartPaths []string, i int, treeI) error (two-part path+
                                      #   content subset check via DiffTreeNames); +func pathSet; +strings.
internal/decompose/decompose.go       # MODIFIED — runLoop: +tStart param (after baseTree); compute
                                      #   tStartPaths once before the loop; insert verifyFreezeSubset after
                                      #   freezeSnapshot (before `skipped :=`); on violation drainMsg+
                                      #   return partial; runLoop call site +tStart; FR-M1c doc comments.
internal/decompose/stager_test.go     # MODIFIED — +TestVerifyFreezeSubset_{Happy,PathViolation,
                                      #   ContentViolation,EmptyStaging} unit tests (real git via git.New).
internal/decompose/decompose_test.go  # MODIFIED — +TestDecompose_StagerFreezeViolation (rogue stager
                                      #   sweeps a post-freeze sentinel → ErrFreezeViolation; HEAD unchanged;
                                      #   sentinel in no commit).
# NO new files. git.go/planner.go/message.go/arbiter.go/chain.go/roles.go UNCHANGED. go.mod/go.sum UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the content check is the DiffTreeNames INTERSECTION — findings §4, the load-bearing insight):
//   tree[i] is a content-subset of T_start (for its changed paths) IFF:
//     (A) PATH:  DiffTreeNames(baseTree, treeI) ⊆ DiffTreeNames(baseTree, tStart)
//     (B) CONTENT: DiffTreeNames(baseTree, treeI) ∩ DiffTreeNames(treeI, tStart) == ∅
//   (B) is PROVEN equivalent to the contract's `git diff treeI tStart -- <changed paths>` empty: a path p
//   is in DiffTreeNames(treeI, tStart) iff blob(treeI,p) != blob(tStart,p); intersecting with changedTreeI
//   isolates tree[i]'s changed paths whose content differs from T_start. The git.Git interface has NO
//   path-restricted diff (StagedDiffOptions has only Excludes) — the intersection trick IS the path-
//   restricted content check. Do NOT add a new git method. (B) alone is complete for detection; keep (A)
//   too for the clearer "path not present in T_start" diagnostic + to match the contract's two-part framing.

// CRITICAL (insert AFTER freezeSnapshot, BEFORE `skipped := treeI == prevTree` — findings §2): the verify
//   call must run before the empty-skip check. An empty-staging treeI (== prevTree) has changedTreeI==nil
//   ⇒ both checks trivially pass (range nil is a no-op) ⇒ NO false positive on the FR-M8 skip. On
//   violation: `drainMsg(inflight); return commits, nil, err` — the EXACT pattern the HEAD-movement guard
//   (invokeStagerRetry) and the freezeSnapshot-error path use (drain the in-flight msg goroutine, return
//   the partial commits that already landed). Do NOT publish before returning (mirrors the guard).

// CRITICAL (ErrFreezeViolation is NON-RESCUE, mirroring ErrStagerMovedHEAD — findings §3): it is NOT a
//   *generate.RescueError (no §18.3 rescue recipe — the violation is caught at tree[i] BEFORE its commit;
//   there is no snapshot-then-CAS to rescue). already-landed commits 0..i-1 stand (returned in `commits`);
//   the in-flight concept's staging remains in the index (FR-M12). Define it in stager.go next to
//   ErrStagerMovedHEAD; wrap violations with %w so errors.Is(err, ErrFreezeViolation) works.

// CRITICAL (runLoop does NOT yet take tStart — findings §6): P3.M1.T1.S2 (on disk) threads tStart into
//   callPlanner/runSingleShortcut/runArbiterPhase but LEFT runLoop unchanged. THIS task adds the `tStart`
//   param to runLoop's signature (after baseTree) + the call site (decompose.go:185). baseTree is already
//   a runLoop param. Do NOT re-capture T_start inside runLoop — thread the one Decompose() captured.

// GOTCHA (compute tStartPaths ONCE, before the loop — findings §6): DiffTreeNames(baseTree, tStart) is
//   INVARIANT across the run (T_start + baseTree don't change). Compute it once at the top of runLoop and
//   pass it to every verifyFreezeSubset call. Do NOT recompute per concept. A git error there →
//   ErrDecomposeFailed wrap, return nil,nil,err (no concepts processed yet).

// GOTCHA (the overlap: commit[i-1] is in-flight at freeze[i] — findings §9): runLoop's 1-deep overlap
//   means publish(inflight) [commit i-1] happens AFTER freezeSnapshot[i]. So when verifyFreezeSubset fires
//   at concept i, commit[i-1] is in-flight (drained, not landed); the LANDED commits are 0..i-2. The
//   contract's "0..i-1 stand" reflects the logical count; the overlap means the in-flight one's STAGING
//   stays in the index. The PRIMARY test uses count=1 (concept 0 violates ⇒ no prior commits, clean
//   abort). Partial-stands semantics are structurally identical to the HEAD guard (already tested).

// GOTCHA (no false positive on the happy path): a well-behaved stager stages concept paths that ARE in
//   T_start with T_start's blob content (it git-adds working-tree files == T_start's content). changedTreeI
//   ⊆ changedTStart + content matches ⇒ verifyFreezeSubset returns nil. The existing happy-path tests
//   (TestDecompose_StagerGuardHappyPath + the dcmStagerSeam-based tests) MUST still pass unchanged.

// GOTCHA (the rogue-stager test stages a post-freeze sentinel — findings §7): the sentinel is written to
//   the working tree AFTER FreezeWorkingTree captured T_start, then `git add`ed by the rogue stager seam.
//   treeI then contains the sentinel ∉ T_start ⇒ path check fires ⇒ ErrFreezeViolation. The sentinel must
//   NOT be a concept path (the planner diffed T_start, which predates it — so it is never a concept).
//   Assert: errors.Is(err, ErrFreezeViolation); HEAD unchanged (dcmLogCount unchanged); sentinel untracked
//   / in no commit. Mirror TestDecompose_StagerMovedHEAD's shape (decompose_test.go:573).

// GOTCHA (arbiter staging is OUT OF SCOPE — findings §8): the contract (point 3) scopes THIS task to the
//   per-concept LOOP (invokeStagerRetry → freezeSnapshot → tree[i]) — the EXTERNAL tooled stager (the trust
//   boundary FR-M1c names). The arbiter is BARE (stagecoach owns its git: chain.go AddAll/Add). Its AddAll
//   COULD sweep a concurrent change, but closing that means staging from T_start (a chain.go change) — NOT
//   this task. Document this as a conscious scoping decision. Under the no-concurrent-change invariant
//   (the common case) the arbiter's AddAll already yields a T_start subset.

// GOTCHA (cross-package test helpers are package-local): dcmInitRepo/dcmWriteFile/dcmRunGit/etc. are in
//   package `decompose` (decompose_test.go) — REUSE them; do NOT redefine (duplicate-symbol error). For
//   the verifyFreezeSubset unit test, call git.New(repo) directly to set up baseTree/tStart/treeI.
```

## Implementation Blueprint

### Data models and structure

No new types. No new structs. One new sentinel + one new helper + one private set helper. Consumes
EXISTING primitives unchanged:

```go
// from internal/git/git.go (CONSUME — do not modify):
//   DiffTreeNames(ctx, treeA, treeB string) (paths []string, err error)   // git.go:258/1240 — sorted, deduped changed-path set
//   FreezeWorkingTree(ctx, baseTree string) (tStart string, err error)    // git.go:240/1223 — already called in Decompose()
//   EmptyTreeSHA                                                           // git.go:500
// from internal/decompose/decompose.go (CONSUME): ErrDecomposeFailed (line 38), drainMsg, runLoop, the msgOut/launch/publish closures.
// from internal/decompose/stager.go (CONSUME): freezeSnapshot (line 108), ErrStagerFailed, ErrStagerMovedHEAD (line 46 — the precedent).
```

```go
// internal/decompose/stager.go — NEW sentinel (next to ErrStagerMovedHEAD)
// ErrFreezeViolation is the sentinel for an FR-M1c FREEZE-ENFORCEMENT violation: the stager produced a
// tree[i] whose changed paths or blob contents are NOT traceable to T_start (a concurrent working-tree
// change the stager swept in, or a mis-behaving stager that ran a bare `git add -A` / staged fabricated
// content). The orchestrator owns the freeze boundary (NOT the stager) — mirroring ErrStagerMovedHEAD
// (the ref-axis guard) on the content axis. NON-RESCUE: the violation is detected at tree[i] BEFORE its
// commit, so there is no snapshot-then-CAS to rescue; already-landed commits 0..i-1 stand (returned as
// partial results), and the in-flight concept's staging remains in the index (FR-M12). Produced by
// verifyFreezeSubset; wrapped with %w so errors.Is(err, ErrFreezeViolation) is true.
var ErrFreezeViolation = errors.New("decompose: freeze violation")
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/decompose/stager.go — ErrFreezeViolation sentinel
  - ADD `var ErrFreezeViolation = errors.New("decompose: freeze violation")` IMMEDIATELY AFTER
    ErrStagerMovedHEAD (stager.go:~70, after the ErrStagerMovedHEAD doc block).
  - DOC COMMENT: cite PRD §9.14 FR-M1c + §13.6.1; explain it is the CONTENT-axis twin of
    ErrStagerMovedHEAD (the ref-axis guard); NON-RESCUE (detected at tree[i] before its commit; no
    snapshot to restore); already-landed commits stand; produced by verifyFreezeSubset; %w-wrapped.
  - PLACEMENT: internal/decompose/stager.go, after ErrStagerMovedHEAD.

Task 2: MODIFY internal/decompose/stager.go — pathSet helper + verifyFreezeSubset
  - ADD a private helper `func pathSet(paths []string) map[string]struct{} { s := make(...); for _, p :=
    range paths { s[p] = struct{}{} }; return s }` (near verifyFreezeSubset).
  - ADD `func verifyFreezeSubset(ctx context.Context, deps Deps, baseTree, tStart string, tStartPaths
    []string, i int, treeI string) error` IMMEDIATELY AFTER freezeSnapshot (stager.go:~120).
    BODY (the two-part check — findings §4):
        // (A) PATH check: tree[i]'s changed paths must all be in T_start's changed set.
        changedTreeI, err := deps.Git.DiffTreeNames(ctx, baseTree, treeI)
        if err != nil {
            return fmt.Errorf("%w: freeze check diff-tree-names[%d]: %w", ErrDecomposeFailed, i, err)
        }
        tStartSet := pathSet(tStartPaths)
        var extra []string
        for _, p := range changedTreeI {
            if _, ok := tStartSet[p]; !ok {
                extra = append(extra, p)
            }
        }
        if len(extra) > 0 {
            return fmt.Errorf("%w: concept %d staged paths not present in T_start: %s",
                ErrFreezeViolation, i, strings.Join(extra, ", "))
        }
        // (B) CONTENT check: tree[i]'s changed paths must have T_start's blob content.
        //     changedTreeI ∩ DiffTreeNames(treeI, tStart) == ∅  (equiv to `git diff treeI tStart -- <paths>` empty).
        delta, err := deps.Git.DiffTreeNames(ctx, treeI, tStart)
        if err != nil {
            return fmt.Errorf("%w: freeze check diff-tree-names[%d]: %w", ErrDecomposeFailed, i, err)
        }
        deltaSet := pathSet(delta)
        var mismatch []string
        for _, p := range changedTreeI {
            if _, ok := deltaSet[p]; ok {
                mismatch = append(mismatch, p)
            }
        }
        if len(mismatch) > 0 {
            return fmt.Errorf("%w: concept %d staged content not traceable to T_start: %s",
                ErrFreezeViolation, i, strings.Join(mismatch, ", "))
        }
        return nil
  - DOC COMMENT: cite FR-M1c; explain the two-part check (path subset + content-match via the
    DiffTreeNames intersection — findings §4); note it is the content-axis enforcement sibling of the
    ErrStagerMovedHEAD ref guard; note empty changedTreeI (empty staging) ⇒ both checks pass ⇒ no false
    positive on FR-M8; note the return contract (nil / ErrFreezeViolation-wrapped / ErrDecomposeFailed-
    wrapped); note the caller (runLoop) owns drainMsg+return-partial on violation.
  - IMPORTS: ADD "strings" (for Join). stager.go already has context/errors/fmt.
  - GOTCHA: the path check runs FIRST (cheaper, clearer diagnostic); the content check (1 extra
    DiffTreeNames call) runs only if the path check passed. Do NOT call Resolve/Validate (DiffTreeNames
    works on raw tree SHAs). changedTreeI nil ⇒ both loops no-op ⇒ nil (empty-staging safe).
  - PLACEMENT: internal/decompose/stager.go, after freezeSnapshot.

Task 3: MODIFY internal/decompose/decompose.go — runLoop signature + tStartPaths + verify call
  - CHANGE runLoop signature (decompose.go:291) from
      `func runLoop(ctx context.Context, deps Deps, concepts []prompt.PlannerCommit, baseTree, preRunHEAD string, isUnborn bool) (...)`
    to
      `func runLoop(ctx context.Context, deps Deps, concepts []prompt.PlannerCommit, baseTree, tStart, preRunHEAD string, isUnborn bool) (...)`
  - ADD, at the top of runLoop (right after the `var commits []CommitResult` / setup, BEFORE the
    launch/publish/invokeStagerRetry closure definitions OR just before the `for` loop — wherever the
    setup lives), the once-per-run baseline:
        // FR-M1c: T_start's changed-path set (invariant across the run) — the subset baseline every
        // tree[i] is verified against after each staging step.
        tStartPaths, err := deps.Git.DiffTreeNames(ctx, baseTree, tStart)
        if err != nil {
            return nil, nil, fmt.Errorf("%w: freeze baseline diff-tree-names: %w", ErrDecomposeFailed, err)
        }
    (NOTE: `err` may already be declared in runLoop's setup — if so reuse it with `=`; if `tStartPaths`
    is the only new var, `tStartPaths, err :=` is valid Go if err is being reassigned alongside a new var.
    Use the form gofmt+vet accept; prefer `tStartPaths, err :=` if err is NOT yet in scope at that point,
    else predeclare or reuse. Check the actual runLoop setup block.)
  - INSERT, in the loop body, AFTER `treeI, err := freezeSnapshot(ctx, deps)` (and its error block) and
    BEFORE `skipped := treeI == prevTree`:
        // FR-M1c freeze enforcement: verify tree[i] is a content-subset of T_start (only T_start paths,
        // with T_start content). The orchestrator owns the freeze boundary — the external stager is NOT
        // trusted. NON-RESCUE on violation: already-landed commits stand; the in-flight concept's staging
        // stays in the index. Mirrors the ErrStagerMovedHEAD guard (drainMsg + return partial).
        if err := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, treeI); err != nil {
            drainMsg(inflight)
            return commits, nil, err
        }
  - CHANGE the runLoop call site (decompose.go:185) from
      `commits, chainData, err := runLoop(ctx, deps, out.Commits, baseTree, preRunHEAD, isUnborn)`
    to
      `commits, chainData, err := runLoop(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)`
  - UPDATE the runLoop doc comment: add the FR-M1c paragraph (after each staging step, tree[i] is verified
    ⊆ T_start; the orchestrator owns the freeze boundary; on violation the run aborts non-rescue with
    partial commits). Update the Decompose()/freeze-boundary doc note (decompose.go:12/129) to mention
    FR-M1c enforcement in addition to FR-M1b capture.
  - GOTCHA: the verify call is BEFORE `skipped :=` so empty staging (treeI==prevTree) is not a false
    positive. The `err` from verifyFreezeSubset shadows the outer `err` in a new `if` block — fine. The
    drainMsg(inflight) mirrors the freezeSnapshot-error + invokeStagerRetry-error paths exactly.
  - PLACEMENT: internal/decompose/decompose.go, runLoop.

Task 4: MODIFY internal/decompose/stager_test.go — verifyFreezeSubset unit tests
  - ADD TestVerifyFreezeSubset_Happy: repo init+commit (baseTree=HEAD^{tree}); dcmWriteFile a.txt+v1,
    b.txt+v1; g:=git.New(repo); tStart,_:=g.FreezeWorkingTree(ctx,baseTree); then `git add a.txt` +
    g.WriteTree → treeI (a.txt only). tStartPaths,_:=g.DiffTreeNames(ctx,baseTree,tStart). Assert
    verifyFreezeSubset(ctx,deps,baseTree,tStart,tStartPaths,0,treeI)==nil. (a.txt ⊆ T_start + content matches.)
    NOTE: verifyFreezeSubset takes `deps Deps` only to access deps.Git. For the unit test, build a minimal
    Deps{Git: git.New(repo)} (the helper uses ONLY deps.Git.DiffTreeNames) — OR refactor the helper to take
    a git.Git directly. PREFERRED: keep verifyFreezeSubset(ctx, deps, ...) for signature symmetry with
    freezeSnapshot; the unit test passes Deps{Git: g}.
  - ADD TestVerifyFreezeSubset_PathViolation: same setup; then write sentinel.txt + `git add sentinel.txt`
    + WriteTree → treeI (base+sentinel). Assert err != nil AND errors.Is(err, ErrFreezeViolation) AND the
    message contains "sentinel.txt" + "not present in T_start".
  - ADD TestVerifyFreezeSubset_ContentViolation: same setup; then modify a.txt to v2 (NOT in T_start) +
    `git add a.txt` + WriteTree → treeI (a.txt=v2). Assert errors.Is(err, ErrFreezeViolation) AND message
    contains "a.txt" + "not traceable to T_start". (Path check passes — a.txt ∈ T_start — but content
    differs; the content check fires.)
  - ADD TestVerifyFreezeSubset_EmptyStaging: treeI == baseTree (WriteTree on the clean base index). Assert
    verifyFreezeSubset returns nil (changedTreeI empty ⇒ no false positive).
  - GOTCHA: use g.Add/g.WriteTree/g.FreezeWorkingTree/g.DiffTreeNames directly (git.New(repo)). The helper
    uses only deps.Git, so Deps{Git: g} suffices. Reuse dcmInitRepo/dcmCommitRaw/dcmWriteFile/dcmRunGit
    (package decompose — do NOT redefine). For "deps" use a literal Deps{Git: git.New(repo)}.
  - PLACEMENT: internal/decompose/stager_test.go.

Task 5: MODIFY internal/decompose/decompose_test.go — TestDecompose_StagerFreezeViolation
  - ADD TestDecompose_StagerFreezeViolation (mirror TestDecompose_StagerMovedHEAD at decompose_test.go:573):
      - Setup: repo + dcmCommitRaw("initial") (born); dcmWriteFile("a.txt","aaa\n") (the legit change in T_start).
      - plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}`;
        plannerM := dcmPlannerManifest(t, bin, plannerJSON); messageM := dcmMessageScriptManifest(t, bin,
        []string{"feat: add a"}); roles := dcmAllRoles(t, bin, stubtest.Options{Out:""}); roles.Planner=
        plannerM; roles.Message=messageM; deps := dcmDeps(t, repo, roles).
      - ROGUE seam: stages the concept path AND a post-freeze sentinel (simulating `git add -A` sweeping a
        concurrent change):
          deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
              dcmRunGit(t, repo, "add", "a.txt")              // the legit concept path (in T_start)
              dcmWriteFile(t, repo, "sentinel.txt", "concurrent\n")  // appears AFTER the freeze
              dcmRunGit(t, repo, "add", "sentinel.txt")      // stager sweeps it in (the violation)
              return nil
          }
      - _, err := Decompose(context.Background(), deps).
      - ASSERT: err != nil; errors.Is(err, ErrFreezeViolation); err.Error() contains "sentinel.txt" +
        "not present in T_start"; dcmLogCount(t, repo) == 1 (HEAD unchanged — only "initial"; no commit
        landed); the sentinel is untracked/in no commit (dcmRunGit "log","--name-only" output lacks
        "sentinel.txt").
  - ADD (optional, strengthens) TestDecompose_StagerFreezeViolation_ContentMismatch: a rogue seam that
    MODIFIES a.txt to new content before staging (content not in T_start) → ErrFreezeViolation with
    "not traceable to T_start". (Mirrors the unit test at the integration level.)
  - GOTCHA: the sentinel is written by the stager seam AFTER FreezeWorkingTree captured T_start in
    Decompose() — so it is genuinely absent from T_start. The planner diffed T_start (frozen), so the
    sentinel is never a concept. Do NOT redefine dcm* helpers. errors.Is + message-substring assertions
    mirror TestDecompose_StagerMovedHEAD.
  - PLACEMENT: internal/decompose/decompose_test.go (near TestDecompose_StagerMovedHEAD).

Task 6: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/decompose/stager.go internal/decompose/decompose.go internal/decompose/stager_test.go internal/decompose/decompose_test.go`
  - `go build ./...`   (the runLoop signature change + the new sentinel/helper compile)
  - `go vet ./...`
  - `golangci-lint run ./...`   (errcheck: the DiffTreeNames errors are checked; unused: pathSet/verifyFreezeSubset used; watch the `err` shadowing in runLoop)
  - `go test -race ./internal/decompose/ -run "VerifyFreezeSubset|StagerFreeze|StagerGuard|StagerMoved" -v`
  - `go test -race ./internal/decompose/`   (the WHOLE decompose package — esp. the happy-path tests prove no false positive)
  - `go test ./...`   (FULL regression)
  - `git diff --exit-code go.mod go.sum` ⇒ empty.
  - `git status --short` ⇒ M stager.go + M decompose.go + M stager_test.go + M decompose_test.go (4 modified;
    NO new files; git.go/planner.go/message.go/arbiter.go/chain.go/roles.go UNCHANGED).
```

### Implementation Patterns & Key Details

```go
// === Task 2: verifyFreezeSubset — the two-part content-subset check (DiffTreeNames only) ===
func verifyFreezeSubset(ctx context.Context, deps Deps, baseTree, tStart string, tStartPaths []string, i int, treeI string) error {
	// (A) PATH check: every path tree[i] changed (vs base) must be in T_start's changed set.
	changedTreeI, err := deps.Git.DiffTreeNames(ctx, baseTree, treeI)
	if err != nil {
		return fmt.Errorf("%w: freeze check diff-tree-names[%d]: %w", ErrDecomposeFailed, i, err)
	}
	tStartSet := pathSet(tStartPaths)
	var extra []string
	for _, p := range changedTreeI {
		if _, ok := tStartSet[p]; !ok {
			extra = append(extra, p)
		}
	}
	if len(extra) > 0 {
		return fmt.Errorf("%w: concept %d staged paths not present in T_start: %s", ErrFreezeViolation, i, strings.Join(extra, ", "))
	}
	// (B) CONTENT check: tree[i]'s changed paths must carry T_start's blob. Equiv to
	//     `git diff treeI tStart -- <changedTreeI>` empty, done WITHOUT a pathspec: changedTreeI ∩
	//     DiffTreeNames(treeI, tStart) == ∅ isolates the changed paths whose content differs from T_start.
	delta, err := deps.Git.DiffTreeNames(ctx, treeI, tStart)
	if err != nil {
		return fmt.Errorf("%w: freeze check diff-tree-names[%d]: %w", ErrDecomposeFailed, i, err)
	}
	deltaSet := pathSet(delta)
	var mismatch []string
	for _, p := range changedTreeI {
		if _, ok := deltaSet[p]; ok {
			mismatch = append(mismatch, p)
		}
	}
	if len(mismatch) > 0 {
		return fmt.Errorf("%w: concept %d staged content not traceable to T_start: %s", ErrFreezeViolation, i, strings.Join(mismatch, ", "))
	}
	return nil
}

func pathSet(paths []string) map[string]struct{} {
	s := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		s[p] = struct{}{}
	}
	return s
}

// === Task 3: the runLoop insertion (after freezeSnapshot, before the empty-skip check) ===
		treeI, err := freezeSnapshot(ctx, deps)
		if err != nil {
			drainMsg(inflight)
			return commits, nil, fmt.Errorf("%w: freeze snapshot[%d]: %w", ErrDecomposeFailed, i, err)
		}
		// FR-M1c freeze enforcement: verify tree[i] is a content-subset of T_start. The orchestrator owns
		// the freeze boundary (the external stager is NOT trusted). NON-RESCUE on violation: drain the
		// in-flight message + return the partial commits that already landed (mirrors ErrStagerMovedHEAD).
		if vErr := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, treeI); vErr != nil {
			drainMsg(inflight)
			return commits, nil, vErr
		}
		skipped := treeI == prevTree // FR-M8 empty-skip (verify passed trivially on an empty changedTreeI)

// CRITICAL: tStartPaths is computed ONCE before the loop:
//   tStartPaths, err := deps.Git.DiffTreeNames(ctx, baseTree, tStart)
//   if err != nil { return nil, nil, fmt.Errorf("%w: freeze baseline diff-tree-names: %w", ErrDecomposeFailed, err) }

// CRITICAL: the runLoop call site gains tStart (after baseTree):
//   commits, chainData, err := runLoop(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)
```

### Integration Points

```yaml
LOOP (internal/decompose/decompose.go runLoop):
  - signature: "+tStart string (after baseTree)"
  - once-before-loop: "tStartPaths, err := deps.Git.DiffTreeNames(ctx, baseTree, tStart) (ErrDecomposeFailed wrap)"
  - body: "after freezeSnapshot + before `skipped := treeI == prevTree`: verifyFreezeSubset(...); on err
           drainMsg(inflight) + return commits, nil, err"
  - call site (decompose.go:185): "+tStart (after baseTree)"

FREEZE LAYER (internal/decompose/stager.go):
  - add: "var ErrFreezeViolation = errors.New(...) (after ErrStagerMovedHEAD)"
  - add: "func verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths []string, i int, treeI) error
          (after freezeSnapshot) + func pathSet"

NO DATABASE / NO CONFIG-FILE / NO ROUTE / NO git.go changes. go.mod/go.sum UNCHANGED.
arbiter staging (chain.go) is OUT OF SCOPE (findings §8) — documented as a conscious decision.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after each file creation/modification — fix before proceeding.
gofmt -w internal/decompose/stager.go internal/decompose/decompose.go \
  internal/decompose/stager_test.go internal/decompose/decompose_test.go
go vet ./internal/decompose/...
go build ./...

# Expected: zero errors. Likely failures: a runLoop `err` shadow/redeclare (fix per the actual setup
# block — use `:=` only with a new var on the left, else `=`); a missing "strings" import in stager.go.
```

### Level 2: Unit Tests (Component Validation)

```bash
# verifyFreezeSubset unit tests (real git via git.New).
go test -race ./internal/decompose/ -run VerifyFreezeSubset -v

# Expected: Happy ⇒ nil; PathViolation ⇒ ErrFreezeViolation + "sentinel.txt"/"not present in T_start";
# ContentViolation ⇒ ErrFreezeViolation + "a.txt"/"not traceable to T_start"; EmptyStaging ⇒ nil.
# If Happy false-positives, re-check findings §4 (the content intersection) — a well-behaved treeI's
# changedTreeI must NOT intersect DiffTreeNames(treeI, tStart).
```

### Level 3: Integration (Loop + No-Regressions Validation)

```bash
# The rogue-stager integration test + the happy-path regression (NO false positive).
go test -race ./internal/decompose/ -run "StagerFreeze|StagerGuard|StagerMoved|Decompose" -v
# The WHOLE decompose package (esp. the dcmStagerSeam happy-path tests must still pass).
go test -race ./internal/decompose/
# Full module regression.
go test ./...
go vet ./...
golangci-lint run
gofmt -l internal/ pkg/   # MUST be empty

# Confirm scope: exactly 4 modified files, go.mod/go.sum untouched, git.go untouched.
git status --short
git diff --stat            # expect stager.go + decompose.go + stager_test.go + decompose_test.go only

# Expected: the rogue-sentinel stager → ErrFreezeViolation (HEAD unchanged, sentinel in no commit); every
# well-behaved-stager test still GREEN (no false positive); no other package breaks.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# (This task is pure tree-subset verification — no agent runs, no network. Level 4 is a design-coherence +
# invariant check, not a runtime check.)

# Confirm the §20.2 'start-of-run freeze' invariant holds for the loop path: a post-freeze sentinel can
# NEVER enter a commit. (The TestDecompose_StagerFreezeViolation test IS this check for the stager axis.)
go test -race ./internal/decompose/ -run "StagerFreeze" -v

# Confirm the sentinel + ErrStagerMovedHEAD sentinels are both defined + distinct (the two guard axes).
rg -n "ErrFreezeViolation|ErrStagerMovedHEAD" internal/decompose/stager.go

# Expected: both sentinels present; the freeze test proves a post-freeze file cannot enter a commit via
# the stager; the happy-path tests prove legitimate staging is never false-positive'd.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go build ./...` succeeds.
- [ ] `go test ./...` GREEN (new tests pass + NO existing test regressed — esp. the happy-path tests).
- [ ] `go vet ./...` clean.
- [ ] `golangci-lint run` clean (errcheck on DiffTreeNames; no unused pathSet/verifyFreezeSubset; no `err` shadow bug).
- [ ] `gofmt -l internal/ pkg/` empty.
- [ ] go.mod / go.sum UNCHANGED.

### Feature Validation

- [ ] All success criteria from "What" met (ErrFreezeViolation + verifyFreezeSubset + runLoop wiring).
- [ ] verifyFreezeSubset: nil for a subset; ErrFreezeViolation for path-not-in-T_start AND content-mismatch.
- [ ] runLoop calls verifyFreezeSubset after every freezeSnapshot; on violation drainMsg + return partial.
- [ ] Rogue-sentinel stager → ErrFreezeViolation; HEAD unchanged; sentinel in no commit.
- [ ] Well-behaved stager → no false positive (existing happy-path tests GREEN).
- [ ] git.go UNCHANGED (DiffTreeNames consumed as-is — no new primitive).

### Code Quality Validation

- [ ] Mirrors ErrStagerMovedHEAD (sentinel location, non-rescue doc, %w wrap, errors.Is test shape).
- [ ] File placement matches the desired tree (stager.go sentinel+helper; decompose.go runLoop wiring).
- [ ] Anti-patterns avoided (no new git primitive — the intersection trick; no false positive on empty
      staging; no rescue recipe for a non-rescue error; verify BEFORE the empty-skip check).
- [ ] Doc comments cite FR-M1c (the orchestrator owns the freeze boundary, not the stager).

### Documentation & Deployment

- [ ] stager.go ErrFreezeViolation + verifyFreezeSubset doc comments cite PRD §9.14/§13.6.1 FR-M1c.
- [ ] decompose.go runLoop doc + the freeze-boundary note mention FR-M1c enforcement (loop verifies
      tree[i] ⊆ T_start after each staging step).
- [ ] The arbiter-staging scope decision (out of scope; the contract point 3 names the loop) is documented
      as a conscious decision, not an oversight.
- [ ] No new env vars / config keys (this task is pure enforcement over existing T_start + DiffTreeNames).

---

## Anti-Patterns to Avoid

- ❌ Don't add a new git primitive for the content check — the `DiffTreeNames` intersection trick
  (`changedTreeI ∩ DiffTreeNames(treeI, tStart) == ∅`) IS the path-restricted content check; the git.Git
  interface has no pathspec-restricted diff, and none is needed.
- ❌ Don't insert the verify call AFTER `skipped := treeI == prevTree` or after `publish` — it must run
  right after freezeSnapshot (before the empty-skip check), so an empty-staging tree is not a false
  positive and a violation aborts before any commit.
- ❌ Don't make ErrFreezeViolation a *generate.RescueError — it is NON-RESCUE (caught at tree[i] before its
  commit; no snapshot-then-CAS to rescue). Mirror ErrStagerMovedHEAD, not the rescue path.
- ❌ Don't recompute tStartPaths per concept — it is invariant; compute it ONCE before the loop.
- ❌ Don't re-capture T_start inside runLoop — thread the one Decompose() captured (add the `tStart` param).
- ❌ Don't false-positive the happy path — a well-behaved stager's treeI has changedTreeI ⊆ tStartPaths
  with matching blobs; verifyFreezeSubset MUST return nil (the existing tests prove it).
- ❌ Don't skip `drainMsg(inflight)` on violation — the in-flight message goroutine would leak; mirror the
  HEAD guard + freezeSnapshot-error paths exactly.
- ❌ Don't harden the arbiter staging (chain.go) in THIS task — the contract (point 3) scopes it to the
  loop (the external stager). Document the arbiter as a separate freeze surface (findings §8).
- ❌ Don't swallow DiffTreeNames errors (errcheck) — wrap them with ErrDecomposeFailed + the concept index.

---

## Confidence Score

**9/10** — one-pass success is highly likely. The task is the content-axis twin of the ALREADY-IMPLEMENTED
ErrStagerMovedHEAD guard (same sentinel location, same drainMsg+return-partial abort, same errors.Is test
shape). The one non-trivial decision — expressing the content check without a new git primitive — is
fully resolved: the `DiffTreeNames` intersection (`changedTreeI ∩ DiffTreeNames(treeI, tStart) == ∅`) is
PROVEN (findings §4/§5) equivalent to the contract's path-restricted `git diff treeI tStart -- <paths>`,
and the path check is a cheap set-subset on a once-per-run baseline. T_start is already threaded on disk
(P3.M1.T1.S2); runLoop just needs the `+tStart` param. No new files, no git.go change, no new deps → tiny
blast radius. The residual risk (the `err` shadowing in runLoop's setup block) is mechanical and caught by
vet/build. The arbiter-staging scope is a documented conscious decision (contract point 3 names the loop).
