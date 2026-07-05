# multiturn.go chunk-sizing helper — P1.M1.T3.S1 Research

> Verified against the live repo (module `github.com/dustin/stagehand`). The token estimator
> `git.EstimateTokens` (internal/git/tokens.go:25) and the generate-package conventions are confirmed.
> No files modified — research only.

## 1. What this task is (and is not)

Create `internal/generate/multiturn.go` — a PURE string-math chunk-sizing helper for the multi-turn
fallback (PRD §9.24 FR-T3). It splits the captured diff payload into N consecutive request-sized
chunks (each ≤ the token budget), anchoring every boundary FORWARD to the next newline so no diff
line is fractured, and stamps each chunk with a one-line `PART i/N:` prefix OUTSIDE the body budget.

**Scope fence (the plan):**
- **This S1** = the `chunk` type + `chunkPayload` helper (pure production code) + a focused smoke
  test (`multiturn_test.go`) proving the core contract.
- **P1.M1.T3.S2** = the N+1 turn protocol `Run` (session id, priming preamble, per-turn Execute,
  final parse) — CONSUMES `chunkPayload`. Do NOT implement the protocol here.
- **P1.M1.T3.S3** = wire the FR-T1 trigger gate into `CommitStaged` (reads `cfg.MultiTurnFallback`
  + `cfg.MultiTurnChunkTokens`).
- **P1.M1.T3.S4** = the EXHAUSTIVE chunk-math edge matrix + the trigger truth table (S3's gate) +
  the token_limit non-interaction test + the Mode A how-it-works.md doc. **S4 EXTENDS
  `multiturn_test.go`** — S1's smoke test is the minimal contract-proof subset (mirrors how T2.S1
  shipped `TestDefaults` as a smoke while T2.S3 added the dedicated exhaustive test file).

## 2. The token estimator (the SINGLE dependency)

`internal/git/tokens.go:25` — confirmed exact:

```go
func EstimateTokens(s string) int {
    return ceilDiv(utf8.RuneCountInString(s), 4)
}
```

- **RUNE-based** (`utf8.RuneCountInString`, NOT `len(s)`) — multi-byte UTF-8/CJK/emoji does NOT
  over-count (a 4-rune/12-byte CJK string estimates as 1 token, not 3).
- Formula: `ceil(runeCount / 4)`, rounded UP.
- Also available: `EstimateTokensBytes(b []byte) int` (tokens.go:32) — same formula, `[]byte` form
  (not needed here; the payload is a `string`).
- **`ceilDiv` is UNEXPORTED** (tokens.go:37) — `internal/generate` cannot reuse it. multiturn.go
  declares its OWN local `ceilDiv` (1 line, same body). (Alternatively inline the `(n+d-1)/d`
  expression; a named local is clearer and matches the git package's style.)
- `internal/generate` ALREADY imports `internal/git` (generate.go:15; used at :174
  `git.EstimateTokens`). So `git.EstimateTokens(payload)` is a direct call — no new import beyond
  `unicode/utf8` (for the rune-walk helper).

**Use this SINGLE estimator for both sizing (the chunk count) and the per-chunk budget.** Do NOT
introduce a second estimator (FR-T3 anchors on EstimateTokens; research-generate-config.md §4).

## 3. The unit-reconciliation decision (the contract's ambiguity, resolved)

The contract says: *"N = ceilDiv(EstimateTokens(payload), chunkTokens)"* AND *"Walk the payload in
rune-count-sized windows of ≤ chunkTokens."* These two sentences use DIFFERENT units if read
literally — `chunkTokens` is a TOKEN count (FR-T3: "each ≤ `multi_turn_chunk_tokens`"; default
32000), but "rune-count-sized windows of ≤ chunkTokens" reads as ≤ chunkTokens RUNES. A literal
rune-window of chunkTokens runes would be ~4× under budget and produce ~4× the predicted N —
inconsistent with the formula.

**Decision D1 (the reconciliation):** the per-chunk budget is `chunkTokens` TOKENS. Since
`EstimateTokens(s) = ceil(runeCount(s)/4)`, a body of ≤ chunkTokens tokens ⟺ ≤ `chunkTokens*4`
runes. The walk windows by **`chunkTokens*4` RUNES** (the rune-equivalent of the token budget),
advancing rune-by-rune (`utf8.DecodeRuneInString`) so a multi-byte sequence is never split. Each
chunk's `EstimateTokens` is therefore ≈ chunkTokens (the forward newline-anchor may overshoot by
one line — acceptable; the no-fracture guarantee takes precedence over the ≤-budget target). This
makes `len(chunks) = ceil(totalRunes / (chunkTokens*4)) ≈ ceil(EstimateTokens(payload)/chunkTokens)`
= the formula's N.

**Decision D2 (total = actual count, not the pre-computed formula):** `total` (the `N` in
`PART i/N`) is **`len(chunks)`** — the ACTUAL chunk count after the walk — never a pre-computed
formula value. This guarantees the `PART i/N` labels are always self-consistent (N matches the
number of chunks emitted), which is load-bearing because the turn-1 priming preamble (FR-T4,
delivered by S2) interpolates this same N ("I will send a git diff in N parts"). In practice
`len(chunks)` equals the formula N (the formula is the predictor; the walk is the truth). Using
`len(chunks)` is robust to the rare anchor-overshoot case where the two would diverge.

## 4. The algorithm (settled)

```go
func chunkPayload(payload string, chunkTokens int) []chunk {
    if chunkTokens < 1 { chunkTokens = 1 }          // defensive: non-positive budget → single chunk
    runesPerWindow := chunkTokens * 4                // tokens → rune-equivalent (EstimateTokens = runes/4)
    var bodies []string
    offset := 0                                       // byte offset into payload
    for offset < len(payload) {
        end := advanceRunes(payload, offset, runesPerWindow)   // ≤ runesPerWindow runes forward
        if i := strings.IndexByte(payload[end:], '\n'); i >= 0 {
            end += i + 1                              // anchor FORWARD: include the boundary line's '\n'
        } else {
            end = len(payload)                        // no further newline → take the remainder
        }
        bodies = append(bodies, payload[offset:end])
        offset = end
    }
    if len(bodies) == 0 { bodies = append(bodies, "") }   // empty payload → one empty chunk (N ≥ 1)
    total := len(bodies)
    chunks := make([]chunk, total)
    for i, body := range bodies {
        chunks[i] = chunk{
            index: i + 1, total: total,
            text:  fmt.Sprintf("PART %d/%d:\n%s", i+1, total, body),
        }
    }
    return chunks
}

// advanceRunes: byte offset after advancing n runes from start (clamped to len(s)). Rune-by-rune so
// multi-byte UTF-8 is never split. (Stops after n runes — does not scan the whole string.)
func advanceRunes(s string, start, n int) int {
    end := start
    for i := 0; i < n && end < len(s); i++ {
        _, size := utf8.DecodeRuneInString(s[end:])
        end += size
    }
    return end
}

func ceilDiv(n, d int) int { return (n + d - 1) / d }   // local copy; git.ceilDiv is unexported
```

**Progress guarantee (no infinite loop):** `runesPerWindow ≥ 4` (chunkTokens ≥ 1), so `advanceRunes`
advances ≥ 1 rune when `offset < len(payload)`; the anchor only moves `end` forward; thus
`end > offset` every iteration. ✓

**`payload[end:]` is O(1)** (Go string slicing shares the backing array — no copy/allocation).
`utf8.DecodeRuneInString(s[end:])` likewise operates on the slice header. ✓

## 5. Edge cases (all covered by the algorithm + the smoke test)

| Case | Behavior |
|---|---|
| Empty payload (`""`) | `len(bodies)==0` → single chunk, body `""`, `PART 1/1:\n`. N ≥ 1. |
| Small payload (≤ budget) | One window consumes all → single chunk with `PART 1/1` prefix (protocol uniformity). |
| Boundary lands mid-line | Anchor forward to the line's `\n` (inclusive) → line stays whole in the current chunk; next chunk starts clean. |
| Boundary lands exactly on `\n` | `IndexByte(payload[end:], '\n') == 0` → `end += 1` (the `\n` joins this chunk). |
| Single line longer than budget (no `\n`) | `IndexByte == -1` → `end = len(payload)` → whole line in one chunk (can't split without fracturing). |
| CJK / multi-byte content | `advanceRunes` decodes rune-by-rune → never splits a multi-byte sequence; `EstimateTokens` counts runes so CJK doesn't over-count. |
| `chunkTokens < 1` (defensive) | Clamped to 1 → `runesPerWindow=4` → single chunk (the helper is robust to bad input; the caller passes cfg.MultiTurnChunkTokens which is ≥1 in practice). |
| Round-trip | `strings.Join(bodies, "") == payload` (the chunks partition the payload exactly; the PART prefix is OUTSIDE each body). |

## 6. The `chunk` type + the PART prefix (FR-T3/FR-T4)

```go
type chunk struct {
    index int    // 1-based part number (the i in "PART i/N")
    total int    // N (len(chunks)); identical across all chunks in a result
    text  string // "PART i/N:\n<body>" — the prefix is OUTSIDE the body budget
}
```

- `text = fmt.Sprintf("PART %d/%d:\n%s", index, total, body)`. The `\n` after the colon separates
  the prefix from the body. The prefix is one short line, emitted OUTSIDE the body budget (FR-T3);
  its own tokens add on top — acceptable (it is one short line).
- `index`/`total` are also exposed as fields so S2 (the turn protocol) can render "PART i/N:" itself
  if needed (it doesn't need to re-parse `text`). The `text` field is the ready-to-send turn payload.

## 7. Why the smoke test belongs in S1 (and S4 extends it)

The plan assigns the EXHAUSTIVE unit tests to S4. But S1 shipping a verified helper (the T2.S1
`TestDefaults` precedent) is what lets S2 consume a known-correct `chunkPayload` and lets S4 build
the exhaustive matrix on a tested foundation. S1's smoke test covers the CORE contract only:

1. `TestChunkPayload_SingleChunk` — small payload → 1 chunk, `PART 1/1:\n` prefix present, body == payload.
2. `TestChunkPayload_MultiChunkSplit` — payload > budget → N chunks; `strings.Join(bodies,"") == payload`;
   every chunk has `PART i/N` with consistent N; indices 1..N.
3. `TestChunkPayload_NewlineAnchoring` — a boundary that lands mid-line → the line is NOT fractured
   (the chunk ends on a `\n`; the body is a whole number of lines).
4. `TestChunkPayload_EmptyPayload` — `""` → 1 chunk, body `""`, `PART 1/1:\n`.
5. `TestChunkPayload_PrefixOutsideBudget` — the `PART i/N:` line is outside the body budget (body
   alone ≈ ≤ chunkTokens tokens; the prefix is the line before the body).
6. `TestChunkPayload_RuneBasedCJK` — a CJK payload is sized by RUNES (a 4-rune CJK string with a tiny
   budget still chunks without splitting a multi-byte sequence; EstimateTokens counts it as 1 token).
7. `TestChunkPayload_CeilRounding` — N = ceil (a payload 1.5× the budget → 2 chunks, not 1).

S4 adds: boundary-exact, chunkTokens=1, very-long-line-no-newline, huge-payload scaling, the trigger
truth table (S3's gate), and the token_limit non-interaction test, + the how-it-works doc.

## 8. Decisions log (quick reference)

- **D1** window by `chunkTokens*4` RUNES (the rune-equivalent of the token budget); the contract's
  "windows of ≤ chunkTokens" is the TOKEN budget (FR-T3), realized as runes via EstimateTokens's /4.
- **D2** `total = len(chunks)` (actual count), not the pre-computed formula — guarantees `PART i/N`
  self-consistency, which the turn-1 preamble (S2) interpolates.
- **D3** local `ceilDiv` (git's is unexported); local `advanceRunes` (rune-by-rune, no whole-string scan).
- **D4** anchor FORWARD (include the boundary line's `\n` in the current chunk); no-fracture takes
  precedence over the ≤-budget target (the overshoot is ≤ one line).
- **D5** `chunkTokens < 1` clamped to 1 (defensive; the helper is pure string-math and must not panic).
- **D6** empty payload → single empty chunk (N ≥ 1; the preamble still references N=1).
- **D7** S1 ships a focused SMOKE test (the T2.S1 TestDefaults precedent); S4 EXTENDS multiturn_test.go
  with the exhaustive matrix + the gate/token_limit tests + the doc.
