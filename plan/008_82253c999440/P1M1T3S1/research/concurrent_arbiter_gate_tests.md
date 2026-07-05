# Concurrent-Across-Arbiter-Gate + T_start Completeness Tests — P1.M1.T3.S1 Research

> Empirically verified against the live repo (2026-07-05). The freeze-safe arbiter (P1.M1.T2.S1) is
> **LANDED on disk**: the gate uses `DiffTreeNames(tipTree, tStart)` (decompose.go:223), `resolveArbiter`
> is 7-arg (chain.go:50), paths A/B set `treePrime := tStart`, path C uses `OverlayTreePaths` per-j, and
> each path syncs via `ReadTree(tStart)`. These tests are the PERMANENT REGRESSION NET for FR-M1d — they
> must PASS against the landed code. (The contract's "TDD: fail before / pass after" note is historical:
> the fix is already in; these encode the post-fix invariant so a future regression is caught.)

## 1. Scope and boundaries

| Leg | Source | This task (S1)? |
|---|---|---|
| (a) Upgrade `TestDecompose_ConcurrentChangeExclusion`: sentinel in NO commit (incl. arbiter); sentinel remains untracked | contract 3a / arch §5.1 (a)(b) | ✅ |
| (b) Empty-frozen-leftover case: stagers cover ALL of T_start → `DiffTreeNames(tipTree, tStart)==[]` → exactly 2 commits (arbiter skipped) + sentinel unstaged | contract 3b / arch §5.1 (c) | ✅ (merges into the (a) upgrade — same fixture) |
| (c) T_start completeness: `DiffTreeNames(baseTree, HEAD^{tree}) ⊇ DiffTreeNames(baseTree, tStart)`; live status non-empty ONLY from paths outside T_start | contract 3c / arch §5.3 / PRD §20.2 | ✅ (new dedicated test) |
| §5.2 Arbiter folds ONLY T_start content (legitimate leftover + arbiter RUNS + folds) + chain_test.go unit proof | arch §5.2 | ❌ **P1.M1.T3.S2** (the sibling — do NOT duplicate) |

**File touched: ONLY `internal/decompose/decompose_test.go`.** No production code (chain.go/decompose.go/
arbiter.go — S1's territory, landed), no chain_test.go (S1 + S2's territory), no git/*, no docs/*
(P1.M1.T2.S2 — the Mode A arbiter-narrative edit, in flight), no PRD/tasks/snapshot.

## 2. The upgrade target — `TestDecompose_ConcurrentChangeExclusion` (decompose_test.go:819-882)

The existing test's fixture IS the empty-frozen-leftover case:
- Unborn repo; two dirty files `a.txt` + `b.txt` written BEFORE the run ⇒ both in `T_start`.
- Planner returns 2 concepts (`add a`, `add b`).
- `concurrentSentinelSeam` (decompose_test.go:795) stages `a.txt` for concept 0 / `b.txt` for concept 1
  AND writes `sentinel.txt` UNSTAGED on the first concept (post-freeze ⇒ not in `T_start`).
- Both `T_start` paths are staged+committed ⇒ `tipTree == T_start` ⇒ `DiffTreeNames(tipTree, tStart) == []`.

**Pre-fix behavior (the loophole):** the live `StatusPorcelain` gate saw `?? sentinel.txt` → arbiter
RAN → `resolveNewCommit`'s `AddAll` swept the sentinel into a 3rd commit. The test's comments at L838
and L880-882 explicitly ADMIT this ("the arbiter's STAGING picks up sentinel.txt from the working tree…
we verify only the loop commits"). The message script has 4 entries (2 loop + 2 for the arbiter's
`resolveNewCommit`); the arbiter manifest returns `{"target": null}`.

**Post-fix behavior (LANDED):** the frozen gate `DiffTreeNames(tipTree, tStart) == []` → arbiter does
NOT run → exactly 2 commits → sentinel never enters any commit → sentinel stays untracked. The current
test STILL PASSES (it only asserts on the loop commits) but is now testing a weaker property than the
code delivers.

**The upgrade (contract 3a + 3b merged — same fixture):**
- KEEP the existing "sentinel in no LOOP commit" diff-tree check.
- ADD (3a): sentinel in NO commit across the WHOLE run — `git log --name-only --format=` does not
  contain `sentinel.txt` (reuses the exact oracle from `TestDecompose_StagerFreezeViolation`:781).
- ADD (3a): sentinel REMAINS untracked — `dcmStatusPorcelain` contains `?? sentinel.txt`.
- ADD (3b): exactly 2 commits — `dcmLogCount(t, repo) == 2` (arbiter skipped: empty frozen leftover).
- TRIM the message script from 4 entries to 2 (loop only; the arbiter no longer calls for a message).
- REMOVE the stale loophole-admitting comments (L838, L880-882); rewrite the doc comment to state this
  is the post-FR-M1d regression net (arbiter frozen; the sentinel cannot cross the gate).

`DecomposeResult.Amended` is 0 both when the arbiter did not run AND when it made a new (null) commit
(decompose.go:74) — so `Amended` alone does NOT prove "arbiter skipped." `dcmLogCount == 2` is the
authoritative proof (a null-target arbiter would create a 3rd commit). Use `dcmLogCount`.

## 3. The new test — `TestDecompose_TStartCompleteness` (contract 3c / PRD §20.2)

PRD §20.2 renamed "Loop index cleanliness" → "`T_start` completeness": after a fully-successful run,
every `T_start` path landed in HEAD; live `git status --porcelain` may be non-empty ONLY from paths
OUTSIDE `T_start` (the post-freeze sentinel), which are intentionally left unstaged (FR-M1d).

**Note on the existing line-395 reference:** `decompose_test.go:395` is an INLINE assertion (`status == ""`
after a single-shortcut run) inside a shortcut test, NOT a standalone "Loop index cleanliness" test. The
§20.2 rename is narrative; that inline assertion is a valid shortcut-path check and is NOT deleted. This
task adds the DECOMPOSE-LOOP completeness proof (broader: every frozen path landed + status clean except
out-of-T_start paths), which that inline assertion does not cover.

**Fixture (born repo so `baseTree` is a real tree — makes the superset check meaningful):**
- `dcmInitRepo` + `dcmCommitRaw("initial")` ⇒ born; `baseTree = HEAD^{tree}` (capture before the run).
- Two dirty files `x.go` + `y.go` written before the run ⇒ `T_start` path-set = `{x.go, y.go}`.
- Planner returns 2 concepts; `dcmStagerSeam` (or `concurrentSentinelSeam`) stages x.go / y.go ⇒ both land.
- `concurrentSentinelSeam` writes `sentinel.txt` unstaged on concept 0 (post-freeze, outside `T_start`).

**Assertions:**
- (completeness) every frozen path landed: for each `p ∈ {x.go, y.go}`, `p` ∈ `DiffTreeNames(baseTree, headTree)`
  via `git.New(repo).DiffTreeNames(ctx, baseTree, headTree)` where `headTree = HEAD^{tree}` post-run.
  (Equivalent to §20.2's `DiffTreeNames(baseTree, HEAD^{tree}) ⊇ DiffTreeNames(baseTree, tStart)`; the
  test knows the frozen set = the files it wrote, so it asserts that set is ⊆ the landed set.)
- (status discipline) `dcmStatusPorcelain` shows ONLY `?? sentinel.txt` — no `T_start` path is left
  dirty/uncommitted (x.go, y.go are clean); the sole non-empty entry is the out-of-`T_start` sentinel.
- (cross-check) sentinel in no commit (`git log --name-only --format=`); exactly 2 commits (`dcmLogCount == 2`
  on top of the seed → `dcmLogCount == 3` total counting "initial"; OR assert `len(result.Commits) == 2`).

Using `git.New(repo).DiffTreeNames` in the test is elegant — it's the SAME primitive the production gate
uses (decompose.go:223), so the test directly pins the invariant the gate relies on. The `git` package is
already imported in decompose_test.go (L18) and `git.New(repo)` is used at L142/153/232.

## 4. Boundary with P1.M1.T2.S2 (the parallel-ish sibling, §5.2)

S2 is the **legitimate-leftover** case: a concept's stager is a no-op so a REAL frozen leftover exists
(`DiffTreeNames(tipTree, tStart) != []`) → the arbiter RUNS → its commit's tree is EXACTLY `T_start`
(null/tip) or an `OverlayTreePaths(tree[j], T_start, leftoverPaths)` overlay (mid-chain). S2 also adds
the chain_test.go unit-level proof (calling `resolveArbiter` directly with a post-freeze sentinel).

**This S1 is the COMPLEMENTARY empty-leftover case** (arbiter SKIPPED) + the exclusion/completeness
invariants. S1 must NOT add any legitimate-leftover / arbiter-runs scenario — that is S2's exclusive
territory. The fence is clean: S1 = `DiffTreeNames(tipTree, tStart) == []` (arbiter skipped); S2 =
`DiffTreeNames(tipTree, tStart) != []` (arbiter runs, folds only `T_start`).

## 5. Test infrastructure (all reusable — no new helpers needed)

| Helper | Location | Use |
|---|---|---|
| `dcmInitRepo/WriteFile/StageFile/CommitRaw/RunGit/GitOut/HeadSHA/LogOneline/LogCount/StatusPorcelain` | decompose_test.go:28-103 | repo setup + git oracles |
| `dcmPlannerManifest/MessageScriptManifest/ArbiterManifest` | :109-127 | stub agents (stubtest.Manifest/NewScript) |
| `dcmDeps/dcmAllRoles/dcmStagerSeam` | :139-175/189 | Decompose() wiring |
| `concurrentSentinelSeam` | :795 | the post-freeze unstaged-sentinel stager (the crux of both tests) |
| `tooledStubManifest`, `stubtest.Build/Manifest/NewScript` | stubtest package | stager manifest + binary |
| `git.New(repo).DiffTreeNames(ctx, a, b)` | internal/git | the completeness superset check |

`concurrentSentinelSeam(t, repo, conceptFiles map[string][]string, sentinel string)` stages each
concept's files AND, on the FIRST concept only, writes `sentinel` UNSTAGED via `os.WriteFile` (no
`git add`). The sentinel is post-freeze (the seam runs during the loop, after `FreezeWorkingTree`)
⇒ excluded from `T_start`. This is exactly the FR-M1b/M1c/M1d concurrent-change seam.

## 6. Decisions log

- **D1** — Merge contract (a)+(b) into the ONE upgraded `TestDecompose_ConcurrentChangeExclusion`. The
  existing fixture IS the empty-frozen-leftover case (stagers cover all of `T_start`), so (b)'s "exactly
  2 commits" assertion is a natural strengthening of (a)'s upgrade. A separate redundant (b) test would
  duplicate the fixture. (arch §5.1 treats them as one upgrade.)
- **D2** — `dcmLogCount == 2` (not `result.Amended == 0`) is the "arbiter skipped" proof. `Amended` is 0
  both for "arbiter did not run" AND "arbiter ran + null target" (decompose.go:74); only the commit count
  distinguishes them. A null-target arbiter would create a 3rd commit.
- **D3** — Trim the message script 4→2 entries in the upgraded test. Post-fix the arbiter does not run,
  so entries 2/3 are never consumed. Trimming makes the test's intent ("arbiter won't call for a
  message") explicit. Keep a stub Arbiter manifest (the role is required) — it is never invoked.
- **D4** — `TestDecompose_TStartCompleteness` is a NEW dedicated test on a BORN repo (real `baseTree`),
  distinct from the upgraded exclusion test (unborn repo). The born-repo variant makes the
  `DiffTreeNames(baseTree, headTree)` superset check meaningful and maps cleanly to PRD §20.2's
  "T_start completeness" invariant. Do NOT delete the line-395 inline shortcut assertion (it is a
  valid shortcut-path check; the §20.2 rename is narrative).
- **D5** — Use `git.New(repo).DiffTreeNames` for the completeness check (same primitive the production
  gate uses). It is already imported/used in decompose_test.go. This pins the invariant the gate relies
  on with the identical oracle.
- **D6** — Do NOT add any legitimate-leftover / arbiter-runs scenario — that is S2's (§5.2) exclusive
  territory. S1 is empty-leftover (arbiter skipped) + exclusion + completeness only.
- **D7** — The tests PASS against the landed freeze-safe arbiter (the fix is in). They are the permanent
  regression net. Do NOT attempt to demonstrate a "before" failure (the pre-fix code is gone).
