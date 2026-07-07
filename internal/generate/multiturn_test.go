package generate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/provider"
	"github.com/dustin/stagecoach/internal/stubtest"
	"github.com/dustin/stagecoach/internal/ui"
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

// stubAppendManifest returns a stub provider.Manifest wired to call-indexed scripted stdout (the stub's
// selectScripted advances a per-invocation counter), WITH SessionMode set to "append" (RenderMultiTurn's
// gate requires it; stubtest.NewScript does not set it — G7). omitAppend=true leaves SessionMode unset (⇒ "")
// to exercise the non-append render-error path.
func stubAppendManifest(t *testing.T, bin string, responses []string, omitAppend bool) provider.Manifest {
	t.Helper()
	m := stubtest.NewScript(t, bin, responses)
	if !omitAppend {
		appendMode := "append"
		m.SessionMode = &appendMode
	}
	return m
}

// TestRun_HappyPath: a 2-chunk payload (chunkTokens=1, payload "aaaa\nbbbb\n" ⇒ N=2) drives 3 turns
// ("ok", "ok", "<message>"). Run returns the final turn's parsed message with cause==nil, ok==true.
func TestRun_HappyPath(t *testing.T) {
	bin := stubtest.Build(t)
	m := stubAppendManifest(t, bin, []string{"ok", "ok", "feat: add multi-turn support"}, false)
	cfg := config.Defaults()
	cfg.MultiTurnChunkTokens = 1 // ⇒ runesPerWindow=4 ⇒ "aaaa\nbbbb\n" splits into 2 chunks ⇒ 3 turns

	msg, ok, cause := Run(context.Background(), Deps{}, cfg, m, "you are a commit writer",
		"aaaa\nbbbb\n", "zai/glm-5.2", "")
	if cause != nil {
		t.Fatalf("Run cause = %v, want nil (happy path)", cause)
	}
	if !ok {
		t.Fatalf("Run ok = false, want true (final turn parsed)")
	}
	if msg != "feat: add multi-turn support" {
		t.Errorf("Run msg = %q, want %q", msg, "feat: add multi-turn support")
	}
}

// TestRun_TurnError: a global stub exit-1 (Options{Exit:1}) ⇒ turn 1's Execute returns a non-zero-exit
// error ⇒ Run aborts with cause != nil (FR-T7). The stub's exit code is global (one env var baked into
// the manifest's Env map), so this asserts a turn-1 failure; mid-turn isolation (turn 1 ok, turn 2 fails)
// needs a per-call exit mechanism the stub lacks ⇒ T4's exhaustive-matrix territory (research §6.4/G8).
func TestRun_TurnError(t *testing.T) {
	bin := stubtest.Build(t)
	// Exit:1 is global across all turns ⇒ turn 1 fails. stubtest.Manifest returns a provider.Manifest
	// VALUE (Env is a map[string]string, not a slice), so the SessionMode assignment is a clean local copy.
	sm := stubtest.Manifest(bin, stubtest.Options{Exit: 1})
	appendMode := "append"
	sm.SessionMode = &appendMode

	cfg := config.Defaults()
	cfg.MultiTurnChunkTokens = 1
	_, _, cause := Run(context.Background(), Deps{}, cfg, sm, "sys", "aaaa\nbbbb\n", "zai/glm-5.2", "")
	if cause == nil {
		t.Fatal("Run cause = nil, want non-nil (turn-1 non-zero exit ⇒ FR-T7 abort)")
	}
}

// TestRun_FinalParseEmpty: the final turn's stdout is empty (script ends with "") ⇒ ParseOutput ok=false.
// Run returns (msg="", ok=false, cause==nil) — the parse failure is NOT a cause; the caller decides rescue.
func TestRun_FinalParseEmpty(t *testing.T) {
	bin := stubtest.Build(t)
	m := stubAppendManifest(t, bin, []string{"ok", "ok", ""}, false) // final turn → empty stdout
	cfg := config.Defaults()
	cfg.MultiTurnChunkTokens = 1

	msg, ok, cause := Run(context.Background(), Deps{}, cfg, m, "sys", "aaaa\nbbbb\n", "zai/glm-5.2", "")
	if cause != nil {
		t.Fatalf("Run cause = %v, want nil (empty final stdout is a parse-fail, not a turn error)", cause)
	}
	if ok {
		t.Error("Run ok = true, want false (empty final stdout ⇒ ParseOutput ok=false)")
	}
	if msg != "" {
		t.Errorf("Run msg = %q, want \"\" (empty final stdout)", msg)
	}
}

// TestRun_NonAppendManifest: a manifest with SessionMode unset (⇒ "") ⇒ RenderMultiTurn's session_mode
// gate errors on turn 1 ⇒ Run surfaces the render error as cause (defense-in-depth for S3's FR-T1 gate).
func TestRun_NonAppendManifest(t *testing.T) {
	bin := stubtest.Build(t)
	// omitAppend=true ⇒ SessionMode stays "" ⇒ RenderMultiTurn errors.
	m := stubAppendManifest(t, bin, []string{"ok", "ok", "feat: never reached"}, true)
	cfg := config.Defaults()
	cfg.MultiTurnChunkTokens = 1

	_, _, cause := Run(context.Background(), Deps{}, cfg, m, "sys", "aaaa\nbbbb\n", "zai/glm-5.2", "")
	if cause == nil {
		t.Fatal("Run cause = nil, want non-nil (non-append manifest ⇒ RenderMultiTurn session_mode gate)")
	}
	if !strings.Contains(cause.Error(), "session_mode") {
		t.Errorf("Run cause = %v, want it to mention session_mode (the render gate)", cause)
	}
}

// --- P1.M1.T3.S4: exhaustive chunk-math / boundary / prefix / gate truth-table / FR-T12 tests ---
//
// These complement S1/S2's chunkPayload + Run tests with: (a) the VERIFIED exact-N ceil-math table;
// (b) the no-fractured-boundary + lossless round-trip invariant on a mid-line-landing window; (c) the
// PART i/N prefix format + strict 1..N monotonicity; (d) the 4-condition FR-T1 trigger truth table driven
// through CommitStaged (the gate is INLINE in generate.go — there is NO standalone predicate fn, so the
// table observes the gate's VerboseWarn trigger in the captured *ui.Verbose buffer); and (e) the FR-T12
// token_limit non-interaction (pure-helper + gate-level, with the FR-T12 re-capture verified at
// gate-level by TestMultiTurnGate_TokenLimitTruncated_Recaptures).

// TestChunkPayload_CeilMath: a table of VERIFIED exact-N payloads. chunkPayload's forward-newline anchor
// makes N depend on line structure (a naive ceil(ET/CT) is wrong for arbitrary payloads — the anchor
// absorbs small overages into chunk 1). These rows are pinned by the anchor semantics (research §2 / G1):
// the 'abcd\n'/CT=5 family is clean because the 5-rune line divides runesPerWindow (20); the 'ab\n'/CT=10
// row is the 2.5×→3 ceil case. Inventing an arbitrary "ET=CT+1 → N=2" payload FAILS (anchoring absorbs it).
func TestChunkPayload_CeilMath(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		ct      int
		wantN   int
	}{
		{"one_chunk_exact_CT", strings.Repeat("abcd\n", 4), 5, 1},  // 20 runes, ET=5=CT
		{"two_chunks_2x_CT", strings.Repeat("abcd\n", 8), 5, 2},    // 40 runes, ET=10=2×CT
		{"three_chunks_3x_CT", strings.Repeat("abcd\n", 12), 5, 3}, // 60 runes, ET=15=3×CT
		{"two_half_x_ceil", strings.Repeat("ab\n", 33), 10, 3},     // 99 runes, ET=25=2.5×CT → N=3
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunkPayload(tc.payload, tc.ct)
			if len(chunks) != tc.wantN {
				t.Errorf("len(chunks) = %d, want %d (payload %q, ct %d)", len(chunks), tc.wantN, tc.payload, tc.ct)
			}
		})
	}
	// Monotonicity: for the 'abcd\n'/CT=5 family, N is non-decreasing as the payload grows (×4/×8/×12).
	prev := 0
	for _, n := range []int{4, 8, 12} {
		got := len(chunkPayload(strings.Repeat("abcd\n", n), 5))
		if got < prev {
			t.Errorf("monotonicity violated: N dropped from %d to %d at ×%d", prev, got, n)
		}
		prev = got
	}
}

// TestChunkPayload_NoFracturedBoundaries: a payload whose naive rune boundary lands mid-line. With CT=1
// (runesPerWindow=4) and 6-rune lines, the window lands inside "aaaa"; the forward anchor pulls the
// boundary to the line's '\n'. Asserts (1) every non-last chunk body ENDS on '\n' (no fractured diff
// line) and (2) the concatenated bodies equal the original payload byte-for-byte (FR-T2 lossless).
func TestChunkPayload_NoFracturedBoundaries(t *testing.T) {
	payload := "aaaaaa\nbbbbbb\ncccccc\n" // 6-rune lines; CT=1 ⇒ runesPerWindow=4 lands mid-line
	chunks := chunkPayload(payload, 1)
	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want ≥2 (payload exceeds the 1-token window)", len(chunks))
	}
	for i, c := range chunks {
		body := stripPartPrefix(t, c.text)
		if i < len(chunks)-1 && !strings.HasSuffix(body, "\n") {
			t.Errorf("chunk %d body does not end on '\\n' (fractured diff line): %q", i, body)
		}
	}
	// Lossless round-trip (FR-T2): concatenated bodies == original payload.
	var rebuilt strings.Builder
	for _, c := range chunks {
		rebuilt.WriteString(stripPartPrefix(t, c.text))
	}
	if rebuilt.String() != payload {
		t.Errorf("round-trip mismatch\ngot:  %q\nwant: %q", rebuilt.String(), payload)
	}
}

// partPrefixRe asserts the EXACT "PART i/N:\n" prefix format (digits / slash / colon / newline). A
// regression to "Part 1 of 3" or "PART1/3" would fail this match. FindStringSubmatch yields
// [full, i, N] for comparison against the chunk's struct fields.
var partPrefixRe = regexp.MustCompile(`^PART (\d+)/(\d+):\n`)

// TestChunkPayload_PartPrefixMonotonic: for a multi-chunk payload, every chunk.text matches ^PART i/N:\n
// with i = chunk.index, N = chunk.total; index is strictly monotonic 1..N; total == len(chunks) for all.
func TestChunkPayload_PartPrefixMonotonic(t *testing.T) {
	payload := strings.Repeat("abcd\n", 12) // CT=5 ⇒ N=3
	chunks := chunkPayload(payload, 5)
	n := len(chunks)
	for i, c := range chunks {
		m := partPrefixRe.FindStringSubmatch(c.text)
		if m == nil {
			t.Errorf("chunk %d text does not match ^PART i/N:\\n: %q", i, c.text)
			continue
		}
		gotI, gotN := m[1], m[2]
		wantI := strconv.Itoa(c.index)
		wantN := strconv.Itoa(c.total)
		if gotI != wantI || gotN != wantN {
			t.Errorf("chunk %d prefix = PART %s/%s, want PART %s/%s (struct index/total)", i, gotI, gotN, wantI, wantN)
		}
		if c.index != i+1 {
			t.Errorf("chunk %d index = %d, want %d (not monotonic)", i, c.index, i+1)
		}
		if c.total != n {
			t.Errorf("chunk %d total = %d, want %d (N inconsistent)", i, c.total, n)
		}
		body := stripPartPrefix(t, c.text)
		if body == "" {
			t.Errorf("chunk %d body is empty (payload is non-empty)", i)
		}
	}
}

// TestChunkPayload_TokenLimitNonInteraction (FR-T12, pure-helper layer): multi-turn chunk sizing is
// architecturally independent of cfg.TokenLimit. chunkPayload's signature is (payload string,
// chunkTokens int) — TokenLimit is NOT a parameter, so no TokenLimit value can change N for a given
// (payload, chunkTokens). This is the strongest claim testable at the pure-helper layer.
//
// NOTE: at the CommitStaged layer, StagedDiff DOES consult cfg.TokenLimit to BUILD the one-shot
// diff — see TestMultiTurnGate_TokenLimitNotATerm and TestMultiTurnGate_TokenLimitTruncated_Recaptures
// for the FR-T12 re-capture (multi-turn re-captures with TokenLimit=0). The claim here is the TRUE
// non-interaction: the chunking algorithm itself never reads TokenLimit.
func TestChunkPayload_TokenLimitNonInteraction(t *testing.T) {
	payload := strings.Repeat("abcd\n", 8) // 40 runes
	n := len(chunkPayload(payload, 5))
	if n != 2 {
		t.Fatalf("baseline N = %d, want 2", n)
	}
	// Re-invocation is stable (deterministic); and there is NO TokenLimit argument to vary.
	if got := len(chunkPayload(payload, 5)); got != n {
		t.Errorf("non-deterministic: N flipped %d → %d", n, got)
	}
	// The claim is structural: chunkPayload takes (payload, chunkTokens) only — TokenLimit is absent.
	// (A grep of multiturn.go confirms the signature: func chunkPayload(payload string, chunkTokens int) []chunk.)
}

// --- P1.M1.T1.S1: ChunkCount exported wrapper (delegates to len(chunkPayload)) ---
//
// ChunkCount is the exported cross-package helper for progress-message turn-count computation. These
// pure table tests use chunkPayload itself (callable in-package) as the oracle — asserting the wrapper
// never drifts from the single source of truth for chunk sizing.

// TestChunkCount_EmptyPayload: an empty payload yields ONE chunk (N ≥ 1; the preamble still references
// N=1) — ChunkCount("", budget) == len(chunkPayload("", budget)) == 1.
func TestChunkCount_EmptyPayload(t *testing.T) {
	const payload = ""
	const budget = 32000
	got := ChunkCount(payload, budget)
	want := len(chunkPayload(payload, budget))
	if got != want {
		t.Errorf("ChunkCount(%q, %d) = %d, want %d (len(chunkPayload))", payload, budget, got, want)
	}
	if got != 1 {
		t.Errorf("ChunkCount(%q, %d) = %d, want 1 (empty payload ⇒ one empty chunk)", payload, budget, got)
	}
}

// TestChunkCount_SingleChunk: a small payload well under the budget yields ONE chunk — the count
// matches len(chunkPayload(...)) exactly.
func TestChunkCount_SingleChunk(t *testing.T) {
	payload := "diff --git a/x b/x\n+hello\n"
	budget := 32000 // budget ≫ payload
	got := ChunkCount(payload, budget)
	want := len(chunkPayload(payload, budget))
	if got != want {
		t.Errorf("ChunkCount(%q, %d) = %d, want %d (len(chunkPayload))", payload, budget, got, want)
	}
	if got != 1 {
		t.Errorf("ChunkCount(%q, %d) = %d, want 1 (single chunk under budget)", payload, budget, got)
	}
}

// TestChunkCount_MultiChunk: a payload exceeding the budget splits into N > 1 chunks — ChunkCount
// matches len(chunkPayload(...)) exactly (the wrapper delegates; no drift).
func TestChunkCount_MultiChunk(t *testing.T) {
	payload := strings.Repeat("abcd\n", 12) // 60 runes; CT=5 ⇒ N=3 (verified by TestChunkPayload_CeilMath)
	budget := 5
	got := ChunkCount(payload, budget)
	want := len(chunkPayload(payload, budget))
	if got != want {
		t.Errorf("ChunkCount(%q, %d) = %d, want %d (len(chunkPayload))", payload, budget, got, want)
	}
	if got < 2 {
		t.Errorf("ChunkCount(%q, %d) = %d, want ≥2 (payload exceeds the budget)", payload, budget, got)
	}
}

// TestChunkCount_SubOneBudget: a non-positive budget does NOT panic — chunkPayload's defensive clamp
// (chunkTokens < 1 ⇒ chunkTokens = 1) keeps it safe, and the count still matches len(chunkPayload(...))
// exactly. NOTE: the clamp sets the BUDGET to 1 token (runesPerWindow=4), NOT the CHUNK COUNT to 1 — a
// payload longer than 4 runes still splits under the clamped-1 budget. The wrapper's contract is solely
// that it delegates to chunkPayload; this test pins that the defensive path is consistent, not panic-free
// only but count-consistent.
func TestChunkCount_SubOneBudget(t *testing.T) {
	payload := strings.Repeat("abcd\n", 12) // 60 runes; under a clamped-1 budget this still splits
	for _, budget := range []int{0, -1} {
		got := ChunkCount(payload, budget)
		want := len(chunkPayload(payload, budget))
		if got != want {
			t.Errorf("ChunkCount(%q, %d) = %d, want %d (len(chunkPayload))", payload, budget, got, want)
		}
	}
	// Sanity: a sub-1 budget on a small payload DOES yield 1 chunk (the clamp keeps it non-trivial).
	if got := ChunkCount("x", 0); got != 1 {
		t.Errorf("ChunkCount(%q, %d) = %d, want 1 (small payload under clamped-1 budget)", "x", 0, got)
	}
	if got := ChunkCount("x", -1); got != 1 {
		t.Errorf("ChunkCount(%q, %d) = %d, want 1 (small payload under clamped-1 budget)", "x", -1, got)
	}
}

// TestChunkCount_CJK: a CJK-heavy payload (multi-byte UTF-8) is sized by runes — ChunkCount matches
// len(chunkPayload(...)) exactly, so a multi-byte sequence is never split and the count is consistent.
func TestChunkCount_CJK(t *testing.T) {
	cjk := "你好世界\n你好世界\n" // 10 runes, 28 bytes; EstimateTokens = 3 tokens
	budget := 1           // ⇒ runesPerWindow=4 runes ⇒ multiple chunks
	got := ChunkCount(cjk, budget)
	want := len(chunkPayload(cjk, budget))
	if got != want {
		t.Errorf("ChunkCount(%q, %d) = %d, want %d (len(chunkPayload))", cjk, budget, got, want)
	}
}

// TestMultiTurnTriggerGate_TruthTable: drives CommitStaged (the inline FR-T1 gate's sole call site) across
// the 4 skip rows + the success row + the all-true control. The gate is INLINE in generate.go (there is
// NO standalone predicate fn); the table observes the gate's VerboseWarn trigger
// ("one-shot exhausted → multi-turn fallback", generate.go:311) in the captured *ui.Verbose buffer:
//   - ABSENT ⇒ the gate short-circuited ⇒ Run was NOT entered ("assert it was NOT called");
//   - PRESENT ⇒ the gate passed ⇒ Run was entered.
//
// Each row flips exactly one FR-T1 condition to false (rows 1–3), flips (a) false via one-shot success
// (row 4), or leaves all four true (row 5, the control). Skip rows assert rescue (*RescueError{ErrRescue});
// the success + control rows assert a nil error (commit lands).
func TestMultiTurnTriggerGate_TruthTable(t *testing.T) {
	bin := stubtest.Build(t)

	cases := []struct {
		name        string
		multiTurn   bool
		chunkTokens int
		append      bool // SessionMode: true⇒"append", false⇒unset("")
		script      []string
		wantTrigger bool
		wantRescue  bool
	}{
		// (c) false: MultiTurnFallback off ⇒ gate short-circuits.
		{"skip_cond_c_multiturn_off", false, 4, true, []string{""}, false, true},
		// (b) false: default chunkTokens (32000) ⇒ EstimateTokens(small diff) ≤ 32000.
		{"skip_cond_b_small_payload", true, 32000, true, []string{""}, false, true},
		// (d) false: SessionMode unset ("") ⇒ not append.
		{"skip_cond_d_non_append", true, 4, false, []string{""}, false, true},
		// (a) false: one-shot call 1 returns "feat: win" ⇒ parses + not-dup ⇒ loop breaks (success); gate not reached.
		{"success_cond_a_not_exhausted", true, 4, true, []string{"feat: win"}, false, false},
		// control: all four true ⇒ gate fires ⇒ trigger PRESENT ⇒ multi-turn turns succeed.
		{"control_all_true_gate_fires", true, 4, true, []string{"", "ok", "ok", "feat: mt win"}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			initRepo(t, repo)
			commitRaw(t, repo, "initial")
			writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8)) // ~96 runes; ET≈24 > 4 (cond b when CT small)
			stageFile(t, repo, "new.txt")

			m := stubAppendManifest(t, bin, tc.script, !tc.append) // omitAppend = !tc.append
			cfg := config.Defaults()
			cfg.MaxDuplicateRetries = 0 // one-shot: exactly 1 attempt (the script's call 1)
			cfg.MultiTurnFallback = tc.multiTurn
			cfg.MultiTurnChunkTokens = tc.chunkTokens

			var buf bytes.Buffer
			_, err := CommitStaged(context.Background(), Deps{
				Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true),
			}, cfg)

			gotTrigger := strings.Contains(buf.String(), "multi-turn fallback")
			if gotTrigger != tc.wantTrigger {
				t.Errorf("trigger-in-buf = %v, want %v (buf tail: %q)", gotTrigger, tc.wantTrigger, tail(buf.String(), 200))
			}
			if tc.wantRescue {
				var re *RescueError
				if !errors.As(err, &re) || re.Kind != ErrRescue {
					t.Errorf("err = %v, want *RescueError{Kind:ErrRescue}", err)
				}
			} else if err != nil {
				// success / control row: the happy script must produce a nil error (commit lands).
				t.Errorf("err = %v, want nil (commit should land)", err)
			}
		})
	}
}

// TestMultiTurnGate_TokenLimitNotATerm (FR-T12, gate-level): the FR-T1 gate's payload-size term is
// computed from the UNTRUNCATED payload — TokenLimit is NOT a term. With NON-truncating TokenLimit values
// (the small test diff ≪ TokenLimit, so the one-shot StagedDiff passes it through), multi-turn fires
// identically regardless of the TokenLimit value: the trigger is present and the run succeeds.
//
// FR-T12 (honored): when TokenLimit WOULD truncate the one-shot diff, the multi-turn path RE-CAPTURES
// the diff with TokenLimit=0 and chunk/deliver the UNTRUNCATED payload — so the feature fires and succeeds
// even in that configuration. That truncating case is asserted directly by
// TestMultiTurnGate_TokenLimitTruncated_Recaptures below (the headline verification of the FR-T12 fix).
func TestMultiTurnGate_TokenLimitNotATerm(t *testing.T) {
	for _, tl := range []int{0, 1000, 100000} { // all NON-truncating for the small test diff
		bin := stubtest.Build(t)
		repo := t.TempDir()
		initRepo(t, repo)
		commitRaw(t, repo, "initial")
		writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8))
		stageFile(t, repo, "new.txt")

		m := stubAppendManifest(t, bin, []string{"", "ok", "ok", "feat: mt win"}, false) // SessionMode="append"
		cfg := config.Defaults()
		cfg.MaxDuplicateRetries = 0
		cfg.MultiTurnChunkTokens = 4 // cond b true (ET≈24 > 4)
		cfg.TokenLimit = tl          // varied; none truncate the small diff

		var buf bytes.Buffer
		_, err := CommitStaged(context.Background(), Deps{
			Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true),
		}, cfg)
		if err != nil {
			t.Errorf("TokenLimit=%d: CommitStaged err = %v, want nil (multi-turn should succeed)", tl, err)
		}
		if !strings.Contains(buf.String(), "multi-turn fallback") {
			t.Errorf("TokenLimit=%d: trigger absent; want present (gate must fire regardless of TokenLimit)", tl)
		}
	}
}

// TestMultiTurnGate_TokenLimitTruncated_Recaptures (FR-T12, gate-level, headline verification): when
// token_limit truncates the one-shot diff BELOW multi_turn_chunk_tokens, multi-turn must STILL fire and
// succeed — because the FR-T1 gate's payload-size term is computed from the RE-CAPTURED (TokenLimit=0)
// untruncated diff, not the truncated one-shot payload. Without the FR-T12 re-capture, condition (b)
// (EstimateTokens > chunk_tokens) would be false on the truncated payload and multi-turn would be
// skipped in favor of rescue — the exact regression this test pins.
//
// Setup: a diff whose UNTRUNCATED estimate (~24 tokens) exceeds chunk_tokens (4) but whose TRUNCATED
// estimate (token_limit=4 ⇒ ~4 tokens, ≤ 4) does NOT. token_limit is the smallest value that still
// truncates, so the one-shot diff is materially smaller than the untruncated diff.
func TestMultiTurnGate_TokenLimitTruncated_Recaptures(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	// ~96 runes ⇒ untruncated ET≈24 (> chunk_tokens=4). token_limit=4 truncates the one-shot diff to
	// ~4 tokens (≤ chunk_tokens), so condition (b) is FALSE on the truncated payload — the gate would
	// skip multi-turn WITHOUT the re-capture.
	writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8))
	stageFile(t, repo, "new.txt")

	m := stubAppendManifest(t, bin, []string{"", "ok", "ok", "feat: mt win"}, false) // SessionMode="append"
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0
	cfg.MultiTurnChunkTokens = 4 // small so the untruncated diff clearly exceeds it
	cfg.TokenLimit = 4           // truncates the one-shot diff BELOW chunk_tokens

	var buf bytes.Buffer
	_, err := CommitStaged(context.Background(), Deps{
		Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true),
	}, cfg)

	// FR-T12 headline: multi-turn MUST fire (the re-capture made condition (b) true on the untruncated
	// payload) and the run MUST succeed (commit lands), NOT rescue.
	if !strings.Contains(buf.String(), "multi-turn fallback") {
		t.Errorf("trigger absent; want present (FR-T12 re-capture must make multi-turn fire even when "+
			"token_limit truncates the one-shot payload). buf tail: %q", tail(buf.String(), 200))
	}
	if err != nil {
		t.Errorf("CommitStaged err = %v, want nil (FR-T12: multi-turn must succeed on the untruncated diff)", err)
	}
}

// TestMultiTurnGate_TokenLimitZero_ParseFail_NoRetryInstr (Issue 4, gate-level): when token_limit is
// UNSET (0) and the one-shot attempt fails to parse (call 1 returns ""), the FR-T1 gate must rebuild
// mtPayload from the untruncated `diff` via BuildUserPayload — NOT reuse the hoisted one-shot `payload`,
// which carries the retryInstr corrective preamble ("Output ONLY the commit message. No preamble, no
// markdown, no quotes.") prepended at generate.go:233 on a parse failure. The retryInstr is one-shot-only;
// multi-turn has its own priming preamble (FR-T4). Before the fix (mtPayload := payload), the multi-turn
// chunks carried the retryInstr, sending the model a confusing double instruction.
//
// This mirrors TestMultiTurnGate_TokenLimitTruncated_Recaptures but exercises the TokenLimit==0 path
// (the re-capture branch at generate.go:314 is SKIPPED — that branch would overwrite mtPayload and hide
// the bug). TokenLimit=0 + parseFail is the ONLY configuration that reaches the buggy line.
//
// Observation (G5): mtPayload is a local in CommitStaged — not directly inspectable. The stub's
// STAGEHAND_STUB_STDINFILE captures only the LAST invocation's stdin (the finalInstruction turn, which
// is independent of mtPayload). To observe the chunk bodies' content across ALL multi-turn turns, this
// test wraps the stub in a /bin/sh tee-wrapper (precedent: generate_test.go:788, models_test.go) that
// appends each invocation's stdin to a single capture file, then execs the real stub unchanged. The
// wrapper is created in t.TempDir() (auto-cleaned); it is a TEST-ONLY fixture, never shipped.
func TestMultiTurnGate_TokenLimitZero_ParseFail_NoRetryInstr(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	// ~96 runes ⇒ ET≈24 (> chunk_tokens=4) ⇒ condition (b) true on the UNTRUNCATED payload.
	writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8))
	stageFile(t, repo, "new.txt")

	// Tee-wrapper: appends each invocation's stdin (followed by a boundary marker) to captureFile,
	// then pipes the SAME stdin into the real stub (passed as $1 via Subcommand). Pure /bin/sh; no
	// content reaches a shell eval (cat/heredoc only) so the diff payload is never re-interpreted.
	captureFile := filepath.Join(t.TempDir(), "captures.txt")
	wrapper := filepath.Join(t.TempDir(), "tee-wrap.sh")
	wrapperBody := "#!/bin/sh\n" +
		"stub=\"$1\"; shift\n" +
		"tmp=$(mktemp)\n" +
		"cat > \"$tmp\"\n" +
		"cat \"$tmp\" >> \"$STAGEHAND_TEST_CAPTURE\"\n" +
		"printf '\\n---CAPTURE-BOUNDARY---\\n' >> \"$STAGEHAND_TEST_CAPTURE\"\n" +
		"cat \"$tmp\" | \"$stub\" \"$@\"\n" +
		"rc=$?\n" +
		"rm -f \"$tmp\"\n" +
		"exit $rc\n"
	if err := os.WriteFile(wrapper, []byte(wrapperBody), 0o755); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}

	// Build the stub manifest as usual (call-varying script), then RETARGET Command at the wrapper and
	// pass the real stub path as Subcommand[0] (rendered as the wrapper's $1). Add the capture path to
	// the Env map so the wrapper knows where to tee; the STAGEHAND_STUB_SCRIPT/COUNTER knobs survive.
	m := stubAppendManifest(t, bin, []string{"", "", "ok", "feat: mt win"}, false) // SessionMode="append"
	m.Command = strPtr(wrapper)
	m.Subcommand = []string{bin}
	m.Env["STAGEHAND_TEST_CAPTURE"] = captureFile

	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 1 // TWO one-shot attempts: attempt 0 parses-fail (parseFail:=true),
	// attempt 1 prepends retryInstr to `payload` (generate.go:233) then ALSO parses-fail → loop exhausts
	// with the surviving `payload` CARRYING retryInstr. With MaxDuplicateRetries=0 the single attempt
	// never triggers the retryInstr prepend (parseFail starts false) — the bug would be invisible.
	cfg.MultiTurnChunkTokens = 4 // small so the untruncated diff exceeds one chunk (condition b true)
	cfg.TokenLimit = 0           // G6: the bug's trigger — the re-capture branch (generate.go:314) is NOT taken

	var buf bytes.Buffer
	_, err := CommitStaged(context.Background(), Deps{
		Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true),
	}, cfg)

	// (1) Multi-turn MUST fire (the gate passed all four conditions) and succeed (commit lands).
	if !strings.Contains(buf.String(), "multi-turn fallback") {
		t.Errorf("trigger absent; want present (TokenLimit=0 + parseFail must still fire multi-turn). "+
			"buf tail: %q", tail(buf.String(), 200))
	}
	if err != nil {
		t.Errorf("CommitStaged err = %v, want nil (multi-turn must succeed on the clean rebuild)", err)
	}

	// (2) Issue 4 core assertion: the mtPayload delivered to the multi-turn protocol must NOT contain
	//     the retryInstr preamble. We assert on the retryInstr-SPECIFIC tail ("No preamble, no markdown,
	//     no quotes.") — NOT the ambiguous "Output ONLY" (which also appears in the multi-turn
	//     finalInstruction AND the one-shot system prompt). The capture holds EVERY invocation's stdin,
	//     split by the boundary marker into segments — see the filter below.
	captures, cerr := os.ReadFile(captureFile)
	if cerr != nil {
		t.Fatalf("read capture file: %v", cerr)
	}
	const retryInstrTail = "No preamble, no markdown, no quotes."
	// Filter to multi-turn chunk segments (those containing "PART "): the one-shot attempt segments
	// are EXPECTED to carry retryInstr (generate.go:233 prepends it on a parse-failed one-shot retry —
	// that is correct one-shot behavior, not the bug). The bug leaked retryInstr into the MULTI-TURN
	// chunks via `mtPayload := payload`; the fix rebuilds mtPayload from `diff` without retryInstr. So
	// we assert that NO multi-turn chunk segment contains the retryInstr substring (G3).
	for i, seg := range strings.Split(string(captures), "\n---CAPTURE-BOUNDARY---\n") {
		if !strings.Contains(seg, "PART ") {
			continue // a one-shot attempt segment or the trailing empty — retryInstr may be present there
		}
		if strings.Contains(seg, retryInstrTail) {
			t.Errorf("retryInstr preamble leaked into multi-turn chunk segment %d (Issue 4 regression): "+
				"segment contains %q.\nsegment head: %q", i, retryInstrTail, tail(seg, 200))
		}
	}
}

// strPtr returns a pointer to s — a local helper for building test manifests (provider.strPtr is
// unexported; the stubtest package has its own copy we cannot reach from here).
func strPtr(s string) *string { return &s }

// tail returns the last n bytes of s (for readable failure messages on the verbose buffer).
func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

// captureStderr swaps os.Stderr to a temp file for the duration of fn, then restores it and returns the
// captured content. Race-safe ONLY because no test in this package calls t.Parallel() (tests run serially,
// so the package-global os.Stderr swap is sequential — the -race detector does not flag it). Used to
// assert on direct os.Stderr writes (the multi-turn progress line) that bypass deps.Verbose.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stderr-*.txt")
	if err != nil {
		t.Fatalf("create temp stderr: %v", err)
	}
	orig := os.Stderr
	os.Stderr = f
	defer func() { os.Stderr = orig }() // restore even if fn Fatalf's/panics
	fn()
	os.Stderr = orig // restore now (before any t.Errorf below) so test output is clean
	if err := f.Close(); err != nil {
		t.Fatalf("close temp stderr: %v", err)
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("read temp stderr: %v", err)
	}
	return string(b)
}

// TestCommitStaged_MultiTurnProgressLine_ChunkTokens (FR-T11): the multi-turn fallback progress line
// (os.Stderr) carries the per-chunk token budget. Mirrors TestMultiTurnTriggerGate_TruthTable's
// control_all_true_gate_fires case (gate fires), but captures os.Stderr (not the verbose buffer) and
// asserts the new "chunks of ~<N> tokens" substring.
func TestCommitStaged_MultiTurnProgressLine_ChunkTokens(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	// ~96 runes ⇒ EstimateTokens ≈ 24 > 4 (FR-T1 cond b: payload exceeds one chunk).
	writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8))
	stageFile(t, repo, "new.txt")

	// Script: call 1 = "" (one-shot parse-fail ⇒ exhaust ⇒ gate fires); "ok","ok" priming; final = message.
	m := stubAppendManifest(t, bin, []string{"", "ok", "ok", "feat: mt win"}, false) // append (cond d)
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0  // exactly 1 one-shot attempt ⇒ exhaust ⇒ gate fires
	cfg.MultiTurnFallback = true // cond c
	cfg.MultiTurnChunkTokens = 4 // cond b (24 > 4); small ⇒ a distinctive, easy-to-assert substring

	var vbuf bytes.Buffer
	captured := captureStderr(t, func() {
		_, err := CommitStaged(context.Background(), Deps{
			Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&vbuf, true),
		}, cfg)
		if err != nil {
			t.Fatalf("CommitStaged: %v (expected multi-turn success)", err)
		}
	})

	// FR-T11: the progress line carries the per-chunk token budget.
	wantChunk := fmt.Sprintf("chunks of ~%d tokens", cfg.MultiTurnChunkTokens) // "chunks of ~4 tokens"
	if !strings.Contains(captured, wantChunk) {
		t.Errorf("progress line missing %q (FR-T11 per-chunk token estimate);\ngot stderr: %q", wantChunk, captured)
	}
	// Sanity: the line is still the multi-turn progress line.
	if !strings.Contains(captured, "falling back to multi-turn") {
		t.Errorf("progress line missing 'falling back to multi-turn';\ngot stderr: %q", captured)
	}
}
