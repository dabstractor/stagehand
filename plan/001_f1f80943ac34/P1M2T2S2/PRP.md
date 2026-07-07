---
name: "P1.M2.T2.S2 — gemini + opencode built-in manifests (read-only constraint / no sys-prompt flag)"
description: |
  Land the SECOND subtask of Built-in Provider Manifests (P1.M2.T2): EXTEND the compiled-in
  `internal/provider/builtin.go` to add the **gemini** and **opencode** manifests — two of the §12.7.1
  "read-only constraint" providers (no global tool-disable switch; instead constrained to a read-only,
  never-ask profile: gemini `--approval-mode default`, opencode `run` non-interactive with no bare flags).
  S1 already landed pi+claude (the "explicit tool-disable switch" pair) in this same file; S2 adds these
  two; S3 (codex+cursor) will add the last two.

  This subtask MODIFIES the two files S1 created (`builtin.go` + `builtin_test.go`). It does NOT create
  new files, does NOT touch `manifest.go`/`manifest_test.go` (S1 frozen contract) or `merge.go`/
  `merge_test.go` (S2 merge), and adds NO dependency (go.mod unchanged — `builtin.go` stays import-free).

  ⚠️ **THE central design call — THE gemini stdin revision.** The PRD §12.5 TOML writes
  `prompt_delivery = "positional"`. The work-item contract REQUIRES `prompt_delivery = "stdin"`, and this
  is backed by THREE independent sources: external_deps.md §gemini ("default to stdin … avoids arg-length
  limits"), PRD §12.5 itself ("candidates are stdin first, positional as fallback"), and Appendix E item 1
  ("default to stdin for gemini (avoids arg limits)"). Therefore `builtinGemini().PromptDelivery =
  strPtr("stdin")` — NOT "positional". Because S1's decode-parity oracle is "built-in == decode(PRD TOML)",
  the gemini decode-parity fixture is the §12.5 TOML **with `prompt_delivery` changed to `"stdin"`** and a
  comment documenting the deviation. opencode has NO such revision (§12.6 == work-item spec verbatim), so
  its fixture is the verbatim §12.6 TOML. See research §3.

  ⚠️ **THE second design call — reproduce the TOML's nil/non-nil pattern EXACTLY (explicit-empty vs
  absent), same discipline as S1.** go-toml/v2 decodes `x = ""` → non-nil `*""` and an ABSENT key → `nil`
  (S1 FINDING C/D). Both gemini and opencode TOMLs mix explicit-empty keys and omitted keys; the literal
  construction MUST match (or `reflect.DeepEqual(built-in, decode(TOML))` fails — nil ≠ non-nil):
    • gemini: `PrintFlag`/`SystemPromptFlag`/`ProviderFlag` = `strPtr("")` (NON-NIL empty — §12.5 writes
      them `""`); `DefaultProvider` = nil (§12.5 OMITS the key); `Subcommand`/`PromptFlag`/`JsonField`/
      `RetryInstruction`/`Env` = nil (absent).
    • opencode: `PrintFlag`/`DefaultModel`/`SystemPromptFlag`/`ProviderFlag` = `strPtr("")` (NON-NIL
      empty); `DefaultProvider` = nil (absent); `PromptFlag`/`JsonField`/`RetryInstruction`/`Env` = nil;
      `Subcommand` = `[]string{"run"}` (NON-NIL — §12.6 writes `subcommand = ["run"]`). See research §2.

  ⚠️ **THE third design call — opencode `BareFlags` is a NON-NIL EMPTY slice `[]string{}`, NOT nil.**
  §12.6 writes `bare_flags = []`. Per FINDING D, a present-but-empty TOML array decodes to a NON-NIL empty
  slice (`len 0`, `!= nil`). So opencode's literal MUST be `BareFlags: []string{}` (explicit non-nil
  empty). An implementer who OMITS the field (→ nil) will FAIL decode-parity. This is purely a fidelity
  concern (the renderer's `append(args, BareFlags...)` is a no-op for both nil and empty), but the
  decode-parity test treats nil ≠ non-nil-empty. See research §4.

  ⚠️ **THE fourth design call — both providers PREPEND the system prompt (no sys-prompt flag).** Neither
  gemini-cli nor opencode `run` exposes a `--system-prompt` flag (external_deps.md §gemini/§opencode
  VERIFIED). Both set `SystemPromptFlag = strPtr("")` (explicit empty, NON-NIL — matching the TOML). The
  §12.2 renderer's `if sys_flag != "" and sys` is therefore FALSE → the sys prompt is NOT a flag; per
  §12.2 it is PREPENDED to the payload (`"<sys>\n\n<user payload>"`). This is the "no sys-prompt flag →
  prepend" branch the §12.1 schema exists to express. (opencode's richer `--agent` persona workflow is a
  documented v1.1 follow-on — Appendix E item 3 — NOT this subtask.)

  ⚠️ **THE fifth design call — EXTEND the existing files (S1's), do NOT create new ones; update the two
  S1 tests that would otherwise break.** S1 created `builtin.go` + `builtin_test.go` with pi+claude.
  S2 adds `builtinGemini()`/`builtinOpenCode()` constructors, extends `BuiltinManifests()`'s map
  (2→4 keys), and EXTENDS the test file: (a) UPDATE `TestBuiltinManifests_KeysAndCount` (2→4 keys — else
  it fails), (b) UPDATE `TestBuiltinManifests_DecodeParity` table (+gemini/+opencode rows), (c) ADD
  `geminiTOML`/`opencodeTOML` constants + 4 new tests (GeminiFields, OpenCodeFields, RenderedCommand_Gemini,
  RenderedCommand_OpenCode). `NameMatchesKey` + `Validate` iterate the whole map → auto-cover the new
  providers (no edit). The `renderArgs`/`assertStr`/`assertNilStr` helpers are REUSED unchanged (S1's pi
  render test calls `renderArgs(builtinPi(), "zai", "", "<sys>")` — do NOT change its signature). S3 will
  extend the same map to 6 keys (update KeysAndCount 4→6). See research §6.

  Deliverable: MODIFIED `internal/provider/builtin.go` (`package provider`, ZERO imports) —
  `BuiltinManifests()` now returning `{"pi","claude","gemini","opencode"}` + the two new unexported
  constructors; MODIFIED `internal/provider/builtin_test.go` — the 2 updated tests + 4 new tests, all
  passing. INPUT = S1's `Manifest` + `strPtr`/`boolPtr` (frozen in `manifest.go`). OUTPUT = the 4 built-in
  manifests the registry (P1.M2.T3) consumes; gemini/opencode argv the renderer/executor/parser will run.
---

## Goal

**Feature Goal**: Add the gemini and opencode provider manifests to the compiled-in defaults so
`BuiltinManifests() map[string]Manifest` returns FOUR providers (pi, claude, gemini, opencode), every field
matching PRD §12.5 (gemini, with the one mandated `prompt_delivery`="stdin" revision) and §12.6 (opencode,
verbatim) exactly — nil/non-nil pattern included — and each manifest `Validate()`ing clean.

**Deliverable**:
1. **MODIFY** `internal/provider/builtin.go` (`package provider`, **ZERO imports**):
   (a) `func BuiltinManifests() map[string]Manifest` now returns
       `{"pi": builtinPi(), "claude": builtinClaude(), "gemini": builtinGemini(), "opencode": builtinOpenCode()}`
       (fresh construction each call — S1 design call #4, unchanged).
   (b) **ADD** unexported `func builtinGemini() Manifest` — every field per the gemini table below, built
       with S1's `strPtr`/`boolPtr`; `PromptDelivery=strPtr("stdin")` (**REVISED** from §12.5 positional);
       `PrintFlag`/`SystemPromptFlag`/`ProviderFlag`=strPtr("") (non-nil empty); `DefaultProvider`/`Subcommand`/
       `PromptFlag`/`JsonField`/`RetryInstruction`/`Env` nil (absent in §12.5).
   (c) **ADD** unexported `func builtinOpenCode() Manifest` — every field per the opencode table;
       `Subcommand=[]string{"run"}`; `PrintFlag`/`DefaultModel`/`SystemPromptFlag`/`ProviderFlag`=strPtr("");
       `BareFlags=[]string{}` (NON-NIL empty — §4); `DefaultProvider`/`PromptFlag`/`JsonField`/
       `RetryInstruction`/`Env` nil (absent in §12.6).
   (d) UPDATE the `BuiltinManifests` doc comment: "§12.3 pi + §12.4 claude" → "pi + claude + gemini +
       opencode"; "remaining four … S2/S3" → "remaining two (codex, cursor) … S3".
2. **MODIFY** `internal/provider/builtin_test.go` (`package provider`, white-box; S1's imports
   `testing`+`reflect`+`go-toml/v2` unchanged) — the 2 updated tests + 4 new tests (see Implementation
   Tasks), all passing.

No other files touched. **No go.mod/go.sum change** (`builtin.go` has zero imports). NO edit to
`manifest.go`/`manifest_test.go` (S1) or `merge.go`/`merge_test.go` (S2). No registry (P1.M2.T3), no
renderer/executor/parser (P1.M2.T4/T5/T6), no codex/cursor (S3 of this task), no `providers/*.toml` files
(P1.M5.T2).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/` clean; `go mod tidy`
is a no-op; `go test -race ./internal/provider/ -v` passes (S1's pi/claude tests STILL green + the 2 updated
+ 4 new gemini/opencode tests green) and `go test -race ./...` stays green; the gemini + opencode manifests
match the tables below exactly (incl. the explicit-empty vs absent nil/non-nil pattern and opencode's
non-nil-empty BareFlags); `reflect.DeepEqual(builtinGemini(), decode(geminiTOML))` and the opencode
equivalent both hold (geminiTOML = §12.5 with the documented stdin revision; opencodeTOML = verbatim §12.6);
both `Validate()` → nil.

## User Persona

**Target User**: The registry (P1.M2.T3) — it calls `BuiltinManifests()` to fetch the compiled-in defaults,
then `MergeManifest(builtin, userOverride)` (S2 merge) to overlay any `[provider.gemini]`/`[provider.opencode]`
config, then `Validate()` + `Resolve()` before handing the manifest to the renderer/executor/parser.
Transitively every user story routed through "call an agent" (US) and FR36/FR37 (provider management).

**Use Case**: A user runs `stagecoach` with zero config and has `gemini` (or `opencode`) installed. The
registry has no `[provider.*]` override, so `BuiltinManifests()["gemini"]` IS the resolved gemini manifest;
the renderer turns it into `gemini -m gemini-2.5-pro --approval-mode default` (payload piped to stdin);
the executor runs it; the parser cleans stdout. This subtask is what makes "zero config" work for the
read-only-constrained agents.

**User Journey**: (internal API, no end-user surface yet) `BuiltinManifests()` (THIS subtask adds 2 entries)
→ registry selects `gemini`/`opencode` (or merges a user override via S2) → `Validate()` → `Resolve()` →
renderer builds argv per §12.2 → executor runs → parser cleans.

**Pain Points Addressed**: Removes "what are the exact default flags for gemini/opencode / is the stdin-vs-
positional decision made / does the built-in match the PRD TOML" ambiguity by landing two literal,
decode-parity-tested manifests now. The gemini stdin decision is made once, here, with a traceable rationale.

## Why

- **Zero config works because of this.** PRD §12.1: "Built-in manifests are compiled into the binary (so
  the tool works with zero config)." gemini and opencode are two of the §12.7.1 "read-only constraint"
  providers — they have no global tool-disable switch, but `--approval-mode default` (gemini) /
  non-interactive `run` with no flags (opencode) constrains them to a read-only, never-ask profile. Landing
  them now lets the registry + renderer be built/tested against all three provider categories (explicit
  switch: pi/claude; read-only constraint: gemini/opencode/codex/cursor).
- **The gemini stdin call is made ONCE, here, traceably.** PRD §12.5 + Appendix E #1 left stdin-vs-positional
  as an open question. The work-item contract + external_deps.md resolve it: stdin (avoids arg-length
  limits on ~300 KB diffs; gemini appends stdin to the prompt). Encoding it now + pinning it in a
  decode-parity test with a documented deviation comment means no future agent re-litigates it.
- **Unlocks the registry + renderer for more targets.** P1.M2.T3 imports `BuiltinManifests()`; P1.M2.T4
  renders one. Adding gemini (stdin) + opencode (positional) gives the renderer a SECOND delivery mode to
  build against (S1's pi/claude were both stdin), exercising the §12.2 positional branch.
- **Proves the prepend branch end-to-end.** Both providers have `SystemPromptFlag=""` → the §12.2 renderer
  prepends the sys prompt to the payload instead of emitting a flag. This is the "no sys-prompt flag"
  branch; landing two such manifests validates that the schema expresses it cleanly.
- **No user-facing surface change** (PRD "DOCS: none — compiled-in defaults"). `providers show`
  (P1.M4.T1.S3) and the reference `providers/*.toml` files (P1.M5.T2) are where users SEE these later.
- **No new dependency, no new import edge.** `builtin.go` stays import-free (literal construction); the
  package's production code stays stdlib-only (S1/S2 discipline); go.mod is unchanged.

## What

A compiled `internal/provider` package exporting `BuiltinManifests() map[string]Manifest` now returning
FOUR literal manifests (pi, claude from S1; gemini, opencode from THIS subtask), each constructed fresh per
call, decode-parity-verified against its PRD TOML (gemini with the documented stdin revision; opencode
verbatim), both `Validate()`ing clean. No registry, no rendering, no execution, no parsing, no codex/cursor.

### Success Criteria

- [ ] `BuiltinManifests()` returns EXACTLY 4 keys: `pi`, `claude`, `gemini`, `opencode` (no more, no less —
      codex/cursor are S3). Each returned manifest's `.Name` equals its map key.
- [ ] `builtinGemini()` sets every field per the gemini table (below) with `strPtr`/`boolPtr`: `Name="gemini"`,
      `Detect`/`Command`="gemini", `PromptDelivery="stdin"` (**REVISED**), `PrintFlag`/`SystemPromptFlag`/
      `ProviderFlag`=strPtr("") (**non-nil empty**), `ModelFlag="-m"`, `DefaultModel="gemini-2.5-pro"`,
      `BareFlags=["--approval-mode","default"]` (2 tokens), `Output="raw"`, `StripCodeFence=true`; AND leaves
      `Subcommand`/`PromptFlag`/`DefaultProvider`/`JsonField`/`RetryInstruction`/`Env` **nil** (absent in §12.5).
- [ ] `builtinOpenCode()` sets every field per the opencode table: `Name="opencode"`, `Detect`/`Command`=
      "opencode", `Subcommand=["run"]` (**non-nil 1-element**), `PromptDelivery="positional"`,
      `PrintFlag`/`DefaultModel`/`SystemPromptFlag`/`ProviderFlag`=strPtr("") (**non-nil empty**),
      `ModelFlag="-m"`, `BareFlags=[]string{}` (**NON-NIL empty** — §4), `Output="raw"`,
      `StripCodeFence=true`; AND leaves `PromptFlag`/`DefaultProvider`/`JsonField`/`RetryInstruction`/`Env`
      **nil** (absent in §12.6).
- [ ] Both `builtinGemini().Validate()` and `builtinOpenCode().Validate()` return nil.
- [ ] `reflect.DeepEqual(builtinGemini(), decode(geminiTOML))` AND `reflect.DeepEqual(builtinOpenCode(),
      decode(opencodeTOML))` both hold. `geminiTOML` = §12.5 verbatim EXCEPT `prompt_delivery = "stdin"`
      (the documented revision); `opencodeTOML` = verbatim §12.6 (incl. `bare_flags = []`).
- [ ] `TestBuiltinManifests_KeysAndCount` updated to expect 4 keys; `TestBuiltinManifests_DecodeParity`
      table extended with gemini + opencode rows.
- [ ] New tests pass: `TestBuiltinManifests_GeminiFields`, `TestBuiltinManifests_OpenCodeFields`,
      `TestBuiltinManifests_RenderedCommand_Gemini` (== `["gemini","-m","gemini-2.5-pro","--approval-mode",
      "default"]` — stdin, payload not in argv), `TestBuiltinManifests_RenderedCommand_OpenCode` (==
      `["opencode","run","-m","anthropic/claude-sonnet-4","<sys>\n\n<payload>"]` — positional).
- [ ] S1's tests STILL pass unchanged: `PiFields`, `ClaudeFields`, `RenderedCommand_Pi_MatchesCommitPi`,
      `FreshEachCall`, `NameMatchesKey`, `Validate` (the latter two now also cover gemini/opencode via the
      larger map). The `renderArgs`/`assertStr`/`assertNilStr` helpers are UNCHANGED (signature-preserving).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` (diff-empty) clean;
      `go test -race ./internal/provider/` AND `go test -race ./...` green.
- [ ] go.mod/go.sum byte-unchanged; `manifest.go`/`manifest_test.go` (S1) + `merge.go`/`merge_test.go` (S2)
      byte-unchanged; every file outside the two modified `builtin*.go` files byte-unchanged; `builtin.go`
      STILL has ZERO imports.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the two field tables
(gemini/opencode, with the explicit-empty vs absent map), the `strPtr`/`boolPtr` construction idiom (from
S1's `manifest.go`), the decode-parity test approach + the verbatim TOML strings (provided below), the
§12.2 render algorithm (S1's `renderArgs` helper, reused) + the exact expected argv, and the test specs.
The one subtlety (gemini stdin revision) is documented inline in both the literal and the test TOML.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- file: internal/provider/builtin.go   (S1 — ALREADY EXISTS with pi+claude; you MODIFY it)
  why: the file you are EXTENDING. Read S1's builtinPi()/builtinClaude() to mirror their construction
       style (doc comment citing the PRD section + the explicit-empty notes; strPtr/boolPtr field
       assignment; absent fields omitted with a trailing comment). You ADD builtinGemini()/builtinOpenCode()
       and extend the BuiltinManifests() map (one line per new entry) + update its doc comment.
  pattern: copy S1's constructor structure EXACTLY. The new constructors are FREE FUNCTIONS (no receiver):
       `func builtinGemini() Manifest` / `func builtinOpenCode() Manifest`.
  critical: do NOT add imports (the file stays import-free). do NOT call Validate/Resolve inside the
       constructors (they BUILD; the registry validates). do NOT add a package-level var (fresh per call).

- file: internal/provider/manifest.go   (S1 — ALREADY EXISTS, COMPLETE; read it, do NOT edit it)
  why: the EXACT Manifest type + field names/tags the constructors build, AND the unexported helpers
       `strPtr(string) *string` / `boolPtr(bool) *bool` (same package — use them directly, no import).
       Also `Validate()` (both new built-ins must pass it) + `Resolve()` + the Default* constants. Confirm
       field names match exactly: Name, Detect, Command, Subcommand ([]string — PLAIN slice), PromptDelivery,
       PromptFlag, PrintFlag, ModelFlag, DefaultModel, SystemPromptFlag, ProviderFlag, DefaultProvider,
       BareFlags ([]string — PLAIN slice), Output, JsonField, StripCodeFence, RetryInstruction, Env.
  critical: Subcommand and BareFlags are PLAIN []string (not pointers) — a non-nil empty slice is a
       distinct value from nil (this is why opencode.BareFlags must be `[]string{}`, see §4). do NOT edit.

- file: internal/provider/builtin_test.go   (S1 — ALREADY EXISTS with 8 tests; you MODIFY it)
  why: the file you are EXTENDING. Reuse S1's `assertStr`/`assertNilStr`/`renderArgs` helpers (do NOT
       change their signatures — S1's pi render test depends on `renderArgs(m, provider, model, sys)`).
       Mirror the existing test style (table-driven DecodeParity; per-provider Fields tests). You ADD the
       geminiTOML/opencodeTOML constants + 4 new tests, and UPDATE KeysAndCount (2→4) + DecodeParity
       (+2 rows).
  critical: the geminiTOML fixture is §12.5 with `prompt_delivery = "stdin"` (NOT "positional") — see §3.
       If you paste the verbatim §12.5 TOML, the gemini decode-parity test WILL FAIL (positional≠stdin).
       opencodeTOML is verbatim §12.6.

- docfile: plan/001_f1f80943ac34/P1M2T2S2/research/gemini-opencode-manifests.md
  why: the field-by-field value tables (§1), the explicit-empty vs absent map (§2 — THE subtlety), the
       gemini stdin revision rationale + its 3-source evidence chain (§3), the opencode non-nil-empty
       BareFlags gotcha (§4), the §12.2 render walkthrough for both argvs (§5), the test strategy (§6).
       The single most important read.
  critical: §3 (gemini stdin — the one intentional deviation) and §4 (opencode BareFlags=[]string{} non-nil
       empty) are the two things most likely to be implemented wrong.

- file: PRD.md
  section: "12.5 Built-in provider: Gemini CLI" (h3.41) — the AUTHORITATIVE gemini manifest TOML. The TOML
       block is the decode-parity base, BUT `prompt_delivery` is REVISED to "stdin" in the fixture (§3).
  why: every gemini field value comes from here. Note `print_flag = ""`, `system_prompt_flag = ""`,
       `provider_flag = ""` are WRITTEN (explicit empty → non-nil) and there is NO `subcommand`/
       `prompt_flag`/`default_provider`/`json_field`/`retry_instruction`/`[env]` key (→ nil).
  critical: §12.5's "Rendered" block shows the POSITIONAL form; with the stdin revision the argv drops the
       positional payload (piped to stdin instead). The RenderedCommand_Gemini test asserts the stdin argv.

- file: PRD.md
  section: "12.6 Built-in provider: opencode" (h3.42) — the AUTHORITATIVE opencode manifest TOML (VERBATIM,
       no revision). The TOML block IS the decode-parity fixture.
  why: every opencode field value comes from here. Note `subcommand = ["run"]` (→ []string{"run"}), the four
       explicit-empty scalars (`print_flag`/`default_model`/`system_prompt_flag`/`provider_flag` = ""),
       `bare_flags = []` (→ []string{} NON-NIL empty), and NO `default_provider`/`prompt_flag`/`json_field`/
       `retry_instruction`/`[env]` key (→ nil).
  critical: `default_model = ""` is intentional (opencode requires user-set model — Appendix E #3); do NOT
       "helpfully" set a default. `bare_flags = []` is a NON-NIL empty slice (FINDING D); do NOT omit it.

- file: PRD.md
  section: "12.2 Command rendering algorithm" (h3.38) — the AUTHORITATIVE argv algorithm. S1 already ported
       it into the `renderArgs` test helper (REUSED unchanged here). The §12.2 positional branch
       (`args += [payload]` when delivery=="positional") is new to S2 (S1's pi/claude were stdin).
  critical: for stdin delivery the payload is NOT in argv (piped); sys is prepended to the payload when
       sys_flag=="". For positional delivery the payload IS the trailing positional arg.

- file: PRD.md
  section: "12.7.1 The tools-disable asymmetry" (h4.0) — the conceptual framing: gemini+opencode are
       "read-only constraint" providers (no global disable switch; constrained so they cannot mutate the
       repo or block on a prompt, but the model may still internally reason with tools). This is WHY this
       subtask groups gemini+opencode together (the work-item title).
  why: explains the design intent — distinct from S1's "explicit tool-disable switch" pair (pi/claude).

- file: plan/001_f1f80943ac34/architecture/external_deps.md
  section: §gemini (VERIFIED) + §opencode (VERIFIED) — live `--help` captures (2026-06-29).
  why: independent confirmation. §gemini: positional `query` default, `-p/--prompt` DEPRECATED,
       `--approval-mode` choices (default|auto_edit|yolo), NO system-prompt flag, RECOMMENDS stdin
       ("avoids arg-length limits"). §opencode: `run [message..]`, `-m/--model` (provider/model format),
       `--agent <name>` (v1.1 follow-on), NO system-prompt flag, no single default model.
  critical: §codex flags a discrepancy (--ask-for-approval) — that is an S3 concern, NOT this subtask.
       This subtask encodes ONLY gemini + opencode, both fully verified with no discrepancies.

- file: PRD.md
  section: "Appendix D — Built-in manifest quick reference" (h2.27) — the cross-provider table. gemini row
       (delivery positional/stdin, model flag -m, sys-prompt *(prepend)*, bare `--approval-mode default`,
       output raw); opencode row (command `opencode run`, delivery positional, model flag -m provider/model,
       sys-prompt *(prepend)*, bare —, output raw). Use as a final cross-check.
  section: "Appendix E — Open questions" (h2.28) — item 1 (gemini delivery → stdin) and item 3 (opencode
       sys-prompt → prepend for v1) are resolved by THIS subtask's design calls (#1 and #4).

- file: plan/001_f1f80943ac34/P1M2T1S1/research/go-toml-pointer-behavior.md
  why: FINDING C/D — absent key → nil; present key (even `""`/`[]`/`false`) → non-nil. This is WHY the
       literal must reproduce the TOML's nil/non-nil pattern exactly (design call #2) and why opencode's
       `bare_flags = []` → NON-NIL empty slice (design call #3, FINDING D).
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; go-toml/v2 v2.4.2 + pflag  (UNCHANGED by this subtask)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — FROZEN, do NOT touch; do NOT import from provider
  git/                          # P1.M1.T2/T3 — untouched
  provider/                     # S1 created the package; S2(merge) added merge.go; S1(builtin) added builtin.go
    manifest.go                 # S1 — Manifest + Validate + DetectCommand + Resolve + strPtr/boolPtr  (CONTRACT — do NOT edit)
    manifest_test.go            # S1 — tests  (do NOT edit)
    merge.go                    # S2(merge) — MergeManifest  (do NOT edit)
    merge_test.go               # S2(merge) — tests  (do NOT edit)
    builtin.go                  # S1(builtin) created this — pi + claude. THIS subtask MODIFIES it (+gemini +opencode)
    builtin_test.go             # S1(builtin) created this — 8 tests. THIS subtask MODIFIES it (+4 tests, update 2)
cmd/stagecoach/main.go           # `package main; func main(){}` stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be modified

```bash
internal/
  provider/
    builtin.go                  # MODIFIED — BuiltinManifests() now returns 4 keys + builtinGemini() + builtinOpenCode() (still ZERO imports)
    builtin_test.go             # MODIFIED — geminiTOML/opencodeTOML + 4 new tests + KeysAndCount(2→4) + DecodeParity(+2)
# manifest.go/manifest_test.go (S1) + merge.go/merge_test.go (S2 merge) UNCHANGED. go.mod/go.sum UNCHANGED.
# Every other file UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call #1 — THE gemini stdin revision): gemini.PromptDelivery MUST be strPtr("stdin"),
// NOT strPtr("positional"). The PRD §12.5 TOML writes "positional" but the work-item contract + external_deps.md
// §gemini + Appendix E #1 all mandate stdin (avoids arg-length limits on ~300 KB diffs). The decode-parity
// fixture (geminiTOML) is §12.5 with this ONE line changed to "stdin" + a comment. If you paste verbatim
// §12.5, the gemini DecodeParity test FAILS. opencode.PromptDelivery = strPtr("positional") (no revision).

// CRITICAL (design call #2 — explicit empty vs absent): go-toml/v2 decodes `x = ""` → non-nil *"" and an
// ABSENT key → nil (S1 FINDING C/D). The literal MUST mirror the PRD TOML's pattern or DecodeParity fails:
//   gemini.PrintFlag/SystemPromptFlag/ProviderFlag  = strPtr("")   // §12.5 WRITES them "" → non-nil empty
//   gemini.DefaultProvider                          = nil          // §12.5 OMITS the key → nil (do NOT set)
//   opencode.PrintFlag/DefaultModel/SystemPromptFlag/ProviderFlag = strPtr("")  // §12.6 WRITES them "" → non-nil
//   opencode.DefaultProvider                        = nil          // §12.6 OMITS the key → nil
//   both.Subcommand(gemini)/PromptFlag/JsonField/RetryInstruction/Env = nil  // absent in the TOML
// Resolve() turns nil optionals into *""/defaults at consume time, so nil≈*"" AFTER Resolve — but the
// DecodeParity test compares the UNRESOLVED built-in to the UNRESOLVED decode, where nil ≠ non-nil.

// CRITICAL (design call #3 — opencode BareFlags is a NON-NIL EMPTY slice): opencode.BareFlags MUST be
// []string{}, NOT nil. §12.6 writes `bare_flags = []`; FINDING D says a present empty array decodes to a
// NON-NIL empty slice. Omitting the field (→ nil) FAILS decode-parity (nil ≠ non-nil-empty). This is a
// fidelity concern only — the renderer's append(args, BareFlags...) is a no-op for both. Write it explicitly.
//   opencode.Subcommand = []string{"run"}   // §12.6 writes subcommand = ["run"] → NON-NIL 1-element
//   gemini.Subcommand is OMITTED (nil)      // §12.6 has NO subcommand key

// CRITICAL (design call #4 — both PREPEND the system prompt): both set SystemPromptFlag = strPtr("")
// (explicit empty, NON-NIL — matching the TOML). The §12.2 renderer's `if sys_flag != "" and sys` is
// therefore FALSE → the sys prompt is NOT a flag; per §12.2 it is PREPENDED to the payload. Neither CLI
// exposes a --system-prompt flag (external_deps.md VERIFIED). Do NOT invent a flag value.

// CRITICAL (design call #5 — EXTEND, don't recreate; preserve S1's helper signatures): this subtask
// MODIFIES builtin.go + builtin_test.go (S1 created them with pi+claude). The renderArgs helper signature
// is `renderArgs(m Manifest, provider, model, sys string) []string` — S1's pi render test depends on it;
// do NOT add a payload param. For the opencode positional render test, append the payload manually:
//   flags := renderArgs(builtinOpenCode(), "", "anthropic/claude-sonnet-4", "")
//   argv := append(flags, "<sys>\n\n<payload>")
// KeysAndCount MUST be updated 2→4 (else it fails once the map has 4 entries). DecodeParity table +2 rows.

// GOTCHA: do NOT call Validate/Resolve inside the constructors. The constructors BUILD; the registry
// (P1.M2.T3) runs Validate → Resolve on the (merged) result. Tests assert both new built-ins Validate(),
// but the constructors stay pure data.

// GOTCHA: the DecodeParity test uses reflect.DeepEqual on Manifest. nil pointers compare equal ONLY to
// nil; nil slices compare equal ONLY to nil; a non-nil empty slice compares UNEQUAL to nil. So the test is
// exactly the right oracle for "built-in matches the decoded TOML" — it catches any nil/non-nil mismatch
// (the gemini stdin value, the opencode empty-slice, the explicit-empty scalars).

// GOTCHA: opencode.DefaultModel = strPtr("") (NON-NIL empty). This is INTENTIONAL ("user must set model"
// — Appendix E #3). In renderArgs, if model is "" AND default is "", modelToUse stays "" → no -m flag
// emitted (proving the design). The RenderedCommand_OpenCode test passes an explicit model so the flag
// appears and the argv matches §12.6.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/provider/builtin.go — ADD these two constructors; EXTEND BuiltinManifests()'s map.
package provider
// (NO imports — literal construction via same-package strPtr/boolPtr only. UNCHANGED from S1.)

// ... S1's builtinPi() / builtinClaude() UNCHANGED above ...

// builtinGemini returns the gemini manifest per PRD §12.5 (VERIFIED vs `gemini --help`, external_deps.md
// §gemini), with prompt_delivery REVISED to "stdin" per the work-item contract (external_deps.md §gemini
// recommendation + Appendix E #1: stdin avoids arg-length limits on ~300 KB diffs; gemini appends stdin
// to the prompt). gemini has no global tool-disable switch; `--approval-mode default` constrains it to a
// read-only, never-ask profile (§12.7.1 "read-only constraint").
//
// NOTE: (1) PrintFlag/SystemPromptFlag/ProviderFlag are strPtr("") — §12.5 WRITES them "" (NON-NIL empty):
// no print flag (positional/stdin implies one-shot), no sys-prompt flag (sys PREPENDED to the payload per
// §12.2), no sub-provider. (2) DefaultProvider is NIL — §12.5 OMITS the key (do NOT set it). (3) The sys
// prompt is prepended to the payload (no --system-prompt flag exists on gemini-cli).
func builtinGemini() Manifest {
	return Manifest{
		Name:             "gemini",
		Detect:           strPtr("gemini"),
		Command:          strPtr("gemini"),
		PromptDelivery:   strPtr("stdin"), // REVISED from §12.5 "positional" (work-item + external_deps.md + Appx E #1)
		PrintFlag:        strPtr(""),      // §12.5 explicit empty (NON-NIL) — positional/stdin implies one-shot
		ModelFlag:        strPtr("-m"),
		DefaultModel:     strPtr("gemini-2.5-pro"),
		SystemPromptFlag: strPtr(""), // §12.5 explicit empty (NON-NIL) — no sys flag; sys prepended to payload (§12.2)
		ProviderFlag:     strPtr(""), // §12.5 explicit empty (NON-NIL) — gemini has no sub-provider
		BareFlags: []string{
			"--approval-mode", "default", // read-only, never-ask profile (don't auto-run tools)
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// Subcommand, PromptFlag, DefaultProvider, JsonField, RetryInstruction, Env: nil (absent in §12.5).
	}
}

// builtinOpenCode returns the opencode manifest per PRD §12.6 (VERIFIED vs `opencode run --help`,
// external_deps.md §opencode), VERBATIM (no revisions). opencode `run` is non-interactive and prints the
// final message to stdout. It has no global tool-disable switch and no bare flags — `run` is already a
// read-only, non-interactive one-shot (§12.7.1 "read-only constraint").
//
// NOTE: (1) Subcommand = ["run"] (§12.6 writes subcommand = ["run"] → NON-NIL 1-element). (2)
// PrintFlag/DefaultModel/SystemPromptFlag/ProviderFlag are strPtr("") — §12.6 WRITES them "" (NON-NIL
// empty): `run` is already non-interactive (no print flag), user MUST set model (no single sensible
// default — model space is huge), no sys-prompt flag (sys prepended), provider is part of the model string.
// (3) BareFlags = []string{} — §12.6 writes bare_flags = []; a present empty array decodes NON-NIL empty
// (FINDING D). (4) DefaultProvider is NIL — §12.6 OMITS the key.
func builtinOpenCode() Manifest {
	return Manifest{
		Name:             "opencode",
		Detect:           strPtr("opencode"),
		Command:          strPtr("opencode"),
		Subcommand:       []string{"run"}, // §12.6 `subcommand = ["run"]` → NON-NIL 1-element slice
		PromptDelivery:   strPtr("positional"),
		PrintFlag:        strPtr(""), // §12.6 explicit empty (NON-NIL) — `run` is already non-interactive
		ModelFlag:        strPtr("-m"),
		DefaultModel:     strPtr(""), // §12.6 explicit empty (NON-NIL) — user MUST set model (Appx E #3)
		SystemPromptFlag: strPtr(""), // §12.6 explicit empty (NON-NIL) — no sys flag on `run`; sys prepended (§12.2)
		ProviderFlag:     strPtr(""), // §12.6 explicit empty (NON-NIL) — provider is part of the model string
		BareFlags:        []string{}, // §12.6 `bare_flags = []` → NON-NIL empty slice (FINDING D); do NOT omit
		Output:           strPtr("raw"),
		StripCodeFence:   boolPtr(true),
		// PromptFlag, DefaultProvider, JsonField, RetryInstruction, Env: nil (absent in §12.6).
	}
}
```

**And update `BuiltinManifests()`** (extend the map + doc comment):

```go
// BuiltinManifests returns the compiled-in default provider manifests (PRD §12.3 pi, §12.4 claude,
// §12.5 gemini, §12.6 opencode), keyed by manifest name. These are the zero-config defaults a user
// override (config [provider.<name>]) merges onto via MergeManifest (S2) in the registry (P1.M2.T3).
//
// The manifests are constructed FRESH on every call (no package-level var): strPtr/boolPtr allocate new
// pointers and slice literals allocate new backing arrays each call, so no caller can corrupt a built-in
// by mutating a returned BareFlags/Env. MergeManifest (S2) already never mutates base, so the normal
// registry path is safe either way — fresh-per-call additionally guards against any direct mutation.
//
// This subtask adds gemini + opencode (§12.7.1 "read-only constraint" providers: no global tool-disable
// switch; constrained to read-only, never-ask profiles). pi + claude (the "explicit tool-disable switch"
// pair) landed in S1. The remaining two (codex, cursor) are added by S3.
func BuiltinManifests() map[string]Manifest {
	return map[string]Manifest{
		"pi":       builtinPi(),
		"claude":   builtinClaude(),
		"gemini":   builtinGemini(),
		"opencode": builtinOpenCode(),
	}
}
```

> **gofmt note:** run `gofmt -w internal/provider/builtin.go internal/provider/builtin_test.go`. Do not
> hand-align (gofmt will align the map keys). One doc comment per function (citing the PRD section + the
> explicit-empty notes + the gemini stdin revision) is required — it seeds `providers show` / reference-file
> docs later.
>
> **Imports:** `builtin.go` has NONE (unchanged from S1). `builtin_test.go` imports are UNCHANGED
> (`testing` + `reflect` + `github.com/pelletier/go-toml/v2` — already there from S1; S2 adds nothing).

### The decode-parity TOML fixtures (ADD to builtin_test.go)

```go
// geminiTOML — PRD §12.5 VERBATIM EXCEPT prompt_delivery is REVISED to "stdin" (the work-item contract;
// external_deps.md §gemini recommendation; Appendix E #1: stdin avoids arg-length limits on ~300 KB diffs).
// This is the ONE intentional deviation from the verbatim PRD TOML; decoding it must match builtinGemini().
const geminiTOML = `name = "gemini"
detect = "gemini"
command = "gemini"
prompt_delivery = "stdin"   # REVISED from §12.5 "positional" (work-item + external_deps.md + Appx E #1)
print_flag = ""
model_flag = "-m"
default_model = "gemini-2.5-pro"
system_prompt_flag = ""
provider_flag = ""
bare_flags = [
  "--approval-mode", "default",
]
output = "raw"
strip_code_fence = true
`

// opencodeTOML — PRD §12.6 VERBATIM (no revision). Decoding it must match builtinOpenCode().
// Note bare_flags = [] decodes to a NON-NIL empty slice (FINDING D) — builtinOpenCode sets []string{}.
const opencodeTOML = `name = "opencode"
detect = "opencode"
command = "opencode"
subcommand = ["run"]
prompt_delivery = "positional"
print_flag = ""
model_flag = "-m"
default_model = ""
system_prompt_flag = ""
provider_flag = ""
bare_flags = []
output = "raw"
strip_code_fence = true
`
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/provider/builtin.go — add builtinGemini + builtinOpenCode + extend the map
  - ADD the two constructors per the Data Models block. Use S1's strPtr/boolPtr (same package).
  - gemini: per the gemini table; PromptDelivery=strPtr("stdin") (REVISED); PrintFlag/SystemPromptFlag/
      ProviderFlag=strPtr("") (non-nil empty); DefaultProvider/Subcommand/PromptFlag/JsonField/
      RetryInstruction/Env nil (omit); BareFlags=["--approval-mode","default"].
  - opencode: per the opencode table; Subcommand=[]string{"run"}; PrintFlag/DefaultModel/SystemPromptFlag/
      ProviderFlag=strPtr("") (non-nil empty); BareFlags=[]string{} (NON-NIL empty — write it explicitly);
      DefaultProvider/PromptFlag/JsonField/RetryInstruction/Env nil (omit).
  - EXTEND BuiltinManifests() to return {"pi","claude","gemini","opencode"} (fresh per call). UPDATE its
      doc comment (4 providers; remaining two codex/cursor → S3).
  - IMPORTS: NONE (verify with grep — builtin.go must have zero import lines, same as S1).
  - GOTCHA: do NOT add Validate/Resolve calls; do NOT add a package-level var; do NOT set absent fields;
      do NOT change S1's builtinPi()/builtinClaude().

Task 2: MODIFY internal/provider/builtin_test.go — add fixtures + 4 tests, update 2 tests
  - ADD the geminiTOML / opencodeTOML constants (above). geminiTOML has prompt_delivery="stdin" (REVISED).
  - UPDATE TestBuiltinManifests_KeysAndCount: assert len==4 and all of pi/claude/gemini/opencode present.
  - UPDATE TestBuiltinManifests_DecodeParity table: add {"gemini", builtinGemini(), geminiTOML} and
      {"opencode", builtinOpenCode(), opencodeTOML} rows. (reflect.DeepEqual catches nil/non-nil + the
      stdin value + the empty-slice.)
  - ADD TestBuiltinManifests_GeminiFields: assert EVERY gemini field (Detect/Command non-nil right value;
      PromptDelivery=="stdin" [REVISED]; PrintFlag/SystemPromptFlag/ProviderFlag NON-NIL *==""; ModelFlag
      "-m"; DefaultModel "gemini-2.5-pro"; BareFlags reflect.DeepEqual ["--approval-mode","default"];
      Output "raw"; StripCodeFence non-nil true) AND absent fields nil (Subcommand/PromptFlag/DefaultProvider/
      JsonField/RetryInstruction/Env). Reuse S1's assertStr/assertNilStr helpers.
  - ADD TestBuiltinManifests_OpenCodeFields: assert EVERY opencode field (Detect/Command right value;
      Subcommand reflect.DeepEqual ["run"] NON-NIL; PromptDelivery "positional"; PrintFlag/DefaultModel/
      SystemPromptFlag/ProviderFlag NON-NIL *==""; ModelFlag "-m"; BareFlags != nil && len==0 [NON-NIL
      EMPTY — assert `m.BareFlags != nil` explicitly, then len 0]; Output "raw"; StripCodeFence true) AND
      absent fields nil (PromptFlag/DefaultProvider/JsonField/RetryInstruction/Env).
  - ADD TestBuiltinManifests_RenderedCommand_Gemini: argv := renderArgs(builtinGemini(), "", "", "<sys>")
      (model="" → default gemini-2.5-pro); assert argv == ["gemini","-m","gemini-2.5-pro",
      "--approval-mode","default"] (stdin: payload NOT in argv; sys prepended to stdin payload).
  - ADD TestBuiltinManifests_RenderedCommand_OpenCode: flags := renderArgs(builtinOpenCode(), "",
      "anthropic/claude-sonnet-4", "") (explicit model — default is "" so no -m if model==""); argv :=
      append(flags, "<sys>\n\n<payload>") (positional: payload appended per §12.2); assert argv ==
      ["opencode","run","-m","anthropic/claude-sonnet-4","<sys>\n\n<payload>"] (matches §12.6 block).
  - DO NOT change renderArgs/assertStr/assertNilStr signatures (S1's pi render test depends on them).
  - NameMatchesKey + Validate auto-cover gemini/opencode (they iterate the whole map) — no edit needed.

Task 3: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. S1's manifest.go/
      manifest_test.go AND S2(merge)'s merge.go/merge_test.go MUST be byte-unchanged. S1's pi/claude tests
      MUST stay green (no field/type/import/signature change). The config + git suites MUST stay green.
```

### Implementation Patterns & Key Details

```go
// The gemini Fields test — pins the stdin revision + the explicit-empty/absent pattern (mirrors S1's
// PiFields/ClaudeFields style using the existing assertStr/assertNilStr helpers).
func TestBuiltinManifests_GeminiFields(t *testing.T) {
	m := builtinGemini()
	assertStr(t, "Detect", m.Detect, "gemini")
	assertStr(t, "Command", m.Command, "gemini")
	assertStr(t, "PromptDelivery", m.PromptDelivery, "stdin") // REVISED from §12.5 "positional"
	assertStr(t, "PrintFlag", m.PrintFlag, "")                // NON-NIL explicit empty
	assertStr(t, "ModelFlag", m.ModelFlag, "-m")
	assertStr(t, "DefaultModel", m.DefaultModel, "gemini-2.5-pro")
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "") // NON-NIL explicit empty (prepend)
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")         // NON-NIL explicit empty
	assertNilStr(t, "DefaultProvider", m.DefaultProvider)    // ABSENT in §12.5 → nil
	wantBare := []string{"--approval-mode", "default"}
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
	}
	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}
	// Absent → nil
	if m.Subcommand != nil {
		t.Errorf("Subcommand = %v, want nil", m.Subcommand)
	}
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "JsonField", m.JsonField)
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// The opencode Fields test — pins Subcommand=["run"], the NON-NIL-EMPTY BareFlags, and the 4 explicit-
// empty scalars. Note the BareFlags assertion: NON-NIL and len 0 (NOT nil).
func TestBuiltinManifests_OpenCodeFields(t *testing.T) {
	m := builtinOpenCode()
	assertStr(t, "Detect", m.Detect, "opencode")
	assertStr(t, "Command", m.Command, "opencode")
	wantSub := []string{"run"}
	if !reflect.DeepEqual(m.Subcommand, wantSub) {
		t.Errorf("Subcommand = %v, want %v", m.Subcommand, wantSub)
	}
	assertStr(t, "PromptDelivery", m.PromptDelivery, "positional")
	assertStr(t, "PrintFlag", m.PrintFlag, "")        // NON-NIL explicit empty
	assertStr(t, "ModelFlag", m.ModelFlag, "-m")
	assertStr(t, "DefaultModel", m.DefaultModel, "")  // NON-NIL explicit empty (user must set)
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "") // NON-NIL explicit empty (prepend)
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")         // NON-NIL explicit empty
	assertNilStr(t, "DefaultProvider", m.DefaultProvider)    // ABSENT → nil
	// BareFlags: §12.6 writes bare_flags = [] → NON-NIL empty slice (FINDING D). Assert BOTH non-nil AND len 0.
	if m.BareFlags == nil {
		t.Fatal("BareFlags = nil, want NON-NIL empty []string{} (§12.6 bare_flags = [] per FINDING D)")
	}
	if len(m.BareFlags) != 0 {
		t.Errorf("BareFlags = %v, want empty", m.BareFlags)
	}
	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}
	// Absent → nil
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "JsonField", m.JsonField)
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// The gemini render test — stdin delivery: argv has NO payload (piped); sys prepended to stdin payload.
// renderArgs (S1) models stdin → full argv directly.
func TestBuiltinManifests_RenderedCommand_Gemini(t *testing.T) {
	argv := renderArgs(builtinGemini(), "", "", "<sys>") // model="" → default gemini-2.5-pro
	want := []string{
		"gemini", "-m", "gemini-2.5-pro",
		"--approval-mode", "default",
		// stdin delivery: "<sys>\n\n<user payload>" piped to stdin (NOT in argv). No print/sys/provider flag.
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("gemini rendered argv:\n got %v\nwant %v", argv, want)
	}
}

// The opencode render test — positional delivery: payload IS the trailing positional arg (§12.2).
// renderArgs (S1, stdin-focused) yields the flag portion; append the payload manually for the full argv.
func TestBuiltinManifests_RenderedCommand_OpenCode(t *testing.T) {
	flags := renderArgs(builtinOpenCode(), "", "anthropic/claude-sonnet-4", "") // explicit model (default is "")
	argv := append(flags, "<sys>\n\n<payload>")                                 // positional: payload appended per §12.2
	want := []string{
		"opencode", "run", // command + subcommand
		"-m", "anthropic/claude-sonnet-4", // model_flag + user-set model
		"<sys>\n\n<payload>", // positional payload (sys prepended — no sys flag on `run`)
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("opencode rendered argv:\n got %v\nwant %v", argv, want)
	}
}

// The DecodeParity table update — add two rows. The existing loop + reflect.DeepEqual handle them.
// (geminiTOML has prompt_delivery="stdin"; opencodeTOML is verbatim §12.6 with bare_flags=[].)
func TestBuiltinManifests_DecodeParity(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  Manifest
		toml string
	}{
		{"pi", builtinPi(), piTOML},
		{"claude", builtinClaude(), claudeTOML},
		{"gemini", builtinGemini(), geminiTOML},       // geminiTOML = §12.5 with stdin revision
		{"opencode", builtinOpenCode(), opencodeTOML}, // opencodeTOML = verbatim §12.6
	} {
		var decoded Manifest
		if err := toml.Unmarshal([]byte(tc.toml), &decoded); err != nil {
			t.Fatalf("%s: decode failed: %v", tc.name, err)
		}
		if !reflect.DeepEqual(tc.got, decoded) {
			t.Errorf("%s: built-in != decoded TOML\n built-in: %+v\n decoded:  %+v", tc.name, tc.got, decoded)
		}
	}
}

// KeysAndCount update — 2 → 4 keys.
func TestBuiltinManifests_KeysAndCount(t *testing.T) {
	m := BuiltinManifests()
	if len(m) != 4 {
		t.Fatalf("BuiltinManifests() returned %d keys, want 4", len(m))
	}
	for _, k := range []string{"pi", "claude", "gemini", "opencode"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. builtin.go has zero imports (unchanged from S1); builtin_test.go uses testing + reflect +
        go-toml/v2 (all already in go.mod from S1; S2 adds nothing). `go mod tidy` MUST be a no-op.
        `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - internal/provider → (stdlib: NONE in builtin.go; testing+reflect+toml in builtin_test.go) ONLY.
        UNCHANGED from S1 — no new edge.
  - internal/provider → internal/config : FORBIDDEN (cycle; same as S1/S2). The REGISTRY (P1.M2.T3) is
        the sole importer of both config and provider.

FROZEN FILES (do NOT edit):
  - internal/provider/manifest.go + manifest_test.go (S1): the Manifest type + strPtr/boolPtr + Validate
        are a CONTRACT. This subtask MODIFIES builtin.go/builtin_test.go ONLY.
  - internal/provider/merge.go + merge_test.go (S2 merge): MergeManifest. This subtask does NOT depend on
        it; do NOT edit it.
  - internal/config/* (P1.M1.T4), internal/git/* (P1.M1.T2/T3), cmd/stagecoach/main.go, Makefile.

DOWNSTREAM CONTRACTS (do NOT implement here — just honor the shapes they will consume):
  - P1.M2.T3 (registry): `builtins := BuiltinManifests(); base := builtins["gemini"]` (or "opencode");
        `merged := MergeManifest(base, decode(reencode(config.Providers["gemini"])))`. This subtask
        provides gemini + opencode as bases.
  - P1.M2.T4 (renderer): reads the RESOLVED manifest per §12.2 — the two render tests prove the DATA is
        sufficient to render the §12.5(stdin)/§12.6(positional) argvs. The real renderer will have its OWN
        tests; renderArgs here is throwaway.
  - P1.M2.T5 (executor): reads *resolved.Command ("gemini"/"opencode") + resolved.Env (nil → none); for
        gemini the payload goes to the stdin pipe; for opencode it is the positional arg.
  - P1.M2.T6 (parser): reads *resolved.Output ("raw"), *resolved.JsonField (""), *resolved.StripCodeFence (true).
  - P1.M2.T2.S3 (sibling subtask): will ADD codex/cursor constructors and extend BuiltinManifests()'s map
        (4→6 keys, update KeysAndCount). The map-returning function is designed so each addition is a
        one-line change (it is: append `"codex": builtinCodex()` etc.).
  => BuiltinManifests() signature + the gemini/opencode field values are now FROZEN for downstream. Do not
     change them after this subtask.

NO DATABASE / NO ROUTES / NO CLI / NO RENDERER/EXECUTOR/PARSER / NO REGISTRY / NO CODEX/CURSOR /
NO providers/*.toml FILES (P1.M5.T2).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1 (builtin.go):
gofmt -w internal/provider/builtin.go internal/provider/builtin_test.go
test -z "$(gofmt -l internal/provider/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/provider/        # (and `go vet ./...`) Expect zero diagnostics.
go build ./...                     # Whole module compiles. Expect exit 0.
# Expected: clean. builtin.go STILL has ZERO imports (verify) — the only `import` lines are in builtin_test.go.
grep -n '^import\|^	"' internal/provider/builtin.go && echo "note: builtin.go has imports (should be NONE)" || echo "builtin.go zero-imports (good)"

# Confirm NO new dependency + NO edit to S1(manifest)/S2(merge) files + no config edge:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"   # MUST be empty.
git diff --exit-code internal/provider/manifest.go internal/provider/manifest_test.go internal/provider/merge.go internal/provider/merge_test.go && echo "S1(manifest)+S2(merge) files UNCHANGED (expected)"   # MUST be empty.
grep -n 'internal/config' internal/provider/builtin.go && echo "BAD: config import" || echo "no config import (good)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# All tests (white-box; no git/exec/config needed — pure literal data + toml round-trip + §12.2 port):
go test -race ./internal/provider/ -v
# Expected: PASS — S1's pi/claude tests (PiFields, ClaudeFields, RenderedCommand_Pi_MatchesCommitPi,
#   FreshEachCall, NameMatchesKey, Validate) STILL GREEN; the 2 updated (KeysAndCount now 4, DecodeParity
#   now 4 rows) + 4 new (GeminiFields, OpenCodeFields, RenderedCommand_Gemini, RenderedCommand_OpenCode)
#   GREEN. Validate + NameMatchesKey now also cover gemini/opencode via the larger map.

# Full suite must stay green (no regression; confirms no stray import edge broke config/git):
go test -race ./...
# Expected: all packages PASS (config, git, provider).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build + scope checks:
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"   # main.go stub still links.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged"
# Confirm this subtask touched ONLY the two builtin*.go files:
git diff --exit-code -- internal/config internal/git cmd Makefile internal/provider/manifest.go internal/provider/manifest_test.go internal/provider/merge.go internal/provider/merge_test.go && echo "frozen + S1(manifest) + S2(merge) files UNCHANGED by this subtask"
grep -n 'func builtinGemini\|func builtinOpenCode' internal/provider/builtin.go   # MUST print both function lines.
grep -c '"gemini"\|"opencode"' internal/provider/builtin.go                       # MUST be >= 2 (the map keys).
# Expected: binary builds; go.mod/go.sum unchanged; frozen+S1(manifest)+S2(merge) files unchanged; both
# constructors present; the map has 4 keys.

# Coverage of the new code (Makefile has a coverage target):
go test -race ./internal/provider/ -coverprofile=/tmp/cov.out && go tool cover -func=/tmp/cov.out | grep -E 'builtinGemini|builtinOpenCode|BuiltinManifests'
# Expected: builtinGemini + builtinOpenCode at/near 100% line coverage (exercised by Fields/DecodeParity/
# Render/Keys/Validate). (make coverage runs the project-wide gate; the ≥85% target is enforced at P1.M5.T3.S3.)

# Smoke the two render equivalences directly (the argv pins for the renderer):
go test -race ./internal/provider/ -run 'TestBuiltinManifests_RenderedCommand_Gemini|TestBuiltinManifests_RenderedCommand_OpenCode' -v
# Expected: PASS — gemini renders the stdin argv; opencode renders the §12.6 positional argv.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Lint gate (project-wide):
golangci-lint run ./internal/provider/ 2>/dev/null || echo "golangci-lint not installed (Makefile lint gate is project-wide; run `make lint` in CI)."

# Cross-check against Appendix D (h2.27) quick-reference table by eye:
#   gemini row: delivery positional/stdin (we chose stdin), model flag -m, sys-prompt *(prepend)*,
#     bare `--approval-mode default`, output raw. ✔
#   opencode row: command `opencode run`, delivery positional, model flag -m (provider/model),
#     sys-prompt *(prepend)*, bare —, output raw. ✔
# The DecodeParity + Fields tests already assert these mechanically; this is a human sanity pass.

# Sanity: confirm the gemini stdin deviation is the ONLY diff from verbatim §12.5 (diff the TOMLs mentally):
#   every §12.5 line is present in geminiTOML except prompt_delivery (positional→stdin). ✔
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` is a
      no-op; `git diff --exit-code go.mod go.sum` empty; `builtin.go` STILL has ZERO imports.
- [ ] Level 2 green: `go test -race ./internal/provider/ -v` (S1's pi/claude tests + 2 updated + 4 new)
      AND `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; `manifest.go`/`manifest_test.go` (S1 manifest) +
      `merge.go`/`merge_test.go` (S2 merge) unchanged; every file outside the two modified `builtin*.go`
      files unchanged; both new constructors present; map has 4 keys.

### Feature Validation

- [ ] `BuiltinManifests() map[string]Manifest` returns exactly `{"pi","claude","gemini","opencode"}`.
- [ ] gemini manifest: every field per §12.5 (with the stdin revision); `PromptDelivery`=="stdin";
      `PrintFlag`/`SystemPromptFlag`/`ProviderFlag` are NON-NIL `*""`; `DefaultProvider`/`Subcommand`/
      `PromptFlag`/`JsonField`/`RetryInstruction`/`Env` nil; `BareFlags` is the 2-token slice.
- [ ] opencode manifest: every field per §12.6 (verbatim); `Subcommand`==`["run"]` (non-nil);
      `PrintFlag`/`DefaultModel`/`SystemPromptFlag`/`ProviderFlag` are NON-NIL `*""`; `BareFlags`==
      `[]string{}` (NON-NIL empty); `DefaultProvider`/`PromptFlag`/`JsonField`/`RetryInstruction`/`Env` nil.
- [ ] Both `Validate()` → nil.
- [ ] `reflect.DeepEqual(built-in, decode(TOML))` holds for both (geminiTOML = §12.5 with stdin revision;
      opencodeTOML = verbatim §12.6).
- [ ] gemini rendered via §12.2 == `["gemini","-m","gemini-2.5-pro","--approval-mode","default"]` (stdin).
- [ ] opencode rendered via §12.2 == `["opencode","run","-m","anthropic/claude-sonnet-4",
      "<sys>\n\n<payload>"]` (positional, matches §12.6 block).
- [ ] S1's pi/claude tests + helpers UNCHANGED and still passing.

### Code Quality Validation

- [ ] Follows repo conventions: white-box `package provider`, stdlib `testing`, `t.Errorf`, `reflect.DeepEqual`,
      reuse of S1's `assertStr`/`assertNilStr`/`renderArgs` helpers (mirrors `builtin_test.go`); free-function
      constructors; zero imports in `builtin.go`.
- [ ] File placement matches the desired tree (MODIFY `builtin.go` + `builtin_test.go` ONLY — S1(manifest)/
      S2(merge) untouched).
- [ ] The explicit-empty vs absent pattern is reproduced exactly for both providers (3 + 4 non-nil-empty
      scalars; absent fields nil) — NOT flattened.
- [ ] opencode's `BareFlags` is `[]string{}` (non-nil empty), matching §12.6's `bare_flags = []` (FINDING D)
      — NOT omitted to nil.
- [ ] `internal/provider` production code still imports nothing outside stdlib (S1/S2 discipline preserved);
      `builtin.go` imports NOTHING.
- [ ] No premature scope: no registry (P1.M2.T3), no Validate/Resolve call inside constructors, no
      renderer/exec/parse (T4/T5/T6), no codex/cursor (S3), no `providers/*.toml` (P1.M5.T2), no opencode
      `--agent` workflow (Appendix E #3, v1.1).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Doc comment on `BuiltinManifests` (updated to 4 providers) + each new constructor citing the PRD
      section + the explicit-empty notes + the gemini stdin revision rationale (seeds `providers show` /
      reference-file docs later).
- [ ] No new env vars / no user-facing docs (PRD "DOCS: none — compiled-in defaults"; public docs come
      with `providers show` P1.M4.T1.S3 and reference files P1.M5.T2).

---

## Anti-Patterns to Avoid

- ❌ Don't create new files — this subtask MODIFIES the two `builtin*.go` files S1 created (extends the map
  + adds constructors/tests). Creating a new file duplicates the `BuiltinManifests` surface and breaks the
  single-source-of-truth invariant the registry depends on.
- ❌ Don't paste the verbatim §12.5 TOML as the gemini decode-parity fixture — it has `prompt_delivery =
  "positional"` which contradicts the mandated "stdin" revision; the test WILL FAIL. Use the fixture with
  `prompt_delivery = "stdin"` (+ the deviation comment).
- ❌ Don't omit opencode's `BareFlags` (leaving it nil) — §12.6 writes `bare_flags = []` which decodes
  NON-NIL empty; nil fails decode-parity. Write `BareFlags: []string{}` explicitly.
- ❌ Don't "helpfully" set a default model for opencode — `default_model = ""` is INTENTIONAL (Appendix E
  #3: the model space is huge and user-specific; require the user to set it).
- ❌ Don't invent a system-prompt flag value for gemini/opencode — neither CLI has one; both set
  `system_prompt_flag = ""` (explicit empty) and the §12.2 renderer prepends the sys prompt to the payload.
- ❌ Don't change the `renderArgs`/`assertStr`/`assertNilStr` helper signatures — S1's pi render test
  depends on them. For the opencode positional render, append the payload manually.
- ❌ Don't forget to UPDATE `TestBuiltinManifests_KeysAndCount` (2→4) — it WILL fail once the map has 4
  entries. Same for the `DecodeParity` table (+2 rows).
- ❌ Don't call `Validate`/`Resolve` inside the constructors — they BUILD; the registry validates.
- ❌ Don't add imports to `builtin.go` — it stays import-free (literal `strPtr`/`boolPtr` + slice literals).
- ❌ Don't edit the frozen files (`manifest.go`, `merge.go`, config, git, main.go, Makefile, go.mod).
