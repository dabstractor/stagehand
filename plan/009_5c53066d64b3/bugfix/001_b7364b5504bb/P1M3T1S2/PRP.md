---
name: "P1.M3.T1.S2 — Insert FR-T1 multi-turn gate into hook.Run preserving FR-H5 never-block (Issue 2: hook exec path multi-turn propagation) — PRD §9.24 FR-T1 / §9.20 FR-H5"
description: |

  Land the SECOND subtask of Hook Exec Multi-Turn Propagation (P1.M3.T1, Issue 2): insert the FR-T1 multi-turn
  trigger gate into `internal/hook/exec.go`'s `Run`, between the one-shot generate→parse→dedupe loop and the
  exhaustion error return. The hook path is the THIRD of three duplicated generation loops (CommitStaged has
  the gate; runPipeline gets it via the parallel P1.M2.T1.S2); without this, a `git commit` (via the installed
  prepare-commit-msg hook) on a large diff with an append-mode provider silently never falls back to
  multi-turn — Issue 2.

  S1 (P1.M3.T1.S1) is the REFACTOR PREREQUISITE (being implemented in parallel): it binds `resolved :=
  deps.Manifest.Resolve()` (exposing `resolved.SessionMode`, which the inline form discarded) and hoists
  `var payload string` to function scope. S2 CONSUMES `resolved` (+ `diff`, `sysPrompt`, `msgModel`,
  `msgReasoning`, `recent`, `rejected`, `msgFile` — all already in scope) and inserts the gate. S2 does NOT
  re-do S1's edits; it assumes they have landed.

  The gate is a FAITHFUL PORT of the canonical reference gate in `internal/generate/generate.go:300-360`
  (CommitStaged's FR-T1 gate), which ALREADY carries the Issue 3 fix (the progress line prints the per-chunk
  token estimate: `"↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n"`) and the
  Issue 4 fix (`mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)` — rebuilt from `diff`,
  NOT the one-shot `payload`). S2 mirrors it with FOUR hook-specific adaptations:
    1. `generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens)` (EXPORTED) replaces `len(chunkPayload(...))`
       (unexported — unreachable from package hook). `turns := generate.ChunkCount(...) + 1`.
    2. `generate.Run(...)` (EXPORTED) replaces the unexported `Run`. Same `(msg, ok, cause)` return.
    3. Nil-guard `deps.Verbose` for the `VerboseWarn` trigger (`if deps.Verbose != nil`) — the hook's existing
       style; CommitStaged assumes Verbose non-nil, the hook does NOT (tests pass Verbose nil).
    4. NO `signal.SetCandidate` / NO `RescueError` (the hook has no rescue — git owns the commit). On success →
       `return WriteMessageFile(msgFile, finalMsg)` (the ONLY write site); on ANY failure → fall through to the
       existing exhaustion error return.

  ⚠️ **#1 — FR-H5 never-block holds in EVERY gate outcome (the load-bearing guarantee).** The gate writes the
      msg-file ONLY on full success (`cause==nil && ok2 && !duplicate`). Every other outcome — cause!=nil (a
      turn errored/timed out), ok2==false (final parse empty), OR duplicate subject — falls through to the
      existing `return fmt.Errorf("stagecoach: hook generation failed after %d retries", ...)`, which the cmd
      layer's `neverBlock` closure (internal/cmd/hookexec.go:69-72, invoked at L137-138) maps to exit 0 + one
      stderr line + an UNTOUCHED msg-file (or exit 1 if --strict). The msg-file is byte-identical to its
      pre-Run content unless the gate returns from WriteMessageFile. See research §3.

  ⚠️ **#2 — Add `"time"` to exec.go's imports.** exec.go does NOT import `time` today (context/errors/fmt/os/
      strings + 5 internal). The progress line computes `totalMin := int((cfg.Timeout * time.Duration(turns))
      .Minutes())` — needs `time`. This is the ONLY import change. See research §4.

  ⚠️ **#3 — Mirror the reference gate VERBATIM for the mtPayload logic (Issue 4 + FR-T12).** `mtPayload` is
      ALWAYS rebuilt from `diff` via `prompt.BuildUserPayload(diff, cfg.Context, rejected)` (NOT reused from
      the one-shot `payload`, which may carry the retryInstr corrective preamble from a failed parse). When
      `cfg.TokenLimit != 0`, RE-CAPTURE via `deps.Git.StagedDiff(... TokenLimit: 0 ...)` and rebuild. Do NOT
      "simplify" by reusing the hoisted `payload` — that re-introduces Issue 4. See research §1/§2.

  ⚠️ **#4 — Do NOT edit the reference gate (generate.go) or the cmd layer (hookexec.go).** S2 MIRRORS
      generate.go:300-360 (the canonical gate, already fixed for Issues 3/4); it does not modify it. The cmd
      `neverBlock` already maps the exhaustion error correctly — no cmd change. See research §0.

  ⚠️ **#5 — The hook needs its OWN `appendScriptManifest` test helper.** `appendScriptManifest` (generate_
      test.go:857) is LOCAL to the generate test package (not exported); it is `stubtest.NewScript(...)` +
      `m.SessionMode = &"append"`. Replicate this 5-line helper in exec_test.go for the multi-turn tests.
      See research §7.

  ⚠️ **#6 — No new deps; go.mod UNCHANGED.** The gate uses already-imported symbols + stdlib `time`. `go mod
      tidy` is a no-op.

  Deliverable: MODIFIED `internal/hook/exec.go` (add `"time"` import + insert the FR-T1 gate before the
  exhaustion return) + MODIFIED `internal/hook/exec_test.go` (a local `appendScriptManifest` helper + 4 new
  tests). NO other file. OUTPUT: `hook.Run` has the FR-T1 gate; a hook commit on a large diff with an
  append-mode provider activates multi-turn and writes the generated message on success; on any failure FR-H5
  never-block is preserved (exit 0, msg-file untouched). `go build/vet/test ./...` green.

---

## Goal

**Feature Goal**: Wire the FR-T1 multi-turn fallback trigger into the git-hook generation path
(`internal/hook/exec.go::Run`) so that `git commit` (via the installed `prepare-commit-msg` hook) on a large
diff with an append-mode provider falls back to a lossless N+1-turn session — closing the Issue-2 functional
gap (the hook path was the only one of three generation loops without the gate). Preserve FR-H5 never-block:
the msg-file is written ONLY on full success; every failure outcome (turn error, empty final parse,
duplicate subject) falls through to the existing exhaustion error, which the cmd layer maps to exit 0 + an
untouched msg-file.

**Deliverable** (MODIFIED files only):
1. **MODIFIED `internal/hook/exec.go`** — add `"time"` to the import block; insert the FR-T1 gate between the
   generate→parse→dedupe loop and the `return fmt.Errorf("stagecoach: hook generation failed after %d
   retries", ...)` line. The gate mirrors `generate.go:300-360` (CommitStaged) with the 4 hook adaptations
   (generate.ChunkCount; generate.Run; nil-guard deps.Verbose; WriteMessageFile-on-success / fall-through-on-
   failure; no signal/rescue).
2. **MODIFIED `internal/hook/exec_test.go`** — add a local `appendScriptManifest` helper + 4 tests:
   `TestRun_MultiTurnSuccess_WritesMessageFile`, `TestRun_MultiTurnFailure_NeverBlock`,
   `TestRun_MultiTurnSkipped_NonAppend`, `TestRun_MultiTurnSmallPayloadSkip`.

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean; a hook
`Run` on a large diff + append-mode provider activates multi-turn and writes the generated message to the
msg-file on success; on multi-turn failure the msg-file is byte-identical to its pre-Run content (FR-H5);
non-append providers and small payloads skip the gate (existing exhaustion error); generate.go + hookexec.go
byte-unchanged; go.mod/go.sum byte-unchanged.

## User Persona

**Target User**: A user who runs `git commit` (firing stagecoach's installed `prepare-commit-msg` hook) on a
large diff with an append-mode provider (e.g. pi). Today the hook's one-shot generation exhausts and the
commit falls through to an empty editor (FR-H5 never-block). After S2, the hook falls back to multi-turn and
actually delivers a message for the large diff — the same headline benefit CommitStaged already provides.

**Use Case**: `git commit` with a large staged diff + pi configured → the hook's one-shot loop exhausts → the
FR-T1 gate fires (conditions a–d) → multi-turn session delivers the message → `WriteMessageFile` writes it at
the top of git's message file → the commit proceeds with the generated message. On any failure, the msg-file
is untouched and git opens an empty editor (FR-H5).

**User Journey**: (hook path) `git commit` → `prepare-commit-msg` → `stagecoach hook exec <msg-file>` →
`hook.Run` → one-shot loop exhausts → FR-T1 gate (S2) → `generate.Run` (multi-turn) → `WriteMessageFile` →
git uses the message. On failure: → exhaustion error → cmd `neverBlock` → exit 0 + untouched msg-file → git
opens an empty editor.

**Pain Points Addressed**: Issue 2 — the hook path silently lacks multi-turn, so large-diff hook commits
always fall through to an empty editor even when the provider could deliver via multi-turn. S2 makes the hook
path's behavior match CommitStaged's.

## Why

- **Closes the Issue-2 functional gap (Bug-Fix PRD §h2.0/§h2.3).** The multi-turn fallback landed in
  CommitStaged but was never propagated to the hook path. S2 ports it, so all three generation loops
  (CommitStaged / runPipeline / hook.Run) have the FR-T1 gate.
- **Satisfies PRD §9.24 FR-T1 (the trigger gate) on the hook path.** Conditions a–d (MultiTurnFallback;
  payload > one chunk; provider is multi-turn-capable; SessionMode=="append") gate the fallback.
- **Preserves PRD §9.20 FR-H5 (never-block) by construction.** The gate writes ONLY on success; every
  failure falls through to the exhaustion error the cmd layer already maps to exit 0 + untouched. No new
  failure mode is introduced.
- **Faithful port, minimal blast radius.** The gate is a copy of the canonical, already-fixed reference
  (Issue 3/4), with four small hook adaptations. No new deps, no config/API/CLI surface.

## What

A modified `internal/hook/exec.go` (one import + one inserted gate) and a modified `internal/hook/exec_test.go`
(a local helper + 4 tests). No new files, no new deps, no config/API/CLI/doc surface.

### Success Criteria

- [ ] `internal/hook/exec.go` adds `"time"` to the import block.
- [ ] The FR-T1 gate is inserted between the one-shot loop and the exhaustion `return fmt.Errorf(...)`. It
      checks `cfg.MultiTurnFallback && resolved.SessionMode != nil && *resolved.SessionMode == "append"`;
      builds `mtPayload` from `diff` (Issue 4); re-captures with `TokenLimit:0` when `cfg.TokenLimit != 0`
      (FR-T12); checks `git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens`; prints the progress line
      (Issue 3 format) + nil-guarded `VerboseWarn`; calls `generate.Run(...)`; on `cause==nil && ok2`:
      `FinalizeMessage` → `IsDuplicate` → if not dup: `return WriteMessageFile(msgFile, finalMsg)`.
- [ ] On ANY failure (cause!=nil, ok2==false, duplicate): the gate falls through to the existing exhaustion
      error return (NO WriteMessageFile, NO signal/rescue).
- [ ] `TestRun_MultiTurnSuccess_WritesMessageFile` — multi-turn fires → msg-file written with the message.
- [ ] `TestRun_MultiTurnFailure_NeverBlock` — multi-turn fails → exhaustion error + msg-file byte-identical.
- [ ] `TestRun_MultiTurnSkipped_NonAppend` — SessionMode unset → gate skips → exhaustion error.
- [ ] `TestRun_MultiTurnSmallPayloadSkip` — payload ≤ chunkTokens → gate skips → exhaustion error.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean; generate.go + hookexec.go +
      go.mod/go.sum byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the copy-ready gate code
(Blueprint §1), the `time` import note, the 4 hook adaptations (§1 of research), the FR-H5 fall-through
guarantee (§3), the 4 tests + the local helper (Blueprint §2), and the variable-scope confirmation (S1
exposes `resolved`; all other gate-reads are already in scope). No snapshot/decompose knowledge required —
S2 is a gate insertion that mirrors the reference.

### Documentation & References

```yaml
# MUST READ — the design calls (the 4 adaptations, FR-H5, the test plan)
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M3T1S2/research/design-decisions.md
  why: §0 (scope: exec.go + tests; generate.go/cmd frozen), §1 (the 4 hook adaptations), §2 (copy-ready gate
       code), §3 (FR-H5 in every outcome — the load-bearing guarantee), §4 (`time` import), §5 (variables in
       scope after S1), §6 (Verbose nil-safety), §7 (the 4 tests + local helper), §8 (no new deps).
  critical: §1 (generate.ChunkCount not chunkPayload; nil-guard Verbose; no signal/rescue; WriteMessageFile/
       fall-through), §3 (FR-H5), §3 (Issue 4: mtPayload from diff not payload) are the things most likely
       to go wrong.

# MUST READ — the S1 CONTRACT (the refactor prerequisite S2 builds on)
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M3T1S1/PRP.md
  why: S1 binds `resolved := deps.Manifest.Resolve()` (exposes resolved.SessionMode) + hoists `var payload
       string` + L158 `:=`→`=`. S2 assumes these have landed and reads `resolved` at the gate.
  critical: S2 does NOT re-do S1's edits; it inserts the gate AFTER the loop, reading `resolved` (and `diff`,
       `sysPrompt`, etc. already in scope). The hoisted `payload` is NOT read by the gate (Issue 4 rebuilds
       mtPayload from `diff`).

# MUST READ — the CANONICAL reference gate (the code S2 mirrors VERBATIM for the mtPayload/progress logic)
- file: internal/generate/generate.go
  section: CommitStaged's FR-T1 gate, L300-360 — the outer `if cfg.MultiTurnFallback && resolved.SessionMode
           != nil && *resolved.SessionMode == "append"`; `mtPayload := prompt.BuildUserPayload(diff,
           cfg.Context, rejected)` (Issue 4); the TokenLimit!=0 re-capture (FR-T12); `git.EstimateTokens(
           mtPayload) > cfg.MultiTurnChunkTokens`; the progress line (Issue 3: `"... %d turns (chunks of ~%d
           tokens), ~%dm total\n"`); `deps.Verbose.VerboseWarn(...)`; `Run(...)`; on success `FinalizeMessage`
           → dedupe.
  why: the EXACT logic S2 ports. The reference ALREADY has the Issue 3 + Issue 4 fixes (landed by
       P1.M1.T2/T3) — copy them; do NOT re-derive.
  critical: copy the mtPayload logic + progress line VERBATIM. The 4 hook adaptations (ChunkCount, generate.
       Run, nil-guard Verbose, WriteMessageFile/fall-through) are the ONLY differences.

# THE FILE BEING MODIFIED — READ the loop + exhaustion return before editing
- file: internal/hook/exec.go
  section: `Run` — the imports (NO `time`); Step F (`resolved` after S1; `msgModel`/`msgReasoning`); Step G
           var block (`rejected`, `parseFail`, hoisted `payload` after S1); the loop (Render/Execute/
           ParseOutput/FinalizeMessage/dedupe/WriteMessageFile); the exhaustion `return fmt.Errorf(
           "stagecoach: hook generation failed after %d retries", cfg.MaxDuplicateRetries)` — the INSERTION
           POINT (the gate goes immediately before this return).
  why: the EXACT insertion anchor + the imports to amend. Confirms all gate-read variables are in scope.
  critical: insert the gate BEFORE the exhaustion return; do NOT touch the loop body or the never-block
       returns (timeout/cancel). Add `"time"` to the import block.

# The FR-H5 preservation mechanism (read-only — confirms no cmd change needed)
- file: internal/cmd/hookexec.go
  section: `neverBlock` (L69-72) — maps a non-ErrNoOp error → `exitcode.New(exitcode.Error, nil)` (exit 0, or
           exit 1 if --strict); L137-138 `return neverBlock(rerr)`.
  why: confirms the exhaustion error the gate falls through to is ALREADY mapped to exit 0 + untouched
       msg-file. S2 changes NO cmd code.
  critical: do NOT edit hookexec.go — the neverBlock closure already preserves FR-H5 for the exhaustion path.

# The exported multiturn API S2 calls
- file: internal/generate/multiturn.go
  section: `func ChunkCount(payload string, chunkTokens int) int` (L96 = `len(chunkPayload(...))`, the N
           chunks); `func Run(ctx, deps, cfg, manifest, sysPrompt, payload, model, reasoning) (msg string,
           ok bool, cause error)` (L145).
  why: confirms the exact signatures. `turns := generate.ChunkCount(...) + 1` (N chunks + 1 final turn).
  critical: ChunkCount returns N (chunks), NOT N+1 — the `+ 1` for the final turn is in the gate.

# The test patterns to mirror
- file: internal/hook/exec_test.go
  section: `initTempRepo`/`runGit`/`mustWriteFile` helpers; `stubtest.Build`/`stubtest.Manifest(bin, Options{
           Exit,Out,SleepMS})`; the never-block assertion (`err!=nil && os.ReadFile(msgFile)==orig`) at
           TestRun_StubExit1_NeverBlock (L237) / TestRun_TimeoutNeverBlock (L264).
  why: the hook test idiom S2's 4 tests mirror.
  critical: the never-block test asserts BOTH `err!=nil` AND the msg-file is byte-identical to pre-Run content.
- file: internal/generate/generate_test.go
  section: `appendScriptManifest(t, bin, responses)` (L857) — the 5-line helper to REPLICATE in exec_test.go
           (`stubtest.NewScript(...)` + `m.SessionMode = &"append"`).
  why: the multi-turn mock pattern (an append-mode scripted provider). It is NOT exported ⇒ the hook test
       needs its own copy.
- file: internal/generate/generate_multiturn_failure_test.go
  section: TestCommitStaged_MultiTurnMidTurnFailureRescue / _SmallPayloadSkip / _NonAppendSkip — the proven
           mock + assertion patterns for the failure/skip cases.
  why: the model for TestRun_MultiTurnFailure_NeverBlock + the two skip tests.

# The bug context (in your context as selected_prd_content)
- file: plan/009_…/bugfix/001_b7364b5504bb/prd_snapshot.md (Bug-Fix PRD)
  section: §h2.0 Overview + §h2.3 Issue 2 (hook exec path lacks multi-turn) + §h3.1 Issue 3 + §h3.3 Issue 4.
  critical: the gate must preserve FR-H5 (§9.20) in every outcome — that is the hook's defining contract.
```

### Current Codebase tree (relevant slice)

```bash
internal/hook/
  exec.go           # Run — EDIT (add "time" import + insert the FR-T1 gate before the exhaustion return)
  exec_test.go      # EDIT (local appendScriptManifest helper + 4 new tests)
internal/generate/
  generate.go       # CommitStaged FR-T1 reference gate (L300-360) — UNCHANGED (the code S2 mirrors)
  multiturn.go      # ChunkCount (L96) + Run (L145) — UNCHANGED (the exported API S2 calls)
internal/cmd/
  hookexec.go       # neverBlock (L69) — UNCHANGED (already maps exhaustion → exit 0 + untouched)
go.mod / go.sum     # UNCHANGED (stdlib `time` only)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. TWO in-place edits: internal/hook/exec.go (import + gate) + internal/hook/exec_test.go (helper + tests).
```

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (#1 — FR-H5: write msg-file ONLY on success): the gate returns WriteMessageFile(msgFile, finalMsg)
//   ONLY when cause==nil && ok2 && !duplicate. EVERY other outcome falls through to the exhaustion error
//   return (the cmd neverBlock maps it to exit 0 + untouched). Do NOT add a candidate/rescue path. (research §3)

// CRITICAL (#2 — add "time" to exec.go imports): exec.go does NOT import time today. The progress line's
//   totalMin uses time.Duration(turns). Without the import it won't compile. The ONLY import change. (research §4)

// CRITICAL (#3 — mtPayload from `diff`, NOT the hoisted `payload`): Issue 4 — the one-shot `payload` may
//   carry the retryInstr corrective preamble from a failed parse; multi-turn has its own priming preamble.
//   Rebuild mtPayload via prompt.BuildUserPayload(diff, cfg.Context, rejected). Do NOT reuse `payload`. (research §1/§3)

// CRITICAL (#4 — generate.ChunkCount, not chunkPayload): chunkPayload is UNEXPORTED (package generate). Use
//   generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) (exported wrapper = len(chunkPayload(...))).
//   turns := generate.ChunkCount(...) + 1 (N chunks + 1 final turn). (research §1)

// CRITICAL (#5 — nil-guard deps.Verbose for VerboseWarn): the hook nil-guards Verbose (CommitStaged does
//   not). `if deps.Verbose != nil { deps.Verbose.VerboseWarn(...) }`. generate.Run is Verbose-nil-safe
//   (Execute nil-guards). (research §1/§6)

// GOTCHA (no signal/rescue in the hook): the hook returns the plain exhaustion error (or WriteMessageFile),
//   NOT a RescueError. Do NOT call signal.SetCandidate (no signal package in the hook). Duplicate → fall
//   through (one-shot parity; NOT rescue).
// GOTCHA (insertion anchor): insert the gate immediately BEFORE `return fmt.Errorf("stagecoach: hook
//   generation failed after %d retries", cfg.MaxDuplicateRetries)` (the post-loop exhaustion return). After
//   S1's edits the line number shifts ~+2; anchor on the return statement, not a line number.
// GOTCHA (do NOT edit the loop body or the never-block returns): the timeout/cancel returns
//   (`errors.Is(execErr, context.DeadlineExceeded)` → "hook generation timed out"; likewise Canceled) stay
//   UNCHANGED — they short-circuit BEFORE the gate. Only the gate is added before the exhaustion return.
// GOTCHA (the hoisted `payload` is NOT read by the gate): Issue 4 made the gate rebuild from `diff`. S1's
//   hoist is still the structural prerequisite (and future-proofing); the gate does not read `payload`.
// GOTCHA (local appendScriptManifest helper): it is NOT exported from generate; replicate the 5-line helper
//   in exec_test.go (stubtest.NewScript + m.SessionMode = &"append"). (research §7)
// GOTCHA (no new deps): the gate uses already-imported symbols + stdlib `time`. `go mod tidy` is a no-op.
```

## Implementation Blueprint

### §1. EDIT `internal/hook/exec.go` — add `"time"` + insert the FR-T1 gate

**Edit A — the import block.** Add `"time"` (alphabetical position: after `"strings"` within the stdlib
group, or per gofmt's grouping — gofmt will place it). The import block becomes:
```go
import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/generate"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/prompt"
	"github.com/dustin/stagecoach/internal/provider"
)
```

**Edit B — the gate.** Insert immediately BEFORE the post-loop exhaustion return
(`return fmt.Errorf("stagecoach: hook generation failed after %d retries", cfg.MaxDuplicateRetries)`):

```go
	// FR-T1 multi-turn fallback (PRD §9.24). The one-shot loop above exhausted; if the provider is
	// multi-turn-capable (append session mode) and the untruncated payload exceeds one chunk, retry as a
	// lossless N+1-turn session. On success the message is written to the msg-file (the ONLY write site);
	// on ANY failure (turn error, empty final parse, or duplicate subject) fall through to the exhaustion
	// error below — the cmd layer's neverBlock maps that to exit 0 + an untouched msg-file (FR-H5 always).
	// Mirrors the canonical gate in internal/generate/generate.go (CommitStaged), with hook adaptations:
	// generate.ChunkCount (exported), generate.Run (exported), nil-guarded Verbose, WriteMessageFile-on-
	// success / fall-through-on-failure, NO signal/rescue.
	if cfg.MultiTurnFallback &&
		resolved.SessionMode != nil && *resolved.SessionMode == "append" {

		// FR-T2/FR-T12 (Issue 4): mtPayload is ALWAYS rebuilt from the untruncated `diff` (NOT reused from
		// the one-shot `payload`, which may carry the retryInstr corrective preamble from a failed parse).
		// When token_limit is set (non-zero) the one-shot `diff` was truncated → RE-CAPTURE with TokenLimit=0.
		mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
		if cfg.TokenLimit != 0 {
			fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
				MaxDiffBytes:        cfg.MaxDiffBytes,
				MaxMDLines:          cfg.MaxMdLines,
				BinaryExtensions:    cfg.BinaryExtensions,
				Excludes:            deps.Excludes,
				TokenLimit:          0, // FR-T12: multi-turn ignores token_limit
				DiffContext:         cfg.DiffContextValue(),
				PromptReserveTokens: 0, // multi-turn chunking doesn't use the one-shot reserve
			})
			if derr == nil {
				mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
			}
			// On re-capture error, fall back to the (possibly-truncated) one-shot diff's payload (best-effort).
		}

		// Condition (b): the (now-untruncated) payload must exceed one chunk for multi-turn to help.
		if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {
			turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1 // N chunks + 1 final turn
			totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
			if totalMin < 1 {
				totalMin = 1
			}
			fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n",
				turns, cfg.MultiTurnChunkTokens, totalMin)

			// FR-T11 verbose trigger line (per-turn verbose is emitted inside generate.Run).
			if deps.Verbose != nil { // hook nil-guard (CommitStaged assumes non-nil; the hook does not)
				deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")
			}

			// FR-T2/FR-T4: lossless N+1-turn delivery of the UNTRUNCATED payload (FR-T12).
			msg2, ok2, cause := generate.Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)

			if cause == nil && ok2 {
				finalMsg := generate.FinalizeMessage(msg2, cfg)
				if !generate.IsDuplicate(generate.ExtractSubject(finalMsg), recent) {
					return WriteMessageFile(msgFile, finalMsg) // SUCCESS — the ONLY write site (FR-H4)
				}
				// Duplicate subject → fall through to exhaustion (FR-H5: exit 0, msg-file untouched).
			}
			// cause != nil (turn error/timeout) OR ok2==false (final parse empty) OR duplicate → fall through.
		}
	}

	return fmt.Errorf("stagecoach: hook generation failed after %d retries", cfg.MaxDuplicateRetries)
```

### §2. EDIT `internal/hook/exec_test.go` — local helper + 4 tests

```go
// appendScriptManifest builds an append-mode (SessionMode="append") scripted stub manifest: the stub
// emits `responses` sequentially across calls (one-shot + multi-turn turns). Replicates generate's
// appendScriptManifest (internal/generate/generate_test.go:857) — it is NOT exported, so the hook test
// needs its own copy. Use for the multi-turn success/small-payload tests.
func appendScriptManifest(t *testing.T, bin string, responses []string) provider.Manifest {
	t.Helper()
	m := stubtest.NewScript(t, bin, responses)
	appendMode := "append"
	m.SessionMode = &appendMode
	return m
}

// (1) SUCCESS — large diff + append provider → multi-turn fires → msg-file written with the message.
func TestRun_MultiTurnSuccess_WritesMessageFile(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)
	// A diff large enough to exceed MultiTurnChunkTokens=4 (the diff body — diff --git/+++/@@/+lines — is
	// well over 4 tokens even for one file, as in generate's TestCommitStaged_MultiTurnFallbackSuccess).
	mustWriteFile(t, filepath.Join(repoDir, "big.go"), []byte(strings.Repeat("// line\n", 20)))
	runGit(t, repoDir, "add", "big.go")

	// script[0]="" ⇒ one-shot parse-fail ⇒ exhaust (MaxDuplicateRetries=0 ⇒ 1 attempt). Then multi-turn
	// consumes ["ok","ok","feat: multi-turn win"] across its turns; the final returns the message.
	m := appendScriptManifest(t, stubBin, []string{"", "ok", "ok", "feat: multi-turn win"})
	cfg := config.Config{
		Timeout:              10 * time.Second,
		MaxDuplicateRetries:  0, // one-shot: 1 attempt (the "")
		MultiTurnFallback:    true,
		MultiTurnChunkTokens: 4, // low ⇒ the diff exceeds one chunk ⇒ condition (b) true
		TokenLimit:           0,
	}

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("# original comments\n"))

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err != nil {
		t.Fatalf("expected multi-turn success (nil err), got: %v", err)
	}
	data, _ := os.ReadFile(msgFile)
	if !strings.HasPrefix(string(data), "feat: multi-turn win") {
		t.Errorf("msg-file should start with the generated message; got:\n%s", string(data))
	}
}

// (2) FAILURE → never-block. An append-mode EXIT-1 stub: one-shot exhausts (exit 1), the gate fires, the
// multi-turn Run fails (turn exit 1 → cause!=nil) → fall through → exhaustion error; msg-file UNTOUCHED.
func TestRun_MultiTurnFailure_NeverBlock(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)
	mustWriteFile(t, filepath.Join(repoDir, "big.go"), []byte(strings.Repeat("// line\n", 20)))
	runGit(t, repoDir, "add", "big.go")

	// stubtest.Manifest (single-response, exits 1) + manually set SessionMode="append" so the gate fires.
	m := stubtest.Manifest(stubBin, stubtest.Options{Exit: 1, Out: ""})
	appendMode := "append"
	m.SessionMode = &appendMode
	cfg := config.Config{
		Timeout:              10 * time.Second,
		MaxDuplicateRetries:  0,
		MultiTurnFallback:    true,
		MultiTurnChunkTokens: 4,
	}

	msgFile := filepath.Join(t.TempDir(), "msg")
	orig := "# original comments\n"
	mustWriteFile(t, msgFile, []byte(orig))

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil {
		t.Fatal("expected exhaustion error (multi-turn failed), got nil")
	}
	if !strings.Contains(err.Error(), "hook generation failed") {
		t.Errorf("expected the exhaustion error, got: %v", err)
	}
	data, _ := os.ReadFile(msgFile)
	if string(data) != orig {
		t.Errorf("FR-H5 violated: msg-file modified on multi-turn failure; got:\n%s", string(data))
	}
}

// (3) SKIP — non-append provider. stubtest.NewScript ⇒ SessionMode nil ⇒ gate's outer if false ⇒ skip.
func TestRun_MultiTurnSkipped_NonAppend(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)
	mustWriteFile(t, filepath.Join(repoDir, "big.go"), []byte(strings.Repeat("// line\n", 20)))
	runGit(t, repoDir, "add", "big.go")

	// RAW NewScript ⇒ SessionMode nil (NOT append) ⇒ condition (d) false ⇒ gate skips.
	m := stubtest.NewScript(t, stubBin, []string{""})
	cfg := config.Config{
		Timeout:              10 * time.Second,
		MaxDuplicateRetries:  0,
		MultiTurnFallback:    true,
		MultiTurnChunkTokens: 4,
	}

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("# original comments\n"))

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil || !strings.Contains(err.Error(), "hook generation failed") {
		t.Errorf("expected the exhaustion error (non-append skip), got: %v", err)
	}
}

// (4) SKIP — small payload. Append provider but a TINY diff + huge chunkTokens ⇒ condition (b) false ⇒ skip.
func TestRun_MultiTurnSmallPayloadSkip(t *testing.T) {
	stubBin := stubtest.Build(t)
	repoDir, g := initTempRepo(t)
	mustWriteFile(t, filepath.Join(repoDir, "tiny.txt"), []byte("x\n")) // a 1-char change ⇒ tiny payload
	runGit(t, repoDir, "add", "tiny.txt")

	m := appendScriptManifest(t, stubBin, []string{""}) // SessionMode="append" (cond d true)
	cfg := config.Config{
		Timeout:              10 * time.Second,
		MaxDuplicateRetries:  0,
		MultiTurnFallback:    true,
		MultiTurnChunkTokens: 100000, // huge ⇒ EstimateTokens(payload) ≤ chunkTokens ⇒ cond (b) false
	}

	msgFile := filepath.Join(t.TempDir(), "msg")
	mustWriteFile(t, msgFile, []byte("# original comments\n"))

	err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
	if err == nil || !strings.Contains(err.Error(), "hook generation failed") {
		t.Errorf("expected the exhaustion error (small-payload skip), got: %v", err)
	}
}
```

> **NOTE — imports for exec_test.go:** the helper + tests use `provider` (for the helper return type) and
> `stubtest` (already imported) + `strings`/`time` (already imported). Add `"github.com/dustin/stagecoach/
> internal/provider"` to exec_test.go's imports if not already present (gofmt/build will flag it). Check the
> existing import block; the file already imports config/generate/git/stubtest/ui — add `provider` only if
> missing.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/hook/exec.go — add "time" import
  - ADD "time" to the stdlib import group. (gofmt will place it; the build confirms.)
  - GOTCHA: the ONLY import change.

Task 2: EDIT internal/hook/exec.go — insert the FR-T1 gate (Blueprint §1 Edit B)
  - INSERT the gate immediately BEFORE `return fmt.Errorf("stagecoach: hook generation failed after %d
      retries", cfg.MaxDuplicateRetries)`.
  - USE generate.ChunkCount (not chunkPayload); generate.Run; nil-guard deps.Verbose; WriteMessageFile on
      success; fall through on any failure. NO signal/rescue.
  - mtPayload from `diff` (Issue 4); TokenLimit!=0 re-capture (FR-T12); progress line (Issue 3) VERBATIM.
  - DO NOT touch the loop body, the timeout/cancel never-block returns, or WriteMessageFile's signature.

Task 3: EDIT internal/hook/exec_test.go — local appendScriptManifest helper + 4 tests (Blueprint §2)
  - ADD the appendScriptManifest helper (5 lines; replicate generate_test.go:857).
  - ADD TestRun_MultiTurnSuccess_WritesMessageFile, TestRun_MultiTurnFailure_NeverBlock,
      TestRun_MultiTurnSkipped_NonAppend, TestRun_MultiTurnSmallPayloadSkip.
  - ADD the "provider" import if missing.
  - GOTCHA: the failure test asserts BOTH err!=nil AND msg-file byte-identical (FR-H5).

Task 4: VERIFY
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. generate.go + hookexec.go +
      multiturn.go byte-unchanged. `go build/vet/test ./...` green. The 4 new tests pass.
```

### Implementation Patterns & Key Details

```go
// THE success outcome (the ONLY write site) — cause==nil && ok2 && !duplicate:
if cause == nil && ok2 {
	finalMsg := generate.FinalizeMessage(msg2, cfg)
	if !generate.IsDuplicate(generate.ExtractSubject(finalMsg), recent) {
		return WriteMessageFile(msgFile, finalMsg) // SUCCESS
	}
	// duplicate → fall through
}
// cause != nil || ok2==false || duplicate → fall through to the exhaustion return (FR-H5).

// THE nil-guarded verbose (hook style — CommitStaged omits the guard):
if deps.Verbose != nil { deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback") }

// THE turn count (ChunkCount = N chunks; +1 for the final turn):
turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1

// THE Issue-4 mtPayload (from `diff`, NOT the hoisted `payload`):
mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. The gate uses already-imported symbols + stdlib `time`. `go mod tidy`
      is a no-op.

PACKAGE EDGES: NONE added. exec.go stays in package hook; it calls already-imported generate/git/prompt +
      the new stdlib `time`. exec_test.go may add the `provider` import (for the helper's return type).

UPSTREAM (consume, do NOT edit):
  - S1 (P1.M3.T1.S1): `resolved` (bound) + hoisted `payload` (structural prereq; NOT read by the gate).
  - generate.go:300-360 (the reference gate — the logic S2 mirrors).
  - multiturn.go: ChunkCount (L96) + Run (L145) — the exported API.
  - prompt.BuildUserPayload, git.StagedDiff/EstimateTokens, config fields (MultiTurnFallback,
    MultiTurnChunkTokens, TokenLimit, DiffContextValue, Timeout, Context, MaxDiffBytes, MaxMdLines,
    BinaryExtensions, MaxDuplicateRetries).

DOWNSTREAM (the FR-H5 preservation — NOT this task):
  - internal/cmd/hookexec.go neverBlock (L69-72, L137-138): maps the exhaustion error → exit 0 + one stderr
        line + untouched msg-file (or exit 1 if --strict). S2 changes NO cmd code — the existing mapping
        already covers the gate's fall-through.

FROZEN/LEAVE (do NOT edit):
  - internal/generate/* (the canonical reference gate + multiturn.go — MIRRORED, not modified).
  - internal/cmd/hookexec.go (the neverBlock closure — already correct).
  - internal/provider/*, pkg/stagecoach/* (P1.M2.T1.S2's scope), internal/config/*.
  - The hook loop body (Render/Execute/ParseOutput/FinalizeMessage/dedupe/WriteMessageFile), the timeout/
    cancel never-block returns. PRD.md, go.mod, Makefile.

NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/hook/exec.go internal/hook/exec_test.go
go vet ./internal/hook/
go build ./...
grep -n '"time"' internal/hook/exec.go && echo "(time import present)"
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; build clean; "time" imported; go.mod/go.sum byte-unchanged.
```

### Level 2: The 4 new tests + no regression

```bash
go test ./internal/hook/ -v -run 'TestRun_MultiTurn'
go test ./internal/hook/
# Expected PASS — verify:
#   TestRun_MultiTurnSuccess_WritesMessageFile ... multi-turn fires; msg-file starts with the message
#   TestRun_MultiTurnFailure_NeverBlock ........ exhaustion error + msg-file BYTE-IDENTICAL (FR-H5)
#   TestRun_MultiTurnSkipped_NonAppend ......... SessionMode nil → skip → exhaustion error
#   TestRun_MultiTurnSmallPayloadSkip .......... payload ≤ chunkTokens → skip → exhaustion error
#   The existing hook tests (HappyPath/ParseFailRetry/DuplicateRejected/StubExit1_NeverBlock/Timeout/
#    NoPlumbing) — still PASS UNCHANGED.
# If Success fails (msg-file empty), the gate didn't fire — check conditions (SessionMode=append, payload >
#   chunkTokens). If NeverBlock fails (msg-file modified), the gate wrote on a failure path — FR-H5 violation.
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...   # Expect clean.
go test ./...    # Expect all PASS (the hook suite + no regression; generate/provider/config unchanged).
git diff --name-only | grep -E 'internal/hook/(exec|exec_test)\.go' && echo "(expected files)"
git diff --exit-code internal/generate internal/cmd/hookexec.go internal/provider pkg internal/config && echo "frozen packages UNCHANGED (expected)"
git diff --exit-code go.mod go.sum PRD.md && echo "go.mod/PRD UNCHANGED (expected)"
# Confirm the gate landed (and the time import):
grep -n 'MultiTurnFallback && resolved.SessionMode\|generate.ChunkCount\|generate.Run(ctx, deps\|"time"' internal/hook/exec.go
```

### Level 4: FR-H5 correctness reasoning (the never-block contract)

```bash
# The gate's FR-H5 guarantee rests on the single write site + the fall-through. Verify by reasoning + tests:
#   1. WriteMessageFile is reached ONLY via `return WriteMessageFile(msgFile, finalMsg)` inside the
#      `cause==nil && ok2 && !duplicate` branch. Every other path falls through. (TestRun_MultiTurnFailure)
#   2. The fall-through returns the EXISTING exhaustion error (unchanged) → the cmd neverBlock maps it to
#      exit 0 + untouched msg-file. NO new return/error path was added. (the byte-identical assertion)
#   3. The timeout/cancel never-block returns (in the loop) short-circuit BEFORE the gate — unchanged.
#   4. Duplicate → fall through (NOT rescue; one-shot parity) → neverBlock.
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on the edited files.
- [ ] `go test ./...` GREEN (the 4 new tests + existing hook tests + no regression).
- [ ] go.mod/go.sum byte-unchanged; `"time"` added to exec.go (the only import change).
- [ ] generate.go / multiturn.go / hookexec.go byte-unchanged; the hook loop body + never-block returns unchanged.

### Feature Validation
- [ ] The FR-T1 gate is inserted before the exhaustion return; mirrors the reference (mtPayload from `diff`,
      FR-T12 re-capture, Issue 3 progress line) with the 4 hook adaptations.
- [ ] On success: `return WriteMessageFile(msgFile, finalMsg)` (the ONLY write site).
- [ ] On any failure: fall through to the exhaustion error → cmd neverBlock → exit 0 + untouched (FR-H5).
- [ ] The 4 tests pass: success writes; failure never-blocks; non-append skips; small-payload skips.

### Code Quality Validation
- [ ] Faithful port of generate.go:300-360 (the mtPayload/progress logic copied verbatim); only the 4 hook
      adaptations differ.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (generate.go/cmd/provider/pkg frozen).

### Documentation
- [ ] Inline comments cite PRD §9.24 FR-T1, §9.20 FR-H5, the Issue 4 (mtPayload from diff) + Issue 3 (per-
      chunk estimate) + FR-T12 (token_limit non-interaction) rationale, and the neverBlock fall-through. No
      docs/*.md edits (the cross-cutting overview is P1.M4.T1's Mode-B sweep).

---

## Anti-Patterns to Avoid

- ❌ **Don't write the msg-file on any failure path.** WriteMessageFile is reached ONLY in the
      `cause==nil && ok2 && !duplicate` branch. Every failure (cause!=nil, ok2==false, duplicate) falls
      through to the exhaustion error → cmd neverBlock → exit 0 + untouched. (FR-H5; research §3)
- ❌ **Don't reuse the hoisted `payload` for mtPayload.** Issue 4: rebuild from `diff` via BuildUserPayload
      (the one-shot `payload` may carry the retryInstr preamble). (research §1/§3)
- ❌ **Don't use `chunkPayload` (unexported) or `Run` (unexported).** Use `generate.ChunkCount` and
      `generate.Run` (both exported). `turns := generate.ChunkCount(...) + 1`. (research §1)
- ❌ **Don't omit the `deps.Verbose` nil-guard.** CommitStaged assumes Verbose non-nil; the hook does NOT
      (tests pass nil). `if deps.Verbose != nil { ... VerboseWarn(...) }`. (research §1/§6)
- ❌ **Don't add a signal/rescue path.** The hook has no rescue (git owns the commit). No `signal.
      SetCandidate`, no `RescueError`. Duplicate → fall through (NOT rescue). (research §1)
- ❌ **Don't edit generate.go (the reference gate) or hookexec.go (the neverBlock).** S2 MIRRORS the
      reference; the cmd neverBlock already maps the exhaustion error. No change to either. (research §0)
- ❌ **Don't forget the `"time"` import.** exec.go lacks it; the progress line's `time.Duration(turns)` needs
      it. The ONLY import change. (research §4)
- ❌ **Don't touch the loop body or the timeout/cancel returns.** Only the gate is added before the exhaustion
      return. The Render/Execute/ParseOutput/FinalizeMessage/dedupe/WriteMessageFile calls + the
      DeadlineExceeded/Canceled returns stay byte-identical.
- ❌ **Don't conflate the hook's duplicate handling with CommitStaged's.** CommitStaged rescues on duplicate;
      the hook falls through to exhaustion (one-shot parity — git proceeds with an empty editor). (research §3)
- ❌ **Don't replicate generate's `appendScriptManifest` by importing it.** It is NOT exported; copy the 5-line
      helper into exec_test.go. (research §7)

---

## Confidence Score

**9/10** — the gate is a faithful port of the canonical, already-fixed reference gate (generate.go:300-360,
read in full — Issue 3 per-chunk estimate + Issue 4 mtPayload-from-diff are already there), with four small,
clearly-specified hook adaptations (generate.ChunkCount; generate.Run; nil-guard Verbose; WriteMessageFile/
fall-through; no signal/rescue). The exported multiturn API (ChunkCount L96, Run L145) is confirmed; the cmd
neverBlock (hookexec.go:69) is confirmed to already map the exhaustion error → exit 0 + untouched (so FR-H5
holds by construction — the gate adds NO new failure path, only a single guarded write site). The `time`
import requirement is confirmed (exec.go lacks it). The variable-scope confirmation (S1 exposes `resolved`;
all other gate-reads are in scope) closes the "will it compile" question. The 4 tests mirror the proven
hook + generate multiturn test idioms (stubtest.Build/NewScript/Manifest, appendScriptManifest, the
byte-identical never-block assertion). The one residual risk — a test's stub script length / chunkTokens
tuning needed to make condition (b) fire exactly as intended for the success case — is covered by mirroring
generate's TestCommitStaged_MultiTurnFallbackSuccess (chunkTokens=4, a multi-line diff) and the run-until-
green validation. The -1 reserves for the local `appendScriptManifest` helper needing the `provider` import
added to exec_test.go (a one-line import the build will flag).
