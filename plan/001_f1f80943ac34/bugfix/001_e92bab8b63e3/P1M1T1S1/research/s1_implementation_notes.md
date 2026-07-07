# S1 Implementation Notes — resolved-config injection (research)

> Scope: P1.M1.T1.S1 — add `Config *config.Config` to `pkg/stagecoach.Options`; skip
> `config.Load` in `resolveConfig` when non-nil. Verified against the live source on 2026-06-30.

## 1. The single edit locus (confirmed verbatim)

`pkg/stagecoach/stagecoach.go`:

- **`Options` struct** — currently 7 fields (Provider, Model, SystemExtra, DryRun, Timeout, Verbose,
  VerboseOn). Additive-only contract ("Stable as of v1.0 / ADDITIVE-ONLY"). The new `Config` field
  goes here.
- **`resolveConfig(ctx, opts Options) (config.Config, string, error)`** — line ~108-141. Body:
  `repoDir, _ := os.Getwd()` → `cfgPtr, err := config.Load(ctx, config.LoadOpts{RepoDir: repoDir, Flags: nil})`
  → `cfg := *cfgPtr` → apply opts overrides → return. **The `config.Load` call is the ONLY thing
  that changes.** The `os.Getwd()` repoDir derivation and the override block are preserved verbatim.

## 2. Copy semantics — `cfg := *opts.Config` is a SHALLOW copy (safe here, matches existing pattern)

`config.Config` contains a `Providers map[string]map[string]any` field (`config.go:55`). A value copy
`cfg := *opts.Config` copies the **map header** (the pointer), NOT the map body — so `cfg.Providers`
aliases `opts.Config.Providers`.

**This is SAFE and intentional** for two reasons:
1. **No downstream mutation of the map.** `resolveConfig`'s override block only writes scalar fields
   (`cfg.Provider`, `.Model`, `.Timeout`, `.Verbose`) — never `cfg.Providers[k]`. `buildDeps` reads
   `cfg.Providers` via `provider.DecodeUserOverrides(cfg.Providers)` (re-encodes to TOML; reads only).
   So the shared map is never written through the copy.
2. **It is the IDENTICAL pattern to the existing code.** The current `cfg := *cfgPtr` (where `cfgPtr`
   is `Load`'s `*Config` return) is the exact same shallow copy. Introducing `cfg := *opts.Config`
   for the injected path introduces **no new aliasing risk** vs. status quo.

> Implementer: do NOT deep-copy / clone the Providers map. The contract says "set cfg := *opts.Config
> (copy)" — a plain value copy. Deep-copying is scope creep and would diverge from the existing
> `cfg := *cfgPtr` pattern used for the Load path.

## 3. No new import edge

`pkg/stagecoach/stagecoach.go` already imports `"github.com/dustin/stagecoach/internal/config"` (used
in `resolveConfig`'s return type and `config.Load`). The new field type `*config.Config` reuses it.
**Zero import changes.** No import-cycle risk (config does not import pkg/stagecoach).

## 4. API-stability — additive-only, in-module only

Per `decisions.md` D1: external (out-of-module) callers cannot name the unexported
`internal/config.Config` type, so they cannot construct a non-nil `Options.Config`. That is fine —
the field exists for the **in-module CLI** (`runDefault` in S2); standalone library callers leave it
nil and get the existing single-`Load` behavior unchanged. Adding the field is explicitly permitted
by the "ADDITIVE-ONLY" doc on `Options`.

## 5. Existing tests are UNAFFECTED (the nil path is unchanged)

Every existing test in `pkg/stagecoach/stagecoach_test.go` constructs `Options{Provider: "stub", …}`
with **no `Config` field** → `opts.Config == nil` → the existing `config.Load` branch runs
**unchanged**. Verified test inventory: `_Success`, `_DryRun`, `_NothingStaged`, `_ProviderOverride`,
`_Timeout` (dryrun + commit_path), `_SystemExtra`. All pass on the nil path → `go test -race ./...`
stays green with zero edits to existing tests.

## 6. What S1 does NOT do (scope discipline — owned by S2/S3)

- **S2** wires `runDefault` (`internal/cmd/default_action.go:147`) to pass `Config: cfg` from
  `Config()`. S1 does NOT touch `internal/cmd/`.
- **S3** adds the CLI-level regression tests: `--config` honored by the default action end-to-end,
  and the §19 notice printed exactly **once** (captured via stderr in `default_action_test.go`).
  S1 does NOT add those cross-package assertions.
- S1 changes neither the `GenerateCommit` signature nor the `Result` struct.
- S1 adds no docs (contract: "DOCS: none").

## 7. S1's own verifiability (within pkg/stagecoach, no CLI)

At the `pkg/stagecoach` level we can directly assert S1's contract:
- Inject `Options{Config: &config.Config{Provider: "stub", Providers: {stub→...}}, ...}` →
  `GenerateCommit` resolves the stub provider → proves `config.Load` was skipped (Load would NOT see
  the in-memory `Providers` map; only the injected config carries it).
- The "no second Load / notice printed once" property is **only** observable end-to-end through the
  CLI (the `loadRepoLocalConfig` notice writes to `internal/config.noticeOut`, an unexported var
  inaccessible from `pkg/stagecoach` tests). That assertion lives in **S3**, not S1.

So S1 ships one focused unit test proving the injected-config path resolves the injected provider
without any on-disk config file — and defers the stderr/notice-count regression to S3.
