# Research: Provider Registry — merge semantics, $PATH detection, TOML marshal, and the config bridge

> Subtask P1.M2.T3.S1 — `internal/provider/registry.go`. Empirically verified against the real
> `github.com/pelletier/go-toml/v2 v2.4.2` (the version pinned in `go.mod`) and the already-landed
> `Manifest` (S1, `manifest.go`), `MergeManifest` (S2, `merge.go`), and `BuiltinManifests` (S2+S3,
> `builtin.go`). Built-ins will be 6 keys (pi/claude/gemini/opencode/codex/cursor) once the parallel
> P1.M2.T2.S3 lands — this PRP assumes that contract.

---

## 1. The registry's three jobs (mapped to the work-item contract)

The work-item contract names exactly these methods on a `Registry` struct holding `map[string]Manifest`:

| Contract method | What it does | Source of truth |
|---|---|---|
| `NewRegistry(userOverrides map[string]Manifest) *Registry` | seed with `BuiltinManifests()`; for each override, `MergeManifest` if name exists else add new | PRD §16.1 + §12.8; S2 `MergeManifest` |
| `Get(name) (Manifest, bool)` | map lookup | — |
| `List() []Manifest` | all manifests, sorted by `Name` | `sort.Slice` |
| `IsInstalled(m Manifest) bool` | `exec.LookPath(m.DetectCommand())` == nil | FR46; stdlib `os/exec` |
| `MarshalTOML(name) (string, error)` | `toml.Marshal` of the stored manifest | FR47; go-toml/v2 |
| `DefaultProvider(installed []string) string` | first preferred built-in that is installed (pi first) | FR46 |

Plus ONE bridge helper this PRP adds (see §5): `DecodeUserOverrides(map[string]map[string]any) (map[string]Manifest, error)`.

---

## 2. NewRegistry merge algorithm (verified against S1+S2)

```
manifests := copy of BuiltinManifests()   // fresh per call → no shared-mutation hazard
for name, override := range userOverrides:
    if base, ok := manifests[name]; ok:   // OVERRIDE an existing built-in
        merged := MergeManifest(base, override)   // S2: field-by-field, nil/empty→inherit
        merged.Name = name                       // table key is authoritative identity
        manifests[name] = merged
    else:                                  // DEFINE a brand-new §12.8 provider
        override.Name = name               // body carries no "name"; key is the identity
        manifests[name] = override
```

- **Override path** uses S2 `MergeManifest`, which already (a) preserves every non-overridden built-in
  field, (b) honors an explicit-zero override (`strip_code_fence=false`, `print_flag=""`), (c) never
  mutates `base`. So NewRegistry inherits all of S2's guarantees for free. The §16.1 keystone —
  merging `{default_model:"glm-5.2"}` onto pi changes ONLY `default_model` — holds at the registry
  level (TestNewRegistry_OverrideExisting_OnlyTouchedFieldChanges).
- **New-name path** stores the override verbatim (it IS the whole manifest). No merge base.
- **Name handling**: `name` is the `[provider.<name>]` table key, NOT a field in the body (verified:
  PRD §12.8's `[provider.myagent]` block has no `name =` line). So a decoded override always has
  `Name == ""` until the registry/decoder sets it from the key. Setting it explicitly in BOTH branches
  is defensive + idempotent (a caller that already set Name is unaffected).
- **No Validate / no Resolve in NewRegistry** — see §6.

---

## 3. IsInstalled — exec.LookPath + DetectCommand (verified)

- `m.DetectCommand()` (S1): `*Detect` if non-nil & non-empty, else `*Command`, else `""`.
- `exec.LookPath(cmd)`: returns `(path, nil)` if `cmd` is found on `$PATH`, else a non-nil error.
- `IsInstalled(m) = (m.DetectCommand() != "" && exec.LookPath(...) == nil)`.
- **Edge case**: `DetectCommand()==""` (both Detect+Command nil) → short-circuit `false` (LookPath("")
  would error anyway, but the explicit guard is clearer and avoids a pointless exec). This is the
  correct behavior for a half-defined §12.8 provider that omits `command`.
- **cursor is the ONLY built-in where Detect ≠ Name**: cursor.Detect == "agent" (the binary),
  cursor.Name == "cursor". `IsInstalled(cursorManifest)` therefore looks up `agent` on $PATH —
  exactly right (the Cursor Agent CLI ships as `agent`, not `cursor`).

**Deterministic test approach** (exec.LookPath depends on $PATH):
- FALSE (deterministic): bogus command `"definitely-not-a-real-binary-xyz123"`; empty DetectCommand.
- TRUE: `"go"` is guaranteed on $PATH during `go test` (it's how the test is invoked). Guard with
  `t.Skip` if `exec.LookPath("go")` fails, so the suite never flakes on an odd CI image.
- Detect-wins: a manifest `{Detect:"bogus", Command:"go"}` → false (proves DetectCommand is used).

---

## 4. MarshalTOML — go-toml/v2 Marshal behavior (EMPIRICALLY VERIFIED)

Ran `toml.Marshal(Manifest{...})` against a manifest with SOME nil fields (a §12.8 provider missing
Detect/Subcommand/StripCodeFence/Env). Output:

```toml
name = 'myagent'
command = '/opt/myagent/bin/agent'
subcommand = []
prompt_delivery = 'stdin'
print_flag = '--once'
model_flag = '--model'
default_model = 'my-model-7b'
bare_flags = ['--no-mcp', '--ephemeral']
output = 'raw'
```

Confirmed (all consistent with S1's `research/go-toml-pointer-behavior.md`):
- **nil `*string`/`*bool` pointers are OMITTED** (Detect, StripCodeFence, Env absent above). This is
  the free omitempty (FINDING A) — no custom marshaler needed for a clean `providers show` output.
- **nil slices marshal as `[]`** (subcommand = []). Acceptable; not omitted. (Cosmetic only; if
  P1.M4.T1.S3 wants them omitted too, it adds a custom `MarshalTOML` there — out of scope here.)
- **nil map (Env) is omitted** entirely (no `[env]` header).
- **Single-quoted literal strings** (`'myagent'`, not `"myagent"`). Valid TOML; cosmetic. The
  hand-written `providers/*.toml` reference files (P1.M5.T2) are independent of this output.

**Design decision — marshal the STORED (merged, unresolved) manifest, NOT a Resolve() copy.** FR47
says "fully-resolved manifest (built-in merged with user overrides)". The parenthetical defines
"resolved" as PRECEDENCE-resolution (merge), not default-resolution (Resolve). Marshaling the merged
manifest shows exactly what is configured (inherited + overridden); nil optionals are suppressed by
go-toml. For built-ins + full overrides the output is complete. `Resolve()` (defaults-inflated) is a
display transform that belongs to the CLI `providers show` command (P1.M4.T1.S3), not this method —
keeping `MarshalTOML(name)` a thin "name → TOML" lookup. **This is the ONE interpretation risk in the
PRP; if FR47 is later read as "Resolve() first", it's a one-line change** (`toml.Marshal(m.Resolve())`).
The test asserts a round-trip (marshal → unmarshal → reflect.DeepEqual the stored manifest), which is
unambiguous either way.

---

## 5. The config bridge — DecodeUserOverrides (the frozen config.go comment)

`internal/config/config.go` (P1.M1.T4, FROZEN) carries provider overrides as a RAW map:

```go
Providers map[string]map[string]any `toml:"-"`
// doc comment: "The registry (P1.M2.T3) consumes this map — for each name it re-encodes the entry to
// TOML and unmarshals into a Manifest, then field-merges with the built-in manifest per PRD §16.1."
```

But the work-item contract signature is `NewRegistry(userOverrides map[string]Manifest)` — decoded
Manifests, NOT the raw map. So the re-encode step must live in a **separate free function** that
produces the `map[string]Manifest` NewRegistry consumes. This PRP adds it as
`DecodeUserOverrides(raw map[string]map[string]any) (map[string]Manifest, error)`:

```
for name, entry := range raw:
    data := toml.Marshal(entry)          // map[string]any → TOML (verified §5.8 pattern)
    var m Manifest; toml.Unmarshal(data, &m)   // TOML → Manifest (absent keys → nil; FINDING C)
    m.Name = name                        // table key is the identity (body has no "name")
    out[name] = m
```

**EMPIRICALLY VERIFIED round-trip** (`map[string]any` → `toml.Marshal` → `toml.Unmarshal` into Manifest):
- Present fields (command/prompt_delivery/print_flag/default_model/bare_flags) decode correctly.
- Absent fields (Detect/Subcommand/Env/StripCodeFence) → nil (FINDING C preserved through the bridge).
- `Name == ""` (no `name` key in body) → set from the table key.
- `bare_flags` as `[]any{"--no-mcp","--ephemeral"}` (the shape go-toml produces when decoding a TOML
  array into `map[string]any`) re-encodes + decodes into Manifest's `[]string` correctly.

**Why a free function, not a method / not inside NewRegistry:**
1. The contract pins `NewRegistry(map[string]Manifest)` — the raw map cannot be its param.
2. The registry must stay config-free (S1 design call #4: provider imports neither config nor toml in
   the TYPE; the registry file legitimately imports toml for MarshalTOML + this bridge, but NEVER
   `internal/config`). DecodeUserOverrides takes the raw-map TYPE without importing config → no cycle.
3. It is independently testable (feed it a hand-built `map[string]map[string]any`; no config load).
4. It is the ONLY place that knows the `map[string]any` shape; everything downstream is typed Manifests.

**This is the bridge the frozen config.go comment assigns to "the registry (P1.M2.T3)".** Without it,
`NewRegistry` could never consume `config.Providers` and FR48 (user overrides) would be unwirable. It
is therefore in-scope for this subtask even though the contract's method list doesn't name it.

---

## 6. Lifecycle: where Validate + Resolve happen (design decision)

- **NewRegistry does NOT Validate or Resolve.** The contract signature returns `*Registry` (no error),
  so validation cannot surface there. A partial §12.8 override legitimately lacks fields; lazy
  validation is the right model.
- **MarshalTOML does NOT Validate** (so `providers show` can display a partially-defined provider for
  debugging — useful).
- **The consumer lifecycle** (renderer P1.M2.T4 / generate flow P1.M3.T4): `m, ok := reg.Get(name)`;
  `m.Validate()`; `m := m.Resolve()`; consume. This matches S1/S2's documented lifecycle
  (decode → merge → Validate → Resolve → consume) — the registry owns "merge + store"; the consumer
  owns "Validate + Resolve". This keeps the registry a pure data structure.

---

## 7. DefaultProvider — preference order (pi first)

- Only BUILT-IN names are candidates (PRD: auto-detect picks among the shipped agents; a user-defined
  §12.8 provider is never auto-selected — the user names it explicitly).
- Preference order (PRD §12.3–12.7 listing order, pi first):
  `["pi", "claude", "gemini", "opencode", "codex", "cursor"]`.
- `installed` is the caller's list of installed provider NAMES (computed via IsInstalled over List()).
  Taking it as a param makes DefaultProvider pure/testable (no exec inside).
- Returns the first preferred built-in in `installed`; "" if none installed.
- **Safety net test**: assert `preferredBuiltins` is exactly the set of `BuiltinManifests()` keys and
  `preferredBuiltins[0] == "pi"`. If a future built-in is added without updating the list, this fails.

---

## 8. Imports & module impact

`registry.go` imports: `fmt`, `os/exec`, `sort`, `github.com/pelletier/go-toml/v2`.
- This is the FIRST non-test file in `internal/provider` to import go-toml. That is BY DESIGN (the
  registry is the marshal + config-bridge layer). go-toml/v2 v2.4.2 is already in `go.mod`
  (P1.M1.T4.S2) → **go.mod/go.sum unchanged**. No `internal/config` import (cycle guard).
- `registry_test.go` imports: `testing`, `reflect`, `os/exec` (for the IsInstalled true-case skip
  guard), `github.com/pelletier/go-toml/v2` (for MarshalTOML + DecodeUserOverrides round-trips).

## 9. Files touched / NOT touched

- NEW: `internal/provider/registry.go`, `internal/provider/registry_test.go`.
- UNCHANGED: `manifest.go`/`manifest_test.go` (S1), `merge.go`/`merge_test.go` (S2),
  `builtin.go`/`builtin_test.go` (S2 + parallel S3), all of `internal/config/*`, `internal/git/*`,
  `cmd/stagecoach/main.go`, `Makefile`, `go.mod`, `go.sum`.
