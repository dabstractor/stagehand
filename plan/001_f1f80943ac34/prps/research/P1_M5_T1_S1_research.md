# Research — P1.M5.T1.S1: internal/config/config.go Config + internal/config/defaults.go

## Task contract (verbatim from tasks.json P1.M5.T1.S1)
- **INPUT**: `provider.Manifest` (P1.M2.T1.S1) — used for `ProviderOverrides map[string]Manifest`.
- **LOGIC**: `Config` struct fields: `Provider, Model string; Timeout time.Duration; AutoStageAll, Verbose, NoColor bool; MaxDiffBytes, MaxMdLines, MaxDuplicateRetries, SubjectTargetChars int; Output string; StripCodeFence bool; ConfigPath string; ProviderOverrides map[string]Manifest`.
- **Default()**: §16.1 values — Timeout 120s, AutoStageAll true, MaxDiffBytes 300000, MaxMdLines 100, MaxDuplicateRetries 3, Output "raw", StripCodeFence true, SubjectTargetChars 50. ProviderOverrides EMPTY (built-in manifests applied separately by registry).
- **MOCKING tests**: Default() returns the exact table; round-trip a populated Config.
- **OUTPUT**: Config + Default() consumed by Load() (P1.M5.T3.S1), generate (P1.M6.T1.S1), CLI (P1.M7.T2).

## Source-of-truth documents read
- **PRD §16.1 (lines 1002–1012)** — resolution order; bullet 1 is the defaults table:
  `timeout 120s, auto_stage_all true, max_diff_bytes 300000, max_md_lines 100, max_duplicate_retries 3, output raw, strip_code_fence true`.
  NOTE: §16.1 does NOT list `subject_target_chars` in bullet 1, but §16.2 (line 1032) sets `subject_target_chars = 50` and the work item contract EXPLICITLY mandates `SubjectTargetChars 50` in Default(). ⇒ include it.
- **PRD §16.2 (lines 1014–1050)** — full TOML example. CRITICAL OBSERVATION: the file is split across `[defaults]` and `[generation]` tables, with `[provider.<name>]` tables for overrides. ⇒ a FLAT `Config` struct CANNOT be directly go-toml-unmarshaled from this file (scalars live under two different tables). The TOML DTO→Config assembly belongs to T2.S1 (file loaders), NOT this task.
- **PRD §16.3 (lines 1051–1062)** — git-config keys (`stagehand.*`, `autoStageAll` camelCase, booleans via `git config --bool`). T2.S1 concern.
- **decisions.md §6** — restates precedence + the defaults table identically: "Defaults: timeout 120s, auto_stage_all true, max_diff_bytes 300000, max_md_lines 100, max_duplicate_retries 3, output raw, strip_code_fence true, subject_target_chars 50." Confirms ProviderOverrides merge is "field-by-field" (registry concern, M2.T3.S2 — already shipped).
- **plan_overview.md key decision 1**: "Manifest type lives in provider (M2); config (M5) imports it. provider/registry does NOT import config." ⇒ `config` imports `provider`; NO cycle.

## The input type (internal/provider/manifest.go — DONE)
- `type Manifest struct {...}` with `toml:"..."` tags on every field (snake_case §12.1 keys).
- Exported enum constants: `provider.OutputRaw = "raw"`, `provider.OutputJSON = "json"`.
- ⇒ `Config.Output` default should reference `provider.OutputRaw` (no magic string; ties the default to the provider enum).

## How ProviderOverrides is consumed (internal/provider/registry.go — DONE)
- `NewRegistry(builtins, overrides map[string]Manifest) *Registry` — clones each builtin, then layers overrides field-by-field (`mergeManifest`); deep-copies all maps/slices.
- T3.S1 will call `provider.NewRegistry(provider.Builtins(), cfg.ProviderOverrides)`.
- ⇒ Default() MUST leave `ProviderOverrides` EMPTY (nil). The six built-in manifests are NOT baked into Config; they are injected by the registry at Load time. Baking them in here would double-layer them.

## Key design decisions for THIS task

### D1 — Two files, matching the work-item title + PRD §16.1 path
- `internal/config/config.go` — OWNS `// Package config` doc + the `Config` struct (mirror internal/git/git.go & internal/ui/exitcode.go package-doc ownership).
- `internal/config/defaults.go` — plain `package config` + exported default constants + `func Default() Config`.

### D2 — Config has NO toml struct tags (it is the resolved in-memory type)
Manifest has toml tags because it IS directly unmarshaled from `[provider.X]` tables. Config CANNOT be directly unmarshaled: §16.2 splits scalars across `[defaults]` and `[generation]`. Adding toml tags to a flat Config would be misleading. T2.S1 owns the `defaultsDTO`/`generationDTO` (tagged) → Config assembly. This keeps T1.S1 a clean type definition with zero go-toml coupling. (Confirmed: config.go must NOT import go-toml.)

### D3 — Exported default constants (centralize the §16.1 table)
`DefaultTimeout`, `DefaultAutoStageAll`, `DefaultMaxDiffBytes`, `DefaultMaxMdLines`, `DefaultMaxDuplicateRetries`, `DefaultSubjectTargetChars`, `DefaultOutput` (= `provider.OutputRaw`), `DefaultStripCodeFence`. Rationale: (a) Default() reads them; (b) M7 CLI flag defaults reference them; (c) M5.T3.S1's "leave-default sentinel" logic references them; (d) they self-document the table. Matches the manifest.go exported-constants pattern.

### D4 — ProviderOverrides stays nil in Default()
`len(nil map) == 0`; range over nil is a no-op; reads are safe. Only writes panic, and the loader assigns a fresh map when overrides exist. nil == "no overrides" is the correct default. Registry deep-copies at construction; Config does NOT clone.

### D5 — Timeout is `time.Duration` in-memory; Default = `120 * time.Second`
The TOML form `"120s"` is a STRING (§16.2 line 1024). go-toml cannot unmarshal a string into `time.Duration` directly. That conversion (custom `UnmarshalText` or a DTO string field parsed via `time.ParseDuration`) is a T2.S1 concern, NOT this task. Here Timeout is `time.Duration` and Default uses `time.Second`.

## Test plan (stdlib `testing`, package config — white-box, house convention)
- `defaults_test.go`:
  - `TestDefault_ExactTable` — assert EVERY field of `Default()` matches §16.1 + SubjectTargetChars==50, AND the zero fields (Provider=="", Model=="", Verbose/NoColor false, ConfigPath==""). The "Default() returns the exact table" MOCKING scenario.
  - `TestDefault_ProviderOverridesEmpty` — `len(Default().ProviderOverrides)==0` (built-ins NOT baked in).
- `config_test.go`:
  - `TestConfig_RoundTrip_Populated` — build a Config with every scalar set to a distinct non-default value AND a ProviderOverrides map of ≥2 Manifest entries; assert each field reads back exactly (including ProviderOverrides contents). Plus a value-copy `c2 := cfg` preserves all fields, documenting Go map reference semantics (Config does not deep-copy; registry does). The "round-trip a populated Config" MOCKING scenario.

## Validation gates (verified the toolchain works)
`go test ./...` is currently GREEN (git, prompt, provider, ui all ok; go1.26.4). Gates for this task:
- `go build ./internal/config/`
- `go vet ./internal/config/`
- `test -z "$(gofmt -l internal/config/)"`
- `go test ./internal/config/`
- `go test ./...`

## Scope boundaries (DO NOT do)
- NO go.mod/go.sum change — `time` is stdlib; `provider` is already a direct dep; config.go/defaults.go import NO go-toml. NO `go mod tidy`.
- NO file.go (T2.S1 loaders), NO Load() (T3.S1), NO docs/CONFIGURATION.md (owned by T3.S1 — changeset-level).
- main.go/Makefile/internal/{ui,provider,git,prompt} untouched.
- `// Package config` doc appears EXACTLY ONCE (in config.go).

## DOCS impact
Mode A — godoc on each Config field + Default() + each default constant cites PRD §16.1/§16.2.
The comprehensive config-reference surface (precedence order, STAGEHAND_* env vars, stagehand.* git-config keys, .stagehand.toml keys → docs/CONFIGURATION.md) is OWNED by P1.M5.T3.S1. This PRP surfaces that so docs are not silently dropped.
