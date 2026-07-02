// Package prompt builds the system prompt and user payload for the generate
// step (plan_overview §M4). It is DECOUPLED from both config and the concrete
// git wrapper: the builders take scalar settings, and the history they consume
// is exposed only through the small [HistoryReader] interface that
// internal/git's *git.Git satisfies structurally (the generate layer — M6 —
// injects a *git.Git). Prompt production code therefore imports NOTHING from
// internal/git; the seam is the typed HistoryReader contract.
//
// This file is the first/primary file of package prompt and OWNS the package
// doc; sibling files (system.go in S2, payload.go in S3) will use a plain
// "package prompt" line and must NOT duplicate this doc — mirroring how
// internal/git/git.go owns "// Package git".
package prompt

import "strings"

// HistoryReader is the minimal git-history surface [FetchExamples] needs: the
// commit count (the gate) and the raw recent-message stream (the examples).
// It DECOUPLES this package from internal/git: *git.Git (internal/git/log.go,
// DONE P1.M3.T4.S1) satisfies it structurally with its exact
// CommitCount() (int, error) and RecentMessages(n int) (string, error) method
// signatures, so the generate layer (M6) can inject a *git.Git while prompt
// production code imports NO internal/git (PRD FR10/FR11, reference_impl.md
// §1/§6). RecentMessages returns the raw `---%n%B` output VERBATIM — no trim,
// no cap, the `---` separators and the intra-message blank lines intact —
// precisely so THIS layer can trim, cap, and group it (see [FetchExamples]).
type HistoryReader interface {
	// CommitCount mirrors git.CommitCount: `git rev-list --count HEAD`, with
	// the unborn/rootless case already routed to (0, nil) in the git layer.
	CommitCount() (int, error)

	// RecentMessages mirrors git.RecentMessages: the raw `---%n%B` stream of
	// the last n commits, newest-first, UNTRIMMED (blank lines + `---`
	// separators intact) so this layer trims/caps/groups.
	RecentMessages(n int) (string, error)
}

const (
	// DefaultExampleCount is the canonical number of recent commit messages
	// [FetchExamples] fetches (PRD FR11 / reference_impl.md §1: `git log
	// --format='---%n%B' -20`; reference_impl.md §6). The generate layer passes
	// this as `n` when it calls FetchExamples(g, prompt.DefaultExampleCount).
	DefaultExampleCount = 20

	// exampleLineCap is the maximum total non-blank lines the returned
	// examples may span (PRD FR11 / reference_impl.md §1: `... | head -100`;
	// PRD §17.1: "≤100 lines total"). It is a WHOLE-MESSAGE cap: whole
	// messages are accumulated newest-first while the running total stays at
	// or under this value (no message is cut mid-way). Unexported because it
	// is an internal detail of [FetchExamples].
	exampleLineCap = 100
)

// FetchExamples returns the recent commit messages to show the model as style
// examples and the multi-line signal that picks the conditional commit-format
// rule in PRD §17.1. It faithfully ports reference_impl.md §1/§6 to Go:
//
//   - examples = `git log --format='---%n%B' -n | sed '/^$/d' | head -100`
//     (reference_impl.md §1), and
//   - hasMultiline = awk '/^---$/{ if(lines>1) found=1; lines=0; next }
//     { lines++ } END { print found+0 }' (reference_impl.md §6): split the raw
//     log on `---` separators and flag multi-line if ANY group has >1
//     non-blank line (PRD FR10/FR11/FR12).
//
// Semantics:
//
//   - On a CommitCount/RecentMessages error it returns (nil, false, err)
//     WITHOUT swallowing — a real git failure is a genuine error (the unborn
//     case is already (0,nil)/("",nil) in internal/git, so it never reaches
//     here as an error).
//   - When CommitCount <= 1 it returns (nil, false, nil) — the new-repo /
//     root-commit path (FR39) that PRD §17.2's conventional-commit fallback
//     hinges on. This is NOT an error.
//   - Otherwise each returned element is ONE whole commit message, blank-line
//     trimmed (lines joined by "\n"), accumulated newest-first while the
//     total non-blank line count stays at or under exampleLineCap
//     (whole-message cap; no message is cut mid-way).
//   - hasMultiline is computed over ALL groups (pre-cap), per the contract
//     "split the raw log" — the cap affects ONLY the returned examples. It is
//     true iff ANY group has >1 non-blank line, INCLUDING the last/oldest
//     group: the reference awk's END block never re-checks the final
//     accumulated group, so a multi-line OLDEST commit is MISSED (verified
//     empirically). This port DELIBERATELY flushes + checks the final group
//     (see [splitExampleGroups]); do NOT "fix" it back toward the buggy
//     script.
func FetchExamples(g HistoryReader, n int) (examples []string, hasMultiline bool, err error) {
	count, err := g.CommitCount()
	if err != nil {
		return nil, false, err
	}
	if count <= 1 {
		// New-repo / root-commit path (FR39): NOT an error, just no examples.
		return nil, false, nil
	}
	raw, err := g.RecentMessages(n)
	if err != nil {
		return nil, false, err
	}
	groups := splitExampleGroups(raw)

	// hasMultiline is computed over ALL groups, BEFORE the 100-line cap, per
	// the contract "split the raw log" — the cap affects only the returned
	// examples.
	for _, grp := range groups {
		if len(grp) > 1 {
			hasMultiline = true
			break
		}
	}

	// Build examples newest-first with the whole-message 100-line cap. git log
	// emits newest-first, so groups[0] is the newest; iterating in order
	// yields newest-first examples. Include a whole group while the running
	// total stays at or under exampleLineCap; stop (dropping later groups)
	// otherwise — no message is cut mid-way.
	var total int
	for _, grp := range groups {
		if total+len(grp) > exampleLineCap {
			break
		}
		examples = append(examples, strings.Join(grp, "\n"))
		total += len(grp)
	}
	return examples, hasMultiline, nil
}

// splitExampleGroups ports reference_impl.md §1/§6's
// `sed '/^$/d'` + the awk `---` group split to Go. It scans the RAW
// `git log --format=---%n%B -<n>` stream line by line and partitions it into
// per-commit groups of non-blank lines (newest-first, because git log emits
// newest-first):
//
//   - A line whose trimmed content is exactly "---" is a group separator (the
//     leading separator each `--format=---%n%B` commit emits): flush the
//     current group (append to groups if non-empty, reset).
//   - A blank line (trimmed == "") is dropped — the `sed '/^$/d'` step.
//   - Any other (non-blank) line is appended VERBATIM (NOT TrimSpace'd) to the
//     current group, faithful to `sed '/^$/d'` which strips only empty lines,
//     not interior whitespace.
//
// After the loop the FINAL group is flushed too: the oldest commit has no
// trailing `---` after it, so without this final flush it would be lost. This
// is the DELIBERATE fix for the reference awk's END-block omission (the awk
// only sets `found=1` inside the `/^---$/` action and its `END` block merely
// prints `found+0` without re-checking the last accumulated `lines`, so a
// multi-line oldest commit is MISSED — verified empirically). Do NOT remove
// the final flush.
//
// Known faithful quirk: a commit body line that happens to be exactly "---"
// (trimmed) is treated as a separator, matching the reference awk's /^---$/
// pattern — a body containing a literal "---" line will be split there. This
// matches the shell behavior exactly.
func splitExampleGroups(raw string) [][]string {
	var groups [][]string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			groups = append(groups, cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.TrimSpace(line) == "---":
			// The `---` separator emitted by --format=---%n%B: end this commit.
			flush()
		case strings.TrimSpace(line) == "":
			// The `sed '/^$/d'` blank-line trim: drop empty lines.
		default:
			// Non-blank line: keep VERBATIM (do NOT TrimSpace — faithful to
			// sed, which only strips empty lines, not interior whitespace).
			cur = append(cur, line)
		}
	}
	flush() // ★ the OLDEST commit has no trailing `---` — flush it (awk-bug fix). ★
	return groups
}
