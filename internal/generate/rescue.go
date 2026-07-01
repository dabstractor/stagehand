package generate

import (
	"strconv"
	"strings"
)

// FormatRescue implements PRD §18.3 / §9.10 FR44: assemble the rescue message
// byte-for-byte for display when commit generation fails after the snapshot was taken.
// It is a pure string assembler (3 strings in, 1 string out, no I/O, no error) that
// mirrors prompt.BuildUserPayload's Builder + no-trailing-newline style.
//
// The rescue message (PRD §18.3) reassures the user their staged files are safe,
// shows the Tree ID, and provides the exact git commit-tree recovery command:
//
//	2 leading spaces; -p <parentSHA> omitted when parentSHA == "" (root/unborn repo),
//	mirroring git.CommitTree's root semantics (P1.M1.T2.S4, FR39).
//
// When candidateMsg != "" (a message was produced but rejected for duplication or
// parse failure), the §18.3 candidate note is appended after the closing separator,
// separated by one blank line, with the message in literal double-quotes.
//
// The return value has NO trailing newline — the CLI layer (P1.M4.T1) adds it via
// fmt.Fprintln. The "(interrupted)" variant is OUT OF SCOPE — owned by the signal
// handler (P1.M4.T2, §5); FormatRescue always produces the §18.3 base form.
//
// Provenance: faithful port of commit-pi's rescue message (PRD Appendix C).
//
// Signature is FROZEN per the work-item contract. Downstream consumer: orchestrator
// P1.M3.T4 calls msg := generate.FormatRescue(treeSHA, parentSHA, candidateMsg) on
// the rescue path (FR43–FR45), hands the string to the CLI layer (P1.M4.T1) which
// prints it + the exit-code system (P1.M4.T3.S3) sets exit 3.
func FormatRescue(treeSHA, parentSHA, candidateMsg string) string {
	var b strings.Builder

	// Line 1 — failure notice. ❌ = U+274C (NON-ASCII, §18.3). NOT "(interrupted)" (§5).
	b.WriteString("❌ Commit generation failed.\n")
	// Line 2 — separator (exactly 60 '-', verified prd_snapshot.md 1178).
	b.WriteString(rescueSep)
	b.WriteByte('\n')
	// Line 3 — reassurance.
	b.WriteString("Your staged files were safely snapshotted before generation.\n")
	// Line 4 — Tree ID (substitute treeSHA).
	b.WriteString("Tree ID: ")
	b.WriteString(treeSHA)
	b.WriteByte('\n')
	// Line 5 — blank.
	b.WriteByte('\n')
	// Line 6 — manual-recovery header.
	b.WriteString("To commit the originally staged files manually:\n")
	// Line 7 — the command (2 leading spaces; -p <parentSHA> omitted when root — mirrors git.CommitTree).
	b.WriteString("  git commit-tree")
	if parentSHA != "" {
		b.WriteString(" -p ")
		b.WriteString(parentSHA)
	}
	b.WriteString(` -m "Your message" `) // literal ASCII " around Your message (shell template)
	b.WriteString(treeSHA)
	b.WriteString(" | xargs git update-ref HEAD")
	b.WriteByte('\n')
	// Line 8 — blank.
	b.WriteByte('\n')
	// Line 9 — first-commit hint (kept ALWAYS — §18.3 verbatim, design-decisions §3).
	b.WriteString(`(omit "-p <PARENT_SHA>" if this is the repository's first commit)`)
	b.WriteByte('\n')
	// Line 10 — closing separator (NO trailing newline — the CLI's Fprintln adds it, §7).
	b.WriteString(rescueSep)

	// Candidate note (§18.3 last paragraph): appended AFTER the closing separator with ONE blank
	// line, iff a candidate message was produced but rejected (duplicate-exhaustion / parse).
	if candidateMsg != "" {
		b.WriteString("\n\nA candidate message was produced but rejected: \"")
		b.WriteString(candidateMsg)
		b.WriteString("\". You can use it manually in the command above.")
	}

	return b.String()
}

// FormatRescueMulti implements PRD §18.3's multi-commit variant (last ¶): when a single concept
// fails mid-loop, the rescue is scoped to that concept's frozen tree[i]. It prints tree[i], its parent
// (newSHA[i-1]), and the same commit-tree|update-ref recipe as FormatRescue — plus a concept-naming
// header ("concept <index+1> of <count>: <title>") and a multi-commit reassurance line.
// index is 0-based (printed 1-based); count is N. conceptTitle=="" omits the title suffix.
// parentSHA=="" omits " -p <parent>" (root/unborn — mirrors git.CommitTree's root semantics).
// candidateMsg!="" appends the §18.3 candidate note. No trailing newline (the caller's Fprintln adds it).
// Reuses rescueSep (60 '-') + FormatRescue's recipe lines verbatim.
func FormatRescueMulti(treeSHA, parentSHA, candidateMsg, conceptTitle string, index, count int) string {
	var b strings.Builder

	// Line 1 — concept-naming header.
	b.WriteString("❌ Commit generation failed for concept ")
	b.WriteString(strconv.Itoa(index + 1)) // 1-based for humans
	b.WriteString(" of ")
	b.WriteString(strconv.Itoa(count))
	if conceptTitle != "" {
		b.WriteString(": ")
		b.WriteString(conceptTitle)
	}
	b.WriteString(".\n")
	// Line 2 — separator.
	b.WriteString(rescueSep)
	b.WriteByte('\n')
	// Line 3 — multi-commit reassurance (§18.3 multi-commit variant).
	b.WriteString("Concepts already published are final and untouched. Remaining staged changes are safe in your index.\n")
	// Line 4 — Tree ID.
	b.WriteString("Tree ID: ")
	b.WriteString(treeSHA)
	b.WriteByte('\n')
	// Line 5 — blank.
	b.WriteByte('\n')
	// Line 6 — manual-recovery header.
	b.WriteString("To commit this concept's staged files manually:\n")
	// Line 7 — the command (2 leading spaces; -p <parentSHA> omitted when root).
	b.WriteString("  git commit-tree")
	if parentSHA != "" {
		b.WriteString(" -p ")
		b.WriteString(parentSHA)
	}
	b.WriteString(` -m "Your message" `)
	b.WriteString(treeSHA)
	b.WriteString(" | xargs git update-ref HEAD")
	b.WriteByte('\n')
	// Line 8 — blank.
	b.WriteByte('\n')
	// Line 9 — first-commit hint (kept ALWAYS — mirrors FormatRescue).
	b.WriteString(`(omit "-p <PARENT_SHA>" if this is the repository's first commit)`)
	b.WriteByte('\n')
	// Line 10 — closing separator.
	b.WriteString(rescueSep)

	// Candidate note (§18.3 last paragraph): same as FormatRescue.
	if candidateMsg != "" {
		b.WriteString("\n\nA candidate message was produced but rejected: \"")
		b.WriteString(candidateMsg)
		b.WriteString("\". You can use it manually in the command above.")
	}

	return b.String()
}

// rescueSep is the §18.3 separator line: exactly 60 hyphens (verified against
// prd_snapshot.md lines 1178 + 1186). Used for both the top (line 2) and bottom
// (line 10) separators. A literal (not strings.Repeat) for byte-for-byte §18.3
// fidelity — mirrors prompt/payload.go's verbatim-constant philosophy.
const rescueSep = "------------------------------------------------------------" // 60 × '-'
