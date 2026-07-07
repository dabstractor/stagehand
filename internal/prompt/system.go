package prompt

import (
	"fmt"
	"strings"
)

// Canonical prompt string constants — committed VERBATIM from PRD §17.1 (Appendix A). These are the
// "refined from commit-pi" versions: commit-pi shipped the JSON contract; PRD §17.4 replaced it with
// the raw-output contract (no double-quote constraint, no fragile sed parse). See research
// commit-pi-origin.md §3 for the full commit-pi→PRD diff.
//
// Constants are defined WITHOUT trailing newlines; BuildSystemPrompt owns ALL inter-block newline
// placement so the §17.1 blank-line topology lives in exactly one auditable place (design-decisions §11).

// promptPreamble is the shared role + RAW-output contract (§17.4 design call — NOT commit-pi's JSON
// contract) + essence instruction, common to both the auto examples-block header and the non-auto
// format scaffolds (PRD §9.19 FR-F2/F3/F4 / §17.8). NO trailing newline; NO "Match the tone…" line — that
// is auto-only (examplesIntro).
const promptPreamble = `You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences,
no quoting. If a body is warranted, use a blank line between subject and body.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.`

// examplesIntro is the auto-mode-only "Match the tone…" line that introduces the style-examples block
// (PRD §17.1). Non-auto format modes (§17.8) omit it entirely — the scaffold body replaces the block.
const examplesIntro = "Match the tone and style of these recent commits from this repository:"

// maturePromptHeader is the prompt preamble through the examples-intro line: role, RAW-output contract,
// essence instruction, and "Match the tone…" intro. Note "from this repository" (PRD refinement;
// commit-pi had "from these recent commits:"). Defined as a COMPILE-TIME constant concatenation of
// promptPreamble + examplesIntro so the auto-mode assembled bytes are unchanged by the §17.8 split
// (FR-F1: auto + empty locale stays byte-identical to today).
const maturePromptHeader = promptPreamble + "\n\n" + examplesIntro

// antiReuseProhibition is the verbatim anti-reuse block (PRD §17.1). NOTE the EM-DASH "—" (U+2014) in
// "the STYLE to match — format" — commit-pi used an ASCII hyphen "-"; the PRD refined it to an em-dash.
// It is the ONLY non-ASCII byte in the entire prompt.
const antiReuseProhibition = `CRITICAL: You MUST NOT copy or reuse ANY phrasing from the examples above.
They show the STYLE to match — format, tone, length, conventions. Producing
the same text you have seen is STRICTLY FORBIDDEN. Your output must be
entirely original wording describing THIS specific change. Reusing example
text is a critical failure.`

// The two multi-line rules (PRD §17.1), selected by hasMultiline. Verbatim, including the "multi-line"
// hyphenation. commit-pi's wording is identical here (only cosmetic hyphenation differs).
const (
	// multilineRuleAllow is used when FR12 detected multi-line commits in history.
	multilineRuleAllow = "Only add a body (blank line + description) if the history shows multi-line commits AND these changes truly warrant detailed explanation. Otherwise, use a single-line subject only."
	// multilineRuleSingle is used when the history is single-line only (or examples is empty).
	multilineRuleSingle = "Only output a single-line subject (no body)."
)

// subjectTargetLine renders the PRD §17.1 subject-length target. The "~" is literal (PRD: "Target ~50
// characters for the subject line."). subjectTarget is wired from config.Config.SubjectTargetChars
// (P1.M1.T4.S1, default 50) by the orchestrator (P1.M3.T4); BuildSystemPrompt is decoupled from config.
func subjectTargetLine(subjectTarget int) string {
	return fmt.Sprintf("Target ~%d characters for the subject line.", subjectTarget)
}

// DetectMultiline implements PRD §9.3 FR12: detect whether the recent history contains multi-line
// (subject + body) commits by scanning the examples. It is a faithful port of commit-pi's awk heuristic:
//
//	examples=$(git log --format="---%n%B" -20 | sed '/^$/d' | head -100)
//	has_multiline=$(echo "$examples" | awk '/^---$/{if(lines>1)found=1; lines=0; next} {lines++} END{print found+0}')
//
// The awk counts, per commit (delimited by "---"), the number of NON-BLANK lines (sed stripped blanks
// first); found=1 if ANY commit had >1 non-blank line. git.RecentMessages (P1.M1.T3.S3) has already
// split on the NUL delimiter, trimmed each message, and capped at 100 lines keeping complete messages,
// so DetectMultiline only needs the per-message ">1 non-blank line" test. It returns true iff ANY
// example has more than one non-blank line. nil/empty → false; never panics.
//
// Why countNonBlankLines and NOT strings.Contains(msg, "\n"): they agree for every realistic git
// message, but the awk strips blanks THEN counts, and countNonBlankLines mirrors that exactly —
// removing all doubt about whitespace-only body lines (which sed '/^$/d' does NOT strip, so the awk
// counts them, and so does countNonBlankLines). See research commit-pi-origin.md §2.
func DetectMultiline(examples []string) bool {
	for _, msg := range examples {
		if countNonBlankLines(msg) > 1 {
			return true
		}
	}
	return false
}

// countNonBlankLines returns the number of non-blank lines in s (a line is blank iff strings.TrimSpace
// of it is empty — mirroring commit-pi's `sed '/^$/d'` which strips truly-empty lines, then counting).
// It is the per-message embodiment of the awk's `lines` counter.
func countNonBlankLines(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

// BuildSystemPrompt implements PRD §9.3 FR13 / §17.1: assemble the mature-repo system prompt from the
// canonical constants + the repo's recent-commit examples + the detected multi-line flag + the subject
// target. It is the style-learning half of stagecoach's core IP (PRD §13): the model is shown up to 20
// real recent commits so its output matches the repo's conventions, while the verbatim anti-reuse
// prohibition forbids copying the example text.
//
// ASSEMBLY TOPOLOGY (PRD §17.1, exact — see research commit-pi-origin.md §5):
//
//	maturePromptHeader          // "…from this repository:" (no trailing \n)
//	'\n'
//	for each ex: "---\n" + ex + '\n'   (one "---" BEFORE each message; examples are pre-trimmed)
//	'\n'                        // blank line between last example and the anti-reuse block
//	antiReuseProhibition        // "…is a critical failure." (no trailing \n)
//	'\n' '\n'                   // blank line between anti-reuse and the multi-line rule
//	<multi-line rule>           // selected by hasMultiline (no trailing \n)
//	'\n'
//	subjectTargetLine(subjectTarget)   // "Target ~N characters for the subject line."
//
// The "(up to 20, ≤100 lines total)" line from PRD §17.1's code block is INTENTIONALLY EXCLUDED — it is
// a structural annotation, not literal text (commit-pi never emitted it; caps are enforced upstream by
// RecentMessages). See commit-pi-origin.md §4.
//
// WHY hasMultiline is a PARAMETER (not computed inside): §9.3 splits this into FR12 (detect) and FR13
// (construct); BuildSystemPrompt is FR13 and takes the flag as input so detection (DetectMultiline) is
// independently testable and the caller controls it. The orchestrator wires them:
//
//	hasMulti := prompt.DetectMultiline(recent)                                  // FR12
//	sys := prompt.BuildSystemPrompt(recent, hasMulti, cfg.SubjectTargetChars, cfg.Format, cfg.Locale)   // FR13
//
// Defensive: nil/empty examples emit NO "---" lines and no panic (the orchestrator gates on
// CommitCount>1, so examples are non-empty in practice). See design-decisions.md §9.
// fallbackPromptBody is PRD §17.2 MINUS the final target/format line: role, the SHORT §17.2 raw-output
// contract (no "no quoting", no body clause — new repo ⇒ single-line subject), and the essence
// instruction. Committed VERBATIM from PRD §17.2 (Appendix A). NO trailing newline — BuildFallbackPrompt
// owns the single "\n\n" (one blank line) before the target line, mirroring S1's rule that constants
// carry no trailing newline so inter-block newline placement lives in exactly one auditable place.
//
// Unlike §17.1 (S1), §17.2 is ALL ASCII — no em-dash, no non-ASCII bytes; the "~" in the target line is
// an ASCII tilde. See research design-decisions.md §4.
const fallbackPromptBody = `You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.`

// BuildFallbackPrompt implements PRD §9.3 FR14 / §17.2: the new-repo (≤1 commit) conventional-commit
// fallback system prompt. When the orchestrator (P1.M3.T4) finds git.CommitCount(...) <= 1 it calls
// THIS instead of S1's BuildSystemPrompt — there is no history to learn style from, so the model is
// taught the Conventional Commits scaffold (type(scope): description) directly.
//
// ASSEMBLY (PRD §17.2, exact):
//
//	fallbackPromptBody                       // role + short output contract + essence (no trailing \n)
//	'\n' '\n'                                // exactly ONE blank line
//	fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
//
// THE interpolation call (research design-decisions.md §2 — the load-bearing decision): ONLY the `50`
// in §17.2's "Target ~50 characters (~7 words). Format: type(scope): description" becomes `%d` — exact
// analogy with S1's subjectTargetLine (where §17.1's "Target ~50 characters for the subject line."
// became "Target ~%d characters for the subject line."). "(~7 words)" stays VERBATIM (a fixed gloss,
// not a spec; scaling it would add an undocumented ÷7 magic constant). With the default
// cfg.SubjectTargetChars=50 (P1.M1.T4.S1) the output is BYTE-IDENTICAL to PRD §17.2.
//
// Do NOT reuse S1's subjectTargetLine — it emits the §17.1 wording, wrong for §17.2.
//
// format/locale implement PRD §9.19 FR-F1/F4/F6 / §17.8: format=="auto" reproduces the §17.2 body exactly
// (FR-F1 byte-identity); any other mode replaces it with the mode's scaffold (buildFormatSystemPrompt,
// hasMultiline=false — a new/unborn repo has no history to detect multi-line commits from). locale, when
// non-empty, appends the FR-F6 one-line language instruction in EITHER path (withLocale is a no-op when
// locale=="").
//
// Defensive: subjectTarget is a plain int with no failure mode; fmt.Sprintf cannot fail. Returns string
// only (no error). See design-decisions.md §1/§2/§3.
func BuildFallbackPrompt(subjectTarget int, format, locale string) string {
	if format != "auto" {
		return withLocale(buildFormatSystemPrompt(format, false, subjectTarget), locale)
	}
	s := fallbackPromptBody + "\n\n" +
		fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
	return withLocale(s, locale)
}

// format/locale implement PRD §9.19 FR-F1/F2/F3/F6 / §17.8: format=="auto" reproduces the §17.1 assembly
// exactly (FR-F1 byte-identity — the ONLY change is wrapping the return in withLocale, a no-op when
// locale==""); any other mode replaces the style-examples block + anti-reuse warning with the mode's
// scaffold (buildFormatSystemPrompt) while retaining the multi-line rule (FR12 detection still runs) and
// the subject-target line.
func BuildSystemPrompt(examples []string, hasMultiline bool, subjectTarget int, format, locale string) string {
	if format != "auto" {
		return withLocale(buildFormatSystemPrompt(format, hasMultiline, subjectTarget), locale)
	}

	var b strings.Builder

	// Header (role + raw-output contract + essence + examples intro).
	b.WriteString(maturePromptHeader)
	b.WriteByte('\n')

	// Style examples: one "---" line BEFORE each message. RecentMessages returns trimmed messages
	// (no trailing newline), so append '\n' so the next "---" starts on its own line.
	for _, ex := range examples {
		b.WriteString("---\n")
		b.WriteString(ex)
		b.WriteByte('\n')
	}

	// Blank line, then the verbatim anti-reuse prohibition.
	b.WriteByte('\n')
	b.WriteString(antiReuseProhibition)

	// Blank line, then the multi-line rule selected by the detection (FR12 → FR13).
	b.WriteByte('\n')
	b.WriteByte('\n')
	if hasMultiline {
		b.WriteString(multilineRuleAllow)
	} else {
		b.WriteString(multilineRuleSingle)
	}

	// Subject target on its own line (no blank line between rule and target per §17.1).
	b.WriteByte('\n')
	b.WriteString(subjectTargetLine(subjectTarget))

	return withLocale(b.String(), locale)
}
