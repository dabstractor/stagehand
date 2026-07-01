---
name: "P1.M3.T3.S1 — RoleModelDefaults table + per-provider default models (FR-D4): the static provider→role→model lookup table + DefaultModelsForProvider accessor that the config bootstrap (P1.M4.T2) reads to populate per-role model defaults — PRD §9.16 FR-D3/FR-D4/FR-D5"
description: |

  Land the ONLY subtask of "Define tier-based per-provider × per-role default model table" (P1.M3.T3):
  CREATE `internal/config/role_defaults.go` (`package config`, import-free) exporting the FR-D4
  per-provider × per-role default-model table + accessor. Three deliverables:

    1. `internal/config/role_defaults.go`  — CREATE:
       - `type RoleModelDefaults map[string]map[string]string` (provider → role → model; PRD §9.16 FR-D4);
       - the static `var roleDefaults` table: all 7 built-in providers × the 4 canonical roles
         (planner/stager/message/arbiter), with the §16.4 model values;
       - `func DefaultModelsForProvider(name string) map[string]string` — returns a COPY of the named
         provider's role→model column, or nil for an unknown provider;
       - the FR-D5 verification comment block (date 2026-07, source = PRD §9.16 + item exemplars, the
         per-provider re-verification status, and the stager-capability basis).
    2. `internal/config/role_defaults_test.go` — CREATE: white-box `package config` table-driven tests
       (per-provider column correctness incl. stager="", all-4-roles-present, unknown→nil, copy
       semantics, table-key sanity). Mirror `internal/config/config_test.go`.
    3. `docs/providers.md` — MODIFY: ADD a "## Per-role default models (FR-D4)" section (the FR-D3 tier
       strategy + the FR-D4 table) after the existing "## Tooled mode and the stager role" section.

  SCOPE BOUNDARY (load-bearing): this subtask provides the DATA + ACCESSOR ONLY. It does NOT bootstrap a
  config (that is P1.M4.T2 `config init`), does NOT resolve the stager fallback (pick the next
  TooledFlags-capable provider — the bootstrap/orchestrator's job), does NOT call BuiltinManifests() or
  the registry (the table is a pure static constant — it does NOT import internal/provider), and does NOT
  modify config.go/roles.go/load.go/builtin.go. See research design-decisions.md §0/§11.

  INPUT (upstream — already built, read-only): the 7 built-in provider NAMES + TooledFlags state come
  from `internal/provider/builtin.go` BuiltinManifests() (pi/claude/gemini/opencode/codex/cursor/agy).
  The model NAMES come from PRD §9.16 FR-D4 + the work-item exemplars (item_description §1). The
  stager-capability basis is the manifest's TooledFlags field (manifest.go line 68).

  OUTPUT (downstream consumer): the config bootstrap (P1.M4.T2) iterates provider.BuiltinManifests() to
  find installed providers, calls config.DefaultModelsForProvider(detectedName) to get the role→model
  column, and writes the detected provider's [role.*] block UNCOMMENTED (FR-B1 step 3) + other installed
  providers COMMENTED (step 4). It interprets stager=="" as "cannot be stager" and applies the FR-D4
  fallback. The RoleModelDefaults type + DefaultModelsForProvider signature are FROZEN after this subtask.

  ⚠️ **THE #1 key call — stager="" for the 5 NON-stager-capable providers (TRUST THE CODE, not the stale
  PRD note).** The contract: "providers that cannot serve as stager (empty tooled_flags) ⇒ stager=''". The
  AUTHORITATIVE capability source is `internal/provider/builtin.go` (TooledFlags field), NOT the PRD §9.16
  note (which is STALE — it names only "agy and opencode" but the code shows FIVE providers lack
  TooledFlags). Verified by reading every manifest: ONLY `pi` (line 64) + `claude` (line 108) have
  non-empty TooledFlags → stager-capable. `gemini`, `agy`, `opencode`, `codex`, `cursor` ALL have nil
  TooledFlags → stager="". This matches docs/providers.md's existing "Stager?" column (pi/claude=yes,
  rest=no). The implementing agent MUST re-confirm the TooledFlags state in builtin.go and set the stager
  column accordingly. stager="" is a SIGNAL the bootstrap interprets as "apply FR-D4 fallback". (§3)

  ⚠️ **THE #2 key call — placement: internal/config/role_defaults.go (NOT provider/).** The consumer is
  config bootstrap (P1.M4.T2); config is a leaf that CANNOT import provider (import cycle —
  provider/registry.go imports config); a static table needs NO imports. The contract lists config FIRST.
  Co-located with roles.go (P1.M3.T2.S2) — both are per-role config data. (§1)

  ⚠️ **THE #3 key call — DefaultModelsForProvider returns a COPY (defensive), not the internal map.**
  Returning the live map lets callers mutate the package table (latent bug). Mirror BuiltinManifests()
  fresh-per-call discipline: return a freshly-allocated map copy (4 entries). nil for unknown provider.
  A test pins the copy semantics. (§7)

  ⚠️ **THE #4 key call — FR-D5 re-verification is MANDATORY but delegated.** Model names are the PRD's
  2026-07 product spec (gpt-5.4, gemini-3.5-pro, opus/sonnet/haiku, etc.). The PRP-research agent cannot
  independently verify 2026-07 fictional models against external live docs (would conflict with the PRD's
  timeline). The PRD FR-D4 table + work-item exemplars are the authoritative BASELINE. The implementing
  agent MUST attempt re-verification per provider (against each CLI's --help/docs) and record verified
  names + date in the FR-D5 comment block. Defaults authored trivially-refreshable (one cell per
  provider×role). (§4/§6)

  ⚠️ **THE #5 key call — cursor's cells are UNVERIFIED best-guesses.** The FR-D4 table gives cursor TIER
  NAMES (flagship/mid/nano), not concrete models; the work-item exemplars do NOT list cursor. Best-guess:
  planner=gpt-5.4, message=gpt-5.4-nano, arbiter=gpt-5.4-mini (OpenAI-tier tokens — cursor is
  OpenAI-backed). Prominently mark "FR-D5: VERIFY against agent --help". (§5)

  ⚠️ **Model-string conventions (FR-R5/FR-R5b):** opencode models are provider-PREFIXED ("openai/gpt-5.4"
  — opencode's ProviderFlag is empty, provider is part of the model string); pi + claude + codex + agy +
  gemini are BARE (pi sets its sub-provider separately via --provider; claude uses bare aliases
  opus/sonnet/haiku that resolve to the current gen). Match each provider's manifest. (§5)

  ⚠️ **Exact keys:** providers = the manifest NAME field = {pi, claude, gemini, opencode, codex, cursor,
  agy} (cursor's key is "cursor", NOT "agent"). Roles = {planner, stager, message, arbiter} (lowercase,
  matching roleNames in load.go). Every provider column has all 4 role keys present (stager key present
  even when value is ""). (§8)

  ⚠️ **DOCS (Mode A):** ADD a "## Per-role default models (FR-D4)" section to docs/providers.md (after
  "## Tooled mode and the stager role") with the FR-D3 tier strategy + the FR-D4 table. Do NOT touch the
  existing "Stager?" column (already correct). (§10)

  Deliverable: CREATE internal/config/role_defaults.go + role_defaults_test.go; MODIFY docs/providers.md
  (additive section). NO edit to config.go, roles.go, load.go, file.go, git.go, internal/provider/*,
  cmd/*, pkg/*, Makefile, go.mod/go.sum, PRD.md, providers/*.toml. NO new dependency (import-free).

---

## Goal

**Feature Goal**: Materialize the PRD §9.16 FR-D4 per-provider × per-role default-model table as a
compiled-in Go data structure (`RoleModelDefaults`) with a single accessor
(`DefaultModelsForProvider`), so the config bootstrap (P1.M4.T2 `config init`) can populate a working
`[role.*]` config for the detected provider in one lookup — giving each of the four agent roles
(planner/stager/message/arbiter) the right tier-sized model out of the box (FR-D3: planner=smart,
stager=mid, message=fast, arbiter=mid), with `stager=""` encoding which providers cannot serve as the
stager (empty `tooled_flags`). This is the DATA half of P1.M3.T3; the bootstrap that WRITES it is
P1.M4.T2, and the per-role RESOLUTION that READS a populated config is P1.M3.T2.S2 (`ResolveRoleModel`).

**Deliverable**:
1. `internal/config/role_defaults.go` — `type RoleModelDefaults map[string]map[string]string`; the static
   `var roleDefaults` (7 providers × 4 roles, FR-D4 values, stager="" for the 5 non-capable providers);
   `func DefaultModelsForProvider(name string) map[string]string` (returns a copy or nil); the FR-D5
   verification comment block.
2. `internal/config/role_defaults_test.go` — white-box `package config` table-driven tests (per-provider
   column correctness incl. stager="", all-4-roles-present, unknown→nil, copy semantics, key sanity).
3. `docs/providers.md` — ADD a "## Per-role default models (FR-D4)" section (FR-D3 tier strategy +
   FR-D4 table).

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `DefaultModelsForProvider("pi")`
returns `{planner:gpt-5.4, stager:gpt-5.4-mini, message:gpt-5.4-nano, arbiter:gpt-5.4-mini}`;
`DefaultModelsForProvider("gemini")` returns a column with `stager:""` (gemini can't be stager); the same
for agy/opencode/codex/cursor; only pi+claude have non-empty stager; `DefaultModelsForProvider("nope")==nil`;
mutating a returned map does not change the table (copy semantics); every provider column has exactly the
4 role keys; the table has exactly the 7 provider keys; go.mod/go.sum byte-unchanged; config.go + roles.go
+ load.go + every provider/* file + every non-target file byte-unchanged (only docs/providers.md modified
additively + the 2 new files).

## User Persona

**Target User**: The config bootstrap (P1.M4.T2 `config init` / first-run fallback) — it calls
`config.DefaultModelsForProvider(detectedName)` to get the detected provider's role→model column, then
writes `[role.planner]`, `[role.stager]`, `[role.message]`, `[role.arbiter]` blocks (FR-B1 step 3,
uncommented for the detected provider; commented for other installed providers, step 4). It interprets
`stager==""` as "this provider cannot be the stager" and applies the FR-D4 fallback (writes the stager
block for the next TooledFlags-capable provider, annotated). Transitively the end user is "the
multi-agent tinkerer" (PRD §7.3) who runs `stagehand config init` and immediately gets a working
per-role config sized to each role's job (planner on the flagship, message on the cheap-fast tier).

**Use Case**: A user installs stagehand; `config init` detects `pi` on `$PATH` (FR-D1), calls
`DefaultModelsForProvider("pi")` → `{planner:gpt-5.4, stager:gpt-5.4-mini, message:gpt-5.4-nano,
arbiter:gpt-5.4-mini}`, and writes those four `[role.*]` blocks uncommented so the tool works
immediately on both the single-commit path (message role) and the multi-commit path (all four roles).

**User Journey**: (internal) `config init` → `Registry.DefaultProvider(installed)` (FR-D1) →
`config.DefaultModelsForProvider(detected)` → write `[role.*]` blocks → user runs stagehand →
`ResolveRoleModel(role, cfg)` (P1.M3.T2.S2) resolves each role's (provider,model) from the populated
config → registry renders the command → agent runs.

**Pain Points Addressed**: (1) No compiled-in per-role default models — the bootstrap would have to
hardcode them inline (duplicated, stale-prone). (2) Users getting the wrong tier per role (flagship for
a cheap text-gen message, or a cheap model for tool-use staging) — solved by the FR-D3 tier strategy
baked into the table. (3) Stager routed to a provider that can't do tooled mode — solved by stager=""
signaling non-capability. (4) Stale model names — mitigated by the FR-D5 verification block + the
trivially-refreshable one-cell-per-role structure.

## Why

- **Completes the per-role DEFAULTS data layer.** P1.M3.T2.S2 (`ResolveRoleModel`) is the READ side that
  resolves a role's (provider,model) from a populated config; THIS subtask is the DEFAULT DATA the
  bootstrap writes into that config in the first place. Without it, `config init` has no per-role models
  to write and the user must hand-configure every role.
- **Satisfies PRD §9.16 FR-D3 (tier strategy) + FR-D4 (per-provider table) + FR-D5 (verification).**
  The table IS FR-D4 materialized as Go; the FR-D3 tier strategy is encoded in the column values
  (planner=flagship-tier model, stager=mid-tier, message=fast-tier, arbiter=mid-tier); the FR-D5
  verification block records provenance + the re-verification mandate.
- **Trivially refreshable (FR-D5).** One cell per provider×role — a future periodic-refresh process (out
  of scope) updates the table in one place; the bootstrap and resolver need no changes.
- **Decoupled from the provider package.** A pure static table in `internal/config` (import-free) keeps
  config a leaf and avoids the config↔provider import cycle. The capability basis (TooledFlags) is read
  from builtin.go at AUTHORING time and encoded as stager=""/value; the bootstrap re-checks live
  TooledFlags when applying the fallback.

## What

NEW `internal/config/role_defaults.go` (the type + table + accessor + FR-D5 comment block), NEW
`internal/config/role_defaults_test.go` (the tests), and an ADDITIVE section in `docs/providers.md`.
No new dependency, no edits to config.go/roles.go/load.go/provider/*.

### Success Criteria

- [ ] `role_defaults.go` is `package config`, import-free, defines exported `type RoleModelDefaults
      map[string]map[string]string`, an unexported `var roleDefaults = RoleModelDefaults{…}` (7 providers
      × 4 roles), and exported `func DefaultModelsForProvider(name string) map[string]string`.
- [ ] The FR-D5 verification comment block is present (date 2026-07, source, per-provider status,
      stager-capability basis) above `roleDefaults`.
- [ ] `DefaultModelsForProvider("pi")` == `{planner:"gpt-5.4", stager:"gpt-5.4-mini", message:"gpt-5.4-nano",
      arbiter:"gpt-5.4-mini"}`; `("claude")` == `{planner:"opus", stager:"sonnet", message:"haiku",
      arbiter:"sonnet"}` (the only two stager-capable — non-empty stager).
- [ ] `DefaultModelsForProvider("gemini")`, `("agy")`, `("opencode")`, `("codex")`, `("cursor")` each
      return a 4-key column with `stager:""` (NOT stager-capable — nil TooledFlags in builtin.go). The
      other 3 roles carry their FR-D4 values (e.g. gemini: planner=gemini-3.5-pro, message=gemini-3.1-flash-lite,
      arbiter=gemini-3.5-flash).
- [ ] opencode's models are provider-PREFIXED (`openai/gpt-5.4`, `openai/gpt-5.4-nano`, `openai/gpt-5.4-mini`);
      cursor's cells carry an inline "FR-D5: verify" comment.
- [ ] `DefaultModelsForProvider("nonexistent") == nil`.
- [ ] Mutating a returned map does NOT change the table (copy semantics — re-call returns the original values).
- [ ] Every provider column has EXACTLY the 4 role keys {planner, stager, message, arbiter}; the table has
      EXACTLY the 7 provider keys {pi, claude, gemini, opencode, codex, cursor, agy}.
- [ ] `docs/providers.md` has a new "## Per-role default models (FR-D4)" section (FR-D3 tier strategy +
      FR-D4 table) placed after "## Tooled mode and the stager role"; the existing "Stager?" column is
      unchanged.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/` clean.
- [ ] go.mod/go.sum byte-unchanged; config.go + roles.go + load.go + file.go + git.go + every
      internal/provider/* file + cmd/* + pkg/* + Makefile byte-unchanged (only the 2 new files +
      docs/providers.md differ).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact table values
(spelled out in the Blueprint + design-decisions §4), the stager-capability basis (read builtin.go's
TooledFlags — only pi/claude), the type/accessor signatures (quoted verbatim), the 6 test cases (each
spelled out), the docs section content (given verbatim), and the LEAVE list. No registry/render/git/prompt
knowledge required — this is a static map literal + a map-copy accessor + a docs section.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions (the judgment calls)
- docfile: plan/002_a17bb6c8dc1d/P1M3T3S1/research/design-decisions.md
  why: the 11 decisions — scope (§0), placement in config (§1), the type+accessor design (§2), THE stager=""
       decision + the stale-PRD-note finding (§3 — read builtin.go, only pi/claude are stager-capable),
       the model names + FR-D5 (§4), per-provider model-string conventions + the cursor uncertainty (§5),
       the FR-D5 verification comment block (§6), copy semantics (§7), exact keys (§8), tests (§9), the
       docs/providers.md update (§10), frozen files + contracts (§11).
  critical: §3 (stager="" for 5 providers — trust builtin.go TooledFlags, NOT the PRD note), §4/§6 (FR-D5
       re-verification mandate + the comment block), §5 (cursor is unverified best-guess; opencode is
       provider-prefixed), §1 (config placement — no provider import) are the things most likely to go wrong.

# MUST READ — the stager-capability SOURCE OF TRUTH (read every manifest's TooledFlags; do NOT edit)
- file: internal/provider/builtin.go   (read for TooledFlags + provider Names; do NOT edit)
  section: `BuiltinManifests()` returns {pi, claude, gemini, opencode, codex, cursor, agy} (the 7 Names —
           these are the table's provider keys; note cursor's Name is "cursor" though detect/command="agent").
           The TooledFlags field per manifest: pi (line 64, []string{...}) + claude (line 108, []string{...})
           are the ONLY two with non-empty TooledFlags. gemini/agy/opencode/codex/cursor have nil TooledFlags
           (agy is explicit `// TooledFlags: nil` line 184; the others simply omit the field).
  why: the §3 stager-capability basis. A provider's stager cell is non-empty IFF its manifest has non-empty
       TooledFlags. ONLY pi + claude qualify → the other 5 get stager="". The PRD §9.16 note ("agy and
       opencode today") is STALE (it names 2; the code shows 5) — trust the code + docs/providers.md's
       "Stager?" column, not the prose.
  critical: re-confirm the TooledFlags state at implementation (the table must match the live manifests).
       Do NOT give gemini/codex/cursor a stager model — they CANNOT be the stager (RenderTooled errors on
       nil tooled_flags). Do NOT edit builtin.go.

# MUST READ — the model-name baseline (the FR-D4 table + exemplars)
- file: PRD.md (or plan/002_a17bb6c8dc1d/prd_snapshot.md)
  section: "9.16 Default provider & per-role model defaults" (h3.32) — FR-D3 (tier strategy: planner=smart,
           stager=mid, message=fast, arbiter=mid), FR-D4 (the per-provider × per-role table), FR-D5 (the
           re-verification mandate). The FR-D4 table values are the authoritative BASELINE for the model names.
  critical: FR-D4's cursor row is TIER NAMES ("flagship"/"mid"/"nano"), not concrete models — resolve to
       best-guess OpenAI tokens (gpt-5.4/gpt-5.4-mini/gpt-5.4-nano) and mark "FR-D5: verify". FR-D4's
       opencode row is provider-prefixed ("openai/gpt-5.4"). FR-D4's claude row uses bare aliases with
       gen annotations (opus=4.8, sonnet=5) — store bare (opus/sonnet/haiku). FR-D5 MANDATES re-verification.

# MUST READ — the sibling per-role file + the canonical role names
- file: internal/config/roles.go   (P1.M3.T2.S2 — read for the per-role READ-side sibling + style; do NOT edit)
  section: `ResolveRoleModel(role string, cfg Config) (provider, model string)` — the READ side that resolves
           a role's (provider,model) from a POPULATED cfg. role_defaults.go is the DEFAULT DATA the bootstrap
           writes into cfg so ResolveRoleModel has per-role values to return.
  why: co-locates the role-config knowledge in internal/config. Confirms the 4 canonical roles
       (planner/stager/message/arbiter) and the per-role-config design philosophy (pure data, no provider
       import). Mirror its doc-comment density (cite PRD §/FR).
  critical: do NOT edit roles.go. role_defaults.go is its DATA sibling (both read-only consumers of cfg /
       providers of defaults), not a modifier.

# MUST READ — the canonical role-name list (the table's role keys must match it)
- file: internal/config/load.go   (P1.M3.T2.S1 — read for roleNames; do NOT edit)
  section: `var roleNames = []string{"planner", "stager", "message", "arbiter"}` (package-level).
  why: confirms the EXACT role-key strings (lowercase) the table must use. The table's role keys must
       match roleNames so the bootstrap's role-iteration aligns. (Do NOT import load.go's roleNames into
       role_defaults.go — hardcode the 4 strings; they're stable. But they MUST be identical strings.)
  critical: do NOT edit load.go. Hardcode {"planner","stager","message","arbiter"} in role_defaults.go.

# THE TEST STYLE TO MIRROR — white-box package config, one t.Errorf per assertion
- file: internal/config/config_test.go   (read for style; do NOT edit)
  section: `TestDefaults` — `package config`, construct the value under test directly, one descriptive
           `t.Errorf` per assertion.
  why: the test STYLE — white-box `package config`, build expectations directly (here: expected per-provider
       maps), assert with descriptive t.Errorf. Use a table loop over the 7 providers for the per-column
       case + explicit sub-tests for unknown→nil and copy semantics.
  critical: do NOT edit config_test.go. Mirror its assertion style in role_defaults_test.go.

# THE DOCS FILE TO UPDATE (additive section) + the existing stager context
- file: docs/providers.md   (MODIFY — add a section)
  section: "## Tooled mode and the stager role" (ends ~line 95) — ADD the new "## Per-role default models
           (FR-D4)" section AFTER it, BEFORE "## Adding a new agent" (line 97). The existing "The 7 built-in
           providers" table already has a correct "Stager?" column (pi/claude=yes, rest=no) — do NOT alter it.
  why: the DOCS (Mode A) requirement — "Update docs/providers.md to show the tier assignments." The new
       section shows FR-D3 (tier strategy) + FR-D4 (per-provider table) + the stager-fallback note.
  critical: ADDITIVE only. Do NOT rewrite the file or alter the existing tables. The exact section content
       is given in the Blueprint (Task 4) — paste then adapt any FR-D5-verified names.

# The manifest schema (the TooledFlags field the stager-capability basis reads)
- file: internal/provider/manifest.go   (read for the field; do NOT edit)
  section: `TooledFlags []string toml:"tooled_flags"` (line 68) — nil is the natural "absent" sentinel for
           slices (line 31). A provider with nil/empty TooledFlags cannot serve as the stager.
  why: confirms the capability signal. The table's stager column encodes this: non-empty stager IFF the
       provider's manifest has non-empty TooledFlags (only pi, claude). Do NOT edit manifest.go.
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go            # M3.T1.S1 SHIPPED (FROZEN) — RoleConfig + Roles/Provider/Model. READ for fields; do NOT edit.
  config_test.go       # M3.T1.S1. READ for test style; do NOT edit.
  file.go / file_test.go   # M3.T1.S2 — overlay()/materialize(). UNCHANGED.
  git.go / git_test.go     # git-config reader. UNCHANGED.
  load.go / load_test.go   # P1.M3.T2.S1 — roleNames + setRole* + Load(). READ for roleNames; do NOT edit.
  roles.go / roles_test.go # P1.M3.T2.S2 — ResolveRoleModel. READ for the read-side sibling; do NOT edit.
  role_defaults.go      # *** CREATE *** — RoleModelDefaults + roleDefaults + DefaultModelsForProvider + FR-D5 block.
  role_defaults_test.go # *** CREATE *** — the tests (this subtask).
internal/provider/
  builtin.go            # the 7 manifests + TooledFlags. READ (the §3 capability basis + provider Names); do NOT edit.
  manifest.go           # Manifest struct (TooledFlags field). READ; do NOT edit.
  registry.go           # imports config (the import-cycle reason config can't import provider). UNCHANGED.
docs/providers.md       # *** MODIFY *** — ADD "## Per-role default models (FR-D4)" section.
go.mod / go.sum         # UNCHANGED (role_defaults.go needs NO imports; test uses only testing).
```

### Desired Codebase tree with files to be added

```bash
internal/config/
  role_defaults.go      # NEW — RoleModelDefaults type + roleDefaults table (7×4) + DefaultModelsForProvider + FR-D5 block.
  role_defaults_test.go # NEW — white-box package config tests (per-provider columns, unknown→nil, copy, key sanity).
docs/providers.md       # MODIFIED — + "## Per-role default models (FR-D4)" section (FR-D3 tiers + FR-D4 table).
# (NO other file changes. go.mod/go.sum unchanged.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — stager="" for the 5 NON-stager-capable providers; TRUST THE CODE): the AUTHORITATIVE
//   capability source is internal/provider/builtin.go (TooledFlags), NOT the PRD §9.16 note. Verified:
//   ONLY pi (line 64) + claude (line 108) have non-empty TooledFlags → stager-capable (non-empty stager).
//   gemini/agy/opencode/codex/cursor have nil TooledFlags → stager="". The PRD note ("agy and opencode
//   today") is STALE (names 2; code shows 5). Re-confirm TooledFlags in builtin.go at implementation.
//   stager="" is a SIGNAL: the bootstrap (P1.M4.T2) applies the FR-D4 fallback on stager=="". (§3)

// CRITICAL (#2 — placement internal/config/role_defaults.go, NOT provider/): config is a leaf that CANNOT
//   import provider (provider/registry.go imports config → cycle). The table is import-free. The consumer
//   is config bootstrap (P1.M4.T2). The contract lists config FIRST. (§1)

// CRITICAL (#3 — DefaultModelsForProvider returns a COPY, not the internal map): returning the live map
//   lets callers mutate the package table. Return a freshly-allocated map copy (4 entries). nil for unknown.
//   Mirror BuiltinManifests()'s fresh-per-call discipline. A test pins copy semantics. (§7)

// CRITICAL (#4 — FR-D5 re-verification MANDATORY but delegated): the model names are the PRD's 2026-07
//   baseline (gpt-5.4, gemini-3.5-pro, opus/sonnet/haiku, …). The PRP-research agent could NOT verify
//   2026-07 fictional models against external live docs (would conflict). The implementing agent MUST
//   attempt per-provider re-verification (--help/docs) and record verified names + date in the FR-D5 block.
//   cursor's cells are UNVERIFIED best-guesses (PRD gives tier names) — mark "FR-D5: verify". (§4/§5/§6)

// CRITICAL (#5 — model-string conventions): opencode = provider-PREFIXED ("openai/gpt-5.4" — empty
//   ProviderFlag, provider is part of the model string). pi/claude/codex/agy/gemini = BARE (pi sets
//   sub-provider separately via --provider; claude uses bare aliases opus/sonnet/haiku resolving to current
//   gen). Match each provider's manifest. (§5)

// GOTCHA (exact keys): providers = manifest Name = {pi, claude, gemini, opencode, codex, cursor, agy}
//   (cursor's key is "cursor", NOT "agent"). Roles = {planner, stager, message, arbiter} (lowercase,
//   matching load.go roleNames). Every provider column has all 4 role keys (stager key present even
//   when value is ""). (§8)

// GOTCHA (the type is EXPORTED, the var is UNEXPORTED): `type RoleModelDefaults` exported (contract);
//   `var roleDefaults` unexported (accessed only via DefaultModelsForProvider, which returns copies).
//   Do NOT export the var (callers must go through the accessor). (§2)

// GOTCHA (import-free): role_defaults.go needs NO imports (a map literal + a map lookup). role_defaults_test.go
//   uses only "testing". `go mod tidy` MUST be a no-op. Do NOT import internal/provider (cycle). (§1)

// GOTCHA (docs update is ADDITIVE): add ONE section to docs/providers.md; do NOT rewrite it or alter the
//   existing "Stager?" column (already correct). (§10)

// GOTCHA (do NOT pre-empt the bootstrap): this subtask owns the TABLE + ACCESSOR ONLY. config init
//   (P1.M4.T2), the stager-fallback selection, and the [role.*] writing are P1.M4.T2's job. Do NOT
//   implement them. (§0/§11)
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/role_defaults.go — ONE exported type, ONE unexported table var, ONE exported accessor.
// NO new structs beyond the RoleModelDefaults map type. NO imports.

// RoleModelDefaults is the FR-D4 per-provider × per-role default-model table (PRD §9.16 FR-D4):
// keyed provider → role → model. The 4 roles are planner/stager/message/arbiter (FR-R1); the 7 providers
// are the built-in manifest Names. A stager value of "" means "this provider cannot serve as the stager"
// (its manifest has nil/empty TooledFlags) — the bootstrap applies the FR-D4 fallback on that signal.
type RoleModelDefaults map[string]map[string]string
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/config/role_defaults.go — type + table + accessor + FR-D5 block
  - FILE: NEW internal/config/role_defaults.go. PACKAGE: `package config`. IMPORTS: NONE.
  - WRITE the FR-D5 verification comment block (design-decisions §6 — date 2026-07, source, per-provider
      status, stager-capability basis). Place it immediately above `var roleDefaults`.
  - DEFINE `type RoleModelDefaults map[string]map[string]string` with a doc comment citing PRD §9.16 FR-D4
      + the stager="" semantics.
  - DEFINE `var roleDefaults = RoleModelDefaults{ … }` with all 7 providers × 4 roles (the §4 values).
      Per-cell inline comments: "// stager-capable (TooledFlags set)" for pi/claude stager;
      "// NOT stager-capable (TooledFlags nil) — bootstrap applies FR-D4 fallback" for the 5 "" stagers;
      "// FR-D5: PRD tier-name → best-guess OpenAI token; VERIFY agent --help" for cursor's cells.
      Use the EXACT values:
        pi:      planner=gpt-5.4,        stager=gpt-5.4-mini,   message=gpt-5.4-nano,        arbiter=gpt-5.4-mini
        claude:  planner=opus,           stager=sonnet,         message=haiku,                arbiter=sonnet
        gemini:  planner=gemini-3.5-pro, stager="",             message=gemini-3.1-flash-lite,arbiter=gemini-3.5-flash
        opencode:planner=openai/gpt-5.4, stager="",             message=openai/gpt-5.4-nano, arbiter=openai/gpt-5.4-mini
        codex:   planner=gpt-5.1-codex-max, stager="",          message=gpt-5.4-nano,        arbiter=gpt-5.1-codex-mini
        cursor:  planner=gpt-5.4,        stager="",             message=gpt-5.4-nano,        arbiter=gpt-5.4-mini   (FR-D5: verify)
        agy:     planner=gemini-3.5-pro, stager="",             message=gemini-3.1-flash-lite,arbiter=gemini-3.5-flash
  - DEFINE `func DefaultModelsForProvider(name string) map[string]string` — returns a COPY of
      roleDefaults[name] (fresh map, 4 entries), or nil if name is unknown. Doc comment cites PRD §9.16
      FR-D4 + the copy semantics + the stager="" signal + the downstream consumer (P1.M4.T2 bootstrap).
  - GOTCHA: import-free; the var unexported; the type + function exported.

Task 2: CREATE internal/config/role_defaults_test.go — white-box package config tests
  - FILE: NEW internal/config/role_defaults_test.go. PACKAGE: `package config`. IMPORT: "testing".
  - CASES (mirror config_test.go's style — construct expectations directly, one t.Errorf per assertion):
      * TestDefaultModelsForProvider_PerProvider — a table over the 7 names, each asserting its 4-role
        column matches the §4 values (hardcoded expected maps — NOT derived from the table). PINS stager=""
        for the 5 non-capable + non-empty stager for pi/claude. (The load-bearing test.)
      * TestDefaultModelsForProvider_AllRolesPresent — for each provider, len(col)==4 AND each of
        planner/stager/message/arbiter is a key (stager key present even when "").
      * TestDefaultModelsForProvider_StagerCapability — explicitly assert pi+claude stager != "" and the
        other 5 stager == "" (isolates §3 from the per-provider case).
      * TestDefaultModelsForProvider_UnknownReturnsNil — DefaultModelsForProvider("nonexistent") == nil.
      * TestDefaultModelsForProvider_CopySemantics — get a column, mutate it, re-get; the second call
        returns the original values (the table was NOT mutated). PINS §7.
      * TestRoleDefaults_KeySanity — roleDefaults has EXACTLY the 7 provider keys; no provider column has
        a role key outside {planner,stager,message,arbiter}.
  - GOTCHA: hardcode expected values (do NOT derive from the table — circular). White-box package config.

Task 3: MODIFY docs/providers.md — ADD the "## Per-role default models (FR-D4)" section
  - PLACE the new section AFTER "## Tooled mode and the stager role" (ends ~line 95) and BEFORE
      "## Adding a new agent" (line 97).
  - CONTENT (paste, then adapt any FR-D5-verified names):
      * A one-paragraph intro: out of the box each role is tier-sized (FR-D3); the table (FR-D4) is the
        compiled-in source (internal/config/role_defaults.go); models are 2026-07 baselines (FR-D5:
        re-verify per provider).
      * The FR-D3 tier strategy (planner=flagship/smart, stager=mid, message=fast, arbiter=mid) — one line
        each with the one-clause rationale.
      * The FR-D4 per-provider table (the §4 table above) — markdown table with the 7 providers × 4 roles,
        stager marked "" for the 5 non-capable + a footnote that the stager falls back to the next
        TooledFlags-capable provider.
  - GOTCHA: ADDITIVE only — do NOT alter the existing "The 7 built-in providers" table or its "Stager?"
      column (already correct).

Task 4: VERIFY (no further edits)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. config.go + roles.go +
      load.go + file.go + git.go + every internal/provider/* file + cmd/* + pkg/* + Makefile byte-unchanged.
      `go build ./... && go test ./...` green. `git status` shows: 2 new files (role_defaults.go,
      role_defaults_test.go) + 1 modified (docs/providers.md); NOTHING else.
```

### Implementation Patterns & Key Details

```go
// === internal/config/role_defaults.go ===

package config // import-free

// [FR-D5 verification comment block — see design-decisions §6; date 2026-07, source, per-provider
//  status, stager-capability basis. Pasted verbatim from the Blueprint §6 / Task 1.]

// RoleModelDefaults is the PRD §9.16 FR-D4 per-provider × per-role default-model table, keyed
// provider → role → model. The four roles are planner/stager/message/arbiter (FR-R1). A stager value
// of "" means the provider cannot serve as the stager (its built-in manifest has nil/empty TooledFlags
// — only pi and claude are stager-capable); the bootstrap (P1.M4.T2) applies the FR-D4 fallback on
// that signal. See the FR-D5 block above for model-name provenance + the re-verification mandate.
type RoleModelDefaults map[string]map[string]string

// roleDefaults is the compiled-in FR-D4 table (unexported; access via DefaultModelsForProvider, which
// returns copies). Stager cells: non-empty IFF the provider's manifest has non-empty TooledFlags
// (pi, claude); "" otherwise (gemini, agy, opencode, codex, cursor) — the bootstrap applies the fallback.
var roleDefaults = RoleModelDefaults{
	"pi": {
		"planner": "gpt-5.4",      // flagship/smart tier (FR-D3)
		"stager":  "gpt-5.4-mini", // stager-capable (TooledFlags set in builtin.go)
		"message": "gpt-5.4-nano", // fast tier
		"arbiter": "gpt-5.4-mini", // mid tier
	},
	"claude": {
		"planner": "opus",   // flagship/smart (bare alias → current gen, opus=4.8)
		"stager":  "sonnet", // stager-capable (TooledFlags set); bare alias (sonnet=5)
		"message": "haiku",  // fast tier
		"arbiter": "sonnet", // mid tier
	},
	"gemini": {
		"planner": "gemini-3.5-pro",
		"stager":  "", // NOT stager-capable (TooledFlags nil) — bootstrap applies FR-D4 fallback
		"message": "gemini-3.1-flash-lite",
		"arbiter": "gemini-3.5-flash",
	},
	"agy": {
		"planner": "gemini-3.5-pro",
		"stager":  "", // NOT stager-capable (TooledFlags nil)
		"message": "gemini-3.1-flash-lite",
		"arbiter": "gemini-3.5-flash",
	},
	"opencode": {
		"planner": "openai/gpt-5.4",      // provider-prefixed (opencode ProviderFlag empty)
		"stager":  "",                     // NOT stager-capable (TooledFlags nil)
		"message": "openai/gpt-5.4-nano",
		"arbiter": "openai/gpt-5.4-mini",
	},
	"codex": {
		"planner": "gpt-5.1-codex-max",
		"stager":  "", // NOT stager-capable (TooledFlags nil)
		"message": "gpt-5.4-nano",
		"arbiter": "gpt-5.1-codex-mini",
	},
	"cursor": {
		"planner": "gpt-5.4",      // FR-D5: PRD tier-name "flagship" → best-guess OpenAI token; VERIFY agent --help
		"stager":  "",             // NOT stager-capable (TooledFlags nil)
		"message": "gpt-5.4-nano", // FR-D5: PRD tier-name "nano" → best-guess; VERIFY
		"arbiter": "gpt-5.4-mini", // FR-D5: PRD tier-name "mid" → best-guess; VERIFY
	},
}

// DefaultModelsForProvider returns a COPY of the named provider's role→model column from the FR-D4 table
// (PRD §9.16 FR-D4), or nil if name is not a built-in provider. The copy is defensive — callers (the
// config bootstrap, P1.M4.T2) may mutate it freely without affecting the package-level table (mirrors
// provider.BuiltinManifests' fresh-per-call discipline).
//
// The bootstrap writes the detected provider's [role.*] block from this column (FR-B1 step 3) and other
// installed providers' blocks commented (step 4). A stager value of "" means the provider cannot serve
// as the stager (nil TooledFlags) — the bootstrap applies the FR-D4 fallback (next TooledFlags-capable
// provider) on that signal. See roleDefaults' FR-D5 block for model-name provenance.
func DefaultModelsForProvider(name string) map[string]string {
	if col, ok := roleDefaults[name]; ok {
		out := make(map[string]string, len(col))
		for role, model := range col {
			out[role] = model
		}
		return out
	}
	return nil
}
```

```go
// === internal/config/role_defaults_test.go — white-box; hardcoded expectations; copy-semantics guard ===

package config

import "testing"

// expected columns — hardcoded (NOT derived from the table, so the test is meaningful).
func TestDefaultModelsForProvider_PerProvider(t *testing.T) {
	want := map[string]map[string]string{
		"pi":       {"planner": "gpt-5.4", "stager": "gpt-5.4-mini", "message": "gpt-5.4-nano", "arbiter": "gpt-5.4-mini"},
		"claude":   {"planner": "opus", "stager": "sonnet", "message": "haiku", "arbiter": "sonnet"},
		"gemini":   {"planner": "gemini-3.5-pro", "stager": "", "message": "gemini-3.1-flash-lite", "arbiter": "gemini-3.5-flash"},
		"agy":      {"planner": "gemini-3.5-pro", "stager": "", "message": "gemini-3.1-flash-lite", "arbiter": "gemini-3.5-flash"},
		"opencode": {"planner": "openai/gpt-5.4", "stager": "", "message": "openai/gpt-5.4-nano", "arbiter": "openai/gpt-5.4-mini"},
		"codex":    {"planner": "gpt-5.1-codex-max", "stager": "", "message": "gpt-5.4-nano", "arbiter": "gpt-5.1-codex-mini"},
		"cursor":   {"planner": "gpt-5.4", "stager": "", "message": "gpt-5.4-nano", "arbiter": "gpt-5.4-mini"},
	}
	for name, exp := range want {
		got := DefaultModelsForProvider(name)
		if got == nil { t.Errorf("DefaultModelsForProvider(%q) = nil, want a column", name); continue }
		for role, m := range exp {
			if got[role] != m {
				t.Errorf("DefaultModelsForProvider(%q)[%q] = %q, want %q", name, role, got[role], m)
			}
		}
	}
}

func TestDefaultModelsForProvider_StagerCapability(t *testing.T) {
	for _, capable := range []string{"pi", "claude"} {
		if m := DefaultModelsForProvider(capable)["stager"]; m == "" {
			t.Errorf("%q should be stager-capable (non-empty stager), got %q", capable, m)
		}
	}
	for _, incapable := range []string{"gemini", "agy", "opencode", "codex", "cursor"} {
		if m := DefaultModelsForProvider(incapable)["stager"]; m != "" {
			t.Errorf("%q must have stager=="" (not stager-capable), got %q", incapable, m)
		}
	}
}

func TestDefaultModelsForProvider_UnknownReturnsNil(t *testing.T) {
	if got := DefaultModelsForProvider("nonexistent"); got != nil {
		t.Errorf("DefaultModelsForProvider(nonexistent) = %v, want nil", got)
	}
}

func TestDefaultModelsForProvider_CopySemantics(t *testing.T) {
	first := DefaultModelsForProvider("pi")
	first["stager"] = "MUTATED"
	second := DefaultModelsForProvider("pi")
	if second["stager"] != "gpt-5.4-mini" {
		t.Errorf("table was mutated via returned map: second call stager = %q, want gpt-5.4-mini (must return a copy)", second["stager"])
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): change NONE. role_defaults.go needs NO imports (a map literal + lookup);
      role_defaults_test.go imports only "testing". `go mod tidy` MUST be a no-op. `git diff --exit-code
      go.mod go.sum` MUST be empty.

PACKAGE EDGES: NONE added. config stays a leaf. Do NOT import internal/provider (import cycle —
      provider/registry.go imports config). role_defaults.go is import-free.

UPSTREAM CONTRACT (the data sources — read-only, already SHIPPED):
  - internal/provider/builtin.go BuiltinManifests(): the 7 provider Names (the table's provider keys) +
        the TooledFlags per manifest (the §3 stager-capability basis — only pi, claude non-empty).
  - PRD §9.16 FR-D4 + work-item exemplars: the model NAMES (the §4 baseline).
  - internal/config/load.go roleNames: the 4 canonical role strings (the table's role keys must match).

DOWNSTREAM CONTRACTS (the consumers — do NOT implement here, just honor the accessor):
  - P1.M4.T2 (config bootstrap, config init): iterates provider.BuiltinManifests() to find installed
        providers; calls config.DefaultModelsForProvider(detectedName); writes [role.*] blocks
        (uncommented for the detected provider, commented for others); interprets stager=="" as
        "cannot be stager" and applies the FR-D4 fallback.
  - pkg/stagehand (P4.M2): may expose DefaultModelsForProvider.
  => The RoleModelDefaults type + DefaultModelsForProvider(name string) map[string]string signatures
     are FROZEN after this subtask.

DOCS: docs/providers.md gains ONE additive section ("## Per-role default models (FR-D4)"). No other doc
      files change.

FROZEN/LEAVE (do NOT edit):
  - internal/config/config.go (+_test.go), roles.go (+_test.go), load.go (+_test.go), file.go (+_test.go),
    git.go (+_test.go).
  - internal/provider/* (builtin.go/manifest.go/registry.go/render.go/merge.go + tests) — the manifests
    the table is derived FROM.
  - internal/git/*, internal/prompt/*, internal/generate/*, internal/ui/*, internal/cmd/*, pkg/*, cmd/*.
  - PRD.md, Makefile, providers/*.toml, go.mod, go.sum.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/config/role_defaults.go internal/config/role_defaults_test.go
go vet ./internal/config/
# Confirm role_defaults.go is import-free and defines the type + accessor:
grep -c '^import' internal/config/role_defaults.go   # expect 0 (import-free) OR use `head` to confirm no import block
grep -n 'type RoleModelDefaults\|func DefaultModelsForProvider' internal/config/role_defaults.go
# Confirm the FR-D5 verification block is present:
grep -n 'FR-D5\|2026-07' internal/config/role_defaults.go
# Confirm go.mod/go.sum unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; role_defaults.go has no import block; the type + accessor present; FR-D5 block present.
```

### Level 2: Config-package unit tests (the new suite + no regression)

```bash
go test ./internal/config/ -run 'DefaultModelsForProvider|RoleDefaults' -v
# Expected PASS — verify explicitly:
#   TestDefaultModelsForProvider_PerProvider ...... all 7 columns match the §4 values (incl. stager="" for 5)
#   TestDefaultModelsForProvider_StagerCapability . pi/claude non-empty stager; the 5 others stager==""
#   TestDefaultModelsForProvider_UnknownReturnsNil  unknown → nil
#   TestDefaultModelsForProvider_CopySemantics .... mutating the returned map does NOT change the table
#   TestRoleDefaults_KeySanity ................... 7 provider keys; only the 4 role keys per column
# Then the FULL config suite (no regression in Defaults/loadEnv/loadFlags/file/git/ResolveRoleModel tests):
go test ./internal/config/ -v
# Expected: all PASS — the existing tests stay green (this subtask only ADDS a file + tests; touches nothing).
# If TestDefaultModelsForProvider_StagerCapability fails for a non-capable provider, that provider got a
# stager model it shouldn't have (§3 violated — re-check builtin.go TooledFlags).
# If TestDefaultModelsForProvider_CopySemantics fails, DefaultModelsForProvider returns the live map (§7
# violated — make it return a copy).
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...     # Expect clean (purely additive: import-free table + accessor).
go test ./...      # Expect all PASS — nothing else depends on DefaultModelsForProvider yet.
# Confirm the LEAVE files are byte-unchanged:
git diff --exit-code internal/config/config.go internal/config/config_test.go internal/config/roles.go \
  internal/config/roles_test.go internal/config/load.go internal/config/load_test.go \
  internal/config/file.go internal/config/file_test.go internal/config/git.go internal/config/git_test.go \
  internal/provider internal/git internal/prompt internal/generate internal/ui \
  internal/cmd cmd pkg Makefile go.mod go.sum PRD.md providers \
  && echo "frozen files UNCHANGED (expected)"
# Confirm ONLY the 2 new files + docs/providers.md differ (nothing else):
git status --porcelain internal/config/ docs/providers.md
# Expected: two untracked files (role_defaults.go, role_defaults_test.go) + one modified (docs/providers.md);
# NO other modified/untracked files anywhere.
```

### Level 4: Correctness reasoning + the FR-D5 re-verification gate

```bash
# This subtask is a static table + accessor + docs — no server/DB/subprocess/git. Verify by reasoning + tests:
#   1. Stager capability: re-read internal/provider/builtin.go and confirm ONLY pi + claude have non-empty
#      TooledFlags. The table's stager column must match (non-empty for pi/claude, "" for the other 5).
#      [Test_StagerCapability]
#   2. Copy semantics: DefaultModelsForProvider returns a fresh map, so the package table is immutable from
#      outside. [Test_CopySemantics]
#   3. Key sanity: exactly 7 providers × 4 roles; cursor's key is "cursor" (not "agent"). [Test_KeySanity]
#   4. docs/providers.md: the new section is additive and the existing "Stager?" column is unchanged.
#
# FR-D5 RE-VERIFICATION GATE (the implementing agent's responsibility — record in the FR-D5 block):
#   For each provider, attempt to verify the current flagship/mid/fast model against the provider's live
#   docs / --help (pi --help, claude --help, gemini --help, opencode run --help, codex exec --help,
#   agent --help [cursor], agy --help). Where a CLI isn't installed/docs inaccessible, keep the PRD
#   baseline and mark "unverified — PRD §9.16 2026-07". cursor's cells are the priority (best-guess).
#   Update the FR-D5 comment block + docs table with any verified names + the verification date.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/` clean.
- [ ] `go test ./...` PASS (config suite incl. the new DefaultModelsForProvider tests + no repo-wide regression).
- [ ] go.mod/go.sum byte-unchanged.
- [ ] config.go + roles.go + load.go + file.go + git.go (+tests) + every internal/provider/* file + cmd/* +
      pkg/* + Makefile byte-unchanged; PRD.md byte-unchanged.

### Feature Validation
- [ ] `RoleModelDefaults` type + `roleDefaults` table (7×4) + `DefaultModelsForProvider` accessor exist;
      role_defaults.go is import-free.
- [ ] pi + claude have non-empty stager; gemini/agy/opencode/codex/cursor have stager=="" (matches builtin.go
      TooledFlags — ONLY pi/claude are stager-capable).
- [ ] opencode models are provider-prefixed (openai/…); pi/claude/codex/agy/gemini bare; cursor cells carry
      an "FR-D5: verify" comment.
- [ ] `DefaultModelsForProvider` returns a COPY (mutating it doesn't change the table); unknown → nil.
- [ ] Every provider column has exactly the 4 role keys; the table has exactly the 7 provider keys.
- [ ] The FR-D5 verification comment block is present (date 2026-07, source, per-provider status, capability basis).
- [ ] docs/providers.md has the additive "## Per-role default models (FR-D4)" section; existing tables unchanged.

### Code Quality Validation
- [ ] Follows existing conventions: import-free config-leaf file (mirrors the config package's leaf style);
      white-box `package config` tests mirroring `TestDefaults` (hardcoded expectations, one t.Errorf/assertion);
      doc comments cite PRD §9.16 FR-D3/D4/D5.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (LEAVE files untouched); no provider-import-cycle.

### Documentation
- [ ] role_defaults.go FR-D5 comment block records verification date + source + per-provider status + the
      stager-capability basis; cursor's cells + the FR-D5 re-verification mandate are called out.
- [ ] docs/providers.md "## Per-role default models (FR-D4)" shows the FR-D3 tier strategy + the FR-D4 table +
      the stager-fallback note + a pointer to role_defaults.go as the source of truth.

---

## Anti-Patterns to Avoid

- ❌ **Don't give a stager model to a non-capable provider.** ONLY pi + claude have non-empty TooledFlags
  (builtin.go). gemini/agy/opencode/codex/cursor MUST have stager="". Trust the CODE, not the stale PRD
  §9.16 note ("agy and opencode today" — actually 5). (§3)
- ❌ **Don't place the file in internal/provider/.** config can't import provider (cycle); the table is
  import-free; the consumer is config bootstrap. Use internal/config/role_defaults.go. (§1)
- ❌ **Don't import internal/provider (or anything).** role_defaults.go is a static map literal + lookup —
  import-free. The capability basis (TooledFlags) is read from builtin.go at AUTHORING time and encoded as
  stager=""/value; the table does not consult the live manifest. (§1/§0)
- ❌ **Don't return the internal map from DefaultModelsForProvider.** Return a COPY (defensive — mirrors
  BuiltinManifests). nil for unknown. A test pins copy semantics. (§7)
- ❌ **Don't derive the test expectations from the table.** Hardcode the expected per-provider maps (else the
  test is circular and can't catch a wrong value). (§9)
- ❌ **Don't skip the FR-D5 re-verification gate.** The model names are 2026-07 PRD baselines; the
  implementing agent MUST attempt per-provider verification (--help/docs) and record verified names + date in
  the FR-D5 block. cursor's cells are best-guesses — prioritize verifying them. (§4/§6)
- ❌ **Don't use "agent" as cursor's key.** The key is the manifest NAME ("cursor"); "agent" is the
  detect/command binary. All 7 keys = {pi, claude, gemini, opencode, codex, cursor, agy}. (§8)
- ❌ **Don't rewrite docs/providers.md.** ADD one section; don't alter the existing "Stager?" column (already
  correct). (§10)
- ❌ **Don't implement the bootstrap / stager-fallback / [role.*] writing.** Those are P1.M4.T2. This
  subtask is the TABLE + ACCESSOR + DOCS only. (§0/§11)
- ❌ **Don't forget the stager KEY for non-capable providers.** The key "stager" must be present (value "");
  don't omit it (the bootstrap iterates all 4 roles). (§8)
