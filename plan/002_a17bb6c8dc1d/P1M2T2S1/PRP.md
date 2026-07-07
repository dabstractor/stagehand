---
name: "P1.M2.T2.S1 — Reorder preferredBuiltins (FR-D1) + pi default_model → empty (FR-D2)"
description: |
  Two small, high-leverage value changes with a wide-but-mechanical test ripple:
  (1) **FR-D1** — reorder `preferredBuiltins` (internal/provider/registry.go:15) from the agy-appended
  `[pi, claude, gemini, opencode, codex, cursor, agy]` to the FR-D1 cascading priority
  `[pi, opencode, cursor, agy, gemini, codex, claude]` (open/self-hostable harnesses first; closed
  subscription CLIs last; pi first). `Registry.DefaultProvider(installed)` walks this slice — its LOGIC
  is unchanged; only the slice order + its doc comment change.
  (2) **FR-D2** — in `builtinPi()` (internal/provider/builtin.go:42) change `DefaultModel` from
  `strPtr("glm-5-turbo")` to `strPtr("")`, decoupling the shipped pi default from the original author's
  personal z.ai/GLM subscription. `DefaultProvider` is ALREADY `strPtr("")` (no change). The
  zai/glm-5-turbo setup becomes a documented PERSONAL OVERRIDE (config init fills per-role models from
  FR-D4 in a later task). Update the `builtinPi` doc comment (it currently falsely claims the shipped
  default "reproduces commit-pi byte-for-byte").

  PRD basis: §9.16 FR-D1 (h3.32) + FR-D2, and §12.3 (h3.45) which now shows the shipped pi manifest with
  BOTH `default_model=""` and `default_provider=""`, framing `zai`/`glm-5-turbo` as "Personal-override
  example (NOT the shipped default)".

  ⚠️ **PREREQUISITE (assumed COMPLETE): P1.M2.T1.S1 (agy).** When this task begins, `BuiltinManifests()`
  has 7 keys (agy added) and `preferredBuiltins` is `[pi, claude, gemini, opencode, codex, cursor, agy]`
  (agy appended at the END by the agy task). This task REORDERS that 7-element slice to FR-D1 and does
  NOT touch any agy-specific edit (agyTOML, AgyFields, providerFiles+agy, KeysAndCount=7 are all agy's).
  If agy has not landed, this task cannot start (its reorder assumes 7 elements incl. agy).

  ⚠️ **THE #1 trap — the three-way `reflect.DeepEqual` parity chain on `default_model`.** `default_model`
  is a parity-checked field across THREE artifacts: `builtinPi()` (Go) ⇄ `piTOML` (builtin_test.go literal)
  ⇄ `providers/pi.toml` (file). go-toml/v2 decodes `default_model = ""` to a NON-NIL `*""`. After the
  change ALL THREE must carry `""` or `TestBuiltinManifests_DecodeParity/pi` and
  `TestProviderReferenceFiles_DecodeParity/pi` FAIL. Mechanical fix: set `""` in all three. `default_provider`
  is ALREADY `""` in all three — DO NOT change or omit it.

  ⚠️ **THE #2 trap — the pi render tests split into SHIPPED DEFAULT vs PERSONAL OVERRIDE.** With
  `DefaultModel=""`, a `model=""` render no longer emits `--model` (the fallback `modelToUse = *r.DefaultModel`
  now yields ""). So the commit-pi byte-for-byte invocation (`pi --provider zai --model glm-5-turbo …`) is
  NO LONGER the shipped default — it is now produced only by EXPLICIT `model="glm-5-turbo", provider="zai"`.
  The render tests must be REFRAMED (not deleted): (a) a shipped-default case (`model="", provider=""` →
  NO `--model`/`--provider`); (b) a personal-override case (`model="glm-5-turbo", provider="zai"` →
  byte-for-byte commit-pi, PRESERVED as a regression). The `Render` code logic is UNCHANGED (comment-only
  edit to render.go) — only the test expectations move.

  ⚠️ **THE #3 trap — `TestDefaultProvider`'s `["claude","gemini"]→"claude"` case BREAKS.** Under FR-D1,
  gemini (rank 5) precedes claude (rank 7), so `DefaultProvider(["claude","gemini"])` now returns
  "gemini". Rewrite the test's cases to assert the FR-D1 cascade (gemini before claude; codex before
  claude; cursor before agy/gemini; pi always tops).

  ⚠️ **Do NOT touch the INDEPENDENT fixtures.** `manifest_test.go`'s `piManifestTOML` (S1's structural
  decode fixture — it OMITS `default_provider`, so it's already ≠ builtinPi) and `merge_test.go`'s
  `sampleBase` (the merge-test helper) use `glm-5-turbo` as an ARBITRARY value; they are NOT parity
  oracles and NOT assertions of the pi built-in. Editing them is out-of-scope churn.

  Deliverable: 7 files edited (registry.go, registry_test.go, builtin.go, builtin_test.go, render.go
  [comment], render_test.go, providers/pi.toml) + 2 opt-in/CLI test fixes (realagent_test.go,
  providers_test.go). NO new files, NO new types, NO logic change to Render/Validate/Resolve/Merge, NO
  go.mod change. OUTPUT: `DefaultProvider(installed)` returns providers in FR-D1 priority; the shipped
  pi manifest is model-less/provider-less (FR-D2); all sync guards + render tests green.
---

## Goal

**Feature Goal**: Align two shipped defaults with the v2.0 PRD — (1) the auto-default provider cascade
follows FR-D1 (open/self-hostable first, closed last), and (2) the pi built-in no longer assumes the
author's z.ai/GLM subscription (FR-D2: `default_model=""`, `default_provider=""`). Every test that
asserted the old order or the old pi default is updated to match; the commit-pi byte-for-byte regression
is PRESERVED as an explicit-override test.

**Deliverable** (all EDITS — no new files):
1. `internal/provider/registry.go`: `preferredBuiltins` reordered to FR-D1; comment rewritten to cite FR-D1.
2. `internal/provider/registry_test.go`: `TestPreferredBuiltins_MatchesBuiltinKeys` gains an exact FR-D1
   order assertion; `TestDefaultProvider` cases rewritten for the FR-D1 cascade.
3. `internal/provider/builtin.go`: `builtinPi()` `DefaultModel` → `strPtr("")`; doc comment rewritten (FR-D2).
4. `internal/provider/builtin_test.go`: `piTOML` `default_model` → `""`; `PiFields` `DefaultModel` → `""`;
   `RenderedCommand_Pi_MatchesCommitPi` → two tests (shipped default + personal override).
5. `internal/provider/render.go`: stale comment rewrite (logic unchanged).
6. `internal/provider/render_test.go`: golden pi row → shipped default; `Pi_ByteForByteCommitPi` → explicit
   override; `ModelDefaultFallback` → use claude + pi-emits-no-model case.
7. `providers/pi.toml`: `default_model` → `""`; rendered-command comment → placeholders + override note.
8. `internal/generate/realagent_test.go`: pi entry `{"glm-5-turbo","zai"}` (explicit) + comment; `providerNames`
   reordered to FR-D1-minus-agy + comment.
9. `internal/cmd/providers_test.go`: `default_model = 'glm-5-turbo'` → `default_model = ''`.

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `preferredBuiltins` ==
`[pi, opencode, cursor, agy, gemini, codex, claude]`; `builtinPi().DefaultModel` is non-nil `*""`;
`piTOML`/`providers/pi.toml` carry `default_model = ""` (3-way parity green); `DefaultProvider` returns
the FR-D1 winner in every case; the commit-pi argv is still asserted (via explicit override params); no
non-provider package regresses; the `LEAVE` fixtures (manifest_test piManifestTOML, merge_test sampleBase)
are byte-unchanged.

## User Persona

**Target User**: Every Stagecoach user on first run. `DefaultProvider(installed)` picks the auto-default
provider; FR-D1 makes it prefer open/self-hostable harnesses (pi/opencode/cursor/agy) over closed
subscription CLIs (gemini/codex/claude). FR-D2 means a fresh install's pi default no longer silently
routes to the maintainer's personal z.ai backend — the user (or `config init`) supplies the model.

**Use Case**: A user installs stagecoach and runs it with no config. The registry auto-selects the
highest-priority installed provider (FR-D1). If that's pi, the shipped manifest has no pinned model —
`config init` (later task) or the user fills it. No built-in assumes someone else's account.

**User Journey**: (internal) `Registry.DefaultProvider(installed)` walks the FR-D1 `preferredBuiltins`
→ returns first installed → that manifest is rendered (pi now renders without `--model`/`--provider`
unless config supplies them) → executor runs → parser cleans.

**Pain Points Addressed**: Removes the "fresh install routes to the maintainer's z.ai subscription"
surprise (FR-D2) and makes the auto-default preference explicit/correctable (FR-D1 — "lives in one slice
in the registry; trivial to reorder").

## Why

- **FR-D1/D2 are P0 v2.0 requirements (§9.16).** FR-D1 sets the cascade order; FR-D2 decouples pi from
  one subscription. Both are stated maintainer preferences the v2.0 PRD pins.
- **Correctness of the auto-default.** The old order (`claude` before `gemini`/`opencode`/…) meant a
  machine with claude installed would pick claude over the now-preferred open harnesses. FR-D1 fixes that.
- **The pi default was a latent footgun.** `glm-5-turbo` is meaningless without `--provider zai`; a user
  overriding only the model (not the provider) would get a bare-model misroute (the exact bug FR37a +
  FR-R5b exist to prevent). FR-D2 makes the shipped manifest model-less so this can't happen by default.
- **Preserves the commit-pi lineage as a documented override** (not a silent default) — PRD §12.3 h3.45
  keeps the zai/glm-5-turbo invocation as the reference personal-override example.
- **No logic change, no new dep.** Pure data + comment + test-expectation edits. Render/Validate/Resolve/
  Merge/registry-construction code is untouched (one stale comment excepted). go.mod unchanged.

## What

Two shipped-default value changes + their test/comment ripple. No new files, no new types, no behavioral
logic change. The `Render` fallback code (`if modelToUse=="" { modelToUse = *r.DefaultModel }`) is
correct as-is and unchanged; only its stale comment + the tests that bake in the old default move.

### Success Criteria

- [ ] `preferredBuiltins == []string{"pi","opencode","cursor","agy","gemini","codex","claude"}` (FR-D1;
      7 elements; pi first; agy present).
- [ ] `registry.go` `preferredBuiltins` doc comment cites FR-D1 (open/self-hostable first; closed last).
- [ ] `TestPreferredBuiltins_MatchesBuiltinKeys` asserts the EXACT FR-D1 order via `reflect.DeepEqual`
      (in addition to the existing set-equality + pi-first checks).
- [ ] `TestDefaultProvider` cases pass under FR-D1 (gemini before claude; codex before claude; cursor
      before agy/gemini; pi always tops; user-defined never selected; nil → "").
- [ ] `builtinPi().DefaultModel` is NON-NIL `*""` (FR-D2); `DefaultProvider` unchanged (`*""`).
- [ ] `builtinPi` doc comment reflects FR-D2 (decoupled; zai/glm-5-turbo is a personal override).
- [ ] `piTOML` (builtin_test.go) has `default_model = ""`; `PiFields` asserts `DefaultModel == ""`.
- [ ] `providers/pi.toml` has `default_model = ""` (parity with builtinPi/piTOML).
- [ ] The 3-way parity chain is green: `TestBuiltinManifests_DecodeParity/pi` AND
      `TestProviderReferenceFiles_DecodeParity/pi`.
- [ ] `RenderedCommand_Pi` covers BOTH: shipped default (no `--model`/`--provider`) AND personal override
      (explicit `glm-5-turbo`/`zai` → byte-for-byte commit-pi).
- [ ] `render_test.go`: golden pi row = shipped default; `Pi_ByteForByteCommitPi` uses explicit model;
      `ModelDefaultFallback` uses claude + a pi-emits-no-model assertion.
- [ ] `render.go` fallback logic UNCHANGED (comment-only edit).
- [ ] `realagent_test.go` pi entry = `{"glm-5-turbo","zai"}` (explicit) + comment; `providerNames` FR-D1-minus-agy.
- [ ] `providers_test.go` `TestProvidersShow_BuiltInTOML` asserts `default_model = ''`.
- [ ] The `LEAVE` fixtures (`manifest_test.go piManifestTOML`, `merge_test.go sampleBase`) are byte-unchanged.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean on edited files; go.mod unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact new
`preferredBuiltins` slice, the exact `builtinPi` field change, the complete break-site map (table below
with file:line + old→new for every edit), the three-way parity chain rule, the pi render reframe
(shipped vs override), the `TestDefaultProvider` rewrite, and the `LEAVE`-fixtures warning. No
git/generate/prompt knowledge required — this is data + test-expectation surgery.

### Documentation & References

```yaml
# MUST READ — the authoritative research (every edit enumerated with file:line + old→new)
- docfile: plan/002_a17bb6c8dc1d/P1M2T2S1/research/reorder-and-pi-default.md
  why: the COMPLETE break-site map (§2 — 18 edits across 9 files, categorized CHANGE vs LEAVE), the
       TestDefaultProvider rewrite (§3), the 3-way DeepEqual parity chain (§4), the pi render reframe
       (§5 — shipped default vs personal override, with exact argv), scope fences (§6), validation
       commands (§7). The single most important read.
  critical: §4 (default_model "" must match across builtinPi/piTOML/providers/pi.toml) and §5 (the
       render tests SPLIT — do not delete the commit-pi regression, reframe it as an explicit override).

# The PRD basis
- file: plan/002_a17bb6c8dc1d/prd_snapshot.md   (or PRD.md)
  section: "9.16 Default provider & per-role model defaults" (h3.32) — FR-D1 (the exact cascade order)
       and FR-D2 (pi decoupled from z.ai/GLM). h3.32 is duplicated in the selected PRD content.
  why: FR-D1 gives the exact slice order; FR-D2 gives the decoupling requirement + the "personal override
       not the default" framing.
  critical: the order is "pi, opencode, cursor, agy, gemini, codex, claude" — 7 elements, agy INCLUDED.

- file: plan/002_a17bb6c8dc1d/prd_snapshot.md   (or PRD.md)
  section: "12.3 Built-in provider: pi" (h3.45) — the shipped pi manifest now shows `default_model = ""`
       AND `default_provider = ""`, with the zai/glm-5-turbo invocation as "Personal-override example
       (NOT the shipped default)".
  why: the authoritative pi manifest values after FR-D2; the source for the providers/pi.toml comment rewrite.
  critical: BOTH default_model and default_provider are "" in the shipped manifest. pi's default_provider
       was ALREADY "" in the code — only default_model changes.

# The prerequisite (assume COMPLETE) — what the codebase looks like when this task starts
- file: plan/002_a17bb6c8dc1d/P1M2T1S1/PRP.md
  why: agy (P1.M2.T1.S1) appends "agy" to preferredBuiltins, adds builtinAgy() + the 7th BuiltinManifests
       key, creates providers/agy.toml, and updates agy test coverage (KeysAndCount→7, agyTOML, AgyFields,
       providerFiles+agy). This task ASSUMES all of that is done.
  critical: do NOT duplicate agy's edits. This task's registry.go edit REPLACES the whole preferredBuiltins
       slice literal (whatever its current 7-element form) with the FR-D1 order — it does not "append" or
       "remove" agy.

# The files to edit (read each before editing)
- file: internal/provider/registry.go
  section: preferredBuiltins (L11 comment + L15 var) + DefaultProvider (L104-118, UNCHANGED logic).
  why: the slice reorder + comment. DefaultProvider's walk logic is correct for any order — do not touch it.

- file: internal/provider/registry_test.go
  section: TestPreferredBuiltins_MatchesBuiltinKeys (L15) + TestDefaultProvider (~L140).
  why: add the exact-order assertion; rewrite TestDefaultProvider cases for FR-D1 (gemini before claude now).

- file: internal/provider/builtin.go
  section: builtinPi() (L28 doc comment + L42 DefaultModel).
  why: the FR-D2 field change + doc-comment rewrite. Other builtins UNTOUCHED.

- file: internal/provider/builtin_test.go
  section: piTOML (L22), PiFields (~L202), RenderedCommand_Pi_MatchesCommitPi (~L336).
  why: the parity-oracle literal + field assertion + the render-test split. renderArgs helper (L134) is
       UNCHANGED (its fallback logic is correct).
  pattern: mirror the existing per-provider test style (assertStr/assertNilStr/reflect.DeepEqual).

- file: internal/provider/render.go
  section: Render's model-fallback comment (~L67).
  why: stale "→ glm-5-turbo" comment. The fallback CODE (`if modelToUse=="" { modelToUse = *r.DefaultModel }`)
       is UNCHANGED and correct. Comment-only edit.
  critical: do NOT change Render's logic. The empty-default behavior (no --model) is the INTENDED FR-D2 result.

- file: internal/provider/render_test.go
  section: TestRender_GoldenPerProvider pi row (~L47), TestRender_Pi_ByteForByteCommitPi (~L88),
       TestRender_ModelDefaultFallback (~L128). TestRender_SystemPromptPrependFallback/ProviderDefaultFallback/
       DoesNotMutateManifest are UNAFFECTED (they don't assert on --model) — leave them.
  why: the three render tests that bake in glm-5-turbo as pi's default. Reframe per research §5.

- file: providers/pi.toml
  section: default_model (L43) + rendered-command comment (L21).
  why: parity oracle (TestProviderReferenceFiles_DecodeParity reads it) → default_model must equal builtinPi.
       The comment becomes the FR-D2 "personal override" framing.

- file: internal/generate/realagent_test.go   (//go:build integration_real — opt-in, NOT in CI)
  section: realDefaults pi entry (L36) + providerNames (L44).
  why: pi's manifest default is now "" → the real run must pass an explicit model (glm-5-turbo) to keep
       testing the commit-pi shape; providerNames comment claims "preferredBuiltins order" → align to FR-D1.
  gotcha: this test is SKIPPED unless -tags integration_real + STAGECOACH_RUN_REAL=1; it won't break CI, but
       it must still be CORRECT (the comment + model value are now wrong without the edit).

- file: internal/cmd/providers_test.go
  section: TestProvidersShow_BuiltInTOML (~L208 substring list).
  why: `providers show pi` now marshals `default_model = ''` (was 'glm-5-turbo'). Update the substring.

# The LEAVE files (DO NOT EDIT — independent fixtures, not parity oracles)
- file: internal/provider/manifest_test.go
  section: piManifestTOML (L18) + TestUnmarshal_FullManifest (L63).
  why: S1's STRUCTURAL decode fixture. It OMITS default_provider (asserts nil), so it is ALREADY ≠ builtinPi.
       Its glm-5-turbo is an arbitrary decode-test value, not a built-in assertion. Editing it is churn.
  critical: do NOT "fix" it to "" — it is testing the Manifest type's decode mechanics, not the pi built-in.

- file: internal/provider/merge_test.go
  section: sampleBase (L19).
  why: the merge test's helper fixture. glm-5-turbo is an arbitrary model value; NOT builtinPi; NOT a parity
       oracle. Leave as-is.
```

### Current Codebase tree (relevant slice — POST-agy assumed)

```bash
internal/provider/
  registry.go              # preferredBuiltins (L15) + DefaultProvider — EDIT (slice + comment)
  registry_test.go         # TestPreferredBuiltins + TestDefaultProvider — EDIT
  builtin.go               # builtinPi (L42) — EDIT (DefaultModel + doc comment)
  builtin_test.go          # piTOML (L22) + PiFields + RenderedCommand_Pi — EDIT
  render.go                # Render fallback comment (~L67) — EDIT (comment only)
  render_test.go           # 3 pi/glm render tests — EDIT
  manifest.go              # Manifest type — UNCHANGED
  manifest_test.go         # piManifestTOML — LEAVE (independent structural fixture)
  merge.go / merge_test.go # MergeManifest + sampleBase — LEAVE
providers/
  pi.toml                  # default_model (L43) + comment (L21) — EDIT
  agy.toml                 # (agy task) — UNCHANGED
internal/generate/realagent_test.go  # realDefaults pi + providerNames — EDIT (opt-in test)
internal/cmd/providers_test.go       # TestProvidersShow_BuiltInTOML — EDIT
go.mod / go.sum            # UNCHANGED (no new dep; pure data + comment edits)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. All 9 edits are in-place modifications to existing files (listed above).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — 3-way parity): default_model is reflect.DeepEqual-checked across builtinPi() (Go),
// piTOML (builtin_test.go), and providers/pi.toml. go-toml decodes `default_model = ""` to NON-NIL *"".
// After the change ALL THREE must be "" or DecodeParity/pi + ProviderReferenceFiles_DecodeParity/pi FAIL.
// default_provider is ALREADY "" in all three — do NOT change/omit it (pi writes it NON-NIL empty).

// CRITICAL (#2 — render tests SPLIT, not delete): with DefaultModel="", a model="" render emits NO --model.
// So commit-pi's `pi --provider zai --model glm-5-turbo …` is no longer the shipped default — it now
// requires EXPLICIT model="glm-5-turbo". Reframe the render tests into (a) shipped-default (no model/
// provider) and (b) personal-override (explicit glm-5-turbo/zai → byte-for-byte commit-pi). PRESERVE the
// commit-pi regression (it moves to explicit params, it is not removed).

// CRITICAL (#3 — TestDefaultProvider breaks): under FR-D1, gemini(5) precedes claude(7). The existing
// case DefaultProvider(["claude","gemini"])→"claude" now returns "gemini". Rewrite the cases for FR-D1.

// CRITICAL (Render logic is UNCHANGED): the fallback `if modelToUse=="" { modelToUse = *r.DefaultModel }`
// is correct for all providers; for pi it now yields "" (no --model) — the INTENDED FR-D2 behavior. Only
// the stale render.go:~67 comment changes. Do NOT add special-casing for empty defaults.

// GOTCHA (LEAVE the independent fixtures): manifest_test.go piManifestTOML and merge_test.go sampleBase
// use glm-5-turbo as an ARBITRARY value in fixtures that are NOT the pi built-in and NOT parity oracles.
// Editing them is out-of-scope churn. The grep hits there are coincidental.

// GOTCHA (agy is a prerequisite, not a sibling edit): assume agy (P1.M2.T1.S1) is COMPLETE — 7 builtins,
// agy in preferredBuiltins (appended at end), agy test coverage done. This task REPLACES the preferredBuiltins
// slice literal with the FR-D1 order (which includes agy at rank 4). Do NOT add/remove agy or touch its tests.

// GOTCHA (realagent_test.go is opt-in): //go:build integration_real — skipped in CI. But its pi comment
// ("glm-5-turbo from manifest default") is now FALSE and its model="" would run pi without --model. Fix
// both (explicit glm-5-turbo + comment) for correctness even though CI won't catch it.

// GOTCHA (providers show marshals empty as ''): go-toml marshals *"" as `default_model = ''` (single
// quotes). So providers_test.go asserts `default_model = ''`, NOT `default_model = ""`.
```

## Implementation Blueprint

### Data models and structure

No new types. Two value changes: a `[]string` literal reorder (registry.go) and one `*string` field
(`builtinPi().DefaultModel`: `strPtr("glm-5-turbo")` → `strPtr("")`). Everything else is comments and
test expectations.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: REORDER preferredBuiltins (registry.go)
  - EDIT L15: replace the slice literal with:
      var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"}
  - EDIT L11 comment: rewrite to cite FR-D1 — "preferredBuiltins is the FR-D1 cascading provider priority
      (PRD §9.16 FR-D1): open/self-hostable harnesses first (pi, opencode, cursor, agy), closed
      subscription CLIs last (gemini, codex, claude); pi first. DefaultProvider returns the first name in
      this list that the caller reports installed. …" (keep the sync-invariant + user-defined-never-selected notes).
  - DO NOT touch DefaultProvider logic, NewRegistry, IsInstalled, etc.
  - GOTCHA: this REPLACES the whole literal (agy task left it as [..., "cursor", "agy"]); the FR-D1 order
      has agy at rank 4 (after cursor). Confirm 7 elements, pi first, agy present.

Task 2: PI DEFAULT_MODEL → "" (builtin.go)
  - EDIT builtinPi() L42: DefaultModel: strPtr("glm-5-turbo")  →  DefaultModel: strPtr("")
  - EDIT the builtinPi doc comment (L28-35): remove the "Rendered with provider=zai, model=default …
      reproduces commit-pi byte-for-byte" claim. Replace with FR-D2 framing: "Per FR-D2 (PRD §9.16/§12.3),
      the shipped pi default is DECOUPLED from any one subscription: default_model AND default_provider are
      both "" (NON-NIL empty). config init fills per-role models from the FR-D4 table; the user/config
      picks the backend. The original commit-pi setup (provider=zai, model=glm-5-turbo) is a documented
      PERSONAL OVERRIDE, not the shipped default — see TestBuiltinManifests_RenderedCommand_Pi_PersonalOverride."
  - DO NOT change DefaultProvider (already strPtr("")). DO NOT touch other builtins.

Task 3: UPDATE provider-package tests (registry_test.go, builtin_test.go, render.go comment, render_test.go)
  - registry_test.go TestPreferredBuiltins_MatchesBuiltinKeys: ADD (after the existing pi-first check):
        wantOrder := []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"}
        if !reflect.DeepEqual(preferredBuiltins, wantOrder) {
            t.Errorf("preferredBuiltins order = %v, want FR-D1 %v", preferredBuiltins, wantOrder)
        }
    (reflect is already imported in registry_test.go.)
  - registry_test.go TestDefaultProvider: REWRITE the cases for FR-D1 (see research §3 — gemini before
        claude; codex before claude; cursor before agy/gemini; pi tops; user-defined never; nil→""). Keep
        the flat-if or convert to a table; the expectations are what matter.
  - builtin_test.go piTOML (L22): default_model = "glm-5-turbo"  →  default_model = ""
        (add/update the surrounding comment to note FR-D2: default_model empty in the shipped default.)
  - builtin_test.go PiFields (~L202): assertStr(t, "DefaultModel", m.DefaultModel, "glm-5-turbo")  →  ""
  - builtin_test.go RenderedCommand_Pi: REPLACE TestBuiltinManifests_RenderedCommand_Pi_MatchesCommitPi
        with TWO tests (research §5):
        (a) TestBuiltinManifests_RenderedCommand_Pi_ShippedDefault:
            argv := renderArgs(builtinPi(), "", "", "<sys>")  // model="" provider="" → no --model/--provider
            want := []string{"pi","--system-prompt","<sys>","--no-tools","--no-extensions","--no-skills",
                             "--no-prompt-templates","--no-context-files","--no-session","-p"}
        (b) TestBuiltinManifests_RenderedCommand_Pi_PersonalOverride:
            argv := renderArgs(builtinPi(), "zai", "glm-5-turbo", "<sys>")  // explicit → commit-pi
            want := []string{"pi","--provider","zai","--model","glm-5-turbo","--system-prompt","<sys>",
                             "--no-tools","--no-extensions","--no-skills","--no-prompt-templates",
                             "--no-context-files","--no-session","-p"}
  - render.go (~L67): rewrite the stale comment — replace "lets the pi golden test pass with model="" →
        glm-5-turbo" with "lets a manifest's DefaultModel serve as the fallback when the caller omits
        model (pi's default is now "" per FR-D2, so a bare pi render emits no --model until config
        supplies one)". NO code change.
  - render_test.go TestRender_GoldenPerProvider pi row: change to the SHIPPED DEFAULT —
        {"pi", pi, "", "", "pi",
         []string{"--system-prompt","<sys>","--no-tools","--no-extensions","--no-skills",
                  "--no-prompt-templates","--no-context-files","--no-session","-p"},
         "<user>"}   // model="" provider="" → no --model/--provider; sys via flag → only <user> via stdin
  - render_test.go TestRender_Pi_ByteForByteCommitPi: pass EXPLICIT model —
        spec, err := builtinPi().Render("glm-5-turbo", "zai", "<sys>", "<user>")  // was ("", "zai", …)
        // keep wantArgs unchanged (--provider zai --model glm-5-turbo …). Add comment: FR-D2 personal-override path.
  - render_test.go TestRender_ModelDefaultFallback: pi's default is now "" → switch the fallback mechanic
        to CLAUDE (DefaultModel="sonnet"):
        byDefault, _ := builtinClaude().Render("", "", "", "")           // was builtinPi(); "" → "sonnet"
        if !containsPair(byDefault.Args, "--model", "sonnet") { t.Error(...) }
        explicit, _ := builtinClaude().Render("custom-model", "", "", "") // explicit wins
        if !containsPair(explicit.Args, "--model", "custom-model") { t.Error(...) }
        // ADD: pi (FR-D2 empty default) emits NO --model:
        piNoModel, _ := builtinPi().Render("", "zai", "", "")
        if containsToken(piNoModel.Args, "--model") { t.Errorf("pi should emit no --model by default (FR-D2): %v", piNoModel.Args) }
        // update the test comment: "claude→sonnet; pi default is now empty (FR-D2)".
  - LEAVE TestRender_SystemPromptPrependFallback / ProviderDefaultFallback / DoesNotMutateManifest (they
        don't assert on --model; still pass unchanged). LEAVE manifest_test.go piManifestTOML + merge_test.go sampleBase.

Task 4: UPDATE providers/pi.toml (parity oracle + comment)
  - EDIT L43: default_model = "glm-5-turbo"       →  default_model = ""            # FR-D2: empty in the
        shipped default; config init fills per-role (§9.16 FR-D4). The zai/glm-5-turbo setup is a personal override.
  - EDIT the rendered-command comment (L21): replace the literal `zai`/`glm-5-turbo` invocation with
        placeholders per PRD §12.3 h3.45:
        #   pi --provider <backend> --model <m> --system-prompt "<sys>" \
        #      --no-tools --no-extensions --no-skills --no-prompt-templates \
        #      --no-context-files --no-session -p   < <user payload via stdin>
        #   (Personal-override example, NOT the shipped default: <backend>=zai, <m>=glm-5-turbo = commit-pi.)
  - FIELD LINES must still equal piTOML (Task 3) BYTE-FOR-BYTE modulo comments (default_model="" both sides).
  - DO NOT change default_provider (already ""). DO NOT remove any field.

Task 5: UPDATE the opt-in + CLI tests (realagent_test.go, providers_test.go)
  - realagent_test.go realDefaults pi entry (L36):
        "pi": {"glm-5-turbo", "zai"},   // was {"", "zai"} — manifest default is now "" (FR-D2); pass the
                                        // commit-pi model explicitly. provider=zai (personal override).
  - realagent_test.go providerNames (L44): reorder to FR-D1 MINUS agy (agy excluded — experimental/non-TTY):
        var providerNames = []string{"pi", "opencode", "cursor", "gemini", "codex", "claude"}
        // comment: "FR-D1 preference order (registry.go preferredBuiltins) minus agy (experimental —
        //  non-TTY stdout drop, issue #76; not real-tested). Subtest display order only."
  - providers_test.go TestProvidersShow_BuiltInTOML (~L208): change the substring
        "default_model = 'glm-5-turbo'"  →  "default_model = ''"
        (go-toml marshals *"" single-quoted as ''.) Keep the other substrings (name/command/output/strip).

Task 6: VERIFY (no further edits)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. The LEAVE fixtures
        (manifest_test.go piManifestTOML, merge_test.go sampleBase) byte-unchanged. agy's tests green
        (untouched). No non-provider package regresses.
```

### Implementation Patterns & Key Details

```go
// THE central value changes (the entire "logic" of this task):
// registry.go:
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"} // FR-D1
// builtin.go (builtinPi):
DefaultModel: strPtr(""), // FR-D2: was strPtr("glm-5-turbo"); default_provider already strPtr("")

// THE render reframe — the shipped default now emits NEITHER --model NOR --provider (both empty):
//   renderArgs(builtinPi(), "", "", "<sys>") →
//     ["pi","--system-prompt","<sys>","--no-tools",…,"--no-session","-p"]   (NO --model, NO --provider)
// The commit-pi invocation is PRESERVED as an EXPLICIT override:
//   renderArgs(builtinPi(), "zai", "glm-5-turbo", "<sys>") →
//     ["pi","--provider","zai","--model","glm-5-turbo","--system-prompt","<sys>","--no-tools",…,"-p"]

// THE TestDefaultProvider fix — gemini now beats claude (FR-D1 ranks: gemini=5, claude=7):
//   OLD: DefaultProvider(["claude","gemini"]) == "claude"   ← WRONG under FR-D1
//   NEW: DefaultProvider(["claude","gemini"]) == "gemini"

// THE 3-way parity invariant (default_model=""):
//   builtinPi().DefaultModel == strPtr("")  ⇄  piTOML "default_model = \"\""  ⇄  providers/pi.toml "default_model = \"\""
//   All decode to NON-NIL *"". reflect.DeepEqual across all three MUST hold.

// THE LEAVE rule — these grep hits are NOT pi-built-in assertions; do NOT edit:
//   manifest_test.go piManifestTOML  (structural decode fixture; omits default_provider ⇒ already ≠ builtinPi)
//   merge_test.go sampleBase          (merge-test helper; arbitrary model value)
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): change NONE. Pure data + comment + test-expectation edits. `go mod tidy`
      MUST be a no-op. `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES: NONE added/removed. No import changes anywhere.

FROZEN/LEAVE (do NOT edit):
  - internal/provider/manifest.go (the Manifest type), manifest_test.go piManifestTOML (structural fixture).
  - internal/provider/merge.go / merge_test.go sampleBase (merge fixture).
  - agy's files/edits (builtinAgy, agyTOML, AgyFields, providers/agy.toml, providerFiles agy entry,
    KeysAndCount=7) — assumed already landed by P1.M2.T1.S1; this task does not touch them.
  - Render/Validate/Resolve/Merge/NewRegistry/DefaultProvider LOGIC (one stale render.go comment excepted).

DOWNSTREAM (NOT this task):
  - P1.M2.T2.S2 adds tooled_flags to pi + claude (orthogonal — the parity chain holds as long as Go+TOML agree).
  - P1.M3.T3 / P1.M4.T2 populate per-role models via config init (FR-D4) — this task only EMPTIES pi's default.
  - FR-R5b (provider+model coupled for multi-provider agents) is enforced in the generate/resolution layer,
    not the manifest — pi's empty defaults are correct at the manifest level.

NO NEW DATABASE / ROUTES / CLI COMMANDS / FILES.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
go build ./...          # Expect clean.
go vet ./...            # Expect clean.
gofmt -l internal/provider/ internal/generate/ internal/cmd/ providers/   # Expect empty; gofmt -w any listed.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"   # MUST be empty.
# Confirm the LEAVE fixtures are untouched:
git diff --exit-code internal/provider/manifest_test.go internal/provider/merge_test.go && echo "LEAVE fixtures UNCHANGED (expected)"
```

### Level 2: Provider-package unit tests (where the break sites live)

```bash
go test ./internal/provider/... -v
# Expected PASS — verify explicitly:
#   TestPreferredBuiltins_MatchesBuiltinKeys ... new reflect.DeepEqual FR-D1 order assertion holds
#   TestDefaultProvider ........................ reframed cases (gemini before claude, etc.)
#   TestBuiltinManifests_PiFields .............. DefaultModel == ""
#   TestBuiltinManifests_DecodeParity/pi ....... piTOML default_model="" matches builtinPi
#   TestBuiltinManifests_RenderedCommand_Pi_ShippedDefault   ... no --model/--provider
#   TestBuiltinManifests_RenderedCommand_Pi_PersonalOverride ... byte-for-byte commit-pi (explicit)
#   TestProviderReferenceFiles_DecodeParity/pi . providers/pi.toml default_model="" matches builtinPi
#   TestProviderReferenceFiles_AllBuiltinsCovered ... agy still covered (untouched)
#   TestRender_GoldenPerProvider/pi ............ shipped-default args (no --model/--provider)
#   TestRender_Pi_ByteForByteCommitPi .......... explicit glm-5-turbo → commit-pi argv
#   TestRender_ModelDefaultFallback ............ claude fallback + pi-emits-no-model
#   ALL agy tests ................................ still green (untouched)
```

### Level 3: Whole-repo build/test + CLI

```bash
go build ./...   # Expect clean.
go test ./...    # Expect all PASS. If a non-provider package breaks, it hardcoded the old order/default —
                 # fix that call site (do NOT revert the FR-D1/FR-D2 change).
go test ./internal/cmd/... -run TestProvidersShow -v   # providers_test.go: default_model = '' assertion.

# Straggler grep — confirm no DEFAULT assertion still references the old values:
grep -rn 'glm-5-turbo' internal/ providers/
# Expected remaining hits (all CORRECT/intentional):
#   manifest_test.go (piManifestTOML structural fixture — LEAVE)
#   merge_test.go (sampleBase fixture — LEAVE)
#   realagent_test.go (explicit override model — intentional)
#   render_test.go (explicit-override byte-for-byte test — intentional)
# MUST BE ABSENT from: builtin.go, builtin_test.go (piTOML/PiFields), providers/pi.toml default, providers_test.go show assertion.
grep -rn '"pi", "claude", "gemini", "opencode", "codex", "cursor"' internal/   # old full order — MUST be gone
```

### Level 4: Behavioral spot-check (proves the cascade + show output)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach
/tmp/stagecoach providers show pi | grep "default_model"   # → default_model = ''  (FR-D2)
/tmp/stagecoach providers list 2>/dev/null | head           # default-provider resolution respects FR-D1
# (DefaultProvider itself is unit-tested in registry_test.go; this is a smoke check of the CLI surface.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on edited files.
- [ ] `go test ./...` PASS (provider suite + cmd providers test + no non-provider regression).
- [ ] go.mod/go.sum byte-unchanged; the LEAVE fixtures (manifest_test piManifestTOML, merge_test sampleBase) byte-unchanged.

### Feature Validation
- [ ] `preferredBuiltins == [pi, opencode, cursor, agy, gemini, codex, claude]` (FR-D1; exact-order test green).
- [ ] `DefaultProvider` returns the FR-D1 winner in every test case (gemini before claude; etc.).
- [ ] `builtinPi().DefaultModel` is non-nil `*""` (FR-D2); `DefaultProvider` unchanged (`*""`).
- [ ] 3-way parity green: builtinPi ⇄ piTOML ⇄ providers/pi.toml (all `default_model = ""`).
- [ ] RenderedCommand_Pi covers shipped default (no model/provider) + personal override (byte-for-byte commit-pi).
- [ ] `providers show pi` outputs `default_model = ''`.

### Code Quality Validation
- [ ] Render/Validate/Resolve/Merge/NewRegistry/DefaultProvider LOGIC unchanged (render.go comment-only).
- [ ] Edits follow existing conventions (assertStr/reflect.DeepEqual/table tests; comment style).
- [ ] No out-of-scope churn (LEAVE fixtures untouched; agy edits untouched).
- [ ] Anti-patterns avoided (see below).

### Documentation
- [ ] `builtinPi` doc comment reflects FR-D2 (decoupled; personal-override framing).
- [ ] `registry.go` preferredBuiltins comment cites FR-D1.
- [ ] `providers/pi.toml` rendered-command comment uses placeholders + override note (matches PRD §12.3 h3.45).
- [ ] `render.go` stale comment rewritten (no glm-5-turbo-as-default claim).

---

## Anti-Patterns to Avoid

- ❌ **Don't delete the commit-pi render regression — REFRAME it.** FR-D2 makes `zai`/`glm-5-turbo` an
  explicit override, not the default. Keep `TestBuiltinManifests_RenderedCommand_Pi_PersonalOverride` and
  `TestRender_Pi_ByteForByteCommitPi` asserting the exact commit-pi argv, but pass the model EXPLICITLY
  (`"glm-5-turbo"`). The byte-for-byte protection stays; only its framing (default → override) changes.
- ❌ **Don't change `Render`'s fallback logic.** `if modelToUse=="" { modelToUse = *r.DefaultModel }` is
  correct; for pi it now yields "" (no --model), which IS the FR-D2 intent. Only the stale comment moves.
- ❌ **Don't break the 3-way `default_model` parity.** builtinPi (`strPtr("")`), piTOML (`default_model = ""`),
  and providers/pi.toml (`default_model = ""`) must ALL carry "". One stale `glm-5-turbo` (or an omitted
  key → nil) fails DecodeParity. `default_provider` is already "" in all three — don't change/omit it.
- ❌ **Don't forget `TestDefaultProvider`'s `["claude","gemini"]` case.** It returned "claude" under the
  old order; under FR-D1 it returns "gemini". Rewrite the cases or the test fails.
- ❌ **Don't edit the LEAVE fixtures.** `manifest_test.go piManifestTOML` (structural decode fixture,
  already ≠ builtinPi) and `merge_test.go sampleBase` (merge helper) use glm-5-turbo as an arbitrary
  value. They are not pi-built-in assertions. Touching them is churn that can break unrelated tests.
- ❌ **Don't touch agy.** agy (P1.M2.T1.S1) is a prerequisite assumed complete. This task only reorders
  the slice agy appears in and empties pi's default_model. Do not add/remove agy or edit its tests/files.
- ❌ **Don't change `default_provider`.** It's already `strPtr("")` in builtinPi/piTOML/providers/pi.toml.
  The item's "empty default_provider" is the EXISTING state, not a new edit. Changing it breaks parity.
- ❌ **Don't hardcode provider counts.** Use `len(BuiltinManifests())` / `len(preferredBuiltins)` in any
  count assertion (the agy task already moved KeysAndCount to 7 dynamically; don't reintroduce a literal).
- ❌ **Don't skip the opt-in test fix.** `realagent_test.go` is `//go:build integration_real` (not in CI),
  but its pi comment ("glm-5-turbo from manifest default") is now FALSE and model="" would mis-run. Fix it
  (explicit glm-5-turbo + comment) for correctness even though CI won't catch it.
- ❌ **Don't add tooled_flags.** That's P1.M2.T2.S2 (sibling). This task is ordering + pi default_model only.

---

## Confidence Score

**9/10** — A well-scoped "two value changes + mechanical test ripple" task. Every break site is
enumerated with file:line + old→new (verified by grep), the three-way parity chain is flagged, the
render-test split (the one genuinely tricky part) is specified with exact argv, and the LEAVE fixtures
are called out to prevent churn. The -1 reserves for the `realagent_test.go`/`providers_test.go` edits
(low-risk, but in packages outside the provider suite) and the possibility of an unseen doc that hardcodes
the old order (the Level-3 straggler grep catches it).
