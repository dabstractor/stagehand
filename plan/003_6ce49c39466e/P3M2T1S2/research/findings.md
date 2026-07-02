# P3.M2.T1.S2 — One-file short-circuit (FR-M2b) — Empirical Findings

## §1 The contract (verbatim, from the work item)

FR-M2b: In AUTO mode (`Config.Commits == 0` AND not `Single`), AFTER `T_start` capture, if the
`T_start` changed-path count (`DiffTreeNames(baseTree, T_start)`) is EXACTLY 1: bypass the planner
ENTIRELY — stage that one file's `T_start` content, generate one message via the MESSAGE role,
publish via `commit-tree` + `update-ref`. ZERO planner agent call (the same outcome as the FR-M11
single shortcut but with NO planner round-trip). An explicit `--commits N` (N≥2) OVERRIDES this
short-circuit. Insert before `callPlanner`. DOCS: decompose.go comment.

**Research note (contract point 1):** use `DiffTreeNames(baseTree, T_start)` — NOT `StatusPorcelain`
(which includes untracked `??`). `DiffTreeNames(baseTree, tStart)` is the changed-path set of the
FROZEN tree, so the count matches EXACTLY what the run would commit (FR-M1b). Deterministic (changed-
path COUNT), not model judgment.

## §2 The insertion point (decompose.go — current on-disk line numbers)

`Decompose()` (decompose.go:139) routing, AFTER P3.M1.T1.S2 wired the freeze:

```
decompose.go:143   if deps.Config.Single || deps.Config.Commits == 1 → runSingleEscape  (escape-hatch, ABOVE the freeze)
decompose.go:148   RevParseHEAD → preRunHEAD, isUnborn
decompose.go:152   baseTree = EmptyTreeSHA (unborn) OR RevParseTree("HEAD")
decompose.go:164   tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)   ← FR-M1b freeze (index reset → baseTree)
decompose.go:172   out, err := callPlanner(ctx, deps, deps.Config.Commits, isUnborn, baseTree, tStart)   ← planner call
decompose.go:178   if out.Single → runSingleShortcut(...)   ← FR-M11
decompose.go:185   commits, chainData, err := runLoop(ctx, deps, out.Commits, baseTree, preRunHEAD, isUnborn)
```

**FR-M2b insertion: BETWEEN line 164 (freeze) and line 172 (callPlanner).** By this point the
escape-hatch (Single || Commits==1) has ALREADY returned at line 143, so `Single` is false and
`Commits` is 0 (auto) or ≥2 (forced). The auto-mode guard is therefore `deps.Config.Commits == 0`
(`!deps.Config.Single` is structurally true here — include it for contract-fidelity/defense).

Natural insertion:
```go
// decompose.go, AFTER the freeze (line 164 err-check), BEFORE callPlanner (line 172):
// FR-M2b one-file short-circuit (auto mode): exactly one changed path ⇒ bypass the planner ENTIRELY.
if deps.Config.Commits == 0 && !deps.Config.Single {
    changedPaths, err := deps.Git.DiffTreeNames(ctx, baseTree, tStart)
    if err != nil {
        return DecomposeResult{}, fmt.Errorf("%w: one-file check diff-tree-names: %w", ErrDecomposeFailed, err)
    }
    if len(changedPaths) == 1 {
        return runOneFileShortcut(ctx, deps, preRunHEAD, isUnborn, baseTree, tStart)
    }
}
```

**NOTE (parallel S1):** P3.M2.T1.S1 (freeze enforcement) ALSO computes
`DiffTreeNames(baseTree, tStart)` INSIDE `runLoop` (as `tStartPaths`). That is an INDEPENDENT
computation in a different function — no conflict, minor one-liner duplication. Both are read-only
`git diff-tree` calls. S1 edits `runLoop` (+ its call site + stager.go); THIS task edits the
`Decompose()` routing block + adds `runOneFileShortcut` (placed near `runSingleShortcut`). The two
tasks touch NON-OVERLAPPING regions of decompose.go.

## §3 The pattern to mirror: runSingleShortcut (FR-M11) — decompose.go:244-268

```go
func runSingleShortcut(ctx context.Context, deps Deps, plannerMsg, preRunHEAD string, isUnborn bool, baseTree, tStart string) (DecomposeResult, error) {
    treePrime := tStart                                                    // commit T_start DIRECTLY (freeze-safe)
    msg := plannerMsg
    if dupCheckMessage(ctx, deps, plannerMsg, isUnborn) {                  // dup → fallback to message agent
        msg, err = generateMessage(ctx, deps, baseTree, tStart) ...
    }
    newSHA, err := publishCommit(ctx, deps, treePrime, preRunHEAD, msg)    // commit-tree + update-ref CAS
    ...
    cr, err := buildCommitResult(ctx, deps, newSHA, msg, isUnborn) ...
    return DecomposeResult{Commits: []CommitResult{cr}, Amended: 0}, nil
}
```

FR-M2b differs from FR-M11 in exactly ONE way: there is NO planner message (the planner was NEVER
called). So the message is ALWAYS generated via the message agent (`generateMessage`), with no
dup-check-of-planner-msg branch. The publish tail (publishCommit → buildCommitResult → return) is
IDENTICAL.

**Reusable primitives (CONSUME — do NOT redefine):**
- `generateMessage(ctx, deps, baseTree, tStart)` (message.go) — the MESSAGE-role generate/dedupe/parse
  loop over the tree-to-tree diff. Returns the message OR `*generate.RescueError` (propagate DIRECTLY).
- `publishCommit(ctx, deps, tree, parentSHA, msg)` (message.go) — `commit-tree` + `update-ref` CAS.
  parentSHA = preRunHEAD (root if ""). Returns newSHA OR `*generate.CASError` (propagate DIRECTLY).
- `buildCommitResult(ctx, deps, newSHA, msg, isRoot)` (decompose.go) — builds a CommitResult (DiffTree).
- `DiffTreeNames(ctx, treeA, treeB)` (git.go:1240) — sorted/deduped changed-path set (nil for identical).

## §4 THE CRITICAL FINDING: committing T_start directly leaves a STALE INDEX (must sync)

**Empirically verified** (see §6). The freeze (`FreezeWorkingTree`, git.go:1220) does:
`AddAll → WriteTree → ReadTree(baseTree)`. After it returns, **index == baseTree** (the working-tree
files on disk are UNCHANGED — `read-tree` only rewrites `.git/index`).

`runSingleShortcut` commits `tStart` directly via `commit-tree` + `update-ref` WITHOUT touching the
index. Result: **HEAD.tree == tStart, but index == baseTree** (stale). `git status --porcelain` then
shows the file as `MM` (both staged-revert AND unstaged) — confusing/wrong.

For FR-M2b's `runOneFileShortcut`, the SAME stale-index state would result if it merely mirrored
`runSingleShortcut`. **This PRP makes `runOneFileShortcut` leave a CLEAN index** by calling
`deps.Git.ReadTree(ctx, tStart)` AFTER a successful publish (tStart == HEAD.tree post-commit ⇒ index
synced to HEAD.tree ⇒ `git status` clean). This is the CORRECT post-state. It is freeze-safe (tStart
is the frozen tree; `ReadTree` never reads the working tree).

**This is MORE correct than the current `runSingleShortcut`** (which has the SAME latent stale-index
issue — a side-effect of P3.M1.T1.S2's freeze change; its tests do not assert `git status` cleanliness,
so it shipped undetected). Fixing `runSingleShortcut` is OUT OF SCOPE for FR-M2b (it is FR-M11, owned
by the completed P3.M1.T1.S2). This PRP documents it as a known finding + recommends a follow-up
(the same one-line `ReadTree(tStart)` applies). The PRP's acceptance gates do NOT require touching
`runSingleShortcut`.

**Why ReadTree(tStart) on success-only:** on a publish failure (CAS / commit-tree error) we return
early WITHOUT touching the index (index stays at baseTree as the freeze left it — the rescue recipe
uses the tree SHA, not the index). On success, `ReadTree(tStart)` syncs the index → clean. Minimal
partial-state surface.

## §5 The empirical proof (§4 verified in /tmp/freeze_test)

```
repo: init; commit "init" (a.txt="old"); modify a.txt → "new" (un-staged).
freeze: git add -A; tStart=$(git write-tree); git read-tree <baseTree>   # index → baseTree
commit tStart: newSHA=$(git commit-tree tStart -p HEAD -m "..."); git update-ref HEAD newSHA <parent>
git status --porcelain → "MM a.txt"   ← STALE (proves §4)
HEAD.tree == tStart; index.WriteTree == baseTree   ← mismatch
# Fix: git read-tree <tStart>   → index == tStart == HEAD.tree → git status clean
```

## §6 Detection edge cases (DiffTreeNames semantics — git.go:1240)

- `DiffTreeNames(baseTree, tStart)` returns the SORTED, DEDUPED changed-path set; **nil for identical
  trees** (len 0). On exactly one path, `len(...) == 1`.
- Includes ALL change kinds the freeze captured: modified (tracked), added (untracked), AND deleted
  (the freeze's `AddAll` stages deletions too). So a single deletion ⇒ count 1 ⇒ short-circuit fires.
- A **rename** shows as TWO paths (`git diff-tree --name-only` does NOT pass `-M`/`--find-renames`, so
  a rename = delete+add = 2 lines) ⇒ count 2 ⇒ short-circuit does NOT fire. This is the DETERMINISTIC
  behavior the contract wants (path count, not judgment). Document this.
- `DiffTreeNames` is read-only (no ref/index mutation). Calling it here + again inside runLoop (S1) is
  harmless (tiny redundancy).
- Precondition (FR-M1, owned by the CLI router): the working tree HAS changes ⇒ `tStart != baseTree`
  ⇒ `DiffTreeNames` is non-empty. Defensively, if it returned 0 paths, `len != 1` ⇒ no short-circuit ⇒
  falls through to the planner (safe).

## §7 Freeze enforcement (S1) is IRRELEVANT to the one-file path

P3.M2.T1.S1's `verifyFreezeSubset` runs INSIDE `runLoop` after each staging step (it checks
`tree[i] ⊆ T_start` against the external tooled stager's output). `runOneFileShortcut` RETURNS before
`runLoop` and commits `T_start` DIRECTLY (the frozen tree itself) — no stager, no `tree[i]`, no
staging step. It is trivially a content-subset of `T_start` (it IS `T_start`). So freeze enforcement
never applies to the one-file path, and there is NO ordering dependency on S1 for correctness.

## §8 Test seams + helpers (REUSE — package `decompose`, do NOT redefine)

From decompose_test.go: `dcmInitRepo` (git init + config; NO initial commit ⇒ repo is UNBORN),
`dcmWriteFile`, `dcmStageFile`, `dcmCommitRaw` (empty `--allow-empty` commit ⇒ makes repo BORN),
`dcmRunGit`, `dcmLogCount` (0 on unborn), `dcmIsUnborn`, `dcmStatusPorcelain`, `dcmDeps` (default
config: `Commits=0` auto, `Single=false`), `dcmDepsWithConfig`, `dcmPlannerManifest`,
`dcmMessageManifest`, `dcmMessageScriptManifest`, `dcmAllRoles`, `dcmStagerSeam`.

**"Agent NOT called" assertion pattern** (from `TestDecompose_SingleShortcut_CleanMessage`, line 313):
build a manifest with a `Counter` file; the counter file is created ONLY when the stub executes. Assert
the file is ABSENT (or reads "0") ⇒ the agent was never invoked. Use this for the PLANNER (to prove
FR-M2b bypasses it):
```go
counterDir := t.TempDir(); counterFile := counterDir + "/counter"
plannerM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})
// after Decompose: assert counterFile absent OR "0" (planner never called).
```

## §9 Designed tests (4 core + 1 optional edge)

1. `TestDecompose_OneFileShortcut_PlannerBypassed` — BORN repo (dcmCommitRaw "initial"), write ONE
   untracked file; planner counter-manifest (must NOT be called); message manifest "feat: ...";
   `Decompose` (auto). Assert: 1 commit; subject == message; planner counter absent/"0"; **`dcmStatusPorcelain == ""`**
   (proves the ReadTree index-sync §4); dcmLogCount == 2 (initial + 1).
2. `TestDecompose_OneFileShortcut_Unborn` — UNBORN repo, one file → short-circuit; assert planner
   counter absent/"0"; 1 commit (ROOT); dcmLogCount == 1; clean status.
3. `TestDecompose_OneFileShortcut_CommitsOverride` — `cfg.Commits = 2`, one file; planner 2-concept
   manifest + stager seam (c1 stages the file, c2 nothing → empty-skip); assert planner CALLED
   (counter != 0) ⇒ short-circuit overridden; err == nil.
4. `TestDecompose_OneFileShortcut_TwoFilesNoBypass` — TWO files; planner manifest (returns a plan);
   assert planner CALLED (counter != 0) ⇒ count==1 threshold exact.
5. (optional) `TestDecompose_OneFileShortcut_Deletion` — born repo, delete ONE tracked file →
   `DiffTreeNames` count 1 ⇒ short-circuit; assert planner not called, 1 commit, the deletion landed.

## §10 Validation gates (verified working in this repo)

```bash
gofmt -w internal/decompose/decompose.go internal/decompose/decompose_test.go
go build ./...
go vet ./...
golangci-lint run ./...
go test -race ./internal/decompose/ -run "OneFileShortcut" -v
go test -race ./internal/decompose/        # whole package — esp. SingleShortcut tests (un-touched) still GREEN
go test ./...                              # full regression
git diff --stat                            # expect decompose.go + decompose_test.go ONLY (2 files); NO git.go change; go.mod/sum UNCHANGED
```
Expected: the planner-counter tests prove the planner is bypassed on 1 file (auto) and called on
`--commits N≥2` / 2+ files; the clean-status assertion proves the index sync; the existing
SingleShortcut / happy-path tests still pass (runOneFileShortcut is a NEW path, no shared mutation).
