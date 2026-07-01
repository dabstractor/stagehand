package prompt

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBuildArbiterSystemPrompt_CanonicalExact asserts the FULL system prompt string is byte-for-byte
// faithful to PRD §17.7. Independently derived from §17.7 (NOT from the implementation) so a match is
// meaningful. The want is the verbatim §17.7 fenced block: role, decision rules (with em-dash), and
// the JSON-contract line with the literal `<sha from the list>` token.
func TestBuildArbiterSystemPrompt_CanonicalExact(t *testing.T) {
	got := BuildArbiterSystemPrompt()

	const want = "You reconcile leftover changes into commits that were just made. You are given the commits\n" +
		"created this run (with their messages and changed files) and a diff of changes that were not\n" +
		"included in any of them.\n" +
		"\n" +
		"Decide: do these leftovers logically belong WITH one of those commits, or do they warrant a\n" +
		"NEW commit?\n" +
		"- Choose an existing commit only if the leftovers are part of the SAME logical change.\n" +
		"- When in doubt, prefer a NEW commit (return null) — never force a fit.\n" +
		"- You may only target a commit from the provided list.\n" +
		"\n" +
		`Respond with ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.`

	if got != want {
		t.Errorf("BuildArbiterSystemPrompt mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestBuildArbiterSystemPrompt_Properties is a table of structural invariants on the arbiter system
// prompt, including anti-copy-paste guards that pin §17.1/§17.5/§17.6 elements are ABSENT and §17.7
// elements are PRESENT.
func TestBuildArbiterSystemPrompt_Properties(t *testing.T) {
	p := BuildArbiterSystemPrompt()

	cases := []struct {
		name      string
		needle    string
		mustExist bool
	}{
		// §17.7 elements PRESENT.
		{"role is reconcile arbiter PRESENT", "You reconcile leftover changes into commits", true},
		{"JSON contract line PRESENT verbatim", `Respond with ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.`, true},
		{"'prefer a NEW commit (return null)' PRESENT", "prefer a NEW commit (return null)", true},
		{"'You may only target a commit from the provided list' PRESENT", "You may only target a commit from the provided list.", true},

		// Em-dash PRESENT, NOT ASCII hyphen.
		{"em-dash PRESENT (NOT ascii hyphen)", "return null) — never force a fit", true},

		// §17.1 mature elements ABSENT (anti-copy-paste guards).
		{"§17.1 'commit message generator' ABSENT", "You are a commit message generator", false},
		{"§17.1 anti-reuse block ABSENT", "CRITICAL: You MUST NOT copy", false},
		{"§17.1 subject-target line ABSENT", "Target ~", false},

		// §17.5 planner elements ABSENT.
		{"§17.5 'commit-planning assistant' ABSENT", "You are a commit-planning assistant", false},
		{"§17.5 JSON contract ABSENT", `{"count": <int>`, false},
		{"§17.5 planner user-instruction ABSENT", "Decompose these un-staged changes", false},

		// §17.6 stager elements ABSENT.
		{"§17.6 stager instruction ABSENT", "Stage, but do NOT commit", false},
		{"§17.6 stager guardrails ABSENT", "git apply --cached", false},

		// §17.7 has no style-examples placeholder.
		{"no <style examples> token", "<style examples>", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			has := strings.Contains(p, tc.needle)
			if tc.mustExist && !has {
				t.Errorf("expected %q in arbiter prompt; not found", tc.needle)
			}
			if !tc.mustExist && has {
				t.Errorf("arbiter prompt must NOT contain %q (copy-paste leak)", tc.needle)
			}
		})
	}

	// Extra em-dash guard: the ASCII hyphen variant must NOT be present.
	if strings.Contains(p, "return null) - never force a fit") {
		t.Error("arbiter prompt uses ASCII hyphen '-' instead of em-dash '—'")
	}

	// No trailing newline.
	if strings.HasSuffix(p, "\n") {
		t.Error("arbiter system prompt must NOT end with a trailing newline")
	}
}

// TestBuildArbiterUserPayload_CanonicalExact asserts the FULL assembled user payload for two commits
// with files is byte-for-byte the §2 assembly. Independently derived from the design decision (NOT
// from the implementation).
func TestBuildArbiterUserPayload_CanonicalExact(t *testing.T) {
	commits := []ArbiterCommit{
		{SHA: "a1b2", Subject: "feat: add login", Files: []string{"a/login.go", "a/login_test.go"}},
		{SHA: "c3d4", Subject: "fix: nil deref", Files: []string{"a/x.go"}},
	}
	leftoverDiff := "diff --git a/left.go b/left.go\n@@ ...s..."

	want := arbiterCommitsHeader + "\n" +
		"\n" +
		"a1b2\n" +
		"feat: add login\n" +
		"a/login.go\n" +
		"a/login_test.go\n" +
		"\n" +
		"c3d4\n" +
		"fix: nil deref\n" +
		"a/x.go\n" +
		"\n" +
		arbiterLeftoverHeader + "\n" +
		"\n" +
		leftoverDiff

	got := BuildArbiterUserPayload(commits, leftoverDiff)
	if got != want {
		t.Errorf("BuildArbiterUserPayload mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestBuildArbiterUserPayload_Properties is a table of structural invariants guarding the user payload
// assembly: headers, SHA/subject/file ordering, blank-line topology, and the verbatim-diff tail.
func TestBuildArbiterUserPayload_Properties(t *testing.T) {
	commits := []ArbiterCommit{
		{SHA: "AAA", Subject: "feat: add login", Files: []string{"a/login.go", "a/login_test.go"}},
		{SHA: "BBB", Subject: "fix: nil deref", Files: []string{"a/x.go"}},
	}
	leftoverDiff := "TAILDIFF"

	got := BuildArbiterUserPayload(commits, leftoverDiff)

	cases := []struct {
		name  string
		check func(t *testing.T, p string)
	}{
		{"commits header present and starts output", func(t *testing.T, p string) {
			if !strings.HasPrefix(p, arbiterCommitsHeader) {
				t.Errorf("payload must start with commits header; got %q", near(p, arbiterCommitsHeader))
			}
		}},
		{"leftover header present", func(t *testing.T, p string) {
			if !strings.Contains(p, arbiterLeftoverHeader) {
				t.Error("payload missing leftover header")
			}
		}},
		{"SHAs present, in order", func(t *testing.T, p string) {
			i := strings.Index(p, "AAA")
			j := strings.Index(p, "BBB")
			if i < 0 || j < 0 || i >= j {
				t.Errorf("SHAs out of order: AAA@%d BBB@%d", i, j)
			}
		}},
		{"subjects present", func(t *testing.T, p string) {
			if !strings.Contains(p, "feat: add login") || !strings.Contains(p, "fix: nil deref") {
				t.Error("payload missing subject lines")
			}
		}},
		{"files present, one per line", func(t *testing.T, p string) {
			if !strings.Contains(p, "a/login.go\na/login_test.go") {
				t.Error("files not rendered one per line")
			}
		}},
		{"leftover diff is the verbatim tail", func(t *testing.T, p string) {
			if !strings.HasSuffix(p, leftoverDiff) {
				t.Errorf("diff must be the verbatim tail; got suffix %q", suffix(p, 40))
			}
		}},
		{"one blank line between commit blocks", func(t *testing.T, p string) {
			if !strings.Contains(p, "a/login_test.go\n\nc3d4") {
				// Use actual data from canonical test — with AAA/BBB instead:
				if !strings.Contains(p, "a/login_test.go\n\nBBB") {
					t.Error("expected one blank line between commit blocks")
				}
			}
		}},
		{"one blank line after commits header", func(t *testing.T, p string) {
			if !strings.HasPrefix(p, arbiterCommitsHeader+"\n\n") {
				t.Errorf("expected blank line after commits header; got prefix %q", near(p, arbiterCommitsHeader))
			}
		}},
		{"one blank line after leftover header", func(t *testing.T, p string) {
			if !strings.Contains(p, arbiterLeftoverHeader+"\n\n") {
				t.Errorf("expected blank line after leftover header; got %q", near(p, arbiterLeftoverHeader))
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, got)
		})
	}
}

// TestBuildArbiterUserPayload_EdgeCases verifies defensive behavior on empty/nil inputs.
func TestBuildArbiterUserPayload_EdgeCases(t *testing.T) {
	t.Run("nil commits does not panic", func(t *testing.T) {
		got := BuildArbiterUserPayload(nil, "DIFF")
		if !strings.Contains(got, arbiterLeftoverHeader) {
			t.Error("nil commits: leftover header missing")
		}
	})

	t.Run("empty commits slice", func(t *testing.T) {
		got := BuildArbiterUserPayload([]ArbiterCommit{}, "DIFF")
		if !strings.Contains(got, arbiterCommitsHeader) {
			t.Error("empty commits: commits header missing")
		}
	})

	t.Run("commit with empty Files", func(t *testing.T) {
		got := BuildArbiterUserPayload(
			[]ArbiterCommit{{SHA: "x", Subject: "s", Files: nil}},
			"DIFF",
		)
		if !strings.Contains(got, "x\ns\n\n") {
			t.Errorf("empty Files: expected 'x\\ns\\n\\n'; got %q", near(got, "x\ns"))
		}
	})

	t.Run("empty leftoverDiff", func(t *testing.T) {
		got := BuildArbiterUserPayload(
			[]ArbiterCommit{{SHA: "a", Subject: "s", Files: []string{"f.go"}}},
			"",
		)
		if !strings.Contains(got, arbiterLeftoverHeader) {
			t.Error("empty leftoverDiff: leftover header missing")
		}
		if !strings.HasSuffix(got, arbiterLeftoverHeader+"\n\n") {
			t.Errorf("empty leftoverDiff: expected suffix to be header+blank; got suffix %q", suffix(got, 60))
		}
	})
}

// TestParseArbiterOutput is a table of parse scenarios covering clean JSON, prose-wrapped,
// code-fenced, edge cases, and error cases for the *string Target field.
func TestParseArbiterOutput(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantNil bool   // true ⇒ Target must be nil; false ⇒ Target must be non-nil
		wantSHA string // only used when wantNil==false
		wantErr bool
	}{
		{"null target → nil", `{"target": null}`, true, "", false},
		{"sha target → non-nil", `{"target": "a1b2c3d4"}`, false, "a1b2c3d4", false},
		{"literal placeholder sha", `{"target": "<sha from the list>"}`, false, "<sha from the list>", false},
		{"JSON in prose (brace-balanced fallback)", "The answer is {\"target\":\"a1b2\"} — done", false, "a1b2", false},
		{"JSON in code fence", "```json\n{\"target\":\"a1b2\"}\n```", false, "a1b2", false},
		{"leading/trailing whitespace trimmed", "  \n{\"target\":null}\n  ", true, "", false},
		{"field absent → nil (new-commit default)", `{}`, true, "", false},
		{"extra unknown fields ignored", `{"target":"a1b2","extra":"ignored","note":"x"}`, false, "a1b2", false},
		{"empty string target → non-nil empty (caller rejects)", `{"target": ""}`, false, "", false},
		{"non-string target (number) → error", `{"target": 123}`, false, "", true},
		{"non-string target (bool) → error", `{"target": true}`, false, "", true},
		{"malformed → error", "not json at all", false, "", true},
		{"empty → error", "", false, "", true},
		{"unbalanced braces → error", `{"target":"a1b2"`, false, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ParseArbiterOutput(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Error("expected non-nil error, got nil")
				}
				if out.Target != nil {
					t.Errorf("on error, expected nil Target; got %q", *out.Target)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantNil {
				if out.Target != nil {
					t.Errorf("expected nil Target; got %q", *out.Target)
				}
			} else {
				if out.Target == nil {
					t.Fatal("expected non-nil Target, got nil")
				}
				if *out.Target != tc.wantSHA {
					t.Errorf("Target = %q, want %q", *out.Target, tc.wantSHA)
				}
			}
		})
	}
}

// TestParseArbiterOutput_RoundTrip verifies that ArbiterOutput can be marshaled and parsed back,
// confirming the *string Target survives marshal/parse.
func TestParseArbiterOutput_RoundTrip(t *testing.T) {
	t.Run("nil Target round-trip", func(t *testing.T) {
		original := ArbiterOutput{Target: nil}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		out, err := ParseArbiterOutput(string(data))
		if err != nil {
			t.Fatalf("ParseArbiterOutput: %v", err)
		}
		if out.Target != nil {
			t.Errorf("round-trip nil Target: got %q, want nil", *out.Target)
		}
	})

	t.Run("non-nil Target round-trip", func(t *testing.T) {
		sha := "a1b2c3d4"
		original := ArbiterOutput{Target: &sha}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		out, err := ParseArbiterOutput(string(data))
		if err != nil {
			t.Fatalf("ParseArbiterOutput: %v", err)
		}
		if out.Target == nil {
			t.Fatal("round-trip non-nil Target: got nil")
		}
		if *out.Target != sha {
			t.Errorf("round-trip Target = %q, want %q", *out.Target, sha)
		}
	})
}

// TestArbiterOutput_NullSemantics is the dedicated *string tri-state guard: nil ⇔ null ⇔ new commit,
// and &"" != nil (a Go sanity check pinning the design — empty-string-target is non-nil so the
// caller can distinguish it from null).
func TestArbiterOutput_NullSemantics(t *testing.T) {
	// {"target": null} → Target == nil
	outNull, err := ParseArbiterOutput(`{"target": null}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outNull.Target != nil {
		t.Errorf("null JSON → expected nil Target; got %q", *outNull.Target)
	}

	// {"target": "a1b2"} → Target != nil, *Target == "a1b2"
	outSHA, err := ParseArbiterOutput(`{"target": "a1b2"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outSHA.Target == nil {
		t.Fatal("sha JSON → expected non-nil Target, got nil")
	}
	if *outSHA.Target != "a1b2" {
		t.Errorf("Target = %q, want %q", *outSHA.Target, "a1b2")
	}

	// nil Target and &"" are DISTINCT: a Go sanity check.
	var emptyStr = ""
	emptyPtr := &emptyStr
	if outNull.Target == emptyPtr {
		t.Error("nil Target and &\"\" should be distinct pointers")
	}
}
