# Decomposition Architecture — Multi-Commit Pipeline

## Overview

The decompose pipeline (`internal/decompose/`) turns a dirty, un-staged working tree into N logically-coherent commits using four agent roles: **planner**, **stager**, **message**, **arbiter**. Each role independently resolves its provider/model. The pipeline composes v1's snapshot machinery N times in a loop.

## The Four Agent Roles

| Role | Mode | Job | Output | Reuses |
|---|---|---|---|---|
| planner | bare | analyze full working-tree diff; decide count + partition | JSON `{count, single, commits:[...], message?}` | prompt/planner.go |
| stager | **tooled** | stage one concept's changes (`git add`, hunk-stage) | exits 0; mutates index | prompt/stager.go, Render(RenderTooled) |
| message | bare | generate one commit message from tree-to-tree diff | raw text | v1 generate.CommitStaged primitives |
| arbiter | bare | decide which just-made commit leftovers belong to, or "new" | JSON `{target: "<sha>"\|null}` | prompt/arbiter.go |

## Pipeline Flow (§13.6.3)

```
1. planner ──▶ concepts[0..N-1]   (one bare call; single-shortcut → done §13.6.4)
2. for i in 0..N-1:
   a. stager[i]     ──▶  index now holds concepts[0..i]   (tooled call)
   b. snapshot[i]   ──▶  tree[i] = write-tree             (FROZEN before stager[i+1])
   c. message[i]    : diff(tree[i-1], tree[i]) ──▶ msg[i] (bare call; overlaps stager[i+1])
   d. stager[i+1]   ──▶  index now holds concepts[0..i+1] (parallel with c, only if i+1 < N)
   e. commit[i]     =  commit-tree -p newSHA[i-1] tree[i] msg[i]
   f. update-ref    HEAD newSHA[i] newSHA[i-1]            (serialized CAS)
3. arbiter (only if status --porcelain non-empty)
```

## Three Safety Invariants (§13.6.7)

1. **tree[i] is frozen before stager[i+1] starts.** WriteTree is a pure, ref/index-read-only operation. The orchestrator snapshots immediately after stager[i] returns and before launching stager[i+1].

2. **Concept diffs are tree-to-tree, never index-vs-HEAD.** `message[i]` reasons over `git diff tree[i-1] tree[i]`. This is immune to concurrent staging AND to earlier commits landing. The single-commit path's StagedDiff (index-vs-HEAD) is deliberately NOT reused.

3. **update-refs serialize.** commit[i] parents to newSHA[i-1] and CAS-moves HEAD only if `HEAD == newSHA[i-1]`. Generation may overlap; publication may not.

## Index Model: Accumulate, Never Reset

Stagehand does NOT reset the index between concepts. After stager[i], the index holds concepts[0..i]; tree[i] is the full accumulation. After commit[N-1] lands, HEAD.tree == tree[N-1] == full accumulated index, so the index is clean relative to HEAD. Un-committed residue lives ONLY in the working tree — that's the arbiter's input.

## Base Cases
- `tree[-1]` = original parent tree (`git rev-parse HEAD^{tree}`, or empty tree for unborn repo)
- commit[0] may be a root commit (unborn repo); subsequent commits chain normally
- Per-concept "what landed" = `diff-tree newSHA[i]` vs `newSHA[i-1]`

## Single-Commit Shortcut (§13.6.4)

If planner returns `single: true`, stagehand uses planner's `message` directly: `git add -A → snapshot → commit-tree → update-ref`. No separate message agent call. If that message fails the duplicate check (§9.7), fall back to standard message agent.

## Arbiter Resolution (§13.6.5)

Stagehand performs ALL git; arbiter only decides:
- **null / "new"**: stage leftovers, snapshot, message-agent, commit-tree + update-ref as (N+1)-th commit
- **target == HEAD (tip)**: stage leftovers, write-tree → tree', commit-tree -p tip's parent tree' reusing tip's message, update-ref HEAD (plumbing amend)
- **target == earlier commit[i] (mid-chain)**: rebuild linear chain via read-tree/write-tree/commit-tree reconstruction (NEVER interactive rebase, NEVER touches non-HEAD refs)
- **Ambiguous → null (new commit)**

## Failure Handling (§13.6.6)

| Failure | Response |
|---|---|
| Stager stages nothing (tree[i] == tree[i-1]) | Skip commit[i]; no empty commits; log; continue |
| Stager exits non-zero | Retry once; second failure → treat as empty; continue |
| message[i] generation fails | Rescue for concept i only; prior commits stand; remaining staged work left in index |
| CAS failure on commit[i] | Abort with §13.5 HEAD-moved message; prior commits stand; print tree[i] recovery |
| Planner unparseable/fails | Surface error; nothing snapshotted yet; exit non-rescue |

## Required New Git Methods

```go
// RevParseTree returns the tree SHA of HEAD (or a given commit's tree).
// Uses: git rev-parse HEAD^{tree} or git rev-parse <sha>^{tree}
RevParseTree(ctx context.Context, ref string) (string, error)

// TreeDiff returns the diff between two tree SHAs (concept diff).
// Uses: git diff <treeA> <treeB> with binary filtering applied.
TreeDiff(ctx context.Context, treeA, treeB string, opts StagedDiffOptions) (string, error)

// ReadTree loads a tree into the index (for chain rebuild).
// Uses: git read-tree <tree>
ReadTree(ctx context.Context, tree string) error

// StatusPorcelain returns `git status --porcelain` output (arbiter trigger).
StatusPorcelain(ctx context.Context) (string, error)

// WorkingTreeDiff returns the unstaged working-tree diff (planner input).
// Uses: git diff (no --cached) with binary filtering applied.
WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions) (string, error)
```

## Package Structure

```
internal/decompose/
├── decompose.go    # Decompose() orchestrator — full pipeline
├── roles.go        # per-role provider/model resolution + manifest building
├── planner.go      # planner agent call + JSON parse/retry + single-shortcut
├── stager.go       # tooled stager agent call + snapshot freeze + overlap scheduling
├── arbiter.go      # arbiter agent call + JSON parse
├── chain.go        # linear-chain rebuild for mid-chain amend (FR-M10)
└── *_test.go       # integration with stub planner/stager/arbiter + temp repo
```

## Decompose Deps Structure (injectable for testing)

```go
type Deps struct {
    Git       git.Git
    Registry  *provider.Registry
    Config    config.Config
    Roles     RoleManifests  // resolved (provider, manifest) per role
    Verbose   *ui.Verbose
}

type RoleManifests struct {
    Planner  provider.Manifest  // bare
    Stager   provider.Manifest  // tooled
    Message  provider.Manifest  // bare
    Arbiter  provider.Manifest  // bare
}
```

Tests use the stubtest infrastructure: stub planner (canned JSON), stub stager (scripted git add of named paths), stub arbiter (canned target/null).
