# P1.M4.T4.S1 Research Findings — First-run auto-bootstrap fallback (FR-B3)

## TL;DR
Implement FR-B3: when `config.Load()` finds the global config **missing AND not explicit** (no `--config`,
no `STAGECOACH_CONFIG`), auto-write the populated bootstrap config once, print `stagecoach: wrote bootstrap
config to <path>` to stderr (via `noticeOut`), then load it as Layer 2. The bootstrap generation is
**refactored out of `internal/cmd/config.go`** into a shared **`config.GenerateBootstrapConfig(provider string) string`**
that BOTH `config init` and this fallback call.

## §1 Import graph (the load-bearing fact)
- `internal/config` does NOT import `internal/provider` (grep: empty).
- `internal/provider` does NOT import `internal/config` (grep: empty).
- ⇒ `internal/config/bootstrap.go` MAY import `internal/provider` (for `$PATH` detection) with **NO cycle**.
  Only `GenerateBootstrapConfig` (detection) needs `internal/provider`; the pure `buildBootstrapConfig`
  needs only `config.DefaultModelsForProvider` + `config.CurrentConfigVersion` (same package).

## §2 What already exists (P1.M4.T2.S1 — Complete) — the code to MOVE
`internal/cmd/config.go` currently owns the populated bootstrap:
- `buildBootstrapConfig(target, installed) string` — PURE TOML generator (no I/O, no detection).
- Helpers: `writeRoleBlock`, `writeCommentedRoleBlock`, `stagerFallback`, `isInstalledName`.
- Detection: `configInitInstalledNames(reg)`, `resolveBootstrapTarget(reg, providerName, installed)`
  (does BOTH --provider validation AND cascade detect + "pi" fallback).
- Consts: `bootstrapHeader`, `generationCommented`. Var: `preferredBuiltins` (local mirror of registry's).
- `runConfigInit` wires it: `reg := provider.NewRegistry(nil)` → installed → resolveBootstrapTarget →
  buildBootstrapConfig → WriteFile. `--template` path uses `exampleConfigTemplate` (STAYS in cmd).
- **Only referenced in `internal/cmd/config.go` + `config_test.go`** (5 `TestBuildBootstrapConfig_*` tests,
  lines 555-668). No other package touches them ⇒ the move is self-contained.

## §3 The refactor (the contract's "shared GenerateBootstrapConfig")
CREATE `internal/config/bootstrap.go` (imports: fmt, strings, internal/provider):
- MOVE: `buildBootstrapConfig`, `writeRoleBlock`, `writeCommentedRoleBlock`, `stagerFallback`,
  `isInstalledName`, `bootstrapHeader`, `generationCommented`, `preferredBuiltins`, `configInitInstalledNames`
  (rename → `bootstrapProviderNames`).
- ADD `GenerateBootstrapConfig(provider string) string`: builds a registry, gets installed, resolves target
  (`provider` if non-empty; else `reg.DefaultProvider(installed)`; else `"pi"`), returns `buildBootstrapConfig`.
  **No error return** (contract signature). It does NOT validate `provider` — callers validate first
  (`config init --provider` validates via `reg.Get`; the fallback passes `""`).
- ADD `bootstrapWriteConfig(path string) error` (MkdirAll + WriteFile of `GenerateBootstrapConfig("")`).

EDIT `internal/cmd/config.go`:
- DELETE the moved symbols. KEEP `preferredBuiltins` (local) for the `--provider` validation error message
  (preserves `TestConfigInit_UnknownProvider`'s exact message — avoids touching the provider package).
- REWRITE `runConfigInit`: validate `--provider` (`reg.Get`; error uses `preferredBuiltins`), then
  `content = config.GenerateBootstrapConfig(providerName)`. `--template` path UNCHANGED.
- The move is byte-identical generation ⇒ `TestConfigInit_ProviderPin_ExactOutput` /
  `TestConfigInit_ProviderStagerFallback` / `TestConfigInit_Populated_*` PASS unchanged (same TOML out).

## §4 The behavior change + the test seam (CRITICAL)
`Defaults().Provider = ""`. The bootstrap writes `[defaults] provider = "pi"` (or detected). So a missing
global config now resolves `cfg.Provider = "pi"` (was `""`), AND writes a file + does `$PATH` detection.
This BREAKS tests that assert "no config ⇒ pure defaults":
- `TestLoad_DefaultsOnly` (L453): asserts `cfg.Provider == Defaults().Provider` (`""`) → **FAILS**.
- `TestLoad_ConfigVersionAdvisory_NoFile` (L1128): captures `noticeOut`, asserts `buf==""` AND
  `ConfigVersion==0` → **FAILS** (bootstrap notice + `config_version=2`).
- `TestLoad_DiscoveryMissingFileOK` (L769): asserts `err==nil && cfg!=nil` → still passes, but its INTENT
  ("discovery tolerates absence") is now wrong (it bootstraps). Side-effects a file write + `$PATH` scan.
- Other no-global-config tests (`EnvOverridesGit`, `CLIOverridesEnv`, …) set env/flag that OVERRIDE
  provider ⇒ functionally still pass, but each now side-effects a write + non-deterministic `$PATH` detect.

**Decision: add `LoadOpts.DisableBootstrap bool` — a test-only seam.** Rationale: the bootstrap is a
filesystem-mutating, `$PATH`-dependent side effect; resolver-isolation tests must opt out to preserve
intent + determinism. Production callers (`cmd.PersistentPreRunE`, `pkg/stagecoach.resolveConfig`) leave it
`false` ⇒ FR-B3 fully active. Mirrors the existing `noticeOut` swappable sink + `Flags: nil` seams. The
contract ("missing + !explicit ⇒ bootstrap") is preserved for ALL production paths; the seam is additive.

The implementing agent: add `DisableBootstrap: true` to the 3 intent-contradicted tests above, RUN
`go test ./internal/config/`, and add it to any OTHER test that fails from the side effect (empirical).
Then ADD new bootstrap tests (§5).

## §5 Load() fallback (the core logic)
In `Load()`, the global-file block currently is:
```go
if g, err := loadTOML(globalPath); err != nil {
    return nil, fmt.Errorf("global config: %w", err)
} else if g != nil {
    fileLoaded = true; overlay(&cfg, g)
} else if explicit {
    return nil, fmt.Errorf("config file not found: %s", globalPath)
}
```
ADD an `else if !opts.DisableBootstrap` branch (the FR-B3 fallback): `bootstrapWriteConfig(globalPath)` →
`fmt.Fprintf(noticeOut, "stagecoach: wrote bootstrap config to %s\n", globalPath)` → re-`loadTOML` + overlay.
- The bootstrap config carries `config_version = CurrentConfigVersion` (buildBootstrapConfig writes it) ⇒
  `configVersionNotice(true, 2)` returns `""` ⇒ **no spurious advisory** after bootstrap. ✓
- Notice goes to `noticeOut` (default `os.Stderr`) — matches "print a notice to stderr". ✓
- `explicit` (–config / STAGECOACH_CONFIG) + missing still hard-errors BEFORE this branch ⇒ fallback never
  fires for explicit paths (FR-B3 scope: discovery only). ✓

## §6 New tests (internal/config/bootstrap_test.go + load_test.go)
- MOVE the 5 `TestBuildBootstrapConfig_*` (cmd/config_test.go:555-668) → `internal/config/bootstrap_test.go`
  (same package; call `buildBootstrapConfig` directly — byte-identical).
- ADD `TestGenerateBootstrapConfig_*`: `("")` ⇒ `[defaults] provider = "pi"` (nothing on $PATH in CI) +
  valid TOML + `config_version = 2`; `("claude")` ⇒ provider claude + claude role models.
- ADD `TestLoad_FirstRun_Bootstraps`: no global file, `DisableBootstrap=false` ⇒ file exists at
  `globalConfigPath()` after Load; `noticeOut` contains the notice; `cfg.Provider=="pi"`; re-Load finds it.
- ADD `TestLoad_Bootstrap_SkippedWhenExplicit` (–config missing → error; STAGECOACH_CONFIG missing → error).
- ADD `TestLoad_Bootstrap_DisabledNoWrite` (`DisableBootstrap=true` + no file ⇒ no file written, defaults).
- ADD `TestLoad_Bootstrap_DoesNotReFire` (Load twice ⇒ 2nd finds file, single notice).

## §7 Parallel coordination (P1.M4.T3.S1 — config upgrade, in flight)
- P1.M4.T3.S1 edits `internal/cmd/config.go`: ADDS `configUpgradeCmd`/`runConfigUpgrade`/
  `upgradeConfigVersion`/helpers + `configCmd.AddCommand(configUpgradeCmd)` + FR-B6 (removes configCmd.Long
  "Subcommands:" block) + toml/strconv/regexp imports. Its tests (`TestUpgradeConfigVersion_*`/
  `TestConfigUpgrade_*`) are ALREADY in `config_test.go` (lines 720-1018).
- THIS task edits `internal/cmd/config.go`: DELETES the bootstrap-helper block + REWRITES `runConfigInit`.
- **Non-overlapping regions**: my deletions/rewrite do NOT touch configCmd, configUpgradeCmd, the upgrade
  symbols, init()'s AddCommand lines, or configCmd.Long. Merge may shift line numbers but is semantically
  independent. `config_test.go`: I remove the 5 `TestBuildBootstrapConfig_*` (moved); P1.M4.T3.S1's upgrade
  tests are untouched. ⇒ Describe edits by SYMBOL/REGION, not line numbers.
- root.go / providers.go / load.go / bootstrap.go are NOT touched by P1.M4.T3.S1 ⇒ conflict-free there.

## §8 Confidence: 8/10
Sound design (no cycle verified; generation move is byte-identical; seam preserves test intent). Residual
risk: breadth of test breakage in `load_test.go` (mitigated by the seam + "run the suite, fix failures"
instruction) and the parallel merge on `cmd/config.go` (mitigated: non-overlapping symbol regions).
