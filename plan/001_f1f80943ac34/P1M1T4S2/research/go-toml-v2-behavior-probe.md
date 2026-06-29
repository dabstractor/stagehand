# Research: go-toml/v2 empirical behavior probe (for P1.M1.T4.S2 + downstream P1.M2.T3)

> Verified by execution against `github.com/pelletier/go-toml/v2 v2.4.2` (the version S1 pinned in
> `go.mod`/`go.sum`). These findings directly drive the S2 decode design and the P1.M2.T3 registry's
> consumption of `Config.Providers`.

## Setup

A throwaway `module tomlprobe` (Go 1.22, `GOTOOLCHAIN=local GOPROXY=off go run`) decoded a §16.2-shaped
document through three struct shapes: (1) plain nested with `Timeout string`, (2) pointer-typed
subtable, (3) marshaled back out. Document used:

```toml
[defaults]
provider = "pi"
timeout = "120s"
auto_stage_all = false          # explicit false — the crux case

[generation]
max_diff_bytes = 999999
output = "json"

[provider.pi]
default_model = "glm-5.2"
default_provider = "zai"

[provider.myagent]
command = "/opt/myagent/bin/agent"
bare_flags = ["--no-mcp", "--ephemeral"]
```

## Finding A — `[provider.X]` decodes into `map[string]map[string]any` natively ✅

```go
type fileConfig struct {
    Defaults   fileDefaults             `toml:"defaults"`
    Generation fileGeneration           `toml:"generation"`
    Provider   map[string]map[string]any `toml:"provider"`
}
```

Decoded result (exact):

```
len(Provider)=2
  [pi]      = map[default_model:glm-5.2 default_provider:zai]
  [myagent] = map[command:/opt/myagent/bin/agent bare_flags:[--no-mcp --ephemeral]]
```

Inner value types observed:
- string TOML value  → Go `string`           (`default_model`, `command`)
- array TOML value   → Go `[]interface{}`     (`bare_flags` → `[]interface {}{"--no-mcp", "--ephemeral"}`)
- (int TOML value    → Go `int64` — relevant for P1.M2.T3, not exercised in provider bodies above)

**Implication for S2:** `map[string]map[string]any` is a valid, zero-friction field type for the
file-decode struct AND for `Config.Providers` (S2 carries the raw map through to the registry).
**Implication for P1.M2.T3 (registry):** when materializing a `Manifest` from an entry, type-assert
`int64`→`int` for int fields and `[]any`→`[]string` for `bare_flags`; OR re-encode the entry map to
TOML bytes and `toml.Unmarshal` into a pointer-typed `Manifest` (cleaner — see arch §5.8 round-trip).

## Finding B — pointer fields PERFECTLY distinguish present vs absent (incl. `*bool = false`) ✅

```go
type pDefaults struct {
    Provider     *string `toml:"provider"`
    AutoStageAll *bool   `toml:"auto_stage_all"`
}
```

With `auto_stage_all = false` present in the file, `model` absent:

```
Defaults.Provider     ptr == nil? false   (file DID set provider)
Defaults.AutoStageAll ptr == nil? false   (file DID set auto_stage_all=false)
  *AutoStageAll = false   <-- pointer CORRECTLY captures the false value
```

With the `[defaults]` section entirely absent from the file:

```
Provider ptr nil? true   AutoStageAll ptr nil? true   <-- both nil => absent
```

**Implication:** arch §5.4 Option A (pointer-typed fields) is a viable, correct way to track field
presence for an overlay/merge layer. This is the documented "Future Refinement" for S2's overlay
(see PRP §Known Gotchas) if full `bool=false` file overrides become required. S2's *contract-specified*
overlay is non-zero (plain `*Config`), which cannot use this — but the probe proves the upgrade path
is real and cheap.

## Finding C — plain types CANNOT distinguish absent vs zero-value (the bool caveat) ⚠️

With a plain `AutoStageAll bool`, a file that sets `auto_stage_all = false` produces `AutoStageAll: false`,
which is byte-identical to "section absent" (`AutoStageAll: false` zero value). Confirmed by probe.
This is exactly FINDING 5 / arch §2.4's documented caveat. S2's non-zero overlay therefore cannot
propagate a `false` (or `0` / `""`) override from a file layer — see PRP for the affected-field list
and the env/CLI escape hatches.

## Finding D — marshaling a nested decode struct emits ALL fields (no omitempty) ✅

Marshaling the nested `fileConfig` produced `model = ''`, `max_md_lines = 0`, `verbose = false`, etc.
— every tagged field is emitted; go-toml/v2 has no `omitempty` (FINDING 5 confirmed). The provider
section marshals as a parent `[provider]` table with `[provider.<name>]` subtables (valid TOML, §16.2
shape). Relevant only for `config init` (P1.M4.T1.S4), NOT for S2 (S2 only reads, never writes files).

## Finding E — repo-local path is `.stagehand.toml` (file), NOT `.stagehand/config.toml` (dir) ⚠️

- Contract (P1.M1.T4.S2 point 1) + PRD §16.1 (h3.57 item 4): repo-local = **`./.stagehand.toml`**
  (a file in the repo root).
- arch `go_ecosystem_patterns.md` §2.8 uses **`filepath.Join(".stagehand", "config.toml")`** (a
  directory) — this is INCONSISTENT with the contract/PRD.
- **Resolution:** the contract + PRD win. `repoLocalConfigPath() == ".stagehand.toml"`. The arch §2.8
  snippet is the *global-path* + *Load-orchestration* pattern reference, NOT the repo-path authority.

## Decision summary carried into the PRP

1. file-decode struct `fileConfig` = nested `[defaults]`/`[generation]`/`[provider.X]`, **plain**
   scalar types (matches arch §2.4 + the contract's non-zero overlay), `Timeout string`, and
   `Provider map[string]map[string]any` (Finding A).
2. `loadTOML` materializes non-zero fields into the plain resolved `*Config`, parsing the `Timeout`
   string via `time.ParseDuration`.
3. `overlay(dst, src *Config)` = non-zero scalar overlay + provider-map key-replace (arch §2.4).
4. The `bool=false` / zero-value gap (Finding C) is documented as a bounded v1 tradeoff; the
   pointer-based fix (Finding B) is the noted future refinement.
5. `Config` gains `Providers map[string]map[string]any \`toml:"-"\`` (raw, marshal-excluded — avoids
   tag clash with `Provider string`; consumed by P1.M2.T3 per Finding A's typing notes).
