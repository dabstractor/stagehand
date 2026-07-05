# Research: mtPayload Rebuild Fix (Issue 4) — P1.M1.T2.S1

> **Purpose:** Pin the exact, source-verified one-line fix for the TokenLimit==0 mtPayload inconsistency
> (Issue 4), checked against the live codebase on 2026-07-05. Baseline `go test ./internal/generate/` is
> GREEN (4.140s). The bug is at `generate.go:311` (`mtPayload := payload`); the fix rebuilds from the
> untruncated `diff` variable.

---

## 1. Environment & Baseline

| Item | Value / Evidence |
|---|---|
| Module | `github.com/dustin/stagehand`, `go 1.22` |
| Edit targets | `internal/generate/generate.go` (line 311 + comment at ~307), `internal/generate/multiturn_test.go` (new test) |
| Bug line | generate.go:311 `mtPayload := payload` (confirmed verbatim) |
| Baseline | `go test ./internal/generate/` → **ok (4.140s)** |
| Prior PRP (S1) | ChunkCount wrapper in `internal/generate/multiturn.go`. Does NOT touch `generate.go` → **no conflict**. |

---

## 2. The Bug (Issue 4)

The CommitStaged multi-turn gate at generate.go:290-374. The `payload` variable is hoisted (L227) and
rebuilt each loop iteration: `payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)` (L231), then
conditionally prepended with retryInstr when parseFail: `payload = retryInstr + "\n\n" + payload` (L233).

The multi-turn trigger fires when one-shot is exhausted (`!success`). The LAST one-shot attempt's `payload`
survives the loop (hoisted). When the last attempt failed parsing (parseFail==true), `payload` carries the
retryInstr preamble.

At the gate:
- **TokenLimit==0 path (L311):** `mtPayload := payload` → INCLUDES retryInstr (if parseFail).
- **TokenLimit≠0 path (L323):** `mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)` → NO retryInstr (rebuilt fresh).

**Inconsistency:** TokenLimit==0 includes retryInstr; TokenLimit≠0 does not. The retryInstr preamble
("Output ONLY the commit message. No preamble, no markdown, no quotes.") is one-shot-only; multi-turn has
its own priming preamble (FR-T4). Including it in mtPayload sends a confusing double instruction to the
model and misrepresents the diff payload.

---

## 3. The Fix (one line)

Replace generate.go:311:
```go
mtPayload := payload
```
with:
```go
mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
```

This rebuilds from the existing untruncated `diff` variable (captured at L193-198, already in scope at the
gate) WITHOUT retryInstr. When TokenLimit==0, `diff` is already untruncated (the one-shot StagedDiff used
TokenLimit=cfg.TokenLimit=0). When TokenLimit≠0, the existing re-capture branch (L312-326) overwrites
mtPayload with the re-captured fullDiff version. So both paths produce a retryInstr-free mtPayload derived
from the untruncated diff.

Also update the comment at L307 to reflect that mtPayload is ALWAYS rebuilt from diff (not just in the
TokenLimit≠0 branch).

**Variables confirmed in scope at L311:** `diff` (string, L193), `cfg.Context` (string), `rejected`
([]string). `prompt.BuildUserPayload(diff, context string, rejected []string) string` (payload.go:97).

---

## 4. The Test (mirror TestMultiTurnGate_TokenLimitTruncated_Recaptures at multiturn_test.go:618)

The existing test `TestMultiTurnGate_TokenLimitTruncated_Recaptures` (at :618, NOT :418 as the contract's
stale line number said — S1's ChunkCount tests shifted it) exercises the TokenLimit≠0 re-capture path. The
new test mirrors it for the TokenLimit==0 + parseFail case:

**Setup:**
- `cfg.TokenLimit = 0` (the bug's trigger — the re-capture branch is NOT taken).
- Force a one-shot parse failure: the stub script's call 1 returns `""` (empty → ParseOutput ok=false →
  parseFail → retryInstr prepended to `payload`).
- Large diff (exceeds MultiTurnChunkTokens → condition (b) true).
- `cfg.MultiTurnChunkTokens` small (e.g. 4) so the untruncated diff clearly exceeds one chunk.
- `cfg.MaxDuplicateRetries = 0` (one-shot: exactly 1 attempt, then exhausted → multi-turn fires).
- `stubAppendManifest(t, bin, []string{"", "ok", ..., "feat: mt win"}, false)` — SessionMode="append";
  call 1 = "" (parseFail), calls 2..N = "ok", final call = a valid message.

**Assertions:**
1. Multi-turn fires + succeeds (commit lands, err==nil) — mirrors the existing test.
2. The mtPayload delivered to the multi-turn protocol does NOT contain the retryInstr preamble. The
   retryInstr-specific substring is `"No preamble, no markdown, no quotes."` (NOT the ambiguous "Output
   ONLY" which also appears in the multi-turn final-turn prompt). Observe via `STAGEHAND_STUB_STDINFILE`
   (the stub writes received stdin to that file) OR via the verbose turn-count (if the retryInstr inflated
   the payload, the chunk count would differ). The implementing agent should verify the observation path
   works (the STDINFILE captures the last turn's stdin; the first chunk's content may need a per-turn
   capture mechanism or the verbose turn-count proxy).

---

## 5. Decisions Log

| # | Question | Decision | Rationale |
|---|---|---|---|
| D1 | The fix? | Replace `mtPayload := payload` with `mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)`. | Rebuilds from the untruncated diff WITHOUT retryInstr, matching the TokenLimit≠0 path (L323). `diff` is already in scope. |
| D2 | Comment update? | YES — update L307 to reflect mtPayload is ALWAYS rebuilt from diff (not just TokenLimit≠0). | The current comment says "When token_limit is unset, `payload` is already untruncated... so we use it directly" — this is now wrong (we rebuild from `diff`, not reuse `payload`). |
| D3 | Test assertion substring? | `"No preamble, no markdown, no quotes."` (the retryInstr-specific tail). | NOT "Output ONLY the commit message" — that phrase ALSO appears in the multi-turn final-turn prompt ("Output ONLY the message"), causing a false positive. |
| D4 | Test observation mechanism? | STAGEHAND_STUB_STDINFILE (preferred) or verbose turn-count (proxy). | Direct mtPayload inspection isn't possible (it's a local in CommitStaged). The STDINFILE captures stdin per turn (last turn survives); the implementing agent verifies the path. |
| D5 | Scope? | ONLY generate.go (L311 + comment) + multiturn_test.go (new test). | The corrected gate is the copy source for P1.M2.T1.S2 (runPipeline) and P1.M3.T1.S2 (hook). This task is the reference gate fix only. |
