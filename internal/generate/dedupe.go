// Package generate provides commit-message generation primitives for Stagehand's
// generation pipeline. This subtask implements the duplicate-rejection primitives
// (PRD §9.7 FR30 / FR32) that the orchestrator's retry loop (P1.M3.T4) is built
// from: ExtractSubject (FR30) and IsDuplicate (FR32).
package generate

import "strings"

// ExtractSubject implements PRD §9.7 FR30: "Extract the generated subject (first
// line of the message)." It returns the trimmed first line of message — the commit
// subject. The body (everything after the first '\n') is excluded. This is the
// faithful Go port of commit-pi's "head -1" subject extraction.
//
// The message arrives pre-trimmed + newline-normalized by provider.ParseOutput
// (P1.M2.T6.S1, Step 4/5), so we trim only the first line (clears any trailing
// spaces on the subject line itself). Empty message → "".
//
// Uses strings.IndexByte (O(pos), zero-alloc) — semantically identical to
// strings.Split(message, "\n")[0] but avoids building the full line slice.
//
// Signature is FROZEN per the work-item contract. Downstream consumer: orchestrator
// P1.M3.T4 calls subject := generate.ExtractSubject(msg) on every generation attempt.
func ExtractSubject(message string) string {
	first := message
	if nl := strings.IndexByte(message, '\n'); nl >= 0 {
		first = message[:nl]
	}
	return strings.TrimSpace(first)
}

// IsDuplicate implements PRD §9.7 FR32: "If the subject exactly matches one of the
// 50 [recent commit subjects], retry." It builds a set from recent (the Go map-set
// port of commit-pi's grep -Fxq: -x = whole-line match, no -i = case-sensitive)
// and reports whether subject is a member. O(1) lookup after O(n) set construction.
//
// Match semantics are EXACT, case-SENSITIVE, and whole-subject:
//   - "Fix: Foo" != "fix: foo" (case differs → NOT a duplicate)
//   - "fix: foobar" != "fix: foo" (prefix → NOT a duplicate)
//
// Both inputs are pre-trimmed by their producers: subject comes from ExtractSubject
// (which TrimSpaces the first line); each recent element comes from
// git.RecentSubjects (P1.M1.T3.S4, which TrimSpaces each line and skips empties).
// A plain map lookup is correct — NO defensive trimming (which would mask an
// upstream bug).
//
// Signature is FROZEN per the work-item contract. Downstream consumer: orchestrator
// P1.M3.T4 calls generate.IsDuplicate(subject, recent) on every generation attempt;
// on match it appends subject to rejected and calls prompt.BuildUserPayload(diff,
// rejected) for the retry.
func IsDuplicate(subject string, recent []string) bool {
	set := make(map[string]struct{}, len(recent))
	for _, s := range recent {
		set[s] = struct{}{}
	}
	_, dup := set[subject]
	return dup
}
