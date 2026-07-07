---
name: "P1.M1.T2.S1 — Freeze-safe arbiter: gate→frozen leftover; resolveArbiter 7-arg; 3 paths tree-only-from-T_start; ReadTree(T_start) sync; chain_test updates"
description: |
  Code-only bugfix (FR-M1d/M9/M10 — arbiter freeze parity). Close the loophole where the v2.0–v2.1 arbiter
  gated on LIVE `StatusPorcelain` (decompose.go:217) and staged via `AddAll`/`Add` against the live tree
  (chain.go resolveNewCommit/resolveTipAmend/resolveMidChain), so a post-`T_start` working-tree file could
  be swept into an arbiter commit. (1) GATE: replace `StatusPorcelain` with the frozen `DiffTreeNames(tipTree,
  tStart)`; run the arbiter iff `len(leftoverPaths) > 0 && len(commits) > 0`. (2) runArbiterPhase + resolveArbiter
  signatures gain `leftoverPaths []string` (resolveArbiter also gains `tStart string` → 7-arg). (3) Path A/B:
  `treePrime := tStart` (drop AddAll + WriteTree); `CommitTree(tStart, …)`; after CAS, `ReadTree(tStart)`.
  (4) Path C: per-j `treePrime := OverlayTreePaths(tree[j], tStart, leftoverPaths)` (drop StatusPorcelain/
  ReadTree/Add/WriteTree); after the final CAS, `ReadTree(tStart)`. (5) Remove the dead `leftoverPaths` helper
  + `TestLeftoverPaths`. (6) Extend `chnBuildChain` to return `tStart, leftoverPaths`; update the 7
  `resolveArbiter(...)` call sites to the 7-arg form. Consumes `OverlayTreePaths` from S1 (LANDED). Baseline GREEN.
---

## Goal

**Feature Goal**: Make the decompose arbiter freeze-safe on all three resolution paths (FR-M1d): NO arbiter
code path reads `git status` or stages against the live working tree. The gate is the FROZEN leftover
`DiffTreeNames(tipTree, T_start)`; every arbiter commit's tree is built from `T_start` (paths A/B) or
`OverlayTreePaths(tree[j], T_start, leftoverPaths)` (path C); the index is synced to `T_start` after each
path. A working-tree file written after `T_start` capture can therefore never enter an arbiter commit.

**Deliverable** (3 files modified; code-only):
1. `internal/decompose/decompose.go` — gate (replace `StatusPorcelain` with `DiffTreeNames(tipTree, tStart)`);
   `runArbiterPhase` signature `+leftoverPaths`; its `resolveArbiter` call `+tStart, leftoverPaths`.
2. `internal/decompose/chain.go` — `resolveArbiter` → 7-arg; `resolveNewCommit`/`resolveTipAmend` `+tStart`
   and `treePrime := tStart` + `ReadTree(tStart)`; `resolveMidChain` `+tStart, leftoverPaths` and
   `OverlayTreePaths` per-j + `ReadTree(tStart)`; REMOVE the `leftoverPaths` helper.
3. `internal/decompose/chain_test.go` — extend `chnBuildChain` to return `tStart, leftoverPaths`; update
   the 7 `resolveArbiter(...)` call sites to 7-arg; REMOVE `TestLeftoverPaths`.

**Success Definition**: `go test -race ./internal/decompose/` green; NO resolution path calls
`StatusPorcelain` or `AddAll`/`Add` against the live tree (the gate, the diff, and the committed trees are
all derived from `T_start`/`tipTree`/`tree[j]`); each path syncs the index via `ReadTree(tStart)`; the
rebuilt mid-chain tip == `T_start`; the existing per-path assertions (HEAD.tree contains the leftovers,
git status clean post-resolution) still hold. `go build/vet/gofmt` clean.

## User Persona

**Target User**: The user running `stagecoach` (decompose) while ANOTHER tool (an editor save, a concurrent coding agent) writes to the working tree during the planner/stager calls. Also the contributor implementing the P1.M1.T3 invariant integration tests (which consume the 7-arg `resolveArbiter`).

**Use Case**: A decompose run's stagers cover all of `T_start`; a concurrent process then dirties the working tree. Today the live `StatusPorcelain` gate sees the dirt → the arbiter runs → `AddAll` sweeps the concurrent file into an arbiter commit. After the fix, the frozen `DiffTreeNames(tipTree, T_start)` is empty → the arbiter does NOT run → the concurrent change is left untouched in the working tree.

**Pain Points Addressed**: Eliminates the silent concurrency loophole (FR-M1d) where a post-freeze file could land in an arbiter commit — restoring the "the run commits exactly the working-tree state as it existed when the run began" guarantee (FR-M1b) across the WHOLE run (planner + stager + arbiter).

## Why

- **FR-M1d is the explicit mandate.** The PRD (§9.14 FR-M1d, §13.6.5) names the arbiter as the "third freeze surface" held to the identical invariant as the stager: its gate, its diff, and its committed trees are all derived from `T_start`/`tipTree` (frozen SHAs), never a live working-tree read. v2.0–v2.1's live `StatusPorcelain` gate + `AddAll`/`Add` staging is the documented loophole FR-M1d closes.
- **The arbiter's DIFF is ALREADY frozen.** `runArbiterPhase` (decompose.go:617-618) already computes `tipTree := chainData[last].Tree` and `TreeDiff(tipTree, tStart)` — so FR-M1d (2) is satisfied. Only the GATE (1) and the STAGING (3) leak. This task closes both.
- **`OverlayTreePaths` is landed (S1).** The mid-chain path's tree-only fold primitive exists (git.go:290-302/1635). This task is its sole consumer (`OverlayTreePaths(tree[j], tStart, leftoverPaths)`).
- **Single-point freeze enforcement.** After this change, the WHOLE decompose run (planner reads T_start's diff; stager is FR-M1c-enforced; arbiter gate/diff/staging all frozen) commits only T_start content. The orchestrator owns the freeze boundary on every surface.
- **No behavior change on the happy path.** When no concurrent change occurs, the frozen leftover == the live leftover, so the arbiter runs identically and produces the same commits. The change is purely defensive (closes the concurrency loophole); existing assertions hold.

## What

A signature/threading change across 5 functions + a gate rewrite + the three resolution paths rebuilt to
use `T_start`/`OverlayTreePaths` instead of live `AddAll`/`Add`, plus a `ReadTree(T_start)` index-sync
after each path. Plus chain_test.go updates (chnBuildChain extension, 7 call sites, remove TestLeftoverPaths).
No docs (P1.M1.T2.S2), no decompose_test.go integration tests (P1.M1.T3).

### Success Criteria

- [ ] `decompose.go` gate: `leftoverPaths := DiffTreeNames(tipTree, tStart)`; arbiter runs iff `len(leftoverPaths) > 0 && len(commits) > 0`. NO `StatusPorcelain` at the gate.
- [ ] `runArbiterPhase(ctx, deps, commits, chainData, tStart, leftoverPaths)`; calls `resolveArbiter(ctx, deps, target, commits, chainData, tStart, leftoverPaths)`.
- [ ] `resolveArbiter` is 7-arg `(ctx, deps, target *string, commits, chainData, tStart string, leftoverPaths []string)`.
- [ ] `resolveNewCommit`/`resolveTipAmend`: `treePrime := tStart` (NO `AddAll`, NO `WriteTree`); `CommitTree(tStart, …)`; `ReadTree(tStart)` after CAS.
- [ ] `resolveMidChain`: per-j `treePrime := deps.Git.OverlayTreePaths(ctx, chainData[j].Tree, tStart, leftoverPaths)` (NO `StatusPorcelain`, NO `ReadTree(tree[j])`+`Add`+`WriteTree`); `ReadTree(tStart)` after the final CAS.
- [ ] The `leftoverPaths` helper func (chain.go) is REMOVED; `TestLeftoverPaths` (chain_test.go) is REMOVED.
- [ ] `chnBuildChain` returns `(commits, chainData, tStart string, leftoverPaths []string)`; the 7 `resolveArbiter(...)` call sites use the 7-arg form.
- [ ] NO resolution path calls `deps.Git.StatusPorcelain`, `deps.Git.AddAll`, or `deps.Git.Add`.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim current gate, the 5 current signatures (with file:line),
the exact live-tree call line numbers per path, the target code for each path (tree-only-from-T_start), the
OverlayTreePaths call shape, the chnBuildChain extension, the 7 call sites, and the rationale for
OverlayTreePaths-vs-ReadTree in path C. The architecture `arbiter_freeze_parity.md` is the authoritative
spec (§2 target state, §4 ReadTree sync, §6 risks). S1's OverlayTreePaths is confirmed landed; baseline green.

### Documentation & References

```yaml
# MUST READ — the authoritative spec (current state + target + rationale)
- docfile: plan/008_82253c999440/docs/architecture/arbiter_freeze_parity.md
  why: "§1 the CURRENT live-tree calls with exact line numbers (gate 213-227, runArbiterPhase 603-642 already frozen, chain.go 3 paths AddAll/Add/StatusPorcelain/ReadTree/WriteTree line numbers, leftoverPaths helper ~248). §2 the TARGET state per FR-M1d (gate→DiffTreeNames, runArbiterPhase+resolveArbiter signatures, path A/B treePrime=tStart, path C OverlayTreePaths). §4 the ReadTree(T_start) index-sync after each path. §6 risks (deletion-overlay, two-tipTree-compute, quotepath). §7 the file-by-file change summary."
  critical: "§2.1 (gate), §2.2 (runArbiterPhase), §2.3 (resolveArbiter 7-arg), §2.4-§2.6 (the 3 paths' exact target), §2.7 (WHY OverlayTreePaths not ReadTree for mid-chain), §4 (ReadTree sync) ARE the implementation spec. §6.7 (two tipTree computes — accept the recompute)."

- docfile: plan/008_82253c999440/P1M1T1S1/PRP.md
  why: "The S1 sibling (OverlayTreePaths primitive — LANDED). Confirms the primitive's signature `OverlayTreePaths(ctx, baseTree, sourceTree, paths) (treeSHA, err)` and its discipline (index/object-store-only; never touches working tree; never moves a ref). The mid-chain path C is its sole consumer."
  critical: "OverlayTreePaths(baseTree=tree[j], sourceTree=T_start, paths=leftoverPaths) overlays ONLY the leftover paths from T_start onto tree[j]; unchanged paths keep tree[j]'s blob. For j==N-1 the result == T_start (the rebuilt-tip-equals-T_start invariant)."

- docfile: plan/008_82253c999440/P1M1T2S1/research/arbiter_freeze_rewrite_notes.md
  why: "EMPIRICALLY VERIFIED (live code, 2026-07-04): baseline GREEN; S1 OverlayTreePaths LANDED; the 5 current→target signatures; the 3 paths' current live-tree calls → target tree-only logic; the gate rewrite (verbatim current + target); the chnBuildChain extension + the 7 call sites + TestLeftoverPaths removal; the ReadTree(tStart) rationale; decisions D1–D9. READ THIS FIRST."
  critical: "§3 (the 5 signatures), §4 (the 3 paths' exact target code), §5 (the gate verbatim), §6 (chain_test changes) are copy-paste-ready. §2 is the loophole being closed."

# The edit targets
- file: internal/decompose/chain.go
  why: "EDIT (the bulk). resolveArbiter (:50) → 7-arg; resolveNewCommit (:78) + resolveTipAmend (:137) → +tStart, treePrime:=tStart, drop AddAll(:96/:142)+WriteTree(:100/:145), +ReadTree(tStart); resolveMidChain (:185) → +tStart+leftoverPaths, per-j OverlayTreePaths (drop StatusPorcelain:190/leftoverPaths()call/ReadTree:201/Add:205/WriteTree:209), +ReadTree(tStart); REMOVE the leftoverPaths helper (:236)."
  pattern: "The tipSHA/tipTree/parents/CAS-expectedOld/CAS-error-handling logic in each path is UNCHANGED — only the treePrime derivation (T_start/OverlayTreePaths vs AddAll+WriteTree) and the post-CAS ReadTree(tStart) sync change. generateMessage(ctx, deps, tipTree, tStart) on path A diffs the frozen leftovers."
  gotcha: "Path C folds at EVERY j ∈ [i, N-1] (G-FOLD invariant — unchanged from the current code). OverlayTreePaths(tree[j], tStart, leftoverPaths) per j; the rebuilt tip == T_start. Do NOT fold only at j==i (would revert at j+1)."

- file: internal/decompose/decompose.go
  why: "EDIT (gate + runArbiterPhase). Gate ~:213-227: replace `status := StatusPorcelain` + `status != \"\"` with `leftoverPaths := DiffTreeNames(tipTree, tStart)` + `len(leftoverPaths) > 0`. runArbiterPhase (:607): +leftoverPaths param; its resolveArbiter call (:638) → +tStart, leftoverPaths. runArbiterPhase KEEPS deriving tipTree (:617) for its TreeDiff (cheap recompute — arch §6.7)."
  pattern: "tipTree = chainData[len(chainData)-1].Tree (computed at the gate AND inside runArbiterPhase — accept the recompute). The gate's `buildArbiterCommits` + `rereadFinalCommits` flow is UNCHANGED."
  gotcha: "The gate's `len(commits) > 0` guard stays (the arbiter does not run on an empty/aborted loop). DiffTreeNames(tipTree, tStart) is the frozen leftover — empty ⇒ arbiter skipped (the perfect-run / concurrent-change-excluded case)."

- file: internal/decompose/chain_test.go
  why: "EDIT (test plumbing). chnBuildChain (:81) → return `(commits, chainData, tStart, leftoverPaths)`. The 7 resolveArbiter(...) call sites (:137/:191/:251/:324/:363/:401/:548) → 7-arg. REMOVE TestLeftoverPaths (:418)."
  pattern: "chnBuildChain builds C0/C1/C2 (c0.go/c1.go/c2.go) + untracked leftover.go (:108). tStart = a tree holding tree2(c0+c1+c2) + leftover.go (stage leftover.go → chnRunGit write-tree → tStart → chnRunGit read-tree tree2 to restore a clean index). leftoverPaths = []string{\"leftover.go\"} (or chnRunGit diff-tree --name-only tree2 tStart)."
  gotcha: "The existing per-path ASSERTIONS (HEAD.tree contains leftover.go; git status clean) STILL HOLD — the outcome is identical (tree-only-from-T_start produces the same trees as live AddAll when there is no concurrent change). Only the call signature + chnBuildChain return change. Do NOT relax the assertions."

# Read-only refs (do NOT edit)
- file: internal/git/git.go
  why: "READ-ONLY. OverlayTreePaths (interface :290-302, gitRunner :1635-1643 — S1 LANDED). DiffTreeNames, ReadTree, TreeDiff, CommitTree, UpdateRefCAS, RevParseHEAD, RevParseTree, EmptyTreeSHA — the primitives the paths use. const EmptyTreeSHA at :641."
- file: internal/decompose/message.go
  why: "READ-ONLY. generateMessage (reused by path A) + handleUpdateRefErr (the CAS-error helper reused by all paths). runOneFileShortcut/runSingleShortcut (in decompose.go) are the 'commit T_start directly + ReadTree(tStart)' template paths A/B mirror."

# External references
- url: https://git-scm.com/docs/git-read-tree
  why: "`git read-tree <tree>` loads a tree into the index without touching the working tree. The post-CAS `ReadTree(tStart)` sync sets index == T_start == HEAD.tree → git status clean for the committed set. Confirms it is index-only (no working-tree mutation)."
- url: https://git-scm.com/docs/git-update-index
  why: "Confirms `update-index --cacheinfo <mode>,<blob>,<path>` (overlay) and `--force-remove <path>` (deletion-overlay) — the two ops inside OverlayTreePaths (path C). No shell; argv-only (PRD §19)."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/decompose/
    ├── decompose.go     # EDIT: gate (~213-227) + runArbiterPhase (~607) signature + resolveArbiter call
    ├── chain.go         # EDIT: resolveArbiter (7-arg) + 3 paths (tree-only-from-T_start) + remove leftoverPaths helper
    └── chain_test.go    # EDIT: chnBuildChain (return tStart+leftoverPaths) + 7 call sites + remove TestLeftoverPaths
# (internal/git/git.go OverlayTreePaths is LANDED — read-only input)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/decompose/decompose.go    # gate→DiffTreeNames; runArbiterPhase +leftoverPaths
    internal/decompose/chain.go        # resolveArbiter 7-arg; 3 paths tree-only; -leftoverPaths helper
    internal/decompose/chain_test.go   # chnBuildChain +tStart/leftoverPaths; 7 call sites 7-arg; -TestLeftoverPaths
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/decompose/decompose.go` | MODIFY | Gate → frozen `DiffTreeNames(tipTree, tStart)`; `runArbiterPhase` +leftoverPaths + the resolveArbiter call threads tStart+leftoverPaths. |
| `internal/decompose/chain.go` | MODIFY | `resolveArbiter` 7-arg; 3 paths tree-only-from-T_start (+ReadTree(tStart) sync); remove the `leftoverPaths` helper. **Bulk of the change.** |
| `internal/decompose/chain_test.go` | MODIFY | `chnBuildChain` returns tStart+leftoverPaths; 7 call sites → 7-arg; remove `TestLeftoverPaths`. |

**Explicitly NOT touched**: `internal/git/*` (OverlayTreePaths is S1's landed input; read-only here),
`internal/decompose/decompose_test.go` (the FR-M1d invariant integration tests = P1.M1.T3),
`internal/decompose/planner.go`/`message.go`/`stager.go`/`arbiter.go` (unaffected), `docs/*` (P1.M1.T2.S2),
any other package, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — treePrime derives from T_start, NEVER AddAll/WriteTree): paths A and B set treePrime := tStart
// (a frozen SHA). They MUST NOT call AddAll or WriteTree — that reads the live working tree (the loophole).
// Path A's generateMessage(ctx, deps, tipTree, tStart) diffs the frozen leftovers (TreeDiff(tip→T_start)).
// The tipSHA/tipTree/parents/CAS-expectedOld logic is UNCHANGED.

// CRITICAL (G2 — path C folds via OverlayTreePaths at EVERY j, not ReadTree+Add): the current loop does
// ReadTree(tree[j]) + Add(paths) + WriteTree per j. The replacement is ONE call per j:
//   treePrime, err := deps.Git.OverlayTreePaths(ctx, chainData[j].Tree, tStart, leftoverPaths)
// This overlays ONLY the leftover paths from T_start onto tree[j]; unchanged paths keep tree[j]'s blob.
// The G-FOLD invariant (fold at EVERY j ∈ [i,N-1]) is PRESERVED — OverlayTreePaths runs per j. Do NOT fold
// only at j==i.

// CRITICAL (G3 — the rebuilt mid-chain tip == T_start): OverlayTreePaths(tree[N-1], T_start, leftoverPaths)
// overlays every changed path ⇒ the result == T_start. This is the deterministic-reconstruction invariant
// (arch §2.6). The final ReadTree(tStart) sync then makes index == HEAD.tree == T_start.

// CRITICAL (G4 — ReadTree(tStart) after EACH path's CAS): every path appends `deps.Git.ReadTree(ctx, tStart)`
// after UpdateRefCAS succeeds. This syncs the index to T_start so git status is clean for the committed set
// (index == HEAD.tree == T_start). A sync failure is best-effort (the commit already landed) — wrap with
// ErrArbiterResolutionFailed. Mirrors runOneFileShortcut (decompose.go:300) / runSingleShortcut (:356).

// CRITICAL (G5 — the gate uses the FROZEN leftover, NOT live StatusPorcelain): replace `status := StatusPorcelain`
// with `leftoverPaths := DiffTreeNames(tipTree, tStart)`. A post-T_start untracked file appears in live
// StatusPorcelain but NOT in DiffTreeNames(tipTree, T_start) — that is exactly the freeze property. The
// `len(commits) > 0` guard stays. Do NOT call StatusPorcelain anywhere in the arbiter paths.

// GOTCHA (G6 — leftoverPaths is now a PARAM, not a parsed porcelain string): remove the `leftoverPaths(status)`
// helper CALL in resolveMidChain AND the `leftoverPaths` helper FUNC (chain.go:236) AND TestLeftoverPaths
// (chain_test.go:418). The fold set is DiffTreeNames(tipTree, tStart), computed once at the gate and threaded in.

// GOTCHA (G7 — chnBuildChain must return tStart + leftoverPaths): the 7 call sites need them. chnBuildChain
// builds the chain + untracked leftover.go; tStart = tree2 + leftover.go (stage leftover → write-tree → tStart
// → read-tree tree2 to restore clean index); leftoverPaths = []string{"leftover.go"}. Update the 7 call sites
// to `resolveArbiter(ctx, deps, target, commits, chainData, tStart, leftoverPaths)`.

// GOTCHA (G8 — existing per-path assertions STILL HOLD): the NullNewCommit/TipAmend/MidChainRebuild/
// CleanTreePostcondition tests assert HEAD.tree contains the leftovers + git status clean. These remain valid:
// tree-only-from-T_start produces the same trees as live AddAll when there is no concurrent change. Do NOT
// relax them. (The P1.M1.T3 integration tests add the concurrent-sentinel exclusion proof.)

// GOTCHA (G9 — two tipTree computes): the gate derives tipTree = chainData[last].Tree AND runArbiterPhase
// derives it again (:617) for its TreeDiff. Accept the recompute (arch §6.7: cheap slice index, minimal-change).
// Do NOT pass tipTree into runArbiterPhase (cosmetic; runArbiterPhase already does it for the TreeDiff).

// GOTCHA (G10 — scope): ONLY decompose.go + chain.go + chain_test.go. The decompose_test.go FR-M1d invariant
// integration tests (concurrent-sentinel exclusion) are P1.M1.T3; the docs/how-it-works.md narrative is
// P1.M1.T2.S2. Do NOT edit those here.
```

## Implementation Blueprint

### Data models and structure

No new types. `ChainEntry{SHA, Tree, Message, Parent}` is unchanged. The change is signature/threading
(`tStart`, `leftoverPaths` params) + the treePrime derivation per path (T_start / OverlayTreePaths vs
AddAll+WriteTree) + the ReadTree(tStart) index-sync + the gate metric (frozen DiffTreeNames vs live
StatusPorcelain) + removing the dead `leftoverPaths` helper.

### Implementation Tasks (ordered by dependencies — TDD: tests-first per the contract)

```yaml
Task 1: EXTEND chnBuildChain + update the 7 resolveArbiter call sites (chain_test.go) — do FIRST (TDD)
  - FILE: internal/decompose/chain_test.go
  - EDIT chnBuildChain (:81): change the return signature to
        func chnBuildChain(t *testing.T, repo string) (commits []CommitInfo, chainData []ChainEntry, tStart string, leftoverPaths []string)
    Build tStart after the chain + leftover.go exist: stage leftover.go (chnStageFile), tStart = chnRunGit(t,
    repo, "write-tree"), then chnRunGit(t, repo, "read-tree", tree2) to restore a clean index (so the tests'
    starting state is unchanged). leftoverPaths = []string{"leftover.go"} (or chnRunGit diff-tree --no-commit-id
    --name-only -r tree2 tStart). Update the return statement.
  - UPDATE the 7 call sites (:137/:191/:251/:324/:363/:401/:548): each currently
        resolveArbiter(context.Background(), deps, <target>, commits, chainData)
    → become
        resolveArbiter(context.Background(), deps, <target>, commits, chainData, tStart, leftoverPaths)
    (capture the new chnBuildChain returns at each test's setup: `commits, chainData, tStart, leftoverPaths := chnBuildChain(t, repo)`).
  - REMOVE TestLeftoverPaths (:418-~485) entirely (the helper it tests is removed in Task 4).
  - NOTE: the existing per-path assertions (HEAD.tree contains leftover.go; git status clean) are UNCHANGED
    and still hold (Task 2/3 produce the same trees). Do NOT relax them.
  - DO NOT: change the chn* helpers otherwise; relax assertions; edit decompose_test.go (P1.M1.T3).
  - (After Task 2-4 this compiles + passes; until then it will not compile — that is expected for TDD.)

Task 2: REWRITE the gate + runArbiterPhase (decompose.go)
  - FILE: internal/decompose/decompose.go
  - GATE (~:213-227): replace
        amended := 0
        status, err := deps.Git.StatusPorcelain(ctx)
        if err != nil { return DecomposeResult{}, fmt.Errorf("%w: status: %w", ErrDecomposeFailed, err) }
        if status != "" && len(commits) > 0 {
            arbiterCommits := buildArbiterCommits(ctx, deps, commits, isUnborn)
            amended, err = runArbiterPhase(ctx, deps, arbiterCommits, chainData, tStart)
    with
        tipTree := chainData[len(chainData)-1].Tree
        leftoverPaths, err := deps.Git.DiffTreeNames(ctx, tipTree, tStart)
        if err != nil { return DecomposeResult{}, fmt.Errorf("%w: arbiter gate diff-names: %w", ErrDecomposeFailed, err) }
        amended := 0
        if len(leftoverPaths) > 0 && len(commits) > 0 {
            arbiterCommits := buildArbiterCommits(ctx, deps, commits, isUnborn)
            amended, err = runArbiterPhase(ctx, deps, arbiterCommits, chainData, tStart, leftoverPaths)
    PRESERVE the post-runArbiterPhase `rereadFinalCommits` flow UNCHANGED.
  - runArbiterPhase (:607): signature +leftoverPaths []string (6th param). Its internal tipTree (:617) +
    TreeDiff (:618) are UNCHANGED (already frozen). Its resolveArbiter call (:638) →
        resolveArbiter(ctx, deps, out.Target, commits, chainData, tStart, leftoverPaths)
  - DO NOT: change runArbiter's internal logic; pass tipTree in (G9 — accept the recompute); touch buildArbiterCommits/rereadFinalCommits.

Task 3: REWRITE resolveArbiter + the 3 paths (chain.go)
  - FILE: internal/decompose/chain.go
  - resolveArbiter (:50): signature → 7-arg `(ctx, deps, target *string, commits []CommitInfo, chainData []ChainEntry, tStart string, leftoverPaths []string) error`.
    Dispatch UNCHANGED, but thread the new args into each path:
      resolveNewCommit(ctx, deps, commits, chainData, tStart)
      resolveTipAmend(ctx, deps, chainData, tStart)
      resolveMidChain(ctx, deps, idx, chainData, tStart, leftoverPaths)
  - resolveNewCommit (:78): signature +tStart string. REMOVE AddAll (:96) + WriteTree (:100). Set treePrime := tStart.
    generateMessage(ctx, deps, treeA, tStart) (treeA = tipTree/EmptyTreeSHA — UNCHANGED). CommitTree(tStart, parents, msg).
    UpdateRefCAS UNCHANGED. AFTER CAS succeeds: `if err := deps.Git.ReadTree(ctx, tStart); err != nil { return fmt.Errorf("%w: read-tree sync: %w", ErrArbiterResolutionFailed, err) }`.
  - resolveTipAmend (:137): signature +tStart string. REMOVE AddAll (:142) + WriteTree (:145). Set treePrime := tStart.
    CommitTree(tStart, parents, tipMsg) (msg verbatim — UNCHANGED). UpdateRefCAS UNCHANGED. AFTER CAS: ReadTree(tStart) (same wrap).
  - resolveMidChain (:185): signature +tStart string +leftoverPaths []string. REMOVE StatusPorcelain (:190) + the `leftoverPaths(status)` call.
    In the per-j loop: REMOVE ReadTree(tree[j]) (:201) + Add(paths) (:205) + WriteTree (:209); REPLACE with
        treePrime, err := deps.Git.OverlayTreePaths(ctx, chainData[j].Tree, tStart, leftoverPaths)
        if err != nil { return fmt.Errorf("%w: overlay-tree-paths[%d]: %w", ErrArbiterResolutionFailed, j, err) }
    (the `paths` var is GONE — leftoverPaths is the param). CommitTree(treePrime, parents, chainData[j].Message) UNCHANGED.
    Final UpdateRefCAS UNCHANGED. AFTER CAS: ReadTree(tStart) (same wrap).
  - DO NOT: change findTargetIndex, handleUpdateRefErr, the parents/CAS-expectedOld logic, or the G-FOLD (OverlayTreePaths runs per j).

Task 4: REMOVE the leftoverPaths helper (chain.go)
  - FILE: internal/decompose/chain.go
  - DELETE the `func leftoverPaths(status string) []string { ... }` func (~:236-~252). It has NO live call site
    after Task 3 (resolveMidChain no longer calls it; the fold set is DiffTreeNames(tipTree, tStart)).
  - (TestLeftoverPaths was removed in Task 1.)
  - DO NOT: remove the `strings` import if still used elsewhere (check: it IS used by RevParseHEAD zero-hash
    `strings.Repeat("0", 40)` in resolveNewCommit/resolveTipAmend — keep it).

Task 5: VALIDATE
  - RUN: gofmt -w internal/decompose/decompose.go internal/decompose/chain.go internal/decompose/chain_test.go
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test -race ./internal/decompose/    # ALL green — the updated TestResolveArbiter_* pass
  - RUN: go test -race ./...                      # full suite green
  - RUN (freeze-parity grep — the contract's headline check):
        grep -n "StatusPorcelain\|AddAll\|\.Add(" internal/decompose/chain.go internal/decompose/decompose.go
        # EXPECT: NO StatusPorcelain/AddAll/Add in chain.go; decompose.go's AddAll calls are ONLY in
        # runSingleEscape/runOneFileShortcut/runSingleShortcut (the v1/shortcut paths, NOT the arbiter).
  - FIX-FORWARD: read failures, fix, re-run.
```

### Implementation Patterns & Key Details

```go
// === Path A (resolveNewCommit) — treePrime := tStart (the freeze-safe shape) ===
	tipSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx)   // UNCHANGED
	... tipTree derivation UNCHANGED ...
	treePrime := tStart                                    // ← was: AddAll + WriteTree
	treeA := tipTree; if treeA == "" { treeA = git.EmptyTreeSHA }
	msg, err := generateMessage(ctx, deps, treeA, tStart)  // ← diffs TreeDiff(tip, T_start) = frozen leftovers
	... CommitTree(tStart, parents, msg) ... UpdateRefCAS UNCHANGED ...
	if err := deps.Git.ReadTree(ctx, tStart); err != nil { // ← index sync (FR-M1d (3))
		return fmt.Errorf("%w: read-tree sync: %w", ErrArbiterResolutionFailed, err)
	}

// === Path C (resolveMidChain) — OverlayTreePaths per j (the freeze-safe fold) ===
	for j := i; j < N; j++ {
		treePrime, err := deps.Git.OverlayTreePaths(ctx, chainData[j].Tree, tStart, leftoverPaths) // ← was: ReadTree+Add+WriteTree
		if err != nil { return fmt.Errorf("%w: overlay-tree-paths[%d]: %w", ErrArbiterResolutionFailed, j, err) }
		... CommitTree(treePrime, parents, chainData[j].Message) ... rebuiltParent = newSHA ...
	}
	... UpdateRefCAS(HEAD, rebuiltParent, tipSHA) UNCHANGED ...
	if err := deps.Git.ReadTree(ctx, tStart); err != nil { ... }   // ← index sync

// === The gate — frozen leftover (FR-M1d (1)) ===
	tipTree := chainData[len(chainData)-1].Tree
	leftoverPaths, err := deps.Git.DiffTreeNames(ctx, tipTree, tStart) // ← was: StatusPorcelain
	...
	if len(leftoverPaths) > 0 && len(commits) > 0 { ... runArbiterPhase(..., tStart, leftoverPaths) }

// === WHY OverlayTreePaths (not ReadTree(tStart)) for path C ===
// Path C folds the leftovers onto each INTERMEDIATE tree[j], not onto T_start. A path UNCHANGED between
// tree[j] and T_start must keep tree[j]'s blob (e.g. c1.go in C1); a leftover path must take T_start's blob.
// OverlayTreePaths(base=tree[j], source=T_start, paths=leftoverPaths) overlays ONLY the leftover paths.
// For j==N-1 the result == T_start (rebuilt-tip-equals-T_start). ReadTree(tStart) would clobber tree[j]'s
// unique blobs with T_start's — WRONG for intermediate commits.
```

### Integration Points

```yaml
PRODUCTION (internal/decompose/decompose.go):
  - gate: DiffTreeNames(tipTree, tStart) replaces StatusPorcelain; arbiter runs iff len(leftoverPaths)>0 && len(commits)>0
  - runArbiterPhase: +leftoverPaths param; resolveArbiter call threads tStart+leftoverPaths

PRODUCTION (internal/decompose/chain.go):
  - resolveArbiter: 7-arg (ctx, deps, target, commits, chainData, tStart, leftoverPaths)
  - resolveNewCommit/resolveTipAmend: +tStart; treePrime:=tStart; -AddAll/-WriteTree; +ReadTree(tStart)
  - resolveMidChain: +tStart+leftoverPaths; per-j OverlayTreePaths; -StatusPorcelain/-ReadTree/-Add/-WriteTree; +ReadTree(tStart)
  - REMOVED: leftoverPaths helper func

TESTS (internal/decompose/chain_test.go):
  - chnBuildChain returns (commits, chainData, tStart, leftoverPaths)
  - 7 resolveArbiter call sites → 7-arg
  - REMOVED: TestLeftoverPaths

CONSUMED (READ-ONLY):
  - internal/git/git.go: OverlayTreePaths (S1 LANDED), DiffTreeNames, ReadTree, TreeDiff, CommitTree, UpdateRefCAS, RevParseHEAD/Tree, EmptyTreeSHA
  - internal/decompose/message.go: generateMessage, handleUpdateRefErr
  - internal/decompose/decompose.go: runOneFileShortcut/runSingleShortcut (the "commit T_start + ReadTree" template)

GATE: go test -race ./internal/decompose/ → GREEN; grep StatusPorcelain|AddAll|.Add( chain.go → NONE

NO-TOUCH (explicitly):
  - internal/git/*                       # OverlayTreePaths is S1's landed input (read-only here)
  - internal/decompose/decompose_test.go # the FR-M1d invariant integration tests = P1.M1.T3
  - internal/decompose/{planner,message,stager,arbiter}.go  # unaffected
  - docs/* (P1.M1.T2.S2); PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by LATER subtasks):
  - P1.M1.T2.S2: Mode A doc edit (docs/how-it-works.md arbiter freeze narrative — "gate/diff/staging frozen").
  - P1.M1.T3.S1/S2: the FR-M1d acceptance proof — concurrent-sentinel-exclusion integration tests in
    decompose_test.go (upgrade TestDecompose_ConcurrentChangeExclusion + the T_start-completeness proof).
    These consume the freeze-safe arbiter + may call resolveArbiter directly with the 7-arg signature.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/decompose/decompose.go internal/decompose/chain.go internal/decompose/chain_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/decompose/...  # Expected: exit 0 (the signature changes compile across the package)
go build ./...                   # Expected: exit 0

# Expected: Zero errors. The build confirms the 5 signature changes + the chnBuildChain return are consistent.
```

### Level 2: Unit Tests (the canonical resolveArbiter tests)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./internal/decompose/ -v -run TestResolveArbiter

# Expected: ALL pass — NullNewCommit (HEAD.tree contains leftover.go; the new commit's tree == tStart),
# TipAmend (msg verbatim; both files in HEAD.tree), MidChainRebuild (C0 unchanged; C1'/C2' rebuilt;
# leftover.go folded into every rebuilt tree via OverlayTreePaths), TargetNotFound, CASFailure,
# RescueErrorPropagation, CleanTreePostcondition (status --porcelain == "" via ReadTree(tStart) sync).
# TestLeftoverPaths is GONE (removed).
```

### Level 3: Whole-Repository Regression + Freeze-Parity Grep (the headline check)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages green
go vet ./...                     # Expected: exit 0

# THE freeze-parity check: NO arbiter resolution path reads live status or stages against the live tree.
grep -n "StatusPorcelain\|AddAll\|\.Add(" internal/decompose/chain.go
# Expected: ZERO matches in chain.go (the arbiter paths are entirely tree-only-from-T_start now).

grep -n "StatusPorcelain" internal/decompose/decompose.go
# Expected: ZERO matches at the gate (the gate now uses DiffTreeNames). (runSingleEscape/runOneFileShortcut
# keep their AddAll — those are the v1/shortcut paths, NOT the arbiter; FR-M1d scopes only the arbiter.)

# Confirm ONLY the 3 intended files changed
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/decompose/{decompose,chain,chain_test}.go only.
```

### Level 4: Behavioral Smoke (the rebuilt-tip-equals-T_start invariant)

```bash
cd /home/dustin/projects/stagecoach

# TestResolveArbiter_MidChainRebuild is the direct proof: after a mid-chain rebuild, the rebuilt tip's tree
# == T_start (OverlayTreePaths(tree[N-1], T_start, leftoverPaths) overlays every changed path). The test
# already asserts HEAD.tree contains both c2.go AND leftover.go (== T_start's content). Run it explicitly:
go test -race ./internal/decompose/ -v -run 'TestResolveArbiter_MidChainRebuild'

# Expected: PASS. The mid-chain path produced trees via OverlayTreePaths (no live Add); the final HEAD.tree
# == T_start; git status clean (ReadTree(tStart) sync).
# (The concurrent-sentinel exclusion proof — a post-T_start file in NO arbiter commit — lands in P1.M1.T3.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.

### Feature Validation

- [ ] Gate uses `DiffTreeNames(tipTree, tStart)` (NO `StatusPorcelain`); arbiter runs iff `len(leftoverPaths) > 0 && len(commits) > 0`.
- [ ] `resolveArbiter` is 7-arg `(ctx, deps, target, commits, chainData, tStart, leftoverPaths)`.
- [ ] Paths A/B: `treePrime := tStart`; NO `AddAll`/`WriteTree`; `ReadTree(tStart)` after CAS.
- [ ] Path C: per-j `OverlayTreePaths(tree[j], tStart, leftoverPaths)`; NO `StatusPorcelain`/`ReadTree(tree[j])`/`Add`/`WriteTree`; `ReadTree(tStart)` after the final CAS.
- [ ] `leftoverPaths` helper func REMOVED; `TestLeftoverPaths` REMOVED.
- [ ] `chnBuildChain` returns `(commits, chainData, tStart, leftoverPaths)`; the 7 call sites use the 7-arg form.
- [ ] `grep "StatusPorcelain\|AddAll\|\.Add(" internal/decompose/chain.go` → ZERO matches.

### Scope Discipline Validation

- [ ] ONLY `internal/decompose/{decompose,chain,chain_test}.go` modified (`git diff --stat` confirms).
- [ ] Did NOT edit `internal/git/*` (OverlayTreePaths is S1's landed input).
- [ ] Did NOT edit `internal/decompose/decompose_test.go` (FR-M1d integration tests = P1.M1.T3).
- [ ] Did NOT edit `docs/*` (P1.M1.T2.S2) or any other package.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research note).

### Code Quality Validation

- [ ] The tipSHA/tipTree/parents/CAS-expectedOld/handleUpdateRefErr logic in each path is UNCHANGED (only treePrime derivation + ReadTree sync change).
- [ ] The G-FOLD invariant (OverlayTreePaths runs at EVERY j ∈ [i,N-1]) is preserved.
- [ ] The ReadTree(tStart) sync uses the same `ErrArbiterResolutionFailed` wrap as the other infra errors.
- [ ] Existing per-path assertions are NOT relaxed (the outcome is identical on the happy path).

---

## Anti-Patterns to Avoid

- ❌ Don't call `AddAll`/`WriteTree`/`Add`/`StatusPorcelain` in any arbiter path — that reads/stages the live
  working tree (the loophole). Paths A/B use `treePrime := tStart`; path C uses `OverlayTreePaths` per j;
  the gate uses `DiffTreeNames(tipTree, tStart)` (gotchas G1/G2/G5).
- ❌ Don't use `ReadTree(tStart)` for path C's per-j tree. Path C folds ONLY the leftover paths onto each
  INTERMEDIATE tree[j] (unchanged paths keep tree[j]'s blob). `ReadTree(tStart)` would clobber tree[j]'s
  unique blobs. Use `OverlayTreePaths(tree[j], tStart, leftoverPaths)` per j (gotcha G2 + the §"WHY" block).
- ❌ Don't fold only at j==i in path C. The G-FOLD invariant requires folding at EVERY j ∈ [i,N-1], else
  commit[i+1] reverts the fold. OverlayTreePaths runs per j (gotcha G2).
- ❌ Don't forget the `ReadTree(tStart)` index-sync after EACH path's CAS. Without it, `git status` is not
  clean post-resolution (index != HEAD.tree). FR-M1d (3) / §13.6.5 mandate it; the CleanTreePostcondition
  test pins it (gotcha G4).
- ❌ Don't keep the `leftoverPaths` helper or `TestLeftoverPaths`. Both are dead code under FR-M1d (the fold
  set is `DiffTreeNames(tipTree, tStart)`, computed at the gate, not parsed from porcelain). Remove both
  (gotcha G6).
- ❌ Don't change `tipSHA`/`tipTree`/`parents`/`CAS-expectedOld`/`handleUpdateRefErr` logic. Only the
  treePrime derivation + the ReadTree sync change. The CAS discipline (§18.1) is unchanged.
- ❌ Don't relax the existing per-path assertions. The NullNewCommit/TipAmend/MidChainRebuild/CleanTreePostcondition
  outcomes are identical on the happy path (tree-only-from-T_start == live-AddAll result when no concurrent
  change). The assertions must still pass (gotcha G8).
- ❌ Don't pass `tipTree` into `runArbiterPhase`. Two computes (gate + runArbiterPhase:617) is acceptable
  (cheap slice index; arch §6.7). Threading it is cosmetic churn (gotcha G9).
- ❌ Don't edit `internal/git/*` (OverlayTreePaths is S1's landed input), `decompose_test.go` (P1.M1.T3), or
  `docs/*` (P1.M1.T2.S2). This task is the 3 decompose files only (gotcha G10).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a well-specified signature/threading + per-path-rewrite change backed by an exceptionally
detailed architecture doc (`arbiter_freeze_parity.md` §1 current line numbers, §2 target state, §4 ReadTree
sync, §6 risks) that IS the implementation spec. Five independent de-riskings: (1) the arbiter's DIFF is
ALREADY frozen (`runArbiterPhase` :617-618 already computes `tipTree` + `TreeDiff(tipTree, tStart)`) — only
the gate + staging leak; (2) `OverlayTreePaths` (the one new primitive path C needs) is LANDED from S1
(git.go:290-302/1635-1643); (3) paths A/B's `treePrime := tStart` + `ReadTree(tStart)` mirror the
already-shipped `runOneFileShortcut`/`runSingleShortcut` (decompose.go:285/300, :321/356) — a proven
pattern; (4) the baseline is GREEN and the existing per-path assertions hold on the happy path (the outcome
is identical; only the mechanism changes); (5) the change is mechanical given the spec (drop live-tree
calls, substitute T_start/OverlayTreePaths, append ReadTree(tStart)). The contract's TDD note (update
chain_test canonical tests first) plus the 7-call-site inventory + chnBuildChain extension are spelled out.
The one residual uncertainty (not 10/10) is the chnBuildChain tStart construction bookkeeping (stage
leftover.go → write-tree → read-tree tree2 to restore the clean index) — fiddly test plumbing the
implementer must get right for the per-path assertions to hold, mitigated by the chn* oracle helpers and
the fact that the existing assertions already verify the same HEAD.tree content. The P1.M1.T2.S2 (docs)
and P1.M1.T3 (integration tests) boundaries are cleanly fenced.
