# Research: gemini + opencode built-in manifests (P1.M2.T2.S2)

> This subtask EXTENDS `internal/provider/builtin.go` + `builtin_test.go` (created by S1 with pi+claude)
> to add the **gemini** and **opencode** manifests. These are the first two of the §12.7.1
> "read-only constraint" providers (no global tool-disable switch; instead constrained to a read-only,
> never-ask profile). S3 (codex+cursor) will extend the same files again.
>
> Source of truth: PRD §12.5 (gemini), §12.6 (opencode), §12.7.1, Appendix D (h2.27), Appendix E (h2.28),
> and `architecture/external_deps.md` §gemini + §opencode (both VERIFIED live against `--help`, 2026-06-29).

---

## §1 — Field-by-field value tables

### gemini (PRD §12.5 + work-item stdin revision)

| Field | Value | Source / note |
|---|---|---|
| `Name` | `"gemini"` | identity / map key |
| `Detect` | `strPtr("gemini")` | §12.5 `detect = "gemini"` |
| `Command` | `strPtr("gemini")` | §12.5 `command = "gemini"` |
| `Subcommand` | **nil** | §12.5 has NO `subcommand` key (gemini is a flat `gemini …` command) |
| `PromptDelivery` | `strPtr("stdin")` | **REVISED** from §12.5 `"positional"` — see §3 |
| `PromptFlag` | **nil** | §12.5 has NO `prompt_flag` key |
| `PrintFlag` | `strPtr("")` | §12.5 `print_flag = ""` (explicit empty, NON-NIL) |
| `ModelFlag` | `strPtr("-m")` | §12.5 `model_flag = "-m"` |
| `DefaultModel` | `strPtr("gemini-2.5-pro")` | §12.5 `default_model = "gemini-2.5-pro"` |
| `SystemPromptFlag` | `strPtr("")` | §12.5 `system_prompt_flag = ""` (explicit empty, NON-NIL) — no sys flag → prepend per §12.2 |
| `ProviderFlag` | `strPtr("")` | §12.5 `provider_flag = ""` (explicit empty, NON-NIL) — gemini has no sub-provider |
| `DefaultProvider` | **nil** | §12.5 OMITS `default_provider` (absent) |
| `BareFlags` | `[]string{"--approval-mode", "default"}` | §12.5 `bare_flags = ["--approval-mode", "default"]` — read-only, never-ask profile |
| `Output` | `strPtr("raw")` | §12.5 `output = "raw"` |
| `JsonField` | **nil** | §12.5 has NO `json_field` key (output is raw, no JSON extraction) |
| `StripCodeFence` | `boolPtr(true)` | §12.5 `strip_code_fence = true` |
| `RetryInstruction` | **nil** | §12.5 has NO `retry_instruction` key (Resolve → DefaultRetryInstruction) |
| `Env` | **nil** | §12.5 has NO `[env]` table |

### opencode (PRD §12.6 — VERBATIM, no revisions)

| Field | Value | Source / note |
|---|---|---|
| `Name` | `"opencode"` | identity / map key |
| `Detect` | `strPtr("opencode")` | §12.6 `detect = "opencode"` |
| `Command` | `strPtr("opencode")` | §12.6 `command = "opencode"` |
| `Subcommand` | `[]string{"run"}` | §12.6 `subcommand = ["run"]` — opencode is `opencode run …` (NON-NIL 1-element slice) |
| `PromptDelivery` | `strPtr("positional")` | §12.6 `prompt_delivery = "positional"` (`opencode run [message..]`) |
| `PromptFlag` | **nil** | §12.6 has NO `prompt_flag` key |
| `PrintFlag` | `strPtr("")` | §12.6 `print_flag = ""` (explicit empty, NON-NIL) — `run` is already non-interactive |
| `ModelFlag` | `strPtr("-m")` | §12.6 `model_flag = "-m"` (format `provider/model`) |
| `DefaultModel` | `strPtr("")` | §12.6 `default_model = ""` (explicit empty, NON-NIL) — user MUST set model |
| `SystemPromptFlag` | `strPtr("")` | §12.6 `system_prompt_flag = ""` (explicit empty, NON-NIL) → prepend per §12.2 |
| `ProviderFlag` | `strPtr("")` | §12.6 `provider_flag = ""` (explicit empty, NON-NIL) — provider is part of the model string |
| `DefaultProvider` | **nil** | §12.6 OMITS `default_provider` (absent) |
| `BareFlags` | `[]string{}` | §12.6 `bare_flags = []` (NON-NIL EMPTY slice — see §4) |
| `Output` | `strPtr("raw")` | §12.6 `output = "raw"` |
| `JsonField` | **nil** | §12.6 has NO `json_field` key |
| `StripCodeFence` | `boolPtr(true)` | §12.6 `strip_code_fence = true` |
| `RetryInstruction` | **nil** | §12.6 has NO `retry_instruction` key |
| `Env` | **nil** | §12.6 has NO `[env]` table |

---

## §2 — The explicit-empty vs absent nil/non-nil pattern (THE subtlety — mirrors S1 FINDING C/D)

go-toml/v2 decodes a PRESENT key (even `""`/`""`/`[]`/`false`) to a NON-NIL value, and an ABSENT key to
`nil` (`P1.M2T1S1/research/go-toml-pointer-behavior.md` FINDING C/D). The literal construction MUST mirror
the PRD TOML's pattern EXACTLY or `reflect.DeepEqual(built-in, decode(TOML))` fails (nil ≠ non-nil).

| Provider | Field | Pattern | Construct as |
|---|---|---|---|
| gemini | `PrintFlag` | explicit `""` | `strPtr("")` (NON-NIL empty) |
| gemini | `SystemPromptFlag` | explicit `""` | `strPtr("")` (NON-NIL empty) |
| gemini | `ProviderFlag` | explicit `""` | `strPtr("")` (NON-NIL empty) |
| gemini | `DefaultProvider` | ABSENT | **nil** (do NOT set) |
| gemini | `Subcommand`/`PromptFlag`/`JsonField`/`RetryInstruction`/`Env` | ABSENT | **nil** (omit) |
| opencode | `PrintFlag` | explicit `""` | `strPtr("")` (NON-NIL empty) |
| opencode | `DefaultModel` | explicit `""` | `strPtr("")` (NON-NIL empty) — "user must set model" |
| opencode | `SystemPromptFlag` | explicit `""` | `strPtr("")` (NON-NIL empty) |
| opencode | `ProviderFlag` | explicit `""` | `strPtr("")` (NON-NIL empty) |
| opencode | `DefaultProvider` | ABSENT | **nil** (do NOT set) |
| opencode | `PromptFlag`/`JsonField`/`RetryInstruction`/`Env` | ABSENT | **nil** (omit) |

**Why this matters functionally (after Resolve):** nil and `*""` converge (Resolve fills nil → `*""`),
so the renderer treats them identically. But the DECODE-PARITY test compares the UNRESOLVED built-in to
the UNRESOLVED decode, where nil ≠ non-nil. Flattening (e.g. writing `DefaultProvider: strPtr("")` for
opencode, or omitting `PrintFlag` for gemini) breaks parity. The literal must encode the TOML's pattern.

---

## §3 — THE gemini stdin revision (the single intentional deviation from verbatim §12.5)

**The PRD §12.5 TOML writes `prompt_delivery = "positional"`. The work-item contract REQUIRES `stdin`.**

Evidence chain (all three agree):
1. **Work-item contract:** "prompt_delivery=stdin (revised from PRD's positional — see external_deps.md
   recommendation)."
2. **external_deps.md §gemini (VERIFIED):** "Recommendation: default to `stdin` (the help says 'stdin is
   appended to the prompt' and it avoids arg-length limits); positional as documented fallback."
3. **PRD §12.5 itself:** "candidates are `stdin` first, `positional` as fallback" + "should default to
   whichever is verified to handle a ~300 KB payload."
4. **PRD Appendix E item 1 (open question):** "default to stdin for gemini (avoids arg limits)."

**Consequence for the decode-parity test:** S1 used the VERBATIM PRD TOML as the decode oracle. For gemini,
the verbatim §12.5 TOML has `prompt_delivery = "positional"`, which would NOT match the built-in's `"stdin"`.
Therefore the gemini decode-parity fixture is the §12.5 TOML **with `prompt_delivery` changed to `"stdin"`**,
annotated with a comment explaining the revision. This is the honest, traceable representation: the manifest
IS §12.5 except for this one documented, externally-validated change.

**opencode has NO such revision** — §12.6 matches the work-item spec verbatim (`positional`). Its
decode-parity fixture is the verbatim §12.6 TOML.

---

## §4 — opencode `bare_flags = []` → NON-NIL EMPTY slice (FINDING D gotcha)

`P1.M2T1S1/research/go-toml-pointer-behavior.md` FINDING D: a present-but-empty TOML array `bare_flags = []`
decodes to a **NON-NIL** empty slice (`len 0`, `!= nil`); an ABSENT key decodes to `nil`. Since §12.6
WRITES `bare_flags = []`, the decode produces `[]string{}` (non-nil empty). The literal MUST be
`BareFlags: []string{}` (explicit non-nil empty), **NOT** an omitted field (which would be nil and fail
decode-parity: nil ≠ non-nil-empty).

**Merge implication (not a problem):** `MergeManifest` regime 2 treats `len(override.BareFlags) > 0` as
"replace". A non-nil empty slice (`len 0`) is therefore "not overridden" — base preserved. That is the
CORRECT behavior: opencode's "no bare flags" stays "no bare flags" unless a user provides real flags. The
non-nil-empty is purely a decode-parity / fidelity concern, not a functional one.

`Subcommand` is the opposite case: §12.6 WRITES `subcommand = ["run"]` → non-nil 1-element slice →
`Subcommand: []string{"run"}`. gemini has NO `subcommand` key → nil → omit the field.

---

## §5 — §12.2 command rendering for gemini + opencode (the argv the renderer will produce)

Porting PRD §12.2 (the authoritative algorithm; S1 already has a `renderArgs` test helper):

```
args = [command]
args += subcommand                                  # nil-safe no-op
if provider_flag != "" and provider: args += [provider_flag, provider]
if model_flag    != "" and model:    args += [model_flag, model]      # model = passed OR default
if sys_flag      != "" and sys:      args += [sys_flag, sys]
args += bare_flags
if print_flag    != "": args += [print_flag]
if prompt_delivery == "positional": args += [payload]                # payload is the positional arg
# stdin delivery: payload goes to stdin (NOT in argv); sys is prepended to payload when sys_flag == ""
```

### gemini (stdin delivery; provider="", model=default, sys prepended to payload)
```
gemini -m gemini-2.5-pro --approval-mode default            # payload "<sys>\n\n<user>" via STDIN
```
- `Command="gemini"`; no subcommand; `ProviderFlag=""` → no `--provider`; `ModelFlag="-m"` + default
  `gemini-2.5-pro`; `SystemPromptFlag=""` → sys NOT a flag (prepended to stdin payload per §12.2);
  `BareFlags=["--approval-mode","default"]`; `PrintFlag=""` → none; delivery `stdin` → no positional arg.
- Matches external_deps.md §gemini rendered block (flags only; payload on stdin). The PRD §12.5 "Rendered"
  block shows the POSITIONAL form; with the stdin revision the argv drops the positional payload and pipes
  it on stdin (same flags).

### opencode (positional delivery; model user-set = "anthropic/claude-sonnet-4", sys prepended)
```
opencode run -m anthropic/claude-sonnet-4 "<sys>\n\n<user payload>"
```
- `Command="opencode"`; `Subcommand=["run"]`; `ModelFlag="-m"` + model `anthropic/claude-sonnet-4`;
  `SystemPromptFlag=""` → prepend; `BareFlags=[]` → none; `PrintFlag=""` → none; delivery `positional` →
  payload appended as the positional arg.
- Matches PRD §12.6 "Rendered" block byte-for-byte (§12.2 and §12.6 agree here).
- **`DefaultModel=""` forces user-set model:** if renderArgs is called with `model=""`, `modelToUse=""`
  (default also empty) → `if model_flag!="" && model!=""` is FALSE → no `-m` flag. This proves the
  design intent (opencode requires the user to set a model). A render test with an explicit model shows
  the flag appears; the renderer/registry relies on config/env to supply it.

### Render-test coupling note
S1's `renderArgs` helper models **stdin** (it does NOT append the positional payload). For gemini (stdin)
the helper yields the FULL argv. For opencode (positional) the helper yields the FLAG PORTION; the test
appends the positional payload manually (`append(flags, payload)`) to assert the complete §12.6 argv. The
helper is THROWAWAY test scaffolding (NOT the P1.M2.T4 renderer); do not over-engineer it.

---

## §6 — Test strategy (EXTEND S1's `builtin_test.go`; do NOT create a new file)

S1 created `builtin_test.go` with 8 test groups + the `renderArgs` helper + `assertStr`/`assertNilStr`
helpers + the `piTOML`/`claudeTOML` constants. S2 EXTENDS that file:

**MODIFY (must update — otherwise S2 breaks S1's tests):**
- `TestBuiltinManifests_KeysAndCount`: 2 keys → **4 keys** (`pi`, `claude`, `gemini`, `opencode`).
- `TestBuiltinManifests_DecodeParity` table: add `{"gemini", builtinGemini(), geminiTOML}` and
  `{"opencode", builtinOpenCode(), opencodeTOML}` rows.
- `BuiltinManifests()` doc comment: update "§12.3 pi + §12.4 claude" → "pi + claude + gemini + opencode";
  "remaining four… S2/S3" → "remaining two (codex, cursor)… S3".

**ADD (new constants + tests):**
- `geminiTOML` constant: §12.5 verbatim **EXCEPT** `prompt_delivery = "stdin"` (§3 deviation, commented).
- `opencodeTOML` constant: §12.6 **verbatim**.
- `TestBuiltinManifests_GeminiFields`: assert every gemini field (3 explicit-empty non-nil pointers;
  absent DefaultProvider/PromptFlag/JsonField/RetryInstruction/Env/Subcommand nil; BareFlags 2 tokens).
- `TestBuiltinManifests_OpenCodeFields`: assert every opencode field (4 explicit-empty non-nil pointers;
  Subcommand `["run"]` non-nil; BareFlags `[]string{}` NON-NIL-empty; absent fields nil).
- `TestBuiltinManifests_RenderedCommand_Gemini`: `renderArgs(builtinGemini(), "", "", "<sys>")` ==
  `["gemini","-m","gemini-2.5-pro","--approval-mode","default"]` (stdin; payload not in argv).
- `TestBuiltinManifests_RenderedCommand_OpenCode`: flag portion via renderArgs + appended positional
  payload == `["opencode","run","-m","anthropic/claude-sonnet-4","<sys>\n\n<payload>"]` (§12.6 block).

**UNCHANGED (auto-cover or not needed):**
- `TestBuiltinManifests_NameMatchesKey`: iterates the whole map → auto-covers gemini/opencode.
- `TestBuiltinManifests_Validate`: iterates the whole map → auto-covers (both must `Validate()==nil`).
- `TestBuiltinManifests_PiFields` / `ClaudeFields` / `RenderedCommand_Pi` / `FreshEachCall`: untouched
  (fresh-per-call design is proven by pi; gemini/opencode use the same strPtr/slice-literal idiom).
- `assertStr` / `assertNilStr` / `renderArgs` helpers: reuse as-is (do NOT change signatures — S1's pi
  render test calls `renderArgs(builtinPi(), "zai", "", "<sys>")`).

---

## §7 — Files touched / frozen / dependencies

- **TOUCH (2):** `internal/provider/builtin.go` (add 2 constructors + extend map + update doc comment),
  `internal/provider/builtin_test.go` (add 2 TOML consts + 4 tests + update 2 tests).
- **FROZEN (do NOT edit):** `manifest.go` + `manifest_test.go` (S1 Manifest type — the CONTRACT),
  `merge.go` + `merge_test.go` (S2 merge), `internal/config/*`, `internal/git/*`, `cmd/stagecoach/main.go`,
  `Makefile`, `go.mod`, `go.sum`.
- **IMPORTS:** `builtin.go` stays ZERO-import (literal `strPtr`/`boolPtr` + slice literals only — same as
  S1). `builtin_test.go` already imports `testing`+`reflect`+`go-toml/v2`; S2 adds NOTHING new.
- **go.mod / go.sum:** UNCHANGED (no new dep; toml is test-only, already pinned). `go mod tidy` is a no-op.
- **DOWNSTREAM:** registry (P1.M2.T3) consumes `BuiltinManifests()["gemini"]`/`["opencode"]` as the merge
  base. S3 (codex+cursor) will extend the same map to 6 keys (update KeysAndCount 4→6). Design the map so
  each addition is a one-line change (it is: `return map[string]Manifest{"pi":…, "claude":…, "gemini":…,
  "opencode":…}` — S3 appends two more keys).
