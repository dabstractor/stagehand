# Research: Freeze-Safe Arbiter (Gate + 3 Paths + chain_test) — P1.M1.T2.S1

> **Purpose:** Pin the exact, source-verified edit for the FR-M1d arbiter freeze-parity rewrite, checked
> against the live codebase on 2026-07-04. **Baseline `go test ./internal/decompose/` is GREEN (9.389s);**
> S1's `OverlayTreePaths` primitive HAS LANDED (git.go:290-302 interface + 1635-1643 gitRunner impl).
> The architecture `arbiter_freeze_parity.md` is the authoritative spec (current line numbers + target
> state + rationale). This task closes the loophole where the v2.0–v2.1 arbiter gated on LIVE
> `StatusPorcelain` and staged via `AddAll`/`Add` against the live tree, so a post-`T_start` file could
> land in an arbiter commit.

---

## 1. Environment & Baseline

| Item | Value / Evidence |
|---|---|
| Module | `github.com/dustin/stagecoach`, `go 1.22` |
| Edit targets | `internal/decompose/decompose.go` (gate + runArbiterPhase), `internal/decompose/chain.go` (resolveArbiter + 3 paths + remove leftoverPaths helper), `internal/decompose/chain_test.go` (7 call sites + chnBuildChain + remove TestLeftoverPaths) |
| S1 (OverlayTreePaths) | **LANDED** — git.go:290-302 (interface) + 1635-1643 (gitRunner impl). `go build ./internal/git/` OK. |
| Baseline | `go test ./internal/decompose/` → **ok (9.389s)**; `go vet ./internal/decompose/` clean (test files compile). |
| Prior PRP (S1) | The OverlayTreePaths primitive (my input). S1 does NOT touch internal/decompose → no conflict. |
| Sibling tasks | P1.M1.T2.S2 (Mode A doc — docs/how-it-works.md), P1.M1.T3 (invariant integration tests in decompose_test.go). THIS task is code-only (decompose.go + chain.go + chain_test.go). |

---

## 2. The Loophole (why this change)

v2.0–v2.1's arbiter:
- **Gate** (`decompose.go:217`): `status, err := deps.Git.StatusPorcelain(ctx)` → `if status != "" && len(commits) > 0`.
  A file written to the working tree AFTER `T_start` capture makes `status != ""` even when the frozen
  leftover set is empty → the arbiter runs.
- **Resolution** (chain.go): all three paths stage via `AddAll`/`Add` against the LIVE working tree
  (resolveNewCommit:96/100, resolveTipAmend:142/145, resolveMidChain:190/201/205/209) → the post-freeze
  file is swept into an arbiter commit.

FR-M1d closes this: gate on the FROZEN leftover `DiffTreeNames(tipTree, T_start)`; build every arbiter
tree from `T_start` / `OverlayTreePaths` (never the live tree); sync the index to `T_start` via `ReadTree`.

---

## 3. The Five Current Signatures → Target Signatures (verified against live source)

| Function | CURRENT signature (file:line) | TARGET signature |
|---|---|---|
| `runArbiterPhase` | `(ctx, deps, commits []CommitInfo, chainData []ChainEntry, tStart string) (int, error)` (decompose.go:607) | `+ leftoverPaths []string` (6th param); pass `tStart, leftoverPaths` to resolveArbiter |
| `resolveArbiter` | `(ctx, deps, target *string, commits []CommitInfo, chainData []ChainEntry) error` (chain.go:50) | `+ tStart string, leftoverPaths []string` (7-arg) |
| `resolveNewCommit` | `(ctx, deps, commits []CommitInfo, chainData []ChainEntry) error` (chain.go:78) | `+ tStart string` |
| `resolveTipAmend` | `(ctx, deps, chainData []ChainEntry) error` (chain.go:137) | `+ tStart string` |
| `resolveMidChain` | `(ctx, deps, i int, chainData []ChainEntry) error` (chain.go:185) | `+ tStart string, leftoverPaths []string` |

The branch dispatch in `resolveArbiter` (target==nil/N==0 → newCommit; idx==N-1 → tipAmend; else → midChain)
is UNCHANGED — only the args threaded into each path change.

---

## 4. The Three Resolution Paths — current live-tree calls → target tree-only-from-T_start

### 4.1 Path A — `resolveNewCommit` (null → new N+1 commit)
- **Current**: `AddAll` (chain.go:96) + `WriteTree` (:100) → `treePrime`; `generateMessage(ctx, deps, treeA=tipTree, treePrime)`; `CommitTree(treePrime, [tipSHA])`; `UpdateRefCAS(HEAD, newSHA, tipSHA)`.
- **Target**: drop `AddAll` + `WriteTree`; `treePrime := tStart`; `generateMessage(ctx, deps, tipTree, tStart)` (concept diff = `TreeDiff(tipTree, T_start)` = exactly the frozen leftovers — FR-M10); `CommitTree(tStart, [tipSHA], msg)`; `UpdateRefCAS`; then **`ReadTree(tStart)`** to sync the index. The tipSHA/tipTree/parents/CAS-expectedOld logic is UNCHANGED.
- Mirrors `runOneFileShortcut` (decompose.go:285/300) / `runSingleShortcut` (:321/356) — identical "commit T_start directly + ReadTree(tStart)" pattern.

### 4.2 Path B — `resolveTipAmend` (target==tip → plumbing amend)
- **Current**: `AddAll` (:142) + `WriteTree` (:145) → `treePrime`; `CommitTree(treePrime, [tipParent], tipMsg)`; `UpdateRefCAS(HEAD, newSHA, tipSHA)`.
- **Target**: drop `AddAll` + `WriteTree`; `treePrime := tStart`; `CommitTree(tStart, [tipParent], tipMsg)` (msg reused verbatim); `UpdateRefCAS`; then `ReadTree(tStart)`.

### 4.3 Path C — `resolveMidChain` (target==earlier commit[i] → linear-chain rebuild)
- **Current**: `StatusPorcelain` (:190) → `leftoverPaths(status)` helper; per j: `ReadTree(chainData[j].Tree)` (:201) + `Add(paths)` (:205) + `WriteTree` (:209) → `treePrime`; `CommitTree(treePrime, [rebuiltParent], msg[j])`; final `UpdateRefCAS(HEAD, rebuiltParent, tipSHA)`.
- **Target**: drop `StatusPorcelain` + the `leftoverPaths(...)` helper CALL (leftoverPaths is now a PARAM); per j: `treePrime, err := deps.Git.OverlayTreePaths(ctx, chainData[j].Tree, tStart, leftoverPaths)`; `CommitTree(treePrime, [rebuiltParent], chainData[j].Message)`; final `UpdateRefCAS`; then `ReadTree(tStart)`.
- The rebuilt tip == `T_start` (because `OverlayTreePaths(tree[N-1], T_start, leftoverPaths)` overlays every changed path ⇒ `T_start`). The `rebuiltParent`/`parents`/CAS-expectedOld=tipSHA logic is UNCHANGED.
- **WHY OverlayTreePaths (not ReadTree(tStart))**: Path C folds the leftovers onto each INTERMEDIATE `tree[j]`, not onto T_start directly. A path unchanged between `tree[j]` and `T_start` must keep `tree[j]`'s blob (e.g. c1.go in C1), while a leftover path must take T_start's blob. `OverlayTreePaths(baseTree=tree[j], sourceTree=T_start, paths=leftoverPaths)` is exactly that operation.

### 4.4 Remove the `leftoverPaths` helper (chain.go:236) + `TestLeftoverPaths` (chain_test.go:418)
Under FR-M1d the helper has NO live call site (resolveMidChain no longer calls it; the fold set is
`DiffTreeNames(tipTree, T_start)` computed in runArbiterPhase). It + its test become dead code → remove both.

---

## 5. The Gate (decompose.go:213-227) — current → target

**Current** (verified verbatim):
```go
amended := 0
status, err := deps.Git.StatusPorcelain(ctx)
if err != nil {
    return DecomposeResult{}, fmt.Errorf("%w: status: %w", ErrDecomposeFailed, err)
}
if status != "" && len(commits) > 0 {
    arbiterCommits := buildArbiterCommits(ctx, deps, commits, isUnborn)
    amended, err = runArbiterPhase(ctx, deps, arbiterCommits, chainData, tStart)
    ...
}
```

**Target**:
```go
tipTree := chainData[len(chainData)-1].Tree
leftoverPaths, err := deps.Git.DiffTreeNames(ctx, tipTree, tStart)
if err != nil {
    return DecomposeResult{}, fmt.Errorf("%w: arbiter gate diff-names: %w", ErrDecomposeFailed, err)
}
amended := 0
if len(leftoverPaths) > 0 && len(commits) > 0 {
    arbiterCommits := buildArbiterCommits(ctx, deps, commits, isUnborn)
    amended, err = runArbiterPhase(ctx, deps, arbiterCommits, chainData, tStart, leftoverPaths)
    ...
}
```
Note: `tipTree` is derived both here and inside runArbiterPhase (:617). The architecture §6.7 flags this
as a cheap recompute (slice index) — acceptable; runArbiterPhase keeps deriving it for its TreeDiff.

---

## 6. chain_test.go Changes

### 6.1 Extend `chnBuildChain` to return `tStart` + `leftoverPaths`
Current: `func chnBuildChain(t *testing.T, repo string) ([]CommitInfo, []ChainEntry)` (chain_test.go:81).
It builds C0/C1/C2 (c0.go/c1.go/c2.go) + an untracked `leftover.go` (:108). It does NOT capture tStart.

Target: return `(commits []CommitInfo, chainData []ChainEntry, tStart string, leftoverPaths []string)`.
Build tStart = a tree holding tree2 (c0+c1+c2) + leftover.go (e.g. stage leftover.go → `chnRunGit
write-tree` → tStart → `chnRunGit read-tree tree2` to restore a clean index). `leftoverPaths =
[]string{"leftover.go"}` (or `chnRunGit diff-tree --no-commit-id --name-only -r tree2 tStart`).

### 6.2 Update the 7 `resolveArbiter(...)` call sites to the 7-arg form
Lines 137, 191, 251, 324, 363, 401, 548 — all currently `resolveArbiter(ctx, deps, target, commits, chainData)`
→ become `resolveArbiter(ctx, deps, target, commits, chainData, tStart, leftoverPaths)`.

### 6.3 Remove `TestLeftoverPaths` (chain_test.go:418)
The helper it tests (`leftoverPaths`) is removed in §4.4.

### 6.4 Existing per-path assertions REMAIN VALID
- `TestResolveArbiter_NullNewCommit`: HEAD.tree contains leftover.go (the new commit's tree == tStart, which contains leftover.go) ✓; git status clean (ReadTree(tStart) sync) ✓.
- `TestResolveArbiter_TipAmend`: 3 commits, both files in HEAD.tree ✓.
- `TestResolveArbiter_MidChainRebuild`: C0 unchanged, C1'/C2' rebuilt, leftover.go folded into every rebuilt tree ✓ (OverlayTreePaths folds leftoverPaths at every j).
- `TestResolveArbiter_CleanTreePostcondition`: status --porcelain == "" ✓ (ReadTree(tStart) sync).
The existing assertions hold because the OUTCOME is the same; only the MECHANISM changes (tree-only-from-T_start vs live AddAll). The contract's TDD note: the canonical tests assert the SAME post-conditions + that the mechanism never reads live status.

---

## 7. Decisions Log

| # | Question | Decision | Rationale |
|---|---|---|---|
| D1 | Gate metric? | `DiffTreeNames(tipTree, tStart)` (frozen), NOT `StatusPorcelain` (live). | FR-M1d (1): a post-T_start file must NOT trip the gate. tipTree = chainData[last].Tree. |
| D2 | treePrime for paths A/B? | `tStart` (drop AddAll + WriteTree). | FR-M10: folding all leftovers into the tip yields T_start. Mirrors runOneFileShortcut/runSingleShortcut. |
| D3 | treePrime for path C? | `OverlayTreePaths(tree[j], t_start, leftoverPaths)` per j (drop StatusPorcelain/ReadTree/Add/WriteTree). | FR-M10 mid-chain: fold ONLY the leftover paths onto each intermediate tree[j]; unchanged paths keep tree[j]'s blob. OverlayTreePaths (S1) is exactly this. |
| D4 | Index sync after each path? | `ReadTree(tStart)` after CAS succeeds. | FR-M1d (3) / §13.6.5: index == T_start == HEAD.tree → git status clean for the committed set. Mirrors the single shortcuts. |
| D5 | leftoverPaths helper + TestLeftoverPaths? | REMOVE both (dead code under FR-M1d). | The fold set is now DiffTreeNames(tipTree, tStart) computed in runArbiterPhase; the porcelain parser is unused. |
| D6 | Signature shape? | resolveArbiter → 7-arg; the 3 inner paths gain tStart (+leftoverPaths for mid-chain); runArbiterPhase gains leftoverPaths. | Threads the frozen fold set + T_start to every path; P1.M1.T3 consumes the 7-arg resolveArbiter. |
| D7 | chnBuildChain extension? | Return `(commits, chainData, tStart, leftoverPaths)`. | The 7 call sites need tStart + leftoverPaths; chnBuildChain is the one place that builds the chain+leftover fixture (architecture §5.2). |
| D8 | Two tipTree computes (gate + runArbiterPhase)? | Accept the recompute (runArbiterPhase keeps deriving it for TreeDiff). | Architecture §6.7: cheap slice index; minimal-change preference. Passing tipTree in is cosmetic. |
| D9 | Scope vs siblings? | ONLY decompose.go (gate + runArbiterPhase) + chain.go (resolveArbiter + 3 paths + remove helper) + chain_test.go. NOT decompose_test.go (P1.M1.T3), NOT docs (P1.M1.T2.S2). | Contract: "code-only; the doc rides with the task (P1.M1.T2.S2)." |
