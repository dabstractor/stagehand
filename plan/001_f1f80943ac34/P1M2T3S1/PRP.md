---
name: "P1.M2.T3.S1 — Provider Registry: built-in + user-override merge, $PATH detection, TOML marshal"
description: |
  Land the SOLE subtask of Provider Registry (P1.M2.T3): `internal/provider/registry.go` — a `Registry`
  struct (holding `map[string]Manifest`) that seeds from `BuiltinManifests()` (P1.M2.T2, the 6-agent
  set: pi/claude/gemini/opencode/codex/cursor — S3 lands in parallel) and overlays user overrides via
  `MergeManifest` (P1.M2.T1.S2) per PRD §16.1/§12.8. Exposes `NewRegistry`, `Get`, `List` (sorted),
  `IsInstalled` (exec.LookPath on DetectCommand), `MarshalTOML` (for `providers show`, FR47), and
  `DefaultProvider` (first preferred installed built-in, pi first; FR46). PLUS a config-bridge free
  function `DecodeUserOverrides(map[string]map[string]any) (map[string]Manifest, error)` that realizes
  the frozen `config.go` comment ("the registry re-encodes the entry to TOML and unmarshals into a
  Manifest") — the ONLY way `config.Providers` (raw map) can feed `NewRegistry` (which takes decoded
  Manifests). This is the read site for the CLI `providers` subcommands (P1.M4.T1.S3) and provider
  resolution in the generate flow (P1.M3.T4).

  Builds DIRECTLY on the already-landed `Manifest`+`Validate`+`DetectCommand`+`Resolve`+`strPtr`/`boolPtr`
  (S1, `manifest.go`), `MergeManifest` (S2, `merge.go`), and `BuiltinManifests`+constructors (S2/S3,
  `builtin.go`). It does NOT edit any of those files (frozen contracts).

  ⚠️ **THE central design call — `NewRegistry` stores the MERGED manifest; it does NOT Validate or
  Resolve.** The contract signature is `NewRegistry(userOverrides map[string]Manifest) *Registry` (no
  error return), so validation cannot surface there. A partial §12.8 override legitimately lacks fields;
  lazy validation is correct. The CONSUMER lifecycle (renderer P1.M2.T4 / generate P1.M3.T4) is
  `Get → Validate → Resolve → consume` — matching S1/S2's documented lifecycle (decode → merge →
  Validate → Resolve → consume). The registry owns "merge + store"; the consumer owns "Validate +
  Resolve". This keeps the registry a pure data structure and makes `providers show` able to display a
  partially-defined provider for debugging (MarshalTOML does not Validate either).

  ⚠️ **THE second design call — `MarshalTOML` marshals the STORED (merged, unresolved) manifest, NOT a
  `Resolve()` copy.** FR47 says "fully-resolved manifest (built-in merged with user overrides)". The
  parenthetical defines "resolved" as PRECEDENCE-resolution (the merge), not default-resolution
  (Resolve). go-toml/v2 OMITS nil `*string`/`*bool` pointers on marshal (free omitempty — FINDING A,
  EMPIRICALLY RE-VERIFIED for this subtask), so the output shows exactly what is configured (inherited +
  overridden fields) with absent fields suppressed. For built-ins + full overrides the output is
  complete. `Resolve()` (defaults-inflated) is a display transform belonging to the CLI `providers show`
  command (P1.M4.T1.S3), not this method. **This is the ONE interpretation risk; if FR47 is later read
  as "Resolve() first", it's a one-line change (`toml.Marshal(m.Resolve())`).** The test pins a
  round-trip (marshal → unmarshal → equals stored), unambiguous either way. See research §4.

  ⚠️ **THE third design call — `DecodeUserOverrides` is a FREE FUNCTION, NOT a method and NOT inside
  NewRegistry.** (1) The contract pins `NewRegistry(map[string]Manifest)` — the raw map cannot be its
  param. (2) The registry must stay config-free (S1 design call #4: the provider package never imports
  `internal/config`); DecodeUserOverrides takes the raw-map TYPE without importing config → no cycle.
  (3) It is the ONLY place that knows the `map[string]any` shape; everything downstream is typed. The
  re-encode (`map[string]any` → `toml.Marshal` → `toml.Unmarshal` into Manifest) is EMPIRICALLY VERIFIED
  (research §5): present fields decode correctly; absent fields → nil (FINDING C preserved through the
  bridge); `Name` set from the table key (the body carries no `name`). Without this helper, NewRegistry
  could never consume `config.Providers` and FR48 (user overrides) would be unwirable — so it IS in
  scope, as the frozen config.go comment assigns this exact step to "the registry (P1.M2.T3)".

  ⚠️ **THE fourth design call — `IsInstalled` uses `exec.LookPath(m.DetectCommand())`; DetectCommand
  short-circuits to false when empty.** cursor is the ONLY built-in where Detect ≠ Name (cursor.Detect
  == "agent" — the binary; cursor.Name == "cursor"), so `IsInstalled(cursor)` correctly probes `agent`
  on $PATH. A half-defined §12.8 manifest missing both Detect and Command → DetectCommand()=="" → false
  (guard before the pointless LookPath("")). Deterministic test: FALSE for a bogus command + empty
  DetectCommand; TRUE for `"go"` (guaranteed on $PATH during `go test`, guarded by `t.Skip`).

  ⚠️ **THE fifth design call — `DefaultProvider(installed []string)` takes the installed NAMES as a
  param (the caller computes them via IsInstalled over List()).** This makes it pure/testable (no exec
  inside DefaultProvider). Only BUILT-IN names are candidates (a §12.8 user provider is never
  auto-selected); preference order `["pi","claude","gemini","opencode","codex","cursor"]` (PRD §12.3–12.7
  order, pi first). A safety-net test asserts `preferredBuiltins` == `BuiltinManifests()` keys + pi-first.

  Deliverable: `internal/provider/registry.go` (`package provider`) — the `Registry` struct + the 6
  contract methods + `DecodeUserOverrides` + the `preferredBuiltins` var; and `internal/provider/
  registry_test.go` (`package provider`, white-box) — ~12 test groups. INPUT = S1's `Manifest`/`strPtr`/
  `boolPtr`, S2's `MergeManifest`, S2/S3's `BuiltinManifests`. Touches ONLY the two new files — NO
  go.mod/go.sum change (go-toml/v2 v2.4.2 already present), NO edit to any frozen file. OUTPUT = the
  registry the CLI providers subcommands (P1.M4.T1.S3) and the generate flow (P1.M3.T4) consume.
---

## Goal

**Feature Goal**: Implement the provider `Registry` — the single read site that holds the fully-merged
provider manifests (the 6 built-ins overlaid field-by-field with user overrides per §16.1, plus brand-new
§12.8 providers added verbatim) and answers the questions the CLI + generate flow need: "what providers
exist" (`List`), "is this one on $PATH" (`IsInstalled`), "show me its resolved TOML" (`MarshalTOML`),
"which is the default" (`DefaultProvider`), plus the config→Manifest bridge (`DecodeUserOverrides`).

**Deliverable**:
1. **CREATE** `internal/provider/registry.go` (`package provider`) —
   (a) `type Registry struct { manifests map[string]Manifest }`.
   (b) `var preferredBuiltins = []string{"pi","claude","gemini","opencode","codex","cursor"}`.
   (c) `func NewRegistry(userOverrides map[string]Manifest) *Registry` — seed `BuiltinManifests()`;
       for each override, `MergeManifest` onto the built-in if the name exists, else add verbatim;
       set `.Name = name` in both branches (table key is authoritative). No Validate/Resolve.
   (d) `func (r *Registry) Get(name string) (Manifest, bool)`.
   (e) `func (r *Registry) List() []Manifest` — sorted by `.Name` (asc), deterministic.
   (f) `func (r *Registry) IsInstalled(m Manifest) bool` — `cmd := m.DetectCommand(); cmd=="" → false;
       else exec.LookPath(cmd) == nil`.
   (g) `func (r *Registry) MarshalTOML(name string) (string, error)` — look up; `toml.Marshal` the stored
       manifest; unknown name → wrapped error.
   (h) `func (r *Registry) DefaultProvider(installed []string) string` — first entry in
       `preferredBuiltins` that appears in `installed`; else "".
   (i) `func DecodeUserOverrides(raw map[string]map[string]any) (map[string]Manifest, error)` — for each
       entry: `toml.Marshal(entry)` → `toml.Unmarshal` into a fresh `Manifest` → set `.Name = name`.
   (j) Imports: `fmt`, `os/exec`, `sort`, `github.com/pelletier/go-toml/v2` ONLY (NO `internal/config`).
2. **CREATE** `internal/provider/registry_test.go` (`package provider`, white-box) — the ~12 test groups
   in Implementation Tasks, all passing. Uses S1's unexported `strPtr`/`boolPtr` (same package);
   `reflect.DeepEqual` for struct/slice/map comparison; `os/exec` + `t.Skip` for the IsInstalled true-case.

No other files touched. **No go.mod/go.sum change** (go-toml/v2 already in go.mod; first non-test use in
the provider package is BY DESIGN — the registry is the marshal + config-bridge layer). NO edit to
`manifest.go`/`manifest_test.go` (S1), `merge.go`/`merge_test.go` (S2), `builtin.go`/`builtin_test.go`
(S2 + parallel S3), or any file outside `internal/provider/registry*.go`.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/` clean;
`go mod tidy` is a no-op; `go test -race ./internal/provider/ -v` passes (S1/S2/S3 tests STILL green +
all new registry tests green) and the full suite `go test -race ./...` stays green; the §16.1 keystone
holds at the registry level (overriding pi's `default_model` changes ONLY `default_model`); a §12.8
brand-new provider is added and `Get`-able; `List` is sorted; `IsInstalled` reflects $PATH (false for
bogus/empty, true for a known-present binary); `MarshalTOML` round-trips; `DefaultProvider` prefers pi;
`DecodeUserOverrides` bridges `config.Providers` faithfully (present fields set, absent → nil, Name from
key). go.mod/go.sum and every frozen file byte-unchanged.

## User Persona

**Target User**: The CLI `providers` subcommands (P1.M4.T1.S3 — `providers list` marks installed +
shows the default; `providers show <name>` prints the manifest as TOML) and the generate flow
(P1.M3.T4 — resolves the active provider manifest before rendering/executing). Transitively FR46/FR47/FR48
and every user story routed through "call an agent".

**Use Case**: A user runs `stagecoach` with `[provider.pi] default_model = "glm-5.2"` in their config.
The config loader (P1.M1.T4) populates `config.Providers["pi"]` (raw map). The wiring layer calls
`overrides, _ := provider.DecodeUserOverrides(config.Providers)` then `reg := provider.NewRegistry(overrides)`.
`reg.Get("pi")` returns the MERGED pi manifest (glm-5.2 model, everything else inherited). The renderer
turns it into an argv; `providers show pi` marshals it to TOML; `providers list` marks pi installed via
`IsInstalled` and names it the default via `DefaultProvider`.

**User Journey**: (internal API, no end-user surface yet) `config.Providers` (raw map) →
`DecodeUserOverrides` → `NewRegistry` (seed built-ins + merge) → `Get`/`List`/`IsInstalled`/
`MarshalTOML`/`DefaultProvider` → consumer `Validate` → `Resolve` → render/exec/parse.

**Pain Points Addressed**: Removes "where do built-ins + user overrides get merged / how does the raw
config map become typed manifests / how do we detect installed providers / what does `providers show`
print / which provider is the default" ambiguity by landing one tested registry now.

## Why

- **The registry is the single merge + read site for the whole provider system.** P1.M4.T1.S3 (CLI) and
  P1.M3.T4 (generate) both need "the resolved manifest for <name>". Centralizing the merge here means
  the §16.1/§12.8 logic lives in ONE place (not re-implemented in the CLI and the generate flow).
- **Realizes the frozen `config.go` contract.** `config.Providers` is a raw `map[string]map[string]any`
  precisely because config must not import the Manifest type (cycle). The frozen comment assigns the
  re-encode-to-TOML step to "the registry (P1.M2.T3)". `DecodeUserOverrides` IS that step — without it,
  `NewRegistry` could never consume config and FR48 (user overrides) would be unwirable.
- **Unlocks the CLI provider UX (FR46/FR47).** `providers list` needs `List` + `IsInstalled` +
  `DefaultProvider`; `providers show` needs `MarshalTOML`. Landing these now lets P1.M4.T1.S3 be thin
  presentation code over a stable API.
- **Proves S1+S2 compose end-to-end.** The registry is the first caller of `BuiltinManifests() →
  MergeManifest → store`. Its tests are the integration proof that the pointer-merge design (S1) and
  the field-by-field merge (S2) produce a correct, complete registry.
- **No user-facing surface change** (PRD "DOCS: none — internal"). The `providers` UX docs come with
  P1.M4.T1.S3 (Mode A).
- **No new dependency, no config import edge.** go-toml/v2 is already in go.mod; the registry is the
  first non-test file in `provider` to use it (by design). `internal/config` is NEVER imported (cycle).

## What

A compiled `internal/provider` package exporting `Registry` + its 6 methods + `DecodeUserOverrides` +
`preferredBuiltins`, layered on S1's `Manifest`/`MergeManifest`/`BuiltinManifests`. No rendering, no
execution, no parsing, no CLI, no config edits.

### Success Criteria

- [ ] `internal/provider/registry.go` exists, `package provider`, imports EXACTLY `fmt`, `os/exec`,
      `sort`, `github.com/pelletier/go-toml/v2`. It does NOT import `internal/config`.
- [ ] `type Registry struct { manifests map[string]Manifest }` (unexported map; methods on `*Registry`).
- [ ] `var preferredBuiltins = []string{"pi","claude","gemini","opencode","codex","cursor"}`.
- [ ] `NewRegistry(userOverrides map[string]Manifest) *Registry`: seeds a FRESH copy of
      `BuiltinManifests()`; for each override, if name exists → `MergeManifest(base, override)` with
      `.Name = name`, else add `override` with `.Name = name`. Does NOT call `Validate`/`Resolve`.
- [ ] `Get(name)` returns `(manifest, true)` for a known name, `(zero, false)` for unknown.
- [ ] `List()` returns ALL manifests sorted ascending by `.Name` (deterministic; built-ins + user-added).
- [ ] `IsInstalled(m)` returns `false` when `m.DetectCommand()==""`; otherwise `exec.LookPath(cmd)==nil`.
- [ ] `MarshalTOML(name)` returns the `toml.Marshal`-ed stored manifest as a string for a known name,
      and `("", error)` for an unknown name.
- [ ] `DefaultProvider(installed)` returns the first `preferredBuiltins` entry present in `installed`,
      else `""`.
- [ ] `DecodeUserOverrides(raw)` returns a non-nil `map[string]Manifest` (empty for nil/empty input);
      each entry re-encoded via `toml.Marshal` → `toml.Unmarshal` into a Manifest with `.Name` set from
      the key; a malformed entry yields a wrapped error naming the key.
- [ ] `registry_test.go` has the ~12 test groups below, all passing.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` (diff-empty) clean;
      `go test -race ./internal/provider/` AND `go test -race ./...` green.
- [ ] go.mod/go.sum byte-unchanged; every file outside the two new `registry*.go` files byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact method
signatures + the merge algorithm (§2 of the research note), the empirically-verified go-toml marshal +
re-encode behaviors (§4/§5), the `Manifest`/`MergeManifest`/`BuiltinManifests`/`DetectCommand`/`strPtr`/
`boolPtr` contracts (already landed — read `manifest.go`/`merge.go`/`builtin.go`), the IsInstalled
deterministic-test approach (`t.Skip` guard), and the ~12 test specs. No git/generate/CLI knowledge
required — the registry is a pure data structure over already-landed types.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- file: internal/provider/manifest.go   (S1 — COMPLETE; read, do NOT edit)
  why: the EXACT Manifest type + the methods the registry calls: `DetectCommand() string` (Detect if
       set & non-empty else Command else ""), `Validate() error`, `Resolve() Manifest`, and the
       unexported helpers `strPtr(string) *string` / `boolPtr(bool) *bool` (same package — use in tests).
       Also the `Default*` constants. Field names/types are FROZEN.
  critical: `DetectCommand()` is the IsInstalled probe target (handles the cursor detect="agent" case
       and the empty case). Do NOT edit this file.

- file: internal/provider/merge.go   (S2 — COMPLETE; read, do NOT edit)
  why: `func MergeManifest(base, override Manifest) Manifest` — the field-by-field overlay NewRegistry
       calls for built-in overrides. Its three regimes (scalar `!= nil` wins incl. explicit zero; slice
       `len>0` replaces wholesale; Env key-by-key into a FRESH map) + its no-mutate guarantee are
       inherited wholesale by NewRegistry. MergeManifest preserves `base.Name`.
  critical: NewRegistry does NOT re-implement merging — it CALLS MergeManifest. Do NOT edit.

- file: internal/provider/builtin.go   (S2 + parallel S3; read, do NOT edit)
  why: `func BuiltinManifests() map[string]Manifest` returns the 6 built-ins (pi/claude/gemini/
       opencode/codex/cursor) FRESH per call. NewRegistry seeds from this. The `preferredBuiltins` list
       MUST match these keys (a test enforces it). NOTE cursor: Detect/Command="agent" (≠ Name "cursor").
  critical: BuiltinManifests() returns a FRESH map each call (no package-level var) — NewRegistry copies
       it into its own map so no caller can corrupt the built-ins. Do NOT edit this file. (If S3 is not
       yet landed when you start, assume the contract: 6 keys; the registry is written against the
       6-key API and its tests assert 6 built-ins.)

- file: internal/config/config.go   (P1.M1.T4 — FROZEN; read, do NOT edit)
  section: the `Providers map[string]map[string]any \`toml:"-"\`` field + its doc comment.
  why: the FROZEN contract `DecodeUserOverrides` realizes: "for each name it re-encodes the entry to
       TOML and unmarshals into a Manifest, then field-merges with the built-in manifest per PRD §16.1."
       DecodeUserOverrides is the re-encode step; NewRegistry is the field-merge step.
  critical: the registry must NOT import `internal/config` (cycle). DecodeUserOverrides takes the
       raw-map TYPE (`map[string]map[string]any`) without importing config. Do NOT edit config.go.

- docfile: plan/001_f1f80943ac34/P1M2T3S1/research/registry-design-and-toml-behavior.md
  why: the SINGLE most important read — the merge algorithm (§2), the IsInstalled approach + cursor
       detect≠name + empty short-circuit (§3), the EMPIRICALLY-VERIFIED go-toml Marshal behavior incl.
       nil-pointer omission (§4), the config bridge round-trip (§5), the lifecycle decision (§6), and
       the DefaultProvider preference order (§7).
  critical: §4 (MarshalTOML marshals the stored/unresolved manifest — the ONE interpretation risk) and
       §5 (DecodeUserOverrides re-encode round-trip) are the things most likely to be implemented wrong
       or second-guessed. Read them before writing MarshalTOML / DecodeUserOverrides.

- file: PRD.md
  section: "16.1 Resolution order (FR34)" (h3.57) — "Provider manifests merge field-by-field (a user
       override that sets only default_model leaves all other fields from the built-in intact)."
  why: the merge NewRegistry performs. The §16.2 example (`[provider.pi] default_model = "glm-5.2"`) is
       the canonical registry test fixture.
  critical: the merge is FIELD-BY-FIELD, not whole-manifest-replace. NewRegistry delegates to S2's
       MergeManifest, which already implements this correctly.

- file: PRD.md
  section: "12.8 Extensibility: user-defined providers" (h3.44) — the brand-new-provider case.
  why: a `[provider.myagent]` block with NO matching built-in is ADDED verbatim (not merged). The TOML
       body has NO `name =` line (the key IS the name) → DecodeUserOverrides/NewRegistry set `.Name`
       from the key. The §12.8 example block is the DecodeUserOverrides test fixture.

- file: PRD.md
  section: "9.11 Provider management" (h3.27) — FR46 (providers list: mark detected-on-$PATH, show
       resolved default), FR47 (providers show: fully-resolved manifest as TOML), FR48 (user overrides
       merge; new names add).
  why: the registry methods exist to serve these FRs. FR46 → List + IsInstalled + DefaultProvider;
       FR47 → MarshalTOML; FR48 → NewRegistry merge + add + DecodeUserOverrides.
  critical: FR47 "fully-resolved (built-in merged with user overrides)" → marshal the MERGED manifest
       (research §4). FR46 default → DefaultProvider (pi preferred).

- file: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md
  section: "5.7 Streaming Encode with Encoder" + "5.8 Decoding into map[string]any" + "5.5 Decoding
       [provider.X] into map[string]Manifest".
  why: §5.8 is the re-encode pattern DecodeUserOverrides uses (decode-to-generic-map → marshal →
       re-decode into the typed struct); §5.7 shows `toml.NewEncoder` (not needed — `toml.Marshal` to a
       []byte suffices for MarshalTOML's string return); §5.5 confirms map-of-struct decode is native.
  critical: do NOT over-engineer MarshalTOML with a custom Encoder/IndentSymbol — `toml.Marshal(m)` to
       []byte → string is the minimal correct implementation (research §4 verified the output).

- file: plan/001_f1f80943ac34/architecture/critical_findings.md
  section: "FINDING 5" — go-toml/v2 has no omitempty; the merge must be field-by-field.
  why: the constraint S1/S2 satisfied with pointers and that this registry inherits. FINDING 5's note
       that "providers show" should "use pointer fields or a custom MarshalTOML() to suppress empty
       fields" is ALREADY solved by S1's pointer design (nil pointers are omitted by go-toml — research
       §4) → no custom marshaler needed.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; require go-toml/v2 v2.4.2 + pflag v1.0.10  (UNCHANGED)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — FROZEN, do NOT touch; do NOT import from provider
    config.go                   # Providers map[string]map[string]any `toml:"-"`  ← DecodeUserOverrides' input type
    ...                         # file.go / git.go / load.go + tests — untouched
  git/                          # P1.M1.T2/T3 — untouched
  provider/                     # S1 created; S2 added merge+builtin(pi/claude/gemini/opencode); S3(parallel) adds codex/cursor
    manifest.go                 # S1 — Manifest + Validate + DetectCommand + Resolve + Default* + strPtr/boolPtr  (CONTRACT — do NOT edit)
    manifest_test.go            # S1 — tests  (do NOT edit)
    merge.go                    # S2 — MergeManifest  (do NOT edit)
    merge_test.go               # S2 — tests  (do NOT edit)
    builtin.go                  # S2+S3 — BuiltinManifests() (6 keys) + constructors  (do NOT edit)
    builtin_test.go             # S2+S3 — tests  (do NOT edit)
    registry.go                 # NEW (this subtask) ← Registry + 6 methods + DecodeUserOverrides + preferredBuiltins
    registry_test.go            # NEW (this subtask) ← ~12 test groups (white-box)
cmd/stagecoach/main.go           # `package main; func main(){}` stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  provider/
    registry.go                 # NEW — Registry struct + NewRegistry/Get/List/IsInstalled/MarshalTOML/DefaultProvider + DecodeUserOverrides + preferredBuiltins
    registry_test.go            # NEW — ~12 test groups, package provider (white-box)
# manifest.go/manifest_test.go (S1) + merge.go/merge_test.go (S2) + builtin.go/builtin_test.go (S2/S3)
# UNCHANGED. go.mod/go.sum UNCHANGED. Every other file UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call #1 — NewRegistry stores MERGED, does NOT Validate/Resolve): the contract
// signature is `NewRegistry(...) *Registry` (no error), so validation cannot surface there. A partial
// §12.8 override legitimately lacks fields. The CONSUMER does Get → Validate → Resolve → consume.
// MarshalTOML also does NOT Validate (so `providers show` can display a partial provider for debugging).
// Do NOT add Validate/Resolve calls inside NewRegistry/MarshalTOML.

// CRITICAL (design call #2 — MarshalTOML marshals the STORED/unresolved manifest): go-toml OMITS nil
// *string/*bool pointers on marshal (free omitempty — FINDING A, re-verified). So the output shows what
// is configured (inherited + overridden) with absent fields suppressed. Do NOT call m.Resolve() before
// marshal (that would inflate defaults the user didn't set, misleading `providers show`). If FR47 is
// later re-read as "Resolve first", it's a one-line change — flagged as the ONE interpretation risk.

// CRITICAL (design call #3 — DecodeUserOverrides is a FREE FUNCTION, not a method, not in NewRegistry):
// (1) contract pins NewRegistry(map[string]Manifest); (2) provider must NOT import config (cycle) —
// DecodeUserOverrides takes the raw-map TYPE without importing config; (3) it's the only place that
// knows the map[string]any shape. The re-encode round-trip (toml.Marshal(entry) → toml.Unmarshal into
// Manifest) is VERIFIED (research §5): present fields decode, absent → nil, Name from key. MUST set
// m.Name = name (the body carries no "name").

// CRITICAL (design call #4 — IsInstalled uses DetectCommand; empty short-circuits): cmd := m.DetectCommand();
// if cmd == "" { return false }; return exec.LookPath(cmd) == nil. cursor is the ONLY built-in with
// Detect ≠ Name (Detect="agent"), so IsInstalled(cursor) probes "agent" — correct. Do NOT probe m.Name.

// CRITICAL (design call #5 — DefaultProvider takes installed NAMES as a param): pure/testable (no exec).
// Only built-in names are candidates (preferredBuiltins); a §12.8 provider is never auto-selected.
// Returns "" if none of the preferred built-ins are installed. preferredBuiltins[0] MUST be "pi".

// GOTCHA: seed NewRegistry's map from a FRESH copy of BuiltinManifests() (range into your own map). Do
// NOT store the BuiltinManifests() return directly and mutate it — though BuiltinManifests() is fresh
// per call (S2 design), copying is defense-in-depth and makes the "no shared mutation" guarantee local.

// GOTCHA: in NewRegistry's override loop, set merged.Name = name (override branch) AND override.Name =
// name (new branch) — the table key is authoritative and the decoded body has no "name". Idempotent if a
// caller already set Name. Do NOT skip this (a §12.8 provider would otherwise have Name=="").

// GOTCHA: registry.go imports go-toml/v2 — the FIRST non-test file in provider to do so. This is BY
// DESIGN (the registry is the marshal + config-bridge layer). go.mod is UNCHANGED (dep already present).
// Verify with `git diff --exit-code go.mod go.sum`. Do NOT import internal/config (cycle).

// GOTCHA: go-toml marshals nil slices as `[]` (e.g. `subcommand = []`) and nil maps are omitted. This is
// cosmetic + acceptable for `providers show`. Do NOT add a custom MarshalTOML to suppress `[]` — out of
// scope (P1.M4.T1.S3 may add presentation polish if desired).

// GOTCHA: go-toml emits SINGLE-quoted literal strings ('pi', not "pi"). Valid TOML; cosmetic. The
// hand-written providers/*.toml reference files (P1.M5.T2) are independent of marshaled output.

// GOTCHA: List() must SORT (map iteration order is random). Use sort.Slice by .Name asc. Return a fresh
// slice (do NOT expose the internal map). Deterministic order is required for `providers list` output.

// GOTCHA: IsInstalled's true-case test uses "go" (guaranteed on $PATH during `go test`). Guard with
// t.Skip if exec.LookPath("go") fails so the suite never flakes on an odd CI image. The FALSE cases
// (bogus command, empty DetectCommand) are fully deterministic — those are the important logic tests.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/provider/registry.go
package provider

import (
	"fmt"
	"os/exec"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

// preferredBuiltins is the default-provider preference order (PRD §12.3–12.7 listing order; pi first).
// DefaultProvider returns the first name in this list that the caller reports installed. It MUST stay
// in sync with BuiltinManifests() keys — a test (TestPreferredBuiltins_MatchesBuiltinKeys) enforces this.
// Only built-in names are candidates; user-defined §12.8 providers are never auto-selected.
var preferredBuiltins = []string{"pi", "claude", "gemini", "opencode", "codex", "cursor"}

// Registry holds the fully-merged provider manifests: the built-in defaults (BuiltinManifests, P1.M2.T2)
// overlaid field-by-field with user overrides via MergeManifest (S2) per PRD §16.1/§12.8. Brand-new §12.8
// provider names are added verbatim. It is the single read site for the CLI providers subcommands
// (P1.M4.T1.S3) and provider resolution in the generate flow (P1.M3.T4).
//
// NewRegistry does NOT Validate or Resolve — consumers call Validate then Resolve on the manifest they
// Get (lifecycle: decode → merge → [store] → Validate → Resolve → consume). This keeps the registry a
// pure data structure and lets `providers show` display a partially-defined provider for debugging.
type Registry struct {
	manifests map[string]Manifest
}

// NewRegistry builds a Registry seeded with BuiltinManifests(), then overlays each userOverride: if the
// name matches a built-in, MergeManifest (S2) field-merges it (PRD §16.1); otherwise the override is
// added verbatim as a brand-new provider (§12.8). In both branches the table key is set as the manifest
// Name (the [provider.<name>] body carries no "name" key). userOverrides are EXPECTED to be decoded
// Manifests — use DecodeUserOverrides to bridge from config.Providers (raw map). No Validate/Resolve.
func NewRegistry(userOverrides map[string]Manifest) *Registry {
	manifests := make(map[string]Manifest, len(userOverrides)+6) // built-ins + overrides headroom
	// Seed with a FRESH copy of the built-ins (BuiltinManifests constructs fresh each call; copying
	// localizes the no-shared-mutation guarantee).
	for name, m := range BuiltinManifests() {
		manifests[name] = m
	}
	// Overlay user overrides: merge onto a built-in, or add brand-new verbatim (§12.8).
	for name, override := range userOverrides {
		if base, ok := manifests[name]; ok {
			merged := MergeManifest(base, override) // S2: field-by-field; preserves base.Name; never mutates base
			merged.Name = name                      // table key is the authoritative identity
			manifests[name] = merged
		} else {
			override.Name = name // a §12.8 provider: identity comes from the table key
			manifests[name] = override
		}
	}
	return &Registry{manifests: manifests}
}

// Get returns the merged manifest for name and whether it exists in the registry.
func (r *Registry) Get(name string) (Manifest, bool) {
	m, ok := r.manifests[name]
	return m, ok
}

// List returns every manifest in the registry, sorted ascending by Name (deterministic for `providers
// list`). The returned slice is a fresh copy; the internal map is not exposed.
func (r *Registry) List() []Manifest {
	out := make([]Manifest, 0, len(r.manifests))
	for _, m := range r.manifests {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// IsInstalled reports whether the provider's discovery command is on $PATH (FR46). It probes
// m.DetectCommand() (Detect if set & non-empty, else Command) via exec.LookPath. A manifest with
// neither Detect nor Command set (DetectCommand()=="") reports false. NOTE cursor is the only built-in
// where Detect ≠ Name (Detect="agent" — the binary), so this correctly probes "agent", not "cursor".
func (r *Registry) IsInstalled(m Manifest) bool {
	cmd := m.DetectCommand()
	if cmd == "" {
		return false
	}
	_, err := exec.LookPath(cmd)
	return err == nil
}

// MarshalTOML returns the stored (merged) manifest for name as TOML (FR47 — providers show). It marshals
// the MERGED manifest (built-in ⊕ user overrides), NOT a Resolve() copy: go-toml OMITS nil *string/*bool
// pointers (free omitempty), so the output shows what is actually configured with absent fields
// suppressed. Returns a wrapped error if name is unknown. (If a defaults-inflated view is ever wanted,
// the CLI layer may Get+Resolve+marshal itself; this method stays a thin name→TOML lookup.)
func (r *Registry) MarshalTOML(name string) (string, error) {
	m, ok := r.manifests[name]
	if !ok {
		return "", fmt.Errorf("provider registry: unknown provider %q", name)
	}
	data, err := toml.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("provider registry: marshal %q: %w", name, err)
	}
	return string(data), nil
}

// DefaultProvider returns the first built-in (in preference order, pi first) that the caller reports
// installed, or "" if none of the preferred built-ins are installed (FR46 — "show the resolved default").
// installed is the caller's list of installed provider NAMES (computed via IsInstalled over List()).
// Taking it as a param keeps this pure/testable (no exec inside). Only built-in names are candidates;
// user-defined §12.8 providers are never auto-selected.
func (r *Registry) DefaultProvider(installed []string) string {
	present := make(map[string]struct{}, len(installed))
	for _, name := range installed {
		present[name] = struct{}{}
	}
	for _, name := range preferredBuiltins {
		if _, ok := present[name]; ok {
			return name
		}
	}
	return ""
}

// DecodeUserOverrides bridges config.Providers (raw map[string]map[string]any, P1.M1.T4) to the
// map[string]Manifest NewRegistry consumes. For each [provider.<name>] entry it re-encodes the raw map
// to TOML and unmarshals into a Manifest (the pattern the frozen config.go comment specifies for "the
// registry (P1.M2.T3)"), then sets Name from the table key (the TOML body carries no "name"). Returns a
// non-nil (possibly empty) map for nil/empty input. A malformed entry yields a wrapped error naming the
// key. This is the ONLY place that touches the raw config-map shape; the Registry works in typed
// Manifests. It does NOT import internal/config (it takes the type, not the package) — no import cycle.
func DecodeUserOverrides(raw map[string]map[string]any) (map[string]Manifest, error) {
	out := make(map[string]Manifest, len(raw))
	for name, entry := range raw {
		data, err := toml.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("provider override %q: re-encode to TOML: %w", name, err)
		}
		var m Manifest
		if err := toml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("provider override %q: decode: %w", name, err)
		}
		m.Name = name // the table key is the identity; the body has no "name"
		out[name] = m
	}
	return out, nil
}
```

> **gofmt note:** run `gofmt -w internal/provider/registry.go internal/provider/registry_test.go`. Do
> not hand-align. One doc comment per exported identifier (citing the PRD section + the design calls) is
> required — it seeds `providers show` / reference-file docs later.
>
> **Imports:** EXACTLY `fmt`, `os/exec`, `sort`, `github.com/pelletier/go-toml/v2` in registry.go. If
> `go vet` flags an unused import, remove it. `registry_test.go` adds `testing`, `reflect`, `os/exec`,
> `github.com/pelletier/go-toml/v2` (all already in go.mod).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/provider/registry.go — Registry + NewRegistry + Get + List
  - IMPLEMENT type Registry struct { manifests map[string]Manifest } (unexported map).
  - IMPLEMENT var preferredBuiltins = []string{"pi","claude","gemini","opencode","codex","cursor"}.
  - IMPLEMENT NewRegistry per the Data Models block: make a fresh map; range BuiltinManifests() to seed;
      range userOverrides; if name exists → MergeManifest(base, override), merged.Name=name, store; else
      override.Name=name, store. Return &Registry{manifests: manifests}.
  - IMPLEMENT Get(name) (Manifest, bool) — map lookup.
  - IMPLEMENT List() []Manifest — fresh slice; sort.Slice by .Name asc.
  - IMPORTS so far: sort (List). (fmt/exec/toml added in Tasks 2–4.)
  - GOTCHA: do NOT Validate/Resolve in NewRegistry. Do NOT store BuiltinManifests() return directly and
      mutate — copy via range. Set .Name=name in BOTH branches.

Task 2: ADD IsInstalled + DefaultProvider to registry.go
  - IMPLEMENT IsInstalled(m): cmd := m.DetectCommand(); if cmd == "" { return false }; _, err :=
      exec.LookPath(cmd); return err == nil.
  - IMPLEMENT DefaultProvider(installed): build a set from installed; range preferredBuiltins; return
      the first present; else "".
  - ADD import "os/exec".
  - GOTCHA: use DetectCommand() (NOT m.Name, NOT *m.Command directly) — handles cursor detect="agent"
      and the empty case. DefaultProvider must NOT exec (it takes the names as a param).

Task 3: ADD MarshalTOML + DecodeUserOverrides to registry.go
  - IMPLEMENT MarshalTOML(name): lookup; unknown → ("", fmt.Errorf("provider registry: unknown
      provider %q", name)); toml.Marshal(m) → on error wrap; return string(data), nil. Do NOT Resolve.
  - IMPLEMENT DecodeUserOverrides(raw): make(map[string]Manifest, len(raw)); for name, entry := range raw:
      toml.Marshal(entry) → toml.Unmarshal into fresh Manifest → m.Name=name → out[name]=m. Wrap errors
      naming the key. Return out, nil for nil/empty raw.
  - ADD imports "fmt" + "github.com/pelletier/go-toml/v2".
  - GOTCHA: MarshalTOML marshals the STORED (unresolved) manifest — do NOT call Resolve(). DecodeUserOverrides
      MUST set m.Name=name (the body has no "name"). DecodeUserOverrides is a FREE FUNCTION (not a method).

Task 4: CREATE internal/provider/registry_test.go — the ~12 test groups (see Test Specs below)
  - PACKAGE: `package provider` (white-box — uses S1's unexported strPtr/boolPtr). Imports: testing,
      reflect, os/exec, github.com/pelletier/go-toml/v2.
  - MIRROR repo test style: stdlib testing, direct t.Errorf("X = %v, want %v", got, want), reflect.DeepEqual
      for structs/slices/maps. (See internal/provider/builtin_test.go + merge_test.go for the idiom.)
  - ADD a sampleOverride() helper if useful (a full §12.8 myagent manifest for the add-new tests).

Task 5: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. S1's manifest*.go,
      S2's merge*.go + builtin*.go MUST be byte-unchanged. S1/S2/S3 tests MUST stay green. The config +
      git suites MUST stay green (no import edge into them).
```

### Test Specs (registry_test.go — ~12 groups)

```go
// 1. preferredBuiltins sanity (safety net — fails if a built-in is added without updating the list).
func TestPreferredBuiltins_MatchesBuiltinKeys(t *testing.T) {
	bk := BuiltinManifests()
	set := map[string]struct{}{}
	for _, n := range preferredBuiltins { set[n] = struct{}{} }
	if len(set) != len(bk) { t.Errorf("preferredBuiltins has %d, builtins %d", len(set), len(bk)) }
	for k := range bk { if _, ok := set[k]; !ok { t.Errorf("built-in %q not in preferredBuiltins", k) } }
	if len(preferredBuiltins) == 0 || preferredBuiltins[0] != "pi" { t.Errorf("pi must be first; got %v", preferredBuiltins) }
}

// 2. NewRegistry with no overrides → exactly the 6 built-ins, each Get-able, Name == key.
//    (If S3 hasn't landed yet, this test still passes against the 6-key contract; if only 4 land first,
//     adjust the count — but the CONTRACT is 6.)
func TestNewRegistry_NoOverrides_HasAllBuiltins(t *testing.T) {
	r := NewRegistry(nil)
	if got := len(r.manifests); got != len(BuiltinManifests()) { t.Fatalf("len = %d, want %d", got, len(BuiltinManifests())) }
	for name := range BuiltinManifests() {
		m, ok := r.Get(name)
		if !ok { t.Errorf("Get(%q) missing", name) }
		if m.Name != name { t.Errorf("%q: Name = %q, want %q", name, m.Name, name) }
	}
}

// 3. THE §16.1 KEYSTONE at the registry level: override pi's default_model → ONLY default_model changes.
func TestNewRegistry_OverrideExisting_OnlyTouchedFieldChanges(t *testing.T) {
	base, _ := NewRegistry(nil).Get("pi") // the built-in pi
	r := NewRegistry(map[string]Manifest{"pi": {DefaultModel: strPtr("glm-5.2")}})
	got, ok := r.Get("pi")
	if !ok { t.Fatal("pi missing") }
	if got.DefaultModel == nil || *got.DefaultModel != "glm-5.2" { t.Errorf("DefaultModel = %v, want glm-5.2", got.DefaultModel) }
	// Untouched fields survive from the built-in:
	if *got.Command != *base.Command { t.Errorf("Command changed: %q vs %q", *got.Command, *base.Command) }
	if !reflect.DeepEqual(got.BareFlags, base.BareFlags) { t.Errorf("BareFlags changed: %v vs %v", got.BareFlags, base.BareFlags) }
	if *got.PrintFlag != *base.PrintFlag { t.Errorf("PrintFlag changed") }
	if got.Name != "pi" { t.Errorf("Name = %q, want pi", got.Name) }
}

// 4. Explicit-zero override wins at the registry level (proves S1's pointer design carries through).
func TestNewRegistry_OverrideExisting_ExplicitZeroWins(t *testing.T) {
	r := NewRegistry(map[string]Manifest{"pi": {StripCodeFence: boolPtr(false), PrintFlag: strPtr("")}})
	got, _ := r.Get("pi")
	if got.StripCodeFence == nil || *got.StripCodeFence != false { t.Errorf("StripCodeFence = %v, want false", got.StripCodeFence) }
	if got.PrintFlag == nil || *got.PrintFlag != "" { t.Errorf("PrintFlag = %v, want \"\"", got.PrintFlag) }
}

// 5. Brand-new §12.8 provider is ADDED verbatim (not merged), Name from key, count = builtins+1.
func TestNewRegistry_NewName_AddedVerbatim(t *testing.T) {
	my := Manifest{Command: strPtr("/opt/agent"), PromptDelivery: strPtr("stdin"), BareFlags: []string{"--x"}}
	r := NewRegistry(map[string]Manifest{"myagent": my})
	got, ok := r.Get("myagent")
	if !ok { t.Fatal("myagent missing") }
	if got.Name != "myagent" { t.Errorf("Name = %q, want myagent", got.Name) }
	if *got.Command != "/opt/agent" { t.Errorf("Command lost") }
	if !reflect.DeepEqual(got.BareFlags, []string{"--x"}) { t.Errorf("BareFlags = %v", got.BareFlags) }
	if got := len(r.manifests); got != len(BuiltinManifests())+1 { t.Errorf("count = %d, want %d", got, len(BuiltinManifests())+1) }
}

// 6. Get missing → (zero, false).
func TestGet_MissingReturnsFalse(t *testing.T) {
	r := NewRegistry(nil)
	if _, ok := r.Get("nope"); ok { t.Error("want false for unknown name") }
}

// 7. List sorted ascending by Name (built-ins + a user provider), deterministic.
func TestList_SortedByName(t *testing.T) {
	r := NewRegistry(map[string]Manifest{"zzz": {Command: strPtr("x")}, "aaa": {Command: strPtr("y")}})
	list := r.List()
	for i := 1; i < len(list); i++ {
		if !(list[i-1].Name < list[i].Name) { t.Errorf("not sorted at %d: %q >= %q", i, list[i-1].Name, list[i].Name) }
	}
}

// 8. IsInstalled: false for bogus + empty; true for a known-present binary ("go"); Detect wins over Command.
func TestIsInstalled(t *testing.T) {
	r := NewRegistry(nil)
	if r.IsInstalled(Manifest{Command: strPtr("definitely-not-a-real-binary-xyz123")}) { t.Error("bogus reported installed") }
	if r.IsInstalled(Manifest{}) { t.Error("empty DetectCommand reported installed") }
	if _, err := exec.LookPath("go"); err != nil { t.Skip("go not on PATH (unexpected in go test env)") }
	if !r.IsInstalled(Manifest{Command: strPtr("go")}) { t.Error("go reported not installed") }
	// Detect overrides Command for detection:
	if r.IsInstalled(Manifest{Detect: strPtr("definitely-not-real-abc"), Command: strPtr("go")}) { t.Error("Detect=bogus should be false despite Command=go") }
}

// 9. MarshalTOML: known name → valid TOML that round-trips to the stored manifest; unknown → error.
func TestMarshalTOML_RoundTrip(t *testing.T) {
	r := NewRegistry(nil)
	s, err := r.MarshalTOML("pi")
	if err != nil { t.Fatalf("MarshalTOML(pi): %v", err) }
	var decoded Manifest
	if err := toml.Unmarshal([]byte(s), &decoded); err != nil { t.Fatalf("re-decode: %v", err) }
	stored, _ := r.Get("pi")
	if !reflect.DeepEqual(decoded, stored) { t.Errorf("round-trip mismatch:\nmarshaled=\n%s\ndecoded=%+v\nstored=%+v", s, decoded, stored) }
}
func TestMarshalTOML_UnknownErrors(t *testing.T) {
	r := NewRegistry(nil)
	if _, err := r.MarshalTOML("nope"); err == nil { t.Error("want error for unknown name") }
}

// 10. MarshalTOML reflects a merged override (the overridden value appears in the TOML).
func TestMarshalTOML_ReflectsMerge(t *testing.T) {
	r := NewRegistry(map[string]Manifest{"pi": {DefaultModel: strPtr("glm-5.2")}})
	s, _ := r.MarshalTOML("pi")
	if !strings.Contains(s, "glm-5.2") { t.Errorf("merged default_model missing from TOML:\n%s", s) }
}

// 11. DefaultProvider: pi preferred; falls through to next; "" if none; ignores user-defined names.
func TestDefaultProvider(t *testing.T) {
	r := NewRegistry(nil)
	if got := r.DefaultProvider([]string{"pi", "claude"}); got != "pi" { t.Errorf("got %q, want pi", got) }
	if got := r.DefaultProvider([]string{"claude", "gemini"}); got != "claude" { t.Errorf("got %q, want claude", got) }
	if got := r.DefaultProvider([]string{"cursor", "pi"}); got != "pi" { t.Errorf("got %q, want pi (preferred)", got) }
	if got := r.DefaultProvider([]string{"myagent"}); got != "" { t.Errorf("user-defined-only: got %q, want \"\"", got) }
	if got := r.DefaultProvider(nil); got != "" { t.Errorf("nil: got %q, want \"\"", got) }
}

// 12. DecodeUserOverrides: bridges raw config map → typed Manifests; absent→nil; Name from key; errors wrapped.
func TestDecodeUserOverrides(t *testing.T) {
	raw := map[string]map[string]any{
		"myagent": {"command": "/opt/agent", "prompt_delivery": "stdin", "bare_flags": []any{"--no-mcp"}, "default_model": "m1"},
		"pi":      {"default_model": "glm-5.2"}, // override a built-in name
	}
	got, err := DecodeUserOverrides(raw)
	if err != nil { t.Fatalf("DecodeUserOverrides: %v", err) }
	if len(got) != 2 { t.Fatalf("len = %d, want 2", len(got)) }
	my := got["myagent"]
	if my.Name != "myagent" { t.Errorf("myagent.Name = %q", my.Name) }
	if *my.Command != "/opt/agent" { t.Errorf("Command = %q", *my.Command) }
	if !reflect.DeepEqual(my.BareFlags, []string{"--no-mcp"}) { t.Errorf("BareFlags = %v", my.BareFlags) }
	if my.Detect != nil { t.Errorf("Detect = %v, want nil (absent)", my.Detect) } // FINDING C through the bridge
	if my.StripCodeFence != nil { t.Errorf("StripCodeFence = %v, want nil (absent)", my.StripCodeFence) }
	pi := got["pi"]
	if *pi.DefaultModel != "glm-5.2" { t.Errorf("pi.DefaultModel = %q", *pi.DefaultModel) }
	if pi.Command != nil { t.Errorf("pi.Command = %v, want nil (absent in override)", pi.Command) }
	// nil input → empty non-nil map, no error.
	got0, err0 := DecodeUserOverrides(nil)
	if err0 != nil || got0 == nil || len(got0) != 0 { t.Errorf("nil input: got=%v err=%v", got0, err0) }
}
```

> **Note on TestMarshalTOML_ReflectsMerge:** it uses `strings.Contains` — add `"strings"` to the test
> imports, OR replace with a decode+check (marshal → unmarshal → assert *DefaultModel == "glm-5.2").
> Either is fine; the decode+check avoids the strings import.

### Implementation Patterns & Key Details

```go
// The merge-onto-built-in call (NewRegistry override branch) — delegates ENTIRELY to S2's MergeManifest.
// Do NOT re-implement field-by-field merge here; S2 already does it correctly (incl. explicit-zero wins
// and the no-mutate guarantee). NewRegistry only adds the "seed built-ins + route override vs new" logic.
if base, ok := manifests[name]; ok {
	merged := MergeManifest(base, override) // S2: field-by-field; explicit ""/false wins; base never mutated
	merged.Name = name                      // table key is the identity
	manifests[name] = merged
} else {
	override.Name = name // §12.8 brand-new: identity from the key (body has no "name")
	manifests[name] = override
}

// IsInstalled — DetectCommand() handles Detect-vs-Command AND the empty case; cursor's detect="agent"
// is correctly probed (NOT the Name "cursor"). Empty → false (guard before a pointless LookPath(" ")).
cmd := m.DetectCommand()
if cmd == "" {
	return false
}
_, err := exec.LookPath(cmd)
return err == nil

// MarshalTOML — marshal the STORED manifest; go-toml omits nil pointers (free omitempty). Do NOT Resolve.
data, err := toml.Marshal(m) // m is the stored (merged, unresolved) manifest
// (nil *string/*bool omitted; nil slice → `[]`; nil map omitted. Single-quoted strings. All cosmetic/valid.)

// DecodeUserOverrides — the config bridge (frozen config.go comment). The round-trip preserves FINDING C
// (absent keys → nil) through the re-encode, so a partial [provider.X] becomes a partial Manifest that
// MergeManifest will correctly overlay onto a built-in.
data, _ := toml.Marshal(entry)        // map[string]any → TOML
var m Manifest
_ = toml.Unmarshal(data, &m)          // TOML → Manifest (absent → nil)
m.Name = name                         // table key is the identity
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. go-toml/v2 v2.4.2 is already required (P1.M1.T4.S2); registry.go uses it in non-test
        code (the FIRST non-test use in provider — BY DESIGN). `go mod tidy` MUST be a no-op.
        `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - internal/provider → (stdlib: fmt, os/exec, sort) + github.com/pelletier/go-toml/v2.
  - internal/provider → internal/config : FORBIDDEN (cycle; S1 design call #4). DecodeUserOverrides takes
        the raw-map TYPE (map[string]map[string]any) WITHOUT importing config. The WIRING layer
        (config loader or CLI, a later subtask) calls provider.DecodeUserOverrides(config.Providers).
  - The registry does NOT import internal/git, cmd, or anything else.

FROZEN FILES (do NOT edit):
  - internal/provider/manifest.go + manifest_test.go (S1): Manifest + DetectCommand + Validate + Resolve
        + strPtr/boolPtr are a CONTRACT.
  - internal/provider/merge.go + merge_test.go (S2): MergeManifest is a CONTRACT.
  - internal/provider/builtin.go + builtin_test.go (S2 + parallel S3): BuiltinManifests() (6 keys) is a
        CONTRACT.
  - internal/config/* (P1.M1.T4), internal/git/* (P1.M1.T2/T3), cmd/stagecoach/main.go, Makefile.

DOWNSTREAM CONTRACTS (do NOT implement here — just honor the shapes they will consume):
  - P1.M4.T1.S3 (providers list/show): reg.List() → render rows (name, installed=reg.IsInstalled(m),
        default=reg.DefaultProvider(installedNames)); reg.MarshalTOML(name) → print TOML. The CLI computes
        installedNames by ranging List() and collecting names where IsInstalled is true.
  - P1.M3.T4 (generate flow): reg.Get(cfg.Provider) → Validate → Resolve → hand to renderer/executor/parser.
  - Config wiring (a later subtask): overrides, err := provider.DecodeUserOverrides(cfg.Providers);
        reg := provider.NewRegistry(overrides).
  => The Registry API (NewRegistry/Get/List/IsInstalled/MarshalTOML/DefaultProvider/DecodeUserOverrides)
     is now FROZEN for downstream. Do not change signatures after this subtask.

NO DATABASE / NO ROUTES / NO CLI / NO RENDER/EXEC/PARSE (T4/T5/T6) / NO CONFIG EDITS / NO BUILT-IN CHANGES.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Tasks 1–3 (registry.go):
gofmt -w internal/provider/registry.go internal/provider/registry_test.go
test -z "$(gofmt -l internal/provider/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/provider/        # (and `go vet ./...`) Expect zero diagnostics.
go build ./...                     # Whole module compiles (incl. the new package members). Expect exit 0.
# Expected: clean. Verify the EXACT import set in registry.go (fmt, os/exec, sort, go-toml/v2):
grep -nE '^import|^\s"' internal/provider/registry.go
# Expected: an import block with exactly fmt, os/exec, sort, github.com/pelletier/go-toml/v2. NO internal/config.

# Confirm NO new dependency + NO edit to frozen/S1/S2/S3 files + no config edge:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"   # MUST be empty.
git diff --exit-code -- internal/provider/manifest.go internal/provider/manifest_test.go \
  internal/provider/merge.go internal/provider/merge_test.go \
  internal/provider/builtin.go internal/provider/builtin_test.go \
  internal/config internal/git cmd Makefile && echo "frozen + S1/S2/S3 files UNCHANGED (expected)"   # MUST be empty.
grep -n 'internal/config' internal/provider/registry.go && echo "BAD: config import" || echo "no config import (good)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# The ~12 test groups (white-box; no git/config/generate needed — pure data structure + toml round-trips
# + one exec.LookPath probe):
go test -race ./internal/provider/ -v
# Expected: PASS — TestPreferredBuiltins_MatchesBuiltinKeys, TestNewRegistry_NoOverrides_HasAllBuiltins,
#   TestNewRegistry_OverrideExisting_OnlyTouchedFieldChanges (§16.1 keystone), TestNewRegistry_OverrideExisting_
#   ExplicitZeroWins, TestNewRegistry_NewName_AddedVerbatim, TestGet_MissingReturnsFalse, TestList_SortedByName,
#   TestIsInstalled, TestMarshalTOML_RoundTrip, TestMarshalTOML_UnknownErrors, TestMarshalTOML_ReflectsMerge,
#   TestDefaultProvider, TestDecodeUserOverrides — PLUS S1/S2/S3 tests still green.

# Full suite must stay green (no regression; confirms no stray import edge broke config/git):
go test -race ./...
# Expected: all packages PASS (config, git, provider).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build + scope/additive checks:
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"   # main.go stub still links.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged"
# Confirm this subtask touched ONLY the two new files:
git diff --exit-code -- internal/config internal/git cmd Makefile \
  internal/provider/manifest.go internal/provider/manifest_test.go \
  internal/provider/merge.go internal/provider/merge_test.go \
  internal/provider/builtin.go internal/provider/builtin_test.go && echo "frozen + S1/S2/S3 UNCHANGED by this subtask"
grep -n 'func NewRegistry\|func (r \*Registry)\|func DecodeUserOverrides' internal/provider/registry.go
# Expected: prints NewRegistry, Get, List, IsInstalled, MarshalTOML, DefaultProvider, DecodeUserOverrides.

# Coverage of the new code (Makefile has a coverage target):
go test -race ./internal/provider/ -coverprofile=/tmp/cov.out && go tool cover -func=/tmp/cov.out | grep -iE 'registry|NewRegistry|IsInstalled|MarshalTOML|DefaultProvider|DecodeUserOverrides'
# Expected: the registry functions at (or near) 100% line coverage — every branch (override vs new,
# installed true/false/empty, known/unknown name, present/absent override fields) hit by the test table.

# Smoke the end-to-end config→registry→TOML path (sanity for P1.M4.T1.S3/P1.M3.T4 authors):
# a throwaway in-package test is already the cleanest expression — TestDecodeUserOverrides +
# TestMarshalTOML_RoundTrip + TestNewRegistry_OverrideExisting_* ARE the smoke test. (DecodeUserOverrides
# → NewRegistry → Get → MarshalTOML is exercised across the test groups; no standalone binary needed.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Lint gate (project-wide):
golangci-lint run ./internal/provider/ 2>/dev/null || echo "golangci-lint not installed (Makefile lint gate is project-wide; run `make lint` in CI)."

# Property-style invariant (optional, recommended): for a few random userOverride maps (each a random
# SUBSET of the built-in names overridden with a random single field, plus a couple of new names), the
# registry MUST (1) contain exactly len(BuiltinManifests()) + (new names) entries; (2) for every
# overridden built-in, ONLY the overridden field differs from the built-in (rest inherited); (3) List()
# is always sorted; (4) MarshalTOML round-trips. A short loop over field-kind × present/absent covers it.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` is a
      no-op; `git diff --exit-code go.mod go.sum` empty; registry.go imports EXACTLY fmt/os/exec/sort/go-toml.
- [ ] Level 2 green: `go test -race ./internal/provider/ -v` (all ~12 registry groups + S1/S2/S3 tests) AND
      `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; every frozen + S1/S2/S3 file unchanged; the 7
      exported identifiers present.

### Feature Validation

- [ ] `NewRegistry(map[string]Manifest) *Registry` exists with the exact contract signature; seeds from
      `BuiltinManifests()`; merges overrides onto built-ins via `MergeManifest`; adds new names verbatim.
- [ ] `Get(name) (Manifest, bool)`: known → (manifest, true); unknown → (zero, false).
- [ ] `List() []Manifest`: all manifests, sorted ascending by Name, fresh slice.
- [ ] `IsInstalled(m)`: false for empty DetectCommand + bogus command; true for a $PATH-present binary;
      uses `DetectCommand()` (so cursor's "agent" is probed, not "cursor").
- [ ] `MarshalTOML(name)`: known → TOML string of the stored (merged, unresolved) manifest (round-trips);
      unknown → wrapped error; reflects merged overrides.
- [ ] `DefaultProvider(installed)`: first preferred installed built-in (pi first); "" if none; ignores
      user-defined names.
- [ ] `DecodeUserOverrides(raw)`: bridges `map[string]map[string]any` → `map[string]Manifest`; absent
      fields → nil; Name from key; nil input → empty non-nil map; malformed entry → wrapped error.
- [ ] The §16.1 keystone: overriding pi's `default_model` changes ONLY `default_model` (registry level).
- [ ] An explicit-zero override (`strip_code_fence=false`, `print_flag=""`) wins at the registry level.
- [ ] A brand-new §12.8 provider is added verbatim with Name from the key.

### Code Quality Validation

- [ ] Follows repo conventions: white-box `package provider`, stdlib `testing`, `t.Errorf` assertions,
      `reflect.DeepEqual` for structs/slices/maps (mirrors `internal/provider/merge_test.go`/`builtin_test.go`).
- [ ] File placement matches the desired tree (`registry.go` + `registry_test.go` only — all frozen files
      untouched).
- [ ] `internal/provider` imports nothing outside stdlib + go-toml/v2; NEVER `internal/config`.
- [ ] No premature scope: no Validate/Resolve inside NewRegistry/MarshalTOML; no render/exec/parse
      (T4/T5/T6); no CLI (P1.M4); no config edits; no built-in changes.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Doc comments on every exported identifier (Registry, NewRegistry, Get, List, IsInstalled,
      MarshalTOML, DefaultProvider, DecodeUserOverrides) citing the PRD section + the relevant design call.
- [ ] No new env vars / no user-facing docs (PRD "DOCS: none — internal"; the `providers` UX docs come
      with P1.M4.T1.S3, Mode A).
- [ ] `internal/provider/registry.go` + `registry_test.go` are the ONLY files touched.

---

## Anti-Patterns to Avoid

- ❌ Don't Validate or Resolve inside `NewRegistry` / `MarshalTOML`. The contract signature returns
  `*Registry` (no error); lazy validation is the model. The CONSUMER does `Get → Validate → Resolve`.
  Marshaling without Validate lets `providers show` display a partial provider for debugging.
- ❌ Don't call `m.Resolve()` before marshaling in `MarshalTOML`. Marshal the STORED (merged) manifest —
  go-toml omits nil pointers (free omitempty), showing what's actually configured. Resolve() would inflate
  defaults the user didn't set, misleading `providers show`. (Flagged as the ONE interpretation risk; a
  one-line flip if FR47 is later read otherwise.)
- ❌ Don't re-implement field-by-field merge in `NewRegistry`. Delegate to S2's `MergeManifest` — it already
  handles explicit-zero wins, slice wholesale-replace, Env key-by-key, and no-input-mutation. NewRegistry
  only adds "seed built-ins + route override-vs-new + set Name".
- ❌ Don't make `DecodeUserOverrides` a method, and don't fold it into `NewRegistry`. The contract pins
  `NewRegistry(map[string]Manifest)`. DecodeUserOverrides is a FREE FUNCTION that produces that map from
  the raw config shape — the only place that knows `map[string]any`.
- ❌ Don't import `internal/config` (cycle). DecodeUserOverrides takes the raw-map TYPE
  (`map[string]map[string]any`) without importing the config package. The wiring layer passes
  `config.Providers` through.
- ❌ Don't probe `m.Name` or `*m.Command` directly in `IsInstalled`. Use `m.DetectCommand()` — it handles
  Detect-vs-Command precedence AND the empty case, and correctly probes cursor's `agent` (not `cursor`).
- ❌ Don't omit the `cmd == ""` guard in `IsInstalled`. A half-defined §12.8 manifest (no Detect/Command)
  has `DetectCommand()==""`; short-circuit to false (avoids a pointless `LookPath("")` that errors anyway).
- ❌ Don't make `DefaultProvider` exec. It takes the installed NAMES as a param (the caller computes them
  via `IsInstalled` over `List()`). This keeps it pure + trivially testable.
- ❌ Don't forget to set `.Name = name` in BOTH branches of `NewRegistry` (override + new). The decoded
  body has no `name`; the table key is the identity. A §12.8 provider would otherwise have `Name==""`.
- ❌ Don't store `BuiltinManifests()`'s return map and mutate it. Copy via `range` into the registry's own
  map (defense-in-depth, even though `BuiltinManifests()` is fresh per call).
- ❌ Don't return the internal `manifests` map from `List()` or expose it. `List()` returns a fresh sorted
  SLICE; mutation of the registry's internal map by a caller would break determinism.
- ❌ Don't edit `manifest.go`/`merge.go`/`builtin.go` (+ their tests) — they are frozen contracts (S1/S2/S3).
  This subtask ADDS `registry.go` + `registry_test.go` only.
- ❌ Don't add imports beyond `fmt`, `os/exec`, `sort`, `github.com/pelletier/go-toml/v2` to `registry.go`.
  An unused import fails `go vet`; an extra dep fails the go.mod-unchanged gate.
- ❌ Don't implement render/exec/parse (P1.M2.T4/T5/T6), the CLI (P1.M4), config edits, or new built-ins
  here. This subtask is the registry + its bridge + tests.
