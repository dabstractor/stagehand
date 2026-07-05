package git

import (
	"strings"
	"testing"
)

// Pure table tests for the FR3i truncation-application primitives (splitDiffSections /
// diffSectionPath / firstNRunes / truncateByWaterFill). Mirror numstat_test.go's
// TestResolveNumstatPath style: table-driven, HARDCODED expectations, t.Run subtests, PURE (no git
// repo, no t.TempDir, no I/O). Every case is a string literal — the functions under test are pure
// string arithmetic. (PRP §2 / numstat_test.go pattern.)
//
// All section literals use explicit \n — the index-stripped captured-section shape from
// stagediff_test.go's TestStripIndexLines (e.g. `diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n`).

// TestSplitDiffSections covers the `diff --git ` (LINE-ANCHORED, trailing-space) splitter: drop an empty
// leading element, preserve a non-empty leading element, [] for empty input. Pure; no I/O.
func TestSplitDiffSections(t *testing.T) {
	// A single canonical section (the index-stripped shape — no `index` line).
	one := "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n"

	tests := []struct {
		desc string
		in   string
		want []string
	}{
		{
			desc: "empty input → nil",
			in:   "",
			want: nil,
		},
		{
			desc: "whitespace-only input → nil",
			in:   "   \n  \t\n",
			want: nil,
		},
		{
			desc: "single section → one self-contained element (re-prefixed)",
			in:   one,
			want: []string{one},
		},
		{
			desc: "3-file aggregate → 3 sections, in order, each self-contained",
			in: "diff --git a/A b/A\n--- a/A\n+++ b/A\n@@ -1 +1 @@\n-a\n+a2\n" +
				"diff --git a/B b/B\n--- a/B\n+++ b/B\n@@ -1 +1 @@\n-b\n+b2\n" +
				"diff --git a/C b/C\n--- a/C\n+++ b/C\n@@ -1 +1 @@\n-c\n+c2\n",
			want: []string{
				"diff --git a/A b/A\n--- a/A\n+++ b/A\n@@ -1 +1 @@\n-a\n+a2\n",
				"diff --git a/B b/B\n--- a/B\n+++ b/B\n@@ -1 +1 @@\n-b\n+b2\n",
				"diff --git a/C b/C\n--- a/C\n+++ b/C\n@@ -1 +1 @@\n-c\n+c2\n",
			},
		},
		{
			desc: "non-empty leading element (PREAMBLE) preserved as its own leading section",
			in:   "PREAMBLE\ndiff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n",
			want: []string{
				"PREAMBLE\n",
				"diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n",
			},
		},
		{
			desc: "trailing content after last section is preserved (no whole-input TrimSpace)",
			in:   "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\nextra",
			want: []string{"diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\nextra"},
		},
		{
			desc: "content-embedded diff --git literal → ONE section (line-anchored, inert)",
			in:   "diff --git a/real.txt b/real.txt\n@@ -0,0 +1,1 @@\n+diff --git a/foo b/foo\n",
			want: []string{"diff --git a/real.txt b/real.txt\n@@ -0,0 +1,1 @@\n+diff --git a/foo b/foo\n"},
		},
		{
			desc: "first file body embeds diff --git literal → TWO sections (only real headers are boundaries)",
			in: "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,2 @@\n-old\n+diff --git a/embed b/embed\n" +
				"diff --git a/b.go b/b.go\n--- b/a.go\n+++ b/b.go\n@@ -1 +1 @@\n-b\n+b2\n",
			want: []string{
				"diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,2 @@\n-old\n+diff --git a/embed b/embed\n",
				"diff --git a/b.go b/b.go\n--- b/a.go\n+++ b/b.go\n@@ -1 +1 @@\n-b\n+b2\n",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := splitDiffSections(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("splitDiffSections len = %d, want %d; got=%q", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("section[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestDiffSectionPath covers destination-path extraction: normal edit, new-file, deletion, rename,
// +++-fallback, quote-strip, and the non-diff miss. Pure; no I/O.
func TestDiffSectionPath(t *testing.T) {
	tests := []struct {
		desc    string
		section string
		want    string
		wantOK  bool
	}{
		{
			desc:    "normal edit → destination from diff --git (group 2)",
			section: "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
			want:    "a.go",
			wantOK:  true,
		},
		{
			desc:    "new file → destination (diff --git wins; +++ b/ agrees)",
			section: "diff --git a/x.go b/x.go\nnew file mode 100644\n--- /dev/null\n+++ b/x.go\n@@ -1 +1 @@\n+new\n",
			want:    "x.go",
			wantOK:  true,
		},
		{
			desc:    "deletion → destination from diff --git (+++ /dev/null does not match b/)",
			section: "diff --git a/old.go b/old.go\ndeleted file mode 100644\n--- a/old.go\n+++ /dev/null\n@@ -1 +0 @@\n-old\n",
			want:    "old.go",
			wantOK:  true,
		},
		{
			desc:    "rename → NEW path (destination), from diff --git",
			section: "diff --git a/old.go b/new.go\nsimilarity index 80%\nrename from old.go\nrename to new.go\n--- a/old.go\n+++ b/new.go\n@@ -1 +1 @@\n-a\n+a2\n",
			want:    "new.go",
			wantOK:  true,
		},
		{
			desc:    "+++ fallback: no diff --git line, but +++ b/<p> present",
			section: "+++ b/fallback.go\n@@ -1 +1 @@\n-old\n+new\n",
			want:    "fallback.go",
			wantOK:  true,
		},
		{
			desc:    "quote-strip: one surrounding \" pair removed",
			section: "diff --git \"a/foo bar\" \"b/foo bar\"\n--- a/foo bar\n+++ b/foo bar\n@@ -1 +1 @@\n-old\n+new\n",
			want:    "foo bar",
			wantOK:  true,
		},
		{
			desc:    "path with subdirectories preserved",
			section: "diff --git a/src/pkg/file.go b/src/pkg/file.go\n--- a/src/pkg/file.go\n+++ b/src/pkg/file.go\n@@ -1 +1 @@\n-old\n+new\n",
			want:    "src/pkg/file.go",
			wantOK:  true,
		},
		{
			desc:    "non-diff string → ok=false",
			section: "M\t[binary] assets/logo.png\n",
			want:    "",
			wantOK:  false,
		},
		{
			desc:    "empty string → ok=false",
			section: "",
			want:    "",
			wantOK:  false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, ok := diffSectionPath(tc.section)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("diffSectionPath(%q) = (%q, %v), want (%q, %v)", tc.section, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestFirstNRunes covers the rune-boundary-safe prefix helper: n<=0, fewer-than-n, exact, multibyte
// boundary safety, and the allotment×4 inverse relationship. Pure; no I/O.
func TestFirstNRunes(t *testing.T) {
	tests := []struct {
		desc string
		s    string
		n    int
		want string
	}{
		{"n<=0 → empty", "hello", 0, ""},
		{"negative n → empty", "hello", -3, ""},
		{"empty s → empty", "", 5, ""},
		{"fewer than n runes → whole s", "abc", 10, "abc"},
		{"exact n → whole s", "abc", 3, "abc"},
		{"first 3 of 5 ASCII", "hello", 3, "hel"},
		{"multibyte: does not split a UTF-8 char (4 runes, take 2)", "héllo", 2, "hé"}, // é is 2 bytes
		{"CJK: 3 runes (6 bytes), take 2", "中文词", 2, "中文"},
		{"emoji: take 1 rune", "🎉🎊", 1, "🎉"},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			if got := firstNRunes(tc.s, tc.n); got != tc.want {
				t.Errorf("firstNRunes(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
			}
		})
	}
}

// TestTruncateByWaterFill covers the FR3i per-file truncation application: the item's named tests
// (3-file one-over-budget, headers preserved, sentinel form/line, byte-identical pass-through) plus
// all edge cases (all-within-budget, multi-hunk, markdown per-file, path-miss, pure-rename, empty,
// zero/negative-allotment). Pure; no I/O. All expectations HARDCODED.
func TestTruncateByWaterFill(t *testing.T) {
	// --- Helpers to build predictable bodies -----------------------------------------------
	// makeBody returns a body string of roughly `tokens` EstimateTokens (each line "~4 runes ≈ 1 token").
	// We don't depend on exactness here — only on the over/under-budget relations, which the cases pin.
	_ = func(tokens int) string {
		var sb strings.Builder
		for i := 0; i < tokens; i++ {
			sb.WriteString("xxxx\n") // 5 runes/line ≈ 1-2 tokens/line
		}
		return sb.String()
	}

	// === ITEM TEST: 3-file diff, one exceeds L (the named acceptance case) ===================
	t.Run("item_3file_one_over_budget_truncated_others_byte_identical", func(t *testing.T) {
		// A and C are SMALL (body well under any large allotment). B has a LARGE body that exceeds a
		// small allotment. The allotments key by numstat destination path.
		sectionA := "diff --git a/a/A.go b/a/A.go\n--- a/a/A.go\n+++ b/a/A.go\n@@ -1 +1 @@\n-a\n+a2\n"
		// B's body is a big block of content lines (>> 20 tokens).
		var bodyB strings.Builder
		bodyB.WriteString("@@ -1,100 +1,100 @@\n")
		for i := 0; i < 200; i++ {
			bodyB.WriteString("-old line content payload here number " + itoa(i) + "\n")
			bodyB.WriteString("+new line content payload here number " + itoa(i) + "\n")
		}
		bodyBStr := bodyB.String()
		sectionB := "diff --git a/b/B.go b/b/B.go\n--- a/b/B.go\n+++ b/b/B.go\n" + bodyBStr
		sectionC := "diff --git a/c/C.go b/c/C.go\n--- a/c/C.go\n+++ b/c/C.go\n@@ -1 +1 @@\n-c\n+c2\n"

		sections := []string{sectionA, sectionB, sectionC}
		// A,C get a huge allotment (within budget ⇒ byte-identical); B capped at 20 tokens.
		allotments := map[string]int{
			"a/A.go": 100000,
			"b/B.go": 20,
			"c/C.go": 100000,
		}

		out := truncateByWaterFill(sections, allotments)

		// (a) B is truncated — sentinel present.
		if !strings.Contains(out, "... [truncated]") {
			t.Errorf("expected the truncated sentinel on B's section; out=\n%s", out)
		}
		// (b) sentinel appears EXACTLY ONCE (only B; A and C within budget ⇒ none).
		if c := strings.Count(out, "... [truncated]"); c != 1 {
			t.Errorf("sentinel count = %d, want exactly 1 (only the over-budget file); out tail=\n%s", c, out)
		}
		// (c) A and C are BYTE-IDENTICAL to input (present verbatim; no sentinel appended).
		if !strings.Contains(out, sectionA) {
			t.Errorf("A section not byte-identical in output; missing %q", sectionA)
		}
		if !strings.Contains(out, sectionC) {
			t.Errorf("C section not byte-identical in output; missing %q", sectionC)
		}
		// (d) the legacy N-bearing sentinels do NOT appear.
		if strings.Contains(out, "[diff truncated at") {
			t.Errorf("legacy N-bearing sentinel leaked into output; out=\n%s", out)
		}
	})

	// === ITEM TEST: headers preserved on the truncated file =================================
	t.Run("item_headers_preserved_on_truncated_file", func(t *testing.T) {
		var bodyB strings.Builder
		bodyB.WriteString("@@ -1,50 +1,50 @@\n")
		for i := 0; i < 100; i++ {
			bodyB.WriteString("-old payload content line " + itoa(i) + " here\n")
			bodyB.WriteString("+new payload content line " + itoa(i) + " here\n")
		}
		sectionB := "diff --git a/b/B.go b/b/B.go\n--- a/b/B.go\n+++ b/b/B.go\n" + bodyB.String()
		allotments := map[string]int{"b/B.go": 20}

		out := truncateByWaterFill([]string{sectionB}, allotments)

		// All four headers must survive the truncation.
		for _, header := range []string{
			"diff --git a/b/B.go b/b/B.go",
			"--- a/b/B.go",
			"+++ b/b/B.go",
			"@@ -1,50 +1,50 @@",
		} {
			if !strings.Contains(out, header) {
				t.Errorf("header %q NOT preserved after truncation; out=\n%s", header, out)
			}
		}
	})

	// === ITEM TEST: sentinel on its own line ================================================
	t.Run("item_sentinel_on_its_own_line", func(t *testing.T) {
		var body strings.Builder
		body.WriteString("@@ -1,50 +1,50 @@\n")
		for i := 0; i < 100; i++ {
			body.WriteString("-old content line payload " + itoa(i) + "\n")
			body.WriteString("+new content line payload " + itoa(i) + "\n")
		}
		section := "diff --git a/big.go b/big.go\n--- a/big.go\n+++ b/big.go\n" + body.String()
		out := truncateByWaterFill([]string{section}, map[string]int{"big.go": 10})

		// The sentinel must be on its own line: preceded by \n, and itself ending the line (no trailing
		// content on the same line). It must be the SHORTER form (no N-bearing suffix).
		if !strings.HasSuffix(out, "\n... [truncated]\n") {
			t.Errorf("output must end with '\\n... [truncated]\\n' (own line, trailing newline); got tail=%q",
				tail(out, 40))
		}
	})

	// === EDGE: all files within budget → byte-identical, NO sentinels ========================
	t.Run("all_within_budget_byte_identical_no_sentinels", func(t *testing.T) {
		s1 := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-a\n+a2\n"
		s2 := "diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1 +1 @@\n-b\n+b2\n"
		sections := []string{s1, s2}
		// Generous allotments — both bodies well under.
		allotments := map[string]int{"a.go": 100000, "b.go": 100000}

		out := truncateByWaterFill(sections, allotments)
		want := s1 + s2 // joined in input order, byte-identical
		if out != want {
			t.Errorf("within-budget output not byte-identical\ngot:  %q\nwant: %q", out, want)
		}
		if c := strings.Count(out, "... [truncated]"); c != 0 {
			t.Errorf("expected 0 sentinels for all-within-budget, got %d", c)
		}
	})

	// === EDGE: multi-hunk file truncated mid-body — first @@ kept, later @@ cut ==============
	t.Run("multi_hunk_truncated_first_atat_kept_second_cut", func(t *testing.T) {
		var body strings.Builder
		body.WriteString("@@ -1,10 +1,10 @@\n")
		for i := 0; i < 10; i++ {
			body.WriteString("-first hunk old line payload " + itoa(i) + " content here\n")
			body.WriteString("+first hunk new line payload " + itoa(i) + " content here\n")
		}
		body.WriteString("@@ -50,10 +50,10 @@\n")
		for i := 0; i < 10; i++ {
			body.WriteString("-second hunk old line payload " + itoa(i) + " content here\n")
			body.WriteString("+second hunk new line payload " + itoa(i) + " content here\n")
		}
		section := "diff --git a/multi.go b/multi.go\n--- a/multi.go\n+++ b/multi.go\n" + body.String()
		// Small allotment — should cut partway through (before reaching the second @@).
		out := truncateByWaterFill([]string{section}, map[string]int{"multi.go": 30})

		// First @@ always present (it's at body start, within any positive allotment).
		if strings.Count(out, "@@ -1,10 +1,10 @@") != 1 {
			t.Errorf("first @@ header not present exactly once; out=\n%s", out)
		}
		// The sentinel is appended (content was removed).
		if !strings.HasSuffix(out, "\n... [truncated]\n") {
			t.Errorf("multi-hunk truncation must end with the sentinel on its own line; got tail=%q", tail(out, 40))
		}
	})

	// === EDGE: markdown per-file section uses the SAME code path =============================
	t.Run("markdown_per_file_section_same_code_path", func(t *testing.T) {
		var body strings.Builder
		body.WriteString("@@ -1,30 +1,30 @@\n")
		for i := 0; i < 30; i++ {
			body.WriteString("-old markdown line content payload index " + itoa(i) + "\n")
			body.WriteString("+new markdown line content payload index " + itoa(i) + "\n")
		}
		section := "diff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md\n" + body.String()
		out := truncateByWaterFill([]string{section}, map[string]int{"README.md": 10})

		if !strings.Contains(out, "diff --git a/README.md b/README.md") {
			t.Errorf("markdown header not preserved; out=\n%s", out)
		}
		if !strings.HasSuffix(out, "\n... [truncated]\n") {
			t.Errorf("markdown section must be truncated with the sentinel; got tail=%q", tail(out, 40))
		}
	})

	// === EDGE: path-miss (path absent from allotments) → verbatim pass-through ===============
	t.Run("path_miss_pass_through_verbatim", func(t *testing.T) {
		section := "diff --git a/unknown.go b/unknown.go\n--- a/unknown.go\n+++ b/unknown.go\n@@ -1 +1 @@\n-a\n+a2\n"
		// Empty allotments map → path-miss for every section.
		out := truncateByWaterFill([]string{section}, map[string]int{})
		if out != section {
			t.Errorf("path-miss output not verbatim\ngot:  %q\nwant: %q", out, section)
		}
		if strings.Contains(out, "... [truncated]") {
			t.Errorf("path-miss must NOT append a sentinel; out=\n%s", out)
		}
	})

	// === EDGE: path-miss via ok=false (non-diff section) → verbatim ==========================
	t.Run("non_diff_section_pass_through_verbatim", func(t *testing.T) {
		section := "M\t[binary] assets/logo.png\n"
		out := truncateByWaterFill([]string{section}, map[string]int{"assets/logo.png": 5})
		if out != section {
			t.Errorf("non-diff (ok=false) section not verbatim\ngot:  %q\nwant: %q", out, section)
		}
	})

	// === EDGE: pure-rename section (no @@) → verbatim, no truncation =========================
	t.Run("pure_rename_no_hunk_verbatim", func(t *testing.T) {
		section := "diff --git a/old b/new\nsimilarity index 100%\nrename from old\nrename to new\n"
		// A SMALL allotment would normally truncate — but with no @@ there's no body → verbatim.
		out := truncateByWaterFill([]string{section}, map[string]int{"new": 1})
		if out != section {
			t.Errorf("pure-rename (no @@) output not verbatim\ngot:  %q\nwant: %q", out, section)
		}
		if strings.Contains(out, "... [truncated]") {
			t.Errorf("pure-rename must NOT be truncated; out=\n%s", out)
		}
	})

	// === EDGE: zero / negative allotment treated as path-miss → verbatim =====================
	t.Run("zero_or_negative_allotment_verbatim", func(t *testing.T) {
		var body strings.Builder
		body.WriteString("@@ -1,50 +1,50 @@\n")
		for i := 0; i < 100; i++ {
			body.WriteString("+big content line " + itoa(i) + "\n")
		}
		section := "diff --git a/z.go b/z.go\n--- a/z.go\n+++ b/z.go\n" + body.String()

		for _, a := range []int{0, -5} {
			out := truncateByWaterFill([]string{section}, map[string]int{"z.go": a})
			if out != section {
				t.Errorf("allotment=%d should pass through verbatim\ngot len=%d, want len=%d", a, len(out), len(section))
			}
			if strings.Contains(out, "... [truncated]") {
				t.Errorf("allotment=%d must NOT truncate; out tail=%q", a, tail(out, 40))
			}
		}
	})

	// === EDGE: empty sections → "" ===========================================================
	t.Run("empty_sections_empty_string", func(t *testing.T) {
		if out := truncateByWaterFill(nil, map[string]int{"x": 10}); out != "" {
			t.Errorf("nil sections → %q, want %q", out, "")
		}
		if out := truncateByWaterFill([]string{}, map[string]int{"x": 10}); out != "" {
			t.Errorf("empty sections → %q, want %q", out, "")
		}
	})

	// === EDGE: recompose in INPUT order =====================================================
	t.Run("recompose_in_input_order", func(t *testing.T) {
		s1 := "diff --git a/aaa.go b/aaa.go\n--- a/aaa.go\n+++ b/aaa.go\n@@ -1 +1 @@\n-a\n+a2\n"
		s2 := "diff --git a/bbb.go b/bbb.go\n--- a/bbb.go\n+++ b/bbb.go\n@@ -1 +1 @@\n-b\n+b2\n"
		s3 := "diff --git a/ccc.go b/ccc.go\n--- a/ccc.go\n+++ b/ccc.go\n@@ -1 +1 @@\n-c\n+c2\n"
		sections := []string{s1, s2, s3}
		// All within budget → byte-identical JOIN IN INPUT ORDER (not sorted).
		out := truncateByWaterFill(sections, map[string]int{
			"aaa.go": 100000, "bbb.go": 100000, "ccc.go": 100000,
		})
		want := s1 + s2 + s3
		if out != want {
			t.Errorf("recomposition not in input order\ngot:  %q\nwant: %q", out, want)
		}
	})

	// === EDGE: exact boundary — EstimateTokens(body) == allotment → NO truncation ============
	t.Run("body_exactly_at_allotment_no_truncation", func(t *testing.T) {
		// Build a body whose EstimateTokens is exactly the allotment (boundary: > triggers, == does not).
		// allotment×4 runes ⇒ EstimateTokens(firstNRunes(body, allotment×4)) == allotment.
		// Use a body of exactly allotment×4 runes ⇒ EstimateTokens = allotment ⇒ NOT > allotment ⇒ verbatim.
		allotment := 10
		runeBudget := allotment * 4 // 40 runes
		// Body: "@@ ...\n" (8 runes incl \n) + (runeBudget-8) 'x' runes on content lines.
		hdr := "@@ -1 +1 @@\n"
		// Fill the rest with runeBudget-len(hdr) runes of content (no trailing newline so the count is exact).
		fill := strings.Repeat("x", runeBudget-len(hdr))
		body := hdr + fill
		// Sanity: EstimateTokens(body) should equal allotment.
		if got := EstimateTokens(body); got != allotment {
			t.Fatalf("test setup: EstimateTokens(body)=%d, want %d (body=%q)", got, allotment, body)
		}
		section := "diff --git a/exact.go b/exact.go\n--- a/exact.go\n+++ b/exact.go\n" + body
		out := truncateByWaterFill([]string{section}, map[string]int{"exact.go": allotment})
		if out != section {
			t.Errorf("body exactly at allotment should be verbatim\ngot:  %q\nwant: %q", out, section)
		}
		if strings.Contains(out, "... [truncated]") {
			t.Errorf("body exactly at allotment must NOT be truncated; out=\n%s", out)
		}
	})

	// === REGRESSION (FR3i bugfix): truncated NON-LAST section must not glue its sentinel =============
	// to the next section's `diff --git` header. Before the fix the sentinel was emitted WITHOUT a
	// trailing "\n", so a truncated non-last section ran into the following `diff --git` on one line
	// ("... [truncated]diff --git a/b.go b/b.go"). The fix appends "\n" after the sentinel in the
	// truncation branch so every truncated section ends "... [truncated]\n" and the next `diff --git`
	// begins at a line start. These four subtests cover the non-last-section gap that hid the bug
	// (the existing tests truncated only the LAST/only section). bigBody builds a body that comfortably
	// exceeds a small allotment (10 → 40 runes); the canonical 1-line body stays within budget under a
	// large allotment (100000).
	bigBody := func(n int) string {
		var sb strings.Builder
		sb.WriteString("@@ -1,100 +1,100 @@\n")
		for i := 0; i < n; i++ {
			sb.WriteString("-old content line payload " + itoa(i) + " here\n")
			sb.WriteString("+new content line payload " + itoa(i) + " here\n")
		}
		return sb.String()
	}

	// (a) non-md truncated → non-md within budget.
	t.Run("nonmd_truncated_then_nonmd_within_budget", func(t *testing.T) {
		sectionA := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n" + bigBody(60)
		sectionB := "diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1 +1 @@\n-b\n+b2\n"
		out := truncateByWaterFill([]string{sectionA, sectionB}, map[string]int{"a.go": 10, "b.go": 100000})

		if strings.Contains(out, "[truncated]diff --git") {
			t.Errorf("sentinel glued to next diff --git (no newline separator); out=\n%s", out)
		}
		if !strings.Contains(out, "... [truncated]\ndiff --git a/b.go b/b.go") {
			t.Errorf("next diff --git not at a line start after the sentinel; out=\n%s", out)
		}
		if c := strings.Count(out, "... [truncated]"); c != 1 {
			t.Errorf("sentinel count = %d, want 1 (only A truncated); out=\n%s", c, out)
		}
		if !strings.Contains(out, sectionB) {
			t.Errorf("B section not byte-identical in output; out=\n%s", out)
		}
	})

	// (b) md truncated → non-md within budget.
	t.Run("md_truncated_then_nonmd", func(t *testing.T) {
		sectionA := "diff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md\n" + bigBody(60)
		sectionB := "diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1 +1 @@\n-b\n+b2\n"
		out := truncateByWaterFill([]string{sectionA, sectionB}, map[string]int{"README.md": 10, "b.go": 100000})

		if strings.Contains(out, "[truncated]diff --git") {
			t.Errorf("sentinel glued to next diff --git (no newline separator); out=\n%s", out)
		}
		if !strings.Contains(out, "... [truncated]\ndiff --git a/b.go b/b.go") {
			t.Errorf("next diff --git not at a line start after the sentinel; out=\n%s", out)
		}
		if c := strings.Count(out, "... [truncated]"); c != 1 {
			t.Errorf("sentinel count = %d, want 1 (only the md section truncated); out=\n%s", c, out)
		}
	})

	// (c) non-md truncated → md within budget.
	t.Run("nonmd_truncated_then_md", func(t *testing.T) {
		sectionA := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n" + bigBody(60)
		sectionB := "diff --git a/NOTES.md b/NOTES.md\n--- a/NOTES.md\n+++ b/NOTES.md\n@@ -1 +1 @@\n-x\n+y\n"
		out := truncateByWaterFill([]string{sectionA, sectionB}, map[string]int{"a.go": 10, "NOTES.md": 100000})

		if strings.Contains(out, "[truncated]diff --git") {
			t.Errorf("sentinel glued to next diff --git (no newline separator); out=\n%s", out)
		}
		if !strings.Contains(out, "... [truncated]\ndiff --git a/NOTES.md b/NOTES.md") {
			t.Errorf("next diff --git not at a line start after the sentinel; out=\n%s", out)
		}
		if c := strings.Count(out, "... [truncated]"); c != 1 {
			t.Errorf("sentinel count = %d, want 1 (only A truncated); out=\n%s", c, out)
		}
	})

	// (d) both non-md truncated — also covers the trailing "\n" when the LAST section is truncated.
	t.Run("both_nonmd_truncated", func(t *testing.T) {
		sectionA := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n" + bigBody(60)
		sectionB := "diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n" + bigBody(60)
		out := truncateByWaterFill([]string{sectionA, sectionB}, map[string]int{"a.go": 10, "b.go": 10})

		if strings.Contains(out, "[truncated]diff --git") {
			t.Errorf("sentinel glued to next diff --git (no newline separator); out=\n%s", out)
		}
		if !strings.Contains(out, "... [truncated]\ndiff --git a/b.go b/b.go") {
			t.Errorf("A's sentinel → B's header: next diff --git not at a line start; out=\n%s", out)
		}
		if c := strings.Count(out, "... [truncated]"); c != 2 {
			t.Errorf("sentinel count = %d, want 2 (both truncated); out=\n%s", c, out)
		}
		if !strings.HasSuffix(out, "... [truncated]\n") {
			t.Errorf("last section truncated → output must end with '... [truncated]\\n'; got tail=%q", tail(out, 40))
		}
	})
}

// itoa is a tiny strconv.Itoa stand-in to avoid the strconv import in this pure test file.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// tail returns the last n bytes of s (or s if shorter), for readable error messages.
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
