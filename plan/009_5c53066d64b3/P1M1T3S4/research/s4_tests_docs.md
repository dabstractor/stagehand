# P1.M1.T3.S4 Research — Multi-turn unit tests (chunk math, truth table, FR-T12) + how-it-works docs

## §1. Scope & non-overlap with the parallel S3

S3 (Implementing in parallel) edits `internal/generate/generate.go` (the inline FR-T1 gate) + `internal/generate/generate_test.go` (3–4 focused CommitStaged multi-turn tests). S4 ADDS the **exhaustive unit tests** to `internal/generate/multiturn_test.go` (S1/S2's file — NOT touched by S3) and the **Mode A docs** to `docs/how-it-works.md`.

**File-conflict avoidance:** putting ALL of S4's tests in `multiturn_test.go` (not `generate_test.go`) means S3 and S4 never touch the same file. `multiturn_test.go` is `package generate`, so it CAN call `CommitStaged` (the gate truth table drives it) and reuse `generate_test.go`'s helpers (`initRepo`/`writeFile`/`stageFile`/`commitRaw`/`headSHA`/`gitOut`) — same package, in scope, no redeclaration.

**Test-overlap delineation:** S3's focused tests assert rescue + HEAD/index idempotency for 2 skip cases (non-append, small-payload). S4's truth table is the **exhaustive 4-condition table** using a **different, stronger assertion**: the `VerboseWarn("one-shot exhausted → multi-turn fallback")` trigger line is **ABSENT** from the verbose buffer (proving the gate predicate did not pass — i.e. `Run` was never entered). Different file, different function names, different assertion strategy ⇒ complementary, not duplicative.

## §2. Verified chunkPayload exact-N (empirically pinned — CRITICAL for correct assertions)

`chunkPayload`'s forward-newline anchor means **N ≠ ceil(payload_tokens / chunk_tokens)** in general: each chunk's window advances `runesPerWindow = chunkTokens*4` runes, then anchors FORWARD to include the remainder of the line the window-end fell into. So a chunk is `runesPerWindow` runes + up-to-one-line overshoot. Consequences (verified against a verbatim replica on git 2.x / Go):

- A **no-newline payload collapses to ONE chunk** regardless of size (`strings.IndexByte(payload[end:], '\n')` finds nothing ⇒ `end = len(payload)`). So N>1 REQUIRES interior newlines.
- The "+1 token" boundary does NOT cleanly flip N from 1→2: a small overage is absorbed into chunk 1's forward anchor (e.g. `'abc\n'×5`, CT=4, ET=5=CT+1 → **N=1**, not 2). The item's literal "chunkTokens+1 → N=2" is **not achievable** for arbitrary payloads; it requires a payload family where the ceil relationship survives anchoring.

**VERIFIED clean payloads (use these EXACT values in the table test):**

| payload | chunkTokens (CT) | runes | EstimateTokens (ET) | runesPerWindow | **N (verified)** | relationship |
|---|---|---|---|---|---|---|
| `"abcd\n"×4`  | 5 | 20  | 5  | 20 | **1** | ET = CT (one chunk) |
| `"abcd\n"×8`  | 5 | 40  | 10 | 20 | **2** | ET = 2×CT |
| `"abcd\n"×12` | 5 | 60  | 15 | 20 | **3** | ET = 3×CT |
| `"ab\n"×33`   | 10| 99  | 25 | 40 | **3** | ET = 2.5×CT → N=3 (ceil) |
| `"aaaaaa\nbbbbbb\ncccccc\n"` | 1 | 18 | 5 | 4 | **3** | each 6-rune line ⇒ own chunk |

**Why the `'abcd\n'`/CT=5 family is clean:** line length (5 runes) divides `runesPerWindow` (20), so a full window lands exactly on a line boundary and the forward anchor's overshoot is bounded; the exact multiples (1×,2×,3× CT) yield N=1,2,3 deterministically.

**The 2.5×→3 case** (`'ab\n'×33`, CT=10): ET=25=2.5×CT, and anchoring does not collapse it below 3 ⇒ N=3. This is the item's "2.5×chunkTokens → N=3 (ceil)" realized with a verified payload.

**Monotonicity (assert as a property):** for a fixed CT, N is non-decreasing as the payload grows (more content never reduces the chunk count). Verified across the `'abcd\n'` family (×4→1, ×8→2, ×12→3).

> ⚠️ The implementing agent MUST use these EXACT payloads/assertions. Inventing "chunkTokens+1 → N=2" with an arbitrary payload will FAIL (anchoring absorbs the overage). This is the #1 one-pass failure risk for test (a).

## §3. Newline-anchoring property (test b) — verified

Payload `"aaaaaa\nbbbbbb\ncccccc\n"` (6-rune lines), CT=1 (window=4): the 4-rune window lands mid-line ("aaaa"), anchors forward to the line's `\n`. Verified output: 3 chunks, **every** non-last body ends on `\n` (no fractured line), and the round-trip (concatenated bodies) equals the original payload byte-for-byte. The test asserts BOTH (every boundary on `\n`) AND (lossless round-trip).

## §4. PART i/N prefix (test c)

`chunkPayload` stamps `fmt.Sprintf("PART %d/%d:\n%s", i+1, total, body)`. So every `chunk.text`:
- starts with exactly `"PART i/N:\n"` (i 1-based, N = total),
- i is monotonic 1..N,
- the body (after the first `\n`) is the chunk content.

The existing `TestChunkPayload_MultiChunkSplit` checks index/total *consistency*; S4's test asserts the **exact prefix string** + **strict monotonicity** + the body/prefix boundary.

## §5. Trigger-gate truth table (test d) — strategy

The FR-T1 gate is **inline in `CommitStaged`** (generate.go:297, S3 landed). There is NO standalone gate-predicate function, so the truth table drives `CommitStaged` (the gate's sole call site) — an "integration-style unit test," explicitly allowed by the item ("assert it was NOT called").

**The "was NOT called" assertion (clean + uniform across all rows):** the gate emits `deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")` ONLY when it passes (generate.go:311). `VerboseWarn` writes to the `*ui.Verbose` sink. So:
- Build `Deps{..., Verbose: ui.NewVerbose(&buf, true)}` (the existing pattern, generate_test.go:647).
- After `CommitStaged`, assert `!strings.Contains(buf.String(), "multi-turn fallback")` ⇒ the gate did NOT fire ⇒ `Run` was NOT entered.
- (Corroborating: the result is `*RescueError{Kind: ErrRescue}` for the !success rows; HEAD/index idempotent.)

**The 4 rows (each flips ONE FR-T1 condition to false):**

| Row | condition flipped false | how | one-shot outcome | assertion |
|---|---|---|---|---|
| 1 | (c) `MultiTurnFallback` | `cfg.MultiTurnFallback = false` | exhausts (script `[""]`) | rescue + NO "multi-turn fallback" in verbose |
| 2 | (b) payload > chunk | default `MultiTurnChunkTokens=32000` (tiny diff ⇒ ET ≤ 32000) | exhausts (script `[""]`) | rescue + NO trigger |
| 3 | (d) `session_mode="append"` | `SessionMode` unset (⇒ `""`) | exhausts (script `[""]`) | rescue + NO trigger |
| 4 | (a) one-shot exhausted | script `["feat: win"]` (one-shot SUCCEEDS) | success (loop breaks) | commit lands; gate not reached (success path) — NO trigger, NO rescue |

**Positive control (row 5, optional but recommended):** ALL conditions true (small `MultiTurnChunkTokens`, `SessionMode="append"`, `MultiTurnFallback=true`, payload > chunk, one-shot exhausts via `[""]`, multi-turn final turn `"...msg"`) ⇒ verbose buffer **CONTAINS** "multi-turn fallback" ⇒ the gate FIRED. (Complementary to S3's success-commits test; S4 asserts the *trigger* fired, not the full commit, to keep the truth table self-contained.)

> Row 4 is the "gate not reached" row: it proves the gate is gated on `!success` (condition a). Row 5 is the only row where the trigger IS present (the control). Rows 1–3 are the three skip cases.

## §6. token_limit non-interaction (test e) — the FR-T12 tension

**THE tension (flagged, not silently papered over):**
- PRD §9.24 **FR-T12**: "multi-turn uses the UNTRUNCATED payload" — multi-turn deliberately ignores `token_limit`.
- S3's **D2** (landed, generate.go): the gate passes the **captured** `payload` variable, which was built from `diff = StagedDiff(..., TokenLimit: cfg.TokenLimit, ...)`. So if `TokenLimit` truncated the diff, **multi-turn receives the truncated payload** — NOT the full diff.

This means S3's implementation does **not** fully realize FR-T12's "untruncated" intent in the diff-exceeds-TokenLimit case. S4 **cannot** fix this (it would require re-capturing the diff with `TokenLimit=0` in `generate.go` — S3's territory, and S3's contract forbids the recompute). 

**What S4 CAN honestly test (and does):**
1. **Pure-helper level (unconditionally true):** `chunkPayload`'s signature is `(payload string, chunkTokens int)` — `cfg.TokenLimit` is **structurally absent**. So multi-turn's chunk COUNT is a function of (payload, chunkTokens) only; no TokenLimit value can change it. Test: a payload yields deterministic N; re-invoking is stable; and (by API inspection) TokenLimit is never threaded into `Run`/`chunkPayload`.
2. **Gate-predicate level:** the gate expression (generate.go:297) reads only `(cfg.MultiTurnFallback, git.EstimateTokens(payload), cfg.MultiTurnChunkTokens, resolved.SessionMode)` — **TokenLimit is not a term**. So the gate's decision is TokenLimit-independent.
3. **Behavioral (CommitStaged, diff fits within TokenLimit):** when `cfg.TokenLimit` is set but the test diff is SMALLER than it (so StagedDiff does not truncate), multi-turn still triggers (cond b via small MultiTurnChunkTokens) and chunks the FULL payload. Assert: verbose contains the trigger + the turn count reflects the full payload. This is the closest end-to-end realization of FR-T12 that PASSES against S3.

**Documented gap (not tested as "passes"):** the diff-exceeds-TokenLimit case would, under S3, hand multi-turn a truncated payload — diverging from FR-T12's literal "untruncated." S4 records this in `how-it-works.md` as a known limitation of the v2.3 multi-turn path and flags it for a future fix (re-capture untruncated in the multi-turn branch). Do NOT write a test asserting the full-diff chunk count in that case — it would FAIL against S3 and is integration territory (P1.M1.T4).

## §7. Docs (Mode A) — how-it-works.md placement

`docs/how-it-works.md` EXISTS with sections: snapshot flow (5), stage-while-generating (28), decomposition (47), diff capture (132), safety/rescue (162), prompt engineering (221), hook mode (262). There is **no standalone "Generation" section** — the generation path spans snapshot-flow + prompt-engineering. The item says "under the generation section"; the logical insertion is a new `## Multi-turn generation fallback` section **immediately before `## Hook mode vs the snapshot-based flow` (line 262)** — i.e. after prompt engineering, grouping it with generation-path content. Concise (~25–40 lines), cross-links PRD §9.24, covers: the 4 FR-T1 triggers; lossless chunking (NOT lossy map-reduce); the N+1 turn protocol; final turn reuses parse+dedupe; failure → standard rescue (snapshot safe); token_limit does not apply (FR-T12, with the S3 caveat noted honestly).

## §8. Reused helpers / seams (no new helpers, no new imports beyond what multiturn_test.go already has)

- `initRepo`/`writeFile`/`stageFile`/`commitRaw`/`headSHA`/`gitOut` — generate_test.go (same package, in scope).
- `stubtest.Build`/`stubtest.NewScript`/`stubtest.Manifest` — already imported in multiturn_test.go.
- `config.Defaults()` — already imported.
- `ui.NewVerbose(&buf, true)` — NEW import `internal/ui` + `bytes` (multiturn_test.go does NOT currently import these; ADD them).
- `errors` (for `errors.As` into `*RescueError`) — NEW import (multiturn_test.go has `strings`, `testing`, etc.; check the current import block — `context`, `strings`, `testing`, `unicode/utf8`, `config`, `git`, `provider`, `stubtest`). ADD: `bytes`, `errors`, `ui`.
- `chunk`/`chunkPayload`/`Run` — same package (no import).
- `CommitStaged`/`Deps`/`RescueError`/`ErrRescue` — same package (no import).
