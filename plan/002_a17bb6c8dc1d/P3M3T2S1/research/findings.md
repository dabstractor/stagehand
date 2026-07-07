# P3.M3.T2.S1 — Arbiter Resolution: Empirical Findings

Subject: `resolveArbiter` (new-commit / tip-amend / mid-chain chain rebuild), PRD §13.6.5 / §9.14
FR-M10. Consumes `runArbiter`'s decision (P3.M3.T1.S1) + the git plumbing + the chain built this run.

---

## §1. The contract (verbatim from the work item)

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
3. LOGIC: implement `func resolveArbiter(ctx, deps Deps, target *string, commits []CommitInfo,
   chainData []ChainEntry) error`. For null: AddAll → WriteTree → generate message (reuse the
   message-generation function from P3.M2.T4.S1) → CommitTree → UpdateRefCAS. For tip: AddAll →
   WriteTree → CommitTree(-p tip's parent, tree', tip's message) → UpdateRefCAS (amend). For
   mid-chain: implement in decompose/chain.go — for each j: ReadTree(tree[j]), if j==target fold
   leftovers (read-tree tree[j] then add leftover paths then write-tree), CommitTree against
   rebuilt parent[j], UpdateRefCAS chain.
4. OUTPUT: All leftovers are reconciled. git status is clean after resolution.

## §2. SCOPE — what this task owns / does NOT touch

- OWNED (this task): `internal/decompose/chain.go` (NEW) — ChainEntry + resolveArbiter + the three
  resolution helpers + leftoverPaths + ErrArbiterResolutionFailed. `internal/decompose/chain_test.go`
  (NEW, chn*-prefixed fixtures). `internal/git/git.go` (EDIT — add `Add(ctx, paths []string)` to the
  Git interface + gitRunner — see §5). `internal/git/add_test.go` (NEW).
- DO NOT EDIT (concurrently / shipped): `internal/decompose/arbiter.go` (P3.M3.T1.S1 owns it — it
  defines runArbiter + CommitInfo + ErrArbiterFailed; editing = merge conflict). `internal/decompose/
  {roles,planner,stager,message}.go` (shipped/in-flight). `internal/prompt/arbiter.go`. `internal/generate/*`.
  `internal/git/*` EXCEPT the Add addition. cmd/, pkg/stagecoach/.
- DEVIATION (file placement): the contract says "implement resolveArbiter in decompose/arbiter.go",
  BUT arbiter.go is concurrently owned by P3.M3.T1.S1. To be merge-safe, resolveArbiter lives in
  `decompose/chain.go` (this task's file — explicitly deferred to "S2" by the P3.M3.T1.S1 PRP). It is
  a package-level unexported function, consumed identically by the orchestrator (P3.M4.T1.S1).
- NOT this task: the orchestrator that CALLS runArbiter + resolveArbiter (P3.M4.T1.S1). resolveArbiter
  is the RESOLUTION step only — it takes the target decision as a PARAMETER.

## §3. The three resolution paths (mechanics — verified against git.go + message.go)

PRECONDITION (documented): resolveArbiter runs AFTER the per-concept loop made ≥1 commit. State at
entry: `HEAD.tree == index == tree[N-1]` (the full accumulated index, clean), and the WORKING TREE
holds the leftovers (unstaged changes no stager claimed) — `StatusPorcelain != ""`. The orchestrator
already called runArbiter and passes its `prompt.ArbiterOutput.Target` (nil ⇒ new; &sha ⇒ amend).

**(A) null (target == nil) → new (N+1)-th commit:**
  AddAll (stage all working-tree changes = leftovers onto the tree[N-1] index) → WriteTree → treePrime
  → generateMessage(ctx, deps, tipTree, treePrime) [reuse P3.M2.T4.S1; concept diff = TreeDiff(tipTree,
  treePrime) = exactly the leftovers] → CommitTree(treePrime, [tipSHA], msg) → UpdateRefCAS(HEAD,
  newSHA, tipSHA). tipSHA = chainData[N-1].SHA; tipTree = chainData[N-1].Tree. AddAll is CORRECT here
  (we build on the full tree[N-1] base; AddAll stages ONLY the leftovers because index==tree[N-1]).

**(B) target == tip (chainData[N-1].SHA) → plumbing amend:**
  AddAll → WriteTree → treePrime → CommitTree(treePrime, [tipParent], tipMsg) [NO regeneration — reuse
  tip's message verbatim] → UpdateRefCAS(HEAD, newSHA, tipSHA). tipParent = chainData[N-1].Parent;
  tipMsg = chainData[N-1].Message; the CAS expected-old is tipSHA (CURRENT HEAD), NOT tipParent.

**(C) target == earlier commit[i] (i < N-1) → mid-chain rebuild:**
  1. Capture leftoverPaths = parse(StatusPorcelain()) — MUST be BEFORE any ReadTree (StatusPorcelain
     is INDEX-relative; ReadTree mutates the index).
  2. rebuiltParent = chainData[i].Parent (the rebuild's starting parent — for i>0 it is
     chainData[i-1].SHA; for i==0 it is the pre-run HEAD or "" for a root commit).
  3. for j := i; j < N; j++:
       ReadTree(chainData[j].Tree)        // index = tree[j]
       Add(leftoverPaths)                 // fold leftovers onto tree[j] → index = tree[j]+leftovers
       treePrime = WriteTree()
       newSHA = CommitTree(treePrime, [rebuiltParent] or nil, chainData[j].Message)
       rebuiltParent = newSHA
  4. UpdateRefCAS(HEAD, rebuiltParent, tipSHA)  // single atomic move; tipSHA = chainData[N-1].SHA

## §4. CRITICAL CORRECTION — fold leftovers at EVERY j (not only j==target)

The contract literally reads "if j==target fold leftovers". A LITERAL reading (fold only at j==i;
reuse original tree[j] for j>i, just re-parented) produces a BROKEN CHAIN:
  - commit'[i]   = (tree[i]+leftovers, parent=origParent[i])
  - commit'[i+1] = (tree[i+1] ORIGINAL, parent=commit'[i])
  Diff(commit'[i+1], commit'[i]) = diff(tree[i+1], tree[i]+leftovers) ⇒ the leftovers show as
  REMOVED (tree[i+1] lacks them but its parent has them). So commit'[i+1] reverts the leftovers.
  Final HEAD.tree = tree[N-1] (no leftovers) ≠ working tree ⇒ git status DIRTY ⇒ VIOLATES the
  contract's "git status is clean after resolution".

CORRECT behavior: trees are CUMULATIVE (each commit's tree is a superset of its parent's). Folding
leftovers into commit[i] REQUIRES they also appear in every subsequent tree. So Add(leftoverPaths) is
applied at EVERY j from i to N-1 (the leftovers' working-tree blobs overlay each tree[j], producing
tree'[j] = tree[j]+leftovers). The logical "attribution" is commit[i] (leftovers first appear there);
mechanically every rebuilt tree j∈[i,N-1] includes them. This is the ONLY chain-consistent reading.
The "if j==target" phrasing describes WHERE the fold is ATTRIBUTED, not "only modify tree[i]".

PROOF of consistency: tree'[j] = tree[j]+leftovers (fixed leftover blobs from the stable working
tree). tree'[j+1] = tree[j+1]+leftovers = (tree[j]+concept[j+1])+leftovers = tree'[j]+concept[j+1]
(assuming no path overlap — see §8 gotcha). So each rebuilt commit is a clean superset. ✓

## §5. CRITICAL GAP — git.Git has NO path-specific Add (AddAll is wrong for mid-chain)

The git interface (internal/git/git.go lines 48-170) has AddAll(ctx) ONLY — NO Add(ctx, paths). The
mid-chain rebuild needs "add leftover paths" (contract verbatim: "add leftover paths"). AddAll is
WRONG for mid-chain: after ReadTree(tree[j]) (j<N-1), the index = tree[j], but the working tree =
tree[N-1]+leftovers, so AddAll would stage concepts j+1..N-1 AND leftovers ⇒ tree'[j] collapses to
tree'[N-1] for ALL j ⇒ the chain collapses to identical trees. BROKEN.

RESOLUTION: this task ADDS `Add(ctx context.Context, paths []string) error` to the git.Git interface +
gitRunner + add_test.go. Justification: (a) the contract explicitly requires path-based staging; (b)
only `gitRunner` implements git.Git (no fakes/mocks — verified by grepping all `func (.) AddAll/ReadTree/
WriteTree`); so the addition is contained (one interface method, one impl, one test) and breaks nothing;
(c) ReadTree's own docstring says it is "Consumed ONLY by the arbiter's mid-chain chain rebuild" — the
git layer was DESIGNED for the mid-chain; Add is the missing companion piece the planner's INPUT
enumeration omitted. Add runs `git add -- <paths...>` (the `--` guards pathspec ambiguity; modern git
add stages modifications, additions, AND deletions; empty paths ⇒ no-op nil). This is a minimal,
well-scoped git-interface addition — NOT a refactor.

## §6. publishCommit CANNOT be reused for amend/mid-chain (expectedOld mismatch)

message.go's publishCommit(ctx, deps, tree, parentSHA, msg) hardcodes `expectedOld := parentSHA`. That
is correct for a NORMAL commit (parentSHA = previous HEAD; CAS checks HEAD==parentSHA) but WRONG for:
  - AMEND: new commit's parent = tipParent, but HEAD currently = tipSHA. publishCommit would set
    expectedOld=tipParent ≠ HEAD(tipSHA) ⇒ CAS FAILS. Need expectedOld = tipSHA.
  - MID-CHAIN: final commit's parent = rebuilt tip, but HEAD = tipSHA. Need expectedOld = tipSHA.
publishCommit is owned by message.go (in-flight P3.M2.T4.S1 — do not edit). So resolveArbiter calls
CommitTree + UpdateRefCAS DIRECTLY for all three paths (full control over expectedOld). This matches
the contract's INPUT list (WriteTree + CommitTree + UpdateRefCAS "from git interface"). generateMessage
IS reused for the null path's message (it is the message-gen function from P3.M2.T4.S1, same package).

## §7. The error contract (sentinels + typed errors)

- `ErrArbiterResolutionFailed = errors.New("decompose: arbiter resolution failed")` (NEW, in chain.go).
  Wrapped (%w) around git INFRA failures (AddAll/Add/WriteTree/ReadTree/CommitTree infra + non-CAS
  UpdateRefCAS). errors.Is-able.
- generateMessage failure (null path) → propagate the `*generate.RescueError` DIRECTLY (NOT wrapped)
  so errors.As(err, &re) works (the orchestrator prints the §18.3 rescue: treePrime + manual recovery).
- UpdateRefCAS CAS failure → propagate `*generate.CASError` DIRECTLY (re-read HEAD via RevParseHEAD for
  the §13.5 "HEAD moved" Actual). A private `handleUpdateRefErr(ctx, deps, tree, expectedOld, msg, err)`
  helper centralizes this (ErrCASFailed → *CASError; else → wrapped ErrArbiterResolutionFailed).
- SAFETY: refs move ONLY at the final UpdateRefCAS (§18.1). Mid-chain ReadTree/Add/WriteTree/CommitTree
  create dangling objects + mutate the index but move NO ref until the final CAS. On any mid-rebuild
  failure, HEAD is UNCHANGED (safe); the index is left transient (recovery = reset to the original tip).
  On success the index == HEAD.tree == working tree ⇒ clean.

## §8. ChainEntry type + leftover paths discovery

```go
type ChainEntry struct {
    SHA     string // full commit SHA (== commits[i].SHA — parallel arrays)
    Tree    string // tree SHA (ReadTree/WriteTree target for the rebuild)
    Message string // full message (REUSED verbatim on amend/rebuild — NO regeneration)
    Parent  string // rebuild base — chainData[i].Parent; for i>0 == chainData[i-1].SHA; for i==0 the
                   //   pre-run HEAD (or "" for a root commit on an unborn repo)
}
```
commits []CommitInfo (P3.M3.T1.S1) and chainData []ChainEntry are PARALLEL (same length, order, SHAs).
resolveArbiter locates the target index by SHA (in either; use commits per the contract) then reads
rebuild data (Tree/Message/Parent) from chainData.

leftover paths (mid-chain only): resolveArbiter calls deps.Git.StatusPorcelain() and parses via a
private `leftoverPaths(status string) []string`: each porcelain line "XY <path>" → path = line[3:];
rename/copy "XY <orig> -> <path>" → the DESTINATION after " -> ". Captured BEFORE the rebuild (index-
relative). GOTCHA: StatusPorcelain does NOT disable core.quotepath (verified) — non-ASCII paths may be
C-quoted; v1 assumes ASCII (documented). After the loop the index==HEAD.tree, so StatusPorcelain shows
ONLY the leftovers (unstaged + untracked) — exactly the fold set. `git add <paths>` stages mods, adds
(untracked), and deletions, so all leftover kinds fold correctly.

## §9. Branching in resolveArbiter (dispatcher)

target == nil || N==0 → resolveNewCommit (null). Else find idx where chainData[idx].SHA == *target;
not found (runArbiter should have nulled it; defensive) → resolveNewCommit. idx == N-1 → resolveTipAmend.
idx < N-1 → resolveMidChain. (N==1 ⇒ the only in-run commit is the tip ⇒ any non-nil target is tip amend;
mid-chain requires N≥2.) FR-M8 (no empty commits) is automatically satisfied: null leftovers are
non-empty (arbiter only runs if StatusPorcelain!=""); tip/mid-chain trees change (leftovers added).

## §10. Test infrastructure (chn*-prefixed fixtures, no collision)

- chain_test.go is package decompose. EXISTING fixture prefixes: planner_test = UN-PREFIXED
  (initRepo…), stager_test = stg*, message_test = msg*, arbiter_test = arb*. chain_test MUST use a
  DISTINCT prefix → `chn*` (chnInitRepo, chnWriteFile, chnStageFile, chnCommitRaw, chnRunGit, chnGitOut,
  chnHeadSHA), copied verbatim from internal/generate/generate_test.go (lines 21-72) and renamed.
- Deps construction (mirrors message_test.go): `Deps{Git: git.New(repo), Config: config.Defaults(),
  Roles: RoleManifests{Message: stubtest.Manifest(bin, stubtest.Options{Out: "<msg>"})}, Verbose: nil}`.
  resolveArbiter needs Roles.Message ONLY for the null path (generateMessage). tip/mid-chain tests need
  no message agent (no generation) but providing a Message manifest is harmless.
- Building real chainData: use chnCommitRaw to make each run-commit, then derive Tree via
  `git rev-parse <sha>^{tree}` (or deps.Git.RevParseTree) and Parent via the previous SHA / pre-run HEAD.
- add_test.go (internal/git) follows addall_test.go: init repo, write files, modify/add, call
  g.Add(ctx, paths), assert staged via `git diff --cached --name-only`; assert Add([]) no-op.

## §11. Validation gates (verified Makefile targets)

`go build ./... && go vet ./... && go test ./... && gofmt -l internal/ pkg/` (Makefile `make check` =
vet+test+gofmt; `make lint` = golangci-lint). go.mod/go.sum UNCHANGED (generate already imported by
message.go; git already imported). Expected git diff: 4 changes (chain.go, chain_test.go NEW; git.go
EDIT for Add; add_test.go NEW). arbiter.go UNCHANGED.

## §12. One-paragraph summary

resolveArbiter (decompose/chain.go) takes runArbiter's target decision + the parallel commits/chainData
arrays and reconciles the leftovers: null → AddAll/WriteTree/generateMessage/CommitTree/UpdateRefCAS
(new (N+1)-th commit); tip → AddAll/WriteTree/CommitTree(tree',tipParent,tipMsg)/UpdateRefCAS(expectedOld
=tipSHA) (plumbing amend); mid-chain → capture leftoverPaths from StatusPorcelain, then for each j from
target to tip ReadTree(tree[j])+Add(leftoverPaths)+WriteTree+CommitTree(rebuiltParent,msg[j]), final
UpdateRefCAS(expectedOld=tipSHA). It requires a NEW git.Git.Add(ctx,paths) method (the contract's "add
leftover paths"; AddAll is wrong mid-chain). It corrects the contract's "fold at j==target" to "fold at
every j∈[target,tip]" (else the chain reverts the leftovers). It calls CommitTree+UpdateRefCAS directly
(publishCommit's hardcoded expectedOld=parentSHA is wrong for amend/mid-chain). On success the working
tree is clean; on failure HEAD is safe (refs move only at the final CAS).
