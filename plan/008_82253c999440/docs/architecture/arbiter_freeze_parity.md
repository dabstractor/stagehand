# Arbiter Freeze Parity (FR-M1d) + `OverlayTreePaths` git primitive

**Scope:** v2.2 delta (plan/008), Phase 1 — close the arbiter freeze loophole so a working-tree
file written after `T_start` capture can never land in an arbiter commit, on any of the three
resolution paths (new / tip-amend / mid-chain). Add a new `OverlayTreePaths` git primitive for
the mid-chain path. All work is inside `internal/decompose` + `internal/git`.

This is a RESEARCH/DOCUMENT artifact only — no code changed in this run.

---

## 1. CURRENT state (verified seams, exact line numbers)

### 1.1 The arbiter GATE — live `StatusPorcelain` (the loophole)

`internal/decompose/decompose.go:213-227` is the gate. Today it reads the **live** working tree:

```go
// decompose.go:213-227
amended := 0
status, err := deps.Git.StatusPorcelain(ctx)        // ← LIVE working tree (LOOOPHOLE)
if err != nil {
    return DecomposeResult{}, fmt.Errorf("%w: status: %w", ErrDecomposeFailed, err)
}
if status != "" && len(commits) > 0 {                 // ← gate condition
    arbiterCommits := buildArbiterCommits(ctx, deps, commits, isUnborn)
    amended, err = runArbiterPhase(ctx, deps, arbiterCommits, chainData, tStart)
    ...
}
```

The loophole: a file written to the working tree **after** `T_start` capture makes `status != ""`
even when the frozen leftover set is empty, so the arbiter runs and `resolveArbiter` stages that
post-freeze file into a commit.

### 1.2 `runArbiterPhase` — already frozen for the DIFF (decompose.go:603-642)

`runArbiterPhase` already computes both trees from frozen data and **holds `tStart`**:

```go
// decompose.go:603
func runArbiterPhase(ctx context.Context, deps Deps, commits []CommitInfo,
    chainData []ChainEntry, tStart string) (int, error) {
    ...
    // decompose.go:617-618 — frozen tipTree + frozen leftover DIFF (already correct)
    tipTree := chainData[len(chainData)-1].Tree
    leftoverDiff, err := deps.Git.TreeDiff(ctx, tipTree, tStart, git.StagedDiffOptions{...})
    ...
    // decompose.go:638 — calls resolveArbiter WITHOUT tStart/leftoverPaths
    if err := resolveArbiter(ctx, deps, out.Target, commits, chainData); err != nil {
```

So FR-M1d (2) (frozen diff) is **already satisfied**. Only the gate (1.1) and the staging (1.3)
leak.

### 1.3 `resolveArbiter` + the THREE resolution paths (chain.go) — live-tree staging

Signature today (`chain.go:50`):

```go
func resolveArbiter(ctx context.Context, deps Deps, target *string,
    commits []CommitInfo, chainData []ChainEntry) error
```

All three paths call live-tree mutation primitives:

| Path | Function | Live-tree calls (chain.go) | What it produces |
|---|---|---|---|
| A. new commit | `resolveNewCommit` (~line 76) | `AddAll` (~96), `WriteTree` (~103) | `treePrime` = staged-from-working-tree |
| B. tip amend | `resolveTipAmend` (~line 128) | `AddAll` (~141), `WriteTree` (~146) | `treePrime` = staged-from-working-tree |
| C. mid-chain | `resolveMidChain` (~line 170) | `StatusPorcelain` (~184), per-j `ReadTree` (~205), per-j `Add(paths)` (~209), `WriteTree` (~212) | per-j `treePrime` via folding live leftovers onto `tree[j]` |

Exact live-tree mutations:
- `resolveNewCommit`: `chain.go:99` `deps.Git.AddAll(ctx)`, `chain.go:106` `deps.Git.WriteTree(ctx)`.
- `resolveTipAmend`: `chain.go:142` `deps.Git.AddAll(ctx)`, `chain.go:147` `deps.Git.WriteTree(ctx)`.
- `resolveMidChain`: `chain.go:184` `deps.Git.StatusPorcelain(ctx)` (parses via `leftoverPaths`), then
  for each `j`: `chain.go:205` `ReadTree(chainData[j].Tree)`, `chain.go:209` `Add(paths)`,
  `chain.go:212` `WriteTree()`.

`leftoverPaths` (`chain.go:~248`) parses the porcelain string into the fold set. Under FR-M1d it is
**replaced** by a precomputed `DiffTreeNames(tipTree, tStart)` passed in from `runArbiterPhase`.

### 1.4 The `Git` interface — `OverlayTreePaths` does NOT exist

`internal/git/git.go` confirms:
- `OverlayTreePaths` — **NOT present** (grep across the package returns nothing).
- `ls-tree` / `update-index` / `--cacheinfo` / `--force-remove` — **NOT used** anywhere.
- `EmptyTreeSHA` constant — **PRESENT** at `git.go:641`:
  `const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"`.

Interface / impl positions (verified):

| Primitive | Interface doc-block line | `gitRunner` impl |
|---|---|---|
| `WriteTree` | git.go:~92 | git.go (per `grep` ~Line 660 area) |
| `Add` | git.go:~140 | git.go (~1280 area) |
| `ReadTree` | git.go:~168 | git.go:~1208 (`func (g *gitRunner) ReadTree`) |
| `TreeDiff` | git.go:~173 | git.go:~1225 area |
| `FreezeWorkingTree` | git.go:~245 | **git.go:1548** |
| `DiffTreeNames` | git.go:~275 | **git.go:1568** |
| `DiffTreeNameStatus` | git.go (interface) | git.go:1724 |

`FreezeWorkingTree` (git.go:1548-1566) is the structural template: it orchestrates `AddAll` +
`WriteTree` + `ReadTree`, mutating only `.git/index` + the object store, never the working tree,
never a ref. `OverlayTreePaths` mirrors exactly this discipline.

### 1.5 Test patterns

**`internal/git/*_test.go`** (temp-repo pattern): every test does `repo := t.TempDir(); initRepo(t, repo); …`
then exercises `New(repo).<Method>(ctx, …)` and asserts against an **independent** `git` oracle
(`exec.Command("git", "-C", repo, …)`). Shared helpers (all package-internal):
- `initRepo` (`git_test.go:13`) — `git init` + repo-local `user.name`/`user.email`.
- `writeFile` (`committree_test.go:31`), `stageFile` (`committree_test.go:39`),
  `makeEmptyCommit` (`revparse_test.go:24`), `writeTreeOf` (`committree_test.go:48`),
  `execGit` (`revparsetree_test.go:115`).
- Standard negative cases per primitive: `BadTree`/`NotARepo`/`GitBinaryMissing`/`ContextCancelled`
  (see `readtree_test.go`, `freezeworkingtree_test.go` for the canonical four-case shape).

**`internal/decompose/*_test.go`** — `chn*`-prefixed helpers in `chain_test.go` (`chnInitRepo`,
`chnWriteFile`, `chnStageFile`, `chnCommitRaw`, `chnRunGit`, `chnHeadSHA`, `chnBuildChain`,
`chnDeps`). `chnBuildChain` constructs a 3-commit chain `C0/C1/C2` each with a unique file
(`c0.go`/`c1.go`/`c2.go`) plus an untracked `leftover.go`, returning parallel `[]CommitInfo` +
`[]ChainEntry`. The message manifest is a `stubtest.Manifest(bin, Options{Out: …})`. Existing tests:
- `TestResolveArbiter_NullNewCommit` — null path → N+1 commits, HEAD.tree contains `leftover.go`,
  `git status` clean.
- `TestResolveArbiter_TipAmend` — `&tipSHA` → 3 commits, message reused verbatim, both files in HEAD.tree.
- `TestResolveArbiter_MidChainRebuild` — `&sha1` → 3 commits; C0 unchanged, C1' & C2' rebuilt;
  `leftover.go` folded into **every** rebuilt tree (G-FOLD at every `j`).
- `TestResolveArbiter_CleanTreePostcondition` — `status --porcelain == ""`, `write-tree == HEAD^{tree}`.

`decompose_test.go` carries the **`dcm*`**-prefixed fixtures and the existing FR-M1b/M1c test pair:
- `TestDecompose_StagerFreezeViolation` (lines 736-784) — STAGES the post-freeze sentinel → `ErrFreezeViolation`.
- `concurrentSentinelSeam` (lines 795-815) + `TestDecompose_ConcurrentChangeExclusion`
  (lines 818-882) — writes the sentinel **unstaged**; today the existing comment at lines 838 & 880-882
  admits: *"the arbiter's STAGING (resolveArbiter's AddAll) picks up sentinel.txt from the working tree"*
  — that test currently only asserts on the **loop** commits, precisely because the arbiter loophole
  is open. **This test is the natural upgrade target for FR-M1d** (now also assert the sentinel is in
  no arbiter commit).

---

## 2. TARGET state per FR-M1d / FR-M9 / FR-M10

### 2.1 Gate → frozen leftover (decompose.go:213-227)

Replace `StatusPorcelain` with `DiffTreeNames(tipTree, tStart)` (both trees already held in scope;
`tipTree = chainData[len(chainData)-1].Tree`). Empty set → arbiter does not run.

```go
// TARGET — decompose.go (sketch)
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

Note: `tipTree` is computed **twice** (here and inside `runArbiterPhase` at line 617). Either pass
`tipTree` into `runArbiterPhase` or accept the cheap recompute. The cleanest minimal change is to
**pass `leftoverPaths` into `runArbiterPhase`** and let it keep deriving `tipTree` internally.

### 2.2 `runArbiterPhase` signature + body

```go
func runArbiterPhase(ctx context.Context, deps Deps, commits []CommitInfo,
    chainData []ChainEntry, tStart string, leftoverPaths []string) (int, error) {
    ...
    // line 617 — unchanged
    tipTree := chainData[len(chainData)-1].Tree
    leftoverDiff, err := deps.Git.TreeDiff(ctx, tipTree, tStart, git.StagedDiffOptions{...}) // already frozen
    ...
    // line 638 — CHANGED: pass tStart + leftoverPaths
    if err := resolveArbiter(ctx, deps, out.Target, commits, chainData, tStart, leftoverPaths); err != nil {
        return 0, err
    }
    return amended, nil
}
```

### 2.3 `resolveArbiter` signature change (chain.go:50)

```go
func resolveArbiter(ctx context.Context, deps Deps, target *string,
    commits []CommitInfo, chainData []ChainEntry, tStart string, leftoverPaths []string) error
```

Branch dispatch (unchanged) routes to the three paths; each now receives `tStart` and `leftoverPaths`.

### 2.4 Path A — new commit (`resolveNewCommit`): `treePrime = T_start`

- **Remove** `AddAll` (chain.go:99) + `WriteTree` (chain.go:106).
- Set `treePrime := tStart` (mirrors `runOneFileShortcut` decompose.go:285 and
  `runSingleShortcut` decompose.go:321 — identical FR-M2b/M11 pattern).
- `generateMessage(ctx, deps, treeA, treePrime)` — `treePrime == tStart`, so this diffs `tip → T_start`
  (exactly the leftovers, no concurrent content).
- After `UpdateRefCAS` succeeds, **sync the index** with `deps.Git.ReadTree(ctx, tStart)`
  (mirrors the post-commit sync in `runOneFileShortcut`/`runSingleShortcut`). This is the "after all
  three paths, sync the index to `T_start`" guarantee from FR-M1d (3).

### 2.5 Path B — tip amend (`resolveTipAmend`): `treePrime = T_start`

- **Remove** `AddAll` (chain.go:142) + `WriteTree` (chain.go:147).
- `treePrime := tStart`. The rest of the amend plumbing (`CommitTree(tStart, [tipParent], tipMsg)`
  + `UpdateRefCAS(expectedOld = tipSHA)`) is unchanged.
- After CAS succeeds, `deps.Git.ReadTree(ctx, tStart)` to sync the index.

### 2.6 Path C — mid-chain (`resolveMidChain`): `OverlayTreePaths(tree[j], T_start, leftoverPaths)`

- **Remove** `StatusPorcelain` + `leftoverPaths(...)` (chain.go:184 + the helper call). `leftoverPaths`
  is now a parameter.
- For each `j := i; j < N; j++`:
  - `treePrime, err := deps.Git.OverlayTreePaths(ctx, chainData[j].Tree, tStart, leftoverPaths)`
  - `CommitTree(treePrime, [rebuiltParent], chainData[j].Message)` — message reused verbatim (unchanged).
- Final `UpdateRefCAS(HEAD, rebuiltParent, tipSHA)` unchanged. The rebuilt tip equals `T_start`
  (because `OverlayTreePaths(tree[N-1], T_start, leftoverPaths)` overlays every changed path ⇒ `T_start`).
- After CAS succeeds, `deps.Git.ReadTree(ctx, tStart)` to sync the index (replaces the implicit
  "index ends at tree[N-1]" invariant from the old `ReadTree`/`Add` loop).

`leftoverPaths` (the helper function, chain.go:~248) becomes **dead code** under FR-M1d and can be
removed alongside `TestLeftoverPaths`. (Alternatively keep it for the chain_test that pins its
behaviour, but it has no live call site after the change.)

### 2.7 Why `OverlayTreePaths` not `ReadTree(tStart)` for the mid-chain

Path C folds the **leftovers** onto each intermediate `tree[j]`, not onto `T_start` directly. A path
that is **unchanged between `tree[j]` and `T_start`** must keep `tree[j]`'s blob (e.g. `c1.go` in C1),
while a leftover path must take `T_start`'s blob. `OverlayTreePaths(baseTree=tree[j], sourceTree=T_start,
paths=leftoverPaths)` is exactly that operation: it copies `(mode, blob)` from `T_start` **only** for
paths in `leftoverPaths`, leaving every other path at `tree[j]`'s value. (For `j == N-1`,
`tree[j] + leftovers == T_start` exactly — that is the rebuilt-tip-equals-`T_start` invariant.)

---

## 3. `OverlayTreePaths` design (new git primitive)

### 3.1 Interface signature (add to `Git` in git.go, near `DiffTreeNames` ~line 275 / `FreezeWorkingTree` ~line 245)

```go
// OverlayTreePaths returns a NEW tree equal to baseTree with each path in paths overwritten by its
// state in sourceTree (FR-M10 mid-chain rebuild). For each path:
//   - present in sourceTree  → its (mode, blob) from sourceTree replaces baseTree's entry;
//   - absent in sourceTree   → removed from the result (deletion-overlay).
// paths NOT in `paths` retain their (mode, blob) from baseTree unchanged. Empty paths ⇒ returns
// baseTree verbatim (no-op). Mutates ONLY .git/index + the object store (same discipline as
// FreezeWorkingTree/ReadTree/WriteTree): never touches the working tree, never moves a ref. At its
// sole call site sourceTree == T_start and paths == leftoverPaths (DiffTreeNames(tipTree, tStart)).
OverlayTreePaths(ctx context.Context, baseTree, sourceTree string, paths []string) (treeSHA string, err error)
```

### 3.2 Implementation steps (new `gitRunner.OverlayTreePaths`, place near `FreezeWorkingTree` git.go:1548)

1. `paths` empty ⇒ return `baseTree` (early no-op — avoids a pointless `write-tree`).
2. `read-tree baseTree` → index = `baseTree`.
3. Read `(mode, blob)` for every requested path from `sourceTree` in ONE call:
   `git ls-tree -r --full-tree <sourceTree> -- <paths...>`. Parse each line
   `<mode> <type> <blob>\t<path>` into a `map[path]→{mode, blob}`.
   - `-r` recurses; `--full-tree` disables the `core.quotepath`/path-trimming quirks; the `-- <paths>`
     pathspec limits the listing to the requested set (one round-trip, not N).
4. For each `path` in `paths`:
   - **present** in the map → `git update-index --cacheinfo <mode>,<blob>,<path>`
     (single-arg `<mode>,<blob>,<path>` form, git 2.0+; one argv element after `--cacheinfo`).
   - **absent** in the map (the path was deleted between `baseTree` and `sourceTree`) →
     `git update-index --force-remove <path>` (removes the entry from the index unconditionally).
5. `write-tree` → return the new tree SHA.

Exit-code convention: every sub-command uses the **mutation** form (`code != 0 ⇒ error`, NO
`128-as-non-error` special case) — identical to `ReadTree`/`WriteTree`/`Add`. `ls-tree` is read-only;
`code != 0 ⇒ error` (bad/unresolvable `sourceTree` SHA).

### 3.3 Pseudocode

```go
func (g *gitRunner) OverlayTreePaths(ctx context.Context, baseTree, sourceTree string, paths []string) (string, error) {
    if len(paths) == 0 {
        return baseTree, nil // no-op
    }
    // 1. index = baseTree
    if _, stderr, code, err := g.run(ctx, g.workDir, "read-tree", baseTree); err != nil {
        return "", err
    } else if code != 0 {
        return "", fmt.Errorf("git read-tree (overlay base): failed (exit %d): %s", code, strings.TrimSpace(stderr))
    }
    // 2. ls-tree sourceTree for the requested paths (one round-trip)
    lsArgs := append([]string{"ls-tree", "-r", "--full-tree", sourceTree, "--"}, paths...)
    lsOut, stderr, code, err := g.run(ctx, g.workDir, lsArgs...)
    if err != nil { return "", err }
    if code != 0 { return "", fmt.Errorf("git ls-tree (overlay source): failed (exit %d): %s", code, strings.TrimSpace(stderr)) }
    blobs := parseLsTree(lsOut) // map[path]→{mode, blob}
    // 3. update-index per path (cacheinfo or force-remove)
    for _, p := range paths {
        if ent, ok := blobs[p]; ok {
            if _, stderr, code, err := g.run(ctx, g.workDir, "update-index", "--cacheinfo",
                fmt.Sprintf("%s,%s,%s", ent.mode, ent.blob, p)); err != nil {
                return "", err
            } else if code != 0 {
                return "", fmt.Errorf("git update-index --cacheinfo %s: failed (exit %d): %s",
                    p, code, strings.TrimSpace(stderr))
            }
        } else {
            if _, stderr, code, err := g.run(ctx, g.workDir, "update-index", "--force-remove", p); err != nil {
                return "", err
            } else if code != 0 {
                return "", fmt.Errorf("git update-index --force-remove %s: failed (exit %d): %s",
                    p, code, strings.TrimSpace(stderr))
            }
        }
    }
    // 4. write-tree
    return g.WriteTree(ctx)
}
```

`parseLsTree` splits each line on `\t` (path on the right) and the left side on spaces into
`<mode> <type> <blob>`; ignore `<type>` (always `blob` for `-r` over file paths; symlink modes carry
`mode 120000` correctly because the type column is `blob`, not `commit`).

### 3.4 Interface placement

Add the interface doc-block to `Git` immediately after `DiffTreeNames` (git.go:~275) — keeps the
freeze/tree-primitive cluster (`FreezeWorkingTree`, `DiffTreeNames`, `OverlayTreePaths`) together.

---

## 4. The `ReadTree(T_start)` index-sync after each path

Every resolution path leaves the index at `T_start` after CAS succeeds, mirroring the existing
single-shortcut pattern (`runOneFileShortcut` decompose.go:300, `runSingleShortcut` decompose.go:356).
This guarantees `git status` is clean for the committed set: `index == HEAD.tree == T_start`. A
concurrent working-tree change remains unstaged/untracked (it is not in `T_start`).

- Path A: append `ReadTree(tStart)` after `UpdateRefCAS`.
- Path B: append `ReadTree(tStart)` after `UpdateRefCAS`.
- Path C: append `ReadTree(tStart)` after `UpdateRefCAS` (replaces the old "final ReadTree leaves
  index at tree[N-1]" — now `T_start == OverlayTreePaths(tree[N-1], T_start, leftoverPaths)`).

`ErrArbiterResolutionFailed` wrap on a sync failure is fine — the commit already landed (CAS
succeeded); a dirty index post-run is a best-effort concern, not a correctness one.

---

## 5. Test invariants to add (acceptance proof for FR-M1d)

All three live in `internal/decompose`; the `chn*` helpers in `chain_test.go` build the chain + leftover
fixtures. The `dcm*` helpers in `decompose_test.go` drive the full `Decompose()` integration path.

### 5.1 Concurrent-across-arbiter-gate (the loophole-closure proof)

Upgrade **`TestDecompose_ConcurrentChangeExclusion`** (decompose_test.go:823) — it already writes an
unstaged `sentinel.txt` post-freeze via `concurrentSentinelSeam`. Today its comment (lines 838, 880-882)
admits the arbiter would sweep the sentinel. After FR-M1d, also assert:
- (a) `sentinel.txt` is in **no** commit, including any arbiter commit (`git log --name-only` does
  not contain it).
- (b) `sentinel.txt` **remains** in the working tree post-run (`git status --porcelain` shows
  `?? sentinel.txt`).
- (c) **No arbiter commit is created when the frozen leftover is empty**: drive a 2-concept run whose
  stagers cover ALL of `T_start` (so `DiffTreeNames(tipTree, tStart) == []`); write a sentinel
  post-freeze; assert `rev-list --count` == exactly 2 (no N+1 arbiter commit) AND the sentinel is
  unstaged in the working tree. **This is the case that fails today** — the live gate sees the
  sentinel via `StatusPorcelain` and runs the arbiter.

### 5.2 Arbiter folds only `T_start` content

New test (decompose_test.go, sibling of 5.1): a 2-concept run where concept-1's stager is a no-op
(empty-skip) so a **legitimate** leftover exists (e.g. `b.go` unclaimed). Drive the arbiter (null
target). Then write a sentinel `concurrent.txt` post-freeze (unstaged). Assert:
- Each arbiter commit's tree (`HEAD^{tree}`) is **exactly** `T_start` (null/tip) or an
  `OverlayTreePaths(tree[j], T_start, leftoverPaths)` overlay (mid-chain).
- The sentinel is in **no** commit AND remains in the working tree.

Add the parallel **unit-level** proof in `chain_test.go`: extend `chnBuildChain` (or add a variant)
to capture `tStart` (the tree holding `c0.go`+`c1.go`+`c2.go`+`leftover.go`) and write a
post-freeze `sentinel.go`. Call `resolveArbiter(ctx, deps, &target, commits, chainData, tStart,
leftoverPaths)` directly with `leftoverPaths = []string{"leftover.go"}`. For each of null / `&tipSHA` /
`&sha1`, assert `HEAD^{tree}` (and every rebuilt `Cj'^{tree}` for mid-chain) does **not** contain
`sentinel.go`, and `sentinel.go` is untracked in `git status` post-call. Update the existing
`TestResolveArbiter_*` calls to the new 7-arg signature.

### 5.3 `T_start` completeness (replaces the "Loop index cleanliness" invariant)

After a fully-successful run, every `T_start` leftover is committed: `DiffTreeNames(tipTree, tStart)`
on the final HEAD equals the **empty** set (all of `T_start` is in HEAD). Live `git status` may be
non-empty only from changes **outside** `T_start` (the post-freeze sentinel). Add to 5.1/5.2:
- `DiffTreeNames(baseTree, tStart)` (the full frozen change set) ⊆ `DiffTreeNames(baseTree,
  HEAD^{tree})` after the run (every frozen path landed).
- `git status --porcelain` may show `?? sentinel.txt` (and only paths NOT in `T_start`).

### 5.4 `OverlayTreePaths` unit tests (new `internal/git/overlaytree_test.go`)

Canonical four-case + behaviour set, mirroring `freezeworkingtree_test.go` / `readtree_test.go`:
- `TestOverlayTreePaths_OverlaysOnlyListedPaths` — `baseTree` has `a.go`+`b.go`; `sourceTree` has
  `a.go'`+`c.go`; `paths=["a.go","c.go"]` ⇒ result tree has `a.go'` (from source) + `b.go`
  (unchanged from base) + `c.go` (added from source). Assert via `git ls-tree -r` oracle.
- `TestOverlayTreePaths_DeletionOverlay` — path in `paths` but ABSENT in `sourceTree` ⇒ removed
  from result (the deletion-overlay case from the risk list).
- `TestOverlayTreePaths_EmptyPathsNoOp` — `paths == nil` ⇒ returns `baseTree` verbatim (no index
  mutation; assert `git ls-files` unchanged).
- `TestOverlayTreePaths_OverlaysLeftoversMidChain` — the in-simulation case: build `tree0/c0.go`,
  `tree1/c0+c1`, `tStart = c0+c1+leftover`, `paths = DiffTreeNames(tree1, tStart) = [leftover.go]`;
  `OverlayTreePaths(tree1, tStart, paths)` ⇒ tree holding `c0+c1+leftover` == overlaying leftovers
  onto an intermediate tree. Assert `resultTree` equals the hand-built `c0+c1+leftover` tree.
- `TestOverlayTreePaths_BadTree` / `_NotARepo` / `_GitBinaryMissing` / `_ContextCancelled` — the
  standard four-case negative set (mirror `readtree_test.go`).
- `TestOverlayTreePaths_DoesNotTouchWorkingTree` — write a working-tree file, call
  `OverlayTreePaths`, assert the file is unchanged on disk (mirrors
  `TestFreezeWorkingTree_LeavesWorkingTreeUnchanged`).

---

## 6. Risks / gotchas

1. **Deletion-overlay case.** A path in `leftoverPaths` that is ABSENT in `sourceTree == T_start`
   means `T_start` deleted it relative to `tipTree`. `OverlayTreePaths` must `--force-remove` it
   (not skip it). Concretely: if `c2.go` was deleted in the run and re-added by the loop (so it is
   in `tree[j]` but not in `T_start`), the overlay must drop it. The `--force-remove` branch (3.2
   step 4) handles this; the unit test 5.4 pins it.
2. **`leftoverPaths` computation moves.** Today `resolveMidChain` parses `StatusPorcelain` (index-
   relative). After FR-M1d it is `DiffTreeNames(tipTree, tStart)` (tree-to-tree), computed once in
   `runArbiterPhase`. **Semantically equivalent on the happy path** (post-loop `index == tipTree`,
   so porcelain-vs-`DiffTreeNames` agree), but the porcelain form included `??` untracked paths
   that `DiffTreeNames(tipTree, tStart)` ALSO includes (anything added in `T_start` is in
   `DiffTreeNames`). The one observable difference: a **post-freeze** untracked file appears in
   porcelain but NOT in `DiffTreeNames(tipTree, tStart)` — which is exactly the freeze property
   FR-M1d is enforcing. `leftoverPaths` (the chain.go helper) and `TestLeftoverPaths` become dead
   code; remove or leave as documentation.
3. **Unborn repo base.** `tipTree` is `chainData[N-1].Tree` (always non-empty when the loop ran —
   the loop makes ≥1 commit to reach the gate). `tStart` may equal `EmptyTreeSHA` only on a
   one-concept unborn run, which routes through `runOneFileShortcut` / `runSingleShortcut` (NOT the
   arbiter). So the arbiter paths never see `tStart == EmptyTreeSHA` in practice — but
   `OverlayTreePaths` should still tolerate `baseTree == EmptyTreeSHA` (the test 5.4
   `_OverlaysLeftoversMidChain` does not exercise this; add an explicit case if defensive coverage
   is wanted).
4. **`update-index --cacheinfo` argv form.** The modern single-arg `<mode>,<blob>,<path>` form
   (git 2.0+, Apr 2014) is universally available; do NOT use the legacy 3-arg form. The comma
   inside the single argv element is safe because `run()` builds `exec.CommandContext` argv with NO
   shell (PRD §19).
5. **`ls-tree` pathspec vs path-with-spaces.** `git ls-tree -r --full-tree <tree> -- <paths...>`
   passes each path as a separate argv element (no shell), so spaces in paths are safe. `core.quotepath`
   (default on) C-quotes non-ASCII paths in `ls-tree` output — same documented limitation as the
   existing `leftoverPaths` helper (chain.go comment: "v1 ASSUMES ASCII"). If a path is non-ASCII,
   the `ls-tree` line's path column will be C-quoted and `parseLsTree`'s `\t` split will yield the
   quoted form, which then will NOT match the (unquoted) `leftoverPaths` entry → the path is treated
   as deletion-overlay and `--force-remove`d. This is a pre-existing limitation, not a regression,
   but worth a code comment.
6. **`--full-tree` flag.** Without it, `ls-tree` truncates/quotes paths the same way; `--full-tree`
   disables the path quoting for the TAB delimiter. Verify empirically in the unit test that the
   `\t` split is reliable (the test 5.4 path set is ASCII, so this is not exercised — leave a
   comment pointing at the quotepath limitation).
7. **Two `tipTree` computes.** The gate (2.1) and `runArbiterPhase` (line 617) both derive
   `tipTree = chainData[len(chainData)-1].Tree`. Cheap (slice index), but for cleanliness consider
   passing `tipTree` into `runArbiterPhase` alongside `leftoverPaths`. Minimal-change preference:
   recompute (it's a one-line index, and `runArbiterPhase` already does it for the `TreeDiff`).
8. **`docs/how-it-works.md` Mode A ride-along.** FR-M1d (3) docs require the decompose/arbiter
   narrative to state the gate/diff/staging are frozen. Out of scope for THIS research artifact; the
   implementing task owns it.
9. **`rereadFinalCommits` unaffected.** Post-arbiter re-read (`decompose.go:565` area) reads
   `preRunHEAD..HEAD` via `LogRange` — unchanged by FR-M1d; the rebuilt commits are real objects
   regardless of how their trees were built.

---

## 7. Files to change (summary for the implementing agent)

| File | Change |
|---|---|
| `internal/git/git.go` | Add `OverlayTreePaths` to the `Git` interface (after `DiffTreeNames` ~line 275) + `gitRunner.OverlayTreePaths` impl (near `FreezeWorkingTree` line 1548). Add a small `parseLsTree` helper. |
| `internal/git/overlaytree_test.go` (NEW) | The 5.4 test set. |
| `internal/decompose/decompose.go` | (1) Gate `decompose.go:213-227` → `DiffTreeNames(tipTree, tStart)`. (2) `runArbiterPhase` (line 603) signature + the `resolveArbiter` call (line 638) → pass `tStart, leftoverPaths`. |
| `internal/decompose/chain.go` | (1) `resolveArbiter` (line 50) + the three paths take `tStart, leftoverPaths`. (2) `resolveNewCommit` (line ~76) → `treePrime = tStart`, drop `AddAll`/`WriteTree`, add `ReadTree(tStart)`. (3) `resolveTipAmend` (line ~128) → same. (4) `resolveMidChain` (line ~170) → `OverlayTreePaths(tree[j], tStart, leftoverPaths)`, drop `StatusPorcelain`/`ReadTree`/`Add`/`WriteTree`, add `ReadTree(tStart)`. (5) Remove `leftoverPaths` helper (~line 248) OR leave as dead code. |
| `internal/decompose/chain_test.go` | Update `TestResolveArbiter_*` to the 7-arg signature; add the sentinel/`T_start`-completeness assertions (5.2). Remove `TestLeftoverPaths` if the helper is deleted. |
| `internal/decompose/decompose_test.go` | Upgrade `TestDecompose_ConcurrentChangeExclusion` (line 823) with the 5.1 / 5.3 assertions; add the 5.2 paired case. |
| `docs/how-it-works.md` | Mode A ride-along (FR-M1d (3)) — out of scope here. |

---

## 8. Start here

Open `internal/decompose/chain.go` first — it has all three resolution paths and is where the bulk of
the live-tree removal happens. Then `internal/git/git.go` (around line 1548, `FreezeWorkingTree`) as
the structural template for the new `OverlayTreePaths`. Finally `internal/decompose/decompose.go`
lines 213-227 (gate) and 603-642 (`runArbiterPhase`) for the two small signature/threading edits.

```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "Research/document task only — no scope widened. Wrote one architecture artifact to the authoritative path plan/008_82253c999440/architecture/arbiter_freeze_parity.md covering CURRENT state (exact line numbers), TARGET state, OverlayTreePaths design, resolveArbiter signature change, ReadTree(T_start) sync, test invariants, and risks. All five requested seams verified against source: decompose.go gate (213-227), runArbiterPhase (603-642, already frozen), chain.go three paths (AddAll/Add/StatusPorcelain line numbers captured), git.go interface (OverlayTreePaths absent, EmptyTreeSHA present at 641, FreezeWorkingTree impl at 1548, DiffTreeNames at 1568, ReadTree at ~1208), test patterns (git/*_test.go temp-repo shape, chain_test.go chn* helpers, decompose_test.go concurrentSentinelSeam at 795)."
    }
  ],
  "changedFiles": [
    "plan/008_82253c999440/architecture/arbiter_freeze_parity.md"
  ],
  "testsAddedOrUpdated": [],
  "commandsRun": [
    {
      "command": "read internal/decompose/decompose.go (full)",
      "result": "passed",
      "summary": "Verified gate at 213-227, runArbiterPhase at 603-642 (already frozen), tStart threaded."
    },
    {
      "command": "read internal/decompose/chain.go (full)",
      "result": "passed",
      "summary": "Captured resolveArbiter sig (50), resolveNewCommit AddAll(99)/WriteTree(106), resolveTipAmend AddAll(142)/WriteTree(147), resolveMidChain StatusPorcelain(184)/ReadTree(205)/Add(209)/WriteTree(212), leftoverPaths helper (~248)."
    },
    {
      "command": "read internal/git/git.go (full, 1766 lines via offsets)",
      "result": "passed",
      "summary": "Confirmed Git interface, EmptyTreeSHA at 641, FreezeWorkingTree impl at 1548, DiffTreeNames at 1568, ReadTree impl at ~1208. NO OverlayTreePaths / ls-tree / update-index / cacheinfo / force-remove anywhere."
    },
    {
      "command": "grep OverlayTreePaths|ls-tree|update-index|cacheinfo|force-remove internal/git",
      "result": "passed",
      "summary": "Zero matches — primitive does not exist; no conflicting helpers."
    },
    {
      "command": "read freezeworkingtree_test.go, readtree_test.go, add_test.go, chain_test.go (full)",
      "result": "passed",
      "summary": "Captured temp-repo test shape, chn* helpers + chnBuildChain, existing TestResolveArbiter_* (null/tip/midchain/CAS/rescue/clean) invariants."
    },
    {
      "command": "grep on decompose_test.go for StatusPorcelain/runArbiterPhase/concurrent/sentinel/tStart/freeze",
      "result": "passed",
      "summary": "Found concurrentSentinelSeam (795-815) + TestDecompose_ConcurrentChangeExclusion (823-882) with the loophole-admitting comment at 838/880-882 — the 5.1 upgrade target."
    }
  ],
  "validationOutput": [
    "No build/test commands run — artifact-only task (no source changes).",
    "All line numbers cited are verified against the live source via read/grep, not assumed."
  ],
  "residualRisks": [
    "deletion-overlay path-quotepath edge case (non-ASCII path in leftoverPaths) is a pre-existing v1 limitation; documented in §6 risk 5.",
    "Two tipTree computes (gate + runArbiterPhase) — cosmetic; flagged in §6 risk 7."
  ],
  "noStagedFiles": true,
  "diffSummary": "Added one new architecture document (plan/008_82253c999440/architecture/arbiter_freeze_parity.md). No source/test files modified.",
  "reviewFindings": [
    "no blockers — research artifact only; no executable code changed"
  ],
  "manualNotes": "Authoritative output path used: plan/008_82253c999440/architecture/arbiter_freeze_parity.md. Implementing agent should start at internal/decompose/chain.go (3 paths), then git.go (OverlayTreePaths near line 1548), then decompose.go gate (213-227) + runArbiterPhase (603-642). The existing TestDecompose_ConcurrentChangeExclusion (decompose_test.go:823) is the natural upgrade target for the FR-M1d acceptance proof — its current comment explicitly admits the arbiter loophole."
}
```
