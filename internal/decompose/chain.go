package decompose

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dabstractor/stagecoach/internal/generate"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/hooks"
)

// ErrArbiterResolutionFailed is the sentinel for arbiter-RESOLUTION infra failures (AddAll/Add/
// WriteTree/ReadTree/CommitTree infra + NON-CAS UpdateRefCAS). Wrapped (%w) so errors.Is works.
// generateMessage failure (null path) → propagate *generate.RescueError DIRECTLY (not wrapped).
// CAS failure → propagate *generate.CASError DIRECTLY (not wrapped).
var ErrArbiterResolutionFailed = errors.New("decompose: arbiter resolution failed")

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

// resolveArbiter reconciles the leftover working-tree changes per runArbiter's target decision.
// It is the RESOLUTION step (PRD §13.6.5 / FR-M10); runArbiter (P3.M3.T1.S1) only DECIDED.
//
// PRECONDITION (documented; the orchestrator guarantees): the per-concept loop made ≥1 commit;
// HEAD.tree == index == tree[N-1] (the full accumulated index, clean). The orchestrator already called
// runArbiter and passes its ArbiterOutput.Target (nil ⇒ new; &sha ⇒ amend). commits []CommitInfo and
// chainData []ChainEntry are PARALLEL (same length, order, SHAs). tStart is the frozen working-tree-as-
// of-run-start tree; leftoverPaths = DiffTreeNames(tipTree, tStart) is the frozen leftover set (the fold
// set for the mid-chain path). FR-M1d: the gate/diff/staging are ALL frozen (no live StatusPorcelain/AddAll).
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
func resolveArbiter(ctx context.Context, deps Deps, target *string, commits []CommitInfo, chainData []ChainEntry, tStart string, leftoverPaths []string) error {
	N := len(chainData)
	if target == nil || N == 0 {
		return resolveNewCommit(ctx, deps, commits, chainData, tStart)
	}
	idx := findTargetIndex(*target, chainData)
	if idx < 0 {
		return resolveNewCommit(ctx, deps, commits, chainData, tStart) // not found → defensive null
	}
	if idx == N-1 {
		return resolveTipAmend(ctx, deps, chainData, tStart)
	}
	return resolveMidChain(ctx, deps, idx, chainData, tStart, leftoverPaths)
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

// resolveNewCommit (path A, null): treePrime := tStart → generateMessage → CommitTree → UpdateRefCAS →
// ReadTree(tStart). Lands an (N+1)-th commit whose tree == T_start (the frozen working-tree-as-of-
// run-start). generateMessage (P3.M2.T4.S1) is REUSED — same package; its concept diff = TreeDiff(tipTree,
// T_start) = exactly the frozen leftovers (FR-M10). FR-M1d: NO AddAll/WriteTree (those read the live
// working tree); the index is synced to T_start via ReadTree after the CAS.
func resolveNewCommit(ctx context.Context, deps Deps, commits []CommitInfo, chainData []ChainEntry, tStart string) error {
	N := len(chainData)
	// tipSHA = current HEAD; tipTree = the base for the concept diff. On an empty run (N==0) this path
	// is reached only defensively (the arbiter does not run on a clean tree) — treat HEAD as the base.
	tipSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil {
		return fmt.Errorf("%w: rev-parse head: %w", ErrArbiterResolutionFailed, err)
	}
	tipTree := ""
	if N > 0 {
		tipSHA = chainData[N-1].SHA // authoritative (chainData tracks this run's commits)
		tipTree = chainData[N-1].Tree
	} else if !isUnborn {
		tree, _ := deps.Git.RevParseTree(ctx, "HEAD") // empty "" on unborn (the EmptyTree base)
		tipTree = tree
	}

	// 1. FR-M1d: treePrime := T_start (the frozen tree). NO AddAll/WriteTree — those read the live tree.
	treePrime := tStart
	// 2. Generate the message from the frozen leftover concept diff (tipTree → T_start).
	treeA := tipTree
	if treeA == "" {
		treeA = git.EmptyTreeSHA // unborn base — generateMessage's TreeDiff treats it as a tree arg
	}
	msg, err := generateMessage(ctx, deps, treeA, treePrime)
	if err != nil {
		return err // *generate.RescueError — propagate DIRECTLY (not wrapped)
	}
	// 3. Commit (parent = tipSHA; root if tipSHA=="").
	var parents []string
	if tipSHA != "" {
		parents = []string{tipSHA}
	}
	// Run the repo's commit hooks scoped to treePrime (PRD §9.25 FR-V1/V3/V7). hookParent=tipSHA
	// (the new commit's parent = current HEAD). On herr → return the *RescueError DIRECTLY
	// (propagates unwrapped, matching generateMessage's pattern). DryRun:false (decompose commits).
	finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx, deps.Git, deps.Config, treePrime, tipSHA, msg,
		hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
	if herr != nil {
		return herr
	}
	newSHA, err := deps.Git.CommitTree(ctx, finalTree, parents, finalMsg)
	if err != nil {
		return fmt.Errorf("%w: commit-tree: %w", ErrArbiterResolutionFailed, err)
	}
	// 4. CAS-advance HEAD (expected-old = tipSHA = CURRENT HEAD).
	expectedOld := tipSHA
	if tipSHA == "" {
		expectedOld = strings.Repeat("0", 40) // root commit on an unborn repo
	}
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
		return handleUpdateRefErr(ctx, deps, treePrime, expectedOld, msg, err)
	}
	// Best-effort post-commit AFTER update-ref succeeded (FR-V7 — exit disregarded; commit stands).
	_ = hooks.RunPostCommit(ctx, deps.Git, deps.Config, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
	// 5. FR-M1d (3): sync the index to T_start so git status is clean (index == HEAD.tree == T_start).
	if err := deps.Git.ReadTree(ctx, tStart); err != nil {
		return fmt.Errorf("%w: read-tree sync: %w", ErrArbiterResolutionFailed, err)
	}
	// (F1) If a permitted pre-commit mutation re-treed (treePrime != finalTree), the blanket ReadTree
	// above reset the index to the PRE-hook T_start — losing the hook's formatting for the committed
	// concept's paths. Reconcile those paths to the committed (post-hook) tree so the index reflects
	// HEAD for them (git-commit parity for formatter/lint-staged/prettier hooks). Best-effort.
	if rerr := hooks.ReconcileIndex(ctx, deps.Git, treePrime, finalTree, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose}); rerr != nil {
		if deps.Verbose != nil {
			deps.Verbose.VerboseWarn(fmt.Sprintf("post-mutation index reconcile failed (commit stands): %v", rerr))
		}
	}
	return nil
}

// resolveTipAmend (path B, target==tip): treePrime := tStart → CommitTree(tStart, [tipParent], tipMsg)
// reusing the tip's message VERBATIM (NO regeneration) → UpdateRefCAS(expectedOld = tipSHA) →
// ReadTree(tStart). A plumbing amend — no `git commit --amend`. publishCommit is NOT used (its
// expectedOld=parentSHA is wrong: HEAD currently == tipSHA, not tipParent). FR-M1d: NO AddAll/WriteTree.
func resolveTipAmend(ctx context.Context, deps Deps, chainData []ChainEntry, tStart string) error {
	N := len(chainData)
	tip := chainData[N-1]
	tipSHA, tipParent, tipMsg := tip.SHA, tip.Parent, tip.Message

	// FR-M1d: treePrime := T_start (the frozen tree). NO AddAll/WriteTree — those read the live tree.
	treePrime := tStart
	// Reuse the tip's message VERBATIM as the hook INPUT (no regeneration). Parent = tipParent
	// (the amend); root if "".
	var parents []string
	if tipParent != "" {
		parents = []string{tipParent}
	}
	// Run the repo's commit hooks scoped to treePrime (PRD §9.25 FR-V1/V3/V7). hookParent=tipParent
	// (the amended commit's parent = the original tip's parent). amend parity: the tip message is the
	// hook INPUT and prepare-commit-msg MAY annotate it — mirrors `git commit --amend` re-running the
	// msg hooks. §20.2 constrains ONLY resolveMidChain's non-target commits; the tip IS the target, so
	// its message MAY change (§5). DryRun:false (decompose commits).
	finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx, deps.Git, deps.Config, treePrime, tipParent, tipMsg,
		hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
	if herr != nil {
		return herr
	}
	newSHA, err := deps.Git.CommitTree(ctx, finalTree, parents, finalMsg)
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
	// Best-effort post-commit AFTER update-ref succeeded (FR-V7 — exit disregarded; commit stands).
	_ = hooks.RunPostCommit(ctx, deps.Git, deps.Config, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
	// FR-M1d (3): sync the index to T_start so git status is clean (index == HEAD.tree == T_start).
	if err := deps.Git.ReadTree(ctx, tStart); err != nil {
		return fmt.Errorf("%w: read-tree sync: %w", ErrArbiterResolutionFailed, err)
	}
	// (F1) If a permitted pre-commit mutation re-treed (treePrime != finalTree), the blanket ReadTree
	// above reset the index to the PRE-hook T_start — losing the hook's formatting for the committed
	// tip's paths. Reconcile those paths to the committed (post-hook) tree so the index reflects HEAD
	// for them. Best-effort.
	if rerr := hooks.ReconcileIndex(ctx, deps.Git, treePrime, finalTree, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose}); rerr != nil {
		if deps.Verbose != nil {
			deps.Verbose.VerboseWarn(fmt.Sprintf("post-mutation index reconcile failed (commit stands): %v", rerr))
		}
	}
	return nil
}

// resolveMidChain (path C, target==earlier commit[i], i<N-1): deterministic linear-chain rebuild.
// NEVER interactive rebase; HEAD only; refs move ONLY at the final UpdateRefCAS. FR-M1d: the tree per j
// is built via OverlayTreePaths (index/object-store-only; never touches the working tree) — NO
// StatusPorcelain/ReadTree(tree[j])/Add/WriteTree (those read/staged the live working tree).
//
//  1. rebuiltParent = chainData[i].Parent (for i>0 == chainData[i-1].SHA; for i==0 the pre-run HEAD / "").
//  2. for j := i; j < N; j++:
//     treePrime = OverlayTreePaths(tree[j], T_start, leftoverPaths)  // fold ONLY the leftover paths
//     onto tree[j]; unchanged paths keep tree[j]'s blob. (FR-M10 mid-chain fold.)
//     parent := rebuiltParent (or nil if rebuiltParent=="" for the root case at j==i==0)
//     newSHA = CommitTree(treePrime, parent, chainData[j].Message)  // REUSE msg[j] verbatim
//     rebuiltParent = newSHA
//  3. UpdateRefCAS(HEAD, rebuiltParent, tipSHA)  // single atomic move; tipSHA = chainData[N-1].SHA
//  4. ReadTree(tStart)  // sync the index to T_start (rebuilt tip == T_start by construction)
//
// The fold runs at EVERY j ∈ [i, N-1] (G-FOLD): trees are cumulative, so leftovers folded into commit[i]
// must also appear in every subsequent rebuilt tree, else commit[i+1] reverts them (dirty tree).
// leftoverPaths (the param) is the frozen set DiffTreeNames(tipTree, T_start) threaded in from the gate.
func resolveMidChain(ctx context.Context, deps Deps, i int, chainData []ChainEntry, tStart string, leftoverPaths []string) error {
	N := len(chainData)
	tipSHA := chainData[N-1].SHA

	// 1. Rebuild base.
	rebuiltParent := chainData[i].Parent

	// 2. Walk j = i..N-1, rebuilding each commit with the leftovers folded in via OverlayTreePaths.
	for j := i; j < N; j++ {
		treePrime, err := deps.Git.OverlayTreePaths(ctx, chainData[j].Tree, tStart, leftoverPaths)
		if err != nil {
			return fmt.Errorf("%w: overlay-tree-paths[%d]: %w", ErrArbiterResolutionFailed, j, err)
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

	// 3. Single CAS move (expected-old = tipSHA = CURRENT HEAD).
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", rebuiltParent, tipSHA); err != nil {
		return handleUpdateRefErr(ctx, deps, "", tipSHA, "", err) // no single tree/msg for the rebuilt chain
	}
	// 4. FR-M1d (3): sync the index to T_start (rebuilt tip == T_start) so git status is clean.
	if err := deps.Git.ReadTree(ctx, tStart); err != nil {
		return fmt.Errorf("%w: read-tree sync: %w", ErrArbiterResolutionFailed, err)
	}
	return nil
}

// handleUpdateRefErr centralizes the two UpdateRefCAS failure kinds: ErrCASFailed → *generate.CASError
// (re-read HEAD for the §13.5 Actual; errors.As-able; NOT wrapped); otherwise → wrapped
// ErrArbiterResolutionFailed (non-CAS infra). Mirrors publishCommit's CAS handling in message.go.
func handleUpdateRefErr(ctx context.Context, deps Deps, tree, expectedOld, msg string, err error) error {
	if errors.Is(err, git.ErrCASFailed) {
		actual, _, _ := deps.Git.RevParseHEAD(ctx) // re-read for the §13.5 message's Actual (D5)
		actualTree := ""                           // + Actual^{tree} for the already-committed fast path
		if actual != "" {
			actualTree, _ = deps.Git.RevParseTree(ctx, actual)
		}
		return &generate.CASError{TreeSHA: tree, Expected: expectedOld, Actual: actual, ActualTree: actualTree, Message: msg}
	}
	return fmt.Errorf("%w: update-ref: %w", ErrArbiterResolutionFailed, err)
}
