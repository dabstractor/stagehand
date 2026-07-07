---
name: "P1.M1.T3.S3 — Wire FR-T1 trigger gate into CommitStaged + resolve payload scope + progress + verbose"
description: |
  Insert the multi-turn fallback trigger gate (PRD §9.24 FR-T1 a–d) into `internal/generate/generate.go`'s
  `CommitStaged`, between the one-shot retry-loop's close (`:287`) and the existing rescue return
  (`:288–292`). Three coupled edits, all in `generate.go`:
    (a) PAYLOAD SCOPE — hoist `var payload string` before the loop (`:~219`) and change `:228`
        `payload :=` → `payload =`, so the last-built payload survives to the gate (honors "do not
        recompute" — multi-turn reads the existing payload variable, NOT a fresh BuildUserPayload call).
    (b) TRIGGER GATE — nest the FR-T1 4-condition gate INSIDE `if !success { … }`, BEFORE the rescue
        return: `cfg.MultiTurnFallback && git.EstimateTokens(payload) > cfg.MultiTurnChunkTokens &&
        resolved.SessionMode != nil && *resolved.SessionMode == "append"` (condition (a) "one-shot
        exhausted" is already true at `!success`). `resolved` is the already-declared `:211` var.
    (c) MULTI-TURN BRANCH — on a passing gate: emit the FR-T5 progress line to stderr (turn count +
        total budget), emit the FR-T11 verbose trigger via `deps.Verbose.VerboseWarn`, call
        `Run(ctx, deps, cfg, deps.Manifest, sysPrompt, payload, msgModel, msgReasoning)` (SAME-PACKAGE,
        unqualified), then map `(msg2, ok2, cause)` → success (finalize+dedupe, one-shot parity) or
        fall through to the EXISTING rescue return (byte-identical struct literal, FR-T7). The rescue
        return is wrapped in a SECOND `if !success` so multi-turn success SKIPS it.
  KEY DECISIONS: (D1) hoist payload (no recompute); (D2) FR-T12 — pass the captured `payload` variable
  as-is (the contract's resolution; multi-turn does NOT re-apply token_limit); (D3) finalize msg2 BEFORE
  dedupe (one-shot parity — avoids the template-duplicate-slip bug); (D4) nested gate + second
  `if !success` preserves the rescue return byte-identical; (D5) progress line = direct stderr write
  (`Deps.Progress` is a no-arg callback that can't carry the message); (D6) verbose trigger =
  `VerboseWarn` (no ui-package change; per-turn verbose is free via provider.Execute).
  Adds 2 stdlib imports (`os`, `time`). Adds 3–4 FOCUSED tests in `generate_test.go` (multi-turn success
  commits / non-append skip → rescue / small-payload skip → rescue). The EXHAUSTIVE 4-condition truth
  table + token_limit non-interaction are P1.M1.T3.S4; the integration matrix is P1.M1.T4. NO docs (S4).
  Touches ONLY `internal/generate/generate.go` + `internal/generate/generate_test.go`. The parallel S2
  (multiturn.go) is a HARD dependency — `Run` must exist before S3 builds.
---

## Goal

**Feature Goal**: Wire the FR-T1 multi-turn fallback trigger gate into `CommitStaged` (PRD §9.24). After
the one-shot generate→parse→dedupe loop exhausts on a payload that exceeds one chunk, AND multi-turn is
enabled AND the provider supports session append, CommitStaged transparently invokes the lossless N+1-turn
protocol (`multiturn.Run`, P1.M1.T3.S2) as an additional attempt — surfacing a progress line + verbose
trigger, then either accepting the multi-turn message (finalize + dedupe, then the unchanged commit path)
or falling through to the EXISTING rescue return byte-for-byte (FR-T7). Multi-turn is strictly upside:
"never leave the run in a worse state than one-shot-exhausted."

**Deliverable** (two files MODIFIED):
1. **MODIFY** `internal/generate/generate.go`: (a) hoist `var payload string` + `:228` `:=`→`=`; (b) the
   nested FR-T1 gate inside `if !success`; (c) the multi-turn branch (progress + verbose + `Run` call +
   success/dedupe/rescue mapping) + the second `if !success` rescue guard; (d) add `os` + `time` imports.
2. **MODIFY** `internal/generate/generate_test.go`: add 3–4 focused tests for the multi-turn branch
   (success-commits / non-append-skip / small-payload-skip; optional duplicate→rescue).

No other files touched. `multiturn.go`/`multiturn_test.go` are S2's (parallel; hard dependency). No docs.

**Success Definition**: `go build/vet/gofmt` clean; `go test -race ./internal/generate/` green with the
new focused tests + all existing CommitStaged tests + S1/S2's multiturn tests passing; with all 4 FR-T1
conditions met (one-shot exhausted + payload > one chunk + multi_turn_fallback + session_mode="append"),
CommitStaged emits the progress line to stderr, emits the verbose trigger, invokes `Run`, and on a
non-duplicate multi-turn message COMMITS it (HEAD advances, Result.Message == the multi-turn message); if
ANY of the 4 conditions fails, the existing rescue return fires byte-identically (FR-T7); a multi-turn
duplicate sets Candidate and falls through to rescue (FR-T7); the in-loop rescue returns (`:246`, `:252`)
are unchanged; `multiturn.Run`/`chunkPayload`/`newSessionID`/S1's helpers are byte-identical (S2's file).

## User Persona

**Target User**: The end user running `stagecoach` against a repo with a LARGE diff (e.g. a 266K-token
working-tree change) against a provider whose per-request reliability ceiling lies below its context
window (PRD §9.24). And — transitively — the CLI default action (P1.M4.T1.S2) / hook path that calls
`CommitStaged` / `GenerateCommit`.

**Use Case**: One-shot generation exhausts its retries on empty/unparseable output (the provider choked
on the request size). Instead of immediately rescuing, CommitStaged — silently, when all four FR-T1
conditions hold — re-delivers the SAME captured payload across N+1 smaller session turns and asks for the
message at the end. The user sees a one-line progress notice ("↳ falling back to multi-turn: 5 turns,
~10m total") and, if multi-turn succeeds, a committed message where one-shot failed. If multi-turn also
fails, the rescue is identical to a plain one-shot failure (the user is no worse off).

**User Journey**: `stagecoach` → one-shot (fails after retries) → [FR-T1 gate: all 4 hold] → stderr
progress line → `multiturn.Run` (N+1 turns) → [success] FinalizeMessage + dedupe + commit → success
report; OR [failure/dup] → existing rescue message (byte-identical) → exit 3.

**Pain Points Addressed**: (1) Without the gate, a large diff that one-shot can't reliably carry goes
straight to rescue — the user must hand-write the message despite the model being ABLE to handle the
content across turns. (2) Without the payload-scope fix, the multi-turn branch can't see the captured
payload (it's loop-local) — so multi-turn could never be wired correctly. (3) Without finalize-before-
dedupe, a templated multi-turn message could slip past the duplicate check.

## Why

- **PRD §9.24 FR-T1 (Fallback, not default; gated trigger):** the one-shot path runs first, unchanged;
  multi-turn activates ONLY when ALL FOUR conditions hold. This subtask IS condition-routing: it evaluates
  (a)–(d) and either invokes `Run` or falls through to the existing rescue.
- **PRD §9.24 FR-T2 (Lossless):** the multi-turn payload is the SAME captured payload the one-shot path
  would send. The payload-scope hoist (D1) makes that payload reachable at the gate.
- **PRD §9.24 FR-T5 (Per-turn timeout; total budget surfaced):** "the CLI prints the turn count and total
  budget on the progress line at fallback time." This subtask emits that line (D5).
- **PRD §9.24 FR-T7 (Failure handling):** "Multi-turn can never leave the run in a worse state than
  one-shot-exhausted." The second `if !success` + byte-identical rescue return (D4) realize this.
- **PRD §9.24 FR-T11 (Verbose surface):** "prints … the trigger ('one-shot exhausted → multi-turn
  fallback')". This subtask emits it via VerboseWarn (D6); per-turn verbose is free via Execute.
- **PRD §9.24 FR-T12 (No interaction with token_limit):** multi-turn uses the captured payload, NOT a
  re-truncated one. D2 documents the contract's resolution.
- **Foundation for S4/T4:** S4 (exhaustive truth table + how-it-works doc) and T4 (integration matrix)
  build on a wired, testable gate.

## What

### The three coupled edits to `CommitStaged` (all in `internal/generate/generate.go`)

**(a) Payload scope (hoist):** before the loop (among the `var rejected`/`candidate`/`parseFail`/
`lastCause`/`msg` declarations, ~L219), add `var payload string`. Inside the loop, change
`payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)` (`:228`) to `payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)` (assignment, not declaration). The `if parseFail { payload = retryInstr + "\n\n" + payload }` line stays. The last-built payload now survives to the gate.

**(b)+(c) The trigger gate + multi-turn branch (replaces the bare `if !success { return &RescueError{...} }` at L287–292):**

```go
	if !success {
		// FR-T1 multi-turn fallback trigger gate (PRD §9.24). Multi-turn activates ONLY when one-shot
		// exhausted (already true here at !success) AND the captured payload exceeds one chunk AND
		// multi_turn_fallback is enabled AND the resolved manifest declares session_mode="append".
		// If any condition fails, fall through to the existing rescue (byte-identical, FR-T7).
		if cfg.MultiTurnFallback &&
			git.EstimateTokens(payload) > cfg.MultiTurnChunkTokens &&
			resolved.SessionMode != nil && *resolved.SessionMode == "append" {

			// FR-T5: surface the turn count + total wall-clock budget (timeout × turns) on the progress
			// line. Deps.Progress is a no-arg callback (can't carry the message) → direct stderr write.
			turns := len(chunkPayload(payload, cfg.MultiTurnChunkTokens)) + 1 // N chunks + 1 final turn
			totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
			if totalMin < 1 {
				totalMin = 1
			}
			fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)

			// FR-T11: verbose trigger line (per-turn verbose is emitted by provider.Execute inside Run).
			deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")

			// FR-T2/FR-T4: lossless N+1-turn delivery of the captured payload (FR-T12: NOT re-truncated).
			msg2, ok2, cause := Run(ctx, deps, cfg, deps.Manifest, sysPrompt, payload, msgModel, msgReasoning)

			if cause == nil && ok2 {
				// Dedupe the multi-turn result. §9.7 judges the FINAL subject → finalize BEFORE dedupe
				// (one-shot parity; avoids the template-duplicate-slip bug — D3).
				finalMsg := FinalizeMessage(msg2, cfg)
				signal.SetCandidate(finalMsg)
				if !IsDuplicate(ExtractSubject(finalMsg), recent) {
					msg = finalMsg
					success = true // multi-turn succeeded → skip the rescue return
				} else {
					// Duplicate → rescue with the finalized candidate (one-shot parity: candidate = m post-finalize).
					candidate = finalMsg
				}
			} else {
				// cause != nil (turn error/timeout) OR ok2==false (final parse empty) → rescue.
				if cause != nil {
					lastCause = cause // the multi-turn failure supersedes one-shot's lastCause
				}
				if msg2 != "" {
					candidate = msg2 // raw parse output (one-shot parse-fail parity: candidate = m raw)
				}
			}
		}
		if !success {
			return Result{}, &RescueError{
				Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
				Candidate: candidate, Cause: lastCause,
			}
		}
	}
```

> The rescue return struct literal (`Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA, Candidate:
> candidate, Cause: lastCause`) is BYTE-IDENTICAL to the original L288–292 (FR-T7). The only structural
> addition is the nested gate + the second `if !success` guard. The in-loop rescue returns at `:246`
> (timeout) and `:252` (cancel) are UNTOUCHED.

### Imports to add (`os`, `time`)

gofmt-sorted into the stdlib block of `generate.go`:
```go
import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/lock"
	"github.com/dustin/stagecoach/internal/prompt"
	"github.com/dustin/stagecoach/internal/provider"
	"github.com/dustin/stagecoach/internal/signal"
	"github.com/dustin/stagecoach/internal/ui"
)
```
NO new internal-package imports: `Run`/`chunkPayload` (same package), `FinalizeMessage`/`ExtractSubject`/
`IsDuplicate` (same package), `git.EstimateTokens` (already imported), `signal.SetCandidate` (already
imported). `config`/`provider`/`prompt`/`ui`/`lock` already imported.

### Success Criteria

- [ ] `var payload string` is declared before the loop; `:228` uses `=` (not `:=`); the loop body is
      otherwise unchanged.
- [ ] The FR-T1 gate is nested inside `if !success`, evaluating all 4 conditions (one-shot-exhausted is
      implicit in `!success`; `cfg.MultiTurnFallback`; `git.EstimateTokens(payload) > cfg.MultiTurnChunkTokens`;
      `resolved.SessionMode != nil && *resolved.SessionMode == "append"`).
- [ ] On a passing gate: the progress line is written to stderr; the verbose trigger fires via VerboseWarn;
      `Run(...)` is called with `(ctx, deps, cfg, deps.Manifest, sysPrompt, payload, msgModel, msgReasoning)`.
- [ ] On `cause == nil && ok2 && !duplicate`: `msg = FinalizeMessage(msg2, cfg)`, `success = true`, and
      the rescue return is SKIPPED (commit path runs unchanged).
- [ ] On duplicate: `candidate = finalMsg` (finalized); falls through to rescue.
- [ ] On `cause != nil || !ok2`: `lastCause = cause` (if cause != nil); `candidate = msg2` (if non-empty);
      falls through to rescue.
- [ ] The rescue return struct literal is byte-identical to the original; a SECOND `if !success` guards it.
- [ ] `os` + `time` added to imports; NO other import change.
- [ ] `Run`/`chunkPayload`/`newSessionID`/S1's helpers in multiturn.go are byte-identical (S2's file).
- [ ] The in-loop rescue returns (`:246` timeout, `:252` cancel) are unchanged.
- [ ] The 3–4 focused tests exist in `generate_test.go` and PASS.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] `git diff --stat` shows ONLY `internal/generate/generate.go` + `internal/generate/generate_test.go`.
- [ ] NO exhaustive truth table (S4), NO how-it-works doc (S4), NO integration matrix (T4).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim gate + branch (copy-paste-ready), the exact edit
anchors (line numbers verified by grep), the verbatim seam contracts (`Run`/`chunkPayload`/`EstimateTokens`/
`SessionMode`/`FinalizeMessage`/`ExtractSubject`/`IsDuplicate`/`VerboseWarn`), the 7 decisions (D1–D7)
with rationale, the focused test recipe (NewScript clamping + chunkTokens tuning + the one-shot-consumes-
call-1 sequencing), and the hard scope fences. No inference required.

### Documentation & References

```yaml
# MUST READ — the FR spec, the seam contracts, and this task's research
- file: PRD.md
  why: "§9.24 FR-T1 (the 4-condition gated trigger: a one-shot-exhausted, b payload>chunk, c multi_turn_
        fallback, d session_mode=append); FR-T2 (lossless — the SAME captured payload); FR-T5 (progress
        line: turn count + total budget); FR-T7 (failure → existing rescue byte-identical); FR-T11
        (verbose: the trigger line + per-turn payload/stdout/stderr); FR-T12 (NO token_limit interaction
        — multi-turn uses the untruncated captured payload). §9.7 (duplicate rejection — judges the FINAL
        subject). §13.3 (CommitStaged pipeline — the loop + rescue boundary). §18.3 (rescue message —
        Candidate is shown to the user)."
  critical: "FR-T1's 4 conditions are the gate. FR-T7's 'byte-identical rescue' is why the struct literal
             must be unchanged + a second if !success guards it (D4). FR-T12's 'untruncated payload' is
             resolved by D2 (pass the captured payload variable as-is). §9.7 'judges the final subject'
             is why D3 finalizes before dedupe."

- docfile: plan/009_5c53066d64b3/architecture/research-generate-config.md
  why: "§1a (the rescue boundary at generate.go:288 — the EXACT insertion point); §1b (the payload-scope
        tension: payload is loop-scoped at :228 with :=, NOT in scope at :288 — Option 1 hoist is the
        chosen resolution, D1); §1c (EstimateTokens is passed as a fn-value at :174, NOT yet called
        directly on the payload — the gate is its first direct call); §1d (resolved/retryInstr/msgModel/
        msgReasoning are function-scoped, available at :288)."
  critical: "§1b is THE reason this subtask exists as more than a one-line insert: payload MUST be hoisted
             or the gate can't see it. §1a pins the exact line (288) — the gate goes BETWEEN the loop
             close (287) and the rescue return."

- docfile: plan/009_5c53066d64b3/architecture/research-tests-ui.md
  why: "§1 item 10 (provider.Execute already emits per-turn verbose — VerboseCommand/VerbosePayload/
        VerboseRawOutput/VerboseStderr — so FR-T11 per-turn is FREE, no extra wiring); §1 item 13 (the ↳
        progress prefix + Progress writes to stderr in output.go — context for the progress-line format);
        §2 (the CommitStaged test template + the stub NewScript call-indexed mechanism + clamping)."
  critical: "§1 item 10 means S3 need ONLY emit the trigger line (VerboseWarn) — NOT per-turn verbose.
             §2's NewScript clamping (clamps to last line after exhaustion) is what makes the focused
             test work with an unknown N."

- docfile: plan/009_5c53066d64b3/P1M1T3S3/research/s3_trigger_gate.md
  why: "THIS subtask's research: §1 the exact edit-site line anchors; §2 the verified seam contracts;
        §3 the 4 gate conditions; §4 the 7 decisions D1–D7 (READ FIRST); §5 the focused test recipe;
        §6 the non-overlap with S2; §7 the imports."
  critical: "§4 D3 (finalize-before-dedupe, deviating from the contract's literal pseudocode for
             correctness) and §4 D2 (FR-T12 — pass the captured payload as-is) are the two non-obvious
             calls. §6 documents the HARD dependency on S2 (Run must exist)."

- docfile: plan/009_5c53066d64b3/P1M1T3S2/PRP.md
  why: "The CONTRACT for Run (the HARD dependency, Implementing in parallel): the exact Run signature
        `func Run(ctx, deps Deps, cfg config.Config, manifest provider.Manifest, sysPrompt, payload,
        msgModel, msgReasoning string) (msg string, ok bool, cause error)`; the return contract (cause
        != nil ⟺ a turn aborted, raw error NOT wrapped in *RescueError; cause==nil ⟹ (msg,ok) from
        ParseOutput, ok==false = final parse empty); the same-package call (unqualified Run)."
  critical: "S3 CALLS Run; S3 does NOT re-implement it. S3 maps Run's (msg,ok,cause) → success or rescue.
             Run returns the RAW cause; S3 sets RescueError.Cause = cause. If S2 has not landed, `go build`
             fails on the undefined Run symbol — S3 MUST wait for S2."

- docfile: plan/009_5c53066d64b3/P1M1T3S1/PRP.md
  why: "The CONTRACT for chunkPayload (LANDED): `func chunkPayload(payload string, chunkTokens int)
        []chunk` (same package). S3 calls `len(chunkPayload(payload, cfg.MultiTurnChunkTokens))` for the
        progress-line turn count. Pure string math; deterministic; safe to call (Run calls it again)."
  critical: "chunkPayload is UNEXPORTED but same-package (generate) — accessible from generate.go. N =
             len(chunks); turns = N+1. Do NOT modify chunkPayload (S1's deliverable)."

- docfile: plan/009_5c53066d64b3/P1M1T2S1/PRP.md
  why: "The config CONTRACT (LANDED): Config.MultiTurnFallback bool (default true) is the gate's condition
        (c); Config.MultiTurnChunkTokens int (default 32000) is condition (b)'s threshold AND chunkPayload's
        budget; Config.Timeout time.Duration (default 120s) is the per-turn budget AND the progress-line
        total-budget factor (Timeout × turns)."
  critical: "MultiTurnFallback is the gate's (c); MultiTurnChunkTokens is BOTH the gate threshold (b) and
             the chunk budget; Timeout drives the totalMin computation (needs the `time` import)."

- docfile: plan/009_5c53066d64b3/P1M1T1S3/PRP.md
  why: "The CONTRACT for RenderMultiTurn + SessionMode (LANDED): SessionMode *string (manifest.go:66);
        Resolve() defaults nil → strPtr(''); Validate() enforces ''/'append'. RenderMultiTurn's own
        session_mode='append' gate is Run's defense-in-depth (a non-append manifest ⇒ turn-1 render error
        ⇒ surfaced as cause)."
  critical: "S3's gate condition (d) reads resolved.SessionMode — MUST nil-check before deref
             (*resolved.SessionMode). The gate (d) is S3's primary check; RenderMultiTurn's gate is the
             secondary (inside Run). Both agree for a correct manifest."

# The edit site + the seams (READ-ONLY — consumed, not modified, except generate.go)
- file: internal/generate/generate.go
  why: "EDIT (file 1 of 2). The CommitStaged pipeline. The loop at :226; payload at :228 (loop-scoped,
        must hoist); resolved at :211 (function-scoped, reusable); the rescue return at :287–292 (the
        gate insertion point). Deps/Result/RescueError/CASError + the sentinel errors are all here."
  pattern: "Match the existing doc-comment + error-handling style. The gate is a focused insert; the
            payload hoist is a pure :=→= refactor. Use fmt.Fprintf(os.Stderr, …) for the progress line."
  gotcha: "Do NOT re-declare `resolved` (it's at :211). Do NOT touch the in-loop rescue returns (:246/:252).
           Do NOT touch the post-loop EditMessage/CommitTree/UpdateRefCAS/DiffTree path — multi-turn
           success flows into it unchanged once msg+success are set."

- file: internal/generate/multiturn.go
  why: "READ-ONLY (S2's file; HARD dependency). Run + chunkPayload + newSessionID + preambleFmt/
        finalInstruction constants. S3 CALLS Run (unqualified) and chunkPayload (unqualified) — both
        same-package (package generate)."
  gotcha: "If multiturn.go lacks `func Run` (S2 not landed), `go build ./internal/generate/` fails.
           This is the hard dependency — S3 cannot build without S2. Do NOT modify multiturn.go."

- file: internal/git/tokens.go
  why: "READ-ONLY. `func EstimateTokens(s string) int` at :25 (= ceil(runes/4)). The gate's condition (b)
        and the single token measure. Already imported in generate.go (the git package)."
  gotcha: "This is the FIRST direct EstimateTokens(payload) call in generate.go (it was previously only
           passed as a fn-value to MessageReserveTokens at :174). No new import (git already imported)."

- file: internal/provider/manifest.go
  why: "READ-ONLY. SessionMode *string at :66; Resolve() at :177 defaults nil → strPtr(''); Validate()
        at :121 enforces ''/'append'. The gate's condition (d) reads resolved.SessionMode."
  gotcha: "SessionMode is a POINTER (*string) — MUST nil-check (`resolved.SessionMode != nil`) before
           dereferencing. Resolve() guarantees non-nil after resolution, but the nil-check is the safe
           idiom and costs nothing."

- file: internal/generate/finalize.go + dedupe.go
  why: "READ-ONLY (same package). FinalizeMessage(msg, cfg) at finalize.go:37; ExtractSubject(m) at
        dedupe.go:19; IsDuplicate(subject, recent) at dedupe.go:46. All same-package — no import."
  gotcha: "FinalizeMessage applies the §9.19 template. D3: call it BEFORE IsDuplicate so the dedupe
           compares the FINAL subject (one-shot parity)."

- file: internal/ui/verbose.go
  why: "READ-ONLY. VerboseWarn(msg) at :103 — nil-safe generic one-liner ('DEBUG: '+msg). The FR-T11
        verbose trigger uses it. NO new verbose method needed (no ui-package change)."
  gotcha: "VerboseWarn is nil-safe (v==nil ⇒ no-op). There is NO dedicated multi-turn-trigger method;
           VerboseWarn is the closest fit. Per-turn verbose is emitted by provider.Execute (free)."

- file: internal/generate/generate_test.go
  why: "EDIT (file 2 of 2). The CommitStaged integration tests. TestCommitStaged_Success /
        TestCommitStaged_DedupeRetryThenSuccess / TestCommitStaged_ParseFailRescue are the templates.
        Has fixture helpers: initRepo/writeFile/stageFile/headSHA/commitRaw/gitOut/shaRe."
  pattern: "Match the existing test style (plain if/t.Errorf, no testify; stubtest.Build +
            stubtest.Manifest/stubtest.NewScript; config.Defaults())."
  gotcha: "The stub's NewScript is call-indexed AND clamps to the last line after exhaustion. The one-shot
           path consumes call 1; multi-turn consumes calls 2..(N+2). Sequence the script accordingly. Use
           cfg.MultiTurnChunkTokens small (e.g. 2–4) to keep N≥2 but bounded; default (32000) for the
           small-payload-skip test."

# External references
- url: https://pkg.go.dev/fmt#Fprintf
  why: "fmt.Fprintf(os.Stderr, format, args…) writes the formatted progress line to stderr. os.Stderr is
        the unbuffered standard-error file (the same stream ui.Progress writes to)."
  critical: "Use os.Stderr (NOT os.Stdout) for the progress line — it must appear even if stdout is
             piped/redirected (matching ui.Progress's stream). Adds the `os` import."
- url: https://pkg.go.dev/time#Duration
  why: "time.Duration(turns) converts the int turn count to a Duration for the Timeout × turns product.
        .Minutes() gives the float minutes for the totalMin computation."
  critical: "Adds the `time` import. cfg.Timeout is already a time.Duration; the product
             cfg.Timeout * time.Duration(turns) is a Duration; int((…).Minutes()) is the minute count."
```

### Current Codebase Tree (this task's scope — generate.go + generate_test.go; multiturn.go is S2's)

```bash
stagecoach/
└── internal/generate/
    ├── generate.go        # EDIT (file 1): hoist payload; nest FR-T1 gate + multi-turn branch; +os +time imports
    ├── generate_test.go   # EDIT (file 2): +3–4 focused multi-turn tests
    ├── multiturn.go       # READ-ONLY (S2's file; HARD dep — Run + chunkPayload must exist here)
    ├── multiturn_test.go  # READ-ONLY (S1/S2's tests)
    ├── rescue.go / finalize.go / dedupe.go / invariants_test.go / realagent_test.go  # READ-ONLY (siblings)
# internal/provider/{manifest,render,parse,executor}.go = READ-ONLY (the consumed seams)
# internal/git/tokens.go = READ-ONLY (EstimateTokens)
# internal/ui/verbose.go = READ-ONLY (VerboseWarn)
# internal/stubtest/* + cmd/stubagent = READ-ONLY (the test seam)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/generate/
    ├── generate.go        # + var payload hoist, + FR-T1 gate + multi-turn branch, + os/time imports
    └── generate_test.go   # + TestCommitStaged_MultiTurnFallbackSuccess, + ..._MultiTurnSkipped_NonAppend,
                           #   + ..._MultiTurnSkipped_SmallPayload (+ optional ..._MultiTurnDuplicateRescue)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/generate/generate.go` | MODIFY | (a) Hoist `var payload string` + `:228` `:=`→`=`; (b) nest the FR-T1 4-condition gate inside `if !success`; (c) the multi-turn branch (progress + verbose + Run + success/dedupe/rescue mapping) + the second `if !success` rescue guard; (d) add `os` + `time` imports. |
| `internal/generate/generate_test.go` | MODIFY | Add 3–4 focused tests for the multi-turn branch (success-commits / non-append-skip / small-payload-skip; optional duplicate→rescue). |

**Explicitly NOT touched**: `internal/generate/multiturn.go` + `multiturn_test.go` (S1/S2's), `internal/generate/{rescue,finalize,dedupe}.go` and their tests, `internal/generate/{invariants,realagent}_test.go`, `internal/provider/*` (read-only seams), `internal/git/*` (read-only; EstimateTokens), `internal/config/*` (T2 — LANDED), `internal/ui/*` (read-only; VerboseWarn), `internal/stubtest/*` + `cmd/stubagent` (read-only test seam), `internal/cmd/*` (the CLI wires progress elsewhere — not this task), any docs (S4/S5), `README.md` (S5), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — payload is LOOP-SCOPED; MUST hoist). generate.go:228 declares `payload :=` INSIDE the
// for-loop, so it is NOT in scope at the gate (:287). Hoist `var payload string` before the loop (among
// the var rejected/candidate/parseFail/lastCause/msg block) and change :228 to `payload =` (assignment).
// The `if parseFail { payload = retryInstr + "\n\n" + payload }` line stays. This is a PURE refactor
// (the loop's payload construction is unchanged except :=→=); the last-built payload now survives. (D1.)

// CRITICAL (G2 — the gate is INSIDE `if !success`; condition (a) is free). FR-T1(a) "one-shot exhausted
// its retry loop on empty/unparseable output" is ALREADY true at `!success`. So the gate need only check
// (b) payload>chunk, (c) MultiTurnFallback, (d) session_mode="append". Do NOT re-check success/!success
// inside the gate (it's the enclosing condition). (Research §3.)

// CRITICAL (G3 — resolved is ALREADY declared at :211; do NOT re-declare). `resolved := deps.Manifest.
// Resolve()` is function-scoped (declared before the loop). The gate reads `resolved.SessionMode` directly.
// A second `resolved :=` inside the gate would shadow it (and `go vet` may warn). Reuse the existing var.

// CRITICAL (G4 — SessionMode is *string; MUST nil-check before deref). manifest.go:66: `SessionMode
// *string`. Resolve() defaults nil → strPtr("") (non-nil after resolution), but the SAFE idiom is
// `resolved.SessionMode != nil && *resolved.SessionMode == "append"`. Validate() already enforced the
// ""/"append" enum, so no other value reaches here. Omitting the nil-check risks a nil-pointer deref if
// a manifest ever bypasses Resolve() (defense-in-depth).

// CRITICAL (G5 — finalize BEFORE dedupe, NOT after — D3). The contract's pseudocode says "IsDuplicate(
// ExtractSubject(msg2), ...)" (un-finalized), but the one-shot path finalizes BEFORE dedupe (generate.go
// comment: "template BEFORE dedupe (§9.7 judges the final subject)"). To avoid the template-duplicate-slip
// bug (msg2="feat: x" → template → "feat: x (#1)" matches history, but un-finalized dedupe misses it),
// call FinalizeMessage(msg2, cfg) FIRST, then IsDuplicate(ExtractSubject(finalMsg), recent). "Run the
// EXISTING duplicate check" = do what the one-shot path does.

// CRITICAL (G6 — the rescue return must be BYTE-IDENTICAL; guard with a second `if !success`). FR-T7:
// "Multi-turn can never leave the run in a worse state than one-shot-exhausted." The struct literal
// `&RescueError{Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA, Candidate: candidate, Cause:
// lastCause}` must be unchanged. Structure: `if !success { if <gate> { <multi-turn> } if !success {
// return &RescueError{...} } }`. The SECOND `if !success` lets multi-turn success skip the return. (D4.)

// CRITICAL (G7 — candidate/lastCause mapping mirrors the one-shot path exactly). One-shot sets:
//   - parse-fail (!ok): candidate = m (RAW parse output), lastCause unchanged.
//   - duplicate: candidate = m (FINALIZED — FinalizeMessage ran before dedupe).
//   - exec-error: lastCause = execErr.
// Multi-turn mirrors:
//   - cause != nil: lastCause = cause; candidate = msg2 (RAW) if msg2 != "".
//   - ok2==false (cause==nil): lastCause UNCHANGED; candidate = msg2 (RAW) if msg2 != "".
//   - duplicate: candidate = finalMsg (FINALIZED); lastCause UNCHANGED.
//   - success: msg = finalMsg; success = true.
// Do NOT set lastCause on the duplicate path (a duplicate is not an error). (D7.)

// CRITICAL (G8 — FR-T12: pass the captured `payload` variable as-is; do NOT re-capture or re-truncate).
// The contract is EXPLICIT: multi-turn reads the existing payload variable. FR-T2/FR-T12 say "no
// token_limit water-fill / untruncated payload," but re-capturing the diff with TokenLimit=0 is a
// "recompute" the contract forbids. Pass `payload` (the last-built loop value) to Run unchanged. The
// multi-turn path does NOT call EstimateTokens to re-truncate, does NOT call StagedDiff again, does NOT
// call BuildUserPayload again. (D2.)

// GOTCHA (G9 — Run/chunkPayload are SAME-PACKAGE; call UNQUALIFIED). generate.go and multiturn.go are
// both `package generate`. Call `Run(...)` and `chunkPayload(...)` WITHOUT a package qualifier (NOT
// `multiturn.Run`). The contract's `multiturn.Run` is pseudocode. (Research §2.)

// GOTCHA (G10 — the progress line needs `os` + `time` imports; neither is currently in generate.go).
// `fmt.Fprintf(os.Stderr, ...)` needs `os`; `cfg.Timeout * time.Duration(turns)` + `.Minutes()` needs
// `time`. Add both gofmt-sorted into the stdlib block (context, errors, fmt, os, strings, time). NO new
// internal-package imports. (D5; Research §7.)

// GOTCHA (G11 — chunkPayload is called TWICE: once in the gate for the turn count, once inside Run).
// This is fine — chunkPayload is PURE string math (deterministic; same payload + same chunkTokens ⇒
// identical result). The duplication is negligible (microseconds). Do NOT try to share state between the
// gate and Run (Run owns its own chunking). turns = len(chunkPayload(...)) + 1 (N chunks + 1 final turn).

// GOTCHA (G12 — totalMin floor of 1; format "~%dm"). cfg.Timeout × turns could be < 1 minute for tiny
// timeouts/turn counts. Floor totalMin at 1 (avoid "~0m"). The progress line is informational; minutes
// is the right granularity (multi-turn total = timeout × (N+1) ≥ timeout × 3, default 120s × 3 = 6m).

// GOTCHA (G13 — the stub NewScript is call-indexed AND clamps to the last line). The one-shot path
// consumes call 1; multi-turn consumes calls 2..(N+2). NewScript clamps to the last line after exhaustion,
// so a script ["", "ok", "ok", "<msg>"] works for ANY N≥2 (extra turns get "<msg>", discarded). For the
// success test, the FINAL multi-turn turn gets "<msg>" (clamped or indexed) → parsed → committed.

// GOTCHA (G14 — cfg.MultiTurnChunkTokens controls N in tests; keep it small but bounded). Default 32000
// ⇒ a small diff yields N=1 (2 turns) but EstimateTokens(payload) ≤ 32000 ⇒ condition (b) FALSE ⇒
// multi-turn skipped (the small-payload-skip test). To FORCE multi-turn, set MultiTurnChunkTokens small
// (2–4) so EstimateTokens(payload) > chunkTokens AND N is bounded (a small diff → N≈3–8). Do NOT use 1
// (N becomes huge for a real diff → slow test). (Research §5.)

// GOTCHA (G15 — S2 is a HARD dependency; S3 cannot build without Run). If multiturn.go lacks `func Run`
// (S2 not landed), `go build ./internal/generate/` fails on the undefined symbol. S3 MUST wait for S2.
// The edit anchors in generate.go are independent of multiturn.go's state, but the BUILD is not. (§6.)

// GOTCHA (G16 — signal.SetCandidate keeps the §18.3 rescue candidate current). The one-shot path calls
// signal.SetCandidate(m) after FinalizeMessage (L273). Mirror this in the multi-turn success path:
// signal.SetCandidate(finalMsg) after FinalizeMessage, BEFORE the dedupe check. (If multi-turn succeeds,
// no rescue fires, but SetCandidate is belt-and-suspenders for the signal handler.)
```

## Implementation Blueprint

### Data models and structure

None added or changed. The gate reuses existing types (`config.Config`, `provider.Manifest`, `RescueError`,
`Result`). No new constants, structs, or sentinels. The two new imports are stdlib (`os`, `time`).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: PREREQUISITE — confirm S2 (multiturn.Run) has landed
  - RUN: grep -n '^func Run' internal/generate/multiturn.go
  - EXPECT: a match (func Run(...)). If NO match, S2 has not landed — STOP. This subtask cannot build
    without Run (G15). Wait for S2, then proceed.

Task 1: MODIFY internal/generate/generate.go (imports — add os + time)
  - EDIT the import block (L8): add "os" and "time" gofmt-sorted into the stdlib group. Result:
      context, errors, fmt, os, strings, time  (then the internal-package group unchanged).
  - DO NOT add any internal-package import. DO NOT remove/reorder existing imports.
  - VERIFY: go build ./internal/generate/ → exit 0 (os/time not yet used → "imported and not used" is
    EXPECTED until Task 3; build will fail here — that's fine, proceed; Task 3 uses them).

Task 2: MODIFY internal/generate/generate.go (payload scope — hoist)
  - FIND the var block before the loop (the `var rejected []string` / `var candidate string` / `var
    parseFail bool` / `var lastCause error` / `var msg string` / `success := false` declarations).
  - ADD `var payload string` to that block (so it is function-scoped, surviving the loop).
  - FIND inside the loop (L228): `payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)`.
  - CHANGE `:=` → `=` (so it assigns the hoisted var, not redeclares). The `if parseFail { payload =
    retryInstr + "\n\n" + payload }` line stays unchanged.
  - DO NOT touch any other line in the loop body (Render/Execute/ParseOutput/FinalizeMessage/dedupe).
  - VERIFY: go build ./internal/generate/ → exit 0 (payload now function-scoped; the :=→= is a pure
    refactor — the loop behaves identically).

Task 3: MODIFY internal/generate/generate.go (the trigger gate + multi-turn branch)
  - FIND (L287–292):
        if !success {
            return Result{}, &RescueError{
                Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
                Candidate: candidate, Cause: lastCause,
            }
        }
  - REPLACE with the gate + branch verbatim from §"What" (the nested `if cfg.MultiTurnFallback && …`,
    the progress line, the VerboseWarn trigger, the Run call, the cause==nil&&ok2 success/dedupe branch,
    the else cause/parse-fail branch, and the SECOND `if !success { return &RescueError{...} }`).
  - PRESERVE: the struct literal is byte-identical (Kind/TreeSHA/ParentSHA/Candidate/Cause, same order).
  - DO NOT: re-declare `resolved` (reuse L211); touch the in-loop rescue returns (L246/L252); touch the
    post-loop EditMessage/CommitTree/UpdateRefCAS/DiffTree path; modify multiturn.go; add exhaustive
    tests (S4); add docs (S4).
  - VERIFY: go build ./... → exit 0 (os + time now used; Run/chunkPayload same-package resolve).

Task 4: MODIFY internal/generate/generate_test.go (add 3–4 focused tests)
  - ADD 3 (or 4) tests modeled on TestCommitStaged_Success (same fixtures: initRepo/commitRaw/writeFile/
    stageFile; stubtest.Build + stubtest.Manifest/NewScript; config.Defaults()). See §"Test cases".
  - TEST MATRIX:
      TestCommitStaged_MultiTurnFallbackSuccess — all 4 conditions met; multi-turn commits the message.
      TestCommitStaged_MultiTurnSkipped_NonAppend — SessionMode unset; rescue (no multi-turn).
      TestCommitStaged_MultiTurnSkipped_SmallPayload — default chunkTokens (32000); rescue (cond b false).
      (optional) TestCommitStaged_MultiTurnDuplicateRescue — multi-turn returns a duplicate → rescue w/ Candidate.
  - DO NOT: write the exhaustive 4-condition truth table (S4); write integration tests (T4); use testify.
  - VERIFY: go test -race ./internal/generate/ -run TestCommitStaged_MultiTurn → all PASS.

Task 5: VALIDATE — full gate set + scope discipline
  - RUN: gofmt -w internal/generate/generate.go internal/generate/generate_test.go ; gofmt -l .
  - RUN: go vet ./...
  - RUN: go build ./...
  - RUN: go test -race ./internal/generate/ -v -run 'TestCommitStaged'
        → expect ALL existing CommitStaged tests + the new multi-turn tests PASS.
  - RUN: go test -race ./...                # whole repo green (additive change to 2 files)
  - RUN (scope): git status --porcelain → expect EXACTLY:
        M internal/generate/generate.go
        M internal/generate/generate_test.go
  - RUN (byte-identical rescue grep): the rescue struct literal appears, unchanged:
        grep -n 'Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA' internal/generate/generate.go
        # EXPECT: matches at the in-loop returns (L246/L252) AND the post-gate return — same literal.
  - RUN (no multiturn.go edit): git diff --name-only internal/generate/multiturn.go
        # EXPECT: no output (multiturn.go untouched — S2's file).
```

### Test cases (the focused matrix — `generate_test.go`)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestCommitStaged_MultiTurnFallbackSuccess` | small staged file; `cfg.MaxDuplicateRetries=0`, `cfg.MultiTurnChunkTokens=4`; manifest `SessionMode=&"append"`; NewScript `["", "ok", "ok", "feat: multi-turn win"]` | `err==nil`; `res.Subject=="feat: multi-turn win"`; HEAD == res.CommitSHA; `git log` msg matches | the gate fires multi-turn; a non-duplicate multi-turn message COMMITS (the wiring works end-to-end) |
| `TestCommitStaged_MultiTurnSkipped_NonAppend` | same, but `SessionMode` UNSET (⇒ ""); NewScript `[""]` | `err` is `*RescueError{Kind:ErrRescue}`; HEAD unchanged (idempotent); multi-turn did NOT fire | condition (d) false ⇒ fall through to rescue (byte-identical) |
| `TestCommitStaged_MultiTurnSkipped_SmallPayload` | small staged file; DEFAULT `cfg` (MultiTurnChunkTokens=32000); NewScript `[""]` | `err` is `*RescueError{Kind:ErrRescue}`; HEAD unchanged; multi-turn did NOT fire | condition (b) false (payload ≤ 32000 tokens) ⇒ small-payload one-shot failure skips multi-turn (FR-T1b) |
| _(optional)_ `TestCommitStaged_MultiTurnDuplicateRescue` | HEAD subject == "feat: dup"; `cfg.MaxDuplicateRetries=0`, `cfg.MultiTurnChunkTokens=4`; `SessionMode=&"append"`; NewScript `["", "ok", "ok", "feat: dup"]` | `err` is `*RescueError{Kind:ErrRescue}`; `Candidate` contains "feat: dup" | multi-turn duplicate → rescue with Candidate (D3/D7) |

```go
// Helper: build an append-capable stub manifest for multi-turn tests (stubtest.NewScript does NOT set
// SessionMode; the gate's condition (d) + RenderMultiTurn's own gate both require "append").
func appendScriptManifest(t *testing.T, bin string, responses []string) provider.Manifest {
	t.Helper()
	m := stubtest.NewScript(t, bin, responses)
	appendMode := "append"
	m.SessionMode = &appendMode
	return m
}

// TestCommitStaged_MultiTurnFallbackSuccess: one-shot exhausts (call 1 = ""), multi-turn fires (cond
// a–d all hold), final turn returns "feat: multi-turn win" → committed. chunkTokens=4 keeps N bounded.
func TestCommitStaged_MultiTurnFallbackSuccess(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hello world\n")
	stageFile(t, repo, "new.txt")

	m := appendScriptManifest(t, bin, []string{"", "ok", "ok", "feat: multi-turn win"})
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0     // one-shot: 1 attempt (the "")
	cfg.MultiTurnChunkTokens = 4    // small enough that EstimateTokens(payload) > 4 (cond b); N bounded

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v (expected multi-turn success)", err)
	}
	if res.Subject != "feat: multi-turn win" {
		t.Errorf("Subject = %q, want %q (the multi-turn final-turn message)", res.Subject, "feat: multi-turn win")
	}
	if got := headSHA(t, repo); got != res.CommitSHA {
		t.Errorf("HEAD = %q, want %q (commit must land)", got, res.CommitSHA)
	}
}

// TestCommitStaged_MultiTurnSkipped_NonAppend: SessionMode unset ⇒ cond (d) false ⇒ no multi-turn ⇒ rescue.
func TestCommitStaged_MultiTurnSkipped_NonAppend(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "new.txt", "hello world\n")
	stageFile(t, repo, "new.txt")

	m := stubtest.NewScript(t, bin, []string{""}) // SessionMode unset (⇒ "") — NO append override
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0
	cfg.MultiTurnChunkTokens = 4 // cond (b) would hold, but (d) fails first

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	var re *RescueError
	if !errors.As(err, &re) || re.Kind != ErrRescue {
		t.Fatalf("err = %v, want *RescueError{Kind:ErrRescue} (non-append ⇒ no multi-turn ⇒ rescue)", err)
	}
	if got := headSHA(t, repo); got == "" {
		t.Error("HEAD empty — repo state corrupted")
	}
	// (HEAD unchanged: the initial unborn repo has no commits; idempotency = no tree/ref mutation.)
}

// TestCommitStaged_MultiTurnSkipped_SmallPayload: default chunkTokens (32000) ⇒ EstimateTokens(payload)
// ≤ 32000 ⇒ cond (b) false ⇒ small-payload one-shot failure skips multi-turn (FR-T1b) ⇒ rescue.
func TestCommitStaged_MultiTurnSkipped_SmallPayload(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hi\n") // tiny diff
	stageFile(t, repo, "new.txt")

	appendMode := "append"
	m := stubtest.NewScript(t, bin, []string{""})
	m.SessionMode = &appendMode
	cfg := config.Defaults() // MultiTurnChunkTokens=32000 (default) — cond (b) false for a tiny diff
	cfg.MaxDuplicateRetries = 0

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	var re *RescueError
	if !errors.As(err, &re) || re.Kind != ErrRescue {
		t.Fatalf("err = %v, want *RescueError{Kind:ErrRescue} (small payload ⇒ no multi-turn ⇒ rescue)", err)
	}
}
```
> The test file already imports `context`, `errors`, `testing`, `config`, `git`, `provider`, `stubtest`
> (verify against the current import block; add any missing). `shaRe`/`headSHA`/`initRepo`/`commitRaw`/
> `writeFile`/`stageFile` are the file's existing helpers — reuse them (no redeclaration).

### Implementation Patterns & Key Details

```go
// === Why hoist payload (not re-capture) — D1/G1 ===
// The contract forbids recomputing the payload ("reads the existing payload variable, NOT a fresh
// BuildUserPayload call"). The cleanest literal reading: hoist `var payload string` before the loop and
// assign inside (`payload = ...`). The last-built payload (final loop iteration, with whatever
// rejected/retryInstr state) survives to the gate. This is a pure refactor — the loop is unchanged except
// :=→=. Re-capturing via a fresh BuildUserPayload(diff, cfg.Context, nil) would (a) be a recompute the
// contract forbids, and (b) drop the retryInstr/rejected state (a subtle behavior change).

// === Why pass the captured payload as-is (FR-T12) — D2/G8 ===
// FR-T2/FR-T12 say "no token_limit water-fill / untruncated payload." The `diff` was captured WITH
// TokenLimit (StagedDiff at :175), so `payload` MAY be water-filled. The contract resolves this by
// treating the captured `payload` variable as what multi-turn delivers, and NOT re-applying token_limit
// in the multi-turn path. Rationale: re-capturing with TokenLimit=0 is a recompute (forbidden); the
// primary reliability mechanism is chunkPayload's per-request chunking (works on any payload); if
// TokenLimit truncated, multi-turn still helps. Pass `payload` unchanged.

// === Why finalize BEFORE dedupe — D3/G5 ===
// The one-shot path: FinalizeMessage THEN IsDuplicate (generate.go comment: "template BEFORE dedupe
// (§9.7 judges the final subject)"). If cfg.Template is set, FinalizeMessage applies it; dedupe must
// compare the TEMPLATED subject or a templated duplicate slips through. So: finalMsg := FinalizeMessage(
// msg2, cfg); IsDuplicate(ExtractSubject(finalMsg), recent). "Run the EXISTING duplicate check" = do
// what the one-shot path does. (Deviates from the contract's literal pseudocode for correctness.)

// === Why the second `if !success` — D4/G6 ===
// FR-T7: the rescue return must be byte-identical and multi-turn must never worsen the state. Structure:
// `if !success { if <gate> { <multi-turn; may set success=true> } if !success { return &RescueError{...} } }`.
// The second `if !success` lets multi-turn success SKIP the return. The struct literal is unchanged. No
// goto/labeled-break needed. (Cleaner than the contract's pseudocode "skip the rescue return" hand-wave.)

// === Why a direct stderr write for the progress line — D5/G10 ===
// FR-T5 wants a USER-VISIBLE progress line (not --verbose-gated). Deps.Progress is a `func()` callback
// (no message param) — it can't carry the dynamic turn-count/budget. Adding a new Deps callback is scope
// creep (ui/cmd wiring). The contract allows a direct stderr write: fmt.Fprintf(os.Stderr, ...). This
// matches ui.Progress's stream (output.go writes progress to stderr). Adds `os` + `time` imports.

// === Why VerboseWarn for the trigger — D6 ===
// There's no dedicated multi-turn-trigger verbose method. VerboseWarn(msg) is the nil-safe generic
// one-liner. FR-T11's PER-TURN verbose (payload size + raw stdout/stderr) is emitted by provider.Execute
// automatically (Run calls Execute per turn) — FREE, no extra wiring. S3 emits ONLY the one trigger line.

// === Why chunkPayload is called twice (gate + Run) — G11 ===
// The gate calls len(chunkPayload(payload, cfg.MultiTurnChunkTokens)) for the turn-count progress line;
// Run calls chunkPayload again internally. Both are pure string math (deterministic; identical result).
// The duplication is negligible. Do NOT try to share state (Run owns its chunking).

// === Why candidate/lastCause mirror the one-shot path — D7/G7 ===
// On duplicate: candidate = finalMsg (FINALIZED — one-shot sets candidate = m post-finalize). On
// cause/parse-fail: candidate = msg2 (RAW — one-shot sets candidate = m = raw parse output on !ok);
// lastCause = cause only if cause != nil. This makes the RescueError indistinguishable from a one-shot
// failure of the same shape (FR-T7 "no worse state").
```

### Integration Points

```yaml
PRODUCTION (internal/generate/generate.go — MODIFIED):
  - + `var payload string` (hoisted before the loop)
  - + `:228` `:=` → `=` (assign the hoisted var)
  - + the FR-T1 gate (nested in `if !success`) + multi-turn branch (progress + verbose + Run + mapping)
  - + the second `if !success` rescue guard (byte-identical struct literal)
  - + imports: os, time

TESTS (internal/generate/generate_test.go — MODIFIED):
  - + appendScriptManifest helper + 3–4 TestCommitStaged_MultiTurn* tests

CONSUMED (READ-ONLY — the seams):
  - internal/generate/multiturn.go: Run(ctx, deps, cfg, manifest, sysPrompt, payload, msgModel, msgReasoning) (msg, ok, cause)
  - internal/generate/multiturn.go: chunkPayload(payload, chunkTokens) []chunk  (same package; len() for turn count)
  - internal/generate/finalize.go: FinalizeMessage(msg, cfg) string  (same package)
  - internal/generate/dedupe.go: ExtractSubject(m) string ; IsDuplicate(subject, recent) bool  (same package)
  - internal/git/tokens.go: EstimateTokens(s) int  (git already imported)
  - internal/provider/manifest.go: Resolve().SessionMode (*string; nil-check)  (already imported via deps)
  - internal/ui/verbose.go: VerboseWarn(msg)  (nil-safe; deps.Verbose)
  - internal/signal: SetCandidate(m)  (already imported)

DEPENDENCY (HARD — S2 must land first):
  - internal/generate/multiturn.go MUST contain `func Run` (P1.M1.T3.S2's deliverable). If absent, build fails.

DOWNSTREAM (informational — do NOT implement now):
  - P1.M1.T3.S4: the exhaustive 4-condition truth table + token_limit non-interaction tests + how-it-works.md.
  - P1.M1.T4: the integration matrix (N+1 turns end-to-end, --session-id present, --no-session dropped, commit lands,
    mid-turn failure → rescue, small-payload skip, non-append skip).
  - P1.M4.T1.S2 / internal/cmd: the CLI default action / hook path call CommitStaged; the progress line
    appears on stderr (no CLI change needed — CommitStaged writes it directly).

GATE: go test -race ./internal/generate/ → GREEN ; git status → ONLY the 2 modified files

NO-TOUCH (explicitly — owned by siblings):
  - internal/generate/multiturn.go + multiturn_test.go (S1/S2), rescue.go/finalize.go/dedupe.go + their tests,
    invariants_test.go, realagent_test.go
  - internal/provider/* (read-only seams), internal/git/* (read-only), internal/config/* (T2 — LANDED),
    internal/ui/* (read-only), internal/stubtest/* + cmd/stubagent (read-only test seam), internal/cmd/*
  - docs/* (S4/S5), README.md (S5), PRD.md, tasks.json, prd_snapshot.md, plan/*
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/generate/generate.go internal/generate/generate_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/generate/...   # Expected: exit 0 (no shadowing of `resolved`; no unused import)
go build ./...                   # Expected: exit 0 (os+time used; Run/chunkPayload same-package resolve)

# Expected: zero errors. If `undefined: Run` — S2 (multiturn.go) has not landed; STOP (G15). If
# `imported and not used: "os"`/`"time"` — Task 3's body wasn't written; complete it. If `resolved
# declared and not used` or `resolved shadowed` — you re-declared resolved; reuse the L211 var (G3).
```

### Level 2: The Focused Multi-Turn Tests (the deliverable)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./internal/generate/ -v -run 'TestCommitStaged_MultiTurn'
# Expected: 3 (or 4) tests PASS, exit 0:
#   TestCommitStaged_MultiTurnFallbackSuccess   — multi-turn commits "feat: multi-turn win"; HEAD advances
#   TestCommitStaged_MultiTurnSkipped_NonAppend — SessionMode unset ⇒ *RescueError{ErrRescue}
#   TestCommitStaged_MultiTurnSkipped_SmallPayload — default chunkTokens ⇒ *RescueError{ErrRescue}
#   (TestCommitStaged_MultiTurnDuplicateRescue) — multi-turn dup ⇒ *RescueError with Candidate
```

### Level 3: Whole-Repository Regression + Scope Discipline

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...    # Expected: ALL packages green (2 files changed, additive + a pure refactor)
go vet ./...           # Expected: exit 0

# Scope: ONLY the two modified generate files.
git status --porcelain
# Expected EXACTLY:
#   M internal/generate/generate.go
#   M internal/generate/generate_test.go

# multiturn.go is UNTOUCHED (S2's file):
git diff --name-only internal/generate/multiturn.go internal/generate/multiturn_test.go
# Expected: NO output.

# The rescue struct literal is byte-identical (appears at the in-loop returns AND the post-gate return):
grep -n 'Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA' internal/generate/generate.go
# Expected: 3 matches (L246 timeout-path uses ErrTimeout — NOT this literal; the ErrRescue literal appears
# at L252 cancel-path AND the post-gate return). Confirm the post-gate return literal matches the original.

# The payload hoist (:= → =):
grep -n 'payload = prompt.BuildUserPayload\|var payload string' internal/generate/generate.go
# Expected: `var payload string` (pre-loop) AND `payload = prompt.BuildUserPayload(...)` (in-loop, = not :=).

# The gate conditions are all present:
grep -n 'cfg.MultiTurnFallback\|EstimateTokens(payload) > cfg.MultiTurnChunkTokens\|resolved.SessionMode != nil' internal/generate/generate.go
# Expected: matches in the gate block.
```

### Level 4: Runtime Smoke Test (prove the gate fires against a real stub)

```bash
cd /home/dustin/projects/stagecoach

# A focused in-process proof (mirrors TestCommitStaged_MultiTurnFallbackSuccess) — run via go test -v
# and observe (a) the "↳ falling back to multi-turn" progress line on stderr, (b) the commit landing.
go test -race ./internal/generate/ -v -run 'TestCommitStaged_MultiTurnFallbackSuccess' 2>&1 | \
  grep -E 'falling back to multi-turn|PASS|FAIL'
# Expected: a "↳ falling back to multi-turn: N turns, ~Mm total" line (on stderr) + PASS.

# Negative: the small-payload path does NOT print the progress line:
go test -race ./internal/generate/ -v -run 'TestCommitStaged_MultiTurnSkipped_SmallPayload' 2>&1 | \
  grep -c 'falling back to multi-turn'
# Expected: 0 (no progress line — multi-turn was skipped).
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -l .` reports nothing.
- [ ] `go vet ./...` exits 0.
- [ ] `go build ./...` exits 0 (os+time used; Run/chunkPayload same-package resolve; S2 landed).
- [ ] `go test -race ./internal/generate/` exits 0 (new multi-turn tests + all existing tests pass).
- [ ] `make test` (or `go test -race ./...`) exits 0.

### Feature Validation

- [ ] `var payload string` hoisted before the loop; `:228` uses `=` (not `:=`).
- [ ] The FR-T1 gate is nested in `if !success` with all 4 conditions (a implicit; b/c/d explicit).
- [ ] On a passing gate: progress line to stderr; VerboseWarn trigger; `Run(...)` called with the captured payload.
- [ ] On multi-turn success (non-duplicate): `msg = FinalizeMessage(msg2, cfg)`, `success = true`, commit path runs.
- [ ] On multi-turn duplicate: `candidate = finalMsg`; falls through to rescue.
- [ ] On multi-turn cause/parse-fail: `lastCause = cause` (if cause != nil); `candidate = msg2` (if non-empty); rescue.
- [ ] The rescue struct literal is byte-identical; a second `if !success` guards it.
- [ ] With ANY gate condition false: the existing rescue fires (no multi-turn, no progress line).

### Security & Scope Discipline Validation

- [ ] `multiturn.go`/`multiturn_test.go` UNTOUCHED (S1/S2's file).
- [ ] The in-loop rescue returns (`:246` timeout, `:252` cancel) unchanged.
- [ ] The post-loop EditMessage/CommitTree/UpdateRefCAS/DiffTree path unchanged.
- [ ] `Run`/`chunkPayload`/`newSessionID` consumed (NOT modified).
- [ ] NO new internal-package imports (only `os`, `time` stdlib).
- [ ] `resolved` reused (L211), NOT re-declared.
- [ ] `SessionMode` nil-checked before deref.
- [ ] NO exhaustive truth table (S4), NO how-it-works doc (S4), NO integration matrix (T4).
- [ ] Only `internal/generate/generate.go` (modified) + `internal/generate/generate_test.go` (modified).
- [ ] `go.mod`/`go.sum` unchanged (no new deps).

### Documentation & Deployment

- [ ] Doc comment on the gate explains FR-T1 a–d + FR-T7 (byte-identical rescue) + FR-T12 (captured payload).
- [ ] The progress-line format is human-readable (turn count + ~minutes total).
- [ ] No new environment variables or config keys (MultiTurnFallback/MultiTurnChunkTokens already landed in T2).

---

## Anti-Patterns to Avoid

- ❌ Don't re-capture or re-truncate the payload in the multi-turn path — pass the captured `payload`
  variable as-is (FR-T12, D2).
- ❌ Don't dedupe the UN-finalized msg2 — finalize FIRST, then dedupe (one-shot parity; D3).
- ❌ Don't re-declare `resolved` — reuse the L211 var (G3).
- ❌ Don't omit the nil-check on `resolved.SessionMode` — it's `*string` (G4).
- ❌ Don't change the rescue struct literal — it must be byte-identical (FR-T7, G6); use a second
  `if !success` to skip it on multi-turn success.
- ❌ Don't qualify `Run`/`chunkPayload` with a package name — they're same-package (`package generate`).
- ❌ Don't write the exhaustive 4-condition truth table — that's S4. Ship 3–4 focused tests only.
- ❌ Don't add a new ui.Verbose method or Deps callback — use VerboseWarn + a direct stderr write.
- ❌ Don't set `lastCause` on the duplicate path — a duplicate is not an error (G7).
- ❌ Don't proceed if S2 (multiturn.Run) hasn't landed — the build will fail (G15); it's a hard dependency.
