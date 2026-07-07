# Code Context — Provider Manifest/Render Surface for `session_mode` + Multi-Turn

Scout for PRD §9.24 (FR-T1–T12) — adding a `session_mode *string` manifest field and a multi-turn `Render` variant. **Research-only; no files modified** (per task instruction). All line numbers from the current tree (verified 2026-07-05).

---

## 1. `internal/provider/manifest.go` — the `Manifest` struct

**File:** `internal/provider/manifest.go`

### Struct layout — section headers in order
| Lines | Section | Notes |
|---|---|---|
| 35–40 | `// --- discovery (§12.1) ---` | `Name`, `Detect`, `Command`, `ListModelsCommand`, `Subcommand` |
| 42–45 | `// --- prompt delivery (§12.1) ---` | `PromptDelivery`, `PromptFlag` |
| 47–49 | `// --- non-interactive / print mode (§12.1) ---` | `PrintFlag` |
| 51–54 | `// --- model (§12.1) ---` | `ModelFlag`, `DefaultModel` |
| 56–58 | `// --- system prompt (§12.1) ---` | `SystemPromptFlag` |
| 60–61 | `// --- sub-provider (§12.1) ---` | `ProviderFlag *string` |
| 63–64 | `// --- bare mode (§12.1) ---` | `BareFlags []string` |
| 66–72 | `// --- tooled mode (v2; §11.5, §12.1) ---` | `TooledFlags []string` |
| 74–77 | `// --- experimental (§12.7.2, §12.5.1) ---` | `Experimental *bool` |
| 79–82 | `// --- output (§12.1) ---` | `Output`, `JsonField`, `StripCodeFence` |
| 84–85 | `// --- retry (§12.1) ---` | `RetryInstruction *string` |
| 87–88 | `// --- environment (§12.1) ---` | `Env map[string]string` |
| 90–94 | `// --- reasoning levels (v3; FR-R6) ---` | `ReasoningLevels map[string][]string` |

### Where `SessionMode *string` slots in
**Authoritative placement comes from PRD §12.1 itself (`PRD.md:726–733`):**
```toml
# --- session continuation (multi-turn fallback, §9.24) ------------------
# "" (default): provider cannot append turns across one-shot calls → multi-turn
#   fallback unavailable for this provider (one-shot → rescue, unchanged).
# "append": re-invoking the same session id appends a turn the model can recall
#   (pi: `--session-id <id> ... -p`, repeated). REQUIRES a verified append
#   rendering (FR-T9); never set speculatively.
session_mode = ""

# --- bare mode -----------------------------------------------------------
```
So the new field goes **between `ProviderFlag` (sub-provider section, line 60–61) and `BareFlags` (bare mode section, line 63–64)** — i.e. a new `// --- session continuation (multi-turn fallback, §9.24) ---` block, mirroring the TOML ordering the PRD fixes. Proposed:

```go
// --- session continuation (multi-turn fallback, §9.24) ---
// "" (default; multi-turn unavailable) | "append" (pi: --session-id <id> ... -p repeats append a
// recallable turn). REQUIRES a verified append rendering (FR-T9); never set speculatively.
SessionMode *string `toml:"session_mode"`
```

### Why `*string` (pointer-scalar convention) — manifest.go:14–24
The doc-comment block (lines 14–24) explains: go-toml/v2 has no `omitempty`, so optional SCALAR fields are `*string`/`*bool`. **Absent in a user override → nil → inherit the built-in on merge.** Present (even `""`/`false`) → non-nil → override. This is the ONLY way a field-by-field merge can honor a user's explicit `strip_code_fence=false` or `print_flag=""` override. `SessionMode` must follow the same convention (it's a config-overridable scalar, default-empty).

### `Resolve()` — manifest.go:110–164
Signature: `func (m Manifest) Resolve() Manifest` (returns a copy; caller's `m` untouched).
- For each nil optional, fills the §12.1 default (e.g. `PromptDelivery→"stdin"`, `Output→"raw"`, `StripCodeFence→true`).
- Remaining optional `*string` fields take `*""`.
- Slices/maps left as-is (nil stays nil).
- **EXPLICIT values are PRESERVED** — never overwritten (the correctness keystone of the pointer design).
- For `SessionMode`: `Resolve()` should add a `if out.SessionMode == nil { out.SessionMode = strPtr("") }` clause (mirrors `ProviderFlag`, `PrintFlag`, etc.). Insert adjacent to those (≈line 137, before the output block) or grouped with provider-related defaults.

`strPtr`/`boolPtr` helpers: manifest.go:166–167.

`Validate()` (manifest.go:88–106) is nil-tolerant on optional enums; if you want to reject anything other than `""`/`"append"`, add a check here (PRD §12.1 says the enum is `"" | "append"`).

---

## 2. `internal/provider/render.go` — `Manifest.Render`

**File:** `internal/provider/render.go`

### Full signature — render.go:89
```go
func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error)
```
- It is a **method on `Manifest`** (not a standalone func).
- `mode ...RenderMode` is **variadic**, defaulting to `RenderBare` when omitted — this is what keeps every v1 caller byte-for-byte unchanged (see the v1 caller inventory below).
- Calls `m.Validate()` then `m.Resolve()` internally (render.go:91–94), so it is safe against an unresolved registry manifest.

### `RenderMode` type — render.go:40–55
```go
type RenderMode string
const (
    RenderBare   RenderMode = "bare"   // BareFlags; tools off; default
    RenderTooled RenderMode = "tooled" // TooledFlags; tools on; errors if empty
)
```

### Exact args-construction order — render.go:96–143 (§12.2 v3 AUTHORITATIVE)
```
args = [subcommand...]
+ (--provider, <inference>)    if provider_flag != "" && model contains "/"  (FR-R5b fold)
+ (model_flag,    model)       if model_flag    != "" && model    != ""
+ reasoning_level_tokens...    if reasoning != "" && ReasoningLevels[reasoning] non-empty (FR-R6)
+ (system_prompt_flag, sys)    if system_prompt_flag != "" && sys != ""
+ (mode==tooled ? tooled_flags : bare_flags)...    # §11.5/§12.2 mode ternary; default bare
+ print_flag                   if print_flag != ""        // ALWAYS LAST (after flags)
+ payload                      per prompt_delivery switch (positional/flag only)
```
Code path for the mode ternary — render.go:124–136:
```go
selectedMode := RenderBare
if len(mode) > 0 && mode[0] != "" { selectedMode = mode[0] }
switch selectedMode {
case RenderTooled:
    if len(r.TooledFlags) == 0 {
        return nil, fmt.Errorf("provider %q: tooled mode requires non-empty tooled_flags", m.Name)
    }
    args = append(args, r.TooledFlags...)
default: // RenderBare — also fallback for "" / unrecognized
    args = append(args, r.BareFlags...)
}
```

### Where a multi-turn render variant hooks in
The cleanest seam is the **mode switch above (render.go:124–136)**. Two viable shapes (the delta_prd.md §R1 leaves this to the implementer; the PRD does not dictate):

**Option A — new `RenderMode` value:** add `RenderSessionAppend RenderMode = "session"` and a `case RenderSessionAppend:` branch that:
- drops `--no-session` (or, more generally, the manifest's "session-kill flag") from `BareFlags`,
- appends `--session-id <id>` (needs an extra param — see below),
- errors if `*r.SessionMode != "append"` (capability gate, FR-T8/T9).
- **Problem:** the variadic `mode ...RenderMode` carries no place for the session-id string. Either widen the signature (breaks ~24 call sites) or add a sibling method.

**Option B — sibling method `RenderMultiTurn`:** add `func (m Manifest) RenderMultiTurn(model, sysPrompt, userPayload, reasoning, sessionID string, turn int) (*CmdSpec, error)` that reuses the same token-order pipeline but swaps the bare_flags block for the session-flags variant. This keeps `Render` (and all 24+ existing call sites) untouched — strongly preferred; the decompose suite used the same sibling-pattern when adding `RenderTooled`.

### Existing concept of "dropping specific bare_flags"
**There is NO existing mechanism** for selectively dropping individual `bare_flags`. The renderer treats `BareFlags` as an opaque verbatim slice — `args = append(args, r.BareFlags...)` (render.go:135). The only precedent for flag-set *substitution* is the bare/tooled mode ternary itself. A multi-turn variant that drops `--no-session` from pi's `BareFlags` will need to either:
- filter by exact token (`"--no-session"`) inside the new branch, or
- add a manifest field like `SessionKillFlag *string` (`--no-session` for pi) so the drop target is provider-configurable. The PRD does not name this field — it's an implementer decision; the simplest correct option is hardcoding the `--no-session` filter (pi-only-shipped value, FR-T9 verified) inside the multi-turn branch.

### v1 `Render` call sites (for impact analysis — DO NOT change)
- `internal/generate/generate.go:237` (the one-shot message loop — **this is where multi-turn will be wired in, just before line 288's `RescueError`**)
- `pkg/stagecoach/stagecoach.go:495`, `internal/hook/exec.go:163`
- `internal/decompose/{planner,message,stager,arbiter}.go` (stager uses `RenderTooled`; others use bare default)
- `internal/stubtest/stubtest_test.go` (the stub-agent harness the multi-turn integration test should extend)

---

## 3. `internal/provider/builtin.go` — `builtinPi()`

**File:** `internal/provider/builtin.go`

### `builtinPi()` — full current manifest — builtin.go:30–96
```go
func builtinPi() Manifest {
    return Manifest{
        Name:              "pi",
        Detect:            strPtr("pi"),
        Command:           strPtr("pi"),
        ListModelsCommand: []string{"pi", "--list-models"}, // VERIFIED 2026-07-03
        PromptDelivery:    strPtr("stdin"),
        PrintFlag:         strPtr("-p"),
        ModelFlag:         strPtr("--model"),
        DefaultModel:      strPtr(""),
        SystemPromptFlag:  strPtr("--system-prompt"),
        ProviderFlag:      strPtr("--provider"),
        BareFlags: []string{
            "--no-tools",
            "--no-extensions",
            "--no-skills",
            "--no-prompt-templates",
            "--no-context-files",
            "--no-session",          // <-- MUST be dropped in multi-turn render variant (FR-T6)
        },
        ReasoningLevels: map[string][]string{
            "high":   {"--thinking", "high"},
            "medium": {"--thinking", "medium"},
            "low":    {"--thinking", "low"},
        },
        TooledFlags: []string{
            "--no-extensions",
            "--no-skills",
            "--no-prompt-templates",
            "--no-context-files",
            "--no-session",
        },
        Output:         strPtr("raw"),
        StripCodeFence: boolPtr(true),
    }
}
```

**Key findings:**
- `BareFlags` **DOES** contain `"--no-session"` (builtin.go:64) — this is exactly the flag FR-T6 says the multi-turn render must drop. (Same flag is also in `TooledFlags` line 91, but multi-turn uses the message role = bare, so only the `BareFlags` instance matters.)
- **No `SessionMode` field exists today** anywhere in the codebase (grep confirmed). The pi manifest has no such field.
- Where to add: `SessionMode: strPtr("append"),` — but **ONLY after FR-T9 verification** (see R1 in `delta_prd.md:43`). Until verified empirically (`pi --session-id X <isolation-minus-no-session> -p "remember BANANA"` then recall returns `BANANA`), ship `strPtr("")` or omit (which `Resolve()` defaults to `""`).

### Comparison builtin — `builtinClaude()` (builtin.go:101–149)
For shape-comparison: claude uses `--tools ""` / `--setting-sources ""` / `--no-session-persistence` (NOT `--no-session`), `ProviderFlag: strPtr("")` (NON-NIL empty, no sub-provider), `BareFlags` includes TWO `""` value tokens (the args to `--tools`/`--setting-sources`) which the doc explicitly warns **must not be dropped**. claude ships `SessionMode` absent (empty) — its append-turn mechanism is unverified.

### `# VERIFIED …` comment discipline (FR-D5)
Pattern is inline trailing comments on the field, format: `// VERIFIED <date> via \`<cmd>\` (exit 0); <FR ref>`. Examples:
- builtin.go:47 `ListModelsCommand: []string{"pi", "--list-models"}, // VERIFIED 2026-07-03 via \`pi --list-models\` (exit 0); FLAG form, not a subcommand. FR-L2/FR-D5.`
- builtin.go:184 (agy): "Flag surface VERIFIED vs `agy --help` + live -p runs on 2026-07-03"

When pi's `SessionMode: strPtr("append")` ships, the field SHOULD carry an inline comment recording the FR-T9 verification: `// VERIFIED <date> via \`pi --session-id X <flags> -p "remember BANANA"\` then recall returns BANANA; FR-T9.` Same discipline as `# VERIFIED` (mirrored in `providers/pi.toml:34`).

---

## 4. `internal/provider/merge.go` — `MergeManifest`

**File:** `internal/provider/merge.go` (NOT registry.go — `MergeManifest` lives in its own file)

### Signature — merge.go:28
```go
func MergeManifest(base, override Manifest) Manifest
```
Called from `NewRegistry` at registry.go:47. Three merge regimes (merge.go:30–52):

1. **Scalar pointer fields** (merge.go:54–95): `if override.X != nil { out.X = override.X }` — non-nil override WINS, explicit `""`/`false` included.
2. **Slices** (merge.go:97–108): `len(override.X) > 0` REPLACES wholesale (no element merge).
3. **Maps** (`Env`, `ReasoningLevels`) (merge.go:111–133): key-by-key into a FRESH map.

### Does a plain `*string` `SessionMode` merge the same way?
**YES — confirmed.** Add a clause in regime-1, mirroring the existing scalars (e.g. right after `ProviderFlag` at merge.go:80–82):
```go
if override.SessionMode != nil {
    out.SessionMode = override.SessionMode
}
```
This gives the exact FR-37a/§16.1 semantics: a user who sets only `session_mode = ""` in `[provider.pi]` overrides the built-in `"append"`; a user who omits the key inherits the built-in. A user setting `session_mode = "append"` on a provider whose built-in ships `""` (claude etc.) would override-up — but that's their explicit choice and matches the FR-T9 duty being on the SHIPPED default, not user config.

### Call site
`NewRegistry` (registry.go:42–55) — `merged := MergeManifest(base, override)` per override key. No changes needed there beyond the new merge clause.

---

## 5. `internal/provider/executor.go` — `Execute`

**File:** `internal/provider/executor.go`

### Full signature — executor.go:44
```go
func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout string, stderr string, err error)
```
This is **what each multi-turn turn will call** — unchanged. One `Execute` per turn.

### Behavior
- **Subprocess:** `exec.CommandContext(ctx, spec.Command, spec.Args...)` (executor.go:46).
- **stdin:** `spec.Stdin` (non-empty) → piped via `strings.NewReader`; `""` → `nil` → child gets `/dev/null` (executor.go:47–49).
- **stdout/stderr:** captured to SEPARATE buffers, **returned even on error** (executor.go:51–52, 75–78). Partial output is available to the rescue path.
- **Env:** `cmd.Env = spec.Env` when non-empty (Render builds `os.Environ() + manifest env`); nil → inherit parent (executor.go:53–55).
- **Timeout:** when `timeout > 0`, derives `context.WithTimeout` (executor.go:45–48) — **SHADOWS ctx** (load-bearing; later `ctx.Err()` reads the timeout ctx). Default `cfg.Timeout` = 120s (FR25). `timeout ≤ 0` ⇒ no timeout.
- **Process group / kill:** `setupProcessGroup(cmd)` (executor.go:56; platform seam in `procgroup_*.go`) — `Setpgid` makes the child its own PG leader; `cmd.Cancel` sends SIGTERM to `-pid` (whole group); `WaitDelay=3s` grace before SIGKILL. `signal.RegisterChild(cmd.Process.Pid)` arms forwarding (executor.go:62).
- **cmd.Dir NOT set** — agent runs in user's CWD.

### Error contract (executor.go:31–42) — check `ctx.Err()` FIRST
- timeout → `err IS context.DeadlineExceeded` → orchestrator exit 124 + rescue
- signal/parent → `err IS context.Canceled` → exit 3 + rescue
- non-zero exit → wrapped `*exec.ExitError` → retry, then rescue
- start failure → wrapped LookPath/start error → "command not found", exit 1
- success → `err == nil`

### Call sites for the multi-turn path
`internal/generate/generate.go:242` is the one-shot call: `out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)`. The multi-turn loop will issue N+1 `Execute` calls with the **same** `cfg.Timeout` per turn (FR-T5); total budget = `timeout × (N+1)`.

---

## 6. `parseOutput` / `ParseOutput` — parse.go

**File:** `internal/provider/parse.go`

### Signature — parse.go:41
```go
func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)
```
- Exported (consumer is `internal/generate`, a different package).
- Calls `m.Resolve()` internally (parse.go:53) → nil-pointer-safe; copy, caller's `m` untouched.
- Pure function over stdout + the manifest's three output fields (`Output`, `JsonField`, `StripCodeFence`).

### 5-step pipeline (parse.go:31–39, AUTHORITATIVE)
1. `s = TrimSpace(raw)`.
2. If `strip_code_fence` and `s` starts with ``` or ~~~: drop opener line + last closer; re-trim.
3. Switch on `Output`: `raw` → `msg=s`; `json` → `Unmarshal` whole, then brace-balanced substring, then `obj[json_field]` as string; ANY failure → fall through to raw + `fellback=true`.
4. Normalize newlines: `\r\n→\n`, collapse 3+ `\n` to 2.
5. `msg = TrimSpace(msg); ok = msg != ""`.

### Where it's called in the generate flow
`internal/generate/generate.go:264` — inside the one-shot retry loop:
```go
m, ok, _ := provider.ParseOutput(out, deps.Manifest)
if !ok { parseFail = true; candidate = m; ...; continue }  // FR29 retry
```
Other call sites: `pkg/stagecoach/stagecoach.go:520`, `internal/hook/exec.go:179`, `internal/decompose/message.go:165`.

### Multi-turn integration
FR-T4 specifies: **the FINAL turn's stdout is parsed by the EXISTING pipeline unchanged** — intermediate turns' "ok" stdout is discarded. So `ParseOutput` is called exactly ONCE per multi-turn run, on the final turn's `out`, and the result then runs through duplicate rejection (§9.7) as normal. No change to `ParseOutput` is required.

---

## Architecture — how the pieces connect (data flow)

```
config [provider.<name>]  ──►  DecodeUserOverrides (registry.go:161)
                                     │
                                     ▼
                          NewRegistry (registry.go:32)
                                     │  MergeManifest(base builtin, override) per name  (merge.go:28)
                                     ▼
                          Registry.manifests map[name]Manifest   (UNRESOLVED, UNVALIDATED)
                                     │  .Get(name) → Manifest
                                     ▼
   ┌──────────────── one-shot generate flow (generate.go:237–264) ───────────────┐
   │                                                                             │
   │  Manifest.Render(model, sys, payload, reasoning [, mode])  (render.go:89)   │
   │      └─ Validate() + Resolve() → safe *deref                                │
   │      └─ builds CmdSpec{Command, Args, Stdin, Env}                            │
   │            │                                                                │
   │            ▼                                                                │
   │  provider.Execute(ctx, *spec, timeout, vb)  (executor.go:44)                │
   │      └─ returns (stdout, stderr, err)                                       │
   │            │                                                                │
   │            ▼                                                                │
   │  provider.ParseOutput(stdout, manifest)  (parse.go:41)                      │
   │      └─ returns (msg, ok, fellback)                                         │
   │            │ ok==false → retry loop (FR29)                                   │
   │            │ dedupe (FR32)                                                   │
   │            ▼                                                                │
   │  FinalizeMessage → commit                                                   │
   └─────────────────────────────────────────────────────────────────────────────┘

   MULTI-TURN (NEW, FR-T1–T12) inserts BETWEEN one-shot-exhausted (generate.go:288
   RescueError) and rescue:
       trigger gate (FR-T1 a–d, incl. resolved SessionMode == "append")
         │
         ▼
       for turn in 1..N+1:
         spec = Manifest.RenderMultiTurn(...)   ← NEW sibling method (Option B, preferred)
              or Render(..., RenderSessionAppend) with session-id threading
              drops --no-session from BareFlags; adds --session-id <run-uuid>; -p
              sys prompt on turn 1 only (FR-T6)
         out, _, err = provider.Execute(ctx, *spec, cfg.Timeout, vb)   ← UNCHANGED
         on err/timeout → abort to rescue (FR-T7)
       msg, ok, _ = provider.ParseOutput(finalTurnStdout, manifest)    ← UNCHANGED
       dedupe as normal → commit OR rescue
```

---

## Start Here
1. **`internal/provider/manifest.go:60–64`** — add `SessionMode *string` between `ProviderFlag` and `BareFlags` (matches PRD §12.1 TOML ordering at `PRD.md:726–733`). Add the matching `Resolve()` clause (≈line 137, mirroring `ProviderFlag`). Optionally extend `Validate()` (line 88) to reject values other than `""`/`"append"`.
2. **`internal/provider/merge.go:80–82`** — add the regime-1 scalar clause for `SessionMode` (one `if override.SessionMode != nil` block).
3. **`internal/provider/builtin.go:30–96`** — on `builtinPi()` only, add `SessionMode: strPtr("append")` WITH an inline `// VERIFIED <date> ... FR-T9` comment, **after** the empirical append-turn verification passes (blocking per `delta_prd.md:43`). All other builtins stay absent (Resolve defaults to `""`).
4. **`internal/provider/render.go:124–136`** — add the multi-turn render variant (preferred: sibling `RenderMultiTurn(model, sys, payload, reasoning, sessionID string, turn int)` method that reuses the token pipeline but filters `--no-session` out of `BareFlags` and appends `--session-id <id>`; errors if `*r.SessionMode != "append"`). Keeps all 24+ existing `Render` call sites byte-for-byte unchanged.
5. **`internal/generate/generate.go:288`** — wire the FR-T1 trigger gate and the N+1-turn loop just before the `RescueError` return; reuse `provider.Execute` + `provider.ParseOutput` unchanged.

## Constraints, risks, open questions
- **FR-T9 is BLOCKING for the pi `"append"` value.** If empirical verification fails, ship pi with `SessionMode` absent/`""` and the whole feature is inert — surface this clearly (`delta_prd.md:92`).
- **No existing "drop a specific bare_flag" mechanism** — the multi-turn variant must introduce one (filter `--no-session` by exact token, or add a manifest field naming the kill flag). PRD does not name a `SessionKillFlag` field; simplest correct path is hardcoding `--no-session` (pi-only-shipped value).
- **`Render` signature widening is high-blast-radius** (24+ call sites across generate/hook/stagecoach/decompose/stubtest). Prefer a sibling method (`RenderMultiTurn`) over a 7th positional param.
- **Per-turn timeout × (N+1) can be many minutes** (FR-T5) — progress-line surface belongs in `generate.go`, not the provider package.
- **`token_limit` MUST NOT interact** with multi-turn (FR-T12) — the multi-turn payload is the UNTRUNCATED captured payload.

```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "Research completed within the requested scope (6 named files + the MergeManifest/ParseOutput/Execute call graph). No scope widening: report covers exactly the session_mode field placement, the Render multi-turn seam, builtin pi SessionMode, MergeManifest scalar-merge confirmation, Execute signature, and ParseOutput flow. Task instruction 'Do not modify any files' honored — no source edits; only the research artifact was written."
    }
  ],
  "changedFiles": [
    "plan/009_5c53066d64b3/architecture/research-provider.md"
  ],
  "testsAddedOrUpdated": [],
  "commandsRun": [
    {
      "command": "grep -n 'func MergeManifest' internal/provider/*.go",
      "result": "passed",
      "summary": "Located MergeManifest in merge.go:28 (not registry.go)"
    },
    {
      "command": "grep -rn 'SessionMode|session_mode' .",
      "result": "passed",
      "summary": "Confirmed no SessionMode symbol exists in source (only PRD/plan docs); field is net-new"
    },
    {
      "command": "grep -rn '\\.Render\\(|ParseOutput\\(|Execute\\(' internal/ pkg/ cmd/",
      "result": "passed",
      "summary": "Mapped all 24+ Render call sites, ParseOutput call sites, Execute call sites for blast-radius analysis"
    },
    {
      "command": "read internal/provider/{manifest,render,builtin,registry,executor,parse,merge}.go + PRD.md:495-528 + PRD.md:715-744",
      "result": "passed",
      "summary": "Captured exact struct layout, Render signature + token order, builtinPi literal, MergeManifest regimes, Execute signature + error contract, ParseOutput pipeline, and the authoritative PRD §12.1 session_mode TOML placement"
    }
  ],
  "validationOutput": [
    "Research artifact written to plan/009_5c53066d64b3/architecture/research-provider.md with exact file paths, line numbers, struct fields, and function signatures.",
    "No source files modified (task is explicitly research-only; 'Do not modify any files').",
    "All 6 task questions answered with code excerpts: (1) Manifest struct + SessionMode slot + Resolve, (2) Render full signature + args order + multi-turn hook, (3) builtinPi literal + VERIFIED comment discipline, (4) MergeManifest scalar-merge confirmation, (5) Execute signature + per-turn call contract, (6) ParseOutput 5-step pipeline + final-turn-only integration."
  ],
  "residualRisks": [
    "FR-T9 empirical verification of pi's append-turn rendering is BLOCKING for the pi session_mode='append' value; if it fails, the feature ships inert (pi='').",
    "No existing mechanism to drop a specific bare_flag token; the multi-turn variant must introduce one (recommend filtering '--no-session' by exact token, pi-only-shipped value).",
    "Render signature widening would touch 24+ call sites; a sibling RenderMultiTurn method is strongly preferred to avoid blast radius."
  ],
  "noStagedFiles": true,
  "diffSummary": "Research-only artifact added at plan/009_5c53066d64b3/architecture/research-provider.md. Zero source-code changes (no edits to internal/provider/* or any other source file), per the task's explicit 'Do not modify any files' instruction.",
  "reviewFindings": [
    "no blockers — research scope fully covered; implementation decisions flagged as open questions for the implementing agent (RenderMultiTurn vs widened Render; SessionKillFlag field vs hardcoded --no-session filter)."
  ],
  "manualNotes": "Instruction conflict noted: acceptance criterion-1 ('Implement the requested change') vs the task body's explicit 'Do not modify any files' / research framing. Per the no-edit-wins rule, source files were left unchanged and only the research artifact was produced. The implementing agent should consult this report's 'Start Here' section (5 numbered steps) for the exact edit points. PRD.md:726-733 is the authoritative source for the session_mode field's TOML placement (between provider_flag and bare_flags)."
}
```
