# Bug Fix Requirements

## Overview

Comprehensive end-to-end validation of the multi-turn generation fallback feature (PRD §9.24, FR-T1–FR-T12, plan P1.M1) against the original PRD. The testing covered: provider surface (SessionMode field, RenderMultiTurn, pi builtin value), config surface (multi_turn_fallback / multi_turn_chunk_tokens), the generate core (chunkPayload, Run protocol, trigger gate wiring into CommitStaged), the integration tests (happy path, render contract, failure paths), documentation (README, how-it-works, configuration, providers), and the FR-T12 token_limit non-interaction.

The core multi-turn implementation in `internal/generate/generate.go::CommitStaged` is **solid and correct**: the trigger gate (FR-T1 conditions a–d), the N+1 turn protocol (FR-T4), the lossless chunking (FR-T2/FR-T3), the session lifecycle (FR-T6), the failure→rescue path (FR-T7), and the token_limit non-interaction (FR-T12) all match the PRD specification. The unit and integration tests are thorough.

However, the multi-turn fallback was **only wired into `CommitStaged`** (`internal/generate/generate.go`) — one of **three** duplicated generation-loop implementations in the codebase. The other two — the `--dry-run` path (`runPipeline` in `pkg/stagecoach/stagecoach.go`) and the git hook path (`internal/hook/exec.go`) — were not updated, creating a functional gap where multi-turn is silently absent on those paths. The `--dry-run` gap is a Major issue because it directly contradicts FR49's "run the full pipeline" requirement.

---

## Critical Issues (Must Fix)

None.

---

## Major Issues (Should Fix)

### Issue 1: `--dry-run` path does not get the multi-turn fallback

**Severity**: Major

**PRD Reference**: §9.12 FR49 ("`--dry-run` — run the full diff→snapshot→generate→parse→duplicate-check pipeline"); §9.24 FR-T1/FR-T10 (multi-turn trigger gate).

**Expected Behavior**: When `stagecoach --dry-run` is run on a large diff whose one-shot generation exhausts (empty/unparseable output after all retries), the multi-turn fallback should activate identically to the commit path — losslessly re-delivering the full diff across N+1 session turns to produce a single commit message. FR49 explicitly requires `--dry-run` to "run the full ... pipeline," and multi-turn is now part of that pipeline.

**Actual Behavior**: The `--dry-run` path routes through `runPipeline` (`pkg/stagecoach/stagecoach.go:415`), NOT through `CommitStaged`. `runPipeline` has its **own** generation loop (line 489) that is a pre-multi-turn copy — it has **zero** references to `MultiTurnFallback`, `multiturn.Run`, or any multi-turn logic. When the one-shot loop exhausts on a large diff, `runPipeline` directly returns a `*RescueError` (line 543–547) with no multi-turn gate in between. The user sees:

```
could not generate a commit message; run without --dry-run to see retries and the recovery recipe
```

(exit 1) — even though running `stagecoach` **without** `--dry-run` on the **same** diff would succeed via multi-turn.

**Steps to Reproduce**:

1. Configure a pi provider with `session_mode = "append"` and a model (e.g., `zai/glm-5.2`).
2. Stage a large diff (e.g., a file with ~200+ lines of changes, exceeding `multi_turn_chunk_tokens`).
3. Run `stagecoach --dry-run` → observe rescue error (exit 1).
4. Run `stagecoach` (without `--dry-run`) → multi-turn fires and succeeds (exit 0, commit lands).

The inconsistency: `--dry-run` fails where the commit path succeeds, on the same diff.

**Root Cause**: Three generation loops exist in the codebase (confirmed via `grep -rn "for attempt := 0; attempt <= cfg.MaxDuplicateRetries"`):

| Path | File | Multi-turn? |
|---|---|---|
| `CommitStaged` (commit path) | `internal/generate/generate.go:229` | ✅ Yes (gate at line ~300) |
| `runPipeline` (dry-run / SystemExtra) | `pkg/stagecoach/stagecoach.go:489` | ❌ No |
| hook exec (git hook mode) | `internal/hook/exec.go:157` | ❌ No |

The routing decision in `GenerateCommit` (`pkg/stagecoach/stagecoach.go:147`):
```go
if !opts.DryRun && opts.SystemExtra == "" {
    res, gerr := generate.CommitStaged(ctx, deps, cfg)  // HAS multi-turn
    ...
}
return runPipeline(ctx, deps, cfg, opts.SystemExtra, opts.DryRun)  // NO multi-turn
```

When `DryRun=true`, the code always takes the `runPipeline` branch, which lacks the multi-turn gate.

**Suggested Fix**: Propagate the multi-turn trigger gate into `runPipeline` identically to how it was wired into `CommitStaged` (P1.M1.T3.S3). Specifically:
1. Hoist `var payload string` before the loop in `runPipeline` (currently loop-scoped with `:=` at line 491, same pre-fix pattern CommitStaged had).
2. Insert the FR-T1 trigger gate between the `if !success {` block and the `return &RescueError` — mirroring `CommitStaged` lines ~300–340: check `cfg.MultiTurnFallback && *resolved.SessionMode == "append"`, do the FR-T12 re-capture if `cfg.TokenLimit != 0`, check `EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens`, call `multiturn.Run`, run dedupe on the result.
3. In dry-run mode, a multi-turn success should return early (before CommitTree) with the message, just like the current dry-run success path does. The multi-turn progress line (`fmt.Fprintf(os.Stderr, ...)`) works identically in dry-run.

The cleanest long-term fix is to **eliminate the code duplication** — extract the generation+dedupe+multi-turn loop into a single shared function that both `CommitStaged` and `runPipeline` call (with a dry-run flag), so multi-turn and future generation features land in one place. But the immediate fix is to copy the multi-turn gate into `runPipeline`.

---

## Minor Issues (Nice to Fix)

### Issue 2: Git hook exec path does not get the multi-turn fallback

**Severity**: Minor (arguably within FR-T10 scope, but affects users)

**PRD Reference**: §9.24 FR-T10 ("Multi-turn serves the message role (the single-commit path, §13.1–§13.5)"); §9.20 FR-H4 ("run the standard pipeline — ... message-role generation ...").

**Expected Behavior**: When a user has the `prepare-commit-msg` hook installed and runs `git commit` (from terminal, VS Code, or JetBrains) on a large diff, the hook's message-role generation should benefit from multi-turn, just as `stagecoach` (the plumbing path) does.

**Actual Behavior**: The hook exec path (`internal/hook/exec.go:157`) has its own generation loop with **zero** multi-turn references. On a large diff, the one-shot loop exhausts, the hook prints a warning and exits 0 (FR-H5 never-block), and the user gets an empty editor — the multi-turn fallback never fires. The user would need to run `stagecoach` directly (not `git commit`) to get multi-turn.

**Steps to Reproduce**:
1. `stagecoach hook install` with pi as the provider.
2. Stage a large diff (exceeding `multi_turn_chunk_tokens`).
3. `git commit` → empty editor (no generated message); one-shot exhausted, no multi-turn.
4. `stagecoach` (plumbing path) → multi-turn fires, message generated.

**Note**: FR-T10 explicitly scopes multi-turn to "the single-commit path (§13.1–§13.5)", and hook mode is §9.20 (a different path). So this is arguably by design. However, the practical inconsistency (hook mode silently lacks a feature the plumbing path has) could surprise users who rely on hook mode for IDE integration. If this is intentional, it should be documented; if not, the multi-turn gate should be propagated to the hook exec loop.

**Suggested Fix**: Either (a) propagate the multi-turn gate into the hook exec loop (mirroring the CommitStaged gate, with the FR-H5 never-block contract preserved — multi-turn failure still exits 0), or (b) document explicitly in the hook FAQ that multi-turn is unavailable in hook mode and the user should use `stagecoach` directly for large diffs.

### Issue 3: `--verbose` does not print the per-chunk token estimate (FR-T11)

**Severity**: Minor

**PRD Reference**: §9.24 FR-T11 ("`--verbose` prints, for a multi-turn run: the trigger, N+1, **the per-chunk token estimate**, the session id, and — per turn — the payload size + raw stdout + raw stderr").

**Expected Behavior**: At multi-turn fallback time, `--verbose` should print a summary line including the per-chunk token estimate (how many tokens each chunk targets), so the user can verify the chunking budget is appropriate.

**Actual Behavior**: The verbose output at trigger time (`internal/generate/generate.go:311`) prints:
- ✅ The trigger: `VerboseWarn("one-shot exhausted → multi-turn fallback")`
- ✅ N+1 and total budget: `fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)`
- ❌ The per-chunk token estimate: **NOT printed**. The progress line says "N turns, ~Mm total" but does not include the per-chunk token budget (`cfg.MultiTurnChunkTokens`).

The per-turn `VerbosePayload` call (in `executor.go`) does print each turn's byte count + token estimate, which indirectly conveys chunk sizes. But the FR-T11 requirement for an explicit "per-chunk token estimate" at trigger time is not met.

**Steps to Reproduce**:
1. Configure a large diff with pi as the provider.
2. `stagecoach -v` → observe the trigger line; it lacks the chunk token estimate.

**Suggested Fix**: Add the chunk token budget to the progress line or the verbose trigger line. For example, extend the progress line to: `"↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n"` where the second `%d` is `cfg.MultiTurnChunkTokens`. Alternatively, add a `VerbosePayload`-style line for the total payload token count alongside the trigger.

### Issue 4: Payload inconsistency between TokenLimit=0 and TokenLimit≠0 multi-turn paths

**Severity**: Minor

**PRD Reference**: §9.24 FR-T2 ("The multi-turn payload is the SAME captured payload the one-shot path would send ... captured ONCE and unmodified").

**Expected Behavior**: The multi-turn payload should be consistent regardless of whether `token_limit` is set — it should always be the untruncated captured payload.

**Actual Behavior**: In `CommitStaged`'s multi-turn gate (`internal/generate/generate.go`), the `mtPayload` variable is set differently depending on `cfg.TokenLimit`:

- **When `TokenLimit == 0`**: `mtPayload = payload` — the hoisted loop variable, which may have the `retryInstr` prepended (if the last one-shot attempt failed parsing: `payload = retryInstr + "\n\n" + payload`). The `retryInstr` is the corrective preamble ("Output ONLY the commit message. No preamble, no markdown, no quotes.").

- **When `TokenLimit != 0`**: `mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)` — a freshly rebuilt payload from the re-captured diff. This does **NOT** include `retryInstr`.

So the multi-turn session content differs depending on whether `token_limit` is set. The `retryInstr` in the TokenLimit=0 path is somewhat at odds with the multi-turn preamble ("Do not analyze or write any commit message until I explicitly ask at the end"), but it doesn't cause a functional failure — the model replies "ok" to chunk turns regardless, and the final turn produces the message.

**Steps to Reproduce**:
1. Configure a large diff with `token_limit = 0` and a pi provider.
2. Force a one-shot parse failure (e.g., via a model that returns empty stdout).
3. Observe (via `--verbose` or a debugger) that the multi-turn turn-1 payload includes the retry instruction.
4. Repeat with `token_limit = 120000` — the multi-turn turn-1 payload does NOT include the retry instruction.

**Suggested Fix**: When `TokenLimit == 0`, rebuild `mtPayload` from the existing `diff` (which is already untruncated when TokenLimit=0) via `prompt.BuildUserPayload(diff, cfg.Context, rejected)`, stripping the `retryInstr` prepend — matching the TokenLimit≠0 path. Alternatively, always rebuild `mtPayload` from the captured `diff` regardless of TokenLimit, since `diff` is already in scope at the gate.

---

## Testing Summary

- **Total tests performed**: ~15 manual code-path traces, 4 existing test suites run (`go test ./internal/generate/...`, `./internal/provider/...`, `./internal/config/...` — all passing), 3 generation-loop code audits, documentation cross-references, edge-case analysis (empty payloads, single-line payloads, CJK content, ceil rounding, newline anchoring, chunk boundary math).
- **Passing**: All existing unit and integration tests pass. The core multi-turn implementation in `CommitStaged` is correct and well-tested.
- **Failing**: N/A (no existing test covers the dry-run × multi-turn intersection — this gap is itself evidence of the Issue 1 bug).
- **Areas with good coverage**:
  - Provider surface: `SessionMode` field, `RenderMultiTurn` golden tests, pi builtin value, capability gate, non-mutation guard — all comprehensive.
  - Config surface: `multi_turn_fallback` / `multi_turn_chunk_tokens` materialize/overlay tests, end-to-end TOML decode, the accepted false-ignored limitation pinned.
  - Generate core: `chunkPayload` ceil math, newline anchoring, CJK rune handling, lossless round-trip, PART prefix format; `Run` happy path / turn-error / final-parse-empty / non-append; `CommitStaged` trigger gate truth table (all 4 FR-T1 conditions), token_limit non-interaction, token_limit-truncated re-capture.
  - Integration: N+1 turn render contract (`--session-id` present/stable, `--no-session` dropped, system-prompt turn-1-only), mid-turn failure → rescue with full idempotent-index invariant, small-payload skip, non-append skip.
- **Areas needing more attention**:
  - **`runPipeline` (dry-run path)**: zero multi-turn coverage — the multi-turn gate was never propagated here (Issue 1).
  - **Hook exec path**: zero multi-turn coverage (Issue 2).
  - **Multi-turn with `--edit`**: not explicitly tested (the `--edit` gate runs after multi-turn succeeds, which is correct, but no integration test confirms the composition).
  - **Multi-turn with duplicate subjects**: the FR-T7 "final turn's output failing to dedupe → rescue" path is tested at the unit level (`Run` returns ok=false for empty final stdout) but the CommitStaged-level multi-turn-then-duplicate → rescue path is only inferred from the code, not explicitly tested.
