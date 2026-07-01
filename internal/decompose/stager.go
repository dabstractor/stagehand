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
//     --cached); it NEVER commits, amends, or moves refs (stagehand owns all ref mutations).
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

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/prompt"
	"github.com/dustin/stagehand/internal/provider"
)

// ErrStagerFailed is the sentinel for stager-agent failures (render error, non-zero exit, timeout,
// cancellation). It is wrapped (%w) around the underlying cause so errors.Is works. The orchestrator
// (P3.M4.T1.S1) detects stager failures via errors.Is to apply the FR-M8/M12 retry-once-then-empty.
//
// Non-rescue: stageConcept mutates the INDEX only (the agent runs git add / git apply --cached); it
// NEVER commits, amends, or moves refs (stagehand owns all ref mutations — §13.6.2/§19). Its
// failures are NOT generate.RescueError scenarios (no snapshot-then-CAS here; refs move only at
// P3.M2.T4's UpdateRefCAS).
var ErrStagerFailed = errors.New("decompose: stager failed")

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
	// 1. Derive the stager (provider, model) — Deps has no Models field.
	prov, mdl := config.ResolveRoleModel("stager", deps.Config)

	// 2. Build the §17.6 stager task from the concept's title + description.
	task := prompt.BuildStagerTask(concept.Title, concept.Description)

	// 3. Render the stager manifest in TOOLED mode (system prompt empty — the task IS the payload).
	spec, rerr := deps.Roles.Stager.Render(mdl, prov, "", task, provider.RenderTooled)
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
