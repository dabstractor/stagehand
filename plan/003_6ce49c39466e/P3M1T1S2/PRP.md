---
name: "P3.M1.T1.S2 — Wire T_start into the Decompose orchestrator (planner/stagers/arbiter/shortcuts draw from T_start) (PRD §13.6.1 FR-M1b, §9.14 FR-M1b)"
description: |

  MODIFY the decompose orchestrator so the planner, the FR-M11 single-shortcut, and the arbiter's
  leftover diff draw from a FROZEN `T_start` (the working-tree change set captured at run start) instead
  of live working-tree re-reads. This is the T_start WIRING half of the v2 start-of-run freeze (PRD
  §13.6.1 FR-M1b). It consumes P3.M1.T1.S1's `FreezeWorkingTree` primitive + the EXISTING `TreeDiff`.
  The freeze ENFORCEMENT (FR-M1c subset check) is a SEPARATE next task (P3.M2.T1.S1) — THIS task only
  captures T_start and threads it into the diff INPUTS + the shortcut.

  CONTRACT (P3.M1.T1.S2, verbatim from the work item):
    1. RESEARCH NOTE: scout_decompose_freeze.md §(b). Insert T_start capture AFTER baseTree/preRunHEAD
       derivation (decompose.go) and BEFORE callPlanner. baseTree (HEAD^{tree}) ≠ T_start (the working-tree
       change set) — KEEP prevTree:=baseTree unchanged. The planner currently diffs via WorkingTreeDiff.
       runSingleShortcut currently does AddAll→WriteTree.
    2. INPUT: P3.M1.T1.S1 (FreezeWorkingTree/DiffTreeNames) + P1.M2.T1.S2 (decompose callers use v3 render).
       Decompose (decompose.go), callPlanner (planner.go), runLoop (decompose.go), runSingleShortcut
       (decompose.go), runArbiterPhase (decompose.go, WorkingTreeDiff at the leftover-diff call).
    3. LOGIC: (a) Capture T_start immediately after baseTree/preRunHEAD derivation (FreezeWorkingTree);
       the index is left clean (reset to baseTree) so the per-concept stager starts clean. (b) The planner
       diffs T_start (a tree-to-tree diff: baseTree → T_start, with binary placeholders per FR3c) — NOT a
       fresh WorkingTreeDiff of the live tree. (c) Per-concept loop: stagers stage content drawn strictly
       from T_start (the working tree is the frozen snapshot's source; the freeze invariant ensures no
       concurrent changes reach it). (d) runSingleShortcut and the arbiter's leftover staging draw from
       T_start, not a live re-read. Thread T_start + baseTree through Deps/closures as needed.
    4. OUTPUT: a decompose run commits EXACTLY the working-tree state as it existed when the run began;
       files created/modified after T_start is captured are invisible to every commit.
    5. DOCS: [Mode A] decompose.go doc comment documenting the T_start freeze boundary (FR-M1b: the run
       owns the freeze, not the stager).

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - stager.go (stageConcept, freezeSnapshot) — UNCHANGED. Stagers run git add against the working tree
      (== T_start's source under the invariant); the freeze ENFORCEMENT (subset check) is P3.M2.T1.S1.
    - message.go (generateMessage, publishCommit) — UNCHANGED. Already tree-to-tree (frozen).
    - chain.go (resolveArbiter + resolveNewCommit/resolveTipAmend/resolveMidChain) — UNCHANGED. Staging
      uses AddAll/Add against the working tree (== T_start's source); hardened by P3.M2.T1.S1.
    - arbiter.go (runArbiter) — UNCHANGED. Takes leftoverDiff as a PARAM (runArbiterPhase passes the frozen diff).
    - roles.go (Deps) — UNCHANGED. T_start/baseTree threaded as function PARAMS, not Deps fields.
    - git.go — UNCHANGED. Consumes FreezeWorkingTree (S1, git.go:1223) + the EXISTING TreeDiff (git.go:1011).
    - runSingleEscape (escape-hatch) — UNCHANGED. Returns BEFORE the freeze (FR-M2c "v1 behavior"; FR-M1b
      omits the escape-hatch from its freeze-consumer enumeration).
    - go.mod / go.sum — UNCHANGED (stdlib only; no new deps).

  DELIVERABLES (2 files MODIFIED + their tests; 0 NEW files):
    MODIFY internal/decompose/decompose.go — (1) Decompose(): capture tStart via FreezeWorkingTree(ctx,
      baseTree) AFTER baseTree derivation, BEFORE callPlanner; pass baseTree+tStart to callPlanner +
      runSingleShortcut, pass tStart to runArbiterPhase. (2) runSingleShortcut(): +tStart param;
      `treePrime := tStart` (remove AddAll+WriteTree). (3) runArbiterPhase(): +tStart param;
      TreeDiff(tipTree, tStart) replaces WorkingTreeDiff. (4) doc comments: the freeze boundary (FR-M1b).
    MODIFY internal/decompose/planner.go — callPlanner(): +baseTree,+tStart params; TreeDiff(baseTree,
      tStart) replaces WorkingTreeDiff.
    MODIFY internal/decompose/planner_test.go — update ~12 callPlanner call sites (+baseTree,+tStart) +
      add a freezeForPlanner helper; add the callPlanner-diffs-tStart exclusion test.
    MODIFY internal/decompose/decompose_test.go — update any direct callPlanner call sites; add the
      Decompose-level sentinel test (§20.2 "Start-of-run freeze (v2)").

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing decompose tests still pass
  (the frozen diffs are byte-identical to the live diffs under the no-concurrent-change invariant); a
  post-freeze working-tree change is invisible to the planner's diff payload, the shortcut's commit, and
  the arbiter's leftover diff (exclusion tests pass).

---

## Goal

**Feature Goal**: Wire the v2 start-of-run working-tree freeze (PRD §13.6.1 FR-M1b) into the decompose
orchestrator so the planner, the FR-M11 single-shortcut, and the arbiter's leftover diff all draw from
a frozen `T_start` (the working-tree change set captured at run start) instead of live working-tree
re-reads. This makes a decompose run commit EXACTLY the working-tree state as it existed when the run
began: a file a concurrent process (editor save, another agent) writes DURING the (potentially long) run
is invisible to every commit. The freeze is captured via P3.M1.T1.S1's `FreezeWorkingTree` (AddAll →
WriteTree → ReadTree(baseTree)); the index is left clean (== baseTree) so the per-concept stager starts
clean. The planner/shortcut/arbiter then read frozen `TreeDiff`s (the EXISTING git.go:1011 method), not
live `WorkingTreeDiff`s.

**Deliverable** (2 files MODIFIED + their tests; 0 NEW files):
1. `internal/decompose/decompose.go` — `Decompose()` gains the tStart capture (`FreezeWorkingTree`) after
   baseTree derivation + threads baseTree/tStart to callPlanner/runSingleShortcut and tStart to
   runArbiterPhase; `runSingleShortcut()` replaces its `AddAll → WriteTree` with `treePrime := tStart`;
   `runArbiterPhase()` replaces its `WorkingTreeDiff` with `TreeDiff(chainData[last].Tree, tStart)`;
   doc comments document the freeze boundary (FR-M1b: the run owns the freeze, not the stager).
2. `internal/decompose/planner.go` — `callPlanner()` gains baseTree+tStart params; its `WorkingTreeDiff`
   is replaced with `TreeDiff(baseTree, tStart)`.
3. `internal/decompose/planner_test.go` + `internal/decompose/decompose_test.go` — call-site updates +
   the freeze-exclusion tests.

**Success Definition**:
- `Decompose()` calls `deps.Git.FreezeWorkingTree(ctx, baseTree)` exactly once, AFTER baseTree/preRunHEAD
  derivation and BEFORE callPlanner; the result `tStart` is threaded to callPlanner (+baseTree),
  runSingleShortcut (+baseTree), and runArbiterPhase.
- `callPlanner` reads `TreeDiff(baseTree, tStart, opts)` — NOT `WorkingTreeDiff`. A file written to the
  working tree AFTER the freeze is absent from the planner's diff payload.
- `runSingleShortcut` commits `tStart` directly (`treePrime := tStart`) — NOT a live `AddAll → WriteTree`.
  A post-freeze file is absent from the shortcut's commit tree.
- `runArbiterPhase` reads `TreeDiff(chainData[last].Tree, tStart, opts)` — NOT `WorkingTreeDiff`. A
  post-freeze file is absent from the arbiter's leftover-diff payload.
- `runLoop`'s `prevTree := baseTree` is UNCHANGED; `runSingleEscape` (escape-hatch) is UNCHANGED.
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED.

## User Persona

**Target User**: the end user running `stagecoach` on an un-staged working tree to get multiple
logically-coherent commits, while another tool (an editor auto-save, a concurrent coding agent) may also
be writing files. The freeze is NOT a user-facing flag; it is an internal safety guarantee.

**Use Case**: a long decompose run (multiple agent round-trips — planner, N stagers, N message gens,
arbiter) can take tens of seconds. Without the freeze, a file saved mid-run could be swept into a commit
the user never intended. With the freeze, the run commits EXACTLY the working-tree state at run start;
the concurrent save is left untouched in the working tree for the user to handle separately.

**Pain Points Addressed**: the concurrency hole in v2 decompose — every diff (planner input, arbiter
leftovers) and the single-shortcut's commit was a LIVE read of the working tree, so a mid-run change
could leak into a commit. The freeze closes that hole for the diff INPUTS + the shortcut. (The stager's
live staging is hardened by the ENFORCEMENT task P3.M2.T1.S1.)

## Why

- **Closes the wiring half of PRD §13.6.1 FR-M1b.** FR-M1b: "the first action on activation is to freeze
  the entire working-tree change set into T_start ... The planner partitions T_start's diff (never a
  fresh re-read of the live tree), and every stager, the arbiter's leftover staging, the one-file
  short-circuit (FR-M2b), and the single shortcuts (FR-M11) stage and commit content drawn strictly from
  T_start." This task is the literal wiring: capture T_start + replace the 3 live reads (planner diff,
  shortcut commit, arbiter leftover diff) with frozen tree-to-tree diffs. (The one-file short-circuit
  FR-M2b is P3.M2.T1.S2; the stager/arbiter staging enforcement is P3.M2.T1.S1.)
- **Minimal, localized, behaviorally-equivalent-under-the-invariant change.** The frozen diffs
  (TreeDiff(baseTree,tStart), TreeDiff(tipTree,tStart)) produce the SAME bytes as the live diffs
  (WorkingTreeDiff) when no concurrent process modifies the working tree — so all existing tests pass
  unchanged in content; only the test CALL SITES gain 2 params. The only NEW behavior is the freeze
  capture + the exclusion of post-freeze changes (new tests pin it).
- **High reuse.** FreezeWorkingTree is S1 (done). TreeDiff, EmptyTreeSHA, StagedDiffOptions, ErrDecomposeFailed
  all EXIST. No new types, no new deps, no new files.
- **Unblocks the freeze feature.** P3.M2.T1.S1 (enforcement) consumes T_start via the subset check
  (DiffTreeNames(prevTree, treeI) ⊆ DiffTreeNames(baseTree, tStart)) — which requires T_start to be
  captured + threaded. This task is its prerequisite.

## What

MODIFY 2 production files (`decompose.go`, `planner.go`) + their tests. No new files. No new types. No
new deps. Specifically:

- **`Decompose()` (decompose.go):** after the `baseTree` derivation block (the `if !isUnborn { baseTree,
  err = RevParseTree(ctx, "HEAD") }` block) and BEFORE `callPlanner`, add `tStart, err :=
  deps.Git.FreezeWorkingTree(ctx, baseTree)` (error → `ErrDecomposeFailed` wrap, non-rescue). Then pass
  `baseTree, tStart` to `callPlanner`; `baseTree, tStart` to `runSingleShortcut`; `tStart` to
  `runArbiterPhase`. (The freeze is BELOW the escape-hatch return, so `runSingleEscape` is untouched.)
- **`callPlanner()` (planner.go):** signature `+baseTree, tStart string`; replace
  `deps.Git.WorkingTreeDiff(ctx, opts)` with `deps.Git.TreeDiff(ctx, baseTree, tStart, opts)`; error wrap
  `"working-tree diff"` → `"tree diff"`.
- **`runSingleShortcut()` (decompose.go):** signature `+tStart string`; replace the `AddAll` + `WriteTree`
  block with `treePrime := tStart`; the dup-check fallback becomes `generateMessage(ctx, deps, baseTree,
  tStart)`.
- **`runArbiterPhase()` (decompose.go):** signature `+tStart string`; replace
  `deps.Git.WorkingTreeDiff(ctx, opts)` with `tipTree := chainData[len(chainData)-1].Tree;
  deps.Git.TreeDiff(ctx, tipTree, tStart, opts)`.
- **Doc comments:** `Decompose()` + the package doc document the freeze boundary (FR-M1b: the run owns
  the freeze, not the stager); callPlanner/runSingleShortcut/runArbiterPhase note they read the frozen
  tree-to-tree diff.
- **Tests:** update ~12 `callPlanner` call sites in planner_test.go (+baseTree,+tStart via a
  `freezeForPlanner` helper); update any direct callPlanner call sites in decompose_test.go; add the
  freeze-exclusion tests (callPlanner-diffs-tStart, runSingleShortcut-commits-tStart,
  arbiter-leftover-diff-frozen, + the Decompose-level §20.2 sentinel).

### Success Criteria

- [ ] `Decompose()` captures tStart via `FreezeWorkingTree(ctx, baseTree)` AFTER baseTree derivation and
      BEFORE callPlanner; tStart is threaded to callPlanner (+baseTree), runSingleShortcut (+baseTree),
      and runArbiterPhase.
- [ ] `callPlanner` reads `TreeDiff(baseTree, tStart, opts)` — a post-freeze working-tree file is absent
      from the planner's diff payload (exclusion test passes).
- [ ] `runSingleShortcut` commits tStart directly (`treePrime := tStart`; AddAll+WriteTree removed) — a
      post-freeze file is absent from the shortcut's commit tree (exclusion test passes).
- [ ] `runArbiterPhase` reads `TreeDiff(chainData[last].Tree, tStart, opts)` — a post-freeze file is
      absent from the arbiter's leftover-diff payload (exclusion test passes).
- [ ] `runLoop`'s `prevTree := baseTree` is UNCHANGED; `runSingleEscape` (escape-hatch) is UNCHANGED and
      does NOT freeze (it returns above the freeze insertion point).
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact freeze insertion
point + baseTree≠T_start distinction (findings §1); the index-idempotency-on-failure guarantee (§2); the
3 diff-source swaps with exact before/after code (§3 planner, §4 shortcut, §5 arbiter); the unchanged
files + why (§6); the threading decision (function params, §7); the test impact + the freezeForPlanner
helper + the 4 exclusion tests (§8, §9); the doc-comment requirement (§11); the scope boundaries (§10).
The scout note (§b) anchors every file:line. S1 (FreezeWorkingTree) is done; TreeDiff exists. No external
research needed — this is pure internal wiring.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (exact code changes + the invariant + test strategy)
- docfile: plan/003_6ce49c39466e/P3M1T1S2/research/findings.md
  why: §1 the freeze insertion point (AFTER baseTree derivation, BEFORE callPlanner) + baseTree≠T_start +
       KEEP prevTree:=baseTree + the escape-hatch exclusion; §2 the index-idempotency-on-failure guarantee;
       §3 callPlanner: WorkingTreeDiff→TreeDiff(baseTree,tStart) + signature; §4 runSingleShortcut:
       AddAll→WriteTree REPLACED by treePrime:=tStart + signature; §5 runArbiterPhase:
       WorkingTreeDiff→TreeDiff(tipTree,tStart) + the runArbiter-takes-leftoverDiff-as-param fact + the
       staging-is-UNCHANGED (enforcement is next) fact; §6 the UNCHANGED files (runLoop/stager/message/
       chain/arbiter/roles) + why; §7 threading = function params (not Deps); §8 the test impact
       (~12 callPlanner call sites + the freezeForPlanner helper); §9 the 4 NEW freeze-exclusion tests;
       §10 scope boundaries; §11 doc comments.
  critical: §1 (baseTree≠T_start — prevTree stays baseTree, NOT tStart); §4 (remove AddAll+WriteTree from
            runSingleShortcut — treePrime:=tStart); §5 (runArbiter takes leftoverDiff as a PARAM, so the
            change is localized to runArbiterPhase; tipTree=chainData[last].Tree; the arbiter STAGING is
            unchanged — enforcement is P3.M2.T1.S1); §1 (the escape-hatch does NOT freeze — it returns
            above the insertion point).

# MUST READ — the S1 PRP (the FreezeWorkingTree CONTRACT this task consumes)
- docfile: plan/003_6ce49c39466e/P3M1T1S1/PRP.md
  section: the FreezeWorkingTree interface doc comment + impl (AddAll→WriteTree→ReadTree(baseTree); index
           == baseTree after return; working-tree files UNCHANGED; caller supplies baseTree = HEAD^{tree}
           or EmptyTreeSHA; unborn case baseTree=EmptyTreeSHA → empty index; mutates index, touches NO ref).
  why: this task CALLS FreezeWorkingTree(ctx, baseTree) — its contract (return tStart; reset index to
       baseTree; leave the working tree unchanged) IS the foundation. The §2 index-idempotency argument
       (findings) follows directly from FreezeWorkingTree's ReadTree(baseTree) reset == the pre-freeze
       index state (nothing staged ⇒ index == baseTree).
  critical: FreezeWorkingTree is ALREADY IMPLEMENTED (git.go:1223) — do NOT re-implement it; just CALL it.
            DiffTreeNames (also S1) is NOT used by this task (it's the P3.M2.T1.S1 enforcement primitive).

# MUST READ — the scout research note (the authoritative anchor map)
- docfile: plan/003_6ce49c39466e/architecture/scout_decompose_freeze.md
  section: §(b) the T_start capture insertion point (decompose.go derivation block, before callPlanner) +
           the one-file short-circuit insertion point (P3.M2.T1.S2, NOT this task); §(c) stageConcept/
           freezeSnapshot (stager.go — UNCHANGED); §(d) how the stager reads the working tree (via the
           tooled agent; the orchestrator owns retry + freezeSnapshot); §(e) EmptyTreeSHA + NO reset helper
           (ReadTree IS the reset — used by FreezeWorkingTree internally); the Open Question #3
           (baseTree≠T_start; prevTree stays baseTree).
  why: confirms the exact insertion point + the baseTree≠T_start distinction + that the stager loop is
       unchanged (the stager stages from the working tree == T_start's source). The "Key Dependencies"
       section lists FreezeWorkingTree (S1) + TreeDiff (existing) as this task's inputs.

# MUST READ — the FILES TO MODIFY
- file: internal/decompose/decompose.go
  section: `Decompose()` (the entry point): the mode-routing block (escape-hatch return at ~line 132),
           the baseTree/preRunHEAD derivation block (~lines 140–156), the callPlanner call (~line 150),
           the runSingleShortcut call (~line 157), the runLoop call (UNCHANGED — already takes baseTree),
           the runArbiterPhase call (~line 180); `runSingleShortcut()` (~line 227: AddAll→WriteTree→dup-
           check→publish); `runArbiterPhase()` (~line 465: WorkingTreeDiff→runArbiter→resolveArbiter).
  why: ALL three modifications live here (freeze capture + the shortcut body + the arbiter diff). The
       doc comment at the top + on Decompose() get the freeze-boundary note.
  pattern: insert `tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)` after the baseTree derivation,
           error-wrapped `fmt.Errorf("%w: freeze working tree: %w", ErrDecomposeFailed, err)` (non-rescue,
           mirrors the surrounding baseTree-derivation errors). Thread baseTree+tStart as trailing params.
  gotcha: the escape-hatch (`runSingleEscape`) returns ABOVE the insertion point → it is NOT modified and
          does NOT freeze (FR-M2c "v1 behavior"). runLoop is UNCHANGED (prevTree:=baseTree stays).

- file: internal/decompose/planner.go
  section: `callPlanner()` signature (~line 59) + the WorkingTreeDiff call (~line 65).
  why: the planner's diff SOURCE changes from live WorkingTreeDiff to frozen TreeDiff(baseTree, tStart).
  pattern: signature `func callPlanner(ctx, deps, forcedCount int, isUnborn bool, baseTree, tStart string)`;
           replace `deps.Git.WorkingTreeDiff(ctx, opts)` with `deps.Git.TreeDiff(ctx, baseTree, tStart, opts)`;
           error wrap "working-tree diff" → "tree diff". TreeDiff takes the SAME opts struct (MaxDiffBytes/
           MaxMDLines/BinaryExtensions) — binary placeholders per FR3c are preserved (TreeDiff applies them).
  gotcha: do NOT re-derive baseTree inside callPlanner (the caller already derived it — re-deriving risks a
          HEAD-move race). do NOT add tStart to the RetryInstruction path (the retry re-Renders with the
          SAME payload — the diff is captured ONCE before the retry loop).

# MUST READ — the UNCHANGED files (scope boundary — read to confirm they need NO change)
- file: internal/decompose/stager.go
  section: stageConcept (~line 58: builds the §17.6 task; the tooled agent runs git add against the working
           tree); freezeSnapshot (~line 108: thin WriteTree wrapper).
  why: UNCHANGED. The stager stages from the working tree == T_start's source (the freeze left the working
           tree untouched). The freeze ENFORCEMENT (subset check after each staging step) is P3.M2.T1.S1.
- file: internal/decompose/message.go
  section: generateMessage (~line 53: already uses TreeDiff(treeA, treeB) — frozen); publishCommit.
  why: UNCHANGED. generateMessage is already tree-to-tree (never index-vs-HEAD). The shortcut's fallback
       call becomes generateMessage(ctx, deps, baseTree, tStart) — baseTree→tStart = the whole change set.
- file: internal/decompose/chain.go
  section: resolveArbiter + resolveNewCommit (AddAll→WriteTree) + resolveTipAmend (AddAll→WriteTree) +
           resolveMidChain (ReadTree+Add per commit).
  why: UNCHANGED. The arbiter's STAGING uses AddAll/Add against the working tree (== T_start's source).
           The freeze OUTPUT guarantee for these paths is the ENFORCEMENT task (P3.M2.T1.S1, FR-M1c).
- file: internal/decompose/arbiter.go
  section: runArbiter (~line 79: takes `leftoverDiff string` as a PARAMETER — does NOT compute the diff).
  why: UNCHANGED. This is WHY the arbiter diff change is localized to runArbiterPhase (runArbiter just
           receives whatever frozen diff runArbiterPhase passes).
- file: internal/decompose/roles.go
  section: the Deps struct (~line 54).
  why: UNCHANGED. T_start/baseTree are threaded as function PARAMS (matches the existing baseTree
           param pattern), NOT Deps fields. Deps is for collaborators + test injection, not per-run state.

# MUST READ — the git primitives this task consumes (UNCHANGED)
- file: internal/git/git.go
  section: FreezeWorkingTree interface (~line 211) + impl (~line 1223); TreeDiff (~line 1011);
           EmptyTreeSHA (~line 500); StagedDiffOptions (~line 23).
  why: FreezeWorkingTree (S1, DONE) captures tStart + resets the index to baseTree. TreeDiff (existing)
           produces the frozen tree-to-tree diff with binary placeholders. Both are CALLED, not modified.

# MUST READ — the test exemplars (the patterns to mirror)
- file: internal/decompose/planner_test.go
  section: the fixture helpers (initRepo/writeFile/commitRaw/runGit/plannerDeps) — REUSE; the ~12
           callPlanner call sites (lines 84,112,138,158,183,206,227,258,301,324,348,382) — EACH must add
           baseTree+tStart; the stubtest.Manifest/NewScript stub-agent pattern (the stub records nothing
           by default — for the diff-exclusion test use a stub that captures stdin).
  why: the call-site updates + the freezeForPlanner helper + the callPlanner-diffs-tStart exclusion test.
  gotcha: do NOT redefine initRepo/writeFile/commitRaw/runGit/plannerDeps (duplicate-symbol error). For the
          exclusion test, the stub agent's payload arrives via stdin (bare mode) — capture it to assert the
          sentinel is absent.
- file: internal/decompose/decompose_test.go
  section: the stager seam `dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})` (stages SPECIFIC
           paths — a well-behaved stager; lines 411,550,627,733,776,871,897); the Decompose() end-to-end
           test shape (stub planner + stub stager + temp repo).
  why: the Decompose-level sentinel test (§20.2) — inject a stager that writes a sentinel file on its
           FIRST invocation (simulating a concurrent change mid-run) but stages only the concept path;
           assert the sentinel appears in NO commit. dcmStagerSeam stages specific paths, so the sentinel
           (a different path) is naturally excluded — THIS task's freeze guarantee (the planner diffed
           tStart, so the sentinel is not even a concept).

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §13.6.1 (FR-M1b — the start-of-run freeze; "the first action on activation is to freeze
       the entire working-tree change set into T_start") + §9.14 FR-M1b (the full freeze contract) +
       §9.14 FR-M2c (the escape-hatch = "v1 behavior", excluded from the freeze).
  why: FR-M1b mandates the freeze + enumerates its consumers (planner/stagers/arbiter/FR-M2b/FR-M11 — NOT
       the escape-hatch). §20.2 "Start-of-run freeze (v2)" defines the sentinel test. FR3c mandates binary
       placeholders in EVERY diff path (TreeDiff applies them — preserved).
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go            # MODIFY: Decompose() freeze capture + 3 call sites; runSingleShortcut body;
                          #   runArbiterPhase body; doc comments.
  planner.go              # MODIFY: callPlanner signature + diff source (WorkingTreeDiff→TreeDiff).
  stager.go               # READ/UNCHANGED: stageConcept + freezeSnapshot (stagers stage from working tree).
  message.go              # READ/UNCHANGED: generateMessage (already TreeDiff) + publishCommit.
  chain.go                # READ/UNCHANGED: resolveArbiter staging (AddAll/Add against working tree).
  arbiter.go              # READ/UNCHANGED: runArbiter takes leftoverDiff as a param.
  roles.go                # READ/UNCHANGED: Deps (T_start threaded as params, not a field).
  planner_test.go         # MODIFY: ~12 callPlanner call sites + freezeForPlanner helper + exclusion test.
  decompose_test.go       # MODIFY: direct callPlanner call sites + Decompose-level sentinel test.
internal/git/git.go       # READ/UNCHANGED: FreezeWorkingTree (S1, :1223) + TreeDiff (:1011) + EmptyTreeSHA.
go.mod / go.sum           # UNCHANGED (stdlib only; no new deps).
```

### Desired Codebase tree with files to be modified

```bash
internal/decompose/decompose.go    # MODIFIED — Decompose(): after baseTree derivation, before callPlanner:
                                   #   tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)
                                   #   (ErrDecomposeFailed wrap, non-rescue). Pass baseTree,tStart to
                                   #   callPlanner + runSingleShortcut; pass tStart to runArbiterPhase.
                                   #   runSingleShortcut(): +tStart param; treePrime := tStart (REMOVE
                                   #   AddAll + WriteTree). runArbiterPhase(): +tStart param;
                                   #   tipTree := chainData[len-1].Tree; TreeDiff(tipTree, tStart, opts)
                                   #   (REPLACE WorkingTreeDiff). Doc comments: the freeze boundary (FR-M1b).
internal/decompose/planner.go      # MODIFIED — callPlanner(): +baseTree, tStart params;
                                   #   TreeDiff(ctx, baseTree, tStart, opts) (REPLACE WorkingTreeDiff);
                                   #   error wrap "tree diff".
internal/decompose/planner_test.go # MODIFIED — ~12 callPlanner call sites add baseTree,tStart (via
                                   #   freezeForPlanner helper); + callPlanner-diffs-tStart exclusion test.
internal/decompose/decompose_test.go # MODIFIED — direct callPlanner call sites updated; + the
                                   #   Decompose-level §20.2 sentinel test.
# NO new files. stager.go/message.go/chain.go/arbiter.go/roles.go/git.go UNCHANGED. go.mod/go.sum UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (baseTree ≠ T_start — findings §1): baseTree = HEAD^{tree} (the COMMITTED parent tree);
//   tStart = the working-tree change set staged as a tree (the FROZEN record). They are DIFFERENT objects.
//   runLoop's prevTree := baseTree stays baseTree (it is the message[i] diff base / tree[-1], NOT T_start).
//   If you set prevTree := tStart, concept 0's message would diff tStart→treeI against the wrong base and
//   break the commit chain. KEEP prevTree := baseTree.

// CRITICAL (the escape-hatch does NOT freeze — findings §1): runSingleEscape (Config.Single || Commits==1)
//   returns at the TOP of Decompose(), ABOVE the baseTree derivation + the freeze insertion point. It is
//   FR-M2c "v1 behavior (git add -A → CommitStaged)" and is OMITTED from FR-M1b's freeze-consumer list.
//   Do NOT move the freeze above the escape-hatch return. Do NOT add tStart to runSingleEscape.

// CRITICAL (the freeze is INDEX-IDEMPOTENT on failure — findings §2): before Decompose, nothing is staged
//   (FR-M1) ⇒ index == HEAD.tree == baseTree. FreezeWorkingTree does AddAll→WriteTree→ReadTree(baseTree) ⇒
//   index == baseTree again (net zero). So a non-rescue failure after the freeze (e.g. planner fails)
//   leaves the index byte-identical to the start + the working tree unchanged (user's changes still
//   unstaged). The only artifact is a dangling tStart tree object (harmless; gc'd). §18.1 holds. Do NOT
//   add index-restore-on-failure logic — the net-zero reset already guarantees it.

// CRITICAL (runSingleShortcut: REMOVE AddAll+WriteTree — findings §4): replace them with treePrime := tStart.
//   The whole point of the freeze is that the shortcut commits the FROZEN change set, not a live AddAll
//   (which would sweep in concurrent changes). publishCommit takes the tree SHA directly (CommitTree does
//   not read the index), so committing tStart works regardless of the live index. baseTree is still needed
//   for the dup-check fallback's generateMessage(ctx, deps, baseTree, tStart).

// CRITICAL (runArbiterPhase: the arbiter STAGING is UNCHANGED — findings §5): ONLY the leftover DIFF
//   source changes (WorkingTreeDiff → TreeDiff(tipTree, tStart)). resolveArbiter's AddAll/Add (chain.go)
//   still stage from the working tree (== T_start's source under the invariant). The freeze OUTPUT
//   guarantee for the staging path is the ENFORCEMENT task (P3.M2.T1.S1, FR-M1c subset check). Do NOT
//   modify chain.go in this task.

// GOTCHA (runArbiter takes leftoverDiff as a PARAM — findings §5): runArbiter (arbiter.go:79) does NOT
//   compute the diff — runArbiterPhase pre-computes it and passes it in. So the diff-source change is
//   LOCALIZED to runArbiterPhase (one call site). runArbiter + resolveArbiter are UNCHANGED.

// GOTCHA (tipTree = chainData[last].Tree — findings §5): after the loop, HEAD.tree == the last published
//   commit's tree == chainData[len(chainData)-1].Tree. runArbiterPhase is only called when len(commits)>0
//   (decompose.go:187) and chainData is parallel to commits (same length) ⇒ chainData is non-empty ⇒
//   chainData[last] is valid. Do NOT RevParseTree("HEAD") (an extra git call; chainData already has it).

// GOTCHA (threading = function PARAMS, not Deps — findings §7): baseTree is ALREADY a bare function param
//   in runSingleShortcut/runLoop. Thread baseTree+tStart as trailing params to callPlanner/runSingleShortcut,
//   and tStart to runArbiterPhase. Do NOT add T_start to Deps (per-run state; empty/meaningless in the
//   escape-hatch path; Deps is for collaborators + test injection). runLoop is UNCHANGED (already takes
//   baseTree; does not need tStart).

// GOTCHA (TreeDiff applies binary placeholders per FR3c — findings §3): TreeDiff takes the SAME
//   StagedDiffOptions (MaxDiffBytes/MaxMDLines/BinaryExtensions) WorkingTreeDiff takes. Swapping
//   WorkingTreeDiff → TreeDiff preserves binary filtering. Pass the SAME opts the current calls use.

// GOTCHA (the frozen diffs are byte-identical to the live diffs under the no-concurrent-change invariant
//   — findings §8): so existing tests pass UNCHANGED in CONTENT; only the test CALL SITES gain 2 params
//   (baseTree, tStart). Do NOT rewrite the test assertions — just add the params + the freezeForPlanner
//   helper. The NEW behavior (post-freeze exclusion) gets NEW tests.

// GOTCHA (the planner retry re-Renders with the SAME diff — findings §3): callPlanner captures the diff
//   ONCE (before its retry loop) and reuses it across attempts. Do NOT re-capture tStart or re-TreeDiff
//   inside the retry loop. tStart is captured once in Decompose() and passed in.

// GOTCHA (DiffTreeNames is NOT this task — findings §10): S1 added BOTH FreezeWorkingTree AND
//   DiffTreeNames. This task uses FreezeWorkingTree ONLY. DiffTreeNames is the P3.M2.T1.S1 enforcement
//   primitive (the subset check). Do NOT wire DiffTreeNames into the loop in this task.
```

## Implementation Blueprint

### Data models and structure

No new types. No new constants. No new structs. This task consumes EXISTING primitives unchanged:

```go
// from internal/git/git.go (CONSUME — do not modify):
//   FreezeWorkingTree(ctx, baseTree string) (tStart string, err error)   // S1, git.go:1223 — AddAll→WriteTree→ReadTree(baseTree)
//   TreeDiff(ctx, treeA, treeB string, opts StagedDiffOptions) (string, error)  // git.go:1011 — frozen tree-to-tree diff w/ binary placeholders
//   EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"            // git.go:500
//   StagedDiffOptions{MaxDiffBytes, MaxMDLines, Excludes, BinaryExtensions}     // git.go:23
// from internal/decompose/decompose.go (CONSUME): ErrDecomposeFailed, DecomposeResult, runLoop, runSingleEscape,
//   buildArbiterCommits, rereadFinalCommits (all UNCHANGED).
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/decompose/planner.go — callPlanner signature + frozen diff
  - CHANGE the signature from `func callPlanner(ctx context.Context, deps Deps, forcedCount int, isUnborn
    bool) (prompt.PlannerOutput, error)` to add trailing `baseTree, tStart string`:
      func callPlanner(ctx context.Context, deps Deps, forcedCount int, isUnborn bool, baseTree, tStart string) (prompt.PlannerOutput, error)
  - REPLACE the WorkingTreeDiff call:
      // OLD:
      diff, err := deps.Git.WorkingTreeDiff(ctx, git.StagedDiffOptions{...})
      if err != nil { return prompt.PlannerOutput{}, fmt.Errorf("%w: working-tree diff: %w", ErrPlannerFailed, err) }
      // NEW:
      diff, err := deps.Git.TreeDiff(ctx, baseTree, tStart, git.StagedDiffOptions{
          MaxDiffBytes:     deps.Config.MaxDiffBytes,
          MaxMDLines:       deps.Config.MaxMdLines,
          BinaryExtensions: deps.Config.BinaryExtensions,
      })
      if err != nil { return prompt.PlannerOutput{}, fmt.Errorf("%w: tree diff: %w", ErrPlannerFailed, err) }
  - UPDATE the callPlanner doc comment: note it now diffs the FROZEN tStart (TreeDiff(baseTree, tStart))
    per FR-M1b, NOT a live WorkingTreeDiff (a concurrent working-tree change after the freeze is invisible).
  - GOTCHA: TreeDiff takes the SAME opts struct — binary placeholders (FR3c) are preserved. The diff is
    captured ONCE before the retry loop; the retry re-Renders with the SAME payload (unchanged).
  - PLACEMENT: internal/decompose/planner.go, the callPlanner function.

Task 2: MODIFY internal/decompose/decompose.go — Decompose() freeze capture + 3 call sites
  - In Decompose(), AFTER the baseTree derivation block (the `if !isUnborn { baseTree, err = deps.Git.
    RevParseTree(ctx, "HEAD"); if err != nil {...} }` block) and BEFORE the callPlanner call, INSERT:
      // (FR-M1b) Freeze the entire working-tree change set into T_start — the immutable record the
      // planner/shortcut/arbiter draw from (NOT a live re-read). baseTree (HEAD^{tree}) ≠ T_start; keep
      // runLoop's prevTree := baseTree unchanged. The index is reset to baseTree so the per-concept stager
      // starts clean; the working tree is untouched (read-tree rewrites .git/index only). The escape-hatch
      // (runSingleEscape) returns above → it preserves v1 behavior and does NOT freeze.
      tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)
      if err != nil {
          return DecomposeResult{}, fmt.Errorf("%w: freeze working tree: %w", ErrDecomposeFailed, err)
      }
    (NOTE: `err` is already declared by the RevParseHEAD/RevParseTree calls above — reuse it, do NOT `:=`
    redeclare; use `tStart, err :=` only if err is NOT already in scope, else `tStart, err =`. Check the
    existing block: `preRunHEAD, isUnborn, err := ...` declares err; `baseTree, err = ...` reuses it. So
    here use `tStart, err = deps.Git.FreezeWorkingTree(...)` — wait, tStart is NEW, so `tStart, err :=` is
    correct IF err is being shadowed; otherwise `var tStart string; tStart, err = ...`. Use the form that
    gofmt+go vet accept — prefer `tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)` if err is in
    scope it will be a redeclare error; in that case use `tStart, ferr := ...` or predeclare. SIMPLEST:
    since `err` is already declared above, write `tStart, err := ` will fail (no new var on left besides
    tStart is fine actually — `:=` requires at least one NEW var on the left; tStart is new, so
    `tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)` IS valid Go — tStart is the new var, err is
    reused). USE `tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)`.)
  - UPDATE the 3 call sites in Decompose():
      // OLD: out, err := callPlanner(ctx, deps, deps.Config.Commits, isUnborn)
      // NEW:
      out, err := callPlanner(ctx, deps, deps.Config.Commits, isUnborn, baseTree, tStart)
      // OLD: return runSingleShortcut(ctx, deps, out.Message, preRunHEAD, isUnborn, baseTree)
      // NEW:
      return runSingleShortcut(ctx, deps, out.Message, preRunHEAD, isUnborn, baseTree, tStart)
      // OLD: amended, err = runArbiterPhase(ctx, deps, arbiterCommits, chainData)
      // NEW:
      amended, err = runArbiterPhase(ctx, deps, arbiterCommits, chainData, tStart)
  - GOTCHA: runLoop's call is UNCHANGED (`runLoop(ctx, deps, out.Commits, baseTree, preRunHEAD, isUnborn)`
    — it already takes baseTree; it does NOT take tStart). runSingleEscape's call is UNCHANGED (above the
    freeze).
  - UPDATE the Decompose() doc comment + the package doc comment (decompose.go top): add the freeze-
    boundary paragraph (FR-M1b: the run owns the freeze, not the stager; the planner/shortcut/arbiter draw
    from T_start; the escape-hatch does NOT freeze; the stager's live staging is hardened by FR-M1c /
    P3.M2.T1.S1).
  - PLACEMENT: internal/decompose/decompose.go, the Decompose() function (after baseTree derivation).

Task 3: MODIFY internal/decompose/decompose.go — runSingleShortcut body (commit tStart directly)
  - CHANGE the signature from `func runSingleShortcut(ctx context.Context, deps Deps, plannerMsg,
    preRunHEAD string, isUnborn bool, baseTree string) (DecomposeResult, error)` to add trailing `tStart string`:
      func runSingleShortcut(ctx context.Context, deps Deps, plannerMsg, preRunHEAD string, isUnborn bool, baseTree, tStart string) (DecomposeResult, error)
  - REPLACE the AddAll + WriteTree block:
      // OLD:
      if err := deps.Git.AddAll(ctx); err != nil {
          return DecomposeResult{}, fmt.Errorf("%w: add -A: %w", ErrDecomposeFailed, err)
      }
      treePrime, err := deps.Git.WriteTree(ctx)
      if err != nil {
          return DecomposeResult{}, fmt.Errorf("%w: write-tree: %w", ErrDecomposeFailed, err)
      }
      msg := plannerMsg
      if dupCheckMessage(ctx, deps, plannerMsg, isUnborn) {
          msg, err = generateMessage(ctx, deps, baseTree, treePrime)
          if err != nil { return DecomposeResult{}, err }
      }
      // NEW:
      // FR-M1b: commit the frozen T_start directly (NOT a live AddAll → WriteTree). The freeze already
      // captured the working-tree change set; a live AddAll would pick up concurrent changes.
      treePrime := tStart
      msg := plannerMsg
      if dupCheckMessage(ctx, deps, plannerMsg, isUnborn) {
          var err error
          msg, err = generateMessage(ctx, deps, baseTree, tStart) // message agent regenerates from baseTree→tStart
          if err != nil { return DecomposeResult{}, err }
      }
  - The publishCommit + buildCommitResult calls AFTER are UNCHANGED (they take treePrime + preRunHEAD).
  - UPDATE the runSingleShortcut doc comment: note it now commits tStart directly (FR-M1b) instead of a
    live AddAll → WriteTree; the dup-check fallback diffs baseTree→tStart (the whole change set).
  - GOTCHA: the `err` variable handling — the OLD code reused `err` from WriteTree; the NEW code declares
    `var err error` inside the `if dupCheckMessage` block (since treePrime:=tStart introduces no error).
    Ensure no unused-variable / shadowing lint error (golangci/vet). The publishCommit call below uses
    `newSHA, err := publishCommit(...)` — that's fine.
  - PLACEMENT: internal/decompose/decompose.go, the runSingleShortcut function.

Task 4: MODIFY internal/decompose/decompose.go — runArbiterPhase body (frozen leftover diff)
  - CHANGE the signature from `func runArbiterPhase(ctx context.Context, deps Deps, commits []CommitInfo,
    chainData []ChainEntry) (int, error)` to add trailing `tStart string`:
      func runArbiterPhase(ctx context.Context, deps Deps, commits []CommitInfo, chainData []ChainEntry, tStart string) (int, error)
  - REPLACE the WorkingTreeDiff call:
      // OLD:
      leftoverDiff, err := deps.Git.WorkingTreeDiff(ctx, git.StagedDiffOptions{...})
      if err != nil { return 0, fmt.Errorf("%w: leftover diff: %w", ErrDecomposeFailed, err) }
      // NEW:
      // FR-M1b: the leftover diff is FROZEN — TreeDiff(tipTree, tStart), not a live WorkingTreeDiff.
      // tipTree = the last committed tree (HEAD.tree post-loop; == chainData[last].Tree). A concurrent
      // working-tree change after the freeze is invisible (it's not in tStart).
      tipTree := chainData[len(chainData)-1].Tree
      leftoverDiff, err := deps.Git.TreeDiff(ctx, tipTree, tStart, git.StagedDiffOptions{
          MaxDiffBytes:     deps.Config.MaxDiffBytes,
          MaxMDLines:       deps.Config.MaxMdLines,
          BinaryExtensions: deps.Config.BinaryExtensions,
      })
      if err != nil { return 0, fmt.Errorf("%w: leftover diff: %w", ErrDecomposeFailed, err) }
  - The runArbiter + computeAmended + resolveArbiter calls AFTER are UNCHANGED (runArbiter takes
    leftoverDiff as a param — it now receives the frozen diff).
  - UPDATE the runArbiterPhase doc comment: note the leftover diff is now frozen (TreeDiff(tipTree,
    tStart), FR-M1b); the arbiter's STAGING (resolveArbiter) is unchanged (stages from the working tree ==
    T_start's source; hardened by P3.M2.T1.S1).
  - GOTCHA: chainData is guaranteed non-empty (runArbiterPhase is called only when len(commits)>0,
    decompose.go:187; chainData is parallel to commits). tipTree = chainData[last].Tree == HEAD.tree.
  - PLACEMENT: internal/decompose/decompose.go, the runArbiterPhase function.

Task 5: MODIFY internal/decompose/planner_test.go — update call sites + freezeForPlanner helper + exclusion test
  - ADD a helper (near the other fixture helpers):
      // freezeForPlanner captures baseTree + tStart for a callPlanner test (matures: rev-parse HEAD^{tree};
      // unborn: EmptyTreeSHA). Mirrors what Decompose() does after baseTree derivation.
      func freezeForPlanner(t *testing.T, repo string, isUnborn bool) (baseTree, tStart string) {
          t.Helper()
          if isUnborn {
              baseTree = git.EmptyTreeSHA
          } else {
              baseTree = runGit(t, repo, "rev-parse", "HEAD^{tree}")
          }
          g := git.New(repo)
          ts, err := g.FreezeWorkingTree(context.Background(), baseTree)
          if err != nil { t.Fatalf("freeze working tree: %v", err) }
          return baseTree, ts
      }
  - UPDATE ALL ~12 callPlanner call sites: insert `baseTree, tStart := freezeForPlanner(t, repo, isUnborn)`
    before each callPlanner, and add `baseTree, tStart` to the call. E.g.:
      // OLD: out, err := callPlanner(context.Background(), deps, 0, false)
      // NEW:
      baseTree, tStart := freezeForPlanner(t, repo, false)
      out, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
    For the unborn test (TestCallPlanner_UnbornNilExamples): `baseTree, tStart := freezeForPlanner(t, repo, true)`.
  - ADD TestCallPlanner_DiffsFrozenTStart (the exclusion test):
      - Setup: repo + commitRaw + writeFile("a.txt", "orig") committed; writeFile("a.txt", "changed")
        (unstaged).
      - baseTree, tStart := freezeForPlanner(t, repo, false).
      - AFTER the freeze: writeFile(t, repo, "sentinel.txt", "concurrent") (simulates a concurrent change).
      - Use a planner stub that CAPTURES its stdin payload (e.g. a small stubtest variant or a custom
        stager seam that writes stdin to a *string / *bytes.Buffer before emitting validMultiJSON).
      - callPlanner(ctx, deps, 0, false, baseTree, tStart).
      - ASSERT the captured payload does NOT contain "sentinel.txt" (the planner diffed the FROZEN tStart,
        which predates the sentinel).
  - GOTCHA: the stub agents (stubtest.Manifest/NewScript) emit canned JSON but do NOT capture stdin by
    default. For the exclusion test, write a tiny stub (or extend stubtest) that copies os.Stdin to a
    buffer then prints the canned JSON. Reference how stubtest.Build works (cmd/stubagent).
  - PLACEMENT: internal/decompose/planner_test.go.

Task 6: MODIFY internal/decompose/decompose_test.go — call sites + Decompose-level sentinel test
  - UPDATE any DIRECT callPlanner call sites (grep: `callPlanner(context` in decompose_test.go) to add
    baseTree, tStart (via freezeForPlanner — note: freezeForPlanner is defined in planner_test.go, SAME
    package `decompose`, so it's visible — do NOT redefine it).
  - ADD TestDecompose_SentinelAfterFreezeExcluded (the §20.2 "Start-of-run freeze (v2)" test):
      - Setup: repo + 2 committed files (a.txt, b.txt); make unstaged changes (modify a.txt, add c.txt).
      - Inject a stub planner returning 2 concepts (c1: a.txt, c2: c.txt).
      - Inject a stager seam that, on its FIRST invocation, writes a sentinel file
        writeFile(t, repo, "sentinel.txt", "concurrent") (simulating a concurrent change mid-run, AFTER the
        freeze — the freeze happens in Decompose before the stager runs), then stages the concept's path
        via git add (use the dcmStagerSeam pattern or a closure wrapping it).
      - Run Decompose(ctx, deps).
      - ASSERT: the sentinel appears in NO produced commit (DiffTree each commit's files; none contain
        sentinel.txt); the sentinel REMAINS in the working tree afterward (untracked, StatusPorcelain).
  - NOTE: this test verifies the freeze for the planner (the sentinel is not a concept — the planner
    diffed tStart) + the well-behaved stager (stages concept paths, not the sentinel). The misbehaving-
    stager case (git add -A sweeping the sentinel) is the ENFORCEMENT task (P3.M2.T1.S1).
  - PLACEMENT: internal/decompose/decompose_test.go.

Task 7: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/decompose/decompose.go internal/decompose/planner.go internal/decompose/planner_test.go internal/decompose/decompose_test.go`
  - `go build ./...`   (whole module compiles — the signature changes + the freeze capture)
  - `go vet ./...`
  - `golangci-lint run ./...`   (errcheck/gosimple/govet/ineffassign/staticcheck/unused — watch for
    unused-variable / shadowing from the runSingleShortcut `var err error` change)
  - `go test -race ./internal/decompose/ -run "CallPlanner|Decompose|Sentinel|Freeze" -v`   (all affected)
  - `go test -race ./internal/decompose/`   (the WHOLE decompose package)
  - `go test ./...`   (FULL regression — no other package breaks)
  - `git diff --exit-code go.mod go.sum` ⇒ empty.
  - `git status --short` ⇒ `M internal/decompose/decompose.go` + `M internal/decompose/planner.go` +
    `M internal/decompose/planner_test.go` + `M internal/decompose/decompose_test.go` (4 modified; NO new
    files; stager.go/message.go/chain.go/arbiter.go/roles.go/git.go UNCHANGED).
```

### Implementation Patterns & Key Details

```go
// === Task 2: the freeze capture in Decompose() (AFTER baseTree derivation, BEFORE callPlanner) ===
	// (FR-M1b) Freeze the entire working-tree change set into T_start — the immutable record the
	// planner/shortcut/arbiter draw from (NOT a live re-read). baseTree (HEAD^{tree}) ≠ T_start; keep
	// runLoop's prevTree := baseTree unchanged. The index is reset to baseTree so the per-concept stager
	// starts clean; the working tree is untouched. The escape-hatch (runSingleEscape) returns above → v1.
	tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)
	if err != nil {
		return DecomposeResult{}, fmt.Errorf("%w: freeze working tree: %w", ErrDecomposeFailed, err)
	}
	// (3) Planner (forcedCount = Config.Commits). NON-RESCUE on error.
	out, err := callPlanner(ctx, deps, deps.Config.Commits, isUnborn, baseTree, tStart)


// === Task 3: runSingleShortcut — commit tStart directly (REMOVE AddAll+WriteTree) ===
func runSingleShortcut(ctx context.Context, deps Deps, plannerMsg, preRunHEAD string, isUnborn bool, baseTree, tStart string) (DecomposeResult, error) {
	// FR-M1b: commit the frozen T_start directly (NOT a live AddAll → WriteTree). The freeze already
	// captured the working-tree change set; a live AddAll would pick up concurrent changes.
	treePrime := tStart
	msg := plannerMsg
	if dupCheckMessage(ctx, deps, plannerMsg, isUnborn) {
		var err error
		msg, err = generateMessage(ctx, deps, baseTree, tStart) // message agent regenerates from baseTree→tStart
		if err != nil {
			return DecomposeResult{}, err
		}
	}
	newSHA, err := publishCommit(ctx, deps, treePrime, preRunHEAD, msg)
	// ... (buildCommitResult + return — UNCHANGED)


// === Task 4: runArbiterPhase — frozen leftover diff (WorkingTreeDiff → TreeDiff(tipTree, tStart)) ===
func runArbiterPhase(ctx context.Context, deps Deps, commits []CommitInfo, chainData []ChainEntry, tStart string) (int, error) {
	// FR-M1b: the leftover diff is FROZEN — TreeDiff(tipTree, tStart), not a live WorkingTreeDiff.
	tipTree := chainData[len(chainData)-1].Tree
	leftoverDiff, err := deps.Git.TreeDiff(ctx, tipTree, tStart, git.StagedDiffOptions{
		MaxDiffBytes:     deps.Config.MaxDiffBytes,
		MaxMDLines:       deps.Config.MaxMdLines,
		BinaryExtensions: deps.Config.BinaryExtensions,
	})
	if err != nil {
		return 0, fmt.Errorf("%w: leftover diff: %w", ErrDecomposeFailed, err)
	}
	out, err := runArbiter(ctx, deps, commits, leftoverDiff) // runArbiter takes leftoverDiff as a param — UNCHANGED
	// ... (computeAmended + resolveArbiter — UNCHANGED)


// === Task 1: callPlanner — frozen planner diff (WorkingTreeDiff → TreeDiff(baseTree, tStart)) ===
func callPlanner(ctx context.Context, deps Deps, forcedCount int, isUnborn bool, baseTree, tStart string) (prompt.PlannerOutput, error) {
	_, mdl, rsn := config.ResolveRoleModel("planner", deps.Config)
	// FR-M1b: the FROZEN concept diff — TreeDiff(baseTree, tStart), with binary placeholders per FR3c.
	// NOT a live WorkingTreeDiff (a concurrent change after the freeze must be invisible to the planner).
	diff, err := deps.Git.TreeDiff(ctx, baseTree, tStart, git.StagedDiffOptions{
		MaxDiffBytes:     deps.Config.MaxDiffBytes,
		MaxMDLines:       deps.Config.MaxMdLines,
		BinaryExtensions: deps.Config.BinaryExtensions,
	})
	if err != nil {
		return prompt.PlannerOutput{}, fmt.Errorf("%w: tree diff: %w", ErrPlannerFailed, err)
	}
	// ... (examples + sysPrompt + basePayload + the retry loop — UNCHANGED)
```

### Integration Points

```yaml
DATABASE:
  - none new. FreezeWorkingTree adds a T_start tree object to the git object store + transiently mutates
    the index (AddAll then ReadTree(baseTree) back — net-zero: index == baseTree before and after). No ref
    moves. The 3 TreeDiff calls are read-only (tree-to-tree diffs).

CONFIG:
  - none. tStart/baseTree are derived at runtime (RevParseTree/FreezeWorkingTree), not from config. The
    StagedDiffOptions (MaxDiffBytes/MaxMDLines/BinaryExtensions) come from deps.Config — UNCHANGED.

ROUTES:
  - none (internal orchestrator wiring; no CLI flag, no public API change). The wiring inside Decompose():
      baseTree = RevParseTree("HEAD") (or EmptyTreeSHA)           # EXISTING derivation
      tStart  = FreezeWorkingTree(ctx, baseTree)                  # NEW — this task (FR-M1b)
      out     = callPlanner(ctx, deps, Commits, isUnborn, baseTree, tStart)   # +baseTree,tStart
      if out.Single { return runSingleShortcut(..., baseTree, tStart) }       # +tStart; commits tStart
      commits, chainData = runLoop(ctx, deps, out.Commits, baseTree, preRunHEAD, isUnborn)  # UNCHANGED
      if status != "" && len(commits) > 0 {
          amended = runArbiterPhase(ctx, deps, arbiterCommits, chainData, tStart)             # +tStart
      }
    The freeze enforcement (FR-M1c subset check) is P3.M2.T1.S1 — it adds DiffTreeNames(prevTree,treeI)
    ⊆ DiffTreeNames(baseTree,tStart) inside runLoop's per-concept step. NOT this task.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/decompose/decompose.go internal/decompose/planner.go internal/decompose/planner_test.go internal/decompose/decompose_test.go
gofmt -l internal/ pkg/          # Expected: empty.

golangci-lint run ./internal/decompose/...
golangci-lint run ./...          # Expected: clean. WATCH: unused-variable / shadowing from the
                                 # runSingleShortcut `var err error` change + the call-site param additions.

go vet ./...                     # Expected: no findings.

# Expected: zero errors. READ any lint/vet output and fix before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The updated planner tests (all ~12 call sites compile + pass with the frozen diff).
go test -race ./internal/decompose/ -run "TestCallPlanner" -v

# The new freeze-exclusion tests.
go test -race ./internal/decompose/ -run "TestCallPlanner_DiffsFrozenTStart" -v
go test -race ./internal/decompose/ -run "Sentinel|Freeze" -v

# Whole decompose package (stager/message/arbiter/chain/roles tests must still pass — UNCHANGED code).
go test -race ./internal/decompose/

# Expected: all pass. The frozen diffs are byte-identical to the live diffs under the no-concurrent-change
# invariant, so the EXISTING test assertions pass UNCHANGED (only call sites gained params). The NEW
# exclusion tests pin the freeze behavior (a post-freeze file is invisible to the planner/shortcut/arbiter).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the whole module (the signature changes compile + link; no existing importer breaks — callPlanner/
# runSingleShortcut/runArbiterPhase are unexported, package-local).
go build ./...

# Full regression.
go test ./...

# Confirm scope (4 modified files; NO new files; the UNCHANGED files untouched).
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged"
git status --short   # Expected: M internal/decompose/decompose.go  M internal/decompose/planner.go
                     #           M internal/decompose/planner_test.go  M internal/decompose/decompose_test.go
# Verify the UNCHANGED files are untouched:
git diff --exit-code internal/decompose/stager.go internal/decompose/message.go internal/decompose/chain.go internal/decompose/arbiter.go internal/decompose/roles.go internal/git/git.go && echo "UNCHANGED files untouched"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# FR-M1b faithfulness self-check — the freeze exclusion (the behavior THIS task adds):
go test -run "TestCallPlanner_DiffsFrozenTStart" -v ./internal/decompose/ && \
  echo "PASS: planner diffs the frozen T_start (post-freeze change invisible)"
go test -run "TestDecompose_SentinelAfterFreezeExcluded" -v ./internal/decompose/ && \
  echo "PASS: a post-freeze sentinel appears in no commit (§20.2 Start-of-run freeze)"

# The index-idempotency spot-check (§18.1): after a planner FAILURE (non-rescue), the index == baseTree
# (the freeze's net-zero reset). Add/confirm a test: callPlanner failure leaves `git diff --cached` empty
# (index == HEAD.tree == baseTree) — the freeze is index-idempotent.

# The escape-hatch spot-check: runSingleEscape (--single) does NOT freeze — it still does its own AddAll.
# Confirm via an existing escape-hatch test (unchanged behavior).

# Expected: all pass. The exclusion tests are the domain-specific validations — they prove the freeze
# makes the planner/shortcut/arbiter reason over the FROZEN T_start, so a concurrent change is excluded.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] All tests pass: `go test ./...` (and specifically `go test -race ./internal/decompose/`).
- [ ] No lint errors: `golangci-lint run ./internal/decompose/...` (and `./...`).
- [ ] No vet errors: `go vet ./...`.
- [ ] No formatting issues: `gofmt -l internal/ pkg/` empty.
- [ ] go.mod / go.sum UNCHANGED (`git diff --exit-code go.mod go.sum` ⇒ empty).

### Feature Validation

- [ ] `Decompose()` captures tStart via `FreezeWorkingTree(ctx, baseTree)` after baseTree derivation,
      before callPlanner; threads it to callPlanner(+baseTree)/runSingleShortcut(+baseTree)/runArbiterPhase.
- [ ] `callPlanner` reads `TreeDiff(baseTree, tStart)` — a post-freeze file is absent from the planner's
      diff payload (TestCallPlanner_DiffsFrozenTStart passes).
- [ ] `runSingleShortcut` commits tStart directly (AddAll+WriteTree REMOVED) — a post-freeze file is
      absent from the shortcut's commit tree.
- [ ] `runArbiterPhase` reads `TreeDiff(chainData[last].Tree, tStart)` — a post-freeze file is absent from
      the arbiter's leftover-diff payload.
- [ ] `runLoop`'s `prevTree := baseTree` UNCHANGED; `runSingleEscape` UNCHANGED (does NOT freeze).
- [ ] The Decompose-level sentinel test (§20.2) passes: a post-freeze file appears in NO commit + remains
      in the working tree.

### Code Quality Validation

- [ ] Follows existing decompose conventions (ErrDecomposeFailed/ErrPlannerFailed wrapping; the two-branch
      error style; trailing-param threading matching the baseTree precedent; rich doc comments citing FR-M1b).
- [ ] Anti-patterns avoided (no T_start in Deps; no re-deriving baseTree inside callPlanner; no re-capture
      in the planner retry loop; no modification of stager/message/chain/arbiter; no escape-hatch freeze).
- [ ] Dependencies: stdlib only; no new internal dep; no new files; no new types.
- [ ] The frozen diffs preserve binary placeholders (FR3c) — TreeDiff takes the SAME StagedDiffOptions.

### Documentation & Deployment

- [ ] `Decompose()` doc comment + package doc document the T_start freeze boundary (FR-M1b: the run owns
      the freeze, not the stager; planner/shortcut/arbiter draw from T_start; escape-hatch does NOT freeze;
      stager staging hardened by FR-M1c / P3.M2.T1.S1).
- [ ] callPlanner/runSingleShortcut/runArbiterPhase doc comments note the frozen tree-to-tree diff.
- [ ] No new environment variables or config.

---

## Anti-Patterns to Avoid

- ❌ Don't set `runLoop`'s `prevTree := tStart` — it MUST stay `baseTree` (prevTree is the message[i] diff
  base / tree[-1] = the committed parent, NOT T_start). baseTree ≠ T_start (findings §1).
- ❌ Don't freeze the escape-hatch (`runSingleEscape`) — it returns ABOVE the insertion point; FR-M2c
  defines it as "v1 behavior" and FR-M1b omits it from the freeze-consumer list.
- ❌ Don't add index-restore-on-failure logic — the freeze's AddAll→ReadTree(baseTree) is net-zero (index
  == baseTree before and after because nothing was staged), so §18.1's idempotent-index holds for free.
- ❌ Don't modify stager.go / message.go / chain.go / arbiter.go / roles.go — the stager loop + arbiter
  staging rely on the working-tree-unchanged invariant; the freeze ENFORCEMENT (subset check) is
  P3.M2.T1.S1. This task only wires tStart into the diff INPUTS + the shortcut.
- ❌ Don't add T_start to the Deps struct — thread it as a function param (matches the existing baseTree
  param pattern; Deps is for collaborators, not per-run state; T_start is empty in the escape-hatch path).
- ❌ Don't re-derive baseTree inside callPlanner (the caller derived it; re-deriving risks a HEAD-move race)
  and don't re-capture tStart inside the planner retry loop (capture ONCE in Decompose; the retry re-Renders
  the SAME payload).
- ❌ Don't use `WorkingTreeDiff` for the arbiter's leftover diff after this task — it's a LIVE read; use
  `TreeDiff(chainData[last].Tree, tStart)` (frozen). And don't `RevParseTree("HEAD")` for tipTree —
  `chainData[len(chainData)-1].Tree` already holds it.
- ❌ Don't wire `DiffTreeNames` (also S1) into the loop — it's the P3.M2.T1.S1 enforcement primitive, NOT
  this task.
- ❌ Don't weaken the freeze-exclusion tests to make them pass — if the sentinel appears in the payload/
  commit, the freeze wiring is wrong (debug: confirm callPlanner uses TreeDiff(baseTree,tStart) and
  runSingleShortcut uses treePrime:=tStart).
- ❌ Don't forget the `runSingleShortcut` `var err error` inside the dup-check block — removing AddAll+
  WriteTree removes the `err` they declared; redeclare it locally to avoid unused/shadow lint errors.

---

**Confidence Score: 8/10** — This is a localized, behaviorally-equivalent-under-the-invariant wiring change
(1 new call + 3 diff-source swaps), consuming an ALREADY-IMPLEMENTED primitive (FreezeWorkingTree, S1) and
an EXISTING method (TreeDiff). No new files, no new types, no new deps. The frozen diffs produce the SAME
bytes as the live diffs when no concurrent process writes — so existing tests pass unchanged in content
(only ~12 call sites gain 2 params). The residual risks: (a) the arbiter leftover-diff DIRECTION (tipTree→
tStart, not tStart→tipTree) — pinned by an exclusion test; (b) the `err` variable handling in
runSingleShortcut after removing AddAll+WriteTree (a `var err error` redeclare) — caught by gofmt/vet; (c)
the escape-hatch correctly NOT freezing — structurally guaranteed by the insertion point. The freeze
ENFORCEMENT (stager subset check, FR-M1c) is deliberately deferred to P3.M2.T1.S1 — this task's freeze is
the necessary foundation (capture + frozen diff inputs + frozen shortcut), and the exclusion tests prove
the observable guarantee for the planner/shortcut/arbiter paths. Parallel-safe: S1 (git.go) is done; this
task edits decompose.go + planner.go (which S1 does not touch).
