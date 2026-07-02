// This file adds the two staging PRIMITIVES that feed the CLI layer's
// auto-stage path and the empty-diff check: HasStagedChanges and AddAll
// (P1.M3.T4.S2). They are thin methods over the shipped, unexported [Git.run]
// exec seam against the REAL git binary (PRD §22.3: no go-git; §19: no sh -c
// — the --cached/--quiet/-A flags are passed as literal args, never
// interpolated into a shell string). Each method builds a []string of args,
// calls g.run, and HasStagedChanges routes the exit code via errors.As into
// the typed *[ExitError] (Code 1 = staged, Code 0 = clean). It uses a plain
// "package git" line because [git.go] OWNS the package doc comment, mirroring
// how plumbing.go/diff.go/log.go defer to git.go.
//
// These are PRIMITIVES ONLY — staging POLICY (auto-stage-all, the
// exit-2-if-still-clean path, the --all/--no-auto-stage flag wiring) lives in
// the CLI layer (P1.M7.T2), NOT here (decisions.md §1: the core is
// CommitStaged(ctx, cfg) that ASSUMES the index is already staged; staging
// policy is decided in main = maybeAutoStage(); CommitStaged()).
package git

import "errors"

// HasStagedChanges reports whether the index holds staged (index-vs-HEAD)
// changes — the empty-check primitive the CLI layer's auto-stage path and the
// FR5 nothing-staged gate consume (PRD §22.3: shell out to the real git
// binary, no go-git; §19: no sh -c; FR5: the empty-diff nothing-staged path;
// FR16: "if `git diff --cached --quiet` reports no staged changes ...";
// FR20: the --all/-a force-stage path; decisions.md §1: primitive only — the
// CLI layer decides WHEN to call this and what to do on a still-clean index).
//
// It runs `git diff --cached --quiet` (the --cached/--quiet flags are literal
// args — PRD §19). git's --quiet flag implies --exit-code, so git deliberately
// uses the EXIT STATUS as a boolean rather than emitting a diff:
//   - exit 0 (run returns a nil error) → the index MATCHES HEAD → there are
//     NO staged changes → returns (false, nil). NOTE this is git's
//     --exit-code convention ("exit 0 means no differences") and is the
//     CORRECTED, empirically-verified semantics — the method name
//     "HasStagedChanges" must return false when nothing is staged.
//   - exit 1 (typed *[ExitError] with Code==1, detected via errors.As) →
//     there ARE staged differences → returns (true, nil). Exit 1 is git's
//     normal "differences present" signal, NOT an error: it is routed to
//     (true, nil) the way plumbing.go's [Git.RevParseHEAD] routes Code 128 to
//     hasParent=false rather than treating every non-zero exit as a failure.
//   - any OTHER non-zero exit (e.g. "not a git repository") → a genuine
//     failure, surfaced as-is as the typed *[ExitError] so the caller can
//     route it.
//
// An UNBORN / rootless repo needs NO special handling here (unlike
// [Git.CommitCount]/[Git.RecentMessages], which detect "unknown revision" /
// "does not have any commits yet"): `git diff --cached --quiet` on an unborn
// repo exits 0 (clean) or 1 (staged) just like a committed repo, so stage.go
// has no unborn special-case at all.
func (g *Git) HasStagedChanges() (bool, error) {
	_, err := g.run("diff", "--cached", "--quiet")
	if err == nil {
		// exit 0: the index matches HEAD → CLEAN → no staged changes.
		return false, nil
	}
	var ee *ExitError
	if errors.As(err, &ee) && ee.Code == 1 {
		// exit 1: git's --exit-code "differences present" signal → STAGED. It
		// is NOT an error; route it to (true, nil) the way RevParseHEAD routes
		// Code 128 to a normal non-error signal.
		return true, nil
	}
	// Any other non-zero exit (e.g. not-a-repo): a genuine failure — surface
	// the typed *ExitError as-is.
	return false, err
}

// AddAll stages new + modified + deleted files across the whole worktree —
// the bare `git add -A` primitive the CLI layer's auto-stage path consumes
// (PRD §22.3: shell out to the real git binary, no go-git; §19: no sh -c;
// FR16: the auto-stage-all path's `git add -A` step; FR20: the --all/-a
// force-stage step; decisions.md §1: primitive only — the CLI layer decides
// WHEN to call it and what to do on a still-clean index; this method does NOT
// read config or implement any staging POLICY).
//
// It runs `git add -A` (the -A flag is a literal arg — PRD §19). git's -A /
// --all updates the index to match the worktree across added, modified, AND
// removed entries. `git add -A` exits 0 even when there is nothing to stage
// (a clean worktree), so "nothing to add" is NOT an error: on success it
// returns nil; on a non-zero exit it returns the typed *[ExitError]. It needs
// no exit-code routing.
func (g *Git) AddAll() error {
	_, err := g.run("add", "-A")
	return err
}
