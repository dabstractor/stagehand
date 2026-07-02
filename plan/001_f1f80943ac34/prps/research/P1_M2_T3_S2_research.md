# Research Notes — P1.M2.T3.S2: provider/registry.go (override merge + PATH detect)

## Dependency (S1) surface consumed
- `Builtins() map[string]Manifest` in `internal/provider/builtin.go` — returns a FRESH
  map on every call (so the registry may take ownership / mutate a clone).
- `Manifest` struct in `internal/provider/manifest.go` — value type with:
  - scalar strings: Name, Detect, Command, PromptDelivery, PromptFlag, PrintFlag,
    ModelFlag, DefaultModel, SystemPromptFlag, ProviderFlag, DefaultProvider, Output,
    JSONField, RetryInstruction
  - slices: Subcommand []string, BareFlags []string
  - map: Env map[string]string
  - bool: StripCodeFence  ← zero value is `false`; cannot be set to false via merge.

## Merge semantics (decisions.md §6 + task contract)
"Provider manifests merge field-by-field (a user override setting only default_model
keeps the rest of the built-in manifest)."
- scalar string X: `if ov.X != "" { out.X = ov.X }`
- slice X (Subcommand/BareFlags): `if len(ov.X) > 0 { out.X = copy }` → replaced wholesale
- map X (Env): `if len(ov.X) > 0 { out.X = copy }` → replaced wholesale (NOT deep-merged)
- bool StripCodeFence: `if ov.StripCodeFence { out.StripCodeFence = true }`
  (KNOWN LIMITATION: cannot flip builtin true→false; document, do not block.)
- new name (no builtin): override used AS-IS (cloned).

### Slice "present" decision: len()>0 vs !=nil
Chose `len() > 0` to match the contract's literal word "non-empty". Both interpretations
pass the required test cases (override{BareFlags:[...]} non-nil non-empty; nil when
unset). Divergence only on an explicit `bare_flags = []` in TOML (would NOT clear under
len>0). go-toml/v2 unmarshals `[]`→`[]string{}` (non-nil). Accepted as v1 behavior.

## Defensive-copy requirement
mergeManifest / new-name clone MUST copy slices & maps (append([]string(nil), s...) and
a fresh map) so the registry's merged state never aliases the caller's override map or
the Builtins() return after construction. Mirrors the "fresh, independently-addressable"
philosophy asserted in builtin_test.go (TestBuiltins_KeysAndCount).

## API (verified against downstream consumers)
- `NewRegistry(builtins, overrides map[string]Manifest) *Registry` — precompute merged map.
- `Get(name string) (Manifest, bool)` — ok=false for unknown names. (M5 config Load +
  M7 `providers show` use this; show errors on ok=false.)
- `List() []string` — sorted union of builtin+override keys (FR46 `providers list`).
- `Detect() map[string]bool` — for each name, merged manifest, exec.LookPath(m.Detect or
  m.Command). (FR46 marks detected providers; FR47 resolved manifest.)

## Hard constraints / scope boundaries
- `provider/registry` MUST NOT import `internal/config` (plan_overview key decision 1 →
  no import cycle). Overrides injected as `map[string]Manifest`. Verified: only imports
  needed are "os/exec" and "sort".
- Same `package provider` (plain package line — package doc lives in manifest.go).
- Pure-ish: only Detect() does I/O (exec.LookPath); merge/List/Get are pure.

## Verification environment (confirmed live)
- `go test ./...` works; `go vet ./...` clean; `gofmt -l internal/provider/` empty.
- `pi` IS on $PATH (`/home/dustin/.local/bin/pi`) → Detect()["pi"] must be true.
- `definitely-not-an-agent-xyz` NOT on $PATH → must be false. Use as an override with
  Command="definitely-not-an-agent-xyz" so Detect exercises the not-found branch.

## Test patterns to mirror (builtin_test.go conventions)
- package `provider` (white-box), table-less focused funcs or t.Run subtests.
- `reflect.DeepEqual` for struct/map comparisons; `normalizeEmpty` exists if needed but
  registry should keep nil vs [] distinct intentionally.
- Inline Manifest literals for determinism.
- Name tests `TestRegistry_<Behavior>`.
