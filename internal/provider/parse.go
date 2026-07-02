package provider

import (
	"encoding/json"
	"regexp"
	"strings"
)

// collapseNewlines collapses runs of 3+ consecutive newlines to exactly 2
// (PRD §12.9 step 4). It is compiled once at package init and applied after
// the "\r\n"→"\n" conversion so that CRLF runs collapse correctly.
var collapseNewlines = regexp.MustCompile(`\n{3,}`)

// parseOutput implements the §12.9 output-parsing pipeline. It reads ONLY
// m.Output, m.JSONField, and m.StripCodeFence (the three §12.1 fields
// manifest.go marks "Consumed by the output parser, not by Render") and
// returns the cleaned commit message plus ok = msg != "".
//
// The pipeline is fixed-order: (1) trim; (2) optional single-layer code-fence
// unwrap, which runs BEFORE the output switch so a fenced JSON block is
// unwrapped to the JSON object and then parsed; (3) the output switch, where
// the default "" and "raw" both yield msg=s and ONLY "json" attempts
// extraction; (4) newline normalization ("\r\n"→"\n" first, then collapse 3+
// newlines to 2); (5) final trim.
//
// On any JSON-parse failure the pipeline falls back to the cleaned raw stdout
// (§17.4 robustness) — it NEVER returns an error. The fallback BEHAVIOR (json
// fails ⇒ msg=s, byte-identical to raw success) is the contract; verbose
// logging of a fallback EVENT is explicitly deferred to the generate layer
// (P1.M6.T1.S1), which holds the --verbose UI handle. Importing log/os/config
// here would break purity, testability, and the no-import-cycle milestone
// constraint, so parseOutput stays pure.
func parseOutput(raw string, m Manifest) (string, bool) {
	// (1) trim leading/trailing whitespace.
	s := strings.TrimSpace(raw)

	// (2) optional single-layer code-fence unwrap (§12.9 step 2). Enter ONLY
	// when StripCodeFence is set and the trimmed output opens with ``` or ~~~.
	// fence := s[:3] is safe (entry is gated on HasPrefix) and matches only
	// its own kind: a ``` opener never closes on ~~~. After dropping the opener
	// line (including any language tag) the LAST fence marker is searched for
	// in the REMAINING body — never the original s — so the opener itself
	// cannot be re-matched.
	if m.StripCodeFence && (strings.HasPrefix(s, "```") || strings.HasPrefix(s, "~~~")) {
		fence := s[:3]
		if nl := strings.IndexByte(s, '\n'); nl != -1 {
			s = s[nl+1:] // drop the opener line (incl. any language tag)
		} else {
			s = "" // the opener was the entire string
		}
		if last := strings.LastIndex(s, fence); last != -1 {
			s = s[:last] // drop everything from the LAST closer onward
		}
		s = strings.TrimSpace(s)
	}

	// (3) output switch. The default "" and "raw" both yield msg=s; ONLY
	// "json" attempts extraction, with ANY failure silently leaving msg=s
	// (the §17.4 raw fallback).
	msg := s
	if m.Output == OutputJSON {
		if v, ok := extractJSON(s, m.JSONField); ok {
			msg = v
		}
	}

	// (4) normalize newlines: convert "\r\n"→"\n" FIRST, then collapse runs
	// of 3+ "\n" to exactly 2 ("\r\n\r\n\r\n" → "\n\n\n" → "\n\n"). Lone "\r"
	// (old-Mac) is out of scope — PRD names only "\r\n".
	msg = strings.ReplaceAll(msg, "\r\n", "\n")
	msg = collapseNewlines.ReplaceAllString(msg, "\n\n")

	// (5) final trim + ok.
	msg = strings.TrimSpace(msg)
	return msg, msg != ""
}

// extractJSON attempts whole-then-balanced-substring JSON extraction of field
// (PRD §12.9 step 3 / §17.4 robustness). It first tries json.Unmarshal on the
// whole string; on failure it retries on the brace-balanced substring from the
// first '{' to the last '}' (guarded end>start), which tolerates prose around
// the JSON object. The selected field is extracted with a STRICT string type
// assertion: a non-string value (number/bool/null), a missing field, or an
// empty JSONField all count as failure. ANY failure returns ("", false) so
// parseOutput falls back to raw. json.Unmarshal returns an error (never
// panics) on malformed input, so no recover is needed.
func extractJSON(s, field string) (string, bool) {
	// First attempt: the whole string as a JSON object.
	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err == nil {
		if v, ok := obj[field].(string); ok {
			return v, true
		}
	}

	// Second attempt: the brace-balanced substring (first '{' … last '}').
	start := strings.IndexByte(s, '{')
	if start == -1 {
		return "", false
	}
	end := strings.LastIndexByte(s, '}')
	if end == -1 || end <= start {
		return "", false
	}
	var obj2 map[string]any
	if err := json.Unmarshal([]byte(s[start:end+1]), &obj2); err == nil {
		if v, ok := obj2[field].(string); ok {
			return v, true
		}
	}
	return "", false
}
