package generate

import (
	"strings"
	"testing"
)

// TestFormatRescue_RootedNoCandidate asserts the FULL assembled rescue message for the
// rooted/no-candidate case is byte-for-byte the §18.3 rendering with -p <parentSHA>
// present and NO candidate note. Independently derived from PRD §18.3 (not from the
// implementation) so a match is meaningful.
func TestFormatRescue_RootedNoCandidate(t *testing.T) {
	treeSHA, parentSHA := "9f3a1c", "abc1234"
	sep := strings.Repeat("-", 60)
	want := "❌ Commit generation failed.\n" +
		sep + "\n" +
		"Your staged files were safely snapshotted before generation.\n" +
		"Tree ID: " + treeSHA + "\n" +
		"\n" +
		"To commit the originally staged files manually:\n" +
		"  git commit-tree -p " + parentSHA + ` -m "Your message" ` + treeSHA + " | xargs git update-ref HEAD\n" +
		"\n" +
		`(omit "-p <PARENT_SHA>" if this is the repository's first commit)` + "\n" +
		sep // NO trailing newline
	got := FormatRescue(treeSHA, parentSHA, "")
	if got != want {
		t.Errorf("FormatRescue rooted/no-candidate mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestFormatRescue_RootlessNoCandidate asserts the FULL assembled rescue message for the
// root/unborn case (parentSHA == "") omits the -p segment entirely, while the hint line
// is STILL present. Independently derived from PRD §18.3.
func TestFormatRescue_RootlessNoCandidate(t *testing.T) {
	treeSHA := "9f3a1c"
	sep := strings.Repeat("-", 60)
	want := "❌ Commit generation failed.\n" +
		sep + "\n" +
		"Your staged files were safely snapshotted before generation.\n" +
		"Tree ID: " + treeSHA + "\n" +
		"\n" +
		"To commit the originally staged files manually:\n" +
		`  git commit-tree -m "Your message" ` + treeSHA + " | xargs git update-ref HEAD\n" +
		"\n" +
		`(omit "-p <PARENT_SHA>" if this is the repository's first commit)` + "\n" +
		sep // NO trailing newline
	got := FormatRescue(treeSHA, "", "")
	if got != want {
		t.Errorf("FormatRescue rootless/no-candidate mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestFormatRescue_RootedWithCandidate asserts the rooted message + the §18.3 candidate note
// appended after the closing separator with one blank line, with the message in literal quotes.
func TestFormatRescue_RootedWithCandidate(t *testing.T) {
	treeSHA, parentSHA := "9f3a1c", "abc1234"
	candidateMsg := "feat: add bar"
	sep := strings.Repeat("-", 60)
	want := "❌ Commit generation failed.\n" +
		sep + "\n" +
		"Your staged files were safely snapshotted before generation.\n" +
		"Tree ID: " + treeSHA + "\n" +
		"\n" +
		"To commit the originally staged files manually:\n" +
		"  git commit-tree -p " + parentSHA + ` -m "Your message" ` + treeSHA + " | xargs git update-ref HEAD\n" +
		"\n" +
		`(omit "-p <PARENT_SHA>" if this is the repository's first commit)` + "\n" +
		sep +
		"\n\n" +
		`A candidate message was produced but rejected: "feat: add bar". You can use it manually in the command above.`
	got := FormatRescue(treeSHA, parentSHA, candidateMsg)
	if got != want {
		t.Errorf("FormatRescue rooted/with-candidate mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestFormatRescue_RootlessWithCandidate asserts the rootless message + the §18.3 candidate note.
func TestFormatRescue_RootlessWithCandidate(t *testing.T) {
	treeSHA := "9f3a1c"
	candidateMsg := "fix: x"
	sep := strings.Repeat("-", 60)
	want := "❌ Commit generation failed.\n" +
		sep + "\n" +
		"Your staged files were safely snapshotted before generation.\n" +
		"Tree ID: " + treeSHA + "\n" +
		"\n" +
		"To commit the originally staged files manually:\n" +
		`  git commit-tree -m "Your message" ` + treeSHA + " | xargs git update-ref HEAD\n" +
		"\n" +
		`(omit "-p <PARENT_SHA>" if this is the repository's first commit)` + "\n" +
		sep +
		"\n\n" +
		`A candidate message was produced but rejected: "fix: x". You can use it manually in the command above.`
	got := FormatRescue(treeSHA, "", candidateMsg)
	if got != want {
		t.Errorf("FormatRescue rootless/with-candidate mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestFormatRescue_Properties is a table of structural invariants guarding the load-bearing
// decisions: ❌ present, 2× 60-dash separators, -p iff rooted, Tree ID line, hint line always
// present, candidate note iff candidateMsg != "", no "interrupted", no trailing newline.
func TestFormatRescue_Properties(t *testing.T) {
	cases := []struct {
		name         string
		treeSHA      string
		parentSHA    string
		candidateMsg string
	}{
		{"rooted no candidate", "9f3a1c", "abc1234", ""},
		{"rootless no candidate", "9f3a1c", "", ""},
		{"rooted with candidate", "9f3a1c", "abc1234", "feat: add bar"},
		{"rootless with candidate", "9f3a1c", "", "fix: x"},
		{"empty treeSHA (defensive)", "", "abc1234", ""},
		{"candidate with quote", "9f3a1c", "abc1234", `he said "hello"`},
		{"candidate with newline", "9f3a1c", "", "line1\nline2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatRescue(tc.treeSHA, tc.parentSHA, tc.candidateMsg)
			sep := strings.Repeat("-", 60)

			// ❌ present (U+274C)
			if !strings.Contains(got, "❌") {
				t.Error("output must contain ❌ (U+274C)")
			}

			// No "(interrupted)"
			if strings.Contains(got, "interrupted") {
				t.Error("output must NOT contain 'interrupted'")
			}

			// No trailing newline
			if strings.HasSuffix(got, "\n") {
				t.Error("output must NOT end with \\n")
			}

			// Exactly 2 separator lines, each exactly 60 dashes
			lines := strings.Split(got, "\n")
			sepLines := 0
			for _, line := range lines {
				if line == sep {
					sepLines++
				}
			}
			if sepLines != 2 {
				t.Errorf("expected exactly 2 separator lines of 60 dashes; got %d", sepLines)
			}

			// Tree ID line present
			treeIDLine := "Tree ID: " + tc.treeSHA
			if !strings.Contains(got, treeIDLine) {
				t.Errorf("output must contain %q", treeIDLine)
			}

			// Command line has 2 leading spaces and contains treeSHA
			if !strings.Contains(got, "  git commit-tree") {
				t.Error("command line must start with exactly 2 leading spaces")
			}
			// treeSHA appears in the command
			if tc.treeSHA != "" && !strings.Contains(got, tc.treeSHA+" | xargs") {
				t.Errorf("command must contain treeSHA before xargs; got %q", got)
			}

			// -p present iff parentSHA != "" (check only the command line, not the hint line)
			var cmdLine string
			for _, line := range lines {
				if strings.HasPrefix(line, "  git commit-tree") {
					cmdLine = line
					break
				}
			}
			if tc.parentSHA != "" {
				if !strings.Contains(cmdLine, "-p "+tc.parentSHA) {
					t.Errorf("rooted command must contain '-p %s'; got %q", tc.parentSHA, cmdLine)
				}
			} else {
				if strings.Contains(cmdLine, "-p ") {
					t.Errorf("rootless command must NOT contain '-p'; got %q", cmdLine)
				}
			}

			// Hint line present ALWAYS (§18.3 verbatim, design-decisions §3)
			if !strings.Contains(got, `(omit "-p <PARENT_SHA>" if this is the repository's first commit)`) {
				t.Error("hint line must be present in BOTH rooted and rootless cases")
			}

			// Candidate note iff candidateMsg != ""
			if tc.candidateMsg != "" {
				noteFragment := `"` + tc.candidateMsg + `"`
				if !strings.Contains(got, noteFragment) {
					t.Errorf("candidate note must contain %q (with literal quotes)", noteFragment)
				}
				if !strings.Contains(got, "A candidate message was produced but rejected:") {
					t.Error("candidate note preamble missing")
				}
			} else {
				if strings.Contains(got, "A candidate message was produced but rejected:") {
					t.Error("candidate note must NOT be present when candidateMsg is empty")
				}
			}
		})
	}
}

// TestFormatRescueMulti_TitledRooted asserts the FULL assembled multi-commit rescue for the
// titled+rooted case (concept 2 of 3, with parent, no candidate). Independently derived from PRD §18.3.
func TestFormatRescueMulti_TitledRooted(t *testing.T) {
	treeSHA, parentSHA := "9f3a1c", "abc1234"
	sep := strings.Repeat("-", 60)
	want := "❌ Commit generation failed for concept 2 of 3: feat: add b.\n" +
		sep + "\n" +
		"Concepts already published are final and untouched. Remaining staged changes are safe in your index.\n" +
		"Tree ID: " + treeSHA + "\n" +
		"\n" +
		"To commit this concept's staged files manually:\n" +
		"  git commit-tree -p " + parentSHA + ` -m "Your message" ` + treeSHA + " | xargs git update-ref HEAD\n" +
		"\n" +
		`(omit "-p <PARENT_SHA>" if this is the repository's first commit)` + "\n" +
		sep // NO trailing newline
	got := FormatRescueMulti(treeSHA, parentSHA, "", "feat: add b", 1, 3)
	if got != want {
		t.Errorf("FormatRescueMulti titled/rooted mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestFormatRescueMulti_RootlessNoTitle asserts the root/unborn case with no concept title.
func TestFormatRescueMulti_RootlessNoTitle(t *testing.T) {
	treeSHA := "9f3a1c"
	sep := strings.Repeat("-", 60)
	want := "❌ Commit generation failed for concept 1 of 2.\n" +
		sep + "\n" +
		"Concepts already published are final and untouched. Remaining staged changes are safe in your index.\n" +
		"Tree ID: " + treeSHA + "\n" +
		"\n" +
		"To commit this concept's staged files manually:\n" +
		`  git commit-tree -m "Your message" ` + treeSHA + " | xargs git update-ref HEAD\n" +
		"\n" +
		`(omit "-p <PARENT_SHA>" if this is the repository's first commit)` + "\n" +
		sep
	got := FormatRescueMulti(treeSHA, "", "", "", 0, 2)
	if got != want {
		t.Errorf("FormatRescueMulti rootless/no-title mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestFormatRescueMulti_WithCandidate asserts the candidate note is appended after the closing sep.
func TestFormatRescueMulti_WithCandidate(t *testing.T) {
	treeSHA, parentSHA := "9f3a1c", "abc1234"
	candidate := "feat: add c"
	sep := strings.Repeat("-", 60)
	want := "❌ Commit generation failed for concept 3 of 3: fix: something.\n" +
		sep + "\n" +
		"Concepts already published are final and untouched. Remaining staged changes are safe in your index.\n" +
		"Tree ID: " + treeSHA + "\n" +
		"\n" +
		"To commit this concept's staged files manually:\n" +
		"  git commit-tree -p " + parentSHA + ` -m "Your message" ` + treeSHA + " | xargs git update-ref HEAD\n" +
		"\n" +
		`(omit "-p <PARENT_SHA>" if this is the repository's first commit)` + "\n" +
		sep +
		"\n\n" +
		`A candidate message was produced but rejected: "feat: add c". You can use it manually in the command above.`
	got := FormatRescueMulti(treeSHA, parentSHA, candidate, "fix: something", 2, 3)
	if got != want {
		t.Errorf("FormatRescueMulti with-candidate mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestFormatRescue_Properties pins the separator constant to exactly 60 hyphens.
func TestFormatRescue_RescueSepLen(t *testing.T) {
	if len(rescueSep) != 60 {
		t.Errorf("rescueSep length = %d, want 60", len(rescueSep))
	}
	for i, c := range rescueSep {
		if c != '-' {
			t.Errorf("rescueSep[%d] = %q, want '-'", i, c)
		}
	}
}
