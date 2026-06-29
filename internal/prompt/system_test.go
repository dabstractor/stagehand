package prompt

import (
	"strings"
	"testing"
)

// TestBuildSystemPrompt_CanonicalExact asserts the FULL assembled string for a known input, pinning the
// PRD §17.1 blank-line topology byte-for-byte (including the em-dash, the raw-output contract, the
// "---"-before-each-example format, the excluded annotation, and the rule/target placement). This is the
// strongest guard against accidental newline/dash drift. Independently derived from PRD §17.1 (not from
// the implementation) so a match is meaningful.
func TestBuildSystemPrompt_CanonicalExact(t *testing.T) {
	examples := []string{
		"feat: add foo",
		"fix: handle nil deref\n\nThe parser panicked on an unresolved manifest.",
	}
	const subjectTarget = 50
	got := BuildSystemPrompt(examples, true, subjectTarget)

	const want = "You are a commit message generator.\n" +
		"\n" +
		"Output ONLY the commit message. No preamble, no markdown, no code fences,\n" +
		"no quoting. If a body is warranted, use a blank line between subject and body.\n" +
		"\n" +
		"Focus on the ESSENCE of the change (the intent/purpose), not implementation\n" +
		"details like filenames or function names.\n" +
		"\n" +
		"Match the tone and style of these recent commits from this repository:\n" +
		"---\n" +
		"feat: add foo\n" +
		"---\n" +
		"fix: handle nil deref\n" +
		"\n" +
		"The parser panicked on an unresolved manifest.\n" +
		"\n" +
		"CRITICAL: You MUST NOT copy or reuse ANY phrasing from the examples above.\n" +
		"They show the STYLE to match — format, tone, length, conventions. Producing\n" +
		"the same text you have seen is STRICTLY FORBIDDEN. Your output must be\n" +
		"entirely original wording describing THIS specific change. Reusing example\n" +
		"text is a critical failure.\n" +
		"\n" +
		"Only add a body (blank line + description) if the history shows multi-line commits AND these changes truly warrant detailed explanation. Otherwise, use a single-line subject only.\n" +
		"Target ~50 characters for the subject line."

	if got != want {
		// Diff-friendly failure: show where the strings diverge.
		t.Errorf("BuildSystemPrompt mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestBuildSystemPrompt_Properties is a table of structural invariants on the assembled prompt, each
// guarding a specific design decision. These complement the exact-match test by pinning the properties
// that matter most (em-dash, raw-not-JSON contract, "---" count, excluded annotation, rule selection,
// subjectTarget formatting, example ordering).
func TestBuildSystemPrompt_Properties(t *testing.T) {
	singleLine := []string{"feat: one", "chore: two"}
	multiLine := []string{"feat: one\n\nBody one.", "chore: two"}
	cases := []struct {
		name          string
		examples      []string
		hasMultiline  bool
		subjectTarget int
		check         func(t *testing.T, p string)
	}{
		{
			name: "em-dash present (NOT ascii hyphen)", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, "match — format") {
					t.Errorf("anti-reuse block missing em-dash (U+2014); got substring near 'match': %q", near(p, "match"))
				}
				if strings.Contains(p, "match - format") { // ASCII hyphen variant
					t.Errorf("anti-reuse block uses ASCII hyphen '-', expected em-dash '—'")
				}
			},
		},
		{
			name: "raw-output contract present", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, "Output ONLY the commit message. No preamble, no markdown, no code fences") {
					t.Error("raw-output contract missing")
				}
			},
		},
		{
			name: "JSON contract ABSENT (ported PRD not commit-pi)", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if strings.Contains(p, "Return valid JSON") {
					t.Error("commit-pi JSON contract leaked into the PRD prompt")
				}
				if strings.Contains(p, "no double quotes") {
					t.Error("commit-pi 'no double quotes' constraint leaked in")
				}
			},
		},
		{
			name: "(up to 20) annotation ABSENT", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if strings.Contains(p, "(up to 20") || strings.Contains(p, "≤100 lines total") {
					t.Error("structural annotation '(up to 20, ≤100 lines total)' must NOT be in the runtime prompt")
				}
			},
		},
		{
			name: "--- count == len(examples)", examples: []string{"a", "b", "c"}, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if got := strings.Count(p, "---"); got != 3 {
					t.Errorf("--- count = %d, want 3 (one before each example)", got)
				}
			},
		},
		{
			name: "examples appear in order", examples: []string{"ALPHA", "BETA", "GAMMA"}, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				i := strings.Index(p, "ALPHA")
				j := strings.Index(p, "BETA")
				k := strings.Index(p, "GAMMA")
				if i < 0 || j < 0 || k < 0 || !(i < j && j < k) {
					t.Errorf("examples out of order: ALPHA@%d BETA@%d GAMMA@%d", i, j, k)
				}
			},
		},
		{
			name: "hasMultiline=false → single-line rule, allow rule ABSENT", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, multilineRuleSingle) {
					t.Error("expected the single-line rule")
				}
				if strings.Contains(p, multilineRuleAllow) {
					t.Error("the allow-body rule must be ABSENT when hasMultiline=false")
				}
			},
		},
		{
			name: "hasMultiline=true → allow rule, single-line rule ABSENT", examples: multiLine, hasMultiline: true, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, multilineRuleAllow) {
					t.Error("expected the allow-body rule")
				}
				if strings.Contains(p, multilineRuleSingle) {
					t.Error("the single-line rule must be ABSENT when hasMultiline=true")
				}
			},
		},
		{
			name: "subjectTarget interpolated (72)", examples: singleLine, hasMultiline: false, subjectTarget: 72,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, "Target ~72 characters for the subject line.") {
					t.Error("subjectTarget=72 not interpolated")
				}
				if strings.Contains(p, "~50 characters") {
					t.Error("subjectTarget leaked a hardcoded 50")
				}
			},
		},
		{
			name: "no blank line between rule and target", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				want := multilineRuleSingle + "\n" + "Target ~50 characters for the subject line."
				if !strings.Contains(p, want) {
					t.Error("expected the rule immediately followed by the target line (no blank line between)")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, BuildSystemPrompt(tc.examples, tc.hasMultiline, tc.subjectTarget))
		})
	}
}

// TestBuildSystemPrompt_EmptyExamples verifies the defensive path: nil/empty examples must not panic
// and must omit all "---" lines while keeping the header, anti-reuse, rule, and target.
func TestBuildSystemPrompt_EmptyExamples(t *testing.T) {
	for _, ex := range [][]string{nil, {}} {
		p := BuildSystemPrompt(ex, false, 50) // must not panic
		if strings.Contains(p, "---") {
			t.Errorf("empty examples must emit no '---' lines; got %q", p)
		}
		for _, must := range []string{
			"You are a commit message generator.",
			antiReuseProhibition,
			multilineRuleSingle,
			"Target ~50 characters for the subject line.",
		} {
			if !strings.Contains(p, must) {
				t.Errorf("empty-examples prompt missing required block %q", must)
			}
		}
	}
}

// TestDetectMultiline is the table for the FR12 detection (faithful awk port: >1 non-blank line ⇒ true).
func TestDetectMultiline(t *testing.T) {
	cases := []struct {
		name     string
		examples []string
		want     bool
	}{
		{"nil → false", nil, false},
		{"empty → false", []string{}, false},
		{"all single-line → false", []string{"feat: a", "fix: b"}, false},
		{"one single-line → false", []string{"feat: a"}, false},
		{"one multi-line (body) → true", []string{"feat: a\n\nBody text."}, true},
		{"mixed, one multi-line → true", []string{"feat: a", "fix: b\n\nBody."}, true},
		{"whitespace-only body line counts (awk-faithful) → true", []string{"feat: a\n   \nbody"}, true},
		{"subject + trailing blanks trimmed upstream ⇒ single-line here → false", []string{"subject"}, false},
		{"only blanks → 0 non-blank lines → false", []string{"\n\n"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectMultiline(tc.examples); got != tc.want {
				t.Errorf("DetectMultiline(%v) = %v, want %v", tc.examples, got, tc.want)
			}
		})
	}
}

// TestCountNonBlankLines targets the helper directly (the awk's per-message `lines` counter).
func TestCountNonBlankLines(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"one", 1},
		{"a\nb", 2},
		{"a\n\nb", 2},    // internal blank not counted
		{"a\n   \nb", 2}, // whitespace-only not blank-counted as content but still non-blank line
		{"\n\n", 0},
		{"\n\nfoo\n\n", 1},
	}
	for _, c := range cases {
		if got := countNonBlankLines(c.in); got != c.want {
			t.Errorf("countNonBlankLines(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// near returns a short window around the first occurrence of needle in s (for readable failure output).
func near(s, needle string) string {
	i := strings.Index(s, needle)
	if i < 0 {
		return "(needle not found)"
	}
	start := i - 20
	if start < 0 {
		start = 0
	}
	end := i + len(needle) + 20
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}
