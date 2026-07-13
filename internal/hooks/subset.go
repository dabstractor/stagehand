// Package hooks implements the scoped commit-hook policy layer (PRD §9.25 FR-V3).
// The git primitives the policy consumes live in internal/git; THIS package owns the
// "what mutations a hook is permitted to make" decision (the FR-V3 freeze backstop) and,
// in P1.M3.T1, the hook runner that drives the scoped pre-commit sequence.
package hooks

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dabstractor/stagecoach/internal/git"
)

// ErrHookSweptConcurrentWork is the sentinel for an FR-V3 FREEZE violation by a scoped pre-commit hook:
// the hook's output tree contains a path NOT present in the snapshot tree (an added file, or the
// destination of a rename/copy) — i.e. the hook tried to sweep concurrent/unstaged work into the commit.
//
// This is the hook-path twin of decompose.ErrFreezeViolation (the FR-M1c stager freeze guard): both are
// content-axis, NON-RESCUE hard errors (detected before commit-tree; HEAD and the index are untouched).
// Wrapped with %w so errors.Is(err, ErrHookSweptConcurrentWork) is true. Produced by enforceSubset.
var ErrHookSweptConcurrentWork = errors.New("hooks: pre-commit swept concurrent work")

// enforceSubset verifies the FR-V3 freeze backstop: postTree's path set must be a SUBSET of snapshotTree's
// (the hook introduced NO new paths). It is the "subset check" step in the scoped pre-commit sequence
// (external_deps.md §8): after ReadTreeInto(snapshot, tmp) → pre-commit → WriteTreeFrom(tmp) = postTree,
// the runner calls enforceSubset(snapshot, postTree). nil ⇒ the hook's mutations are permitted (M/D/T of
// existing snapshot paths); the caller uses postTree as the commit's tree (re-tree, git-commit parity).
// ErrHookSweptConcurrentWork ⇒ the hook added a path not in the snapshot (would sweep concurrent work in);
// the caller aborts the run (FR-V7 rescue state — no update-ref ran).
//
// The check uses DiffTreeNameStatus(snapshot, post), which runs WITHOUT -M/-C: an 'A' status line is,
// BY DEFINITION, a path in postTree not in snapshotTree (a subset violation). 'M'/'D'/'T' (modify/delete/
// typechange of an existing snapshot path) do NOT violate the subset and are permitted. A rename by the
// hook appears as D+A (no -M ⇒ no R line), so the 'A' correctly fires (a rename stages a new path).
// Defensively, 'C'/'R' status letters (which would only appear if -M/-C were ever added) are also flagged;
// the offending path is the LAST tab-field of the status line (the new/destination path).
//
// A git failure from DiffTreeNameStatus (e.g. a bad tree SHA) is a wrapped NON-sentinel error (not a sweep).
//
// Re-tree decision (the CALLER's one-liner — NOT in this function):
//
//	tree := snapshotTree
//	if postTree != snapshotTree {
//	    if err := enforceSubset(ctx, g, snapshotTree, postTree); err != nil { return err }
//	    tree = postTree // permitted mutation → re-tree (git-commit parity)
//	}
func enforceSubset(ctx context.Context, g git.Git, snapshotTree, postTree string) error {
	nameStatus, err := g.DiffTreeNameStatus(ctx, snapshotTree, postTree)
	if err != nil {
		return fmt.Errorf("hook subset check: diff-tree-name-status: %w", err)
	}
	var added []string
	for _, line := range strings.Split(nameStatus, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		status := fields[0]
		if len(status) == 0 {
			continue
		}
		// 'A' = added (not in snapshot); 'C'/'R' = copy/rename destination (defensive — no -M/-C today).
		// 'M'/'D'/'T' (modify/delete/typechange of an existing snapshot path) are permitted.
		switch status[0] {
		case 'A', 'C', 'R':
			added = append(added, fields[len(fields)-1]) // the new/destination path (last tab-field)
		}
	}
	if len(added) > 0 {
		return fmt.Errorf("%w: pre-commit staged a path not in the snapshot: %s — refusing to sweep concurrent work into the commit (FR-V3)",
			ErrHookSweptConcurrentWork, strings.Join(added, ", "))
	}
	return nil
}
