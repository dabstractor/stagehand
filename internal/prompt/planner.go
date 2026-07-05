package prompt

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Planner prompt constants — committed VERBATIM from PRD §17.5. The planner is the first of three
// decomposition agent roles (planner §17.5, stager §17.6, arbiter §17.7). It receives the full
// working-tree diff + the §17.1 style examples, decides ONE vs SEVERAL commits, partitions into
// logical units, and emits structured JSON.
//
// Constants are defined WITHOUT trailing newlines; BuildPlannerSystemPrompt / BuildPlannerUserPayload
// own ALL inter-block newline placement so the §17.5 blank-line topology lives in exactly one
// auditable place (mirrors system.go / payload.go convention).
//
// §17.5 is ENTIRELY ASCII — no em-dash, no non-ASCII bytes.

// plannerSystemPrompt is the verbatim §17.5 system prompt from "You are a commit-planning assistant."
// through `The "description" must be specific enough that a staging agent can find the exact changes.`
// The JSON-contract line (with its <int>/<bool>/<short concept> tokens) is LITERAL text shown to the
// model as the output-format example — those tokens are NOT replaced at runtime. The trailing
// "<style examples>" is a RUNTIME placeholder and is NOT part of this constant; BuildPlannerSystemPrompt
// appends the actual examples there. NO trailing newline.
const plannerSystemPrompt = `You are a commit-planning assistant. Given a diff of un-staged changes, decide whether they
form ONE coherent commit or SEVERAL, and partition them into logical units.

Rules:
- Prefer FEWER commits. A single commit is correct unless the changes clearly span
  unrelated concerns. Do not manufacture tiny commits.
- Each commit must be independently meaningful and reviewable. Group tightly-coupled
  changes (a function + its test, a refactor + its callers) together.
- Respect dependencies: if change B depends on change A, A comes first.
- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.

Respond with ONLY JSON, no prose, no code fences:
{"count": <int>, "single": <bool>, "commits": [{"title": "<short concept>", "description": "<precisely which files/hunks belong here, by path>"}, ...]}
- If single is true, set count=1 and ALSO include "message": "<the full commit message>".
- The "description" must be specific enough that a staging agent can find the exact changes.`

// plannerUserInstruction is the §17.5 normal user-payload instruction (trailing COLON — matches
// payload.go's §17.3 normal-instruction precedent). NO trailing newline.
const plannerUserInstruction = "Decompose these un-staged changes into commits:"

// PlannerRetryInstruction is the §17.5 retry user payload for the caller's ONE retry after a parse
// failure (§13.6.6). Owned here so the verbatim §17.5 text is auditable in one place. Consumed by
// decompose/planner.go (P3.M2.T2.S1) — NOT part of BuildPlannerUserPayload. NO trailing newline.
const PlannerRetryInstruction = "Respond with ONLY the JSON object described, no other text."

// PlannerCommit is one partitioned concept from the planner (§17.5 JSON contract).
type PlannerCommit struct {
	Title       string   `json:"title"`       // "<short concept>" — a short label for the concept.
	Description string   `json:"description"` // "<precisely which files/hunks belong here, by path>" — staging instructions.
	Files       []string `json:"files"`       // FR-M3: every path this concept touches; guidance, not a constraint (FR-M1c is the content guarantee).
}

// PlannerOutput is the planner's full JSON response (§17.5). Message is present iff Single==true
// (the single-commit shortcut, §13.6.4); when Single==false it is the zero value "". The caller
// (decompose/planner.go) enforces the single⇔message contract, NOT this struct.
type PlannerOutput struct {
	Count   int             `json:"count"`   // N (== len(Commits); ==1 iff Single)
	Single  bool            `json:"single"`  // true ⇒ single-commit shortcut (§13.6.4)
	Commits []PlannerCommit `json:"commits"` // the partition (1..N); nil if the model emitted "commits":null
	Message string          `json:"message"` // the full commit message; present iff Single==true
}

// BuildPlannerSystemPrompt assembles the planner system prompt from the verbatim §17.5 constant
// followed by a blank line and EITHER the style examples in "---\n<msg>\n" format (format=="auto",
// identical to system.go's BuildSystemPrompt example loop) OR the §17.8 format scaffold body
// (formatScaffoldBody — shared with the message builders, FR-F5: the planner's single-call-shortcut
// message obeys the same substitution). The PARTITIONING contract (plannerSystemPrompt) itself is
// UNCHANGED in every mode (FR-F5); only the trailing style-examples-vs-scaffold block varies. locale,
// when non-empty, appends the FR-F6 one-line language instruction (withLocale — a no-op when
// locale=="", preserving FR-F1 byte-identity for format=="auto" && locale=="").
//
// ASSEMBLY TOPOLOGY (§17.5/§17.8, exact):
//
//	plannerSystemPrompt              // "…find the exact changes." (no trailing \n)
//	'\n' '\n'                        // one blank line before the style examples / scaffold
//	auto: for each ex: "---\n" + ex + '\n'   // one "---" BEFORE each message (same as system.go)
//	non-auto: formatScaffoldBody(format)     // "" for "plain" — contract + (locale) only
//	<withLocale>
//
// Defensive: nil/empty examples ⇒ no "---" lines and no panic. The auto result is
// plannerSystemPrompt + "\n\n" (a trailing blank line where the examples section is simply empty).
func BuildPlannerSystemPrompt(examples []string, format, locale string) string {
	var b strings.Builder
	b.WriteString(plannerSystemPrompt)
	b.WriteByte('\n')
	b.WriteByte('\n') // one blank line between the JSON contract and the style examples/scaffold
	if format == "auto" {
		for _, ex := range examples {
			b.WriteString("---\n") // one "---" BEFORE each message (same format as system.go)
			b.WriteString(ex)      // examples are pre-trimmed by RecentMessages
			b.WriteByte('\n')
		}
	} else {
		b.WriteString(formatScaffoldBody(format)) // scaffold REPLACES the examples (FR-F5)
	}
	return withLocale(b.String(), locale)
}

// BuildPlannerUserPayload assembles the §17.5 user payload: the instruction + blank line + the diff.
// When forcedCount > 0, a forced-count directive is prepended (§17.5 forced-count mode). `context` is the
// §9.19 FR-F7 `--context` flag text ("" when unset); when non-empty, the same contextBlock (payload.go)
// is inserted after the instruction line and before the diff (§17.8) — after the forced-count directive
// in forced mode.
//
// ASSEMBLY (§17.5/§17.8, exact):
//
//	NORMAL (forcedCount <= 0):
//	  plannerUserInstruction + "\n\n" + [contextBlock(context) + "\n\n" if context != ""] + diff
//	    → "Decompose these un-staged changes into commits:\n\n<diff>"
//
//	FORCED (forcedCount > 0):
//	  "Produce EXACTLY N commits from these changes (do not reconsider the count):\n"
//	  + plannerUserInstruction + "\n\n" + [contextBlock(context) + "\n\n" if context != ""] + diff
//	    → the two colon-ending instructions on consecutive lines, then ONE blank line, then
//	      (optionally) the context block, then the diff.
//
// forcedCount <= 0 (incl. negative) ⇒ normal path. The diff is appended VERBATIM (no normalization;
// mirrors payload.go's "diff is the exact tail"). context=="" ⇒ BYTE-IDENTICAL to the pre-FR-F7 payload
// in both normal and forced modes.
func BuildPlannerUserPayload(diff, context string, forcedCount int) string {
	block := ""
	if context != "" {
		block = contextBlock(context) + "\n\n" // shared helper (payload.go, same package)
	}
	if forcedCount <= 0 {
		return plannerUserInstruction + "\n\n" + block + diff // §17.5 normal (diff verbatim as tail)
	}
	forced := fmt.Sprintf("Produce EXACTLY %d commits from these changes (do not reconsider the count):", forcedCount)
	return forced + "\n" + plannerUserInstruction + "\n\n" + block + diff // §17.5 forced-count prepend
}

// ParsePlannerOutput parses the planner agent's raw JSON output into a typed PlannerOutput. It first
// attempts a whole-string json.Unmarshal; on failure, it falls back to a brace-balanced JSON extractor
// that finds the first '{' and scans to the matching '}' (handles JSON embedded in prose or code fences).
// Returns a non-nil error on any parse failure so the caller can trigger the one retry (§13.6.6).
//
// It does NOT validate the single⇔message contract — the caller (decompose/planner.go) owns that
// decision. It tolerates "commits":null (→ nil slice), extra unknown fields (ignored), and missing
// "message" with single:false (→ "" zero value).
func ParsePlannerOutput(raw string) (PlannerOutput, error) {
	s := strings.TrimSpace(raw)
	var out PlannerOutput

	// Attempt 1: whole-string Unmarshal.
	err1 := json.Unmarshal([]byte(s), &out)
	if err1 == nil {
		return out, nil
	}

	// Attempt 2: brace-balanced fallback (handles JSON embedded in prose / code fences).
	if sub, found := extractJSONObject(s); found {
		err2 := json.Unmarshal([]byte(sub), &out)
		if err2 == nil {
			return out, nil
		}
		return PlannerOutput{}, fmt.Errorf("planner output: not valid JSON: %w", err2)
	}

	return PlannerOutput{}, fmt.Errorf("planner output: not valid JSON: %w", err1)
}

// extractJSONObject finds the first '{' in s and scans to the matching '}' that returns brace depth to
// zero, correctly ignoring braces and quotes that appear INSIDE JSON string values. Returns the balanced
// substring (inclusive of the braces) and true, or "" and false if there is no '{' or the braces never
// balance.
//
// This is a verbatim copy of provider/parse.go's extractJSONObject algorithm, reimplemented privately
// here to avoid coupling the prompt leaf package to the provider package (sanctioned by the work item:
// "reuse provider.parseJSON PATTERN"). The prompt package has zero internal dependencies; this keeps it
// that way.
//
// State machine: `inString` suppresses brace counting inside "..."; `escaped` (one-byte lookahead)
// consumes the byte after a backslash inside a string so an escaped quote `\"` does NOT toggle inString.
// Byte scanning is UTF-8-safe: '{' '}' '"' '\\' are all ASCII (<0x80) and RFC 3629 §3 guarantees ASCII
// bytes never appear as UTF-8 continuation bytes — no utf8.DecodeRune needed.
func extractJSONObject(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}
