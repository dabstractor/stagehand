package prompt

import (
	"strings"
	"testing"
)

// TestBuildUserPayload_NormalCanonicalExact asserts the FULL assembled NORMAL payload (empty/nil rejected)
// is byte-for-byte the §17.3 normal rendering: COLON instruction + blank line + diff verbatim. Independently
// derived from PRD §17.3 (not from the implementation) so a match is meaningful. Pins the colon.
func TestBuildUserPayload_NormalCanonicalExact(t *testing.T) {
	const diff = "diff --git a/foo.go b/foo.go\n@@ -1,3 +1,4 @@"
	want := "Generate a commit message for these changes:\n" +
		"\n" +
		diff

	for _, rej := range [][]string{nil, {}} { // nil and empty are equivalent → normal path
		if got := BuildUserPayload(diff, rej); got != want {
			t.Errorf("BuildUserPayload(diff, %v) mismatch:\n--- got ---\n%q\n--- want ---\n%q", rej, got, want)
		}
	}
}

// TestBuildUserPayload_RejectionCanonicalExact asserts the FULL assembled REJECTION payload (non-empty
// rejected) is byte-for-byte the §17.3 rejection rendering: PERIOD instruction + blank + two-line IMPORTANT
// preamble + per-subject "- " list + blank + epilogue + blank + diff. Pins the period (NOT colon) and the
// exact blank-line topology. Independently derived from PRD §17.3.
func TestBuildUserPayload_RejectionCanonicalExact(t *testing.T) {
	const diff = "diff --git a/foo.go b/foo.go\n@@ -1,3 +1,4 @@"
	rejected := []string{"fix: handle null user", "feat: add bar"}
	got := BuildUserPayload(diff, rejected)

	want := "Generate a commit message for these changes.\n" + // PERIOD
		"\n" +
		"IMPORTANT: The following messages were REJECTED because they already exist\n" +
		"in git history. You MUST generate something COMPLETELY DIFFERENT:\n" +
		"- fix: handle null user\n" +
		"- feat: add bar\n" +
		"\n" +
		"Create an entirely new message with different wording.\n" +
		"\n" +
		diff

	if got != want {
		t.Errorf("BuildUserPayload rejection mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestBuildUserPayload_Properties is a table of structural invariants guarding the load-bearing decisions:
// the colon-vs-period distinction, the per-subject "- " list, the diff-always-the-tail rule, and the
// presence/absence of the rejection block in each path.
func TestBuildUserPayload_Properties(t *testing.T) {
	const diff = "DIFFCONTENT"
	cases := []struct {
		name     string
		diff     string
		rejected []string
		check    func(t *testing.T, p string)
	}{
		{
			name: "normal: instruction ends with COLON", diff: diff, rejected: nil,
			check: func(t *testing.T, p string) {
				if !strings.HasPrefix(p, "Generate a commit message for these changes:\n\n") {
					t.Errorf("normal payload must start with colon instruction + blank line; got %q", near(p, "Generate"))
				}
			},
		},
		{
			name: "normal: rejection block ABSENT", diff: diff, rejected: nil,
			check: func(t *testing.T, p string) {
				for _, absent := range []string{"IMPORTANT:", "REJECTED", "Create an entirely new message"} {
					if strings.Contains(p, absent) {
						t.Errorf("normal payload must NOT contain rejection element %q", absent)
					}
				}
			},
		},
		{
			name: "rejection: instruction ends with PERIOD (not colon)", diff: diff, rejected: []string{"a"},
			check: func(t *testing.T, p string) {
				if !strings.HasPrefix(p, "Generate a commit message for these changes.\n\n") {
					t.Errorf("rejection payload must start with PERIOD instruction + blank line; got %q", near(p, "Generate"))
				}
				if strings.HasPrefix(p, "Generate a commit message for these changes:") { // colon variant
					t.Error("rejection instruction must end with PERIOD, not COLON (design-decisions §2)")
				}
			},
		},
		{
			name: "rejection: IMPORTANT preamble + epilogue present", diff: diff, rejected: []string{"a"},
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, "IMPORTANT: The following messages were REJECTED because they already exist") {
					t.Error("rejection preamble line 1 missing")
				}
				if !strings.Contains(p, "in git history. You MUST generate something COMPLETELY DIFFERENT:") {
					t.Error("rejection preamble line 2 missing")
				}
				if !strings.Contains(p, "Create an entirely new message with different wording.") {
					t.Error("rejection epilogue missing")
				}
			},
		},
		{
			name: "rejection: each subject on its own '- ' line, in order", diff: diff, rejected: []string{"ONE", "TWO", "THREE"},
			check: func(t *testing.T, p string) {
				for _, want := range []string{"- ONE\n", "- TWO\n", "- THREE\n"} {
					if !strings.Contains(p, want) {
						t.Errorf("rejection list missing line %q", want)
					}
				}
				if got := strings.Count(p, "\n- "); got != 3 { // 3 subjects ⇒ 3 "- "-prefixed lines
					t.Errorf("expected 3 '- '-prefixed list lines; got %d", got)
				}
				// order
				i, j, k := strings.Index(p, "- ONE"), strings.Index(p, "- TWO"), strings.Index(p, "- THREE")
				if !(i < j && j < k) {
					t.Errorf("subjects out of order: ONE@%d TWO@%d THREE@%d", i, j, k)
				}
			},
		},
		{
			name: "rejection: single subject yields exactly one list line", diff: diff, rejected: []string{"solo"},
			check: func(t *testing.T, p string) {
				if got := strings.Count(p, "\n- "); got != 1 {
					t.Errorf("single subject ⇒ 1 list line; got %d", got)
				}
			},
		},
		{
			name: "diff is the exact tail — normal", diff: "TAIL_NORMAL\nnope", rejected: nil,
			check: func(t *testing.T, p string) {
				if !strings.HasSuffix(p, "TAIL_NORMAL\nnope") {
					t.Errorf("diff must be the verbatim tail; got suffix %q", suffix(p, 40))
				}
			},
		},
		{
			name: "diff is the exact tail — rejection", diff: "TAIL_REJECT", rejected: []string{"a"},
			check: func(t *testing.T, p string) {
				if !strings.HasSuffix(p, "TAIL_REJECT") {
					t.Errorf("diff must be the verbatim tail; got suffix %q", suffix(p, 40))
				}
			},
		},
		{
			name: "diff with trailing newline preserved verbatim", diff: "diff\n", rejected: nil,
			check: func(t *testing.T, p string) {
				if !strings.HasSuffix(p, "diff\n") {
					t.Error("trailing newline of diff must be preserved (no normalization)")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, BuildUserPayload(tc.diff, tc.rejected))
		})
	}
}

// TestBuildUserPayload_EdgeCases covers the defensive paths: empty diff (no panic), the nil==empty
// equivalence, and the blank-line topology count.
func TestBuildUserPayload_EdgeCases(t *testing.T) {
	t.Run("empty diff does not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("BuildUserPayload(\"\", nil) panicked: %v", r)
			}
		}()
		got := BuildUserPayload("", nil)
		const want = "Generate a commit message for these changes:\n\n"
		if got != want {
			t.Errorf("empty-diff normal payload = %q, want %q", got, want)
		}
	})

	t.Run("nil and empty rejected produce identical output", func(t *testing.T) {
		const diff = "D"
		if BuildUserPayload(diff, nil) != BuildUserPayload(diff, []string{}) {
			t.Error("nil and []string{} rejected must produce identical normal payloads")
		}
	})

	t.Run("rejection: exactly two blank lines separate epilogue from diff and list from epilogue", func(t *testing.T) {
		p := BuildUserPayload("DIFF", []string{"a"})
		// list item '- a\n' then '\n' (blank) then epilogue then '\n\n' (blank) then diff
		if !strings.Contains(p, "- a\n\nCreate an entirely new message with different wording.\n\nDIFF") {
			t.Errorf("rejection blank-line topology wrong around list/epilogue/diff; got %q", near(p, "Create an"))
		}
	})
}

// NOTE: `near` and `suffix` are already defined in system_test.go (same package). Do NOT redeclare them
// (compile error). Reuse them directly as shown above.
