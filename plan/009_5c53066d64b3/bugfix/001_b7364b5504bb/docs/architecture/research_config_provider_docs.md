# Research: Config / Provider / Prompt / Verbose / Docs (Issue 3, Issue 4, docs-sync)

Scope: read-only architecture investigation supporting Issue 3 (verbose per-chunk token estimate),
Issue 4 (payload rebuild for `mtPayload`), and the final documentation sync. All paths are repo-relative
to `/home/dustin/projects/stagehand`.

---

## 1. CONFIG — `internal/config/config.go`, `internal/config/multiturn_test.go`, `internal/config/file.go`

### 1a. The relevant `Config` fields (internal/config/config.go:80-145)

All live on the flat, resolved `Config` struct (produced by the 7-layer precedence resolver):

| Field | Go type | TOML key | Default (`Defaults()`, config.go:104-135) | Notes |
|---|---|---|---|---|
| `MultiTurnFallback` | `bool` | `[generation] multi_turn_fallback` | `true` | §9.24 FR-T1c. **Only-true-propagates** — setting `false` in a file is silently ignored (mirrors `AutoStageAll`); to disable, set `session_mode=""` on the provider. |
| `MultiTurnChunkTokens` | `int` | `[generation] multi_turn_chunk_tokens` | `32000` | §9.24 FR-T3 per-request chunk size (tokens est). `!= 0` guard (mirrors `TokenLimit`). Does NOT interact with `token_limit` (FR-T12). |
| `TokenLimit` | `int` | `[generation] token_limit` | `0` | FR3d holistic token cap over whole payload. `0` = unset ⇒ legacy per-section caps (`max_diff_bytes`/`max_md_lines`) apply. Multi-turn path deliberately IGNORES it (re-captures diff with `TokenLimit=0`). |
| `MaxDuplicateRetries` | `int` | `[generation] max_duplicate_retries` | `3` | Re-gen attempts on duplicate subject. `!= 0` guard. |
| `DiffContext` | `*int` | `[generation] diff_context` | `intPtr(1)` (nil-safe → 1) | FR3f reduced context (0–3). `*int` so an explicit `0` (changed-lines-only) is distinguishable from "omitted". `!= nil` guard (NOT `!= 0`). `DiffContextValue()` returns the plain int. |
| `Context` | `string` | **none** (`toml:"-"`) | `""` | §9.19 FR-F7 `--context` text. **FLAG-ONLY** (no env, no git key, no file key). Injected into user + planner payloads. |

### 1b. How these are loaded from TOML (internal/config/file.go)

Decode path: TOML → intermediate `fileConfig.fileGeneration` struct → `materialize()` copies into `Config` → `overlay()` merges layer-by-layer onto `Defaults()`.

`fileGeneration` struct (file.go:49-56):
```go
type fileGeneration struct {
    MultiTurnFallback    bool     `toml:"multi_turn_fallback"`
    MultiTurnChunkTokens int      `toml:"multi_turn_chunk_tokens"`
    // ... TokenLimit, DiffContext *int, MaxDuplicateRetries, etc.
}
```

**`materialize()` (file.go:236-242)** — file → Config copy (does NOT seed defaults):
```go
if g.MultiTurnFallback { c.MultiTurnFallback = true }        // only-true-propagates
if g.MultiTurnChunkTokens != 0 { c.MultiTurnChunkTokens = g.MultiTurnChunkTokens }
```

**`overlay()` (file.go:359-364)** — layer-merge into `dst` (Defaults sits at the bottom):
```go
if src.MultiTurnFallback { dst.MultiTurnFallback = true }
if src.MultiTurnChunkTokens != 0 { dst.MultiTurnChunkTokens = src.MultiTurnChunkTokens }
```

Key consequence (the v1 limitation pinned by tests): because `materialize`/`overlay` only copy when the
value is truthy/non-zero, **`multi_turn_fallback=false` in a file is indistinguishable from omitted** →
the default `true` always wins through the resolver. `multi_turn_chunk_tokens` honors any non-zero value
end-to-end (test `overlay/chunk_override_16000` proves it).

### 1c. The pinning test (internal/config/multiturn_test.go)

`TestMaterializeOverlay_MultiTurn` is the load-bearing proof. It covers:
- materialize-only cases (omitted ⇒ Go zero-value `false`/`0`, NOT the default);
- overlay chain cases including `fallback_false_ignored_STAYS_true` (the deliberate limitation pin) and
  `chunk_override_16000`/`chunk_override_48000`;
- repo-file-overrides-global precedence for chunk tokens;
- end-to-end via `loadTOML` (the TOML decode → resolved value path).

The test comment for the limitation case is explicit: to disable multi-turn, set `session_mode=""` on the
provider (see docs/configuration.md).

---

## 2. PROVIDER — `internal/provider/render.go`, `internal/provider/manifest.go`, `internal/provider/builtin.go`

### 2a. The `SessionMode` manifest field (manifest.go:62-67)

```go
// --- session continuation (multi-turn fallback, §9.24) ---
// "" (default): provider cannot append turns across one-shot calls → multi-turn fallback unavailable
//   for this provider (one-shot → rescue, unchanged). "append": re-invoking the same session id
//   appends a turn the model can recall (pi: `--session-id <id> ... -p`, repeated). REQUIRES a
//   verified append rendering (FR-T9); never set speculatively. nil => Resolve→"".
SessionMode *string `toml:"session_mode"`
```

- Type: `*string` (pointer; nil ⇒ `Resolve()` returns `""`).
- Validation (manifest.go:121-123): only `""` or `"append"` allowed; any other value is a hard error.
- Resolve default (manifest.go:177-178): if nil, set to `strPtr("")`.

### 2b. pi builtin `SessionMode` value — `"append"` (builtin.go, `builtinPi()`)

```go
SessionMode: strPtr("append"), // VERIFIED 2026-07-05 via `pi --session-id X ...` then recall returns BANANA; FR-T9.
```

**Only pi ships `"append"`.** Every other built-in (claude, gemini, opencode, codex, cursor, agy,
qwen-code) leaves it nil → resolves to `""` → multi-turn unavailable. This is also reflected in
`providers/pi.toml:57` (`session_mode = "append"`).

### 2c. `Render` method (render.go:97-189)

Signature (variadic mode, defaults `RenderBare` — keeps all 24+ v1 callers unchanged):
```go
func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error)
```

`Render` is the single-commit / one-shot path. It does NOT consult `SessionMode`. `RenderMode` is one of
`RenderBare` (default, tools off) or `RenderTooled` (stager role). Token order per §12.2: subcommand →
provider flag (slash-prefix split for `provider_flag` providers) → model flag → reasoning tokens → system
prompt flag → bare/tooled flags → print flag (last) → payload via the delivery switch.

### 2d. `RenderMultiTurn` method (render.go:196-312)

Signature — a SIBLING of `Render` (deliberately NOT a `RenderMode`, because the variadic mode has no slot
for a session id, and widening `Render` would touch 24+ call sites):
```go
func (m Manifest) RenderMultiTurn(model, sysPrompt, userPayload, reasoning, sessionID string, turn int) (*CmdSpec, error)
```

It is identical to `Render` except for THREE multi-turn deltas:
1. **Capability gate (FR-T8/T9)** — FIRST check, errors out unless `*r.SessionMode == "append"`:
   ```go
   if *r.SessionMode != "append" {
       return nil, fmt.Errorf("provider %q: multi-turn render requires session_mode=\"append\"", m.Name)
   }
   ```
2. **Turn-1-only system prompt (FR-T6)** — a `turnSys` local (`sysPrompt` iff `turn==1` else `""`) keys
   BOTH the flag-emission guard AND the prepend-fallback guard.
3. **Session-flags block (FR-T6)** — `BareFlags` MINUS the exact `"--no-session"` token, PLUS
   `"--session-id", sessionID` (built into a FRESH slice; `r.BareFlags` is never mutated). `--continue`/`-c`
   is NEVER used (incompatible with `--session-id`).

`print_flag` stays LAST; the payload + delivery switch + Env are byte-identical to `Render`.
`P1.M1.T3.S2` (the `Run` function) calls this once per turn (N+1 calls total).

### 2e. The `resolved` provider in `CommitStaged` (internal/generate/generate.go:222)

```go
resolved := deps.Manifest.Resolve()
retryInstr := *resolved.RetryInstruction  // resolved default: "Output ONLY the commit message…"
```

`resolved` is the **resolved manifest snapshot** (`Manifest.Resolve()` returns a copy with every pointer
non-nil → safe `*r.X` deref). It is used:
- to read `RetryInstruction` (one-shot retry preamble);
- in the multi-turn trigger gate (next item) via `resolved.SessionMode`.

The capability gate in the trigger (generate.go:329-331) reads the resolved manifest's `SessionMode`:
```go
if cfg.MultiTurnFallback &&
    resolved.SessionMode != nil && *resolved.SessionMode == "append" {
    // ... FR-T12 re-capture, chunk-size check (b), Run(...)
}
```
Condition (d) in the gate = `resolved.SessionMode == "append"`. (Note `Resolve()` guarantees non-nil, so
the `!= nil` guard is defensive — `*r.SessionMode` is safe.)

---

## 3. PROMPT — `internal/prompt/payload.go`

### `BuildUserPayload` signature (payload.go:120)

```go
func BuildUserPayload(diff, context string, rejected []string) string
```

Assembles the user payload delivered to the agent via stdin (never as a CLI arg — FR15). Inputs:
- `diff` — the staged diff body (appended VERBATIM, no normalization; trailing bytes preserved).
- `context` — the §9.19 FR-F7 `--context` text (`""` when unset). Used via `contextBlock(context)` which
  returns `contextIntro + "\n" + text` or `""`.
- `rejected` — `[]string{}` on first attempt; non-empty (matched duplicate subjects) on a retry.

Assembly (§17.3/§17.8):
- **NORMAL** (`len(rejected) == 0`): `userInstruction + "\n\n" + [contextBlock + "\n\n" if context!=""] + diff`
  → `"Generate a commit message for these changes:\n\n<diff>"` (COLON instruction + blank + diff).
- **REJECTION** (`len(rejected) > 0`): period instruction → blank → [context block → blank] →
  `rejectionPreamble` → per-subject list (`"- " + s` each) → blank → `rejectionEpilogue` → blank → diff.

### Why this matters for Issue 4 (the `mtPayload` rebuild)

In `CommitStaged`'s multi-turn trigger gate (generate.go:336-351), when `cfg.TokenLimit != 0` the one-shot
`payload` is truncated and unsuitable, so it RE-CAPTURES the diff and REBUILDS the payload:
```go
mtPayload := payload
if cfg.TokenLimit != 0 {
    fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
        ...same as one-shot EXCEPT TokenLimit: 0,  PromptReserveTokens: 0,   // FR-T12
    })
    if derr == nil {
        mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
    }
    // on derr: fall back to the one-shot payload (best-effort)
}
```
So `BuildUserPayload(fullDiff, cfg.Context, rejected)` is THE rebuild call for Issue 4. The `rejected`
slice at that point holds every duplicate subject from the exhausted one-shot loop (so the multi-turn
turn-N+1 message is steered away from duplicates — parity with one-shot).

---

## 4. VERBOSE — `internal/ui/verbose.go`

The verbose helpers live in **`internal/ui/verbose.go`** (NOT `internal/generate`). Receiver type `*ui.Verbose`.
Every method is a no-op when `v == nil`, `v.w == nil`, or `!v.on` (so callers thread a possibly-nil
`*Verbose` and call unconditionally — no nil guards).

Constructor:
```go
func NewVerbose(w io.Writer, on bool) *Verbose   // on = cfg.Verbose (resolved across all 7 layers)
```

The relevant helper signatures:
```go
func (v *Verbose) VerboseCommand(cmd string)         // "DEBUG: command: <argv>\n"  (argv only — NEVER Env/Stdin)
func (v *Verbose) VerboseRawOutput(output string)    // "DEBUG: raw output:\n<stdout>"
func (v *Verbose) VerboseStderr(stderr string)       // "DEBUG: stderr:\n<stderr>" (no-op if stderr=="")
func (v *Verbose) VerbosePayload(bytes int)          // "DEBUG: payload: <bytes> bytes (~<tokens> tokens est)\n"
func (v *Verbose) VerboseWarn(msg string)            // "DEBUG: <msg>\n"
func (v *Verbose) VerboseRetry(attempt int, reason string)  // "DEBUG: attempt <n>: <reason>\n"
func (v *Verbose) VerboseRoles(roles []RoleLine)     // the four-role roster (PRD §9.13 FR51b)
```

### Issue 3 hook point: `VerbosePayload` (verbose.go:85-99)

```go
func (v *Verbose) VerbosePayload(bytes int) {
    if v == nil || v.w == nil || !v.on || bytes <= 0 {
        return
    }
    fmt.Fprintf(v.w, "DEBUG: payload: %d bytes (~%d tokens est)\n", bytes, (bytes+3)/4)
}
```

- **Signature: `VerbosePayload(bytes int)`** — takes BYTES, not tokens. It computes its OWN rough estimate
  inline as `(bytes+3)/4` (byte-based, NOT the rune-based `git.EstimateTokens`).
- Called in **`internal/provider/executor.go:64`**: `vb.VerbosePayload(len(spec.Stdin))` — once per Execute.
- For Issue 3 (per-chunk token estimate), the natural approach is to add a NEW verbose method (e.g.
  `VerboseMultiTurnChunk(...)`) OR call `git.EstimateTokens` and `VerboseWarn`/a new method at the
  `chunkPayload(...)` call site in `generate.go`'s trigger gate. Note the multi-turn path already emits
  `deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")` (generate.go:341) right after
  computing `turns := len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1` (generate.go:347) — so a
  per-chunk estimate can be derived from the same `chunkPayload` result (each `chunk` has a known token size).

**Consistency note for Issue 3**: `VerbosePayload` uses `(bytes+3)/4` (byte-based), while
`git.EstimateTokens` uses `ceilDiv(utf8.RuneCountInString(s), 4)` (rune-based, UTF-8/CJK-safe). The trigger
gate and chunk sizing use the rune-based `git.EstimateTokens`. If Issue 3 surfaces a per-chunk estimate, it
should use `git.EstimateTokens(chunk.text)` (the authoritative estimator), NOT `(bytes+3)/4`.

---

## 5. DOCS — multi-turn documentation surface

### 5a. All files in `docs/` (and the index)

- `docs/README.md` — documentation index (lists all four pages + capability anchors).
- `docs/cli.md` — CLI reference.
- `docs/configuration.md` — 7-layer precedence, config format, env vars, git-config keys, built-in defaults.
- `docs/how-it-works.md` — snapshot architecture, multi-commit decomposition, prompt engineering.
- `docs/providers.md` — 21-field manifest schema, command rendering, 8 built-in providers.

(`FUTURE_SPEC.md` and `PRD.md` sit at repo root, not in `docs/`.)

### 5b. Where multi-turn is documented today

**README.md:68** — one row in the Features table:
```
| Multi-turn fallback | Lossless multi-turn fallback: when a one-shot generation of a large diff fails,
  stagehand re-delivers the full diff across session turns so the message still lands — no truncation,
  no extra commits ([how it works](docs/how-it-works.md#multi-turn-generation-fallback) · [knobs](docs/configuration.md#built-in-defaults)). |
```

**docs/how-it-works.md:262-298** — a full `## Multi-turn generation fallback` section. Covers:
- why it exists (per-request reliability ceiling below advertised context window);
- the FOUR trigger conditions (1. one-shot exhausted; 2. payload > one chunk via `multi_turn_chunk_tokens`
  default 32000; 3. `multi_turn_fallback` enabled default true; 4. resolved provider `session_mode="append"`,
  only pi);
- lossless-not-summarized design (N+1 turns: turn-1 priming+chunk1, turns 2..N `PART i/N:` chunks, turn N+1
  "write the message"; newline-anchored boundaries so no diff line fractures);
- failure handling (any turn error/timeout/parse-fail → standard rescue; snapshot safe);
- **`token_limit` does not apply (FR-T12)** — re-captures the diff with `token_limit` disabled and delivers
  the untruncated diff; chunking never consults `token_limit`.

**docs/configuration.md**:
- Lines 110-111 — commented defaults in the config template:
  ```
  # multi_turn_fallback     = true   # lossless multi-turn fallback on one-shot exhaustion (§9.24 FR-T1c); CANNOT disable via file
  # multi_turn_chunk_tokens = 32000  # per-turn chunk budget in tokens (§9.24 FR-T3); does NOT interact with token_limit (FR-T12)
  ```
- Lines 137-138 — built-in defaults table: `multi_turn_fallback`=`true`, `multi_turn_chunk_tokens`=`32000`.
- Lines 155-157 — a `**Multi-turn fallback.**` blockquote that documents the only-true-propagates limitation
  (`multi_turn_fallback=false` in a file is silently ignored; to disable set `session_mode=""` on the
  provider; shipped pi default is `"append"`) AND the FR-T12 non-interaction with `token_limit`.
- Line 24 — a `session_mode` override note (set `session_mode=""` on a pi provider to disable; setting
  `session_mode="append"` on a `""`-shipped provider is at the user's FR-T9 risk).

**docs/providers.md**:
- Line 29 — the `session_mode` row in the manifest schema table (default `""`; only pi ships `"append"`;
  VERIFIED 2026-07-05 FR-T9).
- Lines 40-49 — a `### Multi-turn capability (`session_mode`)` subsection defining `"append"` vs `""` and
  the FR-T9 verification bar ("a manifest MUST NOT declare `"append"` speculatively").

**docs/README.md** — does NOT have a dedicated multi-turn capability-index line (the capability index lists
payload exclusions, message shaping, git hook, tool integrations, `--edit`/`--push`, discovery). Multi-turn
is reachable only via the how-it-works/configuration anchors from the README Features row.

### 5c. Docs-sync implications

For the final docs-sync task, the multi-turn surface is already substantial and accurate as of the
FR-T9/FR-T12 verification. Likely sync needs if Issue 3/4 change behavior:
- If Issue 3 adds a per-chunk token estimate to verbose output, no doc change is strictly required (verbose
  diagnostics are not a user-facing feature surface), but a one-line note under how-it-works.md's
  multi-turn section could mention `--verbose` shows the per-chunk breakdown.
- If Issue 4 changes the `mtPayload` rebuild (e.g. handles the re-capture error path differently), the
  how-it-works.md `token_limit does not apply (FR-T12)` paragraph is the place to verify wording still
  matches code.
- The configuration.md `multi_turn_fallback`/`multi_turn_chunk_tokens` defaults (110-111, 137-138) and the
  limitation blockquote (155-157) should be re-checked against any default/behavior change.

---

## 6. ESTIMATE TOKENS — `internal/git/tokens.go`

```go
func EstimateTokens(s string) int       // tokens.go:25 — ceil(runeCount / 4), rune-based
func EstimateTokensBytes(b []byte) int  // tokens.go:32 — []byte form, same formula
func ceilDiv(n, d int) int { return (n + d - 1) / d }  // tokens.go:39
```

- **The SINGLE model-agnostic token estimator** (PRD §9.1 FR3d/FR3i). Rune-based via
  `utf8.RuneCountInString(s)` (NOT `len(s)`) so multi-byte UTF-8 (CJK, emoji) does not over-count.
- Formula: `ceil(runes / 4)` — the standard "~4 chars ≈ 1 token" heuristic, rounded UP.
- Used by BOTH the prompt-reserve measurement (`prompt.MessageReserveTokens`) AND the FR3i water-fill
  sizing/truncation, so budget arithmetic is in consistent units.
- **Used in the multi-turn trigger gate** (generate.go:343):
  ```go
  if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens { ... }
  ```
  This is condition (b) of the four-condition gate. The same function sizes the chunks inside
  `chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)` (internal/generate/multiturn.go:52).
- Do NOT introduce a second estimator; do NOT "improve" it to chars/3 (the safety `margin` is the
  reconciliation mechanism, not the formula).

---

## Start Here

Open `internal/generate/generate.go` around lines 325-360 — the multi-turn trigger gate. It ties together
every piece: `resolved.SessionMode` (provider capability), `cfg.MultiTurnFallback`/`cfg.MultiTurnChunkTokens`
(config), `git.EstimateTokens` (the gate's condition b), `prompt.BuildUserPayload` (the `mtPayload` rebuild
for Issue 4), `deps.Verbose.VerboseWarn` (the Issue 3 verbose seam), and `chunkPayload` (the per-chunk
sizing for Issue 3).

---

## Files Retrieved (exact paths + line ranges)

1. `internal/config/config.go:80-135` — the `Config` struct fields + `Defaults()` (config knobs + defaults).
2. `internal/config/file.go:49-56, 220-242, 345-364` — `fileGeneration` decode struct, `materialize()`, `overlay()` (TOML → Config flow + the only-true-propagates limitation).
3. `internal/config/multiturn_test.go:1-end` — the load-bearing proof for the multi-turn config knobs (incl. the limitation pin).
4. `internal/provider/manifest.go:62-67, 121-123, 177-178` — `SessionMode` field, validation, Resolve default.
5. `internal/provider/render.go:97-189` — `Render` (one-shot, ignores SessionMode).
6. `internal/provider/render.go:196-312` — `RenderMultiTurn` (sibling method; capability gate + 3 deltas).
7. `internal/provider/builtin.go` (`builtinPi`) — pi `SessionMode = strPtr("append")`; all others nil → `""`.
8. `internal/generate/generate.go:160-365` — `CommitStaged`, the `resolved := deps.Manifest.Resolve()` snapshot, the multi-turn trigger gate (the `mtPayload` rebuild + `VerboseWarn` + `git.EstimateTokens` check).
9. `internal/provider/executor.go:64` — `vb.VerbosePayload(len(spec.Stdin))` (the per-Execute verbose call).
10. `internal/prompt/payload.go:120-end` — `BuildUserPayload(diff, context, rejected)` (the rebuild target for Issue 4).
11. `internal/ui/verbose.go:32-end` — the `*Verbose` helpers (`VerbosePayload(bytes int)`, `VerboseWarn`, `VerboseRetry`, etc.).
12. `internal/git/tokens.go:25-39` — `EstimateTokens(s string) int`, `EstimateTokensBytes`, `ceilDiv` (the single estimator; gate condition b).
13. `docs/how-it-works.md:262-298` — the full `## Multi-turn generation fallback` section.
14. `docs/configuration.md:24, 110-111, 137-138, 150-157` — multi-turn knobs in defaults table + template + limitation blockquote.
15. `docs/providers.md:29, 40-49` — `session_mode` schema row + the multi-turn capability subsection.
16. `docs/README.md` — the docs index (no dedicated multi-turn capability-index line).
17. `README.md:68` — the Features-table multi-turn row.
18. `providers/pi.toml:57` — `session_mode = "append"` (the shipped pi default).
19. `internal/generate/multiturn.go:52` — `func chunkPayload(payload string, chunkTokens int) []chunk` (chunk sizing for Issue 3's per-chunk estimate).
