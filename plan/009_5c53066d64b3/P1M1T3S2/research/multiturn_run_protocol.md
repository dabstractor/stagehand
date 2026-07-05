# multiturn.go N+1 turn protocol (Run) — P1.M1.T3.S2 Research

> Verified against the live repo (HEAD = a10b4eb, module `github.com/dustin/stagehand`). All
> signatures/line numbers confirmed by direct read. No files modified — research only.

## 1. What this task is (the contract, restated)

Add `func Run` to `internal/generate/multiturn.go` (the file S1 creates) — the N+1 turn protocol for
the multi-turn fallback (PRD §9.24 FR-T4/FR-T5/FR-T7/FR-T10). S1 lands `chunkPayload` + the `chunk`
type; THIS task lands the protocol that consumes them. Signature (FIXED by the contract):

```go
func Run(ctx context.Context, deps Deps, cfg config.Config, manifest provider.Manifest,
    sysPrompt, payload, msgModel, msgReasoning string) (msg string, ok bool, cause error)
```

Returns `(msg, ok, cause)`: `cause != nil` ⟺ a turn aborted (Execute/render error or timeout —
FR-T7); `cause == nil` ⟹ the final turn completed and `(msg, ok)` is its ParseOutput result (ok may
be false on parse-fail — the CALLER, not Run, decides rescue). Run does NOT fork ParseOutput or
dedupe — CommitStaged's existing path (P1.M1.T3.S3) handles dedupe on the returned msg.

## 2. The seam contracts (verified signatures + behaviors)

### 2.1 `provider.Execute` (executor.go, the Execute func — confirmed)

```go
func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout string, stderr string, err error)
```

- SHADOWS ctx with `context.WithTimeout(ctx, timeout)` when `timeout > 0` (load-bearing — the later
  `ctx.Err()` reads the timeout ctx). `cfg.Timeout` (default 120s) is the per-turn timeout (FR-T5).
- Returns `(stdout, stderr, err)` EVEN ON ERROR (partial output captured to separate buffers).
- Error contract (check `ctx.Err()` first inside Execute):
  - timeout ⇒ `err IS context.DeadlineExceeded`
  - signal/parent cancel ⇒ `err IS context.Canceled`
  - non-zero exit ⇒ wrapped `*exec.ExitError` (`fmt.Errorf("provider %q: %w", spec.Command, werr)`)
  - start failure ⇒ wrapped LookPath/start error
  - success ⇒ `err == nil`
- `vb *ui.Verbose` is NIL-SAFE (verbose.go:40 `if v == nil || v.w == nil || !v.on { return }`).
  Run passes `deps.Verbose` (which may be nil) directly — no nil guard needed. Execute also emits the
  per-turn verbose surface (VerboseCommand/VerboseRawOutput/VerboseStderr/VerbosePayload) automatically,
  so Run needs NO extra verbose calls (FR-T11's per-turn logging is handled by Execute).

### 2.2 `manifest.RenderMultiTurn` (render.go:203 — confirmed)

```go
func (m Manifest) RenderMultiTurn(model, sysPrompt, userPayload, reasoning, sessionID string, turn int) (*CmdSpec, error)
```

- Validates; then **FR-T8/T9 capability gate**: `if *r.SessionMode != "append" { return nil, error }`.
  ⇒ Run does NOT need to check session_mode itself — the render gate rejects a non-append provider and
  Run surfaces that error as `cause`. (S3's FR-T1 gate is the upstream guard; this is defense-in-depth.)
- **FR-T6 turn-1-only system prompt**: `turnSys := sysPrompt; if turn != 1 { turnSys = "" }`. Delivered
  via `system_prompt_flag` when non-empty; else prepended to the payload. Run passes `sysPrompt` on
  turn 1 and `""` on turns 2..N+1 — exactly this contract.
- Session flags: `BareFlags` MINUS `--no-session`, PLUS `--session-id <sessionID>`. `--continue`/`-c`
  NEVER used. `-p` (print flag) last. stdin delivery (`PromptDelivery="stdin"` ⇒ `spec.Stdin = payload`).
- Returns `*CmdSpec{Command, Args, Stdin, Env}` (render.go:22). Run dereferences `*spec` for Execute.

### 2.3 `provider.ParseOutput` (parse.go:41 — confirmed)

```go
func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)
```

- The EXISTING §9.6 pipeline (trim → strip-fence → raw/json → normalize → trim). Run calls it ONCE on
  the final turn's stdout. `ok = msg != ""`. Run returns `(m, ok, nil)` — the caller runs dedupe.
- `fellback` (the json-mode parse-fallback flag) is discarded by Run (it's a logging signal; FR-T11
  verbose already logged the raw output via Execute).

### 2.4 `chunk` + `chunkPayload` (S1's deliverable — the CONTRACT)

```go
type chunk struct { index, total int; text string }   // text = "PART i/N:\n<body>"
func chunkPayload(payload string, chunkTokens int) []chunk
```

- `chunks[0].text` = "PART 1/N:\n<body0>" (the PART prefix is baked into `.text`, OUTSIDE the body).
- `N = len(chunks)`; `total` is consistent across all chunks (== N). Run reads `N` for the preamble.
- Run sends `chunks[0].text` on turn 1 (after the preamble), `chunks[i-1].text` on turn i (2..N).

## 3. The N+1 turn protocol (the exact Run body)

```go
func Run(ctx context.Context, deps Deps, cfg config.Config, manifest provider.Manifest,
    sysPrompt, payload, msgModel, msgReasoning string) (msg string, ok bool, cause error) {

    // (1) Chunk the captured payload (FR-T2 lossless; FR-T3 sizing). chunkPayload is S1's helper.
    chunks := chunkPayload(payload, cfg.MultiTurnChunkTokens)
    N := len(chunks)

    // (2) Mint a fresh, one-run-scope session id (FR-T6). Never resumed.
    sessionID := newSessionID()

    // (3) Priming preamble (FR-T4, verbatim with N interpolated).
    preamble := fmt.Sprintf(preambleFmt, N)

    // (4) Turn 1: system prompt + preamble + chunk 1.
    spec, rerr := manifest.RenderMultiTurn(msgModel, sysPrompt, preamble+"\n\n"+chunks[0].text,
        msgReasoning, sessionID, 1)
    if rerr != nil {
        return "", false, rerr // non-append provider ⇒ RenderMultiTurn gate (FR-T8); surface as cause
    }
    if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {
        return "", false, execErr // FR-T7: any turn error/timeout aborts
    }

    // (6) Turns 2..N: each chunk's body; no system prompt (turn > 1 ⇒ RenderMultiTurn drops the flag).
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

    // (7) Turn N+1 (final): the commit-message request.
    spec, rerr = manifest.RenderMultiTurn(msgModel, "", finalInstruction,
        msgReasoning, sessionID, N+1)
    if rerr != nil {
        return "", false, rerr
    }
    out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
    if execErr != nil {
        return "", false, execErr // (8) FR-T7: final-turn error aborts
    }

    // (9) Parse the final turn's stdout via the EXISTING pipeline (FR-T4); do NOT fork dedupe.
    m, parseOK, _ := provider.ParseOutput(out, manifest)
    return m, parseOK, nil
}
```

With package-level constants:
```go
const preambleFmt = "I will send a git diff in %d parts. After each part, reply with exactly: ok. Do not analyze or write any commit message until I explicitly ask at the end."
const finalInstruction = "Now write the commit message for the diff above. Output ONLY the message."
```

## 4. The decision: `execErr != nil` (NOT `errors.Is(...)`)

The contract's pseudocode mentions `errors.Is(execErr, context.DeadlineExceeded) || ...`. But its own
clarification ("treat non-zero exit as failure too per FR-T7") means **ANY** execErr != nil aborts.
So the simplified `execErr != nil` is correct AND cleaner:

- **Intermediate turns (1..N):** the output is the discarded "ok". There is NO value in parsing partial
  stdout from a failed intermediate turn (unlike the one-shot path, which falls through to ParseOutput
  on a non-zero exit because the message might be partial-valid). So any error ⇒ abort.
- **Final turn (N+1):** FR-T7 explicitly lists "a turn's provider error (non-zero exit that is not a
  timeout), a turn timeout" as abort conditions. So any error ⇒ abort.

This AVOIDS importing `errors` for this purpose. (The CALLER, S3, discriminates timeout-vs-error via
`errors.Is(cause, context.DeadlineExceeded)` when constructing the RescueError — that's S3's job.)

**Contrast with the one-shot path (generate.go:242):** one-shot falls through to ParseOutput on a
non-zero exit (`lastCause = execErr; ... m, ok, _ := ParseOutput(out, ...)`). Multi-turn is STRICTER
(FR-T7) because a failed intermediate turn compromises the session's accumulated context. Run deliberately
does NOT mirror one-shot's fall-through.

## 5. The session id (no uuid lib exists — use crypto/rand)

`grep -rn 'uuid|crypto/rand' --include='*.go' internal/ cmd/ pkg/` ⇒ ZERO matches. go.mod has NO uuid
library (only go-toml/v2, cobra, pflag, yaml.v3). So mint the id with `crypto/rand`:

```go
func newSessionID() string {
    var b [16]byte
    if _, err := rand.Read(b[:]); err != nil {
        return fmt.Sprintf("stagehand-%d", time.Now().UnixNano()) // crypto/rand should never fail on a sane system
    }
    return "stagehand-" + hex.EncodeToString(b[:])
}
```

- 16 random bytes → 32 hex chars → "stagehand-<32 hex>". Sufficient entropy for per-run uniqueness.
- The fallback (time.UnixNano) is defense-in-depth (crypto/rand.Read practically never fails on Linux/macOS/Windows).
- FR-T6: "stagehand mints a fresh, unique session id per multi-turn run … never resumed on a later run."

## 6. The focused smoke test (the S1 precedent; T4 extends)

S1 shipped a focused smoke test (7 functions); S4 extends with the exhaustive matrix. By the same
precedent, S2 ships a FOCUSED Run smoke test; **T4 (P1.M1.T4.S1/S2) extends with the exhaustive
integration matrix** (mid-turn failure isolation, small-payload skip, non-append provider skip,
--session-id presence end-to-end, commit lands).

### 6.1 The stub seam (verified)

`stubtest.Build(t)` compiles `cmd/stubagent` once per test process. `stubtest.NewScript(t, bin,
responses)` writes `responses` joined by "\n" to a script file + a counter file; the stub's
`selectScripted` (cmd/stubagent/main.go) advances the counter per invocation ⇒ call 1 returns
`responses[0]`, call 2 returns `responses[1]`, etc. (clamps to last when exhausted). This is EXACTLY
the N+1 turn pattern: `["ok", "ok", …, "<final message>"]`.

### 6.2 The SessionMode="append" requirement (the one test-side gotcha)

`stubtest.Manifest`/`NewScript` do NOT set `SessionMode`. `RenderMultiTurn` REQUIRES
`*r.SessionMode == "append"` (else it errors). So the test MUST set it on the returned manifest value:

```go
m := stubtest.NewScript(t, bin, []string{"ok", "ok", "feat: add multi-turn"})
appendMode := "append"
m.SessionMode = &appendMode   // RenderMultiTurn's FR-T8 gate requires this
```

`stubtest.NewScript` returns `provider.Manifest` BY VALUE ⇒ the field assignment mutates the local
copy (clean; no shared-state leak to other tests).

### 6.3 The test matrix (4 focused tests)

| Test | Fixture | Asserts | What it proves |
|---|---|---|---|
| `TestRun_HappyPath` | payload "aaaa\nbbbb\n", chunkTokens=1 ⇒ N=2; script ["ok","ok","feat: add mt"]; SessionMode=append | `(msg=="feat: add mt", ok==true, cause==nil)` | The 3-turn protocol (priming + chunk2 + final) returns the final parsed message losslessly |
| `TestRun_TurnError` | same, but `Options{Exit:1}` (global) | `cause != nil` on turn 1 | FR-T7: a turn's provider error aborts; Run surfaces the raw cause |
| `TestRun_FinalParseEmpty` | script ["ok","ok",""] | `(msg=="", ok==false, cause==nil)` | Final-turn parse failure ⇒ ok=false, NOT a cause (caller decides rescue) |
| `TestRun_NonAppendManifest` | SessionMode NOT set (⇒ "") | `cause != nil` (the render error) | Defense-in-depth: Run surfaces RenderMultiTurn's session_mode gate as cause |

`cfg.MultiTurnChunkTokens = 1` (⇒ runesPerWindow=4 ⇒ "aaaa\nbbbb\n" chunks into 2). `deps := Deps{}`
(Verbose nil — safe). `cfg := config.Defaults()` then override MultiTurnChunkTokens.

### 6.4 The mid-turn-failure gap (T4's territory)

The stub's `STAGEHAND_STUB_EXIT` is a single env var baked into the manifest's Env map ⇒ ALL turns
get the same exit code. So "turn 1 ok, turn 2 fails" cannot be isolated with the current stub. S2's
`TestRun_TurnError` uses a GLOBAL exit failure (turn 1 fails). T4 owns the exhaustive mid-turn
isolation matrix (it can extend the stub with a per-call exit-indexed mechanism, or use a wrapper).
This is the clean S1-pre precedent: ship the focused proof, defer the exhaustive matrix.

## 7. Decisions log

- **D1** `execErr != nil` (any error aborts), NOT `errors.Is(...)`. FR-T7 treats timeout AND non-zero
  exit AND cancel as abort; intermediate turns discard stdout anyway. Avoids importing `errors` for this.
- **D2** Run returns the RAW cause (execErr/rerr); it does NOT construct `*RescueError`. The caller
  (S3/CommitStaged) maps `cause` to `&RescueError{Cause: cause}` (per the contract OUTPUT §4).
- **D3** `newSessionID()` via `crypto/rand` + `encoding/hex` (no uuid lib/helper exists in the repo).
- **D4** Run does NOT call `config.ResolveRoleModel` — the caller resolves `(msgModel, msgReasoning)`
  and passes them in (mirrors how CommitStaged resolves at generate.go:222 then passes to Render).
- **D5** Run does NOT check `cfg.MultiTurnFallback` or `session_mode` — S3 owns the FR-T1 trigger gate
  (upstream); RenderMultiTurn's own `session_mode` gate is the defense-in-depth (surfaced as `cause`).
- **D6** Run does NOT fork ParseOutput or dedupe — returns `(msg, ok, nil)`; CommitStaged's existing
  dedupe path handles the returned msg (per the contract NOTE).
- **D7** Constants `preambleFmt`/`finalInstruction` are package-level in multiturn.go (verbatim FR-T4
  strings; the preamble interpolates N). Avoids re-allocating the final instruction per run.
- **D8** The smoke test is focused (4 tests); T4 extends (the S1-pre precedent — S1 shipped 7 smoke
  tests, S4 extends). The mid-turn-isolation gap is T4's (the stub's global exit code can't isolate).

## 8. Imports S2 adds to multiturn.go (S1 created the file)

S1's multiturn.go imports: `fmt`, `strings`, `unicode/utf8`, and (optionally) `internal/git` (S1's PRP
offers two options: the `var _ = git.EstimateTokens` anchor OR drop the import + cite /4 in a comment).
S2 ADDS (Run + newSessionID):
- `context` (Run's ctx param)
- `crypto/rand` (newSessionID)
- `encoding/hex` (newSessionID)
- `time` (newSessionID fallback)
- `github.com/dustin/stagehand/internal/config` (config.Config)
- `github.com/dustin/stagehand/internal/provider` (Manifest, Execute, ParseOutput, CmdSpec)

S2 does NOT add `errors` (D1: `execErr != nil` suffices). S2 PRESERVES S1's choice for `internal/git`
(Run adds no git usage; if S1 kept the anchor, it stays; if S1 dropped the import, it stays dropped).

## 9. Scope fence (the plan)

- **S1 (Implementing, in parallel)** = `chunkPayload` + `chunk` + smoke test (creates multiturn.go +
  multiturn_test.go). THIS task BUILDS ON S1's file (adds Run + newSessionID + extends the test file).
- **S2 (this)** = the `Run` protocol + `newSessionID` + focused Run smoke test.
- **S3** = wire the FR-T1 trigger gate into CommitStaged (reads cfg.MultiTurnFallback +
  EstimateTokens(payload) > cfg.MultiTurnChunkTokens) + progress + verbose. S3 is the CALLER of Run.
- **S4** = exhaustive chunk-math matrix + trigger truth table (S3's gate) + token_limit non-interaction
  + Mode A how-it-works.md. S4 EXTENDS multiturn_test.go.
- **T4 (P1.M1.T4)** = integration tests: stub multi-turn end-to-end (N+1 turns, --session-id present,
  --no-session dropped, commit lands; mid-turn failure → rescue; small-payload skip; non-append skip).

DO NOT: implement the trigger gate (S3), the exhaustive matrix (S4), the integration tests (T4), the
how-it-works doc (S4), or touch any file outside multiturn.go/multiturn_test.go.
