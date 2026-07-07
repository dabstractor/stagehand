---
name: "P1.M1.T3.S1 — multiturn.go chunk-sizing helper (EstimateTokens ceil, newline-anchored, PART i/N prefix)"
description: |
  Create `internal/generate/multiturn.go` — a PURE string-math chunk-sizing helper for the multi-turn
  fallback (PRD §9.24 FR-T3). Defines `type chunk struct { index, total int; text string }` and
  `func chunkPayload(payload string, chunkTokens int) []chunk`. Splits the captured diff payload into N
  consecutive chunks each targeting ≤ chunkTokens TOKENS, anchoring every boundary FORWARD to the next
  newline so no diff line is fractured (the boundary line stays in the current chunk), and stamps each
  chunk's text with a one-line `PART i/N:` prefix emitted OUTSIDE the body budget. N = len(chunks) (the
  actual count, which realizes ceil(EstimateTokens(payload)/chunkTokens)); N ≥ 1 (empty payload ⇒ one
  empty chunk; small payload ⇒ one chunk that still carries `PART 1/1:` for protocol uniformity). Depends
  ONLY on `git.EstimateTokens` (internal/git/tokens.go:25, rune-based ceil(runes/4)) — the SINGLE
  estimator (no second estimator). The token budget is realized as `chunkTokens*4` RUNES (the
  rune-equivalent), walked rune-by-rune via a local `advanceRunes` so multi-byte UTF-8/CJK is never
  split. `ceilDiv` is redeclared locally (git's is unexported). Also creates a FOCUSED smoke test
  `internal/generate/multiturn_test.go` (the T2.S1 TestDefaults precedent) proving the core contract
  (single/multi chunk, newline anchoring, prefix-outside-budget, empty, CJK, ceil rounding, round-trip);
  P1.M1.T3.S4 EXTENDS this file with the exhaustive edge matrix + the trigger truth table + the
  token_limit non-interaction test + the how-it-works doc. Consumed by P1.M1.T3.S2 (the N+1 turn
  protocol). NO protocol, NO Execute calls, NO trigger gate, NO docs here — pure, unit-testable helper +
  smoke test only.
---

## Goal

**Feature Goal**: Land the pure, deterministic chunk-sizing primitive the multi-turn fallback (PRD
§9.24) uses to split a large captured diff payload into N consecutive request-sized chunks. The helper
is the sizing seam between "the captured payload" (FR-T2, delivered unchanged) and "the N+1 turn
protocol" (FR-T4, P1.M1.T3.S2): it partitions the payload so no diff line is fractured and stamps each
chunk with a `PART i/N:` prefix the protocol emits verbatim on turns 2..N.

**Deliverable** (two NEW files in `internal/generate/`):
1. **`multiturn.go`** (`package generate`) — the `chunk` type + the `chunkPayload` helper + two private
   helpers (`advanceRunes`, `ceilDiv`). Pure string math; depends only on `git.EstimateTokens`.
2. **`multiturn_test.go`** (`package generate`) — a focused smoke test (7 functions) proving the core
   contract; S4 extends it with the exhaustive matrix.

No other files touched. No production code outside `multiturn.go`. No docs.

**Success Definition**: `go build/vet/gofmt` clean; `go test -race ./internal/generate/` green with the
new smoke tests passing; `chunkPayload` partitions any payload into N ≥ 1 chunks whose bodies
concatenate back to the original payload, each body carries no fractured line (every chunk ends on a
`\n` or at end-of-payload), each chunk's `text` is `PART i/N:\n<body>` with `total = len(chunks)`
consistent across all chunks, the prefix sits OUTSIDE the body budget, multi-byte UTF-8 is never split,
and the SINGLE estimator `git.EstimateTokens` is the only sizing measure.

## User Persona

**Target User**: The contributor implementing P1.M1.T3.S2 (the N+1 turn protocol), which calls
`chunkPayload(payload, cfg.MultiTurnChunkTokens)` to obtain the per-turn bodies + the N for the turn-1
priming preamble ("I will send a git diff in N parts").

**Use Case**: A 266K-token diff that fails one-shot (per-request reliability ceiling below the context
window) is delivered losslessly across N+1 session turns. `chunkPayload` sizes the N request chunks so
each fits a reliable per-request budget (default 32000 tokens) without fracturing a diff line, and
stamps the `PART i/N:` prefix so the model can track the parts.

**User Journey**: caller passes the captured payload + `cfg.MultiTurnChunkTokens` → `chunkPayload`
returns `[]chunk{{1, N, "PART 1/N:\n…"}, …}` → S2 sends turn 1 (system prompt + priming preamble with
N + chunk 1), turns 2..N (each chunk's `text`), turn N+1 ("now write the message"). The bodies
concatenate to the original payload (lossless); no line is fractured.

**Pain Points Addressed**: Gives S2 a verified, pure, unit-testable sizing seam (no I/O, no Execute, no
protocol entanglement) so the turn-protocol subtask can focus on session/transport concerns. Prevents
the two classic chunking bugs: (a) a diff line fractured across chunks (confuses the model); (b) an
inconsistent `PART i/N` where N ≠ the actual chunk count (the preamble would mis-state the part count).

## Why

- **PRD §9.24 FR-T3 (chunk sizing) is the mandate:** *"Split the captured payload into N consecutive
  chunks each ≤ `multi_turn_chunk_tokens` (default 32000). `N = ceil(payload_tokens / chunk_size)`,
  using the shared `EstimateTokens`. Chunk boundaries anchor forward to the next newline so no diff
  line is fractured. Each chunk carries a one-line prefix ('PART i/N:') emitted OUTSIDE the chunk
  budget."* This helper IS FR-T3's sizing step, factored out as a pure function.
- **FR-T2 (lossless) is preserved by construction:** the bodies concatenate back to the payload
  exactly (round-trip); only the per-REQUEST size is bounded. This is the deliberate non-lossy
  alternative to the rejected "chunk-summarize-combine" scheme (§9.24 preamble; FUTURE_SPEC.md).
- **The single-estimator rule (research-generate-config.md §4):** `git.EstimateTokens` is the ONE
  token measure used everywhere (FR3d prompt-reserve, FR3i water-fill, FR-T3 sizing, FR-T1b gate).
  This helper consumes it for sizing and does NOT introduce a second estimator — keeping the budget
  arithmetic in consistent units across the codebase.
- **Foundation for P1.M1.T3.S2/S3/S4:** S2 consumes `chunkPayload` directly; S3's FR-T1 trigger gate
  uses `git.EstimateTokens(payload) > cfg.MultiTurnChunkTokens` (the same measure); S4 extends this
  test file with the exhaustive matrix. Landing a verified helper now lets those subtasks build on a
  tested foundation (the T2.S1 `TestDefaults` precedent).

## What

A pure helper `chunkPayload(payload string, chunkTokens int) []chunk` plus the `chunk` type and two
private helpers. The helper:

1. Guards `chunkTokens < 1` → treats it as 1 (defensive; collapses to a single chunk).
2. Converts the token budget to a RUNE budget: `runesPerWindow = chunkTokens * 4` (since
   `EstimateTokens = ceil(runes/4)`, a body of ≤ chunkTokens tokens ⟺ ≤ chunkTokens*4 runes).
3. Walks the payload in rune-sized windows of `runesPerWindow`, advancing each boundary FORWARD to
   the next `\n` (inclusive — the boundary line stays in the current chunk; no fracture). A
   rune-by-rune walk (`utf8.DecodeRuneInString`) never splits a multi-byte UTF-8 sequence.
4. Empty payload ⇒ a single chunk with an empty body (`N ≥ 1`).
5. `total = len(bodies)` (the actual count); each chunk = `{index: i+1, total, text:
   fmt.Sprintf("PART %d/%d:\n%s", i+1, total, body)}`.

Plus a focused smoke test (`multiturn_test.go`, 7 functions) proving: single chunk (small payload,
still `PART 1/1`); multi-chunk split (round-trip + consistent N + 1..N indices); newline anchoring
(no fractured line); prefix-outside-budget; empty payload; CJK rune-based sizing; ceil rounding.

### Success Criteria

- [ ] `internal/generate/multiturn.go` exists, `package generate`, with `type chunk struct { index,
      total int; text string }` and `func chunkPayload(payload string, chunkTokens int) []chunk`.
- [ ] `chunkPayload` uses `git.EstimateTokens` (the SINGLE estimator) for sizing semantics and
      `chunkTokens*4` runes as the window unit; NO second estimator introduced.
- [ ] Every boundary anchors FORWARD to the next `\n` (inclusive); no chunk ends mid-line unless the
      remainder of the payload has no further newline (a single line longer than the budget).
- [ ] `total == len(chunks)` on every chunk (the `PART i/N` N is self-consistent); indices are 1..N.
- [ ] Empty payload ⇒ exactly 1 chunk with body `""` and `PART 1/1:\n`.
- [ ] `chunkTokens < 1` ⇒ no panic (defensive clamp).
- [ ] The bodies of the returned chunks concatenate back to the original payload (lossless round-trip;
      the `PART i/N:` prefix is OUTSIDE each body).
- [ ] `internal/generate/multiturn_test.go` exists, `package generate`, with the 7 smoke tests, all
      passing.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] NO N+1 turn protocol, Execute calls, session logic, trigger gate, or docs in this subtask
      (S2/S3/S4 own those).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives the verbatim `multiturn.go` body (the `chunk` type +
`chunkPayload` + `advanceRunes` + `ceilDiv`, copy-paste-ready), the verbatim smoke-test bodies, the
exact `git.EstimateTokens` signature and formula (tokens.go:25), the unit-reconciliation decision
(chunkTokens tokens ⟺ chunkTokens*4 runes), the progress/no-infinite-loop proof, every edge case,
the import (internal/git is already imported in the generate package), and the hard scope fences
(S2 = protocol, S3 = gate, S4 = exhaustive tests + doc). No inference required.

### Documentation & References

```yaml
# MUST READ — the FR spec, the estimator, and this task's research
- file: PRD.md
  why: "§9.24 FR-T3 (chunk sizing: N = ceil(payload_tokens/chunk_size) via EstimateTokens; boundaries
        anchor forward to the next newline so no diff line is fractured; each chunk carries a one-line
        'PART i/N:' prefix OUTSIDE the chunk budget). §9.24 FR-T2 (lossless — the SAME captured payload,
        only per-request size bounded; no truncation/summarization). §9.24 FR-T4 (the turn protocol;
        turn-1 preamble interpolates N: 'I will send a git diff in N parts'). §9.1 FR3d (the ~4 chars ≈ 1
        token estimate; EstimateTokens is the single measure)."
  critical: "FR-T3 IS the implementation spec for chunkPayload. FR-T4's 'preamble interpolates N' is WHY
             total must equal the actual chunk count (D2). FR3d's chars/4 is the formula basis."

- docfile: plan/009_5c53066d64b3/architecture/research-generate-config.md
  why: "§4 pins git.EstimateTokens at tokens.go:25 (signature + rune-based ceil(runes/4) formula),
        EstimateTokensBytes at :32, ceilDiv at :37 (UNEXPORTED). §4's 'SINGLE estimator' rule: use it for
        both sizing and the gate; do NOT introduce a second estimator. §1c confirms the R3 chunk-sizing
        helper is the FIRST direct EstimateTokens(payload) call site in generate (beyond the :174 seam)."
  critical: "§4's SINGLE-estimator rule is binding. ceilDiv is unexported ⟹ multiturn.go needs its own
             local ceilDiv (or inline the expression)."

- docfile: plan/009_5c53066d64b3/P1M1T3S1/research/multiturn_chunk_sizing.md
  why: "THIS subtask's research: §3 the unit-reconciliation decision (D1: chunkTokens tokens ⟺
        chunkTokens*4 runes; D2: total = len(chunks) not the formula); §4 the verbatim algorithm;
        §5 every edge case; §6 the chunk type + PART prefix; §7 why the smoke test belongs in S1
        (T2.S1 TestDefaults precedent) and S4 extends it; §8 decisions D1–D7. READ THIS FIRST."
  critical: "§3 (D1/D2 — the unit reconciliation + total=len(chunks)) is the single most important
             insight: the contract's 'windows of ≤ chunkTokens' is the TOKEN budget, realized as
             chunkTokens*4 RUNES. §4's algorithm is copy-paste-ready. §5's edge cases are the test matrix."

- docfile: plan/009_5c53066d64b3/P1M1T2S1/PRP.md
  why: "The config CONTRACT (LANDED): Config.MultiTurnChunkTokens int (default 32000) + MultiTurnFallback
        bool (default true). chunkPayload's `chunkTokens` parameter IS cfg.MultiTurnChunkTokens at the
        S2/S3 call site. Confirms the default budget (32000) the helper must handle."
  critical: "The caller passes cfg.MultiTurnChunkTokens (≥1 in practice, guarded != 0 in the config
             layer). The helper's `chunkTokens < 1` clamp is defensive only."

# The dependency + the conventions (READ-ONLY — internal/git is the single dependency)
- file: internal/git/tokens.go
  why: "READ-ONLY (the single dependency). EstimateTokens(s string) int at :25 = ceilDiv(runeCount/4)
        (rune-based). EstimateTokensBytes at :32. ceilDiv at :37 (UNEXPORTED — do NOT reach for it;
        redeclare locally)."
  pattern: "rune-based ceil; (n+d-1)/d ceiling. The /4 is the chars≈token heuristic."
  gotcha: "ceilDiv is unexported. Redeclare a local ceilDiv in multiturn.go (same 1-line body). Do NOT
           add a new exported helper to internal/git (scope creep)."

- file: internal/generate/generate.go
  why: "READ-ONLY (package + import conventions). package generate (line 9); already imports
        internal/git (line 15; used at :174 `git.EstimateTokens`). So multiturn.go needs NO new import
        beyond unicode/utf8 (for advanceRunes) + fmt + strings (all stdlib)."
  pattern: "Sibling files (rescue.go, finalize.go, dedupe.go) are `package generate`, each with a
            focused doc comment naming the PRD section. Match that style for multiturn.go."
  gotcha: "Do NOT edit generate.go (S3 owns the trigger-gate insertion at ~:288). This task adds a NEW
           sibling file; generate.go is untouched."

# External references
- url: https://pkg.go.dev/unicode/utf8#DecodeRuneInString
  why: "DecodeRuneInString(s) returns (r rune, size int) — the byte size of the first UTF-8 rune in s.
        Used in advanceRunes to step byte-by-byte over RUNE boundaries (never splits a multi-byte
        sequence). s[end:] is an O(1) string-header slice (no copy/allocation)."
  critical: "Rune-by-rune decoding is what makes the walk RUNE-based (not byte-based) — the property
             that keeps CJK/emoji from being over-counted or split. Do NOT use len()/byte indexing."
- url: https://pkg.go.dev/strings#IndexByte
  why: "strings.IndexByte(s, '\n') returns the byte index of the first '\n' in s (or -1). Used to anchor
        the chunk boundary FORWARD to the next newline (inclusive). O(n) but on the small tail slice."
- url: https://git-scm.com/docs/git-diff#_description
  why: "Confirms a unified diff body is a sequence of newline-terminated lines (diff --git / --- / +++ /
        @@ / context / + / -). Anchoring chunk boundaries to '\n' therefore never fractures a logical
        diff line — the property FR-T3 mandates."
```

### Current Codebase Tree (this task's scope)

```bash
stagecoach/
└── internal/generate/
    ├── generate.go       # READ-ONLY (S3 owns the trigger-gate insertion at ~:288)
    ├── rescue.go         # READ-ONLY (sibling — the pure-string-assembler style to mirror)
    ├── finalize.go       # READ-ONLY (sibling — imports internal/git)
    ├── dedupe.go         # READ-ONLY (sibling)
    └── ...               # (existing tests unchanged)
# internal/git/tokens.go = READ-ONLY (the single dependency: git.EstimateTokens)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/generate/
    ├── multiturn.go      # NEW — chunk type + chunkPayload + advanceRunes + ceilDiv (pure string math)
    └── multiturn_test.go # NEW — 7 focused smoke tests (S4 extends with the exhaustive matrix)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/generate/multiturn.go` | CREATE | `package generate`. `chunk` type, `chunkPayload` helper, private `advanceRunes` + `ceilDiv`. Pure; depends only on `git.EstimateTokens`. |
| `internal/generate/multiturn_test.go` | CREATE | `package generate`. 7 focused smoke tests (single/multi/newline-anchor/prefix-outside-budget/empty/CJK/ceil). S4 extends. |

**Explicitly NOT touched**: `internal/generate/generate.go` (S3's trigger gate), `internal/generate/{rescue,finalize,dedupe,message}`.go and their tests, `internal/git/*` (read-only dependency), `internal/config/*` (T2 — config surface), `internal/provider/*` (T1 — session_mode/RenderMultiTurn), any docs (S4/S5), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — the unit reconciliation: chunkTokens is TOKENS; window by chunkTokens*4 RUNES). The
// contract's "windows of ≤ chunkTokens" is the TOKEN budget (FR-T3: "each ≤ multi_turn_chunk_tokens";
// N = ceil(payload_tokens/chunk_tokens)). EstimateTokens(s) = ceil(runeCount(s)/4), so a body of
// ≤ chunkTokens tokens ⟺ ≤ chunkTokens*4 runes. Window by chunkTokens*4 RUNES (NOT chunkTokens runes —
// that would be ~4× under budget and produce ~4× the predicted N). Walk rune-by-rune (DecodeRuneInString)
// so multi-byte UTF-8 is never split. (Research §3 / D1.)

// CRITICAL (G2 — total = len(chunks), NOT the pre-computed formula). The N in "PART i/N" must equal the
// ACTUAL chunk count, because S2's turn-1 preamble interpolates this same N ("I will send a git diff in
// N parts"). If you pre-compute N = ceil(EstimateTokens(payload)/chunkTokens) and the walk's forward-
// anchoring produces a different count, the labels lie. Set total = len(bodies) AFTER the walk. In
// practice len(bodies) == the formula N (the formula is the predictor; the walk is the truth). (D2.)

// CRITICAL (G3 — anchor FORWARD; the boundary line stays in the CURRENT chunk). When the tentative
// (runesPerWindow) boundary lands mid-line, extend end to the NEXT '\n' (inclusive — end += idx + 1).
// This may push the chunk's EstimateTokens slightly ABOVE chunkTokens (by the partial line) — that is
// ACCEPTABLE; the no-fracture guarantee (FR-T3) takes precedence over the ≤-budget target. Do NOT
// anchor BACKWARD (would push the partial line to the next chunk AND still risk a fractured read).

// CRITICAL (G4 — git.ceilDiv is UNEXPORTED; redeclare locally). internal/git/tokens.go:37 defines
// ceilDiv but does not export it. multiturn.go declares its OWN local ceilDiv (same 1-line body
// `(n+d-1)/d`). Do NOT add an exported ceilDiv to internal/git (scope creep across packages). (D3.)

// CRITICAL (G5 — use the SINGLE estimator git.EstimateTokens; do NOT introduce a second). FR-T3 and
// research-generate-config.md §4 bind this: EstimateTokens is the ONE token measure (FR3d reserve,
// FR3i water-fill, FR-T3 sizing, FR-T1b gate). chunkPayload realizes the budget as chunkTokens*4 runes
// (the rune-equivalent of EstimateTokens's /4); it does NOT call len()/byte-count or a second formula.

// GOTCHA (G6 — progress guarantee; no infinite loop). runesPerWindow = chunkTokens*4 ≥ 4 (chunkTokens
// clamped to ≥1), so advanceRunes advances ≥1 rune when offset < len(payload). The anchor only moves
// end FORWARD. Thus end > offset every iteration. Verify with a mental check: a payload with no '\n'
// and length > runesPerWindow → iteration 1 anchors end to len(payload) (IndexByte == -1) → loop ends
// after 1 chunk. No stuck loop.

// GOTCHA (G7 — payload[end:] is O(1); DecodeRuneInString on the slice is allocation-free). Go string
// slicing shares the backing array (a string header copy, no byte copy). So `payload[end:]` and
// `utf8.DecodeRuneInString(payload[end:])` do NOT allocate per iteration. Do NOT convert to []byte.

// GOTCHA (G8 — empty payload ⇒ ONE empty chunk; N ≥ 1). If the loop produces 0 bodies (payload == ""),
// append a single "" body so total = 1. The preamble (S2) still references N=1; the protocol is
// uniform. (A chunk with body "" still carries the "PART 1/1:\n" prefix.)

// GOTCHA (G9 — chunkTokens < 1 is defensive; do not assume the caller's guarantee). cfg.MultiTurnChunkTokens
// is guarded != 0 in the config layer (T2) so it is ≥1 in practice — but chunkPayload is a PURE helper
// that must not panic on bad input. Clamp chunkTokens < 1 to 1 (single chunk). Do not return nil/empty.

// GOTCHA (G10 — the smoke test is the T2.S1 TestDefaults precedent; S4 EXTENDS multiturn_test.go).
// Ship a FOCUSED smoke test (the 7 core-contract proofs). S4 owns the EXHAUSTIVE edge matrix
// (boundary-exact, chunkTokens=1, very-long-line-no-newline, huge-payload), the trigger truth table
// (S3's gate), the token_limit non-interaction test, and the how-it-works doc. Do NOT write the
// exhaustive matrix here (S4's territory — avoid duplication). Use plain if/t.Errorf (no testify —
// matches generate/*_test.go style).

// GOTCHA (G11 — round-trip is the lossless proof). strings.Join(bodies, "") == payload. The bodies
// partition the payload exactly (offset chains: body[i] ends where body[i+1] starts). The "PART i/N:\n"
// prefix is OUTSIDE the body (it is prepended in the text field, not part of body). Assert round-trip
// in the smoke test — it is the FR-T2 lossless guarantee made executable.
```

## Implementation Blueprint

### Data models and structure

```go
// chunk is one part of a multi-turn chunked payload (PRD §9.24 FR-T3). index/total carry the "PART i/N"
// labeling; text is the ready-to-send turn body ("PART i/N:\n<body>"). The prefix is OUTSIDE the body
// budget (body targets ≤ chunkTokens tokens; the prefix is one short line on top).
type chunk struct {
	index int    // 1-based part number (the i in "PART i/N")
	total int    // N = len(chunks); identical across all chunks in a result
	text  string // "PART i/N:\n<body>" — the prefix line, then the chunk's body
}
```

No other types. The helper returns `[]chunk`.

### `multiturn.go` (exact — copy verbatim)

```go
// Package generate — multi-turn chunk sizing (PRD §9.24 FR-T3).
//
// chunkPayload is the PURE string-math sizing seam between the captured diff payload (FR-T2, delivered
// unchanged) and the N+1 turn protocol (FR-T4, P1.M1.T3.S2). It splits the payload into N consecutive
// chunks each targeting ≤ chunkTokens TOKENS, anchoring every boundary FORWARD to the next newline so
// no diff line is fractured, and stamps each chunk with a one-line "PART i/N:" prefix OUTSIDE the body
// budget. The bodies concatenate back to the payload exactly (FR-T2 lossless). Depends only on
// git.EstimateTokens (the SINGLE estimator — FR3d/FR3i/FR-T3/FR-T1b all share it).
package generate

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dustin/stagecoach/internal/git"
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
// The token budget is realized as chunkTokens*4 RUNES — the rune-equivalent of EstimateTokens's
// ceil(runes/4) formula (a body of ≤ chunkTokens tokens ⟺ ≤ chunkTokens*4 runes). The walk advances
// rune-by-rune (utf8.DecodeRuneInString) so a multi-byte UTF-8 sequence (CJK, emoji) is never split and
// is not over-counted. Forward-anchoring may push a chunk's estimate slightly above chunkTokens (by the
// partial boundary line); the no-fracture guarantee takes precedence over the ≤-budget target (FR-T3).
//
// Pure string math; no I/O, no error. Consumed by the N+1 turn protocol (P1.M1.T3.S2), which also reads
// total (N) for the turn-1 priming preamble ("I will send a git diff in N parts", FR-T4).
func chunkPayload(payload string, chunkTokens int) []chunk {
	if chunkTokens < 1 {
		chunkTokens = 1 // defensive: a non-positive budget collapses to a single chunk (no panic)
	}
	// chunkTokens tokens ≈ chunkTokens*4 runes (EstimateTokens = ceil(runes/4)). Window by RUNES, not
	// bytes, so multi-byte UTF-8 is never split mid-sequence.
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

// _ = git.EstimateTokens // (compile-time anchor) — the SINGLE estimator (FR-T3/FR3d). chunkPayload's
// budget is the rune-equivalent (chunkTokens*4) of EstimateTokens's /4; this import is the dependency
// surface. (Remove this guard line once a direct git.EstimateTokens call is added in S3's gate or S4's
// token_limit test — it exists only so goimports does not drop the dependency mid-plan.)
var _ = git.EstimateTokens
```

> **Note on the `_ = git.EstimateTokens` anchor:** it keeps the `internal/git` import meaningful in
> THIS file (otherwise, since chunkPayload realizes the budget as `chunkTokens*4` runes rather than
> calling `git.EstimateTokens` inline, goimports/go vet would flag the import as unused). The
// dependency is real (the budget unit is *defined* by EstimateTokens's /4). S3/S4 will call
// `git.EstimateTokens` directly (the gate / the token_limit non-interaction test), at which point the
// anchor can be removed. If you prefer, drop the anchor and add an inline comment citing the formula
// instead — but keep the `internal/git` import only if something in the file references it (Go rejects
// unused imports). The cleanest no-anchor option: add a tiny `chunkTokensToRunes` helper that calls
// `git.EstimateTokens` semantics inline — but that over-engineers a `*4`. The anchor is the pragmatic
// choice; document why.

> **Alternative (PREFERRED if the implementer dislikes the blank-assign anchor):** drop the
> `internal/git` import entirely from multiturn.go and express the budget as `chunkTokens * 4` with a
> comment citing `git.EstimateTokens = ceil(runes/4)` (tokens.go:25) as the source of the /4. The helper
> then has ZERO non-stdlib dependencies (truly pure string math), and the SINGLE-estimator rule is
> honored at the CALL SITE (S3's gate / S4's test call `git.EstimateTokens` directly). Choose ONE: keep
> the anchor (explicit dependency) OR drop the import + comment (pure-stdlib). Either is correct; the
> PRP's smoke-test `TestChunkPayload_UsesEstimateTokensSemantics` pins the /4 relationship either way.

### `multiturn_test.go` (exact — the focused smoke test; S4 extends)

```go
package generate

import (
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/git"
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
func TestChunkPayload_RuneBasedCJK(t *testing.T) {
	cjk := "你好世界你好世界" // 8 runes, 24 bytes; EstimateTokens = ceil(8/4) = 2 tokens
	if got := git.EstimateTokens(cjk); got != 2 {
		t.Fatalf("prerequisite: EstimateTokens(%q) = %d, want 2 (rune-based)", cjk, got)
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
	// And the chunk count matches the rune-based prediction (8 runes / 4-per-window = 2).
	if len(chunks) != 2 {
		t.Errorf("len(chunks) = %d, want 2 (8 runes / 4-per-window; rune-based sizing)", len(chunks))
	}
}

// TestChunkPayload_CeilRounding: N rounds UP — a payload 1.5× the budget yields 2 chunks, not 1.
func TestChunkPayload_CeilRounding(t *testing.T) {
	// 12 content runes ⇒ EstimateTokens = ceil(12/4) = 3 tokens. budget 2 tokens ⇒ runesPerWindow = 8 runes.
	// 12/8 = 1.5 → ceil → 2 chunks (NOT 1).
	payload := "0123456789ab\n" // 13 runes (12 content + newline)
	chunks := chunkPayload(payload, 2)
	if len(chunks) < 2 {
		t.Errorf("len(chunks) = %d, want ≥2 (ceil rounding: 12 runes / 8-per-window = 1.5 → 2)", len(chunks))
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/generate/multiturn.go
  - FILE: internal/generate/multiturn.go ; PACKAGE: generate.
  - IMPORTS: fmt, strings, unicode/utf8 (stdlib) + github.com/dustin/stagecoach/internal/git.
    (DECIDE: keep the `var _ = git.EstimateTokens` anchor OR drop the internal/git import and cite the
    /4 in a comment. The PRP's §"multiturn.go" note explains both; pick ONE. The smoke test imports
    internal/git regardless, so the dependency is exercised either way.)
  - WRITE the file verbatim from §"multiturn.go" above: the `chunk` type, `chunkPayload`, `advanceRunes`,
    `ceilDiv`. Keep the doc comments (they cite PRD §9.24 FR-T3 and the SINGLE-estimator rule).
  - DO NOT: add the N+1 turn protocol (S2), Execute calls, session logic (S2), the trigger gate (S3),
    docs (S4/S5), or any I/O. Pure string math only.
  - VERIFY: go build ./internal/generate/ → exit 0.

Task 2: CREATE internal/generate/multiturn_test.go
  - FILE: internal/generate/multiturn_test.go ; PACKAGE: generate (white-box — same as generate/*_test.go).
  - IMPORTS: strings, testing (stdlib) + github.com/dustin/stagecoach/internal/git (for
    the EstimateTokens assertions in TestChunkPayload_PrefixOutsideBudget and _RuneBasedCJK).
  - WRITE the file verbatim from §"multiturn_test.go" above: stripPartPrefix helper + the 7 tests.
  - DO NOT: write the exhaustive edge matrix (S4), the trigger truth table (S4 — S3's gate), the
    token_limit non-interaction test (S4), or the how-it-works doc (S4). Plain if/t.Errorf (no testify).
  - VERIFY: go test -race ./internal/generate/ -run TestChunkPayload → all 7 PASS.

Task 3: VALIDATE — full gate set + scope discipline
  - RUN: gofmt -w internal/generate/multiturn.go internal/generate/multiturn_test.go ; gofmt -l .
  - RUN: go vet ./...
  - RUN: go build ./...
  - RUN: go test -race ./internal/generate/    # the 7 new tests + existing generate tests green
  - RUN: go test -race ./...                    # whole repo green (additive new files)
  - RUN (single-estimator grep — no second estimator introduced):
        grep -nE 'len\(.*\)|/[0-9]+|RuneCount' internal/generate/multiturn.go | grep -v 'utf8\|len(payload)\|len(bodies)\|len(chunks)\|len(s)'
        # EXPECT: no ad-hoc token formula (the budget is chunkTokens*4 runes, citing EstimateTokens's /4).
  - RUN (scope): git status --porcelain → expect EXACTLY:
        ?? internal/generate/multiturn.go
        ?? internal/generate/multiturn_test.go
  - RUN (no-protocol grep): grep -nE 'Execute|session|SessionID|priming|preamble|turn' internal/generate/multiturn.go
        # EXPECT: only doc-comment mentions of "turn protocol" (P1.M1.T3.S2) — NO actual protocol code.
```

### Implementation Patterns & Key Details

```go
// === The unit reconciliation (D1/G1) ===
// chunkTokens is TOKENS (FR-T3). EstimateTokens(s) = ceil(utf8.RuneCountInString(s) / 4). So a body of
// ≤ chunkTokens tokens ⟺ ≤ chunkTokens*4 runes. Window by chunkTokens*4 RUNES:
//   runesPerWindow := chunkTokens * 4
// This is why a 4-rune CJK string is 1 token (not 3 bytes) and why the walk must be rune-by-rune.

// === Why total = len(chunks), not the pre-computed formula (D2/G2) ===
// The turn-1 preamble (S2, FR-T4) interpolates N: "I will send a git diff in N parts". If N (the formula)
// disagreed with the actual chunk count (the walk, after forward-anchoring), the preamble would lie.
// total = len(bodies) (set AFTER the walk) guarantees N == the number of "PART i/N" chunks emitted.
// In practice they're equal: len(bodies) = ceil(totalRunes / (chunkTokens*4)) ≈ ceil(tokens/chunkTokens).

// === The forward anchor (D4/G3) ===
//   end := advanceRunes(payload, offset, runesPerWindow)   // tentative (≤ budget runes)
//   if i := strings.IndexByte(payload[end:], '\n'); i >= 0 { end += i + 1 }  // anchor FORWARD (include '\n')
//   else { end = len(payload) }                             // no more newlines → take the remainder
// The boundary line stays in the CURRENT chunk (no fracture). Overshoot ≤ one line (acceptable).

// === Why advanceRunes is rune-by-rune (G7) ===
//   for i := 0; i < n && end < len(s); i++ { _, size := utf8.DecodeRuneInString(s[end:]); end += size }
// Byte indexing (end += 1) would split a multi-byte UTF-8 sequence. DecodeRuneInString returns the
// rune's BYTE SIZE, so end advances one RUNE per iteration. s[end:] is an O(1) slice (no allocation).

// === The round-trip is the FR-T2 lossless proof (G11) ===
// offset chains: body[i] is payload[offset_i : offset_{i+1}], so offset_{i+1} = offset_i + len(body[i]).
// strings.Join(bodies, "") == payload exactly. The "PART i/N:\n" prefix is prepended in `text`, NOT in
// body, so it never perturbs the round-trip. Assert this in TestChunkPayload_MultiChunkSplit.
```

### Integration Points

```yaml
PRODUCTION (internal/generate/multiturn.go — NEW):
  - chunk type + chunkPayload(payload, chunkTokens) []chunk + advanceRunes + ceilDiv (private)

TESTS (internal/generate/multiturn_test.go — NEW, S4 extends):
  - 7 focused smoke tests (single/multi/newline-anchor/prefix-outside-budget/empty/CJK/ceil)

CONSUMED (READ-ONLY — the single dependency):
  - internal/git/tokens.go: git.EstimateTokens(s) int (:25) = ceil(runeCount/4); EstimateTokensBytes (:32)

CALLER (downstream — owned by S2/S3, NOT this task):
  - P1.M1.T3.S2 (N+1 turn protocol): chunks := chunkPayload(payload, cfg.MultiTurnChunkTokens);
    sends turn 1 (sys prompt + priming preamble with chunks[0].total + chunk 1), turns 2..N (chunk.text),
    turn N+1 (final). Reads chunk.index/total/text.
  - P1.M1.T3.S3 (FR-T1 trigger gate): git.EstimateTokens(payload) > cfg.MultiTurnChunkTokens (same measure)
  - P1.M1.T3.S4 (exhaustive tests + doc): EXTENDS multiturn_test.go (edge matrix, trigger truth table,
    token_limit non-interaction); writes docs/how-it-works.md multi-turn section.

CONFIG (downstream — T2 LANDED):
  - cfg.MultiTurnChunkTokens (default 32000) is the chunkTokens argument at the S2 call site
  - cfg.MultiTurnFallback (default true) is the FR-T1c gate condition (S3)

GATE: go test -race ./internal/generate/ → GREEN ; git status → ONLY the 2 new files

NO-TOUCH (explicitly — owned by siblings):
  - internal/generate/generate.go (S3 trigger gate), rescue.go/finalize.go/dedupe.go/message.go (siblings)
  - internal/git/* (read-only dependency), internal/config/* (T2), internal/provider/* (T1)
  - docs/* (S4/S5), README.md (S5), PRD.md, tasks.json, prd_snapshot.md, plan/*
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/generate/multiturn.go internal/generate/multiturn_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/generate/...   # Expected: exit 0
go build ./...                   # Expected: exit 0

# Expected: zero errors. If `imported and not used: "github.com/dustin/stagecoach/internal/git"` in
# multiturn.go, you dropped the `var _ = git.EstimateTokens` anchor without adding a direct call —
# either restore the anchor OR drop the import and cite the /4 in a comment (the PRP's §"multiturn.go"
# note explains both options).
```

### Level 2: The Smoke Tests (the deliverable)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./internal/generate/ -v -run TestChunkPayload
# Expected: ALL 7 PASS:
#   TestChunkPayload_SingleChunk            — small payload ⇒ 1 chunk w/ PART 1/1 prefix; body == payload
#   TestChunkPayload_MultiChunkSplit        — N chunks; round-trip; consistent N; 1..N indices
#   TestChunkPayload_NewlineAnchoring       — every non-last body ends on '\n' (no fractured line)
#   TestChunkPayload_EmptyPayload           — "" ⇒ 1 chunk, empty body, PART 1/1
#   TestChunkPayload_PrefixOutsideBudget    — body ≤ chunkTokens+1 tokens; prefix is one line before it
#   TestChunkPayload_RuneBasedCJK           — CJK round-trip byte-identical; rune-based chunk count
#   TestChunkPayload_CeilRounding           — N rounds up (1.5× budget ⇒ 2 chunks)
```

### Level 3: Whole-Repository Regression + Scope Discipline

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...    # Expected: ALL packages green (two additive new files; no existing file touched)
go vet ./...           # Expected: exit 0

# Scope: ONLY the two new files.
git status --porcelain
# Expected EXACTLY:
#   ?? internal/generate/multiturn.go
#   ?? internal/generate/multiturn_test.go

# No-protocol / no-second-estimator grep:
grep -nE 'Execute|SessionID|priming|preamble' internal/generate/multiturn.go || true
# Expected: only doc-comment mentions of "turn protocol (P1.M1.T3.S2)" — NO actual protocol code.

# Single-estimator discipline: the budget is chunkTokens*4 runes (EstimateTokens's /4), not an ad-hoc formula.
grep -n 'chunkTokens \* 4' internal/generate/multiturn.go
# Expected: exactly one match (the runesPerWindow derivation, with the comment citing EstimateTokens).

# Sibling/production territory UNTOUCHED:
git diff --stat -- internal/generate/generate.go internal/git/ internal/config/ internal/provider/ docs/
# Expected: EMPTY.
```

### Level 4: Behavioral Cross-Check (manual repro of the chunk math)

```bash
cd /home/dustin/projects/stagecoach

# Reproduce the core property by hand (mirrors what the smoke tests assert): a payload splits into N
# chunks whose bodies concatenate back to the original, with no fractured line. The authoritative proof
# is the unit test (Level 2); this is an optional sanity smoke via a tiny throwaway program.
cat > /tmp/sh_chunk.go <<'EOF'
package main
import ("fmt";"strings"
 "github.com/dustin/stagecoach/internal/generate")
func main() {
	payload := strings.Repeat("line\n", 24) // 120 runes
	chunks := generate.ChunkPayloadExported(payload, 1) // (see note below)
	var rebuilt strings.Builder
	for _, c := range chunks { rebuilt.WriteString(c.Text) }
	fmt.Printf("N=%d round-trip=%v\n", len(chunks), rebuilt.String() == payload)
}
EOF
# NOTE: chunk/chunkPayload are UNEXPORTED in package generate, so a /tmp main can't call them directly.
# The authoritative proof is the in-package smoke test (Level 2). Delete the scratch file:
rm -f /tmp/sh_chunk.go
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.
- [ ] `go test -race ./internal/generate/ -v -run TestChunkPayload` — all 7 smoke tests PASS.

### Feature Validation
- [ ] `chunkPayload` returns N ≥ 1 chunks for every input (empty ⇒ 1 empty chunk; small ⇒ 1 chunk).
- [ ] Bodies concatenate back to the payload (FR-T2 lossless round-trip).
- [ ] No chunk ends mid-line (every non-last body ends on `\n`; the boundary line stays whole).
- [ ] `total == len(chunks)` on every chunk; indices 1..N (PART i/N self-consistent).
- [ ] The `PART i/N:` prefix is OUTSIDE the body budget (body alone ≤ chunkTokens tokens + ≤1 line overshoot).
- [ ] Multi-byte UTF-8 (CJK) is never split; sizing is rune-based (EstimateTokens semantics).
- [ ] `chunkTokens < 1` does not panic (defensive clamp).
- [ ] The SINGLE estimator `git.EstimateTokens` is the only sizing basis (chunkTokens*4 runes = its /4).

### Scope Discipline Validation
- [ ] ONLY `internal/generate/multiturn.go` + `internal/generate/multiturn_test.go` created (new files).
- [ ] NO N+1 turn protocol, Execute/session/priming code, or trigger gate (S2/S3).
- [ ] NO exhaustive edge matrix / trigger truth table / token_limit test / how-it-works doc (S4).
- [ ] Did NOT edit `internal/generate/generate.go` (S3) or any sibling file; did NOT touch `internal/git/*`,
      `internal/config/*`, `internal/provider/*`, docs, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Code Quality Validation
- [ ] `multiturn.go` has a focused doc comment citing PRD §9.24 FR-T3 + the SINGLE-estimator rule.
- [ ] Private helpers `advanceRunes` + `ceilDiv` are documented (ceilDiv = local copy; git's is unexported).
- [ ] Smoke test mirrors `generate/*_test.go` style (plain if/t.Errorf; no testify; white-box package).
- [ ] The unit reconciliation (chunkTokens tokens ⟺ chunkTokens*4 runes) is documented in the code.

---

## Anti-Patterns to Avoid

- ❌ Don't window by `chunkTokens` RUNES (literal reading of "windows of ≤ chunkTokens"). `chunkTokens` is
  a TOKEN budget (FR-T3); a chunkTokens-RUNE window is ~4× under budget and produces ~4× the predicted N.
  Window by `chunkTokens*4` runes (the rune-equivalent of EstimateTokens's /4) (gotcha G1/D1).
- ❌ Don't set `total` to the pre-computed formula `ceil(EstimateTokens(payload)/chunkTokens)`. The walk's
  forward-anchoring can shift the count; the preamble (S2) interpolates N, so N MUST equal the actual
  chunk count. Set `total = len(bodies)` AFTER the walk (gotcha G2/D2).
- ❌ Don't anchor BACKWARD (push the partial line to the next chunk). FR-T3 says anchor FORWARD — the
  boundary line stays in the CURRENT chunk. Backward anchoring still risks inconsistency and diverges
  from the spec (gotcha G3/D4).
- ❌ Don't reach for `git.ceilDiv` — it is UNEXPORTED. Redeclare a local `ceilDiv` in multiturn.go (same
  1-line body). Do NOT add an exported helper to internal/git (scope creep) (gotcha G4/D3).
- ❌ Don't introduce a second token estimator. `git.EstimateTokens` (ceil(runes/4)) is the SINGLE measure
  (FR3d/FR3i/FR-T3/FR-T1b). The budget is `chunkTokens*4` runes (its rune-equivalent); no `len()`/byte
  formula, no chars/3 variant (gotcha G5).
- ❌ Don't byte-index the walk (`end += 1`). That splits multi-byte UTF-8 sequences. Use
  `utf8.DecodeRuneInString` to advance by the rune's BYTE SIZE (gotcha G7).
- ❌ Don't return nil / an empty slice for an empty payload. N ≥ 1: return a single chunk with an empty
  body + `PART 1/1:\n` (the preamble references N=1) (gotcha G8/D6).
- ❌ Don't assume `chunkTokens ≥ 1` from the caller. chunkPayload is a PURE helper that must not panic;
  clamp `chunkTokens < 1` to 1 (gotcha G9/D5).
- ❌ Don't write the N+1 turn protocol, the trigger gate, or the exhaustive test matrix here. This task is
  the pure helper + a focused SMOKE test only (S2 = protocol, S3 = gate, S4 = exhaustive matrix + doc)
  (gotcha G10/D7).
- ❌ Don't drop the round-trip assertion. `strings.Join(bodies, "") == payload` is the FR-T2 lossless
  guarantee made executable — the single most important correctness property (gotcha G11).
- ❌ Don't leave the `internal/git` import unused (Go rejects it). Either keep the `var _ = git.EstimateTokens`
  anchor OR drop the import and cite the /4 in a comment (the PRP explains both; pick one).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a small, self-contained pure-function task with a verbatim, copy-paste-ready
implementation (the `chunk` type, `chunkPayload`, `advanceRunes`, `ceilDiv`), a verbatim smoke-test
suite, and a single read-only dependency (`git.EstimateTokens`, confirmed at tokens.go:25 with the exact
rune-based ceil(runes/4) formula). Four independent de-riskings: (1) the unit-reconciliation decision
(chunkTokens tokens ⟺ chunkTokens*4 runes) is pinned with a proof and a CJK test that would catch a
byte/factor-of-4 mistake; (2) `total = len(chunks)` guarantees the PART i/N labels are self-consistent
(the preamble-N tension is resolved); (3) the forward-anchor + round-trip assertions make the
no-fracture + lossless properties executable; (4) the generate package already imports internal/git
(generate.go:15) and follows a consistent sibling-file style (rescue.go/finalize.go). The progress/no-
infinite-loop guarantee is proven (runesPerWindow ≥ 4 ⇒ ≥1 rune/iteration). The residual uncertainty
(not 10/10) is the cosmetic `var _ = git.EstimateTokens` anchor vs dropping the import — a real Go
"unused import" sharp edge the implementer must resolve one of two documented ways; and the S4 boundary
(S4 extends multiturn_test.go — if S4's PRP doesn't note the existing file, minor merge care is needed,
but S4 is planned/sequential, not parallel). No production-code risk outside the new file; no parallel-
edit risk (only new files in internal/generate/, which no sibling touches — S2 adds a Run func to a
distinct file or this same file later, S3 edits generate.go, S4 edits the test file).
