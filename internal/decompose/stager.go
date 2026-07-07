// Package decompose implements the multi-commit decomposition pipeline (PRD §13.6.2): given an
// un-staged working tree, it produces N logically-coherent commits by running a four-agent pipeline
// (planner → stager → message → arbiter) with per-role provider/model resolution.
//
// This file (stager.go) implements the stager half of the pipeline (PRD §13.6.2 / §13.6.3,
// §9.14 FR-M5/M6/M8):
//
//   - stageConcept: the TOOLED, no-retry, no-parse stager invocation. It is the tooled
//     counterpart of callPlanner (RenderTooled instead of RenderBare; no retry loop — the
//     orchestrator owns FR-M8 retry-once-then-empty; no output parsing — the stager has no JSON
//     contract; the index is the truth source). It mutates the INDEX only (git add / git apply
//     --cached); it NEVER commits, amends, or moves refs (stagecoach owns all ref mutations).
//   - freezeSnapshot: the §13.6.3 invariant-1 primitive. A thin, documented wrapper over
//     deps.Git.WriteTree that freezes the current index into an immutable tree SHA. The
//     orchestrator calls it synchronously after stageConcept returns and BEFORE the next
//     stageConcept starts — whatever the next stageConcept does to the live index afterward
//     CANNOT reach the frozen tree.
//
// Consumed by the orchestrator (P3.M4.T1.S1); no caller wiring in this file.
package decompose

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/prompt"
	"github.com/dustin/stagecoach/internal/provider"
)

// ErrStagerFailed is the sentinel for stager-agent failures (render error, non-zero exit, timeout,
// cancellation). It is wrapped (%w) around the underlying cause so errors.Is works. The orchestrator
// (P3.M4.T1.S1) detects stager failures via errors.Is to apply the FR-M8/M12 retry-once-then-empty.
//
// Non-rescue: stageConcept mutates the INDEX only (the agent runs git add / git apply --cached); it
// NEVER commits, amends, or moves refs (stagecoach owns all ref mutations — §13.6.2/§19). Its
// failures are NOT generate.RescueError scenarios (no snapshot-then-CAS here; refs move only at
// P3.M2.T4's UpdateRefCAS).
var ErrStagerFailed = errors.New("decompose: stager failed")

// ErrStagerMovedHEAD is the sentinel for a stager SAFETY VIOLATION: the stager agent moved HEAD
// (committed, amended, update-ref'd, or reset a ref). The stager is contractually allowed to mutate
// the INDEX only (git add / git apply --cached); all ref mutations are owned exclusively by stagecoach
// via UpdateRefCAS (PRD §18.1/§19).
//
// This guard is DEFENSE-IN-DEPTH for providers that cannot be flag-scoped — specifically pi's
// tooled profile, which has no tool allowlist (Issue 2 / PRD §19). For such providers the stager
// constraint is instructionally enforced only; this runtime check provides the structural guarantee.
//
// This is a HARD (non-rescue) error: when the guard fires, there is no snapshot to restore — the stager
// corrupted repo state. The run aborts (exit 1). Contrast *generate.RescueError (exit 3) which IS
// rescuable because a snapshot exists.
//
// Produced by the HEAD pre/post-snapshot guard in invokeStagerRetry (decompose.go). Wrapped with %w
// so errors.Is(err, ErrStagerMovedHEAD) is true for test assertions and exit-code mapping.
var ErrStagerMovedHEAD = errors.New("decompose: stager moved HEAD")

// ErrFreezeViolation is the sentinel for an FR-M1c FREEZE-ENFORCEMENT violation (PRD §9.14/§13.6.1
// FR-M1c): the stager produced a tree[i] whose changed paths or blob contents are NOT traceable to
// T_start — a concurrent working-tree change the external stager swept in, or a mis-behaving stager
// that ran a bare `git add -A` staging a path or content not in T_start.
//
// This is the CONTENT-AXIS twin of ErrStagerMovedHEAD (the ref-axis guard): both are stager-safety
// sentinels in stager.go, both are produced by the orchestrator (decompose.go), both are HARD
// (non-rescue) errors. The orchestrator owns the freeze boundary, NOT the stager (PRD §19 / FR-M1c).
//
// NON-RESCUE: the violation is detected at tree[i] BEFORE its commit, so there is no
// snapshot-then-CAS to rescue. Already-landed commits 0..i-1 stand (returned as partial results),
// and the in-flight concept's staging remains in the index (FR-M12). Mirrors ErrStagerMovedHEAD's
// drainMsg+return-partial abort pattern.
//
// Produced by verifyFreezeSubset; wrapped with %w so errors.Is(err, ErrFreezeViolation) is true.
var ErrFreezeViolation = errors.New("decompose: freeze violation")

// stageConcept invokes the stager agent once (TOOLED, no retry) for a single concept from the
// planner's partition (PRD §13.6.2 / FR-M5). It is the tooled, no-retry, no-parse counterpart of
// callPlanner: the stager is the ONLY tooled role (RenderTooled), it mutates the INDEX only (never
// refs), and the caller (orchestrator) owns the FR-M8 retry-once-then-empty and the output parsing
// (there is none — the stager returns free-form text; the index is the truth source).
//
// Pipeline: derive stager model via ResolveRoleModel → build §17.6 stager task → Render in TOOLED
// mode (empty system prompt — the task IS the payload) → Execute once → return nil on success or
// ErrStagerFailed-wrapped error on any failure (render error, non-zero exit, timeout, cancel).
//
// The model is derived via config.ResolveRoleModel("stager", deps.Config) because Deps carries
// RoleManifests (merged-but-unresolved manifests) but no pre-resolved (provider, model) pairs —
// the orchestrator retains those separately (or derives them per-call, as here).
//
// NO retry loop: the orchestrator retries once then treats the concept as empty (FR-M8/M12).
// NO output parse: the stager has no JSON contract; the exit code is the signal.
func stageConcept(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error {
	// 1. Derive the <role> model — Deps has no Models field. (Provider is the manifest name; it is NOT
	// passed to Render — v3 FR-R5b folds the inference backend into the model slash-prefix.)
	_, mdl, rsn := config.ResolveRoleModel("stager", deps.Config)

	// 2. Build the §17.6 stager task from the concept's title + description.
	task := prompt.BuildStagerTask(concept.Title, concept.Description, concept.Files)

	// v3 FR-R5b: the inference provider is the model slash-prefix ("inference/model"),
	// which Render splits into --provider <inference>. P1.M2 wires real per-role reasoning
	// via ResolveRoleModel's 3rd return (rsn).
	spec, rerr := deps.Roles.Stager.Render(mdl, "", task, rsn, provider.RenderTooled)
	if rerr != nil {
		return fmt.Errorf("%w: render: %v", ErrStagerFailed, rerr)
	}

	// 4. Execute once. NO retry (the orchestrator owns FR-M8); NO parse (no JSON contract).
	if _, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose); execErr != nil {
		return fmt.Errorf("%w: %w", ErrStagerFailed, execErr)
	}

	// 5. Success — the agent mutated the index (git add / git apply --cached). The orchestrator
	//    freezes via freezeSnapshot BEFORE the next stageConcept (§13.6.3 invariant 1).
	return nil
}

// freezeSnapshot freezes the current git index into an immutable tree SHA (PRD §13.6.3 invariant 1:
// "tree[i] is frozen before stager[i+1] starts"). It is a thin wrapper over deps.Git.WriteTree:
// read-only with respect to refs and the index (WriteTree writes a tree object to the object
// store; it does NOT modify .git/index or HEAD — §13.2).
//
// The returned SHA is an IMMUTABLE record of the index at call time — whatever the next
// stageConcept does to the live index afterward CANNOT reach it. This is the safety basis for the
// overlapped pipeline (stager[i+1] ∥ message[i]): tree[i] is frozen before stager[i+1] mutates
// the index, so message[i] reasons over a stable tree-to-tree diff.
//
// The orchestrator MUST call this synchronously after stageConcept returns and BEFORE the next
// stageConcept starts. WriteTree fails (exit 128) on unresolved merge conflicts — the error
// propagates verbatim and aborts the run.
func freezeSnapshot(ctx context.Context, deps Deps) (string, error) {
	treeSHA, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return "", err
	}
	return treeSHA, nil
}

// verifyFreezeSubset verifies that tree[i] is a CONTENT-SUBSET of T_start (PRD §9.14/§13.6.1
// FR-M1c): every path changed in diff(baseTree, treeI) must be present in tStartPaths AND carry
// T_start's blob content. The orchestrator owns the freeze boundary — the external stager is NOT
// trusted. This is the content-axis enforcement sibling of the ErrStagerMovedHEAD ref guard.
//
// The two-part check uses ONLY DiffTreeNames (no new git primitive — findings §4):
//
//	(A) PATH:  DiffTreeNames(baseTree, treeI) ⊆ tStartPaths (every changed path is in T_start's set).
//	(B) CONTENT: changedTreeI ∩ DiffTreeNames(treeI, tStart) == ∅
//	           (equivalent to `git diff treeI tStart -- <changedTreeI>` empty, done via the
//	           intersection trick since the git.Git interface has no pathspec-restricted diff).
//
// Empty changedTreeI (empty staging / treeI == baseTree) ⇒ both checks trivially pass ⇒ no false
// positive on the FR-M8 empty-skip.
//
// Returns nil if the subset holds; ErrFreezeViolation-wrapped error naming the offending path(s)
// on a path-not-in-T_start or content-mismatch; ErrDecomposeFailed-wrapped error on a DiffTreeNames
// git failure. The caller (runLoop) owns drainMsg+return-partial on violation.
func verifyFreezeSubset(ctx context.Context, deps Deps, baseTree, tStart string, tStartPaths []string, i int, treeI string) error {
	// (A) PATH check: tree[i]'s changed paths must all be in T_start's changed set.
	changedTreeI, err := deps.Git.DiffTreeNames(ctx, baseTree, treeI)
	if err != nil {
		return fmt.Errorf("%w: freeze check diff-tree-names[%d]: %w", ErrDecomposeFailed, i, err)
	}
	tStartSet := pathSet(tStartPaths)
	var extra []string
	for _, p := range changedTreeI {
		if _, ok := tStartSet[p]; !ok {
			extra = append(extra, p)
		}
	}
	if len(extra) > 0 {
		return fmt.Errorf("%w: concept %d staged paths not present in T_start: %s",
			ErrFreezeViolation, i, strings.Join(extra, ", "))
	}

	// (B) CONTENT check: tree[i]'s changed paths must carry T_start's blob content.
	//     changedTreeI ∩ DiffTreeNames(treeI, tStart) == ∅ isolates the changed paths whose
	//     content differs from T_start (proven equivalent to the contract's path-restricted
	//     `git diff treeI tStart -- <changed paths>` without needing a pathspec).
	delta, err := deps.Git.DiffTreeNames(ctx, treeI, tStart)
	if err != nil {
		return fmt.Errorf("%w: freeze check diff-tree-names[%d]: %w", ErrDecomposeFailed, i, err)
	}
	deltaSet := pathSet(delta)
	var mismatch []string
	for _, p := range changedTreeI {
		if _, ok := deltaSet[p]; ok {
			mismatch = append(mismatch, p)
		}
	}
	if len(mismatch) > 0 {
		return fmt.Errorf("%w: concept %d staged content not traceable to T_start: %s",
			ErrFreezeViolation, i, strings.Join(mismatch, ", "))
	}
	return nil
}

// pathSet builds a set from a path slice for membership lookups.
func pathSet(paths []string) map[string]struct{} {
	s := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		s[p] = struct{}{}
	}
	return s
}
