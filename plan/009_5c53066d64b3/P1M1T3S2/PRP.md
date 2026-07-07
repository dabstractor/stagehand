---
name: "P1.M1.T3.S2 — multiturn.go N+1 turn protocol (Run): session id, priming, per-turn Execute, final parse"
description: |
  Add `func Run` (the N+1 turn protocol) + `newSessionID` to `internal/generate/multiturn.go` (the file
  S1 creates with `chunkPayload`/`chunk`). PRD §9.24 FR-T4 (N+1 turn protocol verbatim), FR-T5 (per-turn
  timeout = cfg.Timeout), FR-T7 (failure → rescue; any turn error/timeout aborts), FR-T10 (message role
  only), FR-T2 (lossless — the SAME captured payload, chunked, no truncation). Signature (FIXED by the
  contract): `func Run(ctx, deps Deps, cfg config.Config, manifest provider.Manifest, sysPrompt, payload,
  msgModel, msgReasoning string) (msg string, ok bool, cause error)`. Steps: (1) `chunks :=
  chunkPayload(payload, cfg.MultiTurnChunkTokens)`, `N := len(chunks)` (S1's helper); (2) `sessionID :=
  newSessionID()` (crypto/rand, 16 bytes → "stagecoach-<32hex>"; NO uuid lib exists in the repo; mint
  per-run, never resumed); (3) priming preamble verbatim with N interpolated; (4) Turn 1:
  `manifest.RenderMultiTurn(msgModel, sysPrompt, preamble+"\n\n"+chunks[0].text, msgReasoning, sessionID,
  1)` (sys prompt via the flag, turn=1); `provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)`; ANY
  execErr/renderErr ⇒ return ("", false, cause) [FR-T7]; discard the "ok" stdout; (6) Turns 2..N:
  RenderMultiTurn(msgModel, "", chunks[i-1].text, …, sessionID, i) (sysPrompt="" → no system flag,
  turn>1); Execute; discard "ok"; (7) Turn N+1 (final): finalInstruction verbatim; RenderMultiTurn(…,
  "", finalInstruction, …, sessionID, N+1); Execute → `out`; (8) execErr ⇒ return ("", false, cause);
  (9) `m, parseOK, _ := provider.ParseOutput(out, manifest); return (m, parseOK, nil)` — the CALLER
  (P1.M1.T3.S3) runs dedupe via CommitStaged's existing path. KEY DECISION (D1): `execErr != nil` (ANY
  error aborts — timeout/cancel/non-zero-exit/start-fail/render-fail), NOT `errors.Is(...)` — FR-T7
  treats ALL of those as abort, and intermediate turns discard stdout anyway (so no value in parsing
  partial output, unlike the one-shot path's fall-through). Run returns the RAW cause; it does NOT
  construct *RescueError (S3 maps cause → &RescueError{Cause: cause}). Mocking: provider.Execute is the
  seam; the stub agent (cmd/stubagent) + stubtest.NewScript simulate append via call-indexed scripted
  output ("ok"…"ok"…<message>). Adds a FOCUSED 4-test smoke (happy/turn-error/final-parse-empty/
  non-append-manifest); T4 (P1.M1.T4) extends with the exhaustive integration matrix. Touches ONLY
  multiturn.go (add Run + newSessionID + 2 constants + imports) and multiturn_test.go (add 4 tests).
  NO trigger gate (S3), NO exhaustive matrix (S4), NO how-it-works doc (S4), NO integration tests (T4).
---

## Goal

**Feature Goal**: Land the N+1 turn protocol — the transport layer of the multi-turn fallback (PRD
§9.24). After S1's `chunkPayload` splits the captured diff into N request-sized chunks, `Run` drives
the N+1 sequential provider invocations against ONE session id: turn 1 (system prompt + priming
preamble + chunk 1), turns 2..N (each chunk), turn N+1 (the "now write the message" request). It
returns the final turn's parsed message for the caller to dedupe, or a non-nil `cause` on any
turn-level failure (FR-T7). Lossless (FR-T2): the model sees the whole diff in its session history.

**Deliverable** (two files MODIFIED — S1 creates them; S2 extends):
1. **MODIFY** `internal/generate/multiturn.go`: add `func Run` + `func newSessionID` + two package-level
   constants (`preambleFmt`, `finalInstruction`) + the new imports (context, crypto/rand, encoding/hex,
   time, internal/config, internal/provider). `chunkPayload`/`chunk`/`advanceRunes`/`ceilDiv` (S1's
   deliverables) are untouched.
2. **MODIFY** `internal/generate/multiturn_test.go`: add 4 focused Run smoke tests (happy / turn-error
   / final-parse-empty / non-append-manifest). S1's chunkPayload smoke tests stay.

No other files touched. No production code outside multiturn.go. No docs.

**Success Definition**: `go build/vet/gofmt` clean; `go test -race ./internal/generate/` green with the
4 new Run tests + S1's chunk tests passing; `Run` drives exactly N+1 provider invocations against one
session id, with the system-prompt flag on turn 1 only, `--session-id` on every turn, `--no-session`
dropped (all enforced by `RenderMultiTurn`); a happy-path 2-chunk run returns the final turn's parsed
message with `cause==nil, ok==true`; any turn Execute error or render error ⇒ `cause != nil`; the final
turn's parse failure ⇒ `ok==false, cause==nil` (caller decides rescue); the session id is a fresh
`stagecoach-<32hex>` per call.

## User Persona

**Target User**: The contributor implementing P1.M1.T3.S3 (the FR-T1 trigger gate wiring in
`CommitStaged`), which is `Run`'s sole caller. S3 decides WHEN to invoke multi-turn (one-shot exhausted
+ payload exceeds one chunk + `multi_turn_fallback` enabled + `session_mode="append"`), passes the
captured payload + resolved message-role model + system prompt, then maps `Run`'s `(msg, ok, cause)` to
either the existing dedupe path (`cause==nil`) or `&RescueError{Cause: cause}` (`cause != nil`).

**Use Case**: A 266K-token diff that exhausted one-shot retries (the provider's per-request reliability
ceiling lies below its context window) is delivered losslessly across N+1 session turns. `Run` is the
"deliver the diff in pieces, then ask for the message" orchestration; the model sees the entire diff in
its session history and writes one message at the end.

**User Journey**: S3 → `msg, ok, cause := multiturn.Run(ctx, deps, cfg, msgManifest, sysPrompt, payload,
msgModel, msgReasoning)` → if `cause != nil`: map to `&RescueError{Cause: cause}` (rescue); if `ok ==
false`: rescue (final turn produced no parseable message); else: hand `msg` to CommitStaged's existing
dedupe loop (unchanged).

**Pain Points Addressed**: Gives S3 a single, focused, testable entry point that hides the session
lifecycle (id minting, turn counting, per-turn Execute, the FR-T4 verbatim prompts). S3 need only decide
trigger + map the return. Prevents the two classic protocol bugs: (a) re-sending the system prompt on
later turns (the session already carries it — `RenderMultiTurn` turn-1-only handles this); (b) treating
a turn timeout as "skip and continue" (FR-T7 mandates abort — `Run` surfaces it as `cause`).

## Why

- **PRD §9.24 FR-T4 (turn protocol) IS the spec:** turn 1 (system prompt + preamble + chunk 1), turns
  2..N ("PART i/N:" + chunk i), turn N+1 ("Now write the commit message…"). Intermediate "ok" discarded;
  the final turn's stdout is parsed by the EXISTING §9.6 pipeline. `Run` IS FR-T4's transport.
- **FR-T5 (per-turn timeout):** each turn is a separate `provider.Execute(ctx, *spec, cfg.Timeout, …)`
  with `cfg.Timeout` (default 120s) as the per-turn budget. `Execute` shadows ctx with
  `context.WithTimeout` — Run just passes `cfg.Timeout` through.
- **FR-T7 (failure → rescue):** "On any of — a turn's provider error (non-zero exit that is not a
  timeout), a turn timeout, or the final turn's output failing to parse/dedupe — stagecoach aborts." Run
  surfaces the first three as `cause`; the parse/dedupe outcome is `(msg, ok)` for the caller.
- **FR-T2 (lossless):** `Run` sends the captured payload unchanged, only chunked (S1's `chunkPayload`).
  No truncation, no summarization — the model sees the whole diff across the session.
- **FR-T10 (message role only):** `Run` takes the message-role `(msgModel, msgReasoning, sysPrompt,
  payload)` — it serves the single-commit §13.1–§13.5 path. The planner/stager/arbiter are out of scope.
- **Foundation for S3/S4/T4:** S3 (the gate+caller), S4 (exhaustive tests + doc), T4 (integration) all
  build on a verified `Run`. Landing it now with a focused smoke test lets those subtasks proceed.

## What

`Run` is a sequential N+1-turn loop over one session id. The exact body is in §Implementation Blueprint.
Behaviorally:

1. **Chunk** the payload via S1's `chunkPayload(payload, cfg.MultiTurnChunkTokens)` → `chunks`, `N`.
2. **Mint** a fresh session id (`newSessionID()` — `crypto/rand`, 16 bytes, `"stagecoach-"+hex`).
3. **Turn 1:** `RenderMultiTurn(msgModel, sysPrompt, preamble+"\n\n"+chunks[0].text, msgReasoning,
   sessionID, 1)` → `Execute(ctx, *spec, cfg.Timeout, deps.Verbose)`. Discard stdout. ANY error ⇒ `cause`.
4. **Turns 2..N:** `RenderMultiTurn(msgModel, "", chunks[i-1].text, msgReasoning, sessionID, i)` →
   `Execute`. Discard. ANY error ⇒ `cause`.
5. **Turn N+1:** `RenderMultiTurn(msgModel, "", finalInstruction, msgReasoning, sessionID, N+1)` →
   `Execute` → `out`. ANY error ⇒ `cause`.
6. **Parse** the final `out` via `provider.ParseOutput(out, manifest)` → `(m, parseOK, _)`. Return
   `(m, parseOK, nil)`.

`preambleFmt` and `finalInstruction` are verbatim from FR-T4 (constants in multiturn.go). The system
prompt is passed ONLY on turn 1 (RenderMultiTurn's turn-1-only gate enforces it). `--session-id` is on
every turn; `--no-session` is dropped (RenderMultiTurn's session-flags block enforces both).

**Return contract:**
- `cause != nil` ⟺ a turn aborted (Execute returned non-nil err, OR RenderMultiTurn returned a non-nil
  render error — e.g. a non-append provider). Then `msg == ""`, `ok == false`. The raw error is returned
  (NOT wrapped in `*RescueError` — the caller does that).
- `cause == nil` ⟹ the final turn's Execute succeeded. `(msg, ok)` = ParseOutput's result. `ok == false`
  ⟹ the final turn's stdout did not parse to a non-empty message (the caller treats this as rescue per
  FR-T7's "final turn's output failing to parse"). `ok == true` ⟹ `msg` is ready for dedupe.

### Success Criteria

- [ ] `func Run(ctx, deps Deps, cfg config.Config, manifest provider.Manifest, sysPrompt, payload,
      msgModel, msgReasoning string) (msg string, ok bool, cause error)` exists in multiturn.go.
- [ ] `Run` calls `chunkPayload` once; `N = len(chunks)`; the preamble interpolates N.
- [ ] Turn 1 uses `RenderMultiTurn(…, sysPrompt, preamble+"\n\n"+chunks[0].text, …, sessionID, 1)`.
- [ ] Turns 2..N use `RenderMultiTurn(…, "", chunks[i-1].text, …, sessionID, i)` (sysPrompt="").
- [ ] Turn N+1 uses `RenderMultiTurn(…, "", finalInstruction, …, sessionID, N+1)`.
- [ ] EVERY turn's Execute error AND every RenderMultiTurn error ⇒ `return "", false, <err>` (FR-T7).
- [ ] The final turn's stdout is parsed via `provider.ParseOutput(out, manifest)`; Run returns
      `(m, parseOK, nil)` — NO forked ParseOutput, NO dedupe.
- [ ] `newSessionID()` returns `"stagecoach-"+hex(16 random bytes)`; never panics (crypto/rand fallback).
- [ ] `Run` does NOT construct `*RescueError` (returns the raw cause; the caller wraps).
- [ ] `Run` does NOT check `cfg.MultiTurnFallback` or `session_mode` (S3 owns the gate; RenderMultiTurn's
      own session_mode gate is the defense-in-depth, surfaced as `cause`).
- [ ] The 4 focused Run smoke tests exist in multiturn_test.go and PASS.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] `git diff --stat` shows ONLY `internal/generate/multiturn.go` + `internal/generate/multiturn_test.go`.
- [ ] NO trigger gate (S3), exhaustive matrix (S4), how-it-works doc (S4), or integration tests (T4).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim `Run` body + `newSessionID` + the two constants
(copy-paste-ready), the verbatim `provider.Execute` / `RenderMultiTurn` / `ParseOutput` signatures and
return contracts (verified at executor.go/render.go:203/parse.go:41), the verbatim S1 `chunk`/
`chunkPayload` contract, the exact 4-test smoke matrix with the stub seam (stubtest.NewScript +
SessionMode="append" override), the `execErr != nil` decision (D1), the no-uuid-lib finding
(crypto/rand), and the hard scope fences. The freeze-safe `RenderMultiTurn` enforces session_mode and
turn-1-only system prompt — Run need not duplicate those checks. No inference required.

### Documentation & References

```yaml
# MUST READ — the FR spec, the seam contracts, and this task's research
- file: PRD.md
  why: "§9.24 FR-T4 (the N+1 turn protocol verbatim: turn-1 preamble 'I will send a git diff in N
        parts. After each part, reply with exactly: ok…'; turns 2..N 'PART i/N:'+chunk; turn N+1 'Now
        write the commit message for the diff above. Output ONLY the message.'; intermediate 'ok'
        discarded; final turn parsed by the EXISTING §9.6 pipeline). FR-T5 (per-turn timeout =
        cfg.Timeout). FR-T7 (failure → rescue: a turn's provider error / timeout / final parse-fail ⇒
        abort). FR-T2 (lossless — the SAME captured payload, chunked, not truncated). FR-T6 (session
        lifecycle: fresh session id per run; system prompt turn-1-only; re-invoking --session-id
        appends). FR-T10 (message role only)."
  critical: "FR-T4's verbatim strings ARE preambleFmt/finalInstruction (D7). FR-T7's 'any turn error
             aborts' IS the execErr != nil decision (D1). FR-T6's 'system prompt turn-1-only' and
             '--session-id on every turn' are enforced by RenderMultiTurn — Run just passes turn=1
             vs turn>1 and the sessionID."

- docfile: plan/009_5c53066d64b3/architecture/fr-t9-verification.md
  why: "The verified pi turn protocol (the live-run proof that session_mode='append' works): turn 1 has
        --system-prompt + --session-id + the isolation flags MINUS --no-session; turns 2..N+1 omit the
        system-prompt flag (the session carries it) but keep --session-id. Confirms RenderMultiTurn's
        flag contract is the verified one."
  critical: "This is the empirical basis for FR-T6's turn-1-only system-prompt semantics. Run relies on
             RenderMultiTurn to emit exactly this flag set; Run itself never touches flags."

- docfile: plan/009_5c53066d64b3/P1M1T3S2/research/multiturn_run_protocol.md
  why: "THIS subtask's research: §2 the verified seam contracts (Execute/RenderMultiTurn/ParseOutput);
        §3 the exact Run body; §4 the execErr != nil decision (D1); §5 newSessionID via crypto/rand (no
        uuid lib exists); §6 the focused smoke test + the stub seam + the SessionMode='append' test
        gotcha + the mid-turn-isolation gap (T4); §7 decisions D1–D8; §8 the import list. READ FIRST."
  critical: "§4 (D1 — execErr != nil, NOT errors.Is) and §6.2 (the SessionMode='append' test override)
             are the two things most likely to be gotten wrong. §5 (no uuid lib ⇒ crypto/rand) pins
             newSessionID. §6.4 (the mid-turn gap is T4's) sets the test scope."

- docfile: plan/009_5c53066d64b3/P1M1T3S1/PRP.md
  why: "The S1 CONTRACT (Implementing, in parallel): S1 creates multiturn.go with `type chunk struct
        {index, total int; text string}` + `func chunkPayload(payload string, chunkTokens int) []chunk`
        + advanceRunes + ceilDiv, AND multiturn_test.go with 7 chunk smoke tests. Run CONSUMES
        chunkPayload + chunks[i].text + N=len(chunks). S2 BUILDS ON S1's file (does not recreate it)."
  critical: "Treat S1 as LANDED. Run reads chunks[i].text (the 'PART i/N:\\n<body>' string) and
             len(chunks) for N. Do NOT modify chunkPayload/chunk/advanceRunes/ceilDiv (S1's deliverables).
             Preserve S1's choice for the internal/git import (anchor vs dropped — Run adds no git usage)."

- docfile: plan/009_5c53066d64b3/P1M1T1S3/PRP.md
  why: "The CONTRACT for RenderMultiTurn (LANDED): the turn-1-only system prompt, the session-flags
        block (BareFlags MINUS --no-session PLUS --session-id), the session_mode='append' gate, the
        stdin delivery. Confirms Run need only pass (model, sysPrompt|'', userPayload, reasoning,
        sessionID, turn) — RenderMultiTurn handles the flag mechanics."
  critical: "RenderMultiTurn's session_mode gate is Run's defense-in-depth (a non-append manifest ⇒ a
             render error ⇒ Run surfaces it as cause). Run does NOT re-check session_mode."

- docfile: plan/009_5c53066d64b3/P1M1T2S1/PRP.md
  why: "The config CONTRACT (LANDED): Config.MultiTurnChunkTokens int (default 32000) is the chunkPayload
        budget; Config.Timeout (default 120s) is the per-turn Execute timeout; Config.MultiTurnFallback
        bool (default true) is S3's gate (NOT Run's)."
  critical: "Run reads cfg.MultiTurnChunkTokens (→ chunkPayload) and cfg.Timeout (→ Execute). Run does
             NOT read cfg.MultiTurnFallback (S3's gate)."

# The edit site + the seams (READ-ONLY — internal/provider is consumed, not modified)
- file: internal/generate/multiturn.go
  why: "EDIT (file 1 of 2; S1 creates it, S2 extends). Add Run + newSessionID + preambleFmt/finalInstruction
        constants + the new imports. chunkPayload/chunk/advanceRunes/ceilDiv (S1) stay byte-identical."
  pattern: "package generate, sibling to generate.go/rescue.go/finalize.go/dedupe.go. Match the focused
            doc-comment style (cite PRD §9.24 FR-T4). Run is the protocol; newSessionID is a small private
            helper (crypto/rand)."
  gotcha: "S1 may have left the `var _ = git.EstimateTokens` anchor OR dropped the internal/git import.
           Run adds NO git usage, so PRESERVE S1's choice. Do NOT add errors to the import block (D1:
           execErr != nil suffices)."

- file: internal/provider/executor.go
  why: "READ-ONLY (the Execute seam). `func Execute(ctx, spec CmdSpec, timeout time.Duration, vb *ui.Verbose)
        (stdout, stderr string, err error)`. Shadows ctx with WithTimeout when timeout>0. Returns
        stdout/stderr EVEN ON ERROR. vb is nil-safe (verbose.go:40)."
  pattern: "Run calls `provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)` per turn. The returned
            stdout is discarded on turns 1..N; the final turn's stdout goes to ParseOutput."
  gotcha: "Execute's err covers timeout (DeadlineExceeded), cancel (Canceled), non-zero exit
           (*exec.ExitError), AND start failure. Run treats ALL of them as abort (execErr != nil, D1) —
           it does NOT fall through to ParseOutput on a non-zero exit (unlike one-shot at generate.go:242)."

- file: internal/provider/render.go
  why: "READ-ONLY. `RenderMultiTurn(model, sysPrompt, userPayload, reasoning, sessionID string, turn int)
        (*CmdSpec, error)` at :203. Returns *CmdSpec (Run dereferences `*spec`). Enforces session_mode='append'
        (else error) and turn-1-only system prompt. CmdSpec{Command, Args, Stdin, Env} at :22."
  gotcha: "RenderMultiTurn's session_mode gate means a non-append manifest ⇒ render error on turn 1 ⇒
           Run returns it as cause. Run does NOT pre-check session_mode (defense-in-depth is the render's job)."

- file: internal/provider/parse.go
  why: "READ-ONLY. `func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)` at :41.
        The EXISTING §9.6 pipeline. Run calls it ONCE on the final turn's stdout; returns (m, ok, nil)."
  gotcha: "Run discards fellback (a logging signal; Execute already logged the raw output via Verbose).
           Run does NOT fork ParseOutput or dedupe — the caller (S3) runs CommitStaged's existing dedupe."

- file: internal/generate/generate.go
  why: "READ-ONLY (the one-shot path Run mirrors + the Deps/RescueError contracts). Deps at :26 (Verbose
        *ui.Verbose, nil-safe). RescueError at :82 ({Kind, TreeSHA, ParentSHA, Candidate, Cause}). The
        one-shot Execute+Parse loop at :242–289 — Run's contrast: one-shot falls through to ParseOutput
        on a non-zero exit; Run does NOT (FR-T7 stricter)."
  pattern: "Run takes `deps Deps` for deps.Verbose (passed to Execute) and `manifest provider.Manifest`
            separately (the message-role manifest to Render+Execute+Parse). Deps{Verbose: nil} is safe."
  gotcha: "Run does NOT construct *RescueError (it returns the raw cause). S3 maps cause →
           &RescueError{Cause: cause}. Do NOT import generate's sentinel errors."

- file: internal/stubtest/stubtest.go
  why: "READ-ONLY (the test seam). stubtest.Build(t) compiles cmd/stubagent once. stubtest.NewScript(t,
        bin, responses) wires call-indexed stdout (the stub's selectScripted advances a counter per
        invocation ⇒ responses[0] on call 1, responses[1] on call 2, …). The stub manifest does NOT set
        SessionMode — the test MUST override it to &\"append\"."
  gotcha: "stubtest.NewScript returns provider.Manifest BY VALUE ⇒ `m.SessionMode = &appendMode` mutates
           the local copy (clean). The stub's STAGECOACH_STUB_EXIT is global (not per-call) ⇒ S2's
           turn-error test uses a GLOBAL failure (turn 1); mid-turn isolation is T4's (§6.4)."

# External references
- url: https://pkg.go.dev/crypto/rand#Read
  why: "rand.Read(b) fills b with cryptographically-secure random bytes (the session-id entropy source).
        Returns (n, err); n == len(b) always; err is practically never non-nil on Linux/macOS/Windows."
  critical: "Used in newSessionID. NO uuid library exists in the repo (go.mod has none) and no helper —
             crypto/rand+encoding/hex is the zero-dependency choice (D3)."
- url: https://pkg.go.dev/encoding/hex#EncodeToString
  why: "hex.EncodeToString(b) returns the lowercase hex string for the 16 random bytes (32 chars). Combined
        with the 'stagecoach-' prefix ⇒ the FR-T6 session id."
- url: https://git-scm.com/docs/git-config#Documentation/git-config.txt-corequotepath
  why: "(context for the stub agent's behavior — not directly used by Run.) Documents that git porcelain
        C-quotes non-ASCII paths; the stub agent is stdlib-only and unaffected."
```

### Current Codebase Tree (this task's scope — S1 creates the files; S2 extends)

```bash
stagecoach/
└── internal/generate/
    ├── multiturn.go       # EDIT (file 1): S1 created it (chunkPayload/chunk/advanceRunes/ceilDiv);
    │                      #   S2 ADDS Run + newSessionID + preambleFmt/finalInstruction + imports
    ├── multiturn_test.go  # EDIT (file 2): S1 created it (7 chunk smoke tests); S2 ADDS 4 Run smoke tests
    ├── generate.go        # READ-ONLY (S3 owns the trigger-gate insertion; Run's caller lives here)
    ├── rescue.go / finalize.go / dedupe.go  # READ-ONLY (siblings)
    └── ...
# internal/provider/{executor,render,parse}.go = READ-ONLY (the consumed seams)
# internal/stubtest/stubtest.go + cmd/stubagent = READ-ONLY (the test seam)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/generate/
    ├── multiturn.go       # + Run, + newSessionID, + preambleFmt/finalInstruction constants, + imports
    └── multiturn_test.go  # + TestRun_HappyPath, + TestRun_TurnError, + TestRun_FinalParseEmpty, + TestRun_NonAppendManifest
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/generate/multiturn.go` | MODIFY (extend S1's file) | Add `Run` (the N+1 turn protocol), `newSessionID` (crypto/rand session id), `preambleFmt`/`finalInstruction` constants, and the new imports (context, crypto/rand, encoding/hex, time, internal/config, internal/provider). S1's `chunkPayload`/`chunk`/`advanceRunes`/`ceilDiv` untouched. |
| `internal/generate/multiturn_test.go` | MODIFY (extend S1's file) | Add 4 focused Run smoke tests (happy / turn-error / final-parse-empty / non-append-manifest) using stubtest.NewScript + the SessionMode="append" override. S1's 7 chunk tests untouched. |

**Explicitly NOT touched**: `internal/generate/generate.go` (S3's trigger gate + Run's caller), `internal/generate/{rescue,finalize,dedupe,message}.go` and their tests, `internal/provider/*` (read-only seams), `internal/stubtest/*` + `cmd/stubagent` (read-only test seam), `internal/git/*` (read-only), `internal/config/*` (T2 — LANDED), `internal/ui/*` (read-only), any docs (S4/S5), `README.md` (S5), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — execErr != nil, NOT errors.Is(...)). FR-T7 treats a turn timeout, a turn non-zero-exit,
// AND a cancel ALL as abort conditions. The contract's pseudocode mentions errors.Is(...) but its own
// clarification ("treat non-zero exit as failure too") means ANY execErr != nil aborts. The simplified
// `if execErr != nil { return "", false, execErr }` is correct AND cleaner, and avoids importing errors.
// The CALLER (S3) discriminates timeout-vs-error via errors.Is(cause, context.DeadlineExceeded) when
// building the RescueError — that's S3's job, not Run's. (Research §4 / D1.)

// CRITICAL (G2 — Run does NOT fall through to ParseOutput on a non-zero exit, unlike one-shot). The
// one-shot path (generate.go:242) sets lastCause = execErr then parses partial stdout (the message
// might be partial-valid). Run is STRICTER: intermediate turns discard stdout anyway (it's just "ok"),
// and the final turn's non-zero exit means the message is unreliable. So ANY execErr ⇒ abort. Do NOT
// mirror one-shot's fall-through. (Research §4.)

// CRITICAL (G3 — Run returns the RAW cause; it does NOT construct *RescueError). The contract OUTPUT §4:
// "cause is non-nil only on abort — then CommitStaged maps it to &RescueError{Cause: cause}." Run returns
// the execErr/rerr verbatim. Do NOT import generate's ErrRescue/ErrTimeout/RescueError or wrap. (D2.)

// CRITICAL (G4 — NO uuid lib/helper exists; use crypto/rand). grep -rn 'uuid|crypto/rand' --include='*.go'
// internal/ cmd/ pkg/ ⇒ ZERO matches; go.mod has no uuid library. newSessionID uses crypto/rand.Read
// (16 bytes) + encoding/hex. The fallback (time.UnixNano) is defense-in-depth (rand.Read practically
// never fails). Do NOT add a uuid dependency. (Research §5 / D3.)

// CRITICAL (G5 — RenderMultiTurn enforces session_mode + turn-1-only system prompt; Run does NOT). The
// render gate (render.go:213 `if *r.SessionMode != "append" { return nil, error }`) rejects a non-append
// provider; the turn-1-only sysPrompt (render.go:228 `if turn != 1 { turnSys = "" }`) drops the flag on
// later turns. Run passes (sysPrompt on turn 1, "" on turns 2..N+1) and (turn=1, i, N+1) — RenderMultiTurn
// does the rest. Run surfaces a render error as cause (defense-in-depth for S3's gate). (D5.)

// GOTCHA (G6 — deps.Verbose is nil-safe; pass it straight to Execute). *ui.Verbose methods begin
// `if v == nil || v.w == nil || !v.on { return }` (verbose.go:40). So `deps := Deps{}` (Verbose nil) is
// safe in tests AND in production (the CLI passes a real *Verbose). Execute also emits the per-turn
// verbose surface (VerboseCommand/VerboseRawOutput/VerboseStderr/VerbosePayload) automatically — Run
// needs NO extra verbose calls (FR-T11's per-turn logging is handled by Execute). (Research §2.1.)

// GOTCHA (G7 — the stub manifest needs SessionMode=&\"append\" set IN THE TEST). stubtest.NewScript does
// NOT set SessionMode (defaults to "" via Resolve). RenderMultiTurn's gate would reject it. The test
// must override: `appendMode := "append"; m.SessionMode = &appendMode` (NewScript returns a VALUE ⇒ the
// assignment is a clean local copy). For the non-append test, leave SessionMode unset (⇒ ""). (Research §6.2.)

// GOTCHA (G8 — the stub's STAGECOACH_STUB_EXIT is GLOBAL, not per-call). The exit code is baked into the
// manifest's Env map ⇒ ALL turns get the same exit. So S2's TestRun_TurnError uses a GLOBAL failure
// (turn 1 fails). Mid-turn isolation (turn 1 ok, turn 2 fails) needs a per-call exit mechanism the stub
// lacks ⇒ that's T4's exhaustive-matrix territory. Do NOT try to hack per-call exit in S2. (Research §6.4.)

// GOTCHA (G9 — chunkTokens=1 in the test forces a deterministic multi-chunk N). config.Defaults() has
// MultiTurnChunkTokens=32000 (a small payload ⇒ N=1 ⇒ 2 turns). To exercise the turns-2..N loop, set
// cfg.MultiTurnChunkTokens=1 (⇒ runesPerWindow=4) with payload "aaaa\nbbbb\n" ⇒ N=2 ⇒ 3 turns. The script
// is then ["ok", "ok", "<final message>"]. (Research §6.1.)

// GOTCHA (G10 — the smoke test is the S1 precedent; T4 extends). S1 shipped 7 focused chunk smoke tests;
// S4 extends with the exhaustive matrix. S2 ships 4 focused Run smoke tests (happy/turn-error/final-parse
// -empty/non-append); T4 (P1.M1.T4) extends with the integration matrix (N+1 turns end-to-end, --session-id
// present, --no-session dropped, commit lands, mid-turn failure → rescue, small-payload skip, non-append
// skip). Do NOT write the exhaustive matrix here. Use plain if/t.Errorf (no testify — matches generate/*_test.go).

// GOTCHA (G11 — preserve S1's internal/git import choice). S1's PRP offers two options for multiturn.go's
// internal/git import: keep `var _ = git.EstimateTokens` anchor OR drop the import + cite /4 in a comment.
// Run adds NO git usage. PRESERVE whatever S1 decided — do not add/remove the internal/git import. (Research §8.)

// GOTCHA (G12 — preambleFmt/finalInstruction are verbatim FR-T4; do NOT paraphrase). The preamble MUST be
// exactly "I will send a git diff in %d parts. After each part, reply with exactly: ok. Do not analyze or
// write any commit message until I explicitly ask at the end." (with %d for N). The final instruction MUST
// be exactly "Now write the commit message for the diff above. Output ONLY the message." These are the
// model-facing prompts FR-T4 specifies verbatim — paraphrasing risks degrading the protocol. (D7.)

// GOTCHA (G13 — N=1 is valid: 2 turns). If the payload fits one chunk (N=1), the turns-2..N loop body
// never executes (range 2..1 is empty), and Run does turn 1 (priming + chunk 1) + turn 2 (final). In
// practice S3's gate ensures N≥2 (payload > one chunk), but Run is correct for N=1 too. (Research §3.)
```

## Implementation Blueprint

### Data models and structure

None added beyond S1's `chunk` type. `Run`'s signature types (`Deps`, `config.Config`,
`provider.Manifest`) are all already declared in their packages. The two new package-level constants
are strings (verbatim FR-T4).

### The `Run` body + `newSessionID` + constants (exact — copy verbatim into multiturn.go)

Place these AFTER S1's `chunkPayload`/`advanceRunes`/`ceilDiv` (keep S1's code byte-identical). Add the
imports listed in §"Imports" to S1's import block.

```go
// preambleFmt is the turn-1 priming preamble (PRD §9.24 FR-T4, VERBATIM with N interpolated). The model
// is told to expect N parts and to reply "ok" to each, deferring the commit message until the final turn.
const preambleFmt = "I will send a git diff in %d parts. After each part, reply with exactly: ok. Do not analyze or write any commit message until I explicitly ask at the end."

// finalInstruction is the turn-N+1 request (PRD §9.24 FR-T4, VERBATIM). The candidate commit message is
// THIS turn's stdout (parsed by the existing §9.6 pipeline via ParseOutput).
const finalInstruction = "Now write the commit message for the diff above. Output ONLY the message."

// Run drives the N+1-turn multi-turn generation protocol (PRD §9.24 FR-T4/FR-T5/FR-T7). It is the
// transport layer invoked by the FR-T1 trigger gate (P1.M1.T3.S3, the sole caller) AFTER the one-shot
// path exhausted its retries on a payload that exceeds one chunk. Lossless (FR-T2): the SAME captured
// payload is chunked (chunkPayload) and delivered across N+1 sequential provider invocations against ONE
// session id — the model sees the entire diff in its session history, then writes one message at the end.
//
// Protocol (FR-T4):
//   - Turn 1:   system prompt (via the manifest's system_prompt_flag; turn-1-only) + preamble + chunk 1.
//   - Turns 2..N: each chunk's body ("PART i/N:\n<body>"); no system-prompt flag (turn > 1).
//   - Turn N+1:  the finalInstruction; THIS turn's stdout is the candidate message.
//
// Per-turn timeout = cfg.Timeout (FR-T5; Execute shadows ctx with WithTimeout). Intermediate turns'
// stdout ("ok") is discarded. Failure handling (FR-T7): ANY turn's Execute error OR any RenderMultiTurn
// error ⇒ Run aborts and returns the raw error as `cause` (the caller maps it to &RescueError{Cause:
// cause}). The final turn's parse outcome is (msg, ok): ok==false (empty/unparseable final stdout) is
// NOT a cause — the caller decides rescue per FR-T7's "final turn's output failing to parse".
//
// Run does NOT fork ParseOutput or run dedupe — the caller (CommitStaged, via S3) runs the existing
// dedupe path on the returned msg. Run does NOT check cfg.MultiTurnFallback or session_mode — S3 owns
// the trigger gate; RenderMultiTurn's own session_mode="append" gate is the defense-in-depth (a non-append
// manifest ⇒ a turn-1 render error ⇒ surfaced as cause).
func Run(ctx context.Context, deps Deps, cfg config.Config, manifest provider.Manifest,
	sysPrompt, payload, msgModel, msgReasoning string) (msg string, ok bool, cause error) {

	// (1) Chunk the captured payload (FR-T2 lossless; FR-T3 sizing — S1's helper).
	chunks := chunkPayload(payload, cfg.MultiTurnChunkTokens)
	N := len(chunks)

	// (2) Mint a fresh, one-run-scope session id (FR-T6 — never resumed on a later run).
	sessionID := newSessionID()

	// (3) Priming preamble (FR-T4, verbatim with N interpolated).
	preamble := fmt.Sprintf(preambleFmt, N)

	// (4) Turn 1: system prompt + preamble + chunk 1. RenderMultiTurn emits the system_prompt_flag only
	//     on turn 1 (its own turn-1-only gate). chunks[0].text already carries "PART 1/N:\n<body>".
	spec, rerr := manifest.RenderMultiTurn(msgModel, sysPrompt, preamble+"\n\n"+chunks[0].text,
		msgReasoning, sessionID, 1)
	if rerr != nil {
		return "", false, rerr // non-append provider ⇒ RenderMultiTurn's session_mode gate; surface as cause
	}
	if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {
		return "", false, execErr // FR-T7: any turn error/timeout/cancel/non-zero-exit aborts
	}

	// (6) Turns 2..N: each chunk's body; sysPrompt="" ⇒ RenderMultiTurn drops the system_prompt_flag.
	for i := 2; i <= N; i++ {
		spec, rerr := manifest.RenderMultiTurn(msgModel, "", chunks[i-1].text,
			msgReasoning, sessionID, i)
		if rerr != nil {
			return "", false, rerr
		}
		if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {
			return "", false, execErr // FR-T7
		}
	}

	// (7) Turn N+1 (final): the commit-message request. The candidate message is THIS turn's stdout.
	spec, rerr = manifest.RenderMultiTurn(msgModel, "", finalInstruction,
		msgReasoning, sessionID, N+1)
	if rerr != nil {
		return "", false, rerr
	}
	out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
	if execErr != nil {
		return "", false, execErr // (8) FR-T7: final-turn error aborts
	}

	// (9) Parse the final turn's stdout via the EXISTING §9.6 pipeline (FR-T4). Return (msg, ok, nil) —
	//     the caller runs dedupe. ok==false (empty/unparseable) is NOT a cause (caller decides rescue).
	m, parseOK, _ := provider.ParseOutput(out, manifest)
	return m, parseOK, nil
}

// newSessionID mints a fresh, one-run-scope session id for a multi-turn run (PRD §9.24 FR-T6). Format:
// "stagecoach-<32 hex>" (16 cryptographically-random bytes). The id is NEVER resumed on a later run —
// providers that persist sessions leave it behind (harmless). Uses crypto/rand (no uuid library exists in
// the repo and none is added); the time-based fallback is defense-in-depth (rand.Read practically never
// fails on Linux/macOS/Windows).
func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("stagecoach-%d", time.Now().UnixNano())
	}
	return "stagecoach-" + hex.EncodeToString(b[:])
}
```

### Imports (add to S1's import block)

S1's multiturn.go imports `fmt`, `strings`, `unicode/utf8`, and (optionally) `internal/git`. S2 ADDS:

```go
"context"
"crypto/rand"
"encoding/hex"
"time"

"github.com/dustin/stagecoach/internal/config"
"github.com/dustin/stagecoach/internal/provider"
```

S2 does NOT add `errors` (D1: `execErr != nil` suffices). PRESERVE S1's `internal/git` choice (G11).

> **Note on `internal/provider` + `internal/config` + `package generate`:** multiturn.go is `package
> generate`; it imports config + provider (different packages). `Deps` is same-package (no import). This
// matches generate.go (which imports both).

### The 4 focused Run smoke tests (exact — add to multiturn_test.go)

```go
// stubAppendManifest returns a stub provider.Manifest wired to call-indexed scripted stdout (the stub's
// selectScripted advances a per-invocation counter), WITH SessionMode set to "append" (RenderMultiTurn's
// gate requires it; stubtest.NewScript does not set it). omitAppend=true leaves SessionMode unset (⇒ "")
// to exercise the non-append render-error path.
func stubAppendManifest(t *testing.T, bin string, responses []string, omitAppend bool) provider.Manifest {
	t.Helper()
	m := stubtest.NewScript(t, bin, responses)
	if !omitAppend {
		appendMode := "append"
		m.SessionMode = &appendMode
	}
	return m
}

// TestRun_HappyPath: a 2-chunk payload (chunkTokens=1, payload "aaaa\nbbbb\n" ⇒ N=2) drives 3 turns
// ("ok", "ok", "<message>"). Run returns the final turn's parsed message with cause==nil, ok==true.
func TestRun_HappyPath(t *testing.T) {
	bin := stubtest.Build(t)
	m := stubAppendManifest(t, bin, []string{"ok", "ok", "feat: add multi-turn support"}, false)
	cfg := config.Defaults()
	cfg.MultiTurnChunkTokens = 1 // ⇒ runesPerWindow=4 ⇒ "aaaa\nbbbb\n" splits into 2 chunks ⇒ 3 turns

	msg, ok, cause := Run(context.Background(), Deps{}, cfg, m, "you are a commit writer",
		"aaaa\nbbbb\n", "zai/glm-5.2", "")
	if cause != nil {
		t.Fatalf("Run cause = %v, want nil (happy path)", cause)
	}
	if !ok {
		t.Fatalf("Run ok = false, want true (final turn parsed)")
	}
	if msg != "feat: add multi-turn support" {
		t.Errorf("Run msg = %q, want %q", msg, "feat: add multi-turn support")
	}
}

// TestRun_TurnError: a global stub exit-1 (Options{Exit:1}) ⇒ turn 1's Execute returns a non-zero-exit
// error ⇒ Run aborts with cause != nil (FR-T7). (The stub's exit code is global; mid-turn isolation is
// T4's exhaustive-matrix territory — see research §6.4.)
func TestRun_TurnError(t *testing.T) {
	bin := stubtest.Build(t)
	// NewScript + Exit: build via Manifest directly so Exit is baked into Env.
	sm := stubtest.Manifest(bin, stubtest.Options{Exit: 1})
	script := []string{"ok", "ok", "feat: never reached"}
	// Re-use the call-indexed mechanism by pointing Script at a file (NewScript's approach, inline):
	dir := t.TempDir()
	scriptFile := dir + "/script.txt"
	os.WriteFile(scriptFile, []byte(strings.Join(script, "\n")), 0o644)
	sm.Env = append(sm.Env, "STAGECOACH_STUB_SCRIPT="+scriptFile, "STAGECOACH_STUB_COUNTER="+dir+"/counter")
	appendMode := "append"
	sm.SessionMode = &appendMode

	cfg := config.Defaults()
	cfg.MultiTurnChunkTokens = 1
	_, _, cause := Run(context.Background(), Deps{}, cfg, sm, "sys", "aaaa\nbbbb\n", "zai/glm-5.2", "")
	if cause == nil {
		t.Fatal("Run cause = nil, want non-nil (turn-1 non-zero exit ⇒ FR-T7 abort)")
	}
}

// TestRun_FinalParseEmpty: the final turn's stdout is empty (script ends with "") ⇒ ParseOutput ok=false.
// Run returns (msg="", ok=false, cause==nil) — the parse failure is NOT a cause; the caller decides rescue.
func TestRun_FinalParseEmpty(t *testing.T) {
	bin := stubtest.Build(t)
	m := stubAppendManifest(t, bin, []string{"ok", "ok", ""}, false) // final turn → empty stdout
	cfg := config.Defaults()
	cfg.MultiTurnChunkTokens = 1

	msg, ok, cause := Run(context.Background(), Deps{}, cfg, m, "sys", "aaaa\nbbbb\n", "zai/glm-5.2", "")
	if cause != nil {
		t.Fatalf("Run cause = %v, want nil (empty final stdout is a parse-fail, not a turn error)", cause)
	}
	if ok {
		t.Error("Run ok = true, want false (empty final stdout ⇒ ParseOutput ok=false)")
	}
	if msg != "" {
		t.Errorf("Run msg = %q, want \"\" (empty final stdout)", msg)
	}
}

// TestRun_NonAppendManifest: a manifest with SessionMode unset (⇒ "") ⇒ RenderMultiTurn's session_mode
// gate errors on turn 1 ⇒ Run surfaces the render error as cause (defense-in-depth for S3's FR-T1 gate).
func TestRun_NonAppendManifest(t *testing.T) {
	bin := stubtest.Build(t)
	// omitAppend=true ⇒ SessionMode stays "" ⇒ RenderMultiTurn errors.
	m := stubAppendManifest(t, bin, []string{"ok", "ok", "feat: never reached"}, true)
	cfg := config.Defaults()
	cfg.MultiTurnChunkTokens = 1

	_, _, cause := Run(context.Background(), Deps{}, cfg, m, "sys", "aaaa\nbbbb\n", "zai/glm-5.2", "")
	if cause == nil {
		t.Fatal("Run cause = nil, want non-nil (non-append manifest ⇒ RenderMultiTurn session_mode gate)")
	}
	if !strings.Contains(cause.Error(), "session_mode") {
		t.Errorf("Run cause = %v, want it to mention session_mode (the render gate)", cause)
	}
}
```

> The test file's imports: S1 imports `strings`, `testing`, `internal/git`. S2 ADDS `context`, `os`,
> `github.com/dustin/stagecoach/internal/config`, `github.com/dustin/stagecoach/internal/provider`,
> `github.com/dustin/stagecoach/internal/stubtest`. (S1's stripPartPrefix helper + 7 chunk tests stay.)

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/generate/multiturn.go (add Run + newSessionID + constants + imports)
  - OPEN internal/generate/multiturn.go (S1 created it; if S1 has not landed yet, WAIT — this task
    extends S1's file. The edit anchors are S1's chunkPayload/advanceRunes/ceilDiv, which stay intact.)
  - ADD to the import block: context, crypto/rand, encoding/hex, time, internal/config, internal/provider.
    PRESERVE S1's internal/git choice (G11). Do NOT add errors (D1).
  - ADD the two constants (preambleFmt, finalInstruction) verbatim from §"The Run body".
  - ADD func Run verbatim from §"The Run body" (place AFTER S1's ceilDiv).
  - ADD func newSessionID verbatim.
  - DO NOT: modify chunkPayload/chunk/advanceRunes/ceilDiv (S1); add a trigger gate (S3); construct
    *RescueError (S3's job); check cfg.MultiTurnFallback or session_mode (S3/render's job); add docs (S4).
  - VERIFY: go build ./internal/generate/ → exit 0 (imports resolve; no unused-import error).

Task 2: MODIFY internal/generate/multiturn_test.go (add the 4 Run smoke tests)
  - OPEN internal/generate/multiturn_test.go (S1 created it with 7 chunk tests + stripPartPrefix).
  - ADD imports: context, os, internal/config, internal/provider, internal/stubtest.
  - ADD the stubAppendManifest helper + the 4 TestRun_* functions verbatim from §"The 4 focused tests".
  - DO NOT: modify S1's 7 chunk tests or stripPartPrefix; write the exhaustive matrix (T4); use testify.
  - VERIFY: go test -race ./internal/generate/ -run TestRun → all 4 PASS.

Task 3: VALIDATE — full gate set + scope discipline
  - RUN: gofmt -w internal/generate/multiturn.go internal/generate/multiturn_test.go ; gofmt -l .
  - RUN: go vet ./...
  - RUN: go build ./...
  - RUN: go test -race ./internal/generate/ -v -run 'TestChunkPayload|TestRun'
        → expect S1's 7 chunk tests + S2's 4 Run tests ALL PASS.
  - RUN: go test -race ./...                # whole repo green (additive change to 2 files)
  - RUN (scope): git status --porcelain → expect EXACTLY:
        M internal/generate/multiturn.go
        M internal/generate/multiturn_test.go
  - RUN (no-gate grep): grep -nE 'MultiTurnFallback|EstimateTokens\(payload\)' internal/generate/multiturn.go
        # EXPECT: no matches (the FR-T1 trigger gate is S3's; Run does not read MultiTurnFallback).
  - RUN (no-RescueError grep): grep -nE 'RescueError|ErrRescue|ErrTimeout' internal/generate/multiturn.go
        # EXPECT: no matches (Run returns the raw cause; S3 wraps it).
```

### Implementation Patterns & Key Details

```go
// === Why execErr != nil (not errors.Is) — D1/G1 ===
// FR-T7 lists timeout, non-zero-exit, and (for the final turn) parse-fail as abort conditions. The first
// two are BOTH "Execute returned err != nil". The parse-fail is handled separately (ok==false, cause==nil).
// So for EVERY turn: `if execErr != nil { return "", false, execErr }`. No errors.Is needed IN RUN — the
// caller (S3) discriminates timeout-vs-error when building the RescueError. This is stricter than one-shot
// (which falls through to ParseOutput on a non-zero exit) because intermediate turns discard stdout.

// === Why Run does NOT construct *RescueError — D2/G3 ===
// Run is a library function returning (msg, ok, cause). The RescueError needs TreeSHA/ParentSHA/Candidate
// (snapshot context) that Run doesn't have — the caller (CommitStaged via S3) does. Run returns the raw
// cause; S3 does: `if cause != nil { return &RescueError{Kind: ErrRescue, ..., Cause: cause} }`.

// === Why newSessionID uses crypto/rand (no uuid lib) — D3/G4 ===
// grep confirms ZERO uuid usage in the repo and NO uuid lib in go.mod. crypto/rand.Read(16 bytes) +
// hex.EncodeToString ⇒ "stagecoach-<32hex>". Zero new dependencies. The time fallback handles the
// practically-never case of rand.Read failing.

// === Why Run passes sysPrompt only on turn 1 — G5 ===
// RenderMultiTurn's `if turn != 1 { turnSys = "" }` (render.go) drops the system_prompt_flag on turns
// 2..N+1. The session carries the system prompt (FR-T6, verified in fr-t9-verification.md). Run passes
// sysPrompt on turn 1 and "" on turns 2..N+1; the render does the rest.

// === Why the test sets SessionMode=&"append" — G7 ===
// stubtest.NewScript returns a Manifest with SessionMode unset (Resolve ⇒ ""). RenderMultiTurn's gate
// (render.go:213) would reject it. The test overrides the field on the VALUE copy: `m.SessionMode =
// &appendMode`. For the non-append test, leave it unset (⇒ the render-error path).

// === Why the happy-path test uses chunkTokens=1 — G9 ===
// config.Defaults() has MultiTurnChunkTokens=32000 ⇒ a small payload yields N=1 (2 turns). To exercise
// the turns-2..N loop, set cfg.MultiTurnChunkTokens=1 (runesPerWindow=4); "aaaa\nbbbb\n" (10 runes,
// 2 lines) ⇒ N=2 ⇒ 3 turns. The script is ["ok", "ok", "<message>"].

// === Why TestRun_TurnError uses a global exit — G8 ===
// The stub's STAGECOACH_STUB_EXIT is one env var baked into the manifest's Env map ⇒ all turns share it.
// So the test asserts turn-1 failure (the first Execute sees Exit:1). Mid-turn isolation (turn 1 ok,
// turn 2 fails) needs a per-call exit mechanism the stub lacks ⇒ T4's territory.
```

### Integration Points

```yaml
PRODUCTION (internal/generate/multiturn.go — MODIFIED, extends S1's file):
  - + const preambleFmt, const finalInstruction (verbatim FR-T4)
  - + func Run(ctx, deps, cfg, manifest, sysPrompt, payload, msgModel, msgReasoning) (msg, ok, cause)
  - + func newSessionID() string (crypto/rand)
  - + imports: context, crypto/rand, encoding/hex, time, internal/config, internal/provider

TESTS (internal/generate/multiturn_test.go — MODIFIED, extends S1's file):
  - + stubAppendManifest helper + 4 TestRun_* smoke tests (happy/turn-error/final-parse-empty/non-append)

CONSUMED (READ-ONLY — the seams):
  - internal/provider/executor.go: Execute(ctx, CmdSpec, timeout, *ui.Verbose) (stdout, stderr, err)
  - internal/provider/render.go: RenderMultiTurn(model, sysPrompt, userPayload, reasoning, sessionID, turn) (*CmdSpec, error)
  - internal/provider/parse.go: ParseOutput(raw, Manifest) (msg, ok, fellback)
  - internal/generate/generate.go: Deps (Verbose *ui.Verbose, nil-safe); RescueError (the CALLER's wrapper, not Run's)
  - internal/stubtest/stubtest.go + cmd/stubagent: the test seam (call-indexed scripted stdout)

CALLER (downstream — owned by S3, NOT this task):
  - P1.M1.T3.S3 (FR-T1 trigger gate): the gate that decides WHEN to call Run, and the wrapper that maps
    Run's (msg, ok, cause) → either CommitStaged's existing dedupe path (cause==nil) or
    &RescueError{Cause: cause} (cause != nil).

CONFIG (consumed — T2 LANDED):
  - cfg.MultiTurnChunkTokens (→ chunkPayload budget) ; cfg.Timeout (→ per-turn Execute timeout)
  - (cfg.MultiTurnFallback is S3's gate, NOT Run's)

GATE: go test -race ./internal/generate/ → GREEN ; git status → ONLY the 2 modified files

NO-TOUCH (explicitly — owned by siblings):
  - internal/generate/generate.go (S3 trigger gate + Run's caller), rescue.go/finalize.go/dedupe.go/message.go
  - internal/provider/* (read-only seams), internal/stubtest/* + cmd/stubagent (read-only test seam)
  - internal/git/* (read-only), internal/config/* (T2 — LANDED), internal/ui/* (read-only)
  - docs/* (S4/S5), README.md (S5), PRD.md, tasks.json, prd_snapshot.md, plan/*
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/generate/multiturn.go internal/generate/multiturn_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/generate/...   # Expected: exit 0
go build ./...                   # Expected: exit 0

# Expected: zero errors. If `imported and not used` in multiturn.go: every added import must be used
# (context in Run's signature; crypto/rand+encoding/hex+time in newSessionID; config/provider in Run's
# signature/body). If `declared and not used` for S1's `var _ = git.EstimateTokens` anchor, S1 chose the
# anchor option and Run added no git usage — leave the anchor (it's the dependency anchor; G11).
```

### Level 2: The Run Smoke Tests (the deliverable)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./internal/generate/ -v -run 'TestChunkPayload|TestRun'
# Expected: S1's 7 chunk tests + S2's 4 Run tests ALL PASS, exit 0:
#   TestRun_HappyPath         — 2 chunks ⇒ 3 turns; returns "feat: add multi-turn support"; cause==nil; ok==true
#   TestRun_TurnError         — global stub exit-1 ⇒ turn-1 Execute error ⇒ cause != nil (FR-T7)
#   TestRun_FinalParseEmpty   — final stdout "" ⇒ ParseOutput ok=false; cause==nil (parse-fail, not a turn error)
#   TestRun_NonAppendManifest — SessionMode unset ⇒ RenderMultiTurn session_mode gate ⇒ cause mentions session_mode
```

### Level 3: Whole-Repository Regression + Scope Discipline

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...    # Expected: ALL packages green (only 2 files changed, both additive/extending)
go vet ./...           # Expected: exit 0

# Scope: ONLY the two modified multiturn files.
git status --porcelain
# Expected EXACTLY:
#   M internal/generate/multiturn.go
#   M internal/generate/multiturn_test.go

# No trigger-gate / no RescueError construction in Run:
grep -nE 'MultiTurnFallback|RescueError|ErrRescue|ErrTimeout' internal/generate/multiturn.go || true
# Expected: no matches (the gate is S3's; RescueError wrapping is the caller's).

# S1's chunk helpers are byte-identical (this task only ADDED Run + newSessionID + constants):
grep -nE 'func chunkPayload|func advanceRunes|func ceilDiv|type chunk' internal/generate/multiturn.go
# Expected: all four present (S1's deliverables, untouched).

# Sibling/production territory UNTOUCHED:
git diff --stat -- internal/generate/generate.go internal/provider/ internal/stubtest/ internal/git/ \
                   internal/config/ internal/ui/ docs/ README.md
# Expected: EMPTY.
```

### Level 4: Behavioral Cross-Check (the stub-agent end-to-end proof)

```bash
cd /home/dustin/projects/stagecoach

# The 4 Run tests ARE the behavioral proof (they drive Run → RenderMultiTurn → Execute → the real stub
# binary → ParseOutput, end-to-end against cmd/stubagent with call-indexed scripted stdout). For a manual
# cross-check of the session-id freshness (FR-T6), confirm two Runs produce DIFFERENT session ids:
# (Run does not expose the session id, but the stub's STAGECOACH_STUB_ARGSFILE captures the rendered argv,
#  which includes --session-id <id>. A test could assert two Runs' argsfiles differ — that's T4's
#  integration territory. S2's smoke tests prove the protocol mechanics; T4 proves the end-to-end wiring.)
echo "S2 smoke tests (Level 2) are the behavioral gate. T4 (P1.M1.T4) owns the end-to-end integration matrix."
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.
- [ ] `go test -race ./internal/generate/ -v -run 'TestChunkPayload|TestRun'` — S1's 7 + S2's 4 ALL PASS.

### Feature Validation
- [ ] `Run` signature matches the contract: `(ctx, deps, cfg, manifest, sysPrompt, payload, msgModel,
      msgReasoning) (msg, ok, cause)`.
- [ ] Turn 1 = `RenderMultiTurn(msgModel, sysPrompt, preamble+"\n\n"+chunks[0].text, …, sessionID, 1)`.
- [ ] Turns 2..N = `RenderMultiTurn(msgModel, "", chunks[i-1].text, …, sessionID, i)`.
- [ ] Turn N+1 = `RenderMultiTurn(msgModel, "", finalInstruction, …, sessionID, N+1)`.
- [ ] ANY Execute error or render error ⇒ `return "", false, <err>` (FR-T7; execErr != nil, D1).
- [ ] Final turn parsed via `provider.ParseOutput`; Run returns `(m, parseOK, nil)` (no forked dedupe).
- [ ] `newSessionID()` returns `"stagecoach-"+hex(16 random bytes)`; never panics.
- [ ] Run does NOT construct `*RescueError`, check `cfg.MultiTurnFallback`, or check `session_mode`.
- [ ] `preambleFmt`/`finalInstruction` are verbatim FR-T4 (not paraphrased).

### Scope Discipline Validation
- [ ] `git diff --stat` shows ONLY `internal/generate/multiturn.go` + `internal/generate/multiturn_test.go`.
- [ ] S1's `chunkPayload`/`chunk`/`advanceRunes`/`ceilDiv` + 7 chunk tests byte-identical (this task only ADDED).
- [ ] Did NOT edit `internal/generate/generate.go` (S3), any sibling file, `internal/provider/*`, `internal/stubtest/*`,
      `internal/git/*`, `internal/config/*`, `internal/ui/*`, `cmd/stubagent`, docs, `PRD.md`, `tasks.json`,
      `prd_snapshot.md`, or `plan/*`.
- [ ] NO trigger gate (S3), exhaustive Run matrix (T4), how-it-works doc (S4), or integration tests (T4).

### Code Quality Validation
- [ ] `Run` + `newSessionID` have focused doc comments citing PRD §9.24 FR-T4/FR-T5/FR-T7/FR-T6.
- [ ] The `execErr != nil` decision (D1) is documented in the code (why not errors.Is; why not fall-through).
- [ ] Smoke tests mirror `generate/*_test.go` style (plain if/t.Errorf; no testify; white-box package).
- [ ] The SessionMode="append" test override is documented (the stub doesn't set it; G7).

---

## Anti-Patterns to Avoid

- ❌ Don't use `errors.Is(execErr, ...)` to discriminate timeout-vs-exit IN RUN. FR-T7 treats ALL Execute
  errors as abort. `if execErr != nil { return "", false, execErr }` is correct and cleaner. The CALLER
  (S3) discriminates when building the RescueError (D1/G1).
- ❌ Don't fall through to ParseOutput on a non-zero exit (the one-shot pattern at generate.go:242).
  Multi-turn is STRICTER (FR-T7): intermediate turns discard stdout; a failed turn compromises the
  session. Any execErr ⇒ abort (G2).
- ❌ Don't construct `*RescueError` inside Run. Run returns the raw `cause`; the caller (S3) wraps it
  with the snapshot context (TreeSHA/ParentSHA/Candidate) Run doesn't have (D2/G3).
- ❌ Don't add a uuid library or reach for a non-existent uuid helper. `grep` confirms none exists.
  Use `crypto/rand` + `encoding/hex` (zero new deps) (D3/G4).
- ❌ Don't check `cfg.MultiTurnFallback` or `session_mode` inside Run. S3 owns the FR-T1 trigger gate;
  `RenderMultiTurn`'s own session_mode gate is the defense-in-depth (surfaced as `cause`). Run is the
  transport, not the gate (D5/G5).
- ❌ Don't fork ParseOutput or run dedupe. Return `(msg, ok, nil)`; CommitStaged's existing dedupe path
  (via S3) handles the returned msg (D6).
- ❌ Don't paraphrase the FR-T4 prompts. `preambleFmt` and `finalInstruction` are verbatim — the model-
  facing protocol strings. Paraphrasing risks degrading the multi-turn reliability (G12/D7).
- ❌ Don't pass the system prompt on turns 2..N+1. RenderMultiTurn drops the flag when `turn != 1`, but
  only if Run passes `sysPrompt=""` on those turns. The session carries the system prompt (FR-T6) (G5).
- ❌ Don't forget the SessionMode="append" override in the stub-based tests. `stubtest.NewScript` does
  NOT set it; RenderMultiTurn's gate would reject the manifest. Override the field on the value copy (G7).
- ❌ Don't try to isolate mid-turn failure (turn 1 ok, turn 2 fails) with the current stub. Its exit code
  is global. S2's TestRun_TurnError uses a global failure; mid-turn isolation is T4's (G8).
- ❌ Don't write the exhaustive Run matrix, the trigger gate, the how-it-works doc, or the integration
  tests. This task is Run + newSessionID + a focused 4-test smoke (S3 = gate, S4 = matrix+doc, T4 =
  integration) (G10).
- ❌ Don't modify S1's `chunkPayload`/`chunk`/`advanceRunes`/`ceilDiv` or S1's 7 chunk tests. This task
  EXTENDS multiturn.go/multiturn_test.go; S1's deliverables are byte-identical.
- ❌ Don't drop or add the `internal/git` import based on your own preference. PRESERVE S1's choice (the
  `var _ = git.EstimateTokens` anchor OR the dropped-import+comment). Run adds no git usage (G11).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a focused, single-function addition (Run + newSessionID + 2 constants) to a file S1
creates, with a verbatim copy-paste-ready implementation, verified seam contracts (Execute/RenderMultiTurn/
ParseOutput read directly from source), and a focused 4-test smoke using the proven stub seam. Five
independent de-riskings: (1) `RenderMultiTurn` (LANDED) enforces the session_mode gate AND the turn-1-only
system prompt — Run need only pass the params, so the flag mechanics are not Run's concern; (2) `Execute`
(LANDED) is nil-safe on `*ui.Verbose` and returns stdout/stderr even on error — Run passes `deps.Verbose`
straight through and treats any err as abort; (3) the stub's call-indexed `selectScripted` is EXACTLY the
N+1-turn pattern ("ok"…"ok"…<message>), so the happy-path test is deterministic; (4) the `execErr != nil`
decision eliminates the errors.Is complexity AND the one-shot fall-through trap in one stroke; (5) the
S1 contract (`chunkPayload`/`chunk`/`N=len(chunks)`/`chunks[i].text`) is the verified sizing seam Run
consumes. The CRITICAL gotchas front-loaded — (G1) execErr != nil, (G2) no fall-through, (G3) raw cause
not RescueError, (G4) crypto/rand, (G7) SessionMode override — are the things an implementer would
otherwise get wrong. The residual uncertainty (not 10/10) is the S1 coordination (S1 is Implementing in
parallel; if S1's anchor/import choice differs from the implementer's assumption, a one-line import
adjustment is needed — mitigated by G11) and the TestRun_TurnError's manual Env wiring (it doesn't use
stubtest.NewScript's clean path because Exit+Script must be combined inline — slightly fiddly, mitigated
by the verbatim test body and the alternative of using Options{Exit:1} alone if the script wiring proves
problematic). No production-code risk outside multiturn.go; no parallel-edit risk (only multiturn.go +
multiturn_test.go, which S1 owns and this task extends — S1 is the sole other writer, and it has landed
its version before S2 begins).
