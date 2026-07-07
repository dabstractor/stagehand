# Findings — P3.M1.T1.S1 (git freeze primitives: FreezeWorkingTree + DiffTreeNames)

Source of truth: work-item contract + `plan/003_6ce49c39466e/architecture/scout_decompose_freeze.md`
(§a, §e) + the existing `internal/git/git.go` (interface + impls + `run()` invariant) + empirical
verification in a real temp repo (two `mktemp` git sessions, all edge cases confirmed).

---

## §1 — The two method contracts (verbatim from the work item + scout §a/§e)

**(a) FreezeWorkingTree — the T_start freeze capture:**
```go
FreezeWorkingTree(ctx context.Context, baseTree string) (tStart string, err error)
```
- Internally: `AddAll()` → `WriteTree()` (returns T_start) → `ReadTree(baseTree)` (resets the index
  back to the clean base so the per-concept stager starts clean).
- The caller supplies `baseTree` (HEAD^{tree} or EmptyTreeSHA) — the orchestrator already derives it
  (decompose.go:152: `baseTree = EmptyTreeSHA (unborn) or RevParseTree("HEAD")`). FreezeWorkingTree
  does NOT re-derive baseTree.
- Returns T_start — the immutable tree object recording the ENTIRE working-tree change set (every
  modified/added/deleted/untracked path + its byte content) at the instant AddAll ran (PRD §13.6.1
  FR-M1b: "the first action on activation is to freeze the entire working-tree change set into T_start").

**(b) DiffTreeNames — the changed-path-set for subset enforcement:**
```go
DiffTreeNames(ctx context.Context, treeA, treeB string) ([]string, error)
```
- Runs `git diff-tree -r --name-only --no-commit-id <treeA> <treeB>`, parses stdout into a sorted,
  deduped `[]string` of changed paths.
- Consumed by the freeze-enforcement layer (P3.M2.T1.S1, a DIFFERENT task): it computes
  `DiffTreeNames(prevTree, tree[i])` (the concept the stager just staged) and verifies every path is a
  member of `DiffTreeNames(baseTree, tStart)` (the T_start change set) — hard abort on violation (FR-M1c).
- Also reusable for FR-M9's arbiter file-lists (diff-tree of each commit) and FR-M8's empty-skip
  (`tree[i] == tree[i-1]` ⇔ `len(DiffTreeNames(tree[i-1], tree[i])) == 0`).

## §2 — The index-mutator set + why the freeze uses AddAll+WriteTree+ReadTree

From scout §a + exhaustive grep: the `git.Git` interface has **ONLY THREE index mutators**:
1. `AddAll` (git.go:840) — `git add -A`, stages all (new/mod/del).
2. `Add` (git.go:858) — `git add -- <paths>`, stages specific paths.
3. `ReadTree` (git.go:944) — `git read-tree <tree>`, **REPLACES** the index with a tree's contents.

**There is NO `ResetIndex`/`RestoreIndex`/`git reset`/`git restore` helper** (confirmed by exhaustive
grep of internal/git/). The ONLY way to "reset the index to a known tree" is `ReadTree(<tree>)`
(scout §e). This is the existing pattern in `resolveMidChain` (chain.go:201: `ReadTree(tree[j])`).

So the freeze's three-step sequence is composed ENTIRELY of existing primitives — NO new plumbing:
- `AddAll` stages the full working-tree change set (matches the existing `runSingleShortcut` pattern at
  decompose.go:229–234: `AddAll` → `WriteTree` → `treePrime`).
- `WriteTree` freezes it into T_start (the existing snapshot primitive, §13.2).
- `ReadTree(baseTree)` resets the index to the clean base (the existing chain-rebuild primitive).

This means FreezeWorkingTree is a thin ORCHESTRATION of three existing methods — its body is 3 calls +
error handling, no new git invocations, no new logic. It exists as a named primitive so the freeze
contract (AddAll→WriteTree→ReadTree, in that exact order) lives in one auditable place and the
orchestrator (P3.M1.T1.S2) can call a single method instead of inlining the 3-step dance.

## §3 — Empirically verified git behavior (two real temp-repo sessions)

**DiffTreeNames** (`git diff-tree -r --name-only --no-commit-id <treeA> <treeB>`):
- Emits each changed path on its OWN line (tree-walk order — NOT guaranteed alphabetical/sorted).
- Each path emitted EXACTLY ONCE for a single tree-pair (a path cannot be both added and removed in one
  diff; tree-walk visits each path once). So `sort.Strings()` is needed for deterministic order;
  explicit dedup is defensive (no duplicates occur, but a `seen` map or the standard
  sort-then-skip-adjacent-dupes idiom makes the "sorted, deduped" contract airtight).
- Identical trees (treeA == treeB) → EMPTY stdout → return `nil` (len 0). ✓
- EmptyTreeSHA as treeA → lists ALL files in treeB (the unborn-base case). ✓
- `--no-commit-id` is the safe explicit form: when the args are TREES (not commits) no commit SHA is
  emitted anyway, but `--no-commit-id` guarantees it even if a caller passes a commit-ish. ✓
- Exit code: `git diff-tree` exits 0 with OR without changes; exit 128 = bad/unresolvable tree SHA = a
  REAL error. → SIMPLE branch (`code != 0` → error), byte-identical to StagedDiff/TreeDiff/StatusPorcelain.

**FreezeWorkingTree** (`AddAll → WriteTree → ReadTree(baseTree)`):
- AddAll stages the full working-tree change set (modifications, additions/untracked, AND deletions).
- WriteTree returns T_start (the immutable tree of the staged change set).
- ReadTree(baseTree) REPLACES the index with baseTree's contents; **the working-tree files on disk are
  UNCHANGED** (read-tree only rewrites .git/index).
- After FreezeWorkingTree returns: `git ls-files` == baseTree's files; `git status --porcelain` shows
  the user's changes as UNSTAGED (modified/deleted/untracked relative to the reset index). This is
  CORRECT and DESIRED (see §7).
- Unborn case (baseTree == EmptyTreeSHA): AddAll stages all untracked; WriteTree makes T_start;
  `git read-tree <EmptyTreeSHA>` exits 0 and resets the index to EMPTY (ls-files empty); the untracked
  files reappear in `git status`. ✓ (verified empirically — EmptyTreeSHA is a valid read-tree target).

## §4 — run() invariant + the error-handling convention (mirror AddAll/WriteTree/ReadTree exactly)

`run()` (git.go:234) invariant: a non-zero git exit is returned as `(stdout, stderr, exitCode, nil)` —
`err` stays nil; the exit code is the signal. `err != nil` ⟺ infrastructural failure ONLY
(LookPath miss / context cancel / start-I/O), with `exitCode == -1`.

Therefore EVERY git invocation follows the two-branch pattern (copy verbatim from AddAll/ReadTree):
1. `if err != nil { return ..., err }` — FIRST, UNWRAPPED (propagates context.Canceled + git-binary-
   missing so `errors.Is` works at the call site).
2. `if code != 0 { return ..., fmt.Errorf("git <cmd>: failed (exit %d): %s", code, strings.TrimSpace(stderr)) }`.

FreezeWorkingTree chains THREE such calls (AddAll, WriteTree, ReadTree) — each with its own two branches.
On ANY step failure, return that step's error immediately. Document the partial-failure state: if
WriteTree succeeds but ReadTree fails, the index is left STAGED (holding the full change set) — the
orchestrator owns recovery; FreezeWorkingTree does not roll back (mirrors how runSingleShortcut leaves
the index staged on a mid-sequence failure).

Error-message prefixes (mirror the existing method docs):
- AddAll step failure: surfaced by AddAll's OWN error ("git add -A: failed (exit %d): %s") — FreezeWorkingTree
  just returns it; do NOT re-wrap (preserves errors.Is and the existing message text).
- WriteTree step failure: surfaced by WriteTree's OWN error (incl. the "unresolved merge conflicts"
  special-case) — return as-is.
- ReadTree step failure: surfaced by ReadTree's OWN error ("git read-tree: failed (exit %d): %s") — return as-is.

DiffTreeNames has its OWN two branches with prefix "git diff-tree: failed (exit %d): %s".

## §5 — Interface placement + the only-one-impl guarantee (verified)

- The `Git` interface block is git.go:59–210; the LAST method is `LogRange` (interface sig at git.go:207,
  impl at git.go:787). APPEND `FreezeWorkingTree` and `DiffTreeNames` at the END of the interface
  (after LogRange) AND at the END of the impl section (after LogRange's body) → minimal merge friction
  (consistent with the prior StatusPorcelain/WorkingTreeDiff/LogRange additions, each appended at the end).
- **`*gitRunner` is the ONLY implementor of `Git`** (verified by exhaustive grep: no other type defines
  RevParseHEAD/WriteTree/AddAll/StatusPorcelain; no mock/fake/stub/memory Git anywhere — decompose tests
  all use `git.New(repo)`, the real gitRunner). So adding two interface methods is PURELY ADDITIVE: no
  existing type needs updating, no existing caller or test breaks.
- Do NOT edit the `// Method ownership` comment block (if present) or any existing method.

## §6 — Imports / module / lint (verified)

- git.go ALREADY imports: bytes, context, errors, fmt, io, os/exec, sort, strconv, strings.
- FreezeWorkingTree needs: context (param) — already imported. It calls existing methods (no new import).
- DiffTreeNames needs: context, sort, strings — ALL already imported. NO new import. NO go.mod change.
- go.mod: module `github.com/dustin/stagecoach`, go 1.22.
- Lint (.golangci.yml): errcheck/gosimple/govet/ineffassign/staticcheck/unused. The new methods are USED
  (impls called via the interface; tests call them) so `unused` won't flag them. No errcheck concern
  (every error return is checked; AddAll/WriteTree/ReadTree returns are all handled).

## §7 — The freeze's index-reset semantics (the subtle gotcha — scout Open Question #1)

After FreezeWorkingTree returns, the INDEX == baseTree (reset), but the working-tree files on disk are
UNCHANGED. Consequences:
- `git status --porcelain` shows the user's changes as UNSTAGED (M/D/??) relative to the reset index.
- This is CORRECT and DESIRED: T_start is the frozen immutable record of the change set; the per-concept
  stager (tooled agent, FR-M5) then runs `git add`/`git apply --cached` against the working tree to stage
  ONE concept into the clean (base-matching) index. The stager starts from a known-clean index state.
- FreezeWorkingTree does NOT touch the working-tree files (read-tree only rewrites .git/index). Document
  this in the interface doc comment — a reader might expect the freeze to also snapshot/restore the
  working tree, but it does not (it doesn't need to: T_start + the per-concept write-tree snapshots are
  the immutable records; the working tree is the stager's mutable input).
- The defense-in-depth (FR-M1c: verify tree[i] ⊆ T_start) is a SEPARATE task (P3.M2.T1.S1) and is what
  catches a concurrent working-tree change the stager might pick up. THIS task only provides the capture
  primitive + the path-set primitive that enforcement consumes.

## §8 — Consumer contract (what P3.M1.T1.S2 + P3.M2.T1.S1 will call)

- **P3.M1.T1.S2** (wire T_start into Decompose): after deriving baseTree (decompose.go:~152) and BEFORE
  callPlanner (decompose.go:~150), calls:
    `tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)`
  then threads `tStart` through the planner/stagers/arbiter/shortcuts so every commit draws from T_start.
  This task (S1) only PROVIDES FreezeWorkingTree; S2 wires it.
- **P3.M2.T1.S1** (freeze enforcement, FR-M1c): after each staging step computes tree[i], calls:
    `stagedPaths := DiffTreeNames(ctx, prevTree, tree[i])`
    `tStartPaths := DiffTreeNames(ctx, baseTree, tStart)`
    and verifies `stagedPaths ⊆ tStartPaths` (every staged path is a T_start path) — hard abort on
  violation. THIS task (S1) only PROVIDES DiffTreeNames; the enforcement logic is S1-of-M2.

So THIS task's exported surface (the contract both consumers need): `FreezeWorkingTree` + `DiffTreeNames`
on the `Git` interface, implemented on `*gitRunner`, each with a temp-repo test.

## §9 — Test conventions (mirror readtree_test.go / addall_test.go / treediff_test.go)

1. **One test file per method**: `freezeworkingtree_test.go` + `difftreenames_test.go` (NEW files;
   package `git`, internal tests so the helpers are visible). Prefix: `TestFreezeWorkingTree_*` /
   `TestDiffTreeNames_*`.
2. **Package-level helpers (REUSE — do NOT redefine; they live across _test.go files)**:
   - `initRepo(t, dir)` (git_test.go:12) — git init + repo-local identity.
   - `writeFile(t, dir, name, body)` (committree_test.go:31) — write a file.
   - `stageFile(t, dir, name)` (committree_test.go:39) — git add a file.
   - `writeTreeOf(t, dir)` (committree_test.go:48) — git write-tree → SHA (independent oracle).
   - `makeEmptyCommit(t, dir, msg)` (revparse_test.go:24) — git commit --allow-empty (clean-tree base).
   - `execGit(t, dir, args...)` (revparsetree_test.go:115) — run git, return trimmed stdout (oracle).
3. **Oracle pattern**: verify state with INDEPENDENT `execGit`/`exec.Command("git",...)` calls, NOT the
   method under test (e.g. verify T_start via `execGit(t, repo, "cat-file", "-t", tStart)` == "tree";
   verify DiffTreeNames vs an independent `git diff-tree` run).
4. **Standard test matrix per method** (mirror readtree_test.go): happy path + the load/replaces
   semantic + BadTreeSHA + NotARepo + GitBinaryMissing (`t.Setenv("PATH","")`) + ContextCancelled
   (pre-cancelled ctx, `errors.Is(err, context.Canceled)`).
5. **FreezeWorkingTree-specific tests**: (a) T_start captures the FULL change set (verify via
   DiffTreeNames(baseTree, tStart) OR an independent diff-tree lists all changed paths); (b) index is
   reset to baseTree after the call (verify `execGit ls-files` == baseTree's files); (c) working-tree
   files unchanged (verify content on disk); (d) unborn case (baseTree=EmptyTreeSHA → T_start holds all
   untracked, index reset to empty); (e) a SECOND FreezeWorkingTree on an already-clean tree returns a
   T_start == baseTree (no changes) — idempotent-ish.
6. **DiffTreeNames-specific tests**: (a) lists all changed paths between two trees; (b) sorted+deduped
   output; (c) identical trees → nil/empty; (d) EmptyTreeSHA as treeA → all files; (e) deletions +
   additions + modifications all listed; (f) BadTreeSHA → error; (g) NotARepo → error; (h)
   GitBinaryMissing → "git binary not found"; (i) ContextCancelled → errors.Is(context.Canceled).

## §10 — Scope boundaries (frozen / owned elsewhere — do NOT touch)

- `internal/git/git.go` — MODIFY: append 2 interface methods (after LogRange) + 2 `*gitRunner` impls
  (after LogRange's body). NO other change to git.go. Do NOT touch run()/AddAll/WriteTree/ReadTree/
  TreeDiff/StatusPorcelain/WorkingTreeDiff/LogRange/EmptyTreeSHA — all CONSUMED as-is.
- `internal/decompose/*` — UNCHANGED (this task does NOT wire FreezeWorkingTree into the orchestrator;
  that is P3.M1.T1.S2; it does NOT add freeze enforcement; that is P3.M2.T1.S1).
- go.mod / go.sum — UNCHANGED (stdlib only; all imports already present).
- Parallel work P2.M1.T1.S2 (qwen-code tier row + model-token refresh + docs) does NOT touch git.go
  (confirmed: 0 MODIFY/CREATE internal/git references) → NO merge conflict. Append at END to be safe.
- No mocks/fakes to update (only *gitRunner implements Git — §5).
