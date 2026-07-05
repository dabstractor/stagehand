# System Context — Multi-Turn Fallback Propagation & Hardening

> **Bugfix scope**: Propagate the FR-T1 multi-turn fallback trigger gate from the reference
> implementation (`CommitStaged`) into the two generation-loop mirrors that lack it
> (`runPipeline` for `--dry-run`/`SystemExtra`, and `hook.Run` for git hooks), plus three
> minor correctness/UX fixes (payload consistency, verbose token estimate, hook FR-H5 safety).
>
> **Date**: 2026-07-05 · **Module**: `github.com/dustin/stagehand` (Go 1.22)

---

## 1. The Three Generation Loops

The codebase has **three near-identical generation loops** — the structural root cause of this bugfix.
The multi-turn gate was wired into only ONE of them (CommitStaged). The other two are pre-multi-turn copies.

| # | Path | File | Loop line | Multi-turn? | Package |
|---|------|------|-----------|-------------|---------|
| 1 | `CommitStaged` (commit path) | `internal/generate/generate.go:229` | ✅ gate at 290–374 | ✅ **reference** | `internal/generate` |
| 2 | `runPipeline` (dry-run / SystemExtra) | `pkg/stagehand/stagehand.go:489` | ❌ **MISSING** | `pkg/stagehand` |
| 3 | `hook.Run` (git hook mode) | `internal/hook/exec.go:157` | ❌ **MISSING** | `internal/hook` |

### Routing decision (`GenerateCommit`, `pkg/stagehand/stagehand.go:160`)

```go
if !opts.DryRun && opts.SystemExtra == "" {
    res, gerr := generate.CommitStaged(ctx, deps, cfg)  // ← HAS multi-turn
}
return runPipeline(ctx, deps, cfg, opts.SystemExtra, opts.DryRun)  // ← NO multi-turn
```

When `DryRun=true` OR `SystemExtra != ""`, execution always takes the `runPipeline` branch.

---

## 2. The Reference FR-T1 Multi-Turn Gate (`CommitStaged`)

**Location**: `internal/generate/generate.go:290–374` — inserted between the generation loop's
end and the byte-identical rescue return.

### The 4 trigger conditions (FR-T1)

| # | Condition | Code | Line |
|---|-----------|------|------|
| (a) | One-shot exhausted | `if !success {` | 290 |
| (b) | Payload exceeds one chunk | `git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens` | 330 |
| (c) | Multi-turn enabled in config | `cfg.MultiTurnFallback` | 304 |
| (d) | Provider declares `session_mode="append"` | `resolved.SessionMode != nil && *resolved.SessionMode == "append"` | 305 |

### The mtPayload (FR-T12 re-capture) — lines 311–329

```go
mtPayload := payload                              // DEFAULT: reuse one-shot payload
if cfg.TokenLimit != 0 {                          // FR-T12: one-shot was truncated
    fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
        ... TokenLimit: 0, PromptReserveTokens: 0,   // re-capture UNTRUNCATED
    })
    if derr == nil {
        mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
    }
}
```

### Post-Run dedupe — lines 344–366

After `multiturn.Run(...)` returns `(msg2, ok2, cause)`:
- `cause==nil && ok2` → `FinalizeMessage` → `IsDuplicate` → success or candidate=dup
- `cause!=nil` → `lastCause = cause`, candidate set from raw msg2
- Falls through to byte-identical rescue (`&RescueError{...}`) if not successful

### Key load-bearing facts

1. **`var payload string` hoisted** (L226) OUT of the loop — the gate reads it at L311.
2. **Gate ordering**: (c)+(d) checked first → FR-T12 re-capture → (b) on untruncated mtPayload.
3. **Dedupe runs ONCE post-Run** — `multiturn.Run` does NOT dedupe.
4. **Rescue is byte-identical** to the pre-multi-turn version (FR-T7: exit 3, not 124).

---

## 3. The `multiturn.Run` N+1 Protocol (`internal/generate/multiturn.go:136`)

**Signature**: `func Run(ctx, deps, cfg, manifest, sysPrompt, payload, msgModel, msgReasoning string) (msg string, ok bool, cause error)`

Protocol:
- **Turn 1**: `RenderMultiTurn(turn=1, sysPrompt, preamble + chunk[0])` → Execute
- **Turns 2..N**: `RenderMultiTurn(turn=i, sysPrompt="", chunk[i-1])` → Execute (system prompt on turn 1 ONLY)
- **Turn N+1**: `RenderMultiTurn(turn=N+1, sysPrompt="", finalInstruction)` → Execute → ParseOutput

Returns `(msg, parseOK, nil)` on success; `(msg, false, nil)` on empty final; `("", false, cause)` on any turn error.

---

## 4. The `chunkPayload` Export Constraint

`chunkPayload` (`internal/generate/multiturn.go:52`) is **unexported** and returns `[]chunk`
(`chunk` type is also unexported). It is used in exactly ONE spot outside `multiturn.Run`:
the progress-line turn-count at `generate.go:332`:

```go
turns := len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1
```

Both target paths (`pkg/stagehand`, `internal/hook`) are OUTSIDE `internal/generate` and cannot
call `chunkPayload`. **Resolution**: add a thin exported wrapper:

```go
// ChunkCount returns the number of chunks chunkPayload would produce (for progress messages).
func ChunkCount(payload string, chunkTokens int) int {
    return len(chunkPayload(payload, chunkTokens))
}
```

This avoids exposing the internal `chunk` type. CommitStaged keeps using `chunkPayload` directly
(same package); the new paths use `generate.ChunkCount`.

---

## 5. Config / Provider / Verbose Architecture

| Concern | Location | Key detail |
|---------|----------|------------|
| `MultiTurnFallback` (bool) | `internal/config/config.go:84` | default `true`; only-true-propagates (false is ignored) |
| `MultiTurnChunkTokens` (int) | `internal/config/config.go:85` | default `32000`; per-chunk token budget |
| `TokenLimit` (int) | config.go | default `0`; FR-T12: multi-turn IGNORES it |
| `resolved.SessionMode` (`*string`) | `provider.Manifest.Resolve()` | pi ships `"append"`; all others `""` |
| `git.EstimateTokens(s)` | `internal/git/tokens.go:25` | `ceil(runeCount/4)`; rune-based (CJK-safe) |
| `prompt.BuildUserPayload(diff, ctx, rejected)` | `internal/prompt/payload.go:120` | rebuilds payload from diff |
| `deps.Verbose.VerboseWarn(msg)` | `internal/ui/verbose.go:103` | nil-safe verbose trigger line |

---

## 6. Fix Strategy Summary

| Issue | Severity | Fix | Target | Milestone |
|-------|----------|-----|--------|-----------|
| ChunkCount export | Foundation | Add exported wrapper | `internal/generate/multiturn.go` | P1.M1 |
| Issue 4: mtPayload inconsistency | Minor | Rebuild from `diff` when TokenLimit==0 (strip retryInstr) | `internal/generate/generate.go` (reference gate) | P1.M1 |
| Issue 3: per-chunk token estimate | Minor | Add `cfg.MultiTurnChunkTokens` to progress line | `internal/generate/generate.go` (reference gate) | P1.M1 |
| Issue 1: dry-run multi-turn | **Major** | Hoist payload + insert gate (copy reference) | `pkg/stagehand/stagehand.go::runPipeline` | P1.M2 |
| Issue 2: hook multi-turn | Minor | Bind resolved + hoist payload + insert gate (FR-H5 preserved) | `internal/hook/exec.go::Run` | P1.M3 |
| Documentation sync | Mode B | Update overview docs | `docs/how-it-works.md`, `README.md` | P1.M4 |

**Key ordering principle**: fix the reference gate FIRST (M1), then propagate the corrected gate
to the two target paths (M2, M3). This ensures all three paths have consistent behavior.
