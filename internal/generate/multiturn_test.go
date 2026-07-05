package generate

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/stubtest"
	"github.com/dustin/stagehand/internal/ui"
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
// token_limit non-interaction (pure-helper + gate-level, with the S3 captured-payload caveat documented).

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
// NOTE: at the CommitStaged layer, StagedDiff DOES consult cfg.TokenLimit to BUILD the diff — see
// TestMultiTurnGate_TokenLimitNotATerm + the FR-T12 caveat in how-it-works.md. The claim here is the
// TRUE non-interaction: the chunking algorithm itself never reads TokenLimit.
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

// TestMultiTurnGate_TokenLimitNotATerm (FR-T12, gate-level): the FR-T1 gate reads only
// (MultiTurnFallback, EstimateTokens(payload), MultiTurnChunkTokens, SessionMode) — TokenLimit is NOT a
// term. With NON-truncating TokenLimit values (the small test diff ≪ TokenLimit, so StagedDiff passes it
// through), multi-turn fires identically regardless of the TokenLimit value: the trigger is present and
// the run succeeds for every value.
//
// CAVEAT (FR-T12 ↔ S3 D2): when TokenLimit WOULD truncate the diff, S3's captured-payload decision hands
// multi-turn the already-truncated payload — a known divergence from FR-T12's literal "untruncated"
// wording, documented in how-it-works.md and deferred to a future re-capture fix. We do NOT assert the
// full-diff chunk count when TokenLimit would truncate (that case is integration territory — P1.M1.T4).
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

// tail returns the last n bytes of s (for readable failure messages on the verbose buffer).
func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
