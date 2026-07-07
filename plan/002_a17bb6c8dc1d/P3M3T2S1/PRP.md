---
name: "P3.M3.T2.S1 — Implement arbiter resolution (new commit + tip amend + mid-chain chain rebuild): decompose/chain.go + git.Git.Add (PRD §13.6.5, §9.14 FR-M10, §18)"
description: |

  CREATE TWO NEW FILES `internal/decompose/chain.go` + `internal/decompose/chain_test.go`, EDIT
  `internal/git/git.go` (add ONE interface method + impl), and CREATE `internal/git/add_test.go`.
  chain.go is the RESOLUTION half of the multi-commit decomposition pipeline (PRD §13.6.5): it
  takes `runArbiter`'s target decision (P3.M3.T1.S1 — `prompt.ArbiterOutput.Target`, nil⇒new /
  &sha⇒amend) plus the parallel `[]CommitInfo` + `[]ChainEntry` arrays built this run, and reconciles
  the leftover working-tree changes so `git status` is clean. It dispatches to ONE of three pure
  git-plumbing paths — ALL git owned by stagecoach (FR-M10); the arbiter only decided:

    (A) null / new → resolveNewCommit: AddAll → WriteTree → generateMessage (REUSED from P3.M2.T4.S1;
        concept diff = the leftovers) → CommitTree → UpdateRefCAS. Lands an (N+1)-th commit.
    (B) target == tip → resolveTipAmend: AddAll → WriteTree → CommitTree(tree', [tipParent], tipMsg)
        reusing the tip's message verbatim (NO regeneration) → UpdateRefCAS(expectedOld = tipSHA).
        A plumbing amend — no `git commit --amend`.
    (C) target == earlier commit[i] (i < N-1) → resolveMidChain: deterministic linear-chain rebuild.
        Capture leftoverPaths from StatusPorcelain BEFORE any ReadTree; then for EACH j from i to tip:
        ReadTree(tree[j]) → Add(leftoverPaths) → WriteTree → CommitTree(rebuiltParent, msg[j]); finally
        a single UpdateRefCAS(expectedOld = tipSHA). NEVER interactive rebase; HEAD only.

  THIS TASK ADDS A NEW GIT INTERFACE METHOD — `git.Git.Add(ctx, paths []string) error` running
  `git add -- <paths...>` — because the mid-chain rebuild requires path-specific staging (the contract
  literally says "add leftover paths"). AddAll is WRONG for mid-chain (it would collapse every rebuilt
  tree to tree[N-1]); only Add lets `git add -- <leftover paths>` fold JUST the leftovers onto each
  ReadTree(tree[j]). This is a minimal, contained git-interface addition (one interface method, one
  impl, one test file) — NOT a refactor — and breaks nothing (only gitRunner implements git.Git).

  CRITICAL CORRECTION to the contract's literal "if j==target fold leftovers": trees are CUMULATIVE
  (tree[j] ⊇ tree[j-1]). Folding leftovers L into commit[i] REQUIRES L also appear in every
  subsequent rebuilt tree, else commit[i+1] reverts L (diff(tree[i+1], tree[i]+L) shows L as removed).
  So Add(leftoverPaths) is applied at EVERY j ∈ [i, N-1], not only j==i. The "at j==target" phrasing
  describes WHERE the fold is ATTRIBUTED (leftovers first appear in commit[i]'s tree), not "only
  modify tree[i]". See Implementation Blueprint §mid-chain for the proof.

  publishCommit (message.go, P3.M2.T4.S1) CANNOT be reused for amend/mid-chain: it hardcodes
  `expectedOld := parentSHA`, which is WRONG for amend (HEAD currently == tipSHA, not tipParent) and
  mid-chain (HEAD == tipSHA, not rebuiltParent). So resolveArbiter calls CommitTree + UpdateRefCAS
  DIRECTLY for all three paths (full control over expectedOld). generateMessage IS reused (null path
  only — same package, unexported, the message-gen function from P3.M2.T4.S1).

  CONTRACT (P3.M3.T2.S1, verbatim from the work item):
    1. RESEARCH NOTE: FR-M10 specifies three paths. (1) null/new: stage leftovers, snapshot,
       message-agent, commit-tree + update-ref as (N+1)-th commit. (2) target==HEAD (tip): stage
       leftovers, write-tree → tree', commit-tree -p tip's parent tree' reusing tip's message,
       update-ref HEAD (plumbing amend). (3) target==earlier commit[i] (mid-chain): rebuild linear
       chain — read-tree each base, fold leftovers at i, write-tree + commit-tree against rebuilt
       parent, update-ref (deterministic, NEVER interactive rebase). Ambiguous → null. Amend
       restricted to commits made THIS run. The mid-chain rebuild uses ReadTree to load each tree,
       adds leftovers, re-commits.
    2. INPUT: runArbiter from P3.M3.T1.S1, ReadTree + TreeDiff + WriteTree + CommitTree + UpdateRefCAS
       from git interface, the chain of commits made this run (trees, messages, SHAs).
    3. LOGIC: In decompose/arbiter.go [DEVIATION: see scope — resolveArbiter lives in decompose/chain.go
       to be merge-safe with the concurrently-owned arbiter.go]: implement
       `func resolveArbiter(ctx, deps Deps, target *string, commits []CommitInfo, chainData []ChainEntry)
       error`. For null: AddAll → WriteTree → generate message (reuse the message-generation function
       from P3.M2.T4.S1) → CommitTree → UpdateRefCAS. For tip: AddAll → WriteTree → CommitTree(-p, tree',
       tip's message) → UpdateRefCAS (amend). For mid-chain: implement in decompose/chain.go — for each
       j in the chain: ReadTree(tree[j]), if j==target fold leftovers (read-tree tree[j] then add
       leftover paths then write-tree), CommitTree against rebuilt parent[j], UpdateRefCAS chain. This
       is complex but deterministic.
    4. OUTPUT: All leftovers are reconciled — either folded into an existing commit or made into a new
       one. git status is clean after resolution.
    5. DOCS: none — internal git orchestration.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/decompose/arbiter.go — SHIPPED (P3.M3.T1.S1). Defines runArbiter + CommitInfo{SHA,
      Subject, Files []git.FileChange} + ErrArbiterFailed + convertArbiterCommits + targetInRun.
      CONSUMED VERBATIM: the CommitInfo type (commits []CommitInfo param) — parallel to chainData.
      DO NOT EDIT (merge conflict with P3.M3.T1.S1; it explicitly deferred resolution to "S2" = this task).
    - internal/decompose/message.go — SHIPPED (P3.M2.T4.S1). CONSUMED: generateMessage(ctx, deps,
      treeA, treeB) (string, error) [the null path's message-gen — REUSED, returns msg or
      *generate.RescueError] + ErrMessageFailed/ErrPublicationFailed (sibling sentinel conventions).
      DO NOT EDIT. publishCommit is NOT reused (see §6 below / description).
    - internal/decompose/roles.go — SHIPPED (P3.M2.T1.S1). CONSUMED: Deps{Git, Registry, Config,
      Roles RoleManifests, Verbose}, RoleManifests{.Message}. DO NOT EDIT.
    - internal/decompose/{planner,stager}.go — SHIPPED. Sibling sentinel conventions only. DO NOT EDIT.
    - internal/prompt/arbiter.go — SHIPPED (P3.M1.T1.S3). CONSUMED: ArbiterOutput{Target *string}
      (runArbiter's return; resolveArbiter takes target *string, NOT ArbiterOutput — the orchestrator
      unwraps it: resolveArbiter(ctx, deps, out.Target, commits, chainData)). DO NOT EDIT.
    - internal/git/git.go — EDIT (this task): ADD Add(ctx, paths []string) error to the Git interface +
      gitRunner. DO NOT modify any other method. ReadTree/StatusPorcelain/WriteTree/CommitTree/
      UpdateRefCAS/RevParseHEAD/RevParseTree are CONSUMED VERBATIM (shipped P1/P2). git.ErrCASFailed +
      git.EmptyTreeSHA CONSUMED.
    - internal/generate/* — CONSUMED (read-only): generate.RescueError{Kind,TreeSHA,ParentSHA,
      Candidate,Cause}, generate.CASError{TreeSHA,Expected,Actual,Message}, generate.ErrTimeout,
      generate.ErrRescue. NOT edited.
    - internal/decompose/decompose.go — DOES NOT EXIST YET (P3.M4.T1.S1 — the orchestrator that CALLS
      runArbiter + resolveArbiter). resolveArbiter is the RESOLUTION step only; it takes the target
      decision as a PARAMETER. No caller wiring in this task.
    - internal/signal/* — DO NOT IMPORT. resolveArbiter is SIGNAL-FREE (a synchronous resolution
      primitive; the orchestrator P3.M4.T1.S2/S4 owns signal arming).
    - cmd/, pkg/stagecoach/ — UNCHANGED.

  DELIVERABLES (4 git changes: 1 NEW file + 1 NEW test in decompose; 1 EDIT + 1 NEW test in git):
    CREATE internal/decompose/chain.go — package `decompose`; `ErrArbiterResolutionFailed` sentinel;
      `type ChainEntry struct { SHA, Tree, Message, Parent string }`; `resolveArbiter` (dispatcher) +
      `resolveNewCommit` + `resolveTipAmend` + `resolveMidChain` (the three paths); private
      `leftoverPaths` (StatusPorcelain parser) + `handleUpdateRefErr` (ErrCASFailed→*CASError centralizer)
      + `findTargetIndex` (SHA→index lookup).
    CREATE internal/decompose/chain_test.go — real-git integration tests (chn*-prefixed fixtures —
      DISTINCT from arb*-planner-stg-msg) covering all three paths + the mid-chain fold-at-every-j
      correction + clean-tree postcondition + CAS/rescue error propagation + leftoverPaths parser.
    EDIT internal/git/git.go — add `Add(ctx context.Context, paths []string) error` to the Git
      interface (with a doc comment matching the family's `add` convention) + the gitRunner impl
      (`git add -- <paths...>`, empty⇒no-op nil, non-zero⇒error). NO other changes.
    CREATE internal/git/add_test.go — integration tests mirroring addall_test.go (stages
      modified+untracked+deletion; empty-paths no-op; clean-tree no-op; non-repo error; git-binary-
      missing; context-cancelled).

  SUCCESS: `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; internal/git coverage stays ≥85% (`make coverage-gate`); go.mod/
  go.sum UNCHANGED (generate already imported by message.go; git already imported); all existing tests
  still pass; resolveArbiter(nil) makes an (N+1)-th commit with a generateMessage message; resolveArbiter
  (&tipSHA) amends the tip (reuses tip's message; HEAD parent chain intact); resolveArbiter(&earlierSHA)
  rebuilds the chain with leftovers folded at EVERY j≥i (final HEAD.tree == working tree, git status
  clean, NO leftover reverted); commit count == N for tip/mid-chain (no extra commit); == N+1 for null;
  Add stages exactly the given paths (verified via `git diff --cached --name-only`); refs move ONLY at
  the final UpdateRefCAS; arbiter.go/message.go untouched; only 4 git changes (chain.go, chain_test.go,
  git.go, add_test.go).

---

## Goal

**Feature Goal**: Implement the arbiter RESOLUTION step for multi-commit decomposition leftover
reconciliation (PRD §13.6.5 / §9.14 FR-M10 / §18.1) as `internal/decompose/chain.go`. `resolveArbiter`
takes `runArbiter`'s target decision (`prompt.ArbiterOutput.Target`: nil⇒new, &sha⇒amend) plus the
parallel `[]CommitInfo` + `[]ChainEntry` arrays (SHAs, trees, messages, parents of every commit made
this run) and reconciles the leftover working-tree changes via pure git plumbing so `git status` is
clean afterward. It dispatches to one of three deterministic paths — new (N+1)-th commit (null), tip
plumbing-amend (target==tip), or mid-chain linear rebuild (target==earlier) — ALL of which build commits
from frozen trees via `commit-tree` and move HEAD via a single CAS `update-ref` (refs move ONLY at the
final UpdateRefCAS per §18.1). Stagecoach performs ALL git; the arbiter only decided (FR-M10).

**Deliverable** (4 git changes across 2 packages):
1. `internal/decompose/chain.go` (NEW) — `ErrArbiterResolutionFailed`; `type ChainEntry struct { SHA,
   Tree, Message, Parent string }`; `resolveArbiter(ctx, deps, target *string, commits []CommitInfo,
   chainData []ChainEntry) error` + `resolveNewCommit` + `resolveTipAmend` + `resolveMidChain`;
   private `leftoverPaths` + `handleUpdateRefErr` + `findTargetIndex`.
2. `internal/decompose/chain_test.go` (NEW) — real-git integration tests (chn*-prefixed fixtures).
3. `internal/git/git.go` (EDIT) — add `Add(ctx context.Context, paths []string) error` to the `Git`
   interface + the `gitRunner` impl.
4. `internal/git/add_test.go` (NEW) — integration tests mirroring `addall_test.go`.

**Success Definition**:
- **Null → new commit**: commits/chainData length N; working tree has leftover `leftover.go`; stub
  message agent returns "chore: leftover"; resolveArbiter(ctx, deps, nil, commits, chainData) ⇒ repo
  has N+1 commits; HEAD's subject == "chore: leftover"; HEAD's parent == old tip; `git status --porcelain`
  == "" (clean); HEAD.tree contains leftover.go.
- **Tip amend**: target == chainData[N-1].SHA; leftover `related.go` staged-in; resolveArbiter ⇒ repo
  STILL has N commits (no extra commit); HEAD's subject == the tip's ORIGINAL subject (message reused
  verbatim, NOT regenerated); HEAD's parent == tip's parent (the chain above the tip is preserved);
  HEAD.tree contains both the tip's original files AND related.go; `git status --porcelain` == "" (clean).
- **Mid-chain rebuild**: N=3 (commits C0,C1,C2); target == C1.SHA (i=1); leftover `folded.go`.
  resolveArbiter ⇒ repo STILL has 3 commits; the NEW C1' (and C2') both contain folded.go (fold at
  EVERY j≥i, NOT only j==i — the load-bearing correction); C0' == C0 (unchanged); `git status --porcelain`
  == "" (clean); a LITERAL "fold only at j==i" reading would leave `git status` dirty (folded.go
  reverted by C2') — the test must assert clean AND that folded.go is present in the FINAL HEAD.tree.
- **CAS failure**: with HEAD moved externally before the final UpdateRefCAS, resolveArbiter returns a
  `*generate.CASError` (errors.As-able) whose .Actual is the moved HEAD (re-read via RevParseHEAD); HEAD
  is NOT force-updated.
- **generateMessage failure (null path)**: stub message agent times out ⇒ resolveArbiter returns the
  `*generate.RescueError` DIRECTLY (errors.As-able; NOT wrapped in ErrArbiterResolutionFailed).
- **Add**: `g.Add(ctx, ["a.go","b.go"])` stages exactly a.go+b.go (verified via `git diff --cached
  --name-only`); `g.Add(ctx, nil)` is a no-op (stages nothing); stages modifications, additions
  (untracked), AND deletions.

## Why

- **Business value**: FR-M10 / §13.6.5 promise that after a decomposition run, ALL leftovers are
  reconciled — either folded into an existing commit or made into a new one — with `git status` clean.
  Without resolution, the run leaves the user with orphan un-staged changes the stagers ignored, breaking
  the "perfect run or deterministic recovery" guarantee. This is the last step of the decompose pipeline
  that makes the whole feature feel finished (the arbiter DECIDES, this RESOLVES).
- **Integration with existing features**: consumes `runArbiter` (P3.M3.T1.S1 — the decision),
  `generateMessage` (P3.M2.T4.S1 — reused for the null path's message), the git plumbing layer
  (ReadTree/StatusPorcelain/WriteTree/CommitTree/UpdateRefCAS — P1/P2), and the `generate` typed-error
  family (RescueError/CASError — so the orchestrator's existing error handling works unchanged). It is
  the RESOLUTION half of P3.M3; the orchestrator (P3.M4.T1.S1) wires runArbiter → resolveArbiter.
- **Problems this solves and for whom**: the plan-holder persona (§7.1) runs `stagecoach` expecting a
  clean history with no leftover noise. The mid-chain rebuild is the hard part: it lets the arbiter
  attribute leftovers to an EARLIER commit (not just the tip or a new commit) WITHOUT an interactive
  rebase — deterministically, via the plumbing stagecoach already owns.

## What

**User-visible behavior**: after a multi-commit decomposition run, if the arbiter ran (working tree was
non-empty after the loop) and decided where the leftovers belong, stagecoach performs all git to make it
so. The user sees a clean working tree (`git status` empty) and a history where the leftovers landed in
exactly one place: a brand-new (N+1)-th commit, a plumbing-amended tip, or a mid-chain fold into an
earlier commit (which deterministically rebuilds that commit and every one after it). HEAD only ever
moves forward via a single compare-and-swap; on any failure HEAD is unchanged (safe).

**Technical requirements**: `resolveArbiter` is a package-level unexported function in `internal/decompose/chain.go`
consumed by the orchestrator (P3.M4.T1.S1). It reuses `generateMessage` (null path), calls
`git.AddAll`/`git.Add`/`git.WriteTree`/`git.CommitTree`/`git.UpdateRefCAS`/`git.StatusPorcelain`/
`git.ReadTree`/`git.RevParseHEAD` directly, and propagates `*generate.RescueError` (null path) and
`*generate.CASError` (any path's CAS failure) UNWRAPPED so the orchestrator's `errors.As` works. The
mid-chain path adds a NEW git interface method `Add(ctx, paths []string)`.

### Success Criteria

- [ ] resolveArbiter(nil, ...) lands an (N+1)-th commit whose message is generateMessage's output; HEAD's
      parent == the old tip; `git status --porcelain` == "".
- [ ] resolveArbiter(&tipSHA, ...) amends the tip: commit count unchanged (N); HEAD's subject == tip's
      original subject (message reused verbatim); HEAD's parent == tip's parent; HEAD.tree == tip's tree
      + leftovers; `git status --porcelain` == "".
- [ ] resolveArbiter(&earlierSHA, ...) rebuilds the chain: commit count unchanged (N); leftovers present
      in EVERY rebuilt commit from index i through the tip (fold at every j≥i); commits before i
      unchanged; FINAL `git status --porcelain` == "" (no leftover reverted).
- [ ] resolveArbiter treats target-not-found (defensive) and target==nil as the null/new path.
- [ ] A CAS failure (HEAD moved externally) returns `*generate.CASError` (errors.As-able) with Actual
      re-read from HEAD; HEAD is not force-updated.
- [ ] A generateMessage failure (null path) returns `*generate.RescueError` directly (errors.As-able).
- [ ] git infra failures (AddAll/Add/WriteTree/ReadTree/CommitTree, non-CAS UpdateRefCAS) return a wrapped
      `ErrArbiterResolutionFailed` (errors.Is-able).
- [ ] `git.Git.Add(ctx, paths)` stages exactly the given paths (modifications + additions + deletions);
      empty paths is a no-op returning nil.
- [ ] `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
      `gofmt -l internal/ pkg/` empty; internal/git coverage ≥85%.

## All Needed Context

### Context Completeness Check

_Before writing this PRP, validate: "If someone knew nothing about this codebase, would they have
everything needed to implement this successfully?"_ — YES. The three resolution algorithms (with exact
git plumbing), the ChainEntry type, the new Add method, the error contract, the test infrastructure, and
the validation gates are all specified below with exact file paths and line-level patterns to follow.
The most subtle point — the mid-chain fold-at-every-j correction — is proven, not just stated.

### Documentation & References

```yaml
# MUST READ — include these in your context window
- url: https://git-scm.com/docs/git-commit-tree   # the commit-tree plumbing used by ALL three paths
  why: "commit-tree <tree> (-p <parent>) -m <msg> builds a dangling commit (no ref move). parents==nil ⇒ root."
  critical: "Reuse each commit's ORIGINAL message for amend/mid-chain (no regeneration). The CommitTree
    interface method takes parents []string + msg via stdin (-F -). Root = empty parents."

- url: https://git-scm.com/docs/git-update-ref    # the CAS update-ref (3-arg form)
  why: "update-ref <ref> <newSHA> <expectedOld> moves the ref only if current==expectedOld."
  critical: "The amend/mid-chain paths pass expectedOld = tipSHA (CURRENT HEAD), NOT the new commit's
    parent. publishCommit hardcodes expectedOld=parentSHA and is WRONG here (see gotcha G-CAS)."

- url: https://git-scm.com/docs/git-read-tree     # read-tree <tree> (default form, no -m)
  why: "read-tree REPLACES the index with <tree>'s contents. The mid-chain rebuild loads each tree[j]."
  critical: "read-tree MUTATES THE INDEX but touches NO ref. Capture leftoverPaths from StatusPorcelain
    BEFORE the first read-tree (StatusPorcelain is INDEX-relative — after a read-tree it would show the
    wrong diff)."

- url: https://git-scm.com/docs/git-add           # git add -- <paths>
  why: "git add stages modifications, additions (untracked), AND deletions for the given paths."
  critical: "Pass paths as []string to exec.Command (run() builds one argv; NO shell — PRD §19). The
    `--` guards pathspec ambiguity. Empty paths ⇒ no-op. Path-specific add is REQUIRED for mid-chain;
    AddAll would collapse every rebuilt tree to tree[N-1] (gotcha G-ADDALL)."

- url: https://git-scm.com/docs/git-status        # git status --porcelain output format
  why: "Porcelain v1 line format: 'XY <path>' (2 status chars + 1 space + path). Renames: 'XY <orig> -> <dst>'."
  critical: "Path = line[3:] for normal lines; for ' -> ' rename lines take the part AFTER ' -> '.
    core.quotepath (default on) C-quotes non-ASCII paths — v1 assumes ASCII (documented limitation)."

# CODEBASE FILES — pattern sources (all verified, paths exact)
- file: internal/decompose/message.go
  why: "SIBLING PATTERN + CONSUMED DEPENDENCY. generateMessage(ctx, deps, treeA, treeB) (string, error)
    is REUSED for the null path's message — same package, unexported. publishCommit is the reference for
    the CommitTree→UpdateRefCAS shape BUT is NOT reused (expectedOld mismatch, gotcha G-CAS). ErrMessageFailed/
    ErrPublicationFailed show the sentinel-wrapping convention."
  pattern: "ErrXxxFailed sentinel; direct CommitTree+UpdateRefCAS calls; *RescueError/*CASError returned
    DIRECTLY (not wrapped) so errors.As works; fmt.Errorf('%w: ...: %w', sentinel, cause)."
  gotcha: "generateMessage derives parent+isUnborn INTERNALLY via RevParseHEAD and runs the dedupe loop
    FRESH — safe to call for the null path. It returns *generate.RescueError on failure; propagate UNWRAPPED."

- file: internal/decompose/arbiter.go
  why: "CONSUMED DEPENDENCY (read-only — do NOT edit). Defines CommitInfo{SHA, Subject string; Files
    []git.FileChange} — the commits []CommitInfo param type, PARALLEL to chainData. resolveArbiter takes
    target *string (the orchestrator unwraps runArbiter's ArbiterOutput.Target)."
  pattern: "CommitInfo type; ErrArbiterFailed sentinel; the FileChange→path seam (convertArbiterCommits)."
  gotcha: "arbiter.go is concurrently owned by P3.M3.T1.S1. resolveArbiter MUST live in chain.go (this
    task's file), NOT arbiter.go — editing arbiter.go = merge conflict. The P3.M3.T1.S1 PRP explicitly
    deferred resolution to 'S2' = this task."

- file: internal/decompose/roles.go
  why: "CONSUMED DEPENDENCY. Defines Deps{Git git.Git; Registry *provider.Registry; Config config.Config;
    Roles RoleManifests; Verbose *ui.Verbose} + RoleManifests{Planner, Stager, Message, Arbiter}."
  pattern: "Deps is the threaded-collaborators struct; resolveArbiter reads deps.Git + deps.Roles.Message
    (null path's generateMessage) + deps.Config (caps). No Models field."

- file: internal/git/git.go
  why: "EDIT TARGET + CONSUMED DEPENDENCY. Add the Add(ctx, paths []string) error method to the Git
    interface (after the AddAll method, ~line 91 interface / ~line 760 impl) + the gitRunner impl.
    CONSUMED: ReadTree, StatusPorcelain, WriteTree, CommitTree, UpdateRefCAS, RevParseHEAD, RevParseTree,
    AddAll, ErrCASFailed, EmptyTreeSHA."
  pattern: "gitRunner methods delegate to g.run(ctx, g.workDir, args...); mutations (AddAll/ReadTree/
    WriteTree/CommitTree) treat ALL non-zero exits as errors (no 128-as-non-error special-case); fmt.Errorf
    with the command name + exit code + trimmed stderr."
  gotcha: "ReadTree's docstring says it is 'Consumed ONLY by the arbiter's mid-chain chain rebuild' — the
    git layer was DESIGNED for the mid-chain; Add is the missing companion. EmptyTreeSHA const exists for
    the unborn base."

- file: internal/git/addall_test.go
  why: "PATTERN TO FOLLOW for add_test.go. initRepo/writeFile/stageFile/makeEmptyCommit helpers live in
    the git test package (shared across *_test.go)."
  pattern: "TestAddX_StagesModified/Deletion/CleanTreeNoOp/UnbornRepo/NotARepo/GitBinaryMissing/
    ContextCancelled — mirror these EXACTLY, swapping AddAll→Add and adding an empty-paths no-op test +
    a stages-only-given-paths (not all) test. Verify staging via `git -C repo diff --cached --name-only`."

- file: internal/decompose/arbiter_test.go
  why: "PATTERN TO FOLLOW for chain_test.go. arb*-prefixed fixture helpers (arbInitRepo/arbWriteFile/
    arbStageFile/arbCommitRaw/arbRunGit/arbCommits) + arbDeps(Deps{Git,Config,Roles:RoleManifests{...}})."
  pattern: "Real temp git repo (t.TempDir); repo-local identity config; stubtest.Build + stubtest.Manifest
    for the message agent; assert via arbRunGit (log/rev-parse/status/diff-tree). chain_test MUST use a
    DISTINCT prefix → chn* (see gotcha G-PREFIX)."
  gotcha: "chain_test.go MUST use chn*-prefixed helpers (chnInitRepo, chnWriteFile, ...) copied from
    generate_test.go / arbiter_test.go and renamed — DISTINCT from arb* (arbiter), stg* (stager),
    msg* (message), and un-prefixed (planner). Failing to prefix ⇒ duplicate-decl compile errors."

- file: internal/generate/generate.go
  why: "CONSUMED (read-only). RescueError{Kind,TreeSHA,ParentSHA,Candidate,Cause}, CASError{TreeSHA,
    Expected,Actual,Message}, ErrTimeout, ErrRescue. CASError.Error() IS the §13.5 'HEAD moved' message."
  pattern: "Construct *CASError with TreeSHA=the tree that failed to land, Expected=the CAS expected-old
    (tipSHA), Actual=re-read HEAD, Message=the message. Construct *RescueError from generateMessage's
    return (propagate as-is)."

- file: internal/prompt/arbiter.go
  why: "CONSUMED (read-only). ArbiterOutput{Target *string} — the orchestrator unwraps .Target before
    calling resolveArbiter(ctx, deps, out.Target, commits, chainData)."
  pattern: "nil Target ⇔ new commit; &sha ⇔ amend. resolveArbiter takes the *string, not ArbiterOutput."

- file: internal/decompose/planner.go
  why: "SIBLING PATTERN (Render→Execute→parse; ErrPlannerFailed sentinel; ResolveRoleModel derivation).
    NOT directly used by resolveArbiter, but establishes the package's error-sentinel + ResolveRoleModel
    conventions that chain.go must match."
```

### Current Codebase tree (relevant subset)

```bash
internal/
  decompose/
    roles.go        # SHIPPED — Deps, RoleManifests, ResolveRoles
    planner.go      # SHIPPED — callPlanner, ErrPlannerFailed (sibling pattern)
    stager.go       # SHIPPED — stageConcept, freezeSnapshot, ErrStagerFailed
    message.go      # SHIPPED — generateMessage, publishCommit, ErrMessageFailed/ErrPublicationFailed
    arbiter.go      # SHIPPED — runArbiter, CommitInfo, ErrArbiterFailed (DO NOT EDIT)
    chain.go        # ← NEW (this task): ChainEntry, resolveArbiter + 3 paths
    chain_test.go   # ← NEW (this task): chn*-prefixed integration tests
    decompose.go    # (does not exist yet — P3.M4.T1.S1 orchestrator)
  git/
    git.go          # Git interface + gitRunner — EDIT (add Add method)
    add_test.go     # ← NEW (this task): Add integration tests
    addall_test.go  # PATTERN for add_test.go
    (revparse/writetree/committree/updateref/difftree/..._test.go)
  generate/
    generate.go     # RescueError, CASError, ErrTimeout, ErrRescue (CONSUMED, read-only)
  prompt/arbiter.go # ArbiterOutput{Target} (CONSUMED)
  stubtest/stubtest.go  # Build + Manifest + NewScript (test harness)
  config/...        # Config{Timeout, MaxDiffBytes, MaxMdLines, BinaryExtensions} (CONSUMED)
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
internal/decompose/chain.go     # NEW — ChainEntry type + ErrArbiterResolutionFailed sentinel;
                                #        resolveArbiter (dispatcher: nil/not-found→new, tip→amend,
                                #        earlier→mid-chain) + resolveNewCommit + resolveTipAmend +
                                #        resolveMidChain; private leftoverPaths (StatusPorcelain parser),
                                #        handleUpdateRefErr (CAS→*CASError centralizer), findTargetIndex.
internal/decompose/chain_test.go # NEW — chn* fixtures + integration tests for all 3 paths, the
                                 #        fold-at-every-j correction, clean-tree postcondition, CAS/rescue
                                 #        error propagation, leftoverPaths unit test.
internal/git/git.go             # EDIT — add `Add(ctx context.Context, paths []string) error` to the
                                #        Git interface (doc comment) + gitRunner impl (`git add -- <paths>`).
internal/git/add_test.go        # NEW — Add integration tests mirroring addall_test.go.
```

### Known Gotchas of our codebase & Library Quirks

```go
// G-CAS (CRITICAL): publishCommit (message.go) hardcodes `expectedOld := parentSHA`. That is correct
//   for a NORMAL commit (HEAD == previous = parent) but WRONG for amend/mid-chain: at resolution time
//   HEAD == tipSHA (the loop already advanced HEAD to the tip). For amend the new commit's parent is
//   tipParent; for mid-chain it's rebuiltParent — but in BOTH cases the CAS expected-old must be tipSHA
//   (CURRENT HEAD), so the CAS checks "did HEAD move since we started". Therefore resolveArbiter calls
//   CommitTree + UpdateRefCAS DIRECTLY for all three paths (NOT publishCommit). This matches the
//   contract's INPUT list (WriteTree + CommitTree + UpdateRefCAS "from git interface").

// G-ADDALL (CRITICAL): AddAll stages the ENTIRE working tree. After ReadTree(tree[j]) for j < N-1, the
//   index = tree[j] but the working tree = tree[N-1]+leftovers, so AddAll would stage concepts j+1..N-1
//   AND leftovers ⇒ tree'[j] collapses to tree'[N-1] for ALL j ⇒ the chain collapses to identical trees.
//   Mid-chain REQUIRES path-specific Add (git add -- <leftoverPaths>) — hence the new git.Git.Add method.
//   AddAll is CORRECT for the null + tip paths (the index there == tree[N-1], so AddAll stages ONLY the
//   leftovers).

// G-FOLD (CRITICAL CORRECTION): trees are CUMULATIVE (tree[j] ⊇ tree[j-1]). Folding leftovers L into
//   commit[i] REQUIRES L appear in EVERY rebuilt tree j ∈ [i, N-1], else commit[i+1] reverts L:
//     diff(tree[i+1], tree[i]+L) shows L as REMOVED (tree[i+1] lacks L but its parent has it).
//   So Add(leftoverPaths) runs at EVERY j from i to tip, NOT only j==i. The contract's "if j==target
//   fold leftovers" describes WHERE the fold is ATTRIBUTED (leftovers first appear in commit[i]), NOT
//   "only modify tree[i]". PROOF: tree'[j]=tree[j]+L (fixed leftover blobs from the stable working tree);
//   tree'[j+1]=tree[j+1]+L=(tree[j]+concept[j+1])+L=tree'[j]+concept[j+1] (no path overlap assumed) ⇒
//   each rebuilt commit is a clean superset. ✓

// G-STATUS-FIRST: StatusPorcelain is INDEX-relative. It MUST be captured BEFORE the first ReadTree
//   (mid-chain), because ReadTree mutates the index (index becomes tree[j], so StatusPorcelain would
//   show tree[j]-vs-working-tree, not the leftover set). At resolution entry index == tree[N-1] (clean),
//   so StatusPorcelain shows ONLY the leftovers (unstaged + untracked) — exactly the fold set.

// G-LEFTOVER-DELETIONS: a leftover may be a DELETED file (` D gone.go`). After ReadTree(tree[j]) the
//   index HAS gone.go but the working tree does NOT. `git add -- gone.go` stages the DELETION (modern
//   git add handles deletions for explicit paths). So Add folds deletions correctly too.

// G-PORCELAIN-FORMAT: porcelain v1 line = "XY <path>" → path = line[3:] (skip 2 status chars + 1 space).
//   Rename/copy = "XY <orig> -> <dst>" → take the part AFTER " -> " (the destination). Filter lines
//   shorter than 4 chars. core.quotepath (default on) C-quotes non-ASCII paths — v1 ASSUMES ASCII
//   (documented limitation; the C-quoted literal would be passed verbatim to git add and likely miss).
//   After the per-concept loop index == HEAD.tree, so ONLY leftovers appear (no concept residue).

// G-PREFIX: chain_test.go MUST use chn*-prefixed fixture helpers (chnInitRepo, chnWriteFile,
//   chnStageFile, chnCommitRaw, chnRunGit, chnHeadSHA) — DISTINCT from arb* (arbiter_test),
//   stg* (stager_test), msg* (message_test), un-prefixed (planner_test). The package `decompose` test
//   files share ONE package scope; duplicate helper names = compile errors.

// G-SIGNAL-FREE: resolveArbiter does NOT import or call internal/signal (signal.RestoreDefault is
//   one-shot; the loop/resolution signal is owned by the orchestrator P3.M4.T1.S2). It is a synchronous
//   resolution primitive. generateMessage is itself signal-free (verified — message.go doc).

// G-NO-FORCE: NEVER use the 2-arg force `update-ref`. Every ref move is the 3-arg CAS (UpdateRefCAS).
//   On CAS failure, HEAD is UNCHANGED (safe) and resolveArbiter returns *generate.CASError.

// G-ROOT-COMMIT: for a root commit (i==0 on an unborn repo), CommitTree takes parents=nil (no -p) and
//   UpdateRefCAS uses expectedOld = strings.Repeat("0", 40). chainData[0].Parent == "" on a root commit.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/decompose/chain.go

// ChainEntry is one commit made this run, carrying the rebuild data the mid-chain path needs. It is
// PARALLEL to CommitInfo (P3.M3.T1.S1): same length, same order, same SHAs (chainData[i].SHA ==
// commits[i].SHA). The orchestrator builds it as it publishes each commit (it already holds tree[i],
// msg[i], newSHA[i]). resolveArbiter locates the target index via commits (per the contract) then reads
// rebuild data (Tree/Message/Parent) from chainData.
type ChainEntry struct {
	SHA     string // full commit SHA (== commits[i].SHA — parallel arrays).
	Tree    string // the commit's tree SHA (ReadTree/WriteTree target for the rebuild).
	Message string // full message — REUSED VERBATIM on amend/rebuild (NO regeneration).
	Parent  string // rebuild base: chainData[i].Parent == chainData[i-1].SHA for i>0; for i==0 the
	               //   pre-run HEAD (or "" for a root commit on an unborn repo).
}

// ErrArbiterResolutionFailed is the sentinel for arbiter-RESOLUTION infra failures (AddAll/Add/
// WriteTree/ReadTree/CommitTree infra + NON-CAS UpdateRefCAS). Wrapped (%w) so errors.Is works.
// generateMessage failure (null path) → propagate *generate.RescueError DIRECTLY (not wrapped).
// CAS failure → propagate *generate.CASError DIRECTLY (not wrapped).
var ErrArbiterResolutionFailed = errors.New("decompose: arbiter resolution failed")
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/git/git.go — add Add to the Git interface + gitRunner
  - ADD to the Git interface (place immediately AFTER the AddAll method declaration):
      // Add stages the given paths (modifications, additions/untracked, AND deletions) into the index
      // via `git add -- <paths...>`. It MUTATES THE INDEX (writes .git/index) but touches NO ref
      // (PRD §18.1). It is the path-specific companion to AddAll, consumed ONLY by the arbiter's
      // mid-chain chain rebuild (PRD §13.6.5 — "add leftover paths"): after ReadTree(tree[j]) the index
      // == tree[j] and Add(leftoverPaths) folds JUST the leftovers onto it (AddAll would collapse the
      // chain). Empty paths ⇒ no-op nil. The `--` guards pathspec ambiguity. ALL non-zero exits are
      // errors (the mutation convention shared with AddAll/ReadTree/WriteTree/CommitTree — no 128-as-
      // non-error special-case). Read-only w.r.t. refs.
      Add(ctx context.Context, paths []string) error
  - ADD the gitRunner impl (place immediately AFTER the AddAll method):
      func (g *gitRunner) Add(ctx context.Context, paths []string) error {
          if len(paths) == 0 {
              return nil // no-op (contract: "add leftover paths" — empty set stages nothing)
          }
          args := make([]string, 0, 2+len(paths))
          args = append(args, "add", "--")
          args = append(args, paths...)            // each path one argv element (no shell — PRD §19)
          _, stderr, code, err := g.run(ctx, g.workDir, args...)
          if err != nil {
              return err                            // git binary missing / context cancelled / start failure
          }
          if code != 0 {
              return fmt.Errorf("git add: failed (exit %d): %s", code, strings.TrimSpace(stderr))
          }
          return nil
      }
  - FOLLOW pattern: internal/git/git.go AddAll (lines ~760-790) — SAME run()-delegation + mutation
    convention (ALL non-zero exits are errors; fmt.Errorf with command name + exit + trimmed stderr).
  - NAMING: Add (matches AddAll family); signature `Add(ctx context.Context, paths []string) error`.
  - PRESERVE: every other method on the Git interface + gitRunner UNCHANGED.

Task 2: CREATE internal/git/add_test.go — Add integration tests
  - IMPLEMENT: TestAdd_StagesOnlyGivenPaths (NOT all) + TestAdd_StagesModifiedAndUntracked +
    TestAdd_StagesDeletion + TestAdd_EmptyPathsNoOp + TestAdd_CleanTreeNoOp + TestAdd_UnbornRepoStages +
    TestAdd_NotARepo + TestAdd_GitBinaryMissing + TestAdd_ContextCancelled.
  - FOLLOW pattern: internal/git/addall_test.go VERBATIM (same fixture helpers: initRepo, writeFile,
    stageFile, makeEmptyCommit — already defined in the git test package). Swap AddAll(ctx)→Add(ctx, paths).
  - KEY ADDITIONAL TEST (vs AddAll): TestAdd_StagesOnlyGivenPaths — write a.go + b.go + c.go; commit;
    modify all three; Add(ctx, ["a.go","b.go"]); assert `git diff --cached --name-only` == {a.go,b.go}
    (c.go NOT staged). This is the property that distinguishes Add from AddAll and that the mid-chain
    rebuild depends on. Verify staging via exec.Command("git","-C",repo,"diff","--cached","--name-only").
  - PLACEMENT: internal/git/add_test.go (alongside addall_test.go).

Task 3: CREATE internal/decompose/chain.go — ChainEntry + resolveArbiter + the three paths
  - IMPLEMENT (in order): ErrArbiterResolutionFailed; type ChainEntry; resolveArbiter (dispatcher);
    resolveNewCommit; resolveTipAmend; resolveMidChain; leftoverPaths; handleUpdateRefErr; findTargetIndex.
  - FOLLOW pattern: internal/decompose/message.go (sibling sentinel + direct CommitTree/UpdateRefCAS calls
    + *RescueError/*CASError returned directly) + internal/decompose/arbiter.go (package doc-comment style).
  - NAMING: resolveArbiter, resolveNewCommit, resolveTipAmend, resolveMidChain (all unexported, package-
    level); ChainEntry (exported-type-style PascalCase but the func is unexported — keep ChainEntry as an
    unexported-by-convention type used in the signature; it MAY be exported since resolveArbiter's signature
    references it and the orchestrator builds []ChainEntry — EXPORT it so the orchestrator (P3.M4.T1.S1,
    same package) can construct it; it stays in the internal package so not part of the public API).
  - PLACEMENT: internal/decompose/chain.go (NOT arbiter.go — merge-safety, see scope).

Task 4: CREATE internal/decompose/chain_test.go — integration tests (chn* fixtures)
  - IMPLEMENT: chn*-prefixed fixture helpers (copied from arbiter_test.go/generate_test.go, renamed);
    TestResolveArbiter_NullNewCommit; TestResolveArbiter_TipAmend; TestResolveArbiter_MidChainRebuild
    (asserts fold-at-every-j: leftover in C1' AND C2'; clean tree); TestResolveArbiter_TargetNotFound→Null;
    TestResolveArbiter_CASFailure; TestResolveArbiter_RescueErrorPropagation (null path timeout);
    TestLeftoverPaths (porcelain parser unit test); TestResolveArbiter_CleanTreePostcondition.
  - FOLLOW pattern: internal/decompose/arbiter_test.go (arbDeps-style Deps construction; stubtest.Build +
    stubtest.Manifest for the message agent; chnRunGit for assertions; real temp git repo).
  - NAMING: chn* helpers + TestResolve* / TestLeftoverPaths.
  - COVERAGE: all 3 paths + both error types (CAS, RescueError) + the parser + clean-tree postcondition.
  - PLACEMENT: internal/decompose/chain_test.go.
```

### resolveArbiter — the dispatcher (PRD §13.6.5, findings §9)

```go
// resolveArbiter reconciles the leftover working-tree changes per runArbiter's target decision.
// It is the RESOLUTION step (PRD §13.6.5 / FR-M10); runArbiter (P3.M3.T1.S1) only DECIDED.
//
// PRECONDITION (documented; the orchestrator guarantees): the per-concept loop made ≥1 commit;
// HEAD.tree == index == tree[N-1] (the full accumulated index, clean); the WORKING TREE holds the
// leftovers (StatusPorcelain != ""). The orchestrator already called runArbiter and passes its
// ArbiterOutput.Target (nil ⇒ new; &sha ⇒ amend). commits []CommitInfo and chainData []ChainEntry are
// PARALLEL (same length, order, SHAs).
//
// Branching: target==nil || N==0 → resolveNewCommit. Else find idx where chainData[idx].SHA == *target;
// not found (runArbiter should have nulled it — defensive) → resolveNewCommit. idx==N-1 → resolveTipAmend.
// idx<N-1 → resolveMidChain. (N==1 ⇒ the only in-run commit is the tip ⇒ any non-nil target is tip amend;
// mid-chain requires N≥2.)
//
// On success the working tree is CLEAN (index == HEAD.tree == working tree). On any failure HEAD is
// UNCHANGED (refs move ONLY at the final UpdateRefCAS — §18.1). *generate.RescueError (null path) and
// *generate.CASError (any CAS failure) propagate DIRECTLY (not wrapped); other infra failures wrap
// ErrArbiterResolutionFailed.
func resolveArbiter(ctx context.Context, deps Deps, target *string, commits []CommitInfo, chainData []ChainEntry) error {
	N := len(chainData)
	if target == nil || N == 0 {
		return resolveNewCommit(ctx, deps, commits, chainData)
	}
	idx := findTargetIndex(*target, chainData)
	if idx < 0 {
		return resolveNewCommit(ctx, deps, commits, chainData) // not found → defensive null
	}
	if idx == N-1 {
		return resolveTipAmend(ctx, deps, chainData)
	}
	return resolveMidChain(ctx, deps, idx, chainData)
}

// findTargetIndex returns the index of sha in chainData, or -1 if absent.
func findTargetIndex(sha string, chainData []ChainEntry) int {
	for i, c := range chainData {
		if c.SHA == sha {
			return i
		}
	}
	return -1
}
```

### resolveNewCommit — null path (A)

```go
// resolveNewCommit (path A, null): AddAll → WriteTree → generateMessage → CommitTree → UpdateRefCAS.
// Lands an (N+1)-th commit whose message is generated from the leftovers (concept diff = TreeDiff(tip,
// treePrime) = exactly the leftovers). generateMessage (P3.M2.T4.S1) is REUSED — same package.
func resolveNewCommit(ctx context.Context, deps Deps, commits []CommitInfo, chainData []ChainEntry) error {
	N := len(chainData)
	// tipSHA = current HEAD; tipTree = the base for the concept diff. On an empty run (N==0) this path
	// is reached only defensively (the arbiter does not run on a clean tree) — treat HEAD as the base.
	tipSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil {
		return fmt.Errorf("%w: rev-parse head: %w", ErrArbiterResolutionFailed, err)
	}
	tipTree := ""
	if N > 0 {
		tipSHA = chainData[N-1].SHA       // authoritative (chainData tracks this run's commits)
		tipTree = chainData[N-1].Tree
	} else if !isUnborn {
		tipTree, _ = deps.Git.RevParseTree(ctx, "HEAD") // empty "" on unborn (the EmptyTree base)
	}

	// 1. Stage all leftovers (index == tree[N-1] ⇒ AddAll stages ONLY the leftovers).
	if err := deps.Git.AddAll(ctx); err != nil {
		return fmt.Errorf("%w: add -A: %w", ErrArbiterResolutionFailed, err)
	}
	// 2. Snapshot the staged index.
	treePrime, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return fmt.Errorf("%w: write-tree: %w", ErrArbiterResolutionFailed, err)
	}
	// 3. Generate the message from the leftover concept diff (generateMessage derives its own parent).
	treeA := tipTree
	if treeA == "" {
		treeA = git.EmptyTreeSHA // unborn base — generateMessage's TreeDiff treats it as a tree arg
	}
	msg, err := generateMessage(ctx, deps, treeA, treePrime)
	if err != nil {
		return err // *generate.RescueError — propagate DIRECTLY (not wrapped)
	}
	// 4. Commit (parent = tipSHA; root if tipSHA=="").
	var parents []string
	if tipSHA != "" {
		parents = []string{tipSHA}
	}
	newSHA, err := deps.Git.CommitTree(ctx, treePrime, parents, msg)
	if err != nil {
		return fmt.Errorf("%w: commit-tree: %w", ErrArbiterResolutionFailed, err)
	}
	// 5. CAS-advance HEAD (expected-old = tipSHA = CURRENT HEAD).
	expectedOld := tipSHA
	if tipSHA == "" {
		expectedOld = strings.Repeat("0", 40) // root commit on an unborn repo
	}
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
		return handleUpdateRefErr(ctx, deps, treePrime, expectedOld, msg, err)
	}
	return nil
}
```

### resolveTipAmend — tip path (B)

```go
// resolveTipAmend (path B, target==tip): AddAll → WriteTree → CommitTree(tree', [tipParent], tipMsg)
// reusing the tip's message VERBATIM (NO regeneration) → UpdateRefCAS(expectedOld = tipSHA). A plumbing
// amend — no `git commit --amend`. publishCommit is NOT used (its expectedOld=parentSHA is wrong: HEAD
// currently == tipSHA, not tipParent).
func resolveTipAmend(ctx context.Context, deps Deps, chainData []ChainEntry) error {
	N := len(chainData)
	tip := chainData[N-1]
	tipSHA, tipParent, tipMsg := tip.SHA, tip.Parent, tip.Message

	if err := deps.Git.AddAll(ctx); err != nil {
		return fmt.Errorf("%w: add -A: %w", ErrArbiterResolutionFailed, err)
	}
	treePrime, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return fmt.Errorf("%w: write-tree: %w", ErrArbiterResolutionFailed, err)
	}
	// Reuse the tip's message VERBATIM (no regeneration). Parent = tipParent (the amend); root if "".
	var parents []string
	if tipParent != "" {
		parents = []string{tipParent}
	}
	newSHA, err := deps.Git.CommitTree(ctx, treePrime, parents, tipMsg)
	if err != nil {
		return fmt.Errorf("%w: commit-tree: %w", ErrArbiterResolutionFailed, err)
	}
	// CAS expected-old = tipSHA (CURRENT HEAD), NOT tipParent. (publishCommit would use tipParent = WRONG.)
	expectedOld := tipSHA
	if tipParent == "" && tipSHA == "" {
		expectedOld = strings.Repeat("0", 40) // unborn root (defensive)
	}
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
		return handleUpdateRefErr(ctx, deps, treePrime, expectedOld, tipMsg, err)
	}
	return nil
}
```

### resolveMidChain — the deterministic linear rebuild (path C) — CRITICAL: fold at EVERY j≥i

```go
// resolveMidChain (path C, target==earlier commit[i], i<N-1): deterministic linear-chain rebuild.
// NEVER interactive rebase; HEAD only; refs move ONLY at the final UpdateRefCAS.
//
//   1. Capture leftoverPaths = parse(StatusPorcelain()) — MUST be BEFORE any ReadTree (G-STATUS-FIRST).
//   2. rebuiltParent = chainData[i].Parent (for i>0 == chainData[i-1].SHA; for i==0 the pre-run HEAD / "").
//   3. for j := i; j < N; j++:
//        ReadTree(chainData[j].Tree)   // index = tree[j]
//        Add(leftoverPaths)            // fold leftovers onto tree[j] → index = tree[j]+leftovers
//        treePrime = WriteTree()
//        parent := rebuiltParent (or nil if rebuiltParent=="" for the root case at j==i==0)
//        newSHA = CommitTree(treePrime, parent, chainData[j].Message)  // REUSE msg[j] verbatim
//        rebuiltParent = newSHA
//   4. UpdateRefCAS(HEAD, rebuiltParent, tipSHA)  // single atomic move; tipSHA = chainData[N-1].SHA
//
// The fold runs at EVERY j ∈ [i, N-1] (G-FOLD): trees are cumulative, so leftovers folded into commit[i]
// must also appear in every subsequent rebuilt tree, else commit[i+1] reverts them (dirty tree).
func resolveMidChain(ctx context.Context, deps Deps, i int, chainData []ChainEntry) error {
	N := len(chainData)
	tipSHA := chainData[N-1].SHA

	// 1. Capture leftover paths BEFORE any ReadTree (StatusPorcelain is index-relative).
	status, err := deps.Git.StatusPorcelain(ctx)
	if err != nil {
		return fmt.Errorf("%w: status: %w", ErrArbiterResolutionFailed, err)
	}
	leftoverPaths := leftoverPaths(status)
	// (leftoverPaths is non-empty — the arbiter only runs if StatusPorcelain != "" — but guard anyway.)

	// 2. Rebuild base.
	rebuiltParent := chainData[i].Parent

	// 3. Walk j = i..N-1, rebuilding each commit with leftovers folded in.
	for j := i; j < N; j++ {
		if err := deps.Git.ReadTree(ctx, chainData[j].Tree); err != nil {
			return fmt.Errorf("%w: read-tree[%d]: %w", ErrArbiterResolutionFailed, j, err)
		}
		if len(leftoverPaths) > 0 {
			if err := deps.Git.Add(ctx, leftoverPaths); err != nil { // fold (NOT AddAll — G-ADDALL)
				return fmt.Errorf("%w: add[%d]: %w", ErrArbiterResolutionFailed, j, err)
			}
		}
		treePrime, err := deps.Git.WriteTree(ctx)
		if err != nil {
			return fmt.Errorf("%w: write-tree[%d]: %w", ErrArbiterResolutionFailed, j, err)
		}
		var parents []string
		if rebuiltParent != "" {
			parents = []string{rebuiltParent}
		}
		newSHA, err := deps.Git.CommitTree(ctx, treePrime, parents, chainData[j].Message) // msg[j] verbatim
		if err != nil {
			return fmt.Errorf("%w: commit-tree[%d]: %w", ErrArbiterResolutionFailed, j, err)
		}
		rebuiltParent = newSHA
	}

	// 4. Single CAS move (expected-old = tipSHA = CURRENT HEAD).
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", rebuiltParent, tipSHA); err != nil {
		return handleUpdateRefErr(ctx, deps, "", tipSHA, "", err) // no single tree/msg for the rebuilt chain
	}
	return nil
}
```

### Private helpers: leftoverPaths + handleUpdateRefErr

```go
// leftoverPaths parses `git status --porcelain` output into the leftover path set (mid-chain only).
// Each non-empty line "XY <path>" → path = line[3:]; rename/copy "XY <orig> -> <dst>" → the part after
// " -> " (destination). Lines shorter than 4 chars are skipped. core.quotepath (default on) C-quotes
// non-ASCII paths — v1 ASSUMES ASCII (documented limitation). After the per-concept loop index ==
// HEAD.tree, so ONLY leftovers (unstaged + untracked + deletions) appear — exactly the fold set.
func leftoverPaths(status string) []string {
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		if len(line) < 4 { // "XY <path>" minimum (2 status + 1 space + ≥1 path char)
			continue
		}
		rest := line[3:] // skip "XY "
		if idx := strings.Index(rest, " -> "); idx >= 0 {
			rest = rest[idx+len(" -> "):] // rename/copy: take the destination
		}
		if rest = strings.TrimSpace(rest); rest != "" {
			paths = append(paths, rest)
		}
	}
	return paths
}

// handleUpdateRefErr centralizes the two UpdateRefCAS failure kinds: ErrCASFailed → *generate.CASError
// (re-read HEAD for the §13.5 Actual; errors.As-able; NOT wrapped); otherwise → wrapped
// ErrArbiterResolutionFailed (non-CAS infra). Mirrors publishCommit's CAS handling in message.go.
func handleUpdateRefErr(ctx context.Context, deps Deps, tree, expectedOld, msg string, err error) error {
	if errors.Is(err, git.ErrCASFailed) {
		actual, _, _ := deps.Git.RevParseHEAD(ctx) // re-read for the §13.5 message's Actual (D5)
		return &generate.CASError{TreeSHA: tree, Expected: expectedOld, Actual: actual, Message: msg}
	}
	return fmt.Errorf("%w: update-ref: %w", ErrArbiterResolutionFailed, err)
}
```

### Integration Points

```yaml
GIT INTERFACE (internal/git/git.go):
  - add method: "Add(ctx context.Context, paths []string) error" → the Git interface + gitRunner impl
  - consumed (no change): AddAll, WriteTree, CommitTree, UpdateRefCAS, ReadTree, StatusPorcelain,
    RevParseHEAD, RevParseTree, ErrCASFailed, EmptyTreeSHA

DECOMPOSE PACKAGE (internal/decompose/chain.go):
  - new file: ChainEntry type, ErrArbiterResolutionFailed, resolveArbiter + 3 paths + helpers
  - consumed (no change): CommitInfo (arbiter.go), generateMessage (message.go), Deps/RoleManifests
    (roles.go), prompt.ArbiterOutput (the orchestrator unwraps .Target)
  - consumed (read-only): generate.RescueError, generate.CASError

ORCHESTRATOR WIRING (P3.M4.T1.S1 — NOT this task): the orchestrator builds []CommitEntry as it publishes
  each commit (it holds tree[i], msg[i], newSHA[i]), calls runArbiter, then
  resolveArbiter(ctx, deps, out.Target, commits, chainData). No caller wiring in this task.

CONFIG (internal/config): consumed read-only — deps.Config.Timeout/MaxDiffBytes/MaxMdLines/BinaryExtensions
  (flow through generateMessage's TreeDiff). No new config keys.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after each file creation — fix before proceeding.
go build ./...                                    # compile (catches the new Add interface method wiring)
go vet ./...                                      # go vet (catches shadowed vars, printf issues)
gofmt -l internal/ pkg/                           # MUST print nothing (empty = formatted)
golangci-lint run                                 # the repo's linter (Makefile `make lint`)

# Scope-specific quick check after the git.go edit:
go build ./internal/git/... && go vet ./internal/git/...
# Scope-specific quick check after chain.go:
go build ./internal/decompose/... && go vet ./internal/decompose/...

# Expected: zero errors. If gofmt prints file names, run `gofmt -w internal/ pkg/` and re-check.
```

### Level 2: Unit & Integration Tests (Component Validation)

```bash
# The new Add method (internal/git):
go test -race ./internal/git/... -run 'TestAdd' -v

# The new resolveArbiter + helpers (internal/decompose):
go test -race ./internal/decompose/... -run 'TestResolveArbiter|TestLeftoverPaths|TestFindTargetIndex' -v

# Full packages (regression — nothing existing broke):
go test -race ./internal/git/...
go test -race ./internal/decompose/...

# The whole suite (Makefile `make test`):
go test -race ./...

# Expected: all pass. If the mid-chain test fails with "git status dirty", the fold-at-every-j
# correction (G-FOLD) was implemented incorrectly — verify Add(leftoverPaths) runs inside the j loop.
```

### Level 3: Integration Testing (System Validation)

```bash
# Coverage gate — CRITICAL because this task EDITS internal/git (the gate enforces ≥85% on the 4 core
# packages). The new Add method + add_test.go must keep internal/git above threshold:
make coverage-gate      # enforces >=85% on internal/{git,provider,generate,config}
# (equivalently: go test -coverprofile=coverage.out ./internal/git/... && go tool cover -func=coverage.out)

# Manual mid-chain sanity (real repo, 3 commits) — optional, the integration tests cover this, but to
# eyeball the rebuilt history:
#   build a 3-commit chain, leave a leftover, set target to the middle commit's SHA, run resolveArbiter,
#   then: git log --oneline (still 3 commits) + git status (clean) + the leftover file present in HEAD.

# Expected: coverage-gate PASS (internal/git ≥85%); manual history shows N commits (tip/mid) or N+1 (null)
# with a clean tree.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# The mid-chain rebuild is the hard part — validate the "fold at every j" invariant explicitly.
# In TestResolveArbiter_MidChainRebuild, after resolveArbiter with target=middle commit:
#   1. assert commit count UNCHANGED (N commits, NOT N+1)
#   2. for EACH rebuilt commit from i to tip: assert its tree CONTAINS the leftover path
#      (git rev-parse <sha>^{tree} then git ls-tree -r, OR diff-tree against its parent)
#   3. assert `git status --porcelain` == "" (clean — no leftover reverted)
#   4. assert commits BEFORE i are byte-identical (chainData[j].SHA unchanged for j < i)

# The tip-amend invariant (TestResolveArbiter_TipAmend):
#   1. assert commit count UNCHANGED (N)
#   2. assert HEAD's subject == the tip's ORIGINAL subject (message reused verbatim — provide a stub
#      message agent that returns a DIFFERENT string and assert it was NOT used; tip path never calls it)
#   3. assert HEAD's parent == the tip's parent (chain above preserved)
#   4. assert HEAD.tree contains BOTH the tip's original files AND the leftovers
#   5. assert `git status --porcelain` == ""

# Error-path validation:
#   - CAS failure: move HEAD externally (git commit) between resolveArbiter's tree-build and its
#     UpdateRefCAS (inject a mutation, or test the helper directly) → errors.As(*generate.CASError) true;
#     .Actual == the moved HEAD; HEAD NOT force-updated.
#   - RescueError (null path): stub message agent with SleepMS > Timeout → resolveArbiter returns
#     *generate.RescueError (errors.As true); ErrArbiterResolutionFailed NOT wrapping it.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` compiles (Add interface method wired; chain.go compiles; no import cycles).
- [ ] `go vet ./...` clean.
- [ ] `go test -race ./...` green (Makefile `make test`).
- [ ] `golangci-lint run` clean (Makefile `make lint`).
- [ ] `gofmt -l internal/ pkg/` empty.
- [ ] `make coverage-gate` PASS (internal/git ≥85% after adding Add).

### Feature Validation

- [ ] resolveArbiter(nil, ...) lands an (N+1)-th commit; message == generateMessage output; HEAD parent == old tip; tree clean.
- [ ] resolveArbiter(&tipSHA, ...) amends: N commits; subject == tip's original (reused); parent == tip's parent; HEAD.tree == tip tree + leftovers; clean.
- [ ] resolveArbiter(&earlierSHA, ...) rebuilds: N commits; leftovers in EVERY rebuilt commit i..tip; commits before i unchanged; FINAL tree clean (no leftover reverted).
- [ ] resolveArbiter(target-not-found) and resolveArbiter(nil) both → new-commit path (defensive null).
- [ ] CAS failure → `*generate.CASError` (errors.As-able; Actual re-read; HEAD not force-updated).
- [ ] generateMessage failure (null path) → `*generate.RescueError` returned directly (errors.As-able).
- [ ] git infra failures → wrapped `ErrArbiterResolutionFailed` (errors.Is-able).
- [ ] `git.Git.Add(ctx, ["a.go","b.go"])` stages exactly a.go+b.go (not all); empty paths ⇒ no-op nil; stages mods+adds+deletions.

### Code Quality Validation

- [ ] chain.go follows the package's sentinel convention (ErrXxxFailed) + direct CommitTree/UpdateRefCAS calls (message.go pattern).
- [ ] Add follows the AddAll mutation convention (all non-zero exits are errors; run()-delegation; -C repo flag).
- [ ] resolveArbiter is SIGNAL-FREE (no internal/signal import).
- [ ] refs move ONLY at the final UpdateRefCAS in every path (§18.1).
- [ ] No 2-arg force `update-ref` anywhere.
- [ ] chain_test.go uses chn*-prefixed fixtures (no collision with arb*/stg*/msg*/planner's un-prefixed).
- [ ] arbiter.go, message.go, roles.go, planner.go, stager.go UNCHANGED (only 4 git changes).
- [ ] go.mod/go.sum UNCHANGED (generate already imported by message.go; git already imported).

### Documentation & Deployment

- [ ] Doc comments on Add (interface + impl) match the AddAll/ReadTree family (mutation convention; consumer = mid-chain rebuild).
- [ ] Doc comments on resolveArbiter + the three paths + ChainEntry + ErrArbiterResolutionFailed (the package's doc-first style; every exported/unexported symbol commented).
- [ ] No new environment variables or config keys.

---

## Anti-Patterns to Avoid

- ❌ Don't reuse `publishCommit` for the amend/mid-chain paths — it hardcodes `expectedOld := parentSHA`, which is the WRONG CAS expected-old when HEAD == tipSHA (gotcha G-CAS). Call CommitTree + UpdateRefCAS directly.
- ❌ Don't use `AddAll` in the mid-chain rebuild — after `ReadTree(tree[j])` for j < N-1 it would stage concepts j+1..N-1 AND leftovers, collapsing every rebuilt tree to tree[N-1] (gotcha G-ADDALL). Use `Add(leftoverPaths)`.
- ❌ Don't fold leftovers ONLY at j==i in the mid-chain rebuild — trees are cumulative, so commit[i+1] would revert the leftovers (dirty tree). Fold at EVERY j ∈ [i, N-1] (gotcha G-FOLD).
- ❌ Don't capture `StatusPorcelain` AFTER the first `ReadTree` — it is index-relative; ReadTree mutates the index (gotcha G-STATUS-FIRST). Capture leftover paths FIRST.
- ❌ Don't put `resolveArbiter` in `arbiter.go` — that file is concurrently owned by P3.M3.T1.S1 (merge conflict). It lives in `chain.go` (gotcha: the P3.M3.T1.S1 PRP explicitly deferred resolution to "S2" = this task).
- ❌ Don't regenerate messages for the tip/mid-chain paths — reuse each commit's ORIGINAL message verbatim (the amend/rebuild preserves authorship intent; only the null path generates a fresh message).
- ❌ Don't use the 2-arg force `update-ref` (gotcha G-NO-FORCE) — every ref move is the 3-arg CAS.
- ❌ Don't wrap `*generate.RescueError` or `*generate.CASError` in `ErrArbiterResolutionFailed` — return them DIRECTLY so the orchestrator's `errors.As` works (the message.go/arbiter.go convention).
- ❌ Don't edit arbiter.go / message.go / roles.go / planner.go / stager.go — only 4 git changes (chain.go, chain_test.go, git.go, add_test.go).
- ❌ Don't skip the Add-only-given-paths test (TestAdd_StagesOnlyGivenPaths) — it is the property that distinguishes Add from AddAll and that the mid-chain rebuild depends on.
