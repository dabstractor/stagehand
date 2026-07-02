// This file adds the four snapshot-based atomic-commit plumbing primitives
// that underpin the generate commit step (P1.M6.T1.S1) and the §18.1 safety
// invariant. They are thin methods over the shipped, unexported [Git.run] exec
// seam against the REAL git binary (PRD §22.3/§19: no go-git, no sh -c). Each
// method builds a []string of args, calls g.run, and on a non-zero exit uses
// errors.As into the typed *[ExitError] (reading OUR Stderr buffer) to route
// precisely. It uses a plain "package git" line because [git.go] OWNS the
// package doc comment, mirroring how internal/ui/exitcode.go defers to
// internal/ui/ui.go.
package git

import (
	"errors"
	"fmt"
	"strings"
)

// RevParseHEAD reads the current HEAD commit SHA — the parent that a
// snapshot-based commit will build on (PRD §13.5: "PARENT_SHA is captured
// before write-tree", so the parent is fixed at snapshot time and a later
// HEAD movement is caught by [UpdateRefCAS]).
//
// It returns three states:
//   - A repo WITH history → (40-hex SHA, hasParent=true, nil).
//   - An UNBORN / rootless repo (no commits yet) → ("", false, nil). git
//     reports this as exit 128 with stderr "ambiguous argument 'HEAD':
//     unknown revision ..."; that is the normal root-commit case (FR39: the
//     first commit omits -p and has no expected-old), NOT a failure, so the
//     error is swallowed into hasParent=false (contract: "treat
//     non-zero/empty as ok=false, not error").
//   - Any OTHER non-zero exit (e.g. "not a git repository") → a genuine
//     failure, returned as the typed *[ExitError] so the caller can route it.
//
// decisions.md §7: this is the ONLY read of HEAD used to build the commit, so
// it is never followed by `git commit` (Stagehand never runs `git commit`;
// commits are built via [CommitTree] + [UpdateRefCAS] only).
func (g *Git) RevParseHEAD() (sha string, hasParent bool, err error) {
	out, runErr := g.run("rev-parse", "HEAD")
	if runErr != nil {
		// An UNBORN repo makes rev-parse HEAD exit 128 with the "unknown
		// revision" message. That is the root-commit case (FR39), returned as
		// hasParent=false with a NIL error — NOT a failure. Any other
		// non-zero exit (e.g. not-a-repo) is a real error surfaced as-is.
		var ee *ExitError
		if errors.As(runErr, &ee) && ee.Code == 128 && strings.Contains(ee.Stderr, "unknown revision") {
			return "", false, nil
		}
		return "", false, runErr
	}
	sha = strings.TrimSpace(out)
	if sha == "" {
		// Defensive: rev-parse HEAD should never print an empty SHA on exit 0,
		// but the contract says an empty result is ok=false, not error.
		return "", false, nil
	}
	return sha, true, nil
}

// WriteTree freezes the current index into an immutable tree object and
// returns its SHA (PRD §13.2 step 1; FR7: "snapshot the index with
// write-tree"). It is read-only w.r.t. HEAD and the index — verified, an
// empty index yields the well-known empty-tree SHA
// 4b825dc642cb6eb9a060e54bf8d69288fbee4904 and a later `git add` does NOT
// change a previously returned tree SHA (snapshot immutability).
//
// If the index holds an UNRESOLVED MERGE CONFLICT, git aborts with exit 128.
// FR8 requires aborting in that case BEFORE any generation, and git 2.54.0
// reports the conflict via the substring "unmerged" (one line per conflicting
// path) plus "fatal: git-write-tree: error building trees" — it NEVER prints
// FR8's literal phrase, so the detection matches "unmerged" and returns a
// CLEAR error whose message contains "conflict". Any other non-zero exit is
// surfaced as the typed *[ExitError] (already a clear, routable error).
//
// decisions.md §7: write-tree never touches refs, so it is safe to call
// repeatedly during generation; only [UpdateRefCAS] mutates HEAD.
func (g *Git) WriteTree() (string, error) {
	out, err := g.run("write-tree")
	if err != nil {
		// An unresolved merge conflict in the index makes write-tree exit 128
		// with "<path>: unmerged (...)" lines + "error building trees".
		// Match "unmerged" (NOT FR8's literal wording, which git never emits)
		// and return a CLEAR abort error mentioning "conflict" (FR8) so the
		// generate layer never reaches the model. Other non-zero exits are
		// already a clear typed *ExitError — surface them as-is.
		var ee *ExitError
		if errors.As(err, &ee) && strings.Contains(ee.Stderr, "unmerged") {
			return "", fmt.Errorf("git: unresolved merge conflicts in index (resolve them before generating): %s", strings.TrimSpace(ee.Stderr))
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// CommitTree builds a commit object from the given tree SHA and returns its
// SHA (PRD §13.2 step 2). The message is passed as ONE -m arg, which preserves
// a multi-line / multi-paragraph body verbatim (verified via
// `git log --format=%B`); g.run leaves cmd.Stdin nil, so -F - is not used.
//
// When parent is "" (the root commit, FR39), -p is OMITTED; otherwise
// `-p <parent>` makes the new commit a child. The returned SHA is DANGLING
// until [UpdateRefCAS] advances HEAD — commit-tree touches NO ref (verified:
// a root commit-tree on an unborn repo leaves `rev-parse HEAD` still failing
// until update-ref runs), which is what keeps the §18.1 atomicity invariant
// structural (the commit object exists but is unreachable until the CAS step).
//
// decisions.md §7: Stagehand NEVER runs `git commit`; the generate COMMIT
// block calls CommitTree then UpdateRefCAS, so a commit is never created by
// porcelain that would mutate HEAD directly.
func (g *Git) CommitTree(parent, msg, tree string) (string, error) {
	// Build the args explicitly (do NOT append mid-slice) so the -p flag, when
	// present, lands before -m and the tree argument is always last.
	var args []string
	if parent != "" {
		args = []string{"commit-tree", "-p", parent, "-m", msg, tree} // child commit
	} else {
		args = []string{"commit-tree", "-m", msg, tree} // root commit: omit -p (FR39)
	}
	out, err := g.run(args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// UpdateRefCAS atomically advances ref to newSHA — the ONLY step in the
// snapshot-based commit flow that mutates a ref (PRD §13.2 step 3, §18.1: the
// repository's refs are modified only here, and only if HEAD is unchanged
// since the snapshot). It uses git's compare-and-swap forms and NEVER
// `--force` (decisions.md §7; PRD §18.2: do not force-update):
//
//   - expected != "" (normal CAS, FR40): `update-ref <ref> <new> <expected>`.
//     git refuses with exit 128 if ref is not at expected (e.g. HEAD moved
//     concurrently), leaving HEAD byte-for-byte UNCHANGED — the §18.1 atomic
//     invariant. FR41 requires aborting on this failure (the generate layer
//     turns it into the §18.2 "HEAD moved" message + exit 1).
//   - expected == "" (root commit, FR39): `update-ref <ref> <new>` — the
//     1-arg no-expected form. decisions.md §7: this form is ONLY legal for the
//     root commit on an unborn HEAD; a non-root commit MUST use the CAS form.
//
// It returns nil on success or the typed *[ExitError] (Code 128) on a CAS
// failure; callers assert HEAD is unchanged afterward.
func (g *Git) UpdateRefCAS(ref, newSHA, expected string) error {
	args := []string{"update-ref", ref, newSHA}
	if expected != "" {
		// 3-arg CAS form <ref> <new> <expected>: refuses if ref != expected.
		args = append(args, expected)
	}
	// NEVER append "--force" (decisions.md §7; PRD §18.2). The 1-arg form
	// (no expected) is used ONLY for the root commit on an unborn HEAD.
	_, err := g.run(args...)
	return err
}
