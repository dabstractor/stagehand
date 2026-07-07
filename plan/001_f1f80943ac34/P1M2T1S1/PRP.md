---
name: "P1.M2.T1.S1 — Manifest struct with all §12.1 fields, TOML tags, and validation"
description: |
  Land the foundational `internal/provider` package: a `Manifest` struct carrying ALL 18 fields of the
  PRD §12.1 provider-manifest schema (snake_case `toml:` tags), plus `Validate() error`,
  `DetectCommand() string`, `Resolve() Manifest`, and the default/enum constants. This is the FIRST
  subtask of the 2-subtask Provider Manifest Schema & Merge Logic (P1.M2.T1): S2 (field-by-field merge)
  builds ON this struct. The Manifest is the contract consumed by the registry (P1.M2.T3), the command
  renderer (P1.M2.T4), the executor (P1.M2.T5), and the output parser (P1.M2.T6). PRD §12.1 (schema) +
  §12.2 (rendering algorithm — the field-access contract) are the spec; arch `go_ecosystem_patterns.md`
  §5.2/§5.4 (struct tags + the no-omitempty pointer workaround) + `external_deps.md` (verified field
  values per provider) are the patterns; `critical_findings.md` FINDING 5 (merge must be field-by-field,
  distinguish absent vs zero) is the constraint.

  ⚠️ **THE central design call — Manifest uses POINTER types (`*string`, `*bool`) for the optional
  SCALAR fields; slices/map stay plain; `Name` stays plain `string`.** go-toml/v2 has NO `omitempty`
  (FINDING 5 / arch §5.4). The item says "Use pointer types OR a merge approach for the omitempty
  limitation (§5.4)." Pointers are chosen because they are the ONLY way a **field-by-field struct merge**
  (the S2 contract AND the frozen `config.go` Providers comment: "re-encodes the entry to TOML and
  unmarshals into a Manifest, then field-merges with the built-in manifest per PRD §16.1") can
  distinguish "field ABSENT in the override" (nil → inherit built-in) from "field EXPLICITLY set to its
  zero value" (non-nil → override). This was VERIFIED EMPIRICALLY (see
  `research/go-toml-pointer-behavior.md`): absent TOML keys decode to nil pointers/slices/maps; present
  keys decode to non-nil EVEN WHEN the value is `""`, `false`, or `[]`; and nil pointer fields are
  OMITTED on marshal (a free omitempty for `providers show`). The decisive cases — a user override of
  `strip_code_fence = false` against a built-in `true`, or `print_flag = ""` against a built-in `-p` —
  are correctly applied with pointers and are IMPOSSIBLE to express correctly with plain `bool`/`string`.

  ⚠️ **THE second design call — slices (`Subcommand`, `BareFlags`) and `Env` map stay PLAIN (no
  pointers).** Per the probe: a nil slice/map is the natural "absent" sentinel (absent → nil; present →
  non-nil even if empty), so pointers buy nothing here and would only add dereference noise. nil slices
  are safe for the §12.2 renderer (`append(args, nilSlice...)` is a no-op). (Cosmetic note: nil slices
  marshal as `[]`, not omitted — acceptable; the PRD §12.1 manifests themselves show `subcommand = []`.)

  ⚠️ **THE third design call — `Resolve()` gives consumers a guaranteed-non-nil manifest.** Pointer
  fields make MERGE correct but would force every renderer/executor/parser line to nil-check +
  dereference. `Resolve() Manifest` returns a copy with every nil optional pointer filled to its default
  (the 4 PRD-defaulted fields → their defaults; the remaining optional `*string` fields → `*""`;
  `Command` left nil if absent so `Validate` can flag it — but every OTHER pointer becomes non-nil).
  Lifecycle: **decode → merge (S2/registry) → Validate → Resolve → consume**. Consumers call
  `r := m.Resolve()` once then dereference freely (`*r.PrintFlag`, `*r.StripCodeFence`, …). This mirrors
  the Config precedent (P1.M1.T4.S1) of a single "resolved" value on the read path.

  ⚠️ **THE fourth design call — `internal/provider` does NOT import `internal/config` (import-cycle
  guard).** `config.go` (P1.M1.T4.S1, FROZEN) carries `Providers map[string]map[string]any \`toml:"-"\``
  as a RAW map precisely because "the provider MANIFEST type is defined later (P1.M2.T1), so config must
  not import it (import-cycle risk). The registry (P1.M2.T3) consumes this map." So the dependency arrow
  is ONE-WAY: **registry (P1.M2.T3) → both `config` and `provider`**; `provider` imports neither. The
  Manifest is fully self-contained (stdlib `fmt`/`errors`/`strings` only; go-toml/v2 is used in the TEST
  only, already in go.mod). S1 adds NO new dependency and NO new import edges into config.

  ⚠️ **THE fifth design call — `Validate()` is nil-tolerant on optional enums but STRICT on required
  fields.** It enforces: `Name != ""`; `Command != nil && *Command != ""`; any NON-NIL `PromptDelivery`
  ∈ {"stdin","positional","flag"}; any NON-NIL `Output` ∈ {"raw","json"`. A nil enum is allowed (it will
  take its default via Resolve); a present-but-invalid enum is an error. This makes `Validate` safe to
  run on a PARTIAL override (nil optional fields OK) yet still catch a malformed required/enum — but its
  PRIMARY use is on the FINAL merged manifest (a partial override legitimately lacks `Command`).

  Deliverable: `internal/provider/manifest.go` (`package provider`) — the `Manifest` struct (18 fields,
  toml-tagged) + `Validate` + `DetectCommand` + `Resolve` + the `Default*` constants and `valid*` enum
  sets + the private `strPtr`/`boolPtr` helpers; and `internal/provider/manifest_test.go` (`package
  provider`, white-box) — decode (full/partial/explicit-empty), marshal (nil-pointer omission),
  `Validate` (valid + each failure mode), `DetectCommand` (Detect-set / Detect-empty / Detect-nil),
  `Resolve` (defaults applied, non-nil guarantee, slices left nil). INPUT = the go module from
  P1.M1.T1.S1 + go-toml/v2 v2.4.2 (already in go.mod). Touches ONLY `internal/provider/` — NO change to
  go.mod/go.sum, NO edit to any FROZEN file (config/*, git/*, …). OUTPUT = the `Manifest` type for S2
  (merge), P1.M2.T3 (registry), P1.M2.T4 (render), P1.M2.T5 (exec), P1.M2.T6 (parse).
---

## Goal

**Feature Goal**: Define the single provider-manifest type for all of Stagecoach — a `Manifest` struct
whose 18 fields carry the PRD §12.1 schema (name, detect, command, subcommand, prompt_delivery,
prompt_flag, print_flag, model_flag, default_model, system_prompt_flag, provider_flag,
default_provider, bare_flags, output, json_field, strip_code_fence, retry_instruction, env) as
pointer-typed scalars (for correct field-by-field merge under go-toml/v2's no-omitempty limitation) plus
plain slices/map, with `toml:` snake_case tags; plus the `Validate()` / `DetectCommand()` / `Resolve()`
methods and the PRD §12.1 defaults. This type is the contract produced by the registry merge and read by
every downstream provider-system consumer.

**Deliverable**:
1. **CREATE** `internal/provider/manifest.go` (`package provider`) —
   (a) `Manifest` struct: exactly the 18 fields below (1 plain `string` `Name`; 13 `*string` scalar
       pointers `Command`/`Detect`/`PromptDelivery`/`PromptFlag`/`PrintFlag`/`ModelFlag`/`DefaultModel`/
       `SystemPromptFlag`/`ProviderFlag`/`DefaultProvider`/`Output`/`JsonField`/`RetryInstruction`; 1
       `*bool` `StripCodeFence`; 2 `[]string` `Subcommand`/`BareFlags`; 1 `map[string]string` `Env`),
       each tagged with its §12.1 snake_case `toml:` key. Field order mirrors §12.1's commented sections
       (discovery → prompt delivery → print → model → system prompt → sub-provider → bare → output →
       retry → env) with a comment per field.
   (b) Package-level constants: `DefaultPromptDelivery = "stdin"`, `DefaultOutput = "raw"`,
       `DefaultStripCodeFence = true`, `DefaultRetryInstruction` = the §12.1 literal; and the unexported
       `validPromptDeliveries` / `validOutputs` enum sets.
   (c) `func (m Manifest) Validate() error` — required-field + enum checks (see design call #5).
   (d) `func (m Manifest) DetectCommand() string` — `*Detect` if non-nil & non-empty, else `*Command`
       ("" if both nil/empty).
   (e) `func (m Manifest) Resolve() Manifest` — copy with all nil optional pointers filled to defaults
       (`PromptDelivery`→stdin, `Output`→raw, `StripCodeFence`→true, `RetryInstruction`→default,
       remaining optional `*string`→`*""`); `Command` left nil if absent; slices/map left as-is.
   (f) Private helpers `strPtr(string) *string` and `boolPtr(bool) *bool` (used by `Resolve` + tests).
   (g) Imports: stdlib `fmt` + `errors` + `strings` ONLY (no `config`, no `toml` in this file).
2. **CREATE** `internal/provider/manifest_test.go` (`package provider`, white-box) —
   (a) `TestUnmarshal_FullManifest`: decode the §12.3 pi manifest TOML → every field non-nil & correct.
   (b) `TestUnmarshal_PartialManifest_NilPointers`: decode a sparse override (name + print_flag +
       bare_flags only) → assert the ABSENT fields are nil (Command/Detect/Strip/... nil; Sub/Env nil)
       and the PRESENT ones non-nil (the merge foundation — FINDING C).
   (c) `TestUnmarshal_ExplicitZeroNonNil`: decode `print_flag = ""`, `strip_code_fence = false`,
       `subcommand = []` → assert non-nil pointers carrying ""/false and a non-nil empty slice
       (FINDING D — the override-to-zero case pointers make correct).
   (d) `TestMarshal_OmitsNilPointers`: build a Manifest with only Name set → `toml.Marshal` output
       contains `name` and omits every nil-pointer key (command/print_flag/...) but may include
       `subcommand = []`/`bare_flags = []` (FINDING A/B). Asserts the omitempty property.
   (e) `TestValidate_*`: ValidManifest_Passes (pi manifest); MissingName_Errors; MissingCommand_Errors
       (Command nil AND Command=&""); BadPromptDelivery_Errors (*"weird"); BadOutput_Errors (*"xml");
       NilEnumsAreOK (nil PromptDelivery/Output do not error).
   (f) `TestDetectCommand_*`: ReturnsDetectWhenSet; FallsBackToCommandWhenDetectEmpty
       (`detect = ""`); FallsBackToCommandWhenDetectNil; EmptyWhenBothNil.
   (g) `TestResolve_*`: AppliesDefaultsToNilOptionals (nil PromptDelivery/Output/StripCodeFence/
       RetryInstruction → stdin/raw/true/default); PreservesExplicitValues (explicit false/"" survive,
       NOT overwritten by defaults — the correctness keystone); OptionalStringsBecomeEmpty (unset
       PrintFlag/ModelFlag/... → non-nil `*""`); SlicesLeftNil (nil Subcommand/BareFlags stay nil);
       CommandLeftNilIfAbsent (Resolve does not fabricate a command).

No other files touched. **No go.mod/go.sum change** (go-toml/v2 already present, test-only use). No
merge logic (S2), no registry (P1.M2.T3), no rendering/execution/parsing (P1.M2.T4/T5/T6), no built-in
manifest content (P1.M2.T2).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/` clean;
`go mod tidy` is a no-op (no new dep); `go test -race ./internal/provider/ -v` passes (all 7 test groups
green) and the full suite `go test -race ./...` stays green; the 18 fields/types/tags match the table
below exactly; `Validate` accepts the pi manifest and rejects each malformed case; `DetectCommand`
returns Detect-or-Command correctly; `Resolve` applies the 4 defaults, preserves explicit zeros, and
leaves every optional pointer non-nil (except `Command` when absent); `internal/provider` imports NO
package outside the stdlib (in particular NOT `internal/config`).

## User Persona

**Target User**: Downstream Stagecoach subtasks — S2 (field-by-field merge), P1.M2.T3 (registry: decodes
`config.Providers[<name>]` raw map → Manifest, merges with the built-in, Validate, Resolve), P1.M2.T4
(renderer: reads the resolved Manifest per §12.2), P1.M2.T5 (executor: reads Command/Env), P1.M2.T6
(parser: reads Output/JsonField/StripCodeFence), and the built-in manifest authors (P1.M2.T2). Transit-
ively US11 (provider management, FR36/FR37) and every user story routed through "call an agent."

**Use Case**: The registry holds a `Manifest` per provider; the renderer turns one into an `exec.Cmd`
argv; the executor runs it; the parser cleans its output. This subtask fixes the TYPE all four share.

**User Journey**: (internal API, no end-user surface yet) built-in TOML (P1.M2.T2) → `toml.Unmarshal`
into `Manifest` (pointers) → registry merges a user override (S2) → `Validate()` → `Resolve()` →
renderer/executor/parser read `*r.ModelFlag`, `*r.StripCodeFence`, `r.BareFlags`, …

**Pain Points Addressed**: Removes "what shape is a manifest / how do we merge a partial user override
without clobbering built-ins / how does `providers show` omit unset fields / can a user override a bool
to false" ambiguity for S2 + T3–T6 by fixing one pointer-typed struct + Validate/Resolve now.

## Why

- **Contract for the whole Provider System.** S2 (merge), P1.M2.T3 (registry), T4 (render), T5 (exec),
  T6 (parse) all consume THIS `Manifest`. Landing the type + methods first lets those be implemented and
  tested against a stable target (no churn later).
- **Makes the merge provably correct.** Pointers (empirically verified) let S2's field-by-field merge
  distinguish absent (nil → inherit) from set-to-zero (non-nil → override). Plain types would silently
  drop a user's `strip_code_fence = false` / `print_flag = ""` override — a real bug, not a cosmetic
  one (FINDING 5).
- **Gives `providers show` free omitempty.** nil pointer fields are omitted on marshal (FINDING A), so
  P1.M4.T1.S3 can render a clean manifest without a custom marshaler (though one may still be added
  later for ordering/comments).
- **Locks the §12.2 field-access contract.** The rendering algorithm reads `m.provider_flag`,
  `m.print_flag`, … as plain strings; `Resolve()` + dereference delivers exactly that to the renderer.
- **No import-cycle risk.** `provider` is self-contained (stdlib only); `config` stays raw-map until the
  registry bridges them. S1 adds zero new module edges.
- **No user-facing surface change** (PRD "DOCS: none — internal struct"). Manifest field docs arrive
  with `providers show` (P1.M4.T1.S3, Mode A) and the reference files (P1.M5.T2).

## What

A compiled `internal/provider` package exporting `Manifest` (18 toml-tagged fields, pointer-typed
scalars) + `Validate` + `DetectCommand` + `Resolve` + the default/enum constants, with go-toml/v2 used
in the test only (already declared). No merge, no registry, no rendering, no execution, no parsing.

### Success Criteria

- [ ] `internal/provider/manifest.go` exists, `package provider`, imports ONLY stdlib
      (`fmt`, `errors`, `strings`). It does NOT import `github.com/pelletier/go-toml/v2` (struct tags
      are string literals) and does NOT import `internal/config`.
- [ ] `Manifest` has exactly these 18 fields, types, and tags (gofmt-aligned), in this order:
      `Name string \`toml:"name"\`` · `Detect *string \`toml:"detect"\`` · `Command *string
      \`toml:"command"\`` · `Subcommand []string \`toml:"subcommand"\`` · `PromptDelivery *string
      \`toml:"prompt_delivery"\`` · `PromptFlag *string \`toml:"prompt_flag"\`` · `PrintFlag *string
      \`toml:"print_flag"\`` · `ModelFlag *string \`toml:"model_flag"\`` · `DefaultModel *string
      \`toml:"default_model"\`` · `SystemPromptFlag *string \`toml:"system_prompt_flag"\`` ·
      `ProviderFlag *string \`toml:"provider_flag"\`` · `DefaultProvider *string
      \`toml:"default_provider"\`` · `BareFlags []string \`toml:"bare_flags"\`` · `Output *string
      \`toml:"output"\`` · `JsonField *string \`toml:"json_field"\`` · `StripCodeFence *bool
      \`toml:"strip_code_fence"\`` · `RetryInstruction *string \`toml:"retry_instruction"\`` ·
      `Env map[string]string \`toml:"env"\``.
- [ ] Constants: `DefaultPromptDelivery=="stdin"`, `DefaultOutput=="raw"`, `DefaultStripCodeFence==true`,
      `DefaultRetryInstruction=="Output ONLY the commit message. No preamble, no markdown, no quotes."`
      (the §12.1 literal). Unexported `validPromptDeliveries`={stdin,positional,flag},
      `validOutputs`={raw,json}.
- [ ] `Validate()` returns nil for the §12.3 pi manifest; returns non-nil (wrapped, identifiable cause)
      when `Name==""`, when `Command==nil` or `*Command==""`, when a non-nil `PromptDelivery` is not in
      {stdin,positional,flag}, or when a non-nil `Output` is not in {raw,json}. nil enums are allowed.
- [ ] `DetectCommand()` returns `*Detect` when Detect is non-nil and non-empty; else `*Command`; else "".
- [ ] `Resolve()` returns a copy where: nil `PromptDelivery`→&"stdin", nil `Output`→&"raw", nil
      `StripCodeFence`→&true, nil `RetryInstruction`→&DefaultRetryInstruction, every OTHER nil optional
      `*string` (Detect/PromptFlag/PrintFlag/ModelFlag/DefaultModel/SystemPromptFlag/ProviderFlag/
      DefaultProvider/JsonField)→&""; `Command` is left nil if it was nil; `Subcommand`/`BareFlags`/`Env`
      are left as-is (nil stays nil). EXPLICIT values are preserved (an explicit `*false` / `*""` is NOT
      overwritten by a default).
- [ ] `manifest_test.go` has the 7 test groups above, all passing.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` (diff-empty) clean;
      `go test -race ./internal/provider/` AND `go test -race ./...` green.
- [ ] go.mod/go.sum byte-unchanged by S1; every file outside `internal/provider/` byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the field/type/tag
table above, the Validate/DetectCommand/Resolve specs, the empirically-verified pointer behavior in
`research/go-toml-pointer-behavior.md`, and the 7 test specs. No git/config/generation knowledge
required — this subtask is one self-contained struct + three methods + tests.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M2T1S1/research/go-toml-pointer-behavior.md
  why: the EMPIRICAL basis for the pointer design. FINDING A (nil pointers omitted on marshal),
       FINDING C (absent keys → nil pointers/slices/maps on unmarshal), FINDING D (present zero values
       → NON-NIL pointers — the override-to-false/"" correctness keystone). Read before touching the
       struct: it proves why plain bool/string CANNOT merge correctly.
  critical: do NOT "simplify" pointers to plain types — the merge (S2) and the override-to-zero cases
       depend on nil-vs-non-nil distinction that plain types cannot express.

- file: PRD.md
  section: "12.1 The manifest schema" (h3.37) — the AUTHORITATIVE field list, TOML key names,
       comments, and the four defaults (prompt_delivery="stdin", output="raw", strip_code_fence=true,
       retry_instruction="Output ONLY the commit message. No preamble, no markdown, no quotes.").
  why: every field name/type/tag/default in the struct comes from here, verbatim. §12.1 also states
       "All fields except `name` and `command` are optional with sensible defaults" — the basis for
       Name=plain/required, Command=pointer/required-after-merge, everything-else-optional.
  critical: the TOML keys are snake_case (prompt_delivery, print_flag, model_flag, system_prompt_flag,
       provider_flag, default_provider, bare_flags, json_field, strip_code_fence, retry_instruction).
       Do NOT camelCase the tags.

- file: PRD.md
  section: "12.2 Command rendering algorithm" (h3.38) — the FIELD-ACCESS CONTRACT consumers will use.
  why: the renderer reads `m.subcommand`, `m.provider_flag`, `m.model_flag`, `m.system_prompt_flag`,
       `m.bare_flags`, `m.print_flag`, `m.prompt_delivery`, `m.prompt_flag` and the executor reads
       `m.command`/`m.env`. This is why field NAMES and the Resolve()→dereference consumer pattern
       exist. It also fixes the valid `prompt_delivery` enum (stdin|positional|flag) and that
       `print_flag`/`provider_flag`/`model_flag`/`system_prompt_flag` empty == "not used".
  pattern: after `r := m.Resolve()`, the renderer writes `if *r.ProviderFlag != "" && provider != ""
       { args = append(args, *r.ProviderFlag, provider) }` etc. — mirrors the pseudocode exactly.

- file: PRD.md
  section: "12.3–12.7 Built-in providers" — the canonical FULLY-EXPANDED manifests (pi/claude/gemini/
       opencode/codex/cursor) used as test fixtures (esp. the §12.3 pi manifest for TestValidate/
       TestUnmarshal_FullManifest) and as the proof that every field is exercised across the set.
  why: a real manifest is the best Validate() happy-path fixture. The pi manifest (§12.3) sets every
       optional field non-empty, so decoding+validating it exercises the whole struct.

- file: plan/001_f1f80943ac34/architecture/external_deps.md
  why: VERIFIED per-provider field values (captured from live `--help` 2026-06-29). Confirms the enum
       members (prompt_delivery stdin|positional|flag; output raw|json) and that several manifests
       legitimately use empty strings (gemini/opencode/codex/cursor have system_prompt_flag="",
       provider_flag="", print_flag="") — exactly the override-to-empty case pointers protect.
  critical: §codex flags a discrepancy (--ask-for-approval not on `codex exec`) — that is a
       P1.M2.T2 (built-in manifest content) concern, NOT S1. S1 only defines the SCHEMA; it encodes no
       built-in manifest values.

- file: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md
  section: "5.2 Struct Tag Reference" + "5.4 Omitempty Workarounds" (Option A: pointers).
  why: confirms go-toml/v2 honors `toml:"<key>"`, has NO omitempty, and that pointer fields are the
       recommended workaround for "field absent vs zero" (exactly the merge problem). §5.5 confirms
       map-of-struct decode works natively (relevant to the registry, P1.M2.T3, not S1).
  critical: the arch §5.4 Option A claim "nil pointers are omitted during marshal" is VERIFIED TRUE
       by the probe (FINDING A) — rely on it for TestMarshal_OmitsNilPointers.

- file: plan/001_f1f80943ac34/architecture/critical_findings.md
  section: "FINDING 5"
  why: states the constraint directly: go-toml/v2 has no omitempty; "Provider manifest merge: when a
       user override sets only default_model, ALL other fields must remain from the built-in. The merge
       must be field-by-field (only override non-zero/non-nil fields). Use pointer types (*string,*bool)
       or decode to map[string]any first to detect key presence." S1 chooses pointers (FINDING 5's first
       option) so S2's field-by-field merge is trivial and correct.

- file: internal/config/config.go
  section: the `Providers map[string]map[string]any \`toml:"-"\`` field + its doc comment.
  why: the FROZEN contract this struct must NOT collide with. config holds provider overrides as a RAW
       map precisely so it need not import the Manifest type (cycle). The comment ("the provider
       MANIFEST type is defined later (P1.M2.T1)... the registry (P1.M2.T3) consumes this map — for each
       name it re-encodes the entry to TOML and unmarshals into a Manifest, then field-merges with the
       built-in manifest per PRD §16.1") is the reason Manifest is pointer-typed (struct field-merge
       needs nil-detection) and the reason `provider` imports neither `config` nor `toml`.
  gotcha: S1 does NOT touch config.go; it only reads it to honor the no-import-config constraint.

- file: internal/provider/manifest.go   (the file you are creating — N/A for "follow a pattern")
  note: there is no existing `internal/provider` package; this subtask CREATES it. For Go STRUCT +
       method + table-test conventions, mirror `internal/config/config.go` (doc comments, gofmt tag
       alignment, value-receiver methods) and `internal/config/config_test.go` / `internal/git/git_test.go`
       (white-box `package <pkg>`, stdlib `testing`, direct `t.Errorf` assertions).

- url: https://github.com/pelletier/go-toml/v2
  why: confirms struct-tag semantics (`toml:"key"`, lowercased-Go-name default, `toml:"-"` exclusion),
       map/slice decode, and the deliberate absence of omitempty. (No Duration concern here — Manifest
       has no time fields.)
  critical: go-toml/v2 decodes a TOML string into a `*string` field as non-nil even when the value is
       "" (FINDING D); decode into a plain `string` cannot distinguish absent from "". That asymmetry is
       the entire reason for the pointer choice.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; require go-toml/v2 v2.4.2 + pflag v1.0.10  (UNCHANGED by S1)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 (Config + loaders + Load) — FROZEN, do NOT touch; do NOT import from provider
    config.go                   # Providers map[string]map[string]any `toml:"-"`  ← the raw-map bridge target (registry consumes it, NOT S1)
    ...                         # file.go / git.go / load.go + tests — untouched
  git/                          # P1.M1.T2/T3 — untouched
  provider/                     # NEW (S1) ← Manifest struct + Validate + DetectCommand + Resolve + constants
    manifest.go                 # NEW
    manifest_test.go            # NEW
cmd/stagecoach/main.go           # `package main; func main(){}` stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  provider/
    manifest.go                 # NEW — Manifest (18 fields, toml tags, pointer scalars) + Validate + DetectCommand + Resolve + Default* constants + valid* sets + strPtr/boolPtr
    manifest_test.go            # NEW — decode/marshal/validate/detectcommand/resolve tests (7 groups)
# go.mod / go.sum UNCHANGED (go-toml/v2 already present; test-only use). Every other file UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call #1): Manifest scalar fields are POINTERS (*string/*bool). This is NOT optional
// styling — it is the only way S2's field-by-field merge can tell "override absent" (nil → keep built-in)
// from "override explicitly zero" (non-nil → apply, even if "" or false). Verified empirically: absent
// TOML key → nil pointer; present key (even "") → non-nil pointer (research FINDING C/D). Plain types
// would silently drop a user's strip_code_fence=false / print_flag="" override. Do NOT "simplify" away.

// CRITICAL (design call #2): Subcommand/BareFlags ([]string) and Env (map[string]string) stay PLAIN.
// Slices/maps have a natural nil sentinel (absent → nil; present → non-nil even if empty — FINDING C/D),
// so pointers buy nothing and add dereference noise. nil slices are renderer-safe (append(nil...) no-op).

// CRITICAL (design call #3): Resolve() is the consumer's friend. Pointer fields make MERGE correct but
// force nil-checks on every read. Resolve() returns a copy with all nil OPTIONAL pointers filled to
// defaults (the 4 PRD defaults + "" for the other optionals), so consumers do `r := m.Resolve()` once
// then `*r.PrintFlag` etc. safely. Command is left nil if absent (Validate flags it); slices/map as-is.
// Lifecycle enforced by the registry (P1.M2.T3): decode → merge → Validate → Resolve → consume.

// CRITICAL (design call #4): internal/provider imports NO internal package (NOT config, NOT git). It is
// stdlib-only (fmt/errors/strings). config.go's Providers raw-map comment is explicit that config must
// not import the Manifest type (cycle); symmetrically, provider must not import config. The REGISTRY
// (P1.M2.T3) is the only place that imports both. Adding a config import here = import cycle + violates
// the frozen config.go contract.

// CRITICAL (design call #5): Validate is nil-tolerant on optional enums but strict on required fields.
// nil PromptDelivery/Output are OK (Resolve supplies the default); a NON-NIL but invalid value errors.
// Name must be non-empty; Command must be non-nil AND *Command non-empty. This lets Validate run safely
// on a partial override (nil optionals fine) yet still guard the merged result. Its PRIMARY call site
// is post-merge (a partial override legitimately lacks Command — that's not a Validate error there
// because Validate is called on the MERGED manifest, not the partial).

// CRITICAL: go-toml/v2 is used in the TEST only. manifest.go has NO toml import (struct tags are string
// literals). This keeps the package stdlib-only (see design call #4) and means go.mod is unchanged.
// go-toml/v2 v2.4.2 is already required (P1.M1.T4.S1) — verify with `go list -m`; do NOT re-add it.

// GOTCHA: go-toml/v2 marshals nil pointer fields as OMITTED (FINDING A) but nil slices as `[]`
// (FINDING B). TestMarshal_OmitsNilPointers must assert scalar omission while TOLERATING `subcommand =
// []` / `bare_flags = []` appearing. (If a future P1.M4.T1.S3 wants slices omitted too, it adds a
// custom MarshalTOML — out of scope for S1.)

// GOTCHA: go-toml/v2 emits single-quoted literal strings (`'pi'`) by default, not double-quoted. This
// is valid TOML and cosmetic; the hand-written providers/*.toml reference files (P1.M5.T2) are
// independent of any marshaled output. Do NOT add a custom marshaler to force double quotes in S1.

// GOTCHA: an empty `Env` map marshals as an empty `[env]` table header; a nil Env map is omitted. Tests
// that marshal a Name-only Manifest should NOT assert anything about `env` presence (nil → omitted is
// fine). Validate/Resolve treat nil Env as "no extra env" (correct).

// GOTCHA: Resolve must PRESERVE explicit zeros. If StripCodeFence is a non-nil *bool=false, Resolve
// MUST leave it false (do NOT blanket-apply the true default to every StripCodeFence — only to nil
// ones). TestResolve_PreservesExplicitValues is the correctness keystone for the whole pointer design.
// Pattern: `if out.StripCodeFence == nil { out.StripCodeFence = boolPtr(DefaultStripCodeFence) }`.

// GOTCHA: helper constructors strPtr/boolPtr must be UNEXPORTED (lowercase). They are an internal
// convenience for Resolve + tests; exporting them pollutes the package API. (The registry/tests in the
// same package can use them; other packages build manifests via decode, not literal construction.)

// GOTCHA: methods use VALUE receivers (func (m Manifest) ...) — Manifest is a small struct of pointers/
// slices/headers (the map/slice headers are cheap to copy); value receivers avoid nil-pointer hazards
// and match the Config/Defaults value-receiver precedent. Resolve returns Manifest BY VALUE.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/provider/manifest.go
package provider

import (
	"errors"
	"fmt"
)

// PRD §12.1 default values (applied by Resolve to nil optional fields).
const (
	DefaultPromptDelivery  = "stdin"                                                     // §12.1 prompt_delivery default
	DefaultOutput          = "raw"                                                       // §12.1 output default
	DefaultStripCodeFence  = true                                                        // §12.1 strip_code_fence default
	DefaultRetryInstruction = "Output ONLY the commit message. No preamble, no markdown, no quotes." // §12.1 retry_instruction
)

// validPromptDeliveries / validOutputs are the §12.1 / §12.2 enum members Validate enforces.
var (
	validPromptDeliveries = map[string]struct{}{"stdin": {}, "positional": {}, "flag": {}}
	validOutputs          = map[string]struct{}{"raw": {}, "json": {}}
)

// Manifest describes one AI-provider CLI per PRD §12.1. Built-in manifests are compiled in (P1.M2.T2);
// user manifests live under [provider.<name>] in config (raw map — config.Providers) and are merged
// field-by-field onto a built-in by the registry (P1.M2.T3) per PRD §16.1.
//
// DESIGN CALL — POINTER SCALARS. go-toml/v2 has no omitempty (arch §5.4 / FINDING 5), so the optional
// SCALAR fields are *string / *bool: a field ABSENT in a user override decodes to nil (→ inherit the
// built-in value on merge), while a field PRESENT — even set to "" or false — decodes to a NON-NIL
// pointer (→ override). This is the only way a field-by-field struct merge can honor a user's
// strip_code_fence=false or print_flag="" override. Verified empirically; see research FINDING C/D.
// Slices (Subcommand, BareFlags) and the Env map stay plain: nil is their natural "absent" sentinel
// (absent → nil; present → non-nil even if empty), so pointers would add only dereference noise.
// Name is plain: it is the identity, always set by the registry from the [provider.<name>] table key.
//
// Decode target + merge participant + (after Resolve) consumed value — one struct, three roles.
type Manifest struct {
	// --- discovery (§12.1) ---
	Name     string   `toml:"name"`     // REQUIRED. The identity; registry sets this from the table key.
	Detect   *string  `toml:"detect"`   // nil/"" => DetectCommand falls back to Command.
	Command  *string  `toml:"command"`  // REQUIRED (post-merge). nil in a partial override => inherit.
	Subcommand []string `toml:"subcommand"` // nil => none; inserted between command and flags.

	// --- prompt delivery (§12.1) ---
	PromptDelivery *string `toml:"prompt_delivery"` // stdin|positional|flag; nil => Resolve→"stdin".
	PromptFlag     *string `toml:"prompt_flag"`     // used only when PromptDelivery=="flag".

	// --- non-interactive / print mode (§12.1) ---
	PrintFlag *string `toml:"print_flag"` // nil/"" => no print flag appended.

	// --- model (§12.1) ---
	ModelFlag   *string `toml:"model_flag"`
	DefaultModel *string `toml:"default_model"` // nil/"" => user must set a model.

	// --- system prompt (§12.1) ---
	SystemPromptFlag *string `toml:"system_prompt_flag"` // nil/"" => prepend sys to payload (§12.2).

	// --- sub-provider (§12.1) ---
	ProviderFlag    *string `toml:"provider_flag"`
	DefaultProvider *string `toml:"default_provider"` // e.g. "zai" for pi; nil/"" => omit.

	// --- bare mode (§12.1) ---
	BareFlags []string `toml:"bare_flags"` // appended verbatim; nil => none.

	// --- output (§12.1) ---
	Output         *string `toml:"output"`          // raw|json; nil => Resolve→"raw".
	JsonField      *string `toml:"json_field"`      // used only when Output=="json".
	StripCodeFence *bool   `toml:"strip_code_fence"` // nil => Resolve→true.

	// --- retry (§12.1) ---
	RetryInstruction *string `toml:"retry_instruction"` // prepended on a parse-retry; nil => Resolve→default.

	// --- environment (§12.1) ---
	Env map[string]string `toml:"env"` // set ONLY for the subprocess; nil => none.
}

// Validate checks the merged manifest's required fields and enum members (PRD §12.1). It is
// nil-tolerant on optional enums (a nil PromptDelivery/Output will take its default via Resolve) but
// strict on Name (non-empty) and Command (non-nil, non-empty), and rejects any NON-NIL but invalid
// enum. Safe to run on a partial override (nil optionals pass); its primary call site is post-merge.
func (m Manifest) Validate() error {
	if m.Name == "" {
		return errors.New("provider manifest: name is required")
	}
	if m.Command == nil || *m.Command == "" {
		return errors.New("provider manifest: command is required")
	}
	if m.PromptDelivery != nil {
		if _, ok := validPromptDeliveries[*m.PromptDelivery]; !ok {
			return fmt.Errorf("provider manifest %q: prompt_delivery %q must be one of stdin|positional|flag", m.Name, *m.PromptDelivery)
		}
	}
	if m.Output != nil {
		if _, ok := validOutputs[*m.Output]; !ok {
			return fmt.Errorf("provider manifest %q: output %q must be one of raw|json", m.Name, *m.Output)
		}
	}
	return nil
}

// DetectCommand returns the discovery command: Detect if set and non-empty, else Command (§12.1:
// "If absent, `command` is used"). Returns "" if neither is set (the registry treats "" as
// "not installed" via exec.LookPath).
func (m Manifest) DetectCommand() string {
	if m.Detect != nil && *m.Detect != "" {
		return *m.Detect
	}
	if m.Command != nil {
		return *m.Command
	}
	return ""
}

// Resolve returns a copy of m with every nil OPTIONAL pointer filled to its default, so consumers
// (renderer/executor/parser) can dereference every pointer safely. The four PRD-defaulted fields take
// their §12.1 defaults; the remaining optional *string fields take *"" (semantically "not used");
// Command is left nil if it was nil (Validate, run before Resolve, flags a missing command); slices
// and the Env map are left as-is (nil stays nil — append(nil...) is a no-op for the renderer).
// EXPLICIT values — including a non-nil *false or *"" — are PRESERVED (Resolve never overwrites a
// present value; this is the correctness keystone of the pointer design).
func (m Manifest) Resolve() Manifest {
	out := m // copy the headers/pointers/slices/map
	if out.Detect == nil {
		out.Detect = strPtr("")
	}
	if out.Command == nil {
		out.Command = nil // left nil; Validate enforces requiredness. (Do NOT fabricate "".)
	}
	if out.PromptDelivery == nil {
		out.PromptDelivery = strPtr(DefaultPromptDelivery)
	}
	if out.PromptFlag == nil {
		out.PromptFlag = strPtr("")
	}
	if out.PrintFlag == nil {
		out.PrintFlag = strPtr("")
	}
	if out.ModelFlag == nil {
		out.ModelFlag = strPtr("")
	}
	if out.DefaultModel == nil {
		out.DefaultModel = strPtr("")
	}
	if out.SystemPromptFlag == nil {
		out.SystemPromptFlag = strPtr("")
	}
	if out.ProviderFlag == nil {
		out.ProviderFlag = strPtr("")
	}
	if out.DefaultProvider == nil {
		out.DefaultProvider = strPtr("")
	}
	if out.Output == nil {
		out.Output = strPtr(DefaultOutput)
	}
	if out.JsonField == nil {
		out.JsonField = strPtr("")
	}
	if out.StripCodeFence == nil {
		out.StripCodeFence = boolPtr(DefaultStripCodeFence)
	}
	if out.RetryInstruction == nil {
		out.RetryInstruction = strPtr(DefaultRetryInstruction)
	}
	// Subcommand / BareFlags / Env: left as-is (nil stays nil).
	return out
}

// strPtr / boolPtr are unexported helpers for constructing pointer fields (Resolve + same-package tests).
func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool     { return &b }
// IMPORTS: errors + fmt only (Validate uses fmt.Errorf for wrapped, field-named errors). Do NOT import
// `strings` unless a method genuinely uses it — an unused import fails `go vet`/`go build`. Do NOT import
// `internal/config` (cycle) or `go-toml/v2` (struct tags are string literals; toml is test-only).
```

> **gofmt note:** run `gofmt -w internal/provider/manifest.go` — it aligns the `toml:"…"` tag column
> across adjacent fields. Do not hand-align. The block comments per field are encouraged (they become
> the §12.1 field docs surfaced by `providers show` later) but keep each to one line.
>
> **Imports:** the sketch needs only `errors` + `fmt` (Validate uses `fmt.Errorf` for wrapped, field-
> named errors). Do NOT import `strings` speculatively — an unused import fails `go vet`/`go build`. Add
> an import only when a method genuinely calls into it.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/provider/manifest.go — constants + enum sets + helpers
  - IMPLEMENT the 4 Default* constants (exact §12.1 values) and the 2 unexported valid* maps
    (validPromptDeliveries={stdin,positional,flag}, validOutputs={raw,json}).
  - IMPLEMENT unexported strPtr(string) *string + boolPtr(bool) *bool.
  - IMPORTS: fmt + errors (the Validate sketch uses fmt.Errorf; add `strings` ONLY if a method genuinely
    uses it — never speculatively, `go vet` rejects unused imports).
  - WHY FIRST: the struct + methods reference these; landing them first keeps the file compiling at
    each step. NO new module dep (go-toml/v2 is test-only and already in go.mod).

Task 2: ADD the Manifest struct to internal/provider/manifest.go
  - IMPLEMENT the 18-field struct per the Data Models block: Name plain; Command/Detect/PromptDelivery/
    PromptFlag/PrintFlag/ModelFlag/DefaultModel/SystemPromptFlag/ProviderFlag/DefaultProvider/Output/
    JsonField/RetryInstruction as *string; StripCodeFence as *bool; Subcommand/BareFlags as []string;
    Env as map[string]string. snake_case toml tags per §12.1. One-line doc comment per field.
  - GOTCHA: pointer types are NON-NEGOTIABLE (design call #1). Do NOT collapse to plain string/bool.
  - GOTCHA: do NOT import config or toml here. Stdlib only.

Task 3: ADD Validate + DetectCommand + Resolve to internal/provider/manifest.go
  - IMPLEMENT Validate() per the Data Models block: Name!="" ; Command!=nil && *Command!="" ; non-nil
    PromptDelivery ∈ enum ; non-nil Output ∈ enum ; nil enums OK. Wrapped errors that name the field.
  - IMPLEMENT DetectCommand(): *Detect if non-nil&non-empty else *Command else "".
  - IMPLEMENT Resolve(): copy; fill nil optionals per the table (4 defaults + "" for the rest); PRESERVE
    explicit values (only fill when ==nil); Command left nil; slices/map left as-is. Return by value.
  - GOTCHA: Resolve MUST NOT blanket-apply defaults to non-nil fields (PreservesExplicitValues test).

Task 4: CREATE internal/provider/manifest_test.go — decode tests (the merge foundation)
  - PACKAGE: `package provider` (white-box). Imports: testing + github.com/pelletier/go-toml/v2 + fmt.
  - TEST TestUnmarshal_FullManifest: decode the §12.3 pi TOML (name..env, all set) → assert every field
    non-nil and equal to the expected value (Name=="pi", *Command=="pi", *PromptDelivery=="stdin",
    *PrintFlag=="-p", *ModelFlag=="--model", *DefaultModel=="glm-5-turbo", *SystemPromptFlag==
    "--system-prompt", *ProviderFlag=="--provider", BareFlags==[...6...], *Output=="raw",
    *StripCodeFence==true, Subcommand==nil-or-[], Env==nil-or-{}).
  - TEST TestUnmarshal_PartialManifest_NilPointers: decode `name="x"`, `print_flag="-p"`,
    `bare_flags=["a"]` → assert Command==nil, Detect==nil, PromptDelivery==nil, StripCodeFence==nil,
    ModelFlag==nil, … (every absent scalar) AND Subcommand==nil, Env==nil; assert *PrintFlag=="-p"
    (non-nil) and BareFlags==["a"]. THIS is the proof absent→nil (FINDING C).
  - TEST TestUnmarshal_ExplicitZeroNonNil: decode `print_flag=""`, `strip_code_fence=false`,
    `subcommand=[]` → assert PrintFlag!=nil && *PrintFlag=="" , StripCodeFence!=nil && *StripCodeFence
    ==false, Subcommand!=nil && len==0. THIS is the proof present-zero→non-nil (FINDING D — the
    override-to-false/"" keystone).

Task 5: ADD marshal + Validate + DetectCommand + Resolve tests
  - TEST TestMarshal_OmitsNilPointers: build Manifest{Name:"gemini"} (rest zero) → toml.Marshal → assert
    output contains `name` and does NOT contain `command`, `print_flag`, `strip_code_fence`,
    `prompt_delivery`, `model_flag` (nil pointers omitted — FINDING A). TOLERATE `subcommand = []` /
    `bare_flags = []` possibly appearing (FINDING B) — do not assert their absence.
  - TEST TestValidate_*: (a) pi manifest → nil; (b) Manifest{Name:""} → error mentioning "name";
    (c) Manifest{Name:"x"} (Command nil) → error mentioning "command"; (d) Manifest{Name:"x",
    Command:strPtr("x"), PromptDelivery:strPtr("weird")} → error mentioning "prompt_delivery"; (e) same
    with Output:strPtr("xml") → error mentioning "output"; (f) Manifest{Name:"x", Command:strPtr("x")}
    (nil enums) → nil (enums optional).
  - TEST TestDetectCommand_*: Detect set → returns *Detect; Detect=strPtr("") → falls back to *Command;
    Detect nil → falls back to *Command; both nil → "".
  - TEST TestResolve_*: (a) nil optionals → *PromptDelivery=="stdin", *Output=="raw", *StripCodeFence
    ==true, *RetryInstruction==DefaultRetryInstruction; (b) explicit StripCodeFence=boolPtr(false) →
    Resolve leaves *StripCodeFence==false (PRESERVED — the keystone); explicit Output=strPtr("json") →
    stays "json"; (c) unset PrintFlag/ModelFlag/... → non-nil *""; (d) nil Subcommand/BareFlags/Env →
    stay nil; (e) nil Command → stays nil (not fabricated).

Task 6: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged
    (`git diff --exit-code go.mod go.sum` empty). Every file outside internal/provider/ MUST be
    byte-unchanged. The config + git suites MUST stay green (no import edge added).
```

### Implementation Patterns & Key Details

```go
// The "fill nil with default, PRESERVE explicit" Resolve pattern — the correctness keystone. Only fill
// when the field is nil; a non-nil field (even *false / *"") is left untouched.
if out.StripCodeFence == nil {
	out.StripCodeFence = boolPtr(DefaultStripCodeFence) // absent → default true
}
// (an explicit boolPtr(false) above is NON-nil → this branch is skipped → false survives. CORRECT.)

// The "absent vs explicit-zero" decode asymmetry — the reason pointers exist (verified FINDING C/D):
//   TOML absent            → m.PrintFlag == nil         (inherit on merge)
//   TOML `print_flag = ""` → m.PrintFlag != nil, *==""  (override to empty on merge)
// A plain `string` field cannot make this distinction (both look like ""). Never use plain for a
// scalar that a user might override to its zero value.

// Validate is nil-tolerant on enums (optional) but strict on required — safe on a partial, authoritative
// on the merged result:
if m.PromptDelivery != nil { // only validate a PRESENT enum
	if _, ok := validPromptDeliveries[*m.PromptDelivery]; !ok {
		return fmt.Errorf("provider manifest %q: prompt_delivery %q must be stdin|positional|flag", m.Name, *m.PromptDelivery)
	}
}

// Consumer pattern (renderer, P1.M2.T4) — Resolve once, dereference freely, mirroring §12.2:
//   r := m.Resolve()
//   args := append([]string{}, r.Subcommand...)           // nil-safe (append(nil...) is a no-op)
//   if *r.ProviderFlag != "" && provider != "" {          // §12.2: "if m.provider_flag and provider"
//       args = append(args, *r.ProviderFlag, provider)
//   }
//   if *r.PrintFlag != "" { args = append(args, *r.PrintFlag) }
// (Documented here for the implementer; the renderer itself is P1.M2.T4 — NOT implemented in S1.)
```

```go
// manifest_test.go — the keystone test (explicit zero survives Resolve). If this fails, the pointer
// design's whole point is lost.
func TestResolve_PreservesExplicitValues(t *testing.T) {
	m := Manifest{
		Name:           "x",
		Command:        strPtr("x"),
		StripCodeFence: boolPtr(false), // explicit false — must NOT become the true default
		Output:         strPtr("json"),  // explicit json — must NOT become the raw default
	}
	r := m.Resolve()
	if r.StripCodeFence == nil || *r.StripCodeFence != false {
		t.Errorf("Resolve clobbered explicit strip_code_fence=false (got %v)", r.StripCodeFence)
	}
	if r.Output == nil || *r.Output != "json" {
		t.Errorf("Resolve clobbered explicit output=json (got %v)", r.Output)
	}
}

// manifest_test.go — the absent-vs-present asymmetry (the merge foundation).
func TestUnmarshal_PartialVsExplicit(t *testing.T) {
	// Absent print_flag → nil.
	var a Manifest
	if err := toml.Unmarshal([]byte(`name="a"`+"\n"+`command="a"`+"\n"), &a); err != nil { t.Fatal(err) }
	if a.PrintFlag != nil { t.Errorf("absent print_flag: want nil, got non-nil %q", *a.PrintFlag) }

	// Explicit print_flag="" → non-nil, empty.
	var b Manifest
	if err := toml.Unmarshal([]byte(`name="b"`+"\n"+`command="b"`+"\n"+`print_flag=""`+"\n"), &b); err != nil { t.Fatal(err) }
	if b.PrintFlag == nil { t.Fatal("explicit print_flag=\"\": want non-nil, got nil") }
	if *b.PrintFlag != "" { t.Errorf("explicit print_flag=\"\": want \"\", got %q", *b.PrintFlag) }
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. go-toml/v2 v2.4.2 is already required (P1.M1.T4.S1); S1 uses it in the TEST only.
    `go mod tidy` MUST be a no-op. `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - internal/provider → (stdlib: fmt, errors, strings?) ONLY.
  - internal/provider → internal/config : FORBIDDEN (cycle; config.go's raw-map Providers exists to
        avoid importing the Manifest type). The REGISTRY (P1.M2.T3) is the sole importer of both.
  - internal/provider → github.com/pelletier/go-toml/v2 : in the TEST file only (manifest.go has no
        toml import; struct tags are string literals).

FROZEN FILES (do NOT edit):
  - internal/config/* (P1.M1.T4): Config.Providers raw map is the bridge target the registry consumes.
  - internal/git/* (P1.M1.T2/T3), cmd/stagecoach/main.go, Makefile: untouched.

DOWNSTREAM CONTRACTS (do NOT implement here — just honor the shapes they will consume):
  - P1.M2.T1.S2 (field-by-field merge): for each field, `if override.Field != nil { base.Field =
        override.Field }` (scalars/pointers); `if override.Slice != nil { base.Slice = override.Slice }`
        (slices/map). Pointers make this trivial + correct.
  - P1.M2.T2 (built-in manifests): authors embedded TOML per §12.3–12.7; decode into THIS Manifest.
  - P1.M2.T3 (registry): re-encode config.Providers[<name>] → TOML → Unmarshal into Manifest (partial)
        → merge onto built-in (S2) → Validate → Resolve → hand to renderer/executor/parser.
  - P1.M2.T4 (renderer): reads the RESOLVED manifest per §12.2 (*r.ModelFlag, r.BareFlags, …).
  - P1.M2.T5 (executor): reads *r.Command (post-Resolve/Validate, non-nil) + r.Env.
  - P1.M2.T6 (parser): reads *r.Output, *r.JsonField, *r.StripCodeFence.
  => Manifest field names/types/tags + the Resolve/Validate/DetectCommand contracts are now FROZEN for
     downstream. Do not rename/retag after this subtask.

NO DATABASE / NO ROUTES / NO CLI / NO BUILT-IN MANIFEST CONTENT (that is P1.M2.T2) / NO MERGE (S2).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Tasks 1–3 (manifest.go):
gofmt -w internal/provider/manifest.go internal/provider/manifest_test.go
test -z "$(gofmt -l internal/provider/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/provider/        # (and `go vet ./...`) Expect zero diagnostics.
go build ./...                     # Whole module compiles (incl. the new package). Expect exit 0.
# Expected: clean. NO unused import (add `strings` ONLY if a method genuinely uses it; never speculatively).
#   NO toml/config import in manifest.go (verify): grep should find tags only as string literals.

# Confirm NO new dependency + NO import edge into config:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"   # MUST be empty.
grep -n 'internal/config' internal/provider/manifest.go && echo "BAD: config import" || echo "no config import (good)"
grep -n 'pelletier/go-toml' internal/provider/manifest.go && echo "note: toml in non-test (ok only if justified)" || echo "toml test-only (good)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 7 test groups (white-box; no git/exec needed — pure struct + toml round-trips):
go test -race ./internal/provider/ -v
# Expected: PASS — TestUnmarshal_FullManifest, TestUnmarshal_PartialManifest_NilPointers,
#   TestUnmarshal_ExplicitZeroNonNil, TestMarshal_OmitsNilPointers, TestValidate_* (6 cases),
#   TestDetectCommand_* (4 cases), TestResolve_* (5 cases incl. PreservesExplicitValues keystone).

# Full suite must stay green (no regression; confirms no stray import edge broke config/git):
go test -race ./...
# Expected: all packages PASS (config, git, provider).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build + scope/additive checks:
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"   # main.go stub still links.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged"
# Confirm S1 did NOT touch anything outside internal/provider/:
git diff --exit-code -- internal/config internal/git cmd Makefile && echo "frozen files UNCHANGED by S1"
grep -n 'func.*Manifest.*Validate\|func.*DetectCommand\|func.*Resolve' internal/provider/manifest.go
# Expected: binary builds; go.mod/go.sum unchanged; frozen files unchanged; Validate/DetectCommand/
#   Resolve present; Manifest has 18 toml-tagged fields (`grep -c 'toml:"' internal/provider/manifest.go`
#   prints 18).
grep -c 'toml:"' internal/provider/manifest.go   # MUST print 18.

# Smoke the decode→validate→resolve→render-readiness with the §12.3 pi manifest (sanity for the S2/T3–T6
# authors): a throwaway /tmp test that decodes pi, merges nothing, Validates, Resolves, and prints the
# argv the renderer WOULD build — eyeball it against the §12.3 rendered command.
cat > /tmp/smoke_manifest_test.go <<'EOF'
package main
import ("fmt";"os";"github.com/pelletier/go-toml/v2";"github.com/dustin/stagecoach/internal/provider")
func main(){
  tomlSrc := []byte(`name="pi"
detect="pi"
command="pi"
prompt_delivery="stdin"
print_flag="-p"
model_flag="--model"
default_model="glm-5-turbo"
system_prompt_flag="--system-prompt"
provider_flag="--provider"
bare_flags=["--no-tools","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session"]
output="raw"
strip_code_fence=true
`)
  var m provider.Manifest
  if err := toml.Unmarshal(tomlSrc, &m); err != nil { fmt.Println("decode:",err); os.Exit(1) }
  if err := m.Validate(); err != nil { fmt.Println("validate:",err); os.Exit(1) }
  r := m.Resolve()
  fmt.Printf("detect=%q  *printFlag=%q  *strip=%v  *output=%q  bare=%v\n",
    m.DetectCommand(), *r.PrintFlag, *r.StripCodeFence, *r.Output, r.BareFlags)
}
EOF
# (Run from repo root so the replace-less module resolves: `go run /tmp/smoke_manifest_test.go` is NOT
#  valid as a standalone file importing internal — instead rely on the in-package table tests, which
#  already assert the full matrix. The snippet above is illustrative of the consumer contract.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Enum-exhaustiveness + cross-provider coverage (mirrors the §12.1 contract across the six manifests).
# Optional but recommended: extend manifest_test.go with a table of the §12.3–12.7 manifests (as TOML
# strings) and assert each (a) decodes without error, (b) Validates, (c) Resolve leaves StripCodeFence
# non-nil and Output in {raw,json}. This proves the schema admits all six real providers — the strongest
# evidence the struct is complete. (The manifest VALUES themselves are P1.M2.T2's deliverable; S1 only
# needs the schema to ADMIT them, which this table demonstrates.)

# Lint gate (project-wide):
golangci-lint run ./internal/provider/ 2>/dev/null || echo "golangci-lint not installed (Makefile lint gate is project-wide; run `make lint` in CI)."

# Property-style invariant (optional, if you add a quick loop): for random subsets of fields set to
# random values, Resolve must (1) never panic, (2) leave every optional pointer non-nil, (3) never
# change a non-nil field's value. This is the formal statement of the PreservesExplicitValues keystone.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, and
      `go mod tidy` is a no-op; `git diff --exit-code go.mod go.sum` empty.
- [ ] Level 2 green: `go test -race ./internal/provider/ -v` (all 7 groups) AND `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; every file outside `internal/provider/`
      unchanged; `Validate`/`DetectCommand`/`Resolve` present; exactly 18 `toml:"` tags.

### Feature Validation

- [ ] `Manifest` has exactly the 18 fields/types/tags in Success Criteria (1 plain Name, 13 `*string`,
      1 `*bool`, 2 `[]string`, 1 `map[string]string`).
- [ ] `Validate` accepts the §12.3 pi manifest; rejects empty Name, nil/empty Command, invalid
      PromptDelivery, invalid Output; allows nil enums.
- [ ] `DetectCommand` returns Detect→Command→"" in priority order.
- [ ] `Resolve` applies the 4 §12.1 defaults to nil fields, PRESERVES explicit zeros (the keystone),
      leaves optional `*string` unset → non-nil `*""`, leaves slices/Env/Command(nil) as-is.
- [ ] Decode: absent keys → nil pointers/slices/maps; present zero values → non-nil (FINDING C/D).
- [ ] Marshal: nil pointer fields omitted (FINDING A).

### Code Quality Validation

- [ ] Follows repo conventions: white-box `package provider`, stdlib `testing`, `t.Errorf` assertions
      (mirrors `internal/config`/`internal/git` test style); value-receiver methods; gofmt tag alignment.
- [ ] File placement matches the desired tree (`manifest.go` + `manifest_test.go` only).
- [ ] Pointer scalars (NOT plain) on the optional fields — the merge depends on it (design call #1).
- [ ] `internal/provider` imports stdlib only — NO `config`, NO `toml` in `manifest.go` (design call #4).
- [ ] No premature scope: no merge (S2), no registry (T3), no render/exec/parse (T4/T5/T6), no built-in
      manifest values (T2), no custom MarshalTOML.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Doc comments on `Manifest` (pointer-design rationale), `Validate`/`DetectCommand`/`Resolve`
      (contracts), and each field (one-line §12.1 reference) — these seed the `providers show` docs.
- [ ] No new env vars / no user-facing docs (PRD "DOCS: none — internal struct"; public manifest docs
      come with `providers show` P1.M4.T1.S3 and reference files P1.M5.T2).
- [ ] `internal/provider/manifest.go` + `manifest_test.go` are the ONLY files touched.

---

## Anti-Patterns to Avoid

- ❌ Don't use PLAIN `string`/`bool` for the optional scalar fields. go-toml/v2 has no omitempty, and the
  S2 field-by-field merge MUST distinguish "override absent" (nil) from "override set to zero" (non-nil).
  Plain types silently drop a user's `strip_code_fence=false` / `print_flag=""` override. Use pointers
  (empirically verified — research FINDING C/D).
- ❌ Don't make `Subcommand`/`BareFlags`/`Env` pointers — slices/maps have a natural nil sentinel (absent
  → nil; present → non-nil even if empty). Pointers here add dereference noise for zero correctness gain.
- ❌ Don't `import "internal/config"` (or `toml`) in `manifest.go`. The frozen `config.go` Providers
  raw-map exists precisely to avoid a config↔provider cycle; the registry (P1.M2.T3) is the sole bridge.
  `provider` is stdlib-only; `toml` is test-only.
- ❌ Don't blanket-apply defaults in `Resolve` to NON-NIL fields — an explicit `*false`/`*""` MUST survive
  (only fill `== nil`). `TestResolve_PreservesExplicitValues` is the keystone; if it fails, the pointer
  design's entire purpose is defeated.
- ❌ Don't add a `toml:",omitempty"` tag — go-toml/v2 does not support it (it's a silent no-op at best,
  confusing at worst). The pointer field IS the omitempty mechanism.
- ❌ Don't add a custom `MarshalTOML` in S1 to control output (double quotes, slice omission, ordering).
  That is a `providers show` (P1.M4.T1.S3) / reference-file (P1.M5.T2) concern. S1 ships the plain struct
  whose nil-pointer-omission already gives a clean partial render.
- ❌ Don't encode any built-in manifest VALUES (pi/claude/…) in S1 — that is P1.M2.T2. S1 defines the
  SCHEMA only; built-ins are TOML decoded into this struct later.
- ❌ Don't implement merge/registry/render/exec/parse here — S2, P1.M2.T3/T4/T5/T6 own those. S1 is the
  type + three methods + tests.
- ❌ Don't change go.mod/go.sum — go-toml/v2 is already present (P1.M1.T4.S1) and used test-only. An
  unintended `go get`/`go mod tidy` mutation means an unused import crept in; fix the import, not the dep.
- ❌ Don't skip `go vet`/`gofmt`/`go mod tidy` — they are the cheap gates that catch an unused `strings`
  import, an accidental config edge, and formatting drift before downstream subtasks freeze on this
  struct's shape.
