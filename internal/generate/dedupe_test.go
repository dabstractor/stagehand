package generate

import (
	"strings"
	"testing"
)

// TestExtractSubject is a table-driven pure-function suite for PRD §9.7 FR30:
// "Extract the generated subject (first line of the message)." Pure-function tests —
// no subprocess, no mocking, no temp repo. Covers multi-line body exclusion, single-line
// passthrough, empty message, trailing/leading spaces, \r\n defensive, and whitespace-only
// input. Mirrors internal/provider/parse_test.go's table-driven style.
func TestExtractSubject(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// --- FR30: subject = first line (body excluded) ---
		{"multi-line body excluded", "fix: foo\n\nbody text here", "fix: foo"},
		{"multi-line no blank separator", "fix: foo\nbody text", "fix: foo"},

		// --- FR30: single-line passthrough ---
		{"single line", "fix: foo", "fix: foo"},
		{"single line no trailing newline", "fix: handle null user", "fix: handle null user"},

		// --- FR30: empty message ---
		{"empty message", "", ""},

		// --- FR30: trailing spaces trimmed ---
		{"trailing spaces on subject line", "fix: foo  \nbody", "fix: foo"},
		{"trailing tab on subject line", "fix: foo\t\nbody", "fix: foo"},

		// --- leading spaces trimmed (TrimSpace is symmetric) ---
		{"leading spaces on subject line", "  fix: foo\nbody", "fix: foo"},
		{"leading + trailing spaces", "  fix: foo  \nbody", "fix: foo"},

		// --- \r\n defensive (ParseOutput normalizes to \n, but be robust) ---
		{"crlf defensive", "fix: foo\r\nbody", "fix: foo"},

		// --- whitespace-only first line ---
		{"whitespace-only first line", "   \nbody text", ""},
		{"tab-only first line", "\t\nbody text", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractSubject(tc.in)
			if got != tc.want {
				t.Errorf("ExtractSubject(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestIsDuplicate is a table-driven pure-function suite for PRD §9.7 FR32: "If the
// subject exactly matches one of the 50 [recent subjects], retry." Pure-function
// tests — no subprocess, no mocking. Pins the EXACT, case-SENSITIVE, whole-subject
// match semantics (faithful to commit-pi's grep -Fxq). Mirrors
// internal/provider/parse_test.go's table-driven style.
func TestIsDuplicate(t *testing.T) {
	cases := []struct {
		name    string
		subject string
		recent  []string
		want    bool
	}{
		// --- FR32: exactly matches → true ---
		{"match present", "fix: foo", []string{"a", "fix: foo", "b"}, true},
		{"match in middle", "fix: foo", []string{"x", "fix: foo", "y"}, true},
		{"match first element", "fix: foo", []string{"fix: foo", "b", "c"}, true},
		{"match last element", "fix: foo", []string{"a", "b", "fix: foo"}, true},
		{"only element is match", "fix: foo", []string{"fix: foo"}, true},

		// --- FR32: no match → false ---
		{"no match", "fix: foo", []string{"a", "b"}, false},
		{"single different element", "fix: foo", []string{"fix: bar"}, false},
		{"recent has one entry, no match", "fix: foo", []string{"other"}, false},

		// --- nil / empty recent → false ---
		{"nil recent", "fix: foo", nil, false},
		{"empty recent", "fix: foo", []string{}, false},

		// --- empty subject ---
		{"empty subject, empty recent", "", nil, false},
		{"empty subject, non-empty recent", "", []string{"a", "b"}, false},
		{"empty subject matches empty in recent", "", []string{"", "x"}, true},

		// --- FR32: case-SENSITIVE (grep -Fxq has no -i) ---
		{"case sensitive different case → false", "Fix: Foo", []string{"fix: foo"}, false},
		{"case sensitive all upper → false", "FIX: FOO", []string{"fix: foo"}, false},
		{"case sensitive all lower → false", "fix: foo", []string{"Fix: Foo"}, false},

		// --- FR32: EXACT whole-subject (grep -x, not substring/prefix) ---
		{"prefix not a match", "fix: foobar", []string{"fix: foo"}, false},
		{"suffix not a match", "foobar: fix", []string{"bar: fix"}, false},
		{"substring not a match", "x fix: foo y", []string{"fix: foo"}, false},

		// --- duplicate entries in recent (set deduplicates) ---
		{"duplicate entries in recent, match", "fix: foo", []string{"fix: foo", "fix: foo", "a"}, true},
		{"duplicate entries in recent, no match", "fix: bar", []string{"fix: foo", "fix: foo", "a"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsDuplicate(tc.subject, tc.recent)
			if got != tc.want {
				t.Errorf("IsDuplicate(%q, %v) = %v, want %v",
					tc.subject, tc.recent, got, tc.want)
			}
		})
	}
}

// TestIsDuplicate_LargeRecent verifies IsDuplicate handles a larger recent slice
// (simulating 50 subjects from git.RecentSubjects) without issues.
func TestIsDuplicate_LargeRecent(t *testing.T) {
	// Build 50 unique subjects.
	recent := make([]string, 50)
	for i := range recent {
		recent[i] = strings.Repeat("x", i+1) // unique lengths, no duplicates
	}

	if IsDuplicate("fix: foo", recent) {
		t.Error("IsDuplicate should return false for subject not in 50-element recent")
	}

	// Add the subject and verify match.
	recent[25] = "fix: foo"
	if !IsDuplicate("fix: foo", recent) {
		t.Error("IsDuplicate should return true when subject is in 50-element recent")
	}
}
