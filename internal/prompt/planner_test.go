package prompt

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestBuildPlannerSystemPrompt_CanonicalExact asserts the FULL assembled string for a known input,
// pinning the PRD §17.5 blank-line topology byte-for-byte. Independently derived from PRD §17.5
// (not from the implementation) so a match is meaningful. AUTO mode (forcedCount=0, maxCommits=12
// ⇒ soft target 6).
func TestBuildPlannerSystemPrompt_CanonicalExact(t *testing.T) {
	examples := []string{"feat: a", "fix: b\n\nBody."}
	got := BuildPlannerSystemPrompt(examples, "auto", "", 0, 12)

	const want = "You are a commit-planning assistant. Given a diff of un-staged changes, decide whether they\n" +
		"form ONE coherent commit or SEVERAL, and partition them into logical units.\n" +
		"\n" +
		"These changes were left UNSTAGED on purpose and handed to you to organize -- finding the real\n" +
		"commit boundaries is the job you were asked to do, not a fallback to resist.\n" +
		"\n" +
		"Rules:\n" +
		"- Split changes that serve DIFFERENT purposes into separate commits. Two changes you would\n" +
		"  describe with different verbs, or explain to a reviewer in separate sentences, almost always\n" +
		"  belong in separate commits. When torn between one commit and several, lean toward SEVERAL.\n" +
		"- Do not manufacture tiny commits. Group changes that only make sense together (a function plus\n" +
		"  its test, a refactor plus the callers it updates). A single commit is correct only when the\n" +
		"  whole changeset pursues ONE purpose.\n" +
		"- Keep the count modest: in ordinary cases at or below 6 (half the max of 12). Only exceed that\n" +
		"  when the changes genuinely span many unrelated concerns; do not approach the max casually.\n" +
		"- Account for every changed path: each file in the diff should appear in some commit's \"files\".\n" +
		"  A single file may be split across two concepts -- name it in both and say, per file, WHICH\n" +
		"  part belongs here.\n" +
		"- Each commit must be independently meaningful and reviewable.\n" +
		"- Respect dependencies: if change B depends on change A, A comes first.\n" +
		"- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.\n" +
		"\n" +
		"Respond with ONLY JSON, no prose, no code fences:\n" +
		`{"count": <int>, "single": <bool>, "commits": [{"title": "<short concept>", "description": "<which change belongs here, per file>", "files": ["<path>", ...]}, ...]}` + "\n" +
		`- If single is true, set count=1 and ALSO include "message": "<the full commit message>".` + "\n" +
		`- "files" must list every path this commit touches; "description" must say, per file, WHICH` + "\n" +
		"  change belongs to this commit so a stager can find the exact hunks. Do NOT emit hunks or\n" +
		"  line numbers.\n" +
		"\n" +
		"---\n" +
		"feat: a\n" +
		"---\n" +
		"fix: b\n" +
		"\n" +
		"Body.\n"

	if got != want {
		t.Errorf("BuildPlannerSystemPrompt mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestBuildPlannerSystemPrompt_ForcedCount_CanonicalExact asserts the byte-identity of the SHARED
// blocks across modes (opener+framing is a TRUE contiguous prefix; plannerJSONContract is an identical
// NON-contiguous substring) plus the forced-only markers (the directive PRESENT; the auto-only soft
// target and "lean toward SEVERAL" bullet ABSENT).
func TestBuildPlannerSystemPrompt_ForcedCount_CanonicalExact(t *testing.T) {
	examples := []string{"feat: a", "fix: b\n\nBody."}
	autoP := BuildPlannerSystemPrompt(examples, "auto", "", 0, 12)
	forcedP := BuildPlannerSystemPrompt(examples, "auto", "", 3, 12)

	// §3 identity: opener+framing is a TRUE contiguous prefix of both modes.
	sharedPrefix := plannerOpener + "\n\n" + plannerUnstagedFraming
	if !strings.HasPrefix(forcedP, sharedPrefix) || !strings.HasPrefix(autoP, sharedPrefix) {
		t.Error("opener+framing must be a byte-identical contiguous prefix of both modes")
	}
	// §3 identity: plannerJSONContract is byte-identical (same substring) in both, but NON-contiguous
	// (the differing rules block sits between the prefix and the contract).
	if !strings.Contains(forcedP, plannerJSONContract) || !strings.Contains(autoP, plannerJSONContract) {
		t.Error("plannerJSONContract must be byte-identical (same substring) in both modes")
	}
	// Forced-only markers.
	if !strings.Contains(forcedP, "You MUST partition into EXACTLY") {
		t.Error("forced prompt must contain the forced-count directive")
	}
	if strings.Contains(forcedP, "Keep the count modest") {
		t.Error("forced prompt must NOT contain the soft-target line")
	}
	if strings.Contains(forcedP, "lean toward SEVERAL") {
		t.Error("forced prompt must NOT contain the auto-only 'lean toward SEVERAL' bullet")
	}
}

// TestBuildPlannerSystemPrompt_SoftTarget_Interpolation verifies the soft target uses Go integer
// division (maxCommits/2) and that the FORCED prompt carries NEITHER number.
func TestBuildPlannerSystemPrompt_SoftTarget_Interpolation(t *testing.T) {
	for _, tc := range []struct{ max, half int }{{12, 6}, {10, 5}, {20, 10}, {4, 2}} {
		auto := BuildPlannerSystemPrompt(nil, "auto", "", 0, tc.max)
		wantLine := fmt.Sprintf("at or below %d (half the max of %d)", tc.half, tc.max)
		if !strings.Contains(auto, wantLine) {
			t.Errorf("max=%d: want %q", tc.max, wantLine)
		}
		forced := BuildPlannerSystemPrompt(nil, "auto", "", 3, tc.max)
		if strings.Contains(forced, fmt.Sprintf("half the max of %d", tc.max)) {
			t.Errorf("max=%d: forced prompt must not carry the soft-target line", tc.max)
		}
	}
}

// TestBuildPlannerSystemPrompt_Properties is a table of structural invariants on the assembled prompt,
// including anti-copy-paste guards that pin §17.1 mature-prompt elements are ABSENT (the #1 risk).
func TestBuildPlannerSystemPrompt_Properties(t *testing.T) {
	examples := []string{"ALPHA", "BETA", "GAMMA"}
	p := BuildPlannerSystemPrompt(examples, "auto", "", 0, 12)

	cases := []struct {
		name      string
		needle    string
		mustExist bool
	}{
		// §17.5 elements PRESENT.
		{"role is commit-PLANNING assistant", "You are a commit-planning assistant.", true},
		{"JSON contract line PRESENT verbatim", `{"count": <int>, "single": <bool>`, true},
		{"single/message clause PRESENT", "If single is true, set count=1", true},
		{"files in JSON contract PRESENT", `"files": ["<path>"`, true},
		{"auto: lean toward SEVERAL PRESENT", "lean toward SEVERAL", true},
		{"auto: soft target PRESENT", "at or below 6 (half the max of 12)", true},

		// §17.1 mature elements ABSENT (anti-copy-paste guards).
		{"§17.1 'commit message generator' ABSENT", "You are a commit message generator", false},
		{"§17.1 anti-reuse block ABSENT", "CRITICAL: You MUST NOT copy", false},
		{"§17.1 subject-target line ABSENT", "Target ~", false},
		{"§17.1 multi-line rule ABSENT", "multi-line commits AND", false},
		{"§17.1 examples intro ABSENT", "Match the tone and style of these recent commits from this repository:", false},
		// Forced-only directive ABSENT in auto mode (mode-conditional rule).
		{"forced-only 'MUST partition into EXACTLY' ABSENT in auto", "You MUST partition into EXACTLY", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			has := strings.Contains(p, tc.needle)
			if tc.mustExist && !has {
				t.Errorf("expected %q in planner prompt; not found", tc.needle)
			}
			if !tc.mustExist && has {
				t.Errorf("planner prompt must NOT contain §17.1 element %q (copy-paste leak)", tc.needle)
			}
		})
	}

	// "---" count == len(examples).
	if got := strings.Count(p, "---"); got != 3 {
		t.Errorf("--- count = %d, want 3 (one before each example)", got)
	}

	// Examples appear in order.
	i := strings.Index(p, "ALPHA")
	j := strings.Index(p, "BETA")
	k := strings.Index(p, "GAMMA")
	if i < 0 || j < 0 || k < 0 || !(i < j && j < k) {
		t.Errorf("examples out of order: ALPHA@%d BETA@%d GAMMA@%d", i, j, k)
	}
}

// TestBuildPlannerSystemPrompt_EmptyExamples verifies the defensive path: nil/empty examples must not
// panic and must omit all "---" lines while keeping the header.
func TestBuildPlannerSystemPrompt_EmptyExamples(t *testing.T) {
	for _, ex := range [][]string{nil, {}} {
		p := BuildPlannerSystemPrompt(ex, "auto", "", 0, 12) // must not panic
		if strings.Contains(p, "---") {
			t.Errorf("empty examples must emit no '---' lines; got %q", p)
		}
		if !strings.Contains(p, "You are a commit-planning assistant.") {
			t.Error("empty-examples prompt missing the planner header")
		}
		if !strings.Contains(p, "line numbers.") {
			t.Error("empty-examples prompt missing the JSON contract section")
		}
	}
}

// TestBuildPlannerSystemPrompt_FormatModes_CanonicalExact pins the exact §17.5/§17.8 assembly when
// format != "auto": the shared preamble (opener + UNSTAGED framing + JSON contract) is byte-identical,
// but the trailing style-examples block is REPLACED by the format scaffold body (FR-F5), and locale
// appends its line (FR-F6). The rules block is AUTO here (forcedCount=0) — the two dimensions
// (rules-block via forcedCount; examples/scaffold via format) are ORTHOGONAL.
func TestBuildPlannerSystemPrompt_FormatModes_CanonicalExact(t *testing.T) {
	examples := []string{"feat: a", "fix: b"} // IGNORED in non-auto modes

	// forcedCount=0 ⇒ auto rules with soft target 6/12, REGARDLESS of format (the dimensions are
	// orthogonal: you can have a conventional format scaffold WITH the auto rules).
	sharedAuto := plannerOpener + "\n\n" + plannerUnstagedFraming + "\n\n" +
		fmt.Sprintf(plannerAutoRules, 6, 12) + "\n\n" + plannerJSONContract

	cases := []struct {
		name   string
		format string
		locale string
		want   string
	}{
		{
			name: "conventional, no locale", format: "conventional", locale: "",
			want: sharedAuto + "\n\n" + conventionalScaffold,
		},
		{
			name: "conventional, locale French", format: "conventional", locale: "French",
			want: sharedAuto + "\n\n" + conventionalScaffold + "\nWrite the commit message in French.",
		},
		{
			name: "gitmoji, no locale", format: "gitmoji", locale: "",
			want: sharedAuto + "\n\n" + gitmojiScaffoldInstruction + "\n\n" + RenderGitmojiTable(),
		},
		{
			name: "plain, no locale", format: "plain", locale: "",
			want: sharedAuto + "\n\n", // scaffold body is "" for plain
		},
		{
			name: "plain, locale ja", format: "plain", locale: "ja",
			want: sharedAuto + "\nWrite the commit message in ja.", // withLocale trims the trailing "\n\n" to "\n"
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildPlannerSystemPrompt(examples, tc.format, tc.locale, 0, 12)
			if got != tc.want {
				t.Errorf("BuildPlannerSystemPrompt(%q, %q) mismatch:\n--- got ---\n%q\n--- want ---\n%q", tc.format, tc.locale, got, tc.want)
			}
		})
	}
}

// TestBuildPlannerSystemPrompt_FormatModes_Properties asserts the shared preamble survives verbatim
// in every mode (FR-F5) while the examples/scaffold swap behaves per §17.8.
func TestBuildPlannerSystemPrompt_FormatModes_Properties(t *testing.T) {
	examples := []string{"feat: a", "fix: b"}
	for _, format := range []string{"conventional", "gitmoji", "plain"} {
		t.Run(format, func(t *testing.T) {
			p := BuildPlannerSystemPrompt(examples, format, "", 0, 12)

			if !strings.Contains(p, "You are a commit-planning assistant.") {
				t.Error("partitioning contract role line must survive unchanged (FR-F5)")
			}
			if !strings.Contains(p, `{"count": <int>, "single": <bool>`) {
				t.Error("partitioning JSON contract must survive unchanged (FR-F5)")
			}
			if strings.Contains(p, "---") {
				t.Error("non-auto planner prompt must not embed '---' example markers")
			}
			if strings.Contains(p, "feat: a") || strings.Contains(p, "fix: b") {
				t.Error("history examples must NOT be embedded in non-auto planner modes")
			}
		})
	}

	// auto path unaffected by the new params (regression: identical to the pre-existing behavior).
	autoGot := BuildPlannerSystemPrompt(examples, "auto", "", 0, 12)
	if !strings.Contains(autoGot, "---\nfeat: a") {
		t.Error("auto mode must still embed the style examples")
	}
}

// TestBuildPlannerUserPayload_NormalCanonicalExact asserts the FULL assembled NORMAL payload (forcedCount==0)
// is byte-for-byte the §17.5 rendering: instruction + blank line + diff verbatim. Independently derived.
func TestBuildPlannerUserPayload_NormalCanonicalExact(t *testing.T) {
	const diff = "diff --git a/foo.go b/foo.go\n@@ -1,3 +1,4 @@"
	const want = "Decompose these un-staged changes into commits:\n\n" + diff

	for _, fc := range []int{0, -1, -5} {
		got := BuildPlannerUserPayload(diff, "", fc)
		if got != want {
			t.Errorf("BuildPlannerUserPayload(diff, \"\", %d) mismatch:\n--- got ---\n%q\n--- want ---\n%q", fc, got, want)
		}
	}
}

// TestBuildPlannerUserPayload_ForcedCanonicalExact asserts the FULL assembled FORCED payload (forcedCount>0)
// is byte-for-byte the §17.5 rendering: forced directive + newline + instruction + blank + diff.
func TestBuildPlannerUserPayload_ForcedCanonicalExact(t *testing.T) {
	const diff = "diff --git a/foo.go b/foo.go\n@@ -1,3 +1,4 @@"
	const want = "Produce EXACTLY 3 commits from these changes (do not reconsider the count):\n" +
		"Decompose these un-staged changes into commits:\n\n" + diff

	got := BuildPlannerUserPayload(diff, "", 3)
	if got != want {
		t.Errorf("BuildPlannerUserPayload forced mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestBuildPlannerUserPayload_Properties is a table of structural invariants guarding the load-bearing
// decisions: the normal vs forced path, the diff-always-tail rule, and the interpolation correctness.
func TestBuildPlannerUserPayload_Properties(t *testing.T) {
	const diff = "DIFFCONTENT"
	cases := []struct {
		name  string
		diff  string
		fc    int
		check func(t *testing.T, p string)
	}{
		{
			name: "normal: no Produce EXACTLY", diff: diff, fc: 0,
			check: func(t *testing.T, p string) {
				if strings.Contains(p, "Produce EXACTLY") {
					t.Error("normal payload must NOT contain forced-count directive")
				}
			},
		},
		{
			name: "forced: Produce EXACTLY N present with N interpolated", diff: diff, fc: 3,
			check: func(t *testing.T, p string) {
				if !strings.HasPrefix(p, "Produce EXACTLY 3 commits from these changes (do not reconsider the count):\n") {
					t.Errorf("forced payload must start with forced directive; got %q", near(p, "Produce"))
				}
			},
		},
		{
			name: "forced: N interpolated (5)", diff: diff, fc: 5,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, "Produce EXACTLY 5 commits") {
					t.Error("forcedCount=5 not interpolated")
				}
				if strings.Contains(p, "Produce EXACTLY 3 commits") {
					t.Error("leaked a hardcoded 3")
				}
			},
		},
		{
			name: "diff is the verbatim tail (normal)", diff: "TAIL_NORMAL\nnope", fc: 0,
			check: func(t *testing.T, p string) {
				if !strings.HasSuffix(p, "TAIL_NORMAL\nnope") {
					t.Errorf("diff must be the verbatim tail; got suffix %q", suffix(p, 40))
				}
			},
		},
		{
			name: "diff is the verbatim tail (forced)", diff: "TAIL_FORCED", fc: 2,
			check: func(t *testing.T, p string) {
				if !strings.HasSuffix(p, "TAIL_FORCED") {
					t.Errorf("diff must be the verbatim tail; got suffix %q", suffix(p, 40))
				}
			},
		},
		{
			name: "negative forcedCount == normal", diff: diff, fc: -1,
			check: func(t *testing.T, p string) {
				if strings.Contains(p, "Produce EXACTLY") {
					t.Error("negative forcedCount must be treated as normal")
				}
				want := plannerUserInstruction + "\n\n" + diff
				if p != want {
					t.Errorf("negative forcedCount payload = %q, want %q", p, want)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, BuildPlannerUserPayload(tc.diff, "", tc.fc))
		})
	}
}

// TestPlannerRetryInstruction asserts the exported retry instruction constant is byte-faithful to §17.5.
func TestPlannerRetryInstruction(t *testing.T) {
	const want = "Respond with ONLY the JSON object described, no other text."
	if PlannerRetryInstruction != want {
		t.Errorf("PlannerRetryInstruction = %q, want %q", PlannerRetryInstruction, want)
	}
}

// TestParsePlannerOutput is a table of parse scenarios covering clean JSON, prose-wrapped, code-fenced,
// edge cases, and error cases.
func TestParsePlannerOutput(t *testing.T) {
	cleanMulti := `{"count":2,"single":false,"commits":[{"title":"A","description":"d1","files":["a.go","b.go"]},{"title":"B","description":"d2"}]}`
	singleMsg := `{"count":1,"single":true,"commits":[{"title":"X","description":"d"}],"message":"feat: add thing"}`
	proseWrapped := "Here is the plan:\n" + cleanMulti + "\nThanks!"
	codeFenced := "```json\n" + cleanMulti + "\n```"
	whitespace := "  \n" + cleanMulti + "\n  "
	nullCommits := `{"count":0,"single":false,"commits":null}`
	extraFields := `{"count":1,"single":true,"commits":[{"title":"T","description":"D"}],"message":"M","extra":"ignored"}`
	nullFiles := `{"count":1,"single":false,"commits":[{"title":"N","description":"d","files":null}]}`

	zero := PlannerOutput{Count: -999} // sentinel; non-nil Commits also compared below
	cases := []struct {
		name    string
		raw     string
		wantOut PlannerOutput
		wantErr bool
	}{
		{"clean multi-commit JSON", cleanMulti,
			PlannerOutput{Count: 2, Single: false, Commits: []PlannerCommit{{Title: "A", Description: "d1", Files: []string{"a.go", "b.go"}}, {Title: "B", Description: "d2"}}}, false},
		{"single-commit with message", singleMsg,
			PlannerOutput{Count: 1, Single: true, Commits: []PlannerCommit{{Title: "X", Description: "d"}}, Message: "feat: add thing"}, false},
		{"JSON in prose (brace-balanced fallback)", proseWrapped,
			PlannerOutput{Count: 2, Single: false, Commits: []PlannerCommit{{Title: "A", Description: "d1", Files: []string{"a.go", "b.go"}}, {Title: "B", Description: "d2"}}}, false},
		{"JSON in code fence", codeFenced,
			PlannerOutput{Count: 2, Single: false, Commits: []PlannerCommit{{Title: "A", Description: "d1", Files: []string{"a.go", "b.go"}}, {Title: "B", Description: "d2"}}}, false},
		{"leading/trailing whitespace trimmed", whitespace,
			PlannerOutput{Count: 2, Single: false, Commits: []PlannerCommit{{Title: "A", Description: "d1", Files: []string{"a.go", "b.go"}}, {Title: "B", Description: "d2"}}}, false},
		{"commits:null → nil slice, no panic", nullCommits,
			PlannerOutput{Count: 0, Single: false, Commits: nil}, false},
		{"extra unknown fields ignored", extraFields,
			PlannerOutput{Count: 1, Single: true, Commits: []PlannerCommit{{Title: "T", Description: "D"}}, Message: "M"}, false},
		{"files:null → nil slice, no panic", nullFiles,
			PlannerOutput{Count: 1, Single: false, Commits: []PlannerCommit{{Title: "N", Description: "d"}}}, false},
		{"malformed → error", "not json at all", zero, true},
		{"empty → error", "", zero, true},
		{"unbalanced braces → error", `{"count":1`, zero, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ParsePlannerOutput(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Error("expected non-nil error, got nil")
				}
				if out.Count != 0 || out.Single || out.Message != "" || out.Commits != nil {
					t.Errorf("on error, expected zero PlannerOutput; got %+v", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Count != tc.wantOut.Count {
				t.Errorf("Count = %d, want %d", out.Count, tc.wantOut.Count)
			}
			if out.Single != tc.wantOut.Single {
				t.Errorf("Single = %v, want %v", out.Single, tc.wantOut.Single)
			}
			if out.Message != tc.wantOut.Message {
				t.Errorf("Message = %q, want %q", out.Message, tc.wantOut.Message)
			}
			if len(out.Commits) != len(tc.wantOut.Commits) {
				t.Fatalf("len(Commits) = %d, want %d", len(out.Commits), len(tc.wantOut.Commits))
			}
			if tc.wantOut.Commits == nil {
				if out.Commits != nil {
					t.Errorf("Commits = %v, want nil", out.Commits)
				}
			} else {
				for i, c := range out.Commits {
					if c.Title != tc.wantOut.Commits[i].Title {
						t.Errorf("Commits[%d].Title = %q, want %q", i, c.Title, tc.wantOut.Commits[i].Title)
					}
					if c.Description != tc.wantOut.Commits[i].Description {
						t.Errorf("Commits[%d].Description = %q, want %q", i, c.Description, tc.wantOut.Commits[i].Description)
					}
					if !reflect.DeepEqual(c.Files, tc.wantOut.Commits[i].Files) {
						t.Errorf("Commits[%d].Files = %v, want %v", i, c.Files, tc.wantOut.Commits[i].Files)
					}
				}
			}
		})
	}
}

// TestParsePlannerOutput_MissingMessage verifies that when "message" is absent and single==false,
// Message is the zero value "" (no error).
func TestParsePlannerOutput_MissingMessage(t *testing.T) {
	out, err := ParsePlannerOutput(`{"count":2,"single":false,"commits":[]}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Message != "" {
		t.Errorf("missing message ⇒ zero value; got %q", out.Message)
	}
}

// TestExtractJSONObject verifies the private brace-balanced JSON extractor covers the expected cases.
func TestExtractJSONObject(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		wantFound  bool
		wantSubstr string
	}{
		{"clean JSON object", `{"a":1}`, true, `{"a":1}`},
		{"prose-wrapped", `text {"a":1} more`, true, `{"a":1}`},
		{"code-fenced", "```json\n{\"a\":1}\n```", true, `{"a":1}`},
		{"no brace at all", "plain text", false, ""},
		{"unbalanced opening", `{"a":1`, false, ""},
		{"braces in string ignored", `{"a":"{b}"}`, true, `{"a":"{b}"}`},
		{"nested objects", `{"a":{"b":2}}`, true, `{"a":{"b":2}}`},
		{"empty object", `{}`, true, `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub, found := extractJSONObject(tc.input)
			if found != tc.wantFound {
				t.Errorf("found = %v, want %v", found, tc.wantFound)
			}
			if found && sub != tc.wantSubstr {
				t.Errorf("substr = %q, want %q", sub, tc.wantSubstr)
			}
		})
	}
}

// TestParsePlannerOutput_RoundTrip verifies that PlannerOutput can be marshaled and parsed back.
func TestParsePlannerOutput_RoundTrip(t *testing.T) {
	original := PlannerOutput{
		Count:   3,
		Single:  false,
		Commits: []PlannerCommit{{Title: "A", Description: "dA", Files: []string{"x", "y"}}, {Title: "B", Description: "dB"}, {Title: "C", Description: "dC"}},
		Message: "",
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out, err := ParsePlannerOutput(string(data))
	if err != nil {
		t.Fatalf("ParsePlannerOutput: %v", err)
	}
	if out.Count != original.Count || out.Single != original.Single || out.Message != original.Message {
		t.Errorf("round-trip mismatch: got %+v, want %+v", out, original)
	}
	if len(out.Commits) != len(original.Commits) {
		t.Fatalf("Commits length = %d, want %d", len(out.Commits), len(original.Commits))
	}
	for i := range out.Commits {
		if !reflect.DeepEqual(out.Commits[i], original.Commits[i]) {
			t.Errorf("Commits[%d] = %+v, want %+v", i, out.Commits[i], original.Commits[i])
		}
	}
}
