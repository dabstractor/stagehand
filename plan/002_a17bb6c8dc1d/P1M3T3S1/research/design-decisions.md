# P1.M3.T3.S1 — RoleModelDefaults table + per-provider default models: design decisions

> The single source of truth for the judgment calls in this subtask. Read BEFORE implementing.
> The PRD (§9.16 FR-D4 + the work-item exemplars) is the authoritative product spec; where the PRD
> prose conflicts with the actual CODE, the CODE wins (it is the implemented source of truth), and the
> conflict is documented. Every decision is pinned by a test in `role_defaults_test.go`.

The work-item CONTRACT (item_description, point 3/5) is:

> Create `internal/config/role_defaults.go` (or `provider/role_defaults.go`). Define
> `type RoleModelDefaults map[string]map[string]string` keyed by provider→role→model. Also define
> `func DefaultModelsForProvider(name string) map[string]string` returning that provider's role→model
> column. Include the four roles: "planner", "stager", "message", "arbiter". For providers that cannot
> serve as stager (empty tooled_flags), mark the stager entry as "" and document the fallback. Add a
> comment block listing the verification date and source for each model name.
>
> DOCS: [Mode A] Add inline comments recording the model verification date per FR-D5. Update
> docs/providers.md to show the tier assignments.

---

## §0 — Scope boundary: the TABLE + ACCESSOR + DOCS, nothing else

This subtask produces THREE things:

1. `internal/config/role_defaults.go` — the `RoleModelDefaults` type, the static `roleDefaults` table
   (7 providers × 4 roles), the `DefaultModelsForProvider(name)` accessor, and the FR-D5 verification
   comment block.
2. `internal/config/role_defaults_test.go` — table-driven white-box tests.
3. `docs/providers.md` — ADD a "Per-role default models (FR-D4)" section (the tier assignments + table).

It does NOT:
- **bootstrap** a config (that is P1.M4.T2 — `config init` writes the detected provider's `[role.*]`
  block; this subtask only provides the DATA the bootstrap reads),
- **resolve the stager fallback** (pick the next TooledFlags-capable provider when the detected one
  can't be the stager — that is the bootstrap/orchestrator's job; this subtask only ENCODES `stager=""`
  as the signal),
- **call** `BuiltinManifests()` or the registry (the table is a pure static constant; it does not
  import `internal/provider`),
- **modify** `config.go`, `load.go`, `roles.go` (P1.M3.T2.S2), `file.go`, `git.go`, `builtin.go`,
  `manifest.go`, `registry.go`, or any manifest.

`DefaultModelsForProvider` is a pure map lookup (returns a copy). The table is the FR-D4 data, frozen
at authoring time, with the FR-D5 verification block recording provenance + the re-verification mandate.

---

## §1 — Package placement: `internal/config/role_defaults.go` (the contract's FIRST option)

**Decision: `internal/config/role_defaults.go`.** NOT `internal/provider/role_defaults.go`.

Reasoning:

1. **The consumer is config bootstrap (P1.M4.T2)**, which lives in the CLI/cmd layer and writes the
   detected provider's per-role models into the config file. The default-model data is CONFIG-BOOTSTRAP
   data, so it belongs in `internal/config`.
2. **Import-cycle safety.** `internal/provider/registry.go` imports `internal/config` (verified:
   `grep -rln internal/config internal/provider/` → `registry.go`). Therefore `internal/config` CANNOT
   import `internal/provider` (it would form a cycle). A pure static table needs NO imports, so it fits
   the config leaf cleanly. Putting it in `provider/` would couple config-bootstrap data to the provider
   package and force the bootstrap to import provider for config data (backwards dependency).
3. **The contract lists config FIRST** ("internal/config/role_defaults.go (or provider/role_defaults.go)")
   — config is the preferred home.
4. **Consistency with the resolution half.** `ResolveRoleModel` (P1.M3.T2.S2) lives in
   `internal/config/roles.go`; the default-MODEL table is its natural sibling (both are per-role
   config data). Co-locating them in `internal/config` keeps the role-config knowledge in one package.

`role_defaults.go` is import-free (a static `map` literal + a `map` lookup). `go mod tidy` is a no-op.

---

## §2 — The type + accessor design

```go
// RoleModelDefaults is the FR-D4 per-provider × per-role default-model table, keyed
// provider → role → model. (PRD §9.16 FR-D4.)
type RoleModelDefaults map[string]map[string]string

// roleDefaults is the compiled-in FR-D4 table (unexported; accessed via DefaultModelsForProvider).
var roleDefaults = RoleModelDefaults{ /* 7 providers × 4 roles — see §4 */ }

// DefaultModelsForProvider returns a COPY of the named provider's role→model column, or nil for an
// unknown provider. The copy is defensive — callers (the bootstrap) may mutate it freely without
// affecting the package-level table (mirrors BuiltinManifests' fresh-per-call philosophy).
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

- **`RoleModelDefaults` is EXPORTED** (the contract says "Define `type RoleModelDefaults …`"). It is the
  type of the internal `roleDefaults` var and is available for future accessors / the public API.
- **`roleDefaults` is UNEXPORTED** (package-level `var`). The sole access path is `DefaultModelsForProvider`.
  This keeps the table immutable from outside the package (callers get copies, not the live map).
- **`DefaultModelsForProvider` returns a COPY** (§7), not the internal map — defensive against mutation,
  matching `BuiltinManifests()`'s "constructed fresh on every call, no caller can corrupt" rule
  (builtin.go doc comment). Returns `nil` for an unknown provider (the bootstrap iterates
  `provider.BuiltinManifests()` keys, all 7 of which are in the table, so nil is the never-happens
  fallback for robustness).

---

## §3 — THE key decision: `stager=""` for the 5 non-stager-capable providers

The contract: *"For providers that cannot serve as stager (empty tooled_flags), mark the stager entry
as '' and document the fallback."*

**Which providers are stager-capable?** The AUTHORITATIVE source is the actual manifest code
(`internal/provider/builtin.go`), NOT the PRD §9.16 prose note. Verified by reading every manifest:

| Provider | `TooledFlags` in builtin.go | Stager-capable? |
|----------|----------------------------|-----------------|
| **pi** | `[]string{"--no-extensions", …}` (line 64) | ✅ YES |
| **claude** | `[]string{"--allowed-tools", "Bash(git:*),Read,Edit", …}` (line 108) | ✅ YES |
| **gemini** | absent (nil) | ❌ no |
| **agy** | absent — explicit `// TooledFlags: nil` comment (line 184) | ❌ no |
| **opencode** | absent (nil) | ❌ no |
| **codex** | absent (nil) | ❌ no |
| **cursor** | absent (nil) | ❌ no |

**So ONLY `pi` and `claude` are stager-capable.** The other FIVE (`gemini`, `agy`, `opencode`, `codex`,
`cursor`) lack `TooledFlags` and therefore `stager=""`.

**The PRD §9.16 note is STALE.** It says "A provider whose tooled_flags is empty (agy and opencode
today …)" — naming only TWO. But the CODE (and `docs/providers.md`'s "Stager?" column, which already
shows pi=yes, claude=yes, all others=no) shows FIVE providers lack `TooledFlags`. The discrepancy is
because P1.M2.T2.S2 only added `TooledFlags` to pi and claude; gemini/codex/cursor were never given a
tooled profile.

**Decision: TRUST THE CODE.** Encode `stager=""` for all FIVE non-capable providers (gemini, agy,
opencode, codex, cursor), and the real mid-tier model only for pi and claude. This is consistent with
(a) the contract ("empty tooled_flags ⇒ stager=''"), (b) the actual builtin.go, and (c) providers.md's
existing "Stager?" column. Do NOT give gemini/codex/cursor a stager model just because the FR-D4 table
lists one — they CANNOT serve as stager (`RenderTooled` errors on nil tooled_flags, per render.go), so
the table must reflect that with "".

**The `stager=""` is a SIGNAL, not a value.** The bootstrap (P1.M4.T2) interprets `stager==""` as
"this provider cannot be the stager; apply the FR-D4 fallback (the next-priority TooledFlags-capable
provider, per the FR-D4 note)." The fallback SELECTION is the bootstrap's job, NOT this table's. The
table only encodes "can this provider be the stager, and if so, what mid-tier model." Document this
contract in the doc comment + the per-provider inline comment for the "" entries.

**IMPORTANT — verify at implementation, do not hardcode-blindly.** The implementing agent MUST re-read
`internal/provider/builtin.go` and confirm which manifests have non-empty `TooledFlags` (the capability
basis), then set the table's stager column accordingly. If the codebase has since added `TooledFlags`
to another provider, that provider's stager entry gets its mid-tier model (not ""). The table is
authored to be "trivially refreshable" (FR-D5) — one cell per provider.

---

## §4 — The model NAMES + the FR-D5 re-verification mandate

**Source of truth for the names:** PRD §9.16 FR-D4 table + the work-item exemplars (item_description
point 1). These are internally consistent and dated 2026-07. The complete table (stager column already
adjusted per §3):

| Provider | planner (smart) | stager (mid) | message (fast) | arbiter (mid) |
|----------|-----------------|--------------|----------------|---------------|
| `pi` | `gpt-5.4` | `gpt-5.4-mini` ✅ | `gpt-5.4-nano` | `gpt-5.4-mini` |
| `opencode` | `openai/gpt-5.4` | `""` (can't) | `openai/gpt-5.4-nano` | `openai/gpt-5.4-mini` |
| `cursor` | `gpt-5.4` ⚠️ | `""` (can't) | `gpt-5.4-nano` ⚠️ | `gpt-5.4-mini` ⚠️ |
| `agy` | `gemini-3.5-pro` | `""` (can't) | `gemini-3.1-flash-lite` | `gemini-3.5-flash` |
| `gemini` | `gemini-3.5-pro` | `""` (can't) | `gemini-3.1-flash-lite` | `gemini-3.5-flash` |
| `codex` | `gpt-5.1-codex-max` | `""` (can't) | `gpt-5.4-nano` | `gpt-5.1-codex-mini` |
| `claude` | `opus` | `sonnet` ✅ | `haiku` | `sonnet` |

(✅ = stager-capable per builtin.go TooledFlags; ⚠️ = cursor's FR-D4 entries are TIER NAMES, not concrete
models — see §5.)

**FR-D5 re-verification (the load-bearing research directive).** PRD §9.16 FR-D5: *"Model lineups
change fast … The implementing agent MUST verify, per provider, the current flagship / mid / fast model
names against each provider's live docs / --help before pinning any default, and record the verified
names + verification date in the manifest source."*

**Honest limitation (research).** This PRP is authored in the PRD's 2026-07 timeline. The model names
(`gpt-5.4`, `gemini-3.5-pro`, `opus`/`sonnet`/`haiku` 4.8/5, etc.) are the PRD's product spec for that
timeline. As the research/PRP agent I CANNOT independently re-verify 2026-07 model names against
external "live docs" (they are part of the PRD's fiction; a real-world web search would return
conflicting 2024–2025 names and break the PRD's internal consistency). **Therefore the PRD FR-D4 table
+ work-item exemplars are used as the authoritative baseline**, and the FR-D5 re-verification is
delegated to the implementing agent (who operates in the same 2026-07 timeline and should attempt
verification against each provider's `--help`/docs at implementation time).

**What the implementing agent MUST do for FR-D5 (recorded in the comment block — §6):**

1. For each provider, attempt to verify the current flagship/mid/fast model names against that
   provider's live docs / `--help` (e.g. `pi --help`, `claude --help`, `gemini --help`, etc.).
2. Where verification succeeds, record the verified name + date in the comment block.
3. Where verification is not possible at implementation time (e.g. the CLI isn't installed, or the docs
   are inaccessible), record the PRD-baseline name marked "unverified — PRD §9.16 baseline 2026-07".
4. The defaults MUST be authored to be trivially refreshable (one table / one constant set per
   provider) — a future periodic-refresh process (out of scope, FR-D5) keeps them current.

The table's structure (one cell per provider×role) already satisfies "trivially refreshable."

---

## §5 — Per-provider model-string conventions (and the cursor uncertainty)

The model STRINGS are stored exactly as each provider's manifest expects them (FR-R5/FR-R5b — a model
is interpreted by its provider's manifest; the `(provider, model)` is a coupled unit for multi-provider
agents). Verified against builtin.go:

- **opencode — provider-PREFIXED model strings** (`openai/gpt-5.4`). opencode's manifest has empty
  `ProviderFlag` (provider is part of the model string — builtin.go line ~219). So opencode models carry
  the `openai/` upstream prefix. FR-D5 note: verify the upstream is still `openai` (the FR-D4 table says
  "openai/gpt-5.4").
- **pi — BARE model strings** (`gpt-5.4`). pi HAS a `ProviderFlag` (`--provider`) and a `DefaultProvider`
  (FR-D2: both `""`). The sub-provider (e.g. openrouter/openai, per the FR-D4 note "verify") is set
  SEPARATELY via `--provider`, NOT embedded in the model string. So pi models are bare. The pi sub-provider
  choice is a separate bootstrap/config concern (not this table's — the table stores the MODEL only).
- **claude — BARE ALIASES** (`opus`, `sonnet`, `haiku`). claude's `--model` accepts generation aliases;
  `sonnet` resolves to the current Sonnet (5), `opus` to Opus (4.8), `haiku` to Haiku. The manifest's
  `DefaultModel` is already `"sonnet"` (bare). So bare aliases. The "(4.8)"/"(5)" in FR-D4 are doc
  annotations of the current generation, NOT part of the model string.
- **codex — bare, mixed** (`gpt-5.1-codex-max`, `gpt-5.4-nano`). codex's planner/stager/arbiter use the
  codex-specific `-max`/`-mini` tokens; its message (fast) tier uses the generic `gpt-5.4-nano` (codex's
  cheapest text model). Per builtin.go, codex reads its model from `~/.codex/config.toml`, but the
  FR-D4 table pins explicit defaults for the bootstrap to write.
- **agy / gemini — bare Gemini-family** (`gemini-3.5-pro`, `gemini-3.5-flash`, `gemini-3.1-flash-lite`).
  Note message uses `gemini-3.1-flash-lite` (an OLDER, cheaper gen) while planner/arbiter use `gemini-3.5`
  — that is the FR-D4 spec (message = fastest/cheapest tier).

**⚠️ cursor — the ONE genuinely-uncertain provider.** The FR-D4 table gives cursor TIER NAMES, not
concrete models: planner="flagship (e.g. gpt-5.4)", stager="mid", message="nano", arbiter="mid". The
work-item exemplars (item_description point 1) do NOT list cursor. The FR-D4 note says "cursor's …
exact model tokens are resolved during implementation by reading each CLI's live model list (FR-D5)."

**Decision for cursor:** use best-guess OpenAI-tier tokens — planner=`gpt-5.4` (flagship),
message=`gpt-5.4-nano` (fast), arbiter=`gpt-5.4-mini` (mid); stager=`""` (can't, §3). Rationale: cursor
is OpenAI-backed (like opencode), so its tier→token mapping mirrors OpenAI's. BUT this is a GUESS —
prominently mark cursor's cells "FR-D5: cursor tokens are PRD tier-names (flagship/mid/nano) resolved
to best-guess OpenAI tokens; VERIFY against `agent --help` / cursor's model list at implementation."
The implementing agent MUST verify or override these.

---

## §6 — The FR-D5 verification comment block (Mode A docs, in-source)

The contract: *"Add a comment block listing the verification date and source for each model name."*
This is an IN-SOURCE comment block at the top of `role_defaults.go` (above the `roleDefaults` var),
recording:

```
// FR-D4 / FR-D5 verification block (PRD §9.16).
//
// Verification date: 2026-07
// Primary source:   PRD §9.16 FR-D4 table + work-item exemplars (P1.M3.T3.S1 item_description §1).
// FR-D5 mandate:    Model lineups change fast. The implementing agent MUST re-verify each provider's
//                   current flagship/mid/fast model names against that provider's live docs / --help
//                   and record verified names + date here. Defaults are authored trivially-refreshable
//                   (one cell per provider×role).
//
// Per-provider status (update on re-verification):
//   pi      — gpt-5.4 / gpt-5.4-mini / gpt-5.4-nano — PRD baseline 2026-07 (bare; sub-provider set
//             separately via --provider; verify pi's OpenAI-routing sub-provider, FR-D4 note).
//   opencode— openai/gpt-5.4 / -mini / -nano — PRD baseline 2026-07 (provider-prefixed; verify upstream).
//   cursor  — gpt-5.4 / gpt-5.4-mini / gpt-5.4-nano — UNVERIFIED: PRD gives tier names (flagship/mid/
//             nano); resolved to best-guess OpenAI tokens (cursor is OpenAI-backed). VERIFY `agent --help`.
//   agy     — gemini-3.5-pro / gemini-3.5-flash / gemini-3.1-flash-lite — PRD baseline 2026-07.
//   gemini  — same as agy — PRD baseline 2026-07.
//   codex   — gpt-5.1-codex-max / gpt-5.1-codex-mini / gpt-5.4-nano — PRD baseline 2026-07.
//   claude  — opus / sonnet / haiku — PRD baseline 2026-07 (bare aliases; opus=4.8, sonnet=5 per FR-D4).
//
// Stager-capability basis: a provider's stager cell is non-empty IFF its built-in manifest
// (internal/provider/builtin.go) has non-empty TooledFlags. As of 2026-07 that is ONLY pi + claude.
// gemini/agy/opencode/codex/cursor have stager="" (nil TooledFlags ⇒ RenderTooled errors ⇒ cannot be
// stager). The bootstrap (P1.M4.T2) applies the FR-D4 fallback (next TooledFlags-capable provider) on
// stager=="". VERIFY the TooledFlags state in builtin.go at implementation — if a provider has since
// gained TooledFlags, give it the mid-tier stager model.
```

This block satisfies FR-D5's "record the verified names + verification date in the manifest source" and
the contract's "comment block listing the verification date and source for each model name." Per-cell
inline comments (e.g. `// stager-capable (TooledFlags set)` / `// NOT stager-capable — fallback`) augment it.

---

## §7 — `DefaultModelsForProvider` returns a COPY (defensive)

Returning the internal `roleDefaults[name]` map directly would let a caller mutate the package-level
table (a latent bug — the next caller sees the mutation). `BuiltinManifests()` avoids this by
constructing fresh per call (builtin.go doc). `DefaultModelsForProvider` mirrors that discipline: it
returns a freshly-allocated `map[string]string` copy of the column. Cost is negligible (4 entries).

- Known provider → a 4-entry copy (planner/stager/message/arbiter).
- Unknown provider → `nil` (the bootstrap iterates `provider.BuiltinManifests()` keys, all of which are
  in the table; nil is the robustness fallback).

A test pins the copy semantics: mutate the returned map, re-call, confirm the table is unchanged.

---

## §8 — Role keys + provider keys (exact strings)

- **Role keys** (the 4 canonical roles): exactly `"planner"`, `"stager"`, `"message"`, `"arbiter"`
  (lowercase — matching `roleNames` in load.go, P1.M3.T2.S1, and FR-R1). Each provider's column has all
  four keys present (stager present even when "" — the key exists, the value is "").
- **Provider keys**: exactly the `Name` field of each built-in manifest (builtin.go): `"pi"`, `"claude"`,
  `"gemini"`, `"opencode"`, `"codex"`, `"cursor"`, `"agy"`. NOTE cursor's key is `"cursor"` (the manifest
  NAME), NOT `"agent"` (agent is the detect/command binary, ≠ name). All 7 are present.

A test asserts every provider column has exactly the 4 role keys, and the table has exactly the 7
provider keys (no typos like "codx", no missing role).

---

## §9 — Test strategy: table-driven white-box `package config`

Mirror `internal/config/config_test.go`'s style (white-box `package config`, construct directly, one
`t.Errorf` per assertion) and the table-driven idiom. Cases:

1. **Per-provider column correctness** (one sub-test per provider, or a table loop over the 7 names):
   each returns its 4-role column with the §4 values, INCLUDING stager="" for the 5 non-capable
   providers and the real mid-tier model for pi/claude.
2. **All 4 roles present** for every known provider (no missing role key — `len(col) == 4` and each of
   planner/stager/message/arbiter is a key).
3. **Stager capability matches the table**: pi+claude have non-empty stager; the other 5 have stager=="".
4. **Unknown provider → nil**: `DefaultModelsForProvider("nonexistent") == nil`.
5. **Copy semantics (defensive)**: get a column, mutate it, re-get — the second is unchanged (proves the
   table wasn't mutated). This is the §7 guard.
6. **Table key sanity**: the table has exactly the 7 built-in provider names; no provider column has a
   stray/typo role key.

Hardcode the expected values in the test (do NOT derive them from the table — else the test is circular).
The stager="" assertions (cases 1+3) are the load-bearing ones — if a non-capable provider has a
non-empty stager, §3 was violated.

NO subprocess, NO temp repo, NO registry — pure map lookups.

---

## §10 — docs/providers.md update (DOCS, Mode A)

The contract: *"Update docs/providers.md to show the tier assignments."* This is an IMPLEMENTATION task
(the PRP-research agent does not modify docs; the implementing agent does, per this spec). ADD a new
section to `docs/providers.md`:

- **Placement:** AFTER the existing "## Tooled mode and the stager role" section (currently ends ~line
  95) and BEFORE "## Adding a new agent" (line 97). Title: `## Per-role default models (FR-D4)`.
- **Content:**
  - The FR-D3 universal role→tier strategy (planner = flagship/smart, stager = mid, message = fast,
    arbiter = mid) — one line each with the one-clause rationale.
  - The FR-D4 per-provider table (the §4 table above), with stager marked "" for the 5 non-capable
    providers and a note that the stager falls back to the next TooledFlags-capable provider.
  - A sentence pointing to `internal/config/role_defaults.go` as the source of truth + the FR-D5
    re-verification mandate (models are 2026-07 baselines, re-verify per provider).
- **Do NOT** alter the existing "Stager?" column in the "7 built-in providers" table (it is already
  correct: pi/claude=yes, rest=no) — this is an ADDITIVE section.

The exact markdown is given in the PRP's Implementation Blueprint (Task 4) for the implementing agent
to paste (then adapt any verified names).

---

## §11 — Frozen files + upstream/downstream contracts

**FROZEN (do NOT edit):**
- `internal/config/config.go` (M3.T1.S1 — RoleConfig/Roles/Provider/Model), `roles.go` (P1.M3.T2.S2 —
  ResolveRoleModel), `load.go`/`load_test.go` (P1.M3.T2.S1 — roleNames), `file.go`/`git.go` (M3.T1.S2+).
- `internal/provider/*` (builtin.go/manifest.go/registry.go/render.go/merge.go — the manifests +
  TooledFlags the table is derived FROM, read-only).
- `internal/git/*`, `internal/prompt/*`, `internal/generate/*`, `cmd/*`, `pkg/*`, `Makefile`, `go.mod`,
  `go.sum`, `PRD.md`, `providers/*.toml`.

**UPSTREAM (the data sources — read-only):**
- `internal/provider/builtin.go` `BuiltinManifests()` — the 7 provider NAMES (the table's provider keys
  match these Names) + the `TooledFlags` per manifest (the §3 stager-capability basis).
- PRD §9.16 FR-D4 table + work-item exemplars — the model NAMES (the §4 baseline).

**DOWNSTREAM (the consumers — do NOT implement here):**
- `P1.M4.T2` (config bootstrap, `config init`): iterates `provider.BuiltinManifests()` to find installed
  providers, calls `config.DefaultModelsForProvider(detectedName)` to get the role→model column, writes
  the detected provider's `[role.*]` block UNCOMMENTED (FR-B1 step 3) and other installed providers'
  blocks COMMENTED (step 4). Interprets `stager==""` as "cannot be stager" and applies the FR-D4
  fallback (writes the stager block for the next TooledFlags-capable provider, annotated).
- The public API `pkg/stagehand` (P4.M2) may expose `DefaultModelsForProvider`.

The `RoleModelDefaults` type + `DefaultModelsForProvider(name string) map[string]string` signatures are
FROZEN after this subtask.
