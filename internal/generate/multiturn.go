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
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/provider"
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

// ChunkCount returns the number of chunks chunkPayload would split payload into at the given
// chunkTokens budget. It is the exported cross-package helper for progress-message turn-count
// computation (the progress line prints N+1 turns where N = ChunkCount). It delegates to the
// unexported chunkPayload — the single source of truth for chunk sizing — so the count is always
// consistent with the actual N+1 turn protocol. Pure; no I/O.
func ChunkCount(payload string, chunkTokens int) int {
	return len(chunkPayload(payload, chunkTokens))
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

// preambleFmt is the turn-1 priming preamble (PRD §9.24 FR-T4, VERBATIM with N interpolated). The model
// is told to expect N parts and to reply "ok" to each, deferring the commit message until the final turn.
const preambleFmt = "I will send a git diff in %d parts. After each part, reply with exactly: ok. Do not analyze or write any commit message until I explicitly ask at the end."

// finalInstruction is the turn-N+1 request (PRD §9.24 FR-T4, VERBATIM). The candidate commit message is
// THIS turn's stdout (parsed by the existing §9.6 pipeline via ParseOutput).
const finalInstruction = "Now write the commit message for the diff above. Output ONLY the message."

// Run drives the N+1-turn multi-turn generation protocol (PRD §9.24 FR-T4/FR-T5/FR-T7). It is the
// transport layer invoked by the FR-T1 trigger gate (P1.M1.T3.S3, the sole caller) AFTER the one-shot
// path exhausted its retries on a payload that exceeds one chunk. Lossless (FR-T2): the SAME captured
// payload is chunked (chunkPayload) and delivered across N+1 sequential provider invocations against ONE
// session id — the model sees the entire diff in its session history, then writes one message at the end.
//
// Protocol (FR-T4):
//   - Turn 1:    system prompt (via the manifest's system_prompt_flag; turn-1-only) + preamble + chunk 1.
//   - Turns 2..N: each chunk's body ("PART i/N:\n<body>"); no system-prompt flag (turn > 1).
//   - Turn N+1:   the finalInstruction; THIS turn's stdout is the candidate message.
//
// Per-turn timeout = cfg.Timeout (FR-T5; Execute shadows ctx with WithTimeout). Intermediate turns'
// stdout ("ok") is discarded. Failure handling (FR-T7): ANY turn's Execute error OR any RenderMultiTurn
// error ⇒ Run aborts and returns the raw error as `cause` (the caller maps it to &RescueError{Cause:
// cause}). The decision is `execErr != nil` (NOT errors.Is(...)) because FR-T7 treats a turn timeout,
// cancel, AND non-zero-exit ALL as abort — and intermediate turns discard stdout anyway, so there is no
// value in parsing partial output (unlike the one-shot path's fall-through at generate.go:242). The
// final turn's parse outcome is (msg, ok): ok==false (empty/unparseable final stdout) is NOT a cause —
// the caller decides rescue per FR-T7's "final turn's output failing to parse".
//
// Run does NOT fork ParseOutput or run dedupe — the caller (CommitStaged, via S3) runs the existing
// dedupe path on the returned msg. Run does NOT check cfg.MultiTurnFallback or session_mode — S3 owns
// the trigger gate; RenderMultiTurn's own session_mode="append" gate is the defense-in-depth (a non-append
// manifest ⇒ a turn-1 render error ⇒ surfaced as cause).
func Run(ctx context.Context, deps Deps, cfg config.Config, manifest provider.Manifest,
	sysPrompt, payload, msgModel, msgReasoning string) (msg string, ok bool, cause error) {

	// (1) Chunk the captured payload (FR-T2 lossless; FR-T3 sizing — S1's helper).
	chunks := chunkPayload(payload, cfg.MultiTurnChunkTokens)
	N := len(chunks)

	// (2) Mint a fresh, one-run-scope session id (FR-T6 — never resumed on a later run).
	sessionID := newSessionID()

	// (3) Priming preamble (FR-T4, verbatim with N interpolated).
	preamble := fmt.Sprintf(preambleFmt, N)

	// (4) Turn 1: system prompt + preamble + chunk 1. RenderMultiTurn emits the system_prompt_flag only
	//     on turn 1 (its own turn-1-only gate). chunks[0].text already carries "PART 1/N:\n<body>".
	spec, rerr := manifest.RenderMultiTurn(msgModel, sysPrompt, preamble+"\n\n"+chunks[0].text,
		msgReasoning, sessionID, 1)
	if rerr != nil {
		return "", false, rerr // non-append provider ⇒ RenderMultiTurn's session_mode gate; surface as cause
	}
	if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {
		return "", false, execErr // FR-T7: any turn error/timeout/cancel/non-zero-exit aborts
	}

	// (6) Turns 2..N: each chunk's body; sysPrompt="" ⇒ RenderMultiTurn drops the system_prompt_flag.
	for i := 2; i <= N; i++ {
		spec, rerr := manifest.RenderMultiTurn(msgModel, "", chunks[i-1].text,
			msgReasoning, sessionID, i)
		if rerr != nil {
			return "", false, rerr
		}
		if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {
			return "", false, execErr // FR-T7
		}
	}

	// (7) Turn N+1 (final): the commit-message request. The candidate message is THIS turn's stdout.
	spec, rerr = manifest.RenderMultiTurn(msgModel, "", finalInstruction,
		msgReasoning, sessionID, N+1)
	if rerr != nil {
		return "", false, rerr
	}
	out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
	if execErr != nil {
		return "", false, execErr // (8) FR-T7: final-turn error aborts
	}

	// (9) Parse the final turn's stdout via the EXISTING §9.6 pipeline (FR-T4). Return (msg, ok, nil) —
	//     the caller runs dedupe. ok==false (empty/unparseable) is NOT a cause (caller decides rescue).
	m, parseOK, _ := provider.ParseOutput(out, manifest)
	return m, parseOK, nil
}

// newSessionID mints a fresh, one-run-scope session id for a multi-turn run (PRD §9.24 FR-T6). Format:
// "stagehand-<32 hex>" (16 cryptographically-random bytes). The id is NEVER resumed on a later run —
// providers that persist sessions leave it behind (harmless). Uses crypto/rand (no uuid library exists in
// the repo and none is added); the time-based fallback is defense-in-depth (rand.Read practically never
// fails on Linux/macOS/Windows).
func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("stagehand-%d", time.Now().UnixNano())
	}
	return "stagehand-" + hex.EncodeToString(b[:])
}
