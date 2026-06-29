package prompt

import "strings"

// Canonical user-payload string constants — committed VERBATIM from PRD §17.3 (re-verified against
// prd_snapshot.md lines 1117–1142). The user payload is the stdin content for the provider executor
// (P1.M2.T5 — render.go line 100 `case "stdin": spec.Stdin = payload`); the diff is NEVER a CLI arg
// (PRD §9.3 FR15: avoids arg-length limits and shell injection).
//
// Constants are defined WITHOUT trailing newlines; BuildUserPayload owns ALL inter-block newline
// placement so the §17.3 blank-line topology lives in exactly one auditable place (mirrors S1/S2's rule).
//
// §17.3 is ENTIRELY ASCII — no em-dash (unlike §17.1's anti-reuse block), no non-ASCII bytes. The
// `<diff payload>` / `<rejected subject N>` tokens in the PRD code block are STRUCTURAL annotations
// (placeholders for dynamic content), NOT literal output — BuildUserPayload substitutes the runtime
// `diff` arg and `rejected` slice elements. See research design-decisions.md §5.

// userInstruction is the §17.3 NORMAL instruction (FR15): the stable user-facing string that introduces
// the diff directly. NOTE the trailing COLON — §17.3's normal block ends the line with ":" (prd_snapshot.md
// line 1123). Used when len(rejected) == 0.
const userInstruction = "Generate a commit message for these changes:"

// userInstructionReject is the §17.3 REJECTION instruction: the SAME stable string but ending with a PERIOD
// (prd_snapshot.md line 1130), because the rejection block's "IMPORTANT: …" directive follows on its own.
// NOTE: this is INTENTIONAL §17.3 fidelity — the work-item paraphrase elides the period (shows the colon
// uniformly), but the PRD §17.3 is authoritative (exact S1/S2 precedent: copy §17.x character-for-character).
// See research design-decisions.md §2 — the single most error-prone decision; a canonical test pins it.
const userInstructionReject = "Generate a commit message for these changes."

// rejectionPreamble is the two-line §17.3 IMPORTANT block header that opens the rejection list. Committed
// VERBATIM (the line break falls after "already exist", exactly as in §17.3). NO trailing newline —
// BuildUserPayload appends '\n' then the per-subject list. Used only when len(rejected) > 0.
const rejectionPreamble = `IMPORTANT: The following messages were REJECTED because they already exist
in git history. You MUST generate something COMPLETELY DIFFERENT:`

// rejectionEpilogue is the single-line §17.3 closing directive, emitted AFTER the rejected-subject list.
// Committed VERBATIM. NO trailing newline — BuildUserPayload appends "\n\n" (one blank line) then the diff.
const rejectionEpilogue = "Create an entirely new message with different wording."

// BuildUserPayload implements PRD §9.3 FR15 / §9.7 FR32 / §17.3: assemble the user payload delivered to
// the agent via stdin (never as a CLI arg — FR15). It is the user-payload half of the prompt layer (the
// system-prompt half is S1/S2's BuildSystemPrompt / BuildFallbackPrompt). The orchestrator (P1.M3.T4)
// calls it on EVERY generation attempt:
//
//	payload := prompt.BuildUserPayload(diff, rejected)
//	spec, _ := manifest.Render(model, provider, sys, payload)   // payload is the 4th arg (P1.M2.T4)
//
// `rejected` is []string{} on the first attempt and non-empty (the matched duplicate subjects, FR30:
// each "the first line of the message") on a duplicate-rejection retry (FR32, up to
// max_duplicate_retries default 3). len(rejected) == 0 selects the normal path.
//
// ASSEMBLY (PRD §17.3, exact — see research design-decisions.md §3):
//
//	NORMAL (len(rejected) == 0):
//	  userInstruction + "\n\n" + diff
//	    → "Generate a commit message for these changes:\n\n<diff>"   (COLON instruction + blank + diff)
//
//	REJECTION (len(rejected) > 0):
//	  userInstructionReject + "\n\n"          // PERIOD instruction + blank line
//	  + rejectionPreamble + "\n"              // the two-line IMPORTANT header, then end its 2nd line
//	  + for each s in rejected: "- " + s + "\n"   // one list line per rejected subject (single-line per FR30)
//	  + "\n"                                  // blank line after the list
//	  + rejectionEpilogue + "\n\n"            // "Create an entirely new message…" + blank line
//	  + diff                                  // appended VERBATIM (no normalization)
//
// THE colon-vs-period call (design-decisions.md §2): the normal instruction ends ":" (introduces the diff),
// the rejection instruction ends "." (the IMPORTANT directive follows). BOTH are §17.3 renderings; the
// work-item paraphrase elides the period but the PRD is authoritative.
//
// The diff is appended VERBATIM — its trailing bytes (including whether it ends with '\n' and any
// StagedDiff truncation sentinel) are preserved. The diff's shape is git.StagedDiff's contract (P1.M1.T3.S1),
// not this assembler's. See design-decisions.md §6.
//
// Defensive: nil/empty rejected ⇒ normal path (len(nil)==0); empty diff ⇒ instruction (+ optional block)
// with an empty tail — no panic (the orchestrator gates on HasStagedChanges, so diff is non-empty in
// practice). Subjects are assumed single-line (FR30) — no sanitization (design-decisions.md §8).
func BuildUserPayload(diff string, rejected []string) string {
	if len(rejected) == 0 {
		// §17.3 NORMAL: colon instruction + blank line + diff (verbatim). Fast path — no loop, no Builder.
		return userInstruction + "\n\n" + diff
	}

	// §17.3 REJECTION: period instruction + blank + IMPORTANT preamble + per-subject list + blank + epilogue + blank + diff.
	var b strings.Builder
	b.WriteString(userInstructionReject)
	b.WriteString("\n\n")
	b.WriteString(rejectionPreamble)
	b.WriteByte('\n') // end the preamble's second line, then the list
	for _, s := range rejected {
		b.WriteString("- ")
		b.WriteString(s)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')                // blank line after the list
	b.WriteString(rejectionEpilogue) // "Create an entirely new message with different wording."
	b.WriteString("\n\n")            // blank line before the diff
	b.WriteString(diff)              // verbatim
	return b.String()
}
