name: "P1.M6.T1.S1 — Manifest field list_models_command across schema, built-ins, and reference TOMLs"
description: |

---

## Goal

**Feature Goal**: Add the optional `list_models_command` field (`[]string`, TOML `list_models_command`,
default empty/nil) to the provider `Manifest` struct (PRD §12.1, FR-L2) and propagate it across every
place the manifest system requires for parity: the merge layer (`MergeManifest`, slice regime), the
registry's TOML marshal surface (`providers show`), all 8 compiled-in built-ins (`builtin.go`, populated
**only** for providers whose CLI exposes a verified listing — 4 of 8), the 8 reference TOMLs
(`providers/*.toml`), and the docs field reference (`docs/providers.md`). This is the **schema-change
root** of milestone P1.M6: it adds the field and the data, NOT the `stagecoach models` consumer (that is
P1.M6.T1.S2). Empty-by-default semantics mean the field is backward-compatible and the existing
`DecodeUserOverrides` path already lets user-defined `[provider.<name>]` blocks set it for free.

**Deliverable**:
1. `internal/provider/manifest.go` (EDIT) — add `ListModelsCommand []string \`toml:"list_models_command"\``
   in the discovery block (between `Command` and `Subcommand`, matching PRD §12.1 ordering).
2. `internal/provider/merge.go` (EDIT) — add the **slice regime 2** merge line for `ListModelsCommand`
   (non-empty override REPLACES wholesale; empty/nil override preserves base — identical to
   `Subcommand`/`BareFlags`/`TooledFlags`).
3. `internal/provider/registry.go` (VERIFY, no body edit) — `MarshalTOML` already does
   `tomml.Marshal(m)` reflectively, so the new field appears in `providers show` output **for free** once
   it has a `toml` tag. Confirm via test; do NOT hand-roll marshal code.
4. `internal/provider/builtin.go` (EDIT) — set `ListModelsCommand` on the **4 verified** built-ins
   (opencode, pi, agy, cursor) with a dated `// VERIFIED <date> via '<cmd> --help' + live run` source
   comment; leave it **absent (nil)** on the other 4 (claude, codex, gemini, qwen-code).
5. `providers/{opencode,pi,agy,cursor}.toml` (EDIT) — add the `list_models_command = [...]` line (with a
   dated comment); `providers/{claude,codex,gemini,qwen-code}.toml` — OMIT the field entirely (absent in
   TOML ⇒ decodes to nil ⇒ matches the nil builtin; keeps `referencefiles_test.go` parity intact).
6. `docs/providers.md` (EDIT) — add the `list_models_command` row to the schema table (discovery section)
   + reconcile the field-count strings (see Implementation Tasks Task 6).
7. Tests (EDIT/ADD) — `merge_test.go` (slice-replace + empty-preserves for the new field),
   `registry_test.go` (assert `list_models_command` appears in `MarshalTOML` output for a populated
   builtin), `builtin_test.go` (assert the 4 populated builtins carry the expected argv + the 4 unpopulated
   are nil). `referencefiles_test.go` needs NO change (decode-parity covers the new field automatically).

**Success Definition**:
- `go build ./...`, `go test ./internal/provider/... -v`, `go vet ./...`, `golangci-lint run`, `gofmt -l`
  all green.
- `stagecoach providers show opencode` prints a TOML block containing `list_models_command =
  ['opencode', 'models']` (or `["opencode", "models"]`).
- `stagecoach providers show claude` does NOT emit `list_models_command` (the field is nil ⇒ omitted by
  go-toml in the reference TOML; in `show` output nil slices render as `[]` — see Gotchas).
- `TestProviderReferenceFiles_DecodeParity` still passes (all 8 `.toml` files decode-DeepEqual their
  builtins, including the new field) — proving the reference docs never drift from the code.
- A user-defined `[provider.myagent]` block setting `list_models_command = ["myagent", "list"]` is
  honored by the existing merge path (no config-layer change) — asserted by a merge/registry test.

## User Persona

**Target User**: the "plan-holder" (PRD §7.1) and the "multi-agent tinkerer" (§7.3) who want to see what
models a provider CLI can reach before pinning a default. Today stagecoach has no model-discovery surface;
this field is the data foundation for `stagecoach models` (S2) and the `config init --interactive` wizard
(P1.M6.T2.S1), both of which consume `Manifest.ListModelsCommand`.

**Use Case** (enabling, not delivered here): `stagecoach models opencode` (S2) reads
`manifest.ListModelsCommand`, runs `["opencode", "models"]`, and prints the CLI's own model list under a
heading — **never an HTTP call** (PRD §6.2 N2 / FR-L1), because stagecoach has no API key and the agent CLI
is the only model authority. S1 makes that argv available; S2 wires the command.

**Pain Points Addressed**: incumbents (aicommits/opencommit) list models by hitting provider HTTP APIs
with the user's key — a key stagecoach refuses to require. `list_models_command` routes discovery through
the agent CLI the user already has installed, sidestepping the key entirely.

## Why

- **FR-L2 (PRD §9.23 / §12.1)**: an optional argv array in the provider manifest, e.g.
  `["opencode", "models"]`, empty by default, "Populated at implementation time only for providers whose
  CLI actually exposes a listing (verified per FR-D5, recorded with date)."
- **FR-L1**: `stagecoach models` source-of-truth order — (a) run `list_models_command`, print stdout; (b)
  if absent/fails, print the curated FR-D4 tier table. So an empty field is a FIRST-CLASS, EXPECTED state
  (graceful fallback), not a gap.
- **N2 (PRD §6.2)**: never an HTTP call — the field's documented semantics.
- **architecture/system_context.md §3**: "`list_models_command` does NOT exist — the one new §12.1 field";
  and "any Manifest field addition must touch all three [builtin.go + providers/*.toml] + the merge +
  MarshalTOML" (the parity surface asserted by `referencefiles_test.go`).
- **architecture/external_deps.md §9**: "Known-good case: opencode exposes `opencode models`. Other
  providers must be checked against their live `--help` at implementation time; populate the manifest
  field ONLY where verified." (Live verification found 4, not 1 — see research.)
- **Scope fences**: S1 CONSUMES the existing manifest/merge/registry/builtin/TOML/doc surface (adds one
  field across it) and PROVIDES `Manifest.ListModelsCommand` for S2. S1 does NOT implement `stagecoach
  models` (S2), `config init --interactive` (P1.M6.T2.S1), or any CLI flag/config key (the field is set
  per-provider in manifests, not via the 5-layer config resolver). The change is additive + nil-default ⇒
  byte-identical behavior for every code path that does not read the new field (which is all of them
  today — no consumer exists until S2).

## What

A new optional `[]string` field on `Manifest`, merged like the other slice fields (wholesale replace on
non-empty override), populated for the 4 CLIs verified to expose a listing, absent for the other 4, and
surfaced in `providers show` + the docs. No new behavior ships beyond the data — S2 reads it.

### Success Criteria

- [ ] `manifest.go`: `ListModelsCommand []string \`toml:"list_models_command"\`` added in the discovery
      block (between `Command` and `Subcommand`), with a doc comment stating nil/empty ⇒ no listing
      command (FR-L1 falls back to FR-D4 curated table), populated only for verified providers (FR-D5),
      NEVER an HTTP call (§6.2 N2).
- [ ] `merge.go`: slice regime 2 line added — `if len(override.ListModelsCommand) > 0 { out.ListModelsCommand = override.ListModelsCommand }` — grouped with the other slice-replace lines.
- [ ] `registry.go`: `MarshalTOML` confirmed to emit the field via `toml.Marshal(m)` (no body edit); a
      test asserts `list_models_command` is present in `providers show opencode` output.
- [ ] `builtin.go`: 4 builtins populated with the EXACT argv + dated comment; 4 left absent.
      opencode=`["opencode","models"]`, pi=`["pi","--list-models"]`, agy=`["agy","models"]`,
      cursor=`["agent","models"]`.
- [ ] `providers/*.toml`: the 4 populated providers carry `list_models_command = [...]`; the 4 unpopulated
      OMIT the line (absent ⇒ nil ⇒ decode-parity holds).
- [ ] `docs/providers.md`: schema table has the `list_models_command` row + reconciled field count.
- [ ] `TestProviderReferenceFiles_DecodeParity` passes unchanged (the parity guard covers the new field).
- [ ] All build/test/vet/lint/fmt green.

## All Needed Context

### Context Completeness Check

_This PRP names the EXACT struct field + toml tag + placement line; the EXACT merge line + regime; the
proof that MarshalTOML needs no body edit (reflective `toml.Marshal`); the EXACT 4 argv values to populate
(with the two non-obvious gotchas — pi is a FLAG `--list-models`, cursor's binary is `agent`); the EXACT
files to touch; the proof that `referencefiles_test.go` and `DecodeUserOverrides` need NO change; the
field-count reconciliation for docs; and the live-verification evidence (2026-07-03) with the re-verify
mandate (FR-D5). An implementer with no prior codebase knowledge can build it from this document +
codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/P1M6T1S1/research/listing-cli-verification.md
  why: THE verified-argv table for the 8 built-ins. Live `--help` + executed-command evidence per
       provider (exit codes, sample output). 4 POPULATE (opencode/pi/agy/cursor), 4 LEAVE EMPTY
       (claude/codex/gemini/qwen-code). The two argv gotchas: pi is `["pi","--list-models"]` (a FLAG),
       cursor is `["agent","models"]` (binary=agent, ≠ name).
  section: Results + Contract delta + argv form gotchas
  critical: |
    The work-item contract's "expected: opencode only" was based on external_deps.md §9's PARTIAL
    knowledge. In-task live verification (mandated by the contract) found 3 more. Populate all 4
    verified; leave the rest empty. Re-confirm each at implementation time and stamp the date (FR-D5).

- file: internal/provider/manifest.go
  why: THE struct to extend. The field is a SLICE ([]string) → follows the slice fields' design
       (Subcommand/BareFlags/TooledFlags), NOT the *string/*bool pointer regime. It does NOT go in
       Resolve() — slices are "left as-is (nil stays nil)" with no default to fill. Add the toml tag.
  pattern: the discovery block `// --- discovery (§12.1) ---` with Name/Detect/Command/Subcommand.
           Insert ListModelsCommand between Command and Subcommand (PRD §12.1 ordering).
  gotcha: |
    Do NOT make it a pointer or add it to Resolve(). A nil slice is the natural "absent" sentinel
    (the comment on the struct explains why slices stay plain). An empty/nil ListModelsCommand is the
    EXPECTED state for 4 of 8 providers — FR-L1's fallback handles it. Validate() needs NO new case
    (a free-form argv array has no enum to check).

- file: internal/provider/merge.go
  why: THE merge to extend. ListModelsCommand is regime 2 (slice — non-empty override REPLACES base
       wholesale; empty/nil override preserves base). Mirror the Subcommand/BareFlags/TooledFlags block.
  pattern: |
       // --- regime 2: slices — non-empty override REPLACES wholesale (no element merge) ---
       if len(override.Subcommand) > 0 {
           out.Subcommand = override.Subcommand
       }
  gotcha: Do NOT add it to the Env/ReasoningLevels map-regime block — it is a slice, not a map.

- file: internal/provider/registry.go
  why: MarshalTOML (the `providers show` source, FR47). It does `toml.Marshal(m)` — go-toml/v2 marshals
       every struct field with a toml tag reflectively, so the new field appears AUTOMATICALLY. NO body
       edit is needed; the contract's "extend MarshalTOML" is satisfied by the struct-field addition.
  pattern: the existing `MarshalTOML` body — `data, err := toml.Marshal(m)`.
  gotcha: |
    go-toml marshals a NIL []string as `key = []` (the TestMarshalTOML_RoundTrip comment notes "nil
    slices marshal as `[]`"). So `providers show claude` will print `list_models_command = []` (display-
    only; harmless). The REFERENCE .toml files instead OMIT the key for nil providers so decode-parity
    holds (absent in TOML ⇒ decodes to nil ⇒ DeepEquals the nil builtin). Do not "fix" the `[]` in show
    output — it is consistent with how nil `subcommand` already behaves for pi.

- file: internal/provider/builtin.go
  why: THE 8 built-in manifests. Add ListModelsCommand to the 4 verified; leave absent on the other 4.
       Follow the existing FR-D5 date-stamp comment pattern (see builtinGemini's
       `// WAS gemini-2.5-pro — refreshed per FR-D5 (verified 2026-07-02)`).
  pattern: a field set with a dated verification comment, e.g.
       ListModelsCommand: []string{"opencode", "models"}, // VERIFIED 2026-07-03 via `opencode models` (exit 0); FR-L2/FR-D5.
  gotcha: |
    (1) pi argv = ["pi","--list-models"] — a FLAG, NOT ["pi","models"]. (2) cursor argv = ["agent",
    "models"] — the binary is `agent` (Detect/Command="agent"), not `cursor`. (3) The manifests are
    constructed FRESH per call (strPtr/slice literals) — do not share a package-level slice var. (4) For
    the 4 unpopulated builtins, OMIT the field (nil) — do NOT write `ListModelsCommand: []string{}`; a
    non-nil empty slice would break decode-parity with an absent-in-TOML key.

- file: internal/provider/referencefiles_test.go
  why: THE parity guard — reads each providers/*.toml, decodes, DeepEquals the builtin. Adding the field
       to a builtin REQUIRES the matching .toml to carry the same value (or both be nil). NO test edit.
  pattern: the `providerFiles` slice + `TestProviderReferenceFiles_DecodeParity` + the
           `_AllBuiltinsCovered` guard.
  gotcha: |
    NO CODE CHANGE here — the existing tests cover the new field automatically. But you MUST keep the 8
    .toml files in lock-step with builtin.go (the test will fail loudly if they drift). For a nil builtin
    the .toml must OMIT the key; for a populated builtin the .toml must carry the identical argv.

- file: providers/opencode.toml  (and pi.toml, agy.toml, cursor.toml)
  why: the 4 reference TOMLs to EDIT — add `list_models_command = [...]` with a dated comment, placed in
       the discovery section (after `command`, before `subcommand`), matching PRD §12.1 ordering.
  pattern: pi.toml's `# --- identity / discovery ---` block + the dated comment style
       (`# FR-D2: empty in the shipped default; ...`).
  gotcha: |
    For the 4 UNPOPULATED providers (claude/codex/gemini/qwen-code.toml) — OMIT the line entirely and add
    `list_models_command` to that file's "absent fields" comment list (e.g. opencode.toml's
    `# prompt_flag, json_field, ... are NOT set ... therefore omitted above`). Absent ⇒ nil ⇒ parity.

- file: docs/providers.md
  why: THE field reference (Mode A docs). Add the `list_models_command` row to the schema table + fix the
       field-count strings (intro says "19-field schema"; schema section says "18 fields" — both stale vs
       the actual struct; after this addition the struct has 21 fields).
  pattern: the schema table rows (`| field | type | default | purpose |`) + the `## The schema` intro.
  gotcha: |
    The row's "purpose" MUST state the never-an-HTTP-call semantics (N2) + empty-by-default + verified-
    only population. Place the row in discovery order (after `command`, before `subcommand`). Reconcile
    BOTH count strings to the accurate total (count the struct fields in manifest.go).

- url: PRD §12.1 (in plan/005_c38aa48290f0/prd_snapshot.md, heading "12.1 The manifest schema")
  why: the authoritative schema — `list_models_command = []` sits between `command` and `subcommand`,
       documented as "Optional argv that asks the AGENT CLI to list its reachable models … NEVER an HTTP
       call (§6.2 N2)."
  section: the discovery block of the example manifest.
- url: PRD §9.23 FR-L1/L2 (prd_snapshot.md)
  why: the feature contract — FR-L2 defines the field; FR-L1 defines the consumer's source-of-truth
       order (run the command → else fall back to the FR-D4 curated table).
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/
  manifest.go            # Manifest struct + Validate + Resolve (+ strPtr/boolPtr helpers)
  merge.go               # MergeManifest — 3 regimes (scalar ptr / slice / map)
  registry.go            # Registry + MarshalTOML (toml.Marshal) + DecodeUserOverrides + DefaultProvider ...
  builtin.go             # BuiltinManifests() → 8 builtins (builtinPi/Claude/Gemini/OpenCode/Codex/Cursor/Agy/QwenCode)
  referencefiles_test.go # decode-parity guard (NO edit needed)
  merge_test.go          # merge regime tests (EDIT — add ListModelsCommand slice tests)
  registry_test.go       # MarshalTOML round-trip + reflect-merge tests (EDIT — assert field in show)
  builtin_test.go        # builtin shape tests (EDIT — assert the 4 populated argv)
providers/
  pi.toml claude.toml gemini.toml opencode.toml codex.toml cursor.toml agy.toml qwen-code.toml
docs/
  providers.md           # schema field reference (EDIT — add row + reconcile count)
```

### Desired Codebase tree with files to be added/edited (no NEW files)

```bash
internal/provider/manifest.go       # EDIT — +ListModelsCommand field (slice, discovery block)
internal/provider/merge.go          # EDIT — +slice regime 2 line
internal/provider/registry.go       # NO body edit (reflective marshal covers it; verify via test)
internal/provider/builtin.go        # EDIT — populate 4 builtins + dated comments
internal/provider/referencefiles_test.go  # NO edit (parity covers the field)
internal/provider/merge_test.go     # EDIT — +ListModelsCommand slice-replace + empty-preserves tests
internal/provider/registry_test.go  # EDIT — +assert list_models_command in MarshalTOML("opencode")
internal/provider/builtin_test.go   # EDIT — +assert 4 populated argv + 4 nil
providers/{opencode,pi,agy,cursor}.toml  # EDIT — +list_models_command = [...] (dated)
providers/{claude,codex,gemini,qwen-code}.toml  # EDIT — +absent-fields comment line (key OMITTED)
docs/providers.md                   # EDIT — +schema row + reconciled field count
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (go-toml/v2): nil slices marshal as `key = []` in `providers show` (display-only, harmless —
//   matches how nil `subcommand` already renders for pi). But the REFERENCE .toml files OMIT nil keys
//   entirely so decode-parity holds (absent ⇒ decodes to nil ⇒ DeepEquals the nil builtin). Do NOT add
//   `list_models_command = []` to the 4 unpopulated providers/*.toml — that would decode to a NON-NIL
//   empty slice and BREAK TestProviderReferenceFiles_DecodeParity.

// CRITICAL (slice regime): ListModelsCommand is regime 2 (wholesale replace on non-empty override),
//   identical to Subcommand/BareFlags/TooledFlags. An empty override slice is "not overridden" (base
//   preserved) — so a user `[provider.opencode]` block that omits the field inherits the builtin argv.
//   Do NOT put it in Resolve() (slices have no default to fill) and do NOT make it a pointer.

// CRITICAL (argv form): the field is the FULL argv (binary + args), per PRD §12.1 `["opencode","models"]`.
//   pi is a FLAG → ["pi","--list-models"] (NOT ["pi","models"]). cursor's binary is `agent` →
//   ["agent","models"] (NOT ["cursor","models"]). S2 will run it as exec.Command(argv[0], argv[1:]...).

// CRITICAL (MarshalTOML needs NO body edit): it calls toml.Marshal(m) reflectively — every tagged field
//   is emitted automatically. The contract's "extend MarshalTOML" is satisfied by adding the struct
//   field + toml tag. Do not hand-roll a marshal branch (it would diverge from the reflective path).

// FR-D5: model CLIs iterate monthly. The 4 populated argv were VERIFIED 2026-07-03 via live --help +
//   executed command (see research/listing-cli-verification.md). RE-CONFIRM each at implementation time
//   and stamp the date in the source comment. If a CLI dropped/renamed its listing command, empty the
//   field (FR-L1's fallback covers it) rather than shipping a stale argv.
```

## Implementation Blueprint

### Data models and structure

The only data model change is one new field on the existing `Manifest` struct. There are no new types,
no ORM, no migrations — Go struct + TOML tag.

```go
// In internal/provider/manifest.go, Manifest struct, the `// --- discovery (§12.1) ---` block.
// Insert ListModelsCommand between Command and Subcommand (PRD §12.1 ordering):
//
//	// --- discovery (§12.1) ---
//	Name              string   `toml:"name"`              // REQUIRED; identity from the [provider.<name>] key.
//	Detect            *string  `toml:"detect"`            // nil/"" => DetectCommand falls back to Command.
//	Command           *string  `toml:"command"`           // REQUIRED (post-merge); nil in a partial override => inherit.
//	ListModelsCommand []string `toml:"list_models_command"` // nil/empty => no listing (FR-L1 falls back to FR-D4 table); populated ONLY for verified CLIs (FR-D5); NEVER an HTTP call (§6.2 N2).
//	Subcommand        []string `toml:"subcommand"`        // nil => none; inserted between command and flags.
//
// REGIME: slice (like Subcommand/BareFlags/TooledFlags) — nil is the natural "absent" sentinel. NOT a
// pointer, NOT in Resolve() (slices have no default to fill), NOT in Validate() (free-form argv, no enum).
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/provider/manifest.go — add the field
  - IMPLEMENT: `ListModelsCommand []string \`toml:"list_models_command"\`` in the discovery block,
    between `Command` and `Subcommand`.
  - FOLLOW pattern: the existing slice fields (Subcommand/BareFlags/TooledFlags) — plain []string,
    nil-as-absent. The struct's top doc-comment already explains the slice regime; you may add a
    one-line note that list_models_command follows it.
  - NAMING: Go `ListModelsCommand` (Exported, CamelCase), TOML `list_models_command` (snake_case tag).
  - PLACEMENT: discovery block, after Command, before Subcommand (PRD §12.1 ordering).
  - DO NOT: add to Resolve() (slices left as-is), Validate() (no enum), or make it a *[]string pointer.
  - DEPENDENCIES: none (this is the root change).

Task 2: EDIT internal/provider/merge.go — wire the slice merge (regime 2)
  - IMPLEMENT: in the `// --- regime 2: slices ---` block, add:
        if len(override.ListModelsCommand) > 0 {
            out.ListModelsCommand = override.ListModelsCommand
        }
  - FOLLOW pattern: the Subcommand/BareFlags/TooledFlags lines directly above/below (wholesale replace
    on non-empty override; empty/nil preserves base).
  - DO NOT: put it in the Env/ReasoningLevels map-regime block (it is a slice, not a map). Do NOT
    element-merge (slices replace wholesale by design).
  - DEPENDENCIES: Task 1 (the field must exist).

Task 3: VERIFY internal/provider/registry.go — MarshalTOML (NO body edit)
  - CONFIRM: `MarshalTOML` does `toml.Marshal(m)`; go-toml/v2 reflectively marshals every tagged field,
    so `list_models_command` appears in `providers show` output for free once Task 1 lands.
  - DO NOT: edit the MarshalTOML body or hand-roll a marshal branch (would diverge from the reflective
    path the other 20 fields use).
  - DEPENDENCIES: Task 1. Verification is via the Task 8 registry test.

Task 4: EDIT internal/provider/builtin.go — populate the 4 verified builtins
  - IMPLEMENT: add `ListModelsCommand: []string{...}` to builtinOpenCode/builtinPi/builtinAgy/builtinCursor
    with a dated verification comment. Leave builtinClaude/builtinGemini/builtinCodex/builtinQwenCode
    UNTOUCHED (field absent ⇒ nil).
  - VALUES (from research/listing-cli-verification.md, VERIFIED 2026-07-03):
      opencode: []string{"opencode", "models"}      // subcommand
      pi:       []string{"pi", "--list-models"}      // FLAG form (NOT "models")
      agy:      []string{"agy", "models"}            // subcommand
      cursor:   []string{"agent", "models"}          // binary is `agent` (≠ name "cursor"); requires auth — note in comment
  - FOLLOW pattern: the existing FR-D5 date-stamp comments, e.g. builtinGemini's
    `DefaultModel: strPtr("gemini-3.1-pro"), // WAS gemini-2.5-pro — refreshed per FR-D5 (verified 2026-07-02)`.
    Each new line: `// VERIFIED 2026-07-03 via '<cmd>' (exit 0); FR-L2/FR-D5. Re-confirm at refresh.`
  - NAMING/PLACEMENT: place the field in each manifest's discovery area (near Detect/Command/Subcommand),
    matching the struct ordering. Keep the slice literal inline (manifests are constructed fresh per call).
  - DO NOT: write `ListModelsCommand: []string{}` for the 4 unpopulated (non-nil empty breaks parity).
  - DEPENDENCIES: Task 1.

Task 5: EDIT providers/*.toml — keep the 8 reference files in lock-step with builtin.go
  - IMPLEMENT: add `list_models_command = ["...", "..."]` (with a dated comment) to opencode.toml,
    pi.toml, agy.toml, cursor.toml — SAME argv as Task 4, placed in the discovery section (after
    `command`, before `subcommand`), matching PRD §12.1 ordering.
  - IMPLEMENT: for claude.toml, codex.toml, gemini.toml, qwen-code.toml — OMIT the key entirely and add
    `list_models_command` to that file's "absent fields" comment list (mirror opencode.toml's
    `# prompt_flag, json_field, retry_instruction, and [env] are NOT set ... therefore omitted above`).
  - FOLLOW pattern: pi.toml/opencode.toml discovery section + dated comment style.
  - GOTCHA: go-toml array literal syntax — `list_models_command = ["opencode", "models"]` (double quotes,
    comma-separated). Verify by re-running `TestProviderReferenceFiles_DecodeParity` after each file.
  - DEPENDENCIES: Task 4 (values must match builtin.go byte-for-byte).

Task 6: EDIT docs/providers.md — field reference (Mode A)
  - IMPLEMENT: add a row to the schema table (discovery section, after `command`, before `subcommand`):
      | `list_models_command` | list of string | `[]` (none) | Full argv that asks the agent CLI to list its
        reachable models (e.g. `["opencode", "models"]`), used by `stagecoach models`. Empty/nil ⇒ stagecoach
        prints its curated per-role tier table instead (FR-L1). Populated only for providers whose CLI
        exposes a verified listing (opencode, pi, agy, cursor); never an HTTP call (§6.2 N2). |
  - IMPLEMENT: reconcile the field-count strings — the `## What a manifest is` intro says "the 19-field
    schema" and `## The schema` says "Each manifest has 18 fields"; both are stale. Count the struct
    fields in manifest.go (20 today) + this addition (1) = 21. Update BOTH strings to "21".
  - FOLLOW pattern: the existing table rows (4 columns: field/type/default/purpose).
  - DEPENDENCIES: Tasks 1–5 (docs reflect the final schema).

Task 7: EDIT internal/provider/merge_test.go — slice regime tests for the new field
  - IMPLEMENT: add to `TestMergeManifest_NonEmptySliceReplacesWholesale` (or a focused new test) an
    assertion that a non-empty `override.ListModelsCommand` REPLACES base wholesale; and to
    `TestMergeManifest_EmptyOrNilSlicePreservesBase` that a nil/empty override preserves base's value.
  - FOLLOW pattern: the existing BareFlags/Subcommand/TooledFlags assertions in those two tests
    (reflect.DeepEqual checks).
  - NAMING: reuse the existing test functions (add assertions) OR add `TestMergeManifest_ListModelsCommandReplacesWholesale`.
  - DEPENDENCIES: Task 2.

Task 8: EDIT internal/provider/registry_test.go — assert the field surfaces in `providers show`
  - IMPLEMENT: add a test (or extend `TestMarshalTOML_ReflectsMerge` / `TestMarshalTOML_RoundTrip`) that
    calls `r.MarshalTOML("opencode")`, re-decodes the TOML, and asserts `decoded.ListModelsCommand`
    equals `[]string{"opencode","models"}`. This proves Task 3 (MarshalTOML emits the field).
  - FOLLOW pattern: `TestMarshalTOML_ReflectsMerge` (marshal → unmarshal → field assertion, avoids
    string-search). Also assert `MarshalTOML("claude")` re-decodes to a nil ListModelsCommand.
  - DEPENDENCIES: Tasks 3–5.

Task 9: EDIT internal/provider/builtin_test.go — assert the 4 populated + 4 nil
  - IMPLEMENT: add a test asserting BuiltinManifests() carries the expected argv for opencode/pi/agy/cursor
    and a nil ListModelsCommand for claude/codex/gemini/qwen-code.
  - FOLLOW pattern: the existing builtin-shape assertions in builtin_test.go.
  - DEPENDENCIES: Task 4.
```

### Implementation Patterns & Key Details

```go
// === manifest.go: the one field (discovery block) ===
// Insert between Command and Subcommand:
ListModelsCommand []string `toml:"list_models_command"` // FR-L2: optional argv to list the CLI's models; nil/empty => FR-L1 curated-table fallback; verified-only (FR-D5); never HTTP (§6.2 N2).

// === merge.go: regime 2 (slice — wholesale replace on non-empty override) ===
// In the `// --- regime 2: slices ---` block, alongside Subcommand/BareFlags/TooledFlags:
if len(override.ListModelsCommand) > 0 {
	out.ListModelsCommand = override.ListModelsCommand
}

// === builtin.go: the 4 populated values (dated, FR-D5) ===
// builtinOpenCode():
ListModelsCommand: []string{"opencode", "models"}, // VERIFIED 2026-07-03 via `opencode models` (exit 0); FR-L2/FR-D5.
// builtinPi():
ListModelsCommand: []string{"pi", "--list-models"}, // VERIFIED 2026-07-03 via `pi --list-models` (exit 0); FLAG form, not a subcommand. FR-L2/FR-D5.
// builtinAgy():
ListModelsCommand: []string{"agy", "models"}, // VERIFIED 2026-07-03 via `agy models` (exit 0); FR-L2/FR-D5.
// builtinCursor():
ListModelsCommand: []string{"agent", "models"}, // VERIFIED 2026-07-03: `agent --help` lists `models`; live run exits 1 (auth required) — valid for authed users, FR-L1 fallback covers unauthed. Binary is `agent` (≠ name). FR-L2/FR-D5.
// builtinClaude/Gemini/Codex/QwenCode: OMIT (nil) — no verified listing surface.

// === providers/opencode.toml: discovery section ===
command = "opencode"
list_models_command = ["opencode", "models"]   # VERIFIED 2026-07-03 via `opencode models` (exit 0); FR-L2/FR-D5. Never an HTTP call (§6.2 N2).
subcommand = ["run"]

// === providers/claude.toml (and codex/gemini/qwen-code): absent-fields comment ===
# list_models_command, prompt_flag, json_field, ... are NOT set for claude and therefore omitted above.
```

### Integration Points

```yaml
MANIFEST SCHEMA (manifest.go):
  - field: "ListModelsCommand []string (toml:\"list_models_command\") in the discovery block"
  - default: "nil (no listing command — FR-L1 curated-table fallback)"
  - regime: "slice regime 2 in MergeManifest (wholesale replace on non-empty override)"

REGISTRY (registry.go):
  - MarshalTOML: "NO body edit — toml.Marshal(m) emits the field reflectively (verify via test)"

BUILT-INS (builtin.go):
  - populated: "opencode, pi, agy, cursor (4 — verified 2026-07-03)"
  - absent: "claude, codex, gemini, qwen-code (4 — no verified listing)"

REFERENCE TOMLS (providers/*.toml):
  - populated: "opencode/pi/agy/cursor.toml carry the key"
  - absent: "claude/codex/gemini/qwen-code.toml OMIT the key (absent ⇒ nil ⇒ decode-parity)"

CONFIG (NO change):
  - user overrides: "[provider.<name>] blocks set list_models_command via the EXISTING DecodeUserOverrides
    → MergeManifest path (slice regime 2) — free, no config-layer edit"

DOCS (docs/providers.md):
  - schema table: "+list_models_command row (discovery section); reconcile field-count strings to 21"

DOWNSTREAM CONSUMERS (NOT this task — do not implement):
  - P1.M6.T1.S2: "stagecoach models [<provider>] reads Manifest.ListModelsCommand, runs it, prints stdout (FR-L1)"
  - P1.M6.T2.S1: "config init --interactive wizard may surface the listing"
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After each file edit — fix before proceeding.
gofmt -w internal/provider/manifest.go internal/provider/merge.go internal/provider/builtin.go \
  internal/provider/merge_test.go internal/provider/registry_test.go internal/provider/builtin_test.go
go vet ./internal/provider/...
golangci-lint run ./internal/provider/...

# Expected: zero errors. gofmt -l (check mode) prints nothing for edited files.
gofmt -l internal/provider/
```

### Level 2: Unit Tests (Component Validation)

```bash
# The parity guard — MUST pass unchanged (proves the 8 .toml files match builtin.go, new field included).
go test ./internal/provider/ -run TestProviderReferenceFiles -v

# The merge regime + MarshalTOML + builtin-shape tests (Tasks 7/8/9).
go test ./internal/provider/ -run 'TestMergeManifest|TestMarshalTOML|TestBuiltin' -v

# Full provider package.
go test ./internal/provider/... -v

# Expected: all pass. If TestProviderReferenceFiles_DecodeParity fails, a .toml drifted from builtin.go
# (a populated builtin missing the key in its .toml, or a nil builtin with a spurious `= []`). Fix the
# .toml, NOT the test.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary.
go build ./...

# `providers show` surfaces the field for a populated provider (FR47).
go run . providers show opencode | grep -i list_models_command
# Expected: a line like `list_models_command = ["opencode", "models"]`.

# `providers show` for an unpopulated provider renders nil as `[]` (display-only; harmless — matches
# how nil `subcommand` already renders for pi). This is NOT a bug.
go run . providers show claude | grep -i list_models_command
# Expected: `list_models_command = []` (go-toml nil-slice rendering).

# `providers list` still works (no regression).
go run . providers list

# Expected: all commands succeed; opencode's show output carries the populated argv.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Live CLI re-confirmation (FR-D5) — re-run each populated command and confirm it still lists models.
opencode models            | head -3   # exit 0, model list
pi --list-models           | head -3   # exit 0, model table
agy models                 | head -3   # exit 0, Gemini model list
agent models 2>&1          | head -3   # exit 1 (auth) — recognized subcommand; valid for authed users

# Confirm the unpopulated CLIs still have NO listing surface (defend against shipping a stale empty).
claude --help 2>&1 | grep -iE 'models' | head    # only --model/--fallback-model (selection, not listing)
codex --help 2>&1  | grep -iE 'models' | head    # only -m/--model
gemini --help 2>&1 | grep -iE 'models' | head    # only `gemini gemma` (local routing) + -m/--model

# Expected: the 4 populated commands behave as documented; the 4 unpopulated have no listing surface.
# If a CLI CHANGED since the 2026-07-03 verification, update the argv + date (or empty it) per FR-D5.
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed.
- [ ] `go build ./...` succeeds.
- [ ] `go test ./internal/provider/... -v` — all pass, incl. `TestProviderReferenceFiles_DecodeParity`.
- [ ] `go vet ./...` clean.
- [ ] `golangci-lint run` clean.
- [ ] `gofmt -l internal/provider/` prints nothing.

### Feature Validation

- [ ] `manifest.go` has `ListModelsCommand []string` (discovery block, toml tag).
- [ ] `merge.go` has the slice regime 2 line for ListModelsCommand.
- [ ] `MarshalTOML("opencode")` output contains `list_models_command = ["opencode", "models"]`.
- [ ] `builtin.go` populates opencode/pi/agy/cursor with the exact argv + dated comments.
- [ ] `builtin.go` leaves claude/codex/gemini/qwen-code nil (field absent).
- [ ] The 8 `providers/*.toml` files match `builtin.go` (decode-parity holds).
- [ ] `docs/providers.md` has the schema row + reconciled field count.
- [ ] A user `[provider.x] list_models_command = [...]` override is honored by the existing merge path.

### Code Quality Validation

- [ ] Follows the existing slice-field conventions (Subcommand/BareFlags/TooledFlags) — no new pattern.
- [ ] Field placement matches PRD §12.1 ordering (discovery block, after command, before subcommand).
- [ ] Dated verification comments follow the FR-D5 pattern already in builtin.go.
- [ ] No new files; no new dependencies; no config-layer changes; no consumer code (S2 owns that).

### Documentation & Deployment

- [ ] `docs/providers.md` row states the never-an-HTTP-call semantics + empty-by-default + verified-only.
- [ ] Source comments record the verification date (2026-07-03) + the re-confirm mandate (FR-D5).

---

## Anti-Patterns to Avoid

- ❌ Don't make `ListModelsCommand` a `*[]string` pointer or add it to `Resolve()` — slices use
  nil-as-absent; there is no default to fill (unlike the `*string`/`*bool` scalar fields).
- ❌ Don't hand-roll a marshal branch in `MarshalTOML` — `toml.Marshal(m)` is reflective; adding the field
  + tag is the entire change. A hand-rolled branch would diverge from the path the other 20 fields use.
- ❌ Don't write `ListModelsCommand: []string{}` for the 4 unpopulated builtins — a non-nil empty slice
  breaks decode-parity with the absent-in-TOML key. OMIT the field (nil) for them.
- ❌ Don't add `list_models_command = []` to the 4 unpopulated `providers/*.toml` — same parity break.
  OMIT the key; note it in the absent-fields comment.
- ❌ Don't use `["pi", "models"]` for pi — pi's listing is a FLAG: `["pi", "--list-models"]`.
- ❌ Don't use `["cursor", "models"]` for cursor — the binary is `agent`: `["agent", "models"]`.
- ❌ Don't implement the `stagecoach models` command, a CLI flag, or a config key — those are S2 / out of
  scope. S1 is schema + data + docs only.
- ❌ Don't skip the live re-confirmation (FR-D5) — model CLIs change; stamp the date, and empty the field
  if a CLI dropped its listing rather than shipping a stale argv.
- ❌ Don't touch `referencefiles_test.go`, `DecodeUserOverrides`, or the config resolver — the parity test
  covers the new field for free, and user overrides flow through the existing merge path unchanged.
