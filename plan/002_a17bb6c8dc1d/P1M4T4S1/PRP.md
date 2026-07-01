---
name: "P1.M4.T4.S1 — First-run auto-bootstrap fallback (PRD §9.17 FR-B3): refactor config init's TOML generation into a shared config.GenerateBootstrapConfig + auto-write on first Load()"
description: |

  Implement FR-B3: "if stagehand starts with no global config and no STAGEHAND_CONFIG, it auto-writes the
  bootstrap config once, prints a notice with the path, and continues — the tool is never unconfigured."
  The populated bootstrap generation (shipped in P1.M4.T2.S1, living in internal/cmd/config.go) is
  REFACTORED into a shared **`config.GenerateBootstrapConfig(provider string) string`** (new file
  internal/config/bootstrap.go) that BOTH `config init` and the new `config.Load()` fallback call. In Load(),
  when the global file is missing AND not explicit (no --config, no STAGEHAND_CONFIG), auto-write the
  populated config to globalConfigPath(), print `stagehand: wrote bootstrap config to <path>` to stderr
  (via noticeOut), then load it as Layer 2.

  CONTRACT (P1.M4.T4.S1, verbatim):
    1. RESEARCH: "FR-B3 … happens in config.Load() (internal/config/load.go) when the global file is absent
       AND no STAGEHAND_CONFIG override. It runs the same cascading detection + populated bootstrap as config
       init (P1.M4.T2.S1), then proceeds normally."
    2. INPUT: "The populated config init logic from P1.M4.T2.S1 (extract the bootstrap-generation into a
       reusable function)."
    3. LOGIC: "In Load(), when globalPath discovery yields a missing file AND !explicit (no --config, no
       STAGEHAND_CONFIG): call the bootstrap function (extracted from config init's write logic) to write the
       populated config, print a notice to stderr ('stagehand: wrote bootstrap config to <path>'), then
       proceed to read it as Layer 2. Refactor config init's TOML generation into a shared
       `GenerateBootstrapConfig(provider string) string` function that both config init and this fallback call."
    4. OUTPUT: "First run with no config → bootstrap config is written automatically, a notice is printed,
       and the tool works immediately."
    5. DOCS: "none — the notice printed at runtime IS the documentation."

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `internal/cmd/config.go` configCmd/configUpgradeCmd/runConfigUpgrade/upgradeConfigVersion +
      FR-B6 configCmd.Long dedup + init()'s AddCommand lines — PARALLEL P1.M4.T3.S1 (config upgrade). Do NOT
      touch those symbols/regions. My edits to config.go are CONFINED to: (a) deleting the moved
      bootstrap-helper block, (b) rewriting runConfigInit. Describe edits by SYMBOL, not line number.
    - `exampleConfigTemplate` (the --template inert config) STAYS in cmd/config.go (unchanged).
    - internal/provider/* (Complete P1.M2) — READ-ONLY (GenerateBootstrapConfig consumes the registry).
    - default_action.go, root.go, role_defaults.go, registry.go — NOT touched by this task.

  DELIVERABLES (2 NEW files, 4 EDITED files):
    CREATE internal/config/bootstrap.go        — GenerateBootstrapConfig(provider) + moved pure generation
                                                  (buildBootstrapConfig + helpers + consts) + bootstrapWriteConfig.
    CREATE internal/config/bootstrap_test.go   — moved TestBuildBootstrapConfig_* + new TestGenerateBootstrapConfig_*.
    EDIT   internal/config/load.go             — LoadOpts.DisableBootstrap (test seam) + the FR-B3 fallback branch.
    EDIT   internal/config/load_test.go        — DisableBootstrap:true on intent-contradicted tests + new bootstrap tests.
    EDIT   internal/cmd/config.go              — delete moved helpers; rewrite runConfigInit to call config.GenerateBootstrapConfig.
    EDIT   internal/cmd/config_test.go         — delete the 5 moved TestBuildBootstrapConfig_* tests.

  SUCCESS: a first run with no global config (and no --config/STAGEHAND_CONFIG) writes the populated
  bootstrap config, prints the notice to stderr, and Load() proceeds with that config (provider resolved,
  no advisory). `config init` still produces byte-identical output (same generation, now shared). Explicit
  missing paths still hard-error. `go build ./... && go test ./...` green; `go vet ./...` clean; go.mod/
  go.sum unchanged; the only files changed are the 6 listed above.

---

## Goal

**Feature Goal**: Realize PRD §9.17 FR-B3 — Stagehand is "never unconfigured." On the first run with no
global config and no explicit `--config`/`STAGEHAND_CONFIG`, `config.Load()` auto-writes the populated
bootstrap config (the same cascading-detection + per-role-models output as `config init`), prints a
one-line notice with the path to stderr, and continues normally. The bootstrap TOML generation is lifted
out of `internal/cmd/config.go` into a shared `config.GenerateBootstrapConfig(provider string) string` so
`config init` and the Load() fallback share ONE implementation.

**Deliverable** (2 new files + 4 edits):
1. CREATE `internal/config/bootstrap.go` — `GenerateBootstrapConfig(provider string) string` (detection via
   the registry + delegation to the pure generator), `bootstrapWriteConfig(path) error`, and the MOVED
   pure generation: `buildBootstrapConfig`, `writeRoleBlock`, `writeCommentedRoleBlock`, `stagerFallback`,
   `isInstalledName`, `bootstrapProviderNames`, `preferredBuiltins`, `bootstrapHeader`, `generationCommented`.
2. CREATE `internal/config/bootstrap_test.go` — the 5 moved `TestBuildBootstrapConfig_*` + new
   `TestGenerateBootstrapConfig_*`.
3. EDIT `internal/config/load.go` — add `LoadOpts.DisableBootstrap bool` (test seam) + the FR-B3 fallback
   branch in `Load()`.
4. EDIT `internal/config/load_test.go` — `DisableBootstrap: true` on the 3 intent-contradicted tests + new
   `TestLoad_FirstRun_Bootstrap*` tests; fix any other test the suite surfaces.
5. EDIT `internal/cmd/config.go` — delete the moved bootstrap helpers; rewrite `runConfigInit` to validate
   `--provider` then call `config.GenerateBootstrapConfig(providerName)`.
6. EDIT `internal/cmd/config_test.go` — delete the 5 moved `TestBuildBootstrapConfig_*` tests.

**Success Definition**:
- First run (no global file, no explicit override): `Load()` writes the populated config to
  `globalConfigPath()`, prints `stagehand: wrote bootstrap config to <path>` to `noticeOut` (os.Stderr),
  and returns a Config with the detected/"pi" provider and `ConfigVersion == CurrentConfigVersion` — and NO
  config_version advisory (the bootstrap file is current).
- `config init` output is BYTE-IDENTICAL to before (same generation code, now shared) — the existing
  `TestConfigInit_*` tests pass unchanged.
- Explicit missing paths (`--config /missing`, `STAGEHAND_CONFIG=/missing`) still hard-error (no bootstrap).
- `go build ./... && go test ./...` GREEN; `go vet ./...` clean; `gofmt -l internal/ cmd/` empty;
  go.mod/go.sum unchanged; only the 6 listed files differ.

## User Persona

**Target User**: a brand-new Stagehand user who just installed the binary and runs `stagehand` for the
first time WITHOUT having run `config init` (FR-B3 also covers installers that lack a post-install step).
They expect the tool to "just work," not to error with "no provider configured."

**Use Case**: `stagehand` (first run, no config) → the tool auto-writes a working bootstrap config, prints
"stagehand: wrote bootstrap config to ~/.config/stagehand/config.toml", and proceeds to generate the
commit message. The next run finds the config and is silent.

**User Journey**: install → `stagehand` → notice on stderr → commit generated → (later) `stagehand config
path` / edit the file / `config init --force` to customize. The tool was never "unconfigured."

**Pain Points Addressed**: the cold-start failure ("no provider configured and none of the built-ins are
installed" — actually a missing-config state) and the friction of remembering to run `config init` first.

## Why

- **Closes PRD §9.17 FR-B3 (P0).** The "tool is never unconfigured" guarantee is a v2 ship-list item;
  without it, a first run with no config hits the "no provider configured" error path.
- **One source of truth for the bootstrap.** Today the populated-config generator lives in the CLI layer
  (`cmd/config.go`); the library layer (`config.Load`) cannot reach it (import cycle: config can't import
  cmd). Extracting `GenerateBootstrapConfig` into `internal/config` lets BOTH `config init` and `Load()`
  share it — eliminating drift between the two write paths.
- **Deterministic, advisory-clean.** The bootstrap writes `config_version = CurrentConfigVersion`, so the
  load-time advisory (P1.M4.T1.S1) stays silent on a fresh bootstrap — no scary "missing config_version"
  warning on the user's first run.

## What

A refactor + a Load() branch:

**Refactor (internal/config/bootstrap.go, NEW):** move the PURE populated-config generator + its helpers +
consts out of `cmd/config.go` (byte-identical), and add the shared entry point:
```go
// GenerateBootstrapConfig returns the populated bootstrap TOML (PRD §9.17 FR-B1/B3). provider != "" is
// used directly (caller validates); "" ⇒ cascading auto-detect (FR-D1) ⇒ "pi" fallback. NO I/O; $PATH
// detection via the registry. Shared by `config init` and the Load() first-run fallback.
func GenerateBootstrapConfig(provider string) string {
    reg := provider.NewRegistry(nil)
    installed := bootstrapProviderNames(reg)
    target := provider
    if target == "" {
        if det := reg.DefaultProvider(installed); det != "" { target = det } else { target = "pi" }
    }
    return buildBootstrapConfig(target, installed)
}
```

**Load() branch (internal/config/load.go, EDIT):** when the global file is missing AND `!explicit` AND
`!opts.DisableBootstrap`, write the bootstrap + notice + re-read:
```go
} else if !opts.DisableBootstrap {
    if err := bootstrapWriteConfig(globalPath); err != nil {
        return nil, fmt.Errorf("bootstrap config: %w", err)
    }
    fmt.Fprintf(noticeOut, "stagehand: wrote bootstrap config to %s\n", globalPath)
    if g, err := loadTOML(globalPath); err != nil {
        return nil, fmt.Errorf("global config: %w", err)
    } else if g != nil {
        fileLoaded = true
        overlay(&cfg, g)
    }
}
```

`config init` (`internal/cmd/config.go`, EDIT) becomes: validate `--provider` (`reg.Get`; keep local
`preferredBuiltins` for the error message), then `content = config.GenerateBootstrapConfig(providerName)`.

### Success Criteria

- [ ] `config.GenerateBootstrapConfig(provider string) string` exists in `internal/config/bootstrap.go`;
      `""` ⇒ auto-detect→"pi"; non-empty ⇒ used directly; output is byte-identical to the old
      `buildBootstrapConfig` for the same (target, installed).
- [ ] `config init` (runConfigInit) calls `config.GenerateBootstrapConfig`; its output is UNCHANGED
      (TestConfigInit_ProviderPin_ExactOutput / _ProviderStagerFallback / _Populated_* pass unchanged).
- [ ] `Load()` with no global file + `!explicit` + `!DisableBootstrap` ⇒ writes the file, prints the
      notice to `noticeOut`, overlays it (provider resolved, `ConfigVersion==CurrentConfigVersion`, NO advisory).
- [ ] Explicit missing paths (`--config`/`STAGEHAND_CONFIG`) still hard-error BEFORE the fallback.
- [ ] `LoadOpts.DisableBootstrap bool` exists (defaults false; production never sets it; FR-B3 active).
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` clean; `gofmt -l` empty; go.mod/go.sum
      unchanged; only the 6 listed files differ.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact code to MOVE
(`buildBootstrapConfig` + helpers + consts, quoted in `internal/cmd/config.go` and reproduced in
findings.md §2), the verified no-cycle import fact (§1), the exact Load() branch to insert + its anchor
(the `if g, err := loadTOML(globalPath)` block), the `noticeOut`/`globalConfigPath`/`loadTOML` signatures
(all quoted), the test-seam rationale + the 3 tests to set it on, the new tests to add, and the
parallel-coordination map (non-overlapping symbol regions in cmd/config.go). No git/prompt/decompose
knowledge required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design + findings
- docfile: plan/002_a17bb6c8dc1d/P1M4T4S1/research/findings.md
  why: §1 the no-cycle import fact (load-bearing), §2 the exact symbols to MOVE, §3 the refactor shape, §4
       the behavior-change + DisableBootstrap seam rationale + the 3 tests that break, §5 the Load() branch,
       §6 the test plan, §7 parallel coordination with P1.M4.T3.S1, §8 confidence 8/10.
  critical: §1 (no cycle → bootstrap.go can import provider), §4 (DisableBootstrap seam — the test strategy),
       §7 (cmd/config.go edits are NON-OVERLAPPING with the parallel sibling).

# MUST READ — the file whose code MOVES (EDIT: delete helpers, rewrite runConfigInit)
- file: internal/cmd/config.go   (EDIT — the bootstrap code lives here today)
  section: `runConfigInit` (calls reg → configInitInstalledNames → resolveBootstrapTarget → buildBootstrapConfig);
       `resolveBootstrapTarget` (validation + cascade — SPLIT: validation stays in cmd, cascade moves into
       GenerateBootstrapConfig); `configInitInstalledNames` (→ bootstrapProviderNames, MOVE); `buildBootstrapConfig`
       (PURE, MOVE); `writeRoleBlock`/`writeCommentedRoleBlock`/`stagerFallback`/`isInstalledName` (MOVE);
       `bootstrapHeader`/`generationCommented` consts (MOVE); `preferredBuiltins` (KEEP in cmd for the
       --provider error message); `exampleConfigTemplate` (KEEP — the --template path, unchanged).
  why: this is the source of the move. After: runConfigInit = validate --provider (reg.Get + preferredBuiltins
       msg) + `config.GenerateBootstrapConfig(providerName)`. The move is byte-identical ⇒ config init output
       is unchanged.
  pattern: KEEP `provider.NewRegistry(nil)` + `reg.Get` for --provider validation (TestConfigInit_UnknownProvider
       asserts the exact error message — keep preferredBuiltins so the message is unchanged).
  gotcha: DO NOT touch configCmd / configUpgradeCmd / runConfigUpgrade / upgradeConfigVersion / init()'s
       AddCommand lines / configCmd.Long — those belong to the PARALLEL P1.M4.T3.S1. Edit by SYMBOL, not line.

# MUST READ — the new file's home + the Load() edit
- file: internal/config/load.go   (EDIT — add DisableBootstrap + the fallback branch)
  section: `type LoadOpts struct { ConfigPathOverride; RepoDir; Flags }` (add `DisableBootstrap bool`); the
       `Load()` global-file block:
         `globalPath := opts.ConfigPathOverride; explicit := ...; if globalPath=="" {... globalConfigPath()}`
         then `if g, err := loadTOML(globalPath); err != nil {...} else if g != nil {fileLoaded=true; overlay}
         else if explicit {return error}` — ADD `else if !opts.DisableBootstrap { bootstrapWriteConfig + notice + re-loadTOML + overlay }`.
  why: this is where FR-B3 fires. `noticeOut` (file.go:62, default os.Stderr) is the notice sink. The branch
       sits AFTER the explicit-missing hard-error, so explicit paths never bootstrap.
  pattern: mirror the existing overlay(&cfg, g) + fileLoaded=true idiom for the re-read.
  gotcha: the bootstrap writes config_version=CurrentConfigVersion ⇒ configVersionNotice(true, current)==""
       ⇒ no spurious advisory (verify: the advisory call at the end of Load uses fileLoaded+cfg.ConfigVersion).
  gotcha: DisableBootstrap is a TEST-ONLY seam (production callers — cmd PersistentPreRunE, pkg/stagehand.
       resolveConfig — never set it). Default false ⇒ FR-B3 active everywhere real.

# MUST READ — the sink + path + loader signatures (consume read-only)
- file: internal/config/file.go   (READ — noticeOut, globalConfigPath, loadTOML)
  section: `var noticeOut io.Writer = os.Stderr` (L62) + SetNoticeOut/NoticeOut (test swappable);
       `func globalConfigPath() string` (XDG > ~/.config/stagehand/config.toml); `func loadTOML(path) (*Config,error)`
       (MISSING ⇒ (nil,nil); the re-read contract the fallback relies on).
  why: the fallback writes to globalConfigPath(), notices to noticeOut, re-reads via loadTOML.

# MUST READ — the schema const + the per-provider model table (consume read-only)
- file: internal/config/config.go   (READ — CurrentConfigVersion, Defaults)
  section: `const CurrentConfigVersion = 2` (L18); `Defaults()` Provider="" (the behavior-change baseline).
  why: buildBootstrapConfig writes `config_version = CurrentConfigVersion` (keeps the advisory silent);
       Defaults().Provider="" is why the bootstrap (provider="pi") is a behavior change for tests.
- file: internal/config/role_defaults.go   (READ — DefaultModelsForProvider)
  section: `func DefaultModelsForProvider(name string) map[string]string` — the FR-D4 per-provider×per-role
       table buildBootstrapConfig reads. UNCHANGED (moved code calls it the same way, now same-package).

# MUST READ — the registry (consume read-only; NO cycle — findings §1)
- file: internal/provider/registry.go   (READ — NewRegistry, DefaultProvider, List, IsInstalled, Get)
  section: `func NewRegistry(overrides map[string]map[string]any) *Registry`; `func (r *Registry)
       DefaultProvider(installed []string) string` (FR-D1 cascade over preferredBuiltins);
       `List() []*Manifest` (sorted); `IsInstalled(m) bool` (exec.LookPath); `Get(name) (*Manifest, bool)`.
  why: GenerateBootstrapConfig + runConfigInit validation consume these. `preferredBuiltins` here is the
       canonical FR-D1 order — config/bootstrap.go keeps its OWN copy (mirrors, as cmd does today) to avoid
       touching the Complete provider package.

# MUST READ — the config init tests (EDIT: delete moved tests; verify init tests still pass)
- file: internal/cmd/config_test.go   (EDIT — delete 5 moved tests; the rest pass unchanged)
  section: `TestBuildBootstrapConfig_Pi/_GeminiStagerFallback/_OtherInstalledCommented/_NoInstallFallback/
       _ValidTOML` (L555-668 — MOVE to internal/config/bootstrap_test.go). `TestConfigInit_ProviderPin_
       ExactOutput`/`_ProviderStagerFallback`/`_Populated_*` (drive runConfigInit via Execute, assert exact
       output — PASS unchanged because generation is byte-identical). `TestConfigInit_UnknownProvider`
       (asserts the --provider error message — PASS unchanged because preferredBuiltins stays in cmd).
  why: confirms the refactor is output-preserving (the hard guarantee). Only the 5 pure-generator tests move.
  gotcha: do NOT touch the PARALLEL sibling's tests (TestUpgradeConfigVersion_*/TestConfigUpgrade_* L720-1018).

# MUST READ — the parallel sibling PRP (consume its outputs; coordinate on cmd/config.go)
- docfile: plan/002_a17bb6c8dc1d/P1M4T3S1/PRP.md   (PARALLEL — config upgrade)
  why: it edits the SAME file (internal/cmd/config.go). Its additions (configUpgradeCmd/runConfigUpgrade/
       upgradeConfigVersion + configCmd.AddCommand + FR-B6 configCmd.Long + toml/strconv/regexp imports) are
       in NON-OVERLAPPING regions from this task's (delete bootstrap helpers + rewrite runConfigInit).
  critical: edit by SYMBOL; do NOT touch configCmd/configUpgradeCmd/upgrade symbols/init() AddCommand/
       configCmd.Long. Sequence-agnostic (independent regions).

- url: (PRD internal) PRD.md §9.17 FR-B3 — the authoritative spec ("auto-writes the bootstrap config once,
       prints a notice with the path, and continues — the tool is never 'unconfigured'").
  critical: the fallback fires ONLY when missing AND !explicit (discovery path); explicit missing still errors.
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  load.go              # EDIT — LoadOpts.DisableBootstrap + the FR-B3 fallback branch.
  load_test.go         # EDIT — DisableBootstrap on 3 tests + new TestLoad_FirstRun_Bootstrap*.
  config.go            # READ CurrentConfigVersion(=2), Defaults(). DO NOT EDIT.
  file.go              # READ noticeOut, globalConfigPath, loadTOML. DO NOT EDIT.
  role_defaults.go     # READ DefaultModelsForProvider. DO NOT EDIT.
  bootstrap.go         # CREATE — GenerateBootstrapConfig + moved pure generation + bootstrapWriteConfig.
  bootstrap_test.go    # CREATE — moved TestBuildBootstrapConfig_* + new TestGenerateBootstrapConfig_*.
internal/cmd/
  config.go            # EDIT — delete moved helpers; rewrite runConfigInit (calls config.GenerateBootstrapConfig).
  config_test.go       # EDIT — delete the 5 moved TestBuildBootstrapConfig_*.
  root.go / providers.go / default_action.go  # NOT touched (P1.M4.T3.S1 owns root/providers edits this cycle).
internal/provider/registry.go   # READ — NewRegistry/DefaultProvider/List/IsInstalled/Get. DO NOT EDIT.
go.mod / go.sum        # UNCHANGED (go-toml/v2 + cobra + provider already deps; internal/provider already importable).
```

### Desired Codebase tree with files to be added/changed

```bash
internal/config/bootstrap.go        # CREATE — GenerateBootstrapConfig(provider) string; bootstrapWriteConfig(path);
                                     #        buildBootstrapConfig + writeRoleBlock/writeCommentedRoleBlock/
                                     #        stagerFallback/isInstalledName/bootstrapProviderNames (MOVED);
                                     #        preferredBuiltins/bootstrapHeader/generationCommented (MOVED).
internal/config/bootstrap_test.go   # CREATE — 5 moved TestBuildBootstrapConfig_* + TestGenerateBootstrapConfig_*.
internal/config/load.go             # EDIT — LoadOpts.DisableBootstrap bool; Load() FR-B3 fallback branch.
internal/config/load_test.go        # EDIT — DisableBootstrap:true on DefaultsOnly/DiscoveryMissingFileOK/
                                     #        ConfigVersionAdvisory_NoFile (+ any other the suite surfaces);
                                     #        + TestLoad_FirstRun_Bootstrap* (4-5 new tests).
internal/cmd/config.go              # EDIT — delete moved helpers/consts; rewrite runConfigInit
                                     #        (validate --provider via reg.Get + preferredBuiltins msg; then
                                     #        config.GenerateBootstrapConfig). KEEP exampleConfigTemplate + preferredBuiltins.
internal/cmd/config_test.go         # EDIT — delete the 5 moved TestBuildBootstrapConfig_*.
# go.mod/go.sum UNCHANGED. exampleConfigTemplate/configCmd/configUpgradeCmd/upgrade symbols UNCHANGED (sibling/frozen).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (NO IMPORT CYCLE — findings §1): internal/config does NOT import internal/provider today, and
// internal/provider does NOT import internal/config. So internal/config/bootstrap.go MAY import
// internal/provider (for $PATH detection in GenerateBootstrapConfig) safely. Only GenerateBootstrapConfig
// needs it; buildBootstrapConfig + helpers need only same-package config symbols.

// CRITICAL (the move is BYTE-IDENTICAL): buildBootstrapConfig and its helpers/consts move VERBATIM from
// cmd/config.go to config/bootstrap.go (only the package + the DefaultModelsForProvider/CurrentConfigVersion
// references become same-package instead of config.-qualified). The generated TOML is identical ⇒ config init
// output is unchanged ⇒ TestConfigInit_ProviderPin_ExactOutput/_ProviderStagerFallback/_Populated_* PASS as-is.

// CRITICAL (behavior change + DisableBootstrap seam — findings §4): Defaults().Provider="" but the bootstrap
// writes provider="pi". So a missing global config now resolves Provider="pi" AND writes a file + scans $PATH.
// Tests asserting "no config ⇒ pure defaults" (TestLoad_DefaultsOnly, _ConfigVersionAdvisory_NoFile,
// _DiscoveryMissingFileOK) BREAK. Add LoadOpts.DisableBootstrap (test-only seam; production leaves it false)
// and set it on those tests. RUN go test ./internal/config/ and set it on any other test the suite surfaces.

// GOTCHA (no spurious advisory): the bootstrap writes config_version=CurrentConfigVersion(=2). After overlay,
// configVersionNotice(fileLoaded=true, version=2) returns "" (current) ⇒ NO advisory on a fresh bootstrap.
// Do NOT add a special case — it falls out of buildBootstrapConfig already writing the version line.

// GOTCHA (explicit missing still errors): the fallback branch is an `else if !opts.DisableBootstrap` AFTER the
// `else if explicit { return error }` branch. So --config /missing and STAGEHAND_CONFIG=/missing still hard-error
// BEFORE the fallback. FR-B3 scope is the DISCOVERY path only.

// GOTCHA (keep preferredBuiltins in cmd for the error message): runConfigInit still validates --provider via
// reg.Get and the error message lists preferredBuiltins. KEEP a local preferredBuiltins in cmd/config.go so
// TestConfigInit_UnknownProvider's exact message is unchanged. (config/bootstrap.go has its OWN copy for
// stagerFallback + commented-block ordering — pre-existing mirror pattern; do NOT touch the provider package.)

// GOTCHA (GenerateBootstrapConfig has NO error return — contract signature): it does NOT validate `provider`.
// config init validates --provider FIRST (reg.Get), THEN calls GenerateBootstrapConfig(validatedName). The
// Load() fallback calls GenerateBootstrapConfig("") (auto-detect — never needs validation).

// GOTCHA (notice sink): the notice goes to noticeOut (file.go:62, default os.Stderr) — NOT a hardcoded
// os.Stderr. This matches "print a notice to stderr" AND makes it testable (tests swap noticeOut). Reuse it;
// do NOT introduce a new sink.

// GOTCHA (parallel sibling on cmd/config.go — findings §7): P1.M4.T3.S1 adds configUpgradeCmd + FR-B6 dedup
// to the SAME file. My edits (delete bootstrap helpers, rewrite runConfigInit) are NON-OVERLAPPING regions.
// Edit by SYMBOL; do NOT touch configCmd/configUpgradeCmd/upgrade symbols/init() AddCommand/configCmd.Long.
// In config_test.go: delete only the 5 TestBuildBootstrapConfig_*; leave the sibling's upgrade tests (L720+) alone.
```

## Implementation Blueprint

### Data models and structure

No new exported TYPES. The refactor moves existing functions/consts and adds two: `GenerateBootstrapConfig`
and `bootstrapWriteConfig` (both in internal/config). `LoadOpts` gains one field (`DisableBootstrap bool`).
`Config`/`RoleConfig`/`CurrentConfigVersion` are unchanged.

```go
// internal/config/bootstrap.go — NEW FILE (package config). Imports: fmt, os, path/filepath, strings,
// "github.com/dustin/stagehand/internal/provider". (go-toml NOT needed here — generation is string-building.)

// preferredBuiltins is the FR-D1 cascading provider priority (MOVED from cmd/config.go; mirrors
// internal/provider/registry.go's unexported slice). Used by stagerFallback + commented-block ordering.
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"}

// GenerateBootstrapConfig returns the populated bootstrap config TOML (PRD §9.17 FR-B1/B3). Shared by
// `config init` and the Load() first-run fallback. provider != "" is used as the target directly (the
// CALLER validates it — config init via reg.Get; this func has no error return per its contract);
// provider == "" ⇒ cascading auto-detect (FR-D1, reg.DefaultProvider over installed) ⇒ "pi" fallback.
// NO file I/O; $PATH detection runs via the registry. (P1.M4.T4.S1.)
func GenerateBootstrapConfig(provider string) string {
	reg := provider.NewRegistry(nil) // built-ins only
	installed := bootstrapProviderNames(reg)
	target := provider
	if target == "" {
		if det := reg.DefaultProvider(installed); det != "" {
			target = det
		} else {
			target = "pi" // nothing on $PATH — valid default; annotated by buildBootstrapConfig
		}
	}
	return buildBootstrapConfig(target, installed)
}

// bootstrapWriteConfig writes the populated bootstrap config to path (MkdirAll + WriteFile), used by the
// Load() first-run fallback (FR-B3). Returns a wrapped error on failure. (P1.M4.T4.S1.)
func bootstrapWriteConfig(path string) error {
	content := GenerateBootstrapConfig("")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

// bootstrapProviderNames returns built-in provider names whose command is on $PATH (MOVED from cmd's
// configInitInstalledNames). reg.List() is sorted ascending.
func bootstrapProviderNames(reg *provider.Registry) []string {
	var installed []string
	for _, m := range reg.List() {
		if reg.IsInstalled(m) {
			installed = append(installed, m.Name)
		}
	}
	return installed
}

// buildBootstrapConfig — the PURE populated-config generator (MOVED VERBATIM from cmd/config.go). NO
// detection, NO I/O; takes an already-resolved target + installed list, returns the exact TOML. Writes
// header docs, config_version (uncommented), [defaults] provider=<target>, four [role.*] blocks (stager
// routed to the fallback when target can't stage), each OTHER installed provider as commented [role.*],
// then a commented [generation] section. (PRD §9.17 FR-B1.)
func buildBootstrapConfig(target string, installed []string) string {
	// ... BYTE-IDENTICAL body from cmd/config.go (DefaultModelsForProvider/CurrentConfigVersion are now
	// same-package references; everything else unchanged) ...
}

// writeRoleBlock / writeCommentedRoleBlock / stagerFallback / isInstalledName — MOVED VERBATIM.
// bootstrapHeader / generationCommented — MOVED VERBATIM (consts).
```

```go
// internal/config/load.go — EDIT LoadOpts + Load().

type LoadOpts struct {
	ConfigPathOverride string
	RepoDir            string
	Flags              *pflag.FlagSet
	DisableBootstrap   bool // TEST-ONLY seam (FR-B3): true ⇒ skip the first-run auto-write. Production never sets it.
}

// In Load(), change the global-file block's tail:
//   } else if explicit {
//       return nil, fmt.Errorf("config file not found: %s", globalPath)
//   }
// to add the fallback:
//   } else if explicit {
//       return nil, fmt.Errorf("config file not found: %s", globalPath)
//   } else if !opts.DisableBootstrap {
//       // FR-B3 first-run fallback (P1.M4.T4.S1): no global config AND no explicit override → auto-write
//       // the populated bootstrap config, notice the path, then load it as Layer 2.
//       if err := bootstrapWriteConfig(globalPath); err != nil {
//           return nil, fmt.Errorf("bootstrap config: %w", err)
//       }
//       fmt.Fprintf(noticeOut, "stagehand: wrote bootstrap config to %s\n", globalPath)
//       if g, err := loadTOML(globalPath); err != nil {
//           return nil, fmt.Errorf("global config: %w", err)
//       } else if g != nil {
//           fileLoaded = true
//           overlay(&cfg, g)
//       }
//   }
```

```go
// internal/cmd/config.go — EDIT runConfigInit (the populated branch; --template path UNCHANGED).
// DELETE: resolveBootstrapTarget, configInitInstalledNames, stagerFallback, isInstalledName, writeRoleBlock,
//         writeCommentedRoleBlock, buildBootstrapConfig, bootstrapHeader, generationCommented.
// KEEP:   preferredBuiltins (local, for the --provider error message), exampleConfigTemplate, provider import.
// In runConfigInit, REPLACE the populated-config block:
//   reg := provider.NewRegistry(nil)
//   installed := configInitInstalledNames(reg)
//   target, err := resolveBootstrapTarget(reg, providerName, installed)
//   if err != nil { return exitcode.New(exitcode.Error, err) }
//   content = buildBootstrapConfig(target, installed)
// WITH:
//   providerName, _ := cmd.Flags().GetString("provider")
//   if providerName != "" {
//       reg := provider.NewRegistry(nil)
//       if _, ok := reg.Get(providerName); !ok {
//           return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q (use a built-in: %s)",
//               providerName, strings.Join(preferredBuiltins, ", ")))
//       }
//   }
//   content = config.GenerateBootstrapConfig(providerName)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/config/bootstrap.go (MOVE the pure generation + ADD the shared entry)
  - CREATE the file with: imports (fmt, os, path/filepath, strings, internal/provider); `preferredBuiltins`;
    `GenerateBootstrapConfig(provider string) string`; `bootstrapWriteConfig(path string) error`;
    `bootstrapProviderNames(reg)`; and buildBootstrapConfig/writeRoleBlock/writeCommentedRoleBlock/
    stagerFallback/isInstalledName/bootstrapHeader/generationCommented MOVED VERBATIM from cmd/config.go.
  - GOTCHA: the moved code referenced config.DefaultModelsForProvider / config.CurrentConfigVersion — in
    package config these are now bare DefaultModelsForProvider / CurrentConfigVersion (drop the config. prefix).
    Everything else is byte-identical.
  - GOTCHA: GenerateBootstrapConfig does NOT validate provider (no error return). bootstrapWriteConfig calls
    GenerateBootstrapConfig("") (auto-detect).

Task 2: EDIT internal/cmd/config.go (delete moved helpers; rewrite runConfigInit)
  - DELETE the symbols listed above. KEEP preferredBuiltins (local) + exampleConfigTemplate + provider import.
  - REWRITE runConfigInit's populated branch to: validate --provider (reg.Get + preferredBuiltins msg) →
    config.GenerateBootstrapConfig(providerName). The --template branch and force/existence/MkdirAll/WriteFile/
    print logic are UNCHANGED.
  - GOTCHA: edit by SYMBOL — do NOT touch configCmd, configUpgradeCmd, runConfigUpgrade, upgradeConfigVersion,
    configVersionLineRe, isTableHeader, leadingHeaderEnd, init()'s AddCommand lines, or configCmd.Long
    (PARALLEL P1.M4.T3.S1 owns those). If the import block lost `strings`/`fmt` consumers, verify they're still
    used (preferredBuiltins msg uses strings.Join; runConfigInit uses fmt — both still needed).

Task 3: EDIT internal/config/load.go (LoadOpts.DisableBootstrap + the FR-B3 fallback branch)
  - ADD `DisableBootstrap bool` to LoadOpts (with a doc comment: test-only seam; production leaves it false).
  - ADD the `else if !opts.DisableBootstrap { bootstrapWriteConfig + notice + re-loadTOML + overlay }` branch
    to the global-file block, AFTER `else if explicit { return error }`.
  - GOTCHA: the notice uses noticeOut (file.go:62), NOT os.Stderr directly. The re-read uses loadTOML + overlay
    + fileLoaded=true (same idiom as the present-file branch).

Task 4: CREATE internal/config/bootstrap_test.go (MOVE 5 tests + ADD generator tests)
  - MOVE TestBuildBootstrapConfig_Pi/_GeminiStagerFallback/_OtherInstalledCommented/_NoInstallFallback/_ValidTOML
    from cmd/config_test.go (they call buildBootstrapConfig directly — same package now, drop any config. prefix).
  - ADD TestGenerateBootstrapConfig_AutoDetectPi (("")⇒ provider="pi" when nothing on $PATH; valid TOML;
    config_version=2), TestGenerateBootstrapConfig_NamedProvider (("claude")⇒ provider="claude" + claude models).
  - GOTCHA: these are same-package (package config) — they can call buildBootstrapConfig/GenerateBootstrapConfig directly.

Task 5: EDIT internal/cmd/config_test.go (delete the 5 moved tests)
  - DELETE the 5 TestBuildBootstrapConfig_* (now in config/bootstrap_test.go). LEAVE everything else,
    INCLUDING the sibling's TestUpgradeConfigVersion_*/TestConfigUpgrade_* (L720+) and the TestConfigInit_*
    tests (they pass unchanged — generation is byte-identical, --provider error message unchanged).

Task 6: EDIT internal/config/load_test.go (seam on intent-contradicted tests + new bootstrap tests)
  - ADD `DisableBootstrap: true` to: TestLoad_DefaultsOnly, TestLoad_DiscoveryMissingFileOK,
    TestLoad_ConfigVersionAdvisory_NoFile (their intent is "no file ⇒ resolver defaults/no advisory").
  - RUN `go test ./internal/config/` — for ANY other test that now fails (side-effect: bootstrap writes a file
    + $PATH detect changes Provider/Roles), add `DisableBootstrap: true` to preserve its resolver-isolation intent.
  - ADD: TestLoad_FirstRun_BootstrapsConfig (no global file, DisableBootstrap=false ⇒ file written at
    globalConfigPath(); noticeOut contains "wrote bootstrap config"; cfg.Provider non-empty/"pi";
    cfg.ConfigVersion==CurrentConfigVersion; re-Load finds it); TestLoad_Bootstrap_SkippedWhenExplicit
    (ConfigPathOverride=/missing → error; STAGEHAND_CONFIG=/missing → error — no bootstrap, no file);
    TestLoad_Bootstrap_DisabledNoWrite (DisableBootstrap=true + no file ⇒ no file written, Defaults());
    TestLoad_Bootstrap_DoesNotReFire (Load twice in one temp dir ⇒ 2nd finds the file, single notice).
  - PATTERN: reuse loadEnvSetup (isolates HOME/XDG) + capture noticeOut via the save/restore idiom (origNoticeOut).
  - GOTCHA: capture noticeOut in the bootstrap tests (they print to it). For DoesNotReFire, assert the notice
    appears exactly once across two Load() calls in the same temp HOME.

Task 7: VERIFY (run all gates; fix before declaring done)
  - `go build ./... && go vet ./...` clean.
  - `go test -race ./internal/config/ -v` → green (moved + new bootstrap tests; seam-updated resolver tests).
  - `go test -race ./internal/cmd/ -v` → green (config init tests pass unchanged; 5 moved tests gone).
  - `go test ./...` → GREEN (no regression).
  - `gofmt -l internal/ cmd/` empty.
  - `git diff --exit-code go.mod go.sum` → empty.
  - `git status` shows EXACTLY 6 files (bootstrap.go, bootstrap_test.go NEW; load.go, load_test.go,
    cmd/config.go, cmd/config_test.go EDITED). No other file changed.
```

### Implementation Patterns & Key Details

```go
// PATTERN (shared generation, one source of truth): GenerateBootstrapConfig is the ONLY entry both config
// init and Load() call. buildBootstrapConfig stays PURE (target+installed → TOML) so it is unit-testable
// without $PATH. Detection (registry) lives ONLY in GenerateBootstrapConfig.

// PATTERN (byte-identical move): copy buildBootstrapConfig + helpers + consts VERBATIM; only same-package
// reference adjustments (drop config. prefix on DefaultModelsForProvider/CurrentConfigVersion). This is the
// hard guarantee that config init output is unchanged.

// PATTERN (test seam, mirrors noticeOut/Flags:nil): DisableBootstrap lets resolver-isolation tests opt out
// of the filesystem-mutating, $PATH-dependent fallback. Production never sets it ⇒ FR-B3 active everywhere real.

// CRITICAL (no spurious advisory): buildBootstrapConfig writes config_version=CurrentConfigVersion ⇒ after
// overlay, configVersionNotice(true, current)=="" ⇒ silent. Do NOT add a special case.

// CRITICAL (explicit-missing still errors): the fallback is the LAST else-if, after `else if explicit`.
// Order in the global-file block: error → present(overlay) → explicit-missing(error) → !DisableBootstrap(bootstrap).

// GOTCHA (keep preferredBuiltins in cmd): the --provider validation error message lists preferredBuiltins.
// Keep a local copy in cmd/config.go so TestConfigInit_UnknownProvider's exact message is unchanged.
// config/bootstrap.go has its OWN copy (pre-existing mirror pattern) — do NOT export/touch the provider package.

// GOTCHA (parallel sibling): edit cmd/config.go + cmd/config_test.go by SYMBOL. Do NOT touch configCmd,
// configUpgradeCmd, the upgrade symbols, init() AddCommand, configCmd.Long, or the sibling's tests.
```

### Integration Points

```yaml
CONFIG.LOAD (internal/config/load.go):
  - add: "LoadOpts.DisableBootstrap bool (test seam); Load() FR-B3 branch: missing+!explicit+!Disable ⇒
          bootstrapWriteConfig + notice(noticeOut) + re-loadTOML + overlay."

SHARED GENERATION (internal/config/bootstrap.go):
  - add: "GenerateBootstrapConfig(provider string) string — shared by config init AND the Load() fallback."
  - add: "bootstrapWriteConfig(path) error — MkdirAll + WriteFile of GenerateBootstrapConfig("")."

CONFIG INIT (internal/cmd/config.go):
  - change: "runConfigInit validates --provider (reg.Get) then calls config.GenerateBootstrapConfig(providerName).
             Moved helpers deleted. Output byte-identical."

NOTICE SINK (internal/config/file.go — read-only):
  - reuse: "noticeOut (default os.Stderr) — the bootstrap notice destination (testable via SetNoticeOut)."

SCHEMA VERSION (internal/config/config.go — read-only):
  - consume: "CurrentConfigVersion(=2) — buildBootstrapConfig writes it ⇒ no advisory after bootstrap."

REGISTRY (internal/provider/registry.go — read-only, NO cycle):
  - consume: "NewRegistry/DefaultProvider/List/IsInstalled/Get — detection + --provider validation."

GO.MODULE: change NONE. internal/provider already importable from config (no cycle); go-toml/cobra already deps.

FROZEN/LEAVE (do NOT edit):
  - configCmd / configUpgradeCmd / runConfigUpgrade / upgradeConfigVersion / configCmd.Long / init() AddCommand
    (PARALLEL P1.M4.T3.S1).
  - exampleConfigTemplate (stays in cmd; --template path unchanged).
  - internal/provider/* (Complete P1.M2); internal/config/config.go, file.go, role_defaults.go (read-only).
  - root.go, providers.go, default_action.go (not this task's scope).
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/config/bootstrap.go internal/config/bootstrap_test.go \
          internal/config/load.go internal/config/load_test.go \
          internal/cmd/config.go internal/cmd/config_test.go
go vet ./internal/config/ ./internal/cmd/
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; gofmt -l empty; go.mod/go.sum unchanged.
```

### Level 2: Unit + Component Tests

```bash
# The moved pure generator + new GenerateBootstrapConfig (same package):
go test -race ./internal/config/ -run "TestBuildBootstrapConfig|TestGenerateBootstrapConfig" -v
# The Load() fallback + seam:
go test -race ./internal/config/ -run "TestLoad_FirstRun_Bootstrap|TestLoad_Bootstrap|TestLoad_DefaultsOnly|TestLoad_DiscoveryMissingFileOK|TestLoad_ConfigVersionAdvisory_NoFile" -v
# config init still byte-identical (the refactor guarantee):
go test -race ./internal/cmd/ -run "TestConfigInit" -v
# Full suites:
go test -race ./internal/config/ -v
go test -race ./internal/cmd/ -v
go test ./...
# Expected: all green. TestConfigInit_ProviderPin_ExactOutput/_ProviderStagerFallback/_Populated_* pass
# unchanged (byte-identical output). TestConfigInit_UnknownProvider passes (preferredBuiltins msg kept).
```

### Level 3: Integration Testing (the real first-run behavior)

```bash
make build

# --- first run with NO global config (FR-B3) ---
T=$(mktemp -d); export HOME="$T" XDG_CONFIG_HOME="$T"   # no config.toml exists
( cd "$(mktemp -d)" && git init -q . && git config user.email t@t.co && git config user.name t &&
  echo hi > a.txt && git add a.txt &&
  /home/dustin/projects/stagehand/bin/stagehand --provider stub --dry-run 2>notice.txt; echo "exit=$?" )
echo "--- notice (stderr) ---"; cat notice.txt
# Expected: notice contains "stagehand: wrote bootstrap config to <T>/stagehand/config.toml".
test -f "$T/stagehand/config.toml" && echo "PASS: bootstrap config written"
grep -q '^config_version = 2$' "$T/stagehand/config.toml" && echo "PASS: version 2"
grep -q 'provider = "pi"' "$T/stagehand/config.toml" && echo "PASS: provider pi (or detected)"

# --- explicit --config /missing still hard-errors (no bootstrap) ---
( cd "$(mktemp -d)" && /home/dustin/projects/stagehand/bin/stagehand --config /nope/missing.toml --provider stub --dry-run; echo "exit=$?" )
# Expected: exit != 0; "config file not found: /nope/missing.toml"; NO file written, NO bootstrap notice.

# --- second run is silent (config now exists) ---
( cd "$(mktemp -d)" && /home/dustin/projects/stagehand/bin/stagehand --provider stub --dry-run 2>n2.txt; echo "exit=$?" )
grep -q "wrote bootstrap config" n2.txt && echo "UNEXPECTED: 2nd run re-bootstrapped" || echo "PASS: no re-bootstrap"
# Expected: no bootstrap notice on the 2nd run (the file exists).
```

### Level 4: Regression & Audit

```bash
# config init output is byte-identical (the hard refactor guarantee):
T=$(mktemp -d); export HOME="$T" XDG_CONFIG_HOME="$T"
/home/dustin/projects/stagehand/bin/stagehand config init --provider pi >/dev/null 2>&1
diff <(cat "$T/stagehand/config.toml") <(git stash list >/dev/null; echo "compare against pre-refactor golden if available")
# (Or: go test -race ./internal/cmd/ -run TestConfigInit_ProviderPin_ExactOutput -v — the canonical byte-identical check.)

# Race + full regression (the gate):
go test -race ./...
go vet ./...
gofmt -l internal/ cmd/
# Expected: all green; exactly 6 files changed (2 new, 4 edited); go.mod/go.sum unchanged.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l internal/ cmd/` empty; `go vet ./internal/config/ ./internal/cmd/` clean.
- [ ] Level 2: `go test -race ./internal/config/` + `./internal/cmd/` green; `go test ./...` green.
- [ ] Level 3: first run writes the bootstrap + prints the notice + proceeds; explicit-missing still errors.
- [ ] Level 4: `go build ./...`; `git status` shows EXACTLY 6 files; go.mod/go.sum unchanged.

### Feature Validation

- [ ] `Load()` with no global file + `!explicit` + `!DisableBootstrap` ⇒ writes the populated config, prints
      `stagehand: wrote bootstrap config to <path>` to `noticeOut`, overlays it (provider resolved,
      `ConfigVersion==CurrentConfigVersion`, NO advisory).
- [ ] `config GenerateBootstrapConfig("")` ⇒ auto-detect→"pi" (nothing on $PATH); `GenerateBootstrapConfig("claude")`
      ⇒ claude config. Output == old buildBootstrapConfig for the same (target, installed).
- [ ] `config init` output byte-identical (TestConfigInit_ProviderPin_ExactOutput etc. pass unchanged).
- [ ] Explicit missing (`--config`/`STAGEHAND_CONFIG`) still hard-errors BEFORE the fallback.
- [ ] `LoadOpts.DisableBootstrap` exists; production callers never set it (FR-B3 active).

### Code Quality Validation

- [ ] `GenerateBootstrapConfig`/`buildBootstrapConfig` split: detection in the former, PURE generation in the latter.
- [ ] The move is byte-identical (only same-package reference adjustments).
- [ ] `DisableBootstrap` seam mirrors existing seams (noticeOut, Flags:nil) — test-only, documented.
- [ ] File placement matches the desired tree (only the 6 listed files); no cross-package churn (provider untouched).
- [ ] Anti-patterns avoided (see below): no cycle, no validation-in-generator, no sibling overwrite.

### Documentation & Deployment

- [ ] `GenerateBootstrapConfig`/`bootstrapWriteConfig`/`DisableBootstrap` have doc comments naming FR-B3/P1.M4.T4.S1.
- [ ] The runtime notice ("stagehand: wrote bootstrap config to <path>") IS the documentation (contract DOCS: none).
- [ ] Implementation summary records: the move, the Load() branch, the seam, the parallel-coordination outcome.

---

## Anti-Patterns to Avoid

- ❌ **Don't introduce an import cycle.** internal/config CAN import internal/provider (verified: neither imports the
  other today). But do NOT make internal/provider import internal/config (it doesn't today — keep it that way). Only
  `GenerateBootstrapConfig` needs the registry; keep buildBootstrapConfig provider-free.
- ❌ **Don't duplicate-drift the bootstrap generation.** The WHOLE POINT is ONE shared `GenerateBootstrapConfig`.
  Do NOT leave a second copy of the TOML generation in cmd/config.go. Move it; cmd calls the shared function.
- ❌ **Don't put validation inside `GenerateBootstrapConfig`.** Its contract signature is `(provider string) string`
  (no error). config init validates `--provider` separately (reg.Get); the Load() fallback passes `""`. Validation
  stays in the caller.
- ❌ **Don't break resolver-isolation tests by omitting the seam.** The bootstrap is a filesystem-mutating,
  $PATH-dependent side effect. Tests that exercise the resolver with no global config MUST be able to opt out
  (`DisableBootstrap`) — otherwise they become non-deterministic / wrong. Add the seam; set it where intent requires.
- ❌ **Don't fire the advisory on a fresh bootstrap.** buildBootstrapConfig writes `config_version=CurrentConfigVersion`;
  that MUST keep `configVersionNotice` silent. If you see an advisory on first run, you broke the version line.
- ❌ **Don't bootstrap on explicit-missing paths.** The fallback is the LAST else-if, after `else if explicit { error }`.
  `--config /missing` and `STAGEHAND_CONFIG=/missing` hard-error; FR-B3 is the DISCOVERY path only.
- ❌ **Don't overwrite/touch the parallel sibling's symbols in cmd/config.go.** P1.M4.T3.S1 owns configCmd,
  configUpgradeCmd, the upgrade symbols, init()'s AddCommand, and configCmd.Long (FR-B6). My edits are the
  bootstrap-helper deletions + the runConfigInit rewrite — NON-OVERLAPPING. Edit by SYMBOL.
- ❌ **Don't touch the provider package to "dedupe" preferredBuiltins.** It's a Complete milestone (P1.M2) and the
  mirror-copy pattern is pre-existing. config/bootstrap.go and cmd/config.go each keep a local copy.
- ❌ **Don't hardcode `os.Stderr` for the notice.** Use `noticeOut` (file.go:62) — it's the testable sink and the
  existing convention (matches §19 notices).
