# Research: `internal/hook/exec.go` — hook generation loop & multi-turn propagation

Scope: investigate the git-hook `prepare-commit-msg` exec path (PRD §9.20 FR-H4/H5/H6),
its generate→parse→dedupe loop, exhaustion handling, the FR-H5 never-block contract,
and whether the FR-T1 multi-turn fallback (currently only in `generate.CommitStaged`)
can be propagated here while preserving FR-H5.

All line numbers refer to the files as read on 2026-07-05.

---

## 1. Files Retrieved

1. `internal/hook/exec.go` (1-206, full) — the hook runtime: `Run` (97), `WriteMessageFile` (46-61),
   the source/empty-diff gates, and the generate→parse→dedupe loop (157-205).
2. `internal/cmd/hookexec.go` (1-138, full) — the cobra leaf that calls `hook.Run` and applies the
   FR-H5 never-block exit-code mapping via the `neverBlock` closure (67-72, called at 137).
3. `internal/generate/generate.go` (1-60, 155-375+) — `CommitStaged` and its FR-T1 multi-turn trigger
   gate (the reference implementation to mirror).
4. `internal/generate/multiturn.go` (1-52, 130-205) — `Run` (the multi-turn N+1 protocol, 136) and
   `chunkPayload` (52); the importable seam hook.Run would call.
5. `internal/hook/exec_test.go` (1-end) — proves the never-block / msg-file-untouched contract on
   exhaustion (`TestRun_DuplicateRejected`, `TestRun_StubExit1_NeverBlock`, `TestRun_TimeoutNeverBlock`).
6. `internal/provider/manifest.go` / `builtin.go` — `Resolve()` (150), `SessionMode *string` (66);
   pi ships `"append"` (builtin.go:54).
7. `docs/how-it-works.md` (262-324) — multi-turn fallback § and "Hook mode vs snapshot-based flow".

---

## 2. The `hook.Run` loop (exec.go:157-205)

```go
// exec.go:154-205
var rejected []string
var parseFail bool

for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
    payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)   // 158
    if parseFail {
        payload = retryInstr + "\n\n" + payload                       // 160, FR29 corrective preamble
    }
    spec, rerr := deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning) // 162
    if rerr != nil { return fmt.Errorf("hook render: %w", rerr) }     // 163-165
    out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose) // 167
    if execErr != nil {
        if errors.Is(execErr, context.DeadlineExceeded) {
            return errors.New("stagehand: hook generation timed out") // 170-171, no retry; never-block
        }
        if errors.Is(execErr, context.Canceled) {
            return errors.New("stagehand: hook generation cancelled") // 173-174
        }
        // Non-zero exit (*exec.ExitError): fall through to ParseOutput (partial stdout may be valid). 176
    }
    m, ok, _ := provider.ParseOutput(out, deps.Manifest)              // 179
    if !ok {
        parseFail = true
        if deps.Verbose != nil { deps.Verbose.VerboseRetry(attempt+1, "parse failed ...") } // 182-184
        continue                                                       // 185, FR29 retry
    }
    parseFail = false
    m = generate.FinalizeMessage(m, cfg)                              // 187, FR-F8 template BEFORE dedupe
    subject := generate.ExtractSubject(m)                             // 189
    if generate.IsDuplicate(subject, recent) {                        // 192
        rejected = append(rejected, subject)
        if deps.Verbose != nil { deps.Verbose.VerboseRetry(attempt+1, "subject ... matches ...") } // 195
        continue                                                       // 196, FR32 retry
    }
    return WriteMessageFile(msgFile, m)                               // 201, SUCCESS
}
// 204: exhaustion after bounded retries
return fmt.Errorf("stagehand: hook generation failed after %d retries", cfg.MaxDuplicateRetries) // 205
```

### Loop structure
- One bounded counter shared by parse-fail (FR29) and duplicate (FR32) retries — identical policy to
  `CommitStaged` (generate.go step 6).
- Per attempt: rebuild `payload` (with rejection list + optional `retryInstr` corrective preamble) →
  `Manifest.Render` → `provider.Execute` → `provider.ParseOutput` → `FinalizeMessage` → `ExtractSubject`
  → `IsDuplicate`.
- Exit-on-failure (never-block): timeout/cancel return immediately (170-174); render errors return
  immediately (163-165); on exhaustion the loop falls through to line 205.

### Exhaustion handling (exec.go:204-205)
- **Warning printed?** NO. `exec.go` itself prints nothing. It returns the descriptive error
  `"stagehand: hook generation failed after %d retries"`. The single stderr line is emitted by the
  **cmd layer** (`hookexec.go:69` `fmt.Fprintf(stderr, "stagehand: %s\n", err)` inside `neverBlock`).
- **Empty message written to commit-msg file?** NO. `WriteMessageFile` (exec.go:201) is invoked ONLY on
  the success path. On exhaustion (and on timeout/cancel/render-error) the `msgFile` is left byte-for-byte
  untouched. Verified by `TestRun_DuplicateRejected` and `TestRun_StubExit1_NeverBlock`, which assert the
  msg-file equals its pre-Run content on exhaustion.

---

## 3. FR-H5 never-block contract

The contract is enforced at TWO layers:

**(a) `hook.Run` (exec.go):** never writes the file on any failure path. Every non-success branch
returns a descriptive `error` (timeout/cancel/exhaustion/render-error). The package doc (exec.go:1-7)
states the invariant: *"...any failure → descriptive error, <msg-file> UNTOUCHED."*

**(b) `cmd/hookexec.go:67-72` — the `neverBlock` closure:**
```go
neverBlock := func(err error) error {
    fmt.Fprintf(stderr, "stagehand: %s\n", err)  // ONE stderr line
    if flagHookExecStrict {
        return exitcode.New(exitcode.Error, nil)  // --strict → exit 1, aborts the commit (silent)
    }
    return nil  // default → exit 0 → commit proceeds to an empty editor
}
```
Wired at `hookexec.go:137`:
```go
if rerr := hook.Run(ctx, ..., msgFile, source); rerr != nil && !errors.Is(rerr, hook.ErrNoOp) {
    return neverBlock(rerr)
}
return nil  // success OR ErrNoOp → exit 0
```

**Exit-0 guarantee:** `neverBlock` returns `nil` (cobra `RunE` nil ⇒ process exit 0) for every
generation error unless `--strict` is set. `ErrNoOp` (source gate / empty diff) is a separate intended
no-op that also exits 0 silently. The msg-file is untouched on every error branch, so `git` opens the
editor on the original (comment-block) content → empty message ⇒ commit proceeds without an AI message.

---

## 4. Variables in scope at the exhaustion point (exec.go:205)

| Variable | Type | Declared | Notes |
|---|---|---|---|
| `ctx` | `context.Context` | param (97) | |
| `deps` | `generate.Deps` | param (97) | `.Git`, `.Manifest`, `.Verbose`, `.Excludes`, `.Progress` |
| `cfg` | `config.Config` | param (97) | carries `MultiTurnFallback`, `MultiTurnChunkTokens`, `TokenLimit`, `Timeout`, `MaxDiffBytes`, … |
| `msgFile` | `string` | param (97) | git's `prepare-commit-msg` file path |
| `source` | `string` | param (97) | already past the gate |
| `isUnborn` | `bool` | 109 | from `RevParseHEAD` |
| `sysPrompt` | `string` | 118 | built ONCE, stable across attempts (reusable for multi-turn turn 1) |
| `reserve` | `int` | 120 | prompt-reserve token count (unused by multi-turn) |
| `diff` | `string` | 134 | the (possibly `token_limit`-truncated) staged diff |
| `recent` | `[]string` | 144 | recent subjects for dedupe |
| `msgModel`, `msgReasoning` | `string` | 146 | from `config.ResolveRoleModel("message", cfg)` |
| `retryInstr` | `string` | 151 | from `*deps.Manifest.Resolve().RetryInstruction` |
| `rejected` | `[]string` | 154 | accumulated duplicate subjects |
| `parseFail` | `bool` | 155 | last attempt's parse status |

**NOT in scope at exhaustion (vs CommitStaged):**
- `resolved` (Manifest) — CommitStaged binds it (`generate.go:244 resolved := deps.Manifest.Resolve()`);
  hook pulls only `RetryInstruction` inline (151) and discards the rest. **Needed** for `resolved.SessionMode`.
- `payload` — CommitStaged hoists it (`var payload string`, generate.go:249) so the last-built payload
  survives for the FR-T1 token-estimate gate. **Hook recomputes it loop-locally (158) and it does NOT
  survive the loop.** Must be hoisted to propagate multi-turn.
- `candidate`, `lastCause`, `msg`, `success`, `treeSHA`, `parentSHA` — all absent (hook has no snapshot /
  no rescue; these are CommitStaged-only rescue plumbing).

---

## 5. CommitStaged vs hook.Run loop — shared / different

**Shared (the loop body is a near-clone):**
- Same single bounded counter for FR29 (parse-fail) + FR32 (duplicate).
- Same `prompt.BuildUserPayload` + `retryInstr` corrective preamble.
- Same `Manifest.Render` → `provider.Execute` → `provider.ParseOutput` → `FinalizeMessage` →
  `ExtractSubject` → `IsDuplicate` ordering (Finalize BEFORE dedupe — §9.7).
- Same `cfg.MaxDuplicateRetries` bound.

**Different:**

| Aspect | `CommitStaged` | `hook.Run` |
|---|---|---|
| Snapshot | `WriteTree` + `signal.SetSnapshot` (step 4) | **none** — git owns the commit |
| Timeout/cancel | → `*RescueError{Kind: ErrTimeout/ErrRescue}` (snapshot rescue) | → plain `error` ("timed out"/"cancelled"), never-block |
| `Verbose` calls | unguarded (`deps.Verbose.VerboseRetry(...)`) | nil-guarded (`if deps.Verbose != nil`) |
| Success action | set `msg`+`success`, `break`, then commit plumbing | `return WriteMessageFile(msgFile, m)` directly |
| Exhaustion | **FR-T1 multi-turn gate**, else `*RescueError` | `fmt.Errorf("hook generation failed after %d retries")` — **no multi-turn** |
| Hoisted `payload` | yes (FR-T1 gate needs it) | no (loop-local) |
| Bound `resolved` | yes (`SessionMode` for the gate) | no (only `RetryInstruction` pulled) |

The differences that matter for multi-turn propagation are exactly the last three rows.

---

## 6. Can multi-turn be propagated to hook.Run while preserving FR-H5?

**YES.** The FR-T1 multi-turn gate in `CommitStaged` (generate.go:274-352) is self-contained and
exit-0-compatible. The never-block contract is enforced at the **cmd layer** (`hookexec.go` neverBlock),
which is independent of the loop's internals: any `error` returned from `hook.Run` already maps to
exit 0 + one stderr line + untouched msg-file. Adding a multi-turn attempt before the exhaustion return
does not change that mapping — it only adds one more success opportunity.

### Propagation recipe (minimal, scope-preserving)

1. **Bind `resolved`** before the loop (replace inline 151):
   ```go
   resolved := deps.Manifest.Resolve()
   retryInstr := *resolved.RetryInstruction
   ```
2. **Hoist `payload`** so the last-built one-shot payload survives:
   ```go
   var rejected []string
   var parseFail bool
   var payload string  // hoisted: survives the loop for the FR-T1 gate
   for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
       payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)  // assign, not :=
       ...
   }
   ```
3. **Insert the FR-T1 gate between the loop and the exhaustion return** (exec.go:204), mirroring
   generate.go:274-352. The relevant `generate.Run` is already importable (hook already imports
   `internal/generate`):
   ```go
   // FR-T1 multi-turn fallback gate — mirrors CommitStaged. Preserves FR-H5: on any multi-turn
   // failure (cause != nil / parse empty / duplicate) we fall through to the exhaustion error,
   // which the cmd layer maps to exit 0 (neverBlock). WriteMessageFile runs ONLY on success.
   if cfg.MultiTurnFallback &&
       resolved.SessionMode != nil && *resolved.SessionMode == "append" {

       // FR-T12: re-capture the diff with TokenLimit=0 when token_limit truncated the one-shot payload.
       mtPayload := payload
       if cfg.TokenLimit != 0 {
           if fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{ /* TokenLimit: 0, ... */ }); derr == nil {
               mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
           }
       }
       // Condition (b): payload must exceed one chunk for multi-turn to help.
       if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {
           turns := len(generate.ChunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1
           totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
           if totalMin < 1 { totalMin = 1 }
           fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)
           if deps.Verbose != nil { deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback") }

           msg2, ok2, cause := generate.Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)
           if cause == nil && ok2 {
               finalMsg := generate.FinalizeMessage(msg2, cfg)
               if !generate.IsDuplicate(generate.ExtractSubject(finalMsg), recent) {
                   return WriteMessageFile(msgFile, finalMsg)  // SUCCESS — only write site (FR-H5 preserved)
               }
           }
       }
   }
   return fmt.Errorf("stagehand: hook generation failed after %d retries", cfg.MaxDuplicateRetries)
   ```
   (`generate.ChunkPayload` is currently unexported as `chunkPayload` (multiturn.go:52) — it would need
   exporting, OR the turn-count estimate can reuse `git.EstimateTokens(mtPayload)/cfg.MultiTurnChunkTokens`
   without exporting. Simplest: export `ChunkPayload`, or compute turns from the token ratio.)

### FR-H5 preservation analysis (each multi-turn failure mode)

| Multi-turn outcome | hook.Run returns | cmd neverBlock | exit | msg-file |
|---|---|---|---|---|
| `cause != nil` (turn error/timeout/cancel) | exhaustion error (fall through) | prints + exit 0 | **0** | untouched ✓ |
| `ok2 == false` (final parse empty) | exhaustion error (fall through) | prints + exit 0 | **0** | untouched ✓ |
| duplicate subject | exhaustion error (fall through) | prints + exit 0 | **0** | untouched ✓ |
| success + non-dup | `WriteMessageFile(msgFile, finalMsg)` → `nil` | exit 0 | **0** | written ✓ |

In every case exit is 0 (default; `--strict` still aborts on the error path unchanged) and the msg-file is
written ONLY on success. **FR-H5 holds.** The only behavioral change for users: large diffs that
previously exhausted one-shot now get one extra lossless multi-turn attempt before the never-block exit.

---

## 7. Docs / FAQ on multi-turn in hook mode

- `docs/how-it-works.md:262-296` (multi-turn §) describes the feature purely in the snapshot-flow
  context ("...then commits like any other message", "...control passes to the standard rescue protocol").
- `docs/how-it-works.md:300-324` ("Hook mode vs the snapshot-based flow") lists the hook contract
  (never-block, no snapshot, no rescue) and **does NOT mention multi-turn** — implicitly it is
  unimplemented in hook mode today. There is no FAQ entry stating multi-turn is unavailable in hook mode.
- `docs/configuration.md:155-157` and `docs/providers.md:40-49` describe the two knobs
  (`multi_turn_fallback`, `multi_turn_chunk_tokens`) and the `session_mode="append"` capability gate
  generically — they are not path-specific.

**Doc-debt if propagated:** the hook-mode section (how-it-works.md:300-324) should note that multi-turn
is available in hook mode too (as an extra attempt, not a rescue — it composes with never-block). This is
an accuracy update, not a blocker.

---

## 8. Risks & open questions

1. **`chunkPayload` export.** `multiturn.go:52` is package-private. The hook's turn-count estimate
   (for the stderr "N turns, ~Nm total" line) either needs `generate.ChunkPayload` exported or a
   token-ratio approximation. Low risk; choose one.
2. **Verbose nil-guarding inconsistency.** The hook loop nil-guards `deps.Verbose`; CommitStaged does
   not. In production `deps.Verbose` is always non-nil (`hookexec.go:120`). Propagation should pick one
   style (the snippet above keeps the hook's nil-guard for safety).
3. **Multi-turn turn-timeout surfacing.** The one-shot timeout path returns a specific
   `"stagehand: hook generation timed out"` message (exec.go:170). A multi-turn turn-timeout surfaces as
   `cause` (multiturn.go:142/152/163/173) and would currently fall through to the generic exhaustion
   error. Acceptable for FR-H5 (still exit 0), but the stderr line is less specific. Optional: wrap
   `cause` into a clearer message.
4. **No hook multi-turn tests exist.** Propagation needs new tests mirroring
   `generate_multiturn_test.go` / `generate_multiturn_failure_test.go` (trigger gate conditions,
   success-writes-file, all-failure-modes-leave-file-untouched-and-exit-0).
5. **`source` / `--strict` unaffected.** The source gate (exec.go:99) and the `--strict` flag
   (hookexec.go:70) are upstream/downstream of the loop and need no change.
6. **FR-T7 parity.** generate.Run aborts on any turn error/timeout and returns `(msg, ok, cause)`; the
   hook treats `cause != nil` as failure → never-block. Consistent with FR-T7.

---

## 9. Start here

Open `internal/hook/exec.go` and read `Run` (97-206), focusing on the loop (157-205) and the exhaustion
return (204-205). Then read `internal/generate/generate.go:244-352` (CommitStaged's loop tail + FR-T1
gate) as the copy-source. The cmd-layer contract that FR-H5 depends on lives in
`internal/cmd/hookexec.go:67-72,137` and does not need editing.
