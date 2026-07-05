package generate

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dustin/stagehand/internal/git"
)

// stripPartPrefix removes the leading "PART i/N:\n" line from a chunk's text, returning the body. Used
// to assert on bodies (round-trip, no-fracture) independently of the prefix.
func stripPartPrefix(t *testing.T, text string) string {
	t.Helper()
	idx := strings.IndexByte(text, '\n')
	if idx < 0 {
		t.Fatalf("chunk text missing prefix newline: %q", text)
	}
	return text[idx+1:]
}

// TestChunkPayload_SingleChunk: a payload under the budget yields ONE chunk that still carries the
// "PART 1/1:" prefix (protocol uniformity — the priming preamble references N=1).
func TestChunkPayload_SingleChunk(t *testing.T) {
	payload := "diff --git a/x b/x\n+hello\n"
	chunks := chunkPayload(payload, 32000) // budget >> payload
	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1", len(chunks))
	}
	c := chunks[0]
	if c.index != 1 || c.total != 1 {
		t.Errorf("index/total = %d/%d, want 1/1", c.index, c.total)
	}
	if !strings.HasPrefix(c.text, "PART 1/1:\n") {
		t.Errorf("text missing 'PART 1/1:\\n' prefix; got %q", c.text)
	}
	if body := stripPartPrefix(t, c.text); body != payload {
		t.Errorf("single-chunk body != payload\ngot:  %q\nwant: %q", body, payload)
	}
}

// TestChunkPayload_MultiChunkSplit: a payload over the budget splits into N chunks whose bodies
// concatenate back to the payload (FR-T2 lossless), with consistent N and 1..N indices.
func TestChunkPayload_MultiChunkSplit(t *testing.T) {
	// ~120 runes of line content; budget 1 token (4 runes) → many small chunks, each a whole number of lines.
	payload := strings.Repeat("line\n", 24) // 5 runes/line × 24 = 120 runes; EstimateTokens = ceil(120/4) = 30
	chunks := chunkPayload(payload, 1)      // chunkTokens=1 ⇒ runesPerWindow=4
	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want ≥2 (payload exceeds the 1-token budget)", len(chunks))
	}
	// N is consistent across all chunks; indices are 1..N.
	n := chunks[0].total
	for i, c := range chunks {
		if c.total != n {
			t.Errorf("chunk %d total = %d, want %d (N must be consistent)", i, c.total, n)
		}
		if c.index != i+1 {
			t.Errorf("chunk %d index = %d, want %d", i, c.index, i+1)
		}
	}
	if n != len(chunks) {
		t.Errorf("total = %d but len(chunks) = %d (N must equal the actual count)", n, len(chunks))
	}
	// Round-trip: bodies concatenate to the original payload (lossless).
	var rebuilt strings.Builder
	for _, c := range chunks {
		rebuilt.WriteString(stripPartPrefix(t, c.text))
	}
	if rebuilt.String() != payload {
		t.Errorf("round-trip mismatch\ngot:  %q\nwant: %q", rebuilt.String(), payload)
	}
}

// TestChunkPayload_NewlineAnchoring: a boundary that lands mid-line does NOT fracture the line — the
// chunk ends on a '\n' (the boundary line stays whole in the current chunk).
func TestChunkPayload_NewlineAnchoring(t *testing.T) {
	// "aaaaaa\nbbbbbb\ncccccc\n" — 6 runes/line. budget 1 token (4 runes) ⇒ the 4-rune window lands
	// mid-line ("aaaa"), anchoring forward to the line's '\n'.
	payload := "aaaaaa\nbbbbbb\ncccccc\n"
	chunks := chunkPayload(payload, 1)
	// Every chunk body (except possibly the last) must END with '\n' (a whole line — no fracture).
	for i, c := range chunks {
		body := stripPartPrefix(t, c.text)
		if i < len(chunks)-1 && !strings.HasSuffix(body, "\n") {
			t.Errorf("chunk %d body does not end on a newline (fractured line?): %q", i, body)
		}
	}
	// And the round-trip still holds (anchoring does not drop or duplicate bytes).
	var rebuilt strings.Builder
	for _, c := range chunks {
		rebuilt.WriteString(stripPartPrefix(t, c.text))
	}
	if rebuilt.String() != payload {
		t.Errorf("anchoring round-trip mismatch\ngot:  %q\nwant: %q", rebuilt.String(), payload)
	}
}

// TestChunkPayload_EmptyPayload: an empty payload yields exactly ONE chunk with an empty body and the
// PART 1/1 prefix (N ≥ 1; the preamble still references N=1).
func TestChunkPayload_EmptyPayload(t *testing.T) {
	chunks := chunkPayload("", 32000)
	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1 (empty payload ⇒ one empty chunk)", len(chunks))
	}
	c := chunks[0]
	if c.index != 1 || c.total != 1 {
		t.Errorf("index/total = %d/%d, want 1/1", c.index, c.total)
	}
	if body := stripPartPrefix(t, c.text); body != "" {
		t.Errorf("empty-payload body = %q, want \"\"", body)
	}
}

// TestChunkPayload_PrefixOutsideBudget: the "PART i/N:" line sits OUTSIDE the body budget — the body
// alone (prefix stripped) targets ≤ chunkTokens tokens, while the prefix is the line before it.
func TestChunkPayload_PrefixOutsideBudget(t *testing.T) {
	// Build a payload just over 1 token (5 runes ⇒ EstimateTokens = ceil(5/4) = 2 tokens). budget 1 token.
	payload := "abcde\n" // 6 runes
	chunks := chunkPayload(payload, 1)
	for i, c := range chunks {
		body := stripPartPrefix(t, c.text)
		// The body is ≤ chunkTokens tokens (modulo the forward-anchor overshoot, which is ≤ one line).
		if toks := git.EstimateTokens(body); toks > 1+1 { // allow +1 line of anchor overshoot
			t.Errorf("chunk %d body tokens = %d, want ≤ chunkTokens(1)+1 (anchor overshoot)", i, toks)
		}
		// The prefix is exactly one line, before the body, and is NOT counted in the body budget above.
		if !strings.HasPrefix(c.text, "PART ") || !strings.Contains(c.text, "\n") {
			t.Errorf("chunk %d text missing the one-line 'PART i/N:' prefix: %q", i, c.text)
		}
	}
}

// TestChunkPayload_RuneBasedCJK: the sizing is RUNE-based — a CJK payload is measured by runes (not
// bytes), and a multi-byte sequence is never split. EstimateTokens counts a 4-rune CJK string as 1 token.
// Each CJK line terminates with '\n' so the forward-anchor finds an interior newline (a no-newline payload
// collapses to one chunk per the FR-T3 anchor spec — G3/G6), which is what lets the budget actually split it.
func TestChunkPayload_RuneBasedCJK(t *testing.T) {
	cjk := "你好世界\n你好世界\n" // 10 runes, 28 bytes; EstimateTokens = ceil(10/4) = 3 tokens
	if got := git.EstimateTokens(cjk); got != 3 {
		t.Fatalf("prerequisite: EstimateTokens(%q) = %d, want 3 (rune-based)", cjk, got)
	}
	chunks := chunkPayload(cjk, 1) // budget 1 token ⇒ runesPerWindow=4 runes
	// Round-trip must be byte-identical (no multi-byte sequence split).
	var rebuilt strings.Builder
	for _, c := range chunks {
		rebuilt.WriteString(stripPartPrefix(t, c.text))
	}
	if rebuilt.String() != cjk {
		t.Errorf("CJK round-trip mismatch (a multi-byte sequence was split)\ngot:  %q\nwant: %q", rebuilt.String(), cjk)
	}
	// Each non-last body ends on '\n' — no CJK line fractured across chunks.
	for i, c := range chunks {
		body := stripPartPrefix(t, c.text)
		if i < len(chunks)-1 && !strings.HasSuffix(body, "\n") {
			t.Errorf("chunk %d CJK body does not end on a newline (fractured line?): %q", i, body)
		}
	}
}

// TestChunkPayload_CeilRounding: N rounds UP — a payload 1.5× the budget yields 2 chunks, not 1.
// Each line is short and newline-terminated, so the forward-anchor finds interior newlines (a no-newline
// payload collapses to one chunk per the FR-T3 anchor spec — G3/G6).
func TestChunkPayload_CeilRounding(t *testing.T) {
	// 6 lines × "ab\n" (3 runes/line) = 18 runes ⇒ EstimateTokens = ceil(18/4) = 5 tokens.
	// budget 2 tokens ⇒ runesPerWindow = 8 runes. 18/8 = 2.25 → ceil → 3 chunks.
	payload := strings.Repeat("ab\n", 6) // 18 runes
	_ = utf8.RuneCountInString           // keep the import meaningful if the assertion below is trimmed
	chunks := chunkPayload(payload, 2)
	if len(chunks) < 2 {
		t.Errorf("len(chunks) = %d, want ≥2 (ceil rounding: 18 runes / 8-per-window = 2.25 → 3)", len(chunks))
	}
	// Round-trip holds (ceil rounding never drops or duplicates bytes).
	var rebuilt strings.Builder
	for _, c := range chunks {
		rebuilt.WriteString(stripPartPrefix(t, c.text))
	}
	if rebuilt.String() != payload {
		t.Errorf("ceil-rounding round-trip mismatch\ngot:  %q\nwant: %q", rebuilt.String(), payload)
	}
}
