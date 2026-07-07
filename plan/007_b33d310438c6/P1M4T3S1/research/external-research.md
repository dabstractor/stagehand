# P1.M4.T3.S1 — External Research (water-fill fairness + token estimation)

The truncation-application primitives (split/extract/truncate) are owned by the parallel sibling
(P1.M4.T2.S2); the solver is P1.M4.T2.S1. This task is the GATE that wires them into the 3 diff functions.
The relevant external concepts are the water-fill fairness algorithm and the chars/4 token estimate.

## 1. Max-min-fair allocation with caps ("water-filling") — the FR3i algorithm

FR3i is the classic **max-min-fair allocation with per-flow caps** (a.k.a. water-filling). Given N flows
with demands `size_i` and a total budget `B`, find the water level `L` such that
`Σ min(size_i, L) = B`: every demand ≤ L is satisfied in full; every demand > L is capped at L; the unused
share from small flows is reclaimed and redistributed to the large flows until the budget is exhausted.

Properties (FR3i a–d map directly):
- (a) Only flows that actually exceed L are trimmed, each by the minimal amount.
- (b) No budget is wasted — small flows' surplus is reclaimed.
- (c) The budget is fully utilized (Σ min(size_i, L) = B when Σ size_i > B).
- (d) No single flow monopolizes (all capped at L), yet a large flow still receives the bulk when budget
  allows — it is never penalized beyond the shared level.

The O(N log N) "sort-and-walk" realization (the sibling's `waterFillLevel`, P1.M4.T2.S1): sort demands
ascending; walk; at each step, if `size_i × remaining_count ≤ budget_left`, serve file i whole and subtract;
else the water level is `budget_left / remaining_count` (integer floor) shared by the rest. This is the
standard progressive-filling algorithm.

References:
- https://en.wikipedia.org/wiki/Max-min_fairness — "water-filling" / progressive filling.
- The solver is ALREADY implemented (`internal/git/waterfill.go` — `waterFillLevel` + `allocByWaterFill`).
  This task CONSUMES `allocByWaterFill`; it does not reimplement the algorithm.

**Why it matters for the gate:** the gate's job is to (1) compute the budget, (2) measure each file's body
size in the SAME unit the solver and the enforcement use (tokens, via `EstimateTokens`), (3) call
`allocByWaterFill` → per-file allotments, (4) hand the allotments to `truncateByWaterFill`. Coherence (same
unit + same body definition across size/enforce) is what makes guarantees (a)–(d) EXACT rather than
approximate — see design-decisions D2/D8.

## 2. The "~4 chars ≈ 1 token" estimate (model-agnostic budgeting)

Stagecoach never loads a tokenizer (it shells out to an arbitrary agent CLI — the whole product premise), so
FR3d/FR3i budget tokens with the standard heuristic: **≈4 characters per token**, ceiling-rounded, RUNE-based
(not byte-based, so CJK/emoji don't over-count). Implemented as `EstimateTokens(s) = ceil(runes/4)`
(`internal/git/tokens.go`).

- Reference (the heuristic's origin): OpenAI's coarse guidance that ~4 chars ≈ 1 token for English text
  (https://platform.openai.com/docs/guides/tokens — "a useful rule of thumb"). Stagecoach uses it as a
  model-agnostic CONSERVATIVE estimate; the user sets `token_limit` with their own slack below the real
  context window, and a safety `margin` absorbs the code-vs-prose density gap.
- The contract PINS chars/4 (NOT the architecture doc's chars/3 ceiling-recommendation); the chars/3 gap is
  reconciled by the separate `margin`, not by changing the formula (tokens.go doc; design-decisions §2 of
  P1.M4.T1.S1). DO NOT "improve" the estimator — the budget arithmetic requires a SINGLE consistent measure.

**Why it matters for the gate:** the budget, the skeleton cost, the prompt reserve, the per-file body sizes,
AND the truncation enforcement ALL use `EstimateTokens` ⇒ the budget arithmetic is in one consistent unit.
The gate computes `body_budget = token_limit − EstimateTokens(skeleton) − promptReserve − margin` and sizes
each body with `EstimateTokens(body)`; the enforcement (`truncateByWaterFill`) truncates at
`EstimateTokens(body) > allotment` using the SAME function. Same unit end-to-end ⇒ coherent fairness.

## 3. The two sentinels (DO NOT confuse them)

system_context.md §6 invariant 2 pins TWO DISTINCT truncation sentinels:
- **Legacy (`token_limit == 0`):** `... [diff truncated at %d bytes]` (non-md aggregate) and
  `... [diff truncated at %d lines]` (md per-file) — the N-bearing aggregate sentinels (git.go:840/868/1302/
  1326/1458/1482). These stay byte-identical on the `==0` path (the regression anchor).
- **Water-fill (`token_limit > 0`):** the SHORTER `... [truncated]` form (per truncated FILE — the sibling's
  `truncatedSentinel` const in `truncatediff.go`). The gate's `>0` path MUST NOT emit the legacy N-bearing
  forms; it emits the per-file short form via `truncateByWaterFill` (which appends `truncatedSentinel`).

The gate never constructs a sentinel string itself — `==0` keeps the existing `fmt.Sprintf("\n... [diff
truncated at %d bytes/lines]", …)` calls untouched; `>0` delegates to `truncateByWaterFill` (the sentinel is
the sibling's concern). This division keeps the two forms physically separated by the branch.
