package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestVerboseRoles_On(t *testing.T) {
	var buf bytes.Buffer
	v := NewVerbose(&buf, true)
	v.VerboseRoles([]RoleLine{
		{Name: "planner", Model: "p", Provider: "pi", Reasoning: "high"},
		{Name: "stager", Model: "s", Provider: "pi", Reasoning: ""},
		{Name: "message", Model: "m", Provider: "pi", Reasoning: ""},
		{Name: "arbiter", Model: "a", Provider: "pi", Reasoning: ""},
	})
	got := buf.String()
	// Planner line: %-8s pads "planner" (7 chars) → "planner " + space = two spaces before invocation.
	if !strings.Contains(got, "DEBUG: planner  p in pi (reasoning: high)\n") {
		t.Errorf("planner line missing or wrong; got %q", got)
	}
	// Stager line: no reasoning suffix.
	if !strings.Contains(got, "DEBUG: stager   s in pi\n") {
		t.Errorf("stager line missing or wrong; got %q", got)
	}
	if !strings.Contains(got, "DEBUG: message  m in pi\n") {
		t.Errorf("message line missing or wrong; got %q", got)
	}
	if !strings.Contains(got, "DEBUG: arbiter  a in pi\n") {
		t.Errorf("arbiter line missing or wrong; got %q", got)
	}
	// Must have exactly 4 lines.
	if count := strings.Count(got, "\n"); count != 4 {
		t.Errorf("got %d lines, want 4; got %q", count, got)
	}
}

func TestVerboseRoles_Off(t *testing.T) {
	var buf bytes.Buffer
	v := NewVerbose(&buf, false)
	v.VerboseRoles([]RoleLine{
		{Name: "planner", Model: "p", Provider: "pi", Reasoning: "high"},
	})
	if buf.Len() != 0 {
		t.Errorf("off: wrote %q, want zero bytes", buf.String())
	}
}

func TestVerboseRoles_NilSafe(t *testing.T) {
	var v *Verbose = nil
	v.VerboseRoles([]RoleLine{
		{Name: "planner", Model: "p", Provider: "pi", Reasoning: "high"},
	}) // must not panic
}

func TestReasoningSuffix(t *testing.T) {
	tests := []struct {
		level string
		want  string
	}{
		{"", ""},
		{"off", ""},
		{"low", " (reasoning: low)"},
		{"medium", " (reasoning: medium)"},
		{"high", " (reasoning: high)"},
	}
	for _, tc := range tests {
		t.Run(tc.level, func(t *testing.T) {
			got := reasoningSuffix(tc.level)
			if got != tc.want {
				t.Errorf("reasoningSuffix(%q) = %q, want %q", tc.level, got, tc.want)
			}
		})
	}
}

func TestVerbose_CommandWhenOn(t *testing.T) {
	var buf bytes.Buffer
	v := NewVerbose(&buf, true)
	v.VerboseCommand("pi --model x")
	want := "DEBUG: command: pi --model x\n"
	if buf.String() != want {
		t.Errorf("VerboseCommand: got %q, want %q", buf.String(), want)
	}
}

func TestVerbose_RawOutputWhenOn(t *testing.T) {
	var buf bytes.Buffer
	v := NewVerbose(&buf, true)

	// With trailing newline in output.
	v.VerboseRawOutput("feat: x\n")
	want := "DEBUG: raw output:\nfeat: x\n"
	if buf.String() != want {
		t.Errorf("VerboseRawOutput (trailing NL): got %q, want %q", buf.String(), want)
	}
	buf.Reset()

	// Without trailing newline — should add one.
	v.VerboseRawOutput("feat: x")
	want = "DEBUG: raw output:\nfeat: x\n"
	if buf.String() != want {
		t.Errorf("VerboseRawOutput (no trailing NL): got %q, want %q", buf.String(), want)
	}
}

func TestVerbose_RetryWhenOn(t *testing.T) {
	var buf bytes.Buffer
	v := NewVerbose(&buf, true)
	v.VerboseRetry(1, `subject "x" matches an existing commit`)
	want := `DEBUG: attempt 1: subject "x" matches an existing commit` + "\n"
	if buf.String() != want {
		t.Errorf("VerboseRetry: got %q, want %q", buf.String(), want)
	}
}

func TestVerbose_NoOpWhenOff(t *testing.T) {
	var buf bytes.Buffer
	v := NewVerbose(&buf, false)
	v.VerboseCommand("pi --model x")
	v.VerboseRawOutput("feat: x\n")
	v.VerboseRetry(1, "reason")
	if buf.Len() != 0 {
		t.Errorf("off: wrote %q, want zero bytes", buf.String())
	}
}

func TestVerbose_NilSafeReceiver(t *testing.T) {
	var v *Verbose = nil
	v.VerboseCommand("x")   // must not panic
	v.VerboseRawOutput("y") // must not panic
	v.VerboseRetry(1, "z")  // must not panic
}

func TestVerbose_NilWriterNoOp(t *testing.T) {
	v := NewVerbose(nil, true)
	v.VerboseCommand("x")   // must not panic
	v.VerboseRawOutput("y") // must not panic
	v.VerboseRetry(1, "z")  // must not panic
}

func TestVerbose_MultipleLinesAccumulate(t *testing.T) {
	var buf bytes.Buffer
	v := NewVerbose(&buf, true)
	v.VerboseCommand("pi --model x")
	v.VerboseRawOutput("feat: hello\n")
	v.VerboseRetry(1, `subject "feat: existing" matches an existing commit`)

	s := buf.String()

	// All three substrings present in order.
	if !strings.Contains(s, "DEBUG: command: pi --model x\n") {
		t.Errorf("missing command line; got %q", s)
	}
	if !strings.Contains(s, "DEBUG: raw output:\nfeat: hello\n") {
		t.Errorf("missing raw output line; got %q", s)
	}
	if !strings.Contains(s, `DEBUG: attempt 1: subject "feat: existing" matches an existing commit`+"\n") {
		t.Errorf("missing retry line; got %q", s)
	}

	// Command comes before raw output.
	cmdIdx := strings.Index(s, "DEBUG: command:")
	rawIdx := strings.Index(s, "DEBUG: raw output:")
	if cmdIdx >= rawIdx {
		t.Errorf("command (%d) should come before raw output (%d)", cmdIdx, rawIdx)
	}

	// Raw output comes before retry.
	retryIdx := strings.Index(s, "DEBUG: attempt")
	if rawIdx >= retryIdx {
		t.Errorf("raw output (%d) should come before retry (%d)", rawIdx, retryIdx)
	}
}
