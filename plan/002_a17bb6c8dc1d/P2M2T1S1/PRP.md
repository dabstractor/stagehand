---
name: "P2.M2.T1.S1 — Implement RevParseTree + ReadTree on the Git interface and gitRunner (PRD §13.6.3 / §13.6.5)"
description: |

  ADD two new methods to the `Git` interface (`internal/git/git.go`) AND implement them on the
  `gitRunner` struct, consuming the existing `run()` helper:

    RevParseTree(ctx context.Context, ref string) (string, error)
    ReadTree(ctx context.Context, tree string) error

  These are the V2 git-plumbing primitives that enable multi-commit decomposition (PRD §13.6):
    - **RevParseTree** runs `git rev-parse <ref>^{tree}` and returns the tree SHA of a commit-ish
      (HEAD, or a commit SHA). It provides `tree[-1]` — the original parent tree that the per-concept
      tree-to-tree concept diffs (§13.6.3 invariant 2; P2.M2.T1.S2 TreeDiff) and the unborn/empty-tree
      base case are computed against. Exit 128 (unborn repo / unresolvable ref) → return ("", nil)
      defensively (callers gate on isUnborn), mirroring RevParseHEAD's 128 convention.
    - **ReadTree** runs `git read-tree <tree>` and REPLACES the index with that tree's contents. It
      MUTATES THE INDEX (writes .git/index) but touches NEITHER HEAD NOR any ref. It is consumed ONLY
      by the arbiter's mid-chain rebuild (PRD §13.6.5; P3.M3.T2): read-tree a base → fold leftovers in
      via git add → write-tree → commit-tree. Non-zero exit (128 = bad SHA / not-a-repo / corrupt) →
      non-nil error (it is a mutation: ALL non-zero exits are errors, like AddAll/WriteTree/CommitTree).

  CONTRACT (P2.M2.T1.S1, verbatim from the work item):
    1. RESEARCH: The Git interface in internal/git/git.go lists all methods with doc comments. New
       methods are added to the interface AND the gitRunner struct. RevParseTree:
       `git rev-parse <ref>^{tree}` returns the tree SHA of a commit-ish (e.g. HEAD, or a commit SHA).
       For an unborn repo with ref=HEAD, git exits 128 — return ("", nil) defensively (callers gate on
       isUnborn). ReadTree: `git read-tree <tree>` loads a tree into the index — MUTATES THE INDEX.
       Used only by the arbiter's mid-chain rebuild (P3.M3.T2). Both use the existing run()/runWithInput()
       helpers with -C flag.
    2. INPUT: The existing Git interface and gitRunner.run() helper.
    3. LOGIC: Add to Git interface: `RevParseTree(ctx, ref string) (string, error)` and
       `ReadTree(ctx, tree string) error`. Implement on gitRunner: RevParseTree runs
       `git rev-parse <ref>^{tree}`, handles 128 as empty (unborn), trims stdout. ReadTree runs
       `git read-tree <tree>`, non-zero exit → error.
    4. OUTPUT: Two new Git interface methods available to the decompose pipeline. RevParseTree provides
       the base tree (tree[-1]) for concept diffs. ReadTree enables chain rebuild.
    5. DOCS: none — interface additions, documented by doc comments on the interface.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `run()` / `runWithInput()` in git.go — CONSUMED, not modified. Neither new method needs stdin.
    - `internal/git/binary.go` + `binary_test.go` — P2.M1 (binary filtering). NOT touched (RevParseTree
      and ReadTree do not diff, so there is no binary handling here). TreeDiff (S2) reuses binary.go.
    - go.mod / go.sum — UNCHANGED (only stdlib already imported: context/fmt/strings).
    - Decompose wiring (roles.go / planner.go / stager.go / arbiter.go) — P3.x. NOT this task; this task
      only adds + tests the two primitives. No caller references them yet (the interface gains methods;
      existing callers are unaffected — additions are backward-compatible).
    - TreeDiff (P2.M2.T1.S2), StatusPorcelain (P2.M2.T2.S1), WorkingTreeDiff (P2.M2.T2.S2) — sibling
      work items. Do NOT implement them here. To avoid file-level merge conflicts with S2's
      treediff_test.go, place RevParseTree/ReadTree tests in their OWN new files.

  DELIVERABLES (1 file MODIFIED, 2 new files):
    MODIFY internal/git/git.go            — add `RevParseTree` + `ReadTree` to the `Git` INTERFACE
                                           (with doc comments) AND implement both on `*gitRunner`.
    CREATE internal/git/revparsetree_test.go — tests for RevParseTree (born/HEAD, born/commitSHA,
                                           unborn→("",nil), git-binary-missing, context-cancelled).
    CREATE internal/git/readtree_test.go  — tests for ReadTree (loads tree into index, replaces index,
                                           bad-tree error, not-a-repo error, git-binary-missing,
                                           context-cancelled).

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass (the two new
  interface methods are additive — no existing caller breaks); `git rev-parse HEAD^{tree}` on a born
  repo returns the SAME SHA as `git write-tree` over the same staged+committed index; `read-tree` of a
  prior tree REPLACES the index (verified independently via `git ls-files`); exit 128 on RevParseTree
  returns ("", nil); every non-zero ReadTree exit returns a non-nil error.

---

## Goal

**Feature Goal**: Add the two V2 git-plumbing primitives the multi-commit decomposition pipeline (PRD
§13.6) depends on, at the `Git` interface boundary, by delegating to the existing `run()` helper:

1. **`RevParseTree(ctx, ref string) (string, error)`** — runs `git rev-parse <ref>^{tree}`, returns the
   tree SHA of a commit-ish (`ref` = `HEAD`, a branch, or a commit SHA). This is the producer of
   `tree[-1]` — the original-parent tree that anchors the tree-to-tree concept-diff loop (§13.6.3
   invariant 2 / base-case note: "`tree[-1]` is the original parent tree (`git rev-parse HEAD^{tree}`,
   or the empty tree for an unborn repo)"). Exit 128 (unborn repo with `ref=HEAD`, OR an unresolvable
   ref) → `("", nil)` defensively — callers gate on `isUnborn` (from RevParseHEAD) before calling, so
   an empty return is the correct, non-error signal. This 128-as-non-error convention is IDENTICAL to
   RevParseHEAD / RecentMessages / RecentSubjects / CommitCount.

2. **`ReadTree(ctx, tree string) error`** — runs `git read-tree <tree>`, REPLACING the index with that
   tree's contents. It is a pure index mutation (writes `.git/index`); it touches NEITHER HEAD NOR any
   ref. It is consumed ONLY by the arbiter's mid-chain chain rebuild (§13.6.5: "for each j, read-tree
   the appropriate base, fold the leftovers in at j==i, write-tree, commit-tree against the rebuilt
   parent, update-ref"). Because it is a mutation, EVERY non-zero exit (128 = bad/unresolvable tree
   SHA, not-a-repo, corrupt object) is a real error — the SAME convention as AddAll / WriteTree /
   CommitTree (mutations never special-case 128 as "unborn").

**Deliverable** (1 file MODIFIED, 2 new):
1. `internal/git/git.go` — (a) two new methods appended to the `Git` interface, each with a doc comment
   explaining its command, its exit-code semantics, its role in §13.6, and the read-vs-mutation
   distinction; (b) the two corresponding `(*gitRunner)` implementations, each following the universal
   4-step `run()` shape (err-check → signal-branch → error-branch → success).
2. `internal/git/revparsetree_test.go` — 5 tests: born-repo HEAD → SHA equals an independent
   `write-tree` oracle; born-repo commit-SHA → SHA equals the HEAD-tree oracle; unborn-repo HEAD →
   `("", nil)`; git-binary-missing → `"git binary not found"` error; context-cancelled →
   `errors.Is(err, context.Canceled)`.
3. `internal/git/readtree_test.go` — 6 tests: loads tree into index (verified via independent
   `git ls-files`); REPLACES the index (load an OLDER tree → only that tree's files remain); bad-tree
   SHA → non-nil error containing the failure marker; not-a-repo → non-nil error; git-binary-missing →
   `"git binary not found"`; context-cancelled → `errors.Is(err, context.Canceled)`.

**Success Definition**:
- On a born repo with a committed `a.txt`, `RevParseTree(ctx, "HEAD")` returns a SHA byte-identical to
  `writeTreeOf(t, repo)` computed over the same staged-and-committed index (proves `^{tree}` peeling =
  git's own tree resolution).
- `RevParseTree(ctx, "<commitSHA>")` returns the same SHA as `RevParseTree(ctx, "HEAD")` when that SHA
  IS HEAD (proves commit-ish peeling works, not just the `HEAD` literal).
- On an unborn (zero-commit) repo, `RevParseTree(ctx, "HEAD")` returns `("", nil)` — NOT an error, NOT
  the literal `HEAD^{tree}` string that git prints to stdout (the exit-128, not-stdout-emptiness rule).
- `ReadTree(ctx, <validTree>)` mutates the index: after loading `HEAD~1^{tree}` into a repo whose index
  holds `a.txt`+`b.txt`, an independent `git ls-files` shows ONLY `a.txt` (proves REPLACES, not merges).
- `ReadTree(ctx, "0000…0000")` returns a non-nil error; `ReadTree` on a non-repo dir returns a non-nil
  error (every non-zero exit is an error — mutation convention).
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` shows EXACTLY 1 modified
  (`internal/git/git.go`) + 2 new untracked test files — nothing else.

## User Persona

**Target User**: the decompose pipeline's orchestrator + arbiter (internal code, P3.x), and by
extension the end user running `stagecoach` on an un-staged working tree to get multiple logically-
coherent commits. These primitives are NOT user-facing CLI flags; they are the plumbing the pipeline is
built on.

**Use Case**:
- *RevParseTree*: at the start of a decompose run, the orchestrator captures `tree[-1] =
  RevParseTree(ctx, "HEAD")` — the immutable baseline tree that each per-concept `tree[i]` is diffed
  against (concept diff = `git diff tree[i-1] tree[i]`, §13.6.3 invariant 2). For an unborn repo the
  empty return triggers the "commit[0] is a root commit" base case.
- *ReadTree*: only in the arbiter's mid-chain branch (§13.6.5, "target == an earlier commit[i]"), the
  orchestrator rebuilds the linear chain by `read-tree`-ing each base tree in turn, folding leftovers in
  at commit[i], then `write-tree` + `commit-tree`. ReadTree is the primitive that loads a base into the
  index for that deterministic reconstruction.

**Pain Points Addressed**: the decompose pipeline currently has NO way to resolve a commit's tree
(rev-parse `^{tree}`) or to load a known tree back into the index (read-tree). Without RevParseTree,
there is no `tree[-1]` baseline → concept diffs are impossible. Without ReadTree, the arbiter's
mid-chain rebuild (the only non-trivial leftover-reconciliation path) is impossible. This task provides
both primitives, isolated, tested, and at the interface boundary so the P3 pipeline can compose them.

## Why

- **Closes PRD §13.6.3 / §13.6.5 at the plumbing layer.** §13.6.3 invariant 2 mandates tree-to-tree
  concept diffs (never index-vs-HEAD), which requires resolving `tree[-1]` via `rev-parse HEAD^{tree}`
  (the section's own base-case note names this exact command). §13.6.5's mid-chain rebuild mandates
  `read-tree` the base. This task is the literal implementation of those two named commands.
- **Reuses the proven run() boundary — zero new exec surface.** Both methods are thin wrappers over the
  existing `run()` helper (the SAME helper RevParseHEAD/AddAll/WriteTree/StagedDiff use). No new shell
  surface, no new LookPath, no new buffer handling. The only new logic is the per-command exit-code
  interpretation, which follows the two conventions already established in the file (read-methods:
  128 = unborn-not-error; mutation-methods: all non-zero = error).
- **Read vs mutation is encoded structurally.** RevParseTree (read) special-cases 128 → ("", nil);
  ReadTree (mutation) does not. This structural distinction makes the safety contract (§18.1: refs move
  ONLY at UpdateRefCAS; the index is the stager's domain) self-documenting at the type level — a reader
  sees RevParseTree is a read (returns a value, can fail soft on unborn) and ReadTree is a mutation
  (returns only an error, fails hard on anything non-zero), matching AddAll's documented mutation
  convention exactly.
- **Low-risk, additive, backward-compatible.** The interface GAINS two methods (Go interfaces are
  open for extension — no existing implementation or caller breaks). No existing file's behavior
  changes. The two new methods are independently testable. go.mod/go.sum untouched.
- **Unblocks P3.M2.T4 (per-concept message gen + tree[-1]) and P3.M3.T2 (arbiter chain rebuild).**
  Those work items name these exact methods in their contracts; this task is their prerequisite.

## What

Two new methods on the `Git` interface (`internal/git/git.go`), implemented on `*gitRunner`, both
delegating to `run()`. No new types, no new options structs, no new dependencies, no changes to `run()`
or `runWithInput()`. No caller wiring (that is P3.x). The only structural edits are: append two method
signatures + doc comments to the `Git` interface, and append two `(*gitRunner)` method bodies.

### Success Criteria

- [ ] `RevParseTree(ctx context.Context, ref string) (string, error)` is declared on the `Git`
      interface with a doc comment naming: the command (`git rev-parse <ref>^{tree}`), the `^{tree}`
      peeling semantics (tree SHA of a commit-ish: HEAD/branch/SHA), the exit-128 → `("", nil)`
      defensive return (callers gate on isUnborn), and its §13.6.3 role (producer of `tree[-1]`).
- [ ] `ReadTree(ctx context.Context, tree string) error` is declared on the `Git` interface with a doc
      comment naming: the command (`git read-tree <tree>`), that it MUTATES THE INDEX (replaces it)
      but touches NO ref, that it is consumed ONLY by the arbiter mid-chain rebuild (§13.6.5 / P3.M3.T2),
      and that ALL non-zero exits are errors (mutation convention, like AddAll).
- [ ] `(*gitRunner).RevParseTree` runs `g.run(ctx, g.workDir, "rev-parse", ref+"^{tree}")` (the
      `^{tree}` suffix is ONE argv element — NOT two args); on `code == 128` returns `("", nil)`;
      on `code != 0` returns a wrapped error; on `code == 0` returns `(strings.TrimSpace(stdout), nil)`.
- [ ] `(*gitRunner).ReadTree` runs `g.run(ctx, g.workDir, "read-tree", tree)`; on `code != 0` returns
      `fmt.Errorf("git read-tree: failed (exit %d): %s", code, strings.TrimSpace(stderr))`; on
      `code == 0` returns `nil`. NO 128 special-case (it is a mutation).
- [ ] Both implementations propagate infrastructural failures (`err != nil` from run()) UNWRAPPED so
      `errors.Is(err, context.Canceled)` survives (the universal run() contract).
- [ ] `revparsetree_test.go` proves: born-HEAD SHA == independent `write-tree` oracle; born-commitSHA
      SHA == HEAD-tree oracle; unborn-HEAD → `("", nil)`; git-missing → `"git binary not found"`;
      ctx-cancelled → `errors.Is(err, context.Canceled)`.
- [ ] `readtree_test.go` proves: index loads the tree (independent `git ls-files`); REPLACES the index
      (older tree → only its files); bad-tree → non-nil error; non-repo → non-nil error; git-missing →
      `"git binary not found"`; ctx-cancelled → `errors.Is(err, context.Canceled)`.
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 1 modified + 2 new files.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact run() contract
(findings §3 — the 4-step shape + the err-nil-for-non-zero-exit invariant); the verified exit-code
behaviors (findings §1 rev-parse: 128 on unborn/literal-stdout; findings §2 read-tree: REPLACES index,
all non-zero = error); the `^{tree}`-as-one-argv-element gotcha (findings §1); the read-vs-mutation
convention split (findings §1/§2 — mirrors RevParseHEAD vs AddAll); the test fixtures to reuse
(findings §5 — initRepo/makeEmptyCommit/writeFile/stageFile/writeTreeOf/headSHA, all package-visible,
no new helpers); the one-file-per-method test convention (findings §6 — new files, distinct names); and
the independent-oracle assertion idiom (findings §5). No prompt/decompose/registry knowledge required —
the contract is fully self-contained at the git-plumbing layer.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (verified exit codes + argv construction + conventions)
- docfile: plan/002_a17bb6c8dc1d/P2M2T1S1/research/findings.md
  why: §1 the four rev-parse behavioral shapes + the 128-branch-on-code-not-stdout rule + the
       `^{tree}`-as-one-argv-element gotcha; §2 read-tree REPLACES the index + all-non-zero=error
       (mutation convention); §3 the run() 4-step shape + the err-nil-for-non-zero-exit invariant;
       §4 the two cross-cutting error tests (git-missing / ctx-cancelled); §5 reusable test fixtures +
       the independent-oracle idiom; §6 one-file-per-method test placement.
  critical: §1 (branch on code==128, NEVER on stdout — rev-parse prints the LITERAL arg on unborn;
            `^{tree}` MUST be a single argv element); §2 (ReadTree is a MUTATION → no 128 special-case,
            branch on code != 0, identical to AddAll).

# MUST READ — the FILE TO MODIFY: the Git interface + run() + the two exemplar methods (READ then EDIT)
- file: internal/git/git.go
  section: the `Git` interface (append the 2 signatures + doc comments, modeled on RevParseHEAD's +
           DiffTree's doc comments); `(*gitRunner).RevParseHEAD` (the EXACT 4-step shape + 128-as-non-error
           convention to MIRROR for RevParseTree); `(*gitRunner).run` (the helper BOTH methods call — its
           INVARIANT comment: non-zero git exit → (stdout, stderr, exitCode, nil), err nil); the doc comment
           on `(*gitRunner).AddAll` (the mutation convention — "treats ALL non-zero exits as errors" — to
           MIRROR for ReadTree).
  why: RevParseHEAD is the single best template for RevParseTree (same command family, same 128 convention,
       same read semantics). AddAll is the single best template for ReadTree (same mutation class, same
       all-non-zero=error branch, same `g.run(ctx, g.workDir, "add", "-A")` call shape). DiffTree's doc
       comment is the template for the interface doc comments (length, the "what landed"/role framing).
  pattern: copy RevParseHEAD's body LITERALLY and change (cmd → rev-parse <ref>^{tree}, return arity);
           copy AddAll's body LITERALLY and change (cmd → read-tree <tree>, return arity).
  gotcha: do NOT modify run() or runWithInput() — they are the consumed helpers. Do NOT edit the existing
          `// Method ownership (each implemented in its own later subtask):` comment block — it is a v1
          provenance map; the new doc comments are self-documenting.

# MUST READ — the test exemplars (the patterns to mirror in the 2 NEW test files)
- file: internal/git/revparse_test.go   (READ — pattern for RevParseTree tests)
  why: the EXACT shape RevParseTree tests copy: TestRevParseHEAD_UnbornRepo (initRepo, New(repo),
       RevParseHEAD, assert isUnborn + sha==""); TestRevParseHEAD_GitBinaryMissing
       (t.Setenv("PATH",""); New(t.TempDir()); assert err contains "git binary not found");
       TestRevParseHEAD_ContextCancelled (pre-cancel ctx; errors.Is(err, context.Canceled)).
       ALSO defines the package-level `makeEmptyCommit(t, dir, msg)` + `minGitEnv()` helpers RevParseTree
       tests reuse (establish a HEAD so the repo is "born").
  pattern: TestRevParseTree_BornRepoHEAD mirrors TestRevParseHEAD_BornRepo but asserts the returned SHA
           equals an INDEPENDENT writeTreeOf(t, repo) oracle (not just a regex).
- file: internal/git/addall_test.go   (READ — pattern for ReadTree tests)
  why: the EXACT shape ReadTree tests copy: TestAddAll_StagesModifiedAndUntracked (verify via INDEPENDENT
       `exec.Command("git", "-C", repo, "diff", "--cached", "--name-only")` — the independent-oracle
       idiom); TestAddAll_NotARepo (New(t.TempDir()) plain dir; assert err contains "failed");
       TestAddAll_GitBinaryMissing; TestAddAll_ContextCancelled. ReadTree's index-mutation is verified the
       SAME way: independent `git ls-files`, NOT a re-call of ReadTree.
  pattern: TestReadTree_LoadsTreeIntoIndex writes+commits a file, captures the tree, RESETS the index
           (git rm --cached), calls ReadTree, asserts git ls-files shows the file again.

# MUST READ — the shared test fixtures (reuse, do NOT redefine)
- file: internal/git/committree_test.go   (READ — fixture definitions)
  why: defines the package-level helpers `writeFile`, `stageFile`, `writeTreeOf`, `headSHA`,
       `setIdentityConfig`, `commitMessage` — ALL reusable from the new test files (same package git).
       `writeTreeOf(t, repo)` is the independent oracle for the RevParseTree SHA-equality assertion;
       `headSHA(t, repo)` gives the commit SHA to pass to RevParseTree(ctx, commitSHA).
  gotcha: these helpers are ALREADY defined — do NOT redefine them in the new files (duplicate-symbol
          compile error). Just call them.

# MUST READ — the design reference (signatures + roles in the pipeline)
- docfile: plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md
  section: "## Required New Git Methods" (the exact signatures this task implements, verbatim) and the
           §13.6.3 base-case note ("`tree[-1]` = original parent tree (`git rev-parse HEAD^{tree}`, or
           empty tree for unborn repo)") and §13.6.5 mid-chain note ("read-tree the appropriate base").
  why: confirms the signatures, the RevParseTree→tree[-1] role, and the ReadTree→arbiter role. NOTE: the
       doc lists 5 new methods (RevParseTree/TreeDiff/ReadTree/StatusPorcelain/WorkingTreeDiff); THIS task
       implements ONLY RevParseTree + ReadTree. The other 3 are sibling work items — do NOT implement them.
  critical: ReadTree's doc string in the architecture doc ("loads a tree into the index (for chain
            rebuild)") — that IS the full contract; no flags, no merge mode.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §13.6.3 (invariant 2 + base case) and §13.6.5 (arbiter mid-chain rebuild)
  why: §13.6.3 mandates tree-to-tree concept diffs requiring `tree[-1]` (the section names
       `git rev-parse HEAD^{tree}`); §13.6.5 mandates `read-tree` the base for mid-chain rebuild. This
       task implements both named commands at the interface boundary.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go              # MODIFY: append RevParseTree + ReadTree to the Git interface (doc comments) AND
                      #   implement both on *gitRunner. run()/runWithInput() UNCHANGED (consumed).
  binary.go           # UNCHANGED (P2.M1; not touched — no diff in these two methods).
  git_test.go         # READ: initRepo(t, dir) helper + TestRun_* (the run() contract tests).
  revparse_test.go    # READ: makeEmptyCommit + minGitEnv helpers + RevParseHEAD test patterns (template).
  addall_test.go      # READ: AddAll mutation test patterns + independent-oracle idiom (template).
  committree_test.go  # READ: writeFile/stageFile/writeTreeOf/headSHA/setIdentityConfig helpers (reuse).
  writetree_test.go   # READ: WriteTree mutation patterns (secondary template for ReadTree).
  (*_test.go)         # READ: other per-method test files (the one-file-per-method convention).
go.mod / go.sum       # UNCHANGED (stdlib only: context/fmt/strings already imported in git.go).
.golangci.yml         # READ: linter config (errcheck/gosimple/govet/ineffassign/staticcheck/unused).
```

### Desired Codebase tree with files to be added/modified

```bash
internal/git/git.go              # MODIFY — append to the `Git` interface:
                                 #     // RevParseTree returns the tree SHA of a commit-ish (ref = HEAD,
                                 #     // a branch, or a commit SHA) via `git rev-parse <ref>^{tree}`.
                                 #     // (doc: exit-128 → ("",nil) defensive; producer of tree[-1], §13.6.3)
                                 #     RevParseTree(ctx context.Context, ref string) (string, error)
                                 #     // ReadTree REPLACES the index with <tree>'s contents via
                                 #     // `git read-tree <tree>`. MUTATES THE INDEX; touches NO ref.
                                 #     // (doc: consumed by arbiter mid-chain rebuild §13.6.5; all
                                 #     // non-zero exits are errors — mutation convention like AddAll)
                                 #     ReadTree(ctx context.Context, tree string) error
                                 #   AND append the two *gitRunner method bodies (mirror RevParseHEAD +
                                 #   AddAll exactly). NO other change to git.go.
internal/git/revparsetree_test.go  # NEW — 5 tests (TestRevParseTree_BornRepoHEAD,
                                   #   TestRevParseTree_BornRepoCommitSHA, TestRevParseTree_UnbornRepoHEAD,
                                   #   TestRevParseTree_GitBinaryMissing, TestRevParseTree_ContextCancelled).
internal/git/readtree_test.go      # NEW — 6 tests (TestReadTree_LoadsTreeIntoIndex,
                                   #   TestReadTree_ReplacesIndex, TestReadTree_BadTree, TestReadTree_NotARepo,
                                   #   TestReadTree_GitBinaryMissing, TestReadTree_ContextCancelled).
# go.mod/go.sum UNCHANGED. binary.go/binary_test.go UNCHANGED. run()/runWithInput() UNCHANGED. 0 other edits.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (branch on exit CODE, not stdout — findings §1): on an unborn repo `git rev-parse HEAD^{tree}`
// exits 128 AND prints the LITERAL string "HEAD^{tree}\n" to stdout (verified — the SAME latent-bug shape
// as `git rev-parse HEAD` printing "HEAD\n", which is FINDING 1 of the RevParseHEAD research). A naive
// `if strings.TrimSpace(stdout) == ""` check would be WRONG: stdout is NON-empty on unborn. Branch on
// `code == 128`, exactly as RevParseHEAD does. This is non-negotiable.

// CRITICAL (`^{tree}` must be ONE argv element — findings §1): the peeling suffix attaches to <ref>. It
// MUST be passed as the single string `ref + "^{tree}"` to run(), NOT as two args. (run() takes
// `args ...string` and builds one exec.CommandContext argv; NO shell is involved per PRD §19, so `{`/`}`
// are not glob-expanded.) Correct: `g.run(ctx, g.workDir, "rev-parse", ref+"^{tree}")`.
// WRONG: `g.run(ctx, g.workDir, "rev-parse", ref, "^{tree}")` — git would treat "^{tree}" as a second
// positional and fail. This is the single most likely implementation bug.

// CRITICAL (RevParseTree is a READ → 128 = not-an-error; ReadTree is a MUTATION → all non-zero = error):
// the two methods look superficially similar (both thin run() wrappers) but have OPPOSITE exit-code
// conventions, and the convention is determined by READ-vs-MUTATION class, established in the file:
//   - RevParseTree (read, like RevParseHEAD/RecentMessages/CommitCount): special-case `code == 128` →
//     return ("", nil); `code != 0` (non-128) → error; `code == 0` → trimmed stdout.
//   - ReadTree (mutation, like AddAll/WriteTree/CommitTree): NO 128 special-case; `code != 0` → error;
//     `code == 0` → nil. AddAll's doc comment states the rule verbatim: "treats ALL non-zero exits as
//     errors (it is a mutation, structurally identical to WriteTree/CommitTree)". ReadTree is identical.
// DO NOT give RevParseTree the mutation convention or ReadTree the read convention.

// GOTCHA (run() returns err==nil for non-zero git exits — findings §3): run()'s INVARIANT is that a
// non-zero git exit is returned as (stdout, stderr, exitCode, nil) — err is nil, the exit code is the
// signal. Only infrastructural failures (LookPath miss / context cancel / start-I/O) return err != nil
// (with exitCode -1). So BOTH implementations do `if err != nil { return <zero>, err }` FIRST (this
// catches context.Canceled and git-binary-missing, propagated UNWRAPPED so errors.Is works), THEN branch
// on `code`. Copy RevParseHEAD/AddAll's bodies — they already encode this.

// GOTCHA (do NOT modify run() or runWithInput()): both are the consumed helpers. Neither new method
// reads stdin (rev-parse and read-tree take no stdin), so runWithInput() is NOT used at all — only run().
// Editing run() would be a scope violation AND would change every other method's behavior.

// GOTCHA (ReadTree REPLACES the index — findings §2, verified): `git read-tree <tree>` (no -m/--merge)
// REPLACES the entire index with <tree>'s contents; it does NOT merge. Loading HEAD~1^{tree} into an
// index holding a.txt+b.txt leaves ONLY a.txt. This is the behavior the arbiter's mid-chain rebuild
// relies on (read-tree base → fold leftovers via git add → write-tree). The method takes NO flags in
// this contract — plain `git read-tree <tree>`. Do NOT add -m/-u/--prefix.

// GOTCHA (the `// Method ownership` comment block in git.go): it is a v1 provenance map listing
// RevParseHEAD/WriteTree/CommitTree/etc with their subtask IDs. It does NOT list the v2 methods and it
// is NOT the source of truth for the interface (the interface signature block is). Do NOT edit it — the
// new doc comments on RevParseTree/ReadTree are self-documenting. Editing it risks a merge conflict with
// the parallel S2 (TreeDiff) work item which adds its own v2 method.

// GOTCHA (Go interface extension is backward-compatible): adding methods to the `Git` interface does NOT
// break existing callers or the single production implementation (`*gitRunner`, which you are also
// extending). There are no other implementors of `Git` in the codebase (verified: `grep -rn "Git interface"
// internal/` — New() returns *gitRunner; tests use New() or &gitRunner{} directly). So adding 2 methods
// is safe and additive.

// GOTCHA (one-file-per-method test convention — findings §6): the package uses one test file per Git
// method. Put RevParseTree tests in a NEW `revparsetree_test.go` (NOT in `revparse_test.go`, which owns
// RevParseHEAD + the shared makeEmptyCommit/minGitEnv helpers — keep it untouched to avoid conflicts)
// and ReadTree tests in a NEW `readtree_test.go`. Test function names MUST be distinct from all existing
// ones (prefix TestRevParseTree_ / TestReadTree_). This also avoids a file-level merge conflict with the
// parallel S2 (TreeDiff → treediff_test.go).

// GOTCHA (independent-oracle assertions — findings §5): existing tests verify a method's output via an
// INDEPENDENT git invocation (exec.Command), NOT by re-calling the method under test. RevParseTree tests
// compare the returned SHA against `writeTreeOf(t, repo)` (an independent `git write-tree`). ReadTree
// tests verify index mutation via an independent `git ls-files` (like addall_test.go's
// `git diff --cached --name-only` oracle). Do NOT assert "ReadTree then WriteTree == tree" (that uses the
// method under test as the oracle — circular).

// GOTCHA (no new dependencies): only stdlib already imported in git.go (context/fmt/strings) is used.
// The test files reuse existing imports (context/errors/os/os/exec/regexp/strings/testing + the package's
// own helpers). go.mod/go.sum MUST stay byte-identical.
```

## Implementation Blueprint

### Data models and structure

No new TYPES. No new OPTIONS structs (neither method takes options — `RevParseTree` takes a `ref`
string, `ReadTree` takes a `tree` string). The only structural change is two new method signatures on
the `Git` interface and two corresponding `(*gitRunner)` method bodies. The implementations consume the
existing helpers unchanged:

```go
// from internal/git/git.go (CONSUME — do not modify):
//   func (g *gitRunner) run(ctx context.Context, repo string, args ...string) (stdout, stderr string, exitCode int, err error)
// run()'s invariant: non-zero git exit → (stdout, stderr, exitCode, nil); err != nil ⟺ infrastructural
// failure only (LookPath miss / context cancel / start-I/O), with exitCode == -1.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go — append RevParseTree + ReadTree to the `Git` interface
  - LOCATE the end of the `Git` interface (the closing `}` after the `StagedFileCount` method, ~line 95).
  - INSERT, before the closing brace, two new method declarations WITH doc comments (see "Implementation
    Patterns" for the exact text). Place RevParseTree near RevParseHEAD (read family) and ReadTree near
    AddAll (mutation family) if you want grouping — OR just append both at the end (simpler, no diff
    churn in the middle). APPEND-AT-END is preferred (minimal diff, no risk of disturbing the existing
    method ordering).
  - NAMING: `RevParseTree(ctx context.Context, ref string) (string, error)` and
    `ReadTree(ctx context.Context, tree string) error` (EXACT, from the architecture doc + work item).
  - DOC COMMENTS must name, for each: the exact git command; the read-vs-mutation class; the exit-code
    convention (RevParseTree: 128 → ("",nil) defensive, callers gate on isUnborn; ReadTree: all non-zero
    = error, mutation convention like AddAll); the §13.6 role (RevParseTree → tree[-1] for concept diffs;
    ReadTree → arbiter mid-chain rebuild).
  - GOTCHA: do NOT edit the `// Method ownership` comment block — it is a v1 provenance map.
  - PLACEMENT: internal/git/git.go, inside `type Git interface { … }`.

Task 2: MODIFY internal/git/git.go — implement (*gitRunner).RevParseTree
  - APPEND a new method `func (g *gitRunner) RevParseTree(ctx context.Context, ref string) (string, error)`
    after the existing `(*gitRunner).RevParseHEAD` method (co-locate with its read-family sibling) OR at
    the end of the file (after StagedFileCount). Co-location near RevParseHEAD is preferred (readability).
  - BODY (mirror RevParseHEAD EXACTLY, see "Implementation Patterns"):
      (a) `stdout, stderr, code, err := g.run(ctx, g.workDir, "rev-parse", ref+"^{tree}")` — the `^{tree}`
          suffix is ONE argv element (CRITICAL gotcha).
      (b) `if err != nil { return "", err }` — infrastructural failure (context.Canceled / git-missing),
          propagated UNWRAPPED.
      (c) `if code == 128 { return "", nil }` — unborn repo / unresolvable ref → defensive empty
          (callers gate on isUnborn). Branch on CODE, not stdout.
      (d) `if code != 0 { return "", fmt.Errorf("git rev-parse %s^{tree}: failed (exit %d): %s", ref,
          code, strings.TrimSpace(stderr)) }` — any other non-zero → error.
      (e) `return strings.TrimSpace(stdout), nil`.
  - PATTERN: copy `(*gitRunner).RevParseHEAD`'s body and change the command + the return arity. The
    4-step shape (err-check → 128 → non-zero → success) is IDENTICAL.
  - GOTCHA: do NOT special-case `ref == "HEAD"` — the method is generic over any commit-ish; the caller
    chooses HEAD or a SHA. The 128 branch covers both unborn-HEAD and bad-SHA uniformly (the contract's
    "defensive" choice).
  - PLACEMENT: internal/git/git.go, a new `(*gitRunner)` method.

Task 3: MODIFY internal/git/git.go — implement (*gitRunner).ReadTree
  - APPEND a new method `func (g *gitRunner) ReadTree(ctx context.Context, tree string) error` near
    `(*gitRunner).AddAll` (mutation family) OR at the end of the file. Co-location near AddAll preferred.
  - BODY (mirror AddAll EXACTLY, see "Implementation Patterns"):
      (a) `_, stderr, code, err := g.run(ctx, g.workDir, "read-tree", tree)` — stdout is unused (read-tree
          prints nothing on success); assign to `_`.
      (b) `if err != nil { return err }` — infrastructural failure, propagated UNWRAPPED.
      (c) `if code != 0 { return fmt.Errorf("git read-tree: failed (exit %d): %s", code,
          strings.TrimSpace(stderr)) }` — ALL non-zero exits are errors (mutation convention; NO 128
          special-case).
      (d) `return nil`.
  - PATTERN: copy `(*gitRunner).AddAll`'s body and change the command + return arity. The 3-step shape
    (err-check → non-zero → success) is IDENTICAL (AddAll has no 128 branch — neither does ReadTree).
  - GOTCHA: do NOT add a 128 branch (ReadTree is a mutation, not a read). Do NOT add -m/-u/--prefix flags.
  - GOTCHA: assign stdout to `_` (read-tree prints nothing on success; referencing it would be an unused-
    var lint error — mirrors AddAll which uses `_, stderr, code, err :=`).
  - PLACEMENT: internal/git/git.go, a new `(*gitRunner)` method.

Task 4: CREATE internal/git/revparsetree_test.go — 5 tests for RevParseTree
  - IMPORTS: context, errors, regexp, strings, testing (mirror revparse_test.go). The package is `git`
    (same package — helpers initRepo/makeEmptyCommit/writeFile/stageFile/writeTreeOf/headSHA are visible).
  - ADD TestRevParseTree_BornRepoHEAD:
      initRepo(t, repo); writeFile(t, repo, "a.txt", "hello\n"); stageFile(t, repo, "a.txt");
      makeEmptyCommit(t, repo, "init");  // establishes HEAD so the repo is born
      g := New(repo);
      treeSHA, err := g.RevParseTree(context.Background(), "HEAD");
      assert err == nil;
      assert treeSHA matches `^[0-9a-f]{40,64}$`;
      // INDEPENDENT ORACLE: write-tree over the same staged+committed index yields the SAME tree.
      want := writeTreeOf(t, repo);  // independent git write-tree
      assert treeSHA == want.  // ^{tree} peeling == git's own tree resolution
  - ADD TestRevParseTree_BornRepoCommitSHA:
      (same setup as above); commitSHA := headSHA(t, repo);  // independent git rev-parse HEAD
      treeSHA, err := g.RevParseTree(context.Background(), commitSHA);  // pass the COMMIT SHA, not "HEAD"
      assert err == nil;
      assert treeSHA == writeTreeOf(t, repo);  // same tree as the HEAD test (HEAD's commit IS this SHA)
  - ADD TestRevParseTree_UnbornRepoHEAD:
      initRepo(t, repo);  // zero commits — unborn
      g := New(repo);
      treeSHA, err := g.RevParseTree(context.Background(), "HEAD");
      assert err == nil;       // 128 is NOT an error (defensive)
      assert treeSHA == "";    // empty return (NOT the literal "HEAD^{tree}" string git prints to stdout)
  - ADD TestRevParseTree_GitBinaryMissing:
      t.Setenv("PATH", "");  g := New(t.TempDir());
      treeSHA, err := g.RevParseTree(context.Background(), "HEAD");
      assert err != nil && strings.Contains(err.Error(), "git binary not found");  assert treeSHA == "";
  - ADD TestRevParseTree_ContextCancelled:
      ctx, cancel := context.WithCancel(context.Background()); cancel();
      g := New(t.TempDir());
      treeSHA, err := g.RevParseTree(ctx, "HEAD");
      assert errors.Is(err, context.Canceled);  assert treeSHA == "";
  - PATTERN: copy TestRevParseHEAD_BornRepo/_UnbornRepo/_GitBinaryMissing/_ContextCancelled and adapt.
  - GOTCHA: the unborn test MUST assert `treeSHA == ""` (empty) — git prints "HEAD^{tree}" to stdout on
    unborn, so a correct implementation (branching on code==128) returns ""; a BUGGY implementation
    (branching on stdout) would return "HEAD^{tree}". This test catches that bug.
  - GOTCHA: do NOT redefine makeEmptyCommit/minGitEnv/writeFile/stageFile/writeTreeOf/headSHA — they are
    already package-level (redefining = compile error).
  - PLACEMENT: NEW file internal/git/revparsetree_test.go.

Task 5: CREATE internal/git/readtree_test.go — 6 tests for ReadTree
  - IMPORTS: context, errors, os, os/exec, strings, testing (mirror addall_test.go).
  - ADD TestReadTree_LoadsTreeIntoIndex:
      initRepo(t, repo); writeFile(t, repo, "a.txt", "hello\n"); stageFile(t, repo, "a.txt");
      makeEmptyCommit(t, repo, "init");  tree := writeTreeOf(t, repo);  // the tree holding a.txt
      // reset the index so the load is observable
      exec.Command("git", "-C", repo, "rm", "--cached", "-q", "a.txt").Run();  // remove a.txt from index
      // INDEPENDENT oracle BEFORE: git ls-files shows nothing
      g := New(repo);
      err := g.ReadTree(context.Background(), tree);
      assert err == nil;
      // INDEPENDENT oracle AFTER: git ls-files shows a.txt again (read-tree loaded it)
      out, _ := exec.Command("git", "-C", repo, "ls-files").Output();
      assert strings.TrimSpace(string(out)) == "a.txt";
  - ADD TestReadTree_ReplacesIndex:
      initRepo(t, repo);
      writeFile(t, repo, "a.txt", "a\n"); stageFile(t, repo, "a.txt"); makeEmptyCommit(t, repo, "one");
      writeFile(t, repo, "b.txt", "b\n"); stageFile(t, repo, "b.txt"); makeEmptyCommit(t, repo, "two");
      olderTree := writeTreeOf(t, repo);  // wait — write-tree reflects the INDEX (a+b), not HEAD~1.
      // CORRECT: get the tree of HEAD~1 (holds only a.txt) via an independent git rev-parse HEAD~1^{tree}
      olderTree := strings.TrimSpace(string(execCmd(t, repo, "rev-parse", "HEAD~1^{tree}")));
      g := New(repo);
      err := g.ReadTree(context.Background(), olderTree);  // REPLACES index with the a.txt-only tree
      assert err == nil;
      out, _ := exec.Command("git", "-C", repo, "ls-files").Output();
      got := strings.Fields(string(out));
      // index now holds ONLY a.txt (b.txt dropped) — proves REPLACES, not merge
      assert len(got)==1 && got[0]=="a.txt";
      // (use a small execCmd helper OR inline exec.Command with t.Helper + Fatalf on err)
  - ADD TestReadTree_BadTree:
      initRepo(t, repo);  g := New(repo);
      err := g.ReadTree(context.Background(), "0000000000000000000000000000000000000000");
      assert err != nil && strings.Contains(err.Error(), "git read-tree: failed");  // exit 128 → error
  - ADD TestReadTree_NotARepo:
      g := New(t.TempDir());  // plain dir, NOT a repo
      err := g.ReadTree(context.Background(), "2e81171448eb9f2ee3821e3d447aa6b2fe3ddba1");
      assert err != nil && strings.Contains(err.Error(), "git read-tree: failed");  // exit 128 → error
  - ADD TestReadTree_GitBinaryMissing:
      t.Setenv("PATH", "");  g := New(t.TempDir());
      err := g.ReadTree(context.Background(), "tree");
      assert err != nil && strings.Contains(err.Error(), "git binary not found");
  - ADD TestReadTree_ContextCancelled:
      ctx, cancel := context.WithCancel(context.Background()); cancel();
      g := New(t.TempDir());
      err := g.ReadTree(ctx, "tree");
      assert errors.Is(err, context.Canceled);
  - PATTERN: copy TestAddAll_* (the mutation + independent-oracle idiom) and adapt. For
    TestReadTree_ReplacesIndex, capture the older tree via an INDEPENDENT `git rev-parse HEAD~1^{tree}`
    (NOT via the method under test — that would be circular); a tiny t.Helper-wrapped execCmd or inline
    exec.Command both work (addall_test.go inlines exec.Command with .Output()).
  - GOTCHA: TestReadTree_ReplacesIndex is the KEY test — it proves read-tree REPLACES (not merges) the
    index, which is the property the arbiter's mid-chain rebuild depends on. Do NOT skip it.
  - GOTCHA: for the index-oracle assertions, use `git ls-files` (lists the index) — it directly reflects
    read-tree's mutation. `git diff --cached --name-only` (addall_test.go's oracle) also works but
    compares against HEAD; `ls-files` is the cleaner index mirror for read-tree.
  - PLACEMENT: NEW file internal/git/readtree_test.go.

Task 6: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/git/git.go internal/git/revparsetree_test.go internal/git/readtree_test.go`
  - `go build ./...`   (whole module compiles — the interface + 2 impls + 2 test files)
  - `go vet ./...`
  - `golangci-lint run ./...`   (errcheck/gosimple/govet/ineffassign/staticcheck/unused)
  - `go test -race ./internal/git/ -run "TestRevParseTree|TestReadTree" -v`   (all 11 new tests)
  - `go test -race ./internal/git/`   (the WHOLE git package — existing tests still pass)
  - `go test ./...`   (FULL regression — no other package breaks; the 2 new interface methods are additive)
  - `git diff --exit-code go.mod go.sum` ⇒ empty.
  - `git status --short` ⇒ EXACTLY: `M internal/git/git.go` + `?? internal/git/revparsetree_test.go` +
    `?? internal/git/readtree_test.go` (3 entries); binary.go/binary_test.go/run()/runWithInput() UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// === Task 1: the interface additions (append inside `type Git interface { … }`) ===

// (append near RevParseHEAD — read family — OR at the end of the interface block)

	// RevParseTree returns the tree SHA of a commit-ish: ref is "HEAD", a branch name, or a commit SHA.
	// It runs `git rev-parse <ref>^{tree}`, where the `^{tree}` suffix peels the commit-ish to its tree
	// object (the tree a commit points at). It is the producer of tree[-1] — the original-parent tree
	// that anchors the multi-commit concept-diff loop (PRD §13.6.3: "`tree[-1]` is the original parent
	// tree (`git rev-parse HEAD^{tree}`, or the empty tree for an unborn repo)"; invariant 2 mandates
	// tree-to-tree concept diffs, never index-vs-HEAD).
	//
	// On an unborn repo with ref="HEAD", or on any unresolvable ref, git exits 128; RevParseTree returns
	// ("", nil) defensively (NOT an error) — callers gate on RevParseHEAD's isUnborn before calling, so an
	// empty return is the correct non-error signal for the unborn/empty-tree base case. This 128-as-non-error
	// convention is identical to RevParseHEAD / RecentMessages / RecentSubjects / CommitCount. Branch on the
	// exit code, NOT on stdout emptiness: git prints the literal argument string to stdout on exit 128.
	RevParseTree(ctx context.Context, ref string) (tree string, err error)

// (append near AddAll — mutation family — OR at the end of the interface block)

	// ReadTree REPLACES the index with the contents of <tree> via `git read-tree <tree>` (the default,
	// no -m/--merge form). It MUTATES THE INDEX (writes .git/index) but touches NEITHER HEAD NOR any ref
	// (PRD §18.1: refs move ONLY at UpdateRefCAS). It is consumed ONLY by the arbiter's mid-chain chain
	// rebuild (PRD §13.6.5: "for each j, read-tree the appropriate base, fold the leftovers in at j==i,
	// write-tree, commit-tree against the rebuilt parent, update-ref"). Because it is a mutation, EVERY
	// non-zero exit (128 = bad/unresolvable tree SHA, not-a-repo, corrupt object) is a real error — the
	// SAME convention as AddAll / WriteTree / CommitTree (mutations never special-case 128 as "unborn").
	ReadTree(ctx context.Context, tree string) error


// === Task 2: (*gitRunner).RevParseTree — MIRROR RevParseHEAD EXACTLY ===
// (the ONLY differences from RevParseHEAD: the command + the return arity)

// RevParseTree returns the tree SHA of a commit-ish (ref = "HEAD", a branch, or a commit SHA) via
// `git rev-parse <ref>^{tree}`. The `^{tree}` suffix is passed as ONE argv element (run() takes
// args ...string and builds one exec.CommandContext argv; no shell — PRD §19). On an unborn repo with
// ref="HEAD" (or an unresolvable ref) git exits 128; RevParseTree returns ("", nil) defensively
// (callers gate on RevParseHEAD's isUnborn — the empty return is the unborn/empty-tree base case). This
// mirrors RevParseHEAD's 128 convention exactly: branch on the exit CODE, NOT on stdout (git prints the
// literal argument to stdout on exit 128). Producer of tree[-1] for the multi-commit concept-diff loop
// (PRD §13.6.3).
func (g *gitRunner) RevParseTree(ctx context.Context, ref string) (string, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "rev-parse", ref+"^{tree}")
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
	}
	if code == 128 {
		return "", nil // unborn repo / unresolvable ref — defensive (callers gate on isUnborn). Branch on CODE.
	}
	if code != 0 {
		return "", fmt.Errorf("git rev-parse %s^{tree}: failed (exit %d): %s", ref, code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

// === Task 3: (*gitRunner).ReadTree — MIRROR AddAll EXACTLY ===
// (the ONLY differences from AddAll: the command + the return arity; NO 128 branch — it is a mutation)

// ReadTree REPLACES the index with <tree>'s contents via `git read-tree <tree>`. It MUTATES THE INDEX
// (writes .git/index) but touches NEITHER HEAD NOR any ref — refs move ONLY at UpdateRefCAS (PRD §18.1).
// Consumed ONLY by the arbiter's mid-chain chain rebuild (PRD §13.6.5). Because it is a mutation, EVERY
// non-zero exit (128 = bad/unresolvable tree SHA, not-a-repo, corrupt) is a real error — the mutation
// convention shared with AddAll / WriteTree / CommitTree (no 128-as-non-error special-case). read-tree
// prints nothing to stdout on success, so stdout is discarded.
func (g *gitRunner) ReadTree(ctx context.Context, tree string) error {
	_, stderr, code, err := g.run(ctx, g.workDir, "read-tree", tree) // stdout unused (read-tree prints nothing)
	if err != nil {
		return err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
	}
	if code != 0 {
		// ALL non-zero exits are errors (mutation convention — like AddAll). NO 128 special-case.
		return fmt.Errorf("git read-tree: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return nil
}

// PATTERN NOTE (why these mirror existing methods so closely): the package's git methods are deliberately
// uniform — every one is a thin run()/runWithInput() wrapper with a per-command exit-code interpretation.
// RevParseTree IS RevParseHEAD with a different command; ReadTree IS AddAll with a different command.
// Copying the sibling's body (and adjusting command + arity) is the INTENDED authoring pattern, NOT
// duplication-for-its-own-sake: it guarantees the run() contract, the error-wrapping shape, and the
// infrastructural-failure propagation are byte-consistent across the file.
```

### Integration Points

```yaml
INTERNAL/GIT (internal/git/git.go — MODIFY):
  - Git interface: "+ RevParseTree(ctx, ref string) (string, error)" + "+ ReadTree(ctx, tree string)
          error", each with a doc comment (command, read-vs-mutation class, exit-code convention, §13.6
          role). Appended (minimal diff).
  - *gitRunner: "+ func (g *gitRunner) RevParseTree(...)" (mirrors RevParseHEAD; 4-step shape; 128→("",nil))
          and "+ func (g *gitRunner) ReadTree(...)" (mirrors AddAll; 3-step shape; all-non-zero=error).
  - run()/runWithInput(): UNCHANGED (consumed). Neither new method uses runWithInput (no stdin).

INTERNAL/GIT TESTS (2 NEW files):
  - revparsetree_test.go: 5 tests (born-HEAD==write-tree oracle, born-commitSHA, unborn→("",nil),
          git-missing, ctx-cancelled). Reuses initRepo/makeEmptyCommit/writeFile/stageFile/writeTreeOf/headSHA.
  - readtree_test.go: 6 tests (loads-into-index, replaces-index, bad-tree, not-a-repo, git-missing,
          ctx-cancelled). Index-mutation verified via INDEPENDENT git ls-files.

DOWNSTREAM CONSUMERS (NOT this task — wiring is P3.x):
  - decompose orchestrator (P3.M2.T4): tree[-1] = RevParseTree(ctx, "HEAD") for the concept-diff loop.
  - arbiter mid-chain rebuild (P3.M3.T2): ReadTree(base) → fold leftovers → write-tree → commit-tree.
  - No caller references these methods yet; the interface additions are backward-compatible.

FROZEN/LEAVE (do NOT edit):
  - run() / runWithInput() (consumed helpers).
  - internal/git/binary.go + binary_test.go (P2.M1; not touched).
  - The `// Method ownership` comment block in git.go (v1 provenance map).
  - go.mod / go.sum (stdlib only; unchanged).
  - TreeDiff / StatusPorcelain / WorkingTreeDiff (sibling work items S2 / P2.M2.T2).
  - Any file outside internal/git/ (no caller wiring in this task).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
gofmt -w internal/git/git.go internal/git/revparsetree_test.go internal/git/readtree_test.go
go build ./...                 # whole module compiles (interface + 2 impls + 2 test files)
go vet ./...
golangci-lint run ./...        # errcheck/gosimple/govet/ineffassign/staticcheck/unused
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: build OK; go vet clean; golangci-lint clean; go.mod/go.sum unchanged.
# NOTE: run `go vet ./...` + `golangci-lint run ./...` (whole module), not just ./internal/git/, to catch
# any transitive issue from the interface change (there should be none — additions are backward-compatible).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 11 new tests, verbose (race detector on — mirrors the existing test convention):
go test -race ./internal/git/ -run "TestRevParseTree|TestReadTree" -v
# Expected: all 11 green. Specifically:
#   TestRevParseTree_BornRepoHEAD        ⇒ treeSHA == writeTreeOf(t, repo) (independent oracle)
#   TestRevParseTree_BornRepoCommitSHA   ⇒ treeSHA == writeTreeOf(t, repo) (commit-SHA peeling)
#   TestRevParseTree_UnbornRepoHEAD      ⇒ ("", nil) — NOT "HEAD^{tree}" (the exit-128, not-stdout rule)
#   TestRevParseTree_GitBinaryMissing    ⇒ err contains "git binary not found"
#   TestRevParseTree_ContextCancelled    ⇒ errors.Is(err, context.Canceled)
#   TestReadTree_LoadsTreeIntoIndex      ⇒ git ls-files shows a.txt after the load
#   TestReadTree_ReplacesIndex           ⇒ git ls-files shows ONLY a.txt (b.txt dropped) — REPLACES not merge
#   TestReadTree_BadTree                 ⇒ err contains "git read-tree: failed" (exit 128 → error)
#   TestReadTree_NotARepo                ⇒ err contains "git read-tree: failed" (exit 128 → error)
#   TestReadTree_GitBinaryMissing        ⇒ err contains "git binary not found"
#   TestReadTree_ContextCancelled        ⇒ errors.Is(err, context.Canceled)

# The WHOLE git package (existing tests still pass — the 2 new interface methods are additive):
go test -race ./internal/git/

# Full regression (no other package breaks):
go test ./...
# Expected: GREEN across all packages. The interface gains 2 methods; *gitRunner gains 2 impls; no
# existing caller or implementor is affected (verified: New() is the only Git constructor; no other type
# implements Git).
```

### Level 3: Integration / Behavioral Proof (empirical confirmation of the git wire format)

```bash
# Reproduce empirically: rev-parse ^{tree} on born vs unborn, and read-tree REPLACES the index.
T=$(mktemp -d); cd "$T"; git init -q .; git config user.name t; git config user.email t@t.co

echo "=== born repo: rev-parse HEAD^{tree} == write-tree over the same index ==="
echo "hello" > a.txt; git add a.txt; git commit -q -m init
echo "rev-parse HEAD^{tree}: $(git rev-parse 'HEAD^{tree}')"
echo "write-tree (index):    $(git write-tree)"
# Expected: the two SHAs are IDENTICAL (proves RevParseTree's ^{tree} peeling == git's tree resolution).

echo ""
echo "=== unborn repo: rev-parse HEAD^{tree} exits 128, prints the LITERAL arg ==="
cd "$(mktemp -d)"; git init -q .
git rev-parse 'HEAD^{tree}'; echo "EXIT=$?"
# Expected: EXIT=128; stdout = the literal "HEAD^{tree}" — so the impl MUST branch on code==128, not stdout.

echo ""
echo "=== read-tree REPLACES the index (load an OLDER tree) ==="
cd "$T"
echo "b" > b.txt; git add b.txt; git commit -q -m two      # index/HEAD now: a.txt + b.txt
OLDER=$(git rev-parse 'HEAD~1^{tree}')                      # the tree holding ONLY a.txt
echo "index BEFORE read-tree: $(git ls-files | tr '\n' ' ')"  # a.txt b.txt
git read-tree "$OLDER"; echo "EXIT=$?"
echo "index AFTER read-tree:  $(git ls-files | tr '\n' ' ')"  # a.txt  (b.txt DROPPED — REPLACES not merge)
# Expected: AFTER shows only a.txt — proves read-tree replaces the index (the arbiter's mid-chain property).

cd /; rm -rf "$T" "$(mktemp -d)"
# (The Go tests in Level 2 assert exactly this end-to-end through RevParseTree + ReadTree.)
```

### Level 4: Regression & Audit

```bash
cd /home/dustin/projects/stagecoach
go build ./...                 # whole module compiles
go test ./...                  # FULL regression
git status --short             # Expected: EXACTLY:
                               #    M internal/git/git.go
                               #    ?? internal/git/revparsetree_test.go
                               #    ?? internal/git/readtree_test.go
                               # 3 entries total. binary.go/binary_test.go + go.mod/go.sum UNCHANGED.
git diff --stat internal/git/binary.go internal/git/binary_test.go   # Expected: empty (P2.M1 files untouched)
git diff --exit-code go.mod go.sum                                    # Expected: empty
grep -n "func (g \*gitRunner) run\b\|func (g \*gitRunner) runWithInput" internal/git/git.go   # Expected: the
                               # 2 helper signatures UNCHANGED (count == 2; no new helpers, no edits to them).
# Expected: build + full test green; only 1 modified + 2 new files; run()/runWithInput() untouched;
# binary.go untouched; go.mod/go.sum unchanged.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l internal/ pkg/` empty; `go build ./...` OK; `go vet ./...` clean; `golangci-lint
      run ./...` clean.
- [ ] Level 2: `go test -race ./internal/git/ -run "TestRevParseTree|TestReadTree" -v` green — 11 new tests.
- [ ] Level 2: `go test -race ./internal/git/` green (existing tests unaffected).
- [ ] Level 3: empirical proof — born `rev-parse HEAD^{tree}` == `write-tree`; unborn exits 128; read-tree
      REPLACES the index (older tree → only its files).
- [ ] Level 4: `go build ./...` + `go test ./...` green; `git status` = 1 modified + 2 new files;
      binary.go/binary_test.go + run()/runWithInput() + go.mod/go.sum UNCHANGED.

### Feature Validation

- [ ] `RevParseTree(ctx, ref string) (string, error)` declared on `Git` with a doc comment (command,
      read class, 128→("",nil) defensive, §13.6.3 tree[-1] role).
- [ ] `ReadTree(ctx, tree string) error` declared on `Git` with a doc comment (command, mutation class,
      all-non-zero=error, §13.6.5 arbiter role).
- [ ] `RevParseTree` on a born repo returns the SAME SHA as an independent `write-tree` oracle (HEAD and
      commit-SHA both).
- [ ] `RevParseTree` on an unborn repo returns `("", nil)` — NOT an error, NOT the literal stdout string.
- [ ] `ReadTree` loads a tree into the index (verified via independent `git ls-files`).
- [ ] `ReadTree` REPLACES the index (loading an older tree drops the newer files — the arbiter property).
- [ ] `ReadTree` returns a non-nil error on a bad SHA, a non-repo dir, AND any non-zero exit (mutation
      convention — no 128 special-case).
- [ ] Both methods propagate `context.Canceled` (errors.Is) and `"git binary not found"` unchanged.

### Code Quality Validation

- [ ] `RevParseTree` mirrors RevParseHEAD's body (4-step shape: err-check → 128 → non-zero → success);
      `ReadTree` mirrors AddAll's body (3-step shape: err-check → non-zero → success).
- [ ] The `^{tree}` suffix is ONE argv element (`ref+"^{tree}"`), NOT two args.
- [ ] RevParseTree branches on `code == 128`; ReadTree branches on `code != 0` (read-vs-mutation split).
- [ ] Both methods propagate infrastructural failures UNWRAPPED (`if err != nil { return <zero>, err }`).
- [ ] run() / runWithInput() are UNCHANGED; binary.go/binary_test.go UNCHANGED; go.mod/go.sum UNCHANGED.
- [ ] Tests use INDEPENDENT oracles (writeTreeOf / headSHA / git ls-files via exec.Command), NOT re-calls
      of the method under test.
- [ ] Test files are NEW (revparsetree_test.go, readtree_test.go); function names distinct (TestRevParseTree_*
      / TestReadTree_*); existing helpers reused (not redefined).

### Documentation & Deployment

- [ ] Both interface doc comments name the exact git command, the read-vs-mutation class, the exit-code
      convention, and the §13.6 role.
- [ ] Both `(*gitRunner)` method bodies have a doc comment reiterating the command + convention + role.
- [ ] Implementation summary records: the 2 interface methods + 2 impls, the RevParseHEAD/AddAll mirroring,
      the `^{tree}`-one-argv gotcha, and the read-vs-mutation exit-code split.

---

## Anti-Patterns to Avoid

- ❌ **Don't branch RevParseTree on stdout emptiness.** On an unborn repo `git rev-parse HEAD^{tree}`
  exits 128 AND prints the literal string `HEAD^{tree}\n` to stdout (verified — the same latent-bug shape
  as `git rev-parse HEAD` printing `HEAD\n`). A `if strings.TrimSpace(stdout) == ""` check would be WRONG.
  Branch on `code == 128`, exactly as RevParseHEAD does.
- ❌ **Don't pass `^{tree}` as a separate argv element.** The peeling suffix attaches to `<ref>`; it MUST
  be the single string `ref+"^{tree}"`. `g.run(ctx, g.workDir, "rev-parse", ref, "^{tree}")` would make
  git treat `^{tree}` as a second positional and fail. Correct: `..., "rev-parse", ref+"^{tree}")`.
- ❌ **Don't give ReadTree a 128 special-case.** ReadTree is a MUTATION (like AddAll/WriteTree/CommitTree),
  not a read. Mutations treat EVERY non-zero exit as an error — there is no "unborn is not an error"
  escape hatch for them. Branch on `code != 0`. (Conversely, don't give RevParseTree the mutation
  convention — it IS a read and MUST special-case 128 → ("", nil).)
- ❌ **Don't modify `run()` or `runWithInput()`.** They are the consumed helpers every method uses. Neither
  new method needs stdin (rev-parse and read-tree take none), so runWithInput() is not even called. Editing
  either helper is a scope violation and would change every other method's behavior.
- ❌ **Don't add flags to ReadTree.** The contract is plain `git read-tree <tree>` — no `-m`/`-u`/`--prefix`.
  The arbiter's "fold the leftovers in" is done by a SEPARATE `git add` in the P3.M3.T2 orchestrator, NOT
  by a read-tree merge mode here. Adding `-m` would turn a replace into a merge and break the rebuild.
- ❌ **Don't re-call the method under test as the test oracle.** Use an INDEPENDENT git invocation:
  `writeTreeOf(t, repo)` (independent `git write-tree`) for RevParseTree's SHA-equality assertion; an
  `exec.Command("git", "-C", repo, "ls-files")` for ReadTree's index-mutation assertion. Re-calling the
  method is circular (it would pass even if the method were a no-op stub).
- ❌ **Don't edit `binary.go` / `binary_test.go`.** They are P2.M1's (binary filtering). RevParseTree and
  ReadTree do not diff, so there is no binary handling here. TreeDiff (S2) is the sibling that reuses
  binary.go. Editing them = scope violation + a conflict with P2.M1.
- ❌ **Don't edit the `// Method ownership` comment block.** It is a v1 provenance map. The new doc comments
  on RevParseTree/ReadTree are self-documenting. Editing the block risks a merge conflict with the parallel
  S2 (TreeDiff) work item.
- ❌ **Don't add the methods to an existing test file.** The package uses one-file-per-method. Put
  RevParseTree tests in a NEW `revparsetree_test.go` (NOT `revparse_test.go`, which owns RevParseHEAD +
  shared helpers) and ReadTree tests in a NEW `readtree_test.go`. New files avoid both duplicate-helper
  compile errors and file-level merge conflicts with S2's `treediff_test.go`.
- ❌ **Don't add dependencies or touch go.mod/go.sum.** Only stdlib already imported in git.go
  (context/fmt/strings) is used. Any new dep is out of scope. Editing binary.go, run(), or any file outside
  internal/git/ is out of scope.
- ❌ **Don't skip the REPLACES-index test.** `TestReadTree_ReplacesIndex` is the single most important
  ReadTree test: it proves read-tree REPLACES (not merges) the index — the exact property the arbiter's
  mid-chain rebuild (§13.6.5) depends on. Without it, a `-m` regression could slip through silently.
