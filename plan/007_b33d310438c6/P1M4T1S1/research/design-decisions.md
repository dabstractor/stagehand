# P1.M4.T1.S1 — Design Decisions & Research Notes

> Research backing `PRP.md`: a single model-agnostic token estimator (`chars/4`, rune-based, ceiling)
> consumed by S2 (prompt-reserve measurement) and M4.T2 (water-fill sizing). Pure utility, no deps.

## 0. Placement: NEW `internal/git/tokens.go`, EXPORTED `EstimateTokens` (+`EstimateTokensBytes`)

The item says "in internal/git (or a small internal/util)". There is NO `internal/util` package today, and
creating one for a single 2-line function is over-engineering. The PRIMARY consumer is the diff pipeline
(internal/git) for M4.T2's water-fill; the SECONDARY consumer is S2 (prompt-reserve measurement at the 6
call sites). Verified reachability: every S2 call-site package (generate/generate.go, decompose/*,
pkg/stagecoach, cmd/*) ALREADY imports `internal/git` (grep confirmed) — so an EXPORTED `EstimateTokens` in
`internal/git` is reachable by all consumers with NO new import edge and NO cycle. `internal/prompt` does
NOT import `internal/git` (grep confirmed) and is not a consumer (the prompt-reserve measurement happens at
the orchestrator/call-site layer, not in the prompt builders). So: NEW `internal/git/tokens.go`, `package
git`, EXPORTED `EstimateTokens(s string) int` + `EstimateTokensBytes(b []byte) int`. Mirror the precedent of
`numstat.go` / `binary.go` (pure-helper files in this package).

The item writes the name lowercase (`estimateTokens`) as pseudocode, but S2's measurement is CROSS-PACKAGE
(generate/decompose setting `StagedDiffOptions.PromptReserveTokens` — confirmed the field exists at
git.go:72, "the git layer RECEIVES this; it does not compute it"). So the function MUST be exported:
`EstimateTokens`.

## 1. The formula: `ceil(runeCount / 4)`, rune-based (utf8.RuneCount), ceiling division

Contract (item_description §3): `len([]rune(s)) / 4` rounded UP, rune count (not byte count). Concretely:
`ceilDiv(utf8.RuneCountInString(s), 4)` where `ceilDiv(n, d) = (n + d - 1) / d`. `utf8.RuneCountInString` is
the allocation-free stdlib equivalent of `len([]rune(s))` (the item's literal) — same count, no rune-slice
allocation (better for large diffs). `EstimateTokensBytes` uses `utf8.RuneCount(b)`.

Ceiling division `(n+3)/4` gives the right answers across the contract's table:
- n=0 → (0+3)/4 = 0 (empty ⇒ 0) ✓
- n=4 → (4+3)/4 = 1 (4 ASCII ⇒ 1) ✓
- n=8 → (8+3)/4 = 2 (8 ⇒ 2) ✓
- n=4 runes (a 4-rune CJK string = 12 bytes) → ceilDiv(4,4) = 1 (rune-based ⇒ 1, NOT byte-based 3) ✓

The `(n+d-1)/d` form naturally yields 0 for n=0 — no special-case needed. (Equivalent alternative
`if n==0 {return 0}; return (n-1)/d+1` — same results; the `(n+3)/4` form is one expression.)

## 2. chars/4 (contract) vs chars/3 (architecture doc §5) — the margin reconciles, NOT the estimator

**The tension:** `architecture/git_diff_semantics.md §5` says `tokens ≈ chars/4` is the standard
model-agnostic heuristic, BUT for a budget CEILING on code-heavy diffs it RECOMMENDS `chars/3` (code is
~3 chars/token, so `chars/3` estimates MORE tokens and thus truncates sooner — the safe direction). The
item_description and PRD §9.1 **FR3d** ("the ~4 chars ≈ 1 token estimate with a safety margin") specify
**chars/4**.

**Resolution (do NOT change the formula):** implement **chars/4** — it is the contract and matches FR3d's
stated "~4 chars ≈ 1 token estimate." The architecture doc's chars/3 ceiling-recommendation is reconciled by
the SEPARATE **`margin`** in FR3d/FR3i (`body_budget = token_limit − skeleton − prompt − margin`): the
margin is the actual safety buffer that absorbs the chars/4-vs-chars/3 gap (code is ~33% denser than chars/4
predicts). The estimator's job is to be a CONSISTENT, model-agnostic measure; the `margin` (sized in M4.T2)
is where the safety lives. **This subtask delivers ONLY the estimator (chars/4); do not "improve" it to
chars/3** — that would diverge from the contract and desynchronize S2/M4.T2, which both consume THIS formula
(see §4). Implement chars/4, full stop.

(The item's parenthetical "chars/4 errs toward under-spending the budget" is best read as: the FR3d/FR3i
margin is the safety mechanism; chars/4 is the standard estimate. Do not litigate the direction here — the
formula is pinned by the contract.)

## 3. Rune-based, not byte-based (the UTF-8 correctness call)

Byte-based (`len(s)/4`) would OVER-COUNT multi-byte UTF-8 (a 4-rune CJK string is 12 bytes → byte-based
says 3 tokens; rune-based says 1). Diffs are ASCII-dominated, but commit messages, paths, and doc strings
can carry UTF-8; the rune count is the correct model-agnostic unit (a tokenizer counts code points /
grapheme-ish units far more than raw bytes). The contract's test case "a 4-rune CJK string ⇒ 1 (rune-based)"
PINS this. Use `utf8.RuneCountInString` / `utf8.RuneCount`.

## 4. The SINGLE estimator — no second formula

The item is explicit: "This is the SINGLE estimator used by S2 (prompt reserve) and M4.T2 (numstat sizing)
— do not introduce a second formula." S2 (P1.M4.T1.S2) will call `EstimateTokens(promptString)` to set
`StagedDiffOptions.PromptReserveTokens` at the 6 call sites; M4.T2.S1/S2 will call it for water-fill sizing
(per-file body token estimates) and the level-solver arithmetic. Both MUST use the same `ceil(runes/4)` so
the budget arithmetic is internally consistent (a token measured one place equals a token measured another).
Do NOT add a `chars/3` variant, a line-based variant, or a per-provider tokenizer hook in this (or any)
subtask — the model-agnostic design (N2: stagecoach never loads a tokenizer) depends on ONE formula.

## 5. Tests: NEW `internal/git/tokens_test.go`, table-test mirroring `TestIsBinaryByExtension`

The internal/git test convention is ONE FILE PER CONCERN (`binary_test.go`, `numstat_test.go`, …) → the new
file is `tokens_test.go`, `package git` (white-box). Mirror `binary_test.go`'s `TestIsBinaryByExtension`
table style: `cases := []struct{ name; in; want }{…}` + a loop with one `t.Errorf` per case. Cover the
contract's table PLUS edge cases:
- `""` → 0
- 4 ASCII chars ("abcd") → 1
- 8 ASCII chars → 2
- 1 rune → 1 (ceiling: any non-empty string is ≥1 token)
- 5 ASCII chars → 2 (ceiling rounds up: ceilDiv(5,4)=2)
- 4-rune CJK string ("你好世界" — 12 bytes, 4 runes) → 1 (rune-based, NOT byte-based 3) — THE UTF-8 pin
- `EstimateTokensBytes` parity: a []byte and its string form give the same count
- a long string (e.g. 4000 runes) → 1000 (sanity, no overflow at int range)

Pure unit tests — no git repo, no I/O, no fixtures. Fast.

## 6. No conflict with the parallel M3.T1.S2 (skeleton.go) or any sibling

The parallel P1.M3.T1.S2 creates `internal/git/skeleton.go` (numstat skeleton render/prepend) and edits
`git.go`'s 3 diff functions + the golden tests. It does NOT create `tokens.go` or `tokens_test.go`. No
overlap. ✓ This subtask touches EXACTLY 2 NEW files: `internal/git/tokens.go` + `internal/git/tokens_test.go`.
It edits NOTHING (no git.go, no existing test). M4.T2 (the consumer) runs LATER (sequential). go.mod/go.sum
unchanged (stdlib `unicode/utf8` only).

## Sources
- `plan/007_b33d310438c6/architecture/git_diff_semantics.md §5` — the chars/4 heuristic + the chars/3
  ceiling caveat (§2 resolves the tension via the FR3d/FR3i margin).
- PRD §9.1 **FR3d** ("~4 chars ≈ 1 token estimate with a safety margin") + **FR3i** (`body_budget =
  token_limit − skeleton − prompt − margin`) — the contract formula + the margin that is the real safety.
- `internal/git/git.go:72` — `StagedDiffOptions.PromptReserveTokens` (M1.T2 done); "the git layer RECEIVES
  this; it does not compute it" ⇒ S2 measures it at cross-package call sites ⇒ EstimateTokens is EXPORTED.
- `internal/git/binary.go` + `binary_test.go` (`TestIsBinaryByExtension`) — the pure-helper-file + table-test
  pattern to mirror.
- import-graph grep — every S2 call-site package already imports internal/git; internal/prompt does not.
