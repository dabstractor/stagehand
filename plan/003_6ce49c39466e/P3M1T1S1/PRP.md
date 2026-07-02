---
name: "P3.M1.T1.S1 — git.Git freeze primitives: FreezeWorkingTree (capture T_start) + DiffTreeNames (subset path-set) (PRD §13.6.1 FR-M1b, FR-M1c)"
description: |

  ADD TWO NEW METHODS to the `Git` interface (`internal/git/git.go`) AND implement each on
  `*gitRunner`, each with a temp-repo test. These are the plumbing primitives for the v2 decompose
  **start-of-run working-tree freeze** (PRD §13.6.1 FR-M1b) and its **freeze enforcement**
  (FR-M1c). Both are composed ENTIRELY of existing git primitives + one new `git diff-tree` invocation;
  no new types, no new options struct, no new logic concept.

  CONTRACT (P3.M1.T1.S1, verbatim from the work item):
    1. RESEARCH NOTE: scout_decompose_freeze.md §(a)/(e). The git.Git interface (git.go:59–210) has
       AddAll (git.go:840), WriteTree (git.go:337), ReadTree (git.go:944 — REPLACES the index with a
       tree's contents), EmptyTreeSHA (git.go:500). There is NO reset/restore-index helper; the index
       has ONLY three mutators (AddAll, Add, ReadTree). Existing test pattern: internal/git tests
       create a temp repo with real git.
    2. INPUT: the git.Git interface + *gitRunner impl (internal/git/git.go).
    3. LOGIC: Add TWO new git.Git methods, each with a matching *gitRunner impl + a temp-repo test:
       (a) a freeze capture — FreezeWorkingTree(ctx, baseTree string) (tStart string, err error):
           internally AddAll → WriteTree (T_start) → ReadTree(baseTree) to reset the index back to the
           clean base so the per-concept stager starts clean. Handle the unborn case (baseTree =
           EmptyTreeSHA). The caller supplies baseTree (HEAD^{tree} or EmptyTreeSHA) since the
           orchestrator already derives it.
       (b) a changed-path-set for subset enforcement — DiffTreeNames(ctx, treeA, treeB string)
           ([]string, error) via `git diff-tree -r --name-only --no-commit-id <treeA> <treeB>`
           (sorted, deduped). Add BOTH to the interface declaration AND the impl. Keep them shell-free
           ([]string args, never sh -c — §19).
    4. OUTPUT: two new git primitives enabling T_start capture + path/content subset verification, each
       tested against a real temp repo.
    5. DOCS: [Mode A] git.Git interface doc comments for the new methods (freeze semantics: T_start is
       the immutable record of the working-tree change set at run start).

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `run()` / `runWithInput()` in git.go — CONSUMED, not modified.
    - `AddAll` / `WriteTree` / `ReadTree` / `Add` / `EmptyTreeSHA` / `TreeDiff` / `StatusPorcelain` /
      `WorkingTreeDiff` / `LogRange` — CONSUMED as-is. FreezeWorkingTree ORCHESTRATES AddAll+WriteTree+
      ReadTree; it does NOT duplicate them.
    - `internal/decompose/*` — UNCHANGED. Wiring FreezeWorkingTree into the orchestrator is P3.M1.T1.S2;
      freeze enforcement (the subset check) is P3.M2.T1.S1. THIS task only adds + tests the primitives.
    - go.mod / go.sum — UNCHANGED (stdlib only: context/fmt/sort/strings all already imported in git.go).
    - Parallel work P2.M1.T1.S2 (qwen-code) does NOT touch git.go → no merge conflict. Append at END.

  DELIVERABLES (1 file MODIFIED, 2 new files):
    MODIFY internal/git/git.go — (a) append `FreezeWorkingTree` + `DiffTreeNames` to the `Git`
      INTERFACE (after LogRange, the current last method at git.go:207) with doc comments; (b) append the
      two `(*gitRunner)` method bodies at the END of the file (after LogRange's impl).
    CREATE internal/git/freezeworkingtree_test.go — ~6 tests (full-change-set capture, index-reset-to-
      base, working-tree-unchanged, unborn EmptyTreeSHA case, no-change idempotent, error cases).
    CREATE internal/git/difftreenames_test.go — ~8 tests (changed-path listing, sorted+deduped,
      identical→empty, EmptyTreeSHA base, del+add+mod, BadTreeSHA, NotARepo, GitBinaryMissing,
      ContextCancelled).

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass (the two new
  interface methods are additive — *gitRunner is the only Git implementor, verified; no mock exists);
  FreezeWorkingTree returns a T_start whose `git diff-tree` vs baseTree lists EXACTLY the working-tree
  change set and leaves the index == baseTree; DiffTreeNames returns the sorted, deduped path set and
  `nil` for identical trees.

---

## Goal

**Feature Goal**: Implement the two git-plumbing primitives that underpin the v2 decompose
**start-of-run working-tree freeze** (PRD §13.6.1 FR-M1b) and its **defense-in-depth enforcement**
(FR-M1c) at the `Git` interface boundary. `FreezeWorkingTree` captures `T_start` — the immutable tree
object recording the entire working-tree change set (every modified/added/deleted/untracked path + its
byte content) at run start — by orchestrating the three existing index primitives `AddAll → WriteTree →
ReadTree(baseTree)`; it leaves the index reset to the clean base so the per-concept stager starts clean.
`DiffTreeNames` returns the sorted, deduped changed-path set between two trees via
`git diff-tree -r --name-only --no-commit-id`, the primitive the freeze-enforcement layer uses to
verify `tree[i]` is a content-subset of `T_start`.

**Deliverable** (1 file MODIFIED, 2 new):
1. `internal/git/git.go` — two new methods appended to the `Git` interface (after `LogRange`) with doc
   comments naming: FreezeWorkingTree's 3-step orchestration (AddAll→WriteTree→ReadTree) + its freeze
   semantics (T_start is the immutable record of the working-tree change set at run start) + its
   index-reset contract (index == baseTree after return; working-tree files UNCHANGED) + the unborn
   case (baseTree=EmptyTreeSHA); and DiffTreeNames's command (`git diff-tree -r --name-only
   --no-commit-id`), its sorted+deduped output contract, its `nil`-for-identical-trees contract, and
   its exit-code convention (0 with/without changes; 128 = bad tree = real error). Plus the two
   `(*gitRunner)` method bodies.
2. `internal/git/freezeworkingtree_test.go` — ~6 temp-repo tests.
3. `internal/git/difftreenames_test.go` — ~8 temp-repo tests.

**Success Definition**:
- On a born repo with a modified file `a.txt`, a new file `c.txt`, and a deleted `b.txt` (all un-staged),
  `FreezeWorkingTree(ctx, baseTree)` returns a non-empty SHA `tStart` whose `git cat-file -t` is "tree",
  and `git diff-tree baseTree tStart` lists EXACTLY `{a.txt, b.txt, c.txt}`; AND after the call the index
  matches baseTree (`git ls-files` == baseTree's files) while the working-tree files are unchanged.
- On an unborn repo (zero commits), `FreezeWorkingTree(ctx, EmptyTreeSHA)` returns a `tStart` holding
  all untracked files and leaves `git ls-files` EMPTY.
- `DiffTreeNames(ctx, treeA, treeB)` returns the sorted, deduped path set; for identical trees returns
  `nil` (len 0); for `EmptyTreeSHA` as treeA lists all of treeB's files.
- On a plain non-repo directory, both methods return a non-nil error (git exit 128/129 = real error).
- With `PATH=""`, both return a non-nil error containing `"git binary not found"`.
- With a pre-cancelled context, both return a non-nil error with `errors.Is(err, context.Canceled)`.
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 1 modified (`internal/git/
  git.go`) + 2 new untracked test files — nothing else.

## User Persona

**Target User**: the decompose orchestrator (internal code, P3.M1.T1.S2) and the freeze-enforcement
layer (P3.M2.T1.S1), and by extension the end user running `stagehand` on an un-staged working tree to
get multiple commits while another tool (an editor save, a concurrent coding agent) may also be writing
files. Neither method is a user-facing CLI flag; they are plumbing primitives.

**Use Case**: when decomposition activates (nothing staged, working tree dirty — FR-M1/§13.6.1), the
orchestrator FIRST calls `FreezeWorkingTree(ctx, baseTree)` to capture `T_start` — the immutable record
of the working-tree change set at run start (FR-M1b). The planner, every stager, the arbiter, and the
shortcuts then draw strictly from `T_start`, so a file a concurrent process writes DURING the (potentially
long) run is excluded from every commit. After each staging step the enforcement layer (P3.M2.T1.S1)
calls `DiffTreeNames(prevTree, tree[i])` and verifies it's a subset of `DiffTreeNames(baseTree, tStart)`
(FR-M1c) — a stager that swept in a concurrent change (or ran a bare `git add -A` against the live tree)
is a hard abort.

**Pain Points Addressed**: v2 decompose runs can take tens of seconds (multiple agent round-trips).
Without a start-of-run freeze, a file saved by the user's editor mid-run could be swept into a commit
the user never intended. The freeze makes the run commit EXACTLY the working-tree state as it existed
when the run began (FR-M1b), and the subset check makes that guarantee enforceable even though the
stager is an untrusted external agent running git against the live tree (FR-M1c). These two primitives
are the plumbing foundation for both guarantees.

## Why

- **Closes PRD §13.6.1 FR-M1b (the freeze) + FR-M1c (the enforcement) at the plumbing layer.** FR-M1b:
  "the instant decomposition activates, stagehand captures an immutable snapshot of the entire
  working-tree change set … as a tree object T_start." FR-M1c: "after each staging step, stagehand
  verifies the resulting tree is a subset of T_start — only paths present in T_start, with T_start's
  content." This task is the literal interface-level implementation of the capture primitive + the
  path-set primitive the enforcement consumes.
- **Zero new logic — pure orchestration of existing primitives + one new git invocation.** FreezeWorkingTree
  is `AddAll + WriteTree + ReadTree(baseTree)` — three EXISTING methods, called in that exact order. This
  mirrors the existing `runSingleShortcut` pattern (decompose.go:229–234: `AddAll → WriteTree → treePrime`)
  and the existing `resolveMidChain` pattern (chain.go:201: `ReadTree(tree[j])` for index reset). The
  ONLY new git command in the whole task is DiffTreeNames's `git diff-tree -r --name-only --no-commit-id`.
  No new types, no new options struct, no new shell surface, no new parsing beyond split/trim/sort/dedupe.
- **Lowest-risk, maximal-reuse, backward-compatible.** The interface GAINS two methods (Go interfaces
  are open for extension — `*gitRunner` is the ONLY `Git` implementor, verified; no mock exists, so no
  existing type or test breaks). No existing file's behavior changes. go.mod/go.sum untouched. Unblocks
  P3.M1.T1.S2 (T_start wiring) and P3.M2.T1.S1 (freeze enforcement).

## What

Two new methods on the `Git` interface (`internal/git/git.go`), each implemented on `*gitRunner`.
`FreezeWorkingTree` orchestrates three existing methods (AddAll, WriteTree, ReadTree) — it issues NO new
git command of its own. `DiffTreeNames` issues one new git command (`git diff-tree -r --name-only
--no-commit-id`) via the existing `run()` helper, then parses stdout into a sorted, deduped `[]string`.
No new types. No new options struct. No new dependencies. No caller wiring (that is P3.M1.T1.S2 /
P3.M2.T1.S1). The structural edits are: append two interface methods + doc comments, append two
`(*gitRunner)` method bodies, and add the two test files.

### Success Criteria

- [ ] `FreezeWorkingTree(ctx context.Context, baseTree string) (tStart string, err error)` is declared
      on the `Git` interface with a doc comment naming: the 3-step orchestration (AddAll→WriteTree→
      ReadTree); the freeze semantics (T_start = immutable record of the working-tree change set at run
      start, FR-M1b); the index-reset contract (index == baseTree after return; working-tree files
      UNCHANGED); the caller-supplies-baseTree contract (HEAD^{tree} or EmptyTreeSHA); the unborn case
      (baseTree=EmptyTreeSHA → index reset to empty); and the mutation/read-only convention (mutates the
      index transitively; touches NO ref).
- [ ] `(*gitRunner).FreezeWorkingTree` body is exactly: `AddAll(ctx)` → `WriteTree(ctx)` (capture
      tStart) → `ReadTree(ctx, baseTree)` (reset index) — each call following the two-branch error
      convention (err-first UNWRAPPED, then code-check), returning the existing method's error AS-IS on
      any step failure.
- [ ] `DiffTreeNames(ctx context.Context, treeA, treeB string) ([]string, error)` is declared on the
      interface with a doc comment naming: the command (`git diff-tree -r --name-only --no-commit-id`);
      the sorted+deduped output contract; the `nil`-for-identical-trees contract; the EmptyTreeSHA-as-
      treeA unborn-base contract; the exit-code convention (0 with/without changes; 128 = bad tree =
      real error; simple branch — NO --quiet, NO 128-special-case); and the read-only contract.
- [ ] `(*gitRunner).DiffTreeNames` body runs `git diff-tree -r --name-only --no-commit-id treeA treeB`,
      splits stdout on "\n", drops empty lines, `sort.Strings()`, dedupes adjacent equal entries, returns
      the slice (`nil` if empty).
- [ ] `freezeworkingtree_test.go` + `difftreenames_test.go` prove every success-definition bullet above
      against a REAL temp repo, plus the error matrix (BadTreeSHA, NotARepo, GitBinaryMissing,
      ContextCancelled).
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 1 modified + 2 new files.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the two contracts
(findings §1 — exact signatures + the 3-step orchestration + the diff-tree command, confirmed by the
work item + scout §a/§e); the empirical git behavior (findings §3 — diff-tree output is line-per-path
unsorted, identical→empty, EmptyTreeSHA works; freeze leaves index==baseTree + working tree unchanged;
unborn EmptyTreeSHA read-tree resets to empty, all verified in two temp repos); the exact methods to
call (findings §2 — AddAll/WriteTree/ReadTree are the ONLY index mutators; FreezeWorkingTree is a thin
orchestration); the error convention (findings §4 — run()'s invariant + the two-branch pattern + return
existing errors AS-IS); the placement + only-one-impl guarantee (findings §5 — append after LogRange;
*gitRunner is the sole implementor, no mocks); the index-reset semantics gotcha (findings §7 — working
tree unchanged, that's desired); the test conventions (findings §9 — one file per method, REUSE the
package helpers, oracle pattern); and the scope boundaries (findings §10 — git.go MODIFY only by
append; decompose untouched; parallel qwen-code work doesn't conflict). No decompose/orchestrator
knowledge required — the contracts are fully self-contained at the git-plumbing layer.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (contracts + verified behaviors + the gotcha)
- docfile: plan/003_6ce49c39466e/P3M1T1S1/research/findings.md
  why: §1 the TWO CONTRACTS (signatures + the 3-step orchestration + the diff-tree command); §2 why the
       freeze uses AddAll+WriteTree+ReadTree (the ONLY 3 index mutators; NO reset helper exists —
       ReadTree IS the reset); §3 the EMPIRICALLY VERIFIED git behavior (diff-tree: line-per-path
       unsorted, identical→empty, EmptyTreeSHA works, exit 0/128; freeze: index==baseTree after return,
       working tree UNCHANGED, unborn EmptyTreeSHA→empty index); §4 run()'s invariant + the two-branch
       error convention + return-existing-errors-as-is; §5 placement (append after LogRange) + the
       only-one-impl guarantee (no mocks); §7 the index-reset semantics gotcha; §9 test conventions;
       §10 scope boundaries.
  critical: §3 (the verified behaviors — these ARE the implementation contract); §2 (FreezeWorkingTree
            orchestrates EXISTING methods, it does NOT issue new git commands); §4 (return AddAll/
            WriteTree/ReadTree errors AS-IS — do NOT re-wrap); §7 (working tree unchanged after the
            freeze — document it, do NOT try to also snapshot/restore the working tree).

# MUST READ — the scout research note (the authoritative anchor map for the freeze feature)
- docfile: plan/003_6ce49c39466e/architecture/scout_decompose_freeze.md
  section: §(a) the git.Git interface method table (signatures + file:line anchors — confirms AddAll at
           git.go:840, WriteTree at git.go:337, ReadTree at git.go:944, EmptyTreeSHA at git.go:500, and
           the "ONLY three index mutators" fact); §(e) EmptyTreeSHA EXISTS + NO reset helper exists +
           "to reset index to T_start use ReadTree(T_start)".
  why: confirms the freeze is AddAll+WriteTree+ReadTree (§b: "the capture sequence is AddAll → WriteTree
       → returns T_start SHA … the index must be restored via ReadTree(T_start)") and that these are the
       exact primitives to reuse. Also documents the consumer insertion points (decompose.go:148–166).
  critical: §(e) — there is NO ResetIndex; ReadTree IS the reset primitive. Do NOT add a `git reset`/
            `git restore` method (scope creep; the contract mandates ReadTree).

# MUST READ — the FILE TO MODIFY: the Git interface + the three methods FreezeWorkingTree orchestrates
- file: internal/git/git.go
  section: the `Git` interface (append FreezeWorkingTree + DiffTreeNames at the END, after `LogRange`,
           currently the last method — interface sig at git.go:207, impl at git.go:787); `(*gitRunner).
           AddAll` at git.go:840 (the AddAll step — error returns "git add -A: failed (exit %d): %s");
           `(*gitRunner).WriteTree` at git.go:337 (the WriteTree step — returns the tStart SHA, or
           "unresolved merge conflicts" / "git write-tree failed"); `(*gitRunner).ReadTree` at git.go:944
           (the ReadTree step — REPLACES the index, error "git read-tree: failed (exit %d): %s");
           `EmptyTreeSHA` const at git.go:500; `(*gitRunner).run` at git.go:234 (the helper DiffTreeNames
           calls — its INVARIANT: non-zero git exit → (stdout, stderr, exitCode, nil), err nil).
  why: FreezeWorkingTree's body is literally `g.AddAll(ctx); tStart, err := g.WriteTree(ctx);
       g.ReadTree(ctx, baseTree)` — three calls to the existing methods, each error-checked. DiffTreeNames
       is a port of the read-only-query pattern (run() → split → parse). ReadTree's doc comment is the
       template for the index-mutation doc-comment style (the "MUTATES THE INDEX … touches NO ref" clause).
  pattern: FreezeWorkingTree: copy AddAll/WriteTree/ReadTree's two-branch error handling VERBATIM into a
           3-call sequence; return each method's error AS-IS (no re-wrap). DiffTreeNames: copy
           StagedFileCount's query+parse body (run() → split on "\n" → trim → drop empty), then add
           sort.Strings() + adjacent-dedupe; use the simple exit-code branch (code != 0 → error).
  gotcha: do NOT re-wrap AddAll/WriteTree/ReadTree errors (preserves errors.Is + their existing message
          text, incl. WriteTree's "unresolved merge conflicts" special-case). do NOT add a 128-special-
          case to DiffTreeNames (it's the simple-branch convention, like StagedDiff/TreeDiff). do NOT
          edit any existing method.

# MUST READ — the test exemplars (the patterns to mirror)
- file: internal/git/readtree_test.go   (READ — the CLOSEST template: an index-MUTATING method tested
  against a real temp repo, with the independent-oracle pattern: it verifies `git ls-files` via a
  separate exec.Command BEFORE/AFTER the call, not via the method under test.)
  why: FreezeWorkingTree mutates the index (transitively, via AddAll+ReadTree) just like ReadTree; its
       tests mirror readtree_test.go's oracle pattern (verify `git ls-files` == baseTree's files after the
       call, via an independent `execGit(t, repo, "ls-files")` / exec.Command). Also defines the standard
       error matrix (BadTreeSHA, NotARepo, GitBinaryMissing with t.Setenv("PATH",""), ContextCancelled
       with errors.Is(err, context.Canceled)) — copy these error-case tests verbatim for BOTH methods.
- file: internal/git/stagedcount_test.go   (READ — the CLOSEST query+parse template for DiffTreeNames)
  why: StagedFileCount runs a git query, splits stdout on "\n", counts non-empty lines — DiffTreeNames
       does the SAME split/trim/drop-empty then sort+dedupe. Mirror its two-branch error handling and
       its NotARepo/GitBinaryMissing/ContextCancelled tests.
- file: internal/git/treediff_test.go   (READ — the two-tree-diff setup idiom)
  why: TreeDiff takes two tree SHAs and diffs them — the SAME shape DiffTreeNames has. Its tests show
       how to build two distinct trees in a temp repo (writeFile+stageFile+makeEmptyCommit to build a
       base, then modify to build a second tree) and how to obtain tree SHAs via writeTreeOf/execGit.
- file: internal/git/committree_test.go   (READ — fixture helpers writeFile/stageFile/writeTreeOf)
  why: defines the package-level helpers `writeFile`, `stageFile`, `writeTreeOf` — ALL reusable.
  gotcha: these helpers are ALREADY defined — do NOT redefine them (duplicate-symbol compile error).
- file: internal/git/revparsetree_test.go   (READ — execGit helper + RevParseTree pattern)
  why: defines `execGit(t, dir, args...)` (runs git, returns trimmed stdout) — the independent ORACLE for
       verifying FreezeWorkingTree's index state and DiffTreeNames's output. Also shows the
       EmptyTreeSHA unborn-base idiom.
- file: internal/git/revparse_test.go   (READ — makeEmptyCommit helper)
  why: defines `makeEmptyCommit(t, dir, msg)` — usable for establishing a clean committed base.
- file: internal/git/git_test.go   (READ — initRepo helper)
  why: defines `initRepo(t, dir)` — EVERY temp-repo test starts here (git init + repo-local identity).

# MUST READ — the design references (role + freeze semantics)
- docfile: plan/003_6ce49c39466e/architecture/system_context.md
  why: confirms the v2 decompose pipeline + the freeze feature; lists the git.Git interface as the seam.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §13.6.1 (FR-M1b — the start-of-run freeze; FR-M1c — freeze enforcement) + §9.14 (FR-M1b/c)
  why: FR-M1b mandates "the first action on activation is to freeze the entire working-tree change set
       into T_start"; FR-M1c mandates "after each staging step, stagehand verifies the resulting tree is
       a subset of T_start." These two methods are the plumbing for both.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go                       # MODIFY: append FreezeWorkingTree + DiffTreeNames to the Git interface
                               #   (after LogRange at git.go:207) AND implement on *gitRunner (after
                               #   LogRange's impl at git.go:787). run()/AddAll/WriteTree/ReadTree/Add/
                               #   EmptyTreeSHA/TreeDiff/StatusPorcelain/WorkingTreeDiff/LogRange UNCHANGED.
  readtree_test.go             # READ: the index-MUTATING test template (oracle pattern + error matrix).
  stagedcount_test.go          # READ: the query+parse template for DiffTreeNames.
  treediff_test.go             # READ: the two-tree-diff setup idiom.
  committree_test.go           # READ: writeFile/stageFile/writeTreeOf helpers (REUSE).
  revparsetree_test.go         # READ: execGit helper (the oracle) + EmptyTreeSHA idiom.
  revparse_test.go             # READ: makeEmptyCommit helper (REUSE).
  git_test.go                  # READ: initRepo helper (REUSE in every test).
  (*_test.go)                  # READ: other per-method test files (the one-file-per-method convention).
go.mod / go.sum                # UNCHANGED (stdlib only: context/fmt/sort/strings already imported in git.go).
.golangci.yml                  # READ: linter config (errcheck/gosimple/govet/ineffassign/staticcheck/unused).
```

### Desired Codebase tree with files to be added/modified

```bash
internal/git/git.go              # MODIFY — append to the `Git` interface (at the END, after `LogRange`,
                                 #   currently ~line 207):
                                 #     // FreezeWorkingTree captures T_start — the immutable tree object recording
                                 #     // the entire working-tree change set at run start (PRD §13.6.1 FR-M1b) — by
                                 #     // staging everything (AddAll), snapshotting the index (WriteTree → tStart),
                                 #     // then resetting the index back to the clean base (ReadTree(baseTree)) so the
                                 #     // per-concept stager starts clean. …index == baseTree after return; working-
                                 #     // tree files UNCHANGED; caller supplies baseTree (HEAD^{tree} or EmptyTreeSHA);
                                 #     // unborn case baseTree=EmptyTreeSHA → index reset to empty. Mutates the index;
                                 #     // touches NO ref.
                                 #     FreezeWorkingTree(ctx context.Context, baseTree string) (tStart string, err error)
                                 #     // DiffTreeNames returns the sorted, deduped list of paths that differ between
                                 #     // treeA and treeB (PRD §13.6.1 FR-M1c — the path-set primitive for freeze
                                 #     // enforcement). Runs `git diff-tree -r --name-only --no-commit-id <treeA>
                                 #     // <treeB>`. nil for identical trees. EmptyTreeSHA as treeA lists all of treeB.
                                 #     // Read-only; simple exit-code branch (0 with/without changes; 128 = real error).
                                 #     DiffTreeNames(ctx context.Context, treeA, treeB string) ([]string, error)
                                 #   AND append the two (*gitRunner) method bodies at the END of the file.
                                 #   NO other change to git.go.
internal/git/freezeworkingtree_test.go   # NEW — ~6 tests (full-change-set capture, index-reset-to-base,
                                         #   working-tree-unchanged, unborn EmptyTreeSHA, no-change idempotent,
                                         #   error cases: NotARepo/GitBinaryMissing/ContextCancelled).
internal/git/difftreenames_test.go       # NEW — ~8 tests (changed-path listing, sorted+deduped,
                                         #   identical→nil, EmptyTreeSHA base, del+add+mod, BadTreeSHA, NotARepo,
                                         #   GitBinaryMissing, ContextCancelled).
# go.mod/go.sum UNCHANGED. run()/AddAll/WriteTree/ReadTree UNCHANGED. 0 other edits.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (FreezeWorkingTree is ORCHESTRATION, not new plumbing — findings §2): its body is EXACTLY
//   g.AddAll(ctx); tStart, err := g.WriteTree(ctx); if err != nil { return "", err };
//   if err := g.ReadTree(ctx, baseTree); err != nil { return "", err }; return tStart, nil
// It issues NO new git command of its own. It reuses the three existing index mutators in the
// mandated order (AddAll → WriteTree → ReadTree). Do NOT inline raw `git add -A`/`git write-tree`/
// `git read-tree` calls (that would duplicate AddAll/WriteTree/ReadTree and bypass their error
// handling + doc comments). Do NOT add a `git reset`/`git restore` method — the contract mandates
// ReadTree as the index-reset primitive (scout §e: "to reset index to T_start use ReadTree").

// CRITICAL (return AddAll/WriteTree/ReadTree errors AS-IS — findings §4): FreezeWorkingTree must NOT
// re-wrap the three methods' errors. Return them directly (e.g. `if err := g.AddAll(ctx); err != nil
// { return "", err }`). This (a) preserves errors.Is(err, context.Canceled) / the "git binary not
// found" message at the call site, and (b) preserves WriteTree's "unresolved merge conflicts in the
// index — resolve them first" special-case message (which a re-wrap would bury). The methods already
// format their own stderr-inclusive messages.

// CRITICAL (the index-reset semantics — findings §7, scout Open Question #1): after FreezeWorkingTree
// returns, the INDEX == baseTree (ReadTree replaced it), but the working-tree files on disk are
// UNCHANGED (read-tree only rewrites .git/index). So `git status --porcelain` shows the user's changes
// as UNSTAGED. This is CORRECT and DESIRED (T_start is the frozen record; the stager then `git add`s
// from the working tree into the clean index). FreezeWorkingTree does NOT snapshot/restore the working
// tree (it doesn't need to). DOCUMENT this in the doc comment — a reader may expect the freeze to also
// restore the working tree, but it intentionally does not.

// CRITICAL (DiffTreeNames exit-code convention — findings §3): `git diff-tree` (without --quiet) exits
// 0 whether or not there are changes (identical trees → exit 0 + empty stdout; differing trees → exit 0
// + non-empty stdout). Exit 128 = bad/unresolvable tree SHA = a REAL error. Use the SIMPLE branch
// (`if code != 0 → error`), byte-identical to StagedDiff/TreeDiff/StagedFileCount. Do NOT add a
// 128-special-case (that is RevParseHEAD/RevParseTree's unborn convention — IRRELEVANT here; the caller
// resolves trees and passes EmptyTreeSHA for the unborn base). Do NOT use --quiet.

// CRITICAL (DiffTreeNames output is NOT sorted — findings §3): `git diff-tree -r --name-only` emits
// paths in TREE-WALK order, which is NOT guaranteed alphabetical. The contract mandates "sorted,
// deduped", so the impl MUST sort.Strings() the slice. Dedup is defensive (a single tree-pair diff
// lists each path exactly once), but apply the standard "sort then skip adjacent equal entries" idiom
// to make the deduped contract airtight. Identical trees → empty stdout → return nil (len 0).

// GOTCHA (run() returns err==nil for non-zero git exits — findings §4): run()'s INVARIANT is that a
// non-zero git exit is returned as (stdout, stderr, exitCode, nil) — err is nil, the exit code is the
// signal. Only infrastructural failures (LookPath miss / context cancel / start-I/O) return err != nil
// (exitCode -1). So DiffTreeNames does `if err != nil { return nil, err }` FIRST (catches context.Canceled
// + git-binary-missing, propagated UNWRAPPED so errors.Is works at the call site), THEN `if code != 0`.

// GOTCHA (the ONLY Git implementor is *gitRunner — findings §5): verified by exhaustive grep — no mock,
// fake, stub, or memory Git exists; every decompose test uses git.New(repo) (the real gitRunner). So
// adding two interface methods is PURELY ADDITIVE: no existing type needs updating, no existing test
// breaks. Do NOT search for / update a mock (there isn't one).

// GOTCHA (one-file-per-method test convention — findings §9): the package uses one test file per Git
// method. Put the tests in NEW files `freezeworkingtree_test.go` + `difftreenames_test.go`. Test
// function names MUST be distinct (prefix `TestFreezeWorkingTree_` / `TestDiffTreeNames_`). Do NOT
// redefine any package-level helper (initRepo/writeFile/stageFile/writeTreeOf/makeEmptyCommit/execGit)
// — redefining = duplicate-symbol compile error. Package is `git` (internal tests — helpers visible).

// GOTCHA (oracle pattern — findings §9): verify FreezeWorkingTree's index state and DiffTreeNames's
// output with INDEPENDENT git invocations (execGit(t, repo, "ls-files"), execGit(t, repo, "diff-tree",
// ...), execGit(t, repo, "cat-file", "-t", sha)), NOT via the method under test. This makes a passing
// test meaningful (it cross-checks two independent implementations). Mirror readtree_test.go's
// BEFORE/AFTER oracle pattern.

// GOTCHA (append at END to avoid merge friction — findings §5): append FreezeWorkingTree + DiffTreeNames
// at the END of the interface block (after LogRange) and at the END of the impl section (after LogRange's
// body). The parallel qwen-code work (P2.M1.T1.S2) does NOT touch git.go (confirmed), so there is no
// conflict, but END-append is the safe house style regardless. Do NOT edit the `// Method ownership`
// comment block or any existing method.
```

## Implementation Blueprint

### Data models and structure

No new TYPES. No new OPTIONS structs. No new constants (EmptyTreeSHA already exists at git.go:500). The
implementation consumes the existing primitives unchanged:

```go
// from internal/git/git.go (CONSUME — do not modify):
//   func (g *gitRunner) AddAll(ctx context.Context) error                                          // git.go:840
//   func (g *gitRunner) WriteTree(ctx context.Context) (sha string, err error)                     // git.go:337
//   func (g *gitRunner) ReadTree(ctx context.Context, tree string) error                           // git.go:944
//   func (g *gitRunner) run(ctx, repo, args ...string) (stdout, stderr string, exitCode int, err error)  // git.go:234
//   const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"                                // git.go:500
// run()'s invariant: non-zero git exit → (stdout, stderr, exitCode, nil); err != nil ⟺ infrastructural
// failure only (LookPath miss / context cancel / start-I/O), with exitCode == -1.
// AddAll/WriteTree/ReadTree each follow the two-branch convention internally and format their own
// stderr-inclusive error messages (FreezeWorkingTree returns them AS-IS).
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go — append FreezeWorkingTree + DiffTreeNames to the `Git` interface
  - LOCATE the END of the `Git` interface (the closing `}` after the `LogRange` method, currently
    ~line 210). APPEND both methods there (after LogRange) — appending at the end is the house style.
  - INSERT, before the closing brace, BOTH method declarations WITH doc comments (see "Implementation
    Patterns" for the exact text).
  - NAMING: `FreezeWorkingTree(ctx context.Context, baseTree string) (tStart string, err error)` and
    `DiffTreeNames(ctx context.Context, treeA, treeB string) ([]string, error)` (EXACT, from the work
    item). Use named returns where the file's doc-comment style does (FreezeWorkingTree: `(tStart string,
    err error)`; DiffTreeNames: `([]string, error)`).
  - FreezeWorkingTree DOC COMMENT must name: the 3-step orchestration (AddAll → WriteTree → ReadTree);
    the freeze semantics (T_start = immutable record of the working-tree change set at run start, PRD
    §13.6.1 FR-M1b); the index-reset contract (index == baseTree after return; working-tree files
    UNCHANGED — read-tree only rewrites .git/index); the caller-supplies-baseTree contract (HEAD^{tree}
    or EmptyTreeSHA); the unborn case (baseTree=EmptyTreeSHA → WriteTree captures all untracked;
    ReadTree(EmptyTreeSHA) resets the index to empty); the mutation/read-only convention (mutates the
    index transitively via AddAll+ReadTree; touches NO ref — refs move only at UpdateRefCAS, §18.1); and
    the partial-failure note (if ReadTree fails after WriteTree, the index is left staged — the caller
    owns recovery).
  - DiffTreeNames DOC COMMENT must name: the command (`git diff-tree -r --name-only --no-commit-id`);
    the sorted+deduped output contract; the `nil`-for-identical-trees contract (len 0); the EmptyTreeSHA-
    as-treeA unborn-base contract (lists all of treeB's files); the exit-code convention (0 with/without
    changes; 128 = bad/unresolvable tree SHA = a REAL error; simple branch — NO --quiet, NO
    128-special-case); and the read-only contract (mutates neither index nor ref).
  - GOTCHA: do NOT edit the `// Method ownership` comment block. Do NOT add any options struct.
  - PLACEMENT: internal/git/git.go, inside `type Git interface { … }`, at the END (after LogRange).

Task 2: MODIFY internal/git/git.go — implement (*gitRunner).FreezeWorkingTree (ORCHESTRATION of 3 methods)
  - APPEND `func (g *gitRunner) FreezeWorkingTree(ctx context.Context, baseTree string) (tStart string,
    err error)` at the END of the file (after LogRange's body).
  - BODY (see "Implementation Patterns" for the exact text): the 3-call sequence — AddAll, WriteTree
    (capture tStart), ReadTree(baseTree) — each error-checked, returning the existing method's error
    AS-IS. Return tStart on success.
  - GOTCHA: NO new git invocation (no g.run call). NO re-wrapping of AddAll/WriteTree/ReadTree errors.
    The body is ~6 lines. Mirror AddAll/ReadTree's doc-comment style on the impl (a short comment
    restating it orchestrates the three primitives + the index-reset contract).
  - PLACEMENT: internal/git/git.go, a new `(*gitRunner)` method at the END of the file (after LogRange).

Task 3: MODIFY internal/git/git.go — implement (*gitRunner).DiffTreeNames (one git diff-tree + parse)
  - APPEND `func (g *gitRunner) DiffTreeNames(ctx context.Context, treeA, treeB string) ([]string, error)`
    at the END of the file (after FreezeWorkingTree's body).
  - BODY (see "Implementation Patterns"): run `git diff-tree -r --name-only --no-commit-id treeA treeB`;
    two-branch error handling (err-first UNWRAPPED, then `if code != 0`); split stdout on "\n", trim each
    line, drop empty; sort.Strings(); dedupe adjacent equals; return the slice (nil if empty).
  - GOTCHA: the simple exit-code branch (NO 128-special-case, NO --quiet). sort.Strings() is REQUIRED
    (output is tree-walk order, not sorted). Empty stdout (identical trees) → nil slice, nil error.
    Error-message prefix: "git diff-tree: failed (exit %d): %s" (mirror StagedFileCount's wording).
  - PLACEMENT: internal/git/git.go, a new `(*gitRunner)` method at the END of the file.

Task 4: CREATE internal/git/freezeworkingtree_test.go — ~6 temp-repo tests
  - IMPORTS: context, errors, os, os/exec, strings, testing (mirror readtree_test.go). Package `git`.
  - ADD TestFreezeWorkingTree_CapturesFullChangeSet:
      repo := t.TempDir(); initRepo(t, repo)
      writeFile(t, repo, "a.txt", "a\n"); writeFile(t, repo, "b.txt", "b\n")
      stageFile(t, repo, "a.txt"); stageFile(t, repo, "b.txt"); makeEmptyCommit(t, repo, "base")
      baseTree := execGit(t, repo, "rev-parse", "HEAD^{tree}")
      # now make un-staged changes: modify a.txt, add c.txt, delete b.txt
      writeFile(t, repo, "a.txt", "a-modified\n"); writeFile(t, repo, "c.txt", "c-new\n")
      execGit(t, repo, "rm", "b.txt")   # tracked delete in the working tree
      g := New(repo)
      tStart, err := g.FreezeWorkingTree(context.Background(), baseTree)
      assert err == nil; assert tStart != "" && tStart != baseTree
      # ORACLE: git cat-file -t tStart == "tree"; git diff-tree baseTree tStart lists {a.txt, b.txt, c.txt}
      assert execGit(t, repo, "cat-file", "-t", tStart) == "tree"
      changed := execGit(t, repo, "diff-tree", "-r", "--name-only", "--no-commit-id", baseTree, tStart)
      assert sorted(changed split) == [a.txt b.txt c.txt]
  - ADD TestFreezeWorkingTree_ResetsIndexToBase:
      (same setup as above, after FreezeWorkingTree returns)
      # ORACLE: git ls-files == baseTree's files (a.txt b.txt — b.txt is STILL in the index because
      # ReadTree(baseTree) reset the index to baseTree, which still HAS b.txt; the working-tree delete
      # is not reflected in the reset index). Verify via execGit ls-files == "a.txt\nb.txt".
      assert execGit(t, repo, "ls-files") == "a.txt\nb.txt"
  - ADD TestFreezeWorkingTree_LeavesWorkingTreeUnchanged:
      (same setup) # the working-tree files on disk are UNCHANGED by the freeze
      # ORACLE: read a.txt from disk == "a-modified\n"; c.txt still present; b.txt gone from disk
      assert os.ReadFile("a.txt") == "a-modified\n"; assert fileExists("c.txt"); assert !fileExists("b.txt")
  - ADD TestFreezeWorkingTree_UnbornEmptyTreeBase:
      repo := t.TempDir(); initRepo(t, repo)   # ZERO commits — unborn
      writeFile(t, repo, "x.txt", "x\n"); writeFile(t, repo, "y.txt", "y\n")   # untracked
      g := New(repo)
      tStart, err := g.FreezeWorkingTree(context.Background(), EmptyTreeSHA)
      assert err == nil; assert tStart != ""
      # ORACLE: diff-tree EmptyTreeSHA tStart lists {x.txt, y.txt}; ls-files is EMPTY after reset
      changed := execGit(t, repo, "diff-tree", "-r", "--name-only", "--no-commit-id", EmptyTreeSHA, tStart)
      assert sorted(changed) == [x.txt y.txt]
      assert execGit(t, repo, "ls-files") == ""
  - ADD TestFreezeWorkingTree_NoChangesIdempotent:
      repo := t.TempDir(); initRepo(t, repo)
      writeFile(t, repo, "a.txt", "a\n"); stageFile(t, repo, "a.txt"); makeEmptyCommit(t, repo, "base")
      baseTree := execGit(t, repo, "rev-parse", "HEAD^{tree}")
      g := New(repo)
      tStart, err := g.FreezeWorkingTree(context.Background(), baseTree)   # clean tree — no changes
      assert err == nil; assert tStart == baseTree   # no changes ⇒ tStart == baseTree
  - ADD the error-matrix tests (copy the shape from readtree_test.go):
      TestFreezeWorkingTree_NotARepo: g := New(t.TempDir()) (no initRepo); _, err := g.FreezeWorkingTree(
        ctx, EmptyTreeSHA); assert err != nil (git add -A fails on a non-repo). Substring-agnostic OR
        assert it contains "git add -A: failed" (AddAll's error).
      TestFreezeWorkingTree_GitBinaryMissing: t.Setenv("PATH",""); g := New(t.TempDir()); _, err := ...;
        assert err != nil && strings.Contains(err.Error(), "git binary not found").
      TestFreezeWorkingTree_ContextCancelled: pre-cancelled ctx; _, err := ...; assert errors.Is(err,
        context.Canceled).
  - GOTCHA: do NOT redefine initRepo/writeFile/stageFile/makeEmptyCommit/execGit. For TestFreezeWorkingTree_
    ResetsIndexToBase, REMEMBER that ReadTree(baseTree) resets the index to baseTree's contents — which
    STILL INCLUDE b.txt (b.txt's delete was a working-tree delete; the baseTree was captured BEFORE the
    delete). So ls-files == "a.txt\nb.txt", NOT just "a.txt". Pin this with the test (it documents the
    index-reset semantics). Use small helpers (fileExists) defined LOCALLY in this test file (unique names).
  - PLACEMENT: NEW file internal/git/freezeworkingtree_test.go.

Task 5: CREATE internal/git/difftreenames_test.go — ~8 temp-repo tests
  - IMPORTS: context, errors, strings, testing. Package `git`.
  - ADD TestDiffTreeNames_ListsChangedPaths:
      repo := t.TempDir(); initRepo(t, repo)
      writeFile(t, repo, "a.txt", "a\n"); writeFile(t, repo, "b.txt", "b\n")
      stageFile(t, repo, "a.txt"); stageFile(t, repo, "b.txt"); makeEmptyCommit(t, repo, "base")
      treeA := execGit(t, repo, "rev-parse", "HEAD^{tree}")
      # modify a.txt, add c.txt, delete b.txt, stage all, capture treeB
      writeFile(t, repo, "a.txt", "a-mod\n"); writeFile(t, repo, "c.txt", "c\n"); execGit(t, repo, "rm", "b.txt")
      execGit(t, repo, "add", "-A"); treeB := writeTreeOf(t, repo)
      g := New(repo)
      got, err := g.DiffTreeNames(context.Background(), treeA, treeB)
      assert err == nil; assert got == [a.txt b.txt c.txt] (SORTED — b.txt is the deletion)
  - ADD TestDiffTreeNames_SortedAndDeduped:
      build a tree with many paths in non-sorted creation order; assert the returned slice is sorted and
      has no duplicates (use sort.Strings as the oracle and reflect.DeepEqual).
  - ADD TestDiffTreeNames_IdenticalTreesNil:
      g.DiffTreeNames(ctx, treeA, treeA) → (nil, nil) or ([]string{}, nil); assert len == 0.
  - ADD TestDiffTreeNames_EmptyTreeBase:
      unborn setup (untracked files → treeB); DiffTreeNames(ctx, EmptyTreeSHA, treeB) lists all of treeB.
  - ADD TestDiffTreeNames_DeletionsAdditionsModifications:
      (covered by TestDiffTreeNames_ListsChangedPaths — a.txt mod, b.txt del, c.txt add; assert all three).
  - ADD TestDiffTreeNames_BadTreeSHA:
      DiffTreeNames(ctx, "0000...0", treeB) → err != nil, contains "git diff-tree: failed".
  - ADD TestDiffTreeNames_NotARepo:
      g := New(t.TempDir()) (no initRepo); DiffTreeNames → err != nil.
  - ADD TestDiffTreeNames_GitBinaryMissing + TestDiffTreeNames_ContextCancelled:
      (copy the shape from readtree_test.go; t.Setenv("PATH","") → "git binary not found"; pre-cancelled
      ctx → errors.Is(err, context.Canceled)).
  - GOTCHA: do NOT redefine execGit/writeTreeOf/initRepo/writeFile/stageFile/makeEmptyCommit. For the
    sorted-oracle, use sort.Strings + reflect.DeepEqual (import "reflect" or compare via strings.Join).
  - PLACEMENT: NEW file internal/git/difftreenames_test.go.

Task 6: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/git/git.go internal/git/freezeworkingtree_test.go internal/git/difftreenames_test.go`
  - `go build ./...`   (whole module compiles — the interface + 2 impls + 2 test files)
  - `go vet ./...`
  - `golangci-lint run ./...`   (errcheck/gosimple/govet/ineffassign/staticcheck/unused)
  - `go test -race ./internal/git/ -run "TestFreezeWorkingTree|TestDiffTreeNames" -v`   (all new tests)
  - `go test -race ./internal/git/`   (the WHOLE git package — existing tests still pass)
  - `go test ./...`   (FULL regression — no other package breaks; the 2 new interface methods are additive)
  - `git diff --exit-code go.mod go.sum` ⇒ empty.
  - `git status --short` ⇒ EXACTLY: `M internal/git/git.go` + `?? internal/git/freezeworkingtree_test.go`
    + `?? internal/git/difftreenames_test.go` (3 entries); run()/AddAll/WriteTree/ReadTree UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// === Task 1: the interface additions (append at the END of `type Git interface { … }`, after LogRange) ===

	// FreezeWorkingTree captures T_start — the immutable tree object recording the ENTIRE working-tree
	// change set (every modified/added/deleted/untracked path AND its byte content) at the instant the
	// run begins (PRD §13.6.1 FR-M1b: "the first action on activation is to freeze the entire working-tree
	// change set into T_start"). It orchestrates three existing primitives in this exact order:
	//   1. AddAll  — stages the full working-tree change set into the index (git add -A).
	//   2. WriteTree — snapshots the index into the immutable tree object T_start (git write-tree).
	//   3. ReadTree(baseTree) — REPLACES the index with baseTree's contents (git read-tree), resetting it
	//      to the clean base so the per-concept stager starts from a known-clean index state.
	// The caller supplies baseTree (HEAD^{tree} via RevParseTree, or EmptyTreeSHA for an unborn repo) —
	// the orchestrator already derives it, so FreezeWorkingTree does not re-derive it.
	//
	// INDEX-RESET SEMANTICS: after FreezeWorkingTree returns, the INDEX == baseTree (ReadTree replaced
	// it), but the working-tree files on disk are UNCHANGED (read-tree only rewrites .git/index). So
	// `git status` shows the user's changes as UNSTAGED relative to the reset index — this is CORRECT and
	// DESIRED: T_start is the frozen immutable record; the stager then `git add`s from the working tree
	// into the clean (base-matching) index. FreezeWorkingTree does NOT snapshot or restore the working
	// tree (it does not need to). Defense-in-depth (FR-M1c: verify tree[i] ⊆ T_start) is the caller's job.
	//
	// UNBORN CASE (baseTree == EmptyTreeSHA): AddAll stages all untracked files; WriteTree makes T_start
	// (a root tree holding them); ReadTree(EmptyTreeSHA) resets the index to EMPTY. The untracked files
	// reappear in `git status` (unstaged). EmptyTreeSHA is a valid read-tree target (verified).
	//
	// FreezeWorkingTree MUTATES THE INDEX (transitively, via AddAll + ReadTree) but touches NO ref — refs
	// move ONLY at UpdateRefCAS (PRD §18.1). Partial failure: if ReadTree fails after WriteTree succeeds,
	// the index is left STAGED (holding the full change set) and T_start is discarded — the caller owns
	// recovery (mirrors runSingleShortcut's mid-sequence failure handling). WriteTree's "unresolved merge
	// conflicts" special-case (its own error) propagates AS-IS.
	FreezeWorkingTree(ctx context.Context, baseTree string) (tStart string, err error)

	// DiffTreeNames returns the SORTED, DEDUPED list of paths that differ between two tree SHAs via
	// `git diff-tree -r --name-only --no-commit-id <treeA> <treeB>`. It is the path-set primitive for
	// freeze enforcement (PRD §13.6.1 FR-M1c: "after each staging step, stagehand verifies the resulting
	// tree is a subset of T_start — only paths present in T_start") — the enforcement layer computes
	// DiffTreeNames(prevTree, tree[i]) and checks it is a subset of DiffTreeNames(baseTree, tStart). It is
	// also reusable for FR-M9's arbiter file-lists and FR-M8's empty-skip (tree[i]==tree[i-1] ⇔ empty set).
	//
	// `-r` recurses into subdirectories (lists individual files, not subtrees); `--name-only` emits just
	// the path (no status code); `--no-commit-id` suppresses the commit-SHA header line (safe even when
	// the args are trees, which emit no SHA anyway). For the unborn-repo base case the caller passes
	// EmptyTreeSHA as treeA (DiffTreeNames is NOT unborn-aware — like TreeDiff, the caller resolves trees).
	// Identical trees (treeA == treeB) ⇒ empty stdout ⇒ returns (nil, nil) (a nil slice, len 0).
	//
	// `git diff-tree` (without --quiet) exits 0 whether or not there are changes; exit 128 means a bad or
	// unresolvable tree SHA — a REAL error (NOT an unborn signal: branch on code != 0, never on code ==
	// 128; never use --quiet). Read-only with respect to refs and the index (PRD §18.1).
	DiffTreeNames(ctx context.Context, treeA, treeB string) (paths []string, err error)


// === Task 2: FreezeWorkingTree impl (ORCHESTRATION — no new git command; reuse AddAll/WriteTree/ReadTree) ===

// FreezeWorkingTree captures T_start by staging everything, snapshotting the index, then resetting the
// index to the clean base (PRD §13.6.1 FR-M1b). It is a thin orchestration of AddAll + WriteTree +
// ReadTree — see the interface doc comment for the full semantics. It issues NO git command of its own.
func (g *gitRunner) FreezeWorkingTree(ctx context.Context, baseTree string) (string, error) {
	// 1. Stage the full working-tree change set (modifications, additions/untracked, AND deletions).
	if err := g.AddAll(ctx); err != nil {
		return "", err // AddAll's own error (incl. "git binary not found", context.Canceled) — UNWRAPPED.
	}
	// 2. Freeze the index into the immutable tree object T_start.
	tStart, err := g.WriteTree(ctx)
	if err != nil {
		return "", err // WriteTree's own error (incl. "unresolved merge conflicts") — UNWRAPPED.
	}
	// 3. Reset the index to the clean base so the per-concept stager starts clean.
	if err := g.ReadTree(ctx, baseTree); err != nil {
		return "", err // ReadTree's own error ("git read-tree: failed") — UNWRAPPED. (Index left staged.)
	}
	return tStart, nil
}


// === Task 3: DiffTreeNames impl (one git diff-tree invocation + split/trim/sort/dedupe) ===

// DiffTreeNames returns the sorted, deduped changed-path set between two trees (PRD §13.6.1 FR-M1c).
// See the interface doc comment for the full contract. It runs `git diff-tree -r --name-only
// --no-commit-id <treeA> <treeB>`, parses stdout into a sorted, deduped []string (nil if identical).
func (g *gitRunner) DiffTreeNames(ctx context.Context, treeA, treeB string) ([]string, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir,
		"diff-tree", "-r", "--name-only", "--no-commit-id", treeA, treeB)
	if err != nil {
		return nil, err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
	}
	if code != 0 {
		return nil, fmt.Errorf("git diff-tree: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	var paths []string
	for _, line := range strings.Split(stdout, "\n") {
		if p := strings.TrimSpace(line); p != "" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths) // tree-walk order is NOT alphabetical — the contract mandates sorted output
	// Dedupe adjacent equal entries (defensive: a single tree-pair diff lists each path once).
	out := paths[:0]
	for i, p := range paths {
		if i == 0 || p != paths[i-1] {
			out = append(out, p)
		}
	}
	return out, nil // nil if stdout was empty (identical trees); out aliases paths (same backing array)
}
```

### Integration Points

```yaml
DATABASE:
  - none (git object store gains the T_start tree object; the index is reset to baseTree; no ref moves).
    FreezeWorkingTree is the producer of T_start; ReadTree is the index-reset.

CONFIG:
  - none directly. baseTree is passed in by the caller (the orchestrator resolves it: HEAD^{tree} via
    RevParseTree, or EmptyTreeSHA for an unborn repo). This layer takes it as a plain param — it is
    decoupled from config (mirrors RevParseTree/TreeDiff taking refs/trees as params).

ROUTES:
  - none (internal plumbing methods; no CLI flag, no public API surface in this task). The consumers are:
    - P3.M1.T1.S2 (T_start wiring): after deriving baseTree (decompose.go:~152), BEFORE callPlanner:
        tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)
      then threads tStart through the planner/stagers/arbiter/shortcuts.
    - P3.M2.T1.S1 (freeze enforcement, FR-M1c): after each staging step:
        staged := DiffTreeNames(ctx, prevTree, tree[i])
        allowed := DiffTreeNames(ctx, baseTree, tStart)
        # verify staged ⊆ allowed (every staged path is a T_start path); hard abort on violation.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the modified + new files.
gofmt -w internal/git/git.go internal/git/freezeworkingtree_test.go internal/git/difftreenames_test.go
gofmt -l internal/ pkg/          # Expected: empty.

# Lint.
golangci-lint run ./internal/git/...
golangci-lint run ./...          # Expected: clean.

# Vet.
go vet ./...                     # Expected: no findings.

# Expected: zero errors. If any exist, READ the output and fix before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Run the new tests in isolation (verbose — confirm every case).
go test -race ./internal/git/ -run "TestFreezeWorkingTree" -v
go test -race ./internal/git/ -run "TestDiffTreeNames" -v

# Whole git package (existing tests must still pass — the 2 new methods are additive).
go test -race ./internal/git/

# Expected: all pass. The freeze tests' ORACLE assertions (independent git ls-files / diff-tree / cat-file)
# are the strongest guards — a mismatch means the index-reset or capture semantics drifted; debug root
# cause (do NOT weaken the oracle assertion to make it pass). The DiffTreeNames sorted/deduped test pins
# the ordering contract (output is NOT git's raw tree-walk order).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the whole module (the interface + 2 impls compile + link; no existing importer breaks).
go build ./...

# Full regression (purely additive — no other package should change behavior).
go test ./...

# Confirm the module is unchanged apart from the 1 modified + 2 new files.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged"
git status --short   # Expected: M internal/git/git.go  ?? internal/git/freezeworkingtree_test.go  ?? internal/git/difftreenames_test.go

# (No live agent / service to start — these are pure git-plumbing methods tested against real temp repos.
#  The "integration" is the decompose orchestrator in P3.M1.T1.S2, a SEPARATE work item not yet existing.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# §13.6.1 FR-M1b faithfulness self-check (run interactively; eyeball the freeze semantics):
go test -run "TestFreezeWorkingTree_CapturesFullChangeSet|TestFreezeWorkingTree_ResetsIndexToBase|TestFreezeWorkingTree_Unborn" -v ./internal/git/

# FR-M1c path-set primitive self-check (the subset-enforcement foundation):
go test -run "TestDiffTreeNames" -v ./internal/git/

# Freeze-immutability spot-check (PRD §20.2 "Start-of-run freeze (v2)"): T_start is stable regardless of
# later index mutations — the freeze tests assert tStart is a tree object whose diff vs baseTree is the
# fixed change set; a follow-up index mutation (e.g. a second AddAll) does NOT change tStart's content
# (write-tree captured it immutably). (This invariant is inherent to git tree objects; the test documents it.)

# Expected: all pass. The freeze tests prove T_start captures the FULL change set + the index is reset to
# base + the working tree is untouched; the DiffTreeNames tests prove the sorted/deduped path set + the
# identical→nil + EmptyTreeSHA-base contracts. These are the domain-specific validations for the freeze
# feature's plumbing layer.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] All tests pass: `go test ./...` (and specifically `go test -race ./internal/git/`).
- [ ] No lint errors: `golangci-lint run ./internal/git/...` (and `./...`).
- [ ] No vet errors: `go vet ./...`.
- [ ] No formatting issues: `gofmt -l internal/ pkg/` empty.
- [ ] go.mod / go.sum UNCHANGED (`git diff --exit-code go.mod go.sum` ⇒ empty).

### Feature Validation

- [ ] FreezeWorkingTree captures T_start whose `git diff-tree` vs baseTree lists EXACTLY the working-tree
      change set (mod + add + del), verified by an independent oracle.
- [ ] After FreezeWorkingTree, the index == baseTree (`git ls-files` oracle) and the working-tree files
      are unchanged on disk.
- [ ] FreezeWorkingTree handles the unborn case (baseTree=EmptyTreeSHA → T_start holds all untracked;
      index reset to empty).
- [ ] DiffTreeNames returns the sorted, deduped path set; `nil` for identical trees; EmptyTreeSHA-as-treeA
      lists all of treeB.
- [ ] Both methods return a non-nil error on NotARepo / GitBinaryMissing ("git binary not found") /
      ContextCancelled (`errors.Is(err, context.Canceled)`) / BadTreeSHA (DiffTreeNames).
- [ ] FreezeWorkingTree returns AddAll/WriteTree/ReadTree errors AS-IS (no re-wrap; WriteTree's
      "unresolved merge conflicts" message survives).

### Code Quality Validation

- [ ] Follows existing git-package conventions (one-file-per-method tests; two-branch error handling;
      rich interface doc comments citing PRD §13.6.1 FR-M1b/c; []string args, no shell — §19).
- [ ] File placement matches the desired tree (1 modified + 2 new in internal/git/).
- [ ] Anti-patterns avoided (no re-wrap of existing errors; no `git reset`/`git restore`; no 128-special-
      case in DiffTreeNames; no redefined test helpers; no edit to existing methods).
- [ ] Dependencies: stdlib only (context/fmt/sort/strings — all already imported); no new internal dep.

### Documentation & Deployment

- [ ] Rich doc comments on both interface methods (cite §13.6.1 FR-M1b/c; the freeze's 3-step
      orchestration + index-reset semantics + working-tree-unchanged contract + unborn case; DiffTreeNames's
      command + sorted/deduped contract + exit-code convention).
- [ ] No new environment variables or config (baseTree is a plain param).
- [ ] [Mode A] self-documenting via the interface doc comments (no separate docs file needed).

---

## Anti-Patterns to Avoid

- ❌ Don't issue raw `git add -A`/`git write-tree`/`git read-tree` inside FreezeWorkingTree — ORCHESTRATE
  the existing AddAll/WriteTree/ReadTree methods (they own the error handling + doc comments).
- ❌ Don't re-wrap AddAll/WriteTree/ReadTree errors — return them AS-IS (preserves errors.Is + WriteTree's
  "unresolved merge conflicts" message).
- ❌ Don't add a `git reset`/`git restore` method — the contract mandates ReadTree as the index-reset
  primitive (no reset helper exists by design; scout §e).
- ❌ Don't snapshot/restore the working tree in FreezeWorkingTree — it intentionally leaves working-tree
  files unchanged (read-tree only rewrites .git/index); T_start is the immutable record.
- ❌ Don't add a 128-special-case or `--quiet` to DiffTreeNames — it's the simple-branch convention
  (code != 0 → error), byte-identical to StagedDiff/TreeDiff/StagedFileCount.
- ❌ Don't skip `sort.Strings()` in DiffTreeNames — git's tree-walk order is NOT alphabetical; the contract
  mandates sorted output.
- ❌ Don't modify run(), AddAll, WriteTree, ReadTree, or any existing method — all CONSUMED as-is.
- ❌ Don't search for / update a Git mock — *gitRunner is the only implementor; no mock exists.
- ❌ Don't redefine initRepo/writeFile/stageFile/writeTreeOf/makeEmptyCommit/execGit in the new test
  files — they're already package-level (duplicate-symbol compile error).
- ❌ Don't weaken an oracle assertion to make a test pass — debug the root cause (a mismatch means the
  freeze/reset/sort semantics drifted).
- ❌ Don't wire FreezeWorkingTree into the decompose orchestrator or add freeze enforcement — those are
  P3.M1.T1.S2 and P3.M2.T1.S1 (separate work items). THIS task only adds + tests the two primitives.

---

**Confidence Score: 9/10** — This is a low-risk, high-reuse plumbing task. FreezeWorkingTree is a 6-line
orchestration of three EXISTING, well-tested methods (AddAll/WriteTree/ReadTree) — no new git command, no
new logic; its only subtlety (the index-reset leaves the working tree unchanged) is empirically verified
and documented. DiffTreeNames is one new `git diff-tree` invocation + split/trim/sort/dedupe, mirroring
the established StagedFileCount query+parse pattern. Both methods are PURELY ADDITIVE to the interface
(*gitRunner is the sole Git implementor — verified, no mocks to update), stdlib-only deps, no import
cycle, no caller wiring in this task. The two consumers (P3.M1.T1.S2 wiring + P3.M2.T1.S1 enforcement)
are separate work items whose contracts this PRP pins precisely. The only residual uncertainty is the
exact freeze-test oracle for the index-reset state (ls-files == baseTree's files, which still INCLUDE a
working-tree-deleted file because the baseTree pre-dates the delete) — resolved by an explicit test that
pins it, and trivially correct since it follows directly from ReadTree's REPLACES semantics. The parallel
qwen-code work does not touch git.go (confirmed), so there is no merge conflict.
