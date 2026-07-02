package generate

// White-box test for the rescue render function Rescue (P1.M6.T1.S3),
// matching the internal/generate/dedupe_test.go and internal/ui/output_test.go
// house conventions: the _test.go file is `package generate` (NOT
// `package generate_test`) so it sits in the same package as the (exported but
// pure-render) function under test. It exercises Rescue over an injected
// *ui.Output wired to captured buffers — NO git binary, NO exec, NO filesystem
// — so it needs stdlib `bytes`/`io`/`strings`/`testing` + internal/ui ONLY (NO
// testify, NO internal/git). Because ui.Output's fields are unexported, the
// Output is constructed via the exported ui.NewOutput(io.Discard, &stderr,
// false, true) (stdout=io.Discard so a routing leak is observable; verbose
// off; noColor=true so out.Red is a no-op and the captured text is plain). It
// uses table-driven `t.Run(tc.name, ...)` subtests with `tc := tc` capture and
// asserts substring presence/absence via strings.Contains, exactly as
// output_test.go does for the FR51 stream discipline.

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/dustin/stagehand/internal/ui"
)

// newRescueOutput builds a *ui.Output whose stderr is captured into stderr and
// whose stdout is io.Discard (so a routing leak onto stdout is silently
// swallowed and asserted against via a dedicated buffer in the FR51 test).
// noColor=true forces out.Red to be a no-op, keeping the captured text plain
// so substring assertions hold regardless of the ❌ wrapper.
func newRescueOutput(stderr *bytes.Buffer) *ui.Output {
	return ui.NewOutput(io.Discard, stderr, false, true)
}

// containsAll reports whether got contains every one of the want substrings,
// reporting the first missing one via the returned string (empty when all are
// present). It keeps the t.Errorf calls in the tests one-liner-clean.
func containsAll(got string, want ...string) string {
	for _, w := range want {
		if !strings.Contains(got, w) {
			return w
		}
	}
	return ""
}

// TestRescue_BlockContainsTreeAndCommandAndNote pins the non-root, empty-
// candidate contract (FR44): the frozen TREE_SHA is echoed, the manual
// recovery command is present with the -p <parent> adjacency, and the static
// omit-note appears — and the candidate enrichment line is ABSENT (no rejected
// message). This covers MOCKING bullets 1 (tree SHA), 2 (manual command
// tokens), 3 (omit-note), 4-absent (candidate line absent), and the non-root
// side of 5 (-p present).
func TestRescue_BlockContainsTreeAndCommandAndNote(t *testing.T) {
	var stderr bytes.Buffer
	o := newRescueOutput(&stderr)
	Rescue(o, "9f3a1c0deadbeef", "abc1234", "")
	got := stderr.String()

	if missing := containsAll(got,
		"9f3a1c0deadbeef",                                 // the frozen TREE_SHA (FR44)
		"git commit-tree",                                 // manual command token
		"xargs git update-ref HEAD",                       // manual command token
		"commit-tree -p abc1234",                          // -p <parent> adjacency (non-root)
		"first commit",                                    // static omit-note
		"Commit generation failed",                        // the failure notice (FR44)
		"safely snapshotted before generation",            // the snapshot-safety line
		"To commit the originally staged files manually:", // command intro
	); missing != "" {
		t.Errorf("rescue output missing %q\n--got--\n%s", missing, got)
	}

	// No rejected candidate message was produced, so the enrichment line is
	// absent.
	if strings.Contains(got, "candidate message") {
		t.Errorf("rescue output must NOT contain a candidate line for candidate=\"\"\n--got--\n%s", got)
	}
}

// TestRescue_RootOmitsParentFlag pins the root-commit enrichment: when
// parent=="" (an unborn repo — git.RevParseHEAD returns hasParent=false), the
// manual command omits `-p` entirely. The check asserts the COMMAND adjacency
// `commit-tree -p` is absent (the static omit-note literally contains `-p`
// but never adjacent to `commit-tree`, so this isolates the command line) and
// that the root command shape `commit-tree -m "Your message"` is present. The
// tree SHA, manual-command tokens, and omit-note must still be present.
func TestRescue_RootOmitsParentFlag(t *testing.T) {
	var stderr bytes.Buffer
	o := newRescueOutput(&stderr)
	Rescue(o, "9f3a1c0deadbeef", "", "")
	got := stderr.String()

	// Root command omits `-p`: the `commit-tree -p` adjacency must be absent.
	// (The static note's literal `-p` is never adjacent to `commit-tree`, so
	// this isolates the command from the note.)
	if strings.Contains(got, "commit-tree -p") {
		t.Errorf("root rescue command must omit -p, but output contains the `commit-tree -p` adjacency\n--got--\n%s", got)
	}
	// The root command shape is `commit-tree -m "Your message" <tree>`.
	if !strings.Contains(got, "commit-tree -m \"Your message\"") {
		t.Errorf("root rescue output missing the `commit-tree -m \"Your message\"` command shape\n--got--\n%s", got)
	}

	if missing := containsAll(got,
		"9f3a1c0deadbeef",           // tree SHA still present
		"git commit-tree",           // manual command token
		"xargs git update-ref HEAD", // manual command token
		"first commit",              // static omit-note still present
	); missing != "" {
		t.Errorf("root rescue output missing %q\n--got--\n%s", missing, got)
	}
}

// TestRescue_CandidateLinePresentIffNonEmpty pins the PRD §18.3 candidate-
// message enrichment: the `A candidate message was produced but rejected:
// "<msg>". You can use it manually in the command above.` line appears ONLY
// when candidate != "". The table covers the two branches (absent vs present)
// over a fixed non-root setup so the candidate line is the only variable.
func TestRescue_CandidateLinePresentIffNonEmpty(t *testing.T) {
	tests := []struct {
		name        string
		candidate   string
		wantPresent bool
	}{
		{
			name:        "no candidate -> enrichment line absent",
			candidate:   "",
			wantPresent: false,
		},
		{
			name:        "rejected candidate -> enrichment line present",
			candidate:   "feat: add rescue protocol",
			wantPresent: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			o := newRescueOutput(&stderr)
			Rescue(o, "9f3a1c0deadbeef", "abc1234", tc.candidate)
			got := stderr.String()

			if tc.wantPresent {
				// The full enrichment line + the candidate text echoed + the
				// trailing "use it manually" wording must all be present.
				if missing := containsAll(got,
					"A candidate message was produced but rejected:",
					tc.candidate,
					"You can use it manually in the command above.",
				); missing != "" {
					t.Errorf("rescue output missing %q for candidate=%q\n--got--\n%s", missing, tc.candidate, got)
				}
				// The candidate is interpolated with explicit "%s" double
				// quotes (NOT %q), so it appears wrapped in literal quotes.
				if !strings.Contains(got, "\""+tc.candidate+"\"") {
					t.Errorf("rescue output missing the candidate in literal double quotes for candidate=%q\n--got--\n%s", tc.candidate, got)
				}
			} else {
				// No enrichment line.
				if strings.Contains(got, "A candidate message was produced") {
					t.Errorf("rescue output must NOT contain the candidate line for candidate=\"\"\n--got--\n%s", got)
				}
				// (For candidate="" the candidate text is empty, so a
				// strings.Contains(got, tc.candidate) check would be vacuously
				// true and is therefore skipped — the absence of the line is
				// the meaningful assertion.)
			}
		})
	}
}

// TestRescue_FiresOnStderrStdoutStaysClean pins the FR51 stream discipline: a
// failure/recovery notice is NOT a commit result, so the rescue block lands on
// STDERR (out.Progressf) and stdout stays byte-clean — otherwise
// `stagehand --dry-run | tee /tmp/msg.txt` pipelines would be corrupted. With
// stdout wired to a SEPARATE buffer, stderr must contain the failure notice and
// stdout must NOT.
func TestRescue_FiresOnStderrStdoutStaysClean(t *testing.T) {
	var stdout, stderr bytes.Buffer
	o := ui.NewOutput(&stdout, &stderr, false, true) // stdout to a real buffer so a leak is observable
	Rescue(o, "9f3a1c0deadbeef", "abc1234", "feat: x")

	if !strings.Contains(stderr.String(), "Commit generation failed") {
		t.Errorf("stderr = %q, missing the failure notice `Commit generation failed`", stderr.String())
	}
	if strings.Contains(stdout.String(), "Commit generation failed") {
		t.Errorf("stdout = %q, must NOT contain the failure notice (FR51 stdout must stay clean)", stdout.String())
	}
	// The whole block — manual command, tree ID, candidate line — must stay
	// off stdout too, not just the notice.
	for _, leak := range []string{
		"git commit-tree",
		"Tree ID: 9f3a1c0deadbeef",
		"A candidate message was produced",
	} {
		if strings.Contains(stdout.String(), leak) {
			t.Errorf("stdout = %q, must NOT contain %q (FR51 stdout must stay clean)", stdout.String(), leak)
		}
	}
}

// TestRescue_ExactCommandSubstrings asserts the TIGHTEST possible check on the
// dynamic manual command for both the non-root and root branches (FR44 exact
// command): non-root renders the full `-p <parent> -m "Your message" <tree>`
// adjacency, and root renders `-m "Your message" <tree>` with NO `-p`. Literal
// substrings are used (rather than fmt.Sprintf) to keep the test's import set
// to the mandated stdlib bytes/io/strings/testing + internal/ui.
func TestRescue_ExactCommandSubstrings(t *testing.T) {
	t.Run("non-root exact command", func(t *testing.T) {
		var stderr bytes.Buffer
		o := newRescueOutput(&stderr)
		Rescue(o, "9f3a1c0deadbeef", "abc1234", "")
		got := stderr.String()
		want := "commit-tree -p abc1234 -m \"Your message\" 9f3a1c0deadbeef | xargs git update-ref HEAD"
		if !strings.Contains(got, want) {
			t.Errorf("non-root rescue output missing exact command %q\n--got--\n%s", want, got)
		}
	})

	t.Run("root exact command", func(t *testing.T) {
		var stderr bytes.Buffer
		o := newRescueOutput(&stderr)
		Rescue(o, "9f3a1c0deadbeef", "", "")
		got := stderr.String()
		want := "commit-tree -m \"Your message\" 9f3a1c0deadbeef | xargs git update-ref HEAD"
		if !strings.Contains(got, want) {
			t.Errorf("root rescue output missing exact command %q\n--got--\n%s", want, got)
		}
	})
}
