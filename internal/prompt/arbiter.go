package prompt

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Arbiter prompt constants — committed VERBATIM from PRD §17.7. The arbiter is the third of three
// decomposition agent roles (planner §17.5, stager §17.6, arbiter §17.7). It is a BARE agent that runs
// ONLY if the working tree is non-empty after the per-concept loop (§13.6.5). It receives the commits
// made this run (SHA + subject + file-list each) and a diff of the remaining (leftover) changes, and
// returns a JSON object indicating whether the leftovers belong with an existing commit or warrant a new
// one. It performs NO git itself — stagehand owns all ref mutations (FR-M10); the arbiter only decides.
//
// Constants are defined WITHOUT trailing newlines; BuildArbiterUserPayload owns ALL inter-block newline
// placement so the blank-line topology lives in exactly one auditable place (mirrors system.go /
// payload.go / planner.go convention).
//
// §17.7 is ASCII EXCEPT one em-dash (U+2014) in "return null) — never force a fit." (see §1.1 note
// below). §17.7 has NO backticks ⇒ a backtick raw string compiles cleanly. §17.7 has NO <style
// examples> placeholder ⇒ BuildArbiterSystemPrompt is zero-arg (unlike the planner §17.5).

// arbiterSystemPrompt is the verbatim §17.7 system prompt from "You reconcile leftover changes into
// commits that were just made." through `Respond with ONLY JSON: {"target": "<sha from the list>"} or
// {"target": null}.` The JSON-contract line (with its literal `<sha from the list>` token) is
// LITERAL instructive text shown to the model — that token is NOT a runtime placeholder. NOTE the
// EM-DASH "—" (U+2014) in "return null) — never force a fit." — the ONE non-ASCII byte; do NOT
// replace it with an ASCII hyphen. NO trailing newline. NO `<style examples>` token (§17.7 has none).
const arbiterSystemPrompt = `You reconcile leftover changes into commits that were just made. You are given the commits
created this run (with their messages and changed files) and a diff of changes that were not
included in any of them.

Decide: do these leftovers logically belong WITH one of those commits, or do they warrant a
NEW commit?
- Choose an existing commit only if the leftovers are part of the SAME logical change.
- When in doubt, prefer a NEW commit (return null) — never force a fit.
- You may only target a commit from the provided list.

Respond with ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.`

// arbiterCommitsHeader is the section header introducing the commit list in the arbiter user
// payload. A designed (NOT verbatim §17.7) faithful section header — §17.7 does not specify headers.
// Trailing COLON, NO trailing newline.
const arbiterCommitsHeader = "The commits made this run (message and changed files for each):"

// arbiterLeftoverHeader is the section header introducing the leftover diff in the arbiter user
// payload. A designed (NOT verbatim §17.7) faithful section header — §17.7 does not specify headers.
// Trailing COLON, NO trailing newline.
const arbiterLeftoverHeader = "A diff of changes that were not included in any commit:"

// ArbiterCommit is one commit made this run, as shown to the arbiter (§13.6.5: "SHAs, messages, and
// file-lists"). Built by the consumer (decompose/arbiter.go) from diff-tree output; the SHA is the
// value the arbiter may return as "target".
type ArbiterCommit struct {
	SHA     string   // the commit's full SHA (40/64 hex) — the value the arbiter may return as "target".
	Subject string   // the commit's subject line (one line; §13.6.5's "messages").
	Files   []string // the file-list (diff-tree --name-only) for this commit; may be empty.
}

// ArbiterOutput is the arbiter's JSON response (§17.7). A nil Target means {"target": null} → a NEW
// commit (the §17.7 "when in doubt" default); a non-nil Target points at the SHA to amend. The
// caller (decompose/arbiter.go) validates Target is in the provided commit list and non-empty, and
// resolves new/tip-amend/mid-chain/ambiguous→null (§13.6.5). ParseArbiterOutput does NOT validate —
// it only parses.
type ArbiterOutput struct {
	Target *string `json:"target"` // nil ⇔ null ⇔ new commit; &"<sha>" ⇔ amend that commit.
}

// BuildArbiterSystemPrompt returns the verbatim §17.7 arbiter system prompt. It takes NO arguments
// because §17.7 has NO <style examples> placeholder (unlike §17.5's planner). The thin wrapper keeps
// the constant private (no-trailing-newline enforced in one place) and provides API symmetry with the
// Build* family. Stagehand performs all git (FR-M10); the arbiter only decides.
func BuildArbiterSystemPrompt() string {
	return arbiterSystemPrompt
}

// BuildArbiterUserPayload assembles the arbiter user payload from the commit list and leftover diff
// (§17.7 / §13.6.5). The consumer (decompose/arbiter.go) builds []ArbiterCommit from diff-tree output
// and passes the working-tree diff as leftoverDiff.
//
// ASSEMBLY TOPOLOGY:
//
//	arbiterCommitsHeader + "\n\n"
//	for each commit:
//	  SHA + "\n" + Subject + "\n" + (each file + "\n") + "\n"   // blank line separating blocks
//	arbiterLeftoverHeader + "\n\n" + leftoverDiff               // verbatim tail (no normalization)
//
// Defensive: empty/nil commits ⇒ the loop body never runs (no panic; output is headers + diff —
// acceptable though impossible in practice). A commit with empty Files ⇒ block is SHA+Subject only.
// leftoverDiff is appended VERBATIM (payload.go precedent: "diff is the exact tail").
func BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string) string {
	var b strings.Builder
	b.WriteString(arbiterCommitsHeader)
	b.WriteByte('\n')
	b.WriteByte('\n') // one blank line before the commit list
	for _, c := range commits {
		b.WriteString(c.SHA)
		b.WriteByte('\n')
		b.WriteString(c.Subject)
		b.WriteByte('\n')
		for _, f := range c.Files {
			b.WriteString(f)
			b.WriteByte('\n') // files one per line
		}
		b.WriteByte('\n') // one blank line separating commit blocks
	}
	b.WriteString(arbiterLeftoverHeader)
	b.WriteByte('\n')
	b.WriteByte('\n')           // one blank line before the diff
	b.WriteString(leftoverDiff) // verbatim tail (no normalization)
	return b.String()
}

// ParseArbiterOutput parses the arbiter agent's raw JSON output into a typed ArbiterOutput. It first
// attempts a whole-string json.Unmarshal; on failure, it falls back to a brace-balanced JSON extractor
// (the package-level extractJSONObject defined in planner.go — REUSED, NOT redeclared here) that
// finds the first '{' and scans to the matching '}' (handles JSON embedded in prose or code fences).
// Returns a non-nil error on any parse failure so the caller can retry / default-to-null (§13.6.5).
//
// It does NOT validate target-in-list or non-empty — the caller (decompose/arbiter.go) owns that
// decision (§13.6.5 "Ambiguous → default to null"). A non-string target (number/bool) yields a JSON
// type error. Extra unknown fields are ignored. A missing "target" field yields nil Target (new commit
// default). §17.7 defines NO retry instruction — this layer does not export one.
func ParseArbiterOutput(raw string) (ArbiterOutput, error) {
	s := strings.TrimSpace(raw)
	var out ArbiterOutput

	// Attempt 1: whole-string Unmarshal.
	err1 := json.Unmarshal([]byte(s), &out)
	if err1 == nil {
		return out, nil
	}

	// Attempt 2: brace-balanced fallback (handles JSON embedded in prose / code fences).
	// extractJSONObject is defined in planner.go (same package) — REUSED, NOT redeclared here.
	if sub, found := extractJSONObject(s); found {
		err2 := json.Unmarshal([]byte(sub), &out)
		if err2 == nil {
			return out, nil
		}
		return ArbiterOutput{}, fmt.Errorf("arbiter output: not valid JSON: %w", err2)
	}

	return ArbiterOutput{}, fmt.Errorf("arbiter output: not valid JSON: %w", err1)
}
