# Resolution Strategy — Per-Issue Code Changes

> Exact, verified code changes for each issue. All line numbers against the working tree
> as of 2026-07-05. The reference gate lives at `internal/generate/generate.go:290–374`.

---

## FOUNDATION: Export `ChunkCount` (prerequisite for Issue 1 & Issue 2)

**File**: `internal/generate/multiturn.go`
**Location**: after `chunkPayload` (around line 93)
**Change**: Add exported wrapper:

```go
// ChunkCount returns the number of chunks chunkPayload would produce for the given payload and
// per-chunk token budget. Exported so cross-package callers (runPipeline, hook.Run) can compute
// the N+1 turn count for progress messages without depending on the internal chunk type.
func ChunkCount(payload string, chunkTokens int) int {
    return len(chunkPayload(payload, chunkTokens))
}
```

**Impact**: Zero behavioral change. `CommitStaged` continues calling `chunkPayload` directly
(same package). `runPipeline` and `hook.Run` call `generate.ChunkCount`.
**Test**: `TestChunkCount_*` — verify it matches `len(chunkPayload(...))` for edge cases
(empty, single-chunk, multi-chunk, sub-1 budget).

---

## ISSUE 4: mtPayload Inconsistency (TokenLimit==0 vs ≠0)

**File**: `internal/generate/generate.go`
**Location**: line 311 (inside the FR-T1 gate, the `mtPayload := payload` line)
**Problem**: When `TokenLimit==0`, `mtPayload = payload` — but `payload` may have `retryInstr`
prepended (from the last failed one-shot attempt: `payload = retryInstr + "\n\n" + payload`).
The TokenLimit≠0 path rebuilds via `BuildUserPayload(fullDiff, ...)` WITHOUT retryInstr.
This makes the multi-turn session content inconsistent.

**Fix**: When `TokenLimit==0`, rebuild `mtPayload` from the existing untruncated `diff` variable
(stripping any `retryInstr`), matching the TokenLimit≠0 path:

Before (line 311):
```go
mtPayload := payload
```

After:
```go
// FR-T2: multi-turn payload = the SAME captured payload the one-shot path sends, captured ONCE
// and unmodified. Always rebuild from the untruncated diff WITHOUT retryInstr (the retry
// corrective preamble is one-shot-only; multi-turn has its own preamble). When TokenLimit==0,
// `diff` is already untruncated — rebuild from it directly (avoids a redundant StagedDiff call).
mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
```

The `diff` variable is in scope (declared at the top of `CommitStaged`, line ~193, from
`deps.Git.StagedDiff`). When `TokenLimit==0`, this diff is untruncated. When `TokenLimit!=0`,
the existing re-capture branch (lines 312–326) overwrites `mtPayload` with the re-captured
untruncated version. So both paths now produce an `mtPayload` derived from the untruncated diff,
WITHOUT `retryInstr`. Consistent.

**Test**: Assert `mtPayload` does NOT contain the retry instruction preamble when triggered
via a forced parse-failure on a TokenLimit==0 large diff. Mirror the pattern of
`TestMultiTurnGate_TokenLimitTruncated_Recaptures` but for the TokenLimit==0 + parseFail case.

---

## ISSUE 3: Per-Chunk Token Estimate in Verbose (FR-T11)

**File**: `internal/generate/generate.go`
**Location**: line 337 (the progress `fmt.Fprintf` line)
**Problem**: The progress line prints turn count and total minutes, but NOT the per-chunk token
budget (`cfg.MultiTurnChunkTokens`). FR-T11 requires the per-chunk token estimate.

**Fix**: Extend the progress line to include the chunk token budget:

Before (line 337):
```go
fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)
```

After:
```go
fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n",
    turns, cfg.MultiTurnChunkTokens, totalMin)
```

The second `%d` (`cfg.MultiTurnChunkTokens`) is the per-chunk token estimate — the target size
each chunk aims for. This is the PRD's suggested fix ("chunks of ~%d tokens").

**Impact**: This fix is in the reference gate. When propagated to `runPipeline` and `hook.Run`
(M2, M3), the same format string is used, ensuring all three paths have identical verbose output.

**Test**: Assert the verbose/progress line contains the chunk token value. Mirror the truth-table
test's substring assertion pattern (currently asserts `"multi-turn fallback"`; add assertion for
the chunk token substring).

---

## ISSUE 1: Dry-Run Path Multi-Turn Propagation (MAJOR)

**File**: `pkg/stagehand/stagehand.go`
**Function**: `runPipeline` (line 415)
**Insertion point**: between the loop end (line 542) and the rescue return (line 543)

### Edit 1 — Hoist `payload` (lines 483–487)

Before:
```go
retryInstr := *resolved.RetryInstruction
var rejected []string
var candidate, msg string
var parseFail, success bool
var lastCause error
```

After (add one line):
```go
retryInstr := *resolved.RetryInstruction
var rejected []string
var candidate, msg string
var parseFail, success bool
var lastCause error
var payload string // hoisted: survives the loop for the FR-T1 multi-turn gate (mirrors CommitStaged)
```

### Edit 2 — Loop body: `:=` → `=` (line 490)

Before:
```go
payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
```

After:
```go
payload = prompt.BuildUserPayload(diff, cfg.Context, rejected) // `=` (payload hoisted above)
```

### Edit 3 — Insert the FR-T1 gate (lines 543–548)

Before:
```go
if !success {
    return Result{}, &generate.RescueError{
        Kind: generate.ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
        Candidate: candidate, Cause: lastCause,
    }
}
```

After (wrap return with gate; copy from CommitStaged with `generate.`/`prompt.`/`git.` prefixes;
uses `generate.ChunkCount`, `generate.Run`; applies Issue 4 fix via `prompt.BuildUserPayload(diff, ...)`
and Issue 3 fix via the chunk-tokens progress format):

```go
if !success {
    // ---- FR-T1 multi-turn fallback trigger gate (PRD §9.24) — ported from CommitStaged (generate.go:290-374). ----
    if cfg.MultiTurnFallback &&
        resolved.SessionMode != nil && *resolved.SessionMode == "append" {

        mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected) // FR-T2: rebuild from untruncated diff, no retryInstr
        if cfg.TokenLimit != 0 { // FR-T12: re-capture with TokenLimit=0
            fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
                MaxDiffBytes:        cfg.MaxDiffBytes,
                MaxMDLines:          cfg.MaxMdLines,
                BinaryExtensions:    cfg.BinaryExtensions,
                Excludes:            deps.Excludes,
                TokenLimit:          0,
                DiffContext:         cfg.DiffContextValue(),
                PromptReserveTokens: 0,
            })
            if derr == nil {
                mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
            }
        }

        if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {
            turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1
            totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
            if totalMin < 1 {
                totalMin = 1
            }
            fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n",
                turns, cfg.MultiTurnChunkTokens, totalMin)
            deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")

            msg2, ok2, cause := generate.Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)

            if cause == nil && ok2 {
                finalMsg := generate.FinalizeMessage(msg2, cfg)
                signal.SetCandidate(finalMsg)
                if !generate.IsDuplicate(generate.ExtractSubject(finalMsg), recent) {
                    msg = finalMsg
                    success = true
                } else {
                    candidate = finalMsg
                }
            } else {
                if cause != nil {
                    lastCause = cause
                }
                if msg2 != "" {
                    candidate = msg2
                }
            }
        }
    }
    if !success {
        return Result{}, &generate.RescueError{
            Kind: generate.ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
            Candidate: candidate, Cause: lastCause,
        }
    }
}
```

### Variables confirmed in scope at insertion point

`ctx` ✅ `deps` ✅ `cfg` ✅ `resolved` ✅ (L470) `sysPrompt` ✅ (L425) `msgModel`/`msgReasoning` ✅ (L474)
`diff` ✅ (L446) `rejected` ✅ (L484) `recent` ✅ (L464) `payload` ✅ (after hoist) `candidate`/`msg`/`success`/`lastCause` ✅
`treeSHA`/`parentSHA` ✅

### Dry-run success path — NO change needed

The gate only sets `msg`/`success`. On a multi-turn win with `dryRun=true`, the existing
`if dryRun { return Result{...}, nil }` at line ~565 fires correctly. The gate is purely
a `msg`/`success` transformation — all downstream plumbing is shared.

---

## ISSUE 2: Hook Exec Multi-Turn Propagation

**File**: `internal/hook/exec.go`
**Function**: `Run` (line 97)
**Insertion point**: between loop end (line 204) and exhaustion return (line 205)

### Edit 1 — Bind `resolved` (line 151)

Before:
```go
retryInstr := *deps.Manifest.Resolve().RetryInstruction
```

After:
```go
resolved := deps.Manifest.Resolve()
retryInstr := *resolved.RetryInstruction
```

### Edit 2 — Hoist `payload` (lines 154–155)

Before:
```go
var rejected []string
var parseFail bool
```

After:
```go
var rejected []string
var parseFail bool
var payload string // hoisted: survives the loop for the FR-T1 gate
```

### Edit 3 — Loop body: `:=` → `=` (line 158)

Before:
```go
payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
```

After:
```go
payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)
```

### Edit 4 — Insert the FR-T1 gate (before line 205)

Before:
```go
// Step G: exhaustion after bounded retries.
return fmt.Errorf("stagehand: hook generation failed after %d retries", cfg.MaxDuplicateRetries)
```

After (insert gate BEFORE the exhaustion return; on success calls `WriteMessageFile`;
on ANY failure falls through to the exhaustion error → cmd layer maps to exit 0 = FR-H5 preserved):

```go
// ---- FR-T1 multi-turn fallback gate (PRD §9.24) — mirrors CommitStaged, preserves FR-H5. ----
// On any multi-turn failure (cause != nil / parse empty / duplicate), we fall through to the
// exhaustion error, which the cmd layer maps to exit 0 (neverBlock). WriteMessageFile runs ONLY
// on success — the msg-file is untouched on every failure path (FR-H5 invariant holds).
if cfg.MultiTurnFallback &&
    resolved.SessionMode != nil && *resolved.SessionMode == "append" {

    mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
    if cfg.TokenLimit != 0 {
        fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
            MaxDiffBytes:        cfg.MaxDiffBytes,
            MaxMDLines:          cfg.MaxMdLines,
            BinaryExtensions:    cfg.BinaryExtensions,
            Excludes:            deps.Excludes,
            TokenLimit:          0,
            DiffContext:         cfg.DiffContextValue(),
            PromptReserveTokens: 0,
        })
        if derr == nil {
            mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
        }
    }

    if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {
        turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1
        totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
        if totalMin < 1 {
            totalMin = 1
        }
        fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n",
            turns, cfg.MultiTurnChunkTokens, totalMin)
        if deps.Verbose != nil {
            deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")
        }

        msg2, ok2, cause := generate.Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)
        if cause == nil && ok2 {
            finalMsg := generate.FinalizeMessage(msg2, cfg)
            if !generate.IsDuplicate(generate.ExtractSubject(finalMsg), recent) {
                return WriteMessageFile(msgFile, finalMsg) // SUCCESS — only write site (FR-H5 preserved)
            }
        }
    }
}

// Step G: exhaustion after bounded retries.
return fmt.Errorf("stagehand: hook generation failed after %d retries", cfg.MaxDuplicateRetries)
```

### Import needed

`internal/hook/exec.go` does NOT currently import `time`. The progress line uses
`time.Duration(turns)`. **Add `"time"` to the import block.**

### FR-H5 preservation analysis

| Multi-turn outcome | `hook.Run` returns | cmd `neverBlock` | exit | msg-file |
|---|---|---|---|---|
| `cause != nil` | exhaustion error (fall through) | prints + exit 0 | **0** | untouched ✅ |
| `ok2 == false` | exhaustion error (fall through) | prints + exit 0 | **0** | untouched ✅ |
| duplicate subject | exhaustion error (fall through) | prints + exit 0 | **0** | untouched ✅ |
| success + non-dup | `WriteMessageFile(...)` → `nil` | exit 0 | **0** | written ✅ |

In every case exit is 0 and the msg-file is written ONLY on success. **FR-H5 holds.**

---

## Variables confirmed in scope at hook insertion point

`ctx` ✅ `deps` ✅ `cfg` ✅ `msgFile` ✅ `sysPrompt` ✅ (L118) `diff` ✅ (L134)
`recent` ✅ (L144) `msgModel`/`msgReasoning` ✅ (L146) `rejected` ✅ (L154)
`payload` ✅ (after hoist) `resolved` ✅ (after Edit 1)
