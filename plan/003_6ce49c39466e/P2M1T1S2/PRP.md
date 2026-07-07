---
name: "P2.M1.T1.S2 — qwen-code tier row (FR-D4) + FR-D5 model-token refresh + docs/providers.md"
description: |

  Add the `qwen-code` column to the FR-D4 per-role tier table (`internal/config/role_defaults.go`) and
  refresh model tokens to CURRENT per FR-D5 (PRD §9.16): gemini/agy flagship `gemini-3.5-pro`→`gemini-3.1-pro`
  in the tier table + `gemini-2.5-pro`→`gemini-3.1-pro` in the manifest `default_model` (builtin.go) + the
  matching providers/*.toml + docs/providers.md. qwen-code ships its tier values as `# TO CONFIRM`
  placeholders (live CLI lookup not available this pass).

  CONTRACT (P2.M1.T1.S2, verbatim — with ONE necessary correctness deviation flagged in §1):
    1. RESEARCH NOTE: PRD §9.16 FR-D3/D4/D5 — tier-based per-role defaults. role_defaults.go:32-87 is
       DefaultModelsForProvider; builtin.go default_model fields; providers/*.toml.
    2. INPUT: P2.M1.T1.S1 (qwen-code registered). role_defaults.go, builtin.go, providers/*.toml,
       docs/providers.md.
    3. LOGIC: (a) Add the qwen-code row to DefaultModelsForProvider per FR-D4. (b) Refresh model tokens to
       CURRENT per FR-D5 across builtin.go default_model + matching providers/*.toml: agy/gemini flagship →
       gemini-3.1-pro, message → gemini-3.1-flash-lite, stager/arbiter → gemini-3.5-flash; claude (opus 4.8 /
       sonnet 5 / haiku); codex (gpt-5.1-codex-max etc.); pi stays blank (FR-D2). Record verified names +
       verification date in the manifest source comments. Keep # TO CONFIRM discipline where a live lookup is
       still needed (cursor/codex/qwen-code exact tokens). (c) FR-D3 rationale note: the message tier is the
       cheapest / free-tier-eligible model.
    4. OUTPUT: per-provider default models are current as of implementation; qwen-code has a complete tier row.
    5. DOCS: [Mode A] docs/providers.md reference table (add the qwen-code row, note experimental + DashScope;
       refresh the refreshed tokens). Appendix D lives in PRD.md (read-only).

  ⚠️ §1 — NECESSARY DEVIATION FROM THE LITERAL CONTRACT TEXT (qwen-code `stager`). The contract says
  "stager qwen3-coder-flash", but S1's manifest sets qwen-code `TooledFlags=nil` (not stager-capable), and
  `stagerFallback` (bootstrap.go:75) treats a NON-EMPTY stager cell as "this provider IS the stager" — so a
  non-empty cell would route the stager to qwen-code and `RenderTooled` would ERROR (nil tooled_flags). Every
  non-stager-capable provider (gemini/agy/opencode/codex/cursor) already uses `stager=""`. Therefore qwen-code
  `stager MUST be ""` (the PRD's own note — "a platform whose tooled_flags is empty cannot serve as the
  stager" — encodes this; the bootstrap applies the FR-D4 fallback on stager=""). Following the literal
  contract text here would BREAK the stager. The qwen-code row is: planner=qwen3-coder-plus, stager="",
  message=qwen3-coder-flash, arbiter=qwen3-coder-plus (all # TO CONFIRM).

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - providers/qwen-code.toml + builtinQwenCode() + the qwen-code tests + registry.go preferredBuiltins →
      S1 (P2.M1.T1.S1, in progress). S2 CONSUMES S1's outputs (qwen-code registered, default_model already
      "qwen3-coder-plus"). Do NOT change qwen-code's manifest value (it IS the flagship — correct).
    - The Manifest struct, Render, MergeManifest, the Git interface → COMPLETE (read-only).
    - internal/git/*, internal/decompose/*, internal/generate/*, internal/prompt/* → untouched.
    - docs/providers.md's broader v3-schema inconsistencies (default_provider field, "18 fields",
      agent→provider terminology) → P4.M2.T1.S1. S2 touches ONLY the qwen-code rows + token refresh.

  DELIVERABLES (0 NEW files, 8 EDITED files):
    EDIT internal/config/role_defaults.go         — add qwen-code row (stager=""); gemini/agy planner
                                                     gemini-3.5-pro→gemini-3.1-pro; update verification block.
    EDIT internal/config/role_defaults_test.go    — 4 tests: PerProvider (gemini/agy refresh + qwen-code row),
                                                     AllRolesPresent (+qwen-code), StagerCapability (+qwen-code),
                                                     KeySanity (+qwen-code, 8).
    EDIT internal/config/bootstrap.go             — local preferredBuiltins += "qwen-code" (consistency fix; §4).
    EDIT internal/provider/builtin.go             — builtinGemini/builtinAgy DefaultModel gemini-2.5-pro→
                                                     gemini-3.1-pro + doc comments + verified-on note.
    EDIT internal/provider/builtin_test.go        — geminiTOML/agyTOML default_model; GeminiFields/AgyFields +
                                                     RenderedCommand_Gemini/Agy assertions (gemini-2.5-pro→gemini-3.1-pro).
    EDIT providers/gemini.toml                    — default_model + rendered-command comment (gemini-2.5-pro→gemini-3.1-pro).
    EDIT providers/agy.toml                       — default_model + rendered-command comment (gemini-2.5-pro→gemini-3.1-pro).
    EDIT docs/providers.md                        — add qwen-code rows (built-in table + FR-D4 table); refresh
                                                     gemini tokens; count 7→8; auto-detect order + qwen-code.

  SUCCESS: qwen-code has a complete FR-D4 tier row in role_defaults.go (stager=""); gemini/agy flagship is
  gemini-3.1-pro everywhere (role_defaults.go + builtin.go + toml + docs); all 4 role_defaults tests + the
  refreshed builtin tests pass; bootstrap.go's local preferredBuiltins matches registry.go (8 elements);
  docs/providers.md shows qwen-code (experimental + DashScope + # TO CONFIRM); `go build/vet/test ./...`
  green; go.mod/go.sum unchanged; the 8 files above are the ONLY changes.

---

## Goal

**Feature Goal**: Complete qwen-code's per-role model story and bring the shipped model tokens current per
FR-D5 (PRD §9.16). Concretely: (a) give qwen-code a complete 4-role tier row in `DefaultModelsForProvider`
(planner=qwen3-coder-plus, stager="" [not stager-capable], message=qwen3-coder-flash, arbiter=qwen3-coder-plus,
all `# TO CONFIRM`); (b) refresh the gemini/agy flagship token `gemini-3.5-pro`→`gemini-3.1-pro` in the tier
table and `gemini-2.5-pro`→`gemini-3.1-pro` in the manifest `default_model` + the reference tomls + docs;
(c) record the FR-D3 rationale (message tier = cheapest/free-tier-eligible) + verification date in source
comments; (d) surface qwen-code in docs/providers.md. After S2, every built-in provider has a current,
documented, tier-appropriate default-model column, and qwen-code is a fully-fledged (if still experimental
+ # TO CONFIRM) member of the FR-D4 table.

**Deliverable** (0 NEW + 8 EDITED): see the DELIVERABLES list above. No new files (S1 creates
providers/qwen-code.toml; S2 only edits existing files). No new types, no interface change, no import change,
no dependency change.

**Success Definition**:
- `DefaultModelsForProvider("qwen-code")` returns
  `{planner:"qwen3-coder-plus", stager:"", message:"qwen3-coder-flash", arbiter:"qwen3-coder-plus"}`.
- `DefaultModelsForProvider("gemini")["planner"]` == `"gemini-3.1-pro"` (was `gemini-3.5-pro`); same for agy.
- `builtinGemini().DefaultModel` == `*strPtr("gemini-3.1-pro")` (was `gemini-2.5-pro`); same for agy.
- providers/gemini.toml + providers/agy.toml `default_model` == `gemini-3.1-pro`.
- docs/providers.md shows qwen-code in BOTH the built-in table + the FR-D4 table (experimental + DashScope +
  # TO CONFIRM), gemini/agy tokens refreshed, count "8", auto-detect order incl. qwen-code.
- bootstrap.go's local `preferredBuiltins` == `[pi, opencode, cursor, agy, gemini, qwen-code, codex, claude]`.
- `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/` empty; go.mod/go.sum unchanged;
  EXACTLY the 8 listed files change.

## User Persona

**Target User**: a developer running `stagecoach` with qwen-code (or gemini/agy) as their agent. After S2,
`config init` materializes qwen-code's per-role models (planner/message/arbiter uncommented if qwen-code is
the default; commented otherwise), and gemini/agy users get the current flagship (`gemini-3.1-pro`) rather
than the stale `gemini-2.5-pro`.

**Use Case**: a user with qwen-code on `$PATH` runs `stagecoach config init`; the bootstrap writes
qwen-code's `[role.planner] model = "qwen3-coder-plus"` etc., and (because qwen-code can't stager) routes
`[role.stager]` to the first stager-capable provider (pi/claude) with an annotation — exactly like gemini/agy.

**Pain Points Addressed**: today qwen-code has NO tier row (S1 registered the provider + a flagship
default_model, but `DefaultModelsForProvider("qwen-code")` returns nil ⇒ the bootstrap can't write per-role
models for it), and gemini/agy ship a stale flagship token. S2 closes both gaps.

## Why

- **Closes PRD §9.16 FR-D4 (qwen-code column) + FR-D5 (token currency) at the data layer.** S1 registered the
  provider; S2 gives it the per-role model story + refreshes the gemini-line tokens that drifted since the
  last pass.
- **Necessary for the bootstrap to work for qwen-code.** `buildBootstrapConfig` calls
  `DefaultModelsForProvider(target)`; without a qwen-code row it gets nil and can't write the `[role.*]`
  blocks. Plus bootstrap.go's local `preferredBuiltins` must list qwen-code for it to appear as a commented
  switchable agent (§4).
- **Honest progressive verification (FR-D5).** qwen-code + cursor + codex tokens are marked `# TO CONFIRM`
  (no live CLI lookup this pass); gemini/agy/claude/pi/opencode are pinned to PRD-baseline current values with
  a verification date. The table is authored trivially-refreshable (one cell per provider×role) per FR-D5.
- **Low-risk, mechanical refresh + one new table column.** No new types, no logic change — data values +
  their pinning tests + docs. The one design decision (qwen-code stager="") is the SAME convention already
  applied to 5 other providers.

## What

Data-value edits across 8 files: one new tier-table column (qwen-code) + the gemini/agy flagship refresh
(`gemini-3.5-pro`→`gemini-3.1-pro` in the table; `gemini-2.5-pro`→`gemini-3.1-pro` in the manifest/toml/docs)
+ their pinning tests + a bootstrap consistency fix + docs rows. No structural changes.

### Success Criteria

- [ ] `roleDefaults` has an 8th entry `"qwen-code"` with planner=`qwen3-coder-plus`, stager=`""`,
      message=`qwen3-coder-flash`, arbiter=`qwen3-coder-plus` (all `# TO CONFIRM` per FR-D5).
- [ ] `roleDefaults["gemini"]` + `["agy"]` planner == `gemini-3.1-pro` (was `gemini-3.5-pro`); their
      message/arbiter unchanged (`gemini-3.1-flash-lite` / `gemini-3.5-flash`).
- [ ] `builtinGemini()` + `builtinAgy()` `DefaultModel` == `gemini-3.1-pro` (was `gemini-2.5-pro`) + doc
      comments updated + a verified-on note.
- [ ] `geminiTOML` + `agyTOML` test constants' `default_model` == `gemini-3.1-pro`; the Gemini/Agy Fields +
      RenderedCommand tests assert `gemini-3.1-pro`.
- [ ] providers/gemini.toml + providers/agy.toml `default_model` + rendered-command comment == `gemini-3.1-pro`.
- [ ] bootstrap.go local `preferredBuiltins` == `[pi, opencode, cursor, agy, gemini, qwen-code, codex, claude]`.
- [ ] docs/providers.md: qwen-code in BOTH tables (experimental + DashScope + # TO CONFIRM ⚠️), gemini/agy
      tokens refreshed, "8 built-in providers", auto-detect order incl. qwen-code.
- [ ] role_defaults.go verification block: records the gemini-3.1-pro refresh, the qwen-code row, the FR-D3
      message-tier-cheapest rationale, + verification date 2026-07-02.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/` empty; go.mod/go.sum
      unchanged; EXACTLY the 8 listed files change.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the qwen-code stager=""
deviation rationale (§1 — necessary correctness fix vs the literal contract text), the exact gemini/agy
refresh deltas (§2/§3 — 3.5-pro→3.1-pro in the table, 2.5-pro→3.1-pro in the manifest/toml), the 4
role_defaults_test.go edits (§5), the bootstrap.go consistency fix (§4), the docs narrow scope (§6), the S1
file-overlap coordination (§7 — non-overlapping regions), and the FR-D4 target table (§9). No Render/Merge/
git/decompose logic knowledge required — this is a data-value + pinning-test + docs task.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE findings (the stager deviation + exact deltas + test map + coordination)
- docfile: plan/003_6ce49c39466e/P2M1T1S2/research/findings.md
  why: §1 (qwen-code stager MUST be "" — RenderTooled would break on a non-empty cell + nil TooledFlags);
       §2 (role_defaults.go gemini/agy planner gemini-3.5-pro→gemini-3.1-pro); §3 (builtin.go + toml +
       tests default_model gemini-2.5-pro→gemini-3.1-pro, with EXACT line numbers); §4 (bootstrap.go stale
       local preferredBuiltins needs qwen-code); §5 (the 4 role_defaults_test.go edits); §6 (docs narrow
       scope); §7 (S1/S2 non-overlapping edits in builtin.go/builtin_test.go); §8 (verification date); §9
       (the FR-D4 target table).
  critical: §1 (without it the agent writes stager="qwen3-coder-flash" and breaks the stager at runtime),
       §3 (exact line numbers for the builtin_test.go ripple), §7 (do NOT touch qwen-code's manifest value or
       S1's qwen-code tests).

# MUST READ — S1's PRP (the CONTRACT for qwen-code's manifest; what S2 consumes)
- docfile: plan/003_6ce49c39466e/P2M1T1S1/PRP.md
  section: the builtinQwenCode() blueprint (DefaultModel="qwen3-coder-plus" = the flagship; TooledFlags=nil
       ⇒ not stager-capable) + S1's SCOPE BOUNDARY ("S2 owns role_defaults.go + docs/providers.md + the
       FR-D5 token refresh").
  critical: S1 ships qwen-code DefaultModel="qwen3-coder-plus" (already the flagship — S2 does NOT change
       it) and TooledFlags=nil (⇒ S2's stager="" in the tier row). S1 edits builtin.go/builtin_test.go too
       — S2's edits there are in DIFFERENT functions (builtinGemini/builtinAgy) ⇒ non-overlapping (§7).

# MUST READ — the FILE TO EDIT: the FR-D4 tier table (add the qwen-code column + refresh gemini/agy)
- file: internal/config/role_defaults.go   (EDIT)
  section: the FR-D5 verification block (L1-31 — add qwen-code + the gemini-3.1-pro refresh + FR-D3 note +
       date); `var roleDefaults = RoleModelDefaults{...}` (L33-87 — add the "qwen-code" column; change
       gemini/agy planner gemini-3.5-pro→gemini-3.1-pro).
  why: this IS the FR-D4 table. qwen-code's stager cell MUST be "" (TooledFlags nil — see the table's own
       doc comment L20-22 + §1).
  pattern: mirror the gemini/agy column shape (4 roles, stager=""); add a `# TO CONFIRM per FR-D5` comment
       on each qwen-code value.
  gotcha: do NOT set qwen-code stager="qwen3-coder-flash" (§1 — breaks RenderTooled). Use "" like the 5 other
       non-stager-capable providers.

# MUST READ — the 4 tests that PIN the table (must be updated in lockstep)
- file: internal/config/role_defaults_test.go   (EDIT)
  section: TestDefaultModelsForProvider_PerProvider (hardcoded want table — refresh gemini/agy planner +
       ADD qwen-code); _AllRolesPresent (provider loop — ADD "qwen-code"); _StagerCapability (incapable
       slice — ADD "qwen-code"); TestRoleDefaults_KeySanity (expectedProviders set — ADD "qwen-code" ⇒ 8).
  why: these tests hardcode the table values/provider set; they fail until updated. _StagerCapability PINS
       the stager="" convention (§1) — adding qwen-code to its incapable list is the test-side guard.
  gotcha: do NOT add qwen-code to the `capable` slice {pi, claude} — it is NOT stager-capable.

# MUST READ — the bootstrap fallback logic (WHY stager must be "" + the stale local preferredBuiltins)
- file: internal/config/bootstrap.go   (EDIT — preferredBuiltins only)
  section: `stagerFallback` (L75-86: `if m := models["stager"]; m != "" { return target, m }` — a non-empty
       cell means "this provider IS the stager"; empty ⇒ iterate preferredBuiltins for the first
       stager-capable); the local `var preferredBuiltins` (L15 — the 7-element copy; ADD "qwen-code").
  why: stagerFallback is WHY a non-empty qwen-code stager cell would break (it'd route to qwen-code then
       RenderTooled errors). The local preferredBuiltins is STALE after S1 (registry.go has 8, this has 7) —
       qwen-code won't get a commented [role.*] block in config init until fixed (§4).
  gotcha: bootstrap_test.go does NOT assert on the preferredBuiltins slice directly (verified — it installs
       subsets like pi-only/pi+claude), so adding qwen-code is safe. Keep the local copy's doc comment honest.

# MUST READ — the manifest default_model to refresh (gemini/agy)
- file: internal/provider/builtin.go   (EDIT — builtinGemini/builtinAgy only)
  section: `builtinGemini()` (DefaultModel strPtr("gemini-2.5-pro") + doc comment → gemini-3.1-pro);
       `builtinAgy()` (same). Add a verified-on note.
  why: the manifest default_model is the single-value fallback; the flagship tier is now gemini-3.1-pro.
  gotcha: do NOT touch builtinQwenCode() (S1's — its DefaultModel "qwen3-coder-plus" is already the flagship).
       do NOT touch builtinPi/Claude/Codex/Cursor/OpenCode (their default_models are correct/empty).

# MUST READ — the test ripple for the builtin.go refresh
- file: internal/provider/builtin_test.go   (EDIT — gemini/agy assertions only)
  section: geminiTOML const (L77 default_model) + agyTOML const (L135 default_model) → gemini-3.1-pro;
       TestBuiltinManifests_GeminiFields (L444) + _AgyFields (L669) assertStr DefaultModel → gemini-3.1-pro;
       TestBuiltinManifests_RenderedCommand_Gemini (L510,512) + _Agy (L707,709) → gemini-3.1-pro.
  why: these assertions + DecodeParity constants pin the default_model; they fail until refreshed.
  gotcha: do NOT touch the qwen-code tests/consts (S1's) or the count test (S1 updated 7→8). Your edits are
       in the gemini/agy regions ONLY (§7).

# MUST READ — the reference tomls to refresh (mirror the manifest byte-for-byte)
- file: providers/gemini.toml   (EDIT)
  section: the field line `default_model = "gemini-2.5-pro"` (L51) + the RENDERED COMMAND comment
       (`gemini -m gemini-2.5-pro ...`, L21) → gemini-3.1-pro.
  why: these files mirror the compiled-in manifest for readers; keep them byte-faithful after the refresh.
  gotcha: NOT runtime-loaded — human reference doc. Update BOTH the field + the rendered-command comment.
- file: providers/agy.toml   (EDIT) — same two edits (L51 field + L21 rendered-command comment) → gemini-3.1-pro.

# MUST READ — the docs to update (NARROW scope: qwen-code rows + token refresh only)
- file: docs/providers.md   (EDIT)
  section: "the 7 built-in providers"/"Seven providers are compiled in" → 8; auto-detection order line
       (insert qwen-code between gemini and codex); the built-in providers TABLE (ADD qwen-code row; refresh
       gemini/agy Default-model cell gemini-2.5-pro→gemini-3.1-pro); the Per-role (FR-D4) TABLE (ADD
       qwen-code row; refresh gemini/agy planner gemini-3.5-pro→gemini-3.1-pro); a ⚠️ footnote for qwen-code.
  why: S2's docs deliverable (item §5) — add the qwen-code row + refresh the refreshed tokens.
  gotcha: do NOT fix the broader v3-schema inconsistencies (default_provider field, "18 fields", agent→provider
       terminology) — that is P4.M2.T1.S1's scope. Touch ONLY the qwen-code rows + the gemini token refresh +
       the count/order directly tied to them.

# The PRD basis (in your context as selected_prd_content)
- file: PRD.md
  section: §9.16 FR-D4 (h3.32 — the per-provider × per-role table incl. qwen-code + the stager-fallback
       note "a platform whose tooled_flags is empty ... cannot serve as the stager"); FR-D5 (the re-verify
       mandate + record names+date); FR-D3 (message = cheapest/free-tier-eligible); §12.5.2 (h3.49 — qwen-code
       experimental + DashScope + # TO CONFIRM).
  critical: §9.16's own note REQUIRES stager fallback for empty-tooled_flags providers ⇒ qwen-code stager="".
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  role_defaults.go       # EDIT: add qwen-code column (stager=""); gemini/agy planner 3.5-pro→3.1-pro; verification block.
  role_defaults_test.go  # EDIT: 4 tests (PerProvider, AllRolesPresent, StagerCapability, KeySanity) + qwen-code.
  bootstrap.go           # EDIT: local preferredBuiltins += "qwen-code" (consistency; §4).
internal/provider/
  builtin.go             # EDIT: builtinGemini/builtinAgy DefaultModel 2.5-pro→3.1-pro + doc comments. (S1 also edits — non-overlap)
  builtin_test.go        # EDIT: geminiTOML/agyTOML + Gemini/Agy Fields/RenderedCommand assertions. (S1 also edits — non-overlap)
providers/
  gemini.toml            # EDIT: default_model + rendered-command comment → gemini-3.1-pro.
  agy.toml               # EDIT: default_model + rendered-command comment → gemini-3.1-pro.
  qwen-code.toml         # S1 CREATES (do NOT touch — qwen3-coder-plus is already the flagship).
docs/
  providers.md           # EDIT: qwen-code rows (both tables) + token refresh + count/order. (narrow scope)
go.mod / go.sum          # UNCHANGED (no new import — data + docs only).
```

### Desired Codebase tree with files to be MODIFIED

```bash
internal/config/role_defaults.go        # EDIT — + "qwen-code" column (stager=""); gemini/agy planner → gemini-3.1-pro;
                                        #   verification block: qwen-code line + gemini-3.1-pro refresh + FR-D3 note + date.
internal/config/role_defaults_test.go   # EDIT — PerProvider (gemini/agy refresh + qwen-code row); AllRolesPresent (+qwen-code);
                                        #   StagerCapability (+qwen-code incapable); KeySanity (+qwen-code, 8).
internal/config/bootstrap.go            # EDIT — local preferredBuiltins: [.., gemini, qwen-code, codex, ..] (8 elements).
internal/provider/builtin.go            # EDIT — builtinGemini/builtinAgy DefaultModel gemini-3.1-pro + doc comments + verified-on.
internal/provider/builtin_test.go       # EDIT — geminiTOML/agyTOML default_model; Gemini/AgyFields + RenderedCommand assertions.
providers/gemini.toml                   # EDIT — default_model + rendered-command comment → gemini-3.1-pro.
providers/agy.toml                      # EDIT — default_model + rendered-command comment → gemini-3.1-pro.
docs/providers.md                       # EDIT — qwen-code rows (built-in + FR-D4 tables); gemini token refresh; count 8; order.
# go.mod/go.sum UNCHANGED. providers/qwen-code.toml + builtinQwenCode + qwen-code tests = S1 (UNTOUCHED).
# Manifest struct/Render/Merge/the Git interface UNCHANGED. internal/{git,decompose,generate,prompt}/* UNTOUCHED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (§1 — qwen-code stager MUST be ""): the contract says "stager qwen3-coder-flash" but that BREAKS
// the stager. S1's builtinQwenCode() sets TooledFlags=nil (not stager-capable). stagerFallback
// (bootstrap.go:75) treats a NON-EMPTY stager cell as "this provider IS the stager" → it would route the
// stager to qwen-code → RenderTooled ERRORS on nil tooled_flags. Every non-stager-capable provider already
// uses stager="". PRD §9.16's own note mandates the fallback for empty-tooled_flags providers. ⇒ qwen-code
// stager = "" (the bootstrap then falls back to pi/claude). Add qwen-code to StagerCapability's incapable list.

// CRITICAL (§3 — default_model refresh ripple is 6 edits, not 1): changing gemini/agy DefaultModel in
// builtin.go ripples to: geminiTOML + agyTOML test consts, GeminiFields + AgyFields assertions,
// RenderedCommand_Gemini + _Agy assertions (builtin_test.go), + providers/gemini.toml + providers/agy.toml
// (field + rendered-command comment). Miss any and the DecodeParity/Fields/RenderedCommand tests fail.

// CRITICAL (§2 — the 3.5-pro vs 3.1-pro discrepancy is REAL): the current role_defaults.go has
// gemini/agy planner = gemini-3.5-pro (P1.M3.T3.S1 wrote 3.5-pro). The PRD FR-D4 table + S2 contract say
// gemini-3.1-pro. Refresh 3.5-pro → 3.1-pro (table + the PerProvider test). message (gemini-3.1-flash-lite)
// and arbiter (gemini-3.5-flash) are ALREADY correct — do NOT touch them.

// GOTCHA (§4 — bootstrap.go has its OWN preferredBuiltins copy): it is NOT registry.go's. It's a local
// duplicate (L15, documented "mirrors registry.go's preferredBuiltins"). S1 updates registry.go's to 8;
// this local copy is STALE (7) until S2 adds qwen-code. Without it qwen-code gets no commented [role.*]
// block in config init. bootstrap_test.go does NOT assert on this slice (safe to edit).

// GOTCHA (§7 — S1/S2 both edit builtin.go + builtin_test.go): the edits are NON-OVERLAPPING. S1 ADDS
// builtinQwenCode + map entry + count + qwen-code tests (NEW code); S2 MODIFIES builtinGemini/builtinAgy
// values + their tests (EXISTING values). Do NOT touch qwen-code's manifest value (qwen3-coder-plus = the
// flagship, already correct) or any qwen-code test. Do NOT touch builtinPi/Claude/Codex/Cursor/OpenCode.

// GOTCHA (docs narrow scope): docs/providers.md has KNOWN v3-schema inconsistencies (default_provider field,
// "18 fields", agent→provider terminology) that are P4.M2.T1.S1's. S2 touches ONLY the qwen-code rows +
# the gemini token refresh + the count/order directly tied to adding qwen-code. Do NOT "fix" the schema table.

// GOTCHA (verification date): the role_defaults.go block is dated "2026-07". Update to 2026-07-02 + record
// the gemini-3.1-pro refresh + the qwen-code row + the FR-D3 message-tier-cheapest rationale. qwen-code/
// cursor/codex tokens stay # TO CONFIRM (no live CLI lookup this pass).

// GOTCHA (no new imports / no interface change): this is data + pinning-tests + docs. go.mod/go.sum
// byte-unchanged. The Manifest struct, Render, MergeManifest, the Git interface are UNCHANGED.
```

## Implementation Blueprint

### Data models and structure

No new TYPES. The only structural addition is one new map column in the existing `roleDefaults`
(`RoleModelDefaults = map[string]map[string]string`). The refresh is value-only edits to existing fields.

```go
// === internal/config/role_defaults.go — ADD the qwen-code column + refresh gemini/agy planner ===
// In `var roleDefaults = RoleModelDefaults{ ... }`:

	"qwen-code": {
		"planner": "qwen3-coder-plus",  // flagship/smart (FR-D3). # TO CONFIRM per FR-D5
		"stager":  "",                   // NOT stager-capable (TooledFlags nil in builtinQwenCode) — bootstrap applies FR-D4 fallback
		"message": "qwen3-coder-flash",  // fast/cheapest tier (FR-D3). # TO CONFIRM per FR-D5
		"arbiter": "qwen3-coder-plus",   // mid tier. # TO CONFIRM per FR-D5
	},

// And in the EXISTING "gemini" + "agy" columns, change the planner ONLY:
	"gemini": {
		"planner": "gemini-3.1-pro",    // WAS gemini-3.5-pro — refreshed per FR-D5 (PRD §9.16 FR-D4 table)
		"stager":  "",                  // unchanged
		"message": "gemini-3.1-flash-lite", // unchanged
		"arbiter": "gemini-3.5-flash",  // unchanged
	},
	"agy": {
		"planner": "gemini-3.1-pro",    // WAS gemini-3.5-pro — refreshed per FR-D5
		// ... stager/message/arbiter unchanged
	},
```

```go
// === internal/config/role_defaults.go — UPDATE the FR-D5 verification block (L1-31) ===
// Bump "Verification date: 2026-07" → "2026-07-02". In the per-provider status list, ADD:
//   qwen-code — qwen3-coder-plus / "" (cannot stager) / qwen3-coder-flash / qwen3-coder-plus — # TO CONFIRM
//               per FR-D5 (Alibaba Qwen3-Coder via DashScope; no live CLI lookup this pass).
// And UPDATE the agy/gemini lines: "...gemini-3.1-pro / gemini-3.5-flash / gemini-3.1-flash-lite — refreshed
//   2026-07-02 per FR-D5 (was 3.5-pro planner)."
// ADD an FR-D3 note (if not already present): "message tier = the cheapest / free-tier-eligible model
//   (highest-volume role; many users on free tiers)."
```

```go
// === internal/config/bootstrap.go — local preferredBuiltins (L15) ===
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "qwen-code", "codex", "claude"}
//   ↑ insert "qwen-code" between "gemini" and "codex" to mirror registry.go (S1) — keeps config init's
//     commented-block emission + stagerFallback iteration covering qwen-code.
```

```go
// === internal/provider/builtin.go — builtinGemini + builtinAgy DefaultModel refresh ===
// In builtinGemini(): DefaultModel: strPtr("gemini-3.1-pro"), // WAS gemini-2.5-pro — refreshed per FR-D5 (verified 2026-07-02)
//   + update the doc comment's "gemini-2.5-pro" mention → "gemini-3.1-pro".
// In builtinAgy():   DefaultModel: strPtr("gemini-3.1-pro"), // WAS gemini-2.5-pro — refreshed per FR-D5 (verified 2026-07-02)
//   + update the doc comment's "(4) default_model is \"gemini-2.5-pro\"" → "gemini-3.1-pro".
```

```go
// === internal/provider/builtin_test.go — gemini/agy default_model ripple (6 edits) ===
// geminiTOML const (L77):  default_model = "gemini-3.1-pro"   // WAS gemini-2.5-pro
// agyTOML const (L135):    default_model = "gemini-3.1-pro"   // WAS gemini-2.5-pro
// TestBuiltinManifests_GeminiFields (L444): assertStr(t, "DefaultModel", m.DefaultModel, "gemini-3.1-pro")
// TestBuiltinManifests_AgyFields   (L669): assertStr(t, "DefaultModel", m.DefaultModel, "gemini-3.1-pro")
// TestBuiltinManifests_RenderedCommand_Gemini (L510,512): comment + want slice "gemini-3.1-pro"
// TestBuiltinManifests_RenderedCommand_Agy    (L707,709): comment + want slice "gemini-3.1-pro"
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: internal/config/role_defaults.go — qwen-code column + gemini/agy planner refresh + verification block
  - ADD the "qwen-code" column to roleDefaults (Blueprint above; stager="" — §1).
  - CHANGE gemini + agy planner gemini-3.5-pro → gemini-3.1-pro (message/arbiter UNCHANGED).
  - UPDATE the FR-D5 verification block: date 2026-07-02; add qwen-code line + # TO CONFIRM; update
    agy/gemini lines to gemini-3.1-pro (refreshed); add the FR-D3 message-tier-cheapest note.
  - GOTCHA: qwen-code stager MUST be "" (§1). Do NOT write "qwen3-coder-flash" there.
  - GOTCHA: do NOT change gemini/agy message (gemini-3.1-flash-lite) or arbiter (gemini-3.5-flash).
  - PLACEMENT: internal/config/role_defaults.go (roleDefaults var + the top verification block).

Task 2: internal/config/role_defaults_test.go — 4 tests in lockstep
  - TestDefaultModelsForProvider_PerProvider: gemini+agy want planner "gemini-3.5-pro"→"gemini-3.1-pro";
    ADD want["qwen-code"] = {planner:"qwen3-coder-plus", stager:"", message:"qwen3-coder-flash",
    arbiter:"qwen3-coder-plus"}.
  - TestDefaultModelsForProvider_AllRolesPresent: provider loop += "qwen-code".
  - TestDefaultModelsForProvider_StagerCapability: incapable slice += "qwen-code" (NOT capable).
  - TestRoleDefaults_KeySanity: expectedProviders += "qwen-code" (now 8).
  - GOTCHA: do NOT add qwen-code to the capable slice {pi, claude} in _StagerCapability.
  - PLACEMENT: internal/config/role_defaults_test.go.

Task 3: internal/config/bootstrap.go — local preferredBuiltins += qwen-code
  - EDIT the local `var preferredBuiltins` (L15): insert "qwen-code" between "gemini" and "codex" (8 elements).
  - GOTCHA: this is the LOCAL copy (mirrors registry.go); bootstrap_test.go does not assert on it (safe).
    Keeps config init's commented-block emission + stagerFallback covering qwen-code.
  - PLACEMENT: internal/config/bootstrap.go.

Task 4: internal/provider/builtin.go — gemini/agy DefaultModel refresh
  - builtinGemini(): DefaultModel strPtr("gemini-2.5-pro")→strPtr("gemini-3.1-pro") + doc comment + verified-on note.
  - builtinAgy():   DefaultModel strPtr("gemini-2.5-pro")→strPtr("gemini-3.1-pro") + doc comment + verified-on note.
  - GOTCHA: do NOT touch builtinQwenCode (S1's; qwen3-coder-plus is correct) or other built-ins.
  - PLACEMENT: internal/provider/builtin.go (builtinGemini + builtinAgy ONLY).

Task 5: internal/provider/builtin_test.go — gemini/agy default_model ripple (6 edits)
  - geminiTOML (L77) + agyTOML (L135): default_model "gemini-2.5-pro"→"gemini-3.1-pro".
  - GeminiFields (L444) + AgyFields (L669): assertStr DefaultModel "gemini-3.1-pro".
  - RenderedCommand_Gemini (L510,512) + RenderedCommand_Agy (L707,709): comment + want "gemini-3.1-pro".
  - GOTCHA: do NOT touch qwen-code tests/consts (S1's) or the count test (S1 did 7→8). gemini/agy regions ONLY.
  - PLACEMENT: internal/provider/builtin_test.go.

Task 6: providers/gemini.toml + providers/agy.toml — default_model + rendered-command refresh
  - gemini.toml: field `default_model = "gemini-2.5-pro"` (L51) → "gemini-3.1-pro"; rendered-command comment
    (L21) "gemini -m gemini-2.5-pro ..." → "gemini-3.1-pro".
  - agy.toml: field `default_model = "gemini-2.5-pro"` (L51) → "gemini-3.1-pro"; rendered-command comment
    (L21) "agy -m gemini-2.5-pro ..." → "gemini-3.1-pro".
  - GOTCHA: these mirror the manifest byte-for-byte — update BOTH the field + the comment. NOT runtime-loaded.
  - PLACEMENT: providers/gemini.toml + providers/agy.toml.

Task 7: docs/providers.md — qwen-code rows + token refresh (NARROW scope)
  - "7 built-in providers"/"Seven providers are compiled in" → "8".
  - Auto-detection order: insert "qwen-code" between "gemini" and "codex".
  - Built-in providers TABLE: ADD qwen-code row (stdin / -p / -m / qwen3-coder-plus ⚠️ / prepended /
    --approval-mode default / no); refresh gemini+agy Default-model cell gemini-2.5-pro→gemini-3.1-pro.
  - Per-role (FR-D4) TABLE: ADD qwen-code row (qwen3-coder-plus ⚠️ / *(cannot)* / qwen3-coder-flash ⚠️ /
    qwen3-coder-plus ⚠️); refresh gemini+agy planner gemini-3.5-pro→gemini-3.1-pro.
  - Add/note: qwen-code is experimental (Gemini-CLI fork for Qwen3-Coder via DashScope); ⚠️ = # TO CONFIRM per FR-D5.
  - GOTCHA: do NOT fix the broader v3-schema inconsistencies (default_provider field, "18 fields",
    agent→provider terminology) — P4.M2.T1.S1's scope. Touch ONLY the qwen-code rows + gemini token refresh + count/order.
  - PLACEMENT: docs/providers.md.

Task 8: VERIFY (run all gates; fix before declaring done)
  - gofmt -w internal/config/{role_defaults.go,role_defaults_test.go,bootstrap.go} \
            internal/provider/{builtin.go,builtin_test.go}
  - go build ./... && go vet ./...
  - go test -race ./internal/config/ -run "TestDefaultModelsForProvider|TestRoleDefaults|TestBootstrapConfig" -v
  - go test -race ./internal/provider/ -run "TestBuiltinManifests_GeminiFields|TestBuiltinManifests_AgyFields|TestBuiltinManifests_RenderedCommand_Gemini|TestBuiltinManifests_RenderedCommand_Agy|TestBuiltinManifests_DecodeParity" -v
  - go test ./...   (full regression — qwen-code tests from S1 must also pass once S1 lands)
  - git diff --exit-code go.mod go.sum → empty.
  - git status → EXACTLY the 8 edited files. providers/qwen-code.toml (S1's) NOT touched by S2.
```

### Implementation Patterns & Key Details

```go
// PATTERN (the qwen-code column mirrors gemini/agy's shape — 4 roles, stager=""): qwen-code is the
// gemini-lineage experimental twin; its tier row follows the SAME convention (stager="" because TooledFlags
// nil). Only the model TOKENS differ (Qwen3-Coder family, # TO CONFIRM).

// PATTERN (refresh ripple = update the value + every assertion that pins it): changing a default_model is
// NOT a 1-line edit — it ripples to the DecodeParity toml const + the Fields assertion + the RenderedCommand
// assertion (+ the reference toml field + comment). Update ALL of them or the pinning tests fail.

// CRITICAL (§1 — stager="" is NOT optional): a non-empty stager cell is the bootstrap's "this provider IS the
// stager" signal (stagerFallback L75). qwen-code has nil TooledFlags ⇒ RenderTooled errors. So stager MUST be
// "". This matches gemini/agy/opencode/codex/cursor + TestDefaultModelsForProvider_StagerCapability.

// CRITICAL (§2 — 3.5-pro is STALE, 3.1-pro is current): the PRD FR-D4 table is authoritative. The current
// code's gemini-3.5-pro predates the refresh; the S2 contract + PRD both say gemini-3.1-pro. Refresh it.

// PATTERN (verification-date discipline FR-D5): record WHAT changed + WHEN in the role_defaults.go block.
// qwen-code/cursor/codex tokens stay # TO CONFIRM (no live CLI lookup); gemini/agy/claude/pi/opencode are
// pinned with a date. The table is one-cell-per-provider×role ⇒ trivially refreshable.
```

### Integration Points

```yaml
ROLE DEFAULTS (internal/config/role_defaults.go — EDIT):
  - add: "qwen-code column {planner:qwen3-coder-plus, stager:\"\", message:qwen3-coder-flash,
          arbiter:qwen3-coder-plus} (all # TO CONFIRM)."
  - refresh: "gemini/agy planner gemini-3.5-pro → gemini-3.1-pro."
  - effect: "DefaultModelsForProvider('qwen-code') returns its 4-role column (was nil); the bootstrap
          (config init) can now write qwen-code's [role.*] blocks; stagerFallback routes qwen-code's stager
          to pi/claude (stager='')."

BOOTSTRAP (internal/config/bootstrap.go — EDIT, preferredBuiltins only):
  - add: "'qwen-code' to the LOCAL preferredBuiltins (mirrors registry.go's 8-element copy after S1)."
  - effect: "qwen-code appears as a commented [role.*] switchable agent in config init when installed;
          stagerFallback's iteration covers it (harmless — qwen-code stager='' so it's skipped)."

MANIFEST DEFAULT_MODEL (internal/provider/builtin.go — EDIT, gemini/agy only):
  - refresh: "builtinGemini/builtinAgy DefaultModel gemini-2.5-pro → gemini-3.1-pro."
  - effect: "the single-value fallback model is current; `providers show gemini|agy` prints gemini-3.1-pro."

REFERENCE DOCS (providers/gemini.toml + agy.toml + docs/providers.md — EDIT):
  - refresh/add: "gemini-3.1-pro in tomls; qwen-code rows + token refresh in docs/providers.md."

GO MODULE (go.mod/go.sum): change NONE. No new import. Data + pinning-tests + docs only.

UPSTREAM (consume, do NOT edit): S1's builtinQwenCode (qwen3-coder-plus flagship, TooledFlags nil,
      Experimental=true) + providers/qwen-code.toml. The Manifest struct/Render/Merge (COMPLETE).

DOWNSTREAM (consumers — not this task):
  - P4.M2.T1.S1: the broader docs/providers.md v3-schema sync (default_provider field, "18 fields",
        agent→provider terminology). S2 leaves those inconsistencies for P4.M2.
  - FR-D5 periodic re-verification (out of scope per PRD): an automated check against each provider's live
        model list. The table is authored trivially-refreshable to support it.

FROZEN/LEAVE (do NOT edit):
  - providers/qwen-code.toml + builtinQwenCode() + qwen-code tests + registry.go preferredBuiltins → S1.
  - docs/providers.md's v3-schema inconsistencies → P4.M2.T1.S1.
  - manifest.go, merge.go, render.go, the Git interface, internal/{git,decompose,generate,prompt}/*, pkg/*,
    go.mod, Makefile, PRD.md.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach
gofmt -w internal/config/role_defaults.go internal/config/role_defaults_test.go internal/config/bootstrap.go \
          internal/provider/builtin.go internal/provider/builtin_test.go
go vet ./...
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; gofmt -l empty; go.mod/go.sum unchanged.
```

### Level 2: Unit Tests (Component Validation)

```bash
# role_defaults.go table (the 4 updated tests):
go test -race ./internal/config/ -run "TestDefaultModelsForProvider_PerProvider|TestDefaultModelsForProvider_AllRolesPresent|TestDefaultModelsForProvider_StagerCapability|TestDefaultModelsForProvider_UnknownReturnsNil|TestDefaultModelsForProvider_CopySemantics|TestRoleDefaults_KeySanity" -v
# Expected: all green. Specifically:
#   _PerProvider         → qwen-code column {plus, "", flash, plus}; gemini/agy planner gemini-3.1-pro
#   _StagerCapability    → qwen-code in the incapable list (stager=="")
#   _KeySanity           → 8 providers incl. qwen-code

# bootstrap (the preferredBuiltins consistency fix — no behavior regression):
go test -race ./internal/config/ -run "TestBootstrapConfig|TestGenerateBootstrapConfig|TestStagerFallback" -v
# Expected: all green UNCHANGED (bootstrap_test.go doesn't assert on the preferredBuiltins slice; subsets
#   like pi-only/pi+claude are unaffected by adding qwen-code to the order).

# builtin.go gemini/agy default_model refresh:
go test -race ./internal/provider/ -run "TestBuiltinManifests_GeminiFields|TestBuiltinManifests_AgyFields|TestBuiltinManifests_RenderedCommand_Gemini|TestBuiltinManifests_RenderedCommand_Agy|TestBuiltinManifests_DecodeParity" -v
# Expected: all green with default_model = gemini-3.1-pro (DecodeParity: built-in == decoded refreshed toml const).

# Full regression (also runs S1's qwen-code tests once S1 has landed):
go test ./...
```

### Level 3: Integration / Behavioral Proof

```bash
make build

# qwen-code now has a tier row (was nil); the bootstrap can write its [role.*] blocks. If qwen-code is the
# target, stager routes to pi/claude (stager=""):
# (qwen-code is almost certainly NOT on $PATH in CI — exercise the data path directly:)
cat > /tmp/qwen_check.go <<'EOF'
package main
import ("fmt"; "github.com/dustin/stagecoach/internal/config")
func main() {
  q := config.DefaultModelsForProvider("qwen-code")
  fmt.Printf("qwen-code: %+v\n", q)
  fmt.Printf("gemini planner: %s\n", config.DefaultModelsForProvider("gemini")["planner"])
}
EOF
go run /tmp/qwen_check.go
# Expected: qwen-code: map[arbiter:qwen3-coder-plus message:qwen3-coder-flash planner:qwen3-coder-plus stager:]
#           gemini planner: gemini-3.1-pro
rm -f /tmp/qwen_check.go

# `providers show gemini` prints the refreshed default_model (gemini-3.1-pro):
./bin/stagecoach providers show gemini | grep -E 'gemini-3.1-pro|gemini-2.5' && echo "PASS: gemini default_model refreshed" || echo "FAIL"
```

### Level 4: Regression & Audit

```bash
cd /home/dustin/projects/stagecoach
go build ./...                 # whole module compiles
go test ./...                  # FULL regression
git status --short             # Expected: EXACTLY 8 modified files:
                               #   M internal/config/role_defaults.go
                               #   M internal/config/role_defaults_test.go
                               #   M internal/config/bootstrap.go
                               #   M internal/provider/builtin.go
                               #   M internal/provider/builtin_test.go
                               #   M providers/gemini.toml
                               #   M providers/agy.toml
                               #   M docs/providers.md
# Expected: build + full test green; only the 8 files; go.mod/go.sum unchanged; providers/qwen-code.toml
#   (S1's) NOT touched; internal/{git,decompose,generate,prompt}/* byte-unchanged.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l internal/` empty; `go vet ./...` clean.
- [ ] Level 2: `go test -race ./internal/config/` green (4 updated role_defaults tests + bootstrap); `go test
      -race ./internal/provider/` green (refreshed gemini/agy Fields/RenderedCommand/DecodeParity).
- [ ] Level 3: `DefaultModelsForProvider("qwen-code")` returns the 4-role column; `providers show gemini`
      prints gemini-3.1-pro.
- [ ] Level 4: `go build ./...` + `go test ./...` green; `git status` shows EXACTLY 8 files; go.mod/go.sum
      unchanged; providers/qwen-code.toml (S1's) untouched.

### Feature Validation

- [ ] `DefaultModelsForProvider("qwen-code")` == `{planner:qwen3-coder-plus, stager:"", message:qwen3-coder-flash,
      arbiter:qwen3-coder-plus}`.
- [ ] gemini/agy planner == `gemini-3.1-pro` (role_defaults.go) AND DefaultModel == `gemini-3.1-pro` (builtin.go).
- [ ] providers/gemini.toml + providers/agy.toml default_model == `gemini-3.1-pro` (+ rendered-command comment).
- [ ] bootstrap.go local preferredBuiltins == `[pi, opencode, cursor, agy, gemini, qwen-code, codex, claude]`.
- [ ] docs/providers.md shows qwen-code in BOTH tables (experimental + DashScope + # TO CONFIRM ⚠️), gemini
      tokens refreshed, "8 built-in providers", auto-detect order incl. qwen-code.
- [ ] role_defaults.go verification block records the refresh + qwen-code + FR-D3 note + date 2026-07-02.

### Code Quality Validation

- [ ] qwen-code stager is "" (the §1 correctness fix — NOT the literal contract's "qwen3-coder-flash");
      TestDefaultModelsForProvider_StagerCapability lists qwen-code as incapable.
- [ ] The gemini/agy refresh ripple is complete (no stale `gemini-2.5-pro`/`gemini-3.5-pro` left in code/tests/toml).
- [ ] No qwen-code manifest value or qwen-code test touched (S1's domain — §7 non-overlap respected).
- [ ] docs scope is narrow (qwen-code rows + token refresh only; no v3-schema fixes — P4.M2's).
- [ ] Anti-patterns avoided (see below): no stager=qwen3-coder-flash, no S1-file value collision, no docs
      scope creep, no interface/import change.

### Documentation & Deployment

- [ ] role_defaults.go verification block + builtin.go doc comments record the refresh + date (FR-D5).
- [ ] docs/providers.md documents qwen-code (experimental + DashScope + # TO CONFIRM) + the refreshed tokens.
- [ ] providers/gemini.toml + agy.toml mirror the refreshed manifest (field + rendered-command comment).
- [ ] Implementation summary records: the stager="" deviation (§1), the gemini refresh deltas, the bootstrap
      consistency fix, the S1/S2 non-overlap, the docs narrow scope.

---

## Anti-Patterns to Avoid

- ❌ **Don't set qwen-code `stager = "qwen3-coder-flash"`.** The literal contract text says that, but it
  BREAKS the stager: S1's `builtinQwenCode()` has `TooledFlags=nil`, and `stagerFallback` (bootstrap.go:75)
  treats a non-empty stager cell as "this provider IS the stager" → it'd route the stager to qwen-code →
  `RenderTooled` errors on nil tooled_flags. Every non-stager-capable provider uses `stager=""`. PRD §9.16's
  own note mandates the fallback. Use `stager=""` + add qwen-code to `StagerCapability`'s incapable list.
- ❌ **Don't leave the gemini/agy refresh half-done.** Changing `default_model` in builtin.go ripples to
  geminiTOML/agyTOML + Gemini/AgyFields + RenderedCommand_Gemini/Agy + providers/gemini.toml + agy.toml.
  Miss any and a pinning test fails. Likewise the role_defaults.go `gemini-3.5-pro`→`gemini-3.1-pro` must
  update `TestDefaultModelsForProvider_PerProvider`.
- ❌ **Don't touch qwen-code's manifest value or qwen-code tests.** S1 owns `builtinQwenCode()` (DefaultModel
  "qwen3-coder-plus" = the flagship, already correct) + providers/qwen-code.toml + the qwen-code tests.
  S2's edits to builtin.go/builtin_test.go are in the gemini/agy regions ONLY (§7 non-overlap).
- ❌ **Don't leave bootstrap.go's local `preferredBuiltins` stale.** It's a SEPARATE copy from registry.go's.
  S1 updates registry.go to 8 elements; without S2 adding qwen-code here, qwen-code gets no commented
  `[role.*]` block in `config init`. Keep the two copies in sync (the local copy's doc comment promises this).
- ❌ **Don't over-scope docs/providers.md.** The broader v3-schema inconsistencies (default_provider field,
  "18 fields", agent→provider terminology) are P4.M2.T1.S1's. S2 touches ONLY the qwen-code rows + the gemini
  token refresh + the count/order directly tied to adding qwen-code.
- ❌ **Don't change gemini/agy `message` or `arbiter`.** Only `planner` is stale (3.5-pro→3.1-pro). The
  message (`gemini-3.1-flash-lite`) and arbiter (`gemini-3.5-flash`) values are ALREADY correct.
- ❌ **Don't drop the `# TO CONFIRM` discipline.** qwen-code/cursor/codex tokens are unverified (no live CLI
  lookup this pass) — keep them marked `# TO CONFIRM per FR-D5` in role_defaults.go + docs. Pin gemini/agy/
  claude/pi/opencode with a verification date.
- ❌ **Don't add imports, change the Manifest struct, or touch the Git interface.** This is data + pinning-
  tests + docs. go.mod/go.sum byte-unchanged. The v3 schema (P1.M1.T1.S1) is COMPLETE.
- ❌ **Don't edit `internal/cmd/*`, internal/git/*, internal/decompose/*, internal/generate/*, internal/prompt/*,
  pkg/*.** Out of scope. S2 is internal/config/{role_defaults,bootstrap} + internal/provider/{builtin} +
  providers/*.toml + docs/providers.md ONLY.
