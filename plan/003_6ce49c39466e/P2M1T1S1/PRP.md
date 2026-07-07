---
name: "P2.M1.T1.S1 — builtinQwenCode() + registry priority insertion + providers/qwen-code.toml (PRD §12.5.2 / §9.16 FR-D1)"
description: |

  Register `qwen-code` (Alibaba/Qwen's Gemini-CLI fork for Qwen3-Coder, npm @qwen-code/qwen-code) as an
  EXPERIMENTAL eighth built-in provider at FR-D1 priority rank 6 (between gemini and codex), with its
  reference manifest TOML. qwen-code is single-backend (DashScope), so provider_flag is empty; it mirrors
  the gemini/agy flag surface exactly (stdin delivery, -p print, -m model, --approval-mode default, no
  sys-prompt flag → prepend, nil TooledFlags → cannot stager). Ships experimental + `# TO CONFIRM` model
  token per FR-D5 (the actual token refresh + per-role tier row is S2 / P2.M1.T1.S2).

  CONTRACT (P2.M1.T1.S1, verbatim — with ONE stale-reference correction flagged in §0):
    1. RESEARCH: PRD §12.5.2 — qwen-code is a single-backend Gemini-CLI FORK tuned for Qwen3-Coder, reached
       via DashScope (DASHSCOPE_API_KEY or `qwen-code login`). Mirrors gemini/agy flag surface exactly.
       preferredBuiltins (registry.go) is currently [pi, opencode, cursor, agy, gemini, codex, claude].
       TestPreferredBuiltins_MatchesBuiltinKeys asserts order incl a wantOrder slice.
    2. INPUT: P1.M1.T1.S1 (v3 manifest schema: no DefaultProvider, has ReasoningLevels). The builtin
       registry + preferredBuiltins + builtinAgy()/builtinGemini() as the mirror templates.
    3. LOGIC: (a) Add builtinQwenCode() mirroring builtinAgy(): Name 'qwen-code', detect/command
       'qwen-code', prompt_delivery 'stdin', print_flag '-p', model_flag '-m', system_prompt_flag ''
       (prepend), provider_flag '' (single-backend), experimental=boolPtr(true), default_model
       'qwen3-coder-plus' (# TO CONFIRM per FR-D5), bare_flags ['--approval-mode','default']. Register.
       (b) Insert 'qwen-code' into preferredBuiltins between 'gemini' and 'codex' → 8-element FR-D1 order.
       (c) Create providers/qwen-code.toml mirroring the manifest (experimental + DashScope + # TO CONFIRM).
       (d) Update TestPreferredBuiltins_MatchesBuiltinKeys wantOrder + the builtin-keys count assertion.
    4. OUTPUT: qwen-code is a registered experimental built-in at the correct priority; `stagecoach
       providers list` shows it; the order test passes.
    5. DOCS: [Mode A] providers/qwen-code.toml (note experimental + DashScope); builtin.go doc comment for
       builtinQwenCode.

  ⚠️ §0 — THE CONTRACT'S `builtinFuncs` REFERENCE IS STALE. There is NO `builtinFuncs` symbol in the
  codebase (grep confirms). Registration is the MAP LITERAL in `BuiltinManifests()` (builtin.go:17-28):
  add `"qwen-code": builtinQwenCode(),` to that map. Do NOT search for / create `builtinFuncs`.

  ⚠️ §1 — builtinQwenCode() is a NEAR-VERBATIM COPY of builtinAgy() (the closest gemini-lineage twin with
  Experimental=true). ONLY 3 values differ: Name/Detect/Command="qwen-code", DefaultModel="qwen3-coder-plus",
  and the doc comment (Qwen3-Coder/DashScope). Every other field is byte-identical to agy.

  ⚠️ §2 — The model token `qwen3-coder-plus` is a PLACEHOLDER marked `# TO CONFIRM per FR-D5`. The actual
  FR-D5 token refresh + the per-role FR-D4 tier row are S2's deliverable (P2.M1.T1.S2). S1 MUST NOT touch
  `internal/config/role_defaults.go` or `docs/providers.md` (S2 owns them).

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/config/role_defaults.go (DefaultModelsForProvider — the FR-D4 tier table) → S2.
    - docs/providers.md → S2. internal/cmd/config.go + config_test.go → PARALLEL P1.M3.T1.S2.
    - The Manifest struct, Render, MergeManifest, the Git interface → COMPLETE (read-only).
    - internal/git/*, internal/decompose/*, internal/generate/*, internal/prompt/*, pkg/* → untouched.

  DELIVERABLES (1 NEW file, 4 EDITED files):
    CREATE providers/qwen-code.toml          — reference manifest doc (mirrors providers/agy.toml structure).
    EDIT   internal/provider/builtin.go      — builtinQwenCode() + add map entry in BuiltinManifests() +
                                                 update the "seven"→"eight" doc comment.
    EDIT   internal/provider/registry.go     — preferredBuiltins += "qwen-code" (between gemini, codex) +
                                                 NewRegistry map headroom `+7`→`+8` (same-file accuracy fix).
    EDIT   internal/provider/registry_test.go— TestPreferredBuiltins_MatchesBuiltinKeys wantOrder += qwen-code.
    EDIT   internal/provider/builtin_test.go — TestBuiltinManifests_KeysAndCount 7→8 + key; ADD qwenCodeTOML
                                                 const + DecodeParity entry + TestBuiltinManifests_QwenCodeFields
                                                 + TestBuiltinManifests_RenderedCommand_QwenCode.

  SUCCESS: qwen-code is registered, experimental, at FR-D1 rank 6; `providers list` shows it; the order test
  + count test pass; the new Fields/DecodeParity/RenderedCommand tests pass; `go build/vet/test ./...` green;
  go.mod/go.sum unchanged; the 5 files above are the ONLY changes.

---

## Goal

**Feature Goal**: Register `qwen-code` (PRD §12.5.2) as the eighth built-in provider — an experimental,
single-backend Gemini-CLI fork tuned for the Qwen3-Coder family (reached via DashScope) — at FR-D1 cascade
priority rank 6 (between gemini and codex), with a faithful compiled-in manifest, a human-readable reference
TOML, and the same test coverage every other built-in enjoys. The manifest mirrors the gemini/agy flag
surface exactly; what differs is the model line (Qwen3-Coder) and the experimental flag.

**Deliverable** (1 NEW + 4 EDITED):
1. **CREATE `providers/qwen-code.toml`** — reference manifest doc mirroring `providers/agy.toml`'s header
   structure + field lines, annotated experimental + DashScope + `# TO CONFIRM`.
2. **EDIT `internal/provider/builtin.go`** — add `builtinQwenCode()` (near-copy of `builtinAgy()`) +
   `"qwen-code": builtinQwenCode(),` in the `BuiltinManifests()` map + fix the doc comment "seven"→"eight".
3. **EDIT `internal/provider/registry.go`** — `preferredBuiltins` gets `"qwen-code"` between `"gemini"` and
   `"codex"`; `NewRegistry` map headroom `len(userOverrides)+7` → `+8`.
4. **EDIT `internal/provider/registry_test.go`** — `TestPreferredBuiltins_MatchesBuiltinKeys` `wantOrder`
   gains `"qwen-code"` (between gemini and codex).
5. **EDIT `internal/provider/builtin_test.go`** — `TestBuiltinManifests_KeysAndCount` `want 7`→`8` + add
   `"qwen-code"` to the keys loop; ADD `qwenCodeTOML` const + a `DecodeParity` table entry +
   `TestBuiltinManifests_QwenCodeFields` + `TestBuiltinManifests_RenderedCommand_QwenCode`.

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/` empty;
go.mod/go.sum byte-unchanged; `BuiltinManifests()` returns 8 keys incl. `"qwen-code"`; `preferredBuiltins` is
the exact 8-element FR-D1 order with qwen-code at index 5; `builtinQwenCode()` is byte-faithful to its TOML
(DecodeParity); `providers list` (via `Registry.List()`) shows qwen-code; the new Fields/RenderedCommand
tests pass; the 5 listed files are the ONLY diffs.

## User Persona

**Target User**: a developer on the Qwen3-Coder / DashScope stack who wants stagecoach to drive
`qwen-code` as its agent CLI. Today qwen-code is absent from the built-in set, so they must hand-write a
`[provider.qwen-code]` override block. After S1, `stagecoach --provider qwen-code` resolves a compiled-in
manifest (zero-config), and `config init` lists qwen-code as a (commented) switchable agent.

**Use Case**: a user with `qwen-code` on `$PATH` and `DASHSCOPE_API_KEY` set runs `stagecoach --provider
qwen-code --model qwen3-coder-plus`. stagecoach resolves the built-in manifest and renders
`qwen-code -m qwen3-coder-plus --approval-mode default -p < <payload>`. If qwen-code is the highest-priority
agent installed (FR-D1 rank 6), it is the auto-default.

**Pain Points Addressed**: qwen-code support currently requires a hand-written config block (§12.8) that is
easy to get wrong (wrong flags, missing the prepend-fallback for the absent sys-prompt flag). A compiled-in
manifest makes it work out of the box and keeps it correct via the byte-faithfulness DecodeParity test.

## Why

- **Closes PRD §12.5.2 (the provider itself) + §9.16 FR-D1 (its cascade rank).** qwen-code is a first-class
  built-in at the documented priority, not a doc-only stub a user must transcribe.
- **Mirrors a proven twin.** qwen-code is a Gemini-CLI fork with an IDENTICAL flag surface to gemini/agy,
  so the manifest is a near-verbatim copy of `builtinAgy()` (the experimental gemini-lineage twin) — low-risk,
  high-confidence, and provably byte-faithful via DecodeParity.
- **Honest progressive verification (§12.7.2).** Like agy, qwen-code ships `experimental=true` with `# TO
  CONFIRM` notes because its manifest is researched from docs, not yet `--help`-verified. S2 (P2.M1.T1.S2)
  does the FR-D5 token refresh + FR-D4 tier row; S1 ships a correct, discoverable, ready-to-verify manifest.
- **Foundation for the model-token refresh (S2).** S2 adds the qwen-code column to the FR-D4 per-role tier
  table (`role_defaults.go`) and refreshes tokens against qwen-code's live model list — it depends on the
  provider being REGISTERED first (this task).

## What

A new manifest function + map registration + a one-line priority insertion + a reference TOML + test updates
(2 mandated, 4 pattern-consistency). No new types, no interface change, no import change, no dependency change.

### Success Criteria

- [ ] `builtinQwenCode()` exists in `internal/provider/builtin.go` and returns a `Manifest` byte-identical to
      `builtinAgy()` EXCEPT: Name/Detect/Command="qwen-code", DefaultModel="qwen3-coder-plus",
      Experimental=boolPtr(true), and a Mode-A doc comment naming Qwen3-Coder/DashScope/`# TO CONFIRM`/experimental.
- [ ] `BuiltinManifests()` includes `"qwen-code": builtinQwenCode()` and its doc comment says "eight" (not "seven").
- [ ] `preferredBuiltins` (registry.go) is EXACTLY `[pi, opencode, cursor, agy, gemini, qwen-code, codex, claude]`.
- [ ] `NewRegistry`'s map headroom is `len(userOverrides)+8`.
- [ ] `TestPreferredBuiltins_MatchesBuiltinKeys` wantOrder includes qwen-code between gemini and codex.
- [ ] `TestBuiltinManifests_KeysAndCount` asserts 8 and lists qwen-code.
- [ ] `TestBuiltinManifests_QwenCodeFields` + `TestBuiltinManifests_RenderedCommand_QwenCode` pass; the
      `DecodeParity` table has a `{"qwen-code", builtinQwenCode(), qwenCodeTOML}` entry that passes.
- [ ] `providers/qwen-code.toml` mirrors the manifest field-by-field + the agy.toml header structure, noting
      experimental + DashScope + `# TO CONFIRM`.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/` empty; go.mod/go.sum
      byte-unchanged; EXACTLY the 5 listed files change.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the stale-`builtinFuncs`
correction (§0 — registration is the map literal), the 3-diff table vs `builtinAgy()` (§1/§2 of findings —
copy agy, change 3 values), the exact one-line `preferredBuiltins` edit + the `+8` headroom (§1), the 2
mandated test updates (§4), the 4 pattern-consistency test additions (copy the agy tests — §5), the
`providers/agy.toml` mirror template (§6), and the verified no-overlap with the parallel sibling (§7). No
Render/Merge/git/decompose knowledge required — this is a data-registration task.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE findings (the stale-contract correction + the 3-diff table + the test map)
- docfile: plan/003_6ce49c39466e/P2M1T1S1/research/findings.md
  why: §0 (NO builtinFuncs — register in the BuiltinManifests() map literal + fix "seven"→"eight" doc);
       §1 (preferredBuiltins one-line edit + NewRegistry +7→+8); §2 (the field-by-field agy-vs-qwen-code
       table — copy agy, change 3 values); §3 (model token is a # TO CONFIRM PLACEHOLDER; S2 owns the
       refresh — do NOT touch role_defaults.go); §4 (the 2 mandated test updates + why NO other existing test
       changes); §5 (the 4 pattern-consistency test additions, all copies of agy tests); §6 (toml mirror);
       §7 (zero overlap with parallel P1.M3.T1.S2); §8 (rendered argv).
  critical: §0 (without it the agent chases a nonexistent builtinFuncs symbol), §2 (the manifest is a
       near-verbatim agy copy — do not reinvent), §3 (do NOT touch role_defaults.go — S2's), §4 (the order of
       the 2 mandated updates must insert qwen-code between gemini and codex).

# MUST READ — the manifest being COPIED (builtinAgy is the twin); where registration happens
- file: internal/provider/builtin.go   (EDIT)
  section: `BuiltinManifests()` (L17-28 — the map literal: add `"qwen-code": builtinQwenCode()`; update the
       doc comment "All seven providers are now present" → "eight"); `builtinAgy()` (L184 — THE COPY SOURCE:
       qwen-code differs ONLY in Name/Detect/Command, DefaultModel, and the doc comment).
  why: builtinQwenCode() is a near-verbatim copy of builtinAgy(). Copy agy, change the 3 values, rewrite the
       doc comment (Qwen3-Coder / DashScope / `# TO CONFIRM` / experimental). Keep Experimental=boolPtr(true),
       TooledFlags nil, all the strPtr("") NON-NIL-empty fields, Output="raw", StripCodeFence=boolPtr(true).
  pattern: match agy's field order + comment style EXACTLY (the DecodeParity test compares struct values, but
       matching agy's style keeps the file reviewable).
  gotcha: there is NO `builtinFuncs` symbol (contract §0 is stale) — register in the BuiltinManifests() map.

# MUST READ — the priority slice + the headroom hint (both in registry.go)
- file: internal/provider/registry.go   (EDIT)
  section: `var preferredBuiltins = []string{...}` (L16 — insert "qwen-code" between "gemini" and "codex");
       `NewRegistry` `make(map[string]Manifest, len(userOverrides)+7)` (L36 — change +7 → +8).
  why: the FR-D1 rank comes ENTIRELY from preferredBuiltins (DefaultProvider/FirstTooledProvider iterate it);
       the +8 is a one-char accuracy fix in the same file (the map auto-grows, so +7 still works, but keep the
       hint honest).
  gotcha: DefaultProvider/FirstTooledProvider need NO change — they iterate preferredBuiltins dynamically.

# MUST READ — the 2 mandated test updates
- file: internal/provider/registry_test.go   (EDIT — TestPreferredBuiltins_MatchesBuiltinKeys)
  section: `wantOrder := []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"}` (L~30) →
       insert "qwen-code" between "gemini" and "codex".
  why: the only assertion that hardcodes the ORDER. (The `len(set) != len(bk)` guard is dynamic and passes.)
  gotcha: do NOT touch TestDefaultProvider / TestFirstTooledProvider — verified they pass UNCHANGED (no case
       installs qwen-code; relative ranks preserved). See findings §4.
- file: internal/provider/builtin_test.go   (EDIT — TestBuiltinManifests_KeysAndCount + ADD tests)
  section: `TestBuiltinManifests_KeysAndCount` (L194 — `want 7`→`want 8`; add "qwen-code" to the keys slice);
       `agyTOML` const (L129 — the COPY SOURCE for qwenCodeTOML); `TestBuiltinManifests_DecodeParity` table
       (L347 — add `{"qwen-code", builtinQwenCode(), qwenCodeTOML}`); `TestBuiltinManifests_AgyFields` (L662 —
       the COPY SOURCE for the QwenCode Fields test); `TestBuiltinManifests_RenderedCommand_Agy` (L706 — the
       COPY SOURCE for the QwenCode rendered-command test).
  why: the count test hardcodes 7; DecodeParity is THE byte-faithfulness keystone (every builtin has an entry);
       a Fields + RenderedCommand test exist for EVERY builtin — qwen-code must match.
  pattern: qwenCodeTOML = agyTOML with name/detect/command="qwen-code" + default_model="qwen3-coder-plus"
       (keep experimental=true; omit the same nil fields agy omits). QwenCodeFields = AgyFields with the 3
       changed values. RenderedCommand_QwenCode want = ["qwen-code","-m","qwen3-coder-plus","--approval-mode","default","-p"].
  gotcha: DecodeParity DeepEqual requires qwenCodeTOML to set EXACTLY the non-nil fields the manifest sets and
       OMIT the nil ones — copy agyTOML's field set verbatim (it already matches agy's nil/non-nil pattern).

# MUST READ — the toml reference-file template
- file: providers/agy.toml   (READ — the COPY SOURCE for providers/qwen-code.toml)
  section: the big comment header (WHAT THIS FILE IS / HOW TO USE IT AS A CONFIG OVERRIDE / RENDERED COMMAND /
       EXPERIMENTAL / STAGER / TOOLS-DISABLE CATEGORY) + the field lines.
  why: providers/qwen-code.toml mirrors this structure exactly, adapted for qwen-code (Qwen3-Coder/DashScope
       in the header; qwen-code field values; `# TO CONFIRM` notes on model flag / default model / reasoning
       levels / approval-mode gemini-equivalence; experimental=true).
  gotcha: these files are NOT loaded at runtime (built-ins are compiled in) — they are human reference docs.
       Mirror the manifest field-by-field (modulo comments).

# MUST READ — the Manifest struct (v3 schema — the type builtinQwenCode returns)
- file: internal/provider/manifest.go   (READ-ONLY)
  section: `type Manifest struct` — note: NO DefaultProvider field in v3 (the field is ProviderFlag; the
       inference backend is the model slash-prefix per FR-R5b). Experimental is `*bool` (boolPtr). ReasoningLevels
       is `map[string][]string` (leave nil — qwen-code has no verified reasoning tokens, `# TO CONFIRM`).
  why: confirms the v3 schema S1 builds against; strPtr/boolPtr are the same-package helpers to use.
  gotcha: leave ReasoningLevels nil (nil is a graceful no-op per FR-R6; `# TO CONFIRM` per FR-D5 = S2).

# The PRD basis (in your context as selected_prd_content)
- file: PRD.md
  section: §12.5.2 (h3.49 — the qwen-code manifest + the `# TO CONFIRM` items); §9.16 FR-D1 (h3.32 — the
       8-element cascade order with qwen-code at rank 6); §12.1 (h3.43 — the manifest schema); §12.5.1 (agy,
       the twin the manifest mirrors).
  critical: §12.5.2 mandates experimental + single-backend + the gemini-lineage flag surface; FR-D1 mandates
       the EXACT 8-element order [pi, opencode, cursor, agy, gemini, qwen-code, codex, claude].

# The parallel sibling (coordination — ZERO overlap)
- docfile: plan/003_6ce49c39466e/P1M3T1S2/PRP.md   (PARALLEL — config upgrade on-disk)
  why: it touches internal/cmd/config.go + internal/cmd/config_test.go ONLY. This task touches internal/provider/*
       + providers/qwen-code.toml ONLY. No shared file — no coordination needed beyond confirming non-overlap.
  critical: adding qwen-code to the registry is TRANSPARENT to the config layer (bootstrap iterates
       BuiltinManifests() dynamically; v3 migration's v2MultiBackendBuiltins={"pi"} is unaffected — qwen-code
       is single-backend).
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/
  manifest.go       # READ-ONLY: Manifest struct (v3), strPtr/boolPtr, Resolve/Validate/DetectCommand.
  builtin.go        # EDIT: add builtinQwenCode() (near-copy of builtinAgy) + BuiltinManifests() map entry +
                    #   fix the "seven"→"eight" doc comment.
  registry.go       # EDIT: preferredBuiltins += "qwen-code" (between gemini, codex); NewRegistry +7→+8.
  builtin_test.go   # EDIT: KeysAndCount 7→8 + key; ADD qwenCodeTOML const + DecodeParity entry +
                    #   TestBuiltinManifests_QwenCodeFields + TestBuiltinManifests_RenderedCommand_QwenCode.
  registry_test.go  # EDIT: TestPreferredBuiltins_MatchesBuiltinKeys wantOrder += "qwen-code".
  {merge,render,parse,executor,...}.go  # UNCHANGED (COMPLETE — read-only).
providers/
  agy.toml          # READ — the COPY SOURCE for qwen-code.toml.
  qwen-code.toml    # CREATE — reference manifest doc (mirrors agy.toml structure + qwen-code content).
go.mod / go.sum     # UNCHANGED (no new import — only stdlib + existing go-toml).
```

### Desired Codebase tree with files to be added/changed

```bash
providers/qwen-code.toml          # CREATE. Reference manifest doc: header (WHAT/HOW/RENDERED/EXPERIMENTAL/
                                  #   STAGER/TOOLS-DISABLE) + field lines mirroring builtinQwenCode().
internal/provider/builtin.go      # EDIT. + builtinQwenCode() (copy of builtinAgy, 3 values changed);
                                  #   + "qwen-code": builtinQwenCode() in BuiltinManifests(); doc "eight".
internal/provider/registry.go     # EDIT. preferredBuiltins: gemini,qwen-code,codex; NewRegistry +8.
internal/provider/registry_test.go# EDIT. wantOrder += "qwen-code" (between gemini, codex).
internal/provider/builtin_test.go # EDIT. KeysAndCount 7→8+key; + qwenCodeTOML const; + DecodeParity entry;
                                  #   + TestBuiltinManifests_QwenCodeFields; + TestBuiltinManifests_RenderedCommand_QwenCode.
# go.mod/go.sum UNCHANGED. Manifest struct/Render/Merge UNCHANGED. role_defaults.go + docs/providers.md = S2 (UNTOUCHED).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (§0 — NO builtinFuncs): the contract says "Register in builtinFuncs" but that symbol DOES NOT
// EXIST. Registration is the map literal in BuiltinManifests() (builtin.go:17-28): add
// "qwen-code": builtinQwenCode(). Do NOT grep for / create builtinFuncs.

// CRITICAL (§2 — copy builtinAgy, change 3 values): builtinQwenCode() is byte-identical to builtinAgy()
// EXCEPT Name/Detect/Command="qwen-code", DefaultModel="qwen3-coder-plus", and the doc comment. Do NOT
// reinvent the manifest — agy is the gemini-lineage experimental twin; qwen-code is the same shape. Keep
// Experimental=boolPtr(true), TooledFlags nil, all strPtr("") NON-NIL-empty fields, Output="raw",
// StripCodeFence=boolPtr(true), PromptDelivery="stdin", PrintFlag="-p", ModelFlag="-m".

// CRITICAL (§3 — model token is a PLACEHOLDER; S2 owns the refresh): DefaultModel="qwen3-coder-plus" is
// `# TO CONFIRM per FR-D5` (the codebase already refreshed OTHER gemini-lineage tokens to real current
// values, e.g. gemini-2.5-pro — that refresh for qwen-code is S2). Do NOT touch role_defaults.go (the
// FR-D4 per-role tier table) or docs/providers.md — both are S2's deliverable. S1 ships a correct, marked,
// experimental manifest.

// CRITICAL (DecodeParity byte-faithfulness): qwenCodeTOML must set EXACTLY the non-nil fields the manifest
// sets and OMIT the nil ones — copy agyTOML's field set verbatim (name/detect/command/prompt_delivery/
// print_flag/model_flag/default_model/system_prompt_flag/provider_flag/bare_flags/output/strip_code_fence/
// experimental). OMIT subcommand/prompt_flag/json_field/retry_instruction/tooled_flags/env/reasoning_levels
// (nil in the manifest). DeepEqual(struct, decoded-toml) passes only if the nil/non-nil pattern matches.

// GOTCHA (preferredBuiltins EXACT order): the test uses reflect.DeepEqual(preferredBuiltins, wantOrder) —
// the slice must be EXACTLY [pi, opencode, cursor, agy, gemini, qwen-code, codex, claude]. Insert qwen-code
// at index 5 (between gemini and codex), not appended.

// GOTCHA (NewRegistry +8 is a hint, not a cap): make(map, n) pre-sizes but auto-grows; +7 still WORKS.
// Update to +8 for accuracy (same-file edit). Not test-asserted.

// GOTCHA (existing order/count tests are the ONLY mandatory updates): TestDefaultProvider /
// TestFirstTooledProvider pass UNCHANGED — verified (no case installs qwen-code; relative ranks preserved
// by inserting qwen-code between gemini(5) and codex(7→8)). Do NOT edit them. See findings §4.

// GOTCHA (no new imports): builtin.go/registry.go use only existing imports (strPtr/boolPtr are same-pkg).
// go.mod/go.sum byte-unchanged.

// GOTCHA (providers/*.toml are NOT runtime-loaded): they are human reference docs. They mirror the compiled-in
// manifest for readers; the config loader reads .stagecoach.toml, NOT this directory. Byte-faithfulness to the
// manifest is a documentation goal, not a runtime requirement.

// GOTCHA (parallel sibling = internal/cmd/* only): P1.M3.T1.S2 edits config.go/config_test.go. This task edits
// internal/provider/* + providers/qwen-code.toml. No shared file. Adding qwen-code is transparent to config
// (bootstrap iterates BuiltinManifests() dynamically; qwen-code is single-backend so v3 migration is unaffected).
```

## Implementation Blueprint

### Data models and structure

No new TYPES. `builtinQwenCode()` returns the existing `Manifest` struct. The only structural change is one
new map entry + one new slice element. The v3 schema (no DefaultProvider field; ProviderFlag is the
sub-provider flag; ReasoningLevels is a map) is unchanged and consumed as-is.

```go
// === internal/provider/builtin.go — ADD builtinQwenCode() (near-verbatim copy of builtinAgy) ===

// builtinQwenCode returns the qwen-code (Alibaba/Qwen) manifest per PRD §12.5.2. qwen-code
// (npm @qwen-code/qwen-code; GitHub QwenLM/qwen-code) is a FORK of Google's Gemini CLI tuned for the
// Qwen3-Coder family, reached via Alibaba Cloud Model Studio / DashScope (DASHSCOPE_API_KEY, or
// `qwen-code login` for the free coding-plan quota). It is SINGLE-BACKEND (Qwen/DashScope), so
// provider_flag is empty and a bare model is used. Its flag surface mirrors gemini/agy (§12.5/§12.5.1)
// EXACTLY: stdin delivery, -m model, --approval-mode default (read-only), no first-class system-prompt
// flag → sys is PREPENDED to the payload (§12.2).
//
// Flag surface assembled from qwen-code's docs (NOT yet `--help`-verified) → ships Experimental=true
// (§12.7.2) until a real end-to-end run clears it. Marked `# TO CONFIRM` per FR-D5: the exact default
// model token (qwen3-coder-plus et al.), the model-flag token, the reasoning_levels mapping, and the
// gemini-equivalent approval mode. The FR-D5 token refresh + the per-role FR-D4 tier row are S2
// (P2.M1.T1.S2); this manifest ships a correct, documented, experimental PLACEHOLDER.
//
// STAGER: TooledFlags is intentionally nil — qwen-code CANNOT serve as a stager until the scoped,
// non-interactive, git-scoped tool combo is verified (FR-D4 fallback). RenderTooled errors on nil tooled_flags.
//
// NOTE: (1) PrintFlag="-p" (NON-NIL). (2) SystemPromptFlag/ProviderFlag are strPtr("") — NON-NIL empty:
// no sys flag (sys prepended, §12.2), single-backend (no sub-provider). (3) Experimental=boolPtr(true).
// (4) DefaultModel="qwen3-coder-plus" (# TO CONFIRM FR-D5). (5) Subcommand/PromptFlag/JsonField/
// RetryInstruction/Env/TooledFlags/ReasoningLevels are nil (absent, like agy). qwen-code is the
// gemini-lineage twin of agy, differing in Name/Detect/Command + DefaultModel + the Qwen/DashScope context.
func builtinQwenCode() Manifest {
	return Manifest{
		Name:             "qwen-code",
		Detect:           strPtr("qwen-code"),
		Command:          strPtr("qwen-code"),
		PromptDelivery:   strPtr("stdin"),
		PrintFlag:        strPtr("-p"),
		ModelFlag:        strPtr("-m"),
		DefaultModel:     strPtr("qwen3-coder-plus"), // # TO CONFIRM per FR-D5 (S2 owns the refresh)
		SystemPromptFlag: strPtr(""),                  // NON-NIL empty — no sys flag; sys prepended to payload (§12.2)
		ProviderFlag:     strPtr(""),                  // NON-NIL empty — single-backend (Qwen/DashScope)
		BareFlags: []string{
			"--approval-mode", "default", // read-only, never-ask profile (don't auto-run tools). # TO CONFIRM gemini-equivalent
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		Experimental:   boolPtr(true), // §12.5.2/§12.7.2 ships experimental (docs-sourced, not --help-verified)
		// TooledFlags: nil — qwen-code cannot stager until the scoped tool combo is verified (FR-D4 fallback).
		// Subcommand, PromptFlag, JsonField, RetryInstruction, Env, ReasoningLevels: nil (absent, like agy).
	}
}
```

```go
// === internal/provider/builtin.go — EDIT BuiltinManifests() (add the map entry + fix the doc comment) ===
// In the returned map literal, add (placement is not load-bearing — List() sorts; group with the gemini-lineage):
//		"qwen-code": builtinQwenCode(),
// In the doc comment, change "All seven providers are now present." → "All eight providers are now present."
// (and optionally note qwen-code as the §12.5.2 experimental Gemini-CLI fork).
```

```go
// === internal/provider/registry.go — EDIT preferredBuiltins + NewRegistry headroom ===
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "qwen-code", "codex", "claude"}
//   ↑ insert "qwen-code" between "gemini" and "codex" (FR-D1 rank 6). Update the doc comment's example
//     list if it enumerates the names (it currently says "pi, opencode, cursor, agy" then "gemini, codex,
//     claude" — append qwen-code to keep the comment honest).

// In NewRegistry:
//	manifests := make(map[string]Manifest, len(userOverrides)+8) // built-ins + overrides headroom (was +7)
```

```go
// === internal/provider/builtin_test.go — ADD qwenCodeTOML const (next to agyTOML, L129) ===
const qwenCodeTOML = `name = "qwen-code"
detect = "qwen-code"
command = "qwen-code"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "-m"
default_model = "qwen3-coder-plus"
system_prompt_flag = ""
provider_flag = ""
bare_flags = [
  "--approval-mode", "default",
]
output = "raw"
strip_code_fence = true
experimental = true
`
// (Identical field set to agyTOML — copy it, change name/detect/command + default_model. OMIT the same nil
//  fields agy omits so DecodeParity DeepEqual passes.)
```

```go
// === internal/provider/builtin_test.go — ADD TestBuiltinManifests_QwenCodeFields (copy of AgyFields) ===
func TestBuiltinManifests_QwenCodeFields(t *testing.T) {
	m := builtinQwenCode()
	assertStr(t, "Detect", m.Detect, "qwen-code")
	assertStr(t, "Command", m.Command, "qwen-code")
	assertStr(t, "PromptDelivery", m.PromptDelivery, "stdin")
	assertStr(t, "PrintFlag", m.PrintFlag, "-p")
	assertStr(t, "ModelFlag", m.ModelFlag, "-m")
	assertStr(t, "DefaultModel", m.DefaultModel, "qwen3-coder-plus") // # TO CONFIRM FR-D5
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "")          // NON-NIL explicit empty (prepend)
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")                  // NON-NIL explicit empty (single-backend)
	wantBare := []string{"--approval-mode", "default"}
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
	}
	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}
	if m.Experimental == nil || *m.Experimental != true {
		t.Errorf("Experimental = %v, want non-nil true (§12.5.2 ships experimental)", m.Experimental)
	}
	if m.TooledFlags != nil {
		t.Errorf("TooledFlags = %v, want nil (cannot stager until verified)", m.TooledFlags)
	}
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

// === ADD TestBuiltinManifests_RenderedCommand_QwenCode (copy of RenderedCommand_Agy) ===
func TestBuiltinManifests_RenderedCommand_QwenCode(t *testing.T) {
	argv := renderArgs(builtinQwenCode(), "", "", "<sys>") // model="" → default qwen3-coder-plus
	want := []string{
		"qwen-code", "-m", "qwen3-coder-plus",
		"--approval-mode", "default",
		"-p", // print_flag LAST per §12.2
		// stdin delivery: "<sys>\n\n<user payload>" piped to stdin (NOT in argv). No sys/provider flag.
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("qwen-code rendered argv:\n got %v\nwant %v", argv, want)
	}
}

// === ADD the DecodeParity table entry (in TestBuiltinManifests_DecodeParity's table slice) ===
//		{"qwen-code", builtinQwenCode(), qwenCodeTOML}, // qwenCodeTOML = §12.5.2 (experimental=true)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: internal/provider/builtin.go — ADD builtinQwenCode() + register + fix doc comment
  - ADD builtinQwenCode() (the Blueprint above — near-verbatim copy of builtinAgy, 3 values changed).
  - ADD "qwen-code": builtinQwenCode() to the BuiltinManifests() map literal (group with gemini/agy).
  - FIX the BuiltinManifests() doc comment: "All seven providers are now present." → "eight" (and note qwen-code).
  - GOTCHA: there is NO builtinFuncs symbol — register in the map literal. Do NOT reinvent the manifest; copy agy.
  - GOTCHA: leave ReasoningLevels nil (# TO CONFIRM FR-D5 = S2). Keep Experimental=boolPtr(true), TooledFlags nil.

Task 2: internal/provider/registry.go — preferredBuiltins += qwen-code + NewRegistry +8
  - EDIT preferredBuiltins: insert "qwen-code" between "gemini" and "codex" (FR-D1 rank 6, index 5).
  - EDIT NewRegistry: len(userOverrides)+7 → +8 (accuracy hint; same file).
  - UPDATE the preferredBuiltins doc comment's name list if it enumerates names (keep it honest).
  - GOTCHA: DefaultProvider/FirstTooledProvider need NO change (they iterate preferredBuiltins dynamically).

Task 3: internal/provider/registry_test.go — wantOrder += qwen-code
  - EDIT TestPreferredBuiltins_MatchesBuiltinKeys wantOrder: insert "qwen-code" between "gemini" and "codex".
  - GOTCHA: do NOT touch TestDefaultProvider / TestFirstTooledProvider (verified UNCHANGED — findings §4).

Task 4: internal/provider/builtin_test.go — count fix + qwenCodeTOML + DecodeParity + 2 new tests
  - EDIT TestBuiltinManifests_KeysAndCount: `want 7` → `want 8`; add "qwen-code" to the keys slice.
  - ADD qwenCodeTOML const (copy agyTOML, change name/detect/command + default_model — Blueprint above).
  - ADD the DecodeParity table entry {"qwen-code", builtinQwenCode(), qwenCodeTOML}.
  - ADD TestBuiltinManifests_QwenCodeFields (copy AgyFields, 3 changed values — Blueprint above).
  - ADD TestBuiltinManifests_RenderedCommand_QwenCode (copy RenderedCommand_Agy — Blueprint above).
  - GOTCHA: qwenCodeTOML must OMIT the same nil fields agyTOML omits (DecodeParity DeepEqual). Copy agyTOML's set.

Task 5: CREATE providers/qwen-code.toml (mirror providers/agy.toml structure)
  - COPY the providers/agy.toml header structure (WHAT THIS FILE IS / HOW TO USE IT AS A CONFIG OVERRIDE /
    RENDERED COMMAND / EXPERIMENTAL / STAGER / TOOLS-DISABLE CATEGORY) and adapt for qwen-code:
      - RENDERED COMMAND: `qwen-code -m qwen3-coder-plus --approval-mode default -p < "<sys>\n\n<user payload>"`
      - EXPERIMENTAL: docs-sourced (not --help-verified) per §12.5.2/§12.7.2; # TO CONFIRM items (model flag,
        exact default model, reasoning levels, gemini-equivalent approval mode).
      - DashScope: reached via DASHSCOPE_API_KEY or `qwen-code login` (free coding-plan quota); single-backend.
      - STAGER: TooledFlags nil → cannot stager until verified (FR-D4 fallback).
  - FIELD LINES mirror builtinQwenCode() byte-for-byte (modulo comments): name/detect/command="qwen-code",
    prompt_delivery="stdin", print_flag="-p", model_flag="-m", default_model="qwen3-coder-plus" (# TO CONFIRM),
    system_prompt_flag="" (prepend), provider_flag="" (single-backend), bare_flags=["--approval-mode","default"],
    output="raw", strip_code_fence=true, experimental=true. OMIT (note as absent) subcommand/prompt_flag/
    json_field/retry_instruction/tooled_flags/env/reasoning_levels.
  - GOTCHA: NOT runtime-loaded — human reference doc only. Byte-faithfulness to the manifest is the doc goal.

Task 6: VERIFY (run all gates; fix before declaring done)
  - gofmt -w internal/provider/{builtin.go,builtin_test.go,registry.go,registry_test.go}
  - go build ./... && go vet ./internal/provider/
  - go test -race ./internal/provider/ -run "TestPreferredBuiltins|TestBuiltinManifests|TestDefaultProvider|TestFirstTooledProvider|TestNewRegistry" -v
  - go test ./...   (full regression — NO existing test should change behavior except the 2 mandated assertion updates)
  - git diff --exit-code go.mod go.sum → empty.
  - git status → EXACTLY 5 files (1 new providers/qwen-code.toml + 4 edited). role_defaults.go/docs/providers.md/cmd/* UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// PATTERN (copy the proven twin): builtinQwenCode() == builtinAgy() with 3 changed values. Do not reinvent.
//   agy is the gemini-lineage experimental twin; qwen-code is the same shape (fork of the same CLI).

// PATTERN (registration = the map literal, NOT a func slice): there is no builtinFuncs. Add the map entry.
return map[string]Manifest{
	...
	"qwen-code": builtinQwenCode(),
}

// PATTERN (FR-D1 rank comes ENTIRELY from preferredBuiltins): one-line edit; DefaultProvider/
//   FirstTooledProvider iterate it dynamically — no method change needed.
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "qwen-code", "codex", "claude"}

// PATTERN (DecodeParity byte-faithfulness): qwenCodeTOML mirrors agyTOML's field set exactly (set the
//   non-nil fields, omit the nil ones) so DeepEqual(struct, decoded-toml) passes.

// CRITICAL (model token is a PLACEHOLDER): DefaultModel="qwen3-coder-plus" # TO CONFIRM FR-D5. The refresh +
//   the per-role FR-D4 tier row are S2's. Do NOT touch role_defaults.go or docs/providers.md.

// CRITICAL (experimental + single-backend): Experimental=boolPtr(true); ProviderFlag=strPtr("") (NON-NIL
//   empty). TooledFlags nil → cannot stager (FR-D4 fallback). Matches agy exactly.
```

### Integration Points

```yaml
PROVIDER REGISTRY (internal/provider/builtin.go + registry.go — EDIT):
  - add: "builtinQwenCode() (Manifest) + BuiltinManifests() map entry; preferredBuiltins += 'qwen-code' (rank 6);
          NewRegistry map headroom +8."
  - effect: "qwen-code is a registered experimental built-in at FR-D1 rank 6; Registry.List() includes it;
          DefaultProvider/FirstTooledProvider consider it (single-backend, nil TooledFlags); `stagecoach
          providers list` shows it; `config init` lists it as a commented switchable agent (bootstrap iterates
          BuiltinManifests() dynamically — no config change needed)."

REFERENCE DOC (providers/qwen-code.toml — NEW):
  - add: "human-readable manifest reference mirroring builtinQwenCode() + providers/agy.toml structure."

GO MODULE (go.mod/go.sum): change NONE. No new import (strPtr/boolPtr are same-package; stdlib only).

UPSTREAM (consume, do NOT edit): the v3 Manifest struct (manifest.go) — no DefaultProvider field, ProviderFlag
      is the sub-provider flag, ReasoningLevels is a map (leave nil). P1.M1.T1.S1 (the v3 schema) is COMPLETE.

DOWNSTREAM (consumers — not this task):
  - S2 (P2.M1.T1.S2): adds the qwen-code FR-D4 tier row to internal/config/role_defaults.go +
        refreshes the model token per FR-D5 + updates docs/providers.md. S1 MUST NOT touch those.

FROZEN/LEAVE (do NOT edit):
  - internal/config/role_defaults.go (FR-D4 tier table) + docs/providers.md → S2.
  - internal/cmd/config.go + internal/cmd/config_test.go → PARALLEL P1.M3.T1.S2.
  - manifest.go, merge.go, render.go, parse.go, the Git interface, internal/{git,decompose,generate,prompt}/*,
    pkg/*, go.mod, Makefile, PRD.md.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach
gofmt -w internal/provider/builtin.go internal/provider/builtin_test.go \
          internal/provider/registry.go internal/provider/registry_test.go
go vet ./internal/provider/
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; gofmt -l empty; go.mod/go.sum unchanged.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The registration + priority + count + parity + fields + rendered-command:
go test -race ./internal/provider/ -run "TestPreferredBuiltins_MatchesBuiltinKeys|TestBuiltinManifests_KeysAndCount|TestBuiltinManifests_DecodeParity|TestBuiltinManifests_QwenCodeFields|TestBuiltinManifests_RenderedCommand_QwenCode|TestBuiltinManifests_Validate|TestBuiltinManifests_NameMatchesKey" -v
# Expected: all green. Specifically:
#   TestPreferredBuiltins_MatchesBuiltinKeys   → 8-element order with qwen-code at index 5
#   TestBuiltinManifests_KeysAndCount          → want 8, has "qwen-code"
#   TestBuiltinManifests_DecodeParity          → qwen-code entry: built-in == decoded qwenCodeTOML
#   TestBuiltinManifests_QwenCodeFields        → all fields match (experimental true, tooled nil)
#   TestBuiltinManifests_RenderedCommand_QwenCode → argv ["qwen-code","-m","qwen3-coder-plus","--approval-mode","default","-p"]

# Regression: the order-sensitive + count-sensitive tests that must STILL pass UNCHANGED:
go test -race ./internal/provider/ -run "TestDefaultProvider|TestFirstTooledProvider|TestNewRegistry|TestList_SortedByName|TestMarshalTOML" -v
# Expected: all green UNCHANGED (no case installs qwen-code; relative ranks preserved; dynamic len guards).

# Full provider suite + full module:
go test -race ./internal/provider/ -v
go test ./...
```

### Level 3: Integration / Behavioral Proof

```bash
make build

# `providers list` shows qwen-code (experimental); the cascade order is correct.
# (qwen-code is almost certainly NOT on $PATH in CI — it appears as not-installed, which is fine.)
./bin/stagecoach providers list | grep -i qwen-code && echo "PASS: qwen-code listed" || echo "FAIL: qwen-code missing"

# providers show qwen-code prints the merged manifest as TOML (experimental=true, default_model qwen3-coder-plus).
./bin/stagecoach providers show qwen-code | grep -E 'qwen3-coder-plus|experimental' && echo "PASS: manifest fields present"

# Verify the FR-D1 priority: if ONLY qwen-code were installed it would be the default (rank 6). This is
# covered by TestDefaultProvider's logic; the CLI smoke test just confirms listing/show work.
```

### Level 4: Regression & Audit

```bash
cd /home/dustin/projects/stagecoach
go build ./...                 # whole module compiles
go test ./...                  # FULL regression — only the 2 mandated assertion updates change behavior
git status --short             # Expected: EXACTLY 5 files:
                               #   ?? providers/qwen-code.toml
                               #   M  internal/provider/builtin.go
                               #   M  internal/provider/builtin_test.go
                               #   M  internal/provider/registry.go
                               #   M  internal/provider/registry_test.go
# Expected: build + full test green; only the 5 files; go.mod/go.sum unchanged; role_defaults.go /
#   docs/providers.md / internal/cmd/* byte-unchanged (S2's / the parallel sibling's).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l internal/` empty; `go vet ./internal/provider/` clean.
- [ ] Level 2: `go test -race ./internal/provider/` green — incl. the new QwenCode Fields/DecodeParity/RenderedCommand + the updated count/order tests; the UNCHANGED DefaultProvider/FirstTooledProvider pass.
- [ ] Level 3: `providers list` shows qwen-code; `providers show qwen-code` prints the manifest (experimental + default_model).
- [ ] Level 4: `go build ./...` + `go test ./...` green; `git status` shows EXACTLY 5 files; go.mod/go.sum unchanged.

### Feature Validation

- [ ] `builtinQwenCode()` returns a Manifest byte-identical to `builtinAgy()` except Name/Detect/Command="qwen-code", DefaultModel="qwen3-coder-plus", Experimental=true, + a Mode-A doc comment (Qwen3-Coder/DashScope/# TO CONFIRM/experimental).
- [ ] `BuiltinManifests()` has 8 keys incl. "qwen-code"; doc comment says "eight".
- [ ] `preferredBuiltins` is EXACTLY `[pi, opencode, cursor, agy, gemini, qwen-code, codex, claude]` (qwen-code at index 5).
- [ ] `providers/qwen-code.toml` mirrors the manifest + agy.toml structure, noting experimental + DashScope + # TO CONFIRM.
- [ ] DecodeParity passes for qwen-code (built-in == decoded qwenCodeTOML — byte-faithful).

### Code Quality Validation

- [ ] `builtinQwenCode()` copies the proven `builtinAgy()` pattern (gemini-lineage experimental twin) — no reinvention.
- [ ] Registration is the `BuiltinManifests()` map literal (NOT a nonexistent `builtinFuncs` — contract §0 stale-ref corrected).
- [ ] `# TO CONFIRM per FR-D5` marks the placeholder model token; S2's role_defaults.go/docs untouched.
- [ ] File placement matches the desired tree (only the 5 listed files); no cross-package churn.
- [ ] Anti-patterns avoided (see below): no builtinFuncs chase, no reinvented manifest, no role_defaults/docs edit, no interface change.

### Documentation & Deployment

- [ ] `builtinQwenCode()` has a Mode-A doc comment (PRD §12.5.2, DashScope, single-backend, experimental, # TO CONFIRM, stager-nil).
- [ ] `providers/qwen-code.toml` documents the manifest + DashScope + experimental status (Mode A).
- [ ] The `BuiltinManifests()` doc comment is updated (seven → eight).
- [ ] Implementation summary records: the stale-builtinFuncs correction, the 3-diff agy copy, the priority insertion, the S1/S2 split.

---

## Anti-Patterns to Avoid

- ❌ **Don't search for / create `builtinFuncs`.** The contract's reference is STALE — that symbol does not
  exist. Registration is the MAP LITERAL in `BuiltinManifests()`. Add `"qwen-code": builtinQwenCode()` there.
- ❌ **Don't reinvent the manifest.** `builtinQwenCode()` is a near-verbatim copy of `builtinAgy()` (the
  gemini-lineage experimental twin) with 3 changed values. Copy agy; do not derive the flag surface from scratch.
- ❌ **Don't touch `internal/config/role_defaults.go` or `docs/providers.md`.** The FR-D4 per-role tier row +
  the FR-D5 token refresh + docs/providers.md are S2's (P2.M1.T1.S2) deliverable. S1 ships the manifest +
  registration + reference toml ONLY. Editing role_defaults.go = a conflict with S2.
- ❌ **Don't change `DefaultProvider`/`FirstTooledProvider`.** They iterate `preferredBuiltins` dynamically —
  the one-line slice edit propagates the new rank automatically. Verified: `TestDefaultProvider` /
  `TestFirstTooledProvider` pass UNCHANGED.
- ❌ **Don't break DecodeParity byte-faithfulness.** `qwenCodeTOML` must set EXACTLY the non-nil fields the
  manifest sets and OMIT the nil ones. Copy `agyTOML`'s field set verbatim (it already matches agy's
  nil/non-nil pointer pattern, which qwen-code shares).
- ❌ **Don't append qwen-code to the END of `preferredBuiltins`/`wantOrder`.** FR-D1 rank is 6 — it goes
  BETWEEN gemini and codex. The order test uses `reflect.DeepEqual` (exact slice).
- ❌ **Don't populate `ReasoningLevels`.** It's `# TO CONFIRM per FR-D5` = S2. Leave it nil (nil is a graceful
  no-op per FR-R6). The PRD §12.5.2 manifest explicitly comments the reasoning_levels mapping as # TO CONFIRM.
- ❌ **Don't add imports or touch go.mod/go.sum.** Only stdlib + existing go-toml are used; strPtr/boolPtr are
  same-package. No new dependency.
- ❌ **Don't edit the Manifest struct or add to the Git interface.** The v3 schema (P1.M1.T1.S1) is COMPLETE.
  This is a pure data-registration task — one new function, one map entry, one slice element, one toml, tests.
- ❌ **Don't edit `internal/cmd/*`.** That is the PARALLEL sibling P1.M3.T1.S2's domain. Zero overlap — keep it
  that way (this task is internal/provider/* + providers/qwen-code.toml ONLY).
