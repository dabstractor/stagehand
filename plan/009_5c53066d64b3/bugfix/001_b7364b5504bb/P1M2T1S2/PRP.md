---
name: "P1.M2.T1.S2 — Insert FR-T1 multi-turn trigger gate into runPipeline with dedupe and verbose"
description: |
  Close Issue 1 (Major): the `--dry-run` path (`runPipeline` in `pkg/stagecoach/stagecoach.go`) silently lacks
  the FR-T1 multi-turn fallback that `CommitStaged` (the commit path) already has — so `stagecoach --dry-run`
  on a large diff rescues (exit 1) where `stagecoach` (no --dry-run) succeeds via multi-turn, directly
  contradicting FR49 ("--dry-run runs the full pipeline"). This subtask PORTS the proven FR-T1 trigger gate
  from `internal/generate/generate.go::CommitStaged` (~L290-374) into `runPipeline`, verbatim but with
  `generate.`/`prompt.`/`git.`/`signal.` package prefixes (runPipeline is package stagecoach). PRD: §9.12 FR49,
  §9.24 FR-T1–FR-T12. Research: `docs/architecture/resolution_strategy.md` ISSUE 1 Edit 3 (the authoritative
  full gate code), `docs/architecture/research_runpipeline_dryrun.md` §4/§5/§6/§10, and
  `research/s2_runpipeline_gate_map.md` (verified-against-live touchpoint map).

  ⚠️ **THE central design call — PORT THE PROVEN GATE VERBATIM (resolution_strategy.md Edit 3), not a re-imagining.**
  The gate is already designed, reviewed, and battle-tested in CommitStaged (P1.M1.T3.S3 wired it; P1.M1.T2.S1
  fixed its mtPayload; P1.M1.T3.S1 added the chunk-tokens verbose line). S2 copies that exact logic into
  runPipeline's `if !success {` block, swapping (a) `chunkPayload` → `generate.ChunkCount` (runPipeline is
  cross-package; ChunkCount is the exported wrapper from P1.M1.T1.S1), (b) `Run`/`FinalizeMessage`/`IsDuplicate`/
  `ExtractSubject` → `generate.`-prefixed, (c) `signal.SetCandidate` → `signal.`-prefixed. The four FR-T1
  conditions (a: one-shot exhausted [already true at !success]; b: EstimateTokens(mtPayload) >
  MultiTurnChunkTokens; c: cfg.MultiTurnFallback; d: *resolved.SessionMode=="append") are evaluated identically.

  ⚠️ **THE second design call — the gate goes INSIDE the existing `if !success {` block, BEFORE the rescue return,
  and PRESERVES the byte-identical rescue on fall-through + the dry-run success early-return.** The current code
  is `if !success { return Result{}, &generate.RescueError{...} }`. S2 wraps it: the gate runs first (may set
  msg/success=true on a multi-turn win); then a NEW inner `if !success { return ... &generate.RescueError{...} }`
  fires ONLY when the gate did not win (condition fail OR multi-turn failed OR duplicate). On a multi-turn win,
  `success=true` → inner if is false → falls through to the UNCHANGED dry-run success path (`if dryRun { return
  Result{CommitSHA:"", Subject:..., Message: msg}, nil }` at ~L565) or the commit tail. The rescue return is
  byte-identical to today when the gate doesn't win. See `research/s2_runpipeline_gate_map.md` §2/§3/§6.

  ⚠️ **THE third design call — ZERO new imports; all gate dependencies already imported in stagecoach.go.** Verified
  live: `fmt`, `os`, `time`, `git`, `prompt`, `generate`, `signal` are ALL in the import block. `generate.Run`
  (signature: `Run(ctx, deps, cfg, manifest, sysPrompt, payload, msgModel, msgReasoning) (msg, ok, cause)`) and
  `generate.ChunkCount(payload, chunkTokens) int` are exported. Do NOT add imports (an unused import fails go vet)
  and do NOT change go.mod/go.sum. See `research/s2_runpipeline_gate_map.md` §4/§5.

  ⚠️ **THE fourth design call — the gate rebuilds mtPayload from `diff` (Issue 4 fix), NOT from the hoisted `payload`.**
  S1 (P1.M2.T1.S1, ALREADY APPLIED in the live file) hoisted `var payload string` to function scope, but the gate
  does NOT read it — it rebuilds `mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)` from the
  UNTRUNCATED `diff` (and re-captures diff with TokenLimit=0 when cfg.TokenLimit!=0, FR-T12). The hoist is
  structurally harmless (payload stays function-scoped + used by the loop body). Do NOT "use" the hoisted payload
  in the gate — that would reintroduce the Issue 4 bug (retryInstr preamble leaking into multi-turn).

  ⚠️ **THE fifth design call — test infra: append-mode stub via TOML (appendScriptManifest is NOT in stagecoach_test.go).**
  The `appendScriptManifest` helper exists ONLY in internal/generate (returns a direct provider.Manifest for
  CommitStaged tests). pkg/stagecoach tests go through GenerateCommit → config.Load → buildDeps (registry), so the
  stub is registered via repo-local `.stagecoach.toml` `[provider.stub]`. VERIFIED: `RenderMultiTurn` is a METHOD on
  `provider.Manifest` (render.go:203) and `SessionMode *string toml:"session_mode"` (manifest.go:66) is a TOML
  field → a TOML `[provider.stub]` with `session_mode = "append"` supports multi-turn through the registry. S2
  EXTENDS the existing setupScriptedRepo/setupTestRepo pattern (add `session_mode = "append"` to the TOML block)
  and writes 4 integration tests mirroring internal/generate/generate_multiturn_test.go (large diff via a
  strings.Builder loop + MultiTurnChunkTokens=50 + MultiTurnFallback=true). See `research/s2_runpipeline_gate_map.md` §7.

  Deliverable (edits to existing files only — NO new files, NO new deps, NO go.mod change): (1) `pkg/stagecoach/
  stagecoach.go` — insert the FR-T1 gate into runPipeline's `if !success {` block (verbatim from resolution_strategy
  Edit 3); (2) `pkg/stagecoach/stagecoach_test.go` — add an append-mode stub-registration helper + 4 integration tests.
  INPUT = the hoisted payload (S1, done) + CommitStaged's proven gate (P1.M1.T3.S3 + Issue 4 fix P1.M1.T2.S1 +
  Issue 3 verbose P1.M1.T3.S1) + generate.ChunkCount (P1.M1.T1.S1) + the live runPipeline insertion point.
  OUTPUT = runPipeline has the full FR-T1 gate; `stagecoach --dry-run` on a large diff with an append-mode provider
  activates multi-turn identically to the commit path; the existing dry-run success early-return + byte-identical
  rescue on failure are preserved. DOCS = none per-file (the cross-cutting overview/README sync is the final Mode B
  task P1.M4.T1). SCOPE: pkg/stagecoach/{stagecoach.go, stagecoach_test.go} ONLY. Do NOT touch internal/generate/*
  (the reference gate is INPUT) or internal/hook/exec.go (P1.M3's scope).
---

## Goal

**Feature Goal**: Port the FR-T1 multi-turn trigger gate from `CommitStaged` into `runPipeline` (the `--dry-run` /
SystemExtra path in `pkg/stagecoach/stagecoach.go`) so that `stagecoach --dry-run` on a large diff whose one-shot
generation exhausts activates the multi-turn fallback identically to the commit path — closing the Issue 1
inconsistency where `--dry-run` rescues (exit 1) but the commit path succeeds (exit 0) on the same diff. The four
FR-T1 conditions, the FR-T12 token-limit re-capture, the Issue 4 mtPayload rebuild, the Issue 3 chunk-tokens
verbose line, the dedupe of the multi-turn result, and the byte-identical rescue-on-failure are all preserved.

**Deliverable** (edits to existing files only):
1. **`pkg/stagecoach/stagecoach.go`** — insert the FR-T1 multi-turn trigger gate (verbatim from
   `docs/architecture/resolution_strategy.md` ISSUE 1 Edit 3) INTO runPipeline's `if !success {` block, before
   the rescue return. Uses `generate.ChunkCount`/`generate.Run`/`generate.FinalizeMessage`/`generate.IsDuplicate`/
   `generate.ExtractSubject`, `prompt.BuildUserPayload`, `git.StagedDiff`/`git.EstimateTokens`, `signal.SetCandidate`,
   `fmt.Fprintf(os.Stderr, ...)`. ZERO new imports (all already present).
2. **`pkg/stagecoach/stagecoach_test.go`** — (a) a helper (or extension of setupScriptedRepo/setupTestRepo) that
   registers the stub provider with `session_mode = "append"` in the repo-local `.stagecoach.toml`; (b) 4 integration
   tests: `TestGenerateCommit_DryRun_MultiTurnSuccess`, `_MultiTurnSkipped_NonAppend`, `_MultiTurnSmallPayloadSkip`,
   `_MultiTurnMidTurnFailure` — mirroring `internal/generate/generate_multiturn_test.go` patterns.

**Success Definition**: `gofmt -l`, `go vet ./...`, `go build ./...` clean; `go test -race ./...` green (existing
tests unchanged + 4 new tests pass); `stagecoach --dry-run` on a large diff with an append-mode provider activates
multi-turn (the progress line prints, multi-turn Run is called, the message is produced and returned with
CommitSHA="" for dry-run); a non-append provider / small payload / mid-turn failure all fall through to the
byte-identical rescue; the dry-run success early-return fires on a multi-turn win; go.mod/go.sum byte-unchanged;
only `pkg/stagecoach/{stagecoach.go, stagecoach_test.go}` touched.

## User Persona

**Target User**: The user who runs `stagecoach --dry-run` (FR49) — especially on a large diff with an append-mode
provider (pi with `session_mode="append"`). Today they see a rescue error where the commit path would succeed;
after S2, dry-run runs the FULL pipeline including multi-turn.

**Use Case**: `stagecoach --dry-run` on a 200+ line diff with pi configured `session_mode = "append"` — the one-shot
generation exhausts (empty/unparseable after retries), the FR-T1 gate fires, multi-turn losslessly re-delivers the
diff across N+1 session turns, produces a single message, and dry-run returns it (exit 0, no commit).

**User Journey**: `--dry-run` → GenerateCommit → runPipeline → generation loop exhausts → **FR-T1 gate (S2)** →
multi-turn Run → FinalizeMessage → dedupe → msg/success set → `if dryRun { return Result{CommitSHA:"", Subject,
Message}, nil }`. Without S2 the journey ends at `return &RescueError{...}` (exit 1).

**Pain Points Addressed**: the `--dry-run` × multi-turn gap (Issue 1) — dry-run no longer silently lacks the
fallback that the commit path has.

## Why

- **Closes a Major bug (Issue 1) that contradicts FR49.** FR49 requires `--dry-run` to "run the full pipeline";
  multi-turn is now part of that pipeline. The inconsistency (dry-run fails where commit succeeds) is a real
  user-facing regression for anyone previewing a large-diff commit.
- **One proven gate, two call sites.** The gate is already designed + tested in CommitStaged. S2 is a faithful
  port (cross-package prefixes only) — low risk, high parity. The long-term dedup (extract a shared loop) is noted
  in resolution_strategy.md but explicitly out of scope (S2 copies the gate, per the bugfix plan).
- **Preserves all invariants.** The rescue-on-failure is byte-identical; the dry-run success early-return is
  unchanged; the FR-T12 token-limit non-interaction + Issue 4 mtPayload + Issue 3 verbose line all carry over.
- **No API/config/deps change.** Pure control-flow port + tests. go.mod unchanged.

## What

A compiled `runPipeline` with the FR-T1 multi-turn trigger gate inserted between the generation-loop exhaust and
the rescue return; 4 integration tests covering the success / non-append-skip / small-payload-skip / mid-turn-failure
quadrant. No new types, no new files, no dependency change, no config change.

### Success Criteria

- [ ] `pkg/stagecoach/stagecoach.go` runPipeline: the `if !success { return Result{}, &generate.RescueError{...} }`
      block is replaced by `if !success { <FR-T1 gate> ; if !success { return Result{}, &generate.RescueError{...} } }`,
      where the gate is VERBATIM from resolution_strategy.md ISSUE 1 Edit 3 (the 4 conditions, FR-T12 re-capture,
      Issue 4 mtPayload rebuild from `diff`, Issue 3 chunk-tokens Fprintf, `generate.Run`, FinalizeMessage →
      signal.SetCandidate → IsDuplicate → set msg/success or candidate/lastCause).
- [ ] The gate uses `generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens)` (NOT the unexported `chunkPayload`)
      and `generate.`-prefixed Run/FinalizeMessage/IsDuplicate/ExtractSubject; `signal.SetCandidate`;
      `prompt.BuildUserPayload(diff, cfg.Context, rejected)`; `git.EstimateTokens`/`git.StagedDiff`.
- [ ] ZERO new imports in stagecoach.go (fmt/os/time/git/prompt/generate/signal already present); go.mod/go.sum
      byte-unchanged.
- [ ] The dry-run success early-return (`if dryRun { ... return Result{CommitSHA:"", ...}, nil }`) is UNCHANGED —
      it fires on a multi-turn win because the gate only sets msg/success.
- [ ] The rescue `&generate.RescueError{Kind: ErrRescue, TreeSHA, ParentSHA, Candidate, Cause: lastCause}` is
      byte-identical on fall-through (gate condition fail OR multi-turn failure OR duplicate).
- [ ] `pkg/stagecoach/stagecoach_test.go`: an append-mode stub-registration helper exists (writes
      `session_mode = "append"` into `[provider.stub]`); the 4 named tests exist and pass.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l`, `go test -race ./...` clean/green; go.mod/go.sum byte-unchanged;
      only `pkg/stagecoach/{stagecoach.go, stagecoach_test.go}` touched; no edits to `internal/generate/*` or
      `internal/hook/exec.go`.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the verbatim gate code (quoted in full
below + in resolution_strategy.md Edit 3), the exact insertion point (the `if !success {` block), the confirmed
in-scope variable list, the zero-new-imports fact, and the 4 test specs + the append-mode TOML stub pattern. No
git/provider internals beyond "RenderMultiTurn is a method on provider.Manifest; generate.Run runs the N+1 turns".

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M2T1S2/research/s2_runpipeline_gate_map.md
  why: the AUTHORITATIVE verified-against-live touchpoint map. §1 = S1 hoist already applied; §2 = insertion point;
       §3 = the FULL gate code VERBATIM (copy it); §4 = zero new imports (verified); §5 = in-scope vars; §6 =
       dry-run success path unchanged; §7 = test infra (append-mode stub via TOML).
  critical: §3 + §4 + §7. Copy the §3 gate verbatim (it has the correct package prefixes + Issue 3/4 fixes). Do NOT
       add imports (§4 — all present). The test infra (§7) needs an append-mode TOML stub, NOT the generate pkg's
       appendScriptManifest (which is direct-manifest).

- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/docs/architecture/resolution_strategy.md
  section: "ISSUE 1: Dry-Run Path Multi-Turn Propagation" → "Edit 3 — Insert the FR-T1 gate (lines 543–548)"
  why: the canonical edit recipe. Edit 3 is the FULL gate code (the source of truth for §3 of the touchpoint map).
       The "Variables confirmed in scope" + "Dry-run success path — NO change needed" notes confirm the gate is
       self-contained at the insertion point.
  critical: Edit 1 + Edit 2 are S1's hoist (ALREADY DONE — do not re-do). Edit 3 is THIS task (S2). Do NOT touch
       ISSUE 2 (hook exec — P1.M3's scope).

- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/docs/architecture/research_runpipeline_dryrun.md
  section: "§4 (variables in scope), §5 (canonical gate to port), §6 (dry-run success path), §10 (edit recipe Edit 3)"
  why: the deeper scout — confirms every gate input is in scope at the insertion point + the dry-run success path
       needs no change (the gate only sets msg/success).

- file: internal/generate/generate.go   (READ ONLY — the reference gate, ~L290-374 inside CommitStaged's if !success)
  why: the PROVEN gate S2 ports. It is the single source of behavioral truth (the 4 conditions, FR-T12, Issue 4
       mtPayload, Issue 3 Fprintf, dedupe, failure→rescue). READ to confirm parity; do NOT edit generate.go.
  pattern: the runPipeline gate is identical except (a) chunkPayload→generate.ChunkCount, (b) Run/FinalizeMessage/
       IsDuplicate/ExtractSubject→generate.-prefixed, (c) signal.SetCandidate→signal.-prefixed, (d) the rescue
       return is `generate.RescueError`/`generate.ErrRescue` (already the runPipeline style).
  gotcha: generate.go's gate calls `Run(...)` unqualified (same package); runPipeline MUST call `generate.Run(...)`.
       Likewise `chunkPayload` → `generate.ChunkCount` (ChunkCount is the EXPORTED wrapper; chunkPayload is unexported).

- file: pkg/stagecoach/stagecoach.go   (the file you EDIT — runPipeline, ~L415-570)
  why: the insertion site. The var block (~L483-488, payload ALREADY hoisted by S1), the loop (~L490-540), and the
       `if !success { return ... &generate.RescueError{...} }` block (~L542-548) you wrap with the gate.
  pattern: the gate's rescue return must match the EXISTING one byte-for-byte (`Kind: generate.ErrRescue, TreeSHA:
       treeSHA, ParentSHA: parentSHA, Candidate: candidate, Cause: lastCause`).
  gotcha: in scope at the insertion point — ctx, deps, cfg, resolved (~L470), sysPrompt (~L425), msgModel/msgReasoning
       (~L474), diff (~L446), rejected, recent (~L464), candidate/msg/success/lastCause, treeSHA, parentSHA. NO new
       upstream vars needed.

- file: pkg/stagecoach/stagecoach_test.go   (the test file you EDIT)
  why: the test patterns + stub infra. setupScriptedRepo (L82) + setupTestRepo (L122) register [provider.stub] via
       repo-local .stagecoach.toml + STAGECOACH_STUB_SCRIPT/STAGECOACH_STUB_COUNTER env (call-varying responses).
       TestGenerateCommit_DryRun (L232) is the dry-run test PATTERN (setupTestRepo + GenerateCommit(DryRun:true)).
  pattern: EXTEND setupScriptedRepo/setupTestRepo to emit `session_mode = "append"` in the [provider.stub] TOML
           block (a new helper or an option). The 4 new tests mirror TestGenerateCommit_DryRun + generate_multiturn_test.go
           (large diff via strings.Builder loop + MultiTurnChunkTokens=50 + MultiTurnFallback=true).
  gotcha: appendScriptManifest is NOT in this file (it's in internal/generate, returns a direct provider.Manifest for
           CommitStaged). pkg/stagecoach tests MUST register the append stub via TOML — VERIFIED to work because
           RenderMultiTurn is a method on provider.Manifest and session_mode is a TOML field (render.go:203 / manifest.go:66).

- file: internal/generate/generate_multiturn_test.go   (READ ONLY — the test patterns to mirror)
  why: how the generate package tests multi-turn end-to-end: LARGE staged diff (strings.Builder ~60 lines ⇒ ~600
       tokens), cfg.MultiTurnChunkTokens=50 (tiny ⇒ N≥2), cfg.MultiTurnFallback=true, appendScriptManifest (SessionMode
       ="append"). The render-contract test counts verbose command blocks via the `DEBUG: command:` prefix.
  pattern: mirror the large-diff builder + tiny chunkTokens for the Success test; the call-varying stub script makes
           one-shot return empty (exhaust) then multi-turn turns return the message.

- file: PRD.md (bugfix)   §9.12 FR49, §9.24 FR-T1–FR-T12 (the spec the gate implements)
  why: FR49 ("--dry-run runs the full pipeline") is the bug being fixed; FR-T1 (4 trigger conditions), FR-T2/T3
       (lossless chunking), FR-T4 (N+1 protocol), FR-T5 (progress line), FR-T7 (failure→rescue), FR-T11 (verbose),
       FR-T12 (token_limit non-interaction) are the gate's behavioral contract.
  critical: FR-T12 — multi-turn IGNORES token_limit (re-captures diff with TokenLimit=0). FR-T7 — any turn failure
       OR final dedupe failure → rescue (byte-identical). The gate preserves both.
```

### Current Codebase tree (relevant slice)

```bash
pkg/stagecoach/
  stagecoach.go          # runPipeline (~L415-570): var block (payload hoisted by S1) + loop + `if !success { rescue }`  ← EDIT (insert gate)
  stagecoach_test.go     # setupScriptedRepo (L82) + setupTestRepo (L122) + TestGenerateCommit_DryRun (L232)            ← EDIT (append helper + 4 tests)
internal/generate/
  generate.go           # CommitStaged FR-T1 gate (~L290-374) — the REFERENCE (READ ONLY; do NOT edit)                   ← (INPUT)
  multiturn.go          # Run (L145) + ChunkCount (L96) + chunkPayload (L52, unexported) — exported surface S2 calls      ← (INPUT)
  generate_multiturn_test.go  # the multi-turn test patterns to mirror (READ ONLY)                                       ← (INPUT)
internal/provider/
  render.go             # RenderMultiTurn (L203, method on Manifest) + Render — the render surface                                                       ← (INPUT; NO edit)
  manifest.go           # SessionMode *string toml:"session_mode" (L66) — TOML-settable                                                                  ← (INPUT; NO edit)
internal/hook/exec.go   # hook exec generation loop — P1.M3's scope (NOT this task)                                                                      ← (NO edit)
go.mod / go.sum         # unchanged (no new dep)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. All changes are EDITS to existing files (listed in Current Codebase tree above).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call #1): PORT THE GATE VERBATIM from resolution_strategy.md Edit 3 (or the equivalent in
// research/s2_runpipeline_gate_map.md §3). Do NOT re-implement from memory. The gate has 4 conditions, FR-T12,
// Issue 4, Issue 3 — all already correct in the reference. Re-implementing risks dropping a condition or the
// token-limit re-capture.

// CRITICAL (design call #2): the gate goes INSIDE `if !success {`, BEFORE the rescue. The current single return
// becomes: gate, then a NEW inner `if !success { return ... &generate.RescueError{...} }`. On a multi-turn win
// success=true → inner if skipped → falls through to the unchanged dry-run success / commit tail. Do NOT remove
// the rescue return — it must fire byte-identically when the gate doesn't win.

// CRITICAL (design call #3): ZERO new imports. fmt/os/time/git/prompt/generate/signal are ALL already imported in
// stagecoach.go (verified). Do NOT add an import (an unused one fails go vet) and do NOT change go.mod. If the gate
// doesn't compile on an undefined symbol, you used the wrong package prefix (e.g. chunkPayload instead of
// generate.ChunkCount, or Run instead of generate.Run).

// CRITICAL (design call #4): rebuild mtPayload from `diff` (Issue 4), NOT from the hoisted `payload`. S1 hoisted
// payload but the gate does NOT read it — it calls prompt.BuildUserPayload(diff, cfg.Context, rejected). Using the
// hoisted payload would reintroduce the Issue 4 bug (the retryInstr corrective preamble from a failed one-shot parse
// leaking into multi-turn, which has its own priming preamble).

// CRITICAL (design call #5): use generate.ChunkCount (EXPORTED, P1.M1.T1.S1), NOT chunkPayload (unexported —
// internal/generate only). runPipeline is package stagecoach. Likewise generate.Run / generate.FinalizeMessage /
// generate.IsDuplicate / generate.ExtractSubject / signal.SetCandidate.

// GOTCHA: the rescue return on fall-through must be byte-identical to today: `&generate.RescueError{Kind:
// generate.ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA, Candidate: candidate, Cause: lastCause}`. The gate
// may update candidate/lastCause (duplicate → candidate=finalMsg; turn failure → lastCause=cause, candidate=msg2)
// — that is correct (one-shot parity); the rescue then carries the multi-turn's failure context.

// GOTCHA: do NOT touch the dry-run success early-return (~L565 `if dryRun { signal.ClearSnapshot(); return Result{
// CommitSHA:"", Subject:..., Message: msg, ...}, nil }`). It fires unchanged on a multi-turn win (msg now carries
// the multi-turn result). The gate is a pure msg/success transformation.

// GOTCHA: appendScriptManifest is NOT in pkg/stagecoach/stagecoach_test.go (it's in internal/generate). pkg/stagecoach
// tests MUST register the append stub via repo-local .stagecoach.toml with `session_mode = "append"` in [provider.stub].
// VERIFIED this works: RenderMultiTurn is a method on provider.Manifest (render.go:203) + SessionMode is a TOML field
// (manifest.go:66) → the registry-built stub manifest supports multi-turn. Mirror setupScriptedRepo/setupTestRepo.

// GOTCHA: for the Success test, the stub script must be call-varying: the one-shot retries return empty/garbage
// (exhaust MaxDuplicateRetries ⇒ !success), THEN the multi-turn turns return content (final turn returns the message).
// The existing STAGECOACH_STUB_SCRIPT + STAGECOACH_STUB_COUNTER infra drives call-varying responses.

// GOTCHA: do NOT edit internal/generate/* (the reference gate is INPUT — P1.M1.T3.S3 wired it; it's frozen) or
// internal/hook/exec.go (P1.M3.T1 owns the hook-path propagation). This task is pkg/stagecoach ONLY.
```

## Implementation Blueprint

### Data models and structure

N/A — no types or data models. A control-flow port (insert a gate) + tests. The gate's data is the existing
runPipeline locals (msg/success/candidate/lastCause) + the rebuilt `mtPayload`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: stagecoach.go — insert the FR-T1 gate into runPipeline's `if !success {` block
  - LOCATE the block (post-loop, ~L542-548):
      if !success {
          return Result{}, &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: treeSHA,
              ParentSHA: parentSHA, Candidate: candidate, Cause: lastCause}
      }
  - REPLACE it with the WRAPPED form from research/s2_runpipeline_gate_map.md §3 (VERBATIM):
      if !success {
          // ---- FR-T1 multi-turn fallback trigger gate (PRD §9.24) — ported from CommitStaged. ----
          if cfg.MultiTurnFallback && resolved.SessionMode != nil && *resolved.SessionMode == "append" {
              mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)   // Issue 4: rebuild from diff
              if cfg.TokenLimit != 0 {                                            // FR-T12: re-capture TokenLimit=0
                  fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
                      MaxDiffBytes: cfg.MaxDiffBytes, MaxMDLines: cfg.MaxMdLines,
                      BinaryExtensions: cfg.BinaryExtensions, Excludes: deps.Excludes,
                      TokenLimit: 0, DiffContext: cfg.DiffContextValue(), PromptReserveTokens: 0,
                  })
                  if derr == nil { mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected) }
              }
              if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {       // condition (b)
                  turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1
                  totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
                  if totalMin < 1 { totalMin = 1 }
                  fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n",
                      turns, cfg.MultiTurnChunkTokens, totalMin)                  // Issue 3 format
                  deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")
                  msg2, ok2, cause := generate.Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)
                  if cause == nil && ok2 {
                      finalMsg := generate.FinalizeMessage(msg2, cfg)
                      signal.SetCandidate(finalMsg)
                      if !generate.IsDuplicate(generate.ExtractSubject(finalMsg), recent) {
                          msg = finalMsg; success = true                           // multi-turn won → skip rescue
                      } else { candidate = finalMsg }                             // duplicate → rescue w/ finalized candidate
                  } else {
                      if cause != nil { lastCause = cause }
                      if msg2 != "" { candidate = msg2 }
                  }
              }
          }
          if !success {                                                           // fall-through: gate didn't win
              return Result{}, &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: treeSHA,
                  ParentSHA: parentSHA, Candidate: candidate, Cause: lastCause}  // byte-identical rescue
          }
      }
  - IMPORTS: add NONE (fmt/os/time/git/prompt/generate/signal all present). If a symbol is undefined, fix the
    package prefix (generate.ChunkCount not chunkPayload; generate.Run not Run).
  - GOTCHA: preserve the rescue's byte-identical shape (Kind/TreeSHA/ParentSHA/Candidate/Cause). Do NOT touch the
    later dry-run success early-return or the commit tail.

Task 2: stagecoach_test.go — append-mode stub registration helper
  - ADD a helper (e.g. setupScriptedAppendRepo, OR extend setupScriptedRepo/setupTestRepo with an append option)
    that writes the repo-local `.stagecoach.toml` `[provider.stub]` block INCLUDING `session_mode = "append"`,
    pointing at the stubtest binary with STAGECOACH_STUB_SCRIPT (call-varying responses) + STAGECOACH_STUB_COUNTER.
    Mirror setupScriptedRepo (L82) — just add the `session_mode = "append"` line to the TOML writer.
  - PATTERN: the existing helpers do `sb.WriteString("[provider.stub]\n") ... sb.WriteString("command = ...\n")`
    etc.; add `sb.WriteString("session_mode = \"append\"\n")` to the [provider.stub] block.
  - GOTCHA: VERIFIED — a TOML-registered stub with session_mode="append" supports multi-turn because RenderMultiTurn
    is a method on provider.Manifest (render.go:203) and SessionMode is a TOML field (manifest.go:66). No
    direct-manifest injection seam needed (GenerateCommit goes through the registry).

Task 3: stagecoach_test.go — TestGenerateCommit_DryRun_MultiTurnSuccess
  - SETUP: scripted-append repo; stage a LARGE diff (strings.Builder ~60 lines, mirroring generate_multiturn_test.go);
    cfg via Options? No — cfg.MultiTurnChunkTokens/MultiTurnFallback come from config. Set them via the repo-local
    .stagecoach.toml `[generation]` block (multi_turn_chunk_tokens = 50, multi_turn_fallback = true) OR via a Config
    override (Options.Config). Mirror how existing tests set cfg fields (check whether setupTestRepo writes [generation]).
  - SCRIPT responses: one-shot retries return "" (exhaust MaxDuplicateRetries ⇒ !success); multi-turn turns return
    chunks; final turn returns "feat: multi-turn result".
  - CALL: GenerateCommit(ctx, Options{Provider: "stub", DryRun: true}).
  - ASSERT: err == nil; res.CommitSHA == "" (dry-run); res.Subject == "feat: multi-turn result" (or the finalized
    form); res.Message non-empty. (Exit 0 — the bug fix: previously this rescued.)

Task 4: stagecoach_test.go — TestGenerateCommit_DryRun_MultiTurnSkipped_NonAppend
  - SETUP: scripted repo WITHOUT session_mode="append" (the plain setupTestRepo); large diff; multi_turn_fallback=true,
    multi_turn_chunk_tokens=50.
  - SCRIPT: one-shot returns "" (exhaust).
  - ASSERT: err != nil; errors.As(err, &*generate.RescueError) (or whatever the existing dry-run-rescue tests assert);
    the gate was SKIPPED (condition d false). Mirror the existing TestGenerateCommit_DryRun rescue assertions.

Task 5: stagecoach_test.go — TestGenerateCommit_DryRun_MultiTurnSmallPayloadSkip
  - SETUP: scripted-append repo; TINY diff (1-2 lines ⇒ EstimateTokens(payload) ≤ 50); multi_turn_fallback=true,
    multi_turn_chunk_tokens=50.
  - SCRIPT: one-shot returns "" (exhaust).
  - ASSERT: err != nil (rescue) — condition (b) false (payload ≤ chunk threshold) so the gate skipped. Confirms the
    small-payload invariant (don't fire multi-turn when one chunk suffices).

Task 6: stagecoach_test.go — TestGenerateCommit_DryRun_MultiTurnMidTurnFailure
  - SETUP: scripted-append repo; large diff; multi_turn_fallback=true, multi_turn_chunk_tokens=50.
  - SCRIPT: one-shot returns "" (exhaust); a multi-turn turn exits non-zero (STAGECOACH_STUB_SCRIPT returns an
    exit-1 response, or the script arranges a mid-turn failure).
  - ASSERT: err != nil; RescueError — multi-turn Run aborted (cause != nil) → lastCause set → byte-identical rescue.
    Confirms FR-T7 (any turn failure → rescue).

Task 7: VERIFY (no further file change)
  - RUN `gofmt -w`; `go vet ./...`; `go build ./...`; `go test -race ./pkg/stagecoach/... -v`; `go test -race ./...`.
  - go.mod/go.sum byte-unchanged. Only pkg/stagecoach/{stagecoach.go, stagecoach_test.go} touched. internal/generate/*
    and internal/hook/exec.go byte-unchanged. Existing dry-run tests stay green (the gate only adds a path, doesn't
    alter the one-shot path).
```

### Implementation Patterns & Key Details

```go
// The gate, in miniature — port verbatim from CommitStaged with cross-package prefixes (runPipeline is pkg stagecoach):
if cfg.MultiTurnFallback && resolved.SessionMode != nil && *resolved.SessionMode == "append" { // c + d
	mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected) // Issue 4: from diff, NOT the hoisted payload
	if cfg.TokenLimit != 0 { // FR-T12: re-capture untruncated
		fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{ /* TokenLimit:0, PromptReserveTokens:0 */ })
		if derr == nil {
			mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
		}
	}
	if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens { // b
		turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1 // generate.ChunkCount (EXPORTED), not chunkPayload
		// ... Fprintf + VerboseWarn ...
		msg2, ok2, cause := generate.Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning) // generate.Run
		// ... on win: FinalizeMessage → signal.SetCandidate → IsDuplicate → msg/success or candidate ...
		// ... on fail: lastCause/candidate ...
	}
}
// then the byte-identical inner `if !success { return &generate.RescueError{...} }`

// The append-mode stub registration (pkg/stagecoach tests, via TOML — NOT the generate pkg's direct-manifest helper):
// in the [provider.stub] TOML block written by setupScriptedRepo/setupTestRepo, ADD:
sb.WriteString("session_mode = \"append\"\n")
// VERIFIED: RenderMultiTurn is a method on provider.Manifest (render.go:203) + SessionMode is a TOML field
// (manifest.go:66) → the registry-built stub supports multi-turn. No direct-manifest seam needed.
```

```go
// stagecoach_test.go — the Success test skeleton (mirror TestGenerateCommit_DryRun + generate_multiturn_test.go):
func TestGenerateCommit_DryRun_MultiTurnSuccess(t *testing.T) {
	// scripted-append repo: [provider.stub] with session_mode="append"; STAGECOACH_STUB_SCRIPT call-varying
	// (one-shot returns "" × MaxDuplicateRetries+1 to exhaust; multi-turn turns return chunks; final returns msg).
	repo := setupScriptedAppendRepo(t, "initial", []string{"", "", "", "feat: multi-turn result"})
	chdir(t, repo)
	// stage a LARGE diff (~60 lines ⇒ EstimateTokens ≫ 50) + set multi_turn_chunk_tokens=50, multi_turn_fallback=true
	// via the repo-local .stagecoach.toml [generation] block (or Options.Config).
	stageLargeDiff(t, repo) // strings.Builder loop, mirror generate_multiturn_test.go
	ctx := context.Background()
	res, err := GenerateCommit(ctx, Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun: %v (want nil — multi-turn should win, Issue 1 fix)", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want \"\" (dry-run)", res.CommitSHA)
	}
	if res.Subject == "" {
		t.Errorf("Subject empty — multi-turn message did not land")
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — pure control-flow port + tests; no new dep. go mod tidy MUST be a no-op.

PACKAGE EDGES:
  - pkg/stagecoach → internal/generate (generate.Run/ChunkCount/FinalizeMessage/IsDuplicate/ExtractSubject/RescueError),
    internal/git (StagedDiff/EstimateTokens), internal/prompt (BuildUserPayload), internal/signal (SetCandidate).
    ALL already imported in stagecoach.go — NO new import edge. NO new dep.

FROZEN / NOT-EDITED:
  - internal/generate/* (the reference gate in CommitStaged + multiturn.go's Run/ChunkCount are INPUT — frozen;
    P1.M1.T3.S3/P1.M1.T2.S1/P1.M1.T3.S1/P1.M1.T1.S1 own them).
  - internal/hook/exec.go (P1.M3.T1.S1/S2 own the hook-path multi-turn propagation — separate scope).
  - internal/provider/render.go + manifest.go (RenderMultiTurn + SessionMode are the render surface — INPUT).
  - The runPipeline dry-run success early-return + commit tail (UNCHANGED — the gate only sets msg/success).

DOWNSTREAM CONSUMER (do NOT implement here):
  - P1.M3 (hook exec propagation) is the analogous port into internal/hook/exec.go (separate subtask).
  - P1.M4.T1 (Mode B docs sync) updates docs/how-it-works.md (multi-turn now applies to dry-run) + README — NOT this task.

NO DATABASE / NO ROUTES / NO CONFIG SCHEMA CHANGE (the gate reads existing cfg.MultiTurnFallback/MultiTurnChunkTokens/
TokenLimit/Timeout — all already plumbed by P1.M1.T2) / NO CLI WIRING / NO NEW FILES.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w pkg/stagecoach/stagecoach.go pkg/stagecoach/stagecoach_test.go
test -z "$(gofmt -l pkg/stagecoach/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./...          # Expect zero diagnostics (catches an undefined symbol → wrong package prefix, e.g. chunkPayload vs generate.ChunkCount).
go build ./...        # Whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm ZERO new imports in stagecoach.go (the gate reuses fmt/os/time/git/prompt/generate/signal already present):
git diff pkg/stagecoach/stagecoach.go | grep -E '^\+.*"(fmt|os|time)"|^\+.*internal/(git|prompt|generate|signal)' && echo "UNEXPECTED new import (re-check)" || echo "no new imports (good)"
# Confirm the gate landed + the rescue is preserved:
grep -n 'falling back to multi-turn' pkg/stagecoach/stagecoach.go   # the Fprintf (1 hit inside the gate)
grep -n 'generate.ChunkCount\|generate.Run(' pkg/stagecoach/stagecoach.go   # cross-package calls
grep -A2 'if !success {' pkg/stagecoach/stagecoach.go | grep 'generate.RescueError'   # the byte-identical rescue still present
# Expected: clean. If `go vet`/`go build` fails on an undefined `chunkPayload`/`Run`/`FinalizeMessage`, add the
#   `generate.` prefix. If it fails on an unused import, REMOVE the import you wrongly added (none should be needed).
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./pkg/stagecoach/... -v   # the 4 new DryRun_MultiTurn* tests + every existing dry-run/generate test stays green
go test -race ./...                    # full module — NO regression (internal/generate/provider/config untouched).
# Expected: all PASS. Key new assertions:
#   TestGenerateCommit_DryRun_MultiTurnSuccess       → err==nil, CommitSHA=="", Subject set (Issue 1 FIX: previously rescued).
#   TestGenerateCommit_DryRun_MultiTurnSkipped_NonAppend → err!=nil (rescue) — gate skipped (condition d false).
#   TestGenerateCommit_DryRun_MultiTurnSmallPayloadSkip  → err!=nil (rescue) — condition b false.
#   TestGenerateCommit_DryRun_MultiTurnMidTurnFailure    → err!=nil (RescueError) — FR-T7 turn failure → rescue.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only pkg/stagecoach/{stagecoach.go,stagecoach_test.go} changed:
git diff --name-only | grep -Ev 'pkg/stagecoach/(stagecoach|stagecoach_test)\.go' && echo "UNEXPECTED file changed" || echo "only listed files changed (good)"
# Confirm the reference gate (internal/generate) + hook path are byte-unchanged (frozen inputs / other scope):
git diff --exit-code -- internal/generate internal/hook && echo "generate + hook UNCHANGED (expected)"
# Optional manual smoke (requires a real append-mode provider like pi): on a repo with a large staged diff +
# session_mode="append", `stagecoach --dry-run` prints the "↳ falling back to multi-turn" line and returns a
# message (exit 0); before S2 it rescued (exit 1). (The in-package stagecoach_test covers this without a real agent.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Parity audit: the runPipeline gate should be byte-identical to CommitStaged's gate modulo (a) chunkPayload→
# generate.ChunkCount, (b) generate./signal. prefixes, (c) generate.RescueError/ErrRescue (runPipeline style).
diff <(sed -n '/FR-T1 multi-turn fallback trigger gate/,/if !success {/p' internal/generate/generate.go) \
     <(sed -n '/FR-T1 multi-turn fallback trigger gate/,/if !success {/p' pkg/stagecoach/stagecoach.go) || true
# Eyeball the diff: only the 3 expected transformations differ; the 4 conditions, FR-T12 re-capture, Issue 4
# mtPayload, Issue 3 Fprintf, dedupe, and failure→rescue are identical.
# golangci-lint: `make lint` (project-wide gate).
# FR49 belt-and-suspenders: confirm runPipeline now references MultiTurnFallback/multiturn (it had ZERO before):
grep -c 'MultiTurnFallback\|generate.Run\|generate.ChunkCount' pkg/stagecoach/stagecoach.go   # ≥3 (the gate is wired)
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `gofmt -l pkg/stagecoach/`, `go vet ./...`, `go build ./...`, `go mod tidy` no-op;
      `git diff --exit-code go.mod go.sum` empty; ZERO new imports in stagecoach.go.
- [ ] Level 2 green: `go test -race ./pkg/stagecoach/...` (4 new + existing) AND `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only `pkg/stagecoach/{stagecoach.go, stagecoach_test.go}`
      changed; internal/generate + internal/hook byte-unchanged.

### Feature Validation

- [ ] The FR-T1 gate is present in runPipeline inside `if !success {`, with all 4 conditions (a implicitly true
      at !success; b = EstimateTokens > MultiTurnChunkTokens; c = MultiTurnFallback; d = *SessionMode=="append").
- [ ] The gate uses `generate.ChunkCount`/`generate.Run`/`generate.FinalizeMessage`/`generate.IsDuplicate`/
      `generate.ExtractSubject`/`signal.SetCandidate`; rebuilds `mtPayload` from `diff` (Issue 4); re-captures
      with TokenLimit=0 when cfg.TokenLimit!=0 (FR-T12); prints the Issue 3 chunk-tokens Fprintf.
- [ ] On a multi-turn win: msg/success set → the dry-run success early-return fires (CommitSHA="", Subject set).
- [ ] On condition-fail / multi-turn failure / duplicate: byte-identical `&generate.RescueError{...}` rescue.
- [ ] The 4 named tests exist and pass (Success / Skipped-NonAppend / SmallPayloadSkip / MidTurnFailure).

### Code Quality Validation

- [ ] The gate is a faithful port of CommitStaged's proven gate (parity audit in Level 4 shows only the 3 expected
      prefix/format transformations differ).
- [ ] No scope creep into internal/generate (frozen reference) or internal/hook (P1.M3) or the dry-run success /
      commit tail (unchanged).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] A brief inline comment on the gate (ported from CommitStaged; PRD §9.24; Issue 3/4/FR-T12 noted) — Mode A.
- [ ] No docs/*.md edits (the cross-cutting how-it-works/README sync is P1.M4.T1, Mode B).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't re-implement the gate from memory. PORT IT VERBATIM from `resolution_strategy.md` ISSUE 1 Edit 3 (or
  `research/s2_runpipeline_gate_map.md` §3). The 4 conditions, FR-T12 re-capture, Issue 4 mtPayload, Issue 3 Fprintf,
  and the dedupe/failure branches are all already correct in the reference — re-implementing risks dropping one.
- ❌ Don't read the hoisted `payload` in the gate. The gate rebuilds `mtPayload` from `diff` (Issue 4). Using
  `payload` would reintroduce the retryInstr-preamble-leak bug (P1.M1.T2.S1 fixed it in CommitStaged). The hoist
  (S1) is structurally harmless; the gate is independent of it.
- ❌ Don't use the unexported `chunkPayload` or unprefixed `Run`/`FinalizeMessage`/`IsDuplicate`/`ExtractSubject`.
  runPipeline is package stagecoach — use `generate.ChunkCount`, `generate.Run`, `generate.FinalizeMessage`,
  `generate.IsDuplicate`, `generate.ExtractSubject`, `signal.SetCandidate`. (`go build` will catch a miss.)
- ❌ Don't add imports or change go.mod. fmt/os/time/git/prompt/generate/signal are ALL already imported (verified).
  An unused import fails `go vet`; a new dep is wrong (the gate needs none).
- ❌ Don't remove or alter the rescue return. It must fire byte-identically (`generate.ErrRescue`/TreeSHA/ParentSHA/
  Candidate/Cause) when the gate doesn't win. Wrap it: gate first, then `if !success { return rescue }`.
- ❌ Don't touch the dry-run success early-return or the commit tail. The gate only sets msg/success; those paths
  fire unchanged on a multi-turn win. Editing them is out of scope and risks the dry-run semantics.
- ❌ Don't look for `appendScriptManifest` in pkg/stagecoach/stagecoach_test.go — it's in internal/generate (direct-
  manifest for CommitStaged). pkg/stagecoach tests register the append stub via TOML (`session_mode = "append"` in
  `[provider.stub]`), VERIFIED to work because RenderMultiTurn is a method on provider.Manifest + SessionMode is a
  TOML field.
- ❌ Don't edit `internal/generate/*` (the reference gate is a frozen INPUT) or `internal/hook/exec.go` (P1.M3's
  scope). This task is `pkg/stagecoach/{stagecoach.go, stagecoach_test.go}` ONLY.
- ❌ Don't change go.mod/go.sum or add new files. One gate insertion + tests, two files.
- ❌ Don't skip `go vet`/`go build`/`go test -race ./pkg/stagecoach/...`/`go test -race ./...` — they catch a wrong
  package prefix (chunkPayload vs generate.ChunkCount), an unused import, a broken rescue, and a regression in the
  existing dry-run path. The 4 new tests pin the Issue 1 fix (dry-run × multi-turn).
