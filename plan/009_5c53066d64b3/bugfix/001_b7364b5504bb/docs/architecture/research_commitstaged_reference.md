# REFERENCE: CommitStaged Multi-Turn Fallback Architecture

Authoritative architectural map of the multi-turn fallback wired into `internal/generate/generate.go::CommitStaged`. This is the REFERENCE implementation to be propagated to two other paths. All line numbers are exact (file as of 2026-07-05).

---

## 1. Files Retrieved

1. `internal/generate/generate.go` (lines 1-468, full file) — `CommitStaged` orchestrator; the multi-turn gate lives inline at lines 290-368. No standalone predicate function exists.
2. `internal/generate/multiturn.go` (lines 1-204, full file) — `chunkPayload`, `Run`, `advanceRunes`, `newSessionID`, `preambleFmt`, `finalInstruction`.
3. `internal/generate/multiturn_test.go` (lines 1-463, full file) — chunkPayload pure-math tests + `Run` unit tests + the FR-T1 truth table driven through CommitStaged.
4. `internal/generate/generate_multiturn_test.go` (lines 1-235, full file) — `TestCommitStaged_MultiTurnRenderContract` (per-turn render contract: session-id stability, turn-1-only system prompt flag).
5. `internal/generate/generate_multiturn_failure_test.go` (lines 1-176, full file) — mid-turn failure → rescue (FR-T7), small-payload skip, non-append skip, with idempotent-index + stub-counter invariants.
6. `internal/git/tokens.go` (lines 25-46) — `EstimateTokens` = `ceil(runeCount/4)`.
7. `internal/provider/render.go` (lines 203-211) — `RenderMultiTurn` signature + session_mode gate.
8. `internal/config/config.go` (lines 84-85, 179-180) — `MultiTurnFallback bool`, `MultiTurnChunkTokens int` (defaults: true / 32000).

---

## 2. CommitStaged — Line-by-Line Map of the Multi-Turn Surface

### 2a. Variable hoists (the D1 "payload survives the loop" seam) — lines 221-226

```go
221:	var rejected []string
222:	var candidate string // last generated message (for RescueError.Candidate)
223:	var parseFail bool   // previous attempt failed parsing → prepend retryInstr next attempt
224:	var lastCause error  // last Execute error (for RescueError.Cause)
225:	var msg string       // the successful message (set on break)
226:	var payload string   // hoisted: the last-built payload survives the loop for the FR-T1 gate (D1)
227:	success := false
```

`payload` (L226) and `success` (L227) are hoisted OUT of the loop so the FR-T1 gate (L290+) can read the last-built one-shot payload. This is the load-bearing hoist: without it the gate could not inspect condition (b).

### 2b. The generate→parse→dedupe LOOP — lines 229-289

- **retryInstr prepend** — L231-234:
  ```go
  231: payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)
  232: if parseFail {
  233:     payload = retryInstr + "\n\n" + payload // FR29 corrective preamble
  234: }
  ```
  `payload` is rebuilt every attempt (rejection list changes). `retryInstr` is `*resolved.RetryInstruction` (the default "Output ONLY the commit message…"), prepended only after a prior parse failure.

- `retryInstr` defined at L220: `retryInstr := *resolved.RetryInstruction`.
- `resolved := deps.Manifest.Resolve()` at L219.
- `msgModel, msgReasoning` from `config.ResolveRoleModel("message", cfg)` at L227-228 (the `_` discards the resolved provider).

- Render + Execute + ParseOutput + Finalize + dedupe at L236-289. On success: `msg = m; success = true; break` (L284-286).

### 2c. THE FR-T1 GATE — lines 290-368 (the critical region)

The gate lives BETWEEN the loop (`for ... break`, ends L287) and the `if !success { return rescue }` (L369-374). Structure:

```
L287:   } <- end of loop
L288:
L290:   if !success {                               <- condition (a) implicitly true here
L291-303:   [comment block explaining FR-T1 conds a-d + FR-T12]
L304-305:   if cfg.MultiTurnFallback &&             <- condition (c)
L305:         resolved.SessionMode != nil && *resolved.SessionMode == "append" {  <- condition (d)
L307-329:     [mtPayload: FR-T12 re-capture]
L331:         if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {  <- condition (b)
L333-341:       [turn-count + verbose lines]
L344:           msg2, ok2, cause := Run(...)        <- N+1 protocol entry
L347-366:       [dedupe the result]
L367:         }
L368:       }
L369:   if !success {                               <- byte-identical rescue (FR-T7)
L370-373:     return &RescueError{Kind: ErrRescue, ...}
```

---

## 3. The FR-T1 Trigger Gate — 4 Conditions

The gate has NO standalone predicate function. It is INLINE at `generate.go:290-331`. The 4 conditions:

| # | Condition | Variable(s) | Line | Logic |
|---|-----------|-------------|------|-------|
| **(a)** | One-shot path exhausted (no message accepted) | `success == false` | L290 | Enclosing `if !success {` — already true before the gate body executes |
| **(b)** | (Untruncated) payload exceeds one chunk | `git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens` | L331 | Computed on `mtPayload` (post FR-T12 re-capture), NOT on `payload` |
| **(c)** | Multi-turn fallback enabled in config | `cfg.MultiTurnFallback` (bool, default true) | L304 | `multi_turn_fallback` TOML key; only-true-propagates |
| **(d)** | Manifest declares session_mode="append" | `resolved.SessionMode != nil && *resolved.SessionMode == "append"` | L305 | `resolved := deps.Manifest.Resolve()` (L219) |

**Ordering**: (c) and (d) are checked FIRST (L304-305), then FR-T12 re-capture (L307-329) computes `mtPayload`, THEN (b) is evaluated on `mtPayload` (L331). This ordering is load-bearing: (b) must run AFTER the re-capture so it sees the untruncated payload.

**Exact gate code** (L304-331):
```go
304: if cfg.MultiTurnFallback &&
305:     resolved.SessionMode != nil && *resolved.SessionMode == "append" {
306:
307:     // FR-T12 re-capture ...
311:     mtPayload := payload
312:     if cfg.TokenLimit != 0 {
313:         fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
314:             MaxDiffBytes:        cfg.MaxDiffBytes,
315:             MaxMDLines:          cfg.MaxMdLines,
316:             BinaryExtensions:    cfg.BinaryExtensions,
317:             Excludes:            deps.Excludes,
318:             TokenLimit:          0, // FR-T12: multi-turn ignores token_limit
319:             DiffContext:         cfg.DiffContextValue(),
320:             PromptReserveTokens: 0,
321:         })
322:         if derr == nil {
323:             mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
324:         }
325:         // On re-capture error, fall back to the one-shot payload (best-effort)
326:     }
330:     if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {
```

---

## 4. The `mtPayload` Variable — FR-T12 Re-Capture (CRITICAL for Issue 4)

Defined at `generate.go:311`. This is the variable whose handling differs by `cfg.TokenLimit`.

```go
311: mtPayload := payload                              // DEFAULT: reuse the one-shot payload
312: if cfg.TokenLimit != 0 {                          // FR-T12 branch: one-shot diff was truncated
313:     fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
...
318:         TokenLimit:          0,                   // re-capture UNTRUNCATED
319:         PromptReserveTokens: 0,                   // multi-turn doesn't use one-shot reserve
...
321:     })
322:     if derr == nil {
323:         mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
324:     }
325:     // On re-capture error: mtPayload stays == payload (the truncated one-shot, best-effort)
326: }
330: if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {   // (b) on UNTRUNCATED
```

### Two cases:

**Case A — `cfg.TokenLimit == 0` (token limit unset):**
- L312 branch NOT entered.
- `mtPayload = payload` (L311) — the one-shot payload is already untruncated (derived from an untruncated diff), so no re-capture needed. Fast path: avoids a second `StagedDiff` call.

**Case B — `cfg.TokenLimit != 0` (token limit set):**
- L312 branch entered.
- Re-capture diff with `TokenLimit: 0` (L318) → `fullDiff` is untruncated.
- Rebuild payload: `mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)` (L323).
- `PromptReserveTokens: 0` (L320) — multi-turn chunking doesn't use the one-shot reserve.
- On re-capture error (`derr != nil`): silently fall back to `payload` (best-effort; the truncated payload, if any). No rescue, no warning.

### Why this matters (Issue 4 context):
The whole point of multi-turn (FR-T12) is **lossless delivery of a payload that exceeded what one request could carry**. When `token_limit` is set, it truncated the one-shot `diff`/`payload`; condition (b) would be FALSE on the truncated payload, and multi-turn would WRONGLY be skipped. The re-capture (L312-326) ensures condition (b) is evaluated against the UNTRUNCATED payload, so the feature fires even under a truncating `token_limit`. This is verified by `TestMultiTurnGate_TokenLimitTruncated_Recaptures` (multiturn_test.go:418-463).

---

## 5. FR-T11 Verbose Output at Trigger Time — lines 332-341

Two outputs are emitted when the gate fires (after condition (b) passes):

```go
332: turns := len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1 // N chunks + 1 final turn
333: totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
334: if totalMin < 1 {
335:     totalMin = 1
336: }
337: fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)
...
341: deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")
```

**Line 1 (L337)** — direct stderr write (NOT via Deps.Progress, because Progress is a no-arg callback that can't carry a message). Format: `↳ falling back to multi-turn: %d turns, ~%dm total`. Reports turn count (`N+1`) and total wall-clock budget (`cfg.Timeout × turns`, in minutes, clamped to ≥1).

**Line 2 (L341)** — `deps.Verbose.VerboseWarn(...)` — the trigger marker line `"one-shot exhausted → multi-turn fallback"`. This is the line the truth-table tests assert on (the `multi-turn fallback` substring; multiturn_test.go:246, generate_multiturn_test.go:185).

**What's MISSING** (per the task brief's "per-chunk token estimate" question): there is NO per-chunk token estimate printed. The `turns` count comes from `len(chunkPayload(...))` (L332) but the individual chunk sizes are never logged. Only the turn count + total minutes are surfaced. Per-turn verbose (each provider.Execute command) is emitted separately inside `multiturn.Run` via `deps.Verbose` passed to `provider.Execute` (comment at L341: "per-turn verbose is emitted by provider.Execute inside Run"). The `mtPayload` total token estimate (`git.EstimateTokens(mtPayload)`) is computed at L330 but never printed.

---

## 6. Dedupe Logic After `multiturn.Run` Returns — lines 344-366

After `Run` returns `(msg2, ok2, cause)` at L344:

```go
344: msg2, ok2, cause := Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)
345:
346: if cause == nil && ok2 {
347:     // Dedupe the multi-turn result. §9.7 judges the FINAL subject → finalize BEFORE dedupe
348:     finalMsg := FinalizeMessage(msg2, cfg)         // template BEFORE dedupe (FR-F8 + D3)
349:     signal.SetCandidate(finalMsg)
350:     if !IsDuplicate(ExtractSubject(finalMsg), recent) {
351:         msg = finalMsg
352:         success = true                              // multi-turn succeeded → skip rescue
353:     } else {
354:         // Duplicate → rescue with the finalized candidate (one-shot parity)
355:         candidate = finalMsg
356:     }
357: } else {
358:     // cause != nil (turn error/timeout) OR ok2==false (final parse empty) → rescue
359:     if cause != nil {
360:         lastCause = cause                           // multi-turn failure supersedes one-shot's lastCause
361:     }
362:     if msg2 != "" {
363:         candidate = msg2                            // raw parse output (one-shot parse-fail parity)
364:     }
365: }
```

**Three branches:**

1. **`cause == nil && ok2` (success)** — L346-356:
   - `FinalizeMessage(msg2, cfg)` BEFORE dedupe (D3: §9.7 judges the final subject; avoids the template-duplicate-slip bug). One-shot parity with L275.
   - `IsDuplicate(ExtractSubject(finalMsg), recent)` (L350) — same dedupe used in the one-shot loop (L279). `recent` was fetched once at L207.
   - Not-dup → `msg = finalMsg; success = true` (L351-352) — flow continues past the rescue to EditMessage (L378) + CommitTree.
   - Dup → `candidate = finalMsg` (L355) — falls through to the byte-identical rescue (L369).

2. **`cause != nil` (turn error/timeout/cancel)** — L358-364:
   - `lastCause = cause` (L360) — the multi-turn failure supersedes one-shot's `lastCause`.
   - `candidate = msg2` if non-empty (L362-363) — raw parse output.
   - Falls through to rescue (L369) with `Cause: lastCause`.

3. **`cause == nil && ok2 == false` (final turn empty/unparseable)** — L358-364:
   - `lastCause` NOT updated (cause is nil).
   - `candidate = msg2` if non-empty (likely "").
   - Falls through to rescue (L369). The existing `lastCause` (from one-shot) propagates.

**The byte-identical rescue** (FR-T7) — L369-373:
```go
369: if !success {
370:     return Result{}, &RescueError{
371:         Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
372:         Candidate: candidate, Cause: lastCause,
373:     }
374: }
```
This is byte-identical to the pre-multi-turn rescue (the same struct literal would have been the only path before FR-T1 was added). FR-T7 requires multi-turn failure to be indistinguishable from one-shot exhaustion at the exit code level (exit 3, not 124).

---

## 7. `internal/generate/multiturn.go` — Full Map

### 7a. Type `chunk` (L31-35)
```go
31: type chunk struct {
32:     index int    // 1-based part number (the i in "PART i/N")
33:     total int    // N = len(chunks); identical across all chunks
34:     text  string // "PART i/N:\n<body>"
35: }
```

### 7b. `chunkPayload` (L52-93) — the pure string-math sizing seam
```go
52: func chunkPayload(payload string, chunkTokens int) []chunk {
```
- **Token budget**: `chunkTokens` tokens ≈ `chunkTokens*4` runes (since `git.EstimateTokens = ceil(runes/4)`). `runesPerWindow := chunkTokens * 4` (L62).
- **Walk**: advance rune-by-rune via `advanceRunes` (L95-104; uses `utf8.DecodeRuneInString` so multi-byte UTF-8 is never split). Each window anchors FORWARD to the next `\n` so no diff line is fractured (L67-72).
- **Empty payload** → single empty chunk (N≥1; L79-81).
- **Prefix stamping**: each chunk.text = `fmt.Sprintf("PART %d/%d:\n%s", i+1, total, body)` (L89). The prefix sits OUTSIDE the body budget.
- `chunkTokens < 1` collapses to 1 (L53-55).

### 7c. `preambleFmt` (L106) and `finalInstruction` (L110) — VERBATIM protocol strings
```go
106: const preambleFmt = "I will send a git diff in %d parts. After each part, reply with exactly: ok. Do not analyze or write any commit message until I explicitly ask at the end."
110: const finalInstruction = "Now write the commit message for the diff above. Output ONLY the message."
```

### 7d. `Run` — the N+1 protocol (L136-187)

**Signature** (L136-137):
```go
136: func Run(ctx context.Context, deps Deps, cfg config.Config, manifest provider.Manifest,
137:     sysPrompt, payload, msgModel, msgReasoning string) (msg string, ok bool, cause error)
```

**Protocol** (comments L138-135 are extensive):

- **(1)** Chunk the payload: `chunks := chunkPayload(payload, cfg.MultiTurnChunkTokens)` (L142); `N := len(chunks)` (L143).
- **(2)** Mint session id: `sessionID := newSessionID()` (L145) — format `"stagecoach-<32 hex>"` (L194-202), one-run-scope (FR-T6, never resumed).
- **(3)** Preamble: `preamble := fmt.Sprintf(preambleFmt, N)` (L148).
- **(4) Turn 1**: system prompt + preamble + chunk 1.
  ```go
  149: // (4) Turn 1: system prompt + preamble + chunk 1. RenderMultiTurn emits the system_prompt_flag
  151: spec, rerr := manifest.RenderMultiTurn(msgModel, sysPrompt, preamble+"\n\n"+chunks[0].text,
  152:     msgReasoning, sessionID, 1)
  153: if rerr != nil {
  154:     return "", false, rerr  // non-append provider ⇒ session_mode gate; surface as cause
  155: }
  156: if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {
  157:     return "", false, execErr  // FR-T7: any turn error aborts
  158: }
  ```
  `sysPrompt` passed on turn 1 ONLY — `RenderMultiTurn` emits `system_prompt_flag` only when `turn==1` AND sysPrompt != "".
- **(5) Turns 2..N** (L160-169): each chunk's body; `sysPrompt=""` drops the system_prompt_flag.
  ```go
  160: for i := 2; i <= N; i++ {
  162:     spec, rerr := manifest.RenderMultiTurn(msgModel, "", chunks[i-1].text,
  163:         msgReasoning, sessionID, i)
  164:     ...
  167:     if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {
  168:         return "", false, execErr
  169:     }
  170: }
  ```
- **(6) Turn N+1 (final)** (L172-183): the commit-message request. stdout is the candidate message.
  ```go
  172: spec, rerr = manifest.RenderMultiTurn(msgModel, "", finalInstruction,
  173:     msgReasoning, sessionID, N+1)
  ...
  178: out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
  179: if execErr != nil {
  180:     return "", false, execErr  // FR-T7: final-turn error aborts
  181: }
  ```
- **(7) Parse** (L185-186): `m, parseOK, _ := provider.ParseOutput(out, manifest); return m, parseOK, nil`. `ok==false` (empty/unparseable) is NOT a cause — the caller decides rescue.

**Failure handling (FR-T7)**: `decision = execErr != nil` (NOT `errors.Is(...)`). Any turn's Execute error OR any RenderMultiTurn error ⇒ Run aborts, returns the raw error as `cause`. The caller (CommitStaged L344) maps it to `&RescueError{Cause: cause}`.

**Run does NOT fork ParseOutput or run dedupe** — the caller runs the existing dedupe path (§6 above). Run does NOT check `cfg.MultiTurnFallback` or session_mode — S3 (CommitStaged) owns the trigger gate; RenderMultiTurn's own session_mode="append" gate (render.go:210-211) is defense-in-depth.

### 7e. `RenderMultiTurn` signature (render.go:203)
```go
203: func (m Manifest) RenderMultiTurn(model, sysPrompt, userPayload, reasoning, sessionID string, turn int) (*CmdSpec, error)
```
Gate at render.go:210-211:
```go
210: // *r.SessionMode is non-nil (S1 default ""), so the deref is safe.
211: if *r.SessionMode != "append" {
```

---

## 8. Tests — What's Covered

### 8a. `multiturn_test.go` (pure-helper + Run + gate-level)

**`chunkPayload` pure-math tests:**
- `TestChunkPayload_SingleChunk` (L46) — one chunk under budget, still carries `PART 1/1:`.
- `TestChunkPayload_MultiChunkSplit` (L64) — round-trip lossless + consistent N + 1..N indices.
- `TestChunkPayload_NewlineAnchoring` (L92) — boundary lands mid-line → forward-anchored, no fracture.
- `TestChunkPayload_EmptyPayload` (L119) — single empty chunk with `PART 1/1`.
- `TestChunkPayload_PrefixOutsideBudget` (L134) — body ≤ chunkTokens tokens (+1 anchor overshoot); prefix is one line.
- `TestChunkPayload_RuneBasedCJK` (L157) — rune-based sizing; CJK never split.
- `TestChunkPayload_CeilRounding` (L192) — N rounds UP.

**`Run` unit tests (stub provider):**
- `TestRun_HappyPath` (L248) — 2-chunk payload → 3 turns ("ok","ok",message); returns parsed msg, cause nil, ok true.
- `TestRun_TurnError` (L269) — global stub exit-1 ⇒ turn-1 abort, cause non-nil (FR-T7).
- `TestRun_FinalParseEmpty` (L293) — empty final stdout ⇒ ParseOutput ok=false; cause nil (not a cause).
- `TestRun_NonAppendManifest` (L310) — SessionMode unset ⇒ RenderMultiTurn session_mode gate errors on turn 1 ⇒ cause non-nil, mentions "session_mode".

**S4 exhaustive tests (L331-463):**
- `TestChunkPayload_CeilMath` (L346) — VERIFIED exact-N table (one/two/three chunks, 2.5×→3 ceil) + monotonicity.
- `TestChunkPayload_NoFracturedBoundaries` (L386) — mid-line-landing window; no fracture + lossless.
- `TestChunkPayload_PartPrefixMonotonic` (L411) — exact `^PART i/N:\n` regex; strict 1..N monotonicity.
- `TestChunkPayload_TokenLimitNonInteraction` (L437) — pure-helper layer: `chunkPayload(payload, chunkTokens)` has no TokenLimit param; structural independence. (Cross-ref: gate-level FR-T12 tested separately.)
- `TestMultiTurnTriggerGate_TruthTable` (L246) — **THE FR-T1 TRUTH TABLE**: 4 skip rows (flip each of a/b/c/d false) + 1 success row + 1 all-true control. Observes the gate's `VerboseWarn` trigger substring `"multi-turn fallback"` in a captured `*ui.Verbose` buffer. Drives `CommitStaged` directly (gate is inline, no predicate fn).
- `TestMultiTurnGate_TokenLimitNotATerm` (L366) — TokenLimit ∈ {0, 1000, 100000} (all non-truncating for the small diff); trigger present + commit lands regardless.
- `TestMultiTurnGate_TokenLimitTruncated_Recaptures` (L418) — **FR-T12 HEADLINE**: `token_limit=4` truncates one-shot diff BELOW `chunk_tokens=4`, but the re-capture makes condition (b) true on the untruncated payload → multi-turn fires + succeeds.

### 8b. `generate_multiturn_test.go` — the render-contract integration test

- `TestCommitStaged_MultiTurnRenderContract` (L46) — large staged file (≈600 tokens, N≥2 at chunkTokens=50). Asserts across ALL N+1 turns:
  - **(a)** commit lands (SHA hex, subject match, HEAD advanced, git log message match).
  - **(b)** exactly 1 one-shot call + N+1 multi-turn calls; counter cross-check (`STAGECOACH_STUB_COUNTER == N+2`).
  - **(c)** every multi-turn turn carries `--session-id <stable-id>`; `--no-session` (BareFlags) absent on all multi-turn turns; id is STABLE across all turns (FR-T6).
  - **(d)** turn-1-only `--system` flag: exactly ONE multi-turn turn carries it (turn 1); turns 2..N+1 omit it. Asserted on multi-turn subset only (one-shot Render also emits `--system` — no turn gate).
  - **Final-turn byte-exact argv** via `STAGECOACH_STUB_ARGSFILE` (holds only turn N+1).

### 8c. `generate_multiturn_failure_test.go` — failure/invariant integration

- `assertMultiTurnRescue` helper (L58) — asserts the FULL FR-T7/idempotent invariant: `re.TreeSHA == frozen write-tree`, `ParentSHA == pre-run HEAD`, HEAD unchanged (atomic-HEAD), staged index unchanged (name-set + full diff), AND `STAGECOACH_STUB_COUNTER == wantCalls` (discriminator: 1=gate skipped Run, 2=gate fired + Run aborted at turn 1).
- `TestCommitStaged_MultiTurnMidTurnFailureRescue` (L101) — (a) global `STAGECOACH_STUB_EXIT=1` ⇒ one-shot exits 1 but stdout "" ⇒ parse-fail ⇒ exhaust ⇒ gate fires ⇒ Run turn-1 exits 1 ⇒ abort ⇒ rescue. Counter==2. `re.Cause != nil` (the wrapped `*exec.ExitError` supersedes one-shot's lastCause).
- `TestCommitStaged_MultiTurnSmallPayloadSkip_RescueInvariant` (L135) — (b) tiny diff + default chunkTokens=32000 ⇒ condition (b) false ⇒ gate skips Run ⇒ existing rescue. Counter==1.
- `TestCommitStaged_MultiTurnNonAppendSkip_RescueInvariant` (L153) — (d) SessionMode unset (raw `stubtest.NewScript`, SessionMode nil) + large payload ⇒ condition (d) false ⇒ gate skips Run silently ⇒ existing rescue. Counter==1.

---

## 9. Architecture — How the Pieces Connect

```
CommitStaged (generate.go:109)
  │
  ├─ Step 1-5: RevParseHEAD, buildSystemPrompt, StagedDiff, WriteTree, recentSubjects
  │           (payload = diff, captured at L193-198; reserve computed at L176)
  │
  ├─ Step 6: GENERATION LOOP (L229-289)
  │   ├─ payload = BuildUserPayload(diff, cfg.Context, rejected)  [L231]
  │   ├─ [+ retryInstr prepend if parseFail]                       [L233]
  │   ├─ Manifest.Render(...) → provider.Execute(...) → ParseOutput(...)
  │   ├─ FinalizeMessage(m, cfg)  [BEFORE dedupe, L275]
  │   └─ IsDuplicate → break (success=true) | continue (rejected++)
  │
  ├─ if !success {                                  [L290, condition (a)]
  │   ├─ if cfg.MultiTurnFallback                   [L304, condition (c)]
  │   │     && *resolved.SessionMode == "append"    [L305, condition (d)]
  │   │   ├─ mtPayload := payload                   [L311]
  │   │   ├─ if cfg.TokenLimit != 0 {               [L312, FR-T12 re-capture]
  │   │   │     fullDiff = StagedDiff(TokenLimit:0, PromptReserveTokens:0)
  │   │   │     mtPayload = BuildUserPayload(fullDiff, ...)  [L323]
  │   │   │  }
  │   │   ├─ if EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {  [L330, condition (b)]
  │   │   │     turns = len(chunkPayload(mtPayload, chunkTokens)) + 1   [L332]
  │   │   │     fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", ...)
  │   │   │                                                            [L337]
  │   │   │     deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")  [L341]
  │   │   │     ┌─────────────────────────────────────────────────────┐
  │   │   │     │ multiturn.Run(ctx, deps, cfg, manifest,            │
  │   │   │     │   sysPrompt, mtPayload, msgModel, msgReasoning)    │ [L344]
  │   │   │     │  → chunkPayload(mtPayload, chunkTokens) → N chunks │
  │   │   │     │  → newSessionID() (one-run-scope)                  │
  │   │   │     │  → Turn 1: RenderMultiTurn(turn=1, sysPrompt,      │
  │   │   │     │              preamble + chunk[0]) → Execute        │
  │   │   │     │  → Turns 2..N: RenderMultiTurn(turn=i, sysPrompt="",│
  │   │   │     │              chunk[i-1]) → Execute                 │
  │   │   │     │  → Turn N+1: RenderMultiTurn(turn=N+1, sysPrompt="",│
  │   │   │     │              finalInstruction) → Execute → ParseOutput│
  │   │   │     │  → return (msg2, ok2, cause)                      │
  │   │   │     └─────────────────────────────────────────────────────┘
  │   │   │     ├─ cause==nil && ok2: FinalizeMessage(msg2) → IsDuplicate?
  │   │   │     │     not-dup → msg=finalMsg, success=true  [L350-352]
  │   │   │     │     dup → candidate=finalMsg              [L355]
  │   │   │     └─ else: lastCause=cause; candidate=msg2    [L359-364]
  │   │   │  }
  │   │  }
  │   ├─ if !success { return &RescueError{Kind:ErrRescue, ...} }  [L369-373, FR-T7 byte-identical]
  │  }
  │
  ├─ EditMessage (FR-E1 gate, L378)
  ├─ CommitTree (L399)
  ├─ UpdateRefCAS (L411) — sole ref mutation
  └─ DiffTree (L435) → Result
```

### Key dependency facts
- **Single estimator**: `git.EstimateTokens` (tokens.go:25, `ceil(runeCount/4)`) used by condition (b) AND by `chunkPayload`'s `chunkTokens*4` rune budget. Consistency is load-bearing.
- **`payload` (L226) and `success` (L227) hoisted** so the gate (L290+) can read the last one-shot payload.
- **`mtPayload` reuses `payload`** when `TokenLimit==0`; otherwise re-captures with `TokenLimit:0` (FR-T12).
- **Dedupe runs ONCE post-Run** (CommitStaged L348-356); Run does NOT dedupe. One-shot parity: Finalize BEFORE dedupe (D3).
- **The rescue (L369-373) is byte-identical** to the pre-multi-turn version — FR-T7 requires multi-turn failure to be indistinguishable from one-shot exhaustion (exit 3, not 124).
- **`resolved` = `deps.Manifest.Resolve()`** at L219; `resolved.SessionMode` is a `*string` (nil ⇒ default "" from Resolve, render.go:177-178).

---

## 10. Start Here

Open **`internal/generate/generate.go:290`** (the `if !success {` line) and read down to L374. This is the entire FR-T1 gate + FR-T12 re-capture + post-Run dedupe + byte-identical rescue. Everything in this region (L290-373) is the multi-turn surface that must be propagated verbatim to the two target paths. The supporting `multiturn.Run` is `internal/generate/multiturn.go:136`; the supporting `chunkPayload` is `internal/generate/multiturn.go:52`.

---

## 11. Residual Notes / Propagation Risks

1. **Hoist seam (D1)**: `var payload string` MUST be hoisted out of the loop (L226). If a target path keeps payload loop-local, the gate cannot evaluate condition (b).
2. **Gate ordering**: (c)+(d) checked FIRST (L304-305), THEN FR-T12 re-capture (L312-326), THEN (b) on `mtPayload` (L331). Reordering breaks FR-T12 (truncated payload → false (b) → multi-turn skipped).
3. **`mtPayload` fallback on re-capture error** (L325): silently reuses the truncated one-shot payload. No warning. Best-effort by design.
4. **Missing per-chunk token estimate in verbose** (L332-341): only turn count + total minutes are printed; the per-chunk token sizes from `chunkPayload` are never logged. `git.EstimateTokens(mtPayload)` is computed at L330 but not printed.
5. **`success = true` semantics**: the post-Run dedupe sets `success = true` ONLY on not-dup (L352). A dup post-Run leaves `success = false` → falls through to rescue (L369) with `candidate = finalMsg`.
6. **FR-T7 cause handling**: `lastCause = cause` (L360) supersedes one-shot's lastCause only when `cause != nil`. A final-turn-empty (`ok2==false`, `cause==nil`) does NOT touch `lastCause`.
7. **RenderMultiTurn defense-in-depth** (render.go:210-211): even if CommitStaged's gate is bypassed, RenderMultiTurn itself errors on non-append manifests. `newSessionID` (multiturn.go:194) is the FR-T6 one-run-scope id.

---

# Acceptance Report

This is a research/scouting task: read-only investigation, no code changes. The "change" delivered is this findings document at the authoritative output path.

```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "Read-only investigation completed within scope: mapped CommitStaged multi-turn gate (generate.go:290-373), multiturn.Run (multiturn.go:136), chunkPayload (multiturn.go:52), FR-T1 4 conditions, mtPayload FR-T12 re-capture (L311-329), FR-T11 verbose (L332-341), post-Run dedupe (L344-366), and all 3 test files. No source files modified."
    }
  ],
  "changedFiles": [],
  "testsAddedOrUpdated": [],
  "commandsRun": [
    {
      "command": "grep -n for var hoists, gate, mtPayload, EstimateTokens in generate.go",
      "result": "passed",
      "summary": "Confirmed exact line numbers L221-226 (hoists), L226 (payload), L290 (gate), L311 (mtPayload), L330 (cond b), L344 (Run), L349 (Finalize), L350 (IsDuplicate)."
    },
    {
      "command": "grep -n for func/const/type in multiturn.go",
      "result": "passed",
      "summary": "Confirmed chunkPayload L52, Run L136, preambleFmt L106, finalInstruction L110, newSessionID L194."
    },
    {
      "command": "read generate.go, multiturn.go, 3 test files, tokens.go, render.go, config.go",
      "result": "passed",
      "summary": "Full read of all 8 relevant files; findings documented with exact line numbers."
    }
  ],
  "validationOutput": [
    "generate.go multi-turn gate region L290-373 mapped line-by-line.",
    "FR-T1 4 conditions identified: (a) !success L290, (b) EstimateTokens(mtPayload)>MultiTurnChunkTokens L330, (c) cfg.MultiTurnFallback L304, (d) *resolved.SessionMode=='append' L305.",
    "mtPayload two-case handling documented: TokenLimit==0 → reuse payload (L311); TokenLimit!=0 → re-capture StagedDiff(TokenLimit:0) + BuildUserPayload (L312-326).",
    "FR-T11 verbose: 2 lines (stderr '↳ falling back...' L337 + VerboseWarn L341); per-chunk token estimate confirmed MISSING.",
    "Post-Run dedupe: Finalize→IsDuplicate (L348-350); 3 branches (success/dup/cause) documented L346-365.",
    "multiturn.Run N+1 protocol documented: Turn1 (sysPrompt+preamble+chunk1), Turns2..N (chunks), TurnN+1 (finalInstruction); cause=execErr!=nil (NOT errors.Is).",
    "Test coverage documented: chunkPayload 9 tests, Run 4 tests, gate truth table, render contract, failure invariants."
  ],
  "residualRisks": [
    "No code changes; this is a read-only reference document. Propagation to the two target paths is the parent's responsibility.",
    "Per-chunk token estimate is absent from verbose output (L332-341) — flagged as a known gap per the task brief; propagation should decide whether to add it."
  ],
  "noStagedFiles": true,
  "diffSummary": "No diff. Findings written to /home/dustin/projects/stagecoach/.pi-subagents/artifacts/outputs/9ac638be/plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/architecture/research_commitstaged_reference.md and progress to progress.md. No source files in the repo were modified.",
  "reviewFindings": [
    "no blockers — read-only research task complete"
  ],
  "manualNotes": "The two target paths for propagation must preserve: (1) the var payload hoist at L226, (2) gate ordering (c)+(d) before FR-T12 re-capture before (b), (3) mtPayload two-case handling, (4) Finalize-before-dedupe post-Run, (5) byte-identical rescue at L369-373. The per-chunk token estimate gap (finding #5 in §11) is the only known omission from FR-T11 verbose."
}
```
