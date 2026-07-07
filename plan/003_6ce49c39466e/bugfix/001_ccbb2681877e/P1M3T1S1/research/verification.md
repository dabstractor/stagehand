# Verification Notes — P1.M3.T1.S1 (runSingleShortcut ReadTree index-sync, Issue 3)

> Live-tree verification of the bugfix contract (2026-07-02). Every claim below is from reading the
> current `internal/decompose/decompose.go` + `decompose_test.go` + `internal/git/git.go`. Numbered for
> cross-reference from the PRP.

## §1 — The bug is real and exactly as described

`runSingleShortcut` (decompose.go:316) commits `treePrime` (`treePrime := tStart`, assigned at the top of
the function, never reassigned) via `publishCommit` (which does `CommitTree` + `UpdateRefCAS` — touches HEAD
only, NOT the index), then goes straight to `buildCommitResult` with **no `ReadTree`**. The T_start freeze
(Decompose entry, step 3) reset the index to `baseTree`. So after a successful single-shortcut run:
`HEAD.tree == treePrime == tStart`, but `index == baseTree ≠ tStart` ⇒ `git status --porcelain` is dirty
(the just-committed files show as staged deletions + untracked). This violates §20.2 (loop-index-
cleanliness) + §18.1 (byte-for-byte-unchanged modulo dangling objects).

The sibling `runOneFileShortcut` (decompose.go:280) has the fix (the `CRITICAL (findings §4)` block, ~L294):
`deps.Git.ReadTree(ctx, tStart)` after `publishCommit` succeeds, wrapped in `ErrDecomposeFailed`.
`runSingleShortcut` lacks it. Confirmed by direct read of both function bodies.

## §2 — The fix is a precise, single-block insertion

Insert between the `publishCommit` error-return block and `buildCommitResult` in `runSingleShortcut`:

```go
if err := deps.Git.ReadTree(ctx, treePrime); err != nil {
    return DecomposeResult{}, fmt.Errorf("%w: single-shortcut index sync: %w", ErrDecomposeFailed, err)
}
```

Verified prerequisites (all in scope, no new imports):
- `treePrime` — local (`treePrime := tStart`); `ReadTree(treePrime) ≡ ReadTree(tStart)` (the reference fix).
- `deps.Git.ReadTree(ctx, tree string) error` — exists (git.go:131; replaces index with the tree's contents,
  mutates index only, never HEAD/refs).
- `ErrDecomposeFailed` — sentinel in package decompose (used by the sibling).
- `fmt` — already imported (the sibling uses `fmt.Errorf`).
- `ctx` — the function's param.

The comment mirrors `runOneFileShortcut`'s `CRITICAL (findings §4)` block (adapted: `treePrime` for `tStart`).
Error string: `"single-shortcut index sync"` (hyphenated, matching the sibling's `"one-file index sync"` and
the contract LOGIC).

## §3 — Routing: 2 files + auto mode reaches runSingleShortcut (not the escape-hatch / one-file bypass)

`Decompose` (decompose.go:124-205) routing, verified:
1. L142 `if Config.Single || Config.Commits == 1` → `runSingleEscape` (escape-hatch). My test: Single=false,
   Commits=0 ⇒ SKIP.
2. L179 `if Config.Commits == 0 && !Config.Single` → FR-M2b one-file short-circuit (→ `runOneFileShortcut`
   at L185). This fires ONLY for EXACTLY ONE changed file. My test has TWO files ⇒ SKIP.
3. L192 `callPlanner(forcedCount=Config.Commits)` (0=auto). Planner stub returns `single:true`.
4. L198 `if out.Single` → `runSingleShortcut` (L199). ✓ the target.

So a BORN repo + 2 un-staged files + default config (Commits=0, auto) + planner `{"single":true,...}`
reliably routes through the planner to `runSingleShortcut`. (Setting Commits≥2 would be WRONG — forced mode
asks the planner to partition into N, conflicting with `single:true`; auto mode is the correct path.)

## §4 — The existing test misses the bug; the new test reproduces it

`TestDecompose_SingleShortcut_CleanMessage` (decompose_test.go:314) uses `dcmInitRepo` ONLY (unborn — no
`dcmCommitRaw`) and asserts on the message/subject, NOT `git status`. So it never exercises the
born-repo index mismatch. The new `TestDecompose_SingleShortcut_CleanStatus` mirrors
`TestDecompose_OneFileShortcut_PlannerBypassed` (decompose_test.go:1507): BORN repo (`dcmCommitRaw
"initial"`), un-staged files, then `dcmStatusPorcelain(t, repo) == ""`.

**TDD proof obligation (contract):** the new test MUST FAIL before the fix (status non-empty:
`D  a.txt`/`D  b.txt`/`?? a.txt`/`?? b.txt`-shaped) and PASS after. The PRP's Validation Loop runs the test
on the UNFIXED tree first to confirm it reproduces the bug, then after the fix to confirm green.

## §5 — No conflict with the parallel work item

P1.M2.T2.S1 (bugfix Issue 2, message-role provider) touches `internal/cmd/default_action.go`,
`internal/config/roles.go`, `internal/generate/generate.go`, `internal/stubtest/stubtest.go`,
`pkg/stagecoach/stagecoach.go`, `pkg/stagecoach/stagecoach_test.go` — **NOT** `internal/decompose/*`. This task
(P1.M3.T1.S1) touches ONLY `internal/decompose/decompose.go` + `internal/decompose/decompose_test.go`.
Zero file overlap ⇒ no merge conflict; the two are independent.

## §6 — Test-infrastructure helpers (all pre-existing, reused as-is)

- `dcmInitRepo(t, dir)` — `git init` (unborn).
- `dcmCommitRaw(t, dir, msg)` — `git commit --allow-empty -m msg` (makes the repo BORN with an empty
  initial commit; sets preRunHEAD + the dup-check baseline).
- `dcmWriteFile(t, dir, name, body)` — writes a working-tree file (UN-staged/untracked).
- `dcmStatusPorcelain(t, dir)` — `git status --porcelain` output (the §20.2 assertion target).
- `dcmPlannerManifest(t, bin, jsonOut)` — stub planner manifest returning fixed JSON.
- `dcmDeps(t, repo, roles)` — default Deps (Commits=0 auto, Single=false).
- `dcmLogCount(t, dir)` — `git rev-list --count HEAD`.
- `stubtest.Manifest(bin, Options{Script, Counter})` — counter stub (assert an agent was NOT called).
- `stubtest.Build(t)` — builds the stub binary.
