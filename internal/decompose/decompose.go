// Package decompose implements the multi-commit decomposition pipeline (PRD §13.6.2): given an
// un-staged working tree, it produces N logically-coherent commits by running a four-agent pipeline
// (planner → stager → message → arbiter) with per-role provider/model resolution.
//
// This file (decompose.go) is the TOP-LEVEL orchestrator (PRD §13.6 / §11.4 / §9.14 FR-M1/M2/M4/M11):
// Decompose is the single entry point that turns an un-staged, dirty working tree into an ordered
// sequence of logically-coherent commits. It routes by mode (single escape-hatch / auto / forced),
// runs the planner, takes the FR-M11 single-call shortcut when the planner judges N=1, drives the
// 1-deep-overlapped per-concept loop (stage→freeze→generate→publish) with FR-M8 empty-skip and
// serialized CAS publication, and wires the arbiter (runArbiter→resolveArbiter) when leftovers remain.
//
// It is the single place overlap goroutine scheduling lives — every sibling (callPlanner /
// stageConcept / generateMessage / publishCommit / runArbiter / resolveArbiter) is a SIGNAL-FREE
// synchronous primitive consumed here. Signal arming is S2 (P3.M4.T1.S2); the escape-hatch gets
// signal handling for free via generate.CommitStaged's internal signal.SetSnapshot.
//
// S1/S2 boundary: FR-M12 per-concept isolation (stager retry-once-then-empty, message rescue-for-
// concept-i, CAS abort-with-recovery) and loop signal arming are S2 — S1 propagates errors
// structurally and documents the seams so S2 can wrap them.
package decompose

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/prompt"
	"github.com/dustin/stagehand/internal/signal"
)

// ErrDecomposeFailed is the sentinel for orchestrator-level INFRA failures not owned by a sibling
// sentinel (e.g. baseTree derivation, WorkingTreeDiff for the arbiter, AddAll before the escape-hatch).
// callPlanner/stager/message/arbiter/resolveArbiter failures carry their OWN sentinels (propagated).
// *generate.RescueError (message gen) and *generate.CASError (CAS) propagate DIRECTLY (not wrapped).
var ErrDecomposeFailed = errors.New("decompose: orchestrator failed")

// CommitResult is one commit produced by a Decompose run (mirrors generate.Result's commit-relevant
// fields). Ordered oldest-first in DecomposeResult.Commits. On the happy path (arbiter did not run) the
// SHA/Subject/Message/Files are the published commit's. When the arbiter RAN, Decompose re-reads git
// post-arbiter (rereadFinalCommits) so Commits carry the FINAL, resolvable SHAs/subjects/file-lists;
// Message is "" in those rebuilt entries (the success report prints only SHA+Subject+Files).
type CommitResult struct {
	SHA     string           // the published commit SHA (newSHA[i])
	Subject string           // ExtractSubject(Message) — for the "[<short-sha>] <subject>] line (FR42)
	Message string           // the full commit message committed verbatim
	Files   []git.FileChange // DiffTree(newSHA, isRoot) — the "what landed" file-list (FR42)
}

// DecomposeResult is the outcome of Decompose. Commits is ordered oldest-first. Amended is the number of
// commits the arbiter rewrote (tip amend=1, mid-chain at index i = N-i); 0 if the arbiter did not run
// (clean tree) or made a new commit (null target). Designed to mirror the future pkg/stagehand public
// DecomposeResult (P4.M2.T1.S1).
//
// §G-RESULT — the post-arbiter gap is CLOSED: when the arbiter runs, Decompose calls
// rereadFinalCommits (LogRange(preRunHEAD..HEAD) + DiffTree) AFTER runArbiterPhase succeeds and
// replaces Commits with the FINAL, resolvable post-arbiter entries (tip/mid-chain new SHAs; the
// null-path (N+1)-th commit is included). The re-read is best-effort: on a git-read error it logs via
// Verbose and KEEPS the loop's pre-arbiter Commits (commits are already published — stale SHAs beat
// erroring). When the arbiter did NOT run (clean tree → StatusPorcelain "" → arbiter skipped, or
// len(commits)==0), Commits are the loop's accurate entries, unchanged.
type DecomposeResult struct {
	Commits []CommitResult // the ordered commits created this run (oldest first)
	Amended int            // arbiter rewrite count (0/1/N-i); 0 if the arbiter did not run or made a new commit (null).
}

// DecomposeRescueError carries the partial result + concept context when message[i] generation fails
// mid-loop (PRD §13.6.6 / §18.3 multi-commit variant / §9.14 FR-M12a). It wraps *generate.RescueError
// (held as a field, NOT embedded — embedding would shadow Error/Unwrap) so errors.As to *RescueError
// and errors.Is to generate.ErrRescue/ErrTimeout traverse the Unwrap chain for exit-code mapping
// (exitcode.For → Rescue=3 / Timeout=124). Commits holds the already-published commits 0..i-1 (they
// stand). The loop prints generate.FormatRescueMulti(...) to deps.Out BEFORE returning this.
type DecomposeRescueError struct {
	Rescue       *generate.RescueError // the concept-i failure: TreeSHA=tree[i], ParentSHA=newSHA[i-1], Candidate, Kind
	ConceptTitle string                // concepts[i].Title — for the rescue header
	Index        int                   // concept index i (0-based)
	Count        int                   // total concept count N
	Commits      []CommitResult        // partial commits 0..i-1 (already published — they stand)
}

func (e *DecomposeRescueError) Error() string {
	if e.Rescue != nil {
		return e.Rescue.Error()
	}
	return "decompose: concept generation failed"
}

// Unwrap returns the underlying *RescueError so errors.As(&re) + errors.Is(ErrRescue/ErrTimeout)
// (→ exitcode 3/124) traverse the chain. nil-safe.
func (e *DecomposeRescueError) Unwrap() error {
	if e.Rescue != nil {
		return e.Rescue
	}
	return nil
}

// msgOut carries the result of an in-flight generateMessage goroutine. It is unexported and scoped to
// runLoop; the buffered(1) channel type is chan msgOut. Every goroutine sends exactly once then exits.
type msgOut struct {
	conceptIdx int
	treeA      string
	treeB      string
	msg        string
	err        error
}

// Decompose is the top-level orchestrator for the multi-commit pipeline (PRD §13.6 / §11.4 / §9.14). It
// turns an un-staged, dirty working tree into an ordered sequence of logically-coherent commits by
// composing the four-role pipeline (planner → stager → message → arbiter).
//
// PRECONDITION (FR-M1, owned by the CLI router — P4.M1.T1.S1): the caller routed here because NOTHING is
// staged (HasStagedChanges false) AND the working tree has changes. Decompose does NOT re-check this; it
// assumes correct routing.
//
// MODE ROUTING (FR-M2): Config.Single==true || Config.Commits==1 → single ESCAPE-HATCH (planner bypassed
// → AddAll → generate.CommitStaged, v1 behavior). Else → callPlanner(forcedCount=Config.Commits). If the
// planner returns Single==true → FR-M11 single-SHORTCUT (use planner's message, dup-check first). Else →
// runLoop (1-deep overlap, N concepts).
//
// Error contract: planner failure + safety cap are NON-RESCUE (nothing snapshotted — §13.6.6); returned
// directly (NOT *RescueError). *generate.RescueError (message gen) and *generate.CASError (CAS) propagate
// DIRECTLY (errors.As-able). Other infra wraps ErrDecomposeFailed. SIGNAL-FREE for loop/shortcut in S1
// (signal arming is S2); the escape-hatch gets signal via CommitStaged internally.
//
// Decompose REQUIRES deps.Roles to be populated (by ResolveRoles, called by the CLI/P4 before Decompose).
// The optional deps.stager seam (nil in production) lets tests inject a staging-capable stager.
func Decompose(ctx context.Context, deps Deps) (DecomposeResult, error) {
	// (1) Mode routing: single ESCAPE-HATCH (planner bypassed) → v1 path.
	if deps.Config.Single || deps.Config.Commits == 1 {
		return runSingleEscape(ctx, deps)
	}

	// (2) Derive isUnborn + preRunHEAD + baseTree ONCE (callPlanner + the loop both need them).
	preRunHEAD, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil {
		return DecomposeResult{}, fmt.Errorf("%w: rev-parse head: %w", ErrDecomposeFailed, err)
	}
	baseTree := git.EmptyTreeSHA
	if !isUnborn {
		baseTree, err = deps.Git.RevParseTree(ctx, "HEAD")
		if err != nil {
			return DecomposeResult{}, fmt.Errorf("%w: rev-parse head^{tree}: %w", ErrDecomposeFailed, err)
		}
	}

	// (3) Planner (forcedCount = Config.Commits: 0=auto, ≥2=forced; ==1 caught above).
	//     NON-RESCUE on error. callPlanner ALREADY enforces the FR-M4 safety cap in auto mode.
	out, err := callPlanner(ctx, deps, deps.Config.Commits, isUnborn)
	if err != nil {
		return DecomposeResult{}, err // ErrPlannerFailed wrap OR safety-cap error — both non-rescue (§G-PLANNER-NONRESCUE)
	}

	// (4) FR-M11 single-SHORTCUT: planner judged N=1 + supplied a message.
	if out.Single {
		return runSingleShortcut(ctx, deps, out.Message, preRunHEAD, isUnborn, baseTree)
	}

	// (5) Safety cap is enforced inside callPlanner (auto mode). Forced mode: user asserted N — no cap.

	// (6) The loop (1-deep overlap, FR-M8 empty-skip, serialized CAS, FR-M12 isolation).
	commits, chainData, err := runLoop(ctx, deps, out.Commits, baseTree, preRunHEAD, isUnborn)
	if err != nil {
		// FR-M12: partial failures (rescue/CAS) AND hard failures return the partial commits that
		// already landed (0..i-1). The arbiter does NOT run on a loop abort (§18.3).
		return DecomposeResult{Commits: commits}, err
	}

	// (7)+(8) Arbiter gate: StatusPorcelain != "" → runArbiter → resolveArbiter.
	// Happy path ONLY: the arbiter does NOT run on loop abort (rescue/CAS/hard error) — §18.3.
	amended := 0
	status, err := deps.Git.StatusPorcelain(ctx)
	if err != nil {
		return DecomposeResult{}, fmt.Errorf("%w: status: %w", ErrDecomposeFailed, err)
	}
	if status != "" && len(commits) > 0 {
		// Build []CommitInfo from the loop's published commits (parallel to chainData).
		arbiterCommits := buildArbiterCommits(ctx, deps, commits, isUnborn)
		amended, err = runArbiterPhase(ctx, deps, arbiterCommits, chainData)
		if err != nil {
			return DecomposeResult{}, err // resolveArbiter errors propagated (incl. *RescueError/*CASError)
		}
		// §G-RESULT gap closed: re-read the FINAL commits (post-arbiter) for accurate, resolvable SHAs.
		finalCommits, rerr := rereadFinalCommits(ctx, deps, preRunHEAD, isUnborn)
		if rerr != nil {
			// Best-effort: the commits are already published. Log and keep the loop's pre-arbiter commits.
			deps.Verbose.VerboseRawOutput(fmt.Sprintf("decompose: reread final commits failed (best-effort, keeping loop commits): %v", rerr))
		} else {
			commits = finalCommits
		}
	}

	// (9) Return.
	return DecomposeResult{Commits: commits, Amended: amended}, nil
}

// runSingleEscape is the v1 single-commit path (Config.Single || Commits==1): planner is BYPASSED
// entirely. AddAll → generate.CommitStaged. CommitStaged GENERATES its own message + arms signal
// internally. Its typed errors (ErrNothingToCommit / *RescueError / *CASError) propagate verbatim.
func runSingleEscape(ctx context.Context, deps Deps) (DecomposeResult, error) {
	if err := deps.Git.AddAll(ctx); err != nil {
		return DecomposeResult{}, fmt.Errorf("%w: add -A: %w", ErrDecomposeFailed, err)
	}
	res, err := generate.CommitStaged(ctx, generate.Deps{
		Git:      deps.Git,
		Manifest: deps.Roles.Message, // the bare message role == the §13.1–§13.5 agent
		Verbose:  deps.Verbose,
	}, deps.Config)
	if err != nil {
		return DecomposeResult{}, err // typed errors propagated verbatim (errors.As-able)
	}
	return DecomposeResult{
		Commits: []CommitResult{{
			SHA: res.CommitSHA, Subject: res.Subject, Message: res.Message, Files: res.Changes,
		}},
		Amended: 0,
	}, nil
}

// runSingleShortcut (FR-M11): the planner ALREADY ran and returned Single==true + a Message. Use the
// planner's message DIRECTLY (AddAll → WriteTree → dup-check → publish), dup-checking it first. If it's
// a duplicate, fall back to generateMessage (the message agent) to regenerate. ZERO separate
// message-agent call on a clean subject (the shortcut's whole point — one agent round-trip).
// Distinct from the escape-hatch (which bypasses the planner and regenerates via CommitStaged).
// SIGNAL-FREE in S1.
func runSingleShortcut(ctx context.Context, deps Deps, plannerMsg, preRunHEAD string, isUnborn bool, baseTree string) (DecomposeResult, error) {
	if err := deps.Git.AddAll(ctx); err != nil {
		return DecomposeResult{}, fmt.Errorf("%w: add -A: %w", ErrDecomposeFailed, err)
	}
	treePrime, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return DecomposeResult{}, fmt.Errorf("%w: write-tree: %w", ErrDecomposeFailed, err)
	}

	// Dup-check the planner's message. Fallback to the message agent ONLY on a duplicate (FR-M11).
	msg := plannerMsg
	if dupCheckMessage(ctx, deps, plannerMsg, isUnborn) {
		msg, err = generateMessage(ctx, deps, baseTree, treePrime) // the message agent regenerates
		if err != nil {
			return DecomposeResult{}, err // *RescueError — propagate DIRECTLY
		}
	}

	// Publish (parentSHA = preRunHEAD; root if unborn). publishCommit returns *CASError DIRECTLY on CAS.
	newSHA, err := publishCommit(ctx, deps, treePrime, preRunHEAD, msg)
	if err != nil {
		return DecomposeResult{}, err
	}
	cr, err := buildCommitResult(ctx, deps, newSHA, msg, isUnborn)
	if err != nil {
		return DecomposeResult{}, err
	}
	return DecomposeResult{Commits: []CommitResult{cr}, Amended: 0}, nil
}

// runLoop drives the per-concept pipeline with 1-DEEP overlap (stager[i+1] ∥ message[i]) + FR-M8
// empty-skip + serialized CAS publication + FR-M12 per-concept failure isolation (PRD §13.6.3/§13.6.6).
// It returns the ordered []CommitResult (oldest first) + the parallel []ChainEntry (for resolveArbiter).
// On any error it returns the PARTIAL commits that already landed (0..i-1) — they are real and stand.
//
// Algorithm (§13.6.3 + FR-M12): for each concept i — stage[i] with retry-once-then-empty (msg[i-1] in
// flight) → freeze tree[i] → FR-M8 skip check → drain+publish msg[i-1] (FR-M12a: *RescueError →
// FormatRescueMulti + *DecomposeRescueError; FR-M12b: *CASError → ce.Error() + abort) → launch msg[i]
// with signal armed (SetSnapshot). Final: drain+publish msg[N-1].
//
// Safety: message[i] uses diff(prevTree, tree[i]) — frozen, immune to the live index (§13.6.3 inv. 2).
// Publication is strictly ordered (CAS chain). Channels are buffered(1) so goroutines never block on send.
// On ANY error, drain the in-flight channel (<-ch) before returning (no leak). Signal uses SetSnapshot/
// ClearSnapshot toggling (NOT RestoreDefault — one-shot+permanent, §G-RESTOREDEFAULT-ONESHOT).
func runLoop(ctx context.Context, deps Deps, concepts []prompt.PlannerCommit, baseTree, preRunHEAD string, isUnborn bool) ([]CommitResult, []ChainEntry, error) {
	var commits []CommitResult
	var chainData []ChainEntry
	prevTree := baseTree
	prevSHA := preRunHEAD // CAS expected-old + parent for the next commit

	// launch runs generateMessage for one concept in a goroutine, returning a buffered result channel.
	launch := func(i int, treeA, treeB string) chan msgOut {
		ch := make(chan msgOut, 1) // buffered(1) — goroutine sends once + exits; never blocks
		go func() {
			m, e := generateMessage(ctx, deps, treeA, treeB)
			ch <- msgOut{conceptIdx: i, treeA: treeA, treeB: treeB, msg: m, err: e}
		}()
		return ch
	}

	// publish drains a message channel + publishes the commit in order. Returns the newSHA + updated chain.
	// FR-M12a: catches *RescueError → prints FormatRescueMulti + returns *DecomposeRescueError.
	// FR-M12b: catches *CASError → prints ce.Error() + returns ce.
	publish := func(ch chan msgOut) error {
		if ch == nil {
			return nil
		}
		res := <-ch
		signal.ClearSnapshot() // disarm before the CAS (§18.4 analog; RestoreDefault is one-shot — see G-RESTOREDEFAULT)
		if res.err != nil {
			var re *generate.RescueError
			if errors.As(res.err, &re) {
				// FR-M12a: message[i] failed → rescue for concept i ONLY. re.ParentSHA == newSHA[i-1]
				// (generateMessage captured RevParseHEAD after commit[i-1] landed). Print the §18.3
				// multi-commit variant (names the concept + position). The overlapped stager[i+1] has
				// already completed (synchronous-before-drain) → its staging stays in the index.
				title := ""
				if res.conceptIdx < len(concepts) {
					title = concepts[res.conceptIdx].Title
				}
				if deps.Out != nil {
					fmt.Fprintln(deps.Out, generate.FormatRescueMulti(re.TreeSHA, re.ParentSHA, re.Candidate, title, res.conceptIdx, len(concepts)))
				}
				return &DecomposeRescueError{Rescue: re, ConceptTitle: title, Index: res.conceptIdx, Count: len(concepts), Commits: commits}
			}
			return res.err // HARD (ErrMessageFailed-wrapped infra) — propagate
		}
		newSHA, err := publishCommit(ctx, deps, res.treeB, prevSHA, res.msg) // parentSHA = prevSHA (CAS expected-old)
		if err != nil {
			var ce *generate.CASError
			if errors.As(err, &ce) {
				// FR-M12b: CAS failed → §13.5 message (ce.Error() has tree[i] recovery). Prior commits stand.
				if deps.Out != nil {
					fmt.Fprintln(deps.Out, ce.Error())
				}
				return ce // partial; DecomposeResult.Commits = commits (0..i-1)
			}
			return err // HARD (ErrPublicationFailed-wrapped CommitTree)
		}
		isRoot := res.conceptIdx == 0 && isUnborn
		cr, err := buildCommitResult(ctx, deps, newSHA, res.msg, isRoot)
		if err != nil {
			return fmt.Errorf("%w: diff-tree[%d]: %w", ErrDecomposeFailed, res.conceptIdx, err)
		}
		commits = append(commits, cr)
		chainData = append(chainData, ChainEntry{SHA: newSHA, Tree: res.treeB, Message: res.msg, Parent: prevSHA})
		prevSHA = newSHA
		return nil
	}

	// invokeStagerRetry: FR-M12d — stager exits non-zero → retry once; on second failure treat as empty
	// (return nil → fall through to freezeSnapshot → tree[i]==prevTree → S1's empty-skip). Logs via Verbose.
	// A cancelled ctx propagates (abort the loop) so the run doesn't silently skip every remaining concept.
	// Issue 2 / PRD §19 defense-in-depth: snapshot HEAD before each stager call; abort (HARD, non-rescue)
	// if any stager call moves it. The guard bypasses retry-once-then-empty via errors.Is short-circuits.
	invokeStagerRetry := func(concept prompt.PlannerCommit) error {
		if cerr := ctx.Err(); cerr != nil {
			return cerr // ctx cancelled/moved on → abort (drainMsg + return partial), not skip-everything
		}
		// Issue 2 / PRD §19 defense-in-depth: a correctly-behaving stager mutates the INDEX only,
		// never refs. Snapshot HEAD once; abort (HARD, non-rescue) if any stager call moves it.
		preStagerHEAD, _, _ := deps.Git.RevParseHEAD(ctx)
		// runOnce invokes the stager once and aborts if HEAD moved during that call.
		runOnce := func() error {
			serr := invokeStager(ctx, deps, concept)
			postStagerHEAD, _, _ := deps.Git.RevParseHEAD(ctx)
			if preStagerHEAD != postStagerHEAD {
				return fmt.Errorf("%w: stager moved HEAD from %s to %s — aborting; the stager agent mutated refs which it must not do", ErrStagerMovedHEAD, preStagerHEAD, postStagerHEAD)
			}
			return serr
		}
		if err := runOnce(); err == nil {
			return nil
		} else if errors.Is(err, ErrStagerMovedHEAD) {
			return err // HARD — safety violation; do NOT retry, do NOT empty-skip
		}
		deps.Verbose.VerboseRetry(1, fmt.Sprintf("stager failed for %q; retrying once", concept.Title))
		if err2 := runOnce(); err2 == nil {
			return nil
		} else if errors.Is(err2, ErrStagerMovedHEAD) {
			return err2 // HARD — safety violation even on the retry
		}
		deps.Verbose.VerboseRetry(2, fmt.Sprintf("stager failed twice for %q; treating concept as empty (FR-M8)", concept.Title))
		return nil // empty: freezeSnapshot will yield tree[i]==prevTree → S1's empty-skip
	}

	var inflight chan msgOut // the in-flight message goroutine (concept i-1); nil if none
	for i, concept := range concepts {
		// FR-M12d: stager retry-once-then-empty (replaces S1's single invokeStager + propagate).
		if err := invokeStagerRetry(concept); err != nil {
			drainMsg(inflight) // ctx cancellation (only non-nil return) — abort; partial commits stand
			return commits, nil, err
		}
		treeI, err := freezeSnapshot(ctx, deps)
		if err != nil {
			drainMsg(inflight)
			return commits, nil, fmt.Errorf("%w: freeze snapshot[%d]: %w", ErrDecomposeFailed, i, err)
		}

		// FR-M8 empty-skip: stager staged nothing new → skip commit i (no message, no publish).
		// Also the twice-failed-stager path (FR-M12d): invokeStagerRetry returned nil → nothing staged → tree[i]==prevTree.
		skipped := treeI == prevTree

		// Publish the PREVIOUS concept's commit (drain msg[i-1]) — serialized, in order.
		if err := publish(inflight); err != nil {
			return commits, nil, err
		}
		inflight = nil

		// Launch msg[i] (overlaps stage[i+1] in the NEXT iteration) unless this concept was skipped.
		if !skipped {
			signal.SetSnapshot(treeI, prevSHA, "") // arm rescue during msg[i] (§18.4; nil-safe without Install)
			inflight = launch(i, prevTree, treeI)
			prevTree = treeI
		}
		// skipped: prevTree unchanged (== treeI); no message launched.
	}

	// Drain + publish the final pending message.
	if err := publish(inflight); err != nil {
		return commits, nil, err
	}
	return commits, chainData, nil
}

// drainMsg receives-and-discards a buffered(1) message channel's result to avoid a goroutine leak when
// the loop aborts with a message goroutine in flight. The goroutine sends exactly once to the buffered
// channel before exiting, so the receive is guaranteed to complete. nil-safe.
func drainMsg(ch chan msgOut) {
	if ch == nil {
		return
	}
	<-ch
}

// runArbiterPhase runs the arbiter + resolution when the working tree is non-empty after the loop
// (PRD §13.6.5 / FR-M9). Returns the arbiter's rewrite COUNT for DecomposeResult.Amended (0 if the
// arbiter made a new commit; 1 for tip amend; N-i for mid-chain at index i). resolveArbiter returns
// ONLY an error; the count is computed from the target via findTargetIndex (same package) BEFORE calling
// resolveArbiter.
// rereadFinalCommits re-reads the FINAL commits this run produced (post-arbiter) by listing the range
// preRunHEAD..HEAD via LogRange and pairing each entry's SHA with DiffTree, rebuilding accurate
// []CommitResult for DecomposeResult.Commits. It closes the §G-RESULT post-arbiter gap: after the
// arbiter amends/rebuilds/creates, the loop's pre-arbiter SHAs are stale (dangling) and the null-path
// (N+1)-th commit is missing from the loop's slice.
//
// baseSHA: preRunHEAD (captured at Decompose step (2) BEFORE the loop/arbiter mutated HEAD), or the
// all-zeros sentinel strings.Repeat("0",40) when the repo was originally unborn (isUnborn) — LogRange
// branches the all-zeros sentinel to the no-range HEAD form (lists ALL commits created this run).
//
// isRoot (per DiffTree): true ONLY for the FIRST entry when isUnborn (concept 0 is the repo's root
// commit). Message is set to "" — printDecomposeCommit prints only SHA+Subject+Files (the full message
// is not part of the success report), so it is not re-fetched.
//
// Best-effort: callers log a non-nil error and fall back to the loop's pre-arbiter commits rather than
// failing the run (the commits are already published; stale SHAs beat erroring). Returns (nil, err) on
// any git read failure (the caller decides the fallback).
func rereadFinalCommits(ctx context.Context, deps Deps, preRunHEAD string, isUnborn bool) ([]CommitResult, error) {
	baseSHA := preRunHEAD
	if isUnborn {
		baseSHA = strings.Repeat("0", 40) // LogRange's all-zeros unborn sentinel → no-range HEAD form
	}
	entries, err := deps.Git.LogRange(ctx, baseSHA)
	if err != nil {
		return nil, fmt.Errorf("%w: log range: %w", ErrDecomposeFailed, err)
	}
	out := make([]CommitResult, 0, len(entries))
	for i, entry := range entries {
		isRoot := isUnborn && i == 0 // concept 0 is the root commit only on an originally-unborn repo
		files, err := deps.Git.DiffTree(ctx, entry.SHA, isRoot)
		if err != nil {
			return nil, fmt.Errorf("%w: diff-tree %s: %w", ErrDecomposeFailed, entry.SHA, err)
		}
		out = append(out, CommitResult{SHA: entry.SHA, Subject: entry.Subject, Message: "", Files: files})
	}
	return out, nil
}

func runArbiterPhase(ctx context.Context, deps Deps, commits []CommitInfo, chainData []ChainEntry) (int, error) {
	// Build the arbiter input: the leftover diff.
	leftoverDiff, err := deps.Git.WorkingTreeDiff(ctx, git.StagedDiffOptions{
		MaxDiffBytes:     deps.Config.MaxDiffBytes,
		MaxMDLines:       deps.Config.MaxMdLines,
		BinaryExtensions: deps.Config.BinaryExtensions,
	})
	if err != nil {
		return 0, fmt.Errorf("%w: leftover diff: %w", ErrDecomposeFailed, err)
	}

	out, err := runArbiter(ctx, deps, commits, leftoverDiff)
	if err != nil {
		return 0, err // ErrArbiterFailed wrap (render error) — rare
	}

	amended := computeAmended(out.Target, chainData) // BEFORE resolveArbiter (it doesn't return the count)

	if err := resolveArbiter(ctx, deps, out.Target, commits, chainData); err != nil {
		return 0, err // *RescueError / *CASError / ErrArbiterResolutionFailed — propagated
	}
	return amended, nil
}

// buildArbiterCommits builds []CommitInfo from the loop's []CommitResult for the arbiter input.
// Each entry carries SHA + ExtractSubject(msg) + DiffTree(newSHA, isRoot) files. The DiffTree call
// reads the commit object — it is safe after the loop (HEAD == newSHA[last], all commits are in the
// object store). isRoot is true ONLY for concept 0 on an unborn repo.
func buildArbiterCommits(ctx context.Context, deps Deps, commits []CommitResult, isUnborn bool) []CommitInfo {
	info := make([]CommitInfo, len(commits))
	for i, cr := range commits {
		info[i] = CommitInfo{
			SHA:     cr.SHA,
			Subject: cr.Subject,
			Files:   cr.Files,
		}
	}
	return info
}

// computeAmended returns the arbiter's rewrite count from its target decision: nil → 0 (new commit);
// tip (last entry) → 1; earlier commit at index i → N-i; not-found → 0 (defensive null). Uses
// findTargetIndex (chain.go, same package). See §G-RESULT.
func computeAmended(target *string, chainData []ChainEntry) int {
	if target == nil {
		return 0
	}
	N := len(chainData)
	idx := findTargetIndex(*target, chainData)
	if idx < 0 {
		return 0 // defensive: not-found → treat as null/new
	}
	if idx == N-1 {
		return 1 // tip amend
	}
	return N - idx // mid-chain rebuild at index idx
}

// buildCommitResult builds a CommitResult from a published commit (DiffTree for the FR42 file-list).
// isRoot is true ONLY for concept 0 on an unborn repo.
func buildCommitResult(ctx context.Context, deps Deps, sha, msg string, isRoot bool) (CommitResult, error) {
	files, err := deps.Git.DiffTree(ctx, sha, isRoot)
	if err != nil {
		return CommitResult{}, err
	}
	return CommitResult{SHA: sha, Subject: generate.ExtractSubject(msg), Message: msg, Files: files}, nil
}

// dupCheckMessage reports whether msg's subject exactly matches one of the last 50 commit subjects
// (FR32-style). nil/vacuous on an unborn repo (no dup possible). Reuses generate.IsDuplicate +
// generate.ExtractSubject (same as generateMessage internally) for consistency.
func dupCheckMessage(ctx context.Context, deps Deps, msg string, isUnborn bool) bool {
	if isUnborn {
		return false
	}
	recent, err := deps.Git.RecentSubjects(ctx, 50)
	if err != nil {
		return false // best-effort: treat a read failure as "not a duplicate"
	}
	return generate.IsDuplicate(generate.ExtractSubject(msg), recent)
}

// invokeStager is the test seam: deps.stager if non-nil (tests inject a git-staging stager), else the
// real package-level stageConcept (the tooled agent). Production builds Deps without deps.stager (nil).
func invokeStager(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error {
	if deps.stager != nil {
		return deps.stager(ctx, deps, concept)
	}
	return stageConcept(ctx, deps, concept)
}
