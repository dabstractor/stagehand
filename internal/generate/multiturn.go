// Package generate — multi-turn chunk sizing (PRD §9.24 FR-T3).
//
// chunkPayload is the PURE string-math sizing seam between the captured diff payload (FR-T2, delivered
// unchanged) and the N+1 turn protocol (FR-T4, P1.M1.T3.S2). It splits the payload into N consecutive
// chunks each targeting ≤ chunkTokens TOKENS, anchoring every boundary FORWARD to the next newline so
// no diff line is fractured, and stamps each chunk with a one-line "PART i/N:" prefix OUTSIDE the body
// budget. The bodies concatenate back to the payload exactly (FR-T2 lossless). The budget unit is
// defined by the SINGLE estimator git.EstimateTokens (internal/git/tokens.go:25 = ceil(runes/4)) — the
// ONE token measure used by FR3d/FR3i/FR-T3/FR-T1b. chunkPayload realizes that budget as chunkTokens*4
// RUNES (the rune-equivalent of the /4); the direct git.EstimateTokens call site lives in S3's trigger
// gate and S4's token_limit non-interaction test (this file is pure string math, no non-stdlib deps).
package generate

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// chunk is one part of a multi-turn chunked payload (PRD §9.24 FR-T3). index/total carry the "PART i/N"
// labeling the turn protocol emits; text is the ready-to-send per-turn body ("PART i/N:\n<body>"). The
// prefix line sits OUTSIDE the body budget (body targets ≤ chunkTokens tokens; the prefix adds its own
// tokens on top — acceptable, it is one short line).
type chunk struct {
	index int    // 1-based part number (the i in "PART i/N")
	total int    // N = len(chunks); identical across all chunks in a result
	text  string // "PART i/N:\n<body>"
}

// chunkPayload splits payload into N consecutive chunks each targeting ≤ chunkTokens tokens (PRD §9.24
// FR-T3), anchoring every boundary FORWARD to the next newline so no diff line is fractured. Each chunk
// carries a "PART i/N:" prefix OUTSIDE the body budget. N (total) = the actual chunk count (≥ 1; an
// empty payload yields a single empty chunk; a small payload yields one chunk that still carries the
// "PART 1/1:" prefix for protocol uniformity).
//
// The token budget is realized as chunkTokens*4 RUNES — the rune-equivalent of git.EstimateTokens's
// ceil(runes/4) formula (tokens.go:25): a body of ≤ chunkTokens tokens ⟺ ≤ chunkTokens*4 runes. The
// walk advances rune-by-rune (utf8.DecodeRuneInString) so a multi-byte UTF-8 sequence (CJK, emoji) is
// never split and is not over-counted. Forward-anchoring may push a chunk's estimate slightly above
// chunkTokens (by the partial boundary line); the no-fracture guarantee takes precedence over the
// ≤-budget target (FR-T3).
//
// Pure string math; no I/O, no error. Consumed by the N+1 turn protocol (P1.M1.T3.S2), which also reads
// total (N) for the turn-1 priming preamble ("I will send a git diff in N parts", FR-T4).
func chunkPayload(payload string, chunkTokens int) []chunk {
	if chunkTokens < 1 {
		chunkTokens = 1 // defensive: a non-positive budget collapses to a single chunk (no panic)
	}
	// chunkTokens tokens ≈ chunkTokens*4 runes (git.EstimateTokens = ceil(runes/4); tokens.go:25 — the
	// SINGLE estimator). Window by RUNES, not bytes, so multi-byte UTF-8 is never split mid-sequence.
	runesPerWindow := chunkTokens * 4

	// Walk the payload in runesPerWindow-rune windows, anchoring each boundary forward to the next '\n'.
	var bodies []string
	offset := 0 // byte offset into payload
	for offset < len(payload) {
		end := advanceRunes(payload, offset, runesPerWindow) // ≤ runesPerWindow runes forward
		// Anchor FORWARD: include the boundary line's terminating '\n' in THIS chunk (no fracture).
		if i := strings.IndexByte(payload[end:], '\n'); i >= 0 {
			end += i + 1
		} else {
			end = len(payload) // no further newline → take the remainder
		}
		bodies = append(bodies, payload[offset:end])
		offset = end
	}
	// Empty payload ⇒ a single empty chunk (N ≥ 1; the preamble still references N=1).
	if len(bodies) == 0 {
		bodies = append(bodies, "")
	}

	total := len(bodies)
	chunks := make([]chunk, total)
	for i, body := range bodies {
		chunks[i] = chunk{
			index: i + 1,
			total: total,
			text:  fmt.Sprintf("PART %d/%d:\n%s", i+1, total, body),
		}
	}
	return chunks
}

// advanceRunes returns the byte offset obtained by advancing n runes forward from start in s, clamped
// to len(s). Steps rune-by-rune via utf8.DecodeRuneInString so a multi-byte UTF-8 sequence is never
// split (and s[start:] is an O(1) string-header slice — no allocation). Stops after n runes; does not
// scan the whole string.
func advanceRunes(s string, start, n int) int {
	end := start
	for i := 0; i < n && end < len(s); i++ {
		_, size := utf8.DecodeRuneInString(s[end:])
		end += size
	}
	return end
}

// ceilDiv returns the ceiling of n/d for n≥0, d>0: (n+d-1)/d. Local copy — git.ceilDiv (internal/git/
// tokens.go:37) is unexported. Kept for parity with the estimator's convention (EstimateTokens = ceil(runes/4)).
func ceilDiv(n, d int) int { return (n + d - 1) / d }
