package prompt

// White-box test for [AssemblePayload] (P1.M4.T1.S3), matching the
// internal/ui, internal/provider, internal/git, and S2's system_test.go house
// convention: the _test.go file is `package prompt` (NOT `package prompt_test`)
// so it can reference the unexported §17.3 rejection-string constants
// (rejectionHeader, rejectionBulletPrefix, rejectionFooter) and the
// [rejectionBlock] helper directly. It exercises a PURE string-building
// function, so it needs stdlib `testing` + `strings` ONLY — NO internal/git,
// NO os/exec, NO testify (no real-git integration test is needed). Uses
// strings.Contains for presence checks (robust to join whitespace) and exact
// `==` equality for the deterministic-bytes / base-layout cases.

import (
	"strings"
	"testing"
)

// TestAssemblePayload_BaseLayout_DiffFirst is ★ THE D5 INVARIANT ★
// (reference_impl.md §3 + §4 D5): with no rejected subjects the payload is
// EXACTLY `<diff>\n\n<instruction>` — diff FIRST, instruction LAST, NEVER
// instruction-first. Asserts exact equality AND that the diff precedes the
// instruction by index.
func TestAssemblePayload_BaseLayout_DiffFirst(t *testing.T) {
	got := AssemblePayload("DIFF", "DO", nil)

	const want = "DIFF\n\nDO"
	if got != want {
		t.Errorf("AssemblePayload(DIFF,DO,nil) = %q; want %q (exact, diff-first)", got, want)
	}

	// Diff-first by index: the diff starts at 0 and the instruction appears
	// strictly AFTER it.
	diffIdx := strings.Index(got, "DIFF")
	instrIdx := strings.Index(got, "DO")
	if diffIdx != 0 {
		t.Errorf("diff index = %d; want 0 (diff must be FIRST)", diffIdx)
	}
	if !(instrIdx > diffIdx) {
		t.Errorf("instruction index = %d, diff index = %d; want instruction AFTER diff", instrIdx, diffIdx)
	}
}

// TestAssemblePayload_EmptySliceEqualsNil covers the len(nil)==0 path: an empty
// slice and nil both yield the base layout (no rejection block) and the two are
// byte-identical.
func TestAssemblePayload_EmptySliceEqualsNil(t *testing.T) {
	nilGot := AssemblePayload("D", "I", nil)
	emptyGot := AssemblePayload("D", "I", []string{})

	const want = "D\n\nI"
	if nilGot != want {
		t.Errorf("AssemblePayload(D,I,nil) = %q; want %q (base layout, no block)", nilGot, want)
	}
	if emptyGot != want {
		t.Errorf("AssemblePayload(D,I,[empty]) = %q; want %q (base layout, no block)", emptyGot, want)
	}
	if nilGot != emptyGot {
		t.Errorf("empty slice != nil: nil=%q empty=%q (len(nil)==0 must make them identical)", nilGot, emptyGot)
	}
}

// TestAssemblePayload_RejectionBlockPresent covers the retry layout: with
// rejected non-empty the block contains the §17.3 header + footer and a bullet
// per subject in SLICE ORDER, the block sits AFTER the instruction, and the
// full payload equals exactly `<diff>\n\n<instruction>\n\n` + rejectionBlock.
func TestAssemblePayload_RejectionBlockPresent(t *testing.T) {
	rejected := []string{"dup subject one", "dup subject two"}
	got := AssemblePayload("DIFF", "INSTR", rejected)

	// Presence: header, footer, and each bullet verbatim.
	for _, want := range []string{rejectionHeader, rejectionFooter, "- dup subject one", "- dup subject two"} {
		if !strings.Contains(got, want) {
			t.Errorf("payload missing %q\n--- payload ---\n%s", want, got)
		}
	}

	// Slice order: bullet one precedes bullet two.
	firstIdx := strings.Index(got, "- dup subject one")
	secondIdx := strings.Index(got, "- dup subject two")
	if firstIdx < 0 || secondIdx < 0 {
		t.Fatalf("could not locate both bullets; first=%d second=%d", firstIdx, secondIdx)
	}
	if !(firstIdx < secondIdx) {
		t.Errorf("bullets not in slice order; first=%d second=%d (want first < second)", firstIdx, secondIdx)
	}

	// The rejection block sits AFTER the instruction (block appended LAST).
	headerIdx := strings.Index(got, rejectionHeader)
	instrIdx := strings.Index(got, "INSTR")
	if !(headerIdx > instrIdx) {
		t.Errorf("rejection header index = %d; instruction index = %d; want block AFTER instruction", headerIdx, instrIdx)
	}

	// Exact deterministic bytes: `<diff>\n\n<instruction>\n\n` + rejectionBlock.
	want := "DIFF\n\nINSTR\n\n" + rejectionBlock(rejected)
	if got != want {
		t.Errorf("retry payload = %q; want %q (exact)", got, want)
	}
}

// TestAssemblePayload_RejectionBlockSingleSubject covers the single-subject
// retry case: exactly ONE bullet and the full layout equals
// `<diff>\n\n<instruction>\n\n` + rejectionBlock([only]).
func TestAssemblePayload_RejectionBlockSingleSubject(t *testing.T) {
	rejected := []string{"only"}
	got := AssemblePayload("D", "I", rejected)

	if !strings.Contains(got, "- only") {
		t.Errorf("payload missing the single bullet %q\n--- payload ---\n%s", "- only", got)
	}
	if n := strings.Count(got, rejectionBulletPrefix); n != 1 {
		t.Errorf("bullet count = %d; want exactly 1 (a single rejected subject)", n)
	}

	want := "D\n\nI\n\n" + rejectionBlock(rejected)
	if got != want {
		t.Errorf("single-subject retry payload = %q; want %q (exact)", got, want)
	}
}

// TestAssemblePayload_RejectionBlockAbsentWhenEmpty asserts the base layout
// (nil AND empty slice) carries NO rejection block: neither the header nor the
// footer appears, and there is no bullet prefix as a rejection marker.
func TestAssemblePayload_RejectionBlockAbsentWhenEmpty(t *testing.T) {
	for _, rejected := range [][]string{nil, {}} {
		got := AssemblePayload("D", "I", rejected)
		if strings.Contains(got, rejectionHeader) {
			t.Errorf("base payload (rejected=%v) unexpectedly contains rejectionHeader\n--- payload ---\n%s", rejected, got)
		}
		if strings.Contains(got, rejectionFooter) {
			t.Errorf("base payload (rejected=%v) unexpectedly contains rejectionFooter\n--- payload ---\n%s", rejected, got)
		}
	}
}

// TestRejectionBlock_ExactBytes is a white-box test on the unexported
// [rejectionBlock] helper pinning the EXACT §17.3 byte layout: header, ONE
// newline to the first bullet, bullets joined by "\n", a BLANK line, then the
// footer. This locks the deterministic byte structure independent of how
// AssemblePayload composes the surrounding sections.
func TestRejectionBlock_ExactBytes(t *testing.T) {
	got := rejectionBlock([]string{"s1", "s2"})
	want := rejectionHeader + "\n- s1\n- s2\n\n" + rejectionFooter
	if got != want {
		t.Errorf("rejectionBlock([s1,s2]) = %q; want %q (exact §17.3 byte layout)", got, want)
	}
}

// TestAssemblePayload_Deterministic asserts identical inputs always yield
// byte-identical outputs on BOTH the base and the retry paths (a pure builder
// has no hidden state, time, or randomness).
func TestAssemblePayload_Deterministic(t *testing.T) {
	// Base path.
	base1 := AssemblePayload("DIFF", "DO", nil)
	base2 := AssemblePayload("DIFF", "DO", nil)
	if base1 != base2 {
		t.Errorf("base not deterministic: %q vs %q", base1, base2)
	}

	// Retry path.
	rej := []string{"a", "b", "c"}
	retry1 := AssemblePayload("DIFF", "DO", rej)
	retry2 := AssemblePayload("DIFF", "DO", rej)
	retry3 := AssemblePayload("DIFF", "DO", rej)
	if retry1 != retry2 || retry2 != retry3 {
		t.Errorf("retry not deterministic across repeated calls:\n1=%q\n2=%q\n3=%q", retry1, retry2, retry3)
	}
}
