---
name: "P1.M2.T1.S2 — Field-by-field manifest merge for user overrides"
description: |
  Land the SECOND subtask of Provider Manifest Schema & Merge Logic (P1.M2.T1): a pure, side-effect-free
  `MergeManifest(base, override Manifest) Manifest` function in `internal/provider` that overlays a
  PARTIAL user override onto a built-in manifest **field-by-field** per PRD §16.1 ("a user override that
  sets only `default_model` leaves all other fields from the built-in manifest intact"). It builds
  DIRECTLY on the `Manifest` struct + `strPtr`/`boolPtr` helpers landed by S1 (`internal/provider/manifest.go`,
  implemented in parallel — read as a contract). This is the merge step the registry (P1.M2.T3) calls
  between "decode config.Providers[<name>] into a partial Manifest" and "Validate → Resolve → hand to
  renderer/executor/parser" (the frozen `config.go` Providers comment names this exact step). It is NOT
  the registry itself, NOT the built-in manifest VALUES (P1.M2.T2), NOT render/exec/parse (T4/T5/T6).

  ⚠️ **THE central design call — three merge regimes by field kind (from the work-item contract):**
    (1) Scalar pointer fields (`*string`/`*bool` — 14 of them): `override.Field != nil` → result takes
        override.Field. An EXPLICIT `""` or `false` OVERRIDES (non-nil is the override signal — this is
        the entire reason S1 made the optional scalars pointers; a present zero value is a deliberate
        override, not an absence). This is what makes `strip_code_fence = false` / `print_flag = ""`
        override correctly.
    (2) Slices (`Subcommand`, `BareFlags`): `len(override.Slice) > 0` → result REPLACES base wholesale
        (NO element-level merge). Empty/nil override slice → keep base (a written `bare_flags = []` is
        treated as "not overridden" — the contract's literal "non-empty" wording).
    (3) The `Env` map: merge KEY-BY-KEY — each key in override.Env overwrites the same base key; base
        keys absent from override survive. nil/empty override → keep base.
  These three regimes are the literal contract ("overlay only non-zero/non-empty fields … For slices,
  if override is non-empty, replace entirely. For Env map, merge key-by-key").

  ⚠️ **THE second design call — `Name` is deliberately NOT field-merged; `result.Name = base.Name`.**
  `name` is the `[provider.<name>]` TABLE KEY, never written into the table body, so a decoded override
  always has `Name == ""`. The registry (P1.M2.T3) owns setting the final `Name` from the table key (it
  must, for brand-new §12.8 providers where `base` is the zero `Manifest`). Keeping Name out of the
  field-merge makes MergeManifest a pure, predictable overlay. Pinned by `TestMergeManifest_NamePreservedFromBase`.

  ⚠️ **THE third design call — the map-aliasing bug MUST be avoided (the one real correctness trap).**
  `out := base` copies the struct HEADER, so `out.Env` aliases the caller's `base.Env` map (reference
  type). A naive `out.Env[k] = v` would MUTATE the caller's `base.Env` — a silent, nasty side effect.
  Slices are safe (we only reassign the header on non-empty override, never mutate the shared backing
  array), but Env must be handled by allocating a FRESH map and copying both base + override keys into
  it. `TestMergeManifest_DoesNotMutateInputs` is the test that catches a forgotten fresh-map step. See
  `research/merge-semantics-and-aliasing.md` §2.

  ⚠️ **THE fourth design call — MergeManifest does NOT call Validate.** A partial override legitimately
  lacks `Command` (it inherits the built-in's); Validate is the REGISTRY's post-merge step (lifecycle
  decode→merge→Validate→Resolve→consume). One test asserts a fully-merged result passes Validate
  (proving S2 composes with S1), but the function itself stays pure: it merges, it does not judge.

  ⚠️ **THE fifth design call — NEW FILE, do NOT touch S1's files.** S2 creates `internal/provider/merge.go`
  + `merge_test.go`. It does NOT edit `manifest.go` or `manifest_test.go` (S1's deliverable). The
  function is a free function `MergeManifest(base, override Manifest) Manifest` (not a method) exactly
  as the contract names it — it reads naturally at the call site: `merged := MergeManifest(builtin, userOverride)`.
  `merge.go` is pure logic → ZERO imports (field assignment, `len`, `for range`, `make` only) → stays
  stdlib-only, consistent with S1's design call #4. `merge_test.go` imports `testing` + `reflect`
  (+ optionally `go-toml/v2` for one decoded-fixture test, already in go.mod). NO go.mod/go.sum change.

  Deliverable: `internal/provider/merge.go` (`package provider`) — `func MergeManifest(base, override
  Manifest) Manifest` implementing the three regimes above; and `internal/provider/merge_test.go`
  (`package provider`, white-box) — ~10 test groups covering partial override (the §16.1 keystone:
  only the touched field changes), explicit-zero-pointer override (`false`/`""` win), slice wholesale
  replace vs preserve, Env key-by-key merge, the no-mutate-inputs (aliasing) guarantee, empty-override
  identity, Name-preserved, and merged-result-Validates. INPUT = S1's `Manifest` type (already present
  in `internal/provider/manifest.go`). Touches ONLY `internal/provider/merge.go` + `merge_test.go` —
  NO go.mod/go.sum change, NO edit to `manifest.go`/`manifest_test.go` or any other file. OUTPUT = the
  merge function the registry (P1.M2.T3) calls to turn `[provider.<name>]` overrides into resolved
  manifests for P1.M2.T4 (render) / P1.M2.T5 (exec) / P1.M2.T6 (parse).
---

## Goal

**Feature Goal**: Implement `MergeManifest(base, override Manifest) Manifest` — a pure, side-effect-free,
field-by-field overlay that lets a user config override SOME manifest fields while keeping ALL other
fields from the built-in manifest intact, per PRD §16.1. It is the merge step between config decoding
and Validate/Resolve in the provider-manifest lifecycle.

**Deliverable**:
1. **CREATE** `internal/provider/merge.go` (`package provider`, ZERO imports) — `func MergeManifest(base,
   override Manifest) Manifest` implementing the three regimes: scalar pointer `!= nil` → override
   (explicit `""`/`false` wins); slice `len > 0` → wholesale replace; Env map → key-by-key merge into a
   FRESH map (no input mutation). `Name` preserved from `base` (not merged).
2. **CREATE** `internal/provider/merge_test.go` (`package provider`, white-box) — the ~10 test groups
   listed in Implementation Tasks, all passing. Uses S1's unexported `strPtr`/`boolPtr` helpers
   (accessible in-package) to build manifests; uses `reflect.DeepEqual` for slice/map comparison and
   field-deref comparison for pointers.

No other files touched. **No go.mod/go.sum change** (no new dep — `merge.go` has zero imports; any toml
use in tests is already declared). NO edit to `manifest.go`/`manifest_test.go` (S1). No registry
(P1.M2.T3), no built-in manifest values (P1.M2.T2), no render/exec/parse (P1.M2.T4/T5/T6).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/` clean;
`go mod tidy` is a no-op (no new dep); `go test -race ./internal/provider/ -v` passes (S1's tests STILL
green + all new merge tests green) and the full suite `go test -race ./...` stays green; the §16.1
canonical example works — merging `{default_model="glm-5.2"}` onto the pi built-in changes ONLY
`default_model` and leaves `bare_flags`/`command`/`print_flag`/… untouched; an explicit
`strip_code_fence=false` / `print_flag=""` override is correctly applied (not clobbered by the built-in's
true/`-p`); the Env map is merged key-by-key and the inputs are NEVER mutated.

## User Persona

**Target User**: The registry (P1.M2.T3) — its frozen `config.go` comment says: "for each name it
re-encodes the entry to TOML and unmarshals into a Manifest, then **field-merges with the built-in
manifest per PRD §16.1**." This subtask IS that field-merge. Transitively, every user story routed
through "call an agent" (US) and FR34 (config precedence) + FR36/FR37 (provider management).

**Use Case**: A user drops `[provider.pi]\ndefault_model = "glm-5.2"` into `~/.config/stagehand/config.toml`.
The registry decodes that partial TOML into a `Manifest` (only `DefaultModel` non-nil), then calls
`merged := MergeManifest(builtinPi, override)`. The renderer/executor/parser then consume the MERGED
manifest (with pi's `bare_flags`/`command`/etc. intact and only `default_model` swapped).

**User Journey**: (internal API) `[provider.<name>]` in a config file → registry decodes to partial
`Manifest` → **`MergeManifest(builtin, partial)`** (THIS subtask) → `Validate()` → `Resolve()` →
renderer builds argv / executor runs / parser cleans output.

**Pain Points Addressed**: Removes "how do I override ONE provider field without rewriting the whole
manifest / how does a `strip_code_fence = false` override not get silently dropped / does merging mutate
my inputs" ambiguity for the registry by landing one pure, well-tested function now.

## Why

- **The merge is the heart of §16.1.** PRD §16.1's resolution order explicitly says "Provider manifests
  merge field-by-field (a user override that sets only `default_model` leaves all other fields from the
  built-in manifest intact)." This subtask makes that sentence executable.
- **Unlocks the registry (P1.M2.T3).** The registry is the sole importer of both `config` and
  `provider`; it cannot be written until `MergeManifest` exists. Landing it now lets P1.M2.T3 be
  implemented + tested against a stable target.
- **Proves the pointer design pays off.** S1 chose pointer scalars specifically so THIS merge can
  distinguish "override absent" (nil → inherit) from "override to zero" (non-nil → apply, even `""`/`false`).
  The `TestMergeManifest_ExplicitZeroPointerWins` test is the payoff — it would be IMPOSSIBLE to pass
  with plain `bool`/`string` fields (they'd silently drop `false`/`""`).
- **No user-facing surface change** (PRD "DOCS: none — internal logic"). Manifest override docs arrive
  with `providers show` (P1.M4.T1.S3) and the reference files (P1.M5.T2).
- **No new dependency, no new import edge.** `merge.go` is pure logic (zero imports); the package stays
  stdlib-only; `go.mod` is unchanged.

## What

A compiled `internal/provider` package exporting `MergeManifest` (in addition to S1's `Manifest` +
`Validate` + `DetectCommand` + `Resolve`). Pure function, three merge regimes, side-effect-free. No
registry, no rendering, no execution, no parsing, no built-in manifest content.

### Success Criteria

- [ ] `internal/provider/merge.go` exists, `package provider`, imports NOTHING (zero import lines —
      pure logic). It does NOT import `internal/config`, `go-toml/v2`, `fmt`, or anything.
- [ ] `func MergeManifest(base, override Manifest) Manifest` exists (free function, value params,
      value return — exactly the contract signature).
- [ ] **Scalar pointer fields** (Detect, Command, PromptDelivery, PromptFlag, PrintFlag, ModelFlag,
      DefaultModel, SystemPromptFlag, ProviderFlag, DefaultProvider, Output, JsonField,
      StripCodeFence, RetryInstruction): for each, `if override.Field != nil { out.Field = override.Field }`.
      An explicit non-nil `*""` or `*false` OVERRIDES (the keystone).
- [ ] **Slice fields** (Subcommand, BareFlags): for each, `if len(override.Field) > 0 { out.Field = override.Field }`
      (wholesale replace on non-empty; keep base on nil/empty).
- [ ] **Env map**: `if len(override.Env) > 0` → allocate a FRESH `map[string]string`, copy every
      `base.Env` key into it, then every `override.Env` key (override wins per key), assign to `out.Env`.
      MUST NOT mutate `base.Env` or `override.Env`.
- [ ] **Name**: `result.Name == base.Name` (Name is NOT field-merged — no `override.Name` logic).
- [ ] MergeManifest does NOT call `Validate`/`Resolve` (pure merge; those are the registry's steps).
      Does NOT allocate beyond the Env map (no gratuitous slice copies).
- [ ] `merge_test.go` has the test groups below, all passing.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` (diff-empty) clean;
      `go test -race ./internal/provider/` AND `go test -race ./...` green.
- [ ] go.mod/go.sum byte-unchanged by S2; `manifest.go`/`manifest_test.go` byte-unchanged (S1 untouched);
      every file outside `internal/provider/` byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact
function signature, the three merge regimes + the per-field list (copied from S1's `manifest.go`), the
map-aliasing gotcha + fix, and the ~10 test specs. The only external knowledge needed is "Go maps are
reference types; `out := base` aliases the map" — which is stated explicitly below. No git/config/generation
knowledge required.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- file: internal/provider/manifest.go   (S1 — ALREADY EXISTS; read it, do NOT edit it)
  why: the EXACT field names/types/tags MergeManifest operates on. The 14 scalar pointer fields
       (*string: Detect, Command, PromptDelivery, PromptFlag, PrintFlag, ModelFlag, DefaultModel,
       SystemPromptFlag, ProviderFlag, DefaultProvider, Output, JsonField, RetryInstruction;
       *bool: StripCodeFence), the 2 slices (Subcommand, BareFlags []string), the Env map
       (map[string]string), and the plain `Name string`. It also defines the UNEXPORTED helpers
       strPtr(string) *string and boolPtr(bool) *bool — use these in merge_test.go (same package).
  pattern: copy S1's doc-comment + value-receiver style. MergeManifest is a FREE FUNCTION (not a
       method) because the contract names it `MergeManifest(base, override Manifest) Manifest`.
  critical: do NOT edit this file in S2. Do NOT rename/retag Manifest fields. S1 is a frozen contract.

- docfile: plan/001_f1f80943ac34/P1M2T1S2/research/merge-semantics-and-aliasing.md
  why: the three merge regimes (table) + the map-aliasing bug + the fresh-map fix + why Name is not
       merged + why MergeManifest does not Validate. The single most important read for this subtask.
  critical: the Env-map aliasing trap (§2) is the ONE thing most likely to be implemented wrong.
       `out := base; out.Env[k] = v` MUTATES the caller's base.Env. Allocate a fresh map.

- docfile: plan/001_f1f80943ac34/P1M2T1S1/research/go-toml-pointer-behavior.md
  why: WHY the scalar fields are pointers (absent→nil, present-zero→non-nil). Explains why
       `override.Field != nil` is the correct override test (a present `""`/`false` is non-nil → wins).
       Without this, "explicit zero wins" looks like a bug rather than the design.

- file: PRD.md
  section: "16.1 Resolution order (FR34)" (h3.57) — the AUTHORITATIVE merge spec: "Provider manifests
       merge field-by-field (a user override that sets only `default_model` leaves all other fields
       from the built-in manifest intact)." This sentence IS the keystone test.
  why: every merge semantic in this PRP is justified by §16.1. The §16.2 example
       (`[provider.pi] default_model = "glm-5.2"`) is the canonical fixture.
  critical: the merge is FIELD-BY-FIELD (sub-field), NOT whole-manifest-replace. §16.1 is explicit.

- file: PRD.md
  section: "12.8 Extensibility: user-defined providers" (h3.44) — the brand-new-provider case.
  why: explains why `Name` is not field-merged (a §12.8 provider has no built-in `base`; the registry
       sets Name from the table key). A user-defined `[provider.myagent]` decodes to a full Manifest
       (all fields from TOML); MergeManifest(Manifest{}, full) propagates every field because each is
       non-nil/non-empty. Confirms the merge handles both "override a built-in" AND "define new".

- file: plan/001_f1f80943ac34/architecture/critical_findings.md
  section: "FINDING 5"
  why: go-toml/v2 has no omitempty; "the merge must be field-by-field (only override non-zero/non-nil
       fields)." S1 chose pointers so S2's merge is trivial + correct. FINDING 5 is the constraint S2
       satisfies.

- file: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md
  section: "2.4 Layers 2–3: TOML File Loading and Merging" (the `overlay` pattern) + "5.4 Omitempty
       Workarounds" (Option A: pointers).
  why: §2.4's `overlay(dst, src)` is the sibling pattern (config-layer scalar overlay). NOTE the
       difference: §2.4 overlays CONFIG scalars and does a key-level replace for the Provider MAP (one
       provider entry replaces another entirely — NO sub-field merge at the config layer). S2 does the
       SUB-FIELD merge WITHIN one provider entry (the manifest overlay) — which §2.4 deliberately
       defers ("no sub-field merge within a provider" at the config layer; the manifest merge is S2's
       job). §2.4's bool caveat ("use *bool, check != nil") is exactly the pointer rule S2 applies.
  critical: do NOT confuse the config-layer map replace (§2.4) with the manifest field-merge (S2).
       They are different layers. S2 merges FIELDS of ONE manifest.

- file: internal/config/config.go
  section: the `Providers map[string]map[string]any \`toml:"-"\`` field + its doc comment.
  why: the FROZEN contract that names this subtask. The comment says the registry "re-encodes the
       entry to TOML and unmarshals into a Manifest, then field-merges with the built-in manifest per
       PRD §16.1" — MergeManifest IS that field-merge. Read to confirm S2's role; do NOT edit config.go.
  gotcha: config holds overrides as a RAW map precisely to avoid importing the Manifest type (cycle).
       The registry (P1.M2.T3) bridges config→provider; S2's MergeManifest takes already-decoded
       Manifests and knows nothing about config.

- file: internal/config/config_test.go   (test-style pattern — do NOT edit)
  why: the repo's table-test + assertion convention (`package <pkg>` white-box, stdlib `testing`,
       direct `t.Errorf("X = %v, want %v", got, want)`). Mirror it. No testify/subtest framework.
  pattern: build fixtures with helper funcs (S1's strPtr/boolPtr); compare pointers by deref
       (`if *got != want`), slices/maps by `reflect.DeepEqual`.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagehand ; go 1.22 ; require go-toml/v2 v2.4.2 + pflag v1.0.10  (UNCHANGED by S2)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 (Config + loaders + Load) — FROZEN, do NOT touch; do NOT import from provider
    config.go                   # Providers map[string]map[string]any `toml:"-"`  ← the raw-map bridge (registry consumes it, NOT S2)
    ...                         # file.go / git.go / load.go + tests — untouched
  git/                          # P1.M1.T2/T3 — untouched
  provider/                     # S1 created the package; S2 ADDS to it
    manifest.go                 # S1 — Manifest struct + Validate + DetectCommand + Resolve + Default* + strPtr/boolPtr  (UNCHANGED by S2)
    manifest_test.go            # S1 — decode/marshal/validate/detectcommand/resolve tests  (UNCHANGED by S2)
    merge.go                    # NEW (S2) ← MergeManifest(base, override Manifest) Manifest
    merge_test.go               # NEW (S2) ← ~10 test groups (partial override, explicit-zero, slices, env, no-mutate, ...)
cmd/stagehand/main.go           # `package main; func main(){}` stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  provider/
    merge.go                    # NEW — func MergeManifest(base, override Manifest) Manifest (zero imports)
    merge_test.go               # NEW — ~10 test groups, package provider (white-box)
# manifest.go / manifest_test.go UNCHANGED (S1). go.mod / go.sum UNCHANGED. Every other file UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call #3 — the ONE real trap): the Env map is a REFERENCE TYPE. `out := base` copies
// the struct header, so out.Env and base.Env are the SAME map object. A naive `out.Env[k] = v` MUTATES
// the caller's base.Env — a silent side effect that corrupts the built-in manifest for the rest of the
// process. FIX: on a non-empty override.Env, allocate a FRESH map[string]string, copy every base.Env
// key, then every override.Env key, assign to out.Env. Slices do NOT have this bug (we only reassign
// the header, never mutate the shared backing array). TestMergeManifest_DoesNotMutateInputs catches it.

// CRITICAL (design call #1 — explicit zero wins): scalar pointer fields use `override.Field != nil` as
// the override test — NOT `*override.Field != ""` or `*override.Field != false`. A non-nil pointer to
// "" or false IS an override (the user wrote `print_flag = ""` / `strip_code_fence = false` on purpose).
// S1's pointer design exists so this distinction is expressible. Using `*override != ""` would silently
// drop an explicit-empty override — the exact bug pointers were chosen to prevent.

// CRITICAL (slices use len>0, not != nil): for Subcommand/BareFlags, the test is `len(override.Field) > 0`
// (wholesale replace) — NOT `override.Field != nil`. go-toml decodes `bare_flags = []` to a non-nil but
// EMPTY slice (S1 FINDING D); the contract says treat empty as "not overridden" (keep base). Using
// `!= nil` would let an explicit `[]` clobber the built-in flags — against the contract.

// GOTCHA: MergeManifest takes base BY VALUE. `out := base` is a struct copy; the pointer fields are
// copied (out and base point at the SAME string/bool values — fine, strings/bools are immutable). Slices
// and the Env map are copied by HEADER (shared backing data) — hence the Env-aliasing note above. The
// function has NO other aliasing hazard: scalar reassignment (`out.PrintFlag = override.PrintFlag`)
// just re-points out's field; base.PrintFlag is untouched.

// GOTCHA: do NOT call Validate or Resolve inside MergeManifest. A partial override legitimately lacks
// Command (it inherits the built-in's); Validate would reject it. The registry runs Validate on the
// MERGED result. MergeManifest is a pure overlay — "merge, don't judge."

// GOTCHA: Name is NOT merged. `result.Name == base.Name` (a free side effect of `out := base`). Do NOT
// add `if override.Name != "" { out.Name = override.Name }` — `name` is the table key (never in the
// TOML body), so override.Name is always "" anyway, and the registry owns setting the final Name from
// the table key (esp. for §12.8 brand-new providers where base is the zero Manifest). Keep it simple.

// GOTCHA: merge.go has ZERO imports. If `go vet` complains about an unused import, you added one you
// don't need (fmt? errors? strings?). Remove it. The function body uses only `len`, `make`, `for range`,
// and field assignment — all builtins. Keeping zero imports keeps the package stdlib-only (S1 design
// call #4) and means go.mod is provably unchanged.

// GOTCHA: the Env-map allocation `make(map[string]string, cap)` — pass a capacity hint of
// `len(base.Env)+len(override.Env)` to avoid regrowth. Minor, but it's the one allocation in the function.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/provider/merge.go
package provider

// (NO imports — pure logic: field assignment + len + make + for range only.)

// MergeManifest overlays the non-nil/non-empty fields of override onto a copy of base and returns the
// merged manifest, per PRD §16.1 ("Provider manifests merge field-by-field: a user override that sets
// only default_model leaves all other fields from the built-in manifest intact").
//
// THREE merge regimes (from the work-item contract):
//
//  1. Scalar pointer fields (*string / *bool): `override.Field != nil` → result takes override.Field.
//     An EXPLICIT "" or false OVERRIDES (non-nil is the override signal — the whole reason S1 made the
//     optional scalars pointers; a present zero value is a deliberate override, not an absence).
//
//  2. Slices (Subcommand, BareFlags): `len(override.Slice) > 0` → result REPLACES base's slice
//     wholesale (NO element-level merge). An empty/nil override slice is treated as "not overridden"
//     (base preserved).
//
//  3. Env map: merged KEY-BY-KEY into a FRESH map — each key in override.Env overwrites the same base
//     key, while base keys absent from override survive. A nil/empty override.Env leaves base.Env.
//     (CRITICAL: a fresh map is allocated to avoid mutating the caller's base.Env — maps are reference
//     types and `out := base` aliases them. See research/merge-semantics-and-aliasing.md §2.)
//
// Name is NOT field-merged — result.Name == base.Name. `name` is the [provider.<name>] table key
// (never written into the table body), so a decoded override always has Name==""; the registry
// (P1.M2.T3) sets the final Name from the table key. Keeping Name out of the merge makes this a pure,
// predictable overlay.
//
// MergeManifest does NOT Validate or Resolve — a partial override legitimately lacks Command (it
// inherits the built-in's). The registry runs Validate on the merged result, then Resolve. Pure merge.
func MergeManifest(base, override Manifest) Manifest {
	out := base // struct copy: scalar pointers copied (immutable targets, safe); slices + Env map
	           // copied BY HEADER (shared backing data). See the Env-map note below for why this matters.

	// --- regime 1: scalar pointer fields — non-nil override WINS (explicit "" / false included) ---
	if override.Detect != nil {
		out.Detect = override.Detect
	}
	if override.Command != nil {
		out.Command = override.Command
	}
	if override.PromptDelivery != nil {
		out.PromptDelivery = override.PromptDelivery
	}
	if override.PromptFlag != nil {
		out.PromptFlag = override.PromptFlag
	}
	if override.PrintFlag != nil {
		out.PrintFlag = override.PrintFlag
	}
	if override.ModelFlag != nil {
		out.ModelFlag = override.ModelFlag
	}
	if override.DefaultModel != nil {
		out.DefaultModel = override.DefaultModel
	}
	if override.SystemPromptFlag != nil {
		out.SystemPromptFlag = override.SystemPromptFlag
	}
	if override.ProviderFlag != nil {
		out.ProviderFlag = override.ProviderFlag
	}
	if override.DefaultProvider != nil {
		out.DefaultProvider = override.DefaultProvider
	}
	if override.Output != nil {
		out.Output = override.Output
	}
	if override.JsonField != nil {
		out.JsonField = override.JsonField
	}
	if override.StripCodeFence != nil {
		out.StripCodeFence = override.StripCodeFence
	}
	if override.RetryInstruction != nil {
		out.RetryInstruction = override.RetryInstruction
	}

	// --- regime 2: slices — non-empty override REPLACES wholesale (no element merge) ---
	if len(override.Subcommand) > 0 {
		out.Subcommand = override.Subcommand
	}
	if len(override.BareFlags) > 0 {
		out.BareFlags = override.BareFlags
	}

	// --- regime 3: Env map — key-by-key merge into a FRESH map (override keys win; base keys survive) ---
	// CRITICAL: out.Env currently ALIASES base.Env (out := base copied the map header). We MUST allocate
	// a fresh map and copy both sides into it; mutating out.Env in place would corrupt the caller's
	// base.Env (a silent side effect). Slices are safe above because we only reassign the header.
	if len(override.Env) > 0 {
		merged := make(map[string]string, len(base.Env)+len(override.Env))
		for k, v := range base.Env { // base keys first
			merged[k] = v
		}
		for k, v := range override.Env { // override keys overwrite same-named base keys
			merged[k] = v
		}
		out.Env = merged // break the alias to base.Env
	}

	// Name: NOT merged — out.Name == base.Name (the struct copy). The registry sets the final Name.
	return out
}
```

> **gofmt note:** run `gofmt -w internal/provider/merge.go internal/provider/merge_test.go`. Do not
> hand-align. One doc comment on `MergeManifest` (the three regimes + the Name/Validate notes) is
> encouraged — it becomes the doc surfaced by `providers show` later.
>
> **Imports:** `merge.go` has NONE. Do NOT add `fmt`/`errors`/`strings` speculatively — `go vet`
> rejects unused imports and they're unneeded (the body uses only builtins). `merge_test.go` imports
> `testing` + `reflect` (and optionally `github.com/pelletier/go-toml/v2` for one decoded fixture —
> already in go.mod, test-only).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/provider/merge.go — MergeManifest (the whole function)
  - IMPLEMENT func MergeManifest(base, override Manifest) Manifest per the Data Models block:
      (a) out := base
      (b) 14 scalar pointer fields: `if override.X != nil { out.X = override.X }` (Detect, Command,
          PromptDelivery, PromptFlag, PrintFlag, ModelFlag, DefaultModel, SystemPromptFlag,
          ProviderFlag, DefaultProvider, Output, JsonField, StripCodeFence, RetryInstruction).
      (c) 2 slices: `if len(override.X) > 0 { out.X = override.X }` (Subcommand, BareFlags).
      (d) Env: `if len(override.Env) > 0` → fresh make(map[string]string, len(base.Env)+len(override.Env)),
          copy base.Env keys then override.Env keys, assign out.Env = merged.
  - IMPORTS: NONE. If `go vet` flags an unused import, remove it. Pure logic only.
  - GOTCHA: use `len(override.Env) > 0` (consistent with slices); the fresh-map allocation is the
      aliasing fix — do NOT write `out.Env[k] = v` in place.
  - GOTCHA: do NOT add Name-merge logic; do NOT call Validate/Resolve.
  - WHY ONE TASK: it's ~40 lines of mechanical field assignment + one map merge. Splitting it adds no
      value and risks a half-written file that doesn't compile.

Task 2: CREATE internal/provider/merge_test.go — the keystone + regime tests
  - PACKAGE: `package provider` (white-box — uses S1's unexported strPtr/boolPtr). Imports: testing +
    reflect (+ optionally toml for the decoded fixture). Mirror config_test.go style (t.Errorf).
  - ADD a `sampleBase() Manifest` helper that returns a FULLY-populated Manifest (a pi-like manifest:
      Name="pi", Detect/Command=strPtr("pi"), PromptDelivery=strPtr("stdin"), PrintFlag=strPtr("-p"),
      ModelFlag=strPtr("--model"), DefaultModel=strPtr("glm-5-turbo"), SystemPromptFlag=strPtr("--system-prompt"),
      ProviderFlag=strPtr("--provider"), BareFlags=[]string{"--no-tools","--no-extensions","--no-skills"},
      Output=strPtr("raw"), StripCodeFence=boolPtr(true), Env={"A":"1","B":"2"}). This is the built-in
      the overrides merge onto. (Construct with strPtr/boolPtr — NOT a TOML decode — so the test's
      correctness doesn't depend on go-toml.)
  - TEST TestMergeManifest_PartialOverride_OnlyTouchedFieldChanges (THE §16.1 KEYSTONE):
      base := sampleBase(); override := Manifest{DefaultModel: strPtr("glm-5.2")}; merged := MergeManifest(base, override).
      Assert *merged.DefaultModel == "glm-5.2" (changed) AND every OTHER field == base's value (Command,
      PrintFlag, BareFlags, Output, StripCodeFence, Env, … UNCHANGED). This is PRD §16.1 verbatim.
  - TEST TestMergeManifest_ExplicitZeroPointerWins (THE PAYOFF OF S1's POINTER DESIGN):
      base StripCodeFence=true, PrintFlag="-p"; override StripCodeFence=boolPtr(false), PrintFlag=strPtr("").
      Assert merged StripCodeFence==false (NOT base's true) and PrintFlag=="" (NOT base's "-p"). Proves a
      present zero value overrides — impossible with plain bool/string.
  - TEST TestMergeManifest_NonEmptySliceReplacesWholesale:
      base BareFlags=[6 items], Subcommand=nil; override BareFlags=["--x"], Subcommand=["run"].
      Assert merged BareFlags==["--x"] (wholesale REPLACE, not append — len 1, not 7) and Subcommand==["run"].
  - TEST TestMergeManifest_EmptyOrNilSlicePreservesBase:
      (a) override BareFlags=nil → merged BareFlags==base's [6 items]; (b) override Subcommand=[] (non-nil
      empty) → merged Subcommand==base's (empty override treated as "not overridden" per contract).
  - TEST TestMergeManifest_EnvKeyByKeyMerge:
      base Env={"A":"1","B":"2"}; override Env={"B":"3","C":"4"}. Assert merged Env=={"A":"1","B":"3","C":"4"}
      (B overwritten by override; A survived from base; C added). Use reflect.DeepEqual.
  - TEST TestMergeManifest_EnvNilOverridePreservesBase: override Env=nil → merged Env==base.Env.
  - TEST TestMergeManifest_DoesNotMutateInputs (THE ALIASING GUARD):
      base := sampleBase(); snapshot base.Env, base.BareFlags, base.Command BEFORE the call; call
      MergeManifest(base, override with Env={"X":"9"}). Assert base.Env is UNCHANGED (still {"A":"1","B":"2"},
      NOT {"A","B","X"}) via reflect.DeepEqual(base.Env, snapshot). Also assert base.BareFlags unchanged.
      THIS is the test that catches a forgotten fresh-map allocation.
  - TEST TestMergeManifest_EmptyOverrideIsIdentity: MergeManifest(base, Manifest{}) == base for every
      field (no override set → nothing changes). Assert field-by-field (or a helper that compares two
      resolved manifests). (Note: Manifest has no exported Equal — compare field-by-field or via Resolve
      + reflect, but be careful: reflect.DeepEqual on Manifest compares pointer targets, which is fine
      here since both sides share the same pointers in the identity case.)
  - TEST TestMergeManifest_NamePreservedFromBase: base Name="pi"; override Name="ignored" (or "").
      Assert merged.Name == "pi" (== base.Name), regardless of override.Name. Pins design call #2.
  - TEST TestMergeManifest_MergedResultValidates (S1↔S2 COMPOSITION):
      base := sampleBase() (a complete, valid manifest); override := Manifest{DefaultModel: strPtr("glm-5.2")}.
      Assert MergeManifest(base, override).Validate() returns nil (the merged result is valid — proving S2
      composes with S1's Validate). Also assert base itself Validate()s (fixture sanity).

Task 3: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged
    (`git diff --exit-code go.mod go.sum` empty). manifest.go/manifest_test.go MUST be byte-unchanged
    (`git diff --exit-code internal/provider/manifest.go internal/provider/manifest_test.go` empty).
    S1's tests MUST stay green (no field/type change, no import edge added). The config + git suites
    MUST stay green (no import edge into them).
```

### Implementation Patterns & Key Details

```go
// The three regimes, side by side — the whole function in miniature:
// (1) scalar pointer: non-nil WINS (explicit ""/false included — the S1 pointer-design payoff)
if override.DefaultModel != nil {
	out.DefaultModel = override.DefaultModel // a non-nil *"" overrides; a nil inherits base
}
// (2) slice: non-empty REPLACES wholesale (no element merge; empty = no-op)
if len(override.BareFlags) > 0 {
	out.BareFlags = override.BareFlags // base's 6 flags fully replaced by override's N
}
// (3) Env map: key-by-key into a FRESH map (the aliasing fix)
if len(override.Env) > 0 {
	merged := make(map[string]string, len(base.Env)+len(override.Env))
	for k, v := range base.Env {
		merged[k] = v
	}
	for k, v := range override.Env {
		merged[k] = v
	}
	out.Env = merged // MUST assign a fresh map — out := base aliased base.Env
}

// The anti-pattern this REPLACES (would mutate the caller's base.Env — the aliasing bug):
//   out := base
//   for k, v := range override.Env { out.Env[k] = v } // ❌ mutates base.Env too!

// The keystone assertion (PRD §16.1 verbatim) — partial override touches ONE field, rest survive:
merged := MergeManifest(builtinPi, Manifest{DefaultModel: strPtr("glm-5.2")})
if *merged.DefaultModel != "glm-5.2" { t.Error("override not applied") }
if *merged.Command != "pi" || !reflect.DeepEqual(merged.BareFlags, builtinPi.BareFlags) {
	t.Error("untouched fields did NOT survive — §16.1 violated")
}
```

```go
// merge_test.go — the aliasing guard (catches the one real bug). If this fails, the fresh-map step
// was forgotten and MergeManifest is corrupting the caller's built-in manifest.
func TestMergeManifest_DoesNotMutateInputs(t *testing.T) {
	base := sampleBase()
	envBefore := map[string]string{}
	for k, v := range base.Env { envBefore[k] = v } // snapshot
	bareBefore := append([]string(nil), base.BareFlags...)

	override := Manifest{Env: map[string]string{"X": "9", "B": "overridden"}}
	_ = MergeManifest(base, override) // discard result; we care about base's integrity

	if !reflect.DeepEqual(base.Env, envBefore) {
		t.Errorf("MergeManifest mutated base.Env (aliasing bug): got %v, want %v", base.Env, envBefore)
	}
	if !reflect.DeepEqual(base.BareFlags, bareBefore) {
		t.Errorf("MergeManifest mutated base.BareFlags: got %v, want %v", base.BareFlags, bareBefore)
	}
}

// merge_test.go — the explicit-zero payoff (would FAIL with plain bool/string fields).
func TestMergeManifest_ExplicitZeroPointerWins(t *testing.T) {
	base := sampleBase() // StripCodeFence=true, PrintFlag="-p"
	merged := MergeManifest(base, Manifest{
		StripCodeFence: boolPtr(false), // explicit false — must NOT inherit base's true
		PrintFlag:      strPtr(""),      // explicit empty — must NOT inherit base's "-p"
	})
	if merged.StripCodeFence == nil || *merged.StripCodeFence != false {
		t.Errorf("explicit strip_code_fence=false lost (got %v)", merged.StripCodeFence)
	}
	if merged.PrintFlag == nil || *merged.PrintFlag != "" {
		t.Errorf("explicit print_flag=\"\" lost (got %v)", merged.PrintFlag)
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. merge.go has zero imports; merge_test.go uses only testing + reflect (+ optionally
        go-toml/v2, already in go.mod). `go mod tidy` MUST be a no-op. `git diff --exit-code go.mod
        go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - internal/provider → (stdlib: NONE in merge.go; testing+reflect in merge_test.go) ONLY.
  - internal/provider → internal/config : FORBIDDEN (cycle; same as S1). The REGISTRY (P1.M2.T3) is the
        sole importer of both config and provider.
  - internal/provider → github.com/pelletier/go-toml/v2 : test-only (merge_test.go), optional.

FROZEN FILES (do NOT edit):
  - internal/provider/manifest.go + manifest_test.go (S1): the Manifest type + helpers are a CONTRACT.
        S2 ADDS merge.go/merge_test.go; it does not modify S1's files.
  - internal/config/* (P1.M1.T4), internal/git/* (P1.M1.T2/T3), cmd/stagehand/main.go, Makefile.

DOWNSTREAM CONTRACTS (do NOT implement here — just honor the shapes they will consume):
  - P1.M2.T3 (registry): `merged := MergeManifest(builtin, decode(reencode(config.Providers[<name>])))`;
        then merged.Validate(); then merged.Resolve(); hand to renderer/executor/parser. The registry
        sets merged.Name = <name> from the table key (esp. for §12.8 brand-new providers where builtin
        is the zero Manifest).
  - P1.M2.T4 (renderer): reads the RESOLVED merged manifest per §12.2.
  - P1.M2.T5 (executor): reads *resolved.Command + resolved.Env.
  - P1.M2.T6 (parser): reads *resolved.Output, *resolved.JsonField, *resolved.StripCodeFence.
  => MergeManifest's signature + the three regimes are now FROZEN for the registry. Do not change them.

NO DATABASE / NO ROUTES / NO CLI / NO BUILT-IN MANIFEST CONTENT (P1.M2.T2) / NO REGISTRY (P1.M2.T3) /
NO RENDER/EXEC/PARSE (T4/T5/T6).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1 (merge.go):
gofmt -w internal/provider/merge.go internal/provider/merge_test.go
test -z "$(gofmt -l internal/provider/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/provider/        # (and `go vet ./...`) Expect zero diagnostics.
go build ./...                     # Whole module compiles (incl. the new function). Expect exit 0.
# Expected: clean. ZERO imports in merge.go (verify): the only `import` lines should be in merge_test.go.
grep -n '^import\|^	"' internal/provider/merge.go && echo "note: merge.go has imports (should be NONE)" || echo "merge.go zero-imports (good)"

# Confirm NO new dependency + NO edit to S1's files + no config edge:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"   # MUST be empty.
git diff --exit-code internal/provider/manifest.go internal/provider/manifest_test.go && echo "S1 files UNCHANGED (expected)"   # MUST be empty.
grep -n 'internal/config' internal/provider/merge.go && echo "BAD: config import" || echo "no config import (good)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# The ~10 test groups (white-box; no git/exec/config needed — pure struct merge):
go test -race ./internal/provider/ -v
# Expected: PASS — TestMergeManifest_PartialOverride_OnlyTouchedFieldChanges (§16.1 keystone),
#   TestMergeManifest_ExplicitZeroPointerWins (pointer payoff), TestMergeManifest_NonEmptySliceReplacesWholesale,
#   TestMergeManifest_EmptyOrNilSlicePreservesBase, TestMergeManifest_EnvKeyByKeyMerge,
#   TestMergeManifest_EnvNilOverridePreservesBase, TestMergeManifest_DoesNotMutateInputs (aliasing guard),
#   TestMergeManifest_EmptyOverrideIsIdentity, TestMergeManifest_NamePreservedFromBase,
#   TestMergeManifest_MergedResultValidates (S1↔S2 composition) — PLUS S1's tests still green.

# Full suite must stay green (no regression; confirms no stray import edge broke config/git):
go test -race ./...
# Expected: all packages PASS (config, git, provider).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build + scope/additive checks:
go build -o /tmp/stagehand ./cmd/stagehand && echo "binary builds"   # main.go stub still links.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged"
# Confirm S2 did NOT touch anything outside the two new files:
git diff --exit-code -- internal/config internal/git cmd Makefile internal/provider/manifest.go internal/provider/manifest_test.go && echo "frozen + S1 files UNCHANGED by S2"
grep -n 'func MergeManifest' internal/provider/merge.go   # MUST print the function line.
# Expected: binary builds; go.mod/go.sum unchanged; frozen+S1 files unchanged; MergeManifest present.

# Coverage of the new function (Makefile has a coverage target):
go test -race ./internal/provider/ -coverprofile=/tmp/cov.out && go tool cover -func=/tmp/cov.out | grep -i merge
# Expected: MergeManifest at (or near) 100% line coverage — every regime + the nil/empty branches hit
# by the table of tests. (make coverage runs the project-wide gate; the ≥85% target is enforced at P1.M5.T3.S3.)

# Smoke the §16.1 contract end-to-end against a real-ish built-in (sanity for the registry author):
# a throwaway in-package test is already the cleanest expression — the TestMergeManifest_* table above
# IS the smoke test. (No standalone /tmp binary needed; MergeManifest is pure and unit-tested directly.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Lint gate (project-wide):
golangci-lint run ./internal/provider/ 2>/dev/null || echo "golangci-lint not installed (Makefile lint gate is project-wide; run `make lint` in CI)."

# Property-style invariant (optional, recommended): for a few random partial overrides (each setting a
# random SUBSET of fields to random values), MergeManifest MUST (1) never panic, (2) never mutate base,
# (3) always leave every non-overridden field == base's value, and (4) always leave every overridden
# field == override's value. This is the formal statement of the three regimes. A short loop in
# merge_test.go (table-driven over field-kind × present/absent) covers it without a fuzz dep.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` is a
      no-op; `git diff --exit-code go.mod go.sum` empty; `merge.go` has ZERO imports.
- [ ] Level 2 green: `go test -race ./internal/provider/ -v` (all ~10 merge groups + S1's tests) AND
      `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; `manifest.go`/`manifest_test.go` unchanged;
      every file outside the two new `merge*.go` files unchanged; `MergeManifest` present.

### Feature Validation

- [ ] `MergeManifest(base, override Manifest) Manifest` exists with the exact contract signature.
- [ ] Scalar pointer fields: `override != nil` → override (explicit `""`/`false` WINS — keystone test).
- [ ] Slices: `len(override) > 0` → wholesale replace; empty/nil → keep base.
- [ ] Env map: key-by-key merge into a FRESH map; override keys win, base keys survive; nil override →
      keep base; inputs NEVER mutated (aliasing guard test).
- [ ] Name: `result.Name == base.Name` (not merged).
- [ ] The §16.1 canonical example: merging `{default_model="glm-5.2"}` onto a pi built-in changes ONLY
      `default_model`; `command`/`bare_flags`/`print_flag`/`output`/… all survive untouched.
- [ ] A fully-merged result passes `Validate` (S1↔S2 composition).

### Code Quality Validation

- [ ] Follows repo conventions: white-box `package provider`, stdlib `testing`, `t.Errorf` assertions,
      `reflect.DeepEqual` for slices/maps (mirrors `internal/config`/`internal/git` test style); free
      function (not a method) matching the contract name; zero imports in `merge.go`.
- [ ] File placement matches the desired tree (`merge.go` + `merge_test.go` only — S1's files untouched).
- [ ] The three regimes are distinct and correct (pointer `!= nil`; slice `len > 0`; map key-by-key into
      a fresh map) — NOT a single blanket rule.
- [ ] `internal/provider` still imports nothing outside stdlib (design call #4 preserved); `merge.go`
      imports NOTHING.
- [ ] No premature scope: no registry (P1.M2.T3), no Validate/Resolve call inside MergeManifest, no
      render/exec/parse (T4/T5/T6), no built-in manifest values (T2).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Doc comment on `MergeManifest` documenting the three regimes + Name-not-merged + does-not-Validate
      (seeds the `providers show` / reference-file docs later).
- [ ] No new env vars / no user-facing docs (PRD "DOCS: none — internal logic"; public override docs
      come with `providers show` P1.M4.T1.S3 and reference files P1.M5.T2).
- [ ] `internal/provider/merge.go` + `merge_test.go` are the ONLY files touched.

---

## Anti-Patterns to Avoid

- ❌ Don't mutate the caller's inputs. `out := base` aliases `base.Env` (and `base.BareFlags`/`Subcommand`
  by header). For Env, allocate a FRESH map and copy both sides; never `out.Env[k] = v` in place.
  `TestMergeManifest_DoesNotMutateInputs` exists to catch this. (Slices are safe because we only
  reassign the header, never mutate the shared backing array — but do NOT append to or index-assign
  `out.BareFlags` either.)
- ❌ Don't use `*override.Field != ""` / `!= false` as the override test for scalar pointers. A non-nil
  pointer to `""`/`false` IS an override (the user wrote it on purpose). Use `override.Field != nil`.
  The `!= ""`/`!= false` test would silently drop an explicit-empty/false override — the exact bug S1's
  pointer design exists to prevent. `TestMergeManifest_ExplicitZeroPointerWins` catches it.
- ❌ Don't use `override.Slice != nil` for slices. go-toml decodes `bare_flags = []` to non-nil-but-empty
  (S1 FINDING D); the contract says treat empty as "not overridden" (keep base). Use `len(override.Slice) > 0`.
- ❌ Don't call `Validate` or `Resolve` inside `MergeManifest`. A partial override legitimately lacks
  `Command` (inherits the built-in's); Validate would reject it. MergeManifest is a pure overlay; the
  registry runs Validate on the merged result. "Merge, don't judge."
- ❌ Don't field-merge `Name`. `result.Name == base.Name` (a free side effect of `out := base`). Adding
  `if override.Name != ""` logic couples S2 to an assumption about how the registry pre-processes; the
  registry owns the final Name from the table key (esp. §12.8 brand-new providers).
- ❌ Don't edit `manifest.go` / `manifest_test.go` (S1) — they are a frozen contract. S2 ADDS `merge.go`
  + `merge_test.go`. If you think a field is missing from Manifest, that's an S1 issue, not an S2 one.
- ❌ Don't add imports to `merge.go`. It is pure logic (`len`, `make`, `for range`, field assignment). An
  unused import fails `go vet`. `merge_test.go` may import `testing`/`reflect`/(optionally `toml`).
- ❌ Don't change go.mod/go.sum — no new dep. An unintended `go get`/`go mod tidy` mutation means an
  import crept into `merge.go`; remove the import, don't add the dep.
- ❌ Don't implement the registry (P1.M2.T3), built-in manifest values (P1.M2.T2), or render/exec/parse
  (P1.M2.T4/T5/T6) here. S2 is one function + its tests.
- ❌ Don't add a `Merge` METHOD on Manifest (`func (m Manifest) Merge(override)`) — the contract names a
  FREE FUNCTION `MergeManifest(base, override) Manifest`. A method reads ambiguously at the call site
  (`base.Merge(override)` vs `override.MergeOnto(base)`); the free function is unambiguous.
