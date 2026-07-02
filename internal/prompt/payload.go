// This file adds the user-payload assembler for the generate step
// (P1.M4.T1.S3): AssemblePayload builds the user payload the executor
// (P1.M6.T1.S1) feeds to the agent, in the REFERENCE byte order (diff FIRST,
// instruction LAST — reference_impl.md §3 + §4 D5, an EXPLICIT decision that
// over-rules the PRD §17.3 prose) with the §17.3 rejection block appended LAST
// on duplicate-retry. It uses a plain "package prompt" line because
// [examples.go] OWNS the package doc, mirroring how [system.go] and
// internal/git/log.go defer to git.go.
package prompt

import "strings"

// The canonical §17.3 duplicate-rejection strings, committed VERBATIM from PRD
// §17.3 as named Go constants (byte-exact, confirmed with cat -A) so the exact
// wording is reviewable and stable — the same convention [system.go] used for
// the §17.1/§17.2 canonical strings. Only the per-retry subjects (interpolated
// by the caller as bullets) are NOT literals here. See PRD §17.3.
const (
	// rejectionHeader is the §17.3 rejection-block header. It is TWO lines
	// (keep the \n between "already exist" and "in git history") and ENDS with
	// a colon. Copied VERBATIM from PRD §17.3 — do NOT reflow or trim it.
	rejectionHeader = "IMPORTANT: The following messages were REJECTED because they already exist\nin git history. You MUST generate something COMPLETELY DIFFERENT:"

	// rejectionBulletPrefix is the per-subject bullet prefix (§17.3): each
	// rejected subject is rendered as rejectionBulletPrefix + subject.
	rejectionBulletPrefix = "- "

	// rejectionFooter closes the §17.3 rejection block. Copied VERBATIM from
	// PRD §17.3.
	rejectionFooter = "Create an entirely new message with different wording."
)

// AssemblePayload assembles the user payload the generate step (P1.M6.T1.S1)
// feeds to the agent's stdin/positional/flag via [internal/provider.Executor]
// (P1.M2.T4.S1, which takes the returned `payload string`). It is a PURE
// string builder — no git, no config, no IO, no error return (PRD FR13/FR14,
// plan_overview §M4: M4 is decoupled; builders take scalar settings; the
// generate layer (M6) is the integrator that obtains `diff` from
// git.StagedDiff and supplies `instruction` and `rejected`). Prompt production
// code therefore imports NOTHING from internal/git or internal/config: the
// diff, instruction, and rejected-list are PASSED-IN parameters (the caller
// decisions.md §3: payload = prompt.AssemblePayload(diff, instruction,
// rejected); rejected = append(rejected, subject)).
//
// Byte order is REFERENCE ordering (★ reference_impl.md §3 + §4 D5 — an
// EXPLICIT decision that over-rules the PRD §17.3 prose ★):
//
//   - Base (len(rejected)==0): `<diff>\n\n<instruction>` — diff FIRST,
//     instruction LAST. NEVER instruction-first. The imperative sits closest to
//     where generation begins (recency rationale).
//
//   - Retry (len(rejected)>0): `<diff>\n\n<instruction>\n\n<rejection block>`
//     — diff stays FIRST and the §17.3 rejection block is APPENDED LAST
//     (maximum recency). The "insert the rejection block … BEFORE the diff"
//     phrasing in the over-ruled PRD §17.3 prose is treated as illustrative and
//     IGNORED; the RESOLVED layout is diff-first + rejection-block-last.
//
// The §17.3 rejection block (see [rejectionBlock]) is:
//
//	rejectionHeader + "\n" +
//	  ("- " + subject, per rejected entry, in SLICE ORDER, joined by "\n") +
//	  "\n\n" + rejectionFooter
//
// i.e. ONE newline between the header and the first bullet (NOT a blank line,
// per §17.3), and a BLANK line (\n\n) after the LAST bullet before the footer.
//
// AssemblePayload does NOT validate inputs (no error return; pure renderer):
// empty `diff` and/or empty `instruction` are the caller's concern (M6
// guarantees a non-empty diff by the nothing-to-commit gate), and the elements
// of `rejected` are rendered VERBATIM as bullets. Output is DETERMINISTIC for
// identical inputs; no trailing newline is appended beyond what the layout
// specifies.
func AssemblePayload(diff, instruction string, rejected []string) string {
	// ★ D5 (reference_impl.md §3 + §4 D5): diff FIRST — NEVER instruction-first. ★
	payload := diff + "\n\n" + instruction
	if len(rejected) == 0 {
		// len(nil)==0, so nil and the empty slice both yield the base layout
		// with NO rejection block.
		return payload
	}
	// Rejection block APPENDED LAST (maximum recency): the resolved layout is
	// `<diff>\n\n<instruction>\n\n<rejection block>`, NOT "BEFORE the diff".
	return payload + "\n\n" + rejectionBlock(rejected)
}

// rejectionBlock assembles the PRD §17.3 duplicate-rejection block from the
// list of already-rejected subjects: rejectionHeader, ONE newline, a bullet
// ("- " + subject) per rejected subject in SLICE ORDER joined by "\n", a BLANK
// line, and rejectionFooter. It is the trailing section AssemblePayload appends
// on duplicate-retry (decisions.md §3: rejected = append(rejected, subject)).
// The byte layout (ONE newline to the first bullet, BLANK line before the
// footer) is VERBATIM from §17.3 — do NOT alter it.
func rejectionBlock(rejected []string) string {
	bullets := make([]string, len(rejected))
	for i, subj := range rejected {
		bullets[i] = rejectionBulletPrefix + subj // slice order preserved
	}
	return rejectionHeader + "\n" + strings.Join(bullets, "\n") + "\n\n" + rejectionFooter
}
