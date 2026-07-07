---
name: "P1.M2.T2.S1 — pi and claude built-in manifests (explicit tool-disable switches)"
description: |
  Land the FIRST subtask of Built-in Provider Manifests (P1.M2.T2): a compiled-in
  `internal/provider/builtin.go` exporting `BuiltinManifests() map[string]Manifest` that returns the
  **pi** and **claude** manifests — the two providers in PRD §12.7.1's "explicit tool-disable switch"
  category (pi `--no-tools …`, claude `--tools ""`). pi offers `--no-tools` (and friends); claude offers
  `--tools ""`; both become a pure text-in/text-out call with no agent loop. The other four providers
  (gemini/opencode/codex/cursor) are S2/S3 of this task — NOT here.

  This subtask builds DIRECTLY on S1's `Manifest` type + unexported `strPtr`/`boolPtr` helpers
  (`internal/provider/manifest.go`, COMPLETE — read as a frozen contract) and is consumed by the
  registry (P1.M2.T3), which calls `BuiltinManifests()` to get the defaults a user override merges onto
  via `MergeManifest` (S2). It touches ONLY `internal/provider/builtin.go` + `builtin_test.go` — it does
  NOT edit `manifest.go`/`manifest_test.go` (S1) or `merge.go`/`merge_test.go` (S2), and adds NO
  dependency (go.mod unchanged).

  ⚠️ **THE central design call — construct with literal `strPtr`/`boolPtr`, NOT runtime TOML decode.**
  The built-in manifests are Go literals compiled into the binary. They are built with S1's same-package
  `strPtr(string) *string` / `boolPtr(bool) *bool` helpers so they carry the SAME pointer semantics a
  decoded manifest would (non-nil pointers for set fields). This keeps `builtin.go` **import-free**
  (consistent with S1's `manifest.go` and S2's `merge.go` — the package's production code stays
  stdlib-only, go.mod provably unchanged). The correctness guarantee that "runtime TOML decode" would
  give ("source matches PRD §12.3/§12.4") is instead delivered by `TestBuiltinManifests_DecodeParity`,
  which embeds the verbatim §12.3/§12.4 TOML in the TEST file, decodes it, and asserts
  `reflect.DeepEqual(builtin, decoded)`. So the PRD TOML literally lives in the source (in the test) and
  any transcription error is caught — with zero production imports. See
  `research/builtin-manifests-pi-claude.md` §5.

  ⚠️ **THE second design call — reproduce the TOML's nil/non-nil pattern EXACTLY (explicit-empty vs
  absent).** The PRD §12.3/§12.4 TOML mixes keys written with an explicit empty value and keys omitted
  entirely. go-toml/v2 decodes these differently (S1 FINDING C/D): `x = ""` → non-nil `*""`; absent key
  → `nil`. The literal construction MUST match this pattern, because (a) a built-in should be
  byte-equivalent to decoding the PRD TOML (so `providers show` and `MergeManifest` behave identically
  to a decoded built-in), and (b) the decode-parity test's `reflect.DeepEqual` treats nil ≠ non-nil.
  Concretely:
    • pi sets `DefaultProvider` to `strPtr("")` (§12.3 writes `default_provider = ""` — NON-NIL empty).
    • claude sets `ProviderFlag` to `strPtr("")` (§12.4 writes `provider_flag = "" # n/a` — NON-NIL empty).
    • claude leaves `DefaultProvider` NIL (§12.4 OMITS the key — do NOT "helpfully" set it).
    • BOTH leave `Subcommand`, `PromptFlag`, `JsonField`, `RetryInstruction`, `Env` NIL (absent in the
      TOML; `Resolve()` fills the optionals at consume time anyway).
  See research §2 for the full map.

  ⚠️ **THE third design call — `claude.BareFlags` contains TWO empty-string tokens; do NOT drop them.**
  `["--tools", "", "--setting-sources", "", "--no-session-persistence"]`. The `""` after `--tools` and
  after `--setting-sources` are the VALUE arguments (disable-all-tools / load-no-settings — external_deps.md
  §claude: *"Use \"\" to disable all tools"*). Dropping them yields `--tools --setting-sources …`, a
  DIFFERENT, broken command. The slice literally encodes `--tools ""` and `--setting-sources ""`.

  ⚠️ **THE fourth design call — `BuiltinManifests()` constructs FRESH manifests each call (no package-level
  `var`).** `strPtr` allocates a new pointer each call; a slice literal allocates a new backing array each
  call → zero shared mutable state. A direct caller mutating a returned `BareFlags`/`Env` cannot corrupt
  the built-in for other callers. `MergeManifest` (S2) already never mutates `base`, so the normal
  registry path is safe either way — but fresh-per-call eliminates even the rogue-caller risk at
  negligible cost (the registry calls this once at init). NO `var builtins = …`.

  ⚠️ **THE fifth design call — the §12.2 render test is the byte-for-byte commit-pi check; the §12.4
  claude rendered block is ILLUSTRATIVE and disagrees with §12.2 on flag order — do NOT "fix" the manifest
  to match it.** §12.2 (the authoritative "Command rendering algorithm") appends `print_flag` LAST (after
  `bare_flags`); §12.4's hand-written rendered block shows `-p` SECOND. For pi, §12.2 and §12.3 agree
  (both `-p` last) → the pi render test asserts the EXACT commit-pi argv (the work-item requirement:
  "Verify the rendered command matches commit-pi byte-for-byte"). For claude, §12.2 and §12.4 disagree →
  claude is verified by decode-parity + field assertions (its render order is illustrative-only); if a
  claude render test is added it MUST assert §12.2's output (`-p` last) and cite the discrepancy. Flag
  order is the renderer's (P1.M2.T4) concern; the manifest only supplies the flags+values. See research §4.

  Deliverable: `internal/provider/builtin.go` (`package provider`, ZERO imports) —
  `func BuiltinManifests() map[string]Manifest` returning `{"pi": builtinPi(), "claude": builtinClaude()}`,
  plus the two unexported constructors; and `internal/provider/builtin_test.go` (`package provider`,
  white-box) — 8 test groups: keys/count, Name==key, pi fields (incl. explicit-empty `DefaultProvider` &
  absent fields nil), claude fields (incl. explicit-empty `ProviderFlag`, nil `DefaultProvider`, the
  two `""` bare-flag tokens), both `Validate()`, **decode-parity** (built-in == decode of verbatim §12.3/
  §12.4 TOML — the byte-faithfulness keystone), **pi rendered command == commit-pi byte-for-byte** (a
  local §12.2 argv-builder in the test), and fresh-each-call (no shared mutable state). INPUT = S1's
  `Manifest` + `strPtr`/`boolPtr` (already in `internal/provider/manifest.go`). Touches ONLY
  `internal/provider/builtin.go` + `builtin_test.go`. OUTPUT = the built-in map the registry (P1.M2.T3)
  consumes; the manifest values the renderer/executor/parser (P1.M2.T4/T5/T6) will run.
---

## Goal

**Feature Goal**: Compile the pi and claude provider manifests into the binary as the zero-config
default — `BuiltinManifests() map[string]Manifest` returning two manifests whose every field matches
PRD §12.3 (pi) and §12.4 (claude) exactly (nil/non-nil pattern included), and whose pi manifest, when
rendered per §12.2, reproduces the `commit-pi` invocation byte-for-byte.

**Deliverable**:
1. **CREATE** `internal/provider/builtin.go` (`package provider`, **ZERO imports**):
   (a) `func BuiltinManifests() map[string]Manifest` returning `{"pi": builtinPi(), "claude": builtinClaude()}`
       (fresh construction each call — design call #4).
   (b) unexported `func builtinPi() Manifest` — every field per the pi table below, built with S1's
       `strPtr`/`boolPtr`; `DefaultProvider` = `strPtr("")` (explicit empty); `Subcommand`/`PromptFlag`/
       `JsonField`/`RetryInstruction`/`Env` left nil (absent in §12.3).
   (c) unexported `func builtinClaude() Manifest` — every field per the claude table below; `ProviderFlag`
       = `strPtr("")` (explicit empty); `DefaultProvider` nil (absent in §12.4); `BareFlags` includes the
       two `""` value tokens; same nil-absent set as pi.
2. **CREATE** `internal/provider/builtin_test.go` (`package provider`, white-box; imports `testing` +
   `reflect` + `github.com/pelletier/go-toml/v2` [test-only, already in go.mod]) — the 8 test groups in
   Implementation Tasks, all passing.

No other files touched. **No go.mod/go.sum change** (`builtin.go` has zero imports; toml is test-only and
already declared). NO edit to `manifest.go`/`manifest_test.go` (S1) or `merge.go`/`merge_test.go` (S2).
No registry (P1.M2.T3), no renderer/executor/parser (P1.M2.T4/T5/T6), no gemini/opencode/codex/cursor
(S2/S3 of this task), no `providers/*.toml` files (P1.M5.T2).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/` clean;
`go mod tidy` is a no-op; `go test -race ./internal/provider/ -v` passes (S1's + S2's tests STILL green
+ all 8 new builtin tests green) and the full suite `go test -race ./...` stays green; the pi + claude
manifests match the tables below exactly (incl. the explicit-empty vs absent nil/non-nil pattern);
`reflect.DeepEqual(builtinPi(), decode(piTOML))` and the claude equivalent both hold; the pi manifest
rendered via a local §12.2 port yields EXACTLY `["pi","--provider","zai","--model","glm-5-turbo",
"--system-prompt","<sys>","--no-tools","--no-extensions","--no-skills","--no-prompt-templates",
"--no-context-files","--no-session","-p"]` (byte-for-byte commit-pi); both manifests `Validate()` → nil.

## User Persona

**Target User**: The registry (P1.M2.T3) — it calls `BuiltinManifests()` to fetch the compiled-in
defaults, then `MergeManifest(builtin, userOverride)` (S2) to overlay any `[provider.pi]`/`[provider.claude]`
config, then `Validate()` + `Resolve()` before handing the manifest to the renderer/executor/parser.
Transitively every user story routed through "call an agent" (US) and FR36/FR37 (provider management).

**Use Case**: A user runs `stagecoach` with zero config. The registry has no `[provider.*]` overrides, so
`BuiltinManifests()["pi"]` IS the resolved pi manifest; the renderer turns it into the `pi …` argv; the
executor runs it; the parser cleans stdout. This subtask is what makes "zero config" work for the two
most common agents.

**User Journey**: (internal API, no end-user surface yet) `BuiltinManifests()` (THIS subtask) → registry
selects `pi`/`claude` (or merges a user override via S2) → `Validate()` → `Resolve()` → renderer builds
argv per §12.2 → executor runs → parser cleans.

**Pain Points Addressed**: Removes "what are the exact default flags for pi/claude / does the built-in
match the PRD TOML / does the pi call still match commit-pi byte-for-byte" ambiguity by landing two
literal, decode-parity-tested, render-verified manifests now.

## Why

- **Zero config works because of this.** PRD §12.1: "Built-in manifests are compiled into the binary
  (so the tool works with zero config)." pi and claude are the two "explicit tool-disable switch"
  providers (§12.7.1) — the cleanest, fastest bare calls. Landing them first lets the registry (P1.M2.T3)
  and renderer (P1.M2.T4) be built + tested against real targets immediately.
- **The pi manifest MUST match commit-pi byte-for-byte.** Stagecoach is the successor to commit-pi
  (PRD §2.1); the pi invocation is the compatibility anchor. The render test pins this so a future edit
  to `builtinPi()` cannot silently drift from commit-pi.
- **Unlocks the registry + renderer.** P1.M2.T3 imports `BuiltinManifests()`; P1.M2.T4 renders one.
  Neither can be written until at least one built-in exists. Landing pi+claude (the two fully-matching
  §12.2 cases) gives both downstream tasks concrete fixtures.
- **Proves the pointer design end-to-end.** The explicit-empty cases (`pi.default_provider=""`,
  `claude.provider_flag=""`) and the absent cases (`claude.default_provider` nil) exercise the exact
  nil/non-nil distinction S1 designed for and S2 merges on. The decode-parity test is the proof the
  built-in is indistinguishable from a decoded user manifest.
- **No user-facing surface change** (PRD "DOCS: none — compiled-in defaults"). `providers show`
  (P1.M4.T1.S3) and the reference `providers/*.toml` files (P1.M5.T2) are where users SEE these later.
- **No new dependency, no new import edge.** `builtin.go` is import-free (literal construction); the
  package's production code stays stdlib-only (S1/S2 discipline); go.mod is unchanged.

## What

A compiled `internal/provider` package exporting `BuiltinManifests() map[string]Manifest` (in addition
to S1's `Manifest`/`Validate`/`DetectCommand`/`Resolve` and S2's `MergeManifest`). Two literal manifests
(pi, claude) constructed fresh per call, decode-parity-verified against the PRD TOML, with the pi render
pinned to the commit-pi argv. No registry, no rendering, no execution, no parsing, no other providers.

### Success Criteria

- [ ] `internal/provider/builtin.go` exists, `package provider`, imports NOTHING (zero import lines —
      verify with grep). It does NOT import `fmt`, `go-toml/v2`, `internal/config`, or anything.
- [ ] `func BuiltinManifests() map[string]Manifest` exists and returns EXACTLY `{"pi": …, "claude": …}`
      (2 keys, no more, no less — gemini/opencode/codex/cursor are S2/S3).
- [ ] Each returned manifest's `.Name` equals its map key (`pi`/`claude`).
- [ ] `builtinPi()` sets every field per the pi table (below) with `strPtr`/`boolPtr`: `Name="pi"`,
      `Detect="pi"`, `Command="pi"`, `PromptDelivery="stdin"`, `PrintFlag="-p"`, `ModelFlag="--model"`,
      `DefaultModel="glm-5-turbo"`, `SystemPromptFlag="--system-prompt"`, `ProviderFlag="--provider"`,
      `DefaultProvider=strPtr("")` (**non-nil empty**), `BareFlags=[6 tokens in order]`, `Output="raw"`,
      `StripCodeFence=true`; AND leaves `Subcommand`/`PromptFlag`/`JsonField`/`RetryInstruction`/`Env`
      **nil** (absent in §12.3).
- [ ] `builtinClaude()` sets every field per the claude table: `Name="claude"`, `Detect="claude"`,
      `Command="claude"`, `PromptDelivery="stdin"`, `PrintFlag="-p"`, `ModelFlag="--model"`,
      `DefaultModel="sonnet"`, `SystemPromptFlag="--system-prompt"`, `ProviderFlag=strPtr("")`
      (**non-nil empty**), `BareFlags=["--tools","","--setting-sources","","--no-session-persistence"]`
      (5 tokens, two of them `""`), `Output="raw"`, `StripCodeFence=true`; AND leaves `Subcommand`/
      `PromptFlag`/`JsonField`/`RetryInstruction`/`Env`/`DefaultProvider` **nil**.
- [ ] `BuiltinManifests()` constructs fresh manifests each call (no package-level `var`); mutating a
      returned manifest's `BareFlags`/`Env` does NOT affect a subsequent call's return.
- [ ] Both `builtinPi().Validate()` and `builtinClaude().Validate()` return nil.
- [ ] `reflect.DeepEqual(builtinPi(), decode(piTOML))` AND `reflect.DeepEqual(builtinClaude(), decode(claudeTOML))`
      both hold (piTOML/claudeTOML are the verbatim §12.3/§12.4 TOML embedded in the test).
- [ ] The pi manifest rendered via a local §12.2 argv-builder (provider="zai", model=default, sys="<sys>")
      equals the commit-pi argv exactly (see Implementation Patterns).
- [ ] `builtin_test.go` has the 8 test groups, all passing.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` (diff-empty) clean;
      `go test -race ./internal/provider/` AND `go test -race ./...` green.
- [ ] go.mod/go.sum byte-unchanged; `manifest.go`/`manifest_test.go` (S1) + `merge.go`/`merge_test.go`
      (S2) byte-unchanged; every file outside the two new `builtin*.go` files byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the two field
tables (pi/claude, with the explicit-empty vs absent map), the `strPtr`/`boolPtr` construction idiom
(from S1's `manifest.go`), the decode-parity test approach + the verbatim TOML strings (provided below),
the §12.2 render algorithm (provided verbatim) + the exact expected commit-pi argv, and the 8 test specs.
No git/config/generation knowledge required — this subtask is two literal structs + tests.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- file: internal/provider/manifest.go   (S1 — ALREADY EXISTS, COMPLETE; read it, do NOT edit it)
  why: the EXACT Manifest type + field names/tags builtin.go constructs, AND the unexported helpers
       `strPtr(string) *string` / `boolPtr(bool) *bool` (same package — use them directly, no import).
       Also `DefaultPromptDelivery`/`DefaultOutput`/`DefaultStripCodeFence`/`DefaultRetryInstruction`
       constants and `Validate()` (both built-ins must pass it). Confirm field names match exactly:
       Name, Detect, Command, Subcommand, PromptDelivery, PromptFlag, PrintFlag, ModelFlag, DefaultModel,
       SystemPromptFlag, ProviderFlag, DefaultProvider, BareFlags, Output, JsonField, StripCodeFence,
       RetryInstruction, Env.
  pattern: copy S1's doc-comment + value-return style. The constructors are FREE FUNCTIONS (not methods)
       because they take no receiver — `func builtinPi() Manifest`.
  critical: do NOT edit this file. Do NOT rename fields. S1 is a frozen contract. The helpers are
       UNEXPORTED (lowercase) — that is fine because builtin.go is `package provider` (same package).

- docfile: plan/001_f1f80943ac34/P1M2T2S1/research/builtin-manifests-pi-claude.md
  why: the field-by-field value tables (§1), the explicit-empty vs absent map (§2 — THE subtlety), the
       §12.2 render walkthrough producing the exact commit-pi argv (§3), the §12.4 illustrative-order
       discrepancy (§4 — do NOT "fix" the manifest to match §12.4's `-p`-second block), the construction
       approach decision (§5), and the fresh-per-call rationale (§6). The single most important read.
  critical: §2 (explicit-empty: pi.DefaultProvider & claude.ProviderFlag = strPtr(""); absent: claude.
       DefaultProvider + Subcommand/PromptFlag/JsonField/RetryInstruction/Env = nil) and §3 (claude's
       two `""` bare-flag tokens MUST be present) are the two things most likely to be implemented wrong.

- file: PRD.md
  section: "12.3 Built-in provider: pi" (h3.39) — the AUTHORITATIVE pi manifest TOML + its "Rendered"
       block (the commit-pi byte-for-byte target). The TOML block IS the decode-parity fixture.
  why: every pi field value comes from here, verbatim. Note `default_provider = ""` is WRITTEN (explicit
       empty → non-nil) and there is NO `subcommand`/`prompt_flag`/`json_field`/`retry_instruction`/
       `[env]` key (→ nil).
  critical: the rendered block is reproduced by §12.2 (print_flag LAST); it matches commit-pi. This is
       the byte-for-byte check target.

- file: PRD.md
  section: "12.4 Built-in provider: Claude Code" (h3.40) — the AUTHORITATIVE claude manifest TOML.
  why: every claude field value comes from here. Note `provider_flag = "" # n/a` (explicit empty →
       non-nil), there is NO `default_provider` key (→ nil), and `bare_flags` has TWO `""` value tokens.
  critical: §12.4's "Rendered" block shows `-p` SECOND — this is ILLUSTRATIVE and DISAGREES with §12.2
       (which puts print_flag LAST). Do NOT alter the manifest to match §12.4's order; flag order is the
       renderer's (P1.M2.T4) concern. Verify claude via decode-parity + field assertions, not render order.

- file: PRD.md
  section: "12.2 Command rendering algorithm" (h3.38) — the AUTHORITATIVE argv algorithm. Reproduced
       verbatim in Implementation Patterns below for the render test.
  why: the pi render test ports this algorithm into a local test-only argv-builder and asserts the result
       equals the commit-pi argv. This proves the manifest DATA is sufficient to render commit-pi.
  critical: the algorithm is `args=[]; +[provider_flag,provider] if provider_flag&&provider; +[model_flag,
       model] if model_flag&&model; +[sys_prompt_flag,sys] if sys_prompt_flag&&sys; +bare_flags; +[print_flag]
       if print_flag; stdin delivery appends nothing`. print_flag is LAST.

- file: PRD.md
  section: "12.7.1 The tools-disable asymmetry" (h4.0) — the conceptual framing: pi+claude are the
       "explicit tool-disable switch" providers (a literal turn-tools-off switch → pure text-in/text-out,
       no agent loop). This is WHY this subtask groups pi+claude together (the work-item title).
  why: explains the design intent — these two are the cleanest, fastest bare calls, distinct from the
       read-only-constrained providers (codex/cursor/gemini) in S2/S3.

- file: plan/001_f1f80943ac34/architecture/external_deps.md
  section: §pi ("FULLY VERIFIED") + §claude ("VERIFIED") — live `--help` captures (2026-06-29).
  why: independent confirmation of every flag. §pi: all 6 bare flags confirmed, "Rendered command
       (matching commit-pi byte-for-byte)". §claude: `--tools ""` = *"Use \"\" to disable all tools"*;
       `--setting-sources`; `--no-session-persistence` (only with `-p`).
  critical: §codex flags a discrepancy (--ask-for-approval) — that is an S3 concern, NOT this subtask.
       This subtask encodes ONLY pi + claude, both fully verified with no discrepancies.

- file: plan/001_f1f80943ac34/P1M2T1S1/research/go-toml-pointer-behavior.md
  why: FINDING C/D — absent TOML key → nil pointer/slice/map; present key (even `""`/`false`/`[]`) →
       non-nil. This is WHY the literal construction must reproduce the TOML's nil/non-nil pattern exactly
       (design call #2) and why the decode-parity test (which decodes the TOML) is the correct oracle.
  critical: do NOT "simplify" an explicit `strPtr("")` to a nil field (or vice versa) — the decode-parity
       test will catch the mismatch.

- file: internal/provider/manifest_test.go   (S1 — test-style pattern; do NOT edit)
  why: the repo's white-box test convention (`package provider`, stdlib `testing`, `t.Errorf`, table-driven
       where natural). S1's tests already use `strPtr`/`boolPtr` + `reflect.DeepEqual` + `toml.Unmarshal`
       — mirror that exact style in builtin_test.go.

- file: PRD.md
  section: "Appendix D — Built-in manifest quick reference" (h2.27, the table) — a cross-provider
       summary confirming pi/claude essentials (command, delivery, print, model flag, sys-prompt flag,
       bare essentials, output). Useful as a final cross-check that pi+claude values are right.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; require go-toml/v2 v2.4.2 + pflag v1.0.10  (UNCHANGED by this subtask)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — FROZEN, do NOT touch; do NOT import from provider
  git/                          # P1.M1.T2/T3 — untouched
  provider/                     # S1 created the package; S2 adds merge.go; THIS subtask adds builtin.go
    manifest.go                 # S1 — Manifest + Validate + DetectCommand + Resolve + strPtr/boolPtr  (CONTRACT — do NOT edit)
    manifest_test.go            # S1 — tests  (do NOT edit)
    merge.go                    # S2 — MergeManifest  (do NOT edit; may or may not exist yet — this subtask does NOT depend on it)
    merge_test.go               # S2 — tests  (do NOT edit)
    builtin.go                  # NEW (this subtask) ← BuiltinManifests() + builtinPi() + builtinClaude()
    builtin_test.go             # NEW (this subtask) ← 8 test groups
cmd/stagecoach/main.go           # `package main; func main(){}` stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  provider/
    builtin.go                  # NEW — BuiltinManifests() + builtinPi() + builtinClaude() (zero imports)
    builtin_test.go             # NEW — 8 test groups (package provider, white-box)
# manifest.go/manifest_test.go (S1) + merge.go/merge_test.go (S2) UNCHANGED. go.mod/go.sum UNCHANGED.
# Every other file UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call #2 — explicit empty vs absent): go-toml/v2 decodes `x = ""` to a NON-NIL *""
// and an ABSENT key to nil (S1 FINDING C/D). The literal construction MUST mirror the PRD TOML's
// pattern or the decode-parity test (reflect.DeepEqual) fails:
//   pi.DefaultProvider      = strPtr("")   // §12.3 WRITES default_provider = ""  → non-nil empty
//   claude.ProviderFlag      = strPtr("")   // §12.4 WRITES provider_flag = ""     → non-nil empty
//   claude.DefaultProvider   = nil          // §12.4 OMITS the key                  → nil (do NOT set it)
//   both.Subcommand/PromptFlag/JsonField/RetryInstruction/Env = nil  // absent in TOML
// Resolve() turns nil optionals into *""/defaults at consume time, so functionally nil≈*"" AFTER Resolve
// — but the PARITY test compares the UNRESOLVED built-in to the UNRESOLVED decode, where nil ≠ non-nil.

// CRITICAL (design call #3 — claude's two empty bare-flag tokens): BareFlags MUST be
//   []string{"--tools", "", "--setting-sources", "", "--no-session-persistence"}
// The "" tokens are the VALUE args to --tools (disable-all) and --setting-sources (load-none). Dropping
// them → `--tools --setting-sources …` = a different, broken command. external_deps.md §claude confirms:
// `--tools ""` is documented "Use \"\" to disable all tools".

// CRITICAL (design call #1 — zero imports): builtin.go uses ONLY S1's same-package strPtr/boolPtr +
// field assignment + slice/map literals. NO `import` block. If `go vet` complains about an unused import,
// you added one you don't need (fmt? toml?). Remove it. toml lives in the TEST file only (decode-parity).

// CRITICAL (design call #4 — fresh per call): BuiltinManifests() calls builtinPi()/builtinClaude() inside
// the return expression — `return map[string]Manifest{"pi": builtinPi(), "claude": builtinClaude()}`.
// NO `var builtins = …` package-level variable. strPtr + slice literals allocate fresh each call → no
// shared mutable state. A rogue caller mutating a returned BareFlags/Env cannot corrupt future calls.

// GOTCHA (design call #5 — §12.4 render order is illustrative): §12.2 (authoritative) puts print_flag
// LAST; §12.4's rendered block puts -p SECOND. They disagree for claude; they AGREE for pi. The pi
// render test asserts the EXACT commit-pi argv (print_flag last). Do NOT add a manifest field or tweak
// values to make claude render -p early — there is no such field; order is P1.M2.T4's job. Verify claude
// via decode-parity + field assertions.

// GOTCHA: do NOT call Validate/Resolve inside the constructors. The constructors BUILD; the registry
// (P1.M2.T3) runs Validate → Resolve on the (merged) result. A test asserts both built-ins Validate(),
// but the constructors themselves stay pure data.

// GOTCHA: the decode-parity test uses reflect.DeepEqual on Manifest. DeepEqual dereferences pointers
// and compares targets; nil pointers compare equal ONLY to nil. So the test is EXACTLY the right oracle
// for "built-in matches the decoded TOML" — it catches any nil/non-nil or value mismatch.

// GOTCHA: go-toml/v2 marshals nil pointer fields as OMITTED and nil slices as `[]` (S1 FINDING A/B).
// Irrelevant here (we decode, never marshal, the built-ins) — but if you ever assert on marshaled output,
// tolerate `subcommand = []`. Not needed for this subtask.

// GOTCHA: the embedded TOML in the decode-parity test is a Go raw string literal (backticks). The PRD
// TOML contains `"` and `#` (comment chars) — both are fine inside backticks. The TOML contains NO
// backticks, so a raw string is safe. Strip the `# ...` comments OR keep them (go-toml ignores comments)
// — either decodes identically; keeping them makes the test self-documenting.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/provider/builtin.go
package provider

// (NO imports — literal construction via same-package strPtr/boolPtr only.)

// BuiltinManifests returns the compiled-in default provider manifests (PRD §12.3 pi + §12.4 claude),
// keyed by manifest name. These are the zero-config defaults a user override (config [provider.<name>])
// merges onto via MergeManifest (S2) in the registry (P1.M2.T3).
//
// The manifests are constructed FRESH on every call (no package-level var): strPtr/boolPtr allocate new
// pointers and slice literals allocate new backing arrays each call, so no caller can corrupt a built-in
// by mutating a returned BareFlags/Env. MergeManifest (S2) already never mutates base, so the normal
// registry path is safe either way — fresh-per-call additionally guards against any direct mutation.
//
// This subtask lands pi + claude (the §12.7.1 "explicit tool-disable switch" providers). The remaining
// four (gemini, opencode, codex, cursor — the read-only-constrained providers) are added by S2/S3.
func BuiltinManifests() map[string]Manifest {
	return map[string]Manifest{
		"pi":     builtinPi(),
		"claude": builtinClaude(),
	}
}

// builtinPi returns the pi manifest per PRD §12.3 (FULLY VERIFIED vs `pi --help`, external_deps.md §pi).
// Rendered with provider="zai", model=default, sys set, it reproduces the commit-pi invocation
// byte-for-byte (see TestBuiltinManifests_RenderedCommand_Pi_MatchesCommitPi).
//
// NOTE the explicit-empty DefaultProvider: §12.3 WRITES `default_provider = ""` (non-nil *""), meaning
// "do not add --provider unless the user configures one." This is NOT the same as a nil DefaultProvider
// (absent key) — the decode-parity test enforces the distinction.
func builtinPi() Manifest {
	return Manifest{
		Name:             "pi",
		Detect:           strPtr("pi"),
		Command:          strPtr("pi"),
		PromptDelivery:   strPtr("stdin"),
		PrintFlag:        strPtr("-p"),
		ModelFlag:        strPtr("--model"),
		DefaultModel:     strPtr("glm-5-turbo"),
		SystemPromptFlag: strPtr("--system-prompt"),
		ProviderFlag:     strPtr("--provider"),
		DefaultProvider:  strPtr(""), // §12.3 explicit empty (NON-NIL) — user sets e.g. "zai"
		BareFlags: []string{
			"--no-tools",
			"--no-extensions",
			"--no-skills",
			"--no-prompt-templates",
			"--no-context-files",
			"--no-session",
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// Subcommand, PromptFlag, JsonField, RetryInstruction, Env: nil (absent in §12.3).
	}
}

// builtinClaude returns the claude manifest per PRD §12.4 (VERIFIED vs `claude --help`, external_deps.md
// §claude). claude disables tools via `--tools ""` (documented "Use \"\" to disable all tools") and
// settings via `--setting-sources ""`; `--no-session-persistence` makes it ephemeral (valid only with -p).
//
// NOTE: (1) ProviderFlag is strPtr("") — §12.4 WRITES `provider_flag = "" # n/a` (non-nil empty); the
// §12.2 renderer's `if provider_flag and provider` is therefore false → no --provider emitted (claude has
// no sub-provider concept). (2) DefaultProvider is NIL — §12.4 OMITS the key entirely (do NOT set it).
// (3) BareFlags has TWO "" value tokens (the args to --tools / --setting-sources) — do NOT drop them.
func builtinClaude() Manifest {
	return Manifest{
		Name:             "claude",
		Detect:           strPtr("claude"),
		Command:          strPtr("claude"),
		PromptDelivery:   strPtr("stdin"),
		PrintFlag:        strPtr("-p"),
		ModelFlag:        strPtr("--model"),
		DefaultModel:     strPtr("sonnet"),
		SystemPromptFlag: strPtr("--system-prompt"),
		ProviderFlag:     strPtr(""), // §12.4 explicit empty (NON-NIL) — n/a for claude
		BareFlags: []string{
			"--tools", "", // disable ALL built-in tools (value arg = "")
			"--setting-sources", "", // load no settings sources (value arg = "")
			"--no-session-persistence", // ephemeral (only valid with -p)
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// Subcommand, PromptFlag, JsonField, RetryInstruction, Env, DefaultProvider: nil (absent in §12.4).
	}
}
```

> **gofmt note:** run `gofmt -w internal/provider/builtin.go internal/provider/builtin_test.go`. Do not
> hand-align. One doc comment per function (citing the PRD section + the explicit-empty notes) is
> encouraged — it seeds the `providers show` / reference-file docs later.
>
> **Imports:** `builtin.go` has NONE. `builtin_test.go` imports `testing` + `reflect` +
> `github.com/pelletier/go-toml/v2` (decode-parity; already in go.mod, test-only). NO go.mod change.

### The verbatim PRD TOML for the decode-parity test

```go
// internal/provider/builtin_test.go — embed these EXACTLY (comments optional; go-toml ignores them).
// These are PRD §12.3 (pi) and §12.4 (claude), byte-for-byte. Decoding them is the oracle the built-in
// must match (reflect.DeepEqual).

const piTOML = `name = "pi"
detect = "pi"
command = "pi"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "--model"
default_model = "glm-5-turbo"
system_prompt_flag = "--system-prompt"
provider_flag = "--provider"
default_provider = ""
bare_flags = [
  "--no-tools",
  "--no-extensions",
  "--no-skills",
  "--no-prompt-templates",
  "--no-context-files",
  "--no-session",
]
output = "raw"
strip_code_fence = true
`

const claudeTOML = `name = "claude"
detect = "claude"
command = "claude"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "--model"
default_model = "sonnet"
system_prompt_flag = "--system-prompt"
provider_flag = ""
bare_flags = [
  "--tools", "",
  "--setting-sources", "",
  "--no-session-persistence",
]
output = "raw"
strip_code_fence = true
`
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/provider/builtin.go — BuiltinManifests + builtinPi + builtinClaude
  - IMPLEMENT the three functions per the Data Models block. Use S1's strPtr/boolPtr (same package).
  - pi fields: per the pi table; DefaultProvider=strPtr("") (non-nil empty); absent fields nil.
  - claude fields: per the claude table; ProviderFlag=strPtr("") (non-nil empty); DefaultProvider nil;
      BareFlags=[--tools,"",--setting-sources,"",--no-session-persistence] (5 tokens, two "").
  - BuiltinManifests returns map[string]Manifest{"pi": builtinPi(), "claude": builtinClaude()} (fresh call).
  - IMPORTS: NONE. If `go vet` flags an unused import, remove it.
  - GOTCHA: do NOT add Validate/Resolve calls; do NOT add a package-level var; do NOT set absent fields.
  - WHY ONE TASK: it's ~3 small functions of literal data. Splitting adds no value and risks a
      half-written file that doesn't compile.

Task 2: CREATE internal/provider/builtin_test.go — the 8 test groups
  - PACKAGE: `package provider` (white-box — uses strPtr/boolPtr + reflect + toml). Mirror manifest_test.go.
  - EMBED the piTOML / claudeTOML constants (above) verbatim.
  - ADD a local renderArgs helper (§12.2 port, test-only) for the pi render test — see Implementation
      Patterns. Keep it faithful to §12.2; it is THROWAWAY test scaffolding, NOT the P1.M2.T4 renderer.
  - TEST TestBuiltinManifests_KeysAndCount: BuiltinManifests() returns exactly 2 keys, "pi" and "claude"
      (use a set check; assert len==2 and both present, no extras).
  - TEST TestBuiltinManifests_NameMatchesKey: builtins["pi"].Name=="pi" && builtins["claude"].Name=="claude".
  - TEST TestBuiltinManifests_PiFields: assert EVERY pi field (Detect/Command/PromptDelivery/PrintFlag/
      ModelFlag/DefaultModel/SystemPromptFlag/ProviderFlag non-nil with the right value; DefaultProvider
      NON-NIL with *==""; BareFlags reflect.DeepEqual the 6-token slice IN ORDER; Output=="raw";
      StripCodeFence non-nil *==true) AND the absent fields nil (Subcommand==nil, PromptFlag==nil,
      JsonField==nil, RetryInstruction==nil, Env==nil). This pins the explicit-empty + absent pattern.
  - TEST TestBuiltinManifests_ClaudeFields: assert EVERY claude field (ProviderFlag NON-NIL *=="";
      DefaultProvider==nil [ABSENT]; BareFlags reflect.DeepEqual [--tools,"",--setting-sources,"",
      --no-session-persistence] IN ORDER — verifies the two "" tokens survive; DefaultModel=="sonnet";
      etc.) AND the absent fields nil (Subcommand/PromptFlag/JsonField/RetryInstruction/Env).
  - TEST TestBuiltinManifests_Validate: builtins["pi"].Validate()==nil AND builtins["claude"].Validate()==nil.
  - TEST TestBuiltinManifests_DecodeParity (THE BYTE-FAITHFULNESS KEYSTONE): for each of pi/claude,
      toml.Unmarshal(<TOML>) into a Manifest; assert reflect.DeepEqual(builtin, decoded). This proves the
      built-in is indistinguishable from decoding the PRD TOML (nil/non-nil pattern + values exact). If it
      fails, a literal transcription is wrong (most likely an explicit-empty set to nil, or vice versa).
  - TEST TestBuiltinManifests_RenderedCommand_Pi_MatchesCommitPi (THE BYTE-FOR-BYTE COMMIT-PI CHECK):
      pi := builtinPi(); argv := renderArgs(pi, provider="zai", model="", sys="<sys>") (model="" → use
      pi.DefaultModel); assert argv == []string{"pi","--provider","zai","--model","glm-5-turbo",
      "--system-prompt","<sys>","--no-tools","--no-extensions","--no-skills","--no-prompt-templates",
      "--no-context-files","--no-session","-p"}. (model="" exercises the default_model path.)
  - TEST TestBuiltinManifests_FreshEachCall: a := BuiltinManifests(); b := BuiltinManifests();
      mutate a["pi"].BareFlags (append or index-assign) and a["pi"].Env (if you set one) — assert b["pi"]
      is UNCHANGED (no shared backing array/map). Pins design call #4.

Task 3: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. S1's manifest.go/
      manifest_test.go AND S2's merge.go/merge_test.go MUST be byte-unchanged. S1 + S2 tests MUST stay
      green (no field/type/import change). The config + git suites MUST stay green (no import edge).
```

### Implementation Patterns & Key Details

```go
// The §12.2 argv-builder (test-only, in builtin_test.go). Faithful port of PRD §12.2. This is NOT the
// P1.M2.T4 renderer — it is throwaway scaffolding whose ONLY purpose is to prove the pi manifest renders
// to the commit-pi invocation. Keep it literal and obviously a port.
func renderArgs(m Manifest, provider, model, sys string) []string {
	r := m.Resolve() // safe deref for every pointer
	modelToUse := model
	if modelToUse == "" && r.DefaultModel != nil {
		modelToUse = *r.DefaultModel // §12.2 "model" is the resolved model (passed OR default)
	}
	args := []string{}
	args = append(args, r.Subcommand...)            // nil-safe no-op
	if *r.ProviderFlag != "" && provider != "" {    // §12.2: "if m.provider_flag and provider"
		args = append(args, *r.ProviderFlag, provider)
	}
	if *r.ModelFlag != "" && modelToUse != "" {      // §12.2: "if m.model_flag and model"
		args = append(args, *r.ModelFlag, modelToUse)
	}
	if *r.SystemPromptFlag != "" && sys != "" {      // §12.2: "if m.system_prompt_flag and sys"
		args = append(args, *r.SystemPromptFlag, sys)
	}
	args = append(args, r.BareFlags...)              // §12.2: "args += m.bare_flags"
	if *r.PrintFlag != "" {                          // §12.2: "if m.print_flag"
		args = append(args, *r.PrintFlag)
	}
	// prompt_delivery "stdin" → nothing appended (payload via stdin). pi/claude are both stdin.
	return append([]string{*r.Command}, args...)
}

// The keystone assertion — pi renders to commit-pi byte-for-byte (print_flag LAST per §12.2):
func TestBuiltinManifests_RenderedCommand_Pi_MatchesCommitPi(t *testing.T) {
	argv := renderArgs(builtinPi(), "zai", "", "<sys>") // model="" → default glm-5-turbo
	want := []string{
		"pi", "--provider", "zai",
		"--model", "glm-5-turbo",
		"--system-prompt", "<sys>",
		"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session",
		"-p", // §12.2: print_flag LAST (matches §12.3 + commit-pi)
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("pi rendered argv:\n got %v\nwant %v", argv, want)
	}
}

// The decode-parity keystone — built-in == decode(PRD TOML). Catches ANY transcription error, esp. the
// explicit-empty vs absent nil/non-nil pattern (reflect.DeepEqual treats nil ≠ non-nil).
func TestBuiltinManifests_DecodeParity(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  Manifest
		toml string
	}{
		{"pi", builtinPi(), piTOML},
		{"claude", builtinClaude(), claudeTOML},
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
```

```go
// The fresh-each-call guard — pins design call #4. If this fails, a package-level var crept in (shared
// backing array). strPtr + slice literals allocate fresh each call, so BuiltinManifests() has no shared
// state ONLY if it constructs inline (no var).
func TestBuiltinManifests_FreshEachCall(t *testing.T) {
	a := BuiltinManifests()
	b := BuiltinManifests()
	// Mutate a's pi BareFlags backing array in place.
	if len(a["pi"].BareFlags) > 0 {
		a["pi"].BareFlags[0] = "MUTATED"
	}
	// b's pi must be unaffected.
	want := []string{"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session"}
	if !reflect.DeepEqual(b["pi"].BareFlags, want) {
		t.Errorf("BuiltinManifests() shares state across calls (b corrupted by a): got %v", b["pi"].BareFlags)
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. builtin.go has zero imports; builtin_test.go uses testing + reflect + go-toml/v2
        (test-only, already in go.mod). `go mod tidy` MUST be a no-op. `git diff --exit-code go.mod
        go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - internal/provider → (stdlib: NONE in builtin.go; testing+reflect+toml in builtin_test.go) ONLY.
  - internal/provider → internal/config : FORBIDDEN (cycle; same as S1/S2). The REGISTRY (P1.M2.T3) is
        the sole importer of both config and provider.
  - internal/provider → github.com/pelletier/go-toml/v2 : test-only (builtin_test.go decode-parity).

FROZEN FILES (do NOT edit):
  - internal/provider/manifest.go + manifest_test.go (S1): the Manifest type + strPtr/boolPtr + Validate
        are a CONTRACT. This subtask ADDS builtin.go/builtin_test.go; it does not modify S1's files.
  - internal/provider/merge.go + merge_test.go (S2): MergeManifest. This subtask does NOT depend on S2
        (it depends only on S1's Manifest type); do NOT edit S2's files (they may still be in flight).
  - internal/config/* (P1.M1.T4), internal/git/* (P1.M1.T2/T3), cmd/stagecoach/main.go, Makefile.

DOWNSTREAM CONTRACTS (do NOT implement here — just honor the shapes they will consume):
  - P1.M2.T3 (registry): `builtins := BuiltinManifests(); base := builtins[<name>]; merged := MergeManifest(base,
        decode(reencode(config.Providers[<name>])))`. For a brand-new §12.8 provider, base is the zero
        Manifest (NOT in this map). This subtask provides ONLY pi + claude as bases.
  - P1.M2.T4 (renderer): reads the RESOLVED manifest per §12.2 — the pi render test proves the data is
        sufficient to render commit-pi. The real renderer will have its OWN tests; renderArgs here is
        throwaway.
  - P1.M2.T5 (executor): reads *resolved.Command ("pi"/"claude") + resolved.Env (nil → none).
  - P1.M2.T6 (parser): reads *resolved.Output ("raw"), *resolved.JsonField (""), *resolved.StripCodeFence (true).
  - P1.M2.T2.S2/S3 (sibling subtasks): will ADD gemini/opencode/codex/cursor constructors and extend
        BuiltinManifests()'s map. Design the map-returning function so adding entries is a one-line change
        (it already is: `return map[string]Manifest{"pi": …, "claude": …}` — S2/S3 append their keys).
  => BuiltinManifests() signature + the pi/claude field values are now FROZEN for downstream. Do not
     change them after this subtask.

NO DATABASE / NO ROUTES / NO CLI / NO RENDERER/EXECUTOR/PARSER / NO REGISTRY / NO OTHER PROVIDERS /
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
# Expected: clean. ZERO imports in builtin.go (verify): the only `import` lines should be in builtin_test.go.
grep -n '^import\|^	"' internal/provider/builtin.go && echo "note: builtin.go has imports (should be NONE)" || echo "builtin.go zero-imports (good)"

# Confirm NO new dependency + NO edit to S1/S2 files + no config edge:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"   # MUST be empty.
git diff --exit-code internal/provider/manifest.go internal/provider/manifest_test.go internal/provider/merge.go internal/provider/merge_test.go && echo "S1+S2 files UNCHANGED (expected)"   # MUST be empty.
grep -n 'internal/config' internal/provider/builtin.go && echo "BAD: config import" || echo "no config import (good)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 8 test groups (white-box; no git/exec/config needed — pure literal data + toml round-trip + §12.2 port):
go test -race ./internal/provider/ -v
# Expected: PASS — TestBuiltinManifests_KeysAndCount, TestBuiltinManifests_NameMatchesKey,
#   TestBuiltinManifests_PiFields (explicit-empty DefaultProvider + absent fields nil),
#   TestBuiltinManifests_ClaudeFields (explicit-empty ProviderFlag, nil DefaultProvider, two "" tokens),
#   TestBuiltinManifests_Validate (both nil), TestBuiltinManifests_DecodeParity (THE keystone),
#   TestBuiltinManifests_RenderedCommand_Pi_MatchesCommitPi (byte-for-byte commit-pi),
#   TestBuiltinManifests_FreshEachCall (no shared state) — PLUS S1's + S2's tests still green.

# Full suite must stay green (no regression; confirms no stray import edge broke config/git):
go test -race ./...
# Expected: all packages PASS (config, git, provider).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build + scope checks:
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"   # main.go stub still links.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged"
# Confirm this subtask touched ONLY the two new files:
git diff --exit-code -- internal/config internal/git cmd Makefile internal/provider/manifest.go internal/provider/manifest_test.go internal/provider/merge.go internal/provider/merge_test.go && echo "frozen + S1 + S2 files UNCHANGED by this subtask"
grep -n 'func BuiltinManifests' internal/provider/builtin.go   # MUST print the function line.
# Expected: binary builds; go.mod/go.sum unchanged; frozen+S1+S2 files unchanged; BuiltinManifests present.

# Coverage of the new file (Makefile has a coverage target):
go test -race ./internal/provider/ -coverprofile=/tmp/cov.out && go tool cover -func=/tmp/cov.out | grep -E 'builtin|BuiltinManifests'
# Expected: builtin.go at (or near) 100% line coverage — both constructors + the map literal exercised by
# the Keys/Fields/Validate/DecodeParity/Render/Fresh tests. (make coverage runs the project-wide gate; the
# ≥85% target is enforced at P1.M5.T3.S3.)

# Smoke the commit-pi equivalence directly (the work-item's headline requirement):
go test -race ./internal/provider/ -run TestBuiltinManifests_RenderedCommand_Pi_MatchesCommitPi -v
# Expected: PASS — the pi manifest renders to the exact commit-pi argv.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Lint gate (project-wide):
golangci-lint run ./internal/provider/ 2>/dev/null || echo "golangci-lint not installed (Makefile lint gate is project-wide; run `make lint` in CI)."

# Cross-check against Appendix D (h2.27) quick-reference table by eye: pi row (command=pi, delivery=stdin,
# print=-p, model flag=--model, sys-prompt=--system-prompt, bare essentials=the 6 --no-* flags, output=raw)
# and claude row (command=claude, ... bare essentials=--tools "" --setting-sources "" --no-session-persistence,
# output=raw). The decode-parity + field tests already assert these mechanically; this is a human sanity pass.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` is a
      no-op; `git diff --exit-code go.mod go.sum` empty; `builtin.go` has ZERO imports.
- [ ] Level 2 green: `go test -race ./internal/provider/ -v` (all 8 builtin groups + S1's + S2's tests)
      AND `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; `manifest.go`/`manifest_test.go` (S1) +
      `merge.go`/`merge_test.go` (S2) unchanged; every file outside the two new `builtin*.go` files
      unchanged; `BuiltinManifests` present.

### Feature Validation

- [ ] `BuiltinManifests() map[string]Manifest` exists, returns exactly `{"pi", "claude"}`.
- [ ] pi manifest: every field per §12.3; `DefaultProvider` is NON-NIL `*""` (explicit empty); `Subcommand`/
      `PromptFlag`/`JsonField`/`RetryInstruction`/`Env` nil; `BareFlags` is the 6-token slice in order.
- [ ] claude manifest: every field per §12.4; `ProviderFlag` is NON-NIL `*""`; `DefaultProvider` nil
      (absent); `BareFlags` is `["--tools","","--setting-sources","","--no-session-persistence"]` (two `""`
      tokens present); same nil-absent set as pi.
- [ ] Both `Validate()` → nil.
- [ ] `reflect.DeepEqual(builtin, decode(PRD TOML))` holds for both (decode-parity keystone).
- [ ] pi rendered via §12.2 == commit-pi argv byte-for-byte (the headline requirement).
- [ ] Fresh-per-call: no shared mutable state across calls.

### Code Quality Validation

- [ ] Follows repo conventions: white-box `package provider`, stdlib `testing`, `t.Errorf`, `reflect.DeepEqual`
      (mirrors `manifest_test.go`); free-function constructors; zero imports in `builtin.go`.
- [ ] File placement matches the desired tree (`builtin.go` + `builtin_test.go` only — S1/S2 untouched).
- [ ] The explicit-empty vs absent pattern is reproduced exactly (pi.DefaultProvider & claude.ProviderFlag
      non-nil empty; claude.DefaultProvider & the shared absent set nil) — NOT flattened.
- [ ] `internal/provider` production code still imports nothing outside stdlib (S1/S2 discipline preserved);
      `builtin.go` imports NOTHING.
- [ ] No premature scope: no registry (P1.M2.T3), no Validate/Resolve call inside constructors, no
      renderer/exec/parse (T4/T5/T6), no other providers (S2/S3), no `providers/*.toml` (P1.M5.T2).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Doc comment on `BuiltinManifests` + each constructor citing the PRD section + the explicit-empty
      notes (seeds `providers show` / reference-file docs later).
- [ ] No new env vars / no user-facing docs (PRD "DOCS: none — compiled-in defaults"; public docs come
      with `providers show` P1.M4.T1.S3 and reference files P1.M5.T2).
- [ ] `internal/provider/builtin.go` + `builtin_test.go` are the ONLY files touched.

---

## Anti-Patterns to Avoid

- ❌ Don't decode TOML at runtime in `builtin.go`. Use literal `strPtr`/`boolPtr` construction (design call
  #1) so the file stays import-free and go.mod unchanged. The decode-parity TEST delivers the "matches PRD
  TOML" guarantee; the production code carries no toml dependency.
- ❌ Don't flatten the explicit-empty vs absent distinction. `pi.DefaultProvider` MUST be `strPtr("")`
  (non-nil, because §12.3 writes `default_provider = ""`); `claude.DefaultProvider` MUST be nil (absent in
  §12.4). `claude.ProviderFlag` MUST be `strPtr("")` (§12.4 writes `provider_flag = ""`). Swapping these
  fails the decode-parity test (reflect.DeepEqual: nil ≠ non-nil). Resolve() makes them behave the same
  AFTER resolve, but the UNRESOLVED built-in must match the UNRESOLVED decode.
- ❌ Don't drop claude's two `""` bare-flag tokens. `["--tools","","--setting-sources","",
  "--no-session-persistence"]` — the `""` are the VALUE args (disable-all / load-none). Without them the
  command is `--tools --setting-sources …` = broken.
- ❌ Don't "fix" the manifest to make claude render `-p` second (matching §12.4's illustrative block).
  §12.2 (authoritative) puts print_flag last; §12.4's rendered block is hand-wavy on order and the two
  disagree for claude (they agree for pi). Flag order is the renderer's (P1.M2.T4) concern; the manifest
  only supplies flags+values. Verify claude via decode-parity + field assertions, not render order.
- ❌ Don't use a package-level `var` for the built-ins. Construct fresh in `BuiltinManifests()` (design
  call #4) so no caller can corrupt a shared backing array/map. `TestBuiltinManifests_FreshEachCall`
  catches a shared-state regression.
- ❌ Don't edit `manifest.go`/`manifest_test.go` (S1) or `merge.go`/`merge_test.go` (S2) — they are frozen
  contracts. This subtask ADDS `builtin.go` + `builtin_test.go`. If you think a field is missing from
  Manifest, that's an S1 issue, not this one.
- ❌ Don't add imports to `builtin.go`. It is pure literal data (`strPtr`/`boolPtr` + slice/map literals).
  An unused import fails `go vet`. `builtin_test.go` may import `testing`/`reflect`/`toml`.
- ❌ Don't change go.mod/go.sum — no new dep. An unintended `go get`/`go mod tidy` mutation means an import
  crept into `builtin.go`; remove the import, don't add the dep.
- ❌ Don't implement the registry (P1.M2.T3), the real renderer (P1.M2.T4), exec/parse (T5/T6), the other
  four providers (S2/S3), or the `providers/*.toml` reference files (P1.M5.T2). The §12.2 `renderArgs` in
  the test is THROWAWAY scaffolding to prove the pi manifest data — it is NOT the renderer.
- ❌ Don't call `Validate()`/`Resolve()` inside the constructors. They build pure data; the registry runs
  Validate → Resolve on the merged result. A test asserts both built-ins Validate(), but the constructors
  stay pure.
- ❌ Don't add gemini/opencode/codex/cursor here. `BuiltinManifests()` returns ONLY pi + claude in this
  subtask. S2/S3 extend the same map-returning function with their constructors.
