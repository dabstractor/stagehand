# Research Notes — P1.M7.T3.S1: providers list / show subcommands (FR46–FR48)

## Contract (from tasks.json)
- `stagehand providers list` — print all built-in+user provider names, mark
  detected (on PATH) vs not via `Detect()`, show the resolved default
  provider+model (FR46).
- `stagehand providers show <name>` — print the FULLY-RESOLVED manifest
  (built-in field-merged with user overrides) as TOML (FR47).
- MOCKING: list marks pi/claude/etc detected (real PATH) + fabricated
  not-detected; show emits TOML that decodes back to the same manifest; a user
  override is reflected in show output.
- DOCS: Mode A — the list/show output format is the provider reference surface.
- Dependencies: P1.M2.T3.S2 (provider.Registry), P1.M5.T3.S1 (config.Load).
  NOT dependent on M7.T2.S1 (cobra root + global flags), so this must add the
  `providers` command tree WITHOUT touching main.go's rootCmd Run/flags.

## Dependency API (verified in source)

### provider.Registry — internal/provider/registry.go (DONE)
```go
func NewRegistry(builtins, overrides map[string]Manifest) *Registry
func (r *Registry) Get(name string) (Manifest, bool)        // ok=false if unknown
func (r *Registry) List() []string                          // SORTED, no dupes, fresh slice
func (r *Registry) Detect() map[string]bool                 // exec.LookPath(Detect or Command); ONLY I/O method
```

### config.Load — internal/config/load.go (DONE)
```go
func Load(flags Flags, repoDir string) (cfg Config, reg *provider.Registry, trustNotice string, err error)
// repoDir == "" => inherit cwd (filepath.Join("", ".stagehand.toml") => ".stagehand.toml")
// trustNotice is IGNORED by providers commands (only the commit action prints it).
```
- Returns `*provider.Registry` (Get/List/Detect are pointer receivers).
- `Default()` leaves `cfg.Provider == ""` and `cfg.Model == ""`.

### provider.Manifest — internal/provider/manifest.go
- Has `toml:"..."` struct tags on EVERY field → directly encodable with
  `toml.NewEncoder(w).Encode(m)` (pelletier/go-toml/v2). No DTO needed.

## CRITICAL gotcha: go-toml/v2 nil-vs-empty slice round-trip (verified empirically)

Encoding a `Manifest` with go-toml/v2 and decoding it back does NOT preserve
`reflect.DeepEqual` for providers whose `Subcommand`/`BareFlags` are NIL:
- A nil slice encodes as `key = []` and DECODES to a NON-NIL empty slice
  `[]string{}`. So `pi` (Subcommand nil), `gemini` (Subcommand nil), `opencode`
  (BareFlags nil) FAIL a naive `reflect.DeepEqual(orig, decoded)`.
- A nil map (`Env`) is OMITTED from output and decodes back to nil → OK.
- `codex`/`claude`/`cursor` happen to round-trip exactly because their slices
  are non-nil/non-empty.

Root cause: **TOML cannot represent nil vs empty** — they are the same value.
=> The round-trip is semantically faithful; the TEST must compare SEMANTICALLY
using a normalization helper that treats nil and `[]string{}` (and nil vs
`map[string]string{}`) as equal. Scalars + bool compared exactly. This helper
lives in `providers_test.go`. This is the correct, defensible behavior, not a
workaround.

## CLI structure decision

- `cmd/stagehand/main.go` currently has a bare `rootCmd` (no Run, no children).
- M7.T2.S1 (sibling, NOT a dependency) will add rootCmd.Run + persistent flags.
- To add the `providers` tree with ZERO conflict risk → new file
  `cmd/stagehand/providers.go` (package main) registering the command via a
  package-level `init()` that calls `rootCmd.AddCommand(newProvidersCmd())`.
  No golangci config exists, so init() is safe; keeps main.go untouched.

## Default-resolution logic
`Default()` leaves Provider="" (the common no-config case). FR46 wants "the
resolved default" shown. Resolution:
```
defaultName  = cfg.Provider
if defaultName == "" { defaultName = first detected provider in reg.List() order }
defaultModel = cfg.Model
if defaultModel == "" { if m,ok := reg.Get(defaultName); ok { defaultModel = m.DefaultModel } }
```
Deterministic + testable. If no provider detected at all, defaultName="" → print
"(none detected)".

## Testability design (separate pure rendering from cobra)
- `renderProvidersList(w, reg, detected, defaultName, defaultModel)` — pure,
  tested with a hand-built registry (Builtins() + a fabricated
  "definitely-not-an-agent-xyz" override) and explicit default.
- `show` uses `toml.NewEncoder(w).Encode(manifest)` directly; tested by decoding.
- Override-reflected test: write `.stagehand.toml` with `[provider.pi]
  default_model = "overridden"` in a temp dir, call `config.Load(Flags{}, dir)`,
  assert `reg.Get("pi").DefaultModel == "overridden"` and that it appears in
  the show TOML.

## Exit codes / errors
- ui.ExitSuccess=0, ui.ExitError=1 (internal/ui/exitcode.go).
- `show` uses `cobra.ExactArgs(1)`; unknown name → RunE returns error → main's
  `rootCmd.Execute()` returns err → `os.Exit(1)`.
- `list` uses `cobra.NoArgs`.
- Parent `providers` with no Run → prints help (exit 0).

## DOCS impact (Mode A)
Create `docs/PROVIDERS.md` owning the providers list/show output format
(the provider reference surface). M5.T3.S1 created docs/CONFIGURATION.md;
M8.T1.S1 will write providers/*.toml reference manifests.
